package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
)

// Registry — реестр одного запуска; после конструктора закрыт для изменений.
type Registry struct {
	// definitions — описания, после подготовки отсортированные по номеру V-кода.
	definitions []Definition
	// failure — ошибка регистрации, включая проигнорированный отказ Register.
	failure error
	// sealed — запрет Register после завершения попытки создания реестра.
	sealed bool
}

// NewRegistry создаёт пустой реестр и сам последовательно вызывает все
// функции переданной карты, затем закрывает его для регистрации.
// Порядок регистрации несущественен; выполнение идёт по номеру V-кода.
// nil и пустая карта дают пустой объект реестра без ошибки регистрации;
// app обязан завершить такой запуск с кодом 1 до открытия JSON.
// Пустой ключ, nil-функция, повторный код либо
// ошибка регистрации дают nil и err; частичный реестр не публикуется.
// После первого отказа последующие регистрационные функции не вызывает.
// Карту не сохраняет и не изменяет; даже при ошибке реестр закрывает.
// Непустой failure также запрещает публикацию, даже если callback вернул nil.
func NewRegistry(registrations map[string]RegistrationFunc) (*Registry, error) {
	// Ненулевой срез отличает создаваемый реестр от нулевого Registry.
	r := &Registry{definitions: make([]Definition, 0)}
	defer func() { r.sealed = true }()
	names := make([]string, 0, len(registrations))
	for name := range registrations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		registration := registrations[name]
		if name == "" {
			r.failure = fmt.Errorf("имя регистрации не должно быть пустым")
			return nil, r.failure
		}
		if registration == nil {
			r.failure = fmt.Errorf("функция регистрации %q отсутствует", name)
			return nil, r.failure
		}
		err := registration(r)
		if r.failure == nil {
			r.failure = err
		}
		if r.failure != nil {
			return nil, fmt.Errorf("регистрация %q: %w", name, r.failure)
		}
	}
	sort.Slice(r.definitions, func(i, j int) bool {
		left, right := r.definitions[i].Code, r.definitions[j].Code
		if len(left) != len(right) {
			return len(left) < len(right)
		}
		return left < right
	})
	return r, nil
}

// Register проверяет определение, уникальность кода во всём реестре
// и добавляет копию только во время конструктора. При ошибке состав
// не меняет, но сохраняет failure;
// закрытый реестр отвергает запись без изменения уже опубликованного состояния.
func (r *Registry) Register(definition Definition) error {
	if r == nil {
		return fmt.Errorf("реестр диагностик отсутствует")
	}
	if r.sealed {
		return fmt.Errorf("реестр диагностик закрыт для регистрации")
	}
	if r.failure != nil {
		return r.failure
	}
	if r.definitions == nil {
		r.failure = fmt.Errorf("регистрация допустима только внутри NewRegistry")
		return r.failure
	}
	if err := validateDefinition(definition); err != nil {
		r.failure = fmt.Errorf("диагностика %q: %w", definition.Code, err)
		return r.failure
	}
	for _, registered := range r.definitions {
		if registered.Code == definition.Code {
			r.failure = fmt.Errorf("код диагностики %q уже зарегистрирован", definition.Code)
			return r.failure
		}
	}
	r.definitions = append(r.definitions, definition)
	return nil
}

// Len возвращает число зарегистрированных диагностик; пустой реестр имеет 0.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.definitions)
}

// ForVersion сначала вызывает dslversions.RequireSupported; при отказе
// возвращает nil и error, даже если есть VersionAll или подходящий диапазон.
// Затем возвращает новый срез только применимых описаний по номеру V-кода.
// Поддерживаемая версия без подходящих определений даёт пустой срез и nil;
// отказ от обработки такого документа формирует validator.Prepare.
// Изменение среза и его элементов не меняет реестр. Реестр должен быть закрыт;
// nil-получатель или незавершённая регистрация дают error. Check не запускает.
func (r *Registry) ForVersion(version dslversions.Version) ([]Definition, error) {
	if err := dslversions.RequireSupported(version); err != nil {
		return nil, err
	}
	if r == nil || !r.sealed || r.definitions == nil {
		return nil, fmt.Errorf("реестр диагностик не подготовлен")
	}
	if r.failure != nil {
		return nil, fmt.Errorf("реестр диагностик не создан: %w", r.failure)
	}
	definitions := make([]Definition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		if matchesVersion(definition.Versions, version) {
			definitions = append(definitions, definition)
		}
	}
	return definitions, nil
}

// validateDefinition проверяет код, уровень, область, сообщение, Check
// и Versions через validateVersionRule.
func validateDefinition(definition Definition) error {
	code := definition.Code
	if len(code) < 4 || code[0] != 'V' || code == "V000" || (len(code) > 4 && code[1] == '0') {
		return fmt.Errorf("неверный V-код %q", code)
	}
	for i := 1; i < len(code); i++ {
		if code[i] < '0' || code[i] > '9' {
			return fmt.Errorf("неверный V-код %q", code)
		}
	}
	switch definition.Severity {
	case SeverityError, SeverityWarning, SeverityRecommendation:
	default:
		return fmt.Errorf("неверный уровень диагностики %q", definition.Severity)
	}
	switch definition.Scope {
	case ScopeElement, ScopeLine, ScopeBlock, ScopeDocument:
	default:
		return fmt.Errorf("неверная область диагностики %q", definition.Scope)
	}
	if strings.TrimSpace(definition.Message) == "" {
		return fmt.Errorf("сообщение диагностики не должно быть пустым или пробельным")
	}
	if definition.Check == nil {
		return fmt.Errorf("функция проверки отсутствует")
	}
	return validateVersionRule(definition.Versions)
}

// Package diagnostics задаёт универсальный реестр, регистрацию и ограниченное
// API проверок с выбором по версии DSL. Он не импортирует model, validator
// или checkers, не содержит определений P/V-диагностик и не создаёт записей JSON.
// Интерфейсы ниже — граница доступа к строкам и приёма срабатываний;
// конкретный реестр интерфейсом не оборачивается.
package diagnostics

file values.go:

// Code — машинный код; новые проверки валидатора используют V001 и далее.
// Сам тип допускает прежние коды других инструментов для совместимости model.
type Code string

// Severity — существующий тип уровня из JSON-контракта 1.0.
type Severity string

const (
	// SeverityError — ошибка, устанавливающая hasErrors.
	SeverityError Severity = "error"
	// SeverityWarning — проблема; машинное значение warning сохраняется.
	SeverityWarning Severity = "warning"
	// SeverityRecommendation — рекомендация.
	SeverityRecommendation Severity = "recommendation"
)

// Scope — существующий тип области из JSON-контракта 1.0.
type Scope string

const (
	// ScopeElement — срабатывание на точном диапазоне существующего элемента.
	ScopeElement Scope = "element"
	// ScopeLine — срабатывание внутри одной физической строки.
	ScopeLine Scope = "line"
	// ScopeBlock — срабатывание на диапазоне, допускающем несколько строк.
	ScopeBlock Scope = "block"
	// ScopeDocument — срабатывание на документе, в том числе без позиции.
	ScopeDocument Scope = "document"
)

file definition.go:

// CheckFunc — верхнеуровневая функция одной автономной диагностики.
// Ноль срабатываний означает отсутствие проблемы. err означает технический
// сбой проверки: все её срабатывания отбрасываются, весь конвейер останавливается.
// Срабатывания любого Severity — штатный результат, а не возвращаемая ошибка.
// Первую ошибку Findings проверка обязана сразу вернуть, прекратив свою работу.
// Проверка пользуется только lines и findings, не запускает горутины,
// не выполняет I/O, не хранит состояние между файлами и не сохраняет findings.
type CheckFunc func(lines Lines, findings Findings) error

// Definition — полное описание регистрации одной диагностики.
// Содержит только значения и функцию; реестр сохраняет собственную копию.
type Definition struct {
	// Code — уникальный V-код положительного номера с минимум тремя цифрами:
	// V001..V999, затем V1000 и далее; лишние ведущие нули и V000 запрещены.
	Code Code
	// Severity — один из трёх уровней JSON-контракта.
	Severity Severity
	// Message — непустое нормативное сообщение по умолчанию.
	Message string
	// Scope — область всех срабатываний этой диагностики.
	Scope Scope
	// Versions — обязательное правило применимости Check к версии DSL.
	Versions VersionRule
	// Check — обязательная ненулевая функция проверки.
	Check CheckFunc
}

// RegistrationFunc регистрирует одну или несколько диагностик через Register.
// Не запускает Check, не выполняет I/O и не зависит от других регистраций.
type RegistrationFunc func(registry *Registry) error

file versions.go:

// VersionMode — один из четырёх способов выбора версий; нулевое значение неверно.
type VersionMode uint8

const (
	// VersionExact — только указанная версия.
	VersionExact VersionMode = iota + 1
	// VersionAll — любая версия из каталога поддерживаемых приложением.
	VersionAll
	// VersionFrom — указанная версия и более новые, включая границу.
	VersionFrom
	// VersionThrough — указанная версия и более старые, включая границу.
	VersionThrough
)

// VersionRule — одно правило применимости зарегистрированной диагностики.
// Любое правило действует только в пределах каталога dslversions.Supported.
type VersionRule struct {
	// Mode — обязательный способ сопоставления с версией документа.
	Mode VersionMode
	// Version — точная версия либо включительная граница; для VersionAll пуста.
	// Должна проходить Parse, но не обязана уже входить в каталог поддерживаемых.
	Version dslversions.Version
}

// validateVersionRule проверяет Mode и согласованность Version с режимом.
// VersionAll запрещает границу; остальные режимы требуют успешного Parse.
func validateVersionRule(rule VersionRule) error

// matchesVersion сопоставляет проверенное правило с поддерживаемой версией;
// true означает применимость, false — пропуск. Использует dslversions.Compare,
// а не строковый порядок; поддержку версии отдельно проверяет ForVersion.
func matchesVersion(rule VersionRule, version dslversions.Version) bool

file registry.go:

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
func NewRegistry(registrations map[string]RegistrationFunc) (*Registry, error)

// Register проверяет определение, уникальность кода во всём реестре
// и добавляет копию только во время конструктора. При ошибке состав
// не меняет, но сохраняет failure;
// закрытый реестр отвергает запись без изменения уже опубликованного состояния.
func (r *Registry) Register(definition Definition) error

// Len возвращает число зарегистрированных диагностик; пустой реестр имеет 0.
func (r *Registry) Len() int

// ForVersion сначала вызывает dslversions.RequireSupported; при отказе
// возвращает nil и error, даже если есть VersionAll или подходящий диапазон.
// Затем возвращает новый срез только применимых описаний по номеру V-кода.
// Поддерживаемая версия без подходящих определений даёт пустой срез и nil;
// отказ от обработки такого документа формирует validator.Prepare.
// Изменение среза и его элементов не меняет реестр. Реестр должен быть закрыт;
// nil-получатель или незавершённая регистрация дают error. Check не запускает.
func (r *Registry) ForVersion(version dslversions.Version) ([]Definition, error)

// validateDefinition проверяет код, уровень, область, сообщение, Check
// и Versions через validateVersionRule.
func validateDefinition(definition Definition) error

file lines.go:

// Lines предоставляет только построчные данные без Document, Processing,
// Diagnostics, HasErrors и ErrorIDs. Возвращаемые объекты не имеют setters
// и не раскрывают model, указатели, срезы либо карты.
type Lines interface {
	// Len возвращает число строк; пустой документ имеет длину 0.
	Len() int
	// Line получает строку по физическому номеру 1..Len.
	// found=false означает выход за границы и nil вместо строки.
	// Последовательный вызов с возрастающим или убывающим номером даёт обход
	// вперёд или назад без отдельного курсора и скрытого состояния.
	Line(number int) (line Line, found bool)
}

// Line — доступ только для чтения к содержимому и структуре одной строки.
type Line interface {
	// Number возвращает физический номер начиная с 1.
	Number() int
	// Type возвращает строковое значение существующего model.LineType.
	Type() string
	// NestingLevel возвращает логическую глубину.
	NestingLevel() int
	// ParentLine возвращает номер родителя; found=false и 0 означают корень.
	ParentLine() (number int, found bool)
	// Raw возвращает точный исходный текст строки без BOM и EOL.
	Raw() string
	// EOL возвращает исходное окончание: LF, CRLF или пустую строку.
	EOL() string
	// ElementCount возвращает число элементов строки.
	ElementCount() int
	// Element получает элемент по индексу от 0 до ElementCount исключительно.
	// found=false означает выход за границы и nil вместо элемента.
	Element(index int) (element Element, found bool)
}

// Element — доступ только для чтения к элементу, без его ссылок на ошибки.
type Element interface {
	// Type возвращает строковое значение существующего model.ElementType.
	Type() string
	// Raw возвращает точный фрагмент строки.
	Raw() string
	// Value возвращает семантическое значение; present=false означает JSON null,
	// present=true допускает пустую строку и отличается от отсутствия значения.
	Value() (text string, present bool)
	// Start возвращает включаемую Unicode-колонку начиная с 1.
	Start() int
	// End возвращает исключающую Unicode-колонку; End может равняться Start.
	End() int
}

file findings.go:

// Findings принимает только срабатывания текущего Check; прочитать прежние
// срабатывания, назначить ID, Source, Code, Severity либо Fatal через него нельзя.
// Позиции передаются числами, чтобы не копировать model.Position/Location
// и не создавать цикл model -> diagnostics -> model.
type Findings interface {
	// Add фиксирует диапазон [start,end) по строкам и Unicode-колонкам с 1.
	// Все четыре нуля означают отсутствие Location только для ScopeDocument.
	// Для ScopeElement диапазон обязан совпасть с существующим элементом;
	// для ScopeLine обе строки совпадают. message="" выбирает Definition.Message.
	// occurrence — положительный локальный номер срабатывания, не JSON ID;
	// при ошибке равен 0. Первая ошибка запрещает дальнейшее накопление и делает
	// весь Check неуспешным, даже если проверка ошибочно проигнорировала её.
	Add(startLine, startColumn, endLine, endColumn int, message string) (occurrence int, err error)
	// AddRelated добавляет связанный диапазон к ранее полученному occurrence
	// этого Check. Проверяет координаты, запрещает нулевой диапазон без позиции,
	// повтор и совпадение с основной Location. Не создаёт Element.ErrorIDs.
	AddRelated(occurrence, startLine, startColumn, endLine, endColumn int) error
}

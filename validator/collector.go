package validator

import (
	"fmt"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// collector накапливает срабатывания ровно одного синхронного вызова Check.
// Диагностике передаётся только интерфейс diagnostics.Findings.
type collector struct {
	// lines — строки очищенной проекции linesView для проверки координат;
	// исходные HasErrors/ErrorIDs отсутствуют, наружу срез не выдаётся.
	lines []model.Line
	// scope — неизменяемая область из определения текущей проверки.
	scope diagnostics.Scope
	// occurrences — предварительные записи без глобальных ID и Source.
	occurrences []model.Diagnostic
	// failure — первая ошибка API, даже если Check проигнорировала её;
	// после неё никакие новые срабатывания или связанные места не принимаются.
	failure error
	// closed — запрет любых вызовов после окончания Check.
	closed bool
}

// newCollector создаёт пустой накопитель; получает только очищенные строки
// из linesView, без доступа к исходной модели и её старым диагностикам.
func newCollector(lines []model.Line, scope diagnostics.Scope) *collector {
	return &collector{lines: lines, scope: scope, occurrences: make([]model.Diagnostic, 0)}
}

// Add проверяет диапазон и область, затем добавляет отдельное срабатывание.
// Возвращает локальный номер с 1; при ошибке возвращает 0, сохраняет failure
// и не добавляет запись. Не меняет исходную модель и не создаёт JSON ID.
// При уже установленной failure сразу возвращает её без изменения накопителя.
func (c *collector) Add(startLine, startColumn, endLine, endColumn int, message string) (occurrence int, err error) {
	if c.failure != nil {
		return 0, c.failure
	}
	if c.closed {
		c.failure = fmt.Errorf("накопитель срабатываний закрыт")
		return 0, c.failure
	}
	location, err := makeLocation(startLine, startColumn, endLine, endColumn)
	if err != nil {
		c.failure = err
		return 0, c.failure
	}
	entry := model.Diagnostic{Location: location, Message: message, RelatedLocations: make([]model.Location, 0)}
	if err := checkOccurrence(c.lines, c.scope, entry); err != nil {
		c.failure = err
		return 0, c.failure
	}
	c.occurrences = append(c.occurrences, entry)
	return len(c.occurrences), nil
}

// AddRelated проверяет существование локального occurrence в этом накопителе,
// координаты и отсутствие дубликата, затем дополняет только эту запись.
// При отказе сохраняет failure; вызов на закрытом накопителе также запрещён.
// Установленную ранее failure не заменяет и новых связанных мест не добавляет.
func (c *collector) AddRelated(occurrence, startLine, startColumn, endLine, endColumn int) error {
	if c.failure != nil {
		return c.failure
	}
	if c.closed {
		c.failure = fmt.Errorf("накопитель срабатываний закрыт")
		return c.failure
	}
	if occurrence < 1 || occurrence > len(c.occurrences) {
		c.failure = fmt.Errorf("локальное срабатывание %d не существует", occurrence)
		return c.failure
	}
	location, err := makeLocation(startLine, startColumn, endLine, endColumn)
	if err != nil {
		c.failure = err
		return c.failure
	}
	if location == nil {
		c.failure = fmt.Errorf("связанный диапазон требует позицию")
		return c.failure
	}
	entry := c.occurrences[occurrence-1]
	related := make([]model.Location, len(entry.RelatedLocations)+1)
	copy(related, entry.RelatedLocations)
	related[len(entry.RelatedLocations)] = *location
	entry.RelatedLocations = related
	if err := checkOccurrence(c.lines, c.scope, entry); err != nil {
		c.failure = err
		return c.failure
	}
	c.occurrences[occurrence-1] = entry
	return nil
}

// finish закрывает накопитель и возвращает глубокую копию записей либо
// nil и накопленную ошибку. Повторный вызов считается ошибкой API.
func (c *collector) finish() ([]model.Diagnostic, error) {
	if c.closed && c.failure == nil {
		c.failure = fmt.Errorf("накопитель срабатываний уже завершён")
	}
	c.closed = true
	if c.failure != nil {
		return nil, c.failure
	}
	entries := make([]model.Diagnostic, len(c.occurrences))
	for i, entry := range c.occurrences {
		entries[i] = entry
		if entry.Location != nil {
			location := *entry.Location
			entries[i].Location = &location
		}
		if entry.RelatedLocations != nil {
			entries[i].RelatedLocations = make([]model.Location, len(entry.RelatedLocations))
			copy(entries[i].RelatedLocations, entry.RelatedLocations)
		}
	}
	return entries, nil
}

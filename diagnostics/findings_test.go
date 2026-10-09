package diagnostics

type findingsContract interface {
	Add(startLine, startColumn, endLine, endColumn int, message string) (occurrence int, err error)
	AddRelated(occurrence, startLine, startColumn, endLine, endColumn int) error
}

// Присваиваемость в обе стороны проверяет точный набор методов и сигнатуры:
// удаление, добавление или изменение метода приводит к ошибке компиляции.
var (
	_ findingsContract = (Findings)(nil)
	_ Findings         = (findingsContract)(nil)
)

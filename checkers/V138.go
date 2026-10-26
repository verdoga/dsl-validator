package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV138 регистрирует проверку пустых значений в блочном answer для DSL 1.2.
func registrationV138(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V138",
		Severity: diagnostics.SeverityError,
		Message:  "В блоке @answer указано пустое значение ответа.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV138,
	})
}

// checkV138 проверяет непосредственные content-строки каждого answer независимо.
// Строки blank игнорируются; конкретизация не добавляется.
func checkV138(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "content" {
			continue
		}
		parentNumber, found := line.ParentLine()
		if !found {
			continue
		}
		parent, _ := lines.Line(parentNumber)
		if !v138Answer(parent) {
			continue
		}
		value, known := v138Text(line)
		if !known || answerPrimary(value) != "" {
			continue
		}
		startLine, startColumn, endLine, endColumn := number, 1, number+1, 1
		if number == lines.Len() {
			// У собственной строки блока обязательно есть предшествующая строка.
			previous, _ := lines.Line(number - 1)
			startLine, startColumn = number-1, utf8.RuneCountInString(previous.Raw())+1
			endLine, endColumn = number, utf8.RuneCountInString(line.Raw())+1
		}
		if _, err := findings.Add(startLine, startColumn, endLine, endColumn, ""); err != nil {
			return err
		}
	}
	return nil
}

// v138Answer возвращает true только для распознанного блочного answer.
func v138Answer(line diagnostics.Line) bool {
	if line.Type() != "block-start" {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "answer"
		}
	}
	return false
}

// v138Text получает текст одной строки, не снимая экранирование повторно.
// true означает известное значение; значимые непокрытые участки Raw и
// отсутствие разбора не доказывают пустоту. Пробельные промежутки сохраняются.
func v138Text(line diagnostics.Line) (string, bool) {
	raw := []rune(line.Raw())
	end := 0
	var text strings.Builder
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		value, present := element.Value()
		if element.Type() != "content" || !present {
			return "", false
		}
		// Нулевой content не покрывает Raw и не сдвигает известную границу.
		if element.Start() != element.End() {
			if start := element.Start() - 1; start > end {
				gap := string(raw[end:start])
				if answerPrimary(gap) != "" {
					return "", false
				}
				text.WriteString(gap)
			}
			end = element.End() - 1
		}
		text.WriteString(value)
	}
	tail := string(raw[end:])
	if answerPrimary(tail) != "" {
		return "", false
	}
	text.WriteString(tail)
	return text.String(), line.ElementCount() != 0
}

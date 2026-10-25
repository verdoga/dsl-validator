package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV138 регистрирует запрет пустых строк в блочном answer для DSL 1.2.
func registrationV138(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V138",
		Severity: diagnostics.SeverityError,
		Message:  "В блоке @answer есть пустая строка.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV138,
	})
}

// checkV138 проверяет собственные текстовые строки каждого answer независимо.
// Соседние пустые строки не объединяются, конкретизация не добавляется.
func checkV138(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "blank" && line.Type() != "content" {
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
		if !known || v138Normalize(value) != "" {
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
// true означает известное значение; отсутствие разбора не доказывает пустоту.
// Обычная blank может не иметь элементов: её Raw содержит только пробелы и табы.
func v138Text(line diagnostics.Line) (string, bool) {
	if line.Type() == "blank" {
		return line.Raw(), true
	}
	var text strings.Builder
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		value, present := element.Value()
		if element.Type() != "content" || !present {
			return "", false
		}
		text.WriteString(value)
	}
	return text.String(), line.ElementCount() != 0
}

// v138Normalize сжимает Unicode White_Space до U+0020 и убирает краевые пробелы.
// Пунктуация, регистр и остальные символы сохраняются без изменений.
func v138Normalize(value string) string {
	return strings.Join(strings.FieldsFunc(value, func(char rune) bool {
		return unicode.Is(unicode.White_Space, char)
	}), " ")
}

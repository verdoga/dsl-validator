package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV007 регистрирует предупреждение о непарных круглых скобках для DSL 1.2.
func registrationV007(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V007",
		Severity: diagnostics.SeverityWarning,
		Message:  "Нарушена парность круглых скобок в строке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV007,
	})
}

// checkV007 сопоставляет U+0028 и U+0029 в точном Raw каждой строки отдельно.
// Кавычки и обратная косая черта не скрывают скобки. Каждый непарный символ
// отмечается диапазоном в Unicode-колонках с 1 с исключающей правой границей.
// Закрывающая скобка начального маркера списка не участвует в сопоставлении.
func checkV007(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		marker := v007ListMarker(line)
		var openings []int
		column := 1
		for _, char := range line.Raw() {
			switch char {
			case '(':
				openings = append(openings, column)
			case ')':
				if column == marker {
					break
				}
				if len(openings) > 0 {
					openings = openings[:len(openings)-1]
				} else if _, err := findings.Add(number, column, number, column+1,
					"Закрывающая круглая скобка не имеет открывающей в этой строке"); err != nil {
					return err
				}
			}
			column++
		}
		for _, opening := range openings {
			if _, err := findings.Add(number, opening, number, opening+1,
				"Открывающая круглая скобка не закрыта в этой строке"); err != nil {
				return err
			}
		}
	}
	return nil
}

// v007ListMarker возвращает Unicode-колонку скобки начального маркера списка
// в первом content либо 0. Маркер — число или одна буква, затем ')' и пробел
// либо табуляция перед непустым текстом. Служебные значения не распознаются.
func v007ListMarker(line diagnostics.Line) int {
	if line.Type() != "content" && line.Type() != "tag" {
		return 0
	}
	raw := []rune(line.Raw())
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		start, end := element.Start()-1, element.End()-1
		if line.Type() == "content" && strings.Trim(string(raw[:start]), " \t") != "" {
			return 0
		}
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		if start == end {
			return 0
		}
		if unicode.IsLetter(raw[start]) {
			start++
		} else if unicode.IsDigit(raw[start]) {
			for start < end && unicode.IsDigit(raw[start]) {
				start++
			}
		} else {
			return 0
		}
		if start == end || raw[start] != ')' {
			return 0
		}
		text := start + 1
		for text < len(raw) && (raw[text] == ' ' || raw[text] == '\t') {
			text++
		}
		if text == start+1 || text == len(raw) {
			return 0
		}
		return start + 1
	}
	return 0
}

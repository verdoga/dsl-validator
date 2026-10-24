package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV096 регистрирует запрет заголовков внутри table только для DSL 1.2.
func registrationV096(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V096",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри @table записан заголовок.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV096,
	})
}

// checkV096 проверяет исходное начало непосредственных строк table независимо
// от классификации строки и полноты разбора её элементов.
func checkV096(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		block, _ := lines.Line(parent)
		if !v096Table(block) {
			continue
		}
		start, end := v096Marker(line.Raw())
		if end == 0 {
			continue
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v096Table сообщает, является ли строка открытием блока с каноническим тегом table.
func v096Table(line diagnostics.Line) bool {
	if line.Type() != "block-start" {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "table"
		}
	}
	return false
}

// v096Marker возвращает диапазон начальных решёток в Unicode-колонках с 1,
// правая граница исключается. Нулевой диапазон означает отсутствие маркера.
// Пропускаются только U+0020/U+0009; экранирование не снимается.
func v096Marker(raw string) (start, end int) {
	runes := []rune(raw)
	for start < len(runes) && (runes[start] == ' ' || runes[start] == '\t') {
		start++
	}
	end = start
	for end < len(runes) && runes[end] == '#' {
		end++
	}
	if end == start {
		return 0, 0
	}
	return start + 1, end + 1
}

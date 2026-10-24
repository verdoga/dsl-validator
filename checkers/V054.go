package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV054 регистрирует повтор первого уровня заголовка только для DSL 1.2.
func registrationV054(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V054",
		Severity: diagnostics.SeverityError,
		Message:  "Заголовок первого уровня объявлен повторно.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV054,
	})
}

// checkV054 учитывает одиночный начальный # в Raw, включая ошибочные объявления.
// Первый маркер общий для всего документа; названия и границы блоков его не меняют.
// Диапазоны охватывают один # в исходных Unicode-колонках с исключающим концом.
func checkV054(lines diagnostics.Lines, findings diagnostics.Findings) error {
	firstLine, firstColumn := 0, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := line.Raw()
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		if start == len(raw) || raw[start] != '#' || start+1 < len(raw) && raw[start+1] == '#' {
			continue
		}
		if v054LiteralContent(lines, line) {
			continue
		}
		// Перед # прошли только однобайтовые U+0020/U+0009, поэтому
		// индекс одновременно задаёт смещение в кодовых точках Unicode.
		column := start + 1
		if firstLine == 0 {
			firstLine, firstColumn = number, column
			continue
		}
		occurrence, err := findings.Add(number, column, number, column+1, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, firstLine, firstColumn, firstLine, firstColumn+1); err != nil {
			return err
		}
	}
	return nil
}

// v054LiteralContent возвращает true для содержимого блочного editor/multifill,
// false — при отсутствии такого предка. Использует только структуру готовой модели.
func v054LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV051 регистрирует повторный заголовок второго уровня для DSL 1.2.
func registrationV051(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V051",
		Severity: diagnostics.SeverityError,
		Message:  "Заголовок второго уровня объявлен повторно.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV051,
	})
}

// checkV051 связывает каждый повторный начальный маркер ## с первым в документе.
// Raw не декодируется; пропускаются только начальные U+0020/U+0009.
// Диапазоны охватывают два # в исходных Unicode-колонках с единицы,
// правая граница исключается. Название и классификация строки не учитываются.
func checkV051(lines diagnostics.Lines, findings diagnostics.Findings) error {
	firstLine, firstColumn := 0, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v051LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		end := start
		for end < len(raw) && raw[end] == '#' {
			end++
		}
		if end-start != 2 {
			continue
		}
		if firstLine == 0 {
			firstLine, firstColumn = number, start+1
			continue
		}
		occurrence, err := findings.Add(number, start+1, number, end+1, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, firstLine, firstColumn, firstLine, firstColumn+2); err != nil {
			return err
		}
	}
	return nil
}

// v051LiteralContent возвращает true для потомка блочного editor или multifill;
// false означает отсутствие такого предка в готовой модели.
func v051LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			tag, _ := line.Element(index)
			if tag.Type() != "tag" {
				continue
			}
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
}

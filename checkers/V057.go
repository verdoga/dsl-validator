package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV057 регистрирует проверку разделителя заголовка только для DSL 1.2.
func registrationV057(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V057",
		Severity: diagnostics.SeverityError,
		Message:  "Между маркером заголовка и текстом должен стоять ровно один обычный пробел.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV057,
	})
}

// checkV057 проверяет Raw, включая ошибочные и вложенные попытки заголовков.
// Отступы пропускаются без изменения исходника; экранирование не снимается.
// Колонки считаются в кодовых точках Unicode с 1, правая граница исключается.
func checkV057(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v057LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		markerStart := 0
		for markerStart < len(raw) && (raw[markerStart] == ' ' || raw[markerStart] == '\t') {
			markerStart++
		}
		markerEnd := markerStart
		for markerEnd < len(raw) && raw[markerEnd] == '#' {
			markerEnd++
		}
		level := markerEnd - markerStart
		if level < 1 || level > 3 {
			continue
		}
		textStart := markerEnd
		for textStart < len(raw) && unicode.Is(unicode.White_Space, raw[textStart]) {
			textStart++
		}
		// Отсутствующее название проверяется отдельно, независимо от разделителя.
		if textStart == len(raw) || textStart == markerEnd+1 && raw[markerEnd] == ' ' {
			continue
		}
		start, end, detail := markerEnd, textStart, ""
		if start == end {
			start, end = markerStart, markerEnd
			detail = "После маркера отсутствует пробел"
		}
		if _, err := findings.Add(number, start+1, number, end+1, detail); err != nil {
			return err
		}
	}
	return nil
}

// v057LiteralContent возвращает true для потомков блочного editor или multifill.
// false означает отсутствие такого предка; другие блоки не исключают проверку.
func v057LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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

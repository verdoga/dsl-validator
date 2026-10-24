package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV058 регистрирует отсутствие названия заголовка только для DSL 1.2.
func registrationV058(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V058",
		Severity: diagnostics.SeverityError,
		Message:  "У заголовка отсутствует название.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV058,
	})
}

// checkV058 проверяет весь исходный остаток после начального маркера длиной 1–3.
// Разбор элементов и ошибки разделителя не определяют наличие названия.
// Диапазон охватывает маркер в исходных Unicode-колонках; конец исключается.
func checkV058(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := line.Raw()
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		end := start
		for end < len(raw) && raw[end] == '#' {
			end++
		}
		if end == start || end-start > 3 || v058LiteralContent(lines, line) {
			continue
		}
		hasTitle := false
		for _, char := range raw[end:] {
			if !unicode.Is(unicode.White_Space, char) {
				hasTitle = true
				break
			}
		}
		if hasTitle {
			continue
		}
		// Отступ и маркер состоят только из ASCII: байтовые смещения
		// до конца маркера совпадают со смещениями в кодовых точках.
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v058LiteralContent возвращает true для потомка блочного editor/multifill;
// false означает отсутствие такого предка в готовой структуре документа.
func v058LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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

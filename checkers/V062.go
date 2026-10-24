package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV062 регистрирует отсутствие названия @header только для DSL 1.2.
func registrationV062(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V062",
		Severity: diagnostics.SeverityError,
		Message:  "У @header отсутствует название.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV062,
	})
}

// checkV062 проверяет исходный остаток после распознанного header; при отсутствии
// элемента ищет точное имя в начале Raw. Найденные элементы повторно не проверяются.
func checkV062(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v062LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		if start < len(raw) && raw[start] == '\\' {
			continue
		}
		found := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			name, _ := element.Value()
			if element.Type() != "tag" || name != "header" {
				continue
			}
			found = true
			if err := v062ReportEmpty(findings, raw, number, element.Start(), element.End()); err != nil {
				return err
			}
		}
		if found {
			continue
		}
		if end, ok := v062RawHeader(raw, start); ok {
			if err := v062ReportEmpty(findings, raw, number, start+1, end+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v062RawHeader возвращает индекс сразу после @header и true при точном имени
// с границей White_Space/конец строки; false означает отсутствие объявления.
// start и возвращаемый индекс измеряются в кодовых точках с нуля.
func v062RawHeader(raw []rune, start int) (int, bool) {
	end := start + len("@header")
	if end > len(raw) || !strings.EqualFold(string(raw[start:end]), "@header") {
		return 0, false
	}
	if end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) {
		return 0, false
	}
	return end, true
}

// v062ReportEmpty отмечает имя, если весь исходный остаток состоит из White_Space.
// start/end — исходные Unicode-колонки с 1, конец исключается.
func v062ReportEmpty(findings diagnostics.Findings, raw []rune, number, start, end int) error {
	for _, char := range raw[end-1:] {
		if !unicode.Is(unicode.White_Space, char) {
			return nil
		}
	}
	_, err := findings.Add(number, start, number, end, "")
	return err
}

// v062LiteralContent возвращает true для потомка блочного editor/multifill;
// false означает отсутствие такого предка в готовой структуре документа.
func v062LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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

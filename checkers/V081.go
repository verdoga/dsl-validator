package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV081 регистрирует проверку разделённого источника медиа DSL 1.2.
func registrationV081(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V081",
		Severity: diagnostics.SeverityError,
		Message:  "Источник медиа записан несколькими частями без общих кавычек.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV081,
	})
}

// checkV081 самостоятельно читает тип и весь исходный остаток, включая unparsed.
// Диапазон источника исключает только крайние U+0020/U+0009; позиции в Unicode-
// колонках с 1, правая граница исключается. Исходник и значения не изменяются.
func checkV081(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v081Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "media" || v081LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := tag.End() - 1
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		end := start
		for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' {
			end++
		}
		kind := string(raw[start:end])
		if !strings.EqualFold(kind, "audio") && !strings.EqualFold(kind, "video") && !strings.EqualFold(kind, "image") {
			continue
		}
		start, end = end, len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end || raw[start] == '"' || !v081Separated(raw[start:end]) {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1,
			"Один путь с пробелами заключается в общие двойные кавычки; несколько ресурсов записываются отдельными тегами"); err != nil {
			return err
		}
	}
	return nil
}

// v081Separated возвращает true при двух непустых частях через White_Space.
// Пробелы только до первой части или после последней дают false.
func v081Separated(source []rune) bool {
	part, separator := false, false
	for _, char := range source {
		if unicode.Is(unicode.White_Space, char) {
			separator = part
		} else {
			if separator {
				return true
			}
			part = true
		}
	}
	return false
}

// v081Tag возвращает разобранный тег или nil для строки без объявления.
func v081Tag(line diagnostics.Line) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return element
		}
	}
	return nil
}

// v081LiteralContent возвращает true внутри блочного editor/multifill;
// false означает отсутствие такого предка в готовой структуре документа.
func v081LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if tag := v081Tag(line); tag != nil {
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

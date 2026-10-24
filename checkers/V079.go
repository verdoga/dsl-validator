package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV079 регистрирует проверку исходного типа медиаресурса DSL 1.2.
func registrationV079(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V079",
		Severity: diagnostics.SeverityError,
		Message:  "Тип ресурса отличается от audio, video и image.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV079,
	})
}

// checkV079 читает первый аргумент из Raw, включая unparsed. Наличие и Value
// элемента media-type не ограничивают проверку. Диапазон исходный, в Unicode-
// колонках с 1, правая граница исключается; SOURCE не исследуется.
func checkV079(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v079Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "media" || v079LiteralContent(lines, line) {
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
		if start == end {
			continue
		}
		argument := string(raw[start:end])
		if strings.EqualFold(argument, "audio") || strings.EqualFold(argument, "video") || strings.EqualFold(argument, "image") {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, "Допустимые значения: audio, video, image."); err != nil {
			return err
		}
	}
	return nil
}

// v079Tag возвращает разобранное имя тега или nil для строки без объявления.
func v079Tag(line diagnostics.Line) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return element
		}
	}
	return nil
}

// v079LiteralContent возвращает true внутри блочного editor или multifill;
// false означает отсутствие литерального предка в готовой структуре строк.
func v079LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if tag := v079Tag(line); tag != nil {
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

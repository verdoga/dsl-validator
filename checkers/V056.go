package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV056 регистрирует предупреждение о комментариях editor в DSL 1.2.
func registrationV056(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V056",
		Severity: diagnostics.SeverityWarning,
		Message:  "В документе есть редакторский комментарий @editor.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV056,
	})
}

// checkV056 отмечает только имена объявлений editor в исходных координатах.
// Одно объявление не дублируется резервным поиском по Raw.
func checkV056(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v056LiteralContent(lines, line) {
			continue
		}
		found := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			name, _ := element.Value()
			if element.Type() != "tag" || name != "editor" {
				continue
			}
			found = true
			if _, err := findings.Add(number, element.Start(), number, element.End(), ""); err != nil {
				return err
			}
		}
		if found {
			continue
		}
		if start, end, ok := v056RawTag(line, "editor"); ok {
			if _, err := findings.Add(number, start, number, end, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v056RawTag ищет имя только в invalid либо строке с unparsed.
// Возвращает исходные Unicode-колонки с 1, конец исключается; false означает
// отсутствие кандидата. После имени допускаются Unicode-пробел, { или конец.
func v056RawTag(line diagnostics.Line, name string) (start, end int, found bool) {
	erroneous := line.Type() == "invalid"
	for index := 0; !erroneous && index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		erroneous = element.Type() == "unparsed"
	}
	if !erroneous {
		return 0, 0, false
	}
	raw := []rune(line.Raw())
	first := 0
	for first < len(raw) && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	// Имена editor и multifill состоят только из ASCII.
	last := first + 1 + len(name)
	if last > len(raw) || !strings.EqualFold(string(raw[first:last]), "@"+name) {
		return 0, 0, false
	}
	if last < len(raw) && raw[last] != '{' && !unicode.IsSpace(raw[last]) {
		return 0, 0, false
	}
	return first + 1, last + 1, true
}

// v056LiteralContent возвращает true внутри блочного editor или multifill.
// false означает отсутствие литерального предка; собственное открытие блока
// не исключается. Тип блока и связи берутся только из готовой модели.
func v056LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		hasTag := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			hasTag = true
			name, _ := element.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
		if !hasTag {
			if _, _, found := v056RawTag(line, "editor"); found {
				return true
			}
			if _, _, found := v056RawTag(line, "multifill"); found {
				return true
			}
		}
	}
}

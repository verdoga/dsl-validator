package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV186 регистрирует запрет заголовков внутри ordering для DSL 1.2.
func registrationV186(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V186",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри @ordering записан заголовок.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV186,
	})
}

// checkV186 отмечает каждый непосредственный заголовок отдельно.
func checkV186(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		start, end, heading := v186Heading(line)
		if !heading {
			continue
		}
		parent, tag := v186Ordering(lines, line)
		if tag == nil {
			continue
		}
		occurrence, err := findings.Add(number, start, number, end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, parent, tag.Start(), parent, tag.End()); err != nil {
			return err
		}
	}
	return nil
}

// v186Heading возвращает исходный диапазон [start,end) в Unicode-колонках.
// heading=false означает отсутствие заголовка уровней 1–3. Восстановление
// без heading-level разрешено только для invalid; текст не декодируется.
func v186Heading(line diagnostics.Line) (start, end int, heading bool) {
	raw := []rune(line.Raw())
	first := 0
	for first < len(raw) && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	after := first
	for after < len(raw) && raw[after] == '#' {
		after++
	}
	if after-first < 1 || after-first > 3 {
		return 0, 0, false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "heading-level" && element.Start() == first+1 && element.End() == after+1 {
			heading = true
			break
		}
	}
	if !heading {
		if line.Type() != "invalid" || after == len(raw) || !unicode.Is(unicode.White_Space, raw[after]) {
			return 0, 0, false
		}
	}
	last := len(raw)
	for last > after && (raw[last-1] == ' ' || raw[last-1] == '\t') {
		last--
	}
	return first + 1, last + 1, true
}

// v186Ordering возвращает тег ближайшего фигурного контейнера, если это ordering.
// Логические родители пропускаются, через другой фигурный блок поиск не идёт.
func v186Ordering(lines diagnostics.Lines, line diagnostics.Line) (int, diagnostics.Element) {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				name, _ := element.Value()
				if strings.EqualFold(name, "ordering") {
					return parent, element
				}
			}
		}
		return 0, nil
	}
	return 0, nil
}

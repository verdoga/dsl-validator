package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV193 регистрирует вложение variants в ближайший внешний variants.
func registrationV193(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V193", Severity: diagnostics.SeverityError,
		Message:  "Блок @variants вложен в другой @variants.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV193,
	})
}

func checkV193(lines diagnostics.Lines, findings diagnostics.Findings) error {
	closed := make(map[int]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-end" {
			if parent, found := line.ParentLine(); found {
				closed[parent] = true
			}
			continue
		}
		if line.Type() != "block-start" {
			continue
		}
		start, end, found := v193TagRange(line)
		if !found {
			continue
		}
		outer := v193Outer(lines, line, closed)
		if outer == nil {
			continue
		}
		outerStart, outerEnd, found := v193TagRange(outer)
		if !found {
			continue
		}
		occurrence, err := findings.Add(number, start, number, end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, outer.Number(), outerStart, outer.Number(), outerEnd); err != nil {
			return err
		}
	}
	return nil
}

// v193Outer возвращает ближайшего открытого фигурного родителя либо nil.
// Логические области не создают фигурной границы. Через другой или уже
// закрытый блок поиск внешнего variants не продолжается.
func v193Outer(lines diagnostics.Lines, line diagnostics.Line, closed map[int]bool) diagnostics.Line {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if closed[parent] {
			return nil
		}
		return line
	}
	return nil
}

// v193TagRange возвращает исходные Unicode-колонки [start,end) имени @variants.
// При отсутствии tag восстанавливает только точное начальное имя из Raw;
// экранированная запись, другое имя или неоднозначные элементы дают false.
func v193TagRange(line diagnostics.Line) (start, end int, found bool) {
	var tag diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			if tag != nil {
				return 0, 0, false
			}
			tag = element
		}
	}
	if tag != nil {
		if strings.EqualFold(tag.Raw(), "@variants") {
			return tag.Start(), tag.End(), true
		}
		return 0, 0, false
	}
	raw := []rune(line.Raw())
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	end = start + len("@variants")
	if end > len(raw) || !strings.EqualFold(string(raw[start:end]), "@variants") {
		return 0, 0, false
	}
	if end < len(raw) && !unicode.IsSpace(raw[end]) && raw[end] != '{' {
		return 0, 0, false
	}
	return start + 1, end + 1, true
}

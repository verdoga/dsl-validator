package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV162 регистрирует проверку наличия разделителя в matching.
func registrationV162(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V162", Severity: diagnostics.SeverityError,
		Message:  "В @matching отсутствует разделитель списков ---.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV162,
	})
}

// checkV162 использует границы готовой модели, не восстанавливая стек парсера.
// Отсутствующее или неоднозначное закрытие исключает блок из этой проверки.
func checkV162(lines diagnostics.Lines, findings diagnostics.Findings) error {
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-end" {
			continue
		}
		if parent, found := line.ParentLine(); found {
			if _, exists := ends[parent]; exists {
				ends[parent] = -1
			} else {
				ends[parent] = number
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if opening.Type() != "block-start" || v162LiteralParent(lines, opening) {
			continue
		}
		tag := v162Element(opening, "tag")
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		end := ends[number]
		if !strings.EqualFold(name, "matching") || end <= number || v162Element(opening, "block-open") == nil {
			continue
		}
		closing, _ := lines.Line(end)
		if v162Element(closing, "block-close") == nil {
			continue
		}
		separators := 0
		for next := number + 1; next < end; next++ {
			line, _ := lines.Line(next)
			parent, _ := line.ParentLine()
			if parent == number && strings.Trim(line.Raw(), " \t") == "---" {
				separators++
			}
		}
		if separators == 0 {
			if _, err := findings.Add(number, tag.Start(), number, tag.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v162Element возвращает единственный элемент заданного типа.
// nil означает отсутствие или неоднозначность, в том числе границы блока.
func v162Element(line diagnostics.Line, kind string) diagnostics.Element {
	var result diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if result != nil {
				return nil
			}
			result = element
		}
	}
	return result
}

// v162LiteralParent исключает псевдообъявления в editor и multifill.
func v162LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if tag := v162Element(line, "tag"); tag != nil {
			name, _ := tag.Value()
			if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
				return true
			}
		}
	}
	return false
}

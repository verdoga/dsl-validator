package checkers

import (
	"fmt"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV176 регистрирует минимум распределяемых элементов ordering.
func registrationV176(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V176", Severity: diagnostics.SeverityError,
		Message:  "В @ordering меньше двух распределяемых элементов.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV176,
	})
}

// checkV176 отмечает tag в исходных Unicode-колонках [Start,End).
// Каждая обычная строка считается отдельно, независимо от значений соседей.
func checkV176(lines diagnostics.Lines, findings diagnostics.Findings) error {
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-end" {
			if parent, found := line.ParentLine(); found {
				if _, exists := ends[parent]; exists {
					ends[parent] = -1
				} else {
					ends[parent] = number
				}
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		tag := v176Element(block, "tag")
		if block.Type() != "block-start" || tag == nil {
			continue
		}
		name, _ := tag.Value()
		if !strings.EqualFold(name, "ordering") || !v176Bounds(lines, block, ends) {
			continue
		}
		var children []diagnostics.Line
		separators, limit := 0, ends[number]
		for next := number + 1; next < ends[number]; next++ {
			line, _ := lines.Line(next)
			parent, _ := line.ParentLine()
			if parent != number {
				continue
			}
			children = append(children, line)
			if strings.Trim(line.Raw(), " \t") == "---" {
				separators++
				limit = next
			}
		}
		if separators > 1 {
			continue
		}
		count, known := 0, true
		for _, line := range children {
			if line.Number() >= limit {
				break
			}
			filled, definite := v176Ordinary(line)
			known = known && definite
			if filled {
				count++
			}
		}
		if !known || count >= 2 {
			continue
		}
		if _, err := findings.Add(number, tag.Start(), number, tag.End(),
			fmt.Sprintf("Найдено распределяемых элементов: %d.", count)); err != nil {
			return err
		}
	}
	return nil
}

// v176Element возвращает единственный элемент типа либо nil при неоднозначности.
func v176Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}

// v176End проверяет известную пару границ; ноль означает отсутствие единственной
// пары. Дерево не восстанавливается: используется ParentLine закрывающей строки.
func v176End(lines diagnostics.Lines, block diagnostics.Line, ends map[int]int) int {
	end := ends[block.Number()]
	if end <= block.Number() || v176Element(block, "block-open") == nil {
		return 0
	}
	closing, _ := lines.Line(end)
	if v176Element(closing, "block-close") == nil || strings.Trim(closing.Raw(), " \t") != "}" {
		return 0
	}
	return end
}

// v176Bounds требует замкнутую область с непротиворечивыми родителями и границами
// вложенных блоков. Ошибки значений внутри исключённых областей не мешают подсчёту.
func v176Bounds(lines diagnostics.Lines, block diagnostics.Line, ends map[int]int) bool {
	end := v176End(lines, block, ends)
	if end == 0 {
		return false
	}
	for number := block.Number() + 1; number < end; number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-start" {
			if nestedEnd := v176End(lines, line, ends); nestedEnd == 0 || nestedEnd >= end {
				return false
			}
		}
		ancestor := line
		for {
			parent, found := ancestor.ParentLine()
			if !found || parent < block.Number() {
				return false
			}
			if parent == block.Number() {
				break
			}
			ancestor, _ = lines.Line(parent)
			if ancestor.Type() == "block-start" && (ends[parent] < number || ends[parent] >= end) {
				return false
			}
		}
	}
	return true
}

// v176Ordinary возвращает непустоту обычной строки и определимость результата.
// Объявления, заголовки и границы не являются элементами даже при ошибке в них.
func v176Ordinary(line diagnostics.Line) (filled, known bool) {
	raw := strings.Trim(line.Raw(), " \t")
	if raw == "" {
		return false, true
	}
	switch line.Type() {
	case "tag", "block-start", "block-end", "heading":
		return false, true
	}
	if strings.HasPrefix(raw, "@") || strings.HasPrefix(raw, "#") {
		return false, true
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" || element.Type() == "heading-level" {
			return false, true
		}
	}
	if line.Type() != "content" {
		return false, false
	}
	runes := []rune(line.Raw())
	cursor, contents := 0, 0
	complete := true
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if strings.Trim(string(runes[cursor:element.Start()-1]), " \t") != "" {
			complete = false
		}
		cursor = element.End() - 1
		if element.Type() != "content" {
			complete = false
			continue
		}
		contents++
		value, present := element.Value()
		complete = complete && present
		filled = filled || (present && value != "")
	}
	// Известный непустой фрагмент уже доказывает один элемент этой обычной строки.
	if filled {
		return true, true
	}
	if contents != 0 {
		return false, complete && strings.Trim(string(runes[cursor:]), " \t") == ""
	}
	if line.ElementCount() != 0 {
		return false, false
	}
	value, ok := v176Text(raw)
	return value != "" && ok, ok
}

// v176Text снимает только разрешённое в строке ordering экранирование по 1.21–1.22.
// Готовый Value сюда не передаётся. false означает неоднозначную сырую разметку.
func v176Text(raw string) (string, bool) {
	if raw == `\---` {
		return "---", true
	}
	runes := []rune(raw)
	var text strings.Builder
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' && index+1 < len(runes) {
			next := runes[index+1]
			if next == '\\' || next == '{' || next == '}' || (index == 0 && (next == '@' || next == '#')) {
				text.WriteRune(next)
				index++
				continue
			}
		}
		if char == '{' || char == '}' {
			return "", false
		}
		text.WriteRune(char)
	}
	return text.String(), true
}

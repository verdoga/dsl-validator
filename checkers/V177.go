package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV177 регистрирует минимум позиций нижнего списка ordering.
func registrationV177(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V177",
		Severity: diagnostics.SeverityError,
		Message:  "В нижнем списке @ordering меньше двух позиций.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV177,
	})
}

// checkV177 использует родительские связи модели, не восстанавливая вложенность.
// Неоднозначное закрытие или число разделителей исключает блок из проверки.
func checkV177(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]int)
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		children[parent] = append(children[parent], number)
		if line.Type() == "block-end" {
			if _, exists := ends[parent]; exists {
				ends[parent] = -1
			} else {
				ends[parent] = number
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		tag := v177Element(opening, "tag")
		if opening.Type() != "block-start" || tag == nil || v177Element(opening, "block-open") == nil {
			continue
		}
		name, _ := tag.Value()
		end := ends[number]
		if !strings.EqualFold(name, "ordering") || end <= number {
			continue
		}
		closing, _ := lines.Line(end)
		brace := v177Element(closing, "block-close")
		if brace == nil {
			continue
		}
		separator, separators := 0, 0
		for _, child := range children[number] {
			if child >= end {
				break
			}
			line, _ := lines.Line(child)
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator, separators = child, separators+1
			}
		}
		if separators != 1 {
			continue
		}
		count, known := v177CountPositions(lines, children[number], separator, end)
		if !known || count >= 2 {
			continue
		}
		line, _ := lines.Line(separator)
		// Отступ содержит только однобайтовые U+0020/U+0009; конец исключается.
		start := len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t")) + 1
		if _, err := findings.Add(separator, start, end, brace.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v177CountPositions считает строки, а не уникальные подписи. Второй результат
// false означает, что неразобранная строка не позволяет установить состав списка.
func v177CountPositions(lines diagnostics.Lines, children []int, separator, closing int) (int, bool) {
	count := 0
	for _, number := range children {
		if number <= separator {
			continue
		}
		if number >= closing {
			break
		}
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "" || line.Type() == "blank" ||
			line.Type() == "block-start" || line.Type() == "block-end" || line.Type() == "heading" ||
			v177Element(line, "tag") != nil || v177Element(line, "heading-level") != nil {
			continue
		}
		if line.Type() != "content" {
			return 0, false
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "unparsed" {
				return 0, false
			}
		}
		count++
	}
	return count, true
}

// v177Element возвращает единственный элемент заданного типа; nil означает
// отсутствие либо неоднозначность распознанного объявления или границы.
func v177Element(line diagnostics.Line, kind string) diagnostics.Element {
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

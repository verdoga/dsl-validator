package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV089 регистрирует проверку содержимого после разделителя блока.
func registrationV089(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V089",
		Severity: diagnostics.SeverityError,
		Message:  "После разделителя --- отсутствует содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV089,
	})
}

// checkV089 использует готовые связи строк, не восстанавливая вложенность по Raw.
// Проверяется только часть после единственного непосредственного разделителя.
func checkV089(lines diagnostics.Lines, findings diagnostics.Findings) error {
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
		if opening.Type() != "block-start" {
			continue
		}
		tag := v089Element(opening, "tag")
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "example" && name != "table" {
			continue
		}
		end := ends[number]
		if end <= number || v089Element(opening, "block-open") == nil {
			continue
		}
		closing, _ := lines.Line(end)
		brace := v089Element(closing, "block-close")
		if brace == nil {
			continue
		}
		separator, count := 0, 0
		for _, child := range children[number] {
			if child >= end {
				break
			}
			line, _ := lines.Line(child)
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator, count = child, count+1
			}
		}
		if count != 1 || !v089EmptyAfter(lines, children[number], separator, end) {
			continue
		}
		line, _ := lines.Line(separator)
		// Перед маркером могут быть только однобайтовые U+0020/U+0009.
		start := len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t")) + 1
		occurrence, err := findings.Add(separator, start, separator, start+3, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, end, brace.Start(), end, brace.End()); err != nil {
			return err
		}
	}
	return nil
}

// v089Element возвращает единственный элемент нужного типа; nil означает
// отсутствие либо неоднозначность элемента объявления или границы.
func v089Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v089EmptyAfter возвращает true только при доказанном отсутствии строк
// содержимого. Непустая строка content (в том числе таблица с пустыми ячейками)
// либо неинтерпретируемая строка возвращает false. Разобранные вложенные
// конструкции не являются непосредственным содержимым проверяемого блока.
func v089EmptyAfter(lines diagnostics.Lines, children []int, separator, closing int) bool {
	for _, number := range children {
		if number <= separator {
			continue
		}
		if number >= closing {
			break
		}
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		switch line.Type() {
		case "block-start", "tag":
			if v089Element(line, "tag") == nil {
				// Одной классификации недостаточно, если само объявление
				// не удалось разобрать: его содержимое остаётся неизвестным.
				return false
			}
			// Ни название, ни содержимое вложенной конструкции не заполняют
			// итоговую часть example или тело table.
			continue
		default:
			return false
		}
	}
	return true
}

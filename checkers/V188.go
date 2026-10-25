package checkers

import (
	"fmt"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV188 регистрирует минимум ветвей в фигурном контейнере variants.
func registrationV188(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V188",
		Severity: diagnostics.SeverityError,
		Message:  "В @variants меньше двух ветвей @variant.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV188,
	})
}

// checkV188 использует границы и родительские связи модели без нового разбора DSL.
// Каждое объявление считается независимо от имени и содержимого ветви.
func checkV188(lines diagnostics.Lines, findings diagnostics.Findings) error {
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
		if opening.Type() != "block-start" || v188Tag(opening) != "variants" || v188LiteralParent(lines, opening) {
			continue
		}
		end := ends[number]
		if end <= number || v188Element(opening, "block-open") == nil {
			continue
		}
		closing, _ := lines.Line(end)
		if v188Element(closing, "block-close") == nil {
			continue
		}
		count, known := v188Count(lines, number, end, ends)
		if !known || count >= 2 {
			continue
		}
		tag := v188Element(opening, "tag")
		if _, err := findings.Add(number, tag.Start(), number, tag.End(), fmt.Sprintf("Найдено ветвей: %d.", count)); err != nil {
			return err
		}
	}
	return nil
}

// v188Count считает объявления в физическом диапазоне, пропуская вложенные блоки
// целиком. Логические переходы task, step и newpage не закрывают фигурный блок.
// false означает, что структура не позволяет установить точное число ветвей.
func v188Count(lines diagnostics.Lines, opening, closing int, ends map[int]int) (int, bool) {
	count := 0
	for number := opening + 1; number < closing; number++ {
		line, _ := lines.Line(number)
		last := number
		if line.Type() == "block-start" {
			end := ends[number]
			if end <= number || end >= closing || v188Element(line, "block-open") == nil {
				return 0, false
			}
			boundary, _ := lines.Line(end)
			if v188Element(boundary, "block-close") == nil {
				return 0, false
			}
			last = end
		} else if v188Element(line, "block-open") != nil || v188Element(line, "block-close") != nil {
			return 0, false
		}
		name := v188Tag(line)
		if name == "variant" {
			count++
		} else if name == "" && v188Element(line, "heading-level") == nil && (line.Type() == "invalid" || v188HasUnparsed(line)) {
			// Не восстанавливаем потенциально скрытое объявление из unparsed.
			return 0, false
		}
		number = last
	}
	return count, true
}

// v188Element возвращает единственный элемент типа; nil означает отсутствие
// либо неоднозначность. Исходный диапазон элемента имеет исключающий конец.
func v188Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v188Tag читает распознанное имя объявления, включая ошибку в его параметрах.
func v188Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return ""
	}
	if tag := v188Element(line, "tag"); tag != nil {
		name, _ := tag.Value()
		return strings.ToLower(name)
	}
	return ""
}

// v188HasUnparsed сообщает о наличии любого неразобранного фрагмента строки.
func v188HasUnparsed(line diagnostics.Line) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "unparsed" {
			return true
		}
	}
	return false
}

// v188LiteralParent исключает псевдоблоки внутри editor и multifill.
func v188LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if name := v188Tag(line); name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

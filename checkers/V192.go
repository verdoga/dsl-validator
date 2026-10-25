package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v192Unit хранит одну конструкцию до первой ветви, включая весь фигурный блок.
// first и last — включаемые физические номера строк, parent — логический родитель.
type v192Unit struct {
	first, last, parent int
	name                string
	heading             bool
}

// registrationV192 регистрирует проверку материала перед первой ветвью DSL 1.2.
func registrationV192(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V192", Severity: diagnostics.SeverityError,
		Message:  "Перед первой ветвью @variant размещён материал.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV192,
	})
}

// checkV192 проверяет конструкции по исходной структуре, не раскрывая вложений.
// После обработки участка его вложенные блоки повторно не посещаются.
func checkV192(lines diagnostics.Lines, findings diagnostics.Findings) error {
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-end" {
			parent, _ := line.ParentLine()
			if _, exists := ends[parent]; exists {
				ends[parent] = -1
			} else {
				ends[parent] = number
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if opening.Type() != "block-start" || v192Tag(opening) != "variants" || v192LiteralParent(lines, opening) {
			continue
		}
		end := v192Closing(lines, opening, ends)
		if end == 0 {
			continue
		}
		units, stop := v192Prelude(lines, number, end, ends)
		if err := v192Report(lines, findings, units); err != nil {
			return err
		}
		number = stop
	}
	return nil
}

// v192Prelude выделяет непосредственные конструкции до первой ветви.
// Неопределимая конструкция прекращает обход, сохраняя достоверный префикс.
// stop — последняя посещённая область; вложенный блок представлен одной единицей.
// Фигурный контекст задаётся физическими границами, а не логическим ParentLine.
func v192Prelude(lines diagnostics.Lines, opening, closing int, ends map[int]int) (units []v192Unit, stop int) {
	for number := opening + 1; number < closing; number++ {
		line, _ := lines.Line(number)
		name := v192Tag(line)
		if name == "variant" {
			return units, number - 1
		}
		last := number
		if line.Type() == "block-start" {
			last = v192Closing(lines, line, ends)
			if last == 0 || last >= closing {
				return units, closing
			}
		} else if v192Element(line, "block-open") != nil || v192Element(line, "block-close") != nil {
			return units, closing
		}
		heading := line.Type() == "heading" || v192Element(line, "heading-level") != nil
		if name == "" && !heading && line.Type() == "invalid" {
			// Не восстанавливаем скрытые теги и границы из неразобранного текста.
			return units, closing
		}
		if strings.Trim(line.Raw(), " \t") != "" {
			parent, _ := line.ParentLine()
			units = append(units, v192Unit{number, last, parent, name, heading})
		}
		number = last
	}
	return units, closing
}

// v192Report исключает служебные конструкции независимо от чужих диагностик.
// Связь с вопросом сохраняется только через пустые строки и editor того же места.
// Диапазоны включают полные Raw строк, конец задаётся исключающей Unicode-колонкой.
func v192Report(lines diagnostics.Lines, findings diagnostics.Findings, units []v192Unit) error {
	var previous v192Unit
	for _, unit := range units {
		if unit.name == "editor" {
			continue
		}
		excluded := unit.heading
		switch unit.name {
		case "include", "document-id", "section", "order", "resource-dir", "variants", "newpage":
			excluded = true
		case "answer":
			excluded = previous.name != "question" || previous.first != previous.last || previous.parent != unit.parent
		}
		if !excluded {
			last, _ := lines.Line(unit.last)
			if _, err := findings.Add(unit.first, 1, unit.last, utf8.RuneCountInString(last.Raw())+1, ""); err != nil {
				return err
			}
		}
		previous = unit
	}
	return nil
}

// v192Closing возвращает однозначное закрытие блока; ноль означает неизвестное.
func v192Closing(lines diagnostics.Lines, opening diagnostics.Line, ends map[int]int) int {
	end := ends[opening.Number()]
	if end <= opening.Number() || v192Element(opening, "block-open") == nil {
		return 0
	}
	closing, _ := lines.Line(end)
	if v192Element(closing, "block-close") == nil {
		return 0
	}
	return end
}

// v192Element возвращает единственный элемент типа либо nil при отсутствии
// или неоднозначности; исходные значения и позиции не преобразуются.
func v192Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v192Tag читает распознанное имя объявления без повторного разбора Raw.
func v192Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return ""
	}
	if tag := v192Element(line, "tag"); tag != nil {
		value, _ := tag.Value()
		return strings.ToLower(value)
	}
	return ""
}

// v192LiteralParent исключает литеральное содержимое editor и multifill.
func v192LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			if tag := v192Tag(line); tag == "editor" || tag == "multifill" {
				return true
			}
		}
	}
	return false
}

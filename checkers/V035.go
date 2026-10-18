package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV035 регистрирует проверку пустых блоков, включая answer, для DSL 1.2.
func registrationV035(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V035",
		Severity: diagnostics.SeverityError,
		Message:  "Блок не содержит непустого содержимого.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV035,
	})
}

// checkV035 проверяет каждый блок независимо по готовым родительским связям.
// Неоднозначные границы и неразобранное содержимое не дают ошибки о пустоте.
// Диапазон идёт от тега объявления до конца закрывающей скобки, без конкретизации.
func checkV035(lines diagnostics.Lines, findings diagnostics.Findings) error {
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
				ends[parent] = line.Number()
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if opening.Type() != "block-start" || !v035Selected(v035Tag(opening)) {
			continue
		}
		end := ends[opening.Number()]
		if end <= opening.Number() || v035Element(opening, "block-open") == nil {
			continue
		}
		closing, _ := lines.Line(end)
		brace := v035Element(closing, "block-close")
		if brace == nil || !v035Empty(lines, opening.Number(), end, ends) {
			continue
		}
		tag := v035Element(opening, "tag")
		if _, err := findings.Add(opening.Number(), tag.Start(), closing.Number(), brace.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v035Selected ограничивает проверку четырнадцатью блочными тегами.
func v035Selected(tag string) bool {
	switch tag {
	case "editor", "example", "wordlist", "table", "script", "text", "key", "instr",
		"note", "alt", "hint", "multifill", "fragment", "answer":
		return true
	default:
		return false
	}
}

// v035Element возвращает единственный элемент заданного типа; nil означает
// отсутствие либо неоднозначность. Используется для тегов и границ блоков.
func v035Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v035Tag читает каноническое имя тега без преобразования исходного текста.
func v035Tag(line diagnostics.Line) string {
	if element := v035Element(line, "tag"); element != nil {
		if value, present := element.Value(); present {
			return value
		}
	}
	return ""
}

// v035Descendant проверяет цепочку существующих родителей, не восстанавливая
// грамматику DSL. Целостность и отсутствие циклов обеспечивает конвейер.
func v035Descendant(lines diagnostics.Lines, line diagnostics.Line, ancestor int) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		if parent == ancestor {
			return true
		}
		line, _ = lines.Line(parent)
	}
}

// v035Allows задаёт разрешённые вложенные DSL-теги проверяемых контейнеров.
func v035Allows(parent, child string) bool {
	if parent == "instr" {
		return child == "hint"
	}
	if parent == "fragment" {
		switch child {
		case "media", "example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint":
			return true
		}
	}
	return false
}

// v035AllowedPath проверяет разрешённость всех вложений на пути к блоку.
func v035AllowedPath(lines diagnostics.Lines, line diagnostics.Line, ancestor int) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		container, _ := lines.Line(parent)
		if (line.Type() == "tag" || line.Type() == "block-start") &&
			!v035Allows(v035Tag(container), v035Tag(line)) {
			return false
		}
		if parent == ancestor {
			return true
		}
		line = container
	}
}

// v035FilledValue ищет непустое семантическое значение заданного типа,
// удаляя только крайние U+0020 и U+0009. Несколько текстовых частей допустимы.
func v035FilledValue(line diagnostics.Line, kind string) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if value, present := element.Value(); present && strings.Trim(value, " \t") != "" {
				return true
			}
		}
	}
	return false
}

// v035Empty возвращает true только при доказанной пустоте. Непустое содержимое,
// invalid/unparsed либо неоднозначные границы потомка возвращают false.
func v035Empty(lines diagnostics.Lines, opening, closing int, ends map[int]int) bool {
	for number := opening + 1; number < closing; number++ {
		line, _ := lines.Line(number)
		if !v035Descendant(lines, line, opening) {
			continue
		}
		if line.Type() == "invalid" {
			return false
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "unparsed" {
				return false
			}
		}
		if line.Type() == "block-start" {
			if end := ends[line.Number()]; end <= line.Number() || end >= closing {
				return false
			}
		}
		if !v035AllowedPath(lines, line, opening) {
			continue
		}
		switch line.Type() {
		case "content":
			if strings.Trim(line.Raw(), " \t") != "" {
				return false
			}
		case "tag":
			if v035Tag(line) == "media" {
				if v035FilledValue(line, "media-type") && v035FilledValue(line, "source") {
					return false
				}
			} else if v035FilledValue(line, "content") {
				return false
			}
		case "separator":
			parent, _ := line.ParentLine()
			container, _ := lines.Line(parent)
			tag := v035Tag(container)
			if (tag == "editor" || tag == "multifill") && strings.Trim(line.Raw(), " \t") != "" {
				return false
			}
		}
	}
	return true
}

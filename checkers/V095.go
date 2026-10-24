package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV095 регистрирует отсутствие строк данных таблицы в DSL 1.2.
func registrationV095(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V095",
		Severity: diagnostics.SeverityError,
		Message:  "Таблица не содержит строк данных.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV095,
	})
}

// checkV095 использует границы и родительство готовой модели. Отсутствующее
// или неоднозначное закрытие не позволяет выдать сообщение на весь блок.
// Диапазон — от имени тега до конца закрывающей скобки, в Unicode-колонках
// с 1 и с исключённой правой границей.
func checkV095(lines diagnostics.Lines, findings diagnostics.Findings) error {
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
		if opening.Type() != "block-start" {
			continue
		}
		tag := v095Element(opening, "tag")
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		end := ends[number]
		if name != "table" || end <= number || v095Element(opening, "block-open") == nil {
			continue
		}
		closing, _ := lines.Line(end)
		brace := v095Element(closing, "block-close")
		if brace == nil || !v095Empty(lines, number, end) {
			continue
		}
		if _, err := findings.Add(number, tag.Start(), end, brace.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v095Element возвращает единственный элемент указанного типа.
// nil означает отсутствие либо неоднозначность элемента.
func v095Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v095Empty возвращает true, если непосредственных строк данных нет.
// Разделитель по Raw либо распознанные данные дают false. Ошибочные строки
// и вложенные конструкции не заполняют таблицу и не подавляют диагностику.
func v095Empty(lines diagnostics.Lines, opening, closing int) bool {
	for number := opening + 1; number < closing; number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found || parent != opening {
			continue
		}
		if strings.Trim(line.Raw(), " \t") == "---" {
			return false
		}
		if line.Type() == "content" && v095HasContent(line) {
			return false
		}
	}
	return true
}

// v095HasContent возвращает true при наличии исходных символов, отличных
// от U+0020/U+0009, вне участков unparsed; иначе false. Участки не склеиваются
// и не декодируются. Координаты элементов заданы в кодовых точках.
func v095HasContent(line diagnostics.Line) bool {
	raw := []rune(line.Raw())
	start := 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "unparsed" {
			continue
		}
		if strings.Trim(string(raw[start:element.Start()-1]), " \t") != "" {
			return true
		}
		start = element.End() - 1
	}
	return strings.Trim(string(raw[start:]), " \t") != ""
}

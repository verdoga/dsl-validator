package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v088Content различает пустоту, непосредственное содержимое и неопределённость.
type v088Content uint8

const (
	v088Empty v088Content = iota
	v088Present
	v088Unknown
)

// v088Block хранит число непосредственных разделителей и состояние первой части.
// column — исходная Unicode-колонка первого дефиса, начиная с единицы.
type v088Block struct {
	separators, separator, column int
	before                        v088Content
}

// registrationV088 регистрирует отсутствие первой части блока только для DSL 1.2.
func registrationV088(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V088",
		Severity: diagnostics.SeverityError,
		Message:  "Перед разделителем --- отсутствует содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV088,
	})
}

// checkV088 собирает только непосредственные строки каждого распознанного блока.
// Все разделители считаются до выдачи результатов: второй отменяет применимость.
// Состояния вложенных блоков независимы; сообщения идут в порядке разделителей.
func checkV088(lines diagnostics.Lines, findings diagnostics.Findings) error {
	blocks := make(map[int]v088Block)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-start" {
			if name := v088Tag(line); name == "example" || name == "table" {
				blocks[number] = v088Block{}
			}
		}
		parent, _ := line.ParentLine()
		block, found := blocks[parent]
		if !found {
			continue
		}
		if strings.Trim(line.Raw(), " \t") == "---" {
			block.separators++
			if block.separators == 1 {
				block.separator = number
				// Перед маркером возможны только однобайтовые U+0020/U+0009.
				block.column = len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t")) + 1
			}
		} else if block.separators == 0 {
			state := v088LineContent(line)
			if state == v088Present || block.before == v088Empty {
				block.before = state
			}
		}
		blocks[parent] = block
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		block := blocks[parent]
		if block.separators != 1 || block.separator != number || block.before != v088Empty {
			continue
		}
		if _, err := findings.Add(number, block.column, number, block.column+3, ""); err != nil {
			return err
		}
	}
	return nil
}

// v088Tag читает сохранённое имя объявления без восстановления по Raw.
// Пустое имя означает, что объявление не распознано.
func v088Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.ToLower(name)
		}
	}
	return ""
}

// v088LineContent не считает объявления и границы строками материала.
// Непустой Raw content достаточен и для таблицы с пустыми ячейками: семантическое
// значение не обрезается и ячейки не пересчитываются. Неразобранные данные без
// достоверного содержимого оставляют неопределённость, а не доказывают пустоту.
func v088LineContent(line diagnostics.Line) v088Content {
	if strings.Trim(line.Raw(), " \t") == "" || line.Type() == "blank" ||
		line.Type() == "block-end" || v088Tag(line) != "" {
		return v088Empty
	}
	unparsed, content := false, false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		unparsed = unparsed || element.Type() == "unparsed"
		content = content || element.Type() == "content" && strings.Trim(element.Raw(), " \t") != ""
	}
	if line.Type() == "content" && (!unparsed || content) {
		return v088Present
	}
	return v088Unknown
}

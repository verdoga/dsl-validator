package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v125Range хранит исходный диапазон с исключающей правой Unicode-колонкой.
type v125Range struct{ line, start, end int }

// v125Block хранит физическое открытие блока и исходный диапазон имени тега.
type v125Block struct {
	name string
	tag  v125Range
}

// registrationV125 регистрирует запрет заголовков внутри fragment для DSL 1.2.
func registrationV125(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V125",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри @fragment записан заголовок.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV125,
	})
}

// checkV125 отмечает каждый заголовок один раз и связывает его с ближайшим fragment.
// Заголовок меняет логических родителей, но физический блок закрывает только }.
func checkV125(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var blocks []v125Block
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		literal := len(blocks) > 0 && (blocks[len(blocks)-1].name == "editor" || blocks[len(blocks)-1].name == "multifill")
		// После корневого heading закрытие может остаться в unparsed.
		// В литеральном блоке закрывает только отдельная неэкранированная }.
		if v125Closing(line, literal) {
			closing := len(blocks) - 1
			if parent, found := line.ParentLine(); found && !literal && line.Type() == "block-end" {
				for index := len(blocks) - 1; index >= 0; index-- {
					if blocks[index].tag.line == parent {
						closing = index
						break
					}
				}
			}
			if closing >= 0 {
				blocks = blocks[:closing]
			}
			continue
		}
		if literal {
			continue
		}
		if block, found := v125Opening(line); found {
			blocks = append(blocks, block)
		}
		start, end := v125Heading(line)
		if end == 0 {
			continue
		}
		fragment := v125Fragment(blocks)
		if fragment.line == 0 {
			continue
		}
		occurrence, err := findings.Add(number, start, number, end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, fragment.line, fragment.start, fragment.line, fragment.end); err != nil {
			return err
		}
	}
	return nil
}

// v125Closing учитывает отдельную } и распознанный block-close даже в invalid.
// В литеральном содержимом структурные элементы не заменяют отдельную }.
func v125Closing(line diagnostics.Line, literal bool) bool {
	if strings.Trim(line.Raw(), " \t") == "}" {
		return true
	}
	if literal {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-close" {
			return true
		}
	}
	return false
}

// v125Heading возвращает исходный диапазон маркера уровня 1–3; конец исключается.
// Без heading-level восстановление разрешено только для invalid или unparsed.
// Очистка краёв учитывает U+0020/U+0009, разделитель — Unicode White_Space.
// Экранирование не снимается; нулевой конец означает отсутствие заголовка.
func v125Heading(line diagnostics.Line) (start, end int) {
	raw := []rune(line.Raw())
	start = 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	end = start
	for end < len(raw) && raw[end] == '#' {
		end++
	}
	if end-start < 1 || end-start > 3 {
		return 0, 0
	}
	unparsed := line.Type() == "invalid"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "heading-level" && element.Start() == start+1 && element.End() == end+1 {
			return start + 1, end + 1
		}
		unparsed = unparsed || element.Type() == "unparsed"
	}
	limit := len(raw)
	for limit > end && (raw[limit-1] == ' ' || raw[limit-1] == '\t') {
		limit--
	}
	if !unparsed || end == limit || !unicode.Is(unicode.White_Space, raw[end]) {
		return 0, 0
	}
	return start + 1, end + 1
}

// v125Opening читает реальные block-open, в том числе на частично разобранной
// строке. Скобки в тексте и псевдообъявления литерального тела блок не открывают.
func v125Opening(line diagnostics.Line) (block v125Block, found bool) {
	block.tag.line = line.Number()
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			found = true
		}
		if element.Type() == "tag" {
			block.name, _ = element.Value()
			block.tag.start, block.tag.end = element.Start(), element.End()
		}
	}
	return block, found
}

// v125Fragment возвращает диапазон имени ближайшего физического fragment.
// Нулевая строка означает отсутствие fragment либо ближайший фигурный table;
// литеральное содержимое исключается до вызова этого помощника.
func v125Fragment(blocks []v125Block) v125Range {
	if len(blocks) > 0 && blocks[len(blocks)-1].name == "table" {
		return v125Range{}
	}
	for index := len(blocks) - 1; index >= 0; index-- {
		if blocks[index].name == "fragment" {
			return blocks[index].tag
		}
	}
	return v125Range{}
}

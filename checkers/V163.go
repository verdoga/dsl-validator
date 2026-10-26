package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v163Position хранит исходную строку и Unicode-колонку первого разделителя с единицы.
type v163Position struct{ line, column int }

// v163Block хранит физическое открытие и первый непосредственный разделитель.
type v163Block struct {
	line  int
	name  string
	first v163Position
}

// registrationV163 регистрирует повтор разделителя matching только для DSL 1.2.
func registrationV163(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V163", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "В @matching повторяется разделитель списков ---.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV163,
	})
}

// checkV163 отмечает каждый повтор отдельно, связывая его с первым разделителем
// того же физического блока. Заголовок не закрывает блок и не сбрасывает первый
// разделитель. Учитываются только непосредственные строки, независимо от их типа.
// Диапазоны охватывают три дефиса; правая граница исключается.
func checkV163(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var blocks []v163Block
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		literal := len(blocks) > 0 && (blocks[len(blocks)-1].name == "editor" || blocks[len(blocks)-1].name == "multifill")
		if v163Closing(line, literal) {
			closing := len(blocks) - 1
			if parent, found := line.ParentLine(); found && !literal && line.Type() == "block-end" {
				for index := len(blocks) - 1; index >= 0; index-- {
					if blocks[index].line == parent {
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
		if block, found := v163Opening(line); found {
			blocks = append(blocks, block)
			continue
		}
		if len(blocks) == 0 || blocks[len(blocks)-1].name != "matching" {
			continue
		}
		column := v163Separator(line.Raw())
		if column == 0 {
			continue
		}
		previous := &blocks[len(blocks)-1].first
		if previous.line == 0 {
			*previous = v163Position{line.Number(), column}
			continue
		}
		occurrence, err := findings.Add(line.Number(), column, line.Number(), column+3, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, previous.line, previous.column, previous.line, previous.column+3); err != nil {
			return err
		}
	}
	return nil
}

// v163Closing распознаёт отдельную } или реальный block-close, в том числе в invalid.
// Литеральный блок закрывает только отдельная }; false означает отсутствие закрытия.
func v163Closing(line diagnostics.Line, literal bool) bool {
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

// v163Opening читает реальный block-open и каноническое имя тега из модели.
// found=false означает отсутствие открытия; скобки в текстовом значении не учитываются.
func v163Opening(line diagnostics.Line) (block v163Block, found bool) {
	block.line = line.Number()
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			found = true
		}
		if element.Type() == "tag" {
			block.name, _ = element.Value()
		}
	}
	return block, found
}

// v163Separator возвращает начальную Unicode-колонку трёх дефисов с единицы.
// Ноль означает отсутствие разделителя. Снимаются только краевые U+0020/U+0009;
// экранирование и семантическое значение строки не интерпретируются.
func v163Separator(raw string) int {
	if strings.Trim(raw, " \t") != "---" {
		return 0
	}
	// Каждый допустимый символ отступа занимает один байт и одну кодовую точку.
	return len(raw) - len(strings.TrimLeft(raw, " \t")) + 1
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v175Position хранит исходный диапазон первого разделителя блока.
type v175Position struct{ line, start, end int }

// v175Context хранит имя открытого физического блока и его первый разделитель.
type v175Context struct {
	name  string
	first v175Position
}

// registrationV175 регистрирует повторные разделители ordering только для DSL 1.2.
func registrationV175(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V175", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "В @ordering повторяется разделитель списков ---.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV175,
	})
}

// checkV175 проверяет непосредственные строки физического блока ordering.
// Границы задают готовые элементы скобок и отдельная }; заголовки блок не закрывают.
// Тип строки и наличие unparsed не исключают разделитель с однозначным Raw.
func checkV175(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var blocks []v175Context
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		literal := len(blocks) != 0 && (blocks[len(blocks)-1].name == "editor" || blocks[len(blocks)-1].name == "multifill")
		// После heading отдельная } может остаться в unparsed. В литеральном
		// теле только такая строка закрывает блок, даже при наличии block-close.
		if strings.Trim(line.Raw(), " \t") == "}" || !literal && v175Element(line, "block-close") != nil {
			if len(blocks) != 0 {
				blocks = blocks[:len(blocks)-1]
			}
			continue
		}
		if literal {
			continue
		}
		if v175Element(line, "block-open") != nil {
			var name string
			if tag := v175Element(line, "tag"); tag != nil {
				name, _ = tag.Value()
			}
			blocks = append(blocks, v175Context{name: strings.ToLower(name)})
			continue
		}
		if len(blocks) == 0 || blocks[len(blocks)-1].name != "ordering" {
			continue
		}
		start, end := v175Separator(line.Raw())
		if end == 0 {
			continue
		}
		block := &blocks[len(blocks)-1]
		if block.first.line == 0 {
			block.first = v175Position{line.Number(), start, end}
			continue
		}
		previous := block.first
		occurrence, err := findings.Add(line.Number(), start, line.Number(), end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
			return err
		}
	}
	return nil
}

// v175Element читает распознанный элемент, не извлекая скобки из текста.
func v175Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

// v175Separator возвращает Unicode-колонки трёх дефисов с 1, конец исключается.
// Нули означают отсутствие разделителя. Обрезаются только U+0020 и U+0009;
// экранирование не снимается, семантическое значение не используется.
func v175Separator(raw string) (start, end int) {
	if strings.Trim(raw, " \t") != "---" {
		return 0, 0
	}
	// Перед дефисами допустимы только однобайтовые пробелы и табуляции.
	start = len(raw) - len(strings.TrimLeft(raw, " \t")) + 1
	return start, start + 3
}

package checkers

import (
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV004 регистрирует рекомендацию о соседних пустых строках
// только для DSL 1.2.
func registrationV004(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V004",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Несколько пустых строк подряд.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV004,
	})
}

// checkV004 объединяет непосредственно соседние строки типа blank,
// кроме содержимого блочного multifill. Raw повторно не классифицируется.
func checkV004(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first, last := 0, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "blank" && !v004InsideMultifill(lines, line) {
			if first == 0 {
				first = number
			}
			last = number
			continue
		}
		if err := v004Report(lines, findings, first, last); err != nil {
			return err
		}
		first, last = 0, 0
	}
	return v004Report(lines, findings, first, last)
}

// v004InsideMultifill возвращает true при наличии предка block-start
// с каноническим тегом multifill, иначе false. Проверяется вся цепочка;
// существование предшествующих родителей гарантировано контролем модели.
func v004InsideMultifill(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			if value, present := element.Value(); present && value == "multifill" {
				return true
			}
		}
	}
}

// v004Report выдаёт одно сообщение для последовательности из двух и более
// строк. first=0 означает отсутствие последовательности. Диапазон начинается
// в колонке 1 и заканчивается после Raw последней строки; Unicode-колонки
// считаются с 1, правая граница исключается, пустой Raw заканчивается в 1.
func v004Report(lines diagnostics.Lines, findings diagnostics.Findings, first, last int) error {
	if first == 0 || last == first {
		return nil
	}
	line, _ := lines.Line(last)
	_, err := findings.Add(first, 1, last, utf8.RuneCountInString(line.Raw())+1, "")
	return err
}

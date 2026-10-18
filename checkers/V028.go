package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV028 регистрирует рекомендацию о пробелах внутри скобок для DSL 1.2.
func registrationV028(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V028",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Пробел находится с внутренней стороны скобки.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV028,
	})
}

// checkV028 отмечает максимальные последовательности U+0020 непосредственно
// после ( или [ либо перед ) или ] во всём Raw, включая unparsed.
// Единственные исключения — ( ) и [ ] с одним U+0020. Парность не проверяется.
// Диапазон охватывает только пробелы в Unicode-колонках с 1; конец исключается.
func checkV028(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		runes := []rune(line.Raw())
		for i := 0; i < len(runes); {
			if runes[i] != ' ' {
				i++
				continue
			}
			start := i
			for i < len(runes) && runes[i] == ' ' {
				i++
			}
			afterOpening := start > 0 && (runes[start-1] == '(' || runes[start-1] == '[')
			beforeClosing := i < len(runes) && (runes[i] == ')' || runes[i] == ']')
			if !afterOpening && !beforeClosing {
				continue
			}
			if i-start == 1 && afterOpening && beforeClosing &&
				(runes[start-1] == '(' && runes[i] == ')' || runes[start-1] == '[' && runes[i] == ']') {
				continue
			}
			detail := "После открывающей скобки"
			if afterOpening && beforeClosing {
				detail = "Между открывающей и закрывающей скобками"
			} else if beforeClosing {
				detail = "Перед закрывающей скобкой"
			}
			if _, err := findings.Add(number, start+1, number, i+1, detail); err != nil {
				return err
			}
		}
	}
	return nil
}

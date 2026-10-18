package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV026 регистрирует предупреждение о повторе знака препинания
// только для DSL 1.2.
func registrationV026(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V026",
		Severity: diagnostics.SeverityWarning,
		Message:  "Одинаковый знак препинания повторён подряд.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV026,
	})
}

// checkV026 проверяет все строки; разрешённое форматирование в text прозрачно.
// Максимальная цепочка одинаковых знаков даёт одно сообщение, кроме одиночного
// знака и ровно трёх U+002E. Диапазон задаётся исходными Unicode-колонками с 1
// и исключающей правой границей; между строками цепочки не продолжаются.
func checkV026(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		raw, positions := visibleText(raw, textFormatting(lines, line, raw), 0, len(raw))
		for start := 0; start < len(raw); {
			switch raw[start] {
			case '.', ',', ';', ':', '!', '?', '…':
			default:
				start++
				continue
			}
			end := start + 1
			for end < len(raw) && raw[end] == raw[start] {
				end++
			}
			length := end - start
			if length > 1 && !(raw[start] == '.' && length == 3) {
				if _, err := findings.Add(number, positions[start]+1, number, positions[end-1]+2, ""); err != nil {
					return err
				}
			}
			start = end
		}
	}
	return nil
}

package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV012 регистрирует рекомендацию о тире между цифрами для DSL 1.2.
func registrationV012(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V012",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Между числами используется дефис или длинное тире.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV012,
	})
}

// checkV012 отмечает каждый дефис или длинное тире между цифрами в Raw
// элементов content/title/name. Даты, арифметика и HTML отдельно не распознаются.
// Горизонтальные пробелы вокруг знака пропускаются только при поиске соседей.
// Диапазон охватывает знак в исходных Unicode-колонках с 1, конец исключается.
func checkV012(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name":
			default:
				continue
			}
			raw := []rune(element.Raw())
			for i, char := range raw {
				if char != '-' && char != '—' {
					continue
				}
				left, right := i-1, i+1
				for left >= 0 && v012HorizontalSpace(raw[left]) {
					left--
				}
				for right < len(raw) && v012HorizontalSpace(raw[right]) {
					right++
				}
				if left < 0 || right >= len(raw) || !unicode.IsDigit(raw[left]) || !unicode.IsDigit(raw[right]) {
					continue
				}
				column := element.Start() + i
				if _, err := findings.Add(number, column, number, column+1,
					"Если это числовой диапазон, используйте короткое тире: 1–12"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v012HorizontalSpace возвращает true для табуляции и Unicode Zs;
// false означает символ, через который поиск соседней цифры не продолжается.
func v012HorizontalSpace(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}

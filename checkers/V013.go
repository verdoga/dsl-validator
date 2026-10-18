package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV013 регистрирует рекомендацию о дефисе между пробелами для DSL 1.2.
func registrationV013(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V013",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Между словами стоит дефис, отделённый пробелами.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV013,
	})
}

// checkV013 отмечает U+002D с горизонтальным пробелом непосредственно с обеих
// сторон в точном Raw всех строк, кроме структурных разделителей. Буквы,
// числа, HTML и классификация элементов не влияют на проверку. Диапазон
// охватывает один символ в Unicode-колонках с 1; правая граница исключается.
func checkV013(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "separator" {
			continue
		}
		runes := []rune(line.Raw())
		for i := 1; i+1 < len(runes); i++ {
			// Пробелы с обеих сторон также исключают последовательности дефисов.
			if runes[i] != '-' || !v013HorizontalSpace(runes[i-1]) || !v013HorizontalSpace(runes[i+1]) {
				continue
			}
			if _, err := findings.Add(number, i+1, number, i+2,
				"Для разделения частей фразы рекомендуется короткое тире –"); err != nil {
				return err
			}
		}
	}
	return nil
}

// v013HorizontalSpace возвращает true для U+0009 и Unicode-категории Zs;
// остальные символы, включая вертикальные разделители, дают false.
func v013HorizontalSpace(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}

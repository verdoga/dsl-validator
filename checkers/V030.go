package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV030 регистрирует рекомендацию о нестандартных дефисах и тире
// только для DSL 1.2.
func registrationV030(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V030",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Использован нестандартный знак дефиса или тире.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV030,
	})
}

// checkV030 отмечает каждый заданный символ в исходных текстовых элементах
// и unparsed без преобразований. Диапазон содержит одну кодовую точку Unicode,
// колонки начинаются с 1; конкретизация и связанные диапазоны отсутствуют.
func checkV030(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name", "unparsed":
			default:
				continue
			}
			column := element.Start()
			for _, char := range element.Raw() {
				if v030NonstandardDash(char) {
					if _, err := findings.Add(line.Number(), column, line.Number(), column+1, ""); err != nil {
						return err
					}
				}
				column++
			}
		}
	}
	return nil
}

// v030NonstandardDash возвращает true только для восьми символов V030.
func v030NonstandardDash(char rune) bool {
	switch char {
	case '\u00AD', '\u2010', '\u2011', '\u2012', '\u2015', '\uFE58', '\uFE63', '\uFF0D':
		return true
	default:
		return false
	}
}

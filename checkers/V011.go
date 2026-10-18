package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV011 регистрирует рекомендацию о длинном тире только для DSL 1.2.
func registrationV011(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V011",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Использовано длинное тире.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV011,
	})
}

// checkV011 отмечает каждый U+2014 в Raw элементов content, title и name.
// Соседние символы и разметка не создают исключений; текст не преобразуется.
// Диапазон одного знака — [column,column+1) в исходных Unicode-колонках с 1.
func checkV011(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name":
			default:
				continue
			}
			column := element.Start()
			for _, char := range element.Raw() {
				if char == '\u2014' {
					if _, err := findings.Add(number, column, number, column+1,
						"Для этой позиции рекомендуется короткое тире – U+2013"); err != nil {
						return err
					}
				}
				column++
			}
		}
	}
	return nil
}

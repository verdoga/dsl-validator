package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV001 регистрирует предупреждение о повторных пробелах для DSL 1.2.
func registrationV001(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V001",
		Severity: diagnostics.SeverityWarning,
		Message:  "Несколько пробелов подряд.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV001,
	})
}

// checkV001 отмечает максимальные последовательности из двух и более U+0020
// в Raw всех физических строк без преобразований и исключений по содержимому.
// Колонки считаются в кодовых точках Unicode с 1; правая граница исключается.
func checkV001(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		column, spaces := 1, 0
		for _, char := range line.Raw() {
			if char == ' ' {
				spaces++
			} else {
				if spaces >= 2 {
					if _, err := findings.Add(number, column-spaces, number, column, ""); err != nil {
						return err
					}
				}
				spaces = 0
			}
			column++
		}
		if spaces >= 2 {
			if _, err := findings.Add(number, column-spaces, number, column, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

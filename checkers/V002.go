package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV002 регистрирует предупреждение о табуляциях для DSL 1.2.
func registrationV002(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V002",
		Severity: diagnostics.SeverityWarning,
		Message:  "В строке есть табуляция.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV002,
	})
}

// checkV002 отмечает максимальные непрерывные последовательности U+0009
// в Raw всех физических строк без преобразований и исключений по содержимому.
// Колонки считаются в кодовых точках Unicode с 1; правая граница исключается.
func checkV002(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		column, tabs := 1, 0
		for _, char := range line.Raw() {
			if char == '\t' {
				tabs++
			} else {
				if tabs > 0 {
					if _, err := findings.Add(number, column-tabs, number, column, ""); err != nil {
						return err
					}
				}
				tabs = 0
			}
			column++
		}
		if tabs > 0 {
			if _, err := findings.Add(number, column-tabs, number, column, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

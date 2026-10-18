package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV007 регистрирует предупреждение о непарных круглых скобках для DSL 1.2.
func registrationV007(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V007",
		Severity: diagnostics.SeverityWarning,
		Message:  "Нарушена парность круглых скобок в строке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV007,
	})
}

// checkV007 сопоставляет U+0028 и U+0029 в точном Raw каждой строки отдельно.
// Кавычки и обратная косая черта не скрывают скобки. Каждый непарный символ
// отмечается диапазоном в Unicode-колонках с 1 с исключающей правой границей.
func checkV007(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		var openings []int
		column := 1
		for _, char := range line.Raw() {
			switch char {
			case '(':
				openings = append(openings, column)
			case ')':
				if len(openings) > 0 {
					openings = openings[:len(openings)-1]
				} else if _, err := findings.Add(number, column, number, column+1,
					"Закрывающая круглая скобка не имеет открывающей в этой строке"); err != nil {
					return err
				}
			}
			column++
		}
		for _, opening := range openings {
			if _, err := findings.Add(number, opening, number, opening+1,
				"Открывающая круглая скобка не закрыта в этой строке"); err != nil {
				return err
			}
		}
	}
	return nil
}

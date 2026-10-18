package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV008 регистрирует предупреждение о парности квадратных скобок
// в пределах каждой физической строки только для DSL 1.2.
func registrationV008(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V008",
		Severity: diagnostics.SeverityWarning,
		Message:  "Нарушена парность квадратных скобок в строке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV008,
	})
}

// checkV008 создаёт собственный стек исходных Unicode-колонок для каждой строки.
// Raw не преобразуется; кавычки, обратная косая черта и остальные символы
// не влияют на сопоставление. Диапазон каждой скобки — [column,column+1).
func checkV008(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		var openings []int
		column := 1
		for _, char := range line.Raw() {
			switch char {
			case '[':
				openings = append(openings, column)
			case ']':
				if len(openings) != 0 {
					openings = openings[:len(openings)-1]
				} else if _, err := findings.Add(number, column, number, column+1,
					"Закрывающая квадратная скобка не имеет открывающей в этой строке"); err != nil {
					return err
				}
			}
			column++
		}
		for _, opening := range openings {
			if _, err := findings.Add(number, opening, number, opening+1,
				"Открывающая квадратная скобка не закрыта в этой строке"); err != nil {
				return err
			}
		}
	}
	return nil
}

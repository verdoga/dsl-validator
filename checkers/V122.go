package checkers

import (
	"fmt"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV122 регистрирует повреждения разметки из входной модели DSL 1.2.
func registrationV122(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V122", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Обнаружена повреждённая разметка.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV122,
	})
}

// checkV122 отмечает invalid целиком, а в остальных строках — каждый unparsed.
// Диапазоны используют Unicode-колонки с 1 и исключающий конец; текст не разбирается.
func checkV122(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for index := 1; index <= lines.Len(); index++ {
		line, _ := lines.Line(index)
		number := line.Number()
		if line.Type() == "invalid" {
			if _, err := findings.Add(number, 1, number, utf8.RuneCountInString(line.Raw())+1,
				fmt.Sprintf("Повреждена строка %d", number)); err != nil {
				return err
			}
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "unparsed" {
				if _, err := findings.Add(number, element.Start(), number, element.End(),
					fmt.Sprintf("Повреждён элемент в строке %d", number)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

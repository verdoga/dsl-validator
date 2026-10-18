package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV047 регистрирует отдельную строку с открывающей скобкой для DSL 1.2.
func registrationV047(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V047",
		Severity: diagnostics.SeverityError,
		Message:  "Открывающая фигурная скобка находится на отдельной строке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV047,
	})
}

// checkV047 проверяет все исходные строки независимо от контекста и парности.
// Удаляются только крайние U+0020 и U+0009; диапазон охватывает саму скобку
// в Unicode-колонках с единицы с исключающей правой границей.
func checkV047(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := line.Raw()
		leftTrimmed := strings.TrimLeft(raw, " \t")
		if strings.TrimRight(leftTrimmed, " \t") != "{" {
			continue
		}
		// Удалённый префикс состоит только из однобайтовых пробелов и табуляций.
		column := len(raw) - len(leftTrimmed) + 1
		if _, err := findings.Add(number, column, number, column+1,
			"Открывающая скобка должна завершать строку объявления блока"); err != nil {
			return err
		}
	}
	return nil
}

package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV042 регистрирует проверку положительного целого аргумента order
// только для DSL 1.2. Конкретизация не предусмотрена.
func registrationV042(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V042",
		Severity: diagnostics.SeverityError,
		Message:  "Значение @order не является положительным целым числом.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV042,
	})
}

// checkV042 проверяет весь исходный аргумент распознанного order, включая
// unparsed. Границы заданы в рунах, конец исключается; число не преобразуется
// в машинное целое. Удаляются только крайние U+0020 и U+0009.
func checkV042(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			name, _ := element.Value()
			if element.Type() != "tag" || name != "order" {
				continue
			}
			raw := []rune(line.Raw())
			start, end := element.End()-1, len(raw)
			for start < end && (raw[start] == ' ' || raw[start] == '\t') {
				start++
			}
			for start < end && (raw[end-1] == ' ' || raw[end-1] == '\t') {
				end--
			}
			if start == end {
				continue
			}
			valid, nonzero := true, false
			for _, char := range raw[start:end] {
				if char < '0' || char > '9' {
					valid = false
				}
				if char >= '1' && char <= '9' {
					nonzero = true
				}
			}
			if valid && nonzero {
				continue
			}
			if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

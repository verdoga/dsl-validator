package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV041 регистрирует отсутствие значения у @order для DSL 1.2.
func registrationV041(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V041",
		Severity: diagnostics.SeverityError,
		Message:  "У @order отсутствует значение порядка.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV041,
	})
}

// checkV041 проверяет исходный остаток после распознанного имени order.
// Удаляются только крайние U+0020 и U+0009; непустые значения не проверяются.
func checkV041(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v041Literal(lines, line) {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if name != "order" {
				continue
			}
			// End — исключающая Unicode-колонка с единицы, не байтовый индекс.
			remainder := string([]rune(line.Raw())[element.End()-1:])
			if strings.Trim(remainder, " \t") != "" {
				continue
			}
			if _, err := findings.Add(number, element.Start(), number, element.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v041Literal определяет литеральное содержимое по цепочке родителей строки.
func v041Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV038 регистрирует ошибку отсутствующего аргумента document-id
// только для DSL 1.2. Конкретизация не предусмотрена.
func registrationV038(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V038",
		Severity: diagnostics.SeverityError,
		Message:  "У @document-id отсутствует идентификатор.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV038,
	})
}

// checkV038 выбирает распознанные tag по каноническому Value модели.
// Проверяет весь исходный остаток после End, включая unparsed, удаляя только
// крайние U+0020/U+0009. Диапазон точно совпадает с tag, конец исключается.
func checkV038(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			name, _ := element.Value()
			if element.Type() != "tag" || name != "document-id" {
				continue
			}
			rest := string([]rune(line.Raw())[element.End()-1:])
			if strings.Trim(rest, " \t") != "" {
				continue
			}
			if _, err := findings.Add(number, element.Start(), number, element.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

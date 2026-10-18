package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV040 регистрирует ошибку отсутствующего названия секции в DSL 1.2.
func registrationV040(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V040",
		Severity: diagnostics.SeverityError,
		Message:  "У @section отсутствует название секции.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV040,
	})
}

// checkV040 проверяет исходный остаток строки после распознанного тега section.
// Удаляются только крайние U+0020 и U+0009; наличие элемента name не требуется.
// Диапазон ошибки совпадает с tag: Unicode-колонки с исключающим концом.
func checkV040(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			tag, _ := line.Element(index)
			if tag.Type() != "tag" {
				continue
			}
			name, _ := tag.Value()
			if name != "section" {
				continue
			}
			remainder := string([]rune(line.Raw())[tag.End()-1:])
			if strings.Trim(remainder, " \t") != "" {
				continue
			}
			if _, err := findings.Add(line.Number(), tag.Start(), line.Number(), tag.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

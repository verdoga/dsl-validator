package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV037 регистрирует требование наличия document-id только для DSL 1.2.
func registrationV037(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V037",
		Severity: diagnostics.SeverityError,
		Message:  "В документе отсутствует @document-id.",
		Scope:    diagnostics.ScopeDocument,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV037,
	})
}

// checkV037 ищет распознанное объявление независимо от его аргумента и места.
// При отсутствии доступных строк проверка пропускается. Ошибка относится
// ко всему документу и не имеет диапазона или конкретизации.
func checkV037(lines diagnostics.Lines, findings diagnostics.Findings) error {
	if lines.Len() == 0 {
		return nil
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v037LiteralContent(lines, line) {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if strings.EqualFold(name, "document-id") {
				return nil
			}
		}
	}
	_, err := findings.Add(0, 0, 0, 0, "")
	return err
}

// v037LiteralContent возвращает true для потомков литеральных блоков editor
// и multifill; false означает отсутствие такого предка в готовой модели.
func v037LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "tag" {
				name, _ := element.Value()
				if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
					return true
				}
			}
		}
	}
}

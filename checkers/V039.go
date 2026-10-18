package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// v039Position хранит точный исходный диапазон первого объявления метатега.
type v039Position struct {
	line, start, end int
}

// registrationV039 регистрирует проверку повторных метатегов для DSL 1.2.
func registrationV039(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V039",
		Severity: diagnostics.SeverityError,
		Message:  "Метатег объявлен повторно.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV039,
	})
}

// checkV039 учитывает распознанные объявления независимо от параметров
// и размещения. Каждый повтор связан с первым объявлением своего имени.
func checkV039(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first := make(map[string]v039Position)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v039Literal(lines, line) {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			switch name {
			case "document-id", "section", "order":
			default:
				continue
			}
			previous, found := first[name]
			if !found {
				first[name] = v039Position{number, element.Start(), element.End()}
				continue
			}
			occurrence, err := findings.Add(number, element.Start(), number, element.End(), "Метатег: @"+name)
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v039Literal определяет литеральное содержимое по существующим родителям,
// не разбирая исходный текст и не восстанавливая отсутствующие теги.
func v039Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				name, _ := element.Value()
				if name == "editor" || name == "multifill" {
					return true
				}
			}
		}
	}
	return false
}

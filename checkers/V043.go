package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV043 регистрирует требование resource-dir при любом media
// только для DSL 1.2, включая media с явным путём.
func registrationV043(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V043",
		Severity: diagnostics.SeverityError,
		Message:  "В документе с @media отсутствует @resource-dir.",
		Scope:    diagnostics.ScopeDocument,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV043,
	})
}

// checkV043 учитывает только распознанные объявления во всём документе.
// Заполненность и расположение resource-dir не проверяются. Связанный диапазон
// охватывает первый tag media в исходных Unicode-колонках, исключая конец.
func checkV043(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var firstLine, start, end int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v043Literal(lines, line) {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			tag, _ := element.Value()
			switch tag {
			case "resource-dir":
				return nil
			case "media":
				if firstLine == 0 {
					firstLine, start, end = number, element.Start(), element.End()
				}
			}
		}
	}
	if firstLine == 0 {
		return nil
	}
	occurrence, err := findings.Add(0, 0, 0, 0, "")
	if err != nil {
		return err
	}
	return findings.AddRelated(occurrence, firstLine, start, firstLine, end)
}

// v043Literal сообщает, находится ли строка внутри литерального блока
// editor или multifill. Собственное объявление строки не исключается.
func v043Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			tag, _ := element.Value()
			if tag == "editor" || tag == "multifill" {
				return true
			}
		}
	}
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV036 регистрирует проверку расположения метатегов для DSL 1.2.
func registrationV036(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V036",
		Severity: diagnostics.SeverityError,
		Message:  "Метатег находится вне области метаданных.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV036,
	})
}

// checkV036 проверяет существующие элементы tag, не распознавая unparsed.
// Причины объединяются на полном диапазоне элемента с исключающим концом.
func checkV036(lines diagnostics.Lines, findings diagnostics.Findings) error {
	version, heading := v036Boundaries(lines)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		block, literal := v036Ancestors(lines, line)
		if literal {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			tag, _ := element.Value()
			switch tag {
			case "document-id", "section", "order", "resource-dir":
			default:
				continue
			}
			var reasons []string
			if version == 0 {
				reasons = append(reasons, "Не найден корневой @dsl-version")
			} else if number <= version {
				reasons = append(reasons, "До корневого @dsl-version")
			}
			if heading != 0 && number >= heading {
				reasons = append(reasons, "После первого заголовка уровня 1")
			}
			if block {
				reasons = append(reasons, "Внутри фигурного блока")
			}
			if len(reasons) != 0 {
				if _, err := findings.Add(number, element.Start(), number, element.End(), strings.Join(reasons, "; ")); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v036Boundaries находит первые границы области; ноль означает отсутствие.
func v036Boundaries(lines diagnostics.Lines) (version, heading int) {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		_, parent := line.ParentLine()
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			value, present := element.Value()
			if !present {
				continue
			}
			if version == 0 && !parent && element.Type() == "tag" && value == "dsl-version" {
				version = number
			}
			if heading == 0 && line.Type() == "heading" && element.Type() == "heading-level" && value == "1" {
				heading = number
			}
		}
	}
	return version, heading
}

// v036Ancestors возвращает наличие фигурного блока и литерального контейнера
// среди всех предков. Собственная строка объявления предком не считается.
func v036Ancestors(lines diagnostics.Lines, line diagnostics.Line) (block, literal bool) {
	for {
		parent, found := line.ParentLine()
		if !found {
			return block, false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		block = true
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				tag, _ := element.Value()
				if tag == "editor" || tag == "multifill" {
					return true, true
				}
			}
		}
	}
}

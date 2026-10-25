package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV158 регистрирует ограничение числа объявлений answer в choice.
func registrationV158(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V158", Severity: diagnostics.SeverityError,
		Message:  "В @choice объявлено несколько тегов @answer.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV158,
	})
}

// v158Declaration хранит исходный диапазон имени тега; конец исключается.
type v158Declaration struct {
	line, start, end int
}

// checkV158 считает только непосредственные объявления во всём choice.
// Содержимое ответа не исследуется, варианты и подсказки счётчик не сбрасывают.
func checkV158(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first := make(map[int]v158Declaration)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		answer := v158Tag(line, "answer")
		if answer == nil {
			continue
		}
		parentNumber, found := line.ParentLine()
		if !found {
			continue
		}
		parent, _ := lines.Line(parentNumber)
		if parent.Type() != "block-start" || v158Tag(parent, "choice") == nil || v158LiteralParent(lines, parent) {
			continue
		}
		previous, exists := first[parentNumber]
		if !exists {
			first[parentNumber] = v158Declaration{number, answer.Start(), answer.End()}
			continue
		}
		occurrence, err := findings.Add(number, answer.Start(), number, answer.End(),
			"В @choice допускается только один правильный вариант.")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
			return err
		}
	}
	return nil
}

// v158Tag возвращает единственное распознанное имя объявления, в том числе
// ошибочного. Без элемента tag имя из Raw не восстанавливается.
func v158Tag(line diagnostics.Line, name string) diagnostics.Element {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return nil
	}
	var tag diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			if tag != nil {
				return nil
			}
			tag = element
		}
	}
	if tag != nil {
		value, _ := tag.Value()
		if strings.EqualFold(value, name) {
			return tag
		}
	}
	return nil
}

// v158LiteralParent исключает псевдообъявления в литеральных областях.
func v158LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" && (v158Tag(line, "editor") != nil || v158Tag(line, "multifill") != nil) {
			return true
		}
	}
	return false
}

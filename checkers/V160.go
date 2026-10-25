package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV160 регистрирует запрет вложенных тегов в блоках выбора DSL 1.2.
func registrationV160(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V160",
		Severity: diagnostics.SeverityError,
		Message:  "В блоке выбора использован запрещённый вложенный тег.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV160,
	})
}

// checkV160 проверяет сохранённые объявления только относительно ближайшего
// фигурного родителя. Invalid и unparsed не восстанавливаются из исходника.
func checkV160(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v160Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if strings.EqualFold(name, "answer") || strings.EqualFold(name, "hint") {
			continue
		}
		parent := v160Parent(lines, line)
		if parent == nil {
			continue
		}
		parentTag := v160Tag(parent)
		if parentTag == nil {
			continue
		}
		parentName, _ := parentTag.Value()
		if !strings.EqualFold(parentName, "choice") && !strings.EqualFold(parentName, "multichoice") {
			continue
		}
		occurrence, err := findings.Add(number, tag.Start(), number, tag.End(), "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, parent.Number(), parentTag.Start(), parent.Number(), parentTag.End()); err != nil {
			return err
		}
	}
	return nil
}

// v160Tag возвращает только сохранённый начальный tag вне строки invalid.
// Пропуск отступа не декодирует экранирование и не превращает текст в тег.
func v160Tag(line diagnostics.Line) diagnostics.Element {
	if line.Type() == "invalid" {
		return nil
	}
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			return element
		}
	}
	return nil
}

// v160Parent возвращает ближайшую фигурную границу по готовым связям.
// Даже invalid с сохранённым block-open не позволяет перейти к внешнему блоку.
func v160Parent(lines diagnostics.Lines, line diagnostics.Line) diagnostics.Line {
	for number, found := line.ParentLine(); found; number, found = line.ParentLine() {
		line, _ = lines.Line(number)
		if line.Type() == "block-start" {
			return line
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "block-open" {
				return line
			}
		}
	}
	return nil
}

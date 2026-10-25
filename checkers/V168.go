package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV168 регистрирует проверку соседства hint в matching DSL 1.2.
func registrationV168(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V168",
		Severity: diagnostics.SeverityError,
		Message:  "Подсказка @hint не следует непосредственно за ответом элемента верхнего списка.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV168,
	})
}

// checkV168 проверяет непосредственные подсказки до единственного разделителя.
// Для каждой подсказки рассматривает максимум две значимые конструкции.
func checkV168(lines diagnostics.Lines, findings diagnostics.Findings) error {
	separators, counts := make(map[int]int), make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found && strings.Trim(line.Raw(), " \t") == "---" {
			separators[parent] = number
			counts[parent]++
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v168Tag(line, "hint")
		parent, found := line.ParentLine()
		if tag == nil || !found || counts[parent] != 1 || number >= separators[parent] {
			continue
		}
		container, _ := lines.Line(parent)
		if container.Type() != "block-start" || v168Tag(container, "matching") == nil {
			continue
		}
		answer := v168Previous(lines, number, container)
		if answer != nil && v168Tag(answer, "answer") != nil {
			item := v168Previous(lines, answer.Number(), container)
			if item != nil && item.Type() == "content" && v168Element(item, "tag") == nil {
				continue
			}
		}
		if _, err := findings.Add(number, tag.Start(), number, tag.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v168Previous возвращает ближайшую значимую конструкцию того же контекста.
// Только пустые строки пропускаются. Завершённый answer возвращается по строке
// открытия без просмотра тела. nil означает границу или посторонний контейнер.
func v168Previous(lines diagnostics.Lines, before int, container diagnostics.Line) diagnostics.Line {
	for number := before - 1; number > container.Number(); number-- {
		line, _ := lines.Line(number)
		raw := strings.Trim(line.Raw(), " \t")
		if raw == "" {
			continue
		}
		if raw == "---" {
			return nil
		}
		if line.Type() == "block-end" {
			parent, found := line.ParentLine()
			if !found {
				return nil
			}
			opening, _ := lines.Line(parent)
			if v168Direct(opening, container) && v168Tag(opening, "answer") != nil {
				return opening
			}
			return nil
		}
		if !v168Direct(line, container) || line.Type() == "block-start" || v168Element(line, "block-open") != nil {
			return nil
		}
		return line
	}
	return nil
}

// v168Direct возвращает true для непосредственной строки того же уровня.
func v168Direct(line, container diagnostics.Line) bool {
	parent, found := line.ParentLine()
	return found && parent == container.Number() && line.NestingLevel() == container.NestingLevel()+1
}

// v168Tag читает сохранённый tag с указанным именем без восстановления unparsed.
func v168Tag(line diagnostics.Line, name string) diagnostics.Element {
	tag := v168Element(line, "tag")
	if tag != nil {
		value, _ := tag.Value()
		if strings.EqualFold(value, name) {
			return tag
		}
	}
	return nil
}

// v168Element возвращает существующий элемент указанного типа либо nil.
func v168Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

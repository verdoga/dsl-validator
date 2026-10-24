package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV065 регистрирует завершение без активного задания в DSL 1.2.
func registrationV065(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V065",
		Severity: diagnostics.SeverityError,
		Message:  "У @endtask нет активного задания для завершения.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV065,
	})
}

// checkV065 ведёт состояние задания в физическом порядке строк.
// Ветки определяются событиями variant, включая модель с общим родителем
// variants. Внешнее задание при входе в variants не сохраняется.
func checkV065(lines diagnostics.Lines, findings diagnostics.Findings) error {
	active := false
	branches := make(map[int]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		variants, allowed := v065Context(lines, line)
		if !allowed {
			continue
		}
		if line.Type() == "block-end" {
			parent, found := line.ParentLine()
			if found && parent == variants {
				active = false
				delete(branches, variants)
			}
			continue
		}
		name, start, end := v065Tag(line)
		if name == "variant" && variants != 0 {
			active = false
			branches[variants] = true
			continue
		}
		if variants != 0 && !branches[variants] {
			continue
		}
		thirdLevel := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "heading-level" {
				level, _ := element.Value()
				thirdLevel = level == "3"
			}
		}
		if thirdLevel {
			active = false
			continue
		}
		switch name {
		case "task":
			active = true
		case "header", "newpage", "variants":
			active = false
			if name == "variants" && line.Type() == "block-start" {
				branches[number] = false
			}
		case "endtask":
			if !active {
				if _, err := findings.Add(number, start, number, end, ""); err != nil {
					return err
				}
			}
			active = false
		}
	}
	return nil
}

// v065Tag возвращает каноническое имя и исходный диапазон тега.
// При отсутствии tag резервный поиск разрешён только при наличии unparsed.
// Пустое имя означает отсутствие события; колонки с 1, конец исключается.
func v065Tag(line diagnostics.Line) (name string, start, end int) {
	unparsed := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ = element.Value()
			return name, element.Start(), element.End()
		}
		unparsed = unparsed || element.Type() == "unparsed"
	}
	if !unparsed {
		return "", 0, 0
	}
	raw := []rune(line.Raw())
	first := 0
	for first < len(raw) && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	if first == len(raw) || raw[first] != '@' {
		return "", 0, 0
	}
	last := first + 1
	for last < len(raw) && raw[last] != '{' && !unicode.IsSpace(raw[last]) {
		last++
	}
	for _, candidate := range []string{"task", "endtask", "header", "newpage", "variants", "variant"} {
		if strings.EqualFold(string(raw[first+1:last]), candidate) {
			return candidate, first + 1, last + 1
		}
	}
	return "", 0, 0
}

// v065Context возвращает ближайший variants (0 вне него) и допустимость
// структурных событий. Любой другой фигурный предок исключает строку.
// Родители task, step и заголовки не являются фигурными контейнерами.
func v065Context(lines diagnostics.Lines, line diagnostics.Line) (variants int, allowed bool) {
	for {
		parent, found := line.ParentLine()
		if !found {
			return variants, true
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v065Tag(line)
		if name != "variants" {
			return 0, false
		}
		if variants == 0 {
			variants = parent
		}
	}
}

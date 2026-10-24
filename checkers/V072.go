package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v072Position хранит исходные Unicode-координаты имени; конец исключается.
// line=0 означает, что первое объявление ещё не найдено.
type v072Position struct {
	line, start, end int
}

// registrationV072 регистрирует повтор speaking в задании DSL 1.2.
func registrationV072(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V072",
		Severity: diagnostics.SeverityError,
		Message:  "@speaking объявлен в задании повторно.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV072,
	})
}

// checkV072 самостоятельно восстанавливает области заданий в порядке строк.
// Ветви определяются событиями variant, включая модель с общим родителем
// variants. Внешнее задание после variants не возобновляется.
func checkV072(lines diagnostics.Lines, findings diagnostics.Findings) error {
	active := false
	var first v072Position
	branches := make(map[int]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		variants, allowed := v072Context(lines, line)
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
		name, start, end := v072Tag(line)
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
			first = v072Position{}
		case "endtask", "header", "newpage", "variants":
			active = false
			if name == "variants" && line.Type() == "block-start" {
				branches[number] = false
			}
		case "speaking":
			if !active {
				continue
			}
			if first.line == 0 {
				first = v072Position{number, start, end}
				continue
			}
			occurrence, err := findings.Add(number, start, number, end, "")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, first.line, first.start, first.line, first.end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v072Tag возвращает имя события и исходные колонки с 1, конец исключается.
// Пустое имя означает отсутствие события. При отсутствии tag поиск по Raw
// не ограничен типом строки или наличием unparsed; ошибочный хвост сохраняется.
func v072Tag(line diagnostics.Line) (name string, start, end int) {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ = element.Value()
			return name, element.Start(), element.End()
		}
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
	for last < len(raw) && raw[last] != '{' && raw[last] != '}' && !unicode.IsSpace(raw[last]) {
		last++
	}
	name = strings.ToLower(string(raw[first+1 : last]))
	switch name {
	case "task", "endtask", "header", "newpage", "variants", "variant", "speaking":
		return name, first + 1, last + 1
	default:
		return "", 0, 0
	}
}

// v072Context возвращает ближайший variants (0 вне него) и допустимость
// событий. allowed=false означает наличие другого фигурного предка.
// Родители task, step и заголовки не являются фигурными контейнерами.
func v072Context(lines diagnostics.Lines, line diagnostics.Line) (variants int, allowed bool) {
	for {
		parent, found := line.ParentLine()
		if !found {
			return variants, true
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v072Tag(line)
		if name != "variants" {
			return 0, false
		}
		if variants == 0 {
			variants = parent
		}
	}
}

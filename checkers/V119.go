package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV119 регистрирует key вне активного задания только для DSL 1.2.
func registrationV119(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V119",
		Severity: diagnostics.SeverityError,
		Message:  "@key находится вне активного задания.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV119,
	})
}

// v119Branch хранит состояние текущей ветви одного variants.
// started отличает ветвь от запрещённого материала перед первым variant.
type v119Branch struct {
	started bool
	active  bool
}

// checkV119 независимо восстанавливает задания основного потока и ветвей.
// Исходное родительство определяет контейнеры, а порядок variant — границы
// ветвей, в том числе при общем родителе variants у их содержимого.
func checkV119(lines diagnostics.Lines, findings diagnostics.Findings) error {
	mainActive := false
	branches := make(map[int]v119Branch)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		container, allowed := v119Context(lines, line)
		if !allowed {
			continue
		}
		if line.Type() == "block-end" {
			parent, found := line.ParentLine()
			if found && parent == container {
				delete(branches, container)
			}
			continue
		}
		name, start, end := v119Tag(line)
		if container != 0 && name == "variant" {
			branches[container] = v119Branch{started: true}
			continue
		}
		branch := branches[container]
		if container != 0 && !branch.started {
			continue
		}
		active := mainActive
		if container != 0 {
			active = branch.active
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
		} else {
			switch name {
			case "task":
				active = true
			case "endtask", "header", "newpage", "variants":
				active = false
				if name == "variants" && line.Type() == "block-start" {
					branches[number] = v119Branch{}
				}
			case "key":
				if !active {
					if _, err := findings.Add(number, start, number, end, ""); err != nil {
						return err
					}
				}
			}
		}
		if container == 0 {
			mainActive = active
		} else {
			branch.active = active
			branches[container] = branch
		}
	}
	return nil
}

// v119Tag возвращает имя и исходный диапазон распознанного tag.
// Пустое имя означает отсутствие tag; восстановления по Raw нет.
// Колонки начинаются с единицы, правая граница исключена.
func v119Tag(line diagnostics.Line) (name string, start, end int) {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ = element.Value()
			return strings.ToLower(name), element.Start(), element.End()
		}
	}
	return "", 0, 0
}

// v119Context возвращает ближайший variants (0 в основном потоке).
// allowed=false исключает строки внутри любого другого фигурного блока,
// включая литеральные контексты; true разрешает структурные события.
func v119Context(lines diagnostics.Lines, line diagnostics.Line) (container int, allowed bool) {
	for {
		parent, found := line.ParentLine()
		if !found {
			return container, true
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v119Tag(line)
		if name != "variants" {
			return 0, false
		}
		if container == 0 {
			container = parent
		}
	}
}

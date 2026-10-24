package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV067 регистрирует проверку принадлежности этапа заданию в DSL 1.2.
func registrationV067(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V067",
		Severity: diagnostics.SeverityError,
		Message:  "@step находится вне активного задания.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV067,
	})
}

// checkV067 восстанавливает активное задание в порядке исходных строк.
// Состояние не переносится между ветвями и не восстанавливается после variants.
func checkV067(lines diagnostics.Lines, findings diagnostics.Findings) error {
	active, stream := false, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		variants, excluded := v067Context(lines, line)
		if excluded {
			continue
		}
		if variants != stream {
			active, stream = false, variants
		}
		if line.Type() == "block-end" {
			parent, found := line.ParentLine()
			if found && parent == variants {
				active = false
			}
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			value, _ := element.Value()
			if element.Type() == "heading-level" && value == "3" {
				active = false
			}
		}
		name, start, end := v067Tag(line)
		switch name {
		case "task":
			active = true
		case "endtask", "header", "newpage", "variants":
			active = false
		case "variant":
			if variants != 0 {
				active = false
			}
		case "step":
			if !active {
				if _, err := findings.Add(number, start, number, end, ""); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v067Context возвращает ближайший variants и признак исключённого содержимого.
// Любой другой фигурный предок исключает строку, включая вложенные объявления.
func v067Context(lines diagnostics.Lines, line diagnostics.Line) (int, bool) {
	variants := 0
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v067Tag(line)
		if name != "variants" {
			return 0, true
		}
		if variants == 0 {
			variants = parent
		}
	}
	return variants, false
}

// v067Tag читает готовый tag; только в ошибочной строке без tag использует Raw.
// Позиции — исходные Unicode-колонки с 1, конец исключается; пустое имя означает
// отсутствие структурного объявления. Начальное экранирование не декодируется.
func v067Tag(line diagnostics.Line) (string, int, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] == '\\' {
		return "", 0, 0
	}
	erroneous := line.Type() == "invalid"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			value, _ := element.Value()
			return strings.ToLower(value), element.Start(), element.End()
		}
		if element.Type() == "unparsed" {
			erroneous = true
		}
	}
	if !erroneous {
		return "", 0, 0
	}
	for _, name := range []string{"task", "endtask", "header", "newpage", "variants", "variant", "step"} {
		end := start + len(name) + 1
		if end > len(raw) || !strings.EqualFold(string(raw[start:end]), "@"+name) {
			continue
		}
		if end == len(raw) || raw[end] == '{' || raw[end] == '}' || unicode.Is(unicode.White_Space, raw[end]) {
			return name, start + 1, end + 1
		}
	}
	return "", 0, 0
}

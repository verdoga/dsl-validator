package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV071 регистрирует проверку позиции первого speaking в DSL 1.2.
func registrationV071(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V071",
		Severity: diagnostics.SeverityError,
		Message:  "Перед @speaking внутри задания есть другая значимая конструкция.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV071,
	})
}

// checkV071 хранит первое препятствие и факт первого speaking только в текущем
// задании. Границы внутри контентных блоков не меняют состояние задания.
func checkV071(lines diagnostics.Lines, findings diagnostics.Findings) error {
	active, spoken := false, false
	stream, obstacle := 0, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		context := v071Context(lines, line)
		if context.literal {
			continue
		}
		if context.variants != stream {
			active, spoken, obstacle = false, false, 0
			stream = context.variants
		}
		name, start, end := v071Tag(line)
		if !context.content {
			if name == "task" {
				active, spoken, obstacle = true, false, 0
				continue
			}
			closes := name == "endtask" || name == "header" || name == "newpage" ||
				name == "variants" || name == "variant" && context.variants != 0
			if line.Type() == "block-end" {
				parent, found := line.ParentLine()
				closes = closes || found && parent == context.variants
			}
			for index := 0; index < line.ElementCount(); index++ {
				element, _ := line.Element(index)
				value, _ := element.Value()
				closes = closes || element.Type() == "heading-level" && value == "3"
			}
			if closes {
				active, spoken, obstacle = false, false, 0
				continue
			}
		}
		if !active || spoken {
			continue
		}
		if name == "speaking" {
			spoken = true
			if obstacle != 0 {
				occurrence, err := findings.Add(number, start, number, end, "")
				if err != nil {
					return err
				}
				first, _ := lines.Line(obstacle)
				if err := findings.AddRelated(occurrence, obstacle, 1, obstacle, utf8.RuneCountInString(first.Raw())+1); err != nil {
					return err
				}
			}
			continue
		}
		if line.Type() == "blank" || strings.Trim(line.Raw(), " \t") == "" ||
			!context.content && v071Comment(line) {
			continue
		}
		if obstacle == 0 {
			obstacle = number
		}
	}
	return nil
}

// v071Parents описывает внешний поток и ограничения фигурных предков строки.
type v071Parents struct {
	variants int
	content  bool
	literal  bool
}

// v071Context читает готовые связи родителей. Вложенный в контент variants
// не создаёт поток; после такого предка учитывается лишь внешний variants.
func v071Context(lines diagnostics.Lines, line diagnostics.Line) v071Parents {
	var context v071Parents
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v071Tag(line)
		if name == "variants" {
			context.variants = parent
		} else {
			context.content = true
			context.variants = 0
			if name == "editor" || name == "multifill" {
				context.literal = true
			}
		}
	}
	return context
}

// v071Comment распознаёт готовое объявление editor без неразобранного остатка.
// Допустимость его расположения проверяется вызывающим кодом по предкам.
func v071Comment(line diagnostics.Line) bool {
	if line.Type() != "tag" && line.Type() != "block-start" {
		return false
	}
	editor := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "unparsed" {
			return false
		}
		value, _ := element.Value()
		editor = editor || element.Type() == "tag" && strings.EqualFold(value, "editor")
	}
	return editor
}

// v071Tag читает готовое имя; при invalid/unparsed без tag восстанавливает
// только нужные структурные имена. Координаты исходные, с 1, конец исключается.
func v071Tag(line diagnostics.Line) (string, int, int) {
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
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if erroneous {
		for _, name := range []string{"task", "endtask", "header", "newpage", "variants", "variant", "speaking"} {
			end := start + len(name) + 1
			if end <= len(raw) && strings.EqualFold(string(raw[start:end]), "@"+name) &&
				(end == len(raw) || raw[end] == '{' || raw[end] == '}' || unicode.Is(unicode.White_Space, raw[end])) {
				return name, start + 1, end + 1
			}
		}
	}
	return "", 0, 0
}

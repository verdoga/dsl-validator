package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v134Tag хранит имя и исходный диапазон тега с исключающей правой границей.
type v134Tag struct {
	name             string
	line, start, end int
}

// registrationV134 регистрирует проверку размещения include только для DSL 1.2.
func registrationV134(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V134", Severity: diagnostics.SeverityError,
		Message:  "@include находится в месте, где вставки запрещены.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV134,
	})
}

// checkV134 учитывает порядок заголовков и ветвей, не раскрывая включения.
// Задания и этапы не снимают запреты содержащих их фигурных блоков.
func checkV134(lines diagnostics.Lines, findings diagnostics.Findings) error {
	heading := false
	branches := make(map[int]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v134Declaration(line)
		parent, variants, literal := v134Context(lines, line, branches, tag.name == "variant")
		if literal {
			continue
		}
		if line.Type() == "heading" && line.NestingLevel() == 0 {
			for index := 0; index < line.ElementCount(); index++ {
				element, _ := line.Element(index)
				value, _ := element.Value()
				if element.Type() == "heading-level" && value == "1" {
					heading = true
				}
			}
		}
		if line.Type() == "block-end" {
			opening, _ := line.ParentLine()
			delete(branches, opening)
			continue
		}
		if line.Type() == "block-start" && tag.name == "variants" {
			branches[number] = false
		}
		// Родитель с меньшим номером находится снаружи variants и не мешает
		// началу ветви. Запрет этого внешнего блока сохраняется для include.
		if tag.name == "variant" && variants != 0 && parent.line < variants {
			branches[variants] = true
		}
		if tag.name != "include" || heading && parent.line == 0 {
			continue
		}
		detail := ""
		if parent.name != "" {
			detail = "Родительский блок: @" + parent.name + "."
		}
		occurrence, err := findings.Add(number, tag.start, number, tag.end, detail)
		if err != nil {
			return err
		}
		if parent.line != 0 {
			if err := findings.AddRelated(occurrence, parent.line, parent.start, parent.line, parent.end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v134Declaration читает начальный tag, а в ошибочной строке без tag — имя
// после отступа U+0020/U+0009 до пробела, табуляции, { или конца строки.
// Экранирование и содержимое строки не преобразуются; пустое имя — не тег.
func v134Declaration(line diagnostics.Line) v134Tag {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return v134Tag{}
	}
	erroneous, hasTag := line.Type() == "invalid", false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			hasTag = true
			if element.Start() == start+1 {
				name, _ := element.Value()
				return v134Tag{strings.ToLower(name), line.Number(), element.Start(), element.End()}
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if hasTag || !erroneous {
		return v134Tag{}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' {
		end++
	}
	return v134Tag{strings.ToLower(raw[start+1 : end]), line.Number(), start + 1, utf8.RuneCountInString(raw[:end]) + 1}
}

// v134Context возвращает ближайший запрещающий блок, ближайший variants
// и признак литерального содержимого. При начале ветви её собственный variants
// не запрещает объявление из-за отсутствия предыдущей ветви.
func v134Context(lines diagnostics.Lines, line diagnostics.Line, branches map[int]bool, branchStart bool) (forbidden v134Tag, variants int, literal bool) {
	for number, found := line.ParentLine(); found; number, found = line.ParentLine() {
		line, _ = lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		tag := v134Declaration(line)
		if tag.name == "editor" || tag.name == "multifill" {
			return forbidden, variants, true
		}
		if tag.name == "variants" {
			first := variants == 0
			if first {
				variants = number
			}
			if branches[number] || branchStart && first {
				continue
			}
		}
		if forbidden.line == 0 {
			forbidden = tag
			if forbidden.name == "" {
				// Даже неразобранное имя не отменяет существующий блок модели.
				forbidden = v134Tag{"", number, 1, utf8.RuneCountInString(line.Raw()) + 1}
			}
		}
	}
	return forbidden, variants, false
}

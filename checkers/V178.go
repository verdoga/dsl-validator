package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v178Previous описывает одну значимую непосредственную конструкцию.
// end — физическая строка её завершения; тело блока не хранится.
type v178Previous struct {
	kind string
	end  int
}

// registrationV178 регистрирует расположение подсказок ordering только для DSL 1.2.
func registrationV178(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V178", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeElement,
		Message:  "Подсказка @hint находится вне допустимого места после ответа элемента @ordering.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV178,
	})
}

// checkV178 проверяет физически непосредственные строки ordering.
// Логический родитель после heading не меняет принадлежность фигурному блоку.
func checkV178(lines diagnostics.Lines, findings diagnostics.Findings) error {
	blocks := v178Index(lines)
	for _, block := range blocks {
		if block.opening == nil || block.literal {
			continue
		}
		if name, _ := v178Tag(block.opening); name != "ordering" {
			continue
		}
		if err := v178CheckBlock(block.body, blocks, findings); err != nil {
			return err
		}
	}
	return nil
}

// v178Block хранит открытие, непосредственные физические строки и закрытие.
// end=0 означает незавершённый блок; literal обозначает editor/multifill.
type v178Block struct {
	opening diagnostics.Line
	body    []diagnostics.Line
	end     int
	literal bool
}

// v178Index строит физическое владение по распознанным границам за один проход.
// Дополнительно границей служит отдельная } в invalid/unparsed. Произвольные
// скобки в тексте не разбираются. Heading не снимает открытые блоки; вложенный
// блок входит в тело одной строкой. В editor/multifill закрывает только строка }.
func v178Index(lines diagnostics.Lines) []v178Block {
	blocks := make([]v178Block, lines.Len()+1)
	var active []int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		owner := 0
		if len(active) != 0 {
			owner = active[len(active)-1]
		}
		raw := strings.Trim(line.Raw(), " \t")
		closes := line.Type() == "block-end" || v178HasElement(line, "block-close") ||
			raw == "}" && (line.Type() == "invalid" || v178HasElement(line, "unparsed"))
		if blocks[owner].literal {
			closes = raw == "}"
		}
		if owner != 0 && closes {
			blocks[owner].end = number
			active = active[:len(active)-1]
			continue
		}
		if owner != 0 {
			blocks[owner].body = append(blocks[owner].body, line)
		}
		if !blocks[owner].literal && (line.Type() == "block-start" || v178HasElement(line, "block-open")) {
			name, _ := v178Tag(line)
			blocks[number].opening = line
			blocks[number].literal = name == "editor" || name == "multifill"
			active = append(active, number)
		}
	}
	return blocks
}

// v178CheckBlock рассматривает только непосредственные строки одного ordering.
// Хранятся не более двух предыдущих значимых конструкций; разделитель их сбрасывает.
func v178CheckBlock(body []diagnostics.Line, blocks []v178Block, findings diagnostics.Findings) error {
	separators := 0
	for _, line := range body {
		if strings.Trim(line.Raw(), " \t") == "---" {
			separators++
		}
	}
	if separators > 1 {
		return nil
	}
	var older, previous v178Previous
	afterSeparator := false
	for _, line := range body {
		raw := strings.Trim(line.Raw(), " \t")
		if raw == "" {
			continue
		}
		if raw == "---" {
			afterSeparator = true
			older, previous = v178Previous{}, v178Previous{}
			continue
		}
		name, tag := v178Tag(line)
		if name == "hint" {
			detail := ""
			switch {
			case afterSeparator:
				detail = "Подсказка расположена после разделителя списков."
			case previous.kind == "hint" && previous.end < line.Number():
				detail = "Подсказка повторяется после предыдущего @hint."
			case previous.kind != "answer" || previous.end >= line.Number() || older.kind != "item":
				detail = "Нет непосредственно предшествующего ответа, связанного со строкой элемента."
			}
			if detail != "" {
				if _, err := findings.Add(line.Number(), tag.Start(), line.Number(), tag.End(), detail); err != nil {
					return err
				}
			}
		}
		current := v178Previous{end: line.Number()}
		switch {
		case line.Type() == "content" && tag == nil && blocks[line.Number()].opening == nil:
			current.kind = "item"
		case name == "answer" || name == "hint":
			if blocks[line.Number()].opening != nil && blocks[line.Number()].end > line.Number() {
				current.kind, current.end = name, blocks[line.Number()].end
			} else if line.Type() == "tag" && blocks[line.Number()].opening == nil {
				current.kind = name
			}
		}
		older, previous = previous, current
	}
	return nil
}

// v178HasElement проверяет наличие распознанной границы без разбора Raw.
func v178HasElement(line diagnostics.Line, kind string) bool {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == kind {
			return true
		}
	}
	return false
}

// v178Tag читает распознанный элемент tag; литеральный Raw повторно не разбирается.
func v178Tag(line diagnostics.Line) (string, diagnostics.Element) {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.ToLower(name), element
		}
	}
	return "", nil
}

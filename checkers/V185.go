package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v185Tag хранит каноническое имя и исходный диапазон объявления.
// Колонки считаются с единицы в кодовых точках Unicode; конец исключается.
type v185Tag struct {
	name             string
	line, start, end int
}

// registrationV185 регистрирует запрет вложенных тегов ordering для DSL 1.2.
func registrationV185(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V185", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Внутри @ordering использован запрещённый вложенный тег.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV185,
	})
}

// checkV185 проверяет объявления относительно ближайшего физического блока.
// Корневой heading меняет логических родителей, но не закрывает фигурный блок.
func checkV185(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var blocks []v185Tag
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		literal := len(blocks) > 0 && (blocks[len(blocks)-1].name == "editor" || blocks[len(blocks)-1].name == "multifill")
		if v185Closing(line, literal) {
			closing := len(blocks) - 1
			if parent, found := line.ParentLine(); found && !literal && line.Type() == "block-end" {
				for index := len(blocks) - 1; index >= 0; index-- {
					if blocks[index].line == parent {
						closing = index
						break
					}
				}
			}
			if closing >= 0 {
				blocks = blocks[:closing]
			}
			continue
		}
		if literal {
			continue
		}
		child := v185Declaration(line)
		if len(blocks) > 0 && blocks[len(blocks)-1].name == "ordering" &&
			child.name != "" && child.name != "answer" && child.name != "hint" && child.name != "include" {
			parent := blocks[len(blocks)-1]
			occurrence, err := findings.Add(child.line, child.start, child.line, child.end,
				"Запрещённый тег: @"+child.name+".")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, parent.line, parent.start, parent.line, parent.end); err != nil {
				return err
			}
		}
		if v185Opening(line) {
			// Даже нераспознанное имя сохраняет ближайшую физическую границу.
			child.line = line.Number()
			blocks = append(blocks, child)
		}
	}
	return nil
}

// v185Declaration читает начальное объявление, не декодируя Raw.
// Без tag восстановление разрешено только для ошибочных строк. Границы имени:
// конец строки, U+0020, U+0009, { или }. Пустое имя означает отсутствие объявления.
func v185Declaration(line diagnostics.Line) v185Tag {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return v185Tag{}
	}
	erroneous, hasTag := line.Type() == "invalid", false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			hasTag = true
			name, _ := element.Value()
			name = strings.ToLower(name)
			if element.Start() == start+1 && v185Known(name) {
				return v185Tag{name, line.Number(), element.Start(), element.End()}
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if hasTag || !erroneous {
		return v185Tag{}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	name := strings.ToLower(raw[start+1 : end])
	if !v185Known(name) {
		return v185Tag{}
	}
	return v185Tag{name, line.Number(), start + 1, utf8.RuneCountInString(raw[:end]) + 1}
}

// v185Known ограничивает проверку известными именами DSL 1.2.
func v185Known(name string) bool {
	switch name {
	case "dsl-version", "document-id", "section", "order", "resource-dir",
		"header", "task", "endtask", "step", "speaking", "newpage", "editor",
		"media", "example", "wordlist", "table", "script", "text", "key", "instr",
		"note", "alt", "hint", "fragment", "include", "answer", "question",
		"multifill", "choice", "multichoice", "matching", "ordering", "variants", "variant":
		return true
	}
	return false
}

// v185Closing учитывает отдельную } и реальный block-close, в том числе в invalid.
// В литеральном блоке закрывает только отдельная }; false означает отсутствие закрытия.
func v185Closing(line diagnostics.Line, literal bool) bool {
	if strings.Trim(line.Raw(), " \t") == "}" {
		return true
	}
	if literal {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-close" {
			return true
		}
	}
	return false
}

// v185Opening распознаёт готовый block-open независимо от типа строки.
// false означает отсутствие открытия; произвольные скобки текста не учитываются.
func v185Opening(line diagnostics.Line) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			return true
		}
	}
	return false
}

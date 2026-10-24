package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v076Tag хранит каноническое имя и исходный диапазон объявления.
// Колонки начинаются с единицы; конец диапазона исключается.
type v076Tag struct {
	name             string
	line, start, end int
}

// registrationV076 регистрирует запрет вложенных тегов только для DSL 1.2.
func registrationV076(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V076",
		Severity: diagnostics.SeverityError,
		Message:  "Вложенный тег запрещён в этом блоке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV076,
	})
}

// checkV076 проверяет каждое объявление только относительно ближайшего блока.
// Литеральное содержимое editor и multifill исключается по всей цепочке предков.
func checkV076(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		child := v076Declaration(line)
		if child.name == "" {
			continue
		}
		parent, literal := v076Parent(lines, line)
		if literal || !v076Forbidden(parent.name, child.name) {
			continue
		}
		occurrence, err := findings.Add(child.line, child.start, child.line, child.end,
			"Вложенный тег: @"+child.name+". Родительский блок: @"+parent.name+".")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, parent.line, parent.start, parent.line, parent.end); err != nil {
			return err
		}
	}
	return nil
}

// v076Declaration читает известный начальный tag, включая частичный разбор.
// Без tag восстанавливает только ошибочную строку по уточнению A: после
// начальных U+0020/U+0009 имя завершается концом строки, U+0020/U+0009 или {.
// Пустое имя означает отсутствие объявления. Raw не декодируется.
func v076Declaration(line diagnostics.Line) v076Tag {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return v076Tag{}
	}
	erroneous := line.Type() == "invalid"
	hasTag := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			hasTag = true
			name, _ := element.Value()
			name = strings.ToLower(name)
			if element.Start() == start+1 && v076Known(name) {
				return v076Tag{name, line.Number(), element.Start(), element.End()}
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if hasTag || !erroneous {
		return v076Tag{}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' {
		end++
	}
	name := strings.ToLower(raw[start+1 : end])
	if !v076Known(name) {
		return v076Tag{}
	}
	return v076Tag{name, line.Number(), start + 1, utf8.RuneCountInString(raw[:end]) + 1}
}

// v076Known ограничивает восстановление именами DSL 1.2.
func v076Known(name string) bool {
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

// v076Parent возвращает ближайший block-start и признак литерального контекста.
// Неизвестный ближайший блок не заменяется более дальним известным предком.
func v076Parent(lines diagnostics.Lines, line diagnostics.Line) (nearest v076Tag, literal bool) {
	foundBlock := false
	for {
		number, found := line.ParentLine()
		if !found {
			return nearest, false
		}
		line, _ = lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		tag := v076Declaration(line)
		if tag.name == "editor" || tag.name == "multifill" {
			return nearest, true
		}
		if !foundBlock {
			nearest, foundBlock = tag, true
		}
	}
}

// v076Forbidden применяет запрет ближайшего блока и исключение instr/hint.
func v076Forbidden(parent, child string) bool {
	switch parent {
	case "example", "wordlist", "table", "script", "text", "key", "note", "alt", "hint":
		return true
	case "instr":
		return child != "hint"
	}
	return false
}

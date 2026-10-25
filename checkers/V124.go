package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v124Tag хранит каноническое имя и исходный диапазон объявления.
// Колонки начинаются с единицы; конец диапазона исключается.
type v124Tag struct {
	name             string
	line, start, end int
}

// registrationV124 регистрирует запрет вложенных тегов только для DSL 1.2.
func registrationV124(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V124",
		Severity: diagnostics.SeverityError,
		Message:  "Вложенный тег запрещён в этом блоке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV124,
	})
}

// checkV124 проверяет каждое объявление только относительно ближайшего блока.
// Литеральное содержимое editor и multifill исключается по всей цепочке предков.
func checkV124(lines diagnostics.Lines, findings diagnostics.Findings) error {
	blocks := v196Blocks(lines)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		child := v124Declaration(line)
		if child.name == "" || child.name == "include" {
			continue
		}
		parent, literal := v124Parent(lines, line)
		if child.name == "newpage" {
			// V196 передаёт запрет ближайшему физическому контейнеру.
			// Логический переход страницы не меняет фигурных границ.
			if container, known := v196Container(blocks, number); known {
				parent = v124Tag{container.name, container.open, container.start, container.end}
				literal = false
			}
		}
		if literal || !v124Forbidden(parent.name, child.name) {
			continue
		}
		occurrence, err := findings.Add(child.line, child.start, child.line, child.end,
			"Тег @"+child.name+" запрещён внутри блока @"+parent.name+".")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, parent.line, parent.start, parent.line, parent.end); err != nil {
			return err
		}
	}
	return nil
}

// v124Declaration читает известный начальный tag, включая частичный разбор.
// Без tag восстанавливает только ошибочную строку: после
// начальных U+0020/U+0009 имя завершается концом строки, U+0020/U+0009 или {.
// Пустое имя означает отсутствие объявления. Raw не декодируется.
func v124Declaration(line diagnostics.Line) v124Tag {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return v124Tag{}
	}
	erroneous := line.Type() == "invalid"
	hasTag := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			hasTag = true
			name, _ := element.Value()
			name = strings.ToLower(name)
			if element.Start() == start+1 && v124Known(name) {
				return v124Tag{name, line.Number(), element.Start(), element.End()}
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if hasTag || !erroneous {
		return v124Tag{}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' {
		end++
	}
	name := strings.ToLower(raw[start+1 : end])
	if !v124Known(name) {
		return v124Tag{}
	}
	return v124Tag{name, line.Number(), start + 1, utf8.RuneCountInString(raw[:end]) + 1}
}

// v124Known ограничивает восстановление именами DSL 1.2.
func v124Known(name string) bool {
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

// v124Parent возвращает ближайший block-start и признак литерального контекста.
// Неизвестный ближайший блок не заменяется более дальним известным предком.
func v124Parent(lines diagnostics.Lines, line diagnostics.Line) (nearest v124Tag, literal bool) {
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
		tag := v124Declaration(line)
		if tag.name == "editor" || tag.name == "multifill" {
			return nearest, true
		}
		if !foundBlock {
			nearest, foundBlock = tag, true
		}
	}
}

// v124Forbidden проверяет разрешения ближайшего блока; include исключён вызывающим кодом.
func v124Forbidden(parent, child string) bool {
	switch parent {
	case "example", "wordlist", "table", "script", "text", "key", "note", "alt", "hint":
		return true
	case "instr":
		return child != "hint"
	case "fragment":
		switch child {
		case "media", "example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint":
			return false
		}
		return true
	}
	return false
}

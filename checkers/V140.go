package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v140Tag сохраняет имя и исходный диапазон тега в Unicode-колонках с 1.
// Конец диапазона исключается; пустое имя означает отсутствие объявления.
type v140Tag struct {
	name             string
	line, start, end int
}

// registrationV140 регистрирует запрет вложенных тегов в answer для DSL 1.2.
// Конкретизация не предусмотрена.
func registrationV140(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V140",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри @answer записан вложенный тег.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV140,
	})
}

// checkV140 проверяет объявления относительно ближайшего фигурного родителя.
// Содержимое запрещённого вложенного блока не относится повторно к внешнему answer.
func checkV140(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		child := v140Declaration(line)
		if child.name == "" {
			continue
		}
		parent, literal := v140Parent(lines, line)
		if literal || parent.name != "answer" {
			continue
		}
		occurrence, err := findings.Add(child.line, child.start, child.line, child.end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, parent.line, parent.start, parent.line, parent.end); err != nil {
			return err
		}
	}
	return nil
}

// v140Declaration читает начальный сохранённый tag либо точное имя из Raw.
// По уточнению C без tag имя заканчивается только на U+0020 или конце строки.
// Начальные U+0020/U+0009 — отступ; экранирование и содержимое не преобразуются.
func v140Declaration(line diagnostics.Line) v140Tag {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return v140Tag{}
	}
	hasTag := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		hasTag = true
		name, _ := element.Value()
		name = strings.ToLower(name)
		if element.Start() == start+1 && v140Known(name) {
			return v140Tag{name, line.Number(), element.Start(), element.End()}
		}
	}
	if hasTag {
		return v140Tag{}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' {
		end++
	}
	name := strings.ToLower(string(raw[start+1 : end]))
	if !v140Known(name) {
		return v140Tag{}
	}
	return v140Tag{name, line.Number(), start + 1, end + 1}
}

// v140Known возвращает true только для известных имён тегов DSL 1.2.
func v140Known(name string) bool {
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

// v140Parent находит ближайший block-start по сохранённой цепочке родителей.
// Неизвестное имя ближайшего блока не позволяет перейти к более дальнему answer.
// literal=true исключает псевдотеги внутри editor/multifill; иначе literal=false.
func v140Parent(lines diagnostics.Lines, line diagnostics.Line) (nearest v140Tag, literal bool) {
	foundBlock := false
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		tag := v140Declaration(line)
		if tag.name == "editor" || tag.name == "multifill" {
			return nearest, true
		}
		if !foundBlock {
			nearest, foundBlock = tag, true
		}
	}
	return nearest, false
}

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
			// При неполных границах сохраняем лишь подтверждённого родителя.
			container, known := v196Container(blocks, number)
			if known {
				opening, _ := lines.Line(container.open)
				parent = v124Declaration(opening)
			} else if !v124NewpageLogicalParent(lines, blocks, number, parent.line) {
				continue
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

// v124NewpageLogicalParent подтверждает логического родителя при неполных
// физических границах. true означает ближайшее незакрытое объявление без
// литерального контекста и пересечений активных интервалов; иначе false.
// Первая связанная скобка завершает интервал: её последующий дубликат
// не отменяет вложенность перед ней и не открывает блок заново после неё.
func v124NewpageLogicalParent(lines diagnostics.Lines, blocks []v196Block, number, parent int) bool {
	if parent == 0 {
		return false
	}
	ends := make(map[int]int)
	for current := 1; current <= lines.Len(); current++ {
		line, _ := lines.Line(current)
		if line.Type() != "block-end" {
			continue
		}
		opening, _ := line.ParentLine()
		if ends[opening] == 0 {
			ends[opening] = current
		}
	}
	nearest, end := 0, 0
	for _, block := range blocks {
		if block.open >= number {
			break
		}
		closing := ends[block.open]
		if closing != 0 && closing <= number {
			continue
		}
		opening, _ := lines.Line(block.open)
		tag := v124Declaration(opening)
		if tag.name == "editor" || tag.name == "multifill" {
			return false
		}
		if closing != 0 {
			if end != 0 && closing >= end {
				return false
			}
			end = closing
		}
		nearest = block.open
	}
	return nearest == parent
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

package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v063ContentState различает доказанную пустоту, содержимое и неопределённость.
type v063ContentState uint8

const (
	v063Empty v063ContentState = iota
	v063Filled
	v063Unknown
)

// registrationV063 регистрирует проверку непустоты задания только для DSL 1.2.
func registrationV063(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V063",
		Severity: diagnostics.SeverityError,
		Message:  "Задание не содержит учебного содержимого.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV063,
	})
}

// checkV063 обходит структурный поток, включая последовательность ветвей.
// Последняя строка области предшествует завершающей конструкции; конец
// диапазона исключается и не включает EOL. Конкретизация не предусмотрена.
func checkV063(lines diagnostics.Lines, findings diagnostics.Findings) error {
	start, column := 0, 0
	for number := 1; number <= lines.Len()+1; number++ {
		var task diagnostics.Element
		if number <= lines.Len() {
			line, _ := lines.Line(number)
			if !v063Boundary(lines, line) {
				continue
			}
			if v063Tag(line) == "task" {
				task = v063Element(line, "tag")
			}
		}
		if start != 0 && v063Content(lines, start+1, number, 0, make(map[int]v063ContentState)) == v063Empty {
			last, _ := lines.Line(number - 1)
			if _, err := findings.Add(start, column, number-1,
				utf8.RuneCountInString(last.Raw())+1, ""); err != nil {
				return err
			}
		}
		start = 0
		if task != nil {
			start, column = number, task.Start()
		}
	}
	return nil
}

// v063Boundary возвращает true для границы структурного потока.
// Фигурные контентные предки исключают границу, variants сохраняет её.
// Физический порядок variant учитывает и модель с общим родителем variants.
func v063Boundary(lines diagnostics.Lines, line diagnostics.Line) bool {
	variants := 0
	for ancestor := line; ; {
		parent, found := ancestor.ParentLine()
		if !found {
			break
		}
		ancestor, _ = lines.Line(parent)
		if ancestor.Type() == "block-start" {
			if v063Tag(ancestor) != "variants" {
				return false
			}
			if variants == 0 {
				variants = parent
			}
		}
	}
	if line.Type() == "block-end" {
		parent, _ := line.ParentLine()
		return variants != 0 && parent == variants
	}
	if line.Type() == "heading" {
		if level := v063Element(line, "heading-level"); level != nil {
			value, _ := level.Value()
			return value == "3"
		}
	}
	switch v063Tag(line) {
	case "task", "endtask", "header", "newpage", "variants":
		return true
	case "variant":
		return variants != 0
	}
	return false
}

// v063Element возвращает единственный элемент типа; nil означает отсутствие
// или неоднозначность. Raw не разбирается повторно.
func v063Element(line diagnostics.Line, kind string) diagnostics.Element {
	var result diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if result != nil {
				return nil
			}
			result = element
		}
	}
	return result
}

// v063Tag читает имя распознанного объявления без учёта регистра.
func v063Tag(line diagnostics.Line) string {
	if line.Type() == "tag" || line.Type() == "block-start" {
		if element := v063Element(line, "tag"); element != nil {
			value, _ := element.Value()
			return strings.ToLower(value)
		}
	}
	return ""
}

// v063Hidden исключает editor и определения fragment целиком.
// При исследовании включения его корень fragment не скрывает своё тело.
func v063Hidden(lines diagnostics.Lines, line diagnostics.Line, fragment int) bool {
	for line.Number() != fragment {
		if tag := v063Tag(line); tag == "editor" || tag == "fragment" {
			return true
		}
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
	}
	return false
}

// v063Descendant возвращает true только для потомка указанной строки.
func v063Descendant(lines diagnostics.Lines, line diagnostics.Line, ancestor int) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		if parent == ancestor {
			return true
		}
		line, _ = lines.Line(parent)
	}
}

// v063Content исследует физические строки [first, end), end исключается.
// Ненулевой fragment ограничивает обход потомками включаемого определения.
// resolved принадлежит одному заданию; неопределённость не отменяет
// достоверно найденного содержимого.
func v063Content(lines diagnostics.Lines, first, end, fragment int, resolved map[int]v063ContentState) v063ContentState {
	result := v063Empty
	for number := first; number < end; number++ {
		line, _ := lines.Line(number)
		if fragment != 0 && !v063Descendant(lines, line, fragment) ||
			v063Hidden(lines, line, fragment) {
			continue
		}
		state := v063LineContent(lines, line, resolved)
		if state == v063Filled {
			return state
		}
		if state == v063Unknown {
			result = state
		}
	}
	return result
}

// v063LineContent не оценивает качество отображаемой конструкции и не
// превращает параметры служебных объявлений в обычный текст.
func v063LineContent(lines diagnostics.Lines, line diagnostics.Line, resolved map[int]v063ContentState) v063ContentState {
	switch line.Type() {
	case "blank", "block-end", "separator", "heading":
		return v063Empty
	case "content", "invalid":
		result, hasText := v063Empty, false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "unparsed" {
				result = v063Unknown
			}
			if element.Type() == "content" {
				value, present := element.Value()
				hasText = hasText || present
				if present && strings.ContainsFunc(value, func(r rune) bool {
					return !unicode.Is(unicode.White_Space, r)
				}) {
					return v063Filled
				}
			}
		}
		if !hasText {
			return v063Unknown
		}
		return result
	case "tag", "block-start":
		switch v063Tag(line) {
		case "step", "media", "instr", "example", "wordlist", "table", "script", "text",
			"key", "note", "alt", "hint", "question", "multifill", "choice",
			"multichoice", "matching", "ordering":
			return v063Filled
		case "include":
			return v063IncludedContent(lines, line, resolved)
		case "task", "speaking", "editor", "fragment", "endtask", "header",
			"newpage", "variants", "variant", "dsl-version", "document-id",
			"section", "order", "resource-dir", "answer":
			return v063Empty
		}
	}
	return v063Unknown
}

// v063IncludedContent ищет единственное определение по всему документу.
// ID сравниваются через EqualFold без обрезки и повторного декодирования.
// Отсутствие, неоднозначность и цикл не доказывают пустоту.
func v063IncludedContent(lines diagnostics.Lines, include diagnostics.Line, resolved map[int]v063ContentState) v063ContentState {
	identifier := v063Element(include, "identifier")
	if identifier == nil {
		return v063Unknown
	}
	id, present := identifier.Value()
	if !present || id == "" {
		return v063Unknown
	}
	definition := 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" || v063Tag(line) != "fragment" {
			continue
		}
		candidate := v063Element(line, "identifier")
		if candidate == nil {
			continue
		}
		name, found := candidate.Value()
		if found && strings.EqualFold(id, name) {
			if definition != 0 {
				return v063Unknown
			}
			definition = number
		}
	}
	if definition == 0 {
		return v063Unknown
	}
	if state, found := resolved[definition]; found {
		return state
	}
	// Повторный вход до завершения исследования — цикл, после завершения —
	// обычное повторное включение с уже известным результатом.
	resolved[definition] = v063Unknown
	state := v063Content(lines, definition+1, lines.Len()+1, definition, resolved)
	resolved[definition] = state
	return state
}

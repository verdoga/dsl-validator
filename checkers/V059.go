package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v059ContentState различает доказанную пустоту, содержимое и неопределённость.
type v059ContentState uint8

const (
	v059Empty v059ContentState = iota
	v059Filled
	v059Unknown
)

// v059Boundary хранит вид границы и исходную Unicode-колонку маркера с единицы.
type v059Boundary struct {
	line, column int
	heading      bool
}

// registrationV059 регистрирует пустой шаг между ### и @newpage для DSL 1.2.
func registrationV059(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V059",
		Severity: diagnostics.SeverityError,
		Message:  "Между @newpage и заголовком шага образован пустой шаг.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV059,
	})
}

// checkV059 рассматривает соседние границы разных видов в физическом порядке.
// Диапазон начинается с первого маркера и заканчивается после последнего
// символа Raw второй границы; конец исключается, EOL в него не входит.
func checkV059(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var previous v059Boundary
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		current, found := v059StepBoundary(lines, line)
		if !found {
			continue
		}
		if previous.line != 0 && previous.heading != current.heading &&
			v059Content(lines, previous.line+1, current.line, 0, make(map[int]v059ContentState)) == v059Empty {
			if _, err := findings.Add(previous.line, previous.column,
				current.line, utf8.RuneCountInString(line.Raw())+1, ""); err != nil {
				return err
			}
		}
		previous = current
	}
	return nil
}

// v059StepBoundary возвращает распознанную границу вне фигурных блоков.
// false означает, что строка не является такой границей. По уточнению A
// unparsed-остаток не отменяет достоверно распознанные тип строки и маркер.
func v059StepBoundary(lines diagnostics.Lines, line diagnostics.Line) (v059Boundary, bool) {
	for ancestor := line; ; {
		parent, found := ancestor.ParentLine()
		if !found {
			break
		}
		ancestor, _ = lines.Line(parent)
		if ancestor.Type() == "block-start" {
			return v059Boundary{}, false
		}
	}
	var marker diagnostics.Element
	switch line.Type() {
	case "heading":
		marker = v059Element(line, "heading-level")
		if marker == nil {
			return v059Boundary{}, false
		}
		if level, _ := marker.Value(); level != "3" {
			return v059Boundary{}, false
		}
	case "tag":
		if v059Tag(line) != "newpage" {
			return v059Boundary{}, false
		}
		marker = v059Element(line, "tag")
	default:
		return v059Boundary{}, false
	}
	return v059Boundary{line.Number(), marker.Start(), line.Type() == "heading"}, true
}

// v059Element возвращает единственный элемент типа; nil означает отсутствие
// или неоднозначность. Содержимое исходника не разбирается заново.
func v059Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v059Tag читает каноническое имя распознанного объявления, иначе возвращает "".
func v059Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" {
		return ""
	}
	if tag := v059Element(line, "tag"); tag != nil {
		name, _ := tag.Value()
		return name
	}
	return ""
}

// v059Hidden возвращает true для editor, определения fragment и их потомков.
// При исследовании включения fragment указывает корень исследуемого тела:
// именно это определение не скрывает свои строки. false означает видимый контекст.
func v059Hidden(lines diagnostics.Lines, line diagnostics.Line, fragment int) bool {
	for line.Number() != fragment {
		if tag := v059Tag(line); tag == "editor" || tag == "fragment" {
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

// v059Descendant возвращает true только для потомка указанной строки.
func v059Descendant(lines diagnostics.Lines, line diagnostics.Line, ancestor int) bool {
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

// v059Content исследует [first, end) по физическим номерам; end исключается.
// Ненулевой fragment ограничивает исследование потомками данного определения.
// Любой достоверный отображаемый элемент доказывает непустоту; неопределённость
// сохраняется, если содержимое не найдено. resolved принадлежит одной паре
// границ и хранит результаты исследования включений. Текст не нормализуется.
func v059Content(lines diagnostics.Lines, first, end, fragment int, resolved map[int]v059ContentState) v059ContentState {
	result := v059Empty
	for number := first; number < end; number++ {
		line, _ := lines.Line(number)
		if fragment != 0 && !v059Descendant(lines, line, fragment) ||
			v059Hidden(lines, line, fragment) {
			continue
		}
		state := v059LineContent(lines, line, resolved)
		if state == v059Filled {
			return state
		}
		if state == v059Unknown {
			result = state
		}
	}
	return result
}

// v059LineContent классифицирует достоверно распознанную часть строки прежде
// ошибочного остатка (уточнение A). Служебные значения не становятся текстом.
func v059LineContent(lines diagnostics.Lines, line diagnostics.Line, resolved map[int]v059ContentState) v059ContentState {
	switch line.Type() {
	case "blank", "block-end", "separator", "heading":
		return v059Empty
	case "content":
		unparsed, text := false, false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			unparsed = unparsed || element.Type() == "unparsed"
			text = text || element.Type() == "content" && element.Raw() != ""
		}
		if text || !unparsed && line.Raw() != "" {
			return v059Filled
		}
		if unparsed {
			return v059Unknown
		}
		return v059Empty
	case "tag", "block-start":
		switch v059Tag(line) {
		case "header", "task", "step", "media", "instr", "example", "wordlist", "table",
			"script", "text", "key", "note", "alt", "hint", "question", "multifill",
			"choice", "multichoice", "matching", "ordering":
			return v059Filled
		case "include":
			// Алгоритм V059 и уточнение A требуют исследовать распознанную
			// вставку даже при ошибке её размещения или дополнительном остатке.
			return v059IncludedContent(lines, line, resolved)
		case "editor", "fragment", "endtask", "speaking", "dsl-version",
			"document-id", "section", "order", "resource-dir",
			"newpage", "variants", "variant", "answer":
			return v059Empty
		}
	}
	return v059Unknown
}

// v059IncludedContent разрешает ID по всему файлу без обрезки и декодирования.
// EqualFold применяется только к ID. Неразрешимая ссылка даёт неопределённость.
func v059IncludedContent(lines diagnostics.Lines, include diagnostics.Line, resolved map[int]v059ContentState) v059ContentState {
	identifier := v059Element(include, "identifier")
	if identifier == nil {
		return v059Unknown
	}
	id, present := identifier.Value()
	if !present || id == "" {
		return v059Unknown
	}
	var definition int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" || v059Tag(line) != "fragment" {
			continue
		}
		candidate := v059Element(line, "identifier")
		if candidate == nil {
			continue
		}
		name, found := candidate.Value()
		if found && strings.EqualFold(id, name) {
			if definition != 0 {
				return v059Unknown
			}
			definition = number
		}
	}
	if definition == 0 {
		return v059Unknown
	}
	if state, found := resolved[definition]; found {
		return state
	}
	// До завершения обхода пустота не доказана. Повторный вход по циклу
	// получает Unknown, а не запускает бесконечный обход. Непустая строка
	// после циклической ссылки всё ещё может доказать наличие содержимого.
	// Готовый результат переиспользуется при повторных включениях этой пары.
	resolved[definition] = v059Unknown
	state := v059Content(lines, definition+1, lines.Len()+1, definition, resolved)
	resolved[definition] = state
	return state
}

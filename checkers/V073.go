package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v073ContentState различает доказанную пустоту, материал и неопределённость.
type v073ContentState uint8

const (
	v073Empty v073ContentState = iota
	v073Filled
	v073Unknown
)

// v073Boundary хранит исходную позицию и вид границы шага.
type v073Boundary struct {
	line, column int
	newpage      bool
}

// registrationV073 регистрирует проверку материала после newpage для DSL 1.2.
func registrationV073(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V073",
		Severity: diagnostics.SeverityError,
		Message:  "Шаг после @newpage не содержит отображаемого материала.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV073,
	})
}

// checkV073 проверяет каждый newpage до следующей границы либо EOF.
// Основной диапазон включает строки до границы, связанным становится сама
// граница. Колонки исходные, с единицы; конец исключается, EOL не включается.
func checkV073(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var previous v073Boundary
	for number := 1; number <= lines.Len()+1; number++ {
		var current v073Boundary
		if number <= lines.Len() {
			line, _ := lines.Line(number)
			var found bool
			current, found = v073StepBoundary(lines, line)
			if !found {
				continue
			}
		}
		if previous.newpage &&
			v073Content(lines, previous.line+1, number, 0, make(map[int]v073ContentState)) == v073Empty {
			last, _ := lines.Line(number - 1)
			occurrence, err := findings.Add(previous.line, previous.column,
				number-1, utf8.RuneCountInString(last.Raw())+1, "")
			if err != nil {
				return err
			}
			if current.line != 0 {
				line, _ := lines.Line(current.line)
				if err := findings.AddRelated(occurrence, current.line, current.column,
					current.line, utf8.RuneCountInString(line.Raw())+1); err != nil {
					return err
				}
			}
		}
		previous = current
	}
	return nil
}

// v073StepBoundary возвращает распознанную границу вне фигурных предков.
// false означает отсутствие границы; литеральный Raw заново не разбирается.
func v073StepBoundary(lines diagnostics.Lines, line diagnostics.Line) (v073Boundary, bool) {
	for ancestor := line; ; {
		parent, found := ancestor.ParentLine()
		if !found {
			break
		}
		ancestor, _ = lines.Line(parent)
		if ancestor.Type() == "block-start" {
			return v073Boundary{}, false
		}
	}
	if line.Type() == "heading" {
		if marker := v073Element(line, "heading-level"); marker != nil {
			level, _ := marker.Value()
			if level == "3" {
				return v073Boundary{line: line.Number(), column: marker.Start()}, true
			}
		}
	}
	if (line.Type() == "tag" || line.Type() == "invalid") && v073Tag(line) == "newpage" {
		marker := v073Element(line, "tag")
		return v073Boundary{line: line.Number(), column: marker.Start(), newpage: true}, true
	}
	return v073Boundary{}, false
}

// v073Element возвращает единственный элемент типа; nil означает отсутствие
// либо неоднозначность. Исходник не разбирается повторно.
func v073Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v073Tag читает распознанное имя без учёта регистра, включая сохранённое
// объявление ошибочной строки. Литеральное содержимое не становится тегом.
func v073Tag(line diagnostics.Line) string {
	switch line.Type() {
	case "tag", "block-start", "invalid":
		if tag := v073Element(line, "tag"); tag != nil {
			name, _ := tag.Value()
			return strings.ToLower(name)
		}
	}
	return ""
}

// v073Hidden исключает editor, определения fragment и всех их потомков.
// При исследовании включения корень fragment не скрывает собственное тело.
func v073Hidden(lines diagnostics.Lines, line diagnostics.Line, fragment int) bool {
	for line.Number() != fragment {
		if tag := v073Tag(line); tag == "editor" || tag == "fragment" {
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

// v073Descendant возвращает true только для потомка указанной строки.
func v073Descendant(lines diagnostics.Lines, line diagnostics.Line, ancestor int) bool {
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

// v073Content исследует физические строки [first, end); end исключается.
// Ненулевой fragment ограничивает обход потомками определения. Найденный
// материал достаточен независимо от других ошибок. resolved хранит результаты
// включений только для текущего шага; текст не нормализуется.
func v073Content(lines diagnostics.Lines, first, end, fragment int, resolved map[int]v073ContentState) v073ContentState {
	result := v073Empty
	for number := first; number < end; number++ {
		line, _ := lines.Line(number)
		if fragment != 0 && !v073Descendant(lines, line, fragment) ||
			v073Hidden(lines, line, fragment) {
			continue
		}
		state := v073LineContent(lines, line, resolved)
		if state == v073Filled {
			return state
		}
		if state == v073Unknown {
			result = state
		}
	}
	return result
}

// v073LineContent учитывает наличие конструкции без оценки её полноты.
// Служебные значения не считаются текстом, а существенный unparsed оставляет
// неопределённость, если достоверно распознанных данных недостаточно.
func v073LineContent(lines diagnostics.Lines, line diagnostics.Line, resolved map[int]v073ContentState) v073ContentState {
	switch v073Tag(line) {
	case "header", "task", "step", "media", "instr", "example", "wordlist", "table",
		"script", "text", "key", "note", "alt", "hint", "question", "multifill",
		"choice", "multichoice", "matching", "ordering", "variants":
		return v073Filled
	case "include":
		return v073IncludedContent(lines, line, resolved)
	case "editor", "fragment", "endtask", "speaking", "dsl-version", "document-id",
		"section", "order", "resource-dir", "newpage", "variant", "answer":
		return v073Empty
	}
	switch line.Type() {
	case "blank", "block-end", "separator", "heading":
		return v073Empty
	case "content":
		unparsed := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "content" && element.Raw() != "" {
				return v073Filled
			}
			unparsed = unparsed || element.Type() == "unparsed"
		}
		if !unparsed {
			if line.Raw() != "" {
				return v073Filled
			}
			return v073Empty
		}
	}
	return v073Unknown
}

// v073IncludedContent ищет единственный fragment по всему документу.
// EqualFold применяется к ID без обрезки или повторного декодирования.
// Отсутствующая, неоднозначная либо циклическая ссылка не доказывает пустоту.
func v073IncludedContent(lines diagnostics.Lines, include diagnostics.Line, resolved map[int]v073ContentState) v073ContentState {
	identifier := v073Element(include, "identifier")
	if identifier == nil {
		return v073Unknown
	}
	id, present := identifier.Value()
	if !present || id == "" {
		return v073Unknown
	}
	definition := 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" || v073Tag(line) != "fragment" {
			continue
		}
		if candidate := v073Element(line, "identifier"); candidate != nil {
			name, found := candidate.Value()
			if found && strings.EqualFold(id, name) {
				if definition != 0 {
					return v073Unknown
				}
				definition = number
			}
		}
	}
	if definition == 0 {
		return v073Unknown
	}
	if state, found := resolved[definition]; found {
		return state
	}
	// До завершения обхода повторный вход означает цикл. Завершённый
	// результат позволяет отличить от него обычное повторное включение.
	resolved[definition] = v073Unknown
	state := v073Content(lines, definition+1, lines.Len()+1, definition, resolved)
	resolved[definition] = state
	return state
}

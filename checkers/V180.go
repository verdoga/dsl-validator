package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v180Text хранит обработанное значение и исходный диапазон [start,end).
type v180Text struct {
	value            string
	line, start, end int
}

// registrationV180 регистрирует проверку записи номеров сокращённого ordering.
func registrationV180(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V180",
		Severity: diagnostics.SeverityError,
		Message:  "Номер позиции содержит символы, отличные от цифр 0–9, или начинается с нуля.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV180,
	})
}

// checkV180 проверяет связанные ответы без разделителя в непосредственном
// содержимом ordering. Повторы объединяются только внутри одного answer.
func checkV180(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	separators := make(map[int]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
			if strings.Trim(line.Raw(), " \t") == "---" {
				separators[parent] = true
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		answer, _ := lines.Line(number)
		tag := v180Tag(answer, "answer")
		parent, found := answer.ParentLine()
		if tag == nil || !found || separators[parent] || (answer.Type() != "tag" && answer.Type() != "block-start") {
			continue
		}
		container, _ := lines.Line(parent)
		if container.Type() != "block-start" || v180Tag(container, "ordering") == nil || !v180Linked(lines, number, parent) {
			continue
		}
		values, complete := v180Values(answer, tag, children[number])
		if !complete {
			continue
		}
		occurrences := make(map[string]int)
		for _, text := range values {
			if v180Empty(text.value) || v180Number(text.value) {
				continue
			}
			if occurrence, found := occurrences[text.value]; found {
				if err := findings.AddRelated(occurrence, text.line, text.start, text.line, text.end); err != nil {
					return err
				}
				continue
			}
			occurrence, err := findings.Add(text.line, text.start, text.line, text.end, "")
			if err != nil {
				return err
			}
			occurrences[text.value] = occurrence
		}
	}
	return nil
}

// v180Linked требует предшествующую обычную строку того же ordering.
// Пропускаются только физические строки, пустые после удаления U+0020/U+0009.
func v180Linked(lines diagnostics.Lines, answer, container int) bool {
	for number := answer - 1; number > container; number-- {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		parent, found := line.ParentLine()
		return found && parent == container && line.Type() == "content" && v180Element(line, "tag") == nil
	}
	return false
}

// v180Values собирает все значения до выдачи сообщений. complete=false
// исключает весь answer, включая ранее прочитанные строки. Потомки глубже
// непосредственных строк не входят ни в значения, ни в проверку их полноты.
func v180Values(answer diagnostics.Line, tag diagnostics.Element, rows []diagnostics.Line) (values []v180Text, complete bool) {
	if v180Uncertain(answer) {
		return nil, false
	}
	if answer.Type() == "tag" {
		text, ok := v180Value(answer, tag.End()-1, false)
		return []v180Text{text}, ok
	}
	for _, line := range rows {
		if v180Uncertain(line) {
			return nil, false
		}
		if line.Type() != "content" || v180Element(line, "tag") != nil {
			continue
		}
		text, ok := v180Value(line, 0, true)
		if !ok {
			return nil, false
		}
		values = append(values, text)
	}
	return values, true
}

// v180Value использует полное Value без декодирования. При отсутствии content
// восстанавливает однозначный текст Raw; частичное покрытие content не склеивает.
// start — индекс руны начала содержимого, initial разрешает начальные \@ и \#.
func v180Value(line diagnostics.Line, start int, initial bool) (v180Text, bool) {
	raw := []rune(line.Raw())
	end := len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	text := v180Text{line: line.Number(), start: start + 1, end: end + 1}
	var content diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		if content != nil || element.Start()-1 > start || element.End()-1 < end {
			return text, false
		}
		content = element
	}
	if content != nil {
		value, present := content.Value()
		text.value = strings.Trim(value, " \t")
		return text, present
	}
	text.value = v180Decode(raw[start:end], initial)
	return text, true
}

// v180Decode снимает только разрешённое в служебном answer экранирование
// слева направо, один раз. Неизвестные пары сохраняются; плейсхолдеров здесь нет.
func v180Decode(raw []rune, initial bool) string {
	var text strings.Builder
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' || initial && index == 0 && (next == '@' || next == '#') {
				char = next
				index++
			}
		}
		text.WriteRune(char)
	}
	return text.String()
}

// v180Empty исключает пустые и целиком пробельные ответы по DSL 4.1.8.
// Пробелы в непустых значениях не удаляются и не сворачиваются.
func v180Empty(value string) bool {
	for _, char := range value {
		if !unicode.Is(unicode.White_Space, char) {
			return false
		}
	}
	return true
}

// v180Number проверяет [1-9][0-9]* без числового преобразования и верхней границы.
func v180Number(value string) bool {
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for index := 1; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

// v180Uncertain сообщает о неразобранном участке, исключающем проверку answer.
func v180Uncertain(line diagnostics.Line) bool {
	return line.Type() == "invalid" || v180Element(line, "unparsed") != nil
}

// v180Tag получает сохранённый tag по имени без восстановления объявления.
func v180Tag(line diagnostics.Line, name string) diagnostics.Element {
	tag := v180Element(line, "tag")
	if tag != nil {
		value, _ := tag.Value()
		if strings.EqualFold(value, name) {
			return tag
		}
	}
	return nil
}

// v180Element возвращает сохранённый элемент указанного типа либо nil.
func v180Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

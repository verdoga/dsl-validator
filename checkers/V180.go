package checkers

import (
	"strings"

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
		values := v180Values(answer, tag, children[number])
		type group struct {
			value      string
			occurrence int
		}
		var groups []group
		for _, text := range values {
			if answerPrimary(text.value) == "" {
				continue
			}
			if _, valid := answerPosition(text.value); valid {
				continue
			}
			occurrence := -1
			for _, previous := range groups {
				if answerEqual(previous.value, text.value) {
					occurrence = previous.occurrence
					break
				}
			}
			if occurrence >= 0 {
				if err := findings.AddRelated(occurrence, text.line, text.start, text.line, text.end); err != nil {
					return err
				}
				continue
			}
			occurrence, err := findings.Add(text.line, text.start, text.line, text.end, "")
			if err != nil {
				return err
			}
			groups = append(groups, group{text.value, occurrence})
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

// v180Values собирает известные альтернативы непосредственных строк answer.
// Неопределимость одной альтернативы не исключает остальные; более глубокие
// потомки не входят в значения.
func v180Values(answer diagnostics.Line, tag diagnostics.Element, rows []diagnostics.Line) (values []v180Text) {
	if v180Uncertain(answer) {
		return nil
	}
	if answer.Type() == "tag" {
		text, ok := v180Value(answer, tag.End()-1, false)
		if ok {
			return []v180Text{text}
		}
		return nil
	}
	for _, line := range rows {
		if v180Uncertain(line) {
			continue
		}
		if line.Type() != "content" || v180Element(line, "tag") != nil {
			continue
		}
		text, ok := v180Value(line, 0, true)
		if ok {
			values = append(values, text)
		}
	}
	return values
}

// v180Value соединяет content.Value без повторного декодирования, сохраняя
// пробельные промежутки Raw. Непокрытый непробельный текст делает значение
// неизвестным; при полном отсутствии content используется однократный decode.
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
	var value strings.Builder
	cursor := start
	hasContent := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		hasContent = true
		part, present := element.Value()
		if !present {
			return text, false
		}
		// Пустая отметка не занимает исходник, в том числе внутри фрагмента.
		if element.Start() == element.End() && part == "" {
			continue
		}
		if gapEnd := min(element.Start()-1, end); gapEnd > cursor {
			gap := string(raw[cursor:gapEnd])
			if strings.TrimSpace(gap) != "" {
				return text, false
			}
			value.WriteString(gap)
		}
		value.WriteString(part)
		cursor = max(cursor, element.End()-1)
	}
	if hasContent {
		if cursor < end {
			gap := string(raw[cursor:end])
			if strings.TrimSpace(gap) != "" {
				return text, false
			}
			value.WriteString(gap)
		}
		text.value = value.String()
		return text, true
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

// v180Uncertain сообщает о неразобранном участке проверяемой альтернативы.
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

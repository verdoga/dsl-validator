package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV183 регистрирует неоднозначные подписи позиций ordering.
func registrationV183(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V183", Severity: diagnostics.SeverityError,
		Message:  "В нижнем списке @ordering повторяется подпись позиции.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV183,
	})
}

// v183Label хранит обработанную подпись и исходные Unicode-колонки [start,end).
type v183Label struct {
	text             string
	line, start, end int
}

func checkV183(lines diagnostics.Lines, findings diagnostics.Findings) error {
	bodies := v183Bodies(lines)
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v183Tag(block) != "ordering" {
			continue
		}
		body := bodies[number]
		children, known := body.children, !body.broken && body.last-number == body.size-1
		if !known {
			continue
		}
		separator, count := 0, 0
		for index, line := range children {
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator, count = index, count+1
			}
		}
		if count != 1 || !v183Answered(children[:separator]) {
			continue
		}
		var labels []v183Label
		for _, line := range children[separator+1:] {
			label, ordinary, ok := v183Text(line)
			if !ok {
				known = false
				break
			}
			if ordinary && label.text != "" {
				labels = append(labels, label)
			}
		}
		if known {
			if err := v183Report(labels, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v183Body хранит прямые конструкции и сводку поддерева, включая саму строку.
// broken означает неоднозначное закрытие этого блока или его потомка.
type v183Body struct {
	children                   []diagnostics.Line
	parent, last, size, closed int
	broken                     bool
}

// v183Bodies строит локальный индекс за O(N) времени и памяти. Prepare гарантирует,
// что родитель предшествует потомку: обратный обход сворачивает каждое ребро один раз.
// Пропуск чужих строк обнаруживается по длине диапазона и размеру поддерева;
// закрытие должно быть единственным и последним потомком своего блока.
func v183Bodies(lines diagnostics.Lines) []v183Body {
	bodies := make([]v183Body, lines.Len()+1)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		body := &bodies[number]
		body.parent, _ = line.ParentLine()
		body.last, body.size = number, 1
		parent := &bodies[body.parent]
		if line.Type() == "block-end" {
			parent.broken = parent.broken || parent.closed != 0 || strings.Trim(line.Raw(), " \t") != "}"
			parent.closed = number
		} else if strings.Trim(line.Raw(), " \t") != "" {
			parent.children = append(parent.children, line)
		}
	}
	for number := lines.Len(); number > 0; number-- {
		body := &bodies[number]
		body.broken = body.broken || body.closed != 0 && body.closed != body.last
		parent := &bodies[body.parent]
		parent.broken = parent.broken || body.broken
		parent.size += body.size
		if body.last > parent.last {
			parent.last = body.last
		}
	}
	return bodies
}

func v183Tag(line diagnostics.Line) string {
	name, count := "", 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ = element.Value()
			count++
		}
	}
	if count != 1 {
		return ""
	}
	return strings.ToLower(name)
}

// v183Answered требует ровно один соседний ответ у каждого верхнего элемента.
// Пустые строки уже исключены, другие конструкции при поиске не пропускаются.
func v183Answered(upper []diagnostics.Line) bool {
	items := make([]bool, len(upper))
	count := 0
	for index, line := range upper {
		label, ordinary, known := v183Text(line)
		if !known {
			return false
		}
		items[index] = ordinary && label.text != ""
		if items[index] {
			count++
			if index+1 == len(upper) || v183Tag(upper[index+1]) != "answer" {
				return false
			}
		}
		if v183Tag(line) == "answer" && (index == 0 || !items[index-1]) {
			return false
		}
	}
	return count != 0
}

// v183Text отличает обычную строку от конструкции и возвращает определимость.
// Готовые фрагменты Value соединяются с исходными пробелами между ними;
// при отсутствии content восстанавливается только однозначный обычный текст.
func v183Text(line diagnostics.Line) (label v183Label, ordinary, known bool) {
	switch line.Type() {
	case "tag", "block-start", "block-end", "heading":
		return label, false, true
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" || element.Type() == "heading-level" {
			return label, false, true
		}
	}
	if line.Type() != "content" {
		return label, false, false
	}
	raw := []rune(line.Raw())
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	label = v183Label{line: line.Number(), start: start + 1, end: end + 1}
	if line.ElementCount() == 0 {
		label.text, known = v183Unescape(string(raw[start:end]))
		return label, true, known
	}
	var text strings.Builder
	cursor := 0
	fragment := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			return label, true, false
		}
		value, present := element.Value()
		if !present {
			return label, true, false
		}
		// Пустая отметка может лежать внутри предыдущего занятого диапазона.
		// Она не потребляет исходник и не создаёт пробел перед первым фрагментом.
		if element.Start() == element.End() && value == "" {
			continue
		}
		gap := ""
		if element.Start()-1 >= cursor {
			gap = string(raw[cursor : element.Start()-1])
		}
		if strings.Trim(gap, " \t") != "" {
			return label, true, false
		}
		if fragment {
			text.WriteString(gap)
		}
		text.WriteString(value)
		fragment = true
		if element.End()-1 > cursor {
			cursor = element.End() - 1
		}
	}
	label.text = text.String()
	return label, true, strings.Trim(string(raw[cursor:]), " \t") == ""
}

// v183Unescape выполняет только контекстное экранирование обычной строки ordering.
// false означает сырую разметку, которую нельзя однозначно считать подписью.
func v183Unescape(raw string) (string, bool) {
	if raw == `\---` {
		return "---", true
	}
	runes := []rune(raw)
	var text strings.Builder
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' && index+1 < len(runes) {
			next := runes[index+1]
			if next == '\\' || next == '{' || next == '}' || (index == 0 && (next == '@' || next == '#')) {
				text.WriteRune(next)
				index++
				continue
			}
		}
		if char == '{' || char == '}' || (index == 0 && (char == '@' || char == '#')) {
			return "", false
		}
		text.WriteRune(char)
	}
	return text.String(), true
}

// v183Report объединяет повторы по EqualFold без иных преобразований подписи.
func v183Report(labels []v183Label, findings diagnostics.Findings) error {
	var groups [][]v183Label
	for _, label := range labels {
		found := false
		for index, group := range groups {
			if strings.EqualFold(group[0].text, label.text) {
				groups[index] = append(group, label)
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, []v183Label{label})
		}
	}
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		main := group[1]
		occurrence, err := findings.Add(main.line, main.start, main.line, main.end, "")
		if err != nil {
			return err
		}
		for index, label := range group {
			if index != 1 {
				if err := findings.AddRelated(occurrence, label.line, label.start, label.line, label.end); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

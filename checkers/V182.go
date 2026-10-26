package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v182Value сохраняет обработанное значение и его исходные Unicode-колонки с 1.
// end — исключающая граница; координаты не вычисляются по обработанному тексту.
type v182Value struct {
	text             string
	line, start, end int
}

// registrationV182 регистрирует отсутствующие подписи позиций только для DSL 1.2.
func registrationV182(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V182", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Значение @answer отсутствует среди позиций нижнего списка @ordering.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV182,
	})
}

// checkV182 использует физические границы готовых конструкций; другие диагностики не читаются.
func checkV182(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children, ends, blocks := v182Structure(lines)
	for _, block := range blocks {
		body := children[block.Number()]
		if !v182Closed(block.Number(), ends[block.Number()], body) {
			continue
		}
		separator := -1
		for i, line := range body {
			if strings.Trim(line.Raw(), " \t") == "---" {
				if separator >= 0 {
					separator = -1
					break
				}
				separator = i
			}
		}
		if separator < 0 {
			continue
		}
		answers, complete := v182Answers(body[:separator], body[separator].Number(), children, ends)
		positions, known := v182Positions(body[separator+1:])
		if !complete || !known {
			continue
		}
		var groups [][]v182Value
		for _, answer := range answers {
			rows := []diagnostics.Line{answer}
			if answer.Type() == "block-start" {
				rows = children[answer.Number()]
			}
			values, available := v182AnswerValues(rows, answer.Type() != "block-start")
			if !available {
				complete = false
				break
			}
			groups = append(groups, values)
		}
		if !complete {
			continue
		}
		for _, values := range groups {
			if err := v182Compare(values, positions, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v182Structure индексирует готовые физические фигурные контейнеры.
// heading меняет логический ParentLine, но не завершает блок; его последующие
// строки принадлежат текущему физическому контейнеру. Прочие вложенные связи
// сохраняются. Неизвестные и пересечённые границы получают end=-1.
func v182Structure(lines diagnostics.Lines) (children map[int][]diagnostics.Line, ends map[int]int, blocks []diagnostics.Line) {
	children, ends = make(map[int][]diagnostics.Line), make(map[int]int)
	owners := make([]int, lines.Len()+1)
	opened := make([]bool, lines.Len()+1)
	literal := make([]bool, lines.Len()+1)
	uncertain := make([]bool, lines.Len()+1)
	var stack []int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		owner := 0
		if len(stack) != 0 {
			owner = stack[len(stack)-1]
		}
		owners[number] = owner
		parent, _ := line.ParentLine()
		var parentLine diagnostics.Line
		if parent != 0 {
			parentLine, _ = lines.Line(parent)
		}
		text := strings.Trim(line.Raw(), " \t")
		opening, closing := false, false
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			opening = opening || element.Type() == "block-open"
			closing = closing || element.Type() == "block-close"
		}
		// Литеральное тело не создаёт псевдоблоки и завершается только отдельной }.
		if literal[owner] && text != "}" {
			continue
		}
		if text == "}" || closing {
			if owner == 0 {
				if line.Type() == "block-end" && parent != 0 {
					uncertain[parent] = true
				}
				continue
			}
			if opening || line.Type() == "block-end" && parent != 0 && parent != owner {
				uncertain[owner] = true
				if parent != 0 {
					uncertain[parent] = true
				}
			}
			ends[owner] = number
			stack = stack[:len(stack)-1]
			continue
		}
		if line.Type() == "block-end" {
			// Тип без закрывающего элемента и отдельной скобки не доказывает границу.
			uncertain[owner] = true
		}
		effective := parent
		if owner != 0 {
			switch {
			case line.Type() == "heading", parentLine != nil && parentLine.Type() == "heading", parent == 0 && text == "":
				effective = owner
			case parent == owner:
			case parentLine != nil && parentLine.Type() == "tag" && owners[parent] == owner && !opened[parent]:
				// Логическое тело однострочного контейнера не становится прямым текстом.
			default:
				effective, uncertain[owner] = owner, true
			}
		} else if parentLine != nil && parentLine.Type() != "heading" {
			if opened[parent] {
				uncertain[parent] = true
			} else if owners[parent] != 0 {
				uncertain[owners[parent]] = true
			}
		}
		if effective != 0 {
			children[effective] = append(children[effective], line)
		}
		if opening || line.Type() == "block-start" {
			opened[number] = true
			uncertain[number] = !opening
			name, _ := v182Tag(line)
			literal[number] = name == "editor" || name == "multifill"
			stack = append(stack, number)
			if name == "ordering" && line.Type() == "block-start" {
				blocks = append(blocks, line)
			}
		}
	}
	// Неоднозначность вложенной границы отменяет доказательство для её контейнеров.
	for number := lines.Len(); number > 0; number-- {
		if opened[number] && ends[number] <= number {
			uncertain[number] = true
		}
		if uncertain[number] {
			ends[number] = -1
			uncertain[owners[number]] = true
		}
	}
	return children, ends, blocks
}

// v182Closed проверяет однозначное закрытие и принадлежность тела его границам.
func v182Closed(open, end int, body []diagnostics.Line) bool {
	return end > open && (len(body) == 0 || body[len(body)-1].Number() < end)
}

// v182Answers возвращает следующие за элементами ответы. complete=false означает
// отсутствие однозначного полного набора, включая лишний несвязанный ответ.
func v182Answers(body []diagnostics.Line, separator int, children map[int][]diagnostics.Line, ends map[int]int) (answers []diagnostics.Line, complete bool) {
	for i := 0; i < len(body); i++ {
		line := body[i]
		if line.Type() == "blank" && strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		name, tag := v182Tag(line)
		if tag != nil {
			if name == "answer" {
				return nil, false
			}
			continue
		}
		if line.Type() != "content" {
			return nil, false
		}
		next := i + 1
		for next < len(body) && body[next].Type() == "blank" && strings.Trim(body[next].Raw(), " \t") == "" {
			next++
		}
		if next == len(body) {
			return nil, false
		}
		answer := body[next]
		name, _ = v182Tag(answer)
		if name != "answer" {
			return nil, false
		}
		if answer.Type() == "block-start" {
			end := ends[answer.Number()]
			if !v182Closed(answer.Number(), end, children[answer.Number()]) || end >= separator ||
				next+1 < len(body) && body[next+1].Number() <= end {
				return nil, false
			}
		} else if answer.Type() != "tag" {
			return nil, false
		}
		answers = append(answers, answer)
		i = next
	}
	return answers, len(answers) > 0
}

// v182Positions извлекает непосредственные подписи. known=false означает,
// что неразобранная потенциальная позиция не позволяет доказать отсутствие ссылки.
func v182Positions(body []diagnostics.Line) (positions []string, known bool) {
	for _, line := range body {
		_, tag := v182Tag(line)
		if tag != nil || line.Type() == "blank" && strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		if line.Type() != "content" {
			return nil, false
		}
		value, ok := v182Text(line, false, true)
		if !ok {
			return nil, false
		}
		positions = append(positions, value.text)
	}
	return positions, true
}

// v182AnswerValues требует доступности и непустоты всех альтернатив одного answer.
// Пустые физические строки и известные вложенные конструкции не задают альтернатив.
func v182AnswerValues(lines []diagnostics.Line, inline bool) (values []v182Value, available bool) {
	for _, line := range lines {
		if !inline {
			if line.Type() == "blank" && strings.Trim(line.Raw(), " \t") == "" {
				continue
			}
			if line.Type() != "content" {
				_, tag := v182Tag(line)
				if tag != nil || line.Type() == "heading" {
					continue
				}
				return nil, false
			}
		}
		value, known := v182Text(line, inline, false)
		if !known || answerPrimary(value.text) == "" {
			return nil, false
		}
		values = append(values, value)
	}
	return values, len(values) > 0
}

// v182Compare группирует только отсутствующие значения одного доступного answer.
func v182Compare(values []v182Value, positions []string, findings diagnostics.Findings) error {
	var missing []v182Value
	var occurrences []int
	for _, value := range values {
		matched := false
		for _, position := range positions {
			if answerEqual(value.text, position) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		duplicate := -1
		for i, earlier := range missing {
			if answerEqual(value.text, earlier.text) {
				duplicate = i
				break
			}
		}
		if duplicate >= 0 {
			if err := findings.AddRelated(occurrences[duplicate], value.line, value.start, value.line, value.end); err != nil {
				return err
			}
			continue
		}
		occurrence, err := findings.Add(value.line, value.start, value.line, value.end, "")
		if err != nil {
			return err
		}
		missing = append(missing, value)
		occurrences = append(occurrences, occurrence)
	}
	return nil
}

// v182Text предпочитает готовое Value без декодирования. Иначе восстанавливает
// одну физическую строку по DSL 1.14, 1.21–1.22. false означает неопределимый текст.
// inline исключает тег из диапазона; position разрешает литеральный разделитель.
func v182Text(line diagnostics.Line, inline, position bool) (value v182Value, known bool) {
	raw := []rune(line.Raw())
	start, end := 0, len(raw)
	if inline {
		_, tag := v182Tag(line)
		if tag == nil {
			return value, false
		}
		start = tag.End() - 1
	}
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	value = v182Value{line: line.Number(), start: start + 1, end: end + 1}
	var content diagnostics.Element
	count := 0
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "unparsed" {
			return value, false
		}
		if element.Type() == "content" {
			content, count = element, count+1
		}
	}
	if count == 1 && content.Start() <= start+1 && content.End() >= end+1 {
		value.text, known = content.Value()
		if known {
			return value, true
		}
	}
	text := raw[start:end]
	var decoded strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\\' && i+1 < len(text) {
			next := text[i+1]
			if next == '\\' || next == '{' || next == '}' ||
				!inline && i == 0 && (next == '@' || next == '#') ||
				position && i == 0 && string(text) == `\---` {
				decoded.WriteRune(next)
				i++
				continue
			}
		}
		if c == '{' || c == '}' || !inline && i == 0 && (c == '@' || c == '#') {
			return value, false
		}
		decoded.WriteRune(c)
	}
	value.text = decoded.String()
	return value, true
}

// v182Tag читает распознанное имя; литеральные псевдотеги не разбираются.
func v182Tag(line diagnostics.Line) (string, diagnostics.Element) {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.ToLower(name), element
		}
	}
	return "", nil
}

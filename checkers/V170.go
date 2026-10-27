package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV170 регистрирует проверку наличия ответа в нижнем списке matching.
func registrationV170(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V170", Severity: diagnostics.SeverityError,
		Message:  "Значение @answer отсутствует среди вариантов нижнего списка.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV170,
	})
}

// v170Value сохраняет значение и исходный диапазон в Unicode-колонках [start,end).
type v170Value struct {
	text             string
	line, start, end int
}

func checkV170(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v170Tag(block) != "matching" {
			continue
		}
		children := v170Children(lines, block)
		separator, count := 0, 0
		for index, line := range children {
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator, count = index, count+1
			}
		}
		if count != 1 {
			continue
		}
		answers, ok := v170Answers(children[:separator])
		if !ok {
			continue
		}
		groups, ok := v170Values(lines, answers)
		if !ok {
			continue
		}
		labels, ok := v170Labels(children[separator+1:])
		if !ok {
			continue
		}
		for _, values := range groups {
			if err := v170Report(values, labels, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v170Children исключает пустые физические строки и содержимое вложенных блоков.
// Границы и родители берутся из модели, а не восстанавливаются из текста.
// Пустоту content и invalid вызывающий код определяет по готовому значению.
func v170Children(lines diagnostics.Lines, block diagnostics.Line) []diagnostics.Line {
	var children []diagnostics.Line
	for number := block.Number() + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.NestingLevel() <= block.NestingLevel() {
			break
		}
		parent, _ := line.ParentLine()
		if parent != block.Number() {
			continue
		}
		if line.Type() == "block-end" {
			break
		}
		if line.Type() != "blank" || strings.Trim(line.Raw(), " \t") != "" {
			children = append(children, line)
		}
	}
	return children
}

func v170Tag(line diagnostics.Line) string {
	name := ""
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			if name != "" {
				return ""
			}
			name, _ = element.Value()
		}
	}
	return strings.ToLower(name)
}

// v170Answers принимает только полную и однозначную последовательность связей.
// Один hint после ответа допустим; перед ответом никакие теги не пропускаются.
func v170Answers(upper []diagnostics.Line) ([]diagnostics.Line, bool) {
	var answers []diagnostics.Line
	for index := 0; index < len(upper); {
		if upper[index].Type() != "content" || index+1 == len(upper) {
			return nil, false
		}
		answer := upper[index+1]
		if v170Tag(answer) != "answer" || (answer.Type() != "tag" && answer.Type() != "block-start") {
			return nil, false
		}
		answers = append(answers, answer)
		index += 2
		if index < len(upper) && v170Tag(upper[index]) == "hint" {
			index++
		}
	}
	return answers, len(answers) != 0
}

// v170Values проверяет все альтернативы блока до выдачи любых сообщений.
// Недоступное или пустое по основной форме значение отменяет проверку блока.
func v170Values(lines diagnostics.Lines, answers []diagnostics.Line) ([][]v170Value, bool) {
	var groups [][]v170Value
	for _, answer := range answers {
		rows := []diagnostics.Line{answer}
		inline := answer.Type() != "block-start"
		if !inline {
			rows = v170Children(lines, answer)
		}
		var values []v170Value
		for _, row := range rows {
			if !inline && row.Type() != "content" && row.Type() != "invalid" {
				continue
			}
			value, ok := v170Text(row, inline, false)
			if !ok || answerPrimary(value.text) == "" {
				return nil, false
			}
			values = append(values, value)
		}
		if len(values) == 0 {
			return nil, false
		}
		groups = append(groups, values)
	}
	return groups, true
}

// v170Labels не доказывает отсутствие ответа, если хотя бы одна подпись неизвестна.
func v170Labels(lower []diagnostics.Line) ([]v170Value, bool) {
	var labels []v170Value
	for _, line := range lower {
		if line.Type() != "content" && line.Type() != "invalid" {
			continue
		}
		value, ok := v170Text(line, false, true)
		if !ok {
			return nil, false
		}
		if answerPrimary(value.text) != "" {
			labels = append(labels, value)
		}
	}
	return labels, true
}

// v170Text соединяет готовые content без повторного снятия экранирования.
// Между фрагментами сохраняются однозначные пробельные символы Unicode.
// При отсутствии content восстановление допустимо лишь для целой известной
// текстовой области без дополнительных элементов и неразобранных частей.
func v170Text(line diagnostics.Line, inline, separator bool) (v170Value, bool) {
	raw := []rune(line.Raw())
	start, end := 0, len(raw)
	var contents []diagnostics.Element
	hasContent := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "tag":
			if !inline || start != 0 || len(contents) != 0 || strings.Trim(string(raw[:element.Start()-1]), " \t") != "" {
				return v170Value{}, false
			}
			start = element.End() - 1
		case "content":
			hasContent = true
			text, present := element.Value()
			if !present {
				return v170Value{}, false
			}
			// Пустая отметка не занимает исходник и может лежать внутри фрагмента.
			if element.Start() != element.End() || text != "" {
				contents = append(contents, element)
			}
		default:
			return v170Value{}, false
		}
	}
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	value := v170Value{line: line.Number(), start: start + 1, end: end + 1}
	if hasContent {
		var text strings.Builder
		cursor := start
		for _, content := range contents {
			if gapEnd := min(content.Start()-1, end); gapEnd > cursor {
				gap := string(raw[cursor:gapEnd])
				if strings.TrimSpace(gap) != "" {
					return v170Value{}, false
				}
				text.WriteString(gap)
			}
			part, _ := content.Value()
			text.WriteString(part)
			cursor = max(cursor, content.End()-1)
		}
		value.text = text.String()
		return value, cursor >= end
	}
	var ok bool
	value.text, ok = v170Unescape(raw[start:end], !inline, separator)
	return value, ok
}

// v170Unescape обрабатывает только разрешённые здесь последовательности по 1.21.
// false означает неэкранированную разметку: значение нельзя надёжно восстановить.
func v170Unescape(raw []rune, lineStart, separator bool) (string, bool) {
	if separator && string(raw) == `\---` {
		return "---", true
	}
	var value strings.Builder
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' ||
				(index == 0 && lineStart && (next == '@' || next == '#')) {
				value.WriteRune(next)
				index++
				continue
			}
		}
		if char == '{' || char == '}' || (index == 0 && lineStart && (char == '@' || char == '#')) {
			return "", false
		}
		value.WriteRune(char)
	}
	return value.String(), true
}

// v170Report объединяет отсутствующие значения только внутри одного answer.
// Порядок и диапазоны вхождений сохраняются; сравнение с вариантами и повторами
// использует полную нормализацию ответов.
func v170Report(values, labels []v170Value, findings diagnostics.Findings) error {
	var reported []v170Value
	var occurrences []int
	for _, value := range values {
		matched := false
		for _, label := range labels {
			if answerEqual(value.text, label.text) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		occurrence := 0
		for index, previous := range reported {
			if answerEqual(value.text, previous.text) {
				occurrence = occurrences[index]
				break
			}
		}
		if occurrence != 0 {
			if err := findings.AddRelated(occurrence, value.line, value.start, value.line, value.end); err != nil {
				return err
			}
			continue
		}
		occurrence, err := findings.Add(value.line, value.start, value.line, value.end, "")
		if err != nil {
			return err
		}
		reported = append(reported, value)
		occurrences = append(occurrences, occurrence)
	}
	return nil
}

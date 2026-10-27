package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV171 регистрирует дубликаты нижних вариантов автоматического matching.
func registrationV171(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V171", Severity: diagnostics.SeverityError,
		Message:  "В нижнем списке повторяется подпись варианта.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV171,
	})
}

// v171Label хранит обработанную подпись и исходный диапазон [start,end).
type v171Label struct {
	text             string
	line, start, end int
}

// checkV171 не использует ответы для фильтрации групп одинаковых подписей,
// но добавляет диапазоны совпадающих альтернатив к связанным местам ошибки.
func checkV171(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v171Tag(block) != "matching" || v171LiteralParent(lines, block) {
			continue
		}
		var rows []diagnostics.Line
		separator, count := 0, 0
		for next := number + 1; next <= lines.Len(); next++ {
			line, _ := lines.Line(next)
			parent, _ := line.ParentLine()
			if line.NestingLevel() <= block.NestingLevel() || parent == number && line.Type() == "block-end" {
				break
			}
			if parent != number {
				continue
			}
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator, count = len(rows), count+1
			}
			rows = append(rows, line)
		}
		if count != 1 || !v171Automatic(lines, rows[:separator]) {
			continue
		}
		var answers []v171Label
		for _, row := range rows[:separator] {
			if v171Tag(row) == "answer" {
				values, _ := v171AnswerValues(lines, row)
				answers = append(answers, values...)
			}
		}
		var groups [][]v171Label
		for _, row := range rows[separator+1:] {
			if row.Type() != "content" {
				continue
			}
			label, known := v171Text(row)
			if !known || answerPrimary(label.text) == "" {
				continue
			}
			group := -1
			for index := range groups {
				if answerEqual(groups[index][0].text, label.text) {
					group = index
					break
				}
			}
			if group < 0 {
				groups = append(groups, []v171Label{label})
			} else {
				groups[group] = append(groups[group], label)
			}
		}
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			second := group[1]
			occurrence, err := findings.Add(second.line, second.start, second.line, second.end, "")
			if err != nil {
				return err
			}
			related := append([]v171Label{}, group...)
			for _, answer := range answers {
				if answerEqual(group[0].text, answer.text) {
					related = append(related, answer)
				}
			}
			seen := map[[3]int]bool{{second.line, second.start, second.end}: true}
			for _, label := range related {
				location := [3]int{label.line, label.start, label.end}
				if seen[location] {
					continue
				}
				if err := findings.AddRelated(occurrence, label.line, label.start, label.line, label.end); err != nil {
					return err
				}
				seen[location] = true
			}
		}
	}
	return nil
}

// v171Automatic требует полные связи каждого верхнего элемента с ответом,
// все альтернативы которого известны и непусты после основной нормализации.
// После ответа допускается одна подсказка; тела блоков сюда не входят.
func v171Automatic(lines diagnostics.Lines, rows []diagnostics.Line) bool {
	items, pending, answered := 0, false, false
	for _, row := range rows {
		if row.Type() == "content" {
			label, known := v171Text(row)
			if !known {
				return false
			}
			if answerPrimary(label.text) == "" {
				continue
			}
			if pending {
				return false
			}
			items++
			pending, answered = true, false
			continue
		}
		if strings.Trim(row.Raw(), " \t") == "" {
			continue
		}
		name := v171Tag(row)
		if name == "answer" {
			if !pending || !v171AnswerAvailable(lines, row) {
				return false
			}
			pending, answered = false, true
			continue
		}
		if name == "hint" && answered && (row.Type() == "tag" || row.Type() == "block-start") {
			answered = false
			continue
		}
		return false
	}
	return items > 0 && !pending
}

// v171AnswerAvailable проверяет каждую текстовую альтернативу связанного answer.
// Вложенные теги не создают альтернатив; неизвестная текстовая строка
// или пустая основная форма хотя бы одной альтернативы отменяет проверку.
func v171AnswerAvailable(lines diagnostics.Lines, answer diagnostics.Line) bool {
	_, available := v171AnswerValues(lines, answer)
	return available
}

// v171AnswerValues сохраняет доступные альтернативы answer в порядке исходника.
// Неизвестная или пустая основная форма отменяет все альтернативы ответа.
func v171AnswerValues(lines diagnostics.Lines, answer diagnostics.Line) ([]v171Label, bool) {
	inline := answer.Type() == "tag"
	rows := []diagnostics.Line{answer}
	if !inline {
		if answer.Type() != "block-start" {
			return nil, false
		}
		rows = v170Children(lines, answer)
	}
	var values []v171Label
	for _, row := range rows {
		if !inline && row.Type() != "content" && (row.Type() != "invalid" || v171Tag(row) != "") {
			continue
		}
		value, known := v170Text(row, inline, false)
		if !known || answerPrimary(value.text) == "" {
			return nil, false
		}
		values = append(values, v171Label{value.text, value.line, value.start, value.end})
	}
	return values, len(values) != 0
}

// v171Text использует Value, только если content охватывает всю подпись.
// Иначе восстанавливает Raw по DSL 1.14, 1.21–1.22. false означает, что
// unparsed не позволяет получить полную подпись для достоверного сравнения.
func v171Text(line diagnostics.Line) (v171Label, bool) {
	var content diagnostics.Element
	count := 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "unparsed" {
			return v171Label{}, false
		}
		if element.Type() == "content" {
			content = element
			count++
		}
	}
	if count == 1 {
		raw := []rune(line.Raw())
		prefix, suffix := string(raw[:content.Start()-1]), string(raw[content.End()-1:])
		if value, present := content.Value(); present && strings.Trim(prefix, " \t") == "" && strings.Trim(suffix, " \t") == "" {
			return v171Label{value, line.Number(), content.Start(), content.End()}, true
		}
	}
	raw := line.Raw()
	trimmed := strings.Trim(raw, " \t")
	start := len(raw) - len(strings.TrimLeft(raw, " \t")) + 1
	return v171Label{v171Unescape(trimmed), line.Number(), start, start + utf8.RuneCountInString(trimmed)}, true
}

// v171Unescape снимает экранирование один раз в контексте списка matching.
// Подчёркивания, HTML и разделители ячеек в этом контексте не служебные.
func v171Unescape(raw string) string {
	var text strings.Builder
	for index := 0; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' || index == 0 && (next == '@' || next == '#' || raw == `\---`) {
				index++
			}
		}
		text.WriteByte(raw[index])
	}
	return text.String()
}

// v171Tag возвращает имя объявления, распознанного моделью.
func v171Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return ""
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.ToLower(name)
		}
	}
	return ""
}

// v171LiteralParent исключает литеральное содержимое editor и multifill.
func v171LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if name := v171Tag(line); name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

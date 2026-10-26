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

// checkV171 не использует ответы для фильтрации групп одинаковых подписей.
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
		var groups [][]v171Label
		for _, row := range rows[separator+1:] {
			if row.Type() != "content" || strings.Trim(row.Raw(), " \t") == "" {
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
			for index, label := range group {
				if index == 1 {
					continue
				}
				if err := findings.AddRelated(occurrence, label.line, label.start, label.line, label.end); err != nil {
					return err
				}
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
		if strings.Trim(row.Raw(), " \t") == "" {
			continue
		}
		if row.Type() == "content" {
			if pending {
				return false
			}
			items++
			pending, answered = true, false
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
	if answer.Type() == "tag" {
		value, known := v170Text(answer, true, false)
		return known && answerPrimary(value.text) != ""
	}
	if answer.Type() != "block-start" {
		return false
	}
	found := false
	for _, row := range v170Children(lines, answer) {
		if row.Type() != "content" && (row.Type() != "invalid" || v171Tag(row) != "") {
			continue
		}
		value, known := v170Text(row, false, false)
		if !known || answerPrimary(value.text) == "" {
			return false
		}
		found = true
	}
	return found
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

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV157 регистрирует конфликт признаков правильности вариантов.
func registrationV157(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V157",
		Severity: diagnostics.SeverityError,
		Message:  "Одинаковый текст варианта указан и как правильный, и как неправильный.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV157,
	})
}

// v157Option хранит нормализованную подпись и исходный диапазон [start,end).
// Колонки считаются в кодовых точках Unicode с 1.
type v157Option struct {
	label            string
	line, start, end int
	correct          bool
}

// checkV157 рассматривает каждый блок независимо, сохраняя порядок вариантов.
func checkV157(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		name := v157Tag(block)
		if block.Type() != "block-start" || (name != "choice" && name != "multichoice") {
			continue
		}
		var groups [][]v157Option
		for next := number + 1; next <= lines.Len(); next++ {
			line, _ := lines.Line(next)
			parent, found := line.ParentLine()
			if !found {
				continue
			}
			correct := false
			if parent == number {
				if line.Type() == "block-end" {
					break
				}
				correct = line.Type() == "tag" && v157Tag(line) == "answer"
				if line.Type() != "content" && !correct {
					continue
				}
			} else {
				answer, _ := lines.Line(parent)
				owner, _ := answer.ParentLine()
				if owner != number || answer.Type() != "block-start" || v157Tag(answer) != "answer" || line.Type() != "content" {
					continue
				}
				correct = true
			}
			option, ok := v157ReadOption(line, correct)
			if !ok {
				continue
			}
			group := -1
			for index := range groups {
				if answerEqual(groups[index][0].label, option.label) {
					group = index
					break
				}
			}
			if group < 0 {
				groups = append(groups, []v157Option{option})
			} else {
				groups[group] = append(groups[group], option)
			}
		}
		for _, group := range groups {
			if err := v157Report(group, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v157Tag возвращает единственное распознанное имя тега, не разбирая Raw заново.
func v157Tag(line diagnostics.Line) string {
	name := ""
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		if name != "" {
			return ""
		}
		name, _ = element.Value()
	}
	return strings.ToLower(name)
}

// v157ReadOption собирает подпись из готовых Value и исходных пробельных
// промежутков между content. Экранирование повторно не снимается, нормализация
// выполняется после сборки. Непокрытый значимый остаток делает подпись неизвестной.
func v157ReadOption(line diagnostics.Line, correct bool) (v157Option, bool) {
	option := v157Option{line: line.Number(), correct: correct}
	raw := []rune(line.Raw())
	var label strings.Builder
	cursor := 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		value, present := element.Value()
		// Пустая вставка не занимает исходный текст, в том числе внутри уже
		// покрытого диапазона, и не сдвигает курсор или диапазон подписи.
		if element.Type() == "content" && present && value == "" && element.Start() == element.End() {
			continue
		}
		start, end := element.Start()-1, element.End()-1
		if start < cursor || strings.Trim(string(raw[cursor:start]), " \t") != "" {
			return v157Option{}, false
		}
		switch element.Type() {
		case "tag":
			if line.Type() != "tag" || option.start != 0 {
				return v157Option{}, false
			}
		case "content":
			if !present {
				return v157Option{}, false
			}
			if option.start == 0 {
				option.start = element.Start()
			} else {
				label.WriteString(string(raw[cursor:start]))
			}
			label.WriteString(value)
			option.end = element.End()
		default:
			return v157Option{}, false
		}
		cursor = end
	}
	option.label = answerPrimary(label.String())
	if strings.Trim(string(raw[cursor:]), " \t") != "" || option.label == "" {
		return v157Option{}, false
	}
	return option, true
}

// v157Report отмечает первое вхождение с противоположным признаком, остальные
// связывает в исходном порядке. Однородная группа сообщения не создаёт.
func v157Report(group []v157Option, findings diagnostics.Findings) error {
	primary := -1
	for index := 1; index < len(group); index++ {
		if group[index].correct != group[0].correct {
			primary = index
			break
		}
	}
	if primary < 0 {
		return nil
	}
	option := group[primary]
	occurrence, err := findings.Add(option.line, option.start, option.line, option.end,
		"Проверьте подписи вариантов и назначение правильных вариантов.")
	if err != nil {
		return err
	}
	for index, related := range group {
		if index == primary {
			continue
		}
		if err := findings.AddRelated(occurrence, related.line, related.start, related.line, related.end); err != nil {
			return err
		}
	}
	return nil
}

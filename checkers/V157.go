package checkers

import (
	"strings"
	"unicode"

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
				if v157Equal(groups[index][0].label, option.label) {
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

// v157ReadOption принимает только целиком и однозначно выделенную подпись.
// Value уже содержит результат общей синтаксической обработки. Неразобранные
// части, несколько content и непокрытый значимый остаток не склеиваются.
func v157ReadOption(line diagnostics.Line, correct bool) (v157Option, bool) {
	option := v157Option{line: line.Number(), correct: correct}
	raw := []rune(line.Raw())
	cursor := 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if strings.Trim(string(raw[cursor:element.Start()-1]), " \t") != "" {
			return v157Option{}, false
		}
		cursor = element.End() - 1
		switch element.Type() {
		case "tag":
			if line.Type() != "tag" {
				return v157Option{}, false
			}
		case "content":
			if option.start != 0 {
				return v157Option{}, false
			}
			value, present := element.Value()
			if !present {
				return v157Option{}, false
			}
			option.label = v157Spaces(strings.ToLower(value))
			option.start, option.end = element.Start(), element.End()
		default:
			return v157Option{}, false
		}
	}
	if strings.Trim(string(raw[cursor:]), " \t") != "" || option.label == "" {
		return v157Option{}, false
	}
	return option, true
}

// v157Spaces сворачивает Unicode White_Space и удаляет краевые пробелы.
func v157Spaces(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(char rune) bool {
		return unicode.Is(unicode.White_Space, char)
	}), " ")
}

// v157Equal выполняет строгое сравнение, а при несовпадении — дополнительное
// по 4.0.14–4.0.16. Пустые после основной нормализации подписи исключены ранее.
func v157Equal(left, right string) bool {
	if left == right {
		return true
	}
	withoutPunctuation := func(text string) string {
		return v157Spaces(strings.Map(func(char rune) rune {
			if unicode.IsPunct(char) {
				return -1
			}
			return char
		}, text))
	}
	left, right = withoutPunctuation(left), withoutPunctuation(right)
	return left != "" && right != "" && left == right
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

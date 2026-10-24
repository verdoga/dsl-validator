package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV111 регистрирует проверку отдельной команды br только для DSL 1.2.
func registrationV111(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V111",
		Severity: diagnostics.SeverityError,
		Message:  "Команда пустой строки отличается от отдельной записи <br>.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV111,
	})
}

// checkV111 отмечает всю строку с ошибочным br, исключая крайние U+0020/U+0009.
// Диапазоны исходные, в Unicode-колонках с 1; правая граница исключается.
func checkV111(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v111TextContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := 0, len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end || strings.EqualFold(string(raw[start:end]), "<br>") {
			continue
		}
		if detail := v111Reasons(raw[start:end]); detail != "" {
			if _, err := findings.Add(number, start+1, number, end+1, detail); err != nil {
				return err
			}
		}
	}
	return nil
}

// v111TextContent выбирает собственный текст text, включая ошибочный разбор.
// Структурные объявления, даже целиком unparsed, не становятся текстом блока.
func v111TextContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() != "content" && line.Type() != "invalid" {
		return false
	}
	parent, found := line.ParentLine()
	if !found {
		return false
	}
	block, _ := lines.Line(parent)
	if block.Type() != "block-start" {
		return false
	}
	text := false
	for index := 0; index < block.ElementCount(); index++ {
		element, _ := block.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			text = name == "text"
			break
		}
	}
	if !text {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" && element.Type() != "unparsed" && element.Type() != "placeholder" {
			return false
		}
	}
	raw := strings.TrimLeft(line.Raw(), " \t")
	return raw != "" && raw[0] != '@' && raw[0] != '#'
}

// v111Reasons объединяет причины разных br в исходном порядке категорий.
// Пустая строка означает отсутствие кандидата. Внутренние маркеры не ищутся.
func v111Reasons(raw []rune) string {
	var reasons [6]bool
	for index := 0; index < len(raw); {
		if v111Escape(raw, index) {
			index += 2
			continue
		}
		if raw[index] != '<' {
			index++
			continue
		}
		end, closed, quote := v111MarkerEnd(raw, index+1)
		name := index + 1
		closing := name < len(raw) && raw[name] == '/'
		if closing {
			name++
		}
		after := name + 2
		if after <= len(raw) && strings.EqualFold(string(raw[name:after]), "br") &&
			(after == len(raw) || unicode.Is(unicode.White_Space, raw[after]) || raw[after] == '/' || raw[after] == '>') {
			reasons[0] = reasons[0] || closing
			reasons[3] = reasons[3] || !closed
			reasons[5] = reasons[5] || index != 0 || end != len(raw)
			tail := end
			if closed {
				tail--
			}
			for tail > after && unicode.Is(unicode.White_Space, raw[tail-1]) {
				tail--
			}
			selfClosing := quote == 0 && tail > after && raw[tail-1] == '/'
			reasons[1] = reasons[1] || selfClosing
			if selfClosing {
				tail--
			}
			for tail > after && unicode.Is(unicode.White_Space, raw[tail-1]) {
				tail--
			}
			attributes := tail > after
			reasons[2] = reasons[2] || attributes
			// Пробел перед / и разделители атрибутов не дают отдельной причины.
			reasons[4] = reasons[4] || closed && !selfClosing && !attributes && end-1 > after
		}
		index = end
	}
	var details []string
	for index, label := range [...]string{"закрывающая форма", "самозакрывающаяся форма", "атрибуты",
		"незавершённый маркер", "лишние пробельные символы внутри маркера", "другое содержимое строки"} {
		if reasons[index] {
			details = append(details, label)
		}
	}
	return strings.Join(details, "; ")
}

// v111MarkerEnd возвращает исключающий конец маркера, наличие активного >
// и незакрытую кавычку (0 при её отсутствии). Кавычки не выходят за строку.
func v111MarkerEnd(raw []rune, start int) (end int, closed bool, quote rune) {
	for index := start; index < len(raw); index++ {
		if v111Escape(raw, index) {
			index++
			continue
		}
		if quote != 0 {
			if raw[index] == quote {
				quote = 0
			}
			continue
		}
		switch raw[index] {
		case '\'', '"':
			quote = raw[index]
		case '>':
			return index + 1, true, 0
		}
	}
	return len(raw), false, quote
}

// v111Escape распознаёт пары экранирования текстового контекста без декодирования.
// Обратная косая черта перед кавычкой здесь не является экранированием SOURCE.
func v111Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) && strings.ContainsRune("\\<>{}", raw[index+1])
}

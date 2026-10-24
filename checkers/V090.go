package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV090 регистрирует редакторскую рекомендацию только для DSL 1.2.
func registrationV090(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V090", Severity: diagnostics.SeverityRecommendation,
		Message:  "Первая строка блока подходит для рассмотрения в качестве заголовка.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV090,
	})
}

// checkV090 рассматривает собственные строки каждого безымянного блока.
func checkV090(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, ok := line.ParentLine(); ok {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		name := v090Block(block)
		if name == "" {
			continue
		}
		rows := children[number]
		for index, row := range rows {
			if strings.Trim(row.Raw(), " \t") == "" {
				continue
			}
			text, ok := v090Content(row, name)
			if ok && (v090Word(text) || v090Upper(text)) && v090Remaining(rows[index+1:], name) {
				if _, err := findings.Add(row.Number(), 1, row.Number(), utf8.RuneCountInString(row.Raw())+1,
					"Рассмотрите перенос этой строки в название блока; решение зависит от её роли в материале"); err != nil {
					return err
				}
			}
			break // Первая непустая исходная строка — единственный кандидат.
		}
	}
	return nil
}

// v090Block возвращает имя выбранного блока без названия либо пустую строку.
// Проверка исходного промежутка исключает также неразобранное название.
func v090Block(line diagnostics.Line) string {
	if line.Type() != "block-start" {
		return ""
	}
	name, end, open := "", 0, 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "tag":
			name, _ = element.Value()
			end = element.End()
		case "block-open":
			open = element.Start()
		case "title":
			if strings.Trim(element.Raw(), " \t") != "" {
				return ""
			}
		}
	}
	switch name {
	case "example", "table", "script", "text", "key", "note", "alt":
		if end > 0 && open >= end && strings.Trim(string([]rune(line.Raw())[end-1:open-1]), " \t") == "" {
			return name
		}
	}
	return ""
}

// v090Content возвращает текст для анализа и true только для интерпретируемого
// содержимого. Разделители, объявления и unparsed не становятся кандидатами.
func v090Content(line diagnostics.Line, name string) (string, bool) {
	if line.Type() != "content" || line.ElementCount() == 0 {
		return "", false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			return "", false
		}
	}
	raw := strings.Trim(line.Raw(), " \t")
	if (name == "example" || name == "table") && raw == "---" {
		return "", false
	}
	if name == "text" {
		return v090HTML(raw)
	}
	if name == "table" {
		for index := 0; index < len(raw); index++ {
			if raw[index] == '\\' && index+1 < len(raw) && strings.ContainsRune("\\|<>{}", rune(raw[index+1])) {
				index++
				continue
			}
			if raw[index] == '|' || (index == 0 && raw[index] == '<') || (index == len(raw)-1 && raw[index] == '>') {
				return "", false
			}
		}
	}
	return raw, true
}

// v090Remaining требует содержимое в той же части блока. После разделителя
// данные уже не компенсируют удаление единственной строки предыдущей части.
func v090Remaining(rows []diagnostics.Line, name string) bool {
	for _, row := range rows {
		raw := strings.Trim(row.Raw(), " \t")
		if row.Type() == "block-end" || ((name == "example" || name == "table") && raw == "---") {
			return false
		}
		if name == "text" && v090PlaceholderContent(row) {
			return true
		}
		// Для оставшейся строки table допустимы несколько ячеек: ограничение
		// одной обычной ячейкой относится только к переносимому кандидату.
		contentName := name
		if name == "table" {
			contentName = "example"
		}
		if text, ok := v090Content(row, contentName); ok && strings.Trim(text, " \t") != "" {
			return true
		}
	}
	return false
}

// v090PlaceholderContent учитывает плейсхолдер в оставшемся содержимом text.
// Кандидат проверяется отдельно; ошибочная разметка содержимым не считается.
func v090PlaceholderContent(line diagnostics.Line) bool {
	if line.Type() != "content" {
		return false
	}
	placeholder := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "placeholder":
			placeholder = true
		case "content":
		default:
			return false
		}
	}
	if !placeholder {
		return false
	}
	text, ok := v090HTML(strings.Trim(line.Raw(), " \t"))
	return ok && strings.Trim(text, " \t") != ""
}

// v090HTML снимает только поддерживаемые парные маркеры с правильной
// вложенностью. false означает неинтерпретируемую разметку; <br> даёт пустой текст.
func v090HTML(raw string) (string, bool) {
	if strings.EqualFold(raw, "<br>") {
		return "", true
	}
	var text strings.Builder
	var stack []string
	for index := 0; index < len(raw); {
		if raw[index] == '\\' && index+1 < len(raw) && strings.ContainsRune("\\<>{}", rune(raw[index+1])) {
			text.WriteString(raw[index : index+2])
			index += 2
			continue
		}
		if raw[index] == '>' {
			return "", false
		}
		if raw[index] != '<' {
			text.WriteByte(raw[index])
			index++
			continue
		}
		end := strings.IndexByte(raw[index:], '>')
		if end < 0 {
			return "", false
		}
		marker := strings.ToLower(raw[index+1 : index+end])
		switch marker {
		case "strong", "em", "u":
			stack = append(stack, marker)
		case "/strong", "/em", "/u":
			if len(stack) == 0 || stack[len(stack)-1] != marker[1:] {
				return "", false
			}
			stack = stack[:len(stack)-1]
		default:
			return "", false
		}
		index += end + 1
	}
	return text.String(), len(stack) == 0
}

// v090Word проверяет согласованную форму одного слова без цифр.
// Краевая Unicode-пунктуация разрешена; диакритики относятся к предыдущей букве.
func v090Word(text string) bool {
	runes := []rune(strings.TrimFunc(text, unicode.IsPunct))
	letter := false
	for index, r := range runes {
		switch {
		case v090Connector(r):
			if !letter || index+1 == len(runes) || !unicode.IsLetter(runes[index+1]) || v090Connector(runes[index+1]) {
				return false
			}
			letter = false
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsMark(r):
			if !letter {
				return false
			}
		default:
			return false
		}
	}
	return letter
}

// v090Connector перечисляет только согласованные соединительные знаки слова.
func v090Connector(r rune) bool {
	return r == '-' || r == '\u2010' || r == '\u2011' || r == '\'' || r == '\u2019' || r == '\u02bc'
}

// v090Upper требует хотя бы одну регистровую букву и только верхний регистр.
func v090Upper(text string) bool {
	upper := false
	for _, r := range text {
		if unicode.IsLower(r) || unicode.IsTitle(r) || unicode.Is(unicode.Other_Lowercase, r) {
			return false
		}
		if unicode.IsUpper(r) || unicode.Is(unicode.Other_Uppercase, r) {
			upper = true
		}
	}
	return upper
}

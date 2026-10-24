package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV116 регистрирует рекомендацию о названии text только для DSL 1.2.
func registrationV116(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V116", Severity: diagnostics.SeverityRecommendation, Scope: diagnostics.ScopeLine,
		Message:  "Первая строка текста имеет признаки заголовка.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV116,
	})
}

// v116Rune сохраняет видимый символ и его исходную Unicode-колонку с единицы.
type v116Rune struct {
	value  rune
	column int
}

// checkV116 группирует непосредственные строки по существующему ParentLine.
// Вложенные text проверяются независимо; include не разворачивается.
func checkV116(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if !v116Unnamed(block) || v116Literal(lines, block) {
			continue
		}
		candidate := v116Candidate(children[number])
		if candidate == nil {
			continue
		}
		if _, err := findings.Add(candidate.Number(), 1, candidate.Number(), utf8.RuneCountInString(candidate.Raw())+1,
			"Если это заголовок текста, перенесите его в TITLE после @text; решение принимает редактор"); err != nil {
			return err
		}
	}
	return nil
}

// v116Element возвращает единственный элемент типа; nil — нет или неоднозначен.
func v116Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}

// v116Tag читает распознанное имя, не восстанавливая объявления из текста.
func v116Tag(line diagnostics.Line) string {
	if tag := v116Element(line, "tag"); tag != nil {
		value, _ := tag.Value()
		return value
	}
	return ""
}

// v116Unnamed подтверждает отсутствие TITLE и в элементах, и в исходнике.
func v116Unnamed(line diagnostics.Line) bool {
	if line.Type() != "block-start" || !strings.EqualFold(v116Tag(line), "text") {
		return false
	}
	tag, brace := v116Element(line, "tag"), v116Element(line, "block-open")
	if brace == nil || tag.End() > brace.Start() {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "title" {
			value, _ := element.Value()
			if strings.Trim(value, " \t") != "" {
				return false
			}
		}
	}
	raw := []rune(line.Raw())
	return strings.Trim(string(raw[tag.End()-1:brace.Start()-1]), " \t") == ""
}

// v116Literal исключает потомков литеральных editor и multifill.
func v116Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" && (strings.EqualFold(v116Tag(line), "editor") || strings.EqualFold(v116Tag(line), "multifill")) {
			return true
		}
	}
}

// v116TextRow отделяет текст от объявлений, заголовков и границ блока.
func v116TextRow(line diagnostics.Line) bool {
	if line.Type() != "content" && line.Type() != "invalid" || v116Tag(line) != "" {
		return false
	}
	raw := strings.TrimLeft(line.Raw(), " \t")
	return raw != "" && raw[0] != '@' && raw[0] != '#' && raw != "}" && raw != "{"
}

// v116Candidate возвращает единственного кандидата либо nil. Пустые строки
// не разрывают прописную серию; отдельный br и DSL-конструкция разрывают её.
func v116Candidate(rows []diagnostics.Line) diagnostics.Line {
	for index, line := range rows {
		raw := strings.Trim(line.Raw(), " \t")
		if raw == "" {
			continue
		}
		if !v116TextRow(line) {
			return nil
		}
		if strings.EqualFold(raw, "<br>") {
			continue
		}
		visible, valid := v116Visible(line.Raw())
		upper := v116Upper(visible)
		if !valid || !upper && !v116Word(visible) {
			return nil
		}
		neighbour := upper
		for _, next := range rows[index+1:] {
			if next.Type() == "block-end" {
				break
			}
			raw = strings.Trim(next.Raw(), " \t")
			if raw == "" {
				continue
			}
			if !v116TextRow(next) || strings.EqualFold(raw, "<br>") {
				neighbour = false
				continue
			}
			text, ok := v116Visible(next.Raw())
			if neighbour && ok && v116Upper(text) {
				return nil
			}
			neighbour = false
			if v116HasText(text) {
				return line
			}
		}
		return nil
	}
	return nil
}

// v116Visible выделяет текст между разрешёнными маркерами и проверяет их пары.
// false означает ошибочную HTML-разметку: для кандидата такой результат
// непригоден, но уже распознанный текст сохраняется для проверки остатка блока.
// Ошибочные завершённые маркеры пропускаются вместе с атрибутами: текст
// после них сохраняется, но строка остаётся непригодной для кандидата.
// Исходные позиции сохраняются;
// результат декодирования не разбирается повторно, сущности не декодируются.
func v116Visible(source string) ([]v116Rune, bool) {
	raw := []rune(source)
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	var visible []v116Rune
	var stack []string
	valid := true
	for index := start; index < end; {
		char := raw[index]
		if char == '\\' && index+1 < end {
			next := raw[index+1]
			leading := index == start && (next == '@' || next == '#')
			underscores := next == '_' && index+2 < end && raw[index+2] == '_'
			if next == '\\' || next == '{' || next == '}' || next == '<' || next == '>' || leading || underscores {
				visible = append(visible, v116Rune{next, index + 2})
				index += 2
				continue
			}
		}
		if char == '>' {
			valid = false
			index++
			continue
		}
		if char == '<' {
			matched := false
			for _, marker := range [...]string{"<strong>", "</strong>", "<em>", "</em>", "<u>", "</u>"} {
				if index+len(marker) > end || !strings.EqualFold(string(raw[index:index+len(marker)]), marker) {
					continue
				}
				closing := marker[1] == '/'
				name := marker[1 : len(marker)-1]
				if closing {
					name = name[1:]
					if len(stack) == 0 || stack[len(stack)-1] != name {
						valid = false
					} else {
						stack = stack[:len(stack)-1]
					}
				} else {
					stack = append(stack, name)
				}
				index += len(marker)
				matched = true
				break
			}
			if !matched {
				valid = false
				endMarker := v116MarkupEnd(raw, index+1, end)
				if endMarker == end {
					return visible, false
				}
				index = endMarker + 1
			}
			continue
		}
		visible = append(visible, v116Rune{char, index + 1})
		index++
	}
	for len(visible) > 0 && (visible[0].value == ' ' || visible[0].value == '\t') {
		visible = visible[1:]
	}
	for len(visible) > 0 && (visible[len(visible)-1].value == ' ' || visible[len(visible)-1].value == '\t') {
		visible = visible[:len(visible)-1]
	}
	return visible, valid && len(stack) == 0
}

// v116MarkupEnd возвращает индекс активного > вне кавычек либо end,
// если конец маркера не установлен. Атрибуты не становятся текстом;
// пары экранирования text потребляются до поиска границы и не декодируются.
func v116MarkupEnd(raw []rune, start, end int) int {
	var quote rune
	for index := start; index < end; index++ {
		char := raw[index]
		if char == '\\' && index+1 < end && strings.ContainsRune("\\<>", raw[index+1]) {
			index++
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
		} else if char == '>' {
			return index
		}
	}
	return end
}

// v116HasText проверяет непустоту достоверно распознанной части строки.
// При ранней ошибке HTML эта часть может ещё содержать крайние U+0020/U+0009.
func v116HasText(text []v116Rune) bool {
	for _, char := range text {
		if char.value != ' ' && char.value != '\t' {
			return true
		}
	}
	return false
}

// v116Upper требует хотя бы одну регистровую букву и только прописные среди них.
// Категория Lt не считается прописной; числа и буквы без регистра не создают признак.
func v116Upper(text []v116Rune) bool {
	found := false
	for _, char := range text {
		if !unicode.IsLetter(char.value) {
			continue
		}
		if unicode.IsLower(char.value) || unicode.IsTitle(char.value) || unicode.Is(unicode.Other_Lowercase, char.value) {
			return false
		}
		if unicode.IsUpper(char.value) || unicode.Is(unicode.Other_Uppercase, char.value) {
			found = true
		}
	}
	return found
}

// v116Connector задаёт согласованные внутрисловные дефисы и апострофы.
func v116Connector(char rune) bool {
	return char == '-' || char == '\u2010' || char == '\u2011' || char == '\'' || char == '\u2019' || char == '\u02bc'
}

// v116Word допускает одну буквенную последовательность с диакритиками,
// соединителями между буквенными частями и пунктуацией по краям.
func v116Word(text []v116Rune) bool {
	for len(text) > 0 && (unicode.IsPunct(text[0].value) || v116Connector(text[0].value)) {
		text = text[1:]
	}
	for len(text) > 0 && (unicode.IsPunct(text[len(text)-1].value) || v116Connector(text[len(text)-1].value)) {
		text = text[:len(text)-1]
	}
	needLetter := true
	for _, char := range text {
		if v116Connector(char.value) {
			if needLetter {
				return false
			}
			needLetter = true
		} else if unicode.IsLetter(char.value) {
			needLetter = false
		} else if !unicode.IsMark(char.value) || needLetter {
			return false
		}
	}
	return !needLetter
}

package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v021Row хранит признаки одной строки непосредственного содержимого.
// gapRow и labeledGap учитывают настоящие пропуски, а не их экранированный вид.
type v021Row struct {
	number             int
	text               []rune
	gapRow, labeledGap bool
}

// v021IndependentRows отмечает только подтверждённые самостоятельные серии.
// Индекс результата — физический номер строки, нулевой элемент не используется.
// Границы контейнеров и конструкций разрывают серию; blank и отдельный <br>
// в text прозрачны, как при поиске разорванного абзаца.
func v021IndependentRows(lines diagnostics.Lines) []bool {
	independent := make([]bool, lines.Len()+1)
	var rows []v021Row
	container, tag := 0, ""
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		if parent != container {
			v021MarkSeries(rows, tag, independent)
			rows = rows[:0]
			container, tag = parent, v021ContainerTag(lines, parent)
		}
		if tag == "" || line.Type() == "blank" || line.Type() == "content" && tag == "text" &&
			strings.EqualFold(strings.Trim(line.Raw(), " \t"), "<br>") {
			continue
		}
		if line.Type() != "content" {
			v021MarkSeries(rows, tag, independent)
			rows = rows[:0]
			continue
		}
		text, gap, labeled := v021Visible(line, tag)
		rows = append(rows, v021Row{number, text, gap, labeled})
	}
	v021MarkSeries(rows, tag, independent)
	v021MarkStressLists(lines, independent)
	return independent
}

// v021MarkSeries требует как минимум две соседние записи одного вида.
// Вводное двоеточие исключается только вместе с серией поясняемых пунктов.
func v021MarkSeries(rows []v021Row, tag string, independent []bool) {
	previousKind := 0
	for i, row := range rows {
		kind := v021RowKind(row, tag)
		if i > 0 && kind&previousKind != 0 {
			independent[rows[i-1].number], independent[row.number] = true, true
		}
		previousKind = kind
	}
	if tag != "note" && tag != "instr" {
		return
	}
	for i := 0; i < len(rows); i++ {
		text := rows[i].text
		if len(text) == 0 || text[len(text)-1] != ':' {
			continue
		}
		end := i + 1
		for end < len(rows) && v021ExampleRow(rows[end].text) {
			end++
		}
		if end-i < 3 {
			continue
		}
		for _, row := range rows[i:end] {
			independent[row.number] = true
		}
		i = end - 1
	}
}

// v021RowKind возвращает совместимые признаки серии; ноль означает обычную
// строку. Наличие одного признака без соседнего подтверждения не исключает её.
func v021RowKind(row v021Row, tag string) int {
	const (
		gap = 1 << iota
		labeled
		equation
		numbered
	)
	kind := 0
	if row.gapRow {
		kind |= gap
	}
	if row.labeledGap {
		kind |= labeled
	}
	// Соответствие связывает две фразы. Само наличие равенства в каждой
	// строке рассуждения (например, then x = y + 1.) не делает их списком.
	if index := v021Separator(row.text, '='); index > 0 &&
		v021Phrase(row.text[:index]) && v021Phrase(row.text[index+1:]) {
		kind |= equation
	}
	if prefix, suffix := v021Parenthetical(row.text); tag == "key" && v021Phrase(prefix) && len(suffix) > 0 {
		digits := true
		for _, char := range suffix {
			digits = digits && unicode.IsDigit(char)
		}
		if digits {
			kind |= numbered
		}
	}
	return kind
}

// v021LabelPrefix подтверждает подпись перед настоящим пропуском multifill.
// Двоеточие должно быть вне скобок и кавычек; слева допустима фраза с косой
// чертой (например, I/we). Само двоеточие в прозе не является исключением.
func v021LabelPrefix(text []rune) bool {
	index := v021Separator(text, ':')
	return index > 0 && v021Phrase(text[:index])
}

// v021Separator возвращает индекс единственного верхнеуровневого разделителя
// либо -1. Скобки, двойные кавычки и экранированные символы не раскрываются.
func v021Separator(text []rune, separator rune) int {
	index := -1
	var stack []rune
	quoted := false
	for i := 0; i < len(text); i++ {
		char := text[i]
		if char == '\\' {
			i++
			continue
		}
		if char == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		switch char {
		case '(', '[', '{':
			stack = append(stack, char)
		case ')', ']', '}':
			if len(stack) == 0 ||
				strings.IndexRune("([{", stack[len(stack)-1]) != strings.IndexRune(")]}", char) {
				return -1
			}
			stack = stack[:len(stack)-1]
		default:
			if char == separator && len(stack) == 0 {
				if index != -1 {
					return -1
				}
				index = i
			}
		}
	}
	if quoted || len(stack) != 0 {
		return -1
	}
	return index
}

// v021Parenthetical выделяет конечную сбалансированную пометку (…).
// Нужен пробел перед открывающей скобкой; при иной форме возвращаются nil.
func v021Parenthetical(text []rune) (prefix, suffix []rune) {
	if len(text) < 4 || text[len(text)-1] != ')' {
		return nil, nil
	}
	depth := 0
	for i := len(text) - 1; i >= 0; i-- {
		switch text[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				if i > 0 && unicode.IsSpace(text[i-1]) {
					return text[:i-1], text[i+1 : len(text)-1]
				}
				return nil, nil
			}
		}
	}
	return nil, nil
}

// v021ExampleRow подтверждает пункт с фразой и непустым буквенным примером.
// Без вводной строки и второго такого пункта этот признак ничего не исключает.
func v021ExampleRow(text []rune) bool {
	prefix, example := v021Parenthetical(text)
	return v021StartsLower(prefix) && v021Phrase(prefix) && v021HasLetter(example)
}

// v021Phrase допускает слова, диакритики, пробелы, апострофы, дефисы и косую
// черту. Пунктуация предложения, пропуски и числовые записи дают false.
func v021Phrase(text []rune) bool {
	for _, char := range text {
		if !unicode.IsLetter(char) && !unicode.IsMark(char) && !unicode.IsSpace(char) &&
			char != '\'' && char != '’' && char != '/' && !unicode.Is(unicode.Pd, char) {
			return false
		}
	}
	return v021HasLetter(text)
}

// v021HasLetter возвращает true при наличии хотя бы одной буквы Unicode.
func v021HasLetter(text []rune) bool {
	for _, char := range text {
		if unicode.IsLetter(char) {
			return true
		}
	}
	return false
}

// v021ListMarker отличает явный пункт от обычного слова по начальным рунам.
// За буквой нужны . : или ), за номером — . или ); далее пробел или конец строки.
func v021ListMarker(text []rune) bool {
	if len(text) < 2 {
		return false
	}
	if unicode.IsLower(text[0]) && (text[1] == '.' || text[1] == ':' || text[1] == ')') {
		return len(text) == 2 || unicode.IsSpace(text[2])
	}
	i := 0
	for i < len(text) && unicode.IsDigit(text[i]) {
		i++
	}
	return i > 0 && i < len(text) && (text[i] == '.' || text[i] == ')') &&
		(i+1 == len(text) || unicode.IsSpace(text[i+1]))
}

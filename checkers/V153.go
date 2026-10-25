package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV153 регистрирует специальную форму пропуска вне multifill.
func registrationV153(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V153",
		Severity: diagnostics.SeverityError,
		Message:  "Ответ в фигурных скобках записан вне содержимого @multifill.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV153,
	})
}

// checkV153 проверяет исходные текстовые диапазоны без декодирования и склейки.
// Смежные элементы могут представлять части одного ошибочного шаблона;
// непроверяемый промежуток разделяет области, физическая строка завершает ответ.
func checkV153(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v153TextContext(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		text := v153TextMask(line, len(raw))
		for start := 0; start < len(raw); {
			if !text[start] {
				start++
				continue
			}
			end := start + 1
			for end < len(raw) && text[end] {
				end++
			}
			if err := v153Scan(raw, start, end, number, findings); err != nil {
				return err
			}
			start = end
		}
	}
	return nil
}

// v153Name возвращает имя начального объявления и конец имени в индексах рун.
// Raw восстанавливает имя только при известном объявлении или частичном разборе.
// Пустое имя означает обычный текст; экранированное @ не начинает объявление.
func v153Name(line diagnostics.Line) (string, int) {
	declaration := line.Type() == "tag" || line.Type() == "block-start" || line.Type() == "invalid"
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		declaration = declaration || element.Type() == "tag" || element.Type() == "unparsed"
	}
	if !declaration {
		return "", 0
	}
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	end := start + 1
	for end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	return strings.ToLower(string(raw[start+1 : end])), end
}

// v153TextContext разрешает учебный текст и инструкции, исключая служебные области.
// Литеральные editor/multifill исключаются по всей цепочке; остальные вложенные
// конструкции имеют собственный контекст. Объявление multifill остаётся инструкцией.
func v153TextContext(lines diagnostics.Lines, current diagnostics.Line) bool {
	switch current.Type() {
	case "heading", "blank", "separator", "block-end":
		return false
	}
	var nearest diagnostics.Line
	structural, branch := false, false
	for line := current; ; {
		parent, found := line.ParentLine()
		if !found {
			break
		}
		line, _ = lines.Line(parent)
		name, _ := v153Name(line)
		if line.Type() == "block-start" {
			if name == "editor" || name == "multifill" {
				return false
			}
			if nearest == nil {
				nearest = line
			}
		}
		branch = branch || name == "variant"
		structural = structural || line.Type() == "heading" || name == "task" || name == "step" || branch
	}
	if name, _ := v153Name(current); name != "" {
		switch name {
		case "question", "multifill", "choice", "multichoice", "matching", "ordering":
			return true
		case "example", "wordlist", "instr", "note", "alt", "hint":
			// У блочных контентных тегов здесь TITLE либо запрещённый аргумент,
			// а собственное содержимое начинается на следующих строках.
			return current.Type() != "block-start"
		default:
			// Остальные объявления содержат служебные аргументы, TITLE или NAME.
			return false
		}
	}
	if nearest == nil {
		return structural
	}
	name, _ := v153Name(nearest)
	switch name {
	case "example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint", "fragment":
		return true
	case "matching", "ordering":
		// Верхние элементы — учебный текст. Нижние варианты/позиции исключены.
		for n := nearest.Number() + 1; n < current.Number(); n++ {
			line, _ := lines.Line(n)
			parent, _ := line.ParentLine()
			if parent == nearest.Number() && line.Type() == "separator" {
				return false
			}
		}
		return true
	case "variants":
		return branch
	default:
		return false
	}
}

// v153TextMask отмечает исходные позиции content, placeholder и восстановимого
// unparsed в уже разрешённом контексте. Имена тегов и структурные скобки исключены.
// Промежутки и служебные элементы не достраиваются по предполагаемому содержимому.
func v153TextMask(line diagnostics.Line, length int) []bool {
	mask := make([]bool, length)
	_, nameEnd := v153Name(line)
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		switch element.Type() {
		case "content", "placeholder", "unparsed":
			for j := max(element.Start()-1, nameEnd); j < element.End()-1; j++ {
				mask[j] = true
			}
		}
	}
	return mask
}

// v153Scan ищет максимальные активные последовательности в области [start,end).
// Основной диапазон охватывает {, связанный — пять подчёркиваний; конец исключён.
func v153Scan(raw []rune, start, end, number int, findings diagnostics.Findings) error {
	for i := start; i < end; {
		if raw[i] != '_' {
			i++
			continue
		}
		first := i
		for i < end && raw[i] == '_' {
			i++
		}
		if i-first != 5 || first > 0 && raw[first-1] == '_' ||
			i == end || raw[i] != '{' || v153Escaped(raw, first) {
			continue
		}
		occurrence, err := findings.Add(number, i+1, number, i+2, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, number, first+1, number, first+6); err != nil {
			return err
		}
		i = v153SkipAnswers(raw, i+1, end)
	}
	return nil
}

// v153Escaped возвращает true при нечётном числе косых черт перед позицией.
// Чётные пары не экранируют следующий символ; исходный текст не декодируется.
func v153Escaped(raw []rune, position int) bool {
	start := position
	for start > 0 && raw[start-1] == '\\' {
		start--
	}
	return (position-start)%2 != 0
}

// v153SkipAnswers пропускает служебные значения до первого активного } в каждой
// соседней группе. Вложенные { не образуют стек; отсутствие закрытия завершает область.
func v153SkipAnswers(raw []rune, start, end int) int {
	for i := start; i < end; {
		if raw[i] == '\\' && i+1 < end && strings.ContainsRune("\\{}", raw[i+1]) {
			i += 2
			continue
		}
		if raw[i] == '}' {
			i++
			if i < end && raw[i] == '{' {
				i++
				continue
			}
			return i
		}
		i++
	}
	return end
}

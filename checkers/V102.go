package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV102 регистрирует проверку текста после закрытой разделённой ячейки.
func registrationV102(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V102",
		Severity: diagnostics.SeverityError,
		Message:  "После закрывающего знака > есть текст до следующего разделителя ячеек.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV102,
	})
}

// checkV102 проверяет Raw непосредственных строк данных table независимо
// от разбиения на элементы, включая unparsed. Состояние между строками не живёт.
func checkV102(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v102TableRow(lines, line) {
			if err := v102Row(line, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v102TableRow возвращает true для непосредственной строки содержимого table.
// Объявления, разделители, закрытия и содержимое других вложенных конструкций
// не являются строками данных этой таблицы. Родительство берётся из модели.
func v102TableRow(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() != "content" {
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
	for index := 0; index < block.ElementCount(); index++ {
		element, _ := block.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "table"
		}
	}
	return false
}

// v102Row ищет невложенные закрытые ячейки и проверяет весь остаток каждой.
// Колонки исходные, Unicode, с 1; конец основного и связанного диапазона исключён.
func v102Row(line diagnostics.Line, findings diagnostics.Findings) error {
	raw := []rune(line.Raw())
	first := true
	for index := 0; index < len(raw); {
		if v102Escape(raw, index) {
			first = false
			index += 2
			continue
		}
		if raw[index] == '|' {
			first = true
			index++
			continue
		}
		if first && v102Space(raw[index]) {
			index++
			continue
		}
		if !first || raw[index] != '<' {
			first = false
			index++
			continue
		}
		closing, nested := v102Closing(raw, index)
		if closing < 0 {
			return nil // До конца строки больше нет внешнего разделителя.
		}
		stop := closing + 1
		for stop < len(raw) {
			if v102Escape(raw, stop) {
				stop += 2
			} else if raw[stop] == '|' {
				break
			} else {
				stop++ // Угловые скобки после закрытия остаются текстом.
			}
		}
		start, end := closing+1, stop
		for start < end && v102Space(raw[start]) {
			start++
		}
		for end > start && v102Space(raw[end-1]) {
			end--
		}
		if !nested && start < end {
			occurrence, err := findings.Add(line.Number(), start+1, line.Number(), end+1,
				"Следующая внешняя ячейка отделяется вертикальной чертой; буквальный > внутри части экранируется")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, line.Number(), closing+1, line.Number(), closing+2); err != nil {
				return err
			}
		}
		index, first = stop, false
	}
	return nil
}

// v102Closing возвращает индекс парной > с 0 и признак вложенного открытия.
// Глубина нужна только для восстановления границ; -1 означает отсутствие закрытия.
func v102Closing(raw []rune, opening int) (int, bool) {
	depth, nested := 1, false
	for index := opening + 1; index < len(raw); index++ {
		if v102Escape(raw, index) {
			index++
			continue
		}
		switch raw[index] {
		case '<':
			depth++
			nested = true
		case '>':
			depth--
			if depth == 0 {
				return index, nested
			}
		}
	}
	return -1, nested
}

// v102Escape распознаёт только пары \\, \|, \<, \> без декодирования Raw.
func v102Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) &&
		(raw[index+1] == '\\' || raw[index+1] == '|' || raw[index+1] == '<' || raw[index+1] == '>')
}

// v102Space ограничивает пробельность U+0020/U+0009 согласно принятому поведению.
func v102Space(char rune) bool {
	return char == ' ' || char == '\t'
}

package checkers

import (
	"fmt"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV148 регистрирует проверку серий подчёркиваний только для DSL 1.2.
func registrationV148(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V148", Severity: diagnostics.SeverityError,
		Message:  "Подчёркивания не образуют допустимый пропуск.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV148,
	})
}

// checkV148 проверяет Raw выбранных текстовых участков без нормализации.
// Соседние элементы не разрывают серию, но исключённые участки и строки разделяют её.
func checkV148(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		allowed, answers, html := v148Context(lines, line)
		if !allowed {
			continue
		}
		raw := []rune(line.Raw())
		selected := make([]bool, len(raw))
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "content" && element.Type() != "placeholder" {
				continue
			}
			for position := element.Start() - 1; position < element.End()-1; position++ {
				selected[position] = true
			}
		}
		for start := 0; start < len(raw); {
			if !selected[start] {
				start++
				continue
			}
			end := start + 1
			for end < len(raw) && selected[end] {
				end++
			}
			if err := v148Text(findings, number, raw, start, end, answers, html); err != nil {
				return err
			}
			start = end
		}
	}
	return nil
}

// v148Context возвращает доступность текста, групп ANSWER и HTML-маркеров.
// Содержимое вложенных конструкций определяется ближайшим блоком; editor литерален.
func v148Context(lines diagnostics.Lines, line diagnostics.Line) (allowed, answers, html bool) {
	switch line.Type() {
	case "heading", "block-end", "blank", "separator":
		return false, false, false
	}
	var block diagnostics.Line
	for ancestor := line; ; {
		parent, found := ancestor.ParentLine()
		if !found {
			break
		}
		ancestor, _ = lines.Line(parent)
		if ancestor.Type() != "block-start" {
			continue
		}
		if v148Tag(ancestor) == "editor" {
			return false, false, false
		}
		if block == nil {
			block = ancestor
		}
	}
	if name := v148Tag(line); name != "" {
		// CONTENT объявления — учебный текст или инструкция. TITLE и параметры
		// отсекаются отдельно по типам элементов, без разбора строки объявления.
		switch name {
		case "example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint", "fragment",
			"question", "multifill", "choice", "multichoice", "matching", "ordering":
			return true, false, false
		default:
			return false, false, false
		}
	}
	if block == nil {
		return true, false, false
	}
	switch v148Tag(block) {
	case "key", "answer", "choice", "multichoice":
		return false, false, false
	case "matching", "ordering":
		// Только нижний список содержит варианты или позиции. Учебные элементы
		// верхнего списка и сокращённого ordering остаются проверяемым текстом.
		for number := block.Number() + 1; number < line.Number(); number++ {
			candidate, _ := lines.Line(number)
			parent, _ := candidate.ParentLine()
			if parent == block.Number() && candidate.Type() == "separator" {
				return false, false, false
			}
		}
	case "multifill":
		return true, true, false
	case "text":
		return true, false, true
	}
	return true, false, false
}

// v148Tag читает каноническое имя реального объявления из готовой модели.
func v148Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return ""
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			value, _ := element.Value()
			return value
		}
	}
	return ""
}

// v148Text проверяет один непрерывный участок [start,end) исходных рун.
// Распознавание границ HTML использует существующие функции пакета по их контракту;
// результаты V115 не читаются. Ошибки формы ANSWER не проверяются (уточнение A).
func v148Text(findings diagnostics.Findings, number int, raw []rune, start, end int, answers, html bool) error {
	var markerEnds []int
	if html {
		markerEnds = v115MarkerEnds(raw[start:end])
	}
	for index := start; index < end; {
		if html && raw[index] == '<' && !v148Escaped(raw, index) {
			_, markerEnd := v115Marker(raw[start:end], index-start, markerEnds)
			if markerEnd != 0 {
				index = start + markerEnd
				continue
			}
		}
		if raw[index] != '_' {
			index++
			continue
		}
		first := index
		for index < end && raw[index] == '_' {
			index++
		}
		escaped := v148Escaped(raw, first)
		if escaped || index-first != 5 {
			detail := "Литеральное употребление подчёркиваний запрещено."
			if !escaped {
				detail = fmt.Sprintf("Длина последовательности: %d; требуется 5.", index-first)
			}
			if _, err := findings.Add(number, first+1, number, index+1, detail); err != nil {
				return err
			}
			continue
		}
		if answers {
			// Служебные ответы могут идти несколькими соседними группами.
			// Незавершённый ответ ограничен текущим текстовым участком и строкой.
			for index < end && raw[index] == '{' {
				index = v148AnswerEnd(raw, index+1, end)
			}
		}
	}
	return nil
}

// v148Escaped возвращает true при действующем маркере экранирования перед позицией.
// Пары обратных косых черт не экранируют следующий символ (DSL 1.22).
func v148Escaped(raw []rune, position int) bool {
	first := position
	for first > 0 && raw[first-1] == '\\' {
		first--
	}
	return (position-first)%2 != 0
}

// v148AnswerEnd возвращает позицию после первой неэкранированной } либо end.
// Внутри ANSWER только скобки и обратная косая черта образуют пары экранирования.
func v148AnswerEnd(raw []rune, index, end int) int {
	for index < end {
		if raw[index] == '\\' && index+1 < end && (raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index += 2
			continue
		}
		if raw[index] == '}' {
			return index + 1
		}
		index++
	}
	return end
}

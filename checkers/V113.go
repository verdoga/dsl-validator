package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV113 регистрирует проверку порядка закрытия выделений в text.
func registrationV113(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V113",
		Severity: diagnostics.SeverityError,
		Message:  "Теги выделения закрыты не в порядке вложения.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV113,
	})
}

// v113Marker хранит имя (0=strong, 1=em, 2=u), исходный диапазон в рунах с 0
// и индекс парного маркера. Конец исключён; pair=-1 означает отсутствие пары.
type v113Marker struct {
	name, start, end, pair int
	closing                bool
}

// checkV113 проверяет каждую физическую строку независимо от остальных
// и от диагностик непарности. Raw не ограничивается элементами content.
func checkV113(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v113TextRow(lines, line) {
			continue
		}
		markers := v113Markers([]rune(line.Raw()))
		v113Pair(markers)
		if err := v113Order(number, markers, findings); err != nil {
			return err
		}
	}
	return nil
}

// v113TextRow возвращает true для непосредственной строки содержимого text.
// Названия, границы и посторонние вложенные конструкции не являются такими строками.
func v113TextRow(lines diagnostics.Lines, line diagnostics.Line) bool {
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
			return name == "text"
		}
	}
	return false
}

// v113Markers выделяет полные маркеры, сохраняя исходные позиции.
// Незнакомые имена также обходятся целиком: маркеры внутри их атрибутов
// не становятся выделениями. Самозакрывающиеся маркеры в результат не входят.
func v113Markers(raw []rune) []v113Marker {
	var markers []v113Marker
	for start := 0; start < len(raw); start++ {
		if v113Escape(raw, start) {
			start++
			continue
		}
		if raw[start] != '<' {
			continue
		}
		nameStart := start + 1
		closing := nameStart < len(raw) && raw[nameStart] == '/'
		if closing {
			nameStart++
		}
		if nameStart == len(raw) || !(raw[nameStart] >= 'a' && raw[nameStart] <= 'z' || raw[nameStart] >= 'A' && raw[nameStart] <= 'Z') {
			continue
		}
		nameEnd := nameStart
		// Имя отделяется ASCII-пробелами HTML, включая U+000C.
		// Unicode-пробелы вроде NBSP частью этого набора не являются.
		for nameEnd < len(raw) && !strings.ContainsRune(" \t\n\f\r/>", raw[nameEnd]) {
			nameEnd++
		}
		end := v113MarkerEnd(raw, nameEnd)
		if end < 0 {
			break // Незавершённый маркер, включая кавычки, ограничен этой строкой.
		}
		last := end - 1
		for last > nameStart && strings.ContainsRune(" \t\n\f\r", raw[last]) {
			last--
		}
		name := -1
		switch strings.ToLower(string(raw[nameStart:nameEnd])) {
		case "strong":
			name = 0
		case "em":
			name = 1
		case "u":
			name = 2
		}
		if name >= 0 && raw[last] != '/' {
			markers = append(markers, v113Marker{name: name, start: start, end: end + 1, pair: -1, closing: closing})
		}
		start = end
	}
	return markers
}

// v113MarkerEnd находит неэкранированную > вне одинарных и двойных кавычек.
// Возвращает индекс с 0 либо -1 при незавершённом маркере. Обратная косая
// перед кавычкой не экранирует её: правило SOURCE не переносится в text.
func v113MarkerEnd(raw []rune, start int) int {
	var quote rune
	for index := start; index < len(raw); index++ {
		if v113Escape(raw, index) {
			index++
			continue
		}
		char := raw[index]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
		} else if char == '\'' || char == '"' {
			quote = char
		} else if char == '>' {
			return index
		}
	}
	return -1
}

// v113Escape распознаёт пары \\, \<, \> без изменения исходного текста.
func v113Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) && strings.ContainsRune("\\<>", raw[index+1])
}

// v113Pair сопоставляет закрытия с последними открытиями того же имени.
// Маркеры без пары сохраняют pair=-1 и не участвуют в проверке порядка.
func v113Pair(markers []v113Marker) {
	var openings [3][]int
	for index, marker := range markers {
		stack := openings[marker.name]
		if !marker.closing {
			openings[marker.name] = append(stack, index)
		} else if len(stack) > 0 {
			opening := stack[len(stack)-1]
			markers[opening].pair, markers[index].pair = index, opening
			openings[marker.name] = stack[:len(stack)-1]
		}
	}
}

// v113Order обходит только парные маркеры общим стеком. Ошибочное закрытие
// удаляет только своё открытие; остальные остаются для следующих закрытий.
func v113Order(number int, markers []v113Marker, findings diagnostics.Findings) error {
	var stack []int
	for index, marker := range markers {
		if marker.pair < 0 {
			continue
		}
		if !marker.closing {
			stack = append(stack, index)
			continue
		}
		for position := len(stack) - 1; position >= 0; position-- {
			if stack[position] != marker.pair {
				continue
			}
			if position != len(stack)-1 {
				occurrence, err := findings.Add(number, marker.start+1, number, marker.end+1, "")
				if err != nil {
					return err
				}
				for _, opening := range stack[position+1:] {
					related := markers[opening]
					if err := findings.AddRelated(occurrence, number, related.start+1, number, related.end+1); err != nil {
						return err
					}
				}
			}
			stack = append(stack[:position], stack[position+1:]...)
			break
		}
	}
	return nil
}

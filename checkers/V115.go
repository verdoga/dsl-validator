package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV115 регистрирует неподдерживаемые HTML-теги только для DSL 1.2.
func registrationV115(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V115",
		Severity: diagnostics.SeverityError,
		Message:  "Использован неподдерживаемый HTML-тег.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV115,
	})
}

// checkV115 проверяет исходные строки без декодирования и переноса состояния
// между строками. Диапазоны охватывают маркер целиком в Unicode-колонках
// с единицы; правая граница исключена.
func checkV115(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v115TextRow(lines, line) {
			continue
		}
		for _, marker := range v115Unsupported([]rune(line.Raw())) {
			if _, err := findings.Add(number, marker[0]+1, number, marker[1]+1, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v115Unsupported возвращает диапазоны неподдерживаемых маркеров в исходных
// рунах с нуля, правые границы исключены. Завершённый маркер потребляется целиком.
// Поиск занимает O(n) времени и памяти, включая незавершённые кандидаты.
func v115Unsupported(raw []rune) [][2]int {
	var markers [][2]int
	ends := v115MarkerEnds(raw)
	for start := 0; start < len(raw); {
		if v115Escape(raw, start) {
			start += 2
			continue
		}
		if raw[start] != '<' {
			start++
			continue
		}
		name, end := v115Marker(raw, start, ends)
		if end == 0 {
			start++
			continue
		}
		switch strings.ToLower(name) {
		case "strong", "em", "u", "br":
		default:
			markers = append(markers, [2]int{start, end})
		}
		start = end
	}
	return markers
}

// v115TextRow возвращает true для непосредственного текста блока text,
// включая неразобранный текст; false исключает иной контекст и DSL-объявления.
func v115TextRow(lines diagnostics.Lines, line diagnostics.Line) bool {
	switch line.Type() {
	case "tag", "heading", "block-start", "block-end":
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return false
		}
	}
	// Неразобранное объявление DSL также не становится текстом блока.
	initial := strings.TrimLeft(line.Raw(), " \t")
	if strings.HasPrefix(initial, "@") || strings.HasPrefix(initial, "#") {
		return false
	}
	parent, found := line.ParentLine()
	if !found {
		return false
	}
	opening, _ := lines.Line(parent)
	if opening.Type() != "block-start" {
		return false
	}
	for index := 0; index < opening.ElementCount(); index++ {
		element, _ := opening.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "text"
		}
	}
	return false
}

// v115Marker возвращает полное имя и исключающий индекс конца в рунах.
// ends получен из v115MarkerEnds для той же строки; нулевой конец означает,
// что кандидат не является завершённым маркером.
func v115Marker(raw []rune, start int, ends []int) (name string, end int) {
	first := start + 1
	if first < len(raw) && raw[first] == '/' {
		first++
	}
	if first == len(raw) || !v115Letter(raw[first]) {
		return "", 0
	}
	index := first + 1
	for index < len(raw) && (v115Letter(raw[index]) ||
		raw[index] >= '0' && raw[index] <= '9' || raw[index] == '-' || raw[index] == ':') {
		index++
	}
	if index == len(raw) || !strings.ContainsRune(" \t/>", raw[index]) {
		return "", 0
	}
	if ends[index] == 0 {
		return "", 0
	}
	return string(raw[first:index]), ends[index]
}

// v115MarkerEnds вычисляет конец маркера для начала чтения на каждой руне
// вне кавычек. Значение — исключающий индекс либо 0 при отсутствии закрытия.
// Обратный проход хранит результат суффикса для трёх состояний кавычек,
// поэтому повторные кандидаты не перечитывают остаток строки. Пары экранирования
// пропускаются целиком; обратная косая черта кавычку не экранирует.
func v115MarkerEnds(raw []rune) []int {
	const (
		outside = iota
		single
		double
	)
	ends := make([]int, len(raw))
	var next, afterPair [3]int
	for index := len(raw) - 1; index >= 0; index-- {
		current := next
		if v115Escape(raw, index) {
			current = afterPair
		} else {
			switch raw[index] {
			case '\'':
				current[outside], current[single] = next[single], next[outside]
			case '"':
				current[outside], current[double] = next[double], next[outside]
			case '>':
				current[outside] = index + 1
			}
		}
		ends[index] = current[outside]
		afterPair, next = next, current
	}
	return ends
}

// v115Letter возвращает true для латинской буквы ASCII имени HTML-тега.
func v115Letter(char rune) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

// v115Escape возвращает true для пары с литеральной обратной косой чертой
// либо угловой скобкой; false оставляет символ для обычного чтения.
func v115Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) &&
		strings.ContainsRune("\\<>", raw[index+1])
}

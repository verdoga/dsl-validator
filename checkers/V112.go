package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV112 регистрирует проверку парности выделений только для DSL 1.2.
func registrationV112(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V112", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Тег выделения не имеет пары в своей строке.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV112,
	})
}

// v112Marker хранит маркер и его исходный полуоткрытый диапазон в рунах с нуля.
type v112Marker struct {
	name                string
	start, end          int
	closing, selfClosed bool
	unpaired            bool
}

// checkV112 обрабатывает Raw каждой непосредственной текстовой строки целиком,
// независимо от границ content/unparsed. Состояние между строками не переносится.
func checkV112(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v112TextRow(lines, line) {
			continue
		}
		for _, marker := range v112Unpaired(line.Raw()) {
			if _, err := findings.Add(number, marker.start+1, number, marker.end+1,
				"Открытие и закрытие выделения должны находиться в одной текстовой строке"); err != nil {
				return err
			}
		}
	}
	return nil
}

// v112TextRow исключает названия, границы и объявления вложенных конструкций.
func v112TextRow(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() != "content" && line.Type() != "invalid" || v112Tag(line) != "" {
		return false
	}
	raw := strings.TrimLeft(line.Raw(), " \t")
	if raw == "" || raw[0] == '@' || raw[0] == '#' {
		return false
	}
	parent, found := line.ParentLine()
	if !found {
		return false
	}
	block, _ := lines.Line(parent)
	if block.Type() != "block-start" || !strings.EqualFold(v112Tag(block), "text") {
		return false
	}
	for {
		parent, found = block.ParentLine()
		if !found {
			return true
		}
		block, _ = lines.Line(parent)
		if block.Type() == "block-start" && (strings.EqualFold(v112Tag(block), "editor") || strings.EqualFold(v112Tag(block), "multifill")) {
			return false
		}
	}
}

// v112Tag возвращает только распознанное имя DSL-тега.
func v112Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v112Unpaired сопоставляет имена независимо и возвращает нарушения по исходным
// позициям. Индексы открытий в стеках позволяют сохранить порядок без сортировки.
func v112Unpaired(source string) []v112Marker {
	raw := []rune(source)
	var markers []v112Marker
	var stacks [3][]int
	for index := 0; index < len(raw); {
		if v112Escape(raw, index) {
			index += 2
			continue
		}
		if raw[index] != '<' {
			index++
			continue
		}
		marker, complete := v112ReadMarker(raw, index)
		if marker.end <= index {
			index++
			continue
		}
		index = marker.end
		if !complete {
			continue
		}
		kind := -1
		switch marker.name {
		case "strong":
			kind = 0
		case "em":
			kind = 1
		case "u":
			kind = 2
		}
		if kind < 0 {
			continue
		}
		marker.unpaired = true
		if !marker.selfClosed {
			stack := stacks[kind]
			if !marker.closing {
				stacks[kind] = append(stack, len(markers))
			} else if len(stack) > 0 {
				markers[stack[len(stack)-1]].unpaired = false
				stacks[kind] = stack[:len(stack)-1]
				marker.unpaired = false
			}
		}
		markers = append(markers, marker)
	}
	result := markers[:0]
	for _, marker := range markers {
		if marker.unpaired {
			result = append(result, marker)
		}
	}
	return result
}

// v112ReadMarker читает завершённый маркер, включая неизвестные имена, чтобы
// не распознавать теги внутри атрибутов повторно. Новый < вне кавычек не
// прерывает маркер (уточнение B). false означает отсутствие целого маркера;
// end при незакрытых кавычках или отсутствии > указывает на конец строки.
func v112ReadMarker(raw []rune, start int) (marker v112Marker, complete bool) {
	marker.start = start
	index := start + 1
	if index < len(raw) && raw[index] == '/' {
		marker.closing = true
		index++
	}
	nameStart := index
	for index < len(raw) && (raw[index] >= 'a' && raw[index] <= 'z' || raw[index] >= 'A' && raw[index] <= 'Z' ||
		index > nameStart && (raw[index] >= '0' && raw[index] <= '9' || raw[index] == '-' || raw[index] == ':')) {
		index++
	}
	// Экранированный знак сразу после имени не завершает имя HTML-маркера.
	// Например, <em\> остаётся текстом и не скрывает следующий </em>.
	if index == nameStart || index < len(raw) && v112Escape(raw, index) {
		return marker, false
	}
	if index == len(raw) || unicode.Is(unicode.White_Space, raw[index]) || raw[index] == '/' || raw[index] == '>' {
		marker.name = strings.ToLower(string(raw[nameStart:index]))
	}
	// Даже при неподдерживаемом продолжении имени читаем маркер целиком:
	// его атрибуты не должны становиться самостоятельными выделениями.
	var quote rune
	for ; index < len(raw); index++ {
		if v112Escape(raw, index) {
			index++
			continue
		}
		char := raw[index]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '>' {
			last := index - 1
			for last >= nameStart && (raw[last] == ' ' || raw[last] == '\t') {
				last--
			}
			marker.selfClosed = last >= nameStart && raw[last] == '/'
			marker.end = index + 1
			return marker, true
		}
	}
	marker.end = len(raw)
	return marker, false
}

// v112Escape распознаёт пары \\, \< и \> в исходном содержимом text.
// Кавычки атрибутов не получают правил экранирования SOURCE медиатега.
func v112Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) && (raw[index+1] == '\\' || raw[index+1] == '<' || raw[index+1] == '>')
}

package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV114 регистрирует запрет атрибутов strong, em и u для DSL 1.2.
func registrationV114(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V114",
		Severity: diagnostics.SeverityError,
		Message:  "В теге выделения записаны атрибуты.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV114,
	})
}

// checkV114 отмечает исходный участок атрибутов каждого выбранного маркера.
// Координаты заданы в Unicode-колонках с 1, правая граница исключается.
func checkV114(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v114TextContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		for index := 0; index < len(raw); {
			if v114Escape(raw, index) {
				index += 2
				continue
			}
			if raw[index] != '<' {
				index++
				continue
			}
			end, closed := v114MarkerEnd(raw, index+1)
			if !closed {
				break
			}
			start := v114NameEnd(raw, index+1, end-1)
			if start != 0 {
				last := end - 1 // Сам > не входит в участок атрибутов.
				if last > start && raw[last-1] == '/' {
					last--
				}
				for start < last && unicode.Is(unicode.White_Space, raw[start]) {
					start++
				}
				for last > start && unicode.Is(unicode.White_Space, raw[last-1]) {
					last--
				}
				if start < last {
					if _, err := findings.Add(number, start+1, number, last+1, ""); err != nil {
						return err
					}
				}
			}
			index = end
		}
	}
	return nil
}

// v114TextContent выбирает собственный текст text, включая текстовые unparsed.
// Объявления вложенных конструкций исключаются даже при неполном разборе.
func v114TextContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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

// v114NameEnd возвращает индекс после полного имени strong, em или u.
// Ноль означает другое имя. start следует за <, limit указывает на завершающий >.
func v114NameEnd(raw []rune, start, limit int) int {
	if start < limit && raw[start] == '/' {
		start++
	}
	end := start
	for end < limit && raw[end] != '/' && !unicode.Is(unicode.White_Space, raw[end]) {
		end++
	}
	name := string(raw[start:end])
	if strings.EqualFold(name, "strong") || strings.EqualFold(name, "em") || strings.EqualFold(name, "u") {
		return end
	}
	return 0
}

// v114MarkerEnd выделяет маркер целиком, не завершая его на > внутри кавычек.
// end исключающий; closed=false означает незавершённый маркер до конца строки.
func v114MarkerEnd(raw []rune, start int) (end int, closed bool) {
	var quote rune
	for index := start; index < len(raw); index++ {
		if v114Escape(raw, index) {
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
			return index + 1, true
		}
	}
	return len(raw), false
}

// v114Escape распознаёт пары текстового контекста, сохраняя исходное написание.
// Кавычки здесь не экранируются по правилам SOURCE медиа.
func v114Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) && strings.ContainsRune("\\<>{}", raw[index+1])
}

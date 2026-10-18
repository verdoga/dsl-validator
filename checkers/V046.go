package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV046 регистрирует проверку разделителя перед открытием блока
// только для DSL 1.2. Конкретизация зависит от вида нарушения.
func registrationV046(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V046",
		Severity: diagnostics.SeverityError,
		Message:  "Перед открывающей фигурной скобкой отсутствует разделитель из обычных пробелов.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV046,
	})
}

// checkV046 обходит точный Raw, включая invalid и unparsed. Стек хранит признак
// литерального содержимого каждого найденного блока независимо от разбора
// модели. Диапазоны заданы в Unicode-колонках с 1, конец исключается.
func checkV046(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var literal []bool
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "}" {
			if len(literal) > 0 {
				literal = literal[:len(literal)-1]
			}
			continue
		}
		if len(literal) > 0 && literal[len(literal)-1] {
			continue
		}
		raw := []rune(line.Raw())
		name, end := v046Declaration(raw)
		if name == "" {
			continue
		}
		open := v046Opening(raw, end)
		if open < 0 {
			continue
		}
		literal = append(literal, name == "editor" || name == "multifill")
		start, ordinary := open, true
		for start > 0 && unicode.Is(unicode.White_Space, raw[start-1]) {
			start--
			ordinary = ordinary && raw[start] == ' '
		}
		if start < open && ordinary {
			continue
		}
		stop := open
		detail := "Разделитель содержит символы, отличные от U+0020."
		if start == open {
			stop++
			detail = "Перед скобкой нет разделителя."
		}
		if _, err := findings.Add(number, start+1, number, stop+1, detail); err != nil {
			return err
		}
	}
	return nil
}

// v046Declaration распознаёт ровно перечисленные в V046 имена после отступа
// U+0020/U+0009. Возвращает имя и индекс сразу после него либо пустое имя.
func v046Declaration(raw []rune) (string, int) {
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	end := start + 1
	for end < len(raw) && (raw[end] >= 'a' && raw[end] <= 'z' || raw[end] >= 'A' && raw[end] <= 'Z') {
		end++
	}
	if end == len(raw) || raw[end] != '{' && !unicode.Is(unicode.White_Space, raw[end]) {
		return "", 0
	}
	name := strings.ToLower(string(raw[start+1 : end]))
	switch name {
	case "editor", "instr", "hint", "answer", "example", "wordlist", "note",
		"alt", "table", "script", "text", "key", "choice", "multichoice",
		"matching", "ordering", "fragment", "variants", "multifill":
		return name, end
	default:
		return "", 0
	}
}

// v046Opening находит первую скобку блока после имени, не снимая экранирование.
// Скобки ответов после ровно пяти неэкранированных подчёркиваний пропускаются;
// ответ заканчивается первой неэкранированной } или границей строки.
func v046Opening(raw []rune, start int) int {
	for i := start; i < len(raw); {
		if v046Escape(raw, i) {
			i += 2
			continue
		}
		if raw[i] == '\\' && i+1 < len(raw) && raw[i+1] == '_' {
			// Экранируется вся последовательность подчёркиваний, не следующая {.
			i += 2
			for i < len(raw) && raw[i] == '_' {
				i++
			}
			continue
		}
		if raw[i] == '_' {
			end := i + 1
			for end < len(raw) && raw[end] == '_' {
				end++
			}
			if end-i == 5 && end < len(raw) && raw[end] == '{' {
				i = end + 1
				for i < len(raw) {
					if v046Escape(raw, i) {
						i += 2
					} else if raw[i] == '}' {
						i++
						break
					} else {
						i++
					}
				}
			} else {
				i = end
			}
			continue
		}
		if raw[i] == '{' {
			return i
		}
		i++
	}
	return -1
}

// v046Escape определяет пару \\, \{ или \}; первая косая черта уже неэкранирована.
func v046Escape(raw []rune, index int) bool {
	return raw[index] == '\\' && index+1 < len(raw) &&
		(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}')
}

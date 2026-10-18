package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV048 регистрирует проверку остатка после открытия блока для DSL 1.2.
func registrationV048(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V048",
		Severity: diagnostics.SeverityError,
		Message:  "После открывающей фигурной скобки блока есть содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV048,
	})
}

// checkV048 восстанавливает контексты только по Raw, не проверяя парность.
// Диапазоны задаются исходными Unicode-колонками с 1, правая граница исключается.
func checkV048(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var stack []string
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := line.Raw()
		if strings.Trim(raw, " \t") == "}" {
			if len(stack) != 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if len(stack) != 0 && (stack[len(stack)-1] == "editor" || stack[len(stack)-1] == "multifill") {
			continue
		}
		runes := []rune(raw)
		tag, opening := v048Opening(runes)
		if opening < 0 {
			continue
		}
		stack = append(stack, tag)
		end := len(runes)
		for end > opening+1 && (runes[end-1] == ' ' || runes[end-1] == '\t') {
			end--
		}
		if end == opening+1 {
			continue
		}
		start := opening + 1
		for runes[start] == ' ' || runes[start] == '\t' {
			start++
		}
		occurrence, err := findings.Add(number, opening+1, number, opening+2, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, number, start+1, number, end+1); err != nil {
			return err
		}
	}
	return nil
}

// v048Opening возвращает каноническое имя блочного тега и индекс первой
// неэкранированной скобки в рунах с 0; -1 означает отсутствие открытия.
// Unicode White_Space завершает имя, но начальные отступы — только U+0020/U+0009.
func v048Opening(raw []rune) (string, int) {
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", -1
	}
	end := start + 1
	for end < len(raw) && raw[end] != '{' && !unicode.IsSpace(raw[end]) {
		end++
	}
	tag := strings.ToLower(string(raw[start+1 : end]))
	switch tag {
	case "editor", "instr", "hint", "answer", "example", "wordlist", "note", "alt",
		"table", "script", "text", "key", "choice", "multichoice", "matching", "ordering",
		"fragment", "variants", "multifill":
	default:
		return "", -1
	}
	for index := end; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' {
				index++
				continue
			}
		}
		if raw[index] == '{' {
			return tag, index
		}
	}
	return "", -1
}

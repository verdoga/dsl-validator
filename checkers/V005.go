package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV005 регистрирует проверку парности фигурных скобок для DSL 1.2.
func registrationV005(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V005",
		Severity: diagnostics.SeverityError,
		Message:  "Нарушено оформление фигурных скобок.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV005,
	})
}

// v005Opening хранит исходную строку и Unicode-колонку открывающей скобки с 1.
type v005Opening struct {
	line, column int
}

// checkV005 строит собственный стек исключительно по Raw физических строк.
// Парность не зависит от тегов, подчёркиваний и границ строк. Контекст до
// заголовка нужен только для буквальных путей корневого resource-dir
// (DSL 1.31.2–1.31.4, 1.35.8); внутри открытых скобок это исключение не действует.
func checkV005(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var stack []v005Opening
	beforeHeading := true
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := line.Raw()
		if beforeHeading && len(stack) == 0 {
			boundary := strings.Trim(raw, " \t")
			if strings.HasPrefix(boundary, "# ") {
				beforeHeading = false
			} else if tag, _, found := strings.Cut(boundary, " "); found && strings.EqualFold(tag, "@resource-dir") {
				continue
			}
		}
		runes := []rune(raw)
		for index := 0; index < len(runes); index++ {
			if runes[index] == '\\' && index+1 < len(runes) {
				next := runes[index+1]
				if next == '\\' || next == '{' || next == '}' {
					index++
					continue
				}
			}
			switch runes[index] {
			case '{':
				stack = append(stack, v005Opening{line: number, column: index + 1})
			case '}':
				if len(stack) != 0 {
					stack = stack[:len(stack)-1]
					continue
				}
				if _, err := findings.Add(number, index+1, number, index+2, "Закрывающая скобка не имеет открывающей"); err != nil {
					return err
				}
			}
		}
	}
	for _, opening := range stack {
		if _, err := findings.Add(opening.line, opening.column, opening.line, opening.column+1, "Открывающая скобка не имеет закрывающей"); err != nil {
			return err
		}
	}
	return nil
}

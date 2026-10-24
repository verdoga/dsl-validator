package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV100 регистрирует вложенные разделённые ячейки в DSL 1.2.
func registrationV100(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V100",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри разделённой ячейки находится неэкранированный знак <.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV100,
	})
}

// checkV100 проверяет весь Raw непосредственных строк content таблицы,
// включая участки unparsed. Вложенные конструкции не являются её строками.
func checkV100(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "content" || !v100TableRow(lines, line) {
			continue
		}
		trimmed := strings.Trim(line.Raw(), " \t")
		if trimmed == "" || trimmed == "---" {
			continue
		}
		if err := v100CheckRow(line, findings); err != nil {
			return err
		}
	}
	return nil
}

// v100TableRow возвращает true, если непосредственный родитель — блок table;
// false означает иной контекст либо отсутствие родителя.
func v100TableRow(lines diagnostics.Lines, line diagnostics.Line) bool {
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
			return name == "table"
		}
	}
	return false
}

// v100CheckRow сохраняет глубину ошибочных открытий до соответствующих
// закрытий. Состояние локально для физической строки. Диапазон каждого
// сообщения — один исходный символ в Unicode-колонках с 1, конец исключён.
func v100CheckRow(line diagnostics.Line, findings diagnostics.Findings) error {
	raw := []rune(line.Raw())
	depth, atStart := 0, true
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '|' || next == '<' || next == '>' {
				index++
				atStart = false
				continue
			}
		}
		if char == '|' && depth == 0 {
			atStart = true
			continue
		}
		if char == '<' {
			if depth > 0 {
				if _, err := findings.Add(line.Number(), index+1, line.Number(), index+2,
					"Вложенные разделённые ячейки не поддерживаются; буквальный знак записывается как \\<"); err != nil {
					return err
				}
				depth++
			} else if atStart {
				depth = 1
			}
		} else if char == '>' && depth > 0 {
			depth--
		}
		if char != ' ' && char != '\t' {
			atStart = false
		}
	}
	return nil
}

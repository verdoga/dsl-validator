package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV099 регистрирует проверку конечного > обычной ячейки для DSL 1.2.
func registrationV099(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V099",
		Severity: diagnostics.SeverityError,
		Message:  "В конце обычной ячейки знак > не экранирован.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV099,
	})
}

// checkV099 рассматривает собственные строки данных table, включая unparsed.
// Служебные строки и попытки объявления тегов или заголовков ячейками не являются.
func checkV099(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "content" && line.Type() != "invalid" {
			continue
		}
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		block, _ := lines.Line(parent)
		if !v099Table(block) {
			continue
		}
		start := strings.TrimLeft(line.Raw(), " \t")
		if strings.HasPrefix(start, "@") || strings.HasPrefix(start, "#") {
			continue
		}
		if err := v099Row(line, findings); err != nil {
			return err
		}
	}
	return nil
}

// v099Table сообщает, является ли строка открытием блока с каноническим тегом table.
func v099Table(line diagnostics.Line) bool {
	if line.Type() != "block-start" {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "table"
		}
	}
	return false
}

// v099Row выделяет внешние ячейки по Raw без изменения или декодирования текста.
// divided сохраняет исключение всей ячейки после закрытия её разделённой части.
// last хранит индекс последнего непробельного символа; диапазоны выдаются
// в Unicode-колонках с 1, с исключающей правой границей.
func v099Row(line diagnostics.Line, findings diagnostics.Findings) error {
	raw := []rune(line.Raw())
	last, escaped, divided, inside := -1, false, false, false
	for index := 0; index <= len(raw); index++ {
		if index == len(raw) || raw[index] == '|' && !inside {
			if !divided && last >= 0 && raw[last] == '>' && !escaped {
				if _, err := findings.Add(line.Number(), last+1, line.Number(), last+2,
					`Для буквального знака используйте \>`); err != nil {
					return err
				}
			}
			last, escaped, divided, inside = -1, false, false, false
			continue
		}
		char := raw[index]
		if char == '\\' && index+1 < len(raw) && strings.ContainsRune("\\|<>", raw[index+1]) {
			index++
			last, escaped = index, true
			continue
		}
		if char == ' ' || char == '\t' {
			continue
		}
		if last == -1 && char == '<' {
			divided, inside = true, true
		} else if inside && char == '>' {
			inside = false
		}
		last, escaped = index, false
	}
	return nil
}

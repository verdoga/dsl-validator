package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV150 регистрирует незакрытый ответ в строке multifill для DSL 1.2.
func registrationV150(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V150", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Ответ в фигурных скобках не закрыт в строке пропуска.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV150,
	})
}

// checkV150 проверяет исходные строки предзаполненного multifill независимо.
// Отмечается только первоначальная открывающая скобка незавершённого ответа.
func checkV150(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v150Prefilled(lines, line) {
			continue
		}
		if column := v150Unclosed(line.Raw()); column != 0 {
			if _, err := findings.Add(line.Number(), column, line.Number(), column+1, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v150Prefilled возвращает true только для содержимого ближайшего блока multifill.
// Объявления и закрытия блоков не входят в предзаполненный текст; false исключает строку.
func v150Prefilled(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() == "block-start" || line.Type() == "block-end" {
		return false
	}
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "tag" {
				name, _ := element.Value()
				return strings.EqualFold(name, "multifill")
			}
		}
		return false
	}
	return false
}

// v150Unclosed возвращает Unicode-колонку незакрытой { с 1 либо 0 при её отсутствии.
// Состояние ограничено одной физической строкой; декодирование Raw не выполняется.
func v150Unclosed(raw string) int {
	runes := []rune(raw)
	for i := 0; i < len(runes); {
		if runes[i] == '\\' && i+1 < len(runes) {
			if runes[i+1] == '_' {
				i += 2
				for i < len(runes) && runes[i] == '_' {
					i++
				}
				continue
			}
			if strings.ContainsRune("\\{}", runes[i+1]) {
				i += 2
				continue
			}
		}
		if runes[i] != '_' {
			i++
			continue
		}
		start := i
		for i < len(runes) && runes[i] == '_' {
			i++
		}
		if i-start != 5 || i == len(runes) || runes[i] != '{' {
			continue
		}
		opening := i
		i++
		for i < len(runes) {
			if runes[i] == '\\' && i+1 < len(runes) && strings.ContainsRune("\\{}", runes[i+1]) {
				i += 2
				continue
			}
			if runes[i] == '}' {
				break
			}
			i++
		}
		if i == len(runes) {
			return opening + 1
		}
		i++
	}
	return 0
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV123 регистрирует редакторский запрет @hint в однострочном instr
// только для DSL 1.2. Маркер внутри строки не означает настоящее вложение тега.
func registrationV123(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V123",
		Severity: diagnostics.SeverityError,
		Message:  "В однострочной инструкции записан маркер @hint.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV123,
	})
}

// checkV123 ищет маркеры только в исходном content однострочных объявлений.
// Координаты заданы в кодовых точках Unicode с 1; правая граница исключается.
// Содержимое не нормализуется, поиск не продолжается на следующей строке.
func checkV123(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v123InlineInstr(line) {
			continue
		}
		raw := []rune(line.Raw())
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "content" {
				continue
			}
			text := []rune(element.Raw())
			for offset := 0; offset+5 <= len(text); offset++ {
				if text[offset] != '@' {
					continue
				}
				if !strings.EqualFold(string(text[offset:offset+5]), "@hint") {
					continue
				}
				start := element.Start() - 1 + offset
				end := start + 5
				// Перед CONTENT есть разделитель тега. Граница следующего
				// элемента (например, после _____) не начинает CONTENT заново.
				// Проверка исходного символа исключает также предшествующую \\.
				if start > 0 && !v123Space(raw[start-1]) {
					continue
				}
				if end < len(raw) && !v123Space(raw[end]) && raw[end] != '{' && raw[end] != '}' {
					continue
				}
				detail := "Для подсказки, вложенной в инструкцию, используйте блочный @instr и отдельное объявление @hint внутри него"
				if _, err := findings.Add(number, start+1, number, end+1, detail); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v123InlineInstr возвращает true для распознанного однострочного instr.
// Литеральный текст и объявления с открытием блока дают false независимо от глубины.
func v123InlineInstr(line diagnostics.Line) bool {
	if line.Type() != "tag" {
		return false
	}
	instr := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			return false
		}
		if element.Type() == "tag" && strings.EqualFold(element.Raw(), "@instr") {
			instr = true
		}
	}
	return instr
}

// v123Space возвращает true только для разделителей U+0020 и U+0009.
func v123Space(char rune) bool {
	return char == ' ' || char == '\t'
}

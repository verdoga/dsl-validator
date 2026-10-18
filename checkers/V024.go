package checkers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV024 регистрирует предупреждение о диакритике без буквы-основы
// только для DSL 1.2.
func registrationV024(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V024",
		Severity: diagnostics.SeverityWarning,
		Message:  "Диакритический знак не имеет предшествующей буквы-основы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV024,
	})
}

// checkV024 проверяет точный Raw всех строк без разбора разметки и нормализации.
// Любая L устанавливает основу, M сохраняет её, остальные символы сбрасывают.
// Диапазоны групп без основы заданы в Unicode-колонках с 1, конец исключается.
func checkV024(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		text := []rune(line.Raw())
		base := false
		for i := 0; i < len(text); {
			char := text[i]
			if !base && v024Diacritic(char) {
				start := i
				for i < len(text) && v024Diacritic(text[i]) {
					i++
				}
				if _, err := findings.Add(number, start+1, number, i+1, v024Detail(text[start:i])); err != nil {
					return err
				}
				continue
			}
			if unicode.IsLetter(char) {
				base = true
			} else if !unicode.IsMark(char) {
				base = false
			}
			i++
		}
	}
	return nil
}

// v024Diacritic возвращает true только для пересечения M и Diacritic;
// самостоятельные модификаторы и остальные символы дают false.
func v024Diacritic(char rune) bool {
	return unicode.IsMark(char) && unicode.Is(unicode.Diacritic, char)
}

// v024Detail перечисляет коды группы в исходном порядке, сохраняя повторы.
func v024Detail(chars []rune) string {
	codes := make([]string, len(chars))
	for i, char := range chars {
		codes[i] = fmt.Sprintf("U+%04X", char)
	}
	return "Коды знаков: " + strings.Join(codes, ", ") + "."
}

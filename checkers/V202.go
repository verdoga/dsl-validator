package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV202 регистрирует рекомендацию по регистру заголовка в DSL 1.2.
func registrationV202(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V202", Severity: diagnostics.SeverityRecommendation, Scope: diagnostics.ScopeElement,
		Message:  "Регистр букв заголовка второго уровня отличается от оформления с прописной первой буквой и строчными остальными.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV202,
	})
}

// checkV202 проверяет семантическое название, сохраняя исходный диапазон title.
// Наличие строкового Value у title гарантировано контрактом входной модели.
func checkV202(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "heading" {
			continue
		}
		secondLevel := false
		var title diagnostics.Element
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "heading-level" && element.Raw() == "##" {
				secondLevel = true
			}
			if element.Type() == "title" && title == nil {
				title = element
			}
		}
		if !secondLevel || title == nil {
			continue
		}
		value, _ := title.Value()
		if !v202WrongCase(value) {
			continue
		}
		if _, err := findings.Add(line.Number(), title.Start(), line.Number(), title.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v202WrongCase возвращает true при нарушении регистра хотя бы одной буквы.
// false означает правильный регистр либо отсутствие регистровых букв.
// Пробелы, знаки и буквы без регистра не начинают последовательность заново.
func v202WrongCase(value string) bool {
	first := true
	for _, letter := range value {
		if !unicode.IsLetter(letter) {
			continue
		}
		upper := unicode.IsUpper(letter) || unicode.Is(unicode.Other_Uppercase, letter)
		lower := unicode.IsLower(letter) || unicode.Is(unicode.Other_Lowercase, letter)
		if !upper && !lower && !unicode.IsTitle(letter) {
			continue
		}
		if (first && !upper) || (!first && !lower) {
			return true
		}
		first = false
	}
	return false
}

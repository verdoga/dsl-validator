package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV205 регистрирует рекомендацию по регистру названий блоков в DSL 1.2.
func registrationV205(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V205", Severity: diagnostics.SeverityRecommendation, Scope: diagnostics.ScopeElement,
		Message:  "Регистр заголовка отличается от оформления с прописной первой буквой и строчными остальными.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV205,
	})
}

// checkV205 проверяет только title выбранных объявлений block-start.
// Глубина и содержимое блока не влияют на результат. Строковое Value у title
// гарантировано контрактом модели; диапазон берётся из исходного элемента.
func checkV205(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		eligible := false
		var title diagnostics.Element
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "tag":
				name, _ := element.Value()
				switch strings.ToLower(name) {
				case "table", "alt", "script", "text", "key", "example", "wordlist":
					eligible = true
				}
			case "title":
				if title == nil {
					title = element
				}
			}
		}
		if !eligible || title == nil {
			continue
		}
		value, _ := title.Value()
		if !v205WrongCase(value) {
			continue
		}
		if _, err := findings.Add(line.Number(), title.Start(), line.Number(), title.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v205WrongCase возвращает true при нарушении регистра хотя бы одной буквы.
// false означает правильный регистр либо отсутствие регистровых букв.
// Знаки, включая двоеточие, пробелы и буквы без регистра не сбрасывают проверку.
func v205WrongCase(value string) bool {
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

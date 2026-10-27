package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV203 регистрирует рекомендацию по регистру названия в DSL 1.2.
func registrationV203(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V203", Severity: diagnostics.SeverityRecommendation, Scope: diagnostics.ScopeElement,
		Message:  "Регистр заголовка отличается от схемы «Тема» или «РАЗДЕЛ: Тема».",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV203,
	})
}

// checkV203 проверяет title распознанных ### и header; содержимое под ними не учитывается.
// Строковое Value у title гарантировано контрактом входной модели.
func checkV203(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		kind := line.Type()
		if kind != "heading" && kind != "tag" {
			continue
		}
		eligible := false
		var title diagnostics.Element
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "heading-level":
				if kind == "heading" && element.Raw() == "###" {
					eligible = true
				}
			case "tag":
				name, _ := element.Value()
				if kind == "tag" && strings.EqualFold(name, "header") {
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
		left, right, divided := strings.Cut(value, ":")
		wrong := v203WrongCase(left, divided)
		if divided {
			wrong = wrong || v203WrongCase(right, false)
		}
		if !wrong {
			continue
		}
		if _, err := findings.Add(line.Number(), title.Start(), line.Number(), title.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v203WrongCase возвращает true при нарушении регистра хотя бы одной буквы.
// allUpper=true требует все прописные, false — первую прописную и далее строчные.
// Часть без регистровых букв корректна; знаки и пробелы не сбрасывают проверку.
func v203WrongCase(text string, allUpper bool) bool {
	first := true
	for _, letter := range text {
		if !unicode.IsLetter(letter) {
			continue
		}
		upper := unicode.IsUpper(letter) || unicode.Is(unicode.Other_Uppercase, letter)
		lower := unicode.IsLower(letter) || unicode.Is(unicode.Other_Lowercase, letter)
		if !upper && !lower && !unicode.IsTitle(letter) {
			continue
		}
		if first || allUpper {
			if !upper {
				return true
			}
		} else if !lower {
			return true
		}
		first = false
	}
	return false
}

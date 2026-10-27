package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV201 регистрирует проверку регистра заголовков только для DSL 1.2.
func registrationV201(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V201", Severity: diagnostics.SeverityRecommendation, Scope: diagnostics.ScopeElement,
		Message:  "Не все буквы заголовка первого уровня прописные.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV201,
	})
}

// checkV201 проверяет семантический текст без преобразований и отмечает
// весь исходный title. Вложенность и содержимое под заголовком не учитываются.
func checkV201(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "heading" {
			continue
		}
		firstLevel := false
		var title diagnostics.Element
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "heading-level":
				firstLevel = firstLevel || element.Raw() == "#"
			case "title":
				title = element
			}
		}
		if !firstLevel || title == nil {
			continue
		}
		text, present := title.Value()
		if !present || !v201HasNonUppercaseLetter(text) {
			continue
		}
		if _, err := findings.Add(line.Number(), title.Start(), line.Number(), title.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v201HasNonUppercaseLetter сообщает, есть ли буква с регистром,
// которая не является прописной. false включает отсутствие таких букв.
func v201HasNonUppercaseLetter(text string) bool {
	for _, letter := range text {
		if !unicode.IsLetter(letter) || unicode.IsUpper(letter) || unicode.Is(unicode.Other_Uppercase, letter) {
			continue
		}
		if unicode.IsLower(letter) || unicode.IsTitle(letter) || unicode.Is(unicode.Other_Lowercase, letter) {
			return true
		}
	}
	return false
}

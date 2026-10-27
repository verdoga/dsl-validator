package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV204 регистрирует проверку регистра заголовков @note только для DSL 1.2.
func registrationV204(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V204", Severity: diagnostics.SeverityRecommendation, Scope: diagnostics.ScopeElement,
		Message:  "Регистр заголовка @note отличается от схемы «Тема» или «РАЗДЕЛ: Тема».",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV204,
	})
}

// checkV204 проверяет только семантический заголовок объявления блока.
// Вложенность, содержимое блока и другие объявления на результат не влияют.
func checkV204(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		note := false
		var title diagnostics.Element
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "tag":
				name, present := element.Value()
				note = note || present && strings.EqualFold(name, "note")
			case "title":
				title = element
			}
		}
		if !note || title == nil {
			continue
		}
		text, present := title.Value()
		if !present {
			continue
		}
		left, right, colon := strings.Cut(text, ":")
		valid := v204MatchesCase(left, !colon)
		if colon {
			valid = valid && v204MatchesCase(right, true)
		}
		if valid {
			continue
		}
		if _, err := findings.Add(line.Number(), title.Start(), line.Number(), title.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v204MatchesCase проверяет часть без преобразований: sentence=true требует
// первую прописную и остальные строчные буквы, false — только прописные.
// Возвращает true при соответствии схеме или отсутствии букв с регистром.
func v204MatchesCase(text string, sentence bool) bool {
	needUpper := true
	for _, letter := range text {
		if !unicode.IsLetter(letter) {
			continue
		}
		upper := unicode.IsUpper(letter) || unicode.Is(unicode.Other_Uppercase, letter)
		lower := unicode.IsLower(letter) || unicode.Is(unicode.Other_Lowercase, letter)
		if !upper && !lower && !unicode.IsTitle(letter) {
			continue
		}
		if needUpper && !upper || !needUpper && !lower {
			return false
		}
		if sentence {
			needUpper = false
		}
	}
	return true
}

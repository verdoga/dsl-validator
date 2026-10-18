package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV014 регистрирует рекомендацию о тире рядом с буквами
// только для DSL 1.2.
func registrationV014(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V014",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Тире находится между буквенными частями без пробелов.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV014,
	})
}

// checkV014 проверяет U+2013 и U+2014 в точном Raw всех строк, кроме separator.
// Тире между буквенными частями отмечается, если хотя бы одна содержит больше
// одной буквы; сочетание буквенной части и пробельного символа отмечается всегда.
// HTML, пути, unparsed и фонетические записи не исключаются и не преобразуются.
// Диапазон содержит один знак; Unicode-колонки начинаются с 1, конец исключается.
func checkV014(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "separator" {
			continue
		}
		text := []rune(line.Raw())
		for i := 1; i+1 < len(text); i++ {
			if text[i] != '\u2013' && text[i] != '\u2014' {
				continue
			}
			left, right := v014LeftLetters(text, i), v014RightLetters(text, i)
			betweenLetters := left > 0 && right > 0 && (left > 1 || right > 1)
			besideSpace := left > 0 && unicode.IsSpace(text[i+1]) || right > 0 && unicode.IsSpace(text[i-1])
			if !betweenLetters && !besideSpace {
				continue
			}
			if _, err := findings.Add(number, i+1, number, i+2,
				"Если это составное слово, используйте дефис; если граница частей фразы — проверьте пробелы вокруг тире"); err != nil {
				return err
			}
		}
	}
	return nil
}

// v014LeftLetters считает буквы непосредственно слева от тире с индексом dash.
// Присоединённые M пропускаются при подсчёте; одни M без буквы дают 0.
func v014LeftLetters(text []rune, dash int) int {
	letters := 0
	for i := dash - 1; i >= 0; i-- {
		if unicode.IsLetter(text[i]) {
			letters++
		} else if !unicode.IsMark(text[i]) {
			break
		}
	}
	return letters
}

// v014RightLetters считает буквы непосредственно справа от тире с индексом dash.
// Часть обязана начинаться с L; последующие M не увеличивают число букв.
func v014RightLetters(text []rune, dash int) int {
	if dash+1 >= len(text) || !unicode.IsLetter(text[dash+1]) {
		return 0
	}
	letters := 0
	for i := dash + 1; i < len(text); i++ {
		if unicode.IsLetter(text[i]) {
			letters++
		} else if !unicode.IsMark(text[i]) {
			break
		}
	}
	return letters
}

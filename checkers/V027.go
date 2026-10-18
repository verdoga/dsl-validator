package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV027 регистрирует предупреждение об одинарных кавычках
// только для DSL 1.2.
func registrationV027(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V027",
		Severity: diagnostics.SeverityWarning,
		Message:  "Текстовый фрагмент выделен одинарными кавычками.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV027,
	})
}

// checkV027 сопоставляет кавычки в целых строках, сохраняя границы DSL.
// Разрешённое форматирование в text прозрачно для кавычек и апострофов.
// Основной и связанный диапазоны отмечают по одному исходному символу;
// Unicode-колонки начинаются с 1, правые границы исключаются.
func checkV027(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		raw, positions := visibleText(raw, textFormatting(lines, line, raw), 0, len(raw))
		excluded := v027Apostrophes(raw)
		opening := -1
		for i, char := range raw {
			if char != '\'' || excluded[i] {
				continue
			}
			if opening >= 0 {
				if i > opening+1 && !unicode.IsSpace(raw[i-1]) &&
					(i+1 == len(raw) || unicode.IsSpace(raw[i+1]) || unicode.IsPunct(raw[i+1])) {
					occurrence, err := findings.Add(number, positions[opening]+1, number, positions[opening]+2,
						"Для выделения фрагмента используйте двойные кавычки; внутрисловные апострофы сохраните")
					if err != nil {
						return err
					}
					if err := findings.AddRelated(occurrence, number, positions[i]+1, number, positions[i]+2); err != nil {
						return err
					}
					opening = -1
				}
				continue
			}
			if i+1 < len(raw) && !unicode.IsSpace(raw[i+1]) &&
				(i == 0 || unicode.IsSpace(raw[i-1]) || unicode.Is(unicode.Ps, raw[i-1]) ||
					unicode.Is(unicode.Pi, raw[i-1]) || raw[i-1] == '"') {
				opening = i
			}
		}
	}
	return nil
}

// v027Apostrophes отмечает исключённые знаки до поиска пар.
// true означает апостроф, который не участвует в сопоставлении кавычек.
func v027Apostrophes(raw []rune) []bool {
	excluded := make([]bool, len(raw))
	for i, char := range raw {
		if char != '\'' {
			continue
		}
		if v027LetterBefore(raw, i) && i+1 < len(raw) && unicode.IsLetter(raw[i+1]) {
			excluded[i] = true
			continue
		}
		if i > 0 && v027WordRune(raw[i-1]) {
			continue
		}
		end := i + 1
		for end < len(raw) && v027WordRune(raw[end]) {
			end++
		}
		word := string(raw[i+1 : end])
		for _, form := range []string{"tis", "twas", "twere", "twill", "twould", "em", "cause", "cos", "cuz", "bout", "til"} {
			if strings.EqualFold(word, form) {
				excluded[i] = true
				break
			}
		}
		if (len(word) == 2 || len(word) == 3 && (word[2] == 's' || word[2] == 'S')) &&
			word[0] >= '0' && word[0] <= '9' && word[1] >= '0' && word[1] <= '9' {
			excluded[i] = true
		}
		if end == i+2 && (raw[i+1] == 'n' || raw[i+1] == 'N') && end < len(raw) && raw[end] == '\'' {
			left, right := i, end+1
			for left > 0 && unicode.IsSpace(raw[left-1]) {
				left--
			}
			for right < len(raw) && unicode.IsSpace(raw[right]) {
				right++
			}
			if left < i && right > end+1 && v027LetterBefore(raw, left) && right < len(raw) && unicode.IsLetter(raw[right]) {
				excluded[i], excluded[end] = true, true
			}
		}
	}
	return excluded
}

// v027LetterBefore возвращает true, если слева от end находится буква
// с возможными присоединёнными диакритиками; иначе возвращает false.
func v027LetterBefore(raw []rune, end int) bool {
	for end > 0 && unicode.IsMark(raw[end-1]) {
		end--
	}
	return end > 0 && unicode.IsLetter(raw[end-1])
}

// v027WordRune возвращает true для букв, цифр и диакритик;
// остальные символы завершают форму, проверяемую на исключение.
func v027WordRune(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsDigit(char) || unicode.IsMark(char)
}

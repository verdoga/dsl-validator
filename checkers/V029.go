package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV029 регистрирует рекомендацию о пробелах вокруг короткого тире
// между цифровыми частями только для DSL 1.2.
func registrationV029(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V029",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Короткое тире между числами отделено пробелами.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV029,
	})
}

// v029Part хранит диапазон цифровой части [start,end) в индексах видимых рун.
// word означает примыкание буквы или комбинируемого знака к её границе.
type v029Part struct {
	start, end int
	word       bool
}

// checkV029 проверяет все строки без нормализации числовых частей.
// Разрешённое форматирование в text прозрачно для чисел и промежутков.
// Из максимальных цепочек через U+2013 рассматриваются только пары частей.
// Выделяется промежуток между ними в Unicode-колонках с 1, конец исключается.
func checkV029(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		text := []rune(line.Raw())
		text, positions := visibleText(text, textFormatting(lines, line, text), 0, len(text))
		parts := v029Parts(text)
		for first := 0; first < len(parts); {
			last := first
			for last+1 < len(parts) && v029Join(text[parts[last].end:parts[last+1].start]) {
				last++
			}
			if last == first+1 && !parts[first].word && !parts[last].word {
				start, end := parts[first].end, parts[last].start
				// Единственный знак тире без пробелов не требует рекомендации.
				if end-start > 1 {
					if _, err := findings.Add(number, positions[start]+1, number, positions[end-1]+2,
						"Для числового диапазона рекомендуется запись без пробелов: 1–12"); err != nil {
						return err
					}
				}
			}
			first = last + 1
		}
	}
	return nil
}

// v029Parts выделяет Nd с любым количеством точек и запятых между цифрами.
// Значения не вычисляются, формат дроби не проверяется. Один непосредственно
// предшествующий знак входит в часть. Буквенные границы помечаются для исключения.
func v029Parts(text []rune) []v029Part {
	var parts []v029Part
	for i := 0; i < len(text); {
		if !unicode.IsDigit(text[i]) {
			i++
			continue
		}
		start := i
		if start > 0 && v029Sign(text[start-1]) {
			start--
		}
		i++
		for i < len(text) {
			if unicode.IsDigit(text[i]) {
				i++
			} else if (text[i] == '.' || text[i] == ',') && i+1 < len(text) && unicode.IsDigit(text[i+1]) {
				i += 2
			} else {
				break
			}
		}
		word := start > 0 && (unicode.IsLetter(text[start-1]) || unicode.IsMark(text[start-1])) ||
			i < len(text) && (unicode.IsLetter(text[i]) || unicode.IsMark(text[i]))
		parts = append(parts, v029Part{start: start, end: i, word: word})
	}
	return parts
}

// v029Join возвращает true только для одного U+2013 с горизонтальными пробелами
// или без них. Иной символ либо отсутствие тире дают false.
func v029Join(gap []rune) bool {
	dash := false
	for _, char := range gap {
		if char == '\u2013' && !dash {
			dash = true
		} else if !v029Horizontal(char) {
			return false
		}
	}
	return dash
}

// v029Horizontal возвращает true для U+0009 и Zs, false для остальных символов.
func v029Horizontal(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}

// v029Sign возвращает true для трёх разрешённых знаков числа, иначе false.
func v029Sign(char rune) bool {
	return char == '+' || char == '-' || char == '\u2212'
}

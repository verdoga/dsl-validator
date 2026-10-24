package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v061Position хранит исходный диапазон числовой группы; конец исключается.
type v061Position struct {
	line, start, end int
}

// registrationV061 регистрирует повтор чисел в заголовках шагов DSL 1.2.
func registrationV061(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V061",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Число в заголовках шагов повторяется.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV061,
	})
}

// checkV061 сравнивает все группы Nd в исходных названиях шагов.
// Первое вхождение сохраняется на весь документ, включая переходы newpage.
// Строковый ключ исключает переполнение и не меняет исходные диапазоны.
func checkV061(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first := make(map[string]v061Position)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		title := v061Title(line)
		if title == nil {
			continue
		}
		raw := []rune(title.Raw())
		for start := 0; start < len(raw); {
			if _, digit := v061Digit(raw[start]); !digit {
				start++
				continue
			}
			end := start
			var digits []byte
			for end < len(raw) {
				digit, ok := v061Digit(raw[end])
				if !ok {
					break
				}
				digits = append(digits, digit)
				end++
			}
			key := strings.TrimLeft(string(digits), "0")
			if key == "" {
				key = "0"
			}
			current := v061Position{number, title.Start() + start, title.Start() + end}
			if previous, found := first[key]; found {
				occurrence, err := findings.Add(current.line, current.start, current.line, current.end, "")
				if err != nil {
					return err
				}
				if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
					return err
				}
			} else {
				first[key] = current
			}
			start = end
		}
	}
	return nil
}

// v061Title возвращает название распознанного корневого заголовка уровня 3.
// nil означает отсутствие такого заголовка; псевдозаголовки не разбираются.
func v061Title(line diagnostics.Line) diagnostics.Element {
	if line.Type() != "heading" {
		return nil
	}
	if _, nested := line.ParentLine(); nested {
		return nil
	}
	var title diagnostics.Element
	thirdLevel := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "heading-level":
			thirdLevel = element.Raw() == "###"
		case "title":
			title = element
		}
	}
	if !thirdLevel {
		return nil
	}
	return title
}

// v061Digit возвращает ASCII-цифру для Nd; false означает другую категорию.
// В таблицах Nd наборы 0..9 последовательны; один диапазон может объединять
// несколько таких наборов, поэтому порядковый номер берётся по модулю 10.
func v061Digit(char rune) (byte, bool) {
	if char <= 0xffff {
		for _, span := range unicode.Nd.R16 {
			if char >= rune(span.Lo) && char <= rune(span.Hi) {
				offset := char - rune(span.Lo)
				if offset%rune(span.Stride) == 0 {
					return '0' + byte(offset/rune(span.Stride)%10), true
				}
			}
		}
	} else {
		for _, span := range unicode.Nd.R32 {
			if char >= rune(span.Lo) && char <= rune(span.Hi) {
				offset := char - rune(span.Lo)
				if offset%rune(span.Stride) == 0 {
					return '0' + byte(offset/rune(span.Stride)%10), true
				}
			}
		}
	}
	return 0, false
}

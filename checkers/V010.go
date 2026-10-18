package checkers

import (
	"fmt"
	"slices"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV010 регистрирует предупреждение о необычных пробельных символах
// только для DSL 1.2.
func registrationV010(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V010",
		Severity: diagnostics.SeverityWarning,
		Message:  "Использован пробельный символ, отличный от обычного пробела.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV010,
	})
}

// checkV010 отмечает максимальные последовательности заданных пробельных
// символов в точном Raw всех строк. Колонки считаются в кодовых точках Unicode
// с 1; правая граница исключается. Конкретизация содержит уникальные коды
// последовательности по возрастанию и допустимый U+0020.
func checkV010(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		column, start := 1, 0
		var codes []rune
		for _, char := range line.Raw() {
			if v010Space(char) {
				if len(codes) == 0 {
					start = column
				}
				if !slices.Contains(codes, char) {
					codes = append(codes, char)
				}
			} else if len(codes) > 0 {
				if _, err := findings.Add(number, start, number, column, v010Detail(codes)); err != nil {
					return err
				}
				codes = codes[:0]
			}
			column++
		}
		if len(codes) > 0 {
			if _, err := findings.Add(number, start, number, column, v010Detail(codes)); err != nil {
				return err
			}
		}
	}
	return nil
}

// v010Space возвращает true только для символов из множества V010;
// обычный пробел, табуляция и все остальные символы дают false.
func v010Space(char rune) bool {
	switch char {
	case '\u000B', '\u000C', '\u0085', '\u00A0', '\u1680',
		'\u2028', '\u2029', '\u202F', '\u205F', '\u3000':
		return true
	default:
		return char >= '\u2000' && char <= '\u200A'
	}
}

// v010Detail сортирует переданный непустой набор уникальных кодов
// и формирует конкретизацию без нормативного сообщения.
func v010Detail(codes []rune) string {
	slices.Sort(codes)
	names := make([]string, len(codes))
	for i, code := range codes {
		names[i] = fmt.Sprintf("U+%04X", code)
	}
	return "Обнаружены: " + strings.Join(names, ", ") + ". Допустим U+0020."
}

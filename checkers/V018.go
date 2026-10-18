package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV018 регистрирует предупреждение о пробеле после пунктуации для DSL 1.2.
func registrationV018(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V018",
		Severity: diagnostics.SeverityWarning,
		Message:  "После знака препинания отсутствует пробел.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV018,
	})
}

// checkV018 отдельно проверяет ? . ! , : ; в исходных диапазонах content/title/name.
// Конец строки и горизонтальный пробел после знака допустимы; . , : между двумя
// десятичными цифрами исключаются. Диапазон охватывает один исходный символ
// в Unicode-колонках с 1, правая граница исключается. Конкретизация отсутствует.
func checkV018(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		runes := []rune(line.Raw())
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name":
			default:
				continue
			}
			for i := element.Start() - 1; i < element.End()-1; i++ {
				switch runes[i] {
				case '?', '.', '!', ',', ':', ';':
				default:
					continue
				}
				if i+1 == len(runes) || v018HorizontalSpace(runes[i+1]) {
					continue
				}
				if (runes[i] == '.' || runes[i] == ',' || runes[i] == ':') &&
					i > 0 && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
					continue
				}
				if _, err := findings.Add(number, i+1, number, i+2, ""); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v018HorizontalSpace возвращает true для TAB и Unicode-категории Zs;
// для остальных символов, включая вертикальные разделители, возвращает false.
func v018HorizontalSpace(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}

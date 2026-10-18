package checkers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV015 регистрирует проверку разрешённых символов для DSL 1.2.
func registrationV015(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V015",
		Severity: diagnostics.SeverityWarning,
		Message:  "В тексте присутствует символ вне разрешённого набора.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV015,
	})
}

// checkV015 проверяет исходные символы без преобразований и разбора разметки.
// Последовательности ограничены одним элементом или ответом placeholder;
// диапазоны задаются в исходных Unicode-колонках с исключающим концом.
func checkV015(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name", "unparsed", "placeholder":
			default:
				continue
			}
			raw := []rune(element.Raw())
			start, end := 0, element.End()-element.Start()
			if element.Type() == "placeholder" {
				// Границы уже распознанного элемента включают _____{ и }.
				// Внутри ответа ничего не снимается и не интерпретируется.
				if !strings.HasPrefix(element.Raw(), "_____{") || !strings.HasSuffix(element.Raw(), "}") {
					continue
				}
				start, end = 6, end-1
			}
			for offset := start; offset < end; {
				if v015Allowed(raw[offset]) {
					offset++
					continue
				}
				first := offset
				for offset < end && !v015Allowed(raw[offset]) {
					offset++
				}
				if _, err := findings.Add(number, element.Start()+first,
					number, element.Start()+offset, v015Detail(raw[first:offset])); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v015Allowed возвращает true для разрешённого символа либо символа,
// исключённого из V015 отдельными проверками; false означает нарушение.
// Классификация не зависит от соседей, регистра и типа текстового элемента.
func v015Allowed(char rune) bool {
	if unicode.IsSpace(char) || unicode.Is(unicode.Quotation_Mark, char) ||
		unicode.IsDigit(char) ||
		unicode.IsLetter(char) && unicode.In(char, unicode.Latin, unicode.Cyrillic) ||
		unicode.IsMark(char) && unicode.Is(unicode.Diacritic, char) {
		return true
	}
	if strings.ContainsRune(" .,;:!?…'\"-–—()[]{}/\\+=%°№§@#$€£₽βθχ‿‖|↘↗↑↓^&*~`<>\u00AD\u2010\u2011\u2012\u2015\uFE58\uFE63\uFF0D", char) {
		return true
	}
	// Разрешены только присвоенные фонетические символы из заданных
	// в описании интервалов, включая символы вне BMP.
	phonetic := char >= 0x0250 && char <= 0x02FF ||
		char >= 0x1D00 && char <= 0x1DBF ||
		char >= 0xA700 && char <= 0xA71F ||
		char >= 0xAB30 && char <= 0xAB6F ||
		char >= 0x10780 && char <= 0x107BF ||
		char >= 0x1DF00 && char <= 0x1DFFF
	return phonetic && unicode.IsGraphic(char)
}

// v015Detail перечисляет коды всех символов последовательности в исходном
// порядке, сохраняя повторы. Нормативное сообщение здесь не дублируется.
func v015Detail(chars []rune) string {
	codes := make([]string, len(chars))
	for index, char := range chars {
		codes[index] = fmt.Sprintf("U+%04X", char)
	}
	if len(codes) == 1 {
		return "Обнаружен " + codes[0]
	}
	return "Обнаружены " + strings.Join(codes[:len(codes)-1], ", ") + " и " + codes[len(codes)-1]
}

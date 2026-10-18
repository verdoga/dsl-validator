package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV022 регистрирует предупреждение о похожих на буквы цифрах
// внутри слова только для DSL 1.2.
func registrationV022(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V022",
		Severity: diagnostics.SeverityWarning,
		Message:  "Внутри буквенного слова находятся цифры, похожие на буквы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV022,
	})
}

// checkV022 проверяет Raw вне распознанных идентификаторов и путей.
// Разрешённое форматирование в text прозрачно. Экранирование не декодируется,
// unparsed остаётся проверяемым текстом.
// Диапазон охватывает весь токен в Unicode-колонках с 1; конец исключается.
func checkV022(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := ocrText(line)
		raw, positions := visibleText(raw, textFormatting(lines, line, raw), 0, len(raw))
		for i := 0; i < len(raw); {
			if !unicode.IsLetter(raw[i]) && !unicode.IsDigit(raw[i]) {
				i++
				continue
			}
			start := i
			for i < len(raw) && (unicode.IsLetter(raw[i]) || unicode.IsDigit(raw[i]) || unicode.IsMark(raw[i])) {
				i++
			}
			if detail := v022TokenDetail(raw[start:i]); detail != "" {
				if _, err := findings.Add(number, positions[start]+1, number, positions[i-1]+2, detail); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v022TokenDetail возвращает конкретизацию для подозрительного токена,
// либо пустую строку. Диакритики принадлежат токену, но не меняют тип основы.
// Прописные буквы сами по себе не исключают возможную OCR-подмену.
func v022TokenDetail(token []rune) string {
	letters, digits := 0, 0
	numericLetters := true
	bases := make([]rune, 0, len(token))
	for _, char := range token {
		if unicode.IsMark(char) {
			continue
		}
		bases = append(bases, char)
		if unicode.IsLetter(char) {
			letters++
			numericLetters = numericLetters && strings.ContainsRune("OoОоIlІіZzЗзSsбBВg", char)
		} else {
			digits++
		}
	}
	if letters < 2 || (digits >= 2 && digits > letters && numericLetters) {
		return ""
	}
	var details []string
	var seen [10]bool
	for i := 0; i < len(bases); {
		if !unicode.IsDigit(bases[i]) {
			i++
			continue
		}
		start, supported := i, true
		for i < len(bases) && unicode.IsDigit(bases[i]) {
			supported = supported && v022Correspondence(bases[i]) != ""
			i++
		}
		if start == 0 || i == len(bases) || !supported {
			continue
		}
		for _, digit := range bases[start:i] {
			if !seen[digit-'0'] {
				seen[digit-'0'] = true
				details = append(details, string(digit)+" → "+v022Correspondence(digit))
			}
		}
	}
	if len(details) == 0 {
		return ""
	}
	return "Подозрительные цифры: " + strings.Join(details, "; ") + "."
}

// v022Correspondence возвращает заданные соответствия без ограничения
// письменности. Пустая строка означает цифру без предусмотренного сходства.
func v022Correspondence(digit rune) string {
	switch digit {
	case '0':
		return "o/о"
	case '1':
		return "l/i/і"
	case '2':
		return "z"
	case '3':
		return "з"
	case '5':
		return "s"
	case '6':
		return "b/б"
	case '8':
		return "B/В"
	case '9':
		return "g"
	default:
		return ""
	}
}

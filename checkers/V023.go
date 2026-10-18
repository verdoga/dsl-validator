package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV023 регистрирует предупреждение о буквах в числовой записи для DSL 1.2.
func registrationV023(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V023",
		Severity: diagnostics.SeverityWarning,
		Message:  "В числовой последовательности находится буква, похожая на цифру.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV023,
	})
}

// checkV023 проверяет максимальные токены во всём Raw, включая unparsed.
// Конечные точки и запятые не входят в кандидат. Один диапазон охватывает
// исходный токен в Unicode-колонках с 1; правая граница исключается.
func checkV023(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		runes := []rune(line.Raw())
		for i := 0; i < len(runes); {
			if !v023TokenChar(runes[i]) {
				i++
				continue
			}
			start := i
			for i < len(runes) && v023TokenChar(runes[i]) {
				i++
			}
			end := i
			for end > start && (runes[end-1] == '.' || runes[end-1] == ',') {
				end--
			}
			detail := v023TokenDetail(runes[start:end])
			if detail == "" {
				continue
			}
			if _, err := findings.Add(number, start+1, number, end+1, detail); err != nil {
				return err
			}
		}
	}
	return nil
}

// v023TokenChar возвращает true для букв, десятичных цифр и + - . ,;
// остальные символы дают false и разделяют токены.
func v023TokenChar(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsDigit(char) ||
		char == '+' || char == '-' || char == '.' || char == ','
}

// v023TokenDetail возвращает конкретизацию либо пустую строку для неподходящего
// кандидата. Исходная цифра в самом токене обязательна. O/o/О/о смягчают
// остальные пороги числа цифр и соотношения с буквами;
// остальные условия, включая форму числа после замены, остаются обязательными.
// Замены выполняются в собственной копии, исходный токен не изменяется.
func v023TokenDetail(token []rune) string {
	normalized := append([]rune(nil), token...)
	digits, letters := 0, 0
	hasZeroLetter := false
	seen := ""
	var replacements []string
	for i, char := range token {
		if unicode.IsDigit(char) {
			digits++
			continue
		}
		if !unicode.IsLetter(char) {
			continue
		}
		digit, found := v023Replacement(char)
		if !found {
			return ""
		}
		letters++
		normalized[i] = digit
		if digit == '0' {
			hasZeroLetter = true
		}
		if !strings.ContainsRune(seen, char) {
			seen += string(char)
			replacements = append(replacements, string(char)+" → "+string(digit))
		}
	}
	if digits == 0 || letters == 0 || !hasZeroLetter && (digits < 2 || digits <= letters) || !v023Number(normalized) {
		return ""
	}
	return "Возможные замены: " + strings.Join(replacements, "; ")
}

// v023Replacement возвращает цифру и true только для точного списка сходств;
// для остальных символов возвращает 0 и false, не меняя регистр.
func v023Replacement(char rune) (rune, bool) {
	switch char {
	case 'O', 'o', 'О', 'о':
		return '0', true
	case 'I', 'l', 'І', 'і':
		return '1', true
	case 'Z', 'z':
		return '2', true
	case 'З', 'з':
		return '3', true
	case 'S', 's':
		return '5', true
	case 'б':
		return '6', true
	case 'B', 'В':
		return '8', true
	case 'g':
		return '9', true
	default:
		return 0, false
	}
}

// v023Number проверяет форму целого или десятичного числа с необязательным
// начальным знаком. Обе части десятичного числа непусты; значение не вычисляется.
func v023Number(token []rune) bool {
	i := 0
	if len(token) > 0 && (token[0] == '+' || token[0] == '-') {
		i++
	}
	start := i
	for i < len(token) && unicode.IsDigit(token[i]) {
		i++
	}
	if i == start {
		return false
	}
	if i == len(token) {
		return true
	}
	if token[i] != '.' && token[i] != ',' {
		return false
	}
	i++
	start = i
	for i < len(token) && unicode.IsDigit(token[i]) {
		i++
	}
	return i > start && i == len(token)
}

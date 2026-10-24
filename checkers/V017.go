package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV017 регистрирует предупреждение о пробелах перед пунктуацией
// только для DSL 1.2.
func registrationV017(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V017",
		Severity: diagnostics.SeverityWarning,
		Message:  "Перед знаком препинания находится пробел.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV017,
	})
}

// checkV017 проверяет content/title/name и внутренность ответов placeholder.
// Разрешённое форматирование в text прозрачно при поиске соседей.
// Отмечаются только пробелы в исходных Unicode-колонках с 1, конец исключается.
func checkV017(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		_, answers := v015PlaceholderContext(lines, line)
		formatting := textFormatting(lines, line, raw)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name", "placeholder":
			default:
				continue
			}
			for _, part := range v015TextParts(raw, element, answers) {
				markers := formatting
				if part.answer {
					markers = nil // В ANSWER HTML не является форматированием.
				}
				if err := v017CheckPart(raw, markers, part, number, findings); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v017CheckPart ищет пробелы в одном самостоятельном участке. Границы ответа
// не участвуют в поиске соседей, исходные раздельные пробелы не объединяются.
func v017CheckPart(raw []rune, formatting []bool, part v015TextPart, number int, findings diagnostics.Findings) error {
	text, positions := visibleText(raw, formatting, part.start, part.end)
	var following rune
	if !part.answer && part.end < len(raw) {
		following = raw[part.end]
	}
	for i := 0; i < len(text); {
		switch text[i] {
		case ',', '.', '?', '!', '…', ':', ';':
		default:
			i++
			continue
		}
		if end := v017StandaloneEllipsis(text, i, following); end > i {
			i = end
			continue
		}
		start := i
		for start > 0 && v017HorizontalSpace(text[start-1]) {
			start--
		}
		for start < i {
			end := start + 1
			for end < i && positions[end] == positions[end-1]+1 {
				end++
			}
			if _, err := findings.Add(number, positions[start]+1, number, positions[end-1]+2,
				v017V018Detail(text, start, i+1)); err != nil {
				return err
			}
			start = end
		}
		if text[i] == '.' {
			// Внутренние пробелы видимой цепочки из трёх и более точек
			// проверяет V003, в том числе через разрешённое форматирование.
			end, count, spaced := v017DotChain(text, i)
			if count >= 3 && spaced || !spaced {
				i = end
				continue
			}
		}
		i++
	}
	return nil
}

// v017StandaloneEllipsis возвращает конец отдельного … или трёх точек после пробела.
// Такой образец для продолжения неоднозначен и не требует предупреждения.
// Между точками допустимы горизонтальные пробелы; их оформление проверяет V003.
// Возврат start означает обычную пунктуацию. Длинные серии и примыкающая
// пунктуация исключение не получают. following сохраняет границу
// внешнего текста; для самостоятельного конца ответа он равен нулю.
func v017StandaloneEllipsis(text []rune, start int, following rune) int {
	if start == 0 || !v017HorizontalSpace(text[start-1]) {
		return start
	}
	end := start + 1
	switch text[start] {
	case '…':
	case '.':
		var count int
		end, count, _ = v017DotChain(text, start)
		if count != 3 {
			return start
		}
	default:
		return start
	}
	previous := start - 1
	for previous >= 0 && v017HorizontalSpace(text[previous]) {
		previous--
	}
	if previous >= 0 && (text[previous] == '.' || text[previous] == '…') {
		return start
	}
	next := end
	for next < len(text) && v017HorizontalSpace(text[next]) {
		next++
	}
	if next < len(text) && (text[next] == '.' || text[next] == '…') {
		return start
	}
	right := following
	if end < len(text) {
		right = text[end]
	}
	if right == 0 || v017HorizontalSpace(right) || unicode.In(right, unicode.Pe, unicode.Pf) || right == '\'' || right == '"' {
		return end
	}
	return start
}

// v017DotChain находит максимальную цепочку точек и горизонтальных промежутков
// от start, указывающего на точку. end — исключающий индекс после последней
// точки в кодовых точках с 0; count — число точек. spaced=true означает хотя бы
// один непустой внутренний промежуток; конечные пробелы в цепочку не входят.
func v017DotChain(raw []rune, start int) (end, count int, spaced bool) {
	end, count = start+1, 1
	for {
		next := end
		for next < len(raw) && v017HorizontalSpace(raw[next]) {
			next++
		}
		if next == len(raw) || raw[next] != '.' {
			return end, count, spaced
		}
		spaced = spaced || next > end
		end, count = next+1, count+1
	}
}

// v017HorizontalSpace возвращает true для U+0009 и Unicode Zs;
// false означает символ, не входящий в проверяемую последовательность пробелов.
func v017HorizontalSpace(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}

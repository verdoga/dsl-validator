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

// checkV015 проверяет исходные символы, исключая только синтаксис пропусков.
// Последовательности ограничены одним элементом или ответом плейсхолдера;
// диапазоны задаются в исходных Unicode-колонках с исключающим концом.
func checkV015(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		plain, answers := v015PlaceholderContext(lines, line)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name", "unparsed", "placeholder":
			default:
				continue
			}
			start, end := element.Start()-1, element.End()-1
			excluded := make([]bool, end-start)
			if element.Type() == "placeholder" || element.Type() == "content" && plain {
				for _, gap := range v015Placeholders(raw, start, end, answers || element.Type() == "placeholder") {
					prefixEnd := gap.end
					if gap.answerStart >= 0 {
						prefixEnd = gap.answerStart
						excluded[gap.end-1-start] = true
					}
					for i := gap.start; i < prefixEnd; i++ {
						excluded[i-start] = true
					}
				}
			}
			for offset := start; offset < end; {
				if excluded[offset-start] || v015Allowed(raw[offset]) {
					offset++
					continue
				}
				first := offset
				for offset < end && !excluded[offset-start] && !v015Allowed(raw[offset]) {
					offset++
				}
				if _, err := findings.Add(number, first+1,
					number, offset+1, v015Detail(raw[first:offset])); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v015Placeholder задаёт индексы рун [start,end) в Raw строки.
// answerStart равен -1 для обычного пропуска; иначе ответ занимает [answerStart,end-1).
type v015Placeholder struct {
	start, end, answerStart int
}

// v015TextPart задаёт самостоятельный участок [start,end) в исходных рунах.
// answer отличает внутренность ANSWER от окружающего текстового элемента.
type v015TextPart struct {
	start, end int
	answer     bool
}

// v015TextParts выделяет ответы одинаково из content и placeholder для
// V011, V012, V017, V018 и V030. Оболочка ответа исключается, участки не
// склеиваются. Ошибочная запись в content остаётся обычным проверяемым текстом.
// Типы остальных элементов и допустимость их проверки определяет вызывающий.
func v015TextParts(raw []rune, element diagnostics.Element, answers bool) []v015TextPart {
	start, end := element.Start()-1, element.End()-1
	placeholder := element.Type() == "placeholder"
	if !placeholder && (element.Type() != "content" || !answers) {
		return []v015TextPart{{start: start, end: end}}
	}
	var parts []v015TextPart
	for _, gap := range v015Placeholders(raw, start, end, true) {
		if gap.answerStart < 0 {
			continue
		}
		if !placeholder && start < gap.start {
			parts = append(parts, v015TextPart{start: start, end: gap.start})
		}
		parts = append(parts, v015TextPart{start: gap.answerStart, end: gap.end - 1, answer: true})
		start = gap.end
	}
	if !placeholder && start < end {
		parts = append(parts, v015TextPart{start: start, end: end})
	}
	return parts
}

// v015PlaceholderContext разрешает обычные пропуски и специальную форму ответа
// по DSL 4.3.3 и 4.3.10. false означает литеральное содержимое для данной формы.
func v015PlaceholderContext(lines diagnostics.Lines, line diagnostics.Line) (plain, answers bool) {
	if line.Type() == "tag" || line.Type() == "block-start" {
		switch v015Tag(line) {
		case "editor", "key", "answer":
			return false, false
		default:
			return true, false // CONTENT объявления — инструкция, а не тело блока.
		}
	}
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		switch v015Tag(line) {
		case "editor", "key", "answer", "choice", "multichoice", "matching", "ordering":
			return false, false
		case "multifill":
			return true, true
		default:
			return true, false
		}
	}
	return true, false
}

// v015Tag возвращает каноническое имя тега строки либо пустую строку.
func v015Tag(line diagnostics.Line) string {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "tag" {
			value, _ := element.Value()
			return value
		}
	}
	return ""
}

// v015Placeholders распознаёт пропуски в [start,end), не меняя Raw и не выдавая
// ошибок разметки. Ответы не разбираются повторно как плейсхолдеры. Функция также
// задаёт границы ответов для V018 независимо от регистрации или запуска V015.
func v015Placeholders(raw []rune, start, end int, answers bool) []v015Placeholder {
	var gaps []v015Placeholder
	for i := start; i < end; {
		if raw[i] != '_' {
			i++
			continue
		}
		first := i
		for i < end && raw[i] == '_' {
			i++
		}
		if i-first != 5 || first > 0 && raw[first-1] == '_' ||
			i < len(raw) && raw[i] == '_' || v015Escaped(raw, first) {
			continue
		}
		gap := v015Placeholder{start: first, end: i, answerStart: -1}
		if i < end && raw[i] == '{' {
			answerStart := i + 1
			valid, nonempty := answers, false
			i++
			for i < end && raw[i] != '}' {
				if raw[i] == '{' {
					valid = false
				}
				if !unicode.IsSpace(raw[i]) {
					nonempty = true
				}
				if raw[i] == '\\' && i+1 < end && strings.ContainsRune("{}\\", raw[i+1]) {
					i++
				}
				i++
			}
			if i == end {
				continue
			}
			i++ // Закрывающая скобка; экранированные скобки уже потреблены парами.
			if !valid || !nonempty {
				continue
			}
			gap.end, gap.answerStart = i, answerStart
		}
		gaps = append(gaps, gap)
	}
	return gaps
}

// v015Escaped возвращает true при нечётном числе обратных косых черт перед
// позицией; false означает, что символ может участвовать в синтаксисе DSL.
func v015Escaped(raw []rune, position int) bool {
	first := position
	for first > 0 && raw[first-1] == '\\' {
		first--
	}
	return (position-first)%2 != 0
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
	if strings.ContainsRune(" .,;:!?…'\"-–—()[]{}/\\+=%°№§@#$€£₽βθχ‿‖|↘↗↑↓→^&*~`<>\u00AD\u2010\u2011\u2012\u2015\uFE58\uFE63\uFF0D", char) {
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

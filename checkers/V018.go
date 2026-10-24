package checkers

import (
	"sort"
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

// checkV018 проверяет пробел после группы ? . ! , : ; с закрывающими знаками.
// Разрешённые HTML-маркеры прозрачны, а конец ответа завершает его текст.
// Сообщение охватывает последний знак пунктуации группы в исходных
// Unicode-колонках [column,column+1). Конкретизация показывает местный контекст.
func checkV018(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		_, answers := v015PlaceholderContext(lines, line)
		var html []bool
		if !answers && v016InsideText(lines, line) {
			html = v016HTMLMarkers(line, raw)
		}
		visible, positions := visibleText(raw, html, 0, len(raw))
		text := v018TextScan{raw: visible, positions: positions}
		var previous diagnostics.Element
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name", "placeholder":
			default:
				previous = nil
				continue
			}
			if !answers || !v018AcrossPlaceholder(raw, previous, element) {
				text.quotes = [2]bool{}
			}
			for _, part := range v015TextParts(raw, element, answers) {
				if part.answer {
					// Кавычки ответа не меняют цитату вокруг видимого пропуска.
					outside := text.quotes
					text.quotes = [2]bool{}
					text.scan(part.start, part.end, true)
					text.quotes = outside
				} else {
					text.scan(part.start, part.end, false)
				}
			}
			previous = element
		}
		for _, missing := range text.missing {
			if _, err := findings.Add(number, missing.position+1, number, missing.position+2, missing.detail); err != nil {
				return err
			}
		}
	}
	return nil
}

// v018AcrossPlaceholder сохраняет внешние кавычки только между content и
// корректным placeholder. Иные элементы и непустые текстовые промежутки
// прерывают связь; пробелы вне элементов допускаются, как в исходном Raw.
func v018AcrossPlaceholder(raw []rune, previous, current diagnostics.Element) bool {
	if previous == nil || previous.End() > current.Start() ||
		previous.Type() != "placeholder" && current.Type() != "placeholder" {
		return false
	}
	for _, element := range []diagnostics.Element{previous, current} {
		switch element.Type() {
		case "content":
		case "placeholder":
			gaps := v015Placeholders(raw, element.Start()-1, element.End()-1, true)
			if len(gaps) != 1 || gaps[0].start != element.Start()-1 || gaps[0].end != element.End()-1 {
				return false
			}
		default:
			return false
		}
	}
	for _, char := range raw[previous.End()-1 : current.Start()-1] {
		if !v018HorizontalSpace(char) {
			return false
		}
	}
	return true
}

// v018TextScan хранит состояние обхода строки с независимыми ответами.
// raw содержит видимые символы, positions — их исходные индексы рун.
// missing содержит позиции исходной строки и конкретизации срабатываний.
type v018TextScan struct {
	raw       []rune
	positions []int
	quotes    [2]bool // Открытые прямые одинарная и двойная кавычки.
	missing   []v018MissingSpace
}

// v018MissingSpace хранит индекс руны в исходной строке и видимый пример.
type v018MissingSpace struct {
	position int
	detail   string
}

// scan проверяет [start,end) в исходных индексах рун с 0.
// hardEnd=true завершает самостоятельный ответ;
// иначе символ за концом элемента учитывается, как и пробел в Raw вне элемента.
func (text *v018TextScan) scan(start, end int, hardEnd bool) {
	start, end = sort.SearchInts(text.positions, start), sort.SearchInts(text.positions, end)
	addresses := v018Addresses(text.raw[start:end])
	for i := start; i < end; {
		if addresses[i-start] {
			i++
			continue
		}
		if !v018Punctuation(text.raw[i]) {
			v018Quote(text.raw, i, &text.quotes)
			i++
			continue
		}
		if v018AbbreviationDot(text.raw[start:end], i-start) {
			i++
			continue
		}
		if (text.raw[i] == '.' || text.raw[i] == ',' || text.raw[i] == ':') &&
			i > 0 && i+1 < len(text.raw) && unicode.IsDigit(text.raw[i-1]) && unicode.IsDigit(text.raw[i+1]) {
			i++
			continue
		}
		last := i
		i++
		for i < end {
			if addresses[i-start] {
				break
			}
			if v018Punctuation(text.raw[i]) || v018GroupEllipsis(text.raw[start:end], i-start) {
				last = i
				i++
				continue
			}
			if unicode.Is(unicode.Pf, text.raw[i]) || unicode.Is(unicode.Pe, text.raw[i]) && text.raw[i] != '}' {
				i++
				continue
			}
			quotes := text.quotes
			if v018Quote(text.raw, i, &quotes) {
				text.quotes = quotes
				i++
				continue
			}
			break
		}
		if i == end && hardEnd || i == len(text.raw) || v018HorizontalSpace(text.raw[i]) {
			continue
		}
		text.missing = append(text.missing, v018MissingSpace{
			position: text.positions[last],
			detail:   v017V018Detail(text.raw[start:end], last-start, last-start+1),
		})
	}
}

// v018Punctuation возвращает true только для знаков, проверяемых V018.
func v018Punctuation(char rune) bool {
	switch char {
	case '?', '.', '!', ',', ':', ';':
		return true
	default:
		return false
	}
}

// v018Quote обновляет состояние прямой кавычки и возвращает true при закрытии.
// false означает открытие либо обычный символ; апостроф внутри слова и
// экранированная кавычка не меняют состояние. Завершающая кавычка после
// пунктуации допустима и без парной открывающей: V018 не проверяет их парность.
func v018Quote(raw []rune, i int, quotes *[2]bool) bool {
	if raw[i] != '\'' && raw[i] != '"' || v015Escaped(raw, i) {
		return false
	}
	if raw[i] == '\'' && i > 0 && i+1 < len(raw) &&
		(unicode.IsLetter(raw[i-1]) || unicode.IsMark(raw[i-1]) || unicode.IsDigit(raw[i-1])) &&
		(unicode.IsLetter(raw[i+1]) || unicode.IsMark(raw[i+1]) || unicode.IsDigit(raw[i+1])) {
		return false
	}
	index := 0
	if raw[i] == '"' {
		index = 1
	}
	closed := quotes[index] || i > 0 &&
		(v018Punctuation(raw[i-1]) || v018GroupEllipsis(raw, i-1) || unicode.In(raw[i-1], unicode.Pe, unicode.Pf)) &&
		(i+1 == len(raw) || v018HorizontalSpace(raw[i+1]) ||
			unicode.In(raw[i+1], unicode.Pe, unicode.Pf) || v018Punctuation(raw[i+1]) || raw[i+1] == '\'' || raw[i+1] == '"')
	quotes[index] = !closed
	return closed
}

// v018HorizontalSpace возвращает true для TAB и Unicode-категории Zs;
// для остальных символов, включая вертикальные разделители, возвращает false.
func v018HorizontalSpace(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}

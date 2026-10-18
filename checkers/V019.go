package checkers

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV019 регистрирует предупреждение о смешении Latin и Cyrillic
// внутри одного слова только для DSL 1.2.
func registrationV019(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V019",
		Severity: diagnostics.SeverityWarning,
		Message:  "В одном слове смешаны кириллические и латинские буквы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV019,
	})
}

// checkV019 проверяет точный Raw элементов content/title/name и placeholder
// с ответом. Разрешённое форматирование в text прозрачно для границ слова;
// адреса не распознаются, элементы и строки не склеиваются.
// Диапазон охватывает слово в исходных Unicode-колонках с 1, конец исключается.
func checkV019(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		formatting := v019Formatting(lines, line, raw)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name":
			case "placeholder":
				if _, present := element.Value(); !present {
					continue
				}
			default:
				continue
			}
			text, positions := v019Visible(raw, formatting, element.Start()-1, element.End()-1)
			for start := 0; start < len(text); {
				if !v019Letter(text[start]) {
					start++
					continue
				}
				end := v019WordEnd(text, start)
				if detail := v019Detail(text[start:end]); detail != "" {
					if _, err := findings.Add(number, positions[start]+1, number, positions[end-1]+2, detail); err != nil {
						return err
					}
				}
				start = end
			}
		}
	}
	return nil
}

// v019Formatting отмечает разрешённое форматирование внутри content блока
// text. Маска используется также V012, V017 и V020 без запуска V016.
// Отдельный <br> остаётся границей; литеральные и экранированные теги не скрыты.
func v019Formatting(lines diagnostics.Lines, line diagnostics.Line, raw []rune) []bool {
	if !v016InsideText(lines, line) || strings.EqualFold(strings.Trim(line.Raw(), " \t"), "<br>") {
		return nil
	}
	return v016HTMLMarkers(line, raw)
}

// v019Visible исключает только отмеченные маркеры из [start,end). positions
// хранит исходный индекс каждой оставшейся кодовой точки; Raw не изменяется.
func v019Visible(raw []rune, formatting []bool, start, end int) (text []rune, positions []int) {
	text = make([]rune, 0, end-start)
	positions = make([]int, 0, end-start)
	for i := start; i < end; i++ {
		if len(formatting) != 0 && formatting[i] {
			continue
		}
		text = append(text, raw[i])
		positions = append(positions, i)
	}
	return text, positions
}

// v019Apostrophe возвращает true для трёх соединительных апострофов V019;
// остальные символы дают false.
func v019Apostrophe(char rune) bool {
	return char == '\'' || char == '\u2019' || char == '\u02BC'
}

// v019Letter возвращает true для L, кроме апострофа U+02BC:
// его соединительное правило имеет приоритет над категорией Unicode.
func v019Letter(char rune) bool {
	return unicode.IsLetter(char) && !v019Apostrophe(char)
}

// v019WordEnd возвращает исключающий индекс конца слова, начатого с L в start.
// M присоединяются к буквенной части; апостроф допускается только перед новой L.
func v019WordEnd(text []rune, start int) int {
	end := start + 1
	for end < len(text) {
		char := text[end]
		if v019Apostrophe(char) {
			if end+1 >= len(text) || !v019Letter(text[end+1]) {
				break
			}
		} else if !unicode.IsLetter(char) && !unicode.IsMark(char) {
			break
		}
		end++
	}
	return end
}

// v019Detail возвращает конкретизацию смешанного слова либо пустую строку.
// Перечисляет уникальные буквы Latin и Cyrillic в порядке появления, сохраняя
// регистр. Диакритика и буквы других письменностей не добавляют эти признаки.
func v019Detail(word []rune) string {
	var seen []rune
	var symbols []string
	latin, cyrillic := false, false
	for _, char := range word {
		if !v019Letter(char) || slices.Contains(seen, char) {
			continue
		}
		var script string
		switch {
		case unicode.Is(unicode.Latin, char):
			latin, script = true, "Latin"
		case unicode.Is(unicode.Cyrillic, char):
			cyrillic, script = true, "Cyrillic"
		default:
			continue
		}
		seen = append(seen, char)
		symbols = append(symbols, fmt.Sprintf("«%c» (U+%04X, %s)", char, char, script))
	}
	if !latin || !cyrillic {
		return ""
	}
	return "Символы: " + strings.Join(symbols, ", ") + "."
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV105 регистрирует уточнённый ошибочный шаблон только для DSL 1.2.
func registrationV105(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V105",
		Severity: diagnostics.SeverityError,
		Message:  "Перед открывающим < разделённой ячейки есть текст.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV105,
	})
}

// checkV105 отмечает исходный префикс и связывает его с ошибочным открытием.
func checkV105(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found || !v105DataRow(line) {
			continue
		}
		block, _ := lines.Line(parent)
		if block.Type() != "block-start" || !strings.EqualFold(v105Tag(block), "table") {
			continue
		}
		for _, span := range v105Patterns(line.Raw()) {
			occurrence, err := findings.Add(number, span[0], number, span[1], "")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, number, span[2], number, span[2]+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v105Tag возвращает распознанное имя тега либо пустую строку.
func v105Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v105DataRow исключает объявления, границы и пустые строки.
// Наличие unparsed не мешает самостоятельному распознаванию по Raw.
func v105DataRow(line diagnostics.Line) bool {
	if line.Type() != "content" && line.Type() != "invalid" || v105Tag(line) != "" {
		return false
	}
	raw := strings.Trim(line.Raw(), " \t")
	if raw == "" || raw == "---" || raw == "{" || raw == "}" {
		return false
	}
	// По DSL 1.15–1.16 начальные неэкранированные @/# — попытки разметки.
	return raw[0] != '@' && raw[0] != '#'
}

// v105Patterns возвращает начало/исключающий конец префикса и колонку <.
// Все координаты исходные, в Unicode-символах, начиная с единицы.
func v105Patterns(raw string) [][3]int {
	text := []rune(raw)
	var spans [][3]int
	first, end := -1, 0
	split := false
	for i := 0; i < len(text); i++ {
		char := text[i]
		if char == ' ' || char == '\t' {
			continue
		}
		if v105Escape(text, i) {
			if first < 0 {
				first = i
			}
			end = i + 2
			i++
			continue
		}
		if split {
			if char == '>' {
				split = false
			}
			end = i + 1
			continue
		}
		if char == '|' {
			first, end = -1, 0
			continue
		}
		if char == '<' {
			if first < 0 {
				split = true
			} else if closing, found := v105PatternEnd(text, i); found {
				spans = append(spans, [3]int{first + 1, end + 1, i + 1})
				// Черты подтверждённого шаблона не начинают внешнюю ячейку.
				i = closing
			}
		}
		if first < 0 {
			first = i
		}
		end = i + 1
	}
	return spans
}

// v105PatternEnd проверяет остаток кандидата до закрытия в той же строке.
// Найденный конец — индекс > с нуля; found=false означает неподходящий шаблон.
func v105PatternEnd(text []rune, opening int) (closing int, found bool) {
	separator := false
	for i := opening + 1; i < len(text); i++ {
		if v105Escape(text, i) {
			i++
			continue
		}
		switch text[i] {
		case '<':
			return 0, false
		case '|':
			separator = true
		case '>':
			next := i + 1
			for next < len(text) && (text[next] == ' ' || text[next] == '\t') {
				next++
			}
			return i, separator && (next == len(text) || text[next] == '|')
		}
	}
	return 0, false
}

// v105Escape определяет двухсимвольную пару без снятия экранирования.
func v105Escape(text []rune, index int) bool {
	return text[index] == '\\' && index+1 < len(text) && strings.ContainsRune("\\|<>", text[index+1])
}

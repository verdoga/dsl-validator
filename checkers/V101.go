package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV101 регистрирует минимум внутренних частей только для DSL 1.2.
func registrationV101(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V101",
		Severity: diagnostics.SeverityError,
		Message:  "Разделённая ячейка содержит меньше двух внутренних частей.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV101,
	})
}

// checkV101 проверяет отдельные ячейки непосредственных строк таблицы.
// Ошибка соседней ячейки не отменяет уже выделенные достоверные диапазоны.
func checkV101(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found || !v101DataRow(line) {
			continue
		}
		block, _ := lines.Line(parent)
		if block.Type() != "block-start" || !strings.EqualFold(v101Tag(block), "table") {
			continue
		}
		for _, span := range v101Cells(line.Raw()) {
			if _, err := findings.Add(number, span[0], number, span[1], ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v101Tag возвращает распознанное имя тега либо пустую строку.
func v101Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v101DataRow исключает объявления, границы и пустые строки.
// Неразобранные данные исследуются по Raw независимо от других диагностик.
func v101DataRow(line diagnostics.Line) bool {
	if line.Type() != "content" && line.Type() != "invalid" || v101Tag(line) != "" {
		return false
	}
	raw := strings.Trim(line.Raw(), " \t")
	if raw == "" || raw == "---" || raw == "{" || raw == "}" {
		return false
	}
	// По DSL 1.15–1.16 неэкранированные начальные @/# начинают разметку.
	return raw[0] != '@' && raw[0] != '#'
}

// v101Cells возвращает диапазоны закрытых невложенных ячеек с одной частью.
// Диапазоны включают угловые скобки; Unicode-колонки начинаются с единицы,
// правая граница исключается. Экранирование не декодируется.
func v101Cells(raw string) [][2]int {
	const (
		start = iota
		ordinary
		split
		closed
	)
	text := []rune(raw)
	var spans [][2]int
	state, opening, closing, parts := start, 0, 0, 1
	nested := false
	for i := 0; i < len(text); i++ {
		char := text[i]
		if char == ' ' || char == '\t' {
			continue
		}
		if char == '\\' && i+1 < len(text) && strings.ContainsRune("\\|<>", text[i+1]) {
			if state != split {
				state = ordinary
			}
			i++
			continue
		}
		if state == split {
			switch char {
			case '<':
				nested = true
			case '>':
				state, closing = closed, i
			case '|':
				parts++
			}
			continue
		}
		if char == '|' {
			if state == closed && !nested && parts < 2 {
				spans = append(spans, [2]int{opening + 1, closing + 2})
			}
			state = start
			continue
		}
		if state == start && char == '<' {
			state, opening, parts, nested = split, i, 1, false
			continue
		}
		// В том числе отвергается непробельный хвост после закрывающего >.
		state = ordinary
	}
	if state == closed && !nested && parts < 2 {
		spans = append(spans, [2]int{opening + 1, closing + 2})
	}
	return spans
}

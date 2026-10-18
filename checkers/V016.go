package checkers

import (
	"fmt"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV016 регистрирует проверку специальных символов только для DSL 1.2.
func registrationV016(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V016",
		Severity: diagnostics.SeverityWarning,
		Message:  "В тексте присутствует запрещённый специальный символ.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV016,
	})
}

// checkV016 проверяет весь Raw, включая служебные поля и unparsed.
// Исключаются разрешённые HTML-маркеры в content блока text и структурные
// угловые скобки разделённых ячеек в строках содержимого table.
// Каждый символ отмечается в исходных Unicode-колонках [column,column+1).
func checkV016(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		var excluded []bool
		if v016InsideText(lines, line) {
			excluded = v016HTMLMarkers(line, raw)
		}
		if parent, found := line.ParentLine(); found && line.Type() == "content" {
			block, _ := lines.Line(parent)
			if block.Type() == "block-start" && v015Tag(block) == "table" {
				table := v016TableMarkers(line, raw)
				if excluded == nil {
					excluded = table
				} else {
					for i := range table {
						excluded[i] = excluded[i] || table[i]
					}
				}
			}
		}
		for index, char := range raw {
			switch char {
			case '^', '&', '*', '`', '~', '<', '>':
				if len(excluded) != 0 && excluded[index] {
					continue
				}
				if _, err := findings.Add(number, index+1, number, index+2,
					fmt.Sprintf("Обнаружен символ %c U+%04X", char, char)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v016TableMarkers отмечает только < и > допустимых разделённых ячеек.
// Границы ищутся в Raw строки: парсер может сохранить части разными content.
// Служебные элементы и unparsed исключений не получают. Экранированные знаки
// остаются данными и проверяются обычным обходом V016.
func v016TableMarkers(line diagnostics.Line, raw []rune) []bool {
	excluded := make([]bool, len(raw))
	literal := make([]bool, len(raw))
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			for i := element.Start() - 1; i < element.End()-1; i++ {
				literal[i] = true
			}
		}
	}
	cellStart := true
	for i := 0; i < len(raw); {
		if raw[i] == '|' && !v015Escaped(raw, i) {
			cellStart = true
			i++
			continue
		}
		if cellStart && (raw[i] == ' ' || raw[i] == '\t') {
			i++
			continue
		}
		if !cellStart || raw[i] != '<' || v015Escaped(raw, i) {
			cellStart = false
			i++
			continue
		}
		start, separator, valid := i, false, !literal[i]
		i++
		for i < len(raw) {
			valid = valid && !literal[i]
			if strings.ContainsRune("<>|", raw[i]) && !v015Escaped(raw, i) {
				if raw[i] == '>' {
					break
				}
				if raw[i] == '<' {
					valid = false // Вложенные разделённые ячейки не поддерживаются.
				}
				if raw[i] == '|' {
					separator = true
				}
			}
			i++
		}
		if i == len(raw) {
			break
		}
		next := i + 1
		for next < len(raw) && (raw[next] == ' ' || raw[next] == '\t') {
			next++
		}
		if valid && separator && (next == len(raw) || raw[next] == '|') {
			excluded[start], excluded[i] = true, true
		}
		cellStart = false
		i++
	}
	return excluded
}

// v016InsideText возвращает true, если среди предков есть блочный text.
// Корректность ссылок на предшествующие строки гарантирует контроль модели.
func v016InsideText(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				if value, present := element.Value(); present && value == "text" {
					return true
				}
			}
		}
	}
}

// v016HTMLMarkers отмечает исходные позиции разрешённых маркеров внутри
// отдельных content. Парность HTML не проверяется. Экранирование учитывается
// только при распознавании маркера и само по себе не исключает символы.
func v016HTMLMarkers(line diagnostics.Line, raw []rune) []bool {
	excluded := make([]bool, len(raw))
	standaloneBR := strings.ToLower(strings.Trim(line.Raw(), " \t")) == "<br>"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		for start := element.Start() - 1; start < element.End()-1; start++ {
			if raw[start] != '<' {
				continue
			}
			backslashes := 0
			for previous := start - 1; previous >= 0 && raw[previous] == '\\'; previous-- {
				backslashes++
			}
			if backslashes%2 != 0 {
				continue
			}
			for _, marker := range [...]string{"<strong>", "</strong>", "<em>", "</em>", "<u>", "</u>", "<br>"} {
				end := start + len(marker) // Все разрешённые маркеры состоят из ASCII.
				if end > element.End()-1 || (marker == "<br>" && !standaloneBR) {
					continue
				}
				if strings.ToLower(string(raw[start:end])) != marker {
					continue
				}
				for position := start; position < end; position++ {
					excluded[position] = true
				}
				start = end - 1
				break
			}
		}
	}
	return excluded
}

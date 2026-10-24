package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV103 регистрирует ограничение заголовочной части одной строкой.
func registrationV103(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V103",
		Severity: diagnostics.SeverityError,
		Message:  "Заголовочная часть таблицы содержит больше одной строки.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV103,
	})
}

// checkV103 группирует непосредственные строки по готовому родительству.
// invalid и unparsed не исключают строку; объявления вложенных конструкций
// и закрывающие скобки не являются строками таблицы.
func checkV103(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		switch line.Type() {
		case "tag", "heading", "block-start", "block-end":
			continue
		}
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if !v103Table(opening) {
			continue
		}
		rows := children[number]
		separator, count := 0, 0
		for index, line := range rows {
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator = index
				count++
			}
		}
		if count != 1 {
			continue
		}
		firstLine, firstStart, firstEnd := 0, 0, 0
		for _, line := range rows[:separator] {
			start, end := v103Range(line.Raw())
			if start == end {
				continue
			}
			if firstLine == 0 {
				firstLine, firstStart, firstEnd = line.Number(), start, end
				continue
			}
			occurrence, err := findings.Add(line.Number(), start, line.Number(), end, "")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, firstLine, firstStart, firstLine, firstEnd); err != nil {
				return err
			}
		}
	}
	return nil
}

// v103Table возвращает true для распознанного объявления блока table,
// иначе false. Объявления по исходному тексту не восстанавливаются.
func v103Table(line diagnostics.Line) bool {
	if line.Type() != "block-start" {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "table"
		}
	}
	return false
}

// v103Range возвращает диапазон без крайних U+0020/U+0009 в Unicode-колонках
// с 1 и исключённой правой границей. Равные границы означают пустую строку.
func v103Range(raw string) (start, end int) {
	runes := []rune(raw)
	start, end = 0, len(runes)
	for start < end && (runes[start] == ' ' || runes[start] == '\t') {
		start++
	}
	for end > start && (runes[end-1] == ' ' || runes[end-1] == '\t') {
		end--
	}
	return start + 1, end + 1
}

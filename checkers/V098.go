package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV098 регистрирует проверку незакрытых разделённых ячеек таблицы.
func registrationV098(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V098",
		Severity: diagnostics.SeverityError,
		Message:  "Разделённая ячейка не закрыта до конца строки.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV098,
	})
}

// checkV098 проверяет непосредственные строки данных таблиц по исходному Raw,
// включая неразобранные участки строки. Заголовочная часть обрабатывается так же,
// как тело; стек открытий не переносится между физическими строками.
func checkV098(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v098TableRow(lines, line) {
			continue
		}
		raw := line.Raw()
		if trimmed := strings.Trim(raw, " \t"); trimmed == "" || trimmed == "---" {
			continue
		}
		for _, column := range v098Unclosed(raw) {
			if _, err := findings.Add(number, column, number, column+1, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v098TableRow возвращает true только для строки содержимого с непосредственным
// родителем table. Объявления, закрытия и содержимое вложенных конструкций
// не становятся строками внешней таблицы. Используется готовая модель блоков.
func v098TableRow(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() != "content" {
		return false
	}
	parent, found := line.ParentLine()
	if !found {
		return false
	}
	block, _ := lines.Line(parent)
	if block.Type() != "block-start" {
		return false
	}
	for index := 0; index < block.ElementCount(); index++ {
		element, _ := block.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "table"
		}
	}
	return false
}

// v098Unclosed возвращает исходные Unicode-колонки незакрытых < с 1.
// Пары экранирования потребляются без декодирования. Вложенные открытия
// учитываются только для восстановления границ, без отдельной ошибки вложения.
func v098Unclosed(raw string) []int {
	var openings []int
	first := true
	runes := []rune(raw)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' && index+1 < len(runes) && strings.ContainsRune("\\|<>", runes[index+1]) {
			index++
			first = false // Литеральный символ занимает непробельную позицию.
			continue
		}
		if len(openings) != 0 {
			switch char {
			case '<':
				openings = append(openings, index+1)
			case '>':
				openings = openings[:len(openings)-1]
			}
			continue
		}
		if char == '|' {
			first = true
			continue
		}
		if first && unicode.Is(unicode.White_Space, char) {
			continue
		}
		if first && char == '<' {
			openings = append(openings, index+1)
		}
		first = false
	}
	return openings
}

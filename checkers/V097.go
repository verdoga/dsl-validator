package checkers

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v097Reference хранит количество ячеек и исходный диапазон строки-образца.
// end — исключающая Unicode-колонка, начиная с единицы.
type v097Reference struct {
	line, end, cells int
}

// registrationV097 регистрирует проверку внешних ячеек только для DSL 1.2.
func registrationV097(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V097",
		Severity: diagnostics.SeverityError,
		Message:  "Количество внешних ячеек отличается от других строк таблицы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV097,
	})
}

// checkV097 сравнивает непосредственные строки каждой таблицы с первым
// пригодным образцом. Заголовочная часть и тело используют один образец.
func checkV097(lines diagnostics.Lines, findings diagnostics.Findings) error {
	references := make(map[int]v097Reference)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found || !v097DataRow(line) {
			continue
		}
		block, _ := lines.Line(parent)
		if block.Type() != "block-start" || !strings.EqualFold(v097Tag(block), "table") {
			continue
		}
		cells, valid := v097Cells(line.Raw())
		if !valid {
			continue
		}
		end := utf8.RuneCountInString(line.Raw()) + 1
		reference, exists := references[parent]
		if !exists {
			references[parent] = v097Reference{number, end, cells}
			continue
		}
		if cells == reference.cells {
			continue
		}
		occurrence, err := findings.Add(number, 1, number, end,
			fmt.Sprintf("Найдено внешних ячеек: %d; в строке-образце: %d.", cells, reference.cells))
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, reference.line, 1, reference.line, reference.end); err != nil {
			return err
		}
	}
	return nil
}

// v097Tag возвращает распознанное имя тега либо пустую строку.
func v097Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v097DataRow исключает объявления, границы и пустые строки. Ошибочный
// разбор содержимого не заменяет самостоятельную проверку ячеек по Raw.
func v097DataRow(line diagnostics.Line) bool {
	if line.Type() != "content" && line.Type() != "invalid" || v097Tag(line) != "" {
		return false
	}
	raw := strings.Trim(line.Raw(), " \t")
	if raw == "" || raw == "---" || raw == "{" || raw == "}" {
		return false
	}
	// По DSL 1.15–1.16 начальные неэкранированные @/# — попытки разметки.
	return raw[0] != '@' && raw[0] != '#'
}

// v097Cells считает внешние ячейки без декодирования исходника.
// valid=false исключает всю строку при дефекте разделённой ячейки.
// Структурные символы однобайтовые; остальные байты остаются текстом.
func v097Cells(raw string) (cells int, valid bool) {
	const (
		start = iota
		ordinary
		split
		closed
	)
	cells, state := 1, start
	innerSeparator, lastGreater := false, false
	for i := 0; i < len(raw); i++ {
		char := raw[i]
		if char == ' ' || char == '\t' {
			continue
		}
		if char == '\\' && i+1 < len(raw) && strings.ContainsRune("\\|<>", rune(raw[i+1])) {
			if state == closed {
				return 0, false
			}
			if state == start {
				state = ordinary
			}
			lastGreater = false
			i++
			continue
		}
		if state == split {
			switch char {
			case '<':
				return 0, false
			case '>':
				if !innerSeparator {
					return 0, false
				}
				state = closed
			case '|':
				innerSeparator = true
			}
			continue
		}
		if char == '|' {
			if lastGreater {
				return 0, false
			}
			cells++
			state = start
			continue
		}
		if state == closed {
			return 0, false
		}
		if state == start && char == '<' {
			state, innerSeparator = split, false
			continue
		}
		state, lastGreater = ordinary, char == '>'
	}
	return cells, state != split && !lastGreater
}

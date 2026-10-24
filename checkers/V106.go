package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV106 регистрирует дополнительный запрет разделённых ячеек
// в заголовочной части таблицы DSL 1.2.
// Равенство числа внешних ячеек заголовка и тела отдельно проверяет V097.
func registrationV106(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V106",
		Severity: diagnostics.SeverityError,
		Message:  "В заголовочной части таблицы использована разделённая ячейка.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV106,
	})
}

// v106Separator хранит число непосредственных разделителей блока и номер
// последнего из них. Номер используется только при count == 1.
type v106Separator struct {
	count, line int
}

// checkV106 определяет заголовочные части по Raw и готовым связям родителей.
// Разделители вложенных блоков не влияют на заголовок внешней таблицы.
func checkV106(lines diagnostics.Lines, findings diagnostics.Findings) error {
	separators := make(map[int]v106Separator)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found || strings.Trim(line.Raw(), " \t") != "---" {
			continue
		}
		separator := separators[parent]
		separator.count++
		separator.line = number
		separators[parent] = separator
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		separator := separators[parent]
		if separator.count != 1 || number >= separator.line || !v106TableRow(lines, line) {
			continue
		}
		if err := v106Row(line, findings); err != nil {
			return err
		}
	}
	return nil
}

// v106TableRow возвращает true только для непосредственного содержимого table.
// Наличие unparsed не исключает строку; объявления и границы не являются данными.
func v106TableRow(lines diagnostics.Lines, line diagnostics.Line) bool {
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

// v106Row отмечает внешние открытия < в исходных Unicode-колонках с 1,
// с исключающим концом диапазона. Глубина служит только восстановлению границ;
// состояние не переносится на следующую физическую строку.
func v106Row(line diagnostics.Line, findings diagnostics.Findings) error {
	raw := []rune(line.Raw())
	first, depth := true, 0
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) && strings.ContainsRune("\\|<>", raw[index+1]) {
			index++
			first = false // Литеральный знак занимает непробельную позицию.
			continue
		}
		if depth > 0 {
			switch char {
			case '<':
				depth++
			case '>':
				depth--
			}
			continue
		}
		if char == '|' {
			first = true
			continue
		}
		if first && (char == ' ' || char == '\t') {
			continue
		}
		if first && char == '<' {
			if _, err := findings.Add(line.Number(), index+1, line.Number(), index+2, ""); err != nil {
				return err
			}
			depth = 1
		}
		first = false
	}
	return nil
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v104CellState различает начало, обычную и разделённую ячейки и её закрытый хвост.
type v104CellState uint8

const (
	v104CellStart v104CellState = iota
	v104CellPlain
	v104CellDivided
	v104CellClosed
)

// registrationV104 регистрирует рекомендацию о пустых строках table для DSL 1.2.
func registrationV104(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V104",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Все ячейки строки таблицы пусты.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV104,
	})
}

// checkV104 отмечает собственные пустые строки данных каждого блока table.
// Отступ и конечные U+0020/U+0009 не входят в исходный диапазон сообщения.
func checkV104(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "content" && line.Type() != "invalid" {
			continue
		}
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		block, _ := lines.Line(parent)
		if !v104Table(block) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := 0, len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end || string(raw[start:end]) == "---" || !v104EmptyRow(raw[start:end]) {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v104Table сообщает, является ли строка открытием блока с каноническим тегом table.
func v104Table(line diagnostics.Line) bool {
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

// v104EmptyRow возвращает true только для строки с корректной структурой
// разделённых ячеек и без содержимого во всех ячейках и внутренних частях.
// Экранированные знаки являются содержимым; Raw и его координаты не изменяются.
func v104EmptyRow(raw []rune) bool {
	state, parts, content := v104CellStart, 0, false
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == ' ' || char == '\t' {
			continue
		}
		if state == v104CellClosed && char != '|' {
			return false
		}
		if char == '\\' && index+1 < len(raw) && strings.ContainsRune("\\|<>", raw[index+1]) {
			index++
			content = true
			if state == v104CellStart {
				state = v104CellPlain
			}
			continue
		}
		switch {
		case char == '|':
			if state == v104CellDivided {
				parts++
			} else {
				state = v104CellStart
			}
		case char == '<' && state == v104CellStart:
			state, parts = v104CellDivided, 1
		case char == '<' && state == v104CellDivided:
			return false
		case char == '>' && state == v104CellDivided:
			if parts < 2 {
				return false
			}
			state = v104CellClosed
		default:
			content = true
			if state == v104CellStart {
				state = v104CellPlain
			}
		}
	}
	return state != v104CellDivided && !content
}

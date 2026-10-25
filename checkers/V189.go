package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV189 регистрирует отсутствие названия ветви только для DSL 1.2.
func registrationV189(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V189",
		Severity: diagnostics.SeverityError,
		Message:  "У @variant отсутствует название ветви.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV189,
	})
}

// checkV189 проверяет каждое реальное однострочное объявление отдельно.
func checkV189(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, start, end := v189Declaration(line)
		if !strings.EqualFold(name, "variant") || v189Literal(lines, line) || v189BlockAttempt(line, end) || v189HasName(line, end) {
			continue
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v189Declaration читает начальный tag либо имя ошибочной строки.
// Без tag граница — только U+0020 или конец после удаления краевых SP/TAB.
// Возвращает Unicode-диапазон [start,end) с 1; пустое имя означает отсутствие тега.
func v189Declaration(line diagnostics.Line) (name string, start, end int) {
	raw := []rune(line.Raw())
	first, last := 0, len(raw)
	for first < last && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	if first == last || raw[first] != '@' {
		return "", 0, 0
	}
	canRecover := line.Type() == "invalid"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == first+1 {
			name, _ = element.Value()
			return name, element.Start(), element.End()
		}
		canRecover = canRecover || element.Type() == "unparsed"
	}
	if !canRecover {
		return "", 0, 0
	}
	for last > first && (raw[last-1] == ' ' || raw[last-1] == '\t') {
		last--
	}
	after := first + 1
	for after < last && raw[after] != ' ' {
		after++
	}
	return string(raw[first+1 : after]), first + 1, after + 1
}

// v189Block возвращает true при сохранённом открытии фигурного блока.
func v189Block(line diagnostics.Line) bool {
	if line.Type() == "block-start" {
		return true
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			return true
		}
	}
	return false
}

// v189Literal возвращает true внутри литеральных editor/multifill,
// false — в остальных контекстах, включая запрещённые вложенные объявления.
func v189Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if !v189Block(line) {
			continue
		}
		name, _, _ := v189Declaration(line)
		if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
			return true
		}
	}
	return false
}

// v189BlockAttempt исключает сохранённую и исходную блочную форму для V109.
// Завершающая { после U+0020 не экранирована: непосредственно перед ней нет \.
func v189BlockAttempt(line diagnostics.Line, tagEnd int) bool {
	if v189Block(line) {
		return true
	}
	raw := []rune(line.Raw())
	last := len(raw)
	for last > tagEnd-1 && (raw[last-1] == ' ' || raw[last-1] == '\t') {
		last--
	}
	return last > tagEnd && raw[last-1] == '{' && raw[last-2] == ' '
}

// v189HasName возвращает true, если полное название содержит непробельный
// символ. Готовое name.Value не декодируется; исходные промежутки и хвост,
// включая unparsed, не теряются. false означает отсутствие непустого названия.
func v189HasName(line diagnostics.Line, tagEnd int) bool {
	raw := []rune(line.Raw())
	cursor := tagEnd - 1
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "name" || element.Start()-1 < cursor {
			continue
		}
		value, present := element.Value()
		if !present {
			continue
		}
		if v189Nonempty(value) || v189Nonempty(v189Decode(raw[cursor:element.Start()-1])) {
			return true
		}
		cursor = element.End() - 1
	}
	return v189Nonempty(v189Decode(raw[cursor:]))
}

// v189Decode однократно снимает разрешённые в NAME экранирования \\, \{, \}.
// Неизвестные пары сохраняются; @ и # после тега не находятся в начале строки.
func v189Decode(raw []rune) string {
	var text strings.Builder
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' {
				char = next
				index++
			}
		}
		text.WriteRune(char)
	}
	return text.String()
}

// v189Nonempty возвращает true при наличии символа вне Unicode White_Space.
func v189Nonempty(text string) bool {
	for _, char := range text {
		if !unicode.Is(unicode.White_Space, char) {
			return true
		}
	}
	return false
}

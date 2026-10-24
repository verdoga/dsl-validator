package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV066 регистрирует запрет хвоста трёх тегов только для DSL 1.2.
func registrationV066(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V066",
		Severity: diagnostics.SeverityError,
		Message:  "После имени тега есть запрещённое содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV066,
	})
}

// checkV066 отмечает весь запрещённый хвост в исходных Unicode-колонках.
// Крайние U+0020/U+0009 исключаются; правая граница диапазона исключающая.
func checkV066(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v066LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		name, start := v066Declaration(line, raw)
		if name == "" {
			continue
		}
		end := len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end {
			continue
		}
		detail := "Тег @" + name + " записывается без параметров и блочного содержимого."
		if _, err := findings.Add(number, start+1, number, end+1, detail); err != nil {
			return err
		}
	}
	return nil
}

// v066Declaration возвращает каноническое имя и индекс руны после него.
// Пустое имя означает, что строка не является проверяемым объявлением.
// Распознавание Raw применяется только при отсутствии элемента tag.
func v066Declaration(line diagnostics.Line, raw []rune) (string, int) {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		name, _ := element.Value()
		switch name {
		case "endtask", "speaking", "newpage":
			return name, element.End() - 1
		default:
			return "", 0
		}
	}
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	for _, name := range []string{"endtask", "speaking", "newpage"} {
		end := start + 1 + len(name)
		if end > len(raw) || !strings.EqualFold(string(raw[start+1:end]), name) {
			continue
		}
		if end == len(raw) || raw[end] == '{' || raw[end] == '}' ||
			unicode.Is(unicode.White_Space, raw[end]) {
			return name, end
		}
	}
	return "", 0
}

// v066LiteralContent возвращает true только для потомков editor или multifill.
// Другие блочные предки не исключают проверку вложенных объявлений.
func v066LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

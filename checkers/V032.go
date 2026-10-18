package checkers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV032 регистрирует проверку символов идентификатора для DSL 1.2.
func registrationV032(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V032",
		Severity: diagnostics.SeverityError,
		Message:  "Идентификатор содержит недопустимые символы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV032,
	})
}

// checkV032 проверяет исходный аргумент распознанного тега, включая unparsed.
// Диапазон охватывает аргумент без крайних U+0020 и U+0009; Unicode-колонки
// начинаются с 1, правая граница исключается. Пустота ID здесь не проверяется.
func checkV032(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v032LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := v032Argument(line, len(raw))
		if start < 0 {
			continue
		}
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		var forbidden []string
		seen := make(map[rune]bool)
		for _, char := range raw[start:end] {
			if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '.' || char == '-' || char == '_' || seen[char] {
				continue
			}
			seen[char] = true
			forbidden = append(forbidden, fmt.Sprintf("%q (U+%04X)", string(char), char))
		}
		if len(forbidden) > 0 {
			if _, err := findings.Add(number, start+1, number, end+1,
				"Недопустимые символы: "+strings.Join(forbidden, ", ")+"."); err != nil {
				return err
			}
		}
	}
	return nil
}

// v032Argument возвращает исходные границы аргумента в кодовых точках с 0,
// конец исключается. start=-1 означает отсутствие нужного распознанного тега.
// Скобка ограничивает только fragment и только при наличии элемента block-open.
func v032Argument(line diagnostics.Line, length int) (start, end int) {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() != "tag" {
			continue
		}
		name, _ := element.Value()
		switch name {
		case "task", "document-id", "fragment", "include":
		default:
			continue
		}
		start, end = element.End()-1, length
		if name == "fragment" {
			for j := i + 1; j < line.ElementCount(); j++ {
				next, _ := line.Element(j)
				if next.Type() == "block-open" {
					end = next.Start() - 1
					break
				}
			}
		}
		return start, end
	}
	return -1, length
}

// v032LiteralContent возвращает true для потомков блоков editor и multifill;
// false означает отсутствие такого литерального предка в готовой модели.
func v032LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "tag" {
				name, _ := element.Value()
				if name == "editor" || name == "multifill" {
					return true
				}
			}
		}
	}
}

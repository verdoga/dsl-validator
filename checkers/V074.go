package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV074 регистрирует проверку текста однострочного editor для DSL 1.2.
func registrationV074(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V074",
		Severity: diagnostics.SeverityError,
		Message:  "У @editor отсутствует текст комментария.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV074,
	})
}

// checkV074 проверяет весь исходный остаток после имени, включая unparsed.
// Любой символ вне White_Space исключает срабатывание. В частности, открывающая
// скобка исключает блочную форму без разбора её содержимого и поиска закрытия.
func checkV074(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v074LiteralContent(lines, line) {
			continue
		}
		start, end := v074TagRange(line, "editor")
		if start == 0 {
			continue
		}
		hasText := false
		for _, char := range []rune(line.Raw())[end-1:] {
			if !unicode.Is(unicode.White_Space, char) {
				hasText = true
				break
			}
		}
		if hasText {
			continue
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v074TagRange возвращает исходный диапазон @имя в Unicode-колонках [start,end).
// Нулевой диапазон означает отсутствие объявления. Raw распознаётся только
// при отсутствии tag; перед именем допускаются только U+0020 и U+0009.
func v074TagRange(line diagnostics.Line, name string) (start, end int) {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		value, _ := element.Value()
		if value == name {
			return element.Start(), element.End()
		}
		return 0, 0
	}
	raw := []rune(line.Raw())
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	// Имена editor и multifill состоят только из однобайтовых ASCII-букв.
	end = start + 1 + len(name)
	if end > len(raw) || !strings.EqualFold(string(raw[start:end]), "@"+name) {
		return 0, 0
	}
	if end < len(raw) && raw[end] != '{' && raw[end] != '}' &&
		!unicode.Is(unicode.White_Space, raw[end]) {
		return 0, 0
	}
	return start + 1, end + 1
}

// v074LiteralContent возвращает true для потомков блочных editor или multifill.
// Другие фигурные и логические предки не исключают реальные комментарии.
func v074LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if start, _ := v074TagRange(line, "editor"); start != 0 {
			return true
		}
		if start, _ := v074TagRange(line, "multifill"); start != 0 {
			return true
		}
	}
	return false
}

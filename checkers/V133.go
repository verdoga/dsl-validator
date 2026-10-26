package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV133 регистрирует дополнительное содержимое include для DSL 1.2.
func registrationV133(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V133",
		Severity: diagnostics.SeverityError,
		Message:  "После идентификатора @include записано дополнительное содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV133,
	})
}

// checkV133 отмечает весь хвост после допустимого первого ID одним сообщением.
// Состояние и результаты других диагностик не используются и не изменяются.
func checkV133(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		name, tagEnd := v133Name(line, raw)
		if name != "include" || v133Literal(lines, line) {
			continue
		}
		start := v133IDEnd(line, raw, tagEnd)
		if start < 0 {
			continue
		}
		end := len(raw)
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if end > start && raw[end-1] == '{' {
			backslashes := 0
			for index := end - 2; index >= start && raw[index] == '\\'; index-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				end--
			}
		}
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v133Name возвращает начальное имя и исключающий конец в кодовых точках с нуля.
// Без tag имя заканчивается на U+0020/U+0009, { или конце строки.
// Пустое имя означает отсутствие объявления; экранирование не снимается.
func v133Name(line diagnostics.Line, raw []rune) (string, int) {
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		tag, _ := line.Element(index)
		if tag.Type() == "tag" && tag.Start() == start+1 {
			name, _ := tag.Value()
			return strings.ToLower(name), tag.End() - 1
		}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' {
		end++
	}
	return strings.ToLower(string(raw[start+1 : end])), end
}

// v133IDEnd возвращает исключающий конец допустимого первого ID с нуля либо -1.
// identifier задаёт исходный диапазон; неотделённый остаток unparsed продолжает ID.
// Без block-open скобка внутри первого аргумента недопустима, включая f{ Extra.
func v133IDEnd(line diagnostics.Line, raw []rune, tagEnd int) int {
	start := tagEnd
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	end, limit := start, len(raw)
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "identifier" && element.Start() == start+1 {
			end = element.End() - 1
		}
		if element.Type() == "block-open" && element.Start()-1 >= start && element.Start()-1 < limit {
			limit = element.Start() - 1
		}
	}
	if end > limit {
		return -1
	}
	for end < limit && raw[end] != ' ' && raw[end] != '\t' {
		end++
	}
	if end == start {
		return -1
	}
	for _, char := range raw[start:end] {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return -1
		}
	}
	return end
}

// v133Literal возвращает true внутри блочных editor/multifill и false вне них.
// Используется вся цепочка родителей готовой модели без восстановления дерева.
func v133Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _ := v133Name(line, []rune(line.Raw()))
		if name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

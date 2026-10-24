package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV094 регистрирует проверку наличия открытия блочной таблицы.
func registrationV094(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V094",
		Severity: diagnostics.SeverityError,
		Message:  "У @table отсутствует открывающая скобка блочной формы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV094,
	})
}

// checkV094 проверяет весь исходный остаток объявления, включая unparsed.
// Позиции имени заданы в Unicode-колонках с 1, правая граница исключается.
func checkV094(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, start, end := v094Name(line)
		if name != "table" || v094Literal(lines, line) || v094Opening(line.Raw(), end) {
			continue
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v094Name читает точное начальное имя после отступа U+0020/U+0009.
// Элемент tag используется только для полного исходного имени, не префикса.
// При его отсутствии сохраняется распознавание invalid/unparsed по Raw.
// Пустое имя и нули означают отсутствие начального объявления.
func v094Name(line diagnostics.Line) (string, int, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0, 0
	}
	end := start + 1
	for end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	name := string(raw[start+1 : end])
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 && element.End() == end+1 {
			if value, present := element.Value(); present && strings.EqualFold(name, value) {
				return value, start + 1, end + 1
			}
		}
	}
	return strings.ToLower(name), start + 1, end + 1
}

// v094Opening ищет неэкранированную { начиная с исходной колонки afterName.
// Обработка пар не меняет Raw и не выполняет повторного распознавания текста.
// true означает наличие открытия независимо от его оформления и парности.
func v094Opening(raw string, afterName int) bool {
	runes := []rune(raw)
	for index := afterName - 1; index < len(runes); index++ {
		if runes[index] == '\\' && index+1 < len(runes) &&
			(runes[index+1] == '\\' || runes[index+1] == '{' || runes[index+1] == '}') {
			index++
			continue
		}
		if runes[index] == '{' {
			return true
		}
	}
	return false
}

// v094Literal возвращает true только для потомков блочных editor/multifill.
// Готовые ParentLine задают контекст; ошибочное объявление предка может
// сохранять имя и открытие лишь в Raw. Однострочный комментарий блок не создаёт.
func v094Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		name, _, end := v094Name(line)
		if (name == "editor" || name == "multifill") && v094Opening(line.Raw(), end) {
			return true
		}
	}
	return false
}

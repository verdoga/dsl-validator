package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV128 регистрирует запрет содержимого перед открытием hint для DSL 1.2.
func registrationV128(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V128",
		Severity: diagnostics.SeverityError,
		Message:  "Между @hint и открывающей скобкой записано содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV128,
	})
}

// checkV128 отмечает один исходный промежуток на объявление, включая invalid.
// Края White_Space исключаются, внутренний текст не меняется. Колонки считаются
// в кодовых точках Unicode с 1, правая граница диапазона исключается.
func checkV128(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v128Literal(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		name, start := v128InitialTag(line, raw)
		if !strings.EqualFold(name, "hint") {
			continue
		}
		end := v128Opening(line, raw, start)
		if end < 0 {
			continue
		}
		for start < end && unicode.Is(unicode.White_Space, raw[start]) {
			start++
		}
		for end > start && unicode.Is(unicode.White_Space, raw[end-1]) {
			end--
		}
		if start == end {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1,
			"У @hint нет названия; текст подсказки размещается внутри блока"); err != nil {
			return err
		}
	}
	return nil
}

// v128InitialTag возвращает начальное имя без @ и исключённый конец с 0.
// Пустое имя означает отсутствие объявления. Отступ — только U+0020/U+0009;
// распознанный tag имеет приоритет перед выделением имени из Raw.
func v128InitialTag(line diagnostics.Line, raw []rune) (string, int) {
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			return strings.TrimPrefix(element.Raw(), "@"), element.End() - 1
		}
	}
	end := start + 1
	for end < len(raw) && raw[end] != '{' && raw[end] != '}' && !unicode.Is(unicode.White_Space, raw[end]) {
		end++
	}
	return string(raw[start+1 : end]), end
}

// v128Opening возвращает индекс открытия с 0 либо -1 при его отсутствии.
// Приоритет имеет block-open после имени; иначе ищется первая активная {.
// Пары \\, \{ и \} потребляются целиком; кавычки не скрывают скобку.
func v128Opening(line diagnostics.Line, raw []rune, tagEnd int) int {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" && element.Start()-1 >= tagEnd {
			return element.Start() - 1
		}
	}
	for index := tagEnd; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) &&
			(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index++
			continue
		}
		if raw[index] == '{' {
			return index
		}
	}
	return -1
}

// v128Literal возвращает true для потомков блочных editor/multifill,
// false — для остальных контекстов. Используется вся цепочка родителей.
func v128Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _ := v128InitialTag(line, []rune(line.Raw()))
		if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
			return true
		}
	}
	return false
}

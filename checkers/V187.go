package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV187 регистрирует лишнее содержимое перед открытием variants для DSL 1.2.
func registrationV187(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V187", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Между @variants и открывающей скобкой записано содержимое.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV187,
	})
}

// checkV187 отмечает исходный промежуток без краевых U+0020/U+0009.
// Текст не декодируется; колонки Unicode начинаются с 1, конец исключается.
func checkV187(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, start := v187Declaration(line)
		if name != "variants" || v187Literal(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		end := v187Opening(line, raw, start)
		if end < 0 {
			continue
		}
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		for _, char := range raw[start:end] {
			if !unicode.IsSpace(char) {
				if _, err := findings.Add(line.Number(), start+1, line.Number(), end+1, ""); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// v187Declaration возвращает имя начального объявления и индекс после него в рунах с 0.
// Без tag восстанавливаются только ошибочные строки; пустое имя означает отсутствие
// объявления. Границы восстановленного имени: конец строки, U+0020/U+0009, { и }.
func v187Declaration(line diagnostics.Line) (string, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	erroneous, hasTag := line.Type() == "invalid", false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			hasTag = true
			if element.Start() == start+1 {
				name, _ := element.Value()
				return strings.ToLower(name), element.End() - 1
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if hasTag || !erroneous {
		return "", 0
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	return strings.ToLower(string(raw[start+1 : end])), end
}

// v187Opening возвращает индекс открытия в рунах с 0 или -1 при неопределённости.
// block-open имеет приоритет; без него допустима ровно одна неэкранированная {.
func v187Opening(line diagnostics.Line, raw []rune, start int) int {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" && element.Start()-1 >= start {
			return element.Start() - 1
		}
	}
	opening := -1
	for index := start; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) &&
			(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index++
			continue
		}
		if raw[index] == '{' {
			if opening >= 0 {
				return -1
			}
			opening = index
		}
	}
	return opening
}

// v187Literal возвращает true внутри фигурных editor/multifill по цепочке родителей.
// false означает отсутствие такого литерального предка; дерево не восстанавливается.
func v187Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		number, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		name, _ := v187Declaration(line)
		if name == "editor" || name == "multifill" {
			return true
		}
	}
}

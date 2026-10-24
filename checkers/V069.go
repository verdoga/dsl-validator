package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV069 регистрирует отсутствие названия этапа в DSL 1.2.
func registrationV069(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V069",
		Severity: diagnostics.SeverityError,
		Message:  "У @step отсутствует название этапа.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV069,
	})
}

// checkV069 проверяет весь исходный хвост объявления, включая unparsed.
// Наличие задания и корректность непустого названия не влияют на результат.
func checkV069(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v069LiteralContent(lines, line) {
			continue
		}
		start, end, found := v069Tag(line, "step")
		if !found {
			continue
		}
		empty := true
		for _, char := range []rune(line.Raw())[end-1:] {
			if !unicode.IsSpace(char) {
				empty = false
				break
			}
		}
		if empty {
			if _, err := findings.Add(number, start, number, end, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v069Tag возвращает исходные Unicode-колонки имени: начало с 1, конец
// исключается. found=false означает отсутствие объявления указанного тега.
// Резервный поиск не зависит от типа строки или наличия unparsed.
func v069Tag(line diagnostics.Line, name string) (start, end int, found bool) {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			value, _ := element.Value()
			return element.Start(), element.End(), value == name
		}
	}
	raw := []rune(line.Raw())
	first := 0
	for first < len(raw) && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	// Все вызывающие стороны передают имена из ASCII.
	last := first + 1 + len(name)
	if last > len(raw) || strings.ToLower(string(raw[first:last])) != "@"+name {
		return 0, 0, false
	}
	if last < len(raw) && raw[last] != '{' && raw[last] != '}' && !unicode.IsSpace(raw[last]) {
		return 0, 0, false
	}
	return first + 1, last + 1, true
}

// v069LiteralContent возвращает true только при наличии фигурного предка
// editor или multifill; false означает, что объявление подлежит проверке.
// Границы блоков и родительские связи берутся из готовой модели.
func v069LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if _, _, found := v069Tag(line, "editor"); found {
			return true
		}
		if _, _, found := v069Tag(line, "multifill"); found {
			return true
		}
	}
}

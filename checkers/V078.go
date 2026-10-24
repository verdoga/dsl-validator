package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV078 регистрирует отсутствие типа медиаресурса в DSL 1.2.
func registrationV078(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V078",
		Severity: diagnostics.SeverityError,
		Message:  "У @media отсутствует тип ресурса.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV078,
	})
}

// checkV078 проверяет весь исходный остаток, включая неразобранный текст.
// Удаляются только крайние U+0020/U+0009; непустой ошибочный тип присутствует.
func checkV078(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v078LiteralContent(lines, line) {
			continue
		}
		start, end, found := v078Tag(line, "media")
		if !found {
			continue
		}
		remainder := string([]rune(line.Raw())[end-1:])
		if strings.Trim(remainder, " \t") != "" {
			continue
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v078Tag возвращает исходные Unicode-колонки имени с 1, конец исключается.
// found=false означает отсутствие объявления указанного тега. Резервный
// поиск не ограничен типом строки или наличием unparsed.
func v078Tag(line diagnostics.Line, name string) (start, end int, found bool) {
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
	// Вызывающие стороны передают только имена из ASCII.
	last := first + 1 + len(name)
	if last > len(raw) || strings.ToLower(string(raw[first:last])) != "@"+name {
		return 0, 0, false
	}
	if last < len(raw) && raw[last] != '{' && raw[last] != '}' && !unicode.IsSpace(raw[last]) {
		return 0, 0, false
	}
	return first + 1, last + 1, true
}

// v078LiteralContent возвращает true внутри блочных editor или multifill.
// false означает отсутствие литерального предка; другие блоки не исключаются.
// Границы и родительские связи берутся только из готовой модели.
func v078LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if _, _, found := v078Tag(line, "editor"); found {
			return true
		}
		if _, _, found := v078Tag(line, "multifill"); found {
			return true
		}
	}
}

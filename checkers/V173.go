package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV173 регистрирует запрет заголовков непосредственно в matching.
func registrationV173(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V173", Severity: diagnostics.SeverityError,
		Message:  "Заголовок не разрешён внутри @matching.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV173,
	})
}

// checkV173 проверяет исходный начальный маркер, в том числе в invalid/unparsed.
// Отсутствие текста или правильного разделителя не отменяет запрета размещения.
// Диапазоны включают текст без краевых U+0020/U+0009; конец исключается.
func checkV173(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v173MatchingParent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		markerEnd := start
		for markerEnd < len(raw) && raw[markerEnd] == '#' {
			markerEnd++
		}
		if markerEnd-start < 1 || markerEnd-start > 3 {
			continue
		}
		end := len(raw)
		for end > markerEnd && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v173MatchingParent возвращает true только для непосредственного родителя
// block-start с тегом matching. Более дальние контейнеры не рассматриваются.
func v173MatchingParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	parent, found := line.ParentLine()
	if !found {
		return false
	}
	block, found := lines.Line(parent)
	if !found || block.Type() != "block-start" || line.NestingLevel() != block.NestingLevel()+1 {
		return false
	}
	for index := 0; index < block.ElementCount(); index++ {
		element, _ := block.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.EqualFold(name, "matching")
		}
	}
	return false
}

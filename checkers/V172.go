package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV172 регистрирует запрет посторонних тегов внутри matching DSL 1.2.
func registrationV172(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V172",
		Severity: diagnostics.SeverityError,
		Message:  "Тег не разрешён внутри @matching.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV172,
	})
}

// checkV172 отмечает непосредственные запрещённые tag, не заходя в их тела
// от имени внешнего matching. Наличие разделителя на проверку не влияет.
func checkV172(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v172Tag(line)
		if tag == nil || !v172MatchingParent(lines, line) {
			continue
		}
		name, _ := tag.Value()
		if strings.EqualFold(name, "answer") || strings.EqualFold(name, "hint") {
			continue
		}
		if _, err := findings.Add(number, tag.Start(), number, tag.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v172MatchingParent проверяет готовую связь с непосредственным блочным matching.
func v172MatchingParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	number, found := line.ParentLine()
	if !found {
		return false
	}
	parent, _ := lines.Line(number)
	if parent.Type() != "block-start" || line.NestingLevel() != parent.NestingLevel()+1 {
		return false
	}
	tag := v172Tag(parent)
	if tag == nil {
		return false
	}
	name, _ := tag.Value()
	return strings.EqualFold(name, "matching")
}

// v172Tag возвращает сохранённый начальный tag, включая строки с ошибочным
// хвостом. Известность Value гарантирует входной контракт; unparsed не разбирается.
func v172Tag(line diagnostics.Line) diagnostics.Element {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			return element
		}
	}
	return nil
}

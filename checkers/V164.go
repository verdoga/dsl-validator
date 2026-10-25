package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV164 регистрирует отсутствие элементов верхнего списка DSL 1.2.
func registrationV164(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V164",
		Severity: diagnostics.SeverityError,
		Message:  "В верхнем списке @matching нет элементов.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV164,
	})
}

// checkV164 проверяет только непосредственные строки блока с единственным
// исходным разделителем. Неопределённость нижней части и потомков не влияет.
func checkV164(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		tag := v164Matching(opening)
		if tag == nil {
			continue
		}
		rows := children[number]
		separator, separators := 0, 0
		for index, line := range rows {
			if strings.Trim(line.Raw(), " \t") == "---" {
				separator = index
				separators++
			}
		}
		if separators != 1 {
			continue
		}
		count, uncertain := 0, false
		for _, line := range rows[:separator] {
			item, unknown := v164UpperRow(line)
			uncertain = uncertain || unknown
			if item {
				count++
			}
		}
		if count == 0 && !uncertain {
			if _, err := findings.Add(number, tag.Start(), number, tag.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v164Matching возвращает сохранённый tag блочного matching либо nil.
// Не восстанавливает объявления по Raw; scope element требует готовый элемент.
func v164Matching(line diagnostics.Line) diagnostics.Element {
	if line.Type() != "block-start" {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			if strings.EqualFold(name, "matching") {
				return element
			}
		}
	}
	return nil
}

// v164UpperRow возвращает признак непустого обычного элемента и неопределённости.
// Invalid, unparsed и content без доступного значения подавляют проверку блока.
// Готовые значения content не нормализуются и повторно не разбираются.
func v164UpperRow(line diagnostics.Line) (item, uncertain bool) {
	if line.Type() == "invalid" {
		return false, true
	}
	hasValue, hasTag := false, false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "unparsed":
			return false, true
		case "tag":
			hasTag = true
		case "content":
			value, present := element.Value()
			hasValue = hasValue || present
			item = item || present && value != ""
		}
	}
	if line.Type() != "content" || hasTag {
		return false, false
	}
	return item, !hasValue
}

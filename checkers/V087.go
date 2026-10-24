package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// v087Position хранит исходный диапазон первого разделителя блока.
type v087Position struct{ line, start, end int }

// registrationV087 регистрирует проверку повторных разделителей для DSL 1.2.
func registrationV087(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V087",
		Severity: diagnostics.SeverityError,
		Message:  "В блоке повторён разделитель ---.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV087,
	})
}

// checkV087 учитывает только непосредственные строки каждого example и table.
// Классификация separator не обязательна: решение принимается по исходному Raw.
func checkV087(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first := make(map[int]v087Position)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		start, end := v087Separator(line.Raw())
		if end == 0 {
			continue
		}
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		block, _ := lines.Line(parent)
		if !v087Block(block) {
			continue
		}
		previous, found := first[parent]
		if !found {
			first[parent] = v087Position{number, start, end}
			continue
		}
		occurrence, err := findings.Add(number, start, number, end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
			return err
		}
	}
	return nil
}

// v087Block сообщает, является ли строка открытием блока example или table.
func v087Block(line diagnostics.Line) bool {
	if line.Type() != "block-start" {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		name, _ := element.Value()
		if element.Type() == "tag" && (name == "example" || name == "table") {
			return true
		}
	}
	return false
}

// v087Separator возвращает диапазон трёх дефисов в Unicode-колонках с 1,
// конец исключается. Нулевой диапазон означает отсутствие разделителя.
// Только крайние U+0020/U+0009 исключаются из сравнения; экранирование не снимается.
func v087Separator(raw string) (start, end int) {
	runes := []rune(raw)
	end = len(runes)
	for start < end && (runes[start] == ' ' || runes[start] == '\t') {
		start++
	}
	for end > start && (runes[end-1] == ' ' || runes[end-1] == '\t') {
		end--
	}
	if end-start != 3 || string(runes[start:end]) != "---" {
		return 0, 0
	}
	return start + 1, end + 1
}

package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV091 регистрирует рекомендацию о конечной точке списка в DSL 1.2.
func registrationV091(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V091",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Однострочный список слов не заканчивается точкой.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV091,
	})
}

// checkV091 проверяет весь исходный остаток распознанного однострочного
// wordlist. Удаляются только крайние U+0020/U+0009, без декодирования.
// Диапазон содержимого задан в Unicode-колонках с 1, конец исключается.
func checkV091(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "tag" {
			continue
		}
		tag := v091Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "wordlist" || v091LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := tag.End()-1, len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end || raw[end-1] == '.' {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v091Tag возвращает распознанное имя тега либо nil при его отсутствии.
// Объявление по Raw не восстанавливается.
func v091Tag(line diagnostics.Line) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return element
		}
	}
	return nil
}

// v091LiteralContent возвращает true для потомка блочного editor/multifill;
// false означает отсутствие такого контекста в готовой модели.
func v091LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if tag := v091Tag(line); tag != nil {
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
}

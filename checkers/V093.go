package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV093 регистрирует редакторскую рекомендацию только для DSL 1.2.
func registrationV093(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V093",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Строка блочного списка слов заканчивается знаком препинания.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV093,
	})
}

// checkV093 проверяет непосредственное текстовое содержимое каждого wordlist.
// Сокращения и цитаты не исключаются: их оценка остаётся за редактором.
func checkV093(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "content" || !v093WordlistParent(lines, line) {
			continue
		}
		content := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "content" {
				value, present := element.Value()
				content = content || present && strings.Trim(value, " \t") != ""
			}
		}
		if !content {
			continue
		}
		start, end := v093Suffix(line.Raw())
		if start == end {
			continue
		}
		if _, err := findings.Add(number, start, number, end,
			"Рассмотрите удаление конечной пунктуации, если она не является частью слова, сокращения или цитируемой фразы."); err != nil {
			return err
		}
	}
	return nil
}

// v093WordlistParent проверяет только непосредственного родителя строки.
func v093WordlistParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	number, found := line.ParentLine()
	if !found {
		return false
	}
	parent, found := lines.Line(number)
	if !found || parent.Type() != "block-start" {
		return false
	}
	for index := 0; index < parent.ElementCount(); index++ {
		element, _ := parent.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.EqualFold(name, "wordlist")
		}
	}
	return false
}

// v093Suffix возвращает исходные Unicode-колонки конечной пунктуации с единицы.
// Правая граница исключается; равные границы означают отсутствие пунктуации.
// Пробелы U+0020/U+0009 пропускаются, кавычки и скобки — нет.
func v093Suffix(raw string) (start, end int) {
	text := []rune(raw)
	last := len(text)
	for last > 0 && (text[last-1] == ' ' || text[last-1] == '\t') {
		last--
	}
	first := last
	for first > 0 && strings.ContainsRune(".,;:!?…", text[first-1]) {
		first--
	}
	return first + 1, last + 1
}

package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV034 регистрирует предупреждение о записи известного тега
// после другого содержимого физической строки только для DSL 1.2.
func registrationV034(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V034",
		Severity: diagnostics.SeverityWarning,
		Message:  "Запись с @ внутри строки остаётся обычным текстом.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV034,
	})
}

// checkV034 проверяет точный Raw content/title/name без снятия экранирования.
// Начало элемента разрешает левую границу; справа требуется физический конец
// строки либо U+0020/U+0009. Диапазон @имя задан в колонках с 1, конец исключается.
func checkV034(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v034Excluded(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		first := 0
		for first < len(raw) && v034Space(raw[first]) {
			first++
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "content" && element.Type() != "title" && element.Type() != "name" {
				continue
			}
			text := []rune(element.Raw())
			for i, char := range text {
				if char != '@' || i > 0 && !v034Space(text[i-1]) {
					continue
				}
				start := element.Start() - 1 + i
				if start <= first || raw[start-1] == '\\' {
					continue
				}
				end := i + 1
				for end < len(text) && (text[end] >= 'a' && text[end] <= 'z' || text[end] >= 'A' && text[end] <= 'Z' || text[end] == '-') {
					end++
				}
				if !v034KnownTag(strings.ToLower(string(text[i+1 : end]))) {
					continue
				}
				stop := element.Start() - 1 + end
				if stop < len(raw) && !v034Space(raw[stop]) {
					continue
				}
				if _, err := findings.Add(number, start+1, number, stop+1, "Для отдельного тега нужна новая строка"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v034Excluded возвращает true для комментария editor и потомков editor/multifill.
// Инструкция самой строки multifill не исключается. Родительские ссылки
// предоставляются согласованной моделью, допущенной штатным конвейером.
func v034Excluded(lines diagnostics.Lines, line diagnostics.Line) bool {
	if v034HasTag(line, "@editor") {
		return true
	}
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if v034HasTag(line, "@editor") || v034HasTag(line, "@multifill") {
			return true
		}
	}
}

// v034HasTag возвращает true, если строка содержит распознанный элемент tag
// с указанным именем без учёта регистра; текстовые упоминания дают false.
func v034HasTag(line diagnostics.Line, name string) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && strings.EqualFold(element.Raw(), name) {
			return true
		}
	}
	return false
}

// v034Space возвращает true только для двух разделителей V034: U+0020 и U+0009.
func v034Space(char rune) bool {
	return char == ' ' || char == '\t'
}

// v034KnownTag возвращает true для 34 имён тегов DSL 1.2 в нижнем регистре.
// Перечень взят из разделов 1–4 спецификации; неизвестное имя даёт false.
func v034KnownTag(name string) bool {
	switch name {
	case "dsl-version", "document-id", "section", "order", "resource-dir",
		"header", "task", "endtask", "step", "speaking", "newpage", "editor",
		"media", "example", "wordlist", "table", "script", "text", "key", "instr",
		"note", "alt", "hint", "fragment", "include", "answer", "question",
		"multifill", "choice", "multichoice", "matching", "ordering", "variants", "variant":
		return true
	default:
		return false
	}
}

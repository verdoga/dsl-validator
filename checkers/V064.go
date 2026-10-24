package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV064 регистрирует отсутствие ID у @task только для DSL 1.2.
func registrationV064(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V064",
		Severity: diagnostics.SeverityError,
		Message:  "У @task отсутствует ID.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV064,
	})
}

// checkV064 проверяет весь исходный остаток после task, включая unparsed.
// При отсутствии элемента tag распознаёт имя по Raw; identifier не обязателен.
func checkV064(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v064LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		if start < len(raw) && raw[start] == '\\' {
			continue
		}
		found := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			name, _ := element.Value()
			if element.Type() != "tag" || name != "task" {
				continue
			}
			found = true
			if err := v064ReportEmpty(findings, raw, number, element.Start(), element.End()); err != nil {
				return err
			}
		}
		if found {
			continue
		}
		if end, ok := v064RawTask(raw, start); ok {
			if err := v064ReportEmpty(findings, raw, number, start+1, end+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v064RawTask распознаёт @task после удаления крайних U+0020/U+0009.
// start уже пропускает начальный отступ; исходные индексы в рунах сохраняются.
// Возвращает исключающий конец имени и true либо 0 и false при отсутствии имени.
func v064RawTask(raw []rune, start int) (int, bool) {
	limit := len(raw)
	for limit > start && (raw[limit-1] == ' ' || raw[limit-1] == '\t') {
		limit--
	}
	end := start + len("@task")
	if end > limit || !strings.EqualFold(string(raw[start:end]), "@task") {
		return 0, false
	}
	if end < limit && raw[end] != '{' && raw[end] != '}' && !unicode.Is(unicode.White_Space, raw[end]) {
		return 0, false
	}
	return end, true
}

// v064ReportEmpty отмечает имя, если исходный остаток пуст или состоит из White_Space.
// start/end — исходные Unicode-колонки с 1, конец исключается.
func v064ReportEmpty(findings diagnostics.Findings, raw []rune, number, start, end int) error {
	for _, char := range raw[end-1:] {
		if !unicode.Is(unicode.White_Space, char) {
			return nil
		}
	}
	_, err := findings.Add(number, start, number, end, "")
	return err
}

// v064LiteralContent возвращает true для потомка блочного editor/multifill;
// false означает отсутствие такого предка в готовой структуре документа.
func v064LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

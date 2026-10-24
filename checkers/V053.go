package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV053 регистрирует отсутствие заголовка первого уровня в DSL 1.2.
func registrationV053(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V053",
		Severity: diagnostics.SeverityError,
		Message:  "В документе отсутствует заголовок первого уровня.",
		Scope:    diagnostics.ScopeDocument,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV053,
	})
}

// checkV053 ищет попытку объявления с ровно одним начальным # в Raw.
// Название, разделитель и допустимость вложения не проверяются.
// Пустой Lines не позволяет отличить пустой исходник от недоступных строк.
func checkV053(lines diagnostics.Lines, findings diagnostics.Findings) error {
	if lines.Len() == 0 {
		return nil
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v053LiteralContent(lines, line) {
			continue
		}
		raw := line.Raw()
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		end := start
		for end < len(raw) && raw[end] == '#' {
			end++
		}
		if end-start == 1 {
			return nil
		}
	}
	_, err := findings.Add(0, 0, 0, 0, "")
	return err
}

// v053LiteralContent возвращает true для потомка блочного editor или multifill;
// false означает отсутствие такого предка. Исходная разметка не разбирается.
func v053LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			tag, _ := line.Element(index)
			if tag.Type() != "tag" {
				continue
			}
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
}

package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV050 регистрирует ошибку заголовка выше третьего уровня в DSL 1.2.
func registrationV050(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V050",
		Severity: diagnostics.SeverityError,
		Message:  "В заголовке использовано более трёх знаков #.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV050,
	})
}

// checkV050 отмечает целиком начальный непрерывный маркер из четырёх и более #.
// Проверяется Raw без снятия экранирования; пропускаются только U+0020/U+0009.
// Колонки исходные, в кодовых точках Unicode с 1; правая граница исключается.
func checkV050(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v050LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		end := start
		for end < len(raw) && raw[end] == '#' {
			end++
		}
		if end-start <= 3 {
			continue
		}
		if _, err := findings.Add(line.Number(), start+1, line.Number(), end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v050LiteralContent возвращает true для потомка блочного editor или multifill;
// false означает отсутствие такого предка. Структура берётся из готовой модели.
func v050LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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

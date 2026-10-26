package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV129 регистрирует отсутствие идентификатора fragment только для DSL 1.2.
func registrationV129(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V129",
		Severity: diagnostics.SeverityError,
		Message:  "У @fragment отсутствует идентификатор.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV129,
	})
}

// checkV129 проверяет пустоту исходного промежутка, а не наличие identifier.
// Непустой ошибочный ID не является отсутствующим. Содержимое блока не учитывается.
func checkV129(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		start, end := v129Name(line, raw)
		if start < 0 || v129Literal(lines, line) {
			continue
		}
		limit := v129ArgumentEnd(line, raw, end)
		if strings.Trim(string(raw[end:limit]), " \t") != "" {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v129Name возвращает диапазон начального fragment в кодовых точках с нуля,
// конец исключается. start=-1 означает отсутствие объявления.
// Без tag допустимы только границы имени: конец строки, U+0020, U+0009 и {.
func v129Name(line diagnostics.Line, raw []rune) (start, end int) {
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return -1, 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		tag, _ := line.Element(index)
		if tag.Type() == "tag" && tag.Start() == start+1 {
			name, _ := tag.Value()
			if strings.EqualFold(name, "fragment") {
				return start, tag.End() - 1
			}
			return -1, 0
		}
	}
	end = start + len("@fragment")
	if end > len(raw) || !strings.EqualFold(string(raw[start:end]), "@fragment") {
		return -1, 0
	}
	if end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' {
		return -1, 0
	}
	return start, end
}

// v129ArgumentEnd возвращает исключающий конец исходного промежутка после имени.
// Приоритет имеет block-open; без него берётся первая неэкранированная { либо
// конец строки. Обратные косые черты учитываются слева направо без декодирования.
func v129ArgumentEnd(line diagnostics.Line, raw []rune, start int) int {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" && element.Start()-1 >= start {
			return element.Start() - 1
		}
	}
	for index := start; index < len(raw); index++ {
		if raw[index] == '\\' {
			index++
			continue
		}
		if raw[index] == '{' {
			return index
		}
	}
	return len(raw)
}

// v129Literal возвращает true для содержимого блочных editor и multifill;
// false означает отсутствие такого предка в готовой модели.
func v129Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
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
	return false
}

package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV003 регистрирует предупреждение о разделённых точках многоточия
// только для DSL 1.2.
func registrationV003(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V003",
		Severity: diagnostics.SeverityWarning,
		Message:  "Точки многоточия разделены пробельными символами.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV003,
	})
}

// checkV003 проверяет Raw всех строк, включая комментарии и вложенное содержимое.
// Технические диапазоны исключаются без соединения оставшихся участков.
// Порядок и непересечение элементов гарантированы штатным контролем модели.
func checkV003(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		start := 0
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if !v003Excluded(element.Type()) || element.Start() == element.End() {
				continue
			}
			if err := v003CheckSection(raw, start, element.Start()-1, number, findings); err != nil {
				return err
			}
			start = element.End() - 1
		}
		if err := v003CheckSection(raw, start, len(raw), number, findings); err != nil {
			return err
		}
	}
	return nil
}

// v003Excluded возвращает true только для исключённых V003 типов элементов.
func v003Excluded(kind string) bool {
	switch kind {
	case "tag", "identifier", "version", "number", "media-type", "source", "resource-path", "unparsed":
		return true
	default:
		return false
	}
}

// v003CheckSection проверяет участок [start,end) в исходных кодовых точках с 0.
// Максимальная цепочка содержит ровно три точки и хотя бы один внутренний
// промежуток из U+0009 или Zs. Внешние пробелы в срабатывание не входят.
// Findings получает Unicode-колонки с 1 и исключающую правую границу.
func v003CheckSection(raw []rune, start, end, number int, findings diagnostics.Findings) error {
	for index := start; index < end; {
		if raw[index] != '.' {
			index++
			continue
		}
		first, last, dots := index, index, 1
		spaced := false
		index++
		for index < end {
			char := raw[index]
			if char == '.' {
				spaced = spaced || index-last > 1
				last = index
				dots++
			} else if char != '\t' && !unicode.Is(unicode.Zs, char) {
				break
			}
			index++
		}
		if dots == 3 && spaced {
			if _, err := findings.Add(number, first+1, number, last+2, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

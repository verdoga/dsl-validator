package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV161 регистрирует запрет заголовков внутри блоков выбора.
func registrationV161(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V161",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри блока выбора записан заголовок.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV161,
	})
}

// checkV161 отмечает каждый непосредственный заголовок отдельно.
// Основной диапазон охватывает маркер, связанный — всю строку объявления
// контейнера. Колонки Unicode с 1, правая граница исключается.
func checkV161(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		start, end, found := v161HeadingMarker(line)
		if !found {
			continue
		}
		container, found := v161ChoiceParent(lines, line)
		if !found {
			continue
		}
		occurrence, err := findings.Add(number, start, number, end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, container.Number(), 1,
			container.Number(), utf8.RuneCountInString(container.Raw())+1); err != nil {
			return err
		}
	}
	return nil
}

// v161HeadingMarker возвращает исходный диапазон начального маркера 1–3.
// Raw не декодируется: экранированный # и часть текста не становятся маркером.
// Для invalid/unparsed без сохранённого heading-level обязателен разделитель.
func v161HeadingMarker(line diagnostics.Line) (start, end int, found bool) {
	raw := []rune(line.Raw())
	first := 0
	for first < len(raw) && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	last := first
	for last < len(raw) && raw[last] == '#' {
		last++
	}
	if last-first < 1 || last-first > 3 {
		return 0, 0, false
	}
	erroneous := line.Type() == "invalid"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "heading-level" && element.Start() == first+1 && element.End() == last+1 {
			return element.Start(), element.End(), true
		}
		if element.Type() == "unparsed" {
			erroneous = true
		}
	}
	if erroneous && last < len(raw) && unicode.Is(unicode.White_Space, raw[last]) {
		return first + 1, last + 1, true
	}
	return 0, 0, false
}

// v161ChoiceParent ищет только ближайший фигурный контейнер по готовой модели.
// Логические родители пропускаются; граница любого иного блока останавливает
// поиск, поэтому содержимое вложенных answer, hint и других блоков исключено.
func v161ChoiceParent(lines diagnostics.Lines, line diagnostics.Line) (diagnostics.Line, bool) {
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
			if strings.EqualFold(name, "choice") || strings.EqualFold(name, "multichoice") {
				return line, true
			}
		}
		return nil, false
	}
	return nil, false
}

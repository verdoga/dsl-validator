package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV031 регистрирует проверку регистра служебных обозначений
// только для DSL 1.2.
func registrationV031(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V031",
		Severity: diagnostics.SeverityWarning,
		Message:  "Служебное обозначение содержит прописные буквы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV031,
	})
}

// checkV031 сравнивает Raw распознанных обозначений с нижним регистром.
// Свободный текст не преобразуется. Диапазоны включают обозначение целиком,
// задаются исходными Unicode-колонками с 1 и исключают правую границу.
func checkV031(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		insideText, literal := v031Context(lines, line)
		if literal {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" && element.Type() != "media-type" {
				continue
			}
			if _, present := element.Value(); !present {
				continue
			}
			if element.Raw() != strings.ToLower(element.Raw()) {
				if _, err := findings.Add(number, element.Start(), number, element.End(), ""); err != nil {
					return err
				}
			}
		}
		if insideText {
			if err := v031HTML(line, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v031Context возвращает признаки предка text и литерального контейнера.
// editor/multifill исключают содержимое, но не собственное объявление:
// обход начинается с родителя текущей строки.
func v031Context(lines diagnostics.Lines, line diagnostics.Line) (insideText, literal bool) {
	for {
		parent, found := line.ParentLine()
		if !found {
			return insideText, false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			tag, _ := element.Value()
			switch tag {
			case "editor", "multifill":
				return false, true
			case "text":
				insideText = true
			}
		}
	}
}

// v031HTML проверяет точные поддерживаемые маркеры внутри каждого content.
// Парность HTML не проверяется; маркеры не пересекают границы элементов.
// Нечётное число предшествующих обратных косых черт экранирует открытие,
// поскольку каждая пара \\ потребляется до следующего символа.
func v031HTML(line diagnostics.Line, findings diagnostics.Findings) error {
	raw := []rune(line.Raw())
	standaloneBR := strings.ToLower(strings.Trim(line.Raw(), " \t")) == "<br>"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		for start := element.Start() - 1; start < element.End()-1; start++ {
			if raw[start] != '<' {
				continue
			}
			backslashes := 0
			for previous := start - 1; previous >= 0 && raw[previous] == '\\'; previous-- {
				backslashes++
			}
			if backslashes%2 != 0 {
				continue
			}
			for _, marker := range [...]string{"<strong>", "</strong>", "<em>", "</em>", "<u>", "</u>", "<br>"} {
				end := start + len(marker) // Маркеры состоят только из ASCII.
				if end > element.End()-1 || (marker == "<br>" && !standaloneBR) {
					continue
				}
				original := string(raw[start:end])
				if strings.ToLower(original) != marker {
					continue
				}
				if original != marker {
					if _, err := findings.Add(line.Number(), start+1, line.Number(), end+1, ""); err != nil {
						return err
					}
				}
				start = end - 1
				break
			}
		}
	}
	return nil
}

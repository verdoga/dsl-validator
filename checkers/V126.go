package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v126Rune связывает символ сравнения с исходной Unicode-колонкой, начиная с 1.
// У искусственной пробельной границы координаты нулевые: она не завершает фразу.
type v126Rune struct {
	char         rune
	line, column int
}

// registrationV126 регистрирует предупреждение о двух служебных фразах в alt.
func registrationV126(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V126",
		Severity: diagnostics.SeverityWarning,
		Message:  "В тифлокомментарии есть служебная фраза о его начале или конце.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV126,
	})
}

// checkV126 проверяет каждый alt независимо, используя готовые связи родителей.
// Вложенная конструкция разрывает последовательность собственного содержимого.
func checkV126(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], number)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if v126Tag(opening) != "alt" {
			continue
		}
		var text []v126Rune
		if opening.Type() != "block-start" {
			var err error
			text, err = v126Content(text, opening, findings)
			if err != nil {
				return err
			}
		} else {
			for _, child := range children[number] {
				line, _ := lines.Line(child)
				if (line.Type() == "content" || line.Type() == "invalid" || line.Type() == "blank") && v126Tag(line) == "" {
					var err error
					text, err = v126Content(text, line, findings)
					if err != nil {
						return err
					}
				} else {
					if err := v126Find(text, findings); err != nil {
						return err
					}
					text = nil
				}
			}
		}
		if err := v126Find(text, findings); err != nil {
			return err
		}
	}
	return nil
}

// v126Tag читает каноническое имя из модели, не восстанавливая объявления из Raw.
func v126Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v126Content добавляет исходные content, сохраняя пробельные промежутки между
// ними. Иные элементы и нетекстовые промежутки разрывают поиск. Конец строки
// добавляет пробельную границу; Value, экранирование и пунктуация не преобразуются.
func v126Content(text []v126Rune, line diagnostics.Line, findings diagnostics.Findings) ([]v126Rune, error) {
	raw := []rune(line.Raw())
	previousEnd := -1
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			if err := v126Find(text, findings); err != nil {
				return nil, err
			}
			text, previousEnd = nil, -1
			continue
		}
		start, end := element.Start()-1, element.End()-1
		if previousEnd >= 0 {
			spaceOnly := true
			for _, char := range raw[previousEnd:start] {
				spaceOnly = spaceOnly && unicode.IsSpace(char)
			}
			if spaceOnly {
				start = previousEnd
			} else {
				if err := v126Find(text, findings); err != nil {
					return nil, err
				}
				text = nil
			}
		}
		for column := start; column < end; column++ {
			text = v126Append(text, raw[column], line.Number(), column+1)
		}
		previousEnd = end
	}
	return v126Append(text, ' ', 0, 0), nil
}

// v126Append выполняет только приведение регистра и свёртку Unicode-пробелов.
func v126Append(text []v126Rune, char rune, line, column int) []v126Rune {
	if unicode.IsSpace(char) {
		char = ' '
		if len(text) != 0 && text[len(text)-1].char == ' ' {
			return text
		}
	}
	return append(text, v126Rune{unicode.ToLower(char), line, column})
}

// v126Find выдаёт отдельный исходный диапазон [start,end) для каждого вхождения.
// Только соседняя буква продолжает граничное слово; пунктуация не удаляется.
func v126Find(text []v126Rune, findings diagnostics.Findings) error {
	phrases := [...][]rune{[]rune("начало тифлокомментария"), []rune("конец тифлокомментария")}
	for start := range text {
		if start > 0 && unicode.IsLetter(text[start-1].char) {
			continue
		}
		for _, phrase := range phrases {
			end := start + len(phrase)
			if end > len(text) || end < len(text) && unicode.IsLetter(text[end].char) {
				continue
			}
			matches := true
			for offset, char := range phrase {
				if text[start+offset].char != char {
					matches = false
					break
				}
			}
			if matches {
				first, last := text[start], text[end-1]
				if _, err := findings.Add(first.line, first.column, last.line, last.column+1, ""); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

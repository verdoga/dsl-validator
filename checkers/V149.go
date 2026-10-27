package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV149 регистрирует проверку промежутков перед группами ANSWER.
func registrationV149(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V149",
		Severity: diagnostics.SeverityError,
		Message:  "Между пропуском и открывающей скобкой ответа есть пробельные символы.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV149,
	})
}

// checkV149 читает Raw непосредственного содержимого блочного multifill.
// Типы и границы текстовых элементов, включая unparsed, поиск не ограничивают.
func checkV149(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v149MultifillContent(lines, line) {
			continue
		}
		if err := v149CheckLine(line.Raw(), number, findings); err != nil {
			return err
		}
	}
	return nil
}

// v149MultifillContent возвращает true только для непосредственного тела блока.
// Сохранённое объявление родителя отличает блок от литерального псевдотега.
func v149MultifillContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	parent, found := line.ParentLine()
	if !found || line.Type() == "block-end" {
		return false
	}
	opening, _ := lines.Line(parent)
	if opening.Type() != "block-start" {
		return false
	}
	for index := 0; index < opening.ElementCount(); index++ {
		element, _ := opening.Element(index)
		if element.Type() == "tag" {
			name, present := element.Value()
			return present && strings.EqualFold(name, "multifill")
		}
	}
	return false
}

// v149InitialAnswer распознаёт запрещённый начальный answer без декодирования.
// По уточнению пользователя граница имени — только U+0020 или конец строки;
// начальные U+0020/U+0009 считаются отступом. false оставляет строку текстом.
func v149InitialAnswer(raw string) bool {
	name, _, _ := strings.Cut(strings.TrimLeft(raw, " \t"), " ")
	return strings.EqualFold(name, "@answer")
}

// v149CheckLine проверяет одну физическую строку, не перенося состояние дальше.
// Диапазоны отмечают исходный White_Space в Unicode-колонках [start,end) с 1.
func v149CheckLine(text string, number int, findings diagnostics.Findings) error {
	raw := []rune(text)
	checkFirstGap := !v149InitialAnswer(text)
	for index := 0; index < len(raw); {
		if raw[index] == '\\' && index+1 < len(raw) {
			switch raw[index+1] {
			case '\\', '{', '}':
				index += 2
				continue
			case '_':
				index += 2
				for index < len(raw) && raw[index] == '_' {
					index++
				}
				continue
			}
		}
		if raw[index] != '_' {
			index++
			continue
		}
		start := index
		for index < len(raw) && raw[index] == '_' {
			index++
		}
		if index-start != 5 {
			continue
		}
		gapStart := index
		for index < len(raw) && unicode.Is(unicode.White_Space, raw[index]) {
			index++
		}
		if index == len(raw) || raw[index] != '{' {
			continue
		}
		opening := index
		if checkFirstGap && opening > gapStart {
			if _, err := findings.Add(number, gapStart+1, number, opening+1,
				"Запишите ответ непосредственно после пропуска: _____{ANSWER}"); err != nil {
				return err
			}
		}
		var valid bool
		index, valid = v149GroupEnd(raw, opening)
		// Альтернативы продолжают только корректно начатую цепочку _____{…}.
		inChain := valid && opening == gapStart
		for index < len(raw) {
			gapStart, opening = index, index
			for opening < len(raw) && unicode.Is(unicode.White_Space, raw[opening]) {
				opening++
			}
			if opening == len(raw) || raw[opening] != '{' {
				break
			}
			index, valid = v149GroupEnd(raw, opening)
			inChain = inChain && valid
			if inChain && opening > gapStart {
				if _, err := findings.Add(number, gapStart+1, number, opening+1,
					"Запишите ответ непосредственно после пропуска: _____{ANSWER}"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v149GroupEnd возвращает позицию после первой неэкранированной } либо len(raw).
// valid истинен только для закрытой группы без вложенной неэкранированной {.
func v149GroupEnd(raw []rune, opening int) (end int, valid bool) {
	valid = true
	for index := opening + 1; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) {
			switch raw[index+1] {
			case '\\', '{', '}':
				index++
				continue
			}
		}
		switch raw[index] {
		case '{':
			valid = false
		case '}':
			return index + 1, valid
		}
	}
	return len(raw), false
}

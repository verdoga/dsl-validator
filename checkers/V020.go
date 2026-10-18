package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV020 регистрирует предупреждение о переносе слова для DSL 1.2.
func registrationV020(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V020",
		Severity: diagnostics.SeverityWarning,
		Message:  "Слово разделено дефисом и переносом строки.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV020,
	})
}

// checkV020 проверяет каждую границу отдельно. Между частями разрешены только
// blank того же непосредственного родителя. Форматирование text прозрачно,
// но диапазоны и исключённые элементы проверяются по исходному Raw. Unicode-
// колонки начинаются с 1, конец основного диапазона исключается.
// Две однобуквенные части неоднозначны и не дают предупреждения.
func checkV020(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "content" {
			continue
		}
		raw := []rune(line.Raw())
		first, positions := visibleText(raw, textFormatting(lines, line, raw), 0, len(raw))
		start, end, found := v020Tail(first)
		if !found || v020Excluded(line, positions[start], positions[end]+1) {
			continue
		}
		firstLetters := v020LetterCount(first[start:end])
		for next := number + 1; next <= lines.Len(); next++ {
			continuation, _ := lines.Line(next)
			if !v020SameParent(line, continuation) {
				break
			}
			if continuation.Type() == "blank" {
				continue
			}
			if continuation.Type() != "content" {
				break
			}
			raw := []rune(continuation.Raw())
			second, secondPositions := visibleText(raw, textFormatting(lines, continuation, raw), 0, len(raw))
			secondStart, secondEnd, found := v020Head(second)
			if found && firstLetters+v020LetterCount(second[secondStart:secondEnd]) >= 3 &&
				!v020Excluded(continuation, secondPositions[secondStart], secondPositions[secondEnd-1]+1) &&
				v020SameScript(first[start], second[secondStart]) {
				if _, err := findings.Add(line.Number(), positions[start]+1,
					continuation.Number(), secondPositions[secondEnd-1]+2, ""); err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

// v020Tail возвращает полуоткрытый диапазон буквенной части перед конечным
// U+002D в индексах кодовых точек. found=true означает хотя бы одну букву;
// однобуквенный хвост буквенно-цифрового обозначения не подходит.
func v020Tail(raw []rune) (start, end int, found bool) {
	end = len(raw)
	for end > 0 && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	if end == 0 || raw[end-1] != '-' {
		return 0, 0, false
	}
	end--
	start = end
	for start > 0 && v020WordRune(raw[start-1]) {
		start--
	}
	// Диакритики допустимы только после первой буквы, а не вместо неё.
	for start < end && !unicode.IsLetter(raw[start]) {
		start++
	}
	letters := v020LetterCount(raw[start:end])
	return start, end, letters > 0 && (letters > 1 || start == 0 || !unicode.IsDigit(raw[start-1]))
}

// v020Head возвращает диапазон первого слова после крайних U+0020 и табуляций.
// found=true означает строчную первую букву. Однобуквенное начало
// буквенно-цифрового обозначения не подходит.
func v020Head(raw []rune) (start, end int, found bool) {
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || !unicode.IsLower(raw[start]) {
		return 0, 0, false
	}
	end = start
	for end < len(raw) && v020WordRune(raw[end]) {
		end++
	}
	return start, end, v020LetterCount(raw[start:end]) > 1 || end == len(raw) || !unicode.IsDigit(raw[end])
}

// v020WordRune допускает букву или комбинируемую диакритику Unicode.
func v020WordRune(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsMark(char) && unicode.Is(unicode.Diacritic, char)
}

// v020LetterCount считает только буквы, не увеличивая счётчик на диакритиках.
func v020LetterCount(chars []rune) int {
	count := 0
	for _, char := range chars {
		if unicode.IsLetter(char) {
			count++
		}
	}
	return count
}

// v020SameParent сравнивает непосредственных родителей; две корневые строки
// имеют одного отсутствующего родителя. Глубина не заменяет родительскую связь.
func v020SameParent(first, second diagnostics.Line) bool {
	left, leftFound := first.ParentLine()
	right, rightFound := second.ParentLine()
	return leftFound == rightFound && left == right
}

// v020SameScript сравнивает письменность только двух переданных первых букв.
// Регистр и остальные буквы частей не участвуют в этом сравнении.
func v020SameScript(first, second rune) bool {
	for _, script := range unicode.Scripts {
		if unicode.Is(script, first) && unicode.Is(script, second) {
			return true
		}
	}
	return false
}

// v020Excluded проверяет пересечение полуоткрытого диапазона в индексах
// кодовых точек с исключёнными элементами. Обычный текст не классифицируется
// повторно как путь, число или неразобранная конструкция.
func v020Excluded(line diagnostics.Line, start, end int) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "source", "resource-path", "unparsed", "number":
			if start+1 < element.End() && element.Start() < end+1 {
				return true
			}
		}
	}
	return false
}

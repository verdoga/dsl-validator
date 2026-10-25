package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV121 регистрирует редакторский признак вопроса только для DSL 1.2.
func registrationV121(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V121",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "В инструкции есть вопросительный знак.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV121,
	})
}

const v121Detail = "Проверьте, требуется ли на этот вопрос отдельный ответ пользователя. Если требуется отдельное поле, оформите вопрос через @question. Вопросительная формулировка самой инструкции допустима"

// v121Rune сохраняет символ и его исходный диапазон Unicode-колонок с единицы.
// end исключается; диапазон экранированного символа включает обратную косую.
type v121Rune struct {
	char       rune
	start, end int
}

// checkV121 проверяет каждую собственную строку инструкции независимо.
func checkV121(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		start, end := v121Content(lines, line)
		if start == end {
			continue
		}
		text := v121Text([]rune(line.Raw()), start, end)
		if err := v121Questions(text, number, findings); err != nil {
			return err
		}
	}
	return nil
}

// v121Content возвращает исходные индексы [start,end) содержимого инструкции.
// Равные границы исключают строку. У блока проверяется только непосредственный
// текст; у однострочной формы CONTENT может содержать выделенные плейсхолдеры.
func v121Content(lines diagnostics.Lines, line diagnostics.Line) (start, end int) {
	if line.Type() == "content" {
		parent, found := line.ParentLine()
		if !found {
			return 0, 0
		}
		opening, _ := lines.Line(parent)
		if opening.Type() == "block-start" && v121Tag(opening) == "instr" {
			return 0, len([]rune(line.Raw()))
		}
		return 0, 0
	}
	if line.Type() != "tag" || v121Tag(line) != "instr" {
		return 0, 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" && element.Type() != "placeholder" {
			continue
		}
		if start == end {
			start = element.Start() - 1
		}
		end = max(end, element.End()-1)
	}
	return start, end
}

// v121Tag читает каноническое имя тега из модели, не распознавая Raw повторно.
func v121Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v121Text снимает только экранирование контекста instr слева направо.
// Пробелы сохраняются; начало физической строки после U+0020 и табуляций
// нужно лишь для экранирования @ и #. В однострочной форме там находится тег.
// Подчёркивания остаются текстом, повторного распознавания DSL здесь нет.
func v121Text(raw []rune, start, end int) []v121Rune {
	leading := 0
	for leading < len(raw) && (raw[leading] == ' ' || raw[leading] == '\t') {
		leading++
	}
	text := make([]v121Rune, 0, end-start)
	for index := start; index < end; index++ {
		column := index + 1
		if raw[index] == '\\' && index+1 < end {
			next := raw[index+1]
			atStart := index == leading && (next == '@' || next == '#')
			underscores := next == '_' && index+2 < end && raw[index+2] == '_'
			if next == '\\' || next == '{' || next == '}' || atStart || underscores {
				index++
			}
		}
		text = append(text, v121Rune{char: raw[index], start: column, end: index + 2})
	}
	return text
}

// v121Questions отмечает фрагмент с буквой перед каждой группой с вопросом.
// Любая группа .!? сбрасывает предшествующий текст, даже без срабатывания.
func v121Questions(text []v121Rune, number int, findings diagnostics.Findings) error {
	start, hasLetter := -1, false
	for index := 0; index < len(text); {
		char := text[index].char
		if !v121Terminal(char) {
			if start < 0 && !unicode.IsSpace(char) {
				start = index
			}
			hasLetter = hasLetter || unicode.IsLetter(char)
			index++
			continue
		}
		question := false
		for index < len(text) && v121Terminal(text[index].char) {
			question = question || text[index].char == '?'
			index++
		}
		if question && hasLetter {
			if _, err := findings.Add(number, text[start].start, number, text[index-1].end, v121Detail); err != nil {
				return err
			}
		}
		start, hasLetter = -1, false
	}
	return nil
}

// v121Terminal определяет только три знака завершения из алгоритма V121.
func v121Terminal(char rune) bool {
	return char == '.' || char == '!' || char == '?'
}

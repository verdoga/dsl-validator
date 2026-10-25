package checkers

import (
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV127 регистрирует редакторскую проверку букв alt только для DSL 1.2.
func registrationV127(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V127",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "В тифлокомментарии есть буквы вне русского алфавита.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV127,
	})
}

const v127Detail = "Редактору необходимо проверить и проанализировать, не является ли комментарий подсказкой или готовым ответом к упражнению. Обычное описание допустимо; видимые надписи сохраняются на языке источника внутри русского описания. Не удаляйте учебно значимые признаки изображения только потому, что они нужны для ответа"

// v127Word хранит исходный диапазон слова; правая Unicode-колонка исключена.
type v127Word struct {
	line, start, end int
}

// v127Rune связывает символ текстового значения с исходной Unicode-колонкой.
type v127Rune struct {
	char   rune
	column int
}

// checkV127 выдаёт одну рекомендацию на alt и все подходящие вхождения слов.
// Название и вложенные конструкции не участвуют в анализе собственного текста.
func checkV127(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if (opening.Type() != "tag" && opening.Type() != "block-start") || !v127Alt(opening) {
			continue
		}
		last := opening
		var words []v127Word
		if opening.Type() == "tag" {
			words = v127Words(opening)
		} else {
			for next := number + 1; next <= lines.Len(); next++ {
				line, _ := lines.Line(next)
				if !v127Descendant(lines, line, number) {
					break
				}
				last = line
				parent, _ := line.ParentLine()
				if parent != number {
					continue
				}
				if line.Type() == "block-end" {
					break
				}
				if line.Type() == "content" {
					words = append(words, v127Words(line)...)
				}
			}
		}
		if len(words) == 0 {
			continue
		}
		occurrence, err := findings.Add(number, 1, last.Number(), utf8.RuneCountInString(last.Raw())+1, v127Detail)
		if err != nil {
			return err
		}
		for _, word := range words {
			if err := findings.AddRelated(occurrence, word.line, word.start, word.line, word.end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v127Alt проверяет каноническое имя распознанного tag, не разбирая Raw.
func v127Alt(line diagnostics.Line) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name == "alt"
		}
	}
	return false
}

// v127Descendant использует готовые родительские связи для границы всего блока.
// Для незакрытого блока диапазон ограничен его существующими потомками.
func v127Descendant(lines diagnostics.Lines, line diagnostics.Line, block int) bool {
	for {
		parent, found := line.ParentLine()
		if !found || parent < block {
			return false
		}
		if parent == block {
			return true
		}
		line, _ = lines.Line(parent)
	}
}

// v127Words находит в content слова по правилу Letter (Letter|Mark)*.
// Цифры и пунктуация разделяют слова; каждый исходный фрагмент обходится один раз.
func v127Words(line diagnostics.Line) []v127Word {
	raw := []rune(line.Raw())
	var words []v127Word
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		text := v127Text(raw, element.Start()-1, element.End()-1)
		for position := 0; position < len(text); {
			if !unicode.IsLetter(text[position].char) {
				position++
				continue
			}
			start, foreign := position, false
			for position < len(text) {
				char := text[position].char
				if unicode.IsLetter(char) {
					foreign = foreign || !(char >= 'А' && char <= 'Я' || char >= 'а' && char <= 'я' || char == 'Ё' || char == 'ё')
				} else if !unicode.IsMark(char) {
					break
				}
				position++
			}
			if foreign {
				words = append(words, v127Word{line.Number(), text[start].column, text[position-1].column + 1})
			}
		}
	}
	return words
}

// v127Text получает текстовое значение content с исходными координатами.
// Снимается только экранирование alt по пп. 1.21–1.22 и 4.3.2; пробелы,
// регистр и Unicode не нормализуются. Результат не разбирается как DSL повторно.
func v127Text(raw []rune, start, end int) []v127Rune {
	leading := 0
	for leading < len(raw) && (raw[leading] == ' ' || raw[leading] == '\t') {
		leading++
	}
	text := make([]v127Rune, 0, end-start)
	for index := start; index < end; index++ {
		if raw[index] == '\\' && index+1 < end {
			next := raw[index+1]
			atStart := index == leading && (next == '@' || next == '#')
			underscores := next == '_' && index+2 < end && raw[index+2] == '_'
			if next == '\\' || next == '{' || next == '}' || atStart || underscores {
				index++
			}
		}
		text = append(text, v127Rune{raw[index], index + 1})
	}
	return text
}

package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v021MarkStressLists подтверждает немаркированный список слов соседним key:
// число и порядок строк совпадают, в каждой строке ключа только добавлены
// апострофы ударения перед буквами. Между блоками допустимы лишь blank того же
// родителя. Короткая фраза, строчный регистр или одинокий key недостаточны.
// Хотя бы одна добавленная метка должна стоять в начале слова: одни внутренние
// апострофы могут означать исправление сокращений (dont → don't).
func v021MarkStressLists(lines diagnostics.Lines, independent []bool) {
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if opening.Type() != "block-start" || v015Tag(opening) != "multifill" {
			continue
		}
		words, close := v021WordRows(lines, number)
		if len(words) < 2 || close == 0 {
			continue
		}
		parent, _ := opening.ParentLine()
		next := close + 1
		for next <= lines.Len() {
			line, _ := lines.Line(next)
			nextParent, _ := line.ParentLine()
			if nextParent != parent || line.Type() != "blank" {
				break
			}
			next++
		}
		key, found := lines.Line(next)
		if !found || key.Type() != "block-start" || v015Tag(key) != "key" {
			continue
		}
		keyParent, _ := key.ParentLine()
		if keyParent != parent {
			continue
		}
		answers, end := v021WordRows(lines, next)
		if end == 0 || len(words) != len(answers) {
			continue
		}
		matches, wordStart := true, false
		for i, word := range words {
			match, initial := v021StressOnly(word.text, answers[i].text)
			if !match {
				matches = false
				break
			}
			wordStart = wordStart || initial
		}
		if matches && wordStart {
			for i, word := range words {
				independent[word.number], independent[answers[i].number] = true, true
			}
		}
		number = close
	}
}

// v021WordRows читает только простые фразы непосредственного содержимого до
// закрытия opening. При вложениях, пунктуации или незакрытом блоке возвращает nil.
func v021WordRows(lines diagnostics.Lines, opening int) ([]v021Row, int) {
	var rows []v021Row
	tag := v021ContainerTag(lines, opening)
	for number := opening + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		if parent != opening {
			break
		}
		switch line.Type() {
		case "blank":
			continue
		case "block-end":
			return rows, number
		case "content":
			text, gap, labeled := v021Visible(line, tag)
			if !v021Phrase(text) {
				return nil, 0
			}
			rows = append(rows, v021Row{number, text, gap, labeled})
		default:
			return nil, 0
		}
	}
	return nil, 0
}

// v021StressOnly возвращает match=true, если key получен только добавлением
// ASCII-апострофов перед буквами; wordStart=true отмечает вставку в начале слова.
// При отсутствии вставок match=false. Изначальные апострофы не удаляются:
// сокращения и кавычки не превращаются в совпадающие слова нормализацией.
func v021StressOnly(word, key []rune) (match, wordStart bool) {
	i, added := 0, false
	for j, char := range key {
		if i < len(word) && word[i] == char {
			i++
			continue
		}
		if char != '\'' || j+1 == len(key) || !unicode.IsLetter(key[j+1]) {
			return false, false
		}
		added = true
		wordStart = wordStart || i == 0 || unicode.IsSpace(word[i-1])
	}
	return i == len(word) && added, wordStart
}

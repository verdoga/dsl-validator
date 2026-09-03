package handlers

import (
	"unicode/utf8"

	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/scan"
)

// placeholders находит ровно пятисимвольные неэкранированные последовательности подчёркиваний.
func placeholders(line model.Line) []model.Element {
	return placeholdersAt(line, 1)
}

// placeholdersAt ищет плейсхолдеры относительно заданного начального столбца фрагмента.
func placeholdersAt(line model.Line, base int) []model.Element {
	var result []model.Element
	for i := 0; i < len(line.Text); {
		if i+5 <= len(line.Text) && line.Text[i:i+5] == "_____" && !scan.Escaped(line.Text, i) && (i == 0 || line.Text[i-1] != '_') && (i+5 == len(line.Text) || line.Text[i+5] != '_') {
			start := utf8.RuneCountInString(line.Text[:i]) + base
			result = append(result, model.Element{Kind: elementField, Name: "placeholder", Raw: "_____", Value: "_____", Span: columnSpan(line.Number, start, start+5), Parsed: true})
			i += 5
			continue
		}
		_, size := utf8.DecodeRuneInString(line.Text[i:])
		i += size
	}
	return result
}

// runeCount возвращает количество Unicode-кодовых точек.
func runeCount(s string) int { return utf8.RuneCountInString(s) }

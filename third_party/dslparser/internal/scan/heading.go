package scan

import (
	"unicode"
	"unicode/utf8"
)

// ParseHeading разбирает уровень и текст заголовка относительно триммированной строки.
func ParseHeading(line string) HeadingResult {
	result := HeadingResult{LevelStart: 1, Parsed: true}
	i := 0
	for i < len(line) && line[i] == '#' {
		result.Level++
		i++
	}
	result.LevelEnd = result.Level + 1
	if result.Level == 0 || i >= len(line) {
		return malformedHeading(result, utf8.RuneCountInString(line)+1)
	}
	r, size := utf8.DecodeRuneInString(line[i:])
	if !unicode.IsSpace(r) {
		return malformedHeading(result, utf8.RuneCountInString(line)+1)
	}
	for i < len(line) {
		r, size = utf8.DecodeRuneInString(line[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	if i >= len(line) {
		return malformedHeading(result, utf8.RuneCountInString(line)+1)
	}
	result.Text = line[i:]
	result.TextStart = utf8.RuneCountInString(line[:i]) + 1
	result.TextEnd = utf8.RuneCountInString(line) + 1
	return result
}

// malformedHeading добавляет диагностику нарушенной формы заголовка.
func malformedHeading(result HeadingResult, end int) HeadingResult {
	result.Parsed = false
	result.Diagnostics = []Diagnostic{{Code: "MALFORMED_HEADING", Message: "нарушена форма заголовка", Start: 1, End: end}}
	return result
}

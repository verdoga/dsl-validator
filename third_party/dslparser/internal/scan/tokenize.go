package scan

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokenize разделяет сырой хвост по Unicode-пробелам и двойным кавычкам.
func Tokenize(raw string, startColumn int) Tokens {
	result := Tokens{Raw: raw, Parsed: true}
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRuneInString(raw[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		start := i
		var value strings.Builder
		quoted := raw[i] == '"'
		if quoted {
			i++
		}
		closed := !quoted
		for i < len(raw) {
			r, size = utf8.DecodeRuneInString(raw[i:])
			if quoted && r == '"' && !Escaped(raw, i) {
				i += size
				closed = true
				break
			}
			if !quoted && unicode.IsSpace(r) {
				break
			}
			if r == '\\' && i+size < len(raw) {
				next, nextSize := utf8.DecodeRuneInString(raw[i+size:])
				if next == '"' || next == '\\' {
					value.WriteRune(next)
					i += size + nextSize
					continue
				}
			}
			value.WriteRune(r)
			i += size
		}
		token := Token{Raw: raw[start:i], Value: value.String(), Start: startColumn + utf8.RuneCountInString(raw[:start]), End: startColumn + utf8.RuneCountInString(raw[:i]), Parsed: closed}
		if !closed {
			d := Diagnostic{Code: "UNCLOSED_QUOTE", Message: "не закрыта двойная кавычка", Start: token.Start, End: token.End}
			token.Diagnostics = []Diagnostic{d}
			result.Diagnostics = append(result.Diagnostics, d)
			result.Parsed = false
		}
		result.Values = append(result.Values, token)
	}
	return result
}

// FreeText сохраняет сырой свободный хвост без интерпретации кавычек.
func FreeText(raw string) string { return raw }

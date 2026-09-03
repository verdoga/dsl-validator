package normalize

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line хранит нормализованную физическую строку и её номер.
type Line struct {
	Number int
	Text   string
}

// Input проверяет UTF-8, удаляет начальный BOM и возвращает триммированные физические строки.
func Input(data []byte) ([]Line, bool) {
	if !utf8.Valid(data) {
		return nil, false
	}
	s := string(data)
	if strings.HasPrefix(s, "\ufeff") {
		s = strings.TrimPrefix(s, "\ufeff")
	}
	parts := strings.Split(s, "\n")
	lines := make([]Line, len(parts))
	for i, part := range parts {
		part = strings.TrimSuffix(part, "\r")
		lines[i] = Line{Number: i + 1, Text: strings.TrimFunc(part, unicode.IsSpace)}
	}
	return lines, true
}

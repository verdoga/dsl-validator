package scan

import "strings"

// Escaped сообщает, экранирован ли байт с указанным начальным индексом: true означает нечётное число обратных косых черт, false — чётное.
func Escaped(s string, byteIndex int) bool {
	count := 0
	for i := byteIndex - 1; i >= 0 && s[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

// DecodeEscapes снимает ровно одну косую черту перед каждым разрешённым специальным символом.
func DecodeEscapes(raw, special string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) && strings.ContainsRune(special, rune(raw[i+1])) {
			i++
			b.WriteByte(raw[i])
			continue
		}
		b.WriteByte(raw[i])
	}
	return b.String()
}

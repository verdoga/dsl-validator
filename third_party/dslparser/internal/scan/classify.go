package scan

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Classify выполняет контекстную предварительную классификацию нормализованной строки.
func Classify(line string, mode Mode) Candidate {
	c := Candidate{Raw: line}
	if line == "" {
		c.Kind = Empty
		return c
	}
	if mode == Opaque {
		if line == "}" {
			c.Kind = BlockClose
		} else {
			c.Kind = OpaqueLine
		}
		return c
	}
	if line == "}" {
		c.Kind = BlockClose
		return c
	}
	if strings.HasPrefix(line, "}") {
		c.Kind = InvalidBoundary
		return c
	}
	if line[0] == '#' && !Escaped(line, 0) {
		c.Kind = Heading
		return c
	}
	if line[0] != '@' || Escaped(line, 0) {
		c.Kind = Text
		return c
	}
	nameEnd := 1
	for nameEnd < len(line) {
		r, size := utf8.DecodeRuneInString(line[nameEnd:])
		if unicode.IsSpace(r) || r == '{' || r == '}' {
			break
		}
		nameEnd += size
	}
	c.Name = line[1:nameEnd]
	c.CanonicalName = strings.ToLower(c.Name)
	c.NameStart = 2
	c.NameEnd = 2 + utf8.RuneCountInString(c.Name)
	c.Tail = strings.TrimFunc(line[nameEnd:], unicode.IsSpace)
	if c.CanonicalName == "dsl-version" {
		c.Kind = Version
		return c
	}
	trimmed := strings.TrimFunc(line, unicode.IsSpace)
	last := len(trimmed) - 1
	if last >= 0 && trimmed[last] == '{' && !Escaped(trimmed, last) {
		c.Kind = BlockOpen
	} else {
		c.Kind = Tag
	}
	return c
}

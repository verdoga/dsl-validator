package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV082 регистрирует проверку пустоты quoted-пути media для DSL 1.2.
func registrationV082(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V082",
		Severity: diagnostics.SeverityError,
		Message:  "Путь медиа в кавычках пуст или состоит только из пробелов.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV082,
	})
}

// checkV082 проверяет только закрытый источник, сохраняя его исходный диапазон.
func checkV082(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v082LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start := v082SourceStart(line, raw)
		if start < 0 {
			continue
		}
		value, end, closed := v082QuotedValue(raw, start)
		if !closed || strings.IndexFunc(value, func(char rune) bool {
			return !unicode.Is(unicode.White_Space, char)
		}) >= 0 {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v082SourceStart возвращает индекс открывающей кавычки в рунах с 0 либо -1.
// Распознанные элементы задают позиции; отсутствующие элементы восстанавливаются
// из остатка Raw по DSL 3.1.2 и 1.29, включая целиком неразобранное объявление.
func v082SourceStart(line diagnostics.Line, raw []rune) int {
	tagEnd, typeEnd, sourceStart := -1, -1, -1
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		value, _ := element.Value()
		switch element.Type() {
		case "tag":
			if value != "media" {
				return -1
			}
			tagEnd = element.End() - 1
		case "media-type":
			if value != "audio" && value != "video" && value != "image" {
				return -1
			}
			typeEnd = element.End() - 1
		case "source":
			sourceStart = element.Start() - 1
		}
	}
	if tagEnd < 0 {
		if line.Type() != "invalid" {
			return -1
		}
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		tagEnd = start + len("@media")
		if tagEnd >= len(raw) || !strings.EqualFold(string(raw[start:tagEnd]), "@media") || raw[tagEnd] != ' ' {
			return -1
		}
	}
	if typeEnd < 0 {
		if tagEnd >= len(raw) || raw[tagEnd] != ' ' {
			return -1
		}
		start := tagEnd
		for start < len(raw) && raw[start] == ' ' {
			start++
		}
		typeEnd = start
		for typeEnd < len(raw) && raw[typeEnd] != ' ' {
			typeEnd++
		}
		name := string(raw[start:typeEnd])
		if !strings.EqualFold(name, "audio") && !strings.EqualFold(name, "video") && !strings.EqualFold(name, "image") {
			return -1
		}
	}
	if sourceStart < 0 {
		if typeEnd >= len(raw) || raw[typeEnd] != ' ' {
			return -1
		}
		sourceStart = typeEnd
		for sourceStart < len(raw) && raw[sourceStart] == ' ' {
			sourceStart++
		}
	}
	if sourceStart < len(raw) && raw[sourceStart] == '"' {
		return sourceStart
	}
	return -1
}

// v082QuotedValue снимает экранирование по DSL 1.21–1.22 и 3.1.6 слева направо.
// end — исходный индекс после закрывающей кавычки в рунах с 0. closed=false
// означает отсутствие закрытия. Неизвестные пары и внутренние пробелы сохраняются.
func v082QuotedValue(raw []rune, start int) (value string, end int, closed bool) {
	var decoded strings.Builder
	for i := start + 1; i < len(raw); i++ {
		char := raw[i]
		if char == '"' {
			return decoded.String(), i + 1, true
		}
		if char == '\\' && i+1 < len(raw) {
			switch raw[i+1] {
			case '\\', '"', '{', '}':
				i++
				char = raw[i]
			}
		}
		decoded.WriteRune(char)
	}
	return decoded.String(), len(raw), false
}

// v082LiteralContent исключает потомков блочных editor и multifill.
func v082LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			value, _ := element.Value()
			if element.Type() == "tag" && (value == "editor" || value == "multifill") {
				return true
			}
		}
	}
	return false
}

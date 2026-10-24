package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV084 регистрирует лишний хвост после пути медиа только для DSL 1.2.
func registrationV084(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V084",
		Severity: diagnostics.SeverityError,
		Message:  "После закрывающей кавычки пути медиа есть лишнее содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV084,
	})
}

// checkV084 проверяет Raw каждой физической строки, включая unparsed.
// После первой закрывающей кавычки удаляются только крайние U+0020/U+0009.
// Диапазон остатка задан в исходных Unicode-колонках, конец исключается.
func checkV084(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := line.Raw()
		opening, found := v084Opening(raw)
		if !found || v084Literal(lines, line) {
			continue
		}
		closing, found := v084Closing(raw[opening+1:])
		if !found {
			continue
		}
		start, end := opening+1+closing+1, len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end {
			continue
		}
		startColumn := utf8.RuneCountInString(raw[:start]) + 1
		endColumn := utf8.RuneCountInString(raw[:end]) + 1
		if _, err := findings.Add(number, startColumn, number, endColumn, ""); err != nil {
			return err
		}
	}
	return nil
}

// v084Opening возвращает байтовую позицию начальной кавычки SOURCE.
// false означает отсутствие префикса @media TYPE с допустимым типом и
// кавычкой. Начальные U+0020/U+0009 разрешены; разделители полей — U+0020.
// Семантический source не используется: экранирование в Raw должно сохраниться.
func v084Opening(raw string) (int, bool) {
	prefix := strings.TrimLeft(raw, " \t")
	tag, rest, found := strings.Cut(prefix, " ")
	if !found || !strings.EqualFold(tag, "@media") {
		return 0, false
	}
	kind, rest, found := strings.Cut(strings.TrimLeft(rest, " "), " ")
	if !found || (!strings.EqualFold(kind, "audio") &&
		!strings.EqualFold(kind, "video") && !strings.EqualFold(kind, "image")) {
		return 0, false
	}
	rest = strings.TrimLeft(rest, " ")
	if rest == "" || rest[0] != '"' {
		return 0, false
	}
	return len(raw) - len(rest), true
}

// v084Closing возвращает байтовую позицию первой неэкранированной кавычки
// в остатке одной строки; false означает отсутствие закрытия.
// Пары обратных косых черт и косая черта с кавычкой потребляются целиком;
// символ после пары снова проверяется. Путь не декодируется и не обрезается.
func v084Closing(rest string) (int, bool) {
	for index := 0; index < len(rest); index++ {
		if rest[index] == '\\' && index+1 < len(rest) &&
			(rest[index+1] == '\\' || rest[index+1] == '"') {
			index++
			continue
		}
		if rest[index] == '"' {
			return index, true
		}
	}
	return 0, false
}

// v084Literal исключает потомков блочных editor и multifill на любой глубине.
// Сохранённый tag имеет приоритет; при неразобранном имени проверяется Raw
// с обычным разделителем DSL, без восстановления ошибочных разделителей.
func v084Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimLeft(line.Raw(), " \t"), " ")
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				value, _ := element.Value()
				name = "@" + value
				break
			}
		}
		if strings.EqualFold(name, "@editor") || strings.EqualFold(name, "@multifill") {
			return true
		}
	}
}

package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV083 регистрирует незакрытый путь медиа только для DSL 1.2.
func registrationV083(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V083",
		Severity: diagnostics.SeverityError,
		Message:  "Заключённый в кавычки путь медиа не закрыт до конца строки.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV083,
	})
}

// checkV083 независимо проверяет Raw каждой физической строки, включая unparsed.
// Диапазон охватывает только открывающую кавычку в исходных Unicode-колонках.
func checkV083(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		opening, found := v083Opening(line.Raw())
		if !found || v083Literal(lines, line) || v083Closed(line.Raw()[opening+1:]) {
			continue
		}
		column := utf8.RuneCountInString(line.Raw()[:opening]) + 1
		if _, err := findings.Add(number, column, number, column+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v083Opening возвращает байтовую позицию начальной кавычки SOURCE.
// false означает отсутствие префикса @media TYPE с допустимым типом и
// кавычкой. Начальные U+0020/U+0009 разрешены; разделители полей — U+0020.
// Семантический source не используется: экранирование в Raw должно сохраниться.
func v083Opening(raw string) (int, bool) {
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

// v083Closed ищет первую неэкранированную кавычку в остатке одной строки.
// Пары обратных косых черт и косая черта с кавычкой потребляются целиком;
// символ после пары снова проверяется. Путь не декодируется и не обрезается.
func v083Closed(rest string) bool {
	for index := 0; index < len(rest); index++ {
		if rest[index] == '\\' && index+1 < len(rest) &&
			(rest[index+1] == '\\' || rest[index+1] == '"') {
			index++
			continue
		}
		if rest[index] == '"' {
			return true
		}
	}
	return false
}

// v083Literal исключает потомков блочных editor и multifill на любой глубине.
// Сохранённый tag имеет приоритет; при неразобранном имени проверяется Raw
// с обычным разделителем DSL, без восстановления ошибочных разделителей.
func v083Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
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

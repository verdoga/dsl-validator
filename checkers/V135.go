package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV135 регистрирует лишнее содержимое перед открытием блока answer.
func registrationV135(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V135",
		Severity: diagnostics.SeverityError,
		Message:  "Между @answer и открывающей скобкой записано содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV135,
	})
}

// checkV135 отмечает один исходный участок между именем answer и открытием.
// Ошибочные объявления сохраняют применимость; одни разделители не отмечаются.
func checkV135(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, afterName := v135Name(line)
		if name != "answer" || v135Literal(lines, line) {
			continue
		}
		opening := v135Opening(line, afterName)
		if opening == 0 {
			continue
		}
		raw := []rune(line.Raw())
		start, end := afterName-1, opening-1
		for start < end && unicode.Is(unicode.White_Space, raw[start]) {
			start++
		}
		for end > start && unicode.Is(unicode.White_Space, raw[end-1]) {
			end--
		}
		if start == end {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v135Name читает точное начальное имя после отступа U+0020/U+0009.
// Элемент tag используется для полного имени; иначе работает Raw, в том числе
// для unparsed. Возвращает имя и исключённый конец в Unicode-колонках с 1;
// пустое имя и 0 означают отсутствие начального тега.
func v135Name(line diagnostics.Line) (string, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0
	}
	end := start + 1
	for end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	name := string(raw[start+1 : end])
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 && element.End() == end+1 {
			if value, present := element.Value(); present && strings.EqualFold(name, value) {
				return value, end + 1
			}
		}
	}
	return strings.ToLower(name), end + 1
}

// v135Opening возвращает исходную Unicode-колонку открытия после имени или 0.
// Приоритет имеет block-open; запасной поиск потребляет только пары \\, \{, \}.
// Кавычки не скрывают скобки; декодирование исходной строки не выполняется.
func v135Opening(line diagnostics.Line, afterName int) int {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" && element.Start() >= afterName {
			return element.Start()
		}
	}
	raw := []rune(line.Raw())
	for index := afterName - 1; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) &&
			(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index++
			continue
		}
		if raw[index] == '{' {
			return index + 1
		}
	}
	return 0
}

// v135Literal возвращает true для потомков блочных editor и multifill,
// включая ошибочные объявления предков с сохранившимися родительскими связями.
// false означает отсутствие такого контекста; запрещённый answer в multifill
// относится к отдельной проверке.
func v135Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		name, afterName := v135Name(line)
		if (name == "editor" || name == "multifill") && v135Opening(line, afterName) != 0 {
			return true
		}
	}
	return false
}

package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV107 регистрирует обязательное открытие четырёх блоков для DSL 1.2.
// Открытие table проверяет специализированная диагностика V094.
func registrationV107(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V107",
		Severity: diagnostics.SeverityError,
		Message:  "Тег записан без обязательного открытия блока.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV107,
	})
}

// checkV107 отмечает имя объявления без открытия в той же физической строке.
// Диапазоны заданы в Unicode-колонках с 1, правая граница исключается.
func checkV107(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v107LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		name, start, end := v107Declaration(line, raw)
		if name == "" || v107HasOpening(line, raw, end-1) {
			continue
		}
		detail := "Тег @" + name + " требует открытия { в строке объявления; закрывающая } должна находиться на отдельной строке после содержимого."
		if _, err := findings.Add(number, start, number, end, detail); err != nil {
			return err
		}
	}
	return nil
}

// v107Declaration возвращает каноническое имя и исходный диапазон тега.
// Пустое имя означает отсутствие проверяемого объявления. Raw используется
// только при отсутствии tag; отступом считаются только U+0020 и U+0009.
func v107Declaration(line diagnostics.Line, raw []rune) (string, int, int) {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		value, _ := element.Value()
		if name := v107BlockName(value); name != "" {
			return name, element.Start(), element.End()
		}
		return "", 0, 0
	}
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0, 0
	}
	end := start + 1
	for end < len(raw) && raw[end] != '{' && raw[end] != '}' && !unicode.Is(unicode.White_Space, raw[end]) {
		end++
	}
	name := v107BlockName(string(raw[start+1 : end]))
	if name == "" {
		return "", 0, 0
	}
	return name, start + 1, end + 1
}

// v107BlockName сопоставляет имя без учёта регистра четырём блочным тегам.
func v107BlockName(value string) string {
	for _, name := range []string{"script", "text", "key", "fragment"} {
		if strings.EqualFold(value, name) {
			return name
		}
	}
	return ""
}

// v107HasOpening ищет открытие после имени. Экранированные пары потребляются
// слева направо; кавычки не скрывают скобку. Закрытие блока здесь не проверяется.
func v107HasOpening(line diagnostics.Line, raw []rune, start int) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			return true
		}
	}
	for index := start; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) &&
			(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index++
			continue
		}
		if raw[index] == '{' {
			return true
		}
	}
	return false
}

// v107LiteralContent исключает потомков блочных editor и multifill по всей
// цепочке родителей. Остальные предки не запрещают проверку объявления.
func v107LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() != "tag" {
				continue
			}
			name, _ := element.Value()
			if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
				return true
			}
		}
	}
	return false
}

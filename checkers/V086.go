package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV086 регистрирует проверку обязательного значения контентного тега.
func registrationV086(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V086",
		Severity: diagnostics.SeverityError,
		Message:  "После контентного тега отсутствует обязательное значение.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV086,
	})
}

// checkV086 проверяет весь исходный остаток однострочного объявления, включая
// unparsed. Диапазон имени задан в Unicode-колонках с 1, конец исключается.
func checkV086(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-start" || v086LiteralContent(lines, line) {
			continue
		}
		block := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			block = block || element.Type() == "block-open"
		}
		if block {
			continue
		}
		name, start, end := v086Name(line)
		missing := ""
		switch name {
		case "example", "instr", "note", "alt", "hint":
			missing = "текст"
		case "wordlist":
			missing = "список слов"
		case "include":
			missing = "ID включения"
		default:
			continue
		}
		raw := []rune(line.Raw())
		empty := true
		for _, char := range raw[end-1:] {
			if !unicode.Is(unicode.White_Space, char) {
				empty = false
				break
			}
		}
		if empty {
			detail := string(raw[start-1:end-1]) + ": отсутствует " + missing + "."
			if _, err := findings.Add(number, start, number, end, detail); err != nil {
				return err
			}
		}
	}
	return nil
}

// v086Name предпочитает разобранный tag, иначе читает точное начальное имя.
// Пропускается только отступ U+0020/U+0009. Позиции исходные, с исключающим
// концом; пустое имя и нули означают отсутствие объявления.
func v086Name(line diagnostics.Line) (string, int, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] == '\\' {
		return "", 0, 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name, element.Start(), element.End()
		}
	}
	if raw[start] != '@' {
		return "", 0, 0
	}
	end := start + 1
	for end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	return strings.ToLower(string(raw[start+1 : end])), start + 1, end + 1
}

// v086LiteralContent возвращает true для потомка блочного editor/multifill;
// false означает отсутствие такого предка в готовой структуре документа.
func v086LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v086Name(line)
		if name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

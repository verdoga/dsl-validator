package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV143 регистрирует отсутствие текста вопроса только для DSL 1.2.
func registrationV143(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V143",
		Severity: diagnostics.SeverityError,
		Message:  "У @question отсутствует текст вопроса.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV143,
	})
}

// checkV143 проверяет весь исходный остаток, включая неразобранные фрагменты.
// Непустое ошибочное содержимое не считается отсутствующим; соседние ответы
// не влияют на результат. Срабатывание охватывает только исходное имя тега.
func checkV143(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, start, end := v143Name(line)
		if name != "question" || v143Block(line) || v143Literal(lines, line) {
			continue
		}
		empty := true
		for _, char := range []rune(line.Raw())[end-1:] {
			if !unicode.Is(unicode.White_Space, char) {
				empty = false
				break
			}
		}
		if empty {
			if _, err := findings.Add(number, start, number, end, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v143Name возвращает точное начальное имя после отступа U+0020/U+0009.
// Элемент tag принимается только для полного имени; при его отсутствии имя
// сохраняется в Raw, включая invalid/unparsed. Позиции — Unicode-колонки с 1,
// конец исключён. Пустое имя и нули означают отсутствие начального тега.
func v143Name(line diagnostics.Line) (string, int, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0, 0
	}
	end := start + 1
	for end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	name := strings.ToLower(string(raw[start+1 : end]))
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 && element.End() == end+1 {
			if value, present := element.Value(); present && value == name {
				return value, start + 1, end + 1
			}
		}
	}
	return name, start + 1, end + 1
}

// v143Block возвращает true для сохранённого моделью открытия блока.
// Неразобранная скобка в Raw и без этого исключения делает остаток непустым.
func v143Block(line diagnostics.Line) bool {
	if line.Type() == "block-start" {
		return true
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" {
			return true
		}
	}
	return false
}

// v143Literal возвращает true для потомков блочных editor/multifill, иначе false.
// Контекст определяется готовыми родительскими связями, а не отступами.
func v143Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if v143Block(line) {
			name, _, _ := v143Name(line)
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

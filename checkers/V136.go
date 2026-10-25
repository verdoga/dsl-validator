package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV136 регистрирует пустой однострочный ответ только для DSL 1.2.
// Конкретизация не предусмотрена.
func registrationV136(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V136",
		Severity: diagnostics.SeverityError,
		Message:  "У однострочного @answer отсутствует непустой ответ.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV136,
	})
}

// checkV136 проверяет полный хвост физической строки, в том числе unparsed
// и участки вне content. Ошибки размещения и повторения не исключают объявление.
func checkV136(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v136Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if !strings.EqualFold(name, "answer") || v136Block(line) || v136Excluded(lines, line) {
			continue
		}
		rest := []rune(line.Raw())[tag.End()-1:]
		if v136AnswerText(rest) != "" {
			continue
		}
		if _, err := findings.Add(number, tag.Start(), number, tag.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v136Tag возвращает сохранённый начальный tag; nil означает его отсутствие.
// Из unparsed имя не восстанавливается. Отступы не входят в исходный диапазон.
func v136Tag(line diagnostics.Line) diagnostics.Element {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			return element
		}
	}
	return nil
}

// v136Block возвращает true для сохранённого открытия блока, включая ошибочное
// объявление с block-open; false означает отсутствие такой структуры.
func v136Block(line diagnostics.Line) bool {
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

// v136Excluded возвращает true внутри editor или multifill на любой глубине,
// false — вне этих блоков. Читается готовая цепочка родителей без её изменения.
func v136Excluded(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if !v136Block(line) {
			continue
		}
		if tag := v136Tag(line); tag != nil {
			name, _ := tag.Value()
			if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
				return true
			}
		}
	}
	return false
}

// v136AnswerText снимает разрешённое экранирование исходного хвоста слева
// направо, затем сжимает Unicode White_Space и удаляет краевые пробелы.
// В служебном answer нет плейсхолдеров и HTML; @ и # здесь не начинают строку.
// Неизвестные escape-пары, пунктуация и регистр сохраняются. Результат не разбирается повторно.
func v136AnswerText(raw []rune) string {
	var decoded strings.Builder
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' {
				char = next
				index++
			}
		}
		decoded.WriteRune(char)
	}
	return strings.Join(strings.FieldsFunc(decoded.String(), func(char rune) bool {
		return unicode.Is(unicode.White_Space, char)
	}), " ")
}

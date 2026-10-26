package checkers

import (
	"strings"

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

// checkV136 проверяет доказанную пустоту хвоста физической строки.
// Ошибки размещения и повторения не исключают объявление.
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
		text, known := v136AnswerText(line, tag)
		if !known || answerPrimary(text) != "" {
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

// v136AnswerText собирает готовые content.Value() без повторного декодирования,
// сохраняя исходные пробельные промежутки. true означает известный хвост,
// в том числе отсутствующий; unparsed и значимые непокрытые участки дают false.
func v136AnswerText(line diagnostics.Line, tag diagnostics.Element) (string, bool) {
	raw := []rune(line.Raw())
	end := tag.End() - 1
	var text strings.Builder
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Start() < tag.End() {
			continue
		}
		value, present := element.Value()
		if element.Type() != "content" || !present {
			return "", false
		}
		if start := element.Start() - 1; start > end {
			gap := string(raw[end:start])
			if answerPrimary(gap) != "" {
				return "", false
			}
			text.WriteString(gap)
		}
		text.WriteString(value)
		// Нулевой content внутри предыдущего элемента не уменьшает покрытие.
		if element.End()-1 > end {
			end = element.End() - 1
		}
	}
	tail := string(raw[end:])
	if answerPrimary(tail) != "" {
		return "", false
	}
	text.WriteString(tail)
	return text.String(), true
}

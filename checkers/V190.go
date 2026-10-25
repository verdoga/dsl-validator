package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV190 регистрирует повтор названия ветви внутри variants.
func registrationV190(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V190", Severity: diagnostics.SeverityError,
		Message:  "Название ветви повторяется в одном @variants.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV190,
	})
}

// v190Name хранит название и его исходные Unicode-колонки [start,end).
type v190Name struct {
	text             string
	line, start, end int
}

func checkV190(lines diagnostics.Lines, findings diagnostics.Findings) error {
	firstNames := make(map[int][]v190Name)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "tag" && line.Type() != "invalid" {
			continue
		}
		container := v190Container(lines, line)
		if container == 0 {
			continue
		}
		name, known := v190ReadName(line)
		if !known || name.text == "" {
			continue
		}
		var first *v190Name
		for index := range firstNames[container] {
			candidate := &firstNames[container][index]
			if strings.EqualFold(candidate.text, name.text) {
				first = candidate
				break
			}
		}
		if first == nil {
			firstNames[container] = append(firstNames[container], name)
			continue
		}
		occurrence, err := findings.Add(name.line, name.start, name.line, name.end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, first.line, first.start, first.line, first.end); err != nil {
			return err
		}
	}
	return nil
}

// v190Container возвращает номер ближайшего фигурного variants либо 0.
// Логические родители не создают фигурной границы; другие блоки её создают.
func v190Container(lines diagnostics.Lines, line diagnostics.Line) int {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, tags := "", 0
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				name, _ = element.Value()
				tags++
			}
		}
		if tags != 0 {
			if tags == 1 && strings.EqualFold(name, "variants") {
				return parent
			}
			return 0
		}
		if _, ok := v190TagEnd(line.Raw(), "variants"); ok {
			return parent
		}
		return 0
	}
	return 0
}

// v190TagEnd распознаёт точное начальное имя без учёта регистра.
// Возвращает смещение после имени в кодовых точках; false означает другой текст.
func v190TagEnd(raw, name string) (int, bool) {
	runes := []rune(raw)
	start := 0
	for start < len(runes) && (runes[start] == ' ' || runes[start] == '\t') {
		start++
	}
	end := start + 1 + len(name)
	if end > len(runes) || !strings.EqualFold(string(runes[start:end]), "@"+name) {
		return 0, false
	}
	if end < len(runes) && runes[end] != ' ' && runes[end] != '\t' {
		return 0, false
	}
	return end, true
}

// v190ReadName принимает только полное, однозначно выделенное название.
// Готовое Value не декодируется повторно; отсутствие name допускает Raw.
func v190ReadName(line diagnostics.Line) (v190Name, bool) {
	start, ok := v190TagEnd(line.Raw(), "variant")
	if !ok {
		return v190Name{}, false
	}
	raw := []rune(line.Raw())
	end := len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	name := v190Name{line: line.Number(), start: start + 1, end: end + 1}
	var ready diagnostics.Element
	tags := 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "tag":
			tags++
			value, _ := element.Value()
			if tags > 1 || !strings.EqualFold(value, "variant") || element.End()-1 > start {
				return name, false
			}
		case "name":
			if ready != nil || element.Start()-1 > start || element.End()-1 < end {
				return name, false
			}
			ready = element
		default:
			return name, false
		}
	}
	if ready != nil {
		name.text, ok = ready.Value()
		return name, ok
	}
	name.text, ok = v190Unescape(raw[start:end])
	return name, ok
}

// v190Unescape снимает только экранирование скобок и обратной косой черты.
// NAME находится после тега: @, #, кавычки и подчёркивания здесь обычные символы.
// Неэкранированная скобка означает неоднозначную разметку и даёт false.
func v190Unescape(raw []rune) (string, bool) {
	var text strings.Builder
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if next == '\\' || next == '{' || next == '}' {
				text.WriteRune(next)
				index++
				continue
			}
		}
		if char == '{' || char == '}' {
			return "", false
		}
		text.WriteRune(char)
	}
	return text.String(), true
}

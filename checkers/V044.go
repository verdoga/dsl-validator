package checkers

import (
	"strconv"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV044 регистрирует ошибку пустого пути только для DSL 1.2.
func registrationV044(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V044",
		Severity: diagnostics.SeverityError,
		Message:  "В @resource-dir указан пустой путь.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV044,
	})
}

// checkV044 проверяет весь Raw после распознанного resource-dir, включая unparsed.
// Отмечает список без внешних U+0020/U+0009 либо тег, если список отсутствует.
// Unicode-колонки начинаются с 1, правые границы исключаются.
func checkV044(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v044LiteralContent(lines, line) {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			tag, _ := line.Element(i)
			name, _ := tag.Value()
			if tag.Type() != "tag" || !strings.EqualFold(name, "resource-dir") {
				continue
			}
			raw := []rune(line.Raw())
			start, end := v044Trim(raw, tag.End()-1, len(raw))
			partStart, partNumber, quoted := start, 1, false
			var empty []string
			for pos := start; pos <= end; pos++ {
				if pos < end && raw[pos] == '"' {
					quoted = !quoted
				}
				if pos == end || raw[pos] == ',' && !quoted {
					if v044EmptyPath(raw, partStart, pos) {
						empty = append(empty, strconv.Itoa(partNumber))
					}
					partStart, partNumber = pos+1, partNumber+1
				}
			}
			if len(empty) > 0 {
				startColumn, endColumn := start+1, end+1
				if start == end {
					startColumn, endColumn = tag.Start(), tag.End()
				}
				if _, err := findings.Add(number, startColumn, number, endColumn,
					"Пустые части списка: "+strings.Join(empty, ", ")+"."); err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

// v044EmptyPath возвращает true только для пустой части или корректно
// заключённого в кавычки пути без символов, кроме U+0020/U+0009.
// Незакрытая кавычка и любой остаток после закрытия дают false.
func v044EmptyPath(raw []rune, start, end int) bool {
	start, end = v044Trim(raw, start, end)
	if start == end {
		return true
	}
	if raw[start] != '"' {
		return false
	}
	closing := start + 1
	for closing < end && raw[closing] != '"' {
		closing++
	}
	if closing != end-1 {
		return false
	}
	start, end = v044Trim(raw, start+1, closing)
	return start == end
}

// v044Trim удаляет только крайние U+0020/U+0009 из диапазона [start,end)
// с индексами кодовых точек с 0, не изменяя исходный текст.
func v044Trim(raw []rune, start, end int) (int, int) {
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	return start, end
}

// v044LiteralContent возвращает true для потомков литеральных блоков editor
// и multifill; false означает отсутствие такого предка в готовой модели.
func v044LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "tag" {
				name, _ := element.Value()
				if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
					return true
				}
			}
		}
	}
}

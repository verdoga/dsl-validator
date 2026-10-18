package checkers

import (
	"fmt"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV016 регистрирует проверку специальных символов только для DSL 1.2.
func registrationV016(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V016",
		Severity: diagnostics.SeverityWarning,
		Message:  "В тексте присутствует запрещённый специальный символ.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV016,
	})
}

// checkV016 проверяет весь Raw, включая служебные поля и unparsed.
// Исключаются только разрешённые HTML-маркеры в content блока text.
// Каждый символ отмечается в исходных Unicode-колонках [column,column+1).
func checkV016(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		var excluded []bool
		if v016InsideText(lines, line) {
			excluded = v016HTMLMarkers(line, raw)
		}
		for index, char := range raw {
			switch char {
			case '^', '&', '*', '`', '~', '<', '>', '=':
				if len(excluded) != 0 && excluded[index] {
					continue
				}
				if _, err := findings.Add(number, index+1, number, index+2,
					fmt.Sprintf("Обнаружен символ %c U+%04X", char, char)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v016InsideText возвращает true, если среди предков есть блочный text.
// Корректность ссылок на предшествующие строки гарантирует контроль модели.
func v016InsideText(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				if value, present := element.Value(); present && value == "text" {
					return true
				}
			}
		}
	}
}

// v016HTMLMarkers отмечает исходные позиции разрешённых маркеров внутри
// отдельных content. Парность HTML не проверяется. Экранирование учитывается
// только при распознавании маркера и само по себе не исключает символы.
func v016HTMLMarkers(line diagnostics.Line, raw []rune) []bool {
	excluded := make([]bool, len(raw))
	standaloneBR := strings.ToLower(strings.Trim(line.Raw(), " \t")) == "<br>"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		for start := element.Start() - 1; start < element.End()-1; start++ {
			if raw[start] != '<' {
				continue
			}
			backslashes := 0
			for previous := start - 1; previous >= 0 && raw[previous] == '\\'; previous-- {
				backslashes++
			}
			if backslashes%2 != 0 {
				continue
			}
			for _, marker := range [...]string{"<strong>", "</strong>", "<em>", "</em>", "<u>", "</u>", "<br>"} {
				end := start + len(marker) // Все разрешённые маркеры состоят из ASCII.
				if end > element.End()-1 || (marker == "<br>" && !standaloneBR) {
					continue
				}
				if strings.ToLower(string(raw[start:end])) != marker {
					continue
				}
				for position := start; position < end; position++ {
					excluded[position] = true
				}
				start = end - 1
				break
			}
		}
	}
	return excluded
}

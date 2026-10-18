package checkers

import (
	"fmt"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV006 регистрирует ошибку построчной парности кавычек для DSL 1.2.
func registrationV006(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V006",
		Severity: diagnostics.SeverityError,
		Message:  "Двойная кавычка не имеет пары в строке.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV006,
	})
}

// checkV006 сопоставляет U+0022 в каждой Raw отдельно, не преобразуя исходник.
// Экранирование действует только внутри заключённого в кавычки SOURCE media.
// Диапазон отмечает один символ в Unicode-колонках с 1, конец исключается.
func checkV006(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		raw := []rune(line.Raw())
		sourceStart := v006QuotedSourceStart(lines, line, raw)
		unpaired, inSource := 0, false
		for i := 0; i < len(raw); i++ {
			if inSource && raw[i] == '\\' && i+1 < len(raw) && (raw[i+1] == '\\' || raw[i+1] == '"') {
				i++
				continue
			}
			if raw[i] != '"' {
				continue
			}
			if unpaired == 0 {
				unpaired = i + 1
			} else {
				unpaired = 0
			}
			// SOURCE начинается только в своей исходной позиции. Последующие
			// кавычки ошибочного хвоста объявления уже не включают этот режим.
			inSource = i == sourceStart
		}
		if unpaired != 0 {
			message := fmt.Sprintf("У кавычки в колонке %d нет пары в этой строке", unpaired)
			if _, err := findings.Add(number, unpaired, number, unpaired+1, message); err != nil {
				return err
			}
		}
	}
	return nil
}

// v006QuotedSourceStart возвращает индекс U+0022, начинающего SOURCE media,
// в исходных кодовых точках с 0; -1 означает отсутствие такого SOURCE.
// Для unparsed читает остаток объявления, не полагаясь на готовую парность.
func v006QuotedSourceStart(lines diagnostics.Lines, line diagnostics.Line, raw []rune) int {
	if v006LiteralContent(lines, line) {
		return -1
	}
	switch line.Type() {
	case "tag", "invalid", "block-start":
	default:
		return -1
	}
	tagEnd := -1
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() != "tag" {
			continue
		}
		name, _ := element.Value()
		if name != "media" {
			return -1
		}
		tagEnd = element.End() - 1
		break
	}
	if tagEnd == -1 {
		// Ошибочная строка может целиком состоять из unparsed. Сравнивается
		// только имя тега; Raw и исходные колонки не нормализуются.
		if line.Type() != "invalid" {
			return -1
		}
		start := 0
		for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		tagEnd = start + len("@media")
		if tagEnd > len(raw) || !strings.EqualFold(string(raw[start:tagEnd]), "@media") {
			return -1
		}
	}
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "source" {
			start := element.Start() - 1
			if start < len(raw) && raw[start] == '"' {
				return start
			}
			return -1
		}
	}
	// DSL 3.1.2 и 1.29: после имени идут TYPE и SOURCE, разделённые U+0020.
	// Здесь определяется только начало SOURCE, без проверки TYPE и пути.
	if tagEnd >= len(raw) || raw[tagEnd] != ' ' {
		return -1
	}
	i := tagEnd
	for i < len(raw) && raw[i] == ' ' {
		i++
	}
	for i < len(raw) && raw[i] != ' ' {
		i++
	}
	for i < len(raw) && raw[i] == ' ' {
		i++
	}
	if i < len(raw) && raw[i] == '"' {
		return i
	}
	return -1
}

// v006LiteralContent сообщает, принадлежит ли строка литеральному содержимому
// editor или multifill. true исключает распознавание media, false — не исключает.
// Использует готовые связи родителей и не восстанавливает дерево по Raw.
func v006LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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
				if name == "editor" || name == "multifill" {
					return true
				}
			}
		}
	}
}

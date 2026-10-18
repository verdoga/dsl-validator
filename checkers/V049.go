package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV049 регистрирует закрывающие скобки среди текста для DSL 1.2.
func registrationV049(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V049",
		Severity: diagnostics.SeverityError,
		Message:  "Неэкранированная закрывающая фигурная скобка находится в строке с другим содержимым.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV049,
	})
}

// checkV049 читает точный Raw, используя готовую структуру только для контекста.
// Каждое срабатывание охватывает одну исходную Unicode-колонку с единицы;
// правая граница исключающая. Парность блоков и корректность ответов не проверяются.
func checkV049(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "}" {
			continue
		}
		literal := v049LiteralContext(lines, line)
		text := []rune(line.Raw())
		if literal == "" {
			for index := 0; index < line.ElementCount(); index++ {
				element, _ := line.Element(index)
				name, _ := element.Value()
				if element.Type() == "tag" && name == "resource-dir" {
					// Уточнение задания: ошибки параметров и размещения не
					// отменяют буквальность остатка распознанного объявления.
					text = text[:element.End()-1]
					break
				}
			}
		}
		for index := 0; index < len(text); {
			if v049EscapePair(text, index) {
				index += 2
				continue
			}
			if literal == "multifill" {
				// DSL 4.3.2: обратная косая черта перед первым подчёркиванием
				// экранирует всю максимальную последовательность.
				if text[index] == '\\' && index+1 < len(text) && text[index+1] == '_' {
					index++
					for index < len(text) && text[index] == '_' {
						index++
					}
					continue
				}
				if text[index] == '_' {
					start := index
					for index < len(text) && text[index] == '_' {
						index++
					}
					if index-start == 5 && index < len(text) && text[index] == '{' {
						index = v049AnswerEnd(text, index+1)
					}
					continue
				}
			}
			if text[index] == '}' {
				if _, err := findings.Add(number, index+1, number, index+2,
					"Граница блока записывается отдельной строкой; для буквального символа используйте \\}"); err != nil {
					return err
				}
			}
			index++
		}
	}
	return nil
}

// v049LiteralContext возвращает имя ближайшего литерального блока-родителя
// либо пустую строку. Объявления из исходного текста не восстанавливаются.
func v049LiteralContext(lines diagnostics.Lines, line diagnostics.Line) string {
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
			if name == "editor" || name == "multifill" {
				return name
			}
		}
	}
	return ""
}

// v049EscapePair возвращает true только для пар \\, \{ и \} на позиции index.
func v049EscapePair(text []rune, index int) bool {
	if text[index] != '\\' || index+1 >= len(text) {
		return false
	}
	return text[index+1] == '\\' || text[index+1] == '{' || text[index+1] == '}'
}

// v049AnswerEnd возвращает индекс после первого неэкранированного закрытия
// ответа либо конец текущей строки. Вложенность скобок ответа не учитывается.
func v049AnswerEnd(text []rune, index int) int {
	for index < len(text) {
		if v049EscapePair(text, index) {
			index += 2
			continue
		}
		if text[index] == '}' {
			return index + 1
		}
		index++
	}
	return len(text)
}

package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV009 регистрирует предупреждение о нестандартных кавычках для DSL 1.2.
func registrationV009(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V009",
		Severity: diagnostics.SeverityWarning,
		Message:  "Использована кавычка нестандартного вида.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV009,
	})
}

// checkV009 отмечает каждый запрещённый Quotation_Mark в исходных текстовых
// элементах и ответах placeholder. Колонки считаются в кодовых точках с 1;
// конец диапазона исключается. Конкретизация и связанные диапазоны отсутствуют.
func checkV009(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name", "placeholder":
			default:
				continue
			}
			raw := []rune(element.Raw())
			start, end := 0, element.End()-element.Start()
			if element.Type() == "placeholder" {
				start, end = v009AnswerRange(raw)
			}
			// Поддерживаемые DSL HTML-маркеры без атрибутов не содержат
			// Quotation_Mark. Они не дают срабатываний; текст между ними,
			// включая экранированные литеральные конструкции, проверяется.
			for offset := start; offset < end; offset++ {
				char := raw[offset]
				if char == '"' || char == '\'' || char == '\u02BC' || !unicode.Is(unicode.Quotation_Mark, char) {
					continue
				}
				column := element.Start() + offset
				if _, err := findings.Add(number, column, number, column+1, ""); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v009AnswerRange возвращает полуоткрытый диапазон ANSWER в кодовых точках
// уже выделенного placeholder. При отсутствии полной формы _____{ANSWER}
// возвращает пустой диапазон. Экранирование учитывается только для поиска
// закрывающей скобки по DSL 4.3.8; исходные символы ответа не преобразуются.
func v009AnswerRange(raw []rune) (start, end int) {
	if !strings.HasPrefix(string(raw), "_____{") {
		return 0, 0
	}
	for index := 6; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) {
			switch raw[index+1] {
			case '\\', '{', '}':
				index++
				continue
			}
		}
		if raw[index] == '}' {
			return 6, index
		}
	}
	return 0, 0
}

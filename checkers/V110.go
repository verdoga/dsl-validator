package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV110 регистрирует дополнительный запрет пустых исходных строк
// в непосредственном содержимом text для DSL 1.2.
func registrationV110(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V110",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри @text есть пустые строки исходного текста.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV110,
	})
}

// checkV110 объединяет физически соседние пустые строки одного блока text.
// Пустота определяется только по Raw, а не по классификации blank или Value.
// Пересечение с V004 сохраняется согласно уточнению пользователя.
func checkV110(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first, last, owner := 0, 0, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent := v110EmptyTextParent(lines, line)
		if first != 0 && (parent == 0 || parent != owner) {
			if err := v110Report(lines, findings, first, last); err != nil {
				return err
			}
			first, last, owner = 0, 0, 0
		}
		if parent != 0 {
			if first == 0 {
				first, owner = number, parent
			}
			last = number
		}
	}
	return v110Report(lines, findings, first, last)
}

// v110EmptyTextParent возвращает номер непосредственного родителя text
// для пустой исходной строки; 0 означает, что строка не подлежит проверке.
// Объявления и закрытия блоков исключены, вложенность берётся из готовой модели.
func v110EmptyTextParent(lines diagnostics.Lines, line diagnostics.Line) int {
	if line.Type() == "block-start" || line.Type() == "block-end" || strings.Trim(line.Raw(), " \t") != "" {
		return 0
	}
	parent, found := line.ParentLine()
	if !found {
		return 0
	}
	block, _ := lines.Line(parent)
	if block.Type() != "block-start" {
		return 0
	}
	for index := 0; index < block.ElementCount(); index++ {
		element, _ := block.Element(index)
		if element.Type() == "tag" {
			if name, _ := element.Value(); name == "text" {
				return parent
			}
			return 0
		}
	}
	return 0
}

// v110Report отмечает последовательность вместе с переводами строк:
// от конца предшествующей строки до начала следующей, при EOF — до конца Raw
// последней доступной строки. Колонки Unicode с 1, конец диапазона исключён.
// first=0 означает отсутствие последовательности.
func v110Report(lines diagnostics.Lines, findings diagnostics.Findings, first, last int) error {
	if first == 0 {
		return nil
	}
	previous, _ := lines.Line(first - 1) // Родитель text гарантированно предшествует содержимому.
	endLine, endColumn := last+1, 1
	if endLine > lines.Len() {
		line, _ := lines.Line(last)
		endLine, endColumn = last, utf8.RuneCountInString(line.Raw())+1
	}
	_, err := findings.Add(first-1, utf8.RuneCountInString(previous.Raw())+1, endLine, endColumn,
		"Редактор решает, нужен ли дополнительный визуальный интервал: удалите пустые исходные строки либо замените каждый необходимый интервал отдельной строкой <br>. Абзацы и обычные интервалы между ними отображаются автоматически")
	return err
}

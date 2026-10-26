package checkers

import (
	"fmt"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV139 регистрирует ограничение строк блочного ответа в choice.
func registrationV139(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V139",
		Severity: diagnostics.SeverityError,
		Message:  "Блок @answer внутри @choice содержит несколько строк ответа.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV139,
	})
}

// checkV139 считает физические строки каждого ответа независимо от их значений.
// Диапазон охватывает исходные строки блока целиком, с колонками Unicode с 1
// и исключённым концом. Без закрытия берётся последний потомок по модели.
func checkV139(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if v139BlockName(opening) != "answer" || !v139InChoice(lines, opening) {
			continue
		}
		end := number
		var answers []int
		for next := number + 1; next <= lines.Len(); next++ {
			line, _ := lines.Line(next)
			if !v139Descendant(lines, line, number) {
				break
			}
			end = next
			parent, _ := line.ParentLine()
			if parent != number {
				continue
			}
			if line.Type() == "block-end" {
				break
			}
			if v139NonemptyText(line) {
				answers = append(answers, next)
			}
		}
		if len(answers) <= 1 {
			continue
		}
		last, _ := lines.Line(end)
		occurrence, err := findings.Add(number, 1, end, utf8.RuneCountInString(last.Raw())+1,
			fmt.Sprintf("Количество строк ответа: %d. Оставьте ровно одну строку ответа.", len(answers)))
		if err != nil {
			return err
		}
		for _, related := range answers[1:] {
			line, _ := lines.Line(related)
			if err := findings.AddRelated(occurrence, related, 1, related, utf8.RuneCountInString(line.Raw())+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v139BlockName возвращает каноническое имя распознанного блока либо пустую строку.
// Литеральный текст и неразобранные псевдотеги не становятся объявлениями.
func v139BlockName(line diagnostics.Line) string {
	if line.Type() != "block-start" {
		return ""
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v139InChoice возвращает true только при ближайшем фигурном родителе choice.
// Логические родители пропускаются; иной фигурный блок прекращает поиск.
func v139InChoice(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			return v139BlockName(line) == "choice"
		}
	}
	return false
}

// v139Descendant возвращает true, если цепочка готовых родителей содержит block.
// Новая структура и парность скобок по исходному тексту не восстанавливаются.
func v139Descendant(lines diagnostics.Lines, line diagnostics.Line, block int) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		if parent == block {
			return true
		}
		if parent < block {
			return false
		}
		line, _ = lines.Line(parent)
	}
	return false
}

// v139NonemptyText возвращает true для текстовой строки с непустой основной формой content.
// Value уже содержит результат снятия экранирования; элементы не склеиваются
// и ответы не сравниваются. Пунктуационный content считается непустым.
func v139NonemptyText(line diagnostics.Line) bool {
	if line.Type() != "content" {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		text, _ := element.Value()
		if answerPrimary(text) != "" {
			return true
		}
	}
	return false
}

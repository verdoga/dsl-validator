package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV146 регистрирует ограничение последовательности ответов в DSL 1.2.
func registrationV146(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V146", Severity: diagnostics.SeverityError,
		Message:  "После одного @question подряд объявлено несколько @answer.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV146,
	})
}

// checkV146 проверяет непрерывные последовательности в структурных областях.
func checkV146(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		question, _ := lines.Line(number)
		if v146Tag(question, "question") == nil || !v146Context(lines, question) {
			continue
		}
		var first diagnostics.Line
		for next := number + 1; next <= lines.Len(); {
			answer := v146Following(lines, question, next)
			if answer == nil {
				break
			}
			if first == nil {
				first = answer
			} else if err := v146Report(findings, answer, first, question); err != nil {
				return err
			}
			next = answer.Number() + 1
			if answer.Type() == "block-start" {
				closing := v146Closing(lines, answer)
				if closing == 0 {
					break
				}
				next = closing + 1
			}
		}
	}
	return nil
}

// v146Tag возвращает распознанное имя объявления, а не упоминание в тексте.
func v146Tag(line diagnostics.Line, name string) diagnostics.Element {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		value, _ := element.Value()
		if element.Type() == "tag" && strings.EqualFold(value, name) {
			return element
		}
	}
	return nil
}

// v146Context допускает структурные области, включая начавшуюся ветвь variants.
// Любой другой фигурный предок исключает вопрос независимо от глубины.
func v146Context(lines diagnostics.Lines, question diagnostics.Line) bool {
	for line := question; ; {
		parent, found := line.ParentLine()
		if !found {
			return true
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if v146Tag(line, "variants") == nil {
			return false
		}
		branch := false
		for number := parent + 1; number < question.Number(); number++ {
			candidate, _ := lines.Line(number)
			container, _ := candidate.ParentLine()
			if container == parent && v146Tag(candidate, "variant") != nil {
				branch = true
			}
		}
		if !branch {
			return false
		}
	}
}

// v146Following пропускает только пустые строки и разрешённые editor целиком.
// Любая другая конструкция, смена родителя или глубины прекращает поиск.
func v146Following(lines diagnostics.Lines, question diagnostics.Line, number int) diagnostics.Line {
	parent, hasParent := question.ParentLine()
	for ; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		current, hasCurrent := line.ParentLine()
		if current != parent || hasCurrent != hasParent || line.NestingLevel() != question.NestingLevel() {
			return nil
		}
		if line.Type() == "blank" {
			continue
		}
		if end := v146EditorEnd(lines, line); end != 0 {
			number = end
			continue
		}
		if v146Tag(line, "answer") != nil {
			return line
		}
		return nil
	}
	return nil
}

// v146Closing находит собственную закрывающую границу, пропуская вложенные блоки.
// Ноль означает отсутствие закрытия до выхода из контейнера или конца документа.
func v146Closing(lines diagnostics.Lines, opening diagnostics.Line) int {
	for number := opening.Number() + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		if line.Type() == "block-end" && parent == opening.Number() {
			return number
		}
		for parent > opening.Number() {
			ancestor, _ := lines.Line(parent)
			parent, _ = ancestor.ParentLine()
		}
		if parent != opening.Number() {
			return 0
		}
	}
	return 0
}

// v146EditorEnd возвращает последнюю строку допустимого непустого комментария.
// Ноль означает значимую конструкцию; размещение проверяется контекстом вопроса.
func v146EditorEnd(lines diagnostics.Lines, line diagnostics.Line) int {
	if (line.Type() != "tag" && line.Type() != "block-start") || v146Tag(line, "editor") == nil {
		return 0
	}
	nonempty := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "tag", "block-open":
		case "content":
			if line.Type() == "block-start" {
				return 0
			}
			value, _ := element.Value()
			nonempty = nonempty || strings.Trim(value, " \t") != ""
		default:
			return 0
		}
	}
	if line.Type() == "tag" {
		if nonempty {
			return line.Number()
		}
		return 0
	}
	for number := line.Number() + 1; number <= lines.Len(); number++ {
		child, _ := lines.Line(number)
		parent, _ := child.ParentLine()
		if parent != line.Number() {
			return 0
		}
		if child.Type() == "block-end" {
			if nonempty {
				return number
			}
			return 0
		}
		if child.Type() != "content" && child.Type() != "blank" {
			return 0
		}
		nonempty = nonempty || strings.Trim(child.Raw(), " \t") != ""
	}
	return 0
}

// v146Report отмечает повторный ответ, затем связывает первый ответ и вопрос.
func v146Report(findings diagnostics.Findings, answer, first, question diagnostics.Line) error {
	tag := v146Tag(answer, "answer")
	occurrence, err := findings.Add(answer.Number(), tag.Start(), answer.Number(), tag.End(), "")
	if err != nil {
		return err
	}
	tag = v146Tag(first, "answer")
	if err := findings.AddRelated(occurrence, first.Number(), tag.Start(), first.Number(), tag.End()); err != nil {
		return err
	}
	tag = v146Tag(question, "question")
	return findings.AddRelated(occurrence, question.Number(), tag.Start(), question.Number(), tag.End())
}

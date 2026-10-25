package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV145 регистрирует проверку единственного препятствия перед answer.
func registrationV145(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V145",
		Severity: diagnostics.SeverityError,
		Message:  "Между @question и @answer расположено другое содержимое.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV145,
	})
}

// checkV145 рассматривает ровно одно препятствие, не устанавливая связи ответа.
func checkV145(lines diagnostics.Lines, findings diagnostics.Findings) error {
	branchStarts := v145BranchStarts(lines)
	for number := 1; number <= lines.Len(); number++ {
		answer, _ := lines.Line(number)
		if !v145Named(answer, "answer") || !v145Context(lines, answer, branchStarts) {
			continue
		}
		between := v145Previous(lines, answer, number)
		if between == nil || !v145Obstacle(between) {
			continue
		}
		question := v145Previous(lines, answer, between.Number())
		if question == nil || !v145SingleLine(question) || !v145Named(question, "question") {
			continue
		}
		tag := v145Element(answer, "tag")
		occurrence, err := findings.Add(number, tag.Start(), number, tag.End(), "")
		if err != nil {
			return err
		}
		for _, related := range []diagnostics.Line{question, between} {
			start, end := 1, utf8.RuneCountInString(related.Raw())+1
			if tag := v145Element(related, "tag"); tag != nil {
				start, end = tag.Start(), tag.End()
			}
			if err := findings.AddRelated(occurrence, related.Number(), start, related.Number(), end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v145Previous возвращает предыдущую значимую строку того же контекста либо nil.
// Разрешённый editor пропускается целиком; прочие границы блоков останавливают поиск.
func v145Previous(lines diagnostics.Lines, answer diagnostics.Line, before int) diagnostics.Line {
	parent, _ := answer.ParentLine()
	for number := before - 1; number > parent; number-- {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		if line.Type() == "block-end" {
			opening := v145BlockParent(lines, line)
			if opening == nil || !v145SameLevel(opening, answer) || !v145Named(opening, "editor") {
				return nil
			}
			number = opening.Number()
			continue
		}
		if !v145SameLevel(line, answer) || !v145SingleLine(line) {
			return nil
		}
		if v145Named(line, "editor") {
			continue
		}
		return line
	}
	return nil
}

// v145Obstacle допускает обычный текст либо однострочный неструктурный тег.
// false означает, что продолжать поиск вопроса через эту строку нельзя.
func v145Obstacle(line diagnostics.Line) bool {
	tag := v145Element(line, "tag")
	if tag == nil {
		return line.Type() == "content"
	}
	name, _ := tag.Value()
	switch strings.ToLower(name) {
	case "question", "answer", "multifill", "task", "endtask", "header", "step", "newpage", "variants", "variant":
		return false
	}
	return true
}

// v145SingleLine исключает заголовки и любые сохранённые фигурные границы.
// true означает отсутствие границы, а не подтверждение корректности объявления.
func v145SingleLine(line diagnostics.Line) bool {
	return line.Type() != "heading" && line.Type() != "block-start" && line.Type() != "block-end" &&
		v145Element(line, "block-open") == nil && v145Element(line, "block-close") == nil
}

// v145Context разрешает поток шага, задания, этапа и ветви вне иных блоков.
// false исключает также литеральные псевдотеги внутри editor и multifill.
func v145Context(lines diagnostics.Lines, line diagnostics.Line, branchStarts map[int]int) bool {
	for parent := v145BlockParent(lines, line); parent != nil; parent = v145BlockParent(lines, parent) {
		start := branchStarts[parent.Number()]
		if !v145Named(parent, "variants") || start == 0 || start >= line.Number() {
			return false
		}
	}
	return true
}

// v145BranchStarts за один проход сохраняет номер первого непосредственного
// variant каждого variants. Номер позволяет исключить материал до начала ветви
// без повторного просмотра её префикса для каждого answer; память — O(блоков).
// Это индекс контекста, а не поиск вопроса: предел двух конструкций не меняется.
func v145BranchStarts(lines diagnostics.Lines) map[int]int {
	starts := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v145Named(line, "variant") || !v145SingleLine(line) {
			continue
		}
		parent, found := line.ParentLine()
		if !found || starts[parent] != 0 {
			continue
		}
		block, _ := lines.Line(parent)
		if v145Named(block, "variants") && line.NestingLevel() == block.NestingLevel()+1 {
			starts[parent] = number
		}
	}
	return starts
}

// v145BlockParent находит ближайшего фигурного предка по готовым связям модели.
func v145BlockParent(lines diagnostics.Lines, line diagnostics.Line) diagnostics.Line {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" || v145Element(line, "block-open") != nil {
			return line
		}
	}
	return nil
}

// v145SameLevel возвращает true для одинаковых родителей и логических уровней.
func v145SameLevel(left, right diagnostics.Line) bool {
	leftParent, leftHasParent := left.ParentLine()
	rightParent, rightHasParent := right.ParentLine()
	return leftParent == rightParent && leftHasParent == rightHasParent && left.NestingLevel() == right.NestingLevel()
}

// v145Named сравнивает сохранённое имя без учёта регистра и без восстановления.
func v145Named(line diagnostics.Line, name string) bool {
	tag := v145Element(line, "tag")
	if tag == nil {
		return false
	}
	value, present := tag.Value()
	return present && strings.EqualFold(value, name)
}

// v145Element возвращает существующий элемент нужного вида либо nil.
func v145Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

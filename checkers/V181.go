package checkers

import (
	"strconv"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV181 регистрирует верхнюю границу номера в сокращённом ordering.
func registrationV181(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V181",
		Severity: diagnostics.SeverityError,
		Message:  "Номер позиции превышает количество элементов в @ordering.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV181,
	})
}

// checkV181 считает непосредственные элементы независимо от полноты ответов.
// Содержимое вложенных конструкций доступно только через их собственных детей.
func checkV181(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v181Tag(block) != "ordering" || v181LiteralParent(lines, block) {
			continue
		}
		count, answers, known := v181Items(children[number])
		if !known || count == 0 {
			continue
		}
		limit := strconv.Itoa(count)
		for _, answer := range answers {
			values := []diagnostics.Line{answer}
			if answer.Type() == "block-start" {
				values = children[answer.Number()]
			}
			if err := v181CheckAnswer(values, answer.Type() == "block-start", limit, findings); err != nil {
				return err
			}
		}
	}
	return nil
}

// v181Items возвращает N и связанные ответы; false означает неизвестный состав
// либо несокращённый формат. Пустые строки сохраняют соседство, другие — нет.
func v181Items(rows []diagnostics.Line) (int, []diagnostics.Line, bool) {
	count, pending, closed := 0, false, false
	var answers []diagnostics.Line
	for _, row := range rows {
		if closed {
			// Данные после собственного закрытия не задают однозначный список.
			return 0, nil, false
		}
		if row.Type() == "block-end" {
			closed = true
			continue
		}
		text := strings.Trim(row.Raw(), " \t")
		if text == "---" {
			return 0, nil, false
		}
		if text == "" || row.Type() == "blank" {
			continue
		}
		name := v181Tag(row)
		if name != "" {
			if pending && name == "answer" {
				answers = append(answers, row)
			}
			pending = false
			continue
		}
		if row.Type() == "block-start" || row.Type() == "heading" || v181HasElement(row, "heading-level") {
			pending = false
			continue
		}
		if row.Type() != "content" || v181HasElement(row, "unparsed") {
			return 0, nil, false
		}
		count++
		pending = true
	}
	return count, answers, true
}

// v181CheckAnswer объединяет повторы только в пределах одного объявления.
// Десятичные записи без ведущих нулей сравниваются без преобразования в int.
func v181CheckAnswer(rows []diagnostics.Line, block bool, limit string, findings diagnostics.Findings) error {
	occurrences := make(map[string]int)
	for _, row := range rows {
		if block && row.Type() != "content" {
			continue
		}
		number, start, end := v181Number(row)
		if number == "" || len(number) < len(limit) || len(number) == len(limit) && number <= limit {
			continue
		}
		if occurrence, exists := occurrences[number]; exists {
			if err := findings.AddRelated(occurrence, row.Number(), start, row.Number(), end); err != nil {
				return err
			}
			continue
		}
		occurrence, err := findings.Add(row.Number(), start, row.Number(), end, "Допустимый диапазон: 1…"+limit+".")
		if err != nil {
			return err
		}
		occurrences[number] = occurrence
	}
	return nil
}

// v181Number читает обработанное значение и исходный диапазон [start,end).
// Пустая строка результата означает, что answerPosition не распознал номер.
// Смежные фрагменты content составляют одно значение; промежутки между ними
// не удаляются ради получения числа. Экранирование повторно не обрабатывается.
func v181Number(line diagnostics.Line) (string, int, int) {
	var text strings.Builder
	start, end := 0, 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && start == 0 {
			continue
		}
		if element.Type() != "content" || start != 0 && element.Start() != end {
			return "", 0, 0
		}
		value, present := element.Value()
		if !present {
			return "", 0, 0
		}
		if start == 0 {
			start = element.Start()
		}
		end = element.End()
		text.WriteString(value)
	}
	number, valid := answerPosition(text.String())
	if !valid {
		return "", 0, 0
	}
	return number, start, end
}

// v181Tag возвращает единственное распознанное имя, не восстанавливая Raw.
func v181Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return ""
	}
	name := ""
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			if name != "" {
				return ""
			}
			name, _ = element.Value()
		}
	}
	return strings.ToLower(name)
}

// v181HasElement сообщает о наличии элемента заданного типа.
func v181HasElement(line diagnostics.Line, kind string) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return true
		}
	}
	return false
}

// v181LiteralParent исключает псевдообъявления в литеральных областях.
func v181LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if name := v181Tag(line); name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

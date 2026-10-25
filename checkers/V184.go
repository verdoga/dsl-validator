package checkers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV184 регистрирует проверку существования перестановки ordering.
func registrationV184(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V184",
		Severity: diagnostics.SeverityError,
		Message:  "Указанные ответы не позволяют разместить все элементы по разным позициям.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV184,
	})
}

// checkV184 самостоятельно проверяет все предпосылки до построения графа.
// Родительские связи берутся из модели; тексты элементов не объединяются.
func checkV184(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v184Tag(block) != "ordering" || v184LiteralParent(lines, block) {
			continue
		}
		rows, closing := v184Body(block, children[number])
		if closing == nil {
			continue
		}
		answers, known := v184Answers(rows)
		if !known || len(answers) < 2 {
			continue
		}
		graph := make([][]int, len(answers))
		for index, answer := range answers {
			graph[index], known = v184Positions(answer, children[answer.Number()], len(answers))
			if !known {
				break
			}
		}
		if !known {
			continue
		}
		matched := v184MaximumMatching(graph)
		if matched == len(graph) {
			continue
		}
		tag := v184Element(block, "tag")
		brace := v184Element(closing, "block-close")
		detail := fmt.Sprintf("Одновременно удаётся разместить %d из %d элементов.", matched, len(graph))
		if _, err := findings.Add(number, tag.Start(), closing.Number(), brace.End(), detail); err != nil {
			return err
		}
	}
	return nil
}

// v184Body отделяет собственное закрытие от непосредственного содержимого.
// nil closing означает отсутствие однозначных границ для полного блока.
func v184Body(opening diagnostics.Line, children []diagnostics.Line) (rows []diagnostics.Line, closing diagnostics.Line) {
	if v184Element(opening, "block-open") == nil || len(children) == 0 {
		return nil, nil
	}
	for index, child := range children {
		if child.Type() == "block-end" {
			if index != len(children)-1 || v184Element(child, "block-close") == nil {
				return nil, nil
			}
			return children[:index], child
		}
	}
	return nil, nil
}

// v184Answers возвращает по одному ответу для каждого отдельного элемента.
// false означает разделитель, неизвестный состав либо нарушение связи ответов.
func v184Answers(rows []diagnostics.Line) ([]diagnostics.Line, bool) {
	var answers []diagnostics.Line
	pending := false
	for _, row := range rows {
		text := strings.Trim(row.Raw(), " \t")
		if text == "---" {
			return nil, false
		}
		if text == "" || row.Type() == "blank" {
			continue
		}
		name := v184Tag(row)
		if name == "answer" {
			if !pending {
				return nil, false
			}
			answers = append(answers, row)
			pending = false
			continue
		}
		if pending {
			// Следующая значимая конструкция обязана быть ответом.
			return nil, false
		}
		if name != "" || row.Type() == "block-start" || row.Type() == "heading" || v184Element(row, "heading-level") != nil {
			continue
		}
		if row.Type() != "content" || v184HasUnparsed(row) {
			return nil, false
		}
		pending = true
	}
	return answers, !pending
}

// v184Positions возвращает непустое множество позиций с нулевой индексацией.
// Одно недопустимое значение отменяет весь набор, а не удаляется из него.
func v184Positions(answer diagnostics.Line, children []diagnostics.Line, count int) ([]int, bool) {
	rows := []diagnostics.Line{answer}
	block := answer.Type() == "block-start"
	if block {
		var closing diagnostics.Line
		rows, closing = v184Body(answer, children)
		if closing == nil {
			return nil, false
		}
	}
	var positions []int
	seen := make([]bool, count)
	limit := strconv.Itoa(count)
	for _, row := range rows {
		if block {
			if strings.Trim(row.Raw(), " \t") == "" || row.Type() == "blank" {
				continue
			}
			if row.Type() == "block-start" || v184Tag(row) != "" {
				continue
			}
			if row.Type() != "content" {
				return nil, false
			}
		}
		position := v184Number(row, limit)
		if position == 0 {
			return nil, false
		}
		if !seen[position-1] {
			seen[position-1] = true
			positions = append(positions, position-1)
		}
	}
	return positions, len(positions) != 0
}

// v184Number читает целое обработанное значение без нормализации и декодирования.
// Ноль означает неизвестное значение, неверную запись либо выход из 1…limit.
func v184Number(line diagnostics.Line, limit string) int {
	var text strings.Builder
	end := 0
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && end == 0 {
			continue
		}
		if element.Type() != "content" || end != 0 && element.Start() != end {
			return 0
		}
		value, present := element.Value()
		if !present {
			return 0
		}
		text.WriteString(value)
		end = element.End()
	}
	number := text.String()
	if number == "" || number[0] < '1' || number[0] > '9' {
		return 0
	}
	for index := 1; index < len(number); index++ {
		if number[index] < '0' || number[index] > '9' {
			return 0
		}
	}
	if len(number) > len(limit) || len(number) == len(limit) && number > limit {
		return 0
	}
	// Число уже не больше N, представимого int; переполнение исключено.
	position, err := strconv.Atoi(number)
	if err != nil {
		return 0
	}
	return position
}

// v184MaximumMatching находит максимум, сохраняя отдельные экземпляры элементов.
func v184MaximumMatching(graph [][]int) int {
	owners := make([]int, len(graph))
	for index := range owners {
		owners[index] = -1
	}
	matched := 0
	for item := range graph {
		if v184Augment(item, graph, owners, make([]bool, len(graph))) {
			matched++
		}
	}
	return matched
}

// v184Augment ищет увеличивающий путь; true означает увеличение паросочетания.
// Занятые позиции переназначаются только после успешного продолжения пути.
func v184Augment(item int, graph [][]int, owners []int, seen []bool) bool {
	for _, position := range graph[item] {
		if seen[position] {
			continue
		}
		seen[position] = true
		if owners[position] == -1 || v184Augment(owners[position], graph, owners, seen) {
			owners[position] = item
			return true
		}
	}
	return false
}

// v184Element возвращает единственный элемент типа; nil означает неоднозначность
// либо отсутствие. Координаты элемента имеют исключающую правую границу.
func v184Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}

// v184HasUnparsed обнаруживает любой неразобранный фрагмент строки.
func v184HasUnparsed(line diagnostics.Line) bool {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "unparsed" {
			return true
		}
	}
	return false
}

// v184Tag читает распознанное имя без восстановления объявления из Raw.
func v184Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return ""
	}
	if tag := v184Element(line, "tag"); tag != nil {
		name, _ := tag.Value()
		return strings.ToLower(name)
	}
	return ""
}

// v184LiteralParent исключает псевдообъявления в литеральных областях.
func v184LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if name := v184Tag(line); name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

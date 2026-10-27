package checkers

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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
		items, known := v184Answers(rows)
		if !known || len(items) < 2 {
			continue
		}
		graph := make([][]int, len(items))
		for index, item := range items {
			graph[index], known = v184Positions(item.answer, children[item.answer.Number()], len(items))
			if !known {
				break
			}
		}
		if !known {
			continue
		}
		matched, owners := v184MaximumMatching(graph)
		if matched == len(graph) {
			continue
		}
		tag := v184Element(block, "tag")
		brace := v184Element(closing, "block-close")
		group, positions := v184DeficientGroup(graph, owners)
		labels := make([]string, len(group))
		for index, item := range group {
			labels[index] = fmt.Sprintf("%d «%s»", item+1, strings.TrimSpace(items[item].line.Raw()))
		}
		detail := fmt.Sprintf("Одновременно удаётся разместить %d из %d элементов. Дефицитная группа: элементы %s; доступно позиций: %d, требуется: %d, не хватает: %d.", matched, len(graph), strings.Join(labels, ", "), positions, len(group), len(group)-positions)
		occurrence, err := findings.Add(number, tag.Start(), closing.Number(), brace.End(), detail)
		if err != nil {
			return err
		}
		for _, item := range group {
			// Исходный диапазон объявления ответа имеет исключающую правую границу.
			answer := items[item].answer
			last, end := answer, utf8.RuneCountInString(answer.Raw())+1
			if answer.Type() == "block-start" {
				_, last = v184Body(answer, children[answer.Number()])
				end = v184Element(last, "block-close").End()
			}
			if err := findings.AddRelated(occurrence, answer.Number(), v184Element(answer, "tag").Start(), last.Number(), end); err != nil {
				return err
			}
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

// v184Item сохраняет отдельный исходный экземпляр и принадлежащий ему ответ.
type v184Item struct {
	line, answer diagnostics.Line
}

// v184Answers возвращает по одному ответу для каждого отдельного элемента.
// false означает разделитель, неизвестный состав либо нарушение связи ответов.
func v184Answers(rows []diagnostics.Line) ([]v184Item, bool) {
	var answers []v184Item
	var pending diagnostics.Line
	for _, row := range rows {
		text := strings.Trim(row.Raw(), " \t")
		if text == "---" {
			return nil, false
		}
		if v184Blank(row) {
			continue
		}
		name := v184Tag(row)
		if name == "answer" {
			if pending == nil {
				return nil, false
			}
			answers = append(answers, v184Item{line: pending, answer: row})
			pending = nil
			continue
		}
		if pending != nil {
			// Следующая значимая конструкция обязана быть ответом.
			return nil, false
		}
		if name != "" || row.Type() == "block-start" || row.Type() == "heading" || v184Element(row, "heading-level") != nil {
			continue
		}
		if row.Type() != "content" || v184HasUnparsed(row) {
			return nil, false
		}
		pending = row
	}
	return answers, pending == nil
}

// v184Positions возвращает непустое множество позиций с нулевой индексацией.
// Одно недопустимое значение отменяет весь набор, а не удаляется из него.
func v184Positions(answer diagnostics.Line, children []diagnostics.Line, count int) ([]int, bool) {
	if answer.Type() != "tag" && answer.Type() != "block-start" || v184HasUnparsed(answer) {
		return nil, false
	}
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
			if v184Blank(row) {
				continue
			}
			if row.Type() == "invalid" || v184HasUnparsed(row) {
				return nil, false
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

// v184Number нормализует обработанное значение без повторного декодирования.
// Непокрытые тегом и известными content фрагменты допускают лишь пробелы Unicode.
// Ноль означает неизвестное значение, неверную запись либо выход из 1…limit.
func v184Number(line diagnostics.Line, limit string) int {
	var text strings.Builder
	raw := []rune(line.Raw())
	end := 1
	hasContent := false
	writeGap := func(start int) bool {
		if start < end {
			return false
		}
		gap := raw[end-1 : start-1]
		for _, char := range gap {
			if !unicode.IsSpace(char) {
				return false
			}
		}
		text.WriteString(string(gap))
		return true
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		start, next := element.Start(), element.End()
		if start < 1 || next < start || next > len(raw)+1 {
			return 0
		}
		if element.Type() == "tag" && !hasContent {
			if !writeGap(start) {
				return 0
			}
			end = next
			continue
		}
		if element.Type() != "content" {
			return 0
		}
		value, present := element.Value()
		if !present {
			return 0
		}
		// Пустая аннотация не занимает исходный текст и не меняет его покрытие.
		if start == next && value == "" {
			continue
		}
		if !writeGap(start) {
			return 0
		}
		text.WriteString(value)
		end = next
		hasContent = true
	}
	if !writeGap(len(raw) + 1) {
		return 0
	}
	number, known := answerPosition(text.String())
	if !known {
		return 0
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

// v184Blank отличает физическую пустую строку от пустого значения content.
func v184Blank(line diagnostics.Line) bool {
	return line.Type() == "blank" && strings.Trim(line.Raw(), " \t") == ""
}

// v184MaximumMatching находит максимум, сохраняя отдельные экземпляры элементов.
// owners содержит индекс элемента для позиции либо -1 для свободной позиции.
func v184MaximumMatching(graph [][]int) (matched int, owners []int) {
	owners = make([]int, len(graph))
	for index := range owners {
		owners[index] = -1
	}
	for item := range graph {
		if v184Augment(item, graph, owners, make([]bool, len(graph))) {
			matched++
		}
	}
	return matched, owners
}

// v184DeficientGroup обходит чередующиеся рёбра от несопоставленных элементов.
// Возвращает исходные индексы группы по возрастанию и число доступных позиций.
func v184DeficientGroup(graph [][]int, owners []int) ([]int, int) {
	matched := make([]bool, len(graph))
	for _, item := range owners {
		if item >= 0 {
			matched[item] = true
		}
	}
	seenItems, seenPositions := make([]bool, len(graph)), make([]bool, len(graph))
	var queue []int
	for item := range graph {
		if !matched[item] {
			seenItems[item] = true
			queue = append(queue, item)
		}
	}
	positions := 0
	for next := 0; next < len(queue); next++ {
		item := queue[next]
		for _, position := range graph[item] {
			if owners[position] == item || seenPositions[position] {
				continue
			}
			seenPositions[position] = true
			positions++
			if owner := owners[position]; owner >= 0 && !seenItems[owner] {
				seenItems[owner] = true
				queue = append(queue, owner)
			}
		}
	}
	var group []int
	for item, seen := range seenItems {
		if seen {
			group = append(group, item)
		}
	}
	return group, positions
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

package checkers

import (
	"strconv"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV199 регистрирует недостижимые разрешения только для DSL 1.2.
func registrationV199(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V199", Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeElement,
		Message:  "Указанная допустимая позиция недостижима для элемента в любом правильном полном порядке.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV199,
	})
}

// v199Value сохраняет основную форму и исходный диапазон [start,end).
type v199Value struct {
	text             string
	line, start, end int
}

type v199Item struct {
	label   string
	answers []v199Value
}

// checkV199 доказывает предпосылки независимо от результатов других диагностик.
func checkV199(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]diagnostics.Line)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], line)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		if opening.Type() != "block-start" || v199Tag(opening) != "ordering" {
			continue
		}
		rows, complete := v199Body(opening, children[number])
		if !complete {
			continue
		}
		items, complete := v199Items(rows, children)
		if !complete || len(items) < 2 {
			continue
		}
		graph, references, complete := v199Graph(items)
		if !complete || !v199Matching(graph, -1, -1) {
			continue
		}
		if err := v199Report(items, graph, references, findings); err != nil {
			return err
		}
	}
	return nil
}

// v199Body возвращает непосредственное содержимое доказанно закрытого блока.
// false означает отсутствие однозначной границы или несогласованную глубину.
func v199Body(opening diagnostics.Line, children []diagnostics.Line) ([]diagnostics.Line, bool) {
	if v199Element(opening, "block-open") == nil || len(children) == 0 {
		return nil, false
	}
	for index, child := range children {
		if child.NestingLevel() != opening.NestingLevel()+1 {
			return nil, false
		}
		if child.Type() == "block-end" {
			return children[:index], index == len(children)-1 && v199Element(child, "block-close") != nil
		}
	}
	return nil, false
}

// v199Items устанавливает единственный связанный ответ каждого экземпляра.
// false означает неполную настройку, расширенный формат или неизвестный состав.
func v199Items(rows []diagnostics.Line, children map[int][]diagnostics.Line) ([]v199Item, bool) {
	var items []v199Item
	pending, mayHint := false, false
	for _, row := range rows {
		if row.Type() == "blank" {
			continue
		}
		switch v199Tag(row) {
		case "answer":
			if !pending {
				return nil, false
			}
			answers, complete := v199Answers(row, children[row.Number()])
			if !complete {
				return nil, false
			}
			items[len(items)-1].answers = answers
			pending, mayHint = false, true
		case "hint":
			if pending || !mayHint || row.Type() != "tag" && row.Type() != "block-start" {
				return nil, false
			}
			if row.Type() == "block-start" {
				if _, complete := v199Body(row, children[row.Number()]); !complete {
					return nil, false
				}
			}
			mayHint = false
		case "":
			if pending || row.Type() != "content" {
				return nil, false
			}
			label, complete := v199Text(row)
			if !complete {
				return nil, false
			}
			items = append(items, v199Item{label: label.text})
			pending, mayHint = true, false
		default:
			return nil, false
		}
	}
	return items, !pending
}

// v199Answers читает одну альтернативу на строку, не объединяя строки блока.
// false означает пустое, неизвестное значение или неподходящий состав ответа.
func v199Answers(answer diagnostics.Line, children []diagnostics.Line) ([]v199Value, bool) {
	rows := []diagnostics.Line{answer}
	if answer.Type() == "block-start" {
		var complete bool
		rows, complete = v199Body(answer, children)
		if !complete || v199Element(answer, "content") != nil || v199Element(answer, "unparsed") != nil {
			return nil, false
		}
	} else if answer.Type() != "tag" {
		return nil, false
	}
	var answers []v199Value
	for _, row := range rows {
		if row.Type() == "blank" {
			continue
		}
		if answer.Type() == "block-start" && (row.Type() != "content" || v199Tag(row) != "") {
			return nil, false
		}
		value, complete := v199Text(row)
		if !complete {
			return nil, false
		}
		answers = append(answers, value)
	}
	return answers, len(answers) != 0
}

// v199Text читает цельный content по уточнению пользователя, без декодирования.
// false означает отсутствие единственного непустого основного значения.
func v199Text(line diagnostics.Line) (v199Value, bool) {
	var content diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			continue
		}
		if element.Type() != "content" || content != nil {
			return v199Value{}, false
		}
		content = element
	}
	if content == nil {
		return v199Value{}, false
	}
	value, present := content.Value()
	value = answerPrimary(value)
	return v199Value{value, line.Number(), content.Start(), content.End()}, present && value != ""
}

// v199Graph сравнивает каждую альтернативу с каждой позицией полным циклом.
// references сохраняет исходные значения для каждого ребра; false означает
// хотя бы одну альтернативу, не разрешившуюся ни в одну существующую позицию.
func v199Graph(items []v199Item) (graph [][]int, references [][][]v199Value, complete bool) {
	graph = make([][]int, len(items))
	references = make([][][]v199Value, len(items))
	for index, item := range items {
		references[index] = make([][]v199Value, len(items))
		for _, answer := range item.answers {
			resolved := false
			for position := range items {
				if !answerEqual(answer.text, strconv.Itoa(position+1)) {
					continue
				}
				resolved = true
				if len(references[index][position]) == 0 {
					graph[index] = append(graph[index], position)
				}
				references[index][position] = append(references[index][position], answer)
			}
			if !resolved {
				return nil, nil, false
			}
		}
	}
	return graph, references, true
}

// v199Matching возвращает true, если все незакреплённые экземпляры размещаются.
// fixedItem=fixedPosition=-1 проверяет полный граф; иначе исключаются заданные
// экземпляр и позиция, причём наличие закрепляемого ребра проверяет вызывающий.
func v199Matching(graph [][]int, fixedItem, fixedPosition int) bool {
	owners := make([]int, len(graph))
	for position := range owners {
		owners[position] = -1
	}
	var augment func(int, []bool) bool
	augment = func(item int, seen []bool) bool {
		for _, position := range graph[item] {
			if position == fixedPosition || seen[position] {
				continue
			}
			seen[position] = true
			if owners[position] == -1 || augment(owners[position], seen) {
				owners[position] = item
				return true
			}
		}
		return false
	}
	for item := range graph {
		if item != fixedItem && !augment(item, make([]bool, len(graph))) {
			return false
		}
	}
	return true
}

// v199Report сохраняет порядок исходных экземпляров и значений внутри группы.
func v199Report(items []v199Item, graph [][]int, references [][][]v199Value, findings diagnostics.Findings) error {
	var groups [][]int
	for index, item := range items {
		group := 0
		for group < len(groups) && !answerEqual(item.label, items[groups[group][0]].label) {
			group++
		}
		if group == len(groups) {
			groups = append(groups, nil)
		}
		groups[group] = append(groups[group], index)
	}
	for _, group := range groups {
		for position := range items {
			var values []v199Value
			reachable := false
			for _, item := range group {
				refs := references[item][position]
				if len(refs) == 0 {
					continue
				}
				if v199Matching(graph, item, position) {
					reachable = true
					break
				}
				values = append(values, refs...)
			}
			if reachable || len(values) == 0 {
				continue
			}
			first := values[0]
			occurrence, err := findings.Add(first.line, first.start, first.line, first.end, "")
			if err != nil {
				return err
			}
			for _, value := range values[1:] {
				if err := findings.AddRelated(occurrence, value.line, value.start, value.line, value.end); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func v199Tag(line diagnostics.Line) string {
	if tag := v199Element(line, "tag"); tag != nil {
		name, _ := tag.Value()
		return name
	}
	return ""
}

func v199Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

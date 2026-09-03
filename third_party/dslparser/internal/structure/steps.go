package structure

import "github.com/verdoga/dslparser/internal/model"

// BuildSteps помещает корневые исходные узлы в синтетические области шагов.
func BuildSteps(nodes []*model.Node) []*model.Node {
	if len(nodes) == 0 {
		return nil
	}
	starts := stepStarts(nodes)
	if len(starts) == 0 {
		return []*model.Node{newStep(nodes)}
	}
	result := make([]*model.Node, 0, len(starts)+1)
	first := starts[0]
	if first > 0 && !mergePreamble(nodes[:first], nodes[first]) {
		result = append(result, newStep(nodes[:first]))
	} else {
		starts[0] = 0
	}
	for i, start := range starts {
		end := len(nodes)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if start < end {
			result = append(result, newStep(nodes[start:end]))
		}
	}
	return result
}

// stepStarts возвращает индексы конструкций, начинающих новый шаг.
func stepStarts(nodes []*model.Node) []int {
	var starts []int
	for i, n := range nodes {
		if n.Kind == 2 && n.HeadingLevel == 3 || n.Kind == 3 && n.CanonicalName == "newpage" {
			starts = append(starts, i)
		}
	}
	return starts
}

// mergePreamble сообщает, можно ли присоединить заголовочный пролог к первому заголовку третьего уровня.
func mergePreamble(nodes []*model.Node, boundary *model.Node) bool {
	if boundary.Kind != 2 || boundary.HeadingLevel != 3 {
		return false
	}
	seenFirst := false
	for _, n := range nodes {
		if n.Kind == 2 && n.HeadingLevel == 1 && !seenFirst {
			seenFirst = true
			continue
		}
		if n.Kind == 2 && n.HeadingLevel == 2 && seenFirst || neutral(n) {
			continue
		}
		return false
	}
	return seenFirst
}

// neutral сообщает, является ли узел нейтральным для определения отображаемого материала.
func neutral(n *model.Node) bool {
	return n.Kind == 4 && n.Raw == "" || n.Kind == 3 && n.CanonicalName == "editor"
}

// newStep создаёт синтетическую область и вычисляет её имя и полный диапазон.
func newStep(children []*model.Node) *model.Node {
	step := &model.Node{Kind: 1, Parsed: true, Synthetic: true, Children: append([]*model.Node(nil), children...)}
	for _, n := range children {
		if n.Kind == 2 && (n.HeadingLevel == 3 || step.Value == "" && n.HeadingLevel == 1) {
			step.Value = n.Value
			if n.HeadingLevel == 3 {
				break
			}
		}
	}
	step.Span = childrenSpan(children)
	return step
}

// childrenSpan возвращает диапазон от начала первого до конца последнего дочернего узла.
func childrenSpan(children []*model.Node) model.Span {
	if len(children) == 0 {
		return model.Span{}
	}
	return model.Span{StartLine: children[0].Span.StartLine, StartColumn: children[0].Span.StartColumn, EndLine: children[len(children)-1].Span.EndLine, EndColumn: children[len(children)-1].Span.EndColumn}
}

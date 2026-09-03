package structure

import "github.com/verdoga/dslparser/internal/model"

// BuildTasks строит области заданий и внутренних этапов и возвращает текущий уровень.
func BuildTasks(nodes []*model.Node) []*model.Node {
	nodes = groupTasks(nodes)
	for _, n := range nodes {
		if n.CanonicalName == "task" || n.CanonicalName == "step" {
			buildTasksBelowRegion(n.Children)
			continue
		}
		n.Children = BuildTasks(n.Children)
	}
	return nodes
}

// buildTasksBelowRegion обрабатывает вложенные физические контейнеры, не повторяя группировку готовой логической области.
func buildTasksBelowRegion(nodes []*model.Node) {
	for _, n := range nodes {
		n.Children = BuildTasks(n.Children)
	}
}

// groupTasks переносит последующий материал в открытые на одном уровне задание и этап.
func groupTasks(nodes []*model.Node) []*model.Node {
	var task, step *model.Node
	previousTask := false
	for i := 0; i < len(nodes); {
		n := nodes[i]
		name := n.CanonicalName
		if task != nil && taskBoundary(n) {
			task.Span = logicalSpan(task)
			task, step = nil, nil
			continue
		}
		if name == "task" {
			task, step = n, nil
			previousTask = true
			i++
			continue
		}
		if name == "step" {
			if task == nil {
				addNodeDiagnostic(n, "STEP_OUTSIDE_TASK", "этап находится вне задания")
				i++
				continue
			}
			step = n
			task.Children = append(task.Children, n)
			nodes = removeNode(nodes, i)
			continue
		}
		if name == "endtask" {
			if task == nil {
				addNodeDiagnostic(n, "ENDTASK_WITHOUT_TASK", "завершение не имеет открытого задания")
				i++
				continue
			}
			task.Children = append(task.Children, n)
			nodes = removeNode(nodes, i)
			task.Span = logicalSpan(task)
			task, step = nil, nil
			continue
		}
		if name == "speaking" {
			if !previousTask {
				addNodeDiagnostic(n, "SPEAKING_CONTEXT", "маркер не связан с непосредственно предшествующим заданием")
			}
			i++
			previousTask = false
			continue
		}
		if task != nil {
			if step != nil {
				step.Children = append(step.Children, n)
				step.Span = logicalSpan(step)
			} else {
				task.Children = append(task.Children, n)
			}
			nodes = removeNode(nodes, i)
			continue
		}
		if !neutral(n) {
			previousTask = false
		}
		i++
	}
	if task != nil {
		task.Span = logicalSpan(task)
	}
	return nodes
}

// taskBoundary сообщает, должна ли конструкция завершить открытое задание до себя.
func taskBoundary(n *model.Node) bool {
	if n.Kind == 2 && n.HeadingLevel == 3 {
		return true
	}
	switch n.CanonicalName {
	case "task", "header", "newpage", "variants", "variant":
		return true
	}
	return false
}

// removeNode удаляет узел из текущего уровня после переноса без его копирования.
func removeNode(nodes []*model.Node, i int) []*model.Node {
	copy(nodes[i:], nodes[i+1:])
	nodes[len(nodes)-1] = nil
	return nodes[:len(nodes)-1]
}

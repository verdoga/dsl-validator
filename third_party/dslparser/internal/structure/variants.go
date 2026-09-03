package structure

import "github.com/verdoga/dslparser/internal/model"

// BuildVariants создаёт логические ветви структурных контейнеров вариантов.
func BuildVariants(nodes []*model.Node) {
	for _, n := range nodes {
		if n.Kind == 3 && n.CanonicalName == "variants" {
			buildVariantContainer(n)
		}
		BuildVariants(n.Children)
	}
}

// buildVariantContainer перегруппировывает содержимое одного внешнего контейнера.
func buildVariantContainer(container *model.Node) {
	original := container.Children
	container.Children = nil
	var branch *model.Node
	for _, n := range original {
		if n.Kind == 3 && n.CanonicalName == "variants" {
			addNodeDiagnostic(n, "NESTED_VARIANTS", "вложенный контейнер вариантов недопустим")
		}
		if n.Kind == 3 && n.CanonicalName == "variant" {
			branch = n
			container.Children = append(container.Children, branch)
			continue
		}
		if branch == nil {
			if !neutral(n) {
				addNodeDiagnostic(n, "CONTENT_BEFORE_VARIANT", "содержимое находится до первой ветви")
			}
			container.Children = append(container.Children, n)
			continue
		}
		branch.Children = append(branch.Children, n)
	}
	for _, n := range container.Children {
		if n.CanonicalName == "variant" {
			n.Span = logicalSpan(n)
		}
	}
}

// MarkOutsideVariants отмечает ветви, которые не принадлежат внешнему контейнеру.
func MarkOutsideVariants(nodes []*model.Node, inside bool) {
	for _, n := range nodes {
		next := inside || n.Kind == 3 && n.CanonicalName == "variants"
		if n.Kind == 3 && n.CanonicalName == "variant" && !inside {
			addNodeDiagnostic(n, "VARIANT_OUTSIDE_VARIANTS", "ветвь находится вне контейнера вариантов")
		}
		MarkOutsideVariants(n.Children, next)
	}
}

// logicalSpan расширяет диапазон логического контейнера до последнего потомка.
func logicalSpan(n *model.Node) model.Span {
	span := n.Span
	if len(n.Children) > 0 {
		last := n.Children[len(n.Children)-1]
		span.EndLine, span.EndColumn = last.Span.EndLine, last.Span.EndColumn
	}
	return span
}

// addNodeDiagnostic прикрепляет к узлу одну контекстную диагностику и меняет только его локальный статус.
func addNodeDiagnostic(n *model.Node, code, message string) {
	n.Parsed = false
	n.Diagnostics = append(n.Diagnostics, model.Diagnostic{Code: code, Message: message, Span: n.Span})
}

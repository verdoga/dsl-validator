package handlers

import "github.com/verdoga/dslparser/internal/model"

// applyRaw сохраняет непрозрачные строки тела без интерпретации DSL.
func applyRaw(n *model.Node, lines []model.Line, preserveEmpty bool) {
	for _, line := range bodyLines(n, lines) {
		if line.Text == "" && !preserveEmpty {
			continue
		}
		n.Elements = append(n.Elements, bodyElement(line, "body"))
	}
}

// bodyLines возвращает строки между общей картой границ блока.
func bodyLines(n *model.Node, lines []model.Line) []model.Line {
	if n.Block == nil {
		return nil
	}
	start, end := n.Block.OpenLine+1, len(lines)
	if n.Block.HasClose {
		end = n.Block.CloseLine
	}
	if start > end || start >= len(lines) {
		return nil
	}
	return lines[start:end]
}

// bodyElement создаёт строковый элемент с полным физическим диапазоном.
func bodyElement(line model.Line, name string) model.Element {
	return model.Element{Kind: elementBodyLine, Name: name, Raw: line.Text, Value: line.Text, Span: model.Span{StartLine: line.Number, StartColumn: 1, EndLine: line.Number, EndColumn: runeCount(line.Text) + 1}, Parsed: true}
}

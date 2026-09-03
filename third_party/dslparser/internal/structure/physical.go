package structure

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/scan"
)

// Build создаёт физическое дерево, не дублируя строки закрывающих границ.
func Build(lines []model.Line, blocks Result) []*model.Node {
	roots := []*model.Node{}
	stack := []*model.Node{}
	for i := 1; i < len(lines); i++ { // директива версии является свойством документа.
		c := lines[i].Candidate
		if c.Kind == scan.BlockClose && len(stack) > 0 {
			stack = stack[: len(stack)-1 : len(stack)-1]
			continue
		}
		n := physicalNode(lines[i], blocks.Diagnostics[i])
		if b, ok := blocks.Blocks[i]; ok {
			copy := b
			n.Block = &copy
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, n)
		}
		if n.Block != nil && n.Block.Structural {
			stack = append(stack, n)
		}
		if n.Block != nil && !n.Block.Structural { // непрозрачное тело сохраняется обработчиком, а не как дочерние узлы.
			i = bodyEnd(i, *n.Block, len(lines))
		}
	}
	return roots
}

// physicalNode преобразует одну содержательную строку в базовый узел.
func physicalNode(line model.Line, diagnostics []model.Diagnostic) *model.Node {
	c := line.Candidate
	n := &model.Node{Raw: line.Text, Parsed: len(diagnostics) == 0, Candidate: c, Diagnostics: append([]model.Diagnostic(nil), diagnostics...), Span: lineSpan(line), Tail: c.Tail, OriginalName: c.Name, CanonicalName: c.CanonicalName}
	if line.Text == "{" && len(diagnostics) > 0 {
		n.Kind = 5
		return n
	}
	switch c.Kind {
	case scan.Heading:
		n.Kind = 2
		h := scan.ParseHeading(line.Text)
		n.Value, n.HeadingLevel, n.Parsed = h.Text, h.Level, h.Parsed
		for _, d := range h.Diagnostics {
			n.Diagnostics = append(n.Diagnostics, model.Diagnostic{Code: d.Code, Message: d.Message, Span: columnSpan(line.Number, d.Start, d.End)})
		}
	case scan.Tag, scan.BlockOpen, scan.Version:
		n.Kind = 3
	case scan.InvalidBoundary:
		n.Kind = 5
	default:
		n.Kind = 4
	}
	if n.Kind == 3 && c.Name == "" {
		n.Parsed = false
		n.Diagnostics = append(n.Diagnostics, model.Diagnostic{Code: "EMPTY_TAG_NAME", Message: "имя тега отсутствует", Span: n.Span})
	}
	return n
}

// bodyEnd возвращает последний индекс тела либо закрывающей границы.
func bodyEnd(open int, b model.Block, count int) int {
	if b.HasClose {
		return b.CloseLine
	}
	return count - 1
}

// columnSpan создаёт диапазон на заданной строке.
func columnSpan(line, start, end int) model.Span {
	return model.Span{StartLine: line, StartColumn: start, EndLine: line, EndColumn: end}
}

// PrepareLines создаёт строки строительной модели из нормализованного текста.
func PrepareLines(numbers []int, texts []string) []model.Line {
	lines := make([]model.Line, len(texts))
	for i := range texts {
		lines[i] = model.Line{Number: numbers[i], Text: strings.TrimSpace(texts[i])}
	}
	return lines
}

// EndColumn возвращает исключённый конечный Unicode-столбец строки.
func EndColumn(text string) int { return utf8.RuneCountInString(text) + 1 }

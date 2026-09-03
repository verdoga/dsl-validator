package structure

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/registry"
	"github.com/verdoga/dslparser/internal/scan"
)

// Analyze классифицирует строки в режиме текущего контейнера и строит карту блоков.
func Analyze(lines []model.Line, rules registry.Registry) Result {
	result := Result{Blocks: make(map[int]model.Block), Diagnostics: make(map[int][]model.Diagnostic)}
	type opened struct {
		line       int
		structural bool
	}
	stack := []opened{}
	for i := range lines {
		mode := scan.Structural
		if len(stack) > 0 && !stack[len(stack)-1].structural {
			mode = scan.Opaque
		}
		lines[i].Candidate = scan.Classify(lines[i].Text, mode)
		c := lines[i].Candidate
		if mode == scan.Structural {
			result.validateBoundary(lines, i, c)
		}
		switch c.Kind {
		case scan.BlockOpen:
			structural := false
			if entry, ok := rules.Lookup(c.CanonicalName); ok {
				structural = entry.Body == registry.BodyStructural
			}
			stack = append(stack, opened{i, structural})
		case scan.BlockClose:
			if len(stack) == 0 {
				result.add(i, "ORPHAN_BLOCK_CLOSE", "закрывающая граница не имеет открытого блока", lineSpan(lines[i]))
				continue
			}
			op := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			result.Blocks[op.line] = makeBlock(lines, op.line, i, op.structural)
		}
	}
	for _, op := range stack {
		b := makeBlock(lines, op.line, -1, op.structural)
		result.Blocks[op.line] = b
		result.add(op.line, "UNCLOSED_BLOCK", "фигурный блок не закрыт", b.Open)
	}
	return result
}

// validateBoundary отмечает похожие на границы строки, не являющиеся корректными границами.
func (r Result) validateBoundary(lines []model.Line, i int, c scan.Candidate) {
	text := lines[i].Text
	if c.Kind == scan.InvalidBoundary {
		r.add(i, "INVALID_BLOCK_CLOSE", "после закрывающей границы присутствуют символы", lineSpan(lines[i]))
		return
	}
	if text == "{" {
		r.add(i, "INVALID_BLOCK_OPEN", "открывающая граница должна завершать строку тега", lineSpan(lines[i]))
		return
	}
	if c.Kind == scan.Tag && strings.Contains(text, "{") {
		for p := 0; p < len(text); p++ {
			if text[p] == '{' && !scan.Escaped(text, p) {
				r.add(i, "INVALID_BLOCK_OPEN", "после открывающей границы присутствуют символы", runeSpan(lines[i].Number, text, p, p+1))
				return
			}
		}
	}
}

// add добавляет диагностику к физической строке.
func (r Result) add(line int, code, message string, span model.Span) {
	r.Diagnostics[line] = append(r.Diagnostics[line], model.Diagnostic{Code: code, Message: message, Span: span})
}

// makeBlock создаёт сведения о закрытом или незакрытом блоке.
func makeBlock(lines []model.Line, open, close int, structural bool) model.Block {
	t := lines[open].Text
	p := strings.LastIndex(t, "{")
	b := model.Block{Open: runeSpan(lines[open].Number, t, p, p+1), OpenLine: open, CloseLine: close, Structural: structural}
	end := lines[len(lines)-1]
	b.Whole = model.Span{StartLine: lines[open].Number, StartColumn: 1, EndLine: end.Number, EndColumn: utf8.RuneCountInString(end.Text) + 1}
	if close >= 0 {
		b.HasClose = true
		b.Close = runeSpan(lines[close].Number, lines[close].Text, 0, 1)
		b.Whole.EndLine, b.Whole.EndColumn = lines[close].Number, 2
	}
	return b
}

// lineSpan возвращает диапазон полной нормализованной строки.
func lineSpan(line model.Line) model.Span {
	return model.Span{StartLine: line.Number, StartColumn: 1, EndLine: line.Number, EndColumn: utf8.RuneCountInString(line.Text) + 1}
}

// runeSpan переводит байтовые индексы в Unicode-столбцы.
func runeSpan(line int, text string, start, end int) model.Span {
	return model.Span{StartLine: line, StartColumn: utf8.RuneCountInString(text[:start]) + 1, EndLine: line, EndColumn: utf8.RuneCountInString(text[:end]) + 1}
}

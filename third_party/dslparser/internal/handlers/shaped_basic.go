package handlers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/scan"
)

// applyExample разбивает тело примера при единственном неэкранированном разделителе.
func applyExample(n *model.Node, lines []model.Line) {
	body := bodyLines(n, lines)
	separator := -1
	count := 0
	for i, line := range body {
		if line.Text == "---" && !scan.Escaped(line.Text, 0) {
			count++
			if separator < 0 {
				separator = i
			}
		}
	}
	if separator < 0 {
		n.Elements = append(n.Elements, group("body", body))
		return
	}
	if count > 1 {
		n.Parsed = false
		n.Diagnostics = append(n.Diagnostics, model.Diagnostic{Code: "MULTIPLE_SEPARATORS", Message: "разделитель указан несколько раз", Span: n.Span})
		n.Elements = append(n.Elements, group("body", body))
		return
	}
	n.Elements = append(n.Elements, group("before", body[:separator]), separatorElement(body[separator]), group("after", body[separator+1:]))
}

// applyWordlistInline разбивает однострочное содержимое по неэкранированным точкам с запятой.
func applyWordlistInline(n *model.Node) {
	raw := argumentTail(n)
	var children []model.Element
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && (raw[i] != ';' || scan.Escaped(raw, i)) {
			continue
		}
		part := strings.TrimSpace(raw[start:i])
		byteStart := strings.Index(raw[start:i], part) + start
		line := model.Line{Number: n.Span.StartLine, Text: part}
		e := bodyElement(line, "item")
		base := utf8.RuneCountInString(n.Raw) - utf8.RuneCountInString(raw) + utf8.RuneCountInString(raw[:byteStart]) + 1
		e.Span = columnSpan(n.Span.StartLine, base, base+utf8.RuneCountInString(part))
		e.Children = placeholdersAt(model.Line{Number: line.Number, Text: part}, base)
		children = append(children, e)
		start = i + 1
	}
	n.Elements = append(n.Elements, model.Element{Kind: elementGroup, Name: "items", Parsed: true, Children: children})
}

// applyWordlistBlock создаёт элементы по непустым строкам блока.
func applyWordlistBlock(n *model.Node, lines []model.Line) {
	n.Elements = append(n.Elements, nonemptyGroup("items", bodyLines(n, lines)))
}

// applyItems создаёт список непустых вариантов выбора.
func applyItems(n *model.Node, lines []model.Line) {
	n.Elements = append(n.Elements, nonemptyGroup("items", bodyLines(n, lines)))
}

// nonemptyGroup создаёт группу непустых строк и ищет в них плейсхолдеры.
func nonemptyGroup(name string, lines []model.Line) model.Element {
	filtered := make([]model.Line, 0, len(lines))
	for _, line := range lines {
		if line.Text != "" {
			filtered = append(filtered, line)
		}
	}
	return group(name, filtered)
}

// group создаёт именованную группу строк в порядке источника.
func group(name string, lines []model.Line) model.Element {
	g := model.Element{Kind: elementGroup, Name: name, Parsed: true}
	for _, line := range lines {
		e := bodyElement(line, "item")
		e.Children = placeholders(line)
		g.Children = append(g.Children, e)
	}
	if len(g.Children) > 0 {
		g.Span = model.Span{StartLine: g.Children[0].Span.StartLine, StartColumn: 1, EndLine: g.Children[len(g.Children)-1].Span.EndLine, EndColumn: g.Children[len(g.Children)-1].Span.EndColumn}
	}
	return g
}

// separatorElement сохраняет строку локального разделителя отдельным элементом.
func separatorElement(line model.Line) model.Element {
	e := bodyElement(line, "separator")
	e.Kind = elementSeparator
	return e
}

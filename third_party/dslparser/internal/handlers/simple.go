package handlers

import (
	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/scan"
)

// addTokens добавляет универсальные токены без назначения предметных имён.
func addTokens(n *model.Node, raw string, start int) {
	t := scan.Tokenize(raw, start)
	for _, v := range t.Values {
		e := model.Element{Kind: elementToken, Name: "token", Raw: v.Raw, Value: v.Value, Span: columnSpan(n.Span.StartLine, v.Start, v.End), Parsed: v.Parsed}
		for _, d := range v.Diagnostics {
			e.Diagnostics = append(e.Diagnostics, fromScan(n.Span.StartLine, d))
		}
		n.Elements = append(n.Elements, e)
	}
	if !t.Parsed {
		n.Parsed = false
		for _, d := range t.Diagnostics {
			n.Diagnostics = append(n.Diagnostics, fromScan(n.Span.StartLine, d))
		}
	}
}

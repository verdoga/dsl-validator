package handlers

import (
	"strconv"
	"strings"

	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/scan"
)

// applyMedia именует токены media_type и source и определяет вид источника без ввода-вывода.
func applyMedia(n *model.Node, raw string, start int) {
	t := scan.Tokenize(raw, start)
	for i, name := range []string{"media_type", "source"} {
		if i >= len(t.Values) {
			break
		}
		v := t.Values[i]
		value := v.Value
		if i == 0 {
			value = strings.ToLower(value)
		}
		n.Elements = append(n.Elements, model.Element{Kind: elementToken, Name: name, Raw: v.Raw, Value: value, Span: columnSpan(n.Span.StartLine, v.Start, v.End), Parsed: v.Parsed})
	}
	if len(t.Values) > 1 {
		kind := "path"
		if _, err := strconv.Atoi(t.Values[1].Value); err == nil {
			kind = "number"
		}
		n.Elements = append(n.Elements, model.Element{Kind: elementField, Name: "source_kind", Raw: kind, Value: kind, Span: columnSpan(n.Span.StartLine, t.Values[1].Start, t.Values[1].End), Parsed: true})
	}
	if !t.Parsed {
		n.Parsed = false
		for _, d := range t.Diagnostics {
			n.Diagnostics = append(n.Diagnostics, fromScan(n.Span.StartLine, d))
		}
	}
}

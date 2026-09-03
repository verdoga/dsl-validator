package handlers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/registry"
	"github.com/verdoga/dslparser/internal/scan"
)

const (
	elementField     = 1
	elementToken     = 2
	elementBodyLine  = 3
	elementSeparator = 4
	elementGroup     = 5
)

// applyArguments разбирает аргументный хвост согласно записи реестра.
func applyArguments(n *model.Node, entry registry.Entry) {
	raw := argumentTail(n)
	start := n.Candidate.NameEnd + 1
	switch entry.Arguments {
	case registry.ArgsNone:
		return
	case registry.ArgsID:
		addTokenField(n, "id", raw, start, true)
	case registry.ArgsVersion:
		addTokenField(n, "version", raw, start, false)
	case registry.ArgsMediaTokens:
		applyMedia(n, raw, start)
	default:
		name := argumentName(entry.Arguments)
		n.Elements = append(n.Elements, model.Element{Kind: elementField, Name: name, Raw: raw, Value: raw, Span: tailSpan(n, raw), Parsed: true})
	}
}

// argumentTail удаляет только терминальную открывающую скобку блочной формы.
func argumentTail(n *model.Node) string {
	raw := n.Tail
	if n.Block != nil {
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "{"))
	}
	return raw
}

// argumentName возвращает единое техническое имя свободного поля.
func argumentName(schema registry.ArgumentSchema) string {
	switch schema {
	case registry.ArgsName:
		return "name"
	case registry.ArgsTitle, registry.ArgsOptionalTitle:
		return "title"
	case registry.ArgsOptionalInstruction:
		return "instruction"
	default:
		return "content"
	}
}

// addTokenField добавляет первый универсальный токен и сохраняет ошибки кавычек.
func addTokenField(n *model.Node, name, raw string, start int, lower bool) {
	t := scan.Tokenize(raw, start)
	if len(t.Values) == 0 {
		return
	}
	v := t.Values[0]
	value := v.Value
	if lower {
		value = strings.ToLower(value)
	}
	e := model.Element{Kind: elementToken, Name: name, Raw: v.Raw, Value: value, Span: columnSpan(n.Span.StartLine, v.Start, v.End), Parsed: v.Parsed}
	for _, d := range v.Diagnostics {
		e.Diagnostics = append(e.Diagnostics, fromScan(n.Span.StartLine, d))
	}
	n.Elements = append(n.Elements, e)
	if !t.Parsed {
		n.Parsed = false
		n.Diagnostics = append(n.Diagnostics, e.Diagnostics...)
	}
}

// tailSpan вычисляет диапазон свободного хвоста.
func tailSpan(n *model.Node, raw string) model.Span {
	start := utf8.RuneCountInString(n.Raw) - utf8.RuneCountInString(raw) + 1
	return columnSpan(n.Span.StartLine, start, start+utf8.RuneCountInString(raw))
}

// fromScan преобразует локальную диагностику сканера.
func fromScan(line int, d scan.Diagnostic) model.Diagnostic {
	return model.Diagnostic{Code: d.Code, Message: d.Message, Span: columnSpan(line, d.Start, d.End)}
}

// columnSpan создаёт однострочный диапазон по Unicode-столбцам.
func columnSpan(line, start, end int) model.Span {
	return model.Span{StartLine: line, StartColumn: start, EndLine: line, EndColumn: end}
}

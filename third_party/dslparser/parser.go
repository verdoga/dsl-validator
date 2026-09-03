package dslparser

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dslparser/internal/handlers"
	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/normalize"
	"github.com/verdoga/dslparser/internal/registry"
	"github.com/verdoga/dslparser/internal/structure"
)

// Parse разбирает байтовое представление документа или возвращает типизированную фатальную ошибку.
func Parse(data []byte) (*Document, error) { return parse(data) }

// ParseString разбирает строковое представление документа или возвращает типизированную фатальную ошибку.
func ParseString(source string) (*Document, error) { return parse([]byte(source)) }

// ParseReader читает весь документ и передаёт его общему конвейеру разбора.
func ParseReader(r io.Reader) (*Document, error) {
	if r == nil {
		return nil, newFatalError(ReadFailure, "не удалось прочитать документ: nil Reader", nil)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, newFatalError(ReadFailure, "не удалось прочитать документ", err)
	}
	return parse(data)
}

// parse выполняет общий начальный конвейер разбора.
func parse(data []byte) (*Document, error) {
	lines, valid := normalize.Input(data)
	if !valid {
		return nil, newFatalError(InvalidUTF8, "вход содержит некорректный UTF-8", nil)
	}
	if len(lines) == 0 {
		return nil, newFatalError(MissingVersion, "отсутствует директива версии", nil)
	}
	const prefix = "@dsl-version"
	first := lines[0].Text
	if first == prefix || !strings.HasPrefix(first, prefix) || (len(first) > len(prefix) && !isSpaceAfterPrefix(first[len(prefix):])) {
		return nil, newFatalError(MissingVersion, "первая физическая строка не содержит корректную директиву версии", nil)
	}
	raw := strings.TrimSpace(first[len(prefix):])
	if raw == "" {
		return nil, newFatalError(MissingVersion, "значение версии отсутствует", nil)
	}
	if strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return nil, newFatalError(MissingVersion, "директива версии повреждена", nil)
	}
	canonical := strings.ToLower(raw)
	rules, ok := registry.ForVersion(canonical)
	if !ok {
		return nil, newFatalError(UnsupportedVersion, fmt.Sprintf("версия %q не поддерживается", raw), nil)
	}
	end := utf8.RuneCountInString(first) + 1
	buildingLines := make([]model.Line, len(lines))
	for i, line := range lines {
		buildingLines[i] = model.Line{Number: line.Number, Text: line.Text}
	}
	blocks := structure.Analyze(buildingLines, rules)
	roots := structure.Build(buildingLines, blocks)
	handlers.Apply(roots, buildingLines, rules)
	roots = structure.BuildSteps(roots)
	structure.MarkOutsideVariants(roots, false)
	structure.BuildVariants(roots)
	roots = structure.BuildTasks(roots)
	doc := &Document{versionRaw: raw, version: canonical, versionSpan: Span{start: Position{line: 1, column: 1}, end: Position{line: 1, column: end}}}
	for _, root := range roots {
		doc.roots = append(doc.roots, freezeNode(root))
		collectDiagnostics(root, &doc.diagnostics)
	}
	return doc, nil
}

// freezeNode создаёт независимое неизменяемое публичное представление узла.
func freezeNode(source *model.Node) Node {
	n := Node{kind: NodeKind(source.Kind), raw: source.Raw, value: source.Value, originalName: source.OriginalName, canonicalName: source.CanonicalName, headingLevel: source.HeadingLevel, span: freezeSpan(source.Span), parsed: source.Parsed, synthetic: source.Synthetic}
	for _, d := range source.Diagnostics {
		n.diagnostics = append(n.diagnostics, freezeDiagnostic(d))
	}
	for _, e := range source.Elements {
		n.elements = append(n.elements, freezeElement(e))
	}
	for _, child := range source.Children {
		n.children = append(n.children, freezeNode(child))
	}
	if source.Block != nil {
		b := BlockInfo{open: freezeSpan(source.Block.Open), whole: freezeSpan(source.Block.Whole), hasClose: source.Block.HasClose}
		if source.Block.Structural {
			b.mode = BodyStructural
		} else {
			b.mode = BodyOpaque
		}
		if source.Block.HasClose {
			b.close = freezeSpan(source.Block.Close)
		}
		n.block = &b
	}
	return n
}

// freezeElement создаёт независимую копию разобранного элемента.
func freezeElement(source model.Element) Element {
	e := Element{kind: ElementKind(source.Kind), name: source.Name, raw: source.Raw, value: source.Value, span: freezeSpan(source.Span), parsed: source.Parsed}
	for _, d := range source.Diagnostics {
		e.diagnostics = append(e.diagnostics, freezeDiagnostic(d))
	}
	for _, child := range source.Children {
		e.children = append(e.children, freezeElement(child))
	}
	return e
}

// freezeSpan преобразует внутренний полуоткрытый диапазон.
func freezeSpan(s model.Span) Span {
	return Span{start: Position{line: s.StartLine, column: s.StartColumn}, end: Position{line: s.EndLine, column: s.EndColumn}}
}

// freezeDiagnostic преобразует локальную диагностику строительной модели.
func freezeDiagnostic(d model.Diagnostic) Diagnostic {
	return Diagnostic{code: DiagnosticCode(d.Code), message: d.Message, span: freezeSpan(d.Span)}
}

// collectDiagnostics собирает диагностики узлов в порядке физического дерева.
func collectDiagnostics(n *model.Node, out *[]Diagnostic) {
	for _, d := range n.Diagnostics {
		*out = append(*out, freezeDiagnostic(d))
	}
	for _, e := range n.Elements {
		collectElementDiagnostics(e, out)
	}
	for _, child := range n.Children {
		collectDiagnostics(child, out)
	}
}

// collectElementDiagnostics собирает диагностики вложенного элемента.
func collectElementDiagnostics(e model.Element, out *[]Diagnostic) {
	for _, d := range e.Diagnostics {
		*out = append(*out, freezeDiagnostic(d))
	}
	for _, child := range e.Children {
		collectElementDiagnostics(child, out)
	}
}

// isSpaceAfterPrefix сообщает, является ли первый символ хвоста Unicode-пробелом: true — является, false — нет.
func isSpaceAfterPrefix(tail string) bool {
	r, _ := utf8.DecodeRuneInString(tail)
	return unicode.IsSpace(r)
}

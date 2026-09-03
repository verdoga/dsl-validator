package parseradapter

import (
	"errors"
	"fmt"
	"io"

	"dslparser/validatorapi"
	p "github.com/verdoga/dslparser"
)

// FatalIssue описывает фатальную ошибку без координат.
type FatalIssue struct {
	Code    validatorapi.ParserCode
	Message string
}

// Failure описывает нарушение или неожиданный сбой внешнего контракта.
type Failure struct {
	Code string
	Err  error
}

// Error возвращает контекст сбоя.
func (f *Failure) Error() string { return f.Code + ": " + f.Err.Error() }

// Unwrap возвращает исходную причину.
func (f *Failure) Unwrap() error { return f.Err }

// Result содержит ровно один успешный либо ошибочный вариант.
type Result struct {
	Document validatorapi.Document
	Context  validatorapi.Context
	Fatal    *FatalIssue
	Failure  *Failure
}

// Adapter выполняет один вызов парсера на операцию.
type Adapter struct {
	parse func(io.Reader) (*p.Document, error)
}

// New создаёт production-адаптер.
func New() Adapter { return Adapter{parse: p.ParseReader} }

// ParseReader безопасно классифицирует результат парсера.
func (a Adapter) ParseReader(r io.Reader) (result Result) { return a.parseReader(r) }

// parseReader ограничивает recover границей внешнего вызова.
func (a Adapter) parseReader(r io.Reader) (result Result) {
	defer func() {
		if value := recover(); value != nil {
			result = Result{Failure: &Failure{Code: "PARSER_PANIC", Err: fmt.Errorf("panic внешнего парсера: %v", value)}}
		}
	}()
	parse := a.parse
	if parse == nil {
		parse = p.ParseReader
	}
	document, err := parse(r)
	if document != nil && err != nil {
		return Result{Failure: &Failure{Code: "PARSER_CONTRACT_FAILURE", Err: errors.New("парсер вернул документ вместе с ошибкой")}}
	}
	if err != nil {
		var fatal *p.FatalError
		if errors.As(err, &fatal) {
			return Result{Fatal: &FatalIssue{Code: validatorapi.ParserCode(fatal.Code()), Message: fatal.Error()}}
		}
		return Result{Failure: &Failure{Code: "PARSER_UNEXPECTED_ERROR", Err: err}}
	}
	if document == nil {
		return Result{Failure: &Failure{Code: "PARSER_CONTRACT_FAILURE", Err: errors.New("парсер вернул nil без ошибки")}}
	}
	d := documentView{document}
	if err := validateDocument(d); err != nil {
		return Result{Failure: &Failure{Code: "PARSER_CONTRACT_FAILURE", Err: err}}
	}
	c := newContext(d)
	return Result{Document: d, Context: c}
}

// validateDocument проверяет отображаемые enum до передачи AST потребителям.
func validateDocument(document validatorapi.Document) error {
	var validateElements func([]validatorapi.Element) error
	validateElements = func(elements []validatorapi.Element) error {
		for _, element := range elements {
			if element.Kind() == "" {
				return errors.New("неизвестный вид элемента")
			}
			if err := validateElements(element.Children()); err != nil {
				return err
			}
		}
		return nil
	}
	var validateNodes func([]validatorapi.Node) error
	validateNodes = func(nodes []validatorapi.Node) error {
		for _, node := range nodes {
			if node.Kind() == "" {
				return errors.New("неизвестный вид узла")
			}
			if block, ok := node.Block(); ok && block.Mode() == "" {
				return errors.New("неизвестный режим тела")
			}
			if err := validateElements(node.Elements()); err != nil {
				return err
			}
			if err := validateNodes(node.Children()); err != nil {
				return err
			}
		}
		return nil
	}
	return validateNodes(document.Roots())
}

// position преобразует координату без изменения.
func position(v p.Position) validatorapi.Position {
	return validatorapi.Position{Line: v.Line(), Column: v.Column()}
}

// span преобразует полуоткрытый диапазон без изменения.
func span(v p.Span) validatorapi.Span {
	return validatorapi.Span{Start: position(v.Start()), End: position(v.End())}
}

// issueView является read-only ошибкой.
type issueView struct{ value p.Diagnostic }

func (v issueView) Code() validatorapi.ParserCode { return validatorapi.ParserCode(v.value.Code()) }
func (v issueView) Message() string               { return v.value.Message() }
func (v issueView) Span() validatorapi.Span       { return span(v.value.Span()) }
func (v issueView) RelatedSpan() (validatorapi.Span, bool) {
	s, ok := v.value.RelatedSpan()
	return span(s), ok
}

// issues возвращает новый срез.
func issues(values []p.Diagnostic) []validatorapi.ParserIssue {
	out := make([]validatorapi.ParserIssue, len(values))
	for i, v := range values {
		out[i] = issueView{v}
	}
	return out
}

// blockView является read-only блоком.
type blockView struct{ value p.BlockInfo }

func (v blockView) OpenSpan() validatorapi.Span { return span(v.value.OpenSpan()) }
func (v blockView) CloseSpan() (validatorapi.Span, bool) {
	s, ok := v.value.CloseSpan()
	return span(s), ok
}
func (v blockView) Span() validatorapi.Span { return span(v.value.Span()) }
func (v blockView) Closed() bool            { return v.value.Closed() }
func (v blockView) Mode() validatorapi.BodyMode {
	switch v.value.Mode() {
	case p.BodyStructural:
		return validatorapi.BodyStructural
	case p.BodyOpaque:
		return validatorapi.BodyOpaque
	default:
		return ""
	}
}

// elementView является read-only элементом.
type elementView struct{ value p.Element }

func (v elementView) Kind() validatorapi.ElementKind {
	switch v.value.Kind() {
	case p.ElementField:
		return validatorapi.ElementField
	case p.ElementToken:
		return validatorapi.ElementToken
	case p.ElementBodyLine:
		return validatorapi.ElementBodyLine
	case p.ElementSeparator:
		return validatorapi.ElementSeparator
	case p.ElementGroup:
		return validatorapi.ElementGroup
	default:
		return ""
	}
}
func (v elementView) Name() string                             { return v.value.Name() }
func (v elementView) Raw() string                              { return v.value.Raw() }
func (v elementView) Value() string                            { return v.value.Value() }
func (v elementView) Span() validatorapi.Span                  { return span(v.value.Span()) }
func (v elementView) Parsed() bool                             { return v.value.Parsed() }
func (v elementView) ParserIssues() []validatorapi.ParserIssue { return issues(v.value.Diagnostics()) }
func (v elementView) Children() []validatorapi.Element {
	values := v.value.Children()
	out := make([]validatorapi.Element, len(values))
	for i, e := range values {
		out[i] = elementView{e}
	}
	return out
}

// nodeView является read-only узлом.
type nodeView struct{ value p.Node }

func (v nodeView) Kind() validatorapi.NodeKind {
	switch v.value.Kind() {
	case p.NodeStep:
		return validatorapi.NodeStep
	case p.NodeHeading:
		return validatorapi.NodeHeading
	case p.NodeTag:
		return validatorapi.NodeTag
	case p.NodeText:
		return validatorapi.NodeText
	case p.NodeBlockBoundary:
		return validatorapi.NodeBlockBoundary
	default:
		return ""
	}
}
func (v nodeView) Raw() string                              { return v.value.Raw() }
func (v nodeView) Value() string                            { return v.value.Value() }
func (v nodeView) OriginalName() string                     { return v.value.OriginalName() }
func (v nodeView) CanonicalName() string                    { return v.value.CanonicalName() }
func (v nodeView) HeadingLevel() int                        { return v.value.HeadingLevel() }
func (v nodeView) Span() validatorapi.Span                  { return span(v.value.Span()) }
func (v nodeView) Parsed() bool                             { return v.value.Parsed() }
func (v nodeView) Synthetic() bool                          { return v.value.Synthetic() }
func (v nodeView) ParserIssues() []validatorapi.ParserIssue { return issues(v.value.Diagnostics()) }
func (v nodeView) Elements() []validatorapi.Element {
	values := v.value.Elements()
	out := make([]validatorapi.Element, len(values))
	for i, e := range values {
		out[i] = elementView{e}
	}
	return out
}
func (v nodeView) Children() []validatorapi.Node {
	values := v.value.Children()
	out := make([]validatorapi.Node, len(values))
	for i, n := range values {
		out[i] = nodeView{n}
	}
	return out
}
func (v nodeView) Block() (validatorapi.Block, bool) {
	b, ok := v.value.Block()
	if !ok {
		return nil, false
	}
	return blockView{b}, true
}

// documentView является read-only документом.
type documentView struct{ value *p.Document }

func (v documentView) VersionRaw() string             { return v.value.VersionRaw() }
func (v documentView) Version() validatorapi.Version  { return validatorapi.Version(v.value.Version()) }
func (v documentView) VersionSpan() validatorapi.Span { return span(v.value.VersionSpan()) }
func (v documentView) Roots() []validatorapi.Node {
	values := v.value.Roots()
	out := make([]validatorapi.Node, len(values))
	for i, n := range values {
		out[i] = nodeView{n}
	}
	return out
}
func (v documentView) ParserIssues() []validatorapi.ParserIssue { return issues(v.value.Diagnostics()) }

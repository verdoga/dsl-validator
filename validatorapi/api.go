package validatorapi

import (
	"fmt"
	"strings"
)

// Version задаёт каноническую версию DSL.
type Version string

// Severity задаёт уровень срабатывания.
type Severity string

// Source задаёт источник записи отчёта.
type Source string

// ParserCode задаёт стабильный код ошибки парсера.
type ParserCode string

// NodeKind задаёт вид узла AST.
type NodeKind string

// ElementKind задаёт вид элемента AST.
type ElementKind string

// BodyMode задаёт режим тела блока.
type BodyMode string

// Значения уровней срабатывания.
const (
	SeverityError          Severity = "error"
	SeverityWarning        Severity = "warning"
	SeverityRecommendation Severity = "recommendation"
)

// Значения источников записей.
const (
	SourceParser     Source = "parser"
	SourceDiagnostic Source = "diagnostic"
	SourceValidator  Source = "validator"
)

// Значения видов узлов.
const (
	NodeStep          NodeKind = "step"
	NodeHeading       NodeKind = "heading"
	NodeTag           NodeKind = "tag"
	NodeText          NodeKind = "text"
	NodeBlockBoundary NodeKind = "block_boundary"
)

// Значения видов элементов.
const (
	ElementField     ElementKind = "field"
	ElementToken     ElementKind = "token"
	ElementBodyLine  ElementKind = "body_line"
	ElementSeparator ElementKind = "separator"
	ElementGroup     ElementKind = "group"
)

// Значения режимов тела.
const (
	BodyOpaque     BodyMode = "opaque"
	BodyStructural BodyMode = "structural"
)

// Коды ошибок парсера.
const (
	ParserInvalidUTF8            ParserCode = "INVALID_UTF8"
	ParserMissingVersion         ParserCode = "MISSING_VERSION"
	ParserUnsupportedVersion     ParserCode = "UNSUPPORTED_VERSION"
	ParserReadFailure            ParserCode = "READ_FAILURE"
	ParserEmptyTagName           ParserCode = "EMPTY_TAG_NAME"
	ParserMalformedHeading       ParserCode = "MALFORMED_HEADING"
	ParserUnclosedQuote          ParserCode = "UNCLOSED_QUOTE"
	ParserInvalidBlockOpen       ParserCode = "INVALID_BLOCK_OPEN"
	ParserOrphanBlockClose       ParserCode = "ORPHAN_BLOCK_CLOSE"
	ParserInvalidBlockClose      ParserCode = "INVALID_BLOCK_CLOSE"
	ParserUnclosedBlock          ParserCode = "UNCLOSED_BLOCK"
	ParserUnsupportedTagForm     ParserCode = "UNSUPPORTED_TAG_FORM"
	ParserMissingSeparator       ParserCode = "MISSING_SEPARATOR"
	ParserMultipleSeparators     ParserCode = "MULTIPLE_SEPARATORS"
	ParserStepOutsideTask        ParserCode = "STEP_OUTSIDE_TASK"
	ParserEndtaskWithoutTask     ParserCode = "ENDTASK_WITHOUT_TASK"
	ParserSpeakingContext        ParserCode = "SPEAKING_CONTEXT"
	ParserVariantOutsideVariants ParserCode = "VARIANT_OUTSIDE_VARIANTS"
	ParserNestedVariants         ParserCode = "NESTED_VARIANTS"
	ParserContentBeforeVariant   ParserCode = "CONTENT_BEFORE_VARIANT"
)

// Position задаёт единичные номер строки и Unicode-столбца.
type Position struct {
	Line   int
	Column int
}

// Span задаёт полуоткрытый диапазон, исключающий End.
type Span struct {
	Start Position
	End   Position
}

// Valid сообщает корректность: true означает единичные координаты и неубывающий конец; false — нарушение этих условий.
func (s Span) Valid() bool {
	return s.Start.Line > 0 && s.Start.Column > 0 && s.End.Line > 0 && s.End.Column > 0 && (s.End.Line > s.Start.Line || s.End.Line == s.Start.Line && s.End.Column >= s.Start.Column)
}

// SpanByRuneOffsets строит полуоткрытый поддиапазон однострочного текста по индексам Unicode-кодовых точек.
func SpanByRuneOffsets(text string, base Position, start, end int) (Span, error) {
	if base.Line < 1 || base.Column < 1 || strings.ContainsAny(text, "\r\n") {
		return Span{}, fmt.Errorf("некорректная база или многострочный текст")
	}
	n := len([]rune(text))
	if start < 0 || end < start || end > n {
		return Span{}, fmt.Errorf("индексы [%d,%d) вне диапазона [0,%d]", start, end, n)
	}
	return Span{Position{base.Line, base.Column + start}, Position{base.Line, base.Column + end}}, nil
}

// ParserIssue предоставляет неизменяемую ошибку парсера.
type ParserIssue interface {
	Code() ParserCode
	Message() string
	Span() Span
	RelatedSpan() (Span, bool)
}

// Block предоставляет неизменяемые сведения о блоке.
type Block interface {
	OpenSpan() Span
	CloseSpan() (Span, bool)
	Span() Span
	Closed() bool
	Mode() BodyMode
}

// Element предоставляет неизменяемый элемент AST.
type Element interface {
	Kind() ElementKind
	Name() string
	Raw() string
	Value() string
	Span() Span
	Parsed() bool
	ParserIssues() []ParserIssue
	Children() []Element
}

// Node предоставляет неизменяемый узел AST.
type Node interface {
	Kind() NodeKind
	Raw() string
	Value() string
	OriginalName() string
	CanonicalName() string
	HeadingLevel() int
	Span() Span
	Parsed() bool
	Synthetic() bool
	ParserIssues() []ParserIssue
	Elements() []Element
	Children() []Node
	Block() (Block, bool)
}

// Document предоставляет неизменяемый документ.
type Document interface {
	VersionRaw() string
	Version() Version
	VersionSpan() Span
	Roots() []Node
	ParserIssues() []ParserIssue
}

// Cursor предоставляет навигацию по готовому логическому дереву.
type Cursor interface {
	Node() Node
	Path() []int
	Depth() int
	Parent() (Cursor, bool)
	PreviousSibling() (Cursor, bool)
	NextSibling() (Cursor, bool)
	Children() []Cursor
	Ancestors() []Cursor
}

// Context предоставляет индексы и обход готового AST.
type Context interface {
	Document() Document
	Walk(func(Cursor) bool)
	PhysicalNodes() []Cursor
	NodesByKind(NodeKind) []Cursor
	NodesByCanonicalName(string) []Cursor
	ParserIssues() []ParserIssue
	HasParserIssue(ParserCode, Span) bool
}

// Passport описывает стабильный паспорт диагностики.
type Passport struct {
	Code        string
	Category    string
	Severity    Severity
	Title       string
	Description string
	Basis       string
	Reference   string
	BadExample  string
}

// VersionScope задаёт все версии либо непустой конкретный набор.
type VersionScope struct {
	All      bool
	Versions []Version
}

// Finding задаёт одно срабатывание с необязательными диапазонами.
type Finding struct {
	Message     string
	Span        *Span
	RelatedSpan *Span
}

// Reporter принимает срабатывания диагностики.
type Reporter interface{ Report(Finding) error }

// Check является чистой диагностической функцией.
type Check func(Context, Reporter) error

// Definition связывает паспорт, область версий и функцию.
type Definition struct {
	Passport Passport
	Versions VersionScope
	Check    Check
}

// WalkElements обходит элементы узла depth-first pre-order; false прекращает весь обход.
func WalkElements(node Node, visit func(Element) bool) bool {
	if node == nil || visit == nil {
		return true
	}
	var walk func([]Element) bool
	walk = func(es []Element) bool {
		for _, e := range es {
			if !visit(e) || !walk(e.Children()) {
				return false
			}
		}
		return true
	}
	return walk(node.Elements())
}

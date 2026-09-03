package dslparser

// DiagnosticCode является стабильным машинным кодом диагностики.
type DiagnosticCode string

// Коды фатальных и локальных диагностик.
const (
	InvalidUTF8        DiagnosticCode = "INVALID_UTF8"
	MissingVersion     DiagnosticCode = "MISSING_VERSION"
	UnsupportedVersion DiagnosticCode = "UNSUPPORTED_VERSION"
	ReadFailure        DiagnosticCode = "READ_FAILURE"
	EmptyTagName       DiagnosticCode = "EMPTY_TAG_NAME"
	MalformedHeading   DiagnosticCode = "MALFORMED_HEADING"
	UnclosedQuote      DiagnosticCode = "UNCLOSED_QUOTE"
	InvalidBlockOpen   DiagnosticCode = "INVALID_BLOCK_OPEN"
	OrphanBlockClose   DiagnosticCode = "ORPHAN_BLOCK_CLOSE"
	InvalidBlockClose  DiagnosticCode = "INVALID_BLOCK_CLOSE"
	UnclosedBlock      DiagnosticCode = "UNCLOSED_BLOCK"
	UnsupportedTagForm DiagnosticCode = "UNSUPPORTED_TAG_FORM"
	MissingSeparator   DiagnosticCode = "MISSING_SEPARATOR"
	MultipleSeparators DiagnosticCode = "MULTIPLE_SEPARATORS"
	StepOutsideTask    DiagnosticCode = "STEP_OUTSIDE_TASK"
	EndtaskWithoutTask DiagnosticCode = "ENDTASK_WITHOUT_TASK"
	SpeakingContext    DiagnosticCode = "SPEAKING_CONTEXT"
	VariantOutside     DiagnosticCode = "VARIANT_OUTSIDE_VARIANTS"
	NestedVariants     DiagnosticCode = "NESTED_VARIANTS"
	ContentBefore      DiagnosticCode = "CONTENT_BEFORE_VARIANT"
)

// Diagnostic описывает локальную ошибку и, при наличии, связанную конструкцию.
type Diagnostic struct {
	code    DiagnosticCode
	message string
	span    Span
	related *Span
}

// Code возвращает стабильный машинный код.
func (d Diagnostic) Code() DiagnosticCode { return d.code }

// Message возвращает краткое сообщение на русском языке.
func (d Diagnostic) Message() string { return d.message }

// Span возвращает диапазон ошибки.
func (d Diagnostic) Span() Span { return d.span }

// RelatedSpan возвращает связанный диапазон и true; при его отсутствии возвращает нулевой диапазон и false.
func (d Diagnostic) RelatedSpan() (Span, bool) {
	if d.related == nil {
		return Span{}, false
	}
	return *d.related, true
}

// FatalError является типизированной ошибкой, при которой документ не создаётся.
type FatalError struct {
	code    DiagnosticCode
	message string
	cause   error
}

// Error возвращает описание фатальной ошибки.
func (e *FatalError) Error() string { return e.message }

// Unwrap возвращает исходную ошибку чтения либо nil.
func (e *FatalError) Unwrap() error { return e.cause }

// Code возвращает стабильный код фатальной ошибки.
func (e *FatalError) Code() DiagnosticCode { return e.code }

// newFatalError создаёт фатальную ошибку с необязательной причиной.
func newFatalError(code DiagnosticCode, message string, cause error) error {
	return &FatalError{code: code, message: message, cause: cause}
}

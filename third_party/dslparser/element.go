package dslparser

// ElementKind определяет универсальный вид элемента.
type ElementKind int

// Универсальные виды элементов AST.
const (
	ElementField ElementKind = iota + 1
	ElementToken
	ElementBodyLine
	ElementSeparator
	ElementGroup
)

// Технические имена элементов.
const (
	ElementVersion       = "version"
	ElementLevel         = "level"
	ElementID            = "id"
	ElementName          = "name"
	ElementTitle         = "title"
	ElementContent       = "content"
	ElementInstruction   = "instruction"
	ElementMediaType     = "media_type"
	ElementSource        = "source"
	ElementSourceKind    = "source_kind"
	ElementBody          = "body"
	ElementItems         = "items"
	ElementBefore        = "before"
	ElementAfter         = "after"
	ElementUpper         = "upper"
	ElementLower         = "lower"
	ElementPositions     = "positions"
	ElementSeparatorName = "separator"
	ElementPlaceholder   = "placeholder"
)

// Element представляет разобранную часть универсального узла.
type Element struct {
	kind             ElementKind
	name, raw, value string
	span             Span
	parsed           bool
	diagnostics      []Diagnostic
	children         []Element
}

// Kind возвращает вид элемента.
func (e Element) Kind() ElementKind { return e.kind }

// Name возвращает стабильное техническое имя.
func (e Element) Name() string { return e.name }

// Raw возвращает исходное нормализованное значение с экранированием.
func (e Element) Raw() string { return e.raw }

// Value возвращает разобранное значение.
func (e Element) Value() string { return e.value }

// Span возвращает полуоткрытый диапазон элемента.
func (e Element) Span() Span { return e.span }

// Parsed сообщает локальный статус: true означает успех, false — наличие локальной ошибки элемента.
func (e Element) Parsed() bool { return e.parsed }

// Diagnostics возвращает защитную копию локальных диагностик.
func (e Element) Diagnostics() []Diagnostic { return append([]Diagnostic(nil), e.diagnostics...) }

// Children возвращает защитную копию дочерних элементов в исходном порядке.
func (e Element) Children() []Element { return append([]Element(nil), e.children...) }

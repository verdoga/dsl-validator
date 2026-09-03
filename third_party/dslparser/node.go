package dslparser

// NodeKind определяет универсальный вид узла.
type NodeKind int

// Универсальные виды узлов AST.
const (
	NodeStep NodeKind = iota + 1
	NodeHeading
	NodeTag
	NodeText
	NodeBlockBoundary
)

// BodyMode определяет способ интерпретации тела блока.
type BodyMode int

// Режимы тела блока.
const (
	BodyOpaque BodyMode = iota
	BodyStructural
)

// BlockInfo описывает диапазоны и режим фигурного блока.
type BlockInfo struct {
	open, close, whole Span
	hasClose           bool
	mode               BodyMode
}

// OpenSpan возвращает диапазон открывающей скобки.
func (b BlockInfo) OpenSpan() Span { return b.open }

// CloseSpan возвращает диапазон закрывающей скобки и true либо нулевой диапазон и false.
func (b BlockInfo) CloseSpan() (Span, bool) { return b.close, b.hasClose }

// Span возвращает диапазон всего блока.
func (b BlockInfo) Span() Span { return b.whole }

// Closed сообщает статус: true означает корректное закрытие, false — отсутствие закрывающей скобки.
func (b BlockInfo) Closed() bool { return b.hasClose }

// Mode возвращает режим содержимого блока.
func (b BlockInfo) Mode() BodyMode { return b.mode }

// Node представляет универсальную конструкцию документа.
type Node struct {
	kind                                    NodeKind
	raw, value, originalName, canonicalName string
	headingLevel                            int
	span                                    Span
	parsed, synthetic                       bool
	diagnostics                             []Diagnostic
	elements                                []Element
	children                                []Node
	block                                   *BlockInfo
}

// Kind возвращает вид узла.
func (n Node) Kind() NodeKind { return n.kind }

// Raw возвращает исходную строку после тримминга.
func (n Node) Raw() string { return n.raw }

// Value возвращает разобранное значение узла.
func (n Node) Value() string { return n.value }

// OriginalName возвращает исходное имя тега с сохранением регистра.
func (n Node) OriginalName() string { return n.originalName }

// CanonicalName возвращает каноническое имя тега.
func (n Node) CanonicalName() string { return n.canonicalName }

// HeadingLevel возвращает уровень заголовка либо ноль для другого вида.
func (n Node) HeadingLevel() int { return n.headingLevel }

// Span возвращает полуоткрытый диапазон узла.
func (n Node) Span() Span { return n.span }

// Parsed сообщает локальный статус: true означает успех, false — локальную ошибку узла.
func (n Node) Parsed() bool { return n.parsed }

// Synthetic сообщает происхождение: true означает синтетическую логическую область, false — исходную конструкцию.
func (n Node) Synthetic() bool { return n.synthetic }

// Diagnostics возвращает защитную копию локальных диагностик.
func (n Node) Diagnostics() []Diagnostic { return append([]Diagnostic(nil), n.diagnostics...) }

// Elements возвращает защитную копию элементов в исходном порядке.
func (n Node) Elements() []Element { return append([]Element(nil), n.elements...) }

// Children возвращает защитную копию дочерних узлов в исходном порядке.
func (n Node) Children() []Node { return append([]Node(nil), n.children...) }

// Block возвращает сведения о блоке и true либо нулевое значение и false.
func (n Node) Block() (BlockInfo, bool) {
	if n.block == nil {
		return BlockInfo{}, false
	}
	return *n.block, true
}

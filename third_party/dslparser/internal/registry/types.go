package registry

// Form задаёт поддерживаемую синтаксическую форму тега.
type Form uint8

// Формы тегов можно объединять побитово.
const (
	FormInline Form = 1 << iota
	FormBlock
)

// ArgumentSchema задаёт способ разбора аргументного хвоста.
type ArgumentSchema int

// Поддерживаемые схемы аргументов.
const (
	ArgsNone ArgumentSchema = iota
	ArgsVersion
	ArgsID
	ArgsName
	ArgsTitle
	ArgsOptionalTitle
	ArgsContent
	ArgsOptionalInstruction
	ArgsMediaTokens
)

// BodyMode задаёт способ обработки тела.
type BodyMode int

// Поддерживаемые режимы тела.
const (
	BodyNone BodyMode = iota
	BodyOpaqueRaw
	BodyOpaqueShaped
	BodyStructural
	BodyLogical
)

// HandlerID идентифицирует универсальный или специальный обработчик.
type HandlerID int

// Идентификаторы обработчиков реестра.
const (
	HandlerNone HandlerID = iota
	HandlerTokens
	HandlerFreeText
	HandlerRawBody
	HandlerShapedBody
	HandlerStructural
	HandlerVersion
	HandlerMedia
	HandlerExample
	HandlerWordlist
	HandlerItems
	HandlerMatching
	HandlerOrdering
)

// ContextEffect описывает влияние тега на логический контекст.
type ContextEffect int

// Виды влияния на логический контекст.
const (
	ContextNone ContextEffect = iota
	ContextTaskStart
	ContextTaskEnd
	ContextTaskBoundary
	ContextStepStart
	ContextStepBoundary
	ContextSpeaking
	ContextVariants
	ContextVariant
)

// Entry является копируемым неизменяемым описанием тега.
type Entry struct {
	Name          string
	Forms         Form
	Arguments     ArgumentSchema
	Body          BodyMode
	Universal     HandlerID
	Special       HandlerID
	BodyHandler   HandlerID
	Context       ContextEffect
	PreserveEmpty bool
}

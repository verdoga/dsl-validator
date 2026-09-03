package dslparser

// Position задаёт позицию в нормализованной физической строке; строка и столбец нумеруются с единицы.
type Position struct {
	line   int
	column int
}

// Line возвращает физический номер строки.
func (p Position) Line() int { return p.line }

// Column возвращает номер столбца в Unicode-кодовых точках.
func (p Position) Column() int { return p.column }

// Span задаёт полуоткрытый диапазон [Start, End).
type Span struct {
	start Position
	end   Position
}

// Start возвращает включённое начало диапазона.
func (s Span) Start() Position { return s.start }

// End возвращает исключённый конец диапазона.
func (s Span) End() Position { return s.end }

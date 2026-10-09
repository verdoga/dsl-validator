package diagnostics

type linesContract interface {
	Len() int
	Line(number int) (line Line, found bool)
}

type lineContract interface {
	Number() int
	Type() string
	NestingLevel() int
	ParentLine() (number int, found bool)
	Raw() string
	EOL() string
	ElementCount() int
	Element(index int) (element Element, found bool)
}

type elementContract interface {
	Type() string
	Raw() string
	Value() (text string, present bool)
	Start() int
	End() int
}

// Присваиваемость в обе стороны проверяет точный набор методов и сигнатуры:
// удаление, добавление или изменение метода приводит к ошибке компиляции.
var (
	_ linesContract   = (Lines)(nil)
	_ Lines           = (linesContract)(nil)
	_ lineContract    = (Line)(nil)
	_ Line            = (lineContract)(nil)
	_ elementContract = (Element)(nil)
	_ Element         = (elementContract)(nil)
)

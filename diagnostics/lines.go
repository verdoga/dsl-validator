package diagnostics

// Lines предоставляет только построчные данные без Document, Processing,
// Diagnostics, HasErrors и ErrorIDs. Возвращаемые объекты не имеют setters
// и не раскрывают model, указатели, срезы либо карты.
type Lines interface {
	// Len возвращает число строк; пустой документ имеет длину 0.
	Len() int
	// Line получает строку по физическому номеру 1..Len.
	// found=false означает выход за границы и nil вместо строки.
	// Последовательный вызов с возрастающим или убывающим номером даёт обход
	// вперёд или назад без отдельного курсора и скрытого состояния.
	Line(number int) (line Line, found bool)
}

// Line — доступ только для чтения к содержимому и структуре одной строки.
type Line interface {
	// Number возвращает физический номер начиная с 1.
	Number() int
	// Type возвращает строковое значение существующего model.LineType.
	Type() string
	// NestingLevel возвращает логическую глубину.
	NestingLevel() int
	// ParentLine возвращает номер родителя; found=false и 0 означают корень.
	ParentLine() (number int, found bool)
	// Raw возвращает точный исходный текст строки без BOM и EOL.
	Raw() string
	// EOL возвращает исходное окончание: LF, CRLF или пустую строку.
	EOL() string
	// ElementCount возвращает число элементов строки.
	ElementCount() int
	// Element получает элемент по индексу от 0 до ElementCount исключительно.
	// found=false означает выход за границы и nil вместо элемента.
	Element(index int) (element Element, found bool)
}

// Element — доступ только для чтения к элементу, без его ссылок на ошибки.
type Element interface {
	// Type возвращает строковое значение существующего model.ElementType.
	Type() string
	// Raw возвращает точный фрагмент строки.
	Raw() string
	// Value возвращает семантическое значение; present=false означает JSON null,
	// present=true допускает пустую строку и отличается от отсутствия значения.
	Value() (text string, present bool)
	// Start возвращает включаемую Unicode-колонку начиная с 1.
	Start() int
	// End возвращает исключающую Unicode-колонку; End может равняться Start.
	End() int
}

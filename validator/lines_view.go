package validator

import (
	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// linesView — внутренняя неизменяемая проекция только физических строк.
type linesView struct {
	// lines — собственные глубокие копии строк; HasErrors=false и ErrorIDs=nil.
	// Метаданные документа, история и диагностики никогда сюда не передаются.
	lines []model.Line
}

// newLinesView копирует строки и изменяемые вложенные значения.
// Внутри validator возвращает закрытый тип; в Check он передаётся только
// как diagnostics.Lines. Исходные срезы и указатели не удерживает.
func newLinesView(lines []model.Line) *linesView {
	v := &linesView{lines: make([]model.Line, len(lines))}
	for i, line := range lines {
		v.lines[i] = line
		v.lines[i].HasErrors = false
		if line.ParentLine != nil {
			parent := *line.ParentLine
			v.lines[i].ParentLine = &parent
		}
		if line.Elements != nil {
			v.lines[i].Elements = make([]model.Element, len(line.Elements))
		}
		for j, element := range line.Elements {
			v.lines[i].Elements[j] = element
			v.lines[i].Elements[j].ErrorIDs = nil
			if element.Value != nil {
				value := *element.Value
				v.lines[i].Elements[j].Value = &value
			}
		}
	}
	return v
}

// Len возвращает количество строк проекции.
func (v *linesView) Len() int {
	return len(v.lines)
}

// Line возвращает закрытую обёртку строки; вне 1..Len возвращает nil, false.
func (v *linesView) Line(number int) (line diagnostics.Line, found bool) {
	if number < 1 || number > len(v.lines) {
		return nil, false
	}
	return lineView{line: v.lines[number-1]}, true
}

// lineView скрывает модель строки и предоставляет только скалярные значения.
type lineView struct {
	// line — строка приватной проекции без сведений об ошибках.
	line model.Line
}

// Number возвращает физический номер строки.
func (v lineView) Number() int {
	return v.line.Number
}

// Type возвращает каноническое строковое значение LineType.
func (v lineView) Type() string {
	return string(v.line.LineType)
}

// NestingLevel возвращает глубину строки.
func (v lineView) NestingLevel() int {
	return v.line.NestingLevel
}

// ParentLine возвращает значение родителя; false и 0 означают корень.
func (v lineView) ParentLine() (number int, found bool) {
	if v.line.ParentLine == nil {
		return 0, false
	}
	return *v.line.ParentLine, true
}

// Raw возвращает точный текст строки без изменения экранирования.
func (v lineView) Raw() string {
	return v.line.Raw
}

// EOL возвращает исходное окончание строки.
func (v lineView) EOL() string {
	return string(v.line.LineEnding)
}

// ElementCount возвращает количество элементов.
func (v lineView) ElementCount() int {
	return len(v.line.Elements)
}

// Element возвращает закрытую обёртку элемента; неверный индекс даёт nil, false.
func (v lineView) Element(index int) (element diagnostics.Element, found bool) {
	if index < 0 || index >= len(v.line.Elements) {
		return nil, false
	}
	return elementView{element: v.line.Elements[index]}, true
}

// elementView скрывает модель элемента и не предоставляет ErrorIDs.
type elementView struct {
	// element — элемент приватной проекции без ссылок на ошибки.
	element model.Element
}

// Type возвращает каноническое строковое значение ElementType.
func (v elementView) Type() string {
	return string(v.element.ElementType)
}

// Raw возвращает точный фрагмент исходной строки.
func (v elementView) Raw() string {
	return v.element.Raw
}

// Value возвращает копию значения; false означает null, true — наличие строки.
func (v elementView) Value() (text string, present bool) {
	if v.element.Value == nil {
		return "", false
	}
	return *v.element.Value, true
}

// Start возвращает включаемую Unicode-колонку с 1.
func (v elementView) Start() int {
	return v.element.Start
}

// End возвращает исключающую Unicode-колонку.
func (v elementView) End() int {
	return v.element.End
}

package model

import "github.com/verdoga/dslparser/internal/scan"

// Span задаёт полуоткрытый диапазон в нормализованном исходном тексте.
type Span struct {
	StartLine, StartColumn int
	EndLine, EndColumn     int
}

// Diagnostic хранит локальную ошибку строительной модели.
type Diagnostic struct {
	Code, Message string
	Span          Span
}

// Element хранит именованную разобранную часть узла.
type Element struct {
	Kind             int
	Name, Raw, Value string
	Span             Span
	Parsed           bool
	Diagnostics      []Diagnostic
	Children         []Element
}

// Block хранит единожды вычисленные границы фигурного блока.
type Block struct {
	Open, Close, Whole  Span
	HasClose            bool
	Structural          bool
	OpenLine, CloseLine int
}

// Node хранит универсальный узел во время построения дерева.
type Node struct {
	Kind                                          int
	Raw, Value, OriginalName, CanonicalName, Tail string
	HeadingLevel                                  int
	Span                                          Span
	Parsed, Synthetic                             bool
	Candidate                                     scan.Candidate
	Diagnostics                                   []Diagnostic
	Elements                                      []Element
	Children                                      []*Node
	Block                                         *Block
}

// Line хранит физическую строку и её классификацию.
type Line struct {
	Number    int
	Text      string
	Candidate scan.Candidate
}

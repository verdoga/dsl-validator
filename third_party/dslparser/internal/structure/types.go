package structure

import "github.com/verdoga/dslparser/internal/model"

// Result содержит единую карту блоков и локальные ошибки структурного прохода.
type Result struct {
	Blocks      map[int]model.Block
	Diagnostics map[int][]model.Diagnostic
}

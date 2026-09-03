package dslparser

// Cursor описывает текущую позицию детерминированного обхода документа.
type Cursor struct {
	node  Node
	depth int
	path  []int
}

// Node возвращает текущий узел.
func (c Cursor) Node() Node { return c.node }

// Depth возвращает глубину узла, где корневой узел имеет глубину ноль.
func (c Cursor) Depth() int { return c.depth }

// Path возвращает защитную копию пути индексов от корневого среза до текущего узла.
func (c Cursor) Path() []int { return append([]int(nil), c.path...) }

// Walk обходит узлы документа в прямом порядке; false от visit пропускает только потомков текущего узла.
func Walk(document *Document, visit func(Cursor) bool) {
	if document == nil || visit == nil {
		return
	}
	walkNodes(document.roots, nil, visit)
}

// walkNodes рекурсивно вычисляет глубину и путь без сохранения родительских ссылок.
func walkNodes(nodes []Node, parentPath []int, visit func(Cursor) bool) {
	for i, node := range nodes {
		path := make([]int, len(parentPath)+1)
		copy(path, parentPath)
		path[len(parentPath)] = i
		if visit(Cursor{node: node, depth: len(parentPath), path: path}) {
			walkNodes(node.children, path, visit)
		}
	}
}

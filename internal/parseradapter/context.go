package parseradapter

import (
	"dslparser/validatorapi"
	"sort"
)

// contextView хранит неизменяемые индексы документа.
type contextView struct {
	document validatorapi.Document
	cursors  []*cursorView
}

// cursorView хранит путь и связь с контекстом.
type cursorView struct {
	context  *contextView
	node     validatorapi.Node
	path     []int
	parent   *cursorView
	children []*cursorView
}

// newContext строит навигацию только из готового AST.
func newContext(document validatorapi.Document) *contextView {
	c := &contextView{document: document}
	var add func([]validatorapi.Node, []int, *cursorView)
	add = func(nodes []validatorapi.Node, parentPath []int, parent *cursorView) {
		for i, n := range nodes {
			path := append(append([]int(nil), parentPath...), i)
			cursor := &cursorView{context: c, node: n, path: path, parent: parent}
			c.cursors = append(c.cursors, cursor)
			if parent != nil {
				parent.children = append(parent.children, cursor)
			}
			add(n.Children(), path, cursor)
		}
	}
	add(document.Roots(), nil, nil)
	return c
}
func (c *contextView) Document() validatorapi.Document { return c.document }
func (c *contextView) Walk(visit func(validatorapi.Cursor) bool) {
	if visit == nil {
		return
	}
	var walk func([]*cursorView)
	walk = func(cursors []*cursorView) {
		for _, cursor := range cursors {
			if !visit(cursor) {
				continue
			}
			walk(cursor.children)
		}
	}
	roots := make([]*cursorView, 0)
	for _, cursor := range c.cursors {
		if cursor.parent == nil {
			roots = append(roots, cursor)
		}
	}
	walk(roots)
}
func (c *contextView) PhysicalNodes() []validatorapi.Cursor {
	out := make([]validatorapi.Cursor, 0)
	for _, v := range c.cursors {
		if !v.node.Synthetic() {
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Node().Span().Start, out[j].Node().Span().Start
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	return out
}
func (c *contextView) NodesByKind(kind validatorapi.NodeKind) []validatorapi.Cursor {
	return c.selectNodes(func(n validatorapi.Node) bool { return n.Kind() == kind })
}
func (c *contextView) NodesByCanonicalName(name string) []validatorapi.Cursor {
	return c.selectNodes(func(n validatorapi.Node) bool { return n.CanonicalName() == name })
}
func (c *contextView) selectNodes(match func(validatorapi.Node) bool) []validatorapi.Cursor {
	out := make([]validatorapi.Cursor, 0)
	for _, v := range c.cursors {
		if match(v.node) {
			out = append(out, v)
		}
	}
	return out
}
func (c *contextView) ParserIssues() []validatorapi.ParserIssue { return c.document.ParserIssues() }
func (c *contextView) HasParserIssue(code validatorapi.ParserCode, s validatorapi.Span) bool {
	for _, v := range c.ParserIssues() {
		if v.Code() == code && v.Span() == s {
			return true
		}
	}
	return false
}
func (c *contextView) find(path []int) *cursorView {
	for _, v := range c.cursors {
		if samePath(v.path, path) {
			return v
		}
	}
	return nil
}

// samePath сравнивает пути.
func samePath(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (c *cursorView) Node() validatorapi.Node { return c.node }
func (c *cursorView) Path() []int             { return append([]int(nil), c.path...) }
func (c *cursorView) Depth() int              { return len(c.path) - 1 }
func (c *cursorView) Parent() (validatorapi.Cursor, bool) {
	if c.parent == nil {
		return nil, false
	}
	return c.parent, true
}
func (c *cursorView) PreviousSibling() (validatorapi.Cursor, bool) {
	if c.path[len(c.path)-1] == 0 {
		return nil, false
	}
	p := c.Path()
	p[len(p)-1]--
	v := c.context.find(p)
	return v, v != nil
}
func (c *cursorView) NextSibling() (validatorapi.Cursor, bool) {
	p := c.Path()
	p[len(p)-1]++
	v := c.context.find(p)
	return v, v != nil
}
func (c *cursorView) Children() []validatorapi.Cursor {
	out := make([]validatorapi.Cursor, len(c.children))
	for i, child := range c.children {
		out[i] = child
	}
	return out
}
func (c *cursorView) Ancestors() []validatorapi.Cursor {
	out := make([]validatorapi.Cursor, c.Depth())
	ancestor := c.parent
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = ancestor
		ancestor = ancestor.parent
	}
	return out
}

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
	context *contextView
	node    validatorapi.Node
	path    []int
}

// newContext строит навигацию только из готового AST.
func newContext(document validatorapi.Document) *contextView {
	c := &contextView{document: document}
	var add func([]validatorapi.Node, []int)
	add = func(nodes []validatorapi.Node, parent []int) {
		for i, n := range nodes {
			path := append(append([]int(nil), parent...), i)
			c.cursors = append(c.cursors, &cursorView{context: c, node: n, path: path})
			add(n.Children(), path)
		}
	}
	add(document.Roots(), nil)
	return c
}
func (c *contextView) Document() validatorapi.Document { return c.document }
func (c *contextView) Walk(visit func(validatorapi.Cursor) bool) {
	if visit == nil {
		return
	}
	var walk func([]validatorapi.Node, []int) bool
	walk = func(nodes []validatorapi.Node, parent []int) bool {
		for i, n := range nodes {
			path := append(append([]int(nil), parent...), i)
			cur := c.find(path)
			if !visit(cur) {
				continue
			}
			if !walk(n.Children(), path) {
				return false
			}
		}
		return true
	}
	walk(c.document.Roots(), nil)
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
	if len(c.path) < 2 {
		return nil, false
	}
	return c.context.find(c.path[:len(c.path)-1]), true
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
	out := make([]validatorapi.Cursor, 0)
	for _, v := range c.context.cursors {
		if len(v.path) == len(c.path)+1 && samePath(v.path[:len(c.path)], c.path) {
			out = append(out, v)
		}
	}
	return out
}
func (c *cursorView) Ancestors() []validatorapi.Cursor {
	out := make([]validatorapi.Cursor, 0, len(c.path)-1)
	for n := 1; n < len(c.path); n++ {
		out = append(out, c.context.find(c.path[:n]))
	}
	return out
}

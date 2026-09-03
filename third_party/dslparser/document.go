package dslparser

// Document является неизменяемым результатом разбора DSL.
type Document struct {
	versionRaw, version string
	versionSpan         Span
	roots               []Node
	diagnostics         []Diagnostic
}

// VersionRaw возвращает объявленную версию в исходном виде.
func (d *Document) VersionRaw() string {
	if d == nil {
		return ""
	}
	return d.versionRaw
}

// Version возвращает каноническую версию.
func (d *Document) Version() string {
	if d == nil {
		return ""
	}
	return d.version
}

// VersionSpan возвращает диапазон директивы версии.
func (d *Document) VersionSpan() Span {
	if d == nil {
		return Span{}
	}
	return d.versionSpan
}

// Roots возвращает защитную копию корневых узлов.
func (d *Document) Roots() []Node {
	if d == nil {
		return nil
	}
	return append([]Node(nil), d.roots...)
}

// Diagnostics возвращает защитную копию диагностик в порядке исходного текста.
func (d *Document) Diagnostics() []Diagnostic {
	if d == nil {
		return nil
	}
	return append([]Diagnostic(nil), d.diagnostics...)
}

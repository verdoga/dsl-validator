package registry

import "strings"

// Registry содержит независимую неизменяемую копию правил одной версии.
type Registry struct {
	version string
	entries []Entry
}

// ForVersion возвращает реестр и true для поддерживаемой версии; иначе нулевой реестр и false.
func ForVersion(version string) (Registry, bool) {
	if version != "1.1" {
		return Registry{}, false
	}
	return Registry{version: version, entries: entries11()}, true
}

// Version возвращает версию набора правил.
func (r Registry) Version() string { return r.version }

// Lookup выполняет регистронезависимый поиск и возвращает копию записи.
func (r Registry) Lookup(name string) (Entry, bool) {
	name = strings.ToLower(name)
	for _, entry := range r.entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return Entry{}, false
}

// Entries возвращает независимую копию записей в стабильном порядке.
func (r Registry) Entries() []Entry { return append([]Entry(nil), r.entries...) }

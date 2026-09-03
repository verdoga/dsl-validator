package webassets

import "embed"

// Files содержит неизменяемые встроенные шаблоны и assets.
//
//go:embed templates/*.html assets/*.css assets/*.js
var Files embed.FS

module dslparser

go 1.25.1

require github.com/verdoga/dslparser v0.0.0-20260902080052-8486082e285e

// Локальный снимок точного коммита используется из-за недоступности сети в сборочном окружении.
replace github.com/verdoga/dslparser => ./third_party/dslparser

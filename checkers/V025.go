package checkers

import (
	"fmt"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV025 регистрирует проверку смешанных окончаний строк для DSL 1.2.
func registrationV025(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V025",
		Severity: diagnostics.SeverityWarning,
		Message:  "В документе смешаны окончания строк LF и CRLF.",
		Scope:    diagnostics.ScopeDocument,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV025,
	})
}

// checkV025 считает LF и CRLF во всех физических строках без изменения данных.
// Единственное сообщение не имеет основного диапазона. Связанные пустые
// диапазоны указывают концы первых строк LF и CRLF в Unicode-колонках с 1.
func checkV025(lines diagnostics.Lines, findings diagnostics.Findings) error {
	lf, crlf := 0, 0
	var firstLF, firstCRLF [2]int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		switch line.EOL() {
		case "\n":
			if lf == 0 {
				firstLF = [2]int{line.Number(), utf8.RuneCountInString(line.Raw()) + 1}
			}
			lf++
		case "\r\n":
			if crlf == 0 {
				firstCRLF = [2]int{line.Number(), utf8.RuneCountInString(line.Raw()) + 1}
			}
			crlf++
		}
	}
	if lf == 0 || crlf == 0 {
		return nil
	}
	occurrence, err := findings.Add(0, 0, 0, 0, fmt.Sprintf("LF: %d; CRLF: %d", lf, crlf))
	if err != nil {
		return err
	}
	if err := findings.AddRelated(occurrence, firstLF[0], firstLF[1], firstLF[0], firstLF[1]); err != nil {
		return err
	}
	return findings.AddRelated(occurrence, firstCRLF[0], firstCRLF[1], firstCRLF[0], firstCRLF[1])
}

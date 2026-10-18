package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV045 регистрирует ошибку повторного пути ресурсов для DSL 1.2.
func registrationV045(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V045",
		Severity: diagnostics.SeverityError,
		Message:  "Путь к каталогу ресурсов указан повторно.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV045,
	})
}

// v045Position хранит исходную строку и Unicode-колонки первого пути;
// правая граница исключается.
type v045Position struct {
	line, start, end int
}

// checkV045 сравнивает непустые Value путей только по нижнему регистру Unicode.
// Каждый повтор ссылается на первое вхождение во всём документе.
func checkV045(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first := make(map[string]v045Position)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		resourceDir := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				name, _ := element.Value()
				if name == "resource-dir" {
					resourceDir = true
					break
				}
			}
		}
		if !resourceDir {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			path, _ := line.Element(index)
			if path.Type() != "resource-path" {
				continue
			}
			value, _ := path.Value()
			if value == "" {
				continue
			}
			key := strings.ToLower(value)
			previous, exists := first[key]
			if !exists {
				first[key] = v045Position{line: line.Number(), start: path.Start(), end: path.End()}
				continue
			}
			occurrence, err := findings.Add(line.Number(), path.Start(), line.Number(), path.End(), "")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
				return err
			}
		}
	}
	return nil
}

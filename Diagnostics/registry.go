package diagnostics

import (
	"fmt"

	"dslparser/validatorapi"
)

// All возвращает свежий пустой список предметных диагностик.
func All() []validatorapi.Definition { return []validatorapi.Definition{} }

// SupportedVersions возвращает свежий список поддерживаемых версий.
func SupportedVersions() []validatorapi.Version { return []validatorapi.Version{"1.1"} }

// ForVersion возвращает совместимые определения в стабильном порядке.
func ForVersion(version validatorapi.Version) []validatorapi.Definition {
	out := make([]validatorapi.Definition, 0)
	for _, d := range All() {
		if d.Versions.All || contains(d.Versions.Versions, version) {
			out = append(out, d)
		}
	}
	return out
}

// Validate проверяет production-реестр.
func Validate() error { return validate(All(), SupportedVersions()) }

// contains сообщает принадлежность: true — версия присутствует, false — отсутствует.
func contains(values []validatorapi.Version, want validatorapi.Version) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// validate проверяет определения и пересечение областей.
func validate(definitions []validatorapi.Definition, supported []validatorapi.Version) error {
	seen := make(map[string][]validatorapi.Version)
	for _, d := range definitions {
		p := d.Passport
		if p.Code == "" || p.Category == "" || p.Title == "" || p.Description == "" || p.Basis == "" || p.Reference == "" || p.BadExample == "" || d.Check == nil {
			return fmt.Errorf("неполное определение %q", p.Code)
		}
		if p.Severity != validatorapi.SeverityError && p.Severity != validatorapi.SeverityWarning && p.Severity != validatorapi.SeverityRecommendation {
			return fmt.Errorf("неверный уровень %q", p.Severity)
		}
		if d.Versions.All && len(d.Versions.Versions) > 0 || !d.Versions.All && len(d.Versions.Versions) == 0 {
			return fmt.Errorf("неверная область %q", p.Code)
		}
		scope := d.Versions.Versions
		if d.Versions.All {
			scope = supported
		}
		for _, v := range scope {
			if !contains(supported, v) {
				return fmt.Errorf("неподдерживаемая версия %q", v)
			}
			if contains(seen[p.Code], v) {
				return fmt.Errorf("код %q пересекается в версии %q", p.Code, v)
			}
			seen[p.Code] = append(seen[p.Code], v)
		}
	}
	return nil
}

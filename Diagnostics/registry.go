package diagnostics

import (
	"fmt"
	"regexp"
	"strings"

	"dslparser/validatorapi"
)

// All возвращает свежий пустой список предметных диагностик.
func All() []validatorapi.Definition { return []validatorapi.Definition{} }

// SupportedVersions возвращает свежий список поддерживаемых версий.
func SupportedVersions() []validatorapi.Version { return []validatorapi.Version{"1.1"} }

// ForVersion возвращает совместимые определения в стабильном порядке.
func ForVersion(version validatorapi.Version) []validatorapi.Definition {
	if !contains(SupportedVersions(), version) {
		return []validatorapi.Definition{}
	}
	out := make([]validatorapi.Definition, 0)
	for _, d := range All() {
		if d.Versions.All || contains(d.Versions.Versions, version) {
			out = append(out, cloneDefinition(d))
		}
	}
	return out
}

// cloneDefinition возвращает определение без общего изменяемого среза версий.
func cloneDefinition(definition validatorapi.Definition) validatorapi.Definition {
	definition.Versions.Versions = append([]validatorapi.Version(nil), definition.Versions.Versions...)
	return definition
}

// codePattern задаёт каноническую форму машинного кода диагностики.
var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`)

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
	if len(supported) == 0 {
		return fmt.Errorf("список поддерживаемых версий пуст")
	}
	versions := make(map[validatorapi.Version]struct{}, len(supported))
	for _, version := range supported {
		if strings.TrimSpace(string(version)) != string(version) || version == "" || strings.Count(string(version), ".") != 1 {
			return fmt.Errorf("неканоническая версия %q", version)
		}
		if _, exists := versions[version]; exists {
			return fmt.Errorf("повтор версии %q", version)
		}
		versions[version] = struct{}{}
	}
	seen := make(map[string][]validatorapi.Version)
	for _, d := range definitions {
		p := d.Passport
		if !codePattern.MatchString(p.Code) || strings.TrimSpace(p.Category) == "" || strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Description) == "" || strings.TrimSpace(p.Basis) == "" || strings.TrimSpace(p.Reference) == "" || strings.TrimSpace(p.BadExample) == "" || d.Check == nil {
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
		local := make(map[validatorapi.Version]struct{}, len(d.Versions.Versions))
		for _, version := range d.Versions.Versions {
			if _, exists := local[version]; exists {
				return fmt.Errorf("повтор версии %q в области %q", version, p.Code)
			}
			local[version] = struct{}{}
		}
	}
	return nil
}

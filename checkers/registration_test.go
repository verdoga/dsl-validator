package checkers

import (
	"slices"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
)

func TestRegistrationPreservesMergedDiagnostics(t *testing.T) {
	// Объединение локального каталога и main: ни один код не должен исчезнуть.
	// Новые регистрации разрешены, но все перечисленные должны быть доступны.
	want := []diagnostics.Code{
		"V001", "V002", "V003", "V004", "V005", "V006", "V007", "V008", "V009", "V010",
		"V011", "V012", "V013", "V014", "V015", "V016", "V017", "V018", "V019", "V020",
		"V021", "V022", "V023", "V024", "V025", "V026", "V027", "V028", "V029", "V030",
		"V031", "V032", "V033", "V034", "V035", "V036", "V037", "V038", "V039", "V040",
		"V041", "V042", "V043", "V044", "V045", "V046", "V047", "V048", "V049", "V050",
	}
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions := registrationDefinitions(t, registry, "1.2")
	for _, code := range want {
		if !slices.ContainsFunc(definitions, func(d diagnostics.Definition) bool { return d.Code == code }) {
			t.Errorf("merged diagnostic %s is missing from DSL 1.2", code)
		}
	}
}

func TestRegistrationCheckers(t *testing.T) {
	t.Run("nonempty catalog", func(t *testing.T) {
		registrations := RegistrationCheckers()
		if registrations == nil {
			t.Fatal("RegistrationCheckers returned nil")
		}
		if len(registrations) == 0 {
			t.Fatal("catalog must contain real diagnostics")
		}
		for name, registration := range registrations {
			if name == "" || registration == nil {
				t.Fatalf("invalid registration %q: missing name or function", name)
			}
		}
	})

	t.Run("independent catalogs", func(t *testing.T) {
		first := RegistrationCheckers()
		second := RegistrationCheckers()
		if first == nil || second == nil {
			t.Fatal("RegistrationCheckers returned nil")
		}
		if _, exists := first["test-only"]; exists {
			t.Fatal("test key conflicts with a real registration")
		}
		originalLen := len(second)
		first["test-only"] = nil
		if _, exists := second["test-only"]; exists || len(second) != originalLen {
			t.Fatal("changing the first catalog changed the second")
		}
		for name := range first {
			if name != "test-only" {
				delete(first, name)
			}
		}
		third := RegistrationCheckers()
		if third == nil || len(third) != originalLen || len(second) != originalLen {
			t.Fatal("changing the first catalog changed other catalogs")
		}
		if _, exists := third["test-only"]; exists {
			t.Fatal("a subsequent call retained a caller's registration")
		}
		for name, registration := range second {
			if registration == nil || third[name] == nil {
				t.Fatalf("registration %q was lost", name)
			}
		}
		if _, exists := first["test-only"]; !exists || len(first) != 1 {
			t.Fatal("a subsequent call changed the caller-owned catalog")
		}
	})

	t.Run("creates populated registry", func(t *testing.T) {
		registry, err := diagnostics.NewRegistry(RegistrationCheckers())
		if err != nil {
			t.Fatalf("NewRegistry returned an error: %v", err)
		}
		if registry == nil {
			t.Fatal("NewRegistry returned nil")
		}
		if registry.Len() == 0 {
			t.Fatal("registry must contain real diagnostics")
		}
	})
}

func TestRegistrationRegistryIndependence(t *testing.T) {
	registrations := RegistrationCheckers()
	first, err := diagnostics.NewRegistry(registrations)
	if err != nil {
		t.Fatal(err)
	}
	second, err := diagnostics.NewRegistry(registrations)
	if err != nil {
		t.Fatal(err)
	}
	wantLen := first.Len()
	if wantLen == 0 || second.Len() != wantLen {
		t.Fatalf("registry lengths = %d, %d, want equal nonzero lengths", wantLen, second.Len())
	}
	want := make(map[dslversions.Version][]diagnostics.Definition)
	for _, version := range dslversions.Supported() {
		want[version] = append([]diagnostics.Definition(nil), registrationDefinitions(t, first, version)...)
		assertRegistrationDefinitions(t, registrationDefinitions(t, second, version), want[version])
	}

	clear(registrations)
	third, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		registry *diagnostics.Registry
	}{
		{"first registry", first},
		{"repeated construction", second},
		{"fresh catalog", third},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.registry.Len() != wantLen {
				t.Fatalf("registry length = %d, want %d", tc.registry.Len(), wantLen)
			}
			for _, version := range dslversions.Supported() {
				t.Run(string(version), func(t *testing.T) {
					got := registrationDefinitions(t, tc.registry, version)
					assertRegistrationDefinitions(t, got, want[version])
				})
			}
		})
	}
}

func TestRegistrationVersionSelectionsAreIndependent(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	wantLen := registry.Len()
	for _, version := range dslversions.Supported() {
		t.Run(string(version), func(t *testing.T) {
			first := registrationDefinitions(t, registry, version)
			second := registrationDefinitions(t, registry, version)
			// Снимок не должен разделять массив ни с одной проверяемой выборкой.
			want := append([]diagnostics.Definition(nil), first...)
			assertRegistrationDefinitions(t, second, want)
			clear(first)
			assertRegistrationDefinitions(t, second, want)
			assertRegistrationDefinitions(t, registrationDefinitions(t, registry, version), want)
			if registry.Len() != wantLen {
				t.Fatalf("registry length = %d, want %d", registry.Len(), wantLen)
			}
		})
	}
}

// registrationDefinitions проверяет выборку штатного реестра без привязки к числу диагностик.
func registrationDefinitions(t *testing.T, registry *diagnostics.Registry, version dslversions.Version) []diagnostics.Definition {
	t.Helper()
	definitions, err := registry.ForVersion(version)
	if err != nil {
		t.Fatalf("ForVersion(%s): %v", version, err)
	}
	if len(definitions) > registry.Len() {
		t.Fatalf("selection length = %d exceeds registry length = %d", len(definitions), registry.Len())
	}
	seen := make(map[diagnostics.Code]bool)
	for _, definition := range definitions {
		if definition.Code == "" || seen[definition.Code] || definition.Check == nil {
			t.Fatalf("invalid or duplicate definition: %+v", definition)
		}
		seen[definition.Code] = true
	}
	return definitions
}

// assertRegistrationDefinitions сравнивает метаданные и порядок, не вызывая проверки диагностик.
func assertRegistrationDefinitions(t *testing.T, got, want []diagnostics.Definition) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("definitions length = %d, want %d", len(got), len(want))
	}
	for i, definition := range got {
		expected := want[i]
		if definition.Code != expected.Code || definition.Severity != expected.Severity ||
			definition.Message != expected.Message || definition.Scope != expected.Scope ||
			definition.Versions != expected.Versions || definition.Check == nil {
			t.Fatalf("definition %d = %+v, want %+v with nonnil Check", i, definition, expected)
		}
	}
}

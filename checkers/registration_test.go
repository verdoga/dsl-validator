package checkers

import (
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
)

func TestRegistrationCheckers(t *testing.T) {
	t.Run("nonempty catalog", func(t *testing.T) {
		registrations := RegistrationCheckers()
		if registrations == nil {
			t.Fatal("RegistrationCheckers returned nil")
		}
		if len(registrations) == 0 {
			t.Fatal("catalog must contain real diagnostics")
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

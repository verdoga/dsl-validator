package checkers

import (
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
)

func TestRegistrationCheckers(t *testing.T) {
	t.Run("empty non-nil catalog", func(t *testing.T) {
		registrations := RegistrationCheckers()
		if registrations == nil {
			t.Fatal("RegistrationCheckers returned nil")
		}
		if len(registrations) != 0 {
			t.Fatalf("catalog length = %d, want 0", len(registrations))
		}
	})

	t.Run("independent catalogs", func(t *testing.T) {
		first := RegistrationCheckers()
		second := RegistrationCheckers()
		if first == nil || second == nil {
			t.Fatal("RegistrationCheckers returned nil")
		}
		first["test-only"] = nil
		if len(second) != 0 {
			t.Fatal("changing the first catalog changed the second")
		}
		third := RegistrationCheckers()
		if third == nil || len(third) != 0 {
			t.Fatal("a subsequent call did not return an empty non-nil catalog")
		}
		if _, exists := first["test-only"]; !exists {
			t.Fatal("a subsequent call changed the caller-owned catalog")
		}
	})

	t.Run("creates empty registry", func(t *testing.T) {
		registry, err := diagnostics.NewRegistry(RegistrationCheckers())
		if err != nil {
			t.Fatalf("NewRegistry returned an error: %v", err)
		}
		if registry == nil {
			t.Fatal("NewRegistry returned nil")
		}
		if registry.Len() != 0 {
			t.Fatalf("registry length = %d, want 0", registry.Len())
		}
	})
}

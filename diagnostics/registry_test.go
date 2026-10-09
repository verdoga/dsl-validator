package diagnostics

import (
	"errors"
	"slices"
	"strings"
	"testing"

	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
)

func registryDefinition(t *testing.T, code Code) Definition {
	t.Helper()
	return Definition{Code: code, Severity: SeverityError, Scope: ScopeDocument, Message: " \tсообщение\n",
		Versions: VersionRule{Mode: VersionAll}, Check: func(Lines, Findings) error {
			t.Error("registry invoked Check")
			return nil
		}}
}

func registryWithDefinitions(t *testing.T, definitions ...Definition) *Registry {
	t.Helper()
	r, err := NewRegistry(map[string]RegistrationFunc{"checks": func(r *Registry) error {
		for _, definition := range definitions {
			if err := r.Register(definition); err != nil {
				return err
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestValidateDefinitionChecksCodeFormat(t *testing.T) {
	cases := []struct {
		codes []Code
		valid bool
	}{
		{[]Code{"V001", "V009", "V010", "V099", "V100", "V999", "V1000", Code("V" + strings.Repeat("9", 100))}, true},
		{[]Code{"", "V", "V1", "V01", "V000", "V0000", "V0001", "V0999", "P001", "IO001", "v001", "V-01", "V+01", "V1.0", "V0a1", "V００１", " V001", "V001\n"}, false},
	}
	for _, tc := range cases {
		for _, code := range tc.codes {
			t.Run(string(code), func(t *testing.T) {
				if err := validateDefinition(registryDefinition(t, code)); (err == nil) != tc.valid {
					t.Errorf("code %q: error = %v, want valid: %t", code, err, tc.valid)
				}
			})
		}
	}
}

func TestRegisterValidatesDefinitionWithoutPartialAddition(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Definition)
		valid  bool
	}{
		{"error document", func(*Definition) {}, true},
		{"warning element", func(d *Definition) { d.Severity, d.Scope = SeverityWarning, ScopeElement }, true},
		{"recommendation line", func(d *Definition) { d.Severity, d.Scope = SeverityRecommendation, ScopeLine }, true},
		{"block", func(d *Definition) { d.Scope = ScopeBlock }, true},
		{"invalid code", func(d *Definition) { d.Code = "P001" }, false},
		{"empty severity", func(d *Definition) { d.Severity = "" }, false},
		{"unknown severity", func(d *Definition) { d.Severity = "fatal" }, false},
		{"empty scope", func(d *Definition) { d.Scope = "" }, false},
		{"unknown scope", func(d *Definition) { d.Scope = "file" }, false},
		{"empty message", func(d *Definition) { d.Message = "" }, false},
		{"whitespace message", func(d *Definition) { d.Message = " \t\r\n\u00a0\u2003" }, false},
		{"missing check", func(d *Definition) { d.Check = nil }, false},
		{"missing version mode", func(d *Definition) { d.Versions = VersionRule{} }, false},
		{"unknown version mode", func(d *Definition) { d.Versions.Mode = 255 }, false},
		{"all with boundary", func(d *Definition) { d.Versions.Version = "1.2" }, false},
		{"missing boundary", func(d *Definition) { d.Versions.Mode = VersionExact }, false},
		{"invalid boundary", func(d *Definition) { d.Versions = VersionRule{VersionFrom, "01.2"} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRegistry(map[string]RegistrationFunc{"check": func(r *Registry) error {
				definition := registryDefinition(t, "V001")
				tc.change(&definition)
				err := r.Register(definition)
				wantLen := 0
				if tc.valid {
					wantLen = 1
				}
				if (err == nil) != tc.valid || r.Len() != wantLen {
					t.Errorf("Register: error = %v, length = %d, want valid: %t", err, r.Len(), tc.valid)
				}
				return err
			}})
			if (err == nil) != tc.valid || (r != nil) != tc.valid {
				t.Errorf("NewRegistry: registry = %v, error = %v, want valid: %t", r, err, tc.valid)
			}
		})
	}
}

func TestNewRegistryHandlesEmptyAndInvalidRegistrations(t *testing.T) {
	noop := func(*Registry) error { return nil }
	cases := []struct {
		name          string
		registrations map[string]RegistrationFunc
		valid         bool
	}{
		{"nil map", nil, true}, {"empty map", map[string]RegistrationFunc{}, true},
		{"no definitions", map[string]RegistrationFunc{"noop": noop}, true},
		{"empty name", map[string]RegistrationFunc{"": noop}, false},
		{"nil callback", map[string]RegistrationFunc{"nil": nil}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRegistry(tc.registrations)
			if (err == nil) != tc.valid || (r != nil) != tc.valid {
				t.Fatalf("NewRegistry: registry = %v, error = %v, want valid: %t", r, err, tc.valid)
			}
			if r != nil {
				checks, err := r.ForVersion(dslversions.V1_2)
				if r.Len() != 0 || len(checks) != 0 || err != nil || r.Register(registryDefinition(t, "V001")) == nil {
					t.Errorf("empty registry is not closed and selectable: checks = %v, error = %v", checks, err)
				}
			}
		})
	}
}

func TestRegistryPreservesCopiesOrderAndRegistrationMap(t *testing.T) {
	large := Code("V" + strings.Repeat("9", 100))
	codes := []Code{large, "V1000", "V010", "V999", "V002", "V005"}
	definition := registryDefinition(t, "V001")
	calls := 0
	registrations := map[string]RegistrationFunc{
		"one": func(r *Registry) error {
			calls++
			return r.Register(definition)
		},
		"many": func(r *Registry) error {
			calls++
			for _, code := range codes {
				entry := registryDefinition(t, code)
				if code == "V005" {
					entry.Versions = VersionRule{VersionExact, "1.10"}
				}
				if err := r.Register(entry); err != nil {
					return err
				}
			}
			return nil
		},
	}
	registries := make([]*Registry, 2)
	for i := range registries {
		var err error
		registries[i], err = NewRegistry(registrations)
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls != 4 || len(registrations) != 2 || registrations["one"] == nil || registrations["many"] == nil {
		t.Fatal("registration map changed or callbacks were not called once per construction")
	}
	definition = Definition{}
	clear(registrations)
	other := registryWithDefinitions(t, registryDefinition(t, "V007"))
	if other.Len() != 1 {
		t.Fatal("new registry inherited earlier registrations")
	}
	first, err := registries[0].ForVersion(dslversions.V1_2)
	if err != nil {
		t.Fatal(err)
	}
	clear(first)
	for _, r := range registries {
		if err := r.Register(registryDefinition(t, "V003")); err == nil {
			t.Error("sealed registry accepted registration")
		}
		checks, err := r.ForVersion(dslversions.V1_2)
		if err != nil || len(checks) != 6 || r.Len() != 7 {
			t.Fatalf("registry changed: checks = %v, length = %d, error = %v", checks, r.Len(), err)
		}
		got := make([]Code, len(checks))
		for i, check := range checks {
			got[i] = check.Code
			if check.Message != " \tсообщение\n" || check.Check == nil || check.Severity != SeverityError || check.Scope != ScopeDocument || check.Versions != (VersionRule{Mode: VersionAll}) {
				t.Errorf("definition was changed: %+v", check)
			}
		}
		if want := []Code{"V001", "V002", "V010", "V999", "V1000", large}; !slices.Equal(got, want) {
			t.Errorf("codes = %v, want %v", got, want)
		}
	}
}

func TestNewRegistryStopsOnFailureAndClosesPartialRegistry(t *testing.T) {
	for _, scenario := range []string{"callback error", "ignored invalid definition", "ignored duplicate", "duplicate across callbacks"} {
		t.Run(scenario, func(t *testing.T) {
			var captured *Registry
			var cause error
			calls := 0
			registration := func(r *Registry) error {
				captured = r
				calls++
				definition := registryDefinition(t, "V001")
				if calls > 1 {
					definition.Versions = VersionRule{VersionExact, "1.2"}
				}
				if err := r.Register(definition); err != nil {
					cause = err
					return err
				}
				if scenario == "duplicate across callbacks" {
					return nil
				}
				if scenario == "callback error" {
					cause = errors.New("registration failure")
					return cause
				}
				if scenario == "ignored invalid definition" {
					definition.Code, definition.Message = "V002", " "
				}
				cause = r.Register(definition)
				if err := r.Register(registryDefinition(t, "V004")); cause == nil || !errors.Is(err, cause) || r.Len() != 1 {
					t.Errorf("first registration failure was not retained: %v", err)
				}
				return nil
			}
			r, err := NewRegistry(map[string]RegistrationFunc{"one": registration, "two": registration, "three": registration})
			wantCalls := 1
			if scenario == "duplicate across callbacks" {
				wantCalls = 2
			}
			if r != nil || err == nil || cause == nil || !errors.Is(err, cause) || calls != wantCalls {
				t.Fatalf("registry = %v, error = %v, cause = %v, calls = %d", r, err, cause, calls)
			}
			if captured.Len() != 1 || captured.Register(registryDefinition(t, "V003")) == nil || captured.Len() != 1 {
				t.Error("failed registry changed or accepted further registration")
			}
			if checks, err := captured.ForVersion(dslversions.V1_2); checks != nil || err == nil {
				t.Errorf("partial registry published: %v, %v", checks, err)
			}
		})
	}
}

func TestForVersionFiltersOnlySupportedVersions(t *testing.T) {
	rules := []struct {
		name    string
		rule    VersionRule
		wantLen int
	}{
		{"all", VersionRule{VersionAll, ""}, 1},
		{"exact match", VersionRule{VersionExact, "1.2"}, 1},
		{"exact miss", VersionRule{VersionExact, "1.10"}, 0},
		{"from boundary", VersionRule{VersionFrom, "1.2"}, 1},
		{"from older", VersionRule{VersionFrom, "1.1"}, 1},
		{"from newer", VersionRule{VersionFrom, "1.10"}, 0},
		{"through boundary", VersionRule{VersionThrough, "1.2"}, 1},
		{"through older", VersionRule{VersionThrough, "1.1"}, 0},
		{"through newer", VersionRule{VersionThrough, "1.10"}, 1},
	}
	for _, tc := range rules {
		t.Run(tc.name, func(t *testing.T) {
			definition := registryDefinition(t, "V001")
			definition.Versions = tc.rule
			r := registryWithDefinitions(t, definition)
			checks, err := r.ForVersion(dslversions.V1_2)
			if err != nil || len(checks) != tc.wantLen {
				t.Errorf("rule %+v: checks = %v, error = %v", tc.rule, checks, err)
			}
			for _, version := range []dslversions.Version{"", "1.1", "1.10", "0.0", "01.2", "v1.2", "1.18446744073709551616"} {
				if checks, err := r.ForVersion(version); checks != nil || err == nil {
					t.Errorf("unsupported version %q: checks = %v, error = %v", version, checks, err)
				}
			}
		})
	}
}

func TestRegistryRejectsUseBeforeConstructionCompletes(t *testing.T) {
	for _, r := range []*Registry{nil, {}} {
		if checks, err := r.ForVersion(dslversions.V1_2); checks != nil || err == nil {
			t.Errorf("unprepared registry: checks = %v, error = %v", checks, err)
		}
		if err := r.Register(registryDefinition(t, "V001")); err == nil {
			t.Error("Register accepted a registry outside its constructor")
		}
	}
	_, err := NewRegistry(map[string]RegistrationFunc{"check": func(r *Registry) error {
		if checks, err := r.ForVersion(dslversions.V1_2); checks != nil || err == nil {
			t.Errorf("unfinished registry published: checks = %v, error = %v", checks, err)
		}
		return r.Register(registryDefinition(t, "V001"))
	}})
	if err != nil {
		t.Fatal(err)
	}
}

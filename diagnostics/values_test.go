package diagnostics

import (
	"encoding/json"
	"testing"
)

func TestSeverityPreservesMachineAndJSONValues(t *testing.T) {
	cases := []struct {
		name     string
		severity Severity
		want     string
	}{
		{"error", SeverityError, "error"},
		{"warning", SeverityWarning, "warning"},
		{"recommendation", SeverityRecommendation, "recommendation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(tc.severity); got != tc.want {
				t.Errorf("severity = %q, want %q", got, tc.want)
			}
			encoded, err := json.Marshal(tc.severity)
			if err != nil {
				t.Fatalf("json.Marshal returned an error: %v", err)
			}
			if got, want := string(encoded), `"`+tc.want+`"`; got != want {
				t.Errorf("JSON severity = %s, want %s", got, want)
			}
		})
	}
}

func TestScopePreservesMachineAndJSONValues(t *testing.T) {
	cases := []struct {
		name  string
		scope Scope
		want  string
	}{
		{"element", ScopeElement, "element"},
		{"line", ScopeLine, "line"},
		{"block", ScopeBlock, "block"},
		{"document", ScopeDocument, "document"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(tc.scope); got != tc.want {
				t.Errorf("scope = %q, want %q", got, tc.want)
			}
			encoded, err := json.Marshal(tc.scope)
			if err != nil {
				t.Fatalf("json.Marshal returned an error: %v", err)
			}
			if got, want := string(encoded), `"`+tc.want+`"`; got != want {
				t.Errorf("JSON scope = %s, want %s", got, want)
			}
		})
	}
}

func TestCodePreservesLegacyAndValidatorCodes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"legacy parser code", "P001"},
		{"legacy IO code", "IO001"},
		{"validator code", "V001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := Code(tc.raw)
			if got := string(code); got != tc.raw {
				t.Errorf("code = %q, want %q", got, tc.raw)
			}
			encoded, err := json.Marshal(code)
			if err != nil {
				t.Fatalf("json.Marshal returned an error: %v", err)
			}
			if got, want := string(encoded), `"`+tc.raw+`"`; got != want {
				t.Errorf("JSON code = %s, want %s", got, want)
			}
			var decoded Code
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("json.Unmarshal returned an error: %v", err)
			}
			if decoded != code {
				t.Errorf("decoded code = %q, want %q", decoded, code)
			}
		})
	}
}

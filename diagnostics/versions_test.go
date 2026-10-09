package diagnostics

import (
	"testing"

	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
)

func TestValidateVersionRuleChecksModeAndAllBoundary(t *testing.T) {
	cases := []struct {
		name    string
		rule    VersionRule
		wantErr bool
	}{
		{"zero rule", VersionRule{}, true},
		{"zero mode with version", VersionRule{Version: "1.2"}, true},
		{"unknown mode without version", VersionRule{Mode: 5}, true},
		{"unknown mode with version", VersionRule{Mode: 5, Version: "1.2"}, true},
		{"maximum mode", VersionRule{Mode: 255, Version: "1.2"}, true},
		{"all without boundary", VersionRule{Mode: VersionAll}, false},
		{"all with supported boundary", VersionRule{Mode: VersionAll, Version: "1.2"}, true},
		{"all with unsupported boundary", VersionRule{Mode: VersionAll, Version: "1.10"}, true},
		{"all with invalid boundary", VersionRule{Mode: VersionAll, Version: "invalid"}, true},
		{"all with whitespace boundary", VersionRule{Mode: VersionAll, Version: " "}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVersionRule(tc.rule)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateVersionRule(%+v) error = %v, want error: %t", tc.rule, err, tc.wantErr)
			}
		})
	}
}

func TestValidateVersionRuleRequiresCanonicalBoundary(t *testing.T) {
	modes := []struct {
		name string
		mode VersionMode
	}{
		{"exact", VersionExact},
		{"from", VersionFrom},
		{"through", VersionThrough},
	}
	cases := []struct {
		name    string
		version dslversions.Version
		wantErr bool
	}{
		{"supported version", "1.2", false},
		{"unsupported older version", "1.1", false},
		{"unsupported newer version", "1.10", false},
		{"zero components", "0.0", false},
		{"maximum components", "18446744073709551615.18446744073709551615", false},
		{"empty", "", true},
		{"missing separator", "12", true},
		{"missing major", ".2", true},
		{"missing minor", "1.", true},
		{"third component", "1.2.0", true},
		{"leading zero major", "01.2", true},
		{"leading zero minor", "1.02", true},
		{"leading whitespace", " 1.2", true},
		{"trailing whitespace", "1.2\t", true},
		{"whitespace only", " \t\n", true},
		{"prefix", "v1.2", true},
		{"suffix", "1.2-beta", true},
		{"positive sign", "+1.2", true},
		{"negative component", "1.-2", true},
		{"unicode digit", "1.２", true},
		{"major overflow", "18446744073709551616.2", true},
		{"minor overflow", "1.18446744073709551616", true},
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					rule := VersionRule{Mode: mode.mode, Version: tc.version}
					err := validateVersionRule(rule)
					if (err != nil) != tc.wantErr {
						t.Errorf("validateVersionRule(%+v) error = %v, want error: %t", rule, err, tc.wantErr)
					}
				})
			}
		})
	}
}

func TestMatchesVersionUsesInclusiveNumericBoundaries(t *testing.T) {
	version, err := dslversions.Parse("1.2")
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if err := dslversions.RequireSupported(version); err != nil {
		t.Fatalf("document version is not supported: %v", err)
	}
	cases := []struct {
		name     string
		mode     VersionMode
		boundary string
		want     bool
	}{
		{"all", VersionAll, "", true},
		{"exact match", VersionExact, "1.2", true},
		{"exact older boundary", VersionExact, "1.1", false},
		{"exact newer boundary", VersionExact, "1.3", false},
		{"from inclusive boundary", VersionFrom, "1.2", true},
		{"from older boundary", VersionFrom, "1.1", true},
		{"from newer boundary", VersionFrom, "1.3", false},
		{"from numeric minor order", VersionFrom, "1.10", false},
		{"from older major", VersionFrom, "0.99", true},
		{"from newer major", VersionFrom, "2.0", false},
		{"through inclusive boundary", VersionThrough, "1.2", true},
		{"through older boundary", VersionThrough, "1.1", false},
		{"through newer boundary", VersionThrough, "1.3", true},
		{"through numeric minor order", VersionThrough, "1.10", true},
		{"through older major", VersionThrough, "0.99", false},
		{"through newer major", VersionThrough, "2.0", true},
		{"from maximum boundary", VersionFrom, "18446744073709551615.18446744073709551615", false},
		{"through maximum boundary", VersionThrough, "18446744073709551615.18446744073709551615", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := VersionRule{Mode: tc.mode}
			if tc.boundary != "" {
				boundary, err := dslversions.Parse(tc.boundary)
				if err != nil {
					t.Fatalf("Parse(%q) returned an error: %v", tc.boundary, err)
				}
				rule.Version = boundary
			}
			if err := validateVersionRule(rule); err != nil {
				t.Fatalf("rule is not valid: %v", err)
			}
			if got := matchesVersion(rule, version); got != tc.want {
				t.Errorf("matchesVersion(%+v, %q) = %t, want %t", rule, version, got, tc.want)
			}
		})
	}
}

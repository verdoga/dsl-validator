package dslversions

import "testing"

func TestSupportedReturnsIndependentCatalog(t *testing.T) {
	first := Supported()
	second := Supported()
	if len(first) != 1 || first[0] != V1_2 {
		t.Fatalf("Supported() = %v, want [%s]", first, V1_2)
	}
	if len(second) != 1 || second[0] != V1_2 {
		t.Fatalf("Supported() = %v, want [%s]", second, V1_2)
	}

	first[0] = "1.10"
	if second[0] != V1_2 {
		t.Errorf("modifying one catalog changed another: %v", second)
	}
	if got := Supported(); len(got) != 1 || got[0] != V1_2 {
		t.Errorf("Supported() after modification = %v, want [%s]", got, V1_2)
	}
	if err := RequireSupported(V1_2); err != nil {
		t.Errorf("RequireSupported(%q) after modification returned an error: %v", V1_2, err)
	}
	if err := RequireSupported(first[0]); err == nil {
		t.Errorf("modifying the returned catalog made %q supported", first[0])
	}
}

func TestRequireSupportedValidatesWithoutChangingVersion(t *testing.T) {
	cases := []struct {
		name    string
		version Version
		wantErr string
	}{
		{"supported version", V1_2, ""},
		{"empty", "", "версия DSL не задана не поддерживается; поддерживаются: 1.2"},
		{"missing separator", "12", "версия DSL 12 не поддерживается; поддерживаются: 1.2"},
		{"missing major", ".2", "версия DSL .2 не поддерживается; поддерживаются: 1.2"},
		{"missing minor", "1.", "версия DSL 1. не поддерживается; поддерживаются: 1.2"},
		{"third component", "1.2.0", "версия DSL 1.2.0 не поддерживается; поддерживаются: 1.2"},
		{"leading zero major", "01.2", "версия DSL 01.2 не поддерживается; поддерживаются: 1.2"},
		{"leading zero minor", "1.02", "версия DSL 1.02 не поддерживается; поддерживаются: 1.2"},
		{"positive sign", "+1.2", "версия DSL +1.2 не поддерживается; поддерживаются: 1.2"},
		{"negative sign", "1.-2", "версия DSL 1.-2 не поддерживается; поддерживаются: 1.2"},
		{"leading space", " 1.2", "версия DSL  1.2 не поддерживается; поддерживаются: 1.2"},
		{"trailing space", "1.2 ", "версия DSL 1.2  не поддерживается; поддерживаются: 1.2"},
		{"whitespace only", " ", "версия DSL   не поддерживается; поддерживаются: 1.2"},
		{"prefix", "v1.2", "версия DSL v1.2 не поддерживается; поддерживаются: 1.2"},
		{"suffix", "1.2-beta", "версия DSL 1.2-beta не поддерживается; поддерживаются: 1.2"},
		{"nondecimal digit", "1.２", "версия DSL 1.２ не поддерживается; поддерживаются: 1.2"},
		{"major overflow", "18446744073709551616.2", "версия DSL 18446744073709551616.2 не поддерживается; поддерживаются: 1.2"},
		{"minor overflow", "1.18446744073709551616", "версия DSL 1.18446744073709551616 не поддерживается; поддерживаются: 1.2"},
		{"zero version", "0.0", "версия DSL 0.0 не поддерживается; поддерживаются: 1.2"},
		{"older minor", "1.1", "версия DSL 1.1 не поддерживается; поддерживаются: 1.2"},
		{"newer minor", "1.10", "версия DSL 1.10 не поддерживается; поддерживаются: 1.2"},
		{"newer major", "2.0", "версия DSL 2.0 не поддерживается; поддерживаются: 1.2"},
		{"maximum components", "18446744073709551615.18446744073709551615", "версия DSL 18446744073709551615.18446744073709551615 не поддерживается; поддерживаются: 1.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version := tc.version
			err := RequireSupported(version)
			if version != tc.version {
				t.Errorf("RequireSupported changed version from %q to %q", tc.version, version)
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("RequireSupported(%q) returned an error: %v", version, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("RequireSupported(%q) returned no error", version)
			}
			if got := err.Error(); got != tc.wantErr {
				t.Errorf("RequireSupported(%q) error = %q, want %q", version, got, tc.wantErr)
			}
		})
	}
}

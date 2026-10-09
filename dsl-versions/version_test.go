package dslversions

import "testing"

func TestParsePreservesCanonicalVersion(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want Version
	}{
		{"zero", "0.0", Version("0.0")},
		{"current version", "1.2", V1_2},
		{"unsupported version", "1.10", Version("1.10")},
		{"zero major", "0.12", Version("0.12")},
		{"zero minor", "12.0", Version("12.0")},
		{"maximum major", "18446744073709551615.0", Version("18446744073709551615.0")},
		{"maximum minor", "0.18446744073709551615", Version("0.18446744073709551615")},
		{"maximum both", "18446744073709551615.18446744073709551615", Version("18446744073709551615.18446744073709551615")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if err != nil {
				t.Fatalf("Parse(%q) returned an error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("Parse(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseRejectsInvalidVersion(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"missing separator", "12"},
		{"missing both components", "."},
		{"missing major", ".2"},
		{"missing minor", "1."},
		{"third component", "1.2.3"},
		{"extra trailing separator", "1.2."},
		{"extra leading separator", ".1.2"},
		{"repeated separator", "1..2"},
		{"leading zero major", "01.2"},
		{"leading zero minor", "1.02"},
		{"multiple zero major", "00.2"},
		{"multiple zero minor", "1.00"},
		{"positive major", "+1.2"},
		{"positive minor", "1.+2"},
		{"negative major", "-1.2"},
		{"negative minor", "1.-2"},
		{"leading space", " 1.2"},
		{"trailing space", "1.2 "},
		{"space in major", "1 .2"},
		{"space in minor", "1. 2"},
		{"tab", "1.\t2"},
		{"newline", "1.2\n"},
		{"nonbreaking space", "1.2\u00a0"},
		{"version prefix", "v1.2"},
		{"hexadecimal major", "0x1.2"},
		{"hexadecimal minor", "1.0x2"},
		{"prerelease suffix", "1.2-beta"},
		{"build suffix", "1.2+build"},
		{"letter major", "a.2"},
		{"letter minor", "1.b"},
		{"exponent", "1.2e3"},
		{"underscore major", "1_0.2"},
		{"underscore minor", "1.2_0"},
		{"unicode digit major", "١.2"},
		{"unicode digit minor", "1.２"},
		{"nul byte", "1.2\x00"},
		{"invalid utf8", "1.\xff"},
		{"overflow major", "18446744073709551616.0"},
		{"overflow minor", "0.18446744073709551616"},
		{"large overflow major", "999999999999999999999999999999.2"},
		{"large overflow minor", "1.999999999999999999999999999999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if err == nil {
				t.Errorf("Parse(%q) returned no error", tc.raw)
			}
			if got != "" {
				t.Errorf("Parse(%q) = %q, want an empty version", tc.raw, got)
			}
		})
	}
}

func TestCompareUsesNumericOrder(t *testing.T) {
	cases := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{"equal zero", "0.0", "0.0", 0},
		{"equal current version", "1.2", "1.2", 0},
		{"equal unsupported version", "1.10", "1.10", 0},
		{"numeric minor order", "1.2", "1.10", -1},
		{"numeric major order", "2.0", "10.0", -1},
		{"major takes precedence", "2.0", "1.10", 1},
		{"zero minor", "1.0", "1.1", -1},
		{"zero major", "0.1", "1.0", -1},
		{"maximum major against zero", "18446744073709551615.0", "0.0", 1},
		{"maximum minor against zero", "0.18446744073709551615", "0.0", 1},
		{"adjacent maximum majors", "18446744073709551614.0", "18446744073709551615.0", -1},
		{"adjacent maximum minors", "1.18446744073709551614", "1.18446744073709551615", -1},
		{"major overrides maximum minor", "1.0", "0.18446744073709551615", 1},
		{"equal maximum components", "18446744073709551615.18446744073709551615", "18446744073709551615.18446744073709551615", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			left, err := Parse(tc.left)
			if err != nil {
				t.Fatalf("Parse(%q) returned an error: %v", tc.left, err)
			}
			right, err := Parse(tc.right)
			if err != nil {
				t.Fatalf("Parse(%q) returned an error: %v", tc.right, err)
			}
			if got := Compare(left, right); got != tc.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", left, right, got, tc.want)
			}
			if got := Compare(right, left); got != -tc.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", right, left, got, -tc.want)
			}
		})
	}
}

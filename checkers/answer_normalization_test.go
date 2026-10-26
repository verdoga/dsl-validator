package checkers

import (
	"strings"
	"testing"
)

func TestAnswerPrimary(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"Unicode lowercase", "ЁЖ ÉΛΣ", "ёж éλσ"},
		{"mixed Unicode whitespace", "\u00a0\t YES\r\n\u0085\u1680\u2003\u2028\u2029\u202f\u205f\u3000 No \v\f", "yes no"},
		{"punctuation remains", " \tYES! (No?)\u00a0", "yes! (no?)"},
		{"letters and combining marks remain", "ſ É E\u0301", "ſ é e\u0301"},
		{"digits and symbols remain", "A1 + $ = 😀", "a1 + $ = 😀"},
		{"empty", "", ""},
		{"whitespace only", " \t\r\n\u00a0\u2003\u202f", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := answerPrimary(tc.input); got != tc.want {
				t.Errorf("answerPrimary(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestAnswerEqual(t *testing.T) {
	for _, tc := range []struct {
		name, left, right string
		want              bool
	}{
		{"case and punctuation", "YES!", "Yes", true},
		{"Unicode lowercase", "ЁЖ ÉΛΣ", "ёж éλσ", true},
		{"mixed Unicode whitespace", "\u00a0YES\t\u2003\nNo\u202f", "yes no", true},
		{"identical punctuation", "!", "!", true},
		{"identical punctuation with whitespace", "\u00a0! \t", "!", true},
		{"different punctuation only", "!", "?", false},
		{"different punctuation sequences", "!!", "!", false},
		{"punctuation versus empty", "!", "", false},
		{"punctuation versus whitespace", "!", " \t\u00a0", false},
		{"both empty", "", "", false},
		{"both whitespace", "\t\u00a0", "\n\u2003", false},
		{"empty versus text", "", "yes", false},
		{"whitespace versus text", " \t\u00a0", "yes", false},
		{"Unicode punctuation categories", "_—(«YES»)…", "yes", true},
		{"Unicode closing quote", "‘Yes’", "yes", true},
		{"collapse whitespace after punctuation removal", "One \t—\u00a0\n two", "one two", true},
		{"trim after punctuation removal", "!\u00a0YES\t?", "yes", true},
		{"punctuation joins adjacent letters", "a-b", "ab", true},
		{"punctuation does not insert whitespace", "a-b", "a b", false},
		{"long s is not case folded", "ſ", "S", false},
		{"sharp s is not expanded", "ß", "SS", false},
		{"diacritic remains", "é", "e", false},
		{"combining mark remains", "e\u0301!", "e", false},
		{"Unicode composition is not changed", "é", "e\u0301", false},
		{"digits remain", "a1", "a2", false},
		{"plus remains", "a+b", "ab", false},
		{"currency symbol remains", "$yes", "yes", false},
		{"math symbol remains", "yes=", "yes", false},
		{"emoji remains", "yes😀", "yes", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, order := range []struct {
				name, left, right string
			}{
				{"forward", tc.left, tc.right},
				{"reverse", tc.right, tc.left},
			} {
				t.Run(order.name, func(t *testing.T) {
					if got := answerEqual(order.left, order.right); got != tc.want {
						t.Errorf("answerEqual(%q, %q) = %t, want %t", order.left, order.right, got, tc.want)
					}
				})
			}
		})
	}
}

func TestAnswerPosition(t *testing.T) {
	huge := "1" + strings.Repeat("234567890", 512)
	for _, tc := range []struct {
		name, input, want string
		valid             bool
	}{
		{"one", "1", "1", true},
		{"multiple digits", "120", "120", true},
		{"trailing period", "1.", "1", true},
		{"parentheses", "(1)", "1", true},
		{"hyphen is punctuation", "-1", "1", true},
		{"decimal punctuation is removed", "1.0", "10", true},
		{"Unicode punctuation", "«—1»", "1", true},
		{"punctuation between digits", "1-2", "12", true},
		{"edge NBSP", "\u00a01\u00a0", "1", true},
		{"edge Unicode whitespace", "\u202f\t(1)\n\u2003", "1", true},
		{"trim whitespace after punctuation removal", "(\u00a01\u00a0)", "1", true},
		{"huge canonical number", huge, huge, true},
		{"huge punctuated number", "\u00a0«" + huge + "»\u00a0", huge, true},
		{"plus sign", "+1", "", false},
		{"leading zero", "01", "", false},
		{"punctuated leading zero", "(01)", "", false},
		{"huge leading zero", "0" + huge, "", false},
		{"zero", "0", "", false},
		{"punctuated zero", "(0)", "", false},
		{"internal ASCII space", "1 2", "", false},
		{"internal NBSP", "1\u00a02", "", false},
		{"space after punctuation removal", "1 \t—\u00a0 2", "", false},
		{"Arabic Indic digit", "١", "", false},
		{"fullwidth digit", "１", "", false},
		{"mixed ASCII and Unicode digits", "1٢", "", false},
		{"Unicode math minus remains", "−1", "", false},
		{"letter remains", "a1", "", false},
		{"empty", "", "", false},
		{"whitespace only", "\t\u00a0\n", "", false},
		{"punctuation only", "(!)", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, valid := answerPosition(tc.input)
			if got != tc.want || valid != tc.valid {
				t.Errorf("answerPosition(%q) = (%q, %t), want (%q, %t)", tc.input, got, valid, tc.want, tc.valid)
			}
		})
	}
}

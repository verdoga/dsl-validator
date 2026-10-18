package checkers

import "testing"

func TestV018AddressBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"URL with port and query", "https://example.org:443/a.b?q=x:y&n=1,2#part", nil},
		{"uppercase scheme", "HTTPS://EXAMPLE.ORG/a.b", nil},
		{"mail plus suffix", "user+tag@example.org", nil},
		{"quoted URL", "'https://example.org'", nil},
		{"final punctuation", "(https://example.org). Next", nil},
		{"URL after missing space", "See:https://example.org", [][3]int{{3, 4, 5}}},
		{"email after missing space", "Mail:user@example.org", [][3]int{{3, 5, 6}}},
		{"email before missing space", "user@example.org Next,word", [][3]int{{3, 22, 23}}},
		{"empty host is not address", "https://.org", [][3]int{{3, 6, 7}, {3, 9, 10}}},
		{"abbreviation remains text", "e.g.Next", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"ordinary slash remains text", "/a,b/", [][3]int{{3, 3, 4}}},
		{"unmatched double closing quote", "Ready.\" Next", nil},
		{"opening double quote", "Ready,\"Next\"", [][3]int{{3, 6, 7}}},
		{"mixed closing quote before bracket", "(‘Ready.') Next", nil},
		{"space still required after bracket", "(‘Ready.')Next", [][3]int{{3, 8, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v018Run(t, v018Document(t, v018Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	for _, separate := range []bool{false, true} {
		v018Run(t, v015AnswerDocument(t, "", "https://example.org", "Next,word", separate), "current", [][3]int{{4, 31, 32}})
		v018Run(t, v015AnswerDocument(t, "", "user@example.org", "", separate), "current", nil)
	}
}

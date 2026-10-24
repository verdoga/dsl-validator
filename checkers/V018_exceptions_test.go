package checkers

import "testing"

func TestV018EllipsisGroups(t *testing.T) {
	for _, group := range []string{"?...", "!...", "?..", "!..", "?…", "!…", "?!…"} {
		t.Run(group, func(t *testing.T) {
			for _, raw := range []string{group, "Why" + group, "Why" + group + " Next", "«Why" + group + "» Next", "'Why" + group + "' Next"} {
				v018Run(t, v018Document(t, v018Text(t, raw, 2, 1)), "current", nil)
			}
		})
	}
	for _, tc := range []struct {
		raw  string
		want [][3]int
	}{
		{"Why?…Next", [][3]int{{3, 5, 6}}},
		{"Why!…Next", [][3]int{{3, 5, 6}}},
		{"Why?..Next", [][3]int{{3, 6, 7}}},
		{"Why!...Next", [][3]int{{3, 7, 8}}},
		{"'Why?…'Next", [][3]int{{3, 6, 7}}},
		{"Why?… Next,word", [][3]int{{3, 11, 12}}},
		// Исключение перед … не распространяется на другие знаки.
		{"Hi.…", [][3]int{{3, 3, 4}}},
		{"Hi,…", [][3]int{{3, 3, 4}}},
		{"Hi:…", [][3]int{{3, 3, 4}}},
		{"Hi;…", [][3]int{{3, 3, 4}}},
		{"Hi?.…", [][3]int{{3, 4, 5}}},
		{"Hi!,…", [][3]int{{3, 4, 5}}},
		{"Hi…Next", nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v018Run(t, v018Document(t, v018Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV018Abbreviations(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want [][3]int
	}{
		{"e.g.", nil},
		{"e.g. word", nil},
		{"i.e. word", nil},
		{"E.G. word", nil},
		{"т.е. слово", nil},
		{"«e.g.» word", nil},
		{"e.g.Next", [][3]int{{3, 4, 5}}},
		{"e.g. Next,word", [][3]int{{3, 10, 11}}},
		{"word.g. next", [][3]int{{3, 5, 6}}},
		{"e.word", [][3]int{{3, 2, 3}}},
		{"1.a. next", [][3]int{{3, 2, 3}}},
		{"a.1. next", [][3]int{{3, 2, 3}}},
		{"e.g.…", [][3]int{{3, 4, 5}}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v018Run(t, v018Document(t, v018Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV018ExceptionsInMarkupAndAnswers(t *testing.T) {
	for _, raw := range []string{"Why?<em>…</em> Next", "e<em>.</em>g. word"} {
		v018Run(t, v020TextBlock(t, raw), "current", nil)
	}
	for _, separate := range []bool{false, true} {
		for _, answer := range []string{"Why?…", "Why!...", "e.g. word"} {
			v018Run(t, v015AnswerDocument(t, "", answer, "Next", separate), "current", nil)
		}
		v018Run(t, v015AnswerDocument(t, "", "e.g.", "Next,word", separate), "current", [][3]int{{4, 16, 17}})
	}
	// Сокращение не собирается через границу самостоятельного ответа.
	v018Run(t, v015AnswerDocument(t, "e.", "g.", "", true), "current", [][3]int{{4, 2, 3}})
}

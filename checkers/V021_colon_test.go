package checkers

import (
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

func TestV021ColonTerminatesLineInEveryTargetBlock(t *testing.T) {
	for _, tag := range []string{"multifill", "text", "key", "note", "alt", "script", "instr"} {
		t.Run(tag, func(t *testing.T) {
			for _, intro := range []string{
				"Write an email to a friend. Give information about:",
				"Topics:", "«Topics:»", "[Topics:]", `"Topics:"`, "'Topics:'",
			} {
				v021Run(t, v021RowsDocument(t, tag, []string{intro, "where you are now"}), "current", nil)
			}
			// Двоеточие не отключает проверку последующего обычного текста.
			v021Run(t, v021RowsDocument(t, tag, []string{"Topics:", "first,", "second"}),
				"current", [][4]int{{5, 1, 6, 7}})
			v021Run(t, v021RowsDocument(t, tag, []string{"Start;", "next"}),
				"current", [][4]int{{4, 1, 5, 5}})
			v021Run(t, v021RowsDocument(t, tag, []string{"Start: here", "next"}),
				"current", [][4]int{{4, 1, 5, 5}})
		})
	}
	// Только text поддерживает прозрачное HTML-форматирование.
	v021Run(t, v021RowsDocument(t, "text", []string{"<em>Topics:</em>", "next"}), "current", nil)
}

func TestV021InstrConsecutiveTopics(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
		want [][4]int
	}{
		{"reference", []string{
			"Write an email to a friend. Give information about:",
			"where you are now", "where you study English", "your teacher",
			"the other students in your class", "what you do in the evening.",
		}, nil},
		{"two topics", []string{"Topics:", "first", "second"}, nil},
		{"final period", []string{"Topics:", "first", "second."}, nil},
		{"Unicode phrases", []string{"Расскажи о:", "своём учителе", "своих друзьях."}, nil},
		{"names and apostrophes", []string{"Write about:", "John's teacher", "Mary's class."}, nil},
		{"without colon", []string{"Topics", "first", "second."}, [][4]int{{4, 1, 5, 6}, {5, 1, 6, 8}}},
		{"blank after colon", []string{"Topics:", "", "first", "second"}, [][4]int{{6, 1, 7, 7}}},
		{"blank between topics", []string{"Topics:", "first", "", "second"}, [][4]int{{5, 1, 7, 7}}},
		{"blank after list", []string{"Topics:", "first", "second", "", "tail", "more"}, [][4]int{{8, 1, 9, 5}}},
		{"restart after period", []string{"Topics:", "first", "second.", "tail", "more"}, [][4]int{{7, 1, 8, 5}}},
		{"single completed topic", []string{"Topics:", "first.", "tail", "more"}, [][4]int{{6, 1, 7, 5}}},
		{"other punctuation", []string{"Topics:", "first", "second,", "tail"}, [][4]int{{5, 1, 6, 8}, {6, 1, 7, 5}}},
		{"two lists", []string{"Topics:", "first", "second.", "More topics:", "third", "fourth."}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v021RowsDocument(t, "instr", tc.rows)
			v021Run(t, result, "first", tc.want)
			v021Run(t, result, "second", tc.want)
		})
	}
}

func TestV021TopicListExceptionIsInstrOnly(t *testing.T) {
	for _, tag := range []string{"multifill", "text", "key", "note", "alt", "script"} {
		t.Run(tag, func(t *testing.T) {
			v021Run(t, v021RowsDocument(t, tag, []string{"Topics:", "first", "second."}),
				"current", [][4]int{{5, 1, 6, 8}})
		})
	}
}

func TestV021WordlistRemainsExcluded(t *testing.T) {
	v021Run(t, v021RowsDocument(t, "wordlist", []string{"green", "red", "blue", "black"}), "current", nil)
	v021Run(t, v021RowsDocument(t, "text", []string{"green", "red", "blue", "black"}),
		"current", [][4]int{{4, 1, 5, 4}, {5, 1, 6, 5}, {6, 1, 7, 6}})
}

func TestV021InstrTopicsDoNotCrossConstructions(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"inline hint", []model.Line{
			v021Open(t, "instr", 2, 1), v021Text(t, "Topics:", 3, 2), v021Text(t, "first", 3, 2),
			v021Line(t, "@hint Hint.", "tag", 3, 2,
				v021Element(t, "tag", "@hint", "hint", 1, 6), v021Element(t, "content", "Hint.", "Hint.", 7, 12)),
			v021Text(t, "second", 3, 2), v021Text(t, "third", 3, 2), v021Close(t, 3, 2),
		}, [][4]int{{7, 1, 8, 6}}},
		{"nested hint", []model.Line{
			v021Open(t, "instr", 2, 1), v021Text(t, "Topics:", 3, 2), v021Text(t, "first", 3, 2),
			v021Open(t, "hint", 3, 2), v021Text(t, "Hint.", 6, 3), v021Close(t, 6, 3),
			v021Text(t, "second", 3, 2), v021Text(t, "third", 3, 2), v021Close(t, 3, 2),
		}, [][4]int{{9, 1, 10, 6}}},
		{"different instr blocks", []model.Line{
			v021Open(t, "instr", 2, 1), v021Text(t, "Topics:", 3, 2), v021Text(t, "first", 3, 2), v021Close(t, 3, 2),
			v021Open(t, "instr", 2, 1), v021Text(t, "second", 7, 2), v021Text(t, "third", 7, 2), v021Close(t, 7, 2),
		}, [][4]int{{8, 1, 9, 6}}},
		{"colon in hint does not introduce list", []model.Line{
			v021Open(t, "instr", 2, 1), v021Open(t, "hint", 3, 2),
			v021Text(t, "Topics:", 4, 3), v021Close(t, 4, 3),
			v021Text(t, "first", 3, 2), v021Text(t, "second", 3, 2), v021Close(t, 3, 2),
		}, [][4]int{{7, 1, 8, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Document(t, tc.body...), "current", tc.want)
		})
	}
}

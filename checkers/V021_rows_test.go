package checkers

import (
	"encoding/json"
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

func TestV021ExplicitListMarkers(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		end                 int
	}{
		{"Russian preposition", "Мы остановились", "в этом доме.", 13},
		{"English article", "This sentence continues", "a little further.", 18},
		{"first line starts with short word", "я видел этот дом", "вчера вечером.", 15},
		{"abbreviation", "Он прочитал", "т.е. понял смысл.", 18},
		{"colon without separator", "Начало", "a:word", 7},
		{"parenthesis without separator", "Начало", "a)word", 7},
		{"letter parenthesis", "a) First item", "b) Second item", 0},
		{"Cyrillic parenthesis", "а) Первый пункт", "б) Второй пункт", 0},
		{"letter dot", "a. First item", "b. Second item", 0},
		{"letter colon", "a: First item", "b: Second item", 0},
		{"tab after marker", "Начало", "a)\tпункт", 0},
		{"NBSP after marker", "Начало", "a)\u00a0пункт", 0},
		{"bare marker", "a)", "continuation", 0},
		{"numeric marker", "1) First item", "continuation", 0},
		{"formatted marker", "Начало", "<em>a)</em> пункт", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want [][4]int
			if tc.end != 0 {
				want = [][4]int{{4, 1, 5, tc.end}}
			}
			v021Run(t, v021Block(t, "text", v021Text(t, tc.first, 3, 2), v021Text(t, tc.second, 3, 2)), "current", want)
		})
	}
}

func TestV021CorpusIndependentRows(t *testing.T) {
	data := []byte(`[
  {
    "name": "A:65-67",
    "tag": "multifill",
    "rows": [
      "I/we: I _____{'m} Camila. (= I am)",
      "I/we: We _____{'re} from Spain. (= we are)",
      "you: _____{are} you from the UK?"
    ]
  },
  {
    "name": "A:434-435",
    "tag": "note",
    "rows": [
      "the USA = the United States of America",
      "the UK = the United Kingdom"
    ]
  },
  {
    "name": "C:74-77",
    "tag": "key",
    "rows": [
      "hello (2)",
      "good afternoon (4)",
      "good morning (3)",
      "good evening (3)"
    ]
  },
  {
    "name": "C:310-320",
    "tag": "note",
    "rows": [
      "Capital letters",
      "We use capital letters (A, B, C, D ...):",
      "for names (Sophia Taylor, Ben Wilson, Maria González)",
      "for names of places (Toronto, Canada, High Street)",
      "for nationalities (Italian, British, Chinese)",
      "at the beginning of a sentence (My name's ...)",
      "",
      "Full stops",
      ". = full stop",
      "We usually use full stops at the end of sentences.",
      "My name's Sophia Taylor."
    ]
  },
  {
    "name": "Review:143-147",
    "tag": "multifill",
    "rows": [
      "she is: _____{she's}",
      "we are: _____{we're}",
      "he is: _____{he's}",
      "I am not: _____{I'm} not",
      "he is not: he _____{isn't}"
    ]
  }
]
`)
	var cases []struct {
		Name, Tag string
		Rows      []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			result := v021RowsDocument(t, tc.Tag, tc.Rows)
			v021Run(t, result, "first", nil)
			v021Run(t, result, "second", nil)
		})
	}
}

func TestV021IndependentSeriesRequireEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, tag string
		rows      []string
		want      [][4]int
	}{
		{"equals series", "note", []string{"the USA = the United States", "the UK = the United Kingdom"}, nil},
		{"equals in formatted text", "text", []string{"<em>hello</em> = привет", "goodbye = пока"}, nil},
		{"one equals in prose", "note", []string{"Если x = y", "то выражения равны."}, [][4]int{{4, 1, 5, 20}}},
		{"two equations in connected prose", "note", []string{"If x = y", "then x = y + 1."}, [][4]int{{4, 1, 5, 16}}},
		{"equals in parentheses is prose", "note", []string{"Начало (x = y)", "продолжение (a = b)"}, [][4]int{{4, 1, 5, 20}}},
		{"incomplete equals is prose", "note", []string{"Первое =", "далее ="}, [][4]int{{4, 1, 5, 8}}},
		{"numbered key rows", "key", []string{"hello (2)", "good afternoon (4)"}, nil},
		{"single numbered key row", "key", []string{"Начало (2)", "далее"}, [][4]int{{4, 1, 5, 6}}},
		{"numbered prose outside key", "text", []string{"Начало (2)", "далее (4)"}, [][4]int{{4, 1, 5, 10}}},
		{"unmarked vocabulary alone", "multifill", []string{"hi", "hello", "good morning"}, [][4]int{{4, 1, 5, 6}, {5, 1, 6, 13}}},
		{"stress marks alone", "key", []string{"'hi", "hel'lo"}, [][4]int{{4, 1, 5, 7}}},
		{"labeled internal gaps", "multifill", []string{"I am not: _____{I'm} not", "he is not: he _____{isn't}"}, nil},
		{"unlabeled internal gaps are prose", "multifill", []string{"Начало _____{да} текста", "продолжение _____{нет} текста"}, [][4]int{{4, 1, 5, 30}}},
		{"one labeled gap is prose", "multifill", []string{"Начало: _____{да} текста", "далее"}, [][4]int{{4, 1, 5, 6}}},
		{"escaped labeled gaps are prose", "multifill", []string{`First: \_____ word`, `second: \_____ word`}, [][4]int{{4, 1, 5, 20}}},
		{"literal key gaps are prose", "key", []string{"First: _____ word", "second: _____ word"}, [][4]int{{4, 1, 5, 19}}},
		{"colon is not terminal", "note", []string{"Начало:", "далее"}, [][4]int{{4, 1, 5, 6}}},
		{"one example after colon is prose", "note", []string{"Начало:", "далее (пример)"}, [][4]int{{4, 1, 5, 15}}},
		{"parentheses without introduction are prose", "note", []string{"Начало (пример)", "далее (пример)"}, [][4]int{{4, 1, 5, 15}}},
		{"introduced example series", "note", []string{"Examples:", "for names (John)", "for places (Rome)", "at sentence start (My name's ...)"}, nil},
		{"no pair across independent series", "note", []string{"Начало", "hello = привет", "goodbye = пока", "далее", "ещё"}, [][4]int{{7, 1, 8, 4}}},
		{"different row kinds do not confirm series", "key", []string{"hello (2)", "goodbye = пока"}, [][4]int{{4, 1, 5, 15}}},
		{"blank and br preserve series", "text", []string{"hello = привет", "", "<br>", "goodbye = пока"}, nil},
		{"inline br is a boundary", "text", []string{"hello = привет", "x<br>", "goodbye = пока"}, [][4]int{{4, 1, 5, 6}, {5, 1, 6, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021RowsDocument(t, tc.tag, tc.rows), "current", tc.want)
		})
	}
	t.Run("series cannot cross containers", func(t *testing.T) {
		result := v021Document(t, v021Open(t, "note", 2, 1), v021Text(t, "First = first", 3, 2), v021Close(t, 3, 2),
			v021Open(t, "note", 2, 1), v021Text(t, "second = second", 6, 2), v021Text(t, "continued", 6, 2), v021Close(t, 6, 2))
		v021Run(t, result, "current", [][4]int{{7, 1, 8, 10}})
	})
}

// v021RowsDocument сохраняет порядок содержимого и настоящие пустые строки.
func v021RowsDocument(t *testing.T, tag string, rows []string) *model.Result {
	t.Helper()
	var body []model.Line
	for _, raw := range rows {
		if raw == "" {
			body = append(body, v021Line(t, "", "blank", 3, 2))
		} else {
			body = append(body, v021Text(t, raw, 3, 2))
		}
	}
	return v021Block(t, tag, body...)
}

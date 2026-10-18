package checkers

import (
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

func TestV021PairedStressLists(t *testing.T) {
	for _, tc := range []struct {
		name       string
		words, key []string
		between    []model.Line
		want       [][4]int
	}{
		{"C84-96", []string{"hi", "hello", "good morning", "good afternoon", "good evening"},
			[]string{"'hi", "hel'lo", "good 'morning", "good after'noon", "good 'evening"}, nil, nil},
		{"blank between blocks", []string{"hello", "good morning"}, []string{"hel'lo", "good 'morning"},
			[]model.Line{v021Line(t, "", "blank", 2, 1)}, nil},
		{"different key", []string{"hello", "good morning"}, []string{"hel'lo", "good 'evening"}, nil, [][4]int{{4, 1, 5, 13}, {8, 1, 9, 14}}},
		{"unchanged key is not stress evidence", []string{"Начало", "далее"}, []string{"Начало", "далее"}, nil, [][4]int{{4, 1, 5, 6}, {8, 1, 9, 6}}},
		{"contraction corrections do not prove stress", []string{"cant", "dont"}, []string{"can't", "don't"}, nil, [][4]int{{4, 1, 5, 5}, {8, 1, 9, 6}}},
		{"apostrophes only in some rows", []string{"hello", "good morning"}, []string{"hel'lo", "good morning"}, nil, [][4]int{{4, 1, 5, 13}, {8, 1, 9, 13}}},
		{"not separated by another construction", []string{"hello", "good morning"}, []string{"hel'lo", "good 'morning"},
			[]model.Line{v021Text(t, "Another exercise.", 2, 1)}, [][4]int{{4, 1, 5, 13}, {9, 1, 10, 14}}},
		{"different row count", []string{"hello", "good morning"}, []string{"hel'lo"}, nil, [][4]int{{4, 1, 5, 13}}},
		{"punctuated prose is not a word list", []string{"Начало:", "далее"}, []string{"На'чало:", "да'лее"}, nil, [][4]int{{4, 1, 5, 6}, {8, 1, 9, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v021StressDocument(t, tc.words, tc.key, tc.between...)
			v021Run(t, result, "first", tc.want)
			v021Run(t, result, "second", tc.want)
		})
	}
	t.Run("different parents", func(t *testing.T) {
		result := v021StressDocument(t, []string{"hello", "good morning"}, []string{"hel'lo", "good 'morning"})
		result.Lines[6].ParentLine, result.Lines[6].NestingLevel = nil, 0
		for i := 7; i <= 9; i++ {
			result.Lines[i].NestingLevel = 1
		}
		v021Run(t, result, "current", [][4]int{{4, 1, 5, 13}, {8, 1, 9, 14}})
	})
}

// v021StressDocument задаёт два соседних блока упражнения и ключа.
func v021StressDocument(t *testing.T, words, key []string, between ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v021Open(t, "multifill", 2, 1)}
	for _, word := range words {
		body = append(body, v021Text(t, word, 3, 2))
	}
	body = append(body, v021Close(t, 3, 2))
	body = append(body, between...)
	parent := len(body) + 3
	body = append(body, v021Open(t, "key", 2, 1))
	for _, word := range key {
		body = append(body, v021Text(t, word, parent, 2))
	}
	body = append(body, v021Close(t, parent, 2))
	return v021Document(t, body...)
}

package checkers

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// normalizationWant задаёт исходные Unicode-диапазоны с исключающим концом.
type normalizationWant struct {
	code    diagnostics.Code
	span    [4]int
	related [][4]int
}

func TestAnswerNormalizationIntegrationQuestion(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		block        bool
		want         []normalizationWant
	}{
		{"inline whitespace", "\u00a0\t\u2003", false, []normalizationWant{
			{"V136", [4]int{4, 1, 4, 8}, nil}, {"V002", [4]int{4, 10, 4, 11}, nil}}},
		{"block whitespace", "\u00a0\t\u2003", true, []normalizationWant{
			{"V137", [4]int{4, 1, 6, 2}, nil}, {"V138", [4]int{5, 1, 6, 1}, nil}, {"V002", [4]int{5, 2, 5, 3}, nil}}},
		{"inline punctuation stays nonempty", "!!!", false, nil},
		{"block punctuation stays nonempty", "???", true, nil},
		{"inline Unicode and tab", "ЁЖ\tлес!", false, []normalizationWant{{"V002", [4]int{4, 11, 4, 12}, nil}}},
		{"block Unicode and tab", "ЁЖ\tлес!", true, []normalizationWant{{"V002", [4]int{5, 3, 5, 4}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v138Tag(t, "question", "content", "Where?", 2, 1)}
			if tc.block {
				body = append(body, v138Open(t, "answer", 2, 1), v138TestText(t, tc.answer, tc.answer, 4, 2), v138Close(t, 4, 2))
			} else {
				body = append(body, v138Tag(t, "answer", "content", tc.answer, 2, 1))
			}
			normalizationRun(t, v138Document(t, body...), tc.want)
		})
	}
}

func TestAnswerNormalizationIntegrationMultifill(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []normalizationWant
	}{
		{"ordinary", "_____ _____", nil},
		{"prefilled alternatives", "_____{ ЁЖ\u00a0}{ёж!}", nil},
		{"punctuation only", "_____{!!!}{???}", nil},
		{"mixed punctuation", "_____ _____{!!!}", []normalizationWant{{"V155", [4]int{3, 1, 5, 2}, [][4]int{{4, 1, 4, 6}}}}},
		{"empty special does not enable mixed mode", "_____ _____{\t\u00a0}", []normalizationWant{
			{"V152", [4]int{4, 7, 4, 16}, nil}, {"V002", [4]int{4, 13, 4, 14}, nil}}},
		{"empty alternative cancels mixed mode", "_____ _____{A}{}", []normalizationWant{{"V152", [4]int{4, 7, 4, 17}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			normalizationRun(t, v138Document(t, v138Open(t, "multifill", 2, 1),
				v138TestText(t, tc.text, tc.text, 3, 2), v138Close(t, 3, 2)), tc.want)
		})
	}
	for _, block := range []bool{false, true} {
		t.Run("instruction/block="+strconv.FormatBool(block), func(t *testing.T) {
			opening := v138Tag(t, "multifill", "content", "Complete _____", 2, 1)
			body := []model.Line{opening}
			if block {
				opening = v155TestBlock(t, "multifill", "content", "Complete _____", 2, 1)
				body = []model.Line{opening, v138TestText(t, "_____{ЁЖ!}", "_____{ЁЖ!}", 3, 2), v138Close(t, 3, 2)}
			}
			normalizationRun(t, v138Document(t, body...), nil)
		})
	}
	t.Run("bare declaration", func(t *testing.T) {
		normalizationRun(t, v138Document(t, v138Tag(t, "multifill", "", "", 2, 1)), nil)
	})
}

func TestAnswerNormalizationIntegrationChoice(t *testing.T) {
	for _, name := range []string{"choice", "multichoice"} {
		t.Run(name+"/conflict preserves source", func(t *testing.T) {
			normalizationRun(t, v138Document(t, v138Open(t, name, 2, 1),
				v138Tag(t, "answer", "content", "ЁЖ\tлес!", 3, 2), v138TestText(t, "ёж\u2003лес", "ёж\u2003лес", 3, 2), v138Close(t, 3, 2)),
				[]normalizationWant{{"V157", [4]int{5, 1, 5, 7}, [][4]int{{4, 9, 4, 16}}}, {"V002", [4]int{4, 11, 4, 12}, nil}})
		})
		t.Run(name+"/equal instances still count", func(t *testing.T) {
			var want []normalizationWant
			if name == "choice" {
				want = []normalizationWant{{"V139", [4]int{4, 1, 7, 2}, [][4]int{{6, 1, 6, 4}}}}
			}
			normalizationRun(t, v138Document(t, v138Open(t, name, 2, 1), v138Open(t, "answer", 3, 2),
				v138TestText(t, "A", "A", 4, 3), v138TestText(t, "«a»", "«a»", 4, 3), v138Close(t, 4, 3), v138Close(t, 3, 2)), want)
		})
	}
	normalizationRun(t, v138Document(t, v138Open(t, "choice", 2, 1), v138Tag(t, "answer", "content", "A!", 3, 2),
		v138Tag(t, "answer", "content", "a", 3, 2), v138Close(t, 3, 2)),
		[]normalizationWant{{"V158", [4]int{5, 1, 5, 8}, [][4]int{{4, 1, 4, 8}}}})
	normalizationRun(t, v138Document(t, v138Open(t, "choice", 2, 1), v138Tag(t, "answer", "content", "!!!", 3, 2), v138Close(t, 3, 2)),
		[]normalizationWant{{"V156", [4]int{3, 1, 5, 2}, nil}})
}

func TestAnswerNormalizationIntegrationCompleteness(t *testing.T) {
	for _, name := range []string{"matching", "ordering"} {
		for _, complete := range []bool{false, true} {
			t.Run(name+"/complete="+strconv.FormatBool(complete), func(t *testing.T) {
				body := []model.Line{v138Open(t, name, 2, 1), v138TestText(t, "A!", "A!", 3, 2),
					v138Tag(t, "answer", "content", "\u00a0", 3, 2), v138TestText(t, "a", "a", 3, 2)}
				want := []normalizationWant{{"V136", [4]int{5, 1, 5, 8}, nil}}
				if complete {
					body = append(body, v138Tag(t, "answer", "content", "!!!", 3, 2))
					if name == "ordering" {
						want = append(want, normalizationWant{"V180", [4]int{7, 9, 7, 12}, nil})
					}
				} else if name == "matching" {
					want = append(want, normalizationWant{"V169", [4]int{3, 1, 10, 2}, [][4]int{{6, 1, 6, 2}}})
				} else {
					want = append(want, normalizationWant{"V179", [4]int{3, 1, 7, 2}, [][4]int{{6, 1, 6, 2}}})
				}
				if name == "matching" {
					body = append(body, v138Line(t, "---", "separator", 3, 2), v138TestText(t, "x", "x", 3, 2), v138TestText(t, "y", "y", 3, 2))
				}
				normalizationRun(t, v138Document(t, append(body, v138Close(t, 3, 2))...), want)
			})
		}
	}
}

func TestAnswerNormalizationIntegrationLists(t *testing.T) {
	for _, tc := range []struct {
		name              string
		absent, duplicate diagnostics.Code
	}{{"matching", "V170", "V171"}, {"ordering", "V182", "V183"}} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v138Open(t, tc.name, 2, 1), v138TestText(t, "A", "A", 3, 2), v138Open(t, "answer", 3, 2),
				v138TestText(t, "NEW\tYORK!", "NEW\tYORK!", 5, 3), v138TestText(t, "new\u00a0york", "new\u00a0york", 5, 3),
				v138TestText(t, "!!!", "!!!", 5, 3), v138TestText(t, "???", "???", 5, 3), v138Close(t, 5, 3),
				v138TestText(t, "B", "B", 3, 2), v138Tag(t, "answer", "content", "LONDON!", 3, 2), v138Line(t, "---", "separator", 3, 2),
				v138TestText(t, "London", "London", 3, 2), v138TestText(t, "london.", "london.", 3, 2), v138Close(t, 3, 2)}
			normalizationRun(t, v138Document(t, body...), []normalizationWant{
				{tc.absent, [4]int{6, 1, 6, 10}, [][4]int{{7, 1, 7, 9}}},
				{tc.absent, [4]int{8, 1, 8, 4}, nil}, {tc.absent, [4]int{9, 1, 9, 4}, nil},
				{tc.duplicate, [4]int{15, 1, 15, 8}, [][4]int{{14, 1, 14, 7}}}, {"V002", [4]int{6, 4, 6, 5}, nil}})
		})
	}
}

func TestAnswerNormalizationIntegrationOrdering(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                []normalizationWant
	}{
		{"punctuated permutation", "1.", "(2)", nil},
		{"equal labels remain separate items", "1.", "(1)", []normalizationWant{{"V184", [4]int{3, 1, 8, 2}, nil}}},
		{"leading zero retained", "0!", "2", []normalizationWant{{"V180", [4]int{5, 9, 5, 11}, nil}}},
		{"symbol retained", "+1", "2", []normalizationWant{{"V180", [4]int{5, 9, 5, 11}, nil}}},
		{"normalized number exceeds count", "(3)", "2", []normalizationWant{{"V181", [4]int{5, 9, 5, 12}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			normalizationRun(t, v138Document(t, v138Open(t, "ordering", 2, 1), v138TestText(t, "A!", "A!", 3, 2),
				v138Tag(t, "answer", "content", tc.first, 3, 2), v138TestText(t, "a", "a", 3, 2),
				v138Tag(t, "answer", "content", tc.second, 3, 2), v138Close(t, 3, 2)), tc.want)
		})
	}
	t.Run("block alternatives group by normalized value", func(t *testing.T) {
		normalizationRun(t, v138Document(t, v138Open(t, "ordering", 2, 1), v138TestText(t, "A", "A", 3, 2), v138Open(t, "answer", 3, 2),
			v138TestText(t, "0!", "0!", 5, 3), v138TestText(t, "(0)", "(0)", 5, 3), v138TestText(t, "(3)", "(3)", 5, 3),
			v138TestText(t, "3.", "3.", 5, 3), v138Close(t, 5, 3), v138TestText(t, "a!", "a!", 3, 2),
			v138Tag(t, "answer", "content", "2", 3, 2), v138Close(t, 3, 2)), []normalizationWant{
			{"V180", [4]int{6, 1, 6, 3}, [][4]int{{7, 1, 7, 4}}}, {"V181", [4]int{8, 1, 8, 4}, [][4]int{{9, 1, 9, 3}}}})
	})
}

func TestAnswerNormalizationIntegrationIndependentExercises(t *testing.T) {
	for _, variants := range []bool{false, true} {
		t.Run("variants="+strconv.FormatBool(variants), func(t *testing.T) {
			var body []model.Line
			openings, parents, depth := [2]int{3, 10}, [2]int{2, 2}, 1
			want := []normalizationWant{{"V170", [4]int{5, 9, 5, 18}, nil}, {"V002", [4]int{5, 12, 5, 13}, nil},
				{"V171", [4]int{8, 1, 8, 8}, [][4]int{{7, 1, 7, 7}}}, {"V171", [4]int{15, 1, 15, 10}, [][4]int{{14, 1, 14, 9}}}}
			if variants {
				openings, parents, depth = [2]int{5, 13}, [2]int{4, 12}, 3
				body = append(body, v138Open(t, "variants", 2, 1))
				want = []normalizationWant{{"V170", [4]int{7, 9, 7, 18}, nil}, {"V002", [4]int{7, 12, 7, 13}, nil},
					{"V171", [4]int{10, 1, 10, 8}, [][4]int{{9, 1, 9, 7}}}, {"V171", [4]int{18, 1, 18, 10}, [][4]int{{17, 1, 17, 9}}}}
			}
			for index, opening := range openings {
				answer, first, second := "NEW\tYORK!", "London", "LONDON!"
				if index == 1 {
					answer, first, second = "new york", "New York", "NEW YORK!"
				}
				if variants {
					body = append(body, v138Tag(t, "variant", "name", []string{"A", "B"}[index], 3, 2))
				}
				body = append(body, v138Open(t, "matching", parents[index], depth), v138TestText(t, "A", "A", opening, depth+1),
					v138Tag(t, "answer", "content", answer, opening, depth+1), v138Line(t, "---", "separator", opening, depth+1),
					v138TestText(t, first, first, opening, depth+1), v138TestText(t, second, second, opening, depth+1), v138Close(t, opening, depth+1))
			}
			if variants {
				body = append(body, v138Close(t, 3, 2))
			}
			normalizationRun(t, v138Document(t, body...), want)
		})
	}
}

// normalizationRun запускает весь каталог; сверяет смысловые проверки и V002.
func normalizationRun(t *testing.T, result *model.Result, want []normalizationWant) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatal(err)
	}
	result.Processing = append(result.Processing, model.Processing{ID: "normalization-integration", Tool: "dsl-validator"})
	if err := validator.Run(result, checks, "normalization-integration"); err != nil {
		t.Fatal(err)
	}
	for _, code := range strings.Fields("V002 V136 V137 V138 V139 V140 V141 V142 V143 V144 V145 V146 V147 V152 V155 V156 V157 V158 V169 V170 V171 V179 V180 V181 V182 V183 V184") {
		var got, expected []normalizationWant
		for _, entry := range result.Diagnostics {
			if string(entry.DiagnosticCode) != code || entry.Source != "normalization-integration" {
				continue
			}
			if entry.Location == nil {
				t.Fatalf("%s has no source range", code)
			}
			rangeOf := func(loc model.Location) [4]int {
				return [4]int{loc.Start.Line, loc.Start.Column, loc.End.Line, loc.End.Column}
			}
			actual := normalizationWant{entry.DiagnosticCode, rangeOf(*entry.Location), nil}
			for _, related := range entry.RelatedLocations {
				actual.related = append(actual.related, rangeOf(related))
			}
			got = append(got, actual)
		}
		for _, entry := range want {
			if string(entry.code) == code {
				expected = append(expected, entry)
			}
		}
		if !slices.EqualFunc(got, expected, func(a, b normalizationWant) bool {
			return a.code == b.code && a.span == b.span && slices.Equal(a.related, b.related)
		}) {
			t.Errorf("%s = %+v, want %+v", code, got, expected)
		}
	}
}

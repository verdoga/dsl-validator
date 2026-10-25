package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v183Want содержит явно заданный основной диапазон и связанные строки.
type v183Want struct {
	main    [3]int
	related [][3]int
}

func TestV183Registration(t *testing.T) {
	registry := v183TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V183" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "В нижнем списке @ordering повторяется подпись позиции." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V183 registrations = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV183ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v183Want
	}{
		{"reference", "@ordering {\nA\n@answer First\nB\n@answer First\n---\nFirst\nLast\n}", []model.Line{
			v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "First", 3, 2),
			v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "First", 3, 2),
			v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "Last", "Last", 3, 2)}, nil},
		{"example", "@ordering {\nA\n@answer First\nB\n@answer Other\n---\nFirst\nfirst\nOther\n}", append(v183TestUpper(t),
			v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2), v183TestText(t, "Other", "Other", 3, 2)),
			[]v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v183TestBlock(t, "ordering", tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 фрагмент дополнен только обязательными версией и заголовком.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v183TestRun(t, result, tc.want)
			v183TestRun(t, result, tc.want)
		})
	}
}

func TestV183Grouping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		labels []string
		want   []v183Want
	}{
		{"unique", []string{"First", "Other", "Last"}, nil},
		{"no positions", nil, nil},
		{"one position", []string{"First"}, nil},
		{"three occurrences", []string{"First", "first", "FIRST"}, []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}, {11, 1, 6}}}}},
		{"separated occurrences", []string{"First", "Other", "first"}, []v183Want{{[3]int{11, 1, 6}, [][3]int{{9, 1, 6}}}}},
		{"two groups", []string{"One", "one", "Two", "two", "ONE", "TWO"}, []v183Want{
			{[3]int{10, 1, 4}, [][3]int{{9, 1, 4}, {13, 1, 4}}}, {[3]int{12, 1, 4}, [][3]int{{11, 1, 4}, {14, 1, 4}}}}},
		{"punctuation only", []string{"?!", "?!"}, []v183Want{{[3]int{10, 1, 3}, [][3]int{{9, 1, 3}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lower []model.Line
			for _, label := range tc.labels {
				lower = append(lower, v183TestText(t, label, label, 3, 2))
			}
			v183TestRun(t, v183TestExpanded(t, lower...), tc.want)
		})
	}
	t.Run("unreferenced duplicates", func(t *testing.T) {
		body := []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "Other", 3, 2), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2), v183TestText(t, "Other", "Other", 3, 2)}
		v183TestRun(t, v183TestBlock(t, "ordering", body...), []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}})
	})
}

func TestV183ComparisonAndEscapes(t *testing.T) {
	for _, tc := range []struct {
		name, firstRaw, firstValue, secondRaw, secondValue string
		want                                               []v183Want
	}{
		{"case", "First", "First", "FIRST", "FIRST", []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}}},
		{"unicode columns", "Ёж🌍", "Ёж🌍", "ёж🌍", "ёж🌍", []v183Want{{[3]int{10, 1, 4}, [][3]int{{9, 1, 4}}}}},
		{"internal spaces", "New York", "New York", "New  York", "New  York", nil},
		{"internal tabs", "A B", "A B", "A\tB", "A\tB", nil},
		{"same internal tabs", "A\tB", "A\tB", "a\tb", "a\tb", []v183Want{{[3]int{10, 1, 4}, [][3]int{{9, 1, 4}}}}},
		{"punctuation", "First", "First", "First!", "First!", nil},
		{"numbers", "First", "First", "1. First", "1. First", nil},
		{"quote style", `"First"`, `"First"`, "«First»", "«First»", nil},
		{"non ASCII edge space", "First", "First", "First\u00a0", "First\u00a0", nil},
		{"NBSP alone is retained", "\u00a0", "\u00a0", "\u00a0", "\u00a0", []v183Want{{[3]int{10, 1, 2}, [][3]int{{9, 1, 2}}}}},
		{"unicode normalization forbidden", "é", "é", "e\u0301", "e\u0301", nil},
		{"ASCII edges", " \tFirst\t ", "First", "\t first \t", "first", []v183Want{{[3]int{10, 3, 8}, [][3]int{{9, 3, 8}}}}},
		{"opening brace", `A\{B`, "A{B", `a\{b`, "a{b", []v183Want{{[3]int{10, 1, 5}, [][3]int{{9, 1, 5}}}}},
		{"closing brace", `A\}B`, "A}B", `a\}b`, "a}b", []v183Want{{[3]int{10, 1, 5}, [][3]int{{9, 1, 5}}}}},
		{"initial at sign", `\@First`, "@First", `\@first`, "@first", []v183Want{{[3]int{10, 1, 8}, [][3]int{{9, 1, 8}}}}},
		{"initial hash", `\#First`, "#First", `\#first`, "#first", []v183Want{{[3]int{10, 1, 8}, [][3]int{{9, 1, 8}}}}},
		{"escaped separator", `\---`, "---", `\---`, "---", []v183Want{{[3]int{10, 1, 5}, [][3]int{{9, 1, 5}}}}},
		{"backslash not decoded twice", `a\\b`, `a\b`, "ab", "ab", nil},
		{"paired backslashes", `a\\b`, `a\b`, `A\\B`, `A\B`, []v183Want{{[3]int{10, 1, 5}, [][3]int{{9, 1, 5}}}}},
		{"unknown escape", `a\q`, `a\q`, "aq", "aq", nil},
		{"unknown escapes equal", `a\q`, `a\q`, `A\Q`, `A\Q`, []v183Want{{[3]int{10, 1, 4}, [][3]int{{9, 1, 4}}}}},
		{"at sign away from start", `a\@b`, `a\@b`, "a@b", "a@b", nil},
		{"quote escape not supported here", `a\"b`, `a\"b`, `a"b`, `a"b`, nil},
		{"no placeholder decoding", `\_____`, `\_____`, "_____", "_____", nil},
	} {
		for _, rawOnly := range []bool{false, true} {
			t.Run(tc.name+"/rawOnly="+strconv.FormatBool(rawOnly), func(t *testing.T) {
				first := v183TestText(t, tc.firstRaw, tc.firstValue, 3, 2)
				second := v183TestText(t, tc.secondRaw, tc.secondValue, 3, 2)
				if rawOnly {
					first.Elements, second.Elements = []model.Element{}, []model.Element{}
				}
				v183TestRun(t, v183TestExpanded(t, first, second), tc.want)
			})
		}
	}
}

func TestV183ValueRepresentations(t *testing.T) {
	t.Run("several content fragments form one position", func(t *testing.T) {
		first := v183TestLine(t, "First", "content", 3, 2, v183TestElement(t, "content", "Fi", "Fi", 1), v183TestElement(t, "content", "rst", "rst", 3))
		v183TestRun(t, v183TestExpanded(t, first, v183TestText(t, "first", "first", 3, 2)), []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}})
	})
	t.Run("fragment gaps retain whitespace", func(t *testing.T) {
		first := v183TestLine(t, "New \tYork", "content", 3, 2, v183TestElement(t, "content", "New", "New", 1), v183TestElement(t, "content", "York", "York", 6))
		v183TestRun(t, v183TestExpanded(t, first, v183TestText(t, "new \tyork", "new \tyork", 3, 2)), []v183Want{{[3]int{10, 1, 10}, [][3]int{{9, 1, 10}}}})
	})
	t.Run("content excludes source indentation", func(t *testing.T) {
		first := v183TestLine(t, "  First\t", "content", 3, 2, v183TestElement(t, "content", "First", "First", 3))
		second := v183TestLine(t, "\t first ", "content", 3, 2, v183TestElement(t, "content", "first", "first", 3))
		v183TestRun(t, v183TestExpanded(t, first, second), []v183Want{{[3]int{10, 3, 8}, [][3]int{{9, 3, 8}}}})
	})
	t.Run("ASCII blank rows are not positions", func(t *testing.T) {
		v183TestRun(t, v183TestExpanded(t, v183TestText(t, "First", "First", 3, 2), v183TestLine(t, " \t", "blank", 3, 2), v183TestLine(t, "", "blank", 3, 2), v183TestText(t, "Other", "Other", 3, 2)), nil)
	})
	for _, raw := range []string{"A {", `A\\{`} {
		t.Run("uninterpretable lower/"+raw, func(t *testing.T) {
			v183TestRun(t, v183TestExpanded(t, v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2), v183TestUnparsed(t, raw, "invalid", 3, 2)), nil)
		})
	}
	t.Run("unparsed fragment prevents label recovery", func(t *testing.T) {
		line := v183TestLine(t, "First {", "invalid", 3, 2, v183TestElement(t, "content", "First", "First", 1), model.Element{ElementType: "unparsed", Raw: "{", Start: 7, End: 8, ErrorIDs: []string{}})
		v183TestRun(t, v183TestExpanded(t, line, v183TestText(t, "first", "first", 3, 2)), nil)
	})
}

func TestV183AnswerAssociation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v183Want
	}{
		{"no upper items", nil, nil},
		{"no answers", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestText(t, "B", "B", 3, 2)}, nil},
		{"partial first answered", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "First", 3, 2), v183TestText(t, "B", "B", 3, 2)}, nil},
		{"partial last answered", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, nil},
		{"orphan answer", append([]model.Line{v183TestInline(t, "answer", "First", 3, 2)}, v183TestUpper(t)...), nil},
		{"repeated answer", append(v183TestUpper(t), v183TestInline(t, "answer", "Other", 3, 2)), nil},
		{"one upper item is enough for this check", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "First", 3, 2)}, []v183Want{{[3]int{8, 1, 6}, [][3]int{{7, 1, 6}}}}},
		{"duplicate upper items stay separate", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "First", 3, 2), v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}}},
		{"blank before answer", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestLine(t, " \t", "blank", 3, 2), v183TestInline(t, "answer", "First", 3, 2), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, []v183Want{{[3]int{11, 1, 6}, [][3]int{{10, 1, 6}}}}},
		{"hint after answer", append(v183TestUpper(t), v183TestInline(t, "hint", "Explanation", 3, 2)), []v183Want{{[3]int{11, 1, 6}, [][3]int{{10, 1, 6}}}}},
		{"block answer", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestOpen(t, "answer", 3, 2), v183TestText(t, "First", "First", 5, 3), v183TestText(t, "Other", "Other", 5, 3), v183TestClose(t, 5, 3), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, []v183Want{{[3]int{13, 1, 6}, [][3]int{{12, 1, 6}}}}},
		{"empty block answer", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestOpen(t, "answer", 3, 2), v183TestClose(t, 5, 3), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, []v183Want{{[3]int{11, 1, 6}, [][3]int{{10, 1, 6}}}}},
		{"unknown answer contents", []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestOpen(t, "answer", 3, 2), v183TestUnparsed(t, "x {", "invalid", 5, 3), v183TestClose(t, 5, 3), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, []v183Want{{[3]int{12, 1, 6}, [][3]int{{11, 1, 6}}}}},
		{"unknown upper item", []model.Line{v183TestUnparsed(t, "A {", "invalid", 3, 2), v183TestInline(t, "answer", "First", 3, 2), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.upper, v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2), v183TestText(t, "Other", "Other", 3, 2))
			v183TestRun(t, v183TestBlock(t, "ordering", body...), tc.want)
		})
	}
	for _, tag := range []string{"hint", "editor", "key"} {
		t.Run("intermediate/"+tag, func(t *testing.T) {
			upper := v183TestUpper(t)
			body := append([]model.Line{upper[0], v183TestInline(t, tag, "Text", 3, 2)}, upper[1:]...)
			body = append(body, v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2))
			v183TestRun(t, v183TestBlock(t, "ordering", body...), nil)
		})
	}
	for _, value := range []string{"", "wrong!", "0", "First"} {
		t.Run("answer value irrelevant/"+value, func(t *testing.T) {
			body := []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "AnSwEr", value, 3, 2), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "ANSWER", value, 3, 2), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2)}
			v183TestRun(t, v183TestBlock(t, "ORDERING", body...), []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}})
		})
	}
}

func TestV183SeparatorsAndNestedBodies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		middle []model.Line
		want   []v183Want
	}{
		{"no separator", nil, nil},
		{"two separators", []model.Line{v183TestSeparator(t, "---", 3, 2), v183TestSeparator(t, "---", 3, 2)}, nil},
		{"trim only ASCII edges", []model.Line{v183TestSeparator(t, " \t---\t ", 3, 2)}, []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}}},
		{"escaped separator is ordinary text", []model.Line{v183TestText(t, `\---`, "---", 3, 2)}, nil},
		{"NBSP is not trimmed", []model.Line{v183TestText(t, "\u00a0---", "\u00a0---", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(v183TestUpper(t), tc.middle...)
			body = append(body, v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2))
			v183TestRun(t, v183TestBlock(t, "ordering", body...), tc.want)
		})
	}
	t.Run("separator and labels inside answer excluded", func(t *testing.T) {
		body := []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestOpen(t, "answer", 3, 2),
			v183TestText(t, "First", "First", 5, 3), v183TestText(t, "First", "First", 5, 3), v183TestText(t, "---", "---", 5, 3), v183TestClose(t, 5, 3),
			v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "Other", "Other", 3, 2)}
		v183TestRun(t, v183TestBlock(t, "ordering", body...), nil)
	})
	t.Run("nested hint separator does not disable duplicate check", func(t *testing.T) {
		body := append(v183TestUpper(t), v183TestOpen(t, "hint", 3, 2), v183TestText(t, "---", "---", 8, 3), v183TestText(t, "First", "First", 8, 3), v183TestClose(t, 8, 3),
			v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2))
		v183TestRun(t, v183TestBlock(t, "ordering", body...), []v183Want{{[3]int{14, 1, 6}, [][3]int{{13, 1, 6}}}})
	})
	t.Run("nested lower bodies are not positions", func(t *testing.T) {
		body := append(v183TestUpper(t), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2),
			v183TestOpen(t, "hint", 3, 2), v183TestText(t, "first", "first", 10, 3), v183TestText(t, "---", "---", 10, 3), v183TestClose(t, 10, 3), v183TestText(t, "Other", "Other", 3, 2))
		v183TestRun(t, v183TestBlock(t, "ordering", body...), nil)
	})
	t.Run("known lower tag does not hide direct duplicates", func(t *testing.T) {
		v183TestRun(t, v183TestExpanded(t, v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)), []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}})
	})
	t.Run("heading excluded from lower positions", func(t *testing.T) {
		heading := v183TestLine(t, "### First", "invalid", 3, 2, v183TestElement(t, "heading-level", "###", "3", 1), model.Element{ElementType: "unparsed", Raw: "First", Start: 5, End: 10, ErrorIDs: []string{}})
		v183TestRun(t, v183TestExpanded(t, heading, v183TestText(t, "First", "First", 3, 2), v183TestText(t, "Other", "Other", 3, 2)), nil)
	})
	t.Run("ambiguous closing links", func(t *testing.T) {
		body := append(v183TestUpper(t), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2), v183TestClose(t, 3, 2))
		result := v183TestBlock(t, "ordering", body...)
		// Модель ошибки границ допустима контрактом, аналогично тестам V035.
		v183TestParserError(t, result, 12)
		v183TestRun(t, result, nil)
	})
}

func TestV183ContextsAndLifecycle(t *testing.T) {
	t.Run("instruction is not a lower position", func(t *testing.T) {
		opening := v183TestLine(t, "@ordering First {", "block-start", 2, 1, v183TestElement(t, "tag", "@ordering", "ordering", 1), v183TestElement(t, "content", "First", "First", 11), v183TestElement(t, "block-open", "{", "{", 17))
		body := append([]model.Line{opening}, v183TestUpper(t)...)
		body = append(body, v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "Other", "Other", 3, 2), v183TestClose(t, 3, 2))
		v183TestRun(t, v183TestDocument(t, body...), nil)
	})
	for _, duplicate := range []bool{false, true} {
		t.Run("independent blocks/duplicates="+strconv.FormatBool(duplicate), func(t *testing.T) {
			last := "Other"
			var want []v183Want
			if duplicate {
				last = "first"
				want = []v183Want{{[3]int{8, 1, 6}, [][3]int{{7, 1, 6}}}, {[3]int{15, 1, 6}, [][3]int{{14, 1, 6}}}}
			}
			v183TestRun(t, v183TestDocument(t,
				v183TestOpen(t, "ordering", 2, 1), v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "First", 3, 2), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, last, last, 3, 2), v183TestClose(t, 3, 2),
				v183TestOpen(t, "ordering", 2, 1), v183TestText(t, "B", "B", 10, 2), v183TestInline(t, "answer", "Other", 10, 2), v183TestSeparator(t, "---", 10, 2), v183TestText(t, "First", "First", 10, 2), v183TestText(t, last, last, 10, 2), v183TestClose(t, 10, 2)), want)
		})
	}
	t.Run("branch at depth", func(t *testing.T) {
		first := v183TestLine(t, "@variant A", "tag", 3, 2, v183TestElement(t, "tag", "@variant", "variant", 1), v183TestElement(t, "name", "A", "A", 10))
		second := v183TestLine(t, "@variant B", "tag", 3, 2, v183TestElement(t, "tag", "@variant", "variant", 1), v183TestElement(t, "name", "B", "B", 10))
		v183TestRun(t, v183TestDocument(t, v183TestOpen(t, "variants", 2, 1), first, v183TestOpen(t, "ordering", 4, 3),
			v183TestText(t, "A", "A", 5, 4), v183TestInline(t, "answer", "First", 5, 4), v183TestText(t, "B", "B", 5, 4), v183TestInline(t, "answer", "Other", 5, 4), v183TestSeparator(t, "---", 5, 4), v183TestText(t, "First", "First", 5, 4), v183TestText(t, "first", "first", 5, 4), v183TestText(t, "Other", "Other", 5, 4), v183TestClose(t, 5, 4), second, v183TestText(t, "Read.", "Read.", 15, 3), v183TestClose(t, 3, 2)), []v183Want{{[3]int{12, 1, 6}, [][3]int{{11, 1, 6}}}})
	})
	for _, name := range []string{"matching", "choice", "multichoice"} {
		t.Run("other container/"+name, func(t *testing.T) {
			body := append(v183TestUpper(t), v183TestSeparator(t, "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2))
			v183TestRun(t, v183TestBlock(t, name, body...), nil)
		})
	}
	t.Run("literal pseudo ordering", func(t *testing.T) {
		v183TestRun(t, v183TestBlock(t, "editor", v183TestText(t, "@ordering {", "@ordering {", 3, 2), v183TestText(t, "A", "A", 3, 2), v183TestText(t, "@answer First", "@answer First", 3, 2), v183TestText(t, "---", "---", 3, 2), v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2)), nil)
	})
	t.Run("empty document", func(t *testing.T) {
		result := v183TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v183TestRun(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v183TestExpanded(t, v183TestText(t, "First", "First", 3, 2), v183TestText(t, "first", "first", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v183TestRun(t, result, []v183Want{{[3]int{10, 1, 6}, [][3]int{{9, 1, 6}}}})
			})
		}
	}
}

// v183TestUpper задаёт два элемента с независимо заданными ответами.
func v183TestUpper(t *testing.T) []model.Line {
	t.Helper()
	return []model.Line{v183TestText(t, "A", "A", 3, 2), v183TestInline(t, "answer", "First", 3, 2), v183TestText(t, "B", "B", 3, 2), v183TestInline(t, "answer", "Other", 3, 2)}
}

// v183TestExpanded помещает нижние строки начиная со строки 9 документа.
func v183TestExpanded(t *testing.T, lower ...model.Line) *model.Result {
	t.Helper()
	body := append(v183TestUpper(t), v183TestSeparator(t, "---", 3, 2))
	return v183TestBlock(t, "ordering", append(body, lower...)...)
}

// v183TestRun проверяет полный конвейер, исходные диапазоны и историю модели.
func v183TestRun(t *testing.T, result *model.Result, want []v183Want) {
	t.Helper()
	before := v183TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v183TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v183TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v183-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V183" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V183 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]}, End: model.Position{Line: expected.main[0], Column: expected.main[2]}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "В нижнем списке @ordering повторяется подпись позиции." || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V183 occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, expected)
		}
		for j, related := range expected.related {
			location := model.Location{Start: model.Position{Line: related[0], Column: related[1]}, End: model.Position{Line: related[0], Column: related[2]}}
			if entry.RelatedLocations[j] != location {
				t.Fatalf("related %d = %+v, want %+v", j, entry.RelatedLocations[j], location)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line diagnostic linked to element")
					}
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.main[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v183TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v183TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v183TestUnparsed задаёт неразобранную область; исходную ошибку добавляет Document.
func v183TestUnparsed(t *testing.T, raw string, kind model.LineType, parent, depth int) model.Line {
	t.Helper()
	return v183TestLine(t, raw, kind, parent, depth, model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
}

// v183TestParserError сохраняет исходную ошибку границ блока без фиктивных элементов.
func v183TestParserError(t *testing.T, result *model.Result, number int) {
	t.Helper()
	id := "boundary-" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error", DiagnosticScope: "line", Message: "Неоднозначные границы блока.", RelatedLocations: []model.Location{}, Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}}})
	result.Document.HasErrors, result.Lines[number-1].HasErrors = true, true
}

// v183TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v183TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v183TestLine задаёт структуру строки без повторного разбора DSL.
func v183TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v183TestOpen задаёт блочное объявление без параметров с явным родителем.
func v183TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v183TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v183TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v183TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v183TestClose задаёт закрытие указанного блока.
func v183TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v183TestLine(t, "}", "block-end", parent, depth, v183TestElement(t, "block-close", "}", "}", 1))
}

// v183TestInline задаёт известный тег с обычным содержимым без экранирования.
func v183TestInline(t *testing.T, name, content string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v183TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if content != "" {
		elements = append(elements, v183TestElement(t, "content", content, content, len(name)+3))
		raw += " " + content
	}
	return v183TestLine(t, raw, "tag", parent, depth, elements...)
}

// v183TestText сохраняет исходное и семантическое содержимое независимо.
func v183TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v183TestLine(t, raw, "content", parent, depth, v183TestElement(t, "content", raw, value, 1))
}

// v183TestSeparator задаёт собственную служебную строку без элементов.
func v183TestSeparator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v183TestLine(t, raw, "separator", parent, depth)
}

// v183TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v183TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v183TestOpen(t, name, 2, 1)}, body...)
	return v183TestDocument(t, append(lines, v183TestClose(t, 3, 2))...)
}

// v183TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v183TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v183TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v183TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v183TestElement(t, "version", "1.2", "1.2", 14)),
		v183TestLine(t, "# Lesson", "heading", 0, 0, v183TestElement(t, "heading-level", "#", "1", 1), v183TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Ошибка исходной конструкции.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}})
		}
	}
	return result
}

// v183TestRegistry использует только полный штатный каталог.
func v183TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v183TestSnapshot сохраняет независимое представление модели для сравнения.
func v183TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

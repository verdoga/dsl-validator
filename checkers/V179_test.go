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

// v179Want содержит явно заданный основной диапазон и связанные строки.
type v179Want struct {
	block   [4]int
	related [][3]int
}

func TestV179Registration(t *testing.T) {
	registry := v179TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V179" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "В @ordering ответы заданы только для части элементов." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V179 registrations = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV179ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v179Want
	}{
		{"reference", "@ordering {\nA\n@answer 2\nB\n@answer 1\n}", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, nil},
		{"example", "@ordering {\nA\n@answer 2\nB\n}", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}}},
		{"no answers", "@ordering {\nA\nB\n}", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestText(t, "B", "B", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v179TestBlock(t, "ordering", tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v179TestRun(t, result, tc.want)
			v179TestRun(t, result, tc.want)
		})
	}
}

func TestV179CountsAndAssociation(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v179Want
	}{
		{"empty", nil, nil},
		{"one without answer", []model.Line{v179TestText(t, "A", "A", 3, 2)}, nil},
		{"one with answer", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, nil},
		{"duplicates remain separate", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "A", "A", 3, 2)}, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}}},
		{"two missing occurrences", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "3", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{6, 1, 2}, {7, 1, 2}}}}},
		{"two of three answered", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "3", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "C", "C", 3, 2)}, []v179Want{{[4]int{3, 1, 9, 2}, [][3]int{{8, 1, 2}}}}},
		{"first missing", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 2}}}}},
		{"middle missing", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "3", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestText(t, "C", "C", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, []v179Want{{[4]int{3, 1, 9, 2}, [][3]int{{6, 1, 2}}}}},
		{"blank skipped", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestLine(t, " \t", "blank", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{7, 1, 2}}}}},
		{"hint after answer", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestInline(t, "hint", "Hint", 3, 2), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{7, 1, 2}}}}},
		{"key does not disable check", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestInline(t, "key", "Answer key", 3, 2), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{7, 1, 2}}}}},
		{"forbidden tag without misplaced answer", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "editor", "Comment", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{4, 1, 2}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v179TestRun(t, v179TestBlock(t, "ordering", tc.body...), tc.want) })
	}
}

func TestV179AnswerContentsDoNotMatter(t *testing.T) {
	for _, value := range []string{"", "nonsense", "0", "999", "\u00a0"} {
		t.Run("inline/"+strconv.Quote(value), func(t *testing.T) {
			v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", value, 3, 2), v179TestText(t, "B", "B", 3, 2)), []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}})
		})
	}
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v179Want
	}{
		{"block two values", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestOpen(t, "answer", 3, 2), v179TestText(t, "2", "2", 5, 3), v179TestText(t, "1", "1", 5, 3), v179TestClose(t, 5, 3), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 10, 2}, [][3]int{{9, 1, 2}}}}},
		{"empty block", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestOpen(t, "answer", 3, 2), v179TestClose(t, 5, 3), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{7, 1, 2}}}}},
		{"unknown answer content", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestOpen(t, "answer", 3, 2), v179TestUnparsed(t, "x {", "invalid", 5, 3), v179TestClose(t, 5, 3), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 9, 2}, [][3]int{{8, 1, 2}}}}},
		{"nested answer excluded", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestOpen(t, "hint", 3, 2), v179TestInline(t, "answer", "1", 6, 3), v179TestClose(t, 6, 3), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 10, 2}, [][3]int{{9, 1, 2}}}}},
		{"nested separator excluded", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestOpen(t, "hint", 3, 2), v179TestText(t, "---", "---", 6, 3), v179TestClose(t, 6, 3), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 10, 2}, [][3]int{{9, 1, 2}}}}},
		{"unparsed answer declaration", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestUnparsed(t, "@answer {oops", "invalid", 3, 2), v179TestText(t, "B", "B", 3, 2)}, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}}},
		{"two blocks answer all items", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestOpen(t, "answer", 3, 2), v179TestText(t, "2", "2", 5, 3), v179TestClose(t, 5, 3), v179TestText(t, "B", "B", 3, 2), v179TestOpen(t, "answer", 3, 2), v179TestText(t, "1", "1", 9, 3), v179TestClose(t, 9, 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v179TestRun(t, v179TestBlock(t, "ordering", tc.body...), tc.want) })
	}
}

func TestV179MisplacedAnswersSuppressBlock(t *testing.T) {
	for _, tag := range []string{"hint", "editor", "key"} {
		t.Run("intermediate/"+tag, func(t *testing.T) {
			v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, tag, "Text", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2)), nil)
		})
	}
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"orphan", []model.Line{v179TestInline(t, "answer", "1", 3, 2), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2)}},
		{"repeated inline", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestLine(t, "", "blank", 3, 2), v179TestInline(t, "answer", "1", 3, 2), v179TestText(t, "B", "B", 3, 2)}},
		{"repeated after block", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestOpen(t, "answer", 3, 2), v179TestText(t, "2", "2", 5, 3), v179TestClose(t, 5, 3), v179TestInline(t, "answer", "1", 3, 2), v179TestText(t, "B", "B", 3, 2)}},
		{"intermediate block", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestOpen(t, "hint", 3, 2), v179TestText(t, "Hint", "Hint", 5, 3), v179TestClose(t, 5, 3), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2)}},
		{"answer after separator", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "First", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestInline(t, "answer", "Second", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2)}},
		{"answer after lower item", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "First", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2), v179TestInline(t, "answer", "Second", 3, 2)}},
		{"unparsed lower answer", []model.Line{v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "First", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2), v179TestUnparsed(t, "@ANSWER {oops", "invalid", 3, 2)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v179TestRun(t, v179TestBlock(t, "ordering", tc.body...), nil) })
	}
}

func TestV179ExtendedFormat(t *testing.T) {
	for _, delimiter := range []string{"---", " \t---\t "} {
		t.Run("partial/"+strconv.Quote(delimiter), func(t *testing.T) {
			v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "Second", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, delimiter, 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2)), []v179Want{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}})
		})
	}
	t.Run("all upper items answered", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "Second", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestInline(t, "answer", "First", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2)), nil)
	})
	t.Run("no upper answers", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2)), nil)
	})
	t.Run("several separators", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "Second", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "Second", "Second", 3, 2)), nil)
	})
	t.Run("one upper item", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "Second", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestText(t, "First", "First", 3, 2), v179TestText(t, "Second", "Second", 3, 2)), nil)
	})
	t.Run("unknown lower value cannot add an element", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "First", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestSeparator(t, "---", 3, 2), v179TestUnparsed(t, "x {", "invalid", 3, 2), v179TestText(t, "Second", "Second", 3, 2)), []v179Want{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}})
	})
}

func TestV179TextValuesAndRawRanges(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		end              int
	}{
		{"duplicate", "A", "A", 2},
		{"escaped at", `\@x`, "@x", 4},
		{"escaped hash", `\# x`, "# x", 5},
		{"escaped separator", `\---`, "---", 5},
		{"Unicode and edges", "  Я🌍 ", "Я🌍", 6},
		{"NBSP", "\u00a0", "\u00a0", 2},
		{"internal spacing", "A  b\tC", "A  b\tC", 7},
		{"slash pair then marker", `\\@x`, `\@x`, 5},
		{"escaped braces", `\{x\}`, "{x}", 6},
		{"slash pair then escaped brace", `\\\{`, `\{`, 5},
		{"unknown escape", `\q`, `\q`, 3},
		{"punctuation", "?!", "?!", 3},
		{"Unicode is not trimmed from delimiter", "\u00a0---", "\u00a0---", 5},
	} {
		for _, fallback := range []bool{false, true} {
			t.Run(tc.name+"/raw="+strconv.FormatBool(fallback), func(t *testing.T) {
				row := v179TestText(t, tc.raw, tc.value, 3, 2)
				if fallback {
					row.Elements = []model.Element{}
				}
				v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), row), []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, tc.end}}}})
			})
		}
	}
	t.Run("many content fragments one item", func(t *testing.T) {
		row := v179TestLine(t, "A B", "content", 3, 2, v179TestElement(t, "content", "A", "A", 1), v179TestElement(t, "content", "B", "B", 3))
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), row), []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 4}}}})
	})
}

func TestV179ZeroLengthContentAnnotations(t *testing.T) {
	for _, annotation := range []struct {
		name     string
		elements []model.Element
	}{
		{"before", []model.Element{v179TestElement(t, "content", "", "", 1), v179TestElement(t, "content", "Spain", "Spain", 1)}},
		{"inside", []model.Element{v179TestElement(t, "content", "Spain", "Spain", 1), v179TestElement(t, "content", "", "", 2)}},
		{"after", []model.Element{v179TestElement(t, "content", "Spain", "Spain", 1), v179TestElement(t, "content", "", "", 6)}},
		{"before inside after", []model.Element{v179TestElement(t, "content", "", "", 1), v179TestElement(t, "content", "Spain", "Spain", 1), v179TestElement(t, "content", "", "", 2), v179TestElement(t, "content", "", "", 6)}},
		{"repeated inside", []model.Element{v179TestElement(t, "content", "Spain", "Spain", 1), v179TestElement(t, "content", "", "", 2), v179TestElement(t, "content", "", "", 2), v179TestElement(t, "content", "", "", 4)}},
		{"inside separate fragments", []model.Element{v179TestElement(t, "content", "Sp", "Sp", 1), v179TestElement(t, "content", "", "", 2), v179TestElement(t, "content", "ain", "ain", 3), v179TestElement(t, "content", "", "", 4)}},
	} {
		t.Run(annotation.name, func(t *testing.T) {
			annotated := v179TestLine(t, "Spain", "content", 3, 2, annotation.elements...)
			for _, tc := range []struct {
				name     string
				body     []model.Line
				unclosed bool
				want     []v179Want
			}{
				{"partial annotated owner", []model.Line{annotated, v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "Italy", "Italy", 3, 2)}, false, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 6}}}}},
				{"partial annotated missing", []model.Line{v179TestText(t, "Italy", "Italy", 3, 2), v179TestInline(t, "answer", "2", 3, 2), annotated}, false, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 6}}}}},
				{"full", []model.Line{annotated, v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "Italy", "Italy", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, false, nil},
				{"no answers", []model.Line{annotated, v179TestText(t, "Italy", "Italy", 3, 2)}, false, nil},
				{"partial block answer", []model.Line{annotated, v179TestOpen(t, "AnSwEr", 3, 2), v179TestText(t, "2", "2", 5, 3), v179TestClose(t, 5, 3), v179TestText(t, "Italy", "Italy", 3, 2)}, false, []v179Want{{[4]int{3, 1, 9, 2}, [][3]int{{8, 1, 6}}}}},
				{"full with block answer", []model.Line{annotated, v179TestOpen(t, "answer", 3, 2), v179TestText(t, "2", "2", 5, 3), v179TestClose(t, 5, 3), v179TestText(t, "Italy", "Italy", 3, 2), v179TestInline(t, "answer", "1", 3, 2)}, false, nil},
				{"unclosed partial", []model.Line{annotated, v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "Italy", "Italy", 3, 2)}, true, []v179Want{{[4]int{3, 1, 6, 6}, [][3]int{{6, 1, 6}}}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					result := v179TestBlock(t, "ordering", tc.body...)
					if tc.unclosed {
						result = v179TestDocument(t, append([]model.Line{v179TestOpen(t, "ordering", 2, 1)}, tc.body...)...)
						v179TestParserError(t, result, 3)
					}
					v179TestRun(t, result, tc.want)
				})
			}
		})
	}
}

func TestV179BoundariesAndUncertainty(t *testing.T) {
	t.Run("unclosed ordering", func(t *testing.T) {
		result := v179TestDocument(t, v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2))
		v179TestParserError(t, result, 3)
		v179TestRun(t, result, []v179Want{{[4]int{3, 1, 6, 2}, [][3]int{{6, 1, 2}}}})
	})
	t.Run("unclosed ends in descendant", func(t *testing.T) {
		result := v179TestDocument(t, v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestOpen(t, "hint", 3, 2), v179TestText(t, "Help", "Help", 7, 3))
		v179TestParserError(t, result, 3)
		v179TestParserError(t, result, 7)
		v179TestRun(t, result, []v179Want{{[4]int{3, 1, 8, 5}, [][3]int{{6, 1, 2}}}})
	})
	t.Run("no search beyond container", func(t *testing.T) {
		v179TestRun(t, v179TestDocument(t, v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestClose(t, 3, 2), v179TestInline(t, "answer", "1", 2, 1)), []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}})
	})
	t.Run("unclosed stops at next structural area", func(t *testing.T) {
		heading := v179TestLine(t, "# Next", "heading", 0, 0, v179TestElement(t, "heading-level", "#", "1", 1), v179TestElement(t, "title", "Next", "Next", 3))
		result := v179TestDocument(t, v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), heading, v179TestInline(t, "answer", "1", 7, 1))
		v179TestParserError(t, result, 3)
		v179TestRun(t, result, []v179Want{{[4]int{3, 1, 6, 2}, [][3]int{{6, 1, 2}}}})
	})
	t.Run("ambiguous closing links", func(t *testing.T) {
		// Допустимая контрактом модель ошибки границ, как в тестах V035.
		result := v179TestDocument(t, v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestClose(t, 3, 2), v179TestClose(t, 3, 2))
		v179TestParserError(t, result, 8)
		v179TestRun(t, result, nil)
	})
	t.Run("uncertain ordinary row", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestUnparsed(t, "B {", "invalid", 3, 2), v179TestText(t, "C", "C", 3, 2)), nil)
	})
	t.Run("uncertain owner", func(t *testing.T) {
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestUnparsed(t, "A {", "invalid", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestText(t, "C", "C", 3, 2)), nil)
	})
	t.Run("heading is not an item", func(t *testing.T) {
		heading := v179TestLine(t, "### Heading", "invalid", 3, 2, v179TestElement(t, "heading-level", "###", "3", 1), model.Element{ElementType: "unparsed", Raw: "Heading", Start: 5, End: 12, ErrorIDs: []string{}})
		v179TestRun(t, v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), heading, v179TestText(t, "B", "B", 3, 2)), []v179Want{{[4]int{3, 1, 8, 2}, [][3]int{{7, 1, 2}}}})
	})
}

func TestV179ContextsAndLifecycle(t *testing.T) {
	t.Run("independent blocks", func(t *testing.T) {
		v179TestRun(t, v179TestDocument(t,
			v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestClose(t, 3, 2),
			v179TestOpen(t, "ordering", 2, 1), v179TestText(t, "A", "A", 8, 2), v179TestInline(t, "answer", "2", 8, 2), v179TestText(t, "B", "B", 8, 2), v179TestClose(t, 8, 2)), []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}, {[4]int{8, 1, 12, 2}, [][3]int{{11, 1, 2}}}})
	})
	t.Run("branch at depth", func(t *testing.T) {
		first := v179TestLine(t, "@variant A", "tag", 3, 2, v179TestElement(t, "tag", "@variant", "variant", 1), v179TestElement(t, "name", "A", "A", 10))
		second := v179TestLine(t, "@variant B", "tag", 3, 2, v179TestElement(t, "tag", "@variant", "variant", 1), v179TestElement(t, "name", "B", "B", 10))
		v179TestRun(t, v179TestDocument(t, v179TestOpen(t, "variants", 2, 1), first, v179TestOpen(t, "ordering", 4, 3), v179TestText(t, "A", "A", 5, 4), v179TestInline(t, "answer", "2", 5, 4), v179TestText(t, "B", "B", 5, 4), v179TestClose(t, 5, 4), second, v179TestText(t, "Read.", "Read.", 10, 3), v179TestClose(t, 3, 2)), []v179Want{{[4]int{5, 1, 9, 2}, [][3]int{{8, 1, 2}}}})
	})
	t.Run("case and complete raw line ranges", func(t *testing.T) {
		opening := v179TestLine(t, "\t  @ORDERING {", "block-start", 2, 1, v179TestElement(t, "tag", "@ORDERING", "ordering", 4), v179TestElement(t, "block-open", "{", "{", 14))
		closing := v179TestLine(t, " \t} ", "block-end", 3, 2, v179TestElement(t, "block-close", "}", "}", 3))
		v179TestRun(t, v179TestDocument(t, opening, v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "AnSwEr", "2", 3, 2), v179TestText(t, "  Я🌍 ", "Я🌍", 3, 2), closing), []v179Want{{[4]int{3, 1, 7, 5}, [][3]int{{6, 1, 6}}}})
	})
	t.Run("instruction is not an item", func(t *testing.T) {
		opening := v179TestLine(t, "@ordering Arrange. {", "block-start", 2, 1, v179TestElement(t, "tag", "@ordering", "ordering", 1), v179TestElement(t, "content", "Arrange.", "Arrange.", 11), v179TestElement(t, "block-open", "{", "{", 20))
		v179TestRun(t, v179TestDocument(t, opening, v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2), v179TestInline(t, "answer", "1", 3, 2), v179TestClose(t, 3, 2)), nil)
	})
	for _, name := range []string{"choice", "multichoice", "matching", "note"} {
		t.Run("other container/"+name, func(t *testing.T) {
			v179TestRun(t, v179TestBlock(t, name, v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2)), nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal/"+name, func(t *testing.T) {
			v179TestRun(t, v179TestBlock(t, name, v179TestText(t, "@ordering {", "@ordering {", 3, 2), v179TestText(t, "A", "A", 3, 2), v179TestText(t, "@answer 2", "@answer 2", 3, 2), v179TestText(t, "B", "B", 3, 2)), nil)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v179TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v179TestRun(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v179TestBlock(t, "ordering", v179TestText(t, "A", "A", 3, 2), v179TestInline(t, "answer", "2", 3, 2), v179TestText(t, "B", "B", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v179TestRun(t, result, []v179Want{{[4]int{3, 1, 7, 2}, [][3]int{{6, 1, 2}}}})
			})
		}
	}
}

// v179TestRun проверяет полный конвейер, исходные диапазоны и историю модели.
func v179TestRun(t *testing.T, result *model.Result, want []v179Want) {
	t.Helper()
	before := v179TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v179TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v179TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v179-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V179" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V179 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.block[0], Column: expected.block[1]}, End: model.Position{Line: expected.block[2], Column: expected.block[3]}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal || entry.Message != "В @ordering ответы заданы только для части элементов." || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V179 occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, expected)
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
						t.Fatal("block diagnostic linked to element")
					}
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.block[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v179TestSnapshot(t, result)), &restored); err != nil {
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
	if v179TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v179TestUnparsed задаёт неразобранную область; исходную ошибку добавляет Document.
func v179TestUnparsed(t *testing.T, raw string, kind model.LineType, parent, depth int) model.Line {
	t.Helper()
	return v179TestLine(t, raw, kind, parent, depth, model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
}

// v179TestParserError сохраняет исходную ошибку границ блока без фиктивных элементов.
func v179TestParserError(t *testing.T, result *model.Result, number int) {
	t.Helper()
	id := "boundary-" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error", DiagnosticScope: "line", Message: "Неоднозначные границы блока.", RelatedLocations: []model.Location{}, Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}}})
	result.Document.HasErrors, result.Lines[number-1].HasErrors = true, true
}

// v179TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v179TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v179TestLine задаёт структуру строки без повторного разбора DSL.
func v179TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v179TestOpen задаёт блочное объявление без параметров с явным родителем.
func v179TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v179TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v179TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v179TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v179TestClose задаёт закрытие указанного блока.
func v179TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v179TestLine(t, "}", "block-end", parent, depth, v179TestElement(t, "block-close", "}", "}", 1))
}

// v179TestInline задаёт известный тег с обычным содержимым без экранирования.
func v179TestInline(t *testing.T, name, content string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v179TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if content != "" {
		elements = append(elements, v179TestElement(t, "content", content, content, len(name)+3))
		raw += " " + content
	}
	return v179TestLine(t, raw, "tag", parent, depth, elements...)
}

// v179TestText сохраняет исходное и семантическое содержимое независимо.
func v179TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v179TestLine(t, raw, "content", parent, depth, v179TestElement(t, "content", raw, value, 1))
}

// v179TestSeparator задаёт собственную служебную строку без элементов.
func v179TestSeparator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v179TestLine(t, raw, "separator", parent, depth)
}

// v179TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v179TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v179TestOpen(t, name, 2, 1)}, body...)
	return v179TestDocument(t, append(lines, v179TestClose(t, 3, 2))...)
}

// v179TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v179TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v179TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v179TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v179TestElement(t, "version", "1.2", "1.2", 14)),
		v179TestLine(t, "# Lesson", "heading", 0, 0, v179TestElement(t, "heading-level", "#", "1", 1), v179TestElement(t, "title", "Lesson", "Lesson", 3)),
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

// v179TestRegistry использует только полный штатный каталог.
func v179TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v179TestSnapshot сохраняет независимое представление модели для сравнения.
func v179TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

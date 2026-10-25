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

// v170Want задаёт независимые ожидаемые диапазоны: строка, начало, конец.
type v170Want struct {
	primary [3]int
	related [][3]int
}

func TestV170Registration(t *testing.T) {
	registry := v170TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V170" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Значение @answer отсутствует среди вариантов нижнего списка." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V170 registrations = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV170ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, answer, fragment string
		want                   []v170Want
	}{
		{"reference", "да", "@matching {\nСолнце — звезда?\n@answer да\n---\nДа\nНет\n}", nil},
		{"example", "Да!", "@matching {\nСолнце — звезда?\n@answer Да!\n---\nДа\nНет\n}", []v170Want{{primary: [3]int{5, 9, 12}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v170TestSimple(t, tc.answer, tc.answer, "Да", "Да")
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v170TestRun(t, result, tc.want)
			v170TestRun(t, result, tc.want)
		})
	}
}

func TestV170ComparisonAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, answerRaw, answerValue, lowerRaw, lowerValue string
		want                                               []v170Want
	}{
		{"Unicode case", "ЁЖ", "ЁЖ", "ёж", "ёж", nil},
		{"internal spaces", "New  York", "New  York", "New York", "New York", []v170Want{{primary: [3]int{5, 9, 18}}}},
		{"internal tab", "A\tB", "A\tB", "A B", "A B", []v170Want{{primary: [3]int{5, 9, 12}}}},
		{"number retained", "1. Да", "1. Да", "Да", "Да", []v170Want{{primary: [3]int{5, 9, 14}}}},
		{"quotes retained", "«Да»", "«Да»", `"Да"`, `"Да"`, []v170Want{{primary: [3]int{5, 9, 13}}}},
		{"ASCII edges", " \tДа\t ", "Да", " \tДа\t ", "Да", nil},
		{"ASCII edges on missing value", " \tДа!\t ", "Да!", "Да", "Да", []v170Want{{primary: [3]int{5, 11, 14}}}},
		{"NBSP retained", "Да\u00a0", "Да\u00a0", "Да", "Да", []v170Want{{primary: [3]int{5, 9, 12}}}},
		{"punctuation only", "!!!", "!!!", "?", "?", []v170Want{{primary: [3]int{5, 9, 12}}}},
		{"no Unicode normalization", "e\u0301", "e\u0301", "é", "é", []v170Want{{primary: [3]int{5, 9, 11}}}},
		{"Unicode coordinates", "🌍Нет!", "🌍Нет!", "Да", "Да", []v170Want{{primary: [3]int{5, 9, 14}}}},
		{"T retained", "T", "T", "True", "True", []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"F retained", "F", "F", "False", "False", []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"NG retained", "NG", "NG", "Not Given", "Not Given", []v170Want{{primary: [3]int{5, 9, 11}}}},
		{"empty answer", "", "", "Да", "Да", nil},
		{"ASCII empty answer", " \t", "", "Да", "Да", nil},
		{"Unicode empty answer", "\u00a0\u2003", "\u00a0\u2003", "Да", "Да", nil},
		{"escaped braces", `a\{b\}`, "a{b}", `a\{b\}`, "a{b}", nil},
		{"escaped missing brace", `a\{b`, "a{b", "ab", "ab", []v170Want{{primary: [3]int{5, 9, 13}}}},
		{"no double decoding", `a\\b`, `a\b`, "ab", "ab", []v170Want{{primary: [3]int{5, 9, 13}}}},
		{"slash pair then escaped brace", `a\\\{b`, `a\{b`, `a\{b`, "a{b", []v170Want{{primary: [3]int{5, 9, 15}}}},
		{"unknown escape", `a\q`, `a\q`, "aq", "aq", []v170Want{{primary: [3]int{5, 9, 12}}}},
		{"unknown trailing slash", `a\`, `a\`, "a", "a", []v170Want{{primary: [3]int{5, 9, 11}}}},
		{"unknown underscore escape", `\_`, `\_`, "_", "_", []v170Want{{primary: [3]int{5, 9, 11}}}},
		{"quotes have no escape rule", `\"Да\"`, `\"Да\"`, `"Да"`, `"Да"`, []v170Want{{primary: [3]int{5, 9, 15}}}},
		{"escaped lower at", "@x", "@x", `\@x`, "@x", nil},
		{"escaped lower hash", "# x", "# x", `\# x`, "# x", nil},
		{"escaped lower separator", "---", "---", `\---`, "---", nil},
		{"at not special inline", `\@x`, `\@x`, `\\@x`, `\@x`, nil},
	} {
		for _, fallback := range []bool{false, true} {
			t.Run(tc.name+"/fallback="+strconv.FormatBool(fallback), func(t *testing.T) {
				result := v170TestSimple(t, tc.answerRaw, tc.answerValue, tc.lowerRaw, tc.lowerValue)
				if fallback {
					v170TestWithoutContent(t, result)
				}
				v170TestRun(t, result, tc.want)
			})
		}
	}
}

func TestV170BlockAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v170Want
	}{
		{"group repeats", []model.Line{
			v170TestText(t, "Нет!", "Нет!", 5, 3), v170TestText(t, "НЕТ!", "НЕТ!", 5, 3), v170TestText(t, "Другое", "Другое", 5, 3),
		}, []v170Want{{[3]int{6, 1, 5}, [][3]int{{7, 1, 5}}}, {primary: [3]int{8, 1, 7}}}},
		{"matched and repeated missing", []model.Line{
			v170TestText(t, "Да", "Да", 5, 3), v170TestText(t, "Нет!", "Нет!", 5, 3), v170TestText(t, "НЕТ!", "НЕТ!", 5, 3),
		}, []v170Want{{[3]int{7, 1, 5}, [][3]int{{8, 1, 5}}}}},
		{"all missing repeats related once", []model.Line{
			v170TestText(t, "X", "X", 5, 3), v170TestText(t, "x", "x", 5, 3), v170TestText(t, "X", "X", 5, 3),
		}, []v170Want{{[3]int{6, 1, 2}, [][3]int{{7, 1, 2}, {8, 1, 2}}}}},
		{"punctuation distinguishes groups", []model.Line{
			v170TestText(t, "X", "X", 5, 3), v170TestText(t, "X!", "X!", 5, 3),
		}, []v170Want{{primary: [3]int{6, 1, 2}}, {primary: [3]int{7, 1, 3}}}},
		{"empty lines and values", []model.Line{
			v170TestLine(t, " \t", "blank", 5, 3), v170TestText(t, "\u00a0", "\u00a0", 5, 3), v170TestText(t, "Нет", "Нет", 5, 3),
		}, nil},
		{"empty block", nil, nil},
		{"deeper content excluded", []model.Line{
			v170TestText(t, "Нет!", "Нет!", 5, 3), v170TestOpen(t, "hint", 5, 3),
			v170TestText(t, "Missing", "Missing", 7, 4), v170TestClose(t, 7, 4), v170TestText(t, "Да", "Да", 5, 3),
		}, []v170Want{{primary: [3]int{6, 1, 5}}}},
		{"block markers unescaped once", []model.Line{
			v170TestText(t, `\@x`, "@x", 5, 3), v170TestText(t, `\# x`, "# x", 5, 3), v170TestText(t, "---", "---", 5, 3),
		}, []v170Want{{primary: [3]int{6, 1, 4}}, {primary: [3]int{7, 1, 5}}, {primary: [3]int{8, 1, 4}}}},
		{"separator escape is not special in answer", []model.Line{
			v170TestText(t, `\---`, `\---`, 5, 3),
		}, []v170Want{{primary: [3]int{6, 1, 5}}}},
	} {
		for _, fallback := range []bool{false, true} {
			t.Run(tc.name+"/fallback="+strconv.FormatBool(fallback), func(t *testing.T) {
				body := append([]model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestOpen(t, "answer", 3, 2)}, tc.body...)
				body = append(body, v170TestClose(t, 5, 3), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2))
				result := v170TestBlock(t, "matching", body...)
				if fallback {
					v170TestWithoutContent(t, result)
				}
				v170TestRun(t, result, tc.want)
			})
		}
	}
}

func TestV170Ownership(t *testing.T) {
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v170Want
	}{
		{"no answers", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2)}, nil},
		{"orphan answer", []model.Line{v170TestInline(t, "answer", "X", 3, 2)}, nil},
		{"partial answers", []model.Line{v170TestText(t, "First?", "First?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestText(t, "Second?", "Second?", 3, 2)}, nil},
		{"repeated answer", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestInline(t, "answer", "X", 3, 2)}, nil},
		{"blank skipped", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestLine(t, " \t", "blank", 3, 2), v170TestInline(t, "answer", "Да!", 3, 2)}, []v170Want{{primary: [3]int{6, 9, 12}}}},
		{"hint not skipped", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "hint", "Hint", 3, 2), v170TestInline(t, "answer", "X", 3, 2)}, nil},
		{"editor not skipped", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "editor", "Comment", 3, 2), v170TestInline(t, "answer", "X", 3, 2)}, nil},
		{"linked hint", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "answer", "Да!", 3, 2), v170TestInline(t, "hint", "Missing", 3, 2)}, []v170Want{{primary: [3]int{5, 9, 12}}}},
		{"linked block hint with separator", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestOpen(t, "hint", 3, 2), v170TestText(t, "---", "---", 6, 3), v170TestClose(t, 6, 3)}, []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"second hint breaks association", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestInline(t, "hint", "Hint", 3, 2), v170TestInline(t, "hint", "Hint", 3, 2)}, nil},
		{"no grouping across answers", []model.Line{v170TestText(t, "First?", "First?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestText(t, "Second?", "Second?", 3, 2), v170TestInline(t, "answer", "x", 3, 2)}, []v170Want{{primary: [3]int{5, 9, 10}}, {primary: [3]int{7, 9, 10}}}},
		{"nested answer is not owner", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestOpen(t, "hint", 3, 2), v170TestInline(t, "answer", "X", 5, 3), v170TestClose(t, 5, 3)}, nil},
		{"escaped answer is ordinary text", []model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestText(t, `\@answer X`, "@answer X", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(append([]model.Line{}, tc.upper...), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2))
			v170TestRun(t, v170TestBlock(t, "matching", body...), tc.want)
		})
	}
}

func TestV170LowerListAndSeparators(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lower []model.Line
		want  []v170Want
	}{
		{"no separator", []model.Line{v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2)}, nil},
		{"two separators", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2)}, nil},
		{"ASCII separator edges", []model.Line{v170TestSeparator(t, " \t---\t ", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2)}, []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"escaped is not separator", []model.Line{v170TestText(t, `\---`, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2)}, nil},
		{"Unicode edges are not separator", []model.Line{v170TestText(t, "\u00a0---", "\u00a0---", 3, 2), v170TestText(t, "Да", "Да", 3, 2)}, nil},
		{"duplicate matching lower labels", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestText(t, "x", "x", 3, 2), v170TestText(t, "X", "X", 3, 2)}, nil},
		{"single lower label still checked", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2)}, []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"last lower label matches", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "X", "X", 3, 2)}, nil},
		{"nested labels excluded", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestOpen(t, "hint", 3, 2), v170TestText(t, "X", "X", 7, 3), v170TestClose(t, 7, 3), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2)}, []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"inline tags are not options", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestInline(t, "hint", "X", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2)}, []v170Want{{primary: [3]int{5, 9, 10}}}},
		{"ambiguous lower content", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestLine(t, "A B", "content", 3, 2, v170TestElement(t, "content", "A", "A", 1), v170TestElement(t, "content", "B", "B", 3)), v170TestText(t, "Нет", "Нет", 3, 2)}, nil},
		{"unparsed lower value", []model.Line{v170TestSeparator(t, "---", 3, 2), v170TestLine(t, "{", "invalid", 3, 2, model.Element{ElementType: "unparsed", Raw: "{", Start: 1, End: 2, ErrorIDs: []string{}}), v170TestText(t, "Нет", "Нет", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "answer", "X", 3, 2)}, tc.lower...)
			v170TestRun(t, v170TestBlock(t, "matching", body...), tc.want)
		})
	}
}

// v170TestSimple задаёт полноценный matching с независимыми Raw и Value.
func v170TestSimple(t *testing.T, answerRaw, answerValue, lowerRaw, lowerValue string) *model.Result {
	t.Helper()
	answer := v170TestInline(t, "answer", answerRaw, 3, 2)
	if len(answer.Elements) > 1 {
		answer.Elements[1].Value = &answerValue
	}
	return v170TestBlock(t, "matching", v170TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), answer,
		v170TestSeparator(t, "---", 3, 2), v170TestText(t, lowerRaw, lowerValue, 3, 2), v170TestText(t, "Нет", "Нет", 3, 2))
}

func TestV170ContainersAndBoundaries(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v170TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v170TestRun(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+"/final="+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v170TestSimple(t, "Да!", "Да!", "Да", "Да")
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v170TestRun(t, result, []v170Want{{primary: [3]int{5, 9, 12}}})
			})
		}
	}
	t.Run("case insensitive tag names", func(t *testing.T) {
		v170TestRun(t, v170TestBlock(t, "MaTcHiNg", v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "AnSwEr", "X", 3, 2), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2)), []v170Want{{primary: [3]int{5, 9, 10}}})
	})
	t.Run("instruction not a lower label", func(t *testing.T) {
		result := v170TestSimple(t, "X", "X", "Да", "Да")
		result.Lines[2] = v170TestLine(t, "@matching X {", "block-start", 2, 1,
			v170TestElement(t, "tag", "@matching", "matching", 1), v170TestElement(t, "content", "X", "X", 11), v170TestElement(t, "block-open", "{", "{", 13))
		result.Lines[2].Number = 3
		v170TestRun(t, result, []v170Want{{primary: [3]int{5, 9, 10}}})
	})
	t.Run("different block kind", func(t *testing.T) {
		v170TestRun(t, v170TestBlock(t, "ordering", v170TestText(t, "First", "First", 3, 2), v170TestInline(t, "answer", "1", 3, 2),
			v170TestText(t, "Second", "Second", 3, 2), v170TestInline(t, "answer", "2", 3, 2), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "First", "First", 3, 2), v170TestText(t, "Second", "Second", 3, 2)), nil)
	})
	t.Run("independent first and last blocks", func(t *testing.T) {
		v170TestRun(t, v170TestDocument(t,
			v170TestOpen(t, "matching", 2, 1), v170TestText(t, "First?", "First?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2), v170TestClose(t, 3, 2),
			v170TestOpen(t, "matching", 2, 1), v170TestText(t, "Second?", "Second?", 10, 2), v170TestInline(t, "answer", "Да", 10, 2), v170TestSeparator(t, "---", 10, 2), v170TestText(t, "X", "X", 10, 2), v170TestText(t, "Y", "Y", 10, 2), v170TestClose(t, 10, 2)),
			[]v170Want{{primary: [3]int{5, 9, 10}}, {primary: [3]int{12, 9, 11}}})
	})
	t.Run("answer beyond closing boundary", func(t *testing.T) {
		v170TestRun(t, v170TestDocument(t, v170TestOpen(t, "matching", 2, 1), v170TestText(t, "Question?", "Question?", 3, 2), v170TestSeparator(t, "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2), v170TestText(t, "Нет", "Нет", 3, 2), v170TestClose(t, 3, 2), v170TestInline(t, "answer", "X", 2, 1)), nil)
	})
	t.Run("branch at depth", func(t *testing.T) {
		first := v170TestLine(t, "@variant A", "tag", 3, 2, v170TestElement(t, "tag", "@variant", "variant", 1), v170TestElement(t, "name", "A", "A", 10))
		second := v170TestLine(t, "@variant B", "tag", 3, 2, v170TestElement(t, "tag", "@variant", "variant", 1), v170TestElement(t, "name", "B", "B", 10))
		v170TestRun(t, v170TestDocument(t, v170TestOpen(t, "variants", 2, 1), first, v170TestOpen(t, "matching", 4, 3),
			v170TestText(t, "Question?", "Question?", 5, 4), v170TestInline(t, "answer", "X", 5, 4), v170TestSeparator(t, "---", 5, 4), v170TestText(t, "Да", "Да", 5, 4), v170TestText(t, "Нет", "Нет", 5, 4), v170TestClose(t, 5, 4),
			second, v170TestText(t, "Read.", "Read.", 12, 3), v170TestClose(t, 3, 2)), []v170Want{{primary: [3]int{7, 9, 10}}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			v170TestRun(t, v170TestBlock(t, name, v170TestText(t, "@matching {", "@matching {", 3, 2), v170TestText(t, "@answer X", "@answer X", 3, 2), v170TestText(t, "---", "---", 3, 2), v170TestText(t, "Да", "Да", 3, 2)), nil)
		})
	}
}

func TestV170IndeterminateValues(t *testing.T) {
	t.Run("partial lower content", func(t *testing.T) {
		result := v170TestSimple(t, "X", "X", "A B", "A B")
		result.Lines[6].Elements = []model.Element{v170TestElement(t, "content", "A", "A", 1)}
		v170TestRun(t, result, nil)
	})
	t.Run("unknown lower syntax without content", func(t *testing.T) {
		result := v170TestDocument(t, v170TestOpen(t, "matching", 2, 1), v170TestText(t, "Question?", "Question?", 3, 2), v170TestInline(t, "answer", "X", 3, 2), v170TestSeparator(t, "---", 3, 2),
			v170TestLine(t, "{", "invalid", 3, 2, model.Element{ElementType: "unparsed", Raw: "{", Start: 1, End: 2, ErrorIDs: []string{}}), v170TestText(t, "Нет", "Нет", 3, 2), v170TestClose(t, 3, 2))
		v170TestRun(t, result, nil)
	})
	t.Run("ambiguous answer not compared", func(t *testing.T) {
		result := v170TestSimple(t, "A B", "A B", "Да", "Да")
		result.Lines[4].Elements = []model.Element{v170TestElement(t, "tag", "@answer", "answer", 1), v170TestElement(t, "content", "A", "A", 9), v170TestElement(t, "content", "B", "B", 11)}
		v170TestRun(t, result, nil)
	})
}

// v170TestWithoutContent моделирует отсутствие готового элемента, а не null Value.
func v170TestWithoutContent(t *testing.T, result *model.Result) {
	t.Helper()
	for i := range result.Lines {
		line := &result.Lines[i]
		elements := []model.Element{}
		for _, element := range line.Elements {
			if element.ElementType != "content" {
				elements = append(elements, element)
			}
		}
		line.Elements = elements
	}
}

// v170TestRun проверяет V170 через весь штатный конвейер и целостность результата.
func v170TestRun(t *testing.T, result *model.Result, want []v170Want) {
	t.Helper()
	before := v170TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v170TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v170TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v170-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V170" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V170 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := v170TestLocation(expected.primary)
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Значение @answer отсутствует среди вариантов нижнего списка." ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V170 occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, expected)
		}
		for j, related := range expected.related {
			if entry.RelatedLocations[j] != v170TestLocation(related) {
				t.Fatalf("related %d: %+v, want %+v", j, entry.RelatedLocations[j], related)
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
		if !result.Document.HasErrors || !result.Lines[expected.primary[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v170TestSnapshot(t, result)), &restored); err != nil {
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
	if v170TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v170TestLocation переводит явно заданные координаты в тип модели.
func v170TestLocation(position [3]int) model.Location {
	return model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
}

// v170TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v170TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v170TestLine задаёт структуру строки без повторного разбора DSL.
func v170TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v170TestOpen задаёт блочное объявление без параметров с явным родителем.
func v170TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v170TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v170TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v170TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v170TestClose задаёт закрытие указанного блока.
func v170TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v170TestLine(t, "}", "block-end", parent, depth, v170TestElement(t, "block-close", "}", "}", 1))
}

// v170TestInline задаёт известный тег с обычным содержимым без экранирования.
func v170TestInline(t *testing.T, name, content string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v170TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if content != "" {
		elements = append(elements, v170TestElement(t, "content", content, content, len(name)+3))
		raw += " " + content
	}
	return v170TestLine(t, raw, "tag", parent, depth, elements...)
}

// v170TestText сохраняет исходное и семантическое содержимое независимо.
func v170TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v170TestLine(t, raw, "content", parent, depth, v170TestElement(t, "content", raw, value, 1))
}

// v170TestSeparator задаёт собственную служебную строку без элементов.
func v170TestSeparator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v170TestLine(t, raw, "separator", parent, depth)
}

// v170TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v170TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v170TestOpen(t, name, 2, 1)}, body...)
	return v170TestDocument(t, append(lines, v170TestClose(t, 3, 2))...)
}

// v170TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v170TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v170TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v170TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v170TestElement(t, "version", "1.2", "1.2", 14)),
		v170TestLine(t, "# Lesson", "heading", 0, 0, v170TestElement(t, "heading-level", "#", "1", 1), v170TestElement(t, "title", "Lesson", "Lesson", 3)),
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

// v170TestRegistry использует только полный штатный каталог.
func v170TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v170TestSnapshot сохраняет независимое представление модели для сравнения.
func v170TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

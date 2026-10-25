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

type v157Expected struct {
	main    [3]int
	related [][3]int
}

func TestV157Registration(t *testing.T) {
	registry := v157Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V157" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Одинаковый текст варианта указан и как правильный, и как неправильный." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V157 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV157ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           []v157Expected
	}{
		{"reference", "@choice {\n@answer Spain\nFrance\n}", []model.Line{
			v157Inline(t, "answer", "Spain", "Spain", 3, 2), v157Text(t, "France", "France", 3, 2)}, nil},
		{"example", "@choice {\n@answer Spain\nSpain\nFrance\n}", []model.Line{
			v157Inline(t, "answer", "Spain", "Spain", 3, 2), v157Text(t, "Spain", "Spain", 3, 2), v157Text(t, "France", "France", 3, 2)},
			[]v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 14}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v157Block(t, "choice", tc.rows...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v157Run(t, result, tc.want)
			v157Run(t, result, tc.want) // История не влияет на следующий запуск.
		})
	}
}

func TestV157Normalization(t *testing.T) {
	// Raw и Value задаются независимо. Ожидания — исходные диапазоны,
	// а не результаты вычисления длины нормализованных подписей.
	for _, tc := range []struct {
		name                       string
		answerRaw, answerValue     string
		ordinaryRaw, ordinaryValue string
		want                       []v157Expected
	}{
		{"case", "Spain", "Spain", "SPAIN", "SPAIN", []v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 14}}}}},
		{"case and punctuation", "Spain", "Spain", "SPAIN!", "SPAIN!", []v157Expected{{[3]int{5, 1, 7}, [][3]int{{4, 9, 14}}}}},
		{"Unicode case", "ЁЖ", "ЁЖ", "ёж", "ёж", []v157Expected{{[3]int{5, 1, 3}, [][3]int{{4, 9, 11}}}}},
		{"internal spaces", "New York", "New York", "New  York", "New  York", []v157Expected{{[3]int{5, 1, 10}, [][3]int{{4, 9, 17}}}}},
		{"tab", "New York", "New York", "New\tYork", "New\tYork", []v157Expected{{[3]int{5, 1, 9}, [][3]int{{4, 9, 17}}}}},
		{"NBSP", "New York", "New York", "New\u00a0York", "New\u00a0York", []v157Expected{{[3]int{5, 1, 9}, [][3]int{{4, 9, 17}}}}},
		{"Unicode space sequence", "a b", "a b", "a\u0085\u2003\u202f\u3000b", "a\u0085\u2003\u202f\u3000b", []v157Expected{{[3]int{5, 1, 7}, [][3]int{{4, 9, 12}}}}},
		{"Unicode edge spaces", "Spain", "Spain", "\u00a0Spain\u2003", "\u00a0Spain\u2003", []v157Expected{{[3]int{5, 1, 8}, [][3]int{{4, 9, 14}}}}},
		{"spaces after punctuation removal", "a b", "a b", "a , b", "a , b", []v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 12}}}}},
		{"punctuation removal joins letters", "ab", "ab", "a-b", "a-b", []v157Expected{{[3]int{5, 1, 4}, [][3]int{{4, 9, 11}}}}},
		{"Unicode punctuation", "Spain", "Spain", "«Spain»—", "«Spain»—", []v157Expected{{[3]int{5, 1, 9}, [][3]int{{4, 9, 14}}}}},
		{"identical punctuation only", "!!!", "!!!", "!!!", "!!!", []v157Expected{{[3]int{5, 1, 4}, [][3]int{{4, 9, 12}}}}},
		{"different punctuation only", "!!!", "!!!", "???", "???", nil},
		{"punctuation only versus text", "!!!", "!!!", "Spain", "Spain", nil},
		{"empty normalized answer", "\u00a0", "\u00a0", "!!!", "!!!", nil},
		{"empty normalized ordinary", "!!!", "!!!", "\u00a0", "\u00a0", nil},
		{"both empty after normalization", "\u00a0", "\u00a0", "\u2003", "\u2003", nil},
		{"digits preserved", "a1", "a1", "a2", "a2", nil},
		{"symbols preserved", "a+", "a+", "a", "a", nil},
		{"emoji preserved", "a🌍", "a🌍", "a", "a", nil},
		{"diacritics preserved", "é", "é", "e", "e", nil},
		{"no canonical normalization", "é", "é", "e\u0301", "e\u0301", nil},
		{"lowercase is not full case folding", "ß", "ß", "ss", "ss", nil},
		{"format character preserved", "ab", "ab", "a\u200bb", "a\u200bb", nil},
		{"T not expanded", "T", "T", "True", "True", nil},
		{"F not expanded", "F", "F", "False", "False", nil},
		{"NG not expanded", "NG", "NG", "Not Given", "Not Given", nil},
		{"T lowercased", "T", "T", "t", "t", []v157Expected{{[3]int{5, 1, 2}, [][3]int{{4, 9, 10}}}}},
		{"escaped braces", `a\{b\}`, "a{b}", "ab", "ab", []v157Expected{{[3]int{5, 1, 3}, [][3]int{{4, 9, 15}}}}},
		{"escaped slash and unknown escape", `a\\b`, `a\b`, `a\b`, `a\b`, []v157Expected{{[3]int{5, 1, 4}, [][3]int{{4, 9, 13}}}}},
		{"escaped tag stays text", "@answer Spain", "@answer Spain", `\@answer Spain`, "@answer Spain", []v157Expected{{[3]int{5, 1, 15}, [][3]int{{4, 9, 22}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v157Run(t, v157Block(t, "choice",
				v157Inline(t, "answer", tc.answerRaw, tc.answerValue, 3, 2),
				v157Text(t, tc.ordinaryRaw, tc.ordinaryValue, 3, 2)), tc.want)
		})
	}
}

func TestV157Grouping(t *testing.T) {
	answer := func(label string) model.Line { t.Helper(); return v157Inline(t, "answer", label, label, 3, 2) }
	text := func(label string) model.Line { t.Helper(); return v157Text(t, label, label, 3, 2) }
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v157Expected
	}{
		{"reverse order", []model.Line{text("Spain"), answer("Spain")}, []v157Expected{{[3]int{5, 9, 14}, [][3]int{{4, 1, 6}}}}},
		{"all occurrences once", []model.Line{text("Spain"), text("Spain"), answer("Spain"), answer("Spain")},
			[]v157Expected{{[3]int{6, 9, 14}, [][3]int{{4, 1, 6}, {5, 1, 6}, {7, 9, 14}}}}},
		{"mixed spellings one group", []model.Line{answer("Spain"), answer("SPAIN!"), text("spain?"), text("Spain")},
			[]v157Expected{{[3]int{6, 1, 7}, [][3]int{{4, 9, 14}, {5, 9, 15}, {7, 1, 6}}}}},
		{"two groups", []model.Line{answer("Spain"), text("Spain"), answer("France"), text("France")},
			[]v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 14}}}, {[3]int{7, 1, 7}, [][3]int{{6, 9, 15}}}}},
		{"only correct duplicates", []model.Line{answer("Spain"), answer("SPAIN!"), text("France")}, nil},
		{"only incorrect duplicates", []model.Line{answer("France"), text("Spain"), text("SPAIN!")}, nil},
		{"no answer", []model.Line{text("Spain"), text("SPAIN!")}, nil},
		{"no options", nil, nil},
		{"one option", []model.Line{answer("Spain")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v157Run(t, v157Block(t, "multichoice", tc.rows...), tc.want) })
	}
}

func TestV157BlocksAndExclusions(t *testing.T) {
	for _, name := range []string{"choice", "multichoice"} {
		t.Run(name+" block answer", func(t *testing.T) {
			result := v157Block(t, name, v157Open(t, "answer", 3, 2),
				v157Text(t, "Spain", "Spain", 4, 3), v157Text(t, "France", "France", 4, 3),
				v157Close(t, 4, 3), v157Text(t, "France", "France", 3, 2))
			// В choice лишняя строка ответа — отдельное нарушение V139;
			// V157 по своему условию всё равно сравнивает каждую строку ответа.
			v157Run(t, result, []v157Expected{{[3]int{8, 1, 7}, [][3]int{{6, 1, 7}}}})
		})
	}
	t.Run("separate blocks", func(t *testing.T) {
		v157Run(t, v157Document(t,
			v157Open(t, "choice", 2, 1), v157Inline(t, "answer", "Spain", "Spain", 3, 2), v157Text(t, "France", "France", 3, 2), v157Close(t, 3, 2),
			v157Open(t, "multichoice", 2, 1), v157Inline(t, "answer", "Italy", "Italy", 7, 2), v157Text(t, "Spain", "Spain", 7, 2), v157Close(t, 7, 2)), nil)
	})
	t.Run("nested in variants", func(t *testing.T) {
		// По 4.9.4–4.9.8 нужны две именованные ветви; choice принадлежит первой.
		first := v157Line(t, "@variant First", "tag", 3, 2,
			v157Element(t, "tag", "@variant", "variant", 1), v157Element(t, "name", "First", "First", 10))
		second := v157Line(t, "@variant Second", "tag", 3, 2,
			v157Element(t, "tag", "@variant", "variant", 1), v157Element(t, "name", "Second", "Second", 10))
		v157Run(t, v157Document(t, v157Open(t, "variants", 2, 1), first, v157Open(t, "choice", 4, 3),
			v157Inline(t, "answer", "Spain", "Spain", 5, 4), v157Text(t, "Spain", "Spain", 5, 4),
			v157Close(t, 5, 4), second, v157Text(t, "Read the text.", "Read the text.", 9, 3), v157Close(t, 3, 2)),
			[]v157Expected{{[3]int{7, 1, 6}, [][3]int{{6, 9, 14}}}})
	})
	t.Run("instruction inline and block hints", func(t *testing.T) {
		opening := v157Line(t, "@choice Spain {", "block-start", 2, 1,
			v157Element(t, "tag", "@choice", "choice", 1), v157Element(t, "content", "Spain", "Spain", 9), v157Element(t, "block-open", "{", "{", 15))
		v157Run(t, v157Document(t, opening, v157Inline(t, "answer", "Spain", "Spain", 3, 2),
			v157Open(t, "hint", 3, 2), v157Text(t, "Spain", "Spain", 5, 3), v157Close(t, 5, 3),
			v157Text(t, "France", "France", 3, 2), v157Close(t, 3, 2)), nil)
		v157Run(t, v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2),
			v157Inline(t, "hint", "Spain", "Spain", 3, 2), v157Text(t, "France", "France", 3, 2)), nil)
	})
	t.Run("other nested contents", func(t *testing.T) {
		result := v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2),
			v157Open(t, "text", 3, 2), v157Text(t, "Spain", "Spain", 5, 3), v157Close(t, 5, 3),
			v157Text(t, "France", "France", 3, 2))
		v157ParserError(t, result, 5, -1) // Запрещённая конструкция, не новый вариант.
		v157Run(t, result, nil)
	})
	t.Run("nested contents of answer", func(t *testing.T) {
		result := v157Block(t, "choice", v157Open(t, "answer", 3, 2), v157Text(t, "France", "France", 4, 3),
			v157Open(t, "hint", 4, 3), v157Text(t, "Spain", "Spain", 6, 4), v157Close(t, 6, 4), v157Close(t, 4, 3),
			v157Text(t, "Spain", "Spain", 3, 2))
		v157ParserError(t, result, 6, -1)
		v157Run(t, result, nil)
	})
	t.Run("literal declarations", func(t *testing.T) {
		v157Run(t, v157Document(t, v157Open(t, "editor", 2, 1),
			v157Text(t, "@choice \\{", "@choice {", 3, 2), v157Text(t, "@answer Spain", "@answer Spain", 3, 2),
			v157Text(t, "Spain", "Spain", 3, 2), v157Text(t, "\\}", "}", 3, 2), v157Close(t, 3, 2)), nil)
	})
}

func TestV157SourceRangesAndBoundaries(t *testing.T) {
	t.Run("missing content and blank lines", func(t *testing.T) {
		// Пустая физическая строка по 1.6 является blank, а не вариантом.
		answer := v157Line(t, "@answer", "tag", 3, 2, v157Element(t, "tag", "@answer", "answer", 1))
		v157Run(t, v157Block(t, "choice", answer, v157Line(t, "", "blank", 3, 2),
			v157Line(t, " \t", "blank", 3, 2), v157Text(t, "Spain", "Spain", 3, 2), v157Text(t, "France", "France", 3, 2)), nil)
	})
	t.Run("empty input", func(t *testing.T) {
		result := v157Document(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		v157Run(t, result, nil)
	})
	t.Run("edge spaces excluded by content positions", func(t *testing.T) {
		answer := v157Line(t, " \t@ANSWER \tSpain\t ", "tag", 3, 2,
			v157Element(t, "tag", "@ANSWER", "answer", 3), v157Element(t, "content", "Spain", "Spain", 12))
		ordinary := v157Line(t, "\t Spain \t", "content", 3, 2, v157Element(t, "content", "Spain", "Spain", 3))
		v157Run(t, v157Block(t, "choice", answer, v157Line(t, " \t", "blank", 3, 2), ordinary),
			[]v157Expected{{[3]int{6, 3, 8}, [][3]int{{4, 12, 17}}}})
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2), v157Text(t, "Spain", "Spain", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v157Run(t, result, []v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 14}}}})
			})
		}
	}
}

func TestV157ZeroWidthContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		parts     []model.Element
		ordinary  [3]int
		answer    [3]int
	}{
		{"before content", "Spain", []model.Element{
			v157Element(t, "content", "", "", 1), v157Element(t, "content", "Spain", "Spain", 1)}, [3]int{5, 1, 6}, [3]int{5, 9, 14}},
		{"inside content", "Spain", []model.Element{
			v157Element(t, "content", "Spain", "Spain", 1), v157Element(t, "content", "", "", 2)}, [3]int{5, 1, 6}, [3]int{5, 9, 14}},
		{"several inside content", "Spain", []model.Element{
			v157Element(t, "content", "Spain", "Spain", 1), v157Element(t, "content", "", "", 2),
			v157Element(t, "content", "", "", 2), v157Element(t, "content", "", "", 5)}, [3]int{5, 1, 6}, [3]int{5, 9, 14}},
		{"after content", "Spain", []model.Element{
			v157Element(t, "content", "Spain", "Spain", 1), v157Element(t, "content", "", "", 6)}, [3]int{5, 1, 6}, [3]int{5, 9, 14}},
		{"between adjacent fragments", "Spain", []model.Element{
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "", "", 2),
			v157Element(t, "content", "", "", 3), v157Element(t, "content", "ain", "ain", 3),
			v157Element(t, "content", "", "", 5), v157Element(t, "content", "", "", 6)}, [3]int{5, 1, 6}, [3]int{5, 9, 14}},
		{"outside source range", "  Spain  ", []model.Element{
			v157Element(t, "content", "", "", 1), v157Element(t, "content", "Spain", "Spain", 3),
			v157Element(t, "content", "", "", 9)}, [3]int{5, 3, 8}, [3]int{5, 11, 16}},
	} {
		for _, block := range []string{"choice", "multichoice"} {
			t.Run(block+" "+tc.name, func(t *testing.T) {
				ordinary := v157Line(t, tc.raw, "content", 3, 2, tc.parts...)
				v157Run(t, v157Block(t, block, v157Inline(t, "answer", "Spain", "Spain", 3, 2), ordinary),
					[]v157Expected{{tc.ordinary, [][3]int{{4, 9, 14}}}})
				// Те же исходные фрагменты проверяются в прямом answer.
				parts := []model.Element{v157Element(t, "tag", "@answer", "answer", 1)}
				for _, element := range tc.parts {
					element.Start, element.End = element.Start+8, element.End+8
					parts = append(parts, element)
				}
				answer := v157Line(t, "@answer "+tc.raw, "tag", 3, 2, parts...)
				v157Run(t, v157Block(t, block, v157Text(t, "Spain", "Spain", 3, 2), answer),
					[]v157Expected{{tc.answer, [][3]int{{4, 1, 6}}}})
			})
		}
	}
	t.Run("inside tag range", func(t *testing.T) {
		answer := v157Line(t, "@answer Spain", "tag", 3, 2,
			v157Element(t, "tag", "@answer", "answer", 1), v157Element(t, "content", "", "", 2),
			v157Element(t, "content", "Spain", "Spain", 9))
		v157Run(t, v157Block(t, "choice", v157Text(t, "Spain", "Spain", 3, 2), answer),
			[]v157Expected{{[3]int{5, 9, 14}, [][3]int{{4, 1, 6}}}})
	})
	t.Run("normalized empty content", func(t *testing.T) {
		ordinary := v157Line(t, "\u00a0", "content", 3, 2,
			v157Element(t, "content", "", "", 1), v157Element(t, "content", "\u00a0", "\u00a0", 1),
			v157Element(t, "content", "", "", 2))
		v157Run(t, v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2), ordinary), nil)
	})
}

func TestV157CompositeContent(t *testing.T) {
	for _, tc := range []struct {
		name, answerRaw, answerValue, raw string
		parts                             []model.Element
		want                              []v157Expected
	}{
		{"adjacent", "Spain", "Spain", "Spain", []model.Element{
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "ain", "ain", 3)},
			[]v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 14}}}}},
		{"three fragments", "Spain", "Spain", "Spain", []model.Element{
			v157Element(t, "content", "S", "S", 1), v157Element(t, "content", "pa", "pa", 2), v157Element(t, "content", "in", "in", 4)},
			[]v157Expected{{[3]int{5, 1, 6}, [][3]int{{4, 9, 14}}}}},
		{"source spaces and tab", "New York", "New York", "\t New \t York \t", []model.Element{
			v157Element(t, "content", "New", "New", 3), v157Element(t, "content", "York", "York", 9)},
			[]v157Expected{{[3]int{5, 3, 13}, [][3]int{{4, 9, 17}}}}},
		{"empty element inside source gap", "New York", "New York", "New \t York", []model.Element{
			v157Element(t, "content", "New", "New", 1), v157Element(t, "content", "", "", 5), v157Element(t, "content", "York", "York", 7)},
			[]v157Expected{{[3]int{5, 1, 11}, [][3]int{{4, 9, 17}}}}},
		{"spaces in values preserved until assembly", "New York", "New York", "New York", []model.Element{
			v157Element(t, "content", "New ", "New ", 1), v157Element(t, "content", "York", "York", 5)},
			[]v157Expected{{[3]int{5, 1, 9}, [][3]int{{4, 9, 17}}}}},
		{"Unicode whitespace and case", "ёж юг", "ёж юг", "ЁЖ\u00a0\u2003ЮГ", []model.Element{
			v157Element(t, "content", "ЁЖ\u00a0", "ЁЖ\u00a0", 1), v157Element(t, "content", "\u2003ЮГ", "\u2003ЮГ", 4)},
			[]v157Expected{{[3]int{5, 1, 7}, [][3]int{{4, 9, 14}}}}},
		{"Unicode punctuation", "ёж", "ёж", "«ЁЖ»", []model.Element{
			v157Element(t, "content", "«Ё", "«Ё", 1), v157Element(t, "content", "Ж»", "Ж»", 3)},
			[]v157Expected{{[3]int{5, 1, 5}, [][3]int{{4, 9, 11}}}}},
		{"punctuation only strict match", "!!!", "!!!", "!!!", []model.Element{
			v157Element(t, "content", "!", "!", 1), v157Element(t, "content", "!!", "!!", 2)},
			[]v157Expected{{[3]int{5, 1, 4}, [][3]int{{4, 9, 12}}}}},
		{"punctuation only different", "???", "???", "!!!", []model.Element{
			v157Element(t, "content", "!", "!", 1), v157Element(t, "content", "!!", "!!", 2)}, nil},
		{"escaped values remain authoritative", "a\\{b\\}", "a{b}", "a\\{b\\}", []model.Element{
			v157Element(t, "content", "a\\{", "a{", 1), v157Element(t, "content", "b\\}", "b}", 4)},
			[]v157Expected{{[3]int{5, 1, 7}, [][3]int{{4, 9, 15}}}}},
		{"no repeated unescape strict punctuation", "\\\\\\{", "\\{", "\\\\\\{", []model.Element{
			v157Element(t, "content", "\\\\", "\\", 1), v157Element(t, "content", "\\{", "{", 3)},
			[]v157Expected{{[3]int{5, 1, 5}, [][3]int{{4, 9, 13}}}}},
		{"no repeated unescape cannot erase slash", "\\{", "{", "\\\\\\{", []model.Element{
			v157Element(t, "content", "\\\\", "\\", 1), v157Element(t, "content", "\\{", "{", 3)}, nil},
		{"Unicode source columns", "🌍ёж", "🌍ёж", "🌍ЁЖ", []model.Element{
			v157Element(t, "content", "🌍", "🌍", 1), v157Element(t, "content", "ЁЖ", "ЁЖ", 2)},
			[]v157Expected{{[3]int{5, 1, 4}, [][3]int{{4, 9, 12}}}}},
		{"normalized empty", "!!!", "!!!", "\u00a0\u2003", []model.Element{
			v157Element(t, "content", "\u00a0", "\u00a0", 1), v157Element(t, "content", "\u2003", "\u2003", 2)}, nil},
		{"source gap is not omitted", "NewYork", "NewYork", "New York", []model.Element{
			v157Element(t, "content", "New", "New", 1), v157Element(t, "content", "York", "York", 5)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ordinary := v157Line(t, tc.raw, "content", 3, 2, tc.parts...)
			v157Run(t, v157Block(t, "choice", v157Inline(t, "answer", tc.answerRaw, tc.answerValue, 3, 2), ordinary), tc.want)
		})
	}
	t.Run("inline and block answers form one group", func(t *testing.T) {
		ordinary := v157Line(t, "Spain", "content", 3, 2,
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "ain", "ain", 3))
		inline := v157Line(t, "@answer Spain", "tag", 3, 2, v157Element(t, "tag", "@answer", "answer", 1),
			v157Element(t, "content", "Sp", "Sp", 9), v157Element(t, "content", "ain", "ain", 11))
		blockLine := v157Line(t, "SPAIN!", "content", 6, 3,
			v157Element(t, "content", "SPA", "SPA", 1), v157Element(t, "content", "", "", 2),
			v157Element(t, "content", "IN!", "IN!", 4))
		v157Run(t, v157Block(t, "multichoice", ordinary, inline, v157Open(t, "answer", 3, 2),
			blockLine, v157Close(t, 6, 3), v157Text(t, "spain?", "spain?", 3, 2)),
			[]v157Expected{{[3]int{5, 9, 14}, [][3]int{{4, 1, 6}, {7, 1, 7}, {9, 1, 7}}}})
	})
}

func TestV157CompositeAnswers(t *testing.T) {
	for _, tc := range []struct {
		name, raw, ordinaryRaw, ordinaryValue string
		parts                                 []model.Element
		want                                  []v157Expected
	}{
		{"source spaces", "@answer New \t York", "New York", "New York", []model.Element{
			v157Element(t, "content", "New", "New", 9), v157Element(t, "content", "York", "York", 15)},
			[]v157Expected{{[3]int{5, 9, 19}, [][3]int{{4, 1, 9}}}}},
		{"Unicode values", "@answer ЁЖ\u00a0\u2003ЮГ", "ёж юг", "ёж юг", []model.Element{
			v157Element(t, "content", "ЁЖ\u00a0", "ЁЖ\u00a0", 9), v157Element(t, "content", "\u2003ЮГ", "\u2003ЮГ", 12)},
			[]v157Expected{{[3]int{5, 9, 15}, [][3]int{{4, 1, 6}}}}},
		{"no repeated unescape", "@answer \\\\\\{", "\\\\\\{", "\\{", []model.Element{
			v157Element(t, "content", "\\\\", "\\", 9), v157Element(t, "content", "\\{", "{", 11)},
			[]v157Expected{{[3]int{5, 9, 13}, [][3]int{{4, 1, 5}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := append([]model.Element{v157Element(t, "tag", "@answer", "answer", 1)}, tc.parts...)
			answer := v157Line(t, tc.raw, "tag", 3, 2, parts...)
			v157Run(t, v157Block(t, "multichoice", v157Text(t, tc.ordinaryRaw, tc.ordinaryValue, 3, 2), answer), tc.want)
		})
	}
	for _, name := range []string{"choice", "multichoice"} {
		t.Run(name+" block answer with source gap", func(t *testing.T) {
			answer := v157Line(t, "New \t York", "content", 5, 3,
				v157Element(t, "content", "New", "New", 1), v157Element(t, "content", "", "", 2),
				v157Element(t, "content", "York", "York", 7))
			v157Run(t, v157Block(t, name, v157Text(t, "New York", "New York", 3, 2),
				v157Open(t, "answer", 3, 2), answer, v157Close(t, 5, 3)),
				[]v157Expected{{[3]int{6, 1, 11}, [][3]int{{4, 1, 9}}}})
		})
	}
}

func TestV157AmbiguousCompositeContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		parts     []model.Element
	}{
		{"uncovered prefix", "!Spain", []model.Element{v157Element(t, "content", "Spain", "Spain", 2)}},
		{"uncovered suffix", "Spain!", []model.Element{v157Element(t, "content", "Spain", "Spain", 1)}},
		{"uncovered punctuation between fragments", "Sp!ain", []model.Element{
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "ain", "ain", 4)}},
		{"uncovered letter between fragments", "SpXain", []model.Element{
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "ain", "ain", 4)}},
		{"uncovered escape between fragments", "Sp\\{ain", []model.Element{
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "ain", "ain", 5)}},
		{"empty element does not cover significant gap", "Sp!ain", []model.Element{
			v157Element(t, "content", "Sp", "Sp", 1), v157Element(t, "content", "", "", 4), v157Element(t, "content", "ain", "ain", 4)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ordinary := v157Line(t, tc.raw, "content", 3, 2, tc.parts...)
			v157Run(t, v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2), ordinary), nil)
			parts := []model.Element{v157Element(t, "tag", "@answer", "answer", 1)}
			for _, element := range tc.parts {
				element.Start, element.End = element.Start+8, element.End+8
				parts = append(parts, element)
			}
			answer := v157Line(t, "@answer "+tc.raw, "tag", 3, 2, parts...)
			v157Run(t, v157Block(t, "choice", v157Text(t, "Spain", "Spain", 3, 2), answer), nil)
		})
	}
	t.Run("unparsed fragment between composite contents", func(t *testing.T) {
		unparsed := v157Element(t, "unparsed", "{", "", 3)
		unparsed.Value = nil
		ordinary := v157Line(t, "Sp{ain", "content", 3, 2, v157Element(t, "content", "Sp", "Sp", 1), unparsed,
			v157Element(t, "content", "ain", "ain", 4))
		result := v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2), ordinary)
		v157ParserError(t, result, 5, 1)
		v157Run(t, result, nil)
	})
}

func TestV157AmbiguousValues(t *testing.T) {
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run(string(kind), func(t *testing.T) {
			unparsed := v157Element(t, "unparsed", "{", "", 15)
			unparsed.Value = nil
			answer := v157Line(t, "@answer Spain {", kind, 3, 2, v157Element(t, "tag", "@answer", "answer", 1),
				v157Element(t, "content", "Spain", "Spain", 9), unparsed)
			result := v157Block(t, "choice", answer, v157Text(t, "Spain", "Spain", 3, 2))
			v157ParserError(t, result, 4, 2)
			v157Run(t, result, nil)
		})
	}
	t.Run("ordinary partly unparsed", func(t *testing.T) {
		unparsed := v157Element(t, "unparsed", "{", "", 7)
		unparsed.Value = nil
		ordinary := v157Line(t, "Spain {", "content", 3, 2, v157Element(t, "content", "Spain", "Spain", 1), unparsed)
		result := v157Block(t, "choice", v157Inline(t, "answer", "Spain", "Spain", 3, 2), ordinary)
		v157ParserError(t, result, 5, 1)
		v157Run(t, result, nil)
	})
}

// v157Element сохраняет явные исходное и семантическое значения элемента.
// Вычисление длины относится только к входу; ожидания заданы числами отдельно.
func v157Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v157Line задаёт модель строки без парсинга или восстановления её структуры.
func v157Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v157Open задаёт блок с явно указанным родителем и глубиной.
func v157Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v157Line(t, "@"+name+" {", "block-start", parent, depth,
		v157Element(t, "tag", "@"+name, name, 1), v157Element(t, "block-open", "{", "{", len(name)+3))
}

// v157Close задаёт закрытие указанного блока.
func v157Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v157Line(t, "}", "block-end", parent, depth, v157Element(t, "block-close", "}", "}", 1))
}

// v157Inline задаёт однострочную конструкцию с явным семантическим содержимым.
func v157Inline(t *testing.T, name, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v157Line(t, "@"+name+" "+raw, "tag", parent, depth,
		v157Element(t, "tag", "@"+name, name, 1), v157Element(t, "content", raw, value, len(name)+3))
}

// v157Text задаёт текстовую строку, не нормализуя переданное значение.
func v157Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v157Line(t, raw, "content", parent, depth, v157Element(t, "content", raw, value, 1))
}

// v157Block помещает заданное тело в блок на строке 3, без смены родителей.
func v157Block(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v157Open(t, name, 2, 1)}, body...)
	return v157Document(t, append(lines, v157Close(t, 3, 2))...)
}

// v157Document добавляет обязательные версию и заголовок перед телом.
func v157Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v157Line(t, "@dsl-version 1.2", "tag", 0, 0, v157Element(t, "tag", "@dsl-version", "dsl-version", 1), v157Element(t, "version", "1.2", "1.2", 14)),
		v157Line(t, "# Lesson", "heading", 0, 0, v157Element(t, "heading-level", "#", "1", 1), v157Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
}

// v157ParserError отмечает заданную ошибку модели; index=-1 означает scope line.
func v157ParserError(t *testing.T, result *model.Result, number, index int) {
	t.Helper()
	line := &result.Lines[number-1]
	result.Document.HasErrors, line.HasErrors = true, true
	id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
	start, end, scope := 1, utf8.RuneCountInString(line.Raw)+1, diagnostics.ScopeLine
	if index >= 0 {
		element := &line.Elements[index]
		element.ErrorIDs = append(element.ErrorIDs, id)
		start, end, scope = element.Start, element.End, diagnostics.ScopeElement
	}
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error",
		DiagnosticScope: scope, Message: "Ошибка исходной конструкции", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
}

// v157Registry использует полный штатный каталог без альтернативной регистрации.
func v157Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v157Snapshot сохраняет независимое представление для проверки неизменности.
func v157Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v157Run проверяет только V157 текущего запуска полного конвейера,
// исходные диапазоны и метаданные, отсутствие лишних результатов и сохранность входа.
func v157Run(t *testing.T, result *model.Result, want []v157Expected) {
	t.Helper()
	before := v157Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v157Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v157Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v157-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V157" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V157 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]}, End: model.Position{Line: expected.main[0], Column: expected.main[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Одинаковый текст варианта указан и как правильный, и как неправильный.\nконкретизация: Проверьте подписи вариантов и назначение правильных вариантов." {
			t.Fatalf("occurrence %d = %+v; location = %+v, want %+v and exact metadata", i, entry, entry.Location, location)
		}
		for j, position := range expected.related {
			related := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("related %d = %+v, want %+v", j, entry.RelatedLocations[j], related)
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.main[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line-scoped V157 linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v157Snapshot(t, result)), &restored); err != nil {
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
	if v157Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

package checkers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v181Expected задаёт независимые ожидания и исходные диапазоны [start,end).
type v181Expected struct {
	line, start, end int
	limit            string
	related          [][3]int
}

func TestV181ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, first, source string
		want                []v181Expected
	}{
		{"reference", "2", "@ordering {\nA\n@answer 2\nB\n@answer 1\n}", nil},
		{"example", "3", "@ordering {\nA\n@answer 3\nB\n@answer 2\n}", []v181Expected{{5, 9, 10, "2", nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			second := "1"
			if tc.name == "example" {
				second = "2"
			}
			result := v181TestBase(t, tc.first, second)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// Добавлены только обязательные версия и основной заголовок по DSL 1.30.
			v181TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV181Numbers(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  []v181Expected
	}{
		{"1", nil}, {"2", nil}, {"3", []v181Expected{{5, 9, 10, "2", nil}}},
		{"1.", nil}, {"(2)", nil}, {"-2", nil}, {"\u00a02\u00a0", nil},
		{"3.", []v181Expected{{5, 9, 11, "2", nil}}},
		{"(3)", []v181Expected{{5, 9, 12, "2", nil}}},
		{"-3", []v181Expected{{5, 9, 11, "2", nil}}},
		{"3.0", []v181Expected{{5, 9, 12, "2", nil}}},
		{`\3`, []v181Expected{{5, 9, 11, "2", nil}}},
		{"\u00a03", []v181Expected{{5, 9, 11, "2", nil}}},
		{"3\u00a0", []v181Expected{{5, 9, 11, "2", nil}}},
		{"\u00a03\u00a0", []v181Expected{{5, 9, 12, "2", nil}}},
		{"999999999999999999999999999999", []v181Expected{{5, 9, 39, "2", nil}}},
		{"0", nil}, {"03", nil}, {"+3", nil}, {"3 4", nil}, {"3\t4", nil},
		{"٣", nil}, {"３", nil}, {"3e2", nil}, {"", nil},
	} {
		t.Run(strconv.Quote(tc.value), func(t *testing.T) {
			result := v181TestBase(t, tc.value, "2")
			if tc.value == "" {
				v181TestError(t, result, 5, -1, "Отсутствует значение ответа.")
			}
			v181TestRun(t, result, "current", tc.want)
		})
	}
	for _, tc := range []struct {
		count int
		value string
		want  []v181Expected
	}{
		{9, "10", []v181Expected{{13, 9, 11, "9", nil}}},
		{10, "11", []v181Expected{{14, 9, 11, "10", nil}}},
		{10, "10", nil}, {10, "9", nil},
		{3, "3.", nil}, {3, "(3)", nil}, {3, "-3", nil}, {3, "\u00a03\u00a0", nil},
		{29, "3.0", []v181Expected{{33, 9, 12, "29", nil}}}, {30, "3.0", nil},
	} {
		t.Run(fmt.Sprintf("N=%d answer=%s", tc.count, tc.value), func(t *testing.T) {
			var rows []model.Line
			for i := 0; i < tc.count; i++ {
				rows = append(rows, v181TestText(t, "A", "A", 3, 2))
			}
			rows = append(rows, v181TestTag(t, "answer", "content", tc.value, 3, 2))
			v181TestRun(t, v181TestOrdering(t, rows...), "current", tc.want)
		})
	}
	t.Run("source indentation", func(t *testing.T) {
		result := v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2),
			v181TestLine(t, " \t@answer (3) \t", "tag", 3, 2, v181TestElement(t, "tag", "@answer", "answer", 3, 10), v181TestElement(t, "content", "(3)", "(3)", 11, 14)),
			v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2))
		v181TestRun(t, result, "current", []v181Expected{{5, 11, 14, "2", nil}})
	})
}

func TestV181ItemsAndAssociation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rows      []model.Line
		want      []v181Expected
		errorLine int
	}{
		{"one element", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2)}, []v181Expected{{5, 9, 10, "1", nil}}, 0},
		{"duplicate elements", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2)}, []v181Expected{{5, 9, 10, "2", nil}}, 0},
		{"duplicate elements accept second position", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "(2)", 3, 2), v181TestText(t, "A", "A", 3, 2)}, nil, 0},
		{"partial answers", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{5, 9, 10, "2", nil}}, 0},
		{"partial normalized answer", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "(3)", 3, 2), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{5, 9, 12, "2", nil}}, 0},
		{"partial valid normalized answer", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "(2)", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 0},
		{"no answers", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 0},
		{"no elements", nil, nil, 3},
		{"orphan", []model.Line{v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "A", "A", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 4},
		{"repeated declaration", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "1", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 6},
		{"blank", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestBlank(t, "", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{6, 9, 10, "2", nil}}, 0},
		{"ASCII blank", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestBlank(t, " \t ", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{6, 9, 10, "2", nil}}, 0},
		{"hint interrupts", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "hint", "content", "Note", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 5},
		{"editor interrupts", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "editor", "content", "Note", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 5},
		{"block editor interrupts", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestOpen(t, "editor", 3, 2), v181TestText(t, "Note", "Note", 5, 3), v181TestClose(t, 5, 3), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 5},
		{"foreign block interrupts", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestOpen(t, "note", 3, 2), v181TestText(t, "Note", "Note", 5, 3), v181TestClose(t, 5, 3), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2)}, nil, 5},
		{"hint body not an item", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestOpen(t, "hint", 3, 2), v181TestText(t, "Note", "Note", 6, 3), v181TestClose(t, 6, 3), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{5, 9, 10, "2", nil}}, 0},
		{"unknown item", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestInvalid(t, "@unknown", 3, 2)}, nil, 0},
		{"unknown inside hint", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestOpen(t, "hint", 3, 2), v181TestInvalid(t, "@unknown", 6, 3), v181TestClose(t, 6, 3), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{5, 9, 10, "2", nil}}, 0},
		{"NBSP item counted", []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestText(t, "\u00a0", "\u00a0", 3, 2), v181TestTag(t, "answer", "content", "4", 3, 2), v181TestText(t, "B", "B", 3, 2)}, []v181Expected{{6, 9, 10, "3", nil}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v181TestOrdering(t, tc.rows...)
			if tc.errorLine != 0 {
				v181TestError(t, result, tc.errorLine, -1, "Нарушены правила содержимого или размещения тега.")
			}
			v181TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV181BlockAnswers(t *testing.T) {
	huge := strings.Repeat("9", 256)
	for _, tc := range []struct {
		name   string
		values []model.Line
		want   []v181Expected
	}{
		{"groups", []model.Line{v181TestText(t, "3", "3", 5, 3), v181TestText(t, "3", "3", 5, 3), v181TestText(t, "4", "4", 5, 3)}, []v181Expected{{6, 1, 2, "2", [][3]int{{7, 1, 2}}}, {8, 1, 2, "2", nil}}},
		{"normalized groups", []model.Line{v181TestText(t, "3.", "3.", 5, 3), v181TestText(t, "(3)", "(3)", 5, 3), v181TestText(t, "-3", "-3", 5, 3), v181TestText(t, "\u00a03\u00a0", "\u00a03\u00a0", 5, 3), v181TestText(t, "3", "3", 5, 3)}, []v181Expected{{6, 1, 3, "2", [][3]int{{7, 1, 4}, {8, 1, 3}, {9, 1, 4}, {10, 1, 2}}}}},
		{"normalized interleaved groups", []model.Line{v181TestText(t, "3.0", "3.0", 5, 3), v181TestText(t, "3.", "3.", 5, 3), v181TestText(t, "(30)", "(30)", 5, 3), v181TestText(t, "30", "30", 5, 3), v181TestText(t, "(3)", "(3)", 5, 3)}, []v181Expected{{6, 1, 4, "2", [][3]int{{8, 1, 5}, {9, 1, 3}}}, {7, 1, 3, "2", [][3]int{{10, 1, 4}}}}},
		{"huge normalized groups", []model.Line{v181TestText(t, huge, huge, 5, 3), v181TestText(t, "("+huge+")", "("+huge+")", 5, 3), v181TestText(t, huge+"0", huge+"0", 5, 3)}, []v181Expected{{6, 1, 257, "2", [][3]int{{7, 1, 259}}}, {8, 1, 258, "2", nil}}},
		{"blank before groups", []model.Line{v181TestBlank(t, "", 5, 3), v181TestText(t, "3", "3", 5, 3), v181TestText(t, "3", "3", 5, 3), v181TestText(t, "4", "4", 5, 3)}, []v181Expected{{7, 1, 2, "2", [][3]int{{8, 1, 2}}}, {9, 1, 2, "2", nil}}},
		{"valid alternatives", []model.Line{v181TestText(t, "1", "1", 5, 3), v181TestText(t, "2", "2", 5, 3)}, nil},
		{"valid normalized alternatives", []model.Line{v181TestText(t, "1.", "1.", 5, 3), v181TestText(t, "(2)", "(2)", 5, 3), v181TestText(t, "-2", "-2", 5, 3), v181TestText(t, "\u00a02\u00a0", "\u00a02\u00a0", 5, 3)}, nil},
		{"invalid syntax not merged", []model.Line{v181TestText(t, "3", "3", 5, 3), v181TestText(t, "03", "03", 5, 3), v181TestText(t, "3", "3", 5, 3)}, []v181Expected{{6, 1, 2, "2", [][3]int{{8, 1, 2}}}}},
		{"unrecognized forms not checked or merged", []model.Line{v181TestText(t, "3.", "3.", 5, 3), v181TestText(t, "+3", "+3", 5, 3), v181TestText(t, "03", "03", 5, 3), v181TestText(t, "(3)", "(3)", 5, 3)}, []v181Expected{{6, 1, 3, "2", [][3]int{{9, 1, 4}}}}},
		{"interleaved groups", []model.Line{v181TestText(t, "3", "3", 5, 3), v181TestText(t, "4", "4", 5, 3), v181TestText(t, "3", "3", 5, 3), v181TestText(t, "4", "4", 5, 3)}, []v181Expected{{6, 1, 2, "2", [][3]int{{8, 1, 2}}}, {7, 1, 2, "2", [][3]int{{9, 1, 2}}}}},
		{"unknown value", []model.Line{v181TestInvalid(t, "@unknown", 5, 3)}, nil},
		{"known value beside unknown", []model.Line{v181TestText(t, "3", "3", 5, 3), v181TestInvalid(t, "@unknown", 5, 3)}, []v181Expected{{6, 1, 2, "2", nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestOpen(t, "answer", 3, 2)}
			rows = append(rows, tc.values...)
			rows = append(rows, v181TestClose(t, 5, 3), v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", "1", 3, 2))
			v181TestRun(t, v181TestOrdering(t, rows...), "current", tc.want)
		})
	}
	t.Run("separate answers", func(t *testing.T) {
		v181TestRun(t, v181TestBase(t, "3", "3"), "current", []v181Expected{{5, 9, 10, "2", nil}, {7, 9, 10, "2", nil}})
	})
	t.Run("separate normalized answers", func(t *testing.T) {
		v181TestRun(t, v181TestBase(t, "3.", "(3)"), "current", []v181Expected{{5, 9, 11, "2", nil}, {7, 9, 12, "2", nil}})
	})
	t.Run("deeper value excluded", func(t *testing.T) {
		result := v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2), v181TestOpen(t, "answer", 3, 2), v181TestOpen(t, "note", 5, 3), v181TestText(t, "3", "3", 6, 4),
			v181TestClose(t, 6, 4), v181TestClose(t, 5, 3), v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2))
		v181TestError(t, result, 6, -1, "Вложенные теги запрещены в answer.")
		v181TestRun(t, result, "current", nil)
	})
	t.Run("unparsed inline value", func(t *testing.T) {
		unparsed := v181TestElement(t, "unparsed", "???", "", 9, 12)
		unparsed.Value = nil
		result := v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2), v181TestLine(t, "@answer ???", "invalid", 3, 2, v181TestElement(t, "tag", "@answer", "answer", 1, 8), unparsed), v181TestText(t, "B", "B", 3, 2))
		v181TestRun(t, result, "current", nil)
	})
}

func TestV181Separators(t *testing.T) {
	for _, tc := range []struct {
		name       string
		separators []model.Line
	}{
		{"single", []model.Line{v181TestLine(t, "---", "separator", 3, 2)}},
		{"multiple", []model.Line{v181TestLine(t, "---", "separator", 3, 2), v181TestLine(t, "---", "separator", 3, 2)}},
		{"ASCII edges", []model.Line{v181TestLine(t, " \t--- \t", "separator", 3, 2)}},
		{"unparsed separator", []model.Line{v181TestInvalid(t, "---", 3, 2)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.Line{v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2)}
			rows = append(rows, tc.separators...)
			rows = append(rows, v181TestText(t, "B", "B", 3, 2))
			v181TestRun(t, v181TestOrdering(t, rows...), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\---`, "---"}, {"----", "----"}, {"--", "--"}, {"- - -", "- - -"}, {"\u00a0---", "\u00a0---"},
	} {
		t.Run("ordinary item/"+strconv.Quote(tc.raw), func(t *testing.T) {
			result := v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, tc.raw, tc.value, 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2))
			v181TestRun(t, result, "current", []v181Expected{{5, 9, 10, "2", nil}})
		})
	}
	t.Run("nested separator", func(t *testing.T) {
		result := v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestOpen(t, "hint", 3, 2),
			v181TestText(t, "---", "---", 6, 3), v181TestClose(t, 6, 3), v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2))
		v181TestRun(t, result, "current", []v181Expected{{5, 9, 10, "2", nil}})
	})
}

func TestV181Contexts(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		prefix, suffix         []model.Line
		opening, parent, depth int
		want                   []v181Expected
	}{
		{"task", []model.Line{v181TestTag(t, "task", "identifier", "first", 2, 1)}, []model.Line{v181TestTag(t, "endtask", "", "", 2, 1)}, 4, 3, 2, []v181Expected{{6, 9, 10, "2", nil}}},
		{"step", []model.Line{v181TestTag(t, "task", "identifier", "first", 2, 1), v181TestTag(t, "step", "title", "First", 3, 2)}, []model.Line{v181TestTag(t, "endtask", "", "", 2, 1)}, 5, 4, 3, []v181Expected{{7, 9, 10, "2", nil}}},
		{"variant", []model.Line{v181TestOpen(t, "variants", 2, 1), v181TestTag(t, "variant", "name", "A", 3, 2)},
			[]model.Line{v181TestTag(t, "variant", "name", "B", 3, 2), v181TestText(t, "Читайте.", "Читайте.", 11, 3), v181TestClose(t, 3, 2)}, 5, 4, 3, []v181Expected{{7, 9, 10, "2", nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.prefix, v181TestOpen(t, "ordering", tc.parent, tc.depth), v181TestText(t, "A", "A", tc.opening, tc.depth+1), v181TestTag(t, "answer", "content", "3", tc.opening, tc.depth+1),
				v181TestText(t, "B", "B", tc.opening, tc.depth+1), v181TestTag(t, "answer", "content", "2", tc.opening, tc.depth+1), v181TestClose(t, tc.opening, tc.depth+1))
			body = append(body, tc.suffix...)
			v181TestRun(t, v181TestDocument(t, body...), "current", tc.want)
		})
	}
	t.Run("independent blocks", func(t *testing.T) {
		var body []model.Line
		for _, opening := range []int{3, 9} {
			body = append(body, v181TestOpen(t, "ordering", 2, 1), v181TestText(t, "A", "A", opening, 2), v181TestTag(t, "answer", "content", "3", opening, 2),
				v181TestText(t, "B", "B", opening, 2), v181TestTag(t, "answer", "content", "2", opening, 2), v181TestClose(t, opening, 2))
		}
		v181TestRun(t, v181TestDocument(t, body...), "current", []v181Expected{{5, 9, 10, "2", nil}, {11, 9, 10, "2", nil}})
	})
	t.Run("matching excluded", func(t *testing.T) {
		result := v181TestDocument(t, v181TestOpen(t, "matching", 2, 1), v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", "3", 3, 2), v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2), v181TestClose(t, 3, 2))
		v181TestRun(t, result, "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal ordering in "+tag, func(t *testing.T) {
			result := v181TestDocument(t, v181TestOpen(t, tag, 2, 1), v181TestText(t, "@ordering {", "@ordering {", 3, 2), v181TestText(t, "A", "A", 3, 2), v181TestText(t, "@answer 3", "@answer 3", 3, 2),
				v181TestText(t, "B", "B", 3, 2), v181TestText(t, "@answer 2", "@answer 2", 3, 2), v181TestText(t, `\}`, "}", 3, 2), v181TestClose(t, 3, 2))
			v181TestRun(t, result, "current", nil)
		})
	}
	t.Run("mixed case names", func(t *testing.T) {
		result := v181TestBase(t, "3", "2")
		result.Lines[2].Raw = "@OrDeRiNg {"
		result.Lines[2].Elements[0].Raw = "@OrDeRiNg"
		result.Lines[4].Raw = "@AnSwEr 3"
		result.Lines[4].Elements[0].Raw = "@AnSwEr"
		v181TestRun(t, result, "current", []v181Expected{{5, 9, 10, "2", nil}})
	})
}

func TestV181WholeValues(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		parts []model.Element
		want  []v181Expected
	}{
		{"@answer 30", []model.Element{v181TestElement(t, "tag", "@answer", "answer", 1, 8), v181TestElement(t, "content", "3", "3", 9, 10), v181TestElement(t, "content", "0", "0", 10, 11)}, []v181Expected{{5, 9, 11, "2", nil}}},
		{"@answer 3 0", []model.Element{v181TestElement(t, "tag", "@answer", "answer", 1, 8), v181TestElement(t, "content", "3", "3", 9, 10), v181TestElement(t, "content", "0", "0", 11, 12)}, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2), v181TestLine(t, tc.raw, "tag", 3, 2, tc.parts...), v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", "2", 3, 2))
			v181TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV181RegistrationAndHistory(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V181" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Message != "Номер позиции превышает количество элементов в @ordering." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V181 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v181TestBase(t, "3", "2")
			result.Document.DSLVersion = &version
			result.Lines[0] = v181TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v181TestRun(t, result, "current", nil)
		})
	}
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v181TestBase(t, "3", "2")
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v181Expected{{5, 9, 10, "2", nil}}
			v181TestRun(t, result, "first", want)
			v181TestRun(t, result, "second", want)
			v181TestRun(t, v181TestBase(t, "2", "1"), "separate", nil)
		})
	}
}

// v181TestBase задаёт эталонный контекст с ответами на строках 5 и 7.
func v181TestBase(t *testing.T, first, second string) *model.Result {
	t.Helper()
	return v181TestOrdering(t, v181TestText(t, "A", "A", 3, 2), v181TestTag(t, "answer", "content", first, 3, 2), v181TestText(t, "B", "B", 3, 2), v181TestTag(t, "answer", "content", second, 3, 2))
}

// v181TestOrdering помещает строки в ordering, объявленный на строке 3.
func v181TestOrdering(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v181TestOpen(t, "ordering", 2, 1)}, rows...)
	body = append(body, v181TestClose(t, 3, 2))
	return v181TestDocument(t, body...)
}

// v181TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v181TestDocument.
func v181TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v181TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v181TestLine(t, raw, "invalid", parent, depth, element)
}

// v181TestRun запускает штатный конвейер и проверяет только V181 текущего запуска.
func v181TestRun(t *testing.T, result *model.Result, source string, want []v181Expected) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V181" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V181 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "Номер позиции превышает количество элементов в @ordering.\nконкретизация: Допустимый диапазон: 1…" + expected.limit + "."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for i, related := range expected.related {
			location := model.Location{Start: model.Position{Line: related[0], Column: related[1]}, End: model.Position{Line: related[0], Column: related[2]}}
			if entry.RelatedLocations[i] != location {
				t.Fatalf("related %d = %+v, want %+v", i, entry.RelatedLocations[i], location)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("ScopeLine must not create element errorIds")
					}
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
		}

	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original model.Result
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	for index, line := range original.Lines {
		current := result.Lines[index]
		current.HasErrors = line.HasErrors
		current.Elements = append([]model.Element{}, current.Elements...)
		for element := range current.Elements {
			current.Elements[element].ErrorIDs = line.Elements[element].ErrorIDs
		}
		left, _ := json.Marshal(line)
		right, _ := json.Marshal(current)
		if string(left) != string(right) {
			t.Fatalf("source changed on line %d", index+1)
		}
	}
	for index, old := range original.Diagnostics {
		left, _ := json.Marshal(old)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v181TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v181TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
	})
}

// v181TestElement задаёт исходное и семантическое значения без преобразований.
func v181TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v181TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v181TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v181TestTag создаёт однострочное объявление с явно заданным параметром.
func v181TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v181TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v181TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v181TestLine(t, raw, "tag", parent, depth, elements...)
}

// v181TestOpen создаёт объявление блока без параметров.
func v181TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v181TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v181TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v181TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v181TestClose создаёт закрытие с заданным родителем.
func v181TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v181TestLine(t, "}", "block-end", parent, depth, v181TestElement(t, "block-close", "}", "}", 1, 2))
}

// v181TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v181TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v181TestLine(t, raw, "content", parent, depth, v181TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v181TestBlank задаёт обычную пустую строку без текстового элемента.
func v181TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v181TestLine(t, raw, "blank", parent, depth)
}

// v181TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v181TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v181TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v181TestLine(t, "# Lesson", "heading", 0, 0, v181TestElement(t, "heading-level", "#", "1", 1, 2), v181TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		for j, element := range result.Lines[i].Elements {
			if element.ElementType == "unparsed" {
				v181TestError(t, result, i+1, j, "Строка разобрана не полностью.")
			}
		}
	}
	return result

}

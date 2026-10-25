package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v146Expected хранит явные диапазоны: строка, начало, исключаемый конец.
// Связанные диапазоны перечислены в порядке «первый ответ, вопрос».
type v146Expected struct {
	primary [3]int
	related [2][3]int
}

func TestV146ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v146Expected
	}{
		{"reference", "@question Where does Tom live?\n@answer {\nLondon\nParis\n}", []model.Line{
			v146TestTag(t, "question", "content", "Where does Tom live?", 2, 1), v146TestOpen(t, "answer", 2, 1),
			v146TestText(t, "London", "London", 4, 2), v146TestText(t, "Paris", "Paris", 4, 2), v146TestClose(t, 4, 2)}, nil},
		{"example", "@question Where does Tom live?\n@answer London\n@answer Paris", []model.Line{
			v146TestTag(t, "question", "content", "Where does Tom live?", 2, 1),
			v146TestTag(t, "answer", "content", "London", 2, 1), v146TestTag(t, "answer", "content", "Paris", 2, 1)},
			[]v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v146TestRun(t, v146TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV146Sequences(t *testing.T) {
	q := func() model.Line { t.Helper(); return v146TestTag(t, "question", "content", "Where?", 2, 1) }
	a := func() model.Line { t.Helper(); return v146TestTag(t, "answer", "content", "London", 2, 1) }
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v146Expected
	}{
		{"no answer", []model.Line{q()}, nil},
		{"one answer", []model.Line{q(), a()}, nil},
		{"three equal answers", []model.Line{q(), a(), a(), a()}, []v146Expected{
			{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}, {[3]int{6, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
		{"block then inline", []model.Line{q(), v146TestOpen(t, "answer", 2, 1), v146TestText(t, "London", "London", 4, 2),
			v146TestText(t, "Paris", "Paris", 4, 2), v146TestClose(t, 4, 2), a()},
			[]v146Expected{{[3]int{8, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
		{"inline then block", []model.Line{q(), a(), v146TestOpen(t, "answer", 2, 1), v146TestText(t, "London", "London", 5, 2),
			v146TestText(t, "Paris", "Paris", 5, 2), v146TestClose(t, 5, 2)},
			[]v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
		{"two blocks then inline", []model.Line{q(), v146TestOpen(t, "answer", 2, 1), v146TestText(t, "London", "London", 4, 2),
			v146TestClose(t, 4, 2), v146TestOpen(t, "answer", 2, 1), v146TestText(t, "Paris", "Paris", 7, 2), v146TestClose(t, 7, 2), a()},
			[]v146Expected{{[3]int{7, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}, {[3]int{10, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
		{"blank between answers", []model.Line{q(), a(), v146TestBlank(t, "", 2, 1), a()},
			[]v146Expected{{[3]int{6, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
		{"spaces and tabs before first", []model.Line{q(), v146TestBlank(t, " \t", 2, 1), a(), a()},
			[]v146Expected{{[3]int{6, 1, 8}, [2][3]int{{5, 1, 8}, {3, 1, 10}}}}},
		{"inline editor before first", []model.Line{q(), v146TestTag(t, "editor", "content", "Check.", 2, 1), a(), a()},
			[]v146Expected{{[3]int{6, 1, 8}, [2][3]int{{5, 1, 8}, {3, 1, 10}}}}},
		{"editor block between answers", []model.Line{q(), a(), v146TestOpen(t, "editor", 2, 1),
			v146TestText(t, "@answer Literal", "@answer Literal", 5, 2), v146TestClose(t, 5, 2), a()},
			[]v146Expected{{[3]int{8, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
		{"several comments and blanks", []model.Line{q(), v146TestOpen(t, "editor", 2, 1), v146TestBlank(t, "", 4, 2),
			v146TestText(t, "Check.", "Check.", 4, 2), v146TestClose(t, 4, 2), a(), v146TestBlank(t, " \t", 2, 1),
			v146TestTag(t, "editor", "content", "Again.", 2, 1), a()},
			[]v146Expected{{[3]int{11, 1, 8}, [2][3]int{{8, 1, 8}, {3, 1, 10}}}}},
		{"two questions with one answer each", []model.Line{q(), a(), q(), a()}, nil},
		{"two independent violations", []model.Line{q(), a(), a(), q(), a(), a()},
			[]v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}, {[3]int{8, 1, 8}, [2][3]int{{7, 1, 8}, {6, 1, 10}}}}},
		{"Unicode values", []model.Line{q(), v146TestTag(t, "answer", "content", "Лондон", 2, 1), v146TestTag(t, "answer", "content", "東京", 2, 1)},
			[]v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v146TestRun(t, v146TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV146SignificantBarriers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		barrier model.Line
	}{
		{"hint", v146TestTag(t, "hint", "content", "Look.", 2, 1)},
		{"note", v146TestTag(t, "note", "content", "Read.", 2, 1)},
		{"text", v146TestText(t, "Read.", "Read.", 2, 1)},
		{"NBSP", v146TestText(t, "\u00a0", "\u00a0", 2, 1)},
		{"escaped editor", v146TestText(t, `\@editor Comment.`, "@editor Comment.", 2, 1)},
		{"escaped answer", v146TestText(t, `\@answer London`, "@answer London", 2, 1)},
		{"tag mention", v146TestText(t, "See @answer London", "See @answer London", 2, 1)},
	} {
		for _, beforeFirst := range []bool{false, true} {
			t.Run(tc.name+" before first="+strconv.FormatBool(beforeFirst), func(t *testing.T) {
				q := v146TestTag(t, "question", "content", "Where?", 2, 1)
				a := v146TestTag(t, "answer", "content", "London", 2, 1)
				body := []model.Line{q, a, tc.barrier, a}
				if beforeFirst {
					body = []model.Line{q, tc.barrier, a, a}
				}
				v146TestRun(t, v146TestDocument(t, body...), "current", nil)
			})
		}
	}
	for _, name := range []string{"hint", "note"} {
		t.Run(name+" block", func(t *testing.T) {
			v146TestRun(t, v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1),
				v146TestTag(t, "answer", "content", "London", 2, 1), v146TestOpen(t, name, 2, 1),
				v146TestText(t, "Look.", "Look.", 5, 2), v146TestClose(t, 5, 2),
				v146TestTag(t, "answer", "content", "Paris", 2, 1)), "current", nil)
		})
	}
}

func TestV146StructuralContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v146Expected
	}{
		{"task", []model.Line{v146TestTag(t, "task", "identifier", "first", 2, 1), v146TestTag(t, "question", "content", "Where?", 3, 2),
			v146TestTag(t, "answer", "content", "London", 3, 2), v146TestTag(t, "answer", "content", "Paris", 3, 2), v146TestTag(t, "endtask", "", "", 2, 1)},
			[]v146Expected{{[3]int{6, 1, 8}, [2][3]int{{5, 1, 8}, {4, 1, 10}}}}},
		{"internal step", []model.Line{v146TestTag(t, "task", "identifier", "first", 2, 1), v146TestTag(t, "step", "title", "First", 3, 2),
			v146TestTag(t, "question", "content", "Where?", 4, 3), v146TestTag(t, "answer", "content", "London", 4, 3),
			v146TestTag(t, "answer", "content", "Paris", 4, 3), v146TestTag(t, "step", "title", "Second", 3, 2),
			v146TestText(t, "Read.", "Read.", 8, 3), v146TestTag(t, "endtask", "", "", 2, 1)},
			[]v146Expected{{[3]int{7, 1, 8}, [2][3]int{{6, 1, 8}, {5, 1, 10}}}}},
		{"branch", []model.Line{v146TestOpen(t, "variants", 2, 1), v146TestTag(t, "variant", "name", "A", 3, 2),
			v146TestTag(t, "question", "content", "Where?", 4, 3), v146TestTag(t, "answer", "content", "London", 4, 3),
			v146TestTag(t, "answer", "content", "Paris", 4, 3), v146TestTag(t, "variant", "name", "B", 3, 2),
			v146TestText(t, "Read.", "Read.", 8, 3), v146TestClose(t, 3, 2)},
			[]v146Expected{{[3]int{7, 1, 8}, [2][3]int{{6, 1, 8}, {5, 1, 10}}}}},
		{"shared variants parent", []model.Line{v146TestOpen(t, "variants", 2, 1), v146TestTag(t, "variant", "name", "A", 3, 2),
			v146TestTag(t, "question", "content", "Where?", 3, 2), v146TestTag(t, "answer", "content", "London", 3, 2),
			v146TestTag(t, "answer", "content", "Paris", 3, 2), v146TestTag(t, "variant", "name", "B", 3, 2),
			v146TestText(t, "Read.", "Read.", 3, 2), v146TestClose(t, 3, 2)},
			[]v146Expected{{[3]int{7, 1, 8}, [2][3]int{{6, 1, 8}, {5, 1, 10}}}}},
		{"branch boundary with shared parent", []model.Line{v146TestOpen(t, "variants", 2, 1), v146TestTag(t, "variant", "name", "A", 3, 2),
			v146TestTag(t, "question", "content", "Where?", 3, 2), v146TestTag(t, "answer", "content", "London", 3, 2),
			v146TestTag(t, "variant", "name", "B", 3, 2), v146TestTag(t, "answer", "content", "Paris", 3, 2), v146TestClose(t, 3, 2)}, nil},
		{"closing variants", []model.Line{v146TestOpen(t, "variants", 2, 1), v146TestTag(t, "variant", "name", "A", 3, 2),
			v146TestText(t, "Read.", "Read.", 4, 3), v146TestTag(t, "variant", "name", "B", 3, 2),
			v146TestTag(t, "question", "content", "Where?", 6, 3), v146TestTag(t, "answer", "content", "London", 6, 3),
			v146TestClose(t, 3, 2), v146TestTag(t, "answer", "content", "Paris", 2, 1)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v146TestRun(t, v146TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, name := range []string{"task", "step", "endtask", "header", "newpage"} {
		t.Run("boundary "+name, func(t *testing.T) {
			boundary := v146TestTag(t, name, "", "", 2, 1)
			parent, depth := 2, 1
			switch name {
			case "task":
				boundary = v146TestTag(t, name, "identifier", "second", 2, 1)
				parent, depth = 7, 2
			case "step":
				boundary = v146TestTag(t, name, "title", "Second", 3, 2)
				parent, depth = 7, 3
			case "header":
				boundary = v146TestTag(t, name, "title", "Next", 2, 1)
				parent, depth = 7, 2
			case "newpage":
				parent, depth = 7, 2
			}
			result := v146TestDocument(t, v146TestTag(t, "task", "identifier", "first", 2, 1), v146TestTag(t, "step", "title", "First", 3, 2),
				v146TestTag(t, "question", "content", "Where?", 4, 3), v146TestTag(t, "answer", "content", "London", 4, 3), boundary,
				v146TestTag(t, "answer", "content", "Paris", parent, depth))
			v146TestRun(t, result, "current", nil)
		})
	}
	t.Run("heading boundary", func(t *testing.T) {
		v146TestRun(t, v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestTag(t, "answer", "content", "London", 2, 1),
			v146TestLine(t, "### Next", "heading", 0, 0, v146TestElement(t, "heading-level", "###", "3", 1, 4), v146TestElement(t, "title", "Next", "Next", 5, 9)),
			v146TestTag(t, "answer", "content", "Paris", 5, 1)), "current", nil)
	})
}

func TestV146ExcludedContainers(t *testing.T) {
	for _, name := range []string{"choice", "multichoice", "matching", "ordering", "instr"} {
		t.Run(name, func(t *testing.T) {
			body := []model.Line{v146TestOpen(t, name, 2, 1), v146TestTag(t, "question", "content", "Where?", 3, 2),
				v146TestTag(t, "answer", "content", "London", 3, 2), v146TestTag(t, "answer", "content", "Paris", 3, 2)}
			if name == "matching" || name == "ordering" {
				// Полные списки и разделитель по 4.7.5–4.7.7 и 4.8.10–4.8.12;
				// ошибочный вложенный question остаётся отдельной проверяемой ситуацией.
				body = append(body, v146TestText(t, "Tom", "Tom", 3, 2), v146TestTag(t, "answer", "content", "London", 3, 2),
					v146TestText(t, "Ann", "Ann", 3, 2), v146TestTag(t, "answer", "content", "Paris", 3, 2),
					v146TestLine(t, "---", "separator", 3, 2), v146TestText(t, "London", "London", 3, 2), v146TestText(t, "Paris", "Paris", 3, 2))
			}
			body = append(body, v146TestClose(t, 3, 2))
			result := v146TestDocument(t, body...)
			v146TestError(t, result, 4, -1, "Вопрос внутри этого контейнера запрещён.")
			v146TestRun(t, result, "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			v146TestRun(t, v146TestDocument(t, v146TestOpen(t, name, 2, 1), v146TestText(t, "@question Where?", "@question Where?", 3, 2),
				v146TestText(t, "@answer London", "@answer London", 3, 2), v146TestText(t, "@answer Paris", "@answer Paris", 3, 2), v146TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("before first variant", func(t *testing.T) {
		result := v146TestDocument(t, v146TestOpen(t, "variants", 2, 1), v146TestTag(t, "question", "content", "Where?", 3, 2),
			v146TestTag(t, "answer", "content", "London", 3, 2), v146TestTag(t, "answer", "content", "Paris", 3, 2),
			v146TestTag(t, "variant", "name", "A", 3, 2), v146TestText(t, "Read.", "Read.", 7, 3),
			v146TestTag(t, "variant", "name", "B", 3, 2), v146TestText(t, "Read.", "Read.", 9, 3), v146TestClose(t, 3, 2))
		v146TestError(t, result, 4, -1, "Материал до первой ветви запрещён.")
		v146TestRun(t, result, "current", nil)
	})
	t.Run("task step inside branch with editor", func(t *testing.T) {
		result := v146TestDocument(t, v146TestOpen(t, "variants", 2, 1), v146TestTag(t, "variant", "name", "A", 3, 2),
			v146TestTag(t, "task", "identifier", "first", 4, 3), v146TestTag(t, "step", "title", "First", 5, 4),
			v146TestTag(t, "question", "content", "Where?", 6, 5), v146TestTag(t, "answer", "content", "London", 6, 5),
			v146TestOpen(t, "editor", 6, 5), v146TestText(t, "Check.", "Check.", 9, 6), v146TestClose(t, 9, 6),
			v146TestTag(t, "answer", "content", "Paris", 6, 5), v146TestTag(t, "step", "title", "Second", 5, 4),
			v146TestText(t, "Read.", "Read.", 13, 5), v146TestTag(t, "endtask", "", "", 4, 3),
			v146TestTag(t, "variant", "name", "B", 3, 2), v146TestText(t, "Read.", "Read.", 16, 3), v146TestClose(t, 3, 2))
		v146TestRun(t, result, "current", []v146Expected{{[3]int{12, 1, 8}, [2][3]int{{8, 1, 8}, {7, 1, 10}}}})
	})
}

func TestV146BlockBoundariesAndErrors(t *testing.T) {
	t.Run("own closing after nested illegal block", func(t *testing.T) {
		result := v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestOpen(t, "answer", 2, 1),
			v146TestOpen(t, "note", 4, 2), v146TestText(t, "Read.", "Read.", 5, 3), v146TestClose(t, 5, 3),
			v146TestText(t, "London", "London", 4, 2), v146TestClose(t, 4, 2), v146TestTag(t, "answer", "content", "Paris", 2, 1))
		v146TestError(t, result, 5, -1, "Вложенный блок в answer запрещён.")
		v146TestRun(t, result, "current", []v146Expected{{[3]int{10, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}})
	})
	t.Run("escaped answer values in block", func(t *testing.T) {
		v146TestRun(t, v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestOpen(t, "answer", 2, 1),
			v146TestText(t, `\@answer London`, "@answer London", 4, 2), v146TestText(t, `\@answer Paris`, "@answer Paris", 4, 2),
			v146TestClose(t, 4, 2)), "current", nil)
	})
	t.Run("editor after block answer", func(t *testing.T) {
		v146TestRun(t, v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestOpen(t, "answer", 2, 1),
			v146TestText(t, "London", "London", 4, 2), v146TestClose(t, 4, 2), v146TestTag(t, "editor", "content", "Check.", 2, 1),
			v146TestTag(t, "answer", "content", "Paris", 2, 1)), "current", []v146Expected{{[3]int{8, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}})
	})
	t.Run("missing answer value still counts declaration", func(t *testing.T) {
		result := v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestTag(t, "answer", "", "", 2, 1),
			v146TestTag(t, "answer", "content", "London", 2, 1))
		v146TestError(t, result, 4, -1, "Значение ответа отсутствует.")
		v146TestRun(t, result, "current", []v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}})
	})
	for _, second := range []bool{false, true} {
		t.Run("unclosed answer second="+strconv.FormatBool(second), func(t *testing.T) {
			body := []model.Line{v146TestTag(t, "question", "content", "Where?", 2, 1)}
			opening := 4
			var want []v146Expected
			if second {
				body = append(body, v146TestTag(t, "answer", "content", "London", 2, 1))
				opening = 5
				want = []v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}
			}
			body = append(body, v146TestOpen(t, "answer", 2, 1), v146TestText(t, "Paris", "Paris", opening, 2))
			result := v146TestDocument(t, body...)
			v146TestError(t, result, opening, -1, "Блок не закрыт до конца файла.")
			v146TestRun(t, result, "current", want)
		})
	}
	t.Run("recognized invalid answer", func(t *testing.T) {
		unparsed := v146TestElement(t, "unparsed", "}", "", 16, 17)
		unparsed.Value = nil
		result := v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestTag(t, "answer", "content", "Paris", 2, 1),
			v146TestLine(t, "@answer London }", "invalid", 2, 1, v146TestElement(t, "tag", "@answer", "answer", 1, 8),
				v146TestElement(t, "content", "London", "London", 9, 15), unparsed))
		v146TestError(t, result, 5, 2, "Лишняя закрывающая скобка.")
		v146TestRun(t, result, "current", []v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}})
	})
}

func TestV146InvalidEditors(t *testing.T) {
	for _, block := range []bool{false, true} {
		t.Run("empty editor block="+strconv.FormatBool(block), func(t *testing.T) {
			body := []model.Line{v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestTag(t, "answer", "content", "London", 2, 1),
				v146TestTag(t, "editor", "", "", 2, 1)}
			if block {
				body[2] = v146TestOpen(t, "editor", 2, 1)
				body = append(body, v146TestBlank(t, " \t", 5, 2), v146TestClose(t, 5, 2))
			}
			body = append(body, v146TestTag(t, "answer", "content", "Paris", 2, 1))
			result := v146TestDocument(t, body...)
			v146TestError(t, result, 5, -1, "Пустой editor запрещён.")
			v146TestRun(t, result, "current", nil)
		})
	}
	t.Run("unparsed editor", func(t *testing.T) {
		unparsed := v146TestElement(t, "unparsed", "}", "", 16, 17)
		unparsed.Value = nil
		result := v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestTag(t, "answer", "content", "London", 2, 1),
			v146TestLine(t, "@editor Check. }", "invalid", 2, 1, v146TestElement(t, "tag", "@editor", "editor", 1, 8),
				v146TestElement(t, "content", "Check.", "Check.", 9, 15), unparsed), v146TestTag(t, "answer", "content", "Paris", 2, 1))
		v146TestError(t, result, 5, 2, "Лишняя закрывающая скобка.")
		v146TestRun(t, result, "current", nil)
	})
	t.Run("unclosed editor", func(t *testing.T) {
		result := v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1), v146TestTag(t, "answer", "content", "London", 2, 1),
			v146TestOpen(t, "editor", 2, 1), v146TestText(t, "@answer Paris", "@answer Paris", 5, 2))
		v146TestError(t, result, 5, -1, "Комментарий не закрыт.")
		v146TestRun(t, result, "current", nil)
	})
}

func TestV146PositionsEndingsAndHistory(t *testing.T) {
	t.Run("related ranges preserve indentation", func(t *testing.T) {
		result := v146TestDocument(t,
			v146TestLine(t, "\t@question Где?", "tag", 2, 1, v146TestElement(t, "tag", "@question", "question", 2, 11), v146TestElement(t, "content", "Где?", "Где?", 12, 16)),
			v146TestLine(t, "  @answer Лондон", "tag", 2, 1, v146TestElement(t, "tag", "@answer", "answer", 3, 10), v146TestElement(t, "content", "Лондон", "Лондон", 11, 17)),
			v146TestTag(t, "answer", "content", "Paris", 2, 1))
		v146TestRun(t, result, "current", []v146Expected{{[3]int{5, 1, 8}, [2][3]int{{4, 3, 10}, {3, 2, 11}}}})
	})
	for _, ending := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(ending, func(t *testing.T) {
			result := v146TestDocument(t,
				v146TestLine(t, "@QuEsTiOn Где?", "tag", 2, 1, v146TestElement(t, "tag", "@QuEsTiOn", "question", 1, 10), v146TestElement(t, "content", "Где?", "Где?", 11, 15)),
				v146TestLine(t, "@ANSWER Лондон", "tag", 2, 1, v146TestElement(t, "tag", "@ANSWER", "answer", 1, 8), v146TestElement(t, "content", "Лондон", "Лондон", 9, 15)),
				v146TestLine(t, " \t@AnSwEr Paris", "tag", 2, 1, v146TestElement(t, "tag", "@AnSwEr", "answer", 3, 10), v146TestElement(t, "content", "Paris", "Paris", 11, 16)))
			for index := range result.Lines {
				switch ending {
				case "LF":
					result.Lines[index].LineEnding = "\n"
				case "CRLF":
					result.Lines[index].LineEnding = "\r\n"
				case "mixed":
					if index%2 == 0 {
						result.Lines[index].LineEnding = "\r\n"
					}
				}
			}
			want := []v146Expected{{[3]int{5, 3, 10}, [2][3]int{{4, 1, 8}, {3, 1, 10}}}}
			v146TestRun(t, result, "first", want)
			v146TestRun(t, result, "second", want)
			v146TestRun(t, v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1),
				v146TestTag(t, "answer", "content", "London", 2, 1)), "separate", nil)
		})
	}
}

func TestV146EmptyAndRegistration(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v146TestRun(t, result, "empty", nil)
		})
	}
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
		if definition.Code != "V146" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "После одного @question подряд объявлено несколько @answer." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V146 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v146TestDocument(t, v146TestTag(t, "question", "content", "Where?", 2, 1),
				v146TestTag(t, "answer", "content", "London", 2, 1), v146TestTag(t, "answer", "content", "Paris", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0] = v146TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v146TestRun запускает штатный конвейер и проверяет только V146 текущего запуска.
func v146TestRun(t *testing.T, result *model.Result, source string, want []v146Expected) {
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
		if diagnostic.DiagnosticCode == "V146" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V146 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		primary := model.Location{Start: model.Position{Line: expected.primary[0], Column: expected.primary[1]}, End: model.Position{Line: expected.primary[0], Column: expected.primary[2]}}

		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "После одного @question подряд объявлено несколько @answer." ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 2 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for i, span := range expected.related {
			related := model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[0], Column: span[2]}}
			if entry.RelatedLocations[i] != related {
				t.Fatalf("related %d = %+v, want %+v", i, entry.RelatedLocations[i], related)
			}
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

// v146TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v146TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v146TestElement задаёт исходное и семантическое значения без преобразований.
func v146TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v146TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v146TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v146TestTag создаёт однострочное объявление с явно заданным параметром.
func v146TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v146TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v146TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v146TestLine(t, raw, "tag", parent, depth, elements...)
}

// v146TestOpen создаёт объявление блока без параметров.
func v146TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v146TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v146TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v146TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v146TestClose создаёт закрытие с заданным родителем.
func v146TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v146TestLine(t, "}", "block-end", parent, depth, v146TestElement(t, "block-close", "}", "}", 1, 2))
}

// v146TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v146TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v146TestLine(t, raw, "content", parent, depth, v146TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v146TestBlank задаёт обычную пустую строку без текстового элемента.
func v146TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v146TestLine(t, raw, "blank", parent, depth)
}

// v146TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v146TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v146TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v146TestLine(t, "# Lesson", "heading", 0, 0, v146TestElement(t, "heading-level", "#", "1", 1, 2), v146TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

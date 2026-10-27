package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v184Expected задаёт исходный диапазон [start,end) и независимый размер максимума.
type v184Expected struct {
	line, start, endLine, end int
	matched, total            int
}

func TestV184ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		items        []v184TestItem
		want         []v184Expected
	}{
		{"reference", "@ordering {\nA\n@answer {\n1\n2\n}\nA\n@answer 1\nB\n@answer 3\n}",
			[]v184TestItem{{"A", []string{"1", "2"}, false}, {"A", []string{"1"}, true}, {"B", []string{"3"}, true}}, nil},
		{"example", "@ordering {\nA\n@answer {\n1\n2\n}\nB\n@answer {\n1\n2\n}\nC\n@answer {\n1\n2\n}\nD\n@answer {\n3\n4\n}\n}",
			[]v184TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"1", "2"}, false}, {"C", []string{"1", "2"}, false}, {"D", []string{"3", "4"}, false}}, []v184Expected{{3, 1, 24, 2, 3, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v184TestItems(t, tc.items...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// По DSL 1.30 перед дословным фрагментом добавлены версия и основной заголовок.
			v184TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV184MaximumMatching(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []v184TestItem
		want  []v184Expected
	}{
		{"minimal conflict", []v184TestItem{{"A", []string{"1"}, true}, {"B", []string{"1"}, true}}, []v184Expected{{3, 1, 8, 2, 1, 2}}},
		{"normalized conflict", []v184TestItem{{"A", []string{"1"}, true}, {"B", []string{"1."}, true}}, []v184Expected{{3, 1, 8, 2, 1, 2}}},
		{"complete", []v184TestItem{{"A", []string{"1"}, true}, {"B", []string{"2"}, true}}, nil},
		{"normalized complete", []v184TestItem{{"A", []string{"1."}, true}, {"B", []string{"2!"}, true}}, nil},
		{"normalized audit conflict", []v184TestItem{{"A", []string{"1"}, true}, {"B", []string{"1."}, true}, {"C", []string{"2", "3."}, false}}, []v184Expected{{3, 1, 13, 2, 2, 3}}},
		{"one item", []v184TestItem{{"A", []string{"1"}, true}}, nil},
		{"duplicate number", []v184TestItem{{"A", []string{"1", "1"}, false}, {"B", []string{"1"}, true}}, []v184Expected{{3, 1, 11, 2, 1, 2}}},
		{"duplicate normalized edges conflict", []v184TestItem{{"A", []string{"1", "1.", "(1)", "«1»"}, false}, {"B", []string{"1!"}, true}}, []v184Expected{{3, 1, 13, 2, 1, 2}}},
		{"identical labels conflict", []v184TestItem{{"A", []string{"1"}, true}, {"A", []string{"1"}, true}}, []v184Expected{{3, 1, 8, 2, 1, 2}}},
		{"identical labels different sets", []v184TestItem{{"A", []string{"1"}, true}, {"A", []string{"2"}, true}}, nil},
		{"identical labels normalized different sets", []v184TestItem{{"A", []string{"1.", "2!"}, false}, {"A", []string{"(1)"}, true}}, nil},
		{"one reachable position", []v184TestItem{{"A", []string{"1"}, false}, {"B", []string{"1"}, false}, {"C", []string{"1"}, false}}, []v184Expected{{3, 1, 16, 2, 1, 3}}},
		{"two reachable positions", []v184TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"1", "2"}, false}, {"C", []string{"1", "2"}, false}}, []v184Expected{{3, 1, 19, 2, 2, 3}}},
		{"two deficient components", []v184TestItem{{"A", []string{"1"}, false}, {"B", []string{"1"}, false}, {"C", []string{"3"}, false}, {"D", []string{"3"}, false}}, []v184Expected{{3, 1, 20, 2, 2, 4}}},
		{"duplicate positions still allow full matching", []v184TestItem{{"A", []string{"1", "1", "2"}, false}, {"B", []string{"1", "1"}, false}}, nil},
		{"duplicate normalized edges allow full matching", []v184TestItem{{"A", []string{"1", "1.", "2!"}, false}, {"B", []string{"(1)", "1!"}, false}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v184TestRun(t, v184TestItems(t, tc.items...), "current", tc.want) })
	}
	// Единственное ограниченное ребро D→1 требует цепочки переназначений.
	chain := []v184TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"2", "3"}, false}, {"C", []string{"3", "4"}, false}, {"D", []string{"1"}, false}}
	for _, order := range [][4]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 3, 0, 2}, {2, 0, 3, 1}} {
		t.Run(fmt.Sprint("chain order ", order), func(t *testing.T) {
			items := []v184TestItem{chain[order[0]], chain[order[1]], chain[order[2]], chain[order[3]]}
			v184TestRun(t, v184TestItems(t, items...), "current", nil)
		})
	}
}

func TestV184DeficientGroupDetailsAndAnswers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		items   []v184TestItem
		want    v184Expected
		detail  string
		related []model.Location
	}{
		{"audit inline answers", []v184TestItem{{"A", []string{"1"}, true}, {"B", []string{"1."}, true}, {"C", []string{"2", "3."}, false}},
			v184Expected{3, 1, 13, 2, 2, 3},
			"Одновременно удаётся разместить 2 из 3 элементов. Дефицитная группа: элементы 1 «A», 2 «B»; доступно позиций: 1, требуется: 2, не хватает: 1.",
			[]model.Location{{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 10}}, {Start: model.Position{Line: 7, Column: 1}, End: model.Position{Line: 7, Column: 11}}}},
		{"audit block answers", []v184TestItem{{"A", []string{"1"}, false}, {"B", []string{"1."}, false}, {"C", []string{"2", "3."}, false}},
			v184Expected{3, 1, 17, 2, 2, 3},
			"Одновременно удаётся разместить 2 из 3 элементов. Дефицитная группа: элементы 1 «A», 2 «B»; доступно позиций: 1, требуется: 2, не хватает: 1.",
			[]model.Location{{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 7, Column: 2}}, {Start: model.Position{Line: 9, Column: 1}, End: model.Position{Line: 11, Column: 2}}}},
		{"identical instances retain individual answers", []v184TestItem{{"A", []string{"1"}, true}, {"A", []string{"1."}, true}, {"A", []string{"2", "3."}, false}},
			v184Expected{3, 1, 13, 2, 2, 3},
			"Одновременно удаётся разместить 2 из 3 элементов. Дефицитная группа: элементы 1 «A», 2 «A»; доступно позиций: 1, требуется: 2, не хватает: 1.",
			[]model.Location{{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 10}}, {Start: model.Position{Line: 7, Column: 1}, End: model.Position{Line: 7, Column: 11}}}},
		{"augmenting path reaches every deficient item", []v184TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"1"}, true}, {"C", []string{"2"}, true}},
			v184Expected{3, 1, 13, 2, 2, 3},
			"Одновременно удаётся разместить 2 из 3 элементов. Дефицитная группа: элементы 1 «A», 2 «B», 3 «C»; доступно позиций: 2, требуется: 3, не хватает: 1.",
			[]model.Location{{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 8, Column: 2}}, {Start: model.Position{Line: 10, Column: 1}, End: model.Position{Line: 10, Column: 10}}, {Start: model.Position{Line: 12, Column: 1}, End: model.Position{Line: 12, Column: 10}}}},
		{"two independent deficiencies", []v184TestItem{{"A", []string{"1"}, true}, {"B", []string{"1"}, true}, {"C", []string{"3"}, true}, {"D", []string{"3"}, true}},
			v184Expected{3, 1, 12, 2, 2, 4},
			"Одновременно удаётся разместить 2 из 4 элементов. Дефицитная группа: элементы 1 «A», 2 «B», 3 «C», 4 «D»; доступно позиций: 2, требуется: 4, не хватает: 2.",
			[]model.Location{{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 10}}, {Start: model.Position{Line: 7, Column: 1}, End: model.Position{Line: 7, Column: 10}}, {Start: model.Position{Line: 9, Column: 1}, End: model.Position{Line: 9, Column: 10}}, {Start: model.Position{Line: 11, Column: 1}, End: model.Position{Line: 11, Column: 10}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := v184TestRun(t, v184TestItems(t, tc.items...), "current", []v184Expected{tc.want})
			v184TestDetail(t, got[0], tc.detail, tc.related)
		})
	}
	t.Run("rune columns and complete declarations", func(t *testing.T) {
		result := v184TestOrdering(t, v184TestText(t, " \tА🌞 ", "А🌞", 3, 2),
			v184TestLine(t, " \t@answer (1) \u2003", "tag", 3, 2, v184TestElement(t, "tag", "@answer", "answer", 3, 10), v184TestElement(t, "content", "(1)", "(1)", 11, 14)),
			v184TestText(t, "Б", "Б", 3, 2),
			v184TestLine(t, "\t  @answer { ", "block-start", 3, 2, v184TestElement(t, "tag", "@answer", "answer", 4, 11), v184TestElement(t, "block-open", "{", "{", 12, 13)),
			v184TestText(t, "1.", "1.", 7, 3), v184TestLine(t, "  } \t", "block-end", 7, 3, v184TestElement(t, "block-close", "}", "}", 3, 4)))
		got := v184TestRun(t, result, "current", []v184Expected{{3, 1, 10, 2, 1, 2}})
		v184TestDetail(t, got[0], "Одновременно удаётся разместить 1 из 2 элементов. Дефицитная группа: элементы 1 «А🌞», 2 «Б»; доступно позиций: 1, требуется: 2, не хватает: 1.",
			[]model.Location{{Start: model.Position{Line: 5, Column: 3}, End: model.Position{Line: 5, Column: 16}}, {Start: model.Position{Line: 7, Column: 4}, End: model.Position{Line: 9, Column: 4}}})
	})
}

func TestV184FragmentedAnswerGaps(t *testing.T) {
	for _, inline := range []bool{true, false} {
		for _, gap := range []string{" ", "\t", "\u00a0", "\u2003"} {
			for _, number := range []string{"1", "2"} {
				t.Run("inline="+strconv.FormatBool(inline)+"/gap="+strconv.Quote(gap)+"/number="+number, func(t *testing.T) {
					result := v184TestFragmentedAnswer(t, inline, "("+gap+number+")",
						v184TestElement(t, "content", "(", "(", 1, 2), v184TestElement(t, "content", number+")", number+")", 3, 5))
					var want []v184Expected
					if number == "1" {
						endLine := 8
						if !inline {
							endLine = 10
						}
						want = []v184Expected{{3, 1, endLine, 2, 1, 2}}
					}
					v184TestRun(t, result, "current", want)
				})
			}
		}
	}
}

func TestV184FragmentedAnswerCompleteness(t *testing.T) {
	for _, inline := range []bool{true, false} {
		for _, tc := range []struct {
			name, raw string
			parts     []model.Element
			conflict  bool
		}{
			{"known contiguous fragments", "(1)", []model.Element{v184TestElement(t, "content", "(", "(", 1, 2), v184TestElement(t, "content", "1)", "1)", 2, 4)}, true},
			{"known decoded fragments", `\u0031.`, []model.Element{v184TestElement(t, "content", `\u0031`, "1", 1, 7), v184TestElement(t, "content", ".", ".", 7, 8)}, true},
			{"known empty mark inside content", "(1)", []model.Element{v184TestElement(t, "content", "(1)", "(1)", 1, 4), v184TestElement(t, "content", "", "", 2, 2)}, true},
			{"known whitespace prefix and suffix", "\u00a0\u20031\t ", []model.Element{v184TestElement(t, "content", "1", "1", 3, 4)}, true},
			{"uncovered significant prefix", "x1", []model.Element{v184TestElement(t, "content", "1", "1", 2, 3)}, false},
			{"uncovered significant suffix", "1x", []model.Element{v184TestElement(t, "content", "1", "1", 1, 2)}, false},
			{"uncovered punctuation prefix", "!1", []model.Element{v184TestElement(t, "content", "1", "1", 2, 3)}, false},
			{"uncovered punctuation suffix", "1!", []model.Element{v184TestElement(t, "content", "1", "1", 1, 2)}, false},
			{"uncovered significant interior", "(x1)", []model.Element{v184TestElement(t, "content", "(", "(", 1, 2), v184TestElement(t, "content", "1)", "1)", 3, 5)}, false},
			{"zero width space is significant", "(\u200b1)", []model.Element{v184TestElement(t, "content", "(", "(", 1, 2), v184TestElement(t, "content", "1)", "1)", 3, 5)}, false},
			{"unknown empty mark inside content", "(1)", []model.Element{v184TestElement(t, "content", "(1)", "(1)", 1, 4), v184TestElement(t, "placeholder", "", "", 2, 2)}, false},
			{"unresolved empty mark inside content", "(1)", []model.Element{v184TestElement(t, "content", "(1)", "(1)", 1, 4), {ElementType: "placeholder", Raw: "", Value: nil, Start: 2, End: 2, ErrorIDs: []string{}}}, false},
		} {
			t.Run("inline="+strconv.FormatBool(inline)+"/"+tc.name, func(t *testing.T) {
				var want []v184Expected
				if tc.conflict {
					endLine := 8
					if !inline {
						endLine = 10
					}
					want = []v184Expected{{3, 1, endLine, 2, 1, 2}}
				}
				v184TestRun(t, v184TestFragmentedAnswer(t, inline, tc.raw, tc.parts...), "current", want)
			})
		}
	}
}

func TestV184AssociationAndExclusions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rows      []model.Line
		want      []v184Expected
		errorLine int
	}{
		{"no items", nil, nil, 3},
		{"no answers", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestText(t, "B", "B", 3, 2)}, nil, 0},
		{"missing last answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2)}, nil, 0},
		{"missing first answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 0},
		{"orphan answer", []model.Line{v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 4},
		{"repeated answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 6},
		{"repeated answer after hint", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestTag(t, "hint", "content", "Note", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 7},
		{"blank before answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestBlank(t, "", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, []v184Expected{{3, 1, 9, 2, 1, 2}}, 0},
		{"ASCII blank before answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestBlank(t, " \t ", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, []v184Expected{{3, 1, 9, 2, 1, 2}}, 0},
		{"hint before answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "hint", "content", "Note", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 5},
		{"inline editor before answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "editor", "content", "Note", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 5},
		{"hint after answer", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestTag(t, "hint", "content", "Note", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, []v184Expected{{3, 1, 9, 2, 1, 2}}, 0},
		{"unknown item", []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestInvalid(t, "@unknown", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v184TestOrdering(t, tc.rows...)
			if tc.errorLine != 0 {
				v184TestError(t, result, tc.errorLine, -1, "Нарушены правила содержимого или размещения тега.")
			}
			v184TestRun(t, result, "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "note"} {
		t.Run("block before answer/"+tag, func(t *testing.T) {
			result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2), v184TestOpen(t, tag, 3, 2), v184TestText(t, "Note", "Note", 5, 3), v184TestClose(t, 5, 3),
				v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
			v184TestError(t, result, 5, -1, "Тег запрещён в ordering.")
			v184TestRun(t, result, "current", nil)
		})
	}
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"text", v184TestText(t, "Note", "Note", 6, 3)},
		{"separator", v184TestText(t, "---", "---", 6, 3)},
		{"unknown", v184TestInvalid(t, "@unknown", 6, 3)},
	} {
		t.Run("hint body/"+tc.name, func(t *testing.T) {
			result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestOpen(t, "hint", 3, 2), tc.row, v184TestClose(t, 6, 3), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
			v184TestRun(t, result, "current", []v184Expected{{3, 1, 11, 2, 1, 2}})
		})
	}
}

func TestV184AnswerValues(t *testing.T) {
	for _, value := range []string{"0", "01", "+1", "−1", "1.0", "1 2", "1\t2", "1\u00a02", "١", "１", "(01)", "(0)", "(!)", "\u00a0\t", "3", "3!", "999999999999999999999999999999", "999999999999999999999999999999."} {
		for _, inline := range []bool{true, false} {
			t.Run(strconv.Quote(value)+"/inline="+strconv.FormatBool(inline), func(t *testing.T) {
				result := v184TestItems(t, v184TestItem{"A", []string{value}, inline}, v184TestItem{"B", []string{"1"}, true})
				v184TestRun(t, result, "current", nil)
			})
		}
	}
	for _, tc := range []struct {
		name      string
		values    []model.Line
		want      []v184Expected
		errorLine int
	}{
		{"empty block", nil, nil, 5},
		{"only blanks", []model.Line{v184TestBlank(t, "", 5, 3), v184TestBlank(t, " \t", 5, 3)}, nil, 5},
		{"blank beside valid number", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestBlank(t, "", 5, 3)}, []v184Expected{{3, 1, 11, 2, 1, 2}}, 0},
		{"ASCII blank beside valid number", []model.Line{v184TestBlank(t, " \t", 5, 3), v184TestText(t, "1", "1", 5, 3)}, []v184Expected{{3, 1, 11, 2, 1, 2}}, 0},
		{"content whitespace without elements cancels graph", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestLine(t, " \t", "content", 5, 3)}, nil, 0},
		{"blank with content whitespace preserves conflict", []model.Line{v184TestLine(t, " \t", "blank", 5, 3, v184TestElement(t, "content", " \t", " \t", 1, 3)), v184TestText(t, "1", "1", 5, 3)}, []v184Expected{{3, 1, 11, 2, 1, 2}}, 0},
		{"empty decoded content cancels graph", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestText(t, "\\", "", 5, 3)}, nil, 0},
		{"empty content beside valid number cancels graph", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestText(t, "", "", 5, 3)}, nil, 0},
		{"valid then invalid", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestText(t, "01", "01", 5, 3)}, nil, 0},
		{"invalid then valid", []model.Line{v184TestText(t, "01", "01", 5, 3), v184TestText(t, "1", "1", 5, 3)}, nil, 0},
		{"valid then unknown", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestInvalid(t, "@unknown", 5, 3)}, nil, 0},
		{"unknown then valid", []model.Line{v184TestInvalid(t, "@unknown", 5, 3), v184TestText(t, "1", "1", 5, 3)}, nil, 0},
		{"nested content excluded", []model.Line{v184TestText(t, "1", "1", 5, 3), v184TestOpen(t, "note", 5, 3), v184TestText(t, "2", "2", 7, 4), v184TestClose(t, 7, 4)}, []v184Expected{{3, 1, 13, 2, 1, 2}}, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestOpen(t, "answer", 3, 2)}
			rows = append(rows, tc.values...)
			rows = append(rows, v184TestClose(t, 5, 3), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
			result := v184TestOrdering(t, rows...)
			if tc.errorLine != 0 {
				v184TestError(t, result, tc.errorLine, -1, "Пустой ответ или запрещённое вложение.")
			}
			v184TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("empty inline answer", func(t *testing.T) {
		result := v184TestItems(t, v184TestItem{"A", []string{""}, true}, v184TestItem{"B", []string{"1"}, true})
		v184TestError(t, result, 5, -1, "Отсутствует значение ответа.")
		v184TestRun(t, result, "current", nil)
	})
	t.Run("unparsed inline answer", func(t *testing.T) {
		unparsed := v184TestElement(t, "unparsed", "???", "", 9, 12)
		unparsed.Value = nil
		result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2), v184TestLine(t, "@answer ???", "invalid", 3, 2, v184TestElement(t, "tag", "@answer", "answer", 1, 8), unparsed),
			v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
		v184TestRun(t, result, "current", nil)
	})
	t.Run("ten is within range", func(t *testing.T) {
		var items []v184TestItem
		for i := 0; i < 10; i++ {
			items = append(items, v184TestItem{"A", []string{"10"}, true})
		}
		v184TestRun(t, v184TestItems(t, items...), "current", []v184Expected{{3, 1, 24, 2, 1, 10}})
	})
}

func TestV184AnswerNormalization(t *testing.T) {
	for _, value := range []string{"1.", "(1)", "-1", `\1`, "«—1»", "\u00a01", "1\u00a0", "\u202f\t1!\u2003", "(\u00a01\u3000)"} {
		for _, inline := range []bool{true, false} {
			t.Run(strconv.Quote(value)+"/inline="+strconv.FormatBool(inline), func(t *testing.T) {
				endLine := 8
				if !inline {
					endLine = 10
				}
				result := v184TestItems(t, v184TestItem{"A", []string{value}, inline}, v184TestItem{"B", []string{"1"}, true})
				v184TestRun(t, result, "current", []v184Expected{{3, 1, endLine, 2, 1, 2}})
			})
		}
	}
	t.Run("normalized multi digit number is checked against item count", func(t *testing.T) {
		var items []v184TestItem
		for i := 0; i < 10; i++ {
			items = append(items, v184TestItem{"A", []string{"1.0!"}, true})
		}
		v184TestRun(t, v184TestItems(t, items...), "current", []v184Expected{{3, 1, 24, 2, 1, 10}})
	})
	for _, tc := range []struct {
		name, raw, value string
		want             []v184Expected
	}{
		{"decoded value is normalized", `\u0031.`, "1.", []v184Expected{{3, 1, 10, 2, 1, 2}}},
		{"decoded value is not decoded again", `\\u0031`, `\u0031`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2), v184TestOpen(t, "answer", 3, 2),
				v184TestText(t, tc.raw, tc.value, 5, 3), v184TestClose(t, 5, 3), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
			v184TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV184UnusableAlternativeSuppressesWholeGraph(t *testing.T) {
	placeholder := v184TestElement(t, "placeholder", "_____", "", 1, 6)
	placeholder.Value = nil
	for _, tc := range []struct {
		name   string
		values []model.Line
	}{
		{"no alternatives", nil},
		{"only physical blanks", []model.Line{v184TestBlank(t, " \t", 9, 3)}},
		{"unknown after valid", []model.Line{v184TestText(t, "2", "2", 9, 3), v184TestText(t, "unknown", "unknown", 9, 3)}},
		{"unknown before valid", []model.Line{v184TestText(t, "unknown", "unknown", 9, 3), v184TestText(t, "2", "2", 9, 3)}},
		{"out of range after valid", []model.Line{v184TestText(t, "2", "2", 9, 3), v184TestText(t, "4!", "4!", 9, 3)}},
		{"empty after valid", []model.Line{v184TestText(t, "2", "2", 9, 3), v184TestText(t, "", "", 9, 3)}},
		{"Unicode whitespace after valid", []model.Line{v184TestText(t, "2", "2", 9, 3), v184TestText(t, "\u00a0\u2003", "\u00a0\u2003", 9, 3)}},
		{"unparsed after valid", []model.Line{v184TestText(t, "2", "2", 9, 3), v184TestInvalid(t, "???", 9, 3)}},
		{"unparsed before valid", []model.Line{v184TestInvalid(t, "???", 9, 3), v184TestText(t, "2", "2", 9, 3)}},
		{"unresolved after valid", []model.Line{v184TestText(t, "2", "2", 9, 3), v184TestLine(t, "_____", "content", 9, 3, placeholder)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A и B уже конфликтуют; непригодная альтернатива C отменяет весь вывод.
			rows := []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2),
				v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1.", 3, 2), v184TestText(t, "C", "C", 3, 2), v184TestOpen(t, "answer", 3, 2)}
			rows = append(rows, tc.values...)
			rows = append(rows, v184TestClose(t, 9, 3))
			v184TestRun(t, v184TestOrdering(t, rows...), "current", nil)
		})
	}
}

func TestV184FragmentedUnknownAlternativeSuppressesWholeGraph(t *testing.T) {
	for _, inline := range []bool{true, false} {
		for _, tc := range []struct {
			name, raw string
			parts     []model.Element
		}{
			{"unknown value", "(unknown)", []model.Element{v184TestElement(t, "content", "(", "(", 1, 2), v184TestElement(t, "content", "unknown)", "unknown)", 2, 10)}},
			{"out of range value", "(4)", []model.Element{v184TestElement(t, "content", "(", "(", 1, 2), v184TestElement(t, "content", "4)", "4)", 2, 4)}},
			{"unresolved fragment", "(2)", []model.Element{v184TestElement(t, "content", "(", "(", 1, 2), {ElementType: "placeholder", Raw: "2", Value: nil, Start: 2, End: 3, ErrorIDs: []string{}}, v184TestElement(t, "content", ")", ")", 3, 4)}},
			{"unknown empty mark inside known content", "(2)", []model.Element{v184TestElement(t, "content", "(2)", "(2)", 1, 4), v184TestElement(t, "placeholder", "", "", 2, 2)}},
			{"unresolved empty mark inside known content", "(2)", []model.Element{v184TestElement(t, "content", "(2)", "(2)", 1, 4), {ElementType: "placeholder", Raw: "", Value: nil, Start: 2, End: 2, ErrorIDs: []string{}}}},
		} {
			t.Run("inline="+strconv.FormatBool(inline)+"/"+tc.name, func(t *testing.T) {
				// Конфликт A/B обязан подавляться даже при известной части ответа C.
				result := v184TestItems(t, v184TestItem{"A", []string{"1"}, true}, v184TestItem{"B", []string{"1."}, true}, v184TestItem{"C", []string{"2"}, inline})
				row := 9
				if !inline {
					row = 10
				}
				result.Lines[row-1] = v184TestFragmentLine(t, inline, tc.raw, tc.parts...)
				result.Lines[row-1].Number = row
				if !inline {
					parent := 9
					result.Lines[row-1].ParentLine = &parent
				}
				v184TestRun(t, result, "current", nil)
			})
		}
	}
}

func TestV184SeparatorsAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
	}{
		{"one", []model.Line{v184TestLine(t, "---", "separator", 3, 2)}},
		{"two", []model.Line{v184TestLine(t, "---", "separator", 3, 2), v184TestLine(t, "---", "separator", 3, 2)}},
		{"ASCII edges", []model.Line{v184TestLine(t, " \t--- \t", "separator", 3, 2)}},
		{"raw in invalid", []model.Line{v184TestInvalid(t, "---", 3, 2)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2)}
			rows = append(rows, tc.rows...)
			rows = append(rows, v184TestText(t, "First", "First", 3, 2), v184TestText(t, "Last", "Last", 3, 2))
			v184TestRun(t, v184TestOrdering(t, rows...), "current", nil)
		})
	}
	t.Run("normalized conflict in extended ordering is excluded", func(t *testing.T) {
		result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2),
			v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1.", 3, 2), v184TestLine(t, "---", "separator", 3, 2),
			v184TestText(t, "First", "First", 3, 2), v184TestText(t, "Last", "Last", 3, 2))
		v184TestRun(t, result, "current", nil)
	})
	t.Run("escaped separator item", func(t *testing.T) {
		result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, `\---`, "---", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
		v184TestRun(t, result, "current", []v184Expected{{3, 1, 8, 2, 1, 2}})
	})
	t.Run("indented block", func(t *testing.T) {
		result := v184TestDocument(t,
			v184TestLine(t, " \t@ordering { \t", "block-start", 2, 1, v184TestElement(t, "tag", "@ordering", "ordering", 3, 12), v184TestElement(t, "block-open", "{", "{", 13, 14)),
			v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2),
			v184TestLine(t, " \t} \t", "block-end", 3, 2, v184TestElement(t, "block-close", "}", "}", 3, 4)))
		v184TestRun(t, result, "current", []v184Expected{{3, 3, 8, 4, 1, 2}})
	})
	t.Run("indented answer number", func(t *testing.T) {
		result := v184TestOrdering(t, v184TestText(t, "A", "A", 3, 2),
			v184TestLine(t, " \t@answer 1 \t", "tag", 3, 2, v184TestElement(t, "tag", "@answer", "answer", 3, 10), v184TestElement(t, "content", "1", "1", 11, 12)),
			v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2))
		v184TestRun(t, result, "current", []v184Expected{{3, 1, 8, 2, 1, 2}})
	})
}

func TestV184Contexts(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		prefix, suffix         []model.Line
		opening, parent, depth int
		want                   []v184Expected
	}{
		{"task", []model.Line{v184TestTag(t, "task", "identifier", "first", 2, 1)}, []model.Line{v184TestTag(t, "endtask", "", "", 2, 1)}, 4, 3, 2, []v184Expected{{4, 1, 9, 2, 1, 2}}},
		{"step", []model.Line{v184TestTag(t, "task", "identifier", "first", 2, 1), v184TestTag(t, "step", "title", "First", 3, 2)}, []model.Line{v184TestTag(t, "endtask", "", "", 2, 1)}, 5, 4, 3, []v184Expected{{5, 1, 10, 2, 1, 2}}},
		{"variant", []model.Line{v184TestOpen(t, "variants", 2, 1), v184TestTag(t, "variant", "name", "A", 3, 2)}, []model.Line{v184TestTag(t, "variant", "name", "B", 3, 2), v184TestText(t, "Читайте.", "Читайте.", 11, 3), v184TestClose(t, 3, 2)}, 5, 4, 3, []v184Expected{{5, 1, 10, 2, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.prefix, v184TestOpen(t, "ordering", tc.parent, tc.depth), v184TestText(t, "A", "A", tc.opening, tc.depth+1), v184TestTag(t, "answer", "content", "1", tc.opening, tc.depth+1),
				v184TestText(t, "B", "B", tc.opening, tc.depth+1), v184TestTag(t, "answer", "content", "1", tc.opening, tc.depth+1), v184TestClose(t, tc.opening, tc.depth+1))
			body = append(body, tc.suffix...)
			v184TestRun(t, v184TestDocument(t, body...), "current", tc.want)
		})
	}
	t.Run("independent blocks", func(t *testing.T) {
		var body []model.Line
		for _, opening := range []int{3, 9} {
			body = append(body, v184TestOpen(t, "ordering", 2, 1), v184TestText(t, "A", "A", opening, 2), v184TestTag(t, "answer", "content", "1", opening, 2),
				v184TestText(t, "B", "B", opening, 2), v184TestTag(t, "answer", "content", "1", opening, 2), v184TestClose(t, opening, 2))
		}
		v184TestRun(t, v184TestDocument(t, body...), "current", []v184Expected{{3, 1, 8, 2, 1, 2}, {9, 1, 14, 2, 1, 2}})
	})
	t.Run("matching excluded", func(t *testing.T) {
		result := v184TestDocument(t, v184TestOpen(t, "matching", 2, 1), v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2),
			v184TestLine(t, "---", "separator", 3, 2), v184TestText(t, "1", "1", 3, 2), v184TestText(t, "2", "2", 3, 2), v184TestClose(t, 3, 2))
		v184TestRun(t, result, "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal ordering in "+tag, func(t *testing.T) {
			result := v184TestDocument(t, v184TestOpen(t, tag, 2, 1), v184TestText(t, "@ordering {", "@ordering {", 3, 2), v184TestText(t, "A", "A", 3, 2), v184TestText(t, "@answer 1", "@answer 1", 3, 2),
				v184TestText(t, "B", "B", 3, 2), v184TestText(t, "@answer 1", "@answer 1", 3, 2), v184TestText(t, `\}`, "}", 3, 2), v184TestClose(t, 3, 2))
			v184TestRun(t, result, "current", nil)
		})
	}
	t.Run("mixed case and Unicode labels", func(t *testing.T) {
		result := v184TestItems(t, v184TestItem{"🌞е\u0301", []string{"1"}, true}, v184TestItem{"🌞е\u0301", []string{"1"}, true})
		result.Lines[2].Raw = "@OrDeRiNg {"
		result.Lines[2].Elements[0].Raw = "@OrDeRiNg"
		result.Lines[4].Raw = "@AnSwEr 1"
		result.Lines[4].Elements[0].Raw = "@AnSwEr"
		v184TestRun(t, result, "current", []v184Expected{{3, 1, 8, 2, 1, 2}})
	})
}

func TestV184RegistrationAndHistory(t *testing.T) {
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
		if definition.Code != "V184" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" || definition.Message != "Указанные ответы не позволяют разместить все элементы по разным позициям." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V184 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v184TestBase(t)
			result.Document.DSLVersion = &version
			result.Lines[0] = v184TestTag(t, "dsl-version", "version", version, 0, 0)
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
			v184TestRun(t, result, "current", nil)
		})
	}
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v184TestBase(t)
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v184Expected{{3, 1, 8, 2, 1, 2}}
			v184TestRun(t, result, "first", want)
			v184TestRun(t, result, "second", want)
			v184TestRun(t, v184TestItems(t, v184TestItem{"A", []string{"1", "2"}, false}, v184TestItem{"A", []string{"1"}, true}, v184TestItem{"B", []string{"3"}, true}), "separate", nil)
		})
	}
}

// v184TestItem описывает экземпляр элемента и форму его ответа, без вычисления ожиданий.
type v184TestItem struct {
	text      string
	positions []string
	inline    bool
}

// v184TestFragmentedAnswer задаёт A→1 и ответ B из явных фрагментов исходной строки.
func v184TestFragmentedAnswer(t *testing.T, inline bool, raw string, parts ...model.Element) *model.Result {
	t.Helper()
	rows := []model.Line{v184TestText(t, "A", "A", 3, 2), v184TestTag(t, "answer", "content", "1", 3, 2), v184TestText(t, "B", "B", 3, 2)}
	if inline {
		rows = append(rows, v184TestFragmentLine(t, true, raw, parts...))
	} else {
		rows = append(rows, v184TestOpen(t, "answer", 3, 2), v184TestFragmentLine(t, false, raw, parts...), v184TestClose(t, 7, 3))
	}
	return v184TestOrdering(t, rows...)
}

// v184TestFragmentLine сохраняет фрагменты с координатами относительно значения.
func v184TestFragmentLine(t *testing.T, inline bool, raw string, parts ...model.Element) model.Line {
	t.Helper()
	if !inline {
		return v184TestLine(t, raw, "content", 7, 3, parts...)
	}
	elements := []model.Element{v184TestElement(t, "tag", "@answer", "answer", 1, 8)}
	for _, part := range parts {
		part.Start += 8
		part.End += 8
		elements = append(elements, part)
	}
	return v184TestLine(t, "@answer "+raw, "tag", 3, 2, elements...)
}

// v184TestDetail сравнивает полный текст и исходные диапазоны без анализа графа.
func v184TestDetail(t *testing.T, got model.Diagnostic, detail string, related []model.Location) {
	t.Helper()
	message := "Указанные ответы не позволяют разместить все элементы по разным позициям.\nконкретизация: " + detail
	if got.Message != message {
		t.Errorf("message = %q, want %q", got.Message, message)
	}
	if !slices.Equal(got.RelatedLocations, related) {
		t.Errorf("related = %+v, want %+v", got.RelatedLocations, related)
	}
}

// v184TestItems строит согласованный блок с явными наборами допустимых позиций.
func v184TestItems(t *testing.T, items ...v184TestItem) *model.Result {
	t.Helper()
	body := []model.Line{v184TestOpen(t, "ordering", 2, 1)}
	for _, item := range items {
		body = append(body, v184TestText(t, item.text, item.text, 3, 2))
		if item.inline {
			if len(item.positions) != 1 {
				t.Fatal("inline fixture needs exactly one value")
			}
			body = append(body, v184TestTag(t, "answer", "content", item.positions[0], 3, 2))
			continue
		}
		answer := len(body) + 3
		body = append(body, v184TestOpen(t, "answer", 3, 2))
		for _, position := range item.positions {
			body = append(body, v184TestText(t, position, position, answer, 3))
		}
		body = append(body, v184TestClose(t, answer, 3))
	}
	body = append(body, v184TestClose(t, 3, 2))
	return v184TestDocument(t, body...)
}

// v184TestBase создаёт минимальный конфликт с двумя однострочными ответами.
func v184TestBase(t *testing.T) *model.Result {
	t.Helper()
	return v184TestItems(t, v184TestItem{"A", []string{"1"}, true}, v184TestItem{"B", []string{"1"}, true})
}

// v184TestOrdering задаёт ordering на строке 3 с переданными непосредственными строками.
func v184TestOrdering(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v184TestOpen(t, "ordering", 2, 1)}, rows...)
	body = append(body, v184TestClose(t, 3, 2))
	return v184TestDocument(t, body...)
}

// v184TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v184TestDocument.
func v184TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v184TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v184TestLine(t, raw, "invalid", parent, depth, element)
}

// v184TestRun запускает штатный конвейер и проверяет только V184 текущего запуска.
func v184TestRun(t *testing.T, result *model.Result, source string, want []v184Expected) []model.Diagnostic {
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
		if diagnostic.DiagnosticCode == "V184" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V184 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := fmt.Sprintf("Указанные ответы не позволяют разместить все элементы по разным позициям.\nконкретизация: Одновременно удаётся разместить %d из %d элементов.", expected.matched, expected.total)
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.endLine, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" ||
			!strings.HasPrefix(entry.Message, message+" Дефицитная группа: элементы ") ||
			entry.Location == nil || *entry.Location != primary {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("ScopeBlock must not create element errorIds")
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
	return got
}

// v184TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v184TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v184TestElement задаёт исходное и семантическое значения без преобразований.
func v184TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v184TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v184TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v184TestTag создаёт однострочное объявление с явно заданным параметром.
func v184TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v184TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v184TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v184TestLine(t, raw, "tag", parent, depth, elements...)
}

// v184TestOpen создаёт объявление блока без параметров.
func v184TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v184TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v184TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v184TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v184TestClose создаёт закрытие с заданным родителем.
func v184TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v184TestLine(t, "}", "block-end", parent, depth, v184TestElement(t, "block-close", "}", "}", 1, 2))
}

// v184TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v184TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v184TestLine(t, raw, "content", parent, depth, v184TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v184TestBlank задаёт обычную пустую строку без текстового элемента.
func v184TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v184TestLine(t, raw, "blank", parent, depth)
}

// v184TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v184TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v184TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v184TestLine(t, "# Lesson", "heading", 0, 0, v184TestElement(t, "heading-level", "#", "1", 1, 2), v184TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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
				v184TestError(t, result, i+1, j, "Строка разобрана не полностью.")
			}
		}
	}
	return result

}

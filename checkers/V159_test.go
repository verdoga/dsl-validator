package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v159Expected содержит независимые исходные координаты [start,end), с единицы.
type v159Expected struct {
	line, start, end int
	detail           string
	related          [][4]int
}

func TestV159Registration(t *testing.T) {
	definitions, err := v159TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V159" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Подсказка @hint не следует непосредственно за @answer." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V159 count = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			result := v159TestDocument(t)
			result.Document.DSLVersion = &version
			before := v159TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v159TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v159TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV159ReferenceAndExamples(t *testing.T) {
	// К фрагментам добавлены только техническая версия и обязательный заголовок.
	t.Run("reference", func(t *testing.T) {
		result := v159TestChoice(t, "multichoice",
			v159TestTag(t, "answer", "Italy", 3), v159TestTag(t, "hint", "Rome is its capital.", 3),
			v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 6),
			v159TestTextRow(t, "France", "France", 6), v159TestClose(t, 6),
			v159TestTag(t, "hint", "Both countries are in Europe.", 3), v159TestTextRow(t, "Canada", "Canada", 3))
		v159TestSource(t, result, "@multichoice {\n@answer Italy\n@hint Rome is its capital.\n@answer {\nSpain\nFrance\n}\n@hint Both countries are in Europe.\nCanada\n}")
		v159TestRun(t, result, "current", nil)
	})
	t.Run("repeat example", func(t *testing.T) {
		result := v159TestChoice(t, "choice", v159TestTag(t, "answer", "Spain", 3),
			v159TestTag(t, "hint", "Madrid is its capital.", 3), v159TestTag(t, "hint", "It is in Europe.", 3),
			v159TestTextRow(t, "France", "France", 3))
		v159TestSource(t, result, "@choice {\n@answer Spain\n@hint Madrid is its capital.\n@hint It is in Europe.\nFrance\n}")
		v159TestRun(t, result, "current", []v159Expected{{6, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 6}}}})
	})
	t.Run("gap example", func(t *testing.T) {
		result := v159TestChoice(t, "multichoice", v159TestTag(t, "answer", "Spain", 3),
			v159TestTextRow(t, "Canada", "Canada", 3), v159TestTag(t, "hint", "Madrid is its capital.", 3))
		v159TestSource(t, result, "@multichoice {\n@answer Spain\nCanada\n@hint Madrid is its capital.\n}")
		v159TestRun(t, result, "current", []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 7}}}})
	})
}

// v159TestSource сверяет буквальное содержимое эталона или примера без контекста.
func v159TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var raw []string
	for _, line := range result.Lines[2:] {
		raw = append(raw, line.Raw)
	}
	if strings.Join(raw, "\n") != want {
		t.Fatal("table fragment changed")
	}
}

func TestV159Adjacency(t *testing.T) {
	for _, container := range []string{"choice", "multichoice"} {
		t.Run(container, func(t *testing.T) {
			answer := func() model.Line { return v159TestTag(t, "answer", "Spain", 3) }
			hint := func() model.Line { return v159TestTag(t, "hint", "Madrid.", 3) }
			option := func() model.Line { return v159TestTextRow(t, "Canada", "Canada", 3) }
			// У большинства ситуаций два варианта; заключительная France не влияет на поиск назад.
			for _, tc := range []struct {
				name string
				body []model.Line
				want []v159Expected
			}{
				{"inline answer", []model.Line{answer(), hint()}, nil},
				{"blank lines", []model.Line{answer(), v159TestLine(t, "", "blank", 3), v159TestLine(t, " \t ", "blank", 3), hint()}, nil},
				{"orphan", []model.Line{hint(), option()}, []v159Expected{{4, 1, 6, "", nil}}},
				{"option only", []model.Line{option(), hint()}, []v159Expected{{5, 1, 6, "", nil}}},
				{"repeat", []model.Line{answer(), hint(), hint()}, []v159Expected{{6, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 6}}}}},
				{"third hint", []model.Line{answer(), hint(), hint(), hint()}, []v159Expected{
					{6, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 6}}}, {7, 1, 6, "", nil}}},
				{"ordinary gap", []model.Line{answer(), option(), hint()}, []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 7}}}}},
				{"two options exceed window", []model.Line{answer(), option(), v159TestTextRow(t, "Italy", "Italy", 3), hint()}, []v159Expected{{7, 1, 6, "", nil}}},
				{"tag gap", []model.Line{answer(), v159TestTag(t, "note", "Read.", 3), hint()}, []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 6}}}}},
				{"editor is not skipped", []model.Line{answer(), v159TestTag(t, "editor", "Check.", 3), hint()}, []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 8}}}}},
				{"NBSP is significant", []model.Line{answer(), v159TestTextRow(t, "\u00a0", "\u00a0", 3), hint()}, []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 2}}}}},
				{"raw option edges", []model.Line{answer(), v159TestLine(t, "  Canada ", "content", 3, v159TestElement(t, "content", "Canada", "Canada", 3)), hint()}, []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 10}}}}},
				{"Unicode option", []model.Line{answer(), v159TestTextRow(t, "Япония 🌏", "Япония 🌏", 3), hint()}, []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 9}}}}},
				{"blank lines around gap", []model.Line{answer(), v159TestLine(t, "", "blank", 3), option(), v159TestLine(t, "\t", "blank", 3), hint()}, []v159Expected{{8, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {6, 1, 6, 7}}}}},
				{"no hint", []model.Line{answer()}, nil},
				{"escaped hint is option", []model.Line{answer(), v159TestTextRow(t, `\@hint Madrid.`, "@hint Madrid.", 3)}, nil},
				{"escaped answer is option", []model.Line{v159TestTextRow(t, `\@answer Spain`, "@answer Spain", 3), hint()}, []v159Expected{{5, 1, 6, "", nil}}},
				{"tag mention is option", []model.Line{answer(), v159TestTextRow(t, "See @hint Madrid.", "See @hint Madrid.", 3)}, nil},
				{"invalid predecessor", []model.Line{answer(), v159TestInvalid(t, "@unknown", 3), hint()}, []v159Expected{{6, 1, 6, "", nil}}},
				{"unparsed predecessor", []model.Line{answer(), v159TestLine(t, "@note ?", "tag", 3, v159TestElement(t, "tag", "@note", "note", 1), v159TestElement(t, "unparsed", "?", "", 7)), hint()}, []v159Expected{{6, 1, 6, "", nil}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					body := append(tc.body, v159TestTextRow(t, "France", "France", 3))
					v159TestRun(t, v159TestChoice(t, container, body...), "current", tc.want)
				})
			}
		})
	}
	t.Run("each new answer permits a hint", func(t *testing.T) {
		v159TestRun(t, v159TestChoice(t, "multichoice", v159TestTag(t, "answer", "Spain", 3), v159TestTag(t, "hint", "Madrid.", 3),
			v159TestTag(t, "answer", "France", 3), v159TestTag(t, "hint", "Paris.", 3)), "current", nil)
	})
}

func TestV159RootBlanks(t *testing.T) {
	for _, container := range []string{"choice", "multichoice"} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want []v159Expected
		}{
			{"inline adjacency", []model.Line{v159TestTag(t, "answer", "Spain", 3),
				v159TestLine(t, " \t", "blank", 3), v159TestTag(t, "hint", "Madrid.", 3)}, nil},
			{"inside and after block answer", []model.Line{v159TestOpen(t, "answer", "answer", 3),
				v159TestTextRow(t, "Spain", "Spain", 4), v159TestLine(t, "", "blank", 4), v159TestClose(t, 4),
				v159TestLine(t, "\t", "blank", 3), v159TestTag(t, "hint", "Madrid.", 3)}, nil},
			{"repeat inline", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 3),
				v159TestTag(t, "hint", "Madrid.", 3), v159TestLine(t, " ", "blank", 3), v159TestTag(t, "hint", "Again.", 3)},
				[]v159Expected{{8, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {6, 1, 6, 6}}}}},
			{"both related blocks contain blanks", []model.Line{v159TestOpen(t, "answer", "answer", 3),
				v159TestTextRow(t, "Spain", "Spain", 4), v159TestLine(t, "", "blank", 4), v159TestClose(t, 4),
				v159TestLine(t, "", "blank", 3), v159TestOpen(t, "hint", "hint", 3),
				v159TestTextRow(t, "Madrid.", "Madrid.", 9), v159TestLine(t, "", "blank", 9), v159TestClose(t, 9),
				v159TestLine(t, "", "blank", 3), v159TestTag(t, "hint", "Again.", 3)},
				[]v159Expected{{14, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 7, 2}, {9, 1, 12, 2}}}}},
			{"ordinary gap before block hint", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 3),
				v159TestTextRow(t, "  Я 🌏 ", "Я 🌏", 3), v159TestLine(t, "", "blank", 3),
				v159TestOpen(t, "hint", "hint", 3), v159TestTextRow(t, "Madrid.", "Madrid.", 8), v159TestClose(t, 8)},
				[]v159Expected{{8, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {6, 1, 6, 7}}}}},
			{"editor inline remains significant", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 3),
				v159TestTag(t, "editor", "Read.", 3), v159TestLine(t, "", "blank", 3), v159TestTag(t, "hint", "Madrid.", 3)},
				[]v159Expected{{8, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {6, 1, 6, 8}}}}},
			{"two options exhaust window", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 3),
				v159TestTextRow(t, "Canada", "Canada", 3), v159TestLine(t, "", "blank", 3), v159TestTextRow(t, "Italy", "Italy", 3),
				v159TestLine(t, "", "blank", 3), v159TestTag(t, "hint", "Madrid.", 3)}, []v159Expected{{10, 1, 6, "", nil}}},
			{"third hint does not search farther", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 3),
				v159TestTag(t, "hint", "Madrid.", 3), v159TestLine(t, "", "blank", 3), v159TestTag(t, "hint", "Again.", 3),
				v159TestLine(t, "", "blank", 3), v159TestTag(t, "hint", "Third.", 3)},
				[]v159Expected{{8, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {6, 1, 6, 6}}}, {10, 1, 6, "", nil}}},
			{"hint nested in answer excluded", []model.Line{v159TestOpen(t, "answer", "answer", 3),
				v159TestTextRow(t, "Spain", "Spain", 4), v159TestTag(t, "hint", "Nested.", 4), v159TestLine(t, "", "blank", 4),
				v159TestClose(t, 4), v159TestLine(t, "", "blank", 3), v159TestTag(t, "hint", "Madrid.", 3)}, nil},
		} {
			for _, root := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/root=%t", container, tc.name, root), func(t *testing.T) {
					body := append([]model.Line{}, tc.body...)
					if root {
						for i := range body {
							if body[i].LineType == "blank" {
								body[i].ParentLine = nil
							}
						}
					}
					body = append(body, v159TestTextRow(t, "France", "France", 3))
					v159TestRun(t, v159TestChoice(t, container, body...), "current", tc.want)
				})
			}
		}
	}
}

func TestV159RootBlankStops(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v159Expected
	}{
		{"foreign blank parent", []model.Line{v159TestTag(t, "answer", "Spain", 3),
			v159TestLine(t, "", "blank", 2), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3)},
			[]v159Expected{{7, 1, 6, "", nil}}},
		{"unknown root blank", []model.Line{v159TestTag(t, "answer", "Spain", 3),
			v159TestLine(t, " ", "blank", 0, v159TestElement(t, "unparsed", " ", "", 1)), v159TestTag(t, "hint", "Madrid.", 3)},
			[]v159Expected{{6, 1, 6, "", nil}}},
		{"nonempty root blank", []model.Line{v159TestTag(t, "answer", "Spain", 3),
			v159TestLine(t, "Unknown", "blank", 0, v159TestElement(t, "content", "Unknown", "Unknown", 1)), v159TestTag(t, "hint", "Madrid.", 3)},
			[]v159Expected{{6, 1, 6, "", nil}}},
		{"root content is not blank", []model.Line{v159TestTag(t, "answer", "Spain", 3),
			v159TestTextRow(t, "", "", 0), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3)},
			[]v159Expected{{7, 1, 6, "", nil}}},
		{"heading", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 0),
			v159TestHeading(t), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3)},
			[]v159Expected{{8, 1, 6, "", nil}}},
		{"structural inline tag", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 0),
			v159TestTag(t, "newpage", "", 3), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3)},
			[]v159Expected{{8, 1, 6, "", nil}}},
		{"editor block", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestLine(t, "", "blank", 0),
			v159TestOpen(t, "editor", "editor", 3), v159TestTextRow(t, "Read.", "Read.", 6), v159TestClose(t, 6),
			v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3)}, []v159Expected{{10, 1, 6, "", nil}}},
		{"unclosed answer", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4),
			v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3)}, []v159Expected{{7, 1, 6, "", nil}}},
		{"duplicate answer closing", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4),
			v159TestLine(t, "", "blank", 0), v159TestClose(t, 4), v159TestClose(t, 4), v159TestLine(t, "", "blank", 0),
			v159TestTag(t, "hint", "Madrid.", 3)}, []v159Expected{{10, 1, 6, "", nil}}},
		{"foreign interior content", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4),
			v159TestLine(t, "", "blank", 0), v159TestTextRow(t, "Foreign", "Foreign", 0), v159TestClose(t, 4),
			v159TestTag(t, "hint", "Madrid.", 3)}, []v159Expected{{9, 1, 6, "", nil}}},
		{"descendant after closing and hint", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4),
			v159TestClose(t, 4), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3),
			v159TestTextRow(t, "Late", "Late", 4)}, []v159Expected{{8, 1, 6, "", nil}}},
		{"unclosed nested block", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestOpen(t, "note", "note", 4),
			v159TestTextRow(t, "Read.", "Read.", 5), v159TestLine(t, "", "blank", 0), v159TestClose(t, 4),
			v159TestTag(t, "hint", "Madrid.", 3)}, []v159Expected{{9, 1, 6, "", nil}}},
		{"unclosed hint has no repeat detail", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestOpen(t, "hint", "hint", 3),
			v159TestTextRow(t, "Madrid.", "Madrid.", 5), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Again.", 3)},
			[]v159Expected{{8, 1, 6, "", nil}}},
		{"duplicate hint closing has no repeat detail", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestOpen(t, "hint", "hint", 3),
			v159TestTextRow(t, "Madrid.", "Madrid.", 5), v159TestLine(t, "", "blank", 0), v159TestClose(t, 5), v159TestClose(t, 5),
			v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Again.", 3)}, []v159Expected{{11, 1, 6, "", nil}}},
		{"unknown hint closing tail has no repeat detail", []model.Line{v159TestTag(t, "answer", "Spain", 3), v159TestOpen(t, "hint", "hint", 3),
			v159TestTextRow(t, "Madrid.", "Madrid.", 5), v159TestLine(t, "", "blank", 0), v159TestLine(t, "} extra", "block-end", 5,
				v159TestElement(t, "block-close", "}", "}", 1), v159TestElement(t, "unparsed", "extra", "", 3)),
			v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Again.", 3)}, []v159Expected{{10, 1, 6, "", nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v159TestRun(t, v159TestChoice(t, "choice", tc.body...), "current", tc.want)
		})
	}
	t.Run("container closing", func(t *testing.T) {
		result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, "choice", "choice", 2),
			v159TestTag(t, "answer", "Spain", 3), v159TestClose(t, 3), v159TestLine(t, "", "blank", 0), v159TestTag(t, "hint", "Madrid.", 3))
		v159TestRecovery(t, result, 7)
		v159TestRun(t, result, "current", []v159Expected{{7, 1, 6, "", nil}})
	})
	t.Run("new container opening", func(t *testing.T) {
		result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, "choice", "choice", 2),
			v159TestTag(t, "answer", "Spain", 3), v159TestClose(t, 3), v159TestLine(t, "", "blank", 0),
			v159TestOpen(t, "multichoice", "multichoice", 2), v159TestLine(t, "", "blank", 0),
			v159TestTag(t, "hint", "Madrid.", 7), v159TestClose(t, 7))
		v159TestRun(t, result, "current", []v159Expected{{9, 1, 6, "", nil}})
	})
}

func TestV159IndependentBlocksLineReads(t *testing.T) {
	for _, count := range []int{8, 32, 128} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var body []model.Line
			for i := 0; i < count; i++ {
				body = append(body, v159TestOpen(t, "answer", "answer", 3),
					v159TestTextRow(t, "Spain", "Spain", 4+4*i), v159TestClose(t, 4+4*i), v159TestTag(t, "hint", "Madrid.", 3))
			}
			v159TestRun(t, v159TestChoice(t, "multichoice", body...), "current", nil)
		})
	}
}

func TestV159Blocks(t *testing.T) {
	for _, container := range []string{"choice", "multichoice"} {
		t.Run(container, func(t *testing.T) {
			answer := func() model.Line { return v159TestTag(t, "answer", "Spain", 3) }
			hint := func() model.Line { return v159TestTag(t, "hint", "Madrid.", 3) }
			for _, tc := range []struct {
				name string
				body []model.Line
				want []v159Expected
			}{
				{"block answer", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4), v159TestLine(t, "", "blank", 4), v159TestClose(t, 4), hint()}, nil},
				{"block hint", []model.Line{answer(), v159TestOpen(t, "hint", "hint", 3), v159TestTextRow(t, "Madrid.", "Madrid.", 5), v159TestClose(t, 5)}, nil},
				{"repeat after block hint", []model.Line{answer(), v159TestOpen(t, "hint", "hint", 3), v159TestTextRow(t, "Madrid.", "Madrid.", 5), v159TestClose(t, 5), hint()}, []v159Expected{{8, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {5, 1, 7, 2}}}}},
				{"repeat block hint", []model.Line{answer(), hint(), v159TestOpen(t, "hint", "hint", 3), v159TestTextRow(t, "Europe.", "Europe.", 6), v159TestClose(t, 6)}, []v159Expected{{6, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 6}}}}},
				{"gap after block answer", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4), v159TestLine(t, "", "blank", 4), v159TestClose(t, 4), v159TestTextRow(t, "Canada", "Canada", 3), hint()}, []v159Expected{{9, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 7, 2}, {8, 1, 8, 7}}}}},
				{"orphan block hint", []model.Line{v159TestOpen(t, "hint", "hint", 3), v159TestTextRow(t, "Madrid.", "Madrid.", 4), v159TestClose(t, 4), v159TestTextRow(t, "Spain", "Spain", 3)}, []v159Expected{{4, 1, 6, "", nil}}},
				{"hint inside answer excluded", []model.Line{v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4), v159TestTag(t, "hint", "Madrid.", 4), v159TestClose(t, 4), hint()}, nil},
				{"answer inside other block cannot own hint", []model.Line{v159TestOpen(t, "note", "note", 3), v159TestTag(t, "answer", "Spain", 4), v159TestClose(t, 4), hint()}, []v159Expected{{7, 1, 6, "", nil}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					v159TestRun(t, v159TestChoice(t, container, append(tc.body, v159TestTextRow(t, "France", "France", 3))...), "current", tc.want)
				})
			}
			for _, other := range []string{"editor", "note"} {
				t.Run(other+" block stops search", func(t *testing.T) {
					v159TestRun(t, v159TestChoice(t, container, answer(), v159TestOpen(t, other, other, 3),
						v159TestTextRow(t, "Read.", "Read.", 5), v159TestClose(t, 5), hint(), v159TestTextRow(t, "France", "France", 3)), "current", []v159Expected{{8, 1, 6, "", nil}})
				})
			}
		})
	}
	t.Run("both related constructs are blocks", func(t *testing.T) {
		v159TestRun(t, v159TestChoice(t, "multichoice", v159TestOpen(t, "answer", "answer", 3),
			v159TestTextRow(t, "Spain", "Spain", 4), v159TestTextRow(t, "France", "France", 4), v159TestClose(t, 4),
			v159TestOpen(t, "hint", "hint", 3), v159TestTextRow(t, "Europe.", "Europe.", 8), v159TestClose(t, 8), v159TestTag(t, "hint", "Again.", 3)), "current",
			[]v159Expected{{11, 1, 6, "Повторная подсказка для одного @answer.", [][4]int{{4, 1, 7, 2}, {8, 1, 10, 2}}}})
	})
}

// v159TestTag задаёт однострочное объявление и его текстовое значение явно.
func v159TestTag(t *testing.T, name, content string, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v159TestElement(t, "tag", raw, name, 1)}
	if content != "" {
		elements = append(elements, v159TestElement(t, "content", content, content, len(raw)+2))
		raw += " " + content
	}
	return v159TestLine(t, raw, "tag", parent, elements...)
}

// v159TestTextRow сохраняет исходное и семантическое значения без разбора.
func v159TestTextRow(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v159TestLine(t, raw, "content", parent, v159TestElement(t, "content", raw, value, 1))
}

// v159TestRecovery добавляет исходную ошибку восстановления структуры в согласованную модель.
func v159TestRecovery(t *testing.T, result *model.Result, number int) {
	t.Helper()
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: fmt.Sprintf("recovery-%d", number), Source: "parser", DiagnosticCode: "P002",
		SeverityLevel: "error", DiagnosticScope: "line", Message: "Неоднозначная граница блока.",
		Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}}, RelatedLocations: []model.Location{}})
	result.Document.HasErrors = true
	result.Lines[number-1].HasErrors = true
}

// v159TestInvalid задаёт восстановленную повреждённую строку с unparsed.
func v159TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v159TestLine(t, raw, "invalid", parent, v159TestElement(t, "unparsed", raw, "", 1))
}

// v159TestOpen создаёт открытие блока с исходным и каноническим именем.
func v159TestOpen(t *testing.T, rawName, name string, parent int) model.Line {
	t.Helper()
	return v159TestLine(t, "@"+rawName+" {", "block-start", parent,
		v159TestElement(t, "tag", "@"+rawName, name, 1), v159TestElement(t, "block-open", "{", "{", len(rawName)+3))
}

// v159TestClose создаёт структурное закрытие указанного блока.
func v159TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v159TestLine(t, "}", "block-end", parent, v159TestElement(t, "block-close", "}", "}", 1))
}

// v159TestChoice добавляет контекст документа и границы выбора; тело начинается на строке 4.
func v159TestChoice(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, name, name, 2)}, body...)
	return v159TestDocument(t, append(rows, v159TestClose(t, 3))...)
}

// v159TestElement создаёт элемент модели; диапазоны ожиданий задаются отдельно.
func v159TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v159TestLine задаёт тип и родителя без повторного разбора исходного текста.
func v159TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v159TestVersion задаёт технический тег версии для контекста примеров.
func v159TestVersion(t *testing.T) model.Line {
	t.Helper()
	return v159TestLine(t, "@dsl-version 1.2", "tag", 0,
		v159TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v159TestElement(t, "version", "1.2", "1.2", 14))
}

// v159TestHeading добавляет обязательный заголовок документа перед фрагментом.
func v159TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v159TestLine(t, "# Lesson", "heading", 0,
		v159TestElement(t, "heading-level", "#", "1", 1), v159TestElement(t, "title", "Lesson", "Lesson", 3))
}

// v159TestDocument оформляет заданную модель и обязательные исходные ссылки unparsed.
// Причины повреждения не восстанавливаются: классификация передана тестом явно.
func v159TestDocument(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	version, count := "1.2", len(rows)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines: append([]model.Line{}, rows...), Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			if *line.ParentLine < 1 || *line.ParentLine >= line.Number {
				t.Fatal("invalid fixture parent")
			}
			line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
		}
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", line.Number, j)
			element.ErrorIDs = []string{id}
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неразобранный фрагмент.",
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}}, RelatedLocations: []model.Location{}})
			line.HasErrors, result.Document.HasErrors = true, true
		}
	}
	return result
}

// v159TestRegistry создаёт реестр из полного штатного каталога.
func v159TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v159TestSnapshot сохраняет модель для проверки неизменности истории и исходных данных.
func v159TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestV159ContextsAndBoundaries(t *testing.T) {
	t.Run("empty", func(t *testing.T) { v159TestRun(t, v159TestDocument(t), "current", nil) })
	for _, container := range []string{"choice", "multichoice"} {
		t.Run("nested task/"+container, func(t *testing.T) {
			result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t),
				v159TestLine(t, "@task t", "tag", 2, v159TestElement(t, "tag", "@task", "task", 1), v159TestElement(t, "identifier", "t", "t", 7)),
				v159TestOpen(t, container, container, 3), v159TestTag(t, "answer", "Spain", 4),
				v159TestTextRow(t, "Canada", "Canada", 4), v159TestTag(t, "hint", "Madrid.", 4), v159TestClose(t, 4), v159TestTag(t, "endtask", "", 3))
			v159TestRun(t, result, "current", []v159Expected{{7, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{5, 1, 5, 8}, {6, 1, 6, 7}}}})
		})
		t.Run("new container/"+container, func(t *testing.T) {
			result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, container, container, 2),
				v159TestTag(t, "answer", "Spain", 3), v159TestTextRow(t, "France", "France", 3), v159TestClose(t, 3),
				v159TestOpen(t, container, container, 2), v159TestTag(t, "hint", "Madrid.", 7), v159TestTextRow(t, "Canada", "Canada", 7),
				v159TestTextRow(t, "Italy", "Italy", 7), v159TestClose(t, 7))
			v159TestRun(t, result, "current", []v159Expected{{8, 1, 6, "", nil}})
		})
	}
	t.Run("case and indentation", func(t *testing.T) {
		result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, "CHOICE", "choice", 2),
			v159TestLine(t, "\t@ANSWER Spain", "tag", 3, v159TestElement(t, "tag", "@ANSWER", "answer", 2), v159TestElement(t, "content", "Spain", "Spain", 10)),
			v159TestTag(t, "hint", "Madrid.", 3),
			v159TestLine(t, "  @HINT Europe.", "tag", 3, v159TestElement(t, "tag", "@HINT", "hint", 3), v159TestElement(t, "content", "Europe.", "Europe.", 9)),
			v159TestTextRow(t, "France", "France", 3), v159TestClose(t, 3))
		v159TestRun(t, result, "current", []v159Expected{{6, 3, 8, "Повторная подсказка для одного @answer.", [][4]int{{4, 2, 4, 9}, {5, 1, 5, 6}}}})
	})
	t.Run("root and task hints excluded", func(t *testing.T) {
		v159TestRun(t, v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestTag(t, "hint", "Read.", 2),
			v159TestLine(t, "@task t", "tag", 2, v159TestElement(t, "tag", "@task", "task", 1), v159TestElement(t, "identifier", "t", "t", 7)),
			v159TestTag(t, "hint", "Read.", 4), v159TestTag(t, "endtask", "", 4)), "current", nil)
	})
	for _, owner := range []string{"note", "editor", "multifill"} {
		t.Run("excluded context/"+owner, func(t *testing.T) {
			var body []model.Line
			if owner == "note" {
				body = []model.Line{v159TestTag(t, "hint", "Read.", 3)}
			} else {
				// Это литеральные строки модели, а не повторно разобранные объявления.
				raw := "@hint Read."
				if owner == "multifill" {
					raw = `\@hint Read.`
				}
				body = []model.Line{v159TestTextRow(t, raw, "@hint Read.", 3)}
			}
			rows := append([]model.Line{v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, owner, owner, 2)}, body...)
			v159TestRun(t, v159TestDocument(t, append(rows, v159TestClose(t, 3))...), "current", nil)
		})
	}
	t.Run("EOF at hint", func(t *testing.T) {
		result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, "choice", "choice", 2), v159TestTag(t, "hint", "Read.", 3))
		v159TestRecovery(t, result, 3)
		result.Lines[3].LineEnding = ""
		v159TestRun(t, result, "current", []v159Expected{{4, 1, 6, "", nil}})
	})
	t.Run("heading boundary in recovered block", func(t *testing.T) {
		result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, "choice", "choice", 2),
			v159TestTag(t, "answer", "Spain", 3), v159TestHeading(t), v159TestTag(t, "hint", "Madrid.", 3), v159TestClose(t, 3))
		// Заголовок сохраняет корневой уровень модели; связь hint с восстановленным
		// контейнером не даёт права пересекать встретившийся заголовок.
		v159TestRecovery(t, result, 5)
		v159TestRun(t, result, "current", []v159Expected{{6, 1, 6, "", nil}})
	})
}

func TestV159UncertainBlockBoundaries(t *testing.T) {
	t.Run("duplicate answer closing", func(t *testing.T) {
		result := v159TestChoice(t, "multichoice", v159TestOpen(t, "answer", "answer", 3),
			v159TestTextRow(t, "Spain", "Spain", 4), v159TestClose(t, 4), v159TestClose(t, 4), v159TestTag(t, "hint", "Madrid.", 3))
		v159TestRecovery(t, result, 7)
		v159TestRun(t, result, "current", []v159Expected{{8, 1, 6, "", nil}})
	})
	t.Run("unclosed answer before immediate hint", func(t *testing.T) {
		result := v159TestChoice(t, "choice", v159TestOpen(t, "answer", "answer", 3),
			v159TestTextRow(t, "Spain", "Spain", 4), v159TestTag(t, "hint", "Madrid.", 3))
		v159TestRecovery(t, result, 4)
		v159TestRun(t, result, "current", []v159Expected{{6, 1, 6, "", nil}})
	})
	t.Run("unparsed closing tail", func(t *testing.T) {
		result := v159TestChoice(t, "choice", v159TestOpen(t, "answer", "answer", 3), v159TestTextRow(t, "Spain", "Spain", 4),
			v159TestLine(t, "} extra", "block-end", 4, v159TestElement(t, "block-close", "}", "}", 1), v159TestElement(t, "unparsed", "extra", "", 3)),
			v159TestTag(t, "hint", "Madrid.", 3))
		v159TestRun(t, result, "current", []v159Expected{{7, 1, 6, "", nil}})
	})
	t.Run("container closing cannot stand for answer", func(t *testing.T) {
		result := v159TestDocument(t, v159TestVersion(t), v159TestHeading(t), v159TestOpen(t, "choice", "choice", 2),
			v159TestTag(t, "answer", "Spain", 3), v159TestClose(t, 3), v159TestTag(t, "hint", "Madrid.", 3))
		v159TestRecovery(t, result, 6)
		v159TestRun(t, result, "current", []v159Expected{{6, 1, 6, "", nil}})
	})
}

func TestV159EndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", eol), func(t *testing.T) {
			result := v159TestChoice(t, "choice", v159TestTag(t, "answer", "Spain", 3),
				v159TestTextRow(t, "Canada", "Canada", 3), v159TestTag(t, "hint", "Madrid.", 3))
			for i := range result.Lines {
				if eol != "" {
					result.Lines[i].LineEnding = eol
				}
			}
			result.Lines[len(result.Lines)-1].LineEnding = eol
			want := []v159Expected{{6, 1, 6, "Промежуточная конструкция разрывает соседство.", [][4]int{{4, 1, 4, 8}, {5, 1, 5, 7}}}}
			v159TestRun(t, result, "first", want)
			v159TestRun(t, v159TestChoice(t, "choice", v159TestTag(t, "answer", "Spain", 3),
				v159TestTag(t, "hint", "Madrid.", 3), v159TestTextRow(t, "France", "France", 3)), "other", nil)
			v159TestRun(t, result, "second", want)
		})
	}
}

// v159TestLineReads считает обращения к исходному интерфейсу без изменения строк.
type v159TestLineReads struct {
	diagnostics.Lines
	reads int
}

func (lines *v159TestLineReads) Line(number int) (diagnostics.Line, bool) {
	lines.reads++
	return lines.Lines.Line(number)
}

// v159TestRun проверяет V159 через полный штатный каталог и конвейер.
// Ожидания включают основные и связанные диапазоны, конкретизацию и метаданные.
func v159TestRun(t *testing.T, result *model.Result, processing string, want []v159Expected) {
	t.Helper()
	before := v159TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v159TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(checks) != 198 {
		t.Fatalf("catalogue size = %d, want 198", len(checks))
	}
	if v159TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	var reads int
	for i := range checks {
		if checks[i].Code == "V159" {
			check := checks[i].Check
			checks[i].Check = func(lines diagnostics.Lines, findings diagnostics.Findings) error {
				observed := &v159TestLineReads{Lines: lines}
				err := check(observed, findings)
				reads = observed.reads
				return err
			}
		}
	}
	result.Processing = append(result.Processing, model.Processing{ID: processing, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processing); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Запас на линейные проходы и две предыдущие конструкции каждой подсказки;
	// повторный обход хвоста для каждого блока не укладывается в этот предел.
	if limit := 12*len(result.Lines) + 16; reads > limit {
		t.Errorf("V159 Line calls = %d, want at most %d for %d lines", reads, limit, len(result.Lines))
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V159" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V159 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		message := "Подсказка @hint не следует непосредственно за @answer."
		if expected.detail != "" {
			message += "\nконкретизация: " + expected.detail
		}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processing ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != message ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V159 occurrence = %+v, want %+v", entry, expected)
		}
		for j, r := range expected.related {
			location := model.Location{Start: model.Position{Line: r[0], Column: r[1]}, End: model.Position{Line: r[2], Column: r[3]}}
			if entry.RelatedLocations[j] != location {
				t.Fatalf("related[%d] = %+v, want %+v", j, entry.RelatedLocations[j], location)
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line scope created element link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("output integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v159TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Из сравнения исключаются только производные флаги и новые записи конвейера.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("old element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v159TestSnapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

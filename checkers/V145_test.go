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

// v145Expected задаёт основной диапазон, вопрос и препятствие именно в этом порядке.
// Каждый диапазон содержит строку, начальную и исключающую конечную колонку с 1.
type v145Expected [3][3]int

func TestV145Registration(t *testing.T) {
	definitions, err := v145Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V145" {
			continue
		}
		count++
		if definition.Message != "Между @question и @answer расположено другое содержимое." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V145 definitions = %d, want 1", count)
	}
}

func TestV145ReferenceAndExample(t *testing.T) {
	// DSL 1.1–1.3, 1.30 требуют версию и заголовок. Только этот префикс
	// добавлен к дословным эталону и примеру; их строки начинаются с номера 3.
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v145Expected
	}{
		{"reference", []model.Line{v145Question(t, 2, 1), v145Answer(t, 2, 1)}, nil},
		{"example", []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1)},
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v145Run(t, v145Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV145TextAndTagObstacles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		middle model.Line
		want   []v145Expected
	}{
		{"ordinary text", v145Text(t, "Read carefully.", "Read carefully.", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 16}}}},
		{"raw Unicode and edges", v145Text(t, " \tЖ🙂 \t", "Ж🙂", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 7}}}},
		{"NBSP", v145Text(t, "\u00a0", "\u00a0", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 2}}}},
		{"em space", v145Text(t, "\u2003", "\u2003", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 2}}}},
		{"escaped editor", v145Text(t, `\@editor`, "@editor", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 9}}}},
		{"editor in text", v145Text(t, "See @editor", "See @editor", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 12}}}},
		{"punctuation", v145Text(t, ".", ".", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 2}}}},
		{"instr", v145Tag(t, "instr", "content", "Read.", 2, 1),
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 7}}}},
		{"empty line", v145Line(t, "", "blank", 2, 1), nil},
		{"spaces and tabs", v145Line(t, " \t \t", "blank", 2, 1), nil},
		{"editor", v145Tag(t, "editor", "content", "Comment", 2, 1), nil},
		{"next question", v145Tag(t, "question", "content", "Ready?", 2, 1), nil},
		{"previous answer", v145Answer(t, 2, 1), nil},
		{"single line multifill", v145Tag(t, "multifill", "content", "Write freely.", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v145Run(t, v145Document(t, v145Question(t, 2, 1), tc.middle, v145Answer(t, 2, 1)), "current", tc.want)
		})
	}
	t.Run("indented mixed case tags", func(t *testing.T) {
		q := v145Line(t, "\t@QUESTION Where does Tom live?  ", "tag", 2, 1,
			v145TestElement(t, "tag", "@QUESTION", "question", 2, 11),
			v145TestElement(t, "content", "Where does Tom live?", "Where does Tom live?", 12, 32))
		n := v145Line(t, "  @NOTE Read carefully.\t", "tag", 2, 1,
			v145TestElement(t, "tag", "@NOTE", "note", 3, 8),
			v145TestElement(t, "content", "Read carefully.", "Read carefully.", 9, 24))
		a := v145Line(t, " \t@ANSWER London", "tag", 2, 1,
			v145TestElement(t, "tag", "@ANSWER", "answer", 3, 10),
			v145TestElement(t, "content", "London", "London", 11, 17))
		v145Run(t, v145Document(t, q, n, a), "current", []v145Expected{{{5, 3, 10}, {3, 2, 11}, {4, 3, 8}}})
	})
}

func TestV145SearchLimitAndMultipleAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v145Expected
	}{
		{"no content", nil, nil},
		{"first answer", []model.Line{v145Answer(t, 2, 1)}, nil},
		{"no question", []model.Line{v145Note(t, 2, 1), v145Answer(t, 2, 1)}, nil},
		{"two obstacles", []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1), v145Text(t, "Text", "Text", 2, 1), v145Answer(t, 2, 1)}, nil},
		{"second is text", []model.Line{v145Question(t, 2, 1), v145Text(t, "Text", "Text", 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1)}, nil},
		{"previous answer stops second lookup", []model.Line{v145Question(t, 2, 1), v145Answer(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1)}, nil},
		{"sequence of answers", []model.Line{v145Question(t, 2, 1), v145Answer(t, 2, 1), v145Answer(t, 2, 1)}, nil},
		{"only first answer reports", []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1), v145Answer(t, 2, 1)},
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}}},
		{"two independent violations", []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1),
			v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1)},
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}, {{8, 1, 8}, {6, 1, 10}, {7, 1, 6}}}},
		{"block answer", []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1), v145Open(t, "answer", 2, 1),
			v145Text(t, "London", "London", 5, 2), v145Close(t, 5, 2)},
			[]v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}}},
		{"blanks do not consume budget", []model.Line{v145Question(t, 2, 1), v145Line(t, "", "blank", 2, 1),
			v145Note(t, 2, 1), v145Line(t, " \t", "blank", 2, 1), v145Answer(t, 2, 1)},
			[]v145Expected{{{7, 1, 8}, {3, 1, 10}, {5, 1, 6}}}},
		{"editor before and after obstacle", []model.Line{v145Question(t, 2, 1), v145Tag(t, "editor", "content", "One", 2, 1),
			v145Note(t, 2, 1), v145Tag(t, "EDITOR", "content", "Two", 2, 1), v145Answer(t, 2, 1)},
			[]v145Expected{{{7, 1, 8}, {3, 1, 10}, {5, 1, 6}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v145Run(t, v145Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV145EditorBlocksAndOtherBlocks(t *testing.T) {
	t.Run("editor skipped before obstacle", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Open(t, "editor", 2, 1),
			v145Text(t, "@answer Literal", "@answer Literal", 4, 2), v145Close(t, 4, 2),
			v145Note(t, 2, 1), v145Answer(t, 2, 1)), "current", []v145Expected{{{8, 1, 8}, {3, 1, 10}, {7, 1, 6}}})
	})
	t.Run("editor skipped after obstacle", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Note(t, 2, 1), v145Open(t, "EDITOR", 2, 1),
			v145Text(t, "@variant Literal", "@variant Literal", 5, 2), v145Close(t, 5, 2),
			v145Answer(t, 2, 1)), "current", []v145Expected{{{8, 1, 8}, {3, 1, 10}, {4, 1, 6}}})
	})
	t.Run("consecutive editor blocks preserve adjacency", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Open(t, "editor", 2, 1),
			v145Text(t, "@task literal", "@task literal", 4, 2), v145Close(t, 4, 2), v145Open(t, "editor", 2, 1),
			v145Text(t, "### Literal", "### Literal", 7, 2), v145Close(t, 7, 2), v145Answer(t, 2, 1)), "current", nil)
	})
	for _, tag := range []string{"note", "text", "multifill"} {
		for _, after := range []bool{false, true} {
			t.Run(tag+" obstacle after block="+fmt.Sprint(after), func(t *testing.T) {
				body := []model.Line{v145Question(t, 2, 1), v145Open(t, tag, 2, 1),
					v145Text(t, "Write freely.", "Write freely.", 4, 2), v145Close(t, 4, 2)}
				if after {
					body = append(body, v145Note(t, 2, 1))
				}
				body = append(body, v145Answer(t, 2, 1))
				v145Run(t, v145Document(t, body...), "current", nil)
			})
		}
	}
	t.Run("question inside preceding block is not reused", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Open(t, "text", 2, 1), v145Question(t, 3, 2),
			v145Close(t, 3, 2), v145Note(t, 2, 1), v145Answer(t, 2, 1)), "current", nil)
	})
}

func TestV145TaskAndStageContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v145Expected
	}{
		{"task", []model.Line{v145Tag(t, "task", "identifier", "t", 2, 1), v145Question(t, 3, 2),
			v145Note(t, 3, 2), v145Answer(t, 3, 2), v145Tag(t, "endtask", "", "", 3, 2)},
			[]v145Expected{{{6, 1, 8}, {4, 1, 10}, {5, 1, 6}}}},
		{"stage", []model.Line{v145Tag(t, "task", "identifier", "t", 2, 1), v145Tag(t, "step", "title", "Part", 3, 2),
			v145Question(t, 4, 3), v145Note(t, 4, 3), v145Answer(t, 4, 3), v145Tag(t, "endtask", "", "", 3, 2)},
			[]v145Expected{{{7, 1, 8}, {5, 1, 10}, {6, 1, 6}}}},
		{"editor in task stage", []model.Line{v145Tag(t, "task", "identifier", "t", 2, 1),
			v145Tag(t, "step", "title", "Part", 3, 2), v145Question(t, 4, 3), v145Open(t, "editor", 4, 3),
			v145Text(t, "@answer Literal", "@answer Literal", 6, 4), v145Close(t, 6, 4),
			v145Note(t, 4, 3), v145Answer(t, 4, 3), v145Tag(t, "endtask", "", "", 3, 2)},
			[]v145Expected{{{10, 1, 8}, {5, 1, 10}, {9, 1, 6}}}},
		{"explicit document step", []model.Line{v145Heading(t, "###", "3", "Practice"),
			v145Question(t, 3, 1), v145Note(t, 3, 1), v145Answer(t, 3, 1)},
			[]v145Expected{{{6, 1, 8}, {4, 1, 10}, {5, 1, 6}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v145Run(t, v145Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV145StructuralBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"heading", []model.Line{v145Question(t, 2, 1), v145Heading(t, "###", "3", "Next"),
			v145Note(t, 4, 1), v145Answer(t, 4, 1)}},
		{"task", []model.Line{v145Question(t, 2, 1), v145Tag(t, "task", "identifier", "t", 2, 1),
			v145Note(t, 4, 2), v145Answer(t, 4, 2), v145Tag(t, "endtask", "", "", 4, 2)}},
		{"endtask", []model.Line{v145Tag(t, "task", "identifier", "t", 2, 1), v145Question(t, 3, 2),
			v145Tag(t, "endtask", "", "", 3, 2), v145Note(t, 2, 1), v145Answer(t, 2, 1)}},
		{"header", []model.Line{v145Question(t, 2, 1), v145Tag(t, "header", "title", "Next", 2, 1),
			v145Note(t, 2, 1), v145Answer(t, 2, 1)}},
		{"step", []model.Line{v145Tag(t, "task", "identifier", "t", 2, 1), v145Question(t, 3, 2),
			v145Tag(t, "step", "title", "Next", 3, 2), v145Note(t, 5, 3), v145Answer(t, 5, 3),
			v145Tag(t, "endtask", "", "", 3, 2)}},
		{"newpage", []model.Line{v145Question(t, 2, 1), v145Tag(t, "newpage", "", "", 0, 0),
			v145Note(t, 4, 1), v145Answer(t, 4, 1)}},
		{"header itself is not an obstacle", []model.Line{v145Question(t, 2, 1),
			v145Tag(t, "HEADER", "title", "Next", 2, 1), v145Answer(t, 2, 1)}},
		{"task start after obstacle", []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1),
			v145Tag(t, "task", "identifier", "t", 2, 1), v145Answer(t, 5, 2), v145Tag(t, "endtask", "", "", 5, 2)}},
		{"same depth different tasks", []model.Line{v145Tag(t, "task", "identifier", "a", 2, 1),
			v145Question(t, 3, 2), v145Tag(t, "task", "identifier", "b", 2, 1),
			v145Note(t, 5, 2), v145Answer(t, 5, 2), v145Tag(t, "endtask", "", "", 5, 2)}},
		{"next stage", []model.Line{v145Tag(t, "task", "identifier", "t", 2, 1),
			v145Tag(t, "step", "title", "First", 3, 2), v145Question(t, 4, 3),
			v145Tag(t, "step", "title", "Second", 3, 2), v145Note(t, 6, 3), v145Answer(t, 6, 3),
			v145Tag(t, "endtask", "", "", 3, 2)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v145Run(t, v145Document(t, tc.body...), "current", nil)
		})
	}
}

func TestV145Variants(t *testing.T) {
	t.Run("literal variant does not start branch", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Open(t, "editor", 3, 2),
			v145Text(t, "@variant Literal", "@variant Literal", 4, 3), v145Close(t, 4, 3),
			v145Question(t, 3, 2), v145Note(t, 3, 2), v145Answer(t, 3, 2),
			v145Tag(t, "variant", "name", "A", 3, 2), v145Text(t, "Read.", "Read.", 10, 3),
			v145Tag(t, "variant", "name", "B", 3, 2), v145Text(t, "Write.", "Write.", 12, 3),
			v145Close(t, 3, 2)), "current", nil)
	})
	t.Run("before first variant is outside branch context", func(t *testing.T) {
		// DSL 4.9.8: материал до первого variant не относится ни к одной ветви.
		// Ошибка расположения остаётся другим диагностикам; V145 здесь исключена.
		v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Question(t, 3, 2),
			v145Note(t, 3, 2), v145Answer(t, 3, 2), v145Tag(t, "variant", "name", "A", 3, 2),
			v145Text(t, "Read.", "Read.", 7, 3), v145Tag(t, "variant", "name", "B", 3, 2),
			v145Text(t, "Write.", "Write.", 9, 3), v145Close(t, 3, 2)), "current", nil)
	})
	// Обе формы связей родителей используются существующими моделями проекта:
	// содержимое ветви ссылается на variant либо на общий фигурный variants.
	for _, common := range []bool{false, true} {
		t.Run("branch context common parent="+fmt.Sprint(common), func(t *testing.T) {
			parent, depth := 4, 3
			if common {
				parent, depth = 3, 2
			}
			v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Tag(t, "variant", "name", "A", 3, 2),
				v145Question(t, parent, depth), v145Note(t, parent, depth), v145Answer(t, parent, depth),
				v145Tag(t, "variant", "name", "B", 3, 2), v145Text(t, "Read.", "Read.", 8, 3),
				v145Close(t, 3, 2)), "current", []v145Expected{{{7, 1, 8}, {5, 1, 10}, {6, 1, 6}}})
		})
		t.Run("branch boundary common parent="+fmt.Sprint(common), func(t *testing.T) {
			parentA, parentB, depth := 4, 6, 3
			if common {
				parentA, parentB, depth = 3, 3, 2
			}
			v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Tag(t, "variant", "name", "A", 3, 2),
				v145Question(t, parentA, depth), v145Tag(t, "VARIANT", "name", "B", 3, 2),
				v145Note(t, parentB, depth), v145Answer(t, parentB, depth), v145Close(t, 3, 2)), "current", nil)
		})
		t.Run("variant is not an obstacle common parent="+fmt.Sprint(common), func(t *testing.T) {
			parentA, parentB, depth := 4, 6, 3
			if common {
				parentA, parentB, depth = 3, 3, 2
			}
			v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Tag(t, "variant", "name", "A", 3, 2),
				v145Question(t, parentA, depth), v145Tag(t, "variant", "name", "B", 3, 2),
				v145Answer(t, parentB, depth), v145Close(t, 3, 2)), "current", nil)
		})
	}
	t.Run("editor in branch", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Tag(t, "variant", "name", "A", 3, 2),
			v145Question(t, 4, 3), v145Open(t, "editor", 4, 3), v145Text(t, "@variant Literal", "@variant Literal", 6, 4),
			v145Close(t, 6, 4), v145Note(t, 4, 3), v145Answer(t, 4, 3),
			v145Tag(t, "variant", "name", "B", 3, 2), v145Text(t, "Read.", "Read.", 11, 3), v145Close(t, 3, 2)),
			"current", []v145Expected{{{10, 1, 8}, {5, 1, 10}, {9, 1, 6}}})
	})
	t.Run("task stage within branch", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Tag(t, "variant", "name", "A", 3, 2),
			v145Tag(t, "task", "identifier", "t", 4, 3), v145Tag(t, "step", "title", "Part", 5, 4),
			v145Question(t, 6, 5), v145Note(t, 6, 5), v145Answer(t, 6, 5), v145Tag(t, "endtask", "", "", 5, 4),
			v145Tag(t, "variant", "name", "B", 3, 2), v145Text(t, "Read.", "Read.", 11, 3), v145Close(t, 3, 2)),
			"current", []v145Expected{{{9, 1, 8}, {7, 1, 10}, {8, 1, 6}}})
	})
	t.Run("variants opening separates contexts", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Open(t, "variants", 2, 1),
			v145Tag(t, "variant", "name", "A", 4, 2), v145Note(t, 5, 3), v145Answer(t, 5, 3),
			v145Tag(t, "variant", "name", "B", 4, 2), v145Text(t, "Read.", "Read.", 8, 3), v145Close(t, 4, 2)), "current", nil)
	})
	t.Run("variants closing separates contexts", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Open(t, "variants", 2, 1), v145Tag(t, "variant", "name", "A", 3, 2),
			v145Text(t, "Read.", "Read.", 4, 3), v145Tag(t, "variant", "name", "B", 3, 2), v145Question(t, 6, 3),
			v145Close(t, 3, 2), v145Note(t, 2, 1), v145Answer(t, 2, 1)), "current", nil)
	})
}

func TestV145LongBranches(t *testing.T) {
	for _, common := range []bool{false, true} {
		t.Run("common parent="+fmt.Sprint(common), func(t *testing.T) {
			// В каждой длинной ветви проверяются начало и конец; второй variants
			// снова содержит тройку до первого variant, которая не даёт V145.
			v145Run(t, v145LongBranches(t, 1000, common), "current", []v145Expected{
				{{13, 1, 8}, {11, 1, 10}, {12, 1, 6}},
				{{2016, 1, 8}, {2014, 1, 10}, {2015, 1, 6}},
				{{2023, 1, 8}, {2021, 1, 10}, {2022, 1, 6}},
				{{4026, 1, 8}, {4024, 1, 10}, {4025, 1, 6}},
				{{4035, 1, 8}, {4033, 1, 10}, {4034, 1, 6}},
			})
		})
	}
}

// BenchmarkV145LongBranches измеряет Prepare/Run полного штатного каталога
// на растущих ветвях; построение модели не входит в измеряемый участок.
func BenchmarkV145LongBranches(b *testing.B) {
	for _, pairs := range []int{250, 1000, 4000} {
		b.Run(fmt.Sprintf("pairs=%d", pairs), func(b *testing.B) {
			input := v145LongBranches(b, pairs, false)
			registry := v145Registry(b)
			b.ReportAllocs()
			for b.Loop() {
				result := *input
				checks, err := validator.Prepare(&result, registry)
				if err != nil {
					b.Fatal(err)
				}
				result.Processing = append(result.Processing, model.Processing{ID: "current", Tool: "dsl-validator"})
				if err := validator.Run(&result, checks, "current"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// v145LongBranches задаёт две длинные ветви и независимый второй variants.
// pairs — число корректных пар question/answer между нарушениями каждой ветви.
func v145LongBranches(t testing.TB, pairs int, common bool) *model.Result {
	t.Helper()
	body := []model.Line{v145Open(t, "variants", 2, 1), v145Question(t, 3, 2),
		v145Note(t, 3, 2), v145Answer(t, 3, 2), v145Open(t, "editor", 3, 2),
		v145Text(t, "@variant Literal", "@variant Literal", 7, 3), v145Close(t, 7, 3)}
	for _, name := range []string{"A", "B"} {
		parent, depth := len(body)+3, 3
		body = append(body, v145Tag(t, "variant", "name", name, 3, 2))
		if common {
			parent, depth = 3, 2
		}
		if name == "B" {
			// Вопрос в конце A не связывается с препятствием и ответом в B.
			body = append(body, v145Note(t, parent, depth), v145Answer(t, parent, depth))
		}
		body = append(body, v145Question(t, parent, depth), v145Note(t, parent, depth), v145Answer(t, parent, depth))
		for range pairs {
			body = append(body, v145Question(t, parent, depth), v145Answer(t, parent, depth))
		}
		body = append(body, v145Question(t, parent, depth), v145Note(t, parent, depth), v145Answer(t, parent, depth))
		if name == "A" {
			body = append(body, v145Question(t, parent, depth))
		}
	}
	body = append(body, v145Close(t, 3, 2))
	block := len(body) + 3
	body = append(body, v145Open(t, "variants", 2, 1), v145Question(t, block, 2),
		v145Note(t, block, 2), v145Answer(t, block, 2), v145Tag(t, "variant", "name", "C", block, 2))
	parent, depth := block+4, 3
	if common {
		parent, depth = block, 2
	}
	body = append(body, v145Question(t, parent, depth), v145Note(t, parent, depth), v145Answer(t, parent, depth),
		v145Tag(t, "variant", "name", "D", block, 2))
	parent = len(body) + 2
	if common {
		parent = block
	}
	body = append(body, v145Text(t, "Read.", "Read.", parent, depth), v145Close(t, block, 2))
	return v145Document(t, body...)
}

func TestV145ExcludedContainersAndLiteralTags(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			v145Run(t, v145Document(t, v145Open(t, name, 2, 1),
				v145Text(t, "@question Where does Tom live?", "@question Where does Tom live?", 3, 2),
				v145Text(t, "@note Read carefully.", "@note Read carefully.", 3, 2),
				v145Text(t, "@answer London", "@answer London", 3, 2), v145Close(t, 3, 2)), "current", nil)
		})
	}
	for _, name := range []string{"text", "note", "answer"} {
		t.Run("recognized declarations inside "+name, func(t *testing.T) {
			// По DSL 1.24 запрещённые вложенные объявления остаются объявлениями.
			// Здесь проверяется исключение контейнера из V145, а не допустимость
			// вложения: её контролируют другие диагностики полного каталога.
			v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Open(t, name, 2, 1),
				v145Question(t, 4, 2), v145Note(t, 4, 2), v145Answer(t, 4, 2),
				v145Close(t, 4, 2)), "current", nil)
		})
	}
	t.Run("valid choice answers", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Line(t, "@choice Choose a city. {", "block-start", 2, 1,
			v145TestElement(t, "tag", "@choice", "choice", 1, 8), v145TestElement(t, "content", "Choose a city.", "Choose a city.", 9, 23),
			v145TestElement(t, "block-open", "{", "{", 24, 25)), v145Text(t, "Paris", "Paris", 3, 2),
			v145Answer(t, 3, 2), v145Close(t, 3, 2)), "current", nil)
	})
	t.Run("escaped answer remains text", func(t *testing.T) {
		v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Note(t, 2, 1),
			v145Text(t, `\@answer London`, "@answer London", 2, 1)), "current", nil)
	})
}

func TestV145IncludeRemainsOneConstruction(t *testing.T) {
	// Определение f следует после ответа, поэтому include нельзя раскрывать
	// при поиске соседства. Само определение имеет корректные ID и содержимое.
	v145Run(t, v145Document(t, v145Question(t, 2, 1), v145Tag(t, "include", "identifier", "f", 2, 1),
		v145Answer(t, 2, 1), v145Line(t, "@fragment f {", "block-start", 2, 1,
			v145TestElement(t, "tag", "@fragment", "fragment", 1, 10), v145TestElement(t, "identifier", "f", "f", 11, 12),
			v145TestElement(t, "block-open", "{", "{", 13, 14)), v145Text(t, "Read.", "Read.", 6, 2), v145Close(t, 6, 2)),
		"current", []v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 9}}})
}

func TestV145NoRawRecovery(t *testing.T) {
	for _, index := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("missing tag at offset %d", index), func(t *testing.T) {
			body := []model.Line{v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1)}
			raw := body[index].Raw
			body[index] = v145Line(t, raw, "invalid", 2, 1, model.Element{ElementType: "unparsed", Raw: raw,
				Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
			v145Run(t, v145Document(t, body...), "current", nil)
		})
	}
}

func TestV145EmptyVersionsAndLineEndings(t *testing.T) {
	for _, knownCount := range []bool{false, true} {
		t.Run("empty known count="+fmt.Sprint(knownCount), func(t *testing.T) {
			result := v145Empty(t, "1.2")
			if knownCount {
				count := 0
				result.Document.LineCount = &count
			}
			v145Run(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.21"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v145Empty(t, version)
			before := v145Snapshot(t, result)
			checks, err := validator.Prepare(result, v145Registry(t))
			if err == nil || checks != nil || v145Snapshot(t, result) != before {
				t.Fatalf("Prepare(%q) = %v, %v; want unchanged input and refusal", version, checks, err)
			}
		})
	}
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, finalEnding := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final=%t", ending, finalEnding), func(t *testing.T) {
				result := v145Document(t, v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !finalEnding {
					result.Lines[4].LineEnding = ""
				}
				v145Run(t, result, "current", []v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}})
			})
		}
	}
	t.Run("previous run is not counted", func(t *testing.T) {
		result := v145Document(t, v145Question(t, 2, 1), v145Note(t, 2, 1), v145Answer(t, 2, 1))
		v145Run(t, result, "first", []v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}})
		v145Run(t, result, "second", []v145Expected{{{5, 1, 8}, {3, 1, 10}, {4, 1, 6}}})
	})
}

// v145Registry создаёт реестр из полного штатного каталога, без подмены регистраций.
func v145Registry(t testing.TB) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v145Empty задаёт модель без доступных строк для проверки пустого входа и версий.
func v145Empty(t testing.TB, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v145Document дополняет явно заданные строки обязательной версией и заголовком.
// Типы, родители и элементы тела не выводятся из исходного текста.
func v145Document(t testing.TB, body ...model.Line) *model.Result {
	t.Helper()
	result := v145Empty(t, "1.2")
	title := "Lesson"
	result.Document.Metadata.Title = &title
	result.Lines = append([]model.Line{v145Tag(t, "dsl-version", "version", "1.2", 0, 0),
		v145Heading(t, "#", "1", "Lesson")}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v145Line сохраняет явно заданную структуру одной физической строки.
func v145Line(t testing.TB, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v145TestElement задаёт Raw и Value независимо, с исходными Unicode-колонками [start,end).
func v145TestElement(t testing.TB, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v145Tag собирает известное объявление с одним явно типизированным аргументом.
func v145Tag(t testing.TB, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v145TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v145TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v145Line(t, raw, "tag", parent, depth, elements...)
}

// v145Question сохраняет вопрос из эталона без изменений.
func v145Question(t testing.TB, parent, depth int) model.Line {
	t.Helper()
	return v145Tag(t, "question", "content", "Where does Tom live?", parent, depth)
}

// v145Answer сохраняет ответ из эталона без изменений.
func v145Answer(t testing.TB, parent, depth int) model.Line {
	t.Helper()
	return v145Tag(t, "answer", "content", "London", parent, depth)
}

// v145Note сохраняет промежуточное объявление из примера без изменений.
func v145Note(t testing.TB, parent, depth int) model.Line {
	t.Helper()
	return v145Tag(t, "note", "content", "Read carefully.", parent, depth)
}

// v145Text задаёт обычный текст; его семантическое значение не вычисляется из Raw.
func v145Text(t testing.TB, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v145Line(t, raw, "content", parent, depth, v145TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v145Open задаёт объявление без параметров с открытием блока.
func v145Open(t testing.TB, name string, parent, depth int) model.Line {
	t.Helper()
	return v145Line(t, "@"+name+" {", "block-start", parent, depth,
		v145TestElement(t, "tag", "@"+name, strings.ToLower(name), 1, len(name)+2),
		v145TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v145Close задаёт закрытие явно указанного фигурного родителя.
func v145Close(t testing.TB, parent, depth int) model.Line {
	t.Helper()
	return v145Line(t, "}", "block-end", parent, depth, v145TestElement(t, "block-close", "}", "}", 1, 2))
}

// v145Heading задаёт корневой заголовок с явно указанными маркером и уровнем.
func v145Heading(t testing.TB, marker, level, title string) model.Line {
	t.Helper()
	return v145Line(t, marker+" "+title, "heading", 0, 0,
		v145TestElement(t, "heading-level", marker, level, 1, len(marker)+1),
		v145TestElement(t, "title", title, title, len(marker)+2, len(marker)+2+utf8.RuneCountInString(title)))
}

// v145Location переводит явно заданную тройку ожидания в штатный тип диапазона.
func v145Location(t *testing.T, span [3]int) model.Location {
	t.Helper()
	return model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[0], Column: span[2]}}
}

// v145Run проверяет текущие V145 после выполнения полного штатного конвейера.
func v145Run(t *testing.T, result *model.Result, source string, want []v145Expected) {
	t.Helper()
	before := v145Snapshot(t, result)
	checks, err := validator.Prepare(result, v145Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v145Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	targetIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("new diagnostic source = %q, want %q", entry.Source, source)
		}
		if entry.DiagnosticCode == "V145" {
			got = append(got, entry)
			targetIDs[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V145 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		main := v145Location(t, expected[0])
		related := []model.Location{v145Location(t, expected[1]), v145Location(t, expected[2])}
		if entry.Location == nil || *entry.Location != main || !slices.Equal(entry.RelatedLocations, related) {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v / %+v", i, entry.Location, entry.RelatedLocations, main, related)
		}
		if entry.ID == "" || entry.Source != source || entry.DiagnosticCode != "V145" || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Между @question и @answer расположено другое содержимое." {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0][0]-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v145Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Lines) != len(original.Lines) || len(actual.Processing) != oldProcessing+1 {
		t.Fatal("source or history length changed")
	}
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		if len(actual.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			for _, id := range element.ErrorIDs {
				if targetIDs[id] {
					t.Errorf("ScopeLine V145 linked to element at %d:%d", i+1, j)
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v145Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v145Snapshot сохраняет модель для сравнения исходных данных и истории.
func v145Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

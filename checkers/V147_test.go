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

func TestV147Registration(t *testing.T) {
	registry := v147TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V147" {
			continue
		}
		count++
		if definition.Message != "Для @answer нет допустимого вопроса или элемента, к которому он относится." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V147 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V147 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || len(got) != 0 {
				t.Fatalf("unsupported version accepted: %v, %v", got, err)
			}
			result := v147TestDocument(t)
			result.Document.DSLVersion = &version
			before := v147TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("Prepare accepted version: %v, %v", checks, err)
			}
			if before != v147TestSnapshot(t, result) {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV147ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][3]int
	}{
		{"reference", "@question Where does Tom live?\n@answer London", []model.Line{
			v147TestTag(t, "question", "Where does Tom live?", 3, 2), v147TestTag(t, "answer", "London", 3, 2)}, nil},
		{"example", "@answer London", []model.Line{v147TestTag(t, "answer", "London", 3, 2)}, [][3]int{{4, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v147TestDocument(t, tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены версия, заголовок и контекст задания.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Unit 1\n@task 1\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v147TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV147QuestionAdjacency(t *testing.T) {
	q := v147TestTag(t, "question", "Q?", 3, 2)
	a := v147TestTag(t, "answer", "A", 3, 2)
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"no answer", []model.Line{q}, nil},
		{"several orphans", []model.Line{a, a}, [][3]int{{4, 1, 8}, {5, 1, 8}}},
		{"blank", []model.Line{q, v147TestLine(t, " \t", "blank", 3, 2), a}, nil},
		{"inline editor", []model.Line{q, v147TestTag(t, "editor", "Check", 3, 2), a}, nil},
		{"block editor", []model.Line{q, v147TestOpen(t, "editor", 3, 2),
			v147TestText(t, "Comment", "Comment", 5, 3), v147TestClose(t, 5, 3), a}, nil},
		{"one intermediate V145", []model.Line{q, v147TestTag(t, "note", "Note", 3, 2), a}, nil},
		{"intermediate text V145", []model.Line{q, v147TestText(t, "Note", "Note", 3, 2), a}, nil},
		{"two intermediate", []model.Line{q, v147TestTag(t, "note", "One", 3, 2),
			v147TestTag(t, "note", "Two", 3, 2), a}, [][3]int{{7, 1, 8}}},
		{"V145 with ignored comments", []model.Line{q, v147TestTag(t, "editor", "Check", 3, 2),
			v147TestTag(t, "note", "Note", 3, 2), v147TestLine(t, "", "blank", 3, 2), a}, nil},
		{"repeated answers V146", []model.Line{q, a, a, a}, nil},
		{"V146 with ignored comment", []model.Line{q, a, v147TestTag(t, "editor", "Check", 3, 2), a, a}, nil},
		{"block then repeated V146", []model.Line{q, v147TestOpen(t, "answer", 3, 2),
			v147TestText(t, "A", "A", 5, 3), v147TestClose(t, 5, 3), a}, nil},
		{"hint breaks repetition", []model.Line{q, a, v147TestTag(t, "hint", "Hint", 3, 2), a}, [][3]int{{7, 1, 8}}},
		{"sibling block is barrier", []model.Line{q, v147TestOpen(t, "note", 3, 2),
			v147TestText(t, "Note", "Note", 5, 3), v147TestClose(t, 5, 3), a}, [][3]int{{8, 1, 8}}},
		{"orphan block then orphan", []model.Line{v147TestOpen(t, "answer", 3, 2),
			v147TestText(t, "A", "A", 4, 3), v147TestClose(t, 4, 3), a}, [][3]int{{4, 1, 8}, {7, 1, 8}}},
		{"answer block owns no external answer", []model.Line{q, v147TestOpen(t, "answer", 3, 2),
			v147TestText(t, "A", "A", 5, 3), v147TestClose(t, 5, 3), v147TestTag(t, "hint", "Hint", 3, 2), a}, [][3]int{{9, 1, 8}}},
		{"wrong block question", []model.Line{v147TestOpen(t, "question", 3, 2),
			v147TestText(t, "Q?", "Q?", 4, 3), v147TestClose(t, 4, 3), a}, [][3]int{{7, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v147TestRun(t, v147TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV147StructuralContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"task boundary", []model.Line{v147TestTag(t, "question", "Q?", 3, 2), v147TestTag(t, "task", "2", 2, 1),
			v147TestTag(t, "answer", "A", 5, 2)}, [][3]int{{6, 1, 8}}},
		{"stage boundary", []model.Line{v147TestTag(t, "step", "First", 3, 2), v147TestTag(t, "question", "Q?", 4, 3),
			v147TestTag(t, "step", "Second", 3, 2), v147TestTag(t, "answer", "A", 6, 3)}, [][3]int{{7, 1, 8}}},
		{"within stage", []model.Line{v147TestTag(t, "step", "First", 3, 2), v147TestTag(t, "question", "Q?", 4, 3),
			v147TestTag(t, "editor", "Check", 4, 3), v147TestTag(t, "answer", "A", 4, 3)}, nil},
		{"endtask boundary", []model.Line{v147TestTag(t, "question", "Q?", 3, 2), v147TestTag(t, "endtask", "", 3, 2),
			v147TestTag(t, "answer", "A", 2, 1)}, [][3]int{{6, 1, 8}}},
		{"implicit step", []model.Line{v147TestTag(t, "endtask", "", 3, 2), v147TestTag(t, "question", "Q?", 2, 1),
			v147TestTag(t, "answer", "A", 2, 1)}, nil},
		{"heading boundary", []model.Line{v147TestTag(t, "question", "Q?", 3, 2),
			v147TestLine(t, "### Next", "heading", 0, 0, v147TestElement(t, "heading-level", "###", "3", 1), v147TestElement(t, "title", "Next", "Next", 5)),
			v147TestTag(t, "answer", "A", 5, 1)}, [][3]int{{6, 1, 8}}},
		{"branch boundary", []model.Line{v147TestOpen(t, "variants", 2, 1), v147TestTag(t, "variant", "A", 4, 2),
			v147TestTag(t, "question", "Q?", 5, 3), v147TestTag(t, "variant", "B", 4, 2), v147TestTag(t, "answer", "A", 7, 3),
			v147TestClose(t, 4, 2)}, [][3]int{{8, 1, 8}}},
		{"same branch", []model.Line{v147TestOpen(t, "variants", 2, 1), v147TestTag(t, "variant", "A", 4, 2),
			v147TestTag(t, "question", "Q?", 5, 3), v147TestTag(t, "answer", "A", 5, 3), v147TestTag(t, "variant", "B", 4, 2),
			v147TestTag(t, "question", "Other?", 8, 3), v147TestClose(t, 4, 2)}, nil},
		{"before first branch", []model.Line{v147TestOpen(t, "variants", 2, 1), v147TestTag(t, "question", "Q?", 4, 2),
			v147TestTag(t, "editor", "Check", 4, 2), v147TestTag(t, "answer", "A", 4, 2),
			v147TestTag(t, "variant", "A", 4, 2), v147TestText(t, "First", "First", 8, 3),
			v147TestTag(t, "variant", "B", 4, 2), v147TestText(t, "Second", "Second", 10, 3), v147TestClose(t, 4, 2)}, [][3]int{{7, 1, 8}}},
		{"deep branch task stage", []model.Line{v147TestOpen(t, "variants", 2, 1), v147TestTag(t, "variant", "A", 4, 2),
			v147TestTag(t, "task", "a-1", 5, 3), v147TestTag(t, "step", "Read", 6, 4),
			v147TestTag(t, "question", "Q?", 7, 5), v147TestTag(t, "editor", "Check", 7, 5), v147TestTag(t, "answer", "A", 7, 5),
			v147TestTag(t, "variant", "B", 4, 2), v147TestText(t, "Second", "Second", 11, 3), v147TestClose(t, 4, 2)}, nil},
		{"outer question cannot own nested answer", []model.Line{v147TestTag(t, "question", "Q?", 3, 2),
			v147TestOpen(t, "ordering", 3, 2), v147TestTag(t, "answer", "1", 5, 3),
			v147TestText(t, "One", "One", 5, 3), v147TestText(t, "Two", "Two", 5, 3), v147TestClose(t, 5, 3)}, [][3]int{{6, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v147TestRun(t, v147TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV147Choices(t *testing.T) {
	for _, name := range []string{"choice", "multichoice", "ChOiCe", "MULTICHOICE"} {
		t.Run(name, func(t *testing.T) {
			v147TestRun(t, v147TestDocument(t, v147TestOpen(t, name, 3, 2), v147TestTag(t, "answer", "A", 4, 3),
				v147TestText(t, "B", "B", 4, 3), v147TestOpen(t, "answer", 4, 3),
				v147TestText(t, "C", "C", 7, 4), v147TestText(t, "D", "D", 7, 4), v147TestClose(t, 7, 4), v147TestClose(t, 4, 3)), "current", nil)
		})
	}
	t.Run("choice owner ends with block", func(t *testing.T) {
		v147TestRun(t, v147TestDocument(t, v147TestOpen(t, "choice", 3, 2), v147TestTag(t, "answer", "A", 4, 3),
			v147TestText(t, "B", "B", 4, 3), v147TestClose(t, 4, 3), v147TestTag(t, "answer", "C", 3, 2)), "current", [][3]int{{8, 1, 8}})
	})
}

func TestV147ListOwners(t *testing.T) {
	// Каждый фрагмент получает блок в строке 4; все его прямые дети заданы явно.
	for _, name := range []string{"matching", "ordering", "MaTcHiNg", "ORDERING"} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want [][3]int
		}{
			{"owner", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestTag(t, "answer", "A", 4, 3)}, nil},
			{"blank", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestLine(t, " \t", "blank", 4, 3), v147TestTag(t, "answer", "A", 4, 3)}, nil},
			{"no preceding item", []model.Line{v147TestTag(t, "answer", "A", 4, 3), v147TestText(t, "Item", "Item", 4, 3)}, [][3]int{{5, 1, 8}}},
			{"repeated answer", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestTag(t, "answer", "A", 4, 3), v147TestTag(t, "answer", "B", 4, 3)}, [][3]int{{7, 1, 8}}},
			{"hint is not item", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestTag(t, "answer", "A", 4, 3),
				v147TestTag(t, "hint", "Hint", 4, 3), v147TestTag(t, "answer", "B", 4, 3)}, [][3]int{{8, 1, 8}}},
			{"editor forbidden", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestTag(t, "editor", "Note", 4, 3), v147TestTag(t, "answer", "A", 4, 3)}, [][3]int{{7, 1, 8}}},
			{"block editor forbidden", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestOpen(t, "editor", 4, 3),
				v147TestText(t, "Note", "Note", 6, 4), v147TestClose(t, 6, 4), v147TestTag(t, "answer", "A", 4, 3)}, [][3]int{{9, 1, 8}}},
			{"question not list item", []model.Line{v147TestTag(t, "question", "Q?", 4, 3), v147TestTag(t, "answer", "A", 4, 3),
				v147TestText(t, "Item", "Item", 4, 3)}, [][3]int{{6, 1, 8}}},
			{"no separated question exception", []model.Line{v147TestTag(t, "question", "Q?", 4, 3), v147TestTag(t, "note", "Note", 4, 3),
				v147TestTag(t, "answer", "A", 4, 3), v147TestText(t, "Item", "Item", 4, 3)}, [][3]int{{7, 1, 8}}},
			{"escaped tag is item", []model.Line{v147TestText(t, `\@answer A`, "@answer A", 4, 3), v147TestTag(t, "answer", "A", 4, 3)}, nil},
			{"block answer then repeat", []model.Line{v147TestText(t, "Item", "Item", 4, 3), v147TestOpen(t, "answer", 4, 3),
				v147TestText(t, "A", "A", 6, 4), v147TestClose(t, 6, 4), v147TestTag(t, "answer", "B", 4, 3)}, [][3]int{{9, 1, 8}}},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				body := append([]model.Line{v147TestOpen(t, name, 3, 2)}, tc.body...)
				// Дополнительный элемент с ответом обеспечивает минимум ordering.
				body = append(body, v147TestText(t, "Other", "Other", 4, 3), v147TestTag(t, "answer", "B", 4, 3),
					v147TestSeparator(t, 4, 3), v147TestText(t, "A", "A", 4, 3), v147TestText(t, "B", "B", 4, 3), v147TestClose(t, 4, 3))
				v147TestRun(t, v147TestDocument(t, body...), "current", tc.want)
			})
		}
	}
}

func TestV147Separators(t *testing.T) {
	for _, name := range []string{"matching", "ordering"} {
		// Нижняя часть matching передана V166; ordering остаётся областью V147.
		after, immediate := [][3]int{{9, 1, 8}}, [][3]int{{8, 1, 8}}
		if name == "matching" {
			after, immediate = nil, nil
		}
		for _, tc := range []struct {
			name string
			body []model.Line
			want [][3]int
		}{
			{"after separator", []model.Line{v147TestText(t, "One", "One", 4, 3), v147TestText(t, "Two", "Two", 4, 3),
				v147TestSeparator(t, 4, 3), v147TestText(t, "A", "A", 4, 3), v147TestTag(t, "answer", "A", 4, 3), v147TestText(t, "B", "B", 4, 3)}, after},
			{"immediately after separator", []model.Line{v147TestText(t, "One", "One", 4, 3), v147TestText(t, "Two", "Two", 4, 3),
				v147TestSeparator(t, 4, 3), v147TestTag(t, "answer", "A", 4, 3), v147TestText(t, "A", "A", 4, 3), v147TestText(t, "B", "B", 4, 3)}, immediate},
			{"multiple separators", []model.Line{v147TestText(t, "One", "One", 4, 3), v147TestTag(t, "answer", "A", 4, 3),
				v147TestText(t, "Two", "Two", 4, 3), v147TestSeparator(t, 4, 3), v147TestText(t, "A", "A", 4, 3),
				v147TestSeparator(t, 4, 3), v147TestText(t, "B", "B", 4, 3)}, [][3]int{{6, 1, 8}}},
			{"escaped delimiter is item", []model.Line{v147TestText(t, `\---`, "---", 4, 3), v147TestTag(t, "answer", "A", 4, 3),
				v147TestText(t, "Two", "Two", 4, 3), v147TestTag(t, "answer", "B", 4, 3), v147TestSeparator(t, 4, 3),
				v147TestText(t, "A", "A", 4, 3), v147TestText(t, "B", "B", 4, 3)}, nil},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				body := append([]model.Line{v147TestOpen(t, name, 3, 2)}, tc.body...)
				body = append(body, v147TestClose(t, 4, 3))
				v147TestRun(t, v147TestDocument(t, body...), "current", tc.want)
			})
		}
	}
	for _, tc := range []struct {
		name string
		want [][3]int
	}{
		{"matching", [][3]int{{6, 1, 8}, {8, 1, 8}}}, {"ordering", nil},
	} {
		t.Run(tc.name+"/no separator", func(t *testing.T) {
			v147TestRun(t, v147TestDocument(t, v147TestOpen(t, tc.name, 3, 2),
				v147TestText(t, "One", "One", 4, 3), v147TestTag(t, "answer", "1", 4, 3),
				v147TestText(t, "Two", "Two", 4, 3), v147TestTag(t, "answer", "2", 4, 3), v147TestClose(t, 4, 3)), "current", tc.want)
		})
	}
}

func TestV147LowerBlockAnswerBelongsToV166(t *testing.T) {
	v147TestRun(t, v147TestDocument(t, v147TestOpen(t, "matching", 3, 2),
		v147TestText(t, "Item", "Item", 4, 3), v147TestSeparator(t, 4, 3),
		v147TestText(t, "A", "A", 4, 3), v147TestText(t, "B", "B", 4, 3),
		v147TestOpen(t, "answer", 4, 3), v147TestText(t, "A", "A", 9, 4),
		v147TestText(t, "B", "B", 9, 4), v147TestClose(t, 9, 4), v147TestClose(t, 4, 3)), "current", nil)
}

func TestV147SpecializedNesting(t *testing.T) {
	for _, name := range []string{"example", "wordlist", "table", "script", "text", "key", "instr", "note", "alt", "hint", "fragment"} {
		t.Run(name, func(t *testing.T) {
			opening := v147TestOpen(t, name, 3, 2)
			if name == "fragment" {
				opening = v147TestLine(t, "@fragment f {", "block-start", 3, 2, v147TestElement(t, "tag", "@fragment", "fragment", 1),
					v147TestElement(t, "identifier", "f", "f", 11), v147TestElement(t, "block-open", "{", "{", 13))
			}
			v147TestRun(t, v147TestDocument(t, opening, v147TestTag(t, "answer", "A", 4, 3), v147TestClose(t, 4, 3)), "current", nil)
		})
	}
	t.Run("nested answer V140", func(t *testing.T) {
		v147TestRun(t, v147TestDocument(t, v147TestTag(t, "question", "Q?", 3, 2), v147TestOpen(t, "answer", 3, 2),
			v147TestTag(t, "answer", "A", 5, 3), v147TestClose(t, 5, 3)), "current", nil)
	})
}

func TestV147ListBlockBoundaries(t *testing.T) {
	for _, name := range []string{"matching", "ordering"} {
		t.Run(name+"/delimiter text in answer", func(t *testing.T) {
			v147TestRun(t, v147TestDocument(t, v147TestOpen(t, name, 3, 2),
				v147TestText(t, "Item", "Item", 4, 3), v147TestOpen(t, "answer", 4, 3),
				v147TestText(t, "---", "---", 6, 4), v147TestClose(t, 6, 4),
				v147TestText(t, "Other", "Other", 4, 3), v147TestTag(t, "answer", "A", 4, 3),
				v147TestSeparator(t, 4, 3), v147TestText(t, `\---`, "---", 4, 3),
				v147TestText(t, "A", "A", 4, 3), v147TestClose(t, 4, 3)), "current", nil)
		})
		t.Run(name+"/nested block cannot supply item", func(t *testing.T) {
			v147TestRun(t, v147TestDocument(t, v147TestOpen(t, name, 3, 2),
				v147TestOpen(t, "note", 4, 3), v147TestText(t, "Not an item", "Not an item", 5, 4), v147TestClose(t, 5, 4),
				v147TestTag(t, "answer", "A", 4, 3), v147TestText(t, "One", "One", 4, 3), v147TestText(t, "Two", "Two", 4, 3),
				v147TestSeparator(t, 4, 3), v147TestText(t, "A", "A", 4, 3), v147TestText(t, "B", "B", 4, 3),
				v147TestClose(t, 4, 3)), "current", [][3]int{{8, 1, 8}})
		})
	}
	t.Run("own separators in separate blocks", func(t *testing.T) {
		v147TestRun(t, v147TestDocument(t, v147TestOpen(t, "ordering", 3, 2),
			v147TestText(t, "One", "One", 4, 3), v147TestTag(t, "answer", "A", 4, 3),
			v147TestText(t, "Two", "Two", 4, 3), v147TestTag(t, "answer", "B", 4, 3),
			v147TestLine(t, " \t---\t ", "separator", 4, 3), v147TestText(t, "A", "A", 4, 3), v147TestText(t, "B", "B", 4, 3), v147TestClose(t, 4, 3),
			v147TestOpen(t, "matching", 3, 2), v147TestText(t, "Item", "Item", 13, 3), v147TestTag(t, "answer", "A", 13, 3),
			v147TestSeparator(t, 13, 3), v147TestText(t, "A", "A", 13, 3), v147TestText(t, "B", "B", 13, 3), v147TestClose(t, 13, 3)), "current", nil)
	})
	t.Run("deep choice in branch task stage", func(t *testing.T) {
		v147TestRun(t, v147TestDocument(t, v147TestOpen(t, "variants", 2, 1), v147TestTag(t, "variant", "A", 4, 2),
			v147TestTag(t, "task", "a-1", 5, 3), v147TestTag(t, "step", "Choose", 6, 4), v147TestOpen(t, "choice", 7, 5),
			v147TestTag(t, "answer", "A", 8, 6), v147TestText(t, "B", "B", 8, 6), v147TestClose(t, 8, 6),
			v147TestTag(t, "variant", "B", 4, 2), v147TestText(t, "Other", "Other", 12, 3), v147TestClose(t, 4, 2)), "current", nil)
	})
}

func TestV147MultifillAndLiterals(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"inside multifill", []model.Line{v147TestOpen(t, "multifill", 3, 2), v147TestTag(t, "answer", "A", 4, 3), v147TestClose(t, 4, 3)}, nil},
		{"after inline", []model.Line{v147TestTag(t, "multifill", "Edit", 3, 2), v147TestTag(t, "answer", "A", 3, 2)}, nil},
		{"after inline with blank editor", []model.Line{v147TestTag(t, "multifill", "Edit", 3, 2), v147TestLine(t, "", "blank", 3, 2),
			v147TestTag(t, "editor", "Check", 3, 2), v147TestTag(t, "answer", "A", 3, 2)}, nil},
		{"after block with editor", []model.Line{v147TestOpen(t, "multifill", 3, 2), v147TestText(t, "Text", "Text", 4, 3),
			v147TestClose(t, 4, 3), v147TestTag(t, "editor", "Check", 3, 2), v147TestTag(t, "answer", "A", 3, 2)}, nil},
		{"after block with block editor", []model.Line{v147TestOpen(t, "multifill", 3, 2), v147TestText(t, "Text", "Text", 4, 3),
			v147TestClose(t, 4, 3), v147TestOpen(t, "editor", 3, 2), v147TestText(t, "Check", "Check", 7, 3), v147TestClose(t, 7, 3),
			v147TestTag(t, "answer", "A", 3, 2)}, nil},
		{"nonadjacent multifill", []model.Line{v147TestTag(t, "multifill", "Edit", 3, 2), v147TestTag(t, "note", "Note", 3, 2),
			v147TestTag(t, "answer", "A", 3, 2)}, [][3]int{{6, 1, 8}}},
		{"multifill does not own chain", []model.Line{v147TestTag(t, "multifill", "Edit", 3, 2), v147TestTag(t, "answer", "A", 3, 2),
			v147TestTag(t, "answer", "B", 3, 2)}, [][3]int{{6, 1, 8}}},
		{"literal editor", []model.Line{v147TestOpen(t, "editor", 3, 2), v147TestText(t, "@answer A", "@answer A", 4, 3), v147TestClose(t, 4, 3)}, nil},
		{"escaped answer", []model.Line{v147TestText(t, `\@answer A`, "@answer A", 3, 2)}, nil},
		{"midline answer", []model.Line{v147TestText(t, "Text @answer A", "Text @answer A", 3, 2)}, nil},
		{"literal editor cannot own outer answer", []model.Line{v147TestOpen(t, "editor", 3, 2), v147TestText(t, "@question Q?", "@question Q?", 4, 3),
			v147TestClose(t, 4, 3), v147TestTag(t, "answer", "A", 3, 2)}, [][3]int{{7, 1, 8}}},
		{"literal multifill tags", []model.Line{v147TestOpen(t, "multifill", 3, 2), v147TestText(t, "@choice {", "@choice {", 4, 3),
			v147TestText(t, `\@answer A`, "@answer A", 4, 3), v147TestClose(t, 4, 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v147TestRun(t, v147TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV147RawPositionsAndPartialParsing(t *testing.T) {
	t.Run("partially parsed question remains owner", func(t *testing.T) {
		question := v147TestLine(t, "@QUESTION\tQ?", "invalid", 3, 2,
			v147TestElement(t, "unparsed", "@QUESTION\tQ?", "", 1))
		v147TestRun(t, v147TestDocument(t, question, v147TestTag(t, "answer", "A", 3, 2)), "current", nil)
	})
	t.Run("unparsed answer tail", func(t *testing.T) {
		answer := v147TestLine(t, "\t@answer }", "tag", 3, 2,
			v147TestElement(t, "tag", "@answer", "answer", 2), v147TestElement(t, "unparsed", "}", "", 10))
		v147TestRun(t, v147TestDocument(t, answer), "current", [][3]int{{4, 2, 9}})
	})
	t.Run("case and Unicode answer", func(t *testing.T) {
		line := v147TestLine(t, "  @AnSwEr Я😀", "tag", 3, 2,
			v147TestElement(t, "tag", "@AnSwEr", "answer", 3), v147TestElement(t, "content", "Я😀", "Я😀", 11))
		v147TestRun(t, v147TestDocument(t, line), "current", [][3]int{{4, 3, 10}})
	})
	t.Run("case of question", func(t *testing.T) {
		v147TestRun(t, v147TestDocument(t, v147TestTag(t, "QuEsTiOn", "Где?", 3, 2),
			v147TestTag(t, "ANSWER", "Я😀", 3, 2)), "current", nil)
	})
	for _, text := range []string{"London", "?", "Я😀", "____", "no matching value"} {
		t.Run("content irrelevant/"+text, func(t *testing.T) {
			v147TestRun(t, v147TestDocument(t, v147TestTag(t, "answer", text, 3, 2)), "current", [][3]int{{4, 1, 8}})
		})
	}
	for _, known := range []bool{false, true} {
		for _, owner := range []bool{false, true} {
			t.Run("partial/known="+strconv.FormatBool(known)+"/owner="+strconv.FormatBool(owner), func(t *testing.T) {
				var body []model.Line
				want := [][3]int{{4, 1, 8}}
				if owner {
					body = append(body, v147TestTag(t, "question", "Q?", 3, 2))
					want = nil
				}
				elements := []model.Element{v147TestElement(t, "unparsed", "@answer", "", 1)}
				if known {
					elements = []model.Element{v147TestElement(t, "tag", "@answer", "answer", 1), v147TestElement(t, "unparsed", "", "", 8)}
				}
				body = append(body, v147TestLine(t, "@answer", "invalid", 3, 2, elements...))
				v147TestRun(t, v147TestDocument(t, body...), "current", want)
			})
		}
	}
	for _, raw := range []string{"@answers A", "@answerExtra A", "@answer? A", "\u00a0@answer A"} {
		t.Run("not exact initial name/"+raw, func(t *testing.T) {
			v147TestRun(t, v147TestDocument(t, v147TestLine(t, raw, "invalid", 3, 2,
				v147TestElement(t, "unparsed", raw, "", 1))), "current", nil)
		})
	}
}

func TestV147EndingsAndRepeatedRun(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v147TestDocument(t, v147TestTag(t, "answer", "A", 3, 2), v147TestTag(t, "question", "Q?", 3, 2),
				v147TestTag(t, "answer", "B", 3, 2), v147TestTag(t, "hint", "Hint", 3, 2), v147TestTag(t, "answer", "C", 3, 2))
			for i := range result.Lines {
				result.Lines[i].LineEnding = eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = ""
			want := [][3]int{{4, 1, 8}, {8, 1, 8}}
			v147TestRun(t, result, "first", want)
			v147TestRun(t, result, "second", want)
		})
	}
	t.Run("empty input", func(t *testing.T) {
		result := v147TestDocument(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		v147TestRun(t, result, "current", nil)
	})
}

// v147TestDocument добавляет контекст из п. 1.30 и сохраняет заданные модели строк.
// Классификация, родитель и глубина каждой строки задаются в тесте, не выводятся из DSL.
func v147TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Unit 1"
	lines := append([]model.Line{
		v147TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v147TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v147TestElement(t, "version", "1.2", "1.2", 14)),
		v147TestLine(t, "# Unit 1", "heading", 0, 0, v147TestElement(t, "heading-level", "#", "1", 1), v147TestElement(t, "title", "Unit 1", "Unit 1", 3)),
		v147TestTag(t, "task", "1", 2, 1),
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
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Не разобран фрагмент исходной строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v147TestTag строит известное однострочное объявление с явным контекстом.
func v147TestTag(t *testing.T, name, text string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v147TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if text != "" {
		kind := model.ElementType("content")
		switch strings.ToLower(name) {
		case "task":
			kind = "identifier"
		case "step":
			kind = "title"
		case "variant":
			kind = "name"
		}
		elements = append(elements, v147TestElement(t, kind, text, text, utf8.RuneCountInString(raw)+2))
		raw += " " + text
	}
	return v147TestLine(t, raw, "tag", parent, depth, elements...)
}

// v147TestOpen строит объявление блока без дополнительных аргументов.
func v147TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v147TestLine(t, "@"+name+" {", "block-start", parent, depth, v147TestElement(t, "tag", "@"+name, strings.ToLower(name), 1),
		v147TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v147TestClose закрывает явно указанный блок.
func v147TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v147TestLine(t, "}", "block-end", parent, depth, v147TestElement(t, "block-close", "}", "}", 1))
}

// v147TestText сохраняет независимые исходное и семантическое значения.
func v147TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v147TestLine(t, raw, "content", parent, depth, v147TestElement(t, "content", raw, value, 1))
}

// v147TestSeparator задаёт структурный разделитель, а не текст с тем же значением.
func v147TestSeparator(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v147TestLine(t, "---", "separator", parent, depth)
}

// v147TestLine задаёт структуру входа без парсера или восстановления родителей.
func v147TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v147TestElement измеряет входной элемент в Unicode-колонках с исключённым концом.
// Ожидаемые диапазоны V147 задаются отдельно числовыми литералами.
func v147TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v147TestRegistry использует полный штатный каталог без альтернативной регистрации.
func v147TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v147TestRun проверяет только новые V147 полного конвейера и целостность модели.
// want содержит явные тройки: строка, включённая и исключённая колонки.
func v147TestRun(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v147TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v147TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v147TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V147" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V147 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "Для @answer нет допустимого вопроса или элемента, к которому он относится." {
			t.Fatalf("occurrence %d = %+v, location %+v; want %+v and exact metadata", i, entry, entry.Location, location)
		}
		if !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line diagnostic linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v147TestSnapshot(t, result)), &restored); err != nil {
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
	if v147TestSnapshot(t, &restored) != before {
		t.Fatal("source or prior history changed")
	}
}

// v147TestSnapshot сохраняет независимое представление модели для сравнения.
func v147TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

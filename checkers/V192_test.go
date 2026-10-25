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

// v192Expected задаёт полный исходный диапазон с исключающим концом.
type v192Expected struct{ line, start, endLine, end int }

func TestV192ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		body         []model.Line
		want         []v192Expected
	}{
		{"reference", "Work in pairs.\n@variants {\n@variant A\nRead.\n@variant B\nWrite.\n}", []model.Line{
			v192TestText(t, "Work in pairs.", "Work in pairs.", 2, 1), v192TestOpen(t, "variants", 2, 1),
			v192TestTag(t, "variant", "name", "A", 4, 2), v192TestText(t, "Read.", "Read.", 5, 3),
			v192TestTag(t, "variant", "name", "B", 4, 2), v192TestText(t, "Write.", "Write.", 7, 3), v192TestClose(t, 4, 2),
		}, nil},
		{"example", "@variants {\nWork in pairs.\n@variant A\nRead.\n@variant B\nWrite.\n}", v192TestExample(t), []v192Expected{{4, 1, 4, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// Добавлены только обязательные версия и основной заголовок по DSL 1.30.
			v192TestRun(t, v192TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV192UnitsWithoutBranches(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v192Expected
	}{
		{"empty block", nil, nil},
		{"blank and ASCII whitespace", []model.Line{v192TestBlank(t, "", 3, 2), v192TestBlank(t, " \t ", 3, 2)}, nil},
		{"one line", []model.Line{v192TestText(t, "Work in pairs.", "Work in pairs.", 3, 2)}, []v192Expected{{4, 1, 4, 15}}},
		{"duplicate lines", []model.Line{v192TestText(t, "Read.", "Read.", 3, 2), v192TestText(t, "Read.", "Read.", 3, 2)}, []v192Expected{{4, 1, 4, 6}, {5, 1, 5, 6}}},
		{"inline declaration", []model.Line{v192TestTag(t, "question", "content", "Q?", 3, 2)}, []v192Expected{{4, 1, 4, 13}}},
		{"escaped branch", []model.Line{v192TestText(t, `\@variant A`, "@variant A", 3, 2)}, []v192Expected{{4, 1, 4, 12}}},
		{"mention of branch", []model.Line{v192TestText(t, "Read @variant A", "Read @variant A", 3, 2)}, []v192Expected{{4, 1, 4, 16}}},
		{"escaped editor", []model.Line{v192TestText(t, `\@editor X`, "@editor X", 3, 2)}, []v192Expected{{4, 1, 4, 11}}},
		{"escaped heading", []model.Line{v192TestText(t, `\# X`, "# X", 3, 2)}, []v192Expected{{4, 1, 4, 5}}},
		{"Unicode and outer whitespace", []model.Line{v192TestText(t, " \tЯ😀 \t", "Я😀", 3, 2)}, []v192Expected{{4, 1, 4, 7}}},
		{"NBSP is content", []model.Line{v192TestText(t, "\u00a0", "\u00a0", 3, 2)}, []v192Expected{{4, 1, 4, 2}}},
		{"inline editor", []model.Line{v192TestTag(t, "editor", "content", "Read.", 3, 2)}, nil},
		{"editor body is literal", []model.Line{v192TestOpen(t, "editor", 3, 2), v192TestText(t, "@variant A", "@variant A", 4, 3), v192TestClose(t, 4, 3),
			v192TestText(t, "Work in pairs.", "Work in pairs.", 3, 2)}, []v192Expected{{7, 1, 7, 15}}},
		{"note is one unit", []model.Line{v192TestOpen(t, "note", 3, 2), v192TestText(t, "Read.", "Read.", 4, 3), v192TestClose(t, 4, 3)}, []v192Expected{{4, 1, 6, 2}}},
		{"fragment definition", []model.Line{v192TestFragment(t, 3, 2), v192TestText(t, "Read.", "Read.", 4, 3), v192TestClose(t, 4, 3)}, []v192Expected{{4, 1, 6, 2}}},
		{"multifill literal body", []model.Line{v192TestOpen(t, "multifill", 3, 2), v192TestText(t, "@variant A", "@variant A", 4, 3), v192TestClose(t, 4, 3),
			v192TestText(t, "Read.", "Read.", 3, 2)}, []v192Expected{{4, 1, 6, 2}, {7, 1, 7, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v192TestRun(t, v192TestContainer(t, tc.rows...), "current", tc.want) })
	}
}

func TestV192NestedBlocksAreNotRevisited(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rows      []model.Line
		want      []v192Expected
		errorLine int
	}{
		{"branch inside note is not boundary", []model.Line{
			v192TestOpen(t, "note", 3, 2), v192TestTag(t, "variant", "name", "A", 4, 3), v192TestClose(t, 4, 3), v192TestText(t, "Read.", "Read.", 3, 2),
		}, []v192Expected{{4, 1, 6, 2}, {7, 1, 7, 6}}, 5},
		{"variants inside forbidden block", []model.Line{
			v192TestOpen(t, "note", 3, 2), v192TestOpen(t, "variants", 4, 3), v192TestText(t, "Read.", "Read.", 5, 4),
			v192TestClose(t, 5, 4), v192TestClose(t, 4, 3),
		}, []v192Expected{{4, 1, 8, 2}}, 5},
		{"excluded nested variants body", []model.Line{
			v192TestOpen(t, "variants", 3, 2), v192TestText(t, "Read.", "Read.", 4, 3),
			v192TestTag(t, "variant", "name", "A", 4, 3), v192TestText(t, "Write.", "Write.", 6, 4), v192TestClose(t, 4, 3),
			v192TestText(t, "Read.", "Read.", 3, 2),
		}, []v192Expected{{9, 1, 9, 6}}, 4},
		{"unparsed contents of bounded block", []model.Line{
			v192TestOpen(t, "note", 3, 2), v192TestInvalid(t, "@variant{", 4, 3), v192TestClose(t, 4, 3),
		}, []v192Expected{{4, 1, 6, 2}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v192TestContainer(t, tc.rows...)
			if tc.errorLine != 0 {
				v192TestError(t, result, tc.errorLine, -1, "Конструкция запрещена во вложенном блоке.")
			}
			v192TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("whole block retains outer whitespace", func(t *testing.T) {
		opening := v192TestOpen(t, "note", 3, 2)
		opening.Raw = " \t@note {  "
		for i := range opening.Elements {
			opening.Elements[i].Start += 2
			opening.Elements[i].End += 2
		}
		closing := v192TestLine(t, " \t} \t", "block-end", 4, 3, v192TestElement(t, "block-close", "}", "}", 3, 4))
		v192TestRun(t, v192TestContainer(t, opening, v192TestText(t, "Read.", "Read.", 4, 3), closing), "current", []v192Expected{{4, 1, 6, 6}})
	})
}

func TestV192ExcludedConstructions(t *testing.T) {
	for _, tc := range []struct {
		tag   string
		kind  model.ElementType
		value string
	}{
		{"include", "identifier", "f"}, {"document-id", "identifier", "d"}, {"section", "name", "S"},
		{"order", "number", "1"}, {"resource-dir", "resource-path", "assets"}, {"newpage", "", ""},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			result := v192TestContainer(t, v192TestTag(t, tc.tag, tc.kind, tc.value, 3, 2))
			v192TestRun(t, result, "current", nil)
		})
		t.Run(tc.tag+" does not end prelude", func(t *testing.T) {
			result := v192TestContainer(t, v192TestTag(t, tc.tag, tc.kind, tc.value, 3, 2),
				v192TestText(t, "Read.", "Read.", 3, 2))
			v192TestRun(t, result, "current", []v192Expected{{5, 1, 5, 6}})
		})
	}
	for _, level := range []int{1, 2, 3} {
		t.Run("heading "+strconv.Itoa(level), func(t *testing.T) {
			marker := strings.Repeat("#", level)
			// Вложенный запрещённый заголовок сохраняет контекст через invalid.
			unparsed := v192TestElement(t, "unparsed", " Heading", "", level+1, level+9)
			unparsed.Value = nil
			line := v192TestLine(t, marker+" Heading", "invalid", 3, 2,
				v192TestElement(t, "heading-level", marker, strconv.Itoa(level), 1, level+1), unparsed)
			v192TestRun(t, v192TestContainer(t, line), "current", nil)
			v192TestRun(t, v192TestContainer(t, line, v192TestText(t, "Read.", "Read.", 3, 2)), "following", []v192Expected{{5, 1, 5, 6}})
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal variants in "+tag, func(t *testing.T) {
			result := v192TestDocument(t, v192TestOpen(t, tag, 2, 1), v192TestText(t, "@variants {", "@variants {", 3, 2),
				v192TestText(t, "Read.", "Read.", 3, 2), v192TestText(t, `\}`, "}", 3, 2), v192TestClose(t, 3, 2))
			v192TestRun(t, result, "current", nil)
		})
	}
}

func TestV192AnswerOwnership(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v192Expected
	}{
		{"orphan inline", []model.Line{v192TestTag(t, "answer", "content", "A", 3, 2)}, nil},
		{"orphan does not end prelude", []model.Line{v192TestTag(t, "answer", "content", "A", 3, 2), v192TestText(t, "Read.", "Read.", 3, 2)}, []v192Expected{{5, 1, 5, 6}}},
		{"orphan block", []model.Line{v192TestOpen(t, "answer", 3, 2), v192TestText(t, "A", "A", 4, 3), v192TestClose(t, 4, 3)}, nil},
		{"linked inline", []model.Line{v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestTag(t, "answer", "content", "A", 3, 2)}, []v192Expected{{4, 1, 4, 13}, {5, 1, 5, 10}}},
		{"linked through blank and editor", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestBlank(t, "", 3, 2), v192TestTag(t, "editor", "content", "Check.", 3, 2), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 4, 13}, {7, 1, 7, 10}}},
		{"linked through block editor", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestOpen(t, "editor", 3, 2),
			v192TestText(t, "@answer X", "@answer X", 5, 3), v192TestClose(t, 5, 3), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 4, 13}, {8, 1, 8, 10}}},
		{"linked block", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestOpen(t, "answer", 3, 2), v192TestText(t, "A", "A", 5, 3), v192TestClose(t, 5, 3),
		}, []v192Expected{{4, 1, 4, 13}, {5, 1, 7, 2}}},
		{"text breaks link", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestText(t, "Read.", "Read.", 3, 2), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 4, 13}, {5, 1, 5, 6}}},
		{"excluded tag breaks link", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestTag(t, "include", "identifier", "f", 3, 2), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 4, 13}}},
		{"block breaks link", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestOpen(t, "note", 3, 2), v192TestText(t, "Read.", "Read.", 5, 3),
			v192TestClose(t, 5, 3), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 4, 13}, {5, 1, 7, 2}}},
		{"repeat is orphan", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 3, 2), v192TestTag(t, "answer", "content", "A", 3, 2), v192TestTag(t, "answer", "content", "B", 3, 2),
		}, []v192Expected{{4, 1, 4, 13}, {5, 1, 5, 10}}},
		{"question in closed container is not owner", []model.Line{
			v192TestOpen(t, "note", 3, 2), v192TestTag(t, "question", "content", "Q?", 4, 3), v192TestClose(t, 4, 3), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 6, 2}}},
		{"task boundary breaks link", []model.Line{
			v192TestTag(t, "task", "identifier", "t", 3, 2), v192TestTag(t, "question", "content", "Q?", 4, 3),
			v192TestTag(t, "endtask", "", "", 4, 3), v192TestTag(t, "answer", "content", "A", 3, 2),
		}, []v192Expected{{4, 1, 4, 8}, {5, 1, 5, 13}, {6, 1, 6, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v192TestContainer(t, tc.rows...)
			if tc.name == "question in closed container is not owner" {
				v192TestError(t, result, 5, -1, "Вопрос запрещён внутри note.")
			}
			v192TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV192FirstBranchAndContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v192Expected
	}{
		{"material belongs to branch", []model.Line{
			v192TestOpen(t, "variants", 2, 1), v192TestTag(t, "variant", "name", "A", 3, 2), v192TestText(t, "Read.", "Read.", 4, 3),
			v192TestOpen(t, "note", 4, 3), v192TestText(t, "Write.", "Write.", 6, 4), v192TestClose(t, 6, 4),
			v192TestTag(t, "variant", "name", "B", 3, 2), v192TestText(t, "Read.", "Read.", 9, 3), v192TestClose(t, 3, 2),
		}, nil},
		{"first branch through task and step", []model.Line{
			v192TestOpen(t, "variants", 2, 1), v192TestTag(t, "task", "identifier", "t", 3, 2), v192TestTag(t, "step", "", "", 4, 3),
			v192TestText(t, "Read.", "Read.", 5, 4), v192TestTag(t, "variant", "name", "A", 5, 4),
			v192TestText(t, "Write.", "Write.", 7, 5), v192TestClose(t, 3, 2),
		}, []v192Expected{{4, 1, 4, 8}, {5, 1, 5, 6}, {6, 1, 6, 6}}},
		{"variants inside task", []model.Line{
			v192TestTag(t, "task", "identifier", "t", 2, 1), v192TestOpen(t, "variants", 3, 2), v192TestText(t, "Read.", "Read.", 4, 3),
			v192TestClose(t, 4, 3), v192TestTag(t, "endtask", "", "", 3, 2),
		}, []v192Expected{{5, 1, 5, 6}}},
		{"independent blocks", []model.Line{
			v192TestOpen(t, "variants", 2, 1), v192TestText(t, "Read.", "Read.", 3, 2), v192TestClose(t, 3, 2),
			v192TestOpen(t, "variants", 2, 1), v192TestText(t, "Read.", "Read.", 6, 2), v192TestClose(t, 6, 2),
		}, []v192Expected{{4, 1, 4, 6}, {7, 1, 7, 6}}},
		{"question outside variants is not owner", []model.Line{
			v192TestTag(t, "question", "content", "Q?", 2, 1), v192TestOpen(t, "variants", 2, 1),
			v192TestTag(t, "answer", "content", "A", 4, 2), v192TestClose(t, 4, 2),
		}, nil},
		{"material after variants", []model.Line{
			v192TestOpen(t, "variants", 2, 1), v192TestClose(t, 3, 2), v192TestText(t, "Read.", "Read.", 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v192TestRun(t, v192TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV192KeepsCertainPreludeBeforeUnknownConstruction(t *testing.T) {
	result := v192TestDocument(t, v192TestOpen(t, "variants", 2, 1),
		v192TestText(t, "Read.", "Read.", 3, 2), v192TestInvalid(t, "@variant{", 3, 2),
		v192TestText(t, "Uncertain.", "Uncertain.", 3, 2), v192TestClose(t, 3, 2))
	v192TestRun(t, result, "current", []v192Expected{{4, 1, 4, 6}})
}

func TestV192KeepsCompletedUnitsBeforeUncertainBoundary(t *testing.T) {
	for _, tc := range []struct {
		name      string
		tail      []model.Line
		errorLine int
	}{
		{"unparsed before real branch", []model.Line{
			v192TestInvalid(t, "@variant{", 3, 2), v192TestText(t, "Uncertain.", "Uncertain.", 3, 2),
			v192TestTag(t, "variant", "name", "A", 3, 2), v192TestText(t, "Branch.", "Branch.", 10, 3),
		}, 0},
		{"missing nested closure", []model.Line{
			v192TestOpen(t, "note", 3, 2), v192TestText(t, "Uncertain.", "Uncertain.", 8, 3),
			v192TestTag(t, "variant", "name", "A", 8, 3), v192TestText(t, "Branch.", "Branch.", 10, 4),
		}, 8},
		{"ambiguous nested closure", []model.Line{
			v192TestOpen(t, "note", 3, 2), v192TestText(t, "Uncertain.", "Uncertain.", 8, 3),
			v192TestClose(t, 8, 3), v192TestClose(t, 8, 3),
			v192TestTag(t, "variant", "name", "A", 3, 2), v192TestText(t, "Branch.", "Branch.", 12, 3),
		}, 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.Line{v192TestOpen(t, "note", 3, 2), v192TestText(t, "Read.", "Read.", 4, 3),
				v192TestClose(t, 4, 3), v192TestText(t, "Before.", "Before.", 3, 2)}
			result := v192TestContainer(t, append(rows, tc.tail...)...)
			if tc.errorLine != 0 {
				v192TestError(t, result, tc.errorLine, -1, "Границы вложенного блока не определены однозначно.")
			}
			v192TestRun(t, result, "current", []v192Expected{{4, 1, 6, 2}, {7, 1, 7, 8}})
		})
	}
}

func TestV192PhysicalPreludeSurvivesLogicalNewpage(t *testing.T) {
	result := v192TestDocument(t, v192TestOpen(t, "variants", 2, 1),
		v192TestText(t, "Before.", "Before.", 3, 2), v192TestTag(t, "newpage", "", "", 2, 1),
		v192TestText(t, "After.", "After.", 5, 2), v192TestTag(t, "variant", "name", "A", 5, 2),
		v192TestText(t, "Branch.", "Branch.", 7, 3), v192TestClose(t, 3, 2))
	v192TestError(t, result, 5, -1, "Недопустимая граница шага внутри variants.")
	v192TestRun(t, result, "current", []v192Expected{{4, 1, 4, 8}, {6, 1, 6, 7}})
}

func TestV192PhysicalPreludeSurvivesLogicalHeading(t *testing.T) {
	result := v192TestDocument(t, v192TestOpen(t, "variants", 2, 1),
		v192TestText(t, "Before.", "Before.", 3, 2),
		v192TestLine(t, "### Next", "heading", 0, 0, v192TestElement(t, "heading-level", "###", "3", 1, 4),
			v192TestElement(t, "title", "Next", "Next", 5, 9)),
		v192TestText(t, "After.", "After.", 5, 1), v192TestOpen(t, "note", 5, 1),
		v192TestText(t, "Read.", "Read.", 7, 2), v192TestClose(t, 7, 2),
		v192TestTag(t, "variant", "name", "A", 5, 1), v192TestText(t, "Branch.", "Branch.", 10, 2),
		v192TestClose(t, 3, 2))
	v192TestError(t, result, 5, -1, "Заголовок запрещён внутри variants.")
	v192TestRun(t, result, "current", []v192Expected{{4, 1, 4, 8}, {6, 1, 6, 7}, {7, 1, 9, 2}})
}

func TestV192UncertainBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		errorLine int
	}{
		{"missing closure", []model.Line{v192TestOpen(t, "variants", 2, 1), v192TestText(t, "Read.", "Read.", 3, 2)}, 3},
		{"ambiguous closure", []model.Line{v192TestOpen(t, "variants", 2, 1), v192TestText(t, "Read.", "Read.", 3, 2), v192TestClose(t, 3, 2), v192TestClose(t, 3, 2)}, 6},
		{"missing nested closure", []model.Line{v192TestOpen(t, "variants", 2, 1), v192TestOpen(t, "note", 3, 2), v192TestText(t, "Read.", "Read.", 4, 3), v192TestClose(t, 3, 2)}, 4},
		{"unparsed possible branch", []model.Line{v192TestOpen(t, "variants", 2, 1), v192TestInvalid(t, "@variant{", 3, 2), v192TestText(t, "Read.", "Read.", 3, 2), v192TestClose(t, 3, 2)}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v192TestDocument(t, tc.body...)
			if tc.errorLine != 0 {
				v192TestError(t, result, tc.errorLine, -1, "Границы блока не определены однозначно.")
			}
			v192TestRun(t, result, "current", nil)
		})
	}
}

func TestV192RegistrationAndHistory(t *testing.T) {
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
		if definition.Code != "V192" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" || definition.Message != "Перед первой ветвью @variant размещён материал." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V192 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v192TestDocument(t, v192TestExample(t)...)
			result.Document.DSLVersion = &version
			result.Lines[0] = v192TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v192TestRun(t, result, "current", nil)
		})
	}
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v192TestDocument(t, v192TestExample(t)...)
			for i := range result.Lines {
				if ending == "LF" {
					result.Lines[i].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[i].LineEnding = "\r\n"
				}
			}
			v192TestRun(t, result, "first", []v192Expected{{4, 1, 4, 15}})
			v192TestRun(t, result, "second", []v192Expected{{4, 1, 4, 15}})
			v192TestRun(t, v192TestContainer(t), "separate", nil)
		})
	}
	t.Run("case insensitive names", func(t *testing.T) {
		result := v192TestContainer(t, v192TestTag(t, "editor", "content", "Check.", 3, 2), v192TestTag(t, "question", "content", "Q?", 3, 2),
			v192TestTag(t, "answer", "content", "A", 3, 2), v192TestTag(t, "include", "identifier", "f", 3, 2),
			v192TestTag(t, "variant", "name", "A", 3, 2), v192TestText(t, "Read.", "Read.", 8, 3))
		for i := range result.Lines {
			if i < 2 {
				continue
			}
			line := &result.Lines[i]
			for j := range line.Elements {
				if line.Elements[j].ElementType == "tag" {
					raw := line.Elements[j].Raw
					line.Raw = strings.Replace(line.Raw, raw, strings.ToUpper(raw), 1)
					line.Elements[j].Raw = strings.ToUpper(raw)
				}
			}
		}
		v192TestRun(t, result, "current", []v192Expected{{5, 1, 5, 13}, {6, 1, 6, 10}})
	})
}

// v192TestExample сохраняет пример таблицы с номерами после двух вводных строк.
func v192TestExample(t *testing.T) []model.Line {
	t.Helper()
	return []model.Line{v192TestOpen(t, "variants", 2, 1), v192TestText(t, "Work in pairs.", "Work in pairs.", 3, 2),
		v192TestTag(t, "variant", "name", "A", 3, 2), v192TestText(t, "Read.", "Read.", 5, 3),
		v192TestTag(t, "variant", "name", "B", 3, 2), v192TestText(t, "Write.", "Write.", 7, 3), v192TestClose(t, 3, 2)}
}

// v192TestContainer заключает явные строки в variants, открытый на строке 3.
func v192TestContainer(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v192TestOpen(t, "variants", 2, 1)}, rows...)
	body = append(body, v192TestClose(t, 3, 2))
	return v192TestDocument(t, body...)
}

// v192TestFragment задаёт полноценное объявление фрагмента с обязательным ID.
func v192TestFragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v192TestLine(t, "@fragment f {", "block-start", parent, depth,
		v192TestElement(t, "tag", "@fragment", "fragment", 1, 10), v192TestElement(t, "identifier", "f", "f", 11, 12), v192TestElement(t, "block-open", "{", "{", 13, 14))
}

// v192TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v192TestDocument.
func v192TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v192TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v192TestLine(t, raw, "invalid", parent, depth, element)
}

// v192TestRun запускает штатный конвейер и проверяет только V192 текущего запуска.
func v192TestRun(t *testing.T, result *model.Result, source string, want []v192Expected) {
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
		if diagnostic.DiagnosticCode == "V192" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V192 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "Перед первой ветвью @variant размещён материал."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.endLine, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
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
}

// v192TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v192TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v192TestElement задаёт исходное и семантическое значения без преобразований.
func v192TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v192TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v192TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v192TestTag создаёт однострочное объявление с явно заданным параметром.
func v192TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v192TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v192TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v192TestLine(t, raw, "tag", parent, depth, elements...)
}

// v192TestOpen создаёт объявление блока без параметров.
func v192TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v192TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v192TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v192TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v192TestClose создаёт закрытие с заданным родителем.
func v192TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v192TestLine(t, "}", "block-end", parent, depth, v192TestElement(t, "block-close", "}", "}", 1, 2))
}

// v192TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v192TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v192TestLine(t, raw, "content", parent, depth, v192TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v192TestBlank задаёт обычную пустую строку без текстового элемента.
func v192TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v192TestLine(t, raw, "blank", parent, depth)
}

// v192TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v192TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v192TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v192TestLine(t, "# Lesson", "heading", 0, 0, v192TestElement(t, "heading-level", "#", "1", 1, 2), v192TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
		// Исходные ошибки принадлежат конкретной модели, а не повторно
		// используемому описанию строки в таблице тестов.
		lines[index].Elements = append([]model.Element{}, lines[index].Elements...)
		for element := range lines[index].Elements {
			lines[index].Elements[element].ErrorIDs = append([]string{}, lines[index].Elements[element].ErrorIDs...)
		}
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		for j, element := range result.Lines[i].Elements {
			if element.ElementType == "unparsed" {
				v192TestError(t, result, i+1, j, "Строка разобрана не полностью.")
			}
		}
	}
	return result

}

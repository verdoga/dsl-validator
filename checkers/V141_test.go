package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV141RegistrationAndVersions(t *testing.T) {
	registry := v141TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V141" {
			continue
		}
		count++
		if definition.Message != "Между @answer и @hint расположен другой тег." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V141 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v141TestSequence(t)
			result.Document.DSLVersion = &version
			before := v141TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v141TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV141ReferenceAndExample(t *testing.T) {
	// DSL 1.30: только версия, заголовок и шаг добавлены перед точными фрагментами.
	for _, tc := range []struct {
		name, fragment string
		tail           []model.Line
		want           [][3][3]int
	}{
		{"reference", "@question Where does Tom live?\n@answer London\n@hint Think of a capital.",
			[]model.Line{v141TestTag(t, "hint", "Think of a capital.", "content", 3)}, nil},
		{"example", "@question Where does Tom live?\n@answer London\n@note Read carefully.\n@hint Think of a capital.",
			[]model.Line{v141TestTag(t, "note", "Read carefully.", "content", 3), v141TestTag(t, "hint", "Think of a capital.", "content", 3)},
			[][3][3]int{{{7, 1, 6}, {6, 1, 6}, {5, 1, 8}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v141TestSequence(t, tc.tail...)
			var source []string
			for _, line := range result.Lines[3:] {
				source = append(source, line.Raw)
			}
			if strings.Join(source, "\n") != tc.fragment {
				t.Fatal("control fragment changed")
			}
			v141TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV141TwoSignificantConstructions(t *testing.T) {
	note := func() model.Line { return v141TestTag(t, "note", "Read carefully.", "content", 3) }
	hint := func() model.Line { return v141TestTag(t, "hint", "Think of a capital.", "content", 3) }
	for _, tc := range []struct {
		name string
		tail []model.Line
		want [][3][3]int
	}{
		{"blank lines", []model.Line{v141TestLine(t, "", "blank", 3), note(), v141TestLine(t, " \t ", "blank", 3), hint()},
			[][3][3]int{{{9, 1, 6}, {7, 1, 6}, {5, 1, 8}}}},
		{"NBSP is significant", []model.Line{v141TestText(t, "\u00a0", "\u00a0", 3), note(), hint()}, nil},
		{"text before middle", []model.Line{v141TestText(t, "Text.", "Text.", 3), note(), hint()}, nil},
		{"text after middle", []model.Line{note(), v141TestText(t, "Text.", "Text.", 3), hint()}, nil},
		{"escaped intermediate tag", []model.Line{v141TestText(t, `\@note Read.`, "@note Read.", 3), hint()}, nil},
		{"escaped hint", []model.Line{note(), v141TestText(t, `\@hint Think.`, "@hint Think.", 3)}, nil},
		{"tag mention", []model.Line{v141TestText(t, "See @note Read.", "See @note Read.", 3), hint()}, nil},
		{"two intermediate tags", []model.Line{note(), v141TestTag(t, "alt", "A city.", "content", 3), hint()}, nil},
		{"first hint ends search", []model.Line{hint(), note(), hint()}, nil},
		{"nearest answer only", []model.Line{v141TestTag(t, "answer", "Paris", "content", 3), note(), hint()},
			[][3][3]int{{{8, 1, 6}, {7, 1, 6}, {6, 1, 8}}}},
		{"EOF after answer", nil, nil},
		{"EOF after intermediate", []model.Line{note()}, nil},
		{"invalid intermediate", []model.Line{v141TestInvalid(t, "@unknown Text.", 3), hint()}, nil},
		{"invalid hint", []model.Line{note(), v141TestInvalid(t, "@hint\tThink.", 3)}, nil},
		{"unparsed whitespace is not skipped", []model.Line{v141TestInvalid(t, " ", 3), note(), hint()}, nil},
		{"partially parsed intermediate", []model.Line{v141TestLine(t, "@note ?", "tag", 3,
			v141TestElement(t, "tag", "@note", "note", 1), v141TestElement(t, "unparsed", "?", "", 7)), hint()}, nil},
		{"partially parsed editor is not skipped", []model.Line{v141TestLine(t, "@editor ?", "tag", 3,
			v141TestElement(t, "tag", "@editor", "editor", 1), v141TestElement(t, "unparsed", "?", "", 9)), note(), hint()}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v141TestRun(t, v141TestSequence(t, tc.tail...), "current", tc.want) })
	}
}

func TestV141BlocksAndComments(t *testing.T) {
	note := func() model.Line { return v141TestTag(t, "note", "Read.", "content", 3) }
	hint := func() model.Line { return v141TestTag(t, "hint", "Think.", "content", 3) }
	for _, tc := range []struct {
		name string
		tail []model.Line
		want [][3][3]int
	}{
		{"inline comments on both sides", []model.Line{v141TestTag(t, "editor", "First.", "content", 3), note(),
			v141TestTag(t, "editor", "Second.", "content", 3), hint()}, [][3][3]int{{{9, 1, 6}, {7, 1, 6}, {5, 1, 8}}}},
		{"only inline comment", []model.Line{v141TestTag(t, "editor", "Check.", "content", 3), hint()}, nil},
		{"block comment before middle", []model.Line{v141TestOpen(t, "editor", 3), v141TestText(t, "Check.", "Check.", 6),
			v141TestClose(t, 6), note(), hint()}, [][3][3]int{{{10, 1, 6}, {9, 1, 6}, {5, 1, 8}}}},
		{"block comment after middle", []model.Line{note(), v141TestOpen(t, "editor", 3), v141TestText(t, "Check.", "Check.", 7),
			v141TestClose(t, 7), hint()}, [][3][3]int{{{10, 1, 6}, {6, 1, 6}, {5, 1, 8}}}},
		{"comment pseudo tags and heading", []model.Line{v141TestOpen(t, "editor", 3),
			v141TestText(t, "@answer Paris", "@answer Paris", 6), v141TestText(t, "@note Text.", "@note Text.", 6),
			v141TestText(t, "@hint Think.", "@hint Think.", 6), v141TestText(t, "@task t", "@task t", 6),
			v141TestText(t, "### Heading", "### Heading", 6), v141TestClose(t, 6), note(), hint()},
			[][3][3]int{{{14, 1, 6}, {13, 1, 6}, {5, 1, 8}}}},
		{"only block comment", []model.Line{v141TestOpen(t, "editor", 3), v141TestText(t, "Check.", "Check.", 6),
			v141TestClose(t, 6), hint()}, nil},
		{"intermediate block", []model.Line{v141TestOpen(t, "note", 3), v141TestText(t, "Read.", "Read.", 6),
			v141TestClose(t, 6), hint()}, nil},
		{"block hint", []model.Line{note(), v141TestOpen(t, "hint", 3), v141TestText(t, "Think.", "Think.", 7),
			v141TestClose(t, 7)}, [][3][3]int{{{7, 1, 6}, {6, 1, 6}, {5, 1, 8}}}},
		{"block between intermediate and hint", []model.Line{note(), v141TestOpen(t, "text", 3),
			v141TestText(t, "Read.", "Read.", 7), v141TestClose(t, 7), hint()}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v141TestRun(t, v141TestSequence(t, tc.tail...), "current", tc.want) })
	}
	t.Run("block answer", func(t *testing.T) {
		result := v141TestDocument(t, v141TestTag(t, "question", "Where?", "content", 3), v141TestOpen(t, "answer", 3),
			v141TestText(t, "London", "London", 5), v141TestClose(t, 5), note(), hint())
		v141TestRun(t, result, "current", [][3][3]int{{{9, 1, 6}, {8, 1, 6}, {5, 1, 8}}})
	})
}

func TestV141BlankParentDoesNotChangeAdjacency(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3][3]int
	}{
		{"before intermediate", []model.Line{v141TestTag(t, "question", "Where?", "content", 3),
			v141TestTag(t, "answer", "London", "content", 3), v141TestLine(t, " \t", "blank", 3),
			v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3)},
			[][3][3]int{{{8, 1, 6}, {7, 1, 6}, {5, 1, 8}}}},
		{"after intermediate", []model.Line{v141TestTag(t, "question", "Where?", "content", 3),
			v141TestTag(t, "answer", "London", "content", 3), v141TestTag(t, "note", "Read.", "content", 3),
			v141TestLine(t, " \t", "blank", 3), v141TestTag(t, "hint", "Think.", "content", 3)},
			[][3][3]int{{{8, 1, 6}, {6, 1, 6}, {5, 1, 8}}}},
		{"inside answer", []model.Line{v141TestTag(t, "question", "Where?", "content", 3),
			v141TestOpen(t, "answer", 3), v141TestText(t, "London", "London", 5), v141TestLine(t, " \t", "blank", 5),
			v141TestClose(t, 5), v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3)},
			[][3][3]int{{{10, 1, 6}, {9, 1, 6}, {5, 1, 8}}}},
		{"inside nested answer content", []model.Line{v141TestTag(t, "question", "Where?", "content", 3),
			v141TestOpen(t, "answer", 3), v141TestOpen(t, "note", 5), v141TestText(t, "London", "London", 6),
			v141TestLine(t, " \t", "blank", 6), v141TestClose(t, 6), v141TestClose(t, 5),
			v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3)},
			[][3][3]int{{{12, 1, 6}, {11, 1, 6}, {5, 1, 8}}}},
		{"inside editor before intermediate", []model.Line{v141TestTag(t, "question", "Where?", "content", 3),
			v141TestTag(t, "answer", "London", "content", 3), v141TestOpen(t, "editor", 3),
			v141TestText(t, "Check.", "Check.", 6), v141TestLine(t, " \t", "blank", 6), v141TestClose(t, 6),
			v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3)},
			[][3][3]int{{{11, 1, 6}, {10, 1, 6}, {5, 1, 8}}}},
		{"inside editor after intermediate", []model.Line{v141TestTag(t, "question", "Where?", "content", 3),
			v141TestTag(t, "answer", "London", "content", 3), v141TestTag(t, "note", "Read.", "content", 3),
			v141TestOpen(t, "editor", 3), v141TestText(t, "Check.", "Check.", 7), v141TestLine(t, " \t", "blank", 7),
			v141TestClose(t, 7), v141TestTag(t, "hint", "Think.", "content", 3)},
			[][3][3]int{{{11, 1, 6}, {6, 1, 6}, {5, 1, 8}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v141TestBlankPair(t, tc.body, tc.want) })
	}
}

func TestV141BlanksPreserveStructuralStops(t *testing.T) {
	for _, afterMiddle := range []bool{false, true} {
		for _, name := range []string{"task", "endtask", "header", "step", "newpage", "variants", "variant", "heading", "text", "invalid", "foreign block"} {
			t.Run(name+" after middle="+strconv.FormatBool(afterMiddle), func(t *testing.T) {
				body := []model.Line{v141TestTag(t, "question", "Where?", "content", 3), v141TestTag(t, "answer", "London", "content", 3)}
				if afterMiddle {
					body = append(body, v141TestTag(t, "note", "Read.", "content", 3))
				}
				body = append(body, v141TestLine(t, " \t", "blank", 3))
				var boundary model.Line
				switch name {
				case "heading":
					boundary = v141TestLine(t, "### Next", "heading", 0,
						v141TestElement(t, "heading-level", "###", "3", 1), v141TestElement(t, "title", "Next", "Next", 5))
				case "text":
					boundary = v141TestText(t, "Text.", "Text.", 3)
				case "invalid":
					boundary = v141TestInvalid(t, " ", 3)
				case "variants":
					boundary = v141TestOpen(t, "variants", 3)
				case "foreign block":
					boundary = v141TestOpen(t, "note", 3)
				default:
					// Общий родитель проверяет саму границу, а не только SameLevel.
					boundary = v141TestTag(t, name, "", "", 3)
				}
				body = append(body, boundary)
				if boundary.LineType == "block-start" {
					body = append(body, v141TestClose(t, len(body)+3))
				}
				body = append(body, v141TestLine(t, " \t", "blank", 3), v141TestTag(t, "hint", "Think.", "content", 3))
				v141TestBlankPair(t, body, nil)
			})
		}
	}
	for _, name := range []string{"choice", "multichoice"} {
		t.Run(name+" inline editor remains significant", func(t *testing.T) {
			v141TestBlankPair(t, []model.Line{v141TestOpen(t, name, 3), v141TestTag(t, "answer", "London", "content", 4),
				v141TestLine(t, " \t", "blank", 4), v141TestTag(t, "editor", "Check.", "content", 4),
				v141TestLine(t, " \t", "blank", 4), v141TestTag(t, "hint", "Think.", "content", 4),
				v141TestText(t, "Paris", "Paris", 4), v141TestClose(t, 4)},
				[][3][3]int{{{9, 1, 6}, {7, 1, 8}, {5, 1, 8}}})
		})
		t.Run(name+" close remains significant", func(t *testing.T) {
			v141TestBlankPair(t, []model.Line{v141TestOpen(t, name, 3), v141TestTag(t, "answer", "London", "content", 4),
				v141TestTag(t, "note", "Read.", "content", 4), v141TestLine(t, " \t", "blank", 4),
				v141TestClose(t, 4), v141TestLine(t, " \t", "blank", 3), v141TestTag(t, "hint", "Think.", "content", 3)}, nil)
		})
	}
}

func TestV141BlockBoundsPreserveUncertainty(t *testing.T) {
	for _, name := range []string{"answer", "editor"} {
		for _, defect := range []string{"foreign content", "late descendant", "unclosed nested block", "nested close outside"} {
			t.Run(name+" "+defect, func(t *testing.T) {
				body := []model.Line{v141TestTag(t, "question", "Where?", "content", 3)}
				if name == "editor" {
					body = append(body, v141TestTag(t, "answer", "London", "content", 3))
				}
				opening := len(body) + 4
				body = append(body, v141TestOpen(t, name, 3), v141TestText(t, "London", "London", opening),
					v141TestLine(t, " \t", "blank", opening))
				nested := len(body) + 4
				switch defect {
				case "foreign content":
					body = append(body, v141TestText(t, "Foreign.", "Foreign.", 3))
				case "unclosed nested block", "nested close outside":
					body = append(body, v141TestOpen(t, "note", opening))
				}
				body = append(body, v141TestClose(t, opening), v141TestTag(t, "note", "Read.", "content", 3),
					v141TestTag(t, "hint", "Think.", "content", 3))
				switch defect {
				case "late descendant":
					body = append(body, v141TestText(t, "Late.", "Late.", opening+1))
				case "nested close outside":
					body = append(body, v141TestClose(t, nested))
				}
				v141TestBlankPair(t, body, nil)
			})
		}
	}
}

func TestV141ManyIndependentBlocks(t *testing.T) {
	body := []model.Line{v141TestTag(t, "question", "Where?", "content", 3)}
	for i := 0; i < 64; i++ {
		opening := len(body) + 4
		body = append(body, v141TestOpen(t, "answer", 3), v141TestText(t, "London", "London", opening), v141TestClose(t, opening),
			v141TestOpen(t, "editor", 3), v141TestText(t, "Check.", "Check.", opening+3), v141TestClose(t, opening+3),
			v141TestTag(t, "hint", "Think.", "content", 3))
	}
	body = append(body, v141TestOpen(t, "answer", 3), v141TestText(t, "Paris", "Paris", 453), v141TestClose(t, 453),
		v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3))
	v141TestRun(t, v141TestDocument(t, body...), "current", [][3][3]int{{{457, 1, 6}, {456, 1, 6}, {453, 1, 8}}})
}

// v141TestBlankPair меняет только родителя и глубину blank в паре моделей.
func v141TestBlankPair(t *testing.T, body []model.Line, want [][3][3]int) {
	t.Helper()
	before := v141TestSnapshot(t, v141TestDocument(t, body...))
	for _, root := range []bool{false, true} {
		name := "attached blank"
		if root {
			name = "root blank"
		}
		t.Run(name, func(t *testing.T) {
			var result model.Result
			if err := json.Unmarshal([]byte(before), &result); err != nil {
				t.Fatal(err)
			}
			if root {
				for i := range result.Lines {
					if result.Lines[i].LineType == "blank" {
						result.Lines[i].ParentLine, result.Lines[i].NestingLevel = nil, 0
					}
				}
			}
			v141TestRun(t, &result, "current", want)
		})
	}
}

func TestV141NestedContexts(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		result := v141TestDocument(t, v141TestTag(t, "task", "t", "identifier", 3),
			v141TestTag(t, "question", "Where?", "content", 4), v141TestTag(t, "answer", "London", "content", 4),
			v141TestTag(t, "note", "Read.", "content", 4), v141TestTag(t, "hint", "Think.", "content", 4),
			v141TestTag(t, "endtask", "", "", 4))
		v141TestRun(t, result, "current", [][3][3]int{{{8, 1, 6}, {7, 1, 6}, {6, 1, 8}}})
	})
	t.Run("task stage", func(t *testing.T) {
		result := v141TestDocument(t, v141TestTag(t, "task", "t", "identifier", 3), v141TestTag(t, "step", "Practice", "title", 4),
			v141TestTag(t, "question", "Where?", "content", 5), v141TestTag(t, "answer", "London", "content", 5),
			v141TestTag(t, "note", "Read.", "content", 5), v141TestTag(t, "hint", "Think.", "content", 5),
			v141TestTag(t, "endtask", "", "", 4))
		v141TestRun(t, result, "current", [][3][3]int{{{9, 1, 6}, {8, 1, 6}, {7, 1, 8}}})
	})
	for _, commonParent := range []bool{false, true} {
		t.Run("variant common parent="+strconv.FormatBool(commonParent), func(t *testing.T) {
			parentA, parentB := 5, 10
			if commonParent {
				parentA, parentB = 4, 4
			}
			result := v141TestDocument(t, v141TestOpen(t, "variants", 3), v141TestTag(t, "variant", "A", "name", 4),
				v141TestTag(t, "question", "Where?", "content", parentA), v141TestTag(t, "answer", "London", "content", parentA),
				v141TestTag(t, "note", "Read.", "content", parentA), v141TestTag(t, "hint", "Think.", "content", parentA),
				v141TestTag(t, "variant", "B", "name", 4), v141TestText(t, "Read.", "Read.", parentB), v141TestClose(t, 4))
			v141TestRun(t, result, "current", [][3][3]int{{{9, 1, 6}, {8, 1, 6}, {7, 1, 8}}})
		})
		t.Run("variant comments common parent="+strconv.FormatBool(commonParent), func(t *testing.T) {
			parentA, parentB := 5, 14
			if commonParent {
				parentA, parentB = 4, 4
			}
			result := v141TestDocument(t, v141TestOpen(t, "variants", 3), v141TestTag(t, "variant", "A", "name", 4),
				v141TestTag(t, "question", "Where?", "content", parentA), v141TestTag(t, "answer", "London", "content", parentA),
				v141TestOpen(t, "editor", parentA), v141TestText(t, "Check.", "Check.", 8), v141TestClose(t, 8),
				v141TestTag(t, "note", "Read.", "content", parentA), v141TestTag(t, "editor", "Again.", "content", parentA),
				v141TestTag(t, "hint", "Think.", "content", parentA), v141TestTag(t, "variant", "B", "name", 4),
				v141TestText(t, "Read.", "Read.", parentB), v141TestClose(t, 4))
			v141TestRun(t, result, "current", [][3][3]int{{{13, 1, 6}, {11, 1, 6}, {7, 1, 8}}})
		})
	}
	for _, name := range []string{"choice", "multichoice"} {
		t.Run("forbidden inline editor in "+name, func(t *testing.T) {
			result := v141TestDocument(t, v141TestOpen(t, name, 3), v141TestTag(t, "answer", "London", "content", 4),
				v141TestTag(t, "editor", "Check.", "content", 4), v141TestTag(t, "hint", "Think.", "content", 4),
				v141TestText(t, "Paris", "Paris", 4), v141TestClose(t, 4))
			v141TestRun(t, result, "current", [][3][3]int{{{7, 1, 6}, {6, 1, 8}, {5, 1, 8}}})
		})
		t.Run("forbidden block editor in "+name, func(t *testing.T) {
			result := v141TestDocument(t, v141TestOpen(t, name, 3), v141TestTag(t, "answer", "London", "content", 4),
				v141TestOpen(t, "editor", 4), v141TestText(t, "Check.", "Check.", 6), v141TestClose(t, 6),
				v141TestTag(t, "hint", "Think.", "content", 4), v141TestText(t, "Paris", "Paris", 4), v141TestClose(t, 4))
			v141TestRun(t, result, "current", nil)
		})
	}
	t.Run("matching", func(t *testing.T) {
		result := v141TestDocument(t, v141TestOpen(t, "matching", 3), v141TestText(t, "City", "City", 4),
			v141TestTag(t, "answer", "London", "content", 4), v141TestTag(t, "editor", "Check.", "content", 4),
			v141TestTag(t, "hint", "Think.", "content", 4), v141TestLine(t, "---", "separator", 4),
			v141TestText(t, "London", "London", 4), v141TestClose(t, 4))
		v141TestRun(t, result, "current", [][3][3]int{{{8, 1, 6}, {7, 1, 8}, {6, 1, 8}}})
	})
	t.Run("ordering", func(t *testing.T) {
		result := v141TestDocument(t, v141TestOpen(t, "ordering", 3), v141TestText(t, "First", "First", 4),
			v141TestTag(t, "answer", "1", "content", 4), v141TestTag(t, "editor", "Check.", "content", 4),
			v141TestTag(t, "hint", "Think.", "content", 4), v141TestText(t, "Second", "Second", 4),
			v141TestTag(t, "answer", "2", "content", 4), v141TestClose(t, 4))
		v141TestRun(t, result, "current", [][3][3]int{{{8, 1, 6}, {7, 1, 8}, {6, 1, 8}}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal template in "+name, func(t *testing.T) {
			raw := "@answer London"
			if name == "multifill" {
				raw = `\@answer London` // DSL 4.4.11: литеральный answer требует экранирования.
			}
			result := v141TestDocument(t, v141TestOpen(t, name, 3), v141TestText(t, raw, "@answer London", 4),
				v141TestText(t, "@note Read.", "@note Read.", 4), v141TestText(t, "@hint Think.", "@hint Think.", 4), v141TestClose(t, 4))
			v141TestRun(t, result, "current", nil)
		})
	}
}

func TestV141StructuralBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, argument     string
		kind               model.ElementType
		parent, hintParent int
	}{
		{"task", "other", "identifier", 3, 7},
		{"endtask", "", "", 4, 3},
		{"header", "Theory", "title", 3, 3},
		{"step", "Next", "title", 4, 7},
		{"newpage", "", "", 3, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v141TestDocument(t, v141TestTag(t, "task", "t", "identifier", 3),
				v141TestTag(t, "question", "Where?", "content", 4), v141TestTag(t, "answer", "London", "content", 4),
				v141TestTag(t, tc.name, tc.argument, tc.kind, tc.parent), v141TestTag(t, "hint", "Think.", "content", tc.hintParent))
			v141TestRun(t, result, "current", nil)
		})
	}
	for _, level := range []string{"#", "##", "###"} {
		t.Run("heading "+level, func(t *testing.T) {
			row := v141TestLine(t, level+" Next", "heading", 0, v141TestElement(t, "heading-level", level, strconv.Itoa(len(level)), 1),
				v141TestElement(t, "title", "Next", "Next", len(level)+2))
			result := v141TestSequence(t, row, v141TestTag(t, "hint", "Think.", "content", 6))
			// Поздние #/## сами ошибочны; V141 всё равно не пересекает заголовок.
			if level != "###" {
				v141TestParserError(t, result, 6, -1)
			}
			v141TestRun(t, result, "current", nil)
		})
	}
	t.Run("opening variants", func(t *testing.T) {
		result := v141TestSequence(t, v141TestOpen(t, "variants", 3), v141TestTag(t, "variant", "A", "name", 6),
			v141TestTag(t, "hint", "Think.", "content", 7), v141TestTag(t, "variant", "B", "name", 6),
			v141TestText(t, "Read.", "Read.", 9), v141TestClose(t, 6))
		v141TestRun(t, result, "current", nil)
	})
	// Общий parent=variants допустим по контракту; variant всё равно прерывает поиск.
	for _, withMiddle := range []bool{false, true} {
		t.Run("branch boundary after middle="+strconv.FormatBool(withMiddle), func(t *testing.T) {
			body := []model.Line{v141TestOpen(t, "variants", 3), v141TestTag(t, "variant", "A", "name", 4),
				v141TestTag(t, "question", "Where?", "content", 4), v141TestTag(t, "answer", "London", "content", 4)}
			if withMiddle {
				body = append(body, v141TestTag(t, "note", "Read.", "content", 4))
			}
			body = append(body, v141TestTag(t, "variant", "B", "name", 4), v141TestTag(t, "hint", "Think.", "content", 4), v141TestClose(t, 4))
			v141TestRun(t, v141TestDocument(t, body...), "current", nil)
		})
	}
	t.Run("closing container", func(t *testing.T) {
		result := v141TestDocument(t, v141TestOpen(t, "choice", 3), v141TestText(t, "Paris", "Paris", 4),
			v141TestTag(t, "answer", "London", "content", 4), v141TestTag(t, "editor", "Check.", "content", 4),
			v141TestClose(t, 4), v141TestTag(t, "hint", "Think.", "content", 3))
		v141TestRun(t, result, "current", nil)
	})
	t.Run("matching separator", func(t *testing.T) {
		result := v141TestDocument(t, v141TestOpen(t, "matching", 3), v141TestText(t, "City", "City", 4),
			v141TestTag(t, "answer", "London", "content", 4), v141TestLine(t, "---", "separator", 4),
			v141TestTag(t, "hint", "Think.", "content", 4), v141TestText(t, "London", "London", 4), v141TestClose(t, 4))
		v141TestRun(t, result, "current", nil)
	})
}

func TestV141UncertainBlocks(t *testing.T) {
	for _, name := range []string{"answer", "editor"} {
		t.Run("unclosed "+name, func(t *testing.T) {
			var result *model.Result
			opening := 5
			if name == "answer" {
				result = v141TestDocument(t, v141TestTag(t, "question", "Where?", "content", 3), v141TestOpen(t, "answer", 3),
					v141TestText(t, "London", "London", 5), v141TestTag(t, "note", "Read.", "content", 5),
					v141TestTag(t, "hint", "Think.", "content", 5))
			} else {
				opening = 6
				result = v141TestSequence(t, v141TestOpen(t, "editor", 3), v141TestText(t, "@note Read.", "@note Read.", 6),
					v141TestText(t, "@hint Think.", "@hint Think.", 6))
			}
			v141TestParserError(t, result, opening, -1)
			v141TestRun(t, result, "current", nil)
		})
		t.Run("ambiguous closing "+name, func(t *testing.T) {
			// CheckModel допускает несколько block-end с одним родителем при восстановлении.
			var result *model.Result
			bad := 7
			if name == "answer" {
				result = v141TestDocument(t, v141TestTag(t, "question", "Where?", "content", 3), v141TestOpen(t, "answer", 3),
					v141TestClose(t, 5), v141TestClose(t, 5), v141TestTag(t, "note", "Read.", "content", 3),
					v141TestTag(t, "hint", "Think.", "content", 3))
			} else {
				bad = 9
				result = v141TestSequence(t, v141TestOpen(t, "editor", 3), v141TestText(t, "Check.", "Check.", 6),
					v141TestClose(t, 6), v141TestClose(t, 6), v141TestTag(t, "note", "Read.", "content", 3),
					v141TestTag(t, "hint", "Think.", "content", 3))
			}
			v141TestParserError(t, result, bad, -1)
			v141TestRun(t, result, "current", nil)
		})
	}
	t.Run("nested closing is not answer closing", func(t *testing.T) {
		result := v141TestDocument(t, v141TestTag(t, "question", "Where?", "content", 3), v141TestOpen(t, "answer", 3),
			v141TestOpen(t, "note", 5), v141TestText(t, "Text.", "Text.", 6), v141TestClose(t, 6),
			v141TestText(t, "London", "London", 5), v141TestClose(t, 5), v141TestTag(t, "note", "Read.", "content", 3),
			v141TestTag(t, "hint", "Think.", "content", 3))
		v141TestRun(t, result, "current", [][3][3]int{{{12, 1, 6}, {11, 1, 6}, {5, 1, 8}}})
	})
}

func TestV141CoordinatesHistoryAndEndings(t *testing.T) {
	t.Run("case Unicode and indentation", func(t *testing.T) {
		answer := v141TestLine(t, "\t@ANSWER Лондон", "tag", 3, v141TestElement(t, "tag", "@ANSWER", "answer", 2),
			v141TestElement(t, "content", "Лондон", "Лондон", 10))
		note := v141TestLine(t, " @NoTe Я😀", "tag", 3, v141TestElement(t, "tag", "@NoTe", "note", 2),
			v141TestElement(t, "content", "Я😀", "Я😀", 8))
		hint := v141TestLine(t, "  @HINT Think of a capital.", "tag", 3, v141TestElement(t, "tag", "@HINT", "hint", 3),
			v141TestElement(t, "content", "Think of a capital.", "Think of a capital.", 9))
		result := v141TestDocument(t, v141TestTag(t, "question", "Where?", "content", 3), answer, note, hint)
		v141TestRun(t, result, "current", [][3][3]int{{{7, 3, 8}, {6, 2, 7}, {5, 2, 9}}})
	})
	t.Run("two independent templates", func(t *testing.T) {
		result := v141TestSequence(t, v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3),
			v141TestTag(t, "question", "Where?", "content", 3), v141TestTag(t, "answer", "Paris", "content", 3),
			v141TestTag(t, "note", "Read again.", "content", 3), v141TestTag(t, "hint", "Think again.", "content", 3))
		v141TestRun(t, result, "current", [][3][3]int{{{7, 1, 6}, {6, 1, 6}, {5, 1, 8}}, {{11, 1, 6}, {10, 1, 6}, {9, 1, 8}}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+" final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v141TestSequence(t, v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v141TestRun(t, result, "first", [][3][3]int{{{7, 1, 6}, {6, 1, 6}, {5, 1, 8}}})
				v141TestRun(t, v141TestSequence(t, v141TestTag(t, "hint", "Think.", "content", 3)), "other", nil)
				v141TestRun(t, result, "second", [][3][3]int{{{7, 1, 6}, {6, 1, 6}, {5, 1, 8}}})
			})
		}
	}
	t.Run("no answers", func(t *testing.T) {
		result := v141TestDocument(t, v141TestTag(t, "note", "Read.", "content", 3), v141TestTag(t, "hint", "Think.", "content", 3))
		v141TestRun(t, result, "current", nil)
	})
	t.Run("empty input", func(t *testing.T) {
		result := v141TestDocument(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v141TestRun(t, result, "current", nil)
	})
}

// v141TestSequence помещает вопрос на строку 4, однострочный answer на 5.
func v141TestSequence(t *testing.T, tail ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v141TestTag(t, "question", "Where does Tom live?", "content", 3),
		v141TestTag(t, "answer", "London", "content", 3)}, tail...)
	return v141TestDocument(t, body...)
}

// v141TestElement создаёт исходный элемент; ожидаемые диапазоны заданы отдельно.
func v141TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v141TestLine задаёт тип и родителя явно, без повторного разбора DSL.
func v141TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v141TestTag задаёт известный тег и явный тип аргумента.
func v141TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v141TestElement(t, "tag", raw, name, 1)}
	if argument != "" {
		elements = append(elements, v141TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v141TestLine(t, raw, "tag", parent, elements...)
}

// v141TestOpen задаёт распознанное открытие блока без параметров.
func v141TestOpen(t *testing.T, name string, parent int) model.Line {
	t.Helper()
	return v141TestLine(t, "@"+name+" {", "block-start", parent,
		v141TestElement(t, "tag", "@"+name, name, 1), v141TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v141TestClose закрывает явно указанный блок.
func v141TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v141TestLine(t, "}", "block-end", parent, v141TestElement(t, "block-close", "}", "}", 1))
}

// v141TestText задаёт исходный и семантический текст независимо.
func v141TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v141TestLine(t, raw, "content", parent, v141TestElement(t, "content", raw, value, 1))
}

// v141TestInvalid сохраняет неинтерпретируемую строку целиком.
func v141TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v141TestLine(t, raw, "invalid", parent, v141TestElement(t, "unparsed", raw, "", 1))
}

// v141TestDocument добавляет обязательное окружение и историю ошибочных элементов.
// Родители заданы вызывающим тестом; номера и глубина следуют этой модели.
func v141TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v141TestTag(t, "dsl-version", "1.2", "version", 0),
		v141TestLine(t, "# Lesson", "heading", 0, v141TestElement(t, "heading-level", "#", "1", 1),
			v141TestElement(t, "title", "Lesson", "Lesson", 3)),
		v141TestLine(t, "### Practice", "heading", 0, v141TestElement(t, "heading-level", "###", "3", 1),
			v141TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			if line.Elements[j].ElementType == "unparsed" {
				v141TestParserError(t, result, line.Number, j)
			}
		}
	}
	return result
}

// v141TestParserError добавляет исходную ошибку: index < 0 обозначает scope line.
// Код условный, поскольку каталог внешнего парсера не входит в репозиторий.
func v141TestParserError(t *testing.T, result *model.Result, number, index int) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
	start, end, scope := 1, utf8.RuneCountInString(line.Raw)+1, diagnostics.ScopeLine
	if index >= 0 {
		element := &line.Elements[index]
		element.ErrorIDs = append(element.ErrorIDs, id)
		start, end, scope = element.Start, element.End, diagnostics.ScopeElement
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: scope, Message: "Ошибка исходной разметки.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v141TestRegistry использует полный штатный каталог.
func v141TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v141TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v141TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v141TestLineReads считает обращения V141 к публичному построчному входу.
type v141TestLineReads struct {
	diagnostics.Lines
	reads int
}

func (lines *v141TestLineReads) Line(number int) (diagnostics.Line, bool) {
	lines.reads++
	return lines.Lines.Line(number)
}

// v141TestRun проверяет новые V141 после полного штатного конвейера.
// Ожидание содержит три диапазона [строка, начало, конец): hint, препятствие, answer.
func v141TestRun(t *testing.T, result *model.Result, processingID string, want [][3][3]int) []model.Diagnostic {
	t.Helper()
	before := v141TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v141TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v141TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	var reads int
	for i := range checks {
		if checks[i].Code == "V141" {
			check := checks[i].Check
			checks[i].Check = func(lines diagnostics.Lines, findings diagnostics.Findings) error {
				observed := &v141TestLineReads{Lines: lines}
				err := check(observed, findings)
				reads = observed.reads
				return err
			}
		}
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Запас на несколько проходов запрещает повторные обходы хвоста и предков.
	if limit := 16*len(result.Lines) + 16; reads > limit {
		t.Errorf("V141 read %d lines, want at most %d for %d source lines", reads, limit, len(result.Lines))
	}
	added := result.Diagnostics[oldCount:]
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == "V141" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V141 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0][0], Column: expected[0][1]},
			End: model.Position{Line: expected[0][0], Column: expected[0][2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != "Между @answer и @hint расположен другой тег." ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.Source != processingID || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 2 {
			t.Fatalf("V141 occurrence %d = %+v, want location %+v, no detail and exactly two related locations", i, entry, location)
		}
		for j, span := range expected[1:] {
			wantRelated := model.Location{Start: model.Position{Line: span[0], Column: span[1]},
				End: model.Position{Line: span[0], Column: span[2]}}
			if entry.RelatedLocations[j] != wantRelated {
				t.Fatalf("related %d = %+v, want %+v", j, entry.RelatedLocations[j], wantRelated)
			}
		}
		if !result.Document.HasErrors {
			t.Fatal("document error flag is missing")
		}
		// Контракт validator.errorFlags: отмечается начальная строка Location.
		if !result.Lines[expected[0][0]-1].HasErrors {
			t.Fatal("hint error flag is missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V141 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v141TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Исключаются только новые записи и производные флаги/ссылки.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element error links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v141TestSnapshot(t, &restored) != before {
		t.Fatal("source data or previous history changed")
	}
	return added
}

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

func TestV153Registration(t *testing.T) {
	registry := v153TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V153" {
			continue
		}
		count++
		if definition.Message != "Ответ в фигурных скобках записан вне содержимого @multifill." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V153 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || len(got) != 0 {
				t.Fatalf("unsupported version accepted: %v, %v", got, err)
			}
			result := v153TestDocument(t)
			result.Document.DSLVersion = &version
			before := v153TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("Prepare accepted version: %v, %v", checks, err)
			}
			if before != v153TestSnapshot(t, result) {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV153ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		invalid        bool
		want           [][5]int
	}{
		{"reference", "She _____ here.", false, nil},
		{"example", "She _____{lives} here.", true, [][5]int{{4, 10, 11, 5, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v153TestDocument(t, v153TestRow(t, tc.fragment, tc.fragment, tc.invalid, 3, 2))
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены версия, заголовок и необходимый контекст задания.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Unit 1\n@task 1\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v153TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV153Patterns(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][5]int
	}{
		{"unclosed", "_____{A", [][5]int{{4, 6, 7, 1, 6}}},
		{"empty", "_____{}", [][5]int{{4, 6, 7, 1, 6}}},
		{"only start", "_____{", [][5]int{{4, 6, 7, 1, 6}}},
		{"two placeholders", "_____{A} _____{B}", [][5]int{{4, 6, 7, 1, 6}, {4, 15, 16, 10, 15}}},
		{"adjacent placeholders", "_____{A}_____{B}", [][5]int{{4, 6, 7, 1, 6}, {4, 14, 15, 9, 14}}},
		{"nested shape skipped", "_____{x_____{y}", [][5]int{{4, 6, 7, 1, 6}}},
		{"alternatives", "_____{A}{B}", [][5]int{{4, 6, 7, 1, 6}}},
		{"alternative service value skipped", "_____{A}{_____{B}", [][5]int{{4, 6, 7, 1, 6}}},
		{"resume after alternatives", "_____{A}{_____{B} _____{C}", [][5]int{{4, 6, 7, 1, 6}, {4, 24, 25, 19, 24}}},
		{"no stack", "_____{a{b} _____{C}", [][5]int{{4, 6, 7, 1, 6}, {4, 17, 18, 12, 17}}},
		{"four underscores", "____{A}", nil},
		{"six underscores", "______{A}", nil},
		{"one underscore", "_{A}", nil},
		{"space before brace", "_____ {A}", nil},
		{"tab before brace", "_____\t{A}", nil},
		{"Unicode space before brace", "_____\u00a0{A}", nil},
		{"Unicode and indent", "  Я😀 _____{A}", [][5]int{{4, 11, 12, 6, 11}}},
		{"tab and combining mark", "\tе\u0301_____{A}", [][5]int{{4, 9, 10, 4, 9}}},
		{"uppercase answer", "_____{ЯABC}", [][5]int{{4, 6, 7, 1, 6}}},
		{"lowercase answer", "_____{яabc}", [][5]int{{4, 6, 7, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestRow(t, tc.raw, "", true, 3, 2)), "current", tc.want)
		})
	}
}

func TestV153Escaping(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		invalid          bool
		want             [][5]int
	}{
		{"literal notation", `\_____\{A\}`, "_____{A}", false, nil},
		{"literal underscores", `\_____{A}`, "", true, nil},
		{"escaped brace", `_____\{A\}`, "_____{A}", false, nil},
		{"paired backslashes", `\\_____{A}`, "", true, [][5]int{{4, 8, 9, 3, 8}}},
		{"odd backslashes", `\\\_____{A}`, "", true, nil},
		{"escaped longer run", `\______{A}`, "", true, nil},
		{"escaped closing", `_____{a\}_____{b}`, "", true, [][5]int{{4, 6, 7, 1, 6}}},
		{"paired slashes before closing", `_____{a\\}_____{B}`, "", true, [][5]int{{4, 6, 7, 1, 6}, {4, 16, 17, 11, 16}}},
		{"quotes do not hide closing", `_____{"}"_____{B}`, "", true, [][5]int{{4, 6, 7, 1, 6}, {4, 15, 16, 10, 15}}},
		{"escaped opening inside answer", `_____{a\{_____{B}`, "", true, [][5]int{{4, 6, 7, 1, 6}}},
		{"ordinary slash in answer", `_____{a\q_____{B}`, "", true, [][5]int{{4, 6, 7, 1, 6}}},
		{"trailing slash", `_____{A\`, "", true, [][5]int{{4, 6, 7, 1, 6}}},
		{"literal then active", `\_____\{A\} _____{B}`, "", true, [][5]int{{4, 18, 19, 13, 18}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestRow(t, tc.raw, tc.value, tc.invalid, 3, 2)), "current", tc.want)
		})
	}
}

func TestV153TextRanges(t *testing.T) {
	gap := v153TestElement(t, "placeholder", "_____", "", 5)
	gap.Value = nil
	for _, tc := range []struct {
		name string
		line model.Line
		want [][5]int
	}{
		{"content source", v153TestRow(t, "She _____{A}.", "She _____{A}.", false, 3, 2), [][5]int{{4, 10, 11, 5, 10}}},
		{"placeholder plus unparsed", v153TestLine(t, "She _____{A}.", "content", 3, 2,
			v153TestElement(t, "content", "She ", "She ", 1), gap, v153TestElement(t, "unparsed", "{A}", "", 10),
			v153TestElement(t, "content", ".", ".", 13)), [][5]int{{4, 10, 11, 5, 10}}},
		{"fully unparsed text", v153TestRow(t, "She _____{A}.", "", true, 3, 2), [][5]int{{4, 10, 11, 5, 10}}},
		{"fully unparsed question", v153TestRow(t, "@question _____{A}", "", true, 3, 2), [][5]int{{4, 16, 17, 11, 16}}},
		{"no merging through formatting", v153TestRow(t, "___<em>__</em>{A}", "___<em>__</em>{A}", false, 3, 2), nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v153TestRun(t, v153TestDocument(t, tc.line), "current", tc.want) })
	}
	t.Run("separate table cells", func(t *testing.T) {
		row := v153TestLine(t, "_____{A | _____{B", "content", 4, 3,
			v153TestElement(t, "unparsed", "_____{A", "", 1), v153TestElement(t, "unparsed", "_____{B", "", 11))
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "table", 3, 2), row, v153TestClose(t, 4, 3)), "current",
			[][5]int{{5, 6, 7, 1, 6}, {5, 16, 17, 11, 16}})
	})
	t.Run("HTML is not removed from Raw", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "text", 3, 2),
			v153TestRow(t, "_____<em>X</em>{A}", "_____<em>X</em>{A}", false, 4, 3), v153TestClose(t, 4, 3)), "current", nil)
	})
}

func TestV153FragmentedAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		ranges    [][2]int
		want      [][5]int
	}{
		{"whole answer", "_____{x _____{y}", [][2]int{{1, 17}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"uncovered space", "_____{x _____{y}", [][2]int{{1, 8}, {9, 17}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"adjacent fragments", "_____{x _____{y}", [][2]int{{1, 9}, {9, 17}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"split underscores and braces", "_____{x _____{y}", [][2]int{{1, 3}, {3, 6}, {6, 8}, {9, 11}, {11, 14}, {14, 17}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"uncovered tab", "_____{x\t_____{y}", [][2]int{{1, 8}, {9, 17}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"uncovered mixed whitespace", "_____{x \t  _____{y}", [][2]int{{1, 8}, {12, 20}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"uncovered Unicode spaces", "_____{x\u00a0\u2003_____{y}", [][2]int{{1, 8}, {10, 18}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"Unicode columns", "😀е\u0301_____{я\t_____{б}", [][2]int{{1, 11}, {12, 20}}, [][5]int{{4, 9, 10, 4, 9}}},
		{"unclosed fragmented answer", "_____{x _____{y", [][2]int{{1, 8}, {9, 16}}, [][5]int{{4, 6, 7, 1, 6}}},
		{"resume after closing", "_____{x _____{y} _____{z}", [][2]int{{1, 8}, {9, 17}, {18, 26}}, [][5]int{{4, 6, 7, 1, 6}, {4, 23, 24, 18, 23}}},
		{"escaped closing in fragment", `_____{x \}_____{y} _____{z}`, [][2]int{{1, 8}, {9, 10}, {10, 19}, {20, 28}}, [][5]int{{4, 6, 7, 1, 6}, {4, 25, 26, 20, 25}}},
		{"paired slashes before closing", `_____{x \\}_____{y}`, [][2]int{{1, 8}, {9, 10}, {10, 12}, {12, 20}}, [][5]int{{4, 6, 7, 1, 6}, {4, 17, 18, 12, 17}}},
		{"alternative service group", "_____{a}{x _____{y} _____{z}", [][2]int{{1, 11}, {12, 20}, {21, 29}}, [][5]int{{4, 6, 7, 1, 6}, {4, 26, 27, 21, 26}}},
		{"separated brace stays separate", "_____ {x _____{y}", [][2]int{{1, 6}, {7, 9}, {10, 18}}, [][5]int{{4, 15, 16, 10, 15}}},
		{"tab cannot join underscores", "___\t__{x}", [][2]int{{1, 4}, {5, 10}}, nil},
		{"Unicode space cannot join underscores", "___\u00a0__{x}", [][2]int{{1, 4}, {5, 10}}, nil},
		{"fragmented escaped run", `\_____{x}`, [][2]int{{1, 2}, {2, 4}, {4, 10}}, nil},
		{"fragmented long run", "______{x}", [][2]int{{1, 2}, {2, 10}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []rune(tc.raw)
			var elements []model.Element
			for _, span := range tc.ranges {
				elements = append(elements, v153TestElement(t, "unparsed", string(raw[span[0]-1:span[1]-1]), "", span[0]))
			}
			line := v153TestLine(t, tc.raw, "invalid", 3, 2, elements...)
			v153TestRun(t, v153TestDocument(t, line), "current", tc.want)
		})
	}
}

func TestV153TextBoundaries(t *testing.T) {
	t.Run("media source excluded but alt checked", func(t *testing.T) {
		line := v153TestLine(t, `@media image "_____{x}.png"`, "tag", 3, 2,
			v153TestElement(t, "tag", "@media", "media", 1),
			v153TestElement(t, "media-type", "image", "image", 8),
			v153TestElement(t, "source", `"_____{x}.png"`, "_____{x}.png", 14))
		v153TestRun(t, v153TestDocument(t, line, v153TestTag(t, "alt", "content", "_____{y}", 3, 2)),
			"current", [][5]int{{5, 11, 12, 6, 11}})
	})
	t.Run("table separator between whitespace remains boundary", func(t *testing.T) {
		line := v153TestLine(t, "_____{x\t|\u00a0_____{y}", "content", 4, 3,
			v153TestElement(t, "unparsed", "_____{x", "", 1),
			v153TestElement(t, "unparsed", "_____{y}", "", 11))
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "table", 3, 2), line, v153TestClose(t, 4, 3)),
			"current", [][5]int{{5, 6, 7, 1, 6}, {5, 16, 17, 11, 16}})
	})
	t.Run("zero width content does not reset answer", func(t *testing.T) {
		line := v153TestLine(t, "_____{x _____{y}", "invalid", 3, 2,
			v153TestElement(t, "unparsed", "_____{x", "", 1),
			v153TestElement(t, "content", "", "", 8),
			v153TestElement(t, "unparsed", "_____{y}", "", 9))
		v153TestRun(t, v153TestDocument(t, line), "current", [][5]int{{4, 6, 7, 1, 6}})
	})
}

func TestV153DeclarationsAndBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		want [][5]int
	}{
		{"question", [][5]int{{4, 16, 17, 11, 16}}}, {"note", [][5]int{{4, 12, 13, 7, 12}}},
		{"example", [][5]int{{4, 15, 16, 10, 15}}}, {"instr", [][5]int{{4, 13, 14, 8, 13}}},
		{"alt", [][5]int{{4, 11, 12, 6, 11}}}, {"hint", [][5]int{{4, 12, 13, 7, 12}}},
		{"multifill", [][5]int{{4, 17, 18, 12, 17}}}, {"QuEsTiOn", [][5]int{{4, 16, 17, 11, 16}}},
		{"editor", nil}, {"key", nil},
	} {
		t.Run("inline/"+tc.name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t,
				v153TestTag(t, tc.name, "content", "_____{A}", 3, 2)), "current", tc.want)
		})
	}
	t.Run("inline wordlist", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestTag(t, "wordlist", "content", "_____{A}; Other.", 3, 2)), "current", [][5]int{{4, 16, 17, 11, 16}})
	})
	for _, name := range []string{"example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint", "editor", "key", "multifill"} {
		t.Run("body/"+name, func(t *testing.T) {
			want := [][5]int{{5, 6, 7, 1, 6}}
			if name == "editor" || name == "key" || name == "multifill" {
				want = nil
			}
			v153TestRun(t, v153TestDocument(t, v153TestOpen(t, name, 3, 2),
				v153TestRow(t, "_____{A}", "_____{A}", false, 4, 3), v153TestClose(t, 4, 3)), "current", want)
		})
	}
	for _, name := range []string{"example", "wordlist", "table", "script", "text"} {
		t.Run("title/"+name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestOpenArgument(t, name, "title", "_____{A}", 3, 2),
				v153TestRow(t, "Text", "Text", false, 4, 3), v153TestClose(t, 4, 3)), "current", nil)
		})
	}
	t.Run("answer values", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestTag(t, "question", "content", "Q?", 3, 2),
			v153TestTag(t, "answer", "content", "_____{A}", 3, 2), v153TestTag(t, "question", "content", "Other?", 3, 2),
			v153TestOpen(t, "answer", 3, 2), v153TestRow(t, "_____{B}", "_____{B}", false, 7, 3), v153TestClose(t, 7, 3)), "current", nil)
	})
	t.Run("fragment text but not identifier", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestOpenArgument(t, "fragment", "identifier", "f", 3, 2),
			v153TestRow(t, "_____{A}", "", true, 4, 3), v153TestClose(t, 4, 3)), "current", [][5]int{{5, 6, 7, 1, 6}})
	})
}

func TestV153InteractiveContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		want [][5]int
	}{
		{"multifill", [][5]int{{4, 17, 18, 12, 17}}}, {"choice", [][5]int{{4, 14, 15, 9, 14}}},
		{"multichoice", [][5]int{{4, 19, 20, 14, 19}}}, {"matching", [][5]int{{4, 16, 17, 11, 16}}},
		{"ordering", [][5]int{{4, 16, 17, 11, 16}}},
	} {
		t.Run("instruction/"+tc.name, func(t *testing.T) {
			body := []model.Line{v153TestOpenArgument(t, tc.name, "content", "_____{A}", 3, 2),
				v153TestRow(t, "One", "One", false, 4, 3), v153TestRow(t, "Two", "Two", false, 4, 3)}
			if tc.name == "matching" {
				body = append(body, v153TestLine(t, "---", "separator", 4, 3),
					v153TestRow(t, "A", "A", false, 4, 3), v153TestRow(t, "B", "B", false, 4, 3))
			}
			body = append(body, v153TestClose(t, 4, 3))
			v153TestRun(t, v153TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, name := range []string{"choice", "multichoice"} {
		t.Run("options/"+name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestOpen(t, name, 3, 2), v153TestRow(t, "_____{A}", "", true, 4, 3),
				v153TestRow(t, "Two", "Two", false, 4, 3), v153TestClose(t, 4, 3)), "current", nil)
		})
	}
	for _, name := range []string{"matching", "ordering"} {
		t.Run("items versus options/"+name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestOpen(t, name, 3, 2), v153TestRow(t, "_____{A}", "", true, 4, 3),
				v153TestRow(t, "Other", "Other", false, 4, 3), v153TestLine(t, "---", "separator", 4, 3),
				v153TestRow(t, "_____{B}", "", true, 4, 3), v153TestRow(t, "Choice", "Choice", false, 4, 3), v153TestClose(t, 4, 3)),
				"current", [][5]int{{5, 6, 7, 1, 6}})
		})
	}
	t.Run("ordering without separator", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "ordering", 3, 2), v153TestRow(t, "_____{A}", "", true, 4, 3),
			v153TestRow(t, "Other", "Other", false, 4, 3), v153TestClose(t, 4, 3)), "current", [][5]int{{5, 6, 7, 1, 6}})
	})
	t.Run("hint has its own context in choice", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "choice", 3, 2), v153TestTag(t, "answer", "content", "A", 4, 3),
			v153TestOpen(t, "hint", 4, 3), v153TestRow(t, "_____{B}", "", true, 6, 4), v153TestClose(t, 6, 4),
			v153TestRow(t, "Other", "Other", false, 4, 3), v153TestClose(t, 4, 3)), "current", [][5]int{{7, 6, 7, 1, 6}})
	})
}

func TestV153ExcludedServiceFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind model.ElementType
	}{
		{"document-id", "identifier"}, {"section", "name"}, {"order", "number"}, {"resource-dir", "resource-path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v153TestRun(t, v153TestResult(t, v153TestTag(t, "dsl-version", "version", "1.2", 0, 0),
				v153TestTag(t, tc.name, tc.kind, "_____{A}", 0, 0), v153TestHeading(t), v153TestRow(t, "Text", "Text", false, 3, 1)), "current", nil)
		})
	}
	t.Run("metadata version field", func(t *testing.T) {
		// Ошибочный служебный аргумент остаётся unparsed, а не учебным текстом.
		line := v153TestLine(t, "@dsl-version 1.2 _____{A}", "tag", 0, 0,
			v153TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v153TestElement(t, "version", "1.2", "1.2", 14),
			v153TestElement(t, "unparsed", "_____{A}", "", 18))
		v153TestRun(t, v153TestResult(t, line, v153TestHeading(t), v153TestRow(t, "Text", "Text", false, 2, 1)), "current", nil)
	})
	t.Run("media path", func(t *testing.T) {
		line := v153TestLine(t, `@media image "_____{A}"`, "tag", 3, 2,
			v153TestElement(t, "tag", "@media", "media", 1), v153TestElement(t, "media-type", "image", "image", 8),
			v153TestElement(t, "source", `"_____{A}"`, "_____{A}", 14))
		v153TestRun(t, v153TestDocument(t, line), "current", nil)
	})
	for _, tc := range []struct {
		name string
		kind model.ElementType
	}{{"header", "title"}, {"step", "title"}, {"include", "identifier"}} {
		t.Run(tc.name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestTag(t, tc.name, tc.kind, "_____{A}", 3, 2)), "current", nil)
		})
	}
	t.Run("heading title", func(t *testing.T) {
		line := v153TestLine(t, "### _____{A}", "heading", 0, 0, v153TestElement(t, "heading-level", "###", "3", 1), v153TestElement(t, "title", "_____{A}", "_____{A}", 5))
		v153TestRun(t, v153TestDocument(t, line, v153TestRow(t, "Text", "Text", false, 4, 1)), "current", nil)
	})
	t.Run("task identifier", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestTag(t, "task", "identifier", "_____{A}", 2, 1), v153TestRow(t, "Text", "Text", false, 4, 2)), "current", nil)
	})
}

func TestV153NestedContextsAndBoundaries(t *testing.T) {
	t.Run("branch name excluded and deep text checked", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "variants", 2, 1), v153TestTag(t, "variant", "name", "_____{A}", 4, 2),
			v153TestTag(t, "task", "identifier", "a-1", 5, 3), v153TestTag(t, "step", "title", "Read", 6, 4),
			v153TestRow(t, "_____{B}", "", true, 7, 5), v153TestTag(t, "variant", "name", "B", 4, 2),
			v153TestRow(t, "Other", "Other", false, 9, 3), v153TestClose(t, 4, 2)), "current", [][5]int{{8, 6, 7, 1, 6}})
	})
	t.Run("nested hint in instruction", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestOpen(t, "instr", 3, 2), v153TestRow(t, "Read", "Read", false, 4, 3),
			v153TestOpen(t, "hint", 4, 3), v153TestRow(t, "_____{A}", "", true, 6, 4), v153TestClose(t, 6, 4), v153TestClose(t, 4, 3)),
			"current", [][5]int{{7, 6, 7, 1, 6}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal declarations/"+name, func(t *testing.T) {
			v153TestRun(t, v153TestDocument(t, v153TestOpen(t, name, 3, 2),
				v153TestRow(t, "@question _____{A}", "@question _____{A}", false, 4, 3),
				v153TestRow(t, "@multifill _____{B} {", "@multifill _____{B} {", false, 4, 3), v153TestClose(t, 4, 3)), "current", nil)
		})
	}
	t.Run("only body of multifill excluded", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestRow(t, "_____{A}", "", true, 3, 2), v153TestOpen(t, "multifill", 3, 2),
			v153TestRow(t, "_____{B}{C}", "_____{B}{C}", false, 5, 3), v153TestClose(t, 5, 3), v153TestRow(t, "_____{D}", "", true, 3, 2)),
			"current", [][5]int{{4, 6, 7, 1, 6}, {8, 6, 7, 1, 6}})
	})
	t.Run("separate lines reset answer", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestRow(t, "_____{A", "", true, 3, 2), v153TestRow(t, "_____{B}", "", true, 3, 2)),
			"current", [][5]int{{4, 6, 7, 1, 6}, {5, 6, 7, 1, 6}})
	})
	t.Run("underscores not joined across lines", func(t *testing.T) {
		v153TestRun(t, v153TestDocument(t, v153TestRow(t, "___", "", true, 3, 2), v153TestRow(t, "__{A}", "", true, 3, 2)), "current", nil)
	})
	t.Run("empty input", func(t *testing.T) { v153TestRun(t, v153TestResult(t), "current", nil) })
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v153TestDocument(t, v153TestRow(t, "_____{A} _____{B}", "", true, 3, 2))
			for i := range result.Lines {
				result.Lines[i].LineEnding = ending
			}
			result.Lines[3].LineEnding = ""
			want := [][5]int{{4, 6, 7, 1, 6}, {4, 15, 16, 10, 15}}
			v153TestRun(t, result, "first", want)
			v153TestRun(t, result, "second", want)
		})
	}
}

func TestV153PreservesV005(t *testing.T) {
	result := v153TestDocument(t, v153TestRow(t, "_____{A", "", true, 3, 2))
	v153TestRun(t, result, "current", [][5]int{{4, 6, 7, 1, 6}})
	count := 0
	for _, entry := range result.Diagnostics {
		if entry.Source != "current" || entry.DiagnosticCode != "V005" {
			continue
		}
		count++
		want := model.Location{Start: model.Position{Line: 4, Column: 6}, End: model.Position{Line: 4, Column: 7}}
		if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "Нарушено оформление фигурных скобок.\nконкретизация: Открывающая скобка не имеет закрывающей" {
			t.Fatalf("unexpected V005: %+v, location %+v", entry, entry.Location)
		}
	}
	if count != 1 {
		t.Fatalf("V005 count = %d, want 1", count)
	}
}

// v153TestDocument дополняет фрагмент обязательным контекстом по п. 1.30.
func v153TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v153TestTag(t, "dsl-version", "version", "1.2", 0, 0), v153TestHeading(t),
		v153TestTag(t, "task", "identifier", "1", 2, 1)}, body...)
	return v153TestResult(t, lines...)
}

// v153TestResult сохраняет явно заданную входную структуру и историю частичного разбора.
func v153TestResult(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, count := "1.2", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines:    append([]model.Line{}, lines...), Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
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
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Не разобран фрагмент исходной строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}})
		}
	}
	return result
}

// v153TestHeading задаёт заголовок с правильным контекстом и элементами.
func v153TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v153TestLine(t, "# Unit 1", "heading", 0, 0, v153TestElement(t, "heading-level", "#", "1", 1), v153TestElement(t, "title", "Unit 1", "Unit 1", 3))
}

// v153TestRow задаёт содержимое или явно неразобранный ошибочный фрагмент.
func v153TestRow(t *testing.T, raw, value string, invalid bool, parent, depth int) model.Line {
	t.Helper()
	if invalid {
		return v153TestLine(t, raw, "invalid", parent, depth, v153TestElement(t, "unparsed", raw, "", 1))
	}
	return v153TestLine(t, raw, "content", parent, depth, v153TestElement(t, "content", raw, value, 1))
}

// v153TestTag создаёт известное объявление с явно указанной ролью аргумента.
func v153TestTag(t *testing.T, name string, kind model.ElementType, text string, parent, depth int) model.Line {
	t.Helper()
	return v153TestLine(t, "@"+name+" "+text, "tag", parent, depth, v153TestElement(t, "tag", "@"+name, strings.ToLower(name), 1),
		v153TestElement(t, kind, text, text, len(name)+3))
}

// v153TestOpen создаёт объявление блока без аргумента.
func v153TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v153TestLine(t, "@"+name+" {", "block-start", parent, depth, v153TestElement(t, "tag", "@"+name, name, 1), v153TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v153TestOpenArgument сохраняет указанную тестом роль аргумента блока.
func v153TestOpenArgument(t *testing.T, name string, kind model.ElementType, text string, parent, depth int) model.Line {
	t.Helper()
	line := v153TestTag(t, name, kind, text, parent, depth)
	line.LineType = "block-start"
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.Elements = append(line.Elements, v153TestElement(t, "block-open", "{", "{", start))
	return line
}

// v153TestClose закрывает явно указанный фигурный блок.
func v153TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v153TestLine(t, "}", "block-end", parent, depth, v153TestElement(t, "block-close", "}", "}", 1))
}

// v153TestLine не классифицирует исходник и не восстанавливает родителей.
func v153TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v153TestElement измеряет только входной элемент; ожидания заданы числовыми литералами.
func v153TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v153TestRegistry использует полный штатный каталог регистраций.
func v153TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v153TestRun проверяет новые V153 полного конвейера и сохранность исходной модели.
// want содержит строку, границы основного и связанного диапазонов; концы исключены.
func v153TestRun(t *testing.T, result *model.Result, source string, want [][5]int) {
	t.Helper()
	before := v153TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v153TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v153TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V153" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V153 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		related := model.Location{Start: model.Position{Line: position[0], Column: position[3]}, End: model.Position{Line: position[0], Column: position[4]}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "Ответ в фигурных скобках записан вне содержимого @multifill." {
			t.Fatalf("occurrence %d = %+v, location %+v; want %+v, related %+v and exact metadata", i, entry, entry.Location, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
			t.Fatal("missing error flags")
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
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v153TestSnapshot(t, result)), &restored); err != nil {
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
	if v153TestSnapshot(t, &restored) != before {
		t.Fatal("source or prior history changed")
	}
}

// v153TestSnapshot сохраняет независимое представление модели для сравнения.
func v153TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

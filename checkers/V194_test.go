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

// v194TestExpected задаёт основной и единственный связанный диапазоны [start,end).
type v194TestExpected struct {
	startLine, start, endLine, end        int
	relatedLine, relatedStart, relatedEnd int
}

func TestV194Registration(t *testing.T) {
	definitions, err := v194TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V194" {
			continue
		}
		count++
		if definition.Message != "Ветвь @variant не содержит отображаемого материала." || definition.Severity != "error" ||
			definition.Scope != "block" || definition.Check == nil || definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V194 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v194TestEmptyModel(t, version)
			before := v194TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v194TestRegistry(t))
			if err == nil || checks != nil || v194TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV194ReferenceAndExample(t *testing.T) {
	// Префикс версии, ID и урока — единственное дополнение к фрагментам таблицы.
	for _, tc := range []struct {
		name, raw string
		rows      []model.Line
		want      []v194TestExpected
	}{
		{"reference", "@variants {\n@variant A\n@header Reading\n@variant B\nWrite.\n}", v194TestFirstRows(t,
			v194TestTagLine(t, "header", "title", "Reading", 5, 3)), nil},
		{"example", "@variants {\n@variant A\nRead.\n@variant B\n}", []model.Line{
			v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTextLine(t, "Read.", "Read.", 5, 3), v194TestTagLine(t, "variant", "name", "B", 4, 2), v194TestClose(t, 4, 2)},
			[]v194TestExpected{{7, 1, 7, 11, 8, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v194TestDocument(t, tc.rows...)
			var raw strings.Builder
			for _, line := range result.Lines[3:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.raw {
				t.Fatalf("source fragment changed: %q", raw.String())
			}
			v194TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV194EmptyBranchRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want v194TestExpected
	}{
		{"immediate next branch", nil, v194TestExpected{5, 1, 5, 11, 6, 1, 9}},
		{"empty physical line", []model.Line{v194TestLine(t, "", "blank", 5, 3)}, v194TestExpected{5, 1, 6, 1, 7, 1, 9}},
		{"space and tab line", []model.Line{v194TestLine(t, " \t", "blank", 5, 3)}, v194TestExpected{5, 1, 6, 3, 7, 1, 9}},
		{"single line comment", []model.Line{v194TestTagLine(t, "editor", "content", "Note.", 5, 3)}, v194TestExpected{5, 1, 6, 14, 7, 1, 9}},
		{"block comment", []model.Line{v194TestOpen(t, "editor", "", 5, 3), v194TestTextLine(t, "Visible only to editor.", "Visible only to editor.", 6, 4), v194TestClose(t, 6, 4)}, v194TestExpected{5, 1, 8, 2, 9, 1, 9}},
		{"fragment definition", []model.Line{v194TestFragment(t, "f", 5, 3), v194TestTextLine(t, "Read.", "Read.", 6, 4), v194TestClose(t, 6, 4)}, v194TestExpected{5, 1, 8, 2, 9, 1, 9}},
		{"single line answer", []model.Line{v194TestTagLine(t, "answer", "content", "Yes", 5, 3)}, v194TestExpected{5, 1, 6, 12, 7, 1, 9}},
		{"block answer", []model.Line{v194TestOpen(t, "answer", "", 5, 3), v194TestTextLine(t, "Yes", "Yes", 6, 4), v194TestClose(t, 6, 4)}, v194TestExpected{5, 1, 8, 2, 9, 1, 9}},
		{"service tags", []model.Line{v194TestTagLine(t, "endtask", "", "", 5, 3), v194TestTagLine(t, "speaking", "", "", 5, 3)}, v194TestExpected{5, 1, 7, 10, 8, 1, 9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, tc.body...)...), "current", []v194TestExpected{tc.want})
		})
	}
	t.Run("adjacent empty branches stay separate", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTagLine(t, "variant", "name", "B", 4, 2), v194TestClose(t, 4, 2)), "current",
			[]v194TestExpected{{5, 1, 5, 11, 6, 1, 9}, {6, 1, 6, 11, 7, 1, 2}})
	})
	t.Run("missing name does not hide a clear branch", func(t *testing.T) {
		rows := v194TestFirstRows(t)
		rows[1] = v194TestTagLine(t, "variant", "", "", 4, 2)
		v194TestRun(t, v194TestDocument(t, rows...), "current", []v194TestExpected{{5, 1, 5, 9, 6, 1, 9}})
	})
	t.Run("identical names do not merge branches", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1),
			v194TestTagLine(t, "variant", "name", "A", 4, 2), v194TestTagLine(t, "variant", "name", "A", 4, 2), v194TestClose(t, 4, 2)),
			"current", []v194TestExpected{{5, 1, 5, 11, 6, 1, 9}, {6, 1, 6, 11, 7, 1, 2}})
	})
	t.Run("case indentation and boundary tag only", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "VaRiAnTs", "", 3, 1),
			v194TestLine(t, "  @VaRiAnT A", "tag", 4, 2, v194TestElement(t, "tag", "@VaRiAnT", "variant", 3, 11), v194TestElement(t, "name", "A", "A", 12, 13)),
			v194TestLine(t, " \t@VaRiAnT B", "tag", 4, 2, v194TestElement(t, "tag", "@VaRiAnT", "variant", 3, 11), v194TestElement(t, "name", "B", "B", 12, 13)),
			v194TestTextLine(t, "Write.", "Write.", 6, 3), v194TestClose(t, 4, 2)), "current", []v194TestExpected{{5, 3, 5, 13, 6, 3, 11}})
	})
	t.Run("Unicode name and indented closing", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTextLine(t, "Read.", "Read.", 5, 3), v194TestTagLine(t, "variant", "name", "Я🙂", 4, 2),
			v194TestLine(t, " \t}\t ", "block-end", 4, 2, v194TestElement(t, "block-close", "}", "}", 3, 4))),
			"current", []v194TestExpected{{7, 1, 7, 12, 8, 3, 4}})
	})
}

func TestV194DisplayedMaterial(t *testing.T) {
	for _, tc := range []struct {
		tag   string
		kind  model.ElementType
		value string
	}{
		{"header", "title", "Reading"}, {"task", "identifier", "a"}, {"step", "title", "Part"},
		{"instr", "content", "Read."}, {"example", "content", "A"}, {"wordlist", "content", "A"},
		{"note", "content", "Read."}, {"question", "content", "Ready?"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestTagLine(t, tc.tag, tc.kind, tc.value, 5, 3))...), "current", nil)
		})
	}
	t.Run("media", func(t *testing.T) {
		line := v194TestLine(t, "@media audio 1", "tag", 5, 3, v194TestElement(t, "tag", "@media", "media", 1, 7),
			v194TestElement(t, "media-type", "audio", "audio", 8, 13), v194TestElement(t, "source", "1", "1", 14, 15))
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, line)...), "current", nil)
	})
	for _, tag := range []string{"table", "script", "text", "key", "alt", "hint", "multifill", "choice", "multichoice", "matching", "ordering"} {
		t.Run("recognized empty block/"+tag, func(t *testing.T) {
			// Собственная пустота или неправильное расположение этих конструкций
			// проверяются отдельно и не превращают ветвь в пустую по таблице V194.
			v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestOpen(t, tag, "", 5, 3), v194TestClose(t, 6, 4))...), "current", nil)
		})
	}
	t.Run("recognized malformed instr", func(t *testing.T) {
		line := v194TestLine(t, `@instr \q`, "invalid", 5, 3, v194TestElement(t, "tag", "@instr", "instr", 1, 7), v194TestUnparsed(t, `\q`, 8, 10))
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, line)...), "current", nil)
	})
	for _, tc := range []struct{ raw, value string }{
		{"Read.", "Read."}, {".", "."}, {"Я🙂", "Я🙂"}, {`\@variant`, "@variant"}, {`\# Heading`, "# Heading"},
		{"\u00a0", "\u00a0"}, {"\u200b", "\u200b"},
	} {
		t.Run("ordinary text/"+tc.raw, func(t *testing.T) {
			v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestTextLine(t, tc.raw, tc.value, 5, 3))...), "current", nil)
		})
	}
	t.Run("allowed nested contents", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestOpen(t, "instr", "", 5, 3),
			v194TestTextLine(t, "Read.", "Read.", 6, 4), v194TestClose(t, 6, 4))...), "current", nil)
	})
}

func TestV194Includes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contents []model.Line
		want     []v194TestExpected
	}{
		{"empty fragment", []model.Line{v194TestLine(t, "", "blank", 10, 2)}, []v194TestExpected{{5, 1, 6, 11, 7, 1, 9}}},
		{"nonempty fragment", []model.Line{v194TestTextLine(t, "Read.", "Read.", 10, 2)}, nil},
		{"displayed fragment tag", []model.Line{v194TestTagLine(t, "note", "content", "Read.", 10, 2)}, nil},
		{"uncertain fragment contents", []model.Line{v194TestInvalid(t, "@unknown", 10, 2)}, nil},
		{"forbidden recursive include is uncertain", []model.Line{v194TestTagLine(t, "include", "identifier", "f", 10, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := v194TestFirstRows(t, v194TestTagLine(t, "include", "identifier", "f", 5, 3))
			rows = append(rows, v194TestFragment(t, "f", 3, 1))
			rows = append(rows, tc.contents...)
			rows = append(rows, v194TestClose(t, 10, 2))
			v194TestRun(t, v194TestDocument(t, rows...), "current", tc.want)
		})
	}
	t.Run("empty definition before use", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFragment(t, "f", 3, 1), v194TestLine(t, "", "blank", 4, 2), v194TestClose(t, 4, 2),
			v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 7, 2),
			v194TestTagLine(t, "include", "identifier", "f", 8, 3), v194TestTagLine(t, "variant", "name", "B", 7, 2),
			v194TestTextLine(t, "Write.", "Write.", 10, 3), v194TestClose(t, 7, 2)), "current", []v194TestExpected{{8, 1, 9, 11, 10, 1, 9}})
	})
	for _, tc := range []struct{ reference, definition string }{{"f", "F"}, {"Σ", "ς"}, {"часть-1", "ЧАСТЬ-1"}} {
		t.Run("ID case/"+tc.reference, func(t *testing.T) {
			rows := v194TestFirstRows(t, v194TestTagLine(t, "include", "identifier", tc.reference, 5, 3))
			rows = append(rows, v194TestFragment(t, tc.definition, 3, 1), v194TestTextLine(t, "Read.", "Read.", 10, 2), v194TestClose(t, 10, 2))
			v194TestRun(t, v194TestDocument(t, rows...), "current", nil)
		})
	}
	t.Run("definition in another branch is not material there", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTagLine(t, "include", "identifier", "f", 5, 3), v194TestTagLine(t, "variant", "name", "B", 4, 2),
			v194TestFragment(t, "f", 7, 3), v194TestTextLine(t, "Read.", "Read.", 8, 4), v194TestClose(t, 8, 4), v194TestClose(t, 4, 2)),
			"current", []v194TestExpected{{7, 1, 10, 2, 11, 1, 2}})
	})
	t.Run("same empty fragment in two branches", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTagLine(t, "include", "identifier", "f", 5, 3), v194TestTagLine(t, "variant", "name", "B", 4, 2),
			v194TestTagLine(t, "include", "identifier", "F", 7, 3), v194TestClose(t, 4, 2),
			v194TestFragment(t, "f", 3, 1), v194TestLine(t, "", "blank", 10, 2), v194TestClose(t, 10, 2)), "current",
			[]v194TestExpected{{5, 1, 6, 11, 7, 1, 9}, {7, 1, 8, 11, 9, 1, 2}})
	})
	t.Run("missing definition", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestTagLine(t, "include", "identifier", "absent", 5, 3))...), "current", nil)
	})
	t.Run("unresolved include does not suppress the next branch", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1),
			v194TestTagLine(t, "variant", "name", "A", 4, 2), v194TestTagLine(t, "include", "identifier", "absent", 5, 3),
			v194TestTagLine(t, "variant", "name", "B", 4, 2), v194TestClose(t, 4, 2)),
			"current", []v194TestExpected{{7, 1, 7, 11, 8, 1, 2}})
	})
	t.Run("duplicate definition ID", func(t *testing.T) {
		rows := v194TestFirstRows(t, v194TestTagLine(t, "include", "identifier", "f", 5, 3))
		rows = append(rows, v194TestFragment(t, "f", 3, 1), v194TestLine(t, "", "blank", 10, 2), v194TestClose(t, 10, 2),
			v194TestFragment(t, "F", 3, 1), v194TestLine(t, "", "blank", 13, 2), v194TestClose(t, 13, 2))
		v194TestRun(t, v194TestDocument(t, rows...), "current", nil)
	})
}

func TestV194Uncertainty(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
	}{
		{"unknown declaration", v194TestInvalid(t, "@unknown", 5, 3)},
		{"unparsed content", v194TestLine(t, `\q`, "content", 5, 3, v194TestUnparsed(t, `\q`, 1, 3))},
		{"unparsed possible boundary", v194TestInvalid(t, "@variant\u00a0B", 5, 3)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, tc.line)...), "current", nil)
		})
	}
	t.Run("only the affected branch is suppressed", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestInvalid(t, "@unknown", 5, 3), v194TestTagLine(t, "variant", "name", "B", 4, 2), v194TestClose(t, 4, 2)),
			"current", []v194TestExpected{{7, 1, 7, 11, 8, 1, 2}})
	})
	for _, tag := range []string{"editor", "fragment"} {
		t.Run("uncertainty in excluded body/"+tag, func(t *testing.T) {
			opening := v194TestOpen(t, "editor", "", 5, 3)
			if tag == "fragment" {
				opening = v194TestFragment(t, "f", 5, 3)
			}
			v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, opening, v194TestInvalid(t, `\q`, 6, 4), v194TestClose(t, 6, 4))...),
				"current", []v194TestExpected{{5, 1, 8, 2, 9, 1, 9}})
		})
	}
	t.Run("branch name errors do not make its contents uncertain", func(t *testing.T) {
		rows := v194TestFirstRows(t)
		rows[1] = v194TestLine(t, `@variant A\q`, "invalid", 4, 2, v194TestElement(t, "tag", "@variant", "variant", 1, 9),
			v194TestElement(t, "name", "A", "A", 10, 11), v194TestUnparsed(t, `\q`, 11, 13))
		v194TestRun(t, v194TestDocument(t, rows...), "current", []v194TestExpected{{5, 1, 5, 13, 6, 1, 9}})
	})
	t.Run("uncertainty before first branch is outside its body", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestInvalid(t, "@unknown", 4, 2),
			v194TestTagLine(t, "variant", "name", "A", 4, 2), v194TestTagLine(t, "variant", "name", "B", 4, 2),
			v194TestTextLine(t, "Write.", "Write.", 7, 3), v194TestClose(t, 4, 2)), "current", []v194TestExpected{{6, 1, 6, 11, 7, 1, 9}})
	})
}

func TestV194UncertainIncludes(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
	}{
		{"missing ID", v194TestTagLine(t, "include", "", "", 5, 3)},
		{"invalid ID", v194TestTagLine(t, "include", "identifier", "f?", 5, 3)},
		{"missing identifier element", v194TestLine(t, "@include f", "tag", 5, 3, v194TestElement(t, "tag", "@include", "include", 1, 9))},
		{"tab separator", v194TestLine(t, "@include\tf", "tag", 5, 3, v194TestElement(t, "tag", "@include", "include", 1, 9), v194TestElement(t, "identifier", "f", "f", 10, 11))},
		{"extra parameter", v194TestLine(t, "@include f extra", "invalid", 5, 3, v194TestElement(t, "tag", "@include", "include", 1, 9),
			v194TestElement(t, "identifier", "f", "f", 10, 11), v194TestUnparsed(t, "extra", 12, 17))},
		{"partial ID", v194TestLine(t, `@include f\q`, "invalid", 5, 3, v194TestElement(t, "tag", "@include", "include", 1, 9),
			v194TestElement(t, "identifier", "f", "f", 10, 11), v194TestUnparsed(t, `\q`, 11, 13))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := v194TestFirstRows(t, tc.line)
			rows = append(rows, v194TestFragment(t, "f", 3, 1), v194TestLine(t, "", "blank", 10, 2), v194TestClose(t, 10, 2))
			v194TestRun(t, v194TestDocument(t, rows...), "current", nil)
		})
	}
	t.Run("block include does not provide ordinary text", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestOpen(t, "include", "", 5, 3),
			v194TestTextLine(t, "Read.", "Read.", 6, 4), v194TestClose(t, 6, 4))...), "current", nil)
	})
	t.Run("unclosed definition", func(t *testing.T) {
		rows := v194TestFirstRows(t, v194TestTagLine(t, "include", "identifier", "f", 5, 3))
		rows = append(rows, v194TestFragment(t, "f", 3, 1))
		v194TestRun(t, v194TestDocument(t, rows...), "current", nil)
	})
	t.Run("malformed definition declaration", func(t *testing.T) {
		rows := v194TestFirstRows(t, v194TestTagLine(t, "include", "identifier", "f", 5, 3))
		rows = append(rows, v194TestLine(t, "@fragment f extra {", "block-start", 3, 1,
			v194TestElement(t, "tag", "@fragment", "fragment", 1, 10), v194TestElement(t, "identifier", "f", "f", 11, 12),
			v194TestUnparsed(t, "extra", 13, 18), v194TestElement(t, "block-open", "{", "{", 19, 20)), v194TestClose(t, 10, 2))
		v194TestRun(t, v194TestDocument(t, rows...), "current", nil)
	})
}

func TestV194BranchBoundaries(t *testing.T) {
	t.Run("pseudo boundary inside comment is ignored", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestOpen(t, "editor", "", 5, 3),
			v194TestTextLine(t, "@variant X", "@variant X", 6, 4), v194TestClose(t, 6, 4))...),
			"current", []v194TestExpected{{5, 1, 8, 2, 9, 1, 9}})
	})
	t.Run("endtask does not terminate branch", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestTagLine(t, "endtask", "", "", 5, 3),
			v194TestTextLine(t, "Read.", "Read.", 5, 3))...), "current", nil)
	})
	t.Run("nested closing does not terminate branch", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestFirstRows(t, v194TestOpen(t, "editor", "", 5, 3),
			v194TestTextLine(t, "Note.", "Note.", 6, 4), v194TestClose(t, 6, 4), v194TestTextLine(t, "Read.", "Read.", 5, 3))...), "current", nil)
	})
	t.Run("previous branch task does not create material", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTagLine(t, "task", "identifier", "a", 5, 3), v194TestTagLine(t, "variant", "name", "B", 4, 2),
			v194TestTagLine(t, "speaking", "", "", 7, 3), v194TestTagLine(t, "endtask", "", "", 7, 3), v194TestClose(t, 4, 2)),
			"current", []v194TestExpected{{7, 1, 9, 9, 10, 1, 2}})
	})
	t.Run("unclosed last branch does not hide earlier empty branch", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
			v194TestTagLine(t, "variant", "name", "B", 4, 2)), "current", []v194TestExpected{{5, 1, 5, 11, 6, 1, 9}})
	})
	t.Run("independent containers", func(t *testing.T) {
		rows := v194TestFirstRows(t)
		rows = append(rows, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "C", 9, 2),
			v194TestTagLine(t, "variant", "name", "D", 9, 2), v194TestTextLine(t, "Write.", "Write.", 11, 3), v194TestClose(t, 9, 2))
		v194TestRun(t, v194TestDocument(t, rows...), "current", []v194TestExpected{{5, 1, 5, 11, 6, 1, 9}, {10, 1, 10, 11, 11, 1, 9}})
	})
	for _, outerParent := range []bool{false, true} {
		t.Run(fmt.Sprintf("content parent variants %t", outerParent), func(t *testing.T) {
			parent, depth := 5, 3
			if outerParent {
				parent, depth = 4, 2
			}
			result := v194TestDocument(t, v194TestFirstRows(t, v194TestTagLine(t, "editor", "content", "Note.", parent, depth))...)
			v194TestRun(t, result, "current", []v194TestExpected{{5, 1, 6, 14, 7, 1, 9}})
		})
	}
}

func TestV194EmptyLiteralEndingsAndHistory(t *testing.T) {
	v194TestRun(t, v194TestEmptyModel(t, "1.2"), "current", nil)
	t.Run("no branches", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestClose(t, 4, 2)), "current", nil)
	})
	t.Run("variant outside variants", func(t *testing.T) {
		v194TestRun(t, v194TestDocument(t, v194TestTagLine(t, "variant", "name", "A", 3, 1)), "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo branches/"+tag, func(t *testing.T) {
			v194TestRun(t, v194TestDocument(t, v194TestOpen(t, tag, "", 3, 1),
				v194TestTextLine(t, "@variants {", "@variants {", 4, 2), v194TestTextLine(t, "@variant A", "@variant A", 4, 2),
				v194TestTextLine(t, "@variant B", "@variant B", 4, 2), v194TestClose(t, 4, 2)), "current", nil)
		})
	}
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", ending), func(t *testing.T) {
			result := v194TestDocument(t, v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2),
				v194TestTextLine(t, "Read.", "Read.", 5, 3), v194TestTagLine(t, "variant", "name", "B", 4, 2), v194TestClose(t, 4, 2))
			if ending != "" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v194TestRun(t, result, "first", []v194TestExpected{{7, 1, 7, 11, 8, 1, 2}})
			v194TestRun(t, result, "second", []v194TestExpected{{7, 1, 7, 11, 8, 1, 2}})
		})
	}
}

// v194TestFirstRows задаёт ветвь A с явным содержимым и непустую ветвь B.
func v194TestFirstRows(t *testing.T, body ...model.Line) []model.Line {
	t.Helper()
	rows := append([]model.Line{v194TestOpen(t, "variants", "", 3, 1), v194TestTagLine(t, "variant", "name", "A", 4, 2)}, body...)
	second := 6 + len(body)
	return append(rows, v194TestTagLine(t, "variant", "name", "B", 4, 2), v194TestTextLine(t, "Write.", "Write.", second, 3), v194TestClose(t, 4, 2))
}

// v194TestFragment задаёт блочное определение с identifier, а не инструкцией.
func v194TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	line := v194TestTagLine(t, "fragment", "identifier", id, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v194TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v194TestInvalid сохраняет неразобранную строку с исходной ошибкой парсера.
func v194TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v194TestLine(t, raw, "invalid", parent, depth, v194TestUnparsed(t, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v194TestRegistry создаёт реестр из полного штатного каталога регистраций.
func v194TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v194TestEmptyModel создаёт модель без исходных строк и с явно указанной версией.
func v194TestEmptyModel(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v194TestDocument добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v194TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v194TestEmptyModel(t, "1.2")
	title, id := "Lesson", "v194"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v194TestTagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v194TestTagLine(t, "document-id", "identifier", "v194", 0, 0),
		v194TestLine(t, "# Lesson", "heading", 0, 0, v194TestElement(t, "heading-level", "#", "1", 1, 2), v194TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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
			errorID := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{errorID}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: errorID, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v194TestLine сохраняет явно заданные данные физической строки и её родителя.
func v194TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v194TestElement задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v194TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v194TestUnparsed задаёт фрагмент без значения; историю добавляет v194TestDocument.
func v194TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v194TestTagLine создаёт объявление с явно заданным типом аргумента.
func v194TestTagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v194TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v194TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v194TestLine(t, raw, "tag", parent, depth, elements...)
}

// v194TestTextLine задаёт обычную строку с независимо указанным семантическим значением.
func v194TestTextLine(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v194TestLine(t, raw, "content", parent, depth, v194TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v194TestOpen задаёт блочное объявление с необязательной инструкцией.
func v194TestOpen(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v194TestTagLine(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v194TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v194TestClose задаёт отдельную закрывающую скобку с явным родителем.
func v194TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v194TestLine(t, "}", "block-end", parent, depth, v194TestElement(t, "block-close", "}", "}", 1, 2))
}

// v194TestRun запускает неизменённую выборку штатного каталога и проверяет новые V194.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v194TestRun(t *testing.T, result *model.Result, source string, want []v194TestExpected) {
	t.Helper()
	before := v194TestSnapshot(t, result)
	checks, err := validator.Prepare(result, v194TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v194TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("unexpected new source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V194" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V194 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.startLine, Column: expected.start}, End: model.Position{Line: expected.endLine, Column: expected.end}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Message != "Ветвь @variant не содержит отображаемого материала." {
			t.Errorf("occurrence = %+v, want range %+v and exact V194 metadata", entry, location)
		}
		related := []model.Location{{Start: model.Position{Line: expected.relatedLine, Column: expected.relatedStart}, End: model.Position{Line: expected.relatedLine, Column: expected.relatedEnd}}}
		if !slices.Equal(entry.RelatedLocations, related) {
			t.Errorf("related = %+v, want %+v", entry.RelatedLocations, related)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Error("block diagnostic must not create element links")
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.startLine-1].HasErrors {
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
	if err := json.Unmarshal([]byte(v194TestSnapshot(t, result)), &actual); err != nil {
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
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v194TestSnapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v194TestSnapshot сохраняет модель для проверки неизменности исходника и истории.
func v194TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

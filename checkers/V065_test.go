package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV065Registration(t *testing.T) {
	definitions, err := v065TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V065" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У @endtask нет активного задания для завершения." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V065 definitions = %d, want 1", count)
	}
}

func TestV065ReferenceAndExample(t *testing.T) {
	// Перед неизменёнными фрагментами добавлены только версия и заголовок.
	t.Run("reference", func(t *testing.T) {
		result := v065TestDocument(t,
			v065TestNamed(t, "task", "identifier", "practice", 2, 1),
			v065TestNamed(t, "step", "title", "Read", 3, 2), v065TestText(t, "Read.", "Read.", 4, 3),
			v065TestNamed(t, "step", "title", "Write", 3, 2), v065TestText(t, "Write.", "Write.", 6, 3),
			v065TestTag(t, "endtask", 2, 1))
		v065TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@task practice\n@step Read\nRead.\n@step Write\nWrite.\n@endtask")
		v065TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v065TestDocument(t, v065TestTag(t, "endtask", 2, 1))
		v065TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@endtask")
		v065TestRun(t, result, "current", [][3]int{{3, 1, 9}})
	})
}

func TestV065TaskState(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"no structural events", []model.Line{
			v065TestText(t, "Read.", "Read.", 2, 1), v065TestLine(t, "", "blank", 2, 1),
		}, nil},
		{"task closes", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestTag(t, "endtask", 2, 1),
		}, nil},
		{"second endtask is extra", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestTag(t, "endtask", 2, 1), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{6, 1, 9}}},
		{"each extra endtask reports", []model.Line{
			v065TestTag(t, "endtask", 2, 1), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{3, 1, 9}, {4, 1, 9}}},
		{"new task replaces previous", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestNamed(t, "task", "identifier", "b", 2, 1), v065TestText(t, "Write.", "Write.", 5, 2),
			v065TestTag(t, "endtask", 2, 1), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{8, 1, 9}}},
		{"missing ID still opens task", []model.Line{
			v065TestInvalid(t, "@task", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestTag(t, "endtask", 2, 1),
		}, nil},
		{"missing ID replaces previous task", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestInvalid(t, "@task", 2, 1), v065TestText(t, "Write.", "Write.", 5, 2),
			v065TestTag(t, "endtask", 2, 1), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{8, 1, 9}}},
		{"EOF closes task without finding", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
		}, nil},
		{"blank and comments preserve task", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestLine(t, " \t", "blank", 3, 2), v065TestNamed(t, "editor", "content", "Check.", 3, 2),
			v065TestOpen(t, "editor", 3, 2), v065TestText(t, "Comment.", "Comment.", 7, 3),
			v065TestClose(t, 7, 3), v065TestTag(t, "endtask", 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV065ResetEvents(t *testing.T) {
	for _, tc := range []struct {
		name  string
		event model.Line
	}{
		{"header", v065TestNamed(t, "header", "title", "Theory", 2, 1)},
		{"newpage", v065TestTag(t, "newpage", 0, 0)},
		{"parsed newpage with bad tail", v065TestLine(t, "@newpage extra", "tag", 2, 1,
			v065TestElement(t, "tag", "@newpage", "newpage", 1), v065TestUnparsed(t, "extra", 10))},
		{"unparsed header", v065TestInvalid(t, "@HEADER", 2, 1)},
		{"unparsed newpage", v065TestInvalid(t, "@NEWPAGE extra", 2, 1)},
		{"unparsed variants without block", v065TestInvalid(t, "@variants", 2, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t,
				v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
				tc.event, v065TestText(t, "Text.", "Text.", 2, 1), v065TestTag(t, "endtask", 2, 1)),
				"current", [][3]int{{7, 1, 9}})
		})
	}
	for _, tc := range []struct {
		marker string
		want   [][3]int
	}{
		{"###", [][3]int{{7, 1, 9}}}, {"#", nil}, {"##", nil},
	} {
		t.Run("heading "+tc.marker, func(t *testing.T) {
			result := v065TestDocument(t,
				v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
				v065TestHeading(t, tc.marker, "Next"), v065TestText(t, "Text.", "Text.", 5, 1),
				v065TestTag(t, "endtask", 5, 1))
			if tc.marker != "###" {
				// DSL 2.1.3–2.1.4: ошибочное размещение сохраняется в модели;
				// уровни 1 и 2 не входят в перечень событий сброса V065.
				v065TestParserError(t, result, 5, -1, "Недопустимое размещение заголовка.")
				if tc.marker == "#" {
					result.Document.Metadata.Title = nil
				} else {
					subtitle := "Next"
					result.Document.Metadata.Subtitle = &subtitle
				}
			}
			v065TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV065RawRecognition(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want [][3]int
	}{
		{"@endtask", [][3]int{{3, 1, 9}}},
		{" \t@ENDTASK", [][3]int{{3, 3, 11}}},
		{"@endtask extra", [][3]int{{3, 1, 9}}},
		{"@endtask\textra", [][3]int{{3, 1, 9}}},
		{"@endtask\u00a0extra", [][3]int{{3, 1, 9}}},
		{"@endtask{", [][3]int{{3, 1, 9}}},
		{"@endtaskExtra", nil}, {"@endtask-name", nil}, {"@endtask}", nil},
		{"@endtas", nil}, {"@endtask\u200bextra", nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t, v065TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	t.Run("parsed case and indentation", func(t *testing.T) {
		v065TestRun(t, v065TestDocument(t, v065TestLine(t, " \t@ENDTASK", "tag", 2, 1,
			v065TestElement(t, "tag", "@ENDTASK", "endtask", 3))), "current", [][3]int{{3, 3, 11}})
	})
	for _, parsed := range []bool{false, true} {
		t.Run("bad endtask tail parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
			end := v065TestInvalid(t, "@endtask extra", 2, 1)
			if parsed {
				end = v065TestLine(t, "@endtask extra", "tag", 2, 1,
					v065TestElement(t, "tag", "@endtask", "endtask", 1), v065TestUnparsed(t, "extra", 10))
			}
			v065TestRun(t, v065TestDocument(t, end), "without-task", [][3]int{{3, 1, 9}})
			v065TestRun(t, v065TestDocument(t,
				v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
				end, v065TestTag(t, "endtask", 2, 1)), "with-task", [][3]int{{6, 1, 9}})
		})
	}
	for _, kind := range []model.LineType{"invalid", "tag"} {
		t.Run("task without tag element "+string(kind), func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t,
				v065TestLine(t, "@TASK\tid", kind, 2, 1, v065TestUnparsed(t, "@TASK\tid", 1)),
				v065TestText(t, "Read.", "Read.", 3, 2), v065TestTag(t, "endtask", 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@endtask`, "@endtask"}, {`\\@endtask`, `\@endtask`},
		{"Text @endtask", "Text @endtask"}, {"\u00a0@endtask", "\u00a0@endtask"},
	} {
		t.Run("text "+tc.raw, func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t, v065TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("mention in note", func(t *testing.T) {
		v065TestRun(t, v065TestDocument(t, v065TestNamed(t, "note", "content", "Mention @endtask.", 2, 1)), "current", nil)
	})
}

func TestV065FigureBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"closing text preserves task", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestOpen(t, "text", 3, 2),
			v065TestText(t, "Read.", "Read.", 4, 3), v065TestClose(t, 4, 3), v065TestTag(t, "endtask", 2, 1),
		}, nil},
		{"nested task does not open outer task", []model.Line{
			v065TestOpen(t, "text", 2, 1), v065TestInvalid(t, "@task a", 3, 2),
			v065TestText(t, "Read.", "Read.", 3, 2), v065TestClose(t, 3, 2), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{7, 1, 9}}},
		{"nested endtask does not close outer task", []model.Line{
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestOpen(t, "text", 3, 2),
			v065TestInvalid(t, "@endtask", 4, 3), v065TestText(t, "Read.", "Read.", 4, 3),
			v065TestClose(t, 4, 3), v065TestTag(t, "endtask", 2, 1),
		}, nil},
		{"nested endtask does not report without task", []model.Line{
			v065TestOpen(t, "text", 2, 1), v065TestInvalid(t, "@endtask", 3, 2),
			v065TestText(t, "Read.", "Read.", 3, 2), v065TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, raw := range []string{"@header Next", "@newpage", "### Next", "@variants", "@variant B"} {
		t.Run("nested reset "+raw, func(t *testing.T) {
			v065TestRun(t, v065TestDocument(t,
				v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestOpen(t, "text", 3, 2),
				v065TestInvalid(t, raw, 4, 3), v065TestText(t, "Read.", "Read.", 4, 3),
				v065TestClose(t, 4, 3), v065TestTag(t, "endtask", 2, 1)), "current", nil)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		for _, active := range []bool{false, true} {
			t.Run(tag+"/active="+strconv.FormatBool(active), func(t *testing.T) {
				body := []model.Line{v065TestOpen(t, tag, 2, 1)}
				parent, depth := 3, 2
				if active {
					body = []model.Line{v065TestNamed(t, "task", "identifier", "a", 2, 1),
						v065TestText(t, "Read.", "Read.", 3, 2), v065TestOpen(t, tag, 3, 2)}
					parent, depth = 5, 3
				}
				for _, raw := range []string{"@task hidden", "@endtask", "@variant B", "@newpage", "### Next"} {
					body = append(body, v065TestText(t, raw, raw, parent, depth))
				}
				body = append(body, v065TestClose(t, parent, depth), v065TestTag(t, "endtask", 2, 1))
				var want [][3]int
				if !active {
					want = [][3]int{{10, 1, 9}}
				}
				v065TestRun(t, v065TestDocument(t, body...), "current", want)
			})
		}
	}
}

func TestV065Variants(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"external task never resumes", []model.Line{
			v065TestNamed(t, "task", "identifier", "outer", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2),
			v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 5, 2),
			v065TestText(t, "Read.", "Read.", 6, 3), v065TestNamed(t, "variant", "name", "B", 5, 2),
			v065TestText(t, "Write.", "Write.", 8, 3), v065TestClose(t, 5, 2), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{11, 1, 9}}},
		{"branch without task", []model.Line{
			v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 3, 2),
			v065TestTag(t, "endtask", 4, 3), v065TestText(t, "Read.", "Read.", 4, 3),
			v065TestNamed(t, "variant", "name", "B", 3, 2), v065TestText(t, "Write.", "Write.", 7, 3),
			v065TestClose(t, 3, 2),
		}, [][3]int{{5, 1, 9}}},
		{"each branch has its own task", []model.Line{
			v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 3, 2),
			v065TestNamed(t, "task", "identifier", "a", 4, 3), v065TestText(t, "Read.", "Read.", 5, 4),
			v065TestTag(t, "endtask", 4, 3), v065TestNamed(t, "variant", "name", "B", 3, 2),
			v065TestNamed(t, "task", "identifier", "b", 8, 3), v065TestText(t, "Write.", "Write.", 9, 4),
			v065TestTag(t, "endtask", 8, 3), v065TestClose(t, 3, 2),
		}, nil},
		{"next variant closes previous task", []model.Line{
			v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 3, 2),
			v065TestNamed(t, "task", "identifier", "a", 4, 3), v065TestText(t, "Read.", "Read.", 5, 4),
			v065TestNamed(t, "variant", "name", "B", 3, 2), v065TestTag(t, "endtask", 7, 3),
			v065TestText(t, "Write.", "Write.", 7, 3), v065TestClose(t, 3, 2),
		}, [][3]int{{8, 1, 9}}},
		{"closing variants closes last task", []model.Line{
			v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 3, 2),
			v065TestText(t, "Read.", "Read.", 4, 3), v065TestNamed(t, "variant", "name", "B", 3, 2),
			v065TestNamed(t, "task", "identifier", "b", 6, 3), v065TestText(t, "Write.", "Write.", 7, 4),
			v065TestClose(t, 3, 2), v065TestTag(t, "endtask", 2, 1),
		}, [][3]int{{10, 1, 9}}},
		{"literal variant does not switch branch", []model.Line{
			v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 3, 2),
			v065TestNamed(t, "task", "identifier", "a", 4, 3), v065TestOpen(t, "editor", 5, 4),
			v065TestText(t, "@variant B", "@variant B", 6, 5), v065TestClose(t, 6, 5),
			v065TestTag(t, "endtask", 4, 3), v065TestText(t, "Read.", "Read.", 4, 3),
			v065TestNamed(t, "variant", "name", "B", 3, 2), v065TestText(t, "Write.", "Write.", 11, 3),
			v065TestClose(t, 3, 2),
		}, nil},
		{"events before first branch are excluded", []model.Line{
			v065TestOpen(t, "variants", 2, 1), v065TestInvalid(t, "@task a", 3, 2),
			v065TestInvalid(t, "@endtask", 3, 2), v065TestNamed(t, "variant", "name", "A", 3, 2),
			v065TestTag(t, "endtask", 6, 3), v065TestText(t, "Read.", "Read.", 6, 3),
			v065TestNamed(t, "variant", "name", "B", 3, 2), v065TestText(t, "Write.", "Write.", 9, 3),
			v065TestClose(t, 3, 2),
		}, [][3]int{{7, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, shared := range []bool{false, true} {
				t.Run("shared parent="+strconv.FormatBool(shared), func(t *testing.T) {
					result := v065TestDocument(t, tc.body...)
					if shared {
						v065TestSharedParents(t, result)
					}
					v065TestRun(t, result, "current", tc.want)
				})
			}
		})
	}
}

func TestV065ErroneousBranchEvents(t *testing.T) {
	t.Run("variants and variant recovered from unparsed", func(t *testing.T) {
		// Частичный разбор сохранил block-open и родительские связи.
		// У variant отсутствует обязательное имя, но событие сохраняется.
		v065TestRun(t, v065TestDocument(t,
			v065TestLine(t, "@VaRiAnTs{", "block-start", 2, 1,
				v065TestUnparsed(t, "@VaRiAnTs", 1), v065TestElement(t, "block-open", "{", "{", 10)),
			v065TestInvalid(t, "@variant", 3, 2),
			v065TestNamed(t, "task", "identifier", "a", 3, 2), v065TestText(t, "Read.", "Read.", 5, 3),
			v065TestInvalid(t, "@VARIANT\tB", 3, 2),
			v065TestTag(t, "endtask", 3, 2), v065TestText(t, "Write.", "Write.", 3, 2),
			v065TestClose(t, 3, 2)), "current", [][3]int{{8, 1, 9}})
	})
	for _, tag := range []string{"header", "newpage"} {
		t.Run("reset in branch "+tag, func(t *testing.T) {
			result := v065TestDocument(t,
				v065TestOpen(t, "variants", 2, 1), v065TestNamed(t, "variant", "name", "A", 3, 2),
				v065TestNamed(t, "task", "identifier", "a", 4, 3), v065TestText(t, "Read.", "Read.", 5, 4),
				v065TestInvalid(t, "@"+tag+"\textra", 4, 3), v065TestTag(t, "endtask", 4, 3),
				v065TestText(t, "Text.", "Text.", 4, 3), v065TestNamed(t, "variant", "name", "B", 3, 2),
				v065TestText(t, "Write.", "Write.", 10, 3), v065TestClose(t, 3, 2))
			// Табуляция вместо разделителя ошибочна по DSL 1.29;
			// newpage дополнительно запрещён в ветви по DSL 4.9.27.
			// Уточнение V065 сохраняет структурное событие в обоих случаях.
			v065TestRun(t, result, "current", [][3]int{{8, 1, 9}})
		})
	}
}

func TestV065EndingsHistoryAndEmpty(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v065TestDocument(t, v065TestTag(t, "endtask", 2, 1))
			for i := range result.Lines {
				if ending != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = ending
				}
			}
			v065TestRun(t, result, "first", [][3]int{{3, 1, 9}})
			v065TestRun(t, result, "second", [][3]int{{3, 1, 9}})
		})
	}
	t.Run("task does not leak to next document", func(t *testing.T) {
		v065TestRun(t, v065TestDocument(t,
			v065TestNamed(t, "task", "identifier", "a", 2, 1), v065TestText(t, "Read.", "Read.", 3, 2)), "current", nil)
		v065TestRun(t, v065TestDocument(t, v065TestTag(t, "endtask", 2, 1)), "current", [][3]int{{3, 1, 9}})
	})
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v065TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v065TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v065TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			result.Document.DSLVersion = &version
			before := v065TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v065TestRegistry(t))
			if err == nil || checks != nil || v065TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v065TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v065TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v065TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v065TestHeading(t, "#", "Lesson"),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v065TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v065TestSharedParents задаёт поддерживаемый моделью вариант родительства:
// содержимое ветви связано с variants, не с однострочным variant.
func v065TestSharedParents(t *testing.T, result *model.Result) {
	t.Helper()
	for i := range result.Lines {
		line := &result.Lines[i]
		if line.ParentLine == nil {
			continue
		}
		parent := result.Lines[*line.ParentLine-1]
		for _, element := range parent.Elements {
			if element.ElementType == "tag" && element.Value != nil && *element.Value == "variant" {
				line.ParentLine = parent.ParentLine
				break
			}
		}
		line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
	}
}

// v065TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v065TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		element := &line.Elements[index]
		scope, start, end = diagnostics.ScopeElement, element.Start, element.End
		element.ErrorIDs = []string{id}
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
}

func v065TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v065TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v065TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v065TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v065TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v065TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v065TestLine(t, raw, "invalid", parent, depth, v065TestUnparsed(t, raw, 1))
}

func v065TestTag(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v065TestLine(t, "@"+tag, "tag", parent, depth, v065TestElement(t, "tag", "@"+tag, tag, 1))
}

func v065TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v065TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v065TestElement(t, "tag", "@"+tag, tag, 1), v065TestElement(t, kind, value, value, len(tag)+3))
}

func v065TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v065TestLine(t, marker+" "+title, "heading", 0, 0,
		v065TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v065TestElement(t, "title", title, title, len(marker)+2))
}

func v065TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v065TestLine(t, raw, "content", parent, depth, v065TestElement(t, "content", raw, value, 1))
}

func v065TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v065TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v065TestElement(t, "tag", "@"+tag, tag, 1), v065TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v065TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v065TestLine(t, "}", "block-end", parent, depth, v065TestElement(t, "block-close", "}", "}", 1))
}

func v065TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v065TestRun выполняет полный каталог и сравнивает новые V065 с явными
// тройками строка/начало/конец. Правые границы исключаются.
func v065TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v065TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v065TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v065TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V065" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V065 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "У @endtask нет активного задания для завершения." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V065 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v065TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v065TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v065TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v065TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var source strings.Builder
	for _, line := range result.Lines {
		source.WriteString(line.Raw)
		source.WriteString(string(line.LineEnding))
	}
	if source.String() != want {
		t.Fatalf("source = %q, want %q", source.String(), want)
	}
}

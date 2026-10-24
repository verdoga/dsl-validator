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

func TestV072Registration(t *testing.T) {
	definitions, err := v072TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V072" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "@speaking объявлен в задании повторно." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V072 definitions = %d, want 1", count)
	}
}

func TestV072ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Только версия и заголовок добавлены перед неизменённым эталоном.
		result := v072TestDocument(t,
			v072TestNamed(t, "task", "identifier", "discussion", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestNamed(t, "step", "title", "Prepare", 3, 2), v072TestText(t, "Read.", "Read.", 5, 3),
			v072TestNamed(t, "step", "title", "Discuss", 3, 2), v072TestText(t, "Speak.", "Speak.", 7, 3))
		v072TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@task discussion\n@speaking\n@step Prepare\nRead.\n@step Discuss\nSpeak.")
		v072TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		// Только версия и заголовок добавлены перед неизменённым примером.
		result := v072TestDocument(t,
			v072TestNamed(t, "task", "identifier", "discussion", 2, 1),
			v072TestTag(t, "speaking", 3, 2), v072TestTag(t, "speaking", 3, 2),
			v072TestNamed(t, "step", "title", "Prepare", 3, 2), v072TestText(t, "Read.", "Read.", 6, 3),
			v072TestNamed(t, "step", "title", "Discuss", 3, 2), v072TestText(t, "Speak.", "Speak.", 8, 3))
		v072TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@task discussion\n@speaking\n@speaking\n@step Prepare\nRead.\n@step Discuss\nSpeak.")
		v072TestRun(t, result, "current", [][6]int{{5, 1, 10, 4, 1, 10}})
	})
}

func TestV072TaskState(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"no speaking", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestText(t, "Read.", "Read.", 3, 2),
		}, nil},
		{"one speaking", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestText(t, "Speak.", "Speak.", 3, 2),
		}, nil},
		{"all repeats refer to first", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestTag(t, "speaking", 3, 2), v072TestTag(t, "speaking", 3, 2),
			v072TestText(t, "Speak.", "Speak.", 3, 2),
		}, [][6]int{{5, 1, 10, 4, 1, 10}, {6, 1, 10, 4, 1, 10}}},
		{"late first counts", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestText(t, "Read.", "Read.", 3, 2),
			v072TestTag(t, "speaking", 3, 2), v072TestTag(t, "speaking", 3, 2),
		}, [][6]int{{6, 1, 10, 5, 1, 10}}},
		{"new task clears first", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestText(t, "Speak.", "Speak.", 3, 2),
			v072TestNamed(t, "task", "identifier", "b", 2, 1),
			v072TestTag(t, "speaking", 6, 2), v072TestTag(t, "speaking", 6, 2),
			v072TestText(t, "Discuss.", "Discuss.", 6, 2),
		}, [][6]int{{8, 1, 10, 7, 1, 10}}},
		{"missing task ID", []model.Line{
			v072TestInvalid(t, "@task", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2),
		}, [][6]int{{5, 1, 10, 4, 1, 10}}},
		{"outside task ignored", []model.Line{
			v072TestTag(t, "speaking", 2, 1), v072TestTag(t, "speaking", 2, 1),
			v072TestNamed(t, "task", "identifier", "a", 2, 1),
			v072TestTag(t, "speaking", 5, 2), v072TestText(t, "Speak.", "Speak.", 5, 2),
		}, nil},
		{"step preserves first", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestNamed(t, "step", "title", "Read", 3, 2), v072TestText(t, "Read.", "Read.", 5, 3),
			v072TestTag(t, "speaking", 5, 3),
		}, [][6]int{{7, 1, 10, 4, 1, 10}}},
		{"blank and inline comment preserve first", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestLine(t, " \t", "blank", 3, 2), v072TestNamed(t, "editor", "content", "Check.", 3, 2),
			v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2),
		}, [][6]int{{7, 1, 10, 4, 1, 10}}},
		{"case and indentation", []model.Line{
			v072TestLine(t, "@TASK a", "tag", 2, 1,
				v072TestElement(t, "tag", "@TASK", "task", 1), v072TestElement(t, "identifier", "a", "a", 7)),
			v072TestLine(t, " \t@SpEaKiNg", "tag", 3, 2, v072TestElement(t, "tag", "@SpEaKiNg", "speaking", 3)),
			v072TestLine(t, "  @SPEAKING", "tag", 3, 2, v072TestElement(t, "tag", "@SPEAKING", "speaking", 3)),
			v072TestText(t, "Говори 😀.", "Говори 😀.", 3, 2),
		}, [][6]int{{5, 3, 12, 4, 3, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v072TestRun(t, v072TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV072ResetEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
	}{
		{"endtask", v072TestTag(t, "endtask", 2, 1)},
		{"header", v072TestNamed(t, "header", "title", "Theory", 2, 1)},
		{"newpage", v072TestTag(t, "newpage", 0, 0)},
		{"endtask bad tail", v072TestInvalid(t, "@ENDTASK}", 2, 1)},
		{"header missing title", v072TestInvalid(t, "@header", 2, 1)},
		{"newpage bad tail", v072TestInvalid(t, "@newpage\textra", 2, 1)},
		{"variants missing block", v072TestInvalid(t, "@variants", 2, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v072TestRun(t, v072TestDocument(t,
				v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
				tc.line, v072TestTag(t, "speaking", 2, 1), v072TestTag(t, "speaking", 2, 1),
				v072TestText(t, "Read.", "Read.", 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct {
		marker string
		want   [][6]int
	}{
		{"###", nil}, {"#", [][6]int{{6, 1, 10, 4, 1, 10}}}, {"##", [][6]int{{6, 1, 10, 4, 1, 10}}},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			result := v072TestDocument(t,
				v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
				v072TestHeading(t, tc.marker, "Next"), v072TestTag(t, "speaking", 5, 1),
				v072TestText(t, "Read.", "Read.", 5, 1))
			if tc.marker != "###" {
				// DSL 2.1.3–2.1.4: неверное размещение не делает уровень равным 3.
				v072TestParserError(t, result, 5, -1, "Недопустимое размещение заголовка.")
				if tc.marker == "#" {
					result.Document.Metadata.Title = nil
				} else {
					subtitle := "Next"
					result.Document.Metadata.Subtitle = &subtitle
				}
			}
			v072TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV072RawNamesAndTails(t *testing.T) {
	for _, raw := range []string{"@speaking extra", "@SPEAKING\tлишнее", "@speaking\u00a0extra",
		" \t@SpEaKiNg{", "@speaking}", "@speaking\u3000extra"} {
		t.Run(raw, func(t *testing.T) {
			start, end := 1, 10
			if raw == " \t@SpEaKiNg{" {
				start, end = 3, 12
			}
			// Начало и конец ожидаемого имени заданы явно, а не поиском в raw.
			for _, parsed := range []bool{false, true} {
				t.Run("parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
					line := v072TestInvalid(t, raw, 3, 2)
					if parsed {
						runes := []rune(raw)
						line = v072TestLine(t, raw, "invalid", 3, 2,
							v072TestElement(t, "tag", string(runes[start-1:end-1]), "speaking", start),
							v072TestUnparsed(t, string(runes[end-1:]), end))
					}
					v072TestRun(t, v072TestDocument(t,
						v072TestNamed(t, "task", "identifier", "a", 2, 1), line,
						v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2)),
						"current", [][6]int{{5, 1, 10, 4, start, end}})
				})
			}
		})
	}
	t.Run("raw repeated declaration", func(t *testing.T) {
		v072TestRun(t, v072TestDocument(t,
			v072TestInvalid(t, "@TASK\tid", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestLine(t, " \t@SPEAKING extra", "tag", 3, 2, v072TestUnparsed(t, " \t@SPEAKING extra", 1)),
			v072TestText(t, "Speak.", "Speak.", 3, 2)), "current", [][6]int{{5, 3, 12, 4, 1, 10}})
	})
	for _, raw := range []string{"@speakingExtra", "@speaking-name", "@speakin", "@speaking\u200b", "@ſpeaking"} {
		t.Run(raw, func(t *testing.T) {
			v072TestRun(t, v072TestDocument(t,
				v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestInvalid(t, raw, 3, 2),
				v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@speaking`, "@speaking"}, {`\\@speaking`, `\@speaking`},
		{"Mention @speaking", "Mention @speaking"}, {"\u00a0@speaking", "\u00a0@speaking"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v072TestRun(t, v072TestDocument(t,
				v072TestNamed(t, "task", "identifier", "a", 2, 1),
				v072TestText(t, tc.raw, tc.value, 3, 2), v072TestTag(t, "speaking", 3, 2)), "current", nil)
		})
	}
}

func TestV072Blocks(t *testing.T) {
	t.Run("content and closing brace preserve first", func(t *testing.T) {
		v072TestRun(t, v072TestDocument(t,
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestOpen(t, "text", 3, 2), v072TestText(t, "Read.", "Read.", 5, 3), v072TestClose(t, 5, 3),
			v072TestTag(t, "speaking", 3, 2)), "current", [][6]int{{8, 1, 10, 4, 1, 10}})
	})
	for _, raw := range []string{"@speaking", "@task hidden", "@endtask", "@header Theory", "@newpage", "@variants", "@variant B", "### Next"} {
		t.Run("excluded event "+raw, func(t *testing.T) {
			v072TestRun(t, v072TestDocument(t,
				v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
				v072TestOpen(t, "text", 3, 2), v072TestInvalid(t, raw, 5, 3), v072TestClose(t, 5, 3),
				v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2)),
				"current", [][6]int{{8, 1, 10, 4, 1, 10}})
		})
	}
	t.Run("nested speaking does not become first", func(t *testing.T) {
		v072TestRun(t, v072TestDocument(t,
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestOpen(t, "text", 3, 2),
			v072TestInvalid(t, "@speaking", 4, 3), v072TestClose(t, 4, 3),
			v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2)), "current", nil)
	})
	t.Run("nested task does not open task", func(t *testing.T) {
		v072TestRun(t, v072TestDocument(t,
			v072TestOpen(t, "text", 2, 1), v072TestInvalid(t, "@task hidden", 3, 2),
			v072TestText(t, "Read.", "Read.", 3, 2), v072TestClose(t, 3, 2),
			v072TestTag(t, "speaking", 2, 1), v072TestTag(t, "speaking", 2, 1)), "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		for _, parsed := range []bool{false, true} {
			t.Run(tag+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				open := v072TestOpen(t, tag, 3, 2)
				if !parsed {
					// Ошибка разделителя по DSL 1.29; блок и связи сохранены.
					raw := "@" + strings.ToUpper(tag)
					open = v072TestLine(t, raw+"{", "block-start", 3, 2,
						v072TestUnparsed(t, raw, 1), v072TestElement(t, "block-open", "{", "{", len(raw)+1))
				}
				v072TestRun(t, v072TestDocument(t,
					v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
					open, v072TestText(t, "@speaking", "@speaking", 5, 3),
					v072TestText(t, "@task hidden", "@task hidden", 5, 3),
					v072TestText(t, "@endtask", "@endtask", 5, 3),
					v072TestText(t, "@variant B", "@variant B", 5, 3),
					v072TestClose(t, 5, 3), v072TestTag(t, "speaking", 3, 2),
					v072TestText(t, "Speak.", "Speak.", 3, 2)),
					"current", [][6]int{{11, 1, 10, 4, 1, 10}})
			})
		}
	}
}

func TestV072Variants(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"separate task in each branch", []model.Line{
			v072TestOpen(t, "variants", 2, 1), v072TestNamed(t, "variant", "name", "A", 3, 2),
			v072TestNamed(t, "task", "identifier", "a", 4, 3), v072TestTag(t, "speaking", 5, 4),
			v072TestText(t, "Read.", "Read.", 5, 4), v072TestNamed(t, "variant", "name", "B", 3, 2),
			v072TestNamed(t, "task", "identifier", "b", 8, 3),
			v072TestTag(t, "speaking", 9, 4), v072TestTag(t, "speaking", 9, 4),
			v072TestText(t, "Speak.", "Speak.", 9, 4), v072TestClose(t, 3, 2),
		}, [][6]int{{11, 1, 10, 10, 1, 10}}},
		{"next variant closes task", []model.Line{
			v072TestOpen(t, "variants", 2, 1), v072TestNamed(t, "variant", "name", "A", 3, 2),
			v072TestNamed(t, "task", "identifier", "a", 4, 3), v072TestTag(t, "speaking", 5, 4),
			v072TestText(t, "Read.", "Read.", 5, 4), v072TestNamed(t, "variant", "name", "B", 3, 2),
			v072TestTag(t, "speaking", 8, 3), v072TestTag(t, "speaking", 8, 3),
			v072TestText(t, "Speak.", "Speak.", 8, 3), v072TestClose(t, 3, 2),
		}, nil},
		{"closing variants closes last task", []model.Line{
			v072TestOpen(t, "variants", 2, 1), v072TestNamed(t, "variant", "name", "A", 3, 2),
			v072TestText(t, "Read.", "Read.", 4, 3), v072TestNamed(t, "variant", "name", "B", 3, 2),
			v072TestNamed(t, "task", "identifier", "b", 6, 3), v072TestTag(t, "speaking", 7, 4),
			v072TestText(t, "Speak.", "Speak.", 7, 4), v072TestClose(t, 3, 2),
			v072TestTag(t, "speaking", 2, 1), v072TestTag(t, "speaking", 2, 1),
		}, nil},
		{"external task never resumes", []model.Line{
			v072TestNamed(t, "task", "identifier", "outer", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestText(t, "Speak.", "Speak.", 3, 2), v072TestOpen(t, "variants", 2, 1),
			v072TestNamed(t, "variant", "name", "A", 6, 2), v072TestText(t, "Read.", "Read.", 7, 3),
			v072TestNamed(t, "variant", "name", "B", 6, 2), v072TestText(t, "Write.", "Write.", 9, 3),
			v072TestClose(t, 6, 2), v072TestTag(t, "speaking", 2, 1), v072TestTag(t, "speaking", 2, 1),
		}, nil},
		{"literal variant preserves task in branch", []model.Line{
			v072TestOpen(t, "variants", 2, 1), v072TestNamed(t, "variant", "name", "A", 3, 2),
			v072TestNamed(t, "task", "identifier", "a", 4, 3), v072TestTag(t, "speaking", 5, 4),
			v072TestOpen(t, "editor", 5, 4), v072TestText(t, "@variant B", "@variant B", 7, 5),
			v072TestClose(t, 7, 5), v072TestTag(t, "speaking", 5, 4),
			v072TestText(t, "Speak.", "Speak.", 5, 4), v072TestNamed(t, "variant", "name", "B", 3, 2),
			v072TestText(t, "Write.", "Write.", 12, 3), v072TestClose(t, 3, 2),
		}, [][6]int{{10, 1, 10, 6, 1, 10}}},
		{"events before first variant excluded", []model.Line{
			v072TestOpen(t, "variants", 2, 1), v072TestInvalid(t, "@task a", 3, 2),
			v072TestInvalid(t, "@speaking", 3, 2), v072TestInvalid(t, "@speaking", 3, 2),
			v072TestNamed(t, "variant", "name", "A", 3, 2), v072TestText(t, "Read.", "Read.", 7, 3),
			v072TestNamed(t, "variant", "name", "B", 3, 2), v072TestText(t, "Write.", "Write.", 9, 3),
			v072TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, shared := range []bool{false, true} {
				t.Run("shared parent="+strconv.FormatBool(shared), func(t *testing.T) {
					result := v072TestDocument(t, tc.body...)
					if shared {
						v072TestSharedParents(t, result)
					}
					v072TestRun(t, result, "current", tc.want)
				})
			}
		})
	}
	t.Run("raw variant events", func(t *testing.T) {
		// Частичный разбор variants сохранил фигурный блок.
		// У первого variant нет имени; у второго неверный разделитель.
		v072TestRun(t, v072TestDocument(t,
			v072TestLine(t, "@VARIANTS{", "block-start", 2, 1,
				v072TestUnparsed(t, "@VARIANTS", 1), v072TestElement(t, "block-open", "{", "{", 10)),
			v072TestInvalid(t, "@variant", 3, 2), v072TestNamed(t, "task", "identifier", "a", 3, 2),
			v072TestTag(t, "speaking", 5, 3), v072TestText(t, "Read.", "Read.", 5, 3),
			v072TestInvalid(t, "@VARIANT\tB", 3, 2), v072TestTag(t, "speaking", 3, 2),
			v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2),
			v072TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV072EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v072TestDocument(t,
				v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
				v072TestTag(t, "speaking", 3, 2), v072TestText(t, "Speak.", "Speak.", 3, 2))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v072TestRun(t, result, "first", [][6]int{{5, 1, 10, 4, 1, 10}})
			v072TestRun(t, result, "second", [][6]int{{5, 1, 10, 4, 1, 10}})
		})
	}
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"EOF after first", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestText(t, "Speak.", "Speak.", 3, 2),
			v072TestTag(t, "speaking", 3, 2),
		}, nil},
		{"EOF after repeat", []model.Line{
			v072TestNamed(t, "task", "identifier", "a", 2, 1), v072TestTag(t, "speaking", 3, 2),
			v072TestText(t, "Speak.", "Speak.", 3, 2), v072TestTag(t, "speaking", 3, 2),
		}, [][6]int{{6, 1, 10, 4, 1, 10}}},
		{"next document starts inactive", []model.Line{
			v072TestTag(t, "speaking", 2, 1), v072TestTag(t, "speaking", 2, 1),
			v072TestText(t, "Read.", "Read.", 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v072TestRun(t, v072TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v072TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v072TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v072TestDocument(t)
			result.Document.DSLVersion = &version
			before := v072TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v072TestRegistry(t))
			if err == nil || checks != nil || v072TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v072TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v072TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v072TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v072TestHeading(t, "#", "Lesson"),
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
				v072TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v072TestSharedParents задаёт поддерживаемый моделью вариант родительства:
// содержимое ветви связано с variants, не с однострочным variant.
func v072TestSharedParents(t *testing.T, result *model.Result) {
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

// v072TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v072TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v072TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v072TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v072TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v072TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v072TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v072TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v072TestLine(t, raw, "invalid", parent, depth, v072TestUnparsed(t, raw, 1))
}

func v072TestTag(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v072TestLine(t, "@"+tag, "tag", parent, depth, v072TestElement(t, "tag", "@"+tag, tag, 1))
}

func v072TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v072TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v072TestElement(t, "tag", "@"+tag, tag, 1), v072TestElement(t, kind, value, value, len(tag)+3))
}

func v072TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v072TestLine(t, marker+" "+title, "heading", 0, 0,
		v072TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v072TestElement(t, "title", title, title, len(marker)+2))
}

func v072TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v072TestLine(t, raw, "content", parent, depth, v072TestElement(t, "content", raw, value, 1))
}

func v072TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v072TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v072TestElement(t, "tag", "@"+tag, tag, 1), v072TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v072TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v072TestLine(t, "}", "block-end", parent, depth, v072TestElement(t, "block-close", "}", "}", 1))
}

func v072TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v072TestRun выполняет полный каталог и сравнивает новые V072 с явными
// шестёрками основного и связанного диапазонов строка/начало/конец. Правые границы исключаются.
func v072TestRun(t *testing.T, result *model.Result, processingID string, want [][6]int) {
	t.Helper()
	before := v072TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v072TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v072TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V072" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V072 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		related := model.Location{Start: model.Position{Line: position[3], Column: position[4]},
			End: model.Position{Line: position[3], Column: position[5]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.Message != "@speaking объявлен в задании повторно." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V072 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v072TestSnapshot(t, result)), &restored); err != nil {
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
	if v072TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v072TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v072TestSource(t *testing.T, result *model.Result, want string) {
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

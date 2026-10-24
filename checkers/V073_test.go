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

func TestV073Registration(t *testing.T) {
	definitions, err := v073TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V073" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" ||
			definition.Message != "Шаг после @newpage не содержит отображаемого материала." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) ||
			definition.Check == nil {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V073 definitions = %d, want 1", count)
	}
}

func TestV073ReferenceAndExample(t *testing.T) {
	// DSL 1.3, 1.30 и 2.1.3 требуют добавить версию и # Lesson.
	// Содержимое фрагментов сохранено дословно, включая пустую строку.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][8]int
	}{
		{"reference", "@newpage\n@header Practice\nRead the text.", []model.Line{
			v073TestN(t), v073TestTag(t, "header", "title", "Practice", 3, 1),
			v073TestText(t, "Read the text.", "Read the text.", 4, 2),
		}, nil},
		{"example", "@newpage\n\n### Reading\nRead the text.", []model.Line{
			v073TestN(t), v073TestLine(t, "", "blank", 3, 1), v073TestH(t, "Reading"),
			v073TestText(t, "Read the text.", "Read the text.", 5, 1),
		}, [][8]int{{3, 1, 4, 1, 5, 1, 5, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v073TestDocument(t, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v073TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV073StepRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][8]int
	}{
		{"no newpage", []model.Line{
			v073TestH(t, "Reading"), v073TestText(t, "Read.", "Read.", 3, 1),
		}, nil},
		{"newpage at EOF", []model.Line{v073TestN(t)}, [][8]int{{3, 1, 3, 9, 0, 0, 0, 0}}},
		{"immediate heading", []model.Line{
			v073TestN(t), v073TestH(t, "Reading"), v073TestText(t, "Read.", "Read.", 4, 1),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 12}}},
		{"two newpages then text", []model.Line{
			v073TestN(t), v073TestN(t), v073TestText(t, "Read.", "Read.", 4, 1),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 9}}},
		{"two newpages at EOF", []model.Line{
			v073TestN(t), v073TestN(t),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 9}, {4, 1, 4, 9, 0, 0, 0, 0}}},
		{"three newpages then text", []model.Line{
			v073TestN(t), v073TestN(t), v073TestN(t), v073TestText(t, "Read.", "Read.", 5, 1),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 9}, {4, 1, 4, 9, 5, 1, 5, 9}}},
		{"earlier content does not fill last step", []model.Line{
			v073TestN(t), v073TestText(t, "Read.", "Read.", 3, 1), v073TestN(t),
		}, [][8]int{{5, 1, 5, 9, 0, 0, 0, 0}}},
		{"text before heading", []model.Line{
			v073TestN(t), v073TestText(t, "Read.", "Read.", 3, 1), v073TestH(t, "Reading"),
		}, nil},
		{"whitespace end column", []model.Line{
			v073TestN(t), v073TestLine(t, " \t ", "blank", 3, 1), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 4, 4, 5, 1, 5, 12}}},
		{"several empty lines", []model.Line{
			v073TestN(t), v073TestLine(t, "", "blank", 3, 1),
			v073TestLine(t, "\t", "blank", 3, 1), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 5, 2, 6, 1, 6, 12}}},
		{"newpage parent does not have to be root", []model.Line{
			v073TestTag(t, "newpage", "", "", 2, 1), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 12}}},
		{"indentation and Unicode related title", []model.Line{
			v073TestLine(t, " \t@NEWPAGE ", "tag", 0, 0,
				v073TestElement(t, "tag", "@NEWPAGE", "newpage", 3, 11)),
			v073TestLine(t, "  ### Я😀 ", "heading", 0, 0,
				v073TestElement(t, "heading-level", "###", "3", 3, 6),
				v073TestElement(t, "title", "Я😀", "Я😀", 7, 9)),
		}, [][8]int{{3, 3, 3, 12, 4, 3, 4, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("last physical blank line with EOL", func(t *testing.T) {
		result := v073TestDocument(t, v073TestN(t), v073TestLine(t, "", "blank", 3, 1))
		result.Lines[3].LineEnding = "\n"
		v073TestRun(t, result, "current", [][8]int{{3, 1, 4, 1, 0, 0, 0, 0}})
	})
}

func TestV073ExcludedContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][8]int
	}{
		{"single editor", []model.Line{
			v073TestN(t), v073TestTag(t, "editor", "content", "Check.", 3, 1), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 4, 15, 5, 1, 5, 12}}},
		{"editor block", []model.Line{
			v073TestN(t), v073TestOpen(t, "editor", 3, 1), v073TestText(t, "Check.", "Check.", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}}},
		{"literal newpage in editor", []model.Line{
			v073TestN(t), v073TestOpen(t, "EDITOR", 3, 1), v073TestText(t, "@newpage", "@newpage", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}}},
		{"literal heading in editor", []model.Line{
			v073TestN(t), v073TestOpen(t, "editor", 3, 1), v073TestText(t, "### Reading", "### Reading", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}}},
		{"unparsed comment does not make step uncertain", []model.Line{
			v073TestN(t), v073TestOpen(t, "editor", 3, 1), v073TestInvalid(t, "Unescaped {", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}}},
		{"definition is not insertion", []model.Line{
			v073TestN(t), v073TestFragment(t, "f", 3, 1), v073TestText(t, "Read.", "Read.", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}}},
		{"unused erroneous definition is ignored", []model.Line{
			v073TestN(t), v073TestFragment(t, "f", 3, 1), v073TestInvalid(t, "@unknown", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}}},
		{"comment at EOF", []model.Line{
			v073TestN(t), v073TestTag(t, "editor", "content", "Я😀", 3, 1),
		}, [][8]int{{3, 1, 4, 11, 0, 0, 0, 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV073ServiceDeclarations(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		kind model.ElementType
		text string
		want [][8]int
	}{
		{"endtask", "", "", [][8]int{{3, 1, 4, 9, 5, 1, 5, 12}}},
		{"speaking", "", "", [][8]int{{3, 1, 4, 10, 5, 1, 5, 12}}},
		{"document-id", "identifier", "lesson", [][8]int{{3, 1, 4, 20, 5, 1, 5, 12}}},
		{"section", "name", "Unit", [][8]int{{3, 1, 4, 14, 5, 1, 5, 12}}},
		{"order", "number", "1", [][8]int{{3, 1, 4, 9, 5, 1, 5, 12}}},
		{"resource-dir", "resource-path", "images", [][8]int{{3, 1, 4, 21, 5, 1, 5, 12}}},
		{"dsl-version", "version", "1.2", [][8]int{{3, 1, 4, 17, 5, 1, 5, 12}}},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			// Эти служебные объявления размещены ошибочно (DSL 1.3, 1.31.2,
			// 2.4.6, 2.6). Распознанное имя не становится отображаемым материалом.
			result := v073TestDocument(t, v073TestN(t),
				v073TestTag(t, tc.tag, tc.kind, tc.text, 3, 1), v073TestH(t, "Reading"))
			v073TestParserError(t, result, 4, "Недопустимое размещение служебного тега.")
			v073TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV073TextAndLiteralBoundaries(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"Я", "Я"}, {"😀", "😀"}, {"e\u0301", "e\u0301"}, {"\u00a0", "\u00a0"},
		{"\u2003", "\u2003"}, {`\@newpage`, "@newpage"}, {`\### Reading`, "### Reading"},
		{`\\@newpage`, `\@newpage`}, {"Read @newpage", "Read @newpage"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, v073TestN(t),
				v073TestText(t, tc.raw, tc.value, 3, 1), v073TestH(t, "Reading")), "current", nil)
		})
	}
}

func TestV073DisplayedConstructs(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		kind model.ElementType
		text string
	}{
		{"header", "title", "Practice"}, {"task", "identifier", "practice"},
		{"instr", "content", "Read."}, {"example", "content", "A cat."},
		{"wordlist", "content", "cat; dog."}, {"note", "content", "Remember."},
		{"alt", "content", "A cat."}, {"hint", "content", "Try."},
		{"question", "content", "Why?"}, {"multifill", "", ""},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, v073TestN(t),
				v073TestTag(t, tc.tag, tc.kind, tc.text, 3, 1), v073TestH(t, "Reading")), "current", nil)
		})
	}
	t.Run("media", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestN(t),
			v073TestLine(t, "@media image cat.png", "tag", 3, 1,
				v073TestElement(t, "tag", "@media", "media", 1, 7),
				v073TestElement(t, "media-type", "image", "image", 8, 13),
				v073TestElement(t, "source", "cat.png", "cat.png", 14, 21)),
			v073TestH(t, "Reading")), "current", nil)
	})
	for _, tag := range []string{"table", "script", "text", "choice", "multichoice", "ordering"} {
		t.Run(tag, func(t *testing.T) {
			first, second := "Italy", "Spain"
			if tag == "table" {
				first, second = "Word | Meaning", "cat | animal"
			}
			v073TestRun(t, v073TestDocument(t, v073TestN(t), v073TestOpen(t, tag, 3, 1),
				v073TestText(t, first, first, 4, 2), v073TestText(t, second, second, 4, 2),
				v073TestClose(t, 4, 2), v073TestH(t, "Reading")), "current", nil)
		})
	}
	t.Run("matching with separator", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestN(t), v073TestOpen(t, "matching", 3, 1),
			v073TestText(t, "cat", "cat", 4, 2), v073TestLine(t, "---", "separator", 4, 2),
			v073TestText(t, "animal", "animal", 4, 2), v073TestText(t, "plant", "plant", 4, 2),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading")), "current", nil)
	})
	t.Run("step and key have required task context", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestN(t), v073TestTag(t, "task", "identifier", "a", 3, 1),
			v073TestTag(t, "speaking", "", "", 4, 2), v073TestTag(t, "step", "title", "Read", 4, 2),
			v073TestTag(t, "instr", "content", "Read.", 6, 3), v073TestOpen(t, "key", 6, 3),
			v073TestText(t, "Answer.", "Answer.", 8, 4), v073TestClose(t, 8, 4),
			v073TestTag(t, "endtask", "", "", 4, 2), v073TestH(t, "Reading")), "current", nil)
	})
	t.Run("nested displayed content", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestN(t), v073TestOpen(t, "instr", 3, 1),
			v073TestText(t, "Read.", "Read.", 4, 2), v073TestOpen(t, "hint", 4, 2),
			v073TestText(t, "Try.", "Try.", 6, 3), v073TestClose(t, 6, 3),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading")), "current", nil)
	})
	t.Run("variants", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestN(t), v073TestOpen(t, "variants", 3, 1),
			v073TestTag(t, "variant", "name", "A", 4, 2), v073TestText(t, "Read.", "Read.", 5, 3),
			v073TestTag(t, "variant", "name", "B", 4, 2), v073TestText(t, "Write.", "Write.", 7, 3),
			v073TestClose(t, 4, 2), v073TestH(t, "Reading")), "current", nil)
	})
	for _, tag := range []string{"text", "variants"} {
		t.Run("empty "+tag+" still supplies a construction", func(t *testing.T) {
			// Собственный минимум блока нарушен, но V073 не оценивает
			// полноту распознанной отображаемой конструкции.
			result := v073TestDocument(t, v073TestN(t), v073TestOpen(t, tag, 3, 1),
				v073TestLine(t, "", "blank", 4, 2), v073TestClose(t, 4, 2), v073TestH(t, "Reading"))
			v073TestParserError(t, result, 4, "Блок не содержит обязательного содержимого.")
			v073TestRun(t, result, "current", nil)
		})
	}
}

func TestV073BoundariesInsideBlocks(t *testing.T) {
	t.Run("recognized newpage inside fragment does not end area", func(t *testing.T) {
		result := v073TestDocument(t, v073TestN(t), v073TestFragment(t, "f", 3, 1),
			v073TestTag(t, "newpage", "", "", 4, 2), v073TestClose(t, 4, 2), v073TestH(t, "Reading"))
		v073TestParserError(t, result, 5, "Структурный тег запрещён внутри fragment.")
		v073TestRun(t, result, "current", [][8]int{{3, 1, 6, 2, 7, 1, 7, 12}})
	})
	t.Run("nested newpage does not create a checked step", func(t *testing.T) {
		result := v073TestDocument(t, v073TestOpen(t, "instr", 2, 1),
			v073TestText(t, "Read.", "Read.", 3, 2), v073TestOpen(t, "hint", 3, 2),
			v073TestTag(t, "newpage", "", "", 5, 3),
			v073TestClose(t, 5, 3), v073TestClose(t, 3, 2))
		v073TestParserError(t, result, 6, "Структурный тег запрещён внутри hint.")
		v073TestRun(t, result, "current", nil)
	})
	t.Run("newpage inside variants is excluded", func(t *testing.T) {
		result := v073TestDocument(t, v073TestOpen(t, "variants", 2, 1),
			v073TestTag(t, "variant", "name", "A", 3, 2), v073TestText(t, "Read.", "Read.", 4, 3),
			v073TestTag(t, "variant", "name", "B", 3, 2), v073TestText(t, "Write.", "Write.", 6, 3),
			v073TestTag(t, "newpage", "", "", 6, 3), v073TestClose(t, 3, 2))
		v073TestParserError(t, result, 8, "Тег newpage запрещён внутри variants.")
		v073TestRun(t, result, "current", nil)
	})
	t.Run("literal newpage in multifill", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestOpen(t, "multifill", 2, 1),
			v073TestText(t, "@newpage", "@newpage", 3, 2), v073TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV073Includes(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][8]int
	}{
		{"definition before include", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestText(t, "Read.", "Read.", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 6, 1), v073TestH(t, "Reading"),
		}, nil},
		{"definition after include", []model.Line{
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 3, 1), v073TestH(t, "Reading"),
			v073TestFragment(t, "f", 5, 1), v073TestText(t, "Read.", "Read.", 6, 2), v073TestClose(t, 6, 2),
		}, nil},
		{"empty insertion", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestLine(t, "", "blank", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 6, 1), v073TestH(t, "Reading"),
		}, [][8]int{{6, 1, 7, 11, 8, 1, 8, 12}}},
		{"empty definition after include", []model.Line{
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 3, 1), v073TestH(t, "Reading"),
			v073TestFragment(t, "f", 5, 1), v073TestLine(t, " \t ", "blank", 6, 2), v073TestClose(t, 6, 2),
		}, [][8]int{{3, 1, 4, 11, 5, 1, 5, 12}}},
		{"two empty insertions are not a cycle", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestLine(t, "", "blank", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 6, 1),
			v073TestTag(t, "include", "identifier", "f", 6, 1), v073TestH(t, "Reading"),
		}, [][8]int{{6, 1, 8, 11, 9, 1, 9, 12}}},
		{"missing definition", []model.Line{
			v073TestN(t), v073TestTag(t, "include", "identifier", "missing", 3, 1), v073TestH(t, "Reading"),
		}, nil},
		{"ambiguous definition", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestLine(t, "", "blank", 3, 2), v073TestClose(t, 3, 2),
			v073TestFragment(t, "F", 2, 1), v073TestLine(t, "", "blank", 6, 2), v073TestClose(t, 6, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 9, 1), v073TestH(t, "Reading"),
		}, nil},
		{"unparsed fragment body", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestInvalid(t, "@unknown", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 6, 1), v073TestH(t, "Reading"),
		}, nil},
		{"missing parsed include ID", []model.Line{
			v073TestN(t), v073TestBroken(t, v073TestTag(t, "include", "", "", 3, 1), "?"), v073TestH(t, "Reading"),
		}, nil},
		{"empty included text construction is sufficient", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestOpen(t, "text", 3, 2), v073TestLine(t, "", "blank", 4, 3),
			v073TestClose(t, 4, 3), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 8, 1), v073TestH(t, "Reading"),
		}, nil},
		{"material only in definition is not lost at EOF", []model.Line{
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 3, 1),
			v073TestFragment(t, "f", 3, 1), v073TestText(t, "Read.", "Read.", 5, 2), v073TestClose(t, 5, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		definition, reference string
		want                  [][8]int
	}{
		{"Intro", "INTRO", [][8]int{{6, 1, 7, 15, 8, 1, 8, 12}}},
		{"Ввод", "вВОД", [][8]int{{6, 1, 7, 14, 8, 1, 8, 12}}},
	} {
		t.Run("case insensitive ID "+tc.reference, func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, v073TestFragment(t, tc.definition, 2, 1),
				v073TestLine(t, "", "blank", 3, 2), v073TestClose(t, 3, 2),
				v073TestN(t), v073TestTag(t, "include", "identifier", tc.reference, 6, 1),
				v073TestH(t, "Reading")), "current", tc.want)
		})
	}
}

func TestV073GlobalAndLiteralDefinitions(t *testing.T) {
	for _, filled := range []bool{false, true} {
		t.Run("global definition filled="+strconv.FormatBool(filled), func(t *testing.T) {
			body := v073TestLine(t, "", "blank", 5, 4)
			want := [][8]int{{12, 1, 13, 11, 14, 1, 14, 12}}
			if filled {
				body = v073TestText(t, "Read.", "Read.", 5, 4)
				want = nil
			}
			v073TestRun(t, v073TestDocument(t, v073TestOpen(t, "variants", 2, 1),
				v073TestTag(t, "variant", "name", "A", 3, 2), v073TestFragment(t, "f", 4, 3),
				body, v073TestClose(t, 5, 4), v073TestText(t, "Read.", "Read.", 4, 3),
				v073TestTag(t, "variant", "name", "B", 3, 2), v073TestText(t, "Write.", "Write.", 9, 3),
				v073TestClose(t, 3, 2), v073TestN(t),
				v073TestTag(t, "include", "identifier", "f", 12, 1), v073TestH(t, "Reading")),
				"current", want)
		})
	}
	t.Run("literal fragment is not a second definition", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t, v073TestOpen(t, "editor", 2, 1),
			v073TestText(t, `@fragment f \{`, "@fragment f {", 3, 2), v073TestClose(t, 3, 2),
			v073TestFragment(t, "f", 2, 1), v073TestLine(t, "", "blank", 6, 2), v073TestClose(t, 6, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 9, 1), v073TestH(t, "Reading")),
			"current", [][8]int{{9, 1, 10, 11, 11, 1, 11, 12}})
	})
	t.Run("definition without parsed ID cannot be resolved", func(t *testing.T) {
		v073TestRun(t, v073TestDocument(t,
			v073TestLine(t, "@fragment ? {", "block-start", 2, 1,
				v073TestElement(t, "tag", "@fragment", "fragment", 1, 10),
				v073TestUnparsed(t, "?", 11, 12), v073TestElement(t, "block-open", "{", "{", 13, 14)),
			v073TestText(t, "Read.", "Read.", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "f", 6, 1), v073TestH(t, "Reading")),
			"current", nil)
	})
}

func TestV073IncludeChainsAndCycles(t *testing.T) {
	// V073 требует исследовать распознанную вставку; запрет include внутри
	// fragment из общей спецификации сохраняется отдельной исходной ошибкой.
	for _, tc := range []struct {
		name      string
		body      []model.Line
		misplaced []int
		want      [][8]int
	}{
		{"empty chain", []model.Line{
			v073TestFragment(t, "outer", 2, 1),
			v073TestTag(t, "include", "identifier", "inner", 3, 2), v073TestClose(t, 3, 2),
			v073TestFragment(t, "inner", 2, 1), v073TestLine(t, "", "blank", 6, 2), v073TestClose(t, 6, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "outer", 9, 1), v073TestH(t, "Reading"),
		}, []int{4}, [][8]int{{9, 1, 10, 15, 11, 1, 11, 12}}},
		{"filled chain", []model.Line{
			v073TestFragment(t, "outer", 2, 1),
			v073TestTag(t, "include", "identifier", "inner", 3, 2), v073TestClose(t, 3, 2),
			v073TestFragment(t, "inner", 2, 1), v073TestText(t, "Read.", "Read.", 6, 2), v073TestClose(t, 6, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "outer", 9, 1), v073TestH(t, "Reading"),
		}, []int{4}, nil},
		{"unresolved chain", []model.Line{
			v073TestFragment(t, "outer", 2, 1),
			v073TestTag(t, "include", "identifier", "missing", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "outer", 6, 1), v073TestH(t, "Reading"),
		}, []int{4}, nil},
		{"self cycle does not affect next step", []model.Line{
			v073TestFragment(t, "loop", 2, 1),
			v073TestTag(t, "include", "identifier", "LOOP", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "loop", 6, 1),
			v073TestN(t), v073TestH(t, "Reading"),
		}, []int{4}, [][8]int{{8, 1, 8, 9, 9, 1, 9, 12}}},
		{"mutual cycle", []model.Line{
			v073TestFragment(t, "a", 2, 1),
			v073TestTag(t, "include", "identifier", "b", 3, 2), v073TestClose(t, 3, 2),
			v073TestFragment(t, "b", 2, 1),
			v073TestTag(t, "include", "identifier", "a", 6, 2), v073TestClose(t, 6, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "a", 9, 1), v073TestH(t, "Reading"),
		}, []int{4, 7}, nil},
		{"content after cyclic include", []model.Line{
			v073TestFragment(t, "loop", 2, 1),
			v073TestTag(t, "include", "identifier", "loop", 3, 2),
			v073TestText(t, "Read.", "Read.", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestTag(t, "include", "identifier", "loop", 7, 1), v073TestH(t, "Reading"),
		}, []int{4}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v073TestDocument(t, tc.body...)
			for _, number := range tc.misplaced {
				v073TestParserError(t, result, number, "Вложенный include запрещён внутри fragment.")
			}
			v073TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV073PartialParsing(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][8]int
	}{
		{"only erroneous interval", []model.Line{
			v073TestN(t), v073TestInvalid(t, "@unknown", 3, 1), v073TestH(t, "Reading"),
		}, nil},
		{"unparsed ordinary content", []model.Line{
			v073TestN(t),
			// DSL 4.3.2: два подчёркивания — ошибочная длина плейсхолдера.
			v073TestLine(t, "__", "content", 3, 1, v073TestUnparsed(t, "__", 1, 3)),
			v073TestH(t, "Reading"),
		}, nil},
		{"known text with unparsed remainder", []model.Line{
			v073TestN(t), v073TestBroken(t, v073TestText(t, "Read.", "Read.", 3, 1), "__"),
			v073TestH(t, "Reading"),
		}, nil},
		{"recognized media with broken source", []model.Line{
			v073TestN(t), v073TestBroken(t, v073TestTag(t, "media", "media-type", "image", 3, 1), `"unterminated`),
			v073TestH(t, "Reading"),
		}, nil},
		{"unparsed interval does not affect next step", []model.Line{
			v073TestN(t), v073TestInvalid(t, "@unknown", 3, 1), v073TestN(t), v073TestH(t, "Reading"),
		}, [][8]int{{5, 1, 5, 9, 6, 1, 6, 12}}},
		{"recognized newpage with extra argument", []model.Line{
			v073TestBroken(t, v073TestN(t), "extra"), v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 3, 15, 4, 1, 4, 12}}},
		{"recognized next newpage with extra argument", []model.Line{
			v073TestN(t), v073TestBroken(t, v073TestN(t), "extra"),
			v073TestText(t, "Read.", "Read.", 4, 1),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 15}}},
		{"recognized heading with remainder is next boundary", []model.Line{
			v073TestN(t), v073TestBroken(t, v073TestH(t, "Reading"), "}"),
		}, [][8]int{{3, 1, 3, 9, 4, 1, 4, 14}}},
		{"unrecognized newpage is not reconstructed", []model.Line{
			v073TestInvalid(t, "@newpage", 2, 1),
		}, nil},
		{"unrecognized boundary in interval leaves uncertainty", []model.Line{
			v073TestN(t), v073TestInvalid(t, "###", 3, 1), v073TestH(t, "Reading"),
		}, nil},
		{"comment with erroneous remainder stays excluded", []model.Line{
			v073TestN(t), v073TestBroken(t, v073TestTag(t, "editor", "content", "Check.", 3, 1), "}"),
			v073TestH(t, "Reading"),
		}, [][8]int{{3, 1, 4, 17, 5, 1, 5, 12}}},
		{"recognized include with remainder still resolves", []model.Line{
			v073TestFragment(t, "f", 2, 1), v073TestLine(t, "", "blank", 3, 2), v073TestClose(t, 3, 2),
			v073TestN(t), v073TestBroken(t, v073TestTag(t, "include", "identifier", "f", 6, 1), "extra"),
			v073TestH(t, "Reading"),
		}, [][8]int{{6, 1, 7, 17, 8, 1, 8, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v073TestRun(t, v073TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("invalid line retains recognized newpage", func(t *testing.T) {
		line := v073TestBroken(t, v073TestN(t), "extra")
		line.LineType = "invalid"
		v073TestRun(t, v073TestDocument(t, line, v073TestH(t, "Reading")),
			"current", [][8]int{{3, 1, 3, 15, 4, 1, 4, 12}})
	})
}

func TestV073EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		endings []model.LineEnding
	}{
		{"LF", []model.LineEnding{"\n", "\n", "\n", "\n", "\n", "\n"}},
		{"CRLF", []model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n", "\r\n", "\r\n"}},
		{"mixed", []model.LineEnding{"\r\n", "\n", "\r\n", "\n", "\r\n", "\n"}},
		{"EOF", []model.LineEnding{"\n", "\n", "\n", "\n", "\n", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v073TestDocument(t, v073TestN(t), v073TestLine(t, "", "blank", 3, 1),
				v073TestH(t, "Reading"), v073TestText(t, "Read the text.", "Read the text.", 5, 1))
			for i, ending := range tc.endings {
				result.Lines[i].LineEnding = ending
			}
			v073TestRun(t, result, "first", [][8]int{{3, 1, 4, 1, 5, 1, 5, 12}})
			v073TestRun(t, result, "second", [][8]int{{3, 1, 4, 1, 5, 1, 5, 12}})
			v073TestRun(t, v073TestDocument(t, v073TestN(t),
				v073TestTag(t, "header", "title", "Practice", 3, 1),
				v073TestText(t, "Read the text.", "Read the text.", 4, 2)), "separate", nil)
		})
	}
}

func TestV073EmptyLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		for _, knownCount := range []bool{false, true} {
			t.Run(version+" count known="+strconv.FormatBool(knownCount), func(t *testing.T) {
				result := &model.Result{
					Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version,
						Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
					Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
					Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
				}
				if knownCount {
					count := 0
					result.Document.LineCount = &count
				}
				if version == "1.2" {
					v073TestRun(t, result, "current", nil)
					return
				}
				before := v073TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v073TestRegistry(t))
				if err == nil || checks != nil || v073TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
				}
			})
		}
	}
}

// v073TestH задаёт корневой заголовок третьего уровня.
func v073TestH(t *testing.T, title string) model.Line {
	t.Helper()
	return v073TestHeading(t, "", "###", "3", title)
}

// v073TestN задаёт корневой тег начала нового шага.
func v073TestN(t *testing.T) model.Line {
	t.Helper()
	return v073TestTag(t, "newpage", "", "", 0, 0)
}

// v073TestFragment задаёт определение с явно указанным ID.
func v073TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v073TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
		v073TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v073TestElement(t, "identifier", id, id, 11, end),
		v073TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v073TestUnparsed задаёт неразобранную часть с отсутствующим Value.
// Документ добавляет требуемую исходную ошибку и ссылку на неё.
func v073TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v073TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v073TestBroken добавляет явно ошибочный остаток к распознанной строке.
func v073TestBroken(t *testing.T, line model.Line, extra string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " " + extra
	line.Elements = append(line.Elements, v073TestUnparsed(t, extra, start, start+utf8.RuneCountInString(extra)))
	return line
}

// v073TestParserError сохраняет ошибку размещения распознанной конструкции,
// не превращая её в unparsed. Это исходные данные, не ожидание результата V073.
func v073TestParserError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1},
			End: model.Position{Line: number, Column: utf8.RuneCountInString(line.Raw) + 1}},
	})
	line.HasErrors, result.Document.HasErrors = true, true
}

// v073TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V073.
func v073TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v073TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v073TestHeading(t, "", "#", "1", title),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	result.Lines[len(result.Lines)-1].LineEnding = ""
	return result
}

// v073TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v073TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v073TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v073TestElement(t, "heading-level", marker, level, start, end),
		v073TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v073TestInvalid задаёт ошибочную строку без распознанного heading-level.
func v073TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v073TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v073TestLine(t, raw, "invalid", parent, depth, element)
}

// v073TestTag собирает однострочный тег с одним явно типизированным параметром.
func v073TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v073TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v073TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v073TestLine(t, raw, "tag", parent, depth, elements...)
}

// v073TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v073TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v073TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v073TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v073TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v073TestClose задаёт закрытие блока с явным родителем.
func v073TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v073TestLine(t, "}", "block-end", parent, depth, v073TestElement(t, "block-close", "}", "}", 1, 2))
}

// v073TestText сохраняет независимо заданные исходное и семантическое значения.
func v073TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v073TestLine(t, raw, "content", parent, depth,
		v073TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v073TestLine не классифицирует исходник и не вычисляет родительские связи.
func v073TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v073TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v073TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v073TestRegistry использует штатный каталог целиком.
func v073TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v073TestRun проверяет только новые V073 по явно заданным диапазонам:
// первые четыре числа задают основной диапазон, следующие четыре — связанный.
// Нули связанного диапазона означают его отсутствие. Концы исключаются.
func v073TestRun(t *testing.T, result *model.Result, processingID string, want [][8]int) {
	t.Helper()
	before := v073TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v073TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v073TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V073" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V073 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[2], Column: position[3]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "block" || entry.Fatal || entry.Message != "Шаг после @newpage не содержит отображаемого материала." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V073 metadata", i, entry, location)
		}
		if position[4] == 0 {
			if len(entry.RelatedLocations) != 0 {
				t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
			}
		} else {
			related := model.Location{Start: model.Position{Line: position[4], Column: position[5]},
				End: model.Position{Line: position[6], Column: position[7]}}
			if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
				t.Fatalf("related locations = %+v, want exactly %+v", entry.RelatedLocations, related)
			}
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v073TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 ||
		len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v073TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v073TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v073TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

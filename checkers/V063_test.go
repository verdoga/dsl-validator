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

func TestV063Registration(t *testing.T) {
	definitions, err := v063TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V063" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" ||
			definition.Message != "Задание не содержит учебного содержимого." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) ||
			definition.Check == nil {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V063 definitions = %d, want 1", count)
	}
}

func TestV063ReferenceAndExample(t *testing.T) {
	// Обязательный контекст DSL 1.3, 1.30, 2.1.3: @dsl-version 1.2 и
	// # Lesson. Значимое содержимое обоих фрагментов сохранено дословно.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][4]int
	}{
		{"reference", "@task practice\n@step Read\nRead the text.\n@step Write\nWrite an answer.\n@endtask", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "step", "title", "Read", 3, 2),
			v063TestText(t, "Read the text.", "Read the text.", 4, 3),
			v063TestTag(t, "step", "title", "Write", 3, 2),
			v063TestText(t, "Write an answer.", "Write an answer.", 6, 3),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"example", "@task practice\n@endtask", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 3, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v063TestDocument(t, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v063TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV063TaskRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"no tasks", []model.Line{v063TestText(t, "Read.", "Read.", 2, 1)}, nil},
		{"task at EOF", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
		}, [][4]int{{3, 1, 3, 15}}},
		{"Unicode ID does not change column units", []model.Line{
			v063TestTag(t, "task", "identifier", "учёба", 2, 1),
		}, [][4]int{{3, 1, 3, 12}}},
		{"last blank belongs to task", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestLine(t, "", "blank", 3, 2), v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 4, 1}}},
		{"speaking is not content", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "speaking", "", "", 3, 2), v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 4, 10}}},
		{"tag names ignore case", []model.Line{
			v063TestTag(t, "TASK", "identifier", "practice", 2, 1),
			v063TestTag(t, "SPEAKING", "", "", 3, 2), v063TestTag(t, "ENDTASK", "", "", 3, 2),
		}, [][4]int{{3, 1, 4, 10}}},
		{"next task ends previous", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "task", "identifier", "other", 2, 1),
			v063TestTag(t, "endtask", "", "", 4, 2),
		}, [][4]int{{3, 1, 3, 15}, {4, 1, 4, 12}}},
		{"header is excluded", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "header", "title", "Practice", 2, 1),
			v063TestText(t, "Read.", "Read.", 4, 2),
		}, [][4]int{{3, 1, 3, 15}}},
		{"newpage is excluded", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "newpage", "", "", 0, 0),
			v063TestText(t, "Read.", "Read.", 4, 1),
		}, [][4]int{{3, 1, 3, 15}}},
		{"heading level three is excluded", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestHeading(t, "", "###", "3", "Reading"),
			v063TestText(t, "Read.", "Read.", 4, 1),
		}, [][4]int{{3, 1, 3, 15}}},
		{"content of next task does not fill previous", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "task", "identifier", "other", 2, 1),
			v063TestText(t, "Read.", "Read.", 4, 2),
			v063TestTag(t, "endtask", "", "", 4, 2),
		}, [][4]int{{3, 1, 3, 15}}},
		{"ordinary content and independent empty task", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestText(t, "Read.", "Read.", 3, 2),
			v063TestTag(t, "task", "identifier", "other", 2, 1),
			v063TestTag(t, "endtask", "", "", 5, 2),
		}, [][4]int{{5, 1, 5, 12}}},
		{"source indentation and trailing space", []model.Line{
			v063TestLine(t, " \t@task practice ", "tag", 2, 1,
				v063TestElement(t, "tag", "@task", "task", 3, 8),
				v063TestElement(t, "identifier", "practice", "practice", 9, 17)),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 3, 3, 18}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV063HiddenContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"single editor", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "editor", "content", "Check.", 3, 2),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 4, 15}}},
		{"editor block", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "editor", 3, 2), v063TestText(t, "Check.", "Check.", 4, 3),
			v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 6, 2}}},
		{"literal endtask in editor", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "EDITOR", 3, 2), v063TestText(t, "@endtask", "@endtask", 4, 3),
			v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 6, 2}}},
		{"definition is not rendered", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestFragment(t, "f", 3, 2), v063TestText(t, "Read.", "Read.", 4, 3),
			v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 6, 2}}},
		{"unused erroneous fragment does not make task uncertain", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestFragment(t, "f", 3, 2), v063TestInvalid(t, "@unknown", 4, 3),
			v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 6, 2}}},
		{"closing nested block does not terminate task", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "editor", 3, 2), v063TestText(t, "Check.", "Check.", 4, 3),
			v063TestClose(t, 4, 3), v063TestText(t, "Read.", "Read.", 3, 2),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV063Branches(t *testing.T) {
	t.Run("variants closes outer task before any branch content", func(t *testing.T) {
		v063TestRun(t, v063TestDocument(t,
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "variants", 2, 1),
			v063TestTag(t, "variant", "name", "A", 4, 2), v063TestText(t, "Read.", "Read.", 5, 3),
			v063TestTag(t, "variant", "name", "B", 4, 2), v063TestText(t, "Write.", "Write.", 7, 3),
			v063TestClose(t, 4, 2)), "current", [][4]int{{3, 1, 3, 15}})
	})
	for _, commonParent := range []bool{false, true} {
		t.Run("common parent "+strconv.FormatBool(commonParent), func(t *testing.T) {
			parentA, parentB, taskDepth := 4, 6, 3
			if commonParent {
				parentA, parentB, taskDepth = 3, 3, 2
			}
			t.Run("next variant and closing variants", func(t *testing.T) {
				v063TestRun(t, v063TestDocument(t, v063TestOpen(t, "variants", 2, 1),
					v063TestTag(t, "variant", "name", "A", 3, 2),
					v063TestTag(t, "task", "identifier", "practice", parentA, taskDepth),
					v063TestTag(t, "variant", "name", "B", 3, 2),
					v063TestTag(t, "task", "identifier", "other", parentB, taskDepth),
					v063TestClose(t, 3, 2)), "current", [][4]int{{5, 1, 5, 15}, {7, 1, 7, 12}})
			})
			t.Run("later branch cannot fill earlier task", func(t *testing.T) {
				if !commonParent {
					parentB = 7
				}
				v063TestRun(t, v063TestDocument(t, v063TestOpen(t, "variants", 2, 1),
					v063TestTag(t, "variant", "name", "A", 3, 2),
					v063TestTag(t, "task", "identifier", "practice", parentA, taskDepth),
					v063TestTag(t, "editor", "content", "Check.", 5, taskDepth+1),
					v063TestTag(t, "variant", "name", "B", 3, 2),
					v063TestTag(t, "task", "identifier", "other", parentB, taskDepth),
					v063TestText(t, "Read.", "Read.", 8, taskDepth+1),
					v063TestClose(t, 3, 2)), "current", [][4]int{{5, 1, 6, 15}})
			})
		})
	}
	t.Run("closing variants excludes subsequent text", func(t *testing.T) {
		v063TestRun(t, v063TestDocument(t, v063TestOpen(t, "variants", 2, 1),
			v063TestTag(t, "variant", "name", "A", 3, 2), v063TestText(t, "Read.", "Read.", 4, 3),
			v063TestTag(t, "variant", "name", "B", 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 6, 3),
			v063TestClose(t, 3, 2), v063TestText(t, "After.", "After.", 2, 1)),
			"current", [][4]int{{7, 1, 7, 15}})
	})
}

func TestV063TextValues(t *testing.T) {
	// Явный набор Unicode White_Space, допустимых внутри Raw строки:
	// LF и CR представлены окончаниями строк в отдельном тесте.
	for _, value := range []string{
		"\t", "\v", "\f", " ", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005",
		"\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028",
		"\u2029", "\u202f", "\u205f", "\u3000",
	} {
		t.Run("empty "+strconv.Quote(value), func(t *testing.T) {
			content := v063TestText(t, value, value, 3, 2)
			if value == "\t" || value == " " {
				content = v063TestLine(t, value, "blank", 3, 2)
			}
			v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				content, v063TestTag(t, "endtask", "", "", 3, 2)), "current", [][4]int{{3, 1, 4, 2}})
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{"Я", "Я"}, {"😀", "😀"}, {"e\u0301", "e\u0301"}, {"\u200b", "\u200b"},
		{"\u00a0Я\u3000", "\u00a0Я\u3000"}, {`\#`, "#"}, {`\@endtask`, "@endtask"},
		{`\@task fake`, "@task fake"},
	} {
		t.Run("filled "+strconv.Quote(tc.raw), func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestText(t, tc.raw, tc.value, 3, 2), v063TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
		})
	}
}

func TestV063DisplayedConstructs(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		kind model.ElementType
		text string
	}{
		{"step", "title", "Read"}, {"instr", "content", "Read."},
		{"example", "content", "A cat."}, {"wordlist", "content", "cat; dog."},
		{"note", "content", "Remember."}, {"alt", "content", "A cat."},
		{"hint", "content", "Try."}, {"question", "content", "Why?"}, {"multifill", "", ""},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestTag(t, tc.tag, tc.kind, tc.text, 3, 2),
				v063TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
		})
	}
	t.Run("media", func(t *testing.T) {
		v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestLine(t, "@media image cat.png", "tag", 3, 2,
				v063TestElement(t, "tag", "@media", "media", 1, 7),
				v063TestElement(t, "media-type", "image", "image", 8, 13),
				v063TestElement(t, "source", "cat.png", "cat.png", 14, 21)),
			v063TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
	})
	for _, tag := range []string{"table", "script", "text", "key", "choice", "multichoice", "ordering"} {
		t.Run(tag, func(t *testing.T) {
			first, second := "Italy", "Spain"
			if tag == "table" {
				first, second = "Word | Meaning", "cat | animal"
			}
			v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestOpen(t, tag, 3, 2), v063TestText(t, first, first, 4, 3),
				v063TestText(t, second, second, 4, 3), v063TestClose(t, 4, 3),
				v063TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
		})
	}
	t.Run("matching", func(t *testing.T) {
		v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "matching", 3, 2), v063TestText(t, "cat", "cat", 4, 3),
			v063TestLine(t, "---", "separator", 4, 3),
			v063TestText(t, "animal", "animal", 4, 3), v063TestText(t, "plant", "plant", 4, 3),
			v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
	})
	t.Run("empty text block still supplies a construction", func(t *testing.T) {
		// Собственная пустота блока нарушает DSL 3.0.4 и проверяется V035.
		// V063 прямо запрещено оценивать содержимое найденной конструкции.
		v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "text", 3, 2), v063TestLine(t, "", "blank", 4, 3),
			v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
	})
}

func TestV063StructuralTagsInsideContentBlocks(t *testing.T) {
	t.Run("nested content does not open tasks", func(t *testing.T) {
		result := v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestOpen(t, "instr", 3, 2), v063TestOpen(t, "hint", 4, 3),
			v063TestTag(t, "task", "identifier", "inner", 5, 4),
			v063TestTag(t, "endtask", "", "", 6, 5),
			v063TestClose(t, 5, 4), v063TestClose(t, 4, 3),
			v063TestTag(t, "endtask", "", "", 3, 2))
		v063TestParserError(t, result, 6, "Структурный тег запрещён внутри hint.")
		v063TestParserError(t, result, 7, "Структурный тег запрещён внутри hint.")
		v063TestRun(t, result, "current", nil)
	})
	t.Run("structural tags in definition neither terminate nor fill task", func(t *testing.T) {
		result := v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestFragment(t, "f", 3, 2), v063TestTag(t, "task", "identifier", "inner", 4, 3),
			v063TestTag(t, "endtask", "", "", 5, 4), v063TestClose(t, 4, 3),
			v063TestTag(t, "endtask", "", "", 3, 2))
		v063TestParserError(t, result, 5, "Структурный тег запрещён внутри fragment.")
		v063TestParserError(t, result, 6, "Структурный тег запрещён внутри fragment.")
		v063TestRun(t, result, "current", [][4]int{{3, 1, 7, 2}})
	})
	for _, tc := range []struct {
		tag  string
		kind model.ElementType
		text string
	}{
		{"endtask", "", ""}, {"header", "title", "Hidden"}, {"newpage", "", ""},
		{"variant", "name", "Hidden"},
	} {
		t.Run("excluded boundary "+tc.tag, func(t *testing.T) {
			result := v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestFragment(t, "f", 3, 2), v063TestTag(t, tc.tag, tc.kind, tc.text, 4, 3),
				v063TestClose(t, 4, 3), v063TestTag(t, "endtask", "", "", 3, 2))
			v063TestParserError(t, result, 5, "Структурный тег запрещён внутри fragment.")
			v063TestRun(t, result, "current", [][4]int{{3, 1, 6, 2}})
		})
	}
	t.Run("variants inside content cannot terminate outer task", func(t *testing.T) {
		result := v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestFragment(t, "f", 3, 2), v063TestOpen(t, "variants", 4, 3),
			v063TestTag(t, "variant", "name", "A", 5, 4), v063TestText(t, "Read.", "Read.", 6, 5),
			v063TestTag(t, "variant", "name", "B", 5, 4), v063TestText(t, "Write.", "Write.", 8, 5),
			v063TestClose(t, 5, 4), v063TestClose(t, 4, 3),
			v063TestTag(t, "endtask", "", "", 3, 2))
		v063TestParserError(t, result, 5, "Структурный тег запрещён внутри fragment.")
		v063TestRun(t, result, "current", [][4]int{{3, 1, 11, 2}})
	})
}

func TestV063Includes(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"definition before task", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestText(t, "Read.", "Read.", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 6, 2),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, nil},
		{"definition after task", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 3, 2),
			v063TestTag(t, "endtask", "", "", 3, 2),
			v063TestFragment(t, "f", 2, 1), v063TestText(t, "Read.", "Read.", 6, 2), v063TestClose(t, 6, 2),
		}, nil},
		{"definition within task is resolved but not rendered twice", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestFragment(t, "f", 3, 2), v063TestText(t, "Read.", "Read.", 4, 3), v063TestClose(t, 4, 3),
			v063TestTag(t, "include", "identifier", "f", 3, 2),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"empty definition", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestLine(t, "", "blank", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 6, 2),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, [][4]int{{6, 1, 7, 11}}},
		{"NBSP definition is empty", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestText(t, "\u00a0", "\u00a0", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 6, 2),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, [][4]int{{6, 1, 7, 11}}},
		{"comment only definition", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestOpen(t, "editor", 3, 2),
			v063TestText(t, "@task hidden", "@task hidden", 4, 3),
			v063TestClose(t, 4, 3), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 8, 2),
			v063TestTag(t, "endtask", "", "", 8, 2),
		}, [][4]int{{8, 1, 9, 11}}},
		{"missing definition", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "missing", 3, 2),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"ambiguous case insensitive ID", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestLine(t, "", "blank", 3, 2), v063TestClose(t, 3, 2),
			v063TestFragment(t, "F", 2, 1), v063TestLine(t, "", "blank", 6, 2), v063TestClose(t, 6, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 9, 2),
			v063TestTag(t, "endtask", "", "", 9, 2),
		}, nil},
		{"definition contains only erroneous content", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestInvalid(t, "@unknown", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 6, 2),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, nil},
		{"repeated empty includes are not a cycle", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestLine(t, "", "blank", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 6, 2), v063TestTag(t, "include", "identifier", "f", 6, 2),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, [][4]int{{6, 1, 8, 11}}},
		{"empty displayed block in definition is sufficient", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestOpen(t, "text", 3, 2), v063TestLine(t, "", "blank", 4, 3),
			v063TestClose(t, 4, 3), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 8, 2), v063TestTag(t, "endtask", "", "", 8, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, tc := range []struct{ definition, reference string }{
		{"Intro", "INTRO"}, {"Ввод", "вВОД"},
	} {
		t.Run("case insensitive "+tc.definition, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, v063TestFragment(t, tc.definition, 2, 1),
				v063TestText(t, "Read.", "Read.", 3, 2), v063TestClose(t, 3, 2),
				v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestTag(t, "include", "identifier", tc.reference, 6, 2),
				v063TestTag(t, "endtask", "", "", 6, 2)), "current", nil)
		})
	}
}

func TestV063GlobalFragmentAndLiteralDefinition(t *testing.T) {
	t.Run("fragment in another branch", func(t *testing.T) {
		v063TestRun(t, v063TestDocument(t, v063TestOpen(t, "variants", 2, 1),
			v063TestTag(t, "variant", "name", "A", 3, 2), v063TestFragment(t, "f", 4, 3),
			v063TestText(t, "Read.", "Read.", 5, 4), v063TestClose(t, 5, 4),
			v063TestText(t, "Read.", "Read.", 4, 3),
			v063TestTag(t, "variant", "name", "B", 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 9, 3),
			v063TestTag(t, "include", "identifier", "f", 10, 4),
			v063TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("empty fragment in another branch resolves", func(t *testing.T) {
		v063TestRun(t, v063TestDocument(t, v063TestOpen(t, "variants", 2, 1),
			v063TestTag(t, "variant", "name", "A", 3, 2), v063TestFragment(t, "f", 4, 3),
			v063TestLine(t, "", "blank", 5, 4), v063TestClose(t, 5, 4),
			v063TestText(t, "Read.", "Read.", 4, 3),
			v063TestTag(t, "variant", "name", "B", 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 9, 3),
			v063TestTag(t, "include", "identifier", "f", 10, 4),
			v063TestClose(t, 3, 2)), "current", [][4]int{{10, 1, 11, 11}})
	})
	t.Run("literal fragment does not make reference ambiguous", func(t *testing.T) {
		// DSL 2.8.11: содержимое editor не объявляет теги; экранированная
		// скобка не открывает вложенный блок (DSL 1.21).
		v063TestRun(t, v063TestDocument(t, v063TestOpen(t, "editor", 2, 1),
			v063TestText(t, `@fragment f \{`, "@fragment f {", 3, 2), v063TestClose(t, 3, 2),
			v063TestFragment(t, "f", 2, 1), v063TestLine(t, "", "blank", 6, 2), v063TestClose(t, 6, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "f", 9, 2),
			v063TestTag(t, "endtask", "", "", 9, 2)), "current", [][4]int{{9, 1, 10, 11}})
	})
}

func TestV063IncludeChainsAndCycles(t *testing.T) {
	// Распознанные include внутри fragment исследуются по алгоритму V063.
	// Запрет размещения из общей спецификации не отменяет их распознавание.
	for _, tc := range []struct {
		name      string
		body      []model.Line
		misplaced []int
		want      [][4]int
	}{
		{"empty chain", []model.Line{
			v063TestFragment(t, "outer", 2, 1),
			v063TestTag(t, "include", "identifier", "inner", 3, 2), v063TestClose(t, 3, 2),
			v063TestFragment(t, "inner", 2, 1), v063TestLine(t, "", "blank", 6, 2), v063TestClose(t, 6, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "outer", 9, 2),
			v063TestTag(t, "endtask", "", "", 9, 2),
		}, []int{4}, [][4]int{{9, 1, 10, 15}}},
		{"filled chain", []model.Line{
			v063TestFragment(t, "outer", 2, 1),
			v063TestTag(t, "include", "identifier", "inner", 3, 2), v063TestClose(t, 3, 2),
			v063TestFragment(t, "inner", 2, 1), v063TestText(t, "Read.", "Read.", 6, 2), v063TestClose(t, 6, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "outer", 9, 2),
			v063TestTag(t, "endtask", "", "", 9, 2),
		}, []int{4}, nil},
		{"unresolved chain", []model.Line{
			v063TestFragment(t, "outer", 2, 1),
			v063TestTag(t, "include", "identifier", "missing", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "outer", 6, 2),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, []int{4}, nil},
		{"self cycle does not affect next task", []model.Line{
			v063TestFragment(t, "loop", 2, 1),
			v063TestTag(t, "include", "identifier", "LOOP", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "loop", 6, 2),
			v063TestTag(t, "task", "identifier", "other", 2, 1),
			v063TestTag(t, "endtask", "", "", 8, 2),
		}, []int{4}, [][4]int{{8, 1, 8, 12}}},
		{"mutual cycle", []model.Line{
			v063TestFragment(t, "a", 2, 1),
			v063TestTag(t, "include", "identifier", "b", 3, 2), v063TestClose(t, 3, 2),
			v063TestFragment(t, "b", 2, 1),
			v063TestTag(t, "include", "identifier", "a", 6, 2), v063TestClose(t, 6, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestTag(t, "include", "identifier", "a", 9, 2),
			v063TestTag(t, "endtask", "", "", 9, 2),
		}, []int{4, 7}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v063TestDocument(t, tc.body...)
			for _, number := range tc.misplaced {
				v063TestParserError(t, result, number, "Вложенный include запрещён внутри fragment.")
			}
			v063TestRun(t, result, "current", tc.want)
		})
	}
	for _, tc := range []struct {
		definition, reference string
		want                  [][4]int
	}{
		{"Intro", "INTRO", [][4]int{{6, 1, 7, 15}}},
		{"Ввод", "вВОД", [][4]int{{6, 1, 7, 14}}},
	} {
		t.Run("empty ID match "+tc.reference, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, v063TestFragment(t, tc.definition, 2, 1),
				v063TestLine(t, "", "blank", 3, 2), v063TestClose(t, 3, 2),
				v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestTag(t, "include", "identifier", tc.reference, 6, 2),
				v063TestTag(t, "endtask", "", "", 6, 2)), "current", tc.want)
		})
	}
}

func TestV063PartialParsing(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"invalid interval", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestInvalid(t, "@unknown", 3, 2), v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"unparsed content", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			// DSL 4.3.2: ошибочная длина плейсхолдера в обычном тексте.
			v063TestLine(t, "__", "content", 3, 2, v063TestUnparsed(t, "__", 1, 3)),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"recognized content and unparsed remainder", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestText(t, "Read.", "Read.", 3, 2), "__"),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"whitespace does not resolve unparsed remainder", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestText(t, "\u00a0", "\u00a0", 3, 2), "__"),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"recognized media with malformed source", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestTag(t, "media", "media-type", "image", 3, 2), `"unterminated`),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"comment with malformed remainder remains excluded", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestTag(t, "editor", "content", "Check.", 3, 2), "}"),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 4, 17}}},
		{"recognized task is not its own content", []model.Line{
			v063TestBroken(t, v063TestTag(t, "task", "identifier", "practice", 2, 1), "}"),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, [][4]int{{3, 1, 3, 17}}},
		{"recognized terminator survives remainder", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestTag(t, "endtask", "", "", 3, 2), "extra"),
		}, [][4]int{{3, 1, 3, 15}}},
		{"uncertainty is local to task", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestInvalid(t, "@unknown", 3, 2),
			v063TestTag(t, "task", "identifier", "other", 2, 1),
			v063TestTag(t, "endtask", "", "", 5, 2),
		}, [][4]int{{5, 1, 5, 12}}},
		{"include without identifier", []model.Line{
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestTag(t, "include", "", "", 3, 2), "?"),
			v063TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"recognized include with remainder still resolves", []model.Line{
			v063TestFragment(t, "f", 2, 1), v063TestLine(t, "", "blank", 3, 2), v063TestClose(t, 3, 2),
			v063TestTag(t, "task", "identifier", "practice", 2, 1),
			v063TestBroken(t, v063TestTag(t, "include", "identifier", "f", 6, 2), "extra"),
			v063TestTag(t, "endtask", "", "", 6, 2),
		}, [][4]int{{6, 1, 7, 17}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v063TestRun(t, v063TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV063EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		endings []model.LineEnding
	}{
		{"LF", []model.LineEnding{"\n", "\n", "\n", "\n"}},
		{"CRLF", []model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n"}},
		{"mixed", []model.LineEnding{"\r\n", "\n", "\r\n", "\n"}},
		{"EOF", []model.LineEnding{"\n", "\n", "\n", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestTag(t, "endtask", "", "", 3, 2))
			for i, ending := range tc.endings {
				result.Lines[i].LineEnding = ending
			}
			v063TestRun(t, result, "first", [][4]int{{3, 1, 3, 15}})
			v063TestRun(t, result, "second", [][4]int{{3, 1, 3, 15}})
			v063TestRun(t, v063TestDocument(t, v063TestTag(t, "task", "identifier", "practice", 2, 1),
				v063TestTag(t, "step", "title", "Read", 3, 2),
				v063TestTag(t, "endtask", "", "", 3, 2)), "separate", nil)
		})
	}
}

func TestV063EmptyLinesAndVersions(t *testing.T) {
	t.Run("known empty file", func(t *testing.T) {
		count, version := 0, "1.2"
		result := &model.Result{
			Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
			Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
			Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
		}
		v063TestRun(t, result, "current", nil)
	})
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := &model.Result{
				Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version,
					Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
				Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
			}
			if version == "1.2" {
				v063TestRun(t, result, "current", nil)
				return
			}
			before := v063TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v063TestRegistry(t))
			if err == nil || checks != nil || v063TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v063TestFragment задаёт определение с явно указанным ID.
func v063TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v063TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
		v063TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v063TestElement(t, "identifier", id, id, 11, end),
		v063TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v063TestUnparsed задаёт неразобранную часть с отсутствующим Value.
// Документ добавляет требуемую исходную ошибку и ссылку на неё.
func v063TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v063TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v063TestBroken добавляет явно ошибочный остаток к распознанной строке.
func v063TestBroken(t *testing.T, line model.Line, extra string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " " + extra
	line.Elements = append(line.Elements, v063TestUnparsed(t, extra, start, start+utf8.RuneCountInString(extra)))
	return line
}

// v063TestParserError сохраняет ошибку размещения распознанной конструкции,
// не превращая её в unparsed. Это исходные данные, не ожидание результата V063.
func v063TestParserError(t *testing.T, result *model.Result, number int, message string) {
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

// v063TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V063.
func v063TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v063TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v063TestHeading(t, "", "#", "1", title),
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

// v063TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v063TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v063TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v063TestElement(t, "heading-level", marker, level, start, end),
		v063TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v063TestInvalid задаёт ошибочную строку без распознанного heading-level.
func v063TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v063TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v063TestLine(t, raw, "invalid", parent, depth, element)
}

// v063TestTag собирает однострочный тег с одним явно типизированным параметром.
func v063TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v063TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v063TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v063TestLine(t, raw, "tag", parent, depth, elements...)
}

// v063TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v063TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v063TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v063TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v063TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v063TestClose задаёт закрытие блока с явным родителем.
func v063TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v063TestLine(t, "}", "block-end", parent, depth, v063TestElement(t, "block-close", "}", "}", 1, 2))
}

// v063TestText сохраняет независимо заданные исходное и семантическое значения.
func v063TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v063TestLine(t, raw, "content", parent, depth,
		v063TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v063TestLine не классифицирует исходник и не вычисляет родительские связи.
func v063TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v063TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v063TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v063TestRegistry использует штатный каталог целиком.
func v063TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v063TestRun проверяет только новые V063 по явно заданным диапазонам:
// строка начала, колонка начала, строка конца, исключающая колонка конца.
func v063TestRun(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v063TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v063TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v063TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V063" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V063 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[2], Column: position[3]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "block" || entry.Fatal || entry.Message != "Задание не содержит учебного содержимого." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V063 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v063TestSnapshot(t, result)), &restored); err != nil {
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
	if v063TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v063TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v063TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

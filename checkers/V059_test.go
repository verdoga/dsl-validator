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

func TestV059Registration(t *testing.T) {
	definitions, err := v059TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V059" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" ||
			definition.Message != "Между @newpage и заголовком шага образован пустой шаг." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) ||
			definition.Check == nil {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V059 definitions = %d, want 1", count)
	}
}

func TestV059ReferenceAndExamples(t *testing.T) {
	// DSL 1.3, 1.30 и 2.1.3 требуют добавить @dsl-version 1.2 и # Lesson.
	// Сами фрагменты сохранены дословно; <pre>/<br> — разметка таблицы.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][4]int
	}{
		{"reference", "### Reading\nRead the text.\n@newpage\nWrite an answer.", []model.Line{
			v059TestH(t, "Reading"), v059TestText(t, "Read the text.", "Read the text.", 3, 1),
			v059TestN(t), v059TestText(t, "Write an answer.", "Write an answer.", 5, 1),
		}, nil},
		{"heading then newpage", "### Reading\n@newpage\nWrite an answer.", []model.Line{
			v059TestH(t, "Reading"), v059TestN(t), v059TestText(t, "Write an answer.", "Write an answer.", 4, 1),
		}, [][4]int{{3, 1, 4, 9}}},
		{"newpage then heading", "@newpage\n### Reading\nRead the text.", []model.Line{
			v059TestN(t), v059TestH(t, "Reading"), v059TestText(t, "Read the text.", "Read the text.", 4, 1),
		}, [][4]int{{3, 1, 4, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v059TestDocument(t, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v059TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV059BoundaryPairs(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"no boundaries", []model.Line{v059TestText(t, "Read.", "Read.", 2, 1)}, nil},
		{"one heading", []model.Line{v059TestH(t, "Reading"), v059TestText(t, "Read.", "Read.", 3, 1)}, nil},
		{"one newpage", []model.Line{v059TestN(t), v059TestText(t, "Read.", "Read.", 3, 1)}, nil},
		{"headings of same kind", []model.Line{
			v059TestH(t, "Reading"), v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 4, 1),
		}, nil},
		{"newpages of same kind", []model.Line{
			v059TestN(t), v059TestN(t), v059TestText(t, "Read.", "Read.", 4, 1),
		}, nil},
		{"same kind still replaces previous boundary", []model.Line{
			v059TestH(t, "Reading"), v059TestH(t, "Writing"), v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, [][4]int{{4, 1, 5, 9}}},
		{"same newpage kind replaces previous boundary", []model.Line{
			v059TestN(t), v059TestN(t), v059TestH(t, "Reading"), v059TestText(t, "Read.", "Read.", 5, 1),
		}, [][4]int{{4, 1, 5, 12}}},
		{"two empty pairs", []model.Line{
			v059TestH(t, "Reading"), v059TestN(t), v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 5, 1),
		}, [][4]int{{3, 1, 4, 9}, {4, 1, 5, 12}}},
		{"filled pair followed by empty pair", []model.Line{
			v059TestH(t, "Reading"), v059TestText(t, "Read.", "Read.", 3, 1), v059TestN(t),
			v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 6, 1),
		}, [][4]int{{5, 1, 6, 12}}},
		{"empty pair followed by filled pair", []model.Line{
			v059TestH(t, "Reading"), v059TestN(t), v059TestText(t, "Read.", "Read.", 4, 1),
			v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 6, 1),
		}, [][4]int{{3, 1, 4, 9}}},
		{"newpage has content", []model.Line{
			v059TestN(t), v059TestText(t, "Read.", "Read.", 3, 1), v059TestH(t, "Reading"),
			v059TestText(t, "Write.", "Write.", 5, 1),
		}, nil},
		{"blank lines do not break adjacency", []model.Line{
			v059TestH(t, "Reading"), v059TestLine(t, "", "blank", 3, 1),
			v059TestLine(t, " \t ", "blank", 3, 1), v059TestN(t), v059TestText(t, "Read.", "Read.", 6, 1),
		}, [][4]int{{3, 1, 6, 9}}},
		{"second boundary at EOF", []model.Line{v059TestN(t), v059TestH(t, "Reading")}, [][4]int{{3, 1, 4, 12}}},
		{"unparsed interval does not affect next pair", []model.Line{
			v059TestH(t, "Reading"), v059TestInvalid(t, "@unknown", 3, 1), v059TestN(t),
			v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 6, 1),
		}, [][4]int{{5, 1, 6, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v059TestRun(t, v059TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV059TextAndOriginalRanges(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"Я", "Я"}, {"😀", "😀"}, {"e\u0301", "e\u0301"}, {"\u00a0", "\u00a0"},
		{`\@newpage`, "@newpage"}, {`\### Reading`, "### Reading"},
		{" \t\\@newpage \t", "@newpage"}, {`\\@newpage`, `\@newpage`},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"),
				v059TestText(t, tc.raw, tc.value, 3, 1), v059TestN(t),
				v059TestText(t, "Read.", "Read.", 5, 1)), "current", nil)
		})
	}
	t.Run("leading and trailing whitespace", func(t *testing.T) {
		v059TestRun(t, v059TestDocument(t,
			v059TestHeading(t, " \t", "###", "3", "Reading"),
			v059TestLine(t, "  @newpage \t", "tag", 0, 0,
				v059TestElement(t, "tag", "@newpage", "newpage", 3, 11)),
			v059TestText(t, "Read.", "Read.", 4, 1)), "current", [][4]int{{3, 3, 4, 13}})
	})
	t.Run("Unicode end column and uppercase tag", func(t *testing.T) {
		v059TestRun(t, v059TestDocument(t, v059TestTag(t, "NEWPAGE", "", "", 0, 0),
			v059TestH(t, "😀e\u0301"), v059TestText(t, "Read.", "Read.", 4, 1)),
			"current", [][4]int{{3, 1, 4, 8}})
	})
}

func TestV059CommentsAndDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"single-line editor", []model.Line{
			v059TestH(t, "Reading"), v059TestTag(t, "editor", "content", "Check.", 3, 1), v059TestN(t),
			v059TestText(t, "Read.", "Read.", 5, 1),
		}, [][4]int{{3, 1, 5, 9}}},
		{"literal boundaries in editor", []model.Line{
			v059TestH(t, "Reading"), v059TestOpen(t, "EDITOR", 3, 1),
			v059TestText(t, "### Fake", "### Fake", 4, 2), v059TestText(t, "@newpage", "@newpage", 4, 2),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, [][4]int{{3, 1, 8, 9}}},
		{"definition is not an insertion", []model.Line{
			v059TestH(t, "Reading"), v059TestFragment(t, "f", 3, 1), v059TestText(t, "Read.", "Read.", 4, 2),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 7, 1),
		}, [][4]int{{3, 1, 7, 9}}},
		{"unparsed in unused definition is ignored", []model.Line{
			v059TestH(t, "Reading"), v059TestFragment(t, "f", 3, 1), v059TestInvalid(t, "@unknown", 4, 2),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 7, 1),
		}, [][4]int{{3, 1, 7, 9}}},
		{"recognized boundary in fragment is excluded", []model.Line{
			v059TestH(t, "Reading"), v059TestFragment(t, "f", 3, 1),
			// Запрещённое размещение по DSL 3.12.12; распознанный tag
			// сохраняется в модели, но фигурный предок исключает границу.
			v059TestTag(t, "newpage", "", "", 4, 2), v059TestText(t, "Read.", "Read.", 4, 2),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, [][4]int{{3, 1, 8, 9}}},
		{"literal text in multifill is displayed", []model.Line{
			v059TestH(t, "Reading"), v059TestOpen(t, "multifill", 3, 1),
			v059TestText(t, "@newpage", "@newpage", 4, 2), v059TestText(t, "### Fake", "### Fake", 4, 2),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v059TestDocument(t, tc.body...)
			if tc.name == "recognized boundary in fragment is excluded" {
				v059TestParserError(t, result, 5, "Тег запрещён внутри fragment.")
			}
			v059TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV059Includes(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"definition before include", []model.Line{
			v059TestFragment(t, "f", 2, 1), v059TestText(t, "Read.", "Read.", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 6, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, nil},
		{"definition after include", []model.Line{
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 3, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
			v059TestFragment(t, "f", 5, 1), v059TestText(t, "Read.", "Read.", 7, 2), v059TestClose(t, 7, 2),
		}, nil},
		{"empty insertion", []model.Line{
			v059TestFragment(t, "f", 2, 1), v059TestLine(t, "", "blank", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 6, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, [][4]int{{6, 1, 8, 9}}},
		{"empty definition after include", []model.Line{
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 3, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
			v059TestFragment(t, "f", 5, 1), v059TestLine(t, " \t ", "blank", 7, 2), v059TestClose(t, 7, 2),
		}, [][4]int{{3, 1, 5, 9}}},
		{"empty insertion twice", []model.Line{
			v059TestFragment(t, "f", 2, 1), v059TestLine(t, "", "blank", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 6, 1),
			v059TestTag(t, "include", "identifier", "f", 6, 1), v059TestN(t), v059TestText(t, "Read.", "Read.", 9, 1),
		}, [][4]int{{6, 1, 9, 9}}},
		{"missing definition", []model.Line{
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "missing", 3, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, nil},
		{"two matching definitions", []model.Line{
			v059TestFragment(t, "f", 2, 1), v059TestLine(t, "", "blank", 3, 2), v059TestClose(t, 3, 2),
			v059TestFragment(t, "F", 2, 1), v059TestLine(t, "", "blank", 6, 2), v059TestClose(t, 6, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 9, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 11, 1),
		}, nil},
		{"unparsed fragment body", []model.Line{
			v059TestFragment(t, "f", 2, 1), v059TestInvalid(t, "@unknown", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 6, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, nil},
		{"missing parsed identifier", []model.Line{
			v059TestH(t, "Reading"), v059TestBroken(t, v059TestTag(t, "include", "", "", 3, 1), "???"),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, nil},
		{"nested displayed contents", []model.Line{
			v059TestFragment(t, "f", 2, 1), v059TestOpen(t, "instr", 3, 2),
			v059TestText(t, "Read.", "Read.", 4, 3), v059TestOpen(t, "hint", 4, 3),
			v059TestText(t, "Try.", "Try.", 6, 4), v059TestClose(t, 6, 4),
			v059TestClose(t, 4, 3), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 11, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 13, 1),
		}, nil},
		{"unparsed definition identifier cannot resolve", []model.Line{
			v059TestLine(t, "@fragment ??? {", "block-start", 2, 1,
				v059TestElement(t, "tag", "@fragment", "fragment", 1, 10),
				v059TestUnparsed(t, "???", 11, 14), v059TestElement(t, "block-open", "{", "{", 15, 16)),
			v059TestText(t, "Read.", "Read.", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 6, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v059TestRun(t, v059TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, ids := range [][2]string{{"Intro", "INTRO"}, {"Ввод", "ВВОД"}} {
		t.Run("case insensitive "+ids[0], func(t *testing.T) {
			v059TestRun(t, v059TestDocument(t,
				v059TestFragment(t, ids[0], 2, 1), v059TestText(t, "Read.", "Read.", 3, 2), v059TestClose(t, 3, 2),
				v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", ids[1], 6, 1),
				v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1)), "current", nil)
		})
	}
	for _, body := range []string{"", "Read."} {
		t.Run("recognized include with unparsed remainder "+body, func(t *testing.T) {
			line := v059TestLine(t, "", "blank", 3, 2)
			want := [][4]int{{6, 1, 8, 9}}
			if body != "" {
				line = v059TestText(t, body, body, 3, 2)
				want = nil
			}
			v059TestRun(t, v059TestDocument(t,
				v059TestFragment(t, "f", 2, 1), line, v059TestClose(t, 3, 2), v059TestH(t, "Reading"),
				v059TestBroken(t, v059TestTag(t, "include", "identifier", "f", 6, 1), "extra"),
				v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1)), "current", want)
		})
	}
}

func TestV059RecognizedIncludeInsideFragment(t *testing.T) {
	// DSL 3.12.12 запрещает такое размещение, но алгоритм V059, пункт 4,
	// требует исследовать распознанное включение. По уточнению A ошибка
	// размещения не отменяет достоверные tag и identifier готовой модели.
	// Оба ID однозначны; пустое тело конечной вставки не создаёт содержимого.
	result := v059TestDocument(t,
		v059TestFragment(t, "outer", 2, 1),
		v059TestTag(t, "include", "identifier", "inner", 3, 2), v059TestClose(t, 3, 2),
		v059TestFragment(t, "inner", 2, 1), v059TestLine(t, "", "blank", 6, 2), v059TestClose(t, 6, 2),
		v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "outer", 9, 1),
		v059TestN(t), v059TestText(t, "Read.", "Read.", 11, 1))
	v059TestParserError(t, result, 4, "Вложенный include запрещён внутри fragment.")
	v059TestRun(t, result, "current", [][4]int{{9, 1, 11, 9}})
}

func TestV059IncludeChains(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		misplaced []int
		want      [][4]int
	}{
		{"three definitions ending in empty body", []model.Line{
			v059TestFragment(t, "outer", 2, 1),
			v059TestTag(t, "include", "identifier", "middle", 3, 2), v059TestClose(t, 3, 2),
			v059TestFragment(t, "middle", 2, 1),
			v059TestTag(t, "include", "identifier", "inner", 6, 2), v059TestClose(t, 6, 2),
			v059TestFragment(t, "inner", 2, 1), v059TestLine(t, "", "blank", 9, 2), v059TestClose(t, 9, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "outer", 12, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 14, 1),
		}, []int{4, 7}, [][4]int{{12, 1, 14, 9}}},
		{"nested nonempty body", []model.Line{
			v059TestFragment(t, "outer", 2, 1),
			v059TestTag(t, "include", "identifier", "INNER", 3, 2), v059TestClose(t, 3, 2),
			v059TestFragment(t, "inner", 2, 1), v059TestText(t, "Read.", "Read.", 6, 2), v059TestClose(t, 6, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "outer", 9, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 11, 1),
		}, []int{4}, nil},
		{"shared empty target is not a cycle", []model.Line{
			v059TestFragment(t, "outer", 2, 1),
			v059TestTag(t, "include", "identifier", "inner", 3, 2),
			v059TestTag(t, "include", "identifier", "inner", 3, 2), v059TestClose(t, 3, 2),
			v059TestFragment(t, "inner", 2, 1), v059TestLine(t, "", "blank", 7, 2), v059TestClose(t, 7, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "outer", 10, 1),
			v059TestTag(t, "include", "identifier", "inner", 10, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 13, 1),
		}, []int{4, 5}, [][4]int{{10, 1, 13, 9}}},
		{"missing nested target affects only its pair", []model.Line{
			v059TestFragment(t, "outer", 2, 1),
			v059TestTag(t, "include", "identifier", "missing", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "outer", 6, 1),
			v059TestN(t), v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 9, 1),
		}, []int{4}, [][4]int{{8, 1, 9, 12}}},
		{"ambiguous nested target", []model.Line{
			v059TestFragment(t, "outer", 2, 1),
			v059TestTag(t, "include", "identifier", "f", 3, 2), v059TestClose(t, 3, 2),
			v059TestFragment(t, "f", 2, 1), v059TestLine(t, "", "blank", 6, 2), v059TestClose(t, 6, 2),
			v059TestFragment(t, "F", 2, 1), v059TestLine(t, "", "blank", 9, 2), v059TestClose(t, 9, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "outer", 12, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 14, 1),
		}, []int{4}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v059TestDocument(t, tc.body...)
			for _, number := range tc.misplaced {
				v059TestParserError(t, result, number, "Вложенный include запрещён внутри fragment.")
			}
			v059TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV059IncludeCycles(t *testing.T) {
	// Циклическая цепочка не позволяет закончить исследование вставки:
	// без независимого содержимого пустота не доказана. Ошибка размещения
	// include сохраняется отдельно и не отменяет распознанные ID (уточнение A).
	t.Run("self reference and an independent empty pair", func(t *testing.T) {
		result := v059TestDocument(t, v059TestFragment(t, "loop", 2, 1),
			v059TestTag(t, "include", "identifier", "LOOP", 3, 2), v059TestClose(t, 3, 2),
			v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "loop", 6, 1),
			v059TestN(t), v059TestH(t, "Writing"), v059TestText(t, "Read.", "Read.", 9, 1))
		v059TestParserError(t, result, 4, "Вложенный include запрещён внутри fragment.")
		v059TestRun(t, result, "first", [][4]int{{8, 1, 9, 12}})
		v059TestRun(t, result, "second", [][4]int{{8, 1, 9, 12}})
	})
	for _, body := range []string{"", "Read."} {
		t.Run("mutual references "+body, func(t *testing.T) {
			content := v059TestLine(t, "", "blank", 6, 2)
			if body != "" {
				content = v059TestText(t, body, body, 6, 2)
			}
			result := v059TestDocument(t,
				v059TestFragment(t, "a", 2, 1), v059TestTag(t, "include", "identifier", "b", 3, 2), v059TestClose(t, 3, 2),
				v059TestFragment(t, "b", 2, 1), v059TestTag(t, "include", "identifier", "a", 6, 2),
				content, v059TestClose(t, 6, 2),
				v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "a", 10, 1),
				v059TestN(t), v059TestText(t, "Read.", "Read.", 12, 1))
			v059TestParserError(t, result, 4, "Вложенный include запрещён внутри fragment.")
			v059TestParserError(t, result, 7, "Вложенный include запрещён внутри fragment.")
			v059TestRun(t, result, "current", nil)
		})
	}
}

func TestV059LiteralFragmentDoesNotMakeReferenceAmbiguous(t *testing.T) {
	// DSL 2.8.11: текст комментария не объявляет fragment. Экранированная
	// скобка в его строке не открывает новый блок (DSL 1.21).
	v059TestRun(t, v059TestDocument(t, v059TestOpen(t, "editor", 2, 1),
		v059TestText(t, `@fragment f \{`, "@fragment f {", 3, 2), v059TestClose(t, 3, 2),
		v059TestFragment(t, "f", 2, 1), v059TestLine(t, "", "blank", 6, 2), v059TestClose(t, 6, 2),
		v059TestH(t, "Reading"), v059TestTag(t, "include", "identifier", "f", 9, 1),
		v059TestN(t), v059TestText(t, "Read.", "Read.", 11, 1)),
		"current", [][4]int{{9, 1, 11, 9}})
}

func TestV059PartialParsing(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"recognized newpage with extra argument", []model.Line{
			v059TestH(t, "Reading"), v059TestBroken(t, v059TestN(t), "extra"),
			v059TestText(t, "Read.", "Read.", 4, 1),
		}, [][4]int{{3, 1, 4, 15}}},
		{"recognized newpage with remainder starts pair", []model.Line{
			v059TestBroken(t, v059TestN(t), "extra"), v059TestH(t, "Reading"),
			v059TestText(t, "Read.", "Read.", 4, 1),
		}, [][4]int{{3, 1, 4, 12}}},
		{"recognized displayed tag wins over remainder", []model.Line{
			v059TestH(t, "Reading"),
			v059TestBroken(t, v059TestTag(t, "media", "media-type", "image", 3, 1), `"unterminated`),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, nil},
		{"only erroneous data", []model.Line{
			v059TestH(t, "Reading"), v059TestInvalid(t, "@unknown", 3, 1),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, nil},
		{"content only unparsed", []model.Line{
			v059TestH(t, "Reading"),
			// DSL 4.3.2–4.3.3: два подчёркивания в обычном тексте —
			// ошибочная длина плейсхолдера, а не отображаемая запись.
			v059TestLine(t, "__", "content", 3, 1, v059TestUnparsed(t, "__", 1, 3)),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, nil},
		{"recognized text with unparsed remainder", []model.Line{
			v059TestH(t, "Reading"),
			v059TestBroken(t, v059TestText(t, "Read.", "Read.", 3, 1), "__"),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, nil},
		{"recognized comment with remainder stays excluded", []model.Line{
			v059TestH(t, "Reading"),
			v059TestBroken(t, v059TestTag(t, "editor", "content", "Check.", 3, 1), "}"),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1),
		}, [][4]int{{3, 1, 5, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v059TestRun(t, v059TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV059DisplayedTags(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		kind model.ElementType
		text string
	}{
		{"header", "title", "Practice"}, {"task", "identifier", "a"},
		{"instr", "content", "Read."}, {"example", "content", "A cat."},
		{"wordlist", "content", "cat; dog."},
		{"note", "content", "Remember."}, {"alt", "content", "A cat."},
		{"hint", "content", "Try."}, {"question", "content", "Why?"}, {"multifill", "", ""},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"),
				v059TestTag(t, tc.tag, tc.kind, tc.text, 3, 1), v059TestN(t),
				v059TestText(t, "Read.", "Read.", 5, 1)), "current", nil)
		})
	}
	t.Run("media", func(t *testing.T) {
		v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"),
			v059TestLine(t, "@media image cat.png", "tag", 3, 1,
				v059TestElement(t, "tag", "@media", "media", 1, 7),
				v059TestElement(t, "media-type", "image", "image", 8, 13),
				v059TestElement(t, "source", "cat.png", "cat.png", 14, 21)),
			v059TestN(t), v059TestText(t, "Read.", "Read.", 5, 1)), "current", nil)
	})
	for _, tag := range []string{"table", "script", "text", "choice", "multichoice", "ordering"} {
		t.Run(tag, func(t *testing.T) {
			first, second := "Italy", "Spain"
			if tag == "table" {
				first, second = "Word | Meaning", "cat | animal"
			}
			v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"), v059TestOpen(t, tag, 3, 1),
				v059TestText(t, first, first, 4, 2), v059TestText(t, second, second, 4, 2),
				v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 8, 1)), "current", nil)
		})
	}
	t.Run("matching with separator", func(t *testing.T) {
		v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"), v059TestOpen(t, "matching", 3, 1),
			v059TestText(t, "cat", "cat", 4, 2), v059TestLine(t, "---", "separator", 4, 2),
			v059TestText(t, "animal", "animal", 4, 2), v059TestText(t, "plant", "plant", 4, 2),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 10, 1)), "current", nil)
	})
	t.Run("step and key inside task", func(t *testing.T) {
		// key и step требуют task. speaking/endtask не дают содержимого сами
		// по себе; корректная ситуация уже содержит отображаемое объявление task.
		v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"),
			v059TestTag(t, "task", "identifier", "a", 3, 1), v059TestTag(t, "speaking", "", "", 4, 2),
			v059TestTag(t, "step", "title", "First", 4, 2), v059TestTag(t, "instr", "content", "Read.", 6, 3),
			v059TestOpen(t, "key", 6, 3), v059TestText(t, "Answer.", "Answer.", 8, 4), v059TestClose(t, 8, 4),
			v059TestTag(t, "endtask", "", "", 4, 2), v059TestN(t),
			v059TestText(t, "Read.", "Read.", 12, 1)), "current", nil)
	})
}

func TestV059BranchesAndBlockBoundaries(t *testing.T) {
	t.Run("text in both branches", func(t *testing.T) {
		v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"), v059TestOpen(t, "variants", 3, 1),
			v059TestTag(t, "variant", "name", "A", 4, 2), v059TestText(t, "Read.", "Read.", 5, 3),
			v059TestTag(t, "variant", "name", "B", 4, 2), v059TestText(t, "Write.", "Write.", 7, 3),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 10, 1)), "current", nil)
	})
	t.Run("empty branches and their labels do not fill step", func(t *testing.T) {
		// Пустые ветви ошибочны по DSL 4.9.25. Их распознанные имена
		// остаются структурными объявлениями по условиям V059.
		result := v059TestDocument(t, v059TestH(t, "Reading"), v059TestOpen(t, "variants", 3, 1),
			v059TestTag(t, "variant", "name", "A", 4, 2),
			v059TestTag(t, "editor", "content", "Check A.", 5, 3),
			v059TestTag(t, "variant", "name", "B", 4, 2),
			v059TestTag(t, "editor", "content", "Check B.", 7, 3),
			v059TestClose(t, 4, 2), v059TestN(t), v059TestText(t, "Read.", "Read.", 10, 1))
		v059TestParserError(t, result, 5, "Пустая ветвь.")
		v059TestParserError(t, result, 7, "Пустая ветвь.")
		v059TestRun(t, result, "current", [][4]int{{3, 1, 10, 9}})
	})
	t.Run("definition in a branch is available globally", func(t *testing.T) {
		v059TestRun(t, v059TestDocument(t, v059TestOpen(t, "variants", 2, 1),
			v059TestTag(t, "variant", "name", "A", 3, 2), v059TestFragment(t, "f", 4, 3),
			v059TestText(t, "Read.", "Read.", 5, 4), v059TestClose(t, 5, 4),
			v059TestText(t, "Read.", "Read.", 4, 3),
			v059TestTag(t, "variant", "name", "B", 3, 2), v059TestText(t, "Write.", "Write.", 9, 3),
			v059TestClose(t, 3, 2), v059TestH(t, "Reading"),
			v059TestTag(t, "include", "identifier", "f", 12, 1), v059TestN(t),
			v059TestText(t, "Read.", "Read.", 14, 1)), "current", nil)
	})
	t.Run("newpage deep in block does not create pair", func(t *testing.T) {
		result := v059TestDocument(t, v059TestH(t, "Reading"),
			v059TestOpen(t, "instr", 3, 1), v059TestText(t, "Read.", "Read.", 4, 2),
			v059TestOpen(t, "hint", 4, 2), v059TestTag(t, "newpage", "", "", 6, 3),
			v059TestClose(t, 6, 3), v059TestClose(t, 4, 2), v059TestN(t),
			v059TestText(t, "Read.", "Read.", 10, 1))
		v059TestParserError(t, result, 7, "Тег запрещён внутри hint.")
		v059TestRun(t, result, "current", nil)
	})
}

func TestV059ServiceDeclarations(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		kind model.ElementType
		text string
	}{
		{"document-id", "identifier", "lesson"}, {"section", "name", "Unit"},
		{"order", "number", "1"}, {"resource-dir", "resource-path", "images"},
		{"dsl-version", "version", "1.2"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			// DSL 1.3/1.31.2: размещение метатега здесь ошибочно. Его
			// распознанное служебное значение не становится содержимым шага.
			result := v059TestDocument(t, v059TestH(t, "Reading"),
				v059TestTag(t, tc.tag, tc.kind, tc.text, 3, 1), v059TestN(t),
				v059TestText(t, "Read.", "Read.", 5, 1))
			v059TestParserError(t, result, 4, "Недопустимое размещение метатега.")
			v059TestRun(t, result, "current", [][4]int{{3, 1, 5, 9}})
		})
	}
}

func TestV059EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		endings []model.LineEnding
	}{
		{"LF", []model.LineEnding{"\n", "\n", "\n", "\n", "\n"}},
		{"CRLF", []model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n", "\r\n"}},
		{"mixed", []model.LineEnding{"\r\n", "\n", "\r\n", "\n", "\r\n"}},
		{"EOF", []model.LineEnding{"\n", "\n", "\n", "\n", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				t.Run(strconv.FormatBool(reverse), func(t *testing.T) {
					body := []model.Line{v059TestH(t, "Reading"), v059TestN(t), v059TestText(t, "Read.", "Read.", 4, 1)}
					want := [][4]int{{3, 1, 4, 9}}
					if reverse {
						body[0], body[1] = body[1], body[0]
						want = [][4]int{{3, 1, 4, 12}}
					}
					result := v059TestDocument(t, body...)
					for i, ending := range tc.endings {
						result.Lines[i].LineEnding = ending
					}
					v059TestRun(t, result, "first", want)
					v059TestRun(t, result, "second", want)
					v059TestRun(t, v059TestDocument(t, v059TestH(t, "Reading"),
						v059TestText(t, "Read.", "Read.", 3, 1), v059TestN(t),
						v059TestText(t, "Write.", "Write.", 5, 1)), "separate", nil)
				})
			}
		})
	}
}

func TestV059EmptyLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			// Строки недоступны; версия известна из результата парсера.
			result := &model.Result{
				Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version,
					Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
				Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
			}
			if version == "1.2" {
				v059TestRun(t, result, "current", nil)
				return
			}
			before := v059TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v059TestRegistry(t))
			if err == nil || checks != nil || v059TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v059TestH задаёт корневой заголовок третьего уровня.
func v059TestH(t *testing.T, title string) model.Line {
	t.Helper()
	return v059TestHeading(t, "", "###", "3", title)
}

// v059TestN задаёт корневой тег начала нового шага.
func v059TestN(t *testing.T) model.Line {
	t.Helper()
	return v059TestTag(t, "newpage", "", "", 0, 0)
}

// v059TestFragment задаёт определение с явно указанным ID.
func v059TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v059TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
		v059TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v059TestElement(t, "identifier", id, id, 11, end),
		v059TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v059TestUnparsed задаёт неразобранную часть с отсутствующим Value.
// Документ добавляет требуемую исходную ошибку и ссылку на неё.
func v059TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v059TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v059TestBroken добавляет явно ошибочный остаток к распознанной строке.
func v059TestBroken(t *testing.T, line model.Line, extra string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " " + extra
	line.Elements = append(line.Elements, v059TestUnparsed(t, extra, start, start+utf8.RuneCountInString(extra)))
	return line
}

// v059TestParserError сохраняет ошибку размещения распознанной конструкции,
// не превращая её в unparsed. Это исходные данные, не ожидание результата V059.
func v059TestParserError(t *testing.T, result *model.Result, number int, message string) {
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

// v059TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V059.
func v059TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v059TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v059TestHeading(t, "", "#", "1", title),
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

// v059TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v059TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v059TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v059TestElement(t, "heading-level", marker, level, start, end),
		v059TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v059TestInvalid задаёт ошибочную строку без распознанного heading-level.
func v059TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v059TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v059TestLine(t, raw, "invalid", parent, depth, element)
}

// v059TestTag собирает однострочный тег с одним явно типизированным параметром.
func v059TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v059TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v059TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v059TestLine(t, raw, "tag", parent, depth, elements...)
}

// v059TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v059TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v059TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v059TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v059TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v059TestClose задаёт закрытие блока с явным родителем.
func v059TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v059TestLine(t, "}", "block-end", parent, depth, v059TestElement(t, "block-close", "}", "}", 1, 2))
}

// v059TestText сохраняет независимо заданные исходное и семантическое значения.
func v059TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v059TestLine(t, raw, "content", parent, depth,
		v059TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v059TestLine не классифицирует исходник и не вычисляет родительские связи.
func v059TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v059TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v059TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v059TestRegistry использует штатный каталог целиком.
func v059TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v059TestRun проверяет только новые V059 по явно заданным диапазонам:
// строка начала, колонка начала, строка конца, исключающая колонка конца.
func v059TestRun(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v059TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v059TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v059TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V059" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V059 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[2], Column: position[3]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "block" || entry.Fatal || entry.Message != "Между @newpage и заголовком шага образован пустой шаг." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V059 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v059TestSnapshot(t, result)), &restored); err != nil {
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
	if v059TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v059TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v059TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

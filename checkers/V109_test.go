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

// v109Expected задаёт независимые исходные диапазоны и каноническое имя.
type v109Expected struct {
	line, start, end, tagStart, tagEnd int
	tag                                string
}

func TestV109Registration(t *testing.T) {
	definitions, err := v109TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V109" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У однострочного тега использована блочная форма." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V109 definitions = %d, want 1", count)
	}
}

func TestV109ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		result := v109TestDocument(t, v109TestNamed(t, "question", "content", "What is your name?", 2, 1))
		v109TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@question What is your name?")
		v109TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		// Добавлены только версия и заголовок, пример сохранён буквально.
		result := v109TestDocument(t,
			v109TestBlock(t, "@question What is your name? {", "question", 2, 1,
				v109TestElement(t, "content", "What is your name?", "What is your name?", 11)),
			v109TestText(t, "Write your name.", "Write your name.", 3, 2), v109TestClose(t, 3, 2))
		v109TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@question What is your name? {\nWrite your name.\n}")
		v109TestRun(t, result, "current", []v109Expected{{3, 30, 31, 1, 10, "question"}})
	})
}

func TestV109AllSingleLineTags(t *testing.T) {
	for _, tc := range []struct {
		tag, raw string
		elements []model.Element
		want     v109Expected
	}{
		{"dsl-version", "@dsl-version 1.2", []model.Element{v109TestElement(t, "version", "1.2", "1.2", 14)}, v109Expected{1, 18, 19, 1, 13, "dsl-version"}},
		{"document-id", "@document-id unit", []model.Element{v109TestElement(t, "identifier", "unit", "unit", 14)}, v109Expected{2, 19, 20, 1, 13, "document-id"}},
		{"section", "@section Name", []model.Element{v109TestElement(t, "name", "Name", "Name", 10)}, v109Expected{2, 15, 16, 1, 9, "section"}},
		{"order", "@order 1", []model.Element{v109TestElement(t, "number", "1", "1", 8)}, v109Expected{2, 10, 11, 1, 7, "order"}},
		{"resource-dir", `@resource-dir "assets"`, []model.Element{v109TestElement(t, "resource-path", `"assets"`, "assets", 15)}, v109Expected{2, 24, 25, 1, 14, "resource-dir"}},
		{"header", "@header Title", []model.Element{v109TestElement(t, "title", "Title", "Title", 9)}, v109Expected{3, 15, 16, 1, 8, "header"}},
		{"task", "@task practice", []model.Element{v109TestElement(t, "identifier", "practice", "practice", 7)}, v109Expected{3, 16, 17, 1, 6, "task"}},
		{"endtask", "@endtask", nil, v109Expected{4, 10, 11, 1, 9, "endtask"}},
		{"step", "@step Read", []model.Element{v109TestElement(t, "title", "Read", "Read", 7)}, v109Expected{4, 12, 13, 1, 6, "step"}},
		{"speaking", "@speaking", nil, v109Expected{4, 11, 12, 1, 10, "speaking"}},
		{"newpage", "@newpage", nil, v109Expected{3, 10, 11, 1, 9, "newpage"}},
		{"media", "@media audio 012", []model.Element{
			v109TestElement(t, "media-type", "audio", "audio", 8), v109TestElement(t, "source", "012", "012", 14),
		}, v109Expected{3, 18, 19, 1, 7, "media"}},
		{"include", "@include part", []model.Element{v109TestElement(t, "identifier", "part", "part", 10)}, v109Expected{6, 15, 16, 1, 9, "include"}},
		{"question", "@question What is your name?", []model.Element{
			v109TestElement(t, "content", "What is your name?", "What is your name?", 11),
		}, v109Expected{3, 30, 31, 1, 10, "question"}},
		{"variant", "@variant A", []model.Element{v109TestElement(t, "name", "A", "A", 10)}, v109Expected{4, 12, 13, 1, 9, "variant"}},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			t.Run("parsed block", func(t *testing.T) {
				line := v109TestBlock(t, tc.raw+" {", tc.tag, 0, 0, tc.elements...)
				v109TestRun(t, v109TestTagContext(t, tc.tag, line, true), "current", []v109Expected{tc.want})
			})
			t.Run("unparsed attempt", func(t *testing.T) {
				// Блочная форма запрещена: целиком unparsed имеет реальную причину.
				line := v109TestInvalid(t, tc.raw+" {", 0, 0)
				v109TestRun(t, v109TestTagContext(t, tc.tag, line, false), "current", []v109Expected{tc.want})
			})
			t.Run("single line", func(t *testing.T) {
				elements := append([]model.Element{v109TestElement(t, "tag", "@"+tc.tag, tc.tag, 1)}, tc.elements...)
				line := v109TestLine(t, tc.raw, "tag", 0, 0, elements...)
				v109TestRun(t, v109TestTagContext(t, tc.tag, line, false), "current", nil)
			})
		})
	}
}

func TestV109RawDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v109Expected
	}{
		{"no argument", "@question {", []v109Expected{{3, 11, 12, 1, 10, "question"}}},
		{"case indent and trailing whitespace", " \t@QuEsTiOn Я😀 { \t", []v109Expected{{3, 16, 17, 3, 12, "question"}}},
		{"several spaces", "@question X   {", []v109Expected{{3, 15, 16, 1, 10, "question"}}},
		{"no separator", "@question{", nil},
		{"tab separator before opening", "@question X\t{", nil},
		{"NBSP separator before opening", "@question X\u00a0{", nil},
		{"tag boundary is Unicode space", "@question\u00a0X {", []v109Expected{{3, 13, 14, 1, 10, "question"}}},
		{"tag boundary is tab", "@question\tX {", []v109Expected{{3, 13, 14, 1, 10, "question"}}},
		{"brace is a tag boundary", "@question} {", []v109Expected{{3, 12, 13, 1, 10, "question"}}},
		{"longer name", "@questionExtra X {", nil},
		{"punctuation in name", "@question: X {", nil},
		{"unknown name", "@unknown X {", nil},
		{"non-ASCII initial space is not trimmed", "\u00a0@question X {", nil},
		{"non-ASCII trailing space is not trimmed", "@question X {\u00a0", nil},
		{"ordinary quote does not protect opening", `@question "X {`, []v109Expected{{3, 14, 15, 1, 10, "question"}}},
		{"quoted brace is not terminal", `@question "{"`, nil},
		{"opening followed by text", "@question X { tail", nil},
		{"inline closing", "@question X { }", nil},
		{"unknown escape before active opening", `@question \q {`, []v109Expected{{3, 14, 15, 1, 10, "question"}}},
		{"escaped brace before active opening", `@question \{ X {`, []v109Expected{{3, 16, 17, 1, 10, "question"}}},
		{"escaped closing before active opening", `@question \} X {`, []v109Expected{{3, 16, 17, 1, 10, "question"}}},
		{"paired slash before active opening", `@question \\ X {`, []v109Expected{{3, 16, 17, 1, 10, "question"}}},
		{"two openings still one finding", "@question { {", []v109Expected{{3, 13, 14, 1, 10, "question"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v109TestRun(t, v109TestDocument(t, v109TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	t.Run("escaped brace is literal", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, `@question X \{`, "tag", 2, 1,
				v109TestElement(t, "tag", "@question", "question", 1),
				v109TestElement(t, "content", `X \{`, "X {", 11))), "current", nil)
	})
	t.Run("escaped pseudo tag", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t, v109TestText(t, `\@question X \{`, "@question X {", 2, 1)), "current", nil)
	})
	t.Run("text mention", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t, v109TestText(t, `See @question X \{`, "See @question X {", 2, 1)), "current", nil)
	})
	t.Run("partial invalid with tag", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, "@question X {", "invalid", 2, 1,
				v109TestElement(t, "tag", "@question", "question", 1),
				v109TestElement(t, "content", "X", "X", 11), v109TestUnparsed(t, "{", 13))),
			"current", []v109Expected{{3, 13, 14, 1, 10, "question"}})
	})
	t.Run("tag line with unparsed tail", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, "@question X {", "tag", 2, 1,
				v109TestElement(t, "tag", "@question", "question", 1), v109TestUnparsed(t, "X {", 11))),
			"current", []v109Expected{{3, 13, 14, 1, 10, "question"}})
	})
	t.Run("recognized opening does not need separator", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, "@question{", "block-start", 2, 1,
				v109TestElement(t, "tag", "@question", "question", 1),
				v109TestElement(t, "block-open", "{", "{", 10))),
			"current", []v109Expected{{3, 10, 11, 1, 10, "question"}})
	})
	t.Run("recognized opening does not need terminal position", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, "@question X { tail", "block-start", 2, 1,
				v109TestElement(t, "tag", "@question", "question", 1),
				v109TestElement(t, "content", "X", "X", 11),
				v109TestElement(t, "block-open", "{", "{", 13), v109TestUnparsed(t, "tail", 15))),
			"current", []v109Expected{{3, 13, 14, 1, 10, "question"}})
	})
	t.Run("parsed case and indentation", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, " \t@QUESTION X {", "block-start", 2, 1,
				v109TestElement(t, "tag", "@QUESTION", "question", 3),
				v109TestElement(t, "content", "X", "X", 13),
				v109TestElement(t, "block-open", "{", "{", 15))),
			"current", []v109Expected{{3, 15, 16, 3, 12, "question"}})
	})
}

func TestV109MediaSources(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v109Expected
	}{
		{"unquoted source", "@media audio file {", []v109Expected{{3, 19, 20, 1, 7, "media"}}},
		{"video", "@media video 012 {", []v109Expected{{3, 18, 19, 1, 7, "media"}}},
		{"image", "@media image 012 {", []v109Expected{{3, 18, 19, 1, 7, "media"}}},
		{"invalid type is a separate problem", "@media other 012 {", []v109Expected{{3, 18, 19, 1, 7, "media"}}},
		{"quoted source", `@media audio "a b" {`, []v109Expected{{3, 20, 21, 1, 7, "media"}}},
		{"escaped quote in source", `@media audio "a\"b" {`, []v109Expected{{3, 21, 22, 1, 7, "media"}}},
		{"paired slash before closing quote", `@media audio "a\\" {`, []v109Expected{{3, 20, 21, 1, 7, "media"}}},
		{"escaped braces in source", `@media audio "a\{b\}" {`, []v109Expected{{3, 23, 24, 1, 7, "media"}}},
		{"brace inside and after quoted source", `@media audio "a { b" {`, []v109Expected{{3, 22, 23, 1, 7, "media"}}},
		{"empty quoted source still complete", `@media audio "" {`, []v109Expected{{3, 17, 18, 1, 7, "media"}}},
		{"extra argument does not hide block", `@media audio "a" extra {`, []v109Expected{{3, 24, 25, 1, 7, "media"}}},
		{"brace within unquoted source", "@media audio file{", nil},
		{"brace is the source", "@media audio {", nil},
		{"no type or source", "@media {", nil},
		{"unclosed quoted source", `@media audio "a {`, nil},
		{"escaped quote does not close source", `@media audio "a\" {`, nil},
		{"no separator after source", `@media audio "a"{`, nil},
		{"tab before opening", "@media audio \"a\"\t{", nil},
		{"unparsed media parameters need ordinary spaces", "@media\taudio file {", nil},
		{"unparsed type separator is not tab", "@media audio\tfile {", nil},
		{"source boundary is not NBSP", "@media audio file\u00a0{", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v109TestRun(t, v109TestDocument(t, v109TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`"a \{"`, "a {"}, {`"a \" \{"`, `a " {`}, {`a\{`, "a{"}, {`"a\\\{"`, `a\{`},
	} {
		t.Run("literal source "+tc.raw, func(t *testing.T) {
			v109TestRun(t, v109TestDocument(t,
				v109TestLine(t, "@media audio "+tc.raw, "tag", 2, 1,
					v109TestElement(t, "tag", "@media", "media", 1),
					v109TestElement(t, "media-type", "audio", "audio", 8),
					v109TestElement(t, "source", tc.raw, tc.value, 14))), "current", nil)
		})
	}
	t.Run("recognized source supports partial parsing", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestLine(t, "@media audio\tfile {", "tag", 2, 1,
				v109TestElement(t, "tag", "@media", "media", 1),
				v109TestElement(t, "media-type", "audio", "audio", 8),
				v109TestUnparsed(t, "\t", 13), v109TestElement(t, "source", "file", "file", 14),
				v109TestUnparsed(t, "{", 19))),
			"current", []v109Expected{{3, 19, 20, 1, 7, "media"}})
	})
	t.Run("block-open with missing source", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestBlock(t, "@media audio {", "media", 2, 1, v109TestElement(t, "media-type", "audio", "audio", 8))),
			"current", []v109Expected{{3, 14, 15, 1, 7, "media"}})
	})
	t.Run("source ends before explicit opening", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t,
			v109TestBlock(t, `@media audio "a\{b" {`, "media", 2, 1,
				v109TestElement(t, "media-type", "audio", "audio", 8),
				v109TestElement(t, "source", `"a\{b"`, "a{b", 14))),
			"current", []v109Expected{{3, 21, 22, 1, 7, "media"}})
	})
}

func TestV109ResourcePaths(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v109Expected
	}{
		{"quoted last path", `@resource-dir "assets" {`, []v109Expected{{2, 24, 25, 1, 14, "resource-dir"}}},
		{"several quoted paths", `@resource-dir "a", "b" {`, []v109Expected{{2, 24, 25, 1, 14, "resource-dir"}}},
		{"mixed paths ending in quoted", `@resource-dir a, "b" {`, []v109Expected{{2, 22, 23, 1, 14, "resource-dir"}}},
		{"comma inside path", `@resource-dir "a,b" {`, []v109Expected{{2, 21, 22, 1, 14, "resource-dir"}}},
		{"slash does not escape closing quote", `@resource-dir "a\" {`, []v109Expected{{2, 20, 21, 1, 14, "resource-dir"}}},
		{"two literal slashes", `@resource-dir "a\\" {`, []v109Expected{{2, 21, 22, 1, 14, "resource-dir"}}},
		{"brace inside quoted path and outside", `@resource-dir "a{" {`, []v109Expected{{2, 20, 21, 1, 14, "resource-dir"}}},
		{"empty quoted path", `@resource-dir "" {`, []v109Expected{{2, 18, 19, 1, 14, "resource-dir"}}},
		{"tab around earlier list element", "@resource-dir \"a\"\t,\t\"b\" {", []v109Expected{{2, 25, 26, 1, 14, "resource-dir"}}},
		{"empty list item before quoted path", `@resource-dir , "a" {`, []v109Expected{{2, 21, 22, 1, 14, "resource-dir"}}},
		{"unclosed path", `@resource-dir "a {`, nil},
		{"no separator after quote", `@resource-dir "a"{`, nil},
		{"tab after quote", "@resource-dir \"a\"\t{", nil},
		{"mixed separator after quote", "@resource-dir \"a\"\t {", nil},
		{"trailing comma creates new path", `@resource-dir "a", {`, nil},
		{"error does not turn unquoted path into block", "@resource-dir , assets {", nil},
		{"extra text after quote is not a block guess", `@resource-dir "a" b {`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v109TestRun(t, v109TestResourceDocument(t, v109TestInvalid(t, tc.raw, 0, 0)), "current", tc.want)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{"assets {", "assets {"}, {"assets{", "assets{"},
		{`"assets {"`, "assets {"}, {`C:\assets\{`, `C:\assets\{`},
		{`"C:\assets\{"`, `C:\assets\{`},
	} {
		t.Run("literal path "+tc.raw, func(t *testing.T) {
			line := v109TestLine(t, "@resource-dir "+tc.raw, "tag", 0, 0,
				v109TestElement(t, "tag", "@resource-dir", "resource-dir", 1),
				v109TestElement(t, "resource-path", tc.raw, tc.value, 15))
			v109TestRun(t, v109TestResourceDocument(t, line, tc.value), "current", nil)
		})
	}
	t.Run("last path is unquoted", func(t *testing.T) {
		line := v109TestLine(t, `@resource-dir "a", b {`, "tag", 0, 0,
			v109TestElement(t, "tag", "@resource-dir", "resource-dir", 1),
			v109TestElement(t, "resource-path", `"a"`, "a", 15),
			v109TestElement(t, "resource-path", "b {", "b {", 20))
		v109TestRun(t, v109TestResourceDocument(t, line, "a", "b {"), "current", nil)
	})
	t.Run("recognized block after unquoted path", func(t *testing.T) {
		line := v109TestBlock(t, "@resource-dir assets {", "resource-dir", 0, 0,
			v109TestElement(t, "resource-path", "assets", "assets", 15))
		v109TestRun(t, v109TestResourceDocument(t, line, "assets"), "current",
			[]v109Expected{{2, 22, 23, 1, 14, "resource-dir"}})
	})
	t.Run("parsed last path with unparsed opening", func(t *testing.T) {
		line := v109TestLine(t, `@resource-dir "a" {`, "tag", 0, 0,
			v109TestElement(t, "tag", "@resource-dir", "resource-dir", 1),
			v109TestElement(t, "resource-path", `"a"`, "a", 15), v109TestUnparsed(t, "{", 19))
		v109TestRun(t, v109TestResourceDocument(t, line, "a"), "current",
			[]v109Expected{{2, 19, 20, 1, 14, "resource-dir"}})
	})
	t.Run("V005 behavior is intentionally retained", func(t *testing.T) {
		result := v109TestModel(t, v109TestNamed(t, "dsl-version", "version", "1.2", 0, 0),
			v109TestBlock(t, `@resource-dir "assets" {`, "resource-dir", 0, 0,
				v109TestElement(t, "resource-path", `"assets"`, "assets", 15)),
			v109TestClose(t, 2, 1), v109TestHeading(t, "#", "Lesson"))
		result.Document.Metadata.ResourceDirs = []string{"assets"}
		v109TestRun(t, result, "current", []v109Expected{{2, 24, 25, 1, 14, "resource-dir"}})
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "current" || entry.DiagnosticCode != "V005" {
				continue
			}
			count++
			location := model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 2}}
			if entry.Location == nil || *entry.Location != location ||
				entry.Message != "Нарушено оформление фигурных скобок.\nконкретизация: Закрывающая скобка не имеет открывающей" {
				t.Fatalf("unexpected V005: %+v", entry)
			}
		}
		if count != 1 {
			t.Fatalf("V005 count = %d, want existing single closing finding", count)
		}
	})
}

func TestV109ContextsAndBoundaries(t *testing.T) {
	for _, tag := range []string{"note", "wordlist", "text", "answer", "table", "script", "example", "key", "instr", "alt", "hint", "choice", "multichoice", "matching", "ordering", "variants"} {
		t.Run("not a single-line tag: "+tag, func(t *testing.T) {
			v109TestRun(t, v109TestDocument(t, v109TestOpen(t, tag, 2, 1),
				v109TestText(t, "Text.", "Text.", 3, 2), v109TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("nested real declaration", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t, v109TestOpen(t, "note", 2, 1),
			v109TestBlock(t, "@question X {", "question", 3, 2, v109TestElement(t, "content", "X", "X", 11)),
			v109TestText(t, "Answer.", "Answer.", 4, 3), v109TestClose(t, 4, 3), v109TestClose(t, 3, 2)),
			"current", []v109Expected{{4, 13, 14, 1, 10, "question"}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			// Строка похожа на блок, но внутри литерального контекста не
			// создаёт его; единственная } закрывает editor/multifill.
			v109TestRun(t, v109TestDocument(t, v109TestOpen(t, tag, 2, 1),
				v109TestText(t, "@question X {", "@question X {", 3, 2),
				v109TestText(t, "@media audio 012 {", "@media audio 012 {", 3, 2),
				v109TestClose(t, 3, 2)), "current", nil)
		})
		t.Run("literal ancestor inside note: "+tag, func(t *testing.T) {
			v109TestRun(t, v109TestDocument(t, v109TestOpen(t, "note", 2, 1),
				v109TestOpen(t, tag, 3, 2), v109TestText(t, "@question X {", "@question X {", 4, 3),
				v109TestClose(t, 4, 3), v109TestClose(t, 3, 2)), "current", nil)
		})
		t.Run("partially parsed literal opening: "+tag, func(t *testing.T) {
			// Парсер сохранил границу блока, но ошибочное имя без пробела
			// осталось в unparsed. Контекст определяется по Raw родителя.
			opening := v109TestLine(t, "@"+tag+"{", "block-start", 2, 1,
				v109TestUnparsed(t, "@"+tag, 1), v109TestElement(t, "block-open", "{", "{", len(tag)+2))
			v109TestRun(t, v109TestDocument(t, opening,
				v109TestText(t, "@question X {", "@question X {", 3, 2), v109TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("no content and no closing", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t, v109TestOpen(t, "question", 2, 1)),
			"current", []v109Expected{{3, 11, 12, 1, 10, "question"}})
	})
	t.Run("separate declarations stay separate", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t, v109TestOpen(t, "question", 2, 1), v109TestClose(t, 3, 2),
			v109TestLine(t, "", "blank", 2, 1), v109TestOpen(t, "question", 2, 1), v109TestClose(t, 6, 2)),
			"current", []v109Expected{{3, 11, 12, 1, 10, "question"}, {6, 11, 12, 1, 10, "question"}})
	})
	t.Run("opening on another line is not this diagnostic", func(t *testing.T) {
		v109TestRun(t, v109TestDocument(t, v109TestNamed(t, "question", "content", "X", 2, 1),
			v109TestInvalid(t, "{", 2, 1)), "current", nil)
	})
}

func TestV109EndingsVersionsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"no final EOL", "\n", ""}, {"CRLF no final EOL", "\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v109TestDocument(t, v109TestInvalid(t, "@question X {", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v109TestRun(t, result, "first", []v109Expected{{3, 13, 14, 1, 10, "question"}})
			v109TestRun(t, result, "second", []v109Expected{{3, 13, 14, 1, 10, "question"}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v109TestModel(t)
			result.Document.Metadata.Title = nil
			if !known {
				result.Document.LineCount = nil
			}
			v109TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v109TestDocument(t, v109TestOpen(t, "question", 2, 1))
			result.Document.DSLVersion = &version
			before := v109TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v109TestRegistry(t))
			if err == nil || checks != nil || v109TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v109TestResourceDocument сохраняет метатег до первого заголовка.
func v109TestResourceDocument(t *testing.T, line model.Line, paths ...string) *model.Result {
	t.Helper()
	result := v109TestModel(t, v109TestNamed(t, "dsl-version", "version", "1.2", 0, 0),
		line, v109TestHeading(t, "#", "Lesson"))
	result.Document.Metadata.ResourceDirs = append([]string{}, paths...)
	return result
}

// v109TestBlock задаёт распознанное открытие в конце явно заданной строки.
// Вычисление длины служит только входной модели, не ожиданиям диагностики.
func v109TestBlock(t *testing.T, raw, name string, parent, depth int, arguments ...model.Element) model.Line {
	t.Helper()
	elements := append([]model.Element{v109TestElement(t, "tag", "@"+name, name, 1)}, arguments...)
	elements = append(elements, v109TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(raw)))
	return v109TestLine(t, raw, "block-start", parent, depth, elements...)
}

// v109TestTagContext помещает объявление в полный контекст его DSL-тега.
// closed добавляет закрытие только распознанной блочной попытки.
func v109TestTagContext(t *testing.T, tag string, line model.Line, closed bool) *model.Result {
	t.Helper()
	var lines []model.Line
	target, parent, depth := 3, 2, 1
	switch tag {
	case "dsl-version":
		target, parent, depth = 1, 0, 0
	case "document-id", "section", "order", "resource-dir":
		lines = []model.Line{v109TestNamed(t, "dsl-version", "version", "1.2", 0, 0)}
		target, parent, depth = 2, 0, 0
	default:
		lines = []model.Line{v109TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v109TestHeading(t, "#", "Lesson")}
		switch tag {
		case "endtask", "step", "speaking":
			lines = append(lines, v109TestNamed(t, "task", "identifier", "practice", 2, 1))
			target, parent, depth = 4, 3, 2
		case "variant":
			lines = append(lines, v109TestOpen(t, "variants", 2, 1))
			target, parent, depth = 4, 3, 2
		case "include":
			lines = append(lines, v109TestLine(t, "@fragment part {", "block-start", 2, 1,
				v109TestElement(t, "tag", "@fragment", "fragment", 1),
				v109TestElement(t, "identifier", "part", "part", 11),
				v109TestElement(t, "block-open", "{", "{", 16)),
				v109TestText(t, "Read.", "Read.", 3, 2), v109TestClose(t, 3, 2))
			target = 6
		}
	}
	line.ParentLine = nil
	if parent != 0 {
		line.ParentLine = &parent
	}
	line.NestingLevel = depth
	lines = append(lines, line)
	if closed {
		lines = append(lines, v109TestClose(t, target, depth+1))
	}
	switch tag {
	case "dsl-version", "document-id", "section", "order", "resource-dir":
		lines = append(lines, v109TestHeading(t, "#", "Lesson"))
	case "variant":
		lines = append(lines, v109TestClose(t, 3, 2))
	}
	result := v109TestModel(t, lines...)
	switch tag {
	case "document-id":
		value := "unit"
		result.Document.Metadata.DocumentID = &value
	case "section":
		value := "Name"
		result.Document.Metadata.Section = &value
	case "order":
		value := "1"
		result.Document.Metadata.Order = &value
	case "resource-dir":
		result.Document.Metadata.ResourceDirs = []string{"assets"}
	}
	return result
}

// v109TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v109TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{
		v109TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v109TestHeading(t, "#", "Lesson"),
	}, body...)
	return v109TestModel(t, lines...)
}

// v109TestModel принимает готовые строки без разбора исходного DSL.
func v109TestModel(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines = append([]model.Line{}, lines...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if count != 0 && lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v109TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v109TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v109TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v109TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v109TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v109TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v109TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v109TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v109TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v109TestLine(t, raw, "invalid", parent, depth, v109TestUnparsed(t, raw, 1))
}

func v109TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v109TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v109TestElement(t, "tag", "@"+tag, tag, 1), v109TestElement(t, kind, value, value, len(tag)+3))
}

func v109TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v109TestLine(t, marker+" "+title, "heading", 0, 0,
		v109TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v109TestElement(t, "title", title, title, len(marker)+2))
}

func v109TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v109TestLine(t, raw, "content", parent, depth, v109TestElement(t, "content", raw, value, 1))
}

func v109TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v109TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v109TestElement(t, "tag", "@"+tag, tag, 1), v109TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v109TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v109TestLine(t, "}", "block-end", parent, depth, v109TestElement(t, "block-close", "}", "}", 1))
}

func v109TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v109TestRun выполняет полный каталог и сравнивает только новые V109.
// Ожидания задают основной и связанный диапазоны с исключёнными концами.
func v109TestRun(t *testing.T, result *model.Result, processingID string, want []v109Expected) {
	t.Helper()
	before := v109TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v109TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v109TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V109" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V109 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start},
			End: model.Position{Line: position.line, Column: position.end}}
		related := model.Location{Start: model.Position{Line: position.line, Column: position.tagStart},
			End: model.Position{Line: position.line, Column: position.tagEnd}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("occurrence %d = %+v, want location %+v and related %+v", i, entry, location, related)
		}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "У однострочного тега использована блочная форма.\nконкретизация: Тег @"+position.tag+" поддерживает только однострочную запись" {
			t.Fatalf("unexpected V109 metadata: %+v", entry)
		}
	}
	// CheckModel проверяет уникальность ID, ссылки и флаги всех диагностик.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v109TestSnapshot(t, result)), &restored); err != nil {
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
	if v109TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v109TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v109TestSource(t *testing.T, result *model.Result, want string) {
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

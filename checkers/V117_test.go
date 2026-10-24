package checkers

import (
	"encoding/json"
	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// v117TestExpected хранит явные исходные диапазоны с исключённым концом.
type v117TestExpected struct {
	main    [4]int
	related [][3]int
}

func TestV117Registration(t *testing.T) {
	definitions, err := v117TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V117" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "В тексте есть разделённые обычным текстом строки из прописных букв." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V117 definitions = %d, want 1", count)
	}
}

func TestV117ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		rows           []string
		want           []v117TestExpected
	}{
		{"reference", "@text Cities {\nLondon is a large city.\nParis is another large city.\n}", []string{"London is a large city.", "Paris is another large city."}, nil},
		{"example", "@text Cities {\nLONDON\nLondon is a large city.\nPARIS\nParis is another large city.\n}", []string{"LONDON", "London is a large city.", "PARIS", "Paris is another large city."}, []v117TestExpected{{[4]int{3, 1, 8, 2}, [][3]int{{4, 1, 7}, {6, 1, 6}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opening := v117TestLine(t, "@text Cities {", "block-start", 2, 1,
				v117TestElement(t, "tag", "@text", "text", 1, 6), v117TestElement(t, "title", "Cities", "Cities", 7, 13), v117TestElement(t, "block-open", "{", "{", 14, 15))
			body := []model.Line{opening}
			for _, raw := range tc.rows {
				body = append(body, v117TestText(t, raw, raw, 3, 2))
			}
			body = append(body, v117TestClose(t, 3, 2))
			result := v117TestLesson(t, body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// Добавлены только обязательные версия и заголовок; эталон и пример дословны.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("fragment changed")
			}
			v117TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV117GroupsAndVisibleText(t *testing.T) {
	text := func(raw string) model.Line { return v117TestText(t, raw, raw, 3, 2) }
	literal := func(raw, value string) model.Line { return v117TestText(t, raw, value, 3, 2) }
	blank := func(raw string) model.Line { return v117TestLine(t, raw, "blank", 3, 2) }
	// Ожидания заданы координатами, независимо от числа и содержимого входных строк.
	basic := []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 2}, {6, 1, 2}}}}
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v117TestExpected
	}{
		{"empty", nil, nil},
		{"blank only", []model.Line{blank(""), blank(" \t")}, nil},
		{"only one group", []model.Line{text("A"), text("B"), text("text")}, nil},
		{"adjacent candidates", []model.Line{text("A"), text("B"), text("text"), text("C")}, []v117TestExpected{{[4]int{3, 1, 8, 2}, [][3]int{{4, 1, 2}, {7, 1, 2}}}}},
		{"blank is insufficient", []model.Line{text("A"), blank(""), text("B")}, nil},
		{"spaces are insufficient", []model.Line{text("A"), blank(" \t"), text("B")}, nil},
		{"br is insufficient", []model.Line{text("A"), text("<br>"), text("B")}, nil},
		{"mixed case br is insufficient", []model.Line{text("A"), literal(" \t<BR>\t ", "<BR>"), text("B")}, nil},
		{"text after br", []model.Line{text("A"), text("<br>"), text("text"), text("B")}, []v117TestExpected{{[4]int{3, 1, 8, 2}, [][3]int{{4, 1, 2}, {7, 1, 2}}}}},
		{"same word", []model.Line{text("A"), text("text"), text("A")}, basic},
		{"first pair only", []model.Line{text("A"), text("text"), text("B"), text("text"), text("C")}, []v117TestExpected{{[4]int{3, 1, 9, 2}, [][3]int{{4, 1, 2}, {6, 1, 2}}}}},
		{"earliest group across blank", []model.Line{text("A"), blank(""), text("B"), text("text"), text("C")}, []v117TestExpected{{[4]int{3, 1, 9, 2}, [][3]int{{4, 1, 2}, {8, 1, 2}}}}},
		{"lowercase disqualifies", []model.Line{text("Aa"), text("text"), text("B")}, nil},
		{"Cyrillic", []model.Line{text("МОСКВА"), text("Текст."), text("ПАРИЖ")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 7}, {6, 1, 6}}}}},
		{"no cased letters", []model.Line{text("123!"), text("text"), text("456?")}, nil},
		{"uncased script", []model.Line{text("東京"), text("text"), text("北京")}, nil},
		{"punctuation and digits", []model.Line{text("A 123!"), text("text"), text("B?")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 7}, {6, 1, 3}}}}},
		{"uncased ordinary separator", []model.Line{text("A"), text("123!"), text("B")}, basic},
		{"titlecase has no lowercase", []model.Line{text("ǅ"), text("text"), text("B")}, basic},
		{"other lowercase property", []model.Line{text("Aᵃ"), text("text"), text("B")}, nil},
		{"formatting and case", []model.Line{text("<strong>A</strong>"), text("text"), text("<Em>B</eM>")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 19}, {6, 1, 11}}}}},
		{"nested formatting", []model.Line{text("<u><strong>A</strong></u>"), text("<em>text</em>"), text("B")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 26}, {6, 1, 2}}}}},
		{"empty formatting", []model.Line{text("<u></u>"), text("text"), text("B")}, nil},
		{"empty formatting not separator", []model.Line{text("A"), text("<u> </u>"), text("B")}, nil},
		{"error not separator", []model.Line{text("A"), text("<em>text"), text("B")}, nil},
		{"error does not erase ordinary evidence", []model.Line{text("A"), text("text"), text("<u>"), text("B")}, []v117TestExpected{{[4]int{3, 1, 8, 2}, [][3]int{{4, 1, 2}, {7, 1, 2}}}}},
		{"literal angles", []model.Line{literal(`\<A\>`, "<A>"), text("text"), text("B")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 6}, {6, 1, 2}}}}},
		{"literal marker letters count", []model.Line{literal(`\<strong\>A`, "<strong>A"), text("text"), text("B")}, nil},
		{"paired backslash before marker", []model.Line{literal(`\\<u>A</u>`, `\<u>A</u>`), text("text"), text("B")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 11}, {6, 1, 2}}}}},
		{"unknown escape retained", []model.Line{text(`\A`), text("text"), text("B")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 3}, {6, 1, 2}}}}},
		{"escaped leading tag and braces", []model.Line{literal(`\@A\{\}`, "@A{}"), text("text"), text("B")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 8}, {6, 1, 2}}}}},
		{"escaped underscore run", []model.Line{literal(`\__A`, "__A"), text("text"), text("B")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 5}, {6, 1, 2}}}}},
		{"unescaped placeholder remains visible", []model.Line{text("_____"), text("text"), text("B")}, nil},
		{"raw indentation and Unicode columns", []model.Line{literal(" \tЯ😀 ", "Я😀"), text("text"), text("Б")}, []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 6}, {6, 1, 2}}}}},
		{"no entity decoding", []model.Line{text("&#65;"), text("text"), text("B")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v117TestRun(t, v117TestBlock(t, tc.rows...), "current", tc.want) })
	}
	for _, raw := range []string{"<u>A", "A</u>", "<u><em>A</u></em>", "<u x='y'>A</u>", "<b>A</b>", "<u/>A", "<br>A", "<br/>", "<u A", "A>"} {
		t.Run("invalid HTML/"+raw, func(t *testing.T) {
			result := v117TestBlock(t, text(raw), text("text"), text("B"))
			v117TestParserError(t, result, 4, 4, utf8.RuneCountInString(raw)+1, "line")
			v117TestRun(t, result, "current", nil)
		})
	}
}

func TestV117Contexts(t *testing.T) {
	text := func(raw string, parent, depth int) model.Line { return v117TestText(t, raw, raw, parent, depth) }
	t.Run("title ignored", func(t *testing.T) {
		opening := v117TestLine(t, "@text TITLE {", "block-start", 2, 1,
			v117TestElement(t, "tag", "@text", "text", 1, 6), v117TestElement(t, "title", "TITLE", "TITLE", 7, 12), v117TestElement(t, "block-open", "{", "{", 13, 14))
		v117TestRun(t, v117TestLesson(t, opening, text("text", 3, 2), text("A", 3, 2), v117TestClose(t, 3, 2)), "current", nil)
	})
	for _, tag := range []string{"note", "text"} {
		t.Run("nested barrier/"+tag, func(t *testing.T) {
			result := v117TestLesson(t, v117TestOpen(t, "text", 2, 1), text("A", 3, 2), text("text", 3, 2),
				v117TestOpen(t, tag, 3, 2), text("x", 6, 3), v117TestClose(t, 6, 3), text("B", 3, 2), v117TestClose(t, 3, 2))
			// Модель сохраняет запрещённое вложение вместе с отдельной ошибкой контекста.
			v117TestParserError(t, result, 6, 6, 8, "line")
			v117TestRun(t, result, "current", nil)
		})
		t.Run("nested content/"+tag, func(t *testing.T) {
			result := v117TestLesson(t, v117TestOpen(t, "text", 2, 1), v117TestOpen(t, tag, 3, 2),
				text("A", 4, 3), text("text", 4, 3), text("B", 4, 3), v117TestClose(t, 4, 3), v117TestClose(t, 3, 2))
			v117TestParserError(t, result, 4, 4, 8, "line")
			var want []v117TestExpected
			if tag == "text" {
				want = []v117TestExpected{{[4]int{4, 1, 8, 2}, [][3]int{{5, 1, 2}, {7, 1, 2}}}}
			}
			v117TestRun(t, result, "current", want)
		})
	}
	t.Run("inline barrier", func(t *testing.T) {
		note := v117TestLine(t, "@note x", "tag", 3, 2, v117TestElement(t, "tag", "@note", "note", 1, 6), v117TestElement(t, "content", "x", "x", 7, 8))
		result := v117TestBlock(t, text("A", 3, 2), text("text", 3, 2), note, text("B", 3, 2))
		v117TestParserError(t, result, 6, 6, 8, "line")
		v117TestRun(t, result, "current", nil)
	})
	t.Run("comparison restarts after nested block", func(t *testing.T) {
		result := v117TestLesson(t, v117TestOpen(t, "text", 2, 1), text("A", 3, 2), v117TestOpen(t, "note", 3, 2),
			text("x", 5, 3), v117TestClose(t, 5, 3), text("B", 3, 2), text("text", 3, 2), text("C", 3, 2), v117TestClose(t, 3, 2))
		v117TestParserError(t, result, 5, 5, 8, "line")
		v117TestRun(t, result, "current", []v117TestExpected{{[4]int{3, 1, 11, 2}, [][3]int{{8, 1, 2}, {10, 1, 2}}}})
	})
	t.Run("text inside fragment", func(t *testing.T) {
		fragment := v117TestLine(t, "@fragment f {", "block-start", 2, 1, v117TestElement(t, "tag", "@fragment", "fragment", 1, 10),
			v117TestElement(t, "identifier", "f", "f", 11, 12), v117TestElement(t, "block-open", "{", "{", 13, 14))
		result := v117TestLesson(t, fragment, v117TestOpen(t, "text", 3, 2), text("A", 4, 3), text("text", 4, 3), text("B", 4, 3), v117TestClose(t, 4, 3), v117TestClose(t, 3, 2))
		v117TestRun(t, result, "current", []v117TestExpected{{[4]int{4, 1, 8, 2}, [][3]int{{5, 1, 2}, {7, 1, 2}}}})
	})
	t.Run("multiple independent blocks", func(t *testing.T) {
		result := v117TestLesson(t, v117TestOpen(t, "text", 2, 1), text("A", 3, 2), text("text", 3, 2), text("B", 3, 2), v117TestClose(t, 3, 2),
			v117TestOpen(t, "text", 2, 1), text("A", 8, 2), text("text", 8, 2), text("B", 8, 2), v117TestClose(t, 8, 2))
		v117TestRun(t, result, "current", []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 2}, {6, 1, 2}}}, {[4]int{8, 1, 12, 2}, [][3]int{{9, 1, 2}, {11, 1, 2}}}})
	})
	t.Run("outer and inner blocks both reported", func(t *testing.T) {
		result := v117TestLesson(t, v117TestOpen(t, "text", 2, 1), text("A", 3, 2), text("text", 3, 2), text("B", 3, 2),
			v117TestOpen(t, "text", 3, 2), text("C", 7, 3), text("text", 7, 3), text("D", 7, 3),
			v117TestClose(t, 7, 3), v117TestClose(t, 3, 2))
		v117TestParserError(t, result, 7, 7, 8, "line")
		v117TestRun(t, result, "current", []v117TestExpected{
			{[4]int{3, 1, 12, 2}, [][3]int{{4, 1, 2}, {6, 1, 2}}},
			{[4]int{7, 1, 11, 2}, [][3]int{{8, 1, 2}, {10, 1, 2}}},
		})
	})
	for _, tag := range []string{"editor", "multifill", "note"} {
		t.Run("other context/"+tag, func(t *testing.T) {
			result := v117TestLesson(t, v117TestOpen(t, tag, 2, 1), text("A", 3, 2), text("text", 3, 2), text("B", 3, 2), v117TestClose(t, 3, 2))
			v117TestRun(t, result, "current", nil)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo text/"+tag, func(t *testing.T) {
			result := v117TestLesson(t, v117TestOpen(t, tag, 2, 1), text("@text {", 3, 2), text("A", 3, 2), text("text", 3, 2), text("B", 3, 2), v117TestClose(t, 3, 2))
			v117TestRun(t, result, "current", nil)
		})
	}
	t.Run("Raw is authoritative", func(t *testing.T) {
		row := v117TestLine(t, "<u>A</u>", "content", 3, 2, v117TestElement(t, "content", "A", "A", 4, 5))
		v117TestRun(t, v117TestBlock(t, row, text("text", 3, 2), text("B", 3, 2)), "current", []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 9}, {6, 1, 2}}}})
	})
	t.Run("unparsed faulty markup excluded", func(t *testing.T) {
		row := v117TestLine(t, "<u>A", "content", 3, 2, v117TestElement(t, "unparsed", "<u>A", "", 1, 5))
		v117TestRun(t, v117TestBlock(t, row, text("text", 3, 2), text("B", 3, 2)), "current", nil)
	})
}

func TestV117Lifecycle(t *testing.T) {
	text := func(raw string) model.Line { return v117TestText(t, raw, raw, 3, 2) }
	want := []v117TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 2}, {6, 1, 2}}}}
	t.Run("empty input", func(t *testing.T) {
		result := v117TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v117TestRun(t, result, "current", nil)
	})
	t.Run("no text blocks", func(t *testing.T) { v117TestRun(t, v117TestLesson(t), "current", nil) })
	t.Run("uppercase tag", func(t *testing.T) {
		result := v117TestBlock(t, text("A"), text("text"), text("B"))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@TEXT {", "@TEXT"
		v117TestRun(t, result, "current", want)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v117TestBlock(t, text("A"), text("text"), text("B"))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[6].LineEnding = ""
				}
				v117TestRun(t, result, "current", want)
			})
		}
	}
	t.Run("unclosed at EOF", func(t *testing.T) {
		result := v117TestLesson(t, v117TestOpen(t, "text", 2, 1), text("A"), text("text"), text("B"))
		result.Lines[5].LineEnding = ""
		v117TestParserError(t, result, 3, 6, 2, "block")
		v117TestRun(t, result, "current", []v117TestExpected{{[4]int{3, 1, 6, 2}, [][3]int{{4, 1, 2}, {6, 1, 2}}}})
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v117TestBlock(t, text("A"), text("text"), text("B"))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v117TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v117TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v117TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v117TestBlock(t, text("A"), text("text"), text("B"))
		second := v117TestBlock(t, text("A"), text("B"))
		v117TestRun(t, first, "first", want)
		v117TestRun(t, second, "first", nil)
		v117TestRun(t, first, "second", want)
	})
}

// v117TestParserError фиксирует отдельную ошибку вложения или незакрытого блока.
func v117TestParserError(t *testing.T, result *model.Result, startLine, endLine, endColumn int, scope diagnostics.Scope) {
	t.Helper()
	result.Lines[startLine-1].HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-context-error", Source: "parser", DiagnosticCode: "fixture-context-error", SeverityLevel: "error", DiagnosticScope: scope,
		Message: "Ошибка структуры блока.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: startLine, Column: 1}, End: model.Position{Line: endLine, Column: endColumn}},
	})
}

// v117TestBlock добавляет блок text вокруг явно заданных строк с родителем 3.
func v117TestBlock(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v117TestOpen(t, "text", 2, 1)}, rows...)
	body = append(body, v117TestClose(t, 3, 2))
	return v117TestLesson(t, body...)
}

func v117TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v117TestLine(t, raw, "content", parent, depth,
		v117TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v117TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v117TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v117TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v117TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v117TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v117TestLine(t, "# Lesson", "heading", 0, 0,
			v117TestElement(t, "heading-level", "#", "1", 1, 2), v117TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
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
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v117TestOpen задаёт реальное объявление блока с явным родителем.
func v117TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v117TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v117TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v117TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v117TestClose задаёт закрытие указанного блока.
func v117TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v117TestLine(t, "}", "block-end", parent, depth, v117TestElement(t, "block-close", "}", "}", 1, 2))
}

// v117TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v117TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v117TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v117TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v117TestRegistry использует весь штатный каталог регистраций.
func v117TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v117TestRun проверяет только новые V117 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v117TestRun(t *testing.T, result *model.Result, source string, want []v117TestExpected) {
	t.Helper()
	before := v117TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v117TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v117TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V117" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V117 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.main[0], Column: position.main[1]}, End: model.Position{Line: position.main[2], Column: position.main[3]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(position.related) ||
			entry.ID == "" || entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Message != "В тексте есть разделённые обычным текстом строки из прописных букв.\nконкретизация: Проверьте, не объединены ли здесь несколько самостоятельных текстов. При подтверждении разделите их на отдельные @text с собственными названиями" {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V117 metadata", i, entry, location)
		}
		for j, columns := range position.related {
			related := model.Location{Start: model.Position{Line: columns[0], Column: columns[1]}, End: model.Position{Line: columns[0], Column: columns[2]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("occurrence %d related %d = %+v, want %+v", i, j, entry.RelatedLocations[j], related)
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v117TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Производные флаги и ссылки проверены CheckModel. В копии возвращаем
	// прежние значения этих полей и удаляем только новые записи запуска.
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
	if v117TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v117TestSnapshot сохраняет независимое представление модели.
func v117TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

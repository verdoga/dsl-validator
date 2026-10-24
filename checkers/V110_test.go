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

// v110TestExpected задаёт исходный диапазон серии с исключающим концом.
type v110TestExpected struct{ startLine, startColumn, endLine, endColumn int }

func TestV110Registration(t *testing.T) {
	definitions, err := v110TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V110" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "Внутри @text есть пустые строки исходного текста." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V110 definitions = %d, want 1", count)
	}
}

func TestV110ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		middle         model.Line
		want           []v110TestExpected
	}{
		{"reference", "@text {\nFirst paragraph.\n<br>\nSecond paragraph.\n}",
			v110TestText(t, "<br>", "<br>", 3, 2), nil},
		{"example", "@text {\nFirst paragraph.\n\nSecond paragraph.\n}",
			v110TestBlank(t, "", 3, 2), []v110TestExpected{{4, 17, 6, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v110TestBlock(t, v110TestText(t, "First paragraph.", "First paragraph.", 3, 2),
				tc.middle, v110TestText(t, "Second paragraph.", "Second paragraph.", 3, 2))
			// Добавлены только обязательные версия и заголовок; HTML-разметка
			// таблицы задания означает буквальный <br> и физические переводы строк.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v110TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV110SequencesAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v110TestExpected
	}{
		{"no content lines", nil, nil},
		{"no blanks", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestText(t, "B.", "B.", 3, 2)}, nil},
		{"one blank", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestText(t, "B.", "B.", 3, 2)}, []v110TestExpected{{4, 3, 6, 1}}},
		{"two blanks", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestBlank(t, "", 3, 2), v110TestText(t, "B.", "B.", 3, 2)}, []v110TestExpected{{4, 3, 7, 1}}},
		{"three blanks", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestBlank(t, "", 3, 2), v110TestBlank(t, "", 3, 2), v110TestText(t, "B.", "B.", 3, 2)}, []v110TestExpected{{4, 3, 8, 1}}},
		{"spaces and tabs", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, " \t ", 3, 2),
			v110TestText(t, "B.", "B.", 3, 2)}, []v110TestExpected{{4, 3, 6, 1}}},
		{"mixed blank spellings form one run", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestBlank(t, " \t", 3, 2), v110TestBlank(t, "  ", 3, 2), v110TestText(t, "B.", "B.", 3, 2)}, []v110TestExpected{{4, 3, 8, 1}}},
		{"leading blank", []model.Line{v110TestBlank(t, "", 3, 2), v110TestText(t, "A.", "A.", 3, 2)}, []v110TestExpected{{3, 8, 5, 1}}},
		{"trailing blank", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2)}, []v110TestExpected{{4, 3, 6, 1}}},
		{"only blank", []model.Line{v110TestBlank(t, "", 3, 2)}, []v110TestExpected{{3, 8, 5, 1}}},
		{"only several blanks", []model.Line{v110TestBlank(t, "", 3, 2), v110TestBlank(t, "\t", 3, 2)}, []v110TestExpected{{3, 8, 6, 1}}},
		{"three separate runs", []model.Line{v110TestBlank(t, "", 3, 2), v110TestText(t, "Alpha.", "Alpha.", 3, 2),
			v110TestBlank(t, "\t ", 3, 2), v110TestBlank(t, "", 3, 2), v110TestText(t, "Beta.", "Beta.", 3, 2),
			v110TestBlank(t, "", 3, 2)}, []v110TestExpected{{3, 8, 5, 1}, {5, 7, 8, 1}, {8, 6, 10, 1}}},
		{"Unicode previous line", []model.Line{v110TestText(t, "Я😀", "Я😀", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestText(t, "B.", "B.", 3, 2)}, []v110TestExpected{{4, 3, 6, 1}}},
		{"raw end and physical next line start", []model.Line{v110TestText(t, " A. \t", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestText(t, " \tB.", "B.", 3, 2)}, []v110TestExpected{{4, 6, 6, 1}}},
		{"br separates blank runs", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestText(t, "<br>", "<br>", 3, 2), v110TestBlank(t, "", 3, 2), v110TestText(t, "B.", "B.", 3, 2)},
			[]v110TestExpected{{4, 3, 6, 1}, {6, 5, 8, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v110TestRun(t, v110TestBlock(t, tc.rows...), "current", tc.want)
		})
	}
}

func TestV110RawRepresentations(t *testing.T) {
	for _, char := range []string{"\u00a0", "\u2003", "\u200b", "\v", "\f", "\u0085", "\u1680", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("other whitespace is not empty/"+strconv.Quote(char), func(t *testing.T) {
			v110TestRun(t, v110TestBlock(t, v110TestText(t, " \t"+char+"\t ", char, 3, 2)), "current", nil)
		})
	}
	for _, raw := range []string{"<br>", "<BR>", "<br/>", "<strong></strong>", "---", `\`} {
		t.Run("nonempty source/"+raw, func(t *testing.T) {
			// Ошибки HTML-формы относятся к другим диагностикам: Raw остаётся непустым.
			v110TestRun(t, v110TestBlock(t, v110TestText(t, raw, raw, 3, 2)), "current", nil)
		})
	}
	for _, kind := range []model.LineType{"blank", "content", "invalid"} {
		t.Run("whitespace unparsed/"+string(kind), func(t *testing.T) {
			row := v110TestLine(t, " \t ", kind, 3, 2, v110TestElement(t, "unparsed", " \t ", "", 1, 4))
			v110TestRun(t, v110TestBlock(t, v110TestText(t, "A.", "A.", 3, 2), row,
				v110TestText(t, "B.", "B.", 3, 2)), "current", []v110TestExpected{{4, 3, 6, 1}})
		})
	}
	t.Run("whitespace content has empty semantic value", func(t *testing.T) {
		v110TestRun(t, v110TestBlock(t, v110TestText(t, "A.", "A.", 3, 2), v110TestText(t, " \t ", "", 3, 2),
			v110TestText(t, "B.", "B.", 3, 2)), "current", []v110TestExpected{{4, 3, 6, 1}})
	})
	t.Run("nonempty unparsed splits runs", func(t *testing.T) {
		v110TestRun(t, v110TestBlock(t, v110TestBlank(t, "", 3, 2),
			v110TestLine(t, "<unknown>", "content", 3, 2, v110TestElement(t, "unparsed", "<unknown>", "", 1, 10)),
			v110TestBlank(t, "", 3, 2)), "current", []v110TestExpected{{3, 8, 5, 1}, {5, 10, 7, 1}})
	})
	t.Run("nonempty invalid is not empty", func(t *testing.T) {
		v110TestRun(t, v110TestBlock(t, v110TestLine(t, "@unknown", "invalid", 3, 2,
			v110TestElement(t, "unparsed", "@unknown", "", 1, 9))), "current", nil)
	})
}

func TestV110Contexts(t *testing.T) {
	t.Run("text inside fragment", func(t *testing.T) {
		fragment := v110TestLine(t, "@fragment f {", "block-start", 2, 1,
			v110TestElement(t, "tag", "@fragment", "fragment", 1, 10), v110TestElement(t, "identifier", "f", "f", 11, 12),
			v110TestElement(t, "block-open", "{", "{", 13, 14))
		v110TestRun(t, v110TestLesson(t, fragment, v110TestOpen(t, "text", 3, 2),
			v110TestText(t, "A.", "A.", 4, 3), v110TestBlank(t, "", 4, 3), v110TestText(t, "B.", "B.", 4, 3),
			v110TestClose(t, 4, 3), v110TestClose(t, 3, 2)), "current", []v110TestExpected{{5, 3, 7, 1}})
	})
	t.Run("nested block excluded and breaks runs", func(t *testing.T) {
		result := v110TestLesson(t, v110TestOpen(t, "text", 2, 1), v110TestText(t, "A.", "A.", 3, 2),
			v110TestBlank(t, "", 3, 2), v110TestOpen(t, "note", 3, 2), v110TestBlank(t, "", 6, 3),
			v110TestText(t, "N.", "N.", 6, 3), v110TestBlank(t, "", 6, 3), v110TestClose(t, 6, 3),
			v110TestBlank(t, "", 3, 2), v110TestText(t, "B.", "B.", 3, 2), v110TestClose(t, 3, 2))
		// Модель сохраняет запрещённую вложенную конструкцию и отдельную ошибку;
		// её внутренние пустые строки не становятся непосредственным содержимым text.
		v110TestParserError(t, result, 6, 6, 8, "line")
		v110TestRun(t, result, "current", []v110TestExpected{{4, 3, 6, 1}, {10, 2, 12, 1}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo text/"+tag, func(t *testing.T) {
			v110TestRun(t, v110TestLesson(t, v110TestOpen(t, tag, 2, 1), v110TestText(t, "@text {", "@text {", 3, 2),
				v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2), v110TestBlank(t, " \t ", 3, 2),
				v110TestText(t, "B.", "B.", 3, 2), v110TestText(t, `\}`, "}", 3, 2), v110TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("other block title text is not a tag", func(t *testing.T) {
		v110TestRun(t, v110TestLesson(t, v110TestLine(t, "@note text {", "block-start", 2, 1,
			v110TestElement(t, "tag", "@note", "note", 1, 6), v110TestElement(t, "title", "text", "text", 7, 11),
			v110TestElement(t, "block-open", "{", "{", 12, 13)), v110TestText(t, "A.", "A.", 3, 2),
			v110TestBlank(t, "", 3, 2), v110TestBlank(t, "\t", 3, 2), v110TestText(t, "B.", "B.", 3, 2),
			v110TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("outside blanks do not join inner run", func(t *testing.T) {
		v110TestRun(t, v110TestLesson(t, v110TestBlank(t, "", 2, 1), v110TestOpen(t, "text", 2, 1),
			v110TestBlank(t, "", 4, 2), v110TestText(t, "A.", "A.", 4, 2), v110TestClose(t, 4, 2),
			v110TestBlank(t, "", 2, 1)), "current", []v110TestExpected{{4, 8, 6, 1}})
	})
	t.Run("independent blocks", func(t *testing.T) {
		v110TestRun(t, v110TestLesson(t, v110TestOpen(t, "text", 2, 1), v110TestBlank(t, "", 3, 2),
			v110TestText(t, "A.", "A.", 3, 2), v110TestClose(t, 3, 2), v110TestOpen(t, "text", 2, 1),
			v110TestText(t, "B.", "B.", 7, 2), v110TestBlank(t, "", 7, 2), v110TestClose(t, 7, 2)),
			"current", []v110TestExpected{{3, 8, 5, 1}, {8, 3, 10, 1}})
	})
	t.Run("uppercase tag", func(t *testing.T) {
		result := v110TestBlock(t, v110TestBlank(t, "", 3, 2), v110TestText(t, "A.", "A.", 3, 2))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@TEXT {", "@TEXT"
		v110TestRun(t, result, "current", []v110TestExpected{{3, 8, 5, 1}})
	})
	t.Run("raw opening end and closing start", func(t *testing.T) {
		v110TestRun(t, v110TestLesson(t,
			v110TestLine(t, " \t@text { \t", "block-start", 2, 1, v110TestElement(t, "tag", "@text", "text", 3, 8),
				v110TestElement(t, "block-open", "{", "{", 9, 10)), v110TestBlank(t, "", 3, 2),
			v110TestLine(t, " \t} \t", "block-end", 3, 2, v110TestElement(t, "block-close", "}", "}", 3, 4))),
			"current", []v110TestExpected{{3, 12, 5, 1}})
	})
	t.Run("block title is not content", func(t *testing.T) {
		v110TestRun(t, v110TestLesson(t, v110TestLine(t, "@text Title {", "block-start", 2, 1,
			v110TestElement(t, "tag", "@text", "text", 1, 6), v110TestElement(t, "title", "Title", "Title", 7, 12),
			v110TestElement(t, "block-open", "{", "{", 13, 14)), v110TestBlank(t, "", 3, 2), v110TestClose(t, 3, 2)),
			"current", []v110TestExpected{{3, 14, 5, 1}})
	})
}

func TestV110EOF(t *testing.T) {
	for _, tc := range []struct {
		name                string
		rows                []model.Line
		ending              model.LineEnding
		lastLine, endColumn int
		want                []v110TestExpected
	}{
		{"spaces without EOL", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, " \t", 3, 2)}, "", 5, 3,
			[]v110TestExpected{{4, 3, 5, 3}}},
		{"empty raw with LF", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2)}, "\n", 5, 1,
			[]v110TestExpected{{4, 3, 5, 1}}},
		{"empty raw with CRLF", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2)}, "\r\n", 5, 1,
			[]v110TestExpected{{4, 3, 5, 1}}},
		{"multiple blanks through EOF", []model.Line{v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
			v110TestBlank(t, "", 3, 2)}, "\n", 6, 1, []v110TestExpected{{4, 3, 6, 1}}},
		{"only trailing tab", []model.Line{v110TestBlank(t, "\t", 3, 2)}, "", 4, 2, []v110TestExpected{{3, 8, 4, 2}}},
		{"no blank at EOF", []model.Line{v110TestText(t, "A.", "A.", 3, 2)}, "", 4, 3, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]model.Line{v110TestOpen(t, "text", 2, 1)}, tc.rows...)
			result := v110TestLesson(t, rows...)
			result.Lines[tc.lastLine-1].LineEnding = tc.ending
			// Незакрытый блок — отдельная сохранённая ошибка. На EOF используется
			// конец последнего Raw без вымышленной дополнительной строки модели.
			v110TestParserError(t, result, 3, tc.lastLine, tc.endColumn, "block")
			v110TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV110OverlapWithV004(t *testing.T) {
	result := v110TestBlock(t, v110TestText(t, "A.", "A.", 3, 2), v110TestBlank(t, "", 3, 2),
		v110TestBlank(t, "", 3, 2), v110TestText(t, "B.", "B.", 3, 2))
	v110TestRun(t, result, "current", []v110TestExpected{{4, 3, 7, 1}})
	// Пользователь явно разрешил пересечение и запретил изменять V004.
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == "current" && entry.DiagnosticCode == "V004" {
			got = append(got, entry)
		}
	}
	want := model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 6, Column: 1}}
	if len(got) != 1 || got[0].Location == nil || *got[0].Location != want || got[0].SeverityLevel != "recommendation" ||
		got[0].DiagnosticScope != "block" || got[0].Message != "Несколько пустых строк подряд." || len(got[0].RelatedLocations) != 0 {
		t.Fatalf("existing V004 changed: %+v", got)
	}
}

func TestV110VersionsAndLifecycle(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v110TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v110TestRun(t, result, "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v110TestBlock(t, v110TestText(t, "First paragraph.", "First paragraph.", 3, 2),
					v110TestBlank(t, "", 3, 2), v110TestText(t, "Second paragraph.", "Second paragraph.", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[6].LineEnding = ""
				}
				v110TestRun(t, result, "current", []v110TestExpected{{4, 17, 6, 1}})
			})
		}
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v110TestBlock(t, v110TestBlank(t, "", 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v110TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v110TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v110TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v110TestBlock(t, v110TestBlank(t, "", 3, 2))
		second := v110TestBlock(t, v110TestText(t, "A.", "A.", 3, 2))
		v110TestRun(t, first, "first", []v110TestExpected{{3, 8, 5, 1}})
		v110TestRun(t, second, "first", nil)
		v110TestRun(t, first, "second", []v110TestExpected{{3, 8, 5, 1}})
	})
}

// v110TestParserError фиксирует явно заданную ошибку исходного блока или вложения.
func v110TestParserError(t *testing.T, result *model.Result, startLine, endLine, endColumn int, scope diagnostics.Scope) {
	t.Helper()
	result.Lines[startLine-1].HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-context-error", Source: "parser", DiagnosticCode: "fixture-context-error", SeverityLevel: "error", DiagnosticScope: scope,
		Message: "Ошибка структуры блока.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: startLine, Column: 1}, End: model.Position{Line: endLine, Column: endColumn}},
	})
}

// v110TestBlock добавляет блок text вокруг явно заданных строк с родителем 3.
func v110TestBlock(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v110TestOpen(t, "text", 2, 1)}, rows...)
	body = append(body, v110TestClose(t, 3, 2))
	return v110TestLesson(t, body...)
}

func v110TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v110TestLine(t, raw, "content", parent, depth,
		v110TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v110TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v110TestLine(t, raw, "blank", parent, depth)
}

// v110TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v110TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v110TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v110TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v110TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v110TestLine(t, "# Lesson", "heading", 0, 0,
			v110TestElement(t, "heading-level", "#", "1", 1, 2), v110TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v110TestOpen задаёт реальное объявление блока с явным родителем.
func v110TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v110TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v110TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v110TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v110TestClose задаёт закрытие указанного блока.
func v110TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v110TestLine(t, "}", "block-end", parent, depth, v110TestElement(t, "block-close", "}", "}", 1, 2))
}

// v110TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v110TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v110TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v110TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v110TestRegistry использует весь штатный каталог регистраций.
func v110TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v110TestRun проверяет только новые V110 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v110TestRun(t *testing.T, result *model.Result, source string, want []v110TestExpected) {
	t.Helper()
	before := v110TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v110TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v110TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V110" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V110 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.startLine, Column: position.startColumn}, End: model.Position{Line: position.endLine, Column: position.endColumn}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Message != "Внутри @text есть пустые строки исходного текста.\nконкретизация: Редактор решает, нужен ли дополнительный визуальный интервал: удалите пустые исходные строки либо замените каждый необходимый интервал отдельной строкой <br>. Абзацы и обычные интервалы между ними отображаются автоматически" {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V110 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v110TestSnapshot(t, result)), &restored); err != nil {
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
	if v110TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v110TestSnapshot сохраняет независимое представление модели.
func v110TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

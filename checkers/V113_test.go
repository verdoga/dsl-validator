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

// v113TestExpected задаёт основной и связанные исходные диапазоны на одной строке.
type v113TestExpected struct {
	line, start, end int
	related          [][2]int
}

func TestV113Registration(t *testing.T) {
	definitions, err := v113TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V113" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Теги выделения закрыты не в порядке вложения." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V113 definitions = %d, want 1", count)
	}
}

func TestV113ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, row, fragment string
		want                []v113TestExpected
	}{
		{"reference", "<strong><em>Read this carefully.</em></strong>",
			"@text {\n<strong><em>Read this carefully.</em></strong>\n}", nil},
		{"example", "<strong><em>Read this carefully.</strong></em>",
			"@text {\n<strong><em>Read this carefully.</strong></em>\n}", []v113TestExpected{{4, 33, 42, [][2]int{{9, 13}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v113TestBlock(t, v113TestText(t, tc.row, tc.row, 3, 2))
			// Фрагменты сохранены дословно после снятия HTML-разметки таблицы задания.
			// Добавлены только обязательные версия и заголовок.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v113TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV113PairsAndOrder(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v113TestExpected
	}{
		{"ordinary text", "Read carefully.", nil},
		{"all names nested correctly", "<strong><em><u>x</u></em></strong>", nil},
		{"sequential pairs", "<u>x</u><em>y</em>", nil},
		{"same name nested correctly", "<u><u>x</u></u>", nil},
		{"two crossings at first closing", "<strong><em><u>x</strong></em></u>",
			[]v113TestExpected{{4, 17, 26, [][2]int{{9, 13}, {13, 16}}}, {4, 26, 31, [][2]int{{13, 16}}}}},
		{"last opening of same name", "<u><u><em>x</u></em></u>", []v113TestExpected{{4, 12, 16, [][2]int{{7, 11}}}}},
		{"unpaired inner opening excluded", "<strong><em>x</strong>", nil},
		{"unpaired opening among real crossing", "<strong><u><em>x</strong></em>", []v113TestExpected{{4, 17, 26, [][2]int{{12, 16}}}}},
		{"unpaired outer same name", "<strong><strong><em>x</strong></em>", []v113TestExpected{{4, 22, 31, [][2]int{{17, 21}}}}},
		{"closing before opening", "</u><u><em>x</em>", nil},
		{"only openings", "<strong><em><u>x", nil},
		{"only closings", "x</strong></em></u>", nil},
		{"unpaired closing among real crossing", "<u><em>x</strong></u></em>", []v113TestExpected{{4, 18, 22, [][2]int{{4, 8}}}}},
		{"mixed case", "<U><Em>x</u></eM>", []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}}},
		{"self closing opening excluded", "<u/><em>x</u></em>", nil},
		{"spaced self closing opening excluded", "<u /><em>x</u></em>", nil},
		{"self closing marker among pairs", "<strong><em/><u>x</strong></u>", []v113TestExpected{{4, 18, 27, [][2]int{{14, 17}}}}},
		{"self closing closing excluded", "<u><em>x</u/></em>", nil},
		{"multiple independent violations", "<u><em>x</u></em><strong><em>y</strong></em>",
			[]v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}, {4, 31, 40, [][2]int{{26, 30}}}}},
		{"remove only offending opening", "<u><em><strong>x</em></u></strong>",
			[]v113TestExpected{{4, 17, 22, [][2]int{{8, 16}}}, {4, 22, 26, [][2]int{{8, 16}}}}},
		{"Unicode columns", "Я😀 <u><em>x</u></em>", []v113TestExpected{{4, 12, 16, [][2]int{{7, 11}}}}},
		{"indentation remains in range", " \t<u><em>x</u></em> \t", []v113TestExpected{{4, 11, 15, [][2]int{{6, 10}}}}},
		{"empty highlighted text", "<u><em></u></em>", []v113TestExpected{{4, 8, 12, [][2]int{{4, 8}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v113TestRun(t, v113TestBlock(t, v113TestText(t, tc.raw, strings.Trim(tc.raw, " \t"), 3, 2)), "current", tc.want)
		})
	}
}

func TestV113MarkersAndQuotes(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v113TestExpected
	}{
		{"double quoted angle", `<u a=">"><em>x</u></em>`, []v113TestExpected{{4, 15, 19, [][2]int{{10, 14}}}}},
		{"single quoted angle", `<u a='>'><em>x</u></em>`, []v113TestExpected{{4, 15, 19, [][2]int{{10, 14}}}}},
		{"marker inside attribute", `<u a="<em>"><em>x</u></em>`, []v113TestExpected{{4, 18, 22, [][2]int{{13, 17}}}}},
		{"unknown marker consumes quoted contents", `<span a="<u>"><em>x</u></em>`, nil},
		{"quoted slash is not self closing", `<u a="/"><em>x</u></em>`, []v113TestExpected{{4, 15, 19, [][2]int{{10, 14}}}}},
		{"quoted attribute on related opening", `<u><em a=">">x</u></em>`, []v113TestExpected{{4, 15, 19, [][2]int{{4, 14}}}}},
		{"closing marker range includes attributes", `<u><em>x</u a=">"> </em>`, []v113TestExpected{{4, 9, 19, [][2]int{{4, 8}}}}},
		{"attributes on self closing marker", `<u a="/>" /><em>x</u></em>`, nil},
		{"longer name", "<underline><em>x</u></em>", nil},
		{"name suffix", "<u-extra><em>x</u></em>", nil},
		{"no name", "<><em>x</u></em>", nil},
		{"only angle", "<", nil},
		{"only closing prefix", "</", nil},
		{"unfinished name", "<strong", nil},
		{"unfinished marker", "<u a=value", nil},
		{"unfinished double quote", `<u a="> <em>x</u></em>`, nil},
		{"unfinished single quote", `<u a='> <em>x</u></em>`, nil},
		{"unfinished closing excluded", "<u><em>x</u></em", nil},
		{"quotes outside markers do not hide them", `"<u><em>x</u></em>"`, []v113TestExpected{{4, 10, 14, [][2]int{{5, 9}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Допустимость атрибутов и ошибочных форм не подменяет проверку порядка.
			v113TestRun(t, v113TestBlock(t, v113TestText(t, tc.raw, tc.raw, 3, 2)), "current", tc.want)
		})
	}
}

func TestV113Escapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v113TestExpected
	}{
		{"escaped opening", `<u>\<em>x</u></em>`, `<u><em>x</u></em>`, nil},
		{"escaped closing", `<u><em>x\</u></em>`, `<u><em>x</u></em>`, nil},
		{"double slash before opening", `\\<u><em>x</u></em>`, `\<u><em>x</u></em>`, []v113TestExpected{{4, 11, 15, [][2]int{{6, 10}}}}},
		{"triple slash before opening", `\\\<u><em>x</u></em>`, `\<u><em>x</u></em>`, nil},
		{"four slashes before opening", `\\\\<u><em>x</u></em>`, `\\<u><em>x</u></em>`, []v113TestExpected{{4, 13, 17, [][2]int{{8, 12}}}}},
		{"double slash before closing", `<u><em>x\\</u></em>`, `<u><em>x\</u></em>`, []v113TestExpected{{4, 11, 15, [][2]int{{4, 8}}}}},
		{"escaped angle in attribute", `<u a="\>"><em>x</u></em>`, `<u a=">"><em>x</u></em>`, []v113TestExpected{{4, 16, 20, [][2]int{{11, 15}}}}},
		{"backslash does not escape quote in text", `<u a="\"><em>x</u></em>`, `<u a="\"><em>x</u></em>`, []v113TestExpected{{4, 15, 19, [][2]int{{10, 14}}}}},
		{"final slash does not erase previous violation", `<u><em>x</u></em>\`, `<u><em>x</u></em>\`, []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}}},
		{"final slash in unfinished marker", `<u a=\`, `<u a=\`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v113TestRun(t, v113TestBlock(t, v113TestText(t, tc.raw, tc.value, 3, 2)), "current", tc.want)
		})
	}
}

func TestV113HTMLWhitespace(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v113TestExpected
	}{
		{"space before attribute", "<u class='x'><em>x</u></em>", []v113TestExpected{{4, 19, 23, [][2]int{{14, 18}}}}},
		{"tab before attribute", "<u\tclass='x'><em>x</u></em>", []v113TestExpected{{4, 19, 23, [][2]int{{14, 18}}}}},
		{"form feed before attribute", "<u\fclass='x'><em>x</u></em>", []v113TestExpected{{4, 19, 23, [][2]int{{14, 18}}}}},
		{"form feed on related opening", "<u><em\fclass='x'>x</u></em>", []v113TestExpected{{4, 19, 23, [][2]int{{4, 18}}}}},
		{"form feed on closing", "<u><em>x</u\f></em>", []v113TestExpected{{4, 9, 14, [][2]int{{4, 8}}}}},
		{"quoted angle after form feed", "<u\fclass='>'><em>x</u></em>", []v113TestExpected{{4, 19, 23, [][2]int{{14, 18}}}}},
		{"correct order with form feed", "<u\fclass='x'><em>x</em></u>", nil},
		{"self closing with form feed", "<u\f/><em>x</u></em>", nil},
		{"vertical tab is not HTML whitespace", "<u\vclass='x'><em>x</u></em>", nil},
		{"NBSP is not HTML whitespace", "<u\u00a0class='x'><em>x</u></em>", nil},
		{"em space is not HTML whitespace", "<u\u2003class='x'><em>x</u></em>", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// CR/LF по контракту находятся в EOL, не в Raw. Атрибуты могут
			// быть ошибочны по DSL, но имя выделения для V113 сохраняется.
			v113TestRun(t, v113TestBlock(t, v113TestText(t, tc.raw, tc.raw, 3, 2)), "current", tc.want)
		})
	}
}

func TestV113RowsAndUnparsed(t *testing.T) {
	t.Run("no pairing across rows", func(t *testing.T) {
		v113TestRun(t, v113TestBlock(t, v113TestText(t, "<u><em>x", "<u><em>x", 3, 2),
			v113TestText(t, "</u></em>", "</u></em>", 3, 2)), "current", nil)
	})
	t.Run("independent adjacent rows", func(t *testing.T) {
		v113TestRun(t, v113TestBlock(t, v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2),
			v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2)), "current",
			[]v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}, {5, 9, 13, [][2]int{{4, 8}}}})
	})
	t.Run("unfinished quote does not hide next row", func(t *testing.T) {
		v113TestRun(t, v113TestBlock(t, v113TestText(t, `<u a="`, `<u a="`, 3, 2),
			v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2)), "current",
			[]v113TestExpected{{5, 9, 13, [][2]int{{4, 8}}}})
	})
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"only inner text element", v113TestLine(t, "<u><em>x</u></em>", "content", 3, 2,
			v113TestElement(t, "content", "x", "x", 8, 9))},
		{"unparsed closing", v113TestLine(t, "<u><em>x</u></em>", "content", 3, 2,
			v113TestElement(t, "content", "<u><em>x", "<u><em>x", 1, 9), v113TestElement(t, "unparsed", "</u>", "", 9, 13),
			v113TestElement(t, "content", "</em>", "</em>", 13, 18))},
		{"whole row unparsed", v113TestLine(t, "<u><em>x</u></em>", "content", 3, 2,
			v113TestElement(t, "unparsed", "<u><em>x</u></em>", "", 1, 18))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v113TestRun(t, v113TestBlock(t, tc.row), "current", []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}})
		})
	}
}

func TestV113Contexts(t *testing.T) {
	t.Run("text inside fragment", func(t *testing.T) {
		fragment := v113TestLine(t, "@fragment f {", "block-start", 2, 1,
			v113TestElement(t, "tag", "@fragment", "fragment", 1, 10), v113TestElement(t, "identifier", "f", "f", 11, 12),
			v113TestElement(t, "block-open", "{", "{", 13, 14))
		v113TestRun(t, v113TestLesson(t, fragment, v113TestOpen(t, "text", 3, 2),
			v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 4, 3), v113TestClose(t, 4, 3), v113TestClose(t, 3, 2)),
			"current", []v113TestExpected{{5, 9, 13, [][2]int{{4, 8}}}})
	})
	for _, tag := range []string{"text", "note"} {
		t.Run("nested block/"+tag, func(t *testing.T) {
			result := v113TestLesson(t, v113TestOpen(t, "text", 2, 1), v113TestOpen(t, tag, 3, 2),
				v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 4, 3), v113TestClose(t, 4, 3), v113TestClose(t, 3, 2))
			// Запрещённое вложение сохранено моделью с отдельной ошибкой.
			// Вложенный text всё равно проверяется как самостоятельный блок.
			v113TestParserError(t, result, 4, 4, 8, "line")
			var want []v113TestExpected
			if tag == "text" {
				want = []v113TestExpected{{5, 9, 13, [][2]int{{4, 8}}}}
			}
			v113TestRun(t, result, "current", want)
		})
	}
	t.Run("title excluded", func(t *testing.T) {
		v113TestRun(t, v113TestLesson(t, v113TestLine(t, "@text <u><em>x</u></em> {", "block-start", 2, 1,
			v113TestElement(t, "tag", "@text", "text", 1, 6), v113TestElement(t, "title", "<u><em>x</u></em>", "<u><em>x</u></em>", 7, 24),
			v113TestElement(t, "block-open", "{", "{", 25, 26)), v113TestText(t, "Read.", "Read.", 3, 2), v113TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested inline declaration excluded", func(t *testing.T) {
		result := v113TestBlock(t, v113TestLine(t, "@note <u><em>x</u></em>", "tag", 3, 2,
			v113TestElement(t, "tag", "@note", "note", 1, 6), v113TestElement(t, "content", "<u><em>x</u></em>", "<u><em>x</u></em>", 7, 24)))
		v113TestParserError(t, result, 4, 4, 24, "line")
		v113TestRun(t, result, "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo text/"+tag, func(t *testing.T) {
			v113TestRun(t, v113TestLesson(t, v113TestOpen(t, tag, 2, 1), v113TestText(t, "@text {", "@text {", 3, 2),
				v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2), v113TestText(t, `\}`, "}", 3, 2),
				v113TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("other block and outer text excluded", func(t *testing.T) {
		v113TestRun(t, v113TestLesson(t, v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 2, 1),
			v113TestOpen(t, "note", 2, 1), v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 4, 2),
			v113TestClose(t, 4, 2)), "current", nil)
	})
	t.Run("uppercase source tag", func(t *testing.T) {
		result := v113TestBlock(t, v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@TEXT {", "@TEXT"
		v113TestRun(t, result, "current", []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}})
	})
}

func TestV113VersionsAndLifecycle(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v113TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v113TestRun(t, result, "current", nil)
	})
	t.Run("empty block and blank lines", func(t *testing.T) {
		v113TestRun(t, v113TestBlock(t), "current", nil)
		v113TestRun(t, v113TestBlock(t, v113TestLine(t, "", "blank", 3, 2), v113TestLine(t, " \t", "blank", 3, 2)), "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v113TestBlock(t, v113TestText(t, "<strong><em>Read this carefully.</strong></em>", "<strong><em>Read this carefully.</strong></em>", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[4].LineEnding = ""
				}
				v113TestRun(t, result, "current", []v113TestExpected{{4, 33, 42, [][2]int{{9, 13}}}})
			})
		}
	}
	t.Run("content at EOF", func(t *testing.T) {
		result := v113TestLesson(t, v113TestOpen(t, "text", 2, 1), v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2))
		result.Lines[3].LineEnding = ""
		v113TestParserError(t, result, 3, 4, 18, "block")
		v113TestRun(t, result, "current", []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}})
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v113TestBlock(t, v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v113TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v113TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v113TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v113TestBlock(t, v113TestText(t, "<u><em>x</u></em>", "<u><em>x</u></em>", 3, 2))
		second := v113TestBlock(t, v113TestText(t, "<u><em>x</em></u>", "<u><em>x</em></u>", 3, 2))
		v113TestRun(t, first, "first", []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}})
		v113TestRun(t, second, "first", nil)
		v113TestRun(t, first, "second", []v113TestExpected{{4, 9, 13, [][2]int{{4, 8}}}})
	})
}

// v113TestParserError фиксирует отдельную ошибку вложения или незакрытого блока.
func v113TestParserError(t *testing.T, result *model.Result, startLine, endLine, endColumn int, scope diagnostics.Scope) {
	t.Helper()
	result.Lines[startLine-1].HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-context-error", Source: "parser", DiagnosticCode: "fixture-context-error", SeverityLevel: "error", DiagnosticScope: scope,
		Message: "Ошибка структуры блока.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: startLine, Column: 1}, End: model.Position{Line: endLine, Column: endColumn}},
	})
}

// v113TestBlock добавляет блок text вокруг явно заданных строк с родителем 3.
func v113TestBlock(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v113TestOpen(t, "text", 2, 1)}, rows...)
	body = append(body, v113TestClose(t, 3, 2))
	return v113TestLesson(t, body...)
}

func v113TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v113TestLine(t, raw, "content", parent, depth,
		v113TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v113TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v113TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v113TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v113TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v113TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v113TestLine(t, "# Lesson", "heading", 0, 0,
			v113TestElement(t, "heading-level", "#", "1", 1, 2), v113TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v113TestOpen задаёт реальное объявление блока с явным родителем.
func v113TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v113TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v113TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v113TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v113TestClose задаёт закрытие указанного блока.
func v113TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v113TestLine(t, "}", "block-end", parent, depth, v113TestElement(t, "block-close", "}", "}", 1, 2))
}

// v113TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v113TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v113TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v113TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v113TestRegistry использует весь штатный каталог регистраций.
func v113TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v113TestRun проверяет только новые V113 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v113TestRun(t *testing.T, result *model.Result, source string, want []v113TestExpected) {
	t.Helper()
	before := v113TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v113TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v113TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V113" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V113 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(position.related) ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Теги выделения закрыты не в порядке вложения." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V113 metadata", i, entry, location)
		}
		for j, columns := range position.related {
			related := model.Location{Start: model.Position{Line: position.line, Column: columns[0]}, End: model.Position{Line: position.line, Column: columns[1]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("occurrence %d related %d = %+v, want %+v", i, j, entry.RelatedLocations[j], related)
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v113TestSnapshot(t, result)), &restored); err != nil {
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
	if v113TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v113TestSnapshot сохраняет независимое представление модели.
func v113TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

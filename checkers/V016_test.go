package checkers

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV016Registration(t *testing.T) {
	definitions, err := v016Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V016" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "В тексте присутствует запрещённый специальный символ." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V016 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V016 definitions = %d, want 1", matches)
	}
}

// v016Expected содержит независимые ожидания для одного исходного символа.
type v016Expected struct {
	line, start, end int
	detail           string
}

const (
	v016Caret     = "Обнаружен символ ^ U+005E"
	v016Ampersand = "Обнаружен символ & U+0026"
	v016Star      = "Обнаружен символ * U+002A"
	v016Backtick  = "Обнаружен символ ` U+0060"
	v016Tilde     = "Обнаружен символ ~ U+007E"
	v016Less      = "Обнаружен символ < U+003C"
	v016Greater   = "Обнаружен символ > U+003E"
)

func TestV016RawSymbols(t *testing.T) {
	// Пример сохранён буквально; добавлены только обязательные версия и
	// заголовок (DSL 1.1–1.3, 1.30, 2.1.3). Все ожидания заданы вручную.
	for _, tc := range []struct {
		name, raw string
		want      []v016Expected
	}{
		{"example", "Read ^this.", []v016Expected{{3, 6, 7, v016Caret}}},
		{"ordinary text", "Read this.", nil},
		{"seven forbidden symbols and allowed equal", "^&*`~<>=", []v016Expected{
			{3, 1, 2, v016Caret}, {3, 2, 3, v016Ampersand}, {3, 3, 4, v016Star}, {3, 4, 5, v016Backtick},
			{3, 5, 6, v016Tilde}, {3, 6, 7, v016Less}, {3, 7, 8, v016Greater},
		}},
		{"equal between letters", "a=b", nil},
		{"one symbol", "=", nil},
		{"arithmetic", "2 + 2 = 4", nil},
		{"teaching equivalence", "the USA = the United States of America", nil},
		{"equal does not hide forbidden symbol", "2 + 2 = 4 ^", []v016Expected{{3, 11, 12, v016Caret}}},
		{"adjacent repetitions", "^^", []v016Expected{{3, 1, 2, v016Caret}, {3, 2, 3, v016Caret}}},
		{"first and last", "*x~", []v016Expected{{3, 1, 2, v016Star}, {3, 3, 4, v016Tilde}}},
		{"Unicode columns", "Я🙂=^", []v016Expected{{3, 4, 5, v016Caret}}},
		{"combining mark", "е\u0301^", []v016Expected{{3, 3, 4, v016Caret}}},
		{"tabs and nonbreaking space", "\t^\u00a0=", []v016Expected{{3, 2, 3, v016Caret}}},
		{"backslash does not hide caret", `\^`, []v016Expected{{3, 2, 3, v016Caret}}},
		{"quoted symbol", `"&"`, []v016Expected{{3, 2, 3, v016Ampersand}}},
		{"ordinary HTML-looking text", "<em>x</em>", []v016Expected{
			{3, 1, 2, v016Less}, {3, 4, 5, v016Greater}, {3, 6, 7, v016Less}, {3, 10, 11, v016Greater},
		}},
		{"entities are not decoded", "&lt;", []v016Expected{{3, 1, 2, v016Ampersand}}},
		{"similar Unicode symbols", "ˆ＆∗˜｀≤≥＝", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v016Run(t, v016Document(t, v016Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV016TextMarkers(t *testing.T) {
	// Эталон таблицы — реальные <strong> и </strong>; &lt;/&gt; в её
	// HTML-представлении не являются символами DSL. Обрамление сохранено.
	for _, tc := range []struct {
		name, raw, value string
		want             []v016Expected
	}{
		{"reference", "<strong>Read</strong> this.", "<strong>Read</strong> this.", nil},
		{"all paired markers", "<strong><em><u>x</u></em></strong>", "<strong><em><u>x</u></em></strong>", nil},
		{"case insensitive", "<StRoNg><EM><U>x</u></eM></STRONG>", "<StRoNg><EM><U>x</u></eM></STRONG>", nil},
		{"marker content checked", "<em>^</em>", "<em>^</em>", []v016Expected{{4, 5, 6, v016Caret}}},
		{"equal in marker content", "<u>=</u>", "<u>=</u>", nil},
		{"standalone br", "<br>", "<br>", nil},
		{"standalone uppercase br", "<BR>", "<BR>", nil},
		{"br with edge spaces", " \t<br>\t ", "<br>", nil},
		{"inline br", "x<br>y", "x<br>y", []v016Expected{{4, 2, 3, v016Less}, {4, 5, 6, v016Greater}}},
		{"self closing br", "<br/>", "<br/>", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}}},
		{"spaced self closing br", "<br />", "<br />", []v016Expected{{4, 1, 2, v016Less}, {4, 6, 7, v016Greater}}},
		{"closing br", "</br>", "</br>", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}}},
		{"attributes", "<em x=y>", "<em x=y>", []v016Expected{{4, 1, 2, v016Less}, {4, 8, 9, v016Greater}}},
		{"unknown tag", "<foo>", "<foo>", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}}},
		{"marker with space", "<em >", "<em >", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}}},
		{"comparisons", "1 < 2 > 0", "1 < 2 > 0", []v016Expected{{4, 3, 4, v016Less}, {4, 7, 8, v016Greater}}},
		{"escaped opening", `\<em>`, "<em>", []v016Expected{{4, 2, 3, v016Less}, {4, 5, 6, v016Greater}}},
		{"escaped closing", `<em\>`, "<em>", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}}},
		{"escaped pair", `\<em\>`, "<em>", []v016Expected{{4, 2, 3, v016Less}, {4, 6, 7, v016Greater}}},
		{"paired backslash before marker", `\\<em>x</em>`, `\<em>x</em>`, nil},
		{"odd backslashes before marker", `\\\<em>`, `\<em>`, []v016Expected{{4, 4, 5, v016Less}, {4, 7, 8, v016Greater}}},
		{"no HTML pairing check", "<em>", "<em>", nil},
		{"unmatched closing marker", "</u>", "</u>", nil},
		{"incomplete marker", "<em", "<em", []v016Expected{{4, 1, 2, v016Less}}},
		{"stray closing angle", ">", ">", []v016Expected{{4, 1, 2, v016Greater}}},
		{"Unicode before marker", "Я🙂<u>x</u>=^", "Я🙂<u>x</u>=^", []v016Expected{{4, 12, 13, v016Caret}}},
		{"entity inside text", "&lt;em&gt;", "&lt;em&gt;", []v016Expected{{4, 1, 2, v016Ampersand}, {4, 7, 8, v016Ampersand}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := v016Line(t, tc.raw, "content", 3, 2,
				v016Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v016Run(t, v016Document(t, v016Open(t, "text", 2, 1), content, v016Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV016Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v016Expected
	}{
		{"empty and whitespace rows", []model.Line{v016Line(t, "", "blank", 2, 1), v016Line(t, " \t ", "blank", 2, 1)}, nil},
		{"original indentation", []model.Line{v016Line(t, "  ^  ", "content", 2, 1,
			v016Element(t, "content", "^", "^", 3, 4))}, []v016Expected{{3, 3, 4, v016Caret}}},
		{"inline comment", []model.Line{v016Line(t, "@editor <em>", "tag", 2, 1,
			v016Element(t, "tag", "@editor", "editor", 1, 8), v016Element(t, "content", "<em>", "<em>", 9, 13))},
			[]v016Expected{{3, 9, 10, v016Less}, {3, 12, 13, v016Greater}}},
		{"block comment", []model.Line{v016Open(t, "editor", 2, 1), v016Text(t, "<em>", 3, 2), v016Close(t, 3, 2)},
			[]v016Expected{{4, 1, 2, v016Less}, {4, 4, 5, v016Greater}}},
		{"table comparison", []model.Line{v016Open(t, "table", 2, 1), v016Text(t, "1 < 2", 3, 2), v016Close(t, 3, 2)},
			[]v016Expected{{4, 3, 4, v016Less}}},
		{"text title not exempt", []model.Line{
			v016Line(t, "@text <em> {", "block-start", 2, 1,
				v016Element(t, "tag", "@text", "text", 1, 6), v016Element(t, "title", "<em>", "<em>", 7, 11),
				v016Element(t, "block-open", "{", "{", 12, 13)), v016Text(t, "Body.", 3, 2), v016Close(t, 3, 2)},
			[]v016Expected{{3, 7, 8, v016Less}, {3, 10, 11, v016Greater}}},
		{"canonical text tag", []model.Line{
			v016Line(t, "@TeXt {", "block-start", 2, 1, v016Element(t, "tag", "@TeXt", "text", 1, 6),
				v016Element(t, "block-open", "{", "{", 7, 8)), v016Text(t, "<u>x</u>", 3, 2), v016Close(t, 3, 2)}, nil},
		{"nested hint", []model.Line{v016Open(t, "instr", 2, 1), v016Open(t, "hint", 3, 2),
			v016Text(t, "^", 4, 3), v016Close(t, 4, 3), v016Close(t, 3, 2)}, []v016Expected{{5, 1, 2, v016Caret}}},
		{"placeholder answer", []model.Line{v016Open(t, "multifill", 2, 1),
			v016Line(t, "_____{a=b}", "content", 3, 2, v016Element(t, "placeholder", "_____{a=b}", "a=b", 1, 11)),
			v016Close(t, 3, 2)}, nil},
		{"text context ends at block boundary", []model.Line{v016Open(t, "text", 2, 1), v016Text(t, "<em>x</em>", 3, 2),
			v016Close(t, 3, 2), v016Text(t, "<em>", 2, 1)}, []v016Expected{{6, 1, 2, v016Less}, {6, 4, 5, v016Greater}}},
		{"markers cannot span rows", []model.Line{v016Open(t, "text", 2, 1), v016Text(t, "<em", 3, 2),
			v016Text(t, ">", 3, 2), v016Close(t, 3, 2)}, []v016Expected{{4, 1, 2, v016Less}, {5, 1, 2, v016Greater}}},
		{"br requires entire physical row", []model.Line{v016Open(t, "text", 2, 1),
			v016Line(t, " x<br> ", "content", 3, 2, v016Element(t, "content", "x<br>", "x<br>", 2, 7)), v016Close(t, 3, 2)},
			[]v016Expected{{4, 3, 4, v016Less}, {4, 6, 7, v016Greater}}},
		{"br content with indentation", []model.Line{v016Open(t, "text", 2, 1),
			v016Line(t, " \t<br> ", "content", 3, 2, v016Element(t, "content", "<br>", "<br>", 3, 7)), v016Close(t, 3, 2)}, nil},
		{"Raw and Value differ", []model.Line{v016Line(t, `\{^\}`, "content", 2, 1,
			v016Element(t, "content", `\{^\}`, "{^}", 1, 6))}, []v016Expected{{3, 3, 4, v016Caret}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v016Run(t, v016Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV016SplitTableCells(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v016Expected
	}{
		{"split cell", "<a | b>", nil},
		{"several cells", "x | <a|b> | <c|d> | y", nil},
		{"empty parts", "<|>", nil},
		{"indentation", " \t<a | b>\t ", nil},
		{"content still checked", "<a^ | b>", []v016Expected{{4, 3, 4, v016Caret}}},
		{"Unicode columns", "Я🙂 | <a|b> | =^", []v016Expected{{4, 15, 16, v016Caret}}},
		{"symbol after closing is not cell syntax", "Я🙂 | <a|b> =", []v016Expected{{4, 6, 7, v016Less}, {4, 10, 11, v016Greater}}},
		{"literal comparisons", "1 < 2 > 0", []v016Expected{{4, 3, 4, v016Less}, {4, 7, 8, v016Greater}}},
		{"not at cell start", "x<a|b>", []v016Expected{{4, 2, 3, v016Less}, {4, 6, 7, v016Greater}}},
		{"not at cell end", "<a|b>x", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}}},
		{"no separator", "<ab>", []v016Expected{{4, 1, 2, v016Less}, {4, 4, 5, v016Greater}}},
		{"unclosed", "<a|b", []v016Expected{{4, 1, 2, v016Less}}},
		{"stray closing", "a|b>", []v016Expected{{4, 4, 5, v016Greater}}},
		{"nested", "<a|<b|c>>", []v016Expected{{4, 1, 2, v016Less}, {4, 4, 5, v016Less}, {4, 8, 9, v016Greater}, {4, 9, 10, v016Greater}}},
		{"escaped pipe is not separator", `<a\|b>`, []v016Expected{{4, 1, 2, v016Less}, {4, 6, 7, v016Greater}}},
		{"paired slashes before separator", `<a\\|b>`, nil},
		{"odd slashes before separator", `<a\\\|b>`, []v016Expected{{4, 1, 2, v016Less}, {4, 8, 9, v016Greater}}},
		{"escaped opening", `\<a|b>`, []v016Expected{{4, 2, 3, v016Less}, {4, 6, 7, v016Greater}}},
		{"escaped closing", `<a|b\>`, []v016Expected{{4, 1, 2, v016Less}, {4, 6, 7, v016Greater}}},
		{"paired slashes before closing", `<a|b\\>`, nil},
		{"literal angles inside", `<a\<|b\>>`, []v016Expected{{4, 4, 5, v016Less}, {4, 8, 9, v016Greater}}},
		{"continue after invalid cell", "<a|b>x | <c|d> | ^", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}, {4, 18, 19, v016Caret}}},
		{"two invalid cells", "<a|b>x | <c|d> ^", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}, {4, 10, 11, v016Less}, {4, 14, 15, v016Greater}, {4, 16, 17, v016Caret}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v016Run(t, v016Document(t, v016Open(t, "table", 2, 1), v016Text(t, tc.raw, 3, 2), v016Close(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("outside and after table", func(t *testing.T) {
		v016Run(t, v016Document(t, v016Text(t, "<a|b>", 2, 1), v016Open(t, "table", 2, 1),
			v016Text(t, "<a|b>", 4, 2), v016Close(t, 4, 2), v016Text(t, "<a|b>", 2, 1)), "current",
			[]v016Expected{{3, 1, 2, v016Less}, {3, 5, 6, v016Greater}, {7, 1, 2, v016Less}, {7, 5, 6, v016Greater}})
	})
	t.Run("title is not a cell", func(t *testing.T) {
		v016Run(t, v016Document(t, v016Line(t, "@table <a|b> {", "block-start", 2, 1,
			v016Element(t, "tag", "@table", "table", 1, 7), v016Element(t, "title", "<a|b>", "<a|b>", 8, 13),
			v016Element(t, "block-open", "{", "{", 14, 15)), v016Close(t, 3, 2)), "current",
			[]v016Expected{{3, 8, 9, v016Less}, {3, 12, 13, v016Greater}})
	})
	t.Run("structural markers outside content elements", func(t *testing.T) {
		v016Run(t, v016Document(t, v016Open(t, "table", 2, 1),
			v016Line(t, "<a|b>", "content", 3, 2, v016Element(t, "content", "a", "a", 2, 3),
				v016Element(t, "content", "b", "b", 4, 5)), v016Close(t, 3, 2)), "current", nil)
	})
	t.Run("unparsed cell stays checked beside valid cell", func(t *testing.T) {
		row := v016Line(t, "<a|b> | <c|d>", "content", 3, 2,
			model.Element{ElementType: "unparsed", Raw: "<a|b>", Start: 1, End: 6, ErrorIDs: []string{"parser-error"}},
			v016Element(t, "content", "<c|d>", "<c|d>", 9, 14))
		row.HasErrors = true
		result := v016Document(t, v016Open(t, "table", 2, 1), row, v016Close(t, 3, 2))
		result.Document.HasErrors = true
		result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
			SeverityLevel: "error", DiagnosticScope: "element", Message: "Неразобранный фрагмент.",
			Location:         &model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 6}},
			RelatedLocations: []model.Location{},
		}}
		v016Run(t, result, "current", []v016Expected{{4, 1, 2, v016Less}, {4, 5, 6, v016Greater}})
	})
}

func TestV016ServiceFields(t *testing.T) {
	t.Run("resource path", func(t *testing.T) {
		base := v016Document(t)
		path := v016Line(t, "@resource-dir assets/^", "tag", 0, 0,
			v016Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v016Element(t, "resource-path", "assets/^", "assets/^", 15, 23))
		result := v016Model(t, base.Lines[0], path, base.Lines[1])
		result.Document.Metadata.ResourceDirs = []string{"assets/^"}
		v016Run(t, result, "current", []v016Expected{{2, 22, 23, v016Caret}})
	})
	for _, tc := range []struct {
		path string
		want []v016Expected
	}{{"a=b.png", nil}, {"a^b.png", []v016Expected{{4, 16, 17, v016Caret}}}} {
		t.Run("media source "+tc.path, func(t *testing.T) {
			base := v016Document(t)
			path := v016Line(t, "@resource-dir .", "tag", 0, 0,
				v016Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v016Element(t, "resource-path", ".", ".", 15, 16))
			quoted := `"` + tc.path + `"`
			media := v016Line(t, "@media image "+quoted, "tag", 3, 1,
				v016Element(t, "tag", "@media", "media", 1, 7), v016Element(t, "media-type", "image", "image", 8, 13),
				v016Element(t, "source", quoted, tc.path, 14, 23))
			result := v016Model(t, base.Lines[0], path, base.Lines[1], media)
			result.Document.Metadata.ResourceDirs = []string{"."}
			v016Run(t, result, "current", tc.want)
		})
	}
	t.Run("heading title", func(t *testing.T) {
		base := v016Document(t)
		heading := v016Line(t, "# Unit ^", "heading", 0, 0,
			v016Element(t, "heading-level", "#", "1", 1, 2), v016Element(t, "title", "Unit ^", "Unit ^", 3, 9))
		result := v016Model(t, base.Lines[0], heading)
		title := "Unit ^"
		result.Document.Metadata.Title = &title
		v016Run(t, result, "current", []v016Expected{{2, 8, 9, v016Caret}})
	})
	for _, tc := range []struct {
		name string
		want []v016Expected
	}{{"A=B", nil}, {"A^B", []v016Expected{{2, 11, 12, v016Caret}}}} {
		t.Run("section name "+tc.name, func(t *testing.T) {
			base := v016Document(t)
			section := v016Line(t, "@section "+tc.name, "tag", 0, 0,
				v016Element(t, "tag", "@section", "section", 1, 9), v016Element(t, "name", tc.name, tc.name, 10, 13))
			result := v016Model(t, base.Lines[0], section, base.Lines[1])
			result.Document.Metadata.Section = &tc.name
			v016Run(t, result, "current", tc.want)
		})
	}
}

func TestV016Unparsed(t *testing.T) {
	// Неизвестный тег (DSL 1.16, 1.29) сохранён вместе с исходной ошибкой.
	// Даже под text элемент unparsed не получает исключение для HTML.
	for _, tc := range []struct {
		name, raw  string
		insideText bool
		end        int
		want       []v016Expected
	}{
		{"ordinary invalid row", "@unknown ^", false, 11, []v016Expected{{3, 10, 11, v016Caret}}},
		{"invalid row inside text", "@unknown <em>", true, 14, []v016Expected{{4, 10, 11, v016Less}, {4, 13, 14, v016Greater}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, depth, number := 2, 1, 3
			var body []model.Line
			if tc.insideText {
				parent, depth, number = 3, 2, 4
				body = append(body, v016Open(t, "text", 2, 1))
			}
			line := v016Line(t, tc.raw, "invalid", parent, depth, model.Element{
				ElementType: "unparsed", Raw: tc.raw, Start: 1, End: tc.end, ErrorIDs: []string{"parser-error"},
			})
			line.HasErrors = true
			body = append(body, line)
			if tc.insideText {
				body = append(body, v016Close(t, 3, 2))
			}
			result := v016Document(t, body...)
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный DSL-тег.",
				Location:         &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: tc.end}},
				RelatedLocations: []model.Location{},
			}}
			v016Run(t, result, "current", tc.want)
		})
	}
}

func TestV016LineEndingsAndHistory(t *testing.T) {
	for _, ending := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
		{"mixed", "\r\n", "\n"},
	} {
		t.Run(ending.name, func(t *testing.T) {
			result := v016Document(t, v016Text(t, "^", 2, 1), v016Text(t, "=~", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = ending.eol
			}
			result.Lines[3].LineEnding = ending.last
			want := []v016Expected{{3, 1, 2, v016Caret}, {4, 2, 3, v016Tilde}}
			v016Run(t, result, "first", want)
			v016Run(t, result, "second", want)
			v016Run(t, v016Document(t, v016Text(t, "Clean.", 2, 1)), "current", nil)
		})
	}
}

func TestV016NoLinesAndVersions(t *testing.T) {
	result := v016Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v016Run(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v016Document(t, v016Text(t, "^", 2, 1))
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v016Snapshot(t, result)
			checks, err := validator.Prepare(result, v016Registry(t))
			if err == nil || checks != nil || v016Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV016CoexistsWithOtherChecks(t *testing.T) {
	result := v016Document(t, v016Text(t, "^—", 2, 1))
	v016Run(t, result, "current", []v016Expected{{3, 1, 2, v016Caret}})
	count := 0
	for _, entry := range result.Diagnostics {
		if entry.Source == "current" && entry.DiagnosticCode == "V011" {
			count++
			want := model.Location{Start: model.Position{Line: 3, Column: 2}, End: model.Position{Line: 3, Column: 3}}
			if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "recommendation" ||
				entry.Message != "Использовано длинное тире.\nконкретизация: Для этой позиции рекомендуется короткое тире – U+2013" {
				t.Errorf("unexpected V011: %+v", entry)
			}
		}
	}
	if count != 1 {
		t.Fatalf("V011 count = %d, want 1", count)
	}
}

// v016Registry создаёт реестр только через штатный каталог регистраций.
func v016Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v016Document добавляет обязательные версию и заголовок к явно заданному телу.
func v016Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v016Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v016Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v016Element(t, "version", "1.2", "1.2", 14, 17)),
		v016Line(t, "# Unit 1", "heading", 0, 0,
			v016Element(t, "heading-level", "#", "1", 1, 2), v016Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v016Model(t, append(lines, body...)...)
}

// v016Model задаёт физические номера, не выводя типы и структуру из Raw.
func v016Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, title := "1.2", "UTF-8", "Unit 1"
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	return result
}

// v016Line сохраняет явно заданную структуру строки без разбора Raw.
func v016Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v016Element задаёт исходное и семантическое значения и диапазон [start,end).
func v016Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v016Text задаёт текст с одинаковыми Raw и Value; длина нужна только для
// входного элемента и не используется для вычисления ожидаемых срабатываний.
func v016Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v016Line(t, raw, "content", parent, depth,
		v016Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v016Open задаёт блочное объявление без параметров с согласованными элементами.
func v016Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v016Line(t, "@"+tag+" {", "block-start", parent, depth,
		v016Element(t, "tag", "@"+tag, tag, 1, end), v016Element(t, "block-open", "{", "{", end+1, end+2))
}

// v016Close задаёт закрытие с явно указанными родителем и глубиной.
func v016Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v016Line(t, "}", "block-end", parent, depth, v016Element(t, "block-close", "}", "}", 1, 2))
}

// v016Run проверяет V016 текущего запуска штатного конвейера: точное число,
// метаданные и диапазоны [start,end), неизменность исходной модели и истории.
func v016Run(t *testing.T, result *model.Result, processingID string, want []v016Expected) {
	t.Helper()
	before := v016Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v016Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v016Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V016" && entry.Source != processingID {
			t.Fatalf("new V016 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V016" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V016 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "В тексте присутствует запрещённый специальный символ.\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, location)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing or diagnostic history length")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	v016OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v016Snapshot(t, &previous) != v016Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v016Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v016Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v016OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v016OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V016" {
			continue
		}
		if entry.SeverityLevel == "error" {
			expected.Document.HasErrors = true
			if entry.Location != nil {
				expected.Lines[entry.Location.Start.Line-1].HasErrors = true
			}
		}
		if entry.DiagnosticScope != "element" {
			continue
		}
		if entry.Location == nil {
			t.Fatal("element diagnostic has no location")
		}
		matched := false
		for i := range expected.Lines[entry.Location.Start.Line-1].Elements {
			element := &expected.Lines[entry.Location.Start.Line-1].Elements[i]
			if element.Start == entry.Location.Start.Column && element.End == entry.Location.End.Column {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched = true
				break
			}
		}
		if !matched {
			t.Fatal("element diagnostic does not identify an element")
		}
	}
	for i := range expected.Lines {
		for j := range expected.Lines[i].Elements {
			slices.SortStableFunc(expected.Lines[i].Elements[j].ErrorIDs, func(a, b string) int { return order[a] - order[b] })
		}
	}
}

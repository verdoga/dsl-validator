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

func TestV115Registration(t *testing.T) {
	definitions, err := v115TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V115" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Использован неподдерживаемый HTML-тег." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V115 definitions = %d, want 1", count)
	}
}

func TestV115ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"reference", "<strong>Read this carefully.</strong>", nil},
		{"example", "<b>Read this carefully.</b>", [][3]int{{4, 1, 4}, {4, 24, 28}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Добавлены только версия и заголовок. Фрагмент таблицы неизменён.
			result := v115TestDocument(t, v115TestOpen(t, "text", 2, 1),
				v115TestText(t, tc.raw, tc.raw, 3, 2), v115TestClose(t, 3, 2))
			v115TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@text {\n"+tc.raw+"\n}")
			v115TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV115Markers(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"all allowed", "<strong><em><u>x</u></em></strong>", "<strong><em><u>x</u></em></strong>", nil},
		{"mixed case", "<StRoNg><EM><U>x</u></eM></STRONG>", "<StRoNg><EM><U>x</u></eM></STRONG>", nil},
		{"br", "<br>", "<br>", nil},
		{"br uppercase", "<BR>", "<BR>", nil},
		{"br shape belongs to another check", "</br><br/><br />", "</br><br/><br />", nil},
		{"inline br belongs to another check", "x<br>y", "x<br>y", nil},
		{"allowed attributes consume inner markers", `<strong a='<b>'>`, `<strong a='<b>'>`, nil},
		{"unpaired allowed", "<em>", "<em>", nil},
		{"unpaired unknown closing", "</B>", "</B>", [][3]int{{4, 1, 5}}},
		{"full names", "<i/><x-y></x:y>", "<i/><x-y></x:y>", [][3]int{{4, 1, 5}, {4, 5, 10}, {4, 10, 16}}},
		{"digit in name", "<h1>", "<h1>", [][3]int{{4, 1, 5}}},
		{"allowed prefix is not whole name", "<stronger><em-x><u:x><br1>", "<stronger><em-x><u:x><br1>", [][3]int{{4, 1, 11}, {4, 11, 17}, {4, 17, 22}, {4, 22, 27}}},
		{"comparison", "a < b", "a < b", nil},
		{"digit cannot begin name", "<3>", "<3>", nil},
		{"space after slash", "</ b>", "</ b>", nil},
		{"punctuation is not boundary", "<b!>", "<b!>", nil},
		{"underscore is not name continuation", "<b_x>", "<b_x>", nil},
		{"NBSP is not boundary", "<b\u00a0>", "<b\u00a0>", nil},
		{"tab boundary", "<b\t>", "<b\t>", [][3]int{{4, 1, 5}}},
		{"space boundary", "<b >", "<b >", [][3]int{{4, 1, 5}}},
		{"slash boundary", "<b/>", "<b/>", [][3]int{{4, 1, 5}}},
		{"no boundary", "<b", "<b", nil},
		{"no end", "<b ", "<b", nil},
		{"trailing slash", "</", "</", nil},
		{"lone angle", "<", "<", nil},
		{"escaped angles", `\<b\>`, "<b>", nil},
		{"escaped opener", `\<b>`, "<b>", nil},
		{"paired slash before opener", `\\<b>`, `\<b>`, [][3]int{{4, 3, 6}}},
		{"odd slashes before opener", `\\\<b>`, `\<b>`, nil},
		{"escaped end followed by active end", `<b \>>`, "<b >>", [][3]int{{4, 1, 7}}},
		{"escaped end only", `<b \>`, "<b >", nil},
		{"paired slash before end", `<b \\>`, `<b \>`, [][3]int{{4, 1, 7}}},
		{"nontechnical escape preserves later opener", `\x<b>`, `\x<b>`, [][3]int{{4, 3, 6}}},
		{"single quotes", `<b a='>'>`, `<b a='>'>`, [][3]int{{4, 1, 10}}},
		{"double quotes", `<b a=">">`, `<b a=">">`, [][3]int{{4, 1, 10}}},
		{"slash does not escape single quote", `<b a='\'>`, `<b a='\'>`, [][3]int{{4, 1, 10}}},
		{"slash does not escape double quote", `<b a="\">`, `<b a="\">`, [][3]int{{4, 1, 10}}},
		{"different quote does not close", `<b a='">'>`, `<b a='">'>`, [][3]int{{4, 1, 11}}},
		{"inner marker consumed", `<b a='<i>'>`, `<b a='<i>'>`, [][3]int{{4, 1, 12}}},
		{"unterminated quote", `<b a='>`, `<b a='>`, nil},
		{"unclosed single quote before real marker", `<b a='<i>`, `<b a='<i>`, [][3]int{{4, 7, 10}}},
		{"unclosed double quote before real marker", `<b a="<i>`, `<b a="<i>`, [][3]int{{4, 7, 10}}},
		{"escaped marker after unclosed quote", `<b a='\<i>`, `<b a='<i>`, nil},
		{"paired slash after unclosed quote", `<b a='\\<i>`, `<b a='\<i>`, [][3]int{{4, 9, 12}}},
		{"allowed and unknown after unclosed quote", `<b a='<em><i></em>`, `<b a='<em><i></em>`, [][3]int{{4, 11, 14}}},
		{"comparison before real marker", "a < b <i>", "a < b <i>", [][3]int{{4, 7, 10}}},
		{"incomplete candidate before complete marker", "<b!><i>", "<b!><i>", [][3]int{{4, 5, 8}}},
		{"escaped pseudo marker before real marker", `\<b\><i>`, "<b><i>", [][3]int{{4, 6, 9}}},
		{"edge whitespace", " <b> ", "<b>", [][3]int{{4, 2, 5}}},
		{"Unicode columns", "Я😀<b>", "Я😀<b>", [][3]int{{4, 3, 6}}},
		{"entities are not decoded", "&lt;b&gt;", "&lt;b&gt;", nil},
		{"escaped DSL start stays text", `\@x <b>`, "@x <b>", [][3]int{{4, 5, 8}}},
		{"escaped heading start stays text", `\#x <b>`, "#x <b>", [][3]int{{4, 5, 8}}},
		{"ordinary at sign stays text", "x @x <b>", "x @x <b>", [][3]int{{4, 6, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v115TestDocument(t, v115TestOpen(t, "text", 2, 1),
				v115TestText(t, tc.raw, tc.value, 3, 2), v115TestClose(t, 3, 2))
			v115TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV115ManyIncompleteMarkers(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"no closing angle", strings.Repeat("<b ", 1000), nil},
		{"first candidate consumes complete marker", strings.Repeat("<b ", 1000) + "<i>", [][3]int{{4, 1, 3004}}},
		{"unclosed quote preserves later marker", "<b a='" + strings.Repeat("x ", 1000) + "<i>", [][3]int{{4, 2007, 2010}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v115TestDocument(t, v115TestOpen(t, "text", 2, 1),
				v115TestText(t, tc.raw, strings.TrimRight(tc.raw, " "), 3, 2), v115TestClose(t, 3, 2))
			v115TestRun(t, result, "current", tc.want)
		})
	}
}

// BenchmarkV115IncompleteMarkers измеряет производственный проход по строке;
// функциональные результаты отдельно проверяются через полный каталог выше.
func BenchmarkV115IncompleteMarkers(b *testing.B) {
	for _, count := range []int{1000, 2000, 4000, 8000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			raw := []rune(strings.Repeat("<a ", count))
			b.ReportAllocs()
			b.SetBytes(int64(len(raw)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if markers := v115Unsupported(raw); len(markers) != 0 {
					b.Fatalf("incomplete markers produced %d findings", len(markers))
				}
			}
		})
	}
}

func TestV115Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"whole invalid text", v115TestInvalid(t, "<b>", 3, 2), [][3]int{{4, 1, 4}}},
		{"whole unparsed content", v115TestLine(t, "<b>", "content", 3, 2,
			v115TestUnparsed(t, "<b>", 1)), [][3]int{{4, 1, 4}}},
		{"partial parsing", v115TestLine(t, "Read <b>now</b>", "content", 3, 2,
			v115TestElement(t, "content", "Read ", "Read ", 1),
			v115TestUnparsed(t, "<b>", 6), v115TestElement(t, "content", "now", "now", 9),
			v115TestUnparsed(t, "</b>", 12)), [][3]int{{4, 6, 9}, {4, 12, 16}}},
		{"marker crosses element boundary", v115TestLine(t, "<b a='>'>", "content", 3, 2,
			v115TestUnparsed(t, "<b a=", 1), v115TestElement(t, "content", "'>'", "'>'", 6),
			v115TestUnparsed(t, ">", 9)), [][3]int{{4, 1, 10}}},
		{"unparsed DSL declaration excluded", v115TestInvalid(t, " \t@unknown <b>", 3, 2), nil},
		{"unparsed heading excluded", v115TestInvalid(t, "#bad <b>", 3, 2), nil},
		{"invalid recognized nested tag", v115TestLine(t, "@note <b> {", "invalid", 3, 2,
			v115TestElement(t, "tag", "@note", "note", 1), v115TestUnparsed(t, "<b> {", 7)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Неизвестные HTML-теги и ошибочные DSL-объявления дают основание
			// для unparsed; помощник добавляет связанные ошибки парсера.
			result := v115TestDocument(t, v115TestOpen(t, "text", 2, 1), tc.line, v115TestClose(t, 3, 2))
			v115TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV115Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"block title", []model.Line{
			v115TestLine(t, "@text <b> {", "block-start", 2, 1,
				v115TestElement(t, "tag", "@text", "text", 1),
				v115TestElement(t, "title", "<b>", "<b>", 7), v115TestElement(t, "block-open", "{", "{", 11)),
			v115TestText(t, "Read.", "Read.", 3, 2), v115TestClose(t, 3, 2)}, nil},
		{"outside text", []model.Line{v115TestText(t, "<b>", "<b>", 2, 1)}, nil},
		{"another block", []model.Line{v115TestOpen(t, "note", 2, 1),
			v115TestText(t, "<b>", "<b>", 3, 2), v115TestClose(t, 3, 2)}, nil},
		{"nested text in fragment", []model.Line{
			v115TestLine(t, "@fragment part {", "block-start", 2, 1,
				v115TestElement(t, "tag", "@fragment", "fragment", 1),
				v115TestElement(t, "identifier", "part", "part", 11),
				v115TestElement(t, "block-open", "{", "{", 16)),
			v115TestOpen(t, "text", 3, 2), v115TestText(t, "<b>", "<b>", 4, 3),
			v115TestClose(t, 4, 3), v115TestClose(t, 3, 2)}, [][3]int{{5, 1, 4}}},
		{"nested editor excluded", []model.Line{v115TestOpen(t, "text", 2, 1),
			v115TestOpen(t, "editor", 3, 2), v115TestText(t, "<b>", "<b>", 4, 3),
			v115TestClose(t, 4, 3), v115TestText(t, "<i>", "<i>", 3, 2), v115TestClose(t, 3, 2)},
			[][3]int{{7, 1, 4}}},
		{"nested single line excluded", []model.Line{v115TestOpen(t, "text", 2, 1),
			v115TestNamed(t, "note", "content", "<b>", 3, 2),
			v115TestText(t, "Read.", "Read.", 3, 2), v115TestClose(t, 3, 2)}, nil},
		{"blank content", []model.Line{v115TestOpen(t, "text", 2, 1),
			v115TestLine(t, " \t", "blank", 3, 2), v115TestClose(t, 3, 2)}, nil},
		{"empty block", []model.Line{v115TestOpen(t, "text", 2, 1), v115TestClose(t, 3, 2)}, nil},
		{"physical lines independent", []model.Line{v115TestOpen(t, "text", 2, 1),
			v115TestText(t, "<b ", "<b", 3, 2), v115TestText(t, ">", ">", 3, 2),
			v115TestText(t, "<i>", "<i>", 3, 2), v115TestClose(t, 3, 2)}, [][3]int{{6, 1, 4}}},
		{"quote does not cross line", []model.Line{v115TestOpen(t, "text", 2, 1),
			v115TestText(t, "<b a='", "<b a='", 3, 2), v115TestText(t, "<i>", "<i>", 3, 2),
			v115TestClose(t, 3, 2)}, [][3]int{{5, 1, 4}}},
		{"independent blocks", []model.Line{
			v115TestOpen(t, "text", 2, 1), v115TestText(t, "<b>", "<b>", 3, 2), v115TestClose(t, 3, 2),
			v115TestOpen(t, "text", 2, 1), v115TestText(t, "<i>", "<i>", 6, 2), v115TestClose(t, 6, 2)},
			[][3]int{{4, 1, 4}, {7, 1, 4}}},
		{"EOF inside text", []model.Line{v115TestOpen(t, "text", 2, 1),
			v115TestText(t, "<b>", "<b>", 3, 2)}, [][3]int{{4, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v115TestRun(t, v115TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV115VersionEndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"no final EOL", "\n", ""}, {"CRLF no final EOL", "\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v115TestDocument(t, v115TestOpen(t, "text", 2, 1),
				v115TestText(t, "<b>x</b>", "<b>x</b>", 3, 2), v115TestClose(t, 3, 2))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			want := [][3]int{{4, 1, 4}, {4, 5, 9}}
			v115TestRun(t, result, "first", want)
			v115TestRun(t, result, "second", want)
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v115TestModel(t)
			result.Document.Metadata.Title = nil
			if !known {
				result.Document.LineCount = nil
			}
			v115TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v115TestDocument(t, v115TestOpen(t, "text", 2, 1),
				v115TestText(t, "<b>", "<b>", 3, 2), v115TestClose(t, 3, 2))
			result.Document.DSLVersion = &version
			before := v115TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v115TestRegistry(t))
			if err == nil || checks != nil || v115TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v115TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v115TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{
		v115TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v115TestHeading(t, "#", "Lesson"),
	}, body...)
	return v115TestModel(t, lines...)
}

// v115TestModel принимает готовые строки без разбора исходного DSL.
func v115TestModel(t *testing.T, lines ...model.Line) *model.Result {
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
				v115TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v115TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v115TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v115TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v115TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v115TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v115TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v115TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v115TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v115TestLine(t, raw, "invalid", parent, depth, v115TestUnparsed(t, raw, 1))
}

func v115TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v115TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v115TestElement(t, "tag", "@"+tag, tag, 1), v115TestElement(t, kind, value, value, len(tag)+3))
}

func v115TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v115TestLine(t, marker+" "+title, "heading", 0, 0,
		v115TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v115TestElement(t, "title", title, title, len(marker)+2))
}

func v115TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v115TestLine(t, raw, "content", parent, depth, v115TestElement(t, "content", raw, value, 1))
}

func v115TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v115TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v115TestElement(t, "tag", "@"+tag, tag, 1), v115TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v115TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v115TestLine(t, "}", "block-end", parent, depth, v115TestElement(t, "block-close", "}", "}", 1))
}

func v115TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v115TestRun выполняет полный каталог и сравнивает только новые V115.
// Ожидания задают основной диапазон с исключённым концом; связанных нет.
func v115TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v115TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v115TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v115TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V115" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V115 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want location %+v and no related locations", i, entry, location)
		}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Использован неподдерживаемый HTML-тег." {
			t.Fatalf("unexpected V115 metadata: %+v", entry)
		}
	}
	// CheckModel проверяет уникальность ID, ссылки и флаги всех диагностик.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v115TestSnapshot(t, result)), &restored); err != nil {
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
	if v115TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v115TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v115TestSource(t *testing.T, result *model.Result, want string) {
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

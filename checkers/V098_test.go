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

type v098TestExpected struct{ line, start, end int }

func TestV098Registration(t *testing.T) {
	definitions, err := v098TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V098" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Разделённая ячейка не закрыта до конца строки." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V098 definitions = %d, want 1", count)
	}
}

func TestV098ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, row, fragment string
		want                []v098TestExpected
	}{
		{"reference", "A | <x | y>", "@table {\nA | <x | y>\n}", nil},
		{"example", "A | <x | y", "@table {\nA | <x | y\n}", []v098TestExpected{{4, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v098TestTable(t, v098TestText(t, tc.row, tc.row, 3, 2))
			// Сохранён дословный фрагмент; HTML-разметка таблицы задания
			// означает символы |, <, >. Добавлен только обязательный контекст.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v098TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV098CellBoundariesAndStack(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v098TestExpected
	}{
		{"first cell", "<x | y", []v098TestExpected{{4, 1, 2}}},
		{"last cell", "<a | b> | <c | d", []v098TestExpected{{4, 11, 12}}},
		{"empty outer cells", "||<x | y", []v098TestExpected{{4, 3, 4}}},
		{"only empty outer cells", "|||", nil},
		{"two closed cells", "<a | b> | <c | d>", nil},
		{"comparison in ordinary cell", "A <x | y", nil},
		{"joined ordinary text", "x<a | b", nil},
		{"extra opening after closing", "<a | b> <c", nil},
		{"comparison after closed cell", "<a | b> | c <d", nil},
		{"two unclosed openings", "<a | <b | c", []v098TestExpected{{4, 1, 2}, {4, 6, 7}}},
		{"inner opening closed", "<a | <b | c>", []v098TestExpected{{4, 1, 2}}},
		{"all nested openings closed", "<a | <b | c>>", nil},
		{"nested opening in middle of inner text", "<a <b | c", []v098TestExpected{{4, 1, 2}, {4, 4, 5}}},
		{"three unclosed openings", "<a | <b | <c | d", []v098TestExpected{{4, 1, 2}, {4, 6, 7}, {4, 11, 12}}},
		{"one of three closed", "<a | <b | <c | d>", []v098TestExpected{{4, 1, 2}, {4, 6, 7}}},
		{"only outer opening remains", "<a | <b | <c | d>>", []v098TestExpected{{4, 1, 2}}},
		{"stray closing then new cell", "> | <a | b", []v098TestExpected{{4, 5, 6}}},
		{"stray closing in same cell", "> <a | b", nil},
		{"angle at end of row", "A | <", []v098TestExpected{{4, 5, 6}}},
		{"uppercase text", "A | <X | Y", []v098TestExpected{{4, 5, 6}}},
		{"internal pipe does not reset opening", "<a | b | c", []v098TestExpected{{4, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Запрещённая вложенность и другие ошибки формы сами по себе
			// не входят в V098; важны только оставшиеся в стеке открытия.
			v098TestRun(t, v098TestTable(t, v098TestText(t, tc.raw, tc.raw, 3, 2)), "current", tc.want)
		})
	}
}

func TestV098Escapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v098TestExpected
	}{
		{"escaped opening", `\<x | y`, `<x | y`, nil},
		{"escaped first sign occupies position", `\><a | b`, `><a | b`, nil},
		{"escaped pipe occupies position", `\|<a | b`, `|<a | b`, nil},
		{"escaped outer separator", `A \|<x`, `A |<x`, nil},
		{"paired slashes before outer separator", `A\\|<x`, `A\|<x`, []v098TestExpected{{4, 5, 6}}},
		{"three slashes before outer separator", `A\\\|<x`, `A\|<x`, nil},
		{"escaped closing", `<x | y\>`, `<x | y>`, []v098TestExpected{{4, 1, 2}}},
		{"paired slashes before closing", `<x | y\\>`, `<x | y\>`, nil},
		{"three slashes before closing", `<x | y\\\>`, `<x | y\>`, []v098TestExpected{{4, 1, 2}}},
		{"four slashes before closing", `<x | y\\\\>`, `<x | y\\>`, nil},
		{"literal angles inside", `<x \< y | z \> q>`, `<x < y | z > q>`, nil},
		{"escaped nested opening", `<a | \<b | c`, `<a | <b | c`, []v098TestExpected{{4, 1, 2}}},
		{"paired slashes before nested opening", `<a | \\<b | c`, `<a | \<b | c`, []v098TestExpected{{4, 1, 2}, {4, 8, 9}}},
		{"backslash pair occupies first position", `\\<x | y`, `\<x | y`, nil},
		{"ordinary escape is not consumed", `\q<x | y`, `\q<x | y`, nil},
		{"ordinary escape inside", `<a \q<b | c`, `<a \q<b | c`, []v098TestExpected{{4, 1, 2}, {4, 6, 7}}},
		{"final backslash inside", `<a | b\`, `<a | b\`, []v098TestExpected{{4, 1, 2}}},
		{"final backslash alone", `\`, `\`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v098TestRun(t, v098TestTable(t, v098TestText(t, tc.raw, tc.value, 3, 2)), "current", tc.want)
		})
	}
}

func TestV098WhitespaceAndCoordinates(t *testing.T) {
	// White_Space задан явно, независимо от unicode.White_Space в реализации.
	// CR/LF находятся в EOL, поэтому проверяются отдельными ситуациями.
	for _, space := range []string{" ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007",
		"\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run(strconv.Quote(space), func(t *testing.T) {
			raw := space + "<x | y"
			v098TestRun(t, v098TestTable(t, v098TestText(t, raw, strings.Trim(raw, " \t"), 3, 2)), "current", []v098TestExpected{{4, 2, 3}})
		})
	}
	for _, tc := range []struct {
		name, raw, value string
		want             []v098TestExpected
	}{
		{"space and tab", " \t<x | y", "<x | y", []v098TestExpected{{4, 3, 4}}},
		{"Unicode column", "Я😀 | <x | y", "Я😀 | <x | y", []v098TestExpected{{4, 6, 7}}},
		{"Unicode space after separator", "A |\u00a0<x | y", "A |\u00a0<x | y", []v098TestExpected{{4, 5, 6}}},
		{"zero width space is text", "\u200b<x | y", "\u200b<x | y", nil},
		{"BOM inside row is text", "\ufeff<x | y", "\ufeff<x | y", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v098TestRun(t, v098TestTable(t, v098TestText(t, tc.raw, tc.value, 3, 2)), "current", tc.want)
		})
	}
}

func TestV098PhysicalRowsAndHeaders(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v098TestExpected
	}{
		{"next row cannot close opening", []model.Line{
			v098TestText(t, "<a | b", "<a | b", 3, 2), v098TestText(t, "c>", "c>", 3, 2),
		}, []v098TestExpected{{4, 1, 2}}},
		{"two independent rows", []model.Line{
			v098TestText(t, "<a | b", "<a | b", 3, 2), v098TestText(t, "<a | b", "<a | b", 3, 2),
		}, []v098TestExpected{{4, 1, 2}, {5, 1, 2}}},
		{"header and body", []model.Line{
			v098TestText(t, "<h | q", "<h | q", 3, 2), v098TestLine(t, "---", "separator", 3, 2),
			v098TestText(t, "<a | b", "<a | b", 3, 2),
		}, []v098TestExpected{{4, 1, 2}, {6, 1, 2}}},
		{"blank rows and separator", []model.Line{
			v098TestText(t, "A | B", "A | B", 3, 2), v098TestLine(t, "", "blank", 3, 2),
			v098TestLine(t, " \t---\t", "separator", 3, 2), v098TestLine(t, " \t", "blank", 3, 2),
			v098TestText(t, "C | D", "C | D", 3, 2),
		}, nil},
		{"escaped separator is ordinary cell text", []model.Line{
			v098TestText(t, `\--- | <x | y`, `--- | <x | y`, 3, 2),
		}, []v098TestExpected{{4, 8, 9}}},
		{"ordinary row", []model.Line{
			v098TestText(t, "A | B", "A | B", 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v098TestRun(t, v098TestTable(t, tc.rows...), "current", tc.want)
		})
	}
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v098TestTable(t, v098TestText(t, "A | <x | y", "A | <x | y", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[4].LineEnding = ""
				}
				v098TestRun(t, result, "current", []v098TestExpected{{4, 5, 6}})
			})
		}
	}
}

func TestV098ContextsAndRepresentations(t *testing.T) {
	t.Run("inside fragment", func(t *testing.T) {
		v098TestRun(t, v098TestLesson(t, v098TestFragment(t, 2, 1), v098TestOpen(t, "table", 3, 2),
			v098TestText(t, "<a | b", "<a | b", 4, 3), v098TestClose(t, 4, 3), v098TestClose(t, 3, 2)),
			"current", []v098TestExpected{{5, 1, 2}})
	})
	t.Run("table title is not a row", func(t *testing.T) {
		v098TestRun(t, v098TestLesson(t,
			v098TestLine(t, "@table < {", "block-start", 2, 1, v098TestElement(t, "tag", "@table", "table", 1, 7),
				v098TestElement(t, "title", "<", "<", 8, 9), v098TestElement(t, "block-open", "{", "{", 10, 11)),
			v098TestText(t, "A | B", "A | B", 3, 2), v098TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested block material excluded", func(t *testing.T) {
		v098TestRun(t, v098TestLesson(t, v098TestOpen(t, "table", 2, 1), v098TestText(t, "A | B", "A | B", 3, 2),
			v098TestOpen(t, "note", 3, 2), v098TestText(t, "<x | y", "<x | y", 5, 3),
			v098TestClose(t, 5, 3), v098TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested single line construction excluded", func(t *testing.T) {
		v098TestRun(t, v098TestTable(t, v098TestText(t, "A | B", "A | B", 3, 2),
			v098TestLine(t, "@note <x | y", "tag", 3, 2, v098TestElement(t, "tag", "@note", "note", 1, 6),
				v098TestElement(t, "content", "<x | y", "<x | y", 7, 13))), "current", nil)
	})
	t.Run("outside and after table", func(t *testing.T) {
		v098TestRun(t, v098TestLesson(t, v098TestText(t, "<x | y", "<x | y", 2, 1),
			v098TestOpen(t, "table", 2, 1), v098TestText(t, "A | B", "A | B", 4, 2), v098TestClose(t, 4, 2),
			v098TestText(t, "<x | y", "<x | y", 2, 1)), "current", nil)
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal pseudo table/"+name, func(t *testing.T) {
			v098TestRun(t, v098TestLesson(t, v098TestOpen(t, name, 2, 1), v098TestText(t, "@table {", "@table {", 3, 2),
				v098TestText(t, "<x | y", "<x | y", 3, 2), v098TestText(t, `\}`, "}", 3, 2), v098TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("uppercase table", func(t *testing.T) {
		result := v098TestTable(t, v098TestText(t, "<a | b", "<a | b", 3, 2))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@TABLE {", "@TABLE"
		v098TestRun(t, result, "current", []v098TestExpected{{4, 1, 2}})
	})
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"separate content elements", v098TestLine(t, "A | <x | y", "content", 3, 2,
			v098TestElement(t, "content", "A", "A", 1, 2), v098TestElement(t, "content", "x", "x", 6, 7), v098TestElement(t, "content", "y", "y", 10, 11))},
		{"unparsed tail", v098TestLine(t, "A | <x | y", "content", 3, 2,
			v098TestElement(t, "content", "A", "A", 1, 2), v098TestElement(t, "unparsed", "<x | y", "", 5, 11))},
		{"whole row unparsed", v098TestLine(t, "A | <x | y", "content", 3, 2,
			v098TestElement(t, "unparsed", "A | <x | y", "", 1, 11))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v098TestRun(t, v098TestTable(t, tc.row), "current", []v098TestExpected{{4, 5, 6}})
		})
	}
	t.Run("row at EOF without closing table", func(t *testing.T) {
		result := v098TestLesson(t, v098TestOpen(t, "table", 2, 1), v098TestText(t, "A | <x | y", "A | <x | y", 3, 2))
		result.Lines[3].LineEnding = ""
		result.Lines[2].HasErrors, result.Document.HasErrors = true, true
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "unclosed", Source: "parser", DiagnosticCode: "fixture-unclosed",
			SeverityLevel: "error", DiagnosticScope: "block", Message: "Блок не закрыт.", RelatedLocations: []model.Location{},
			Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 9}}})
		v098TestRun(t, result, "current", []v098TestExpected{{4, 5, 6}})
	})
}

func TestV098VersionsAndLifecycle(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v098TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v098TestRun(t, result, "current", nil)
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v098TestTable(t, v098TestText(t, "<a | b", "<a | b", 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v098TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v098TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v098TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v098TestTable(t, v098TestText(t, "<a | b", "<a | b", 3, 2))
		second := v098TestTable(t, v098TestText(t, "<a | b>", "<a | b>", 3, 2))
		v098TestRun(t, first, "first", []v098TestExpected{{4, 1, 2}})
		v098TestRun(t, second, "first", nil)
		v098TestRun(t, first, "second", []v098TestExpected{{4, 1, 2}})
	})
}

// v098TestTable окружает явно заданные строки полноценным блоком таблицы.
func v098TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v098TestOpen(t, "table", 2, 1)}, rows...)
	body = append(body, v098TestClose(t, 3, 2))
	return v098TestLesson(t, body...)
}

func v098TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v098TestLine(t, raw, "content", parent, depth,
		v098TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v098TestFragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v098TestLine(t, "@fragment f {", "block-start", parent, depth,
		v098TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v098TestElement(t, "identifier", "f", "f", 11, 12), v098TestElement(t, "block-open", "{", "{", 13, 14))
}

// v098TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v098TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v098TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v098TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v098TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v098TestLine(t, "# Lesson", "heading", 0, 0,
			v098TestElement(t, "heading-level", "#", "1", 1, 2), v098TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v098TestOpen задаёт реальное объявление блока с явным родителем.
func v098TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v098TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v098TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v098TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v098TestClose задаёт закрытие указанного блока.
func v098TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v098TestLine(t, "}", "block-end", parent, depth, v098TestElement(t, "block-close", "}", "}", 1, 2))
}

// v098TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v098TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v098TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v098TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v098TestRegistry использует весь штатный каталог регистраций.
func v098TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v098TestRun проверяет только новые V098 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v098TestRun(t *testing.T, result *model.Result, source string, want []v098TestExpected) {
	t.Helper()
	before := v098TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v098TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v098TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V098" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V098 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Разделённая ячейка не закрыта до конца строки." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V098 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v098TestSnapshot(t, result)), &restored); err != nil {
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
	if v098TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v098TestSnapshot сохраняет независимое представление модели.
func v098TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

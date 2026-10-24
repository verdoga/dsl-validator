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

// v102TestExpected задаёт исходные диапазоны остатка и связанной > на одной строке.
type v102TestExpected struct{ line, start, end, relatedStart, relatedEnd int }

func TestV102Registration(t *testing.T) {
	definitions, err := v102TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V102" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "После закрывающего знака > есть текст до следующего разделителя ячеек." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V102 definitions = %d, want 1", count)
	}
}

func TestV102ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, row, fragment string
		want                []v102TestExpected
	}{
		{"reference", "<A | B> | C", "@table {\n<A | B> | C\n}", nil},
		{"example", "<A | B> C", "@table {\n<A | B> C\n}", []v102TestExpected{{4, 9, 10, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v102TestTable(t, v102TestText(t, tc.row, tc.row, 3, 2))
			// HTML-разметка исходной таблицы означает |, <, >.
			// К дословному фрагменту добавлены только обязательные версия и заголовок.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v102TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV102RemainderAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v102TestExpected
	}{
		{"closing at end", "<A | B>", nil},
		{"only whitespace", "<A | B> \t \t", nil},
		{"trim edges", "<A | B>\t C \t", []v102TestExpected{{4, 10, 11, 7, 8}}},
		{"preserve internal spaces", "<A | B> C  D", []v102TestExpected{{4, 9, 13, 7, 8}}},
		{"preserve internal tab", "<A | B> C\tD", []v102TestExpected{{4, 9, 12, 7, 8}}},
		{"external separator ends remainder", "<A | B> C | D", []v102TestExpected{{4, 9, 10, 7, 8}}},
		{"no gap before text", "<A|B>C", []v102TestExpected{{4, 6, 7, 5, 6}}},
		{"two violations", "<A|B> C | <D|E> F", []v102TestExpected{{4, 7, 8, 5, 6}, {4, 17, 18, 15, 16}}},
		{"no new cell without separator", "<A|B> <C>", []v102TestExpected{{4, 7, 10, 5, 6}}},
		{"pipe inside invalid tail is external", "<A|B> <C|D> E", []v102TestExpected{{4, 7, 9, 5, 6}}},
		{"extra closing", "<A|B>>", []v102TestExpected{{4, 6, 7, 5, 6}}},
		{"empty outer cells", "||<A|B>C||", []v102TestExpected{{4, 8, 9, 7, 8}}},
		{"ordinary cell", "x<A|B> C", nil},
		{"leading space and tab", " \t<A|B> C", []v102TestExpected{{4, 9, 10, 7, 8}}},
		{"leading NBSP is content", "\u00a0<A|B> C", nil},
		{"NBSP in tail is content", "<A|B>\u00a0", []v102TestExpected{{4, 6, 7, 5, 6}}},
		{"Unicode columns", "Я😀 | <A|B> Ж", []v102TestExpected{{4, 12, 13, 10, 11}}},
		{"unclosed", "<A | B", nil},
		{"closed nested", "<A|<B|C>> X", nil},
		{"nested opening in text", "<A x<B|C>> X", nil},
		{"unclosed nested", "<A|<B|C> X | <D|E> F", nil},
		{"resume after nested cell", "<A|<B|C>> X | <D|E> F", []v102TestExpected{{4, 21, 22, 19, 20}}},
		{"nested cell does not suppress previous", "<A|B> C | <D|<E|F>> X", []v102TestExpected{{4, 7, 8, 5, 6}}},
		{"single internal part is separate problem", "<A> C", []v102TestExpected{{4, 5, 6, 3, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Вложенность и другие ошибки формы не подменяют требования V102.
			v102TestRun(t, v102TestTable(t, v102TestText(t, tc.raw, strings.Trim(tc.raw, " \t"), 3, 2)), "current", tc.want)
		})
	}
	for _, char := range []string{"\v", "\f", "\u0085", "\u1680", "\u2000", "\u2003", "\u200b", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("non ASCII whitespace is retained/"+strconv.Quote(char), func(t *testing.T) {
			raw := "<A|B> \t" + char + "\t "
			v102TestRun(t, v102TestTable(t, v102TestText(t, raw, "<A|B> \t"+char, 3, 2)), "current", []v102TestExpected{{4, 8, 9, 5, 6}})
		})
	}
}

func TestV102Escapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v102TestExpected
	}{
		{"escaped tail pipe", `<A|B> \| C`, `<A|B> | C`, []v102TestExpected{{4, 7, 11, 5, 6}}},
		{"escaped tail closing", `<A|B> \>`, `<A|B> >`, []v102TestExpected{{4, 7, 9, 5, 6}}},
		{"escaped tail opening", `<A|B> \<`, `<A|B> <`, []v102TestExpected{{4, 7, 9, 5, 6}}},
		{"two slashes before external pipe", `<A|B> \\| C`, `<A|B> \| C`, []v102TestExpected{{4, 7, 9, 5, 6}}},
		{"three slashes before external pipe", `<A|B> \\\| C`, `<A|B> \| C`, []v102TestExpected{{4, 7, 13, 5, 6}}},
		{"escaped opening", `\<A | B> C`, `<A | B> C`, nil},
		{"escaped first pipe is content", `\|<A|B> C`, `|<A|B> C`, nil},
		{"first backslash pair is content", `\\<A|B> C`, `\<A|B> C`, nil},
		{"escaped closing without real closing", `<A | B\> C`, `<A | B> C`, nil},
		{"real closing after escaped closing", `<A|B\>C> X`, `<A|B>C> X`, []v102TestExpected{{4, 10, 11, 8, 9}}},
		{"two slashes before closing", `<A|B\\> X`, `<A|B\> X`, []v102TestExpected{{4, 9, 10, 7, 8}}},
		{"three slashes before closing", `<A|B\\\> X`, `<A|B\> X`, nil},
		{"four slashes before closing", `<A|B\\\\> X`, `<A|B\\> X`, []v102TestExpected{{4, 11, 12, 9, 10}}},
		{"literal opening is not nested", `<A \< B | C> X`, `<A < B | C> X`, []v102TestExpected{{4, 14, 15, 12, 13}}},
		{"backslash pair before nested opening", `<A\\<B|C>> X`, `<A\<B|C>> X`, nil},
		{"ordinary backslash before nested opening", `<A\q<B|C>> X`, `<A\q<B|C>> X`, nil},
		{"final backslash in tail", `<A|B> \`, `<A|B> \`, []v102TestExpected{{4, 7, 8, 5, 6}}},
		{"final backslash inside unclosed cell", `<A|B\`, `<A|B\`, nil},
		{"unknown escape in tail", `<A|B> \q`, `<A|B> \q`, []v102TestExpected{{4, 7, 9, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v102TestRun(t, v102TestTable(t, v102TestText(t, tc.raw, tc.value, 3, 2)), "current", tc.want)
		})
	}
}

func TestV102RowsAndRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v102TestExpected
	}{
		{"header and body", []model.Line{
			v102TestText(t, "<A|B> C", "<A|B> C", 3, 2), v102TestLine(t, "---", "separator", 3, 2),
			v102TestText(t, "<A|B> C", "<A|B> C", 3, 2),
		}, []v102TestExpected{{4, 7, 8, 5, 6}, {6, 7, 8, 5, 6}}},
		{"next row does not close previous cell", []model.Line{
			v102TestText(t, "<A | B", "<A | B", 3, 2), v102TestText(t, "> C", "> C", 3, 2),
		}, nil},
		{"next row remains independently checked", []model.Line{
			v102TestText(t, "<A | B", "<A | B", 3, 2), v102TestText(t, "<A | B> C", "<A | B> C", 3, 2),
		}, []v102TestExpected{{5, 9, 10, 7, 8}}},
		{"blanks and separator", []model.Line{
			v102TestText(t, "A | B", "A | B", 3, 2), v102TestLine(t, "", "blank", 3, 2),
			v102TestLine(t, " \t---\t", "separator", 3, 2), v102TestLine(t, " \t", "blank", 3, 2),
			v102TestText(t, "<C|D> | E", "<C|D> | E", 3, 2),
		}, nil},
		{"separate content elements", []model.Line{v102TestLine(t, "<A | B> C", "content", 3, 2,
			v102TestElement(t, "content", "A", "A", 2, 3), v102TestElement(t, "content", "B", "B", 6, 7),
			v102TestElement(t, "content", "C", "C", 9, 10))}, []v102TestExpected{{4, 9, 10, 7, 8}}},
		{"unparsed tail", []model.Line{v102TestLine(t, "<A | B> C", "content", 3, 2,
			v102TestElement(t, "content", "A", "A", 2, 3), v102TestElement(t, "content", "B", "B", 6, 7),
			v102TestElement(t, "unparsed", "C", "", 9, 10))}, []v102TestExpected{{4, 9, 10, 7, 8}}},
		{"whole row unparsed", []model.Line{v102TestLine(t, "<A | B> C", "content", 3, 2,
			v102TestElement(t, "unparsed", "<A | B> C", "", 1, 10))}, []v102TestExpected{{4, 9, 10, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v102TestRun(t, v102TestTable(t, tc.rows...), "current", tc.want)
		})
	}
}

func TestV102Contexts(t *testing.T) {
	t.Run("inside fragment", func(t *testing.T) {
		v102TestRun(t, v102TestLesson(t, v102TestFragment(t, 2, 1), v102TestOpen(t, "table", 3, 2),
			v102TestText(t, "<A | B> C", "<A | B> C", 4, 3), v102TestClose(t, 4, 3), v102TestClose(t, 3, 2)),
			"current", []v102TestExpected{{5, 9, 10, 7, 8}})
	})
	t.Run("title is not row data", func(t *testing.T) {
		v102TestRun(t, v102TestLesson(t,
			v102TestLine(t, "@table <A|B> C {", "block-start", 2, 1,
				v102TestElement(t, "tag", "@table", "table", 1, 7), v102TestElement(t, "title", "<A|B> C", "<A|B> C", 8, 15),
				v102TestElement(t, "block-open", "{", "{", 16, 17)),
			v102TestText(t, "A | B", "A | B", 3, 2), v102TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested block excluded", func(t *testing.T) {
		v102TestRun(t, v102TestLesson(t, v102TestOpen(t, "table", 2, 1), v102TestText(t, "A | B", "A | B", 3, 2),
			v102TestOpen(t, "note", 3, 2), v102TestText(t, "<A | B> C", "<A | B> C", 5, 3),
			v102TestClose(t, 5, 3), v102TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested tag excluded", func(t *testing.T) {
		v102TestRun(t, v102TestTable(t, v102TestText(t, "A | B", "A | B", 3, 2),
			v102TestLine(t, "@note <A|B> C", "tag", 3, 2, v102TestElement(t, "tag", "@note", "note", 1, 6),
				v102TestElement(t, "content", "<A|B> C", "<A|B> C", 7, 14))), "current", nil)
	})
	t.Run("outside and after table", func(t *testing.T) {
		v102TestRun(t, v102TestLesson(t, v102TestText(t, "<A|B> C", "<A|B> C", 2, 1),
			v102TestOpen(t, "table", 2, 1), v102TestText(t, "A | B", "A | B", 4, 2), v102TestClose(t, 4, 2),
			v102TestText(t, "<A|B> C", "<A|B> C", 2, 1)), "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo table/"+tag, func(t *testing.T) {
			v102TestRun(t, v102TestLesson(t, v102TestOpen(t, tag, 2, 1), v102TestText(t, "@table {", "@table {", 3, 2),
				v102TestText(t, "<A|B> C", "<A|B> C", 3, 2), v102TestText(t, `\}`, "}", 3, 2), v102TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("uppercase tag", func(t *testing.T) {
		result := v102TestTable(t, v102TestText(t, "<A|B> C", "<A|B> C", 3, 2))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@TABLE {", "@TABLE"
		v102TestRun(t, result, "current", []v102TestExpected{{4, 7, 8, 5, 6}})
	})
}

func TestV102VersionsAndLifecycle(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v102TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v102TestRun(t, result, "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v102TestTable(t, v102TestText(t, "<A | B> C", "<A | B> C", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[4].LineEnding = ""
				}
				v102TestRun(t, result, "current", []v102TestExpected{{4, 9, 10, 7, 8}})
			})
		}
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v102TestTable(t, v102TestText(t, "<A | B> C", "<A | B> C", 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v102TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v102TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v102TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v102TestTable(t, v102TestText(t, "<A | B> C", "<A | B> C", 3, 2))
		second := v102TestTable(t, v102TestText(t, "<A | B> | C", "<A | B> | C", 3, 2))
		v102TestRun(t, first, "first", []v102TestExpected{{4, 9, 10, 7, 8}})
		v102TestRun(t, second, "first", nil)
		v102TestRun(t, first, "second", []v102TestExpected{{4, 9, 10, 7, 8}})
	})
}

// v102TestTable окружает явно заданные строки полноценным блоком таблицы.
func v102TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v102TestOpen(t, "table", 2, 1)}, rows...)
	body = append(body, v102TestClose(t, 3, 2))
	return v102TestLesson(t, body...)
}

func v102TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v102TestLine(t, raw, "content", parent, depth,
		v102TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v102TestFragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v102TestLine(t, "@fragment f {", "block-start", parent, depth,
		v102TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v102TestElement(t, "identifier", "f", "f", 11, 12), v102TestElement(t, "block-open", "{", "{", 13, 14))
}

// v102TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v102TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v102TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v102TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v102TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v102TestLine(t, "# Lesson", "heading", 0, 0,
			v102TestElement(t, "heading-level", "#", "1", 1, 2), v102TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v102TestOpen задаёт реальное объявление блока с явным родителем.
func v102TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v102TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v102TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v102TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v102TestClose задаёт закрытие указанного блока.
func v102TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v102TestLine(t, "}", "block-end", parent, depth, v102TestElement(t, "block-close", "}", "}", 1, 2))
}

// v102TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v102TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v102TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v102TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v102TestRegistry использует весь штатный каталог регистраций.
func v102TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v102TestRun проверяет только новые V102 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v102TestRun(t *testing.T, result *model.Result, source string, want []v102TestExpected) {
	t.Helper()
	before := v102TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v102TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v102TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V102" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V102 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		related := model.Location{Start: model.Position{Line: position.line, Column: position.relatedStart}, End: model.Position{Line: position.line, Column: position.relatedEnd}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "После закрывающего знака > есть текст до следующего разделителя ячеек.\nконкретизация: Следующая внешняя ячейка отделяется вертикальной чертой; буквальный > внутри части экранируется" {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V102 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v102TestSnapshot(t, result)), &restored); err != nil {
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
	if v102TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v102TestSnapshot сохраняет независимое представление модели.
func v102TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

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

func TestV100Registration(t *testing.T) {
	definitions, err := v100TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V100" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Внутри разделённой ячейки находится неэкранированный знак <." ||
			definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V100 definitions = %d, want 1", count)
	}
}

func TestV100ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][4]int
	}{
		{"reference", `<1 \< 2 | 3 \> 2>`, "<1 < 2 | 3 > 2>", nil},
		{"example", "<A | <B | C>>", "<A | <B | C>>", [][4]int{{4, 6, 4, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// HTML-сущности в таблице задания — разметка. Добавлены только
			// обязательные версия и заголовок; сами примеры сохранены.
			result := v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
				v100TestText(t, tc.raw, tc.value, 3, 2), v100TestClose(t, 3, 2))
			v100TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@table {\n"+tc.raw+"\n}")
			v100TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV100DepthAndOuterCells(t *testing.T) {
	// Восстановление требуется и для ошибочных ячеек без разделителей
	// либо закрытия. Строки сохраняются как content по согласованному правилу.
	for _, tc := range []struct {
		name, raw, value string
		want             [][4]int
	}{
		{"ordinary cell", "A<B<C>>", "A<B<C>>", nil},
		{"single opening", "<", "<", nil},
		{"nested at last column", "<A|<", "<A|<", [][4]int{{4, 4, 4, 5}}},
		{"three levels", "<A<B<C>>>", "<A<B<C>>>", [][4]int{{4, 3, 4, 4}, {4, 5, 4, 6}}},
		{"partial closing", "<A<B><C>>", "<A<B><C>>", [][4]int{{4, 3, 4, 4}, {4, 6, 4, 7}}},
		{"full closing", "<A>B<C>", "<A>B<C>", nil},
		{"extra closing does not make negative depth", "> | <A|<B>>", "> | <A|<B>>", [][4]int{{4, 8, 4, 9}}},
		{"first outer cell", "<A|<B>>|X", "<A|<B>>|X", [][4]int{{4, 4, 4, 5}}},
		{"last outer cell", "X|<A|<B>>", "X|<A|<B>>", [][4]int{{4, 6, 4, 7}}},
		{"empty outer cells", "||<A|<B>>|", "||<A|<B>>|", [][4]int{{4, 6, 4, 7}}},
		{"several outer cells", "<A|<B>>|<C|<D>>", "<A|<B>>|<C|<D>>", [][4]int{{4, 4, 4, 5}, {4, 12, 4, 13}}},
		{"inner pipe does not reset depth", "<A|<B>|<C>>", "<A|<B>|<C>>", [][4]int{{4, 4, 4, 5}, {4, 8, 4, 9}}},
		{"outer pipe resets start", "A|<B<C>>", "A|<B<C>>", [][4]int{{4, 5, 4, 6}}},
		{"adjacent valid cells", "<A|B>|<C|D>", "<A|B>|<C|D>", nil},
		{"literal opening before outer pipe", "A<B|<C|D>", "A<B|<C|D>", nil},
		{"spaces and tabs at cell start", " \t<A<B>>\t ", "<A<B>>", [][4]int{{4, 5, 4, 6}}},
		{"spaces do not restore start", "A \t<B<C>>", "A \t<B<C>>", nil},
		{"NBSP at cell start", "\u00a0<A<B>>", "\u00a0<A<B>>", nil},
		{"em space at cell start", "\u2003<A<B>>", "\u2003<A<B>>", nil},
		{"Unicode columns", "<Я😀<Б>>", "<Я😀<Б>>", [][4]int{{4, 4, 4, 5}}},
		{"combining mark is a separate column", "<е\u0301<Б>>", "<е\u0301<Б>>", [][4]int{{4, 4, 4, 5}}},
		{"HTML entities stay text", "&lt;A&lt;B&gt;&gt;", "&lt;A&lt;B&gt;&gt;", nil},
		{"similar Unicode angles", "＜A＜B＞＞", "＜A＜B＞＞", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v100TestRun(t, v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
				v100TestText(t, tc.raw, tc.value, 3, 2), v100TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV100Escapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][4]int
	}{
		{"escaped opening", `<A\<B>`, "<A<B>", nil},
		{"escaped closing", `<A\><B>>`, "<A><B>>", [][4]int{{4, 5, 4, 6}}},
		{"paired slashes before opening", `<A\\<B>>`, `<A\<B>>`, [][4]int{{4, 5, 4, 6}}},
		{"odd slashes before opening", `<A\\\<B>`, `<A\<B>`, nil},
		{"paired slashes before closing", `<A\\><B>`, `<A\><B>`, nil},
		{"odd slashes before closing", `<A\\\><B>>`, `<A\><B>>`, [][4]int{{4, 7, 4, 8}}},
		{"escaped inner pipe", `<A\|<B>>`, "<A|<B>>", [][4]int{{4, 5, 4, 6}}},
		{"escaped outer pipe", `A\|<B<C>>`, "A|<B<C>>", nil},
		{"paired slashes before outer pipe", `A\\|<B<C>>`, `A\|<B<C>>`, [][4]int{{4, 7, 4, 8}}},
		{"odd slashes before outer pipe", `A\\\|<B<C>>`, `A\|<B<C>>`, nil},
		{"escaped opening at start is data", `\<A<B>>`, "<A<B>>", nil},
		{"escaped closing at start is data", `\><A<B>>`, "><A<B>>", nil},
		{"escaped pipe at start is data", `\| <A<B>>`, "| <A<B>>", nil},
		{"paired slashes at start are data", `\\<A<B>>`, `\<A<B>>`, nil},
		{"unknown escape at start is data", `\x<A<B>>`, `\x<A<B>>`, nil},
		{"unknown escape inside", `<A\x<B>>`, `<A\x<B>>`, [][4]int{{4, 5, 4, 6}}},
		{"backslash at end", `<A|B\`, `<A|B\`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v100TestRun(t, v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
				v100TestText(t, tc.raw, tc.value, 3, 2), v100TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV100RowsAndUnparsed(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want [][4]int
	}{
		{"empty table", nil, nil},
		{"blank rows", []model.Line{v100TestLine(t, "", "blank", 3, 2),
			v100TestLine(t, " \t ", "blank", 3, 2)}, nil},
		{"first and last data rows", []model.Line{
			v100TestText(t, "<A|<B>>", "<A|<B>>", 3, 2), v100TestLine(t, "", "blank", 3, 2),
			v100TestText(t, "<C|<D>>", "<C|<D>>", 3, 2),
		}, [][4]int{{4, 4, 4, 5}, {6, 4, 6, 5}}},
		{"header and body", []model.Line{
			v100TestText(t, "<A<B>>", "<A<B>>", 3, 2), v100TestLine(t, " \t---\t ", "separator", 3, 2),
			v100TestText(t, "<C<D>>", "<C<D>>", 3, 2),
		}, [][4]int{{4, 3, 4, 4}, {6, 3, 6, 4}}},
		{"new row resets depth", []model.Line{
			v100TestText(t, "<A", "<A", 3, 2), v100TestText(t, "<B<C>>", "<B<C>>", 3, 2),
		}, [][4]int{{5, 3, 5, 4}}},
		{"new row resets outer start", []model.Line{
			v100TestText(t, "A", "A", 3, 2), v100TestText(t, "<B<C>>", "<B<C>>", 3, 2),
		}, [][4]int{{5, 3, 5, 4}}},
		{"unparsed suffix participates", []model.Line{
			v100TestLine(t, "<A<B>>", "content", 3, 2,
				v100TestElement(t, "content", "<A", "<A", 1), v100TestUnparsed(t, "<B>>", 3)),
		}, [][4]int{{4, 3, 4, 4}}},
		{"unparsed opening participates", []model.Line{
			v100TestLine(t, "<A<B>>", "content", 3, 2,
				v100TestUnparsed(t, "<A", 1), v100TestElement(t, "content", "<B>>", "<B>>", 3)),
		}, [][4]int{{4, 3, 4, 4}}},
		{"fully unparsed content", []model.Line{
			v100TestLine(t, "<A<B>>", "content", 3, 2, v100TestUnparsed(t, "<A<B>>", 1)),
		}, [][4]int{{4, 3, 4, 4}}},
		{"invalid is excluded", []model.Line{v100TestInvalid(t, "<A<B>>", 3, 2)}, nil},
		{"markers outside content elements", []model.Line{
			v100TestLine(t, "<A|<B|C>>", "content", 3, 2,
				v100TestElement(t, "content", "A", "A", 2),
				v100TestElement(t, "content", "B", "B", 5),
				v100TestElement(t, "content", "C", "C", 7)),
		}, [][4]int{{4, 4, 4, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v100TestOpen(t, "table", 2, 1)}, tc.rows...)
			body = append(body, v100TestClose(t, 3, 2))
			v100TestRun(t, v100TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV100Contexts(t *testing.T) {
	t.Run("outside and after table", func(t *testing.T) {
		v100TestRun(t, v100TestDocument(t, v100TestText(t, "<A<B>>", "<A<B>>", 2, 1),
			v100TestOpen(t, "table", 2, 1), v100TestText(t, "<A<B>>", "<A<B>>", 4, 2),
			v100TestClose(t, 4, 2), v100TestText(t, "<A<B>>", "<A<B>>", 2, 1)), "current",
			[][4]int{{5, 3, 5, 4}})
	})
	t.Run("title is excluded and tag is canonical", func(t *testing.T) {
		v100TestRun(t, v100TestDocument(t,
			v100TestLine(t, "@TABLE <A<B>> {", "block-start", 2, 1,
				v100TestElement(t, "tag", "@TABLE", "table", 1),
				v100TestElement(t, "title", "<A<B>>", "<A<B>>", 8),
				v100TestElement(t, "block-open", "{", "{", 15)),
			v100TestText(t, "<A<B>>", "<A<B>>", 3, 2), v100TestClose(t, 3, 2)),
			"current", [][4]int{{4, 3, 4, 4}})
	})
	t.Run("table inside another block", func(t *testing.T) {
		v100TestRun(t, v100TestDocument(t, v100TestOpen(t, "note", 2, 1),
			v100TestOpen(t, "table", 3, 2), v100TestText(t, "<A<B>>", "<A<B>>", 4, 3),
			v100TestClose(t, 4, 3), v100TestClose(t, 3, 2)), "current", [][4]int{{5, 3, 5, 4}})
	})
	t.Run("nested construction is not a table row", func(t *testing.T) {
		v100TestRun(t, v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
			v100TestText(t, "A|B", "A|B", 3, 2), v100TestOpen(t, "note", 3, 2),
			v100TestText(t, "<A<B>>", "<A<B>>", 5, 3), v100TestClose(t, 5, 3),
			v100TestText(t, "C|D", "C|D", 3, 2), v100TestClose(t, 3, 2)), "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			// В литеральном блоке @table не образует отдельного родителя.
			v100TestRun(t, v100TestDocument(t, v100TestOpen(t, tag, 2, 1),
				v100TestText(t, "@table {", "@table {", 3, 2),
				v100TestText(t, "<A<B>>", "<A<B>>", 3, 2),
				v100TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("unfinished table still has data rows", func(t *testing.T) {
		// Для line-диагностики V100 закрытие table не является условием.
		v100TestRun(t, v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
			v100TestText(t, "<A|<B>>", "<A|<B>>", 3, 2)), "current", [][4]int{{4, 4, 4, 5}})
	})
}

func TestV100EndingsVersionsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"no final EOL", "\n", ""}, {"CRLF no final EOL", "\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
				v100TestText(t, "<A | <B | C>>", "<A | <B | C>>", 3, 2), v100TestClose(t, 3, 2))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v100TestRun(t, result, "first", [][4]int{{4, 6, 4, 7}})
			v100TestRun(t, result, "second", [][4]int{{4, 6, 4, 7}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v100TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v100TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v100TestDocument(t, v100TestOpen(t, "table", 2, 1),
				v100TestText(t, "<A<B>>", "<A<B>>", 3, 2), v100TestClose(t, 3, 2))
			result.Document.DSLVersion = &version
			before := v100TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v100TestRegistry(t))
			if err == nil || checks != nil || v100TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v100TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v100TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v100TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v100TestHeading(t, "#", "Lesson"),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v100TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v100TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v100TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v100TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v100TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v100TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v100TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v100TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v100TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v100TestLine(t, raw, "invalid", parent, depth, v100TestUnparsed(t, raw, 1))
}

func v100TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v100TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v100TestElement(t, "tag", "@"+tag, tag, 1), v100TestElement(t, kind, value, value, len(tag)+3))
}

func v100TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v100TestLine(t, marker+" "+title, "heading", 0, 0,
		v100TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v100TestElement(t, "title", title, title, len(marker)+2))
}

func v100TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v100TestLine(t, raw, "content", parent, depth, v100TestElement(t, "content", raw, value, 1))
}

func v100TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v100TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v100TestElement(t, "tag", "@"+tag, tag, 1), v100TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v100TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v100TestLine(t, "}", "block-end", parent, depth, v100TestElement(t, "block-close", "}", "}", 1))
}

func v100TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v100TestRun выполняет полный каталог и сравнивает новые V100 с явными
// четвёрками начальная строка/колонка и конечная строка/колонка. Правые границы исключаются.
func v100TestRun(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v100TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v100TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v100TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V100" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V100 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[2], Column: position[3]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "Внутри разделённой ячейки находится неэкранированный знак <.\nконкретизация: Вложенные разделённые ячейки не поддерживаются; буквальный знак записывается как \\<" {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V100 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v100TestSnapshot(t, result)), &restored); err != nil {
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
	if v100TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v100TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v100TestSource(t *testing.T, result *model.Result, want string) {
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

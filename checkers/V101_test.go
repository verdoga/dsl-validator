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

// v101TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V101.
func v101TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v101TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v101TestHeading(t, "", "#", "1", title),
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

// v101TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v101TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v101TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v101TestElement(t, "heading-level", marker, level, start, end),
		v101TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v101TestTag собирает однострочный тег с одним явно типизированным параметром.
func v101TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v101TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v101TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v101TestLine(t, raw, "tag", parent, depth, elements...)
}

// v101TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v101TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v101TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v101TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v101TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v101TestClose задаёт закрытие блока с явным родителем.
func v101TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v101TestLine(t, "}", "block-end", parent, depth, v101TestElement(t, "block-close", "}", "}", 1, 2))
}

// v101TestText сохраняет независимо заданные исходное и семантическое значения.
func v101TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v101TestLine(t, raw, "content", parent, depth,
		v101TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v101TestLine не классифицирует исходник и не вычисляет родительские связи.
func v101TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v101TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v101TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v101TestRegistry использует штатный каталог целиком.
func v101TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v101TestRun проверяет новые V101: строка, начало и исключающий конец.
func v101TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v101TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v101TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v101TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V101" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V101 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Разделённая ячейка содержит меньше двух внутренних частей." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V101 metadata", i, entry, location)
		}
		if len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v101TestSnapshot(t, result)), &restored); err != nil {
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
	if v101TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v101TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v101TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV101Registration(t *testing.T) {
	definitions, err := v101TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V101" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Разделённая ячейка содержит меньше двух внутренних частей." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V101 definitions = %d, want 1", count)
	}
}

// v101TestTitled задаёт блок с отдельным элементом названия.
func v101TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v101TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v101TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v101TestElement(t, "title", title, title, len(tag)+3, end),
		v101TestElement(t, "block-open", "{", "{", end+1, end+2))
}

func TestV101ReferenceAndExample(t *testing.T) {
	// По DSL 1.30 добавлены только обязательные @dsl-version и # Lesson.
	// Сущности и <br> таблицы задания — оформление, а не исходные символы DSL.
	for _, tc := range []struct {
		name, fragment, row string
		want                [][3]int
	}{
		{"reference", "@table {\n<A | B>\n}", "<A | B>", nil},
		{"example", "@table {\n<A>\n}", "<A>", [][3]int{{4, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v101TestDocument(t, v101TestOpen(t, "table", 2, 1),
				v101TestText(t, tc.row, tc.row, 3, 2), v101TestClose(t, 3, 2))
			var raw strings.Builder
			for _, line := range result.Lines[2:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", raw.String())
			}
			v101TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV101Cells(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"one part", "<A>", "<A>", [][3]int{{4, 1, 4}}},
		{"empty part", "<>", "<>", [][3]int{{4, 1, 3}}},
		{"spaces inside", "<   >", "<   >", [][3]int{{4, 1, 6}}},
		{"two empty parts", "<|>", "<|>", nil},
		{"empty last", "<A|>", "<A|>", nil},
		{"empty first", "<|B>", "<|B>", nil},
		{"three parts", "<A|B|C>", "<A|B|C>", nil},
		{"three empty parts", "<||>", "<||>", nil},
		{"blank parts", "< \t|\t >", "< \t|\t >", nil},
		{"comma", "<A,B>", "<A,B>", [][3]int{{4, 1, 6}}},
		{"space and tab", "<A B\tC>", "<A B\tC>", [][3]int{{4, 1, 8}}},
		{"punctuation", "<A;B:C!>", "<A;B:C!>", [][3]int{{4, 1, 9}}},
		{"escaped pipe", `<A\|B>`, "<A|B>", [][3]int{{4, 1, 7}}},
		{"paired slashes before pipe", `<A\\|B>`, `<A\|B>`, nil},
		{"three slashes before pipe", `<A\\\|B>`, `<A\|B>`, [][3]int{{4, 1, 9}}},
		{"four slashes before pipe", `<A\\\\|B>`, `<A\\|B>`, nil},
		{"escaped and active pipes", `<A\|B|C>`, "<A|B|C>", nil},
		{"double quotes do not hide pipe", `<"A|B">`, `<"A|B">`, nil},
		{"single quotes do not hide pipe", `<'A|B'>`, `<'A|B'>`, nil},
		{"quotes without separator", `<"A">`, `<"A">`, [][3]int{{4, 1, 6}}},
		{"escaped angles inside", `<A\<B\>>`, "<A<B>>", [][3]int{{4, 1, 9}}},
		{"escaped opening", `\<A>`, "<A>", nil},
		{"escaped closing", `<A\>`, "<A>", nil},
		{"paired slashes before closing", `<A\\>`, `<A\>`, [][3]int{{4, 1, 6}}},
		{"three slashes before closing", `<A\\\>`, `<A\>`, nil},
		{"four slashes before closing", `<A\\\\>`, `<A\\>`, [][3]int{{4, 1, 8}}},
		{"literal slash prevents opening", `\\<A>`, `\<A>`, nil},
		{"unknown escape preserved", `<A\q>`, `<A\q>`, [][3]int{{4, 1, 6}}},
		{"unclosed", "<A", "<A", nil},
		{"nested", "<A<B>>", "<A<B>>", nil},
		{"nested after internal separator", "<A|<B>>", "<A|<B>>", nil},
		{"paired slashes before nested opening", `<A\\<B>>`, `<A\<B>>`, nil},
		{"opening inside ordinary cell", "text<A>", "text<A>", nil},
		{"opening inside quotes", `"<A>"`, `"<A>"`, nil},
		{"text after closing", "<A>text", "<A>text", nil},
		{"tab then text after closing", "<A>\ttext", "<A>\ttext", nil},
		{"extra closing", "<A>>", "<A>>", nil},
		{"adjacent cells without separator", "<A><B>", "<A><B>", nil},
		{"escaped pipe after closing", `<A>\|B`, "<A>|B", nil},
		{"paired slashes after closing", `<A>\\|B`, `<A>\|B`, nil},
		{"ascii margins", " \t<A>\t ", "<A>", [][3]int{{4, 3, 6}}},
		{"unicode prefix", "Я😀 | <Б>", "Я😀 | <Б>", [][3]int{{4, 6, 9}}},
		{"unicode content", "<Я😀>", "<Я😀>", [][3]int{{4, 1, 5}}},
		{"nbsp before opening", "\u00a0<A>", "\u00a0<A>", nil},
		{"nbsp after closing", "<A>\u00a0", "<A>\u00a0", nil},
		{"nbsp inside", "<\u00a0>", "<\u00a0>", [][3]int{{4, 1, 4}}},
		{"two violations", "<A> | <B>", "<A> | <B>", [][3]int{{4, 1, 4}, {4, 7, 10}}},
		{"bad ending then violation", "<A>x | <B>", "<A>x | <B>", [][3]int{{4, 8, 11}}},
		{"violation then bad ending", "<A> | <B>x", "<A> | <B>x", [][3]int{{4, 1, 4}}},
		{"nested opening then violation", "<A<B> | <C>", "<A<B> | <C>", [][3]int{{4, 9, 12}}},
		{"closed nested then violation", "<A<B>> | <C>", "<A<B>> | <C>", [][3]int{{4, 10, 13}}},
		{"unclosed later cell", "<A> | <B", "<A> | <B", [][3]int{{4, 1, 4}}},
		{"unclosed outer does not expose inner", "<A | <B>", "<A | <B>", nil},
		{"empty outside cells", "|<A>||<B>|", "|<A>||<B>|", [][3]int{{4, 2, 5}, {4, 7, 10}}},
		{"valid then invalid minimum", "<A|B>|<C>", "<A|B>|<C>", [][3]int{{4, 7, 10}}},
		{"invalid minimum then valid", "<A>|<B|C>", "<A>|<B|C>", [][3]int{{4, 1, 4}}},
		{"ordinary closing then violation", "text> | <A>", "text> | <A>", [][3]int{{4, 9, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v101TestRun(t, v101TestDocument(t, v101TestOpen(t, "table", 2, 1),
				v101TestText(t, tc.raw, tc.value, 3, 2), v101TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV101Scope(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"header and body", []model.Line{v101TestOpen(t, "table", 2, 1),
			v101TestText(t, "<A>", "<A>", 3, 2), v101TestLine(t, "---", "separator", 3, 2),
			v101TestText(t, "<B>", "<B>", 3, 2), v101TestClose(t, 3, 2)}, [][3]int{{4, 1, 4}, {6, 1, 4}}},
		{"title excluded", []model.Line{v101TestTitled(t, "table", "<A>", 2, 1),
			v101TestText(t, "A", "A", 3, 2), v101TestClose(t, 3, 2)}, nil},
		{"nested other content", []model.Line{v101TestOpen(t, "table", 2, 1),
			v101TestText(t, "A", "A", 3, 2), v101TestOpen(t, "note", 3, 2),
			v101TestText(t, "<A>", "<A>", 5, 3), v101TestClose(t, 5, 3), v101TestClose(t, 3, 2)}, nil},
		{"nested table", []model.Line{v101TestOpen(t, "table", 2, 1), v101TestOpen(t, "table", 3, 2),
			v101TestText(t, "<A>", "<A>", 4, 3), v101TestClose(t, 4, 3), v101TestText(t, "A", "A", 3, 2),
			v101TestClose(t, 3, 2)}, [][3]int{{5, 1, 4}}},
		{"inline declaration", []model.Line{v101TestOpen(t, "table", 2, 1),
			v101TestTag(t, "note", "content", "<A>", 3, 2), v101TestText(t, "A", "A", 3, 2),
			v101TestClose(t, 3, 2)}, nil},
		{"uppercase tag", []model.Line{v101TestOpen(t, "TABLE", 2, 1),
			v101TestText(t, "<a>", "<a>", 3, 2), v101TestClose(t, 3, 2)}, [][3]int{{4, 1, 4}}},
		{"no table", []model.Line{v101TestText(t, "<A>", "<A>", 2, 1)}, nil},
		{"empty block", []model.Line{v101TestOpen(t, "table", 2, 1), v101TestClose(t, 3, 2)}, nil},
		{"blank rows", []model.Line{v101TestOpen(t, "table", 2, 1),
			v101TestLine(t, "", "blank", 3, 2), v101TestLine(t, " \t", "blank", 3, 2),
			v101TestText(t, "A", "A", 3, 2), v101TestClose(t, 3, 2)}, nil},
		{"separate tables", []model.Line{v101TestOpen(t, "table", 2, 1),
			v101TestText(t, "<A>", "<A>", 3, 2), v101TestClose(t, 3, 2), v101TestOpen(t, "table", 2, 1),
			v101TestText(t, "<B|C>", "<B|C>", 6, 2), v101TestClose(t, 6, 2)}, [][3]int{{4, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v101TestRun(t, v101TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tag := range []string{"note", "editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			v101TestRun(t, v101TestDocument(t, v101TestOpen(t, tag, 2, 1),
				v101TestText(t, "<A>", "<A>", 3, 2), v101TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV101Representations(t *testing.T) {
	for _, representation := range []string{"whole", "fragmented", "unparsed", "invalid"} {
		t.Run(representation, func(t *testing.T) {
			row := v101TestText(t, "<A>", "<A>", 3, 2)
			switch representation {
			case "fragmented":
				row.Elements = []model.Element{v101TestElement(t, "content", "A", "A", 2, 3)}
			case "unparsed", "invalid":
				row.Elements = []model.Element{{ElementType: "unparsed", Raw: "<A>", Start: 1, End: 4, ErrorIDs: []string{}}}
				if representation == "invalid" {
					row.LineType = "invalid"
				}
			}
			v101TestRun(t, v101TestDocument(t, v101TestOpen(t, "table", 2, 1), row,
				v101TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 4}})
		})
	}
	t.Run("partially parsed row retains following cell", func(t *testing.T) {
		row := v101TestLine(t, "<A>x | <B>", "content", 3, 2,
			model.Element{ElementType: "unparsed", Raw: "<A>x", Start: 1, End: 5, ErrorIDs: []string{}},
			v101TestElement(t, "content", "B", "B", 9, 10))
		v101TestRun(t, v101TestDocument(t, v101TestOpen(t, "table", 2, 1), row,
			v101TestClose(t, 3, 2)), "current", [][3]int{{4, 8, 11}})
	})
}

func TestV101EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v101TestDocument(t, v101TestOpen(t, "table", 2, 1),
				v101TestText(t, "<A>", "<A>", 3, 2), v101TestClose(t, 3, 2))
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v101TestRun(t, result, "first", [][3]int{{4, 1, 4}})
			v101TestRun(t, result, "second", [][3]int{{4, 1, 4}})
			v101TestRun(t, v101TestDocument(t, v101TestOpen(t, "table", 2, 1),
				v101TestText(t, "<A|B>", "<A|B>", 3, 2), v101TestClose(t, 3, 2)), "separate", nil)
		})
	}
}

func TestV101EmptyAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		for _, known := range []bool{false, true} {
			t.Run(version+" known count="+strconv.FormatBool(known), func(t *testing.T) {
				result := &model.Result{Format: model.FormatVersion1,
					Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
					Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
				if known {
					count := 0
					result.Document.LineCount = &count
				}
				if version == "1.2" {
					v101TestRun(t, result, "current", nil)
					return
				}
				before := v101TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v101TestRegistry(t))
				if err == nil || checks != nil || v101TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

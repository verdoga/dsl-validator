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

// v105TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V105.
func v105TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v105TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v105TestHeading(t, "", "#", "1", title),
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

// v105TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v105TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v105TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v105TestElement(t, "heading-level", marker, level, start, end),
		v105TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v105TestTag собирает однострочный тег с одним явно типизированным параметром.
func v105TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v105TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v105TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v105TestLine(t, raw, "tag", parent, depth, elements...)
}

// v105TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v105TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v105TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v105TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v105TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v105TestClose задаёт закрытие блока с явным родителем.
func v105TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v105TestLine(t, "}", "block-end", parent, depth, v105TestElement(t, "block-close", "}", "}", 1, 2))
}

// v105TestText сохраняет независимо заданные исходное и семантическое значения.
func v105TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v105TestLine(t, raw, "content", parent, depth,
		v105TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v105TestLine не классифицирует исходник и не вычисляет родительские связи.
func v105TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v105TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v105TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v105TestRegistry использует штатный каталог целиком.
func v105TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v105TestRun проверяет новые V105: строка и явные основной/связанный диапазоны.
func v105TestRun(t *testing.T, result *model.Result, processingID string, want [][5]int) {
	t.Helper()
	before := v105TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v105TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v105TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V105" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V105 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Перед открывающим < разделённой ячейки есть текст." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V105 metadata", i, entry, location)
		}
		related := model.Location{Start: model.Position{Line: position[0], Column: position[3]}, End: model.Position{Line: position[0], Column: position[4]}}
		if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v105TestSnapshot(t, result)), &restored); err != nil {
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
	if v105TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v105TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v105TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV105Registration(t *testing.T) {
	definitions, err := v105TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V105" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Перед открывающим < разделённой ячейки есть текст." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V105 definitions = %d, want 1", count)
	}
}

// v105TestTitled задаёт блок с отдельным элементом названия.
func v105TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v105TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v105TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v105TestElement(t, "title", title, title, len(tag)+3, end),
		v105TestElement(t, "block-open", "{", "{", end+1, end+2))
}

func TestV105ReferenceAndExample(t *testing.T) {
	// По DSL 1.30 добавлены только @dsl-version и # Lesson.
	// Сущности и <br> в задании — оформление таблицы, а не исходные символы DSL.
	for _, tc := range []struct {
		name, fragment, row string
		want                [][5]int
	}{
		{"reference", "@table {\n<a | b>\n}", "<a | b>", nil},
		{"example", "@table {\nтекст <a | b>\n}", "текст <a | b>", [][5]int{{4, 1, 6, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v105TestDocument(t, v105TestOpen(t, "table", 2, 1),
				v105TestText(t, tc.row, tc.row, 3, 2), v105TestClose(t, 3, 2))
			var raw strings.Builder
			for _, line := range result.Lines[2:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", raw.String())
			}
			v105TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV105Patterns(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][5]int
	}{
		{"adjacent opening", "x<a|b>", "x<a|b>", [][5]int{{4, 1, 2, 2, 3}}},
		{"empty parts", "x <|>", "x <|>", [][5]int{{4, 1, 2, 3, 4}}},
		{"several separators", "x <a|b|c>", "x <a|b|c>", [][5]int{{4, 1, 2, 3, 4}}},
		{"quoted separator", `x <"a|b">`, `x <"a|b">`, [][5]int{{4, 1, 2, 3, 4}}},
		{"several words", "два слова <a|b>", "два слова <a|b>", [][5]int{{4, 1, 10, 11, 12}}},
		{"ascii margins", " \tдва слова \t<a|b>", "два слова \t<a|b>", [][5]int{{4, 3, 12, 14, 15}}},
		{"internal tab preserved", "x\ty <a|b>", "x\ty <a|b>", [][5]int{{4, 1, 4, 5, 6}}},
		{"unicode prefix", "Я😀 <a|b>", "Я😀 <a|b>", [][5]int{{4, 1, 3, 4, 5}}},
		{"uppercase text", "ТЕКСТ <A|B>", "ТЕКСТ <A|B>", [][5]int{{4, 1, 6, 7, 8}}},
		{"second external cell", "A | текст <a|b>", "A | текст <a|b>", [][5]int{{4, 5, 10, 11, 12}}},
		{"two patterns", "x <a|b> | y <c|d>", "x <a|b> | y <c|d>", [][5]int{{4, 1, 2, 3, 4}, {4, 11, 12, 13, 14}}},
		{"empty external cells", "||x <a|b>||", "||x <a|b>||", [][5]int{{4, 3, 4, 5, 6}}},
		{"no nonblank prefix", " \t<a|b>", "<a|b>", nil},
		{"single comparison", "a < b", "a < b", nil},
		{"no internal separator", "x <a>", "x <a>", nil},
		{"empty brackets without separator", "x <>", "x <>", nil},
		{"no closing", "x <a|b", "x <a|b", nil},
		{"escaped opening", `x \<a|b>`, "x <a|b>", nil},
		{"paired slashes before opening", `x \\<a|b>`, `x \<a|b>`, [][5]int{{4, 1, 5, 5, 6}}},
		{"three slashes before opening", `x \\\<a|b>`, `x \<a|b>`, nil},
		{"escaped internal separator", `x <a\|b>`, "x <a|b>", nil},
		{"paired slashes before separator", `x <a\\|b>`, `x <a\|b>`, [][5]int{{4, 1, 2, 3, 4}}},
		{"three slashes before separator", `x <a\\\|b>`, `x <a\|b>`, nil},
		{"escaped and active separators", `x <a\|b|c>`, "x <a|b|c>", [][5]int{{4, 1, 2, 3, 4}}},
		{"escaped closing", `x <a|b\>`, "x <a|b>", nil},
		{"paired slashes before closing", `x <a|b\\>`, `x <a|b\>`, [][5]int{{4, 1, 2, 3, 4}}},
		{"three slashes before closing", `x <a|b\\\>`, `x <a|b\>`, nil},
		{"escaped angles inside", `x <a\<b|c\>>`, "x <a<b|c>>", [][5]int{{4, 1, 2, 3, 4}}},
		{"nested active angles", "x <a<b|c>>", "x <a<b|c>>", nil},
		{"extra closing", "x <a|b>>", "x <a|b>>", nil},
		{"text after closing", "x <a|b> tail", "x <a|b> tail", nil},
		{"allowed tail", "x <a|b> \t", "x <a|b>", [][5]int{{4, 1, 2, 3, 4}}},
		{"nbsp tail", "x <a|b>\u00a0", "x <a|b>\u00a0", nil},
		{"nbsp prefix is text", "\u00a0<a|b>", "\u00a0<a|b>", [][5]int{{4, 1, 2, 2, 3}}},
		{"escaped external separator", `x <a|b>\|y`, "x <a|b>|y", nil},
		{"escaped prefix separator", `x\|y <a|b>`, "x|y <a|b>", [][5]int{{4, 1, 5, 6, 7}}},
		{"paired prefix slashes", `x\\|y <a|b>`, `x\|y <a|b>`, [][5]int{{4, 5, 6, 7, 8}}},
		{"three prefix slashes", `x\\\|y <a|b>`, `x\|y <a|b>`, [][5]int{{4, 1, 7, 8, 9}}},
		{"inside correctly started cell", "<a | text <b|c>>", "<a | text <b|c>>", nil},
		{"inside unclosed correctly started cell", "<a | text <b|c>", "<a | text <b|c>", nil},
		{"after correctly started cell", "<a|b> | x <c|d>", "<a|b> | x <c|d>", [][5]int{{4, 9, 10, 11, 12}}},
		{"failed candidate before next cell", "x <a> | y <b|c>", "x <a> | y <b|c>", [][5]int{{4, 9, 10, 11, 12}}},
		{"failed ending before next cell", "x <a|b>z | y <c|d>", "x <a|b>z | y <c|d>", [][5]int{{4, 12, 13, 14, 15}}},
		{"punctuation is prefix", "! <a|b>", "! <a|b>", [][5]int{{4, 1, 2, 3, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v105TestRun(t, v105TestDocument(t, v105TestOpen(t, "table", 2, 1),
				v105TestText(t, tc.raw, tc.value, 3, 2), v105TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV105Scope(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][5]int
	}{
		{"header and body", []model.Line{v105TestOpen(t, "table", 2, 1),
			v105TestText(t, "x <a|b>", "x <a|b>", 3, 2), v105TestLine(t, "---", "separator", 3, 2),
			v105TestText(t, "y <c|d>", "y <c|d>", 3, 2), v105TestClose(t, 3, 2)}, [][5]int{{4, 1, 2, 3, 4}, {6, 1, 2, 3, 4}}},
		{"title excluded", []model.Line{v105TestTitled(t, "table", "x <a|b>", 2, 1),
			v105TestText(t, "A", "A", 3, 2), v105TestClose(t, 3, 2)}, nil},
		{"nested content excluded", []model.Line{v105TestOpen(t, "table", 2, 1),
			v105TestOpen(t, "note", 3, 2), v105TestText(t, "x <a|b>", "x <a|b>", 4, 3),
			v105TestClose(t, 4, 3), v105TestText(t, "A", "A", 3, 2), v105TestClose(t, 3, 2)}, nil},
		{"inline declaration excluded", []model.Line{v105TestOpen(t, "table", 2, 1),
			v105TestTag(t, "note", "content", "x <a|b>", 3, 2), v105TestText(t, "A", "A", 3, 2),
			v105TestClose(t, 3, 2)}, nil},
		{"uppercase table", []model.Line{v105TestOpen(t, "TABLE", 2, 1),
			v105TestText(t, "X <A|B>", "X <A|B>", 3, 2), v105TestClose(t, 3, 2)}, [][5]int{{4, 1, 2, 3, 4}}},
		{"no table", []model.Line{v105TestText(t, "x <a|b>", "x <a|b>", 2, 1)}, nil},
		{"empty block", []model.Line{v105TestOpen(t, "table", 2, 1), v105TestClose(t, 3, 2)}, nil},
		{"blank rows", []model.Line{v105TestOpen(t, "table", 2, 1),
			v105TestLine(t, "", "blank", 3, 2), v105TestLine(t, " \t", "blank", 3, 2),
			v105TestText(t, "A", "A", 3, 2), v105TestClose(t, 3, 2)}, nil},
		{"closing on next line", []model.Line{v105TestOpen(t, "table", 2, 1),
			v105TestText(t, "x <a|b", "x <a|b", 3, 2), v105TestText(t, ">", ">", 3, 2), v105TestClose(t, 3, 2)}, nil},
		{"independent tables", []model.Line{v105TestOpen(t, "table", 2, 1),
			v105TestText(t, "x <a|b>", "x <a|b>", 3, 2), v105TestClose(t, 3, 2),
			v105TestOpen(t, "table", 2, 1), v105TestText(t, "<a|b>", "<a|b>", 6, 2),
			v105TestClose(t, 6, 2)}, [][5]int{{4, 1, 2, 3, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v105TestRun(t, v105TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tag := range []string{"note", "editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			v105TestRun(t, v105TestDocument(t, v105TestOpen(t, tag, 2, 1),
				v105TestText(t, "x <a|b>", "x <a|b>", 3, 2), v105TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV105Representations(t *testing.T) {
	for _, representation := range []string{"whole", "fragmented", "unparsed", "invalid", "partial"} {
		t.Run(representation, func(t *testing.T) {
			row := v105TestText(t, "x <a|b>", "x <a|b>", 3, 2)
			switch representation {
			case "fragmented":
				row.Elements = []model.Element{
					v105TestElement(t, "content", "x <a", "x <a", 1, 5),
					v105TestElement(t, "content", "b>", "b>", 6, 8),
				}
			case "unparsed", "invalid":
				row.Elements = []model.Element{{ElementType: "unparsed", Raw: "x <a|b>", Start: 1, End: 8, ErrorIDs: []string{}}}
				if representation == "invalid" {
					row.LineType = "invalid"
				}
			case "partial":
				row.Elements = []model.Element{v105TestElement(t, "content", "x", "x", 1, 2),
					{ElementType: "unparsed", Raw: "<a|b>", Start: 3, End: 8, ErrorIDs: []string{}},
				}
			}
			v105TestRun(t, v105TestDocument(t, v105TestOpen(t, "table", 2, 1), row,
				v105TestClose(t, 3, 2)), "current", [][5]int{{4, 1, 2, 3, 4}})
		})
	}
}

func TestV105EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v105TestDocument(t, v105TestOpen(t, "table", 2, 1),
				v105TestText(t, "текст <a | b>", "текст <a | b>", 3, 2), v105TestClose(t, 3, 2))
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v105TestRun(t, result, "first", [][5]int{{4, 1, 6, 7, 8}})
			v105TestRun(t, result, "second", [][5]int{{4, 1, 6, 7, 8}})
			v105TestRun(t, v105TestDocument(t, v105TestOpen(t, "table", 2, 1),
				v105TestText(t, "<a|b>", "<a|b>", 3, 2), v105TestClose(t, 3, 2)), "separate", nil)
		})
	}
}

func TestV105EmptyAndVersions(t *testing.T) {
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
					v105TestRun(t, result, "current", nil)
					return
				}
				before := v105TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v105TestRegistry(t))
				if err == nil || checks != nil || v105TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

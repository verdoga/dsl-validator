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

// v088TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V088.
func v088TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v088TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v088TestHeading(t, "", "#", "1", title),
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

// v088TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v088TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v088TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v088TestElement(t, "heading-level", marker, level, start, end),
		v088TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v088TestInvalid задаёт ошибочную строку без распознанного tag.
func v088TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v088TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v088TestLine(t, raw, "invalid", parent, depth, element)
}

// v088TestTag собирает однострочный тег с одним явно типизированным параметром.
func v088TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v088TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v088TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v088TestLine(t, raw, "tag", parent, depth, elements...)
}

// v088TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v088TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v088TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v088TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v088TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v088TestClose задаёт закрытие блока с явным родителем.
func v088TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v088TestLine(t, "}", "block-end", parent, depth, v088TestElement(t, "block-close", "}", "}", 1, 2))
}

// v088TestText сохраняет независимо заданные исходное и семантическое значения.
func v088TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v088TestLine(t, raw, "content", parent, depth,
		v088TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v088TestLine не классифицирует исходник и не вычисляет родительские связи.
func v088TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v088TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v088TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v088TestRegistry использует штатный каталог целиком.
func v088TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v088TestRun проверяет новые V088: строка, начало и исключающий конец.
func v088TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v088TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v088TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v088TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V088" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V088 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Перед разделителем --- отсутствует содержимое." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V088 metadata", i, entry, location)
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
	if err := json.Unmarshal([]byte(v088TestSnapshot(t, result)), &restored); err != nil {
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
	if v088TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v088TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v088TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV088Registration(t *testing.T) {
	definitions, err := v088TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V088" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Перед разделителем --- отсутствует содержимое." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V088 definitions = %d, want 1", count)
	}
}

// v088TestSeparator задаёт исходный разделитель без распознавания по Raw.
func v088TestSeparator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v088TestLine(t, raw, "separator", parent, depth)
}

// v088TestTitled задаёт блок с отдельным элементом названия.
func v088TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v088TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v088TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v088TestElement(t, "title", title, title, len(tag)+3, end),
		v088TestElement(t, "block-open", "{", "{", end+1, end+2))
}

func TestV088ReferenceAndExample(t *testing.T) {
	// По DSL 1.3, 1.30, 2.1.3 добавлены только @dsl-version и # Lesson.
	// HTML-оформление таблицы передаёт переводы строк и символ |, а не текст сущности.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][3]int
	}{
		{"reference", "@table {\nName | Form\n---\nA | B\n}", []model.Line{
			v088TestOpen(t, "table", 2, 1), v088TestText(t, "Name | Form", "Name | Form", 3, 2),
			v088TestSeparator(t, "---", 3, 2), v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2),
		}, nil},
		{"example", "@table {\n---\nA | B\n}", []model.Line{
			v088TestOpen(t, "table", 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{4, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v088TestDocument(t, tc.body...)
			var raw strings.Builder
			for _, line := range result.Lines[2:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", raw.String())
			}
			v088TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV088BothBlocks(t *testing.T) {
	for _, tag := range []string{"example", "table", "EXAMPLE", "TABLE"} {
		t.Run(tag, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				body []model.Line
				want [][3]int
			}{
				{"empty first part", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, [][3]int{{4, 1, 4}}},
				{"nonempty first part", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestText(t, "Before.", "Before.", 3, 2), v088TestSeparator(t, "---", 3, 2),
					v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"title is not content", []model.Line{
					v088TestTitled(t, tag, "Forms Я😀", 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, [][3]int{{4, 1, 4}}},
				{"blank rows", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestLine(t, "", "blank", 3, 2), v088TestLine(t, " \t", "blank", 3, 2),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, [][3]int{{6, 1, 4}}},
				{"no separator", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestText(t, "Read.", "Read.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"two separators", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestText(t, "Middle.", "Middle.", 3, 2),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"three separators", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestSeparator(t, "---", 3, 2),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"empty second part is separate requirement", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestText(t, "Before.", "Before.", 3, 2), v088TestSeparator(t, "---", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"both parts empty", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestClose(t, 3, 2),
				}, [][3]int{{4, 1, 4}}},
			} {
				t.Run(tc.name, func(t *testing.T) { v088TestRun(t, v088TestDocument(t, tc.body...), "current", tc.want) })
			}
		})
	}
}

func TestV088RawSeparatorsAndContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw  string
		start, end int
	}{
		{"plain", "---", 1, 4},
		{"indent and trailing whitespace", " \t---\t ", 3, 6},
		{"tab indent", "\t---", 2, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), v088TestSeparator(t, tc.raw, 3, 2),
				v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "current", [][3]int{{4, tc.start, tc.end}})
		})
	}
	t.Run("separator recovered from invalid", func(t *testing.T) {
		v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), v088TestInvalid(t, " \t--- ", 3, 2),
			v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "current", [][3]int{{4, 3, 6}})
	})
	t.Run("unparsed second separator still disqualifies block", func(t *testing.T) {
		v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), v088TestSeparator(t, "---", 3, 2),
			v088TestText(t, "Middle.", "Middle.", 3, 2), v088TestInvalid(t, "---", 3, 2),
			v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "current", nil)
	})
	for _, tc := range []struct{ raw, value string }{
		{`\---`, "---"}, {"--", "--"}, {"----", "----"}, {"Text ---", "Text ---"},
		{"--- Text", "--- Text"}, {"\u00a0---", "\u00a0---"}, {"---\u00a0", "---\u00a0"},
		{"\u00a0", "\u00a0"}, {"Я😀", "Я😀"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			// До настоящего разделителя эта строка является обычным содержимым.
			v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), v088TestText(t, tc.raw, tc.value, 3, 2),
				v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "with separator", nil)
			v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), v088TestText(t, tc.raw, tc.value, 3, 2),
				v088TestClose(t, 3, 2)), "without separator", nil)
		})
	}
	for _, tc := range []struct{ name, before, after string }{
		{"two empty cells", "|", "A | B"}, {"three empty cells", "||", "A | B | C"},
		{"spaces around empty cells", " | \t | ", "A | B | C"}, {"single column", "Name", "A"},
		{"escaped separator as data", `\---`, "A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.before
			if tc.name == "escaped separator as data" {
				value = "---"
			}
			v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "table", 2, 1), v088TestText(t, tc.before, value, 3, 2),
				v088TestSeparator(t, "---", 3, 2), v088TestText(t, tc.after, tc.after, 3, 2), v088TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV088ImmediateRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"nested target", []model.Line{
			v088TestOpen(t, "note", 2, 1), v088TestOpen(t, "table", 3, 2), v088TestSeparator(t, "---", 4, 3),
			v088TestText(t, "A | B", "A | B", 4, 3), v088TestClose(t, 4, 3), v088TestClose(t, 3, 2),
		}, [][3]int{{5, 1, 4}}},
		{"nested content does not fill outer", []model.Line{
			v088TestOpen(t, "table", 2, 1), v088TestOpen(t, "example", 3, 2), v088TestText(t, "Read.", "Read.", 4, 3),
			v088TestClose(t, 4, 3), v088TestSeparator(t, "---", 3, 2), v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{7, 1, 4}}},
		{"nested separator does not affect outer count", []model.Line{
			v088TestOpen(t, "table", 2, 1), v088TestOpen(t, "example", 3, 2), v088TestText(t, "Before.", "Before.", 4, 3),
			v088TestSeparator(t, "---", 4, 3), v088TestText(t, "After.", "After.", 4, 3), v088TestClose(t, 4, 3),
			v088TestSeparator(t, "---", 3, 2), v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{9, 1, 4}}},
		{"nested violations are independent", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestOpen(t, "table", 3, 2), v088TestSeparator(t, "---", 4, 3),
			v088TestText(t, "A | B", "A | B", 4, 3), v088TestClose(t, 4, 3), v088TestSeparator(t, "---", 3, 2),
			v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{5, 1, 4}, {8, 1, 4}}},
		{"filled outer does not fill inner", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestText(t, "Before.", "Before.", 3, 2), v088TestOpen(t, "table", 3, 2),
			v088TestSeparator(t, "---", 5, 3), v088TestText(t, "A | B", "A | B", 5, 3), v088TestClose(t, 5, 3),
			v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{6, 1, 4}}},
		{"two direct separators do not suppress inner", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestOpen(t, "table", 3, 2),
			v088TestSeparator(t, "---", 5, 3), v088TestText(t, "A | B", "A | B", 5, 3), v088TestClose(t, 5, 3),
			v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{6, 1, 4}}},
		{"only nested separator", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestOpen(t, "table", 3, 2), v088TestSeparator(t, "---", 4, 3),
			v088TestText(t, "A | B", "A | B", 4, 3), v088TestClose(t, 4, 3), v088TestClose(t, 3, 2),
		}, [][3]int{{5, 1, 4}}},
		{"known inline declaration is not content", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestTag(t, "hint", "content", "Help.", 3, 2),
			v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{5, 1, 4}}},
		{"known inline declaration in table", []model.Line{
			v088TestOpen(t, "table", 2, 1), v088TestTag(t, "hint", "content", "Help.", 3, 2),
			v088TestSeparator(t, "---", 3, 2), v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2),
		}, [][3]int{{5, 1, 4}}},
		{"direct content after nested block", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestOpen(t, "note", 3, 2), v088TestText(t, "Nested.", "Nested.", 4, 3),
			v088TestClose(t, 4, 3), v088TestText(t, "Before.", "Before.", 3, 2), v088TestSeparator(t, "---", 3, 2),
			v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
		}, nil},
		{"separate blocks", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
			v088TestOpen(t, "table", 2, 1), v088TestSeparator(t, "---", 7, 2), v088TestText(t, "A | B", "A | B", 7, 2), v088TestClose(t, 7, 2),
		}, [][3]int{{4, 1, 4}, {8, 1, 4}}},
		{"material in preceding block does not fill next", []model.Line{
			v088TestOpen(t, "example", 2, 1), v088TestText(t, "Read.", "Read.", 3, 2), v088TestClose(t, 3, 2),
			v088TestOpen(t, "table", 2, 1), v088TestSeparator(t, "---", 6, 2), v088TestText(t, "A | B", "A | B", 6, 2), v088TestClose(t, 6, 2),
		}, [][3]int{{7, 1, 4}}},
		{"branch context", []model.Line{
			v088TestOpen(t, "variants", 2, 1), v088TestTag(t, "variant", "name", "First", 3, 2),
			v088TestOpen(t, "table", 4, 3), v088TestSeparator(t, "---", 5, 4), v088TestText(t, "A | B", "A | B", 5, 4), v088TestClose(t, 5, 4),
			v088TestTag(t, "variant", "name", "Second", 3, 2), v088TestText(t, "Read.", "Read.", 9, 3), v088TestClose(t, 3, 2),
		}, [][3]int{{6, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v088TestRun(t, v088TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV088Uncertainty(t *testing.T) {
	for _, tag := range []string{"example", "table"} {
		t.Run(tag, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				body []model.Line
				want [][3]int
			}{
				{"unknown before separator", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestInvalid(t, "@unknown", 3, 2), v088TestSeparator(t, "---", 3, 2),
					v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"unknown then blank", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestInvalid(t, "@unknown", 3, 2), v088TestLine(t, "", "blank", 3, 2),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"unknown then content", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestInvalid(t, "@unknown", 3, 2), v088TestText(t, "Before.", "Before.", 3, 2),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"content then unknown", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestText(t, "Before.", "Before.", 3, 2), v088TestInvalid(t, "@unknown", 3, 2),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, nil},
				{"unknown after separator", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestSeparator(t, "---", 3, 2), v088TestInvalid(t, "@unknown", 3, 2), v088TestClose(t, 3, 2),
				}, [][3]int{{4, 1, 4}}},
				{"unknown descendant does not replace direct row", []model.Line{
					v088TestOpen(t, tag, 2, 1), v088TestOpen(t, "note", 3, 2), v088TestInvalid(t, "@unknown", 4, 3), v088TestClose(t, 4, 3),
					v088TestSeparator(t, "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
				}, [][3]int{{7, 1, 4}}},
			} {
				t.Run(tc.name, func(t *testing.T) { v088TestRun(t, v088TestDocument(t, tc.body...), "current", tc.want) })
			}
			for _, parsed := range []bool{false, true} {
				t.Run("content with unparsed parsed prefix="+strconv.FormatBool(parsed), func(t *testing.T) {
					// Два подчёркивания — ошибочная длина плейсхолдера по DSL 4.3.2.
					raw := "__"
					broken := v088TestElement(t, "unparsed", "__", "", 1, 3)
					broken.Value = nil
					elements := []model.Element{broken}
					if parsed {
						raw = "Read. __"
						broken.Start, broken.End = 7, 9
						elements = []model.Element{v088TestElement(t, "content", "Read.", "Read.", 1, 6), broken}
					}
					line := v088TestLine(t, raw, "content", 3, 2, elements...)
					v088TestRun(t, v088TestDocument(t, v088TestOpen(t, tag, 2, 1), line, v088TestSeparator(t, "---", 3, 2),
						v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "current", nil)
				})
			}
		})
	}
	t.Run("recognized erroneous declaration remains a declaration", func(t *testing.T) {
		line := v088TestTag(t, "hint", "content", "Help.", 3, 2)
		line.LineType = "invalid"
		line.Raw += " __"
		broken := v088TestElement(t, "unparsed", "__", "", 13, 15)
		broken.Value = nil
		line.Elements = append(line.Elements, broken)
		v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), line, v088TestSeparator(t, "---", 3, 2),
			v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "current", [][3]int{{5, 1, 4}})
	})
	t.Run("unrecognized nested block", func(t *testing.T) {
		broken := v088TestElement(t, "unparsed", "@unknown", "", 1, 9)
		broken.Value = nil
		opening := v088TestLine(t, "@unknown {", "block-start", 3, 2, broken, v088TestElement(t, "block-open", "{", "{", 10, 11))
		v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "example", 2, 1), opening,
			v088TestText(t, "Nested.", "Nested.", 4, 3), v088TestClose(t, 4, 3), v088TestSeparator(t, "---", 3, 2),
			v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV088ExcludedContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"inline example", []model.Line{v088TestTag(t, "example", "content", "---", 2, 1)}},
		{"wordlist treats dashes as an item", []model.Line{
			v088TestOpen(t, "wordlist", 2, 1), v088TestText(t, "---", "---", 3, 2), v088TestText(t, "word", "word", 3, 2), v088TestClose(t, 3, 2),
		}},
		{"editor literal block text", []model.Line{
			v088TestOpen(t, "editor", 2, 1), v088TestText(t, "@table {", "@table {", 3, 2),
			v088TestText(t, "---", "---", 3, 2), v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2),
		}},
		{"multifill literal block text", []model.Line{
			v088TestOpen(t, "multifill", 2, 1), v088TestText(t, "@example {", "@example {", 3, 2),
			v088TestText(t, "---", "---", 3, 2), v088TestText(t, "After.", "After.", 3, 2), v088TestClose(t, 3, 2),
		}},
		{"ordinary text", []model.Line{v088TestText(t, "Read.", "Read.", 2, 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v088TestRun(t, v088TestDocument(t, tc.body...), "current", nil) })
	}
}

func TestV088EndingsAndHistory(t *testing.T) {
	for _, name := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(name, func(t *testing.T) {
			result := v088TestDocument(t, v088TestOpen(t, "table", 2, 1), v088TestSeparator(t, "---", 3, 2),
				v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2))
			for i := range result.Lines {
				switch name {
				case "LF":
					result.Lines[i].LineEnding = "\n"
				case "CRLF":
					result.Lines[i].LineEnding = "\r\n"
				case "mixed":
					if i%2 == 0 {
						result.Lines[i].LineEnding = "\r\n"
					} else {
						result.Lines[i].LineEnding = "\n"
					}
				}
			}
			v088TestRun(t, result, "first", [][3]int{{4, 1, 4}})
			v088TestRun(t, result, "second", [][3]int{{4, 1, 4}})
			v088TestRun(t, v088TestDocument(t, v088TestOpen(t, "table", 2, 1),
				v088TestText(t, "Name | Form", "Name | Form", 3, 2), v088TestSeparator(t, "---", 3, 2),
				v088TestText(t, "A | B", "A | B", 3, 2), v088TestClose(t, 3, 2)), "separate", nil)
		})
	}
	t.Run("last physical object is separator", func(t *testing.T) {
		// Ошибка незакрытого блока сохранена отдельно от ожидаемой V088.
		result := v088TestDocument(t, v088TestOpen(t, "table", 2, 1), v088TestSeparator(t, "---", 3, 2))
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
			ID: "unclosed", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line",
			Message: "Блок не закрыт до конца файла.", RelatedLocations: []model.Location{},
			Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 9}},
		})
		result.Document.HasErrors = true
		result.Lines[2].HasErrors = true
		v088TestRun(t, result, "current", [][3]int{{4, 1, 4}})
	})
	t.Run("no body", func(t *testing.T) { v088TestRun(t, v088TestDocument(t), "current", nil) })
}

func TestV088EmptyAndVersions(t *testing.T) {
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
					v088TestRun(t, result, "current", nil)
					return
				}
				before := v088TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v088TestRegistry(t))
				if err == nil || checks != nil || v088TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

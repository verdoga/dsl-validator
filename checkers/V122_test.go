package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v122Expected задаёт независимые ожидания: физическую строку, колонки [start,end) и конкретизацию.
type v122Expected struct {
	line, start, end int
	detail           string
}

func TestV122Registration(t *testing.T) {
	definitions, err := v122Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V122" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Обнаружена повреждённая разметка." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V122 count = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			result := v122Document(t)
			result.Document.DSLVersion = &version
			before := v122Snapshot(t, result)
			if checks, err := validator.Prepare(result, v122Registry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v122Snapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV122LinesAndElements(t *testing.T) {
	// Во всех случаях версия находится на строке 1. Эталон и пример сохранены
	// буквально; только обязательная версия добавлена перед ними.
	// Разметка элементов задана явно: V122 проверяет классификацию JSON,
	// а не пытается установить причину, по которой парсер оставил unparsed.
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v122Expected
	}{
		{"reference", []model.Line{v122Heading(t)}, nil},
		{"example", []model.Line{v122Line(t, "#Заголовок", "invalid", 0,
			v122Element(t, "unparsed", "#Заголовок", "", 1))},
			[]v122Expected{{2, 1, 11, "Повреждена строка 2"}}},
		{"invalid takes precedence over all elements", []model.Line{v122Line(t, "#Заголовок", "invalid", 0,
			v122Element(t, "unparsed", "#", "", 1), v122Element(t, "unparsed", "Заголовок", "", 2))},
			[]v122Expected{{2, 1, 11, "Повреждена строка 2"}}},
		{"Unicode element", []model.Line{v122Heading(t), v122Line(t, "@note Я😀", "tag", 2,
			v122Element(t, "tag", "@note", "note", 1), v122Element(t, "unparsed", "Я😀", "", 7))},
			[]v122Expected{{3, 7, 9, "Повреждён элемент в строке 3"}}},
		{"equal text at separate positions", []model.Line{v122Heading(t), v122Line(t, "@note X X", "tag", 2,
			v122Element(t, "tag", "@note", "note", 1), v122Element(t, "unparsed", "X", "", 7),
			v122Element(t, "unparsed", "X", "", 9))},
			[]v122Expected{{3, 7, 8, "Повреждён элемент в строке 3"}, {3, 9, 10, "Повреждён элемент в строке 3"}}},
		{"whole raw includes indentation and emoji", []model.Line{v122Line(t, "  #Заголовок😀", "invalid", 0,
			v122Element(t, "unparsed", "#Заголовок😀", "", 3))},
			[]v122Expected{{2, 1, 14, "Повреждена строка 2"}}},
		{"tabs and trailing whitespace", []model.Line{v122Line(t, "\t#Заголовок \t", "invalid", 0,
			v122Element(t, "unparsed", "#Заголовок", "", 2))},
			[]v122Expected{{2, 1, 14, "Повреждена строка 2"}}},
		{"combining code point", []model.Line{v122Line(t, "#e\u0301", "invalid", 0,
			v122Element(t, "unparsed", "#e\u0301", "", 1))},
			[]v122Expected{{2, 1, 4, "Повреждена строка 2"}}},
		{"multiple invalid lines", []model.Line{v122Heading(t),
			v122Line(t, "#Заголовок", "invalid", 2, v122Element(t, "unparsed", "#Заголовок", "", 1)),
			v122Line(t, "#Заголовок", "invalid", 2, v122Element(t, "unparsed", "#Заголовок", "", 1))},
			[]v122Expected{{3, 1, 11, "Повреждена строка 3"}, {4, 1, 11, "Повреждена строка 4"}}},
		{"ordinary elements and blank line", []model.Line{v122Heading(t),
			v122Line(t, "", "blank", 2), v122Line(t, "@note Read.", "tag", 2,
				v122Element(t, "tag", "@note", "note", 1), v122Element(t, "content", "Read.", "Read.", 7))}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]model.Line{v122Version(t)}, tc.body...)
			v122Run(t, v122Document(t, rows...), "current", tc.want)
		})
	}
}

func TestV122NestedContent(t *testing.T) {
	for _, block := range []string{"editor", "multifill", "note"} {
		for _, tc := range []struct {
			name string
			row  model.Line
			want []v122Expected
		}{
			{"invalid", v122Line(t, "#Заголовок", "invalid", 3,
				v122Element(t, "unparsed", "#Заголовок", "", 1)), []v122Expected{{4, 1, 11, "Повреждена строка 4"}}},
			{"unparsed", v122Line(t, "X X", "content", 3,
				v122Element(t, "unparsed", "X", "", 1), v122Element(t, "unparsed", "X", "", 3)),
				[]v122Expected{{4, 1, 2, "Повреждён элемент в строке 4"}, {4, 3, 4, "Повреждён элемент в строке 4"}}},
			{"literal", v122Line(t, "#Заголовок", "content", 3,
				v122Element(t, "content", "#Заголовок", "#Заголовок", 1)), nil},
		} {
			t.Run(block+"/"+tc.name, func(t *testing.T) {
				// Даже в литеральном контексте V122 доверяет типам входной модели.
				result := v122Document(t, v122Version(t), v122Heading(t),
					v122Line(t, "@"+block+" {", "block-start", 2,
						v122Element(t, "tag", "@"+block, block, 1), v122Element(t, "block-open", "{", "{", len(block)+3)),
					tc.row, v122Line(t, "}", "block-end", 3, v122Element(t, "block-close", "}", "}", 1)))
				v122Run(t, result, "current", tc.want)
			})
		}
	}
}

func TestV122BoundariesAndHistory(t *testing.T) {
	t.Run("empty input", func(t *testing.T) { v122Run(t, v122Document(t), "current", nil) })
	t.Run("first physical line", func(t *testing.T) {
		// Ошибочная строка перед техническим тегом сохраняется парсером на позиции 1.
		result := v122Document(t, v122Line(t, "#Заголовок", "invalid", 0,
			v122Element(t, "unparsed", "#Заголовок", "", 1)), v122Version(t), v122Heading(t))
		v122Run(t, result, "current", []v122Expected{{1, 1, 11, "Повреждена строка 1"}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", eol), func(t *testing.T) {
			result := v122Document(t, v122Version(t), v122Line(t, "#Заголовок", "invalid", 0,
				v122Element(t, "unparsed", "#Заголовок", "", 1)))
			result.Lines[1].LineEnding = eol
			if eol != "" {
				result.Lines[0].LineEnding = eol
			}
			want := []v122Expected{{2, 1, 11, "Повреждена строка 2"}}
			v122Run(t, result, "first", want)
			v122Run(t, v122Document(t, v122Version(t), v122Heading(t)), "independent", nil)
			v122Run(t, result, "second", want)
		})
	}
}

// v122Element создаёт элемент модели; диапазоны ожиданий задаются отдельно.
func v122Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v122Line задаёт тип и родителя без повторного разбора исходного текста.
func v122Line(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v122Version задаёт технический тег версии для контекста примеров.
func v122Version(t *testing.T) model.Line {
	t.Helper()
	return v122Line(t, "@dsl-version 1.2", "tag", 0,
		v122Element(t, "tag", "@dsl-version", "dsl-version", 1), v122Element(t, "version", "1.2", "1.2", 14))
}

// v122Heading сохраняет эталон из таблицы буквально.
func v122Heading(t *testing.T) model.Line {
	t.Helper()
	return v122Line(t, "# Заголовок", "heading", 0,
		v122Element(t, "heading-level", "#", "1", 1), v122Element(t, "title", "Заголовок", "Заголовок", 3))
}

// v122Document оформляет заданную модель и обязательные исходные ссылки unparsed.
// Причины повреждения не восстанавливаются: классификация передана тестом явно.
func v122Document(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	version, count := "1.2", len(rows)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines: append([]model.Line{}, rows...), Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			if *line.ParentLine < 1 || *line.ParentLine >= line.Number {
				t.Fatal("invalid fixture parent")
			}
			line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
		}
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", line.Number, j)
			element.ErrorIDs = []string{id}
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неразобранный фрагмент.",
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}}, RelatedLocations: []model.Location{}})
			line.HasErrors, result.Document.HasErrors = true, true
		}
	}
	return result
}

// v122Registry создаёт реестр из полного штатного каталога.
func v122Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v122Snapshot сохраняет модель для проверки неизменности истории и исходных данных.
func v122Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v122Run запускает штатный конвейер и проверяет только новые срабатывания V122.
func v122Run(t *testing.T, result *model.Result, processing string, want []v122Expected) {
	t.Helper()
	before := v122Snapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v122Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v122Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processing, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processing); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V122" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V122 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processing ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "Обнаружена повреждённая разметка.\nконкретизация: "+expected.detail {
			t.Fatalf("V122 occurrence = %+v, want %+v", entry, expected)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line scope created element link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("output integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v122Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Только производные флаги, новые ссылки и записи исключаются из сравнения.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("old element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v122Snapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

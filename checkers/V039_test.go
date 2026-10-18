package checkers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v039Expected задаёт независимые ожидаемые диапазоны и имя метатега.
type v039Expected struct {
	line, start, end                         int
	previousLine, previousStart, previousEnd int
	name                                     string
}

func v039Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func v039UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

func v039ModelLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v039ModelElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

func v039Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v039ModelLine(t, raw, "content", parent, depth,
		v039ModelElement(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

func v039Block(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v039ModelLine(t, tag+" {", "block-start", parent, depth,
		v039ModelElement(t, "tag", tag, name, 1, open-1), v039ModelElement(t, "block-open", "{", "{", open, open+1))
}

func v039Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v039ModelLine(t, "}", "block-end", parent, depth, v039ModelElement(t, "block-close", "}", "}", 1, 2))
}

func v039Heading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v039ModelLine(t, marker+" "+title, "heading", 0, 0,
		v039ModelElement(t, "heading-level", marker, fmt.Sprint(level), 1, level+1),
		v039ModelElement(t, "title", title, title, level+2, level+2+utf8.RuneCountInString(title)))
}

func v039ParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	line.HasErrors = true
	result.Document.HasErrors = true
	element.ErrorIDs = append(element.ErrorIDs, id)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
		DiagnosticScope: "element", Message: message,
		Location: &model.Location{
			Start: model.Position{Line: number, Column: element.Start},
			End:   model.Position{Line: number, Column: element.End},
		}, RelatedLocations: []model.Location{},
	})
}

func v039Run(t *testing.T, result *model.Result, processingID string, want []v039Expected) {
	t.Helper()
	before := v039Snapshot(t, result)
	expected := v039Clone(t, result)
	diagnosticCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v039Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v039Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	expected.Processing = append(expected.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	otherNewIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.Source == processingID && entry.DiagnosticCode != "V039" {
			otherNewIDs[entry.ID] = true
		}
		if i >= diagnosticCount && entry.DiagnosticCode == "V039" && entry.Source != processingID {
			t.Fatalf("new V039 has unexpected source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V039" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V039 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: w.line, Column: w.start}, End: model.Position{Line: w.line, Column: w.end}}
		related := model.Location{Start: model.Position{Line: w.previousLine, Column: w.previousStart}, End: model.Position{Line: w.previousLine, Column: w.previousEnd}}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("occurrence %d related locations = %+v, want %+v", i, entry.RelatedLocations, related)
		}
		if entry.DiagnosticCode != "V039" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "element" || entry.Fatal || entry.Message != "Метатег объявлен повторно.\nконкретизация: Метатег: @"+w.name {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
		expected.Document.HasErrors = true
		line := &expected.Lines[w.line-1]
		line.HasErrors = true
		found := false
		for index := range line.Elements {
			element := &line.Elements[index]
			if element.ElementType == "tag" && element.Start == w.start && element.End == w.end {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				found = true
			}
		}
		if !found {
			t.Fatal("expected occurrence does not point to an input tag")
		}
	}
	// Исключаем только новые обратные ссылки других диагностик этого запуска.
	// Ссылки V039 и прежних запусков и остальная модель по-прежнему
	// сравниваются с независимыми ожиданиями. Исходный результат не изменяем.
	// Общие флаги проверяются штатным контролем целостности с учётом
	// всех диагностик. В снимке оставляем прежние данные и эффекты V039.
	if _, err := validator.Prepare(result, v039Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	actual := v039Clone(t, result)
	actual.Document.HasErrors = expected.Document.HasErrors
	for i := range actual.Lines {
		actual.Lines[i].HasErrors = expected.Lines[i].HasErrors
	}
	for i := range actual.Lines {
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			kept := element.ErrorIDs[:0]
			for _, id := range element.ErrorIDs {
				if !otherNewIDs[id] {
					kept = append(kept, id)
				}
			}
			element.ErrorIDs = kept
		}
	}
	expected.Diagnostics = append(expected.Diagnostics, result.Diagnostics[diagnosticCount:]...)
	if v039Snapshot(t, actual) != v039Snapshot(t, expected) {
		t.Fatal("unexpected document, line, flag, element link, processing or previous history changes")
	}
}

func v039Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v039Clone(t *testing.T, result *model.Result) *model.Result {
	t.Helper()
	var cloned model.Result
	if err := json.Unmarshal([]byte(v039Snapshot(t, result)), &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

// v039Document дополняет фрагмент обязательной версией и заголовком.
// Метаданные остаются перед заголовком; родители тела задаются явно.
func v039Document(t *testing.T, metadata, body []model.Line) *model.Result {
	t.Helper()
	result := v039UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{v039ModelLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v039ModelElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
		v039ModelElement(t, "version", "1.2", "1.2", 14, 17))}
	result.Lines = append(result.Lines, metadata...)
	result.Lines = append(result.Lines, v039Heading(t, 1, "Unit 1"))
	result.Lines = append(result.Lines, body...)
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count, title, encoding := len(result.Lines), "Unit 1", "UTF-8"
	result.Document.LineCount = &count
	result.Document.Metadata.Title = &title
	result.Document.Encoding = &encoding
	return result
}

// v039Meta задаёт распознанный тег и параметр без разбора исходной строки.
func v039Meta(t *testing.T, name, value string) model.Line {
	t.Helper()
	tag := "@" + name
	end := utf8.RuneCountInString(tag) + 1
	elements := []model.Element{v039ModelElement(t, "tag", tag, name, 1, end)}
	raw := tag
	if value != "" {
		kind := model.ElementType("name")
		switch name {
		case "document-id":
			kind = "identifier"
		case "order":
			kind = "number"
		case "resource-dir":
			kind = "resource-path"
		case "dsl-version":
			kind = "version"
		}
		raw += " " + value
		elements = append(elements, v039ModelElement(t, kind, value, value, end+1, end+1+utf8.RuneCountInString(value)))
	}
	return v039ModelLine(t, raw, "tag", 0, 0, elements...)
}

func TestV039Registration(t *testing.T) {
	registry := v039Registry(t)
	for _, version := range []string{"1.1", "1.2", "1.3"} {
		definitions, err := registry.ForVersion(dslversions.Version(version))
		if version != "1.2" {
			if err == nil || definitions != nil {
				t.Fatalf("unsupported version %s: definitions=%v err=%v", version, definitions, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range definitions {
			if d.Code == "V039" {
				count++
				if d.Message != "Метатег объявлен повторно." || d.Severity != "error" || d.Scope != "element" || d.Check == nil || d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
					t.Errorf("definition: %+v", d)
				}
			}
		}
		want := 0
		if version == "1.2" {
			want = 1
		}
		if count != want {
			t.Errorf("version %s: count %d, want %d", version, count, want)
		}
	}
}

func TestV039ReferenceAndExample(t *testing.T) {
	// Значимое содержимое обоих фрагментов сохранено; добавлены только
	// обязательная версия перед ними и заголовок после них (DSL 1.31.2).
	t.Run("reference", func(t *testing.T) {
		result := v039Document(t, []model.Line{v039Meta(t, "section", "Studentbook"), v039Meta(t, "order", "20")}, nil)
		section, order := "Studentbook", "20"
		result.Document.Metadata.Section = &section
		result.Document.Metadata.Order = &order
		v039Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v039Run(t, v039Document(t, []model.Line{v039Meta(t, "section", "Studentbook"), v039Meta(t, "section", "Workbook")}, nil), "current", []v039Expected{{3, 1, 9, 2, 1, 9, "section"}})
	})
}

func TestV039RepeatedNamesAndValues(t *testing.T) {
	for _, tc := range []struct {
		name          string
		end           int
		first, second string
	}{
		{"document-id", 13, "abc", "abc"}, {"section", 9, "Studentbook", "Workbook"}, {"order", 7, "20", "21"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v039Document(t, []model.Line{v039Meta(t, tc.name, tc.first), v039Meta(t, tc.name, tc.second), v039Meta(t, tc.name, tc.first)}, nil)
			v039Run(t, result, "current", []v039Expected{{3, 1, tc.end, 2, 1, tc.end, tc.name}, {4, 1, tc.end, 2, 1, tc.end, tc.name}})
		})
		t.Run(tc.name+" missing first value", func(t *testing.T) {
			result := v039Document(t, []model.Line{v039Meta(t, tc.name, ""), v039Meta(t, tc.name, tc.second)}, nil)
			v039ParserError(t, result, 2, 0, "Отсутствует значение метатега.")
			v039Run(t, result, "current", []v039Expected{{3, 1, tc.end, 2, 1, tc.end, tc.name}})
		})
		t.Run(tc.name+" missing repeated value", func(t *testing.T) {
			result := v039Document(t, []model.Line{v039Meta(t, tc.name, tc.first), v039Meta(t, tc.name, "")}, nil)
			v039ParserError(t, result, 3, 0, "Отсутствует значение метатега.")
			v039Run(t, result, "current", []v039Expected{{3, 1, tc.end, 2, 1, tc.end, tc.name}})
		})
	}
	t.Run("invalid number is remembered", func(t *testing.T) {
		result := v039Document(t, []model.Line{v039Meta(t, "order", "-1"), v039Meta(t, "order", "20")}, nil)
		v039ParserError(t, result, 2, 1, "Ожидалось положительное целое число.")
		v039Run(t, result, "current", []v039Expected{{3, 1, 7, 2, 1, 7, "order"}})
	})
	t.Run("interleaved names", func(t *testing.T) {
		result := v039Document(t, []model.Line{v039Meta(t, "section", "A"), v039Meta(t, "order", "1"), v039Meta(t, "document-id", "x"), v039Meta(t, "order", "2"), v039Meta(t, "section", "B"), v039Meta(t, "document-id", "y")}, nil)
		v039Run(t, result, "current", []v039Expected{{5, 1, 7, 3, 1, 7, "order"}, {6, 1, 9, 2, 1, 9, "section"}, {7, 1, 13, 4, 1, 13, "document-id"}})
	})
}

func TestV039CanonicalNameAndPositions(t *testing.T) {
	first := v039ModelLine(t, " @SECTION Имя", "tag", 0, 0, v039ModelElement(t, "tag", "@SECTION", "section", 2, 10), v039ModelElement(t, "name", "Имя", "Имя", 11, 14))
	second := v039ModelLine(t, "  @section Другое", "tag", 0, 0, v039ModelElement(t, "tag", "@section", "section", 3, 11), v039ModelElement(t, "name", "Другое", "Другое", 12, 18))
	v039Run(t, v039Document(t, []model.Line{first, second}, nil), "current", []v039Expected{{3, 3, 11, 2, 2, 10, "section"}})
}

func TestV039ErroneousAndUnparsed(t *testing.T) {
	t.Run("invalid line with tag", func(t *testing.T) {
		line := v039ModelLine(t, "@order extra", "invalid", 0, 0, v039ModelElement(t, "tag", "@order", "order", 1, 7), model.Element{ElementType: "unparsed", Raw: "extra", Start: 8, End: 13, ErrorIDs: []string{}})
		result := v039Document(t, []model.Line{line, v039Meta(t, "order", "20")}, nil)
		v039ParserError(t, result, 2, 1, "Ожидалось целое число.")
		v039Run(t, result, "current", []v039Expected{{3, 1, 7, 2, 1, 7, "order"}})
	})
	t.Run("unparsed alone is not reconstructed", func(t *testing.T) {
		line := v039ModelLine(t, "@section Workbook", "invalid", 0, 0, model.Element{ElementType: "unparsed", Raw: "@section Workbook", Start: 1, End: 18, ErrorIDs: []string{}})
		result := v039Document(t, []model.Line{line, v039Meta(t, "section", "A")}, nil)
		section := "A"
		result.Document.Metadata.Section = &section
		v039ParserError(t, result, 2, 0, "Не удалось разобрать строку.")
		v039Run(t, result, "current", nil)
	})
}

func TestV039WholeDocument(t *testing.T) {
	late := v039Meta(t, "section", "B")
	late.ParentLine = new(int)
	*late.ParentLine = 3
	late.NestingLevel = 1
	nested := v039Meta(t, "section", "C")
	nested.ParentLine = new(int)
	*nested.ParentLine = 6
	nested.NestingLevel = 2
	result := v039Document(t, []model.Line{v039Meta(t, "section", "A")}, []model.Line{
		late, v039Heading(t, 3, "Step"), v039Block(t, "text", 5, 1), nested, v039Close(t, 6, 2),
	})
	v039ParserError(t, result, 4, 0, "Метатег после заголовка.")
	v039ParserError(t, result, 7, 0, "Метатег внутри блока.")
	v039Run(t, result, "current", []v039Expected{{4, 1, 9, 2, 1, 9, "section"}, {7, 1, 9, 2, 1, 9, "section"}})
}

func TestV039LiteralContent(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			result := v039Document(t, nil, []model.Line{
				v039Block(t, name, 2, 1), v039Text(t, "@section A", 3, 2), v039Text(t, "@section B", 3, 2), v039Close(t, 3, 2),
				v039Meta(t, "section", "C"), v039Meta(t, "section", "D"),
			})
			v039ParserError(t, result, 7, 0, "Метатег после заголовка.")
			v039ParserError(t, result, 8, 0, "Метатег после заголовка.")
			v039Run(t, result, "current", []v039Expected{{8, 1, 9, 7, 1, 9, "section"}})
		})
	}
}

func TestV039ExcludedTags(t *testing.T) {
	result := v039Document(t, []model.Line{v039Meta(t, "resource-dir", "assets"), v039Meta(t, "resource-dir", "more"), v039Meta(t, "dsl-version", "1.2")}, nil)
	result.Document.Metadata.ResourceDirs = []string{"assets", "more"}
	v039ParserError(t, result, 4, 0, "Повторное объявление версии.")
	v039Run(t, result, "current", nil)
}

func TestV039EmptyInput(t *testing.T) {
	v039Run(t, v039UnavailableDocument(t, "1.2"), "current", nil)
	result := v039UnavailableDocument(t, "1.2")
	zero := 0
	result.Document.LineCount = &zero
	v039Run(t, result, "current", nil)
}

func TestV039EndingsAndIndependentRuns(t *testing.T) {
	for _, eol := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF} {
		t.Run(string(eol), func(t *testing.T) {
			result := v039Document(t, []model.Line{v039Meta(t, "section", "A"), v039Meta(t, "section", "B")}, nil)
			for i := range result.Lines {
				result.Lines[i].LineEnding = eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = model.LineEndingNone
			want := []v039Expected{{3, 1, 9, 2, 1, 9, "section"}}
			v039Run(t, result, "first", want)
			v039Run(t, result, "second", want)
		})
	}
	v039Run(t, v039Document(t, nil, nil), "current", nil)
}

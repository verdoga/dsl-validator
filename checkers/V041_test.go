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

// v041Expected задаёт независимый ожидаемый диапазон тега.
type v041Expected struct{ line, start, end int }

func v041Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func v041UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

func v041ModelLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v041ModelElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

func v041Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v041ModelLine(t, raw, "content", parent, depth,
		v041ModelElement(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

func v041Block(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v041ModelLine(t, tag+" {", "block-start", parent, depth,
		v041ModelElement(t, "tag", tag, name, 1, open-1), v041ModelElement(t, "block-open", "{", "{", open, open+1))
}

func v041Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v041ModelLine(t, "}", "block-end", parent, depth, v041ModelElement(t, "block-close", "}", "}", 1, 2))
}

func v041Heading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v041ModelLine(t, marker+" "+title, "heading", 0, 0,
		v041ModelElement(t, "heading-level", marker, fmt.Sprint(level), 1, level+1),
		v041ModelElement(t, "title", title, title, level+2, level+2+utf8.RuneCountInString(title)))
}

func v041ParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v041Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v041Clone(t *testing.T, result *model.Result) *model.Result {
	t.Helper()
	var cloned model.Result
	if err := json.Unmarshal([]byte(v041Snapshot(t, result)), &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

func v041Document(t *testing.T, metadata, body []model.Line) *model.Result {
	t.Helper()
	result := v041UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{v041ModelLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v041ModelElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
		v041ModelElement(t, "version", "1.2", "1.2", 14, 17))}
	result.Lines = append(result.Lines, metadata...)
	result.Lines = append(result.Lines, v041Heading(t, 1, "Unit 1"))
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

func TestV041Registration(t *testing.T) {
	registry := v041Registry(t)
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
			if d.Code == "V041" {
				count++
				if d.Message != "У @order отсутствует значение порядка." || d.Severity != "error" || d.Scope != "element" || d.Check == nil || d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
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

// v041Order строит распознанный тег; типизированный остаток задаёт сам тест.
func v041Order(t *testing.T, raw string, elements ...model.Element) model.Line {
	t.Helper()
	return v041ModelLine(t, raw, "tag", 0, 0, append([]model.Element{
		v041ModelElement(t, "tag", "@order", "order", 1, 7)}, elements...)...)
}

func TestV041Remainders(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		extra     []model.Element
		want      []v041Expected
		order     string
	}{
		{name: "reference", raw: "@order 20", extra: []model.Element{v041ModelElement(t, "number", "20", "20", 8, 10)}, order: "20"},
		{name: "example", raw: "@order", want: []v041Expected{{2, 1, 7}}},
		{name: "one space", raw: "@order ", want: []v041Expected{{2, 1, 7}}},
		{name: "spaces", raw: "@order   ", want: []v041Expected{{2, 1, 7}}},
		{name: "tab", raw: "@order\t", want: []v041Expected{{2, 1, 7}}},
		{name: "mixed", raw: "@order \t \t", want: []v041Expected{{2, 1, 7}}},
		{name: "padded value", raw: "@order  20  ", extra: []model.Element{v041ModelElement(t, "number", "20", "20", 9, 11)}, order: "20"},
		{name: "unparsed word without number", raw: "@order abc", extra: []model.Element{{ElementType: "unparsed", Raw: "abc", Start: 8, End: 11, ErrorIDs: []string{}}}},
		{name: "negative number", raw: "@order -1", extra: []model.Element{v041ModelElement(t, "number", "-1", "-1", 8, 10)}},
		{name: "zero", raw: "@order 0", extra: []model.Element{v041ModelElement(t, "number", "0", "0", 8, 9)}},
		{name: "fraction", raw: "@order 1.5", extra: []model.Element{v041ModelElement(t, "number", "1.5", "1.5", 8, 11)}},
		{name: "nonbreaking space", raw: "@order \u00a0", extra: []model.Element{{ElementType: "unparsed", Raw: "\u00a0", Start: 8, End: 9, ErrorIDs: []string{}}}},
		{name: "em space", raw: "@order \u2003", extra: []model.Element{{ElementType: "unparsed", Raw: "\u2003", Start: 8, End: 9, ErrorIDs: []string{}}}},
		{name: "Unicode text", raw: "@order Двадцать", extra: []model.Element{{ElementType: "unparsed", Raw: "Двадцать", Start: 8, End: 16, ErrorIDs: []string{}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Эталон и пример сохранены без изменений; DSL 1.31.2 требует
			// разместить фрагмент после версии и перед заголовком.
			result := v041Document(t, []model.Line{v041Order(t, tc.raw, tc.extra...)}, nil)
			if tc.order != "" {
				result.Document.Metadata.Order = &tc.order
			}
			if len(tc.extra) > 0 && tc.order == "" {
				v041ParserError(t, result, 2, 1, "Недопустимое значение порядка.")
			}
			v041Run(t, result, "current", tc.want)
		})
	}
}

func TestV041PositionsAndErroneousLines(t *testing.T) {
	t.Run("canonical tag with indentation", func(t *testing.T) {
		line := v041ModelLine(t, "  @ORDER", "tag", 0, 0, v041ModelElement(t, "tag", "@ORDER", "order", 3, 9))
		v041Run(t, v041Document(t, []model.Line{line}, nil), "current", []v041Expected{{2, 3, 9}})
	})
	t.Run("tag with previous error", func(t *testing.T) {
		result := v041Document(t, []model.Line{v041Order(t, "@order")}, nil)
		v041ParserError(t, result, 2, 0, "Отсутствует параметр.")
		v041Run(t, result, "current", []v041Expected{{2, 1, 7}})
	})
	t.Run("invalid line with recognized tag", func(t *testing.T) {
		line := v041Order(t, "@order\t", model.Element{ElementType: "unparsed", Raw: "\t", Start: 7, End: 8, ErrorIDs: []string{}})
		line.LineType = "invalid"
		result := v041Document(t, []model.Line{line}, nil)
		// DSL 1.29: табуляция не является разделителем параметров.
		v041ParserError(t, result, 2, 1, "Недопустимый разделитель параметра.")
		v041Run(t, result, "current", []v041Expected{{2, 1, 7}})
	})
	t.Run("unparsed alone", func(t *testing.T) {
		line := v041ModelLine(t, "@order", "invalid", 0, 0, model.Element{ElementType: "unparsed", Raw: "@order", Start: 1, End: 7, ErrorIDs: []string{}})
		result := v041Document(t, []model.Line{line}, nil)
		v041ParserError(t, result, 2, 0, "Строка не распознана.")
		v041Run(t, result, "current", nil)
	})
}

func TestV041Context(t *testing.T) {
	t.Run("after heading", func(t *testing.T) {
		line := v041Order(t, "@order")
		parent := 2
		line.ParentLine = &parent
		line.NestingLevel = 1
		result := v041Document(t, nil, []model.Line{line})
		v041ParserError(t, result, 3, 0, "Метатег после заголовка.")
		v041Run(t, result, "current", []v041Expected{{3, 1, 7}})
	})
	for _, block := range []string{"text", "editor", "multifill"} {
		t.Run(block, func(t *testing.T) {
			task := v041ModelLine(t, "@task t", "tag", 2, 1, v041ModelElement(t, "tag", "@task", "task", 1, 6), v041ModelElement(t, "identifier", "t", "t", 7, 8))
			content := v041Text(t, "@order", 4, 3)
			var want []v041Expected
			if block == "text" {
				content = v041Order(t, "@order")
				parent := 4
				content.ParentLine = &parent
				content.NestingLevel = 3
				want = []v041Expected{{5, 1, 7}}
			}
			result := v041Document(t, nil, []model.Line{task, v041Block(t, block, 3, 2), content, v041Close(t, 4, 3)})
			if block == "text" {
				v041ParserError(t, result, 5, 0, "Метатег внутри блока.")
			}
			v041Run(t, result, "current", want)
		})
	}
}

func TestV041SeveralDeclarations(t *testing.T) {
	result := v041Document(t, []model.Line{v041Order(t, "@order"), v041Order(t, "@order 20", v041ModelElement(t, "number", "20", "20", 8, 10)), v041Order(t, "@order ")}, nil)
	v041Run(t, result, "first", []v041Expected{{2, 1, 7}, {4, 1, 7}})
	v041Run(t, result, "second", []v041Expected{{2, 1, 7}, {4, 1, 7}})
	v041Run(t, v041Document(t, nil, nil), "independent", nil)
}

func TestV041EmptyAndOtherTags(t *testing.T) {
	v041Run(t, v041UnavailableDocument(t, "1.2"), "unavailable", nil)
	result := v041UnavailableDocument(t, "1.2")
	zero := 0
	result.Document.LineCount = &zero
	v041Run(t, result, "empty", nil)
	v041Run(t, v041Document(t, nil, nil), "no-order", nil)
	// Отсутствующие параметры других тегов не являются предметом V041.
	line := v041ModelLine(t, "@section", "tag", 0, 0, v041ModelElement(t, "tag", "@section", "section", 1, 9))
	result = v041Document(t, []model.Line{line}, nil)
	v041ParserError(t, result, 2, 0, "Отсутствует имя секции.")
	v041Run(t, result, "other", nil)
}

func TestV041LineEndings(t *testing.T) {
	for _, eol := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF} {
		for _, lastNone := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/final-none-%t", eol, lastNone), func(t *testing.T) {
				line := v041Order(t, "@order")
				parent := 2
				line.ParentLine = &parent
				line.NestingLevel = 1
				result := v041Document(t, nil, []model.Line{line})
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if lastNone {
					result.Lines[2].LineEnding = model.LineEndingNone
				}
				v041ParserError(t, result, 3, 0, "Метатег после заголовка.")
				v041Run(t, result, "current", []v041Expected{{3, 1, 7}})
			})
		}
	}
}

// v041Run использует штатный конвейер и проверяет результаты V041 отдельно
// от других зарегистрированных диагностик, включая их обратные ссылки.
func v041Run(t *testing.T, result *model.Result, processingID string, want []v041Expected) {
	t.Helper()
	before := v041Clone(t, result)
	checks, err := validator.Prepare(result, v041Registry(t))
	if err != nil {
		t.Fatal(err)
	}
	if v041Snapshot(t, result) != v041Snapshot(t, before) {
		t.Fatal("Prepare changed input")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatal(err)
	}
	var got []model.Diagnostic
	allNewIDs := make(map[string]bool)
	currentIDs := make(map[string]bool)
	uniqueIDs := make(map[string]bool)
	for i, d := range result.Diagnostics {
		if d.ID == "" || uniqueIDs[d.ID] {
			t.Fatalf("empty or duplicate ID: %q", d.ID)
		}
		uniqueIDs[d.ID] = true
		if i < len(before.Diagnostics) {
			continue
		}
		allNewIDs[d.ID] = true
		if d.DiagnosticCode == "V041" {
			got = append(got, d)
			currentIDs[d.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V041 count=%d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		d := got[i]
		location := model.Location{Start: model.Position{Line: w.line, Column: w.start}, End: model.Position{Line: w.line, Column: w.end}}
		if d.Location == nil || *d.Location != location || len(d.RelatedLocations) != 0 {
			t.Errorf("unexpected ranges: %+v, want %+v without related", d, location)
		}
		if d.DiagnosticCode != "V041" || d.Source != processingID || d.SeverityLevel != "error" || d.DiagnosticScope != "element" || d.Fatal || d.Message != "У @order отсутствует значение порядка." {
			t.Errorf("unexpected metadata: %+v", d)
		}
		if !result.Document.HasErrors || !result.Lines[w.line-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	// Каждая новая ссылка V041 ожидается ровно на указанном тестом tag.
	for _, line := range result.Lines {
		for _, element := range line.Elements {
			var links []string
			for _, id := range element.ErrorIDs {
				if currentIDs[id] {
					links = append(links, id)
				}
			}
			var expected []string
			for i, w := range want {
				if line.Number == w.line && element.ElementType == "tag" && element.Start == w.start && element.End == w.end {
					expected = append(expected, got[i].ID)
				}
			}
			if fmt.Sprint(links) != fmt.Sprint(expected) {
				t.Errorf("line %d element %+v: V041 links %v, want %v", line.Number, element, links, expected)
			}
		}
	}
	// Новые флаги и ссылки других проверок допустимы. Сравнение остальной
	// модели подтверждает сохранность исходника, прежних ошибок и структуры.
	actual := v041Clone(t, result)
	actual.Document.HasErrors = before.Document.HasErrors
	for i := range actual.Lines {
		if before.Lines[i].HasErrors && !actual.Lines[i].HasErrors {
			t.Error("previous error flag lost")
		}
		actual.Lines[i].HasErrors = before.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			kept := []string{}
			for _, id := range element.ErrorIDs {
				if !allNewIDs[id] {
					kept = append(kept, id)
				}
			}
			element.ErrorIDs = kept
		}
	}
	before.Processing = append(before.Processing, processing)
	before.Diagnostics = append(before.Diagnostics, result.Diagnostics[len(before.Diagnostics):]...)
	if v041Snapshot(t, actual) != v041Snapshot(t, before) {
		t.Fatal("source, metadata, structure or previous history changed")
	}
}

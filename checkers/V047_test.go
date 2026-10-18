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

// v047Expected задаёт исходный диапазон независимо от реализации.
type v047Expected struct{ line, start, end int }

func v047Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func v047UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

func v047ModelLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v047ModelElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

func v047Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v047ModelLine(t, raw, "content", parent, depth,
		v047ModelElement(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

func v047Block(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v047ModelLine(t, tag+" {", "block-start", parent, depth,
		v047ModelElement(t, "tag", tag, name, 1, open-1), v047ModelElement(t, "block-open", "{", "{", open, open+1))
}

func v047Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v047ModelLine(t, "}", "block-end", parent, depth, v047ModelElement(t, "block-close", "}", "}", 1, 2))
}

func v047Heading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v047ModelLine(t, marker+" "+title, "heading", 0, 0,
		v047ModelElement(t, "heading-level", marker, fmt.Sprint(level), 1, level+1),
		v047ModelElement(t, "title", title, title, level+2, level+2+utf8.RuneCountInString(title)))
}

func v047ParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v047Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v047Clone(t *testing.T, result *model.Result) *model.Result {
	t.Helper()
	var cloned model.Result
	if err := json.Unmarshal([]byte(v047Snapshot(t, result)), &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

func v047Document(t *testing.T, metadata, body []model.Line) *model.Result {
	t.Helper()
	result := v047UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{v047ModelLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v047ModelElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
		v047ModelElement(t, "version", "1.2", "1.2", 14, 17))}
	result.Lines = append(result.Lines, metadata...)
	result.Lines = append(result.Lines, v047Heading(t, 1, "Unit 1"))
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

func TestV047Registration(t *testing.T) {
	registry := v047Registry(t)
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
			if d.Code == "V047" {
				count++
				if d.Message != "Открывающая фигурная скобка находится на отдельной строке." || d.Severity != "error" || d.Scope != "line" || d.Check == nil || d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
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

// v047Invalid сохраняет ошибочный фрагмент целиком, без разбора его скобок.
func v047Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v047ModelLine(t, raw, "invalid", parent, depth, model.Element{
		ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{},
	})
}

func TestV047StandaloneBraces(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v047Expected
	}{
		{"example", "{", []v047Expected{{3, 1, 2}}},
		{"leading spaces", "  {", []v047Expected{{3, 3, 4}}},
		{"trailing spaces", "{   ", []v047Expected{{3, 1, 2}}},
		{"tabs", "\t{\t", []v047Expected{{3, 2, 3}}},
		{"mixed indentation", " \t { \t", []v047Expected{{3, 4, 5}}},
		{"two openings", "{{", nil}, {"empty pair", "{}", nil},
		{"trailing text", "{ текст", nil}, {"leading text", "текст {", nil},
		{"closing brace", "}", nil}, {"nonbreaking prefix", "\u00a0{", nil},
		{"nonbreaking suffix", "{\u00a0", nil}, {"em space prefix", "\u2003{", nil},
		{"em space suffix", "{\u2003", nil}, {"mixed Unicode whitespace", " \t{\u00a0\t ", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Пример сохраняется дословно на строке 3; перед ним добавлены
			// обязательные версия и заголовок. Ошибочный фрагмент остаётся unparsed.
			result := v047Document(t, nil, []model.Line{v047Invalid(t, tc.raw, 2, 1)})
			v047ParserError(t, result, 3, 0, "Ошибочная запись фигурной скобки.")
			v047Run(t, result, "current", tc.want)
		})
	}
}

func TestV047Reference(t *testing.T) {
	// Эталон не изменён: добавлены только обязательные версия и заголовок.
	result := v047Document(t, nil, []model.Line{
		v047Block(t, "note", 2, 1), v047Text(t, "Read carefully.", 3, 2), v047Close(t, 3, 2),
	})
	v047Run(t, result, "current", nil)
}

func TestV047NoMatch(t *testing.T) {
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run(fmt.Sprintf("blank %q", raw), func(t *testing.T) {
			v047Run(t, v047Document(t, nil, []model.Line{v047ModelLine(t, raw, "blank", 2, 1)}), "current", nil)
		})
	}
	for _, tc := range []struct {
		name, raw, fragment, value string
		start, end                 int
	}{
		{"escaped", "\\{", "\\{", "{", 1, 3},
		{"padded escaped", " \t\\{ \t", "\\{", "{", 3, 5},
		{"escaped backslash and brace", "\\\\\\{", "\\\\\\{", "\\{", 1, 5},
		{"fullwidth brace", "｛", "｛", "｛", 1, 2},
		{"ordinary Unicode text", "Я😀", "Я😀", "Я😀", 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v047ModelLine(t, tc.raw, "content", 2, 1, v047ModelElement(t, "content", tc.fragment, tc.value, tc.start, tc.end))
			v047Run(t, v047Document(t, nil, []model.Line{line}), "current", nil)
		})
	}
}

func TestV047Contexts(t *testing.T) {
	for _, name := range []string{"note", "editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			inner := v047Text(t, "{", 3, 2)
			if name == "note" {
				inner = v047Invalid(t, "{", 3, 2)
			}
			result := v047Document(t, nil, []model.Line{v047Block(t, name, 2, 1), inner, v047Close(t, 3, 2)})
			if name == "note" {
				v047ParserError(t, result, 4, 0, "Отдельная открывающая скобка внутри блока.")
			}
			// Даже литеральное содержимое editor и multifill проверяется по Raw.
			v047Run(t, result, "current", []v047Expected{{4, 1, 2}})
		})
	}
	t.Run("nested hint", func(t *testing.T) {
		// DSL 3.8.6 и 3.8.11: инструкция со своим текстом и вложенным hint.
		result := v047Document(t, nil, []model.Line{
			v047Block(t, "instr", 2, 1), v047Text(t, "Read.", 3, 2), v047Block(t, "hint", 3, 2),
			v047Invalid(t, " \t{", 5, 3), v047Close(t, 5, 3), v047Close(t, 3, 2),
		})
		v047ParserError(t, result, 6, 0, "Отдельная открывающая скобка внутри hint.")
		v047Run(t, result, "current", []v047Expected{{6, 3, 4}})
	})
}

func TestV047IndependentLines(t *testing.T) {
	t.Run("adjacent and separated", func(t *testing.T) {
		result := v047Document(t, nil, []model.Line{
			v047Invalid(t, "{", 2, 1), v047Invalid(t, "{", 2, 1),
			v047ModelLine(t, "", "blank", 2, 1), v047Invalid(t, "  {", 2, 1),
		})
		v047ParserError(t, result, 3, 0, "Отдельная открывающая скобка.")
		v047ParserError(t, result, 4, 0, "Отдельная открывающая скобка.")
		v047ParserError(t, result, 6, 0, "Отдельная открывающая скобка.")
		want := []v047Expected{{3, 1, 2}, {4, 1, 2}, {6, 3, 4}}
		v047Run(t, result, "first", want)
		v047Run(t, result, "second", want)
		v047Run(t, v047Document(t, nil, nil), "independent", nil)
	})
	t.Run("closing line does not suppress opening", func(t *testing.T) {
		// Отдельная { не создаёт допустимый блок; последующая } тоже ошибочна.
		result := v047Document(t, nil, []model.Line{v047Invalid(t, "{", 2, 1), v047Invalid(t, "}", 2, 1)})
		v047ParserError(t, result, 3, 0, "Отдельная открывающая скобка.")
		v047ParserError(t, result, 4, 0, "Закрывающая скобка вне блока.")
		v047Run(t, result, "current", []v047Expected{{3, 1, 2}})
	})
}

func TestV047LineEndings(t *testing.T) {
	for _, eol := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF} {
		for _, lastNone := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/final-none-%t", eol, lastNone), func(t *testing.T) {
				result := v047Document(t, nil, []model.Line{v047Invalid(t, "\t{ ", 2, 1)})
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if lastNone {
					result.Lines[2].LineEnding = model.LineEndingNone
				}
				v047ParserError(t, result, 3, 0, "Отдельная открывающая скобка.")
				v047Run(t, result, "current", []v047Expected{{3, 2, 3}})
			})
		}
	}
}

func TestV047EmptyInput(t *testing.T) {
	v047Run(t, v047UnavailableDocument(t, "1.2"), "unavailable", nil)
	result := v047UnavailableDocument(t, "1.2")
	zero := 0
	result.Document.LineCount = &zero
	v047Run(t, result, "empty", nil)
}

// v047Run проверяет сообщения и флаги V047 через полный штатный конвейер.
// Сравнение всей модели сохраняет контроль исходника и прежних ошибок;
// новые обратные ссылки других диагностик исключаются только из сравнения.
func v047Run(t *testing.T, result *model.Result, processingID string, want []v047Expected) {
	t.Helper()
	expected := v047Clone(t, result)
	diagnosticCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v047Registry(t))
	if err != nil {
		t.Fatal(err)
	}
	if v047Snapshot(t, result) != v047Snapshot(t, expected) {
		t.Fatal("Prepare changed input")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	expected.Processing = append(expected.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatal(err)
	}
	var got []model.Diagnostic
	otherNewIDs := make(map[string]bool)
	uniqueIDs := make(map[string]bool)
	for i, d := range result.Diagnostics {
		if d.ID == "" || uniqueIDs[d.ID] {
			t.Fatalf("empty or duplicate ID: %q", d.ID)
		}
		uniqueIDs[d.ID] = true
		if i < diagnosticCount {
			continue
		}
		if d.DiagnosticCode == "V047" {
			got = append(got, d)
		} else {
			otherNewIDs[d.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V047 count=%d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		d := got[i]
		location := model.Location{Start: model.Position{Line: w.line, Column: w.start}, End: model.Position{Line: w.line, Column: w.end}}
		if d.Location == nil || *d.Location != location || len(d.RelatedLocations) != 0 {
			t.Errorf("unexpected ranges: %+v, want %+v without related", d, location)
		}
		if d.DiagnosticCode != "V047" || d.Source != processingID || d.SeverityLevel != "error" || d.DiagnosticScope != "line" || d.Fatal || d.Message != "Открывающая фигурная скобка находится на отдельной строке.\nконкретизация: Открывающая скобка должна завершать строку объявления блока" {
			t.Errorf("unexpected metadata: %+v", d)
		}
		expected.Document.HasErrors = true
		expected.Lines[w.line-1].HasErrors = true
	}
	// Общие флаги проверяются штатным контролем целостности с учётом
	// всех диагностик. В снимке оставляем прежние данные и эффекты V047.
	if _, err := validator.Prepare(result, v047Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	actual := v047Clone(t, result)
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
	// Ссылки V047 не добавляются: scope=line. Такая ссылка или изменение
	// прежней истории нарушат сравнение моделей; флаги проверены выше.
	expected.Diagnostics = append(expected.Diagnostics, result.Diagnostics[diagnosticCount:]...)
	if v047Snapshot(t, actual) != v047Snapshot(t, expected) {
		t.Fatal("unexpected source, flags, element links, metadata or previous history changes")
	}
}

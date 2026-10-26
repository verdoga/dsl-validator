package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV133RegistrationAndVersions(t *testing.T) {
	registry := v133TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V133" {
			continue
		}
		count++
		if definition.Message != "После идентификатора @include записано дополнительное содержимое." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V133 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v133TestDocument(t, "f", v133TestInclude(t, "f", "Extra", 3))
			result.Document.DSLVersion = &version
			before := v133TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v133TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV133ReferenceAndExample(t *testing.T) {
	// DSL 1.30: перед точными фрагментами таблицы добавлены версия, заголовок и шаг.
	for _, tc := range []struct {
		name, tail, fragment string
		want                 [][3]int
	}{
		{"reference", "", "@fragment f {\nRead this text.\n}\n@include f", nil},
		{"example", "Extra title", "@fragment f {\nRead this text.\n}\n@include f Extra title", [][3]int{{7, 12, 23}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v133TestDocument(t, "f", v133TestInclude(t, "f", tc.tail, 3))
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v133TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV133UnparsedDeclarations(t *testing.T) {
	// Ошибочные объявления сохраняются целиком в unparsed, без выдуманных блоков.
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"whole unparsed example", "@include f Extra title", [][3]int{{7, 12, 23}}},
		{"one word", "@include f Extra", [][3]int{{7, 12, 17}}},
		{"quoted tail", `@include f "Extra title"`, [][3]int{{7, 12, 25}}},
		{"empty quotes are extra", `@include f ""`, [][3]int{{7, 12, 14}}},
		{"internal tab", "@include f A\tB", [][3]int{{7, 12, 15}}},
		{"edge whitespace", "@include f \t Extra \t", [][3]int{{7, 14, 19}}},
		{"tab between ID and tail", "@include f\tExtra", [][3]int{{7, 12, 17}}},
		{"tab after tag", "@include\tf Extra", [][3]int{{7, 12, 17}}},
		{"indent and case", "\t @INCLUDE f Extra title", [][3]int{{7, 14, 25}}},
		{"Unicode tail columns", "@include f Я😀", [][3]int{{7, 12, 14}}},
		{"NBSP tail is not trimmed", "@include f \u00a0", [][3]int{{7, 12, 13}}},
		{"Unicode spaces are not trimmed", "@include f \u2003Extra\u00a0", [][3]int{{7, 12, 19}}},
		{"missing ID", "@include", nil},
		{"whitespace instead of ID", "@include \t ", nil},
		{"brace instead of ID", "@include {", nil},
		{"invalid slash in ID", "@include bad/id Extra", nil},
		{"emoji is not ID letter", "@include 😀 Extra", nil},
		{"nondecimal number", "@include ² Extra", nil},
		{"combining mark in ID", "@include e\u0301 Extra", nil},
		{"quoted ID", `@include "f" Extra`, nil},
		{"NBSP does not separate ID", "@include f\u00a0 Extra", nil},
		{"zero width space in ID", "@include f\u200b Extra", nil},
		{"agreed invalid first argument", "@include f{ Extra", nil},
		{"unparsed brace attached to ID", "@include f{", nil},
		{"longer tag name", "@includeExtra f Extra", nil},
		{"punctuation in tag name", "@include: f Extra", nil},
		{"only terminal brace", "@include f {", nil},
		{"tab before terminal brace", "@include f\t{", nil},
		{"text and terminal brace", "@include f Extra title {", [][3]int{{7, 12, 23}}},
		{"whitespace after terminal brace", "@include f Extra { \t", [][3]int{{7, 12, 17}}},
		{"nonterminal brace", "@include f { Extra", [][3]int{{7, 12, 19}}},
		{"closing brace is extra", "@include f }", [][3]int{{7, 12, 13}}},
		{"escaped terminal brace", `@include f \{`, [][3]int{{7, 12, 14}}},
		{"escaped then active brace", `@include f \{ {`, [][3]int{{7, 12, 14}}},
		{"backslash pair then active brace", `@include f \\ {`, [][3]int{{7, 12, 14}}},
		{"three backslashes escape terminal brace", `@include f \\\{`, [][3]int{{7, 12, 16}}},
		{"Unicode trailing space prevents brace removal", "@include f {\u00a0", [][3]int{{7, 12, 14}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			added := v133TestRun(t, v133TestDocument(t, "f", v133TestInvalid(t, tc.raw, 3)), "current", tc.want)
			if tc.name == "only terminal brace" {
				v133TestV109(t, added, [3]int{7, 12, 13})
			}
		})
	}
}

func TestV133IdentifierAlphabetAndParsedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		id, tail string
		want     [][3]int
	}{
		{"-", "", nil}, {"_", "", nil}, {".", "", nil}, {"9", "", nil},
		{"Я١._-", "Extra", [][3]int{{7, 16, 21}}},
		{"字", "Extra", [][3]int{{7, 12, 17}}},
		{"é", "Extra", [][3]int{{7, 12, 17}}},
	} {
		t.Run(tc.id+tc.tail, func(t *testing.T) {
			v133TestRun(t, v133TestDocument(t, tc.id, v133TestInclude(t, tc.id, tc.tail, 3)), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		name string
		row  model.Line
		want [][3]int
	}{
		{"parsed tag and identifier with unparsed tail", v133TestInclude(t, "f", "Extra title", 3), [][3]int{{7, 12, 23}}},
		{"invalid with parsed prefix", v133TestLine(t, "@include f Extra", "invalid", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "f", "f", 10),
			v133TestElement(t, "unparsed", "Extra", "", 12)), [][3]int{{7, 12, 17}}},
		{"tag without identifier", v133TestLine(t, "@include f Extra", "invalid", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "unparsed", "f Extra", "", 10)),
			[][3]int{{7, 12, 17}}},
		{"unparsed tag with identifier", v133TestLine(t, "@include f Extra", "invalid", 3,
			v133TestElement(t, "unparsed", "@include", "", 1), v133TestElement(t, "identifier", "f", "f", 10),
			v133TestElement(t, "unparsed", "Extra", "", 12)), [][3]int{{7, 12, 17}}},
		{"invalid first argument is not its parsed prefix", v133TestLine(t, "@include f/bad Extra", "invalid", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "f", "f", 10),
			v133TestElement(t, "unparsed", "/bad Extra", "", 11)), nil},
		{"brace without structural element continues ID", v133TestLine(t, "@include f{ Extra", "invalid", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "f", "f", 10),
			v133TestElement(t, "unparsed", "{ Extra", "", 11)), nil},
		{"canonical ID value preserves raw positions", v133TestLine(t, "@include F Extra", "tag", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "F", "f", 10),
			v133TestElement(t, "unparsed", "Extra", "", 12)), [][3]int{{7, 12, 17}}},
		{"trailing whitespace only", v133TestLine(t, "@include f \t ", "tag", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "f", "f", 10)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v133TestRun(t, v133TestDocument(t, "f", tc.row), "current", tc.want)
		})
	}
}

func TestV133StructuralBlockOpen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		row   model.Line
		want  [][3]int
		brace [3]int
	}{
		{"only forbidden block", v133TestOpen(t, "include", "f", 3), nil, [3]int{7, 12, 13}},
		{"adjacent recognized block", v133TestLine(t, "@include f{", "block-start", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "f", "f", 10),
			v133TestElement(t, "block-open", "{", "{", 11)), nil, [3]int{7, 11, 12}},
		{"extra text before block", v133TestLine(t, "@include f Extra title {", "block-start", 3,
			v133TestElement(t, "tag", "@include", "include", 1), v133TestElement(t, "identifier", "f", "f", 10),
			v133TestElement(t, "unparsed", "Extra title", "", 12), v133TestElement(t, "block-open", "{", "{", 24)),
			[][3]int{{7, 12, 23}}, [3]int{7, 24, 25}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v133TestDocument(t, "f", tc.row, v133TestText(t, "Body.", "Body.", 7), v133TestClose(t, 7))
			added := v133TestRun(t, result, "current", tc.want)
			v133TestV109(t, added, tc.brace)
		})
	}
}

func TestV133ContextAndIndependentDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"inside task", []model.Line{v133TestTag(t, "task", "t", "identifier", 3),
			v133TestInclude(t, "f", "Extra", 7), v133TestTag(t, "endtask", "", "", 3)}, [][3]int{{8, 12, 17}}},
		{"inside task step", []model.Line{v133TestTag(t, "task", "t", "identifier", 3),
			v133TestTag(t, "step", "Practice", "title", 7), v133TestInvalid(t, "@include f Extra", 8),
			v133TestTag(t, "endtask", "", "", 3)}, [][3]int{{9, 12, 17}}},
		// Запрещённое вложение не превращает реальное объявление в литеральный текст.
		{"inside content block", []model.Line{v133TestOpen(t, "note", "", 3),
			v133TestInclude(t, "f", "Extra", 7), v133TestClose(t, 7)}, [][3]int{{8, 12, 17}}},
		{"separate occurrences with blank line", []model.Line{v133TestInclude(t, "f", "One", 3),
			v133TestLine(t, "", "blank", 3), v133TestInclude(t, "f", "Two", 3)}, [][3]int{{7, 12, 15}, {9, 12, 15}}},
		{"inline comment and escaped text", []model.Line{
			v133TestTag(t, "editor", "@include f Extra", "content", 3),
			v133TestText(t, `\@include f Extra`, "@include f Extra", 3),
			v133TestText(t, "Text @include f Extra", "Text @include f Extra", 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v133TestRun(t, v133TestDocument(t, "f", tc.body...), "current", tc.want)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name+" inside task", func(t *testing.T) {
			result := v133TestDocument(t, "f", v133TestTag(t, "task", "t", "identifier", 3),
				v133TestOpen(t, name, "", 7), v133TestText(t, "@include f Extra", "@include f Extra", 8),
				v133TestText(t, `\@include f Extra`, `\@include f Extra`, 8), v133TestClose(t, 8),
				v133TestInclude(t, "f", "Extra", 7), v133TestTag(t, "endtask", "", "", 3))
			v133TestRun(t, result, "current", [][3]int{{12, 12, 17}})
		})
	}
}

func TestV133LineEndingsEmptyAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+" final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v133TestDocument(t, "f", v133TestInclude(t, "f", "Я😀", 3))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v133TestRun(t, result, "first", [][3]int{{7, 12, 14}})
				v133TestRun(t, result, "second", [][3]int{{7, 12, 14}})
			})
		}
	}
	t.Run("empty document", func(t *testing.T) {
		version, count := "1.2", 0
		result := &model.Result{Format: model.FormatVersion1,
			Document: model.Document{DSLVersion: &version, LineCount: &count,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
			Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
			Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{}}
		v133TestRun(t, result, "current", nil)
	})
}

// v133TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v133TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v133TestLine задаёт тип и родителя явно, без разбора DSL.
func v133TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v133TestTag задаёт известное имя и явный тип аргумента.
func v133TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v133TestElement(t, "tag", raw, name, 1)}
	if argument != "" {
		elements = append(elements, v133TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v133TestLine(t, raw, "tag", parent, elements...)
}

// v133TestInclude сохраняет ID и отдельный ошибочный хвост без его интерпретации.
func v133TestInclude(t *testing.T, id, tail string, parent int) model.Line {
	t.Helper()
	line := v133TestTag(t, "include", id, "identifier", parent)
	if tail != "" {
		line.Elements = append(line.Elements, v133TestElement(t, "unparsed", tail, "", utf8.RuneCountInString(line.Raw)+2))
		line.Raw += " " + tail
	}
	return line
}

// v133TestOpen задаёт блок; непустой аргумент здесь является ID fragment.
func v133TestOpen(t *testing.T, name, id string, parent int) model.Line {
	t.Helper()
	line := v133TestTag(t, name, id, "identifier", parent)
	line.Elements = append(line.Elements, v133TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v133TestClose закрывает явно указанный блок.
func v133TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v133TestLine(t, "}", "block-end", parent, v133TestElement(t, "block-close", "}", "}", 1))
}

// v133TestText сохраняет исходное и семантическое содержимое независимо.
func v133TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v133TestLine(t, raw, "content", parent, v133TestElement(t, "content", raw, value, 1))
}

// v133TestInvalid сохраняет полностью неразобранную строку.
func v133TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v133TestLine(t, raw, "invalid", parent, v133TestElement(t, "unparsed", raw, "", 1))
}

// v133TestDocument добавляет обязательный контекст и определение ID на строках 4–6.
// Проверяемые строки начинаются с седьмой; родители заданы вызывающим тестом.
func v133TestDocument(t *testing.T, id string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v133TestTag(t, "dsl-version", "1.2", "version", 0),
		v133TestLine(t, "# Lesson", "heading", 0, v133TestElement(t, "heading-level", "#", "1", 1),
			v133TestElement(t, "title", "Lesson", "Lesson", 3)),
		v133TestLine(t, "### Practice", "heading", 0, v133TestElement(t, "heading-level", "###", "3", 1),
			v133TestElement(t, "title", "Practice", "Practice", 5)),
		v133TestOpen(t, "fragment", id, 3), v133TestText(t, "Read this text.", "Read this text.", 4), v133TestClose(t, 4),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			if line.Elements[j].ElementType == "unparsed" {
				v133TestParserError(t, result, line.Number, j, "Ошибочный исходный фрагмент.")
			}
		}
	}
	return result
}

// v133TestParserError добавляет согласованную исходную ошибку и обратную ссылку.
// Код условный: каталог диагностик внешнего парсера не входит в репозиторий.
func v133TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v133TestRegistry использует только полный штатный каталог регистраций.
func v133TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v133TestSnapshot сохраняет полную модель для проверки неизменности исходника.
func v133TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v133TestRun проверяет только новые V133 после полного штатного конвейера.
// Другие диагностики, включая согласованные дополнительные V032, не подавляются.
func v133TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) []model.Diagnostic {
	t.Helper()
	before := v133TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v133TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v133TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	added := result.Diagnostics[oldCount:]
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == "V133" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V133 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "После идентификатора @include записано дополнительное содержимое." ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.Source != processingID || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V133 occurrence %d = %+v, want location %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Fatal("V133 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V133 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v133TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Исключаются только новые записи и производные флаги/ссылки.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			newIDs := restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element error links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v133TestSnapshot(t, &restored) != before {
		t.Fatal("source data or previous history changed")
	}
	return added
}

// v133TestV109 проверяет, что запрещённая блочная форма отмечена отдельно.
func v133TestV109(t *testing.T, added []model.Diagnostic, want [3]int) {
	t.Helper()
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == "V109" {
			got = append(got, entry)
		}
	}
	location := model.Location{Start: model.Position{Line: want[0], Column: want[1]}, End: model.Position{Line: want[0], Column: want[2]}}
	related := model.Location{Start: model.Position{Line: want[0], Column: 1}, End: model.Position{Line: want[0], Column: 9}}
	if len(got) != 1 || got[0].Location == nil || *got[0].Location != location ||
		len(got[0].RelatedLocations) != 1 || got[0].RelatedLocations[0] != related {
		t.Fatalf("V109 = %+v, want one occurrence at %+v related to %+v", got, location, related)
	}
}

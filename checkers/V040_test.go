package checkers

import (
	"encoding/json"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV040Registration(t *testing.T) {
	definitions, err := v040TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V040" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "element" ||
			definition.Message != "У @section отсутствует название секции." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V040 definitions = %d, want 1", matches)
	}
}

func TestV040SectionRemainder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		line    model.Line
		section string
		want    [][3]int
	}{
		{"reference", v040TestLine(t, "@section Student book", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "Student book", "Student book", 10, 22)), "Student book", nil},
		{"example", v040TestLine(t, "@section", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9)), "", [][3]int{{2, 1, 9}}},
		{"trailing spaces and tabs", v040TestLine(t, "@section \t  \t", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9)), "", [][3]int{{2, 1, 9}}},
		{"indented declaration", v040TestLine(t, "  @section", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 3, 11)), "", [][3]int{{2, 3, 11}}},
		{"tab counts as one source column", v040TestLine(t, "\t@section\t", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 2, 10)), "", [][3]int{{2, 2, 10}}},
		{"canonical tag value", v040TestLine(t, "@SeCtIoN", "tag", 0, 0,
			v040TestElement(t, "tag", "@SeCtIoN", "section", 1, 9)), "", [][3]int{{2, 1, 9}}},
		{"double quotes are content", v040TestLine(t, "@section \"\"", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "\"\"", "\"\"", 10, 12)), "\"\"", nil},
		{"single quotes are content", v040TestLine(t, "@section ''", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "''", "''", 10, 12)), "''", nil},
		{"nonbreaking space is not trimmed", v040TestLine(t, "@section \u00a0", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "\u00a0", "\u00a0", 10, 11)), "\u00a0", nil},
		{"em space between ordinary edge spaces", v040TestLine(t, "@section  \u2003 \t", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "\u2003", "\u2003", 11, 12)), "\u2003", nil},
		{"Unicode name and internal spaces", v040TestLine(t, "@section Раздел  один", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "Раздел  один", "Раздел  один", 10, 22)), "Раздел  один", nil},
		{"one character is enough", v040TestLine(t, "@section Я", "tag", 0, 0,
			v040TestElement(t, "tag", "@section", "section", 1, 9),
			v040TestElement(t, "name", "Я", "Я", 10, 11)), "Я", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.3 и 1.31.2 требуют версии перед метаданными и заголовка
			// после них: неизменённые эталон и пример становятся строкой 2.
			result := v040TestDocument(t, v040TestVersion(t), tc.line, v040TestHeading(t),
				v040TestLine(t, "Read.", "content", 3, 1, v040TestElement(t, "content", "Read.", "Read.", 1, 6)))
			if tc.section != "" {
				result.Document.Metadata.Section = &tc.section
			}
			v040TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV040MalformedArgument(t *testing.T) {
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run(string(kind), func(t *testing.T) {
			// Имя тега распознано, но ошибочный остаток не стал name.
			unparsed := v040TestElement(t, "unparsed", "{", "", 10, 11)
			unparsed.Value, unparsed.ErrorIDs = nil, []string{"p1"}
			result := v040TestDocument(t, v040TestVersion(t),
				v040TestLine(t, "@section {", kind, 0, 0,
					v040TestElement(t, "tag", "@section", "section", 1, 9), unparsed),
				v040TestHeading(t))
			v040TestParserError(t, result, "element", 2, 10, 11)
			v040TestRun(t, result, "current", nil)
		})
	}
}

func TestV040PlacementAndLiteralContent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []model.Line
		want  [][3]int
	}{
		{"tag absent", []model.Line{v040TestVersion(t), v040TestHeading(t)}, nil},
		{"after heading and on last line", []model.Line{v040TestVersion(t), v040TestHeading(t),
			v040TestLine(t, "@section", "tag", 2, 1,
				v040TestElement(t, "tag", "@section", "section", 1, 9))}, [][3]int{{3, 1, 9}}},
		{"recognized nested tag", []model.Line{v040TestVersion(t), v040TestHeading(t),
			v040TestLine(t, "@note {", "block-start", 2, 1,
				v040TestElement(t, "tag", "@note", "note", 1, 6), v040TestElement(t, "block-open", "{", "{", 7, 8)),
			v040TestLine(t, "@section", "tag", 3, 2, v040TestElement(t, "tag", "@section", "section", 1, 9)),
			v040TestLine(t, "Read.", "content", 3, 2, v040TestElement(t, "content", "Read.", "Read.", 1, 6)),
			v040TestLine(t, "}", "block-end", 3, 2, v040TestElement(t, "block-close", "}", "}", 1, 2))}, [][3]int{{4, 1, 9}}},
		{"literal editor body", []model.Line{v040TestVersion(t), v040TestHeading(t),
			v040TestLine(t, "@editor {", "block-start", 2, 1,
				v040TestElement(t, "tag", "@editor", "editor", 1, 8), v040TestElement(t, "block-open", "{", "{", 9, 10)),
			v040TestLine(t, "@section", "content", 3, 2, v040TestElement(t, "content", "@section", "@section", 1, 9)),
			v040TestLine(t, "}", "block-end", 3, 2, v040TestElement(t, "block-close", "}", "}", 1, 2))}, nil},
		{"literal multifill body", []model.Line{v040TestVersion(t), v040TestHeading(t),
			v040TestLine(t, "@multifill {", "block-start", 2, 1,
				v040TestElement(t, "tag", "@multifill", "multifill", 1, 11), v040TestElement(t, "block-open", "{", "{", 12, 13)),
			v040TestLine(t, "@section", "content", 3, 2, v040TestElement(t, "content", "@section", "@section", 1, 9)),
			v040TestLine(t, "}", "block-end", 3, 2, v040TestElement(t, "block-close", "}", "}", 1, 2))}, nil},
		{"another tag with literal section", []model.Line{v040TestVersion(t), v040TestHeading(t),
			v040TestLine(t, "@note @section", "tag", 2, 1,
				v040TestElement(t, "tag", "@note", "note", 1, 6), v040TestElement(t, "content", "@section", "@section", 7, 15))}, nil},
		{"two missing names", []model.Line{v040TestVersion(t),
			v040TestLine(t, "@section", "tag", 0, 0, v040TestElement(t, "tag", "@section", "section", 1, 9)),
			v040TestLine(t, "@section", "tag", 0, 0, v040TestElement(t, "tag", "@section", "section", 1, 9)),
			v040TestHeading(t)}, [][3]int{{2, 1, 9}, {3, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v040TestRun(t, v040TestDocument(t, tc.lines...), "current", tc.want)
		})
	}
}

func TestV040LineEndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(string(ending), func(t *testing.T) {
			result := v040TestDocument(t, v040TestVersion(t), v040TestHeading(t),
				v040TestLine(t, "@section", "tag", 2, 1, v040TestElement(t, "tag", "@section", "section", 1, 9)))
			result.Lines[2].LineEnding = ending
			// Уже существующая ошибка расположения не исключает проверку имени.
			v040TestParserError(t, result, "line", 3, 1, 9)
			v040TestRun(t, result, "first", [][3]int{{3, 1, 9}})
			v040TestRun(t, result, "second", [][3]int{{3, 1, 9}})
		})
	}
}

func TestV040UnavailableLinesAndVersions(t *testing.T) {
	// Доступных физических строк нет; версия известна из входной модели.
	result := v040TestDocument(t)
	result.Document.LineCount = nil
	v040TestRun(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v040TestDocument(t)
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			before := v040TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v040TestRegistry(t))
			if err == nil || checks != nil || v040TestSnapshot(t, result) != before || len(result.Diagnostics) != 0 {
				t.Fatalf("unsupported version: checks=%v, err=%v, diagnostics=%+v", checks, err, result.Diagnostics)
			}
		})
	}
}

// v040TestRegistry создаёт реестр через штатный каталог целиком.
func v040TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v040TestDocument собирает явно заданные строки без разбора DSL.
func v040TestDocument(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, count := "1.2", "UTF-8", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
		if result.Lines[i].LineType == "heading" {
			title := "Unit 1"
			result.Document.Metadata.Title = &title
		}
	}
	return result
}

// v040TestVersion задаёт обязательную первую строку DSL 1.2.
func v040TestVersion(t *testing.T) model.Line {
	t.Helper()
	return v040TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v040TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v040TestElement(t, "version", "1.2", "1.2", 14, 17))
}

// v040TestHeading задаёт границу метаданных и заголовок юнита.
func v040TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v040TestLine(t, "# Unit 1", "heading", 0, 0,
		v040TestElement(t, "heading-level", "#", "1", 1, 2), v040TestElement(t, "title", "Unit 1", "Unit 1", 3, 9))
}

// v040TestLine сохраняет заданные структуру и исходный текст строки.
func v040TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v040TestElement создаёт элемент с явно заданными Unicode-колонками;
// правая граница исключается, ожидания по этим данным не вычисляются.
func v040TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v040TestParserError добавляет исходную ошибку с явным диапазоном.
func v040TestParserError(t *testing.T, result *model.Result, scope diagnostics.Scope, line, start, end int) {
	t.Helper()
	result.Document.HasErrors, result.Lines[line-1].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{
		ID: "p1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: "Ошибка разбора исходного документа.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: line, Column: start}, End: model.Position{Line: line, Column: end}},
	}}
}

// v040TestRun проверяет V040 через штатный конвейер по явным тройкам
// строка/начало/исключающий конец, включая ссылки tag и сохранение истории.
func v040TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v040TestSnapshot(t, result)
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v040TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v040TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(expected.Diagnostics):] {
		if entry.DiagnosticCode == "V040" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V040 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processingID ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "У @section отсутствует название секции." || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V040 metadata", i, entry, location)
		}

	}
	v040AssertPreserved(t, before, result)
}

// v040TestSnapshot сохраняет полное состояние для проверки побочных изменений.
func v040TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v040AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V040 заданы явно выше.
func v040AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v040TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v040TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

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

func TestV053Registration(t *testing.T) {
	registry := v053TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code == "V053" {
			count++
			if definition.Severity != "error" || definition.Scope != "document" ||
				definition.Message != "В документе отсутствует заголовок первого уровня." ||
				definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
				t.Fatalf("unexpected definition: %+v", definition)
			}
		}
	}
	if count != 1 {
		t.Fatalf("V053 definitions = %d, want 1", count)
	}
}

func TestV053ReferenceAndExample(t *testing.T) {
	// Эталон и пример являются полными документами; контекст не добавляется.
	t.Run("reference", func(t *testing.T) {
		result := v053TestDocument(t, "Lesson",
			v053TestHeading(t, "# Lesson", "Lesson", 1, 3),
			v053TestText(t, "Read carefully.", "Read carefully.", 2, 1))
		v053TestSource(t, result, "@dsl-version 1.2\n# Lesson\nRead carefully.")
		v053TestRun(t, result, "current", 0)
	})
	t.Run("example", func(t *testing.T) {
		result := v053TestDocument(t, "", v053TestText(t, "Read carefully.", "Read carefully.", 0, 0))
		v053TestSource(t, result, "@dsl-version 1.2\nRead carefully.")
		v053TestRun(t, result, "current", 1)
	})
}

func TestV053Candidates(t *testing.T) {
	for _, tc := range []struct {
		raw, title        string
		marker, textStart int
	}{
		{"# Lesson", "Lesson", 1, 3},
		{" # Lesson", "Lesson", 2, 4},
		{"\t# Lesson", "Lesson", 2, 4},
		{" \t # Lesson", "Lesson", 4, 6},
		{"# # Lesson", "# Lesson", 1, 3},
		{"# Урок 😀e\u0301", "Урок 😀e\u0301", 1, 3},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, tc.title,
				v053TestHeading(t, tc.raw, tc.title, tc.marker, tc.textStart)), "current", 0)
		})
	}
	for _, raw := range []string{"#", "# \t", "#Lesson", "#\tLesson", "#\u00a0Lesson", "#\t# Lesson"} {
		t.Run("malformed "+strconv.Quote(raw), func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, "", v053TestInvalid(t, raw, 0, 0)), "current", 0)
		})
	}
}

func TestV053NoCandidate(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"Read # carefully.", "Read # carefully."},
		{`\# Lesson`, "# Lesson"},
		{" \t\\# Lesson", "# Lesson"},
		{`\\# Lesson`, `\# Lesson`},
		{"\u00a0# Lesson", "\u00a0# Lesson"},
		{" \t\u2003# Lesson", "\u2003# Lesson"},
		{"＃ Lesson", "＃ Lesson"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, "", v053TestText(t, tc.raw, tc.value, 0, 0)), "current", 1)
		})
	}
	for _, tc := range []struct{ raw, marker, level, title string }{
		{"## Subtitle", "##", "2", "Subtitle"},
		{"### Practice", "###", "3", "Practice"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v053TestDocument(t, "", v053TestLine(t, tc.raw, "heading", 0, 0,
				v053TestElement(t, "heading-level", tc.marker, tc.level, 1),
				v053TestElement(t, "title", tc.title, tc.title, len(tc.marker)+2)))
			if tc.level == "2" {
				result.Document.Metadata.Subtitle = &tc.title
			}
			v053TestRun(t, result, "current", 1)
		})
	}
	for _, raw := range []string{"#### Practice", strings.Repeat("#", 64) + " Practice"} {
		t.Run(raw, func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, "", v053TestInvalid(t, raw, 0, 0)), "current", 1)
		})
	}
	t.Run("version only", func(t *testing.T) {
		v053TestRun(t, v053TestDocument(t, ""), "current", 1)
	})
	t.Run("blank lines", func(t *testing.T) {
		v053TestRun(t, v053TestDocument(t, "",
			v053TestLine(t, " ", "blank", 0, 0), v053TestLine(t, "\t", "blank", 0, 0),
			v053TestLine(t, "", "blank", 0, 0)), "current", 1)
	})
}

func TestV053Contexts(t *testing.T) {
	for _, tc := range []struct{ rawTag, tag string }{
		{"editor", "editor"}, {"EDITOR", "editor"},
		{"multifill", "multifill"}, {"MultiFill", "multifill"},
	} {
		t.Run("literal "+tc.rawTag, func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, "",
				v053TestOpen(t, tc.rawTag, tc.tag, 0, 0),
				v053TestText(t, "# Lesson", "# Lesson", 2, 1), v053TestClose(t, 2, 1)), "current", 1)
		})
		t.Run("literal in task "+tc.rawTag, func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, "",
				v053TestTask(t), v053TestInstruction(t),
				v053TestOpen(t, tc.rawTag, tc.tag, 2, 1),
				v053TestText(t, "# Lesson", "# Lesson", 4, 2), v053TestClose(t, 4, 2)), "current", 1)
		})
	}
	for _, tc := range []struct {
		name  string
		body  []model.Line
		title string
		want  int
	}{
		{"forbidden heading in text", []model.Line{
			v053TestOpen(t, "text", "text", 0, 0), v053TestInvalid(t, "# Lesson", 2, 1),
			v053TestText(t, "Read.", "Read.", 2, 1), v053TestClose(t, 2, 1),
		}, "", 0},
		{"forbidden heading in task", []model.Line{
			v053TestTask(t), v053TestInstruction(t), v053TestInvalid(t, "# Lesson", 2, 1),
		}, "", 0},
		{"candidate after closed editor", []model.Line{
			v053TestOpen(t, "editor", "editor", 0, 0), v053TestText(t, "# Comment", "# Comment", 2, 1),
			v053TestClose(t, 2, 1), v053TestHeading(t, "# Lesson", "Lesson", 1, 3),
		}, "Lesson", 0},
		{"single-line editor only", []model.Line{
			v053TestLine(t, "@editor # Lesson", "tag", 0, 0,
				v053TestElement(t, "tag", "@editor", "editor", 1), v053TestElement(t, "content", "# Lesson", "# Lesson", 9)),
		}, "", 1},
		{"candidate after single-line editor", []model.Line{
			v053TestLine(t, "@editor # Comment", "tag", 0, 0,
				v053TestElement(t, "tag", "@editor", "editor", 1), v053TestElement(t, "content", "# Comment", "# Comment", 9)),
			v053TestHeading(t, "# Lesson", "Lesson", 1, 3),
		}, "Lesson", 0},
		{"two declarations", []model.Line{
			v053TestHeading(t, "# First", "First", 1, 3), v053TestHeading(t, "# Last", "Last", 1, 3),
		}, "", 0},
		{"one finding for all non-candidates", []model.Line{
			v053TestText(t, "Read.", "Read.", 0, 0), v053TestText(t, `\# Lesson`, "# Lesson", 0, 0),
			v053TestLine(t, "", "blank", 0, 0),
			v053TestOpen(t, "editor", "editor", 0, 0), v053TestText(t, "# Comment", "# Comment", 5, 1),
			v053TestClose(t, 5, 1), v053TestOpen(t, "multifill", "multifill", 0, 0),
			v053TestText(t, "# Prefill", "# Prefill", 8, 1), v053TestClose(t, 8, 1),
		}, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v053TestRun(t, v053TestDocument(t, tc.title, tc.body...), "current", tc.want)
		})
	}
}

func TestV053EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		for _, present := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/"+strconv.FormatBool(present), func(t *testing.T) {
				title, want := "", 1
				line := v053TestText(t, "Read.", "Read.", 0, 0)
				if present {
					title, want = "Lesson", 0
					line = v053TestHeading(t, "# Lesson", "Lesson", 1, 3)
				}
				result := v053TestDocument(t, title, line)
				if ending != "" {
					result.Lines[0].LineEnding = ending
				}
				result.Lines[1].LineEnding = ending
				v053TestRun(t, result, "first", want)
				v053TestRun(t, result, "second", want)
			})
		}
	}
}

func TestV053UnavailableLinesAndVersions(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("known count "+strconv.FormatBool(known), func(t *testing.T) {
			result := v053TestDocument(t, "")
			result.Lines = []model.Line{}
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v053TestRun(t, result, "current", 0)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v053TestDocument(t, "")
			result.Lines = []model.Line{}
			result.Document.LineCount = nil
			result.Document.DSLVersion = &version
			before := v053TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v053TestRegistry(t))
			if err == nil || checks != nil || v053TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v053TestDocument добавляет только обязательную версию перед явно заданными
// строками. Родители заданы абсолютными номерами. unparsed получает исходную
// ошибку парсера по контракту модели; это не ожидаемый результат V053.
func v053TestDocument(t *testing.T, title string, body ...model.Line) *model.Result {
	t.Helper()
	version := "1.2"
	lines := append([]model.Line{v053TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v053TestElement(t, "tag", "@dsl-version", "dsl-version", 1),
		v053TestElement(t, "version", "1.2", "1.2", 14))}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if title != "" {
		result.Document.Metadata.Title = &title
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		line := &lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "unparsed" {
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
	}
	return result
}

// v053TestLine сохраняет явно заданную структуру без разбора Raw.
func v053TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v053TestElement задаёт элемент с исключающей правой Unicode-границей.
func v053TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v053TestHeading(t *testing.T, raw, title string, marker, textStart int) model.Line {
	t.Helper()
	return v053TestLine(t, raw, "heading", 0, 0, v053TestElement(t, "heading-level", "#", "1", marker),
		v053TestElement(t, "title", title, title, textStart))
}

func v053TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v053TestLine(t, raw, "content", parent, depth, v053TestElement(t, "content", raw, value, 1))
}

func v053TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v053TestElement(t, "unparsed", raw, "", 1)
	element.Value = nil
	return v053TestLine(t, raw, "invalid", parent, depth, element)
}

func v053TestOpen(t *testing.T, rawTag, tag string, parent, depth int) model.Line {
	t.Helper()
	return v053TestLine(t, "@"+rawTag+" {", "block-start", parent, depth,
		v053TestElement(t, "tag", "@"+rawTag, tag, 1), v053TestElement(t, "block-open", "{", "{", len(rawTag)+3))
}

func v053TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v053TestLine(t, "}", "block-end", parent, depth, v053TestElement(t, "block-close", "}", "}", 1))
}

func v053TestTask(t *testing.T) model.Line {
	t.Helper()
	return v053TestLine(t, "@task a", "tag", 0, 0,
		v053TestElement(t, "tag", "@task", "task", 1), v053TestElement(t, "identifier", "a", "a", 7))
}

func v053TestInstruction(t *testing.T) model.Line {
	t.Helper()
	return v053TestLine(t, "@instr Read.", "tag", 2, 1,
		v053TestElement(t, "tag", "@instr", "instr", 1), v053TestElement(t, "content", "Read.", "Read.", 8))
}

func v053TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v053TestRun проверяет только новые V053, выполняя полный штатный конвейер.
// Ожидания числа сообщений заданы ситуацией; диапазоны всегда отсутствуют.
func v053TestRun(t *testing.T, result *model.Result, processingID string, want int) {
	t.Helper()
	before := v053TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v053TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v053TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	count := 0
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode != "V053" {
			continue
		}
		count++
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" || entry.DiagnosticScope != "document" ||
			entry.Fatal || entry.Location != nil || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "В документе отсутствует заголовок первого уровня." {
			t.Fatalf("unexpected V053: %+v", entry)
		}
	}
	if count != want {
		t.Fatalf("V053 count = %d, want %d; diagnostics: %+v", count, want, result.Diagnostics)
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех диагностик.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v053TestSnapshot(t, result)), &restored); err != nil {
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
	if v053TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v053TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v053TestSource(t *testing.T, result *model.Result, want string) {
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

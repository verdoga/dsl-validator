package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV045Registration(t *testing.T) {
	definitions, err := v045TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V045" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "element" ||
			definition.Message != "Путь к каталогу ресурсов указан повторно." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V045 definitions = %d, want 1", matches)
	}
}

func TestV045PathValues(t *testing.T) {
	for _, tc := range []struct {
		name, firstRaw, firstValue, nextRaw, nextValue string
		want                                           [][6]int
	}{
		{"reference overlapping directories", "Audio", "Audio", "Audio/Unit1", "Audio/Unit1", nil},
		{"identical", "Audio", "Audio", "Audio", "Audio", [][6]int{{3, 15, 20, 2, 15, 20}}},
		{"ASCII case", "Audio", "Audio", "audio", "audio", [][6]int{{3, 15, 20, 2, 15, 20}}},
		{"quoted duplicate", "Audio", "Audio", `"audio"`, "audio", [][6]int{{3, 15, 22, 2, 15, 20}}},
		{"quoted first occurrence", `"Audio"`, "Audio", "audio", "audio", [][6]int{{3, 15, 20, 2, 15, 22}}},
		{"Cyrillic case", "Аудио", "Аудио", "аудио", "аудио", [][6]int{{3, 15, 20, 2, 15, 20}}},
		{"Latin case outside ASCII", "ÉTÉ", "ÉTÉ", "été", "été", [][6]int{{3, 15, 18, 2, 15, 18}}},
		{"supplementary Unicode columns", "𐐀udio", "𐐀udio", "𐐨UDIO", "𐐨UDIO", [][6]int{{3, 15, 20, 2, 15, 20}}},
		{"comma inside quoted path", `"A,B"`, "A,B", `"a,b"`, "a,b", [][6]int{{3, 15, 20, 2, 15, 20}}},
		{"internal spaces preserved", "Audio Unit", "Audio Unit", "Audio  Unit", "Audio  Unit", nil},
		{"quoted and unquoted spaces", "Audio Unit", "Audio Unit", `"audio unit"`, "audio unit", [][6]int{{3, 15, 27, 2, 15, 25}}},
		{"spaces inside quotes preserved", "Audio", "Audio", `" Audio "`, " Audio ", nil},
		{"quoted whitespace is a nonempty value", `" "`, " ", `" "`, " ", [][6]int{{3, 15, 18, 2, 15, 18}}},
		{"relative dot not resolved", "Audio", "Audio", "./Audio", "./Audio", nil},
		{"parent directory not resolved", "Video", "Video", "Audio/../Video", "Audio/../Video", nil},
		{"absolute path not inferred", "Audio", "Audio", "/Audio", "/Audio", nil},
		{"separators not converted", "Audio/Unit1", "Audio/Unit1", `Audio\Unit1`, `Audio\Unit1`, nil},
		{"backslash remains literal", `Audio\Unit1`, `Audio\Unit1`, `audio\UNIT1`, `audio\UNIT1`, [][6]int{{3, 15, 26, 2, 15, 26}}},
		{"trailing slash preserved", "Audio", "Audio", "Audio/", "Audio/", nil},
		{"no canonical Unicode normalization", "é", "é", "e\u0301", "e\u0301", nil},
		{"no compatibility normalization", "Ａudio", "Ａudio", "Audio", "Audio", nil},
		{"lowercase is not case folding", "Σ", "Σ", "ς", "ς", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v045TestDocument(t, []model.Line{
				v045TestSingle(t, tc.firstRaw, tc.firstValue), v045TestSingle(t, tc.nextRaw, tc.nextValue),
			})
			v045TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV045ListsAndFirstOccurrences(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []model.Line
		want  [][6]int
	}{
		{"tag absent", nil, nil},
		{"single path", []model.Line{v045TestSingle(t, "Audio", "Audio")}, nil},
		{"example", []model.Line{
			v045TestSingle(t, "Audio", "Audio"),
			v045TestResourceDir(t, `@resource-dir "audio", Video, Audio`,
				v045TestElement(t, "resource-path", `"audio"`, "audio", 15, 22),
				v045TestElement(t, "resource-path", "Video", "Video", 24, 29),
				v045TestElement(t, "resource-path", "Audio", "Audio", 31, 36)),
		}, [][6]int{{3, 15, 22, 2, 15, 20}, {3, 31, 36, 2, 15, 20}}},
		{"three occurrences in one list", []model.Line{
			v045TestResourceDir(t, "@resource-dir Audio, audio, AUDIO",
				v045TestElement(t, "resource-path", "Audio", "Audio", 15, 20),
				v045TestElement(t, "resource-path", "audio", "audio", 22, 27),
				v045TestElement(t, "resource-path", "AUDIO", "AUDIO", 29, 34)),
		}, [][6]int{{2, 22, 27, 2, 15, 20}, {2, 29, 34, 2, 15, 20}}},
		{"three declarations retain first occurrence", []model.Line{
			v045TestSingle(t, "Audio", "Audio"), v045TestSingle(t, "audio", "audio"), v045TestSingle(t, "AUDIO", "AUDIO"),
		}, [][6]int{{3, 15, 20, 2, 15, 20}, {4, 15, 20, 2, 15, 20}}},
		{"independent keys", []model.Line{
			v045TestResourceDir(t, "@resource-dir A, B, b, a",
				v045TestElement(t, "resource-path", "A", "A", 15, 16),
				v045TestElement(t, "resource-path", "B", "B", 18, 19),
				v045TestElement(t, "resource-path", "b", "b", 21, 22),
				v045TestElement(t, "resource-path", "a", "a", 24, 25)),
		}, [][6]int{{2, 21, 22, 2, 18, 19}, {2, 24, 25, 2, 15, 16}}},
		{"tab separates list elements", []model.Line{
			v045TestResourceDir(t, "@resource-dir Audio,\t\"audio\"",
				v045TestElement(t, "resource-path", "Audio", "Audio", 15, 20),
				v045TestElement(t, "resource-path", `"audio"`, "audio", 22, 29)),
		}, [][6]int{{2, 22, 29, 2, 15, 20}}},
		{"external spaces and uppercase tag", []model.Line{
			v045TestSingle(t, "Audio", "Audio"),
			v045TestLine(t, "  @RESOURCE-DIR  audio \t", "tag", 0, 0,
				v045TestElement(t, "tag", "@RESOURCE-DIR", "resource-dir", 3, 16),
				v045TestElement(t, "resource-path", "audio", "audio", 18, 23)),
		}, [][6]int{{3, 18, 23, 2, 15, 20}}},
		{"blank does not reset first occurrence", []model.Line{
			v045TestSingle(t, "Audio", "Audio"), v045TestLine(t, "", "blank", 0, 0), v045TestSingle(t, "audio", "audio"),
		}, [][6]int{{4, 15, 20, 2, 15, 20}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v045TestRun(t, v045TestDocument(t, tc.lines), "current", tc.want)
		})
	}
}

func TestV045ExcludedAndMalformedPaths(t *testing.T) {
	t.Run("empty paths", func(t *testing.T) {
		result := v045TestDocument(t, []model.Line{
			v045TestResourceDir(t, `@resource-dir "", ""`,
				v045TestElement(t, "resource-path", `""`, "", 15, 17),
				v045TestElement(t, "resource-path", `""`, "", 19, 21)),
		})
		v045TestParserError(t, result, "p1", 2, 15, 17)
		v045TestParserError(t, result, "p2", 2, 19, 21)
		v045TestRun(t, result, "current", nil)
	})
	t.Run("unparsed is not a first occurrence", func(t *testing.T) {
		unparsed := v045TestElement(t, "unparsed", `"Audio`, "", 15, 21)
		unparsed.Value = nil
		line := v045TestResourceDir(t, `@resource-dir "Audio`, unparsed)
		line.LineType = "invalid"
		result := v045TestDocument(t, []model.Line{line, v045TestSingle(t, "Audio", "Audio"), v045TestSingle(t, "audio", "audio")})
		v045TestParserError(t, result, "p1", 2, 15, 21)
		v045TestRun(t, result, "current", [][6]int{{4, 15, 20, 3, 15, 20}})
	})
	t.Run("recognized prefix in malformed declaration", func(t *testing.T) {
		unparsed := v045TestElement(t, "unparsed", `"broken`, "", 22, 29)
		unparsed.Value = nil
		line := v045TestResourceDir(t, `@resource-dir audio, "broken`,
			v045TestElement(t, "resource-path", "audio", "audio", 15, 20), unparsed)
		line.LineType = "invalid"
		result := v045TestDocument(t, []model.Line{v045TestSingle(t, "Audio", "Audio"), line})
		v045TestParserError(t, result, "p1", 3, 22, 29)
		v045TestRun(t, result, "current", [][6]int{{3, 15, 20, 2, 15, 20}})
	})
	t.Run("literal editor contents", func(t *testing.T) {
		result := v045TestDocument(t, []model.Line{v045TestSingle(t, "Audio", "Audio")},
			v045TestLine(t, "@editor {", "block-start", 3, 1,
				v045TestElement(t, "tag", "@editor", "editor", 1, 8), v045TestElement(t, "block-open", "{", "{", 9, 10)),
			v045TestLine(t, "@resource-dir Audio", "content", 4, 2,
				v045TestElement(t, "content", "@resource-dir Audio", "@resource-dir Audio", 1, 20)),
			v045TestLine(t, "}", "block-end", 4, 2, v045TestElement(t, "block-close", "}", "}", 1, 2)))
		v045TestRun(t, result, "current", nil)
	})
	t.Run("media source is not a resource path", func(t *testing.T) {
		result := v045TestDocument(t, []model.Line{v045TestSingle(t, "Audio", "Audio")},
			v045TestLine(t, "@media audio Audio", "tag", 3, 1,
				v045TestElement(t, "tag", "@media", "media", 1, 7), v045TestElement(t, "media-type", "audio", "audio", 8, 13),
				v045TestElement(t, "source", "Audio", "Audio", 14, 19)))
		v045TestRun(t, result, "current", nil)
	})
}

func TestV045EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(string(ending), func(t *testing.T) {
			// Последнее объявление расположено после заголовка ошибочно,
			// но остаётся распознанным resource-dir и участвует в V045.
			line := v045TestSingle(t, `"audio"`, "audio")
			parent := 3
			line.ParentLine, line.NestingLevel, line.LineEnding = &parent, 1, ending
			result := v045TestDocument(t, []model.Line{v045TestSingle(t, "Audio", "Audio")}, line)
			v045TestRun(t, result, "first", [][6]int{{4, 15, 22, 2, 15, 20}})
			v045TestRun(t, result, "second", [][6]int{{4, 15, 22, 2, 15, 20}})
		})
	}
	// Новый документ после повторных запусков не наследует сохранённые пути.
	v045TestRun(t, v045TestDocument(t, []model.Line{v045TestSingle(t, "Audio", "Audio")}), "current", nil)
}

func TestV045UnavailableLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			// Версия известна, исходные строки недоступны: грамматику
			// неподдерживаемых версий тест не предполагает.
			result := &model.Result{
				Format:     model.FormatVersion1,
				Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{},
			}
			if version == "1.2" {
				v045TestRun(t, result, "current", nil)
				return
			}
			before := v045TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v045TestRegistry(t))
			if err == nil || checks != nil || len(result.Diagnostics) != 0 || v045TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v, diagnostics=%+v", checks, err, result.Diagnostics)
			}
		})
	}
}

// v045TestSingle задаёт один путь с независимыми Raw и Value, без разбора кавычек.
// Длина применяется только к входному элементу, а не к ожидаемой диагностике.
func v045TestSingle(t *testing.T, raw, value string) model.Line {
	t.Helper()
	return v045TestResourceDir(t, "@resource-dir "+raw,
		v045TestElement(t, "resource-path", raw, value, 15, 15+utf8.RuneCountInString(raw)))
}

// v045TestResourceDir добавляет тег к явно заданным элементам исходного списка.
func v045TestResourceDir(t *testing.T, raw string, paths ...model.Element) model.Line {
	t.Helper()
	elements := []model.Element{v045TestElement(t, "tag", "@resource-dir", "resource-dir", 1, 14)}
	return v045TestLine(t, raw, "tag", 0, 0, append(elements, paths...)...)
}

// v045TestDocument дополняет метаданные обязательной версией перед ними
// и заголовком после них (DSL 1.3, 1.31.2). Эталон и пример не изменяются,
// их первая строка получает номер 2. Входные элементы и связи заданы тестами.
func v045TestDocument(t *testing.T, metadata []model.Line, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Unit 1"
	lines := []model.Line{v045TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v045TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v045TestElement(t, "version", "1.2", "1.2", 14, 17))}
	lines = append(lines, metadata...)
	lines = append(lines, v045TestLine(t, "# Unit 1", "heading", 0, 0,
		v045TestElement(t, "heading-level", "#", "1", 1, 2), v045TestElement(t, "title", "Unit 1", "Unit 1", 3, 9)))
	lines = append(lines, body...)
	paths := []string{}
	for _, line := range metadata {
		for _, element := range line.Elements {
			if element.ElementType == "resource-path" && element.Value != nil && *element.Value != "" {
				paths = append(paths, *element.Value)
			}
		}
	}
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: paths}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
}

// v045TestLine сохраняет заданные структуру, элементы и исходный текст.
func v045TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v045TestElement задаёт исходные Unicode-колонки с исключающей правой границей.
func v045TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v045TestParserError добавляет исходную ошибку и обратную ссылку элемента.
func v045TestParserError(t *testing.T, result *model.Result, id string, number, start, end int) {
	t.Helper()
	result.Document.HasErrors, result.Lines[number-1].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
		Message: "Ошибка разбора пути.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
	for i := range result.Lines[number-1].Elements {
		element := &result.Lines[number-1].Elements[i]
		if element.Start == start && element.End == end {
			element.ErrorIDs = append(element.ErrorIDs, id)
			return
		}
	}
	t.Fatal("parser error range does not identify an element")
}

// v045TestRegistry использует штатный каталог целиком.
func v045TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v045TestRun проверяет только новые V045 по явным шестёркам:
// строка/начало/конец текущего элемента и строка/начало/конец первого пути.
// Оба конца исключаются. Исходные поля и история должны сохраниться.
func v045TestRun(t *testing.T, result *model.Result, processingID string, want [][6]int) {
	t.Helper()
	before := v045TestSnapshot(t, result)
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v045TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v045TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(expected.Diagnostics):] {
		if entry.DiagnosticCode == "V045" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V045 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		related := model.Location{Start: model.Position{Line: position[3], Column: position[4]}, End: model.Position{Line: position[3], Column: position[5]}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "Путь к каталогу ресурсов указан повторно." {
			t.Fatalf("occurrence %d = %+v, want location %+v, related %+v and exact V045 metadata", i, entry, location, related)
		}

	}
	v045AssertPreserved(t, before, result)
}

// v045TestSnapshot сохраняет модель для сравнения с ожидаемыми изменениями.
func v045TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v045AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V045 заданы явно выше.
func v045AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v045TestSnapshot(t, result)), &restored); err != nil {
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
	if v045TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

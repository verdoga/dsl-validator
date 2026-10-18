package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV025Endings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		eols    [4]model.LineEnding
		detail  string
		related [][2]int
	}{
		{"reference all LF", [4]model.LineEnding{"\n", "\n", "\n", "\n"}, "", nil},
		{"all CRLF", [4]model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n"}, "", nil},
		{"example", [4]model.LineEnding{"\n", "\n", "\n", "\r\n"}, "LF: 3; CRLF: 1", [][2]int{{1, 17}, {4, 14}}},
		{"LF without final EOL", [4]model.LineEnding{"\n", "\n", "\n", ""}, "", nil},
		{"CRLF without final EOL", [4]model.LineEnding{"\r\n", "\r\n", "\r\n", ""}, "", nil},
		{"reverse first occurrence", [4]model.LineEnding{"\r\n", "\r\n", "\r\n", "\n"}, "LF: 1; CRLF: 3", [][2]int{{4, 14}, {1, 17}}},
		{"mixture without final EOL", [4]model.LineEnding{"\n", "\r\n", "\n", ""}, "LF: 2; CRLF: 1", [][2]int{{1, 17}, {2, 9}}},
		{"several alternations", [4]model.LineEnding{"\n", "\r\n", "\n", "\r\n"}, "LF: 2; CRLF: 2", [][2]int{{1, 17}, {2, 9}}},
		{"reverse alternations", [4]model.LineEnding{"\r\n", "\n", "\r\n", "\n"}, "LF: 2; CRLF: 2", [][2]int{{2, 9}, {1, 17}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Эталон и пример сохранены: оба @note без изменений. Две
			// обязательные строки контекста также входят в общие количества.
			result := v025Document(t,
				v025Line(t, "@note First.", "tag", 2, 1,
					v025Element(t, "tag", "@note", "note", 1, 6), v025Element(t, "content", "First.", "First.", 7, 13)),
				v025Line(t, "@note Second.", "tag", 2, 1,
					v025Element(t, "tag", "@note", "note", 1, 6), v025Element(t, "content", "Second.", "Second.", 7, 14)))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v025Run(t, result, "current", tc.detail, tc.related)
		})
	}
}

func TestV025MinimalAndUnavailable(t *testing.T) {
	v025Run(t, v025UnavailableDocument(t, "1.2"), "current", "", nil)
	for _, tc := range []struct {
		name, detail string
		eols         [2]model.LineEnding
		related      [][2]int
	}{
		{"one LF and one CRLF", "LF: 1; CRLF: 1", [2]model.LineEnding{"\n", "\r\n"}, [][2]int{{1, 17}, {2, 9}}},
		{"one CRLF and one LF", "LF: 1; CRLF: 1", [2]model.LineEnding{"\r\n", "\n"}, [][2]int{{2, 9}, {1, 17}}},
		{"single LF", "", [2]model.LineEnding{"\n", ""}, nil},
		{"single CRLF", "", [2]model.LineEnding{"\r\n", ""}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v025Document(t)
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v025Run(t, result, "current", tc.detail, tc.related)
		})
	}
}

func TestV025BlankAndUnicodePositions(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		column    int
	}{
		{"empty blank", "", 1},
		{"whitespace blank uses Raw", " \t", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v025Document(t, v025Line(t, tc.raw, "blank", 2, 1))
			result.Lines[2].LineEnding = "\r\n"
			v025Run(t, result, "current", "LF: 2; CRLF: 1", [][2]int{{1, 17}, {3, tc.column}})
		})
	}
	for _, tc := range []struct {
		name, title string
		column      int
	}{
		{"supplementary Unicode", "Я😀", 5},
		{"combining Unicode", "e\u0301", 5},
		{"precomposed Unicode", "é", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v025Document(t)
			result.Document.Metadata.Title = &tc.title
			result.Lines[1].Raw = "# " + tc.title
			result.Lines[1].Elements[1] = v025Element(t, "title", tc.title, tc.title, 3, tc.column)
			result.Lines[1].LineEnding = "\r\n"
			v025Run(t, result, "current", "LF: 1; CRLF: 1", [][2]int{{1, 17}, {2, tc.column}})
		})
	}
}

func TestV025AllLineTypesAndUnparsed(t *testing.T) {
	bad := v025Element(t, "unparsed", "@bad", "", 1, 5)
	bad.Value, bad.ErrorIDs = nil, []string{"d1"}
	result := v025Document(t,
		v025Line(t, "@example {", "block-start", 2, 1,
			v025Element(t, "tag", "@example", "example", 1, 9), v025Element(t, "block-open", "{", "{", 10, 11)),
		v025Text(t, "First.", 3, 2),
		v025Line(t, "---", "separator", 3, 2),
		v025Text(t, "Second.", 3, 2),
		v025Line(t, "", "blank", 3, 2),
		v025Line(t, "}", "block-end", 3, 2, v025Element(t, "block-close", "}", "}", 1, 2)),
		v025Line(t, "@bad", "invalid", 2, 1, bad))
	result.Lines[4].LineEnding, result.Lines[8].LineEnding = "\r\n", "\r\n"
	result.Document.HasErrors, result.Lines[8].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{
		ID: "d1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		Message: "Неизвестный тег.", DiagnosticScope: "element",
		Location:         &model.Location{Start: model.Position{Line: 9, Column: 1}, End: model.Position{Line: 9, Column: 5}},
		RelatedLocations: []model.Location{},
	}}
	v025Run(t, result, "current", "LF: 7; CRLF: 2", [][2]int{{1, 17}, {5, 4}})
}

func TestV025HistoryAndOtherChecks(t *testing.T) {
	result := v025Document(t, v025Text(t, "A  \t“x”", 2, 1))
	result.Lines[2].LineEnding = "\r\n"
	wantRelated := [][2]int{{1, 17}, {3, 8}}
	v025Run(t, result, "first", "LF: 2; CRLF: 1", wantRelated)
	v025Run(t, result, "second", "LF: 2; CRLF: 1", wantRelated)
	for _, source := range []string{"first", "second"} {
		for _, tc := range []struct {
			code       diagnostics.Code
			start, end int
		}{
			{"V001", 2, 4}, {"V002", 4, 5}, {"V009", 5, 6}, {"V009", 7, 8},
		} {
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source != source || entry.DiagnosticCode != tc.code || entry.Location == nil {
					continue
				}
				location := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
				if *entry.Location == location {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s at [%d,%d) in %s: count = %d, want 1", tc.code, tc.start, tc.end, source, count)
			}
		}
	}
	v025Run(t, v025Document(t), "first", "", nil)
}

func TestV025UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v025UnavailableDocument(t, version)
			before := v025Snapshot(t, result)
			checks, err := validator.Prepare(result, v025Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v025Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV025Registration(t *testing.T) {
	definitions, err := v025Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V025" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Message != "В документе смешаны окончания строк LF и CRLF." ||
			definition.Scope != "document" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V025 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V025 definitions = %d, want 1", matches)
	}
}

// v025UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v025UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v025Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v025Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v025Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v025Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v025Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v025Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v025Element(t, "version", "1.2", "1.2", 14, 17)),
		v025Line(t, "# Unit 1", "heading", 0, 0,
			v025Element(t, "heading-level", "#", "1", 1, 2),
			v025Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}},
		},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       lines,
		Diagnostics: []model.Diagnostic{},
	}
}

// v025Line собирает строку с явно заданной структурой, без классификации Raw.
func v025Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v025Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v025Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v025Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v025Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v025Line(t, text, "content", parent, depth,
		v025Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v025Run проверяет V025 текущего запуска через штатный каталог и конвейер.
// Непустой detail означает ровно одно сообщение с явно заданными связанными
// позициями. Пустой detail требует отсутствия V025, включая лишние сообщения.
func v025Run(t *testing.T, result *model.Result, processingID, detail string, related [][2]int) {
	t.Helper()
	before := v025Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v025Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v025Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V025" && entry.Source != processingID {
			t.Fatalf("new V025 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V025" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	wantCount := 0
	if detail != "" {
		wantCount = 1
	}
	if len(got) != wantCount {
		t.Fatalf("current V025 count = %d, want %d; diagnostics: %+v", len(got), wantCount, got)
	}
	if wantCount == 1 {
		entry := got[0]
		if entry.DiagnosticCode != "V025" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "В документе смешаны окончания строк LF и CRLF.\nконкретизация: "+detail ||
			entry.DiagnosticScope != "document" || entry.Fatal || entry.Location != nil {
			t.Errorf("unexpected V025 metadata: %+v", entry)
		}
		if entry.RelatedLocations == nil || len(entry.RelatedLocations) != 2 || len(related) != 2 {
			t.Fatalf("related positions = %+v, want exactly two: %v", entry.RelatedLocations, related)
		}
		for i, expected := range related {
			position := model.Position{Line: expected[0], Column: expected[1]}
			want := model.Location{Start: position, End: position}
			if entry.RelatedLocations[i] != want {
				t.Errorf("related position %d = %+v, want %+v", i, entry.RelatedLocations[i], want)
			}
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	v025AssertPreserved(t, before, result)
}

// v025Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v025Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v025AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V025 заданы явно выше.
func v025AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v025Snapshot(t, result)), &restored); err != nil {
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
	if v025Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

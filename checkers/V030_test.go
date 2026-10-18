package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV030Text(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"reference", "word-word – word — word − word", "word-word – word — word − word", nil},
		{"example", "ab‐cd", "ab‐cd", [][3]int{{3, 3, 4}}},
		{"ordinary text", "Текст без нарушения.", "Текст без нарушения.", nil},
		{"adjacent different signs", "a‐‑b", "a‐‑b", [][3]int{{3, 2, 3}, {3, 3, 4}}},
		{"adjacent identical signs", "a‐‐b", "a‐‐b", [][3]int{{3, 2, 3}, {3, 3, 4}}},
		{"first and last characters", "‐a‑", "‐a‑", [][3]int{{3, 1, 2}, {3, 3, 4}}},
		{"single character", "‐", "‐", [][3]int{{3, 1, 2}}},
		{"Unicode columns", "Я😀‐", "Я😀‐", [][3]int{{3, 3, 4}}},
		{"combining character", "e\u0301‐", "e\u0301‐", [][3]int{{3, 3, 4}}},
		{"literal backslash", `\‐`, `\‐`, [][3]int{{3, 2, 3}}},
		{"Raw columns after escaped braces", `\{x\}a‐b`, `{x}a‐b`, [][3]int{{3, 7, 8}}},
		{"entity is not decoded", "&#8208;", "&#8208;", nil},
		{"nearby non-target characters", "¬®‖﹗﹙﹢﹤，．", "¬®‖﹗﹙﹢﹤，．", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v030Line(t, tc.raw, "content", 2, 1,
				v030Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			// Обязательные версия и заголовок добавляются перед фрагментом;
			// его исходные символы не изменяются, он находится на строке 3.
			v030Run(t, v030Document(t, line), "current", tc.want)
		})
	}
}

func TestV030EachCodePoint(t *testing.T) {
	for _, tc := range []struct {
		name string
		char rune
	}{
		{"U+00AD soft hyphen is not removed", '\u00AD'},
		{"U+2010", '\u2010'},
		{"U+2011", '\u2011'},
		{"U+2012", '\u2012'},
		{"U+2015", '\u2015'},
		{"U+FE58", '\uFE58'},
		{"U+FE63 is not normalized", '\uFE63'},
		{"U+FF0D is not normalized", '\uFF0D'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v030Run(t, v030Document(t, v030Text(t, "a"+string(tc.char)+"b", 2, 1)),
				"current", [][3]int{{3, 2, 3}})
		})
	}
}

func TestV030AnswerRepresentations(t *testing.T) {
	for _, separate := range []bool{false, true} {
		for _, sign := range []rune{'\u00AD', '\u2010', '\u2011', '\u2012', '\u2015', '\uFE58', '\uFE63', '\uFF0D'} {
			result := v015AnswerDocument(t, "", "a"+string(sign)+"b", "", separate)
			v030Run(t, result, "current", [][3]int{{4, 8, 9}})
			for _, entry := range result.Diagnostics {
				if entry.DiagnosticCode == "V015" {
					t.Fatal("nonstandard dash belongs to V030, not V015")
				}
			}
		}
		v030Run(t, v015AnswerDocument(t, "Я😀 ", `a\}‐`, " ‑", separate), "current", [][3]int{{4, 13, 14}, {4, 16, 17}})
		v030Run(t, v015AnswerDocument(t, "", "a-b – c — d", "", separate), "current", nil)
	}
	v030Run(t, v021Block(t, "multifill", v021Text(t, "_____{a‐b", 3, 2)), "current", [][3]int{{4, 8, 9}})
}

func TestV030Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"single-line comment", []model.Line{
			v030Line(t, "@editor a‐b", "tag", 2, 1,
				v030Element(t, "tag", "@editor", "editor", 1, 8),
				v030Element(t, "content", "a‐b", "a‐b", 9, 12)),
		}, [][3]int{{3, 10, 11}}},
		{"block comment", []model.Line{
			v030Open(t, "editor", 2, 1), v030Text(t, "a‐b", 3, 2), v030Close(t, 3, 2),
		}, [][3]int{{4, 2, 3}}},
		{"nested hint", []model.Line{
			v030Open(t, "instr", 2, 1), v030Text(t, "Read.", 3, 2),
			v030Open(t, "hint", 3, 2), v030Text(t, "a‐b", 5, 3),
			v030Close(t, 5, 3), v030Close(t, 3, 2),
		}, [][3]int{{6, 2, 3}}},
		{"HTML content", []model.Line{
			v030Open(t, "text", 2, 1), v030Text(t, "<em>a‐b</em>", 3, 2), v030Close(t, 3, 2),
		}, [][3]int{{4, 6, 7}}},
		{"table content", []model.Line{
			v030Open(t, "table", 2, 1), v030Text(t, "a‐b|c‑d", 3, 2), v030Close(t, 3, 2),
		}, [][3]int{{4, 2, 3}, {4, 6, 7}}},
		{"offset inside physical line", []model.Line{
			v030Line(t, "\ta‐b\t", "content", 2, 1, v030Element(t, "content", "a‐b", "a‐b", 2, 5)),
		}, [][3]int{{3, 3, 4}}},
		{"blank line", []model.Line{v030Line(t, " \t", "blank", 2, 1)}, nil},
		{"media path is excluded", []model.Line{
			v030Line(t, "@media image a‐b.png", "tag", 2, 1,
				v030Element(t, "tag", "@media", "media", 1, 7),
				v030Element(t, "media-type", "image", "image", 8, 13),
				v030Element(t, "source", "a‐b.png", "a‐b.png", 14, 21)),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v030Run(t, v030Document(t, tc.body...), "current", tc.want)
		})
	}
	// Согласованные элементы предзаполненного multifill: два текстовых
	// фрагмента разделены обычным плейсхолдером без ответа.
	gap := v030Element(t, "placeholder", "_____", "", 4, 9)
	gap.Value = nil
	v030Run(t, v030Document(t, v030Open(t, "multifill", 2, 1),
		v030Line(t, "a‐b_____c‑d", "content", 3, 2,
			v030Element(t, "content", "a‐b", "a‐b", 1, 4), gap,
			v030Element(t, "content", "c‑d", "c‑d", 9, 12)),
		v030Close(t, 3, 2)), "current", [][3]int{{4, 2, 3}, {4, 10, 11}})
}

func TestV030HeadingAndMetadata(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		result := v030Document(t, v030Text(t, "Text.", 2, 1))
		title := "a‐b"
		result.Document.Metadata.Title = &title
		result.Lines[1].Raw = "# a‐b"
		result.Lines[1].Elements[1] = v030Element(t, "title", "a‐b", "a‐b", 3, 6)
		v030Run(t, result, "current", [][3]int{{2, 4, 5}})
	})
	for _, tc := range []struct {
		name, raw, tag, kind, value string
		tagEnd, start, end          int
		want                        [][3]int
	}{
		{"name", "@section a‐b", "section", "name", "a‐b", 9, 10, 13, [][3]int{{2, 11, 12}}},
		{"resource path", "@resource-dir a‐b", "resource-dir", "resource-path", "a‐b", 14, 15, 18, nil},
		{"identifier", "@document-id unit-1", "document-id", "identifier", "unit-1", 13, 14, 20, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v030Document(t)
			metadata := v030Line(t, tc.raw, "tag", 0, 0,
				v030Element(t, "tag", "@"+tc.tag, tc.tag, 1, tc.tagEnd),
				v030Element(t, model.ElementType(tc.kind), tc.value, tc.value, tc.start, tc.end))
			result.Lines = []model.Line{result.Lines[0], metadata, result.Lines[1]}
			for i := range result.Lines {
				result.Lines[i].Number = i + 1
			}
			*result.Document.LineCount = 3
			switch tc.kind {
			case "name":
				result.Document.Metadata.Section = &tc.value
			case "resource-path":
				result.Document.Metadata.ResourceDirs = []string{tc.value}
			case "identifier":
				result.Document.Metadata.DocumentID = &tc.value
			}
			v030Run(t, result, "current", tc.want)
		})
	}
}

func TestV030Unparsed(t *testing.T) {
	bad := v030Element(t, "unparsed", "@bad‐", "", 1, 6)
	bad.Value, bad.ErrorIDs = nil, []string{"d1"}
	result := v030Document(t, v030Line(t, "@bad‐", "invalid", 2, 1, bad))
	result.Document.HasErrors, result.Lines[2].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{
		ID: "d1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		Message: "Неизвестный тег.", DiagnosticScope: "element",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 6}},
		RelatedLocations: []model.Location{},
	}}
	v030Run(t, result, "current", [][3]int{{3, 5, 6}})
}

func TestV030LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
		{"mixed", "\r\n", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v030Document(t, v030Text(t, "a‐b", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[2].LineEnding = tc.last
			v030Run(t, result, "current", [][3]int{{3, 2, 3}})
		})
	}
}

func TestV030UnavailableAndUnsupportedVersions(t *testing.T) {
	v030Run(t, v030UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v030UnavailableDocument(t, version)
			before := v030Snapshot(t, result)
			checks, err := validator.Prepare(result, v030Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v030Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV030HistoryAndOtherChecks(t *testing.T) {
	result := v030Document(t, v030Text(t, "A  \t“a‐b”", 2, 1))
	want := [][3]int{{3, 7, 8}}
	v030Run(t, result, "first", want)
	v030Run(t, result, "second", want)
	for _, source := range []string{"first", "second"} {
		for _, tc := range []struct {
			code       diagnostics.Code
			start, end int
		}{
			{"V001", 2, 4}, {"V002", 4, 5}, {"V009", 5, 6}, {"V009", 9, 10},
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
	v030Run(t, v030Document(t, v030Text(t, "Clean.", 2, 1)), "first", nil)
}

// v030Open создаёт явно заданное объявление блока без параметров.
func v030Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v030Line(t, "@"+tag+" {", "block-start", parent, depth,
		v030Element(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v030Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v030Close создаёт закрывающую строку с заданным родительским блоком.
func v030Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v030Line(t, "}", "block-end", parent, depth, v030Element(t, "block-close", "}", "}", 1, 2))
}

func TestV030Registration(t *testing.T) {
	definitions, err := v030Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V030" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Message != "Использован нестандартный знак дефиса или тире." ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V030 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V030 definitions = %d, want 1", matches)
	}
}

// v030UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v030UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v030Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v030Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v030Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v030Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v030Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v030Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v030Element(t, "version", "1.2", "1.2", 14, 17)),
		v030Line(t, "# Unit 1", "heading", 0, 0,
			v030Element(t, "heading-level", "#", "1", 1, 2),
			v030Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v030Line собирает строку с явно заданной структурой, без классификации Raw.
func v030Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v030Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v030Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v030Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v030Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v030Line(t, text, "content", parent, depth,
		v030Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v030Run проверяет штатный конвейер и явные тройки: строка, начало, конец.
// Правая граница исключается. Проверяются только V030 текущего запуска;
// прежние результаты и исходные строки должны сохраниться без изменений.
func v030Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v030Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v030Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v030Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V030" && entry.Source != processingID {
			t.Fatalf("new V030 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V030" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V030 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[0], Column: expected[2]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V030" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.Message != "Использован нестандартный знак дефиса или тире." || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	v030AssertPreserved(t, before, result)
}

// v030Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v030Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v030AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V030 заданы явно выше.
func v030AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v030Snapshot(t, result)), &restored); err != nil {
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
	if v030Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

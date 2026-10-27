package checkers

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV002Registration(t *testing.T) {
	definitions, err := v002Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V002" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Message != "В строке есть табуляция." ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V002 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V002 definitions = %d, want 1", matches)
	}
}

func TestV002TextSequences(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Read this.", nil},
		// &#9; в таблице представляет U+0009, а не текст HTML-сущности.
		{"example", "Read\tthis.", [][3]int{{3, 5, 6}}},
		{"two tabs form one occurrence", "Read\t\tthis.", [][3]int{{3, 5, 7}}},
		{"three tabs form one occurrence", "Read\t\t\tthis.", [][3]int{{3, 5, 8}}},
		{"text separates runs", "A\tB\t\tC", [][3]int{{3, 2, 3}, {3, 4, 6}}},
		{"space separates runs", "A\t \tB", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"punctuation separates runs", "A\t.\tB", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"literal backslash t", `Read\tthis.`, nil},
		{"literal HTML entity", "Read&#9;this.", nil},
		{"ordinary spaces", "Read  this.", nil},
		{"nonbreaking spaces", "Read\u00a0\u00a0this.", nil},
		{"em spaces", "Read\u2003\u2003this.", nil},
		{"nonbreaking space separates runs", "A\t\u00a0\tB", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"Unicode columns", "Я😀\tБ", [][3]int{{3, 3, 4}}},
		{"combining mark is a separate column", "e\u0301\tX", [][3]int{{3, 3, 4}}},
		{"case does not affect tabs", "READ\tTHIS.", [][3]int{{3, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v002Run(t, v002Document(t, v002Text(t, tc.raw, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV002RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty physical line", []model.Line{v002Line(t, "", "blank", 2, 1)}, nil},
		{"blank space", []model.Line{v002Line(t, " ", "blank", 2, 1)}, nil},
		{"blank tab", []model.Line{v002Line(t, "\t", "blank", 2, 1)}, [][3]int{{3, 1, 2}}},
		{"blank tab run", []model.Line{v002Line(t, "\t\t", "blank", 2, 1)}, [][3]int{{3, 1, 3}}},
		{"blank separated tabs", []model.Line{v002Line(t, "\t \t", "blank", 2, 1)}, [][3]int{{3, 1, 2}, {3, 3, 4}}},
		{"tabs outside elements at both edges", []model.Line{
			v002Line(t, "\tA\t\t", "content", 2, 1, v002Element(t, "content", "A", "A", 2, 3)),
		}, [][3]int{{3, 1, 2}, {3, 3, 5}}},
		{"runs never join across lines", []model.Line{
			v002Line(t, "A\t", "content", 2, 1, v002Element(t, "content", "A", "A", 1, 2)),
			v002Line(t, "\tB", "content", 2, 1, v002Element(t, "content", "B", "B", 2, 3)),
		}, [][3]int{{3, 2, 3}, {4, 1, 2}}},
		{"single-line editor comment", []model.Line{
			v002Line(t, "@editor A\tB", "tag", 2, 1,
				v002Element(t, "tag", "@editor", "editor", 1, 8),
				v002Element(t, "content", "A\tB", "A\tB", 9, 12)),
		}, [][3]int{{3, 10, 11}}},
		{"uppercase editor tag", []model.Line{
			v002Line(t, "@EDITOR A\tB", "tag", 2, 1,
				v002Element(t, "tag", "@EDITOR", "editor", 1, 8),
				v002Element(t, "content", "A\tB", "A\tB", 9, 12)),
		}, [][3]int{{3, 10, 11}}},
		{"block editor comment", []model.Line{
			v002Line(t, "@editor {", "block-start", 2, 1,
				v002Element(t, "tag", "@editor", "editor", 1, 8), v002Element(t, "block-open", "{", "{", 9, 10)),
			v002Text(t, "A\tB", 3, 2),
			v002Line(t, "}", "block-end", 3, 2, v002Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 2, 3}}},
		// DSL 3.2.8: обе части блока вокруг разделителя непустые.
		{"block boundaries and separator", []model.Line{
			v002Line(t, "@example {\t", "block-start", 2, 1,
				v002Element(t, "tag", "@example", "example", 1, 9), v002Element(t, "block-open", "{", "{", 10, 11)),
			v002Text(t, "A", 3, 2),
			v002Line(t, "\t---\t", "separator", 3, 2),
			v002Text(t, "B", 3, 2),
			v002Line(t, "}\t", "block-end", 3, 2, v002Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{3, 11, 12}, {5, 1, 2}, {5, 5, 6}, {7, 2, 3}}},
		// DSL 3.8.6, 3.8.11: собственный текст инструкции и один вложенный @hint.
		{"nested hint in instruction", []model.Line{
			v002Line(t, "@instr {", "block-start", 2, 1,
				v002Element(t, "tag", "@instr", "instr", 1, 7), v002Element(t, "block-open", "{", "{", 8, 9)),
			v002Text(t, "Read the sentence.", 3, 2),
			v002Line(t, "@hint {", "block-start", 3, 2,
				v002Element(t, "tag", "@hint", "hint", 1, 6), v002Element(t, "block-open", "{", "{", 7, 8)),
			v002Text(t, "Use\tcontext.", 5, 3),
			v002Line(t, "}", "block-end", 5, 3, v002Element(t, "block-close", "}", "}", 1, 2)),
			v002Line(t, "}", "block-end", 3, 2, v002Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{6, 4, 5}}},
		// DSL 4.4.6, 4.4.10: предзаполненное содержимое включает пустые строки.
		{"multifill content and blank line", []model.Line{
			v002Line(t, "@multifill {", "block-start", 2, 1,
				v002Element(t, "tag", "@multifill", "multifill", 1, 11), v002Element(t, "block-open", "{", "{", 12, 13)),
			v002Text(t, "A\tB", 3, 2),
			v002Line(t, "\t", "blank", 3, 2),
			v002Line(t, "}", "block-end", 3, 2, v002Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 2, 3}, {5, 1, 2}}},
		// DSL 1.21–1.22: Value снимает экранирование, Raw сохраняет исходные колонки.
		{"escaped braces preserve source columns", []model.Line{
			v002Line(t, "Use \\{x\\}\tnow.", "content", 2, 1,
				v002Element(t, "content", "Use \\{x\\}\tnow.", "Use {x}\tnow.", 1, 15)),
		}, [][3]int{{3, 10, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v002Run(t, v002Document(t, tc.body...), "current", tc.want, 1)
		})
	}
}

func TestV002InvalidUnparsedLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"tab in unparsed", "@note\tText", [][3]int{{3, 6, 7}}},
		{"tab inside and outside unparsed", "@note\tText\t", [][3]int{{3, 6, 7}, {3, 11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.29: табуляция не заменяет обязательный U+0020 после @note.
			// Код сохранённой ошибки непрозрачен для валидатора; это тестовая история.
			line := v002Line(t, tc.raw, "invalid", 2, 1, model.Element{
				ElementType: "unparsed", Raw: "@note\tText", Value: nil,
				Start: 1, End: 11, ErrorIDs: []string{"parser-error"},
			})
			line.HasErrors = true
			result := v002Document(t, line)
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{
				ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Некорректное объявление @note.",
				Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 11}},
				RelatedLocations: []model.Location{},
			}}
			v002Run(t, result, "current", tc.want, 1)
		})
	}
}

func TestV002FirstAndLastLines(t *testing.T) {
	result := v002Document(t, v002Line(t, "A\t", "content", 2, 1, v002Element(t, "content", "A", "A", 1, 2)))
	result.Lines[0].Raw = "@dsl-version 1.2\t"
	result.Lines[2].LineEnding = model.LineEndingNone
	v002Run(t, result, "current", [][3]int{{1, 17, 18}, {3, 2, 3}}, 1)
}

func TestV002Heading(t *testing.T) {
	result := v002Document(t, v002Text(t, "Read this.", 2, 1))
	result.Lines[1].Raw = "# Unit\t1"
	result.Lines[1].Elements[1] = v002Element(t, "title", "Unit\t1", "Unit\t1", 3, 9)
	title := "Unit\t1"
	result.Document.Metadata.Title = &title
	v002Run(t, result, "current", [][3]int{{2, 7, 8}}, 1)
}

func TestV002LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}},
		{"LF without final EOL", [3]model.LineEnding{"\n", "\n", ""}},
		{"CRLF without final EOL", [3]model.LineEnding{"\r\n", "\r\n", ""}},
		{"mixed endings", [3]model.LineEnding{"\r\n", "\n", "\r\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v002Document(t, v002Text(t, "Read\tthis.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v002Run(t, result, "current", [][3]int{{3, 5, 6}}, 1)
		})
	}
}

func TestV002UnavailableLines(t *testing.T) {
	// Допустимое отсутствие доступных строк с известной версией, а не выдуманный DSL.
	v002Run(t, v002UnavailableDocument(t, "1.2"), "current", nil, 0)
}

func TestV002UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v002UnavailableDocument(t, version)
			before := v002Snapshot(t, result)
			checks, err := validator.Prepare(result, v002Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v002Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV002CoexistsWithV001(t *testing.T) {
	result := v002Document(t, v002Text(t, "A\t  \tB", 2, 1))
	v002Run(t, result, "current", [][3]int{{3, 2, 3}, {3, 5, 6}}, 1)
	matches := 0
	want := model.Location{Start: model.Position{Line: 3, Column: 3}, End: model.Position{Line: 3, Column: 5}}
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode != "V001" || entry.Source != "current" {
			continue
		}
		matches++
		if entry.Location == nil || *entry.Location != want || entry.Message != "Несколько пробелов подряд." ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("unexpected V001 occurrence: %+v", entry)
		}
	}
	if matches != 1 {
		t.Fatalf("current V001 count = %d, want 1", matches)
	}
}

func TestV002RepeatedRunsPreserveHistory(t *testing.T) {
	result := v002Document(t, v002Text(t, "Read\tthis.", 2, 1))
	v002Run(t, result, "first", [][3]int{{3, 5, 6}}, 1)
	v002Run(t, result, "second", [][3]int{{3, 5, 6}}, 1)
	// Новый документ не наследует срабатывания предыдущих запусков.
	v002Run(t, v002Document(t, v002Text(t, "Read this.", 2, 1)), "first", nil, 1)
}

// v002UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v002UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v002Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v002Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v002Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v002Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v002Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v002Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v002Element(t, "version", "1.2", "1.2", 14, 17)),
		v002Line(t, "# Unit 1", "heading", 0, 0,
			v002Element(t, "heading-level", "#", "1", 1, 2),
			v002Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v002Line собирает строку с явно заданной структурой, без классификации Raw.
func v002Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v002Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v002Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v002Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v002Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v002Line(t, text, "content", parent, depth,
		v002Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v002Run проверяет штатный конвейер и явные тройки: строка, начало, конец.
// Правая граница исключается. Проверяются только V002 текущего запуска;
// прежние результаты, строки и ссылки должны сохраниться без изменений.
// wantV037 задаёт точное число сопутствующих ошибок отсутствующего ID;
// флаг документа проверяется отдельно, строки и прежняя история сохраняются.
func v002Run(t *testing.T, result *model.Result, processingID string, want [][3]int, wantV037 int) {
	t.Helper()
	before := v002Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v002Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v002Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	gotV037 := 0
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V002" && entry.Source != processingID {
			t.Fatalf("new V002 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V037" && entry.Source == processingID {
			gotV037++
			if entry.SeverityLevel != "error" || entry.DiagnosticScope != "document" ||
				entry.Message != "В документе отсутствует @document-id." || entry.Location != nil ||
				entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected accompanying V037: %+v", entry)
			}
		}
		if entry.DiagnosticCode == "V002" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V002 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V002" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "В строке есть табуляция." || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	if result.Document.HasErrors != (oldDocumentError || wantV037 > 0) {
		t.Errorf("document.hasErrors = %v, want %v", result.Document.HasErrors, oldDocumentError || wantV037 > 0)
	}
	previous := *result
	previous.Document.HasErrors = oldDocumentError
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	// В снимке исключаются новые ссылки других элементных диагностик каталога.
	newElementIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[len(previous.Diagnostics):] {
		if entry.DiagnosticScope == "element" && entry.DiagnosticCode != "V002" {
			newElementIDs[entry.ID] = true
		}
	}
	previous.Lines = append([]model.Line{}, previous.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].Elements = append([]model.Element{}, previous.Lines[i].Elements...)
		for j := range previous.Lines[i].Elements {
			element := &previous.Lines[i].Elements[j]
			element.ErrorIDs = slices.DeleteFunc(slices.Clone(element.ErrorIDs), func(id string) bool { return newElementIDs[id] })
		}
	}
	if v002Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v002Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v002Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

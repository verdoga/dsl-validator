package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV011Registration(t *testing.T) {
	definitions, err := v011Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V011" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Использовано длинное тире." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V011 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V011 definitions = %d, want 1", matches)
	}
}

func TestV011EveryEmDash(t *testing.T) {
	// Эталон и пример сохранены без изменений. v011Document добавляет только
	// обязательные версию и заголовок (DSL 1.1–1.3, 1.30, 2.1.3).
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"reference", "Москва – столица России.", nil},
		{"example", "Москва — столица России.", [][3]int{{3, 8, 9}}},
		{"no dash", "Read slowly.", nil},
		{"one character", "—", [][3]int{{3, 1, 2}}},
		{"two adjacent dashes", "——", [][3]int{{3, 1, 2}, {3, 2, 3}}},
		{"first middle last", "—a—b—", [][3]int{{3, 1, 2}, {3, 3, 4}, {3, 5, 6}}},
		{"between letters", "слово—слово", [][3]int{{3, 6, 7}}},
		{"between uppercase letters", "А—Б", [][3]int{{3, 2, 3}}},
		{"combining mark before dash", "е\u0301—б", [][3]int{{3, 3, 4}}},
		{"combining mark after dash", "а—\u0301б", [][3]int{{3, 2, 3}}},
		{"ASCII digits", "1—2", [][3]int{{3, 2, 3}}},
		{"Unicode digits", "١—٢", [][3]int{{3, 2, 3}}},
		{"decimal numbers", "1,5 — 2,5", [][3]int{{3, 5, 6}}},
		{"signed number", "1 — −2", [][3]int{{3, 3, 4}}},
		{"letters and numbers", "A1—2B", [][3]int{{3, 3, 4}}},
		{"nonbreaking spaces", "1\u00a0—\u00a02", [][3]int{{3, 3, 4}}},
		{"narrow nonbreaking spaces", "1\u202f—\u202f2", [][3]int{{3, 3, 4}}},
		{"tabs", "1\t—\t2", [][3]int{{3, 3, 4}}},
		{"Unicode before dash", "Я🙂—", [][3]int{{3, 3, 4}}},
		{"quotes", `"—"`, [][3]int{{3, 2, 3}}},
		{"backslash does not hide dash", `\—`, [][3]int{{3, 2, 3}}},
		{"other dash characters", "- – − ‐ ‑ ‒", nil},
		{"HTML entities are not decoded", "&mdash; &#8212; &#x2014;", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v011Run(t, v011Document(t, v011Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV011RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty physical line", []model.Line{v011Line(t, "", "blank", 2, 1)}, nil},
		{"whitespace line", []model.Line{v011Line(t, " \t ", "blank", 2, 1)}, nil},
		{"original indentation", []model.Line{
			v011Line(t, "  —  ", "content", 2, 1, v011Element(t, "content", "—", "—", 3, 4)),
		}, [][3]int{{3, 3, 4}}},
		{"tab counts as one column", []model.Line{
			v011Line(t, "\t—", "content", 2, 1, v011Element(t, "content", "—", "—", 2, 3)),
		}, [][3]int{{3, 2, 3}}},
		// DSL 1.21–1.22: Value теряет экранирующие символы, исходные колонки — нет.
		{"escaped braces preserve coordinates", []model.Line{
			v011Line(t, `\{—\}`, "content", 2, 1, v011Element(t, "content", `\{—\}`, "{—}", 1, 6)),
		}, [][3]int{{3, 3, 4}}},
		{"escaped backslash preserves coordinates", []model.Line{
			v011Line(t, `\\—`, "content", 2, 1, v011Element(t, "content", `\\—`, `\—`, 1, 4)),
		}, [][3]int{{3, 3, 4}}},
		{"inline comment with HTML-like text", []model.Line{
			v011Line(t, "@editor <x—y>", "tag", 2, 1,
				v011Element(t, "tag", "@editor", "editor", 1, 8), v011Element(t, "content", "<x—y>", "<x—y>", 9, 14)),
		}, [][3]int{{3, 11, 12}}},
		{"block comment", []model.Line{
			v011Open(t, "editor", 2, 1), v011Text(t, "<x—y>", 3, 2), v011Close(t, 3, 2),
		}, [][3]int{{4, 3, 4}}},
		{"nested content", []model.Line{
			v011Open(t, "note", 2, 1), v011Text(t, "—", 3, 2), v011Close(t, 3, 2),
		}, [][3]int{{4, 1, 2}}},
		// DSL 3.8, 3.11: подсказка разрешена внутри инструкции.
		{"nested hint", []model.Line{
			v011Open(t, "instr", 2, 1), v011Text(t, "Read.", 3, 2), v011Open(t, "hint", 3, 2),
			v011Text(t, "—", 5, 3), v011Close(t, 5, 3), v011Close(t, 3, 2),
		}, [][3]int{{6, 1, 2}}},
		{"block title and content", []model.Line{
			v011Line(t, "@note — {", "block-start", 2, 1,
				v011Element(t, "tag", "@note", "note", 1, 6), v011Element(t, "title", "—", "—", 7, 8),
				v011Element(t, "block-open", "{", "{", 9, 10)),
			v011Text(t, "—", 3, 2), v011Close(t, 3, 2),
		}, [][3]int{{3, 7, 8}, {4, 1, 2}}},
		// DSL 4.3, 4.4: служебный ответ представлен placeholder, а окружающий
		// текст — двумя content. Выбор типов не расширяется на ответ.
		{"text on both sides of placeholder", []model.Line{
			v011Open(t, "multifill", 2, 1),
			v011Line(t, "— _____{—} —", "content", 3, 2,
				v011Element(t, "content", "— ", "— ", 1, 3),
				v011Element(t, "placeholder", "_____{—}", "—", 3, 11),
				v011Element(t, "content", " —", " —", 11, 13)), v011Close(t, 3, 2),
		}, [][3]int{{4, 1, 2}, {4, 12, 13}}},
		{"placeholder alone is not selected", []model.Line{
			v011Open(t, "multifill", 2, 1),
			v011Line(t, "_____{—}", "content", 3, 2, v011Element(t, "placeholder", "_____{—}", "—", 1, 9)),
			v011Close(t, 3, 2),
		}, nil},
		{"consecutive lines", []model.Line{v011Text(t, "—", 2, 1), v011Text(t, "—", 2, 1)},
			[][3]int{{3, 1, 2}, {4, 1, 2}}},
		{"block separator", []model.Line{
			v011Open(t, "example", 2, 1), v011Text(t, "—", 3, 2), v011Line(t, "---", "separator", 3, 2),
			v011Text(t, "—", 3, 2), v011Close(t, 3, 2),
		}, [][3]int{{4, 1, 2}, {6, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v011Run(t, v011Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV011HeadingNameAndPaths(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v011Document(t)
		heading := v011Line(t, "# Unit —", "heading", 0, 0,
			v011Element(t, "heading-level", "#", "1", 1, 2), v011Element(t, "title", "Unit —", "Unit —", 3, 9))
		result = v011Model(t, result.Lines[0], heading)
		title := "Unit —"
		result.Document.Metadata.Title = &title
		v011Run(t, result, "current", [][3]int{{2, 8, 9}})
	})
	t.Run("section name", func(t *testing.T) {
		base := v011Document(t)
		section := v011Line(t, "@section A—B", "tag", 0, 0,
			v011Element(t, "tag", "@section", "section", 1, 9), v011Element(t, "name", "A—B", "A—B", 10, 13))
		result := v011Model(t, base.Lines[0], section, base.Lines[1])
		name := "A—B"
		result.Document.Metadata.Section = &name
		v011Run(t, result, "current", [][3]int{{2, 11, 12}})
	})
	t.Run("resource path excluded", func(t *testing.T) {
		base := v011Document(t)
		path := v011Line(t, "@resource-dir assets/—", "tag", 0, 0,
			v011Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v011Element(t, "resource-path", "assets/—", "assets/—", 15, 23))
		result := v011Model(t, base.Lines[0], path, base.Lines[1])
		result.Document.Metadata.ResourceDirs = []string{"assets/—"}
		v011Run(t, result, "current", nil)
	})
	t.Run("media source excluded", func(t *testing.T) {
		base := v011Document(t)
		path := v011Line(t, "@resource-dir .", "tag", 0, 0,
			v011Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v011Element(t, "resource-path", ".", ".", 15, 16))
		media := v011Line(t, `@media image "a—b.png"`, "tag", 3, 1,
			v011Element(t, "tag", "@media", "media", 1, 7), v011Element(t, "media-type", "image", "image", 8, 13),
			v011Element(t, "source", `"a—b.png"`, "a—b.png", 14, 23))
		result := v011Model(t, base.Lines[0], path, base.Lines[1], media)
		result.Document.Metadata.ResourceDirs = []string{"."}
		v011Run(t, result, "current", nil)
	})
	t.Run("path-looking text is selected", func(t *testing.T) {
		comment := v011Line(t, "@editor assets/—", "tag", 2, 1,
			v011Element(t, "tag", "@editor", "editor", 1, 8), v011Element(t, "content", "assets/—", "assets/—", 9, 17))
		v011Run(t, v011Document(t, comment), "current", [][3]int{{3, 16, 17}})
	})
}

func TestV011UnparsedIsNotSelected(t *testing.T) {
	// Неизвестный тег — ошибка DSL 1.16, 1.29; unparsed и исходная ошибка
	// согласованы по контракту. Соседняя нормальная строка проверяется независимо.
	line := v011Line(t, "@unknown —", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown —", Start: 1, End: 11, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	for _, tc := range []struct {
		name, next string
		want       [][3]int
	}{
		{"only unparsed dash", "Text.", nil},
		{"next line is checked", "—", [][3]int{{4, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v011Document(t, line, v011Text(t, tc.next, 2, 1))
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{
				ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный DSL-тег.",
				Location: &model.Location{Start: model.Position{Line: 3, Column: 1},
					End: model.Position{Line: 3, Column: 11}}, RelatedLocations: []model.Location{},
			}}
			v011Run(t, result, "current", tc.want)
		})
	}
}

func TestV011LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"reference", "Москва – столица России.", nil},
		{"example", "Москва — столица России.", [][3]int{{3, 8, 9}}},
	} {
		for _, ending := range []struct {
			name string
			eol  model.LineEnding
			last model.LineEnding
		}{
			{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
			{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
			{"mixed", "\r\n", "\n"},
		} {
			t.Run(tc.name+"/"+ending.name, func(t *testing.T) {
				result := v011Document(t, v011Text(t, tc.raw, 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending.eol
				}
				result.Lines[2].LineEnding = ending.last
				v011Run(t, result, "current", tc.want)
			})
		}
	}
}

func TestV011NoLinesAndVersionSelection(t *testing.T) {
	result := v011Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v011Run(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v011Document(t)
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v011Snapshot(t, result)
			checks, err := validator.Prepare(result, v011Registry(t))
			if err == nil || checks != nil || v011Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV011RepeatedRunsPreserveHistory(t *testing.T) {
	result := v011Document(t, v011Text(t, "—", 2, 1))
	v011Run(t, result, "first", [][3]int{{3, 1, 2}})
	v011Run(t, result, "second", [][3]int{{3, 1, 2}})
	v011Run(t, v011Document(t, v011Text(t, "–", 2, 1)), "current", nil)
}

func TestV011CoexistsWithOtherChecks(t *testing.T) {
	result := v011Document(t, v011Text(t, "[— . . .", 2, 1),
		v011Line(t, "", "blank", 2, 1), v011Line(t, "", "blank", 2, 1), v011Text(t, "End.", 2, 1))
	v011Run(t, result, "current", [][3]int{{3, 2, 3}})
	for _, tc := range []struct {
		code     diagnostics.Code
		severity diagnostics.Severity
		scope    diagnostics.Scope
		message  string
		location model.Location
	}{
		{"V003", "warning", "line", "Точки многоточия разделены пробельными символами.",
			model.Location{Start: model.Position{Line: 3, Column: 4}, End: model.Position{Line: 3, Column: 9}}},
		{"V004", "recommendation", "block", "Несколько пустых строк подряд.",
			model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 5, Column: 1}}},
		{"V008", "warning", "line", "Нарушена парность квадратных скобок в строке.\nконкретизация: Открывающая квадратная скобка не закрыта в этой строке",
			model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 2}}},
	} {
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "current" || entry.DiagnosticCode != tc.code {
				continue
			}
			count++
			if entry.Location == nil || *entry.Location != tc.location || entry.Message != tc.message ||
				entry.SeverityLevel != tc.severity || entry.DiagnosticScope != tc.scope || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
		if count != 1 {
			t.Errorf("%s count = %d, want 1", tc.code, count)
		}
	}
}

// v011Registry создаёт реестр только через штатный каталог регистраций.
func v011Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v011Document добавляет обязательные версию и заголовок к явно заданному телу.
func v011Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v011Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v011Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v011Element(t, "version", "1.2", "1.2", 14, 17)),
		v011Line(t, "# Unit 1", "heading", 0, 0,
			v011Element(t, "heading-level", "#", "1", 1, 2), v011Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v011Model(t, append(lines, body...)...)
}

// v011Model задаёт физические номера, не выводя типы и структуру из Raw.
func v011Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, title := "1.2", "UTF-8", "Unit 1"
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	return result
}

// v011Line сохраняет явно заданную структуру строки без разбора Raw.
func v011Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v011Element задаёт исходное и семантическое значения и диапазон [start,end).
func v011Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v011Text задаёт текст с одинаковыми Raw и Value; длина нужна только для
// входного элемента и не используется для вычисления ожидаемых срабатываний.
func v011Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v011Line(t, raw, "content", parent, depth,
		v011Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v011Open задаёт блочное объявление без параметров с согласованными элементами.
func v011Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v011Line(t, "@"+tag+" {", "block-start", parent, depth,
		v011Element(t, "tag", "@"+tag, tag, 1, end), v011Element(t, "block-open", "{", "{", end+1, end+2))
}

// v011Close задаёт закрытие с явно указанными родителем и глубиной.
func v011Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v011Line(t, "}", "block-end", parent, depth, v011Element(t, "block-close", "}", "}", 1, 2))
}

// v011Run проверяет V011 текущего запуска штатного конвейера: точное число,
// метаданные и диапазоны [start,end), неизменность исходной модели и истории.
func v011Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v011Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v011Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v011Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V011" && entry.Source != processingID {
			t.Fatalf("new V011 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V011" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V011 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "Использовано длинное тире.\nконкретизация: Для этой позиции рекомендуется короткое тире – U+2013" ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, location)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing or diagnostic history length")
	}
	previous := *result
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v011Snapshot(t, &previous) != before {
		t.Fatal("source model, flags, element links or previous history changed")
	}
}

// v011Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v011Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

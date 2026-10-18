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

func TestV026Registration(t *testing.T) {
	definitions, err := v026Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V026" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "Одинаковый знак препинания повторён подряд." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V026 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V026 definitions = %d, want 1", matches)
	}
}

func TestV026Runs(t *testing.T) {
	// Эталон и пример сохранены буквально. Документ дополняется только
	// обязательными версией и заголовком (DSL 1.1–1.3, 1.30, 2.1.3).
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"reference", "Wait... Really?!", nil},
		{"example", "Hello,, world!!", [][3]int{{3, 6, 8}, {3, 14, 16}}},
		{"no punctuation", "Read slowly", nil},
		{"single marks", ". , ; : ! ? …", nil},
		{"all seven marks", "..,,;;::!!??……", [][3]int{{3, 1, 3}, {3, 3, 5}, {3, 5, 7},
			{3, 7, 9}, {3, 9, 11}, {3, 11, 13}, {3, 13, 15}}},
		{"one dot", ".", nil},
		{"two dots", "..", [][3]int{{3, 1, 3}}},
		{"three dots", "...", nil},
		{"four dots", "....", [][3]int{{3, 1, 5}}},
		{"five dots", ".....", [][3]int{{3, 1, 6}}},
		{"six dots", "......", [][3]int{{3, 1, 7}}},
		{"seven dots", ".......", [][3]int{{3, 1, 8}}},
		{"three ellipsis characters", "………", [][3]int{{3, 1, 4}}},
		{"mixed single signs", "?! !? ?!?!", nil},
		{"adjacent different runs", "!!??", [][3]int{{3, 1, 3}, {3, 3, 5}}},
		{"mixed dots and ellipsis", "...……..", [][3]int{{3, 4, 6}, {3, 6, 8}}},
		{"long exclamation run", "!!!!!!", [][3]int{{3, 1, 7}}},
		{"single different sign splits run", "!!?!!", [][3]int{{3, 1, 3}, {3, 4, 6}}},
		{"separated allowed ellipses", "... ...", nil},
		{"separated repeated dots", ".. ..", [][3]int{{3, 1, 3}, {3, 4, 6}}},
		{"space splits", "! !", nil},
		{"tab splits", "!\t!", nil},
		{"NBSP splits", "!\u00a0!", nil},
		{"combining mark splits", "!\u0301!", nil},
		{"backslash splits", `!\!`, nil},
		{"backslash does not hide run", `\!!`, [][3]int{{3, 2, 4}}},
		{"first and last", "!!текст??", [][3]int{{3, 1, 3}, {3, 8, 10}}},
		{"Unicode columns", "Я🙂!!", [][3]int{{3, 3, 5}}},
		{"combining mark column", "е\u0301!!", [][3]int{{3, 3, 5}}},
		{"other punctuation not selected", "－－，，？？", nil},
		{"letter case irrelevant", "READ!!read??", [][3]int{{3, 5, 7}, {3, 11, 13}}},
		{"HTML-like marker", "<x!!>", [][3]int{{3, 3, 5}}},
		{"phonetic notation", "/a::/", [][3]int{{3, 3, 5}}},
		{"address", "https://a..b", [][3]int{{3, 10, 12}}},
		{"entities not decoded", "&#33;&#33;", nil},
		{"quoted punctuation", `"!!"`, [][3]int{{3, 2, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v026Run(t, v026Document(t, v026Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV026Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty and whitespace rows", []model.Line{v026Line(t, "", "blank", 2, 1), v026Line(t, " \t ", "blank", 2, 1)}, nil},
		{"original indentation", []model.Line{v026Line(t, " \t!! ", "content", 2, 1,
			v026Element(t, "content", "!!", "!!", 3, 5))}, [][3]int{{3, 3, 5}}},
		{"different Raw and Value", []model.Line{v026Line(t, `\{!!\}`, "content", 2, 1,
			v026Element(t, "content", `\{!!\}`, "{!!}", 1, 7))}, [][3]int{{3, 3, 5}}},
		{"inline comment", []model.Line{v026Line(t, "@editor !!", "tag", 2, 1,
			v026Element(t, "tag", "@editor", "editor", 1, 8), v026Element(t, "content", "!!", "!!", 9, 11))}, [][3]int{{3, 9, 11}}},
		{"block comment", []model.Line{v026Open(t, "editor", 2, 1), v026Text(t, "!!", 3, 2), v026Close(t, 3, 2)}, [][3]int{{4, 1, 3}}},
		{"nested content", []model.Line{v026Open(t, "note", 2, 1), v026Text(t, "!!", 3, 2), v026Close(t, 3, 2)}, [][3]int{{4, 1, 3}}},
		{"nested hint", []model.Line{v026Open(t, "instr", 2, 1), v026Text(t, "Read.", 3, 2), v026Open(t, "hint", 3, 2),
			v026Text(t, "!!", 5, 3), v026Close(t, 5, 3), v026Close(t, 3, 2)}, [][3]int{{6, 1, 3}}},
		{"table content", []model.Line{v026Open(t, "table", 2, 1), v026Text(t, "!!", 3, 2), v026Close(t, 3, 2)}, [][3]int{{4, 1, 3}}},
		{"text HTML content", []model.Line{v026Open(t, "text", 2, 1), v026Text(t, "<em>!!</em>", 3, 2), v026Close(t, 3, 2)},
			[][3]int{{4, 5, 7}}},
		{"placeholder answer", []model.Line{v026Open(t, "multifill", 2, 1),
			v026Line(t, "_____{a!!}", "content", 3, 2, v026Element(t, "placeholder", "_____{a!!}", "a!!", 1, 11)),
			v026Close(t, 3, 2)}, [][3]int{{4, 8, 10}}},
		{"block title", []model.Line{v026Line(t, "@note !! {", "block-start", 2, 1,
			v026Element(t, "tag", "@note", "note", 1, 6), v026Element(t, "title", "!!", "!!", 7, 9),
			v026Element(t, "block-open", "{", "{", 10, 11)), v026Text(t, "Body.", 3, 2), v026Close(t, 3, 2)}, [][3]int{{3, 7, 9}}},
		{"separate blocks", []model.Line{v026Open(t, "note", 2, 1), v026Text(t, "!", 3, 2), v026Close(t, 3, 2),
			v026Open(t, "note", 2, 1), v026Text(t, "!", 6, 2), v026Close(t, 6, 2)}, nil},
		{"block separator", []model.Line{v026Open(t, "example", 2, 1), v026Text(t, "!!", 3, 2),
			v026Line(t, "---", "separator", 3, 2), v026Text(t, "??", 3, 2), v026Close(t, 3, 2)}, [][3]int{{4, 1, 3}, {6, 1, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v026Run(t, v026Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV026ServiceFields(t *testing.T) {
	t.Run("resource path", func(t *testing.T) {
		base := v026Document(t)
		path := v026Line(t, "@resource-dir a..b", "tag", 0, 0, v026Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v026Element(t, "resource-path", "a..b", "a..b", 15, 19))
		result := v026Model(t, base.Lines[0], path, base.Lines[1])
		result.Document.Metadata.ResourceDirs = []string{"a..b"}
		v026Run(t, result, "current", [][3]int{{2, 16, 18}})
	})
	t.Run("media source", func(t *testing.T) {
		base := v026Document(t)
		path := v026Line(t, "@resource-dir .", "tag", 0, 0,
			v026Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v026Element(t, "resource-path", ".", ".", 15, 16))
		line := v026Line(t, `@media image "a..b.png"`, "tag", 3, 1,
			v026Element(t, "tag", "@media", "media", 1, 7), v026Element(t, "media-type", "image", "image", 8, 13),
			v026Element(t, "source", `"a..b.png"`, "a..b.png", 14, 24))
		result := v026Model(t, base.Lines[0], path, base.Lines[1], line)
		result.Document.Metadata.ResourceDirs = []string{"."}
		v026Run(t, result, "current", [][3]int{{4, 16, 18}})
	})
	t.Run("heading title", func(t *testing.T) {
		base := v026Document(t)
		heading := v026Line(t, "# Unit!!", "heading", 0, 0,
			v026Element(t, "heading-level", "#", "1", 1, 2), v026Element(t, "title", "Unit!!", "Unit!!", 3, 9))
		result := v026Model(t, base.Lines[0], heading)
		title := "Unit!!"
		result.Document.Metadata.Title = &title
		v026Run(t, result, "current", [][3]int{{2, 7, 9}})
	})
	t.Run("section name", func(t *testing.T) {
		base := v026Document(t)
		line := v026Line(t, "@section A..B", "tag", 0, 0,
			v026Element(t, "tag", "@section", "section", 1, 9), v026Element(t, "name", "A..B", "A..B", 10, 14))
		result := v026Model(t, base.Lines[0], line, base.Lines[1])
		name := "A..B"
		result.Document.Metadata.Section = &name
		v026Run(t, result, "current", [][3]int{{2, 11, 13}})
	})
}

func TestV026Unparsed(t *testing.T) {
	// Неизвестный тег — ошибка DSL 1.16. Модель сохраняет unparsed,
	// исходную диагностику и её связи; V026 проверяет Raw независимо от неё.
	line := v026Line(t, "@unknown !!", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown !!", Start: 1, End: 12, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	result := v026Document(t, line, v026Text(t, "!!", 2, 1))
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный DSL-тег.",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 12}},
		RelatedLocations: []model.Location{},
	}}
	v026Run(t, result, "current", [][3]int{{3, 10, 12}, {4, 1, 3}})
}

func TestV026LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][3]int
	}{
		{"no cross line match", "!", "!", nil},
		{"dots do not form cross line ellipsis", "..", ".", [][3]int{{3, 1, 3}}},
		{"runs on both lines", "!!", "!!", [][3]int{{3, 1, 3}, {4, 1, 3}}},
		{"reference", "Wait... Really?!", "Done.", nil},
		{"example", "Hello,, world!!", "Done.", [][3]int{{3, 6, 8}, {3, 14, 16}}},
	} {
		for _, ending := range []struct {
			name      string
			eol, last model.LineEnding
		}{
			{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
			{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""}, {"mixed", "\r\n", "\n"},
		} {
			t.Run(tc.name+"/"+ending.name, func(t *testing.T) {
				result := v026Document(t, v026Text(t, tc.first, 2, 1), v026Text(t, tc.second, 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending.eol
				}
				result.Lines[3].LineEnding = ending.last
				v026Run(t, result, "current", tc.want)
			})
		}
	}
}

func TestV026NoLinesAndVersions(t *testing.T) {
	result := v026Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v026Run(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v026Document(t, v026Text(t, "!!", 2, 1))
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v026Snapshot(t, result)
			checks, err := validator.Prepare(result, v026Registry(t))
			if err == nil || checks != nil || v026Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV026HistoryAndOtherChecks(t *testing.T) {
	result := v026Document(t, v026Text(t, "!! . . .", 2, 1))
	v026Run(t, result, "first", [][3]int{{3, 1, 3}})
	v026Run(t, result, "second", [][3]int{{3, 1, 3}})
	v026Run(t, v026Document(t, v026Text(t, "...", 2, 1)), "current", nil)
	for _, source := range []string{"first", "second"} {
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != source || entry.DiagnosticCode != "V003" {
				continue
			}
			count++
			want := model.Location{Start: model.Position{Line: 3, Column: 4}, End: model.Position{Line: 3, Column: 9}}
			if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" ||
				entry.Message != "Точки многоточия разделены пробельными символами." || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected V003: %+v", entry)
			}
		}
		if count != 1 {
			t.Errorf("V003 count for %s = %d, want 1", source, count)
		}
	}
}

// v026Registry создаёт реестр только через штатный каталог регистраций.
func v026Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v026Document добавляет обязательные версию и заголовок к явно заданному телу.
func v026Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v026Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v026Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v026Element(t, "version", "1.2", "1.2", 14, 17)),
		v026Line(t, "# Unit 1", "heading", 0, 0,
			v026Element(t, "heading-level", "#", "1", 1, 2), v026Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v026Model(t, append(lines, body...)...)
}

// v026Model задаёт физические номера, не выводя типы и структуру из Raw.
func v026Model(t *testing.T, lines ...model.Line) *model.Result {
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

// v026Line сохраняет явно заданную структуру строки без разбора Raw.
func v026Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v026Element задаёт исходное и семантическое значения и диапазон [start,end).
func v026Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v026Text задаёт текст с одинаковыми Raw и Value; длина нужна только для
// входного элемента и не используется для вычисления ожидаемых срабатываний.
func v026Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v026Line(t, raw, "content", parent, depth,
		v026Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v026Open задаёт блочное объявление без параметров с согласованными элементами.
func v026Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v026Line(t, "@"+tag+" {", "block-start", parent, depth,
		v026Element(t, "tag", "@"+tag, tag, 1, end), v026Element(t, "block-open", "{", "{", end+1, end+2))
}

// v026Close задаёт закрытие с явно указанными родителем и глубиной.
func v026Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v026Line(t, "}", "block-end", parent, depth, v026Element(t, "block-close", "}", "}", 1, 2))
}

// v026Run проверяет V026 текущего запуска штатного конвейера: точное число,
// метаданные и диапазоны [start,end), неизменность исходной модели и истории.
func v026Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v026Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v026Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v026Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V026" && entry.Source != processingID {
			t.Fatalf("new V026 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V026" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V026 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "Одинаковый знак препинания повторён подряд." ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, location)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing or diagnostic history length")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	v026OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v026Snapshot(t, &previous) != v026Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v026Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v026Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v026OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v026OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V026" {
			continue
		}
		if entry.SeverityLevel == "error" {
			expected.Document.HasErrors = true
			if entry.Location != nil {
				expected.Lines[entry.Location.Start.Line-1].HasErrors = true
			}
		}
		if entry.DiagnosticScope != "element" {
			continue
		}
		if entry.Location == nil {
			t.Fatal("element diagnostic has no location")
		}
		matched := false
		for i := range expected.Lines[entry.Location.Start.Line-1].Elements {
			element := &expected.Lines[entry.Location.Start.Line-1].Elements[i]
			if element.Start == entry.Location.Start.Column && element.End == entry.Location.End.Column {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched = true
				break
			}
		}
		if !matched {
			t.Fatal("element diagnostic does not identify an element")
		}
	}
	for i := range expected.Lines {
		for j := range expected.Lines[i].Elements {
			slices.SortStableFunc(expected.Lines[i].Elements[j].ErrorIDs, func(a, b string) int { return order[a] - order[b] })
		}
	}
}

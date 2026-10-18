package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// Тексты ожиданий заданы независимо от констант и функций реализации.
const (
	v008WantOpen  = "Открывающая квадратная скобка не закрыта в этой строке"
	v008WantClose = "Закрывающая квадратная скобка не имеет открывающей в этой строке"
)

// v008Expected хранит явно заданный диапазон [start,end) в Unicode-колонках с 1.
type v008Expected struct {
	line, start, end int
	detail           string
}

func TestV008Registration(t *testing.T) {
	definitions, err := v008Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V008" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "Нарушена парность квадратных скобок в строке." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V008 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V008 definitions = %d, want 1", matches)
	}
}

func TestV008Pairs(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v008Expected
	}{
		{"no brackets", "Read slowly.", nil},
		{"empty pair", "[]", nil},
		{"nested pairs", "[a[b]c]", nil},
		{"separate pairs", "[a] [b]", nil},
		{"adjacent pairs", "[][]", nil},
		{"opening", "[", []v008Expected{{3, 1, 2, v008WantOpen}}},
		{"closing", "]", []v008Expected{{3, 1, 2, v008WantClose}}},
		{"two openings in source order", "[[", []v008Expected{{3, 1, 2, v008WantOpen}, {3, 2, 3, v008WantOpen}}},
		{"three openings", "[[[", []v008Expected{{3, 1, 2, v008WantOpen}, {3, 2, 3, v008WantOpen}, {3, 3, 4, v008WantOpen}}},
		{"nearest opening closes", "[[]", []v008Expected{{3, 1, 2, v008WantOpen}}},
		{"nested unmatched openings", "[[[]", []v008Expected{{3, 1, 2, v008WantOpen}, {3, 2, 3, v008WantOpen}}},
		{"extra closing", "[]]", []v008Expected{{3, 3, 4, v008WantClose}}},
		{"scan continues after unmatched closing", "][]]", []v008Expected{{3, 1, 2, v008WantClose}, {3, 4, 5, v008WantClose}}},
		{"wrong order", "][", []v008Expected{{3, 1, 2, v008WantClose}, {3, 2, 3, v008WantOpen}}},
		{"several closings and openings", "]][[", []v008Expected{{3, 1, 2, v008WantClose}, {3, 2, 3, v008WantClose}, {3, 3, 4, v008WantOpen}, {3, 4, 5, v008WantOpen}}},
		{"closing then nested pair then openings", "][[][", []v008Expected{{3, 1, 2, v008WantClose}, {3, 2, 3, v008WantOpen}, {3, 5, 6, v008WantOpen}}},
		{"double quotes", `"["`, []v008Expected{{3, 2, 3, v008WantOpen}}},
		{"single quotes", `']'`, []v008Expected{{3, 2, 3, v008WantClose}}},
		{"backslash does not hide opening", `\[`, []v008Expected{{3, 2, 3, v008WantOpen}}},
		{"backslash does not hide closing", `\]`, []v008Expected{{3, 2, 3, v008WantClose}}},
		{"backslashes between matched brackets", `\[\]`, nil},
		{"tabs and spaces between matched brackets", "[ \t ]", nil},
		{"letter case does not matter", "READ [slowly", []v008Expected{{3, 6, 7, v008WantOpen}}},
		{"Unicode columns", "Я🙂[", []v008Expected{{3, 3, 4, v008WantOpen}}},
		{"combining character counts separately", "е\u0301]", []v008Expected{{3, 3, 4, v008WantClose}}},
		{"other Unicode brackets", "［word］【text】", nil},
		{"other brackets do not match ASCII", "［]", []v008Expected{{3, 2, 3, v008WantClose}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v008Run(t, v008Document(t, v008Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV008ReferencesAndLineEndings(t *testing.T) {
	// Содержимое эталона и примера не изменяется. v008Document добавляет
	// только обязательные версию и заголовок (DSL 1.1–1.3, 1.30, 2.1.3).
	for _, tc := range []struct {
		name, first, second string
		want                []v008Expected
	}{
		{"reference", "Read [the word [carefully]].", "Continue [slowly].", nil},
		{"example", "Read [the word.", "Continue].", []v008Expected{{3, 6, 7, v008WantOpen}, {4, 9, 10, v008WantClose}}},
		{"single bracket per line", "[", "]", []v008Expected{{3, 1, 2, v008WantOpen}, {4, 1, 2, v008WantClose}}},
		{"backslash before line ending", `text \`, "[", []v008Expected{{4, 1, 2, v008WantOpen}}},
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
				result := v008Document(t, v008Text(t, tc.first, 2, 1), v008Text(t, tc.second, 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending.eol
				}
				result.Lines[3].LineEnding = ending.last
				v008Run(t, result, "current", tc.want)
			})
		}
	}
}

func TestV008RawAndContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v008Expected
	}{
		{"blank lines", []model.Line{v008Line(t, "", "blank", 2, 1), v008Line(t, " \t ", "blank", 2, 1)}, nil},
		{"edges retain original columns", []model.Line{
			v008Line(t, "\t  [  ", "content", 2, 1, v008Element(t, "content", "[", "[", 4, 5)),
		}, []v008Expected{{3, 4, 5, v008WantOpen}}},
		// DSL 1.21–1.22: Value отличается от Raw после обработки экранирования.
		{"decoded backslash does not shift column", []model.Line{
			v008Line(t, `\\[`, "content", 2, 1, v008Element(t, "content", `\\[`, `\[`, 1, 4)),
		}, []v008Expected{{3, 3, 4, v008WantOpen}}},
		{"escaped brace does not shift column", []model.Line{
			v008Line(t, `\{[`, "content", 2, 1, v008Element(t, "content", `\{[`, `{[`, 1, 4)),
		}, []v008Expected{{3, 3, 4, v008WantOpen}}},
		{"inline comment", []model.Line{
			v008Line(t, "@editor [", "tag", 2, 1,
				v008Element(t, "tag", "@editor", "editor", 1, 8), v008Element(t, "content", "[", "[", 9, 10)),
		}, []v008Expected{{3, 9, 10, v008WantOpen}}},
		{"block comment", []model.Line{
			v008Open(t, "editor", 2, 1), v008Text(t, "[", 3, 2), v008Close(t, 3, 2),
		}, []v008Expected{{4, 1, 2, v008WantOpen}}},
		// DSL 3.8, 3.11: разрешённая подсказка внутри инструкции.
		{"nested hint", []model.Line{
			v008Open(t, "instr", 2, 1), v008Text(t, "Read.", 3, 2),
			v008Open(t, "hint", 3, 2), v008Text(t, "]", 5, 3), v008Close(t, 5, 3), v008Close(t, 3, 2),
		}, []v008Expected{{6, 1, 2, v008WantClose}}},
		// DSL 3.2.8: непустой пример и перевод по разные стороны разделителя.
		{"block separator", []model.Line{
			v008Open(t, "example", 2, 1), v008Text(t, "[", 3, 2),
			v008Line(t, "---", "separator", 3, 2), v008Text(t, "]", 3, 2), v008Close(t, 3, 2),
		}, []v008Expected{{4, 1, 2, v008WantOpen}, {6, 1, 2, v008WantClose}}},
		{"block boundary", []model.Line{
			v008Open(t, "note", 2, 1), v008Text(t, "[", 3, 2), v008Close(t, 3, 2), v008Text(t, "]", 2, 1),
		}, []v008Expected{{4, 1, 2, v008WantOpen}, {6, 1, 2, v008WantClose}}},
		{"blank line between brackets", []model.Line{
			v008Text(t, "[", 2, 1), v008Line(t, "", "blank", 2, 1), v008Text(t, "]", 2, 1),
		}, []v008Expected{{3, 1, 2, v008WantOpen}, {5, 1, 2, v008WantClose}}},
		{"multifill answer", []model.Line{
			v008Open(t, "multifill", 2, 1),
			v008Line(t, "_____{[word}", "content", 3, 2,
				v008Element(t, "placeholder", "_____{[word}", "[word", 1, 13)), v008Close(t, 3, 2),
		}, []v008Expected{{4, 7, 8, v008WantOpen}}},
		{"block title", []model.Line{
			v008Line(t, "@note [ {", "block-start", 2, 1,
				v008Element(t, "tag", "@note", "note", 1, 6), v008Element(t, "title", "[", "[", 7, 8),
				v008Element(t, "block-open", "{", "{", 9, 10)),
			v008Text(t, "Text.", 3, 2), v008Close(t, 3, 2),
		}, []v008Expected{{3, 7, 8, v008WantOpen}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v008Run(t, v008Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV008PathsAndHeading(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v008Expected
	}{
		{"resource path", "assets/[x", "assets/[x", []v008Expected{{2, 22, 23, v008WantOpen}}},
		{"quoted resource path", `"assets/[x"`, "assets/[x", []v008Expected{{2, 23, 24, v008WantOpen}}},
		{"literal backslash in resource path", `C:\[x`, `C:\[x`, []v008Expected{{2, 18, 19, v008WantOpen}}},
		{"paired brackets in resource path", "assets/[x]", "assets/[x]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := v008Document(t)
			path := v008Line(t, "@resource-dir "+tc.raw, "tag", 0, 0,
				v008Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
				v008Element(t, "resource-path", tc.raw, tc.value, 15, 15+utf8.RuneCountInString(tc.raw)))
			result := v008Model(t, base.Lines[0], path, base.Lines[1])
			result.Document.Metadata.ResourceDirs = []string{tc.value}
			v008Run(t, result, "current", tc.want)
		})
	}
	t.Run("media source", func(t *testing.T) {
		base := v008Document(t)
		path := v008Line(t, "@resource-dir .", "tag", 0, 0,
			v008Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v008Element(t, "resource-path", ".", ".", 15, 16))
		media := v008Line(t, `@media image "assets/x]"`, "tag", 3, 1,
			v008Element(t, "tag", "@media", "media", 1, 7),
			v008Element(t, "media-type", "image", "image", 8, 13),
			v008Element(t, "source", `"assets/x]"`, "assets/x]", 14, 25))
		result := v008Model(t, base.Lines[0], path, base.Lines[1], media)
		result.Document.Metadata.ResourceDirs = []string{"."}
		v008Run(t, result, "current", []v008Expected{{4, 23, 24, v008WantClose}})
	})
	t.Run("heading title", func(t *testing.T) {
		result := v008Document(t)
		result.Lines[1] = v008Line(t, "# Unit [", "heading", 0, 0,
			v008Element(t, "heading-level", "#", "1", 1, 2), v008Element(t, "title", "Unit [", "Unit [", 3, 9))
		result.Lines[1].Number = 2
		title := "Unit ["
		result.Document.Metadata.Title = &title
		v008Run(t, result, "current", []v008Expected{{2, 8, 9, v008WantOpen}})
	})
}

func TestV008UnparsedAndFirstPhysicalLine(t *testing.T) {
	// Неизвестный тег и лишнее содержимое объявления версии являются ошибками
	// DSL 1.16, 1.29 и 1.2. Восстановление явно задано unparsed с исходной ошибкой;
	// её код непрозрачен для валидатора и не определяет результат V008.
	for _, tc := range []struct {
		name, raw string
		first     bool
		want      []v008Expected
	}{
		{"unparsed opening", "@unknown [", false, []v008Expected{{3, 10, 11, v008WantOpen}}},
		{"unparsed balanced", "@unknown [x]", false, nil},
		{"first physical line", "@dsl-version 1.2 [", true, []v008Expected{{1, 18, 19, v008WantOpen}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			number, parent, depth := 3, 2, 1
			if tc.first {
				number, parent, depth = 1, 0, 0
			}
			end := utf8.RuneCountInString(tc.raw) + 1
			line := v008Line(t, tc.raw, "invalid", parent, depth, model.Element{
				ElementType: "unparsed", Raw: tc.raw, Start: 1, End: end, ErrorIDs: []string{"parser-error"},
			})
			line.HasErrors = true
			result := v008Document(t, line)
			if tc.first {
				result = v008Model(t, line, result.Lines[1])
			}
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{
				ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Ошибка исходной разметки.",
				Location: &model.Location{Start: model.Position{Line: number, Column: 1},
					End: model.Position{Line: number, Column: end}}, RelatedLocations: []model.Location{},
			}}
			v008Run(t, result, "current", tc.want)
		})
	}
}

func TestV008NoLinesAndVersionSelection(t *testing.T) {
	result := v008Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v008Run(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v008Document(t)
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v008Snapshot(t, result)
			checks, err := validator.Prepare(result, v008Registry(t))
			if err == nil || checks != nil || v008Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV008RepeatedRunsPreserveHistory(t *testing.T) {
	result := v008Document(t, v008Text(t, "[", 2, 1))
	want := []v008Expected{{3, 1, 2, v008WantOpen}}
	v008Run(t, result, "first", want)
	v008Run(t, result, "second", want)
	v008Run(t, v008Document(t, v008Text(t, "]", 2, 1)), "current", []v008Expected{{3, 1, 2, v008WantClose}})
	v008Run(t, v008Document(t, v008Text(t, "[]", 2, 1)), "current", nil)
}

func TestV008CoexistsWithOtherChecks(t *testing.T) {
	result := v008Document(t, v008Text(t, "Wait. . . [", 2, 1),
		v008Line(t, "", "blank", 2, 1), v008Line(t, "", "blank", 2, 1), v008Text(t, "End.", 2, 1))
	v008Run(t, result, "current", []v008Expected{{3, 11, 12, v008WantOpen}})
	for _, tc := range []struct {
		code     diagnostics.Code
		severity diagnostics.Severity
		scope    diagnostics.Scope
		message  string
		location model.Location
	}{
		{"V003", "warning", "line", "Точки многоточия разделены пробельными символами.",
			model.Location{Start: model.Position{Line: 3, Column: 5}, End: model.Position{Line: 3, Column: 10}}},
		{"V004", "recommendation", "block", "Несколько пустых строк подряд.",
			model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 5, Column: 1}}},
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

// v008Registry создаёт реестр только через штатный каталог регистраций.
func v008Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v008Document добавляет обязательные версию и заголовок к явно заданному телу.
func v008Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v008Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v008Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v008Element(t, "version", "1.2", "1.2", 14, 17)),
		v008Line(t, "# Unit 1", "heading", 0, 0,
			v008Element(t, "heading-level", "#", "1", 1, 2), v008Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v008Model(t, append(lines, body...)...)
}

// v008Model задаёт физические номера; типы, родителей и элементы не выводит из Raw.
func v008Model(t *testing.T, lines ...model.Line) *model.Result {
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

// v008Line сохраняет явно заданную структуру строки без разбора Raw.
func v008Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v008Element задаёт исходное и семантическое значения и диапазон [start,end).
func v008Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v008Text задаёт текст, для которого Raw и Value совпадают по правилам DSL.
// Длина используется только для входного элемента, не для ожиданий диагностики.
func v008Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v008Line(t, raw, "content", parent, depth,
		v008Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v008Open задаёт блочное объявление без параметров с согласованными элементами.
func v008Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v008Line(t, "@"+tag+" {", "block-start", parent, depth,
		v008Element(t, "tag", "@"+tag, tag, 1, end), v008Element(t, "block-open", "{", "{", end+1, end+2))
}

// v008Close задаёт закрытие с явно указанными родителем и глубиной.
func v008Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v008Line(t, "}", "block-end", parent, depth, v008Element(t, "block-close", "}", "}", 1, 2))
}

// v008Run проверяет V008 текущего запуска штатного конвейера, точные сообщения,
// диапазоны и сохранность исходной модели, флагов ошибок, ссылок и истории.
func v008Run(t *testing.T, result *model.Result, processingID string, want []v008Expected) {
	t.Helper()
	before := v008Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v008Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v008Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V008" && entry.Source != processingID {
			t.Fatalf("new V008 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V008" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V008 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "Нарушена парность квадратных скобок в строке.\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v, location %+v; want %+v, %q", i, entry, entry.Location, location, expected.detail)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing or diagnostic history length")
	}
	previous := *result
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v008Snapshot(t, &previous) != before {
		t.Fatal("source model, flags, element links or previous history changed")
	}
}

// v008Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v008Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

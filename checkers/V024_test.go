package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v024Expected задаёт независимые ожидания в исходных колонках [start,end).
type v024Expected struct {
	line, start, end int
	detail           string
}

func TestV024Registration(t *testing.T) {
	definitions, err := v024Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V024" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Диакритический знак не имеет предшествующей буквы-основы." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V024 definitions = %d, want 1", matches)
	}
}

func TestV024CharactersAndGroups(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v024Expected
	}{
		// HTML-сущность из примера таблицы обозначает реальный U+0301.
		{"example", "\u0301a", []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}}},
		{"one mark at end", "\u0301", []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}}},
		{"group at end", "\u0301\u0308", []v024Expected{{3, 1, 3, "Коды знаков: U+0301, U+0308."}}},
		{"order and repetitions", "\u0308\u0301\u0308", []v024Expected{{3, 1, 4, "Коды знаков: U+0308, U+0301, U+0308."}}},
		{"Latin base", "a\u0301\u0308", nil},
		{"Cyrillic base", "Я\u0301\u0308", nil},
		{"uppercase base", "A\u0301", nil},
		{"other script base", "漢\u0301", nil},
		{"precomposed base", "é\u0301", nil},
		{"space resets", "a \u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"digit resets", "a1\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"punctuation resets", "a.\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"symbol resets and occupies one column", "a😀\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"tab resets", "a\t\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"NBSP resets", "a\u00A0\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"format character resets", "a\u200D\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"two orphan groups", "\u0301 \u0308", []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}, {3, 3, 4, "Коды знаков: U+0308."}}},
		{"letter ends orphan group", "\u0301a\u0308", []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}}},
		{"later orphan after valid group", "a\u0308.\u0301", []v024Expected{{3, 4, 5, "Коды знаков: U+0301."}}},
		{"Mn without Diacritic", "\u034F", nil},
		{"Mn preserves base", "a\u034F\u0301", nil},
		{"Mn does not create base", "\u034F\u0301", []v024Expected{{3, 2, 3, "Коды знаков: U+0301."}}},
		{"Mn splits orphan groups", "\u0301\u034F\u0308", []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}, {3, 3, 4, "Коды знаков: U+0308."}}},
		{"enclosing mark preserves base", "a\u20DD\u0301", nil},
		{"enclosing mark alone", "\u20DD", nil},
		{"spacing mark preserves base", "क\u093E\u0301", nil},
		{"standalone phonetic modifiers", "ˈˌː", nil},
		{"modifier letter is a base", "ˈ\u0301", nil},
		{"Diacritic outside M is not reported", "^", nil},
		{"Diacritic outside L and M resets", "a^\u0301", []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		{"literal entity is not decoded", "&#x301;a", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v024Run(t, v024Document(t, v024Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV024LinesAndRaw(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v024Expected
	}{
		// <br> в эталоне таблицы — перевод строки, не символы DSL.
		{"reference", []model.Line{v024Text(t, "café", 2, 1), v024Text(t, "a\u0301", 2, 1)}, nil},
		{"new line resets", []model.Line{v024Text(t, "a", 2, 1), v024Text(t, "\u0301", 2, 1)},
			[]v024Expected{{4, 1, 2, "Коды знаков: U+0301."}}},
		{"first and last body lines", []model.Line{v024Text(t, "\u0301a", 2, 1), v024Text(t, "a\u0301", 2, 1), v024Text(t, "\u0308", 2, 1)},
			[]v024Expected{{3, 1, 2, "Коды знаков: U+0301."}, {5, 1, 2, "Коды знаков: U+0308."}}},
		{"blank lines", []model.Line{v024Line(t, "", "blank", 2, 1), v024Line(t, " \t ", "blank", 2, 1), v024Text(t, "Read this.", 2, 1)}, nil},
		{"untrimmed columns", []model.Line{v024Line(t, "  \u0301  ", "content", 2, 1,
			v024Element(t, "content", "\u0301", "\u0301", 3, 4))}, []v024Expected{{3, 3, 4, "Коды знаков: U+0301."}}},
		// DSL 1.21: Raw сохраняет обратные косые черты, Value — снятое экранирование.
		{"escaping preserves columns", []model.Line{v024Line(t, "\\{x\\} \u0301", "content", 2, 1,
			v024Element(t, "content", "\\{x\\} \u0301", "{x} \u0301", 1, 8))}, []v024Expected{{3, 7, 8, "Коды знаков: U+0301."}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v024Run(t, v024Document(t, tc.body...), "current", tc.want) })
	}
}

func TestV024Blocks(t *testing.T) {
	for _, tc := range []struct {
		name, tag, raw string
		want           []v024Expected
	}{
		{"HTML not transparent", "text", "a<em>\u0301</em>", []v024Expected{{4, 6, 7, "Коды знаков: U+0301."}}},
		{"letter inside formatting", "text", "<em>a\u0301</em>", nil},
		{"HTML-like source in editor", "editor", "<a\u0301>", nil},
		{"comment is checked", "editor", "\u0301a", []v024Expected{{4, 1, 2, "Коды знаков: U+0301."}}},
		{"table row is raw text", "table", "a | \u0301", []v024Expected{{4, 5, 6, "Коды знаков: U+0301."}}},
		{"split table cell is raw text", "table", "<a | \u0301>", []v024Expected{{4, 6, 7, "Коды знаков: U+0301."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 3.6, 3.4, 2.8: корректные блоки; их внутренний синтаксис
			// диагностикой не разбирается. Editor дополнен отображаемым текстом.
			result := v024Document(t, v024Open(t, tc.tag, 2, 1), v024Text(t, tc.raw, 3, 2),
				v024Close(t, 3, 2), v024Text(t, "Read this.", 2, 1))
			v024Run(t, result, "current", tc.want)
		})
	}
	t.Run("nested text and following block", func(t *testing.T) {
		// DSL 3.12.9–3.12.10: фрагмент допускает вложенный @text.
		result := v024Document(t,
			v024Line(t, "@fragment f {", "block-start", 2, 1,
				v024Element(t, "tag", "@fragment", "fragment", 1, 10),
				v024Element(t, "identifier", "f", "f", 11, 12), v024Element(t, "block-open", "{", "{", 13, 14)),
			v024Open(t, "text", 3, 2), v024Text(t, "\u0301a", 4, 3), v024Close(t, 4, 3), v024Close(t, 3, 2),
			v024Open(t, "text", 2, 1), v024Text(t, "\u0308", 8, 2), v024Close(t, 8, 2))
		v024Run(t, result, "current", []v024Expected{{5, 1, 2, "Коды знаков: U+0301."}, {9, 1, 2, "Коды знаков: U+0308."}})
	})
	t.Run("table separator", func(t *testing.T) {
		result := v024Document(t, v024Open(t, "table", 2, 1), v024Text(t, "a", 3, 2),
			v024Line(t, "---", "separator", 3, 2), v024Text(t, "\u0301", 3, 2), v024Close(t, 3, 2))
		v024Run(t, result, "current", []v024Expected{{6, 1, 2, "Коды знаков: U+0301."}})
	})
}

func TestV024PlaceholderAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v024Expected
	}{
		{"orphan in answer", v024Line(t, "_____{\u0301a}", "content", 3, 2,
			v024Element(t, "placeholder", "_____{\u0301a}", "\u0301a", 1, 10)), []v024Expected{{4, 7, 8, "Коды знаков: U+0301."}}},
		{"valid answer", v024Line(t, "_____{a\u0301}", "content", 3, 2,
			v024Element(t, "placeholder", "_____{a\u0301}", "a\u0301", 1, 10)), nil},
		{"preceding letter cannot cross braces", v024Line(t, "a_____{\u0301a}", "content", 3, 2,
			v024Element(t, "content", "a", "a", 1, 2), v024Element(t, "placeholder", "_____{\u0301a}", "\u0301a", 2, 11)),
			[]v024Expected{{4, 8, 9, "Коды знаков: U+0301."}}},
		{"following mark has no base", v024Line(t, "_____{a}\u0301", "content", 3, 2,
			v024Element(t, "placeholder", "_____{a}", "a", 1, 9), v024Element(t, "content", "\u0301", "\u0301", 9, 10)),
			[]v024Expected{{4, 9, 10, "Коды знаков: U+0301."}}},
		{"without answer", v024Line(t, "_____", "content", 3, 2,
			model.Element{ElementType: "placeholder", Raw: "_____", Start: 1, End: 6, ErrorIDs: []string{}}), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 4.3.7–4.3.10: ответы допустимы в блочном @multifill.
			v024Run(t, v024Document(t, v024Open(t, "multifill", 2, 1), tc.line, v024Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV024HeadingAndMetadata(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		result := v024Document(t, v024Text(t, "Read this.", 2, 1))
		title := "\u0301a"
		result.Lines[1].Raw = "# \u0301a"
		result.Lines[1].Elements[1] = v024Element(t, "title", title, title, 3, 5)
		result.Document.Metadata.Title = &title
		v024Run(t, result, "current", []v024Expected{{2, 3, 4, "Коды знаков: U+0301."}})
	})
	for _, tc := range []struct {
		name, tag, value string
		kind             model.ElementType
		start, end       int
	}{
		{"resource path", "resource-dir", "\u0301dir", "resource-path", 15, 19},
		{"section name", "section", "\u0301name", "name", 10, 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.31.2: метаданные до заголовка.
			result := v024Document(t, v024Text(t, "Read this.", 2, 1))
			meta := v024Line(t, "@"+tc.tag+" "+tc.value, "tag", 0, 0,
				v024Element(t, "tag", "@"+tc.tag, tc.tag, 1, tc.start-1), v024Element(t, tc.kind, tc.value, tc.value, tc.start, tc.end))
			result.Lines = []model.Line{result.Lines[0], meta, result.Lines[1], result.Lines[2]}
			for i := range result.Lines {
				result.Lines[i].Number = i + 1
			}
			parent, count := 3, 4
			result.Lines[3].ParentLine = &parent
			result.Document.LineCount = &count
			if tc.kind == "resource-path" {
				result.Document.Metadata.ResourceDirs = []string{tc.value}
			} else {
				result.Document.Metadata.Section = &tc.value
			}
			want := []v024Expected{{2, 15, 16, "Коды знаков: U+0301."}}
			if tc.kind == "name" {
				want = []v024Expected{{2, 10, 11, "Коды знаков: U+0301."}}
			}
			v024Run(t, result, "current", want)
		})
	}
}

func TestV024Unparsed(t *testing.T) {
	// DSL 1.16: неизвестный тег сохраняется с ошибкой разбора.
	line := v024Line(t, "@unknown \u0301", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown \u0301", Start: 1, End: 11, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	result := v024Document(t, line, v024Text(t, "\u0308a", 2, 1))
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{
		ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
		DiagnosticScope: "element", Message: "Неизвестный тег.",
		Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 11}}, RelatedLocations: []model.Location{},
	}}
	v024Run(t, result, "current", []v024Expected{{3, 10, 11, "Коды знаков: U+0301."}, {4, 1, 2, "Коды знаков: U+0308."}})
}

func TestV024LineEndingsAndBOM(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
		bom  bool
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}, false},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}, false},
		{"mixed", [3]model.LineEnding{"\n", "\r\n", "\n"}, false},
		{"no final EOL", [3]model.LineEnding{"\n", "\n", ""}, false},
		{"BOM", [3]model.LineEnding{"\n", "\n", "\n"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.13: BOM и EOL отсутствуют в Raw.
			result := v024Document(t, v024Text(t, "\u0301a", 2, 1))
			result.Document.HasBOM = &tc.bom
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v024Run(t, result, "current", []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}})
		})
	}
}

func TestV024EmptyLinesAndVersions(t *testing.T) {
	v024Run(t, v024UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v024UnavailableDocument(t, version)
			before := v024Snapshot(t, result)
			checks, err := validator.Prepare(result, v024Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) did not refuse: %v, %v", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v024Snapshot(t, result) != before {
				t.Fatal("model changed on refusal")
			}
		})
	}
}

func TestV024RepeatedRuns(t *testing.T) {
	result := v024Document(t, v024Text(t, "\u0301a", 2, 1))
	want := []v024Expected{{3, 1, 2, "Коды знаков: U+0301."}}
	v024Run(t, result, "first", want)
	v024Run(t, result, "second", want)
	v024Run(t, v024Document(t, v024Text(t, "a\u0301", 2, 1)), "first", nil)
}

// v024Open создаёт объявление блока без параметров с явными родителем и глубиной.
func v024Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v024Line(t, "@"+tag+" {", "block-start", parent, depth,
		v024Element(t, "tag", "@"+tag, tag, 1, end), v024Element(t, "block-open", "{", "{", end+1, end+2))
}

// v024Close создаёт закрывающую строку указанного блока.
func v024Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v024Line(t, "}", "block-end", parent, depth, v024Element(t, "block-close", "}", "}", 1, 2))
}

// v024Registry создаёт реестр через штатный каталог без подмены регистраций.
func v024Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v024UnavailableDocument сохраняет известную версию при недоступных строках.
func v024UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v024Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v024Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v024Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v024Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v024Element(t, "version", "1.2", "1.2", 14, 17)),
		v024Line(t, "# Unit 1", "heading", 0, 0,
			v024Element(t, "heading-level", "#", "1", 1, 2),
			v024Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v024Line собирает строку с явно заданной структурой, без классификации Raw.
func v024Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v024Element сохраняет исходное и семантическое значения и явный диапазон.
func v024Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v024Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v024Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v024Line(t, text, "content", parent, depth,
		v024Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v024Run проверяет текущие V024 через штатный конвейер и независимые ожидания.
// Проверяет неизменность документа, строк, флагов, ссылок и прежней истории.
func v024Run(t *testing.T, result *model.Result, processingID string, want []v024Expected) {
	t.Helper()
	before := v024Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v024Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v024Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V024" && entry.Source != processingID {
			t.Fatalf("new V024 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V024" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V024 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected.line, Column: expected.start},
			End:   model.Position{Line: expected.line, Column: expected.end},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		message := "Диакритический знак не имеет предшествующей буквы-основы.\nконкретизация: " + expected.detail
		if entry.Message != message {
			t.Errorf("occurrence %d message = %q, want %q", i, entry.Message, message)
		}
		if entry.DiagnosticCode != "V024" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost previous diagnostics")
	}
	previous := *result
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v024Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v024Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v024Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV020TextBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][4]int
	}{
		{"example", "Это предло-", "жение.", [][4]int{{4, 5, 5, 6}}},
		{"minimum two letters", "ab-", "cd", [][4]int{{4, 1, 5, 3}}},
		{"three letters", "abc-", "def", [][4]int{{4, 1, 5, 4}}},
		{"first part too short", "a-", "bc", nil},
		{"second part too short", "ab-", "c", nil},
		{"first diacritic is not a letter", "a\u0301-", "bc", nil},
		{"second diacritic is not a letter", "ab-", "c\u0301", nil},
		{"diacritics in ranges", "ab\u0301-", "cd\u0301", [][4]int{{4, 1, 5, 4}}},
		{"leading combining mark is not a first letter", "ab-", "\u0301cd", nil},
		{"edge spaces and tabs", "  ab- \t", " \tcd.", [][4]int{{4, 3, 5, 5}}},
		{"punctuation after hyphen", "ab-.", "cd", nil},
		{"nonbreaking space after hyphen", "ab-\u00A0", "cd", nil},
		{"nonbreaking space before continuation", "ab-", "\u00A0cd", nil},
		{"no terminal hyphen", "ab", "cd", nil},
		{"space before hyphen", "ab -", "cd", nil},
		{"standalone list marker", "-", "cd", nil},
		{"number before hyphen", "12-", "cd", nil},
		{"number at continuation", "ab-", "12", nil},
		{"uppercase continuation", "ab-", "Cd", nil},
		{"uppercase first part", "AB-", "cd", [][4]int{{4, 1, 5, 3}}},
		{"uppercase later letter", "ab-", "cD", [][4]int{{4, 1, 5, 3}}},
		{"different scripts", "ab-", "вг", nil},
		{"first letters determine script", "aб-", "cd", [][4]int{{4, 1, 5, 3}}},
		{"last letter does not determine script", "aб-", "вг", nil},
		{"later continuation letters do not determine script", "ab-", "cд", [][4]int{{4, 1, 5, 3}}},
		{"Greek is supported", "αβ-", "γδ", [][4]int{{4, 1, 5, 3}}},
		{"supplementary Unicode letters", "\U00010400\U00010401-", "\U00010428\U00010429", [][4]int{{4, 1, 5, 3}}},
		{"hyphen ends continuation", "ab-", "cd-ef", [][4]int{{4, 1, 5, 3}}},
		{"apostrophe ends continuation", "ab-", "cd'ef", [][4]int{{4, 1, 5, 3}}},
		{"digit ends continuation", "ab-", "cd12", [][4]int{{4, 1, 5, 3}}},
		{"punctuation ends continuation", "ab-", "cd, ef", [][4]int{{4, 1, 5, 3}}},
		{"punctuation starts continuation", "ab-", "(cd)", nil},
		{"first word only", "ab-", "c def", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v020Run(t, v020TextBlock(t, tc.first, tc.second), "current", tc.want)
		})
	}
	// Эталон и пример сохранены; v020Document добавляет только обязательные
	// @dsl-version и заголовок перед исходным блоком @text.
	v020Run(t, v020TextBlock(t, "Это предложение."), "reference", nil)
	for _, char := range []rune{'\u00AD', '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\uFE58', '\uFE63', '\uFF0D'} {
		t.Run("only ASCII hyphen "+string(char), func(t *testing.T) {
			v020Run(t, v020TextBlock(t, "ab"+string(char), "cd"), "current", nil)
		})
	}
}

func TestV020BlankLinesAndChains(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"one blank", []model.Line{
			v020Text(t, "ab-", 3, 2), v020Line(t, "", "blank", 3, 2), v020Text(t, "cd", 3, 2),
		}, [][4]int{{4, 1, 6, 3}}},
		{"several blanks", []model.Line{
			v020Text(t, "ab-", 3, 2), v020Line(t, " \t", "blank", 3, 2),
			v020Line(t, "", "blank", 3, 2), v020Text(t, "cd", 3, 2),
		}, [][4]int{{4, 1, 7, 3}}},
		{"chain reports every boundary", []model.Line{
			v020Text(t, "ab-", 3, 2), v020Text(t, "cd-", 3, 2), v020Text(t, "ef", 3, 2),
		}, [][4]int{{4, 1, 5, 3}, {5, 1, 6, 3}}},
		{"does not skip unsuitable content", []model.Line{
			v020Text(t, "ab-", 3, 2), v020Text(t, "X", 3, 2), v020Text(t, "cd", 3, 2),
		}, nil},
		{"independent boundaries", []model.Line{
			v020Text(t, "ab-", 3, 2), v020Text(t, "cd.", 3, 2),
			v020Text(t, "ef-", 3, 2), v020Text(t, "gh.", 3, 2),
		}, [][4]int{{4, 1, 5, 3}, {6, 1, 7, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v020Open(t, "text", 2, 1)}, tc.body...)
			body = append(body, v020Close(t, 3, 2))
			v020Run(t, v020Document(t, body...), "current", tc.want)
		})
	}
}

func TestV020ContainersAndBarriers(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"ordinary step content", []model.Line{
			v020Text(t, "ab-", 2, 1), v020Text(t, "cd", 2, 1),
		}, [][4]int{{3, 1, 4, 3}}},
		{"tag is a barrier", []model.Line{
			v020Text(t, "ab-", 2, 1),
			v020Line(t, "@editor Read.", "tag", 2, 1,
				v020Element(t, "tag", "@editor", "editor", 1, 8),
				v020Element(t, "content", "Read.", "Read.", 9, 14)),
			v020Text(t, "cd", 2, 1),
		}, nil},
		{"content on a tag line is not a candidate", []model.Line{
			v020Line(t, "@editor ab-", "tag", 2, 1,
				v020Element(t, "tag", "@editor", "editor", 1, 8),
				v020Element(t, "content", "ab-", "ab-", 9, 12)),
			v020Text(t, "cd", 2, 1),
		}, nil},
		{"separator is a barrier", []model.Line{
			v020Open(t, "example", 2, 1), v020Text(t, "ab-", 3, 2),
			v020Line(t, "---", "separator", 3, 2), v020Text(t, "cd", 3, 2), v020Close(t, 3, 2),
		}, nil},
		{"neighboring blocks", []model.Line{
			v020Open(t, "example", 2, 1), v020Text(t, "ab-", 3, 2), v020Close(t, 3, 2),
			v020Open(t, "note", 2, 1), v020Text(t, "cd", 6, 2), v020Close(t, 6, 2),
		}, nil},
		{"nested content is not skipped", []model.Line{
			v020Open(t, "instr", 2, 1), v020Text(t, "ab-", 3, 2),
			v020Open(t, "hint", 3, 2), v020Text(t, "xy", 5, 3), v020Close(t, 5, 3),
			v020Text(t, "cd", 3, 2), v020Close(t, 3, 2),
		}, nil},
		{"nested container is checked", []model.Line{
			v020Open(t, "instr", 2, 1), v020Text(t, "Read.", 3, 2),
			v020Open(t, "hint", 3, 2), v020Text(t, "ab-", 5, 3), v020Text(t, "cd", 5, 3),
			v020Close(t, 5, 3), v020Close(t, 3, 2),
		}, [][4]int{{6, 1, 7, 3}}},
		{"block comment content is checked", []model.Line{
			v020Open(t, "editor", 2, 1), v020Text(t, "ab-", 3, 2), v020Text(t, "cd", 3, 2), v020Close(t, 3, 2),
		}, [][4]int{{4, 1, 5, 3}}},
		{"blank outside a finished container", []model.Line{
			v020Open(t, "text", 2, 1), v020Text(t, "ab-", 3, 2), v020Close(t, 3, 2),
			v020Line(t, "", "blank", 2, 1), v020Text(t, "cd", 2, 1),
		}, nil},
		{"path on tag line", []model.Line{
			v020Line(t, "@media image files/ab-", "tag", 2, 1,
				v020Element(t, "tag", "@media", "media", 1, 7),
				v020Element(t, "media-type", "image", "image", 8, 13),
				v020Element(t, "source", "files/ab-", "files/ab-", 14, 23)),
			v020Text(t, "cd", 2, 1),
		}, nil},
		{"plain text is not guessed to be a path", []model.Line{
			v020Text(t, "files/ab-", 2, 1), v020Text(t, "cd", 2, 1),
		}, [][4]int{{3, 7, 4, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v020Run(t, v020Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV020UnparsedBarrier(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		bad  int
	}{
		{"unparsed first line", []model.Line{v020Unparsed(t, "@unknown ab-", 2, 1), v020Text(t, "cd", 2, 1)}, 3},
		{"unparsed intervening line", []model.Line{
			v020Text(t, "ab-", 2, 1), v020Unparsed(t, "@unknown cd", 2, 1), v020Text(t, "ef", 2, 1),
		}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v020Document(t, tc.body...)
			result.Document.HasErrors, result.Lines[tc.bad-1].HasErrors = true, true
			result.Diagnostics = []model.Diagnostic{{
				ID: "d1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
				Message: "Неизвестный тег.", DiagnosticScope: "element",
				Location: &model.Location{Start: model.Position{Line: tc.bad, Column: 1},
					End: model.Position{Line: tc.bad, Column: utf8.RuneCountInString(result.Lines[tc.bad-1].Raw) + 1}},
				RelatedLocations: []model.Location{},
			}}
			v020Run(t, result, "current", nil)
		})
	}
}

func TestV020RawCoordinates(t *testing.T) {
	// Value снимает экранирование по DSL, но V020 проверяет именно Raw.
	result := v020Document(t, v020Open(t, "text", 2, 1),
		v020Line(t, `\{x\} ab-`, "content", 3, 2,
			v020Element(t, "content", `\{x\} ab-`, `{x} ab-`, 1, 10)),
		v020Text(t, "cd", 3, 2), v020Close(t, 3, 2))
	v020Run(t, result, "current", [][4]int{{4, 7, 5, 3}})
}

func TestV020LineEndingsAndFinalLine(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
		{"mixed", "\r\n", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v020Document(t, v020Text(t, "ab-", 2, 1), v020Text(t, "cd.", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[3].LineEnding = tc.last
			v020Run(t, result, "current", [][4]int{{3, 1, 4, 3}})
		})
	}
	v020Run(t, v020Document(t, v020Text(t, "ab-", 2, 1)), "final candidate", nil)
	v020Run(t, v020Document(t, v020Text(t, "ab-", 2, 1), v020Line(t, "", "blank", 2, 1)), "final blank", nil)
}

func TestV020UnavailableAndUnsupportedVersions(t *testing.T) {
	v020Run(t, v020UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v020UnavailableDocument(t, version)
			before := v020Snapshot(t, result)
			checks, err := validator.Prepare(result, v020Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v020Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV020HistoryAndOtherChecks(t *testing.T) {
	result := v020TextBlock(t, "“x”  ab-\t", "cd")
	want := [][4]int{{4, 6, 5, 3}}
	v020Run(t, result, "first", want)
	v020Run(t, result, "second", want)
	for _, source := range []string{"first", "second"} {
		for _, tc := range []struct {
			code       diagnostics.Code
			start, end int
		}{
			{"V001", 4, 6}, {"V002", 9, 10}, {"V009", 1, 2}, {"V009", 3, 4},
		} {
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source != source || entry.DiagnosticCode != tc.code || entry.Location == nil {
					continue
				}
				location := model.Location{Start: model.Position{Line: 4, Column: tc.start}, End: model.Position{Line: 4, Column: tc.end}}
				if *entry.Location == location {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s at [%d,%d) in %s: count = %d, want 1", tc.code, tc.start, tc.end, source, count)
			}
		}
	}
	v020Run(t, v020TextBlock(t, "Clean."), "first", nil)
}

// v020TextBlock добавляет исходный @text с обязательным непустым содержимым.
func v020TextBlock(t *testing.T, texts ...string) *model.Result {
	t.Helper()
	body := []model.Line{v020Open(t, "text", 2, 1)}
	for _, text := range texts {
		// Семантическое значение отличается только краевыми U+0020/HT,
		// удаляемыми парсером по DSL 1.14; Raw и ожидаемые позиции сохранены.
		start, end := 0, len(text)
		for start < end && (text[start] == ' ' || text[start] == '\t') {
			start++
		}
		for end > start && (text[end-1] == ' ' || text[end-1] == '\t') {
			end--
		}
		raw := text[start:end]
		body = append(body, v020Line(t, text, "content", 3, 2,
			v020Element(t, "content", raw, raw, start+1, start+utf8.RuneCountInString(raw)+1)))
	}
	body = append(body, v020Close(t, 3, 2))
	return v020Document(t, body...)
}

// v020Open создаёт явно заданное объявление блока без параметров.
func v020Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v020Line(t, "@"+tag+" {", "block-start", parent, depth,
		v020Element(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v020Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v020Close создаёт закрывающую строку с заданным родительским блоком.
func v020Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v020Line(t, "}", "block-end", parent, depth, v020Element(t, "block-close", "}", "}", 1, 2))
}

// v020Unparsed создаёт неизвестный тег с обязательной ссылкой на ошибку парсера.
func v020Unparsed(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v020Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value, element.ErrorIDs = nil, []string{"d1"}
	return v020Line(t, raw, "invalid", parent, depth, element)
}

func TestV020Registration(t *testing.T) {
	definitions, err := v020Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V020" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Message != "Слово разделено дефисом и переносом строки." ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V020 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V020 definitions = %d, want 1", matches)
	}
}

// v020UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v020UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v020Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v020Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v020Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v020Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v020Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v020Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v020Element(t, "version", "1.2", "1.2", 14, 17)),
		v020Line(t, "# Unit 1", "heading", 0, 0,
			v020Element(t, "heading-level", "#", "1", 1, 2),
			v020Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v020Line собирает строку с явно заданной структурой, без классификации Raw.
func v020Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v020Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v020Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v020Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v020Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v020Line(t, text, "content", parent, depth,
		v020Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v020Run проверяет штатный конвейер и явно заданные многострочные диапазоны.
// Правая граница исключается. Проверяются только V020 текущего запуска;
// прежние результаты и исходные строки должны сохраниться без изменений.
func v020Run(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v020Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v020Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v020Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V020" && entry.Source != processingID {
			t.Fatalf("new V020 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V020" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V020 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[2], Column: expected[3]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V020" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "Слово разделено дефисом и переносом строки." || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	v020AssertPreserved(t, before, result)
}

// v020Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v020Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v020AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V020 заданы явно выше.
func v020AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v020Snapshot(t, result)), &restored); err != nil {
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
	if v020Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV009Text(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"example", "He said “hello”.", "He said “hello”.", [][3]int{{3, 9, 10}, {3, 15, 16}}},
		{"ordinary text", "Ready.", "Ready.", nil},
		{"allowed quotes and modifier", "\"'ʼ", "\"'ʼ", nil},
		{"similar symbols outside property", "`′″", "`′″", nil},
		{"unpaired first and last character", "“", "“", [][3]int{{3, 1, 2}}},
		{"adjacent pairs", "“a”“b”", "“a”“b”", [][3]int{{3, 1, 2}, {3, 3, 4}, {3, 4, 5}, {3, 6, 7}}},
		{"Unicode columns", "Я😀“x”", "Я😀“x”", [][3]int{{3, 3, 4}, {3, 5, 6}}},
		{"combining mark is a column", "e\u0301“x”", "e\u0301“x”", [][3]int{{3, 3, 4}, {3, 5, 6}}},
		{"source escaping", `\{“x”\}`, `{“x”}`, [][3]int{{3, 3, 4}, {3, 5, 6}}},
		{"backslash does not excuse quote", `\“x”`, `\“x”`, [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"entities are not decoded", "&ldquo;x&rdquo;", "&ldquo;x&rdquo;", nil},
		{"uppercase content", "“ABC”", "“ABC”", [][3]int{{3, 1, 2}, {3, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v009Line(t, tc.raw, "content", 2, 1,
				v009Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v009Run(t, v009Document(t, line), "current", tc.want)
		})
	}
	// Эталон сохранён построчно; обязательный контекст добавляет v009Document.
	v009Run(t, v009Document(t,
		v009Text(t, `He said "hello".`, 2, 1),
		v009Text(t, "I'm ready.", 2, 1)), "current", nil)
}

func TestV009QuotationCharacters(t *testing.T) {
	// Явный набор запрещённых символов Quotation_Mark; ожидания не строятся
	// через unicode.Is или помощники проверяемой диагностики.
	for _, char := range []rune{
		'\u00AB', '\u00BB', '\u2018', '\u2019', '\u201A', '\u201B',
		'\u201C', '\u201D', '\u201E', '\u201F', '\u2039', '\u203A',
		'\u2E42', '\u300C', '\u300D', '\u300E', '\u300F', '\u301D',
		'\u301E', '\u301F', '\uFE41', '\uFE42', '\uFE43', '\uFE44',
		'\uFF02', '\uFF07', '\uFF62', '\uFF63',
	} {
		t.Run(string(char), func(t *testing.T) {
			v009Run(t, v009Document(t, v009Text(t, string(char), 2, 1)), "current", [][3]int{{3, 1, 2}})
		})
	}
}

func TestV009Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"single-line comment", []model.Line{
			v009Line(t, "@editor “x”", "tag", 2, 1,
				v009Element(t, "tag", "@editor", "editor", 1, 8), v009Element(t, "content", "“x”", "“x”", 9, 12)),
		}, [][3]int{{3, 9, 10}, {3, 11, 12}}},
		{"block comment", []model.Line{
			v009Open(t, "editor", 2, 1), v009Text(t, "“x”", 3, 2), v009Close(t, 3, 2),
		}, [][3]int{{4, 1, 2}, {4, 3, 4}}},
		// DSL 2.8.11: похожая на DSL-тег строка остаётся комментарием.
		{"literal tag in comment", []model.Line{
			v009Open(t, "editor", 2, 1), v009Text(t, "@text “x”", 3, 2), v009Close(t, 3, 2),
		}, [][3]int{{4, 7, 8}, {4, 9, 10}}},
		// DSL 3.8: собственный текст инструкции и вложенный hint.
		{"nested hint", []model.Line{
			v009Open(t, "instr", 2, 1), v009Text(t, "Read the sentence.", 3, 2),
			v009Open(t, "hint", 3, 2), v009Text(t, "“x”", 5, 3),
			v009Close(t, 5, 3), v009Close(t, 3, 2),
		}, [][3]int{{6, 1, 2}, {6, 3, 4}}},
		{"example separator and blocks", []model.Line{
			v009Open(t, "example", 2, 1), v009Text(t, "“", 3, 2),
			v009Line(t, "---", "separator", 3, 2), v009Text(t, "”", 3, 2), v009Close(t, 3, 2),
			v009Open(t, "note", 2, 1), v009Text(t, "‘", 8, 2), v009Close(t, 8, 2),
		}, [][3]int{{4, 1, 2}, {6, 1, 2}, {9, 1, 2}}},
		{"element offset and surrounding whitespace", []model.Line{
			v009Line(t, " \t“x” \t", "content", 2, 1, v009Element(t, "content", "“x”", "“x”", 3, 6)),
		}, [][3]int{{3, 3, 4}, {3, 5, 6}}},
		{"blank line", []model.Line{v009Line(t, " \t ", "blank", 2, 1)}, nil},
		{"media source excluded", []model.Line{
			v009Line(t, "@media image “x”.png", "tag", 2, 1,
				v009Element(t, "tag", "@media", "media", 1, 7),
				v009Element(t, "media-type", "image", "image", 8, 13),
				v009Element(t, "source", "“x”.png", "“x”.png", 14, 21)),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v009Run(t, v009Document(t, tc.body...), "current", tc.want) })
	}
}

func TestV009HTML(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"emphasis", "<em>“x”</em>", "<em>“x”</em>", [][3]int{{4, 5, 6}, {4, 7, 8}}},
		{"nested uppercase markers", "<STRONG><em>“x”</em></STRONG>", "<STRONG><em>“x”</em></STRONG>", [][3]int{{4, 13, 14}, {4, 15, 16}}},
		{"underline", "<u>‘x’</u>", "<u>‘x’</u>", [][3]int{{4, 4, 5}, {4, 6, 7}}},
		{"supported markers only", "<strong><em><u>x</u></em></strong>", "<strong><em><u>x</u></em></strong>", nil},
		{"break", "<br>", "<br>", nil},
		{"escaped literal brackets", `\<“x”\>`, `<“x”>`, [][3]int{{4, 3, 4}, {4, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v009Line(t, tc.raw, "content", 3, 2,
				v009Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v009Run(t, v009Document(t, v009Open(t, "text", 2, 1), line,
				v009Text(t, "Text.", 3, 2), v009Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV009Placeholders(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		noAnswer         bool
		want             [][3]int
	}{
		{"answer", `_____{“x”}`, `“x”`, false, [][3]int{{4, 7, 8}, {4, 9, 10}}},
		{"ordinary placeholder", `_____`, "", true, nil},
		{"allowed answer", `_____{"x"}`, `"x"`, false, nil},
		{"modifier answer", `_____{ʼ}`, `ʼ`, false, nil},
		{"escaped closing brace", `_____{a\}“b”}`, `a}“b”`, false, [][3]int{{4, 10, 11}, {4, 12, 13}}},
		{"escaped opening brace", `_____{\{“x”}`, `{“x”`, false, [][3]int{{4, 9, 10}, {4, 11, 12}}},
		{"escaped backslash", `_____{\\“x”}`, `\“x”`, false, [][3]int{{4, 9, 10}, {4, 11, 12}}},
		{"unknown escape remains data", `_____{\“x”}`, `\“x”`, false, [][3]int{{4, 8, 9}, {4, 10, 11}}},
		{"Unicode in answer", `_____{😀“”}`, `😀“”`, false, [][3]int{{4, 8, 9}, {4, 9, 10}}},
		{"angle brackets are answer data", `_____{<“x”>}`, `<“x”>`, false, [][3]int{{4, 8, 9}, {4, 10, 11}}},
		{"backslash before closing delimiter", `_____{“\\}`, `“\`, false, [][3]int{{4, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			element := v009Element(t, "placeholder", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1)
			if tc.noAnswer {
				element.Value = nil
			}
			v009Run(t, v009Document(t, v009Open(t, "multifill", 2, 1),
				v009Line(t, tc.raw, "content", 3, 2, element), v009Close(t, 3, 2)), "current", tc.want)
		})
	}
	// Первый и последний content, два разных placeholder на одной строке.
	first := v009Element(t, "placeholder", "_____", "", 4, 9)
	first.Value = nil
	v009Run(t, v009Document(t, v009Open(t, "multifill", 2, 1),
		v009Line(t, "“a _____ b _____{‘x’} c”", "content", 3, 2,
			v009Element(t, "content", "“a ", "“a ", 1, 4), first,
			v009Element(t, "content", " b ", " b ", 9, 12),
			v009Element(t, "placeholder", "_____{‘x’}", "‘x’", 12, 22),
			v009Element(t, "content", " c”", " c”", 22, 25)),
		v009Close(t, 3, 2)), "current", [][3]int{{4, 1, 2}, {4, 18, 19}, {4, 20, 21}, {4, 24, 25}})
}

func TestV009HeadingAndMetadata(t *testing.T) {
	t.Run("heading", func(t *testing.T) {
		result := v009Document(t, v009Text(t, "Text.", 2, 1))
		title := "“X”"
		result.Document.Metadata.Title = &title
		result.Lines[1].Raw = "# “X”"
		result.Lines[1].Elements[1] = v009Element(t, "title", "“X”", "“X”", 3, 6)
		v009Run(t, result, "current", [][3]int{{2, 3, 4}, {2, 5, 6}})
	})
	for _, tc := range []struct {
		name, raw, tag, kind, value string
		tagEnd, start, end          int
		want                        [][3]int
	}{
		// Кавычки входят в свободное NAME, а не служат его ограничителями.
		{"section name", "@section “X”", "section", "name", "“X”", 9, 10, 13, [][3]int{{2, 10, 11}, {2, 12, 13}}},
		{"resource path", "@resource-dir “dir”", "resource-dir", "resource-path", "“dir”", 14, 15, 20, nil},
		{"document identifier", "@document-id unit-1", "document-id", "identifier", "unit-1", 13, 14, 20, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v009Document(t)
			metadata := v009Line(t, tc.raw, "tag", 0, 0,
				v009Element(t, "tag", "@"+tc.tag, tc.tag, 1, tc.tagEnd),
				v009Element(t, model.ElementType(tc.kind), tc.value, tc.value, tc.start, tc.end))
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
			v009Run(t, result, "current", tc.want)
		})
	}
}

func TestV009UnparsedPreservesParserError(t *testing.T) {
	// Неизвестный тег уже отклонён парсером; это исключённый unparsed,
	// а не искусственное содержимое или идентификатор с запрещённой кавычкой.
	element := v009Element(t, "unparsed", "@“", "", 1, 3)
	element.Value = nil
	element.ErrorIDs = []string{"d1"}
	result := v009Document(t, v009Line(t, "@“", "invalid", 2, 1, element))
	result.Document.HasErrors, result.Lines[2].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{
		ID: "d1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		Message: "Неизвестный тег.", DiagnosticScope: "element",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 3}},
		RelatedLocations: []model.Location{},
	}}
	v009Run(t, result, "current", nil)
}

func TestV009LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
		{"mixed", "\r\n", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v009Document(t, v009Text(t, "“", 2, 1), v009Text(t, "”", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[3].LineEnding = tc.last
			v009Run(t, result, "current", [][3]int{{3, 1, 2}, {4, 1, 2}})
		})
	}
}

func TestV009UnavailableAndUnsupportedVersions(t *testing.T) {
	v009Run(t, v009UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v009UnavailableDocument(t, version)
			before := v009Snapshot(t, result)
			checks, err := validator.Prepare(result, v009Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v009Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV009HistoryAndOtherChecks(t *testing.T) {
	result := v009Document(t, v009Text(t, "A\t  “x”", 2, 1))
	v009Run(t, result, "first", [][3]int{{3, 5, 6}, {3, 7, 8}})
	v009Run(t, result, "second", [][3]int{{3, 5, 6}, {3, 7, 8}})
	for _, source := range []string{"first", "second"} {
		for _, tc := range []struct {
			code       diagnostics.Code
			start, end int
		}{
			{"V001", 3, 5}, {"V002", 2, 3},
		} {
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source != source || entry.DiagnosticCode != tc.code {
					continue
				}
				count++
				want := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
				if entry.Location == nil || *entry.Location != want {
					t.Errorf("unexpected %s location: %+v", tc.code, entry.Location)
				}
			}
			if count != 1 {
				t.Errorf("%s count for %s = %d, want 1", tc.code, source, count)
			}
		}
	}
	v009Run(t, v009Document(t, v009Text(t, "Clean.", 2, 1)), "first", nil)
}

// v009Open создаёт явно заданное объявление блока без параметров.
func v009Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v009Line(t, "@"+tag+" {", "block-start", parent, depth,
		v009Element(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v009Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v009Close создаёт закрывающую строку с заданным родительским блоком.
func v009Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v009Line(t, "}", "block-end", parent, depth, v009Element(t, "block-close", "}", "}", 1, 2))
}

func TestV009Registration(t *testing.T) {
	definitions, err := v009Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V009" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Message != "Использована кавычка нестандартного вида." ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V009 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V009 definitions = %d, want 1", matches)
	}
}

// v009UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v009UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v009Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v009Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v009Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v009Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v009Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v009Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v009Element(t, "version", "1.2", "1.2", 14, 17)),
		v009Line(t, "# Unit 1", "heading", 0, 0,
			v009Element(t, "heading-level", "#", "1", 1, 2),
			v009Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v009Line собирает строку с явно заданной структурой, без классификации Raw.
func v009Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v009Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v009Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v009Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v009Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v009Line(t, text, "content", parent, depth,
		v009Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v009Run проверяет штатный конвейер и явные тройки: строка, начало, конец.
// Правая граница исключается. Проверяются только V009 текущего запуска;
// прежние результаты и исходные строки должны сохраниться без изменений.
func v009Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v009Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v009Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v009Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V009" && entry.Source != processingID {
			t.Fatalf("new V009 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V009" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V009 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V009" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "Использована кавычка нестандартного вида." || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	v009AssertPreserved(t, before, result)
}

// v009Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v009Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v009AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V009 заданы явно выше.
func v009AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v009Snapshot(t, result)), &restored); err != nil {
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
	if v009Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

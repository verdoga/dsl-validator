package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV001Registration(t *testing.T) {
	registry := v001Registry(t)
	if registry.Len() != 1 {
		t.Fatalf("registered diagnostics = %d, want exactly V001", registry.Len())
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil || len(definitions) != 1 {
		t.Fatalf("ForVersion(1.2) = %v, %v", definitions, err)
	}
	definition := definitions[0]
	if definition.Code != "V001" || definition.Severity != "warning" ||
		definition.Message != "Несколько пробелов подряд." || definition.Scope != "line" ||
		definition.Versions.Mode != diagnostics.VersionExact || definition.Versions.Version != "1.2" || definition.Check == nil {
		t.Fatalf("unexpected definition: %+v", definition)
	}
}

func TestV001TextSequences(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Read this sentence.", nil},
		{"example", "Read  this   sentence.", [][3]int{{3, 5, 7}, {3, 11, 14}}},
		{"no spaces", "AB", nil},
		{"single space", "A B", nil},
		{"two spaces", "A  B", [][3]int{{3, 2, 4}}},
		{"three spaces form one occurrence", "A   B", [][3]int{{3, 2, 5}}},
		{"four spaces form one occurrence", "A    B", [][3]int{{3, 2, 6}}},
		{"separate runs and a single space", "A  B C  D", [][3]int{{3, 2, 4}, {3, 7, 9}}},
		{"tab separates single spaces", "A \t B", nil},
		{"tabs alone", "A\t\tB", nil},
		{"tab separates runs", "A  \t  B", [][3]int{{3, 2, 4}, {3, 5, 7}}},
		{"nonbreaking spaces are not U+0020", "A\u00a0\u00a0B", nil},
		{"em spaces are not U+0020", "A\u2003\u2003B", nil},
		{"nonbreaking space separates runs", "A  \u00a0  B", [][3]int{{3, 2, 4}, {3, 5, 7}}},
		{"em space separates runs", "A  \u2003  B", [][3]int{{3, 2, 4}, {3, 5, 7}}},
		{"punctuation separates runs", "A  .  B", [][3]int{{3, 2, 4}, {3, 5, 7}}},
		{"Unicode code points instead of bytes", "Я😀  Б", [][3]int{{3, 3, 5}}},
		{"combining mark is a separate code point", "e\u0301  X", [][3]int{{3, 3, 5}}},
		{"case does not affect runs", "READ  THIS   SENTENCE.", [][3]int{{3, 5, 7}, {3, 11, 14}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v001Document(t, v001Text(t, tc.raw, 2, 1))
			v001Run(t, result, "current", tc.want)
		})
	}
}

func TestV001RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty physical line", []model.Line{v001Line(t, "", "blank", 2, 1)}, nil},
		{"one space in blank line", []model.Line{v001Line(t, " ", "blank", 2, 1)}, nil},
		{"two spaces in blank line", []model.Line{v001Line(t, "  ", "blank", 2, 1)}, [][3]int{{3, 1, 3}}},
		{"three spaces in blank line", []model.Line{v001Line(t, "   ", "blank", 2, 1)}, [][3]int{{3, 1, 4}}},
		{"blank tabs alone", []model.Line{v001Line(t, "\t\t", "blank", 2, 1)}, nil},
		{"blank tabs around spaces", []model.Line{v001Line(t, "\t  \t", "blank", 2, 1)}, [][3]int{{3, 2, 4}}},
		{"leading spaces outside elements", []model.Line{
			v001Line(t, "  A", "content", 2, 1, v001Element(t, "content", "A", "A", 3, 4)),
		}, [][3]int{{3, 1, 3}}},
		{"trailing spaces outside elements", []model.Line{
			v001Line(t, "A  ", "content", 2, 1, v001Element(t, "content", "A", "A", 1, 2)),
		}, [][3]int{{3, 2, 4}}},
		{"both edges outside elements", []model.Line{
			v001Line(t, "  A  ", "content", 2, 1, v001Element(t, "content", "A", "A", 3, 4)),
		}, [][3]int{{3, 1, 3}, {3, 4, 6}}},
		{"single edge spaces across lines do not join", []model.Line{
			v001Line(t, "A ", "content", 2, 1, v001Element(t, "content", "A", "A", 1, 2)),
			v001Line(t, " B", "content", 2, 1, v001Element(t, "content", "B", "B", 2, 3)),
		}, nil},
		{"runs across lines stay separate", []model.Line{
			v001Line(t, "A  ", "content", 2, 1, v001Element(t, "content", "A", "A", 1, 2)),
			v001Line(t, "  B", "content", 2, 1, v001Element(t, "content", "B", "B", 3, 4)),
		}, [][3]int{{3, 2, 4}, {4, 1, 3}}},
		{"single-line editor comment", []model.Line{
			v001Line(t, "@editor A  B", "tag", 2, 1,
				v001Element(t, "tag", "@editor", "editor", 1, 8),
				v001Element(t, "content", "A  B", "A  B", 9, 13)),
		}, [][3]int{{3, 10, 12}}},
		{"tag case does not exclude a comment", []model.Line{
			v001Line(t, "@EDITOR A  B", "tag", 2, 1,
				v001Element(t, "tag", "@EDITOR", "editor", 1, 8),
				v001Element(t, "content", "A  B", "A  B", 9, 13)),
		}, [][3]int{{3, 10, 12}}},
		// DSL 2.8: @editor не отображается, но все его строки входят в Raw.
		{"block editor comment", []model.Line{
			v001Line(t, "@editor {", "block-start", 2, 1,
				v001Element(t, "tag", "@editor", "editor", 1, 8), v001Element(t, "block-open", "{", "{", 9, 10)),
			v001Text(t, "A  B", 3, 2),
			v001Line(t, "}", "block-end", 3, 2, v001Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 2, 4}}},
		{"literal tag inside editor comment", []model.Line{
			v001Line(t, "@editor {", "block-start", 2, 1,
				v001Element(t, "tag", "@editor", "editor", 1, 8), v001Element(t, "block-open", "{", "{", 9, 10)),
			v001Text(t, "@task A  B", 3, 2),
			v001Line(t, "}", "block-end", 3, 2, v001Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 8, 10}}},
		// DSL 3.2.8: разделитель имеет непустое содержимое с обеих сторон.
		{"block boundaries and separator", []model.Line{
			v001Line(t, "@example  {", "block-start", 2, 1,
				v001Element(t, "tag", "@example", "example", 1, 9), v001Element(t, "block-open", "{", "{", 11, 12)),
			v001Text(t, "A", 3, 2),
			v001Line(t, "  ---  ", "separator", 3, 2),
			v001Text(t, "B", 3, 2),
			v001Line(t, "}  ", "block-end", 3, 2, v001Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{3, 9, 11}, {5, 1, 3}, {5, 6, 8}, {7, 2, 4}}},
		// DSL 3.8.6, 3.8.11: инструкция содержит собственный текст и один @hint.
		{"nested hint in instruction", []model.Line{
			v001Line(t, "@instr {", "block-start", 2, 1,
				v001Element(t, "tag", "@instr", "instr", 1, 7), v001Element(t, "block-open", "{", "{", 8, 9)),
			v001Text(t, "Read the sentence.", 3, 2),
			v001Line(t, "@hint {", "block-start", 3, 2,
				v001Element(t, "tag", "@hint", "hint", 1, 6), v001Element(t, "block-open", "{", "{", 7, 8)),
			v001Text(t, "Use  context.", 5, 3),
			v001Line(t, "}", "block-end", 5, 3, v001Element(t, "block-close", "}", "}", 1, 2)),
			v001Line(t, "}", "block-end", 3, 2, v001Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{6, 4, 6}}},
		// DSL 4.4.6, 4.4.10: предзаполненный текст и blank не исключаются из V001.
		{"multifill content and blank line", []model.Line{
			v001Line(t, "@multifill {", "block-start", 2, 1,
				v001Element(t, "tag", "@multifill", "multifill", 1, 11), v001Element(t, "block-open", "{", "{", 12, 13)),
			v001Text(t, "Type  here.", 3, 2),
			v001Line(t, "  ", "blank", 3, 2),
			v001Line(t, "}", "block-end", 3, 2, v001Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 5, 7}, {5, 1, 3}}},
		// DSL 1.21–1.22: Value снимает экранирование, но позиции относятся к Raw.
		{"escaped braces preserve source columns", []model.Line{
			v001Line(t, `Use \{x\}  now.`, "content", 2, 1,
				v001Element(t, "content", `Use \{x\}  now.`, "Use {x}  now.", 1, 16)),
		}, [][3]int{{3, 10, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v001Run(t, v001Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV001FirstAndLastLines(t *testing.T) {
	result := v001Document(t, v001Text(t, "A  B", 2, 1))
	result.Lines[0].Raw = "@dsl-version  1.2"
	result.Lines[0].Elements[1] = v001Element(t, "version", "1.2", "1.2", 15, 18)
	result.Lines[2].LineEnding = model.LineEndingNone
	v001Run(t, result, "current", [][3]int{{1, 13, 15}, {3, 2, 4}})
}

func TestV001Heading(t *testing.T) {
	result := v001Document(t, v001Text(t, "Read this sentence.", 2, 1))
	result.Lines[1].Raw = "# Unit  1"
	result.Lines[1].Elements[1] = v001Element(t, "title", "Unit  1", "Unit  1", 3, 10)
	title := "Unit  1"
	result.Document.Metadata.Title = &title
	v001Run(t, result, "current", [][3]int{{2, 7, 9}})
}

func TestV001LineEndings(t *testing.T) {
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
			result := v001Document(t, v001Text(t, "Read  this   sentence.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v001Run(t, result, "current", [][3]int{{3, 5, 7}, {3, 11, 14}})
		})
	}
}

func TestV001UnavailableLines(t *testing.T) {
	// Это отсутствие доступных строк, а не пустой DSL-файл с выдуманным тегом.
	// validator.checkLineSequence допускает nil LineCount и пустой Lines.
	v001Run(t, v001UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV001UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			// Версия известна из метаданных; исходные строки недоступны.
			// Грамматику неподдерживаемых версий тест не предполагает.
			result := v001UnavailableDocument(t, version)
			before := v001Snapshot(t, result)
			checks, err := validator.Prepare(result, v001Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v001Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV001RepeatedRunsPreserveHistory(t *testing.T) {
	result := v001Document(t, v001Text(t, "Read  this   sentence.", 2, 1))
	v001Run(t, result, "first", [][3]int{{3, 5, 7}, {3, 11, 14}})
	v001Run(t, result, "second", [][3]int{{3, 5, 7}, {3, 11, 14}})
	if len(result.Diagnostics) != 4 || len(result.Processing) != 3 {
		t.Fatalf("after two runs: diagnostics=%d, processing=%d", len(result.Diagnostics), len(result.Processing))
	}
	// Новый документ после двух запусков не наследует накопленные срабатывания.
	v001Run(t, v001Document(t, v001Text(t, "Read this sentence.", 2, 1)), "first", nil)
}

// v001UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v001UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v001Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v001Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v001Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v001Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v001Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v001Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v001Element(t, "version", "1.2", "1.2", 14, 17)),
		v001Line(t, "# Unit 1", "heading", 0, 0,
			v001Element(t, "heading-level", "#", "1", 1, 2),
			v001Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v001Line собирает строку с явно заданной структурой, без классификации Raw.
func v001Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v001Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v001Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v001Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v001Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v001Line(t, text, "content", parent, depth,
		v001Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v001Run проверяет штатный конвейер и явные тройки: строка, начало, конец.
// Правая граница исключается. Проверяются только V001 текущего запуска;
// прежние результаты, строки, флаги и ссылки должны сохраниться без изменений.
func v001Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v001Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v001Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v001Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V001" && entry.Source != processingID {
			t.Fatalf("new V001 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V001" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V001 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V001" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "Несколько пробелов подряд." || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	previous := *result
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v001Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v001Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v001Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

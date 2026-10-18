package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v010Expected задаёт независимое ожидание: строку, Unicode-колонки [start,end)
// и точную конкретизацию, без обращения к реализации диагностики.
type v010Expected struct {
	line, start, end int
	detail           string
}

func TestV010Registration(t *testing.T) {
	definitions, err := v010Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V010" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Использован пробельный символ, отличный от обычного пробела." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V010 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V010 definitions = %d, want 1", matches)
	}
}

func TestV010EverySpecifiedCode(t *testing.T) {
	for _, tc := range []struct {
		name, raw, detail string
	}{
		{"U+000B", "A\u000BB", "Обнаружены: U+000B. Допустим U+0020."},
		{"U+000C", "A\u000CB", "Обнаружены: U+000C. Допустим U+0020."},
		{"U+0085", "A\u0085B", "Обнаружены: U+0085. Допустим U+0020."},
		{"U+00A0", "A\u00A0B", "Обнаружены: U+00A0. Допустим U+0020."},
		{"U+1680", "A\u1680B", "Обнаружены: U+1680. Допустим U+0020."},
		{"U+2000", "A\u2000B", "Обнаружены: U+2000. Допустим U+0020."},
		{"U+2001", "A\u2001B", "Обнаружены: U+2001. Допустим U+0020."},
		{"U+2002", "A\u2002B", "Обнаружены: U+2002. Допустим U+0020."},
		{"U+2003", "A\u2003B", "Обнаружены: U+2003. Допустим U+0020."},
		{"U+2004", "A\u2004B", "Обнаружены: U+2004. Допустим U+0020."},
		{"U+2005", "A\u2005B", "Обнаружены: U+2005. Допустим U+0020."},
		{"U+2006", "A\u2006B", "Обнаружены: U+2006. Допустим U+0020."},
		{"U+2007", "A\u2007B", "Обнаружены: U+2007. Допустим U+0020."},
		{"U+2008", "A\u2008B", "Обнаружены: U+2008. Допустим U+0020."},
		{"U+2009", "A\u2009B", "Обнаружены: U+2009. Допустим U+0020."},
		{"U+200A", "A\u200AB", "Обнаружены: U+200A. Допустим U+0020."},
		{"U+2028", "A\u2028B", "Обнаружены: U+2028. Допустим U+0020."},
		{"U+2029", "A\u2029B", "Обнаружены: U+2029. Допустим U+0020."},
		{"U+202F", "A\u202FB", "Обнаружены: U+202F. Допустим U+0020."},
		{"U+205F", "A\u205FB", "Обнаружены: U+205F. Допустим U+0020."},
		{"U+3000", "A\u3000B", "Обнаружены: U+3000. Допустим U+0020."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v010Run(t, v010Document(t, v010Text(t, tc.raw, 2, 1)), "current",
				[]v010Expected{{3, 2, 3, tc.detail}})
		})
	}
}

func TestV010Sequences(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v010Expected
	}{
		{"reference", "Read this.", nil},
		// &#xA0; в таблице обозначает U+00A0, а не текст HTML-сущности.
		{"example", "Read\u00A0this.", []v010Expected{{3, 5, 6, "Обнаружены: U+00A0. Допустим U+0020."}}},
		{"repeated code", "A\u00A0\u00A0\u00A0B", []v010Expected{{3, 2, 5, "Обнаружены: U+00A0. Допустим U+0020."}}},
		{"mixed codes sorted and deduplicated", "A\u2003\u00A0\u2003B", []v010Expected{{3, 2, 5, "Обнаружены: U+00A0, U+2003. Допустим U+0020."}}},
		{"all specified codes in one run", "A\u000B\u000C\u0085\u00A0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200A\u2028\u2029\u202F\u205F\u3000B",
			[]v010Expected{{3, 2, 23, "Обнаружены: U+000B, U+000C, U+0085, U+00A0, U+1680, U+2000, U+2001, U+2002, U+2003, U+2004, U+2005, U+2006, U+2007, U+2008, U+2009, U+200A, U+2028, U+2029, U+202F, U+205F, U+3000. Допустим U+0020."}}},
		{"space separates runs", "A\u00A0 \u2003B", []v010Expected{{3, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."}, {3, 4, 5, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"tab separates runs", "A\u00A0\t\u2003B", []v010Expected{{3, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."}, {3, 4, 5, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"letter separates runs", "A\u00A0X\u2003B", []v010Expected{{3, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."}, {3, 4, 5, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"punctuation separates runs", "A\u00A0.\u2003B", []v010Expected{{3, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."}, {3, 4, 5, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"ordinary spaces", "A  B", nil},
		{"tab", "A\tB", nil},
		{"U+180E is outside the set", "A\u180EB", nil},
		{"U+200B is outside the set", "A\u200BB", nil},
		{"U+2060 is outside the set", "A\u2060B", nil},
		{"internal U+FEFF is outside the set", "A\uFEFFB", nil},
		{"both edges", "\u00A0A\u2003", []v010Expected{{3, 1, 2, "Обнаружены: U+00A0. Допустим U+0020."}, {3, 3, 4, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"Unicode columns", "Я😀\u00A0Б", []v010Expected{{3, 3, 4, "Обнаружены: U+00A0. Допустим U+0020."}}},
		{"combining mark occupies a column", "e\u0301\u2003X", []v010Expected{{3, 3, 4, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"uppercase text", "READ\u00A0THIS.", []v010Expected{{3, 5, 6, "Обнаружены: U+00A0. Допустим U+0020."}}},
		{"literal HTML entity", "Read&#xA0;this.", nil},
		{"literal Unicode escape", `Read\u00A0this.`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v010Run(t, v010Document(t, v010Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV010RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v010Expected
	}{
		{"empty physical line", []model.Line{v010Line(t, "", "blank", 2, 1), v010Text(t, "Read this.", 2, 1)}, nil},
		{"blank spaces and tab", []model.Line{v010Line(t, " \t ", "blank", 2, 1), v010Text(t, "Read this.", 2, 1)}, nil},
		{"ordinary indentation preserves columns", []model.Line{
			v010Line(t, "  \u00A0A\u2003  ", "content", 2, 1, v010Element(t, "content", "\u00A0A\u2003", "\u00A0A\u2003", 3, 6)),
		}, []v010Expected{{3, 3, 4, "Обнаружены: U+00A0. Допустим U+0020."}, {3, 5, 6, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"runs never join across lines", []model.Line{
			v010Text(t, "A\u00A0", 2, 1), v010Text(t, "\u2003B", 2, 1),
		}, []v010Expected{{3, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."}, {4, 1, 2, "Обнаружены: U+2003. Допустим U+0020."}}},
		{"single-line editor comment", []model.Line{
			v010Line(t, "@editor A\u00A0B", "tag", 2, 1,
				v010Element(t, "tag", "@editor", "editor", 1, 8), v010Element(t, "content", "A\u00A0B", "A\u00A0B", 9, 12)),
			v010Text(t, "Read this.", 2, 1),
		}, []v010Expected{{3, 10, 11, "Обнаружены: U+00A0. Допустим U+0020."}}},
		{"uppercase editor tag", []model.Line{
			v010Line(t, "@EDITOR A\u00A0B", "tag", 2, 1,
				v010Element(t, "tag", "@EDITOR", "editor", 1, 8), v010Element(t, "content", "A\u00A0B", "A\u00A0B", 9, 12)),
			v010Text(t, "Read this.", 2, 1),
		}, []v010Expected{{3, 10, 11, "Обнаружены: U+00A0. Допустим U+0020."}}},
		{"block editor comment", []model.Line{
			v010Line(t, "@editor {", "block-start", 2, 1,
				v010Element(t, "tag", "@editor", "editor", 1, 8), v010Element(t, "block-open", "{", "{", 9, 10)),
			v010Text(t, "A\u00A0B", 3, 2),
			v010Line(t, "}", "block-end", 3, 2, v010Element(t, "block-close", "}", "}", 1, 2)),
			v010Text(t, "Read this.", 2, 1),
		}, []v010Expected{{4, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."}}},
		// DSL 3.8.6, 3.8.11: собственный текст инструкции и один вложенный @hint.
		{"nested hint in instruction", []model.Line{
			v010Line(t, "@instr {", "block-start", 2, 1,
				v010Element(t, "tag", "@instr", "instr", 1, 7), v010Element(t, "block-open", "{", "{", 8, 9)),
			v010Text(t, "Read the sentence.", 3, 2),
			v010Line(t, "@hint {", "block-start", 3, 2,
				v010Element(t, "tag", "@hint", "hint", 1, 6), v010Element(t, "block-open", "{", "{", 7, 8)),
			v010Text(t, "Use\u00A0context.", 5, 3),
			v010Line(t, "}", "block-end", 5, 3, v010Element(t, "block-close", "}", "}", 1, 2)),
			v010Line(t, "}", "block-end", 3, 2, v010Element(t, "block-close", "}", "}", 1, 2)),
		}, []v010Expected{{6, 4, 5, "Обнаружены: U+00A0. Допустим U+0020."}}},
		// DSL 1.21–1.22: Value снимает экранирование, Raw сохраняет исходные колонки.
		{"escaped braces preserve source columns", []model.Line{
			v010Line(t, "Use \\{x\\}\u00A0now.", "content", 2, 1,
				v010Element(t, "content", "Use \\{x\\}\u00A0now.", "Use {x}\u00A0now.", 1, 15)),
		}, []v010Expected{{3, 10, 11, "Обнаружены: U+00A0. Допустим U+0020."}}},
		// DSL 3.1.6: кавычки входят в Raw, но не в семантический путь SOURCE.
		// Валидатор получает модель; доступность медиафайла здесь не проверяется.
		{"quoted media path", []model.Line{
			v010Line(t, "@media image \"a\u00A0b.png\"", "tag", 2, 1,
				v010Element(t, "tag", "@media", "media", 1, 7),
				v010Element(t, "media-type", "image", "image", 8, 13),
				v010Element(t, "source", "\"a\u00A0b.png\"", "a\u00A0b.png", 14, 23)),
		}, []v010Expected{{3, 16, 17, "Обнаружены: U+00A0. Допустим U+0020."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v010Run(t, v010Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV010Heading(t *testing.T) {
	result := v010Document(t, v010Text(t, "Read this.", 2, 1))
	result.Lines[1].Raw = "# Unit\u00A01"
	result.Lines[1].Elements[1] = v010Element(t, "title", "Unit\u00A01", "Unit\u00A01", 3, 9)
	title := "Unit\u00A01"
	result.Document.Metadata.Title = &title
	v010Run(t, result, "current", []v010Expected{{2, 7, 8, "Обнаружены: U+00A0. Допустим U+0020."}})
}

func TestV010LineEndings(t *testing.T) {
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
			result := v010Document(t, v010Text(t, "Read\u00A0this.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v010Run(t, result, "current", []v010Expected{{3, 5, 6, "Обнаружены: U+00A0. Допустим U+0020."}})
		})
	}
}

func TestV010InitialBOM(t *testing.T) {
	for _, tc := range []struct {
		name   string
		hasBOM bool
	}{
		{"with BOM", true},
		{"without BOM", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.13: BOM удалён до выделения строк и сохранён только в метаданных.
			result := v010Document(t, v010Text(t, "Read\u00A0this.", 2, 1))
			result.Document.HasBOM = &tc.hasBOM
			v010Run(t, result, "current", []v010Expected{{3, 5, 6, "Обнаружены: U+00A0. Допустим U+0020."}})
		})
	}
}

func TestV010UnavailableLines(t *testing.T) {
	// Допустимое отсутствие доступных строк с известной версией, а не выдуманный DSL.
	v010Run(t, v010UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV010UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v010UnavailableDocument(t, version)
			before := v010Snapshot(t, result)
			checks, err := validator.Prepare(result, v010Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v010Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV010CoexistsWithV001AndV002(t *testing.T) {
	result := v010Document(t, v010Text(t, "A\u00A0  \t\u2003B", 2, 1))
	v010Run(t, result, "current", []v010Expected{
		{3, 2, 3, "Обнаружены: U+00A0. Допустим U+0020."},
		{3, 6, 7, "Обнаружены: U+2003. Допустим U+0020."},
	})
	for _, want := range []struct {
		code       diagnostics.Code
		start, end int
		message    string
	}{
		{"V001", 3, 5, "Несколько пробелов подряд."},
		{"V002", 5, 6, "В строке есть табуляция."},
	} {
		matches := 0
		for _, entry := range result.Diagnostics {
			if entry.DiagnosticCode != want.code || entry.Source != "current" {
				continue
			}
			matches++
			location := model.Location{Start: model.Position{Line: 3, Column: want.start}, End: model.Position{Line: 3, Column: want.end}}
			if entry.Location == nil || *entry.Location != location || entry.Message != want.message ||
				entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", want.code, entry)
			}
		}
		if matches != 1 {
			t.Errorf("current %s count = %d, want 1", want.code, matches)
		}
	}
}

func TestV010RepeatedRunsPreserveHistory(t *testing.T) {
	result := v010Document(t, v010Text(t, "Read\u00A0this.", 2, 1))
	want := []v010Expected{{3, 5, 6, "Обнаружены: U+00A0. Допустим U+0020."}}
	v010Run(t, result, "first", want)
	v010Run(t, result, "second", want)
	v010Run(t, v010Document(t, v010Text(t, "Read this.", 2, 1)), "first", nil)
}

// v010Registry создаёт реестр через штатный каталог без подмены регистраций.
func v010Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v010UnavailableDocument сохраняет известную версию при недоступных строках.
func v010UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v010Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v010Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v010Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v010Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v010Element(t, "version", "1.2", "1.2", 14, 17)),
		v010Line(t, "# Unit 1", "heading", 0, 0,
			v010Element(t, "heading-level", "#", "1", 1, 2),
			v010Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v010Line собирает строку с явно заданной структурой, без классификации Raw.
func v010Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v010Element сохраняет исходное и семантическое значения и явный диапазон.
func v010Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v010Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v010Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v010Line(t, text, "content", parent, depth,
		v010Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v010Run проверяет текущие V010 через штатный конвейер и независимые ожидания.
// Проверяет исходные данные, ссылки и историю; флаги учитывают весь каталог.
func v010Run(t *testing.T, result *model.Result, processingID string, want []v010Expected) {
	t.Helper()
	before := v010Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v010Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v010Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V010" && entry.Source != processingID {
			t.Fatalf("new V010 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V010" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V010 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "Использован пробельный символ, отличный от обычного пробела.\nконкретизация: " + expected.detail
		if entry.Message != message {
			t.Errorf("occurrence %d message = %q, want %q", i, entry.Message, message)
		}
		if entry.DiagnosticCode != "V010" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost previous diagnostics")
	}
	// Полный каталог может добавить error других кодов. Проверяем флаги
	// штатной проверкой целостности, затем сравниваем исходные поля и историю.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("model after full catalog: %v", err)
	}
	var original, previous model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v010Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	previous.Document.HasErrors = original.Document.HasErrors
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
	}
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v010Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v010Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v010Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

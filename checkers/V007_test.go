package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v007Expected задаёт независимые ожидаемые координаты и конкретизацию.
type v007Expected struct {
	line, start, end int
	detail           string
}

const (
	v007OpenDetail  = "Открывающая круглая скобка не закрыта в этой строке"
	v007CloseDetail = "Закрывающая круглая скобка не имеет открывающей в этой строке"
)

func TestV007Registration(t *testing.T) {
	definitions, err := v007Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V007" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "Нарушена парность круглых скобок в строке." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V007 definitions = %d, want 1", matches)
	}
}

func TestV007Text(t *testing.T) {
	for _, tc := range []struct {
		name string
		text []string
		want []v007Expected
	}{
		{"no parentheses", []string{"Read this."}, nil},
		{"reference", []string{"Read (the word (carefully)).", "Continue (slowly)."}, nil},
		{"example", []string{"Read (the word.", "Continue)."}, []v007Expected{{3, 6, 7, v007OpenDetail}, {4, 9, 10, v007CloseDetail}}},
		{"one opening", []string{"("}, []v007Expected{{3, 1, 2, v007OpenDetail}}},
		{"one closing", []string{")"}, []v007Expected{{3, 1, 2, v007CloseDetail}}},
		{"pair", []string{"()"}, nil},
		{"nested and adjacent pairs", []string{"(())()(()())"}, nil},
		{"last opening closes first", []string{"(()"}, []v007Expected{{3, 1, 2, v007OpenDetail}}},
		{"opening order", []string{"((("}, []v007Expected{{3, 1, 2, v007OpenDetail}, {3, 2, 3, v007OpenDetail}, {3, 3, 4, v007OpenDetail}}},
		{"all extra closings", []string{")))"}, []v007Expected{{3, 1, 2, v007CloseDetail}, {3, 2, 3, v007CloseDetail}, {3, 3, 4, v007CloseDetail}}},
		{"continue after extra closing", []string{")()("}, []v007Expected{{3, 1, 2, v007CloseDetail}, {3, 4, 5, v007OpenDetail}}},
		{"line boundary", []string{"(", ")"}, []v007Expected{{3, 1, 2, v007OpenDetail}, {4, 1, 2, v007CloseDetail}}},
		{"quotes do not hide opening", []string{`"("`}, []v007Expected{{3, 2, 3, v007OpenDetail}}},
		{"quotes do not hide closing", []string{`")"`}, []v007Expected{{3, 2, 3, v007CloseDetail}}},
		{"backslash does not hide opening", []string{`\(`}, []v007Expected{{3, 2, 3, v007OpenDetail}}},
		{"backslash does not hide closing", []string{`\)`}, []v007Expected{{3, 2, 3, v007CloseDetail}}},
		{"backslash parentheses still pair", []string{`\(x\)`}, nil},
		{"other brackets", []string{"（［］）[]"}, nil},
		{"other brackets do not close opening", []string{"(［］）"}, []v007Expected{{3, 1, 2, v007OpenDetail}}},
		{"HTML entity is not decoded", []string{"&#40;"}, nil},
		{"HTML entity cannot open pair", []string{"&#40;)"}, []v007Expected{{3, 6, 7, v007CloseDetail}}},
		{"Unicode columns", []string{"Я😀)"}, []v007Expected{{3, 3, 4, v007CloseDetail}}},
		{"combining mark is a column", []string{"e\u0301("}, []v007Expected{{3, 3, 4, v007OpenDetail}}},
		{"case and whitespace do not change pairing", []string{"( A\tB )", "( a\tb )"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := make([]model.Line, 0, len(tc.text))
			for _, text := range tc.text {
				body = append(body, v007Text(text, 2, 1))
			}
			v007Run(t, v007Document(body...), "current", tc.want)
		})
	}
}

func TestV007RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v007Expected
	}{
		{"empty physical line", []model.Line{v007Line("", "blank", 2, 1)}, nil},
		{"blank whitespace", []model.Line{v007Line(" \t ", "blank", 2, 1)}, nil},
		{"blank line does not join scopes", []model.Line{
			v007Text("(", 2, 1), v007Line("", "blank", 2, 1), v007Text(")", 2, 1),
		}, []v007Expected{{3, 1, 2, v007OpenDetail}, {5, 1, 2, v007CloseDetail}}},
		// DSL 1.14: элементы не включают краевые пробелы; Raw сохраняется целиком.
		{"original columns before and after trimmed content", []model.Line{
			v007Line("  A\t( ", "content", 2, 1, v007Element("content", "A\t(", "A\t(", 3, 6)),
		}, []v007Expected{{3, 5, 6, v007OpenDetail}}},
		// DSL 1.21–1.22: Value снимает экранирование фигурной скобки, Raw — нет.
		{"raw differs from semantic text", []model.Line{
			v007Line(`\{)`, "content", 2, 1, v007Element("content", `\{)`, "{)", 1, 4)),
		}, []v007Expected{{3, 3, 4, v007CloseDetail}}},
		{"single-line comment", []model.Line{
			v007Line("@editor (", "tag", 2, 1,
				v007Element("tag", "@editor", "editor", 1, 8), v007Element("content", "(", "(", 9, 10)),
		}, []v007Expected{{3, 9, 10, v007OpenDetail}}},
		{"block comment", []model.Line{
			v007Line("@editor {", "block-start", 2, 1,
				v007Element("tag", "@editor", "editor", 1, 8), v007Element("block-open", "{", "{", 9, 10)),
			v007Text("(", 3, 2),
			v007Line("}", "block-end", 3, 2, v007Element("block-close", "}", "}", 1, 2)),
		}, []v007Expected{{4, 1, 2, v007OpenDetail}}},
		// DSL 3.8.6, 3.8.11: инструкция содержит собственный текст и вложенный hint.
		{"nested hint", []model.Line{
			v007Line("@instr {", "block-start", 2, 1,
				v007Element("tag", "@instr", "instr", 1, 7), v007Element("block-open", "{", "{", 8, 9)),
			v007Text("Read.", 3, 2),
			v007Line("@hint {", "block-start", 3, 2,
				v007Element("tag", "@hint", "hint", 1, 6), v007Element("block-open", "{", "{", 7, 8)),
			v007Text("(", 5, 3),
			v007Line("}", "block-end", 5, 3, v007Element("block-close", "}", "}", 1, 2)),
			v007Line("}", "block-end", 3, 2, v007Element("block-close", "}", "}", 1, 2)),
		}, []v007Expected{{6, 1, 2, v007OpenDetail}}},
		// DSL 3.2.8: обе части example вокруг разделителя имеют содержимое.
		{"separator and block end never carry openings", []model.Line{
			v007Line("@example {", "block-start", 2, 1,
				v007Element("tag", "@example", "example", 1, 9), v007Element("block-open", "{", "{", 10, 11)),
			v007Text("(", 3, 2), v007Line("---", "separator", 3, 2), v007Text(")", 3, 2),
			v007Line("}", "block-end", 3, 2, v007Element("block-close", "}", "}", 1, 2)),
			v007Text(")", 2, 1),
		}, []v007Expected{{4, 1, 2, v007OpenDetail}, {6, 1, 2, v007CloseDetail}, {8, 1, 2, v007CloseDetail}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v007Run(t, v007Document(tc.body...), "current", tc.want)
		})
	}
}

func TestV007ResourcePath(t *testing.T) {
	result := v007Document()
	// DSL 1.31 и 1.35: метаданные находятся перед заголовком; кавычки ограничивают путь.
	resource := v007Line(`@resource-dir "Audio("`, "tag", 0, 0,
		v007Element("tag", "@resource-dir", "resource-dir", 1, 14),
		v007Element("resource-path", `"Audio("`, "Audio(", 15, 23))
	resource.Number = 2
	heading := result.Lines[1]
	heading.Number = 3
	result.Lines = []model.Line{result.Lines[0], resource, heading}
	count := 3
	result.Document.LineCount = &count
	result.Document.Metadata.ResourceDirs = []string{"Audio("}
	v007Run(t, result, "current", []v007Expected{{2, 21, 22, v007OpenDetail}})
}

func TestV007InvalidLine(t *testing.T) {
	// DSL 1.16: неизвестное имя тега ошибочно. Код прежней ошибки — непрозрачная
	// тестовая история, не предположение о конкретном коде внешнего парсера.
	line := v007Line("@unknown (", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown (", Start: 1, End: 11, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	result := v007Document(line)
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{
		ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 11}},
		RelatedLocations: []model.Location{},
	}}
	v007Run(t, result, "current", []v007Expected{{3, 10, 11, v007OpenDetail}})
}

func TestV007LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [4]model.LineEnding
	}{
		{"LF", [4]model.LineEnding{"\n", "\n", "\n", "\n"}},
		{"CRLF", [4]model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n"}},
		{"LF without final EOL", [4]model.LineEnding{"\n", "\n", "\n", ""}},
		{"CRLF without final EOL", [4]model.LineEnding{"\r\n", "\r\n", "\r\n", ""}},
		{"mixed endings", [4]model.LineEnding{"\r\n", "\n", "\r\n", "\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v007Document(v007Text("Read (the word.", 2, 1), v007Text("Continue).", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v007Run(t, result, "current", []v007Expected{{3, 6, 7, v007OpenDetail}, {4, 9, 10, v007CloseDetail}})
		})
	}
}

func TestV007UnavailableLines(t *testing.T) {
	// Контракт Lines допускает отсутствие строк при известной версии документа.
	v007Run(t, v007UnavailableDocument("1.2"), "current", nil)
}

func TestV007UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v007UnavailableDocument(version)
			before := v007Snapshot(t, result)
			checks, err := validator.Prepare(result, v007Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v007Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV007RepeatedRuns(t *testing.T) {
	first := v007Document(v007Text("(", 2, 1))
	v007Run(t, first, "first", []v007Expected{{3, 1, 2, v007OpenDetail}})
	v007Run(t, v007Document(v007Text(")", 2, 1)), "first", []v007Expected{{3, 1, 2, v007CloseDetail}})
	v007Run(t, first, "second", []v007Expected{{3, 1, 2, v007OpenDetail}})
	v007Run(t, v007Document(v007Text("()", 2, 1)), "first", nil)
}

// v007Registry использует только штатный каталог без замены его регистраций.
func v007Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v007UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v007UnavailableDocument(version string) *model.Result {
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v007Document дополняет фрагмент обязательными @dsl-version и заголовком
// по DSL 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по 2.1.15.
// Элементы, родители и глубина тела заданы явно; повторного разбора DSL нет.
func v007Document(body ...model.Line) *model.Result {
	result := v007UnavailableDocument("1.2")
	result.Lines = []model.Line{
		v007Line("@dsl-version 1.2", "tag", 0, 0,
			v007Element("tag", "@dsl-version", "dsl-version", 1, 13),
			v007Element("version", "1.2", "1.2", 14, 17)),
		v007Line("# Unit 1", "heading", 0, 0,
			v007Element("heading-level", "#", "1", 1, 2), v007Element("title", "Unit 1", "Unit 1", 3, 9)),
	}
	result.Lines = append(result.Lines, body...)
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count, title, encoding := len(result.Lines), "Unit 1", "UTF-8"
	result.Document.LineCount = &count
	result.Document.Metadata.Title = &title
	result.Document.Encoding = &encoding
	return result
}

// v007Line сохраняет явно заданную структуру физической строки с LF.
func v007Line(raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v007Element сохраняет явно заданные Raw, Value и исходный диапазон.
func v007Element(kind model.ElementType, raw, value string, start, end int) model.Element {
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v007Text создаёт содержимое без краевых пробелов и снимаемого экранирования.
// Вычисление длины относится только к входной модели, не к ожидаемым результатам.
func v007Text(text string, parent, depth int) model.Line {
	return v007Line(text, "content", parent, depth,
		v007Element("content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v007Run проверяет только V007 текущего запуска через полный штатный конвейер.
// Ожидания заданы независимо от Check; история и исходные данные сохраняются.
func v007Run(t *testing.T, result *model.Result, processingID string, want []v007Expected) {
	t.Helper()
	before := v007Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v007Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v007Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V007" && entry.Source != processingID {
			t.Fatalf("new V007 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V007" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V007 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "Нарушена парность круглых скобок в строке.\nконкретизация: " + expected.detail
		if entry.DiagnosticCode != "V007" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != message || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v; want message %q", i, entry, message)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	// Prepare проверяет флаги и связи полного результата по контракту модели.
	// Затем из копии убираем только эффекты соседних диагностик для снимка
	// исходных данных; ожидания V007 и прежняя история остаются неизменными.
	if _, err := validator.Prepare(result, v007Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var previous, original model.Result
	if err := json.Unmarshal([]byte(v007Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	otherNewIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V007" {
			otherNewIDs[entry.ID] = true
		}
	}
	previous.Document.HasErrors = original.Document.HasErrors
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range previous.Lines[i].Elements {
			element := &previous.Lines[i].Elements[j]
			kept := element.ErrorIDs[:0]
			for _, id := range element.ErrorIDs {
				if !otherNewIDs[id] {
					kept = append(kept, id)
				}
			}
			element.ErrorIDs = kept
		}
	}
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v007Snapshot(t, &previous) != before {
		t.Fatal("document, lines, flags, element links or previous history changed")
	}
}

// v007Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v007Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

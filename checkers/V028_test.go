package checkers

import (
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v028Expected задаёт исходный диапазон и конкретизацию независимо от проверки.
type v028Expected struct {
	line, start, end int
	detail           string
}

const (
	v028After  = "После открывающей скобки"
	v028Before = "Перед закрывающей скобкой"
	v028Both   = "Между открывающей и закрывающей скобками"
)

func TestV028Registration(t *testing.T) {
	definitions, err := v028Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V028" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Пробел находится с внутренней стороны скобки." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V028 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V028 definitions = %d, want 1", matches)
	}
}

func TestV028Text(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v028Expected
	}{
		{"reference", "Read (carefully).", nil},
		{"example", "Read ( carefully ).", []v028Expected{{3, 7, 8, v028After}, {3, 17, 18, v028Before}}},
		{"round single space allowed", "( )", nil},
		{"square single space allowed", "[ ]", nil},
		{"exceptions inside text", "А ( ) Б [ ] В", nil},
		{"round two spaces", "(  )", []v028Expected{{3, 2, 4, v028Both}}},
		{"square two spaces", "[  ]", []v028Expected{{3, 2, 4, v028Both}}},
		{"round three spaces", "(   )", []v028Expected{{3, 2, 5, v028Both}}},
		{"square three spaces", "[   ]", []v028Expected{{3, 2, 5, v028Both}}},
		{"round opening and square closing", "( ]", []v028Expected{{3, 2, 3, v028Both}}},
		{"square opening and round closing", "[ )", []v028Expected{{3, 2, 3, v028Both}}},
		{"after round opening", "( а)", []v028Expected{{3, 2, 3, v028After}}},
		{"after square opening", "[ а]", []v028Expected{{3, 2, 3, v028After}}},
		{"before round closing", "(а )", []v028Expected{{3, 3, 4, v028Before}}},
		{"before square closing", "[а ]", []v028Expected{{3, 3, 4, v028Before}}},
		{"long run after opening", "(   а)", []v028Expected{{3, 2, 5, v028After}}},
		{"long run before closing", "(а   )", []v028Expected{{3, 3, 6, v028Before}}},
		{"external spaces", "А (Б) В", nil},
		{"no parentheses", "А  Б", nil},
		{"empty round pair", "()", nil},
		{"empty square pair", "[]", nil},
		{"four occurrences", "( а ) [ б ]", []v028Expected{
			{3, 2, 3, v028After}, {3, 4, 5, v028Before}, {3, 8, 9, v028After}, {3, 10, 11, v028Before},
		}},
		{"nested round exception", "(( ))", nil},
		{"nested square exception", "([ ])", nil},
		{"inner exception does not suppress outer spaces", "( [ ] )", []v028Expected{{3, 2, 3, v028After}, {3, 6, 7, v028Before}}},
		{"tab only", "(\t)", nil},
		{"space followed by tab", "( \t)", []v028Expected{{3, 2, 3, v028After}}},
		{"space preceded by tab", "(\t )", []v028Expected{{3, 3, 4, v028Before}}},
		{"nonbreaking space separates runs", "( \u00a0 )", []v028Expected{{3, 2, 3, v028After}, {3, 4, 5, v028Before}}},
		{"Unicode columns", "Я😀( а)", []v028Expected{{3, 4, 5, v028After}}},
		{"combining mark is a column", "e\u0301( а)", []v028Expected{{3, 4, 5, v028After}}},
		{"uppercase", "( А)", []v028Expected{{3, 2, 3, v028After}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v028Run(t, v028Document(t, v028Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV028OnlyOrdinarySpace(t *testing.T) {
	// Явные примеры всех Zs кроме U+0020, а также TAB и вертикальных разделителей.
	// Ожидание не вычисляется классификатором из реализации.
	for _, space := range []rune{
		'\t', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005',
		'\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u202f', '\u205f', '\u3000',
		'\v', '\f', '\u0085', '\u2028', '\u2029', '\u200b',
	} {
		t.Run(fmt.Sprintf("U+%04X", space), func(t *testing.T) {
			v028Run(t, v028Document(t, v028Text(t, "("+string(space)+") ["+string(space)+"]", 2, 1)), "current", nil)
		})
	}
}

func TestV028RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v028Expected
	}{
		{"empty physical line", []model.Line{v028Line(t, "", "blank", 2, 1)}, nil},
		{"blank spaces", []model.Line{v028Line(t, "   ", "blank", 2, 1)}, nil},
		// DSL 1.14: краевые пробелы не входят в элемент, но остаются в Raw.
		{"unmatched round opening at last character of element", []model.Line{v028Line(t, "( ", "content", 2, 1,
			v028Element(t, "content", "(", "(", 1, 2))}, []v028Expected{{3, 2, 3, v028After}}},
		{"unmatched square opening", []model.Line{v028Line(t, "[ ", "content", 2, 1,
			v028Element(t, "content", "[", "[", 1, 2))}, []v028Expected{{3, 2, 3, v028After}}},
		{"unmatched square closing at first character of element", []model.Line{v028Line(t, " ]", "content", 2, 1,
			v028Element(t, "content", "]", "]", 2, 3))}, []v028Expected{{3, 1, 2, v028Before}}},
		{"unmatched round closing", []model.Line{v028Line(t, " )", "content", 2, 1,
			v028Element(t, "content", ")", ")", 2, 3))}, []v028Expected{{3, 1, 2, v028Before}}},
		{"raw differs from semantic value", []model.Line{v028Line(t, `\{( а)\}`, "content", 2, 1,
			v028Element(t, "content", `\{( а)\}`, "{( а)}", 1, 9))}, []v028Expected{{3, 4, 5, v028After}}},
		{"literal curly brackets do not participate", []model.Line{v028Line(t, `А \{ Б \} В`, "content", 2, 1,
			v028Element(t, "content", `А \{ Б \} В`, "А { Б } В", 1, 12))}, nil},
		{"no exception across lines", []model.Line{
			v028Text(t, "(", 2, 1),
			v028Line(t, " )", "content", 2, 1, v028Element(t, "content", ")", ")", 2, 3)),
		}, []v028Expected{{4, 1, 2, v028Before}}},
		{"single-line comment", []model.Line{v028Line(t, "@editor ( а)", "tag", 2, 1,
			v028Element(t, "tag", "@editor", "editor", 1, 8), v028Element(t, "content", "( а)", "( а)", 9, 13))},
			[]v028Expected{{3, 10, 11, v028After}}},
		{"block comment", []model.Line{
			v028Line(t, "@editor {", "block-start", 2, 1,
				v028Element(t, "tag", "@editor", "editor", 1, 8), v028Element(t, "block-open", "{", "{", 9, 10)),
			v028Text(t, "( а)", 3, 2),
			v028Line(t, "}", "block-end", 3, 2, v028Element(t, "block-close", "}", "}", 1, 2)),
		}, []v028Expected{{4, 2, 3, v028After}}},
		// DSL 3.8.6, 3.8.11: инструкция содержит собственный текст и один hint.
		{"nested hint", []model.Line{
			v028Line(t, "@instr {", "block-start", 2, 1,
				v028Element(t, "tag", "@instr", "instr", 1, 7), v028Element(t, "block-open", "{", "{", 8, 9)),
			v028Text(t, "Read.", 3, 2),
			v028Line(t, "@hint {", "block-start", 3, 2,
				v028Element(t, "tag", "@hint", "hint", 1, 6), v028Element(t, "block-open", "{", "{", 7, 8)),
			v028Text(t, "( а)", 5, 3),
			v028Line(t, "}", "block-end", 5, 3, v028Element(t, "block-close", "}", "}", 1, 2)),
			v028Line(t, "}", "block-end", 3, 2, v028Element(t, "block-close", "}", "}", 1, 2)),
		}, []v028Expected{{6, 2, 3, v028After}}},
		{"HTML is not parsed", []model.Line{
			v028Line(t, "@text {", "block-start", 2, 1,
				v028Element(t, "tag", "@text", "text", 1, 6), v028Element(t, "block-open", "{", "{", 7, 8)),
			v028Text(t, "<em>( а)</em>", 3, 2),
			v028Line(t, "}", "block-end", 3, 2, v028Element(t, "block-close", "}", "}", 1, 2)),
		}, []v028Expected{{4, 6, 7, v028After}}},
		// DSL 3.4: одна строка двухколоночной таблицы без заголовочной части.
		{"table is not parsed", []model.Line{
			v028Line(t, "@table {", "block-start", 2, 1,
				v028Element(t, "tag", "@table", "table", 1, 7), v028Element(t, "block-open", "{", "{", 8, 9)),
			v028Line(t, "( а)|Б", "content", 3, 2,
				v028Element(t, "content", "( а)", "( а)", 1, 5), v028Element(t, "content", "Б", "Б", 6, 7)),
			v028Line(t, "}", "block-end", 3, 2, v028Element(t, "block-close", "}", "}", 1, 2)),
		}, []v028Expected{{4, 2, 3, v028After}}},
		// DSL 3.2.8: обе части example вокруг разделителя непустые.
		{"structural curly brackets and separator", []model.Line{
			v028Line(t, "@example { ", "block-start", 2, 1,
				v028Element(t, "tag", "@example", "example", 1, 9), v028Element(t, "block-open", "{", "{", 10, 11)),
			v028Text(t, "А", 3, 2), v028Line(t, " --- ", "separator", 3, 2), v028Text(t, "Б", 3, 2),
			v028Line(t, " } ", "block-end", 3, 2, v028Element(t, "block-close", "}", "}", 2, 3)),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v028Run(t, v028Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV028Unparsed(t *testing.T) {
	// DSL 1.16: неизвестный тег ошибочен. Код сохранённой ошибки непрозрачен
	// для валидатора и не выдаётся за конкретный код внешнего парсера.
	line := v028Line(t, "@unknown ( а)", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown ( а)", Start: 1, End: 14, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	result := v028Document(t, line)
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{
		ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 14}},
		RelatedLocations: []model.Location{},
	}}
	v028Run(t, result, "current", []v028Expected{{3, 11, 12, v028After}})
}

func TestV028TitleNameAndPath(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v028Document(t)
		title := "( а)"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v028Line(t, "# ( а)", "heading", 0, 0,
			v028Element(t, "heading-level", "#", "1", 1, 2), v028Element(t, "title", title, title, 3, 7))
		result.Lines[1].Number = 2
		v028Run(t, result, "current", []v028Expected{{2, 4, 5, v028After}})
	})
	for _, tc := range []struct {
		name string
		line model.Line
		want []v028Expected
	}{
		{"section name", v028Line(t, "@section ( а)", "tag", 0, 0,
			v028Element(t, "tag", "@section", "section", 1, 9), v028Element(t, "name", "( а)", "( а)", 10, 14)),
			[]v028Expected{{2, 11, 12, v028After}}},
		{"resource path", v028Line(t, `@resource-dir "( а)"`, "tag", 0, 0,
			v028Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v028Element(t, "resource-path", `"( а)"`, "( а)", 15, 21)), []v028Expected{{2, 17, 18, v028After}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.31: метаданные располагаются до заголовка первого уровня.
			result := v028Document(t)
			heading := result.Lines[1]
			heading.Number = 3
			tc.line.Number = 2
			result.Lines = []model.Line{result.Lines[0], tc.line, heading}
			count := 3
			result.Document.LineCount = &count
			if tc.name == "section name" {
				section := "( а)"
				result.Document.Metadata.Section = &section
			} else {
				result.Document.Metadata.ResourceDirs = []string{"( а)"}
			}
			v028Run(t, result, "current", tc.want)
		})
	}
}

func TestV028LineEndings(t *testing.T) {
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
			result := v028Document(t, v028Text(t, "Read ( carefully ).", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v028Run(t, result, "current", []v028Expected{{3, 7, 8, v028After}, {3, 17, 18, v028Before}})
		})
	}
}

func TestV028UnavailableLines(t *testing.T) {
	// Контракт модели допускает известную версию при отсутствии доступных строк.
	v028Run(t, v028UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV028UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v028UnavailableDocument(t, version)
			before := v028Snapshot(t, result)
			checks, err := validator.Prepare(result, v028Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v028Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV028RepeatedRuns(t *testing.T) {
	result := v028Document(t, v028Text(t, "(  )", 2, 1))
	v028Run(t, result, "first", []v028Expected{{3, 2, 4, v028Both}})
	v028Run(t, result, "second", []v028Expected{{3, 2, 4, v028Both}})
	v028Run(t, v028Document(t, v028Text(t, "( ) [ ]", 2, 1)), "first", nil)
}

// v028Registry создаёт реестр только через штатный каталог без замены регистраций.
func v028Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v028UnavailableDocument задаёт известную версию при недоступном исходнике.
func v028UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v028Document дополняет фрагмент обязательными @dsl-version и заголовком
// по DSL 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по 2.1.15.
// Элементы, родители и глубина тела заданы явно; повторного разбора DSL нет.
func v028Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v028UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{
		v028Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v028Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v028Element(t, "version", "1.2", "1.2", 14, 17)),
		v028Line(t, "# Unit 1", "heading", 0, 0,
			v028Element(t, "heading-level", "#", "1", 1, 2), v028Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v028Line сохраняет заданную структуру физической строки с LF без разбора Raw.
func v028Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v028Element сохраняет заданные Raw, Value и исходный Unicode-диапазон.
func v028Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v028Text создаёт содержимое без краевых пробелов и снимаемого экранирования.
// Длина вычисляется только для входного элемента, не для ожидаемых результатов.
func v028Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v028Line(t, text, "content", parent, depth,
		v028Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v028Run проверяет V028 текущего запуска через полный штатный конвейер.
// Все координаты и конкретизации задаются тестом независимо от реализации.
func v028Run(t *testing.T, result *model.Result, processingID string, want []v028Expected) {
	t.Helper()
	before := v028Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v028Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v028Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V028" && entry.Source != processingID {
			t.Fatalf("new V028 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V028" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V028 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "Пробел находится с внутренней стороны скобки.\nконкретизация: " + expected.detail
		if entry.DiagnosticCode != "V028" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
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
	// исходных данных; ожидания V028 и прежняя история остаются неизменными.
	if _, err := validator.Prepare(result, v028Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var previous, original model.Result
	if err := json.Unmarshal([]byte(v028Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	otherNewIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V028" {
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
	if v028Snapshot(t, &previous) != before {
		t.Fatal("document, lines, flags, element links or previous history changed")
	}
}

// v028Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v028Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

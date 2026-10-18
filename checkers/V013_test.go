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

func TestV013Registration(t *testing.T) {
	definitions, err := v013Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V013" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Между словами стоит дефис, отделённый пробелами." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V013 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V013 definitions = %d, want 1", matches)
	}
}

func TestV013Text(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Москва – столица России.", nil},
		{"example", "Москва - столица России.", [][3]int{{3, 8, 9}}},
		{"no hyphen", "Москва столица России.", nil},
		{"hyphen only", "-", nil},
		{"no spaces", "А-Б", nil},
		{"space only on left", "А -Б", nil},
		{"space only on right", "А- Б", nil},
		{"first character", "- А", nil},
		{"unindented list marker", "- пункт", nil},
		{"last character", "А -", nil},
		{"double hyphen", "А -- Б", nil},
		{"triple hyphen", "А --- Б", nil},
		{"mixed runs and single hyphen", "А -- Б - В --- Г", [][3]int{{3, 8, 9}}},
		{"two occurrences", "А - Б - В", [][3]int{{3, 3, 4}, {3, 7, 8}}},
		{"shared space", "А - - Б", [][3]int{{3, 3, 4}, {3, 5, 6}}},
		{"numbers are included", "10 - 20", [][3]int{{3, 4, 5}}},
		{"punctuation is included", "! - ?", [][3]int{{3, 3, 4}}},
		{"Unicode columns", "Я😀 - Б", [][3]int{{3, 4, 5}}},
		{"combining mark is a column", "e\u0301 - Б", [][3]int{{3, 4, 5}}},
		{"upper case", "МОСКВА - СТОЛИЦА", [][3]int{{3, 8, 9}}},
		{"lower case", "москва - столица", [][3]int{{3, 8, 9}}},
		{"horizontal runs", "А \t-\u00a0 Б", [][3]int{{3, 4, 5}}},
		{"em dash is not hyphen", "А — Б", nil},
		{"minus sign is not hyphen", "А − Б", nil},
		{"nonbreaking hyphen is not ASCII hyphen", "А ‑ Б", nil},
		{"HTML entity is not decoded", "А &#45; Б", nil},
		{"zero width spaces do not qualify", "А\u200b-\u200bБ", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v013Run(t, v013Document(t, v013Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV013HorizontalSpaces(t *testing.T) {
	// Перечень входов задан явно: TAB и все 17 символов категории Zs.
	// Ожидаемые результаты не вычисляются проверяемым классификатором.
	for _, space := range []rune{
		'\t', '\u0020', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004',
		'\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u202f', '\u205f', '\u3000',
	} {
		t.Run(fmt.Sprintf("U+%04X", space), func(t *testing.T) {
			for _, tc := range []struct{ name, raw string }{
				{"both sides", "А" + string(space) + "-" + string(space) + "Б"},
				{"left side", "А" + string(space) + "- Б"},
				{"right side", "А -" + string(space) + "Б"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					v013Run(t, v013Document(t, v013Text(t, tc.raw, 2, 1)), "current", [][3]int{{3, 3, 4}})
				})
			}
		})
	}
}

func TestV013VerticalSpaces(t *testing.T) {
	// LF и CR входят в EOL, а не в Raw; они проверяются отдельно на границах строк.
	for _, space := range []rune{'\v', '\f', '\u0085', '\u2028', '\u2029'} {
		t.Run(fmt.Sprintf("U+%04X", space), func(t *testing.T) {
			for _, tc := range []struct{ name, raw string }{
				{"both sides", "А" + string(space) + "-" + string(space) + "Б"},
				{"left side", "А" + string(space) + "- Б"},
				{"right side", "А -" + string(space) + "Б"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					v013Run(t, v013Document(t, v013Text(t, tc.raw, 2, 1)), "current", nil)
				})
			}
		})
	}
}

func TestV013RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty physical line", []model.Line{v013Line(t, "", "blank", 2, 1)}, nil},
		{"blank whitespace", []model.Line{v013Line(t, " \t ", "blank", 2, 1)}, nil},
		// DSL 1.14: краевые пробелы отсутствуют в элементе, но остаются в Raw.
		{"shortest match", []model.Line{v013Line(t, " - ", "content", 2, 1,
			v013Element(t, "content", "-", "-", 2, 3))}, [][3]int{{3, 2, 3}}},
		{"only trailing space", []model.Line{v013Line(t, "- ", "content", 2, 1,
			v013Element(t, "content", "-", "-", 1, 2))}, nil},
		{"only leading space", []model.Line{v013Line(t, " -", "content", 2, 1,
			v013Element(t, "content", "-", "-", 2, 3))}, nil},
		{"indented list marker", []model.Line{v013Line(t, "  - пункт", "content", 2, 1,
			v013Element(t, "content", "- пункт", "- пункт", 3, 10))}, [][3]int{{3, 3, 4}}},
		{"raw differs from semantic value", []model.Line{v013Line(t, `\{А - Б\}`, "content", 2, 1,
			v013Element(t, "content", `\{А - Б\}`, "{А - Б}", 1, 10))}, [][3]int{{3, 5, 6}}},
		{"hyphen before line boundary", []model.Line{
			v013Text(t, "А -", 2, 1),
			v013Line(t, " Б", "content", 2, 1, v013Element(t, "content", "Б", "Б", 2, 3)),
		}, nil},
		{"hyphen after line boundary", []model.Line{
			v013Line(t, "А ", "content", 2, 1, v013Element(t, "content", "А", "А", 1, 2)),
			v013Text(t, "- Б", 2, 1),
		}, nil},
		{"single-line comment", []model.Line{v013Line(t, "@editor Москва - столица", "tag", 2, 1,
			v013Element(t, "tag", "@editor", "editor", 1, 8),
			v013Element(t, "content", "Москва - столица", "Москва - столица", 9, 25))}, [][3]int{{3, 16, 17}}},
		{"block comment", []model.Line{
			v013Line(t, "@editor {", "block-start", 2, 1,
				v013Element(t, "tag", "@editor", "editor", 1, 8), v013Element(t, "block-open", "{", "{", 9, 10)),
			v013Text(t, "А - Б", 3, 2),
			v013Line(t, "}", "block-end", 3, 2, v013Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 3, 4}}},
		// DSL 3.8.6, 3.8.11: собственный текст инструкции и один вложенный hint.
		{"nested hint", []model.Line{
			v013Line(t, "@instr {", "block-start", 2, 1,
				v013Element(t, "tag", "@instr", "instr", 1, 7), v013Element(t, "block-open", "{", "{", 8, 9)),
			v013Text(t, "Read.", 3, 2),
			v013Line(t, "@hint {", "block-start", 3, 2,
				v013Element(t, "tag", "@hint", "hint", 1, 6), v013Element(t, "block-open", "{", "{", 7, 8)),
			v013Text(t, "А - Б", 5, 3),
			v013Line(t, "}", "block-end", 5, 3, v013Element(t, "block-close", "}", "}", 1, 2)),
			v013Line(t, "}", "block-end", 3, 2, v013Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{6, 3, 4}}},
		// DSL 3.2.8: до и после разделителя обязательно есть содержимое.
		{"structural separator", []model.Line{
			v013Line(t, "@example {", "block-start", 2, 1,
				v013Element(t, "tag", "@example", "example", 1, 9), v013Element(t, "block-open", "{", "{", 10, 11)),
			v013Text(t, "А", 3, 2), v013Line(t, "  ---\t", "separator", 3, 2), v013Text(t, "Б", 3, 2),
			v013Line(t, "}", "block-end", 3, 2, v013Element(t, "block-close", "}", "}", 1, 2)),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v013Run(t, v013Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV013TitleNameAndPath(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v013Document(t)
		title := "Москва - столица"
		result.Lines[1] = v013Line(t, "# Москва - столица", "heading", 0, 0,
			v013Element(t, "heading-level", "#", "1", 1, 2), v013Element(t, "title", title, title, 3, 19))
		result.Lines[1].Number = 2
		result.Document.Metadata.Title = &title
		v013Run(t, result, "current", [][3]int{{2, 10, 11}})
	})
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"section name", v013Line(t, "@section Москва - столица", "tag", 0, 0,
			v013Element(t, "tag", "@section", "section", 1, 9),
			v013Element(t, "name", "Москва - столица", "Москва - столица", 10, 26)), [][3]int{{2, 17, 18}}},
		{"resource path", v013Line(t, `@resource-dir "Audio - Shared"`, "tag", 0, 0,
			v013Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v013Element(t, "resource-path", `"Audio - Shared"`, "Audio - Shared", 15, 31)), [][3]int{{2, 22, 23}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.31: метаданные располагаются перед заголовком первого уровня.
			result := v013Document(t)
			heading := result.Lines[1]
			heading.Number = 3
			tc.line.Number = 2
			result.Lines = []model.Line{result.Lines[0], tc.line, heading}
			count := 3
			result.Document.LineCount = &count
			if tc.name == "section name" {
				section := "Москва - столица"
				result.Document.Metadata.Section = &section
			} else {
				result.Document.Metadata.ResourceDirs = []string{"Audio - Shared"}
			}
			v013Run(t, result, "current", tc.want)
		})
	}
}

func TestV013HTML(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"formatted text", "<strong>Москва - столица</strong>", [][3]int{{4, 16, 17}}},
		{"markup after hyphen", "Москва - <em>столица</em>", [][3]int{{4, 8, 9}}},
		{"markup before hyphen", "<u>Москва</u> - столица", [][3]int{{4, 15, 16}}},
		{"no space before markup", "Москва -<em>столица</em>", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v013Document(t,
				v013Line(t, "@text {", "block-start", 2, 1,
					v013Element(t, "tag", "@text", "text", 1, 6), v013Element(t, "block-open", "{", "{", 7, 8)),
				v013Text(t, tc.raw, 3, 2),
				v013Line(t, "}", "block-end", 3, 2, v013Element(t, "block-close", "}", "}", 1, 2)))
			v013Run(t, result, "current", tc.want)
		})
	}
	t.Run("hyphen inside unsupported HTML marker", func(t *testing.T) {
		// DSL 3.6.14–3.6.15: атрибуты и span ошибочны. Уточнение пользователя
		// требует проверить дефис и внутри такого сохранённого unparsed.
		raw := `<span title="Москва - столица">`
		result := v013Document(t,
			v013Line(t, "@text {", "block-start", 2, 1,
				v013Element(t, "tag", "@text", "text", 1, 6), v013Element(t, "block-open", "{", "{", 7, 8)),
			v013Line(t, raw, "invalid", 3, 2, model.Element{
				ElementType: "unparsed", Raw: raw, Start: 1, End: 32, ErrorIDs: []string{},
			}),
			v013Line(t, "}", "block-end", 3, 2, v013Element(t, "block-close", "}", "}", 1, 2)))
		v013ParserError(t, result, 4, 0, "Неподдерживаемый HTML-тег с атрибутом.")
		v013Run(t, result, "current", [][3]int{{4, 21, 22}})
	})
}

func TestV013Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"unknown tag with spaced hyphen", "@unknown Москва - столица", [][3]int{{3, 17, 18}}},
		{"unknown tag without spaced hyphen", "@unknown Москва-столица", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.16: неизвестное имя тега ошибочно, но unparsed тоже проверяется.
			result := v013Document(t, v013Line(t, tc.raw, "invalid", 2, 1, model.Element{
				ElementType: "unparsed", Raw: tc.raw, Start: 1, End: utf8.RuneCountInString(tc.raw) + 1,
				ErrorIDs: []string{},
			}))
			v013ParserError(t, result, 3, 0, "Неизвестный тег.")
			v013Run(t, result, "current", tc.want)
		})
	}
}

func TestV013LineEndings(t *testing.T) {
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
			result := v013Document(t, v013Text(t, "Москва - столица России.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v013Run(t, result, "current", [][3]int{{3, 8, 9}})
		})
	}
}

func TestV013UnavailableLines(t *testing.T) {
	// Известная версия при недоступном исходнике допустима по контракту модели.
	v013Run(t, v013UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV013UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v013UnavailableDocument(t, version)
			before := v013Snapshot(t, result)
			checks, err := validator.Prepare(result, v013Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v013Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV013RepeatedRuns(t *testing.T) {
	result := v013Document(t, v013Text(t, "А - Б", 2, 1))
	v013Run(t, result, "first", [][3]int{{3, 3, 4}})
	v013Run(t, result, "second", [][3]int{{3, 3, 4}})
	v013Run(t, v013Document(t, v013Text(t, "А – Б", 2, 1)), "first", nil)
}

// v013Registry создаёт реестр исключительно через штатный каталог.
func v013Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v013UnavailableDocument задаёт известную версию при отсутствии исходных строк.
func v013UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v013Document добавляет @dsl-version и заголовок по DSL 1.1–1.3, 1.30, 2.1.3.
// Обычный текст образует шаг по 2.1.15. Тело уже содержит явно заданную структуру;
// ни классификация строк, ни разбор элементов здесь не выполняются.
func v013Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v013UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{
		v013Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v013Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v013Element(t, "version", "1.2", "1.2", 14, 17)),
		v013Line(t, "# Unit 1", "heading", 0, 0,
			v013Element(t, "heading-level", "#", "1", 1, 2), v013Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v013Line сохраняет заданные тип, родителей и элементы без интерпретации Raw.
func v013Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v013Element сохраняет явно заданные исходное и семантическое значения и диапазон.
func v013Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v013Text создаёт содержимое без краевых пробелов и снимаемого экранирования.
// Вычисление длины задаёт только входной элемент, а не ожидаемые срабатывания.
func v013Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v013Line(t, text, "content", parent, depth,
		v013Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v013ParserError связывает заданный ошибочный элемент с сохранённой историей.
// Код ошибки тестовый и непрозрачный для валидатора, не заимствован у парсера.
func v013ParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	element.ErrorIDs = []string{"parser-error"}
	line.HasErrors = true
	result.Document.HasErrors = true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message,
		Location: &model.Location{
			Start: model.Position{Line: number, Column: element.Start},
			End:   model.Position{Line: number, Column: element.End},
		},
		RelatedLocations: []model.Location{},
	})
}

// v013Run проверяет V013 текущего запуска через полный штатный конвейер.
// Ожидаемые тройки задают строку, начало и исключающий конец диапазона.
func v013Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v013Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v013Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v013Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V013" && entry.Source != processingID {
			t.Fatalf("new V013 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V013" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V013 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V013" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.Message != "Между словами стоит дефис, отделённый пробелами.\nконкретизация: Для разделения частей фразы рекомендуется короткое тире –" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	// Prepare проверяет флаги и связи полного результата по контракту модели.
	// Затем из копии убираем только эффекты соседних диагностик для снимка
	// исходных данных; ожидания V013 и прежняя история остаются неизменными.
	if _, err := validator.Prepare(result, v013Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var previous, original model.Result
	if err := json.Unmarshal([]byte(v013Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	otherNewIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V013" {
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
	if v013Snapshot(t, &previous) != before {
		t.Fatal("document, lines, flags, element links or previous history changed")
	}
}

// v013Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v013Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

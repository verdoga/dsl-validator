package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV012Registration(t *testing.T) {
	definitions, err := v012Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V012" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Между числами используется дефис или длинное тире." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V012 definitions = %d, want 1", matches)
	}
}

func TestV012SignsBetweenDigits(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Выполните задания 1–12.", nil},
		{"example", "Выполните задания 1-12.", [][3]int{{3, 20, 21}}},
		{"hyphen", "1-12", [][3]int{{3, 2, 3}}},
		{"em dash", "1—12", [][3]int{{3, 2, 3}}},
		{"long numbers", "123456-789012", [][3]int{{3, 7, 8}}},
		{"spaces", "1 - 12", [][3]int{{3, 3, 4}}},
		{"tabs", "1\t—\t2", [][3]int{{3, 3, 4}}},
		{"nonbreaking spaces", "1\u00a0-\u202f2", [][3]int{{3, 3, 4}}},
		{"mixed horizontal spaces", "1 \t\u2003-\u3000 2", [][3]int{{3, 5, 6}}},
		{"decimal punctuation", "1.5-2,5", [][3]int{{3, 4, 5}}},
		{"date", "2026-10-17", [][3]int{{3, 5, 6}, {3, 8, 9}}},
		{"long chain", "1-2—3", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"mixed dash chain", "1–2-3", [][3]int{{3, 4, 5}}},
		{"equation", "1-2=3", [][3]int{{3, 2, 3}}},
		{"arithmetic", "+1-2+3×4—5÷6*7/8", [][3]int{{3, 3, 4}, {3, 9, 10}}},
		{"alphanumeric uppercase", "A1-12_B", [][3]int{{3, 3, 4}}},
		{"alphanumeric lowercase", "a1-12_b", [][3]int{{3, 3, 4}}},
		{"leading number sign", "-1-2", [][3]int{{3, 3, 4}}},
		{"double hyphen", "1--2", nil},
		{"sign before second number", "1- -2", nil},
		{"missing right digit", "1-", nil},
		{"missing left digit", "-2", nil},
		{"no digits", "a-b", nil},
		{"sign alone", "—", nil},
		{"several matches", "1-2, 3—4", [][3]int{{3, 2, 3}, {3, 7, 8}}},
		{"Unicode columns", "Я😀 1-2", [][3]int{{3, 5, 6}}},
		{"combining mark columns", "e\u0301 1-2", [][3]int{{3, 5, 6}}},
		{"Unicode decimal digits", "١-٢", [][3]int{{3, 2, 3}}},
		{"fullwidth digits", "１—２", [][3]int{{3, 2, 3}}},
		{"superscripts are not Nd", "¹-²", nil},
		{"Unicode minus is not a target", "1−2", nil},
		{"nonbreaking hyphen is not a target", "1‑2", nil},
		{"zero width space is not Zs", "1\u200b-2", nil},
		{"line separator is not horizontal", "1\u2028-2", nil},
		{"paragraph separator is not horizontal", "1-\u20292", nil},
		{"HTML entity is not decoded", "1&#45;2", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wantErrors []v012ErrorExpectation
			if tc.name == "alphanumeric uppercase" || tc.name == "alphanumeric lowercase" {
				// V012 по-прежнему отмечает только дефис; одиночный _ даёт V148.
				wantErrors = []v012ErrorExpectation{{"V148", [3]int{3, 6, 7}}}
			}
			v012Run(t, v012Document(t, v012Text(t, tc.raw, tc.raw, 2, 1)), "current", tc.want, 1, wantErrors...)
		})
	}
}

func TestV012FormattingNeighbours(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		want       [][3]int
		wantErrors []v012ErrorExpectation
	}{
		{"1-<em>12</em>", [][3]int{{4, 2, 3}}, nil},
		{"<em>1</em>-12", [][3]int{{4, 11, 12}}, nil},
		{"1<em>-</em>12", [][3]int{{4, 6, 7}}, nil},
		{"Я😀 <u>١</u> \t— <EM>٢</EM>", [][3]int{{4, 14, 15}}, nil},
		{"1–<em>12</em>", nil, nil},
		{"a-<em>b</em>", nil, nil},
		{"1-<em>-2</em>", nil, nil},
		{`1-\<em>12</em>`, nil, []v012ErrorExpectation{{"V112", [3]int{4, 10, 15}}}},
		{`1-<em\>12</em>`, nil, []v012ErrorExpectation{{"V112", [3]int{4, 10, 15}}}},
		{"1-<foo>12</foo>", nil, []v012ErrorExpectation{{"V115", [3]int{4, 3, 8}}, {"V115", [3]int{4, 10, 16}}}},
		{"1-<em x=y>12</em>", nil, []v012ErrorExpectation{{"V114", [3]int{4, 7, 10}}}},
		{"1-<br>12", nil, []v012ErrorExpectation{{"V111", [3]int{4, 1, 9}}}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v012Run(t, v020TextBlock(t, tc.raw), "current", tc.want, 1, tc.wantErrors...)
		})
	}
	v012Run(t, v020TextBlock(t, "1-", "<em>12</em>"), "current", nil, 1)
	v012Run(t, v021Block(t, "editor", v021Text(t, "1-<em>12</em>", 3, 2)), "current", nil, 1)
	result := v020TextBlock(t, "1-<em>12</em>")
	result.Lines[3].Elements = []model.Element{
		v012Element(t, "content", "1-", "1-", 1),
		v012Element(t, "content", "<em>12</em>", "<em>12</em>", 3),
	}
	v012Run(t, result, "current", nil, 1)
}

func TestV012AnswerRepresentations(t *testing.T) {
	for _, separate := range []bool{false, true} {
		v012Run(t, v015AnswerDocument(t, "", "1-12", "", separate), "current", [][3]int{{4, 8, 9}}, 1)
		v012Run(t, v015AnswerDocument(t, "Я😀 ", `a\}١—٢`, " 3-4", separate), "current", [][3]int{{4, 14, 15}, {4, 19, 20}}, 1)
		v012Run(t, v015AnswerDocument(t, "1-", "12", "", separate), "current", nil, 1)
		v012Run(t, v015AnswerDocument(t, "", "1-", "12", separate), "current", nil, 1)
		v012Run(t, v015AnswerDocument(t, "", "1", "-12", separate), "current", nil, 1)
		v012Run(t, v015AnswerDocument(t, "", "1–12", "", separate), "current", nil, 1)
		v012Run(t, v015AnswerDocument(t, "", "1-<em>12</em>", "", separate), "current", nil, 1)
	}
}

func TestV012RawPositions(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"leading and trailing spaces", v012Line(t, " 1 - 2 ", "content", 2, 1,
			v012Element(t, "content", "1 - 2", "1 - 2", 2)), [][3]int{{3, 4, 5}}},
		// DSL 1.21–1.22: Value снимает экранирование скобок, Raw сохраняет колонки.
		{"escaped braces", v012Text(t, `\{1-2\}`, "{1-2}", 2, 1), [][3]int{{3, 4, 5}}},
		{"single-line editor", v012Line(t, "@editor 1-2", "tag", 2, 1,
			v012Element(t, "tag", "@editor", "editor", 1), v012Element(t, "content", "1-2", "1-2", 9)), [][3]int{{3, 10, 11}}},
		{"empty physical line", v012Line(t, "", "blank", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v012Run(t, v012Document(t, tc.line), "current", tc.want, 1)
		})
	}
}

func TestV012TitleAndName(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v012Document(t)
		title := "1-2"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v012Line(t, "# 1-2", "heading", 0, 0,
			v012Element(t, "heading-level", "#", "1", 1), v012Element(t, "title", "1-2", "1-2", 3))
		result.Lines[1].Number = 2
		v012Run(t, result, "current", [][3]int{{2, 4, 5}}, 1)
	})
	t.Run("section name before heading", func(t *testing.T) {
		result := v012Document(t)
		section := "1-2"
		result.Document.Metadata.Section = &section
		line := v012Line(t, "@section 1-2", "tag", 0, 0,
			v012Element(t, "tag", "@section", "section", 1), v012Element(t, "name", "1-2", "1-2", 10))
		result.Lines = append(result.Lines[:1], line, result.Lines[1])
		v012NumberLines(t, result)
		v012Run(t, result, "current", [][3]int{{2, 11, 12}}, 1)
	})
}

func TestV012BlocksAndElementBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"HTML-like literal content", []model.Line{
			v012Line(t, "@editor {", "block-start", 2, 1,
				v012Element(t, "tag", "@editor", "editor", 1), v012Element(t, "block-open", "{", "{", 9)),
			v012Text(t, `<x data="1-2">`, `<x data="1-2">`, 3, 2),
			v012Line(t, "}", "block-end", 3, 2, v012Element(t, "block-close", "}", "}", 1)),
		}, [][3]int{{4, 11, 12}}},
		{"nested content", []model.Line{
			v012Line(t, "@instr {", "block-start", 2, 1,
				v012Element(t, "tag", "@instr", "instr", 1), v012Element(t, "block-open", "{", "{", 8)),
			v012Text(t, "Read the sentence.", "Read the sentence.", 3, 2),
			v012Line(t, "@hint {", "block-start", 3, 2,
				v012Element(t, "tag", "@hint", "hint", 1), v012Element(t, "block-open", "{", "{", 7)),
			v012Text(t, "1-2", "1-2", 5, 3),
			v012Line(t, "}", "block-end", 5, 3, v012Element(t, "block-close", "}", "}", 1)),
			v012Line(t, "}", "block-end", 3, 2, v012Element(t, "block-close", "}", "}", 1)),
		}, [][3]int{{6, 2, 3}}},
		// DSL 3.4.5–3.4.9: две ячейки одной строки, без склейки их элементов.
		{"separate table cells", []model.Line{
			v012Line(t, "@table {", "block-start", 2, 1,
				v012Element(t, "tag", "@table", "table", 1), v012Element(t, "block-open", "{", "{", 8)),
			v012Line(t, "1- | 2", "content", 3, 2,
				v012Element(t, "content", "1-", "1-", 1), v012Element(t, "content", "2", "2", 6)),
			v012Line(t, "}", "block-end", 3, 2, v012Element(t, "block-close", "}", "}", 1)),
		}, nil},
		{"matches in separate table cells", []model.Line{
			v012Line(t, "@table {", "block-start", 2, 1,
				v012Element(t, "tag", "@table", "table", 1), v012Element(t, "block-open", "{", "{", 8)),
			v012Line(t, "1-2 | 3—4", "content", 3, 2,
				v012Element(t, "content", "1-2", "1-2", 1), v012Element(t, "content", "3—4", "3—4", 7)),
			v012Line(t, "}", "block-end", 3, 2, v012Element(t, "block-close", "}", "}", 1)),
		}, [][3]int{{4, 2, 3}, {4, 8, 9}}},
		{"separate physical lines", []model.Line{
			v012Text(t, "1-", "1-", 2, 1), v012Text(t, "2", "2", 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v012Run(t, v012Document(t, tc.body...), "current", tc.want, 1)
		})
	}
}

func TestV012ExcludedElements(t *testing.T) {
	t.Run("media source", func(t *testing.T) {
		line := v012Line(t, "@media audio 1-2.wav", "tag", 2, 1,
			v012Element(t, "tag", "@media", "media", 1), v012Element(t, "media-type", "audio", "audio", 8),
			v012Element(t, "source", "1-2.wav", "1-2.wav", 14))
		v012Run(t, v012Document(t, line), "current", nil, 1)
	})
	t.Run("resource path", func(t *testing.T) {
		result := v012Document(t)
		line := v012Line(t, "@resource-dir 1-2", "tag", 0, 0,
			v012Element(t, "tag", "@resource-dir", "resource-dir", 1), v012Element(t, "resource-path", "1-2", "1-2", 15))
		result.Lines = append(result.Lines[:1], line, result.Lines[1])
		result.Document.Metadata.ResourceDirs = []string{"1-2"}
		v012NumberLines(t, result)
		v012Run(t, result, "current", nil, 1)
	})
	t.Run("document identifier", func(t *testing.T) {
		result := v012Document(t)
		id := "1-2"
		line := v012Line(t, "@document-id 1-2", "tag", 0, 0,
			v012Element(t, "tag", "@document-id", "document-id", 1), v012Element(t, "identifier", "1-2", "1-2", 14))
		result.Lines = append(result.Lines[:1], line, result.Lines[1])
		result.Document.Metadata.DocumentID = &id
		v012NumberLines(t, result)
		v012Run(t, result, "current", nil, 0)
	})
	t.Run("unparsed with parser history", func(t *testing.T) {
		element := v012Element(t, "unparsed", "@unknown 1-2", "", 1)
		element.Value, element.ErrorIDs = nil, []string{"parser-error"}
		result := v012Document(t, v012Line(t, "@unknown 1-2", "invalid", 2, 1, element))
		result.Document.HasErrors, result.Lines[2].HasErrors = true, true
		result.Diagnostics = []model.Diagnostic{{
			ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
			DiagnosticScope: "element", Message: "Неизвестное имя тега.",
			Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 13}},
			RelatedLocations: []model.Location{},
		}}
		v012Run(t, result, "current", nil, 1)
	})
}

func TestV012LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}},
		{"no final EOL", [3]model.LineEnding{"\n", "\n", ""}},
		{"mixed endings", [3]model.LineEnding{"\r\n", "\n", "\r\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v012Document(t, v012Text(t, "Выполните задания 1-12.", "Выполните задания 1-12.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v012Run(t, result, "current", [][3]int{{3, 20, 21}}, 1)
		})
	}
}

func TestV012UnavailableLinesAndVersions(t *testing.T) {
	v012Run(t, v012UnavailableDocument(t, "1.2"), "current", nil, 0)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v012UnavailableDocument(t, version)
			before := v012Snapshot(t, result)
			checks, err := validator.Prepare(result, v012Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v012Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV012HistoryAndOtherChecks(t *testing.T) {
	result := v012Document(t, v012Text(t, "A  \t1-2", "A  \t1-2", 2, 1))
	v012Run(t, result, "first", [][3]int{{3, 6, 7}}, 1)
	for _, tc := range []struct {
		code    diagnostics.Code
		message string
		start   int
		end     int
	}{
		{"V001", "Несколько пробелов подряд.", 2, 4},
		{"V002", "В строке есть табуляция.", 4, 5},
	} {
		matches := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "first" || entry.DiagnosticCode != tc.code {
				continue
			}
			matches++
			want := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
			if entry.Location == nil || *entry.Location != want || entry.Message != tc.message ||
				entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
		if matches != 1 {
			t.Errorf("current %s count = %d, want 1", tc.code, matches)
		}
	}
	v012Run(t, result, "second", [][3]int{{3, 6, 7}}, 1)
	v012Run(t, v012Document(t, v012Text(t, "1–2", "1–2", 2, 1)), "first", nil, 1)
}

// v012Registry создаёт реестр только через штатный каталог регистраций.
func v012Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v012UnavailableDocument представляет отсутствие доступного исходника
// при известной версии DSL; это не пустой исходный файл с выдуманной версией.
func v012UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v012Document дополняет фрагменты обязательными @dsl-version и заголовком
// по DSL 1.30. Текст фрагмента не меняется; родители и элементы заданы тестом.
// Обычный текст под заголовком образует шаг по DSL 2.1.15.
func v012Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v012UnavailableDocument(t, "1.2")
	title, encoding := "Unit 1", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v012Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v012Element(t, "tag", "@dsl-version", "dsl-version", 1), v012Element(t, "version", "1.2", "1.2", 14)),
		v012Line(t, "# Unit 1", "heading", 0, 0,
			v012Element(t, "heading-level", "#", "1", 1), v012Element(t, "title", "Unit 1", "Unit 1", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v012NumberLines(t, result)
	return result
}

// v012NumberLines присваивает физические номера готовым строкам входной модели.
func v012NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v012Line создаёт строку с явно заданной структурой, без распознавания Raw.
func v012Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v012Element сохраняет заданные Raw и Value. Длина вычисляется только для
// входного элемента; ожидаемые диапазоны диагностики заданы в тестах явно.
func v012Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v012Text создаёт готовую строку содержимого с явно заданным Value.
func v012Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v012Line(t, raw, "content", parent, depth, v012Element(t, "content", raw, value, 1))
}

// v012ErrorExpectation задаёт код сопутствующей ошибки и её диапазон.
// location содержит номер строки, начало и исключающий конец в столбцах.
type v012ErrorExpectation struct {
	code     diagnostics.Code
	location [3]int
}

// v012Run запускает штатный конвейер и проверяет V012 текущего запуска.
// want содержит явные тройки: строка, начало, исключающий конец диапазона.
// wantV037 и wantErrors задают точные ожидания сопутствующих ошибок.
// Флаги документа и строк проверяются с учётом этих ожиданий; исходные
// данные, ссылки элементов и прежняя история должны сохраняться.
func v012Run(t *testing.T, result *model.Result, processingID string, want [][3]int, wantV037 int, wantErrors ...v012ErrorExpectation) {
	t.Helper()
	before := v012Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	oldLineErrors := make([]bool, len(result.Lines))
	for i, line := range result.Lines {
		oldLineErrors[i] = line.HasErrors
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v012Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v012Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got, gotErrors []model.Diagnostic
	gotV037 := 0
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.Source != processingID {
			t.Fatalf("new diagnostic has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V037" && entry.Source == processingID {
			gotV037++
			if entry.SeverityLevel != "error" || entry.DiagnosticScope != "document" ||
				entry.Message != "В документе отсутствует @document-id." || entry.Location != nil ||
				entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected accompanying V037: %+v", entry)
			}
		}
		if (entry.DiagnosticCode == "V111" || entry.DiagnosticCode == "V112" || entry.DiagnosticCode == "V114" || entry.DiagnosticCode == "V115" || entry.DiagnosticCode == "V148") && entry.Source == processingID {
			gotErrors = append(gotErrors, entry)
		}
		if entry.DiagnosticCode == "V012" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V012 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V012" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.Message != "Между числами используется дефис или длинное тире.\nконкретизация: Если это числовой диапазон, используйте короткое тире: 1–12" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(gotErrors) != len(wantErrors) {
		t.Fatalf("current V111/V112/V114/V115/V148 count = %d, want %d; diagnostics: %+v", len(gotErrors), len(wantErrors), gotErrors)
	}
	messages := map[diagnostics.Code]string{
		"V111": "Команда пустой строки отличается от отдельной записи <br>.\nконкретизация: другое содержимое строки",
		"V112": "Тег выделения не имеет пары в своей строке.\nконкретизация: Открытие и закрытие выделения должны находиться в одной текстовой строке",
		"V114": "В теге выделения записаны атрибуты.",
		"V115": "Использован неподдерживаемый HTML-тег.",
		"V148": "Подчёркивания не образуют допустимый пропуск.\nконкретизация: Длина последовательности: 1; требуется 5.",
	}
	wantLineErrors := append([]bool{}, oldLineErrors...)
	for i, expected := range wantErrors {
		entry := gotErrors[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		if entry.DiagnosticCode != expected.code || entry.Location == nil || *entry.Location != location || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != messages[expected.code] {
			t.Errorf("unexpected accompanying diagnostic: %+v; want %s at %+v", entry, expected.code, location)
		}
		wantLineErrors[expected.location[0]-1] = true
	}
	if len(result.Lines) != len(wantLineErrors) {
		t.Fatal("line count changed")
	}
	for i, line := range result.Lines {
		if line.HasErrors != wantLineErrors[i] {
			t.Errorf("line %d hasErrors = %v, want %v", line.Number, line.HasErrors, wantLineErrors[i])
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost diagnostic history")
	}
	if result.Document.HasErrors != (oldDocumentError || wantV037 > 0 || len(wantErrors) > 0) {
		t.Errorf("document.hasErrors = %v, want %v", result.Document.HasErrors, oldDocumentError || wantV037 > 0 || len(wantErrors) > 0)
	}
	previous := *result
	previous.Lines = append([]model.Line{}, result.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = oldLineErrors[i]
	}
	previous.Document.HasErrors = oldDocumentError
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v012Snapshot(t, &previous) != before {
		t.Fatal("source, structure, error flags, element links or previous history changed")
	}
}

// v012Snapshot фиксирует всю модель для сравнения исходных данных и истории.
func v012Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

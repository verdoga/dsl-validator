package checkers

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// Ожидаемые тексты заданы независимо от реализации и сохраняют кодовые точки.
const (
	v019WantAY      = "Символы: «a» (U+0061, Latin), «я» (U+044F, Cyrillic)."
	v019WantBB      = "Символы: «b» (U+0062, Latin), «б» (U+0431, Cyrillic)."
	v019WantExample = "Символы: «с» (U+0441, Cyrillic), «л» (U+043B, Cyrillic), «o» (U+006F, Latin), «в» (U+0432, Cyrillic), «о» (U+043E, Cyrillic)."
)

// v019Expected задаёт строку, исходные Unicode-колонки [start,end) и конкретизацию.
type v019Expected struct {
	line, start, end int
	detail           string
}

func TestV019Registration(t *testing.T) {
	definitions, err := v019Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V019" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "В одном слове смешаны кириллические и латинские буквы." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V019 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V019 definitions = %d, want 1", matches)
	}
}

func TestV019Words(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v019Expected
	}{
		{"reference", "Прочитайте слово.", nil},
		// В примере o в слoво — U+006F, последняя о — U+043E.
		{"example", "Прочитайте слoво.", []v019Expected{{3, 12, 17, v019WantExample}}},
		{"Latin only", "word", nil},
		{"Cyrillic only", "слово", nil},
		{"separate scripts", "word слово", nil},
		{"single Latin letter", "a", nil},
		{"single Cyrillic letter", "я", nil},
		{"minimal mixed word", "aя", []v019Expected{{3, 1, 3, v019WantAY}}},
		{"repeated symbols", "aяaя", []v019Expected{{3, 1, 5, v019WantAY}}},
		{"first appearance order", "яaяa", []v019Expected{{3, 1, 5, "Символы: «я» (U+044F, Cyrillic), «a» (U+0061, Latin)."}}},
		{"case is preserved", "AaЯя", []v019Expected{{3, 1, 5, "Символы: «A» (U+0041, Latin), «a» (U+0061, Latin), «Я» (U+042F, Cyrillic), «я» (U+044F, Cyrillic)."}}},
		{"extended Latin", "éж", []v019Expected{{3, 1, 3, "Символы: «é» (U+00E9, Latin), «ж» (U+0436, Cyrillic)."}}},
		{"extended Cyrillic", "aӁ", []v019Expected{{3, 1, 3, "Символы: «a» (U+0061, Latin), «Ӂ» (U+04C1, Cyrillic)."}}},
		{"attached combining mark", "a\u0301я", []v019Expected{{3, 1, 4, v019WantAY}}},
		{"trailing combining mark is in range", "aя\u0301", []v019Expected{{3, 1, 4, v019WantAY}}},
		{"unattached mark is outside word", "\u0301aя", []v019Expected{{3, 2, 4, v019WantAY}}},
		{"Cyrillic mark does not add script", "a\u0483", nil},
		{"marks alone", "\u0301\u0483", nil},
		{"third script in mixed word", "αaя", []v019Expected{{3, 1, 4, v019WantAY}}},
		{"Greek and Latin", "αa", nil},
		{"Greek and Cyrillic", "αя", nil},
		{"ASCII apostrophe", "a'я", []v019Expected{{3, 1, 4, v019WantAY}}},
		{"typographic apostrophe", "a’я", []v019Expected{{3, 1, 4, v019WantAY}}},
		{"modifier apostrophe", "aʼя", []v019Expected{{3, 1, 4, v019WantAY}}},
		{"apostrophe after attached mark", "a\u0301'я", []v019Expected{{3, 1, 5, v019WantAY}}},
		{"apostrophe before unattached mark", "a'\u0301я", nil},
		{"several valid apostrophes", "a'я’aʼя", []v019Expected{{3, 1, 8, v019WantAY}}},
		{"surrounding ASCII apostrophes", "'aя'", []v019Expected{{3, 2, 4, v019WantAY}}},
		{"surrounding typographic apostrophes", "’aя’", []v019Expected{{3, 2, 4, v019WantAY}}},
		{"surrounding modifier apostrophes", "ʼaяʼ", []v019Expected{{3, 2, 4, v019WantAY}}},
		{"double ASCII apostrophe", "a''я", nil},
		{"double typographic apostrophe", "a’’я", nil},
		{"double modifier apostrophe", "aʼʼя", nil},
		{"apostrophes alone", "'’ʼ", nil},
		{"hyphen splits words", "a-я", nil},
		{"dash splits words", "a—я", nil},
		{"space splits words", "a я", nil},
		{"tab splits words", "a\tя", nil},
		{"nonbreaking space splits words", "a\u00A0я", nil},
		{"digit splits words", "a1я", nil},
		{"comma splits words", "a,я", nil},
		{"other punctuation splits words", "a/я;a_я", nil},
		{"multiple words", "aя bб", []v019Expected{{3, 1, 3, v019WantAY}, {3, 4, 6, v019WantBB}}},
		{"clean word between mixed words", "aя word bб", []v019Expected{{3, 1, 3, v019WantAY}, {3, 9, 11, v019WantBB}}},
		{"email is not excluded", "aя@example.org", []v019Expected{{3, 1, 3, v019WantAY}}},
		{"URL is not excluded", "https://aя.org", []v019Expected{{3, 9, 11, v019WantAY}}},
		{"Unicode source columns", "😀aя.", []v019Expected{{3, 2, 4, v019WantAY}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v019Run(t, v019Document(t, v019Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV019RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v019Expected
	}{
		{"empty physical line", []model.Line{v019Line(t, "", "blank", 2, 1), v019Text(t, "Read this.", 2, 1)}, nil},
		{"blank spaces and tab", []model.Line{v019Line(t, " \t ", "blank", 2, 1), v019Text(t, "Read this.", 2, 1)}, nil},
		{"no joining across lines", []model.Line{v019Text(t, "a", 2, 1), v019Text(t, "я", 2, 1)}, nil},
		{"independent lines", []model.Line{v019Text(t, "aя", 2, 1), v019Text(t, "bб", 2, 1)},
			[]v019Expected{{3, 1, 3, v019WantAY}, {4, 1, 3, v019WantBB}}},
		{"element offset with indentation", []model.Line{
			v019Line(t, "  aя  ", "content", 2, 1, v019Element(t, "content", "aя", "aя", 3, 5)),
		}, []v019Expected{{3, 3, 5, v019WantAY}}},
		// DSL 1.21–1.22: Raw сохраняет экранирование и исходные колонки.
		{"escaped prefix", []model.Line{
			v019Line(t, "\\{x\\} aя", "content", 2, 1,
				v019Element(t, "content", "\\{x\\} aя", "{x} aя", 1, 9)),
		}, []v019Expected{{3, 7, 9, v019WantAY}}},
		// DSL 2.8.11: внутри @editor строка с угловыми скобками — литеральный текст.
		{"HTML-like marker is checked as text", []model.Line{
			v019Line(t, "@editor {", "block-start", 2, 1,
				v019Element(t, "tag", "@editor", "editor", 1, 8), v019Element(t, "block-open", "{", "{", 9, 10)),
			v019Text(t, "<tеg>", 3, 2),
			v019Line(t, "}", "block-end", 3, 2, v019Element(t, "block-close", "}", "}", 1, 2)),
			v019Text(t, "Read this.", 2, 1),
		}, []v019Expected{{4, 2, 5, "Символы: «t» (U+0074, Latin), «е» (U+0435, Cyrillic), «g» (U+0067, Latin)."}}},
		// DSL 3.6: разрешённое форматирование не разрывает слово.
		{"HTML inside mixed word", []model.Line{
			v019Line(t, "@text {", "block-start", 2, 1,
				v019Element(t, "tag", "@text", "text", 1, 6), v019Element(t, "block-open", "{", "{", 7, 8)),
			v019Text(t, "сл<em>o</em>во", 3, 2),
			v019Line(t, "}", "block-end", 3, 2, v019Element(t, "block-close", "}", "}", 1, 2)),
		}, []v019Expected{{4, 1, 15, v019WantExample}}},
		{"mixed word inside formatting", []model.Line{
			v019Line(t, "@text {", "block-start", 2, 1,
				v019Element(t, "tag", "@text", "text", 1, 6), v019Element(t, "block-open", "{", "{", 7, 8)),
			v019Text(t, "<em>aя</em>", 3, 2),
			v019Line(t, "}", "block-end", 3, 2, v019Element(t, "block-close", "}", "}", 1, 2)),
		}, []v019Expected{{4, 5, 7, v019WantAY}}},
		// DSL 3.8.6, 3.8.11: собственный текст инструкции и вложенная подсказка.
		{"nested hint", []model.Line{
			v019Line(t, "@instr {", "block-start", 2, 1,
				v019Element(t, "tag", "@instr", "instr", 1, 7), v019Element(t, "block-open", "{", "{", 8, 9)),
			v019Text(t, "Read the sentence.", 3, 2),
			v019Line(t, "@hint {", "block-start", 3, 2,
				v019Element(t, "tag", "@hint", "hint", 1, 6), v019Element(t, "block-open", "{", "{", 7, 8)),
			v019Text(t, "aя", 5, 3),
			v019Line(t, "}", "block-end", 5, 3, v019Element(t, "block-close", "}", "}", 1, 2)),
			v019Line(t, "}", "block-end", 3, 2, v019Element(t, "block-close", "}", "}", 1, 2)),
		}, []v019Expected{{6, 1, 3, v019WantAY}}},
		{"media source is excluded", []model.Line{
			v019Line(t, "@media image \"aя.png\"", "tag", 2, 1,
				v019Element(t, "tag", "@media", "media", 1, 7), v019Element(t, "media-type", "image", "image", 8, 13),
				v019Element(t, "source", "\"aя.png\"", "aя.png", 14, 22)),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v019Run(t, v019Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV019FormattingWords(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []v019Expected
	}{
		{"слoво", []v019Expected{{4, 1, 6, v019WantExample}}},
		{"сл<em>o</em>во", []v019Expected{{4, 1, 15, v019WantExample}}},
		{"сл<em>о</em>во", nil},
		{"😀 <Em>a<strong>я</strong></eM>", []v019Expected{{4, 7, 17, v019WantAY}}},
		{"a'<em>я</em>", []v019Expected{{4, 1, 8, v019WantAY}}},
		{"a<em>\u0301я</em>", []v019Expected{{4, 1, 8, v019WantAY}}},
		{`\\<em>a<u>я</u></em>`, []v019Expected{{4, 7, 12, v019WantAY}}},
		{"word<em> </em>слово", nil},
		{"a<em>1</em>я", nil},
		{"a<em>,</em>я", nil},
		{`a\<em>я</em>`, nil},
		{`a<em\>я</em>`, nil},
		{"a<foo>я</foo>", nil},
		{"a<em x=y>я</em>", nil},
		{"a<br>я", nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v019Run(t, v020TextBlock(t, tc.raw), "current", tc.want)
		})
	}
	v019Run(t, v021Block(t, "editor", v021Text(t, "сл<em>o</em>во", 3, 2)), "current", nil)
	result := v020TextBlock(t, "a<em>я</em>")
	result.Lines[3].Elements = []model.Element{
		v019Element(t, "content", "a", "a", 1, 2),
		v019Element(t, "content", "<em>я</em>", "<em>я</em>", 2, 12),
	}
	v019Run(t, result, "current", nil)
}

func TestV019PlaceholderAnswers(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v019Expected
	}{
		{"mixed answer", v019Line(t, "_____{слoво}", "content", 3, 2,
			v019Element(t, "placeholder", "_____{слoво}", "слoво", 1, 13)),
			[]v019Expected{{4, 7, 12, v019WantExample}}},
		{"two answers", v019Line(t, "_____{aя} _____{bб}", "content", 3, 2,
			v019Element(t, "placeholder", "_____{aя}", "aя", 1, 10),
			v019Element(t, "placeholder", "_____{bб}", "bб", 11, 20)),
			[]v019Expected{{4, 7, 9, v019WantAY}, {4, 17, 19, v019WantBB}}},
		{"no joining with adjacent answer", v019Line(t, "a_____{я}", "content", 3, 2,
			v019Element(t, "content", "a", "a", 1, 2), v019Element(t, "placeholder", "_____{я}", "я", 2, 10)), nil},
		{"no joining with following content", v019Line(t, "_____{a}я", "content", 3, 2,
			v019Element(t, "placeholder", "_____{a}", "a", 1, 9), v019Element(t, "content", "я", "я", 9, 10)), nil},
		{"without answer", v019Line(t, "_____", "content", 3, 2, model.Element{
			ElementType: "placeholder", Raw: "_____", Value: nil, Start: 1, End: 6, ErrorIDs: []string{},
		}), nil},
		{"escaped answer preserves source columns", v019Line(t, "_____{\\{x\\} aя}", "content", 3, 2,
			v019Element(t, "placeholder", "_____{\\{x\\} aя}", "{x} aя", 1, 16)),
			[]v019Expected{{4, 13, 15, v019WantAY}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 4.3.7–4.3.10: служебные ответы допустимы только в @multifill.
			result := v019Document(t,
				v019Line(t, "@multifill {", "block-start", 2, 1,
					v019Element(t, "tag", "@multifill", "multifill", 1, 11), v019Element(t, "block-open", "{", "{", 12, 13)),
				tc.line,
				v019Line(t, "}", "block-end", 3, 2, v019Element(t, "block-close", "}", "}", 1, 2)))
			v019Run(t, result, "current", tc.want)
		})
	}
}

func TestV019MetadataElements(t *testing.T) {
	for _, tc := range []struct {
		name, tag string
		kind      model.ElementType
		start     int
		want      []v019Expected
	}{
		{"name is checked", "section", "name", 10, []v019Expected{{2, 10, 12, v019WantAY}}},
		{"identifier is excluded", "document-id", "identifier", 14, nil},
		{"resource path is excluded", "resource-dir", "resource-path", 15, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.31.2: метаданные располагаются между версией и заголовком.
			result := v019Document(t, v019Text(t, "Read this.", 2, 1))
			meta := v019Line(t, "@"+tc.tag+" aя", "tag", 0, 0,
				v019Element(t, "tag", "@"+tc.tag, tc.tag, 1, tc.start-1),
				v019Element(t, tc.kind, "aя", "aя", tc.start, tc.start+2))
			result.Lines = []model.Line{result.Lines[0], meta, result.Lines[1], result.Lines[2]}
			for i := range result.Lines {
				result.Lines[i].Number = i + 1
			}
			parent, count, value := 3, 4, "aя"
			result.Lines[3].ParentLine = &parent
			result.Document.LineCount = &count
			switch tc.tag {
			case "section":
				result.Document.Metadata.Section = &value
			case "document-id":
				result.Document.Metadata.DocumentID = &value
			case "resource-dir":
				result.Document.Metadata.ResourceDirs = []string{value}
			}
			v019Run(t, result, "current", tc.want)
		})
	}
}

func TestV019Heading(t *testing.T) {
	result := v019Document(t, v019Text(t, "Read this.", 2, 1))
	result.Lines[1].Raw = "# aя"
	result.Lines[1].Elements[1] = v019Element(t, "title", "aя", "aя", 3, 5)
	title := "aя"
	result.Document.Metadata.Title = &title
	v019Run(t, result, "current", []v019Expected{{2, 3, 5, v019WantAY}})
}

func TestV019UnparsedIsExcluded(t *testing.T) {
	// DSL 1.16: неизвестный тег — ошибка. Код сохранённой ошибки парсера
	// непрозрачен для валидатора и здесь обозначает тестовую историю.
	for _, tc := range []struct {
		name, following string
		want            []v019Expected
	}{
		{"unparsed produces no V019", "Read this.", nil},
		{"following content is still checked", "bб", []v019Expected{{4, 1, 3, v019WantBB}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v019Line(t, "@unknown aя", "invalid", 2, 1, model.Element{
				ElementType: "unparsed", Raw: "@unknown aя", Value: nil, Start: 1, End: 12, ErrorIDs: []string{"parser-error"},
			})
			line.HasErrors = true
			result := v019Document(t, line, v019Text(t, tc.following, 2, 1))
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{
				ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
				Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 12}},
				RelatedLocations: []model.Location{},
			}}
			v019Run(t, result, "current", tc.want)
		})
	}
}

func TestV019LineEndingsAndBOM(t *testing.T) {
	for _, tc := range []struct {
		name   string
		eols   [3]model.LineEnding
		hasBOM bool
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}, false},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}, false},
		{"LF without final EOL", [3]model.LineEnding{"\n", "\n", ""}, false},
		{"CRLF without final EOL", [3]model.LineEnding{"\r\n", "\r\n", ""}, false},
		{"mixed endings", [3]model.LineEnding{"\r\n", "\n", "\r\n"}, false},
		{"BOM", [3]model.LineEnding{"\n", "\n", "\n"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.13: начальный BOM и окончания строк не входят в Raw.
			result := v019Document(t, v019Text(t, "Прочитайте слoво.", 2, 1))
			result.Document.HasBOM = &tc.hasBOM
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v019Run(t, result, "current", []v019Expected{{3, 12, 17, v019WantExample}})
		})
	}
}

func TestV019UnavailableLines(t *testing.T) {
	v019Run(t, v019UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV019UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v019UnavailableDocument(t, version)
			before := v019Snapshot(t, result)
			checks, err := validator.Prepare(result, v019Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v019Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV019CoexistsWithOtherCheckers(t *testing.T) {
	result := v019Document(t, v019Text(t, "aя  \tX\u00A0Y ab—cd", 2, 1))
	v019Run(t, result, "current", []v019Expected{{3, 1, 3, v019WantAY}})
	for _, want := range []struct {
		code       diagnostics.Code
		severity   diagnostics.Severity
		start, end int
		message    string
	}{
		{"V001", "warning", 3, 5, "Несколько пробелов подряд."},
		{"V002", "warning", 5, 6, "В строке есть табуляция."},
		{"V010", "warning", 7, 8, "Использован пробельный символ, отличный от обычного пробела.\nконкретизация: Обнаружены: U+00A0. Допустим U+0020."},
		{"V014", "recommendation", 12, 13, "Тире находится между буквенными частями без пробелов.\nконкретизация: Если это составное слово, используйте дефис; если граница частей фразы — проверьте пробелы вокруг тире"},
	} {
		matches := 0
		for _, entry := range result.Diagnostics {
			if entry.DiagnosticCode != want.code || entry.Source != "current" {
				continue
			}
			matches++
			location := model.Location{Start: model.Position{Line: 3, Column: want.start}, End: model.Position{Line: 3, Column: want.end}}
			if entry.Location == nil || *entry.Location != location || entry.Message != want.message ||
				entry.SeverityLevel != want.severity || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", want.code, entry)
			}
		}
		if matches != 1 {
			t.Errorf("current %s count = %d, want 1", want.code, matches)
		}
	}
}

func TestV019RepeatedRunsPreserveHistory(t *testing.T) {
	result := v019Document(t, v019Text(t, "Прочитайте слoво.", 2, 1))
	want := []v019Expected{{3, 12, 17, v019WantExample}}
	v019Run(t, result, "first", want)
	v019Run(t, result, "second", want)
	v019Run(t, v019Document(t, v019Text(t, "Прочитайте слово.", 2, 1)), "first", nil)
}

// v019Registry создаёт реестр через штатный каталог без подмены регистраций.
func v019Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v019UnavailableDocument сохраняет известную версию при недоступных строках.
func v019UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v019Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v019Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v019Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v019Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v019Element(t, "version", "1.2", "1.2", 14, 17)),
		v019Line(t, "# Unit 1", "heading", 0, 0,
			v019Element(t, "heading-level", "#", "1", 1, 2),
			v019Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v019Line собирает строку с явно заданной структурой, без классификации Raw.
func v019Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v019Element сохраняет исходное и семантическое значения и явный диапазон.
func v019Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v019Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v019Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v019Line(t, text, "content", parent, depth,
		v019Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v019Run проверяет текущие V019 через штатный конвейер и независимые ожидания.
// Проверяет исходные данные, ссылки и историю; флаги учитывают весь каталог.
func v019Run(t *testing.T, result *model.Result, processingID string, want []v019Expected) {
	t.Helper()
	before := v019Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v019Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v019Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V019" && entry.Source != processingID {
			t.Fatalf("new V019 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V019" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V019 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "В одном слове смешаны кириллические и латинские буквы.\nконкретизация: " + expected.detail
		if entry.Message != message {
			t.Errorf("occurrence %d message = %q, want %q", i, entry.Message, message)
		}
		if entry.DiagnosticCode != "V019" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
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
	if err := json.Unmarshal([]byte(v019Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	previous.Document.HasErrors = original.Document.HasErrors
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
	}
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	// В снимке исключаются новые ссылки других элементных диагностик каталога.
	newElementIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[len(previous.Diagnostics):] {
		if entry.DiagnosticScope == "element" && entry.DiagnosticCode != "V019" {
			newElementIDs[entry.ID] = true
		}
	}
	previous.Lines = append([]model.Line{}, previous.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].Elements = append([]model.Element{}, previous.Lines[i].Elements...)
		for j := range previous.Lines[i].Elements {
			element := &previous.Lines[i].Elements[j]
			element.ErrorIDs = slices.DeleteFunc(slices.Clone(element.ErrorIDs), func(id string) bool { return newElementIDs[id] })
		}
	}
	if v019Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v019Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v019Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

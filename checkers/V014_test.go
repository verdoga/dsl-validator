package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV014Registration(t *testing.T) {
	definitions, err := v014Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V014" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Тире находится между буквенными частями без пробелов." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V014 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V014 definitions = %d, want 1", matches)
	}
}

func TestV014LetterPatterns(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"reference", "Северо-западный ветер.", nil},
		{"example", "Северо–западный ветер.", [][3]int{{3, 7, 8}}},
		{"em dash", "Северо—западный ветер.", [][3]int{{3, 7, 8}}},
		{"uppercase single letters", "A–F", nil},
		{"mixed case single letters", "a–F", nil},
		{"lowercase single letters", "a–f", nil},
		{"Cyrillic single letters", "А—я", nil},
		{"IPA single letters", "ɛ–ʃ", nil},
		{"single letter chain", "A–B—C", nil},
		{"single letters bounded by punctuation", "(A–F).", nil},
		{"long right part", "A–BC", [][3]int{{3, 2, 3}}},
		{"long left part", "AB—c", [][3]int{{3, 3, 4}}},
		{"both parts long", "ab—CD", [][3]int{{3, 3, 4}}},
		{"multiple dashes", "аб–вг—де", [][3]int{{3, 3, 4}, {3, 6, 7}}},
		{"one recommendation and one exception", "ab–c—d", [][3]int{{3, 3, 4}}},
		{"single letters with combining marks", "a\u0301—b\u0308", nil},
		{"several marks do not add letters", "a\u0301\u0308–b", nil},
		{"long left part with combining mark", "a\u0301b—c", [][3]int{{3, 4, 5}}},
		{"long right part with combining mark", "a—b\u0301c", [][3]int{{3, 2, 3}}},
		{"unattached right mark", "а—\u0301б", nil},
		{"only marks on left", "\u0301—аб", nil},
		{"enclosing mark attached to single letter", "a\u20DD—b", nil},
		{"spacing combining mark attached to single letter", "का—b", nil},
		{"IPA parts", "ɛə—ʃʊ", [][3]int{{3, 3, 4}}},
		{"slashes do not exclude IPA", "/ɛə—ʃʊ/", [][3]int{{3, 4, 5}}},
		{"IPA single letters inside slashes", "/ɛ—ʃ/", nil},
		{"Unicode source columns", "😀аб—вг", [][3]int{{3, 4, 5}}},
		{"spaces on both sides", "а — б", nil},
		{"space only on right", "аб— вг", [][3]int{{3, 3, 4}}},
		{"space only on left", "аб —вг", [][3]int{{3, 4, 5}}},
		{"single letter beside space", "а– б", [][3]int{{3, 2, 3}}},
		{"space beside single letter", "а –б", [][3]int{{3, 3, 4}}},
		{"combining mark before dash and space", "a\u0301— b", [][3]int{{3, 3, 4}}},
		{"several spaces on right", "a—  b", [][3]int{{3, 2, 3}}},
		{"several spaces on left", "a  —b", [][3]int{{3, 4, 5}}},
		{"digits", "1—2", nil},
		{"digit next to letter part", "ab—2", nil},
		{"punctuation next to letter part", "ab—,cd", nil},
		{"two consecutive dashes", "а——б", nil},
		{"first character dash", "—аб", nil},
		{"last character dash", "аб—", nil},
		{"only dash", "—", nil},
		{"only letter", "а", nil},
		{"non-target hyphen", "ab-cd", nil},
		{"non-target minus", "ab−cd", nil},
		{"literal HTML entity", "ab&#x2014;cd", nil},
		{"literal Unicode escape", `ab\u2014cd`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v014Run(t, v014Document(t, v014Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV014Whitespace(t *testing.T) {
	// LF и CR запрещены в Raw контрактом; все остальные Unicode-пробелы
	// заданы явно. Данные не получаются из проверяемого предиката.
	for _, tc := range []struct {
		name, space string
	}{
		{"tab", "\t"}, {"vertical tab", "\v"}, {"form feed", "\f"}, {"space", " "},
		{"U+0085", "\u0085"}, {"U+00A0", "\u00A0"}, {"U+1680", "\u1680"},
		{"U+2000", "\u2000"}, {"U+2001", "\u2001"}, {"U+2002", "\u2002"},
		{"U+2003", "\u2003"}, {"U+2004", "\u2004"}, {"U+2005", "\u2005"},
		{"U+2006", "\u2006"}, {"U+2007", "\u2007"}, {"U+2008", "\u2008"},
		{"U+2009", "\u2009"}, {"U+200A", "\u200A"}, {"U+2028", "\u2028"},
		{"U+2029", "\u2029"}, {"U+202F", "\u202F"}, {"U+205F", "\u205F"}, {"U+3000", "\u3000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v014Run(t, v014Document(t, v014Text(t, "a—"+tc.space+"b", 2, 1)), "right", [][3]int{{3, 2, 3}})
			v014Run(t, v014Document(t, v014Text(t, "a"+tc.space+"–b", 2, 1)), "left", [][3]int{{3, 3, 4}})
			v014Run(t, v014Document(t, v014Text(t, "a"+tc.space+"—"+tc.space+"b", 2, 1)), "both", nil)
		})
	}
	for _, raw := range []string{"ab—\u200Bcd", "ab\u200B—cd", "ab—\uFEFFcd"} {
		t.Run(raw, func(t *testing.T) {
			v014Run(t, v014Document(t, v014Text(t, raw, 2, 1)), "current", nil)
		})
	}
}

func TestV014RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"trailing space remains visible", []model.Line{
			v014Line(t, "а– ", "content", 2, 1, v014Element(t, "content", "а–", "а–", 1, 3)),
		}, [][3]int{{3, 2, 3}}},
		{"leading space remains visible", []model.Line{
			v014Line(t, " –а", "content", 2, 1, v014Element(t, "content", "–а", "–а", 2, 4)),
		}, [][3]int{{3, 2, 3}}},
		{"no joining across lines", []model.Line{v014Text(t, "аб—", 2, 1), v014Text(t, "вг", 2, 1)}, nil},
		{"empty physical line", []model.Line{v014Line(t, "", "blank", 2, 1), v014Text(t, "Read this.", 2, 1)}, nil},
		{"blank spaces and tab", []model.Line{v014Line(t, " \t ", "blank", 2, 1), v014Text(t, "Read this.", 2, 1)}, nil},
		{"single-line comment", []model.Line{
			v014Line(t, "@editor аб—вг", "tag", 2, 1,
				v014Element(t, "tag", "@editor", "editor", 1, 8), v014Element(t, "content", "аб—вг", "аб—вг", 9, 14)),
			v014Text(t, "Read this.", 2, 1),
		}, [][3]int{{3, 11, 12}}},
		{"block comment", []model.Line{
			v014Line(t, "@editor {", "block-start", 2, 1,
				v014Element(t, "tag", "@editor", "editor", 1, 8), v014Element(t, "block-open", "{", "{", 9, 10)),
			v014Text(t, "аб—вг", 3, 2),
			v014Line(t, "}", "block-end", 3, 2, v014Element(t, "block-close", "}", "}", 1, 2)),
			v014Text(t, "Read this.", 2, 1),
		}, [][3]int{{4, 3, 4}}},
		// DSL 3.8.6, 3.8.11: инструкция содержит свой текст и один вложенный @hint.
		{"nested hint", []model.Line{
			v014Line(t, "@instr {", "block-start", 2, 1,
				v014Element(t, "tag", "@instr", "instr", 1, 7), v014Element(t, "block-open", "{", "{", 8, 9)),
			v014Text(t, "Read the sentence.", 3, 2),
			v014Line(t, "@hint {", "block-start", 3, 2,
				v014Element(t, "tag", "@hint", "hint", 1, 6), v014Element(t, "block-open", "{", "{", 7, 8)),
			v014Text(t, "аб—вг", 5, 3),
			v014Line(t, "}", "block-end", 5, 3, v014Element(t, "block-close", "}", "}", 1, 2)),
			v014Line(t, "}", "block-end", 3, 2, v014Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{6, 3, 4}}},
		// DSL 3.2.8: разделитель находится между непустыми частями блока.
		{"separator and block boundaries", []model.Line{
			v014Line(t, "@example {", "block-start", 2, 1,
				v014Element(t, "tag", "@example", "example", 1, 9), v014Element(t, "block-open", "{", "{", 10, 11)),
			v014Text(t, "аб—вг", 3, 2),
			v014Line(t, "---", "separator", 3, 2),
			v014Text(t, "A–F", 3, 2),
			v014Line(t, "}", "block-end", 3, 2, v014Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 3, 4}}},
		// DSL 3.1.6: кавычки входят в Raw, но не в семантический путь SOURCE.
		{"quoted media path", []model.Line{
			v014Line(t, "@media image \"ab–cd.png\"", "tag", 2, 1,
				v014Element(t, "tag", "@media", "media", 1, 7), v014Element(t, "media-type", "image", "image", 8, 13),
				v014Element(t, "source", "\"ab–cd.png\"", "ab–cd.png", 14, 25)),
		}, [][3]int{{3, 17, 18}}},
		// DSL 1.21–1.22: экранирование сохраняется в Raw и влияет на исходные колонки.
		{"escaped prefix preserves columns", []model.Line{
			v014Line(t, "\\{x\\} аб—вг", "content", 2, 1,
				v014Element(t, "content", "\\{x\\} аб—вг", "{x} аб—вг", 1, 12)),
		}, [][3]int{{3, 9, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v014Run(t, v014Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV014HTMLIsNotInterpreted(t *testing.T) {
	// DSL 3.6: содержимое @text допускает корректно вложенные HTML-теги.
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"markers interrupt adjacency", "северо<strong>–</strong>западный", nil},
		{"text between markers is checked", "<em>аб—вг</em>", [][3]int{{4, 7, 8}}},
		{"uppercase markers", "<EM>аб—вг</EM>", [][3]int{{4, 7, 8}}},
		{"literal angle brackets are not excluded", `\<ab—cd\>`, [][3]int{{4, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.raw
			if tc.name == "literal angle brackets are not excluded" {
				value = "<ab—cd>"
			}
			result := v014Document(t,
				v014Line(t, "@text {", "block-start", 2, 1,
					v014Element(t, "tag", "@text", "text", 1, 6), v014Element(t, "block-open", "{", "{", 7, 8)),
				v014Line(t, tc.raw, "content", 3, 2, v014Element(t, "content", tc.raw, value, 1, utf8.RuneCountInString(tc.raw)+1)),
				v014Line(t, "}", "block-end", 3, 2, v014Element(t, "block-close", "}", "}", 1, 2)))
			v014Run(t, result, "current", tc.want)
		})
	}
}

func TestV014Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		end       int
		want      [][3]int
	}{
		{"unknown tag", "@unknown ab–cd", 15, [][3]int{{3, 12, 13}}},
		{"single letters in unparsed", "@unknown A–F", 13, nil},
		{"HTML-like marker in unparsed", "@unknown <x attr=\"ab—cd\">", 26, [][3]int{{3, 21, 22}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.16: неизвестный тег является ошибкой. unparsed сохраняет
			// исходный фрагмент, null Value и ссылку на существующую ошибку.
			// Код ошибки непрозрачен для валидатора и обозначает тестовую историю.
			line := v014Line(t, tc.raw, "invalid", 2, 1, model.Element{
				ElementType: "unparsed", Raw: tc.raw, Value: nil, Start: 1, End: tc.end, ErrorIDs: []string{"parser-error"},
			})
			line.HasErrors = true
			result := v014Document(t, line, v014Text(t, "Read this.", 2, 1))
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{
				ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
				Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: tc.end}},
				RelatedLocations: []model.Location{},
			}}
			v014Run(t, result, "current", tc.want)
		})
	}
}

func TestV014Heading(t *testing.T) {
	result := v014Document(t, v014Text(t, "Read this.", 2, 1))
	result.Lines[1].Raw = "# аб—вг"
	result.Lines[1].Elements[1] = v014Element(t, "title", "аб—вг", "аб—вг", 3, 8)
	title := "аб—вг"
	result.Document.Metadata.Title = &title
	v014Run(t, result, "current", [][3]int{{2, 5, 6}})
}

func TestV014LineEndingsAndBOM(t *testing.T) {
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
			// DSL 1.13: BOM удалён до выделения строк, окончания хранятся отдельно.
			result := v014Document(t, v014Text(t, "Северо–западный ветер.", 2, 1))
			result.Document.HasBOM = &tc.hasBOM
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v014Run(t, result, "current", [][3]int{{3, 7, 8}})
		})
	}
}

func TestV014UnavailableLines(t *testing.T) {
	// Отсутствующие строки при известной версии допустимы по контракту модели.
	v014Run(t, v014UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV014UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v014UnavailableDocument(t, version)
			before := v014Snapshot(t, result)
			checks, err := validator.Prepare(result, v014Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v014Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV014CoexistsWithOtherCheckers(t *testing.T) {
	result := v014Document(t, v014Text(t, "ab—cd  \tX\u00A0Y", 2, 1))
	v014Run(t, result, "current", [][3]int{{3, 3, 4}})
	for _, want := range []struct {
		code       diagnostics.Code
		start, end int
		message    string
	}{
		{"V001", 6, 8, "Несколько пробелов подряд."},
		{"V002", 8, 9, "В строке есть табуляция."},
		{"V010", 10, 11, "Использован пробельный символ, отличный от обычного пробела.\nконкретизация: Обнаружены: U+00A0. Допустим U+0020."},
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

func TestV014RepeatedRunsPreserveHistory(t *testing.T) {
	result := v014Document(t, v014Text(t, "Северо–западный ветер.", 2, 1))
	v014Run(t, result, "first", [][3]int{{3, 7, 8}})
	v014Run(t, result, "second", [][3]int{{3, 7, 8}})
	v014Run(t, v014Document(t, v014Text(t, "Северо-западный ветер.", 2, 1)), "first", nil)
}

// v014Registry создаёт реестр через штатный каталог без подмены регистраций.
func v014Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v014UnavailableDocument сохраняет известную версию при недоступных строках.
func v014UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v014Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v014Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v014Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v014Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v014Element(t, "version", "1.2", "1.2", 14, 17)),
		v014Line(t, "# Unit 1", "heading", 0, 0,
			v014Element(t, "heading-level", "#", "1", 1, 2),
			v014Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v014Line собирает строку с явно заданной структурой, без классификации Raw.
func v014Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v014Element сохраняет исходное и семантическое значения и явный диапазон.
func v014Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v014Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v014Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v014Line(t, text, "content", parent, depth,
		v014Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v014Run проверяет текущие V014 через штатный конвейер и явные тройки:
// строка, начало, исключающий конец. Проверяет также сохранность исходной модели.
func v014Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v014Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v014Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v014Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V014" && entry.Source != processingID {
			t.Fatalf("new V014 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V014" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V014 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "Тире находится между буквенными частями без пробелов.\nконкретизация: Если это составное слово, используйте дефис; если граница частей фразы — проверьте пробелы вокруг тире"
		if entry.Message != message {
			t.Errorf("occurrence %d message = %q, want %q", i, entry.Message, message)
		}
		if entry.DiagnosticCode != "V014" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
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
	if v014Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v014Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v014Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

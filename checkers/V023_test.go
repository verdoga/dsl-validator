package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v023Expected задаёт исходный диапазон и конкретизацию независимо от проверки.
type v023Expected struct {
	line, start, end int
	detail           string
}

func TestV023Registration(t *testing.T) {
	definitions, err := v023Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V023" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "В числовой последовательности находится буква, похожая на цифру." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V023 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V023 definitions = %d, want 1", matches)
	}
}

func TestV023Text(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v023Expected
	}{
		{"reference", "Стоимость 105 рублей.", nil},
		{"example", "Стоимость 1O5 рублей.", []v023Expected{{3, 11, 14, "O → 0"}}},
		{"one digit with zero letter now reports", "1O", []v023Expected{{3, 1, 3, "O → 0"}}},
		{"equal counts with zero letters now report", "1OO5", []v023Expected{{3, 1, 5, "O → 0"}}},
		{"more zero letters than digits now report", "1OO", []v023Expected{{3, 1, 4, "O → 0"}}},
		{"only digits", "105", nil},
		{"pure decimal", "105.2", nil},
		{"one digit without zero letter", "1S", nil},
		{"equal counts without zero letter", "1SS5", nil},
		{"more letters without zero letter", "12SSS", nil},
		{"minimum accepted counts without zero letter", "1S5", []v023Expected{{3, 1, 4, "S → 5"}}},
		{"digits exceed multiple letters", "123SS", []v023Expected{{3, 1, 6, "S → 5"}}},
		{"no real digits without zero letter", "IS", nil},
		{"multiple different replacements", "1O2l3", []v023Expected{{3, 1, 6, "O → 0; l → 1"}}},
		{"first occurrence order", "1l2O3", []v023Expected{{3, 1, 6, "l → 1; O → 0"}}},
		{"repeated zero letter listed once", "1O2O3", []v023Expected{{3, 1, 6, "O → 0"}}},
		{"repeated other letter listed once", "1S2S3", []v023Expected{{3, 1, 6, "S → 5"}}},
		{"zero letter still requires a real digit", "Ol", nil},
		{"ordinary So", "So", nil},
		{"ordinary lo", "lo", nil},
		{"digit in previous token is insufficient", "123 So", nil},
		{"digit in next token is insufficient", "lo 123", nil},
		{"words beside a mixed number", "So 1O lo", []v023Expected{{3, 4, 6, "O → 0"}}},
		{"unsupported letter", "12G", nil},
		{"unsupported letter still rejects zero candidate", "1O5G", nil},
		{"lowercase i is not mapped", "12i", nil},
		{"uppercase L is not mapped", "12L", nil},
		{"lowercase b is not mapped", "12b", nil},
		{"uppercase Cyrillic be is not mapped", "12Б", nil},
		{"word containing candidate", "abc1O5def", nil},
		{"positive sign", "+1O5", []v023Expected{{3, 1, 5, "O → 0"}}},
		{"negative sign", "-1O5", []v023Expected{{3, 1, 5, "O → 0"}}},
		{"double initial sign", "++1O5", nil},
		{"internal sign", "1O5-2", nil},
		{"trailing sign", "1O5+", nil},
		{"dot decimal", "1O5.2", []v023Expected{{3, 1, 6, "O → 0"}}},
		{"comma decimal", "1O5,2", []v023Expected{{3, 1, 6, "O → 0"}}},
		{"signed decimal", "-1O5,2", []v023Expected{{3, 1, 7, "O → 0"}}},
		{"letter in fraction", "12.S3", []v023Expected{{3, 1, 6, "S → 5"}}},
		{"multiple dots", "1O5.2.3", nil},
		{"mixed separators", "1O5,2.3", nil},
		{"missing integer", ".1O5", nil},
		{"missing integer after sign", "-.1O5", nil},
		{"sentence period", "1O5.", []v023Expected{{3, 1, 4, "O → 0"}}},
		{"sentence comma", "1O5,", []v023Expected{{3, 1, 4, "O → 0"}}},
		{"sentence ellipsis", "1O5...", []v023Expected{{3, 1, 4, "O → 0"}}},
		{"decimal followed by sentence period", "1O5.2.", []v023Expected{{3, 1, 6, "O → 0"}}},
		{"only period", ".", nil},
		{"only commas", ",,", nil},
		{"only signs", "+ -", nil},
		{"parentheses delimit token", "(1O5)", []v023Expected{{3, 2, 5, "O → 0"}}},
		{"quotes delimit token", `"1O5"`, []v023Expected{{3, 2, 5, "O → 0"}}},
		{"two tokens", "1O5 2S6", []v023Expected{{3, 1, 4, "O → 0"}, {3, 5, 8, "S → 5"}}},
		{"adjacent token is not appended", "1O 5", []v023Expected{{3, 1, 3, "O → 0"}}},
		{"short nonzero candidate is not joined", "1S 5", nil},
		{"tab separates tokens", "1S\t5", nil},
		{"nonbreaking space separates tokens", "1S\u00a05", nil},
		{"Unicode decimal digits", "١O٥", []v023Expected{{3, 1, 4, "O → 0"}}},
		{"Unicode columns", "😀 1O5", []v023Expected{{3, 3, 6, "O → 0"}}},
		{"leading zeroes allowed", "001O5", []v023Expected{{3, 1, 6, "O → 0"}}},
		{"slashes are ordinary boundaries", "/1O5/", []v023Expected{{3, 2, 5, "O → 0"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v023Run(t, v023Document(t, v023Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV023ZeroLetterVariants(t *testing.T) {
	for _, variant := range []struct{ name, letter, detail string }{
		{"Latin uppercase", "O", "O → 0"},
		{"Latin lowercase", "o", "o → 0"},
		{"Cyrillic uppercase", "О", "О → 0"},
		{"Cyrillic lowercase", "о", "о → 0"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			for _, tc := range []struct {
				name, raw string
				end       int
			}{
				{"no real digits", variant.letter, 0},
				{"one digit before", "1" + variant.letter, 3},
				{"one digit after", variant.letter + "1", 3},
				{"equal counts", "1" + variant.letter + variant.letter + "5", 5},
				{"letters only", variant.letter + variant.letter, 0},
				{"decimal with no real digits", variant.letter + "." + variant.letter, 0},
				{"comma decimal with no real digits", variant.letter + "," + variant.letter, 0},
				{"negative with no real digits", "-" + variant.letter, 0},
				{"positive with no real digits", "+" + variant.letter, 0},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var want []v023Expected
					if tc.end != 0 {
						want = []v023Expected{{3, 1, tc.end, variant.detail}}
					}
					v023Run(t, v023Document(t, v023Text(t, tc.raw, 2, 1)), "current", want)
				})
			}
		})
	}
}

func TestV023ExactMappings(t *testing.T) {
	for _, tc := range []struct{ letter, detail string }{
		{"O", "O → 0"}, {"o", "o → 0"}, {"О", "О → 0"}, {"о", "о → 0"},
		{"I", "I → 1"}, {"l", "l → 1"}, {"І", "І → 1"}, {"і", "і → 1"},
		{"Z", "Z → 2"}, {"z", "z → 2"}, {"З", "З → 3"}, {"з", "з → 3"},
		{"S", "S → 5"}, {"s", "s → 5"}, {"б", "б → 6"}, {"B", "B → 8"}, {"В", "В → 8"}, {"g", "g → 9"},
	} {
		t.Run(tc.letter, func(t *testing.T) {
			v023Run(t, v023Document(t, v023Text(t, "12"+tc.letter+"3", 2, 1)), "current",
				[]v023Expected{{3, 1, 5, tc.detail}})
		})
	}
}

func TestV023RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v023Expected
	}{
		{"empty physical line", []model.Line{v023Line(t, "", "blank", 2, 1)}, nil},
		{"blank whitespace", []model.Line{v023Line(t, " \t ", "blank", 2, 1)}, nil},
		{"original columns", []model.Line{v023Line(t, "  1O5 ", "content", 2, 1,
			v023Element(t, "content", "1O5", "1O5", 3, 6))}, []v023Expected{{3, 3, 6, "O → 0"}}},
		// DSL 1.21–1.22: только семантическое значение снимает экранирование скобок.
		{"raw differs from semantic value", []model.Line{v023Line(t, `\{1O5\}`, "content", 2, 1,
			v023Element(t, "content", `\{1O5\}`, "{1O5}", 1, 8))}, []v023Expected{{3, 3, 6, "O → 0"}}},
		{"line boundary with zero letter", []model.Line{v023Text(t, "1O", 2, 1), v023Text(t, "5", 2, 1)},
			[]v023Expected{{3, 1, 3, "O → 0"}}},
		{"line boundary without zero letter", []model.Line{v023Text(t, "1S", 2, 1), v023Text(t, "5", 2, 1)}, nil},
		{"single-line comment", []model.Line{v023Line(t, "@editor 1O5", "tag", 2, 1,
			v023Element(t, "tag", "@editor", "editor", 1, 8), v023Element(t, "content", "1O5", "1O5", 9, 12))},
			[]v023Expected{{3, 9, 12, "O → 0"}}},
		{"block comment", []model.Line{
			v023Line(t, "@editor {", "block-start", 2, 1,
				v023Element(t, "tag", "@editor", "editor", 1, 8), v023Element(t, "block-open", "{", "{", 9, 10)),
			v023Text(t, "1O5", 3, 2),
			v023Line(t, "}", "block-end", 3, 2, v023Element(t, "block-close", "}", "}", 1, 2)),
		}, []v023Expected{{4, 1, 4, "O → 0"}}},
		{"HTML is not parsed", []model.Line{
			v023Line(t, "@text {", "block-start", 2, 1,
				v023Element(t, "tag", "@text", "text", 1, 6), v023Element(t, "block-open", "{", "{", 7, 8)),
			v023Text(t, "<em>1O5</em>", 3, 2),
			v023Line(t, "}", "block-end", 3, 2, v023Element(t, "block-close", "}", "}", 1, 2)),
		}, []v023Expected{{4, 5, 8, "O → 0"}}},
		// DSL 3.4: одна строка двухколоночной таблицы без заголовочной части.
		{"table is not parsed", []model.Line{
			v023Line(t, "@table {", "block-start", 2, 1,
				v023Element(t, "tag", "@table", "table", 1, 7), v023Element(t, "block-open", "{", "{", 8, 9)),
			v023Line(t, "1O5|105", "content", 3, 2,
				v023Element(t, "content", "1O5", "1O5", 1, 4), v023Element(t, "content", "105", "105", 5, 8)),
			v023Line(t, "}", "block-end", 3, 2, v023Element(t, "block-close", "}", "}", 1, 2)),
		}, []v023Expected{{4, 1, 4, "O → 0"}}},
		// DSL 4.3.7–4.3.10: ответ плейсхолдера разрешён внутри блочного multifill.
		{"placeholder answer", []model.Line{
			v023Line(t, "@multifill {", "block-start", 2, 1,
				v023Element(t, "tag", "@multifill", "multifill", 1, 11), v023Element(t, "block-open", "{", "{", 12, 13)),
			v023Line(t, "_____{1O5}", "content", 3, 2, v023Element(t, "placeholder", "_____{1O5}", "1O5", 1, 11)),
			v023Line(t, "}", "block-end", 3, 2, v023Element(t, "block-close", "}", "}", 1, 2)),
		}, []v023Expected{{4, 7, 10, "O → 0"}}},
		// DSL 3.8.6, 3.8.11: собственный текст инструкции и один вложенный hint.
		{"nested hint", []model.Line{
			v023Line(t, "@instr {", "block-start", 2, 1,
				v023Element(t, "tag", "@instr", "instr", 1, 7), v023Element(t, "block-open", "{", "{", 8, 9)),
			v023Text(t, "Read.", 3, 2),
			v023Line(t, "@hint {", "block-start", 3, 2,
				v023Element(t, "tag", "@hint", "hint", 1, 6), v023Element(t, "block-open", "{", "{", 7, 8)),
			v023Text(t, "1O5", 5, 3),
			v023Line(t, "}", "block-end", 5, 3, v023Element(t, "block-close", "}", "}", 1, 2)),
			v023Line(t, "}", "block-end", 3, 2, v023Element(t, "block-close", "}", "}", 1, 2)),
		}, []v023Expected{{6, 1, 4, "O → 0"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v023Run(t, v023Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV023Unparsed(t *testing.T) {
	// DSL 1.16: неизвестный тег ошибочен. Код прежней диагностики — непрозрачная
	// тестовая история, а не предположение о кодах внешнего парсера.
	line := v023Line(t, "@unknown 1O5", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown 1O5", Start: 1, End: 13, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	result := v023Document(t, line)
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{
		ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 13}},
		RelatedLocations: []model.Location{},
	}}
	v023Run(t, result, "current", []v023Expected{{3, 10, 13, "O → 0"}})
}

func TestV023TitleNameIdentifierAndPath(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v023Document(t)
		title := "1O5"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v023Line(t, "# 1O5", "heading", 0, 0,
			v023Element(t, "heading-level", "#", "1", 1, 2), v023Element(t, "title", title, title, 3, 6))
		result.Lines[1].Number = 2
		v023Run(t, result, "current", []v023Expected{{2, 3, 6, "O → 0"}})
	})
	t.Run("identifier", func(t *testing.T) {
		v023Run(t, v023Document(t, v023Line(t, "@task 1O5", "tag", 2, 1,
			v023Element(t, "tag", "@task", "task", 1, 6), v023Element(t, "identifier", "1O5", "1O5", 7, 10))),
			"current", []v023Expected{{3, 7, 10, "O → 0"}})
	})
	for _, tc := range []struct {
		name string
		line model.Line
		want []v023Expected
	}{
		{"section name", v023Line(t, "@section 1O5", "tag", 0, 0,
			v023Element(t, "tag", "@section", "section", 1, 9), v023Element(t, "name", "1O5", "1O5", 10, 13)),
			[]v023Expected{{2, 10, 13, "O → 0"}}},
		{"resource path", v023Line(t, `@resource-dir "1O5"`, "tag", 0, 0,
			v023Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v023Element(t, "resource-path", `"1O5"`, "1O5", 15, 20)), []v023Expected{{2, 16, 19, "O → 0"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.31: метаданные располагаются до заголовка первого уровня.
			result := v023Document(t)
			heading := result.Lines[1]
			heading.Number = 3
			tc.line.Number = 2
			result.Lines = []model.Line{result.Lines[0], tc.line, heading}
			count := 3
			result.Document.LineCount = &count
			if tc.name == "section name" {
				section := "1O5"
				result.Document.Metadata.Section = &section
			} else {
				result.Document.Metadata.ResourceDirs = []string{"1O5"}
			}
			v023Run(t, result, "current", tc.want)
		})
	}
}

func TestV023LineEndings(t *testing.T) {
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
			result := v023Document(t, v023Text(t, "Стоимость 1O5 рублей.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v023Run(t, result, "current", []v023Expected{{3, 11, 14, "O → 0"}})
		})
	}
}

func TestV023UnavailableLines(t *testing.T) {
	// Контракт модели допускает известную версию при отсутствии доступных строк.
	v023Run(t, v023UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV023UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v023UnavailableDocument(t, version)
			before := v023Snapshot(t, result)
			checks, err := validator.Prepare(result, v023Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v023Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV023RepeatedRuns(t *testing.T) {
	result := v023Document(t, v023Text(t, "1O5", 2, 1))
	v023Run(t, result, "first", []v023Expected{{3, 1, 4, "O → 0"}})
	v023Run(t, result, "second", []v023Expected{{3, 1, 4, "O → 0"}})
	v023Run(t, v023Document(t, v023Text(t, "105", 2, 1)), "first", nil)
}

// v023Registry создаёт реестр только через штатный каталог без замены регистраций.
func v023Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v023UnavailableDocument задаёт известную версию при недоступном исходнике.
func v023UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v023Document дополняет фрагмент обязательными @dsl-version и заголовком
// по DSL 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по 2.1.15.
// Элементы, родители и глубина тела заданы явно; повторного разбора DSL нет.
func v023Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v023UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{
		v023Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v023Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v023Element(t, "version", "1.2", "1.2", 14, 17)),
		v023Line(t, "# Unit 1", "heading", 0, 0,
			v023Element(t, "heading-level", "#", "1", 1, 2), v023Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v023Line сохраняет заданную структуру физической строки с LF без разбора Raw.
func v023Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v023Element сохраняет заданные Raw, Value и исходный Unicode-диапазон.
func v023Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v023Text создаёт содержимое без краевых пробелов и снимаемого экранирования.
// Длина вычисляется только для входного элемента, не для ожидаемых результатов.
func v023Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v023Line(t, text, "content", parent, depth,
		v023Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v023Run проверяет V023 текущего запуска через полный штатный конвейер.
// Все координаты и пары замен задаются тестом независимо от реализации.
func v023Run(t *testing.T, result *model.Result, processingID string, want []v023Expected) {
	t.Helper()
	before := v023Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v023Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v023Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V023" && entry.Source != processingID {
			t.Fatalf("new V023 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V023" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V023 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "В числовой последовательности находится буква, похожая на цифру.\nконкретизация: Возможные замены: " + expected.detail
		if entry.DiagnosticCode != "V023" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
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
	// исходных данных; ожидания V023 и прежняя история остаются неизменными.
	if _, err := validator.Prepare(result, v023Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var previous, original model.Result
	if err := json.Unmarshal([]byte(v023Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	otherNewIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V023" {
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
	if v023Snapshot(t, &previous) != before {
		t.Fatal("document, lines, flags, element links or previous history changed")
	}
}

// v023Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v023Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

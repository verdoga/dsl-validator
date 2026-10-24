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

func TestV006Registration(t *testing.T) {
	definitions, err := v006Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V006" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Message != "Двойная кавычка не имеет пары в строке." ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V006 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V006 definitions = %d, want 1", matches)
	}
}

func TestV006TextPairs(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"no quotes", "Read this.", nil},
		{"typographic quotes", "«да» “yes” „ja“", nil},
		{"single quote", `"`, [][3]int{{3, 1, 2}}},
		{"empty pair", `""`, nil},
		{"adjacent pairs", `""""`, nil},
		{"three quotes", `"""`, [][3]int{{3, 3, 4}}},
		{"pair followed by unmatched quote", `"a" "b`, [][3]int{{3, 5, 6}}},
		{"pairs separated by text", `"a" and "b"`, nil},
		{"Unicode columns", `Я😀"`, [][3]int{{3, 3, 4}}},
		{"combining mark column", "e\u0301\"", [][3]int{{3, 3, 4}}},
		{"ordinary backslash does not escape quote", `A\"`, [][3]int{{3, 3, 4}}},
		{"ordinary backslash inside pair", `"a\"`, nil},
		{"two backslashes in text", `A\\"`, [][3]int{{3, 4, 5}}},
		{"typographic quotes do not complete pair", `“a"b”`, [][3]int{{3, 3, 4}}},
		{"HTML entity is not decoded", `&quot;`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// В этих строках Value совпадает с Raw, кроме экранированной пары
			// обратных косых черт; её семантическое значение задано явно ниже.
			value := tc.raw
			if tc.name == "two backslashes in text" {
				value = `A\"`
			}
			v006Run(t, v006Document(t, v006Text(t, tc.raw, value, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV006PhysicalLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"reference", []model.Line{
			v006Text(t, `He said "hello".`, `He said "hello".`, 2, 1),
			v006Text(t, `She answered "yes".`, `She answered "yes".`, 2, 1),
		}, nil},
		{"example", []model.Line{
			v006Text(t, `He said "hello.`, `He said "hello.`, 2, 1),
			v006Text(t, `She answered yes".`, `She answered yes".`, 2, 1),
		}, [][3]int{{3, 9, 10}, {4, 17, 18}}},
		{"several unmatched lines", []model.Line{
			v006Text(t, `"`, `"`, 2, 1), v006Text(t, `"""`, `"""`, 2, 1),
		}, [][3]int{{3, 1, 2}, {4, 3, 4}}},
		{"empty physical line", []model.Line{v006Line(t, "", "blank", 2, 1)}, nil},
		{"whitespace only", []model.Line{v006Line(t, " \t ", "blank", 2, 1)}, nil},
		{"raw edges and case", []model.Line{
			v006Line(t, `  Ab"  `, "content", 2, 1, v006Element(t, "content", `Ab"`, `Ab"`, 3)),
		}, [][3]int{{3, 5, 6}}},
		{"tab before quote", []model.Line{
			v006Line(t, "\t\"", "content", 2, 1, v006Element(t, "content", `"`, `"`, 2)),
		}, [][3]int{{3, 2, 3}}},
		{"single-line editor", []model.Line{
			v006Line(t, `@editor A"`, "tag", 2, 1,
				v006Element(t, "tag", "@editor", "editor", 1), v006Element(t, "content", `A"`, `A"`, 9)),
		}, [][3]int{{3, 10, 11}}},
		{"media-like text in single-line editor", []model.Line{
			v006Line(t, `@editor @media audio "a\"`, "tag", 2, 1,
				v006Element(t, "tag", "@editor", "editor", 1),
				v006Element(t, "content", `@media audio "a\"`, `@media audio "a\"`, 9)),
		}, nil},
		{"escaped tag remains text", []model.Line{
			v006Text(t, `\@media audio "a\"`, `@media audio "a\"`, 2, 1),
		}, nil},
		{"media name inside text", []model.Line{
			v006Text(t, `See @media audio "a\"`, `See @media audio "a\"`, 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v006Run(t, v006Document(t, tc.body...), "current", tc.want, 1)
		})
	}
}

func TestV006MediaSources(t *testing.T) {
	// DSL 3.1.6: исходные кавычки и экранирование сохраняются в Raw,
	// а Value содержит путь без ограничивающих кавычек и со снятым экранированием.
	for _, tc := range []struct {
		name                string
		source              string
		value               string
		wantOtherErrorLines []int
	}{
		{"quoted source", `"a"`, "a", nil},
		{"quoted source with spaces", `"a b"`, "a b", nil},
		{"escaped quote", `"a\"b"`, `a"b`, nil},
		{"two backslashes", `"a\\"`, `a\`, nil},
		{"four backslashes", `"a\\\\"`, `a\\`, nil},
		{"three backslashes and closing quote", `"a\\\"b"`, `a\"b`, nil},
		// Кавычки парные: V006 отсутствует, но V085 выставляет флаг строки 3.
		{"unrecognized escape", `"a\q"`, `a\q`, []int{3}},
		{"unquoted path", `a.wav`, `a.wav`, nil},
		{"numeric source", `0012`, `0012`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v006Run(t, v006Document(t, v006Media(t, tc.source, tc.value)), "current", nil, 1, tc.wantOtherErrorLines...)
		})
	}
}

func TestV006UnparsedMedia(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   [][3]int
	}{
		{"unclosed source", `"a`, [][3]int{{3, 14, 15}}},
		{"last quote escaped", `"a\"`, [][3]int{{3, 14, 15}}},
		{"three backslashes before last quote", `"a\\\"`, [][3]int{{3, 14, 15}}},
		{"trailing single backslash", `"a\`, [][3]int{{3, 14, 15}}},
		{"trailing pair of backslashes", `"a\\`, [][3]int{{3, 14, 15}}},
		{"quote not at beginning of source", `a\"`, [][3]int{{3, 16, 17}}},
		// Пустой путь ошибочен по DSL 3.1.6, но его кавычки составляют пару.
		{"empty source has paired quotes", `""`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v006Line(t, "@media audio "+tc.source, "invalid", 2, 1,
				v006Element(t, "tag", "@media", "media", 1),
				v006Element(t, "media-type", "audio", "audio", 8),
				v006Unparsed(t, tc.source, 14))
			v006Run(t, v006Document(t, line), "current", tc.want, 1)
		})
	}
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"uppercase tag and type", v006Line(t, `@MEDIA AUDIO "a\"`, "invalid", 2, 1,
			v006Element(t, "tag", "@MEDIA", "media", 1), v006Element(t, "media-type", "AUDIO", "audio", 8),
			v006Unparsed(t, `"a\"`, 14)), [][3]int{{3, 14, 15}}},
		{"whole declaration unparsed", v006Line(t, `@media audio "a\"`, "invalid", 2, 1,
			v006Unparsed(t, `@media audio "a\"`, 1)), [][3]int{{3, 14, 15}}},
		{"indented uppercase whole declaration", v006Line(t, "\t @MEDIA audio \"a\\\"  ", "invalid", 2, 1,
			v006Unparsed(t, `@MEDIA audio "a\"`, 3)), [][3]int{{3, 16, 17}}},
		{"unparsed type and source", v006Line(t, `@media audio "a\"`, "invalid", 2, 1,
			v006Element(t, "tag", "@media", "media", 1), v006Unparsed(t, `audio "a\"`, 8)), [][3]int{{3, 14, 15}}},
		{"multiple U+0020 separators", v006Line(t, `@media  audio  "a\"`, "invalid", 2, 1,
			v006Element(t, "tag", "@media", "media", 1), v006Element(t, "media-type", "audio", "audio", 9),
			v006Unparsed(t, `"a\"`, 16)), [][3]int{{3, 16, 17}}},
		{"tail after parsed source", v006Line(t, `@media audio "a" \"`, "invalid", 2, 1,
			v006Element(t, "tag", "@media", "media", 1), v006Element(t, "media-type", "audio", "audio", 8),
			v006Element(t, "source", `"a"`, "a", 14), v006Unparsed(t, `\"`, 18)), [][3]int{{3, 19, 20}}},
		{"tail within unparsed remainder", v006Line(t, `@media audio "a" \"`, "invalid", 2, 1,
			v006Element(t, "tag", "@media", "media", 1), v006Element(t, "media-type", "audio", "audio", 8),
			v006Unparsed(t, `"a" \"`, 14)), [][3]int{{3, 19, 20}}},
		{"additional quoted value uses ordinary matching", v006Line(t, `@media audio "a" "b\"`, "invalid", 2, 1,
			v006Element(t, "tag", "@media", "media", 1), v006Element(t, "media-type", "audio", "audio", 8),
			v006Element(t, "source", `"a"`, "a", 14), v006Unparsed(t, `"b\"`, 18)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v006Run(t, v006Document(t, tc.line), "current", tc.want, 1)
		})
	}
}

func TestV006ResourcePaths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		line  model.Line
		paths []string
		want  [][3]int
	}{
		{"backslash is literal", v006Line(t, `@resource-dir "a\"`, "tag", 0, 0,
			v006Element(t, "tag", "@resource-dir", "resource-dir", 1),
			v006Element(t, "resource-path", `"a\"`, `a\`, 15)), []string{`a\`}, nil},
		{"unclosed path", v006Line(t, `@resource-dir "a\`, "invalid", 0, 0,
			v006Element(t, "tag", "@resource-dir", "resource-dir", 1),
			v006Unparsed(t, `"a\`, 15)), nil, [][3]int{{2, 15, 16}}},
		{"second path unclosed", v006Line(t, `@resource-dir "a", "b`, "invalid", 0, 0,
			v006Element(t, "tag", "@resource-dir", "resource-dir", 1),
			v006Element(t, "resource-path", `"a"`, "a", 15), v006Unparsed(t, `"b`, 20)), []string{"a"}, [][3]int{{2, 20, 21}}},
		{"commas and mixed path forms", v006Line(t, `@resource-dir Audio, "a,b", "c"`, "tag", 0, 0,
			v006Element(t, "tag", "@resource-dir", "resource-dir", 1),
			v006Element(t, "resource-path", "Audio", "Audio", 15),
			v006Element(t, "resource-path", `"a,b"`, "a,b", 22),
			v006Element(t, "resource-path", `"c"`, "c", 29)), []string{"Audio", "a,b", "c"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Мета-тег расположен до заголовка, как требует DSL 1.31.2.
			result := v006Document(t)
			result.Lines = append(result.Lines[:1], tc.line, result.Lines[1])
			result.Document.Metadata.ResourceDirs = append([]string{}, tc.paths...)
			v006Complete(t, result)
			v006Run(t, result, "current", tc.want, 1)
		})
	}
}

func TestV006LiteralBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		tag  string
	}{
		{"editor", "@editor", "editor"},
		{"multifill", "@multifill", "multifill"},
		{"uppercase editor", "@EDITOR", "editor"},
		{"uppercase multifill", "@MULTIFILL", "multifill"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 2.8.11 и 4.4.11: строка, похожая на media, является
			// содержимым, а следующий настоящий media находится уже вне блока.
			result := v006Document(t,
				v006Line(t, tc.raw+" {", "block-start", 2, 1,
					v006Element(t, "tag", tc.raw, tc.tag, 1),
					v006Element(t, "block-open", "{", "{", len(tc.raw)+2)),
				v006Text(t, `@media audio "a\"`, `@media audio "a\"`, 3, 2),
				v006Line(t, "}", "block-end", 3, 2, v006Element(t, "block-close", "}", "}", 1)),
			)
			v006Run(t, result, "literal", nil, 1)
			// Та же запись в настоящем SOURCE имеет экранированную кавычку.
			// В литеральном блоке все три U+0022 активны: последняя без пары.
			unpaired := v006Document(t,
				result.Lines[2],
				v006Text(t, `@media audio "a\"b"`, `@media audio "a\"b"`, 3, 2),
				result.Lines[4],
			)
			v006Run(t, unpaired, "current", [][3]int{{4, 19, 20}}, 1)
			// Отдельная модель сохраняет согласованную историю: новый исходник
			// не подставляется в результат уже завершённого запуска.
			withMedia := v006Document(t,
				result.Lines[2], result.Lines[3], result.Lines[4],
				v006Line(t, `@media audio "a\"`, "invalid", 2, 1,
					v006Element(t, "tag", "@media", "media", 1),
					v006Element(t, "media-type", "audio", "audio", 8), v006Unparsed(t, `"a\"`, 14)),
			)
			v006Run(t, withMedia, "current", [][3]int{{6, 14, 15}}, 1)
		})
	}
}

func TestV006NestedContent(t *testing.T) {
	// DSL 3.8.6 и 3.8.11: собственный текст инструкции и вложенный hint.
	result := v006Document(t,
		v006Line(t, "@instr {", "block-start", 2, 1,
			v006Element(t, "tag", "@instr", "instr", 1), v006Element(t, "block-open", "{", "{", 8)),
		v006Text(t, "Read the sentence.", "Read the sentence.", 3, 2),
		v006Line(t, "@hint {", "block-start", 3, 2,
			v006Element(t, "tag", "@hint", "hint", 1), v006Element(t, "block-open", "{", "{", 7)),
		v006Text(t, `Я😀"`, `Я😀"`, 5, 3),
		v006Line(t, "}", "block-end", 5, 3, v006Element(t, "block-close", "}", "}", 1)),
		v006Line(t, "}", "block-end", 3, 2, v006Element(t, "block-close", "}", "}", 1)),
	)
	v006Run(t, result, "current", [][3]int{{6, 3, 4}}, 1)
}

func TestV006FirstAndLastLines(t *testing.T) {
	result := v006Document(t, v006Text(t, `"`, `"`, 2, 1))
	// DSL 1.2 запрещает добавочные параметры версии. Значение 1.2 известно,
	// а лишняя кавычка сохраняется как ошибочный неразобранный остаток.
	result.Lines[0].Raw = `@dsl-version 1.2 "`
	result.Lines[0].LineType = "invalid"
	result.Lines[0].Elements = append(result.Lines[0].Elements, v006Unparsed(t, `"`, 18))
	result.Lines[2].LineEnding = model.LineEndingNone
	v006Complete(t, result)
	v006Run(t, result, "current", [][3]int{{1, 18, 19}, {3, 1, 2}}, 1)
}

func TestV006LineEndings(t *testing.T) {
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
			result := v006Document(t,
				v006Text(t, `He said "hello.`, `He said "hello.`, 2, 1),
				v006Text(t, `She answered yes".`, `She answered yes".`, 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v006Run(t, result, "current", [][3]int{{3, 9, 10}, {4, 17, 18}}, 1)
		})
	}
}

func TestV006UnavailableLinesAndVersions(t *testing.T) {
	v006Run(t, v006UnavailableDocument(t, "1.2"), "current", nil, 0)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v006UnavailableDocument(t, version)
			before := v006Snapshot(t, result)
			checks, err := validator.Prepare(result, v006Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v006Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV006HistoryAndRepeatedRuns(t *testing.T) {
	result := v006Document(t, v006Text(t, `"`, `"`, 2, 1))
	v006Run(t, result, "first", [][3]int{{3, 1, 2}}, 1)
	v006Run(t, result, "second", [][3]int{{3, 1, 2}}, 1)
	v006Run(t, v006Document(t, v006Text(t, `""`, `""`, 2, 1)), "first", nil, 1)

	// Ошибочный вывод прежнего инструмента не является готовым фактом парности.
	// Его код намеренно совпадает с текущим, но источник — другой запуск.
	paired := v006Document(t, v006Text(t, `""`, `""`, 2, 1))
	paired.Processing = append(paired.Processing, model.Processing{ID: "previous", Tool: "dsl-validator"})
	paired.Diagnostics = append(paired.Diagnostics, model.Diagnostic{
		ID: "old-v006", Source: "previous", DiagnosticCode: "V006", SeverityLevel: "error",
		DiagnosticScope: "line", Message: "Прежнее сообщение о кавычке.",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 2}},
		RelatedLocations: []model.Location{},
	})
	paired.Document.HasErrors, paired.Lines[2].HasErrors = true, true
	v006Run(t, paired, "current", nil, 1)
}

func TestV006CoexistsWithOtherChecks(t *testing.T) {
	result := v006Document(t, v006Text(t, "A  \t\"", "A  \t\"", 2, 1))
	v006Run(t, result, "current", [][3]int{{3, 5, 6}}, 1)
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
			if entry.Source != "current" || entry.DiagnosticCode != tc.code {
				continue
			}
			matches++
			want := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
			if entry.Message != tc.message || entry.Location == nil || *entry.Location != want ||
				entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
		if matches != 1 {
			t.Errorf("current %s count = %d, want 1", tc.code, matches)
		}
	}
}

// v006Registry создаёт реестр исключительно через штатный каталог регистраций.
func v006Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v006UnavailableDocument представляет недоступный исходник с известной версией.
func v006UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v006Document дополняет фрагмент технической строкой и заголовком по DSL 1.30.
// Исходный текст фрагмента не меняется. Обычный текст образует шаг по 2.1.15;
// родители, вложенность и элементы тела заданы самими ситуациями, без парсера.
func v006Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v006UnavailableDocument(t, "1.2")
	title, encoding := "Unit 1", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v006Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v006Element(t, "tag", "@dsl-version", "dsl-version", 1), v006Element(t, "version", "1.2", "1.2", 14)),
		v006Line(t, "# Unit 1", "heading", 0, 0,
			v006Element(t, "heading-level", "#", "1", 1), v006Element(t, "title", "Unit 1", "Unit 1", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v006Complete(t, result)
	return result
}

// v006Complete нумерует заданные строки и снабжает unparsed обязательной
// историей ошибки парсера. Это оформление входной модели, не разбор DSL:
// типы и границы ошибочных фрагментов явно заданы тестовыми ситуациями.
func v006Complete(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" || len(element.ErrorIDs) != 0 {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", line.Number, j)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Неразобранный фрагмент объявления.",
				Location: &model.Location{
					Start: model.Position{Line: line.Number, Column: element.Start},
					End:   model.Position{Line: line.Number, Column: element.End},
				}, RelatedLocations: []model.Location{},
			})
		}
	}
}

// v006Line собирает явно заданную строку, не распознавая её содержимое.
func v006Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v006Element сохраняет явные исходное и семантическое значения. Длина нужна
// только для входной модели; ожидаемые диапазоны срабатываний не вычисляются.
func v006Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v006Unparsed создаёт явно указанный ошибочный фрагмент с отсутствующим Value.
// Ссылку и соответствующую диагностику входной модели добавляет v006Complete.
func v006Unparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v006Element(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

// v006Text создаёт строку содержимого с явно указанным семантическим значением.
func v006Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v006Line(t, raw, "content", parent, depth, v006Element(t, "content", raw, value, 1))
}

// v006Media собирает разобранный media с заданными исходником и значением пути.
func v006Media(t *testing.T, source, value string) model.Line {
	t.Helper()
	return v006Line(t, "@media audio "+source, "tag", 2, 1,
		v006Element(t, "tag", "@media", "media", 1), v006Element(t, "media-type", "audio", "audio", 8),
		v006Element(t, "source", source, value, 14))
}

// v006Run запускает штатный конвейер; want содержит явные тройки
// (строка, начало, исключающий конец). Отдельно проверяется сопутствующая V037.
// wantV037 задаёт точное число сопутствующих ошибок отсутствующего ID;
// wantOtherErrorLines явно задаёт строки с ошибками других диагностик.
// флаг документа проверяется отдельно, строки и прежняя история сохраняются.
func v006Run(t *testing.T, result *model.Result, processingID string, want [][3]int, wantV037 int, wantOtherErrorLines ...int) {
	t.Helper()
	before := v006Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v006Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v006Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	gotV037 := 0
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= len(original.Diagnostics) && entry.Source != processingID {
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
		if entry.DiagnosticCode == "V006" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V006 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		// Номер берётся из явного ожидания теста, не из результата проверки.
		message := fmt.Sprintf("Двойная кавычка не имеет пары в строке.\nконкретизация: У кавычки в колонке %d нет пары в этой строке", expected[1])
		if entry.DiagnosticCode != "V006" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.Message != message || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if result.Document.HasErrors != (original.Document.HasErrors || len(want) > 0 || wantV037 > 0 || len(wantOtherErrorLines) > 0) {
		t.Error("unexpected document.hasErrors")
	}
	if len(result.Lines) != len(original.Lines) || len(result.Processing) != len(original.Processing)+1 ||
		len(result.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("unexpected line count, processing count or lost history")
	}
	for i, line := range result.Lines {
		wantError := original.Lines[i].HasErrors
		for _, expected := range want {
			wantError = wantError || expected[0] == line.Number
		}
		for _, number := range wantOtherErrorLines {
			wantError = wantError || number == line.Number
		}
		if line.HasErrors != wantError {
			t.Errorf("line %d hasErrors = %v, want %v", line.Number, line.HasErrors, wantError)
		}
	}
	// После проверки ожидаемых флагов сравниваются все остальные данные:
	// Raw, Value, EOL, структура, ссылки элементов и неизменённая история.
	previous := *result
	previous.Document.HasErrors = original.Document.HasErrors
	previous.Lines = append([]model.Line{}, result.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
	}
	previous.Processing = result.Processing[:len(original.Processing)]
	previous.Diagnostics = result.Diagnostics[:len(original.Diagnostics)]
	if v006Snapshot(t, &previous) != before {
		t.Fatal("source model, element links or previous history changed")
	}
}

// v006Snapshot фиксирует модель без отражения и доступа к внутренностям валидатора.
func v006Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

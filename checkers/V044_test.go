package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

type v044Expected struct {
	line, start, end int
	detail           string
}

func TestV044Registration(t *testing.T) {
	definitions, err := v044Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range definitions {
		if d.Code != "V044" {
			continue
		}
		count++
		if d.Severity != "error" || d.Scope != "line" || d.Check == nil ||
			d.Message != "В @resource-dir указан пустой путь." ||
			d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("definition: %+v", d)
		}
	}
	if count != 1 {
		t.Fatalf("definitions = %d, want 1", count)
	}
}

func TestV044ReferenceAndExample(t *testing.T) {
	// Оба фрагмента из таблицы помещены в метаданные до #; добавлены только
	// версия, ID документа (для V037) и заголовок полноценного документа.
	t.Run("reference", func(t *testing.T) {
		result := v044Document(t, v044Line(t, `@resource-dir Audio, "Shared resources"`, "tag", 0, 0,
			v044Element(t, "tag", "@resource-dir", "resource-dir", 1),
			v044Element(t, "resource-path", "Audio", "Audio", 15),
			v044Element(t, "resource-path", `"Shared resources"`, "Shared resources", 22)))
		v044Run(t, result, "current", nil)
	})
	t.Run("example with parsed prefix and unparsed remainder", func(t *testing.T) {
		result := v044Document(t, v044Line(t, `@resource-dir Audio, "", Video,`, "invalid", 0, 0,
			v044Element(t, "tag", "@resource-dir", "resource-dir", 1),
			v044Element(t, "resource-path", "Audio", "Audio", 15),
			v044Unparsed(t, `"", Video,`, 22)))
		v044Run(t, result, "current", []v044Expected{{3, 15, 32, "Пустые части списка: 2, 4."}})
	})
}

func TestV044EmptyAndMalformedParts(t *testing.T) {
	// Для ошибочного списка парсер сохранил весь остаток как unparsed.
	// История ошибки обязательна по контракту модели; V044 проверяет Raw.
	for _, tc := range []struct {
		name, tail string
		want       []v044Expected
		quotes     [][3]int
	}{
		{"empty quoted path", ` ""`, []v044Expected{{3, 15, 17, "Пустые части списка: 1."}}, nil},
		{"quoted spaces and tab", " \" \t \"", []v044Expected{{3, 15, 20, "Пустые части списка: 1."}}, nil},
		{"comma only", ` ,`, []v044Expected{{3, 15, 16, "Пустые части списка: 1, 2."}}, nil},
		{"first part", ` ,Audio`, []v044Expected{{3, 15, 21, "Пустые части списка: 1."}}, nil},
		{"middle part", ` Audio,,Video`, []v044Expected{{3, 15, 27, "Пустые части списка: 2."}}, nil},
		{"last part", ` Audio,`, []v044Expected{{3, 15, 21, "Пустые части списка: 2."}}, nil},
		{"last part contains horizontal spaces", " Audio, \t", []v044Expected{{3, 15, 21, "Пустые части списка: 2."}}, nil},
		{"trim list edges", `  ,  `, []v044Expected{{3, 16, 17, "Пустые части списка: 1, 2."}}, nil},
		{"three empty parts", ` ,,`, []v044Expected{{3, 15, 17, "Пустые части списка: 1, 2, 3."}}, nil},
		{"part numbers above nine", ` ,,,,,,,,,,`, []v044Expected{{3, 15, 25, "Пустые части списка: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11."}}, nil},
		{"quoted comma and empty part", ` ",",`, []v044Expected{{3, 15, 19, "Пустые части списка: 2."}}, nil},
		{"Unicode path before empty part", ` Ресурсы😀, ""`, []v044Expected{{3, 15, 27, "Пустые части списка: 2."}}, nil},
		{"spaces inside nonempty path", ` " Audio ", ""`, []v044Expected{{3, 15, 28, "Пустые части списка: 2."}}, nil},
		{"trailing text after quotes", ` ""tail`, nil, nil},
		{"trailing text then empty part", ` ""tail,`, []v044Expected{{3, 15, 22, "Пустые части списка: 2."}}, nil},
		{"multiple quote shells", ` "" ""`, nil, nil},
		{"four adjacent quotes", ` """"`, nil, nil},
		{"text before quotes", ` a""`, nil, nil},
		{"Unicode space after closing quote", " \"\"\u00a0", nil, nil},
		{"unclosed opening quote", ` "`, nil, [][3]int{{3, 15, 16}}},
		{"unclosed spaces", " \" \t ", nil, [][3]int{{3, 15, 16}}},
		{"comma inside unclosed quote", ` ",`, nil, [][3]int{{3, 15, 16}}},
		{"empty part before unclosed quote", ` , "Audio`, []v044Expected{{3, 15, 23, "Пустые части списка: 1."}}, [][3]int{{3, 17, 18}}},
		{"backslash does not escape closing quote", ` "\",`, []v044Expected{{3, 15, 19, "Пустые части списка: 2."}}, nil},
		{"backslash does not escape comma", ` Audio\,`, []v044Expected{{3, 15, 22, "Пустые части списка: 2."}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v044Document(t, v044Line(t, "@resource-dir"+tc.tail, "invalid", 0, 0,
				v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, tc.tail, 14)))
			v044Run(t, result, "current", tc.want)
			v044Quotes(t, result, tc.quotes)
		})
	}
}

func TestV044NonemptyPaths(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"unquoted", "Audio", "Audio"},
		{"internal spaces", "Shared resources", "Shared resources"},
		{"quoted comma", `"Audio, Shared"`, "Audio, Shared"},
		{"only comma in quotes", `","`, ","},
		{"backslash in quotes", `"\"`, `\`},
		{"unquoted NBSP", "\u00a0", "\u00a0"},
		{"quoted NBSP", "\"\u00a0\"", "\u00a0"},
		{"unquoted em space", "\u2003", "\u2003"},
		{"quoted em space", "\"\u2003\"", "\u2003"},
		{"no entity decoding", "&#32;", "&#32;"},
		{"no backslash decoding", `"\t"`, `\t`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v044Document(t, v044Line(t, "@resource-dir "+tc.raw, "tag", 0, 0,
				v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Element(t, "resource-path", tc.raw, tc.value, 15)))
			v044Run(t, result, "current", nil)
			v044Quotes(t, result, nil)
		})
	}
}

func TestV044MissingList(t *testing.T) {
	for _, raw := range []string{"@resource-dir", "@resource-dir \t "} {
		t.Run(raw, func(t *testing.T) {
			result := v044Document(t, v044Line(t, raw, "tag", 0, 0, v044Element(t, "tag", "@resource-dir", "resource-dir", 1)))
			v044LineError(t, result, 3, 1, 14)
			v044Run(t, result, "current", []v044Expected{{3, 1, 14, "Пустые части списка: 1."}})
		})
	}
}

func TestV044RecognitionAndPositions(t *testing.T) {
	for _, tc := range []struct {
		name, raw, tag          string
		tagStart, argumentStart int
		want                    v044Expected
	}{
		{"uppercase", `@RESOURCE-DIR ""`, "@RESOURCE-DIR", 1, 15, v044Expected{3, 15, 17, "Пустые части списка: 1."}},
		{"indentation", " \t@resource-dir \"\"", "@resource-dir", 3, 17, v044Expected{3, 17, 19, "Пустые части списка: 1."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Пустое значение resource-path допустимо входной моделью.
			// Проверяем установку новых error-флагов без прежних диагностик.
			result := v044Document(t, v044Line(t, tc.raw, "tag", 0, 0,
				v044Element(t, "tag", tc.tag, "resource-dir", tc.tagStart), v044Element(t, "resource-path", `""`, "", tc.argumentStart)))
			v044Run(t, result, "first", []v044Expected{tc.want})
			v044Run(t, result, "second", []v044Expected{tc.want})
		})
	}
	t.Run("entirely unparsed tag is not recognized", func(t *testing.T) {
		result := v044Document(t, v044Line(t, `@resource-dir ""`, "invalid", 0, 0, v044Unparsed(t, `@resource-dir ""`, 1)))
		v044Run(t, result, "current", nil)
	})
	t.Run("other recognized tag", func(t *testing.T) {
		result := v044Document(t)
		result.Lines = append(result.Lines, v044Line(t, `@header ""`, "tag", 3, 1,
			v044Element(t, "tag", "@header", "header", 1), v044Element(t, "title", `""`, `""`, 9)))
		v044NumberLines(t, result)
		v044Run(t, result, "current", nil)
	})
}

func TestV044RepeatedAndMisplacedDeclarations(t *testing.T) {
	result := v044Document(t,
		v044Line(t, `@resource-dir ""`, "invalid", 0, 0, v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, `""`, 15)),
		v044Line(t, "", "blank", 0, 0),
		v044Line(t, `@resource-dir ,`, "invalid", 0, 0, v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, ",", 15)))
	// После заголовка и внутри обычного блока тег распознан, хотя расположен неверно.
	result.Lines = append(result.Lines,
		v044Line(t, "@text {", "block-start", 6, 1, v044Element(t, "tag", "@text", "text", 1), v044Element(t, "block-open", "{", "{", 7)),
		v044Line(t, `@resource-dir ""`, "tag", 7, 2, v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, `""`, 15)),
		v044Line(t, "}", "block-end", 7, 2, v044Element(t, "block-close", "}", "}", 1)),
		v044Line(t, `@resource-dir ""`, "tag", 6, 1, v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, `""`, 15)))
	v044NumberLines(t, result)
	v044ParserHistory(t, result)
	v044LineError(t, result, 8, 1, 14)
	v044LineError(t, result, 10, 1, 14)
	v044Run(t, result, "current", []v044Expected{
		{3, 15, 17, "Пустые части списка: 1."}, {5, 15, 16, "Пустые части списка: 1, 2."},
		{8, 15, 17, "Пустые части списка: 1."}, {10, 15, 17, "Пустые части списка: 1."},
	})
}

func TestV044LiteralContent(t *testing.T) {
	for _, tag := range []string{"editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			result := v044Document(t)
			result.Lines = append(result.Lines,
				v044Line(t, "@"+tag+" {", "block-start", 3, 1, v044Element(t, "tag", "@"+tag, tag, 1), v044Element(t, "block-open", "{", "{", len(tag)+3)),
				v044Text(t, `@resource-dir ""`, 4, 2),
				v044Text(t, `@resource-dir ,`, 4, 2),
				v044Line(t, "}", "block-end", 4, 2, v044Element(t, "block-close", "}", "}", 1)),
				v044Text(t, `Text mentions @resource-dir ""`, 3, 1))
			v044NumberLines(t, result)
			v044Run(t, result, "current", nil)
		})
	}
}

func TestV044QuoteStateResetsBetweenLines(t *testing.T) {
	result := v044Document(t,
		v044Line(t, `@resource-dir "`, "invalid", 0, 0, v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, `"`, 15)),
		v044Line(t, `@resource-dir "", Audio`, "invalid", 0, 0, v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, `"", Audio`, 15)))
	v044Run(t, result, "current", []v044Expected{{4, 15, 24, "Пустые части списка: 1."}})
	v044Quotes(t, result, [][3]int{{3, 15, 16}})
}

func TestV044VersionsAndLineEndings(t *testing.T) {
	v044Run(t, v044UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v044UnavailableDocument(t, version)
			before := v044Snapshot(t, result)
			checks, err := validator.Prepare(result, v044Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v044Snapshot(t, result) != before {
				t.Fatal("unsupported version changed model")
			}
		})
	}
	for _, ending := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF, model.LineEndingNone} {
		t.Run("ending "+string(ending), func(t *testing.T) {
			result := v044Document(t)
			result.Lines = append(result.Lines, v044Line(t, `@resource-dir ""`, "tag", 3, 1,
				v044Element(t, "tag", "@resource-dir", "resource-dir", 1), v044Unparsed(t, `""`, 15)))
			v044NumberLines(t, result)
			v044ParserHistory(t, result)
			v044LineError(t, result, 4, 1, 14)
			if ending == model.LineEndingNone {
				result.Lines[3].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v044Run(t, result, "current", []v044Expected{{4, 15, 17, "Пустые части списка: 1."}})
		})
	}
}

// v044Quotes проверяет отдельные срабатывания V006 в том же штатном запуске:
// V044 не дублирует ошибку кавычек, но сохраняет независимые пустые части.
func v044Quotes(t *testing.T, result *model.Result, want [][3]int) {
	t.Helper()
	var got []model.Diagnostic
	for _, d := range result.Diagnostics {
		if d.Source == "current" && d.DiagnosticCode == "V006" {
			got = append(got, d)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V006 count = %d, want %d", len(got), len(want))
	}
	for i, p := range want {
		location := model.Location{Start: model.Position{Line: p[0], Column: p[1]}, End: model.Position{Line: p[0], Column: p[2]}}
		if got[i].Location == nil || *got[i].Location != location || got[i].Message != fmt.Sprintf("Двойная кавычка не имеет пары в строке.\nконкретизация: У кавычки в колонке %d нет пары в этой строке", p[1]) {
			t.Errorf("V006 occurrence: %+v, want %+v", got[i], location)
		}
	}
}

// v044LineError добавляет согласованную историю ошибки отсутствующего аргумента
// или неверного расположения тега; каталог ошибок внешнего парсера недоступен.
func v044LineError(t *testing.T, result *model.Result, line, start, end int) {
	t.Helper()
	result.Document.HasErrors, result.Lines[line-1].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: fmt.Sprintf("parser-line-%d", line), Source: "parser", DiagnosticCode: "fixture-declaration-error",
		SeverityLevel: "error", DiagnosticScope: "line", Message: "Ошибочное объявление.",
		Location: &model.Location{Start: model.Position{Line: line, Column: start}, End: model.Position{Line: line, Column: end}}, RelatedLocations: []model.Location{},
	})
	// Вся добавленная здесь история относится к одному запуску parser.
	// Контракт модели требует порядок по исходным позициям внутри запуска.
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		a, b := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return a.Line < b.Line || a.Line == b.Line && a.Column < b.Column
	})
}

// v044Unparsed задаёт ошибочный исходный диапазон. Ссылку на историю добавляет
// v044ParserHistory после назначения физических номеров строк.
func v044Unparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v044Element(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

// v044ParserHistory создаёт согласованную историю только для явно заданных
// тестом unparsed. Код условный: каталог диагностик внешнего парсера недоступен.
func v044ParserHistory(t *testing.T, result *model.Result) {
	t.Helper()
	for i := range result.Lines {
		line := &result.Lines[i]
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" || len(element.ErrorIDs) != 0 {
				continue
			}
			id := fmt.Sprintf("parser-error-%d-%d", line.Number, j)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: "Ошибочный исходный фрагмент.", Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}, RelatedLocations: []model.Location{}})
		}
	}
}

// v044Registry создаёт реестр исключительно через штатный каталог регистраций.
func v044Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v044UnavailableDocument представляет отсутствие доступных строк
// при известной версии DSL, а не пустой исходник с выдуманным тегом версии.
func v044UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v044Document добавляет техническую строку, ID документа и заголовок.
// Метаданные находятся до заголовка; исходные фрагменты не изменяются.
// Структура и значения элементов задаются тестами, Raw не разбирается.
func v044Document(t *testing.T, metadata ...model.Line) *model.Result {
	t.Helper()
	result := v044UnavailableDocument(t, "1.2")
	title, encoding, id := "Lesson", "UTF-8", "lesson-a"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Document.Metadata.DocumentID = &id
	result.Lines = []model.Line{
		v044Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v044Element(t, "tag", "@dsl-version", "dsl-version", 1), v044Element(t, "version", "1.2", "1.2", 14)),
		v044Line(t, "@document-id lesson-a", "tag", 0, 0,
			v044Element(t, "tag", "@document-id", "document-id", 1), v044Element(t, "identifier", "lesson-a", "lesson-a", 14)),
	}
	result.Lines = append(result.Lines, metadata...)
	result.Lines = append(result.Lines, v044Line(t, "# Lesson", "heading", 0, 0,
		v044Element(t, "heading-level", "#", "1", 1), v044Element(t, "title", "Lesson", "Lesson", 3)))
	for _, line := range metadata {
		for _, element := range line.Elements {
			if element.ElementType == "resource-path" {
				result.Document.Metadata.ResourceDirs = append(result.Document.Metadata.ResourceDirs, *element.Value)
			}
		}
	}
	v044NumberLines(t, result)
	v044ParserHistory(t, result)
	return result
}

// v044NumberLines назначает физические номера уже заданным строкам модели.
func v044NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v044Line создаёт строку с заданной структурой, не распознавая Raw.
func v044Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v044Element сохраняет явные Raw и Value; длина нужна только для входа.
// Ожидаемые диапазоны диагностики заданы отдельно и не вычисляются по Raw.
func v044Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v044Text создаёт строку содержимого без экранирования, с Value равным Raw.
func v044Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v044Line(t, raw, "content", parent, depth, v044Element(t, "content", raw, raw, 1))
}

// v044Run проверяет V044 текущего запуска через штатный конвейер.
// want содержит явные диапазоны с исключающим концом и конкретизации.
// Новые флаги error проверяются по явным ожиданиям; исходные данные и история сохраняются.
func v044Run(t *testing.T, result *model.Result, processingID string, want []v044Expected) {
	t.Helper()
	before := v044Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v044Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v044Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.Source != processingID {
			t.Fatalf("new diagnostic has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V044" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V044 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V044" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.Message != "В @resource-dir указан пустой путь.\nконкретизация: "+expected.detail || entry.DiagnosticScope != "line" ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost diagnostic history")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	for _, occurrence := range want {
		expected.Document.HasErrors = true
		expected.Lines[occurrence.line-1].HasErrors = true
	}
	v044OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v044Snapshot(t, &previous) != v044Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v044Snapshot фиксирует всю модель для сравнения исходных данных и истории.
func v044Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v044OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v044OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V044" {
			continue
		}
		if entry.SeverityLevel == "error" {
			expected.Document.HasErrors = true
			if entry.Location != nil {
				expected.Lines[entry.Location.Start.Line-1].HasErrors = true
			}
		}
		if entry.DiagnosticScope != "element" {
			continue
		}
		if entry.Location == nil {
			t.Fatal("element diagnostic has no location")
		}
		matched := false
		for i := range expected.Lines[entry.Location.Start.Line-1].Elements {
			element := &expected.Lines[entry.Location.Start.Line-1].Elements[i]
			if element.Start == entry.Location.Start.Column && element.End == entry.Location.End.Column {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched = true
				break
			}
		}
		if !matched {
			t.Fatal("element diagnostic does not identify an element")
		}
	}
	for i := range expected.Lines {
		for j := range expected.Lines[i].Elements {
			slices.SortStableFunc(expected.Lines[i].Elements[j].ErrorIDs, func(a, b string) int { return order[a] - order[b] })
		}
	}
}

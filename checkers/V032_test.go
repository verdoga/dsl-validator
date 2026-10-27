package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

type v032Expected struct {
	line, start, end int
	detail           string
}

func TestV032Registration(t *testing.T) {
	definitions, err := v032Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range definitions {
		if d.Code != "V032" {
			continue
		}
		count++
		if d.Severity != "error" || d.Scope != "line" || d.Check == nil ||
			d.Message != "Идентификатор содержит недопустимые символы." ||
			d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("definition: %+v", d)
		}
	}
	if count != 1 {
		t.Fatalf("definitions = %d, want 1", count)
	}
}

func TestV032TaskArguments(t *testing.T) {
	for _, tc := range []struct {
		name, raw, argument string
		start               int
		want                []v032Expected
	}{
		{"reference", "@task Задание-А_2.1", "Задание-А_2.1", 7, nil},
		{"example", "@task task/a", "task/a", 7, []v032Expected{{3, 7, 13, `Недопустимые символы: "/" (U+002F).`}}},
		{"Unicode letters and decimal digits", "@task AЯ字١２._-", "AЯ字١２._-", 7, nil},
		{"uppercase tag and ID", "@TASK TASK-A", "TASK-A", 7, nil},
		{"precomposed letter", "@task é", "é", 7, nil},
		{"one allowed character", "@task _", "_", 7, nil},
		{"numeric ID", "@task 12", "12", 7, nil},
		{"internal space", "@task a b", "a b", 7, []v032Expected{{3, 7, 10, `Недопустимые символы: " " (U+0020).`}}},
		{"internal tab", "@task a\tb", "a\tb", 7, []v032Expected{{3, 7, 10, `Недопустимые символы: "\t" (U+0009).`}}},
		{"trim only edge spaces and tabs", "@task  \ta/b \t", "a/b", 9, []v032Expected{{3, 9, 12, `Недопустимые символы: "/" (U+002F).`}}},
		{"valid with edge spaces and tabs", "@task  \tvalid \t", "valid", 9, nil},
		{"trailing NBSP", "@task a\u00a0", "a\u00a0", 7, []v032Expected{{3, 7, 9, `Недопустимые символы: "\u00a0" (U+00A0).`}}},
		{"leading em space", "@task \u2003a", "\u2003a", 7, []v032Expected{{3, 7, 9, `Недопустимые символы: "\u2003" (U+2003).`}}},
		{"combining mark", "@task e\u0301", "e\u0301", 7, []v032Expected{{3, 7, 9, "Недопустимые символы: \"\u0301\" (U+0301)."}}},
		{"number outside Nd", "@task ²", "²", 7, []v032Expected{{3, 7, 8, `Недопустимые символы: "²" (U+00B2).`}}},
		{"letter number outside L", "@task Ⅳ", "Ⅳ", 7, []v032Expected{{3, 7, 8, `Недопустимые символы: "Ⅳ" (U+2163).`}}},
		{"one message and unique symbols", "@task a//b?/?", "a//b?/?", 7, []v032Expected{{3, 7, 14, `Недопустимые символы: "/" (U+002F), "?" (U+003F).`}}},
		{"first appearance order", "@task a?/??/", "a?/??/", 7, []v032Expected{{3, 7, 13, `Недопустимые символы: "?" (U+003F), "/" (U+002F).`}}},
		{"no unescaping", `@task a\/b`, `a\/b`, 7, []v032Expected{{3, 7, 11, `Недопустимые символы: "\\" (U+005C), "/" (U+002F).`}}},
		{"Unicode columns", "@task Я😀/а", "Я😀/а", 7, []v032Expected{{3, 7, 11, `Недопустимые символы: "😀" (U+1F600), "/" (U+002F).`}}},
		{"Unicode dash is forbidden", "@task a—b", "a—b", 7, []v032Expected{{3, 7, 10, `Недопустимые символы: "—" (U+2014).`}}},
		{"quotes are not decoded", `@task "id"`, `"id"`, 7, []v032Expected{{3, 7, 11, `Недопустимые символы: "\"" (U+0022).`}}},
		{"indented declaration", " \t@task a/b", "a/b", 9, []v032Expected{{3, 9, 12, `Недопустимые символы: "/" (U+002F).`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tagRaw, tagStart := tc.raw[:5], 1
			if tc.name == "indented declaration" {
				tagRaw, tagStart = "@task", 3
			}
			argument := v032Element(t, "identifier", tc.argument, tc.argument, tc.start)
			if len(tc.want) > 0 {
				argument = v032Unparsed(t, tc.argument, tc.start)
			}
			result := v032Document(t,
				v032Line(t, tc.raw, "tag", 2, 1, v032Element(t, "tag", tagRaw, "task", tagStart), argument),
				v032Text(t, "Read.", 3, 2))
			// Задания без @step не включают проверку минимума этапов V068.
			v032Run(t, result, "current", tc.want, 1)
		})
	}
}

func TestV032Declarations(t *testing.T) {
	for _, tc := range []struct {
		name, id string
		bad      bool
		want     []v032Expected
	}{
		{"valid document ID", "Good-А_2.1", false, nil},
		{"invalid document ID", "doc/id", true, []v032Expected{{2, 14, 20, `Недопустимые символы: "/" (U+002F).`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v032Document(t)
			argument := v032Element(t, "identifier", tc.id, tc.id, 14)
			if tc.bad {
				argument = v032Unparsed(t, tc.id, 14)
			} else {
				result.Document.Metadata.DocumentID = &tc.id
			}
			line := v032Line(t, "@document-id "+tc.id, "tag", 0, 0, v032Element(t, "tag", "@document-id", "document-id", 1), argument)
			result.Lines = append(result.Lines[:1], line, result.Lines[1])
			v032NumberLines(t, result)
			v032ParserHistory(t, result)
			v032Run(t, result, "current", tc.want, 0)
		})
	}
	for _, tc := range []struct {
		name, id string
		bad      bool
		want     []v032Expected
	}{
		{"valid fragment and include", "Good-А_2.1", false, nil},
		{"invalid fragment and include", "bad/id", true, []v032Expected{{3, 11, 17, `Недопустимые символы: "/" (U+002F).`}, {6, 10, 16, `Недопустимые символы: "/" (U+002F).`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition := v032Element(t, "identifier", tc.id, tc.id, 11)
			reference := v032Element(t, "identifier", tc.id, tc.id, 10)
			if tc.bad {
				definition, reference = v032Unparsed(t, tc.id, 11), v032Unparsed(t, tc.id, 10)
			}
			result := v032Document(t,
				v032Line(t, "@fragment "+tc.id+" {", "block-start", 2, 1, v032Element(t, "tag", "@fragment", "fragment", 1), definition, v032Element(t, "block-open", "{", "{", 12+utf8.RuneCountInString(tc.id))),
				v032Text(t, "Read.", 3, 2), v032Line(t, "}", "block-end", 3, 2, v032Element(t, "block-close", "}", "}", 1)),
				v032Line(t, "@include "+tc.id, "tag", 2, 1, v032Element(t, "tag", "@include", "include", 1), reference))
			v032Run(t, result, "current", tc.want, 1)
		})
	}
}

func TestV032RawArgumentAndRecovery(t *testing.T) {
	t.Run("identifier with unparsed remainder", func(t *testing.T) {
		result := v032Document(t,
			v032Line(t, "@task good bad/id", "tag", 2, 1, v032Element(t, "tag", "@task", "task", 1), v032Element(t, "identifier", "good", "good", 7), v032Unparsed(t, "bad/id", 12)), v032Text(t, "Read.", 3, 2))
		v032Run(t, result, "current", []v032Expected{{3, 7, 18, `Недопустимые символы: " " (U+0020), "/" (U+002F).`}}, 1)
	})
	t.Run("recognized fragment brace", func(t *testing.T) {
		result := v032Document(t,
			v032Line(t, "@fragment a/b {", "block-start", 2, 1, v032Element(t, "tag", "@fragment", "fragment", 1), v032Unparsed(t, "a/b", 11), v032Element(t, "block-open", "{", "{", 15)),
			v032Text(t, "Read.", 3, 2), v032Line(t, "}", "block-end", 3, 2, v032Element(t, "block-close", "}", "}", 1)))
		v032Run(t, result, "current", []v032Expected{{3, 11, 14, `Недопустимые символы: "/" (U+002F).`}}, 1)
	})
	t.Run("fragment brace not recognized", func(t *testing.T) {
		result := v032Document(t, v032Line(t, "@fragment a/b {", "invalid", 2, 1, v032Element(t, "tag", "@fragment", "fragment", 1), v032Unparsed(t, "a/b {", 11)))
		v032Run(t, result, "current", []v032Expected{{3, 11, 16, `Недопустимые символы: "/" (U+002F), " " (U+0020), "{" (U+007B).`}}, 1)
	})
	t.Run("missing fragment brace uses whole remainder", func(t *testing.T) {
		result := v032Document(t, v032Line(t, "@fragment a/b", "invalid", 2, 1, v032Element(t, "tag", "@fragment", "fragment", 1), v032Unparsed(t, "a/b", 11)))
		v032Run(t, result, "current", []v032Expected{{3, 11, 14, `Недопустимые символы: "/" (U+002F).`}}, 1)
	})
	// Каноническое имя тега и Raw аргумента доступны независимо от наличия
	// готового identifier; полностью нераспознанная строка не разбирается заново.
	t.Run("no recognized tag", func(t *testing.T) {
		result := v032Document(t, v032Line(t, "@task a/b", "invalid", 2, 1, v032Unparsed(t, "@task a/b", 1)), v032Text(t, "Read.", 2, 1))
		v032Run(t, result, "current", nil, 1)
	})
	for _, raw := range []string{"@task", "@task \t "} {
		t.Run("empty argument "+raw, func(t *testing.T) {
			result := v032Document(t, v032Line(t, raw, "tag", 2, 1, v032Element(t, "tag", "@task", "task", 1)), v032Text(t, "Read.", 3, 2))
			// Отсутствующий обязательный ID — отдельная ошибка парсера, вне V032.
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "missing-id", Source: "parser", DiagnosticCode: "fixture-missing-id", SeverityLevel: "error", DiagnosticScope: "line", Message: "Отсутствует обязательный ID.", Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 6}}, RelatedLocations: []model.Location{}})
			result.Document.HasErrors, result.Lines[2].HasErrors = true, true
			v032Run(t, result, "current", nil, 1)
		})
	}
}

func TestV032LiteralAndOtherContent(t *testing.T) {
	for _, tag := range []string{"editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			result := v032Document(t,
				v032Line(t, "@"+tag+" {", "block-start", 2, 1, v032Element(t, "tag", "@"+tag, tag, 1), v032Element(t, "block-open", "{", "{", len(tag)+3)),
				v032Text(t, "@task a/b", 3, 2), v032Text(t, "@document-id a/b", 3, 2), v032Text(t, "@include a/b", 3, 2),
				v032Line(t, "}", "block-end", 3, 2, v032Element(t, "block-close", "}", "}", 1)))
			v032Run(t, result, "current", nil, 1)
		})
	}
	result := v032Document(t,
		v032Line(t, "@editor @task a/b", "tag", 2, 1, v032Element(t, "tag", "@editor", "editor", 1), v032Element(t, "content", "@task a/b", "@task a/b", 9)),
		v032Line(t, `\@task a/b`, "content", 2, 1, v032Element(t, "content", `\@task a/b`, "@task a/b", 1)),
		v032Line(t, "@header a/b", "tag", 2, 1, v032Element(t, "tag", "@header", "header", 1), v032Element(t, "title", "a/b", "a/b", 9)),
		v032Line(t, "", "blank", 2, 1))
	v032Run(t, result, "current", nil, 1)
	// Новый запуск добавляет ссылки, сохраняя ссылки и историю предыдущего.
	v032Run(t, result, "second", nil, 1)
}

func TestV032NestedAndMultipleTasks(t *testing.T) {
	result := v032Document(t,
		v032Line(t, "@variants {", "block-start", 2, 1, v032Element(t, "tag", "@variants", "variants", 1), v032Element(t, "block-open", "{", "{", 11)),
		v032Line(t, "@variant A", "tag", 3, 2, v032Element(t, "tag", "@variant", "variant", 1), v032Element(t, "name", "A", "A", 10)),
		v032Line(t, "@task a/b", "tag", 4, 3, v032Element(t, "tag", "@task", "task", 1), v032Unparsed(t, "a/b", 7)), v032Text(t, "Read.", 5, 4),
		v032Line(t, "@variant B", "tag", 3, 2, v032Element(t, "tag", "@variant", "variant", 1), v032Element(t, "name", "B", "B", 10)),
		v032Line(t, "@task good", "tag", 7, 3, v032Element(t, "tag", "@task", "task", 1), v032Element(t, "identifier", "good", "good", 7)), v032Text(t, "Read again.", 8, 4),
		v032Line(t, "}", "block-end", 3, 2, v032Element(t, "block-close", "}", "}", 1)),
		v032Line(t, "@task c?d", "tag", 2, 1, v032Element(t, "tag", "@task", "task", 1), v032Unparsed(t, "c?d", 7)), v032Text(t, "Read.", 11, 2))
	v032Run(t, result, "current", []v032Expected{{5, 7, 10, `Недопустимые символы: "/" (U+002F).`}, {11, 7, 10, `Недопустимые символы: "?" (U+003F).`}}, 1)
}

func TestV032VersionsEndingsAndHistory(t *testing.T) {
	v032Run(t, v032UnavailableDocument(t, "1.2"), "current", nil, 0)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v032UnavailableDocument(t, version)
			before := v032Snapshot(t, result)
			checks, err := validator.Prepare(result, v032Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v032Snapshot(t, result) != before {
				t.Fatal("unsupported version changed model")
			}
		})
	}
	for _, ending := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF, model.LineEndingNone} {
		t.Run("ending "+string(ending), func(t *testing.T) {
			result := v032Document(t, v032Line(t, "@task task/a", "tag", 2, 1, v032Element(t, "tag", "@task", "task", 1), v032Unparsed(t, "task/a", 7)), v032Text(t, "Read.", 3, 2))
			if ending == model.LineEndingNone {
				result.Lines[3].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v032Run(t, result, "current", []v032Expected{{3, 7, 13, `Недопустимые символы: "/" (U+002F).`}}, 1)
		})
	}
	// Модель допускает распознанный identifier с исходным значением без
	// предварительной проверки алфавита: семантику ID CheckModel не проверяет.
	// Это позволяет проверить установку новых флагов error, а не только их сохранение.
	result := v032Document(t, v032Line(t, "@task  a/b", "tag", 2, 1, v032Element(t, "tag", "@task", "task", 1), v032Element(t, "identifier", "a/b", "a/b", 8)), v032Text(t, "Read.", 3, 2))
	want := []v032Expected{{3, 8, 11, `Недопустимые символы: "/" (U+002F).`}}
	v032Run(t, result, "first", want, 1)
	count := 0
	for _, d := range result.Diagnostics {
		if d.Source == "first" && d.DiagnosticCode == "V001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("V001 count = %d, want 1", count)
	}
	v032Run(t, result, "second", want, 1)
	v032Run(t, v032Document(t, v032Line(t, "@task Good", "tag", 2, 1, v032Element(t, "tag", "@task", "task", 1), v032Element(t, "identifier", "Good", "Good", 7)), v032Text(t, "Read.", 3, 2)), "first", nil, 1)
}

// v032Unparsed задаёт ошибочный исходный диапазон. Ссылку на историю добавляет
// v032ParserHistory после назначения физических номеров строк.
func v032Unparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v032Element(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

// v032ParserHistory создаёт согласованную историю только для явно заданных
// тестом unparsed. Код условный: каталог диагностик внешнего парсера недоступен.
func v032ParserHistory(t *testing.T, result *model.Result) {
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

// v032Registry создаёт реестр исключительно через штатный каталог регистраций.
func v032Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v032UnavailableDocument представляет отсутствие доступных строк
// при известной версии DSL, а не пустой исходник с выдуманным тегом версии.
func v032UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v032Document дополняет фрагменты технической строкой и заголовком по DSL 1.30.
// Содержимое фрагментов сохраняется; обычный текст образует шаг по DSL 2.1.15.
// Родители, вложенность, элементы и семантические значения задаются тестом.
func v032Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v032UnavailableDocument(t, "1.2")
	title, encoding := "Unit 1", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v032Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v032Element(t, "tag", "@dsl-version", "dsl-version", 1), v032Element(t, "version", "1.2", "1.2", 14)),
		v032Line(t, "# Unit 1", "heading", 0, 0,
			v032Element(t, "heading-level", "#", "1", 1), v032Element(t, "title", "Unit 1", "Unit 1", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v032NumberLines(t, result)
	v032ParserHistory(t, result)
	return result
}

// v032NumberLines назначает физические номера уже заданным строкам модели.
func v032NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v032Line создаёт строку с заданной структурой, не распознавая Raw.
func v032Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v032Element сохраняет явные Raw и Value; длина нужна только для входа.
// Ожидаемые диапазоны диагностики заданы отдельно и не вычисляются по Raw.
func v032Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v032Text создаёт строку содержимого без экранирования, с Value равным Raw.
func v032Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v032Line(t, raw, "content", parent, depth, v032Element(t, "content", raw, raw, 1))
}

// v032Run проверяет V032 текущего запуска через штатный конвейер.
// want содержит явные диапазоны с исключающим концом и конкретизации.
// Новые флаги error проверяются по явным ожиданиям; исходные данные и история сохраняются.
// wantV037 задаёт точное число сопутствующих ошибок отсутствующего ID.
// wantV068 содержит явные ожидания ошибок минимума этапов; отсутствие аргументов
// означает ноль V068. Флаги учитывают обе проверки, исходная история сохраняется.
func v032Run(t *testing.T, result *model.Result, processingID string, want []v032Expected, wantV037 int, wantV068 ...v032Expected) {
	t.Helper()
	before := v032Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	oldErrors := make([]bool, len(result.Lines))
	for i := range result.Lines {
		oldErrors[i] = result.Lines[i].HasErrors
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v032Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v032Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got, gotV068 []model.Diagnostic
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
		if entry.DiagnosticCode == "V032" && entry.Source == processingID {
			got = append(got, entry)
		}
		if entry.DiagnosticCode == "V068" && entry.Source == processingID {
			gotV068 = append(gotV068, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V032 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		if entry.DiagnosticCode != "V032" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.Message != "Идентификатор содержит недопустимые символы.\nконкретизация: "+expected.detail || entry.DiagnosticScope != "line" ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(gotV068) != len(wantV068) {
		t.Fatalf("current V068 count = %d, want %d; diagnostics: %+v", len(gotV068), len(wantV068), gotV068)
	}
	for i, expected := range wantV068 {
		entry := gotV068[i]
		location := model.Location{
			Start: model.Position{Line: expected.line, Column: expected.start},
			End:   model.Position{Line: expected.line, Column: expected.end},
		}
		if entry.Location == nil || *entry.Location != location ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "В задании меньше двух внутренних этапов @step.\nконкретизация: "+expected.detail ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("unexpected accompanying V068 %d: %+v; want location %+v and detail %q", i, entry, location, expected.detail)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost diagnostic history")
	}
	wantDocumentError := oldDocumentError || len(want) > 0 || wantV037 > 0 || len(wantV068) > 0
	if result.Document.HasErrors != wantDocumentError {
		t.Errorf("document error = %v, want %v", result.Document.HasErrors, wantDocumentError)
	}
	previous := *result
	previous.Document.HasErrors = oldDocumentError
	previous.Lines = append([]model.Line{}, result.Lines...)
	for i := range result.Lines {
		wantError := oldErrors[i]
		for _, expected := range want {
			if expected.line == i+1 {
				wantError = true
			}
		}
		for _, expected := range wantV068 {
			if expected.line == i+1 {
				wantError = true
			}
		}
		if result.Lines[i].HasErrors != wantError {
			t.Errorf("line %d error = %v, want %v", i+1, result.Lines[i].HasErrors, wantError)
		}
		previous.Lines[i].HasErrors = oldErrors[i]
	}
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	// В снимке исключаются новые ссылки других элементных диагностик каталога.
	newElementIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[len(previous.Diagnostics):] {
		if entry.DiagnosticScope == "element" && entry.DiagnosticCode != "V032" {
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
	if v032Snapshot(t, &previous) != before {
		t.Fatal("source, structure, error flags, element links or previous history changed")
	}
}

// v032Snapshot фиксирует всю модель для сравнения исходных данных и истории.
func v032Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV144RegistrationAndVersions(t *testing.T) {
	registry := v144TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	if registry.Len() != 198 || len(definitions) != 198 {
		t.Fatalf("catalog size = %d, DSL 1.2 definitions = %d, want 198", registry.Len(), len(definitions))
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V144" {
			continue
		}
		count++
		if definition.Message != "В @question несколько фрагментов с вопросительными знаками." ||
			definition.Severity != "warning" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V144 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v144TestDocument(t, v144TestTag(t, "question", "A? B?", "content", 3))
			result.Document.DSLVersion = &version
			before := v144TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v144TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v144TestExpected хранит явные исходные диапазоны с исключающим концом.
// Все связанные фрагменты находятся на той же физической строке.
type v144TestExpected struct {
	line, start, end int
	related          [][2]int
}

func TestV144ReferenceAndExample(t *testing.T) {
	// DSL 1.30: добавлены только версия, заголовок и шаг перед точными фрагментами.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v144TestExpected
	}{
		{"reference", "@question Where does Tom live?\n@question Where does he work?",
			[]model.Line{v144TestTag(t, "question", "Where does Tom live?", "content", 3),
				v144TestTag(t, "question", "Where does he work?", "content", 3)}, nil},
		{"example", "@question Where does Tom live? Where does he work?",
			[]model.Line{v144TestTag(t, "question", "Where does Tom live? Where does he work?", "content", 3)},
			[]v144TestExpected{{4, 11, 51, [][2]int{{11, 31}, {31, 51}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v144TestDocument(t, tc.body...)
			var source []string
			for _, line := range result.Lines[3:] {
				source = append(source, line.Raw)
			}
			if strings.Join(source, "\n") != tc.fragment {
				t.Fatal("control fragment changed")
			}
			v144TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV144PunctuationGroups(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []v144TestExpected
	}{
		{"user clarification", "? ?", []v144TestExpected{{4, 11, 14, [][2]int{{11, 12}, {12, 14}}}}},
		{"tab between groups", "?\t?", []v144TestExpected{{4, 11, 14, [][2]int{{11, 12}, {12, 14}}}}},
		{"NBSP between groups", "?\u00a0?", []v144TestExpected{{4, 11, 14, [][2]int{{11, 12}, {12, 14}}}}},
		{"one question mark", "?", nil},
		{"repeated question marks", "???", nil},
		{"mixed single group", "?!?", nil},
		{"mixed group after text", "A?!", nil},
		{"all three signs in one group", "A.!?..!?", nil},
		{"no words", "?? ??", []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}}},
		{"mixed groups", "A?! B??", []v144TestExpected{{4, 11, 18, [][2]int{{11, 14}, {14, 18}}}}},
		{"adjacent fragments", "A?B?", []v144TestExpected{{4, 11, 15, [][2]int{{11, 13}, {13, 15}}}}},
		{"ordinary spaces", "A? B?", []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}}},
		{"several spaces", "A?   B?", []v144TestExpected{{4, 11, 18, [][2]int{{11, 13}, {13, 18}}}}},
		{"three completions", "A? B? C?", []v144TestExpected{{4, 11, 19, [][2]int{{11, 13}, {13, 16}, {16, 19}}}}},
		{"period resets fragment", "No? Maybe. Yes?", []v144TestExpected{{4, 11, 26, [][2]int{{11, 14}, {21, 26}}}}},
		{"exclamation resets fragment", "No? Wait! Yes?", []v144TestExpected{{4, 11, 25, [][2]int{{11, 14}, {20, 25}}}}},
		{"question after period without letters", "A. ? B?", []v144TestExpected{{4, 11, 18, [][2]int{{13, 15}, {15, 18}}}}},
		{"question joined to period", "A.? B?", []v144TestExpected{{4, 11, 17, [][2]int{{11, 14}, {14, 17}}}}},
		{"unfinished tail is included only in primary", "A? B? tail", []v144TestExpected{{4, 11, 21, [][2]int{{11, 13}, {13, 16}}}}},
		{"plain text", "Text", nil},
		{"single completion and tail", "A? tail", nil},
		{"no question marks", "A. B!", nil},
		{"empty question", "", nil},
		{"Unicode text", "Я? ١?", []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}}},
		{"emoji alone also qualifies", "😀? 😀?", []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}}},
		{"fullwidth punctuation is not ASCII", "A？ B？", nil},
		{"Unicode ellipsis is not a delimiter", "A…? B?", []v144TestExpected{{4, 11, 17, [][2]int{{11, 14}, {14, 17}}}}},
		{"ASCII ellipsis resets fragment", "A? ... B?", []v144TestExpected{{4, 11, 20, [][2]int{{11, 13}, {17, 20}}}}},
		{"entity is not decoded", "A&#63; B&#63;", nil},
		{"comma is not a delimiter", "A?, B?", []v144TestExpected{{4, 11, 17, [][2]int{{11, 13}, {13, 17}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v144TestRun(t, v144TestDocument(t, v144TestTag(t, "question", tc.text, "content", 3)), "current", tc.want)
		})
	}
}

func TestV144SourceRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
		want []v144TestExpected
	}{
		{"tag case and indentation", v144TestLine(t, "  @QUESTION A? B?", "tag", 3,
			v144TestElement(t, "tag", "@QUESTION", "question", 3), v144TestElement(t, "content", "A? B?", "A? B?", 13)),
			[]v144TestExpected{{4, 13, 18, [][2]int{{13, 15}, {15, 18}}}}},
		{"content parts with source gap", v144TestLine(t, "@question A? B?", "tag", 3,
			v144TestElement(t, "tag", "@question", "question", 1), v144TestElement(t, "content", "A?", "A?", 11),
			v144TestElement(t, "content", "B?", "B?", 14)), []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}}},
		{"punctuation groups across content boundaries", v144TestLine(t, "@question A?! B??", "tag", 3,
			v144TestElement(t, "tag", "@question", "question", 1), v144TestElement(t, "content", "A?", "A?", 11),
			v144TestElement(t, "content", "! B?", "! B?", 13), v144TestElement(t, "content", "?", "?", 17)),
			[]v144TestExpected{{4, 11, 18, [][2]int{{11, 14}, {14, 18}}}}},
		{"single group split between elements", v144TestLine(t, "@question ???", "tag", 3,
			v144TestElement(t, "tag", "@question", "question", 1), v144TestElement(t, "content", "?", "?", 11),
			v144TestElement(t, "content", "??", "??", 12)), nil},
		{"raw escape coordinates", v144TestLine(t, `@question \{A?\} B?`, "tag", 3,
			v144TestElement(t, "tag", "@question", "question", 1), v144TestElement(t, "content", `\{A?\} B?`, "{A?} B?", 11)),
			[]v144TestExpected{{4, 11, 20, [][2]int{{11, 15}, {15, 20}}}}},
		{"source edges are not trimmed", v144TestLine(t, "@question  A? B? \t", "tag", 3,
			v144TestElement(t, "tag", "@question", "question", 1), v144TestElement(t, "content", " A? B? \t", "A? B?", 11)),
			[]v144TestExpected{{4, 11, 19, [][2]int{{11, 14}, {14, 17}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v144TestRun(t, v144TestDocument(t, tc.row), "current", tc.want) })
	}
}

func TestV144FullQuestionIncludesEdgePlaceholders(t *testing.T) {
	placeholder := func(start int) model.Element {
		t.Helper()
		element := v144TestElement(t, "placeholder", "_____", "", start)
		element.Value = nil
		return element
	}
	for _, tc := range []struct {
		name, raw string
		parts     []model.Element
		want      []v144TestExpected
	}{
		{"leading placeholder", "@question _____ A? B?",
			[]model.Element{placeholder(11), v144TestElement(t, "content", "A? B?", "A? B?", 17)},
			[]v144TestExpected{{4, 11, 22, [][2]int{{11, 19}, {19, 22}}}}},
		{"trailing placeholder", "@question A? B? _____",
			[]model.Element{v144TestElement(t, "content", "A? B?", "A? B?", 11), placeholder(17)},
			[]v144TestExpected{{4, 11, 22, [][2]int{{11, 13}, {13, 16}}}}},
		{"middle placeholder control", "@question A? _____ B?",
			[]model.Element{v144TestElement(t, "content", "A?", "A?", 11), placeholder(14),
				v144TestElement(t, "content", "B?", "B?", 20)},
			[]v144TestExpected{{4, 11, 22, [][2]int{{11, 13}, {13, 22}}}}},
		{"both edge placeholders", "@question _____ A? B? _____",
			[]model.Element{placeholder(11), v144TestElement(t, "content", "A? B?", "A? B?", 17), placeholder(23)},
			[]v144TestExpected{{4, 11, 28, [][2]int{{11, 19}, {19, 22}}}}},
		{"leading placeholder with one group", "@question _____ A?!?",
			[]model.Element{placeholder(11), v144TestElement(t, "content", "A?!?", "A?!?", 17)}, nil},
		{"trailing placeholder with one group", "@question A?!? _____",
			[]model.Element{v144TestElement(t, "content", "A?!?", "A?!?", 11), placeholder(16)}, nil},
		{"placeholder without punctuation", "@question _____", []model.Element{placeholder(11)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := append([]model.Element{v144TestElement(t, "tag", "@question", "question", 1)}, tc.parts...)
			result := v144TestDocument(t, v144TestLine(t, tc.raw, "tag", 3, parts...))
			v144TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV144ContextsAndExclusions(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		result := v144TestDocument(t, v144TestTag(t, "task", "t", "identifier", 3),
			v144TestTag(t, "question", "A? B?", "content", 4), v144TestTag(t, "endtask", "", "", 4))
		v144TestRun(t, result, "current", []v144TestExpected{{5, 11, 16, [][2]int{{11, 13}, {13, 16}}}})
	})
	t.Run("task stage", func(t *testing.T) {
		result := v144TestDocument(t, v144TestTag(t, "task", "t", "identifier", 3), v144TestTag(t, "step", "Practice", "title", 4),
			v144TestTag(t, "question", "A? B?", "content", 5), v144TestTag(t, "endtask", "", "", 4))
		v144TestRun(t, result, "current", []v144TestExpected{{6, 11, 16, [][2]int{{11, 13}, {13, 16}}}})
	})
	t.Run("variant", func(t *testing.T) {
		result := v144TestDocument(t, v144TestOpen(t, "variants", 3), v144TestTag(t, "variant", "A", "name", 4),
			v144TestTag(t, "question", "A? B?", "content", 5), v144TestTag(t, "variant", "B", "name", 4),
			v144TestText(t, "Read.", "Read.", 7), v144TestClose(t, 4))
		v144TestRun(t, result, "current", []v144TestExpected{{6, 11, 16, [][2]int{{11, 13}, {13, 16}}}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name+" inside task", func(t *testing.T) {
			result := v144TestDocument(t, v144TestTag(t, "task", "t", "identifier", 3), v144TestOpen(t, name, 4),
				v144TestText(t, "@question A? B?", "@question A? B?", 5), v144TestClose(t, 5),
				v144TestTag(t, "question", "A? B?", "content", 4), v144TestTag(t, "endtask", "", "", 4))
			v144TestRun(t, result, "current", []v144TestExpected{{8, 11, 16, [][2]int{{11, 13}, {13, 16}}}})
		})
	}
	t.Run("escaped tag and textual mention", func(t *testing.T) {
		result := v144TestDocument(t, v144TestText(t, `\@question A? B?`, "@question A? B?", 3),
			v144TestText(t, "See @question A? B?", "See @question A? B?", 3))
		v144TestRun(t, result, "current", nil)
	})
	t.Run("other tags", func(t *testing.T) {
		result := v144TestDocument(t, v144TestTag(t, "note", "A? B?", "content", 3),
			v144TestTag(t, "editor", "@question A? B?", "content", 3))
		v144TestRun(t, result, "current", nil)
	})
	t.Run("forbidden block question", func(t *testing.T) {
		result := v144TestDocument(t, v144TestOpen(t, "question", 3), v144TestText(t, "A? B?", "A? B?", 4), v144TestClose(t, 4))
		v144TestRun(t, result, "current", nil)
	})
}

func TestV144IndependentQuestionsAndHistory(t *testing.T) {
	t.Run("two violations", func(t *testing.T) {
		result := v144TestDocument(t, v144TestTag(t, "question", "A? B?", "content", 3), v144TestTag(t, "question", "A? B?", "content", 3))
		v144TestRun(t, result, "current", []v144TestExpected{
			{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}, {5, 11, 16, [][2]int{{11, 13}, {13, 16}}},
		})
	})
	t.Run("questions separated by block", func(t *testing.T) {
		result := v144TestDocument(t, v144TestTag(t, "question", "A?", "content", 3), v144TestOpen(t, "note", 3),
			v144TestText(t, "Read? Think?", "Read? Think?", 5), v144TestClose(t, 5), v144TestTag(t, "question", "B?", "content", 3))
		v144TestRun(t, result, "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+" final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v144TestDocument(t, v144TestTag(t, "question", "A? B?", "content", 3))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v144TestRun(t, result, "first", []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}})
				v144TestRun(t, v144TestDocument(t, v144TestTag(t, "question", "A?", "content", 3)), "other", nil)
				v144TestRun(t, result, "second", []v144TestExpected{{4, 11, 16, [][2]int{{11, 13}, {13, 16}}}})
			})
		}
	}
	t.Run("empty input", func(t *testing.T) {
		result := v144TestDocument(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v144TestRun(t, result, "current", nil)
	})
}

// v144TestElement создаёт исходный элемент; ожидаемые диапазоны заданы отдельно.
func v144TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v144TestLine задаёт тип и родителя явно, без повторного разбора DSL.
func v144TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v144TestTag задаёт известный тег и явный тип аргумента.
func v144TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v144TestElement(t, "tag", raw, name, 1)}
	if argument != "" {
		elements = append(elements, v144TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v144TestLine(t, raw, "tag", parent, elements...)
}

// v144TestOpen задаёт распознанное открытие блока без параметров.
func v144TestOpen(t *testing.T, name string, parent int) model.Line {
	t.Helper()
	return v144TestLine(t, "@"+name+" {", "block-start", parent,
		v144TestElement(t, "tag", "@"+name, name, 1), v144TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v144TestClose закрывает явно указанный блок.
func v144TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v144TestLine(t, "}", "block-end", parent, v144TestElement(t, "block-close", "}", "}", 1))
}

// v144TestText задаёт исходный и семантический текст независимо.
func v144TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v144TestLine(t, raw, "content", parent, v144TestElement(t, "content", raw, value, 1))
}

// v144TestDocument добавляет обязательное окружение и начальную запись парсера.
// Родители заданы вызывающим тестом; номера и глубина следуют этой модели.
func v144TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v144TestTag(t, "dsl-version", "1.2", "version", 0),
		v144TestLine(t, "# Lesson", "heading", 0, v144TestElement(t, "heading-level", "#", "1", 1),
			v144TestElement(t, "title", "Lesson", "Lesson", 3)),
		v144TestLine(t, "### Practice", "heading", 0, v144TestElement(t, "heading-level", "###", "3", 1),
			v144TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
	}
	return result
}

// v144TestRegistry использует полный штатный каталог.
func v144TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v144TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v144TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v144TestRun проверяет новые V144 после полного штатного конвейера.
// Основной и связанные диапазоны заданы явно и независимо от реализации.
func v144TestRun(t *testing.T, result *model.Result, processingID string, want []v144TestExpected) []model.Diagnostic {
	t.Helper()
	before := v144TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v144TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v144TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	added := result.Diagnostics[oldCount:]
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == "V144" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V144 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != "В @question несколько фрагментов с вопросительными знаками.\nконкретизация: Проверьте, не объединены ли самостоятельные вопросы. Если каждому требуется отдельный ответ, оформите их отдельными @question" ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.Source != processingID || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V144 occurrence %d = %+v, want location %+v, exact detail and related fragments", i, entry, location)
		}
		for j, span := range expected.related {
			wantRelated := model.Location{Start: model.Position{Line: expected.line, Column: span[0]},
				End: model.Position{Line: expected.line, Column: span[1]}}
			if entry.RelatedLocations[j] != wantRelated {
				t.Fatalf("related %d = %+v, want %+v", j, entry.RelatedLocations[j], wantRelated)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V144 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v144TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Исключаются только новые записи и производные флаги/ссылки.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element error links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v144TestSnapshot(t, &restored) != before {
		t.Fatal("source data or previous history changed")
	}
	return added
}

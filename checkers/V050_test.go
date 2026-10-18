package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV050Registration(t *testing.T) {
	definitions, err := v050TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V050" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "В заголовке использовано более трёх знаков #." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V050 definitions = %d, want 1", matches)
	}
}

func TestV050ReferenceAndExample(t *testing.T) {
	// Обязательная версия и заголовок юнита добавлены перед неизменёнными
	// двухстрочными фрагментами: их строки получают номера 3 и 4.
	t.Run("reference", func(t *testing.T) {
		result := v050TestDocument(t,
			v050TestLine(t, "### Practice", "heading", 0, 0,
				v050TestElement(t, "heading-level", "###", "3", 1, 4), v050TestElement(t, "title", "Practice", "Practice", 5, 13)),
			v050TestText(t, "Read the text.", "Read the text.", 3, 1))
		v050TestRun(t, result, "current", nil)
	})
	t.Run("example without heading-level", func(t *testing.T) {
		result := v050TestDocument(t, v050TestInvalid(t, "#### Practice", 2, 1),
			v050TestText(t, "Read the text.", "Read the text.", 2, 1))
		v050TestRun(t, result, "current", [][3]int{{3, 1, 5}})
	})
}

func TestV050InitialRun(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"four", "#### Practice", [][3]int{{3, 1, 5}}},
		{"five", "##### Practice", [][3]int{{3, 1, 6}}},
		{"eight form one occurrence", "######## Practice", [][3]int{{3, 1, 9}}},
		{"long marker stays whole", strings.Repeat("#", 64) + " Practice", [][3]int{{3, 1, 65}}},
		{"no title", "####", [][3]int{{3, 1, 5}}},
		{"no separator", "####Practice", [][3]int{{3, 1, 5}}},
		{"trailing spaces", "####  \t", [][3]int{{3, 1, 5}}},
		{"leading spaces and tab", "  \t#### Practice", [][3]int{{3, 4, 8}}},
		{"tab is one column", "\t#### Practice", [][3]int{{3, 2, 6}}},
		{"space stops run", "#### ####", [][3]int{{3, 1, 5}}},
		{"tab stops run", "####\t#####", [][3]int{{3, 1, 5}}},
		{"Unicode stops run", "####Я#####", [][3]int{{3, 1, 5}}},
		{"Unicode title", "#### Практика", [][3]int{{3, 1, 5}}},
		{"uppercase title", "#### PRACTICE", [][3]int{{3, 1, 5}}},
		{"supplementary and combining title", "#### 😀e\u0301", [][3]int{{3, 1, 5}}},
		// Эти строки ошибочны из-за отсутствия разделителя или названия,
		// но их начальный маркер не превышает порог V050.
		{"three without separator", "###Practice", nil},
		{"three without title", "###", nil},
		{"three then tab and another run", "###\t####", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v050TestRun(t, v050TestDocument(t, v050TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV050NoInitialMarker(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
	}{
		{"ordinary text", "Read the text.", "Read the text."},
		{"marker inside text", "Text ####", "Text ####"},
		{"nonbreaking space", "\u00a0#### Practice", "\u00a0#### Practice"},
		{"em space after ordinary indentation", " \t\u2003#### Practice", "\u2003#### Practice"},
		{"escaped marker", `\#### Practice`, "#### Practice"},
		{"escaped marker after indentation", "  \t\\#### Practice", "#### Practice"},
		{"literal backslash precedes marker", `\\#### Practice`, `\#### Practice`},
		{"fullwidth number signs", "＃＃＃＃ Practice", "＃＃＃＃ Practice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v050TestRun(t, v050TestDocument(t, v050TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", "  \t  "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v050TestRun(t, v050TestDocument(t, v050TestLine(t, raw, "blank", 2, 1)), "current", nil)
		})
	}
}

func TestV050SupportedHeadings(t *testing.T) {
	// Первый уровень уже задан обязательным заголовком юнита.
	v050TestRun(t, v050TestDocument(t), "current", nil)
	for _, tc := range []struct {
		raw, marker, level, title string
		markerEnd, titleStart     int
	}{
		{"## Practice", "##", "2", "Practice", 3, 4},
		{"### Practice", "###", "3", "Practice", 4, 5},
		{"### # Practice", "###", "3", "# Practice", 4, 5},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v050TestDocument(t,
				v050TestLine(t, tc.raw, "heading", 0, 0,
					v050TestElement(t, "heading-level", tc.marker, tc.level, 1, tc.markerEnd),
					v050TestElement(t, "title", tc.title, tc.title, tc.titleStart, utf8.RuneCountInString(tc.raw)+1)),
				v050TestText(t, "Read.", "Read.", 3, 1))
			if tc.level == "2" {
				result.Document.Metadata.Subtitle = &tc.title
			}
			v050TestRun(t, result, "current", nil)
		})
	}
}

func TestV050BlockContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"ordinary text block", []model.Line{
			v050TestOpen(t, "text", 2, 1), v050TestInvalid(t, "#### Practice", 3, 2), v050TestClose(t, 3, 2),
		}, [][3]int{{4, 1, 5}}},
		{"nested nonliteral block", []model.Line{
			v050TestOpen(t, "instr", 2, 1), v050TestText(t, "Read.", "Read.", 3, 2),
			v050TestOpen(t, "hint", 3, 2), v050TestInvalid(t, "#### Practice", 5, 3),
			v050TestClose(t, 5, 3), v050TestClose(t, 3, 2),
		}, [][3]int{{6, 1, 5}}},
		{"editor contents", []model.Line{
			v050TestOpen(t, "editor", 2, 1), v050TestText(t, "#### Practice", "#### Practice", 3, 2), v050TestClose(t, 3, 2),
		}, nil},
		{"uppercase editor tag", []model.Line{
			v050TestOpen(t, "EDITOR", 2, 1), v050TestText(t, "#### Practice", "#### Practice", 3, 2), v050TestClose(t, 3, 2),
		}, nil},
		{"multifill contents", []model.Line{
			v050TestOpen(t, "multifill", 2, 1), v050TestText(t, "#### Practice", "#### Practice", 3, 2), v050TestClose(t, 3, 2),
		}, nil},
		{"after literal block", []model.Line{
			v050TestOpen(t, "editor", 2, 1), v050TestText(t, "#### Practice", "#### Practice", 3, 2), v050TestClose(t, 3, 2),
			v050TestInvalid(t, "#### Practice", 2, 1),
		}, [][3]int{{6, 1, 5}}},
		{"two independent violations", []model.Line{
			v050TestInvalid(t, "#### A", 2, 1), v050TestLine(t, "", "blank", 2, 1), v050TestInvalid(t, "##### B", 2, 1),
		}, [][3]int{{3, 1, 5}, {5, 1, 6}}},
		{"single-line editor does not exclude following line", []model.Line{
			v050TestLine(t, "@editor #### Practice", "tag", 2, 1,
				v050TestElement(t, "tag", "@editor", "editor", 1, 8), v050TestElement(t, "content", "#### Practice", "#### Practice", 9, 22)),
			v050TestInvalid(t, "#### Practice", 2, 1),
		}, [][3]int{{4, 1, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v050TestRun(t, v050TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag+" inside task", func(t *testing.T) {
			// DSL 2.8.9 разрешает editor внутри задания; multifill следует
			// инструкции задания. EOF завершает область task по DSL 2.3.7.
			result := v050TestDocument(t,
				v050TestLine(t, "@task a", "tag", 2, 1,
					v050TestElement(t, "tag", "@task", "task", 1, 6), v050TestElement(t, "identifier", "a", "a", 7, 8)),
				v050TestLine(t, "@instr Read.", "tag", 3, 2,
					v050TestElement(t, "tag", "@instr", "instr", 1, 7), v050TestElement(t, "content", "Read.", "Read.", 8, 13)),
				v050TestOpen(t, tag, 3, 2), v050TestText(t, "#### Practice", "#### Practice", 5, 3), v050TestClose(t, 5, 3),
				v050TestInvalid(t, "#### Practice", 3, 2))
			v050TestRun(t, result, "current", [][3]int{{8, 1, 5}})
		})
	}
}

func TestV050EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v050TestDocument(t, v050TestInvalid(t, "#### Practice", 2, 1))
			result.Lines[2].LineEnding = ending
			v050TestRun(t, result, "first", [][3]int{{3, 1, 5}})
			v050TestRun(t, result, "second", [][3]int{{3, 1, 5}})
		})
	}
}

func TestV050UnavailableLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			// Версия известна из модели, физические строки недоступны.
			result := &model.Result{
				Format:     model.FormatVersion1,
				Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{},
			}
			if version == "1.2" {
				v050TestRun(t, result, "current", nil)
				return
			}
			before := v050TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v050TestRegistry(t))
			if err == nil || checks != nil || len(result.Diagnostics) != 0 || v050TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v, diagnostics=%+v", checks, err, result.Diagnostics)
			}
		})
	}
}

// v050TestInvalid задаёт неразобранную строку без heading-level;
// v050TestDocument добавит обязательную исходную ошибку для unparsed.
func v050TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v050TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v050TestLine(t, raw, "invalid", parent, depth, element)
}

// v050TestDocument добавляет обязательные версию и заголовок перед телом.
// Элементы и родители заданы тестом; Raw не разбирается. Для заданных unparsed
// создаются согласованные ошибки парсера, не являющиеся ожиданиями V050.
func v050TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Unit 1"
	lines := []model.Line{
		v050TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v050TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v050TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v050TestLine(t, "# Unit 1", "heading", 0, 0,
			v050TestElement(t, "heading-level", "#", "1", 1, 2), v050TestElement(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v050TestOpen задаёт согласованную строку открытия блока без параметров.
func v050TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v050TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v050TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v050TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v050TestClose задаёт закрывающую скобку с явным родителем.
func v050TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v050TestLine(t, "}", "block-end", parent, depth, v050TestElement(t, "block-close", "}", "}", 1, 2))
}

// v050TestText задаёт исходный текст и семантическое значение независимо.
func v050TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v050TestLine(t, raw, "content", parent, depth,
		v050TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v050TestLine сохраняет готовую структуру строки без классификации Raw.
func v050TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v050TestElement задаёт Unicode-колонки с исключающей правой границей.
func v050TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v050TestRegistry использует штатный каталог целиком.
func v050TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v050TestRun проверяет новые V050 по явным тройкам строка/начало/конец.
// Конец исключается; история, исходные данные и элементные ссылки сохраняются.
func v050TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v050TestSnapshot(t, result)
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v050TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v050TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(expected.Diagnostics):] {
		if entry.DiagnosticCode == "V050" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V050 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processingID ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "В заголовке использовано более трёх знаков #." || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V050 metadata", i, entry, location)
		}
	}
	v050AssertPreserved(t, before, result)
}

// v050TestSnapshot сохраняет модель для проверки отсутствия побочных изменений.
func v050TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v050AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V050 заданы явно выше.
func v050AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v050TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v050TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

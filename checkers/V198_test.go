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

const v198TestMessage = "У ветвей @variants совпадает всё содержимое."

// v198TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v198TestWant struct {
	location [4]int
	related  [][4]int
	detail   string
}

func TestV198RegistrationAndVersions(t *testing.T) {
	registry := v198TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V198" {
			continue
		}
		count++
		if definition.Message != v198TestMessage || definition.Severity != "warning" ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V198 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v198TestDocument(t)
			result.Document.DSLVersion = &version
			before := v198TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v198TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v198TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v198TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v198TestOpen(t, name, "", "", 2)}, body...)
	return v198TestDocument(t, append(lines, v198TestClose(t, 3))...)
}

// v198TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v198TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v198TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v198TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v198TestTag задаёт известное имя и явно указанный тип аргумента.
func v198TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v198TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v198TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v198TestLine(t, raw, "tag", parent, elements...)
}

// v198TestOpen добавляет открытие к известному объявлению блока.
func v198TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v198TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v198TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v198TestClose задаёт закрытие конкретного фигурного блока.
func v198TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v198TestLine(t, "}", "block-end", parent, v198TestElement(t, "block-close", "}", "}", 1))
}

// v198TestText сохраняет исходное и семантическое содержимое независимо.
func v198TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v198TestLine(t, raw, "content", parent, v198TestElement(t, "content", raw, value, 1))
}

// v198TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v198TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v198TestTag(t, "dsl-version", "1.2", "version", 0),
		v198TestLine(t, "# Lesson", "heading", 0, v198TestElement(t, "heading-level", "#", "1", 1),
			v198TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			line.Elements[j].ErrorIDs = append([]string{}, line.Elements[j].ErrorIDs...)
			if line.Elements[j].ElementType == "unparsed" {
				v198TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v198TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v198TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	// Все записи здесь принадлежат исходному парсеру. Контракт требует
	// порядка по позиции, даже если ошибки элементов добавлялись позднее.
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v198TestRegistry использует полный штатный каталог без отдельной регистрации.
func v198TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v198TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v198TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v198TestRun сравнивает новые V198 после полного Prepare/Run с явным ожиданием.
func v198TestRun(t *testing.T, result *model.Result, processingID string, want []v198TestWant) {
	t.Helper()
	before := v198TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v198TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v198TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V198" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V198 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := v198TestLocation(expected.location)
		related := make([]model.Location, 0, len(expected.related))
		for _, span := range expected.related {
			related = append(related, v198TestLocation(span))
		}
		if entry.Location == nil || *entry.Location != location || entry.Message != v198TestMessage+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" || !slices.Equal(entry.RelatedLocations, related) {
			t.Fatalf("V198 occurrence %d = %+v, location=%+v; want %+v", i, entry, entry.Location, expected)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("block-scoped V198 created an element error link")
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
	if err := json.Unmarshal([]byte(v198TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Убираются только добавленная история и производные флаги/ссылки.
	// Исходные строки, семантические значения и прежние диагностики сохраняются.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			newIDs := restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("prior element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v198TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

// v198TestLocation переводит явно заданное ожидание в контрактный тип.
func v198TestLocation(span [4]int) model.Location {
	return model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[2], Column: span[3]}}
}

// v198TestBody получает физическую строку объявления ветви для родителей входа.
type v198TestBody func(parent int) []model.Line

// v198TestPair добавляет только корректный контекст и две явно заданные ветви.
func v198TestPair(t *testing.T, a, b v198TestBody) *model.Result {
	t.Helper()
	lines := []model.Line{v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "A", "name", 3)}
	lines = append(lines, a(4)...)
	second := len(lines) + 3
	lines = append(lines, v198TestTag(t, "variant", "B", "name", 3))
	lines = append(lines, b(second)...)
	lines = append(lines, v198TestClose(t, 3))
	return v198TestDocument(t, lines...)
}

func v198TestParagraphs(t *testing.T, texts ...string) v198TestBody {
	t.Helper()
	return func(parent int) []model.Line {
		var lines []model.Line
		for _, text := range texts {
			lines = append(lines, v198TestText(t, text, text, parent))
		}
		return lines
	}
}

func v198TestOneTag(t *testing.T, name, value, kind string) v198TestBody {
	t.Helper()
	return func(parent int) []model.Line {
		return []model.Line{v198TestTag(t, name, value, model.ElementType(kind), parent)}
	}
}

func v198TestContentBlock(t *testing.T, name string, texts ...string) v198TestBody {
	t.Helper()
	return func(parent int) []model.Line {
		lines := []model.Line{v198TestOpen(t, name, "", "", parent)}
		for _, text := range texts {
			lines = append(lines, v198TestText(t, text, text, parent+1))
		}
		return append(lines, v198TestClose(t, parent+1))
	}
}

func TestV198ReferenceAndExample(t *testing.T) {
	// Две первые строки — обязательная версия и заголовок. Содержимое
	// табличных примеров, включая пробелы, остаётся посимвольно неизменным.
	cases := []struct {
		name, raw string
		lines     []model.Line
		want      []v198TestWant
	}{
		{"reference", "@variants {\n@variant Student A\n@instr Read the text.\nAnna lives in London.\n@variant Student B\n@instr Read the text.\nMark lives in Bristol.\n}",
			[]model.Line{v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "Student A", "name", 3),
				v198TestTag(t, "instr", "Read the text.", "content", 4), v198TestText(t, "Anna lives in London.", "Anna lives in London.", 4),
				v198TestTag(t, "variant", "Student B", "name", 3), v198TestTag(t, "instr", "Read the text.", "content", 7),
				v198TestText(t, "Mark lives in Bristol.", "Mark lives in Bristol.", 7), v198TestClose(t, 3)}, nil},
		{"example", "@variants {\n@variant Student A\nAnna lives in London.\n@variant Student B\nAnna lives in London.\n}",
			[]model.Line{v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "Student A", "name", 3),
				v198TestText(t, "Anna lives in London.", "Anna lives in London.", 4), v198TestTag(t, "variant", "Student B", "name", 3),
				v198TestText(t, "Anna lives in London.", "Anna lives in London.", 6), v198TestClose(t, 3)},
			[]v198TestWant{{[4]int{6, 1, 7, 22}, [][4]int{{4, 1, 5, 22}}, "Совпадают ветви: «Student A» (строка 4), «Student B» (строка 6)."}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.raw {
				t.Fatal("table example changed")
			}
			v198TestRun(t, v198TestDocument(t, tc.lines...), "current", tc.want)
		})
	}
}

func TestV198Groups(t *testing.T) {
	cases := []struct {
		name         string
		names, texts []string
		want         []v198TestWant
	}{
		{"below threshold", []string{"A"}, []string{"X"}, nil},
		{"two", []string{"A", "B"}, []string{"X", "X"}, []v198TestWant{{[4]int{6, 1, 7, 2}, [][4]int{{4, 1, 5, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}}},
		{"three", []string{"A", "B", "C"}, []string{"X", "X", "X"}, []v198TestWant{{[4]int{6, 1, 7, 2}, [][4]int{{4, 1, 5, 2}, {8, 1, 9, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6), «C» (строка 8)."}}},
		{"two groups", []string{"A", "B", "C", "D"}, []string{"X", "Y", "X", "Y"}, []v198TestWant{
			{[4]int{8, 1, 9, 2}, [][4]int{{4, 1, 5, 2}}, "Совпадают ветви: «A» (строка 4), «C» (строка 8)."},
			{[4]int{10, 1, 11, 2}, [][4]int{{6, 1, 7, 2}}, "Совпадают ветви: «B» (строка 6), «D» (строка 10)."}}},
		{"same names", []string{"A", "A"}, []string{"X", "X"}, []v198TestWant{{[4]int{6, 1, 7, 2}, [][4]int{{4, 1, 5, 2}}, "Совпадают ветви: «A» (строка 4), «A» (строка 6)."}}},
		{"different", []string{"A", "B"}, []string{"X", "Y"}, nil},
		{"none", nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := []model.Line{v198TestOpen(t, "variants", "", "", 2)}
			for i, name := range tc.names {
				branch := len(lines) + 3
				lines = append(lines, v198TestTag(t, "variant", name, "name", 3), v198TestText(t, tc.texts[i], tc.texts[i], branch))
			}
			lines = append(lines, v198TestClose(t, 3))
			v198TestRun(t, v198TestDocument(t, lines...), "current", tc.want)
		})
	}
}

func TestV198ExactValuesAndKinds(t *testing.T) {
	cases := []struct {
		name string
		a, b v198TestBody
		want []v198TestWant
	}{
		{"case of free text", v198TestParagraphs(t, "Read."), v198TestParagraphs(t, "read."), nil},
		{"punctuation", v198TestParagraphs(t, "Read."), v198TestParagraphs(t, "Read!"), nil},
		{"internal spaces", v198TestParagraphs(t, "A B"), v198TestParagraphs(t, "A  B"), nil},
		{"unicode spaces", v198TestParagraphs(t, "A B"), v198TestParagraphs(t, "A\u00a0B"), nil},
		{"unicode normalization forbidden", v198TestParagraphs(t, "é"), v198TestParagraphs(t, "e\u0301"), nil},
		{"order", v198TestParagraphs(t, "X", "Y"), v198TestParagraphs(t, "Y", "X"), nil},
		{"paragraph boundary", v198TestParagraphs(t, "X Y"), v198TestParagraphs(t, "X", "Y"), nil},
		{"different construct", v198TestParagraphs(t, "Read."), v198TestOneTag(t, "note", "Read.", "content"), nil},
		{"tag case", v198TestOneTag(t, "NOTE", "Read.", "content"), v198TestOneTag(t, "note", "Read.", "content"),
			[]v198TestWant{{[4]int{6, 1, 7, 12}, [][4]int{{4, 1, 5, 12}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}}},
		{"inline and block note", v198TestOneTag(t, "note", "Read.", "content"), v198TestContentBlock(t, "note", "Read."),
			[]v198TestWant{{[4]int{6, 1, 9, 2}, [][4]int{{4, 1, 5, 12}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { v198TestRun(t, v198TestPair(t, tc.a, tc.b), "current", tc.want) })
	}
	t.Run("processed values and source positions", func(t *testing.T) {
		a := func(parent int) []model.Line {
			return []model.Line{v198TestLine(t, " \t\\@X  ", "content", parent, v198TestElement(t, "content", `\@X`, "@X", 3))}
		}
		b := func(parent int) []model.Line { return []model.Line{v198TestText(t, `\@X`, "@X", parent)} }
		v198TestRun(t, v198TestPair(t, a, b), "current", []v198TestWant{{[4]int{6, 1, 7, 4}, [][4]int{{4, 1, 5, 8}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}})
	})
	t.Run("element segmentation is not content", func(t *testing.T) {
		a := func(parent int) []model.Line {
			return []model.Line{v198TestLine(t, "A B", "content", parent, v198TestElement(t, "content", "A", "A", 1), v198TestElement(t, "content", "B", "B", 3))}
		}
		v198TestRun(t, v198TestPair(t, a, v198TestParagraphs(t, "A B")), "current", []v198TestWant{{[4]int{6, 1, 7, 4}, [][4]int{{4, 1, 5, 4}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}})
	})
}

func TestV198IgnoredAndLiteralContent(t *testing.T) {
	t.Run("blank rows editor and source range", func(t *testing.T) {
		a := func(p int) []model.Line {
			return []model.Line{v198TestText(t, "X", "X", p), v198TestTag(t, "editor", "comment", "content", p), v198TestLine(t, "", "blank", p)}
		}
		b := func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "editor", "", "", p), v198TestText(t, "@variant fake", "@variant fake", p+1), v198TestClose(t, p+1), v198TestText(t, "X", "X", p), v198TestLine(t, " \t", "blank", p)}
		}
		v198TestRun(t, v198TestPair(t, a, b), "current", []v198TestWant{{[4]int{8, 1, 13, 3}, [][4]int{{4, 1, 7, 1}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}})
	})
	t.Run("only ignored content is empty", func(t *testing.T) {
		v198TestRun(t, v198TestPair(t, v198TestOneTag(t, "editor", "A", "content"), v198TestOneTag(t, "editor", "B", "content")), "current", nil)
	})
	t.Run("empty branches", func(t *testing.T) {
		v198TestRun(t, v198TestPair(t, v198TestParagraphs(t), v198TestParagraphs(t)), "current", nil)
	})
	t.Run("multifill blank rows matter", func(t *testing.T) {
		a := v198TestContentBlock(t, "multifill", "X")
		b := func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "multifill", "", "", p), v198TestText(t, "X", "X", p+1), v198TestLine(t, "", "blank", p+1), v198TestClose(t, p+1)}
		}
		v198TestRun(t, v198TestPair(t, a, b), "current", nil)
	})
	t.Run("literal variant is not a boundary", func(t *testing.T) {
		body := v198TestContentBlock(t, "multifill", "@variant fake")
		v198TestRun(t, v198TestPair(t, body, body), "current", []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}})
	})
}

func TestV198MultifillForbiddenAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		equal            bool
	}{
		{"answer content", "@answer Yes", "@answer Yes", false},
		{"answer without argument", "@answer", "@answer", false},
		{"case and margins", " \t@AnSwEr Yes \t", "@AnSwEr Yes", false},
		{"tab after name", "@answer\tYes", "@answer\tYes", false},
		{"opening brace after name", "@answer{", "@answer{", false},
		{"closing brace after name", "@answer}", "@answer}", false},
		{"opening brace before content", "@AnSwEr{Yes}", "@AnSwEr{Yes}", false},
		{"escaped answer", `\@answer Yes`, "@answer Yes", true},
		{"escaped answer and opening brace", `\@answer\{`, "@answer{", true},
		{"escaped answer and closing brace", `\@answer\}`, "@answer}", true},
		{"escaped answer with margins", " \t" + `\@answer Yes` + " \t", "@answer Yes", true},
		{"literal backslash before answer", `\\@answer Yes`, `\@answer Yes`, true},
		{"answer in middle", "Text @answer Yes", "Text @answer Yes", true},
		{"longer name", "@answers Yes", "@answers Yes", true},
		{"longer name with suffix", "@answerText Yes", "@answerText Yes", true},
		{"longer name before literal brace", `@answers\{`, "@answers{", true},
		{"unicode space is not a name boundary", "@answer\u00a0Yes", "@answer\u00a0Yes", true},
		{"other tag remains literal", "@hint Think", "@hint Think", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := func(p int) []model.Line {
				return []model.Line{v198TestOpen(t, "multifill", "", "", p),
					v198TestText(t, tc.raw, tc.value, p+1), v198TestClose(t, p+1)}
			}
			var want []v198TestWant
			if tc.equal {
				want = []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}}
			}
			v198TestRun(t, v198TestPair(t, body, body), "current", want)
		})
	}
	t.Run("one forbidden branch cannot match escaped text", func(t *testing.T) {
		escaped := func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "multifill", "", "", p),
				v198TestText(t, `\@answer Yes`, "@answer Yes", p+1), v198TestClose(t, p+1)}
		}
		v198TestRun(t, v198TestPair(t, v198TestContentBlock(t, "multifill", "@answer Yes"), escaped), "current", nil)
	})
	t.Run("forbidden branch does not cancel other matches", func(t *testing.T) {
		result := v198TestDocument(t, v198TestOpen(t, "variants", "", "", 2),
			v198TestTag(t, "variant", "A", "name", 3), v198TestOpen(t, "multifill", "", "", 4),
			v198TestText(t, "@answer Yes", "@answer Yes", 5), v198TestClose(t, 5),
			v198TestTag(t, "variant", "B", "name", 3), v198TestText(t, "X", "X", 8),
			v198TestTag(t, "variant", "C", "name", 3), v198TestText(t, "X", "X", 10), v198TestClose(t, 3))
		v198TestRun(t, result, "current", []v198TestWant{{[4]int{10, 1, 11, 2}, [][4]int{{8, 1, 9, 2}}, "Совпадают ветви: «B» (строка 8), «C» (строка 10)."}})
	})
}

func TestV198ContentFamilies(t *testing.T) {
	for _, name := range []string{"example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint", "multifill"} {
		t.Run(name, func(t *testing.T) {
			body := v198TestContentBlock(t, name, "X")
			v198TestRun(t, v198TestPair(t, body, body), "equal", []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}})
			v198TestRun(t, v198TestPair(t, body, v198TestContentBlock(t, name, "Y")), "different", nil)
		})
	}
	t.Run("block title", func(t *testing.T) {
		body := func(title string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestOpen(t, "note", title, "title", p), v198TestText(t, "X", "X", p+1), v198TestClose(t, p+1)}
			}
		}
		v198TestRun(t, v198TestPair(t, body("One"), body("Two")), "current", nil)
	})
	t.Run("nested instruction hint", func(t *testing.T) {
		body := func(hint string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestOpen(t, "instr", "", "", p), v198TestText(t, "Read.", "Read.", p+1), v198TestTag(t, "hint", hint, "content", p+1), v198TestClose(t, p+1)}
			}
		}
		v198TestRun(t, v198TestPair(t, body("X"), body("X")), "equal", []v198TestWant{{[4]int{9, 1, 13, 2}, [][4]int{{4, 1, 8, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 9)."}})
		v198TestRun(t, v198TestPair(t, body("X"), body("Y")), "different", nil)
	})
	t.Run("media source and mode", func(t *testing.T) {
		body := func(raw, value string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestLine(t, "@media audio "+raw, "tag", p, v198TestElement(t, "tag", "@media", "media", 1), v198TestElement(t, "media-type", "audio", "audio", 8), v198TestElement(t, "source", raw, value, 14))}
			}
		}
		v198TestRun(t, v198TestPair(t, body("x.mp3", "x.mp3"), body("\"x.mp3\"", "x.mp3")), "equal", []v198TestWant{{[4]int{6, 1, 7, 21}, [][4]int{{4, 1, 5, 19}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}})
		v198TestRun(t, v198TestPair(t, body("12", "12"), body("\"12\"", "12")), "numeric and path", nil)
		v198TestRun(t, v198TestPair(t, body("x.mp3", "x.mp3"), body("X.mp3", "X.mp3")), "path case", nil)
	})
	for _, tc := range []struct {
		name, value string
		end         int
	}{{"question", "X?", 13}, {"wordlist", "X; Y.", 16}, {"header", "Read.", 14}, {"multifill", "Read.", 17}} {
		t.Run("inline "+tc.name, func(t *testing.T) {
			kind := "content"
			if tc.name == "header" {
				kind = "title"
			}
			body := v198TestOneTag(t, tc.name, tc.value, kind)
			v198TestRun(t, v198TestPair(t, body, body), "current", []v198TestWant{{[4]int{6, 1, 7, tc.end}, [][4]int{{4, 1, 5, tc.end}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}})
		})
	}
}

func TestV198InteractiveContent(t *testing.T) {
	for _, name := range []string{"choice", "multichoice", "matching", "ordering"} {
		t.Run(name, func(t *testing.T) {
			body := func(p int) []model.Line {
				lines := []model.Line{v198TestOpen(t, name, "", "", p), v198TestText(t, "A", "A", p+1), v198TestText(t, "B", "B", p+1)}
				if name == "matching" {
					lines = append(lines, v198TestLine(t, "---", "separator", p+1, v198TestElement(t, "content", "---", "---", 1)), v198TestText(t, "X", "X", p+1), v198TestText(t, "Y", "Y", p+1))
				}
				return append(lines, v198TestClose(t, p+1))
			}
			want := []v198TestWant{{[4]int{9, 1, 13, 2}, [][4]int{{4, 1, 8, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 9)."}}
			if name == "matching" {
				want = []v198TestWant{{[4]int{12, 1, 19, 2}, [][4]int{{4, 1, 11, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 12)."}}
			}
			v198TestRun(t, v198TestPair(t, body, body), "equal", want)
			other := func(p int) []model.Line { lines := body(p); lines[1] = v198TestText(t, "C", "C", p+1); return lines }
			v198TestRun(t, v198TestPair(t, body, other), "different", nil)
		})
	}
	question := func(answer string) v198TestBody {
		return func(p int) []model.Line {
			return []model.Line{v198TestTag(t, "question", "X?", "content", p), v198TestTag(t, "answer", answer, "content", p), v198TestTag(t, "hint", "Think.", "content", p)}
		}
	}
	t.Run("answer and hint", func(t *testing.T) {
		v198TestRun(t, v198TestPair(t, question("Yes"), question("Yes")), "equal", []v198TestWant{{[4]int{8, 1, 11, 13}, [][4]int{{4, 1, 7, 13}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}})
		v198TestRun(t, v198TestPair(t, question("Yes"), question("yes")), "answer case preserved", nil)
	})
	t.Run("answer block alternatives", func(t *testing.T) {
		body := func(second string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestTag(t, "question", "X?", "content", p), v198TestOpen(t, "answer", "", "", p), v198TestText(t, "Y", "Y", p+2), v198TestText(t, second, second, p+2), v198TestClose(t, p+2)}
			}
		}
		v198TestRun(t, v198TestPair(t, body("Z"), body("Z")), "equal", []v198TestWant{{[4]int{10, 1, 15, 2}, [][4]int{{4, 1, 9, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 10)."}})
		v198TestRun(t, v198TestPair(t, body("Z"), body("W")), "different", nil)
	})
	t.Run("same displayed text different hint relation", func(t *testing.T) {
		other := func(p int) []model.Line {
			lines := question("Yes")(p)
			return []model.Line{lines[2], lines[0], lines[1]}
		}
		v198TestRun(t, v198TestPair(t, question("Yes"), other), "current", nil)
	})
	t.Run("placeholder alternatives all matter", func(t *testing.T) {
		body := func(raw string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestOpen(t, "multifill", "", "", p), v198TestLine(t, raw, "content", p+1, v198TestElement(t, "placeholder", raw, "Y", 1)), v198TestClose(t, p+1)}
			}
		}
		v198TestRun(t, v198TestPair(t, body("_____{Y}{Z}"), body("_____{Y}{Z}")), "equal", []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}})
		v198TestRun(t, v198TestPair(t, body("_____{Y}{Z}"), body("_____{Y}{W}")), "different alternative", nil)
	})
}

func TestV198TaskBoundaries(t *testing.T) {
	body := func(p int) []model.Line {
		return []model.Line{
			v198TestTag(t, "task", "t", "identifier", p), v198TestTag(t, "speaking", "", "", p+1),
			v198TestTag(t, "step", "Read", "title", p+1), v198TestText(t, "X", "X", p+3),
			v198TestOpen(t, "key", "", "", p+3), v198TestText(t, "Y", "Y", p+5), v198TestClose(t, p+5)}
	}
	t.Run("logical task step speaking and key", func(t *testing.T) {
		v198TestRun(t, v198TestPair(t, body, body), "current", []v198TestWant{{[4]int{12, 1, 19, 2}, [][4]int{{4, 1, 11, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 12)."}})
	})
	t.Run("displayed stage title", func(t *testing.T) {
		other := func(p int) []model.Line {
			lines := body(p)
			lines[2] = v198TestTag(t, "step", "Write", "title", p+1)
			return lines
		}
		v198TestRun(t, v198TestPair(t, body, other), "current", nil)
	})
	t.Run("task ending changes ownership", func(t *testing.T) {
		a := func(p int) []model.Line {
			return []model.Line{v198TestTag(t, "task", "t", "identifier", p), v198TestText(t, "X", "X", p+1), v198TestTag(t, "endtask", "", "", p), v198TestText(t, "Y", "Y", p)}
		}
		b := func(p int) []model.Line {
			return []model.Line{v198TestTag(t, "task", "t", "identifier", p), v198TestText(t, "X", "X", p+1), v198TestText(t, "Y", "Y", p+1)}
		}
		v198TestRun(t, v198TestPair(t, a, b), "current", nil)
	})
}

// v198TestDefinitions добавляет явно заданные определения после готового блока.
func v198TestDefinitions(t *testing.T, result *model.Result, definitions ...model.Line) *model.Result {
	t.Helper()
	return v198TestDocument(t, append(result.Lines[2:], definitions...)...)
}

func TestV198Includes(t *testing.T) {
	for _, tc := range []struct {
		name, idA, idB, valueA, valueB string
		want                           []v198TestWant
	}{
		{"different IDs same body", "f", "g", "X", "X", []v198TestWant{{[4]int{6, 1, 7, 11}, [][4]int{{4, 1, 5, 11}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}}},
		{"different body", "f", "g", "X", "Y", nil},
		{"ID case", "F", "f", "X", "Y", []v198TestWant{{[4]int{6, 1, 7, 11}, [][4]int{{4, 1, 5, 11}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}}},
		{"undefined", "missing", "g", "X", "Y", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v198TestPair(t, v198TestOneTag(t, "include", tc.idA, "identifier"), v198TestOneTag(t, "include", tc.idB, "identifier"))
			result = v198TestDefinitions(t, result, v198TestOpen(t, "fragment", "f", "identifier", 2), v198TestText(t, tc.valueA, tc.valueA, 9), v198TestClose(t, 9), v198TestOpen(t, "fragment", "g", "identifier", 2), v198TestText(t, tc.valueB, tc.valueB, 12), v198TestClose(t, 12))
			v198TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("inline material equals inserted material", func(t *testing.T) {
		result := v198TestPair(t, v198TestOneTag(t, "include", "f", "identifier"), v198TestParagraphs(t, "X"))
		result = v198TestDefinitions(t, result, v198TestOpen(t, "fragment", "f", "identifier", 2), v198TestText(t, "X", "X", 9), v198TestClose(t, 9))
		v198TestRun(t, result, "current", []v198TestWant{{[4]int{6, 1, 7, 2}, [][4]int{{4, 1, 5, 11}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}})
	})
	t.Run("ambiguous definition", func(t *testing.T) {
		result := v198TestPair(t, v198TestOneTag(t, "include", "f", "identifier"), v198TestOneTag(t, "include", "F", "identifier"))
		result = v198TestDefinitions(t, result, v198TestOpen(t, "fragment", "f", "identifier", 2), v198TestText(t, "X", "X", 9), v198TestClose(t, 9), v198TestOpen(t, "fragment", "F", "identifier", 2), v198TestText(t, "X", "X", 12), v198TestClose(t, 12))
		v198TestRun(t, result, "current", nil)
	})
	t.Run("unused definitions do not display", func(t *testing.T) {
		body := func(id, value string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestOpen(t, "fragment", id, "identifier", p), v198TestText(t, value, value, p+1), v198TestClose(t, p+1), v198TestText(t, "X", "X", p)}
			}
		}
		v198TestRun(t, v198TestPair(t, body("f", "Y"), body("g", "Z")), "current", []v198TestWant{{[4]int{9, 1, 13, 2}, [][4]int{{4, 1, 8, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 9)."}})
	})
	t.Run("include cannot bind script to external media", func(t *testing.T) {
		media := func(p int) model.Line {
			return v198TestLine(t, "@media audio x.mp3", "tag", p, v198TestElement(t, "tag", "@media", "media", 1), v198TestElement(t, "media-type", "audio", "audio", 8), v198TestElement(t, "source", "x.mp3", "x.mp3", 14))
		}
		a := func(p int) []model.Line {
			return []model.Line{media(p), v198TestTag(t, "include", "f", "identifier", p)}
		}
		result := v198TestPair(t, a, v198TestOneTag(t, "include", "g", "identifier"))
		// f содержит только script; g — media и связанный с ним script.
		result = v198TestDefinitions(t, result, v198TestOpen(t, "fragment", "f", "identifier", 2), v198TestOpen(t, "script", "", "", 10), v198TestText(t, "X", "X", 11), v198TestClose(t, 11), v198TestClose(t, 10),
			v198TestOpen(t, "fragment", "g", "identifier", 2), media(15), v198TestOpen(t, "script", "", "", 15), v198TestText(t, "X", "X", 17), v198TestClose(t, 17), v198TestClose(t, 15))
		v198TestRun(t, result, "current", nil)
	})
	for _, name := range []string{"include", "question", "editor", "variants"} {
		t.Run("forbidden in fragment "+name, func(t *testing.T) {
			result := v198TestPair(t, v198TestOneTag(t, "include", "f", "identifier"), v198TestOneTag(t, "include", "f", "identifier"))
			kind := model.ElementTypeContent
			if name == "include" {
				kind = "identifier"
			}
			result = v198TestDefinitions(t, result, v198TestOpen(t, "fragment", "f", "identifier", 2), v198TestTag(t, name, "f", kind, 9), v198TestClose(t, 9))
			v198TestRun(t, result, "current", nil)
		})
	}
}

func TestV198StructuralText(t *testing.T) {
	for _, tc := range []struct {
		name, a, b, valueA, valueB string
		equal                      bool
	}{
		{"formatting case", "<strong>X</strong>", "<STRONG>X</STRONG>", "<strong>X</strong>", "<STRONG>X</STRONG>", true},
		{"formatting changes meaning", "<strong>X</strong>", "<em>X</em>", "<strong>X</strong>", "<em>X</em>", false},
		{"literal is not HTML", `\<strong\>X\</strong\>`, "<strong>X</strong>", "<strong>X</strong>", "<strong>X</strong>", false},
		{"visual break", "<br>", "<BR>", "<br>", "<BR>", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := func(raw, value string) v198TestBody {
				return func(p int) []model.Line {
					return []model.Line{v198TestOpen(t, "text", "", "", p), v198TestText(t, raw, value, p+1), v198TestClose(t, p+1)}
				}
			}
			var want []v198TestWant
			if tc.equal {
				want = []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}}
			}
			v198TestRun(t, v198TestPair(t, body(tc.a, tc.valueA), body(tc.b, tc.valueB)), "current", want)
		})
	}
	t.Run("table escaped separator", func(t *testing.T) {
		body := func(raw string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestOpen(t, "table", "", "", p), v198TestText(t, raw, "X | Y", p+1), v198TestClose(t, p+1)}
			}
		}
		v198TestRun(t, v198TestPair(t, body(`X \| Y`), body("X | Y")), "current", nil)
	})
	t.Run("inline wordlist escaped separator", func(t *testing.T) {
		body := func(raw string) v198TestBody {
			return func(p int) []model.Line {
				return []model.Line{v198TestLine(t, "@wordlist "+raw, "tag", p, v198TestElement(t, "tag", "@wordlist", "wordlist", 1), v198TestElement(t, "content", raw, "X; Y.", 11))}
			}
		}
		v198TestRun(t, v198TestPair(t, body(`X\; Y.`), body("X; Y.")), "current", nil)
	})
	t.Run("escaped separator is example content", func(t *testing.T) {
		body := func(separator bool) v198TestBody {
			return func(p int) []model.Line {
				middle := v198TestText(t, `\---`, "---", p+1)
				if separator {
					middle = v198TestLine(t, "---", "separator", p+1, v198TestElement(t, "content", "---", "---", 1))
				}
				return []model.Line{v198TestOpen(t, "example", "", "", p), v198TestText(t, "X", "X", p+1), middle, v198TestText(t, "Y", "Y", p+1), v198TestClose(t, p+1)}
			}
		}
		v198TestRun(t, v198TestPair(t, body(false), body(true)), "current", nil)
	})
}

func TestV198ContextualEscapes(t *testing.T) {
	for _, tc := range []struct {
		name, a, b, valueA, valueB string
		equal                      bool
	}{
		{"middle at and hash remain different", `x\@<em>Y</em>`, `x\#<em>Y</em>`, `x\@<em>Y</em>`, `x\#<em>Y</em>`, false},
		{"Unicode before unknown escapes", `Ж\@<em>Ё</em>`, `Ж\#<em>Ё</em>`, `Ж\@<em>Ё</em>`, `Ж\#<em>Ё</em>`, false},
		{"single underscore remains different", `x\_<em>Y</em>`, `x\@<em>Y</em>`, `x\_<em>Y</em>`, `x\@<em>Y</em>`, false},
		{"same middle at escape", `x\@<em>Y</em>`, `x\\@<EM>Y</EM>`, `x\@<em>Y</em>`, `x\@<EM>Y</EM>`, true},
		{"same middle hash escape", `x\#<em>Y</em>`, `x\\#<EM>Y</EM>`, `x\#<em>Y</em>`, `x\#<EM>Y</EM>`, true},
		{"same unknown letter escape", `x\q<em>Y</em>`, `x\\q<EM>Y</EM>`, `x\q<em>Y</em>`, `x\q<EM>Y</EM>`, true},
		{"same single underscore escape", `x\_<em>Y</em>`, `x\\_<EM>Y</EM>`, `x\_<em>Y</em>`, `x\_<EM>Y</EM>`, true},
		{"same unknown quote escape", `x\"<em>Y</em>`, `x\\"<EM>Y</EM>`, `x\"<em>Y</em>`, `x\"<EM>Y</EM>`, true},
		{"semicolon is literal in text", `x\;<em>Y</em>`, `x\\;<EM>Y</EM>`, `x\;<em>Y</em>`, `x\;<EM>Y</EM>`, true},
		{"separator is literal in text", `\---<em>Y</em>`, `\\---<EM>Y</EM>`, `\---<em>Y</em>`, `\---<EM>Y</EM>`, true},
		{"initial at escape", `\@<em>Y</em>`, `\@<EM>Y</EM>`, `@<em>Y</em>`, `@<EM>Y</EM>`, true},
		{"initial hash escape", `\#<em>Y</em>`, `\#<EM>Y</EM>`, `#<em>Y</em>`, `#<EM>Y</EM>`, true},
		{"double slash does not escape initial at", `\@<em>Y</em>`, `\\@<em>Y</em>`, `@<em>Y</em>`, `\@<em>Y</em>`, false},
		{"double slash before HTML", `Ж\\<em>Ё</em>`, `Ж\\<EM>Ё</EM>`, `Ж\<em>Ё</em>`, `Ж\<EM>Ё</EM>`, true},
		{"triple slash keeps HTML literal", `x\\\<em\>Y\</em\>`, `x\\<em>Y</em>`, `x\<em>Y</em>`, `x\<em>Y</em>`, false},
		{"escaped braces before HTML", `\{Ж\}<em>Ё</em>`, `\{Ж\}<EM>Ё</EM>`, `{Ж}<em>Ё</em>`, `{Ж}<EM>Ё</EM>`, true},
		{"escaped two underscores", `x\__<em>Y</em>`, `x\__<EM>Y</EM>`, `x__<em>Y</em>`, `x__<EM>Y</EM>`, true},
		{"escaped placeholder sequence", `x\_____<em>Y</em>`, `x\_____<EM>Y</EM>`, `x_____<em>Y</em>`, `x_____<EM>Y</EM>`, true},
		{"escaped long underscore sequence", `x\______<em>Y</em>`, `x\______<EM>Y</EM>`, `x______<em>Y</em>`, `x______<EM>Y</EM>`, true},
		{"literal tag case stays significant", `\<em\>Y\</em\>`, `\<EM\>Y\</EM\>`, `<em>Y</em>`, `<EM>Y</EM>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := func(raw, value string) v198TestBody {
				return func(p int) []model.Line {
					return []model.Line{v198TestOpen(t, "text", "", "", p), v198TestText(t, raw, value, p+1), v198TestClose(t, p+1)}
				}
			}
			var want []v198TestWant
			if tc.equal {
				want = []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}}
			}
			v198TestRun(t, v198TestPair(t, body(tc.a, tc.valueA), body(tc.b, tc.valueB)), "current", want)
		})
	}
	t.Run("content element does not restart the line", func(t *testing.T) {
		body := func(suffix string) v198TestBody {
			return func(p int) []model.Line {
				line := v198TestLine(t, "Ж"+suffix, "content", p+1,
					v198TestElement(t, "content", "Ж", "Ж", 1), v198TestElement(t, "content", suffix, suffix, 2))
				return []model.Line{v198TestOpen(t, "text", "", "", p), line, v198TestClose(t, p+1)}
			}
		}
		v198TestRun(t, v198TestPair(t, body(`\@<em>Ё</em>`), body(`\#<em>Ё</em>`)), "current", nil)
	})
	t.Run("leading whitespace precedes initial escape", func(t *testing.T) {
		body := func(indented bool) v198TestBody {
			return func(p int) []model.Line {
				line := v198TestText(t, `\@<em>Ё</em>`, `@<em>Ё</em>`, p+1)
				if indented {
					line = v198TestLine(t, " \t"+`\@<EM>Ё</EM>`+" \t", "content", p+1,
						v198TestElement(t, "content", `\@<EM>Ё</EM>`, `@<EM>Ё</EM>`, 3))
				}
				return []model.Line{v198TestOpen(t, "text", "", "", p), line, v198TestClose(t, p+1)}
			}
		}
		want := []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}}
		v198TestRun(t, v198TestPair(t, body(false), body(true)), "current", want)
	})
}

func TestV198EscapesInTableAndWordlist(t *testing.T) {
	for _, tc := range []struct {
		name, context, a, b, valueA, valueB string
		equal                               bool
	}{
		{"middle at", "table", `x\@ | Y`, `x\\@ | Y`, `x\@ | Y`, `x\@ | Y`, true},
		{"middle hash", "table", `x\# | Y`, `x\\# | Y`, `x\# | Y`, `x\# | Y`, true},
		{"single underscore", "table", `x\_ | Y`, `x\\_ | Y`, `x\_ | Y`, `x\_ | Y`, true},
		{"literal opening angle inside cell", "table", `x\<z | Y`, `x\\<z | Y`, `x\<z | Y`, `x\<z | Y`, true},
		{"literal closing angle inside cell", "table", `x\>z | Y`, `x\\>z | Y`, `x\>z | Y`, `x\>z | Y`, true},
		{"literal boundary angles", "table", `\<x\> | Y`, `\<x\> | Y`, `<x> | Y`, `<x> | Y`, true},
		{"literal angles in split cell", "table", `<x \< y | z \> t>`, `<x \< y | z \> t>`, `<x < y | z > t>`, `<x < y | z > t>`, true},
		{"escaped pipe", "table", `X \| Y`, `X | Y`, `X | Y`, `X | Y`, false},
		{"initial at", "table", `\@X | Y`, `\\@X | Y`, `@X | Y`, `\@X | Y`, false},
		{"argument is not line start at", "wordlist", `\@X; Y.`, `\\@X; Y.`, `\@X; Y.`, `\@X; Y.`, true},
		{"argument is not line start hash", "wordlist", `\#X; Y.`, `\\#X; Y.`, `\#X; Y.`, `\#X; Y.`, true},
		{"single underscore", "wordlist", `\_X; Y.`, `\\_X; Y.`, `\_X; Y.`, `\_X; Y.`, true},
		{"escaped underscore sequence", "wordlist", `\__X; Y.`, `\__X; Y.`, `__X; Y.`, `__X; Y.`, true},
		{"angles are literal", "wordlist", `\<X; Y.`, `\\<X; Y.`, `\<X; Y.`, `\<X; Y.`, true},
		{"unknown quote", "wordlist", `\"X; Y.`, `\\"X; Y.`, `\"X; Y.`, `\"X; Y.`, true},
		{"escaped semicolon", "wordlist", `X\; Y.`, `X; Y.`, `X; Y.`, `X; Y.`, false},
	} {
		t.Run(tc.context+"/"+tc.name, func(t *testing.T) {
			body := func(raw, value string) v198TestBody {
				return func(p int) []model.Line {
					if tc.context == "table" {
						return []model.Line{v198TestOpen(t, "table", "", "", p), v198TestText(t, raw, value, p+1), v198TestClose(t, p+1)}
					}
					return []model.Line{v198TestLine(t, "@wordlist "+raw, "tag", p,
						v198TestElement(t, "tag", "@wordlist", "wordlist", 1), v198TestElement(t, "content", raw, value, 11)),
						v198TestText(t, "Tail", "Tail", p)}
				}
			}
			var want []v198TestWant
			if tc.equal {
				want = []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}}
				if tc.context == "wordlist" {
					want = []v198TestWant{{[4]int{7, 1, 9, 5}, [][4]int{{4, 1, 6, 5}}, "Совпадают ветви: «A» (строка 4), «B» (строка 7)."}}
				}
			}
			v198TestRun(t, v198TestPair(t, body(tc.a, tc.valueA), body(tc.b, tc.valueB)), "current", want)
		})
	}
}

func TestV198HTMLAcrossTextElements(t *testing.T) {
	placeholder := func(column int) model.Element {
		element := v198TestElement(t, "placeholder", "_____", "", column)
		element.Value = nil
		return element
	}
	body := func(raw string, elements ...model.Element) v198TestBody {
		return func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "text", "", "", p),
				v198TestLine(t, raw, "content", p+1, elements...), v198TestClose(t, p+1)}
		}
	}
	strong := body("<strong>_____</strong>",
		v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(9),
		v198TestElement(t, "content", "</strong>", "</strong>", 14))
	nested := body("<strong><em>Ж _____</em></strong>",
		v198TestElement(t, "content", "<strong><em>Ж", "<strong><em>Ж", 1), placeholder(15),
		v198TestElement(t, "content", "</em></strong>", "</em></strong>", 20))
	spaced := body("<strong> _____ </strong>",
		v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(10),
		v198TestElement(t, "content", "</strong>", "</strong>", 16))
	unparsed := body("<strong>_____ {</strong>",
		v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(9),
		v198TestElement(t, "unparsed", "{", "", 15),
		v198TestElement(t, "content", "</strong>", "</strong>", 16))
	for _, tc := range []struct {
		name  string
		a, b  v198TestBody
		equal bool
	}{
		{"placeholder with null value", strong, strong, true},
		{"tag case around placeholder", strong, body("<STRONG>_____</STRONG>",
			v198TestElement(t, "content", "<STRONG>", "<STRONG>", 1), placeholder(9),
			v198TestElement(t, "content", "</STRONG>", "</STRONG>", 14)), true},
		{"nested formatting and significant gap", nested, body("<STRONG><EM>Ж _____</EM></STRONG>",
			v198TestElement(t, "content", "<STRONG><EM>Ж", "<STRONG><EM>Ж", 1), placeholder(15),
			v198TestElement(t, "content", "</EM></STRONG>", "</EM></STRONG>", 20)), true},
		{"several placeholders", body("<strong>_____ и _____</strong>",
			v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(9),
			v198TestElement(t, "content", " и ", " и ", 14), placeholder(17),
			v198TestElement(t, "content", "</strong>", "</strong>", 22)), body("<STRONG>_____ и _____</STRONG>",
			v198TestElement(t, "content", "<STRONG>", "<STRONG>", 1), placeholder(9),
			v198TestElement(t, "content", "и", "и", 15), placeholder(17),
			v198TestElement(t, "content", "</STRONG>", "</STRONG>", 22)), true},
		{"gaps and content spaces agree", spaced, body("<strong> _____ </strong>",
			v198TestElement(t, "content", "<strong> ", "<strong> ", 1), placeholder(10),
			v198TestElement(t, "content", " </strong>", " </strong>", 15)), true},
		{"element raw includes line margins", strong, body(" \t<STRONG>_____</STRONG> \t",
			v198TestElement(t, "content", " \t<STRONG>", "<STRONG>", 1), placeholder(11),
			v198TestElement(t, "content", "</STRONG> \t", "</STRONG>", 16)), true},
		{"single content raw includes line margins", v198TestContentBlock(t, "text", "<strong>Ж</strong>"),
			body(" \t<STRONG>Ж</STRONG> \t",
				v198TestElement(t, "content", " \t<STRONG>Ж</STRONG> \t", "<STRONG>Ж</STRONG>", 1)), true},
		{"line margin before initial escape", body(`\@<strong>_____</strong>`,
			v198TestElement(t, "content", `\@<strong>`, "@<strong>", 1), placeholder(11),
			v198TestElement(t, "content", "</strong>", "</strong>", 16)),
			body(" \t"+`\@<STRONG>_____</STRONG>`+" \t",
				v198TestElement(t, "content", " \t"+`\@<STRONG>`, "@<STRONG>", 1), placeholder(13),
				v198TestElement(t, "content", "</STRONG> \t", "</STRONG>", 18)), true},
		{"one content and several content elements", v198TestContentBlock(t, "text", "<strong>Ж</strong>"),
			body("<STRONG>Ж</STRONG>", v198TestElement(t, "content", "<STRONG>", "<STRONG>", 1),
				v198TestElement(t, "content", "Ж", "Ж", 9), v198TestElement(t, "content", "</STRONG>", "</STRONG>", 10)), true},
		{"tag split between content elements", strong, body("<STRONG>_____</STRONG>",
			v198TestElement(t, "content", "<STR", "<STR", 1), v198TestElement(t, "content", "ONG>", "ONG>", 5), placeholder(9),
			v198TestElement(t, "content", "</STR", "</STR", 14), v198TestElement(t, "content", "ONG>", "ONG>", 19)), true},
		{"literal placeholder remains content", strong, body(`<strong>\_____</strong>`,
			v198TestElement(t, "content", `<strong>\_____</strong>`, "<strong>_____</strong>", 1)), false},
		{"different free text case", nested, body("<strong><em>ж _____</em></strong>",
			v198TestElement(t, "content", "<strong><em>ж", "<strong><em>ж", 1), placeholder(15),
			v198TestElement(t, "content", "</em></strong>", "</em></strong>", 20)), false},
		{"different material", strong, body("<strong>X_____</strong>",
			v198TestElement(t, "content", "<strong>X", "<strong>X", 1), placeholder(10),
			v198TestElement(t, "content", "</strong>", "</strong>", 15)), false},
		{"different inner gap", spaced, body("<strong>  _____ </strong>",
			v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(11),
			v198TestElement(t, "content", "</strong>", "</strong>", 17)), false},
		{"unparsed brace excludes formatted branches", unparsed, unparsed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []v198TestWant
			if tc.equal {
				want = []v198TestWant{{[4]int{8, 1, 11, 2}, [][4]int{{4, 1, 7, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 8)."}}
			}
			v198TestRun(t, v198TestPair(t, tc.a, tc.b), "current", want)
		})
	}
	for _, tc := range []struct {
		name string
		body v198TestBody
	}{
		{"crossed closures", body("<strong><em>_____</strong></em>",
			v198TestElement(t, "content", "<strong><em>", "<strong><em>", 1), placeholder(13),
			v198TestElement(t, "content", "</strong></em>", "</strong></em>", 18))},
		{"missing closure", body("<strong>_____", v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(9))},
		{"missing opening", body("_____</strong>", placeholder(1), v198TestElement(t, "content", "</strong>", "</strong>", 6))},
		{"break next to placeholder", body("<br>_____", v198TestElement(t, "content", "<br>", "<br>", 1), placeholder(5))},
		{"formatting crosses paragraph boundary", func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "text", "", "", p),
				v198TestLine(t, "<strong>_____", "content", p+1,
					v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(9)),
				v198TestText(t, "</strong>", "</strong>", p+1), v198TestClose(t, p+1)}
		}},
		{"formatting crosses visual break", v198TestContentBlock(t, "text", "<strong>", "<br>", "</strong>")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v198TestRun(t, v198TestPair(t, tc.body, tc.body), "current", nil)
		})
	}
	t.Run("different answers after equal formatted material", func(t *testing.T) {
		withAnswer := func(answer string) v198TestBody {
			return func(p int) []model.Line {
				return append(strong(p), v198TestTag(t, "question", "Which?", "content", p),
					v198TestTag(t, "answer", answer, "content", p))
			}
		}
		v198TestRun(t, v198TestPair(t, withAnswer("Yes"), withAnswer("No")), "current", nil)
		v198TestRun(t, v198TestPair(t, withAnswer("Yes"), withAnswer("Yes")), "equal", []v198TestWant{{
			[4]int{10, 1, 15, 12}, [][4]int{{4, 1, 9, 12}}, "Совпадают ветви: «A» (строка 4), «B» (строка 10).",
		}})
	})
	t.Run("separate formatted paragraphs remain separate", func(t *testing.T) {
		together := body("<strong>_____</strong><strong>_____</strong>",
			v198TestElement(t, "content", "<strong>", "<strong>", 1), placeholder(9),
			v198TestElement(t, "content", "</strong><strong>", "</strong><strong>", 14), placeholder(31),
			v198TestElement(t, "content", "</strong>", "</strong>", 36))
		separate := func(p int) []model.Line {
			lines := strong(p)
			return []model.Line{lines[0], lines[1], lines[1], lines[2]}
		}
		v198TestRun(t, v198TestPair(t, together, separate), "current", nil)
		v198TestRun(t, v198TestPair(t, separate, separate), "equal", []v198TestWant{{
			[4]int{9, 1, 13, 2}, [][4]int{{4, 1, 8, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 9).",
		}})
	})
}

func TestV198ExcludedBranches(t *testing.T) {
	cases := []struct {
		name string
		body v198TestBody
	}{
		{"unparsed", func(p int) []model.Line {
			return []model.Line{v198TestLine(t, "???", "invalid", p, v198TestElement(t, "unparsed", "???", "", 1))}
		}},
		{"heading", func(p int) []model.Line {
			return []model.Line{v198TestLine(t, "### X", "heading", 0, v198TestElement(t, "heading-level", "###", "3", 1), v198TestElement(t, "title", "X", "X", 5))}
		}},
		{"newpage", v198TestOneTag(t, "newpage", "", " ")},
		{"step outside task", v198TestOneTag(t, "step", "Read", "title")},
		{"key outside task", v198TestContentBlock(t, "key", "X")},
		{"orphan answer", v198TestOneTag(t, "answer", "Yes", "content")},
		{"forbidden nested tag", func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "note", "", "", p), v198TestTag(t, "instr", "Read.", "content", p+1), v198TestClose(t, p+1)}
		}},
		{"nested variants", func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "variants", "", "", p), v198TestTag(t, "variant", "inner", "name", p+1), v198TestText(t, "X", "X", p+2), v198TestClose(t, p+1)}
		}},
		{"hint without instruction", func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "instr", "", "", p), v198TestTag(t, "hint", "X", "content", p+1), v198TestClose(t, p+1)}
		}},
		{"invalid table structure", v198TestContentBlock(t, "table", "X | Y", "X")},
		{"invalid HTML structure", v198TestContentBlock(t, "text", "<strong>X</em>")},
		{"include in content block", func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "note", "", "", p), v198TestTag(t, "include", "f", "identifier", p+1), v198TestClose(t, p+1)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { v198TestRun(t, v198TestPair(t, tc.body, tc.body), "current", nil) })
	}
	t.Run("invalid third branch does not cancel valid group", func(t *testing.T) {
		result := v198TestDocument(t, v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "A", "name", 3), v198TestText(t, "X", "X", 4),
			v198TestTag(t, "variant", "B", "name", 3), v198TestText(t, "X", "X", 6), v198TestTag(t, "variant", "C", "name", 3),
			v198TestLine(t, "???", "invalid", 8, v198TestElement(t, "unparsed", "???", "", 1)), v198TestClose(t, 3))
		v198TestRun(t, result, "current", []v198TestWant{{[4]int{6, 1, 7, 2}, [][4]int{{4, 1, 5, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}})
	})
	t.Run("unclosed variants", func(t *testing.T) {
		result := v198TestDocument(t, v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "A", "name", 3), v198TestText(t, "X", "X", 4), v198TestTag(t, "variant", "B", "name", 3), v198TestText(t, "X", "X", 6))
		v198TestRun(t, result, "current", nil)
	})
	t.Run("unparsed unused definition is ignored", func(t *testing.T) {
		a := func(p int) []model.Line {
			return []model.Line{v198TestOpen(t, "fragment", "f", "identifier", p), v198TestLine(t, "???", "invalid", p+1, v198TestElement(t, "unparsed", "???", "", 1)), v198TestClose(t, p+1), v198TestText(t, "X", "X", p)}
		}
		v198TestRun(t, v198TestPair(t, a, v198TestParagraphs(t, "X")), "current", []v198TestWant{{[4]int{9, 1, 10, 2}, [][4]int{{4, 1, 8, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 9)."}})
	})
}

func TestV198DocumentBoundariesAndHistory(t *testing.T) {
	t.Run("separate containers", func(t *testing.T) {
		result := v198TestDocument(t, v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "A", "name", 3), v198TestText(t, "X", "X", 4), v198TestTag(t, "variant", "B", "name", 3), v198TestText(t, "Y", "Y", 6), v198TestClose(t, 3),
			v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "C", "name", 9), v198TestText(t, "X", "X", 10), v198TestTag(t, "variant", "D", "name", 9), v198TestText(t, "Y", "Y", 12), v198TestClose(t, 9))
		v198TestRun(t, result, "current", nil)
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v198TestPair(t, v198TestParagraphs(t, "Я😀"), v198TestParagraphs(t, "Я😀"))
			for i := range result.Lines {
				result.Lines[i].LineEnding = eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = ""
			want := []v198TestWant{{[4]int{6, 1, 7, 3}, [][4]int{{4, 1, 5, 3}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6)."}}
			v198TestRun(t, result, "first", want)
			v198TestRun(t, result, "second", want)
			v198TestRun(t, v198TestPair(t, v198TestParagraphs(t, "X"), v198TestParagraphs(t, "Y")), "other", nil)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v198TestDocument(t)
		zero := 0
		result.Lines = []model.Line{}
		result.Document.LineCount = &zero
		result.Document.Metadata.Title = nil
		v198TestRun(t, result, "current", nil)
	})
}

func TestV198ParameterSpacing(t *testing.T) {
	// Парсер выделяет placeholder отдельно от соседних content. Промежутки
	// остаются в Raw и не входят в Value этих элементов (контракт Lines).
	// По шагу 2 V198 один и два внутренних пробела не равны.
	body := func(extra bool) v198TestBody {
		return func(parent int) []model.Line {
			gap, column := " ", 13
			if extra {
				gap, column = "  ", 14
			}
			placeholder := v198TestElement(t, "placeholder", "_____", "", column)
			placeholder.Value = nil
			return []model.Line{v198TestLine(t, "@question A"+gap+"_____ B?", "tag", parent,
				v198TestElement(t, "tag", "@question", "question", 1),
				v198TestElement(t, "content", "A", "A", 11), placeholder,
				v198TestElement(t, "content", "B?", "B?", column+6))}
		}
	}
	v198TestRun(t, v198TestPair(t, body(false), body(true)), "current", nil)
	t.Run("equal spacing remains equal", func(t *testing.T) {
		v198TestRun(t, v198TestPair(t, body(false), body(false)), "current", []v198TestWant{{
			[4]int{6, 1, 7, 21}, [][4]int{{4, 1, 5, 21}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6).",
		}})
	})
	t.Run("content segmentation does not change parameter", func(t *testing.T) {
		split := func(parent int) []model.Line {
			return []model.Line{v198TestLine(t, "@question A B", "tag", parent,
				v198TestElement(t, "tag", "@question", "question", 1),
				v198TestElement(t, "content", "A", "A", 11),
				v198TestElement(t, "content", "B", "B", 13))}
		}
		v198TestRun(t, v198TestPair(t, split, v198TestOneTag(t, "question", "A B", "content")), "current", []v198TestWant{{
			[4]int{6, 1, 7, 14}, [][4]int{{4, 1, 5, 14}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6).",
		}})
	})
}

func TestV198ParameterAndContentFields(t *testing.T) {
	for _, name := range []string{"question", "multifill", "note"} {
		t.Run(name, func(t *testing.T) {
			body := func(separate, present bool) v198TestBody {
				return func(p int) []model.Line {
					start := utf8.RuneCountInString(name) + 3
					placeholder := v198TestElement(t, "placeholder", "_____", "_____", start+2)
					if !present {
						placeholder.Value = nil
					}
					left, right, rightStart := "A ", " B", start+7
					if separate {
						left, right, rightStart = "A", "B", start+8
					}
					return []model.Line{v198TestLine(t, "@"+name+" A _____ B", "tag", p,
						v198TestElement(t, "tag", "@"+name, name, 1),
						v198TestElement(t, "content", left, left, start), placeholder,
						v198TestElement(t, "content", right, right, rightStart))}
				}
			}
			end := 20
			if name == "multifill" {
				end = 21
			} else if name == "note" {
				end = 16
			}
			v198TestRun(t, v198TestPair(t, body(true, false), body(false, true)), "current", []v198TestWant{{
				[4]int{6, 1, 7, end}, [][4]int{{4, 1, 5, end}}, "Совпадают ветви: «A» (строка 4), «B» (строка 6).",
			}})
		})
	}
	for _, name := range []string{"question", "multifill", "note"} {
		t.Run("checked placeholder forbidden in "+name+" argument", func(t *testing.T) {
			body := func(p int) []model.Line {
				return []model.Line{v198TestLine(t, "@"+name+" _____{Yes}", "tag", p,
					v198TestElement(t, "tag", "@"+name, name, 1),
					v198TestElement(t, "placeholder", "_____{Yes}", "Yes", utf8.RuneCountInString(name)+3))}
			}
			v198TestRun(t, v198TestPair(t, body, body), "current", nil)
		})
	}
}

func TestV198InteractiveAnswerSettings(t *testing.T) {
	for _, name := range []string{"choice", "multichoice", "matching", "ordering"} {
		t.Run(name, func(t *testing.T) {
			body := func(answer string) v198TestBody {
				return func(p int) []model.Line {
					lines := []model.Line{v198TestOpen(t, name, "", "", p)}
					if name == "choice" || name == "multichoice" {
						return append(lines, v198TestTag(t, "answer", answer, "content", p+1), v198TestTag(t, "hint", "Think.", "content", p+1), v198TestText(t, "Other", "Other", p+1), v198TestClose(t, p+1))
					}
					return append(lines, v198TestText(t, "A", "A", p+1), v198TestTag(t, "answer", answer, "content", p+1), v198TestTag(t, "hint", "Think.", "content", p+1), v198TestText(t, "B", "B", p+1), v198TestTag(t, "answer", "Y", "content", p+1),
						v198TestLine(t, "---", "separator", p+1, v198TestElement(t, "content", "---", "---", 1)), v198TestText(t, "X", "X", p+1), v198TestText(t, "Y", "Y", p+1), v198TestClose(t, p+1))
				}
			}
			want := []v198TestWant{{[4]int{10, 1, 15, 2}, [][4]int{{4, 1, 9, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 10)."}}
			if name == "matching" || name == "ordering" {
				want = []v198TestWant{{[4]int{15, 1, 25, 2}, [][4]int{{4, 1, 14, 2}}, "Совпадают ветви: «A» (строка 4), «B» (строка 15)."}}
			}
			v198TestRun(t, v198TestPair(t, body("X"), body("X")), "equal", want)
			v198TestRun(t, v198TestPair(t, body("X"), body("Y")), "different", nil)
		})
	}
}

func TestV198FragmentDefinitionBeforeUse(t *testing.T) {
	result := v198TestDocument(t, v198TestOpen(t, "fragment", "f", "identifier", 2), v198TestText(t, "X", "X", 3), v198TestClose(t, 3),
		v198TestOpen(t, "variants", "", "", 2), v198TestTag(t, "variant", "A", "name", 6), v198TestTag(t, "include", "f", "identifier", 7),
		v198TestTag(t, "variant", "B", "name", 6), v198TestTag(t, "include", "F", "identifier", 9), v198TestClose(t, 6))
	v198TestRun(t, result, "current", []v198TestWant{{[4]int{9, 1, 10, 11}, [][4]int{{7, 1, 8, 11}}, "Совпадают ветви: «A» (строка 7), «B» (строка 9)."}})
}

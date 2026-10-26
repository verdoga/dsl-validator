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

const v191TestMessage = "@variant находится вне контейнера @variants."

// v191TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v191TestWant [3]int

func TestV191RegistrationAndVersions(t *testing.T) {
	registry := v191TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V191" {
			continue
		}
		count++
		if definition.Message != v191TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V191 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v191TestDocument(t)
			result.Document.DSLVersion = &version
			before := v191TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v191TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v191TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v191TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v191TestOpen(t, name, "", "", 2)}, body...)
	return v191TestDocument(t, append(lines, v191TestClose(t, 3))...)
}

// v191TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v191TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v191TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v191TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v191TestTag задаёт известное имя и явно указанный тип аргумента.
func v191TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v191TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v191TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v191TestLine(t, raw, "tag", parent, elements...)
}

// v191TestOpen добавляет открытие к известному объявлению блока.
func v191TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v191TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v191TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v191TestClose задаёт закрытие конкретного фигурного блока.
func v191TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v191TestLine(t, "}", "block-end", parent, v191TestElement(t, "block-close", "}", "}", 1))
}

// v191TestText сохраняет исходное и семантическое содержимое независимо.
func v191TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v191TestLine(t, raw, "content", parent, v191TestElement(t, "content", raw, value, 1))
}

// v191TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v191TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v191TestTag(t, "dsl-version", "1.2", "version", 0),
		v191TestLine(t, "# Lesson", "heading", 0, v191TestElement(t, "heading-level", "#", "1", 1),
			v191TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v191TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v191TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v191TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v191TestRegistry использует полный штатный каталог без отдельной регистрации.
func v191TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v191TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v191TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v191TestRun сравнивает новые V191 после полного Prepare/Run с явным ожиданием.
func v191TestRun(t *testing.T, result *model.Result, processingID string, want []v191TestWant) {
	t.Helper()
	before := v191TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v191TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v191TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V191" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V191 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v191TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 0 {
			t.Fatalf("V191 occurrence %d = %+v, want location %+v without detail or related ranges", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Fatal("V191 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V191 created an element error link")
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
	if err := json.Unmarshal([]byte(v191TestSnapshot(t, result)), &restored); err != nil {
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
	if v191TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

func TestV191ReferenceAndExample(t *testing.T) {
	// К исходным фрагментам добавлены только версия и заголовок документа.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v191TestWant
	}{
		{"reference", "@variants {\n@variant A\nRead.\n@variant B\nWrite.\n}", []model.Line{
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 3),
			v191TestText(t, "Read.", "Read.", 4), v191TestTag(t, "variant", "B", "name", 3),
			v191TestText(t, "Write.", "Write.", 6), v191TestClose(t, 3)}, nil},
		{"example", "@variant A\nRead.", []model.Line{v191TestTag(t, "variant", "A", "name", 2),
			v191TestText(t, "Read.", "Read.", 3)}, []v191TestWant{{3, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v191TestDocument(t, tc.body...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v191TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV191DeclarationsAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v191TestWant
	}{
		{"recognized", v191TestTag(t, "variant", "A", "name", 2), []v191TestWant{{3, 1, 9}}},
		{"Unicode name", v191TestTag(t, "variant", "Солнце 🌞", "name", 2), []v191TestWant{{3, 1, 9}}},
		{"indentation and case", v191TestLine(t, " \t@VaRiAnT A", "tag", 2,
			v191TestElement(t, "tag", "@VaRiAnT", "variant", 3), v191TestElement(t, "name", "A", "A", 12)), []v191TestWant{{3, 3, 11}}},
		{"tag with unparsed", v191TestLine(t, "@variant A {", "invalid", 2,
			v191TestElement(t, "tag", "@variant", "variant", 1), v191TestElement(t, "name", "A", "A", 10),
			v191TestElement(t, "unparsed", "{", "", 12)), []v191TestWant{{3, 1, 9}}},
		{"unparsed on tag row", v191TestLine(t, "@variant\tA", "tag", 2,
			v191TestElement(t, "unparsed", "@variant\tA", "", 1)), []v191TestWant{{3, 1, 9}}},
		{"erroneous block form", v191TestOpen(t, "variant", "A", "name", 2), []v191TestWant{{3, 1, 9}}},
		{"different tag", v191TestTag(t, "note", "Read.", "content", 2), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{tc.line, v191TestText(t, "Read.", "Read.", 3)}
			if tc.line.LineType == "block-start" {
				body = append(body, v191TestClose(t, 3))
			}
			v191TestRun(t, v191TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		raw  string
		want []v191TestWant
	}{
		{"@variant", []v191TestWant{{3, 1, 9}}},
		{"@variant A", []v191TestWant{{3, 1, 9}}},
		{"@variant\tA", []v191TestWant{{3, 1, 9}}},
		{"@variant{", []v191TestWant{{3, 1, 9}}},
		{"@variant}", []v191TestWant{{3, 1, 9}}},
		{" \t@VaRiAnT A", []v191TestWant{{3, 3, 11}}},
		{"@variants A", nil}, {"@variantExtra A", nil}, {"@variant2 A", nil}, {"@variant-extra A", nil},
		{"@variant! A", nil}, {"@unknown A", nil}, {"@", nil}, {"@@variant A", nil},
		{"@variant\u00a0A", nil}, {"@variant\u2003A", nil}, {"@variant\vA", nil}, {"\u00a0@variant A", nil},
		{`\@variant A`, nil}, {"Text @variant A", nil},
	} {
		t.Run("raw "+strconv.Quote(tc.raw), func(t *testing.T) {
			line := v191TestLine(t, tc.raw, "invalid", 2, v191TestElement(t, "unparsed", tc.raw, "", 1))
			v191TestRun(t, v191TestDocument(t, line, v191TestText(t, "Read.", "Read.", 3)), "current", tc.want)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@variant A`, "@variant A"}, {` \@variant A`, " @variant A"},
		{`\\@variant A`, `\@variant A`}, {"Text @variant A", "Text @variant A"}, {"🌞 @variant A", "🌞 @variant A"},
	} {
		t.Run("literal "+tc.raw, func(t *testing.T) {
			v191TestRun(t, v191TestDocument(t, v191TestText(t, tc.raw, tc.value, 2)), "current", nil)
		})
	}
}

func TestV191LogicalParentsAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v191TestWant
	}{
		{"outside logical task and step", []model.Line{
			v191TestTag(t, "task", "practice", "identifier", 2), v191TestTag(t, "step", "First", "title", 3),
			v191TestTag(t, "variant", "A", "name", 4), v191TestText(t, "Read.", "Read.", 5),
			v191TestTag(t, "endtask", "", "", 2)}, []v191TestWant{{5, 1, 9}}},
		{"inside logical task and step", []model.Line{
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 3),
			v191TestTag(t, "task", "practice", "identifier", 4), v191TestTag(t, "step", "First", "title", 5),
			v191TestText(t, "Read.", "Read.", 6), v191TestTag(t, "endtask", "", "", 4),
			v191TestTag(t, "variant", "B", "name", 3), v191TestText(t, "Write.", "Write.", 9), v191TestClose(t, 3)}, nil},
		{"before and after real block", []model.Line{
			v191TestTag(t, "variant", "Before", "name", 2), v191TestText(t, "Read.", "Read.", 3),
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 5),
			v191TestText(t, "Read.", "Read.", 6), v191TestTag(t, "variant", "B", "name", 5),
			v191TestText(t, "Write.", "Write.", 8), v191TestClose(t, 5),
			v191TestTag(t, "variant", "After", "name", 2), v191TestText(t, "Read.", "Read.", 11)},
			[]v191TestWant{{3, 1, 9}, {11, 1, 9}}},
		{"after reference block", []model.Line{
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 3),
			v191TestText(t, "Read.", "Read.", 4), v191TestTag(t, "variant", "B", "name", 3),
			v191TestText(t, "Write.", "Write.", 6), v191TestClose(t, 3),
			v191TestTag(t, "variant", "C", "name", 2), v191TestText(t, "Read.", "Read.", 9)}, []v191TestWant{{9, 1, 9}}},
		{"unclosed variants remains a container", []model.Line{
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 3),
			v191TestText(t, "Read.", "Read.", 4), v191TestTag(t, "variant", "B", "name", 3),
			v191TestText(t, "Write.", "Write.", 6)}, nil},
		{"inline variants does not open a container", []model.Line{
			v191TestTag(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 2),
			v191TestText(t, "Read.", "Read.", 4)}, []v191TestWant{{4, 1, 9}}},
		{"nested fragment", []model.Line{
			v191TestOpen(t, "fragment", "f", "identifier", 2), v191TestOpen(t, "VARIANTS", "", "", 3),
			v191TestTag(t, "VaRiAnT", "A", "name", 4), v191TestText(t, "Read.", "Read.", 5),
			v191TestTag(t, "variant", "B", "name", 4), v191TestText(t, "Write.", "Write.", 7),
			v191TestClose(t, 4), v191TestClose(t, 3), v191TestTag(t, "include", "f", "identifier", 2)}, nil},
		{"unknown nearest block conceals variants", []model.Line{
			v191TestOpen(t, "variants", "", "", 2),
			v191TestLine(t, "@unknown {", "block-start", 3, v191TestElement(t, "unparsed", "@unknown", "", 1),
				v191TestElement(t, "block-open", "{", "{", 10)),
			v191TestTag(t, "variant", "A", "name", 4), v191TestText(t, "Read.", "Read.", 5), v191TestClose(t, 4),
			v191TestTag(t, "variant", "B", "name", 3), v191TestText(t, "Write.", "Write.", 8), v191TestClose(t, 3)},
			[]v191TestWant{{5, 1, 9}}},
		{"several external branches", []model.Line{v191TestTag(t, "variant", "A", "name", 2),
			v191TestText(t, "Read.", "Read.", 3), v191TestTag(t, "variant", "B", "name", 2),
			v191TestText(t, "Write.", "Write.", 5)}, []v191TestWant{{3, 1, 9}, {5, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v191TestRun(t, v191TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("recovered opening name and child", func(t *testing.T) {
		opening := v191TestLine(t, "@VaRiAnTs {", "block-start", 2,
			v191TestElement(t, "unparsed", "@VaRiAnTs", "", 1), v191TestElement(t, "block-open", "{", "{", 11))
		v191TestRun(t, v191TestDocument(t, opening, v191TestLine(t, "@variant\tA", "invalid", 3,
			v191TestElement(t, "unparsed", "@variant\tA", "", 1)), v191TestText(t, "Read.", "Read.", 4),
			v191TestTag(t, "variant", "B", "name", 3), v191TestText(t, "Write.", "Write.", 6), v191TestClose(t, 3)), "current", nil)
	})
}

func TestV191HeadingKeepsOpenVariants(t *testing.T) {
	// Заголовок — логический корень, но физически variants закрывается строкой 7.
	result := v191TestDocument(t, v191TestOpen(t, "variants", "", "", 2),
		v191TestHeading(t), v191TestTag(t, "variant", "A", "name", 4),
		v191TestText(t, "Read.", "Read.", 5), v191TestClose(t, 3))
	v191TestRun(t, result, "current", nil)
}

func TestV191RecoveredCloseAfterHeading(t *testing.T) {
	result := v191TestDocument(t, v191TestOpen(t, "variants", "", "", 2), v191TestHeading(t),
		v191TestTag(t, "variant", "A", "name", 4), v191TestText(t, "X", "X", 5),
		v191TestLine(t, "}", "invalid", 4, v191TestElement(t, "unparsed", "}", "", 1)),
		v191TestTag(t, "variant", "Outside", "name", 4), v191TestText(t, "Y", "Y", 8))
	v191TestRun(t, result, "current", []v191TestWant{{8, 1, 9}})
}

func TestV191RecoveredClosePreservesNearestBoundary(t *testing.T) {
	result := v191TestDocument(t, v191TestOpen(t, "variants", "", "", 2),
		v191TestLine(t, "@unknown {", "block-start", 3, v191TestElement(t, "unparsed", "@unknown", "", 1),
			v191TestElement(t, "block-open", "{", "{", 10)), v191TestHeading(t),
		v191TestTag(t, "variant", "A", "name", 5),
		v191TestLine(t, " \t} ", "invalid", 5, v191TestElement(t, "unparsed", "}", "", 3)),
		v191TestTag(t, "variant", "B", "name", 5),
		v191TestLine(t, "}", "invalid", 5, v191TestElement(t, "unparsed", "}", "", 1)),
		v191TestTag(t, "variant", "Outside", "name", 5))
	v191TestRun(t, result, "current", []v191TestWant{{6, 1, 9}, {10, 1, 9}})
}

func TestV191BraceInsideContentDoesNotCloseVariants(t *testing.T) {
	for _, raw := range []string{`\}`, ` \} `, "text }", "} text", "_____{answer}", "{answer}", "}\u00a0"} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			result := v191TestDocument(t, v191TestOpen(t, "variants", "", "", 2), v191TestHeading(t),
				v191TestLine(t, raw, "invalid", 4, v191TestElement(t, "unparsed", raw, "", 1)),
				v191TestTag(t, "variant", "A", "name", 4), v191TestClose(t, 3),
				v191TestTag(t, "variant", "Outside", "name", 4))
			v191TestRun(t, result, "current", []v191TestWant{{8, 1, 9}})
		})
	}
}

// v191TestHeading задаёт корневой заголовок, который начинает новый логический шаг.
func v191TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v191TestLine(t, "### Reading", "heading", 0,
		v191TestElement(t, "heading-level", "###", "3", 1), v191TestElement(t, "title", "Reading", "Reading", 5))
}

func TestV191PhysicalBoundariesAfterHeading(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v191TestWant
	}{
		{"closed variants", []model.Line{
			v191TestOpen(t, "variants", "", "", 2), v191TestHeading(t),
			v191TestTag(t, "variant", "A", "name", 4), v191TestClose(t, 3),
			v191TestTag(t, "variant", "B", "name", 4)}, []v191TestWant{{7, 1, 9}}},
		{"unknown block hides variants until its close", []model.Line{
			v191TestOpen(t, "variants", "", "", 2),
			v191TestLine(t, "@unknown {", "block-start", 3, v191TestElement(t, "unparsed", "@unknown", "", 1),
				v191TestElement(t, "block-open", "{", "{", 10)),
			v191TestHeading(t), v191TestTag(t, "variant", "A", "name", 5), v191TestClose(t, 4),
			v191TestTag(t, "variant", "B", "name", 5), v191TestClose(t, 3),
			v191TestTag(t, "variant", "C", "name", 5)}, []v191TestWant{{6, 1, 9}, {10, 1, 9}}},
		{"recovered declarations and logical task", []model.Line{
			v191TestLine(t, "@VaRiAnTs{", "block-start", 2, v191TestElement(t, "unparsed", "@VaRiAnTs", "", 1),
				v191TestElement(t, "block-open", "{", "{", 10)),
			v191TestHeading(t), v191TestTag(t, "task", "practice", "identifier", 4),
			v191TestTag(t, "step", "First", "title", 5),
			v191TestLine(t, "\t@VaRiAnT A", "invalid", 6, v191TestElement(t, "unparsed", "@VaRiAnT A", "", 2)),
			v191TestTag(t, "endtask", "", "", 4), v191TestClose(t, 3)}, nil},
		{"unparsed brace does not open variants", []model.Line{
			v191TestLine(t, "@variants {", "invalid", 2, v191TestElement(t, "unparsed", "@variants {", "", 1)),
			v191TestHeading(t), v191TestTag(t, "variant", "A", "name", 4)}, []v191TestWant{{5, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v191TestRun(t, v191TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, name := range []string{"example", "wordlist", "table", "script", "text", "key", "note", "alt", "hint", "instr",
		"fragment", "answer", "choice", "multichoice", "matching", "ordering"} {
		t.Run("nearest exclusion "+name, func(t *testing.T) {
			v191TestRun(t, v191TestDocument(t, v191TestOpen(t, name, "", "", 2), v191TestHeading(t),
				v191TestTag(t, "variant", "A", "name", 4),
				v191TestLine(t, "@unknown {", "block-start", 5, v191TestElement(t, "unparsed", "@unknown", "", 1),
					v191TestElement(t, "block-open", "{", "{", 10)),
				v191TestTag(t, "variant", "B", "name", 4), v191TestClose(t, 6),
				v191TestTag(t, "variant", "C", "name", 4), v191TestClose(t, 3),
				v191TestTag(t, "variant", "D", "name", 4)), "current", []v191TestWant{{7, 1, 9}, {11, 1, 9}})
		})
	}
}

func TestV191LiteralBlockIgnoresApparentNestedStructure(t *testing.T) {
	for _, name := range []string{"EDITOR", "MULTIFILL"} {
		for _, closingParent := range []int{3, 5, 0} {
			t.Run(name+" closing parent="+strconv.Itoa(closingParent), func(t *testing.T) {
				// Ошибочная классификация тела не открывает DSL-блок в literal:
				// отдельная } завершает literal, а не видимый псевдоблок variants.
				closing := v191TestClose(t, closingParent)
				if closingParent == 0 {
					closing = v191TestLine(t, " \t}", "invalid", 5, v191TestElement(t, "unparsed", "}", "", 3))
				}
				v191TestRun(t, v191TestDocument(t, v191TestOpen(t, name, "", "", 2),
					v191TestText(t, "@variants {", "@variants {", 3),
					v191TestOpen(t, "variants", "", "", 3), v191TestHeading(t),
					v191TestLine(t, "@variant A", "invalid", 6, v191TestElement(t, "unparsed", "@variant A", "", 1)),
					v191TestText(t, `\}`, "}", 6), v191TestTag(t, "variant", "B", "name", 6), closing,
					v191TestTag(t, "variant", "Outside", "name", 6)), "current", []v191TestWant{{11, 1, 9}})
			})
		}
	}
}

func TestV191ExcludedContainers(t *testing.T) {
	// Список задан независимо от реализации, строго по таблице V191.
	for _, name := range []string{"example", "wordlist", "table", "script", "text", "key", "note", "alt", "hint", "instr",
		"fragment", "answer", "choice", "multichoice", "matching", "ordering"} {
		for _, recovered := range []bool{false, true} {
			t.Run(name+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				var body []model.Line
				if name == "answer" || name == "hint" {
					body = append(body, v191TestTag(t, "question", "Where?", "content", 2))
				}
				if name == "hint" {
					body = append(body, v191TestTag(t, "answer", "Here.", "content", 2))
				}
				parent := len(body) + 3
				argument, kind := "", model.ElementType("")
				if name == "fragment" {
					argument, kind = "f", "identifier"
				}
				body = append(body, v191TestOpen(t, strings.ToUpper(name), argument, kind, 2))
				child := v191TestTag(t, "variant", "A", "name", parent)
				if recovered {
					child = v191TestLine(t, "@variant\tA", "invalid", parent, v191TestElement(t, "unparsed", "@variant\tA", "", 1))
				}
				body = append(body, child, v191TestText(t, "Read.", "Read.", parent+1))
				switch name {
				case "table":
					body = append(body, v191TestText(t, "A | B", "A | B", parent), v191TestText(t, "C | D", "C | D", parent))
				case "matching":
					body = append(body, v191TestText(t, "A", "A", parent), v191TestText(t, "B", "B", parent),
						v191TestLine(t, "---", "separator", parent), v191TestText(t, "First", "First", parent), v191TestText(t, "Last", "Last", parent))
				case "choice", "multichoice", "ordering":
					body = append(body, v191TestText(t, "A", "A", parent), v191TestText(t, "B", "B", parent))
				case "script":
					body = append(body, v191TestText(t, "Tom: Hello.", "Tom: Hello.", parent))
				}
				body = append(body, v191TestClose(t, parent))
				if name == "fragment" {
					body = append(body, v191TestTag(t, "include", "f", "identifier", 2))
				}
				v191TestRun(t, v191TestDocument(t, body...), "current", nil)
			})
		}
	}
	t.Run("excluded nested block is not outer variants", func(t *testing.T) {
		v191TestRun(t, v191TestDocument(t, v191TestOpen(t, "variants", "", "", 2),
			v191TestTag(t, "variant", "A", "name", 3), v191TestOpen(t, "note", "", "", 4),
			v191TestTag(t, "variant", "Nested", "name", 5), v191TestText(t, "Read.", "Read.", 6), v191TestClose(t, 5),
			v191TestTag(t, "variant", "B", "name", 3), v191TestText(t, "Write.", "Write.", 9), v191TestClose(t, 3)), "current", nil)
	})
	t.Run("closed excluded block does not hide external variant", func(t *testing.T) {
		v191TestRun(t, v191TestDocument(t, v191TestOpen(t, "note", "", "", 2), v191TestText(t, "Read.", "Read.", 3),
			v191TestClose(t, 3), v191TestTag(t, "variant", "A", "name", 2), v191TestText(t, "Read.", "Read.", 6)),
			"current", []v191TestWant{{6, 1, 9}})
	})
	for _, name := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		for _, recovered := range []bool{false, true} {
			t.Run("literal "+name+" unparsed="+strconv.FormatBool(recovered), func(t *testing.T) {
				child := v191TestText(t, "@variant A", "@variant A", 4)
				if recovered {
					child = v191TestLine(t, "@variant A", "invalid", 4, v191TestElement(t, "unparsed", "@variant A", "", 1))
				}
				result := v191TestDocument(t, v191TestOpen(t, "fragment", "f", "identifier", 2),
					v191TestOpen(t, name, "", "", 3), child, v191TestText(t, "Read.", "Read.", 4), v191TestClose(t, 4),
					v191TestClose(t, 3), v191TestTag(t, "include", "f", "identifier", 2),
					v191TestTag(t, "variant", "Outside", "name", 2), v191TestText(t, "Write.", "Write.", 10))
				v191TestRun(t, result, "current", []v191TestWant{{10, 1, 9}})
			})
		}
	}
}

func TestV191NeighborsAndHistory(t *testing.T) {
	for _, tail := range []string{"Read.", "", " \t"} {
		t.Run("following content "+strconv.Quote(tail), func(t *testing.T) {
			line := v191TestText(t, tail, tail, 3)
			if strings.Trim(tail, " \t") == "" {
				line = v191TestLine(t, tail, "blank", 3)
			}
			v191TestRun(t, v191TestDocument(t, v191TestTag(t, "variant", "A", "name", 2), line), "current", []v191TestWant{{3, 1, 9}})
		})
	}
	t.Run("adjacent blocks keep their own branches", func(t *testing.T) {
		result := v191TestDocument(t,
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 3), v191TestText(t, "Read.", "Read.", 4),
			v191TestTag(t, "variant", "B", "name", 3), v191TestText(t, "Write.", "Write.", 6), v191TestClose(t, 3),
			v191TestTag(t, "variant", "Outside", "name", 2), v191TestText(t, "Read.", "Read.", 9),
			v191TestOpen(t, "variants", "", "", 2), v191TestTag(t, "variant", "A", "name", 11), v191TestText(t, "Read.", "Read.", 12),
			v191TestTag(t, "variant", "B", "name", 11), v191TestText(t, "Write.", "Write.", 14), v191TestClose(t, 11))
		want := []v191TestWant{{9, 1, 9}}
		v191TestRun(t, result, "first", want)
		v191TestRun(t, result, "second", want)
		v191TestRun(t, v191TestDocument(t, v191TestTag(t, "variant", "A", "name", 2)), "other", []v191TestWant{{3, 1, 9}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("endings "+strconv.Quote(string(ending)), func(t *testing.T) {
			result := v191TestDocument(t, v191TestTag(t, "variant", "Солнце 🌞", "name", 2))
			if ending == "" {
				result.Lines[2].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v191TestRun(t, result, "current", []v191TestWant{{3, 1, 9}})
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v191TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v191TestRun(t, result, "current", nil)
	})
}

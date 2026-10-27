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

const v178TestMessage = "Подсказка @hint находится вне допустимого места после ответа элемента @ordering."

// v178TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v178TestWant struct {
	location [3]int
	detail   string
}

const (
	v178TestAfter   = "Подсказка расположена после разделителя списков."
	v178TestRepeat  = "Подсказка повторяется после предыдущего @hint."
	v178TestMissing = "Нет непосредственно предшествующего ответа, связанного со строкой элемента."
)

func TestV178RegistrationAndVersions(t *testing.T) {
	registry := v178TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V178" {
			continue
		}
		count++
		if definition.Message != v178TestMessage || definition.Severity != "error" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V178 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v178TestDocument(t)
			result.Document.DSLVersion = &version
			before := v178TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v178TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v178TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v178TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v178TestLine(t, raw, "invalid", parent, v178TestElement(t, "unparsed", raw, "", 1))
	}
	return v178TestLine(t, raw, "separator", parent)
}

// v178TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v178TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v178TestOpen(t, name, "", "", 2)}, body...)
	return v178TestDocument(t, append(lines, v178TestClose(t, 3))...)
}

// v178TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v178TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v178TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v178TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v178TestTag задаёт известное имя и явно указанный тип аргумента.
func v178TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v178TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v178TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v178TestLine(t, raw, "tag", parent, elements...)
}

// v178TestOpen добавляет открытие к известному объявлению блока.
func v178TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v178TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v178TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v178TestClose задаёт закрытие конкретного фигурного блока.
func v178TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v178TestLine(t, "}", "block-end", parent, v178TestElement(t, "block-close", "}", "}", 1))
}

// v178TestText сохраняет исходное и семантическое содержимое независимо.
func v178TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v178TestLine(t, raw, "content", parent, v178TestElement(t, "content", raw, value, 1))
}

// v178TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v178TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v178TestTag(t, "dsl-version", "1.2", "version", 0),
		v178TestLine(t, "# Lesson", "heading", 0, v178TestElement(t, "heading-level", "#", "1", 1),
			v178TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v178TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v178TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v178TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v178TestRegistry использует полный штатный каталог без отдельной регистрации.
func v178TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v178TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v178TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v178TestRun сравнивает новые V178 после полного Prepare/Run с явным ожиданием.
func v178TestRun(t *testing.T, result *model.Result, processingID string, want []v178TestWant) {
	t.Helper()
	before := v178TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v178TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v178TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	if !slices.ContainsFunc(checks, func(d diagnostics.Definition) bool { return d.Code == "V178" }) {
		t.Fatal("V178 is missing from the prepared catalog")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V178" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V178 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v178TestMessage+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V178 occurrence %d = %+v, want %+v", i, entry, expected)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V178 error flags are missing")
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id != entry.ID {
						continue
					}
					links++
					if line.Number != expected.location[0] || element.ElementType != "tag" ||
						element.Start != expected.location[1] || element.End != expected.location[2] {
						t.Fatal("V178 linked to the wrong element")
					}
				}
			}
		}
		if links != 1 {
			t.Fatalf("V178 element links = %d, want 1", links)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v178TestSnapshot(t, result)), &restored); err != nil {
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
	if v178TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

func TestV178ReferenceAndExample(t *testing.T) {
	// Версия и заголовок добавлены перед неизменёнными фрагментами таблицы.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v178TestWant
	}{
		{"reference", "@ordering {\nA\n@answer {\n2\n}\n@hint Last.\nB\n@answer 1\n}", []model.Line{
			v178TestText(t, "A", "A", 3), v178TestOpen(t, "answer", "", "", 3), v178TestText(t, "2", "2", 5), v178TestClose(t, 5),
			v178TestTag(t, "hint", "Last.", "content", 3), v178TestText(t, "B", "B", 3), v178TestTag(t, "answer", "1", "content", 3)}, nil},
		{"example", "@ordering {\nA\n@answer 2\n@hint Last.\n@hint Think again.\nB\n@answer 1\n}", []model.Line{
			v178TestText(t, "A", "A", 3), v178TestTag(t, "answer", "2", "content", 3), v178TestTag(t, "hint", "Last.", "content", 3),
			v178TestTag(t, "hint", "Think again.", "content", 3), v178TestText(t, "B", "B", 3), v178TestTag(t, "answer", "1", "content", 3)},
			[]v178TestWant{{[3]int{7, 1, 6}, v178TestRepeat}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v178TestBlockDocument(t, "ordering", tc.body...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v178TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV178Adjacency(t *testing.T) {
	item := func(raw string) model.Line { return v178TestText(t, raw, raw, 3) }
	answer := func() model.Line { return v178TestTag(t, "answer", "1", "content", 3) }
	hint := func() model.Line { return v178TestTag(t, "hint", "Think.", "content", 3) }
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v178TestWant
	}{
		{"empty block", nil, nil},
		{"no hints", []model.Line{item("A"), answer(), item("B")}, nil},
		{"valid inline answer", []model.Line{item("A"), answer(), hint()}, nil},
		{"first and last construction", []model.Line{hint()}, []v178TestWant{{[3]int{4, 1, 6}, v178TestMissing}}},
		{"item without answer", []model.Line{item("A"), hint()}, []v178TestWant{{[3]int{5, 1, 6}, v178TestMissing}}},
		{"orphan answer", []model.Line{answer(), hint()}, []v178TestWant{{[3]int{5, 1, 6}, v178TestMissing}}},
		{"repeated answer", []model.Line{item("A"), answer(), answer(), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"answer before item", []model.Line{answer(), item("A"), hint()}, []v178TestWant{{[3]int{6, 1, 6}, v178TestMissing}}},
		{"intervening item", []model.Line{item("A"), answer(), item("B"), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"intervening note", []model.Line{item("A"), answer(), v178TestTag(t, "note", "Read.", "content", 3), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"editor before answer", []model.Line{item("A"), v178TestTag(t, "editor", "Read.", "content", 3), answer(), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"editor after answer", []model.Line{item("A"), answer(), v178TestTag(t, "editor", "Read.", "content", 3), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"three hints", []model.Line{item("A"), answer(), hint(), hint(), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestRepeat}, {[3]int{8, 1, 6}, v178TestRepeat}}},
		{"repetition after orphan hint", []model.Line{hint(), hint()}, []v178TestWant{{[3]int{4, 1, 6}, v178TestMissing}, {[3]int{5, 1, 6}, v178TestRepeat}}},
		{"new answer permits new hint", []model.Line{item("A"), answer(), hint(), item("B"), answer(), hint()}, nil},
		{"blank rows", []model.Line{item("A"), v178TestLine(t, "", "blank", 3), answer(), v178TestLine(t, " \t ", "blank", 3), hint()}, nil},
		{"blank does not hide repetition", []model.Line{item("A"), answer(), hint(), v178TestLine(t, "", "blank", 3), hint()}, []v178TestWant{{[3]int{8, 1, 6}, v178TestRepeat}}},
		{"blank is not an item", []model.Line{v178TestLine(t, " \t", "blank", 3), answer(), hint()}, []v178TestWant{{[3]int{6, 1, 6}, v178TestMissing}}},
		{"Unicode item", []model.Line{item("星 🌞"), answer(), hint()}, nil},
		{"NBSP is not trimmed as blank", []model.Line{item("\u00a0"), answer(), hint()}, nil},
		{"empty answer value is not a placement error", []model.Line{item("A"), v178TestTag(t, "answer", "", "", 3), hint()}, nil},
		{"invalid answer value is not a placement error", []model.Line{item("A"), v178TestTag(t, "answer", "999", "content", 3), hint()}, nil},
		{"invalid row is a barrier", []model.Line{item("A"), v178TestLine(t, "@unknown", "invalid", 3, v178TestElement(t, "unparsed", "@unknown", "", 1)), answer(), hint()}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
}

func TestV178BlockForms(t *testing.T) {
	item := v178TestText(t, "A", "A", 3)
	answer := v178TestTag(t, "answer", "1", "content", 3)
	hint := v178TestTag(t, "hint", "Think.", "content", 3)
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v178TestWant
	}{
		{"answer body does not spend search depth", []model.Line{item, v178TestOpen(t, "answer", "", "", 3),
			v178TestText(t, "1", "1", 5), v178TestLine(t, "", "blank", 5), v178TestText(t, "2", "2", 5), v178TestClose(t, 5), hint}, nil},
		{"empty answer block", []model.Line{item, v178TestOpen(t, "answer", "", "", 3), v178TestClose(t, 5), hint}, nil},
		{"orphan block answer", []model.Line{v178TestOpen(t, "answer", "", "", 3), v178TestText(t, "1", "1", 4), v178TestClose(t, 4), hint}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"repeated block answer", []model.Line{item, v178TestOpen(t, "answer", "", "", 3), v178TestText(t, "1", "1", 5), v178TestClose(t, 5),
			v178TestOpen(t, "answer", "", "", 3), v178TestText(t, "2", "2", 8), v178TestClose(t, 8), hint}, []v178TestWant{{[3]int{11, 1, 6}, v178TestMissing}}},
		{"valid block hint", []model.Line{item, answer, v178TestOpen(t, "hint", "", "", 3), v178TestText(t, "Think.", "Think.", 6), v178TestClose(t, 6)}, nil},
		{"orphan block hint", []model.Line{v178TestOpen(t, "hint", "", "", 3), v178TestText(t, "Think.", "Think.", 4), v178TestClose(t, 4)}, []v178TestWant{{[3]int{4, 1, 6}, v178TestMissing}}},
		{"hint after block hint", []model.Line{item, answer, v178TestOpen(t, "hint", "", "", 3), v178TestText(t, "Think.", "Think.", 6), v178TestClose(t, 6), hint}, []v178TestWant{{[3]int{9, 1, 6}, v178TestRepeat}}},
		{"block hint after inline hint", []model.Line{item, answer, hint, v178TestOpen(t, "hint", "", "", 3), v178TestText(t, "Think.", "Think.", 7), v178TestClose(t, 7)}, []v178TestWant{{[3]int{7, 1, 6}, v178TestRepeat}}},
		{"foreign block after answer", []model.Line{item, answer, v178TestOpen(t, "note", "", "", 3), v178TestText(t, "Read.", "Read.", 6), v178TestClose(t, 6), hint}, []v178TestWant{{[3]int{9, 1, 6}, v178TestMissing}}},
		{"editor block before answer", []model.Line{item, v178TestOpen(t, "editor", "", "", 3), v178TestText(t, "Read.", "Read.", 5), v178TestClose(t, 5), answer, hint}, []v178TestWant{{[3]int{9, 1, 6}, v178TestMissing}}},
		{"nested hint inside answer excluded", []model.Line{item, v178TestOpen(t, "answer", "", "", 3), v178TestTag(t, "hint", "Nested.", "content", 5), v178TestClose(t, 5), hint}, nil},
		{"nested hint inside hint excluded", []model.Line{item, answer, v178TestOpen(t, "hint", "", "", 3), v178TestTag(t, "hint", "Nested.", "content", 6), v178TestClose(t, 6)}, nil},
		{"answer inside foreign block cannot supply owner", []model.Line{v178TestOpen(t, "note", "", "", 3), v178TestText(t, "A", "A", 4), v178TestTag(t, "answer", "1", "content", 4), v178TestClose(t, 4), hint}, []v178TestWant{{[3]int{8, 1, 6}, v178TestMissing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
}

func TestV178Separators(t *testing.T) {
	item := v178TestText(t, "A", "A", 3)
	answer := v178TestTag(t, "answer", "First", "content", 3)
	hint := v178TestTag(t, "hint", "Think.", "content", 3)
	separator := v178TestSeparator(t, "---", 3, false)
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v178TestWant
	}{
		{"valid before separator", []model.Line{item, answer, hint, separator, v178TestText(t, "First", "First", 3), v178TestText(t, "Last", "Last", 3)}, nil},
		{"after separator even after answer", []model.Line{item, answer, separator, v178TestText(t, "First", "First", 3), answer, hint}, []v178TestWant{{[3]int{9, 1, 6}, v178TestAfter}}},
		{"after separator at start", []model.Line{separator, hint}, []v178TestWant{{[3]int{5, 1, 6}, v178TestAfter}}},
		{"after separator takes priority over repetition", []model.Line{item, separator, hint, hint}, []v178TestWant{{[3]int{6, 1, 6}, v178TestAfter}, {[3]int{7, 1, 6}, v178TestAfter}}},
		{"block hint after separator", []model.Line{item, answer, separator, v178TestOpen(t, "hint", "", "", 3), v178TestText(t, "Think.", "Think.", 7), v178TestClose(t, 7)}, []v178TestWant{{[3]int{7, 1, 6}, v178TestAfter}}},
		{"two separators skip whole block", []model.Line{hint, separator, hint, separator, hint}, nil},
		{"three separators skip whole block", []model.Line{hint, separator, separator, separator}, nil},
		{"indented separator", []model.Line{item, answer, v178TestSeparator(t, " \t---\t ", 3, false), hint}, []v178TestWant{{[3]int{7, 1, 6}, v178TestAfter}}},
		{"invalid separator still defines format", []model.Line{item, answer, v178TestSeparator(t, "---", 3, true), hint}, []v178TestWant{{[3]int{7, 1, 6}, v178TestAfter}}},
		{"escaped separator is an item", []model.Line{v178TestText(t, `\---`, "---", 3), answer, hint}, nil},
		{"long dash sequence is an item", []model.Line{v178TestText(t, "----", "----", 3), answer, hint}, nil},
		{"Unicode space is not removed from separator", []model.Line{v178TestText(t, "\u00a0---", "\u00a0---", 3), answer, hint}, nil},
		{"separator inside answer ignored", []model.Line{item, v178TestOpen(t, "answer", "", "", 3), v178TestText(t, "---", "---", 5), v178TestClose(t, 5), hint}, nil},
		{"nested separators do not suppress checking", []model.Line{v178TestOpen(t, "note", "", "", 3), v178TestText(t, "---", "---", 4), v178TestText(t, "---", "---", 4), v178TestClose(t, 4), hint}, []v178TestWant{{[3]int{8, 1, 6}, v178TestMissing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
}

func TestV178ContextsAndCoordinates(t *testing.T) {
	t.Run("indent and case", func(t *testing.T) {
		line := v178TestLine(t, " \t@HiNt 星 🌞", "tag", 3, v178TestElement(t, "tag", "@HiNt", "hint", 3), v178TestElement(t, "content", "星 🌞", "星 🌞", 9))
		v178TestRun(t, v178TestBlockDocument(t, "OrDeRiNg", line), "current", []v178TestWant{{[3]int{4, 3, 8}, v178TestMissing}})
	})
	t.Run("mixed case valid sequence", func(t *testing.T) {
		v178TestRun(t, v178TestBlockDocument(t, "ORDERING", v178TestText(t, "A", "A", 3),
			v178TestTag(t, "AnSwEr", "1", "content", 3), v178TestTag(t, "HiNt", "Think.", "content", 3)), "current", nil)
	})
	t.Run("ordering inside fragment", func(t *testing.T) {
		result := v178TestDocument(t, v178TestOpen(t, "fragment", "f", "identifier", 2), v178TestOpen(t, "ordering", "", "", 3),
			v178TestText(t, "A", "A", 4), v178TestTag(t, "answer", "1", "content", 4), v178TestTag(t, "hint", "Think.", "content", 4),
			v178TestTag(t, "hint", "Again.", "content", 4), v178TestClose(t, 4), v178TestClose(t, 3), v178TestTag(t, "include", "f", "identifier", 2))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{8, 1, 6}, v178TestRepeat}})
	})
	t.Run("two independent blocks", func(t *testing.T) {
		result := v178TestDocument(t, v178TestOpen(t, "ordering", "", "", 2), v178TestTag(t, "hint", "First.", "content", 3), v178TestClose(t, 3),
			v178TestOpen(t, "ordering", "", "", 2), v178TestTag(t, "hint", "Second.", "content", 6), v178TestClose(t, 6))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{4, 1, 6}, v178TestMissing}, {[3]int{7, 1, 6}, v178TestMissing}})
	})
	t.Run("answer from previous container is not reused", func(t *testing.T) {
		result := v178TestDocument(t, v178TestOpen(t, "ordering", "", "", 2), v178TestText(t, "A", "A", 3), v178TestTag(t, "answer", "1", "content", 3), v178TestClose(t, 3),
			v178TestOpen(t, "ordering", "", "", 2), v178TestTag(t, "hint", "Think.", "content", 7), v178TestClose(t, 7))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{8, 1, 6}, v178TestMissing}})
	})
	t.Run("separator count belongs to its container", func(t *testing.T) {
		result := v178TestDocument(t, v178TestOpen(t, "ordering", "", "", 2), v178TestSeparator(t, "---", 3, false), v178TestSeparator(t, "---", 3, true), v178TestTag(t, "hint", "Ignored.", "content", 3), v178TestClose(t, 3),
			v178TestOpen(t, "ordering", "", "", 2), v178TestTag(t, "hint", "Think.", "content", 8), v178TestClose(t, 8))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{9, 1, 6}, v178TestMissing}})
	})
	for _, name := range []string{"choice", "multichoice", "matching", "note"} {
		t.Run("other container "+name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, name, v178TestTag(t, "hint", "Think.", "content", 3)), "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal declarations in "+name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, name, v178TestText(t, "@ordering {", "@ordering {", 3),
				v178TestText(t, "@hint Think.", "@hint Think.", 3), v178TestText(t, `\}`, "}", 3)), "current", nil)
		})
	}
	t.Run("escaped hint is ordinary text", func(t *testing.T) {
		v178TestRun(t, v178TestBlockDocument(t, "ordering", v178TestText(t, `\@hint Think.`, "@hint Think.", 3)), "current", nil)
	})
	t.Run("outside ordering", func(t *testing.T) {
		v178TestRun(t, v178TestDocument(t, v178TestTag(t, "hint", "Before.", "content", 2), v178TestOpen(t, "ordering", "", "", 2),
			v178TestText(t, "A", "A", 4), v178TestText(t, "B", "B", 4), v178TestClose(t, 4), v178TestTag(t, "hint", "After.", "content", 2)), "current", nil)
	})
}

func TestV178EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v178TestBlockDocument(t, "ordering", v178TestText(t, "A", "A", 3), v178TestTag(t, "answer", "1", "content", 3),
				v178TestTag(t, "hint", "Think.", "content", 3), v178TestTag(t, "hint", "Again.", "content", 3))
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v178TestWant{{[3]int{7, 1, 6}, v178TestRepeat}}
			v178TestRun(t, result, "first", want)
			v178TestRun(t, result, "second", want)
			v178TestRun(t, v178TestBlockDocument(t, "ordering", v178TestText(t, "A", "A", 3), v178TestTag(t, "answer", "1", "content", 3), v178TestTag(t, "hint", "Think.", "content", 3)), "other", nil)
		})
	}
	t.Run("prior answer error does not control placement check", func(t *testing.T) {
		result := v178TestBlockDocument(t, "ordering", v178TestText(t, "A", "A", 3), v178TestTag(t, "answer", "999", "content", 3), v178TestTag(t, "hint", "Think.", "content", 3))
		v178TestParserError(t, result, 5, 1, "Номер позиции вне диапазона.")
		v178TestRun(t, result, "current", nil)
	})
	t.Run("empty document", func(t *testing.T) {
		result := v178TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v178TestRun(t, result, "current", nil)
	})
}

// v178TestHeading задаёт корневой heading, который не закрывает фигурный блок.
func v178TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v178TestLine(t, "### Inside", "heading", 0,
		v178TestElement(t, "heading-level", "###", "3", 1), v178TestElement(t, "title", "Inside", "Inside", 5))
}

func TestV178HeadingDoesNotCloseOrdering(t *testing.T) {
	for _, rootHeading := range []bool{false, true} {
		name, parent := "attached blank control", 3
		boundary := v178TestLine(t, "", "blank", 3)
		if rootHeading {
			name, parent = "root heading and reparented hint", 4
			boundary = v178TestHeading(t)
		}
		t.Run(name, func(t *testing.T) {
			result := v178TestBlockDocument(t, "ordering", boundary,
				v178TestTag(t, "hint", "Orphan", "content", parent), v178TestText(t, "One", "One", parent), v178TestText(t, "Two", "Two", parent))
			v178TestRun(t, result, "current", []v178TestWant{{[3]int{5, 1, 6}, v178TestMissing}})
		})
	}
}

func TestV178ReparentedAdjacency(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v178TestWant
	}{
		{"new valid sequence", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4),
			v178TestTag(t, "answer", "1", "content", 4), v178TestTag(t, "hint", "Think.", "content", 4)}, nil},
		{"heading between item and answer", []model.Line{v178TestText(t, "A", "A", 3), v178TestHeading(t),
			v178TestTag(t, "answer", "1", "content", 5), v178TestTag(t, "hint", "Think.", "content", 5)},
			[]v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"heading between answer and hint", []model.Line{v178TestText(t, "A", "A", 3), v178TestTag(t, "answer", "1", "content", 3),
			v178TestHeading(t), v178TestTag(t, "hint", "Think.", "content", 6)}, []v178TestWant{{[3]int{7, 1, 6}, v178TestMissing}}},
		{"repeated hint", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4), v178TestTag(t, "answer", "1", "content", 4),
			v178TestTag(t, "hint", "Think.", "content", 4), v178TestTag(t, "hint", "Again.", "content", 4)}, []v178TestWant{{[3]int{8, 1, 6}, v178TestRepeat}}},
		{"heading resets repeated hint", []model.Line{v178TestTag(t, "hint", "First.", "content", 3), v178TestHeading(t),
			v178TestTag(t, "hint", "Again.", "content", 5)}, []v178TestWant{{[3]int{4, 1, 6}, v178TestMissing}, {[3]int{6, 1, 6}, v178TestMissing}}},
		{"editor is significant", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4), v178TestTag(t, "answer", "1", "content", 4),
			v178TestTag(t, "editor", "Read.", "content", 4), v178TestTag(t, "hint", "Think.", "content", 4)}, []v178TestWant{{[3]int{8, 1, 6}, v178TestMissing}}},
		{"root blanks preserve two constructions", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4),
			v178TestLine(t, " \t", "blank", 0), v178TestTag(t, "answer", "1", "content", 4), v178TestLine(t, " \t", "blank", 0),
			v178TestTag(t, "hint", "Think.", "content", 4)}, nil},
		{"block answer and heading in its body", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4),
			v178TestOpen(t, "answer", "", "", 4), v178TestText(t, "1", "1", 6), v178TestLine(t, "", "blank", 0),
			v178TestHeading(t), v178TestText(t, "2", "2", 9), v178TestClose(t, 6), v178TestTag(t, "hint", "Think.", "content", 9)}, nil},
		{"orphan block hint then repeat", []model.Line{v178TestHeading(t), v178TestOpen(t, "hint", "", "", 4),
			v178TestText(t, "Think.", "Think.", 5), v178TestClose(t, 5), v178TestTag(t, "hint", "Again.", "content", 4)},
			[]v178TestWant{{[3]int{5, 1, 6}, v178TestMissing}, {[3]int{8, 1, 6}, v178TestRepeat}}},
		{"valid block hint", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4), v178TestTag(t, "answer", "1", "content", 4),
			v178TestOpen(t, "hint", "", "", 4), v178TestHeading(t), v178TestTag(t, "hint", "Nested.", "content", 8), v178TestClose(t, 7)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
}

func TestV178ReparentedSeparators(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v178TestWant
	}{
		{"separator before heading", []model.Line{v178TestSeparator(t, "---", 3, false), v178TestHeading(t),
			v178TestTag(t, "hint", "Think.", "content", 5)}, []v178TestWant{{[3]int{6, 1, 6}, v178TestAfter}}},
		{"separator after heading", []model.Line{v178TestHeading(t), v178TestSeparator(t, "---", 4, false),
			v178TestTag(t, "hint", "Think.", "content", 4)}, []v178TestWant{{[3]int{6, 1, 6}, v178TestAfter}}},
		{"block hint after separator", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4),
			v178TestTag(t, "answer", "First", "content", 4), v178TestSeparator(t, "---", 4, false), v178TestOpen(t, "hint", "", "", 4),
			v178TestText(t, "Think.", "Think.", 8), v178TestClose(t, 8)}, []v178TestWant{{[3]int{8, 1, 6}, v178TestAfter}}},
		{"valid sequence before separator", []model.Line{v178TestHeading(t), v178TestText(t, "A", "A", 4),
			v178TestTag(t, "answer", "First", "content", 4), v178TestTag(t, "hint", "Think.", "content", 4),
			v178TestSeparator(t, "---", 4, false), v178TestText(t, "First", "First", 4)}, nil},
		{"two physical separators skip block", []model.Line{v178TestTag(t, "hint", "First.", "content", 3), v178TestSeparator(t, "---", 3, false),
			v178TestHeading(t), v178TestTag(t, "hint", "Second.", "content", 6), v178TestSeparator(t, "---", 6, true),
			v178TestTag(t, "hint", "Third.", "content", 6)}, nil},
		{"after separator overrides repeat", []model.Line{v178TestSeparator(t, "---", 3, false), v178TestHeading(t),
			v178TestTag(t, "hint", "First.", "content", 5), v178TestTag(t, "hint", "Second.", "content", 5)},
			[]v178TestWant{{[3]int{6, 1, 6}, v178TestAfter}, {[3]int{7, 1, 6}, v178TestAfter}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v178TestRun(t, v178TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
}

func TestV178PhysicalNestedBodiesAndClosures(t *testing.T) {
	t.Run("nested heading hint and separators stay nested", func(t *testing.T) {
		result := v178TestBlockDocument(t, "ordering", v178TestHeading(t), v178TestOpen(t, "note", "", "", 4),
			v178TestHeading(t), v178TestTag(t, "hint", "Nested.", "content", 6), v178TestSeparator(t, "---", 6, false),
			v178TestSeparator(t, "---", 6, true), v178TestClose(t, 5), v178TestTag(t, "hint", "Direct.", "content", 6))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{11, 1, 6}, v178TestMissing}})
	})
	t.Run("closed ordering does not own later rows with stale parent", func(t *testing.T) {
		result := v178TestDocument(t, v178TestOpen(t, "ordering", "", "", 2), v178TestHeading(t),
			v178TestTag(t, "hint", "Inside.", "content", 4), v178TestClose(t, 3), v178TestTag(t, "hint", "Outside.", "content", 4),
			v178TestOpen(t, "ordering", "", "", 4), v178TestTag(t, "hint", "Next.", "content", 4), v178TestClose(t, 8))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{5, 1, 6}, v178TestMissing}, {[3]int{9, 1, 6}, v178TestMissing}})
	})
	for _, partial := range []bool{false, true} {
		t.Run("recovered closing partial="+strconv.FormatBool(partial), func(t *testing.T) {
			closing := v178TestLine(t, " \t} ", "invalid", 4, v178TestElement(t, "unparsed", " \t} ", "", 1))
			if partial {
				closing = v178TestLine(t, "} trailing", "invalid", 4, v178TestElement(t, "block-close", "}", "}", 1),
					v178TestElement(t, "unparsed", "trailing", "", 3))
			}
			result := v178TestDocument(t, v178TestOpen(t, "ordering", "", "", 2), v178TestHeading(t),
				v178TestTag(t, "hint", "Inside.", "content", 4), closing, v178TestTag(t, "hint", "Outside.", "content", 4))
			v178TestRun(t, result, "current", []v178TestWant{{[3]int{5, 1, 6}, v178TestMissing}})
		})
	}
	t.Run("recognized opening on invalid row keeps nested scope", func(t *testing.T) {
		opener := v178TestLine(t, "@note ? {", "invalid", 4, v178TestElement(t, "tag", "@note", "note", 1),
			v178TestElement(t, "unparsed", "?", "", 7), v178TestElement(t, "block-open", "{", "{", 9))
		result := v178TestBlockDocument(t, "ordering", v178TestHeading(t), opener, v178TestHeading(t),
			v178TestTag(t, "hint", "Nested.", "content", 6), v178TestLine(t, "}", "invalid", 6, v178TestElement(t, "unparsed", "}", "", 1)),
			v178TestTag(t, "hint", "Direct.", "content", 6))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{9, 1, 6}, v178TestMissing}})
	})
	t.Run("ordinary and escaped braces do not close", func(t *testing.T) {
		result := v178TestBlockDocument(t, "ordering", v178TestHeading(t), v178TestText(t, `\}`, "}", 4),
			v178TestText(t, `Text \}`, "Text }", 4), v178TestText(t, `\@note \{`, "@note {", 4),
			v178TestTag(t, "hint", "Direct.", "content", 4))
		v178TestRun(t, result, "current", []v178TestWant{{[3]int{8, 1, 6}, v178TestMissing}})
	})
	for _, name := range []string{"editor", "multifill"} {
		answerValue := `\@answer Literal.`
		if name == "multifill" {
			answerValue = "@answer Literal."
		}
		for _, literal := range [][2]string{{"@hint Literal.", "@hint Literal."}, {"### Literal", "### Literal"},
			{`\}`, "}"}, {`\@answer Literal.`, answerValue}} {
			t.Run("literal "+name+" content="+literal[0], func(t *testing.T) {
				result := v178TestDocument(t, v178TestHeading(t), v178TestOpen(t, name, "", "", 3),
					v178TestText(t, `@ordering \{`, "@ordering {", 4), v178TestText(t, literal[0], literal[1], 4),
					v178TestLine(t, "} trailing", "invalid", 4, v178TestElement(t, "unparsed", "} trailing", "", 1)),
					v178TestText(t, "@hint Still literal.", "@hint Still literal.", 4), v178TestClose(t, 4),
					v178TestOpen(t, "ordering", "", "", 3), v178TestTag(t, "hint", "Direct.", "content", 3), v178TestClose(t, 10))
				v178TestRun(t, result, "current", []v178TestWant{{[3]int{11, 1, 6}, v178TestMissing}})
			})
		}
	}
}

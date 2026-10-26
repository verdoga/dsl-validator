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

const v169TestMessage = "Ответы заданы только для части элементов верхнего списка."

// v169TestWant задаёт исходный диапазон блока и связанные строки: конец исключается.
type v169TestWant struct {
	location [4]int
	related  [][3]int
}

func TestV169RegistrationAndVersions(t *testing.T) {
	registry := v169TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V169" {
			continue
		}
		count++
		if definition.Message != v169TestMessage || definition.Severity != "error" ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V169 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v169TestDocument(t)
			result.Document.DSLVersion = &version
			before := v169TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v169TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v169TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v169TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v169TestLine(t, raw, "invalid", parent, v169TestElement(t, "unparsed", raw, "", 1))
	}
	return v169TestLine(t, raw, "separator", parent)
}

// v169TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v169TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v169TestOpen(t, name, "", "", 2)}, body...)
	return v169TestDocument(t, append(lines, v169TestClose(t, 3))...)
}

// v169TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v169TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v169TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v169TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v169TestTag задаёт известное имя и явно указанный тип аргумента.
func v169TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v169TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v169TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v169TestLine(t, raw, "tag", parent, elements...)
}

// v169TestOpen добавляет открытие к известному объявлению блока.
func v169TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v169TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v169TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v169TestClose задаёт закрытие конкретного фигурного блока.
func v169TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v169TestLine(t, "}", "block-end", parent, v169TestElement(t, "block-close", "}", "}", 1))
}

// v169TestText сохраняет исходное и семантическое содержимое независимо.
func v169TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v169TestLine(t, raw, "content", parent, v169TestElement(t, "content", raw, value, 1))
}

// v169TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v169TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v169TestTag(t, "dsl-version", "1.2", "version", 0),
		v169TestLine(t, "# Lesson", "heading", 0, v169TestElement(t, "heading-level", "#", "1", 1),
			v169TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v169TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v169TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v169TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v169TestRegistry использует полный штатный каталог без отдельной регистрации.
func v169TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v169TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v169TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v169TestRun сравнивает новые V169 после полного Prepare/Run с явным ожиданием.
func v169TestRun(t *testing.T, result *model.Result, processingID string, want []v169TestWant) {
	t.Helper()
	before := v169TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v169TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v169TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V169" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V169 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[2], Column: expected.location[3]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v169TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V169 occurrence %d = %+v, want %+v, no detail", i, entry, expected)
		}
		for j, r := range expected.related {
			related := model.Location{Start: model.Position{Line: r[0], Column: r[1]}, End: model.Position{Line: r[0], Column: r[2]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("related[%d] = %+v, want %+v", j, entry.RelatedLocations[j], related)
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V169 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("block-scoped V169 created an element error link")
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
	if err := json.Unmarshal([]byte(v169TestSnapshot(t, result)), &restored); err != nil {
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
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v169TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

// v169TestMatching дополняет явные верхние строки разделителем и нижними Да/Нет.
// Блок начинается на строке 3, первая верхняя строка — 4.
func v169TestMatching(t *testing.T, upper ...model.Line) *model.Result {
	t.Helper()
	body := append(upper, v169TestSeparator(t, "---", 3, false), v169TestText(t, "Да", "Да", 3), v169TestText(t, "Нет", "Нет", 3))
	return v169TestBlockDocument(t, "matching", body...)
}

func TestV169ReferenceAndExample(t *testing.T) {
	// Добавлены только версия и обязательный заголовок. Исходные фрагменты неизменны.
	for _, tc := range []struct {
		name, fragment string
		result         *model.Result
		want           []v169TestWant
	}{
		{"reference", "@matching {\nСолнце — звезда?\n@answer Да\nЛуна — звезда?\n@answer Нет\n---\nДа\nНет\n}",
			v169TestMatching(t, v169TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3), v169TestTag(t, "answer", "Да", "content", 3),
				v169TestText(t, "Луна — звезда?", "Луна — звезда?", 3), v169TestTag(t, "answer", "Нет", "content", 3)), nil},
		{"example", "@matching {\nСолнце — звезда?\n@answer Да\nЛуна — звезда?\n---\nДа\nНет\n}",
			v169TestMatching(t, v169TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3), v169TestTag(t, "answer", "Да", "content", 3),
				v169TestText(t, "Луна — звезда?", "Луна — звезда?", 3)), []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 15}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v169TestRun(t, tc.result, "current", tc.want)
		})
	}
}

func TestV169Counts(t *testing.T) {
	text := func(raw string) model.Line { return v169TestText(t, raw, raw, 3) }
	answer := func() model.Line { return v169TestTag(t, "answer", "Да", "content", 3) }
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v169TestWant
	}{
		{"empty upper", nil, nil},
		{"single no answer", []model.Line{text("A")}, nil},
		{"single answered", []model.Line{text("A"), answer()}, nil},
		{"none answered", []model.Line{text("A"), text("B")}, nil},
		{"all answered", []model.Line{text("A"), answer(), text("B"), answer(), text("C"), answer()}, nil},
		{"partial", []model.Line{text("A"), answer(), text("B")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"first item missing", []model.Line{text("A"), text("B"), answer()}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{4, 1, 2}}}}},
		{"two missing", []model.Line{text("A"), answer(), text("B"), text("C")}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{6, 1, 2}, {7, 1, 2}}}}},
		{"one missing among three", []model.Line{text("A"), answer(), text("B"), answer(), text("C")}, []v169TestWant{{[4]int{3, 1, 12, 2}, [][3]int{{8, 1, 2}}}}},
		{"missing on both sides", []model.Line{text("A"), text("B"), answer(), text("C")}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{4, 1, 2}, {7, 1, 2}}}}},
		{"equal items distinct", []model.Line{text("A"), answer(), text("A")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"blank skipped", []model.Line{text("A"), v169TestLine(t, "", "blank", 3), answer(), text("B")}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{7, 1, 2}}}}},
		{"spaces and tabs skipped", []model.Line{text("A"), v169TestLine(t, " \t ", "blank", 3), answer(), text("B")}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{7, 1, 2}}}}},
		{"NBSP is an item", []model.Line{text("A"), answer(), text("\u00a0")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"Unicode item", []model.Line{text("A"), answer(), text("星 🌞")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 4}}}}},
		{"empty answer declaration counts", []model.Line{text("A"), v169TestTag(t, "answer", "", "", 3), text("B")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"wrong answer value still counts", []model.Line{text("A"), v169TestTag(t, "answer", "Missing", "content", 3), text("B")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"several content elements one item", []model.Line{v169TestLine(t, "A B", "content", 3, v169TestElement(t, "content", "A", "A", 1), v169TestElement(t, "content", "B", "B", 3)), answer(), text("C")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"escaped separator is an item", []model.Line{text("A"), answer(), v169TestText(t, `\---`, "---", 3)}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 5}}}}},
		{"escaped answer is an item", []model.Line{text("A"), answer(), v169TestText(t, `\@answer Да`, "@answer Да", 3)}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 12}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v169TestRun(t, v169TestMatching(t, tc.upper...), "current", tc.want) })
	}
}

func TestV169AdjacencyGuard(t *testing.T) {
	text := func(raw string) model.Line { return v169TestText(t, raw, raw, 3) }
	answer := func() model.Line { return v169TestTag(t, "answer", "Да", "content", 3) }
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v169TestWant
	}{
		{"orphan before valid pair", []model.Line{answer(), text("A"), answer(), text("B")}, nil},
		{"repeated answer", []model.Line{text("A"), answer(), answer(), text("B")}, nil},
		{"note between item and answer", []model.Line{text("A"), answer(), text("B"), v169TestTag(t, "note", "Read.", "content", 3), answer(), text("C")}, nil},
		{"editor is not skipped", []model.Line{text("A"), answer(), text("B"), v169TestTag(t, "editor", "Check.", "content", 3), answer(), text("C")}, nil},
		{"block between item and answer", []model.Line{text("A"), answer(), text("B"), v169TestOpen(t, "hint", "", "", 3), v169TestText(t, "Read.", "Read.", 7), v169TestClose(t, 7), answer(), text("C")}, nil},
		{"block editor is not skipped", []model.Line{text("A"), answer(), text("B"), v169TestOpen(t, "editor", "", "", 3), v169TestText(t, "Check.", "Check.", 7), v169TestClose(t, 7), answer(), text("C")}, nil},
		{"invalid between item and answer", []model.Line{text("A"), answer(), text("B"), v169TestLine(t, "@unknown", "invalid", 3, v169TestElement(t, "unparsed", "@unknown", "", 1)), answer(), text("C")}, nil},
		{"tag without following answer does not cancel", []model.Line{text("A"), answer(), text("B"), v169TestTag(t, "editor", "Check.", "content", 3)}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{6, 1, 2}}}}},
		{"answer inside nested block is not attached to list item", []model.Line{text("A"), answer(), text("B"), v169TestOpen(t, "hint", "", "", 3), v169TestText(t, "Nested", "Nested", 7), v169TestTag(t, "answer", "Да", "content", 7), v169TestClose(t, 7)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v169TestRun(t, v169TestMatching(t, tc.upper...), "current", tc.want) })
	}
}

func TestV169InvalidAnswerDeclaration(t *testing.T) {
	invalidAnswer := func(parent int) model.Line {
		return v169TestLine(t, "@answer No }", "invalid", parent,
			v169TestElement(t, "tag", "@answer", "answer", 1),
			v169TestElement(t, "content", "No", "No", 9),
			v169TestElement(t, "unparsed", "}", "", 12))
	}
	text := func(raw string) model.Line { return v169TestText(t, raw, raw, 3) }
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v169TestWant
	}{
		{"all items have a declared answer", []model.Line{text("A"), v169TestTag(t, "answer", "Yes", "content", 3), text("B"), invalidAnswer(3)}, nil},
		{"invalid value still declares the only answer", []model.Line{text("A"), invalidAnswer(3), text("B")}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"orphan invalid answer cancels check", []model.Line{invalidAnswer(3), text("A"), v169TestTag(t, "answer", "Yes", "content", 3), text("B")}, nil},
		{"nested invalid answer is not attached", []model.Line{text("A"), v169TestTag(t, "answer", "Yes", "content", 3), text("B"),
			v169TestOpen(t, "hint", "", "", 3), invalidAnswer(7), v169TestClose(t, 7)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.upper, v169TestSeparator(t, "---", 3, false), text("Yes"), text("No"))
			// v169TestDocument связывает unparsed с исходной ошибкой парсера.
			v169TestRun(t, v169TestBlockDocument(t, "matching", body...), "current", tc.want)
		})
	}
}

func TestV169AnswerAndHintBlocks(t *testing.T) {
	text := func(raw string) model.Line { return v169TestText(t, raw, raw, 3) }
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v169TestWant
	}{
		{"block answer counts once", []model.Line{text("A"), v169TestOpen(t, "answer", "", "", 3), v169TestText(t, "Да", "Да", 5), v169TestText(t, "Нет", "Нет", 5), v169TestClose(t, 5), text("B")}, []v169TestWant{{[4]int{3, 1, 13, 2}, [][3]int{{9, 1, 2}}}}},
		{"empty block answer counts", []model.Line{text("A"), v169TestOpen(t, "answer", "", "", 3), v169TestClose(t, 5), text("B")}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{7, 1, 2}}}}},
		{"all answered with mixed forms", []model.Line{text("A"), v169TestOpen(t, "answer", "", "", 3), v169TestText(t, "Да", "Да", 5), v169TestClose(t, 5), text("B"), v169TestTag(t, "answer", "Нет", "content", 3)}, nil},
		{"inline hint is not an item", []model.Line{text("A"), v169TestTag(t, "answer", "Да", "content", 3), v169TestTag(t, "hint", "Read.", "content", 3), text("B")}, []v169TestWant{{[4]int{3, 1, 11, 2}, [][3]int{{7, 1, 2}}}}},
		{"block hint is not an item", []model.Line{text("A"), v169TestTag(t, "answer", "Да", "content", 3), v169TestOpen(t, "hint", "", "", 3), v169TestText(t, "Read.", "Read.", 6), v169TestClose(t, 6), text("B")}, []v169TestWant{{[4]int{3, 1, 13, 2}, [][3]int{{9, 1, 2}}}}},
		{"nested separator is not a list boundary", []model.Line{text("A"), v169TestOpen(t, "answer", "", "", 3), v169TestText(t, "---", "---", 5), v169TestClose(t, 5), text("B")}, []v169TestWant{{[4]int{3, 1, 12, 2}, [][3]int{{8, 1, 2}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v169TestRun(t, v169TestMatching(t, tc.upper...), "current", tc.want) })
	}
}

func TestV169SeparatorsAndLowerList(t *testing.T) {
	for _, tc := range []struct {
		name string
		tail []model.Line
		want []v169TestWant
	}{
		{"no separator", []model.Line{v169TestText(t, "Да", "Да", 3)}, nil},
		{"two separators", []model.Line{v169TestSeparator(t, "---", 3, false), v169TestText(t, "Да", "Да", 3), v169TestSeparator(t, "---", 3, false)}, nil},
		{"escaped separator only", []model.Line{v169TestText(t, `\---`, "---", 3)}, nil},
		{"separator with spaces and tabs", []model.Line{v169TestSeparator(t, " \t---\t ", 3, false)}, []v169TestWant{{[4]int{3, 1, 8, 2}, [][3]int{{6, 1, 2}}}}},
		{"NBSP does not trim from separator", []model.Line{v169TestText(t, "\u00a0---", "\u00a0---", 3)}, nil},
		{"lower orphan answer does not cancel", []model.Line{v169TestSeparator(t, "---", 3, false), v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "Нет", "Нет", 3)}, []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}},
		{"nested separator only", []model.Line{v169TestOpen(t, "hint", "", "", 3), v169TestText(t, "---", "---", 7), v169TestClose(t, 7)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "B", "B", 3)}
			v169TestRun(t, v169TestBlockDocument(t, "matching", append(body, tc.tail...)...), "current", tc.want)
		})
	}
}

func TestV169ContextsAndCoordinates(t *testing.T) {
	t.Run("case indent and Unicode", func(t *testing.T) {
		opening := v169TestLine(t, " \t@MATCHING {", "block-start", 2,
			v169TestElement(t, "tag", "@MATCHING", "matching", 3), v169TestElement(t, "block-open", "{", "{", 13))
		closing := v169TestLine(t, "\t} ", "block-end", 3, v169TestElement(t, "block-close", "}", "}", 2))
		result := v169TestDocument(t, opening, v169TestText(t, "A", "A", 3), v169TestTag(t, "AnSwEr", "Да", "content", 3),
			v169TestText(t, " \tЛуна — звезда? ", " \tЛуна — звезда? ", 3), v169TestSeparator(t, "---", 3, false),
			v169TestText(t, "Да", "Да", 3), v169TestText(t, "Нет", "Нет", 3), closing)
		v169TestRun(t, result, "current", []v169TestWant{{[4]int{3, 3, 10, 3}, [][3]int{{6, 1, 18}}}})
	})
	t.Run("nested in task", func(t *testing.T) {
		result := v169TestDocument(t, v169TestTag(t, "task", "", "", 2), v169TestOpen(t, "matching", "", "", 3),
			v169TestText(t, "A", "A", 4), v169TestTag(t, "answer", "Да", "content", 4), v169TestText(t, "B", "B", 4),
			v169TestSeparator(t, "---", 4, false), v169TestText(t, "Да", "Да", 4), v169TestText(t, "Нет", "Нет", 4),
			v169TestClose(t, 4), v169TestTag(t, "endtask", "", "", 3))
		v169TestRun(t, result, "current", []v169TestWant{{[4]int{4, 1, 11, 2}, [][3]int{{7, 1, 2}}}})
	})
	t.Run("two independent blocks", func(t *testing.T) {
		result := v169TestDocument(t,
			v169TestOpen(t, "matching", "", "", 2), v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "B", "B", 3),
			v169TestSeparator(t, "---", 3, false), v169TestText(t, "Да", "Да", 3), v169TestText(t, "Нет", "Нет", 3), v169TestClose(t, 3),
			v169TestOpen(t, "matching", "", "", 2), v169TestText(t, "C", "C", 11), v169TestTag(t, "answer", "Да", "content", 11), v169TestText(t, "D", "D", 11),
			v169TestSeparator(t, "---", 11, false), v169TestText(t, "Да", "Да", 11), v169TestText(t, "Нет", "Нет", 11), v169TestClose(t, 11))
		v169TestRun(t, result, "current", []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}, {[4]int{11, 1, 18, 2}, [][3]int{{14, 1, 2}}}})
	})
	for _, name := range []string{"choice", "multichoice", "example", "table"} {
		t.Run("other container "+name, func(t *testing.T) {
			v169TestRun(t, v169TestBlockDocument(t, name, v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Да", "content", 3),
				v169TestText(t, "B", "B", 3), v169TestSeparator(t, "---", 3, false), v169TestText(t, "Да", "Да", 3)), "current", nil)
		})
	}
	t.Run("literal declarations", func(t *testing.T) {
		v169TestRun(t, v169TestBlockDocument(t, "editor", v169TestText(t, "@matching {", "@matching {", 3),
			v169TestText(t, "A", "A", 3), v169TestText(t, "@answer Да", "@answer Да", 3), v169TestText(t, "B", "B", 3),
			v169TestText(t, "---", "---", 3), v169TestText(t, `\}`, "}", 3)), "current", nil)
	})
}

func TestV169BoundariesAndHistory(t *testing.T) {
	partial := func() *model.Result {
		return v169TestMatching(t, v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "B", "B", 3))
	}
	want := []v169TestWant{{[4]int{3, 1, 10, 2}, [][3]int{{6, 1, 2}}}}
	for _, ending := range []string{"\n", "\r\n", ""} {
		t.Run("ending "+strconv.Quote(ending), func(t *testing.T) {
			result := partial()
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(ending)
				}
			}
			v169TestRun(t, result, "first", want)
			v169TestRun(t, result, "second", want)
			v169TestRun(t, v169TestMatching(t, v169TestText(t, "A", "A", 3)), "other", nil)
		})
	}
	t.Run("existing diagnostics do not control check", func(t *testing.T) {
		result := v169TestMatching(t, v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Missing", "content", 3), v169TestText(t, "B", "B", 3))
		v169TestParserError(t, result, 5, 1, "Ответ отсутствует в нижнем списке.")
		v169TestRun(t, result, "current", want)
	})
	t.Run("unclosed matching", func(t *testing.T) {
		v169TestRun(t, v169TestDocument(t, v169TestOpen(t, "matching", "", "", 2), v169TestText(t, "A", "A", 3),
			v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "B", "B", 3), v169TestSeparator(t, "---", 3, false)), "current", nil)
	})
	t.Run("empty document", func(t *testing.T) {
		result := v169TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v169TestRun(t, result, "current", nil)
	})
}

func TestV169AmbiguousBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]model.Line) []model.Line
	}{
		{"duplicate matching close", func(body []model.Line) []model.Line {
			return append(body, v169TestClose(t, 3))
		}},
		{"foreign line within matching", func(body []model.Line) []model.Line {
			body[3] = v169TestText(t, "B", "B", 2)
			return body
		}},
		{"direct descendant after close", func(body []model.Line) []model.Line {
			return append(body, v169TestText(t, "Late", "Late", 3))
		}},
		{"indirect descendant after close", func(body []model.Line) []model.Line {
			return append(body, v169TestText(t, "Late", "Late", 6))
		}},
		{"unclosed nested block", func(body []model.Line) []model.Line {
			return slices.Insert(body, 4, v169TestOpen(t, "hint", "", "", 3))
		}},
		{"duplicate nested close", func(body []model.Line) []model.Line {
			return slices.Insert(body, 4, v169TestOpen(t, "hint", "", "", 3), v169TestClose(t, 7), v169TestClose(t, 7))
		}},
		{"nested close crosses matching close", func(body []model.Line) []model.Line {
			body = slices.Insert(body, 4, v169TestOpen(t, "hint", "", "", 3))
			return append(body, v169TestClose(t, 7))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v169TestOpen(t, "matching", "", "", 2),
				v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "B", "B", 3),
				v169TestSeparator(t, "---", 3, false), v169TestText(t, "Да", "Да", 3), v169TestText(t, "Нет", "Нет", 3), v169TestClose(t, 3)}
			v169TestRun(t, v169TestDocument(t, tc.change(body)...), "current", nil)
		})
	}
}

func TestV169NestedAndFollowingBlocks(t *testing.T) {
	result := v169TestDocument(t,
		v169TestOpen(t, "matching", "", "", 2),
		v169TestText(t, "A", "A", 3), v169TestTag(t, "answer", "Да", "content", 3), v169TestText(t, "B", "B", 3),
		v169TestSeparator(t, "---", 3, false), v169TestText(t, "Да", "Да", 3), v169TestText(t, "Нет", "Нет", 3),
		v169TestOpen(t, "matching", "", "", 3),
		v169TestText(t, "C", "C", 10), v169TestTag(t, "answer", "Да", "content", 10), v169TestText(t, "D", "D", 10),
		v169TestSeparator(t, "---", 10, false), v169TestText(t, "Да", "Да", 10), v169TestText(t, "Нет", "Нет", 10),
		v169TestClose(t, 10), v169TestClose(t, 3),
		v169TestOpen(t, "matching", "", "", 2),
		v169TestText(t, "E", "E", 19), v169TestTag(t, "answer", "Да", "content", 19), v169TestText(t, "F", "F", 19),
		v169TestSeparator(t, "---", 19, false), v169TestText(t, "Да", "Да", 19), v169TestText(t, "Нет", "Нет", 19), v169TestClose(t, 19))
	v169TestRun(t, result, "current", []v169TestWant{
		{[4]int{3, 1, 18, 2}, [][3]int{{6, 1, 2}}},
		{[4]int{10, 1, 17, 2}, [][3]int{{13, 1, 2}}},
		{[4]int{19, 1, 26, 2}, [][3]int{{22, 1, 2}}},
	})
}

package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v130Expected задаёт точную конкретизацию и связанные диапазоны [start,end).
// Тройка координат содержит строку, начальную и исключающую конечную колонки.
type v130Expected struct {
	detail  string
	related [][3]int
}

func TestV130Registration(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V130" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "document" || definition.Check == nil ||
			definition.Message != "Идентификатор фрагмента повторяется в документе." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V130 definitions = %d, want 1", count)
	}
}

func TestV130ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, second, fragment string
		includeG               bool
		want                   []v130Expected
	}{
		{"reference", "g", "@fragment f {\nFirst text.\n}\n@fragment g {\nSecond text.\n}\n@include f\n@include g", true, nil},
		{"example", "F", "@fragment f {\nFirst text.\n}\n@fragment F {\nSecond text.\n}\n@include f", false,
			[]v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{
				v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "First text.", "First text.", 3, 2), v130TestClose(t, 3, 2),
				v130TestFragment(t, tc.second, tc.second, 2, 1), v130TestText(t, "Second text.", "Second text.", 6, 2), v130TestClose(t, 6, 2),
				v130TestTag(t, "include", "identifier", "f", 2, 1),
			}
			if tc.includeG {
				body = append(body, v130TestTag(t, "include", "identifier", "g", 2, 1))
			}
			result := v130TestDocument(t, body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v130TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV130GroupsAndUnicode(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
		want []v130Expected
	}{
		{"no definitions", nil, nil}, {"one definition", []string{"f"}, nil},
		{"different IDs", []string{"f", "g"}, nil},
		{"two definitions", []string{"f", "F"}, []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
		{"three definitions", []string{"f", "F", "f"}, []v130Expected{{"ID: «f». Число определений: 3.", [][3]int{{3, 11, 12}, {6, 11, 12}, {9, 11, 12}}}}},
		{"two groups", []string{"f", "g", "F", "G"}, []v130Expected{
			{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {9, 11, 12}}},
			{"ID: «g». Число определений: 2.", [][3]int{{6, 11, 12}, {12, 11, 12}}},
		}},
		{"source order not lexical or second occurrence order", []string{"z", "a", "A", "Z"}, []v130Expected{
			{"ID: «z». Число определений: 2.", [][3]int{{3, 11, 12}, {12, 11, 12}}},
			{"ID: «a». Число определений: 2.", [][3]int{{6, 11, 12}, {9, 11, 12}}},
		}},
		{"Unicode case", []string{"КОД", "код"}, []v130Expected{{"ID: «код». Число определений: 2.", [][3]int{{3, 11, 14}, {6, 11, 14}}}}},
		{"Unicode decimal digits", []string{"١٢", "١٢"}, []v130Expected{{"ID: «١٢». Число определений: 2.", [][3]int{{3, 11, 13}, {6, 11, 13}}}}},
		{"punctuation in ID", []string{"a_1.2-b", "A_1.2-B"}, []v130Expected{{"ID: «a_1.2-b». Число определений: 2.", [][3]int{{3, 11, 18}, {6, 11, 18}}}}},
		{"dot", []string{".", "."}, []v130Expected{{"ID: «.». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
		{"hyphen", []string{"-", "-"}, []v130Expected{{"ID: «-». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
		{"underscore", []string{"_", "_"}, []v130Expected{{"ID: «_». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
		{"supplementary Unicode letters", []string{"𐐀", "𐐨"}, []v130Expected{{"ID: «𐐨». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
		{"no width normalization", []string{"A", "Ａ"}, nil},
		{"no full case folding", []string{"ß", "SS"}, nil},
		{"no final sigma folding", []string{"Σ", "ς"}, nil},
		{"lowercase sigma", []string{"Σ", "σ"}, []v130Expected{{"ID: «σ». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v130TestRun(t, v130TestFragments(t, tc.ids...), "current", tc.want) })
	}
}

func TestV130InvalidIDs(t *testing.T) {
	for _, id := range []string{"", " f", "f ", "f g", "f\tg", "f\u00a0g", "bad!", "a/b", "²", "Ⅳ", "e\u0301", `f\_g`} {
		t.Run(strconv.Quote(id), func(t *testing.T) {
			result := v130TestFragments(t, id, id)
			v130TestError(t, result, 3, 1, "Недопустимый ID фрагмента.")
			v130TestError(t, result, 6, 1, "Недопустимый ID фрагмента.")
			v130TestRun(t, result, "current", nil)
		})
	}
	result := v130TestFragments(t, "f", "bad!", "F")
	v130TestError(t, result, 6, 1, "Недопустимый ID фрагмента.")
	v130TestRun(t, result, "intervening", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {9, 11, 12}}}})
	result = v130TestFragments(t, " f", "f", "F")
	v130TestError(t, result, 3, 1, "Недопустимый ID фрагмента.")
	v130TestRun(t, result, "not-trimmed", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{6, 11, 12}, {9, 11, 12}}}})
}

// v130TestDocument добавляет обязательные версию и заголовок по DSL 1.30.
// Родители заданы тестами явно; ошибки unparsed получают исходные ссылки.
func v130TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v130TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v130TestHeading(t, "#", "1", "Lesson"),
	}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}}, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for index := range result.Lines {
		for element, value := range result.Lines[index].Elements {
			if value.ElementType == "unparsed" {
				v130TestError(t, result, index+1, element, "Ошибка разбора объявления.")
			}
		}
	}
	return result
}

// v130TestFragments строит полные определения, не вычисляя ожидаемые результаты.
func v130TestFragments(t *testing.T, ids ...string) *model.Result {
	t.Helper()
	var body []model.Line
	for _, id := range ids {
		parent := len(body) + 3
		body = append(body, v130TestFragment(t, id, id, 2, 1), v130TestText(t, "Text.", "Text.", parent, 2), v130TestClose(t, parent, 2))
	}
	return v130TestDocument(t, body...)
}

// v130TestLine сохраняет явные тип, родителей и элементы без разбора исходника.
func v130TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v130TestElement задаёт исходное и семантическое значения независимо.
func v130TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v130TestUnparsed создаёт неразобранный фрагмент с null Value.
func v130TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v130TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v130TestFragment сохраняет полный ID, в том числе ошибочный, без обрезки.
func v130TestFragment(t *testing.T, rawID, value string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(rawID)
	return v130TestLine(t, "@fragment "+rawID+" {", "block-start", parent, depth,
		v130TestElement(t, "tag", "@fragment", "fragment", 1, 10), v130TestElement(t, "identifier", rawID, value, 11, end),
		v130TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v130TestTag создаёт однострочное объявление с одним параметром.
func v130TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v130TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v130TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v130TestLine(t, raw, "tag", parent, depth, elements...)
}

// v130TestOpen создаёт блок без параметров.
func v130TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v130TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v130TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v130TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v130TestClose создаёт закрытие с явно указанным родителем.
func v130TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v130TestLine(t, "}", "block-end", parent, depth, v130TestElement(t, "block-close", "}", "}", 1, 2))
}

// v130TestText не преобразует Raw и Value друг в друга.
func v130TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v130TestLine(t, raw, "content", parent, depth, v130TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v130TestHeading создаёт корневой заголовок с заданным уровнем.
func v130TestHeading(t *testing.T, marker, level, title string) model.Line {
	t.Helper()
	return v130TestLine(t, marker+" "+title, "heading", 0, 0,
		v130TestElement(t, "heading-level", marker, level, 1, len(marker)+1),
		v130TestElement(t, "title", title, title, len(marker)+2, len(marker)+2+utf8.RuneCountInString(title)))
}

// v130TestError добавляет исходную ошибку; index=-1 задаёт область line.
// Все исходные ошибки parser упорядочиваются по позиции до запуска конвейера.
func v130TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
}

// v130TestRun использует весь штатный каталог и сравнивает только новые V130.
func v130TestRun(t *testing.T, result *model.Result, source string, want []v130Expected) {
	t.Helper()
	before := v130TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v130TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics[len(original.Diagnostics):] {
		if diagnostic.DiagnosticCode == "V130" {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V130 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		diagnostic := got[index]
		if diagnostic.Location != nil || diagnostic.SeverityLevel != "error" || diagnostic.DiagnosticScope != "document" || diagnostic.Fatal ||
			diagnostic.Source != source || diagnostic.ID == "" || diagnostic.Message != "Идентификатор фрагмента повторяется в документе.\nконкретизация: "+expected.detail ||
			len(diagnostic.RelatedLocations) != len(expected.related) {
			t.Fatalf("occurrence %d = %+v, want %+v with no primary position and exact metadata", index, diagnostic, expected)
		}
		for related, position := range expected.related {
			location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
			if diagnostic.RelatedLocations[related] != location {
				t.Fatalf("related %d = %+v, want %+v", related, diagnostic.RelatedLocations[related], location)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == diagnostic.ID {
						t.Fatal("document diagnostic created an element error link")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v130TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source shape changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for index := range restored.Lines {
		if len(restored.Lines[index].Elements) != len(original.Lines[index].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[index].HasErrors = original.Lines[index].HasErrors
		for element := range restored.Lines[index].Elements {
			restored.Lines[index].Elements[element].ErrorIDs = original.Lines[index].Elements[element].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v130TestSnapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

// v130TestSnapshot сохраняет модель для проверки неизменности исходника и истории.
func v130TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV130DocumentWideGroups(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v130Expected
	}{
		{"different steps", []model.Line{
			v130TestHeading(t, "###", "3", "First"), v130TestFragment(t, "f", "f", 3, 1),
			v130TestText(t, "First text.", "First text.", 4, 2), v130TestClose(t, 4, 2),
			v130TestHeading(t, "###", "3", "Second"), v130TestFragment(t, "F", "F", 7, 1),
			v130TestText(t, "Second text.", "Second text.", 8, 2), v130TestClose(t, 8, 2),
		}, []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{4, 11, 12}, {8, 11, 12}}}}},
		{"different tasks", []model.Line{
			v130TestTag(t, "task", "identifier", "first", 2, 1), v130TestFragment(t, "f", "f", 3, 2),
			v130TestText(t, "First text.", "First text.", 4, 3), v130TestClose(t, 4, 3),
			v130TestTag(t, "task", "identifier", "second", 2, 1), v130TestFragment(t, "F", "F", 7, 2),
			v130TestText(t, "Second text.", "Second text.", 8, 3), v130TestClose(t, 8, 3),
		}, []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{4, 11, 12}, {8, 11, 12}}}}},
		{"all branches", []model.Line{
			v130TestOpen(t, "variants", 2, 1), v130TestTag(t, "variant", "name", "First", 3, 2),
			v130TestFragment(t, "f", "f", 4, 3), v130TestText(t, "First text.", "First text.", 5, 4), v130TestClose(t, 5, 4),
			v130TestTag(t, "variant", "name", "Second", 3, 2), v130TestFragment(t, "F", "F", 8, 3),
			v130TestText(t, "Second text.", "Second text.", 9, 4), v130TestClose(t, 9, 4), v130TestClose(t, 3, 2),
		}, []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{5, 11, 12}, {9, 11, 12}}}}},
		{"blank and newpage do not reset groups", []model.Line{
			v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "First text.", "First text.", 3, 2), v130TestClose(t, 3, 2),
			v130TestLine(t, " \t", "blank", 2, 1), v130TestTag(t, "newpage", "", "", 2, 1),
			v130TestFragment(t, "F", "F", 2, 1), v130TestText(t, "Second text.", "Second text.", 8, 2), v130TestClose(t, 8, 2),
		}, []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {8, 11, 12}}}}},
		{"include does not create definitions", []model.Line{
			v130TestTag(t, "include", "identifier", "f", 2, 1),
			v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "Text.", "Text.", 4, 2), v130TestClose(t, 4, 2),
			v130TestTag(t, "include", "identifier", "F", 2, 1), v130TestTag(t, "include", "identifier", "f", 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v130TestRun(t, v130TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV130SeparateIDSpacesAndLiteralText(t *testing.T) {
	// document-id находится до заголовка, task и fragment имеют одинаковый ID.
	base := v130TestDocument(t)
	id, count := "f", 8
	base.Document.Metadata.DocumentID, base.Document.LineCount = &id, &count
	base.Lines = []model.Line{base.Lines[0], v130TestTag(t, "document-id", "identifier", "f", 0, 0), base.Lines[1],
		v130TestTag(t, "task", "identifier", "f", 3, 1), v130TestFragment(t, "f", "f", 4, 2),
		v130TestText(t, "Text.", "Text.", 5, 3), v130TestClose(t, 5, 3), v130TestTag(t, "endtask", "", "", 3, 1)}
	for index := range base.Lines {
		base.Lines[index].Number, base.Lines[index].LineEnding = index+1, "\n"
	}
	base.Lines[7].LineEnding = ""
	v130TestRun(t, base, "separate-spaces", nil)
	for _, tag := range []string{"editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			v130TestRun(t, v130TestDocument(t, v130TestOpen(t, tag, 2, 1),
				v130TestText(t, "@fragment f {", "@fragment f {", 3, 2), v130TestText(t, "Literal text.", "Literal text.", 3, 2), v130TestClose(t, 3, 2),
				v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "Real text.", "Real text.", 7, 2), v130TestClose(t, 7, 2)), "current", nil)
		})
	}
	v130TestRun(t, v130TestDocument(t,
		v130TestText(t, `\@fragment f \{`, "@fragment f {", 2, 1),
		v130TestText(t, "See @fragment f.", "See @fragment f.", 2, 1),
		v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "Text.", "Text.", 5, 2), v130TestClose(t, 5, 2)), "pseudo", nil)
}

func TestV130ErroneousDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opening model.Line
		want    []v130Expected
	}{
		{"recognized ID and unparsed extra", v130TestLine(t, "@fragment F extra {", "block-start", 2, 1,
			v130TestElement(t, "tag", "@fragment", "fragment", 1, 10), v130TestElement(t, "identifier", "F", "F", 11, 12),
			v130TestUnparsed(t, "extra", 13, 18), v130TestElement(t, "block-open", "{", "{", 19, 20)),
			[]v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}},
		{"missing ID", v130TestLine(t, "@fragment {", "block-start", 2, 1,
			v130TestElement(t, "tag", "@fragment", "fragment", 1, 10), v130TestElement(t, "block-open", "{", "{", 11, 12)), nil},
		{"unparsed ID is not recovered", v130TestLine(t, "@fragment F {", "block-start", 2, 1,
			v130TestElement(t, "tag", "@fragment", "fragment", 1, 10), v130TestUnparsed(t, "F", 11, 12),
			v130TestElement(t, "block-open", "{", "{", 13, 14)), nil},
		{"unparsed declaration is not recovered", v130TestLine(t, "@fragment F {", "block-start", 2, 1,
			v130TestUnparsed(t, "@fragment F", 1, 12), v130TestElement(t, "block-open", "{", "{", 13, 14)), nil},
		{"multiple ID candidates", v130TestLine(t, "@fragment f g {", "block-start", 2, 1,
			v130TestElement(t, "tag", "@fragment", "fragment", 1, 10), v130TestElement(t, "identifier", "f", "f", 11, 12),
			v130TestElement(t, "identifier", "g", "g", 13, 14), v130TestElement(t, "block-open", "{", "{", 15, 16)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v130TestDocument(t, v130TestFragment(t, "f", "f", 2, 1),
				v130TestText(t, "First text.", "First text.", 3, 2), v130TestClose(t, 3, 2), tc.opening,
				v130TestText(t, "Second text.", "Second text.", 6, 2), v130TestClose(t, 6, 2))
			v130TestError(t, result, 6, -1, "Ошибка объявления фрагмента.")
			v130TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("invalid line retains unambiguous ID", func(t *testing.T) {
		result := v130TestDocument(t, v130TestFragment(t, "f", "f", 2, 1),
			v130TestText(t, "Text.", "Text.", 3, 2), v130TestClose(t, 3, 2),
			v130TestLine(t, "@fragment F ?", "invalid", 2, 1,
				v130TestElement(t, "tag", "@fragment", "fragment", 1, 10), v130TestElement(t, "identifier", "F", "F", 11, 12),
				v130TestUnparsed(t, "?", 13, 14)))
		v130TestRun(t, result, "current", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}})
	})
	t.Run("invalid line has no identifier", func(t *testing.T) {
		result := v130TestDocument(t, v130TestFragment(t, "f", "f", 2, 1),
			v130TestText(t, "Text.", "Text.", 3, 2), v130TestClose(t, 3, 2),
			v130TestLine(t, "@fragment F", "invalid", 2, 1, v130TestUnparsed(t, "@fragment F", 1, 12)))
		v130TestRun(t, result, "current", nil)
	})
	t.Run("placement does not exclude a real definition", func(t *testing.T) {
		result := v130TestDocument(t, v130TestFragment(t, "f", "f", 2, 1),
			v130TestText(t, "Outer text.", "Outer text.", 3, 2), v130TestFragment(t, "F", "F", 3, 2),
			v130TestText(t, "Inner text.", "Inner text.", 5, 3), v130TestClose(t, 5, 3), v130TestClose(t, 3, 2))
		v130TestError(t, result, 5, -1, "Определение фрагмента внутри другого блока запрещено.")
		v130TestRun(t, result, "current", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {5, 11, 12}}}})
	})
}

func TestV130SourcePositionsAndValues(t *testing.T) {
	result := v130TestFragments(t, "F", "f")
	result.Lines[2] = v130TestLine(t, "  @FrAgMeNt   F {", "block-start", 2, 1,
		v130TestElement(t, "tag", "@FrAgMeNt", "fragment", 3, 12), v130TestElement(t, "identifier", "F", "F", 15, 16),
		v130TestElement(t, "block-open", "{", "{", 17, 18))
	result.Lines[2].Number = 3
	v130TestRun(t, result, "indented", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 15, 16}, {6, 11, 12}}}})
	// Модель может хранить ID в нижнем регистре по DSL 1.10; диапазон остаётся исходным.
	result = v130TestDocument(t, v130TestFragment(t, "КОД", "код", 2, 1),
		v130TestText(t, "Text.", "Text.", 3, 2), v130TestClose(t, 3, 2), v130TestFragment(t, "код", "код", 2, 1),
		v130TestText(t, "Text.", "Text.", 6, 2), v130TestClose(t, 6, 2))
	v130TestRun(t, result, "semantic", []v130Expected{{"ID: «код». Число определений: 2.", [][3]int{{3, 11, 14}, {6, 11, 14}}}})
	// Неверный разделитель не делает выделенный ID неоднозначным.
	result = v130TestFragments(t, "f", "F")
	result.Lines[5] = v130TestLine(t, "\t@fragment\tF{", "block-start", 2, 1,
		v130TestElement(t, "tag", "@fragment", "fragment", 2, 11), v130TestElement(t, "identifier", "F", "F", 12, 13),
		v130TestElement(t, "block-open", "{", "{", 13, 14))
	result.Lines[5].Number = 6
	v130TestError(t, result, 6, -1, "Неверные разделители объявления.")
	v130TestRun(t, result, "separator", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 12, 13}}}})
}

func TestV130EndingsHistoryAndVersions(t *testing.T) {
	for _, ending := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(ending, func(t *testing.T) {
			result := v130TestDocument(t,
				v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "First text.", "First text.", 3, 2), v130TestClose(t, 3, 2),
				v130TestFragment(t, "F", "F", 2, 1), v130TestText(t, "Second text.", "Second text.", 6, 2), v130TestClose(t, 6, 2),
				v130TestTag(t, "include", "identifier", "f", 2, 1), v130TestTag(t, "include", "identifier", "F", 2, 1))
			for index := range result.Lines {
				switch ending {
				case "LF":
					result.Lines[index].LineEnding = "\n"
				case "CRLF":
					result.Lines[index].LineEnding = "\r\n"
				case "mixed":
					if index%2 == 0 {
						result.Lines[index].LineEnding = "\r\n"
					}
				}
			}
			want := []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}}
			v130TestRun(t, result, "first", want)
			v130TestRun(t, result, "second", want)
			v130TestRun(t, v130TestFragments(t, "f", "g"), "separate", nil)
		})
	}
	t.Run("last definition at EOF", func(t *testing.T) {
		result := v130TestDocument(t, v130TestFragment(t, "f", "f", 2, 1), v130TestText(t, "Text.", "Text.", 3, 2),
			v130TestClose(t, 3, 2), v130TestFragment(t, "F", "F", 2, 1))
		v130TestError(t, result, 6, -1, "Блок не закрыт до конца файла.")
		v130TestRun(t, result, "current", []v130Expected{{"ID: «f». Число определений: 2.", [][3]int{{3, 11, 12}, {6, 11, 12}}}})
	})
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v130TestRun(t, result, "empty", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v130TestFragments(t, "f", "F")
			result.Document.DSLVersion = &version
			result.Lines[0] = v130TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			before := v130TestSnapshot(t, result)
			registry, err := diagnostics.NewRegistry(RegistrationCheckers())
			if err != nil {
				t.Fatal(err)
			}
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil || v130TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

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

const v182TestMessage = "Значение @answer отсутствует среди позиций нижнего списка @ordering."

// v182TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v182TestWant struct {
	location [3]int
	related  [][3]int
}

func TestV182RegistrationAndVersions(t *testing.T) {
	registry := v182TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V182" {
			continue
		}
		count++
		if definition.Message != v182TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V182 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v182TestDocument(t)
			result.Document.DSLVersion = &version
			before := v182TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v182TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v182TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v182TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v182TestLine(t, raw, "invalid", parent, v182TestElement(t, "unparsed", raw, "", 1))
	}
	return v182TestLine(t, raw, "separator", parent)
}

// v182TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v182TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v182TestOpen(t, name, "", "", 2)}, body...)
	return v182TestDocument(t, append(lines, v182TestClose(t, 3))...)
}

// v182TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v182TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v182TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v182TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v182TestTag задаёт известное имя и явно указанный тип аргумента.
func v182TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v182TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v182TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v182TestLine(t, raw, "tag", parent, elements...)
}

// v182TestOpen добавляет открытие к известному объявлению блока.
func v182TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v182TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v182TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v182TestClose задаёт закрытие конкретного фигурного блока.
func v182TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v182TestLine(t, "}", "block-end", parent, v182TestElement(t, "block-close", "}", "}", 1))
}

// v182TestText сохраняет исходное и семантическое содержимое независимо.
func v182TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v182TestLine(t, raw, "content", parent, v182TestElement(t, "content", raw, value, 1))
}

// v182TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v182TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v182TestTag(t, "dsl-version", "1.2", "version", 0),
		v182TestLine(t, "# Lesson", "heading", 0, v182TestElement(t, "heading-level", "#", "1", 1),
			v182TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v182TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v182TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v182TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v182TestRegistry использует полный штатный каталог без отдельной регистрации.
func v182TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v182TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v182TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v182TestRun сравнивает новые V182 после полного Prepare/Run с явным ожиданием.
func v182TestRun(t *testing.T, result *model.Result, processingID string, want []v182TestWant) {
	t.Helper()
	before := v182TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v182TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(checks) != 198 {
		t.Fatalf("catalogue size = %d, want 198", len(checks))
	}
	if v182TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V182" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V182 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v182TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V182 occurrence %d = %+v, want %+v, no detail", i, entry, expected)
		}
		for j, r := range expected.related {
			related := model.Location{Start: model.Position{Line: r[0], Column: r[1]}, End: model.Position{Line: r[0], Column: r[2]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("related[%d] = %+v, want %+v", j, entry.RelatedLocations[j], related)
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V182 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V182 created an element error link")
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
	if err := json.Unmarshal([]byte(v182TestSnapshot(t, result)), &restored); err != nil {
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
	if v182TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

// v182TestPair задаёт два элемента с ответами на строках 5 и 7, разделитель на 8.
// Ответ второго элемента — Last; нижние строки передаются явно начиная со строки 9.
func v182TestPair(t *testing.T, answer model.Line, lower ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v182TestText(t, "A", "A", 3), answer, v182TestText(t, "B", "B", 3),
		v182TestTag(t, "answer", "Last", "content", 3), v182TestSeparator(t, "---", 3, false)}
	return v182TestBlockDocument(t, "ordering", append(body, lower...)...)
}

func TestV182ReferenceAndExample(t *testing.T) {
	// Только версия и обязательный заголовок добавлены перед исходными фрагментами.
	for _, tc := range []struct {
		name, answer, fragment string
		want                   []v182TestWant
	}{
		{"reference", "first", "@ordering {\nA\n@answer first\nB\n@answer Last\n---\nFirst\nLast\n}", nil},
		{"example", "1", "@ordering {\nA\n@answer 1\nB\n@answer Last\n---\nFirst\nLast\n}", []v182TestWant{{[3]int{5, 9, 10}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v182TestPair(t, v182TestTag(t, "answer", tc.answer, "content", 3), v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v182TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV182ExactValues(t *testing.T) {
	for _, tc := range []struct {
		name, answer, position string
		end                    int
	}{
		{"numeric label", "1", "1", 0},
		{"label is not an ordinal", "1", "First", 10},
		{"numbers in text retained", "2. First", "First", 17},
		{"Unicode case", "ПЕРВЫЙ", "первый", 0},
		{"Unicode case variants", "Σ", "ς", 0},
		{"two interior spaces", "First  place", "First place", 21},
		{"interior tab", "First\tplace", "First place", 20},
		{"same interior spaces", "First  place", "FIRST  PLACE", 0},
		{"punctuation", "First!", "First", 15},
		{"punctuation only", "!", "?", 10},
		{"same punctuation", "!", "!", 0},
		{"no Unicode composition", "e\u0301", "é", 11},
		{"edge NBSP preserved", "\u00a0First", "First", 15},
		{"zero width space is not empty", "\u200b", "First", 10},
		{"no alternatives splitting", "First/Last;Other", "First", 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v182TestPair(t, v182TestTag(t, "answer", tc.answer, "content", 3), v182TestText(t, tc.position, tc.position, 3), v182TestText(t, "Last", "Last", 3))
			var want []v182TestWant
			if tc.end != 0 {
				want = []v182TestWant{{[3]int{5, 9, tc.end}, nil}}
			}
			v182TestRun(t, result, "current", want)
		})
	}
	for _, empty := range []string{"", " \t ", "\u00a0", "\u2003\u202f\u0085"} {
		t.Run("empty "+strconv.Quote(empty), func(t *testing.T) {
			v182TestRun(t, v182TestPair(t, v182TestTag(t, "answer", empty, "content", 3),
				v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3)), "current", nil)
		})
	}
}

func TestV182EscapeAndRawFallback(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value, lowerRaw, lowerValue string
		end                                    int
	}{
		{"braces", `\{X\}`, `{X}`, `\{X\}`, `{X}`, 0},
		{"missing escaped value source span", `\{X\}`, `{X}`, "First", "First", 14},
		{"backslash", `\\`, `\`, `\\`, `\`, 0},
		{"unknown escape preserved", `\q`, `\q`, `\q`, `\q`, 0},
		{"no second decoding", `\\\{X\}`, `\{X}`, `\{X\}`, `{X}`, 16},
		{"literal at start of lower line", "@X", "@X", `\@X`, "@X", 0},
		{"literal hash at start of lower line", "#X", "#X", `\#X`, "#X", 0},
		{"inline at is not initial markup", `\@X`, `\@X`, `\@X`, "@X", 12},
		{"escaped lower separator", "---", "---", `\---`, "---", 0},
		{"inline separator is ordinary data", `\---`, `\---`, `\---`, "---", 13},
		{"underscores not special here", `\_____`, `\_____`, `\_____`, `\_____`, 0},
		{"quotes are preserved", `"First"`, `"First"`, "First", "First", 16},
		{"syntactic edge spaces", " \tFirst \t", "First", " \tFirst \t", "First", 0},
	} {
		for _, parsed := range []bool{true, false} {
			t.Run(tc.name+"/Value="+strconv.FormatBool(parsed), func(t *testing.T) {
				answer := v182TestTag(t, "answer", tc.raw, "content", 3)
				answer.Elements[1].Value = &tc.value
				lower := v182TestText(t, tc.lowerRaw, tc.lowerValue, 3)
				if !parsed {
					// Типы и связи уже известны. Не создаём недопустимый null content.Value:
					// проверяем разрешённое восстановление при отсутствии content.
					answer.Elements = answer.Elements[:1]
					lower.Elements = []model.Element{}
				}
				var want []v182TestWant
				if tc.end != 0 {
					want = []v182TestWant{{[3]int{5, 9, tc.end}, nil}}
				}
				v182TestRun(t, v182TestPair(t, answer, lower, v182TestText(t, "Last", "Last", 3)), "current", want)
			})
		}
	}
}

func TestV182BlockValuesAndGrouping(t *testing.T) {
	t.Run("group within one answer", func(t *testing.T) {
		result := v182TestBlockDocument(t, "ordering", v182TestText(t, "A", "A", 3), v182TestOpen(t, "answer", "", "", 3),
			v182TestText(t, "Missing", "Missing", 5), v182TestText(t, "MISSING", "MISSING", 5), v182TestText(t, "Other", "Other", 5), v182TestText(t, "Missing", "Missing", 5), v182TestClose(t, 5),
			v182TestText(t, "B", "B", 3), v182TestTag(t, "answer", "Last", "content", 3), v182TestSeparator(t, "---", 3, false),
			v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{6, 1, 8}, [][3]int{{7, 1, 8}, {9, 1, 8}}}, {[3]int{8, 1, 6}, nil}})
	})
	t.Run("different answers stay separate", func(t *testing.T) {
		result := v182TestBlockDocument(t, "ordering", v182TestText(t, "A", "A", 3), v182TestTag(t, "answer", "Missing", "content", 3),
			v182TestText(t, "B", "B", 3), v182TestTag(t, "answer", "MISSING", "content", 3), v182TestSeparator(t, "---", 3, false),
			v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{5, 9, 16}, nil}, {[3]int{7, 9, 16}, nil}})
	})
	for _, tc := range []struct {
		name   string
		values []model.Line
		want   []v182TestWant
	}{
		{"all values match", []model.Line{v182TestText(t, "first", "first", 5), v182TestText(t, "LAST", "LAST", 5)}, nil},
		{"empty block still supplies a linked answer", nil, nil},
		{"blank values skipped", []model.Line{v182TestLine(t, " \t", "blank", 5), v182TestText(t, "\u00a0", "\u00a0", 5)}, nil},
		{"lines not joined", []model.Line{v182TestText(t, "Fir", "Fir", 5), v182TestText(t, "st", "st", 5)}, []v182TestWant{{[3]int{6, 1, 4}, nil}, {[3]int{7, 1, 3}, nil}}},
		{"nested values excluded", []model.Line{v182TestOpen(t, "hint", "", "", 5), v182TestText(t, "Missing", "Missing", 6), v182TestClose(t, 6), v182TestText(t, "First", "First", 5)}, nil},
		{"unknown value does not hide known missing value", []model.Line{v182TestLine(t, "{", "invalid", 5, v182TestElement(t, "unparsed", "{", "", 1)), v182TestText(t, "Missing", "Missing", 5)}, []v182TestWant{{[3]int{7, 1, 8}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v182TestText(t, "A", "A", 3), v182TestOpen(t, "answer", "", "", 3)}
			body = append(body, tc.values...)
			body = append(body, v182TestClose(t, 5), v182TestText(t, "B", "B", 3), v182TestTag(t, "answer", "Last", "content", 3), v182TestSeparator(t, "---", 3, false),
				v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
			v182TestRun(t, v182TestBlockDocument(t, "ordering", body...), "current", tc.want)
		})
	}
}

func TestV182Eligibility(t *testing.T) {
	item := v182TestText(t, "A", "A", 3)
	bad := v182TestTag(t, "answer", "Missing", "content", 3)
	last := v182TestTag(t, "answer", "Last", "content", 3)
	separator := v182TestSeparator(t, "---", 3, false)
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v182TestWant
	}{
		{"short format excluded", []model.Line{item, bad, v182TestText(t, "B", "B", 3), last}, nil},
		{"two separators excluded", []model.Line{item, bad, separator, v182TestText(t, "First", "First", 3), separator}, nil},
		{"empty upper list", []model.Line{separator, v182TestText(t, "First", "First", 3)}, nil},
		{"none answered", []model.Line{item, v182TestText(t, "B", "B", 3), separator, v182TestText(t, "First", "First", 3)}, nil},
		{"last element not answered", []model.Line{item, bad, v182TestText(t, "B", "B", 3), separator, v182TestText(t, "First", "First", 3)}, nil},
		{"first element not answered", []model.Line{item, v182TestText(t, "B", "B", 3), bad, separator, v182TestText(t, "First", "First", 3)}, nil},
		{"editor breaks adjacency", []model.Line{item, v182TestTag(t, "editor", "Check.", "content", 3), bad, separator, v182TestText(t, "First", "First", 3)}, nil},
		{"foreign block breaks adjacency", []model.Line{item, v182TestOpen(t, "hint", "", "", 3), v182TestText(t, "Check.", "Check.", 5), v182TestClose(t, 5), bad, separator, v182TestText(t, "First", "First", 3)}, nil},
		{"blank rows preserve adjacency", []model.Line{item, v182TestLine(t, "", "blank", 3), v182TestLine(t, " \t ", "blank", 3), bad,
			v182TestText(t, "B", "B", 3), last, separator, v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3)}, []v182TestWant{{[3]int{7, 9, 16}, nil}}},
		{"nested upper text is not an item", []model.Line{item, bad, v182TestOpen(t, "hint", "", "", 3), v182TestText(t, "Nested", "Nested", 6), v182TestClose(t, 6),
			v182TestText(t, "B", "B", 3), last, separator, v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3)}, []v182TestWant{{[3]int{5, 9, 16}, nil}}},
		{"answers in lower list are not linked", []model.Line{item, v182TestText(t, "B", "B", 3), separator, bad, last}, nil},
		{"unknown upper row prevents complete association", []model.Line{item, bad, v182TestLine(t, "@?", "invalid", 3, v182TestElement(t, "unparsed", "@?", "", 1)), separator, v182TestText(t, "First", "First", 3)}, nil},
		{"nested separator does not select extended format", []model.Line{item, v182TestOpen(t, "answer", "", "", 3), v182TestText(t, "---", "---", 5), v182TestClose(t, 5)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v182TestRun(t, v182TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
}

func TestV182LowerPositions(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		lower        []model.Line
		want         []v182TestWant
	}{
		{"duplicate labels allow a match", "first", []model.Line{v182TestText(t, "First", "First", 3), v182TestText(t, "FIRST", "FIRST", 3), v182TestText(t, "Last", "Last", 3)}, nil},
		{"duplicate labels do not hide missing value", "Missing", []model.Line{v182TestText(t, "First", "First", 3), v182TestText(t, "FIRST", "FIRST", 3), v182TestText(t, "Last", "Last", 3)}, []v182TestWant{{[3]int{5, 9, 16}, nil}}},
		{"different list sizes allowed", "First", []model.Line{v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3), v182TestText(t, "Middle", "Middle", 3)}, nil},
		{"empty lower set", "First", nil, []v182TestWant{{[3]int{5, 9, 14}, nil}, {[3]int{7, 9, 13}, nil}}},
		{"invalid potential position cancels check", "Missing", []model.Line{v182TestText(t, "First", "First", 3), v182TestLine(t, "{", "invalid", 3, v182TestElement(t, "unparsed", "{", "", 1)), v182TestText(t, "Last", "Last", 3)}, nil},
		{"unparsed content cancels check", "Missing", []model.Line{v182TestLine(t, "?", "content", 3, v182TestElement(t, "unparsed", "?", "", 1)), v182TestText(t, "Last", "Last", 3)}, nil},
		{"uninterpretable raw fallback cancels check", "Missing", []model.Line{v182TestLine(t, "Unclosed {", "content", 3), v182TestText(t, "Last", "Last", 3)}, nil},
		{"nested unknown content does not cancel check", "Missing", []model.Line{v182TestOpen(t, "hint", "", "", 3), v182TestLine(t, "{", "invalid", 9, v182TestElement(t, "unparsed", "{", "", 1)), v182TestClose(t, 9), v182TestText(t, "Last", "Last", 3)}, []v182TestWant{{[3]int{5, 9, 16}, nil}}},
		{"nested value is not a position", "Missing", []model.Line{v182TestOpen(t, "hint", "", "", 3), v182TestText(t, "Missing", "Missing", 9), v182TestClose(t, 9), v182TestText(t, "Last", "Last", 3)}, []v182TestWant{{[3]int{5, 9, 16}, nil}}},
		{"inline tag content is not a position", "Missing", []model.Line{v182TestTag(t, "hint", "Missing", "content", 3), v182TestText(t, "Last", "Last", 3)}, []v182TestWant{{[3]int{5, 9, 16}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v182TestRun(t, v182TestPair(t, v182TestTag(t, "answer", tc.answer, "content", 3), tc.lower...), "current", tc.want)
		})
	}
}

func TestV182ContextsAndSourceRanges(t *testing.T) {
	t.Run("indent case and Unicode", func(t *testing.T) {
		answer := v182TestLine(t, " \t@AnSwEr 星", "tag", 3, v182TestElement(t, "tag", "@AnSwEr", "answer", 3), v182TestElement(t, "content", "星", "星", 11))
		result := v182TestPair(t, answer, v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
		result.Lines[2].Raw = "@OrDeRiNg {"
		result.Lines[2].Elements[0].Raw = "@OrDeRiNg"
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{5, 11, 12}, nil}})
	})
	t.Run("nested ordering", func(t *testing.T) {
		result := v182TestDocument(t, v182TestOpen(t, "fragment", "f", "identifier", 2), v182TestOpen(t, "ordering", "", "", 3),
			v182TestText(t, "A", "A", 4), v182TestTag(t, "answer", "1", "content", 4), v182TestText(t, "B", "B", 4), v182TestTag(t, "answer", "Last", "content", 4),
			v182TestSeparator(t, "---", 4, false), v182TestText(t, "First", "First", 4), v182TestText(t, "Last", "Last", 4), v182TestClose(t, 4), v182TestClose(t, 3), v182TestTag(t, "include", "f", "identifier", 2))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{6, 9, 10}, nil}})
	})
	t.Run("two independent blocks", func(t *testing.T) {
		result := v182TestDocument(t,
			v182TestOpen(t, "ordering", "", "", 2), v182TestText(t, "A", "A", 3), v182TestTag(t, "answer", "1", "content", 3), v182TestText(t, "B", "B", 3), v182TestTag(t, "answer", "Last", "content", 3),
			v182TestSeparator(t, "---", 3, false), v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3), v182TestClose(t, 3),
			v182TestOpen(t, "ordering", "", "", 2), v182TestText(t, "A", "A", 12), v182TestTag(t, "answer", "1", "content", 12), v182TestText(t, "B", "B", 12), v182TestTag(t, "answer", "Last", "content", 12),
			v182TestSeparator(t, "---", 12, false), v182TestText(t, "First", "First", 12), v182TestText(t, "Last", "Last", 12), v182TestClose(t, 12))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{5, 9, 10}, nil}, {[3]int{14, 9, 10}, nil}})
	})
	for _, container := range []string{"matching", "choice", "multichoice"} {
		t.Run("other container "+container, func(t *testing.T) {
			result := v182TestPair(t, v182TestTag(t, "answer", "Missing", "content", 3), v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
			opening := v182TestOpen(t, container, "", "", 2)
			opening.Number, opening.NestingLevel = 3, 1
			result.Lines[2] = opening
			v182TestRun(t, result, "current", nil)
		})
	}
	t.Run("literal declarations", func(t *testing.T) {
		v182TestRun(t, v182TestBlockDocument(t, "editor", v182TestText(t, "@ordering {", "@ordering {", 3), v182TestText(t, "A", "A", 3),
			v182TestText(t, "@answer Missing", "@answer Missing", 3), v182TestText(t, "---", "---", 3), v182TestText(t, "First", "First", 3), v182TestText(t, `\}`, "}", 3)), "current", nil)
	})
}

func TestV182EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v182TestPair(t, v182TestTag(t, "answer", "1", "content", 3), v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3))
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v182TestWant{{[3]int{5, 9, 10}, nil}}
			v182TestRun(t, result, "first", want)
			v182TestRun(t, result, "second", want)
			v182TestRun(t, v182TestPair(t, v182TestTag(t, "answer", "First", "content", 3), v182TestText(t, "First", "First", 3), v182TestText(t, "Last", "Last", 3)), "other", nil)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v182TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v182TestRun(t, result, "current", nil)
	})
	t.Run("missing closing boundary", func(t *testing.T) {
		result := v182TestDocument(t, v182TestOpen(t, "ordering", "", "", 2), v182TestText(t, "A", "A", 3), v182TestTag(t, "answer", "Missing", "content", 3),
			v182TestSeparator(t, "---", 3, false), v182TestText(t, "First", "First", 3))
		v182TestRun(t, result, "current", nil)
	})
}

// v182TestHeading задаёт настоящий корневой заголовок по контракту модели.
func v182TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v182TestLine(t, "### Inside", "heading", 0,
		v182TestElement(t, "heading-level", "###", "3", 1), v182TestElement(t, "title", "Inside", "Inside", 5))
}

func TestV182RootHeadings(t *testing.T) {
	for _, heading := range []bool{false, true} {
		for _, answer := range []string{"A", "Missing"} {
			t.Run("lower/heading="+strconv.FormatBool(heading)+"/"+answer, func(t *testing.T) {
				middle, parent := v182TestLine(t, " \t", "blank", 0), 3
				if heading {
					middle, parent = v182TestHeading(t), 9
				}
				result := v182TestBlockDocument(t, "ordering", v182TestText(t, "One", "One", 3), v182TestTag(t, "answer", answer, "content", 3),
					v182TestText(t, "Two", "Two", 3), v182TestTag(t, "answer", "B", "content", 3), v182TestSeparator(t, "---", 3, false),
					middle, v182TestText(t, "A", "A", parent), v182TestText(t, "B", "B", parent))
				var want []v182TestWant
				if !heading && answer == "Missing" {
					want = []v182TestWant{{[3]int{5, 9, 16}, nil}}
				}
				v182TestRun(t, result, "current", want)
			})
		}
	}
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"upper remainder is not lost", []model.Line{v182TestText(t, "One", "One", 3), v182TestTag(t, "answer", "Missing", "content", 3),
			v182TestHeading(t), v182TestText(t, "Two", "Two", 6), v182TestSeparator(t, "---", 3, false), v182TestText(t, "First", "First", 3)}},
		{"heading interrupts answer adjacency", []model.Line{v182TestText(t, "One", "One", 3), v182TestHeading(t),
			v182TestTag(t, "answer", "Missing", "content", 3), v182TestSeparator(t, "---", 3, false), v182TestText(t, "First", "First", 3)}},
		{"upper heading reparents the separator", []model.Line{v182TestText(t, "One", "One", 3), v182TestTag(t, "answer", "Missing", "content", 3),
			v182TestHeading(t), v182TestText(t, "Two", "Two", 6), v182TestTag(t, "answer", "Last", "content", 6),
			v182TestSeparator(t, "---", 6, false), v182TestText(t, "First", "First", 6), v182TestText(t, "Last", "Last", 6)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v182TestRun(t, v182TestBlockDocument(t, "ordering", tc.body...), "current", nil) })
	}
}

func TestV182HeadingsInNestedBlocks(t *testing.T) {
	for _, answer := range []string{"First", "Missing"} {
		t.Run("excluded hint/"+answer, func(t *testing.T) {
			result := v182TestPair(t, v182TestTag(t, "answer", answer, "content", 3), v182TestOpen(t, "hint", "", "", 3),
				v182TestHeading(t), v182TestText(t, "Missing", "Missing", 10), v182TestClose(t, 9),
				v182TestText(t, "First", "First", 10), v182TestText(t, "Last", "Last", 10))
			var want []v182TestWant
			if answer == "Missing" {
				want = []v182TestWant{{[3]int{5, 9, 16}, nil}}
			}
			v182TestRun(t, result, "current", want)
		})
	}
	t.Run("answer values remain grouped across heading", func(t *testing.T) {
		result := v182TestBlockDocument(t, "ordering", v182TestText(t, "One", "One", 3), v182TestOpen(t, "answer", "", "", 3),
			v182TestText(t, "Missing", "Missing", 5), v182TestHeading(t), v182TestText(t, "MISSING", "MISSING", 7), v182TestClose(t, 5),
			v182TestText(t, "Two", "Two", 7), v182TestTag(t, "answer", "Last", "content", 7), v182TestSeparator(t, "---", 7, false),
			v182TestText(t, "First", "First", 7), v182TestText(t, "Last", "Last", 7))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{6, 1, 8}, [][3]int{{8, 1, 8}}}})
	})
	t.Run("nested hint still interrupts answer adjacency", func(t *testing.T) {
		result := v182TestBlockDocument(t, "ordering", v182TestText(t, "One", "One", 3), v182TestOpen(t, "hint", "", "", 3),
			v182TestHeading(t), v182TestText(t, "Check.", "Check.", 6), v182TestClose(t, 5), v182TestTag(t, "answer", "Missing", "content", 6),
			v182TestSeparator(t, "---", 6, false), v182TestText(t, "First", "First", 6))
		v182TestRun(t, result, "current", nil)
	})
}

func TestV182PhysicalBoundaries(t *testing.T) {
	for _, closing := range []model.Line{
		v182TestLine(t, "}", "invalid", 7, v182TestElement(t, "unparsed", "}", "", 1)),
		v182TestLine(t, "} extra", "invalid", 7, v182TestElement(t, "block-close", "}", "}", 1), v182TestElement(t, "unparsed", "extra", "", 3)),
	} {
		t.Run("recovered closing "+closing.Raw, func(t *testing.T) {
			result := v182TestDocument(t, v182TestOpen(t, "ordering", "", "", 2), v182TestText(t, "One", "One", 3), v182TestTag(t, "answer", "A", "content", 3),
				v182TestSeparator(t, "---", 3, false), v182TestHeading(t), v182TestText(t, "A", "A", 7), closing,
				v182TestOpen(t, "ordering", "", "", 7), v182TestText(t, "One", "One", 10), v182TestTag(t, "answer", "Missing", "content", 10),
				v182TestSeparator(t, "---", 10, false), v182TestText(t, "First", "First", 10), v182TestClose(t, 10))
			v182TestRun(t, result, "current", []v182TestWant{{[3]int{12, 9, 16}, nil}})
		})
	}
	t.Run("partial nested opening preserves ownership", func(t *testing.T) {
		partial := v182TestLine(t, "@hint ? {", "invalid", 3, v182TestElement(t, "tag", "@hint", "hint", 1),
			v182TestElement(t, "unparsed", "?", "", 7), v182TestElement(t, "block-open", "{", "{", 9))
		result := v182TestPair(t, v182TestTag(t, "answer", "Missing", "content", 3), partial, v182TestHeading(t),
			v182TestText(t, "Missing", "Missing", 10), v182TestLine(t, "}", "invalid", 10, v182TestElement(t, "unparsed", "}", "", 1)),
			v182TestText(t, "First", "First", 10), v182TestText(t, "Last", "Last", 10))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{5, 9, 16}, nil}})
	})
	t.Run("escaped answer brace is text", func(t *testing.T) {
		result := v182TestBlockDocument(t, "ordering", v182TestText(t, "One", "One", 3), v182TestOpen(t, "answer", "", "", 3),
			v182TestText(t, `\}`, "}", 5), v182TestClose(t, 5), v182TestSeparator(t, "---", 3, false), v182TestText(t, `\}`, "}", 3))
		v182TestRun(t, result, "current", nil)
	})
	t.Run("outside labels do not enter closed ordering", func(t *testing.T) {
		result := v182TestDocument(t, v182TestOpen(t, "ordering", "", "", 2), v182TestText(t, "One", "One", 3), v182TestTag(t, "answer", "Missing", "content", 3),
			v182TestSeparator(t, "---", 3, false), v182TestText(t, "First", "First", 3), v182TestClose(t, 3), v182TestHeading(t), v182TestText(t, "Missing", "Missing", 9),
			v182TestOpen(t, "ordering", "", "", 9), v182TestText(t, "One", "One", 11), v182TestTag(t, "answer", "Other", "content", 11),
			v182TestSeparator(t, "---", 11, false), v182TestText(t, "First", "First", 11), v182TestClose(t, 11))
		v182TestRun(t, result, "current", []v182TestWant{{[3]int{5, 9, 16}, nil}, {[3]int{13, 9, 14}, nil}})
	})
}

func TestV182LiteralOwnership(t *testing.T) {
	for _, literal := range []string{"editor", "multifill"} {
		t.Run(literal, func(t *testing.T) {
			// Настоящий heading расположен в hint. В literal псевдотеги остаются
			// content; скобки и начальный @answer экранированы по правилам DSL.
			answerValue := `\@answer Absent`
			if literal == "multifill" {
				answerValue = "@answer Absent"
			}
			result := v182TestPair(t, v182TestTag(t, "answer", "Missing", "content", 3), v182TestOpen(t, "hint", "", "", 3), v182TestHeading(t),
				v182TestText(t, "Check.", "Check.", 10), v182TestClose(t, 9), v182TestOpen(t, literal, "", "", 10),
				v182TestText(t, `@ordering \{`, "@ordering {", 13), v182TestText(t, "One", "One", 13), v182TestText(t, `\@answer Absent`, answerValue, 13),
				v182TestText(t, "### Inside", "### Inside", 13), v182TestText(t, "---", "---", 13), v182TestText(t, "Missing", "Missing", 13),
				v182TestLine(t, "} extra", "invalid", 13, v182TestElement(t, "unparsed", "} extra", "", 1)),
				v182TestText(t, `\}`, "}", 13), v182TestClose(t, 13), v182TestText(t, "First", "First", 10), v182TestText(t, "Last", "Last", 10))
			v182TestRun(t, result, "current", []v182TestWant{{[3]int{5, 9, 16}, nil}})
		})
	}
}

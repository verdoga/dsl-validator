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

const v175TestMessage = "В @ordering повторяется разделитель списков ---."

// v175TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v175TestWant struct{ location, related [3]int }

func TestV175RegistrationAndVersions(t *testing.T) {
	registry := v175TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V175" {
			continue
		}
		count++
		if definition.Message != v175TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V175 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v175TestDocument(t)
			result.Document.DSLVersion = &version
			before := v175TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v175TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v175TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v175TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v175TestLine(t, raw, "invalid", parent, v175TestElement(t, "unparsed", raw, "", 1))
	}
	return v175TestLine(t, raw, "separator", parent)
}

// v175TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v175TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v175TestOpen(t, name, "", "", 2)}, body...)
	return v175TestDocument(t, append(lines, v175TestClose(t, 3))...)
}

// v175TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v175TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v175TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v175TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v175TestTag задаёт известное имя и явно указанный тип аргумента.
func v175TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v175TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v175TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v175TestLine(t, raw, "tag", parent, elements...)
}

// v175TestOpen добавляет открытие к известному объявлению блока.
func v175TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v175TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v175TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v175TestClose задаёт закрытие конкретного фигурного блока.
func v175TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v175TestLine(t, "}", "block-end", parent, v175TestElement(t, "block-close", "}", "}", 1))
}

// v175TestText сохраняет исходное и семантическое содержимое независимо.
func v175TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v175TestLine(t, raw, "content", parent, v175TestElement(t, "content", raw, value, 1))
}

// v175TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v175TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v175TestTag(t, "dsl-version", "1.2", "version", 0),
		v175TestLine(t, "# Lesson", "heading", 0, v175TestElement(t, "heading-level", "#", "1", 1),
			v175TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v175TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v175TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v175TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v175TestRegistry использует полный штатный каталог без отдельной регистрации.
func v175TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v175TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v175TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v175TestRun сравнивает новые V175 после полного Prepare/Run с явным ожиданием.
func v175TestRun(t *testing.T, result *model.Result, processingID string, want []v175TestWant) {
	t.Helper()
	before := v175TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v175TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v175TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V175" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V175 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v175TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V175 occurrence %d = %+v, want location %+v, related %+v, no detail", i, entry, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V175 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V175 created an element error link")
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
	if err := json.Unmarshal([]byte(v175TestSnapshot(t, result)), &restored); err != nil {
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
	if v175TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

func TestV175ReferenceAndExample(t *testing.T) {
	// Добавлены только версия и заголовок; текст примеров сохранён буквально.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v175TestWant
	}{
		{"reference", "@ordering {\nA\nB\n---\nFirst\nLast\n}", []model.Line{
			v175TestText(t, "A", "A", 3), v175TestText(t, "B", "B", 3), v175TestSeparator(t, "---", 3, false),
			v175TestText(t, "First", "First", 3), v175TestText(t, "Last", "Last", 3)}, nil},
		{"example", "@ordering {\nA\nB\n---\nFirst\nLast\n---\n}", []model.Line{
			v175TestText(t, "A", "A", 3), v175TestText(t, "B", "B", 3), v175TestSeparator(t, "---", 3, false),
			v175TestText(t, "First", "First", 3), v175TestText(t, "Last", "Last", 3), v175TestSeparator(t, "---", 3, true)},
			[]v175TestWant{{[3]int{9, 1, 4}, [3]int{6, 1, 4}}}},
		{"short form", "@ordering {\nA\nB\n}", []model.Line{v175TestText(t, "A", "A", 3), v175TestText(t, "B", "B", 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v175TestBlockDocument(t, "ordering", tc.body...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v175TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV175OccurrencesAndClassification(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v175TestWant
	}{
		{"empty block", nil, nil},
		{"only separator", []model.Line{v175TestSeparator(t, "---", 3, false)}, nil},
		{"three separators", []model.Line{
			v175TestText(t, "A", "A", 3), v175TestSeparator(t, "---", 3, false), v175TestText(t, "B", "B", 3),
			v175TestSeparator(t, "---", 3, true), v175TestText(t, "C", "C", 3), v175TestSeparator(t, "---", 3, true)},
			[]v175TestWant{{[3]int{7, 1, 4}, [3]int{5, 1, 4}}, {[3]int{9, 1, 4}, [3]int{5, 1, 4}}}},
		{"adjacent first and last rows", []model.Line{v175TestSeparator(t, "---", 3, false), v175TestSeparator(t, "---", 3, true)},
			[]v175TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 4}}}},
		{"indentation", []model.Line{v175TestText(t, "A", "A", 3), v175TestSeparator(t, " \t--- ", 3, false),
			v175TestText(t, "星 🌞", "星 🌞", 3), v175TestSeparator(t, "\t---", 3, true)},
			[]v175TestWant{{[3]int{7, 2, 5}, [3]int{5, 3, 6}}}},
		{"blank rows do not reset", []model.Line{v175TestSeparator(t, "---", 3, false), v175TestLine(t, "", "blank", 3),
			v175TestLine(t, " \t", "blank", 3), v175TestSeparator(t, "---", 3, true)},
			[]v175TestWant{{[3]int{7, 1, 4}, [3]int{4, 1, 4}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v175TestRun(t, v175TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
	// Типы задаются входной моделью. unparsed имеет null Value и ссылку на
	// исходную ошибку; ни эта классификация, ни прежние ошибки не скрывают Raw.
	for _, firstKind := range []model.LineType{"separator", "invalid", "content"} {
		for _, repeatKind := range []model.LineType{"separator", "invalid", "content"} {
			t.Run(string(firstKind)+" then "+string(repeatKind), func(t *testing.T) {
				makeLine := func(kind model.LineType) model.Line {
					if kind == "separator" {
						return v175TestSeparator(t, "---", 3, false)
					}
					return v175TestLine(t, "---", kind, 3, v175TestElement(t, "unparsed", "---", "", 1))
				}
				result := v175TestBlockDocument(t, "ordering", v175TestText(t, "A", "A", 3), makeLine(firstKind), v175TestText(t, "First", "First", 3), makeLine(repeatKind))
				v175TestRun(t, result, "current", []v175TestWant{{[3]int{7, 1, 4}, [3]int{5, 1, 4}}})
			})
		}
	}
}

func TestV175ExactRaw(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\---`, "---"}, {` \--- `, " --- "}, {`\\---`, `\---`},
		{"--", "--"}, {"----", "----"}, {"A---", "A---"}, {"--- A", "--- A"},
		{"—", "—"}, {"--- | ---", "--- | ---"}, {"星---", "星---"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v175TestRun(t, v175TestBlockDocument(t, "ordering", v175TestText(t, tc.raw, tc.value, 3),
				v175TestSeparator(t, "---", 3, false), v175TestText(t, tc.raw, tc.value, 3)), "current", nil)
		})
	}
	for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		for _, raw := range []string{space + "---", "---" + space} {
			t.Run(strconv.Quote(raw), func(t *testing.T) {
				v175TestRun(t, v175TestBlockDocument(t, "ordering", v175TestText(t, raw, raw, 3), v175TestSeparator(t, "---", 3, false)), "current", nil)
			})
		}
	}
}

func TestV175StructuralContexts(t *testing.T) {
	for _, inner := range []string{"answer", "hint"} {
		t.Run("nested "+inner, func(t *testing.T) {
			result := v175TestBlockDocument(t, "ordering", v175TestText(t, "A", "A", 3), v175TestOpen(t, inner, "", "", 3),
				v175TestText(t, "---", "---", 5), v175TestText(t, "---", "---", 5), v175TestClose(t, 5), v175TestText(t, "B", "B", 3))
			v175TestRun(t, result, "current", nil)
		})
	}
	t.Run("nested block and tag do not reset", func(t *testing.T) {
		result := v175TestBlockDocument(t, "ordering", v175TestText(t, "A", "A", 3), v175TestSeparator(t, "---", 3, false),
			v175TestOpen(t, "hint", "", "", 3), v175TestText(t, "---", "---", 6), v175TestClose(t, 6),
			v175TestSeparator(t, "---", 3, true), v175TestTag(t, "note", "Read.", "content", 3), v175TestSeparator(t, "---", 3, true))
		v175TestRun(t, result, "current", []v175TestWant{{[3]int{9, 1, 4}, [3]int{5, 1, 4}}, {[3]int{11, 1, 4}, [3]int{5, 1, 4}}})
	})
	t.Run("case and depth", func(t *testing.T) {
		result := v175TestDocument(t, v175TestOpen(t, "fragment", "f", "identifier", 2), v175TestOpen(t, "ORDERING", "", "", 3),
			v175TestText(t, "A", "A", 4), v175TestSeparator(t, "---", 4, false), v175TestText(t, "First", "First", 4),
			v175TestSeparator(t, "---", 4, true), v175TestClose(t, 4), v175TestClose(t, 3), v175TestTag(t, "include", "f", "identifier", 2))
		v175TestRun(t, result, "current", []v175TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}})
	})
	for _, repeated := range []bool{false, true} {
		t.Run("independent blocks "+strconv.FormatBool(repeated), func(t *testing.T) {
			body := []model.Line{v175TestOpen(t, "ordering", "", "", 2), v175TestSeparator(t, "---", 3, false), v175TestClose(t, 3),
				v175TestOpen(t, "ordering", "", "", 2), v175TestSeparator(t, "---", 6, false)}
			var want []v175TestWant
			if repeated {
				body = append(body, v175TestSeparator(t, "---", 6, true))
				want = []v175TestWant{{[3]int{8, 1, 4}, [3]int{7, 1, 4}}}
			}
			body = append(body, v175TestClose(t, 6))
			v175TestRun(t, v175TestDocument(t, body...), "current", want)
		})
	}
	for _, name := range []string{"example", "table", "matching", "editor", "multifill"} {
		t.Run("other container "+name, func(t *testing.T) {
			v175TestRun(t, v175TestBlockDocument(t, name, v175TestText(t, "---", "---", 3), v175TestText(t, "---", "---", 3)), "current", nil)
		})
	}
	t.Run("literal block name", func(t *testing.T) {
		v175TestRun(t, v175TestBlockDocument(t, "editor", v175TestText(t, "@ordering {", "@ordering {", 3),
			v175TestText(t, "---", "---", 3), v175TestText(t, "---", "---", 3), v175TestText(t, `\}`, "}", 3)), "current", nil)
	})
	t.Run("outside and after block", func(t *testing.T) {
		v175TestRun(t, v175TestDocument(t, v175TestText(t, "---", "---", 2), v175TestOpen(t, "ordering", "", "", 2),
			v175TestSeparator(t, "---", 4, false), v175TestClose(t, 4), v175TestText(t, "---", "---", 2)), "current", nil)
	})
	t.Run("inline declaration", func(t *testing.T) {
		v175TestRun(t, v175TestDocument(t, v175TestTag(t, "ordering", "Order.", "content", 2),
			v175TestText(t, "---", "---", 2), v175TestText(t, "---", "---", 2)), "current", nil)
	})
}

// v175TestHeading задаёт корневой заголовок, не закрывающий физический блок.
func v175TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v175TestLine(t, "### Inside", "heading", 0,
		v175TestElement(t, "heading-level", "###", "3", 1), v175TestElement(t, "title", "Inside", "Inside", 5))
}

func TestV175HeadingDoesNotCloseOrdering(t *testing.T) {
	for _, tc := range []struct {
		name   string
		middle model.Line
		parent int
	}{
		{"root heading reparents following lines", v175TestHeading(t), 7},
		{"blank control", v175TestLine(t, "", "blank", 3), 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
				v175TestText(t, "One", "One", 3), v175TestText(t, "Two", "Two", 3),
				v175TestText(t, "---", "---", 3), tc.middle, v175TestText(t, "---", "---", tc.parent),
				v175TestText(t, "A", "A", tc.parent), v175TestText(t, "B", "B", tc.parent), v175TestClose(t, 3))
			v175TestRun(t, result, "current", []v175TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}})
		})
	}
}

func TestV175HeadingInsideNestedContainers(t *testing.T) {
	for _, name := range []string{"answer", "hint", "note"} {
		t.Run(name, func(t *testing.T) {
			result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
				v175TestSeparator(t, "---", 3, false), v175TestOpen(t, name, "", "", 3), v175TestHeading(t),
				v175TestText(t, "---", "---", 6), v175TestText(t, "---", "---", 6), v175TestClose(t, 5),
				v175TestSeparator(t, "---", 6, true), v175TestClose(t, 3), v175TestText(t, "---", "---", 6))
			v175TestRun(t, result, "current", []v175TestWant{{[3]int{10, 1, 4}, [3]int{4, 1, 4}}})
		})
	}
}

func TestV175LiteralsAfterHeading(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
				v175TestSeparator(t, "---", 3, false), v175TestHeading(t), v175TestOpen(t, name, "", "", 5),
				v175TestText(t, `@ordering \{`, "@ordering {", 6), v175TestText(t, "---", "---", 6),
				v175TestText(t, "---", "---", 6), v175TestText(t, `\}`, "}", 6), v175TestClose(t, 6),
				v175TestSeparator(t, "---", 5, true), v175TestClose(t, 3), v175TestText(t, "---", "---", 5))
			v175TestRun(t, result, "current", []v175TestWant{{[3]int{12, 1, 4}, [3]int{4, 1, 4}}})
		})
	}
}

func TestV175PhysicalOrderingBoundaries(t *testing.T) {
	result := v175TestDocument(t, v175TestOpen(t, "ORDERING", "", "", 2),
		v175TestSeparator(t, "\t--- ", 3, false), v175TestHeading(t), v175TestOpen(t, "ordering", "", "", 5),
		v175TestSeparator(t, "  ---", 6, false), v175TestHeading(t), v175TestSeparator(t, "\t---", 8, true),
		v175TestClose(t, 6), v175TestSeparator(t, " --- ", 8, true), v175TestClose(t, 3),
		v175TestText(t, "---", "---", 8), v175TestOpen(t, "ordering", "", "", 8),
		v175TestSeparator(t, "---", 14, false), v175TestSeparator(t, "  --- ", 14, true),
		v175TestClose(t, 14), v175TestText(t, "---", "---", 8))
	v175TestRun(t, result, "current", []v175TestWant{
		{[3]int{9, 2, 5}, [3]int{7, 3, 6}}, {[3]int{11, 2, 5}, [3]int{4, 2, 5}}, {[3]int{16, 3, 6}, [3]int{15, 1, 4}},
	})
}

func TestV175EscapedBracesAfterHeading(t *testing.T) {
	result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
		v175TestSeparator(t, "---", 3, false), v175TestHeading(t),
		v175TestText(t, `\@ordering \{`, "@ordering {", 5), v175TestText(t, `\}`, "}", 5),
		v175TestText(t, `\---`, "---", 5), v175TestSeparator(t, "---", 5, true), v175TestClose(t, 3),
		v175TestText(t, "---", "---", 5))
	v175TestRun(t, result, "current", []v175TestWant{{[3]int{9, 1, 4}, [3]int{4, 1, 4}}})
}

func TestV175RecoveredClosingsAfterHeading(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
		want []v175TestWant
	}{
		{"unparsed closing", v175TestLine(t, " \t} ", "invalid", 5,
			v175TestElement(t, "unparsed", " \t} ", "", 1)), nil},
		{"unparsed closing classified as content", v175TestLine(t, "}", "content", 5,
			v175TestElement(t, "unparsed", "}", "", 1)), nil},
		{"recognized closing with invalid tail", v175TestLine(t, "} ?", "invalid", 5,
			v175TestElement(t, "block-close", "}", "}", 1), v175TestElement(t, "unparsed", "?", "", 3)), nil},
		{"unparsed nonstandalone brace is not a definite closing", v175TestLine(t, "} ?", "invalid", 5,
			v175TestElement(t, "unparsed", "} ?", "", 1)), []v175TestWant{{[3]int{7, 1, 4}, [3]int{4, 1, 4}}}},
		{"escaped brace is not a closing", v175TestText(t, `\}`, "}", 5),
			[]v175TestWant{{[3]int{7, 1, 4}, [3]int{4, 1, 4}}}},
		{"absent closing leaves physical block open", v175TestText(t, "Still open", "Still open", 5),
			[]v175TestWant{{[3]int{7, 1, 4}, [3]int{4, 1, 4}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
				v175TestSeparator(t, "---", 3, false), v175TestHeading(t), tc.row, v175TestText(t, "---", "---", 5))
			v175TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV175PartialNestedOpeningsAfterHeading(t *testing.T) {
	for _, name := range []string{"hint", "ordering", "editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			opening := v175TestOpen(t, name, "", "", 5)
			opening.LineType = "invalid"
			opening.Elements = append(opening.Elements, v175TestElement(t, "unparsed", "?", "", len(opening.Raw)+2))
			opening.Raw += " ?"
			result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
				v175TestSeparator(t, "---", 3, false), v175TestHeading(t), opening,
				v175TestText(t, "---", "---", 5), v175TestText(t, "---", "---", 5),
				v175TestLine(t, "}", "invalid", 5, v175TestElement(t, "unparsed", "}", "", 1)),
				v175TestSeparator(t, "---", 5, true), v175TestClose(t, 3))
			want := []v175TestWant{{[3]int{10, 1, 4}, [3]int{4, 1, 4}}}
			if name == "ordering" {
				want = append([]v175TestWant{{[3]int{8, 1, 4}, [3]int{7, 1, 4}}}, want...)
			}
			v175TestRun(t, result, "current", want)
		})
	}
}

func TestV175LiteralRequiresStandaloneClosing(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2),
				v175TestSeparator(t, "---", 3, false), v175TestHeading(t), v175TestOpen(t, name, "", "", 5),
				v175TestLine(t, "} ?", "invalid", 6, v175TestElement(t, "unparsed", "} ?", "", 1)),
				v175TestText(t, "---", "---", 6),
				v175TestText(t, `\}`, "}", 6), v175TestText(t, "---", "---", 6),
				v175TestLine(t, " \t} ", "invalid", 5, v175TestElement(t, "unparsed", " \t} ", "", 1)),
				v175TestSeparator(t, "---", 5, true), v175TestClose(t, 3), v175TestText(t, "---", "---", 5))
			v175TestRun(t, result, "current", []v175TestWant{{[3]int{12, 1, 4}, [3]int{4, 1, 4}}})
		})
	}
}

func TestV175EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v175TestBlockDocument(t, "ordering", v175TestSeparator(t, "---", 3, false), v175TestSeparator(t, "---", 3, true))
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v175TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 4}}}
			v175TestRun(t, result, "first", want)
			v175TestRun(t, result, "second", want)
			v175TestRun(t, v175TestBlockDocument(t, "ordering", v175TestSeparator(t, "---", 3, false)), "other", nil)
		})
	}
	t.Run("unclosed block does not hide definite repetition", func(t *testing.T) {
		result := v175TestDocument(t, v175TestOpen(t, "ordering", "", "", 2), v175TestSeparator(t, "---", 3, false), v175TestSeparator(t, "---", 3, true))
		v175TestRun(t, result, "current", []v175TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 4}}})
	})
	t.Run("empty document", func(t *testing.T) {
		result := v175TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v175TestRun(t, result, "current", nil)
	})
}

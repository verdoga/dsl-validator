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

const v163TestMessage = "В @matching повторяется разделитель списков ---."

// v163TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v163TestWant struct{ location, related [3]int }

func TestV163RegistrationAndVersions(t *testing.T) {
	registry := v163TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V163" {
			continue
		}
		count++
		if definition.Message != v163TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V163 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v163TestDocument(t)
			result.Document.DSLVersion = &version
			before := v163TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v163TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v163TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v163TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v163TestLine(t, raw, "invalid", parent, v163TestElement(t, "unparsed", raw, "", 1))
	}
	return v163TestLine(t, raw, "separator", parent)
}

// v163TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v163TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v163TestOpen(t, name, "", "", 2)}, body...)
	return v163TestDocument(t, append(lines, v163TestClose(t, 3))...)
}

// v163TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v163TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v163TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v163TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v163TestTag задаёт известное имя и явно указанный тип аргумента.
func v163TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v163TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v163TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v163TestLine(t, raw, "tag", parent, elements...)
}

// v163TestOpen добавляет открытие к известному объявлению блока.
func v163TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v163TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v163TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v163TestClose задаёт закрытие конкретного фигурного блока.
func v163TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v163TestLine(t, "}", "block-end", parent, v163TestElement(t, "block-close", "}", "}", 1))
}

// v163TestText сохраняет исходное и семантическое содержимое независимо.
func v163TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v163TestLine(t, raw, "content", parent, v163TestElement(t, "content", raw, value, 1))
}

// v163TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v163TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v163TestTag(t, "dsl-version", "1.2", "version", 0),
		v163TestLine(t, "# Lesson", "heading", 0, v163TestElement(t, "heading-level", "#", "1", 1),
			v163TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v163TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v163TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v163TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v163TestRegistry использует полный штатный каталог без отдельной регистрации.
func v163TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v163TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v163TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v163TestRun сравнивает новые V163 после полного Prepare/Run с явным ожиданием.
func v163TestRun(t *testing.T, result *model.Result, processingID string, want []v163TestWant) {
	t.Helper()
	before := v163TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v163TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v163TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V163" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V163 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v163TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V163 occurrence %d = %+v, want location %+v, related %+v, no detail", i, entry, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V163 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V163 created an element error link")
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
	if err := json.Unmarshal([]byte(v163TestSnapshot(t, result)), &restored); err != nil {
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
	if v163TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

func TestV163ReferenceAndExample(t *testing.T) {
	// Добавлены только версия DSL и заголовок; фрагменты таблицы сохранены буквально.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v163TestWant
	}{
		{"reference", "@matching {\nЗнак\n---\n\\---\nПлюс\n}", []model.Line{
			v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
			v163TestText(t, `\---`, "---", 3), v163TestText(t, "Плюс", "Плюс", 3)}, nil},
		{"example", "@matching {\nЗнак\n---\nМинус\nПлюс\n---\n}", []model.Line{
			v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
			v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestSeparator(t, "---", 3, false)},
			[]v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v163TestBlockDocument(t, "matching", tc.body...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v163TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV163CountsAndWhitespace(t *testing.T) {
	text := func(raw string) model.Line { return v163TestText(t, raw, raw, 3) }
	separator := func(raw string) model.Line { return v163TestSeparator(t, raw, 3, false) }
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v163TestWant
	}{
		{"zero", []model.Line{text("Знак"), text("Минус"), text("Плюс")}, nil},
		{"one", []model.Line{text("Знак"), separator("---"), text("Минус"), text("Плюс")}, nil},
		{"three", []model.Line{text("Знак"), separator("---"), text("Минус"), text("Плюс"), separator("---"), text("Звезда"), separator("---")},
			[]v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}, {[3]int{10, 1, 4}, [3]int{5, 1, 4}}}},
		{"adjacent", []model.Line{text("Знак"), separator("---"), separator("---"), text("Минус"), text("Плюс")},
			[]v163TestWant{{[3]int{6, 1, 4}, [3]int{5, 1, 4}}}},
		{"edge spaces and tabs", []model.Line{text("Знак"), separator(" \t--- \t"), text("Минус"), text("Плюс"), separator("\t--- ")},
			[]v163TestWant{{[3]int{8, 2, 5}, [3]int{5, 3, 6}}}},
		{"empty lines do not reset", []model.Line{text("Знак"), separator("---"), v163TestLine(t, "", "blank", 3), v163TestLine(t, " \t", "blank", 3), separator("---"), text("Минус"), text("Плюс")},
			[]v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}}},
		{"first and last body rows", []model.Line{separator("---"), text("Минус"), text("Плюс"), separator("---")},
			[]v163TestWant{{[3]int{7, 1, 4}, [3]int{4, 1, 4}}}},
		{"instruction does not count", []model.Line{text("Знак"), separator("---"), text("Минус"), text("Плюс")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v163TestBlockDocument(t, "matching", tc.body...)
			if tc.name == "instruction does not count" {
				opening := v163TestOpen(t, "matching", "---", "content", 2)
				opening.Number, opening.NestingLevel = 3, 1
				result.Lines[2] = opening
			}
			v163TestRun(t, result, "current", tc.want)
		})
	}
	for _, firstInvalid := range []bool{false, true} {
		for _, secondInvalid := range []bool{false, true} {
			t.Run("classification "+strconv.FormatBool(firstInvalid)+strconv.FormatBool(secondInvalid), func(t *testing.T) {
				v163TestRun(t, v163TestBlockDocument(t, "matching", text("Знак"), v163TestSeparator(t, "---", 3, firstInvalid),
					text("Минус"), text("Плюс"), v163TestSeparator(t, "---", 3, secondInvalid)), "current",
					[]v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}})
			})
		}
	}
}

func TestV163ExactRaw(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\---`, "---"}, {" \t\\---\t ", "---"}, {`\\---`, `\---`},
		{"--", "--"}, {"----", "----"}, {"- - -", "- - -"}, {"Текст ---", "Текст ---"}, {"--- текст", "--- текст"},
		{"———", "———"}, {"−−−", "−−−"}, {"--–", "--–"}, {"--- | ---", "--- | ---"},
	} {
		for _, before := range []bool{false, true} {
			t.Run(strconv.Quote(tc.raw)+" before="+strconv.FormatBool(before), func(t *testing.T) {
				candidate := v163TestText(t, tc.raw, tc.value, 3)
				separator := v163TestSeparator(t, "---", 3, false)
				if before {
					candidate, separator = separator, candidate
				}
				v163TestRun(t, v163TestBlockDocument(t, "matching", v163TestText(t, "Знак", "Знак", 3), candidate, separator,
					v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3)), "current", nil)
			})
		}
	}
	for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		for _, raw := range []string{space + "---", "---" + space} {
			t.Run(strconv.Quote(raw), func(t *testing.T) {
				v163TestRun(t, v163TestBlockDocument(t, "matching", v163TestText(t, "Знак", "Знак", 3), v163TestText(t, raw, raw, 3),
					v163TestSeparator(t, "---", 3, false), v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3)), "current", nil)
			})
		}
	}
}

func TestV163NestedContent(t *testing.T) {
	for _, repeat := range []bool{false, true} {
		t.Run("direct repeat="+strconv.FormatBool(repeat), func(t *testing.T) {
			body := []model.Line{v163TestText(t, "Знак", "Знак", 3), v163TestOpen(t, "answer", "", "", 3),
				v163TestText(t, "---", "---", 5), v163TestText(t, "---", "---", 5), v163TestClose(t, 5),
				v163TestOpen(t, "hint", "", "", 3), v163TestText(t, "---", "---", 9), v163TestText(t, "---", "---", 9), v163TestClose(t, 9),
				v163TestSeparator(t, "---", 3, false), v163TestText(t, `\---`, "---", 3), v163TestText(t, "Плюс", "Плюс", 3)}
			var want []v163TestWant
			if repeat {
				body = append(body, v163TestSeparator(t, "---", 3, false))
				want = []v163TestWant{{[3]int{16, 1, 4}, [3]int{13, 1, 4}}}
			}
			v163TestRun(t, v163TestBlockDocument(t, "matching", body...), "current", want)
		})
	}
	t.Run("nested block does not reset first", func(t *testing.T) {
		result := v163TestBlockDocument(t, "matching", v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
			v163TestOpen(t, "hint", "", "", 3), v163TestText(t, "---", "---", 6), v163TestClose(t, 6),
			v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestSeparator(t, "---", 3, false))
		// Подсказка после разделителя ошибочна, но не превращает её содержимое в непосредственное.
		v163TestParserError(t, result, 6, 0, "Недопустимое расположение подсказки.")
		v163TestRun(t, result, "current", []v163TestWant{{[3]int{11, 1, 4}, [3]int{5, 1, 4}}})
	})
	t.Run("nested matching has its own first", func(t *testing.T) {
		result := v163TestBlockDocument(t, "matching", v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
			v163TestOpen(t, "matching", "", "", 3), v163TestText(t, "Знак", "Знак", 6), v163TestSeparator(t, "---", 6, false),
			v163TestText(t, "Минус", "Минус", 6), v163TestText(t, "Плюс", "Плюс", 6), v163TestSeparator(t, "---", 6, false), v163TestClose(t, 6),
			v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestSeparator(t, "---", 3, false))
		v163TestParserError(t, result, 6, 0, "Недопустимое вложение matching.")
		v163TestRun(t, result, "current", []v163TestWant{{[3]int{11, 1, 4}, [3]int{8, 1, 4}}, {[3]int{15, 1, 4}, [3]int{5, 1, 4}}})
	})
}

func TestV163Contexts(t *testing.T) {
	for _, name := range []string{"matching", "MATCHING", "Matching"} {
		t.Run("name "+name, func(t *testing.T) {
			v163TestRun(t, v163TestBlockDocument(t, name, v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
				v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestSeparator(t, "---", 3, false)), "current",
				[]v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}})
		})
	}
	t.Run("indented block", func(t *testing.T) {
		result := v163TestDocument(t, v163TestLine(t, " \t@MaTcHiNg Выберите. {", "block-start", 2,
			v163TestElement(t, "tag", "@MaTcHiNg", "matching", 3), v163TestElement(t, "content", "Выберите.", "Выберите.", 13), v163TestElement(t, "block-open", "{", "{", 23)),
			v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false), v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3),
			v163TestSeparator(t, "---", 3, false), v163TestClose(t, 3))
		v163TestRun(t, result, "current", []v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}})
	})
	t.Run("task", func(t *testing.T) {
		result := v163TestDocument(t, v163TestTag(t, "task", "t", "identifier", 2), v163TestOpen(t, "matching", "", "", 3),
			v163TestText(t, "Знак", "Знак", 4), v163TestSeparator(t, "---", 4, false), v163TestText(t, "Минус", "Минус", 4),
			v163TestText(t, "Плюс", "Плюс", 4), v163TestSeparator(t, "---", 4, false), v163TestClose(t, 4), v163TestTag(t, "endtask", "", "", 3))
		v163TestRun(t, result, "current", []v163TestWant{{[3]int{9, 1, 4}, [3]int{6, 1, 4}}})
	})
	for _, repeat := range []bool{false, true} {
		t.Run("independent blocks repeat="+strconv.FormatBool(repeat), func(t *testing.T) {
			rows := []model.Line{v163TestOpen(t, "matching", "", "", 2), v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
				v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestClose(t, 3),
				v163TestOpen(t, "matching", "", "", 2), v163TestText(t, "Знак", "Знак", 9), v163TestSeparator(t, "---", 9, false),
				v163TestText(t, "Минус", "Минус", 9), v163TestText(t, "Плюс", "Плюс", 9)}
			var want []v163TestWant
			if repeat {
				rows = append(rows, v163TestSeparator(t, "---", 9, false))
				want = []v163TestWant{{[3]int{14, 1, 4}, [3]int{11, 1, 4}}}
			}
			v163TestRun(t, v163TestDocument(t, append(rows, v163TestClose(t, 9))...), "current", want)
		})
	}
	for _, name := range []string{"example", "table", "editor", "hint"} {
		t.Run("excluded "+name, func(t *testing.T) {
			result := v163TestBlockDocument(t, name, v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
				v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestSeparator(t, "---", 3, false))
			if name == "editor" || name == "hint" {
				result.Lines[4] = v163TestText(t, "---", "---", 3)
				result.Lines[4].Number, result.Lines[4].NestingLevel = 5, 2
				result.Lines[7] = v163TestText(t, "---", "---", 3)
				result.Lines[7].Number, result.Lines[7].NestingLevel = 8, 2
			}
			v163TestRun(t, result, "current", nil)
		})
	}
	t.Run("literal block template", func(t *testing.T) {
		v163TestRun(t, v163TestBlockDocument(t, "editor", v163TestText(t, `@matching \{`, "@matching {", 3),
			v163TestText(t, "Знак", "Знак", 3), v163TestText(t, "---", "---", 3), v163TestText(t, "Минус", "Минус", 3),
			v163TestText(t, "---", "---", 3), v163TestText(t, `\}`, "}", 3)), "current", nil)
	})
}

func TestV163HistoryAndEndings(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("ending "+strconv.Quote(string(eol)), func(t *testing.T) {
			result := v163TestBlockDocument(t, "matching", v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
				v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3), v163TestSeparator(t, "---", 3, true))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v163TestWant{{[3]int{8, 1, 4}, [3]int{5, 1, 4}}}
			v163TestRun(t, result, "first", want)
			v163TestRun(t, v163TestBlockDocument(t, "matching", v163TestText(t, "Знак", "Знак", 3), v163TestSeparator(t, "---", 3, false),
				v163TestText(t, "Минус", "Минус", 3), v163TestText(t, "Плюс", "Плюс", 3)), "other", nil)
			v163TestRun(t, result, "second", want)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v163TestDocument(t)
		result.Lines = []model.Line{}
		result.Document.Metadata.Title = nil
		count := 0
		result.Document.LineCount = &count
		v163TestRun(t, result, "current", nil)
	})
}

func TestV163PhysicalHeadingContinuation(t *testing.T) {
	definitions, err := v163TestRegistry(t).ForVersion("1.2")
	if err != nil || len(definitions) != 198 {
		t.Fatalf("full catalog: definitions=%d, err=%v", len(definitions), err)
	}
	for _, heading := range []bool{false, true} {
		t.Run("root heading="+strconv.FormatBool(heading), func(t *testing.T) {
			middle, parent := v163TestLine(t, "", "blank", 3), 3
			if heading {
				middle, parent = v163TestRootHeading(t), 7
			}
			// Точная пара входов аудита: меняются heading и родители продолжения.
			result := v163TestDocument(t,
				v163TestOpen(t, "matching", "", "", 2),
				v163TestText(t, "One", "One", 3),
				v163TestText(t, "Two", "Two", 3),
				v163TestText(t, "---", "---", 3), middle,
				v163TestText(t, "---", "---", parent),
				v163TestText(t, "A", "A", parent),
				v163TestText(t, "B", "B", parent), v163TestClose(t, 3))
			v163TestRun(t, result, "current", []v163TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}})
		})
	}
}

func TestV163PhysicalFirstAfterHeading(t *testing.T) {
	result := v163TestDocument(t,
		v163TestOpen(t, "matching", "", "", 2),
		v163TestRootHeading(t),
		v163TestText(t, `\---`, "---", 4),
		v163TestSeparator(t, " \t---\t", 4, true),
		v163TestText(t, "Текст 😀", "Текст 😀", 4),
		v163TestText(t, `\}`, "}", 4),
		v163TestText(t, " --- ", "---", 4),
		v163TestRootHeading(t),
		v163TestText(t, `\---`, "---", 10),
		v163TestSeparator(t, "\t---", 10, false),
		v163TestClose(t, 3),
		v163TestText(t, "---", "---", 10))
	v163TestRun(t, result, "current", []v163TestWant{
		{[3]int{9, 2, 5}, [3]int{6, 3, 6}},
		{[3]int{12, 2, 5}, [3]int{6, 3, 6}},
	})
}

func TestV163PhysicalNestedBoundaries(t *testing.T) {
	result := v163TestDocument(t,
		v163TestOpen(t, "matching", "", "", 2),
		v163TestSeparator(t, "---", 3, false),
		v163TestOpen(t, "hint", "", "", 3),
		v163TestRootHeading(t),
		v163TestText(t, "---", "---", 6),
		v163TestSeparator(t, "---", 6, true),
		v163TestClose(t, 5),
		v163TestText(t, "---", "---", 6),
		v163TestOpen(t, "matching", "", "", 6),
		v163TestSeparator(t, " \t---", 11, false),
		v163TestRootHeading(t),
		v163TestText(t, "\t---", "---", 13),
		v163TestClose(t, 11),
		v163TestText(t, "---", "---", 13),
		v163TestClose(t, 3),
		v163TestText(t, "---", "---", 13))
	v163TestRun(t, result, "current", []v163TestWant{
		{[3]int{10, 1, 4}, [3]int{4, 1, 4}},
		{[3]int{14, 2, 5}, [3]int{12, 3, 6}},
		{[3]int{16, 1, 4}, [3]int{4, 1, 4}},
	})
}

func TestV163PhysicalLiteralBoundaries(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			result := v163TestDocument(t,
				v163TestOpen(t, "matching", "", "", 2),
				v163TestSeparator(t, "---", 3, false),
				v163TestRootHeading(t),
				v163TestOpen(t, name, "", "", 5),
				v163TestText(t, `@matching \{`, "@matching {", 5),
				v163TestText(t, "---", "---", 5),
				v163TestText(t, `\}`, "}", 5),
				v163TestText(t, "---", "---", 5),
				v163TestLine(t, " \t}\t ", "invalid", 5,
					v163TestElement(t, "unparsed", " \t}\t ", "", 1)),
				v163TestText(t, "---", "---", 5),
				v163TestClose(t, 3),
				v163TestText(t, "---", "---", 5))
			v163TestRun(t, result, "current", []v163TestWant{{[3]int{12, 1, 4}, [3]int{4, 1, 4}}})
		})
	}
}

func TestV163PhysicalClosingAndAdjacentBlocks(t *testing.T) {
	for _, closing := range []string{"parsed", "unparsed", "partial"} {
		t.Run(closing, func(t *testing.T) {
			end := v163TestClose(t, 3)
			if closing == "unparsed" {
				end = v163TestLine(t, "}", "invalid", 5, v163TestElement(t, "unparsed", "}", "", 1))
			}
			if closing == "partial" {
				end = v163TestLine(t, "} ?", "invalid", 5,
					v163TestElement(t, "block-close", "}", "}", 1),
					v163TestElement(t, "unparsed", "?", "", 3))
			}
			result := v163TestDocument(t,
				v163TestOpen(t, "matching", "", "", 2),
				v163TestSeparator(t, "---", 3, false),
				v163TestRootHeading(t),
				v163TestText(t, "---", "---", 5), end,
				v163TestText(t, "---", "---", 5),
				v163TestOpen(t, "matching", "", "", 5),
				v163TestSeparator(t, "\t---", 9, false),
				v163TestRootHeading(t),
				v163TestText(t, "---", "---", 11),
				v163TestClose(t, 9),
				v163TestText(t, "---", "---", 11))
			v163TestRun(t, result, "current", []v163TestWant{
				{[3]int{6, 1, 4}, [3]int{4, 1, 4}},
				{[3]int{12, 1, 4}, [3]int{10, 2, 5}},
			})
		})
	}
}

func TestV163PhysicalPartialOpening(t *testing.T) {
	opening := v163TestOpen(t, "hint", "", "", 5)
	opening.Raw += " ?"
	opening.LineType = "invalid"
	opening.Elements = append(opening.Elements, v163TestElement(t, "unparsed", "?", "", 9))
	result := v163TestDocument(t,
		v163TestOpen(t, "matching", "", "", 2),
		v163TestSeparator(t, "---", 3, false),
		v163TestRootHeading(t), opening,
		v163TestText(t, "---", "---", 5),
		v163TestText(t, "---", "---", 5),
		v163TestLine(t, "}", "invalid", 5, v163TestElement(t, "unparsed", "}", "", 1)),
		v163TestText(t, "---", "---", 5),
		v163TestClose(t, 3),
		v163TestText(t, "---", "---", 5))
	v163TestRun(t, result, "current", []v163TestWant{{[3]int{10, 1, 4}, [3]int{4, 1, 4}}})
}

func TestV163PhysicalUnclosedBlock(t *testing.T) {
	result := v163TestDocument(t,
		v163TestOpen(t, "matching", "", "", 2),
		v163TestSeparator(t, "---", 3, false),
		v163TestRootHeading(t),
		v163TestText(t, "---", "---", 5),
		v163TestText(t, "A", "A", 5),
		v163TestText(t, "---", "---", 5))
	v163TestRun(t, result, "current", []v163TestWant{
		{[3]int{6, 1, 4}, [3]int{4, 1, 4}},
		{[3]int{8, 1, 4}, [3]int{4, 1, 4}},
	})
}

// v163TestRootHeading задаёт heading с обязательными nil ParentLine и глубиной 0.
func v163TestRootHeading(t *testing.T) model.Line {
	t.Helper()
	return v163TestLine(t, "### Inside", "heading", 0,
		v163TestElement(t, "heading-level", "###", "3", 1),
		v163TestElement(t, "title", "Inside", "Inside", 5))
}

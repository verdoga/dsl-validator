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

const v195TestMessage = "Внутри @variants записан заголовок с #."

// v195TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v195TestWant struct {
	location, related [3]int
}

func TestV195RegistrationAndVersions(t *testing.T) {
	registry := v195TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V195" {
			continue
		}
		count++
		if definition.Message != v195TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V195 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v195TestDocument(t)
			result.Document.DSLVersion = &version
			before := v195TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v195TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v195TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v195TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v195TestOpen(t, name, "", "", 2)}, body...)
	return v195TestDocument(t, append(lines, v195TestClose(t, 3))...)
}

// v195TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v195TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v195TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v195TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v195TestTag задаёт известное имя и явно указанный тип аргумента.
func v195TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v195TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v195TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v195TestLine(t, raw, "tag", parent, elements...)
}

// v195TestOpen добавляет открытие к известному объявлению блока.
func v195TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v195TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v195TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v195TestClose задаёт закрытие конкретного фигурного блока.
func v195TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v195TestLine(t, "}", "block-end", parent, v195TestElement(t, "block-close", "}", "}", 1))
}

// v195TestText сохраняет исходное и семантическое содержимое независимо.
func v195TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v195TestLine(t, raw, "content", parent, v195TestElement(t, "content", raw, value, 1))
}

// v195TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v195TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v195TestTag(t, "dsl-version", "1.2", "version", 0),
		v195TestLine(t, "# Lesson", "heading", 0, v195TestElement(t, "heading-level", "#", "1", 1),
			v195TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v195TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v195TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v195TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v195TestRegistry использует полный штатный каталог без отдельной регистрации.
func v195TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v195TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v195TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v195TestRun сравнивает новые V195 после полного Prepare/Run с явным ожиданием.
func v195TestRun(t *testing.T, result *model.Result, processingID string, want []v195TestWant) {
	t.Helper()
	before := v195TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v195TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v195TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V195" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V195 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v195TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V195 occurrence %d = %+v, want location %+v, related %+v without detail", i, entry, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V195 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V195 created an element error link")
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
	if err := json.Unmarshal([]byte(v195TestSnapshot(t, result)), &restored); err != nil {
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
	if v195TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

// v195TestHeading создаёт распознанный заголовок с обязательным корневым положением
// по контракту модели. Его физическое нахождение внутри блока задаётся соседями.
func v195TestHeading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v195TestLine(t, marker+" "+title, "heading", 0,
		v195TestElement(t, "heading-level", marker, strconv.Itoa(level), 1),
		v195TestElement(t, "title", title, title, level+2))
}

// v195TestVariants помещает заданные строки с номера 5 в первую из двух ветвей.
func v195TestVariants(t *testing.T, middle ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v195TestOpen(t, "variants", "", "", 2), v195TestTag(t, "variant", "A", "name", 3)}
	body = append(body, middle...)
	second := len(body) + 3
	body = append(body, v195TestTag(t, "variant", "B", "name", 3),
		v195TestText(t, "Write.", "Write.", second), v195TestClose(t, 3))
	return v195TestDocument(t, body...)
}

func TestV195ReferenceAndExample(t *testing.T) {
	// Добавлены только версия и заголовок документа; фрагменты сохранены буквально.
	for _, tc := range []struct {
		name, fragment string
		line           model.Line
		want           []v195TestWant
	}{
		{"reference", "@variants {\n@variant A\n@header Reading\nRead.\n@variant B\nWrite.\n}",
			v195TestTag(t, "header", "Reading", "title", 4), nil},
		{"example", "@variants {\n@variant A\n### Reading\nRead.\n@variant B\nWrite.\n}",
			v195TestHeading(t, 3, "Reading"), []v195TestWant{{[3]int{5, 1, 12}, [3]int{3, 1, 10}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v195TestVariants(t, tc.line, v195TestText(t, "Read.", "Read.", 4))
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v195TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV195HeadingRecognition(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v195TestWant
	}{
		{"level one", v195TestHeading(t, 1, "X"), []v195TestWant{{[3]int{5, 1, 4}, [3]int{3, 1, 10}}}},
		{"level two", v195TestHeading(t, 2, "X"), []v195TestWant{{[3]int{5, 1, 5}, [3]int{3, 1, 10}}}},
		{"level three", v195TestHeading(t, 3, "X"), []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}}},
		{"indentation and Unicode", v195TestLine(t, " \t### Чтение 🌞 \t", "heading", 0,
			v195TestElement(t, "heading-level", "###", "3", 3), v195TestElement(t, "title", "Чтение 🌞", "Чтение 🌞", 7)),
			[]v195TestWant{{[3]int{5, 3, 15}, [3]int{3, 1, 10}}}},
		{"invalid with marker element", v195TestLine(t, "### Reading", "invalid", 4,
			v195TestElement(t, "heading-level", "###", "3", 1), v195TestElement(t, "unparsed", "Reading", "", 5)),
			[]v195TestWant{{[3]int{5, 1, 12}, [3]int{3, 1, 10}}}},
		{"missing marker element", v195TestLine(t, "### Reading", "invalid", 4,
			v195TestElement(t, "unparsed", "### Reading", "", 1)), []v195TestWant{{[3]int{5, 1, 12}, [3]int{3, 1, 10}}}},
		{"escaped", v195TestText(t, `\### Reading`, "### Reading", 4), nil},
		{"escaped backslash remains a prefix", v195TestText(t, `\\### Reading`, `\### Reading`, 4), nil},
		{"midline", v195TestText(t, "Text ### Reading", "Text ### Reading", 4), nil},
		{"header", v195TestTag(t, "header", "### Reading", "title", 4), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v195TestRun(t, v195TestVariants(t, tc.line, v195TestText(t, "Read.", "Read.", 4)), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		raw  string
		want []v195TestWant
	}{
		{"###\tX", []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}}},
		{"###\u00a0X", []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}}},
		{"### ", []v195TestWant{{[3]int{5, 1, 4}, [3]int{3, 1, 10}}}},
		{"# \t", []v195TestWant{{[3]int{5, 1, 2}, [3]int{3, 1, 10}}}},
		{"### X\u00a0 \t", []v195TestWant{{[3]int{5, 1, 7}, [3]int{3, 1, 10}}}},
		{"###Reading", nil}, {"###", nil}, {"#", nil}, {"#### Reading", nil}, {"##### X", nil},
		{"###\u200bX", nil}, {"\u00a0### X", nil}, {"###! X", nil},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			line := v195TestLine(t, tc.raw, "invalid", 4, v195TestElement(t, "unparsed", tc.raw, "", 1))
			v195TestRun(t, v195TestVariants(t, line, v195TestText(t, "Read.", "Read.", 4)), "current", tc.want)
		})
	}
	for _, space := range []string{"\v", "\f", "\u0085", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("White_Space separator "+strconv.Quote(space), func(t *testing.T) {
			raw := "###" + space + "X"
			v195TestRun(t, v195TestVariants(t, v195TestLine(t, raw, "invalid", 4,
				v195TestElement(t, "unparsed", raw, "", 1))), "current", []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}})
		})
	}
	t.Run("branch name is not a heading", func(t *testing.T) {
		v195TestRun(t, v195TestDocument(t, v195TestOpen(t, "variants", "", "", 2),
			v195TestTag(t, "variant", "### Reading", "name", 3), v195TestText(t, "Read.", "Read.", 4),
			v195TestTag(t, "variant", "### Writing", "name", 3), v195TestText(t, "Write.", "Write.", 6), v195TestClose(t, 3)), "current", nil)
	})
}

func TestV195PhysicalIntervals(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v195TestWant
	}{
		{"before first branch", []model.Line{v195TestOpen(t, "variants", "", "", 2), v195TestHeading(t, 2, "Intro"),
			v195TestTag(t, "variant", "A", "name", 3), v195TestText(t, "Read.", "Read.", 5),
			v195TestTag(t, "variant", "B", "name", 3), v195TestText(t, "Write.", "Write.", 7), v195TestClose(t, 3)},
			[]v195TestWant{{[3]int{4, 1, 9}, [3]int{3, 1, 10}}}},
		{"headings do not end interval", []model.Line{v195TestOpen(t, "variants", "", "", 2),
			v195TestTag(t, "variant", "A", "name", 3), v195TestHeading(t, 1, "X"), v195TestHeading(t, 2, "Y"),
			v195TestText(t, "Read.", "Read.", 4), v195TestTag(t, "variant", "B", "name", 3),
			v195TestText(t, "Write.", "Write.", 8), v195TestClose(t, 3)},
			[]v195TestWant{{[3]int{5, 1, 4}, [3]int{3, 1, 10}}, {[3]int{6, 1, 5}, [3]int{3, 1, 10}}}},
		{"outside both boundaries", []model.Line{v195TestHeading(t, 3, "Before"), v195TestOpen(t, "variants", "", "", 3),
			v195TestTag(t, "variant", "A", "name", 4), v195TestText(t, "Read.", "Read.", 5),
			v195TestTag(t, "variant", "B", "name", 4), v195TestText(t, "Write.", "Write.", 7), v195TestClose(t, 4),
			v195TestHeading(t, 3, "After")}, nil},
		{"unclosed interval", []model.Line{v195TestOpen(t, "variants", "", "", 2),
			v195TestTag(t, "variant", "A", "name", 3), v195TestText(t, "Read.", "Read.", 4),
			v195TestTag(t, "variant", "B", "name", 3), v195TestText(t, "Write.", "Write.", 6), v195TestHeading(t, 3, "Last")},
			[]v195TestWant{{[3]int{8, 1, 9}, [3]int{3, 1, 10}}}},
		{"logical parents do not change interval", []model.Line{v195TestOpen(t, "variants", "", "", 2),
			v195TestTag(t, "variant", "A", "name", 3), v195TestTag(t, "task", "practice", "identifier", 4),
			v195TestTag(t, "step", "First", "title", 5), v195TestHeading(t, 3, "X"),
			v195TestText(t, "Read.", "Read.", 6), v195TestTag(t, "endtask", "", "", 4),
			v195TestTag(t, "variant", "B", "name", 3), v195TestText(t, "Write.", "Write.", 10), v195TestClose(t, 3)},
			[]v195TestWant{{[3]int{7, 1, 6}, [3]int{3, 1, 10}}}},
		{"nearest of two variants", []model.Line{v195TestOpen(t, "variants", "", "", 2),
			v195TestTag(t, "variant", "A", "name", 3), v195TestOpen(t, "variants", "", "", 4),
			v195TestTag(t, "variant", "Inner A", "name", 5), v195TestHeading(t, 3, "X"),
			v195TestText(t, "Read.", "Read.", 6), v195TestTag(t, "variant", "Inner B", "name", 5),
			v195TestText(t, "Write.", "Write.", 9), v195TestClose(t, 5), v195TestHeading(t, 3, "Y"),
			v195TestTag(t, "variant", "B", "name", 3), v195TestText(t, "Write.", "Write.", 13), v195TestClose(t, 3)},
			[]v195TestWant{{[3]int{7, 1, 6}, [3]int{5, 1, 10}}, {[3]int{12, 1, 6}, [3]int{3, 1, 10}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v195TestRun(t, v195TestDocument(t, tc.body...), "current", tc.want) })
	}
	t.Run("tag case and source range", func(t *testing.T) {
		opening := v195TestLine(t, " \t@VaRiAnTs {", "block-start", 2,
			v195TestElement(t, "tag", "@VaRiAnTs", "variants", 3), v195TestElement(t, "block-open", "{", "{", 13))
		v195TestRun(t, v195TestDocument(t, opening, v195TestTag(t, "variant", "A", "name", 3),
			v195TestHeading(t, 3, "X"), v195TestText(t, "Read.", "Read.", 4),
			v195TestTag(t, "variant", "B", "name", 3), v195TestText(t, "Write.", "Write.", 7), v195TestClose(t, 3)),
			"current", []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 3, 12}}})
	})
}
func TestV195NestedContexts(t *testing.T) {
	// После уточнения пользователя остальные специализированные исключения отменены.
	for _, name := range []string{"example", "wordlist", "script", "text", "key", "note", "alt", "hint", "instr", "fragment", "answer", "choice", "multichoice", "matching", "ordering", "table"} {
		t.Run(name, func(t *testing.T) {
			argument, kind := "", model.ElementType("")
			if name == "fragment" {
				argument, kind = "f", "identifier"
			}
			middle := []model.Line{v195TestOpen(t, name, argument, kind, 4), v195TestHeading(t, 3, "X"),
				v195TestText(t, "Read.", "Read.", 5)}
			switch name {
			case "matching":
				middle = append(middle, v195TestText(t, "A", "A", 5), v195TestText(t, "B", "B", 5), v195TestLine(t, "---", "separator", 5),
					v195TestText(t, "First", "First", 5), v195TestText(t, "Last", "Last", 5))
			case "choice", "multichoice", "ordering":
				middle = append(middle, v195TestText(t, "A", "A", 5), v195TestText(t, "B", "B", 5))
			case "table":
				middle = append(middle, v195TestText(t, "A | B", "A | B", 5), v195TestText(t, "C | D", "C | D", 5))
			case "script":
				middle = append(middle, v195TestText(t, "Tom: Hello.", "Tom: Hello.", 5))
			}
			middle = append(middle, v195TestClose(t, 5))
			want := []v195TestWant{{[3]int{6, 1, 6}, [3]int{3, 1, 10}}}
			if name == "table" {
				want = nil
			}
			v195TestRun(t, v195TestVariants(t, middle...), "current", want)
		})
	}
	t.Run("only nearest table excludes", func(t *testing.T) {
		v195TestRun(t, v195TestVariants(t, v195TestOpen(t, "table", "", "", 4),
			v195TestOpen(t, "note", "", "", 5), v195TestHeading(t, 3, "X"), v195TestText(t, "Read.", "Read.", 6),
			v195TestClose(t, 6), v195TestHeading(t, 3, "Y"), v195TestText(t, "A | B", "A | B", 5),
			v195TestClose(t, 5), v195TestHeading(t, 3, "Z")), "current",
			[]v195TestWant{{[3]int{7, 1, 6}, [3]int{3, 1, 10}}, {[3]int{13, 1, 6}, [3]int{3, 1, 10}}})
	})
	for _, name := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run("literal "+name, func(t *testing.T) {
			v195TestRun(t, v195TestVariants(t, v195TestOpen(t, name, "", "", 4),
				v195TestText(t, "### Hidden", "### Hidden", 5), v195TestText(t, "@variants {", "@variants {", 5),
				v195TestText(t, "@table {", "@table {", 5), v195TestText(t, "_____ {", "_____ {", 5),
				v195TestText(t, "_____{A}{B}", "_____{A}{B}", 5), v195TestText(t, `\}`, "}", 5),
				v195TestClose(t, 5), v195TestHeading(t, 3, "Visible")), "current",
				[]v195TestWant{{[3]int{13, 1, 12}, [3]int{3, 1, 10}}})
		})
		t.Run("literal pseudo variants outside real variants "+name, func(t *testing.T) {
			v195TestRun(t, v195TestBlockDocument(t, name, v195TestText(t, "@variants {", "@variants {", 3),
				v195TestText(t, "### Hidden", "### Hidden", 3), v195TestText(t, `\}`, "}", 3)), "current", nil)
		})
	}
}

func TestV195RecoveredBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		active    bool
	}{
		{"plain opening", "@variants {", true},
		{"case and joined brace", "@VaRiAnTs{", true},
		{"escaped brace before opening", `@variants \{ {`, true},
		{"escaped close before opening", `@variants \} {`, true},
		{"two backslashes expose brace", `@variants \\{`, true},
		{"four backslashes expose brace", `@variants \\\\{`, true},
		{"ordinary backslash", `@variants \q{`, true},
		{"escaped brace", `@variants \{`, false},
		{"three backslashes hide brace", `@variants \\\{`, false},
		{"missing opening", "@variants", false},
		{"longer name", "@variantsExtra {", false},
		{"escaped declaration", `\@variants {`, false},
		{"midline declaration", "Text @variants {", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Неразобранное открытие не подменяется искусственным block-start.
			// Объявления ветвей и закрытие сохраняют состояние частичного разбора.
			result := v195TestDocument(t, v195TestLine(t, tc.raw, "invalid", 2, v195TestElement(t, "unparsed", tc.raw, "", 1)),
				v195TestTag(t, "variant", "A", "name", 2), v195TestHeading(t, 3, "X"),
				v195TestText(t, "Read.", "Read.", 4), v195TestTag(t, "variant", "B", "name", 2),
				v195TestText(t, "Write.", "Write.", 7), v195TestLine(t, "}", "invalid", 2, v195TestElement(t, "unparsed", "}", "", 1)),
				v195TestHeading(t, 3, "Outside"))
			var want []v195TestWant
			if tc.active {
				want = []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}}
			}
			v195TestRun(t, result, "current", want)
		})
	}
	t.Run("structural opening without tag element", func(t *testing.T) {
		v195TestRun(t, v195TestDocument(t, v195TestLine(t, "@variants {", "block-start", 2,
			v195TestElement(t, "unparsed", "@variants", "", 1), v195TestElement(t, "block-open", "{", "{", 11)),
			v195TestTag(t, "variant", "A", "name", 3), v195TestHeading(t, 3, "X"),
			v195TestText(t, "Read.", "Read.", 4), v195TestTag(t, "variant", "B", "name", 3),
			v195TestText(t, "Write.", "Write.", 7), v195TestClose(t, 3)), "current",
			[]v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}})
	})
	t.Run("tag and unparsed opening", func(t *testing.T) {
		v195TestRun(t, v195TestDocument(t, v195TestTag(t, "variants", "{", "unparsed", 2),
			v195TestTag(t, "variant", "A", "name", 2), v195TestHeading(t, 3, "X"),
			v195TestText(t, "Read.", "Read.", 4), v195TestTag(t, "variant", "B", "name", 2),
			v195TestText(t, "Write.", "Write.", 7), v195TestLine(t, "}", "invalid", 2, v195TestElement(t, "unparsed", "}", "", 1))),
			"current", []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}})
	})
	for _, name := range []string{"table", "editor", "multifill"} {
		t.Run("recovered excluded context "+name, func(t *testing.T) {
			raw := "@" + name + " {"
			v195TestRun(t, v195TestVariants(t, v195TestLine(t, raw, "invalid", 4, v195TestElement(t, "unparsed", raw, "", 1)),
				v195TestLine(t, "### Hidden", "invalid", 4, v195TestElement(t, "unparsed", "### Hidden", "", 1)),
				v195TestText(t, "Read.", "Read.", 4), v195TestLine(t, "}", "invalid", 4, v195TestElement(t, "unparsed", "}", "", 1)),
				v195TestHeading(t, 3, "Visible")), "current", []v195TestWant{{[3]int{9, 1, 12}, [3]int{3, 1, 10}}})
		})
	}
	t.Run("escaped braces in body do not change intervals", func(t *testing.T) {
		v195TestRun(t, v195TestVariants(t, v195TestText(t, `\{`, "{", 4), v195TestText(t, `\}`, "}", 4),
			v195TestHeading(t, 3, "X")), "current", []v195TestWant{{[3]int{7, 1, 6}, [3]int{3, 1, 10}}})
	})
}

func TestV195EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v195TestDocument(t, v195TestOpen(t, "variants", "", "", 2),
				v195TestTag(t, "variant", "A", "name", 3), v195TestHeading(t, 3, "X"), v195TestText(t, "Read.", "Read.", 4),
				v195TestTag(t, "variant", "B", "name", 3), v195TestText(t, "Write.", "Write.", 7), v195TestClose(t, 3),
				v195TestOpen(t, "variants", "", "", 2), v195TestTag(t, "variant", "A", "name", 10),
				v195TestHeading(t, 3, "Y"), v195TestText(t, "Read.", "Read.", 11),
				v195TestTag(t, "variant", "B", "name", 10), v195TestText(t, "Write.", "Write.", 14), v195TestClose(t, 10))
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v195TestWant{{[3]int{5, 1, 6}, [3]int{3, 1, 10}}, {[3]int{12, 1, 6}, [3]int{10, 1, 10}}}
			v195TestRun(t, result, "first", want)
			v195TestRun(t, result, "second", want)
			v195TestRun(t, v195TestDocument(t, v195TestHeading(t, 3, "Outside")), "other", nil)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v195TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v195TestRun(t, result, "current", nil)
	})
}

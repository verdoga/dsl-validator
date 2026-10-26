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

const v187TestMessage = "Между @variants и открывающей скобкой записано содержимое."

// v187TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v187TestWant [3]int

func TestV187RegistrationAndVersions(t *testing.T) {
	registry := v187TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V187" {
			continue
		}
		count++
		if definition.Message != v187TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V187 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v187TestDocument(t)
			result.Document.DSLVersion = &version
			before := v187TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v187TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v187TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v187TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v187TestOpen(t, name, "", "", 2)}, body...)
	return v187TestDocument(t, append(lines, v187TestClose(t, 3))...)
}

// v187TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v187TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v187TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v187TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v187TestTag задаёт известное имя и явно указанный тип аргумента.
func v187TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v187TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v187TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v187TestLine(t, raw, "tag", parent, elements...)
}

// v187TestOpen добавляет открытие к известному объявлению блока.
func v187TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v187TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v187TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v187TestClose задаёт закрытие конкретного фигурного блока.
func v187TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v187TestLine(t, "}", "block-end", parent, v187TestElement(t, "block-close", "}", "}", 1))
}

// v187TestText сохраняет исходное и семантическое содержимое независимо.
func v187TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v187TestLine(t, raw, "content", parent, v187TestElement(t, "content", raw, value, 1))
}

// v187TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v187TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v187TestTag(t, "dsl-version", "1.2", "version", 0),
		v187TestLine(t, "# Lesson", "heading", 0, v187TestElement(t, "heading-level", "#", "1", 1),
			v187TestElement(t, "title", "Lesson", "Lesson", 3)),
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
				v187TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v187TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v187TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v187TestRegistry использует полный штатный каталог без отдельной регистрации.
func v187TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v187TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v187TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v187TestRun сравнивает новые V187 после полного Prepare/Run с явным ожиданием.
func v187TestRun(t *testing.T, result *model.Result, processingID string, want []v187TestWant) {
	t.Helper()
	before := v187TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v187TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v187TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V187" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V187 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v187TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 0 {
			t.Fatalf("V187 occurrence %d = %+v, want location %+v without detail or related ranges", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Fatal("V187 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V187 created an element error link")
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
	if err := json.Unmarshal([]byte(v187TestSnapshot(t, result)), &restored); err != nil {
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
	if v187TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

// v187TestVariants добавляет полноценные ветви к объявлению на строке 3.
// При неразобранном открытии модель не выдумывает block-start: ветви остаются
// под заголовком, отдельные закрытия — unparsed с исходными ошибками парсера.
func v187TestVariants(t *testing.T, opening model.Line, closings int) *model.Result {
	t.Helper()
	parent := 2
	if opening.LineType == "block-start" {
		parent = 3
	}
	body := []model.Line{opening, v187TestTag(t, "variant", "A", "name", parent),
		v187TestText(t, "Read.", "Read.", 4), v187TestTag(t, "variant", "B", "name", parent),
		v187TestText(t, "Write.", "Write.", 6)}
	for i := 0; i < closings; i++ {
		if i == 0 && parent == 3 {
			body = append(body, v187TestClose(t, 3))
		} else {
			body = append(body, v187TestLine(t, "}", "invalid", 2, v187TestElement(t, "unparsed", "}", "", 1)))
		}
	}
	return v187TestDocument(t, body...)
}

func TestV187ReferenceAndExample(t *testing.T) {
	// Перед исходными фрагментами добавлены только версия и заголовок документа.
	for _, tc := range []struct {
		name, content, fragment string
		want                    []v187TestWant
	}{
		{"reference", "", "@variants {\n@variant A\nRead.\n@variant B\nWrite.\n}", nil},
		{"example", "Pair work", "@variants Pair work {\n@variant A\nRead.\n@variant B\nWrite.\n}", []v187TestWant{{3, 11, 20}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v187TestVariants(t, v187TestOpen(t, "variants", tc.content, "unparsed", 2), 1)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v187TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV187SourceRangeAndWhitespace(t *testing.T) {
	for _, tc := range []struct {
		name, raw      string
		tagStart, open int
		want           []v187TestWant
	}{
		{"one word", "@variants X {", 1, 13, []v187TestWant{{3, 11, 12}}},
		{"several words", "@variants Pair work {", 1, 21, []v187TestWant{{3, 11, 20}}},
		{"internal spaces", "@variants Pair  work {", 1, 22, []v187TestWant{{3, 11, 21}}},
		{"case indentation tabs", "\t@VaRiAnTs \tPair  work\t {", 2, 25, []v187TestWant{{3, 13, 23}}},
		{"no intervening text", "@variants{", 1, 10, nil},
		{"spaces and tabs", "@variants \t \t{", 1, 14, nil},
		{"NBSP at text edges", "@variants \u00a0X\u00a0 {", 1, 15, []v187TestWant{{3, 11, 14}}},
		{"Unicode", "@variants 🌞 {", 1, 13, []v187TestWant{{3, 11, 12}}},
		{"punctuation", "@variants ! {", 1, 13, []v187TestWant{{3, 11, 12}}},
		{"zero width space is content", "@variants \u200b {", 1, 13, []v187TestWant{{3, 11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []rune(tc.raw)
			opening := v187TestLine(t, tc.raw, "block-start", 2,
				v187TestElement(t, "tag", string(raw[tc.tagStart-1:tc.tagStart+8]), "variants", tc.tagStart),
				v187TestElement(t, "block-open", "{", "{", tc.open))
			v187TestRun(t, v187TestVariants(t, opening, 1), "current", tc.want)
		})
	}
	// Все White_Space, допустимые внутри Raw: LF/CR принадлежат окончаниям строк.
	for _, space := range []string{"\t", "\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("only whitespace "+strconv.Quote(space), func(t *testing.T) {
			v187TestRun(t, v187TestVariants(t, v187TestOpen(t, "variants", space, "unparsed", 2), 1), "current", nil)
		})
	}
}

func TestV187PartialParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opening model.Line
	}{
		{"content", v187TestOpen(t, "variants", "Pair work", "content", 2)},
		{"unparsed", v187TestOpen(t, "variants", "Pair work", "unparsed", 2)},
		{"no content element", v187TestLine(t, "@variants Pair work {", "block-start", 2,
			v187TestElement(t, "tag", "@variants", "variants", 1), v187TestElement(t, "block-open", "{", "{", 21))},
		{"invalid with tag and opening", v187TestLine(t, "@variants Pair work {", "invalid", 2,
			v187TestElement(t, "tag", "@variants", "variants", 1), v187TestElement(t, "unparsed", "Pair work", "", 11),
			v187TestElement(t, "block-open", "{", "{", 21))},
		{"tag and unparsed tail", v187TestLine(t, "@variants Pair work {", "tag", 2,
			v187TestElement(t, "tag", "@variants", "variants", 1), v187TestElement(t, "unparsed", "Pair work {", "", 11))},
		{"whole invalid", v187TestLine(t, "@variants Pair work {", "invalid", 2,
			v187TestElement(t, "unparsed", "@variants Pair work {", "", 1))},
		{"unparsed without invalid classification", v187TestLine(t, "@variants Pair work {", "tag", 2,
			v187TestElement(t, "unparsed", "@variants Pair work {", "", 1))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v187TestRun(t, v187TestVariants(t, tc.opening, 1), "current", []v187TestWant{{3, 11, 20}})
		})
	}
	t.Run("semantic value does not replace source", func(t *testing.T) {
		opening := v187TestLine(t, `@variants \{ {`, "block-start", 2,
			v187TestElement(t, "tag", "@variants", "variants", 1), v187TestElement(t, "content", `\{`, "{", 11),
			v187TestElement(t, "block-open", "{", "{", 14))
		v187TestRun(t, v187TestVariants(t, opening, 1), "current", []v187TestWant{{3, 11, 13}})
	})
}
func TestV187RawRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		closings  int
		want      []v187TestWant
	}{
		{"space boundary", "@variants X {", 1, []v187TestWant{{3, 11, 12}}},
		{"tab boundary", "@variants\tX {", 1, []v187TestWant{{3, 11, 12}}},
		{"close boundary", "@variants} X {", 1, []v187TestWant{{3, 10, 13}}},
		{"open boundary", "@variants{", 1, nil},
		{"end boundary", "@variants", 0, nil},
		{"case and indentation", " \t@VaRiAnTs X {", 1, []v187TestWant{{3, 13, 14}}},
		{"longer name", "@variantsExtra X {", 1, nil},
		{"digit suffix", "@variants2 X {", 1, nil},
		{"hyphen suffix", "@variants-extra X {", 1, nil},
		{"punctuation suffix", "@variants! X {", 1, nil},
		{"unknown name", "@unknown X {", 1, nil},
		{"different known name", "@variant X {", 1, nil},
		{"NBSP name boundary excluded", "@variants\u00a0X {", 1, nil},
		{"Unicode indentation excluded", "\u00a0@variants X {", 1, nil},
		{"missing opening", "@variants X", 0, nil},
		{"escaped opening", `@variants X \{`, 0, nil},
		{"escaped before active opening", `@variants \{ X {`, 1, []v187TestWant{{3, 11, 15}}},
		{"escaped close before opening", `@variants \} X {`, 1, []v187TestWant{{3, 11, 15}}},
		{"paired backslashes", `@variants \\ {`, 1, []v187TestWant{{3, 11, 13}}},
		{"paired backslashes adjacent to opening", `@variants \\{`, 1, []v187TestWant{{3, 11, 13}}},
		{"three backslashes hide opening", `@variants \\\{`, 0, nil},
		{"four backslashes expose opening", `@variants \\\\{`, 1, []v187TestWant{{3, 11, 15}}},
		{"ordinary escape not consumed", `@variants \q{`, 1, []v187TestWant{{3, 11, 13}}},
		{"trailing backslash", `@variants X \`, 0, nil},
		{"multiple openings are ambiguous", "@variants X { Y {", 2, nil},
		{"empty first interval still ambiguous", "@variants { Y {", 2, nil},
		{"quotes do not escape brace", `@variants "X {`, 1, []v187TestWant{{3, 11, 13}}},
		{"content after sole opening is outside range", "@variants X { Tail", 1, []v187TestWant{{3, 11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opening := v187TestLine(t, tc.raw, "invalid", 2, v187TestElement(t, "unparsed", tc.raw, "", 1))
			v187TestRun(t, v187TestVariants(t, opening, tc.closings), "current", tc.want)
		})
	}
	t.Run("block-open resolves multiple candidates", func(t *testing.T) {
		opening := v187TestLine(t, "@variants X { Y {", "block-start", 2,
			v187TestElement(t, "tag", "@variants", "variants", 1), v187TestElement(t, "unparsed", "X", "", 11),
			v187TestElement(t, "block-open", "{", "{", 13), v187TestElement(t, "unparsed", "Y {", "", 15))
		v187TestRun(t, v187TestVariants(t, opening, 2), "current", []v187TestWant{{3, 11, 12}})
	})
	t.Run("opening on next physical line is not used", func(t *testing.T) {
		result := v187TestDocument(t, v187TestTag(t, "variants", "X", "unparsed", 2),
			v187TestLine(t, "{", "invalid", 2, v187TestElement(t, "unparsed", "{", "", 1)),
			v187TestTag(t, "variant", "A", "name", 2), v187TestText(t, "Read.", "Read.", 5),
			v187TestTag(t, "variant", "B", "name", 2), v187TestText(t, "Write.", "Write.", 7),
			v187TestLine(t, "}", "invalid", 2, v187TestElement(t, "unparsed", "}", "", 1)))
		v187TestRun(t, result, "current", nil)
	})
}

func TestV187Contexts(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@variants X {`, "@variants X {"}, {` \@variants X {`, " @variants X {"},
		{`\\@variants X {`, `\@variants X {`}, {"Text @variants X {", "Text @variants X {"},
		{"🌞 @variants X {", "🌞 @variants X {"},
	} {
		t.Run("literal or mention "+tc.raw, func(t *testing.T) {
			v187TestRun(t, v187TestDocument(t, v187TestText(t, tc.raw, tc.value, 2)), "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run("literal container "+name, func(t *testing.T) {
			v187TestRun(t, v187TestBlockDocument(t, name,
				v187TestText(t, "@variants Pair work {", "@variants Pair work {", 3),
				v187TestText(t, "@variant A", "@variant A", 3), v187TestText(t, "Read.", "Read.", 3),
				v187TestText(t, "@variant B", "@variant B", 3), v187TestText(t, "Write.", "Write.", 3),
				v187TestText(t, `\}`, "}", 3)), "current", nil)
		})
		t.Run("unparsed in literal container "+name, func(t *testing.T) {
			v187TestRun(t, v187TestBlockDocument(t, name, v187TestLine(t, "@variants X {", "invalid", 3,
				v187TestElement(t, "unparsed", "@variants X {", "", 1))), "current", nil)
		})
	}
	t.Run("nested fragment", func(t *testing.T) {
		result := v187TestDocument(t, v187TestOpen(t, "fragment", "f", "identifier", 2),
			v187TestOpen(t, "variants", "Pair work", "unparsed", 3), v187TestTag(t, "variant", "A", "name", 4),
			v187TestText(t, "Read.", "Read.", 5), v187TestTag(t, "variant", "B", "name", 4),
			v187TestText(t, "Write.", "Write.", 7), v187TestClose(t, 4), v187TestClose(t, 3),
			v187TestTag(t, "include", "f", "identifier", 2))
		v187TestRun(t, result, "current", []v187TestWant{{4, 11, 20}})
	})
	t.Run("neighboring declarations and history", func(t *testing.T) {
		result := v187TestDocument(t,
			v187TestOpen(t, "variants", "X", "unparsed", 2), v187TestTag(t, "variant", "A", "name", 3),
			v187TestText(t, "Read.", "Read.", 4), v187TestTag(t, "variant", "B", "name", 3),
			v187TestText(t, "Write.", "Write.", 6), v187TestClose(t, 3),
			v187TestOpen(t, "variants", "Y", "unparsed", 2), v187TestTag(t, "variant", "A", "name", 9),
			v187TestText(t, "Read.", "Read.", 10), v187TestTag(t, "variant", "B", "name", 9),
			v187TestText(t, "Write.", "Write.", 12), v187TestClose(t, 9))
		want := []v187TestWant{{3, 11, 12}, {9, 11, 12}}
		v187TestRun(t, result, "first", want)
		v187TestRun(t, result, "second", want)
		v187TestRun(t, v187TestVariants(t, v187TestOpen(t, "variants", "", "", 2), 1), "other", nil)
	})
}

func TestV187EndingsAndEmptyInput(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v187TestVariants(t, v187TestOpen(t, "variants", "Pair work", "unparsed", 2), 1)
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v187TestRun(t, result, "current", []v187TestWant{{3, 11, 20}})
		})
	}
	t.Run("last line without opening", func(t *testing.T) {
		result := v187TestDocument(t, v187TestTag(t, "variants", "X", "unparsed", 2))
		result.Lines[2].LineEnding = ""
		v187TestRun(t, result, "current", nil)
	})
	t.Run("empty document", func(t *testing.T) {
		result := v187TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v187TestRun(t, result, "current", nil)
	})
}

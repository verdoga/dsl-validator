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

const v096TestMessage = "Внутри @table записан заголовок."

// v096TestWant задаёт независимый диапазон с исключающей правой границей.
type v096TestWant struct{ line, start, end int }

func TestV096RegistrationAndVersions(t *testing.T) {
	registry := v096TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V096" {
			continue
		}
		count++
		if definition.Message != v096TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V096 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.4.4"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v096TestDocument(t)
			result.Document.DSLVersion = &version
			before := v096TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v096TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v096TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v096TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v096TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v096TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v096TestTag задаёт известное имя и явно указанный тип аргумента.
func v096TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v096TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v096TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v096TestLine(t, raw, "tag", parent, elements...)
}

// v096TestOpen добавляет открытие к известному объявлению блока.
func v096TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v096TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v096TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v096TestClose задаёт закрытие конкретного фигурного блока.
func v096TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v096TestLine(t, "}", "block-end", parent, v096TestElement(t, "block-close", "}", "}", 1))
}

// v096TestText сохраняет исходное и семантическое содержимое независимо.
func v096TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v096TestLine(t, raw, "content", parent, v096TestElement(t, "content", raw, value, 1))
}

// v096TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v096TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v096TestTag(t, "dsl-version", "1.2", "version", 0),
		v096TestLine(t, "# Lesson", "heading", 0, v096TestElement(t, "heading-level", "#", "1", 1),
			v096TestElement(t, "title", "Lesson", "Lesson", 3)),
		v096TestLine(t, "### Practice", "heading", 0, v096TestElement(t, "heading-level", "###", "3", 1),
			v096TestElement(t, "title", "Practice", "Practice", 5)),
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
				v096TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v096TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v096TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v096TestRegistry использует полный штатный каталог без отдельной регистрации.
func v096TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v096TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v096TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v096TestRun сравнивает новые V096 после штатных Prepare/Run с явным ожиданием.
func v096TestRun(t *testing.T, result *model.Result, processingID string, want []v096TestWant) {
	t.Helper()
	before := v096TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v096TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v096TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V096" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V096 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v096TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V096 occurrence %d = %+v, want %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V096 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V096 created an element error link")
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
	if err := json.Unmarshal([]byte(v096TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Из сравнения исключаются только добавления конвейера и производные
	// флаги/ссылки. Исходные строки, метаданные и прежняя история сохраняются.
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
	if v096TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV096ReferenceAndExample(t *testing.T) {
	// Версия и заголовки задают контекст шага перед неизменными фрагментами.
	// &#124; в таблице задания означает обычную вертикальную черту в DSL.
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           []v096TestWant
	}{
		{"reference", "@table {\n\\# Heading | Value\n}",
			[]model.Line{v096TestText(t, `\# Heading | Value`, "# Heading | Value", 4)}, nil},
		{"example", "@table {\n## Heading\nA | B\n}",
			[]model.Line{v096TestInvalid(t, "## Heading", 4), v096TestText(t, "A | B", "A | B", 4)},
			[]v096TestWant{{5, 1, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v096TestTable(t, tc.rows...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v096TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV096MarkersAndPositions(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		start, end int
	}{
		{"#", 1, 2}, {"##", 1, 3}, {"### Heading", 1, 4}, {"#### Heading", 1, 5}, {"######## Heading", 1, 9},
		{"##Heading", 1, 3}, {"# # Heading", 1, 2}, {"## Heading ###", 1, 3},
		{" \t## Заголовок", 3, 5}, {"\t\t# One", 3, 4}, {"  ### ", 3, 6},
		{"##\tHeading", 1, 3}, {"##  Heading", 1, 3}, {"##\u00a0Heading", 1, 3},
		{"## Заголовок 😀", 1, 3}, {"## heading", 1, 3}, {"## HEADING", 1, 3},
		{"## Heading | Value", 1, 3}, {`##\# Heading`, 1, 3},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v096TestRun(t, v096TestTable(t, v096TestInvalid(t, tc.raw, 4), v096TestText(t, "A | B", "A | B", 4)),
				"current", []v096TestWant{{5, tc.start, tc.end}})
		})
	}
}

func TestV096PartialAndWholeUnparsed(t *testing.T) {
	for _, row := range []model.Line{
		v096TestInvalid(t, "## Heading", 4),
		v096TestLine(t, "## Heading", "invalid", 4, v096TestElement(t, "heading-level", "##", "2", 1),
			v096TestElement(t, "unparsed", "Heading", "", 4)),
		v096TestLine(t, "## Heading", "invalid", 4, v096TestElement(t, "unparsed", "##", "", 1),
			v096TestElement(t, "title", "Heading", "Heading", 4)),
	} {
		t.Run(strconv.Itoa(len(row.Elements))+"/"+string(row.Elements[0].ElementType), func(t *testing.T) {
			// Вложенный heading запрещён контрактом модели. Ошибочная попытка
			// сохраняет родителя table и неразобранный участок с исходной ошибкой.
			v096TestRun(t, v096TestTable(t, row, v096TestText(t, "A | B", "A | B", 4)), "current", []v096TestWant{{5, 1, 3}})
		})
	}
}

func TestV096LiteralAndCellContent(t *testing.T) {
	for _, tc := range []struct{ raw, value, rest string }{
		{`\# Heading`, "# Heading", "Data"}, {" \t\\# Heading", "# Heading", "Data"},
		{`\## Heading`, "## Heading", "Data"}, {`\\# Heading`, `\# Heading`, "Data"},
		{"A # Heading", "A # Heading", "Data"}, {"A | # Heading", "A | # Heading", "A | B"},
		{"<A | # Heading>", "<A | # Heading>", "<A | B>"},
		{"| # Heading", "| # Heading", "A | B"},
		{"＃ Heading", "＃ Heading", "Data"}, {"😀 # Heading", "😀 # Heading", "Data"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v096TestRun(t, v096TestTable(t, v096TestText(t, tc.raw, tc.value, 4),
				v096TestText(t, tc.rest, tc.rest, 4)), "current", nil)
		})
	}
	for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003",
		"\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("nonindent "+strconv.Quote(space), func(t *testing.T) {
			raw := space + "## Heading"
			v096TestRun(t, v096TestTable(t, v096TestText(t, raw, raw, 4), v096TestText(t, "Data", "Data", 4)), "current", nil)
		})
	}
}

func TestV096MultipleAndSeparators(t *testing.T) {
	t.Run("three consecutive violations", func(t *testing.T) {
		v096TestRun(t, v096TestTable(t, v096TestInvalid(t, "# One", 4), v096TestInvalid(t, "## Two", 4),
			v096TestInvalid(t, "#### Four", 4), v096TestText(t, "A | B", "A | B", 4)), "current",
			[]v096TestWant{{5, 1, 2}, {6, 1, 3}, {7, 1, 5}})
	})
	t.Run("both table parts and blanks", func(t *testing.T) {
		v096TestRun(t, v096TestTable(t, v096TestInvalid(t, "## First", 4), v096TestText(t, "Name | Form", "Name | Form", 4),
			v096TestLine(t, " \t", "blank", 4), v096TestLine(t, "---", "separator", 4, v096TestElement(t, "content", "---", "---", 1)),
			v096TestLine(t, "", "blank", 4), v096TestText(t, "A | B", "A | B", 4), v096TestInvalid(t, "### Last", 4)), "current",
			[]v096TestWant{{5, 1, 3}, {11, 1, 4}})
	})
	t.Run("title and separator are not headings", func(t *testing.T) {
		result := v096TestDocument(t, v096TestOpen(t, "table", "# Name", "title", 3),
			v096TestText(t, "Name | Form", "Name | Form", 4),
			v096TestLine(t, "---", "separator", 4, v096TestElement(t, "content", "---", "---", 1)),
			v096TestText(t, "A | B", "A | B", 4), v096TestClose(t, 4))
		v096TestRun(t, result, "current", nil)
	})
}

func TestV096NestedBlocks(t *testing.T) {
	for _, inner := range []string{"note", "table"} {
		t.Run(inner, func(t *testing.T) {
			result := v096TestDocument(t, v096TestOpen(t, "table", "", "", 3), v096TestOpen(t, inner, "", "", 4),
				v096TestInvalid(t, "## Inner", 5), v096TestText(t, "A | B", "A | B", 5), v096TestClose(t, 5),
				v096TestInvalid(t, "## Outer", 4), v096TestText(t, "C | D", "C | D", 4), v096TestClose(t, 4))
			v096TestParserError(t, result, 5, 0, "Вложение блока запрещено внутри table.")
			want := []v096TestWant{{9, 1, 3}}
			if inner == "table" {
				want = []v096TestWant{{6, 1, 3}, {9, 1, 3}}
			}
			v096TestRun(t, result, "current", want)
		})
	}
	t.Run("logical parent is not the table", func(t *testing.T) {
		result := v096TestDocument(t, v096TestOpen(t, "table", "", "", 3), v096TestTag(t, "task", "nested", "identifier", 4),
			v096TestInvalid(t, "## Inner", 5), v096TestText(t, "A | B", "A | B", 5), v096TestClose(t, 4))
		v096TestParserError(t, result, 5, 0, "Вложение task запрещено внутри table.")
		v096TestRun(t, result, "current", nil)
	})
	for _, name := range []string{"table", "TABLE", "TaBlE"} {
		t.Run("fragment/"+name, func(t *testing.T) {
			result := v096TestDocument(t, v096TestOpen(t, "fragment", "intro", "identifier", 3), v096TestOpen(t, name, "", "", 4),
				v096TestInvalid(t, "## Заголовок 😀", 5), v096TestText(t, "A | B", "A | B", 5), v096TestClose(t, 5), v096TestClose(t, 4))
			v096TestRun(t, result, "current", []v096TestWant{{6, 1, 3}})
		})
	}
}

func TestV096ExcludedContextsAndBoundaries(t *testing.T) {
	for _, name := range []string{"example", "text", "note"} {
		t.Run("other block "+name, func(t *testing.T) {
			result := v096TestDocument(t, v096TestOpen(t, name, "", "", 3), v096TestInvalid(t, "## Heading", 4),
				v096TestText(t, "Read the text.", "Read the text.", 4), v096TestClose(t, 4))
			v096TestRun(t, result, "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			result := v096TestDocument(t, v096TestTag(t, "task", "practice", "identifier", 3), v096TestOpen(t, name, "", "", 4),
				v096TestText(t, "@table {", "@table {", 5), v096TestText(t, "## Heading", "## Heading", 5),
				v096TestText(t, "A | B", "A | B", 5), v096TestClose(t, 5))
			v096TestRun(t, result, "current", nil)
		})
	}
	t.Run("headings outside the block", func(t *testing.T) {
		result := v096TestDocument(t, v096TestOpen(t, "table", "", "", 3), v096TestText(t, "A | B", "A | B", 4), v096TestClose(t, 4),
			v096TestLine(t, "### Review", "heading", 0, v096TestElement(t, "heading-level", "###", "3", 1),
				v096TestElement(t, "title", "Review", "Review", 5)), v096TestText(t, "Read the text.", "Read the text.", 7))
		v096TestRun(t, result, "current", nil)
	})
	t.Run("two independent tables", func(t *testing.T) {
		result := v096TestDocument(t, v096TestOpen(t, "table", "", "", 3), v096TestInvalid(t, "## First", 4),
			v096TestText(t, "A | B", "A | B", 4), v096TestClose(t, 4),
			v096TestOpen(t, "table", "", "", 3), v096TestText(t, "C | D", "C | D", 8), v096TestInvalid(t, "#### Last", 8), v096TestClose(t, 8))
		v096TestRun(t, result, "current", []v096TestWant{{5, 1, 3}, {10, 1, 5}})
	})
}

func TestV096EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v096TestTable(t, v096TestInvalid(t, "## Heading", 4), v096TestText(t, "A | B", "A | B", 4))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v096TestWant{{5, 1, 3}}
			v096TestRun(t, result, "first", want)
			v096TestRun(t, v096TestTable(t, v096TestText(t, `\# Heading`, "# Heading", 4)), "other", nil)
			v096TestRun(t, result, "second", want)
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v096TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v096TestRun(t, result, "current", nil)
		})
	}
}

// v096TestInvalid задаёт ошибочную попытку конструкции с целиком неразобранным Raw.
func v096TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v096TestLine(t, raw, "invalid", parent, v096TestElement(t, "unparsed", raw, "", 1))
}

// v096TestTable задаёт таблицу внутри шага; непосредственные строки переданы явно.
func v096TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v096TestOpen(t, "table", "", "", 3)}, rows...)
	return v096TestDocument(t, append(body, v096TestClose(t, 4))...)
}

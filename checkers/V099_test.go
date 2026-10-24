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

const v099TestMessage = "В конце обычной ячейки знак > не экранирован."

const v099TestDetail = `Для буквального знака используйте \>`

// v099TestWant задаёт независимый диапазон с исключающей правой границей.
type v099TestWant struct{ line, start, end int }

func TestV099RegistrationAndVersions(t *testing.T) {
	registry := v099TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V099" {
			continue
		}
		count++
		if definition.Message != v099TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V099 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.4.32"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v099TestDocument(t)
			result.Document.DSLVersion = &version
			before := v099TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v099TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v099TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v099TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v099TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v099TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v099TestTag задаёт известное имя и явно указанный тип аргумента.
func v099TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v099TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v099TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v099TestLine(t, raw, "tag", parent, elements...)
}

// v099TestOpen добавляет открытие к известному объявлению блока.
func v099TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v099TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v099TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v099TestClose задаёт закрытие конкретного фигурного блока.
func v099TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v099TestLine(t, "}", "block-end", parent, v099TestElement(t, "block-close", "}", "}", 1))
}

// v099TestText сохраняет исходное и семантическое содержимое независимо.
func v099TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v099TestLine(t, raw, "content", parent, v099TestElement(t, "content", raw, value, 1))
}

// v099TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v099TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v099TestTag(t, "dsl-version", "1.2", "version", 0),
		v099TestLine(t, "# Lesson", "heading", 0, v099TestElement(t, "heading-level", "#", "1", 1),
			v099TestElement(t, "title", "Lesson", "Lesson", 3)),
		v099TestLine(t, "### Practice", "heading", 0, v099TestElement(t, "heading-level", "###", "3", 1),
			v099TestElement(t, "title", "Practice", "Practice", 5)),
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
				v099TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v099TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v099TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v099TestRegistry использует полный штатный каталог без отдельной регистрации.
func v099TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v099TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v099TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v099TestRun сравнивает новые V099 после штатных Prepare/Run с явным ожиданием.
func v099TestRun(t *testing.T, result *model.Result, processingID string, want []v099TestWant) {
	t.Helper()
	before := v099TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v099TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v099TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V099" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V099 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v099TestMessage+"\nконкретизация: "+v099TestDetail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V099 occurrence %d = %+v, want %+v, no related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V099 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V099 created an element error link")
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
	if err := json.Unmarshal([]byte(v099TestSnapshot(t, result)), &restored); err != nil {
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
	if v099TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV099ReferenceAndExample(t *testing.T) {
	// Перед фрагментами добавлены версия и заголовки для контекста шага.
	// &gt; и &#124; в таблице задания обозначают > и |, а не текст DSL.
	for _, tc := range []struct {
		name, raw, value, fragment string
		want                       []v099TestWant
	}{
		{"reference", `x > y | x\>`, "x > y | x>", "@table {\nx > y | x\\>\n}", nil},
		{"example", "x> | y", "x> | y", "@table {\nx> | y\n}", []v099TestWant{{5, 2, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v099TestTable(t, v099TestText(t, tc.raw, tc.value, 4))
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v099TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV099OrdinaryCells(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []v099TestWant
	}{
		{"x>", []v099TestWant{{5, 2, 3}}}, {">", []v099TestWant{{5, 1, 2}}},
		{"x> | y>", []v099TestWant{{5, 2, 3}, {5, 7, 8}}},
		{"|x>||y>|", []v099TestWant{{5, 3, 4}, {5, 7, 8}}},
		{"x>||", []v099TestWant{{5, 2, 3}}}, {"||x>", []v099TestWant{{5, 4, 5}}},
		{" \tx> \t", []v099TestWant{{5, 4, 5}}},
		{"x > y", nil}, {"x>\ty", nil}, {">x", nil}, {"x>>", []v099TestWant{{5, 3, 4}}},
		{"x < y>", []v099TestWant{{5, 6, 7}}}, {"x<a|b>", []v099TestWant{{5, 6, 7}}},
		{"x>\u00a0", nil}, {"\u00a0<x>", []v099TestWant{{5, 4, 5}}},
		{"Я😀>", []v099TestWant{{5, 3, 4}}}, {"Cafe\u0301>", []v099TestWant{{5, 6, 7}}},
		{"X>", []v099TestWant{{5, 2, 3}}}, {"Я> | 中文>", []v099TestWant{{5, 2, 3}, {5, 8, 9}}},
		{"x&gt;", nil}, {"x＞", nil}, {"Data", nil}, {"|", nil}, {"||", nil}, {" \t| \t| ", nil},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v099TestRun(t, v099TestTable(t, v099TestText(t, tc.raw, strings.Trim(tc.raw, " \t"), 4)), "current", tc.want)
		})
	}
	for _, space := range []string{"\v", "\f", "\u0085", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003",
		"\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("nontrimmed "+strconv.Quote(space), func(t *testing.T) {
			raw := "x>" + space
			v099TestRun(t, v099TestTable(t, v099TestText(t, raw, raw, 4)), "current", nil)
		})
	}
}

func TestV099Escapes(t *testing.T) {
	for _, tc := range []struct {
		raw, value string
		want       []v099TestWant
	}{
		{`x\>`, "x>", nil}, {`x\\>`, `x\>`, []v099TestWant{{5, 4, 5}}},
		{`x\\\>`, `x\>`, nil}, {`x\\\\>`, `x\\>`, []v099TestWant{{5, 6, 7}}},
		{`\<x>`, "<x>", []v099TestWant{{5, 4, 5}}},
		{`x>\|y`, "x>|y", nil}, {`x\|y>`, "x|y>", []v099TestWant{{5, 5, 6}}},
		{`x> \| y`, "x> | y", nil}, {`x\> | y>`, "x> | y>", []v099TestWant{{5, 8, 9}}},
		{`x\>\>`, "x>>", nil}, {`x\>>`, "x>>", []v099TestWant{{5, 4, 5}}},
		{`x>\`, `x>\`, nil}, {`x>\\`, `x>\`, nil}, {`x\q>`, `x\q>`, []v099TestWant{{5, 4, 5}}},
		{`\# Title>`, "# Title>", []v099TestWant{{5, 9, 10}}},
		{`\@note x>`, "@note x>", []v099TestWant{{5, 9, 10}}},
		{`a\\|<b|c>`, `a\|<b|c>`, nil},
		{`a\|<b|c>`, `a|<b|c>`, []v099TestWant{{5, 8, 9}}},
		{`\\<a|b>`, `\<a|b>`, []v099TestWant{{5, 7, 8}}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v099TestRun(t, v099TestTable(t, v099TestText(t, tc.raw, tc.value, 4)), "current", tc.want)
		})
	}
}

func TestV099DividedCellsAndTails(t *testing.T) {
	for _, tc := range []struct {
		raw, value string
		invalid    bool
		want       []v099TestWant
	}{
		{"<a | b>", "<a | b>", false, nil},
		{" \t<a | b>\t ", "<a | b>", false, nil},
		{"<a | b> | x>", "<a | b> | x>", false, []v099TestWant{{5, 12, 13}}},
		{"<a | b> tail>", "", true, nil},
		{"<a | b> tail> | x>", "", true, []v099TestWant{{5, 18, 19}}},
		{"<a | b> <c> | x>", "", true, []v099TestWant{{5, 16, 17}}},
		{"<a | b", "", true, nil},
		{"x> | <a | b", "", true, []v099TestWant{{5, 2, 3}}},
		{"<a <b> | x>", "", true, []v099TestWant{{5, 11, 12}}},
		{"<> | x>", "", true, []v099TestWant{{5, 7, 8}}},
		{"<x> | y>", "", true, []v099TestWant{{5, 8, 9}}},
		{"<a | b> | <c | d>", "<a | b> | <c | d>", false, nil},
		{`<a \> b | c> | x>`, "<a > b | c> | x>", false, []v099TestWant{{5, 17, 18}}},
		{`<a \< b | c> | x>`, "<a < b | c> | x>", false, []v099TestWant{{5, 17, 18}}},
		{`<a\|b|c> | x>`, "<a|b|c> | x>", false, []v099TestWant{{5, 13, 14}}},
		{`<a|b> tail\|x> | y>`, "", true, []v099TestWant{{5, 19, 20}}},
		{`<a|b> tail\\|x>`, "", true, []v099TestWant{{5, 15, 16}}},
		{`<a\\>|b>`, "", true, []v099TestWant{{5, 8, 9}}},
		{`<a|b\> | x>`, "<a|b> | x>", false, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			row := v099TestText(t, tc.raw, tc.value, 4)
			if tc.invalid {
				row = v099TestInvalid(t, tc.raw, 4)
			}
			v099TestRun(t, v099TestTable(t, row), "current", tc.want)
		})
	}
	t.Run("unclosed cell does not consume next line", func(t *testing.T) {
		v099TestRun(t, v099TestTable(t, v099TestInvalid(t, "<a | b", 4), v099TestText(t, "x>", "x>", 4)),
			"current", []v099TestWant{{6, 2, 3}})
	})
}

func TestV099UnparsedAndContentElements(t *testing.T) {
	for _, row := range []model.Line{
		v099TestText(t, "x> | y", "x> | y", 4),
		v099TestInvalid(t, "x> | y", 4),
		v099TestLine(t, "x> | y", "invalid", 4, v099TestElement(t, "content", "x", "x", 1),
			v099TestElement(t, "unparsed", ">", "", 2), v099TestElement(t, "content", " | y", " | y", 3)),
		v099TestLine(t, "x> | y", "content", 4, v099TestElement(t, "content", "x>", "x>", 1),
			v099TestElement(t, "content", " | ", " | ", 3), v099TestElement(t, "content", "y", "y", 6)),
	} {
		t.Run(string(row.LineType)+"/"+strconv.Itoa(len(row.Elements)), func(t *testing.T) {
			v099TestRun(t, v099TestTable(t, row), "current", []v099TestWant{{5, 2, 3}})
		})
	}
}

func TestV099TablePartsAndRows(t *testing.T) {
	t.Run("header and body", func(t *testing.T) {
		result := v099TestTable(t, v099TestText(t, "x> | y", "x> | y", 4),
			v099TestLine(t, "---", "separator", 4, v099TestElement(t, "content", "---", "---", 1)),
			v099TestText(t, "x> | y", "x> | y", 4))
		v099TestRun(t, result, "current", []v099TestWant{{5, 2, 3}, {7, 2, 3}})
	})
	t.Run("blank rows and literal separator", func(t *testing.T) {
		v099TestRun(t, v099TestTable(t, v099TestLine(t, " \t", "blank", 4), v099TestText(t, `\---`, "---", 4),
			v099TestText(t, "x>", "x>", 4), v099TestLine(t, "", "blank", 4), v099TestText(t, "y>", "y>", 4)),
			"current", []v099TestWant{{7, 2, 3}, {9, 2, 3}})
	})
	t.Run("two independent tables", func(t *testing.T) {
		result := v099TestDocument(t, v099TestOpen(t, "table", "", "", 3), v099TestText(t, "x>", "x>", 4), v099TestClose(t, 4),
			v099TestOpen(t, "table", "", "", 3), v099TestText(t, "y>", "y>", 7), v099TestClose(t, 7))
		v099TestRun(t, result, "current", []v099TestWant{{5, 2, 3}, {8, 2, 3}})
	})
	for _, name := range []string{"table", "TABLE", "TaBlE"} {
		t.Run("fragment/"+name, func(t *testing.T) {
			result := v099TestDocument(t, v099TestOpen(t, "fragment", "intro", "identifier", 3), v099TestOpen(t, name, "", "", 4),
				v099TestText(t, "x>", "x>", 5), v099TestClose(t, 5), v099TestClose(t, 4))
			v099TestRun(t, result, "current", []v099TestWant{{6, 2, 3}})
		})
	}
}

func TestV099ExcludedContexts(t *testing.T) {
	for _, raw := range []string{"## Heading >", " \t## Heading >", "@unknown x>", " \t@note x>"} {
		t.Run("structural "+raw, func(t *testing.T) {
			v099TestRun(t, v099TestTable(t, v099TestInvalid(t, raw, 4), v099TestText(t, "Data", "Data", 4)), "current", nil)
		})
	}
	t.Run("recognized nested tag", func(t *testing.T) {
		result := v099TestTable(t, v099TestTag(t, "note", "x>", "content", 4), v099TestText(t, "Data", "Data", 4))
		v099TestParserError(t, result, 5, 0, "Вложение note запрещено внутри table.")
		v099TestRun(t, result, "current", nil)
	})
	for _, inner := range []string{"note", "table"} {
		t.Run("nested "+inner, func(t *testing.T) {
			result := v099TestDocument(t, v099TestOpen(t, "table", "", "", 3), v099TestOpen(t, inner, "", "", 4),
				v099TestText(t, "x>", "x>", 5), v099TestClose(t, 5), v099TestText(t, "y>", "y>", 4), v099TestClose(t, 4))
			v099TestParserError(t, result, 5, 0, "Вложение блока запрещено внутри table.")
			want := []v099TestWant{{8, 2, 3}}
			if inner == "table" {
				want = []v099TestWant{{6, 2, 3}, {8, 2, 3}}
			}
			v099TestRun(t, result, "current", want)
		})
	}
	t.Run("logical nested parent", func(t *testing.T) {
		result := v099TestDocument(t, v099TestOpen(t, "table", "", "", 3), v099TestTag(t, "task", "nested", "identifier", 4),
			v099TestText(t, "x>", "x>", 5), v099TestClose(t, 4))
		v099TestParserError(t, result, 5, 0, "Вложение task запрещено внутри table.")
		v099TestRun(t, result, "current", nil)
	})
	t.Run("title and outside table", func(t *testing.T) {
		result := v099TestDocument(t, v099TestText(t, "x>", "x>", 3), v099TestOpen(t, "table", "Name >", "title", 3),
			v099TestText(t, "Data", "Data", 5), v099TestClose(t, 5), v099TestText(t, "y>", "y>", 3))
		v099TestRun(t, result, "current", nil)
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			result := v099TestDocument(t, v099TestTag(t, "task", "practice", "identifier", 3), v099TestOpen(t, name, "", "", 4),
				v099TestText(t, "@table {", "@table {", 5), v099TestText(t, "x>", "x>", 5), v099TestClose(t, 5))
			v099TestRun(t, result, "current", nil)
		})
	}
}

func TestV099EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v099TestTable(t, v099TestText(t, "x> | y", "x> | y", 4))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v099TestWant{{5, 2, 3}}
			v099TestRun(t, result, "first", want)
			v099TestRun(t, v099TestTable(t, v099TestText(t, `x\>`, "x>", 4)), "other", nil)
			v099TestRun(t, result, "second", want)
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v099TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v099TestRun(t, result, "current", nil)
		})
	}
}

// v099TestInvalid задаёт ошибочную строку с целиком неразобранным Raw.
func v099TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v099TestLine(t, raw, "invalid", parent, v099TestElement(t, "unparsed", raw, "", 1))
}

// v099TestTable задаёт таблицу внутри шага с явно переданными строками данных.
func v099TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v099TestOpen(t, "table", "", "", 3)}, rows...)
	return v099TestDocument(t, append(body, v099TestClose(t, 4))...)
}

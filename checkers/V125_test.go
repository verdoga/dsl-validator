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

// v125TestWant задаёт явные диапазоны: строка, начало и исключающий конец.
type v125TestWant struct{ location, related [3]int }

func TestV125RegistrationAndVersions(t *testing.T) {
	registry := v125TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V125" {
			continue
		}
		count++
		if definition.Message != "Внутри @fragment записан заголовок." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V125 registrations = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v125TestFragment(t)
			result.Document.DSLVersion = &version
			before := v125TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v125TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed the input")
			}
		})
	}
}

func TestV125ReferenceAndExample(t *testing.T) {
	// DSL 1.30: версия, заголовок документа и шаг добавлены перед фрагментом.
	// Значимое содержимое обоих фрагментов из таблицы сохраняется дословно.
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           []v125TestWant
	}{
		{"reference", "@fragment f {\n@note Read the text.\n}\n@include f", nil, nil},
		{"example", "@fragment f {\n### Reading\n@note Read the text.\n}\n@include f",
			[]model.Line{v125TestInvalid(t, "### Reading", 4)},
			[]v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v125TestFragment(t, tc.rows...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v125TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV125MarkersAndSeparators(t *testing.T) {
	for _, tc := range []struct {
		name, raw  string
		start, end int
	}{
		{"first level", "# Reading", 1, 2},
		{"second level", "## Reading", 1, 3},
		{"third level", "### Reading", 1, 4},
		{"uppercase title", "### READING", 1, 4},
		{"Unicode title", "### Чтение 😀e\u0301", 1, 4},
		{"indentation", " \t### Reading\t ", 3, 6},
		{"several spaces", "###   Reading", 1, 4},
		{"no separator", "###Reading", 0, 0},
		{"bare marker", "###", 0, 0},
		{"trimmed separator", "### \t ", 0, 0},
		{"zero width space", "###\u200bReading", 0, 0},
		{"fourth level", "#### Reading", 0, 0},
		{"fifth level", "##### Reading", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []v125TestWant
			if tc.end != 0 {
				want = []v125TestWant{{[3]int{5, tc.start, tc.end}, [3]int{4, 1, 10}}}
			}
			got := v125TestRun(t, v125TestFragment(t, v125TestInvalid(t, tc.raw, 4)), "current", want)
			if tc.name == "fourth level" {
				v125TestOther(t, got, "V050", [][3]int{{5, 1, 5}})
			}
			if tc.name == "fifth level" {
				v125TestOther(t, got, "V050", [][3]int{{5, 1, 6}})
			}
		})
	}
	// Явный перечень White_Space, допустимых внутри Raw; CR/LF относятся к EOL.
	for _, separator := range []rune{' ', '\t', '\v', '\f', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run("separator "+strconv.QuoteRune(separator), func(t *testing.T) {
			v125TestRun(t, v125TestFragment(t, v125TestInvalid(t, "###"+string(separator)+"Reading", 4)),
				"current", []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
		})
	}
	t.Run("NBSP survives edge cleaning without title", func(t *testing.T) {
		v125TestRun(t, v125TestFragment(t, v125TestInvalid(t, "###\u00a0", 4)),
			"current", []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
	})
}

func TestV125PartialParsingAndLiteralText(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
		want []v125TestWant
	}{
		{"recognized marker and unparsed title", v125TestLine(t, "### Reading", "invalid", 4,
			v125TestElement(t, "heading-level", "###", "3", 1),
			v125TestElement(t, "unparsed", "Reading", "", 5)),
			[]v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}}},
		{"recognized marker without title", v125TestLine(t, "###", "invalid", 4,
			v125TestElement(t, "heading-level", "###", "3", 1),
			v125TestElement(t, "unparsed", "", "", 4)),
			[]v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}}},
		{"unparsed marker and parsed title", v125TestLine(t, "### Reading", "invalid", 4,
			v125TestElement(t, "unparsed", "###", "", 1),
			v125TestElement(t, "title", "Reading", "Reading", 5)),
			[]v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}}},
		// По уточнению пользователя unparsed проверяется независимо от Type строки.
		{"unparsed in content line", v125TestLine(t, "### Reading", "content", 4,
			v125TestElement(t, "unparsed", "### Reading", "", 1)),
			[]v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}}},
		{"escaped semantic heading", v125TestText(t, `\### Reading`, "### Reading", 4), nil},
		{"indented escape", v125TestText(t, " \t\\### Reading", "### Reading", 4), nil},
		{"escaped prefix in unparsed", v125TestLine(t, `\### Bad {`, "content", 4,
			v125TestElement(t, "content", `\### Bad`, "### Bad", 1),
			v125TestElement(t, "unparsed", "{", "", 10)), nil},
		{"mention in text", v125TestText(t, "Text ### Reading", "Text ### Reading", 4), nil},
		// DSL 1.14–15: NBSP не удаляется с края, а ＃ не является маркером DSL.
		// Эти строки являются содержимым, а не ошибочными попытками заголовка.
		{"Unicode indentation is not trimmed", v125TestText(t, "\u00a0### Reading", "\u00a0### Reading", 4), nil},
		{"full width hash", v125TestText(t, "＃ Reading", "＃ Reading", 4), nil},
		{"unparsed after text", v125TestLine(t, "Text ### Reading", "content", 4,
			v125TestElement(t, "content", "Text", "Text", 1),
			v125TestElement(t, "unparsed", "### Reading", "", 6)), nil},
		{"blank", v125TestLine(t, " \t", "blank", 4), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v125TestRun(t, v125TestFragment(t, tc.row), "current", tc.want)
		})
	}
}

func TestV125CountsAndSourceRanges(t *testing.T) {
	t.Run("levels and first and last children", func(t *testing.T) {
		result := v125TestDocument(t, v125TestOpen(t, "fragment", "f", 3),
			v125TestInvalid(t, "# One", 4), v125TestInvalid(t, "## Two", 4),
			v125TestInvalid(t, "### Three", 4), v125TestClose(t, 4), v125TestTag(t, "include", "f", "identifier", 3))
		v125TestRun(t, result, "current", []v125TestWant{
			{[3]int{5, 1, 2}, [3]int{4, 1, 10}}, {[3]int{6, 1, 3}, [3]int{4, 1, 10}},
			{[3]int{7, 1, 4}, [3]int{4, 1, 10}},
		})
	})
	t.Run("same headings and blank preserve independent findings", func(t *testing.T) {
		v125TestRun(t, v125TestFragment(t, v125TestInvalid(t, "### Reading", 4),
			v125TestLine(t, "", "blank", 4), v125TestInvalid(t, "### Reading", 4)), "current",
			[]v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}, {[3]int{7, 1, 4}, [3]int{4, 1, 10}}})
	})
	t.Run("indented mixed case fragment", func(t *testing.T) {
		result := v125TestDocument(t,
			v125TestLine(t, "  @FrAgMeNt f {", "block-start", 3,
				v125TestElement(t, "tag", "@FrAgMeNt", "fragment", 3),
				v125TestElement(t, "identifier", "f", "f", 13), v125TestElement(t, "block-open", "{", "{", 15)),
			v125TestInvalid(t, "\t ### Чтение", 4), v125TestClose(t, 4), v125TestTag(t, "include", "f", "identifier", 3))
		v125TestRun(t, result, "current", []v125TestWant{{[3]int{5, 3, 6}, [3]int{4, 3, 12}}})
	})
	t.Run("independent fragments", func(t *testing.T) {
		result := v125TestDocument(t, v125TestOpen(t, "fragment", "f", 3), v125TestInvalid(t, "### One", 4),
			v125TestClose(t, 4), v125TestOpen(t, "fragment", "g", 3), v125TestInvalid(t, "### Two", 7),
			v125TestClose(t, 7), v125TestTag(t, "include", "f", "identifier", 3), v125TestTag(t, "include", "g", "identifier", 3))
		v125TestRun(t, result, "current", []v125TestWant{
			{[3]int{5, 1, 4}, [3]int{4, 1, 10}}, {[3]int{8, 1, 4}, [3]int{7, 1, 10}},
		})
	})
	t.Run("includes do not multiply findings", func(t *testing.T) {
		result := v125TestDocument(t, v125TestOpen(t, "fragment", "f", 3), v125TestInvalid(t, "### Reading", 4),
			v125TestClose(t, 4), v125TestTag(t, "include", "f", "identifier", 3), v125TestTag(t, "include", "f", "identifier", 3))
		v125TestRun(t, result, "current", []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
	})
}

func TestV125AncestorContexts(t *testing.T) {
	for _, tc := range []struct {
		name         string
		rows         []model.Line
		want         []v125TestWant
		parserErrors []int
	}{
		{"nested note", []model.Line{v125TestOpen(t, "note", "", 4), v125TestInvalid(t, "### Reading", 5),
			v125TestClose(t, 5)}, []v125TestWant{{[3]int{6, 1, 4}, [3]int{4, 1, 10}}}, nil},
		{"two nested content blocks", []model.Line{v125TestOpen(t, "instr", "", 4),
			v125TestText(t, "Read.", "Read.", 5), v125TestOpen(t, "hint", "", 5),
			v125TestInvalid(t, "### Reading", 7), v125TestClose(t, 7), v125TestClose(t, 5)},
			[]v125TestWant{{[3]int{8, 1, 4}, [3]int{4, 1, 10}}}, nil},
		{"nearest fragment", []model.Line{v125TestOpen(t, "fragment", "g", 4),
			v125TestInvalid(t, "### Reading", 5), v125TestClose(t, 5)},
			[]v125TestWant{{[3]int{6, 1, 4}, [3]int{5, 1, 10}}}, []int{5}},
		{"direct table", []model.Line{v125TestOpen(t, "table", "", 4),
			v125TestInvalid(t, "### Reading", 5), v125TestText(t, "A | B", "A | B", 5), v125TestClose(t, 5)}, nil, nil},
		{"table is not nearest block", []model.Line{v125TestOpen(t, "table", "", 4),
			v125TestOpen(t, "note", "", 5), v125TestInvalid(t, "### Reading", 6), v125TestClose(t, 6),
			v125TestText(t, "A | B", "A | B", 5), v125TestClose(t, 5)},
			[]v125TestWant{{[3]int{7, 1, 4}, [3]int{4, 1, 10}}}, []int{6}},
		{"nonblock parent inside table", []model.Line{v125TestOpen(t, "table", "", 4),
			v125TestTag(t, "task", "nested", "identifier", 5), v125TestInvalid(t, "### Reading", 6),
			v125TestText(t, "A | B", "A | B", 6), v125TestTag(t, "endtask", "", "", 5), v125TestClose(t, 5)}, nil, []int{6, 9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v125TestFragment(t, tc.rows...)
			for _, number := range tc.parserErrors {
				v125TestParserError(t, result, number, 0, "Вложение тега запрещено в этом блоке.")
			}
			got := v125TestRun(t, result, "current", tc.want)
			if tc.name == "direct table" {
				v125TestOther(t, got, "V096", [][3]int{{6, 1, 4}})
			}
			if tc.name == "table is not nearest block" {
				v125TestOther(t, got, "V096", nil)
			}
		})
	}
	t.Run("fragment inside task", func(t *testing.T) {
		result := v125TestDocument(t, v125TestTag(t, "task", "t", "identifier", 3),
			v125TestOpen(t, "fragment", "f", 4), v125TestInvalid(t, "### Reading", 5), v125TestClose(t, 5),
			v125TestTag(t, "include", "f", "identifier", 4), v125TestTag(t, "endtask", "", "", 3))
		v125TestRun(t, result, "current", []v125TestWant{{[3]int{6, 1, 4}, [3]int{5, 1, 10}}})
	})
	for _, name := range []string{"editor", "multifill"} {
		for _, after := range []bool{false, true} {
			t.Run(name+"/after="+strconv.FormatBool(after), func(t *testing.T) {
				rows := []model.Line{v125TestOpen(t, name, "", 4),
					v125TestText(t, "### Reading", "### Reading", 5), v125TestClose(t, 5)}
				var want []v125TestWant
				if after {
					rows = append(rows, v125TestInvalid(t, "### Outside literal", 4))
					want = []v125TestWant{{[3]int{8, 1, 4}, [3]int{4, 1, 10}}}
				}
				result := v125TestFragment(t, rows...)
				// DSL 3.12.12–13 запрещает сам контейнер, но его содержимое литерально.
				v125TestParserError(t, result, 5, 0, "Вложение тега запрещено внутри fragment.")
				v125TestRun(t, result, "current", want)
			})
		}
	}
	t.Run("no fragment and heading after closing", func(t *testing.T) {
		result := v125TestDocument(t, v125TestOpen(t, "note", "", 3), v125TestInvalid(t, "### Nested", 4),
			v125TestClose(t, 4), v125TestOpen(t, "fragment", "f", 3),
			v125TestTag(t, "note", "Read.", "content", 7), v125TestClose(t, 7),
			v125TestLine(t, "### Next", "heading", 0, v125TestElement(t, "heading-level", "###", "3", 1),
				v125TestElement(t, "title", "Next", "Next", 5)), v125TestTag(t, "include", "f", "identifier", 10))
		v125TestRun(t, result, "current", nil)
	})
	t.Run("unparsed placeholder does not turn multifill text into heading", func(t *testing.T) {
		// DSL 4.3.2: два подчёркивания ошибочны; по 4.4.11 начальное ###
		// всё равно остаётся литеральным содержимым multifill.
		placeholder := v125TestElement(t, "placeholder", "_____", "", 1)
		placeholder.Value = nil // Обычный пропуск без ответа, как в контракте модели.
		result := v125TestFragment(t, v125TestOpen(t, "multifill", "", 4),
			v125TestLine(t, "### __", "content", 5,
				v125TestElement(t, "content", "###", "###", 1),
				v125TestElement(t, "unparsed", "__", "", 5)),
			v125TestLine(t, "_____", "content", 5, placeholder),
			v125TestClose(t, 5))
		v125TestParserError(t, result, 5, 0, "Вложение multifill запрещено внутри fragment.")
		v125TestRun(t, result, "current", nil)
	})
}

func TestV125EmptyEndingsAndHistory(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v125TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v125TestRun(t, result, "current", nil)
		})
	}
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(string(eol))+"/final="+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v125TestFragment(t, v125TestInvalid(t, "### Reading", 4))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}}
				v125TestRun(t, result, "first", want)
				v125TestRun(t, v125TestFragment(t), "other-document", nil)
				v125TestRun(t, result, "second", want)
			})
		}
	}
}

// v125TestElement создаёт входной элемент, не вычисляя ожидаемые диапазоны.
func v125TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v125TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v125TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v125TestTag задаёт известный тег с явно выбранным типом аргумента.
func v125TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v125TestElement(t, "tag", raw, name, 1)}
	if argument != "" {
		elements = append(elements, v125TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v125TestLine(t, raw, "tag", parent, elements...)
}

// v125TestOpen задаёт блочную форму; непустой аргумент здесь является ID fragment.
func v125TestOpen(t *testing.T, name, id string, parent int) model.Line {
	t.Helper()
	line := v125TestTag(t, name, id, "identifier", parent)
	line.Elements = append(line.Elements, v125TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v125TestClose закрывает явно указанный фигурный блок.
func v125TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v125TestLine(t, "}", "block-end", parent, v125TestElement(t, "block-close", "}", "}", 1))
}

// v125TestText задаёт исходное и семантическое содержимое независимо.
func v125TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v125TestLine(t, raw, "content", parent, v125TestElement(t, "content", raw, value, 1))
}

// v125TestInvalid сохраняет вложенную ошибочную конструкцию вместе с родителем.
func v125TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v125TestLine(t, raw, "invalid", parent, v125TestElement(t, "unparsed", raw, "", 1))
}

// v125TestFragment помещает строки начиная с пятой внутрь fragment на строке 4.
// Завершает фрагмент содержательным note из эталона, закрытием и включением.
func v125TestFragment(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v125TestOpen(t, "fragment", "f", 3)}, rows...)
	body = append(body, v125TestTag(t, "note", "Read the text.", "content", 4),
		v125TestClose(t, 4), v125TestTag(t, "include", "f", "identifier", 3))
	return v125TestDocument(t, body...)
}

// v125TestDocument добавляет обязательный контекст, нумерацию и ошибки unparsed.
// Родители и классификация строк заданы вызывающим тестом, дерево не разбирается.
func v125TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v125TestTag(t, "dsl-version", "1.2", "version", 0),
		v125TestLine(t, "# Lesson", "heading", 0, v125TestElement(t, "heading-level", "#", "1", 1),
			v125TestElement(t, "title", "Lesson", "Lesson", 3)),
		v125TestLine(t, "### Practice", "heading", 0, v125TestElement(t, "heading-level", "###", "3", 1),
			v125TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			if line.Elements[j].ElementType == "unparsed" {
				v125TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v125TestParserError добавляет согласованную исходную ошибку и обратную ссылку.
func v125TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v125TestRegistry создаёт реестр только через полный штатный каталог.
func v125TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v125TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v125TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v125TestRun выполняет полный цикл и проверяет явные диапазоны и сохранность модели.
// Этот помощник и следующий v125TestOther написаны заново при восстановлении:
// это не точная копия утраченного исторического хвоста файла.
func v125TestRun(t *testing.T, result *model.Result, processingID string, want []v125TestWant) []model.Diagnostic {
	t.Helper()
	before := v125TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v125TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v125TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	current := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, current)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Diagnostics) < oldCount {
		t.Fatal("previous diagnostics removed")
	}
	added := result.Diagnostics[oldCount:]
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.Source != processingID {
			t.Fatalf("new diagnostic %q has source %q, want %q", entry.ID, entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V125" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V125 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != "Внутри @fragment записан заголовок." ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.Source != processingID || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V125 occurrence %d = %+v, want location %+v, no detail and related %+v", i, entry, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V125 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V125 created an element error link")
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
	if err := json.Unmarshal([]byte(v125TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	if restored.Processing[len(original.Processing)] != current {
		t.Fatal("current processing entry changed")
	}
	// Исключаются только новые записи и производные флаги/ссылки.
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
				t.Fatal("previous element error links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v125TestSnapshot(t, &restored) != before {
		t.Fatal("source data or previous history changed")
	}
	return added
}

// v125TestOther проверяет число и исходные диапазоны сопутствующей диагностики.
func v125TestOther(t *testing.T, added []model.Diagnostic, code diagnostics.Code, want [][3]int) {
	t.Helper()
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == code {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%s count = %d, want %d; got %+v", code, len(got), len(want), got)
	}
	for i, expected := range want {
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if got[i].Location == nil || *got[i].Location != location {
			t.Fatalf("%s occurrence %d = %+v, want location %+v", code, i, got[i], location)
		}
	}
}

func TestV125PhysicalRootHeading(t *testing.T) {
	for _, marker := range []string{"#", "##", "###"} {
		t.Run(marker, func(t *testing.T) {
			result := v125TestFragment(t, v125TestRootHeading(t, marker, "Reading"))
			v125TestParserError(t, result, 5, 0, "Заголовок запрещён внутри fragment.")
			// API требует nil ParentLine и нулевую глубину настоящего heading.
			if result.Lines[4].ParentLine != nil || result.Lines[4].NestingLevel != 0 {
				t.Fatal("heading fixture is not at the logical root")
			}
			end := map[string]int{"#": 2, "##": 3, "###": 4}[marker]
			v125TestRun(t, result, "current", []v125TestWant{{[3]int{5, 1, end}, [3]int{4, 1, 10}}})
		})
	}
}

func TestV125PhysicalContentAfterHeading(t *testing.T) {
	t.Run("parsed closing belongs to the fragment", func(t *testing.T) {
		result := v125TestDocument(t,
			v125TestOpen(t, "fragment", "f", 3),
			v125TestRootHeading(t, "###", "Reading"),
			v125TestInvalid(t, "## Before close", 5),
			v125TestClose(t, 4),
			v125TestInvalid(t, "### After close", 5),
			v125TestTag(t, "include", "f", "identifier", 5))
		v125TestRun(t, result, "current", []v125TestWant{
			{[3]int{5, 1, 4}, [3]int{4, 1, 10}},
			{[3]int{6, 1, 3}, [3]int{4, 1, 10}},
		})
	})
	t.Run("unparsed closing survives the lost parser context", func(t *testing.T) {
		result := v125TestDocument(t,
			v125TestOpen(t, "fragment", "f", 3),
			v125TestRootHeading(t, "###", "Reading"),
			v125TestInvalid(t, " \t}\t ", 5),
			v125TestInvalid(t, "### After close", 5),
			v125TestTag(t, "include", "f", "identifier", 5))
		v125TestRun(t, result, "current", []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
	})
	t.Run("partially parsed closing still closes the fragment", func(t *testing.T) {
		result := v125TestDocument(t,
			v125TestOpen(t, "fragment", "f", 3),
			v125TestRootHeading(t, "###", "Reading"),
			v125TestLine(t, "} ?", "invalid", 5,
				v125TestElement(t, "block-close", "}", "}", 1),
				v125TestElement(t, "unparsed", "?", "", 3)),
			v125TestInvalid(t, "### After close", 5))
		v125TestRun(t, result, "current", []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
	})
	t.Run("unclosed fragment continues to EOF", func(t *testing.T) {
		result := v125TestDocument(t,
			v125TestOpen(t, "fragment", "f", 3),
			v125TestRootHeading(t, "###", "Reading"),
			v125TestInvalid(t, "# Last", 5))
		v125TestRun(t, result, "current", []v125TestWant{
			{[3]int{5, 1, 4}, [3]int{4, 1, 10}},
			{[3]int{6, 1, 2}, [3]int{4, 1, 10}},
		})
	})
}

func TestV125PhysicalNestedBoundaries(t *testing.T) {
	result := v125TestDocument(t,
		v125TestOpen(t, "fragment", "f", 3),
		v125TestOpen(t, "note", "", 4),
		v125TestRootHeading(t, "###", "Nested"),
		v125TestInvalid(t, "### Note child", 6),
		v125TestClose(t, 5),
		v125TestOpen(t, "fragment", "g", 6),
		v125TestRootHeading(t, "##", "Inner"),
		v125TestInvalid(t, "## Inner child", 10),
		v125TestClose(t, 9),
		v125TestInvalid(t, "# Outer child", 10),
		v125TestClose(t, 4),
		v125TestRootHeading(t, "###", "Outside"))
	v125TestRun(t, result, "current", []v125TestWant{
		{[3]int{6, 1, 4}, [3]int{4, 1, 10}},
		{[3]int{7, 1, 4}, [3]int{4, 1, 10}},
		{[3]int{10, 1, 3}, [3]int{9, 1, 10}},
		{[3]int{11, 1, 3}, [3]int{9, 1, 10}},
		{[3]int{13, 1, 2}, [3]int{4, 1, 10}},
	})
}

func TestV125PhysicalTableBoundaries(t *testing.T) {
	result := v125TestDocument(t,
		v125TestOpen(t, "fragment", "f", 3),
		v125TestOpen(t, "table", "", 4),
		v125TestRootHeading(t, "###", "Table"),
		v125TestInvalid(t, "### Table child", 6),
		v125TestOpen(t, "note", "", 6),
		v125TestRootHeading(t, "###", "Note"),
		v125TestClose(t, 8),
		v125TestInvalid(t, "### Table again", 9),
		v125TestClose(t, 5),
		v125TestInvalid(t, "## Fragment again", 9),
		v125TestClose(t, 4),
		v125TestInvalid(t, "### Outside", 9))
	v125TestRun(t, result, "current", []v125TestWant{
		{[3]int{9, 1, 4}, [3]int{4, 1, 10}},
		{[3]int{13, 1, 3}, [3]int{4, 1, 10}},
	})
}

func TestV125PhysicalLiteralBoundaries(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			result := v125TestDocument(t,
				v125TestOpen(t, "fragment", "f", 3),
				v125TestRootHeading(t, "###", "Before literal"),
				v125TestOpen(t, name, "", 5),
				v125TestText(t, "### Literal", "### Literal", 5),
				v125TestText(t, `@fragment phantom \{`, "@fragment phantom {", 5),
				v125TestText(t, `@note \{`, "@note {", 5),
				v125TestText(t, `\}`, "}", 5),
				v125TestText(t, "### Still literal", "### Still literal", 5),
				v125TestInvalid(t, " \t}\t ", 5),
				v125TestInvalid(t, "## Fragment again", 5),
				v125TestClose(t, 4),
				v125TestRootHeading(t, "###", "Outside"))
			v125TestRun(t, result, "current", []v125TestWant{
				{[3]int{5, 1, 4}, [3]int{4, 1, 10}},
				{[3]int{13, 1, 3}, [3]int{4, 1, 10}},
			})
		})
	}
}

func TestV125PhysicalPartialOpening(t *testing.T) {
	opening := v125TestOpen(t, "fragment", "f", 3)
	opening.Raw += " ?"
	opening.LineType = "invalid"
	opening.Elements = append(opening.Elements, v125TestElement(t, "unparsed", "?", "", 15))
	result := v125TestDocument(t, opening,
		v125TestRootHeading(t, "###", "Reading"),
		v125TestInvalid(t, "}", 5),
		v125TestRootHeading(t, "###", "Outside"))
	v125TestRun(t, result, "current", []v125TestWant{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
}

// v125TestRootHeading задаёт настоящий heading с обязательным корневым родителем.
func v125TestRootHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v125TestLine(t, marker+" "+title, "heading", 0,
		v125TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v125TestElement(t, "title", title, title, len(marker)+2))
}

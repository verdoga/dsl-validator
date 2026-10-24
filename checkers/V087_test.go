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

const v087TestMessage = "В блоке повторён разделитель ---."

// v087TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v087TestWant struct{ location, related [3]int }

func TestV087RegistrationAndVersions(t *testing.T) {
	registry := v087TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V087" {
			continue
		}
		count++
		if definition.Message != v087TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V087 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v087TestDocument(t)
			result.Document.DSLVersion = &version
			before := v087TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v087TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV087ReferenceAndExample(t *testing.T) {
	// Фрагменты начинаются со строки 4 после версии и двух заголовков.
	// HTML-запись &#124; в таблице задания представляет символ | в DSL.
	for _, tc := range []struct {
		name, fragment string
		result         *model.Result
		want           []v087TestWant
	}{
		{"reference", "@example {\nHe go.\n---\nHe goes.\n}", v087TestBlockDocument(t, "example",
			v087TestText(t, "He go.", "He go.", 4), v087TestSeparator(t, "---", 4, false),
			v087TestText(t, "He goes.", "He goes.", 4)), nil},
		{"example", "@table {\nName | Form\n---\nA | B\n---\nC | D\n}", v087TestBlockDocument(t, "table",
			v087TestText(t, "Name | Form", "Name | Form", 4), v087TestSeparator(t, "---", 4, false),
			v087TestText(t, "A | B", "A | B", 4), v087TestSeparator(t, "---", 4, false),
			v087TestText(t, "C | D", "C | D", 4)), []v087TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v087TestRun(t, tc.result, "current", tc.want)
		})
	}
}

func TestV087CountsAndWhitespace(t *testing.T) {
	for _, name := range []string{"example", "table"} {
		t.Run(name, func(t *testing.T) {
			// Дополнительная таблица имеет одну колонку во всех строках.
			for _, tc := range []struct {
				name string
				body []model.Line
				want []v087TestWant
			}{
				{"zero", []model.Line{v087TestText(t, "Read.", "Read.", 4)}, nil},
				{"one", []model.Line{v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestText(t, "After.", "After.", 4)}, nil},
				{"three", []model.Line{v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestText(t, "Middle.", "Middle.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestText(t, "Later.", "Later.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestText(t, "After.", "After.", 4)},
					[]v087TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}, {[3]int{10, 1, 4}, [3]int{6, 1, 4}}}},
				{"adjacent", []model.Line{v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestSeparator(t, "---", 4, false), v087TestText(t, "After.", "After.", 4)},
					[]v087TestWant{{[3]int{7, 1, 4}, [3]int{6, 1, 4}}}},
				{"indent and trailing whitespace", []model.Line{v087TestText(t, "До😀.", "До😀.", 4),
					v087TestSeparator(t, " \t--- \t", 4, false), v087TestText(t, "После.", "После.", 4),
					v087TestSeparator(t, "\t--- ", 4, false), v087TestText(t, "Конец.", "Конец.", 4)},
					[]v087TestWant{{[3]int{8, 2, 5}, [3]int{6, 3, 6}}}},
				{"blank does not reset", []model.Line{v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestLine(t, " \t", "blank", 4), v087TestText(t, "Middle.", "Middle.", 4),
					v087TestSeparator(t, "---", 4, false), v087TestText(t, "After.", "After.", 4)},
					[]v087TestWant{{[3]int{9, 1, 4}, [3]int{6, 1, 4}}}},
				{"empty line does not reset", []model.Line{v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestLine(t, "", "blank", 4), v087TestText(t, "Middle.", "Middle.", 4),
					v087TestSeparator(t, "---", 4, false), v087TestText(t, "After.", "After.", 4)},
					[]v087TestWant{{[3]int{9, 1, 4}, [3]int{6, 1, 4}}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					v087TestRun(t, v087TestBlockDocument(t, name, tc.body...), "current", tc.want)
				})
			}
			for _, firstInvalid := range []bool{false, true} {
				for _, secondInvalid := range []bool{false, true} {
					t.Run("unparsed "+strconv.FormatBool(firstInvalid)+strconv.FormatBool(secondInvalid), func(t *testing.T) {
						result := v087TestBlockDocument(t, name, v087TestText(t, "Before.", "Before.", 4),
							v087TestSeparator(t, "---", 4, firstInvalid), v087TestText(t, "Middle.", "Middle.", 4),
							v087TestSeparator(t, "---", 4, secondInvalid), v087TestText(t, "After.", "After.", 4))
						v087TestRun(t, result, "current", []v087TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}})
					})
				}
			}
		})
	}
}

func TestV087RawNotSemanticValue(t *testing.T) {
	for _, name := range []string{"example", "table"} {
		for _, tc := range []struct{ raw, value string }{
			{`\---`, "---"}, {" \t\\---\t ", "---"}, {"--", "--"}, {"----", "----"},
			{"- - -", "- - -"}, {"Text ---", "Text ---"}, {"--- Text", "--- Text"},
			{"———", "———"}, {"−−−", "−−−"}, {"--–", "--–"},
		} {
			t.Run(name+"/"+tc.raw, func(t *testing.T) {
				result := v087TestBlockDocument(t, name, v087TestText(t, "Before.", "Before.", 4),
					v087TestText(t, tc.raw, tc.value, 4), v087TestText(t, "Middle.", "Middle.", 4),
					v087TestSeparator(t, "---", 4, false), v087TestText(t, "After.", "After.", 4))
				v087TestRun(t, result, "current", nil)
			})
		}
		// Unicode-пробелы не входят в разрешённую обрезку U+0020/U+0009.
		for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002",
			"\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a",
			"\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
			for _, raw := range []string{space + "---", "---" + space} {
				t.Run(name+"/"+strconv.Quote(raw), func(t *testing.T) {
					result := v087TestBlockDocument(t, name, v087TestText(t, raw, raw, 4),
						v087TestSeparator(t, "---", 4, false), v087TestText(t, "After.", "After.", 4))
					v087TestRun(t, result, "current", nil)
				})
			}
		}
	}
	t.Run("table cells", func(t *testing.T) {
		result := v087TestBlockDocument(t, "table", v087TestText(t, "Name | Form", "Name | Form", 4),
			v087TestText(t, "--- | ---", "--- | ---", 4), v087TestSeparator(t, "---", 4, false),
			v087TestText(t, "--- | ---", "--- | ---", 4), v087TestText(t, "A | B", "A | B", 4))
		v087TestRun(t, result, "current", nil)
	})
}

func TestV087IndependentBlocksAndDepth(t *testing.T) {
	for _, first := range []string{"example", "table"} {
		for _, second := range []string{"example", "table"} {
			t.Run(first+" then "+second, func(t *testing.T) {
				result := v087TestDocument(t, v087TestOpen(t, first, "", "", 3),
					v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
					v087TestText(t, "After.", "After.", 4), v087TestClose(t, 4),
					v087TestOpen(t, second, "", "", 3), v087TestText(t, "Before.", "Before.", 9),
					v087TestSeparator(t, "---", 9, false), v087TestText(t, "Middle.", "Middle.", 9),
					v087TestSeparator(t, "---", 9, false), v087TestText(t, "After.", "After.", 9), v087TestClose(t, 9))
				v087TestRun(t, result, "current", []v087TestWant{{[3]int{13, 1, 4}, [3]int{11, 1, 4}}})
			})
		}
	}
	for _, name := range []string{"EXAMPLE", "TABLE"} {
		t.Run("fragment/"+name, func(t *testing.T) {
			result := v087TestDocument(t, v087TestOpen(t, "fragment", "note", "identifier", 3),
				v087TestOpen(t, name, "Сравнение", "title", 4), v087TestText(t, "Before.", "Before.", 5),
				v087TestSeparator(t, "---", 5, false), v087TestText(t, "Middle.", "Middle.", 5),
				v087TestSeparator(t, "---", 5, false), v087TestText(t, "After.", "After.", 5),
				v087TestClose(t, 5), v087TestClose(t, 4), v087TestTag(t, "include", "note", "identifier", 3))
			v087TestRun(t, result, "current", []v087TestWant{{[3]int{9, 1, 4}, [3]int{7, 1, 4}}})
		})
	}
}

func TestV087NestedConstructions(t *testing.T) {
	for _, inner := range []string{"text", "table"} {
		t.Run(inner, func(t *testing.T) {
			// Вложение запрещено DSL 3.0.7, но парсер сохранил структуру.
			// Строки text являются обычным содержимым; table имеет разделители.
			first := v087TestText(t, "---", "---", 7)
			second := v087TestText(t, "---", "---", 7)
			want := []v087TestWant{{[3]int{15, 1, 4}, [3]int{6, 1, 4}}}
			if inner == "table" {
				first = v087TestSeparator(t, "---", 7, false)
				second = v087TestSeparator(t, "---", 7, true)
				want = []v087TestWant{{[3]int{11, 1, 4}, [3]int{9, 1, 4}}, {[3]int{15, 1, 4}, [3]int{6, 1, 4}}}
			}
			result := v087TestDocument(t, v087TestOpen(t, "example", "", "", 3),
				v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
				v087TestOpen(t, inner, "", "", 4), v087TestText(t, "Inner before.", "Inner before.", 7), first,
				v087TestText(t, "Inner middle.", "Inner middle.", 7), second,
				v087TestText(t, "Inner after.", "Inner after.", 7), v087TestClose(t, 7),
				v087TestText(t, "Middle.", "Middle.", 4), v087TestSeparator(t, "---", 4, false),
				v087TestText(t, "After.", "After.", 4), v087TestClose(t, 4))
			v087TestParserError(t, result, 7, 0, "Недопустимое вложение блока.")
			v087TestRun(t, result, "current", want)
		})
	}
	t.Run("logical nested construction is not direct content", func(t *testing.T) {
		result := v087TestDocument(t, v087TestOpen(t, "example", "", "", 3),
			v087TestText(t, "Before.", "Before.", 4), v087TestSeparator(t, "---", 4, false),
			v087TestTag(t, "task", "nested", "identifier", 4),
			v087TestText(t, "---", "---", 7), v087TestText(t, "---", "---", 7), v087TestClose(t, 4))
		v087TestParserError(t, result, 7, 0, "Недопустимое вложение task.")
		v087TestRun(t, result, "current", nil)
	})
}

func TestV087ExcludedContexts(t *testing.T) {
	for _, container := range []string{"editor", "multifill", "hint"} {
		t.Run(container, func(t *testing.T) {
			result := v087TestDocument(t, v087TestOpen(t, container, "", "", 3),
				v087TestText(t, "---", "---", 4), v087TestText(t, "---", "---", 4), v087TestClose(t, 4),
				v087TestText(t, "Read.", "Read.", 3))
			v087TestRun(t, result, "current", nil)
		})
	}
	t.Run("outside block in document step", func(t *testing.T) {
		// Родитель обычного содержимого после ### — текущий шаг.
		v087TestRun(t, v087TestDocument(t, v087TestText(t, "---", "---", 3),
			v087TestText(t, "---", "---", 3)), "current", nil)
	})
	t.Run("inline example is not block", func(t *testing.T) {
		v087TestRun(t, v087TestDocument(t, v087TestTag(t, "example", "Read.", "content", 3),
			v087TestText(t, "---", "---", 3), v087TestText(t, "---", "---", 3)), "current", nil)
	})
}

func TestV087EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v087TestBlockDocument(t, "table", v087TestText(t, "Before.", "Before.", 4),
				v087TestSeparator(t, "---", 4, false), v087TestText(t, "Middle.", "Middle.", 4),
				v087TestSeparator(t, "---", 4, false), v087TestText(t, "After.", "After.", 4))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v087TestWant{{[3]int{8, 1, 4}, [3]int{6, 1, 4}}}
			v087TestRun(t, result, "first", want)
			v087TestRun(t, v087TestBlockDocument(t, "example", v087TestText(t, "Read.", "Read.", 4)), "other", nil)
			v087TestRun(t, result, "second", want)
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v087TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v087TestRun(t, result, "current", nil)
		})
	}
}

// v087TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v087TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v087TestLine(t, raw, "invalid", parent, v087TestElement(t, "unparsed", raw, "", 1))
	}
	return v087TestLine(t, raw, "separator", parent)
}

// v087TestBlockDocument окружает явно заданное содержимое блоком на строке 4.
func v087TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v087TestOpen(t, name, "", "", 3)}, body...)
	return v087TestDocument(t, append(lines, v087TestClose(t, 4))...)
}

// v087TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v087TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v087TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v087TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v087TestTag задаёт известное имя и явно указанный тип аргумента.
func v087TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v087TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v087TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v087TestLine(t, raw, "tag", parent, elements...)
}

// v087TestOpen добавляет открытие к известному объявлению блока.
func v087TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v087TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v087TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v087TestClose задаёт закрытие конкретного фигурного блока.
func v087TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v087TestLine(t, "}", "block-end", parent, v087TestElement(t, "block-close", "}", "}", 1))
}

// v087TestText сохраняет исходное и семантическое содержимое независимо.
func v087TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v087TestLine(t, raw, "content", parent, v087TestElement(t, "content", raw, value, 1))
}

// v087TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v087TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v087TestTag(t, "dsl-version", "1.2", "version", 0),
		v087TestLine(t, "# Lesson", "heading", 0, v087TestElement(t, "heading-level", "#", "1", 1),
			v087TestElement(t, "title", "Lesson", "Lesson", 3)),
		v087TestLine(t, "### Practice", "heading", 0, v087TestElement(t, "heading-level", "###", "3", 1),
			v087TestElement(t, "title", "Practice", "Practice", 5)),
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
				v087TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v087TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v087TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v087TestRegistry использует полный штатный каталог без отдельной регистрации.
func v087TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v087TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v087TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v087TestRun сравнивает новые V087 после полного Prepare/Run с явным ожиданием.
func v087TestRun(t *testing.T, result *model.Result, processingID string, want []v087TestWant) {
	t.Helper()
	before := v087TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v087TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v087TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V087" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V087 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v087TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V087 occurrence %d = %+v, want location %+v, related %+v, no detail", i, entry, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V087 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V087 created an element error link")
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
	if err := json.Unmarshal([]byte(v087TestSnapshot(t, result)), &restored); err != nil {
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
	if v087TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

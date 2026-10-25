package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

type v161Expected struct {
	main, related [3]int
}

func TestV161Registration(t *testing.T) {
	registry := v161Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V161" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Внутри блока выбора записан заголовок." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V161 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV161ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v161Expected
	}{
		{"reference", "@choice Choose a country. {\nSpain\nFrance\n}", []model.Line{
			v161Line(t, "@choice Choose a country. {", "block-start", 2, 1,
				v161Element(t, "tag", "@choice", "choice", 1), v161Element(t, "content", "Choose a country.", "Choose a country.", 9), v161Element(t, "block-open", "{", "{", 27)),
			v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2)}, nil},
		{"example", "@choice {\n### Countries\nSpain\nFrance\n}", []model.Line{
			v161Open(t, "choice", 2, 1), v161Marked(t, "", "###", " Countries", 3, 2),
			v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2)},
			[]v161Expected{{[3]int{4, 1, 4}, [3]int{3, 1, 10}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v161Document(t, tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v161Run(t, result, tc.want)
			v161Run(t, result, tc.want) // История не влияет на следующий запуск.
		})
	}
}

func TestV161Markers(t *testing.T) {
	for _, container := range []struct {
		name string
		end  int
	}{{"choice", 10}, {"multichoice", 15}} {
		for _, tc := range []struct {
			name, prefix, marker, tail string
			start, end                 int
		}{
			{"first level", "", "#", " Countries", 1, 2},
			{"second level", "", "##", " Countries", 1, 3},
			{"third level", "", "###", " Countries", 1, 4},
			{"indentation", " \t", "###", " Countries", 3, 6},
			{"tab separator", "", "###", "\tCountries", 1, 4},
			{"NBSP separator", "", "###", "\u00a0Countries", 1, 4},
			{"em space separator", "", "###", "\u2003Countries", 1, 4},
			{"no title", "", "###", " ", 1, 4},
			{"Unicode title", "", "###", " Страны 🌍", 1, 4},
			{"uppercase title", "", "###", " COUNTRIES", 1, 4},
		} {
			for _, marked := range []bool{false, true} {
				t.Run(container.name+"/"+tc.name+"/marked="+strconv.FormatBool(marked), func(t *testing.T) {
					candidate := v161Invalid(t, tc.prefix+tc.marker+tc.tail, 3, 2)
					if marked {
						candidate = v161Marked(t, tc.prefix, tc.marker, tc.tail, 3, 2)
					}
					v161Run(t, v161Choice(t, container.name, candidate),
						[]v161Expected{{[3]int{4, tc.start, tc.end}, [3]int{3, 1, container.end}}})
				})
			}
		}
	}
	t.Run("recognized marker without separator", func(t *testing.T) {
		v161Run(t, v161Choice(t, "choice", v161Marked(t, "", "###", "Countries", 3, 2)),
			[]v161Expected{{[3]int{4, 1, 4}, [3]int{3, 1, 10}}})
	})
	t.Run("unparsed on content line", func(t *testing.T) {
		candidate := v161Invalid(t, "### Countries", 3, 2)
		candidate.LineType = model.LineTypeContent
		v161Run(t, v161Choice(t, "choice", candidate), []v161Expected{{[3]int{4, 1, 4}, [3]int{3, 1, 10}}})
	})
}

func TestV161ExcludedSource(t *testing.T) {
	for _, raw := range []string{"###Countries", "###", "#", "##", "###\\ Countries"} {
		t.Run("unparsed "+strconv.Quote(raw), func(t *testing.T) {
			v161Run(t, v161Choice(t, "choice", v161Invalid(t, raw, 3, 2)), nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\### Countries`, "### Countries"}, {" \t\\### Countries", "### Countries"},
		{`\\### Countries`, `\### Countries`}, {"Read ### Countries", "Read ### Countries"},
		{"＃＃＃ Countries", "＃＃＃ Countries"}, {"\u00a0### Countries", "\u00a0### Countries"},
		{" \t\u2003### Countries", "\u2003### Countries"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v161Run(t, v161Choice(t, "choice", v161Text(t, tc.raw, tc.value, 3, 2)), nil)
		})
	}
	for _, raw := range []string{"", " \t"} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v161Run(t, v161Choice(t, "choice", v161Line(t, raw, "blank", 3, 2)), nil)
		})
	}
	for _, tc := range []struct {
		raw string
		end int
	}{{"#### Countries", 5}, {"##### Countries", 6}} {
		t.Run("V050 "+tc.raw, func(t *testing.T) {
			entries := v161Run(t, v161Choice(t, "choice", v161Invalid(t, tc.raw, 3, 2)), nil)
			count := 0
			for _, entry := range entries {
				if entry.DiagnosticCode != "V050" {
					continue
				}
				count++
				want := model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: tc.end}}
				if entry.Location == nil || *entry.Location != want {
					t.Fatalf("unexpected V050 range: %+v", entry.Location)
				}
			}
			if count != 1 {
				t.Fatalf("V050 count = %d, want 1", count)
			}
		})
	}
}

func TestV161Contexts(t *testing.T) {
	t.Run("nested hint", func(t *testing.T) {
		v161Run(t, v161Document(t, v161Open(t, "choice", 2, 1),
			v161Line(t, "@answer Italy", "tag", 3, 2, v161Element(t, "tag", "@answer", "answer", 1), v161Element(t, "content", "Italy", "Italy", 9)),
			v161Open(t, "hint", 3, 2), v161Invalid(t, "### Countries", 5, 3), v161Close(t, 5, 3),
			v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2)), nil)
	})
	t.Run("nested answer", func(t *testing.T) {
		v161Run(t, v161Document(t, v161Open(t, "choice", 2, 1), v161Open(t, "answer", 3, 2),
			v161Marked(t, "", "###", " Countries", 4, 3), v161Text(t, "Spain", "Spain", 4, 3), v161Close(t, 4, 3),
			v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2)), nil)
	})
	t.Run("other block", func(t *testing.T) {
		v161Run(t, v161Document(t, v161Open(t, "text", 2, 1), v161Invalid(t, "### Countries", 3, 2),
			v161Text(t, "Read this text.", "Read this text.", 3, 2), v161Close(t, 3, 2)), nil)
	})
	t.Run("root heading after closed choice", func(t *testing.T) {
		v161Run(t, v161Document(t, v161Open(t, "choice", 2, 1),
			v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2),
			v161Line(t, "### Countries", "heading", 0, 0, v161Element(t, "heading-level", "###", "3", 1), v161Element(t, "title", "Countries", "Countries", 5)),
			v161Text(t, "Read.", "Read.", 7, 1)), nil)
	})
	t.Run("choice in branch", func(t *testing.T) {
		// 4.9.4–4.9.8: две именованные ветви с содержимым, choice в первой.
		first := v161Line(t, "@variant A", "tag", 3, 2, v161Element(t, "tag", "@variant", "variant", 1), v161Element(t, "name", "A", "A", 10))
		second := v161Line(t, "@variant B", "tag", 3, 2, v161Element(t, "tag", "@variant", "variant", 1), v161Element(t, "name", "B", "B", 10))
		v161Run(t, v161Document(t, v161Open(t, "variants", 2, 1), first, v161Open(t, "choice", 4, 3),
			v161Invalid(t, "### Countries", 5, 4), v161Text(t, "Spain", "Spain", 5, 4), v161Text(t, "France", "France", 5, 4),
			v161Close(t, 5, 4), second, v161Text(t, "Read.", "Read.", 10, 3), v161Close(t, 3, 2)),
			[]v161Expected{{[3]int{6, 1, 4}, [3]int{5, 1, 10}}})
	})
	t.Run("choice in fragment", func(t *testing.T) {
		fragment := v161Line(t, "@fragment f {", "block-start", 2, 1, v161Element(t, "tag", "@fragment", "fragment", 1),
			v161Element(t, "identifier", "f", "f", 11), v161Element(t, "block-open", "{", "{", 13))
		result := v161Document(t, fragment, v161Open(t, "choice", 3, 2), v161Invalid(t, "### Countries", 4, 3),
			v161Text(t, "Spain", "Spain", 4, 3), v161Text(t, "France", "France", 4, 3), v161Close(t, 4, 3), v161Close(t, 3, 2))
		// Специально проверяем ошибочный контейнер в fragment по условию V161.
		// Диагностика на строке сохраняет отдельную ошибку запрещённого вложения.
		result.Document.HasErrors, result.Lines[3].HasErrors = true, true
		result.Diagnostics = append([]model.Diagnostic{{ID: "forbidden-choice", Source: "parser", DiagnosticCode: "fixture-nesting",
			SeverityLevel: "error", DiagnosticScope: "line", Message: "Запрещённый контейнер внутри fragment.", RelatedLocations: []model.Location{},
			Location: &model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 10}}}}, result.Diagnostics...)
		v161Run(t, result, []v161Expected{{[3]int{5, 1, 4}, [3]int{4, 1, 10}}})
	})
}

func TestV161RangesAndBoundaries(t *testing.T) {
	t.Run("repeated headings at both ends of body", func(t *testing.T) {
		v161Run(t, v161Document(t, v161Open(t, "choice", 2, 1), v161Invalid(t, "### Countries", 3, 2),
			v161Marked(t, "", "###", " Countries", 3, 2), v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2),
			v161Invalid(t, "### Countries", 3, 2), v161Close(t, 3, 2)), []v161Expected{
			{[3]int{4, 1, 4}, [3]int{3, 1, 10}}, {[3]int{5, 1, 4}, [3]int{3, 1, 10}}, {[3]int{8, 1, 4}, [3]int{3, 1, 10}}})
	})
	t.Run("uppercase indented declaration", func(t *testing.T) {
		opening := v161Line(t, "  @CHOICE {", "block-start", 2, 1, v161Element(t, "tag", "@CHOICE", "choice", 3), v161Element(t, "block-open", "{", "{", 11))
		v161Run(t, v161Document(t, opening, v161Invalid(t, "### Countries", 3, 2),
			v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2)),
			[]v161Expected{{[3]int{4, 1, 4}, [3]int{3, 1, 12}}})
	})
	t.Run("Unicode instruction in related declaration", func(t *testing.T) {
		opening := v161Line(t, "@choice Страны 🌍 {", "block-start", 2, 1, v161Element(t, "tag", "@choice", "choice", 1),
			v161Element(t, "content", "Страны 🌍", "Страны 🌍", 9), v161Element(t, "block-open", "{", "{", 18))
		v161Run(t, v161Document(t, opening, v161Invalid(t, "### Countries", 3, 2),
			v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2)),
			[]v161Expected{{[3]int{4, 1, 4}, [3]int{3, 1, 19}}})
	})
	t.Run("empty input", func(t *testing.T) {
		result := v161Document(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		v161Run(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v161Choice(t, "choice", v161Invalid(t, "### Countries", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v161Run(t, result, []v161Expected{{[3]int{4, 1, 4}, [3]int{3, 1, 10}}})
			})
		}
	}
}

// v161Element сохраняет заданные исходное и семантическое значения.
// Длина вычисляется только для входа; ожидания диапазонов заданы отдельно.
func v161Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v161Line задаёт готовую структуру строки, не разбирая исходный DSL.
func v161Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v161Invalid задаёт полностью неразобранную строку с известным родителем.
func v161Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	unparsed := v161Element(t, "unparsed", raw, "", 1)
	unparsed.Value = nil
	return v161Line(t, raw, "invalid", parent, depth, unparsed)
}

// v161Marked задаёт явно выделенный маркер и неразобранный остаток.
func v161Marked(t *testing.T, prefix, marker, tail string, parent, depth int) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(prefix) + 1
	unparsed := v161Element(t, "unparsed", tail, "", start+len(marker))
	unparsed.Value = nil
	return v161Line(t, prefix+marker+tail, "invalid", parent, depth,
		v161Element(t, "heading-level", marker, strconv.Itoa(len(marker)), start), unparsed)
}

// v161Open задаёт блочное объявление без параметров.
func v161Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v161Line(t, "@"+name+" {", "block-start", parent, depth,
		v161Element(t, "tag", "@"+name, name, 1), v161Element(t, "block-open", "{", "{", len(name)+3))
}

// v161Close задаёт закрытие указанного блока.
func v161Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v161Line(t, "}", "block-end", parent, depth, v161Element(t, "block-close", "}", "}", 1))
}

// v161Text сохраняет исходное и семантическое содержимое независимо.
func v161Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v161Line(t, raw, "content", parent, depth, v161Element(t, "content", raw, value, 1))
}

// v161Choice окружает кандидата блоком и двумя полноценными вариантами.
func v161Choice(t *testing.T, name string, candidate model.Line) *model.Result {
	t.Helper()
	return v161Document(t, v161Open(t, name, 2, 1), candidate,
		v161Text(t, "Spain", "Spain", 3, 2), v161Text(t, "France", "France", 3, 2), v161Close(t, 3, 2))
}

// v161Document добавляет обязательные версию и заголовок. Для явно заданных
// unparsed создаёт согласованные исходные ошибки, не являющиеся ожиданиями V161.
func v161Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v161Line(t, "@dsl-version 1.2", "tag", 0, 0, v161Element(t, "tag", "@dsl-version", "dsl-version", 1), v161Element(t, "version", "1.2", "1.2", 14)),
		v161Line(t, "# Lesson", "heading", 0, 0, v161Element(t, "heading-level", "#", "1", 1), v161Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Ошибка исходной конструкции.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}})
		}
	}
	return result
}

// v161Registry использует только штатный каталог регистраций.
func v161Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v161Snapshot получает независимое представление модели для сравнения.
func v161Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v161Run проверяет точные результаты V161 текущего запуска полного конвейера
// и сохранность исходных данных. Возвращает все сообщения текущего запуска.
func v161Run(t *testing.T, result *model.Result, want []v161Expected) []model.Diagnostic {
	t.Helper()
	before := v161Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v161Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v161Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v161-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got, current []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source != source {
			continue
		}
		current = append(current, entry)
		if entry.DiagnosticCode == "V161" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V161 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		main := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]}, End: model.Position{Line: expected.main[0], Column: expected.main[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]}, End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != main || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Внутри блока выбора записан заголовок." {
			t.Fatalf("occurrence %d = %+v, location %+v; want main %+v, related %+v and exact metadata", i, entry, entry.Location, main, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.main[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line-scoped V161 linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v161Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v161Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
	return current
}

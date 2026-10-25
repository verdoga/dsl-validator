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

// v193Want содержит явно заданный основной диапазон и связанные строки.
type v193Want struct {
	main    [3]int
	related [][3]int
}

func TestV193Registration(t *testing.T) {
	registry := v193TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V193" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Блок @variants вложен в другой @variants." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V193 registrations = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV193ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		result         *model.Result
		want           []v193Want
	}{
		{"reference", "@variants {\n@variant A\nRead.\n@variant B\nWrite.\n}\n@variants {\n@variant C\nListen.\n@variant D\nSpeak.\n}",
			v193TestDocument(t, append([]model.Line{v193TestOpen(t, "variants", 2, 1), v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 6, 3), v193TestClose(t, 3, 2)}, v193TestInner(t, 9, 2, 1)...)...), nil},
		{"example", "@variants {\n@variant A\nRead.\n@variants {\n@variant C\nListen.\n@variant D\nSpeak.\n}\n@variant B\nWrite.\n}", v193TestExample(t), []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v193TestRun(t, tc.result, tc.want)
			v193TestRun(t, tc.result, tc.want)
		})
	}
}

func TestV193PositionAndContents(t *testing.T) {
	t.Run("before first branch", func(t *testing.T) {
		body := append(v193TestInner(t, 4, 3, 2), v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 10, 3), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 12, 3))
		v193TestRun(t, v193TestBlock(t, "variants", body...), []v193Want{{[3]int{4, 1, 10}, [][3]int{{3, 1, 10}}}})
	})
	for _, count := range []int{0, 1, 2} {
		t.Run("inner branch count="+strconv.Itoa(count), func(t *testing.T) {
			body := []model.Line{v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3), v193TestOpen(t, "variants", 4, 3)}
			if count > 0 {
				body = append(body, v193TestNamed(t, "variant", "name", "C", 6, 4), v193TestText(t, "Listen.", "Listen.", 7, 5))
			}
			if count > 1 {
				body = append(body, v193TestNamed(t, "variant", "name", "D", 6, 4), v193TestText(t, "Speak.", "Speak.", 9, 5))
			}
			body = append(body, v193TestClose(t, 6, 4), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 8+count*2, 3))
			v193TestRun(t, v193TestBlock(t, "variants", body...), []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}})
		})
	}
	for _, step := range []bool{false, true} {
		t.Run("after task/step="+strconv.FormatBool(step), func(t *testing.T) {
			body := []model.Line{v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestNamed(t, "task", "identifier", "a", 4, 3)}
			inner := 6
			want := []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}}
			if step {
				body = append(body, v193TestNamed(t, "step", "title", "First", 5, 4))
				inner = 7
				want = []v193Want{{[3]int{7, 1, 10}, [][3]int{{3, 1, 10}}}}
			}
			// По 2.3.7 и 2.5.13 variants завершает task/step; фигурный внешний
			// variants сохраняется, а родителем объявления остаётся ветвь A.
			body = append(body, v193TestInner(t, inner, 4, 3)...)
			body = append(body, v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", inner+6, 3))
			v193TestRun(t, v193TestBlock(t, "variants", body...), want)
		})
	}
	t.Run("two separate nested declarations", func(t *testing.T) {
		body := []model.Line{v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3)}
		body = append(body, v193TestInner(t, 6, 4, 3)...)
		body = append(body, v193TestInner(t, 12, 4, 3)...)
		body = append(body, v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 18, 3))
		v193TestRun(t, v193TestBlock(t, "variants", body...), []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}, {[3]int{12, 1, 10}, [][3]int{{3, 1, 10}}}})
	})
	t.Run("three levels refer to nearest container", func(t *testing.T) {
		body := []model.Line{v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3), v193TestOpen(t, "variants", 4, 3), v193TestNamed(t, "variant", "name", "C", 6, 4), v193TestText(t, "Listen.", "Listen.", 7, 5)}
		body = append(body, v193TestInner(t, 9, 7, 5)...)
		body = append(body, v193TestNamed(t, "variant", "name", "D", 6, 4), v193TestText(t, "Speak.", "Speak.", 15, 5), v193TestClose(t, 6, 4), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 18, 3))
		v193TestRun(t, v193TestBlock(t, "variants", body...), []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}, {[3]int{9, 1, 10}, [][3]int{{6, 1, 10}}}})
	})
}

func TestV193InterveningContainers(t *testing.T) {
	for _, container := range []string{"note", "text", "example", "wordlist", "table", "script", "key", "instr", "alt", "hint", "fragment", "answer", "choice", "multichoice", "matching", "ordering"} {
		t.Run(container, func(t *testing.T) {
			opening := v193TestOpen(t, container, 4, 3)
			if container == "fragment" {
				opening = v193TestLine(t, "@fragment f {", "block-start", 4, 3, v193TestElement(t, "tag", "@fragment", "fragment", 1), v193TestElement(t, "identifier", "f", "f", 11), v193TestElement(t, "block-open", "{", "{", 13))
			}
			body := []model.Line{v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3), opening}
			body = append(body, v193TestInner(t, 7, 6, 4)...)
			body = append(body, v193TestClose(t, 6, 4), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 14, 3))
			v193TestRun(t, v193TestBlock(t, "variants", body...), nil)
		})
	}
	for _, container := range []string{"editor", "multifill"} {
		t.Run("literal/"+container, func(t *testing.T) {
			v193TestRun(t, v193TestBlock(t, "variants", v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3), v193TestOpen(t, container, 4, 3),
				v193TestText(t, "@variants {", "@variants {", 6, 4), v193TestText(t, "@variant C", "@variant C", 6, 4), v193TestText(t, "Listen.", "Listen.", 6, 4), v193TestText(t, "@variant D", "@variant D", 6, 4), v193TestText(t, "Speak.", "Speak.", 6, 4), v193TestClose(t, 6, 4), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 13, 3)), nil)
		})
	}
}

func TestV193SourceRangesAndMissingTags(t *testing.T) {
	for _, missing := range []string{"none", "inner", "outer", "both"} {
		for _, indented := range []bool{false, true} {
			t.Run(missing+"/indented="+strconv.FormatBool(indented), func(t *testing.T) {
				result := v193TestExample(t)
				want := []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}}
				if indented {
					result.Lines[2] = v193TestLine(t, "\t@VaRiAnTs {", "block-start", 2, 1, v193TestElement(t, "tag", "@VaRiAnTs", "variants", 2), v193TestElement(t, "block-open", "{", "{", 12))
					result.Lines[2].Number = 3
					result.Lines[5] = v193TestLine(t, " \t@VARIANTS {", "block-start", 4, 3, v193TestElement(t, "tag", "@VARIANTS", "variants", 3), v193TestElement(t, "block-open", "{", "{", 13))
					result.Lines[5].Number = 6
					want = []v193Want{{[3]int{6, 3, 12}, [][3]int{{3, 2, 11}}}}
				}
				// block-open сохраняется: без tag модель остаётся полноценным block-start.
				if missing == "outer" || missing == "both" {
					result.Lines[2].Elements = result.Lines[2].Elements[1:]
				}
				if missing == "inner" || missing == "both" {
					result.Lines[5].Elements = result.Lines[5].Elements[1:]
				}
				v193TestRun(t, result, want)
			})
		}
	}
}

func TestV193BoundariesAndLifecycle(t *testing.T) {
	t.Run("escaped pseudo declaration", func(t *testing.T) {
		// Для обычного текста экранированы и начальный @, и литеральная скобка (1.21).
		v193TestRun(t, v193TestBlock(t, "variants", v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, `\@variants \{`, "@variants {", 4, 3), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 6, 3)), nil)
	})
	t.Run("inline variants is not a block", func(t *testing.T) {
		v193TestRun(t, v193TestBlock(t, "variants", v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3), v193TestLine(t, "@variants", "tag", 4, 3, v193TestElement(t, "tag", "@variants", "variants", 1)), v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 7, 3)), nil)
	})
	t.Run("one outer block", func(t *testing.T) { v193TestRun(t, v193TestDocument(t, v193TestInner(t, 3, 2, 1)...), nil) })
	t.Run("empty document", func(t *testing.T) {
		result := v193TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v193TestRun(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v193TestExample(t)
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v193TestRun(t, result, []v193Want{{[3]int{6, 1, 10}, [][3]int{{3, 1, 10}}}})
			})
		}
	}
}

// v193TestNamed задаёт логическое объявление с параметром указанного типа.
func v193TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v193TestLine(t, "@"+tag+" "+value, "tag", parent, depth, v193TestElement(t, "tag", "@"+tag, tag, 1), v193TestElement(t, kind, value, value, len(tag)+3))
}

// v193TestInner задаёт полноценный блок с ветвями C/D на известных строках.
func v193TestInner(t *testing.T, number, parent, depth int) []model.Line {
	t.Helper()
	return []model.Line{v193TestOpen(t, "variants", parent, depth), v193TestNamed(t, "variant", "name", "C", number, depth+1), v193TestText(t, "Listen.", "Listen.", number+1, depth+2), v193TestNamed(t, "variant", "name", "D", number, depth+1), v193TestText(t, "Speak.", "Speak.", number+3, depth+2), v193TestClose(t, number, depth+1)}
}

// v193TestExample воспроизводит пример таблицы без изменения содержимого.
func v193TestExample(t *testing.T) *model.Result {
	t.Helper()
	body := []model.Line{v193TestNamed(t, "variant", "name", "A", 3, 2), v193TestText(t, "Read.", "Read.", 4, 3)}
	body = append(body, v193TestInner(t, 6, 4, 3)...)
	body = append(body, v193TestNamed(t, "variant", "name", "B", 3, 2), v193TestText(t, "Write.", "Write.", 12, 3))
	return v193TestBlock(t, "variants", body...)
}

// v193TestRun проверяет полный конвейер, исходные диапазоны и историю модели.
func v193TestRun(t *testing.T, result *model.Result, want []v193Want) {
	t.Helper()
	before := v193TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v193TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v193TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v193-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V193" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V193 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]}, End: model.Position{Line: expected.main[0], Column: expected.main[2]}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Блок @variants вложен в другой @variants." || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V193 occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, expected)
		}
		for j, related := range expected.related {
			location := model.Location{Start: model.Position{Line: related[0], Column: related[1]}, End: model.Position{Line: related[0], Column: related[2]}}
			if entry.RelatedLocations[j] != location {
				t.Fatalf("related %d = %+v, want %+v", j, entry.RelatedLocations[j], location)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line diagnostic linked to element")
					}
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.main[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v193TestSnapshot(t, result)), &restored); err != nil {
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
	if v193TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v193TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v193TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v193TestLine задаёт структуру строки без повторного разбора DSL.
func v193TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v193TestOpen задаёт блочное объявление без параметров с явным родителем.
func v193TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v193TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v193TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v193TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v193TestClose задаёт закрытие указанного блока.
func v193TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v193TestLine(t, "}", "block-end", parent, depth, v193TestElement(t, "block-close", "}", "}", 1))
}

// v193TestText сохраняет исходное и семантическое содержимое независимо.
func v193TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v193TestLine(t, raw, "content", parent, depth, v193TestElement(t, "content", raw, value, 1))
}

// v193TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v193TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v193TestOpen(t, name, 2, 1)}, body...)
	return v193TestDocument(t, append(lines, v193TestClose(t, 3, 2))...)
}

// v193TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v193TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v193TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v193TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v193TestElement(t, "version", "1.2", "1.2", 14)),
		v193TestLine(t, "# Lesson", "heading", 0, 0, v193TestElement(t, "heading-level", "#", "1", 1), v193TestElement(t, "title", "Lesson", "Lesson", 3)),
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

// v193TestRegistry использует только полный штатный каталог.
func v193TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v193TestSnapshot сохраняет независимое представление модели для сравнения.
func v193TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

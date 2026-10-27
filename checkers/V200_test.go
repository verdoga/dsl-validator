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

const (
	v200TestMessage = "Описание изображения расположено между инструкцией и образцом выполнения."
	v200TestDetail  = "Разместите эту группу описаний после образца выполнения, сохранив порядок описаний."
)

// v200TestWant задаёт независимые диапазоны [строка, колонка, строка, колонка).
type v200TestWant struct {
	main, instruction, example [4]int
}

func v200TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value,
		Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v200TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v200TestTag(t *testing.T, name, value string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v200TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if kind != "" {
		elements = append(elements, v200TestElement(t, kind, value, value, utf8.RuneCountInString(raw)+2))
		raw += " " + value
	}
	return v200TestLine(t, raw, "tag", parent, elements...)
}

func v200TestOpen(t *testing.T, name string, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	return v200TestLine(t, raw+" {", "block-start", parent,
		v200TestElement(t, "tag", raw, strings.ToLower(name), 1),
		v200TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(raw)+2))
}

func v200TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v200TestLine(t, "}", "block-end", parent, v200TestElement(t, "block-close", "}", "}", 1))
}

func v200TestText(t *testing.T, text string, parent int) model.Line {
	t.Helper()
	return v200TestLine(t, text, "content", parent, v200TestElement(t, "content", text, text, 1))
}

// v200TestDocument добавляет обязательный контекст по DSL 1.30.
// Типы и родители заданы явно; помощник не разбирает исходный DSL.
func v200TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v200TestTag(t, "dsl-version", "1.2", "version", 0),
		v200TestLine(t, "# Lesson", "heading", 0, v200TestElement(t, "heading-level", "#", "1", 1), v200TestElement(t, "title", "Lesson", "Lesson", 3)),
		v200TestLine(t, "### Practice", "heading", 0, v200TestElement(t, "heading-level", "###", "3", 1), v200TestElement(t, "title", "Practice", "Practice", 5)),
	}
	lines = append(lines, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: "1.0", Lines: lines,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for index := range result.Lines {
		line := &result.Lines[index]
		line.Number = index + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
	}
	result.Lines[len(lines)-1].LineEnding = ""
	return result
}

func v200TestBasic(t *testing.T) *model.Result {
	t.Helper()
	return v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3),
		v200TestTag(t, "instr", "Read.", "content", 4),
		v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "example", "X.", "content", 4))
}

func v200TestJSON(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v200TestLocation(expected [4]int) model.Location {
	return model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
		End: model.Position{Line: expected[2], Column: expected[3]}}
}

// v200TestRun проверяет новые V200 через полный штатный каталог и конвейер.
func v200TestRun(t *testing.T, result *model.Result, source string, want []v200TestWant) {
	t.Helper()
	before := v200TestJSON(t, result)
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v200TestJSON(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldCount := len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V200" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V200 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		if entry.Location == nil || *entry.Location != v200TestLocation(expected.main) ||
			entry.Message != v200TestMessage+"\nконкретизация: "+v200TestDetail ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "block" ||
			entry.Fatal || entry.ID == "" || entry.Source != source || len(entry.RelatedLocations) != 2 {
			t.Fatalf("V200[%d] = %+v, want %+v with exact metadata and detail", index, entry, expected)
		}
		if entry.RelatedLocations[0] != v200TestLocation(expected.instruction) ||
			entry.RelatedLocations[1] != v200TestLocation(expected.example) {
			t.Fatalf("related = %+v, want instruction %v, example %v", entry.RelatedLocations, expected.instruction, expected.example)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("block-scoped V200 created an element link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v200TestJSON(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for index := range restored.Lines {
		line, old := &restored.Lines[index], original.Lines[index]
		line.HasErrors = old.HasErrors
		if len(line.Elements) != len(old.Elements) {
			t.Fatal("element count changed")
		}
		for index := range line.Elements {
			prior, current := old.Elements[index].ErrorIDs, line.Elements[index].ErrorIDs
			if len(current) < len(prior) || !slices.Equal(prior, current[:len(prior)]) {
				t.Fatal("prior element links changed")
			}
			line.Elements[index].ErrorIDs = prior
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v200TestJSON(t, &restored) != before {
		t.Fatal("source data, metadata or prior history changed")
	}
}

func TestV200RegistrationAndVersions(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V200" {
			continue
		}
		count++
		if definition.Message != v200TestMessage || definition.Severity != "warning" ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V200 registered %d times, want one", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			if checks, err := registry.ForVersion(dslversions.Version(version)); err == nil || checks != nil {
				t.Fatal("unsupported version accepted")
			}
			result := v200TestBasic(t)
			result.Document.DSLVersion = &version
			before := v200TestJSON(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatal("Prepare accepted unsupported version")
			}
			if v200TestJSON(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV200ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		before         bool
		want           []v200TestWant
	}{
		{"reference", "@task a\n@instr Correct the mistakes.\n@example {\nIt was suny.\n---\nIt was sunny.\n}\n@alt На фотографии солнечный пляж.",
			false, nil},
		{"example", "@task a\n@instr Correct the mistakes.\n@alt На фотографии солнечный пляж.\n@example {\nIt was suny.\n---\nIt was sunny.\n}",
			true, []v200TestWant{{[4]int{6, 1, 6, 35}, [4]int{5, 1, 5, 29}, [4]int{7, 1, 11, 2}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v200TestTag(t, "task", "a", "identifier", 3),
				v200TestTag(t, "instr", "Correct the mistakes.", "content", 4)}
			if tc.before {
				body = append(body, v200TestTag(t, "alt", "На фотографии солнечный пляж.", "content", 4))
			}
			opening := len(body) + 4
			body = append(body, v200TestOpen(t, "example", 4), v200TestText(t, "It was suny.", opening),
				v200TestLine(t, "---", "separator", opening), v200TestText(t, "It was sunny.", opening), v200TestClose(t, opening))
			if !tc.before {
				body = append(body, v200TestTag(t, "alt", "На фотографии солнечный пляж.", "content", 4))
			}
			var raw []string
			for _, line := range body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v200TestRun(t, v200TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV200InlineAndBlockForms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		blocks [3]bool
		want   v200TestWant
	}{
		{"inline", [3]bool{}, v200TestWant{[4]int{6, 1, 6, 8}, [4]int{5, 1, 5, 13}, [4]int{7, 1, 7, 12}}},
		{"block example", [3]bool{false, false, true}, v200TestWant{[4]int{6, 1, 6, 8}, [4]int{5, 1, 5, 13}, [4]int{7, 1, 9, 2}}},
		{"block alt", [3]bool{false, true, false}, v200TestWant{[4]int{6, 1, 8, 2}, [4]int{5, 1, 5, 13}, [4]int{9, 1, 9, 12}}},
		{"block alt and example", [3]bool{false, true, true}, v200TestWant{[4]int{6, 1, 8, 2}, [4]int{5, 1, 5, 13}, [4]int{9, 1, 11, 2}}},
		{"block instruction", [3]bool{true, false, false}, v200TestWant{[4]int{8, 1, 8, 8}, [4]int{5, 1, 7, 2}, [4]int{9, 1, 9, 12}}},
		{"block instruction and example", [3]bool{true, false, true}, v200TestWant{[4]int{8, 1, 8, 8}, [4]int{5, 1, 7, 2}, [4]int{9, 1, 11, 2}}},
		{"block instruction and alt", [3]bool{true, true, false}, v200TestWant{[4]int{8, 1, 10, 2}, [4]int{5, 1, 7, 2}, [4]int{11, 1, 11, 12}}},
		{"all blocks", [3]bool{true, true, true}, v200TestWant{[4]int{8, 1, 10, 2}, [4]int{5, 1, 7, 2}, [4]int{11, 1, 13, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v200TestTag(t, "task", "a", "identifier", 3)}
			for index, name := range []string{"instr", "alt", "example"} {
				text := []string{"Read.", "A.", "X."}[index]
				if tc.blocks[index] {
					opening := len(body) + 4
					body = append(body, v200TestOpen(t, name, 4), v200TestText(t, text, opening), v200TestClose(t, opening))
				} else {
					body = append(body, v200TestTag(t, name, text, "content", 4))
				}
			}
			v200TestRun(t, v200TestDocument(t, body...), "current", []v200TestWant{tc.want})
		})
	}
}

func TestV200GroupsAndNestedInstruction(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v200TestWant
	}{
		{"maximal group", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "alt", "B.", "content", 4), v200TestTag(t, "example", "X.", "content", 4)},
			[]v200TestWant{{[4]int{6, 1, 7, 8}, [4]int{5, 1, 5, 13}, [4]int{8, 1, 8, 12}}}},
		{"two independent groups", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "example", "X.", "content", 4),
			v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "B.", "content", 4), v200TestTag(t, "example", "Y.", "content", 4)},
			[]v200TestWant{{[4]int{6, 1, 6, 8}, [4]int{5, 1, 5, 13}, [4]int{7, 1, 7, 12}},
				{[4]int{9, 1, 9, 8}, [4]int{8, 1, 8, 13}, [4]int{10, 1, 10, 12}}}},
		{"nearest instruction", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "instr", "Write.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "example", "X.", "content", 4)},
			[]v200TestWant{{[4]int{7, 1, 7, 8}, [4]int{6, 1, 6, 14}, [4]int{8, 1, 8, 12}}}},
		{"new instruction breaks earlier group", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4),
			v200TestTag(t, "instr", "Write.", "content", 4), v200TestTag(t, "alt", "B.", "content", 4), v200TestTag(t, "example", "X.", "content", 4)},
			[]v200TestWant{{[4]int{8, 1, 8, 8}, [4]int{7, 1, 7, 14}, [4]int{9, 1, 9, 12}}}},
		{"instruction includes nested hint", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestOpen(t, "instr", 4),
			v200TestText(t, "Read.", 5), v200TestOpen(t, "hint", 5), v200TestText(t, "Think.", 7), v200TestClose(t, 7), v200TestClose(t, 5),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestOpen(t, "alt", 4), v200TestText(t, "B.", 12), v200TestClose(t, 12),
			v200TestOpen(t, "example", 4), v200TestText(t, "X.", 15), v200TestClose(t, 15)},
			[]v200TestWant{{[4]int{11, 1, 14, 2}, [4]int{5, 1, 10, 2}, [4]int{15, 1, 17, 2}}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v200TestRun(t, v200TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV200EditorsAndBlanks(t *testing.T) {
	t.Run("skipped content inside group is covered by its range", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "editor", "Note.", "content", 4), v200TestLine(t, "", "blank", 4),
			v200TestTag(t, "alt", "B.", "content", 4), v200TestTag(t, "example", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 1, 9, 8}, [4]int{5, 1, 5, 13}, [4]int{10, 1, 10, 12}}})
	})
	t.Run("editors before and after group", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "editor", "Note.", "content", 4), v200TestLine(t, " \t", "blank", 4),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestOpen(t, "editor", 4), v200TestText(t, "Comment.", 9),
			v200TestClose(t, 9), v200TestTag(t, "example", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{8, 1, 8, 8}, [4]int{5, 1, 5, 13}, [4]int{12, 1, 12, 12}}})
	})
	t.Run("literal declarations in editor do not change task or stage", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestOpen(t, "editor", 4),
			v200TestText(t, "@task b", 7), v200TestText(t, "@step Next", 7), v200TestText(t, "@endtask", 7),
			v200TestText(t, "@instr Read.", 7), v200TestText(t, "@alt A.", 7), v200TestText(t, "@example X.", 7),
			v200TestText(t, "### Heading", 7), v200TestClose(t, 7), v200TestTag(t, "example", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 1, 6, 8}, [4]int{5, 1, 5, 13}, [4]int{16, 1, 16, 12}}})
	})
}

func TestV200MissingOrCorrectlyOrderedConstructions(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags []string
	}{
		{"correct order", []string{"instr", "example", "alt"}},
		{"no instruction", []string{"alt", "example"}},
		{"no description", []string{"instr", "example"}},
		{"no example at EOF", []string{"instr", "alt"}},
		{"instruction only", []string{"instr"}},
		{"description only", []string{"alt"}},
		{"example only", []string{"example"}},
		{"description before instruction", []string{"alt", "instr", "example"}},
		{"empty task", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v200TestTag(t, "task", "a", "identifier", 3)}
			for _, name := range tc.tags {
				body = append(body, v200TestTag(t, name, "Text.", "content", 4))
			}
			v200TestRun(t, v200TestDocument(t, body...), "current", nil)
		})
	}
	t.Run("outside active task", func(t *testing.T) {
		v200TestRun(t, v200TestDocument(t, v200TestTag(t, "instr", "Read.", "content", 3),
			v200TestTag(t, "alt", "A.", "content", 3), v200TestTag(t, "example", "X.", "content", 3)), "current", nil)
	})
}

func TestV200OtherSignificantConstructions(t *testing.T) {
	for _, placement := range []string{"before group", "inside group", "after group"} {
		for _, kind := range []string{"text", "tag", "block", "literal editor"} {
			t.Run(placement+"/"+kind, func(t *testing.T) {
				body := []model.Line{v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4)}
				if placement != "before group" {
					body = append(body, v200TestTag(t, "alt", "A.", "content", 4))
				}
				switch kind {
				case "text":
					body = append(body, v200TestText(t, "Explanation.", 4))
				case "tag":
					body = append(body, v200TestTag(t, "note", "Explanation.", "content", 4))
				case "block":
					opening := len(body) + 4
					body = append(body, v200TestOpen(t, "note", 4), v200TestText(t, "Explanation.", opening), v200TestClose(t, opening))
				case "literal editor":
					body = append(body, v200TestLine(t, "\\@editor Note.", "content", 4,
						v200TestElement(t, "content", "\\@editor Note.", "@editor Note.", 1)))
				}
				if placement != "after group" {
					body = append(body, v200TestTag(t, "alt", "B.", "content", 4))
				}
				body = append(body, v200TestTag(t, "example", "X.", "content", 4))
				v200TestRun(t, v200TestDocument(t, body...), "current", nil)
			})
		}
	}
}

func TestV200TaskBoundaries(t *testing.T) {
	for _, beforeAlt := range []bool{true, false} {
		for _, boundary := range []string{"task", "endtask", "header", "newpage", "heading"} {
			t.Run(boundary+"/before-alt="+strconv.FormatBool(beforeAlt), func(t *testing.T) {
				body := []model.Line{v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4)}
				if !beforeAlt {
					body = append(body, v200TestTag(t, "alt", "A.", "content", 4))
				}
				parent := 3
				switch boundary {
				case "task":
					parent = len(body) + 4
					body = append(body, v200TestTag(t, "task", "b", "identifier", 3))
				case "endtask":
					body = append(body, v200TestTag(t, "endtask", "", "", 4))
				case "header":
					body = append(body, v200TestTag(t, "header", "Theory", "title", 3))
				case "newpage":
					// По DSL 2.7.7 новый шаг сразу содержит отображаемый материал.
					parent = len(body) + 4
					body = append(body, v200TestTag(t, "newpage", "", "", 0))
				case "heading":
					parent = len(body) + 4
					body = append(body, v200TestLine(t, "### Next", "heading", 0,
						v200TestElement(t, "heading-level", "###", "3", 1), v200TestElement(t, "title", "Next", "Next", 5)))
				}
				if beforeAlt {
					body = append(body, v200TestTag(t, "alt", "A.", "content", parent))
				}
				body = append(body, v200TestTag(t, "example", "X.", "content", parent))
				v200TestRun(t, v200TestDocument(t, body...), "current", nil)
			})
		}
	}
	t.Run("next task starts independent pattern", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3),
			v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4),
			v200TestTag(t, "task", "b", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 7),
			v200TestTag(t, "alt", "B.", "content", 7), v200TestTag(t, "example", "X.", "content", 7))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{9, 1, 9, 8}, [4]int{8, 1, 8, 13}, [4]int{10, 1, 10, 12}}})
	})
}

func TestV200StepBoundaries(t *testing.T) {
	t.Run("pattern inside one step", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3),
			v200TestTag(t, "step", "First", "title", 4), v200TestTag(t, "instr", "Read.", "content", 5),
			v200TestTag(t, "alt", "A.", "content", 5), v200TestTag(t, "example", "X.", "content", 5),
			v200TestTag(t, "step", "Second", "title", 4), v200TestTag(t, "instr", "Write.", "content", 9))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{7, 1, 7, 8}, [4]int{6, 1, 6, 13}, [4]int{8, 1, 8, 12}}})
	})
	for _, beforeAlt := range []bool{true, false} {
		t.Run("step change/before-alt="+strconv.FormatBool(beforeAlt), func(t *testing.T) {
			body := []model.Line{v200TestTag(t, "task", "a", "identifier", 3),
				v200TestTag(t, "step", "First", "title", 4), v200TestTag(t, "instr", "Read.", "content", 5)}
			if !beforeAlt {
				body = append(body, v200TestTag(t, "alt", "A.", "content", 5))
			}
			next := len(body) + 4
			body = append(body, v200TestTag(t, "step", "Second", "title", 4))
			if beforeAlt {
				body = append(body, v200TestTag(t, "alt", "A.", "content", next))
			}
			body = append(body, v200TestTag(t, "example", "X.", "content", next))
			v200TestRun(t, v200TestDocument(t, body...), "current", nil)
		})
	}
	t.Run("first step does not continue task-level pattern", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3),
			v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4),
			v200TestTag(t, "step", "First", "title", 4), v200TestTag(t, "example", "X.", "content", 7),
			v200TestTag(t, "step", "Second", "title", 4), v200TestTag(t, "instr", "Write.", "content", 9))
		v200TestRun(t, result, "current", nil)
	})
	t.Run("editor is transparent within step", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3),
			v200TestTag(t, "step", "First", "title", 4), v200TestTag(t, "instr", "Read.", "content", 5),
			v200TestOpen(t, "editor", 5), v200TestText(t, "Note.", 7), v200TestClose(t, 7),
			v200TestTag(t, "alt", "A.", "content", 5), v200TestTag(t, "example", "X.", "content", 5),
			v200TestTag(t, "step", "Second", "title", 4), v200TestTag(t, "instr", "Write.", "content", 12))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{10, 1, 10, 8}, [4]int{6, 1, 6, 13}, [4]int{11, 1, 11, 12}}})
	})
}

func TestV200Variants(t *testing.T) {
	for _, branchParent := range []bool{true, false} {
		t.Run("independent tasks/branch-parent="+strconv.FormatBool(branchParent), func(t *testing.T) {
			first, second := 5, 10
			if !branchParent {
				// Модель допускает общего родителя variants у объявлений ветви.
				first, second = 4, 4
			}
			result := v200TestDocument(t, v200TestOpen(t, "variants", 3),
				v200TestTag(t, "variant", "First", "name", 4), v200TestTag(t, "task", "a", "identifier", first),
				v200TestTag(t, "instr", "Read.", "content", 6), v200TestTag(t, "alt", "A.", "content", 6), v200TestTag(t, "example", "X.", "content", 6),
				v200TestTag(t, "variant", "Second", "name", 4), v200TestTag(t, "task", "b", "identifier", second),
				v200TestTag(t, "instr", "Write.", "content", 11), v200TestTag(t, "alt", "B.", "content", 11), v200TestTag(t, "example", "Y.", "content", 11),
				v200TestClose(t, 4))
			v200TestRun(t, result, "current", []v200TestWant{
				{[4]int{8, 1, 8, 8}, [4]int{7, 1, 7, 13}, [4]int{9, 1, 9, 12}},
				{[4]int{13, 1, 13, 8}, [4]int{12, 1, 12, 14}, [4]int{14, 1, 14, 12}},
			})
		})
	}
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"branch change", []model.Line{
			v200TestOpen(t, "variants", 3), v200TestTag(t, "variant", "First", "name", 4),
			v200TestTag(t, "task", "a", "identifier", 5), v200TestTag(t, "instr", "Read.", "content", 6), v200TestTag(t, "alt", "A.", "content", 6),
			v200TestTag(t, "variant", "Second", "name", 4), v200TestTag(t, "example", "X.", "content", 9), v200TestClose(t, 4)}},
		{"container end", []model.Line{
			v200TestOpen(t, "variants", 3), v200TestTag(t, "variant", "First", "name", 4), v200TestText(t, "Read.", 5),
			v200TestTag(t, "variant", "Second", "name", 4), v200TestTag(t, "task", "b", "identifier", 7),
			v200TestTag(t, "instr", "Read.", "content", 8), v200TestTag(t, "alt", "A.", "content", 8),
			v200TestClose(t, 4), v200TestTag(t, "example", "X.", "content", 3)}},
		{"variants ends outer task", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4),
			v200TestOpen(t, "variants", 3), v200TestTag(t, "variant", "First", "name", 7), v200TestTag(t, "example", "X.", "content", 8),
			v200TestTag(t, "variant", "Second", "name", 7), v200TestText(t, "Write.", 10), v200TestClose(t, 7)}},
		{"outer task does not resume", []model.Line{
			v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestOpen(t, "variants", 3), v200TestTag(t, "variant", "First", "name", 6), v200TestText(t, "Read.", 7),
			v200TestTag(t, "variant", "Second", "name", 6), v200TestText(t, "Write.", 9), v200TestClose(t, 6),
			v200TestTag(t, "alt", "A.", "content", 3), v200TestTag(t, "example", "X.", "content", 3)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v200TestRun(t, v200TestDocument(t, tc.body...), "current", nil) })
	}
}

func TestV200LiteralAndNestedContent(t *testing.T) {
	for _, name := range []string{"instr", "alt", "example"} {
		t.Run("escaped "+name, func(t *testing.T) {
			result := v200TestBasic(t)
			index := map[string]int{"instr": 4, "alt": 5, "example": 6}[name]
			original := result.Lines[index]
			result.Lines[index].Raw = "\\" + original.Raw
			result.Lines[index].LineType = "content"
			result.Lines[index].Elements = []model.Element{v200TestElement(t, "content", "\\"+original.Raw, original.Raw, 1)}
			v200TestRun(t, result, "current", nil)
		})
	}
	t.Run("entire pattern inside editor is literal", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestOpen(t, "editor", 4),
			v200TestText(t, "@instr Read.", 5), v200TestText(t, "@alt A.", 5), v200TestText(t, "@example X.", 5), v200TestClose(t, 5))
		v200TestRun(t, result, "current", nil)
	})
	t.Run("literal multifill does not change outer task", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestOpen(t, "multifill", 4),
			v200TestText(t, "@task hidden", 5), v200TestText(t, "@endtask", 5), v200TestClose(t, 5),
			v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "example", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{10, 1, 10, 8}, [4]int{9, 1, 9, 13}, [4]int{11, 1, 11, 12}}})
	})
	t.Run("fragment content is not a task sequence", func(t *testing.T) {
		result := v200TestDocument(t,
			v200TestLine(t, "@fragment f {", "block-start", 3, v200TestElement(t, "tag", "@fragment", "fragment", 1),
				v200TestElement(t, "identifier", "f", "f", 11), v200TestElement(t, "block-open", "{", "{", 13)),
			v200TestTag(t, "instr", "Read.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4),
			v200TestTag(t, "example", "X.", "content", 4), v200TestClose(t, 4))
		v200TestRun(t, result, "current", nil)
	})
}

func TestV200OriginalRangesAndNames(t *testing.T) {
	t.Run("tag case", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "TaSk", "a", "identifier", 3),
			v200TestTag(t, "InStR", "Read.", "content", 4), v200TestTag(t, "ALT", "A.", "content", 4), v200TestTag(t, "ExAmPlE", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 1, 6, 8}, [4]int{5, 1, 5, 13}, [4]int{7, 1, 7, 12}}})
	})
	t.Run("Unicode indentation and trailing spaces", func(t *testing.T) {
		result := v200TestBasic(t)
		result.Lines[5].Raw = "\t@alt Ёж🙂.  "
		result.Lines[5].Elements = []model.Element{v200TestElement(t, "tag", "@alt", "alt", 2),
			v200TestElement(t, "content", "Ёж🙂.", "Ёж🙂.", 7)}
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 2, 6, 13}, [4]int{5, 1, 5, 13}, [4]int{7, 1, 7, 12}}})
	})
	t.Run("instruction retains source edges", func(t *testing.T) {
		result := v200TestBasic(t)
		result.Lines[4].Raw = "\t@instr Read. \t"
		result.Lines[4].Elements = []model.Element{v200TestElement(t, "tag", "@instr", "instr", 2),
			v200TestElement(t, "content", "Read.", "Read.", 9)}
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 1, 6, 8}, [4]int{5, 2, 5, 16}, [4]int{7, 1, 7, 12}}})
	})
	t.Run("block title and indented closing brace", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestLine(t, "@alt View {", "block-start", 4, v200TestElement(t, "tag", "@alt", "alt", 1),
				v200TestElement(t, "title", "View", "View", 6), v200TestElement(t, "block-open", "{", "{", 11)),
			v200TestText(t, "A.", 6), v200TestLine(t, "\t}  ", "block-end", 6, v200TestElement(t, "block-close", "}", "}", 2)),
			v200TestTag(t, "example", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 1, 8, 3}, [4]int{5, 1, 5, 13}, [4]int{9, 1, 9, 12}}})
	})
	t.Run("text does not determine the group", func(t *testing.T) {
		result := v200TestDocument(t, v200TestTag(t, "task", "a", "identifier", 3), v200TestTag(t, "instr", "Read.", "content", 4),
			v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "alt", "A.", "content", 4), v200TestTag(t, "alt", "A!", "content", 4),
			v200TestTag(t, "example", "X.", "content", 4))
		v200TestRun(t, result, "current", []v200TestWant{{[4]int{6, 1, 8, 8}, [4]int{5, 1, 5, 13}, [4]int{9, 1, 9, 12}}})
	})
}

func TestV200EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v200TestBasic(t)
				for index := range result.Lines {
					result.Lines[index].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := []v200TestWant{{[4]int{6, 1, 6, 8}, [4]int{5, 1, 5, 13}, [4]int{7, 1, 7, 12}}}
				v200TestRun(t, result, "first", want)
				v200TestRun(t, result, "second", want)
				v200TestRun(t, v200TestDocument(t), "other", nil)
			})
		}
	}
	t.Run("empty input", func(t *testing.T) {
		result := v200TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v200TestRun(t, result, "current", nil)
	})
}

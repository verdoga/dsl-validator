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

// v190Want содержит явно заданный основной диапазон и связанные строки.
type v190Want struct {
	main    [3]int
	related [][3]int
}

func TestV190Registration(t *testing.T) {
	registry := v190TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V190" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Название ветви повторяется в одном @variants." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V190 registrations = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV190ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, second, fragment string
		want                   []v190Want
	}{
		{"reference", "Student B", "@variants {\n@variant Student A\nRead.\n@variant Student B\nWrite.\n}", nil},
		{"example", "student a", "@variants {\n@variant Student A\nRead.\n@variant student a\nWrite.\n}", []v190Want{{[3]int{6, 10, 19}, [][3]int{{4, 10, 19}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v190TestPair(t, "Student A", "Student A", tc.second, tc.second, false)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v190TestRun(t, result, tc.want)
			v190TestRun(t, result, tc.want)
		})
	}
}

func TestV190EveryRepeatRefersToFirst(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		want  []v190Want
	}{
		{"three occurrences", []string{"Student A", "student a", "STUDENT A"}, []v190Want{
			{[3]int{6, 10, 19}, [][3]int{{4, 10, 19}}}, {[3]int{8, 10, 19}, [][3]int{{4, 10, 19}}}}},
		{"nonadjacent", []string{"Student A", "Student B", "student a"}, []v190Want{{[3]int{8, 10, 19}, [][3]int{{4, 10, 19}}}}},
		{"two independent groups", []string{"A", "B", "a", "b"}, []v190Want{{[3]int{8, 10, 11}, [][3]int{{4, 10, 11}}}, {[3]int{10, 10, 11}, [][3]int{{6, 10, 11}}}}},
		{"no branches", nil, nil},
		{"one branch", []string{"A"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body []model.Line
			for i, name := range tc.names {
				body = append(body, v190TestVariant(t, "variant", name, name, 3, 2), v190TestText(t, "Read.", "Read.", 4+2*i, 3))
			}
			v190TestRun(t, v190TestBlock(t, "variants", body...), tc.want)
		})
	}
}

func TestV190NamesAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		name, firstRaw, firstValue, secondRaw, secondValue string
		want                                               []v190Want
	}{
		{"Unicode case", "Ёж🌍", "Ёж🌍", "ёж🌍", "ёж🌍", []v190Want{{[3]int{6, 10, 13}, [][3]int{{4, 10, 13}}}}},
		{"Unicode sigma case", "Σ", "Σ", "ς", "ς", []v190Want{{[3]int{6, 10, 11}, [][3]int{{4, 10, 11}}}}},
		{"inner spaces", "Student A", "Student A", "Student  A", "Student  A", nil},
		{"inner tabs", "Student A", "Student A", "Student\tA", "Student\tA", nil},
		{"same inner tab", "A\tB", "A\tB", "a\tb", "a\tb", []v190Want{{[3]int{6, 10, 13}, [][3]int{{4, 10, 13}}}}},
		{"punctuation", "A", "A", "A!", "A!", nil},
		{"quotes are part of NAME", "A", "A", `"A"`, `"A"`, nil},
		{"numbers are retained", "A", "A", "1. A", "1. A", nil},
		{"punctuation only", "?!", "?!", "?!", "?!", []v190Want{{[3]int{6, 10, 12}, [][3]int{{4, 10, 12}}}}},
		{"empty", "", "", "", "", nil},
		{"ASCII whitespace only", " \t", "", "\t ", "", nil},
		{"NBSP edge", "Student A", "Student A", "Student A\u00a0", "Student A\u00a0", nil},
		{"NBSP retained", "\u00a0", "\u00a0", "\u00a0", "\u00a0", []v190Want{{[3]int{6, 10, 11}, [][3]int{{4, 10, 11}}}}},
		{"no canonical Unicode normalization", "é", "é", "e\u0301", "e\u0301", nil},
		{"opening brace", `A\{B`, "A{B", `a\{b`, "a{b", []v190Want{{[3]int{6, 10, 14}, [][3]int{{4, 10, 14}}}}},
		{"closing brace", `A\}B`, "A}B", `a\}b`, "a}b", []v190Want{{[3]int{6, 10, 14}, [][3]int{{4, 10, 14}}}}},
		{"backslash not decoded twice", `A\\B`, `A\B`, "ab", "ab", nil},
		{"paired backslashes", `A\\B`, `A\B`, `a\\b`, `a\b`, []v190Want{{[3]int{6, 10, 14}, [][3]int{{4, 10, 14}}}}},
		{"pair then escaped brace", `A\\\{B`, `A\{B`, `a\\\{b`, `a\{b`, []v190Want{{[3]int{6, 10, 16}, [][3]int{{4, 10, 16}}}}},
		{"unknown escape retained", `A\q`, `A\q`, "aq", "aq", nil},
		{"at sign escape not active after tag", `\@A`, `\@A`, "@a", "@a", nil},
		{"hash escape not active after tag", `\#A`, `\#A`, "#a", "#a", nil},
		{"quote escape not active in NAME", `A\"B`, `A\"B`, `a"b`, `a"b`, nil},
		{"underscores are not placeholders", `\_____`, `\_____`, "_____", "_____", nil},
		{"separator escape not active in NAME", `\---`, `\---`, "---", "---", nil},
		{"whole name including at sign", "A @variant B", "A @variant B", "a @variant b", "a @variant b", []v190Want{{[3]int{6, 10, 22}, [][3]int{{4, 10, 22}}}}},
	} {
		for _, rawOnly := range []bool{false, true} {
			t.Run(tc.name+"/rawOnly="+strconv.FormatBool(rawOnly), func(t *testing.T) {
				v190TestRun(t, v190TestPair(t, tc.firstRaw, tc.firstValue, tc.secondRaw, tc.secondValue, rawOnly), tc.want)
			})
		}
	}
}

func TestV190RangesAndAmbiguousNames(t *testing.T) {
	for _, rawOnly := range []bool{false, true} {
		t.Run("ASCII edges/rawOnly="+strconv.FormatBool(rawOnly), func(t *testing.T) {
			second := v190TestLine(t, " \t@variant   student a \t", "tag", 3, 2, v190TestElement(t, "tag", "@variant", "variant", 3), v190TestElement(t, "name", "student a", "student a", 14))
			if rawOnly {
				second.Elements = second.Elements[:1]
			}
			v190TestRun(t, v190TestBlock(t, "variants", v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, "Read.", "Read.", 4, 3), second, v190TestText(t, "Write.", "Write.", 6, 3)), []v190Want{{[3]int{6, 14, 23}, [][3]int{{4, 10, 19}}}})
		})
	}
	t.Run("name element includes outer spaces", func(t *testing.T) {
		v190TestRun(t, v190TestPair(t, "  A\t", "A", "\ta ", "a", false), []v190Want{{[3]int{6, 11, 12}, [][3]int{{4, 12, 13}}}})
	})
	t.Run("partial name excluded and first known occurrence retained", func(t *testing.T) {
		broken := v190TestLine(t, "@variant Student A {", "invalid", 3, 2, v190TestElement(t, "tag", "@variant", "variant", 1), v190TestElement(t, "name", "Student A", "Student A", 10), model.Element{ElementType: "unparsed", Raw: "{", Start: 20, End: 21, ErrorIDs: []string{}})
		v190TestRun(t, v190TestBlock(t, "variants", broken, v190TestText(t, "Read.", "Read.", 4, 3), v190TestVariant(t, "variant", "student a", "student a", 3, 2), v190TestText(t, "Write.", "Write.", 6, 3), v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, "Listen.", "Listen.", 8, 3)), []v190Want{{[3]int{8, 10, 19}, [][3]int{{6, 10, 19}}}})
	})
	t.Run("fully unparsed name excluded", func(t *testing.T) {
		v190TestRun(t, v190TestBlock(t, "variants", v190TestUnparsed(t, "@variant Student A {", "invalid", 3, 2), v190TestText(t, "Read.", "Read.", 4, 3), v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, "Write.", "Write.", 6, 3)), nil)
	})
}

func TestV190Containers(t *testing.T) {
	for _, duplicates := range []bool{false, true} {
		t.Run("separate blocks/duplicates="+strconv.FormatBool(duplicates), func(t *testing.T) {
			second := "Student B"
			var want []v190Want
			if duplicates {
				second = "student a"
				want = []v190Want{{[3]int{6, 10, 19}, [][3]int{{4, 10, 19}}}, {[3]int{12, 10, 19}, [][3]int{{10, 10, 19}}}}
			}
			v190TestRun(t, v190TestDocument(t,
				v190TestOpen(t, "variants", 2, 1), v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, "Read.", "Read.", 4, 3), v190TestVariant(t, "variant", second, second, 3, 2), v190TestText(t, "Write.", "Write.", 6, 3), v190TestClose(t, 3, 2),
				v190TestOpen(t, "variants", 2, 1), v190TestVariant(t, "variant", "Student A", "Student A", 9, 2), v190TestText(t, "Listen.", "Listen.", 10, 3), v190TestVariant(t, "variant", second, second, 9, 2), v190TestText(t, "Speak.", "Speak.", 12, 3), v190TestClose(t, 9, 2)), want)
		})
	}
	t.Run("literal editor contents excluded", func(t *testing.T) {
		v190TestRun(t, v190TestBlock(t, "variants",
			v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, "Read.", "Read.", 4, 3), v190TestOpen(t, "editor", 4, 3), v190TestText(t, "@variant student a", "@variant student a", 6, 4), v190TestClose(t, 6, 4),
			v190TestVariant(t, "variant", "Student B", "Student B", 3, 2), v190TestText(t, "Write.", "Write.", 9, 3)), nil)
	})
	t.Run("another nearest block excludes nested declaration", func(t *testing.T) {
		v190TestRun(t, v190TestBlock(t, "variants",
			v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, "Read.", "Read.", 4, 3), v190TestOpen(t, "note", 4, 3), v190TestVariant(t, "variant", "student a", "student a", 6, 4), v190TestText(t, "Nested.", "Nested.", 7, 5), v190TestClose(t, 6, 4),
			v190TestVariant(t, "variant", "Student B", "Student B", 3, 2), v190TestText(t, "Write.", "Write.", 10, 3)), nil)
	})
	t.Run("logical task ends at next variant", func(t *testing.T) {
		// По 2.3.7 следующий variant завершает task: его родитель снова variants.
		// Строки задания сохраняют собственных логических родителей.
		task := v190TestLine(t, "@task a", "tag", 4, 3, v190TestElement(t, "tag", "@task", "task", 1), v190TestElement(t, "identifier", "a", "a", 7))
		v190TestRun(t, v190TestBlock(t, "variants", v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), task, v190TestText(t, "Read.", "Read.", 5, 4), v190TestVariant(t, "variant", "student a", "student a", 3, 2), v190TestText(t, "Write.", "Write.", 7, 3)), []v190Want{{[3]int{7, 10, 19}, [][3]int{{4, 10, 19}}}})
	})
	t.Run("escaped declaration remains ordinary text", func(t *testing.T) {
		v190TestRun(t, v190TestBlock(t, "variants", v190TestVariant(t, "variant", "Student A", "Student A", 3, 2), v190TestText(t, `\@variant student a`, "@variant student a", 4, 3), v190TestVariant(t, "variant", "Student B", "Student B", 3, 2), v190TestText(t, "Write.", "Write.", 6, 3)), nil)
	})
	t.Run("outside variants", func(t *testing.T) {
		v190TestRun(t, v190TestDocument(t, v190TestVariant(t, "variant", "A", "A", 2, 1), v190TestText(t, "Read.", "Read.", 3, 2), v190TestVariant(t, "variant", "a", "a", 2, 1), v190TestText(t, "Write.", "Write.", 5, 2)), nil)
	})
	t.Run("tag case", func(t *testing.T) {
		v190TestRun(t, v190TestBlock(t, "VARIANTS", v190TestVariant(t, "VaRiAnT", "Student A", "Student A", 3, 2), v190TestText(t, "Read.", "Read.", 4, 3), v190TestVariant(t, "VARIANT", "student a", "student a", 3, 2), v190TestText(t, "Write.", "Write.", 6, 3)), []v190Want{{[3]int{6, 10, 19}, [][3]int{{4, 10, 19}}}})
	})
	t.Run("container source fallback", func(t *testing.T) {
		result := v190TestPair(t, "Student A", "Student A", "student a", "student a", true)
		// block-open остаётся обязательным элементом типа block-start.
		result.Lines[2].Elements = result.Lines[2].Elements[1:]
		v190TestRun(t, result, []v190Want{{[3]int{6, 10, 19}, [][3]int{{4, 10, 19}}}})
	})
}

func TestV190Lifecycle(t *testing.T) {
	t.Run("empty document", func(t *testing.T) {
		result := v190TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v190TestRun(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v190TestPair(t, "Student A", "Student A", "student a", "student a", false)
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v190TestRun(t, result, []v190Want{{[3]int{6, 10, 19}, [][3]int{{4, 10, 19}}}})
			})
		}
	}
}

// v190TestVariant задаёт название независимо от исходного представления.
func v190TestVariant(t *testing.T, tag, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v190TestLine(t, "@"+tag+" "+raw, "tag", parent, depth, v190TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1), v190TestElement(t, "name", raw, value, len(tag)+3))
}

// v190TestPair создаёт две непустые ветви; названия находятся на строках 4 и 6.
func v190TestPair(t *testing.T, firstRaw, firstValue, secondRaw, secondValue string, rawOnly bool) *model.Result {
	t.Helper()
	first := v190TestVariant(t, "variant", firstRaw, firstValue, 3, 2)
	second := v190TestVariant(t, "variant", secondRaw, secondValue, 3, 2)
	if rawOnly {
		first.Elements = first.Elements[:1]
		second.Elements = second.Elements[:1]
	}
	return v190TestBlock(t, "variants", first, v190TestText(t, "Read.", "Read.", 4, 3), second, v190TestText(t, "Write.", "Write.", 6, 3))
}

// v190TestRun проверяет полный конвейер, исходные диапазоны и историю модели.
func v190TestRun(t *testing.T, result *model.Result, want []v190Want) {
	t.Helper()
	before := v190TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v190TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v190TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v190-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V190" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V190 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]}, End: model.Position{Line: expected.main[0], Column: expected.main[2]}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Название ветви повторяется в одном @variants." || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V190 occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, expected)
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
	if err := json.Unmarshal([]byte(v190TestSnapshot(t, result)), &restored); err != nil {
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
	if v190TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v190TestUnparsed задаёт неразобранную область; исходную ошибку добавляет Document.
func v190TestUnparsed(t *testing.T, raw string, kind model.LineType, parent, depth int) model.Line {
	t.Helper()
	return v190TestLine(t, raw, kind, parent, depth, model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
}

// v190TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v190TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v190TestLine задаёт структуру строки без повторного разбора DSL.
func v190TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v190TestOpen задаёт блочное объявление без параметров с явным родителем.
func v190TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v190TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v190TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v190TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v190TestClose задаёт закрытие указанного блока.
func v190TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v190TestLine(t, "}", "block-end", parent, depth, v190TestElement(t, "block-close", "}", "}", 1))
}

// v190TestText сохраняет исходное и семантическое содержимое независимо.
func v190TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v190TestLine(t, raw, "content", parent, depth, v190TestElement(t, "content", raw, value, 1))
}

// v190TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v190TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v190TestOpen(t, name, 2, 1)}, body...)
	return v190TestDocument(t, append(lines, v190TestClose(t, 3, 2))...)
}

// v190TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v190TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v190TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v190TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v190TestElement(t, "version", "1.2", "1.2", 14)),
		v190TestLine(t, "# Lesson", "heading", 0, 0, v190TestElement(t, "heading-level", "#", "1", 1), v190TestElement(t, "title", "Lesson", "Lesson", 3)),
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

// v190TestRegistry использует только полный штатный каталог.
func v190TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v190TestSnapshot сохраняет независимое представление модели для сравнения.
func v190TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

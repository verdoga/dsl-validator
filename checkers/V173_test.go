package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v173Expected задаёт исходные строку и Unicode-колонки [start,end).
type v173Expected struct{ line, start, end int }

func TestV173ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		body         []model.Line
		want         []v173Expected
	}{
		{"reference", "@matching {\n\\# Обозначение\n---\nРешётка\nЗвёздочка\n}",
			[]model.Line{v173TestOpen(t, "matching", 2, 1), v173TestText(t, `\# Обозначение`, "# Обозначение", 3, 2), v173TestLine(t, "---", "separator", 3, 2),
				v173TestText(t, "Решётка", "Решётка", 3, 2), v173TestText(t, "Звёздочка", "Звёздочка", 3, 2), v173TestClose(t, 3, 2)}, nil},
		{"example", "@matching {\nСолнце — звезда?\n# Подсказка\n---\nДа\nНет\n}",
			[]model.Line{v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v173TestInvalid(t, "# Подсказка", 3, 2),
				v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2)}, []v173Expected{{5, 1, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// По DSL 1.30 добавлены только обязательные версия и основной заголовок.
			v173TestRun(t, v173TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV173InitialMarkerAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v173Expected
	}{
		{"one", "# Подсказка", []v173Expected{{5, 1, 12}}},
		{"two", "## Подсказка", []v173Expected{{5, 1, 13}}},
		{"three", "### Подсказка", []v173Expected{{5, 1, 14}}},
		{"one without title", "#", []v173Expected{{5, 1, 2}}},
		{"two without title", "##", []v173Expected{{5, 1, 3}}},
		{"three without title", "###", []v173Expected{{5, 1, 4}}},
		{"marker and trailing spaces", "### \t", []v173Expected{{5, 1, 4}}},
		{"no separator", "#A", []v173Expected{{5, 1, 3}}},
		{"tab separator", "#\tA", []v173Expected{{5, 1, 4}}},
		{"separated runs", "# # A", []v173Expected{{5, 1, 6}}},
		{"escaped next hash", "#\\# A", []v173Expected{{5, 1, 6}}},
		{"ASCII edges", " \t# Подсказка \t", []v173Expected{{5, 3, 14}}},
		{"internal spaces and tab", "#  A\tB", []v173Expected{{5, 1, 7}}},
		{"supplementary and combining Unicode", "# 🌞е\u0301", []v173Expected{{5, 1, 6}}},
		{"trailing NBSP retained", "# A\u00a0", []v173Expected{{5, 1, 5}}},
		{"title escapes retained", `# A\{B\}`, []v173Expected{{5, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v173TestRun(t, v173TestMatching(t, v173TestInvalid(t, tc.raw, 3, 2)), "current", tc.want)
		})
	}
	t.Run("partially recognized marker", func(t *testing.T) {
		unparsed := v173TestElement(t, "unparsed", " Подсказка", "", 2, 12)
		unparsed.Value = nil
		line := v173TestLine(t, "# Подсказка", "invalid", 3, 2, v173TestElement(t, "heading-level", "#", "1", 1, 2), unparsed)
		v173TestRun(t, v173TestMatching(t, line), "current", []v173Expected{{5, 1, 12}})
	})
	t.Run("recognized title with forbidden marker", func(t *testing.T) {
		unparsed := v173TestElement(t, "unparsed", "#", "", 1, 2)
		unparsed.Value = nil
		line := v173TestLine(t, "# Подсказка", "invalid", 3, 2, unparsed, v173TestElement(t, "title", "Подсказка", "Подсказка", 3, 12))
		v173TestRun(t, v173TestMatching(t, line), "current", []v173Expected{{5, 1, 12}})
	})
}

func TestV173ExcludedMarkers(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"escaped one", `\# A`, "# A"},
		{"escaped two", `\## A`, "## A"},
		{"escaped three", `\### A`, "### A"},
		{"escaped with indentation", " \t\\# A", "# A"},
		{"escaped backslash", `\\# A`, `\# A`},
		{"middle of text", "Текст # A", "Текст # A"},
		{"NBSP before marker", "\u00a0# A", "\u00a0# A"},
		{"em space before marker", " \t\u2003# A", "\u2003# A"},
		{"ordinary text", "Подсказка", "Подсказка"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v173TestRun(t, v173TestMatching(t, v173TestText(t, tc.raw, tc.value, 3, 2)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v173TestRun(t, v173TestMatching(t, v173TestBlank(t, raw, 3, 2)), "current", nil)
		})
	}
	for _, tc := range []struct {
		name, raw  string
		start, end int
	}{
		{"four", "#### A", 1, 5},
		{"five", "#####", 1, 6},
		{"long run", strings.Repeat("#", 64) + " A", 1, 65},
		{"four after indentation", " \t#### A", 3, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v173TestMatching(t, v173TestInvalid(t, tc.raw, 3, 2))
			v173TestRun(t, result, "current", nil)
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source == "current" && entry.DiagnosticCode == "V050" {
					count++
					want := model.Location{Start: model.Position{Line: 5, Column: tc.start}, End: model.Position{Line: 5, Column: tc.end}}
					if entry.Location == nil || *entry.Location != want || entry.Message != "В заголовке использовано более трёх знаков #." || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || len(entry.RelatedLocations) != 0 {
						t.Fatalf("unexpected V050: %+v", entry)
					}
				}
			}
			if count != 1 {
				t.Fatalf("V050 count=%d, want 1", count)
			}
		})
	}
}

func TestV173WholeMatchingBody(t *testing.T) {
	t.Run("both lists", func(t *testing.T) {
		v173TestRun(t, v173TestDocument(t, v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", 3, 2), v173TestInvalid(t, "# A", 3, 2),
			v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestInvalid(t, "## B", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2)), "current", []v173Expected{{5, 1, 4}, {8, 1, 5}})
	})
	t.Run("repeated headings are separate", func(t *testing.T) {
		v173TestRun(t, v173TestDocument(t, v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", 3, 2), v173TestInvalid(t, "# A", 3, 2), v173TestInvalid(t, "# A", 3, 2),
			v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2)), "current", []v173Expected{{5, 1, 4}, {6, 1, 4}})
	})
	t.Run("first body line", func(t *testing.T) {
		v173TestRun(t, v173TestDocument(t, v173TestOpen(t, "matching", 2, 1), v173TestInvalid(t, "# A", 3, 2), v173TestText(t, "Вопрос", "Вопрос", 3, 2),
			v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2)), "current", []v173Expected{{4, 1, 4}})
	})
	t.Run("last body line", func(t *testing.T) {
		v173TestRun(t, v173TestDocument(t, v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", 3, 2), v173TestLine(t, "---", "separator", 3, 2),
			v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestInvalid(t, "# A", 3, 2), v173TestClose(t, 3, 2)), "current", []v173Expected{{8, 1, 4}})
	})
	for _, count := range []int{0, 2} {
		t.Run("separator count="+strconv.Itoa(count), func(t *testing.T) {
			body := []model.Line{v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", 3, 2), v173TestInvalid(t, "# Подсказка", 3, 2)}
			for n := 0; n < count; n++ {
				body = append(body, v173TestLine(t, "---", "separator", 3, 2))
			}
			body = append(body, v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2))
			v173TestRun(t, v173TestDocument(t, body...), "current", []v173Expected{{5, 1, 12}})
		})
	}
	for _, partial := range []bool{false, true} {
		t.Run("automatic answers partial="+strconv.FormatBool(partial), func(t *testing.T) {
			body := []model.Line{v173TestOpen(t, "matching", 2, 1), v173TestBlank(t, "", 3, 2), v173TestInvalid(t, "# Подсказка", 3, 2),
				v173TestText(t, "Первый вопрос", "Первый вопрос", 3, 2), v173TestTag(t, "answer", "content", "Да", 3, 2), v173TestText(t, "Другой вопрос", "Другой вопрос", 3, 2)}
			if !partial {
				body = append(body, v173TestTag(t, "answer", "content", "Нет", 3, 2))
			}
			body = append(body, v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2))
			v173TestRun(t, v173TestDocument(t, body...), "current", []v173Expected{{5, 1, 12}})
		})
	}
}

func TestV173Containers(t *testing.T) {
	for _, tag := range []string{"answer", "hint", "editor", "multifill"} {
		t.Run("nested "+tag, func(t *testing.T) {
			body := []model.Line{v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", 3, 2)}
			if tag == "hint" {
				body = append(body, v173TestTag(t, "answer", "content", "Да", 3, 2))
			} else {
				body = append(body, v173TestBlank(t, "", 3, 2))
			}
			body = append(body, v173TestOpen(t, tag, 3, 2))
			if tag == "editor" || tag == "multifill" {
				body = append(body, v173TestText(t, "# A", "# A", 6, 3))
			} else {
				body = append(body, v173TestInvalid(t, "# A", 6, 3))
			}
			body = append(body, v173TestClose(t, 6, 3), v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2))
			result := v173TestDocument(t, body...)
			if tag == "editor" || tag == "multifill" {
				v173TestError(t, result, 6, -1, "Вложенный тег запрещён в matching.")
			}
			v173TestRun(t, result, "current", nil)
		})
	}
	for _, tc := range []struct {
		name                   string
		prefix, suffix         []model.Line
		opening, parent, depth int
		want                   []v173Expected
	}{
		{"task", []model.Line{v173TestTag(t, "task", "identifier", "first", 2, 1)}, []model.Line{v173TestTag(t, "endtask", "", "", 2, 1)}, 4, 3, 2, []v173Expected{{6, 1, 4}}},
		{"step", []model.Line{v173TestTag(t, "task", "identifier", "first", 2, 1), v173TestTag(t, "step", "title", "First", 3, 2)}, []model.Line{v173TestTag(t, "endtask", "", "", 2, 1)}, 5, 4, 3, []v173Expected{{7, 1, 4}}},
		{"variant", []model.Line{v173TestOpen(t, "variants", 2, 1), v173TestTag(t, "variant", "name", "A", 3, 2)},
			[]model.Line{v173TestTag(t, "variant", "name", "B", 3, 2), v173TestText(t, "Читайте.", "Читайте.", 12, 3), v173TestClose(t, 3, 2)}, 5, 4, 3, []v173Expected{{7, 1, 4}}},
		{"fragment", []model.Line{v173TestLine(t, "@fragment f {", "block-start", 2, 1, v173TestElement(t, "tag", "@fragment", "fragment", 1, 10),
			v173TestElement(t, "identifier", "f", "f", 11, 12), v173TestElement(t, "block-open", "{", "{", 13, 14))}, []model.Line{v173TestClose(t, 3, 2)}, 4, 3, 2, []v173Expected{{6, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.prefix, v173TestOpen(t, "matching", tc.parent, tc.depth), v173TestText(t, "Вопрос", "Вопрос", tc.opening, tc.depth+1), v173TestInvalid(t, "# A", tc.opening, tc.depth+1),
				v173TestLine(t, "---", "separator", tc.opening, tc.depth+1), v173TestText(t, "Да", "Да", tc.opening, tc.depth+1), v173TestText(t, "Нет", "Нет", tc.opening, tc.depth+1), v173TestClose(t, tc.opening, tc.depth+1))
			body = append(body, tc.suffix...)
			result := v173TestDocument(t, body...)
			if tc.name == "fragment" {
				v173TestError(t, result, 4, -1, "Интерактивный matching запрещён в fragment; его структура сохранена.")
			}
			v173TestRun(t, result, "current", tc.want)
		})
	}
	for _, tag := range []string{"choice", "ordering", "fragment"} {
		t.Run("other immediate container "+tag, func(t *testing.T) {
			opening := v173TestOpen(t, tag, 2, 1)
			if tag == "fragment" {
				opening = v173TestLine(t, "@fragment f {", "block-start", 2, 1, v173TestElement(t, "tag", "@fragment", "fragment", 1, 10), v173TestElement(t, "identifier", "f", "f", 11, 12), v173TestElement(t, "block-open", "{", "{", 13, 14))
			}
			body := []model.Line{opening, v173TestText(t, "Текст", "Текст", 3, 2), v173TestInvalid(t, "# A", 3, 2)}
			if tag == "ordering" {
				body = append(body, v173TestLine(t, "---", "separator", 3, 2))
			}
			body = append(body, v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2))
			v173TestRun(t, v173TestDocument(t, body...), "current", nil)
		})
	}
	t.Run("after matching closing", func(t *testing.T) {
		v173TestRun(t, v173TestDocument(t, v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", 3, 2), v173TestLine(t, "---", "separator", 3, 2),
			v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2),
			v173TestLine(t, "### Next", "heading", 0, 0, v173TestElement(t, "heading-level", "###", "3", 1, 4), v173TestElement(t, "title", "Next", "Next", 5, 9)),
			v173TestText(t, "Читайте.", "Читайте.", 9, 1)), "current", nil)
	})
	t.Run("independent matching blocks", func(t *testing.T) {
		var body []model.Line
		for _, opening := range []int{3, 10} {
			body = append(body, v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Вопрос", "Вопрос", opening, 2), v173TestInvalid(t, "# A", opening, 2),
				v173TestLine(t, "---", "separator", opening, 2), v173TestText(t, "Да", "Да", opening, 2), v173TestText(t, "Нет", "Нет", opening, 2), v173TestClose(t, opening, 2))
		}
		v173TestRun(t, v173TestDocument(t, body...), "current", []v173Expected{{5, 1, 4}, {12, 1, 4}})
	})
	t.Run("literal matching mention", func(t *testing.T) {
		v173TestRun(t, v173TestDocument(t, v173TestOpen(t, "editor", 2, 1), v173TestText(t, "@matching {", "@matching {", 3, 2), v173TestText(t, "# A", "# A", 3, 2), v173TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV173RegistrationAndHistory(t *testing.T) {
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
		if definition.Code == "V173" {
			count++
			if definition.Severity != "error" || definition.Scope != "line" || definition.Message != "Заголовок не разрешён внутри @matching." || definition.Check == nil || definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
				t.Fatalf("unexpected definition: %+v", definition)
			}
		}
	}
	if count != 1 {
		t.Fatalf("V173 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v173TestMatching(t, v173TestInvalid(t, "# Подсказка", 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0] = v173TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v173TestRun(t, result, "current", nil)
		})
	}
	t.Run("mixed case matching", func(t *testing.T) {
		result := v173TestMatching(t, v173TestInvalid(t, "# Подсказка", 3, 2))
		result.Lines[2] = v173TestLine(t, "@MaTcHiNg {", "block-start", 2, 1, v173TestElement(t, "tag", "@MaTcHiNg", "matching", 1, 10), v173TestElement(t, "block-open", "{", "{", 11, 12))
		result.Lines[2].Number = 3
		v173TestRun(t, result, "current", []v173Expected{{5, 1, 12}})
	})
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v173TestMatching(t, v173TestInvalid(t, "# Подсказка", 3, 2))
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v173Expected{{5, 1, 12}}
			v173TestRun(t, result, "first", want)
			v173TestRun(t, result, "second", want)
			v173TestRun(t, v173TestMatching(t, v173TestText(t, `\# Подсказка`, "# Подсказка", 3, 2)), "separate", nil)
		})
	}
}

// v173TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v173TestDocument.
func v173TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v173TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v173TestLine(t, raw, "invalid", parent, depth, element)
}

// v173TestMatching помещает заданную строку на позицию 5 полного блока.
func v173TestMatching(t *testing.T, candidate model.Line) *model.Result {
	t.Helper()
	return v173TestDocument(t, v173TestOpen(t, "matching", 2, 1), v173TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), candidate,
		v173TestLine(t, "---", "separator", 3, 2), v173TestText(t, "Да", "Да", 3, 2), v173TestText(t, "Нет", "Нет", 3, 2), v173TestClose(t, 3, 2))
}

// v173TestRun запускает штатный конвейер и проверяет только V173 текущего запуска.
func v173TestRun(t *testing.T, result *model.Result, source string, want []v173Expected) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V173" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V173 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "Заголовок не разрешён внутри @matching."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("ScopeLine must not create element errorIds")
					}
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
		}

	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original model.Result
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	for index, line := range original.Lines {
		current := result.Lines[index]
		current.HasErrors = line.HasErrors
		current.Elements = append([]model.Element{}, current.Elements...)
		for element := range current.Elements {
			current.Elements[element].ErrorIDs = line.Elements[element].ErrorIDs
		}
		left, _ := json.Marshal(line)
		right, _ := json.Marshal(current)
		if string(left) != string(right) {
			t.Fatalf("source changed on line %d", index+1)
		}
	}
	for index, old := range original.Diagnostics {
		left, _ := json.Marshal(old)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v173TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v173TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
	})
}

// v173TestElement задаёт исходное и семантическое значения без преобразований.
func v173TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v173TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v173TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v173TestTag создаёт однострочное объявление с явно заданным параметром.
func v173TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v173TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v173TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v173TestLine(t, raw, "tag", parent, depth, elements...)
}

// v173TestOpen создаёт объявление блока без параметров.
func v173TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v173TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v173TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v173TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v173TestClose создаёт закрытие с заданным родителем.
func v173TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v173TestLine(t, "}", "block-end", parent, depth, v173TestElement(t, "block-close", "}", "}", 1, 2))
}

// v173TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v173TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v173TestLine(t, raw, "content", parent, depth, v173TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v173TestBlank задаёт обычную пустую строку без текстового элемента.
func v173TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v173TestLine(t, raw, "blank", parent, depth)
}

// v173TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v173TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v173TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v173TestLine(t, "# Lesson", "heading", 0, 0, v173TestElement(t, "heading-level", "#", "1", 1, 2), v173TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		for j, element := range result.Lines[i].Elements {
			if element.ElementType == "unparsed" {
				v173TestError(t, result, i+1, j, "Запрещённый или неправильно оформленный заголовок.")
			}
		}
	}
	return result

}

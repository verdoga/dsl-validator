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

// v196Expected задаёт независимые основной и связанный диапазоны; конец исключается.
type v196Expected struct{ line, start, end, relatedLine, relatedStart, relatedEnd int }

func TestV196ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		body         []model.Line
		want         []v196Expected
	}{
		{"reference", "@variants {\n@variant A\nRead.\n@variant B\nWrite.\n}\n@newpage\nContinue.", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2), v196TestText(t, "Read.", "Read.", 4, 3),
			v196TestTag(t, "variant", "name", "B", 3, 2), v196TestText(t, "Write.", "Write.", 6, 3), v196TestClose(t, 3, 2),
			v196TestTag(t, "newpage", "", "", 2, 1), v196TestText(t, "Continue.", "Continue.", 9, 2),
		}, nil},
		{"example", "@variants {\n@variant A\nRead.\n@newpage\nContinue.\n@variant B\nWrite.\n}", v196TestExample(t), []v196Expected{{6, 1, 9, 3, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// Добавлены только обязательные версия и основной заголовок по DSL 1.30.
			v196TestRun(t, v196TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV196PhysicalAndLogicalContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v196Expected
	}{
		{"before first branch", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 3, 2), v196TestTag(t, "variant", "name", "A", 3, 2),
			v196TestText(t, "Read.", "Read.", 5, 3), v196TestClose(t, 3, 2),
		}, []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"no branches or following material", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 3, 2), v196TestClose(t, 3, 2),
		}, []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"inside task", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2), v196TestTag(t, "task", "identifier", "t", 4, 3),
			v196TestTag(t, "newpage", "", "", 5, 4), v196TestText(t, "Read.", "Read.", 6, 5), v196TestTag(t, "endtask", "", "", 5, 4), v196TestClose(t, 3, 2),
		}, []v196Expected{{6, 1, 9, 3, 1, 10}}},
		{"inside step", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2), v196TestTag(t, "task", "identifier", "t", 4, 3),
			v196TestTag(t, "step", "", "", 5, 4), v196TestTag(t, "newpage", "", "", 6, 5), v196TestText(t, "Read.", "Read.", 7, 6),
			v196TestTag(t, "endtask", "", "", 5, 4), v196TestClose(t, 3, 2),
		}, []v196Expected{{7, 1, 9, 3, 1, 10}}},
		{"logical parent moved outside", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2), v196TestText(t, "Read.", "Read.", 4, 3),
			v196TestTag(t, "newpage", "", "", 2, 1), v196TestText(t, "Continue.", "Continue.", 6, 2),
			v196TestTag(t, "newpage", "", "", 6, 2), v196TestClose(t, 3, 2),
			v196TestTag(t, "newpage", "", "", 2, 1), v196TestText(t, "Outside.", "Outside.", 10, 2),
		}, []v196Expected{{6, 1, 9, 3, 1, 10}, {8, 1, 9, 3, 1, 10}}},
		{"consecutive declarations", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 3, 2), v196TestTag(t, "newpage", "", "", 3, 2), v196TestClose(t, 3, 2),
		}, []v196Expected{{4, 1, 9, 3, 1, 10}, {5, 1, 9, 3, 1, 10}}},
		{"independent blocks", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 3, 2), v196TestClose(t, 3, 2),
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 6, 2), v196TestClose(t, 6, 2),
		}, []v196Expected{{4, 1, 9, 3, 1, 10}, {7, 1, 9, 6, 1, 10}}},
		{"nested variants owns related range", []model.Line{
			v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2), v196TestOpen(t, "variants", 4, 3),
			v196TestTag(t, "variant", "name", "B", 5, 4), v196TestTag(t, "newpage", "", "", 6, 5), v196TestText(t, "Read.", "Read.", 6, 5),
			v196TestClose(t, 5, 4), v196TestClose(t, 3, 2),
		}, []v196Expected{{7, 1, 9, 5, 1, 10}}},
		{"variants inside task", []model.Line{
			v196TestTag(t, "task", "identifier", "t", 2, 1), v196TestOpen(t, "variants", 3, 2), v196TestTag(t, "newpage", "", "", 4, 3),
			v196TestClose(t, 4, 3), v196TestTag(t, "endtask", "", "", 3, 2),
		}, []v196Expected{{5, 1, 9, 4, 1, 10}}},
		{"outside any variants", []model.Line{v196TestTag(t, "newpage", "", "", 2, 1), v196TestText(t, "Read.", "Read.", 3, 2)}, nil},
		{"empty variants", []model.Line{v196TestOpen(t, "variants", 2, 1), v196TestClose(t, 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v196TestDocument(t, tc.body...)
			if tc.name == "nested variants owns related range" {
				v196TestError(t, result, 5, -1, "Вложение variants запрещено.")
			}
			if tc.name == "logical parent moved outside" {
				v196TestError(t, result, 6, -1, "Недопустимая граница шага внутри variants.")
			}
			v196TestRun(t, result, "current", tc.want)
			if tc.name == "before first branch" {
				for _, entry := range result.Diagnostics {
					if entry.Source == "current" && entry.DiagnosticCode == "V192" {
						t.Fatal("V192 duplicates newpage")
					}
				}
			}
		})
	}
}

func TestV196RawNameBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v196Expected
	}{
		{"end of line", "@newpage", []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"space", "@newpage extra", []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"tab", "@newpage\textra", []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"trailing tabs", "@newpage \t", []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"mixed case and indent", " \t@NeWpAgE", []v196Expected{{4, 3, 11, 3, 1, 10}}},
		{"Unicode remainder", "@newpage Текст😀", []v196Expected{{4, 1, 9, 3, 1, 10}}},
		{"longer name", "@newpageX", nil},
		{"plural", "@newpages", nil},
		{"hyphen", "@newpage-extra", nil},
		{"Unicode suffix", "@newpageЯ", nil},
		{"NBSP is not delimiter", "@newpage\u00a0X", nil},
		{"em space is not delimiter", "@newpage\u2003X", nil},
		{"closing brace is not delimiter", "@newpage}", nil},
		{"punctuation", "@newpage!", nil},
		{"short name", "@newpag", nil},
		{"escaped", `\@newpage`, nil},
		{"mention", "Text @newpage", nil},
		{"NBSP is not indentation", "\u00a0@newpage", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// invalid с unparsed сохраняет исходную ошибку, имя не выделено моделью.
			v196TestRun(t, v196TestContainer(t, v196TestInvalid(t, tc.raw, 3, 2)), "current", tc.want)
		})
	}
	t.Run("opening brace directly after recovered name", func(t *testing.T) {
		unparsed := v196TestElement(t, "unparsed", "@newpage", "", 1, 9)
		unparsed.Value = nil
		line := v196TestLine(t, "@newpage{", "block-start", 3, 2, unparsed, v196TestElement(t, "block-open", "{", "{", 9, 10))
		v196TestRun(t, v196TestContainer(t, line, v196TestClose(t, 4, 3)), "current", []v196Expected{{4, 1, 9, 3, 1, 10}})
	})
	t.Run("unparsed declaration in content line", func(t *testing.T) {
		line := v196TestInvalid(t, "@newpage extra", 3, 2)
		line.LineType = "content"
		v196TestRun(t, v196TestContainer(t, line), "current", []v196Expected{{4, 1, 9, 3, 1, 10}})
	})
	for _, suffix := range []string{" extra", "\textra", "\u00a0extra"} {
		t.Run("recognized tag with remainder "+suffix, func(t *testing.T) {
			unparsed := v196TestElement(t, "unparsed", suffix, "", 9, 9+utf8.RuneCountInString(suffix))
			unparsed.Value = nil
			line := v196TestLine(t, "@newpage"+suffix, "invalid", 3, 2,
				v196TestElement(t, "tag", "@newpage", "newpage", 1, 9), unparsed)
			v196TestRun(t, v196TestContainer(t, line), "current", []v196Expected{{4, 1, 9, 3, 1, 10}})
		})
	}
	for _, tc := range []struct{ raw, value string }{{`\@newpage`, "@newpage"}, {"Text @newpage", "Text @newpage"}} {
		t.Run("ordinary text "+tc.raw, func(t *testing.T) {
			v196TestRun(t, v196TestContainer(t, v196TestText(t, tc.raw, tc.value, 3, 2)), "current", nil)
		})
	}
}

func TestV196ExcludedContainers(t *testing.T) {
	for _, tag := range []string{"example", "wordlist", "table", "script", "text", "key", "instr", "note", "alt", "hint", "fragment",
		"answer", "choice", "multichoice", "matching", "ordering", "editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			opening := v196TestOpen(t, tag, 4, 3)
			if tag == "fragment" {
				opening = v196TestLine(t, "@fragment f {", "block-start", 4, 3, v196TestElement(t, "tag", "@fragment", "fragment", 1, 10),
					v196TestElement(t, "identifier", "f", "f", 11, 12), v196TestElement(t, "block-open", "{", "{", 13, 14))
			}
			child := v196TestTag(t, "newpage", "", "", 5, 4)
			if tag == "editor" || tag == "multifill" {
				child = v196TestText(t, "@newpage", "@newpage", 5, 4)
			}
			result := v196TestDocument(t, v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2),
				opening, child, v196TestClose(t, 5, 4), v196TestClose(t, 3, 2))
			v196TestRun(t, result, "current", nil)
			if tag == "note" {
				count := 0
				for _, entry := range result.Diagnostics {
					if entry.Source != "current" || entry.DiagnosticCode != "V124" {
						continue
					}
					count++
					primary := model.Location{Start: model.Position{Line: 6, Column: 1}, End: model.Position{Line: 6, Column: 9}}
					related := model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 6}}
					if entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
						t.Fatalf("unexpected V124 ranges: %+v", entry)
					}
				}
				if count != 1 {
					t.Fatalf("V124 count=%d, want 1", count)
				}
			}
		})
	}
	t.Run("return from content block", func(t *testing.T) {
		result := v196TestContainer(t, v196TestOpen(t, "note", 3, 2), v196TestTag(t, "newpage", "", "", 4, 3), v196TestClose(t, 4, 3),
			v196TestTag(t, "newpage", "", "", 3, 2))
		v196TestRun(t, result, "current", []v196Expected{{7, 1, 9, 3, 1, 10}})
	})
	t.Run("physical content block despite logical reset", func(t *testing.T) {
		result := v196TestContainer(t, v196TestOpen(t, "note", 3, 2), v196TestTag(t, "newpage", "", "", 2, 1),
			v196TestClose(t, 4, 3), v196TestTag(t, "newpage", "", "", 2, 1))
		v196TestError(t, result, 5, -1, "Недопустимая граница шага внутри note.")
		v196TestRun(t, result, "current", []v196Expected{{7, 1, 9, 3, 1, 10}})
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "current" || entry.DiagnosticCode != "V124" {
				continue
			}
			count++
			primary := model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 9}}
			related := model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 6}}
			if entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
				t.Fatalf("unexpected V124 ranges: %+v", entry)
			}
		}
		if count != 1 {
			t.Fatalf("V124 count=%d, want 1", count)
		}
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal variants in "+tag, func(t *testing.T) {
			v196TestRun(t, v196TestDocument(t, v196TestOpen(t, tag, 2, 1), v196TestText(t, "@variants {", "@variants {", 3, 2),
				v196TestText(t, "@newpage", "@newpage", 3, 2), v196TestText(t, `\}`, "}", 3, 2), v196TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV196BoundedVariantsInsideUnclosedOuterBlock(t *testing.T) {
	result := v196TestDocument(t, v196TestOpen(t, "note", 2, 1), v196TestOpen(t, "variants", 3, 2),
		v196TestTag(t, "newpage", "", "", 4, 3), v196TestClose(t, 4, 3))
	v196TestError(t, result, 3, -1, "Внешний note не закрыт.")
	v196TestError(t, result, 4, -1, "Вложенный variants запрещён в note.")
	v196TestRun(t, result, "current", []v196Expected{{5, 1, 9, 4, 1, 10}})
}

func TestV196UncertainBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		errorLine int
	}{
		{"unclosed variants", []model.Line{v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 3, 2)}, 3},
		{"ambiguous closing", []model.Line{v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "newpage", "", "", 3, 2), v196TestClose(t, 3, 2), v196TestClose(t, 3, 2)}, 6},
		{"unclosed nested block", []model.Line{v196TestOpen(t, "variants", 2, 1), v196TestOpen(t, "note", 3, 2),
			v196TestTag(t, "newpage", "", "", 3, 2), v196TestClose(t, 3, 2)}, 4},
		{"crossed intervals", []model.Line{v196TestOpen(t, "variants", 2, 1), v196TestOpen(t, "variants", 3, 2),
			v196TestTag(t, "newpage", "", "", 4, 3), v196TestClose(t, 3, 2), v196TestTag(t, "newpage", "", "", 4, 3), v196TestClose(t, 4, 3)}, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v196TestDocument(t, tc.body...)
			v196TestError(t, result, tc.errorLine, -1, "Границы блока не определены однозначно.")
			v196TestRun(t, result, "current", nil)
		})
	}
}

func TestV196RangesRegistrationAndHistory(t *testing.T) {
	t.Run("indented declarations and container", func(t *testing.T) {
		result := v196TestContainer(t, v196TestTag(t, "newpage", "", "", 3, 2))
		result.Lines[2].Raw = " \t@VaRiAnTs {"
		result.Lines[2].Elements[0].Raw = "@VaRiAnTs"
		for i := range result.Lines[2].Elements {
			result.Lines[2].Elements[i].Start += 2
			result.Lines[2].Elements[i].End += 2
		}
		result.Lines[3].Raw = " \t@NeWpAgE \t"
		result.Lines[3].Elements[0].Raw = "@NeWpAgE"
		result.Lines[3].Elements[0].Start, result.Lines[3].Elements[0].End = 3, 11
		v196TestRun(t, result, "current", []v196Expected{{4, 3, 11, 3, 3, 12}})
	})
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
		if definition.Code != "V196" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Message != "@newpage размещён внутри @variants." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V196 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v196TestDocument(t, v196TestExample(t)...)
			result.Document.DSLVersion = &version
			result.Lines[0] = v196TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v196TestRun(t, result, "current", nil)
		})
	}
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v196TestDocument(t, v196TestExample(t)...)
			for i := range result.Lines {
				if ending == "LF" {
					result.Lines[i].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[i].LineEnding = "\r\n"
				}
			}
			v196TestRun(t, result, "first", []v196Expected{{6, 1, 9, 3, 1, 10}})
			v196TestRun(t, result, "second", []v196Expected{{6, 1, 9, 3, 1, 10}})
			v196TestRun(t, v196TestContainer(t), "separate", nil)
		})
	}
}

// v196TestExample сохраняет пример таблицы с учётом двух вводных строк.
func v196TestExample(t *testing.T) []model.Line {
	t.Helper()
	return []model.Line{v196TestOpen(t, "variants", 2, 1), v196TestTag(t, "variant", "name", "A", 3, 2), v196TestText(t, "Read.", "Read.", 4, 3),
		v196TestTag(t, "newpage", "", "", 4, 3), v196TestText(t, "Continue.", "Continue.", 6, 4),
		v196TestTag(t, "variant", "name", "B", 3, 2), v196TestText(t, "Write.", "Write.", 8, 3), v196TestClose(t, 3, 2)}
}

// v196TestContainer заключает явно заданные строки в variants на строке 3.
func v196TestContainer(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v196TestOpen(t, "variants", 2, 1)}, rows...)
	body = append(body, v196TestClose(t, 3, 2))
	return v196TestDocument(t, body...)
}

// v196TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v196TestDocument.
func v196TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v196TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v196TestLine(t, raw, "invalid", parent, depth, element)
}

// v196TestRun запускает штатный конвейер и проверяет только V196 текущего запуска.
func v196TestRun(t *testing.T, result *model.Result, source string, want []v196Expected) {
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
		if diagnostic.DiagnosticCode == "V196" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V196 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "@newpage размещён внутри @variants."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 1 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		related := model.Location{Start: model.Position{Line: expected.relatedLine, Column: expected.relatedStart}, End: model.Position{Line: expected.relatedLine, Column: expected.relatedEnd}}
		if entry.RelatedLocations[0] != related {
			t.Fatalf("related = %+v, want %+v", entry.RelatedLocations, related)
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

// v196TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v196TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v196TestElement задаёт исходное и семантическое значения без преобразований.
func v196TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v196TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v196TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v196TestTag создаёт однострочное объявление с явно заданным параметром.
func v196TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v196TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v196TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v196TestLine(t, raw, "tag", parent, depth, elements...)
}

// v196TestOpen создаёт объявление блока без параметров.
func v196TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v196TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v196TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v196TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v196TestClose создаёт закрытие с заданным родителем.
func v196TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v196TestLine(t, "}", "block-end", parent, depth, v196TestElement(t, "block-close", "}", "}", 1, 2))
}

// v196TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v196TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v196TestLine(t, raw, "content", parent, depth, v196TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v196TestBlank задаёт обычную пустую строку без текстового элемента.
func v196TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v196TestLine(t, raw, "blank", parent, depth)
}

// v196TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v196TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v196TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v196TestLine(t, "# Lesson", "heading", 0, 0, v196TestElement(t, "heading-level", "#", "1", 1, 2), v196TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
		// Исходные ошибки принадлежат конкретной модели, а не повторно
		// используемому описанию строки в таблице тестов.
		lines[index].Elements = append([]model.Element{}, lines[index].Elements...)
		for element := range lines[index].Elements {
			lines[index].Elements[element].ErrorIDs = append([]string{}, lines[index].Elements[element].ErrorIDs...)
		}
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		for j, element := range result.Lines[i].Elements {
			if element.ElementType == "unparsed" {
				v196TestError(t, result, i+1, j, "Строка разобрана не полностью.")
			}
		}
	}
	return result

}

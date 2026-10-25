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

// v177Expected задаёт исходный диапазон с исключающим концом.
type v177Expected struct{ line, start, endLine, end int }

func TestV177ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		body         []model.Line
		want         []v177Expected
	}{
		{"reference", "@ordering {\nA\nB\nC\n---\nFirst\nLast\n}", []model.Line{
			v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2), v177TestText(t, "C", "C", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestText(t, "Last", "Last", 3, 2), v177TestClose(t, 3, 2),
		}, nil},
		{"example", "@ordering {\nA\nB\n---\nFirst\n}", []model.Line{
			v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2),
		}, []v177Expected{{6, 1, 8, 2}}},
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
			v177TestRun(t, v177TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV177PositionCount(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v177Expected
	}{
		{"zero", nil, []v177Expected{{6, 1, 7, 2}}},
		{"one", []model.Line{v177TestText(t, "First", "First", 3, 2)}, []v177Expected{{6, 1, 8, 2}}},
		{"two", []model.Line{v177TestText(t, "First", "First", 3, 2), v177TestText(t, "Last", "Last", 3, 2)}, nil},
		{"three", []model.Line{v177TestText(t, "First", "First", 3, 2), v177TestText(t, "Middle", "Middle", 3, 2), v177TestText(t, "Last", "Last", 3, 2)}, nil},
		{"identical labels", []model.Line{v177TestText(t, "First", "First", 3, 2), v177TestText(t, "First", "First", 3, 2)}, nil},
		{"case variants", []model.Line{v177TestText(t, "First", "First", 3, 2), v177TestText(t, "FIRST", "FIRST", 3, 2)}, nil},
		{"numeric labels", []model.Line{v177TestText(t, "01", "01", 3, 2), v177TestText(t, "1", "1", 3, 2)}, nil},
		{"reverse order", []model.Line{v177TestText(t, "Last", "Last", 3, 2), v177TestText(t, "First", "First", 3, 2)}, nil},
		{"blank", []model.Line{v177TestBlank(t, "", 3, 2)}, []v177Expected{{6, 1, 8, 2}}},
		{"ASCII whitespace", []model.Line{v177TestBlank(t, " \t ", 3, 2)}, []v177Expected{{6, 1, 8, 2}}},
		{"blanks around one", []model.Line{v177TestBlank(t, "", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestBlank(t, "\t", 3, 2)}, []v177Expected{{6, 1, 10, 2}}},
		{"NBSP is a position", []model.Line{v177TestText(t, "\u00a0", "\u00a0", 3, 2), v177TestText(t, "First", "First", 3, 2)}, nil},
		{"escaped tag", []model.Line{v177TestText(t, `\@answer First`, "@answer First", 3, 2), v177TestText(t, "Last", "Last", 3, 2)}, nil},
		{"escaped heading", []model.Line{v177TestText(t, `\# First`, "# First", 3, 2), v177TestText(t, "Last", "Last", 3, 2)}, nil},
		{"escaped separator", []model.Line{v177TestText(t, `\---`, "---", 3, 2), v177TestText(t, "Last", "Last", 3, 2)}, nil},
		{"Unicode label", []model.Line{v177TestText(t, "😀е\u0301", "😀е\u0301", 3, 2)}, []v177Expected{{6, 1, 8, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v177TestRun(t, v177TestOrdering(t, tc.rows...), "current", tc.want)
		})
	}
}

func TestV177ExcludedConstructions(t *testing.T) {
	for _, tag := range []string{"hint", "answer", "editor"} {
		t.Run("inline "+tag, func(t *testing.T) {
			result := v177TestOrdering(t, v177TestTag(t, tag, "content", "First", 3, 2))
			v177TestError(t, result, 7, -1, "Тег запрещён после разделителя ordering.")
			v177TestRun(t, result, "current", []v177Expected{{6, 1, 8, 2}})
		})
		t.Run("block "+tag, func(t *testing.T) {
			result := v177TestOrdering(t, v177TestOpen(t, tag, 3, 2), v177TestText(t, "First", "First", 7, 3), v177TestClose(t, 7, 3))
			v177TestError(t, result, 7, -1, "Тег запрещён после разделителя ordering.")
			v177TestRun(t, result, "current", []v177Expected{{6, 1, 10, 2}})
		})
	}
	t.Run("one ordinary row before nested block", func(t *testing.T) {
		result := v177TestOrdering(t, v177TestText(t, "First", "First", 3, 2), v177TestOpen(t, "hint", 3, 2),
			v177TestText(t, "Last", "Last", 8, 3), v177TestClose(t, 8, 3))
		v177TestError(t, result, 8, -1, "Подсказка запрещена после разделителя ordering.")
		v177TestRun(t, result, "current", []v177Expected{{6, 1, 11, 2}})
	})
	t.Run("recognized forbidden heading", func(t *testing.T) {
		unparsed := v177TestElement(t, "unparsed", " First", "", 2, 8)
		unparsed.Value = nil
		line := v177TestLine(t, "# First", "invalid", 3, 2, v177TestElement(t, "heading-level", "#", "1", 1, 2), unparsed)
		v177TestRun(t, v177TestOrdering(t, line), "current", []v177Expected{{6, 1, 8, 2}})
	})
}

func TestV177UnknownContent(t *testing.T) {
	for _, kind := range []model.LineType{"invalid", "content", "tag"} {
		for _, one := range []bool{false, true} {
			t.Run(string(kind)+"/one="+strconv.FormatBool(one), func(t *testing.T) {
				unknown := v177TestInvalid(t, "@unknown ???", 3, 2)
				unknown.LineType = kind
				rows := []model.Line{unknown}
				if one {
					rows = append(rows, v177TestText(t, "First", "First", 3, 2))
				}
				v177TestRun(t, v177TestOrdering(t, rows...), "current", nil)
			})
		}
	}
	t.Run("unknown descendant", func(t *testing.T) {
		result := v177TestOrdering(t, v177TestOpen(t, "hint", 3, 2), v177TestInvalid(t, "@unknown", 7, 3), v177TestClose(t, 7, 3))
		v177TestError(t, result, 7, -1, "Подсказка запрещена после разделителя ordering.")
		v177TestRun(t, result, "current", []v177Expected{{6, 1, 10, 2}})
	})
	t.Run("unknown upper row", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestInvalid(t, "@unknown", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2))
		v177TestRun(t, result, "current", []v177Expected{{6, 1, 8, 2}})
	})
	t.Run("unknown outside", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2), v177TestInvalid(t, "@unknown", 2, 1))
		v177TestRun(t, result, "current", []v177Expected{{6, 1, 8, 2}})
	})
}

func TestV177SeparatorsAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v177Expected
	}{
		{"no separator", nil, nil},
		{"two separators", []model.Line{v177TestLine(t, "---", "separator", 3, 2), v177TestLine(t, "---", "separator", 3, 2)}, nil},
		{"raw separator in invalid", []model.Line{v177TestInvalid(t, "---", 3, 2)}, []v177Expected{{6, 1, 8, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2)}
			body = append(body, tc.rows...)
			body = append(body, v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2))
			v177TestRun(t, v177TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, raw := range []string{"--", "----", "- - -", `\---`, "\u00a0---"} {
		t.Run("not separator/"+strconv.Quote(raw), func(t *testing.T) {
			value := raw
			if raw == `\---` {
				value = "---"
			}
			result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
				v177TestText(t, raw, value, 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2))
			v177TestRun(t, result, "current", nil)
		})
	}
	t.Run("indented boundaries", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, " \t--- \t", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2),
			v177TestLine(t, " \t} \t", "block-end", 3, 2, v177TestElement(t, "block-close", "}", "}", 3, 4)))
		v177TestRun(t, result, "current", []v177Expected{{6, 3, 8, 4}})
	})
	t.Run("only nested separator", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestOpen(t, "example", 3, 2), v177TestText(t, "Before", "Before", 6, 3), v177TestLine(t, "---", "separator", 6, 3),
			v177TestText(t, "After", "After", 6, 3), v177TestClose(t, 6, 3), v177TestClose(t, 3, 2))
		v177TestError(t, result, 6, -1, "Вложенный example запрещён в ordering.")
		v177TestRun(t, result, "current", nil)
	})
	t.Run("own separator plus nested literal separator", func(t *testing.T) {
		result := v177TestOrdering(t, v177TestOpen(t, "editor", 3, 2), v177TestText(t, "---", "---", 7, 3), v177TestClose(t, 7, 3))
		v177TestError(t, result, 7, -1, "Editor запрещён в ordering.")
		v177TestRun(t, result, "current", []v177Expected{{6, 1, 10, 2}})
	})
	t.Run("missing closing", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2))
		v177TestError(t, result, 3, -1, "Не закрыт ordering.")
		v177TestRun(t, result, "current", nil)
	})
	t.Run("ambiguous closing", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2), v177TestClose(t, 3, 2))
		// Модель разрешает сохранить обе претендующие на закрытие строки.
		v177TestError(t, result, 9, -1, "Неоднозначное закрытие ordering.")
		v177TestRun(t, result, "current", nil)
	})
}

func TestV177Contexts(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		prefix, suffix         []model.Line
		opening, parent, depth int
		want                   []v177Expected
	}{
		{"task", []model.Line{v177TestTag(t, "task", "identifier", "first", 2, 1)}, []model.Line{v177TestTag(t, "endtask", "", "", 2, 1)}, 4, 3, 2, []v177Expected{{7, 1, 9, 2}}},
		{"step", []model.Line{v177TestTag(t, "task", "identifier", "first", 2, 1), v177TestTag(t, "step", "title", "First", 3, 2)}, []model.Line{v177TestTag(t, "endtask", "", "", 2, 1)}, 5, 4, 3, []v177Expected{{8, 1, 10, 2}}},
		{"variant", []model.Line{v177TestOpen(t, "variants", 2, 1), v177TestTag(t, "variant", "name", "A", 3, 2)},
			[]model.Line{v177TestTag(t, "variant", "name", "B", 3, 2), v177TestText(t, "Читайте.", "Читайте.", 11, 3), v177TestClose(t, 3, 2)}, 5, 4, 3, []v177Expected{{8, 1, 10, 2}}},
		{"fragment", []model.Line{v177TestLine(t, "@fragment f {", "block-start", 2, 1, v177TestElement(t, "tag", "@fragment", "fragment", 1, 10),
			v177TestElement(t, "identifier", "f", "f", 11, 12), v177TestElement(t, "block-open", "{", "{", 13, 14))}, []model.Line{v177TestClose(t, 3, 2)}, 4, 3, 2, []v177Expected{{7, 1, 9, 2}}},
		{"unclosed fragment", []model.Line{v177TestLine(t, "@fragment f {", "block-start", 2, 1, v177TestElement(t, "tag", "@fragment", "fragment", 1, 10),
			v177TestElement(t, "identifier", "f", "f", 11, 12), v177TestElement(t, "block-open", "{", "{", 13, 14))}, nil, 4, 3, 2, []v177Expected{{7, 1, 9, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.prefix, v177TestOpen(t, "ordering", tc.parent, tc.depth), v177TestText(t, "A", "A", tc.opening, tc.depth+1),
				v177TestText(t, "B", "B", tc.opening, tc.depth+1), v177TestLine(t, "---", "separator", tc.opening, tc.depth+1),
				v177TestText(t, "First", "First", tc.opening, tc.depth+1), v177TestClose(t, tc.opening, tc.depth+1))
			body = append(body, tc.suffix...)
			result := v177TestDocument(t, body...)
			if strings.Contains(tc.name, "fragment") {
				v177TestError(t, result, 4, -1, "Интерактивный ordering запрещён в fragment; структура сохранена.")
			}
			if tc.name == "unclosed fragment" {
				v177TestError(t, result, 3, -1, "Не закрыт fragment.")
			}
			v177TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("independent blocks", func(t *testing.T) {
		var body []model.Line
		for _, opening := range []int{3, 9} {
			body = append(body, v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", opening, 2), v177TestText(t, "B", "B", opening, 2),
				v177TestLine(t, "---", "separator", opening, 2), v177TestText(t, "First", "First", opening, 2), v177TestClose(t, opening, 2))
		}
		v177TestRun(t, v177TestDocument(t, body...), "current", []v177Expected{{6, 1, 8, 2}, {12, 1, 14, 2}})
	})
	t.Run("matching excluded", func(t *testing.T) {
		result := v177TestDocument(t, v177TestOpen(t, "matching", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2),
			v177TestLine(t, "---", "separator", 3, 2), v177TestText(t, "First", "First", 3, 2), v177TestClose(t, 3, 2))
		v177TestRun(t, result, "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal ordering in "+tag, func(t *testing.T) {
			result := v177TestDocument(t, v177TestOpen(t, tag, 2, 1), v177TestText(t, "@ordering {", "@ordering {", 3, 2),
				v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2), v177TestText(t, "---", "---", 3, 2),
				v177TestText(t, "First", "First", 3, 2), v177TestText(t, `\}`, "}", 3, 2), v177TestClose(t, 3, 2))
			v177TestRun(t, result, "current", nil)
		})
	}
	t.Run("case insensitive tag", func(t *testing.T) {
		result := v177TestOrdering(t, v177TestText(t, "First", "First", 3, 2))
		result.Lines[2].Raw = "@OrDeRiNg {"
		result.Lines[2].Elements[0].Raw = "@OrDeRiNg"
		v177TestRun(t, result, "current", []v177Expected{{6, 1, 8, 2}})
	})
}

func TestV177RegistrationAndHistory(t *testing.T) {
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
		if definition.Code == "V177" {
			count++
			if definition.Severity != "error" || definition.Scope != "block" || definition.Message != "В нижнем списке @ordering меньше двух позиций." || definition.Check == nil ||
				definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
				t.Fatalf("unexpected definition: %+v", definition)
			}
		}
	}
	if count != 1 {
		t.Fatalf("V177 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v177TestOrdering(t)
			result.Document.DSLVersion = &version
			result.Lines[0] = v177TestTag(t, "dsl-version", "version", version, 0, 0)
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
			v177TestRun(t, result, "current", nil)
		})
	}
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v177TestOrdering(t, v177TestText(t, "First", "First", 3, 2))
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v177Expected{{6, 1, 8, 2}}
			v177TestRun(t, result, "first", want)
			v177TestRun(t, result, "second", want)
			v177TestRun(t, v177TestOrdering(t, v177TestText(t, "First", "First", 3, 2), v177TestText(t, "Last", "Last", 3, 2)), "separate", nil)
		})
	}
}

// v177TestOrdering задаёт два верхних элемента и нижний список начиная со строки 7.
func v177TestOrdering(t *testing.T, lower ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v177TestOpen(t, "ordering", 2, 1), v177TestText(t, "A", "A", 3, 2), v177TestText(t, "B", "B", 3, 2), v177TestLine(t, "---", "separator", 3, 2)}
	body = append(body, lower...)
	body = append(body, v177TestClose(t, 3, 2))
	return v177TestDocument(t, body...)
}

// v177TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v177TestDocument.
func v177TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v177TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v177TestLine(t, raw, "invalid", parent, depth, element)
}

// v177TestRun запускает штатный конвейер и проверяет только V177 текущего запуска.
func v177TestRun(t *testing.T, result *model.Result, source string, want []v177Expected) {
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
		if diagnostic.DiagnosticCode == "V177" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V177 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "В нижнем списке @ordering меньше двух позиций."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.endLine, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("ScopeBlock must not create element errorIds")
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

// v177TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v177TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v177TestElement задаёт исходное и семантическое значения без преобразований.
func v177TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v177TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v177TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v177TestTag создаёт однострочное объявление с явно заданным параметром.
func v177TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v177TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v177TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v177TestLine(t, raw, "tag", parent, depth, elements...)
}

// v177TestOpen создаёт объявление блока без параметров.
func v177TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v177TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v177TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v177TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v177TestClose создаёт закрытие с заданным родителем.
func v177TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v177TestLine(t, "}", "block-end", parent, depth, v177TestElement(t, "block-close", "}", "}", 1, 2))
}

// v177TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v177TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v177TestLine(t, raw, "content", parent, depth, v177TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v177TestBlank задаёт обычную пустую строку без текстового элемента.
func v177TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v177TestLine(t, raw, "blank", parent, depth)
}

// v177TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v177TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v177TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v177TestLine(t, "# Lesson", "heading", 0, 0, v177TestElement(t, "heading-level", "#", "1", 1, 2), v177TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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
				v177TestError(t, result, i+1, j, "Строка разобрана не полностью.")
			}
		}
	}
	return result

}

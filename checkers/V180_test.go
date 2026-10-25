package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v180Expected задаёт основной и связанные исходные диапазоны [start,end).
type v180Expected struct {
	line, start, end int
	related          [][3]int
}

func TestV180Registration(t *testing.T) {
	definitions, err := v180Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V180" {
			continue
		}
		count++
		if definition.Message != "Номер позиции содержит символы, отличные от цифр 0–9, или начинается с нуля." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V180 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v180EmptyModel(t, version)
			before := v180Snapshot(t, result)
			checks, err := validator.Prepare(result, v180Registry(t))
			if err == nil || checks != nil || v180Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV180ReferenceAndExample(t *testing.T) {
	// Только префикс версии, идентификатора и урока добавлен перед дословными
	// фрагментами таблицы. ordering начинается на строке 4.
	for _, tc := range []struct {
		name, value, raw string
		want             []v180Expected
	}{
		{"reference", "1", "@ordering {\nA\n@answer 1\nB\n@answer 2\n}", nil},
		{"example", "01", "@ordering {\nA\n@answer 01\nB\n@answer 2\n}", []v180Expected{{6, 9, 11, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v180Base(t, tc.value)
			var raw strings.Builder
			for _, line := range result.Lines[3:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.raw {
				t.Fatalf("source fragment changed: %q", raw.String())
			}
			v180Run(t, result, "current", tc.want)
		})
	}
}

func TestV180NumberSyntax(t *testing.T) {
	for _, value := range []string{"1", "9", "10", "20", "999", "999999999999999999999999999999999999999999999999999999999999", " 1\t", "", " \t", "\u00a0", "\u2003\u00a0"} {
		t.Run("allowed or empty/"+value, func(t *testing.T) { v180Run(t, v180Base(t, value), "current", nil) })
	}
	for _, tc := range []struct {
		value string
		end   int
	}{
		{"0", 10}, {"01", 11}, {"00", 11}, {"+1", 11}, {"-1", 11}, {"1.0", 12}, {"1.", 11}, {"1/2", 12}, {"1e2", 12},
		{"١", 10}, {"１", 10}, {"Ⅰ", 10}, {"1 2", 12}, {"1\t2", 12}, {"1\u00a02", 12}, {"\u00a01", 11}, {"1\u00a0", 11},
		{"01 01", 14}, {"\u200b", 10}, {"a", 10}, {"🙂", 10}, {`\1`, 11},
	} {
		t.Run("invalid/"+tc.value, func(t *testing.T) {
			v180Run(t, v180Base(t, tc.value), "current", []v180Expected{{6, 9, tc.end, nil}})
		})
	}
}

func TestV180BlockValuesAndGrouping(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v180Expected
	}{
		{"valid alternatives", []model.Line{v180TextLine(t, "1", "1", 6, 3), v180TextLine(t, "2", "2", 6, 3)}, nil},
		{"duplicates and distinct value", []model.Line{v180TextLine(t, "01", "01", 6, 3), v180TextLine(t, "01", "01", 6, 3), v180TextLine(t, "+1", "+1", 6, 3)},
			[]v180Expected{{7, 1, 3, [][3]int{{8, 1, 3}}}, {9, 1, 3, nil}}},
		{"three repeats", []model.Line{v180TextLine(t, "01", "01", 6, 3), v180TextLine(t, "01", "01", 6, 3), v180TextLine(t, "01", "01", 6, 3)},
			[]v180Expected{{7, 1, 3, [][3]int{{8, 1, 3}, {9, 1, 3}}}}},
		{"different source edges", []model.Line{v180TextLine(t, "01", "01", 6, 3), v180TextLine(t, " 01\t", "01", 6, 3)},
			[]v180Expected{{7, 1, 3, [][3]int{{8, 2, 4}}}}},
		{"value case remains significant", []model.Line{v180TextLine(t, "A", "A", 6, 3), v180TextLine(t, "a", "a", 6, 3)},
			[]v180Expected{{7, 1, 2, nil}, {8, 1, 2, nil}}},
		{"blank and whitespace-only rows", []model.Line{v180Line(t, "", "blank", 6, 3), v180Line(t, " \t", "blank", 6, 3), v180TextLine(t, "\u00a0", "\u00a0", 6, 3)}, nil},
		{"empty block", nil, nil},
		{"processed values are not decoded twice", []model.Line{v180TextLine(t, `\\\{`, `\{`, 6, 3), v180TextLine(t, `\{`, "{", 6, 3)},
			[]v180Expected{{7, 1, 5, nil}, {8, 1, 3, nil}}},
		{"deeper content excluded", []model.Line{v180TextLine(t, "1", "1", 6, 3), v180Open(t, "hint", "", 6, 3), v180TextLine(t, "01", "01", 8, 4), v180Close(t, 8, 4)}, nil},
		{"nested separator is an answer value", []model.Line{v180TextLine(t, "---", "---", 6, 3)}, []v180Expected{{7, 1, 4, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v180Run(t, v180BlockAnswer(t, tc.rows...), "current", tc.want) })
	}
	t.Run("different answers do not merge", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "B", "B", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180Close(t, 4, 2)), "current", []v180Expected{{6, 9, 11, nil}, {8, 9, 11, nil}})
	})
}

func TestV180RawFallback(t *testing.T) {
	t.Run("single line without content element", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180Line(t, "@answer 01", "tag", 4, 2, v180TestElement(t, "tag", "@answer", "answer", 1, 8)),
			v180TextLine(t, "B", "B", 4, 2), v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2)),
			"current", []v180Expected{{6, 9, 11, nil}})
	})
	for _, tc := range []struct {
		raw, value string
		end        int
	}{
		{`\\`, `\`, 3}, {`\{`, "{", 3}, {`\}`, "}", 3}, {`\@tag`, "@tag", 6}, {`\#`, "#", 3},
		{`\---`, `\---`, 5}, {`\_`, `\_`, 3}, {`\1`, `\1`, 3}, {`\\\{`, `\{`, 5},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			// Raw и Value указаны независимо: первое значение восстановится,
			// второе уже обработано. Правильное декодирование объединит их.
			v180Run(t, v180BlockAnswer(t, v180Line(t, tc.raw, "content", 6, 3), v180TextLine(t, tc.raw, tc.value, 6, 3)), "current",
				[]v180Expected{{7, 1, tc.end, [][3]int{{8, 1, tc.end}}}})
		})
	}
	t.Run("raw numeric text", func(t *testing.T) {
		v180Run(t, v180BlockAnswer(t, v180Line(t, " \t1\t ", "content", 6, 3)), "current", nil)
	})
}

func TestV180UncertainAnswer(t *testing.T) {
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run("single line/"+string(kind), func(t *testing.T) {
			v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
				v180Line(t, `@answer 1\q`, kind, 4, 2, v180TestElement(t, "tag", "@answer", "answer", 1, 8),
					v180TestElement(t, "content", "1", "1", 9, 10), v180Unparsed(t, `\q`, 10, 12)),
				v180TextLine(t, "B", "B", 4, 2), v180TagLine(t, "answer", "content", "01", 4, 2), v180Close(t, 4, 2)),
				"current", []v180Expected{{8, 9, 11, nil}})
		})
	}
	for _, kind := range []model.LineType{"content", "invalid"} {
		for _, first := range []bool{false, true} {
			t.Run(fmt.Sprintf("block %s uncertain first %t", kind, first), func(t *testing.T) {
				bad := v180Line(t, `1\q`, kind, 6, 3, v180TestElement(t, "content", "1", "1", 1, 2), v180Unparsed(t, `\q`, 2, 4))
				rows := []model.Line{v180TextLine(t, "01", "01", 6, 3), bad}
				if first {
					rows[0], rows[1] = rows[1], rows[0]
				}
				v180Run(t, v180BlockAnswer(t, rows...), "current", nil)
			})
		}
	}
	t.Run("unparsed entire value suppresses other values", func(t *testing.T) {
		v180Run(t, v180BlockAnswer(t, v180TextLine(t, "01", "01", 6, 3),
			v180Line(t, "?", "invalid", 6, 3, v180Unparsed(t, "?", 1, 2))), "current", nil)
	})
	t.Run("deeper uncertainty is outside checked values", func(t *testing.T) {
		v180Run(t, v180BlockAnswer(t, v180TextLine(t, "01", "01", 6, 3), v180Open(t, "hint", "", 6, 3),
			v180Line(t, "?", "invalid", 8, 4, v180Unparsed(t, "?", 1, 2)), v180Close(t, 8, 4)), "current", []v180Expected{{7, 1, 3, nil}})
	})
}

func TestV180Adjacency(t *testing.T) {
	for _, tc := range []struct {
		name string
		gap  []model.Line
		want []v180Expected
	}{
		{"empty line", []model.Line{v180Line(t, "", "blank", 4, 2)}, []v180Expected{{7, 9, 11, nil}}},
		{"space and tab line", []model.Line{v180Line(t, " \t", "blank", 4, 2)}, []v180Expected{{7, 9, 11, nil}}},
		{"editor is not skipped", []model.Line{v180TagLine(t, "editor", "content", "Comment.", 4, 2)}, nil},
		{"hint breaks adjacency", []model.Line{v180TagLine(t, "hint", "content", "Think.", 4, 2)}, nil},
		{"repeat answer", []model.Line{v180TagLine(t, "answer", "content", "1", 4, 2)}, nil},
		{"block editor is a boundary", []model.Line{v180Open(t, "editor", "", 4, 2), v180TextLine(t, "Comment.", "Comment.", 6, 3), v180Close(t, 6, 3)}, nil},
		{"block answer is a boundary", []model.Line{v180Open(t, "answer", "", 4, 2), v180TextLine(t, "1", "1", 6, 3), v180Close(t, 6, 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := append([]model.Line{v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2)}, tc.gap...)
			rows = append(rows, v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "B", "B", 4, 2),
				v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2))
			v180Run(t, v180Document(t, rows...), "current", tc.want)
		})
	}
	t.Run("answer before any item", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TagLine(t, "answer", "content", "01", 4, 2),
			v180TextLine(t, "A", "A", 4, 2), v180TextLine(t, "B", "B", 4, 2), v180Close(t, 4, 2)), "current", nil)
	})
	t.Run("escaped separator is an item", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, `\---`, "---", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "B", "B", 4, 2),
			v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2)), "current", []v180Expected{{6, 9, 11, nil}})
	})
}

func TestV180FormatsAndContext(t *testing.T) {
	t.Run("expanded and short blocks are independent", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "B", "B", 4, 2),
			v180TagLine(t, "answer", "content", "2", 4, 2), v180Line(t, "---", "separator", 4, 2),
			v180TextLine(t, "01", "01", 4, 2), v180TextLine(t, "2", "2", 4, 2), v180Close(t, 4, 2),
			v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 13, 2),
			v180TagLine(t, "answer", "content", "01", 13, 2), v180TextLine(t, "B", "B", 13, 2),
			v180TagLine(t, "answer", "content", "2", 13, 2), v180Close(t, 13, 2)),
			"current", []v180Expected{{15, 9, 11, nil}})
	})
	for _, delimiter := range []model.Line{
		v180Line(t, "---", "separator", 4, 2), v180Line(t, " \t---\t ", "separator", 4, 2),
		v180Line(t, "---", "invalid", 4, 2, v180Unparsed(t, "---", 1, 4)),
	} {
		t.Run("expanded/"+string(delimiter.LineType)+delimiter.Raw, func(t *testing.T) {
			v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
				v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "B", "B", 4, 2),
				v180TagLine(t, "answer", "content", "1.", 4, 2), delimiter, v180TextLine(t, "01", "01", 4, 2),
				v180TextLine(t, "1.", "1.", 4, 2), v180Close(t, 4, 2)), "current", nil)
		})
	}
	t.Run("two separators also exclude check", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "B", "B", 4, 2),
			v180TagLine(t, "answer", "content", "2", 4, 2), v180Line(t, "---", "separator", 4, 2),
			v180TextLine(t, "01", "01", 4, 2), v180Line(t, "---", "separator", 4, 2), v180TextLine(t, "2", "2", 4, 2), v180Close(t, 4, 2)), "current", nil)
	})
	t.Run("nested separator does not change format", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180Open(t, "hint", "", 4, 2),
			v180TextLine(t, "---", "---", 7, 3), v180Close(t, 7, 3), v180TextLine(t, "B", "B", 4, 2),
			v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2)), "current", []v180Expected{{6, 9, 11, nil}})
	})
	t.Run("NBSP around dashes is not separator", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180TagLine(t, "answer", "content", "01", 4, 2), v180TextLine(t, "\u00a0---", "\u00a0---", 4, 2),
			v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2)), "current", []v180Expected{{6, 9, 11, nil}})
	})
	t.Run("question answer outside ordering", func(t *testing.T) {
		v180Run(t, v180Document(t, v180TagLine(t, "question", "content", "Where?", 3, 1),
			v180TagLine(t, "answer", "content", "01", 3, 1)), "current", nil)
	})
	t.Run("literal ordering inside editor", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "editor", "", 3, 1), v180TextLine(t, "@ordering", "@ordering", 4, 2),
			v180TextLine(t, "A", "A", 4, 2), v180TextLine(t, "@answer 01", "@answer 01", 4, 2), v180Close(t, 4, 2)), "current", nil)
	})
	t.Run("ordering in a task stage", func(t *testing.T) {
		v180Run(t, v180Document(t, v180TagLine(t, "task", "identifier", "1", 3, 1), v180TagLine(t, "step", "title", "Order", 4, 2),
			v180Open(t, "ordering", "", 5, 3), v180TextLine(t, "A", "A", 6, 4), v180TagLine(t, "answer", "content", "01", 6, 4),
			v180TextLine(t, "B", "B", 6, 4), v180TagLine(t, "answer", "content", "2", 6, 4), v180Close(t, 6, 4),
			v180TagLine(t, "endtask", "", "", 3, 1)), "current", []v180Expected{{8, 9, 11, nil}})
	})
}

func TestV180SourceBoundaries(t *testing.T) {
	t.Run("empty input", func(t *testing.T) { v180Run(t, v180EmptyModel(t, "1.2"), "current", nil) })
	t.Run("case indentation trailing whitespace", func(t *testing.T) {
		v180Run(t, v180Document(t, v180Open(t, "OrDeRiNg", "Order.", 3, 1), v180TextLine(t, "A", "A", 4, 2),
			v180Line(t, " \t@AnSwEr 01\t", "tag", 4, 2, v180TestElement(t, "tag", "@AnSwEr", "answer", 3, 10),
				v180TestElement(t, "content", "01", "01", 11, 13)),
			v180TextLine(t, "B", "B", 4, 2), v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2)),
			"current", []v180Expected{{6, 11, 13, nil}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v180Base(t, "01")
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v180Run(t, result, "current", []v180Expected{{6, 9, 11, nil}})
			})
		}
	}
	t.Run("history and related locations survive", func(t *testing.T) {
		result := v180BlockAnswer(t, v180TextLine(t, "01", "01", 6, 3), v180TextLine(t, "01", "01", 6, 3))
		v180Run(t, result, "first", []v180Expected{{7, 1, 3, [][3]int{{8, 1, 3}}}})
		v180Run(t, result, "second", []v180Expected{{7, 1, 3, [][3]int{{8, 1, 3}}}})
	})
}

// v180Base создаёт эталон или пример с явно заданным первым ответом.
func v180Base(t *testing.T, value string) *model.Result {
	t.Helper()
	return v180Document(t, v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2),
		v180TagLine(t, "answer", "content", value, 4, 2), v180TextLine(t, "B", "B", 4, 2),
		v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2))
}

// v180BlockAnswer обрамляет явные строки блоком answer на строке 6.
func v180BlockAnswer(t *testing.T, values ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{v180Open(t, "ordering", "", 3, 1), v180TextLine(t, "A", "A", 4, 2), v180Open(t, "answer", "", 4, 2)}, values...)
	rows = append(rows, v180Close(t, 6, 3), v180TextLine(t, "B", "B", 4, 2), v180TagLine(t, "answer", "content", "2", 4, 2), v180Close(t, 4, 2))
	return v180Document(t, rows...)
}

// v180Registry создаёт реестр из полного штатного каталога регистраций.
func v180Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v180EmptyModel создаёт модель без исходных строк и с явно указанной версией.
func v180EmptyModel(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v180Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v180Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v180EmptyModel(t, "1.2")
	title, id := "Lesson", "v180"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v180TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v180TagLine(t, "document-id", "identifier", "v180", 0, 0),
		v180Line(t, "# Lesson", "heading", 0, 0, v180TestElement(t, "heading-level", "#", "1", 1, 2), v180TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			errorID := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{errorID}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: errorID, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v180Line сохраняет явно заданные данные физической строки и её родителя.
func v180Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v180TestElement задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v180TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v180Unparsed задаёт фрагмент без значения; историю добавляет v180Document.
func v180Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v180TagLine создаёт объявление с явно заданным типом аргумента.
func v180TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v180TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v180TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v180Line(t, raw, "tag", parent, depth, elements...)
}

// v180TextLine задаёт обычную строку с независимо указанным семантическим значением.
func v180TextLine(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v180Line(t, raw, "content", parent, depth, v180TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v180Open задаёт блочное объявление с необязательной инструкцией.
func v180Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v180TagLine(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v180TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v180Close задаёт отдельную закрывающую скобку с явным родителем.
func v180Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v180Line(t, "}", "block-end", parent, depth, v180TestElement(t, "block-close", "}", "}", 1, 2))
}

// v180Run запускает неизменённую выборку штатного каталога и проверяет новые V180.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v180Run(t *testing.T, result *model.Result, source string, want []v180Expected) {
	t.Helper()
	before := v180Snapshot(t, result)
	checks, err := validator.Prepare(result, v180Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v180Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("unexpected new source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V180" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V180 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Номер позиции содержит символы, отличные от цифр 0–9, или начинается с нуля." {
			t.Errorf("occurrence = %+v, want range %+v and exact V180 metadata", entry, location)
		}
		var related []model.Location
		for _, span := range expected.related {
			related = append(related, model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[0], Column: span[2]}})
		}
		if !slices.Equal(entry.RelatedLocations, related) {
			t.Errorf("related = %+v, want %+v", entry.RelatedLocations, related)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Error("line diagnostic must not create element links")
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v180Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Lines) != len(original.Lines) || len(actual.Processing) != oldProcessing+1 {
		t.Fatal("source or history length changed")
	}
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		if len(actual.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v180Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v180Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v180Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

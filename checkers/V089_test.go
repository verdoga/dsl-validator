package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV089Registration(t *testing.T) {
	definitions, err := v089Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V089" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "После разделителя --- отсутствует содержимое." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V089 definitions = %d, want 1", count)
	}
}

// v089Expected содержит независимые ожидания: строка и колонки маркера,
// затем строка и колонки связанной закрывающей скобки; концы исключаются.
type v089Expected struct {
	line, start, end                      int
	relatedLine, relatedStart, relatedEnd int
}

func TestV089ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           []v089Expected
	}{
		{"reference", "@example {\nHe go.\n---\nHe goes.\n}", []model.Line{
			v089Text(t, "He go.", "He go.", 3, 2), v089Separator(t, 3, 2), v089Text(t, "He goes.", "He goes.", 3, 2),
		}, nil},
		{"example", "@example {\nHe go.\n---\n}", []model.Line{
			v089Text(t, "He go.", "He go.", 3, 2), v089Separator(t, 3, 2),
		}, []v089Expected{{5, 1, 4, 6, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v089Block(t, "example", tc.rows...)
			// Только обязательный контекст добавлен перед дословным фрагментом.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v089Run(t, result, "current", tc.want)
		})
	}
}

func TestV089ImmediateContent(t *testing.T) {
	for _, tag := range []string{"example", "table"} {
		for _, tc := range []struct {
			name string
			rows []model.Line
			want []v089Expected
		}{
			{"immediate closing", nil, []v089Expected{{5, 1, 4, 6, 1, 2}}},
			{"blank rows", []model.Line{v089Line(t, "", "blank", 3, 2), v089Line(t, " \t", "blank", 3, 2)},
				[]v089Expected{{5, 1, 4, 8, 1, 2}}},
			{"ordinary content", []model.Line{v089Text(t, "After.", "After.", 3, 2)}, nil},
			{"Unicode content", []model.Line{v089Text(t, "Ответ 😀", "Ответ 😀", 3, 2)}, nil},
			{"NBSP is not an ASCII blank", []model.Line{v089Text(t, "\u00a0", "\u00a0", 3, 2)}, nil},
			{"escaped separator is content", []model.Line{v089Text(t, "\\---", "---", 3, 2)}, nil},
			{"content after blanks", []model.Line{v089Line(t, " \t", "blank", 3, 2), v089Text(t, "After.", "After.", 3, 2)}, nil},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) {
				rows := append([]model.Line{v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2)}, tc.rows...)
				v089Run(t, v089Block(t, tag, rows...), "current", tc.want)
			})
		}
	}
	for _, tc := range []struct{ header, row string }{
		{"A | B", "|"}, {"A | B | C", "||"}, {"A | B", " \t|\t "}, {"A", "Value"},
	} {
		t.Run("table cells/"+strconv.Quote(tc.row), func(t *testing.T) {
			v089Run(t, v089Block(t, "table", v089Text(t, tc.header, tc.header, 3, 2),
				v089Separator(t, 3, 2), v089Text(t, tc.row, strings.Trim(tc.row, " \t"), 3, 2)), "current", nil)
		})
	}
	for _, title := range []string{"Title", "Название 😀"} {
		t.Run("title does not fill body/"+title, func(t *testing.T) {
			result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
			result.Lines[2] = v089Open(t, "example", title, 2, 1)
			result.Lines[2].Number = 3
			v089Run(t, result, "current", []v089Expected{{5, 1, 4, 6, 1, 2}})
		})
	}
}

func TestV089SeparatorRecognition(t *testing.T) {
	for _, tag := range []string{"example", "table"} {
		for _, tc := range []struct {
			name string
			rows []model.Line
			want []v089Expected
		}{
			{"none", nil, nil},
			{"one", []model.Line{v089Separator(t, 3, 2)}, []v089Expected{{5, 1, 4, 6, 1, 2}}},
			{"two", []model.Line{v089Separator(t, 3, 2), v089Separator(t, 3, 2)}, nil},
			{"three", []model.Line{v089Separator(t, 3, 2), v089Separator(t, 3, 2), v089Separator(t, 3, 2)}, nil},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) {
				rows := append([]model.Line{v089Text(t, "Before.", "Before.", 3, 2)}, tc.rows...)
				v089Run(t, v089Block(t, tag, rows...), "current", tc.want)
			})
		}
	}
	for _, tc := range []struct{ raw, value string }{
		{"\\---", "---"}, {"----", "----"}, {"x---", "x---"}, {"---x", "---x"},
		{"\u00a0---\u00a0", "\u00a0---\u00a0"}, {"\u2003---", "\u2003---"}, {"---\u2003", "---\u2003"},
	} {
		t.Run("not a separator/"+strconv.Quote(tc.raw), func(t *testing.T) {
			v089Run(t, v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2),
				v089Text(t, tc.raw, tc.value, 3, 2)), "current", nil)
		})
	}
	t.Run("indented marker and brace", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
			v089Line(t, " \t---\t ", "separator", 3, 2),
			v089Line(t, "  } \t", "block-end", 3, 2, v089TestElement(t, "block-close", "}", "}", 3, 4)))
		v089Run(t, result, "current", []v089Expected{{5, 3, 6, 6, 3, 4}})
	})
	t.Run("mixed case canonical tag", func(t *testing.T) {
		result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@ExAmPlE {", "@ExAmPlE"
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 6, 1, 2}})
	})
}

func TestV089NestedBlocks(t *testing.T) {
	for _, tag := range []string{"example", "table"} {
		t.Run("inside fragment/"+tag, func(t *testing.T) {
			result := v089Lesson(t, v089Open(t, "fragment", "f", 2, 1), v089Open(t, tag, "", 3, 2),
				v089Text(t, "Before.", "Before.", 4, 3), v089Separator(t, 4, 3), v089Close(t, 4, 3), v089Close(t, 3, 2))
			v089Run(t, result, "current", []v089Expected{{6, 1, 4, 7, 1, 2}})
		})
		t.Run("nested material excluded/"+tag, func(t *testing.T) {
			result := v089Lesson(t, v089Open(t, tag, "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
				v089Separator(t, 3, 2), v089Open(t, "note", "", 3, 2), v089Text(t, "Nested.", "Nested.", 6, 3),
				v089Close(t, 6, 3), v089Close(t, 3, 2))
			// Запрещённое вложение отражено самостоятельной ошибкой; V089
			// по условию не считает вложенный материал непосредственным.
			v089ParserError(t, result, 6, 6, "line")
			v089Run(t, result, "current", []v089Expected{{5, 1, 4, 9, 1, 2}})
		})
	}
	t.Run("nested separator does not belong to outer block", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Open(t, "example", "", 3, 2),
			v089Text(t, "Before.", "Before.", 4, 3), v089Separator(t, 4, 3), v089Text(t, "After.", "After.", 4, 3),
			v089Close(t, 4, 3), v089Close(t, 3, 2))
		v089ParserError(t, result, 4, 9, "line")
		v089Run(t, result, "current", nil)
	})
	t.Run("each block has its own separator", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
			v089Separator(t, 3, 2), v089Open(t, "example", "", 3, 2), v089Text(t, "Inner.", "Inner.", 6, 3),
			v089Separator(t, 6, 3), v089Close(t, 6, 3), v089Close(t, 3, 2))
		v089ParserError(t, result, 6, 9, "line")
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 10, 1, 2}, {8, 1, 4, 9, 1, 2}})
	})
	t.Run("single line nested content excluded", func(t *testing.T) {
		result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2),
			v089Line(t, "@note Nested.", "tag", 3, 2, v089TestElement(t, "tag", "@note", "note", 1, 6),
				v089TestElement(t, "content", "Nested.", "Nested.", 7, 14)))
		v089ParserError(t, result, 6, 6, "line")
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 7, 1, 2}})
	})
	t.Run("siblings do not fill each other", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
			v089Separator(t, 3, 2), v089Close(t, 3, 2), v089Open(t, "table", "", 2, 1),
			v089Text(t, "Header", "Header", 7, 2), v089Separator(t, 7, 2), v089Close(t, 7, 2),
			v089Text(t, "Outside.", "Outside.", 2, 1))
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 6, 1, 2}, {9, 1, 4, 10, 1, 2}})
	})
}

func TestV089UnparsedAndBoundaries(t *testing.T) {
	for _, kind := range []model.LineType{"invalid", "content", "tag"} {
		t.Run("unknown direct content/"+string(kind), func(t *testing.T) {
			result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2),
				v089Unparsed(t, "@unknown ???", kind, 3, 2))
			v089Run(t, result, "current", nil)
		})
	}
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v089Expected
	}{
		{"invalid before separator", []model.Line{v089Unparsed(t, "@unknown", "invalid", 3, 2), v089Separator(t, 3, 2)},
			[]v089Expected{{5, 1, 4, 6, 1, 2}}},
		{"raw separator in invalid line", []model.Line{v089Text(t, "Before.", "Before.", 3, 2), v089Unparsed(t, "---", "invalid", 3, 2)},
			[]v089Expected{{5, 1, 4, 6, 1, 2}}},
		{"raw separator in unparsed content", []model.Line{v089Text(t, "Before.", "Before.", 3, 2), v089Unparsed(t, "---", "content", 3, 2)},
			[]v089Expected{{5, 1, 4, 6, 1, 2}}},
		{"second raw separator in invalid line", []model.Line{v089Text(t, "Before.", "Before.", 3, 2),
			v089Separator(t, 3, 2), v089Unparsed(t, "---", "invalid", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v089Run(t, v089Block(t, "example", tc.rows...), "current", tc.want) })
	}
	t.Run("unparsed descendant does not suppress outer check", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
			v089Separator(t, 3, 2), v089Open(t, "note", "", 3, 2), v089Unparsed(t, "@unknown", "invalid", 6, 3),
			v089Close(t, 6, 3), v089Close(t, 3, 2))
		v089ParserError(t, result, 6, 6, "line")
		// Контракт требует исходные диагностики в порядке позиций:
		// ошибка вложения на строке 6 предшествует unparsed на строке 7.
		result.Diagnostics[0], result.Diagnostics[1] = result.Diagnostics[1], result.Diagnostics[0]
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 9, 1, 2}})
	})
	t.Run("unparsed outside does not suppress check", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
			v089Separator(t, 3, 2), v089Close(t, 3, 2), v089Unparsed(t, "@unknown", "invalid", 2, 1))
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 6, 1, 2}})
	})
	t.Run("missing closing", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
		v089ParserError(t, result, 3, 11, "block")
		v089Run(t, result, "current", nil)
	})
	t.Run("ambiguous closing", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "example", "", 2, 1), v089Text(t, "Before.", "Before.", 3, 2),
			v089Separator(t, 3, 2), v089Close(t, 3, 2), v089Close(t, 3, 2))
		// CheckModel допускает несколько block-end с одним родителем.
		v089ParserError(t, result, 7, 2, "line")
		v089Run(t, result, "current", nil)
	})
	t.Run("closed block inside unclosed fragment", func(t *testing.T) {
		result := v089Lesson(t, v089Open(t, "fragment", "f", 2, 1), v089Open(t, "example", "", 3, 2),
			v089Text(t, "Before.", "Before.", 4, 3), v089Separator(t, 4, 3), v089Close(t, 4, 3))
		v089ParserError(t, result, 3, 14, "block")
		v089Run(t, result, "current", []v089Expected{{6, 1, 4, 7, 1, 2}})
	})
}

func TestV089ExclusionsAndLifecycle(t *testing.T) {
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo blocks/"+tag, func(t *testing.T) {
			// В литеральной области строки псевдоблока не являются block-start/end.
			result := v089Block(t, tag, v089Text(t, "@example {", "@example {", 3, 2),
				v089Text(t, "Before.", "Before.", 3, 2), v089Text(t, "---", "---", 3, 2),
				v089Text(t, "\\}", "}", 3, 2))
			v089Run(t, result, "current", nil)
		})
	}
	for _, tag := range []string{"note", "text", "wordlist"} {
		t.Run("other block/"+tag, func(t *testing.T) {
			v089Run(t, v089Block(t, tag, v089Text(t, "Before.", "Before.", 3, 2), v089Text(t, "---", "---", 3, 2)), "current", nil)
		})
	}
	t.Run("empty input", func(t *testing.T) {
		result := v089Lesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v089Run(t, result, "current", nil)
	})
	t.Run("first block after required context", func(t *testing.T) {
		result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
		v089Run(t, result, "current", []v089Expected{{5, 1, 4, 6, 1, 2}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v089Run(t, result, "current", []v089Expected{{5, 1, 4, 6, 1, 2}})
			})
		}
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v089Snapshot(t, result)
			if checks, err := validator.Prepare(result, v089Registry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v089Snapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v089Block(t, "example", v089Text(t, "Before.", "Before.", 3, 2), v089Separator(t, 3, 2))
		second := v089Block(t, "table", v089Text(t, "Header", "Header", 3, 2), v089Separator(t, 3, 2), v089Text(t, "Row", "Row", 3, 2))
		v089Run(t, first, "first", []v089Expected{{5, 1, 4, 6, 1, 2}})
		v089Run(t, second, "first", nil)
		v089Run(t, first, "second", []v089Expected{{5, 1, 4, 6, 1, 2}})
	})
}

// v089Block окружает явно заданные непосредственные строки полноценным блоком.
func v089Block(t *testing.T, tag string, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v089Open(t, tag, "", 2, 1)}, rows...)
	body = append(body, v089Close(t, 3, 2))
	return v089Lesson(t, body...)
}

// v089Lesson добавляет обязательный контекст, нумерацию и ошибки unparsed.
// Родители, глубина, типы строк и элементов заданы тестом, разбора DSL здесь нет.
func v089Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v089Line(t, "@dsl-version 1.2", "tag", 0, 0, v089TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v089TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v089Line(t, "# Lesson", "heading", 0, 0, v089TestElement(t, "heading-level", "#", "1", 1, 2),
			v089TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "unparsed" {
				id := v089ParserError(t, result, line.Number, element.End, "element")
				element.Value, element.ErrorIDs = nil, []string{id}
			}
		}
	}
	return result
}

// v089ParserError сохраняет самостоятельную ошибку входного документа.
func v089ParserError(t *testing.T, result *model.Result, line, end int, scope diagnostics.Scope) string {
	t.Helper()
	id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Document.HasErrors, result.Lines[line-1].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: scope, Message: "Ошибка разбора исходного документа.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: line, Column: 1}, End: model.Position{Line: line, Column: end}}})
	return id
}

// v089Open задаёт объявление с названием либо ID для fragment.
func v089Open(t *testing.T, tag, argument string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v089TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if argument != "" {
		kind := model.ElementTypeTitle
		if tag == "fragment" {
			kind = model.ElementTypeIdentifier
		}
		start := len(raw) + 2
		elements = append(elements, v089TestElement(t, kind, argument, argument, start, start+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	raw += " {"
	column := utf8.RuneCountInString(raw)
	elements = append(elements, v089TestElement(t, "block-open", "{", "{", column, column+1))
	return v089Line(t, raw, "block-start", parent, depth, elements...)
}

func v089Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v089Line(t, "}", "block-end", parent, depth, v089TestElement(t, "block-close", "}", "}", 1, 2))
}

func v089Separator(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v089Line(t, "---", "separator", parent, depth)
}

func v089Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v089Line(t, raw, "content", parent, depth, v089TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v089Unparsed(t *testing.T, raw string, kind model.LineType, parent, depth int) model.Line {
	t.Helper()
	return v089Line(t, raw, kind, parent, depth, v089TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v089Line сохраняет переданную структуру и исходный текст без классификации.
func v089Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v089TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v089TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

func v089Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v089Run запускает полный штатный конвейер, проверяя только V089 этого запуска.
// Ожидания диапазонов и метаданных не зависят от функций реализации.
func v089Run(t *testing.T, result *model.Result, source string, want []v089Expected) {
	t.Helper()
	before := v089Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v089Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v089Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V089" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V089 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		related := model.Location{Start: model.Position{Line: position.relatedLine, Column: position.relatedStart}, End: model.Position{Line: position.relatedLine, Column: position.relatedEnd}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "После разделителя --- отсутствует содержимое." {
			t.Fatalf("occurrence %d = %+v; want %+v related %+v and exact V089 metadata", i, entry, location, related)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v089Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Производные флаги и ссылки проверены CheckModel; снимок сравнивает
	// исходные данные и всю историю до текущего запуска.
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
	if v089Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v089Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

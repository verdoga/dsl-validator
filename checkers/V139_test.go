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

const (
	v139DetailTwo   = "Количество строк ответа: 2. Оставьте ровно одну строку ответа."
	v139DetailThree = "Количество строк ответа: 3. Оставьте ровно одну строку ответа."
)

type v139Expected struct {
	main    [4]int
	related [][3]int
	detail  string
}

func TestV139Registration(t *testing.T) {
	registry := v139Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V139" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "Блок @answer внутри @choice содержит несколько строк ответа." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V139 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted: %+v, %v", version, definitions, err)
		}
	}
}

func TestV139ReferenceAndExample(t *testing.T) {
	choice := v139Line(t, "@choice Choose a country. {", "block-start", 2, 1,
		v139Element(t, "tag", "@choice", "choice", 1), v139Element(t, "content", "Choose a country.", "Choose a country.", 9),
		v139Element(t, "block-open", "{", "{", 27))
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v139Expected
	}{
		{"reference", "@choice Choose a country. {\n@answer Italy\nFrance\n}",
			[]model.Line{choice, v139Inline(t, "answer", "Italy", 3, 2), v139Text(t, "France", "France", 3, 2), v139Close(t, 3, 2)}, nil},
		{"example", "@choice Choose a country. {\n@answer {\nItaly\nSpain\n}\nFrance\n}",
			[]model.Line{choice, v139Open(t, "answer", 3, 2), v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3),
				v139Close(t, 4, 3), v139Text(t, "France", "France", 3, 2), v139Close(t, 3, 2)},
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v139Lesson(t, tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v139Run(t, result, "current", tc.want)
		})
	}
}

func TestV139CountingAndValues(t *testing.T) {
	text := func(raw, value string) model.Line { t.Helper(); return v139Text(t, raw, value, 4, 3) }
	blank := func(raw string) model.Line { t.Helper(); return v139Line(t, raw, "blank", 4, 3) }
	for _, tc := range []struct {
		name    string
		rows    []model.Line
		missing bool
		want    []v139Expected
	}{
		{"zero", nil, true, nil},
		{"blank only", []model.Line{blank(""), blank(" \t")}, true, nil},
		{"one", []model.Line{text("Italy", "Italy")}, false, nil},
		{"one surrounded by blanks", []model.Line{blank(""), text("Italy", "Italy"), blank(" \t")}, false, nil},
		{"two", []model.Line{text("Italy", "Italy"), text("Spain", "Spain")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}}},
		{"three", []model.Line{text("Italy", "Italy"), text("Spain", "Spain"), text("Germany", "Germany")}, false,
			[]v139Expected{{[4]int{4, 1, 8, 2}, [][3]int{{6, 1, 6}, {7, 1, 8}}, v139DetailThree}}},
		{"duplicates count", []model.Line{text("Italy", "Italy"), text("Italy", "Italy")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}}},
		{"case does not matter", []model.Line{text("Italy", "Italy"), text("ITALY", "ITALY")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}}},
		{"case and punctuation do not merge rows", []model.Line{text("Yes", "Yes"), text("YES!", "YES!")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 5}}, v139DetailTwo}}},
		{"blank between", []model.Line{text("Italy", "Italy"), blank(""), text("Spain", "Spain")}, false,
			[]v139Expected{{[4]int{4, 1, 8, 2}, [][3]int{{7, 1, 6}}, v139DetailTwo}}},
		{"mixed Unicode whitespace does not count", []model.Line{text("\u00a0\u2003", "\u00a0\u2003"), text("Yes", "Yes"),
			text("\u202f\u3000", "\u202f\u3000"), text("\tYES! \u2003", "YES! \u2003"), text("\u1680\u205f", "\u1680\u205f")}, false,
			[]v139Expected{{[4]int{4, 1, 10, 2}, [][3]int{{8, 1, 8}}, v139DetailTwo}}},
		{"punctuation-only rows count separately", []model.Line{text("!", "!"), text("?", "?")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 2}}, v139DetailTwo}}},
		{"punctuation and emoji", []model.Line{text("!", "!"), text("😀", "😀")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 2}}, v139DetailTwo}}},
		{"combining mark and zero width space", []model.Line{text("\u0301", "\u0301"), text("\u200b", "\u200b")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 2}}, v139DetailTwo}}},
		{"Cyrillic and supplementary characters", []model.Line{text("Италия", "Италия"), text("𐐀😀", "𐐀😀")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 3}}, v139DetailTwo}}},
		{"no entity decoding", []model.Line{text("Italy", "Italy"), text("&#32;", "&#32;")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}}},
		{"no HTML removal", []model.Line{text("<br>", "<br>"), text("<em></em>", "<em></em>")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 10}}, v139DetailTwo}}},
		{"entire related line", []model.Line{text("Italy", "Italy"), text(" \tSpain \u00a0", "Spain \u00a0")}, false,
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 10}}, v139DetailTwo}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v139Block(t, tc.rows...)
			if tc.missing {
				v139ParserError(t, result, 4, "Нет непустого значения ответа.")
			}
			v139Run(t, result, "current", tc.want)
		})
	}
	for _, tc := range []struct {
		raw, value string
		end        int
	}{
		{`\{`, "{", 3}, {`\}`, "}", 3}, {`\\`, `\`, 3}, {`\@answer`, "@answer", 9},
		{`\#Title`, "#Title", 8}, {`\q`, `\q`, 3}, {"\\\\ \t", "\\", 5},
	} {
		t.Run("escape/"+strconv.Quote(tc.raw), func(t *testing.T) {
			v139Run(t, v139Block(t, text("Italy", "Italy"), text(tc.raw, tc.value)), "current",
				[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, tc.end}}, v139DetailTwo}})
		})
	}
	t.Run("several content elements are one line", func(t *testing.T) {
		row := v139Line(t, "It\\{aly", "content", 4, 3,
			v139Element(t, "content", "It", "It", 1), v139Element(t, "content", `\{`, "{", 3), v139Element(t, "content", "aly", "aly", 5))
		v139Run(t, v139Block(t, row), "single", nil)
		v139Run(t, v139Block(t, row, text("Spain", "Spain")), "two", []v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}})
	})
}

func TestV139UnicodeWhitespace(t *testing.T) {
	// White_Space перечислен независимо от проверяемого кода; CR/LF — это EOL.
	spaces := []string{" ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"}
	for _, space := range spaces {
		t.Run(strconv.Quote(space), func(t *testing.T) {
			row := v139Text(t, space, space, 4, 3)
			if space == " " || space == "\t" {
				row = v139Line(t, space, "blank", 4, 3)
			}
			empty := v139Block(t, row, row)
			v139ParserError(t, empty, 4, "Нет непустого значения ответа.")
			v139Run(t, empty, "empty", nil)
			v139Run(t, v139Block(t, row, v139Text(t, "Italy", "Italy", 4, 3), row), "one", nil)
			v139Run(t, v139Block(t, v139Text(t, "Italy", "Italy", 4, 3), row, v139Text(t, "Spain", "Spain", 4, 3)), "two",
				[]v139Expected{{[4]int{4, 1, 8, 2}, [][3]int{{7, 1, 6}}, v139DetailTwo}})
		})
	}
}

func TestV139Contexts(t *testing.T) {
	t.Run("nested constructions do not count", func(t *testing.T) {
		for _, after := range []bool{false, true} {
			t.Run("outer text after nesting="+strconv.FormatBool(after), func(t *testing.T) {
				rows := []model.Line{v139Text(t, "Italy", "Italy", 4, 3), v139Open(t, "hint", 4, 3),
					v139Text(t, "Inside one.", "Inside one.", 6, 4), v139Text(t, "Inside two.", "Inside two.", 6, 4), v139Close(t, 6, 4)}
				var want []v139Expected
				if after {
					rows = append(rows, v139Text(t, "Spain", "Spain", 4, 3))
					want = []v139Expected{{[4]int{4, 1, 11, 2}, [][3]int{{10, 1, 6}}, v139DetailTwo}}
				}
				result := v139Block(t, rows...)
				v139ParserError(t, result, 6, "Вложенный hint запрещён в answer.")
				v139Run(t, result, "current", want)
			})
		}
	})
	t.Run("inline nested content is not an answer row", func(t *testing.T) {
		result := v139Block(t, v139Text(t, "Italy", "Italy", 4, 3), v139Inline(t, "hint", "Extra text.", 4, 3))
		v139ParserError(t, result, 6, "Вложенный hint запрещён в answer.")
		v139Run(t, result, "current", nil)
	})
	t.Run("question", func(t *testing.T) {
		v139Run(t, v139Lesson(t, v139Inline(t, "question", "Name a country.", 2, 1), v139Open(t, "answer", 2, 1),
			v139Text(t, "Italy", "Italy", 4, 2), v139Text(t, "Spain", "Spain", 4, 2), v139Close(t, 4, 2)), "current", nil)
	})
	t.Run("multichoice", func(t *testing.T) {
		v139Run(t, v139Lesson(t, v139Open(t, "multichoice", 2, 1), v139Open(t, "answer", 3, 2),
			v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3), v139Close(t, 4, 3),
			v139Text(t, "Canada", "Canada", 3, 2), v139Close(t, 3, 2)), "current", nil)
	})
	t.Run("matching", func(t *testing.T) {
		v139Run(t, v139Lesson(t, v139Open(t, "matching", 2, 1), v139Text(t, "European country", "European country", 3, 2),
			v139Open(t, "answer", 3, 2), v139Text(t, "Italy", "Italy", 5, 3), v139Text(t, "Spain", "Spain", 5, 3), v139Close(t, 5, 3),
			v139Line(t, "---", "separator", 3, 2), v139Text(t, "Italy", "Italy", 3, 2), v139Text(t, "Spain", "Spain", 3, 2), v139Close(t, 3, 2)), "current", nil)
	})
	t.Run("ordering", func(t *testing.T) {
		v139Run(t, v139Lesson(t, v139Open(t, "ordering", 2, 1), v139Text(t, "First item", "First item", 3, 2),
			v139Open(t, "answer", 3, 2), v139Text(t, "1", "1", 5, 3), v139Text(t, "2", "2", 5, 3), v139Close(t, 5, 3),
			v139Text(t, "Second item", "Second item", 3, 2), v139Inline(t, "answer", "2", 3, 2), v139Close(t, 3, 2)), "current", nil)
	})
	t.Run("farther choice does not override nearest block", func(t *testing.T) {
		result := v139Lesson(t, v139Open(t, "choice", 2, 1), v139Open(t, "multichoice", 3, 2), v139Open(t, "answer", 4, 3),
			v139Text(t, "Italy", "Italy", 5, 4), v139Text(t, "Spain", "Spain", 5, 4), v139Close(t, 5, 4), v139Close(t, 4, 3),
			v139Text(t, "France", "France", 3, 2), v139Text(t, "Canada", "Canada", 3, 2), v139Close(t, 3, 2))
		v139ParserError(t, result, 4, "Вложенный multichoice запрещён в choice.")
		v139Run(t, result, "current", nil)
	})
	t.Run("nested branch", func(t *testing.T) {
		v139Run(t, v139Lesson(t, v139Open(t, "variants", 2, 1),
			v139Line(t, "@variant A", "tag", 3, 2, v139Element(t, "tag", "@variant", "variant", 1), v139Element(t, "name", "A", "A", 10)),
			v139Open(t, "choice", 4, 3), v139Open(t, "answer", 5, 4), v139Text(t, "Italy", "Italy", 6, 5), v139Text(t, "Spain", "Spain", 6, 5),
			v139Close(t, 6, 5), v139Text(t, "France", "France", 5, 4), v139Close(t, 5, 4),
			v139Line(t, "@variant B", "tag", 3, 2, v139Element(t, "tag", "@variant", "variant", 1), v139Element(t, "name", "B", "B", 10)),
			v139Text(t, "Other material.", "Other material.", 12, 3), v139Close(t, 3, 2)), "current",
			[]v139Expected{{[4]int{6, 1, 9, 2}, [][3]int{{8, 1, 6}}, v139DetailTwo}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal pseudo blocks/"+name, func(t *testing.T) {
			answer := `@answer \{`
			if name == "multifill" {
				answer = `\@answer \{`
			}
			v139Run(t, v139Lesson(t, v139Open(t, name, 2, 1), v139Text(t, `@choice \{`, "@choice {", 3, 2),
				v139Text(t, answer, "@answer {", 3, 2), v139Text(t, "Italy", "Italy", 3, 2), v139Text(t, "Spain", "Spain", 3, 2),
				v139Text(t, `\}`, "}", 3, 2), v139Text(t, `\}`, "}", 3, 2), v139Close(t, 3, 2), v139Text(t, "Read.", "Read.", 2, 1)), "current", nil)
		})
	}
}

func TestV139RangesAndBoundaries(t *testing.T) {
	t.Run("whole original block lines", func(t *testing.T) {
		result := v139Lesson(t,
			v139Line(t, "@CHOICE {", "block-start", 2, 1, v139Element(t, "tag", "@CHOICE", "choice", 1), v139Element(t, "block-open", "{", "{", 9)),
			v139Line(t, "  @AnSwEr {", "block-start", 3, 2, v139Element(t, "tag", "@AnSwEr", "answer", 3), v139Element(t, "block-open", "{", "{", 11)),
			v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3),
			v139Line(t, "\t} ", "block-end", 4, 3, v139Element(t, "block-close", "}", "}", 2)),
			v139Text(t, "France", "France", 3, 2), v139Close(t, 3, 2))
		v139Run(t, result, "current", []v139Expected{{[4]int{4, 1, 7, 4}, [][3]int{{6, 1, 6}}, v139DetailTwo}})
	})
	t.Run("two independent violations", func(t *testing.T) {
		v139Run(t, v139Lesson(t, v139Open(t, "choice", 2, 1), v139Open(t, "answer", 3, 2),
			v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3), v139Close(t, 4, 3), v139Text(t, "France", "France", 3, 2), v139Close(t, 3, 2),
			v139Open(t, "choice", 2, 1), v139Open(t, "answer", 10, 2), v139Text(t, "Italy", "Italy", 11, 3), v139Text(t, "Spain", "Spain", 11, 3),
			v139Close(t, 11, 3), v139Text(t, "France", "France", 10, 2), v139Close(t, 10, 2)), "current",
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}, {[4]int{11, 1, 14, 2}, [][3]int{{13, 1, 6}}, v139DetailTwo}})
	})
	for _, outside := range []bool{false, true} {
		t.Run("unclosed/outside="+strconv.FormatBool(outside), func(t *testing.T) {
			body := []model.Line{v139Open(t, "choice", 2, 1), v139Open(t, "answer", 3, 2),
				v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3)}
			if outside {
				// Модель восстановилась на новом шаге; принадлежность строк явная.
				body = append(body, v139Line(t, "### Next", "heading", 0, 0, v139Element(t, "heading-level", "###", "3", 1), v139Element(t, "title", "Next", "Next", 5)),
					v139Text(t, "Outside text.", "Outside text.", 7, 1))
			}
			result := v139Lesson(t, body...)
			result.Lines[len(result.Lines)-1].LineEnding = ""
			v139ParserError(t, result, 3, "Не закрыт choice.")
			v139ParserError(t, result, 4, "Не закрыт answer.")
			v139Run(t, result, "current", []v139Expected{{[4]int{4, 1, 6, 6}, [][3]int{{6, 1, 6}}, v139DetailTwo}})
		})
	}
	t.Run("unclosed includes final blank descendant", func(t *testing.T) {
		result := v139Lesson(t, v139Open(t, "choice", 2, 1), v139Open(t, "answer", 3, 2),
			v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3), v139Line(t, " \t", "blank", 4, 3))
		v139ParserError(t, result, 3, "Не закрыт choice.")
		v139ParserError(t, result, 4, "Не закрыт answer.")
		v139Run(t, result, "current", []v139Expected{{[4]int{4, 1, 7, 3}, [][3]int{{6, 1, 6}}, v139DetailTwo}})
	})
}

func TestV139Lifecycle(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(eol))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v139Block(t, v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := []v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}}
				v139Run(t, result, "first", want)
				v139Run(t, result, "second", want)
			})
		}
	}
	t.Run("empty model", func(t *testing.T) {
		result := v139Lesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v139Run(t, result, "empty", nil)
	})
	t.Run("independent documents", func(t *testing.T) {
		v139Run(t, v139Block(t, v139Text(t, "Italy", "Italy", 4, 3), v139Text(t, "Spain", "Spain", 4, 3)), "first",
			[]v139Expected{{[4]int{4, 1, 7, 2}, [][3]int{{6, 1, 6}}, v139DetailTwo}})
		v139Run(t, v139Block(t, v139Text(t, "Italy", "Italy", 4, 3)), "second", nil)
	})
}

// v139Block помещает явно заданные строки в answer (4), вложенный в choice (3).
// Два обычных варианта сохраняют минимальный размер choice даже при пустом answer.
func v139Block(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v139Open(t, "choice", 2, 1), v139Open(t, "answer", 3, 2)}, rows...)
	body = append(body, v139Close(t, 4, 3), v139Text(t, "France", "France", 3, 2), v139Text(t, "Canada", "Canada", 3, 2), v139Close(t, 3, 2))
	return v139Lesson(t, body...)
}

// v139Lesson добавляет только обязательные версию и заголовок; body начинается с 3.
// Исходные ошибки структуры тесты задают отдельно, не имитируя разбор DSL.
func v139Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v139Line(t, "@dsl-version 1.2", "tag", 0, 0, v139Element(t, "tag", "@dsl-version", "dsl-version", 1), v139Element(t, "version", "1.2", "1.2", 14)),
		v139Line(t, "# Lesson", "heading", 0, 0, v139Element(t, "heading-level", "#", "1", 1), v139Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
}

// v139ParserError фиксирует явно указанную ошибку на строке, сохраняя порядок.
// Служит только для согласованного контекста пустых, вложенных и незакрытых блоков.
func v139ParserError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	result.Document.HasErrors, result.Lines[number-1].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1), Source: "parser", DiagnosticCode: "fixture-structure", SeverityLevel: "error", DiagnosticScope: "line",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}},
	})
}

// v139Open задаёт объявление блока без параметров с явным родителем.
func v139Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v139Line(t, "@"+name+" {", "block-start", parent, depth,
		v139Element(t, "tag", "@"+name, name, 1), v139Element(t, "block-open", "{", "{", len(name)+3))
}

// v139Inline задаёт однострочную конструкцию с текстовым значением.
func v139Inline(t *testing.T, name, content string, parent, depth int) model.Line {
	t.Helper()
	return v139Line(t, "@"+name+" "+content, "tag", parent, depth,
		v139Element(t, "tag", "@"+name, name, 1), v139Element(t, "content", content, content, len(name)+3))
}

// v139Close задаёт закрытие указанного блока.
func v139Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v139Line(t, "}", "block-end", parent, depth, v139Element(t, "block-close", "}", "}", 1))
}

// v139Text сохраняет явно заданные Raw и семантическое значение content.
func v139Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v139Line(t, raw, "content", parent, depth, v139Element(t, "content", raw, value, 1))
}

// v139Line сохраняет заданную структуру без классификации исходного текста.
func v139Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v139Element вычисляет только длину входного элемента в Unicode-колонках.
// Конец исключён; ожидания срабатываний задаются отдельными явными числами.
func v139Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v139Registry использует исключительно полный штатный каталог регистраций.
func v139Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v139Run проверяет V139 текущего запуска полного конвейера и сохранность модели.
func v139Run(t *testing.T, result *model.Result, source string, want []v139Expected) {
	t.Helper()
	before := v139Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v139Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v139Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V139" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V139 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]}, End: model.Position{Line: expected.main[2], Column: expected.main[3]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Message != "Блок @answer внутри @choice содержит несколько строк ответа.\nконкретизация: "+expected.detail {
			t.Fatalf("occurrence %d = %+v; location = %+v, want %+v and exact V139 metadata", i, entry, entry.Location, location)
		}
		for j, position := range expected.related {
			related := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("related %d = %+v, want %+v", j, entry.RelatedLocations[j], related)
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.main[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("block-scoped V139 linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v139Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// В копии возвращаем только производные флаги/ссылки и убираем новые записи.
	// Исходные строки, метаданные и прежние диагностики должны остаться дословными.
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
	if v139Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v139Snapshot получает независимое представление модели для сравнения.
func v139Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

package checkers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV138NestedContexts(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		t.Run(tag, func(t *testing.T) {
			// У choice ровно один непустой правильный вариант и один обычный.
			result := v138Document(t, v138Open(t, tag, 2, 1), v138Open(t, "answer", 3, 2),
				v138TestText(t, "London", "London", 4, 3), v138TestText(t, "\u00a0", "\u00a0", 4, 3), v138Close(t, 4, 3),
				v138TestText(t, "Paris", "Paris", 3, 2), v138Close(t, 3, 2))
			v138Run(t, result, "current", [][4]int{{6, 1, 7, 1}})
		})
	}
	for _, tag := range []string{"matching", "ordering"} {
		t.Run(tag, func(t *testing.T) {
			// Два верхних элемента, ответы, разделитель и две нижние позиции.
			result := v138Document(t, v138Open(t, tag, 2, 1), v138TestText(t, "City A", "City A", 3, 2),
				v138Open(t, "answer", 3, 2), v138TestText(t, "London", "London", 5, 3), v138TestText(t, "\u00a0", "\u00a0", 5, 3), v138Close(t, 5, 3),
				v138TestText(t, "City B", "City B", 3, 2), v138Tag(t, "answer", "content", "Paris", 3, 2),
				v138Line(t, "---", "separator", 3, 2), v138TestText(t, "London", "London", 3, 2), v138TestText(t, "Paris", "Paris", 3, 2),
				v138Close(t, 3, 2))
			v138Run(t, result, "current", [][4]int{{7, 1, 8, 1}})
		})
	}
	t.Run("task step and choice", func(t *testing.T) {
		result := v138Document(t, v138Tag(t, "task", "identifier", "t", 2, 1), v138Tag(t, "step", "title", "First", 3, 2),
			v138Open(t, "choice", 4, 3), v138Open(t, "answer", 5, 4), v138TestText(t, "London", "London", 6, 5),
			v138TestText(t, "\u00a0", "\u00a0", 6, 5), v138Close(t, 6, 5), v138TestText(t, "Paris", "Paris", 5, 4), v138Close(t, 5, 4),
			v138Tag(t, "step", "title", "Second", 3, 2), v138TestText(t, "Read.", "Read.", 12, 3), v138Tag(t, "endtask", "", "", 2, 1))
		v138Run(t, result, "current", [][4]int{{8, 1, 9, 1}})
	})
	t.Run("nested construction is excluded from outer answer", func(t *testing.T) {
		result := v138AnswerDocument(t, v138TestText(t, "London", "London", 4, 2), v138Open(t, "editor", 4, 2),
			v138Blank(t, "", 6, 3), v138Close(t, 6, 3), v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2))
		v138ParserError(t, result, 6, "Вложенный тег внутри answer запрещён.")
		v138Run(t, result, "current", [][4]int{{9, 1, 10, 1}})
	})
}

func TestV138ExcludedAreasAndSeparateBlocks(t *testing.T) {
	t.Run("one-line answer", func(t *testing.T) {
		result := v138Document(t, v138Tag(t, "question", "content", "Where?", 2, 1), v138Tag(t, "answer", "content", "London", 2, 1),
			v138Blank(t, "", 2, 1), v138TestText(t, "Read.", "Read.", 2, 1))
		v138Run(t, result, "current", nil)
	})
	t.Run("outside answer", func(t *testing.T) {
		result := v138Document(t, v138Blank(t, "", 2, 1), v138Tag(t, "question", "content", "Where?", 2, 1),
			v138Open(t, "answer", 2, 1), v138TestText(t, "London", "London", 5, 2), v138Close(t, 5, 2),
			v138Blank(t, "", 2, 1), v138Tag(t, "note", "content", "Read.", 2, 1))
		v138Run(t, result, "current", nil)
	})
	t.Run("different block", func(t *testing.T) {
		v138Run(t, v138Document(t, v138Open(t, "note", 2, 1), v138TestText(t, "Read.", "Read.", 3, 2),
			v138Blank(t, "", 3, 2), v138Close(t, 3, 2)), "current", nil)
	})
	t.Run("literal answer declaration", func(t *testing.T) {
		v138Run(t, v138Document(t, v138Open(t, "editor", 2, 1), v138TestText(t, "@answer {", "@answer {", 3, 2),
			v138Blank(t, "", 3, 2), v138Close(t, 3, 2)), "current", nil)
	})
	t.Run("two independent blocks", func(t *testing.T) {
		result := v138Document(t, v138Tag(t, "question", "content", "First?", 2, 1), v138Open(t, "answer", 2, 1),
			v138TestText(t, "London", "London", 4, 2), v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138Close(t, 4, 2),
			v138Tag(t, "question", "content", "Second?", 2, 1), v138Open(t, "answer", 2, 1),
			v138TestText(t, "Paris", "Paris", 9, 2), v138TestText(t, "\u00a0", "\u00a0", 9, 2), v138Close(t, 9, 2))
		v138Run(t, result, "current", [][4]int{{6, 1, 7, 1}, {11, 1, 12, 1}})
	})
}

func TestV138EOF(t *testing.T) {
	for _, tc := range []struct {
		name, previous string
		rows           []model.Line
		ending         model.LineEnding
		want           [][4]int
	}{
		{"blank spaces without EOL", "London", []model.Line{v138Blank(t, " \t", 4, 2)}, "", nil},
		{"content spaces without EOL", "London", []model.Line{v138TestText(t, " \t", " \t", 4, 2)}, "", [][4]int{{5, 7, 6, 3}}},
		{"empty last physical row with LF", "London", []model.Line{v138Blank(t, "", 4, 2)}, "\n", nil},
		{"empty last physical row with CRLF", "London", []model.Line{v138Blank(t, "", 4, 2)}, "\r\n", nil},
		{"empty content with LF", "London", []model.Line{v138TestText(t, "", "", 4, 2)}, "\n", [][4]int{{5, 7, 6, 1}}},
		{"empty content with CRLF", "London", []model.Line{v138TestText(t, "", "", 4, 2)}, "\r\n", [][4]int{{5, 7, 6, 1}}},
		{"two blank rows", "London", []model.Line{v138Blank(t, "", 4, 2), v138Blank(t, "", 4, 2)}, "\n", nil},
		{"two empty contents have prescribed equal ranges", "London", []model.Line{v138TestText(t, "", "", 4, 2), v138TestText(t, "", "", 4, 2)},
			"\n", [][4]int{{6, 1, 7, 1}, {6, 1, 7, 1}}},
		{"Unicode previous row before blank", "Я😀", []model.Line{v138Blank(t, " ", 4, 2)}, "", nil},
		{"Unicode previous row before content", "Я😀", []model.Line{v138TestText(t, " ", " ", 4, 2)}, "", [][4]int{{5, 3, 6, 2}}},
		{"Unicode whitespace without EOL", "London", []model.Line{v138TestText(t, "\u00a0", "\u00a0", 4, 2)}, "", [][4]int{{5, 7, 6, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v138Tag(t, "question", "content", "Where?", 2, 1), v138Open(t, "answer", 2, 1),
				v138TestText(t, tc.previous, tc.previous, 4, 2)}
			result := v138Document(t, append(body, tc.rows...)...)
			result.Lines[len(result.Lines)-1].LineEnding = tc.ending
			v138ParserError(t, result, 4, "Блок answer не закрыт до конца файла.")
			v138Run(t, result, "current", tc.want)
		})
	}
	t.Run("first own row is last physical row", func(t *testing.T) {
		result := v138Document(t, v138Tag(t, "question", "content", "Where?", 2, 1), v138Open(t, "answer", 2, 1), v138Blank(t, " ", 4, 2))
		v138ParserError(t, result, 4, "Блок answer не закрыт и не содержит ответа.")
		v138Run(t, result, "current", nil)
	})
	t.Run("first own content is last physical row", func(t *testing.T) {
		result := v138Document(t, v138Tag(t, "question", "content", "Where?", 2, 1), v138Open(t, "answer", 2, 1), v138TestText(t, " ", " ", 4, 2))
		v138ParserError(t, result, 4, "Блок answer не закрыт и не содержит ответа.")
		v138Run(t, result, "current", [][4]int{{4, 10, 5, 2}})
	})
}

func TestV138EndingsCaseAndHistory(t *testing.T) {
	for _, ending := range []string{"LF", "CRLF", "mixed"} {
		t.Run(ending, func(t *testing.T) {
			result := v138AnswerDocument(t, v138TestText(t, "London", "London", 4, 2), v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2))
			for index := range result.Lines {
				if ending == "CRLF" || ending == "mixed" && index%2 == 0 {
					result.Lines[index].LineEnding = "\r\n"
				} else {
					result.Lines[index].LineEnding = "\n"
				}
			}
			v138Run(t, result, "first", [][4]int{{6, 1, 7, 1}})
			v138Run(t, result, "second", [][4]int{{6, 1, 7, 1}})
			v138Run(t, v138AnswerDocument(t, v138TestText(t, "London", "London", 4, 2)), "separate", nil)
		})
	}
	t.Run("case and indentation keep source positions", func(t *testing.T) {
		result := v138AnswerDocument(t, v138TestText(t, "\t  ", "\t  ", 4, 2), v138TestText(t, "London", "London", 4, 2))
		result.Lines[3] = v138Line(t, " \t@AnSwEr {", "block-start", 2, 1,
			v138Element(t, "tag", "@AnSwEr", "answer", 3, 10), v138Element(t, "block-open", "{", "{", 11, 12))
		result.Lines[3].Number = 4
		v138Run(t, result, "current", [][4]int{{5, 1, 6, 1}})
	})
}

func TestV138EmptyAndUnsupportedVersions(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v138Run(t, result, "empty", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v138AnswerDocument(t, v138Blank(t, "", 4, 2), v138TestText(t, "London", "London", 4, 2))
			result.Document.DSLVersion = &version
			result.Lines[0] = v138Tag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			registry, err := diagnostics.NewRegistry(RegistrationCheckers())
			if err != nil {
				t.Fatal(err)
			}
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

func TestV138Registration(t *testing.T) {
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
		if definition.Code != "V138" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "В блоке @answer указано пустое значение ответа." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V138 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V138 definitions = %d, want 1", count)
	}
	if RegistrationCheckers()["V004"] == nil {
		t.Fatal("V004 must remain registered")
	}
}

func TestV138ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           [][4]int
	}{
		{"reference", "@question Where does Tom live?\n@answer {\nLondon\nParis\n}", []model.Line{
			v138TestText(t, "London", "London", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2)}, nil},
		{"example", "@question Where does Tom live?\n@answer {\nLondon\n\nParis\n}", []model.Line{
			v138TestText(t, "London", "London", 4, 2), v138Blank(t, "", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2)}, nil},
		{"empty value", "@question Where does Tom live?\n@answer {\nLondon\n\u00a0\nParis\n}", []model.Line{
			v138TestText(t, "London", "London", 4, 2), v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2)},
			[][4]int{{6, 1, 7, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.30: фрагмент дополняется только обязательными версией и # Lesson.
			result := v138AnswerDocument(t, tc.rows...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v138Run(t, result, "current", tc.want)
		})
	}
}

func TestV138OwnRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want [][4]int
	}{
		{"first blank", []model.Line{v138Blank(t, "", 4, 2), v138TestText(t, "London", "London", 4, 2)}, nil},
		{"last blank before closing", []model.Line{v138TestText(t, "London", "London", 4, 2), v138Blank(t, "", 4, 2)}, nil},
		{"two adjacent blanks", []model.Line{v138TestText(t, "London", "London", 4, 2), v138Blank(t, "", 4, 2),
			v138Blank(t, "", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2)}, nil},
		{"separate blanks", []model.Line{v138Blank(t, "", 4, 2), v138TestText(t, "London", "London", 4, 2), v138Blank(t, "", 4, 2)}, nil},
		{"blank space and tab", []model.Line{v138TestText(t, "London", "London", 4, 2), v138Blank(t, " \t  ", 4, 2)}, nil},
		{"one blank and no nonempty answer", []model.Line{v138Blank(t, "", 4, 2)}, nil},
		{"first content", []model.Line{v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138TestText(t, "London", "London", 4, 2)}, [][4]int{{5, 1, 6, 1}}},
		{"last content before closing", []model.Line{v138TestText(t, "London", "London", 4, 2), v138TestText(t, "\u00a0", "\u00a0", 4, 2)}, [][4]int{{6, 1, 7, 1}}},
		{"two adjacent contents", []model.Line{v138TestText(t, "London", "London", 4, 2), v138TestText(t, "\u00a0", "\u00a0", 4, 2),
			v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2)}, [][4]int{{6, 1, 7, 1}, {7, 1, 8, 1}}},
		{"separate contents", []model.Line{v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138TestText(t, "London", "London", 4, 2), v138TestText(t, "\u00a0", "\u00a0", 4, 2)},
			[][4]int{{5, 1, 6, 1}, {7, 1, 8, 1}}},
		{"content space and tab", []model.Line{v138TestText(t, "London", "London", 4, 2), v138TestText(t, " \t  ", " \t  ", 4, 2)}, [][4]int{{6, 1, 7, 1}}},
		{"one empty content", []model.Line{v138TestText(t, "", "", 4, 2)}, [][4]int{{5, 1, 6, 1}}},
		{"empty content between blanks and alternatives", []model.Line{v138TestText(t, "London", "London", 4, 2), v138Blank(t, "", 4, 2),
			v138TestText(t, "\u00a0", "\u00a0", 4, 2), v138Blank(t, "", 4, 2), v138TestText(t, "Paris", "Paris", 4, 2)}, [][4]int{{7, 1, 8, 1}}},
		{"no rows is not a blank row", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v138AnswerDocument(t, tc.rows...)
			if tc.name == "no rows is not a blank row" || tc.name == "one blank and no nonempty answer" || tc.name == "one empty content" {
				v138ParserError(t, result, 4, "Блок не содержит непустого ответа.")
			}
			v138Run(t, result, "current", tc.want)
		})
	}
}

func TestV138UnicodeWhitespace(t *testing.T) {
	// Список выписан по требованию White_Space, без вычисления ожиданий через
	// нормализатор. U+000A/U+000D представлены физическими LF/CRLF, не внутри Raw.
	for _, char := range []rune{'\u000b', '\u000c', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a',
		'\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(fmt.Sprintf("U+%04X", char), func(t *testing.T) {
			value := string(char)
			result := v138AnswerDocument(t, v138TestText(t, "London", "London", 4, 2), v138TestText(t, value, value, 4, 2))
			v138Run(t, result, "current", [][4]int{{6, 1, 7, 1}})
		})
	}
	t.Run("mixed whitespace", func(t *testing.T) {
		result := v138AnswerDocument(t, v138TestText(t, "London", "London", 4, 2),
			v138TestText(t, " \t\u00a0 \u2009\u3000\t", "\u00a0 \u2009\u3000", 4, 2))
		v138Run(t, result, "current", [][4]int{{6, 1, 7, 1}})
	})
}

func TestV138NonemptyValues(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"  London  ", "London"}, {"\u00a0New\t\u2009York\u3000", "\u00a0New\t\u2009York\u3000"},
		{".", "."}, {"...", "..."}, {"—", "—"}, {" \u00a0...\u3000 ", "\u00a0...\u3000"},
		{"\u200b", "\u200b"}, {"\ufeff", "\ufeff"}, {"\u0301", "\u0301"}, {"\u180e", "\u180e"},
		{`\}`, "}"}, {`\{`, "{"}, {`\\`, `\`}, {`\@answer`, "@answer"}, {`\# Heading`, "# Heading"},
		{`\x`, `\x`}, {"<br>", "<br>"}, {"---", "---"}, {"_____", "_____"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v138Run(t, v138AnswerDocument(t, v138TestText(t, tc.raw, tc.value, 4, 2)), "current", nil)
		})
	}
}

func TestV138ProcessedContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
		want [][4]int
	}{
		{"empty processed value", v138TestText(t, "London", "", 4, 2), [][4]int{{5, 1, 6, 1}}},
		{"processed whitespace", v138TestText(t, `\ `, " ", 4, 2), [][4]int{{5, 1, 6, 1}}},
		{"nonempty processed value", v138TestText(t, " \t", "London", 4, 2), nil},
		{"empty parts covering significant raw", v138Line(t, "AB", "content", 4, 2,
			v138Element(t, "content", "A", "", 1, 2), v138Element(t, "content", "B", "", 2, 3)), [][4]int{{5, 1, 6, 1}}},
		{"whitespace between empty parts", v138Line(t, "\u00a0 \u2003", "content", 4, 2,
			v138Element(t, "content", "\u00a0", "\u00a0", 1, 2), v138Element(t, "content", "\u2003", "\u2003", 3, 4)), [][4]int{{5, 1, 6, 1}}},
		{"nonempty first part", v138Line(t, "Paris\u00a0", "content", 4, 2,
			v138Element(t, "content", "Paris", "Paris", 1, 6), v138Element(t, "content", "\u00a0", "\u00a0", 6, 7)), nil},
		{"nonempty last part", v138Line(t, "\u00a0Paris", "content", 4, 2,
			v138Element(t, "content", "\u00a0", "\u00a0", 1, 2), v138Element(t, "content", "Paris", "Paris", 2, 7)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v138Run(t, v138AnswerDocument(t, tc.row), "current", tc.want)
		})
	}
}

func TestV138UnknownValues(t *testing.T) {
	t.Run("no elements", func(t *testing.T) {
		v138Run(t, v138AnswerDocument(t, v138Line(t, "\u00a0", "content", 4, 2)), "current", nil)
	})
	for _, withContent := range []bool{false, true} {
		t.Run("unparsed with content="+strconv.FormatBool(withContent), func(t *testing.T) {
			unknown := model.Element{ElementType: "unparsed", Raw: "?", Start: 1, End: 2, ErrorIDs: []string{"p1"}}
			row := v138Line(t, "?", "content", 4, 2, unknown)
			if withContent {
				row.Raw += "\u00a0"
				row.Elements = append(row.Elements, v138Element(t, "content", "\u00a0", "\u00a0", 2, 3))
			}
			result := v138AnswerDocument(t, row)
			v138ParserError(t, result, 5, "Значение ответа не разобрано.")
			result.Diagnostics[0].DiagnosticScope = "element"
			result.Diagnostics[0].Location.End.Column = 2
			v138Run(t, result, "current", nil)
		})
	}
}

func TestV138UncoveredSignificantRaw(t *testing.T) {
	// Пустые известные фрагменты не доказывают пустоту неразобранного остатка.
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"prefix", v138Line(t, "Я\u00a0", "content", 4, 2, v138Element(t, "content", "\u00a0", "\u00a0", 2, 3))},
		{"middle", v138Line(t, "\u00a0x\u2003", "content", 4, 2,
			v138Element(t, "content", "\u00a0", "\u00a0", 1, 2), v138Element(t, "content", "\u2003", "\u2003", 3, 4))},
		{"suffix", v138Line(t, "\u00a0x", "content", 4, 2, v138Element(t, "content", "\u00a0", "\u00a0", 1, 2))},
		{"zero width content", v138Line(t, "x", "content", 4, 2, v138Element(t, "content", "", "", 1, 1))},
		{"no content elements", v138Line(t, "x", "content", 4, 2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v138Run(t, v138AnswerDocument(t, tc.row), "current", nil)
		})
	}
}

// v138Run использует штатный каталог и конвейер, выделяя только новые V138.
// Координаты ожидаемых диапазонов передаются из тестов явно и независимо.
func v138Run(t *testing.T, result *model.Result, source string, want [][4]int) {
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
		if diagnostic.DiagnosticCode == "V138" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V138 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[2], Column: expected[3]}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" ||
			entry.Message != "В блоке @answer указано пустое значение ответа." || len(entry.RelatedLocations) != 0 ||
			entry.Location == nil || *entry.Location != location {
			t.Fatalf("occurrence %d = %+v, want exact metadata and range %+v", index, entry, location)
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
	for index, diagnostic := range original.Diagnostics {
		left, _ := json.Marshal(diagnostic)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v138Element задаёт исходное и семантическое значения без преобразований.
func v138Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v138Line сохраняет явные тип, родителя, глубину и элементы строки.
func v138Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v138Tag создаёт однострочное объявление с явно заданным параметром.
func v138Tag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v138Element(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v138Element(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v138Line(t, raw, "tag", parent, depth, elements...)
}

// v138Open создаёт объявление блока без параметров.
func v138Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v138Line(t, "@"+tag+" {", "block-start", parent, depth,
		v138Element(t, "tag", "@"+tag, tag, 1, len(tag)+2), v138Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v138Close создаёт закрытие с заданным родителем.
func v138Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v138Line(t, "}", "block-end", parent, depth, v138Element(t, "block-close", "}", "}", 1, 2))
}

// v138TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v138TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v138Line(t, raw, "content", parent, depth, v138Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v138Blank задаёт обычную пустую строку без текстового элемента.
func v138Blank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v138Line(t, raw, "blank", parent, depth)
}

// v138Document добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v138Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v138Tag(t, "dsl-version", "version", "1.2", 0, 0),
		v138Line(t, "# Lesson", "heading", 0, 0, v138Element(t, "heading-level", "#", "1", 1, 2), v138Element(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

// v138AnswerDocument окружает заданные строки вопросом и полным блоком ответа.
func v138AnswerDocument(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v138Tag(t, "question", "content", "Where does Tom live?", 2, 1), v138Open(t, "answer", 2, 1)}
	body = append(body, rows...)
	body = append(body, v138Close(t, 4, 2))
	return v138Document(t, body...)
}

// v138ParserError добавляет исходную ошибку разбора на явно заданной строке.
func v138ParserError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "p" + strconv.Itoa(len(result.Diagnostics)+1), Source: "parser",
		DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(line.Raw) + 1}}})
}

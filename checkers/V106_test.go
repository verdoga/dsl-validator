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

// v106TestExpected задаёт исходный диапазон одного открытия, конец исключён.
type v106TestExpected struct{ line, start, end int }

func TestV106Registration(t *testing.T) {
	definitions, err := v106TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count, cellChecks := 0, 0
	for _, definition := range definitions {
		if definition.Code == "V097" {
			cellChecks++
		}
		if definition.Code != "V106" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "В заголовочной части таблицы использована разделённая ячейка." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V106 definitions = %d, want 1", count)
	}
	if cellChecks != 1 {
		t.Fatalf("V097 definitions = %d, want 1 alongside V106", cellChecks)
	}
}

func TestV106ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, header, body, fragment string
		want                         []v106TestExpected
	}{
		{"reference", "Форма | Значение", "<am | is> | есть", "@table {\nФорма | Значение\n---\n<am | is> | есть\n}", nil},
		{"example", "<Форма | Вариант> | Значение", "am | есть", "@table {\n<Форма | Вариант> | Значение\n---\nam | есть\n}", []v106TestExpected{{4, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v106TestTable(t, v106TestText(t, tc.header, tc.header, 3, 2),
				v106TestSeparator(t, 3, 2), v106TestText(t, tc.body, tc.body, 3, 2))
			// К дословному фрагменту добавлены только обязательные версия и заголовок.
			// HTML-сущности в таблице задания обозначают исходные |, < и >.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v106TestRun(t, result, "current", tc.want)
			// В обеих строках по две внешние ячейки; черта внутри <...>
			// не увеличивает их число. V097 не должна выдавать сообщение,
			// даже если разделённая ячейка заголовка отдельно нарушает V106.
			for _, entry := range result.Diagnostics {
				if entry.Source == "current" && entry.DiagnosticCode == "V097" {
					t.Fatalf("V097 reported unequal cell counts in a two-column table: %+v", entry)
				}
			}
		})
	}
}

func TestV106Separators(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v106TestExpected
	}{
		{"no separator", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2)}, nil},
		{"two separators", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2), v106TestSeparator(t, 3, 2),
			v106TestText(t, "C", "C", 3, 2), v106TestSeparator(t, 3, 2), v106TestText(t, "D", "D", 3, 2)}, nil},
		{"three separators", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2), v106TestSeparator(t, 3, 2),
			v106TestSeparator(t, 3, 2), v106TestSeparator(t, 3, 2)}, nil},
		{"escaped separator only", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2), v106TestText(t, `\---`, "---", 3, 2)}, nil},
		{"escaped separator does not duplicate real one", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestText(t, `\---`, "---", 3, 2), v106TestSeparator(t, 3, 2), v106TestText(t, "C", "C", 3, 2)}, []v106TestExpected{{4, 1, 2}}},
		{"separator whitespace", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestLine(t, " \t---\t ", "separator", 3, 2), v106TestText(t, "C", "C", 3, 2)}, []v106TestExpected{{4, 1, 2}}},
		{"first row separator means empty header", []model.Line{v106TestSeparator(t, 3, 2), v106TestText(t, "<A|B>", "<A|B>", 3, 2)}, nil},
		{"missing body does not suppress header check", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestSeparator(t, 3, 2)}, []v106TestExpected{{4, 1, 2}}},
		{"blank header", []model.Line{v106TestLine(t, "", "blank", 3, 2), v106TestLine(t, " \t", "blank", 3, 2),
			v106TestSeparator(t, 3, 2), v106TestText(t, "<A|B>", "<A|B>", 3, 2)}, nil},
		{"all header rows but no body rows", []model.Line{v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestLine(t, " \t", "blank", 3, 2), v106TestText(t, "<C|D>", "<C|D>", 3, 2),
			v106TestSeparator(t, 3, 2), v106TestText(t, "<E|F>", "<E|F>", 3, 2)}, []v106TestExpected{{4, 1, 2}, {6, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v106TestRun(t, v106TestTable(t, tc.rows...), "current", tc.want)
		})
	}
	for _, raw := range []string{"--", "----", "--- x", "x ---", "\u00a0---\u00a0", "---\u2003", "\t-- -\t", `\\---`} {
		t.Run("not separator/"+strconv.Quote(raw), func(t *testing.T) {
			value := strings.Trim(raw, " \t")
			if raw == `\\---` {
				value = `\---`
			}
			v106TestRun(t, v106TestTable(t, v106TestText(t, "<A|B>", "<A|B>", 3, 2),
				v106TestText(t, raw, value, 3, 2)), "current", nil)
		})
	}
}

func TestV106Openings(t *testing.T) {
	for _, tc := range []struct {
		name, body, raw string
		want            []v106TestExpected
	}{
		{"ordinary cells", "C | D", "A | B", nil},
		{"ordinary opening in middle", "C | D", "x<A|B>", nil},
		{"leading whitespace", "C", " \t<A|B>", []v106TestExpected{{4, 3, 4}}},
		{"two split cells", "C | D", "<A|B> | <C|D>", []v106TestExpected{{4, 1, 2}, {4, 9, 10}}},
		{"empty outer cells", "C | D | E | F | G", "||<A|B>||", []v106TestExpected{{4, 3, 4}}},
		{"no spaces at separator", "C | D", "A|<B|C>", []v106TestExpected{{4, 3, 4}}},
		{"Unicode columns", "C | D", "Я😀 | <A|B>", []v106TestExpected{{4, 6, 7}}},
		{"closed nested", "C | D", "<A|<B|C>> | <D|E>", []v106TestExpected{{4, 1, 2}, {4, 13, 14}}},
		{"nested opening in text", "C | D", "<A x<B|C>> | <D|E>", []v106TestExpected{{4, 1, 2}, {4, 14, 15}}},
		{"deep nesting", "C | D", "<A|<B|<C|D>>> | <E|F>", []v106TestExpected{{4, 1, 2}, {4, 17, 18}}},
		{"unclosed nested retains internal pipes", "C", "<A|<B|C> | <D|E>", []v106TestExpected{{4, 1, 2}}},
		{"unclosed", "C", "<A|B", []v106TestExpected{{4, 1, 2}}},
		{"only opening", "C", "<", []v106TestExpected{{4, 1, 2}}},
		{"opening at last column", "C | D", "A | <", []v106TestExpected{{4, 5, 6}}},
		{"empty closed cell", "C", "<>", []v106TestExpected{{4, 1, 2}}},
		{"single part", "C", "<A>", []v106TestExpected{{4, 1, 2}}},
		{"extra closing", "C | D", "> | <A|B>", []v106TestExpected{{4, 5, 6}}},
		{"no new cell without external separator", "C", "<A|B> <C>", []v106TestExpected{{4, 1, 2}}},
		{"tail opening is ordinary", "C | D", "<A|B> <C | <D|E>", []v106TestExpected{{4, 1, 2}, {4, 12, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v106TestRun(t, v106TestHeader(t, tc.body, v106TestText(t, tc.raw, strings.Trim(tc.raw, " \t"), 3, 2)), "current", tc.want)
		})
	}
	// Принятая пробельность — U+0020/U+0009, без переноса правила V098.
	for _, char := range []string{"\u00a0", "\v", "\f", "\u0085", "\u1680", "\u2003", "\u200b", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("other whitespace is content/"+strconv.Quote(char), func(t *testing.T) {
			raw := char + "<A|B> |" + char + "<C|D>"
			v106TestRun(t, v106TestHeader(t, "A | B | C | D", v106TestText(t, raw, raw, 3, 2)), "current", nil)
		})
	}
}

func TestV106Escapes(t *testing.T) {
	for _, tc := range []struct {
		name, body, raw, value string
		want                   []v106TestExpected
	}{
		{"literal opening", "C | D", `\<A|B>`, `<A|B>`, nil},
		{"literal pipe", "C | D", `\|<A|B>`, `|<A|B>`, nil},
		{"paired slash then external pipe", "C | D", `\\|<A|B>`, `\|<A|B>`, []v106TestExpected{{4, 4, 5}}},
		{"three slashes then literal pipe", "C | D", `\\\|<A|B>`, `\|<A|B>`, nil},
		{"four slashes then external pipe", "C | D", `\\\\|<A|B>`, `\\|<A|B>`, []v106TestExpected{{4, 6, 7}}},
		{"paired slash occupies first position", "C | D", `\\<A|B>`, `\<A|B>`, nil},
		{"literal closing occupies first position", "C | D", `\><A|B>`, `><A|B>`, nil},
		{"literal nested opening", "C | D", `<A\<B|C> | <D|E>`, `<A<B|C> | <D|E>`, []v106TestExpected{{4, 1, 2}, {4, 12, 13}}},
		{"paired slash then nested opening", "C", `<A\\<B|C> | <D|E>`, `<A\<B|C> | <D|E>`, []v106TestExpected{{4, 1, 2}}},
		{"escaped closing keeps inner pipes", "C", `<A|B\> | <C|D>`, `<A|B> | <C|D>`, []v106TestExpected{{4, 1, 2}}},
		{"paired slash then closing", "C | D", `<A|B\\> | <C|D>`, `<A|B\> | <C|D>`, []v106TestExpected{{4, 1, 2}, {4, 11, 12}}},
		{"three slashes before closing", "C", `<A|B\\\> | <C|D>`, `<A|B\> | <C|D>`, []v106TestExpected{{4, 1, 2}}},
		{"escaped pipe after closing", "C | D", `<A|B> \| <C|D>`, `<A|B> | <C|D>`, []v106TestExpected{{4, 1, 2}}},
		{"unknown escape is content", "C | D", `\q<A|B>`, `\q<A|B>`, nil},
		{"unknown escape inside cell does not hide opening", "C", `<A\q<B|C> | <D|E>`, `<A\q<B|C> | <D|E>`, []v106TestExpected{{4, 1, 2}}},
		{"trailing slash", "C", `<A|B\`, `<A|B\`, []v106TestExpected{{4, 1, 2}}},
		{"single slash", "C", `\`, `\`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v106TestRun(t, v106TestHeader(t, tc.body, v106TestText(t, tc.raw, tc.value, 3, 2)), "current", tc.want)
		})
	}
}

func TestV106RowsAndUnparsed(t *testing.T) {
	t.Run("row state resets", func(t *testing.T) {
		v106TestRun(t, v106TestHeader(t, "C",
			v106TestText(t, "<A|B", "<A|B", 3, 2), v106TestText(t, "<C|D>", "<C|D>", 3, 2)),
			"current", []v106TestExpected{{4, 1, 2}, {5, 1, 2}})
	})
	t.Run("next row closing does not hide new opening", func(t *testing.T) {
		v106TestRun(t, v106TestHeader(t, "C | D",
			v106TestText(t, "X | <A|B", "X | <A|B", 3, 2), v106TestText(t, "> | <C|D>", "> | <C|D>", 3, 2)),
			"current", []v106TestExpected{{4, 5, 6}, {5, 5, 6}})
	})
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"separate elements", v106TestLine(t, "A | <B|C>", "content", 3, 2,
			v106TestElement(t, "content", "A", "A", 1, 2), v106TestElement(t, "content", "B", "B", 6, 7),
			v106TestElement(t, "content", "C", "C", 8, 9))},
		{"unparsed tail", v106TestLine(t, "A | <B|C>", "content", 3, 2,
			v106TestElement(t, "content", "A", "A", 1, 2), v106TestElement(t, "unparsed", "<B|C>", "", 5, 10))},
		{"whole row unparsed", v106TestLine(t, "A | <B|C>", "content", 3, 2,
			v106TestElement(t, "unparsed", "A | <B|C>", "", 1, 10))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v106TestRun(t, v106TestHeader(t, "C | D", tc.row), "current", []v106TestExpected{{4, 5, 6}})
		})
	}
	for _, kind := range []model.LineType{"content", "invalid"} {
		t.Run("raw separator with unparsed/"+string(kind), func(t *testing.T) {
			v106TestRun(t, v106TestTable(t, v106TestText(t, "<A|B>", "<A|B>", 3, 2),
				v106TestLine(t, " \t--- ", kind, 3, 2, v106TestElement(t, "unparsed", "---", "", 3, 6)),
				v106TestText(t, "C", "C", 3, 2)), "current", []v106TestExpected{{4, 1, 2}})
		})
		t.Run("second raw separator with unparsed/"+string(kind), func(t *testing.T) {
			v106TestRun(t, v106TestTable(t, v106TestText(t, "<A|B>", "<A|B>", 3, 2),
				v106TestSeparator(t, 3, 2), v106TestText(t, "C", "C", 3, 2),
				v106TestLine(t, "---", kind, 3, 2, v106TestElement(t, "unparsed", "---", "", 1, 4))), "current", nil)
		})
	}
}

func TestV106Contexts(t *testing.T) {
	t.Run("table inside fragment", func(t *testing.T) {
		fragment := v106TestLine(t, "@fragment f {", "block-start", 2, 1,
			v106TestElement(t, "tag", "@fragment", "fragment", 1, 10), v106TestElement(t, "identifier", "f", "f", 11, 12),
			v106TestElement(t, "block-open", "{", "{", 13, 14))
		v106TestRun(t, v106TestLesson(t, fragment, v106TestOpen(t, "table", 3, 2),
			v106TestText(t, "<A|B>", "<A|B>", 4, 3), v106TestSeparator(t, 4, 3), v106TestText(t, "C", "C", 4, 3),
			v106TestClose(t, 4, 3), v106TestClose(t, 3, 2)), "current", []v106TestExpected{{5, 1, 2}})
	})
	t.Run("title is not row data", func(t *testing.T) {
		v106TestRun(t, v106TestLesson(t, v106TestLine(t, "@table <A|B> {", "block-start", 2, 1,
			v106TestElement(t, "tag", "@table", "table", 1, 7), v106TestElement(t, "title", "<A|B>", "<A|B>", 8, 13),
			v106TestElement(t, "block-open", "{", "{", 14, 15)), v106TestText(t, "Header", "Header", 3, 2),
			v106TestSeparator(t, 3, 2), v106TestText(t, "Body", "Body", 3, 2), v106TestClose(t, 3, 2)), "current", nil)
	})
	for _, outerSeparator := range []bool{false, true} {
		t.Run("nested rows and separator excluded/outer="+strconv.FormatBool(outerSeparator), func(t *testing.T) {
			rows := []model.Line{v106TestOpen(t, "table", 2, 1), v106TestText(t, "<A|B>", "<A|B>", 3, 2),
				v106TestOpen(t, "example", 3, 2), v106TestText(t, "<C|D>", "<C|D>", 5, 3),
				v106TestSeparator(t, 5, 3), v106TestText(t, "<E|F>", "<E|F>", 5, 3), v106TestClose(t, 5, 3)}
			var want []v106TestExpected
			if outerSeparator {
				rows = append(rows, v106TestSeparator(t, 3, 2))
				want = []v106TestExpected{{4, 1, 2}}
			}
			rows = append(rows, v106TestText(t, "G", "G", 3, 2), v106TestClose(t, 3, 2))
			result := v106TestLesson(t, rows...)
			// Запрещённое вложение — отдельная ошибка; строки example не становятся данными table.
			v106TestParserError(t, result, 5, 5, 11, "line")
			v106TestRun(t, result, "current", want)
		})
	}
	t.Run("nested inline declaration excluded", func(t *testing.T) {
		result := v106TestHeader(t, "C", v106TestText(t, "Header", "Header", 3, 2),
			v106TestLine(t, "@note <A|B>", "tag", 3, 2, v106TestElement(t, "tag", "@note", "note", 1, 6),
				v106TestElement(t, "content", "<A|B>", "<A|B>", 7, 12)))
		v106TestParserError(t, result, 5, 5, 12, "line")
		v106TestRun(t, result, "current", nil)
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo table/"+tag, func(t *testing.T) {
			v106TestRun(t, v106TestLesson(t, v106TestOpen(t, tag, 2, 1),
				v106TestText(t, "@table {", "@table {", 3, 2), v106TestText(t, "<A|B>", "<A|B>", 3, 2),
				v106TestText(t, "---", "---", 3, 2), v106TestText(t, "C", "C", 3, 2),
				v106TestText(t, `\}`, "}", 3, 2), v106TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("other block", func(t *testing.T) {
		v106TestRun(t, v106TestLesson(t, v106TestOpen(t, "example", 2, 1), v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestSeparator(t, 3, 2), v106TestText(t, "C", "C", 3, 2), v106TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("uppercase tag", func(t *testing.T) {
		result := v106TestHeader(t, "C", v106TestText(t, "<A|B>", "<A|B>", 3, 2))
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@TABLE {", "@TABLE"
		v106TestRun(t, result, "current", []v106TestExpected{{4, 1, 2}})
	})
	t.Run("independent sibling tables", func(t *testing.T) {
		v106TestRun(t, v106TestLesson(t, v106TestOpen(t, "table", 2, 1),
			v106TestText(t, "<A|B>", "<A|B>", 3, 2), v106TestSeparator(t, 3, 2),
			v106TestText(t, "C", "C", 3, 2), v106TestClose(t, 3, 2), v106TestOpen(t, "table", 2, 1),
			v106TestText(t, "<D|E>", "<D|E>", 8, 2), v106TestSeparator(t, 8, 2),
			v106TestText(t, "F", "F", 8, 2), v106TestClose(t, 8, 2)), "current", []v106TestExpected{{4, 1, 2}, {9, 1, 2}})
	})
	t.Run("sibling separator cannot create header", func(t *testing.T) {
		v106TestRun(t, v106TestLesson(t, v106TestOpen(t, "table", 2, 1), v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestClose(t, 3, 2), v106TestOpen(t, "table", 2, 1), v106TestText(t, "<D|E>", "<D|E>", 6, 2),
			v106TestSeparator(t, 6, 2), v106TestText(t, "F", "F", 6, 2), v106TestClose(t, 6, 2)), "current", []v106TestExpected{{7, 1, 2}})
	})
	t.Run("outside table", func(t *testing.T) {
		v106TestRun(t, v106TestLesson(t, v106TestText(t, "<A|B>", "<A|B>", 2, 1),
			v106TestOpen(t, "table", 2, 1), v106TestText(t, "Header", "Header", 4, 2), v106TestSeparator(t, 4, 2),
			v106TestText(t, "Body", "Body", 4, 2), v106TestClose(t, 4, 2), v106TestText(t, "<C|D>", "<C|D>", 2, 1)), "current", nil)
	})
}

func TestV106VersionsAndLifecycle(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v106TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v106TestRun(t, result, "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v106TestHeader(t, "C", v106TestText(t, "<A|B>", "<A|B>", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[6].LineEnding = ""
				}
				v106TestRun(t, result, "current", []v106TestExpected{{4, 1, 2}})
			})
		}
	}
	t.Run("separator at EOF without closing table", func(t *testing.T) {
		result := v106TestLesson(t, v106TestOpen(t, "table", 2, 1), v106TestText(t, "<A|B>", "<A|B>", 3, 2),
			v106TestSeparator(t, 3, 2))
		result.Lines[4].LineEnding = ""
		v106TestParserError(t, result, 3, 5, 4, "block")
		v106TestRun(t, result, "current", []v106TestExpected{{4, 1, 2}})
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v106TestHeader(t, "C", v106TestText(t, "<A|B>", "<A|B>", 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v106TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v106TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v106TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v106TestHeader(t, "C", v106TestText(t, "<A|B>", "<A|B>", 3, 2))
		second := v106TestTable(t, v106TestText(t, "<A|B>", "<A|B>", 3, 2))
		v106TestRun(t, first, "first", []v106TestExpected{{4, 1, 2}})
		v106TestRun(t, second, "first", nil)
		v106TestRun(t, first, "second", []v106TestExpected{{4, 1, 2}})
	})
}

// v106TestParserError фиксирует явно заданную ошибку исходного блока или вложения.
func v106TestParserError(t *testing.T, result *model.Result, startLine, endLine, endColumn int, scope diagnostics.Scope) {
	t.Helper()
	result.Lines[startLine-1].HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-context-error", Source: "parser", DiagnosticCode: "fixture-context-error", SeverityLevel: "error", DiagnosticScope: scope,
		Message: "Ошибка структуры блока.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: startLine, Column: 1}, End: model.Position{Line: endLine, Column: endColumn}},
	})
}

// v106TestHeader завершает явно заданный заголовок разделителем и строкой тела.
func v106TestHeader(t *testing.T, body string, rows ...model.Line) *model.Result {
	t.Helper()
	rows = append(rows, v106TestSeparator(t, 3, 2), v106TestText(t, body, body, 3, 2))
	return v106TestTable(t, rows...)
}

// v106TestTable окружает явно заданные строки блоком table.
func v106TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v106TestOpen(t, "table", 2, 1)}, rows...)
	body = append(body, v106TestClose(t, 3, 2))
	return v106TestLesson(t, body...)
}

func v106TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v106TestLine(t, raw, "content", parent, depth,
		v106TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v106TestSeparator(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v106TestLine(t, "---", "separator", parent, depth)
}

// v106TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v106TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v106TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v106TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v106TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v106TestLine(t, "# Lesson", "heading", 0, 0,
			v106TestElement(t, "heading-level", "#", "1", 1, 2), v106TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v106TestOpen задаёт реальное объявление блока с явным родителем.
func v106TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v106TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v106TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v106TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v106TestClose задаёт закрытие указанного блока.
func v106TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v106TestLine(t, "}", "block-end", parent, depth, v106TestElement(t, "block-close", "}", "}", 1, 2))
}

// v106TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v106TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v106TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v106TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v106TestRegistry использует весь штатный каталог регистраций.
func v106TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v106TestRun проверяет только новые V106 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v106TestRun(t *testing.T, result *model.Result, source string, want []v106TestExpected) {
	t.Helper()
	before := v106TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v106TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v106TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V106" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V106 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "В заголовочной части таблицы использована разделённая ячейка." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V106 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v106TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Производные флаги и ссылки проверены CheckModel. В копии возвращаем
	// прежние значения этих полей и удаляем только новые записи запуска.
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
	if v106TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v106TestSnapshot сохраняет независимое представление модели.
func v106TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

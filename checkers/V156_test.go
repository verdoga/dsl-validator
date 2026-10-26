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

// v156Expected хранит явные исходные координаты [start,end), с 1.
type v156Expected struct{ first, column, last, end, count int }

func TestV156Registration(t *testing.T) {
	definitions, err := v156Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V156" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" ||
			definition.Message != "В блоке выбора меньше двух вариантов." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V156 count = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			result := v156Document(t)
			result.Document.DSLVersion = &version
			before := v156Snapshot(t, result)
			if checks, err := validator.Prepare(result, v156Registry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v156Snapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV156ReferenceAndExample(t *testing.T) {
	// Добавлены только версия и заголовок; фрагменты таблицы сохранены буквально.
	for _, tc := range []struct {
		name, tag, fragment string
		body                []model.Line
		want                []v156Expected
	}{
		{"reference", "multichoice", "@multichoice {\n@answer {\nSpain\nFrance\n}\n@hint Both countries are in Europe.\n}",
			[]model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 4), v156TextRow(t, "France", "France", 4),
				v156Close(t, 4), v156Tag(t, "hint", "Both countries are in Europe.", 3)}, nil},
		{"example", "choice", "@choice {\n@answer Spain\n@hint Madrid is its capital.\n}",
			[]model.Line{v156Tag(t, "answer", "Spain", 3), v156Tag(t, "hint", "Madrid is its capital.", 3)}, []v156Expected{{3, 1, 6, 2, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v156Choice(t, tc.tag, tc.body...)
			var rows []string
			for _, line := range result.Lines[2:] {
				rows = append(rows, line.Raw)
			}
			if strings.Join(rows, "\n") != tc.fragment {
				t.Fatal("control fragment changed")
			}
			v156Run(t, result, "current", tc.want)
		})
	}
}

func TestV156Counts(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want []v156Expected
		}{
			{"zero", nil, []v156Expected{{3, 1, 4, 2, 0}}},
			{"one", []model.Line{v156TextRow(t, "Spain", "Spain", 3)}, []v156Expected{{3, 1, 5, 2, 1}}},
			{"two without answers", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156TextRow(t, "France", "France", 3)}, nil},
			{"three", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156TextRow(t, "France", "France", 3), v156TextRow(t, "Italy", "Italy", 3)}, nil},
			{"equal labels", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156TextRow(t, "Spain", "Spain", 3)}, nil},
			{"one inline answer", []model.Line{v156Tag(t, "answer", "Spain", 3)}, []v156Expected{{3, 1, 5, 2, 1}}},
			{"ordinary and answer", []model.Line{v156TextRow(t, "France", "France", 3), v156Tag(t, "answer", "Spain", 3)}, nil},
			{"empty inline answer", []model.Line{v156Tag(t, "answer", "", 3)}, []v156Expected{{3, 1, 5, 2, 0}}},
			{"empty block answer", []model.Line{v156Open(t, "answer", "answer", 3), v156Close(t, 4)}, []v156Expected{{3, 1, 6, 2, 0}}},
			{"one block answer line", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 7, 2, 1}}},
			{"two block answer lines", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 4), v156TextRow(t, "France", "France", 4), v156Close(t, 4)}, nil},
			{"same block answer lines", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 4), v156TextRow(t, "Spain", "Spain", 4), v156Close(t, 4)}, nil},
			{"ordinary and block answer", []model.Line{v156TextRow(t, "France", "France", 3), v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 5), v156Close(t, 5)}, nil},
			{"blanks between variants", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Line(t, "", "blank", 3), v156TextRow(t, "France", "France", 3)}, nil},
			{"blank whitespace", []model.Line{v156Line(t, " \t ", "blank", 3)}, []v156Expected{{3, 1, 5, 2, 0}}},
			{"ordinary NBSP counts", []model.Line{v156TextRow(t, "\u00a0", "\u00a0", 3)}, []v156Expected{{3, 1, 5, 2, 1}}},
			{"answer NBSP does not count", []model.Line{v156Tag(t, "answer", "\u00a0", 3)}, []v156Expected{{3, 1, 5, 2, 0}}},
			{"block answer whitespace", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "\t\u00a0 ", "\u00a0", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 7, 2, 0}}},
			{"escaped ordinary text", []model.Line{v156TextRow(t, `\@hint Spain`, "@hint Spain", 3), v156TextRow(t, `\{France\}`, "{France}", 3)}, nil},
			{"escaped block answer text", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, `\\`, `\`, 4), v156TextRow(t, `\# Label`, "# Label", 4), v156Close(t, 4)}, nil},
			{"punctuation answer", []model.Line{v156Tag(t, "answer", "; / ,", 3)}, []v156Expected{{3, 1, 5, 2, 1}}},
			{"unknown escape is not erased", []model.Line{v156Tag(t, "answer", `\ `, 3)}, []v156Expected{{3, 1, 5, 2, 1}}},
			{"one line with multiple content elements", []model.Line{v156Line(t, "Spain France", "content", 3, v156TestElement(t, "content", "Spain", "Spain", 1), v156TestElement(t, "content", "France", "France", 7))}, []v156Expected{{3, 1, 5, 2, 1}}},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) { v156Run(t, v156Choice(t, tag, tc.body...), "current", tc.want) })
		}
	}
	t.Run("all correct multichoice", func(t *testing.T) {
		v156Run(t, v156Choice(t, "multichoice", v156Tag(t, "answer", "Spain", 3), v156Tag(t, "answer", "France", 3)), "current", nil)
	})
}

func TestV156UnattachedBlanks(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want []v156Expected
		}{
			{"before option", []model.Line{v156Line(t, " \t", "blank", 3), v156TextRow(t, "Spain", "Spain", 3)}, []v156Expected{{3, 1, 6, 2, 1}}},
			{"after option", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Line(t, "", "blank", 3)}, []v156Expected{{3, 1, 6, 2, 1}}},
			{"empty choice", []model.Line{v156Line(t, "\t ", "blank", 3)}, []v156Expected{{3, 1, 5, 2, 0}}},
			{"two options", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Line(t, " \t", "blank", 3), v156TextRow(t, "France", "France", 3)}, nil},
			{"inline answer", []model.Line{v156Line(t, "", "blank", 3), v156Tag(t, "answer", "Spain", 3)}, []v156Expected{{3, 1, 6, 2, 1}}},
			{"one block answer line", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 4), v156Line(t, " \t", "blank", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 8, 2, 1}}},
			{"before block answer line", []model.Line{v156Open(t, "answer", "answer", 3), v156Line(t, "", "blank", 4), v156TextRow(t, "Spain", "Spain", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 8, 2, 1}}},
			{"two block answer lines", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 4), v156Line(t, " \t", "blank", 4), v156TextRow(t, "France", "France", 4), v156Close(t, 4)}, nil},
			{"empty block answer", []model.Line{v156Open(t, "answer", "answer", 3), v156Line(t, "", "blank", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 7, 2, 0}}},
			{"excluded hint", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Open(t, "hint", "hint", 3), v156TextRow(t, "Think.", "Think.", 5), v156Line(t, " \t", "blank", 5), v156Close(t, 5)}, []v156Expected{{3, 1, 9, 2, 1}}},
			{"nested note in answer", []model.Line{v156Open(t, "answer", "answer", 3), v156Open(t, "note", "note", 4), v156TextRow(t, "Spain", "Spain", 5), v156Line(t, "", "blank", 5), v156Close(t, 5), v156Close(t, 4)}, []v156Expected{{3, 1, 10, 2, 0}}},
		} {
			for _, root := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/root=%t", tag, tc.name, root), func(t *testing.T) {
					body := append([]model.Line(nil), tc.body...)
					for i := range body {
						if root && body[i].LineType == "blank" {
							body[i].ParentLine = nil
						}
					}
					v156Run(t, v156Choice(t, tag, body...), "current", tc.want)
				})
			}
		}
	}
}

func TestV156FollowingBlocks(t *testing.T) {
	rows := []model.Line{v156Version(t), v156Heading(t)}
	var want []v156Expected
	for i := 0; i < 128; i++ {
		opening := 3 + 5*i
		rows = append(rows, v156Open(t, "choice", "choice", 2), v156Open(t, "answer", "answer", opening),
			v156TextRow(t, "Spain", "Spain", opening+1), v156Close(t, opening+1), v156Close(t, opening))
		want = append(want, v156Expected{opening, 1, opening + 4, 2, 1})
	}
	v156Run(t, v156Document(t, rows...), "current", want)
}

func TestV156WhitespaceRules(t *testing.T) {
	// Явный набор Unicode White_Space без CR/LF, которые находятся вне Raw.
	for _, char := range []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003',
		'\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(fmt.Sprintf("answer U+%04X", char), func(t *testing.T) {
			v156Run(t, v156Choice(t, "multichoice", v156Tag(t, "answer", string(char), 3)), "current", []v156Expected{{3, 1, 5, 2, 0}})
			v156Run(t, v156Choice(t, "choice", v156Open(t, "answer", "answer", 3), v156TextRow(t, string(char), string(char), 4), v156Close(t, 4)), "current", []v156Expected{{3, 1, 7, 2, 0}})
		})
	}
	for _, raw := range []string{"\u200b", "\ufeff", "\u0301", "😀", "Я"} {
		t.Run(fmt.Sprintf("nonspace %q", raw), func(t *testing.T) {
			v156Run(t, v156Choice(t, "choice", v156Tag(t, "answer", raw, 3)), "current", []v156Expected{{3, 1, 5, 2, 1}})
		})
	}
}

func TestV156ExcludedContent(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want []v156Expected
		}{
			{"hint alone", []model.Line{v156Tag(t, "hint", "Spain", 3)}, []v156Expected{{3, 1, 5, 2, 0}}},
			{"hint before answer", []model.Line{v156Tag(t, "hint", "Spain", 3), v156Tag(t, "answer", "France", 3)}, []v156Expected{{3, 1, 6, 2, 1}}},
			{"block hint", []model.Line{v156Open(t, "hint", "hint", 3), v156TextRow(t, "Spain", "Spain", 4), v156TextRow(t, "France", "France", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 8, 2, 0}}},
			{"other nested block", []model.Line{v156Open(t, "note", "note", 3), v156TextRow(t, "Spain", "Spain", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 7, 2, 0}}},
			{"unparsed inside hint", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Open(t, "hint", "hint", 3), v156Invalid(t, "broken {", 5), v156Close(t, 5)}, []v156Expected{{3, 1, 8, 2, 1}}},
			{"nested tag in answer", []model.Line{v156Open(t, "answer", "answer", 3), v156Tag(t, "hint", "Spain", 4), v156Close(t, 4)}, []v156Expected{{3, 1, 7, 2, 0}}},
			{"nested block in answer", []model.Line{v156Open(t, "answer", "answer", 3), v156Open(t, "note", "note", 4), v156TextRow(t, "Spain", "Spain", 5), v156Close(t, 5), v156Close(t, 4)}, []v156Expected{{3, 1, 9, 2, 0}}},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) { v156Run(t, v156Choice(t, tag, tc.body...), "current", tc.want) })
		}
	}
	t.Run("instruction and case", func(t *testing.T) {
		result := v156Document(t, v156Version(t), v156Heading(t),
			v156Line(t, "  @CHOICE Choose. {", "block-start", 2, v156TestElement(t, "tag", "@CHOICE", "choice", 3), v156TestElement(t, "content", "Choose.", "Choose.", 11), v156TestElement(t, "block-open", "{", "{", 19)),
			v156TextRow(t, "Spain", "Spain", 3), v156Close(t, 3))
		v156Run(t, result, "current", []v156Expected{{3, 3, 5, 2, 1}})
	})
	t.Run("literal pseudo declaration", func(t *testing.T) {
		v156Run(t, v156Document(t, v156Version(t), v156Heading(t), v156Open(t, "editor", "editor", 2),
			v156TextRow(t, `@choice \{`, "@choice {", 3), v156TextRow(t, "Spain", "Spain", 3), v156TextRow(t, `\}`, "}", 3), v156Close(t, 3)), "current", nil)
	})
}

func TestV156UnknownContent(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		for _, tc := range []struct {
			name string
			body []model.Line
		}{
			{"invalid possible option", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Invalid(t, "@unknown", 3)}},
			{"partially parsed option", []model.Line{v156Line(t, "Spain {", "content", 3, v156TestElement(t, "content", "Spain", "Spain", 1), v156TestElement(t, "unparsed", "{", "", 7))}},
			{"unparsed inline answer", []model.Line{v156Line(t, "@answer {oops", "tag", 3, v156TestElement(t, "tag", "@answer", "answer", 1), v156TestElement(t, "unparsed", "{oops", "", 9))}},
			{"unparsed block answer", []model.Line{v156Open(t, "answer", "answer", 3), v156Invalid(t, "@unknown", 4), v156Close(t, 4)}},
			{"unparsed answer whitespace", []model.Line{v156Open(t, "answer", "answer", 3), v156Invalid(t, "\u00a0", 4), v156Close(t, 4)}},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) { v156Run(t, v156Choice(t, tag, tc.body...), "current", nil) })
		}
	}
	t.Run("unknown outside block", func(t *testing.T) {
		v156Run(t, v156Document(t, v156Version(t), v156Heading(t), v156Open(t, "choice", "choice", 2), v156Close(t, 3), v156Invalid(t, "@unknown", 2)), "current", []v156Expected{{3, 1, 4, 2, 0}})
	})
}

func TestV156UncertainBoundaries(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		for _, tc := range []struct {
			name string
			body []model.Line
		}{
			{"foreign content", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156TextRow(t, "France", "France", 0)}},
			{"unparsed root whitespace", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Invalid(t, " \t", 0)}},
			{"blank with unparsed whitespace", []model.Line{v156TextRow(t, "Spain", "Spain", 3), v156Line(t, " \t", "blank", 0, v156TestElement(t, "unparsed", " \t", "", 1))}},
			{"noncontiguous answer", []model.Line{v156Open(t, "answer", "answer", 3), v156TextRow(t, "Spain", "Spain", 3), v156Close(t, 4)}},
			{"noncontiguous excluded hint", []model.Line{v156Open(t, "hint", "hint", 3), v156TextRow(t, "Spain", "Spain", 3), v156Close(t, 4)}},
			{"nested duplicate closing", []model.Line{v156Open(t, "hint", "hint", 3), v156Close(t, 4), v156Close(t, 4)}},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) { v156Run(t, v156Choice(t, tag, tc.body...), "current", nil) })
		}
		t.Run(tag+"/unclosed", func(t *testing.T) {
			result := v156Document(t, v156Version(t), v156Heading(t), v156Open(t, tag, tag, 2), v156TextRow(t, "Spain", "Spain", 3))
			v156Recovery(t, result, 3)
			v156Run(t, result, "current", nil)
		})
		t.Run(tag+"/duplicate closing", func(t *testing.T) {
			result := v156Document(t, v156Version(t), v156Heading(t), v156Open(t, tag, tag, 2), v156Close(t, 3), v156Close(t, 3))
			v156Recovery(t, result, 5)
			v156Run(t, result, "current", nil)
		})
		t.Run(tag+"/nested block not closed", func(t *testing.T) {
			result := v156Choice(t, tag, v156Open(t, "hint", "hint", 3))
			v156Recovery(t, result, 4)
			v156Run(t, result, "current", nil)
		})
		t.Run(tag+"/child after closing", func(t *testing.T) {
			result := v156Document(t, v156Version(t), v156Heading(t), v156Open(t, tag, tag, 2), v156Close(t, 3), v156TextRow(t, "Spain", "Spain", 3))
			v156Recovery(t, result, 5)
			v156Run(t, result, "current", nil)
		})
		t.Run(tag+"/closing has unparsed tail", func(t *testing.T) {
			v156Run(t, v156Document(t, v156Version(t), v156Heading(t), v156Open(t, tag, tag, 2),
				v156Line(t, "} extra", "block-end", 3, v156TestElement(t, "block-close", "}", "}", 1), v156TestElement(t, "unparsed", "extra", "", 3))), "current", nil)
		})
		t.Run(tag+"/no opening", func(t *testing.T) {
			v156Run(t, v156Document(t, v156Version(t), v156Heading(t), v156Tag(t, tag, "", 2)), "current", nil)
		})
	}
}

func TestV156ContextsAndHistory(t *testing.T) {
	t.Run("case of answer and hint", func(t *testing.T) {
		result := v156Choice(t, "multichoice",
			v156Line(t, "@ANSWER Spain", "tag", 3, v156TestElement(t, "tag", "@ANSWER", "answer", 1), v156TestElement(t, "content", "Spain", "Spain", 9)),
			v156Line(t, "@HINT Think.", "tag", 3, v156TestElement(t, "tag", "@HINT", "hint", 1), v156TestElement(t, "content", "Think.", "Think.", 7)))
		v156Run(t, result, "current", []v156Expected{{3, 1, 6, 2, 1}})
	})
	for _, tag := range []string{"choice", "multichoice"} {
		t.Run("variant/"+tag, func(t *testing.T) {
			result := v156Document(t, v156Version(t), v156Heading(t), v156Open(t, "variants", "variants", 2),
				v156Line(t, "@variant A", "tag", 3, v156TestElement(t, "tag", "@variant", "variant", 1), v156TestElement(t, "name", "A", "A", 10)),
				v156Open(t, tag, tag, 4), v156TextRow(t, "Spain", "Spain", 5), v156Close(t, 5),
				v156Line(t, "@variant B", "tag", 3, v156TestElement(t, "tag", "@variant", "variant", 1), v156TestElement(t, "name", "B", "B", 10)),
				v156TextRow(t, "France", "France", 8), v156Close(t, 3))
			v156Run(t, result, "current", []v156Expected{{5, 1, 7, 2, 1}})
		})
	}
	t.Run("empty input", func(t *testing.T) { v156Run(t, v156Document(t), "current", nil) })
	t.Run("task", func(t *testing.T) {
		result := v156Document(t, v156Version(t), v156Heading(t),
			v156Line(t, "@task t", "tag", 2, v156TestElement(t, "tag", "@task", "task", 1), v156TestElement(t, "identifier", "t", "t", 7)),
			v156Open(t, "choice", "choice", 3), v156TextRow(t, "Spain", "Spain", 4), v156Close(t, 4), v156Tag(t, "endtask", "", 3))
		v156Run(t, result, "current", []v156Expected{{4, 1, 6, 2, 1}})
	})
	t.Run("nested choices checked independently", func(t *testing.T) {
		v156Run(t, v156Choice(t, "choice", v156Open(t, "multichoice", "multichoice", 3), v156Close(t, 4)), "current", []v156Expected{{3, 1, 6, 2, 0}, {4, 1, 5, 2, 0}})
	})
	t.Run("first and last blocks", func(t *testing.T) {
		v156Run(t, v156Document(t, v156Version(t), v156Heading(t), v156Open(t, "choice", "choice", 2), v156TextRow(t, "Spain", "Spain", 3), v156Close(t, 3),
			v156Open(t, "multichoice", "multichoice", 2), v156Tag(t, "answer", "France", 6), v156Close(t, 6)), "current", []v156Expected{{3, 1, 5, 2, 1}, {6, 1, 8, 2, 1}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", eol), func(t *testing.T) {
			result := v156Choice(t, "choice", v156Tag(t, "answer", "Spain", 3), v156Tag(t, "hint", "Madrid is its capital.", 3))
			for i := range result.Lines {
				if eol != "" {
					result.Lines[i].LineEnding = eol
				}
			}
			result.Lines[len(result.Lines)-1].LineEnding = eol
			want := []v156Expected{{3, 1, 6, 2, 1}}
			v156Run(t, result, "first", want)
			v156Run(t, v156Choice(t, "choice", v156TextRow(t, "A", "A", 3), v156TextRow(t, "B", "B", 3)), "other", nil)
			v156Run(t, result, "second", want)
		})
	}
}

// v156Tag задаёт однострочное объявление и его текстовое значение явно.
func v156Tag(t *testing.T, name, content string, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v156TestElement(t, "tag", raw, name, 1)}
	if content != "" {
		elements = append(elements, v156TestElement(t, "content", content, content, len(raw)+2))
		raw += " " + content
	}
	return v156Line(t, raw, "tag", parent, elements...)
}

// v156TextRow сохраняет исходное и семантическое значения без разбора.
func v156TextRow(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v156Line(t, raw, "content", parent, v156TestElement(t, "content", raw, value, 1))
}

// v156Recovery добавляет исходную ошибку восстановления структуры в согласованную модель.
func v156Recovery(t *testing.T, result *model.Result, number int) {
	t.Helper()
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: fmt.Sprintf("recovery-%d", number), Source: "parser", DiagnosticCode: "P002",
		SeverityLevel: "error", DiagnosticScope: "line", Message: "Неоднозначная граница блока.",
		Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}}, RelatedLocations: []model.Location{}})
	result.Document.HasErrors = true
	result.Lines[number-1].HasErrors = true
}

// v156Invalid задаёт восстановленную повреждённую строку с unparsed.
func v156Invalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v156Line(t, raw, "invalid", parent, v156TestElement(t, "unparsed", raw, "", 1))
}

// v156Open создаёт открытие блока с исходным и каноническим именем.
func v156Open(t *testing.T, rawName, name string, parent int) model.Line {
	t.Helper()
	return v156Line(t, "@"+rawName+" {", "block-start", parent,
		v156TestElement(t, "tag", "@"+rawName, name, 1), v156TestElement(t, "block-open", "{", "{", len(rawName)+3))
}

// v156Close создаёт структурное закрытие указанного блока.
func v156Close(t *testing.T, parent int) model.Line {
	t.Helper()
	return v156Line(t, "}", "block-end", parent, v156TestElement(t, "block-close", "}", "}", 1))
}

// v156Choice добавляет контекст документа и границы выбора; тело начинается на строке 4.
func v156Choice(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{v156Version(t), v156Heading(t), v156Open(t, name, name, 2)}, body...)
	return v156Document(t, append(rows, v156Close(t, 3))...)
}

// v156TestElement создаёт элемент модели; диапазоны ожиданий задаются отдельно.
func v156TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v156Line задаёт тип и родителя без повторного разбора исходного текста.
func v156Line(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v156Version задаёт технический тег версии для контекста примеров.
func v156Version(t *testing.T) model.Line {
	t.Helper()
	return v156Line(t, "@dsl-version 1.2", "tag", 0,
		v156TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v156TestElement(t, "version", "1.2", "1.2", 14))
}

// v156Heading добавляет обязательный заголовок документа перед фрагментом.
func v156Heading(t *testing.T) model.Line {
	t.Helper()
	return v156Line(t, "# Lesson", "heading", 0,
		v156TestElement(t, "heading-level", "#", "1", 1), v156TestElement(t, "title", "Lesson", "Lesson", 3))
}

// v156Document оформляет заданную модель и обязательные исходные ссылки unparsed.
// Причины повреждения не восстанавливаются: классификация передана тестом явно.
func v156Document(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	version, count := "1.2", len(rows)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines: append([]model.Line{}, rows...), Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			if *line.ParentLine < 1 || *line.ParentLine >= line.Number {
				t.Fatal("invalid fixture parent")
			}
			line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
		}
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", line.Number, j)
			element.ErrorIDs = []string{id}
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неразобранный фрагмент.",
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}}, RelatedLocations: []model.Location{}})
			line.HasErrors, result.Document.HasErrors = true, true
		}
	}
	return result
}

// v156Registry создаёт реестр из полного штатного каталога.
func v156Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v156Snapshot сохраняет модель для проверки неизменности истории и исходных данных.
func v156Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v156LineReads измеряет обращения проверки к публичному построчному входу.
type v156LineReads struct {
	diagnostics.Lines
	reads int
}

func (lines *v156LineReads) Line(number int) (diagnostics.Line, bool) {
	lines.reads++
	return lines.Lines.Line(number)
}

// v156Run запускает штатный конвейер и проверяет только новые срабатывания V156.
func v156Run(t *testing.T, result *model.Result, processing string, want []v156Expected) {
	t.Helper()
	before := v156Snapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v156Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(checks) != 198 {
		t.Fatalf("catalogue size = %d, want 198", len(checks))
	}
	if v156Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	var reads int
	for i := range checks {
		if checks[i].Code == "V156" {
			check := checks[i].Check
			checks[i].Check = func(lines diagnostics.Lines, findings diagnostics.Findings) error {
				observed := &v156LineReads{Lines: lines}
				err := check(observed, findings)
				reads = observed.reads
				return err
			}
		}
	}
	result.Processing = append(result.Processing, model.Processing{ID: processing, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processing); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Запас на несколько проходов не допускает повторных обходов хвоста и предков.
	if limit := 16*len(result.Lines) + 16; reads > limit {
		t.Errorf("V156 read %d lines, want at most %d for %d source lines", reads, limit, len(result.Lines))
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V156" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V156 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.first, Column: expected.column},
			End: model.Position{Line: expected.last, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processing ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != fmt.Sprintf("В блоке выбора меньше двух вариантов.\nконкретизация: Количество вариантов: %d", expected.count) {
			t.Fatalf("V156 occurrence = %+v, want %+v", entry, expected)
		}
		if !result.Document.HasErrors || !result.Lines[expected.first-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("block scope created element link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("output integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v156Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Только производные флаги, новые ссылки и записи исключаются из сравнения.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("old element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v156Snapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

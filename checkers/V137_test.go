package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV137RegistrationAndVersions(t *testing.T) {
	registry := v137TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V137" {
			continue
		}
		count++
		if definition.Message != "Блок @answer не содержит непустых ответов." ||
			definition.Severity != "error" || definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V137 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v137TestAnswer(t)
			result.Document.DSLVersion = &version
			before := v137TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v137TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV137ReferenceAndExample(t *testing.T) {
	// По DSL 1.30 перед неизменными фрагментами добавлены версия, заголовок и шаг.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][4]int
	}{
		{"reference", "@question Where does Tom live?\n@answer {\nLondon\nParis\n}",
			[]model.Line{v137TestText(t, "London", "London", 5), v137TestText(t, "Paris", "Paris", 5)}, nil},
		{"example", "@question Where does Tom live?\n@answer {\n}", nil, [][4]int{{5, 1, 6, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v137TestAnswer(t, tc.body...)
			var source []string
			for _, line := range result.Lines[3:] {
				source = append(source, line.Raw)
			}
			if strings.Join(source, "\n") != tc.fragment {
				t.Fatal("control fragment changed")
			}
			added := v137TestRun(t, result, "current", tc.want)
			if tc.name == "example" {
				var got []model.Diagnostic
				for _, entry := range added {
					if entry.DiagnosticCode == "V035" {
						got = append(got, entry)
					}
				}
				location := model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 6, Column: 2}}
				if len(got) != 1 || got[0].Location == nil || *got[0].Location != location {
					t.Fatalf("existing V035 was suppressed or changed: %+v", got)
				}
			}
		})
	}
}

func TestV137TextValues(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][4]int
	}{
		{"ordinary answer", "London", "London", nil},
		{"Unicode whitespace", " \t\u00a0 ", "\u00a0", [][4]int{{5, 1, 7, 2}}},
		{"punctuation", "...", "...", nil},
		{"literal separator", "---", "---", nil},
		{"letter", "Я", "Я", nil},
		{"emoji", "😀", "😀", nil},
		{"zero width space is not White_Space", "\u200b", "\u200b", nil},
		{"BOM inside text is not whitespace", "\ufeff", "\ufeff", nil},
		{"opening brace", `\{`, "{", nil},
		{"closing brace", `\}`, "}", nil},
		{"literal backslash", `\\`, `\`, nil},
		{"escaped tag", `\@answer`, "@answer", nil},
		{"escaped heading", `\#Title`, "#Title", nil},
		{"unknown escape", `\q`, `\q`, nil},
		{"unknown whitespace escape", "\\ \t", "\\", nil},
		{"pair then escaped brace", `\\\{`, `\{`, nil},
		{"escaped leading tag after indentation", " \t\\@answer \t", "@answer", nil},
		{"HTML is text in answer", "<em></em>", "<em></em>", nil},
		{"entity is not decoded", "&nbsp;", "&nbsp;", nil},
		{"underscores are answer data", "_____", "_____", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v137TestRun(t, v137TestAnswer(t, v137TestText(t, tc.raw, tc.value, 5)), "current", tc.want)
		})
	}
	// Явный набор Unicode White_Space без CR/LF, являющихся границами строк DSL.
	for _, source := range []string{"\t", "\v", "\f", " ", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007",
		"\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("whitespace "+strconv.QuoteToASCII(source), func(t *testing.T) {
			row := v137TestText(t, source, source, 5)
			if source == " " || source == "\t" {
				row = v137TestLine(t, source, "blank", 5)
			}
			v137TestRun(t, v137TestAnswer(t, row), "current", [][4]int{{5, 1, 7, 2}})
		})
	}
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"blank", []model.Line{v137TestLine(t, "", "blank", 5)}, [][4]int{{5, 1, 7, 2}}},
		{"several blanks", []model.Line{v137TestLine(t, "", "blank", 5), v137TestLine(t, " \t ", "blank", 5)}, [][4]int{{5, 1, 8, 2}}},
		{"answer between blanks", []model.Line{v137TestLine(t, "", "blank", 5), v137TestText(t, "London", "London", 5),
			v137TestLine(t, "", "blank", 5)}, nil},
		{"last answer", []model.Line{v137TestLine(t, "", "blank", 5), v137TestText(t, "Paris", "Paris", 5)}, nil},
		{"first answer", []model.Line{v137TestText(t, "Paris", "Paris", 5), v137TestLine(t, "", "blank", 5)}, nil},
		{"separate text parts in one row", []model.Line{v137TestLine(t, "\u00a0 \u2003", "content", 5,
			v137TestElement(t, "content", "\u00a0", "\u00a0", 1), v137TestElement(t, "content", "\u2003", "\u2003", 3))}, [][4]int{{5, 1, 7, 2}}},
		{"line boundary does not continue escape", []model.Line{v137TestText(t, `\\`, `\`, 5),
			v137TestText(t, "\u00a0", "\u00a0", 5)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v137TestRun(t, v137TestAnswer(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV137ReadyContentValues(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][4]int
	}{
		{"nonempty source with empty value", "Source", "", [][4]int{{5, 1, 7, 2}}},
		{"nonempty source with Unicode whitespace value", "Source", " \t\u00a0\u1680\u2003\u202f\u3000 ", [][4]int{{5, 1, 7, 2}}},
		{"Unicode whitespace source with answer value", "\u00a0\u2003", "London", nil},
		{"ASCII whitespace source with answer value", " \t ", "Paris", nil},
		{"whitespace source with punctuation value", "\u3000", " \u2003…!?\u00a0 ", nil},
		{"nonempty source with zero width space value", "Source", "\u200b", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := v137TestText(t, tc.raw, tc.value, 5)
			v137TestRun(t, v137TestAnswer(t, row), "current", tc.want)
		})
	}
}

func TestV137CompositeReadyContentValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values [3]string
		want   [][4]int
	}{
		{"all values empty", [3]string{"", "", ""}, [][4]int{{5, 1, 7, 2}}},
		{"all values Unicode whitespace", [3]string{"\u00a0", "\t\u2003", "\u202f\u3000"}, [][4]int{{5, 1, 7, 2}}},
		{"first value is an answer", [3]string{"London", "\u2003", ""}, nil},
		{"middle value is an answer", [3]string{"\u00a0", "London", ""}, nil},
		{"last value is an answer", [3]string{"\u00a0", "", "London"}, nil},
		{"punctuation value is an answer", [3]string{"\u00a0", "…", "\u2003"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := v137TestLine(t, "A B C", "content", 5,
				v137TestElement(t, "content", "A", tc.values[0], 1),
				v137TestElement(t, "content", "B", tc.values[1], 3),
				v137TestElement(t, "content", "C", tc.values[2], 5))
			v137TestRun(t, v137TestAnswer(t, row), "current", tc.want)
		})
	}
	t.Run("whitespace source fragments with nonempty final value", func(t *testing.T) {
		row := v137TestLine(t, "\u00a0 \u2003", "content", 5,
			v137TestElement(t, "content", "\u00a0", "", 1),
			v137TestElement(t, "content", "\u2003", "Paris", 3))
		v137TestRun(t, v137TestAnswer(t, row), "current", nil)
	})
	t.Run("uncovered Unicode whitespace remains unknown", func(t *testing.T) {
		row := v137TestLine(t, "A\u00a0B", "content", 5,
			v137TestElement(t, "content", "A", "", 1),
			v137TestElement(t, "content", "B", "\u2003", 3))
		v137TestRun(t, v137TestAnswer(t, row), "current", nil)
	})
	t.Run("unparsed fragment remains unknown", func(t *testing.T) {
		row := v137TestLine(t, "A ? B", "content", 5,
			v137TestElement(t, "content", "A", "\u00a0", 1),
			v137TestElement(t, "unparsed", "?", "", 3),
			v137TestElement(t, "content", "B", "", 5))
		v137TestRun(t, v137TestAnswer(t, row), "current", nil)
	})
}

func TestV137EmptyReadyValuesAndUnknownRows(t *testing.T) {
	for _, unknownFirst := range []bool{false, true} {
		t.Run("unknown first="+strconv.FormatBool(unknownFirst), func(t *testing.T) {
			body := []model.Line{v137TestText(t, "Source", "\u00a0\u2003", 5), v137TestInvalid(t, "?", 5)}
			if unknownFirst {
				body[0], body[1] = body[1], body[0]
			}
			v137TestRun(t, v137TestAnswer(t, body...), "current", nil)
		})
	}
	t.Run("nested tag value is excluded", func(t *testing.T) {
		v137TestRun(t, v137TestAnswer(t, v137TestText(t, "Source", "\u00a0", 5),
			v137TestTag(t, "note", "London", "content", 5)), "current", [][4]int{{5, 1, 8, 2}})
	})
	t.Run("unknown row inside known nested block is excluded", func(t *testing.T) {
		v137TestRun(t, v137TestAnswer(t, v137TestText(t, "Source", "\u2003", 5),
			v137TestOpen(t, "note", 5), v137TestInvalid(t, "?", 7), v137TestClose(t, 7)),
			"current", [][4]int{{5, 1, 10, 2}})
	})
}

func TestV137ZeroWidthContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		elements  []model.Element
		want      [][4]int
	}{
		{"inside nonempty text", "Yes", []model.Element{v137TestElement(t, "content", "Yes", "Yes", 1),
			v137TestElement(t, "content", "", "", 3)}, nil},
		{"before text", "Yes", []model.Element{v137TestElement(t, "content", "", "", 1),
			v137TestElement(t, "content", "Yes", "Yes", 1)}, nil},
		{"at text end", "Yes", []model.Element{v137TestElement(t, "content", "Yes", "Yes", 1),
			v137TestElement(t, "content", "", "", 4)}, nil},
		{"after text", "Yes \t", []model.Element{v137TestElement(t, "content", "Yes", "Yes", 1),
			v137TestElement(t, "content", "", "", 6)}, nil},
		{"inside first text fragment", "SPAIN!", []model.Element{v137TestElement(t, "content", "SPA", "SPA", 1),
			v137TestElement(t, "content", "", "", 2), v137TestElement(t, "content", "IN!", "IN!", 4)}, nil},
		{"inside whitespace before another fragment", "\u00a0\u2003\t", []model.Element{
			v137TestElement(t, "content", "\u00a0\u2003", "\u00a0\u2003", 1), v137TestElement(t, "content", "", "", 2),
			v137TestElement(t, "content", "\t", "\t", 3)}, [][4]int{{5, 1, 7, 2}}},
		{"empty annotations do not cover whitespace", "\u00a0", []model.Element{
			v137TestElement(t, "content", "", "", 1), v137TestElement(t, "content", "", "", 2)}, nil},
		{"empty annotation does not cover trailing whitespace", "\u00a0\u2003", []model.Element{
			v137TestElement(t, "content", "\u00a0", "\u00a0", 1), v137TestElement(t, "content", "", "", 2)}, nil},
		{"empty unparsed remains unknown", "\u00a0\u2003", []model.Element{
			v137TestElement(t, "content", "\u00a0\u2003", "\u00a0\u2003", 1), v137TestElement(t, "unparsed", "", "", 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := v137TestLine(t, tc.raw, "content", 5, tc.elements...)
			v137TestRun(t, v137TestAnswer(t, row), "current", tc.want)
		})
	}
}

func TestV137RootBlankInsideClosedAnswer(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"root empty blank", []model.Line{v137TestLine(t, "", "blank", 0)}, [][4]int{{5, 1, 7, 2}}},
		{"root ASCII whitespace blank", []model.Line{v137TestLine(t, " \t ", "blank", 0)}, [][4]int{{5, 1, 7, 2}}},
		{"parented blank control", []model.Line{v137TestLine(t, "", "blank", 5)}, [][4]int{{5, 1, 7, 2}}},
		{"mixed blank parents", []model.Line{v137TestLine(t, "", "blank", 0), v137TestLine(t, "\t", "blank", 5),
			v137TestLine(t, " ", "blank", 0)}, [][4]int{{5, 1, 9, 2}}},
		{"nonempty answer between root blanks", []model.Line{v137TestLine(t, "", "blank", 0),
			v137TestText(t, "Yes", "Yes", 5), v137TestLine(t, "", "blank", 0)}, nil},
		{"root content is not a structural blank", []model.Line{v137TestText(t, "", "", 0)}, nil},
		{"blank linked to a different parent", []model.Line{v137TestLine(t, "", "blank", 3)}, nil},
		{"nonempty root blank is unknown", []model.Line{v137TestLine(t, "Yes", "blank", 0,
			v137TestElement(t, "content", "Yes", "Yes", 1))}, nil},
		{"unparsed root blank is unknown", []model.Line{v137TestLine(t, " ", "blank", 0,
			v137TestElement(t, "unparsed", " ", "", 1))}, nil},
		{"root blank inside nested block", []model.Line{v137TestOpen(t, "note", 5),
			v137TestLine(t, "", "blank", 0), v137TestClose(t, 6)}, [][4]int{{5, 1, 9, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v137TestRun(t, v137TestAnswer(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("unclosed answer with root blank", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestOpen(t, "answer", 3), v137TestLine(t, "", "blank", 0))
		v137TestParserError(t, result, 5, -1)
		v137TestRun(t, result, "current", nil)
	})
	t.Run("ambiguous closing with root blank", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestOpen(t, "answer", 3), v137TestLine(t, "", "blank", 0), v137TestClose(t, 5), v137TestClose(t, 5))
		v137TestParserError(t, result, 8, -1)
		v137TestRun(t, result, "current", nil)
	})
}

func TestV137IndependentBlocksLineReads(t *testing.T) {
	for _, count := range []int{8, 32, 128} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			var body []model.Line
			var want [][4]int
			for i := 0; i < count; i++ {
				// После трёх строк окружения каждый блок занимает строки
				// 4+3*i (question), 5+3*i (answer), 6+3*i (закрытие).
				body = append(body, v137TestTag(t, "question", "Where?", "content", 3),
					v137TestOpen(t, "answer", 3), v137TestClose(t, 5+3*i))
				want = append(want, [4]int{5 + 3*i, 1, 6 + 3*i, 2})
			}
			v137TestRun(t, v137TestDocument(t, body...), "current", want)
		})
	}
}

func TestV137IndexedBoundaryConservatism(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"root blank after closing is unrelated", []model.Line{v137TestOpen(t, "answer", 3),
			v137TestClose(t, 5), v137TestLine(t, "", "blank", 0)}, [][4]int{{5, 1, 6, 2}}},
		{"direct descendant after closing", []model.Line{v137TestOpen(t, "answer", 3),
			v137TestClose(t, 5), v137TestLine(t, "", "blank", 5)}, nil},
		{"missing interior line and extra descendant do not cancel", []model.Line{v137TestOpen(t, "answer", 3),
			v137TestText(t, "", "", 0), v137TestClose(t, 5), v137TestLine(t, "", "blank", 5)}, nil},
		{"nested boundary uncertainty suppresses outer answer", []model.Line{v137TestOpen(t, "answer", 3),
			v137TestOpen(t, "note", 5), v137TestClose(t, 6), v137TestLine(t, "", "blank", 6), v137TestClose(t, 5)}, nil},
		{"deep descendant after answer closing", []model.Line{v137TestOpen(t, "answer", 3),
			v137TestOpen(t, "note", 5), v137TestClose(t, 6), v137TestClose(t, 5), v137TestLine(t, "", "blank", 6)}, nil},
		{"earlier uncertain block does not suppress later answer", []model.Line{v137TestOpen(t, "note", 3),
			v137TestClose(t, 5), v137TestLine(t, "", "blank", 5), v137TestOpen(t, "answer", 3),
			v137TestLine(t, "", "blank", 0), v137TestClose(t, 8)}, [][4]int{{8, 1, 10, 2}}},
		{"later uncertain block does not suppress earlier answer", []model.Line{v137TestOpen(t, "answer", 3),
			v137TestClose(t, 5), v137TestOpen(t, "note", 3), v137TestClose(t, 7),
			v137TestLine(t, "", "blank", 7)}, [][4]int{{5, 1, 6, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v137TestTag(t, "question", "Where?", "content", 3)}, tc.body...)
			v137TestRun(t, v137TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV137NestedDeclarationsAndContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"inline nested tag is not an answer", []model.Line{v137TestTag(t, "note", "London", "content", 5)}, [][4]int{{5, 1, 7, 2}}},
		{"nested block text is not an answer", []model.Line{v137TestOpen(t, "note", 5),
			v137TestText(t, "London", "London", 6), v137TestClose(t, 6)}, [][4]int{{5, 1, 9, 2}}},
		{"unparsed text of known nested block is excluded", []model.Line{v137TestOpen(t, "note", 5),
			v137TestInvalid(t, "@unknown", 6), v137TestClose(t, 6)}, [][4]int{{5, 1, 9, 2}}},
		{"own answer after nested block", []model.Line{v137TestOpen(t, "note", 5),
			v137TestText(t, "Text", "Text", 6), v137TestClose(t, 6), v137TestText(t, "London", "London", 5)}, nil},
		{"nested answers checked independently", []model.Line{v137TestOpen(t, "answer", 5),
			v137TestClose(t, 6)}, [][4]int{{5, 1, 8, 2}, {6, 1, 7, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v137TestRun(t, v137TestAnswer(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("task", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "task", "t", "identifier", 3),
			v137TestTag(t, "question", "Where does Tom live?", "content", 4),
			v137TestOpen(t, "answer", 4), v137TestClose(t, 6), v137TestTag(t, "endtask", "", "", 3))
		v137TestRun(t, result, "current", [][4]int{{6, 1, 7, 2}})
	})
	for _, name := range []string{"choice", "multichoice"} {
		t.Run(name, func(t *testing.T) {
			result := v137TestDocument(t, v137TestOpen(t, name, 3), v137TestOpen(t, "answer", 4),
				v137TestClose(t, 5), v137TestText(t, "Paris", "Paris", 4), v137TestClose(t, 4))
			v137TestRun(t, result, "current", [][4]int{{5, 1, 6, 2}})
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			raw := `@answer \{`
			if name == "multifill" {
				// DSL 4.4.11 требует экранировать начальное @answer даже в multifill.
				raw = `\@answer \{`
			}
			result := v137TestDocument(t, v137TestOpen(t, name, 3),
				v137TestText(t, raw, "@answer {", 4), v137TestText(t, `\}`, "}", 4), v137TestClose(t, 4))
			v137TestRun(t, result, "current", nil)
		})
	}
	t.Run("independent blocks and text outside", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestOpen(t, "answer", 3), v137TestClose(t, 5), v137TestTag(t, "question", "When?", "content", 3),
			v137TestOpen(t, "answer", 3), v137TestClose(t, 8), v137TestText(t, "Outside.", "Outside.", 3))
		v137TestRun(t, result, "current", [][4]int{{5, 1, 6, 2}, {8, 1, 9, 2}})
	})
	t.Run("inline answer", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestTag(t, "answer", "London", "content", 3))
		v137TestRun(t, result, "current", nil)
	})
}

func TestV137UnknownContentAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"invalid text", v137TestInvalid(t, "@unknown", 5)},
		{"partially parsed text", v137TestLine(t, "\u00a0?", "content", 5,
			v137TestElement(t, "content", "\u00a0", "\u00a0", 1), v137TestElement(t, "unparsed", "?", "", 2))},
		{"unparsed whitespace", v137TestInvalid(t, "\u00a0", 5)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v137TestRun(t, v137TestAnswer(t, tc.row), "current", nil)
		})
	}
	t.Run("unknown outside block does not suppress finding", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestOpen(t, "answer", 3), v137TestClose(t, 5), v137TestInvalid(t, "@unknown", 3))
		v137TestRun(t, result, "current", [][4]int{{5, 1, 6, 2}})
	})
	for _, withBlank := range []bool{false, true} {
		t.Run("unclosed at EOF blank="+strconv.FormatBool(withBlank), func(t *testing.T) {
			body := []model.Line{v137TestTag(t, "question", "Where?", "content", 3), v137TestOpen(t, "answer", 3)}
			if withBlank {
				body = append(body, v137TestLine(t, "", "blank", 5))
			}
			result := v137TestDocument(t, body...)
			v137TestParserError(t, result, 5, -1)
			v137TestRun(t, result, "current", nil)
		})
	}
	t.Run("ambiguous closing links", func(t *testing.T) {
		// CheckModel допускает две block-end с одним родителем при ошибке разбора.
		// Такая модель восстановления также предусмотрена тестами V035.
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestOpen(t, "answer", 3), v137TestClose(t, 5), v137TestClose(t, 5))
		v137TestParserError(t, result, 7, -1)
		v137TestRun(t, result, "current", nil)
	})
	t.Run("unclosed nested block", func(t *testing.T) {
		// Закрытие восстановлено для answer, но открытый note не получил границы.
		result := v137TestAnswer(t, v137TestOpen(t, "note", 5))
		v137TestParserError(t, result, 6, -1)
		v137TestRun(t, result, "current", nil)
	})
	t.Run("closed answer in unclosed outer block", func(t *testing.T) {
		result := v137TestDocument(t, v137TestOpen(t, "note", 3),
			v137TestTag(t, "question", "Where?", "content", 4), v137TestOpen(t, "answer", 4), v137TestClose(t, 6))
		v137TestParserError(t, result, 4, -1)
		v137TestRun(t, result, "current", [][4]int{{6, 1, 7, 2}})
	})
	t.Run("unparsed opening is not reconstructed", func(t *testing.T) {
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3),
			v137TestInvalid(t, "@answer {", 3), v137TestInvalid(t, "}", 3))
		v137TestRun(t, result, "current", nil)
	})
	t.Run("unknown tail after opening brace", func(t *testing.T) {
		opening := v137TestLine(t, "@answer { extra", "block-start", 3,
			v137TestElement(t, "tag", "@answer", "answer", 1), v137TestElement(t, "block-open", "{", "{", 9),
			v137TestElement(t, "unparsed", "extra", "", 11))
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3), opening, v137TestClose(t, 5))
		v137TestRun(t, result, "current", nil)
	})
	t.Run("unknown tail after closing brace", func(t *testing.T) {
		closing := v137TestLine(t, "} extra", "block-end", 5,
			v137TestElement(t, "block-close", "}", "}", 1), v137TestElement(t, "unparsed", "extra", "", 3))
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3), v137TestOpen(t, "answer", 3), closing)
		v137TestRun(t, result, "current", nil)
	})
}

func TestV137DeclarationAndCoordinates(t *testing.T) {
	for _, kind := range []model.ElementType{"content", "unparsed"} {
		t.Run("extra declaration text "+string(kind), func(t *testing.T) {
			opening := v137TestLine(t, "@answer Extra {", "block-start", 3,
				v137TestElement(t, "tag", "@answer", "answer", 1), v137TestElement(t, kind, "Extra", "Extra", 9),
				v137TestElement(t, "block-open", "{", "{", 15))
			result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3), opening, v137TestClose(t, 5))
			v137TestRun(t, result, "current", [][4]int{{5, 1, 6, 2}})
		})
	}
	t.Run("case and indentation", func(t *testing.T) {
		opening := v137TestLine(t, "  @ANSWER {", "block-start", 3,
			v137TestElement(t, "tag", "@ANSWER", "answer", 3), v137TestElement(t, "block-open", "{", "{", 11))
		closing := v137TestLine(t, "  } \t", "block-end", 5, v137TestElement(t, "block-close", "}", "}", 3))
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3), opening, closing)
		v137TestRun(t, result, "current", [][4]int{{5, 3, 6, 4}})
	})
	t.Run("Unicode declaration does not change range", func(t *testing.T) {
		opening := v137TestLine(t, "@answer Я😀 {", "block-start", 3,
			v137TestElement(t, "tag", "@answer", "answer", 1), v137TestElement(t, "unparsed", "Я😀", "", 9),
			v137TestElement(t, "block-open", "{", "{", 12))
		result := v137TestDocument(t, v137TestTag(t, "question", "Where?", "content", 3), opening, v137TestClose(t, 5))
		v137TestRun(t, result, "current", [][4]int{{5, 1, 6, 2}})
	})
}

func TestV137EndingsEmptyAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+" final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v137TestAnswer(t)
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v137TestRun(t, result, "first", [][4]int{{5, 1, 6, 2}})
				v137TestRun(t, v137TestAnswer(t, v137TestText(t, "London", "London", 5)), "other", nil)
				v137TestRun(t, result, "second", [][4]int{{5, 1, 6, 2}})
			})
		}
	}
	t.Run("no answers", func(t *testing.T) {
		v137TestRun(t, v137TestDocument(t), "current", nil)
	})
	t.Run("empty input", func(t *testing.T) {
		result := v137TestDocument(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v137TestRun(t, result, "current", nil)
	})
}

// v137TestElement создаёт исходный элемент; ожидаемые диапазоны заданы отдельно.
func v137TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v137TestLine задаёт тип и родителя явно, без повторного разбора DSL.
func v137TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v137TestTag задаёт известный тег и явный тип аргумента.
func v137TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v137TestElement(t, "tag", raw, name, 1)}
	if argument != "" {
		elements = append(elements, v137TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v137TestLine(t, raw, "tag", parent, elements...)
}

// v137TestOpen задаёт распознанное открытие блока без параметров.
func v137TestOpen(t *testing.T, name string, parent int) model.Line {
	t.Helper()
	return v137TestLine(t, "@"+name+" {", "block-start", parent,
		v137TestElement(t, "tag", "@"+name, name, 1), v137TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v137TestClose закрывает явно указанный блок.
func v137TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v137TestLine(t, "}", "block-end", parent, v137TestElement(t, "block-close", "}", "}", 1))
}

// v137TestText задаёт исходный и семантический текст независимо.
func v137TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v137TestLine(t, raw, "content", parent, v137TestElement(t, "content", raw, value, 1))
}

// v137TestInvalid сохраняет неинтерпретируемую строку целиком.
func v137TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v137TestLine(t, raw, "invalid", parent, v137TestElement(t, "unparsed", raw, "", 1))
}

// v137TestAnswer помещает вопрос на строку 4, answer на 5 и закрытие после тела.
func v137TestAnswer(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v137TestTag(t, "question", "Where does Tom live?", "content", 3), v137TestOpen(t, "answer", 3)}, body...)
	return v137TestDocument(t, append(lines, v137TestClose(t, 5))...)
}

// v137TestDocument добавляет обязательное окружение и историю ошибочных элементов.
// Родители заданы вызывающим тестом; номера и глубина следуют этой модели.
func v137TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v137TestTag(t, "dsl-version", "1.2", "version", 0),
		v137TestLine(t, "# Lesson", "heading", 0, v137TestElement(t, "heading-level", "#", "1", 1),
			v137TestElement(t, "title", "Lesson", "Lesson", 3)),
		v137TestLine(t, "### Practice", "heading", 0, v137TestElement(t, "heading-level", "###", "3", 1),
			v137TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			if line.Elements[j].ElementType == "unparsed" {
				v137TestParserError(t, result, line.Number, j)
			}
		}
	}
	return result
}

// v137TestParserError добавляет исходную ошибку: index < 0 обозначает scope line.
// Код условный, поскольку каталог внешнего парсера не входит в репозиторий.
func v137TestParserError(t *testing.T, result *model.Result, number, index int) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
	start, end, scope := 1, utf8.RuneCountInString(line.Raw)+1, diagnostics.ScopeLine
	if index >= 0 {
		element := &line.Elements[index]
		element.ErrorIDs = append(element.ErrorIDs, id)
		start, end, scope = element.Start, element.End, diagnostics.ScopeElement
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: scope, Message: "Ошибка исходной разметки.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v137TestRegistry использует полный штатный каталог.
func v137TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v137TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v137TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v137TestLineReads считает обращения к исходному интерфейсу без изменения строк.
type v137TestLineReads struct {
	diagnostics.Lines
	reads int
}

func (lines *v137TestLineReads) Line(number int) (diagnostics.Line, bool) {
	lines.reads++
	return lines.Lines.Line(number)
}

// v137TestRun проверяет новые V137 после полного штатного конвейера.
// Ожидаемые диапазоны задаются явно как [строка, колонка, строка, колонка).
func v137TestRun(t *testing.T, result *model.Result, processingID string, want [][4]int) []model.Diagnostic {
	t.Helper()
	before := v137TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v137TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(checks) != 198 {
		t.Fatalf("prepared catalog size = %d, want 198", len(checks))
	}
	if v137TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	var reads int
	for i := range checks {
		if checks[i].Code == "V137" {
			check := checks[i].Check
			checks[i].Check = func(lines diagnostics.Lines, findings diagnostics.Findings) error {
				observed := &v137TestLineReads{Lines: lines}
				err := check(observed, findings)
				reads = observed.reads
				return err
			}
		}
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Допускает несколько линейных проходов, но не повторный обход хвоста
	// документа для каждого блока. Ограничение не зависит от скорости машины.
	if limit := 8*len(result.Lines) + 16; reads > limit {
		t.Errorf("V137 Line calls = %d, want at most %d for %d lines", reads, limit, len(result.Lines))
	}
	added := result.Diagnostics[oldCount:]
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == "V137" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V137 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[2], Column: expected[3]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != "Блок @answer не содержит непустых ответов." ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal || entry.ID == "" ||
			entry.Source != processingID || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V137 occurrence %d = %+v, want location %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors {
			t.Fatal("document error flag is missing")
		}
		// Контракт validator.errorFlags: блок отмечает начальную строку Location.
		if !result.Lines[expected[0]-1].HasErrors {
			t.Fatal("block start error flag is missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("block-scoped V137 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v137TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Исключаются только новые записи и производные флаги/ссылки.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element error links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v137TestSnapshot(t, &restored) != before {
		t.Fatal("source data or previous history changed")
	}
	return added
}

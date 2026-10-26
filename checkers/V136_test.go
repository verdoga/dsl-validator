package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v136Expected задаёт независимый диапазон в Unicode-колонках с 1, конец исключается.
type v136Expected struct{ line, start, end int }

func TestV136Registration(t *testing.T) {
	definitions, err := v136Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V136" {
			continue
		}
		count++
		if definition.Message != "У однострочного @answer отсутствует непустой ответ." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V136 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v136Unavailable(t, version)
			before := v136Snapshot(t, result)
			checks, err := validator.Prepare(result, v136Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v136Snapshot(t, result) != before {
				t.Fatal("unsupported input changed")
			}
		})
	}
}

func TestV136ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v136Expected
	}{
		{"reference", v136Named(t, "answer", "content", "London", 2, 1), nil},
		{"example", v136Answer(t, "", "tag", 2, 1), []v136Expected{{4, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Дословным фрагментам таблицы предшествуют только обязательные
			// @dsl-version и заголовок (DSL 1.1–1.3 и 1.30).
			v136Run(t, v136Document(t, v136Question(t, 2, 1), tc.line), "current", tc.want)
		})
	}
}

func TestV136Whitespace(t *testing.T) {
	// Все White_Space, допустимые внутри Raw физической строки, перечислены
	// независимо от проверяемого кода. CR/LF — границы строк по контракту модели.
	for _, suffix := range []string{
		"", " ", "   ", "\t", " \t ",
		" \u000b", " \u000c", " \u0085", " \u00a0", " \u1680",
		" \u2000", " \u2001", " \u2002", " \u2003", " \u2004", " \u2005",
		" \u2006", " \u2007", " \u2008", " \u2009", " \u200a",
		" \u2028", " \u2029", " \u202f", " \u205f", " \u3000",
		" \t\u00a0\u2003\u2028\u3000\t ",
	} {
		t.Run(fmt.Sprintf("%q", suffix), func(t *testing.T) {
			t.Run("uncovered", func(t *testing.T) {
				// Краевые пробелы могут не входить ни в один content. Raw сохраняет их.
				result := v136Document(t, v136Question(t, 2, 1), v136Answer(t, suffix, "tag", 2, 1))
				v136Run(t, result, "current", []v136Expected{{4, 1, 8}})
			})
			t.Run("content value", func(t *testing.T) {
				line := v136Answer(t, suffix, "tag", 2, 1,
					v136Element(t, "content", suffix, suffix, 8, 8+utf8.RuneCountInString(suffix)))
				v136Run(t, v136Document(t, v136Question(t, 2, 1), line), "current", []v136Expected{{4, 1, 8}})
			})
		})
	}
	for _, raw := range []string{"@ANSWER", "@AnSwEr", "\t  @AnSwEr \t"} {
		t.Run(raw, func(t *testing.T) {
			start, end, tag := 1, 8, raw
			want := []v136Expected{{4, 1, 8}}
			if raw == "\t  @AnSwEr \t" {
				start, end, tag = 4, 11, "@AnSwEr"
				want = []v136Expected{{4, 4, 11}}
			}
			line := v136Line(t, raw, "tag", 2, 1, v136Element(t, "tag", tag, "answer", start, end))
			v136Run(t, v136Document(t, v136Question(t, 2, 1), line), "current", want)
		})
	}
}

func TestV136NonemptyTextAndEscapes(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"letter", "a", "a"}, {"Unicode", "Токио", "Токио"}, {"digit", "0", "0"},
		{"punctuation", "...", "..."}, {"quotes are text", `""`, `""`},
		{"zero width space", "\u200b", "\u200b"}, {"word joiner", "\u2060", "\u2060"},
		{"combining mark", "\u0301", "\u0301"}, {"Unicode punctuation", "—…", "—…"},
		{"mixed whitespace", "\u00a0 A\t\u2003B \u3000", "\u00a0 A\t\u2003B \u3000"},
		{"escaped backslash", `\\`, `\`}, {"escaped opening", `\{`, `{`}, {"escaped closing", `\}`, `}`},
		{"all allowed pairs", `\\\{\}`, `\{}`}, {"no second decoding", `\\t`, `\t`},
		{"literal slash t", `\t`, `\t`}, {"literal slash n", `\n`, `\n`},
		{"unknown escape", `\q`, `\q`}, {"lone backslash", `\`, `\`},
		{"backslash before whitespace", "\\\u00a0", "\\\u00a0"},
		{"inline at sign", `\@`, `\@`}, {"inline hash", `\#`, `\#`},
		{"underscores are answer data", `\_____`, `\_____`},
		{"HTML is answer data", "<br>", "<br>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Value задан явно по DSL 1.21–1.22 и 4.3.3, не вычисляется декодером.
			line := v136Answer(t, " "+tc.raw, "tag", 2, 1,
				v136Element(t, "content", tc.raw, tc.value, 9, 9+utf8.RuneCountInString(tc.raw)))
			v136Run(t, v136Document(t, v136Question(t, 2, 1), line), "current", nil)
		})
	}
}

func TestV136ProcessedContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		empty            bool
	}{
		{"processed empty", "source", "", true},
		{"processed Unicode whitespace", "source", "\u00a0\u2003\t\u3000", true},
		{"processed nonempty", " ", "London", false},
		{"processed punctuation", " ", "—…", false},
		{"literal slash t remains text", " ", `\t`, false},
		{"literal slash n remains text", " ", `\n`, false},
		{"literal Unicode escape remains text", " ", `\u00a0`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Вывод о пустоте зависит от готового Value, а не Raw. Буквальные
			// последовательности с обратной косой чертой уже обработаны парсером.
			line := v136Answer(t, " "+tc.raw, "tag", 2, 1,
				v136Element(t, "content", tc.raw, tc.value, 9, 9+utf8.RuneCountInString(tc.raw)))
			var want []v136Expected
			if tc.empty {
				want = []v136Expected{{4, 1, 8}}
			}
			v136Run(t, v136Document(t, v136Question(t, 2, 1), line), "current", want)
		})
	}
}

func TestV136ContentFragments(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v136Expected
	}{
		{"all processed empty", v136Answer(t, " ab", "tag", 2, 1,
			v136Element(t, "content", "a", "", 9, 10), v136Element(t, "content", "b", "", 10, 11)), []v136Expected{{4, 1, 8}}},
		{"Unicode values and uncovered whitespace gaps", v136Answer(t, " a \u00a0b\t", "tag", 2, 1,
			v136Element(t, "content", "a", "\u2003", 9, 10), v136Element(t, "content", "b", "\t\u3000", 12, 13)), []v136Expected{{4, 1, 8}}},
		{"nonempty first value", v136Answer(t, " ab", "tag", 2, 1,
			v136Element(t, "content", "a", "!", 9, 10), v136Element(t, "content", "b", "", 10, 11)), nil},
		{"nonempty last value", v136Answer(t, " ab", "tag", 2, 1,
			v136Element(t, "content", "a", "", 9, 10), v136Element(t, "content", "b", "…", 10, 11)), nil},
		{"fragment boundary does not decode slash t", v136Answer(t, " ab", "tag", 2, 1,
			v136Element(t, "content", "a", `\`, 9, 10), v136Element(t, "content", "b", "t", 10, 11)), nil},
		{"significant uncovered gap is unknown", v136Answer(t, " a!b", "tag", 2, 1,
			v136Element(t, "content", "a", "", 9, 10), v136Element(t, "content", "b", "", 11, 12)), nil},
		{"zero length content does not uncover previous text", v136Answer(t, " ab", "tag", 2, 1,
			v136Element(t, "content", "ab", "", 9, 11), v136Element(t, "content", "", "", 10, 10)), []v136Expected{{4, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v136Run(t, v136Document(t, v136Question(t, 2, 1), tc.line), "current", tc.want)
		})
	}
}

func TestV136UnparsedAndUncoveredContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v136Expected
	}{
		{"nonempty unparsed", v136Answer(t, " !", "invalid", 2, 1, v136Unparsed(t, "!", 9, 10)), nil},
		{"whitespace unparsed", v136Answer(t, " \u00a0", "invalid", 2, 1, v136Unparsed(t, "\u00a0", 9, 10)), nil},
		{"empty content then whitespace unparsed", v136Answer(t, " a\u00a0", "invalid", 2, 1,
			v136Element(t, "content", "a", "", 9, 10), v136Unparsed(t, "\u00a0", 10, 11)), nil},
		{"empty content then unparsed", v136Answer(t, "  !", "invalid", 2, 1,
			v136Element(t, "content", " ", "", 9, 10), v136Unparsed(t, "!", 10, 11)), nil},
		{"several content pieces", v136Answer(t, " \u00a0x", "tag", 2, 1,
			v136Element(t, "content", "\u00a0", "\u00a0", 9, 10), v136Element(t, "content", "x", "x", 10, 11)), nil},
		{"empty content element", v136Answer(t, " ", "tag", 2, 1,
			v136Element(t, "content", "", "", 9, 9)), []v136Expected{{4, 1, 8}}},
		// Контракт не требует полного покрытия Raw элементами; отсутствие
		// content само по себе не означает отсутствия текста после имени.
		{"uncovered nonempty tail", v136Answer(t, " London", "tag", 2, 1), nil},
		{"uncovered tail after whitespace content", v136Answer(t, " \u00a0x", "tag", 2, 1,
			v136Element(t, "content", "\u00a0", "\u00a0", 9, 10)), nil},
		{"whole declaration unparsed", v136Line(t, "@answer", "invalid", 2, 1, v136Unparsed(t, "@answer", 1, 8)), nil},
		{"unparsed tag with saved content", v136Line(t, "@answer \u00a0", "invalid", 2, 1,
			v136Unparsed(t, "@answer", 1, 8), v136Element(t, "content", "\u00a0", "\u00a0", 9, 10)), nil},
		{"other exact name", v136Line(t, "@answerText", "invalid", 2, 1, v136Unparsed(t, "@answerText", 1, 12)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v136Run(t, v136Document(t, v136Question(t, 2, 1), tc.line), "current", tc.want)
		})
	}
}

func TestV136ContextAndRepeatedDeclarations(t *testing.T) {
	t.Run("first body object in wrong position", func(t *testing.T) {
		result := v136Document(t, v136Answer(t, "", "tag", 2, 1), v136Text(t, "Continue.", 2, 1))
		v136ParserError(t, result, 3, 0)
		v136Run(t, result, "current", []v136Expected{{3, 1, 8}})
	})
	t.Run("task step", func(t *testing.T) {
		result := v136Document(t,
			v136Named(t, "task", "identifier", "t", 2, 1), v136Named(t, "step", "title", "First", 3, 2),
			v136Question(t, 4, 3), v136Answer(t, "", "tag", 4, 3))
		v136Run(t, result, "current", []v136Expected{{6, 1, 8}})
	})
	t.Run("two repeated empty answers", func(t *testing.T) {
		result := v136Document(t, v136Question(t, 2, 1), v136Answer(t, "", "tag", 2, 1), v136Answer(t, "", "tag", 2, 1))
		v136ParserError(t, result, 5, 0)
		v136Run(t, result, "current", []v136Expected{{4, 1, 8}, {5, 1, 8}})
	})
	t.Run("nonempty answer between empty ones", func(t *testing.T) {
		result := v136Document(t,
			v136Question(t, 2, 1), v136Answer(t, "", "tag", 2, 1),
			v136Question(t, 2, 1), v136Named(t, "answer", "content", "London", 2, 1),
			v136Question(t, 2, 1), v136Answer(t, "", "tag", 2, 1))
		v136Run(t, result, "current", []v136Expected{{4, 1, 8}, {8, 1, 8}})
	})
	t.Run("next physical line does not fill answer", func(t *testing.T) {
		result := v136Document(t, v136Question(t, 2, 1), v136Answer(t, "", "tag", 2, 1), v136Text(t, "London", 2, 1))
		v136Run(t, result, "current", []v136Expected{{4, 1, 8}})
	})
	t.Run("forbidden tag inside note still checked", func(t *testing.T) {
		result := v136Document(t, v136Open(t, "note", 2, 1), v136Answer(t, "", "tag", 3, 2),
			v136Text(t, "Read.", 3, 2), v136Close(t, 3, 2))
		v136ParserError(t, result, 4, 0)
		v136Run(t, result, "current", []v136Expected{{4, 1, 8}})
	})
}

func TestV136InteractiveContexts(t *testing.T) {
	for _, context := range []struct {
		name                  string
		before, after         []model.Line
		parent, depth, number int
	}{
		{"question", []model.Line{v136Question(t, 2, 1)}, nil, 2, 1, 4},
		{"choice", []model.Line{v136Open(t, "choice", 2, 1)},
			[]model.Line{v136Text(t, "Paris", 3, 2), v136Close(t, 3, 2)}, 3, 2, 4},
		{"multichoice", []model.Line{v136Open(t, "multichoice", 2, 1)},
			[]model.Line{v136Text(t, "Paris", 3, 2), v136Close(t, 3, 2)}, 3, 2, 4},
		{"matching", []model.Line{v136Open(t, "matching", 2, 1), v136Text(t, "City A", 3, 2)},
			[]model.Line{v136Text(t, "City B", 3, 2), v136Named(t, "answer", "content", "Paris", 3, 2),
				v136Line(t, "---", "separator", 3, 2), v136Text(t, "London", 3, 2), v136Text(t, "Paris", 3, 2), v136Close(t, 3, 2)}, 3, 2, 5},
		{"ordering without separator", []model.Line{v136Open(t, "ordering", 2, 1), v136Text(t, "First", 3, 2)},
			[]model.Line{v136Text(t, "Second", 3, 2), v136Named(t, "answer", "content", "2", 3, 2), v136Close(t, 3, 2)}, 3, 2, 5},
		{"ordering with separator", []model.Line{v136Open(t, "ordering", 2, 1), v136Text(t, "City A", 3, 2)},
			[]model.Line{v136Text(t, "City B", 3, 2), v136Named(t, "answer", "content", "Paris", 3, 2),
				v136Line(t, "---", "separator", 3, 2), v136Text(t, "London", 3, 2), v136Text(t, "Paris", 3, 2), v136Close(t, 3, 2)}, 3, 2, 5},
	} {
		for _, tc := range []struct {
			name, raw, value string
			empty            bool
		}{
			{"missing", "", "", true},
			{"Unicode whitespace", "\u00a0\t\u2003\u3000", "\u00a0\t\u2003\u3000", true},
			{"processed whitespace", "source", "\u00a0\t\u2003\u3000", true},
			{"punctuation", "...—…", "...—…", false},
			{"no second decoding", `\\t`, `\t`, false},
		} {
			t.Run(context.name+"/"+tc.name, func(t *testing.T) {
				answer := v136Answer(t, "", "tag", context.parent, context.depth)
				if tc.raw != "" {
					answer = v136Answer(t, " "+tc.raw, "tag", context.parent, context.depth,
						v136Element(t, "content", tc.raw, tc.value, 9, 9+utf8.RuneCountInString(tc.raw)))
				}
				body := append([]model.Line{}, context.before...)
				body = append(body, answer)
				body = append(body, context.after...)
				var want []v136Expected
				if tc.empty {
					want = []v136Expected{{context.number, 1, 8}}
				}
				v136Run(t, v136Document(t, body...), "current", want)
			})
		}
	}
}

func TestV136BlocksAndLiteralContexts(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run("answer block empty="+fmt.Sprint(empty), func(t *testing.T) {
			body := []model.Line{v136Question(t, 2, 1), v136Open(t, "answer", 2, 1)}
			if !empty {
				body = append(body, v136Text(t, "London", 4, 2))
			}
			body = append(body, v136Close(t, 4, 2))
			result := v136Document(t, body...)
			if empty {
				v136ParserError(t, result, 4, 0)
			}
			v136Run(t, result, "current", nil)
		})
	}
	t.Run("invalid opening with saved block-open", func(t *testing.T) {
		line := v136Answer(t, " { extra", "invalid", 2, 1,
			v136Element(t, "block-open", "{", "{", 9, 10), v136Unparsed(t, "extra", 11, 16))
		v136Run(t, v136Document(t, v136Question(t, 2, 1), line), "current", nil)
	})
	t.Run("editor literal and following real answer", func(t *testing.T) {
		result := v136Document(t, v136Open(t, "editor", 2, 1), v136Text(t, "@answer", 3, 2), v136Close(t, 3, 2),
			v136Question(t, 2, 1), v136Answer(t, "", "tag", 2, 1))
		v136Run(t, result, "current", []v136Expected{{7, 1, 8}})
	})
	for _, savedTag := range []bool{false, true} {
		t.Run("multifill saved tag="+fmt.Sprint(savedTag), func(t *testing.T) {
			line := v136Line(t, "@answer", "invalid", 3, 2, v136Unparsed(t, "@answer", 1, 8))
			if savedTag {
				line = v136Answer(t, "", "tag", 3, 2)
			}
			result := v136Document(t, v136Open(t, "multifill", 2, 1), line, v136Close(t, 3, 2))
			if savedTag {
				// DSL 4.4.18 запрещает объявление, но V136 явно исключает его.
				v136ParserError(t, result, 4, 0)
			}
			v136Run(t, result, "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@answer`, "@answer"}, {"Mention @answer here.", "Mention @answer here."},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			line := v136Line(t, tc.raw, "content", 2, 1,
				v136Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v136Run(t, v136Document(t, line), "current", nil)
		})
	}
	t.Run("escaped answer in multifill", func(t *testing.T) {
		line := v136Line(t, `\@answer`, "content", 3, 2, v136Element(t, "content", `\@answer`, "@answer", 1, 9))
		v136Run(t, v136Document(t, v136Open(t, "multifill", 2, 1), line, v136Close(t, 3, 2)), "current", nil)
	})
	t.Run("different tag", func(t *testing.T) {
		v136Run(t, v136Document(t, v136Named(t, "note", "content", "@answer", 2, 1)), "current", nil)
	})
}

func TestV136LineEndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name string
		eol  model.LineEnding
		last model.LineEnding
	}{
		{"LF", model.LineEndingLF, model.LineEndingLF},
		{"CRLF", model.LineEndingCRLF, model.LineEndingCRLF},
		{"LF without final EOL", model.LineEndingLF, model.LineEndingNone},
		{"CRLF without final EOL", model.LineEndingCRLF, model.LineEndingNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v136Document(t, v136Question(t, 2, 1), v136Answer(t, "", "tag", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[3].LineEnding = tc.last
			v136Run(t, result, "first", []v136Expected{{4, 1, 8}})
			v136Run(t, result, "second", []v136Expected{{4, 1, 8}})
		})
	}
	v136Run(t, v136Unavailable(t, "1.2"), "current", nil)
	v136Run(t, v136Document(t, v136Question(t, 2, 1)), "current", nil)
}

// v136Registry использует полный штатный каталог без подмены регистраций.
func v136Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v136Unavailable задаёт согласованную модель с недоступными исходными строками.
func v136Unavailable(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v136Document добавляет обязательные внешние строки. Родители и элементы тела
// заданы тестом; для каждого unparsed сохраняется обязательная ошибка парсера.
func v136Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v136Unavailable(t, "1.2")
	title, encoding := "Lesson", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = append([]model.Line{
		v136Named(t, "dsl-version", "version", "1.2", 0, 0),
		v136Line(t, "# Lesson", "heading", 0, 0,
			v136Element(t, "heading-level", "#", "1", 1, 2), v136Element(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
		for j, element := range result.Lines[i].Elements {
			if element.ElementType == "unparsed" {
				v136ParserError(t, result, i+1, j)
			}
		}
	}
	return result
}

// v136Line сохраняет заданную структуру строки, не разбирая Raw.
func v136Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v136Element сохраняет явно заданные Raw, Value и исходные Unicode-колонки.
func v136Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v136Unparsed создаёт элемент с null Value; ошибку добавляет v136Document.
func v136Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v136Answer сохраняет точный хвост и переданные элементы ответа без нормализации.
func v136Answer(t *testing.T, suffix string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	return v136Line(t, "@answer"+suffix, kind, parent, depth,
		append([]model.Element{v136Element(t, "tag", "@answer", "answer", 1, 8)}, elements...)...)
}

// v136Named строит заданный тег с одним типизированным аргументом.
func v136Named(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	start := utf8.RuneCountInString(tag) + 2
	return v136Line(t, tag+" "+value, "tag", parent, depth,
		v136Element(t, "tag", tag, name, 1, start-1), v136Element(t, kind, value, value, start, start+utf8.RuneCountInString(value)))
}

// v136Question сохраняет дословный вопрос из эталона и примера.
func v136Question(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v136Named(t, "question", "content", "Where does Tom live?", parent, depth)
}

// v136Text создаёт текст без экранирования и краевых пробелов.
func v136Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v136Line(t, raw, "content", parent, depth, v136Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v136Open создаёт объявление фигурного блока с указанным родителем.
func v136Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v136Line(t, tag+" {", "block-start", parent, depth,
		v136Element(t, "tag", tag, name, 1, open-1), v136Element(t, "block-open", "{", "{", open, open+1))
}

// v136Close закрывает явно указанный родительский блок.
func v136Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v136Line(t, "}", "block-end", parent, depth, v136Element(t, "block-close", "}", "}", 1, 2))
}

// v136ParserError добавляет согласованную историю парсера на выбранный элемент.
// Вызовы идут в порядке исходных позиций; тестовый код парсера непрозрачен.
func v136ParserError(t *testing.T, result *model.Result, number, index int) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
	line.HasErrors, result.Document.HasErrors = true, true
	element.ErrorIDs = append(element.ErrorIDs, id)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
		Location:         &model.Location{Start: model.Position{Line: number, Column: element.Start}, End: model.Position{Line: number, Column: element.End}},
		RelatedLocations: []model.Location{}})
}

// v136Run использует полный Prepare/Run и проверяет только V136 текущего запуска.
// CheckModel проверяет общие флаги и ссылки с учётом остальных диагностик каталога.
func v136Run(t *testing.T, result *model.Result, source string, want []v136Expected) {
	t.Helper()
	before := v136Snapshot(t, result)
	checks, err := validator.Prepare(result, v136Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v136Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	targetIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("new diagnostic has source %q, want %q", entry.Source, source)
		}
		if entry.DiagnosticCode == "V136" {
			got = append(got, entry)
			targetIDs[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V136 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v / none", i, entry.Location, entry.RelatedLocations, location)
		}
		if entry.Message != "У однострочного @answer отсутствует непустой ответ." || entry.DiagnosticCode != "V136" ||
			entry.Source != source || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v136Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	// Исключаем только добавленную историю и производные эффекты конвейера.
	// Исходный текст, структура, значения элементов и прежняя история неизменны.
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			for _, id := range element.ErrorIDs {
				if targetIDs[id] {
					t.Errorf("ScopeLine V136 linked to element at %d:%d", i+1, j)
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v136Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v136Snapshot сохраняет модель для проверки отсутствия побочных изменений.
func v136Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

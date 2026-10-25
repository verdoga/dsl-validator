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

func TestV176Registration(t *testing.T) {
	registry := v176TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V176" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "element" || definition.Check == nil ||
			definition.Message != "В @ordering меньше двух распределяемых элементов." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V176 registrations = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV176ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][4]int
	}{
		{"reference", "@ordering {\nA\nA\n}", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestText(t, "A", "A", 3, 2)}, nil},
		{"example", "@ordering {\nA\n}", []model.Line{v176TestText(t, "A", "A", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"empty", "@ordering {\n}", nil, [][4]int{{3, 1, 10, 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v176TestBlock(t, "ordering", tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v176TestRun(t, result, tc.want)
			v176TestRun(t, result, tc.want)
		})
	}
}

func TestV176OrdinaryRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows [][2]string
		want [][4]int
	}{
		{"two distinct", [][2]string{{"A", "A"}, {"B", "B"}}, nil},
		{"three distinct", [][2]string{{"A", "A"}, {"B", "B"}, {"C", "C"}}, nil},
		{"three identical", [][2]string{{"A", "A"}, {"A", "A"}, {"A", "A"}}, nil},
		{"ASCII edges", [][2]string{{" \tA\t ", "A"}}, [][4]int{{3, 1, 10, 1}}},
		{"case and internal whitespace", [][2]string{{"A  B", "A  B"}, {"a\tb", "a\tb"}}, nil},
		{"Unicode space is content", [][2]string{{"\u00a0", "\u00a0"}}, [][4]int{{3, 1, 10, 1}}},
		{"Unicode spaces make two rows", [][2]string{{"\u00a0", "\u00a0"}, {"\u2003", "\u2003"}}, nil},
		{"Unicode text", [][2]string{{"🌍 Ёж", "🌍 Ёж"}}, [][4]int{{3, 1, 10, 1}}},
		{"combining mark", [][2]string{{"\u0301", "\u0301"}}, [][4]int{{3, 1, 10, 1}}},
		{"punctuation", [][2]string{{"?!", "?!"}}, [][4]int{{3, 1, 10, 1}}},
		{"escaped at", [][2]string{{`\@answer A`, "@answer A"}}, [][4]int{{3, 1, 10, 1}}},
		{"escaped heading", [][2]string{{`\# Heading`, "# Heading"}}, [][4]int{{3, 1, 10, 1}}},
		{"escaped separator", [][2]string{{`\---`, "---"}}, [][4]int{{3, 1, 10, 1}}},
		{"escaped separator plus item", [][2]string{{`\---`, "---"}, {"A", "A"}}, nil},
		{"three escaped markers", [][2]string{{`\@x`, "@x"}, {`\# x`, "# x"}, {`\---`, "---"}}, nil},
		{"escaped braces", [][2]string{{`\{\}`, "{}"}}, [][4]int{{3, 1, 10, 1}}},
		{"escaped closing brace", [][2]string{{`\}`, "}"}}, [][4]int{{3, 1, 10, 1}}},
		{"slash pair", [][2]string{{`\\`, `\`}}, [][4]int{{3, 1, 10, 1}}},
		{"slash pair before marker", [][2]string{{`\\@x`, `\@x`}}, [][4]int{{3, 1, 10, 1}}},
		{"slash pair then escaped brace", [][2]string{{`a\\\{b`, `a\{b`}}, [][4]int{{3, 1, 10, 1}}},
		{"unknown escape retained", [][2]string{{`\q`, `\q`}}, [][4]int{{3, 1, 10, 1}}},
		{"marker inside text", [][2]string{{"A @answer B # C", "A @answer B # C"}}, [][4]int{{3, 1, 10, 1}}},
		{"not exact delimiter", [][2]string{{"----", "----"}}, [][4]int{{3, 1, 10, 1}}},
	} {
		for _, fallback := range []bool{false, true} {
			t.Run(tc.name+"/raw="+strconv.FormatBool(fallback), func(t *testing.T) {
				var body []model.Line
				for _, row := range tc.rows {
					line := v176TestText(t, row[0], row[1], 3, 2)
					if fallback {
						line.Elements = []model.Element{}
					}
					body = append(body, line)
				}
				v176TestRun(t, v176TestBlock(t, "ordering", body...), tc.want)
			})
		}
	}
}

func TestV176SeparatorsAndBlankLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"blank only", []model.Line{v176TestLine(t, "", "blank", 3, 2), v176TestLine(t, " \t ", "blank", 3, 2)}, [][4]int{{3, 1, 10, 0}}},
		{"blanks between items", []model.Line{v176TestLine(t, "", "blank", 3, 2), v176TestText(t, "A", "A", 3, 2), v176TestLine(t, " \t", "blank", 3, 2), v176TestText(t, "B", "B", 3, 2), v176TestLine(t, "", "blank", 3, 2)}, nil},
		{"extended zero", []model.Line{v176TestSeparator(t, "---", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, [][4]int{{3, 1, 10, 0}}},
		{"extended one", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"extended two", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestText(t, "A", "A", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, nil},
		{"two separators", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, nil},
		{"three separators", []model.Line{v176TestSeparator(t, "---", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestSeparator(t, "---", 3, 2)}, nil},
		{"ASCII separator edges", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestSeparator(t, " \t---\t ", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"Unicode edge is ordinary", []model.Line{v176TestText(t, "\u00a0---", "\u00a0---", 3, 2), v176TestText(t, "A", "A", 3, 2)}, nil},
		{"escaped and real separator", []model.Line{v176TestText(t, `\---`, "---", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"separator found from raw", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestUnparsed(t, "---", "invalid", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v176TestRun(t, v176TestBlock(t, "ordering", tc.body...), tc.want) })
	}
}

func TestV176ExcludedContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"inline answer and hint", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestInline(t, "answer", "1", 3, 2), v176TestInline(t, "hint", "Hint", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"block answer and hint", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestOpen(t, "answer", 3, 2), v176TestText(t, "1", "1", 5, 3), v176TestText(t, "2", "2", 5, 3), v176TestClose(t, 5, 3), v176TestOpen(t, "hint", 3, 2), v176TestText(t, "Hint", "Hint", 9, 3), v176TestClose(t, 9, 3)}, [][4]int{{3, 1, 10, 1}}},
		{"only declarations", []model.Line{v176TestInline(t, "answer", "1", 3, 2), v176TestInline(t, "hint", "Hint", 3, 2)}, [][4]int{{3, 1, 10, 0}}},
		{"other nested block", []model.Line{v176TestOpen(t, "note", 3, 2), v176TestText(t, "A", "A", 4, 3), v176TestText(t, "B", "B", 4, 3), v176TestClose(t, 4, 3)}, [][4]int{{3, 1, 10, 0}}},
		{"nested separator", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestInline(t, "answer", "1", 3, 2), v176TestOpen(t, "hint", 3, 2), v176TestText(t, "---", "---", 6, 3), v176TestClose(t, 6, 3)}, [][4]int{{3, 1, 10, 1}}},
		{"nested separator with real separator", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestInline(t, "answer", "First", 3, 2), v176TestOpen(t, "hint", 3, 2), v176TestText(t, "---", "---", 6, 3), v176TestClose(t, 6, 3), v176TestSeparator(t, "---", 3, 2), v176TestText(t, "First", "First", 3, 2), v176TestText(t, "Second", "Second", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"recognized erroneous heading", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestLine(t, "### Heading", "invalid", 3, 2, v176TestElement(t, "heading-level", "###", "3", 1), model.Element{ElementType: "unparsed", Raw: "Heading", Start: 5, End: 12, ErrorIDs: []string{}})}, [][4]int{{3, 1, 10, 1}}},
		{"unparsed initial heading", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestUnparsed(t, "### Heading", "invalid", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"unknown tag not ordinary", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestUnparsed(t, "@unknown X", "invalid", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"editor declaration", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestInline(t, "editor", "Comment", 3, 2)}, [][4]int{{3, 1, 10, 1}}},
		{"nested uncertainty cannot add items", []model.Line{v176TestText(t, "A", "A", 3, 2), v176TestInline(t, "answer", "1", 3, 2), v176TestOpen(t, "hint", 3, 2), v176TestUnparsed(t, "A {", "invalid", 6, 3), v176TestClose(t, 6, 3)}, [][4]int{{3, 1, 10, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v176TestRun(t, v176TestBlock(t, "ordering", tc.body...), tc.want) })
	}
	for _, filled := range []bool{false, true} {
		t.Run("instruction/filled="+strconv.FormatBool(filled), func(t *testing.T) {
			opening := v176TestLine(t, "@ordering Arrange. {", "block-start", 2, 1, v176TestElement(t, "tag", "@ordering", "ordering", 1), v176TestElement(t, "content", "Arrange.", "Arrange.", 11), v176TestElement(t, "block-open", "{", "{", 20))
			body := []model.Line{opening}
			count := 0
			if filled {
				body = append(body, v176TestText(t, "A", "A", 3, 2))
				count = 1
			}
			v176TestRun(t, v176TestDocument(t, append(body, v176TestClose(t, 3, 2))...), [][4]int{{3, 1, 10, count}})
		})
	}
}

func TestV176UncertaintyAndBoundaries(t *testing.T) {
	t.Run("uncertain counted line", func(t *testing.T) {
		v176TestRun(t, v176TestBlock(t, "ordering", v176TestText(t, "A", "A", 3, 2), v176TestUnparsed(t, "B {", "invalid", 3, 2)), nil)
	})
	t.Run("uncertain content fragments", func(t *testing.T) {
		v176TestRun(t, v176TestBlock(t, "ordering", v176TestUnparsed(t, "B {", "content", 3, 2)), nil)
	})
	t.Run("uncertain lower text excluded", func(t *testing.T) {
		v176TestRun(t, v176TestBlock(t, "ordering", v176TestText(t, "A", "A", 3, 2), v176TestSeparator(t, "---", 3, 2), v176TestUnparsed(t, "B {", "invalid", 3, 2), v176TestText(t, "Second", "Second", 3, 2)), [][4]int{{3, 1, 10, 1}})
	})
	t.Run("two known items need no error", func(t *testing.T) {
		v176TestRun(t, v176TestBlock(t, "ordering", v176TestText(t, "A", "A", 3, 2), v176TestText(t, "B", "B", 3, 2), v176TestUnparsed(t, "C {", "invalid", 3, 2)), nil)
	})
	t.Run("multiple content fragments are one row", func(t *testing.T) {
		line := v176TestLine(t, "A B", "content", 3, 2, v176TestElement(t, "content", "A", "A", 1), v176TestElement(t, "content", "B", "B", 3))
		v176TestRun(t, v176TestBlock(t, "ordering", line), [][4]int{{3, 1, 10, 1}})
	})
	t.Run("missing closing", func(t *testing.T) {
		result := v176TestDocument(t, v176TestOpen(t, "ordering", 2, 1), v176TestText(t, "A", "A", 3, 2))
		v176TestParserError(t, result, 3)
		v176TestRun(t, result, nil)
	})
	t.Run("ambiguous closing links", func(t *testing.T) {
		// Как в TestV035UnparsedAndBoundaries: контракт допускает неоднозначные
		// ссылки двух закрытий после ошибки парсера; удобное закрытие не выбирается.
		result := v176TestDocument(t, v176TestOpen(t, "ordering", 2, 1), v176TestClose(t, 3, 2), v176TestClose(t, 3, 2))
		v176TestParserError(t, result, 5)
		v176TestRun(t, result, nil)
	})
	t.Run("unclosed ordering inside branch", func(t *testing.T) {
		branch := v176TestLine(t, "@variant A", "tag", 3, 2, v176TestElement(t, "tag", "@variant", "variant", 1), v176TestElement(t, "name", "A", "A", 10))
		result := v176TestDocument(t, v176TestOpen(t, "variants", 2, 1), branch, v176TestOpen(t, "ordering", 4, 3), v176TestText(t, "A", "A", 5, 4))
		v176TestParserError(t, result, 5)
		v176TestRun(t, result, nil)
	})
	t.Run("closed ordering in unclosed container", func(t *testing.T) {
		branch := v176TestLine(t, "@variant A", "tag", 3, 2, v176TestElement(t, "tag", "@variant", "variant", 1), v176TestElement(t, "name", "A", "A", 10))
		result := v176TestDocument(t, v176TestOpen(t, "variants", 2, 1), branch, v176TestOpen(t, "ordering", 4, 3), v176TestText(t, "A", "A", 5, 4), v176TestClose(t, 5, 4))
		v176TestParserError(t, result, 3)
		v176TestRun(t, result, [][4]int{{5, 1, 10, 1}})
	})
}

func TestV176ContextsAndCoordinates(t *testing.T) {
	t.Run("separate blocks", func(t *testing.T) {
		v176TestRun(t, v176TestDocument(t, v176TestOpen(t, "ordering", 2, 1), v176TestClose(t, 3, 2), v176TestOpen(t, "ordering", 2, 1), v176TestText(t, "A", "A", 5, 2), v176TestClose(t, 5, 2)), [][4]int{{3, 1, 10, 0}, {5, 1, 10, 1}})
	})
	t.Run("branch at depth", func(t *testing.T) {
		first := v176TestLine(t, "@variant A", "tag", 3, 2, v176TestElement(t, "tag", "@variant", "variant", 1), v176TestElement(t, "name", "A", "A", 10))
		second := v176TestLine(t, "@variant B", "tag", 3, 2, v176TestElement(t, "tag", "@variant", "variant", 1), v176TestElement(t, "name", "B", "B", 10))
		v176TestRun(t, v176TestDocument(t, v176TestOpen(t, "variants", 2, 1), first, v176TestOpen(t, "ordering", 4, 3), v176TestText(t, "A", "A", 5, 4), v176TestClose(t, 5, 4), second, v176TestText(t, "Read.", "Read.", 8, 3), v176TestClose(t, 3, 2)), [][4]int{{5, 1, 10, 1}})
	})
	t.Run("indentation and tag case", func(t *testing.T) {
		opening := v176TestLine(t, "\t  @ORDERING {", "block-start", 2, 1, v176TestElement(t, "tag", "@ORDERING", "ordering", 4), v176TestElement(t, "block-open", "{", "{", 14))
		v176TestRun(t, v176TestDocument(t, opening, v176TestText(t, "A", "A", 3, 2), v176TestClose(t, 3, 2)), [][4]int{{3, 4, 13, 1}})
	})
	t.Run("ordinary text outside block cannot add items", func(t *testing.T) {
		v176TestRun(t, v176TestDocument(t, v176TestOpen(t, "ordering", 2, 1), v176TestText(t, "A", "A", 3, 2), v176TestClose(t, 3, 2), v176TestText(t, "B", "B", 2, 1)), [][4]int{{3, 1, 10, 1}})
	})
	for _, tag := range []string{"choice", "multichoice", "matching", "note"} {
		t.Run("other container/"+tag, func(t *testing.T) { v176TestRun(t, v176TestBlock(t, tag, v176TestText(t, "A", "A", 3, 2)), nil) })
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal/"+tag, func(t *testing.T) {
			v176TestRun(t, v176TestBlock(t, tag, v176TestText(t, "@ordering {", "@ordering {", 3, 2), v176TestText(t, "A", "A", 3, 2)), nil)
		})
	}
	t.Run("escaped pseudo declaration", func(t *testing.T) {
		v176TestRun(t, v176TestDocument(t, v176TestText(t, `\@ordering \{`, "@ordering {", 2, 1)), nil)
	})
	t.Run("empty document", func(t *testing.T) {
		result := v176TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v176TestRun(t, result, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v176TestBlock(t, "ordering", v176TestText(t, "A", "A", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v176TestRun(t, result, [][4]int{{3, 1, 10, 1}})
			})
		}
	}
}

// v176TestUnparsed задаёт неразобранную область; исходную ошибку добавляет Document.
func v176TestUnparsed(t *testing.T, raw string, kind model.LineType, parent, depth int) model.Line {
	t.Helper()
	return v176TestLine(t, raw, kind, parent, depth, model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
}

// v176TestParserError сохраняет исходную ошибку границ блока без фиктивных элементов.
func v176TestParserError(t *testing.T, result *model.Result, number int) {
	t.Helper()
	id := "boundary-" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error", DiagnosticScope: "line", Message: "Неоднозначные границы блока.", RelatedLocations: []model.Location{}, Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}}})
	result.Document.HasErrors, result.Lines[number-1].HasErrors = true, true
}

// v176TestRun задаёт ожидания как [строка, начало, конец, количество элементов].
// Проверяет полный штатный конвейер, метаданные, ссылки и сохранность исходника.
func v176TestRun(t *testing.T, result *model.Result, want [][4]int) {
	t.Helper()
	before := v176TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v176TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v176TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v176-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source != source {
			continue
		}
		if entry.DiagnosticCode == "V176" {
			got = append(got, entry)
		}
		if entry.DiagnosticCode == "V035" && entry.Location != nil {
			for _, element := range result.Lines[entry.Location.Start.Line-1].Elements {
				if element.ElementType == "tag" && element.Value != nil && *element.Value == "ordering" {
					t.Fatal("V035 duplicates ordering minimum")
				}
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V176 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		message := "В @ordering меньше двух распределяемых элементов.\nконкретизация: Найдено распределяемых элементов: " + strconv.Itoa(position[3]) + "."
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal || entry.Message != message || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V176 occurrence %d = %+v, location %+v; want %+v, %q", i, entry, entry.Location, location, message)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id != entry.ID {
						continue
					}
					links++
					if line.Number != position[0] || element.Start != position[1] || element.End != position[2] || element.ElementType != "tag" {
						t.Fatal("linked to wrong element")
					}
				}
			}
		}
		if links != 1 || !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
			t.Fatal("incorrect links or error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v176TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
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
	if v176TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v176TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v176TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v176TestLine задаёт структуру строки без повторного разбора DSL.
func v176TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v176TestOpen задаёт блочное объявление без параметров с явным родителем.
func v176TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v176TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v176TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v176TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v176TestClose задаёт закрытие указанного блока.
func v176TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v176TestLine(t, "}", "block-end", parent, depth, v176TestElement(t, "block-close", "}", "}", 1))
}

// v176TestInline задаёт известный тег с обычным содержимым без экранирования.
func v176TestInline(t *testing.T, name, content string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v176TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if content != "" {
		elements = append(elements, v176TestElement(t, "content", content, content, len(name)+3))
		raw += " " + content
	}
	return v176TestLine(t, raw, "tag", parent, depth, elements...)
}

// v176TestText сохраняет исходное и семантическое содержимое независимо.
func v176TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v176TestLine(t, raw, "content", parent, depth, v176TestElement(t, "content", raw, value, 1))
}

// v176TestSeparator задаёт собственную служебную строку без элементов.
func v176TestSeparator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v176TestLine(t, raw, "separator", parent, depth)
}

// v176TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v176TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v176TestOpen(t, name, 2, 1)}, body...)
	return v176TestDocument(t, append(lines, v176TestClose(t, 3, 2))...)
}

// v176TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v176TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v176TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v176TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v176TestElement(t, "version", "1.2", "1.2", 14)),
		v176TestLine(t, "# Lesson", "heading", 0, 0, v176TestElement(t, "heading-level", "#", "1", 1), v176TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Ошибка исходной конструкции.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}})
		}
	}
	return result
}

// v176TestRegistry использует только полный штатный каталог.
func v176TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v176TestSnapshot сохраняет независимое представление модели для сравнения.
func v176TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

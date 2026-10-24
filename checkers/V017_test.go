package checkers

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV017Registration(t *testing.T) {
	definitions, err := v017Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V017" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Перед знаком препинания находится пробел." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V017 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V017 definitions = %d, want 1", matches)
	}
}

func TestV017Punctuation(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Wait, please!", nil},
		{"example", "Wait , please !", [][3]int{{3, 5, 6}, {3, 14, 15}}},
		{"comma", "Wait ,", [][3]int{{3, 5, 6}}},
		{"period", "Wait .", [][3]int{{3, 5, 6}}},
		{"question mark", "Wait ?", [][3]int{{3, 5, 6}}},
		{"exclamation mark", "Wait !", [][3]int{{3, 5, 6}}},
		{"standalone ellipsis character", "Wait …", nil},
		{"standalone three contiguous dots", "Wait ...", nil},
		{"four contiguous dots", "Wait ....", [][3]int{{3, 5, 6}}},
		{"six contiguous dots", "Wait ......", [][3]int{{3, 5, 6}}},
		{"two contiguous dots", "Wait ..", [][3]int{{3, 5, 6}}},
		{"punctuation only", ",.?!…...", nil},
		{"space after punctuation", "Wait! please", nil},
		{"colon and semicolon", "Wait : next ;", [][3]int{{3, 5, 6}, {3, 12, 13}}},
		{"adjacent punctuation", "Wait ?!", [][3]int{{3, 5, 6}}},
		{"separated punctuation", "Wait ? !", [][3]int{{3, 5, 6}, {3, 7, 8}}},
		{"mixed contiguous punctuation", "Wait ...?!", [][3]int{{3, 5, 6}}},
		{"uppercase", "WAIT !", [][3]int{{3, 5, 6}}},
		{"Unicode columns", "Я😀 ,", [][3]int{{3, 3, 4}}},
		{"combining mark columns", "e\u0301 !", [][3]int{{3, 3, 4}}},
		{"HTML entity not decoded", "Wait &#33;", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v017Run(t, v017Document(t, v017Text(t, tc.raw, tc.raw, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV017AnswerRepresentations(t *testing.T) {
	for _, separate := range []bool{false, true} {
		v017Run(t, v015AnswerDocument(t, "", "Word , next", "", separate), "current", [][3]int{{4, 11, 12}}, 1)
		v017Run(t, v015AnswerDocument(t, "", "Word : next ;", "", separate), "current", [][3]int{{4, 11, 12}, {4, 18, 19}}, 1)
		v017Run(t, v015AnswerDocument(t, " ", ",", "", separate), "current", nil, 1)
		v017Run(t, v015AnswerDocument(t, "", "word ", ",", separate), "current", nil, 1)
		v017Run(t, v015AnswerDocument(t, "", "Hi, I'm …", "Next", separate), "current", nil, 1)
		v017Run(t, v015AnswerDocument(t, "Я😀 ", `a\} ,`, " !", separate), "current", [][3]int{{4, 13, 14}, {4, 16, 17}}, 1)
		v017Run(t, v015AnswerDocument(t, "Wait ...", "x", "", separate), "current", [][3]int{{4, 5, 6}}, 1)
		v017Run(t, v015AnswerDocument(t, "", "Word <em>,</em>", "", separate), "current", nil, 1)
	}
}

func TestV017StandaloneEllipsis(t *testing.T) {
	for _, raw := range []string{"This is …", "Hi, I'm …", "This is ...", "This is … next", "This is ... next", `"This is …"`, "(This is ...)", "This is <em>…</em>", "This is <em>...</em>", "This is .<em>.</em>.", "This is\u00a0…"} {
		t.Run(raw, func(t *testing.T) {
			v017Run(t, v020TextBlock(t, raw), "current", nil, 1)
		})
	}
	for _, raw := range []string{"Wait …next", "Wait ...next", "Wait ....", "Wait ...?!", "Wait …!", "Wait ,", "Wait .", "Wait .."} {
		t.Run(raw, func(t *testing.T) {
			v017Run(t, v020TextBlock(t, raw), "current", [][3]int{{4, 5, 6}}, 1)
		})
	}
	// Исключение многоточия не скрывает другое нарушение в той же строке.
	v017Run(t, v020TextBlock(t, "Hi , I'm …"), "current", [][3]int{{4, 3, 4}}, 1)
}

func TestV017HorizontalSpaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"two spaces and tab", "Wait  \t!", [][3]int{{3, 5, 8}}},
		{"tab only", "Wait\t!", [][3]int{{3, 5, 6}}},
		{"nonbreaking spaces", "Wait\u00a0\u202f!", [][3]int{{3, 5, 7}}},
		{"em and ideographic spaces", "Wait\u2003\u3000!", [][3]int{{3, 5, 7}}},
		{"mixed spaces", "Wait \t\u2003 !", [][3]int{{3, 5, 9}}},
		{"zero width space", "Wait\u200b!", nil},
		{"line separator", "Wait\u2028!", nil},
		{"paragraph separator", "Wait\u2029!", nil},
		{"non-horizontal breaks adjacency", "Wait \u200b!", nil},
		{"only immediately adjacent run", "Wait \u200b !", [][3]int{{3, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v017Run(t, v017Document(t, v017Text(t, tc.raw, tc.raw, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV017SpacedEllipses(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"three spaced dots", "Wait . . .", nil},
		{"gap before last dot", "Wait .. .", nil},
		{"gap after first dot", "Wait . ..", nil},
		{"four spaced dots", "Wait . . . .", [][3]int{{3, 5, 6}}},
		{"long mixed chain", "Wait ... .. .", [][3]int{{3, 5, 6}}},
		{"spaced ellipsis without preceding space", ". . .", nil},
		{"mixed ellipsis without preceding space", ".. .", nil},
		{"two dots below threshold", "Wait . .", [][3]int{{3, 5, 6}, {3, 7, 8}}},
		{"two dots alone", ". .", [][3]int{{3, 2, 3}}},
		{"two dots with multiple spaces", "Wait .  .", [][3]int{{3, 5, 6}, {3, 7, 9}}},
		{"tabs and Zs between dots", "Wait .\t.\u00a0.", nil},
		{"independent chains", "Wait . . . and . . .", nil},
		{"punctuation after chain", "Wait . . . ,", [][3]int{{3, 11, 12}}},
		{"comma breaks chain", "Wait . . , .", [][3]int{{3, 5, 6}, {3, 7, 8}, {3, 9, 10}, {3, 11, 12}}},
		{"ellipsis character is not a dot", "Wait . … .", [][3]int{{3, 5, 6}, {3, 7, 8}, {3, 9, 10}}},
		{"zero width separator breaks chain", ". .\u200b .", [][3]int{{3, 2, 3}, {3, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v017Run(t, v017Document(t, v017Text(t, tc.raw, tc.raw, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV017RawPositionsAndTextKinds(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		// DSL 1.21–1.22: Value снимает экранирование скобок, позиции берутся из Raw.
		{"escaped braces", v017Text(t, `\{x\} !`, "{x} !", 2, 1), [][3]int{{3, 6, 7}}},
		{"element offset", v017Line(t, "  Wait !  ", "content", 2, 1,
			v017Element(t, "content", "Wait !", "Wait !", 3)), [][3]int{{3, 7, 8}}},
		{"single-line comment", v017Line(t, "@editor Wait !", "tag", 2, 1,
			v017Element(t, "tag", "@editor", "editor", 1), v017Element(t, "content", "Wait !", "Wait !", 9)), [][3]int{{3, 13, 14}}},
		{"blank physical line", v017Line(t, "", "blank", 2, 1), nil},
		{"blank whitespace line", v017Line(t, " \t ", "blank", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v017Run(t, v017Document(t, tc.line), "current", tc.want, 1)
		})
	}
	t.Run("heading title", func(t *testing.T) {
		result := v017Document(t)
		title := "Wait !"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v017Line(t, "# Wait !", "heading", 0, 0,
			v017Element(t, "heading-level", "#", "1", 1), v017Element(t, "title", title, title, 3))
		v017NumberLines(t, result)
		v017Run(t, result, "current", [][3]int{{2, 7, 8}}, 1)
	})
	t.Run("section name", func(t *testing.T) {
		result := v017Document(t)
		section := "Wait !"
		result.Document.Metadata.Section = &section
		line := v017Line(t, "@section Wait !", "tag", 0, 0,
			v017Element(t, "tag", "@section", "section", 1), v017Element(t, "name", section, section, 10))
		result.Lines = append(result.Lines[:1], line, result.Lines[1])
		v017NumberLines(t, result)
		v017Run(t, result, "current", [][3]int{{2, 14, 15}}, 1)
	})
}

func TestV017FormattingNeighbours(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		want       [][3]int
		wantErrors []v017ErrorExpectation
	}{
		{"Hello <em>,</em> world", [][3]int{{4, 6, 7}}, nil},
		{"Hello<em>,</em> world", nil, nil},
		{"Wait <em> </em>,", [][3]int{{4, 5, 6}, {4, 10, 11}}, nil},
		{"Я😀\u00a0<u><EM>!</EM></u>", [][3]int{{4, 3, 4}}, nil},
		{"Wait <em>:</em> Next <u>;</u>", [][3]int{{4, 5, 6}, {4, 21, 22}}, nil},
		{"Wait <em>...</em>", nil, nil},
		{". <em>.</em> .", nil, nil},
		{"Wait <em>. . .</em>", nil, nil},
		{`Hello \<em>,</em>`, nil, []v017ErrorExpectation{{"V112", [3]int{4, 13, 18}}}},
		{`Hello <em\>,</em>`, nil, []v017ErrorExpectation{{"V112", [3]int{4, 13, 18}}}},
		{"Hello <foo>,</foo>", nil, nil},
		{"Hello <em x=y>,</em>", nil, []v017ErrorExpectation{{"V114", [3]int{4, 11, 14}}}},
		{"Hello <br>,", nil, []v017ErrorExpectation{{"V111", [3]int{4, 1, 12}}}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v020TextBlock(t, tc.raw)
			v017Run(t, result, "current", tc.want, 1, tc.wantErrors...)
			if tc.raw == ". <em>.</em> ." {
				visibleExpect(t, result, "current", "V003", []visibleExpectation{{Range: [2]int{1, 15}}})
			}
		})
	}
	result := v020TextBlock(t, "Hello <em>,</em>")
	result.Lines[3].Elements = []model.Element{
		v017Element(t, "content", "Hello ", "Hello ", 1),
		v017Element(t, "content", "<em>,</em>", "<em>,</em>", 7),
	}
	v017Run(t, result, "current", nil, 1)
}

func TestV017HTMLIsOrdinaryText(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"marker interrupts adjacency", "Wait <em>,</em>", nil},
		{"spaces separated by markers", "Wait <em> </em>,", nil},
		{"space directly before comma", "<em>Wait ,</em>", [][3]int{{4, 9, 10}}},
		{"attribute text is not excluded", `<x title="A ,">`, [][3]int{{4, 12, 13}}},
		{"markers interrupt dot chain", ". <em>.</em> .", [][3]int{{4, 13, 14}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 2.8.11: литеральный комментарий позволяет HTML-подобный текст
			// без отдельного нарушения DSL из-за неизвестного тега или атрибута.
			result := v017Document(t,
				v017Line(t, "@editor {", "block-start", 2, 1,
					v017Element(t, "tag", "@editor", "editor", 1), v017Element(t, "block-open", "{", "{", 9)),
				v017Text(t, tc.raw, tc.raw, 3, 2),
				v017Line(t, "}", "block-end", 3, 2, v017Element(t, "block-close", "}", "}", 1)),
			)
			v017Run(t, result, "current", tc.want, 1)
		})
	}
}

func TestV017BoundariesAndNesting(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"nested hint", []model.Line{
			v017Line(t, "@instr {", "block-start", 2, 1,
				v017Element(t, "tag", "@instr", "instr", 1), v017Element(t, "block-open", "{", "{", 8)),
			v017Text(t, "Read the sentence.", "Read the sentence.", 3, 2),
			v017Line(t, "@hint {", "block-start", 3, 2,
				v017Element(t, "tag", "@hint", "hint", 1), v017Element(t, "block-open", "{", "{", 7)),
			v017Text(t, "Wait !", "Wait !", 5, 3),
			v017Line(t, "}", "block-end", 5, 3, v017Element(t, "block-close", "}", "}", 1)),
			v017Line(t, "}", "block-end", 3, 2, v017Element(t, "block-close", "}", "}", 1)),
		}, [][3]int{{6, 5, 6}}},
		{"separate table cells", []model.Line{
			v017Line(t, "@table {", "block-start", 2, 1,
				v017Element(t, "tag", "@table", "table", 1), v017Element(t, "block-open", "{", "{", 8)),
			v017Line(t, "Wait | !", "content", 3, 2,
				v017Element(t, "content", "Wait", "Wait", 1), v017Element(t, "content", "!", "!", 8)),
			v017Line(t, "}", "block-end", 3, 2, v017Element(t, "block-close", "}", "}", 1)),
		}, nil},
		{"dot chain cannot cross table cells", []model.Line{
			v017Line(t, "@table {", "block-start", 2, 1,
				v017Element(t, "tag", "@table", "table", 1), v017Element(t, "block-open", "{", "{", 8)),
			v017Line(t, ". . | .", "content", 3, 2,
				v017Element(t, "content", ". .", ". .", 1), v017Element(t, "content", ".", ".", 7)),
			v017Line(t, "}", "block-end", 3, 2, v017Element(t, "block-close", "}", "}", 1)),
		}, [][3]int{{4, 2, 3}}},
		{"separate physical lines", []model.Line{
			v017Line(t, "Wait ", "content", 2, 1, v017Element(t, "content", "Wait", "Wait", 1)),
			v017Text(t, "!", "!", 2, 1),
		}, nil},
		{"dot chain cannot cross physical lines", []model.Line{
			v017Text(t, ". .", ". .", 2, 1), v017Text(t, ".", ".", 2, 1),
		}, [][3]int{{3, 2, 3}}},
		// DSL 3.2: части примера по обе стороны служебного --- непустые.
		{"block separator", []model.Line{
			v017Line(t, "@example {", "block-start", 2, 1,
				v017Element(t, "tag", "@example", "example", 1), v017Element(t, "block-open", "{", "{", 10)),
			v017Text(t, "Wait ,", "Wait ,", 3, 2),
			v017Line(t, "---", "separator", 3, 2),
			v017Text(t, "Wait !", "Wait !", 3, 2),
			v017Line(t, "}", "block-end", 3, 2, v017Element(t, "block-close", "}", "}", 1)),
		}, [][3]int{{4, 5, 6}, {6, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v017Run(t, v017Document(t, tc.body...), "current", tc.want, 1)
		})
	}
}

func TestV017ExcludedElements(t *testing.T) {
	t.Run("media source", func(t *testing.T) {
		line := v017Line(t, `@media audio "a .wav"`, "tag", 2, 1,
			v017Element(t, "tag", "@media", "media", 1), v017Element(t, "media-type", "audio", "audio", 8),
			v017Element(t, "source", `"a .wav"`, "a .wav", 14))
		v017Run(t, v017Document(t, line), "current", nil, 1)
	})
	t.Run("resource path", func(t *testing.T) {
		result := v017Document(t)
		line := v017Line(t, `@resource-dir "a ."`, "tag", 0, 0,
			v017Element(t, "tag", "@resource-dir", "resource-dir", 1), v017Element(t, "resource-path", `"a ."`, "a .", 15))
		result.Lines = append(result.Lines[:1], line, result.Lines[1])
		result.Document.Metadata.ResourceDirs = []string{"a ."}
		v017NumberLines(t, result)
		v017Run(t, result, "current", nil, 1)
	})
	t.Run("identifier", func(t *testing.T) {
		result := v017Document(t)
		id := "unit.1"
		line := v017Line(t, "@document-id unit.1", "tag", 0, 0,
			v017Element(t, "tag", "@document-id", "document-id", 1), v017Element(t, "identifier", id, id, 14))
		result.Lines = append(result.Lines[:1], line, result.Lines[1])
		result.Document.Metadata.DocumentID = &id
		v017NumberLines(t, result)
		v017Run(t, result, "current", nil, 0)
	})
	t.Run("unparsed with parser history", func(t *testing.T) {
		element := v017Element(t, "unparsed", "@unknown Wait !", "", 1)
		element.Value, element.ErrorIDs = nil, []string{"parser-error"}
		result := v017Document(t, v017Line(t, "@unknown Wait !", "invalid", 2, 1, element))
		result.Document.HasErrors, result.Lines[2].HasErrors = true, true
		result.Diagnostics = []model.Diagnostic{{
			ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
			DiagnosticScope: "element", Message: "Неизвестное имя тега.",
			Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 16}},
			RelatedLocations: []model.Location{},
		}}
		v017Run(t, result, "current", nil, 1)
	})
}

func TestV017LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}},
		{"no final EOL", [3]model.LineEnding{"\n", "\n", ""}},
		{"mixed endings", [3]model.LineEnding{"\r\n", "\n", "\r\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v017Document(t, v017Text(t, "Wait , please !", "Wait , please !", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v017Run(t, result, "current", [][3]int{{3, 5, 6}, {3, 14, 15}}, 1)
		})
	}
}

func TestV017UnavailableLinesAndVersions(t *testing.T) {
	v017Run(t, v017UnavailableDocument(t, "1.2"), "current", nil, 0)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v017UnavailableDocument(t, version)
			before := v017Snapshot(t, result)
			checks, err := validator.Prepare(result, v017Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v017Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV017HistoryAndOtherChecks(t *testing.T) {
	result := v017Document(t, v017Text(t, "A  \t1-2 !", "A  \t1-2 !", 2, 1))
	v017Run(t, result, "first", [][3]int{{3, 8, 9}}, 1)
	for _, tc := range []struct {
		code     diagnostics.Code
		severity diagnostics.Severity
		message  string
		start    int
		end      int
	}{
		{"V001", "warning", "Несколько пробелов подряд.", 2, 4},
		{"V002", "warning", "В строке есть табуляция.", 4, 5},
		{"V012", "recommendation", "Между числами используется дефис или длинное тире.\nконкретизация: Если это числовой диапазон, используйте короткое тире: 1–12", 6, 7},
	} {
		matches := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "first" || entry.DiagnosticCode != tc.code {
				continue
			}
			matches++
			want := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
			if entry.Location == nil || *entry.Location != want || entry.Message != tc.message ||
				entry.SeverityLevel != tc.severity || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
		if matches != 1 {
			t.Errorf("current %s count = %d, want 1", tc.code, matches)
		}
	}
	v017Run(t, result, "second", [][3]int{{3, 8, 9}}, 1)
	v017Run(t, v017Document(t, v017Text(t, "Wait!", "Wait!", 2, 1)), "first", nil, 1)
}

// v017Registry создаёт реестр исключительно через штатный каталог регистраций.
func v017Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v017UnavailableDocument представляет отсутствие доступных строк
// при известной версии DSL, а не пустой исходник с выдуманным тегом версии.
func v017UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v017Document дополняет фрагменты технической строкой и заголовком по DSL 1.30.
// Содержимое фрагментов сохраняется; обычный текст образует шаг по DSL 2.1.15.
// Родители, вложенность, элементы и семантические значения задаются тестом.
func v017Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v017UnavailableDocument(t, "1.2")
	title, encoding := "Unit 1", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v017Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v017Element(t, "tag", "@dsl-version", "dsl-version", 1), v017Element(t, "version", "1.2", "1.2", 14)),
		v017Line(t, "# Unit 1", "heading", 0, 0,
			v017Element(t, "heading-level", "#", "1", 1), v017Element(t, "title", "Unit 1", "Unit 1", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v017NumberLines(t, result)
	return result
}

// v017NumberLines назначает физические номера уже заданным строкам модели.
func v017NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v017Line создаёт строку с заданной структурой, не распознавая Raw.
func v017Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v017Element сохраняет явные Raw и Value; длина нужна только для входа.
// Ожидаемые диапазоны диагностики заданы отдельно и не вычисляются по Raw.
func v017Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v017Text создаёт готовую строку содержимого с явно заданным Value.
func v017Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v017Line(t, raw, "content", parent, depth, v017Element(t, "content", raw, value, 1))
}

// v017ErrorExpectation задаёт код сопутствующей ошибки и её диапазон.
// location содержит номер строки, начало и исключающий конец в столбцах.
type v017ErrorExpectation struct {
	code     diagnostics.Code
	location [3]int
}

// v017Run запускает штатный конвейер и проверяет V017 текущего запуска.
// want содержит явные тройки: строка, начало, исключающий конец диапазона.
// wantV037 и wantErrors задают точные ожидания сопутствующих ошибок.
// Флаги документа и строк проверяются с учётом этих ожиданий; исходные
// данные, ссылки элементов и прежняя история должны сохраняться.
func v017Run(t *testing.T, result *model.Result, processingID string, want [][3]int, wantV037 int, wantErrors ...v017ErrorExpectation) {
	t.Helper()
	before := v017Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	oldLineErrors := make([]bool, len(result.Lines))
	for i, line := range result.Lines {
		oldLineErrors[i] = line.HasErrors
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v017Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v017Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got, gotErrors []model.Diagnostic
	gotV037 := 0
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.Source != processingID {
			t.Fatalf("new diagnostic has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V037" && entry.Source == processingID {
			gotV037++
			if entry.SeverityLevel != "error" || entry.DiagnosticScope != "document" ||
				entry.Message != "В документе отсутствует @document-id." || entry.Location != nil ||
				entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected accompanying V037: %+v", entry)
			}
		}
		if (entry.DiagnosticCode == "V111" || entry.DiagnosticCode == "V112" || entry.DiagnosticCode == "V114") && entry.Source == processingID {
			gotErrors = append(gotErrors, entry)
		}
		if entry.DiagnosticCode == "V017" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V017 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[0], Column: expected[2]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V017" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			!strings.HasPrefix(entry.Message, "Перед знаком препинания находится пробел.\nконкретизация: Обратите внимание на знак препинания: «") || !strings.HasSuffix(entry.Message, "»") || entry.DiagnosticScope != "line" ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(gotErrors) != len(wantErrors) {
		t.Fatalf("current V111/V112/V114 count = %d, want %d; diagnostics: %+v", len(gotErrors), len(wantErrors), gotErrors)
	}
	messages := map[diagnostics.Code]string{
		"V111": "Команда пустой строки отличается от отдельной записи <br>.\nконкретизация: другое содержимое строки",
		"V112": "Тег выделения не имеет пары в своей строке.\nконкретизация: Открытие и закрытие выделения должны находиться в одной текстовой строке",
		"V114": "В теге выделения записаны атрибуты.",
	}
	wantLineErrors := append([]bool{}, oldLineErrors...)
	for i, expected := range wantErrors {
		entry := gotErrors[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		if entry.DiagnosticCode != expected.code || entry.Location == nil || *entry.Location != location || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != messages[expected.code] {
			t.Errorf("unexpected accompanying diagnostic: %+v; want %s at %+v", entry, expected.code, location)
		}
		wantLineErrors[expected.location[0]-1] = true
	}
	if len(result.Lines) != len(wantLineErrors) {
		t.Fatal("line count changed")
	}
	for i, line := range result.Lines {
		if line.HasErrors != wantLineErrors[i] {
			t.Errorf("line %d hasErrors = %v, want %v", line.Number, line.HasErrors, wantLineErrors[i])
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost diagnostic history")
	}
	if result.Document.HasErrors != (oldDocumentError || wantV037 > 0 || len(wantErrors) > 0) {
		t.Errorf("document.hasErrors = %v, want %v", result.Document.HasErrors, oldDocumentError || wantV037 > 0 || len(wantErrors) > 0)
	}
	previous := *result
	previous.Lines = append([]model.Line{}, result.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = oldLineErrors[i]
	}
	previous.Document.HasErrors = oldDocumentError
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v017Snapshot(t, &previous) != before {
		t.Fatal("source, structure, error flags, element links or previous history changed")
	}
}

// v017Snapshot фиксирует всю модель для сравнения исходных данных и истории.
func v017Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

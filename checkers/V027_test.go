package checkers

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV027Registration(t *testing.T) {
	definitions, err := v027Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, d := range definitions {
		if d.Code != "V027" {
			continue
		}
		matches++
		if d.Severity != "warning" || d.Scope != "line" || d.Check == nil ||
			d.Message != "Текстовый фрагмент выделен одинарными кавычками." ||
			d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", d)
		}
	}
	if matches != 1 {
		t.Fatalf("V027 definitions = %d, want 1", matches)
	}
}

func TestV027Pairs(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][5]int
	}{
		{"reference", `I'm ready. He said "Be here at six".`, nil},
		{"example", "Phoned the (taxi driver) to say 'Be here at six'?", [][5]int{{3, 33, 34, 48, 49}}},
		{"line boundaries", "'word'", [][5]int{{3, 1, 2, 6, 7}}},
		{"parentheses", "('word')", [][5]int{{3, 2, 3, 7, 8}}},
		{"Unicode opening punctuation", "«'word'»", [][5]int{{3, 2, 3, 7, 8}}},
		{"square brackets", "['word']", [][5]int{{3, 2, 3, 7, 8}}},
		{"double quotes", `"'word'"`, [][5]int{{3, 2, 3, 7, 8}}},
		{"two pairs", "'hello' 'world'", [][5]int{{3, 1, 2, 7, 8}, {3, 9, 10, 15, 16}}},
		{"nearest closure", "'a' and 'b'", [][5]int{{3, 1, 2, 3, 4}, {3, 9, 10, 11, 12}}},
		{"apostrophe inside quotation", "'don't'", [][5]int{{3, 1, 2, 7, 8}}},
		{"one character", "'x'", [][5]int{{3, 1, 2, 3, 4}}},
		{"digit content not abbreviated year", "'7'", [][5]int{{3, 1, 2, 3, 4}}},
		{"unpaired opening", "'word", nil},
		{"unpaired trailing apostrophe", "word'", nil},
		{"only apostrophe", "'", nil},
		{"empty pair", "''", nil},
		{"space after opening", "' word'", nil},
		{"space before closing", "'word '", nil},
		{"whitespace only inside", "' \t '", nil},
		{"possessive", "dogs' toys", nil},
		{"possessive before pair", "dogs' 'word'", [][5]int{{3, 7, 8, 12, 13}}},
		{"lowercase letters before sign", "x'word'", nil},
		{"closing punctuation cannot open", "!'word'", nil},
		{"symbol cannot open", "/'word'/", nil},
		{"symbol cannot follow closure", "'word'$", nil},
		{"punctuation after closure", "'word',", [][5]int{{3, 1, 2, 6, 7}}},
		{"space after closure", "'word' next", [][5]int{{3, 1, 2, 6, 7}}},
		{"letter after sign is apostrophe", "'word'x", nil},
		{"invalid close then valid close", "'a'7 b'", [][5]int{{3, 1, 2, 7, 8}}},
		{"nonoverlapping pairs", "'a 'b' c'", [][5]int{{3, 1, 2, 6, 7}}},
		{"curly quotation marks", "‘word’", nil},
		{"Unicode columns", "Я😀 'word'", [][5]int{{3, 4, 5, 9, 10}}},
		{"spaces not trimmed", "  'word'  ", [][5]int{{3, 3, 4, 8, 9}}},
		{"tabs", "\t'word'\t", [][5]int{{3, 2, 3, 7, 8}}},
		{"Unicode spaces", "\u00a0'word'\u2003", [][5]int{{3, 2, 3, 7, 8}}},
		{"Unicode space immediately inside", "'word\u00a0'", nil},
		{"elision prefix is not exception", "'tissue'", [][5]int{{3, 1, 2, 8, 9}}},
		{"elision with digit suffix", "'tis2'", [][5]int{{3, 1, 2, 6, 7}}},
		{"elision with diacritic suffix", "'tis\u0301'", [][5]int{{3, 1, 2, 6, 7}}},
		{"year with letter suffix", "'24x'", [][5]int{{3, 1, 2, 5, 6}}},
		{"year with extra digit", "'240'", [][5]int{{3, 1, 2, 5, 6}}},
		{"n without surrounding words", "'n'", [][5]int{{3, 1, 2, 3, 4}}},
		{"n missing right word", "rock 'n'", [][5]int{{3, 6, 7, 8, 9}}},
		{"n missing left word", "'n' roll", [][5]int{{3, 1, 2, 3, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v027Run(t, v027Document(t, v027Text(t, tc.raw, tc.raw, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV027ApostropheExceptions(t *testing.T) {
	for _, raw := range []string{
		"I'm don't l'homme d'Artagnan o'clock", "e\u0301'm", "e'm\u0301", "я'я", "A\u0301'B",
		"'tis'", "'twas'", "'twere'", "'twill'", "'twould'", "'em'", "'cause'", "'cos'", "'cuz'", "'bout'", "'til'",
		"'TIS'", "'TWAS'", "'TWERE'", "'TWILL'", "'TWOULD'", "'EM'", "'CAUSE'", "'COS'", "'CUZ'", "'BOUT'", "'TIL'",
		"'90s'", "'24'", "'90S'", "'90s '24 '90S",
		"rock 'n' roll", "rock 'N' roll", "rock\t'n'\u00a0roll",
	} {
		t.Run(raw, func(t *testing.T) {
			v027Run(t, v027Document(t, v027Text(t, raw, raw, 2, 1)), "current", nil, 1)
		})
	}
	// Исключённый знак года не закрывает уже открытую пару.
	v027Run(t, v027Document(t, v027Text(t, "'from '24 to '90s'", "'from '24 to '90s'", 2, 1)), "current", [][5]int{{3, 1, 2, 18, 19}}, 1)
	// Диакритика перед внутренним апострофом не разрывает слово.
	v027Run(t, v027Document(t, v027Text(t, "'e\u0301'm'", "'e\u0301'm'", 2, 1)), "current", [][5]int{{3, 1, 2, 6, 7}}, 1)
}

func TestV027Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][5]int
	}{
		{"comment", v027Line(t, "@editor 'word'", "tag", 2, 1, v027Element(t, "tag", "@editor", "editor", 1), v027Element(t, "content", "'word'", "'word'", 9)), [][5]int{{3, 9, 10, 14, 15}}},
		{"media path", v027Line(t, `@media audio "a 'word'.wav"`, "tag", 2, 1, v027Element(t, "tag", "@media", "media", 1), v027Element(t, "media-type", "audio", "audio", 8), v027Element(t, "source", `"a 'word'.wav"`, "a 'word'.wav", 14)), [][5]int{{3, 17, 18, 22, 23}}},
		{"escaped braces raw positions", v027Text(t, `\{'word'\}`, "{'word'}", 2, 1), [][5]int{{3, 3, 4, 8, 9}}},
		{"indented raw positions", v027Line(t, "  'word'  ", "content", 2, 1, v027Element(t, "content", "'word'", "'word'", 3)), [][5]int{{3, 3, 4, 8, 9}}},
		{"blank physical line", v027Line(t, "", "blank", 2, 1), nil},
		{"blank whitespace line", v027Line(t, " \t ", "blank", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v027Run(t, v027Document(t, tc.line), "current", tc.want, 1) })
	}
	for _, tc := range []struct {
		name, tag, raw string
		elements       []model.Element
		want           [][5]int
	}{
		{"HTML no opening boundary", "editor", "<b>'word'</b>", []model.Element{v027Element(t, "content", "<b>'word'</b>", "<b>'word'</b>", 1)}, nil},
		// Угловые скобки — символы Unicode Sm, а не пунктуация.
		{"HTML no closing boundary", "editor", "<b> 'word'</b>", []model.Element{v027Element(t, "content", "<b> 'word'</b>", "<b> 'word'</b>", 1)}, nil},
		{"HTML with both boundaries", "editor", "<b> 'word' </b>", []model.Element{v027Element(t, "content", "<b> 'word' </b>", "<b> 'word' </b>", 1)}, [][5]int{{4, 5, 6, 10, 11}}},
		{"HTML closing symbol", "editor", "<b title= 'word'>ok</b>", []model.Element{v027Element(t, "content", "<b title= 'word'>ok</b>", "<b title= 'word'>ok</b>", 1)}, nil},
		{"HTML attribute included", "editor", "<b title= 'word' >ok</b>", []model.Element{v027Element(t, "content", "<b title= 'word' >ok</b>", "<b title= 'word' >ok</b>", 1)}, [][5]int{{4, 11, 12, 16, 17}}},
		{"HTML entities not decoded", "editor", "&#39;word&#39;", []model.Element{v027Element(t, "content", "&#39;word&#39;", "&#39;word&#39;", 1)}, nil},
		{"table cells one matching region", "table", "'one | two'", []model.Element{v027Element(t, "content", "'one", "'one", 1), v027Element(t, "content", "two'", "two'", 8)}, [][5]int{{4, 1, 2, 11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v027Document(t,
				v027Line(t, "@"+tc.tag+" {", "block-start", 2, 1, v027Element(t, "tag", "@"+tc.tag, tc.tag, 1), v027Element(t, "block-open", "{", "{", len(tc.tag)+3)),
				v027Line(t, tc.raw, "content", 3, 2, tc.elements...),
				v027Line(t, "}", "block-end", 3, 2, v027Element(t, "block-close", "}", "}", 1)))
			v027Run(t, result, "current", tc.want, 1)
		})
	}
	t.Run("title", func(t *testing.T) {
		result := v027Document(t)
		title := "'word'"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v027Line(t, "# 'word'", "heading", 0, 0, v027Element(t, "heading-level", "#", "1", 1), v027Element(t, "title", title, title, 3))
		v027NumberLines(t, result)
		v027Run(t, result, "current", [][5]int{{2, 3, 4, 8, 9}}, 1)
	})
	for _, tc := range []struct {
		tag        string
		kind       model.ElementType
		start, end int
	}{
		{"section", "name", 10, 15}, {"resource-dir", "resource-path", 15, 20},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			result := v027Document(t)
			value := "'word'"
			if tc.tag == "section" {
				result.Document.Metadata.Section = &value
			} else {
				result.Document.Metadata.ResourceDirs = []string{value}
			}
			line := v027Line(t, "@"+tc.tag+" "+value, "tag", 0, 0, v027Element(t, "tag", "@"+tc.tag, tc.tag, 1), v027Element(t, tc.kind, value, value, tc.start))
			result.Lines = append(result.Lines[:1], line, result.Lines[1])
			v027NumberLines(t, result)
			v027Run(t, result, "current", [][5]int{{2, tc.start, tc.start + 1, tc.end, tc.end + 1}}, 1)
		})
	}
	t.Run("unparsed", func(t *testing.T) {
		element := v027Element(t, "unparsed", "@unknown 'word'", "", 1)
		element.Value, element.ErrorIDs = nil, []string{"parser-error"}
		result := v027Document(t, v027Line(t, "@unknown 'word'", "invalid", 2, 1, element))
		result.Document.HasErrors, result.Lines[2].HasErrors = true, true
		result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестное имя тега.", Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 16}}, RelatedLocations: []model.Location{}}}
		v027Run(t, result, "current", [][5]int{{3, 10, 11, 15, 16}}, 1)
	})
}

func TestV027BoundariesAndNesting(t *testing.T) {
	v027Run(t, v027UnavailableDocument(t, "1.2"), "current", nil, 0)
	v027Run(t, v027Document(t, v027Text(t, "'word", "'word", 2, 1), v027Text(t, "word'", "word'", 2, 1)), "current", nil, 1)
	result := v027Document(t,
		v027Text(t, "'first'", "'first'", 2, 1),
		v027Line(t, "@instr {", "block-start", 2, 1, v027Element(t, "tag", "@instr", "instr", 1), v027Element(t, "block-open", "{", "{", 8)),
		v027Text(t, "Read the sentence.", "Read the sentence.", 4, 2),
		v027Line(t, "@hint {", "block-start", 4, 2, v027Element(t, "tag", "@hint", "hint", 1), v027Element(t, "block-open", "{", "{", 7)),
		v027Text(t, "'nested'", "'nested'", 6, 3),
		v027Line(t, "}", "block-end", 6, 3, v027Element(t, "block-close", "}", "}", 1)),
		v027Line(t, "}", "block-end", 4, 2, v027Element(t, "block-close", "}", "}", 1)),
		v027Line(t, "@example {", "block-start", 2, 1, v027Element(t, "tag", "@example", "example", 1), v027Element(t, "block-open", "{", "{", 10)),
		v027Text(t, "'word", "'word", 10, 2), v027Line(t, "---", "separator", 10, 2), v027Text(t, "word'", "word'", 10, 2),
		v027Line(t, "}", "block-end", 10, 2, v027Element(t, "block-close", "}", "}", 1)),
		v027Text(t, "'last'", "'last'", 2, 1))
	v027Run(t, result, "current", [][5]int{{3, 1, 2, 7, 8}, {7, 1, 2, 8, 9}, {15, 1, 2, 6, 7}}, 1)
	for _, ending := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF, model.LineEndingNone} {
		t.Run("ending "+string(ending), func(t *testing.T) {
			result := v027Document(t, v027Text(t, "'word'", "'word'", 2, 1))
			if ending != model.LineEndingNone {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			} else {
				result.Lines[2].LineEnding = ending
			}
			v027Run(t, result, "current", [][5]int{{3, 1, 2, 6, 7}}, 1)
		})
	}
}

func TestV027VersionsAndHistory(t *testing.T) {
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v027UnavailableDocument(t, version)
			before := v027Snapshot(t, result)
			checks, err := validator.Prepare(result, v027Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v027Snapshot(t, result) != before {
				t.Fatal("unsupported version changed model")
			}
		})
	}
	result := v027Document(t, v027Text(t, "he1o  'word'", "he1o  'word'", 2, 1))
	v027Run(t, result, "first", [][5]int{{3, 7, 8, 12, 13}}, 1)
	for _, code := range []diagnostics.Code{"V001", "V022"} {
		count := 0
		for _, d := range result.Diagnostics {
			if d.Source == "first" && d.DiagnosticCode == code {
				count++
			}
		}
		if count != 1 {
			t.Errorf("other diagnostic %s count = %d, want 1", code, count)
		}
	}
	v027Run(t, result, "second", [][5]int{{3, 7, 8, 12, 13}}, 1)
	v027Run(t, v027Document(t, v027Text(t, "I'm ready.", "I'm ready.", 2, 1)), "first", nil, 1)
}

// v027Registry создаёт реестр исключительно через штатный каталог регистраций.
func v027Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v027UnavailableDocument представляет отсутствие доступных строк
// при известной версии DSL, а не пустой исходник с выдуманным тегом версии.
func v027UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v027Document дополняет фрагменты технической строкой и заголовком по DSL 1.30.
// Содержимое фрагментов сохраняется; обычный текст образует шаг по DSL 2.1.15.
// Родители, вложенность, элементы и семантические значения задаются тестом.
func v027Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v027UnavailableDocument(t, "1.2")
	title, encoding := "Unit 1", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v027Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v027Element(t, "tag", "@dsl-version", "dsl-version", 1), v027Element(t, "version", "1.2", "1.2", 14)),
		v027Line(t, "# Unit 1", "heading", 0, 0,
			v027Element(t, "heading-level", "#", "1", 1), v027Element(t, "title", "Unit 1", "Unit 1", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v027NumberLines(t, result)
	return result
}

// v027NumberLines назначает физические номера уже заданным строкам модели.
func v027NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v027Line создаёт строку с заданной структурой, не распознавая Raw.
func v027Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v027Element сохраняет явные Raw и Value; длина нужна только для входа.
// Ожидаемые диапазоны диагностики заданы отдельно и не вычисляются по Raw.
func v027Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v027Text создаёт готовую строку содержимого с явно заданным Value.
func v027Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v027Line(t, raw, "content", parent, depth, v027Element(t, "content", raw, value, 1))
}

// v027Run проверяет V027 текущего запуска через штатный конвейер.
// want содержит строку и явные границы основного и связанного диапазонов.
// Концы диапазонов исключаются.
// В этих ситуациях новые срабатывания других проверок не меняют флаги ошибок.
// wantV037 задаёт точное число сопутствующих ошибок отсутствующего ID;
// флаг документа проверяется отдельно, строки и прежняя история сохраняются.
func v027Run(t *testing.T, result *model.Result, processingID string, want [][5]int, wantV037 int) {
	t.Helper()
	before := v027Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v027Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v027Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
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
		if entry.DiagnosticCode == "V027" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V027 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		related := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[3]},
			End:   model.Position{Line: expected[0], Column: expected[4]},
		}
		if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("occurrence %d related = %+v, want %+v", i, entry.RelatedLocations, related)
		}

		if entry.DiagnosticCode != "V027" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "Текстовый фрагмент выделен одинарными кавычками.\nконкретизация: Для выделения фрагмента используйте двойные кавычки; внутрисловные апострофы сохраните" || entry.DiagnosticScope != "line" ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 1 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost diagnostic history")
	}
	if result.Document.HasErrors != (oldDocumentError || wantV037 > 0) {
		t.Errorf("document.hasErrors = %v, want %v", result.Document.HasErrors, oldDocumentError || wantV037 > 0)
	}
	previous := *result
	previous.Document.HasErrors = oldDocumentError
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	// В снимке исключаются новые ссылки других элементных диагностик каталога.
	newElementIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[len(previous.Diagnostics):] {
		if entry.DiagnosticScope == "element" && entry.DiagnosticCode != "V027" {
			newElementIDs[entry.ID] = true
		}
	}
	previous.Lines = append([]model.Line{}, previous.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].Elements = append([]model.Element{}, previous.Lines[i].Elements...)
		for j := range previous.Lines[i].Elements {
			element := &previous.Lines[i].Elements[j]
			element.ErrorIDs = slices.DeleteFunc(slices.Clone(element.ErrorIDs), func(id string) bool { return newElementIDs[id] })
		}
	}
	if v027Snapshot(t, &previous) != before {
		t.Fatal("source, structure, error flags, element links or previous history changed")
	}
}

// v027Snapshot фиксирует всю модель для сравнения исходных данных и истории.
func v027Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

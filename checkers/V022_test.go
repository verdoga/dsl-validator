package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV022Registration(t *testing.T) {
	definitions, err := v022Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, d := range definitions {
		if d.Code != "V022" {
			continue
		}
		matches++
		if d.Severity != "warning" || d.Scope != "line" || d.Check == nil ||
			d.Message != "Внутри буквенного слова находятся цифры, похожие на буквы." ||
			d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", d)
		}
	}
	if matches != 1 {
		t.Fatalf("V022 definitions = %d, want 1", matches)
	}
}

type v022Expected struct {
	line, start, end int
	detail           string
}

func TestV022Tokens(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v022Expected
	}{
		{"reference", "Say hello.", nil},
		{"example", "Say he11o.", []v022Expected{{3, 5, 10, "Подозрительные цифры: 1 → l/i/і."}}},
		{"one internal digit", "Say he1o.", []v022Expected{{3, 5, 9, "Подозрительные цифры: 1 → l/i/і."}}},
		{"minimum letters", "a1b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 1 → l/i/і."}}},
		{"one letter", "a1", nil},
		{"no letters", "101", nil},
		{"initial digits", "1hello", nil},
		{"final digits", "hello1", nil},
		{"both edges", "1hello2", nil},
		{"internal with edge digits", "1he1o2", []v022Expected{{3, 1, 7, "Подозрительные цифры: 1 → l/i/і."}}},
		{"zero", "a0b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 0 → o/о."}}},
		{"two", "a2b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 2 → z."}}},
		{"three", "a3b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 3 → з."}}},
		{"five", "a5b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 5 → s."}}},
		{"six", "a6b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 6 → b/б."}}},
		{"eight", "a8b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 8 → B/В."}}},
		{"nine", "a9b", []v022Expected{{3, 1, 4, "Подозрительные цифры: 9 → g."}}},
		{"unsupported four", "a4b", nil},
		{"unsupported seven", "a7b", nil},
		{"unsupported Unicode digit", "a١b", nil},
		{"whole digit run must qualify", "a14b", nil},
		{"another run qualifies", "a1b4c", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"later run qualifies", "a47b0c", []v022Expected{{3, 1, 7, "Подозрительные цифры: 0 → o/о."}}},
		{"one finding unique ordered digits", "a1b0c1d", []v022Expected{{3, 1, 8, "Подозрительные цифры: 1 → l/i/і; 0 → o/о."}}},
		{"all correspondences in one run", "a01235689b", []v022Expected{{3, 1, 11, "Подозрительные цифры: 0 → o/о; 1 → l/i/і; 2 → z; 3 → з; 5 → s; 6 → b/б; 8 → B/В; 9 → g."}}},
		{"mixed scripts", "he1ло", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"Cyrillic with Latin correspondence", "сл2во", []v022Expected{{3, 1, 6, "Подозрительные цифры: 2 → z."}}},
		{"uncased letters", "字1字", []v022Expected{{3, 1, 4, "Подозрительные цифры: 1 → l/i/і."}}},
		{"uppercase formula", "H2O", nil},
		{"uppercase designation", "B2B", nil},
		{"uppercase word", "HE11O", nil},
		{"Cyrillic uppercase", "СЛ0ВО", nil},
		{"mixed case", "He11o", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"numeric exclusion requires two digits", "o1o", []v022Expected{{3, 1, 4, "Подозрительные цифры: 1 → l/i/і."}}},
		{"equal letter and digit count", "o11o", []v022Expected{{3, 1, 5, "Подозрительные цифры: 1 → l/i/і."}}},
		{"numeric candidate", "o111o", nil},
		{"letter outside numeric set", "a111o", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"exact numeric set lowercase b absent", "b111o", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"exact numeric set uppercase B present", "B111o", nil},
		{"exact numeric set uppercase G absent", "G111o", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"exact numeric set lowercase g present", "g111o", nil},
		{"attached diacritic", "he\u03011o", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"final diacritic included", "he1o\u0301", []v022Expected{{3, 1, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"leading unattached mark", "\u0301he1o", []v022Expected{{3, 2, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"Unicode columns", "Я😀 he1o", []v022Expected{{3, 4, 8, "Подозрительные цифры: 1 → l/i/і."}}},
		{"several tokens", "he1o w0rd", []v022Expected{{3, 1, 5, "Подозрительные цифры: 1 → l/i/і."}, {3, 6, 10, "Подозрительные цифры: 0 → o/о."}}},
		{"punctuation separates tokens", "he1o,w0rd", []v022Expected{{3, 1, 5, "Подозрительные цифры: 1 → l/i/і."}, {3, 6, 10, "Подозрительные цифры: 0 → o/о."}}},
		{"underscore separates tokens", "he1_o", nil},
		{"hyphen separates tokens", "he1-o", nil},
		{"slash text included", "/he1o/", []v022Expected{{3, 2, 6, "Подозрительные цифры: 1 → l/i/і."}}},
		{"address included", "https://he1o.test", []v022Expected{{3, 9, 13, "Подозрительные цифры: 1 → l/i/і."}}},
		{"raw escapes not decoded", `he1\{o\}`, nil},
		{"spaces not trimmed", "  he1o  ", []v022Expected{{3, 3, 7, "Подозрительные цифры: 1 → l/i/і."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.raw
			if tc.name == "raw escapes not decoded" {
				value = "he1{o}"
			}
			v022Run(t, v022Document(t, v022Text(t, tc.raw, value, 2, 1)), "current", tc.want, 1)
		})
	}
}

func TestV022WholeLinesAndContexts(t *testing.T) {
	t.Run("single-line comment", func(t *testing.T) {
		line := v022Line(t, "@editor he1o", "tag", 2, 1,
			v022Element(t, "tag", "@editor", "editor", 1), v022Element(t, "content", "he1o", "he1o", 9))
		v022Run(t, v022Document(t, line), "current", []v022Expected{{3, 9, 13, "Подозрительные цифры: 1 → l/i/і."}}, 1)
	})
	t.Run("media path", func(t *testing.T) {
		line := v022Line(t, "@media audio he1o.wav", "tag", 2, 1,
			v022Element(t, "tag", "@media", "media", 1), v022Element(t, "media-type", "audio", "audio", 8), v022Element(t, "source", "he1o.wav", "he1o.wav", 14))
		v022Run(t, v022Document(t, line), "current", []v022Expected{{3, 14, 18, "Подозрительные цифры: 1 → l/i/і."}}, 1)
	})
	for _, tc := range []struct {
		name, tag, raw string
		elements       []model.Element
		want           []v022Expected
	}{
		{"HTML in literal comment", "editor", "<b>he1o</b>", []model.Element{v022Element(t, "content", "<b>he1o</b>", "<b>he1o</b>", 1)}, []v022Expected{{4, 4, 8, "Подозрительные цифры: 1 → l/i/і."}}},
		{"HTML attribute included", "editor", `<b title="he1o">ok</b>`, []model.Element{v022Element(t, "content", `<b title="he1o">ok</b>`, `<b title="he1o">ok</b>`, 1)}, []v022Expected{{4, 11, 15, "Подозрительные цифры: 1 → l/i/і."}}},
		{"HTML does not join tokens", "text", "he1<b>o</b>", []model.Element{v022Element(t, "content", "he1<b>o</b>", "he1<b>o</b>", 1)}, nil},
		{"entity not decoded", "text", "he&#49;o", []model.Element{v022Element(t, "content", "he&#49;o", "he&#49;o", 1)}, nil},
		{"table cells", "table", "he1o | w0rd", []model.Element{v022Element(t, "content", "he1o", "he1o", 1), v022Element(t, "content", "w0rd", "w0rd", 8)}, []v022Expected{{4, 1, 5, "Подозрительные цифры: 1 → l/i/і."}, {4, 8, 12, "Подозрительные цифры: 0 → o/о."}}},
		{"placeholder answer", "multifill", "_____{he1o}", []model.Element{v022Element(t, "placeholder", "_____{he1o}", "he1o", 1)}, []v022Expected{{4, 7, 11, "Подозрительные цифры: 1 → l/i/і."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := v022Line(t, tc.raw, "content", 3, 2, tc.elements...)
			v022Run(t, v022Document(t,
				v022Line(t, "@"+tc.tag+" {", "block-start", 2, 1, v022Element(t, "tag", "@"+tc.tag, tc.tag, 1), v022Element(t, "block-open", "{", "{", len(tc.tag)+3)),
				body, v022Line(t, "}", "block-end", 3, 2, v022Element(t, "block-close", "}", "}", 1))), "current", tc.want, 1)
		})
	}
	t.Run("title", func(t *testing.T) {
		result := v022Document(t)
		title := "he1o"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v022Line(t, "# he1o", "heading", 0, 0, v022Element(t, "heading-level", "#", "1", 1), v022Element(t, "title", title, title, 3))
		v022NumberLines(t, result)
		v022Run(t, result, "current", []v022Expected{{2, 3, 7, "Подозрительные цифры: 1 → l/i/і."}}, 1)
	})
	for _, tc := range []struct {
		tag, kind  string
		start, end int
		wantV037   int
	}{
		{"section", "name", 10, 14, 1}, {"document-id", "identifier", 14, 18, 0}, {"resource-dir", "resource-path", 15, 19, 1},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			result := v022Document(t)
			value := "he1o"
			switch tc.tag {
			case "section":
				result.Document.Metadata.Section = &value
			case "document-id":
				result.Document.Metadata.DocumentID = &value
			case "resource-dir":
				result.Document.Metadata.ResourceDirs = []string{value}
			}
			line := v022Line(t, "@"+tc.tag+" "+value, "tag", 0, 0, v022Element(t, "tag", "@"+tc.tag, tc.tag, 1), v022Element(t, model.ElementType(tc.kind), value, value, tc.start))
			result.Lines = append(result.Lines[:1], line, result.Lines[1])
			v022NumberLines(t, result)
			v022Run(t, result, "current", []v022Expected{{2, tc.start, tc.end, "Подозрительные цифры: 1 → l/i/і."}}, tc.wantV037)
		})
	}
	t.Run("unparsed with parser history", func(t *testing.T) {
		element := v022Element(t, "unparsed", "@unknown he1o", "", 1)
		element.Value, element.ErrorIDs = nil, []string{"parser-error"}
		result := v022Document(t, v022Line(t, "@unknown he1o", "invalid", 2, 1, element))
		result.Document.HasErrors, result.Lines[2].HasErrors = true, true
		result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестное имя тега.", Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 14}}, RelatedLocations: []model.Location{}}}
		v022Run(t, result, "current", []v022Expected{{3, 10, 14, "Подозрительные цифры: 1 → l/i/і."}}, 1)
	})
}

func TestV022BoundariesAndHistory(t *testing.T) {
	v022Run(t, v022UnavailableDocument(t, "1.2"), "current", nil, 0)
	v022Run(t, v022Document(t, v022Line(t, "", "blank", 2, 1)), "current", nil, 1)
	v022Run(t, v022Document(t, v022Text(t, "he1", "he1", 2, 1), v022Text(t, "lo", "lo", 2, 1)), "current", nil, 1)
	for _, ending := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF, model.LineEndingNone} {
		t.Run("ending "+string(ending), func(t *testing.T) {
			result := v022Document(t, v022Text(t, "Say he11o.", "Say he11o.", 2, 1))
			result.Lines[2].LineEnding = ending
			v022Run(t, result, "current", []v022Expected{{3, 5, 10, "Подозрительные цифры: 1 → l/i/і."}}, 1)
		})
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("version "+version, func(t *testing.T) {
			result := v022UnavailableDocument(t, version)
			before := v022Snapshot(t, result)
			checks, err := validator.Prepare(result, v022Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v022Snapshot(t, result) != before {
				t.Fatal("Prepare changed unsupported model")
			}
		})
	}
	result := v022Document(t, v022Text(t, "he1o  \tw0rd !", "he1o  \tw0rd !", 2, 1))
	want := []v022Expected{{3, 1, 5, "Подозрительные цифры: 1 → l/i/і."}, {3, 8, 12, "Подозрительные цифры: 0 → o/о."}}
	v022Run(t, result, "first", want, 1)
	for _, code := range []diagnostics.Code{"V001", "V002", "V017"} {
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
	v022Run(t, result, "second", want, 1)
	v022Run(t, v022Document(t, v022Text(t, "hello", "hello", 2, 1)), "first", nil, 1)
}

func TestV022NestedBlocksAndSeparators(t *testing.T) {
	result := v022Document(t,
		v022Text(t, "he1o", "he1o", 2, 1),
		v022Line(t, "@instr {", "block-start", 2, 1,
			v022Element(t, "tag", "@instr", "instr", 1), v022Element(t, "block-open", "{", "{", 8)),
		v022Text(t, "Read the sentence.", "Read the sentence.", 4, 2),
		v022Line(t, "@hint {", "block-start", 4, 2,
			v022Element(t, "tag", "@hint", "hint", 1), v022Element(t, "block-open", "{", "{", 7)),
		v022Text(t, "w0rd", "w0rd", 6, 3),
		v022Line(t, "}", "block-end", 6, 3, v022Element(t, "block-close", "}", "}", 1)),
		v022Line(t, "}", "block-end", 4, 2, v022Element(t, "block-close", "}", "}", 1)),
		v022Line(t, "@example {", "block-start", 2, 1,
			v022Element(t, "tag", "@example", "example", 1), v022Element(t, "block-open", "{", "{", 10)),
		v022Text(t, "he1", "he1", 10, 2),
		v022Line(t, "---", "separator", 10, 2),
		v022Text(t, "lo", "lo", 10, 2),
		v022Line(t, "}", "block-end", 10, 2, v022Element(t, "block-close", "}", "}", 1)),
		v022Text(t, "a2b", "a2b", 2, 1),
	)
	v022Run(t, result, "current", []v022Expected{
		{3, 1, 5, "Подозрительные цифры: 1 → l/i/і."},
		{7, 1, 5, "Подозрительные цифры: 0 → o/о."},
		{15, 1, 4, "Подозрительные цифры: 2 → z."},
	}, 1)
}

// v022Registry создаёт реестр исключительно через штатный каталог регистраций.
func v022Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v022UnavailableDocument представляет отсутствие доступных строк
// при известной версии DSL, а не пустой исходник с выдуманным тегом версии.
func v022UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v022Document дополняет фрагменты технической строкой и заголовком по DSL 1.30.
// Содержимое фрагментов сохраняется; обычный текст образует шаг по DSL 2.1.15.
// Родители, вложенность, элементы и семантические значения задаются тестом.
func v022Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v022UnavailableDocument(t, "1.2")
	title, encoding := "Unit 1", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v022Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v022Element(t, "tag", "@dsl-version", "dsl-version", 1), v022Element(t, "version", "1.2", "1.2", 14)),
		v022Line(t, "# Unit 1", "heading", 0, 0,
			v022Element(t, "heading-level", "#", "1", 1), v022Element(t, "title", "Unit 1", "Unit 1", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v022NumberLines(t, result)
	return result
}

// v022NumberLines назначает физические номера уже заданным строкам модели.
func v022NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v022Line создаёт строку с заданной структурой, не распознавая Raw.
func v022Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v022Element сохраняет явные Raw и Value; длина нужна только для входа.
// Ожидаемые диапазоны диагностики заданы отдельно и не вычисляются по Raw.
func v022Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{},
	}
}

// v022Text создаёт готовую строку содержимого с явно заданным Value.
func v022Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v022Line(t, raw, "content", parent, depth, v022Element(t, "content", raw, value, 1))
}

// v022Run проверяет V022 текущего запуска через штатный конвейер.
// want содержит явные диапазоны с исключающим концом и конкретизации.
// В этих ситуациях новые срабатывания других проверок не меняют флаги ошибок.
// wantV037 задаёт точное число сопутствующих ошибок отсутствующего ID;
// флаг документа проверяется отдельно, строки и прежняя история сохраняются.
func v022Run(t *testing.T, result *model.Result, processingID string, want []v022Expected, wantV037 int) {
	t.Helper()
	before := v022Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v022Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v022Snapshot(t, result) != before {
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
		if entry.DiagnosticCode == "V022" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if gotV037 != wantV037 {
		t.Fatalf("current V037 count = %d, want %d", gotV037, wantV037)
	}
	if len(got) != len(want) {
		t.Fatalf("current V022 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected.line, Column: expected.start},
			End:   model.Position{Line: expected.line, Column: expected.end},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V022" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "Внутри буквенного слова находятся цифры, похожие на буквы.\nконкретизация: "+expected.detail || entry.DiagnosticScope != "line" ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
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
	if v022Snapshot(t, &previous) != before {
		t.Fatal("source, structure, error flags, element links or previous history changed")
	}
}

// v022Snapshot фиксирует всю модель для сравнения исходных данных и истории.
func v022Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

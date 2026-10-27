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

const v205TestMessage = "Регистр заголовка отличается от оформления с прописной первой буквой и строчными остальными."

func v205TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value,
		Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v205TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v205TestTag(t *testing.T, name, value string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	return v205TestLine(t, raw+" "+value, "tag", parent,
		v205TestElement(t, "tag", raw, strings.ToLower(name), 1),
		v205TestElement(t, kind, value, value, len(raw)+2))
}

// v205TestOpen задаёт объявление явно; пустой title означает отсутствие названия.
func v205TestOpen(t *testing.T, name, title string, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v205TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if title != "" {
		elements = append(elements, v205TestElement(t, "title", title, title, len(raw)+2))
		raw += " " + title
	}
	elements = append(elements, v205TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(raw)+2))
	return v205TestLine(t, raw+" {", "block-start", parent, elements...)
}

func v205TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v205TestLine(t, raw, "content", parent, v205TestElement(t, "content", raw, value, 1))
}

func v205TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v205TestLine(t, "}", "block-end", parent, v205TestElement(t, "block-close", "}", "}", 1))
}

// v205TestDocument дополняет фрагмент обязательными версией и заголовком первого
// уровня. Типы, значения и родители задаются входами; DSL не разбирается.
func v205TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v205TestTag(t, "dsl-version", version, "version", 0),
		v205TestLine(t, "# Lesson", "heading", 0,
			v205TestElement(t, "heading-level", "#", "1", 1), v205TestElement(t, "title", title, title, 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Diagnostics: []model.Diagnostic{},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}}
	for index := range result.Lines {
		line := &result.Lines[index]
		line.Number = index + 1
		if line.ParentLine != nil {
			if *line.ParentLine < 1 || *line.ParentLine >= line.Number {
				t.Fatal("fixture parent must precede its child")
			}
			line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
		}
		for index := range line.Elements {
			element := &line.Elements[index]
			if element.ElementType != "unparsed" {
				continue
			}
			element.Value = nil
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Фрагмент не разобран.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	result.Lines[len(lines)-1].LineEnding = ""
	return result
}

func v205TestBlockInTask(t *testing.T, name, title string) *model.Result {
	t.Helper()
	body := []model.Line{
		v205TestLine(t, "### Practice", "heading", 0,
			v205TestElement(t, "heading-level", "###", "3", 1), v205TestElement(t, "title", "Practice", "Practice", 5)),
		v205TestTag(t, "task", "a", "identifier", 3), v205TestOpen(t, name, title, 4),
	}
	var content []string
	switch name {
	case "table":
		content = []string{"A | B", "C | D"}
	case "wordlist":
		content = []string{"apple", "pear"}
	case "script":
		content = []string{"A: Hello.", "B: Hi."}
	case "alt":
		content = []string{"A sunny beach."}
	case "key":
		content = []string{"1. Answer."}
	case "example":
		content = []string{"It was sunny."}
	default:
		content = []string{"Read the passage."}
	}
	for _, text := range content {
		body = append(body, v205TestText(t, text, text, 5))
	}
	body = append(body, v205TestClose(t, 5))
	return v205TestDocument(t, body...)
}

func v205TestJSON(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v205TestRun исполняет полный каталог и сравнивает только новые V205.
// Ожидания содержат [строка, начальная колонка, исключающая конечная колонка].
func v205TestRun(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v205TestJSON(t, result)
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v205TestJSON(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldCount := len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	newIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldCount:] {
		newIDs[entry.ID] = true
		if entry.DiagnosticCode == "V205" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V205 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v205TestMessage ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "element" ||
			entry.Fatal || entry.ID == "" || entry.Source != source || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V205[%d] = %+v, want %v with exact metadata", index, entry, expected)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if !slices.Contains(element.ErrorIDs, entry.ID) {
					continue
				}
				links++
				if line.Number != expected[0] || element.ElementType != "title" ||
					element.Start != expected[1] || element.End != expected[2] {
					t.Fatal("V205 linked to an unrelated element")
				}
			}
		}
		if links != 1 {
			t.Fatalf("V205 has %d links, want one title link", links)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v205TestJSON(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for index := range restored.Lines {
		line := &restored.Lines[index]
		line.HasErrors = original.Lines[index].HasErrors
		for index := range line.Elements {
			element := &line.Elements[index]
			prior := make([]string, 0, len(element.ErrorIDs))
			for _, id := range element.ErrorIDs {
				if !newIDs[id] {
					prior = append(prior, id)
				}
			}
			element.ErrorIDs = prior
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v205TestJSON(t, &restored) != before {
		t.Fatal("source data, metadata, prior links or history changed")
	}
}

func TestV205RegistrationAndVersions(t *testing.T) {
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
		if definition.Code != "V205" {
			continue
		}
		count++
		if definition.Message != v205TestMessage || definition.Severity != "recommendation" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V205 registered %d times, want one", count)
	}
	// Каталог DSL поддерживает только 1.2; другие версии отклоняются до Run.
	// Общее число диагностик не является ожиданием теста.
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			if checks, err := registry.ForVersion(dslversions.Version(version)); err == nil || checks != nil {
				t.Fatal("unsupported version accepted")
			}
			result := v205TestBlockInTask(t, "text", "Reading Practice")
			result.Document.DSLVersion = &version
			before := v205TestJSON(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatal("Prepare accepted unsupported version")
			}
			if v205TestJSON(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV205ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, textTitle, wordTitle, fragment string
		want                                 [][3]int
	}{
		{"reference", "Reading practice", "New words",
			"@text Reading practice {\nRead the passage.\n}\n\n@wordlist New words {\napple\npear\n}", nil},
		{"example", "Reading Practice", "NEW WORDS",
			"@text Reading Practice {\nRead the passage.\n}\n\n@wordlist NEW WORDS {\napple\npear\n}",
			[][3]int{{3, 7, 23}, {7, 11, 20}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v205TestDocument(t, v205TestOpen(t, "text", tc.textTitle, 2),
				v205TestText(t, "Read the passage.", "Read the passage.", 3), v205TestClose(t, 3),
				v205TestLine(t, "", "blank", 2), v205TestOpen(t, "wordlist", tc.wordTitle, 2),
				v205TestText(t, "apple", "apple", 7), v205TestText(t, "pear", "pear", 7), v205TestClose(t, 7))
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v205TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV205SelectedTags(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end int
	}{
		{"table", 8, 24}, {"alt", 6, 22}, {"script", 9, 25}, {"text", 7, 23},
		{"key", 6, 22}, {"example", 10, 26}, {"wordlist", 11, 27},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v205TestRun(t, v205TestBlockInTask(t, tc.name, "Reading Practice"), "wrong", [][3]int{{5, tc.start, tc.end}})
			v205TestRun(t, v205TestBlockInTask(t, tc.name, "Reading practice"), "correct", nil)
			v205TestRun(t, v205TestBlockInTask(t, tc.name, ""), "untitled", nil)
		})
	}
}

func TestV205OriginalRanges(t *testing.T) {
	for _, tc := range []struct {
		name, tag, indent, gap, raw, value, suffix string
		start, end                                 int
	}{
		{"indentation", "text", "\t", " ", "Reading Practice", "Reading Practice", " {", 8, 24},
		{"spaces around title", "text", "", "   ", "Reading Practice", "Reading Practice", "   {  \t", 9, 25},
		{"mixed case tag", "WoRdLiSt", "", " ", "NEW WORDS", "NEW WORDS", " {", 11, 20},
		{"escaped braces", "text", "", " ", `\{Reading Practice\}`, "{Reading Practice}", " {", 7, 27},
		{"escaped backslash", "text", "", " ", `\\Reading Practice`, `\Reading Practice`, " {", 7, 25},
		// \@ в середине строки не снимается по DSL 1.21–1.22.
		{"ordinary backslash", "text", "", " ", `\@Reading Practice`, `\@Reading Practice`, " {", 7, 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.indent + "@" + tc.tag + tc.gap + tc.raw + tc.suffix
			braceStart := utf8.RuneCountInString(tc.indent+"@"+tc.tag+tc.gap+tc.raw) + strings.Index(tc.suffix, "{") + 1
			line := v205TestLine(t, raw, "block-start", 2,
				v205TestElement(t, "tag", "@"+tc.tag, strings.ToLower(tc.tag), utf8.RuneCountInString(tc.indent)+1),
				v205TestElement(t, "title", tc.raw, tc.value, tc.start),
				v205TestElement(t, "block-open", "{", "{", braceStart))
			result := v205TestDocument(t, line, v205TestText(t, "apple", "apple", 3), v205TestClose(t, 3))
			v205TestRun(t, result, "current", [][3]int{{3, tc.start, tc.end}})
		})
	}
}

func TestV205ExcludedDeclarations(t *testing.T) {
	for _, name := range []string{"note", "instr", "hint"} {
		t.Run(name, func(t *testing.T) {
			v205TestRun(t, v205TestBlockInTask(t, name, "Reading Practice"), "current", nil)
		})
	}
	t.Run("inline forms", func(t *testing.T) {
		result := v205TestDocument(t,
			v205TestTag(t, "alt", "Reading Practice", "content", 2),
			v205TestTag(t, "example", "Reading Practice", "content", 2),
			v205TestTag(t, "wordlist", "NEW WORDS; APPLE.", "content", 2))
		v205TestRun(t, result, "current", nil)
	})
	t.Run("header and heading titles are excluded", func(t *testing.T) {
		result := v205TestDocument(t,
			v205TestLine(t, "### Reading Practice", "heading", 0,
				v205TestElement(t, "heading-level", "###", "3", 1),
				v205TestElement(t, "title", "Reading Practice", "Reading Practice", 5)),
			v205TestTag(t, "header", "NEW WORDS", "title", 3), v205TestText(t, "Read.", "Read.", 3))
		v205TestRun(t, result, "current", nil)
	})
	t.Run("escaped declaration remains text", func(t *testing.T) {
		result := v205TestDocument(t, v205TestText(t, `\@text Reading Practice \{`, "@text Reading Practice {", 2))
		v205TestRun(t, result, "current", nil)
	})
	t.Run("editor content remains literal", func(t *testing.T) {
		result := v205TestDocument(t, v205TestOpen(t, "editor", "", 2),
			v205TestText(t, "@text Reading Practice", "@text Reading Practice", 3),
			v205TestText(t, "@wordlist NEW WORDS", "@wordlist NEW WORDS", 3), v205TestClose(t, 3))
		v205TestRun(t, result, "current", nil)
	})
	t.Run("invalid line with recognized title", func(t *testing.T) {
		// По DSL 1.18 после открывающей скобки не должно быть текста.
		line := v205TestOpen(t, "text", "Reading Practice", 2)
		line.LineType, line.Raw = "invalid", line.Raw+" extra"
		line.Elements = append(line.Elements, v205TestElement(t, "unparsed", "extra", "", 26))
		v205TestRun(t, v205TestDocument(t, line), "current", nil)
	})
	t.Run("tag was not extracted", func(t *testing.T) {
		line := v205TestOpen(t, "text", "Reading Practice", 2)
		line.Elements[0].ElementType = "unparsed"
		result := v205TestDocument(t, line, v205TestText(t, "Read.", "Read.", 3), v205TestClose(t, 3))
		v205TestRun(t, result, "current", nil)
	})
	t.Run("empty title string supported by contract", func(t *testing.T) {
		line := v205TestLine(t, "@text  {", "block-start", 2,
			v205TestElement(t, "tag", "@text", "text", 1),
			v205TestElement(t, "title", "", "", 7),
			v205TestElement(t, "block-open", "{", "{", 8))
		result := v205TestDocument(t, line, v205TestText(t, "Read.", "Read.", 3), v205TestClose(t, 3))
		v205TestRun(t, result, "current", nil)
	})
}

func TestV205ContextAndIndependentTitles(t *testing.T) {
	t.Run("nested text in complete fragment", func(t *testing.T) {
		fragment := v205TestOpen(t, "fragment", "f", 2)
		fragment.Elements[1].ElementType = "identifier"
		result := v205TestDocument(t, fragment, v205TestOpen(t, "text", "Reading Practice", 3),
			v205TestText(t, "Read.", "Read.", 4), v205TestClose(t, 4), v205TestClose(t, 3),
			v205TestTag(t, "include", "f", "identifier", 2))
		v205TestRun(t, result, "current", [][3]int{{4, 7, 23}})
	})
	t.Run("key in task step", func(t *testing.T) {
		result := v205TestDocument(t,
			v205TestLine(t, "### Practice", "heading", 0,
				v205TestElement(t, "heading-level", "###", "3", 1), v205TestElement(t, "title", "Practice", "Practice", 5)),
			v205TestTag(t, "task", "a", "identifier", 3), v205TestTag(t, "step", "First", "title", 4),
			v205TestOpen(t, "key", "Reading Practice", 5), v205TestText(t, "1. Answer.", "1. Answer.", 6), v205TestClose(t, 6),
			v205TestTag(t, "step", "Second", "title", 4), v205TestTag(t, "instr", "Read.", "content", 9))
		v205TestRun(t, result, "current", [][3]int{{6, 6, 22}})
	})
	t.Run("repeated titles", func(t *testing.T) {
		result := v205TestDocument(t,
			v205TestOpen(t, "text", "Reading Practice", 2), v205TestText(t, "Read.", "Read.", 3), v205TestClose(t, 3),
			v205TestOpen(t, "text", "Reading Practice", 2), v205TestText(t, "Write.", "Write.", 6), v205TestClose(t, 6))
		v205TestRun(t, result, "current", [][3]int{{3, 7, 23}, {6, 7, 23}})
	})
	t.Run("first and last candidate with correct middle", func(t *testing.T) {
		result := v205TestDocument(t,
			v205TestOpen(t, "text", "Reading Practice", 2), v205TestText(t, "Read.", "Read.", 3), v205TestClose(t, 3),
			v205TestOpen(t, "text", "Reading practice", 2), v205TestText(t, "Read.", "Read.", 6), v205TestClose(t, 6),
			v205TestOpen(t, "text", "reading practice", 2), v205TestText(t, "Write.", "Write.", 9), v205TestClose(t, 9))
		v205TestRun(t, result, "current", [][3]int{{3, 7, 23}, {9, 7, 23}})
	})
	t.Run("each title starts independently", func(t *testing.T) {
		result := v205TestDocument(t,
			v205TestOpen(t, "text", "А", 2), v205TestText(t, "Read.", "Read.", 3), v205TestClose(t, 3),
			v205TestOpen(t, "text", "Б", 2), v205TestText(t, "Write.", "Write.", 6), v205TestClose(t, 6))
		v205TestRun(t, result, "current", nil)
	})
	for _, tc := range []struct {
		title string
		want  [][3]int
	}{
		{"Reading Practice", [][3]int{{3, 7, 23}}},
		{"Reading practice", nil},
	} {
		t.Run("content ignored/"+tc.title, func(t *testing.T) {
			result := v205TestDocument(t, v205TestOpen(t, "text", tc.title, 2),
				v205TestText(t, "READ THE PASSAGE.", "READ THE PASSAGE.", 3),
				v205TestLine(t, "", "blank", 3), v205TestText(t, "read the passage.", "read the passage.", 3),
				v205TestClose(t, 3))
			v205TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("example separator is not part of title", func(t *testing.T) {
		result := v205TestDocument(t, v205TestOpen(t, "example", "Reading Practice", 2),
			v205TestText(t, "It was suny.", "It was suny.", 3), v205TestLine(t, "---", "separator", 3),
			v205TestText(t, "It was sunny.", "It was sunny.", 3), v205TestClose(t, 3))
		v205TestRun(t, result, "current", [][3]int{{3, 10, 26}})
	})
}

func TestV205EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v205TestDocument(t, v205TestOpen(t, "text", "Reading Practice", 2),
					v205TestText(t, "Read the passage.", "Read the passage.", 3), v205TestClose(t, 3),
					v205TestLine(t, "", "blank", 2), v205TestOpen(t, "wordlist", "NEW WORDS", 2),
					v205TestText(t, "apple", "apple", 7), v205TestText(t, "pear", "pear", 7), v205TestClose(t, 7))
				for index := range result.Lines {
					result.Lines[index].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := [][3]int{{3, 7, 23}, {7, 11, 20}}
				v205TestRun(t, result, "first", want)
				v205TestRun(t, result, "second", want)
				v205TestRun(t, v205TestBlockInTask(t, "wordlist", "New words"), "other", nil)
			})
		}
	}
	t.Run("no blocks", func(t *testing.T) {
		v205TestRun(t, v205TestDocument(t), "current", nil)
	})
	t.Run("empty input", func(t *testing.T) {
		result := v205TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v205TestRun(t, result, "current", nil)
	})
}

func TestV205SentenceCase(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		end         int
	}{
		{"correct Cyrillic", "Новые слова", 0},
		{"lowercase first", "новые слова", 18},
		{"several violations", "НОВЫЕ СЛОВА", 18},
		{"one uppercase", "Я", 0},
		{"one lowercase", "я", 8},
		{"no cased letters", "123 — 世界!", 0},
		{"prefix before lowercase", "123 — тема", 17},
		{"prefix before uppercase", "123 — Тема", 0},
		{"colon does not restart", "Тема: Далее", 18},
		{"lowercase after colon", "Тема: далее", 0},
		{"no uppercase section scheme", "РАЗДЕЛ: Тема", 19},
		{"period does not restart", "Тема. Далее", 18},
		{"no proper name exception", "Про Анну", 15},
		{"no abbreviation exception", "Тема NASA", 16},
		{"titlecase first", "ǅuro", 11},
		{"titlecase later", "Aǅ", 9},
		{"uppercase digraph", "Ǆuro", 0},
		{"additional lowercase property", "Aʰ", 0},
		{"additional lowercase first", "ʰa", 9},
		{"supplementary letters", "𐐀𐐨", 0},
		{"supplementary uppercase later", "𐐀𐐀", 9},
		{"supplementary lowercase first", "𐐨𐐨", 9},
		{"uncased letters before and between", "世界A界b", 0},
		{"uncased letters do not restart", "A界B", 10},
		{"numeric case properties are not letters", "ⅰⓐAⅧb", 0},
		{"combining marks do not restart", "E\u0301cole", 0},
		{"combining mark before lowercase", "\u0301école", 13},
		{"nonbreaking space does not restart", "A\u00a0B", 10},
		{"internal tab does not restart", "A\tB", 10},
		{"ordinary backslash before lowercase", "\\q", 9},
		{"emoji does not restart", "A🙂B", 10},
		{"lowercase without uppercase mapping", "ß", 8},
		{"lowercase without mapping after first", "Aß", 0},
		{"accented letters", "École française", 0},
		{"accented uppercase later", "École Française", 22},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v205TestDocument(t, v205TestOpen(t, "text", tc.title, 2),
				v205TestText(t, "Read.", "Read.", 3), v205TestClose(t, 3))
			var want [][3]int
			if tc.end != 0 {
				want = [][3]int{{3, 7, tc.end}}
			}
			v205TestRun(t, result, "current", want)
		})
	}
}

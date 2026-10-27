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

const v203TestMessage = "Регистр заголовка отличается от схемы «Тема» или «РАЗДЕЛ: Тема»."

func v203TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value,
		Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v203TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v203TestHeading(t *testing.T, marker, raw, value string) model.Line {
	t.Helper()
	return v203TestLine(t, marker+" "+raw, "heading", 0,
		v203TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v203TestElement(t, "title", raw, value, len(marker)+2))
}

func v203TestTag(t *testing.T, name, value string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	return v203TestLine(t, raw+" "+value, "tag", parent,
		v203TestElement(t, "tag", raw, strings.ToLower(name), 1),
		v203TestElement(t, kind, value, value, len(raw)+2))
}

func v203TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v203TestLine(t, raw, "content", parent, v203TestElement(t, "content", raw, value, 1))
}

func v203TestOpen(t *testing.T, name string, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	return v203TestLine(t, raw+" {", "block-start", parent,
		v203TestElement(t, "tag", raw, name, 1), v203TestElement(t, "block-open", "{", "{", len(raw)+2))
}

func v203TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v203TestLine(t, "}", "block-end", parent, v203TestElement(t, "block-close", "}", "}", 1))
}

// v203TestDocument добавляет обязательный контекст DSL 1.3 и 2.1.3 перед фрагментом.
// Структура передаётся явно, DSL не разбирается. unparsed получает исходную
// ошибку парсера для согласованности модели; это не ожидаемая диагностика V203.
func v203TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v203TestTag(t, "dsl-version", version, "version", 0),
		v203TestHeading(t, "#", title, title)}, body...)
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

func v203TestJSON(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v203TestRun исполняет полный каталог и сверяет только новые сообщения V203.
// Ожидания содержат [строка, начальная колонка, исключающая конечная колонка].
func v203TestRun(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v203TestJSON(t, result)
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v203TestJSON(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldCount := len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V203" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V203 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v203TestMessage ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "element" ||
			entry.Fatal || entry.ID == "" || entry.Source != source || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V203[%d] = %+v, want range %v and exact metadata", index, entry, expected)
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
					t.Fatal("V203 linked to an unrelated element")
				}
			}
		}
		if links != 1 {
			t.Fatalf("V203 has %d element links, want one title link", links)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v203TestJSON(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for index := range restored.Lines {
		line, old := &restored.Lines[index], original.Lines[index]
		line.HasErrors = old.HasErrors
		if len(line.Elements) != len(old.Elements) {
			t.Fatal("element count changed")
		}
		for index := range line.Elements {
			prior, current := old.Elements[index].ErrorIDs, line.Elements[index].ErrorIDs
			if len(current) < len(prior) || !slices.Equal(prior, current[:len(prior)]) {
				t.Fatal("prior element links changed")
			}
			line.Elements[index].ErrorIDs = prior
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v203TestJSON(t, &restored) != before {
		t.Fatal("source data, metadata or previous history changed")
	}
}

func TestV203RegistrationAndVersions(t *testing.T) {
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
		if definition.Code != "V203" {
			continue
		}
		count++
		if definition.Message != v203TestMessage || definition.Severity != "recommendation" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V203 registered %d times, want one", count)
	}
	// Каталог DSL поддерживает только 1.2. Проверяем отказ до запуска;
	// общее число зарегистрированных диагностик не фиксируется.
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			if checks, err := registry.ForVersion(dslversions.Version(version)); err == nil || checks != nil {
				t.Fatal("unsupported version accepted")
			}
			result := v203TestDocument(t, v203TestTag(t, "header", "WRITING", "title", 2))
			result.Document.DSLVersion = &version
			before := v203TestJSON(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatal("Prepare accepted unsupported version")
			}
			if v203TestJSON(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV203ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, heading, header, fragment string
		want                            [][3]int
	}{
		{"reference", "GRAMMAR: Be", "Writing", "### GRAMMAR: Be\n@header Writing", nil},
		{"example", "Grammar: be", "WRITING", "### Grammar: be\n@header WRITING", [][3]int{{3, 5, 16}, {4, 9, 16}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Версия и # обязательны; Read. добавляет содержимое шага.
			result := v203TestDocument(t, v203TestHeading(t, "###", tc.heading, tc.heading),
				v203TestTag(t, "header", tc.header, "title", 3), v203TestText(t, "Read.", "Read.", 3))
			if result.Lines[2].Raw+"\n"+result.Lines[3].Raw != tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v203TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV203OriginalRanges(t *testing.T) {
	for _, tc := range []struct {
		name, marker, indent, gap, raw, value, trailing string
		start, end                                      int
	}{
		{"heading indentation", "###", "\t", " ", "Grammar: be", "Grammar: be", " \t", 6, 17},
		{"header indentation", "@header", "\t", " ", "WRITING", "WRITING", " \t", 10, 17},
		{"heading separator", "###", "", "   ", "Grammar: be", "Grammar: be", "", 7, 18},
		{"header separator", "@header", "", "   ", "WRITING", "WRITING", "", 11, 18},
		{"mixed case tag spelling", "@HeAdEr", "", " ", "WRITING", "WRITING", "", 9, 16},
		{"heading escaped braces", "###", "", " ", `\{Grammar: be\}`, "{Grammar: be}", "", 5, 20},
		{"header escaped braces", "@header", "", " ", `\{Grammar: be\}`, "{Grammar: be}", "", 9, 24},
		{"escaped backslash", "###", "", " ", `\\Grammar: be`, `\Grammar: be`, "", 5, 18},
		// Двоеточие не экранируется по DSL 1.21–1.22: косая черта
		// сохраняется в Value, а первое U+003A всё равно разделяет части.
		{"backslash before colon", "@header", "", " ", `Grammar\: Be`, `Grammar\: Be`, "", 9, 21},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind, parent, markerType, markerValue := model.LineType("heading"), 0, model.ElementType("heading-level"), "3"
			contentParent := 3
			if tc.marker != "###" {
				kind, parent, markerType, markerValue, contentParent = "tag", 2, "tag", "header", 2
			}
			line := v203TestLine(t, tc.indent+tc.marker+tc.gap+tc.raw+tc.trailing, kind, parent,
				v203TestElement(t, markerType, tc.marker, markerValue, utf8.RuneCountInString(tc.indent)+1),
				v203TestElement(t, "title", tc.raw, tc.value, tc.start))
			result := v203TestDocument(t, line, v203TestText(t, "Read.", "Read.", contentParent))
			v203TestRun(t, result, "current", [][3]int{{3, tc.start, tc.end}})
		})
	}
}

func TestV203Selection(t *testing.T) {
	t.Run("other heading levels", func(t *testing.T) {
		result := v203TestDocument(t, v203TestHeading(t, "##", "WRITING", "WRITING"),
			v203TestText(t, "Read.", "Read.", 3))
		title, subtitle := "LESSON", "WRITING"
		result.Lines[1] = v203TestHeading(t, "#", title, title)
		result.Lines[1].Number = 2
		result.Document.Metadata.Title, result.Document.Metadata.Subtitle = &title, &subtitle
		v203TestRun(t, result, "current", nil)
	})
	t.Run("other title tags", func(t *testing.T) {
		result := v203TestDocument(t, v203TestTag(t, "task", "a", "identifier", 2),
			v203TestTag(t, "step", "WRITING", "title", 3), v203TestTag(t, "instr", "Read.", "content", 4),
			v203TestTag(t, "step", "GRAMMAR", "title", 3), v203TestTag(t, "instr", "Write.", "content", 6))
		v203TestRun(t, result, "current", nil)
	})
	t.Run("escaped declarations", func(t *testing.T) {
		result := v203TestDocument(t,
			v203TestText(t, `\### Grammar: be`, "### Grammar: be", 2),
			v203TestText(t, `\@header WRITING`, "@header WRITING", 2))
		v203TestRun(t, result, "current", nil)
	})
	t.Run("literal editor content", func(t *testing.T) {
		result := v203TestDocument(t, v203TestOpen(t, "editor", 2),
			v203TestText(t, "### Grammar: be", "### Grammar: be", 3),
			v203TestText(t, "@header WRITING", "@header WRITING", 3), v203TestClose(t, 3))
		v203TestRun(t, result, "current", nil)
	})
	t.Run("block with a title is excluded", func(t *testing.T) {
		result := v203TestDocument(t,
			v203TestLine(t, "@text WRITING {", "block-start", 2,
				v203TestElement(t, "tag", "@text", "text", 1),
				v203TestElement(t, "title", "WRITING", "WRITING", 7),
				v203TestElement(t, "block-open", "{", "{", 15)),
			v203TestText(t, "Read.", "Read.", 3), v203TestClose(t, 3))
		v203TestRun(t, result, "current", nil)
	})
	for _, form := range []string{"heading", "header"} {
		t.Run(form+"/invalid line with title", func(t *testing.T) {
			// Неэкранированная { запрещена в текстовом названии по DSL 1.21.
			line := v203TestHeading(t, "###", "WRITING", "WRITING")
			start := 13
			if form == "header" {
				line = v203TestTag(t, "header", "WRITING", "title", 2)
				start = 17
			}
			line.LineType, line.Raw = "invalid", line.Raw+" {"
			line.Elements = append(line.Elements, v203TestElement(t, "unparsed", "{", "", start))
			v203TestRun(t, v203TestDocument(t, line), "current", nil)
		})
		t.Run(form+"/missing recognized marker", func(t *testing.T) {
			line := v203TestHeading(t, "###", "WRITING", "WRITING")
			if form == "header" {
				line = v203TestTag(t, "header", "WRITING", "title", 2)
			}
			// Контракт допускает частичный разбор с исходной ошибкой unparsed.
			line.Elements[0].ElementType = "unparsed"
			v203TestRun(t, v203TestDocument(t, line), "current", nil)
		})
		t.Run(form+"/missing title", func(t *testing.T) {
			line := v203TestHeading(t, "###", "", "")
			if form == "header" {
				line = v203TestTag(t, "header", "", "title", 2)
			}
			line.Elements[1].ElementType = "unparsed"
			v203TestRun(t, v203TestDocument(t, line), "current", nil)
		})
		t.Run(form+"/empty string value", func(t *testing.T) {
			line := v203TestHeading(t, "###", "", "")
			parent := 3
			if form == "header" {
				line = v203TestTag(t, "header", "", "title", 2)
				parent = 2
			}
			// Пустая строка поддерживается контрактом; null сюда не передаётся.
			v203TestRun(t, v203TestDocument(t, line, v203TestText(t, "Read.", "Read.", parent)), "current", nil)
		})
	}
}

func TestV203IndependentTitlesAndContext(t *testing.T) {
	t.Run("repeated headers", func(t *testing.T) {
		result := v203TestDocument(t, v203TestTag(t, "header", "WRITING", "title", 2),
			v203TestTag(t, "header", "WRITING", "title", 2), v203TestText(t, "Read.", "Read.", 2))
		v203TestRun(t, result, "current", [][3]int{{3, 9, 16}, {4, 9, 16}})
	})
	t.Run("first letter and colon state are local", func(t *testing.T) {
		result := v203TestDocument(t,
			v203TestTag(t, "header", "А", "title", 2), v203TestTag(t, "header", "Б", "title", 2),
			v203TestTag(t, "header", "ABC: Тема", "title", 2), v203TestTag(t, "header", "Тема", "title", 2),
			v203TestText(t, "Read.", "Read.", 2))
		v203TestRun(t, result, "current", nil)
	})
	t.Run("first and last heading", func(t *testing.T) {
		result := v203TestDocument(t, v203TestHeading(t, "###", "Grammar: be", "Grammar: be"),
			v203TestTag(t, "header", "Writing", "title", 3), v203TestText(t, "Read.", "Read.", 3),
			v203TestHeading(t, "###", "тема", "тема"), v203TestText(t, "Write.", "Write.", 6))
		v203TestRun(t, result, "current", [][3]int{{3, 5, 16}, {6, 5, 9}})
	})
	t.Run("headers in complete variant branches", func(t *testing.T) {
		// DSL 4.9.22 разрешает header внутри каждой ветви. Проверяем
		// заголовки до контейнера, внутри обеих ветвей и после закрытия.
		result := v203TestDocument(t, v203TestHeading(t, "###", "Practice", "Practice"),
			v203TestTag(t, "header", "WRITING", "title", 3), v203TestOpen(t, "variants", 3),
			v203TestTag(t, "variant", "First", "name", 5), v203TestTag(t, "header", "Grammar: be", "title", 6),
			v203TestText(t, "Read.", "Read.", 6), v203TestTag(t, "variant", "Second", "name", 5),
			v203TestTag(t, "header", "Writing", "title", 9), v203TestText(t, "Write.", "Write.", 9),
			v203TestClose(t, 5), v203TestTag(t, "header", "WRITING", "title", 3))
		v203TestRun(t, result, "current", [][3]int{{4, 9, 16}, {7, 9, 20}, {13, 9, 16}})
	})
	for _, tc := range []struct {
		title string
		want  [][3]int
	}{
		{"Grammar: be", [][3]int{{3, 5, 16}}},
		{"GRAMMAR: Be", nil},
	} {
		t.Run("content ignored/"+tc.title, func(t *testing.T) {
			result := v203TestDocument(t, v203TestHeading(t, "###", tc.title, tc.title),
				v203TestLine(t, "", "blank", 3), v203TestText(t, "WRITING", "WRITING", 3),
				v203TestOpen(t, "note", 3), v203TestText(t, "grammar: be", "grammar: be", 6), v203TestClose(t, 6))
			v203TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV203EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v203TestDocument(t, v203TestHeading(t, "###", "Grammar: be", "Grammar: be"),
					v203TestTag(t, "header", "WRITING", "title", 3), v203TestText(t, "Read.", "Read.", 3))
				for index := range result.Lines {
					result.Lines[index].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := [][3]int{{3, 5, 16}, {4, 9, 16}}
				v203TestRun(t, result, "first", want)
				v203TestRun(t, result, "second", want)
				v203TestRun(t, v203TestDocument(t, v203TestTag(t, "header", "Writing", "title", 2)), "other", nil)
			})
		}
	}
	t.Run("no eligible titles", func(t *testing.T) {
		v203TestRun(t, v203TestDocument(t), "current", nil)
	})
	t.Run("empty input", func(t *testing.T) {
		result := v203TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v203TestRun(t, result, "current", nil)
	})
}

func TestV203CasePatterns(t *testing.T) {
	for _, tc := range []struct {
		name, title        string
		headingEnd, tagEnd int
	}{
		{"sentence case", "Тема", 0, 0},
		{"lowercase first", "тема", 9, 13},
		{"lowercase title with slash", "a/b", 8, 12},
		{"uppercase later letters", "ТЕМА", 9, 13},
		{"both parts correct", "РАЗДЕЛ: Тема", 0, 0},
		{"left wrong", "Раздел: Тема", 17, 21},
		{"right wrong", "РАЗДЕЛ: тема", 17, 21},
		{"both wrong give one finding", "Раздел: тема", 17, 21},
		{"second colon does not restart", "ТЕМА: Подтема: Далее", 25, 29},
		{"lowercase after second colon", "ТЕМА: Подтема: далее", 0, 0},
		{"only ASCII colon splits", "Тема：Далее", 15, 19},
		{"empty left", ": Тема", 0, 0},
		{"empty left wrong right", ": тема", 11, 15},
		{"empty right", "РАЗДЕЛ:", 0, 0},
		{"empty right wrong left", "Раздел:", 12, 16},
		{"numeric parts", "123: 456", 0, 0},
		{"only separator", ":", 0, 0},
		{"several empty parts", "::", 0, 0},
		{"no case", "世界", 0, 0},
		{"numeric left wrong right", "123: тема", 14, 18},
		{"numeric right wrong left", "раздел:123", 15, 19},
		{"no abbreviation exception", "Тема NASA", 14, 18},
		{"no proper name exception", "Про Анну", 13, 17},
		{"titlecase first", "ǅuro", 9, 13},
		{"titlecase in prefix", "ǅ: A", 9, 13},
		{"titlecase later", "Aǅ", 7, 11},
		{"uppercase digraph prefix", "Ǆ: A", 0, 0},
		{"additional lowercase property", "Aʰ", 0, 0},
		{"additional lowercase prefix", "ʰ:A", 8, 12},
		{"supplementary letters", "𐐀𐐨", 0, 0},
		{"supplementary uppercase later", "𐐀𐐀", 7, 11},
		{"supplementary prefix", "𐐀𐐀: 𐐀𐐨", 0, 0},
		{"supplementary wrong prefix", "𐐀𐐨: 𐐀𐐨", 11, 15},
		{"uncased letters do not consume first", "世界A界b", 0, 0},
		{"uncased letters do not restart", "A界B", 8, 12},
		{"numeric case properties are not letters", "ⅰⓐAⅧb", 0, 0},
		{"single uppercase", "Я", 0, 0},
		{"single lowercase", "я", 6, 10},
		{"single letters on both sides", "А: Б", 0, 0},
		{"single wrong right letter", "А: б", 9, 13},
		{"single wrong left letter", "а: Б", 9, 13},
		{"spaces around separator", "РАЗДЕЛ  :  Тема", 0, 0},
		{"punctuation does not restart", "Тема. Далее", 16, 20},
		{"punctuation before first letter", "123 — тема", 15, 19},
		{"hyphen in prefix", "ABC-DEF: Тема", 0, 0},
		{"lowercase after prefix hyphen", "ABC-def: Тема", 18, 22},
		{"leading repeated separator", ":: Тема: далее", 0, 0},
		{"leading repeated separator wrong later word", ":: Тема: Далее", 19, 23},
		{"combining mark", "E\u0301cole", 0, 0},
		{"combining mark lowercase first", "\u0301école", 11, 15},
		{"nonbreaking space", "A\u00a0B", 8, 12},
		{"emoji", "🙂АБ: Тема", 0, 0},
		{"lowercase without uppercase mapping", "ß: A", 9, 13},
		{"lowercase without mapping after first", "Aß", 0, 0},
	} {
		for _, form := range []string{"heading", "header"} {
			t.Run(tc.name+"/"+form, func(t *testing.T) {
				line := v203TestHeading(t, "###", tc.title, tc.title)
				parent, start, end := 3, 5, tc.headingEnd
				if form == "header" {
					line = v203TestTag(t, "header", tc.title, "title", 2)
					parent, start, end = 2, 9, tc.tagEnd
				}
				var want [][3]int
				if end != 0 {
					want = [][3]int{{3, start, end}}
				}
				result := v203TestDocument(t, line, v203TestText(t, "Read.", "Read.", parent))
				v203TestRun(t, result, "current", want)
			})
		}
	}
}

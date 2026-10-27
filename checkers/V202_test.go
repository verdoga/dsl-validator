package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v202TestMessage = "Регистр букв заголовка второго уровня отличается от оформления с прописной первой буквой и строчными остальными."

func v202TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value,
		Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v202TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v202TestHeading(t *testing.T, marker, raw, value string) model.Line {
	t.Helper()
	return v202TestLine(t, marker+" "+raw, "heading", 0,
		v202TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v202TestElement(t, "title", raw, value, len(marker)+2))
}

// v202TestDocument дополняет фрагмент обязательными версией и первым заголовком.
// Эталон и пример остаются неизменными на строке 3. Родители заданы входами;
// помощник не разбирает DSL. Ошибки unparsed сохраняются в истории парсера.
func v202TestDocument(t *testing.T, subtitle string, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v202TestLine(t, "@dsl-version 1.2", "tag", 0,
			v202TestElement(t, "tag", "@dsl-version", "dsl-version", 1),
			v202TestElement(t, "version", "1.2", "1.2", 14)),
		v202TestHeading(t, "#", title, title),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Diagnostics: []model.Diagnostic{},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}}
	if subtitle != "" {
		result.Document.Metadata.Subtitle = &subtitle
	}
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

func v202TestJSON(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v202TestRun проверяет только новые V202, исполняя полный штатный каталог.
// Ожидания задаются явно как [строка, начальная колонка, исключающая конечная].
func v202TestRun(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v202TestJSON(t, result)
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v202TestJSON(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldCount := len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V202" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V202 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != v202TestMessage || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "element" || entry.Fatal || entry.ID == "" ||
			entry.Source != source || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V202[%d] = %+v, want range %v and exact metadata", index, entry, expected)
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
					t.Fatal("V202 linked to an unrelated element")
				}
			}
		}
		if links != 1 {
			t.Fatalf("V202 has %d element links, want one title link", links)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v202TestJSON(t, result)), &restored); err != nil {
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
	if v202TestJSON(t, &restored) != before {
		t.Fatal("source data, metadata or previous history changed")
	}
}

func TestV202RegistrationAndVersions(t *testing.T) {
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
		if definition.Code != "V202" {
			continue
		}
		count++
		if definition.Message != v202TestMessage || definition.Severity != "recommendation" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V202 registered %d times, want one", count)
	}
	// Сейчас каталог DSL поддерживает только 1.2: остальные версии отклоняются
	// до запуска проверок. Общий размер каталога диагностик не фиксируется.
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			if checks, err := registry.ForVersion(dslversions.Version(version)); err == nil || checks != nil {
				t.Fatal("unsupported version accepted")
			}
			result := v202TestDocument(t, "Новые Слова", v202TestHeading(t, "##", "Новые Слова", "Новые Слова"))
			result.Document.DSLVersion = &version
			before := v202TestJSON(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatal("Prepare accepted unsupported version")
			}
			if v202TestJSON(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV202SentenceCase(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		want        [][3]int
	}{
		{"reference", "Новые слова", nil},
		{"example", "Новые Слова", [][3]int{{3, 4, 15}}},
		{"first lowercase", "новые слова", [][3]int{{3, 4, 15}}},
		{"many violations produce one finding", "НОВЫЕ СЛОВА", [][3]int{{3, 4, 15}}},
		{"one uppercase", "Я", nil},
		{"one lowercase", "я", [][3]int{{3, 4, 5}}},
		{"no cased letters", "123 — 世界!", nil},
		{"ignored prefix", "123 — новые слова", [][3]int{{3, 4, 21}}},
		{"ignored prefix before uppercase", "123 — Новые слова", nil},
		{"period does not restart", "Слово. Другое", [][3]int{{3, 4, 17}}},
		{"lowercase after period", "Слово. другое", nil},
		{"hyphen does not restart", "Новые-Слова", [][3]int{{3, 4, 15}}},
		{"abbreviation has no exception", "Урок NASA", [][3]int{{3, 4, 13}}},
		{"name has no exception", "Про Анну", [][3]int{{3, 4, 12}}},
		{"accented Latin", "École française", nil},
		{"accented Latin violation", "École Française", [][3]int{{3, 4, 19}}},
		{"titlecase is not uppercase", "ǅuro", [][3]int{{3, 4, 8}}},
		{"uppercase digraph", "Ǆuro", nil},
		{"titlecase is not lowercase", "Aǅ", [][3]int{{3, 4, 6}}},
		{"additional lowercase property", "Aʰ", nil},
		{"additional lowercase first", "ʰa", [][3]int{{3, 4, 6}}},
		{"supplementary plane letters", "𐐀𐐨", nil},
		{"supplementary plane lowercase first", "𐐨𐐨", [][3]int{{3, 4, 6}}},
		{"supplementary plane uppercase last", "𐐀𐐀", [][3]int{{3, 4, 6}}},
		{"uncased letters before and between", "世界A界b", nil},
		{"uncased letters do not restart", "A界B", [][3]int{{3, 4, 7}}},
		{"combining mark does not restart", "E\u0301cole", nil},
		{"combining mark before lowercase", "\u0301école", [][3]int{{3, 4, 10}}},
		{"numeric and symbol case properties are not letters", "ⅰⓐAⅧb", nil},
		{"number and emoji do not restart", "A1🙂B", [][3]int{{3, 4, 8}}},
		{"nonbreaking space does not restart", "A\u00a0B", [][3]int{{3, 4, 7}}},
		{"letters without case mapping remain lowercase", "Aß", nil},
		{"lowercase without uppercase mapping first", "ß", [][3]int{{3, 4, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v202TestDocument(t, tc.title, v202TestHeading(t, "##", tc.title, tc.title))
			v202TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV202OriginalRanges(t *testing.T) {
	for _, tc := range []struct {
		name, indent, gap, raw, value, trailing string
		start, end                              int
	}{
		{"tab indentation", "\t", " ", "Новые Слова", "Новые Слова", "", 5, 16},
		{"marker separator", "", "   ", "Новые Слова", "Новые Слова", "", 6, 17},
		{"trailing whitespace outside title", "", " ", "Новые Слова", "Новые Слова", " \t", 4, 15},
		// По DSL 1.21–1.22 внутри заголовка \@ не снимается:
		// @ имеет служебное значение только в начале строки.
		{"ordinary backslash", "", " ", `\@Новые Слова`, `\@Новые Слова`, "", 4, 17},
		{"escaped braces", "", " ", `\{Новые Слова\}`, "{Новые Слова}", "", 4, 19},
		{"escaped backslash", "", " ", `\\Новые Слова`, `\Новые Слова`, "", 4, 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			markerStart := utf8.RuneCountInString(tc.indent) + 1
			line := v202TestLine(t, tc.indent+"##"+tc.gap+tc.raw+tc.trailing, "heading", 0,
				v202TestElement(t, "heading-level", "##", "2", markerStart),
				v202TestElement(t, "title", tc.raw, tc.value, tc.start))
			v202TestRun(t, v202TestDocument(t, tc.value, line), "current", [][3]int{{3, tc.start, tc.end}})
		})
	}
}

func TestV202Selection(t *testing.T) {
	t.Run("first level is excluded", func(t *testing.T) {
		result := v202TestDocument(t, "")
		title := "новые СЛОВА"
		result.Lines[1] = v202TestHeading(t, "#", title, title)
		result.Lines[1].Number, result.Lines[1].LineEnding = 2, ""
		result.Document.Metadata.Title = &title
		v202TestRun(t, result, "current", nil)
	})
	t.Run("third level is excluded", func(t *testing.T) {
		result := v202TestDocument(t, "", v202TestHeading(t, "###", "новые СЛОВА", "новые СЛОВА"),
			v202TestLine(t, "Read.", "content", 3, v202TestElement(t, "content", "Read.", "Read.", 1)))
		v202TestRun(t, result, "current", nil)
	})
	t.Run("escaped heading is content", func(t *testing.T) {
		result := v202TestDocument(t, "", v202TestLine(t, `\## Новые Слова`, "content", 2,
			v202TestElement(t, "content", `\## Новые Слова`, "## Новые Слова", 1)))
		v202TestRun(t, result, "current", nil)
	})
	t.Run("editor content is literal", func(t *testing.T) {
		result := v202TestDocument(t, "",
			v202TestLine(t, "@editor {", "block-start", 2,
				v202TestElement(t, "tag", "@editor", "editor", 1), v202TestElement(t, "block-open", "{", "{", 9)),
			v202TestLine(t, "## Новые Слова", "content", 3,
				v202TestElement(t, "content", "## Новые Слова", "## Новые Слова", 1)),
			v202TestLine(t, "}", "block-end", 3, v202TestElement(t, "block-close", "}", "}", 1)))
		v202TestRun(t, result, "current", nil)
	})
	t.Run("invalid line with recognized title is excluded", func(t *testing.T) {
		// Неэкранированная { запрещена в заголовке по DSL 1.21.
		result := v202TestDocument(t, "",
			v202TestLine(t, "## Новые Слова {", "invalid", 0,
				v202TestElement(t, "heading-level", "##", "2", 1),
				v202TestElement(t, "title", "Новые Слова", "Новые Слова", 4),
				v202TestElement(t, "unparsed", "{", "", 16)))
		v202TestRun(t, result, "current", nil)
	})
	t.Run("heading level was not extracted", func(t *testing.T) {
		// Контракт допускает частичный разбор: unparsed с исходной ошибкой.
		result := v202TestDocument(t, "",
			v202TestLine(t, "## Новые Слова", "heading", 0,
				v202TestElement(t, "unparsed", "##", "", 1),
				v202TestElement(t, "title", "Новые Слова", "Новые Слова", 4)))
		v202TestRun(t, result, "current", nil)
	})
	t.Run("title was not extracted", func(t *testing.T) {
		result := v202TestDocument(t, "",
			v202TestLine(t, "## ", "heading", 0,
				v202TestElement(t, "heading-level", "##", "2", 1),
				v202TestElement(t, "unparsed", "", "", 4)))
		v202TestRun(t, result, "current", nil)
	})
	t.Run("empty string title supported by contract", func(t *testing.T) {
		// Пустая строка — доступное значение, не исключённый пользователем null.
		result := v202TestDocument(t, "", v202TestHeading(t, "##", "", ""))
		v202TestRun(t, result, "current", nil)
	})
}

func TestV202IndependentHeadingsAndContent(t *testing.T) {
	t.Run("two violations", func(t *testing.T) {
		// Несколько ## отдельно нарушают DSL 2.1.4, но таблица V202 требует
		// проверять каждый заголовок независимо, без подавления другими кодами.
		result := v202TestDocument(t, "",
			v202TestHeading(t, "##", "Новые Слова", "Новые Слова"),
			v202TestHeading(t, "##", "новые слова", "новые слова"))
		v202TestRun(t, result, "current", [][3]int{{3, 4, 15}, {4, 4, 15}})
	})
	t.Run("first and last checked independently", func(t *testing.T) {
		result := v202TestDocument(t, "",
			v202TestHeading(t, "##", "Новые Слова", "Новые Слова"),
			v202TestHeading(t, "##", "Новые слова", "Новые слова"),
			v202TestHeading(t, "##", "новые слова", "новые слова"))
		v202TestRun(t, result, "current", [][3]int{{3, 4, 15}, {5, 4, 15}})
	})
	t.Run("first letter state is local to each heading", func(t *testing.T) {
		result := v202TestDocument(t, "",
			v202TestHeading(t, "##", "А", "А"),
			v202TestHeading(t, "##", "Б", "Б"),
			v202TestHeading(t, "##", "123", "123"))
		v202TestRun(t, result, "current", nil)
	})
	for _, tc := range []struct {
		name, title string
		want        [][3]int
	}{
		{"content does not repair title", "Новые Слова", [][3]int{{3, 4, 15}}},
		{"content does not invalidate title", "Новые слова", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v202TestDocument(t, tc.title,
				v202TestHeading(t, "##", tc.title, tc.title),
				v202TestLine(t, "", "blank", 3),
				v202TestLine(t, "НОВЫЕ СЛОВА", "content", 3, v202TestElement(t, "content", "НОВЫЕ СЛОВА", "НОВЫЕ СЛОВА", 1)),
				v202TestLine(t, "@note {", "block-start", 3,
					v202TestElement(t, "tag", "@note", "note", 1), v202TestElement(t, "block-open", "{", "{", 7)),
				v202TestLine(t, "новые слова", "content", 6, v202TestElement(t, "content", "новые слова", "новые слова", 1)),
				v202TestLine(t, "}", "block-end", 6, v202TestElement(t, "block-close", "}", "}", 1)))
			v202TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV202EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v202TestDocument(t, "Новые Слова", v202TestHeading(t, "##", "Новые Слова", "Новые Слова"))
				for index := range result.Lines {
					result.Lines[index].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v202TestRun(t, result, "first", [][3]int{{3, 4, 15}})
				v202TestRun(t, result, "second", [][3]int{{3, 4, 15}})
				v202TestRun(t, v202TestDocument(t, "Новые слова", v202TestHeading(t, "##", "Новые слова", "Новые слова")), "other", nil)
			})
		}
	}
	t.Run("no second level heading", func(t *testing.T) {
		v202TestRun(t, v202TestDocument(t, ""), "current", nil)
	})
	t.Run("empty input", func(t *testing.T) {
		result := v202TestDocument(t, "")
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v202TestRun(t, result, "current", nil)
	})
}

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

const v090TestMessage = "Первая строка блока подходит для рассмотрения в качестве заголовка."

const v090TestDetail = "Рассмотрите перенос этой строки в название блока; решение зависит от её роли в материале"

// v090TestWant задаёт независимый диапазон с исключающей правой границей.
type v090TestWant struct{ line, start, end int }

func TestV090RegistrationAndVersions(t *testing.T) {
	registry := v090TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V090" {
			continue
		}
		count++
		if definition.Message != v090TestMessage || definition.Severity != "recommendation" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V090 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.5"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v090TestDocument(t)
			result.Document.DSLVersion = &version
			before := v090TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v090TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v090TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v090TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v090TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v090TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v090TestTag задаёт известное имя и явно указанный тип аргумента.
func v090TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v090TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v090TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v090TestLine(t, raw, "tag", parent, elements...)
}

// v090TestOpen добавляет открытие к известному объявлению блока.
func v090TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v090TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v090TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v090TestClose задаёт закрытие конкретного фигурного блока.
func v090TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v090TestLine(t, "}", "block-end", parent, v090TestElement(t, "block-close", "}", "}", 1))
}

// v090TestText сохраняет исходное и семантическое содержимое независимо.
func v090TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v090TestLine(t, raw, "content", parent, v090TestElement(t, "content", raw, value, 1))
}

// v090TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v090TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v090TestTag(t, "dsl-version", "1.2", "version", 0),
		v090TestLine(t, "# Lesson", "heading", 0, v090TestElement(t, "heading-level", "#", "1", 1),
			v090TestElement(t, "title", "Lesson", "Lesson", 3)),
		v090TestLine(t, "### Practice", "heading", 0, v090TestElement(t, "heading-level", "###", "3", 1),
			v090TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			line.Elements[j].ErrorIDs = append([]string{}, line.Elements[j].ErrorIDs...)
			if line.Elements[j].ElementType == "unparsed" {
				v090TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v090TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v090TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	// Все записи здесь принадлежат исходному парсеру. Контракт требует
	// порядка по позиции, даже если ошибки элементов добавлялись позднее.
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v090TestRegistry использует полный штатный каталог без отдельной регистрации.
func v090TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v090TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v090TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v090TestRun сравнивает новые V090 после штатных Prepare/Run с явным ожиданием.
func v090TestRun(t *testing.T, result *model.Result, processingID string, want []v090TestWant) {
	t.Helper()
	before := v090TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v090TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v090TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V090" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V090 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v090TestMessage+"\nконкретизация: "+v090TestDetail ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V090 occurrence %d = %+v, want %+v, no related", i, entry, location)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V090 created an element error link")
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
	if err := json.Unmarshal([]byte(v090TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Из сравнения исключаются только добавления конвейера и производные
	// флаги/ссылки. Исходные строки, метаданные и прежняя история сохраняются.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v090TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV090ReferenceAndExample(t *testing.T) {
	// Версия и два заголовка задают внешний контекст. Фрагменты задания
	// сохранены целиком начиная со строки 4, включая название и пунктуацию.
	for _, tc := range []struct {
		name, title, fragment string
		body                  []model.Line
		want                  []v090TestWant
	}{
		{"reference", "Examples", "@example Examples {\nShe lives in London.\n}",
			[]model.Line{v090TestText(t, "She lives in London.", "She lives in London.", 4)}, nil},
		{"example", "", "@example {\nExamples\nShe lives in London.\n}",
			[]model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "She lives in London.", "She lives in London.", 4)},
			[]v090TestWant{{5, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v090TestOpen(t, "example", tc.title, "title", 3)}, tc.body...)
			result := v090TestDocument(t, append(body, v090TestClose(t, 4))...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v090TestRun(t, result, "current", tc.want)
			// V037 сообщает об отсутствии document-id на уровне документа.
			// V090 не должна выставлять флаг ошибки строк содержимого.
			for _, line := range result.Lines[3:] {
				if line.HasErrors {
					t.Fatal("recommendation introduced a line error flag")
				}
			}
		})
	}
}

func TestV090WordsAndCase(t *testing.T) {
	for _, tc := range []struct {
		raw string
		end int
	}{
		{"Examples", 9}, {"USEFUL EXAMPLES", 16}, {"Useful examples", 0},
		{"Cafe\u0301", 6}, {"«Résumé!»", 10}, {"well-being", 11}, {"well‐being", 11}, {"well‑being", 11},
		{"don't", 6}, {"don’t", 6}, {"lʼami", 6}, {"word--word", 0}, {"word/word", 0},
		{"word’'word", 0}, {"word—word", 0}, {"\u0301", 0}, {"\u0301word", 0}, {"word-\u0301word", 0},
		{"中文", 3}, {"123", 0}, {"b2b", 0}, {"B2B", 4}, {"ABC DEF", 8}, {"ABC Def", 0}, {"ǅ ABC", 0},
		{"...", 0}, {"ÉCOLE", 6}, {"école", 6}, {"СЛОВА ТУТ", 10}, {"😀", 0},
		{" \tExamples  ", 13}, {"\u00a0Examples", 0}, {"Examples\u00a0", 0},
		{"Examples\tother", 0}, {"EXAMPLES\tOTHER", 15},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			var want []v090TestWant
			if tc.end != 0 {
				want = []v090TestWant{{5, 1, tc.end}}
			}
			v090TestRun(t, v090TestBlock(t, "example", v090TestText(t, tc.raw, strings.Trim(tc.raw, " \t"), 4),
				v090TestText(t, "Read the text.", "Read the text.", 4)), "current", want)
		})
	}
}

func TestV090SelectedBlocksAndTitles(t *testing.T) {
	for _, name := range []string{"example", "table", "script", "text", "key", "note", "alt", "EXAMPLE", "TEXT", "ALT"} {
		t.Run(name, func(t *testing.T) {
			parent, block, number := 3, 4, 5
			var body []model.Line
			if name == "key" {
				body = append(body, v090TestTag(t, "task", "practice", "identifier", 3))
				parent, block, number = 4, 5, 6
			}
			body = append(body, v090TestOpen(t, name, "", "", parent), v090TestText(t, "Examples", "Examples", block),
				v090TestText(t, "Read the text.", "Read the text.", block), v090TestClose(t, block))
			v090TestRun(t, v090TestDocument(t, body...), "current", []v090TestWant{{number, 1, 9}})
		})
	}
	for _, name := range []string{"wordlist", "instr", "hint", "editor"} {
		t.Run("excluded "+name, func(t *testing.T) {
			v090TestRun(t, v090TestBlock(t, name, v090TestText(t, "Examples", "Examples", 4),
				v090TestText(t, "Other words", "Other words", 4)), "current", nil)
		})
	}
	for _, title := range []string{"Examples", "Примеры", "\u00a0"} {
		for _, kind := range []model.ElementType{"title", "unparsed"} {
			t.Run("title "+title+string(kind), func(t *testing.T) {
				result := v090TestDocument(t, v090TestOpen(t, "example", title, kind, 3), v090TestText(t, "Examples", "Examples", 4),
					v090TestText(t, "Read the text.", "Read the text.", 4), v090TestClose(t, 4))
				v090TestRun(t, result, "current", nil)
			})
		}
	}
	t.Run("whitespace before opening", func(t *testing.T) {
		open := v090TestLine(t, "@example \t {", "block-start", 3,
			v090TestElement(t, "tag", "@example", "example", 1), v090TestElement(t, "unparsed", "\t", "", 10),
			v090TestElement(t, "block-open", "{", "{", 12))
		v090TestRun(t, v090TestDocument(t, open, v090TestText(t, "Examples", "Examples", 4),
			v090TestText(t, "Read the text.", "Read the text.", 4), v090TestClose(t, 4)), "current", []v090TestWant{{5, 1, 9}})
	})
	t.Run("inline form", func(t *testing.T) {
		v090TestRun(t, v090TestDocument(t, v090TestTag(t, "note", "Examples", "content", 3),
			v090TestText(t, "Read the text.", "Read the text.", 3)), "current", nil)
	})
}

func TestV090FirstRowAndRemainingContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v090TestWant
	}{
		{"leading blanks", []model.Line{v090TestLine(t, " \t", "blank", 4), v090TestText(t, "Examples", "Examples", 4),
			v090TestText(t, "Read the text.", "Read the text.", 4)}, []v090TestWant{{6, 1, 9}}},
		{"only candidate", []model.Line{v090TestText(t, "Examples", "Examples", 4)}, nil},
		{"candidate and blanks", []model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestLine(t, " \t", "blank", 4)}, nil},
		{"blank before remainder", []model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestLine(t, " \t", "blank", 4),
			v090TestText(t, "Read the text.", "Read the text.", 4)}, []v090TestWant{{5, 1, 9}}},
		{"no search for later candidate", []model.Line{v090TestText(t, "Useful examples", "Useful examples", 4),
			v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Read the text.", "Read the text.", 4)}, nil},
		{"invalid first row", []model.Line{v090TestLine(t, "@", "invalid", 4, v090TestElement(t, "unparsed", "@", "", 1)),
			v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Read the text.", "Read the text.", 4)}, nil},
		{"invalid remainder", []model.Line{v090TestText(t, "Examples", "Examples", 4),
			v090TestLine(t, "@", "invalid", 4, v090TestElement(t, "unparsed", "@", "", 1))}, nil},
		{"two candidates one finding", []model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Exercises", "Exercises", 4)},
			[]v090TestWant{{5, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v090TestRun(t, v090TestBlock(t, "example", tc.body...), "current", tc.want) })
	}
	for _, name := range []string{"example", "table"} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want []v090TestWant
		}{
			{"empty own part", []model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestSeparator(t, 4),
				v090TestText(t, "Read the text.", "Read the text.", 4)}, nil},
			{"preserved own part", []model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Read the text.", "Read the text.", 4),
				v090TestSeparator(t, 4), v090TestText(t, "Write the answer.", "Write the answer.", 4)}, []v090TestWant{{5, 1, 9}}},
			{"escaped separator is content", []model.Line{v090TestText(t, "Examples", "Examples", 4), v090TestText(t, `\---`, "---", 4)}, []v090TestWant{{5, 1, 9}}},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) { v090TestRun(t, v090TestBlock(t, name, tc.body...), "current", tc.want) })
		}
		t.Run(name+"/separator first", func(t *testing.T) {
			result := v090TestBlock(t, name, v090TestSeparator(t, 4), v090TestText(t, "Examples", "Examples", 4),
				v090TestText(t, "Read the text.", "Read the text.", 4))
			v090TestParserError(t, result, 5, 0, "Перед разделителем отсутствует содержимое.")
			v090TestRun(t, result, "current", nil)
		})
	}
	t.Run("dashes ordinary in text", func(t *testing.T) {
		v090TestRun(t, v090TestBlock(t, "text", v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "---", "---", 4)),
			"current", []v090TestWant{{5, 1, 9}})
	})
}

func TestV090HTML(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		end     int
		invalid bool
	}{
		{"<strong>Examples</strong>", 26, false}, {"<EM><u>Examples</u></EM>", 25, false}, {"Ex<em>am</em>ples", 18, false},
		{"<strong>USEFUL EXAMPLES</strong>", 33, false}, {"<strong>Useful examples</strong>", 0, false},
		{"<strong><em>Examples</em></strong>", 35, false},
		{"<br>", 0, false}, {"<BR>", 0, false}, {"<strong></strong>", 0, false}, {"<em> \t</em>", 0, false},
		{"<strong>Examples", 0, true}, {"<em><u>Examples</em></u>", 0, true}, {"Examples</em>", 0, true},
		{"<strong class=\"title\">Examples</strong>", 0, true}, {"<b>Examples</b>", 0, true},
		{"<br/>", 0, true}, {"<br />", 0, true}, {"<br>Examples", 0, true},
		{`\<em\>Examples\</em\>`, 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			value := tc.raw
			if tc.raw == `\<em\>Examples\</em\>` {
				value = `<em>Examples</em>`
			}
			result := v090TestBlock(t, "text", v090TestText(t, tc.raw, value, 4),
				v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Read the text.", "Read the text.", 4))
			if tc.invalid {
				v090TestParserError(t, result, 5, 0, "Некорректная HTML-разметка.")
			}
			var want []v090TestWant
			if tc.end != 0 {
				want = []v090TestWant{{5, 1, tc.end}}
			}
			v090TestRun(t, result, "current", want)
		})
	}
	for _, remainder := range []string{"<br>", "<strong></strong>", "<em> \t</em>"} {
		t.Run("empty remainder "+remainder, func(t *testing.T) {
			v090TestRun(t, v090TestBlock(t, "text", v090TestText(t, "Examples", "Examples", 4),
				v090TestText(t, remainder, remainder, 4)), "current", nil)
		})
	}
	t.Run("skip empty remainder until visible content", func(t *testing.T) {
		v090TestRun(t, v090TestBlock(t, "text", v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "<br>", "<br>", 4),
			v090TestText(t, "Read the text.", "Read the text.", 4)), "current", []v090TestWant{{5, 1, 9}})
	})
}

func TestV090Tables(t *testing.T) {
	for _, tc := range []struct {
		raw, value, rest string
		end              int
	}{
		{"Examples", "Examples", "Read the text.", 9},
		{"EXAMPLES | FORMS", "EXAMPLES | FORMS", "A | B", 0},
		{"<EXAMPLES | FORMS>", "<EXAMPLES | FORMS>", "<A | B>", 0},
		{`EXAMPLES\|FORMS`, "EXAMPLES|FORMS", "Read the text.", 16},
		{`EXAMPLES\\|FORMS`, `EXAMPLES\|FORMS`, "A | B", 0},
		{`\<EXAMPLES\>`, "<EXAMPLES>", "Read the text.", 13},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			var want []v090TestWant
			if tc.end != 0 {
				want = []v090TestWant{{5, 1, tc.end}}
			}
			v090TestRun(t, v090TestBlock(t, "table", v090TestText(t, tc.raw, tc.value, 4),
				v090TestText(t, tc.rest, tc.rest, 4)), "current", want)
		})
	}
}

func TestV090BoundariesAndNesting(t *testing.T) {
	t.Run("nested in fragment", func(t *testing.T) {
		result := v090TestDocument(t, v090TestOpen(t, "fragment", "intro", "identifier", 3), v090TestOpen(t, "example", "", "", 4),
			v090TestText(t, "Examples", "Examples", 5), v090TestText(t, "Read the text.", "Read the text.", 5), v090TestClose(t, 5), v090TestClose(t, 4))
		v090TestRun(t, result, "current", []v090TestWant{{6, 1, 9}})
	})
	t.Run("neighbor cannot supply content", func(t *testing.T) {
		result := v090TestDocument(t, v090TestOpen(t, "example", "", "", 3), v090TestText(t, "Examples", "Examples", 4), v090TestClose(t, 4),
			v090TestOpen(t, "example", "", "", 3), v090TestText(t, "Useful examples", "Useful examples", 7),
			v090TestText(t, "Read the text.", "Read the text.", 7), v090TestClose(t, 7))
		v090TestRun(t, result, "current", nil)
	})
	for _, first := range []bool{true, false} {
		t.Run("nested declaration first="+strconv.FormatBool(first), func(t *testing.T) {
			body := []model.Line{v090TestOpen(t, "example", "", "", 3)}
			if !first {
				body = append(body, v090TestText(t, "Examples", "Examples", 4))
			}
			body = append(body, v090TestOpen(t, "note", "Nested", "title", 4))
			nested := 5
			if !first {
				nested = 6
			}
			body = append(body, v090TestText(t, "Read the text.", "Read the text.", nested), v090TestClose(t, nested))
			if first {
				body = append(body, v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Read the text.", "Read the text.", 4))
			}
			result := v090TestDocument(t, append(body, v090TestClose(t, 4))...)
			v090TestParserError(t, result, nested, 0, "Вложение note запрещено внутри example.")
			v090TestRun(t, result, "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			result := v090TestDocument(t, v090TestTag(t, "task", "practice", "identifier", 3),
				v090TestTag(t, "step", "Prepare", "title", 4), v090TestOpen(t, name, "", "", 5),
				v090TestText(t, "@example {", "@example {", 6), v090TestText(t, "Examples", "Examples", 6),
				v090TestText(t, "Read the text.", "Read the text.", 6), v090TestClose(t, 6))
			v090TestRun(t, result, "current", nil)
		})
	}
}

func TestV090EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v090TestDocument(t, v090TestOpen(t, "example", "", "", 3), v090TestText(t, "Examples", "Examples", 4),
				v090TestText(t, "Read the text.", "Read the text.", 4), v090TestClose(t, 4),
				v090TestOpen(t, "text", "", "", 3), v090TestText(t, "Examples", "Examples", 8),
				v090TestText(t, "Read the text.", "Read the text.", 8), v090TestClose(t, 8))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v090TestWant{{5, 1, 9}, {9, 1, 9}}
			v090TestRun(t, result, "first", want)
			v090TestRun(t, v090TestDocument(t), "other", nil)
			v090TestRun(t, result, "second", want)
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v090TestDocument(t)
			result.Lines = []model.Line{}
			count := 0
			result.Document.LineCount = &count
			result.Document.Metadata.Title = nil
			if unavailable {
				result.Document.LineCount = nil
			}
			v090TestRun(t, result, "current", nil)
		})
	}
}

// v090TestBlock строит полноценный блок внутри шага; строки задаются явно.
func v090TestBlock(t *testing.T, name string, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v090TestOpen(t, name, "", "", 3)}, rows...)
	return v090TestDocument(t, append(body, v090TestClose(t, 4))...)
}

// v090TestSeparator задаёт служебную строку example/table, не подменяя её текстом.
func v090TestSeparator(t *testing.T, parent int) model.Line {
	t.Helper()
	return v090TestLine(t, "---", "separator", parent, v090TestElement(t, "content", "---", "---", 1))
}

func TestV090PlaceholderRemainingContent(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, suffix string
		want                 []v090TestWant
	}{
		{"only placeholder", "", "", []v090TestWant{{5, 1, 9}}},
		{"formatted placeholder", "<em>", "</em>", []v090TestWant{{5, 1, 9}}},
		{"text and placeholder", "Write ", " here.", []v090TestWant{{5, 1, 9}}},
		{"broken formatting", "<em>", "", nil},
		{"inline br", "", "<br>", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var elements []model.Element
			if tc.prefix != "" {
				elements = append(elements, v090TestElement(t, "content", tc.prefix, tc.prefix, 1))
			}
			start := utf8.RuneCountInString(tc.prefix) + 1
			gap := v090TestElement(t, "placeholder", "_____", "", start)
			gap.Value = nil
			elements = append(elements, gap)
			if tc.suffix != "" {
				elements = append(elements, v090TestElement(t, "content", tc.suffix, tc.suffix, start+5))
			}
			row := v090TestLine(t, tc.prefix+"_____"+tc.suffix, "content", 4, elements...)
			v090TestRun(t, v090TestBlock(t, "text",
				v090TestText(t, "Examples", "Examples", 4), row), "current", tc.want)
		})
	}
	t.Run("placeholder is not a candidate", func(t *testing.T) {
		gap := v090TestElement(t, "placeholder", "_____", "", 1)
		gap.Value = nil
		v090TestRun(t, v090TestBlock(t, "text", v090TestLine(t, "_____", "content", 4, gap),
			v090TestText(t, "Examples", "Examples", 4), v090TestText(t, "Read.", "Read.", 4)), "current", nil)
	})
	t.Run("nested placeholder does not remain in outer text", func(t *testing.T) {
		gap := v090TestElement(t, "placeholder", "_____", "", 1)
		gap.Value = nil
		v090TestRun(t, v090TestBlock(t, "text", v090TestText(t, "Examples", "Examples", 4),
			v090TestOpen(t, "note", "", "", 4), v090TestLine(t, "_____", "content", 6, gap),
			v090TestClose(t, 6)), "current", nil)
	})
}

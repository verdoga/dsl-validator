package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v093TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V093.
func v093TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v093TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v093TestHeading(t, "", "#", "1", title),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	result.Lines[len(result.Lines)-1].LineEnding = ""
	return result
}

// v093TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v093TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v093TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v093TestElement(t, "heading-level", marker, level, start, end),
		v093TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v093TestTag собирает однострочный тег с одним явно типизированным параметром.
func v093TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v093TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v093TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v093TestLine(t, raw, "tag", parent, depth, elements...)
}

// v093TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v093TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v093TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v093TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v093TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v093TestClose задаёт закрытие блока с явным родителем.
func v093TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v093TestLine(t, "}", "block-end", parent, depth, v093TestElement(t, "block-close", "}", "}", 1, 2))
}

// v093TestText сохраняет независимо заданные исходное и семантическое значения.
func v093TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v093TestLine(t, raw, "content", parent, depth,
		v093TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v093TestLine не классифицирует исходник и не вычисляет родительские связи.
func v093TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v093TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v093TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v093TestRegistry использует штатный каталог целиком.
func v093TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v093TestRun проверяет новые V093: строка, начало и исключающий конец.
func v093TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v093TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v093TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v093TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V093" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V093 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Строка блочного списка слов заканчивается знаком препинания.\nконкретизация: Рассмотрите удаление конечной пунктуации, если она не является частью слова, сокращения или цитируемой фразы." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V093 metadata", i, entry, location)
		}
		if len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v093TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 ||
		len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v093TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v093TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v093TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV093Registration(t *testing.T) {
	definitions, err := v093TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V093" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Строка блочного списка слов заканчивается знаком препинания." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V093 definitions = %d, want 1", count)
	}
}

// v093TestTitled задаёт блок с отдельным элементом названия.
func v093TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v093TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v093TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v093TestElement(t, "title", title, title, len(tag)+3, end),
		v093TestElement(t, "block-open", "{", "{", end+1, end+2))
}

func TestV093ReferenceAndExample(t *testing.T) {
	// По DSL 1.3, 1.30 и 2.1.3 добавлены только @dsl-version и # Lesson.
	for _, tc := range []struct {
		name, fragment string
		first, second  string
		want           [][3]int
	}{
		{"reference", "@wordlist {\nItalian\nSpanish\n}", "Italian", "Spanish", nil},
		{"example", "@wordlist {\nItalian;\nSpanish.\n}", "Italian;", "Spanish.", [][3]int{{4, 8, 9}, {5, 8, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v093TestDocument(t, v093TestOpen(t, "wordlist", 2, 1),
				v093TestText(t, tc.first, tc.first, 3, 2), v093TestText(t, tc.second, tc.second, 3, 2), v093TestClose(t, 3, 2))
			var raw strings.Builder
			for _, line := range result.Lines[2:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", raw.String())
			}
			v093TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV093TerminalPunctuation(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"period", "a.", "a.", [][3]int{{4, 2, 3}}},
		{"comma", "a,", "a,", [][3]int{{4, 2, 3}}},
		{"semicolon", "a;", "a;", [][3]int{{4, 2, 3}}},
		{"colon", "a:", "a:", [][3]int{{4, 2, 3}}},
		{"exclamation", "a!", "a!", [][3]int{{4, 2, 3}}},
		{"question", "a?", "a?", [][3]int{{4, 2, 3}}},
		{"ellipsis", "a…", "a…", [][3]int{{4, 2, 3}}},
		{"maximal suffix", "a.,;:!?…", "a.,;:!?…", [][3]int{{4, 2, 9}}},
		{"only punctuation", "...", "...", [][3]int{{4, 1, 4}}},
		{"space interrupts suffix", "a! ?", "a! ?", [][3]int{{4, 4, 5}}},
		{"tab interrupts suffix", "a!\t?", "a!\t?", [][3]int{{4, 4, 5}}},
		{"ascii margins", " \tItalian; \t", "Italian;", [][3]int{{4, 10, 11}}},
		{"trailing nbsp", "a.\u00a0", "a.\u00a0", nil},
		{"trailing em space", "a.\u2003", "a.\u2003", nil},
		{"nbsp before suffix", "a\u00a0.", "a\u00a0.", [][3]int{{4, 3, 4}}},
		{"unicode", "ёж…", "ёж…", [][3]int{{4, 3, 4}}},
		{"non bmp unicode", "😀!", "😀!", [][3]int{{4, 2, 3}}},
		{"combining unicode", "e\u0301!", "e\u0301!", [][3]int{{4, 3, 4}}},
		{"closing quote", "word.\"", "word.\"", nil},
		{"closing apostrophe", "word.'", "word.'", nil},
		{"closing guillemet", "word.»", "word.»", nil},
		{"closing parenthesis", "word.)", "word.)", nil},
		{"closing bracket", "word.]", "word.]", nil},
		{"punctuation after quote", "word\".", "word\".", [][3]int{{4, 6, 7}}},
		{"internal apostrophe", "don't", "don't", nil},
		{"apostrophe and suffix", "don't!", "don't!", [][3]int{{4, 6, 7}}},
		{"abbreviation", "etc.", "etc.", [][3]int{{4, 4, 5}}},
		{"uppercase", "ITALIAN;", "ITALIAN;", [][3]int{{4, 8, 9}}},
		{"internal punctuation", "hello, world", "hello, world", nil},
		{"hyphen is not checked", "word-", "word-", nil},
		{"escaped initial marker", `\@word!`, "@word!", [][3]int{{4, 7, 8}}},
		{"escaped brace", `\{word!`, "{word!", [][3]int{{4, 7, 8}}},
		{"literal slash before period", `word\.`, `word\.`, [][3]int{{4, 6, 7}}},
		{"escaped slash before period", `word\\.`, `word\.`, [][3]int{{4, 7, 8}}},
		{"ordinary text", "Italian", "Italian", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v093TestRun(t, v093TestDocument(t, v093TestOpen(t, "wordlist", 2, 1),
				v093TestText(t, tc.raw, tc.value, 3, 2), v093TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV093Scope(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty block", []model.Line{v093TestOpen(t, "wordlist", 2, 1), v093TestClose(t, 3, 2)}, nil},
		{"blank rows", []model.Line{v093TestOpen(t, "wordlist", 2, 1),
			v093TestLine(t, "", "blank", 3, 2), v093TestLine(t, " \t ", "blank", 3, 2), v093TestClose(t, 3, 2)}, nil},
		{"title is not a row", []model.Line{v093TestTitled(t, "wordlist", "Words!", 2, 1),
			v093TestText(t, "Italian", "Italian", 3, 2), v093TestClose(t, 3, 2)}, nil},
		{"inline wordlist", []model.Line{v093TestTag(t, "wordlist", "content", "Italian; Spanish.", 2, 1)}, nil},
		{"outside wordlist", []model.Line{v093TestText(t, "Italian;", "Italian;", 2, 1)}, nil},
		{"other block", []model.Line{v093TestOpen(t, "note", 2, 1),
			v093TestText(t, "Italian;", "Italian;", 3, 2), v093TestClose(t, 3, 2)}, nil},
		{"uppercase block", []model.Line{v093TestOpen(t, "WORDLIST", 2, 1),
			v093TestText(t, "Italian;", "Italian;", 3, 2), v093TestClose(t, 3, 2)}, [][3]int{{4, 8, 9}}},
		{"nested wordlists", []model.Line{v093TestOpen(t, "wordlist", 2, 1),
			v093TestText(t, "Outer;", "Outer;", 3, 2), v093TestOpen(t, "wordlist", 3, 2),
			v093TestText(t, "Inner.", "Inner.", 5, 3), v093TestClose(t, 5, 3), v093TestClose(t, 3, 2)},
			[][3]int{{4, 6, 7}, {6, 6, 7}}},
		{"nested block does not supply rows", []model.Line{v093TestOpen(t, "wordlist", 2, 1),
			v093TestOpen(t, "note", 3, 2), v093TestText(t, "Hidden;", "Hidden;", 4, 3),
			v093TestClose(t, 4, 3), v093TestText(t, "Visible!", "Visible!", 3, 2), v093TestClose(t, 3, 2)},
			[][3]int{{7, 8, 9}}},
		{"wordlist inside other block", []model.Line{v093TestOpen(t, "note", 2, 1),
			v093TestOpen(t, "wordlist", 3, 2), v093TestText(t, "Inner.", "Inner.", 4, 3),
			v093TestClose(t, 4, 3), v093TestClose(t, 3, 2)}, [][3]int{{5, 6, 7}}},
		{"nested declaration is not content", []model.Line{v093TestOpen(t, "wordlist", 2, 1),
			v093TestTag(t, "note", "content", "Not a list row!", 3, 2), v093TestClose(t, 3, 2)}, nil},
		{"separate blocks", []model.Line{v093TestOpen(t, "wordlist", 2, 1),
			v093TestText(t, "First!", "First!", 3, 2), v093TestClose(t, 3, 2),
			v093TestOpen(t, "wordlist", 2, 1), v093TestText(t, "Last?", "Last?", 6, 2), v093TestClose(t, 6, 2)},
			[][3]int{{4, 6, 7}, {7, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v093TestRun(t, v093TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal rows in "+tag, func(t *testing.T) {
			v093TestRun(t, v093TestDocument(t, v093TestOpen(t, tag, 2, 1),
				v093TestText(t, "@wordlist Words!", "@wordlist Words!", 3, 2),
				v093TestText(t, "Italian;", "Italian;", 3, 2), v093TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV093EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v093TestDocument(t, v093TestOpen(t, "wordlist", 2, 1),
				v093TestText(t, "Italian;", "Italian;", 3, 2), v093TestText(t, "Spanish.", "Spanish.", 3, 2), v093TestClose(t, 3, 2))
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v093TestRun(t, result, "first", [][3]int{{4, 8, 9}, {5, 8, 9}})
			v093TestRun(t, result, "second", [][3]int{{4, 8, 9}, {5, 8, 9}})
			v093TestRun(t, v093TestDocument(t, v093TestOpen(t, "wordlist", 2, 1),
				v093TestText(t, "Italian", "Italian", 3, 2), v093TestClose(t, 3, 2)), "separate", nil)
		})
	}
}

func TestV093EmptyAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		for _, known := range []bool{false, true} {
			t.Run(version+" known count="+strconv.FormatBool(known), func(t *testing.T) {
				result := &model.Result{Format: model.FormatVersion1,
					Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
					Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
				if known {
					count := 0
					result.Document.LineCount = &count
				}
				if version == "1.2" {
					v093TestRun(t, result, "current", nil)
					return
				}
				before := v093TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v093TestRegistry(t))
				if err == nil || checks != nil || v093TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

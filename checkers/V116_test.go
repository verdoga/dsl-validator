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

// v116TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V116.
func v116TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v116TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v116TestHeading(t, "", "#", "1", title),
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

// v116TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v116TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v116TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v116TestElement(t, "heading-level", marker, level, start, end),
		v116TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v116TestTag собирает однострочный тег с одним явно типизированным параметром.
func v116TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v116TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v116TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v116TestLine(t, raw, "tag", parent, depth, elements...)
}

// v116TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v116TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v116TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v116TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v116TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v116TestClose задаёт закрытие блока с явным родителем.
func v116TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v116TestLine(t, "}", "block-end", parent, depth, v116TestElement(t, "block-close", "}", "}", 1, 2))
}

// v116TestText сохраняет независимо заданные исходное и семантическое значения.
func v116TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v116TestLine(t, raw, "content", parent, depth,
		v116TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v116TestLine не классифицирует исходник и не вычисляет родительские связи.
func v116TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v116TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v116TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v116TestRegistry использует штатный каталог целиком.
func v116TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v116TestRun проверяет новые V116: строка, начало и исключающий конец.
func v116TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v116TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v116TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v116TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V116" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V116 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Первая строка текста имеет признаки заголовка.\nконкретизация: Если это заголовок текста, перенесите его в TITLE после @text; решение принимает редактор" {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V116 metadata", i, entry, location)
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
	if err := json.Unmarshal([]byte(v116TestSnapshot(t, result)), &restored); err != nil {
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
	if v116TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v116TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v116TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV116Registration(t *testing.T) {
	definitions, err := v116TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V116" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Первая строка текста имеет признаки заголовка." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V116 definitions = %d, want 1", count)
	}
}

// v116TestTitled задаёт блок с отдельным элементом названия.
func v116TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v116TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v116TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v116TestElement(t, "title", title, title, len(tag)+3, end),
		v116TestElement(t, "block-open", "{", "{", end+1, end+2))
}

func TestV116EmptyAndVersions(t *testing.T) {
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
					v116TestRun(t, result, "current", nil)
					return
				}
				before := v116TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v116TestRegistry(t))
				if err == nil || checks != nil || v116TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

// v116TestBlock окружает непосредственные строки полным неназванным text.
// TestDocument добавляет обязательные версию и заголовок перед блоком.
func v116TestBlock(t *testing.T, rows ...string) *model.Result {
	t.Helper()
	body := []model.Line{v116TestOpen(t, "text", 2, 1)}
	for _, raw := range rows {
		if strings.Trim(raw, " \t") == "" {
			body = append(body, v116TestLine(t, raw, "blank", 3, 2))
		} else {
			body = append(body, v116TestText(t, raw, raw, 3, 2))
		}
	}
	body = append(body, v116TestClose(t, 3, 2))
	return v116TestDocument(t, body...)
}

func TestV116ReferenceAndExample(t *testing.T) {
	// Исходные примеры сохранены; добавлены только @dsl-version и # Lesson.
	v116TestRun(t, v116TestDocument(t, v116TestTitled(t, "text", "London", 2, 1),
		v116TestText(t, "London is a large city.", "London is a large city.", 3, 2), v116TestClose(t, 3, 2)), "current", nil)
	v116TestRun(t, v116TestBlock(t, "London", "London is a large city."), "current", [][3]int{{4, 1, 7}})
}

func TestV116CandidateWordsAndCase(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"one word", "London", [][3]int{{4, 1, 7}}},
		{"lowercase word", "london", [][3]int{{4, 1, 7}}},
		{"uppercase phrase", "THE CITY", [][3]int{{4, 1, 9}}},
		{"mixed phrase", "London city", nil},
		{"lowercase phrase", "a large city", nil},
		{"digits", "123", nil},
		{"Unicode digits", "١٢٣", nil},
		{"Roman number", "Ⅻ", nil},
		{"uppercase with number", "CITY 123", [][3]int{{4, 1, 9}}},
		{"no cased letters but one word", "東京", [][3]int{{4, 1, 3}}},
		{"no cased letters and two words", "東京 大阪", nil},
		{"decomposed accent", "Cafe\u0301", [][3]int{{4, 1, 6}}},
		{"precomposed accent", "Café", [][3]int{{4, 1, 5}}},
		{"marks only", "\u0301", nil},
		{"leading mark", "\u0301London", nil},
		{"edge punctuation", "«London!»", [][3]int{{4, 1, 10}}},
		{"punctuation only", "...", nil},
		{"ASCII hyphen", "co-operate", [][3]int{{4, 1, 11}}},
		{"hyphen", "co‐operate", [][3]int{{4, 1, 11}}},
		{"nonbreaking hyphen", "co‑operate", [][3]int{{4, 1, 11}}},
		{"ASCII apostrophe", "l'amour", [][3]int{{4, 1, 8}}},
		{"curly apostrophe", "l’amour", [][3]int{{4, 1, 8}}},
		{"modifier apostrophe", "lʼamour", [][3]int{{4, 1, 8}}},
		{"en dash", "co–operate", nil},
		{"em dash", "co—operate", nil},
		{"internal punctuation", "co,operate", nil},
		{"doubled connector", "co--operate", nil},
		{"internal digit", "London2", nil},
		{"titlecase in phrase", "ǅEN CITY", nil},
		{"modifier lowercase in phrase", "Aʰ CITY", nil},
		{"source indentation", " \tLondon \t", [][3]int{{4, 1, 11}}},
		{"NBSP preserved", "\u00a0London\u00a0", nil},
		{"later uppercase cannot supply first candidate", "London city", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v116TestRun(t, v116TestBlock(t, tc.raw, "London is a large city."), "current", tc.want)
		})
	}
}

func TestV116HTML(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"strong", "<strong>London</strong>", [][3]int{{4, 1, 24}}},
		{"nested case insensitive", "<EM><u>London</u></eM>", [][3]int{{4, 1, 23}}},
		{"joined word", "Lon<em>don</em>", [][3]int{{4, 1, 16}}},
		{"spaces inside formatting", "<em> London </em>", [][3]int{{4, 1, 18}}},
		{"empty formatting is not a candidate", "<em></em>", nil},
		{"unclosed", "<em>London", nil},
		{"unexpected close", "London</em>", nil},
		{"crossed", "<em><u>London</em></u>", nil},
		{"self closing", "<em/>London", nil},
		{"attribute", "<em class=x>London</em>", nil},
		{"unknown tag", "<span>London</span>", nil},
		{"incomplete tag", "<em London", nil},
		{"spaced marker", "<em >London</em>", nil},
		{"inline br", "<br>London", nil},
		{"self closing br", "<br/>", nil},
		{"br attribute", "<br class=x>", nil},
		{"HTML entities not decoded", "&lt;London&gt;", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v116TestRun(t, v116TestBlock(t, tc.raw, "London is a large city."), "current", tc.want)
		})
	}
}

func TestV116Escaping(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"escaped literal angles", `\<LONDON\>`, `<LONDON>`, [][3]int{{4, 1, 11}}},
		{"decoded marker not rescanned", `\<em\>London\</em\>`, `<em>London</em>`, nil},
		{"escaped braces", `\{London\}`, `{London}`, [][3]int{{4, 1, 11}}},
		{"leading at sign", `\@London`, `@London`, [][3]int{{4, 1, 9}}},
		{"leading hash", `\#London`, `#London`, [][3]int{{4, 1, 9}}},
		{"underscore sequence", `\__London__`, `__London__`, [][3]int{{4, 1, 12}}},
		{"unknown escape retained", `\London`, `\London`, [][3]int{{4, 1, 8}}},
		{"escaped backslash does not escape next marker", `\\<em>London</em>`, `\<em>London</em>`, [][3]int{{4, 1, 18}}},
		{"three slashes keep next angle literal", `\\\<em\>London\</em\>`, `\<em>London</em>`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestText(t, tc.raw, tc.value, 3, 2),
				v116TestText(t, "London is a large city.", "London is a large city.", 3, 2), v116TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV116FirstRowAndRemainingText(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
		want [][3]int
	}{
		{"no body", nil, nil},
		{"only candidate", []string{"London"}, nil},
		{"only gaps remain", []string{"London", "", " \t", "<br>", "<em></em>"}, nil},
		{"punctuation is nonempty remaining text", []string{"London", "."}, [][3]int{{4, 1, 7}}},
		{"initial blank and br", []string{"", "<br>", "London", "London is a large city."}, [][3]int{{6, 1, 7}}},
		{"case and whitespace br", []string{" \t<BR>\t", "London", "London is a large city."}, [][3]int{{5, 1, 7}}},
		{"later candidate is not selected", []string{"London is a large city.", "London", "More text."}, nil},
		{"empty visible first row does not select next", []string{"<em></em>", "London", "More text."}, nil},
		{"HTML error exclusion belongs to first row", []string{"London", "London is <em>a large city."}, [][3]int{{4, 1, 7}}},
		{"remaining text before unexpected close", []string{"London", "More text.</em>"}, [][3]int{{4, 1, 7}}},
		{"remaining text after unexpected close", []string{"London", "</em>More text."}, [][3]int{{4, 1, 7}}},
		{"remaining text with crossed formatting", []string{"London", "<em><u>More text.</em></u>"}, [][3]int{{4, 1, 7}}},
		{"remaining prefix before unknown markup", []string{"London", "More text.<span>"}, [][3]int{{4, 1, 7}}},
		{"remaining prefix before invalid angle", []string{"London", "More text.>"}, [][3]int{{4, 1, 7}}},
		{"only broken markers remain", []string{"London", "<em>", "</u>"}, nil},
		{"whitespace prefix before invalid markup", []string{"London", "<em> \t<span>"}, nil},
		{"attributes alone are not body", []string{"London", `<em class="Body text."></em>`}, nil},
		{"quoted angle alone is not body", []string{"London", `<em title=">Body text."></em>`}, nil},
		{"single quoted angle alone is not body", []string{"London", `<em title='>Body text.'></em>`}, nil},
		{"unterminated attribute is not body", []string{"London", `<em title="Body text.>`}, nil},
		{"unterminated marker is not body", []string{"London", `<span Body text.`}, nil},
		{"stray angle alone is not body", []string{"London", "> \t"}, nil},
		{"body after quoted angle", []string{"London", `<em title=">">Body text.</em>`}, [][3]int{{4, 1, 7}}},
		{"body after escaped angle", []string{"London", `<span title=\>>Body text.</span>`}, [][3]int{{4, 1, 7}}},
		{"body after paired slashes", []string{"London", `<span title=\\>Body text.</span>`}, [][3]int{{4, 1, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v116TestRun(t, v116TestBlock(t, tc.rows...), "current", tc.want) })
	}
}

func TestV116RemainingMalformedHTML(t *testing.T) {
	for _, raw := range []string{`<em class="x">Body text.</em>`, `<span>Body text.</span>`, `<br/>Body text.`, `>Body text.`} {
		t.Run(raw, func(t *testing.T) {
			// Ошибка последующей строки не отменяет текст после маркера.
			row := v116TestLine(t, raw, "invalid", 3, 2, model.Element{
				ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{},
			})
			v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1),
				v116TestText(t, "London", "London", 3, 2), row, v116TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 7}})
		})
	}
	t.Run("parsed body after malformed marker", func(t *testing.T) {
		row := v116TestLine(t, `<em class="x">Body text.</em>`, "content", 3, 2,
			model.Element{ElementType: "unparsed", Raw: `<em class="x">`, Start: 1, End: 15, ErrorIDs: []string{}},
			v116TestElement(t, "content", `Body text.</em>`, `Body text.</em>`, 15, 30))
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1),
			v116TestText(t, "London", "London", 3, 2), row, v116TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 7}})
	})
}

func TestV116UppercaseSeries(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
		want [][3]int
	}{
		{"two uppercase rows", []string{"LONDON", "PARIS"}, nil},
		{"three uppercase rows", []string{"LONDON", "PARIS", "ROME", "More text."}, nil},
		{"blank is skipped", []string{"LONDON", "", " \t", "PARIS", "More text."}, nil},
		{"formatting is transparent", []string{"<em>LONDON</em>", "<strong>PARIS</strong>", "More text."}, nil},
		{"br breaks series", []string{"LONDON", "<br>", "PARIS"}, [][3]int{{4, 1, 7}}},
		{"mixed first row", []string{"London", "PARIS"}, [][3]int{{4, 1, 7}}},
		{"mixed second row", []string{"LONDON", "Paris"}, [][3]int{{4, 1, 7}}},
		{"number is not uppercase", []string{"LONDON", "123"}, [][3]int{{4, 1, 7}}},
		{"no case is not uppercase", []string{"LONDON", "東京"}, [][3]int{{4, 1, 7}}},
		{"third row cannot start excluded series", []string{"LONDON", "More text.", "PARIS", "ROME"}, [][3]int{{4, 1, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v116TestRun(t, v116TestBlock(t, tc.rows...), "current", tc.want) })
	}
}

// v116TestUnparsed создаёт участок с null Value; исходную ошибку добавляет TestDocument.
func v116TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v116TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

func TestV116TitleAndPartialParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opening model.Line
		want    [][3]int
	}{
		{"title", v116TestTitled(t, "text", "Reading", 2, 1), nil},
		{"Unicode title", v116TestTitled(t, "text", "Текст", 2, 1), nil},
		{"unparsed title", v116TestLine(t, "@text London {", "block-start", 2, 1,
			v116TestElement(t, "tag", "@text", "text", 1, 6), v116TestUnparsed(t, "London", 7, 13), v116TestElement(t, "block-open", "{", "{", 14, 15)), nil},
		{"unparsed NBSP title", v116TestLine(t, "@text \u00a0 {", "block-start", 2, 1,
			v116TestElement(t, "tag", "@text", "text", 1, 6), v116TestUnparsed(t, "\u00a0", 7, 8), v116TestElement(t, "block-open", "{", "{", 9, 10)), nil},
		{"tabs and spaces without title", v116TestLine(t, " \t@TeXt \t {", "block-start", 2, 1,
			v116TestElement(t, "tag", "@TeXt", "text", 3, 8), v116TestElement(t, "block-open", "{", "{", 11, 12)), [][3]int{{4, 1, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v116TestRun(t, v116TestDocument(t, tc.opening, v116TestText(t, "London", "London", 3, 2),
				v116TestText(t, "London is a large city.", "London is a large city.", 3, 2), v116TestClose(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("HTML crosses content elements", func(t *testing.T) {
		line := v116TestLine(t, "<em>London</em>", "content", 3, 2,
			v116TestElement(t, "content", "<em>Lon", "<em>Lon", 1, 8), v116TestElement(t, "content", "don</em>", "don</em>", 8, 16))
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), line,
			v116TestText(t, "More text.", "More text.", 3, 2), v116TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 16}})
	})
}

func TestV116NestedContentAndBarriers(t *testing.T) {
	t.Run("nested construction before candidate", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestOpen(t, "note", 3, 2),
			v116TestText(t, "Read.", "Read.", 4, 3), v116TestClose(t, 4, 3), v116TestText(t, "London", "London", 3, 2),
			v116TestText(t, "More text.", "More text.", 3, 2), v116TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested content does not supply remaining text", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestText(t, "London", "London", 3, 2),
			v116TestOpen(t, "note", 3, 2), v116TestText(t, "More text.", "More text.", 5, 3), v116TestClose(t, 5, 3),
			v116TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested construction breaks uppercase series", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestText(t, "LONDON", "LONDON", 3, 2),
			v116TestOpen(t, "note", 3, 2), v116TestText(t, "Read.", "Read.", 5, 3), v116TestClose(t, 5, 3),
			v116TestText(t, "PARIS", "PARIS", 3, 2), v116TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 7}})
	})
	t.Run("single line construction is a barrier", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestTag(t, "note", "content", "Read.", 3, 2),
			v116TestText(t, "London", "London", 3, 2), v116TestText(t, "More text.", "More text.", 3, 2), v116TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested text in fragment", func(t *testing.T) {
		fragment := v116TestLine(t, "@fragment read {", "block-start", 2, 1,
			v116TestElement(t, "tag", "@fragment", "fragment", 1, 10), v116TestElement(t, "identifier", "read", "read", 11, 15),
			v116TestElement(t, "block-open", "{", "{", 16, 17))
		v116TestRun(t, v116TestDocument(t, fragment, v116TestOpen(t, "text", 3, 2),
			v116TestText(t, "London", "London", 4, 3), v116TestText(t, "More text.", "More text.", 4, 3),
			v116TestClose(t, 4, 3), v116TestClose(t, 3, 2), v116TestTag(t, "include", "identifier", "read", 2, 1),
			v116TestTag(t, "include", "identifier", "read", 2, 1)), "current", [][3]int{{5, 1, 7}})
	})
	t.Run("nested text independently", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestOpen(t, "text", 3, 2),
			v116TestText(t, "London", "London", 4, 3), v116TestText(t, "More text.", "More text.", 4, 3),
			v116TestClose(t, 4, 3), v116TestClose(t, 3, 2)), "current", [][3]int{{5, 1, 7}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v116TestRun(t, v116TestDocument(t, v116TestOpen(t, tag, 2, 1), v116TestText(t, "@text {", "@text {", 3, 2),
				v116TestText(t, "London", "London", 3, 2), v116TestText(t, "More text.", "More text.", 3, 2), v116TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("text inside variants", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "variants", 2, 1), v116TestTag(t, "variant", "name", "A", 3, 2),
			v116TestOpen(t, "text", 4, 3), v116TestText(t, "London", "London", 5, 4), v116TestText(t, "More text.", "More text.", 5, 4),
			v116TestClose(t, 5, 4), v116TestTag(t, "variant", "name", "B", 3, 2), v116TestText(t, "Read.", "Read.", 9, 3),
			v116TestClose(t, 3, 2)), "current", [][3]int{{6, 1, 7}})
	})
	t.Run("other block is outside scope", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "note", 2, 1), v116TestText(t, "London", "London", 3, 2),
			v116TestText(t, "More text.", "More text.", 3, 2), v116TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV116EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestText(t, "London", "London", 3, 2),
				v116TestText(t, "More text.", "More text.", 3, 2), v116TestClose(t, 3, 2), v116TestOpen(t, "text", 2, 1),
				v116TestText(t, "Paris", "Paris", 7, 2), v116TestText(t, "More text.", "More text.", 7, 2), v116TestClose(t, 7, 2))
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v116TestRun(t, result, "first", [][3]int{{4, 1, 7}, {8, 1, 6}})
			v116TestRun(t, result, "second", [][3]int{{4, 1, 7}, {8, 1, 6}})
			v116TestRun(t, v116TestBlock(t, "London is a large city.", "More text."), "separate", nil)
		})
	}
	t.Run("remaining text at EOF", func(t *testing.T) {
		v116TestRun(t, v116TestDocument(t, v116TestOpen(t, "text", 2, 1), v116TestText(t, "London", "London", 3, 2),
			v116TestText(t, "More text.", "More text.", 3, 2)), "current", [][3]int{{4, 1, 7}})
	})
}

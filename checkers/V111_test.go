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

const v111TestMessage = "Команда пустой строки отличается от отдельной записи <br>."

// v111TestWant задаёт независимый диапазон с исключающей правой границей.
type v111TestWant struct {
	line, start, end int
	detail           string
}

func TestV111RegistrationAndVersions(t *testing.T) {
	registry := v111TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V111" {
			continue
		}
		count++
		if definition.Message != v111TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V111 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v111TestDocument(t)
			result.Document.DSLVersion = &version
			before := v111TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v111TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v111TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v111TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v111TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v111TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v111TestTag задаёт известное имя и явно указанный тип аргумента.
func v111TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v111TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v111TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v111TestLine(t, raw, "tag", parent, elements...)
}

// v111TestOpen добавляет открытие к известному объявлению блока.
func v111TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v111TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v111TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v111TestClose задаёт закрытие конкретного фигурного блока.
func v111TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v111TestLine(t, "}", "block-end", parent, v111TestElement(t, "block-close", "}", "}", 1))
}

// v111TestText сохраняет исходное и семантическое содержимое независимо.
func v111TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v111TestLine(t, raw, "content", parent, v111TestElement(t, "content", raw, value, 1))
}

// v111TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v111TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v111TestTag(t, "dsl-version", "1.2", "version", 0),
		v111TestLine(t, "# Lesson", "heading", 0, v111TestElement(t, "heading-level", "#", "1", 1),
			v111TestElement(t, "title", "Lesson", "Lesson", 3)),
		v111TestLine(t, "### Practice", "heading", 0, v111TestElement(t, "heading-level", "###", "3", 1),
			v111TestElement(t, "title", "Practice", "Practice", 5)),
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
				v111TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v111TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v111TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v111TestRegistry использует полный штатный каталог без отдельной регистрации.
func v111TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v111TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v111TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v111TestRun сравнивает новые V111 после штатных Prepare/Run с явным ожиданием.
func v111TestRun(t *testing.T, result *model.Result, processingID string, want []v111TestWant) {
	t.Helper()
	before := v111TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v111TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v111TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V111" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V111 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v111TestMessage+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V111 occurrence %d = %+v, want %+v with specified detail and no related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V111 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V111 created an element error link")
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
	if err := json.Unmarshal([]byte(v111TestSnapshot(t, result)), &restored); err != nil {
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
	if v111TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV111OwnsBrAttributes(t *testing.T) {
	// V111 и V114 работают одновременно, но атрибуты br проверяет только V111.
	for _, tc := range []struct {
		raw, detail string
		end         int
	}{
		{`<br class="gap">`, "атрибуты", 17},
		{`</BR class="gap">`, "закрывающая форма; атрибуты", 18},
		{`<br class="gap"/>`, "самозакрывающаяся форма; атрибуты", 18},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v111TestBlock(t, v111TestInvalid(t, tc.raw, 4))
			v111TestRun(t, result, "current", []v111TestWant{{5, 1, tc.end, tc.detail}})
			for _, entry := range result.Diagnostics {
				if entry.Source == "current" && entry.DiagnosticCode == "V114" {
					t.Fatalf("V114 duplicates br attribute check: %+v", entry)
				}
			}
		})
	}
}

func TestV111ReferenceAndExample(t *testing.T) {
	// Добавлены только версия и заголовки для контекста шага. HTML-сущности
	// таблицы задания обозначают исходные < и >; фрагменты сохранены дословно.
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           []v111TestWant
	}{
		{"reference", "@text {\nFirst paragraph.\n<BR>\nSecond paragraph.\n}",
			[]model.Line{v111TestText(t, "First paragraph.", "First paragraph.", 4),
				v111TestText(t, "<BR>", "<BR>", 4), v111TestText(t, "Second paragraph.", "Second paragraph.", 4)}, nil},
		{"example", "@text {\nFirst paragraph.<br>\n<br />\n</br>\n<br class=\"gap\">\nSecond paragraph.\n}",
			[]model.Line{v111TestInvalid(t, "First paragraph.<br>", 4), v111TestInvalid(t, "<br />", 4),
				v111TestInvalid(t, "</br>", 4), v111TestInvalid(t, `<br class="gap">`, 4),
				v111TestText(t, "Second paragraph.", "Second paragraph.", 4)},
			[]v111TestWant{{5, 1, 21, "другое содержимое строки"}, {6, 1, 7, "самозакрывающаяся форма"},
				{7, 1, 6, "закрывающая форма"}, {8, 1, 17, "атрибуты"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v111TestBlock(t, tc.rows...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v111TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV111ReasonsAndRanges(t *testing.T) {
	for _, tc := range []struct {
		raw, detail string
		start, end  int
	}{
		{"<br >", "лишние пробельные символы внутри маркера", 1, 6},
		{"<br\t>", "лишние пробельные символы внутри маркера", 1, 6},
		{"<br \t>", "лишние пробельные символы внутри маркера", 1, 7},
		{"</br >", "закрывающая форма; лишние пробельные символы внутри маркера", 1, 7},
		{"<br/>", "самозакрывающаяся форма", 1, 6},
		{"<br />", "самозакрывающаяся форма", 1, 7},
		{"</BR>", "закрывающая форма", 1, 6},
		{"</br/>", "закрывающая форма; самозакрывающаяся форма", 1, 7},
		{"x</br/>", "закрывающая форма; самозакрывающаяся форма; другое содержимое строки", 1, 8},
		{`<br class="gap">`, "атрибуты", 1, 17},
		{`<br title=">">`, "атрибуты", 1, 15},
		{`<br title='>'>`, "атрибуты", 1, 15},
		{`<br class=gap>`, "атрибуты", 1, 15},
		{`<br hidden>`, "атрибуты", 1, 12},
		{`<br class="gap"/>`, "самозакрывающаяся форма; атрибуты", 1, 18},
		{`</br class="gap"/>`, "закрывающая форма; самозакрывающаяся форма; атрибуты", 1, 19},
		{"<br", "незавершённый маркер", 1, 4},
		{"</br", "закрывающая форма; незавершённый маркер", 1, 5},
		{`<br class="gap`, "атрибуты; незавершённый маркер", 1, 15},
		{`<br title='>`, "атрибуты; незавершённый маркер", 1, 13},
		{"<br><BR>", "другое содержимое строки", 1, 9},
		{"<br/><br/>", "самозакрывающаяся форма; другое содержимое строки", 1, 11},
		{"<strong><br></strong>", "другое содержимое строки", 1, 22},
		{"<br>Text", "другое содержимое строки", 1, 9},
		{"Text<br>", "другое содержимое строки", 1, 9},
		{" \t<br /> \t", "самозакрывающаяся форма", 3, 9},
		{"\u00a0<br>", "другое содержимое строки", 1, 6},
		{"<br>\u00a0", "другое содержимое строки", 1, 6},
		{" \t😀<br>\t ", "другое содержимое строки", 3, 8},
		{"e\u0301<br>", "другое содержимое строки", 1, 7},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestInvalid(t, tc.raw, 4)), "current",
				[]v111TestWant{{5, tc.start, tc.end, tc.detail}})
		})
	}
}

func TestV111AllowedLinesAndNameBoundaries(t *testing.T) {
	for _, raw := range []string{"<br>", "<BR>", "<Br>", "<bR>", " \t<BR>\t ", "Read.",
		"<strong>Read.</strong>", "<em><u>Read.</u></em>", "&lt;br&gt;"} {
		t.Run(raw, func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestText(t, raw, raw, 4)), "current", nil)
		})
	}
	for _, raw := range []string{"<bright>", "</bright>", "<bravo>", "<br-extra>", "<br_>", "<br2>",
		"<br\u200b>", "<br\ufeff>", "<b", "<", "</", "< br>", "</ br>", "<BRIGHT", "<br\\>"} {
		t.Run(raw, func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestInvalid(t, raw, 4)), "current", nil)
		})
	}
	// White_Space задан явно; в отличие от крайних U+0020/U+0009 эти
	// символы не удаляются с краёв строки и не являются текстом атрибута.
	for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001",
		"\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a",
		"\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run(strconv.Quote(space), func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestInvalid(t, "<br"+space+">", 4)), "current",
				[]v111TestWant{{5, 1, 6, "лишние пробельные символы внутри маркера"}})
		})
	}
}

func TestV111WholeMarkersAndEscapes(t *testing.T) {
	for _, raw := range []string{`<span title="<br>">`, `<span title='<br>'>`, `<span title="</br><br/>">`,
		`<span title="<br>`, `<span <br>>`, `<bright title="<br/>">`} {
		t.Run("opaque "+raw, func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestInvalid(t, raw, 4)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\<br\>`, "<br>"}, {`\<br>`, "<br>"}, {`\\\<br>`, `\<br>`},
		{`\<br/>`, "<br/>"}, {`\</br>`, "</br>"},
	} {
		t.Run("literal "+tc.raw, func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestText(t, tc.raw, tc.value, 4)), "current", nil)
		})
	}
	for _, tc := range []struct {
		raw, detail string
		end         int
	}{
		{`\\<br>`, "другое содержимое строки", 7},
		{`\\\\<br>`, "другое содержимое строки", 9},
		{`\q<br>`, "другое содержимое строки", 7},
		{`<br title="<br>">`, "атрибуты", 18},
		{`<br title='</br>'>`, "атрибуты", 19},
		{`<br title="\">`, "атрибуты", 15},
		{`<br title="\>">`, "атрибуты", 16},
		{`<br title="'>'">`, "атрибуты", 17},
		{`<br \>`, "атрибуты; незавершённый маркер", 7},
		{`<br \> x>`, "атрибуты", 10},
		{`<br \\>`, "атрибуты", 8},
		{`<em title="<br>"><br/>`, "самозакрывающаяся форма; другое содержимое строки", 23},
		{`<br><em title="<br/>">`, "другое содержимое строки", 23},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, v111TestInvalid(t, tc.raw, 4)), "current",
				[]v111TestWant{{5, 1, tc.end, tc.detail}})
		})
	}
}

func TestV111TextRangesAndErrorFlags(t *testing.T) {
	for _, row := range []model.Line{
		v111TestInvalid(t, "Read.<br/>", 4),
		v111TestLine(t, "Read.<br/>", "invalid", 4, v111TestElement(t, "content", "Read.", "Read.", 1),
			v111TestElement(t, "unparsed", "<br/>", "", 6)),
		v111TestLine(t, "Read.<br/>", "content", 4, v111TestElement(t, "content", "Read.", "Read.", 1),
			v111TestElement(t, "unparsed", "<br/>", "", 6)),
		v111TestLine(t, "Read.<br/>", "content", 4, v111TestElement(t, "content", "Read.<b", "Read.<b", 1),
			v111TestElement(t, "unparsed", "r/>", "", 8)),
	} {
		t.Run(string(row.LineType)+"/"+strconv.Itoa(len(row.Elements))+"/"+row.Elements[0].Raw, func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, row), "current",
				[]v111TestWant{{5, 1, 11, "самозакрывающаяся форма; другое содержимое строки"}})
		})
	}
	// Эти входы встречаются в старых тестах V012/V017. Ошибка br должна
	// выставить флаг строки, даже если прежний обработчик считал её content.
	for _, tc := range []struct {
		raw string
		end int
	}{{"1-<br>12", 9}, {"Hello <br>,", 12}} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v111TestBlock(t, v111TestText(t, tc.raw, tc.raw, 4))
			if result.Lines[4].HasErrors {
				t.Fatal("fixture already has an error flag")
			}
			v111TestRun(t, result, "current", []v111TestWant{{5, 1, tc.end, "другое содержимое строки"}})
		})
	}
}

func TestV111OwnContentAndNestedContexts(t *testing.T) {
	t.Run("nested text", func(t *testing.T) {
		v111TestRun(t, v111TestDocument(t, v111TestOpen(t, "fragment", "intro", "identifier", 3),
			v111TestOpen(t, "TeXt", "", "", 4), v111TestInvalid(t, "<br/>", 5),
			v111TestText(t, "Read.", "Read.", 5), v111TestClose(t, 5), v111TestClose(t, 4)), "current",
			[]v111TestWant{{6, 1, 6, "самозакрывающаяся форма"}})
	})
	t.Run("title is excluded", func(t *testing.T) {
		v111TestRun(t, v111TestDocument(t, v111TestOpen(t, "text", "Title <br/>", "title", 3),
			v111TestText(t, "Read.", "Read.", 4), v111TestClose(t, 4)), "current", nil)
	})
	t.Run("nested construction is excluded", func(t *testing.T) {
		result := v111TestDocument(t, v111TestOpen(t, "text", "", "", 3),
			v111TestOpen(t, "note", "Title <br/>", "title", 4), v111TestText(t, "Read.<br>", "Read.<br>", 5),
			v111TestClose(t, 5), v111TestInvalid(t, "<br/>", 4), v111TestClose(t, 4))
		v111TestParserError(t, result, 5, 0, "Вложение note запрещено внутри text.")
		v111TestRun(t, result, "current", []v111TestWant{{8, 1, 6, "самозакрывающаяся форма"}})
	})
	for _, row := range []model.Line{
		v111TestInvalid(t, "@note Title <br/>", 4),
		v111TestLine(t, "@note Title <br/>", "invalid", 4, v111TestElement(t, "tag", "@note", "note", 1),
			v111TestElement(t, "unparsed", "Title <br/>", "", 7)),
		v111TestInvalid(t, "# Title <br/>", 4),
	} {
		t.Run("structural "+row.Raw+strconv.Itoa(len(row.Elements)), func(t *testing.T) {
			v111TestRun(t, v111TestBlock(t, row), "current", nil)
		})
	}
	for _, name := range []string{"example", "script", "note", "editor", "multifill"} {
		t.Run("other context "+name, func(t *testing.T) {
			v111TestRun(t, v111TestDocument(t, v111TestTag(t, "task", "practice", "identifier", 3),
				v111TestOpen(t, name, "", "", 4), v111TestText(t, "<br/>", "<br/>", 5), v111TestClose(t, 5)), "current", nil)
		})
	}
	v111TestRun(t, v111TestDocument(t, v111TestText(t, "Read.<br>", "Read.<br>", 3)), "outside", nil)
}

func TestV111BoundariesEndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v111TestBlock(t, v111TestInvalid(t, "<br/>", 4), v111TestLine(t, " \t", "blank", 4),
				v111TestInvalid(t, "<br/>", 4))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v111TestWant{{5, 1, 6, "самозакрывающаяся форма"}, {7, 1, 6, "самозакрывающаяся форма"}}
			v111TestRun(t, result, "first", want)
			v111TestRun(t, v111TestBlock(t, v111TestText(t, "<br>", "<br>", 4)), "other", nil)
			v111TestRun(t, result, "second", want)
		})
	}
	t.Run("last line has no EOL or closing", func(t *testing.T) {
		result := v111TestDocument(t, v111TestOpen(t, "text", "", "", 3), v111TestInvalid(t, "<br/>", 4))
		result.Lines[4].LineEnding = ""
		v111TestRun(t, result, "current", []v111TestWant{{5, 1, 6, "самозакрывающаяся форма"}})
	})
	t.Run("separate blocks and following text", func(t *testing.T) {
		v111TestRun(t, v111TestDocument(t, v111TestOpen(t, "text", "", "", 3), v111TestInvalid(t, "<br/>", 4),
			v111TestClose(t, 4), v111TestText(t, "<br/>", "<br/>", 3),
			v111TestOpen(t, "text", "", "", 3), v111TestInvalid(t, "</br>", 8), v111TestClose(t, 8)), "current",
			[]v111TestWant{{5, 1, 6, "самозакрывающаяся форма"}, {9, 1, 6, "закрывающая форма"}})
	})
	t.Run("empty text block", func(t *testing.T) { v111TestRun(t, v111TestBlock(t), "current", nil) })
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v111TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v111TestRun(t, result, "current", nil)
		})
	}
}

// v111TestInvalid задаёт ошибочный текст с исходной диагностикой для unparsed.
func v111TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v111TestLine(t, raw, "invalid", parent, v111TestElement(t, "unparsed", raw, "", 1))
}

// v111TestBlock помещает явно заданные строки внутрь text на уровне шага.
func v111TestBlock(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v111TestOpen(t, "text", "", "", 3)}, rows...)
	return v111TestDocument(t, append(body, v111TestClose(t, 4))...)
}

func TestV111PlaceholderContent(t *testing.T) {
	t.Run("escaped marker", func(t *testing.T) {
		gap := v111TestElement(t, "placeholder", "_____", "", 1)
		gap.Value = nil
		row := v111TestLine(t, `_____\<br>`, "content", 4, gap,
			v111TestElement(t, "content", `\<br>`, "<br>", 6))
		v111TestRun(t, v111TestBlock(t, row), "current", nil)
	})
	for _, tc := range []struct {
		name, prefix, suffix string
		want                 []v111TestWant
	}{
		{"after placeholder", "ONE ", "<br>", []v111TestWant{{5, 1, 14, "другое содержимое строки"}}},
		{"before placeholder", "<br>", "", []v111TestWant{{5, 1, 10, "другое содержимое строки"}}},
		{"both sides", "<br>", "<br>", []v111TestWant{{5, 1, 14, "другое содержимое строки"}}},
		{"unicode prefix", "Я😀 ", "<br/>", []v111TestWant{{5, 1, 14, "самозакрывающаяся форма; другое содержимое строки"}}},
		{"valid formatting", "<em>", "</em>", nil},
	} {
		for _, kind := range []model.ElementType{"content", "unparsed"} {
			t.Run(tc.name+"/"+string(kind), func(t *testing.T) {
				var elements []model.Element
				if tc.prefix != "" {
					elements = append(elements, v111TestElement(t, kind, tc.prefix, tc.prefix, 1))
				}
				start := utf8.RuneCountInString(tc.prefix) + 1
				gap := v111TestElement(t, "placeholder", "_____", "", start)
				gap.Value = nil
				elements = append(elements, gap)
				if tc.suffix != "" {
					elements = append(elements, v111TestElement(t, kind, tc.suffix, tc.suffix, start+5))
				}
				row := v111TestLine(t, tc.prefix+"_____"+tc.suffix, "content", 4, elements...)
				v111TestRun(t, v111TestBlock(t, row), "current", tc.want)
			})
		}
	}
}

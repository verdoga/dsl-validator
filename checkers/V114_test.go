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

const v114TestMessage = "В теге выделения записаны атрибуты."

// v114TestWant задаёт независимый диапазон с исключающей правой границей.
type v114TestWant struct{ line, start, end int }

func TestV114RegistrationAndVersions(t *testing.T) {
	registry := v114TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V114" {
			continue
		}
		count++
		if definition.Message != v114TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V114 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v114TestDocument(t)
			result.Document.DSLVersion = &version
			before := v114TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v114TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v114TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v114TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v114TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v114TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v114TestTag задаёт известное имя и явно указанный тип аргумента.
func v114TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v114TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v114TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v114TestLine(t, raw, "tag", parent, elements...)
}

// v114TestOpen добавляет открытие к известному объявлению блока.
func v114TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v114TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v114TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v114TestClose задаёт закрытие конкретного фигурного блока.
func v114TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v114TestLine(t, "}", "block-end", parent, v114TestElement(t, "block-close", "}", "}", 1))
}

// v114TestText сохраняет исходное и семантическое содержимое независимо.
func v114TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v114TestLine(t, raw, "content", parent, v114TestElement(t, "content", raw, value, 1))
}

// v114TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v114TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v114TestTag(t, "dsl-version", "1.2", "version", 0),
		v114TestLine(t, "# Lesson", "heading", 0, v114TestElement(t, "heading-level", "#", "1", 1),
			v114TestElement(t, "title", "Lesson", "Lesson", 3)),
		v114TestLine(t, "### Practice", "heading", 0, v114TestElement(t, "heading-level", "###", "3", 1),
			v114TestElement(t, "title", "Practice", "Practice", 5)),
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
				v114TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v114TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v114TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v114TestRegistry использует полный штатный каталог без отдельной регистрации.
func v114TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v114TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v114TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v114TestRun сравнивает новые V114 после штатных Prepare/Run с явным ожиданием.
func v114TestRun(t *testing.T, result *model.Result, processingID string, want []v114TestWant) {
	t.Helper()
	before := v114TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v114TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v114TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V114" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V114 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v114TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V114 occurrence %d = %+v, want %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V114 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V114 created an element error link")
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
	if err := json.Unmarshal([]byte(v114TestSnapshot(t, result)), &restored); err != nil {
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
	if v114TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV114ReferenceAndExample(t *testing.T) {
	// Версия и заголовки задают контекст шага. Фрагменты сохранены дословно;
	// &lt; и &gt; в задании являются разметкой таблицы, а не символами DSL.
	for _, tc := range []struct {
		name, fragment, row string
		want                []v114TestWant
	}{
		{"reference", "@text {\n<strong>Read this carefully.</strong>\n}",
			"<strong>Read this carefully.</strong>", nil},
		{"example", "@text {\n<strong class=\"important\" lang=\"en\">Read this carefully.</strong>\n}",
			`<strong class="important" lang="en">Read this carefully.</strong>`, []v114TestWant{{5, 9, 36}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := v114TestText(t, tc.row, tc.row, 4)
			if tc.name == "example" {
				row = v114TestInvalid(t, tc.row, 4)
			}
			result := v114TestBlock(t, row)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v114TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV114SelectedMarkersAndRanges(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		start, end int
	}{
		{"<strong x>Read.</strong>", 9, 10}, {"<em x>Read.</em>", 5, 6}, {"<u x>Read.</u>", 4, 5},
		{"<strong>Read.</strong x>", 23, 24}, {"<em>Read.</em x>", 15, 16}, {"<u>Read.</u x>", 13, 14},
		{"<STRONG x>Read.</STRONG>", 9, 10}, {"<EM x>Read.</EM>", 5, 6}, {"<U x>Read.</U>", 4, 5},
		{"<StRoNg x>Read.</strong>", 9, 10}, {"<em>Read.</eM x>", 15, 16},
		{"<u hidden>Read.</u>", 4, 10}, {"<em a=1 b=2>Read.</em>", 5, 12},
		{"<em a=1  b=2>Read.</em>", 5, 13}, {"<em a=1\tb=2>Read.</em>", 5, 12},
		{"<em  x \t>Read.</em>", 6, 7}, {"<em\u00a0x\u00a0>Read.</em>", 5, 6},
		{"<em x/>", 5, 6}, {"<em  x \t/>", 6, 7}, {"</em x/>", 6, 7},
		{`<em a="/">Read.</em>`, 5, 10}, {`<em a="  ">Read.</em>`, 5, 11},
		{`<em title=">">Read.</em>`, 5, 14}, {`<em title='>'>Read.</em>`, 5, 14},
		{`<em title='<u x>'>Read.</em>`, 5, 18}, {`<em title="<u x>">Read.</em>`, 5, 18},
		{"<em x>", 5, 6}, {"</u x>", 5, 6},
		{"😀<em x>Read.</em>", 6, 7}, {"e\u0301<em x>Read.</em>", 7, 8},
		{"\t 😀<em x>Read.</em> \t", 8, 9}, {`<em a="я😀">Read.</em>`, 5, 11},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, tc.raw, 4)), "current",
				[]v114TestWant{{5, tc.start, tc.end}})
		})
	}
	for _, space := range []string{" ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001",
		"\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a",
		"\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run("White_Space "+strconv.Quote(space), func(t *testing.T) {
			// Ожидаемые координаты заданы явно для одного символа с каждой стороны.
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, "<em"+space+"x"+space+">Read.</em>", 4)),
				"current", []v114TestWant{{5, 5, 6}})
		})
	}
}

func TestV114NoAttributesAndOtherNames(t *testing.T) {
	for _, raw := range []string{"Read.", "<strong>Read.</strong>", "<em>Read.</em>", "<u>Read.</u>",
		"<strong><em><u>Read.</u></em></strong>", "<em \t>Read.</em>", "<u>Read.</u \t>",
		"&lt;em x&gt;Read.&lt;/em&gt;"} {
		t.Run(raw, func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestText(t, raw, raw, 4)), "current", nil)
		})
	}
	for _, raw := range []string{"<em/>", "<em />", "</em/>", "<strong />", "<u/>",
		"<stronger x>", "</stronger x>", "<embed x>", "<ul x>", "<span x>", "<em-x a>",
		"<em2 x>", "<em=x>", "<em\u200bx>", "<em\ufeffx>", "<EM_EXTRA x>",
		"<em>", "</u>", "<strong><em></strong></em>", "< em x>", "</ em x>"} {
		t.Run(raw, func(t *testing.T) {
			// Ошибки имени, парности и самозакрытия не входят в V114.
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, raw, 4)), "current", nil)
		})
	}
}

func TestV114WholeMarkersAndEscapes(t *testing.T) {
	for _, raw := range []string{`<span title="<em x>">`, `<span title='<strong x>'>`,
		`<br title="<u x>">`, `<span title="<em x>`, `<span <em x>>`} {
		t.Run("opaque "+raw, func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, raw, 4)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\<em x\>`, "<em x>"}, {`\<em x>`, "<em x>"},
		{`\\\<em x>`, `\<em x>`}, {`\</em x\>`, "</em x>"},
	} {
		t.Run("literal "+tc.raw, func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestText(t, tc.raw, tc.value, 4)), "current", nil)
		})
	}
	for _, tc := range []struct {
		raw        string
		start, end int
	}{
		{`\\<em x>`, 7, 8}, {`\\\\<em x>`, 9, 10}, {`\q<em x>`, 7, 8},
		{`<em a="\">Read.</em>`, 5, 10}, {`<em a="\>">Read.</em>`, 5, 11},
		{`<em a="'>'">Read.</em>`, 5, 12}, {`<em a='" >'>Read.</em>`, 5, 12},
		{`<em x\>y>Read.</em>`, 5, 9}, {`<em x\\>Read.</em>`, 5, 8},
		{`<em x\{y\}>Read.</em>`, 5, 11},
		{`<em a="&gt;">Read.</em>`, 5, 13},
		{`<span x><em y>Read.</em>`, 13, 14},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, tc.raw, 4)), "current",
				[]v114TestWant{{5, tc.start, tc.end}})
		})
	}
}

func TestV114UnfinishedAndMultipleMarkers(t *testing.T) {
	for _, raw := range []string{"<em x", "</strong x", "<em title=\">", "<u a='>", "<u x\\>", "<", "</", "<u"} {
		t.Run(raw, func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, raw, 4)), "current", nil)
		})
	}
	t.Run("one occurrence per marker", func(t *testing.T) {
		v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, "<em x><u y>Read.</u></em>", 4)), "current",
			[]v114TestWant{{5, 5, 6}, {5, 10, 11}})
	})
	t.Run("opening and closing attributes", func(t *testing.T) {
		v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, "<strong><em x>Read.</em y></strong>", 4)), "current",
			[]v114TestWant{{5, 13, 14}, {5, 25, 26}})
	})
	t.Run("incomplete marker does not cross physical line", func(t *testing.T) {
		v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, "<em title=\"", 4),
			v114TestInvalid(t, "<u x>Read.</u>", 4)), "current", []v114TestWant{{6, 4, 5}})
	})
	t.Run("complete marker before incomplete", func(t *testing.T) {
		v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, "<em x>Read.</em><u y", 4)), "current",
			[]v114TestWant{{5, 5, 6}})
	})
}

func TestV114PartialParsingAndSourceFlags(t *testing.T) {
	for _, row := range []model.Line{
		v114TestInvalid(t, "<em x>Read.</em>", 4),
		v114TestLine(t, "<em x>Read.</em>", "invalid", 4,
			v114TestElement(t, "unparsed", "<em x>", "", 1), v114TestElement(t, "content", "Read.</em>", "Read.</em>", 7)),
		v114TestLine(t, "<em x>Read.</em>", "content", 4,
			v114TestElement(t, "unparsed", "<em x>", "", 1), v114TestElement(t, "content", "Read.</em>", "Read.</em>", 7)),
	} {
		t.Run(string(row.LineType)+strconv.Itoa(len(row.Elements)), func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, row), "current", []v114TestWant{{5, 5, 6}})
		})
	}
	// Старые тесты V012/V017 используют content без исходной ошибки парсера.
	// Запрет атрибутов всё равно действует, и конвейер должен выставить HasErrors.
	for _, tc := range []struct {
		raw        string
		start, end int
	}{
		{"1-<em x=y>12</em>", 7, 10}, {"Hello <em x=y>,</em>", 11, 14},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v114TestBlock(t, v114TestText(t, tc.raw, tc.raw, 4))
			if result.Lines[4].HasErrors {
				t.Fatal("fixture already has an error flag")
			}
			v114TestRun(t, result, "current", []v114TestWant{{5, tc.start, tc.end}})
		})
	}
}

func TestV114OwnContentAndNestedContexts(t *testing.T) {
	t.Run("text at nested depth", func(t *testing.T) {
		v114TestRun(t, v114TestDocument(t, v114TestOpen(t, "fragment", "intro", "identifier", 3),
			v114TestOpen(t, "TeXt", "", "", 4), v114TestInvalid(t, "<em x>Read.</em>", 5),
			v114TestClose(t, 5), v114TestClose(t, 4)), "current", []v114TestWant{{6, 5, 6}})
	})
	t.Run("title is not content", func(t *testing.T) {
		v114TestRun(t, v114TestDocument(t, v114TestOpen(t, "text", "<em x>Title</em>", "title", 3),
			v114TestText(t, "Read.", "Read.", 4), v114TestClose(t, 4)), "current", nil)
	})
	t.Run("nested construction is excluded", func(t *testing.T) {
		result := v114TestDocument(t, v114TestOpen(t, "text", "", "", 3),
			v114TestOpen(t, "note", "<em x>Title</em>", "title", 4),
			v114TestText(t, "<u x>Read.</u>", "<u x>Read.</u>", 5), v114TestClose(t, 5),
			v114TestInvalid(t, "<em y>Write.</em>", 4), v114TestClose(t, 4))
		v114TestParserError(t, result, 5, 0, "Вложение note запрещено внутри text.")
		v114TestRun(t, result, "current", []v114TestWant{{8, 5, 6}})
	})
	for _, row := range []model.Line{
		v114TestInvalid(t, "@note <em x>Title</em>", 4),
		v114TestLine(t, "@note <em x>Title</em>", "invalid", 4, v114TestElement(t, "tag", "@note", "note", 1),
			v114TestElement(t, "unparsed", "<em x>Title</em>", "", 7)),
		v114TestInvalid(t, "# <em x>Title</em>", 4),
	} {
		t.Run("structural "+row.Raw+strconv.Itoa(len(row.Elements)), func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, row), "current", nil)
		})
	}
	for _, name := range []string{"example", "table", "script", "key", "note", "editor", "multifill"} {
		t.Run("other context "+name, func(t *testing.T) {
			v114TestRun(t, v114TestDocument(t, v114TestTag(t, "task", "practice", "identifier", 3),
				v114TestOpen(t, name, "", "", 4), v114TestText(t, "<em x>Read.</em>", "<em x>Read.</em>", 5),
				v114TestClose(t, 5)), "current", nil)
		})
	}
	v114TestRun(t, v114TestDocument(t, v114TestText(t, "<em x>Read.</em>", "<em x>Read.</em>", 3)), "outside", nil)
}

func TestV114DoesNotOwnBrAttributes(t *testing.T) {
	for _, raw := range []string{`<br class="gap">`, `</BR class="gap">`, `<br class="gap"/>`} {
		t.Run(raw, func(t *testing.T) {
			v114TestRun(t, v114TestBlock(t, v114TestInvalid(t, raw, 4)), "current", nil)
		})
	}
	t.Run("both diagnostics on one line", func(t *testing.T) {
		result := v114TestBlock(t, v114TestInvalid(t, "<br x><em y>Read.</em>", 4))
		v114TestRun(t, result, "current", []v114TestWant{{5, 11, 12}})
		var br []model.Diagnostic
		for _, entry := range result.Diagnostics {
			if entry.Source == "current" && entry.DiagnosticCode == "V111" {
				br = append(br, entry)
			}
		}
		location := model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 23}}
		if len(br) != 1 || br[0].Location == nil || *br[0].Location != location ||
			br[0].Message != "Команда пустой строки отличается от отдельной записи <br>.\nконкретизация: атрибуты; другое содержимое строки" ||
			br[0].SeverityLevel != "error" || br[0].DiagnosticScope != "line" || len(br[0].RelatedLocations) != 0 {
			t.Fatalf("unexpected accompanying V111: %+v", br)
		}
	})
}

func TestV114BoundariesEndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v114TestBlock(t, v114TestInvalid(t, "<em x>Read.</em>", 4),
				v114TestLine(t, " \t", "blank", 4), v114TestInvalid(t, "<u y>Write.</u>", 4))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v114TestWant{{5, 5, 6}, {7, 4, 5}}
			v114TestRun(t, result, "first", want)
			v114TestRun(t, v114TestBlock(t, v114TestText(t, "<em>Read.</em>", "<em>Read.</em>", 4)), "other", nil)
			v114TestRun(t, result, "second", want)
		})
	}
	t.Run("last line without EOL or closing", func(t *testing.T) {
		result := v114TestDocument(t, v114TestOpen(t, "text", "", "", 3), v114TestInvalid(t, "<em x>Read.</em>", 4))
		result.Lines[4].LineEnding = ""
		v114TestRun(t, result, "current", []v114TestWant{{5, 5, 6}})
	})
	t.Run("separate blocks and outside text", func(t *testing.T) {
		v114TestRun(t, v114TestDocument(t, v114TestOpen(t, "text", "", "", 3), v114TestInvalid(t, "<em x>Read.</em>", 4),
			v114TestClose(t, 4), v114TestText(t, "<em x>Read.</em>", "<em x>Read.</em>", 3),
			v114TestOpen(t, "text", "", "", 3), v114TestInvalid(t, "<u y>Write.</u>", 8), v114TestClose(t, 8)),
			"current", []v114TestWant{{5, 5, 6}, {9, 4, 5}})
	})
	t.Run("empty text block", func(t *testing.T) { v114TestRun(t, v114TestBlock(t), "current", nil) })
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v114TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v114TestRun(t, result, "current", nil)
		})
	}
}

// v114TestInvalid задаёт ошибочную строку с исходной диагностикой для unparsed.
func v114TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v114TestLine(t, raw, "invalid", parent, v114TestElement(t, "unparsed", raw, "", 1))
}

// v114TestBlock помещает явно заданные строки непосредственно в text внутри шага.
func v114TestBlock(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v114TestOpen(t, "text", "", "", 3)}, rows...)
	return v114TestDocument(t, append(body, v114TestClose(t, 4))...)
}

func TestV114PlaceholderContent(t *testing.T) {
	t.Run("escaped markers", func(t *testing.T) {
		gap := v114TestElement(t, "placeholder", "_____", "", 1)
		gap.Value = nil
		row := v114TestLine(t, `_____\<em x>TEXT\</em>`, "content", 4, gap,
			v114TestElement(t, "content", `\<em x>TEXT\</em>`, "<em x>TEXT</em>", 6))
		v114TestRun(t, v114TestBlock(t, row), "current", nil)
	})
	for _, tc := range []struct {
		name, prefix, suffix string
		want                 []v114TestWant
	}{
		{"after placeholder", "ONE ", "<em x>TEXT</em>", []v114TestWant{{5, 14, 15}}},
		{"before placeholder", "<em x>", "TEXT</em>", []v114TestWant{{5, 5, 6}}},
		{"both sides", "<em x>", "</em y>", []v114TestWant{{5, 5, 6}, {5, 17, 18}}},
		{"unicode prefix", "Я😀 ", "<u x>TEXT</u>", []v114TestWant{{5, 12, 13}}},
		{"valid formatting", "<em>", "</em>", nil},
	} {
		for _, kind := range []model.ElementType{"content", "unparsed"} {
			t.Run(tc.name+"/"+string(kind), func(t *testing.T) {
				var elements []model.Element
				if tc.prefix != "" {
					elements = append(elements, v114TestElement(t, kind, tc.prefix, tc.prefix, 1))
				}
				start := utf8.RuneCountInString(tc.prefix) + 1
				gap := v114TestElement(t, "placeholder", "_____", "", start)
				gap.Value = nil
				elements = append(elements, gap)
				if tc.suffix != "" {
					elements = append(elements, v114TestElement(t, kind, tc.suffix, tc.suffix, start+5))
				}
				row := v114TestLine(t, tc.prefix+"_____"+tc.suffix, "content", 4, elements...)
				v114TestRun(t, v114TestBlock(t, row), "current", tc.want)
			})
		}
	}
}

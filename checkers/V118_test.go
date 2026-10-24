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

const v118TestMessage = "Несколько соседних строк написаны прописными буквами."
const v118TestDetail = "Проверьте, не разбит ли один заголовок на несколько строк. При подтверждении объедините его; для заголовка всего текста используйте TITLE"

// v118TestWant задаёт независимый диапазон с исключающей правой границей.
type v118TestWant struct{ startLine, start, endLine, end int }

func TestV118RegistrationAndVersions(t *testing.T) {
	registry := v118TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V118" {
			continue
		}
		count++
		if definition.Message != v118TestMessage || definition.Severity != "recommendation" ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V118 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v118TestDocument(t)
			result.Document.DSLVersion = &version
			before := v118TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v118TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v118TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v118TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v118TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v118TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v118TestTag задаёт известное имя и явно указанный тип аргумента.
func v118TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v118TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v118TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v118TestLine(t, raw, "tag", parent, elements...)
}

// v118TestOpen добавляет открытие к известному объявлению блока.
func v118TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v118TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v118TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v118TestClose задаёт закрытие конкретного фигурного блока.
func v118TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v118TestLine(t, "}", "block-end", parent, v118TestElement(t, "block-close", "}", "}", 1))
}

// v118TestText сохраняет исходное и семантическое содержимое независимо.
func v118TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v118TestLine(t, raw, "content", parent, v118TestElement(t, "content", raw, value, 1))
}

// v118TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v118TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v118TestTag(t, "dsl-version", "1.2", "version", 0),
		v118TestLine(t, "# Lesson", "heading", 0, v118TestElement(t, "heading-level", "#", "1", 1),
			v118TestElement(t, "title", "Lesson", "Lesson", 3)),
		v118TestLine(t, "### Practice", "heading", 0, v118TestElement(t, "heading-level", "###", "3", 1),
			v118TestElement(t, "title", "Practice", "Practice", 5)),
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
				v118TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v118TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v118TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v118TestRegistry использует полный штатный каталог без отдельной регистрации.
func v118TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v118TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v118TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v118TestRun сравнивает новые V118 после штатных Prepare/Run с явным ожиданием.
func v118TestRun(t *testing.T, result *model.Result, processingID string, want []v118TestWant) {
	t.Helper()
	before := v118TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v118TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v118TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V118" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V118 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.startLine, Column: expected.start},
			End: model.Position{Line: expected.endLine, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v118TestMessage+"\nконкретизация: "+v118TestDetail ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V118 occurrence %d = %+v, want %+v, exact detail and no related", i, entry, location)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("V118 recommendation created an element error link")
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
	if err := json.Unmarshal([]byte(v118TestSnapshot(t, result)), &restored); err != nil {
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
	if v118TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v118TestBlock помещает явно заданные строки в text на строке 4.
func v118TestBlock(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v118TestOpen(t, "text", "", "title", 3)}, rows...)
	return v118TestDocument(t, append(body, v118TestClose(t, 4))...)
}

// v118TestRows задаёт обычный текст без экранирования; пустые строки имеют тип blank.
// Для экранирования и ошибочных элементов тесты задают модель отдельно.
func v118TestRows(t *testing.T, raw ...string) *model.Result {
	t.Helper()
	var rows []model.Line
	for _, text := range raw {
		value := strings.Trim(text, " \t")
		if value == "" {
			rows = append(rows, v118TestLine(t, text, "blank", 4))
		} else {
			rows = append(rows, v118TestText(t, text, value, 4))
		}
	}
	return v118TestBlock(t, rows...)
}

func TestV118ReferenceAndExample(t *testing.T) {
	// Первые три строки задают версию и контекст шага. Значимое содержимое
	// обоих фрагментов задания сохранено, включая регистр и переводы строк.
	for _, tc := range []struct {
		name, title, fragment string
		rows                  []string
		want                  []v118TestWant
	}{
		{"reference", "LIFE IN LONDON", "@text LIFE IN LONDON {\nLondon is a large city.\n}",
			[]string{"London is a large city."}, nil},
		{"example", "", "@text {\nLIFE IN\nLONDON\nLondon is a large city.\n}",
			[]string{"LIFE IN", "LONDON", "London is a large city."}, []v118TestWant{{5, 1, 6, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v118TestOpen(t, "text", tc.title, "title", 3)}
			for _, row := range tc.rows {
				body = append(body, v118TestText(t, row, row, 4))
			}
			body = append(body, v118TestClose(t, 4))
			var raw []string
			for _, row := range body {
				raw = append(raw, row.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v118TestRun(t, v118TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV118PhysicalRunsAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []string
		want []v118TestWant
	}{
		{"empty block", nil, nil},
		{"one row", []string{"TITLE"}, nil},
		{"below threshold", []string{"TITLE", "Ordinary text."}, nil},
		{"two rows", []string{"ONE", "TWO"}, []v118TestWant{{5, 1, 6, 4}}},
		{"three rows", []string{"ONE", "TWO", "THREE"}, []v118TestWant{{5, 1, 7, 6}}},
		{"two maximal groups", []string{"ONE", "TWO", "Mixed", "THREE", "FOUR"},
			[]v118TestWant{{5, 1, 6, 4}, {8, 1, 9, 5}}},
		{"group after lowercase", []string{"Mixed", "ONE", "TWO"}, []v118TestWant{{6, 1, 7, 4}}},
		{"group before lowercase", []string{"ONE", "TWO", "Mixed"}, []v118TestWant{{5, 1, 6, 4}}},
		{"empty physical row", []string{"ONE", "", "TWO"}, nil},
		{"spaces physical row", []string{"ONE", " \t ", "TWO"}, nil},
		{"standalone br", []string{"ONE", "<br>", "TWO"}, nil},
		{"standalone mixed case br", []string{"ONE", " \t<Br>\t ", "TWO"}, nil},
		{"empty emphasis", []string{"ONE", "<strong></strong>", "TWO"}, nil},
		{"spaces emphasis", []string{"ONE", "<em> \t</em>", "TWO"}, nil},
		{"ASCII edges", []string{"  ONE  ", "\tTWO\t"}, []v118TestWant{{5, 3, 6, 5}}},
		{"Unicode edges retained", []string{"\u00a0ONE\u00a0", "\u00a0TWO\u00a0"}, []v118TestWant{{5, 1, 6, 6}}},
		{"source HTML range", []string{"<strong>LIFE IN</strong>", "<em>LONDON</em>"}, []v118TestWant{{5, 1, 6, 16}}},
		{"nested emphasis", []string{"<strong><em>ONE</em></strong>", "TWO"}, []v118TestWant{{5, 1, 6, 4}}},
		{"same name nesting", []string{"<em><EM>ONE</em></EM>", "TWO"}, []v118TestWant{{5, 1, 6, 4}}},
		{"all supported names", []string{"<StRoNg><u><EM>ONE</em></U></strong>", "TWO"}, []v118TestWant{{5, 1, 6, 4}}},
		{"multiple emphasis spans", []string{"<em>ONE</em> <u>TWO</u>", "THREE"}, []v118TestWant{{5, 1, 6, 6}}},
		{"lowercase inside emphasis", []string{"<em>One</em>", "TWO"}, nil},
		{"empty spans have no cased letters", []string{"<em></em><u></u>", "TWO"}, nil},
		{"emphasis spelling is invisible", []string{"<em>123</em>", "<STRONG>456</STRONG>"}, nil},
		{"numeric row breaks", []string{"ONE", "123", "TWO"}, nil},
		{"emphasis plus source indent", []string{" \t<em>ONE</em> ", " <u>TWO</u> \t"}, []v118TestWant{{5, 3, 6, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v118TestRun(t, v118TestRows(t, tc.rows...), "current", tc.want)
		})
	}
}

func TestV118UnicodeCase(t *testing.T) {
	for _, tc := range []struct {
		first, second string
		want          []v118TestWant
	}{
		{"ЖИЗНЬ", "ÉTÉ", []v118TestWant{{5, 1, 6, 4}}},
		{"E\u0301TE\u0301", "TWO", []v118TestWant{{5, 1, 6, 4}}},
		{"ǅ", "ONE", []v118TestWant{{5, 1, 6, 4}}},
		{"Ⅰ", "ONE", []v118TestWant{{5, 1, 6, 4}}},
		{"ONE 123", "TWO!", []v118TestWant{{5, 1, 6, 5}}},
		{"😀ONE", "TWO😀", []v118TestWant{{5, 1, 6, 5}}},
		{"ONE 中文", "TWO", []v118TestWant{{5, 1, 6, 4}}},
		{"ONE", "TwO", nil}, {"ONE", "Ёж", nil}, {"ONE", "Été", nil},
		{"ONE", "TWOᵃ", nil}, {"ONE", "TWOⅰ", nil}, {"ONE", "A\u0345", nil},
		{"123", "!!!", nil}, {"中文", "日本", nil}, {"\u0301", "ONE", nil},
	} {
		t.Run(strconv.Quote(tc.first+"/"+tc.second), func(t *testing.T) {
			v118TestRun(t, v118TestRows(t, tc.first, tc.second), "current", tc.want)
		})
	}
}

func TestV118Escaping(t *testing.T) {
	for _, tc := range []struct {
		raw, value string
		want       []v118TestWant
	}{
		{`\<ONE\>`, `<ONE>`, []v118TestWant{{5, 1, 6, 4}}},
		{`\<em\>ONE\</em\>`, `<em>ONE</em>`, nil},
		{`\<EM\>ONE\</EM\>`, `<EM>ONE</EM>`, []v118TestWant{{5, 1, 6, 4}}},
		{`\\<em>ONE</em>`, `\<em>ONE</em>`, []v118TestWant{{5, 1, 6, 4}}},
		{`\\\<em\>ONE\</em\>`, `\<em>ONE</em>`, nil},
		{`<em\>ONE</em>`, `<em>ONE</em>`, nil},
		{`\\<ONE\>`, `\<ONE>`, nil},
		{`<em>\<X\>ONE</em>`, `<em><X>ONE</em>`, []v118TestWant{{5, 1, 6, 4}}},
		{`\{ONE\} \\`, `{ONE} \`, []v118TestWant{{5, 1, 6, 4}}},
		{`\# ONE`, `# ONE`, []v118TestWant{{5, 1, 6, 4}}},
		{`\@TEXT`, `@TEXT`, []v118TestWant{{5, 1, 6, 4}}},
		{`ONE\N`, `ONE\N`, []v118TestWant{{5, 1, 6, 4}}},
		{`ONE\n`, `ONE\n`, nil},
		{`ONE\`, `ONE\`, []v118TestWant{{5, 1, 6, 4}}},
		{`\_____ ONE`, `_____ ONE`, []v118TestWant{{5, 1, 6, 4}}},
		{`\__ ONE`, `__ ONE`, []v118TestWant{{5, 1, 6, 4}}},
		{`\_ ONE`, `\_ ONE`, []v118TestWant{{5, 1, 6, 4}}},
		{`&lt;ONE&gt;`, `&lt;ONE&gt;`, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v118TestRun(t, v118TestBlock(t, v118TestText(t, tc.raw, tc.value, 4),
				v118TestText(t, "TWO", "TWO", 4)), "current", tc.want)
		})
	}
}

func TestV118MalformedHTMLBreaks(t *testing.T) {
	// Парсер выделил текстовый content, но не проверил встроенный HTML.
	// V118 обязана обнаружить эти ошибки самостоятельно, без чужих результатов.
	for _, raw := range []string{
		"<em>ONE", "ONE</em>", "<strong><em>ONE</strong></em>", "<u>ONE</em>",
		"<EM X>ONE</EM>", "<em class=\"X\">ONE</em>", "<em x='>'>ONE</em>",
		"<em x=\"<u>ONE</u>\">ONE</em>", "<EM >ONE</EM>", "<em>ONE</em >",
		"<U/>", "<U />", "<em>ONE</em/>", "<SPAN>ONE</SPAN>", "<STRONGER>ONE</STRONGER>",
		"<BR/>", "<BR />", "</BR>", "ONE<BR>", "<BR>ONE", "<BR><BR>",
		"<BR X>", "<BR", "<EM", "<", ">", "ONE > TWO", "< EM>ONE</EM>",
	} {
		t.Run(raw, func(t *testing.T) {
			v118TestRun(t, v118TestRows(t, "ONE", raw, "TWO"), "current", nil)
		})
	}
	t.Run("formatting cannot cross physical rows", func(t *testing.T) {
		v118TestRun(t, v118TestRows(t, "<em>ONE", "TWO</em>"), "current", nil)
	})
	t.Run("runs around malformed HTML", func(t *testing.T) {
		v118TestRun(t, v118TestRows(t, "ONE", "TWO", "<U/>", "THREE", "FOUR"), "current",
			[]v118TestWant{{5, 1, 6, 4}, {8, 1, 9, 5}})
	})
}

func TestV118ParsingAndOtherErrors(t *testing.T) {
	for _, kind := range []model.LineType{"invalid", "content"} {
		t.Run(string(kind)+" with unparsed", func(t *testing.T) {
			row := v118TestLine(t, "TITLE __", kind, 4,
				v118TestElement(t, "content", "TITLE ", "TITLE ", 1),
				v118TestElement(t, "unparsed", "__", "", 7))
			v118TestRun(t, v118TestBlock(t, v118TestText(t, "ONE", "ONE", 4), row,
				v118TestText(t, "TWO", "TWO", 4)), "current", nil)
		})
	}
	t.Run("unparsed does not join surrounding runs", func(t *testing.T) {
		v118TestRun(t, v118TestBlock(t,
			v118TestText(t, "ONE", "ONE", 4), v118TestText(t, "TWO", "TWO", 4),
			v118TestLine(t, "TITLE __", "invalid", 4, v118TestElement(t, "unparsed", "TITLE __", "", 1)),
			v118TestText(t, "THREE", "THREE", 4), v118TestText(t, "FOUR", "FOUR", 4)), "current",
			[]v118TestWant{{5, 1, 6, 4}, {8, 1, 9, 5}})
	})
	t.Run("other source errors do not break content", func(t *testing.T) {
		// Классификация осталась content: ошибка длины плейсхолдера уже
		// отмечена парсером, но не представлена invalid/unparsed. По уточнению
		// пользователя V118 не читает эту диагностику и не повторяет её проверку.
		result := v118TestRows(t, "TITLE __", "TWO")
		v118TestParserError(t, result, 5, 0, "Недопустимая длина плейсхолдера.")
		v118TestRun(t, result, "current", []v118TestWant{{5, 1, 6, 4}})
	})
	t.Run("current validator errors do not break content", func(t *testing.T) {
		result := v118TestRows(t, "ONE }", "TWO")
		v118TestRun(t, result, "current", []v118TestWant{{5, 1, 6, 4}})
		found := false
		for _, entry := range result.Diagnostics {
			if entry.Source == "current" && entry.DiagnosticCode == "V049" {
				found = true
			}
		}
		if !found {
			t.Fatal("fixture must trigger the independent V049 error")
		}
	})
	t.Run("ordinary placeholder is visible without interpreting an answer", func(t *testing.T) {
		gap := v118TestElement(t, "placeholder", "_____", "", 5)
		gap.Value = nil
		row := v118TestLine(t, "ONE _____", "content", 4,
			v118TestElement(t, "content", "ONE ", "ONE ", 1), gap)
		v118TestRun(t, v118TestBlock(t, row, v118TestText(t, "TWO", "TWO", 4)),
			"current", []v118TestWant{{5, 1, 6, 4}})
	})
}

func TestV118Contexts(t *testing.T) {
	t.Run("text in fragment", func(t *testing.T) {
		v118TestRun(t, v118TestDocument(t,
			v118TestOpen(t, "fragment", "intro", "identifier", 3),
			v118TestOpen(t, "TeXt", "TITLE", "title", 4),
			v118TestText(t, "ONE", "ONE", 5), v118TestText(t, "TWO", "TWO", 5),
			v118TestClose(t, 5), v118TestClose(t, 4)), "current", []v118TestWant{{6, 1, 7, 4}})
	})
	t.Run("title does not combine with content", func(t *testing.T) {
		v118TestRun(t, v118TestDocument(t, v118TestOpen(t, "text", "TITLE", "title", 3),
			v118TestText(t, "ONE", "ONE", 4), v118TestClose(t, 4)), "current", nil)
	})
	t.Run("nested construction interrupts but does not contribute", func(t *testing.T) {
		result := v118TestDocument(t, v118TestOpen(t, "text", "", "title", 3),
			v118TestText(t, "ONE", "ONE", 4), v118TestText(t, "TWO", "TWO", 4),
			v118TestOpen(t, "note", "TITLE", "title", 4),
			v118TestText(t, "INNER ONE", "INNER ONE", 7), v118TestText(t, "INNER TWO", "INNER TWO", 7),
			v118TestClose(t, 7),
			v118TestText(t, "THREE", "THREE", 4), v118TestText(t, "FOUR", "FOUR", 4), v118TestClose(t, 4))
		v118TestParserError(t, result, 7, 0, "Вложение note запрещено внутри text.")
		v118TestRun(t, result, "current", []v118TestWant{{5, 1, 6, 4}, {11, 1, 12, 5}})
	})
	t.Run("nested block separates individual rows", func(t *testing.T) {
		result := v118TestDocument(t, v118TestOpen(t, "text", "", "title", 3),
			v118TestText(t, "ONE", "ONE", 4), v118TestOpen(t, "note", "", "title", 4),
			v118TestText(t, "INNER", "INNER", 6), v118TestClose(t, 6),
			v118TestText(t, "TWO", "TWO", 4), v118TestClose(t, 4))
		v118TestParserError(t, result, 6, 0, "Вложение note запрещено внутри text.")
		v118TestRun(t, result, "current", nil)
	})
	t.Run("inline nested declaration separates rows", func(t *testing.T) {
		result := v118TestBlock(t, v118TestText(t, "ONE", "ONE", 4),
			v118TestTag(t, "note", "TITLE", "content", 4), v118TestText(t, "TWO", "TWO", 4))
		v118TestParserError(t, result, 6, 0, "Вложение note запрещено внутри text.")
		v118TestRun(t, result, "current", nil)
	})
	t.Run("adjacent blocks do not combine", func(t *testing.T) {
		v118TestRun(t, v118TestDocument(t, v118TestOpen(t, "text", "", "title", 3),
			v118TestText(t, "ONE", "ONE", 4), v118TestClose(t, 4),
			v118TestOpen(t, "text", "", "title", 3), v118TestText(t, "TWO", "TWO", 7),
			v118TestClose(t, 7)), "current", nil)
	})
	t.Run("independent runs in adjacent blocks", func(t *testing.T) {
		v118TestRun(t, v118TestDocument(t, v118TestOpen(t, "text", "", "title", 3),
			v118TestText(t, "ONE", "ONE", 4), v118TestText(t, "TWO", "TWO", 4), v118TestClose(t, 4),
			v118TestOpen(t, "text", "", "title", 3), v118TestText(t, "THREE", "THREE", 8),
			v118TestText(t, "FOUR", "FOUR", 8), v118TestClose(t, 8)), "current",
			[]v118TestWant{{5, 1, 6, 4}, {9, 1, 10, 5}})
	})
	t.Run("outside text", func(t *testing.T) {
		v118TestRun(t, v118TestDocument(t, v118TestText(t, "ONE", "ONE", 3),
			v118TestText(t, "TWO", "TWO", 3)), "current", nil)
	})
	for _, name := range []string{"example", "table", "key", "note", "editor", "multifill"} {
		t.Run("other block "+name, func(t *testing.T) {
			v118TestRun(t, v118TestDocument(t, v118TestTag(t, "task", "practice", "identifier", 3),
				v118TestOpen(t, name, "", "title", 4), v118TestText(t, "ONE", "ONE", 5),
				v118TestText(t, "TWO", "TWO", 5), v118TestClose(t, 5)), "current", nil)
		})
	}
	t.Run("literal text pseudo declaration", func(t *testing.T) {
		v118TestRun(t, v118TestDocument(t, v118TestOpen(t, "editor", "", "content", 3),
			v118TestText(t, "@text {", "@text {", 4), v118TestText(t, "ONE", "ONE", 4),
			v118TestText(t, "TWO", "TWO", 4), v118TestText(t, `\}`, "}", 4),
			v118TestClose(t, 4)), "current", nil)
	})
}

func TestV118BoundariesEndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v118TestRows(t, "LIFE IN", "LONDON", "London is a large city.")
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v118TestWant{{5, 1, 6, 7}}
			v118TestRun(t, result, "first", want)
			v118TestRun(t, v118TestRows(t, "ONE"), "other", nil)
			v118TestRun(t, result, "second", want)
		})
	}
	t.Run("EOF without block closing", func(t *testing.T) {
		result := v118TestDocument(t, v118TestOpen(t, "text", "", "title", 3),
			v118TestText(t, "ONE", "ONE", 4), v118TestText(t, "TWO", "TWO", 4))
		result.Lines[5].LineEnding = ""
		v118TestParserError(t, result, 4, 1, "Блок text не закрыт.")
		v118TestRun(t, result, "current", []v118TestWant{{5, 1, 6, 4}})
	})
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v118TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v118TestRun(t, result, "current", nil)
		})
	}
}

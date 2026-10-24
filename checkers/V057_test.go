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

const (
	v057TestMessage = "Между маркером заголовка и текстом должен стоять ровно один обычный пробел."
	v057TestMissing = "После маркера отсутствует пробел"
)

// v057TestWant хранит явные исходные координаты с исключающим концом.
type v057TestWant struct {
	line, start, end int
	detail           string
}

func TestV057Registration(t *testing.T) {
	definitions, err := v057TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V057" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != v057TestMessage || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V057 definitions = %d, want 1", count)
	}
}

func TestV057ReferenceAndExamples(t *testing.T) {
	// Перед неизменёнными двухстрочными фрагментами добавлены только
	// обязательная версия и основной заголовок: фрагменты занимают строки 3–4.
	for _, tc := range []struct {
		name string
		line model.Line
		want []v057TestWant
	}{
		{"reference", v057TestHeading(t, 3, " ", "Practice", "", ""), nil},
		{"example", v057TestHeading(t, 3, "  ", "Practice", "", ""), []v057TestWant{{3, 4, 6, ""}}},
		{"missing separator example", v057TestInvalid(t, "###Practice", 2),
			[]v057TestWant{{3, 1, 4, v057TestMissing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := 3
			if tc.line.LineType == "invalid" {
				parent = 2
			}
			result := v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				tc.line, v057TestText(t, "Read.", "Read.", parent))
			v057TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV057LevelsAndSeparators(t *testing.T) {
	for _, tc := range []struct {
		name   string
		level  int
		gap    string
		parsed bool
		want   []v057TestWant
	}{
		{"level one valid", 1, " ", true, nil},
		{"level two valid", 2, " ", true, nil},
		{"level three valid", 3, " ", true, nil},
		{"level one two spaces", 1, "  ", true, []v057TestWant{{2, 2, 4, ""}}},
		{"level two two spaces", 2, "  ", true, []v057TestWant{{3, 3, 5, ""}}},
		{"level three three spaces", 3, "   ", true, []v057TestWant{{3, 4, 7, ""}}},
		{"level one missing", 1, "", false, []v057TestWant{{2, 1, 2, v057TestMissing}}},
		{"level two missing", 2, "", false, []v057TestWant{{3, 1, 3, v057TestMissing}}},
		{"level three missing", 3, "", false, []v057TestWant{{3, 1, 4, v057TestMissing}}},
		{"level one tab", 1, "\t", false, []v057TestWant{{2, 2, 3, ""}}},
		{"level two tab", 2, "\t", false, []v057TestWant{{3, 3, 4, ""}}},
		{"two tabs", 3, "\t\t", false, []v057TestWant{{3, 4, 6, ""}}},
		{"mixed whitespace", 3, " \t\u00a0", false, []v057TestWant{{3, 4, 7, ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Классификация задана таблицей: несколько U+0020 допустимы
			// по DSL 2.1.2, хотя V057 намеренно строже спецификации.
			body := []model.Line{}
			parent := 0
			if tc.level != 1 {
				body = append(body, v057TestHeading(t, 1, " ", "Lesson", "", ""))
				parent = 2
			}
			line := v057TestInvalid(t, strings.Repeat("#", tc.level)+tc.gap+"Practice", parent)
			if tc.parsed {
				line = v057TestHeading(t, tc.level, tc.gap, "Practice", "", "")
				parent = len(body) + 2
			}
			body = append(body, line, v057TestText(t, "Read.", "Read.", parent))
			v057TestRun(t, v057TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV057UnicodeWhitespaceAndMissingTitle(t *testing.T) {
	// Множество задано независимо от unicode.White_Space проверяемого кода.
	// CR и LF запрещены внутри Raw; окончания строк проверяются отдельно.
	whitespace := []string{"\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"}
	for _, gap := range whitespace {
		t.Run("separator "+strconv.Quote(gap), func(t *testing.T) {
			result := v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				v057TestInvalid(t, "###"+gap+"Practice", 2), v057TestText(t, "Read.", "Read.", 2))
			v057TestRun(t, result, "current", []v057TestWant{{3, 4, 5, ""}})
		})
	}
	gaps := append([]string{"", " ", "  ", " \t\u00a0\u3000"}, whitespace...)
	for _, marker := range []string{"#", "##", "###"} {
		for _, gap := range gaps {
			t.Run("no title "+strconv.Quote(marker+gap), func(t *testing.T) {
				body := []model.Line{}
				parent := 0
				if marker != "#" {
					body = append(body, v057TestHeading(t, 1, " ", "Lesson", "", ""))
					parent = 2
				}
				body = append(body, v057TestInvalid(t, marker+gap, parent),
					v057TestText(t, "Read.", "Read.", parent))
				v057TestRun(t, v057TestDocument(t, body...), "current", nil)
			})
		}
	}
}

func TestV057RawRecognitionAndRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v057TestWant
	}{
		{"indent is ignored", v057TestHeading(t, 3, " ", "Practice", " \t", ""), nil},
		{"indented two spaces", v057TestHeading(t, 3, "  ", "Practice", " \t", ""),
			[]v057TestWant{{3, 6, 8, ""}}},
		{"indented missing", v057TestInvalid(t, " \t###Practice", 2),
			[]v057TestWant{{3, 3, 6, v057TestMissing}}},
		{"title internal and trailing spaces", v057TestHeading(t, 3, " ", "My  Lesson", "", "  "), nil},
		{"hash inside title", v057TestHeading(t, 3, " ", "#Practice", "", ""), nil},
		{"uppercase Unicode title", v057TestInvalid(t, "###ПРАКТИКА", 2),
			[]v057TestWant{{3, 1, 4, v057TestMissing}}},
		{"supplementary and combining title", v057TestInvalid(t, "###😀e\u0301", 2),
			[]v057TestWant{{3, 1, 4, v057TestMissing}}},
		{"zero width space is text", v057TestInvalid(t, "###\u200b", 2),
			[]v057TestWant{{3, 1, 4, v057TestMissing}}},
		{"space before zero width text", v057TestHeading(t, 3, " ", "\u200b", "", ""), nil},
		{"four hashes missing separator", v057TestInvalid(t, "####Practice", 2), nil},
		{"four hashes two spaces", v057TestInvalid(t, "####  Practice", 2), nil},
		{"five hashes tab", v057TestInvalid(t, "#####\tPractice", 2), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := 2
			if tc.line.LineType == "heading" {
				parent = 3
			}
			v057TestRun(t, v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				tc.line, v057TestText(t, "Read.", "Read.", parent)), "current", tc.want)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{"\u00a0###  Practice", "\u00a0###  Practice"}, {" \t\u2003###Practice", "\u2003###Practice"},
		{`\###Practice`, "###Practice"}, {" \t\\###Practice", "###Practice"},
		{`\\###Practice`, `\###Practice`}, {"Text ###Practice", "Text ###Practice"},
		{"＃＃＃Practice", "＃＃＃Practice"},
	} {
		t.Run("not candidate "+strconv.Quote(tc.raw), func(t *testing.T) {
			v057TestRun(t, v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				v057TestText(t, tc.raw, tc.value, 2)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v057TestRun(t, v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				v057TestLine(t, raw, "blank", 2)), "current", nil)
		})
	}
	t.Run("preserved level alongside unparsed remainder", func(t *testing.T) {
		unparsed := v057TestElement(t, "unparsed", "Practice", "", 4)
		unparsed.Value = nil
		line := v057TestLine(t, "###Practice", "invalid", 2,
			v057TestElement(t, "heading-level", "###", "3", 1), unparsed)
		v057TestRun(t, v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
			line, v057TestText(t, "Read.", "Read.", 2)), "current",
			[]v057TestWant{{3, 1, 4, v057TestMissing}})
	})
}

func TestV057BlockContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v057TestWant
	}{
		{"nested attempt", []model.Line{
			v057TestOpen(t, "text", 2), v057TestInvalid(t, "###  Practice", 3),
			v057TestText(t, "Read.", "Read.", 3), v057TestClose(t, 3),
		}, []v057TestWant{{4, 4, 6, ""}}},
		{"deeply nested attempt", []model.Line{
			v057TestOpen(t, "instr", 2), v057TestOpen(t, "hint", 3),
			v057TestInvalid(t, "###Practice", 4), v057TestClose(t, 4), v057TestClose(t, 3),
		}, []v057TestWant{{5, 1, 4, v057TestMissing}}},
		{"valid separator in forbidden heading context", []model.Line{
			v057TestOpen(t, "text", 2), v057TestInvalid(t, "### Practice", 3), v057TestClose(t, 3),
		}, nil},
		{"after literal block", []model.Line{
			v057TestOpen(t, "editor", 2), v057TestText(t, "###Practice", "###Practice", 3),
			v057TestClose(t, 3), v057TestInvalid(t, "###Practice", 2), v057TestText(t, "Read.", "Read.", 2),
		}, []v057TestWant{{6, 1, 4, v057TestMissing}}},
		{"one-line editor does not hide next line", []model.Line{
			v057TestTag(t, "editor", "Comment.", "content", 2), v057TestInvalid(t, "###Practice", 2),
		}, []v057TestWant{{4, 1, 4, v057TestMissing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v057TestHeading(t, 1, " ", "Lesson", "", "")}, tc.body...)
			v057TestRun(t, v057TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "EDITOR", "multifill", "MULTIFILL"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v057TestRun(t, v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				v057TestOpen(t, tag, 2), v057TestText(t, "###  Practice", "###  Practice", 3),
				v057TestText(t, "###Practice", "###Practice", 3), v057TestClose(t, 3)), "current", nil)
		})
		t.Run("literal "+tag+" in task", func(t *testing.T) {
			v057TestRun(t, v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				v057TestTag(t, "task", "1", "identifier", 2), v057TestTag(t, "instr", "Read.", "content", 3),
				v057TestOpen(t, tag, 3), v057TestText(t, "###  Practice", "###  Practice", 5),
				v057TestText(t, "###Practice", "###Practice", 5), v057TestClose(t, 5)), "current", nil)
		})
	}
}

func TestV057MultipleFindingsEndingsAndHistory(t *testing.T) {
	result := v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
		v057TestHeading(t, 3, "  ", "One", "", ""), v057TestText(t, "Read.", "Read.", 3),
		v057TestInvalid(t, "###Two", 3), v057TestText(t, "Read.", "Read.", 3))
	v057TestRun(t, result, "current", []v057TestWant{{3, 4, 6, ""}, {5, 1, 4, v057TestMissing}})
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v057TestDocument(t, v057TestHeading(t, 1, " ", "Lesson", "", ""),
				v057TestHeading(t, 3, "  ", "Practice", "", ""), v057TestText(t, "Read.", "Read.", 3))
			if ending == "" {
				result.Lines = result.Lines[:3]
				count := 3
				result.Document.LineCount = &count
				result.Lines[2].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v057TestWant{{3, 4, 6, ""}}
			v057TestRun(t, result, "first", want)
			v057TestRun(t, result, "second", want)
		})
	}
}

func TestV057UnavailableLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v057TestDocument(t)
			result.Document.DSLVersion = &version
			result.Lines = []model.Line{}
			count := 0
			result.Document.LineCount = &count
			if version == "1.2" {
				v057TestRun(t, result, "empty", nil)
				result.Document.LineCount = nil
				v057TestRun(t, result, "unavailable", nil)
				return
			}
			before := v057TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v057TestRegistry(t))
			if err == nil || checks != nil || v057TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v057TestLine сохраняет готовые элементы и явного родителя, не разбирая Raw.
func v057TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v057TestElement вычисляет длину входного элемента, но не ожидания диагностики.
func v057TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v057TestHeading строит разобранный заголовок с явно заданными частями.
// Вызывающий выбирает допустимый по DSL разделитель из U+0020.
func v057TestHeading(t *testing.T, level int, gap, title, indent, trailing string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	start := utf8.RuneCountInString(indent) + 1
	return v057TestLine(t, indent+marker+gap+title+trailing, "heading", 0,
		v057TestElement(t, "heading-level", marker, strconv.Itoa(level), start),
		v057TestElement(t, "title", title, title, start+level+utf8.RuneCountInString(gap)))
}

// v057TestText задаёт исходное и семантическое содержимое независимо.
func v057TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v057TestLine(t, raw, "content", parent, v057TestElement(t, "content", raw, value, 1))
}

// v057TestInvalid задаёт неразобранную ошибочную попытку конструкции.
func v057TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	element := v057TestElement(t, "unparsed", raw, "", 1)
	element.Value = nil
	return v057TestLine(t, raw, "invalid", parent, element)
}

// v057TestTag строит известный тег с заданным типом параметра.
func v057TestTag(t *testing.T, tag, parameter string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v057TestElement(t, "tag", raw, strings.ToLower(tag), 1)}
	if parameter != "" {
		elements = append(elements, v057TestElement(t, kind, parameter, parameter, len(raw)+2))
		raw += " " + parameter
	}
	return v057TestLine(t, raw, "tag", parent, elements...)
}

// v057TestOpen строит блочное объявление без параметров.
func v057TestOpen(t *testing.T, tag string, parent int) model.Line {
	t.Helper()
	line := v057TestTag(t, tag, "", "", parent)
	line.Elements = append(line.Elements, v057TestElement(t, "block-open", "{", "{", len(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v057TestClose связывает закрывающую строку с открытием блока.
func v057TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v057TestLine(t, "}", "block-end", parent, v057TestElement(t, "block-close", "}", "}", 1))
}

// v057TestDocument добавляет обязательную версию к явно заданным строкам.
// Глубины следуют заданным родителям; исходные ошибки обеспечивают ссылки
// элементов unparsed. Ни Raw, ни ожидаемые срабатывания здесь не анализируются.
func v057TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v057TestTag(t, "dsl-version", "1.2", "version", 0)}, body...)
	version, count := "1.2", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	titles := map[string][]string{}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid test parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		level, title := "", ""
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "heading-level" {
				level = *element.Value
			}
			if element.ElementType == "title" {
				title = *element.Value
			}
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
		if line.LineType == "heading" {
			titles[level] = append(titles[level], title)
		}
	}
	if len(titles["1"]) == 1 {
		result.Document.Metadata.Title = &titles["1"][0]
	}
	if len(titles["2"]) == 1 {
		result.Document.Metadata.Subtitle = &titles["2"][0]
	}
	return result
}

// v057TestRegistry создаёт реестр исключительно из полного штатного каталога.
func v057TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v057TestRun проверяет новые V057 после штатных Prepare/Run и сохранность входа.
func v057TestRun(t *testing.T, result *model.Result, processingID string, want []v057TestWant) {
	t.Helper()
	before := v057TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v057TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v057TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V057" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V057 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		message := v057TestMessage
		if expected.detail != "" {
			message += "\nконкретизация: " + expected.detail
		}
		if entry.Location == nil || *entry.Location != location || entry.Message != message ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.ID == "" || entry.Source != processingID {
			t.Fatalf("V057 occurrence %d = %+v; want location %+v, message %q, no related locations", i, entry, location, message)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v057TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Только производные флаги и ссылки возвращаются к исходным значениям.
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
	if v057TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v057TestSnapshot сериализует модель для проверки отсутствия побочных изменений.
func v057TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

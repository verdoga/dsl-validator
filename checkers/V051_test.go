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

func TestV051Registration(t *testing.T) {
	definitions, err := v051TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V051" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Заголовок второго уровня объявлен повторно." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V051 definitions = %d, want 1", count)
	}
}

func TestV051ReferenceAndExample(t *testing.T) {
	// DSL 1.3 и 1.30 требуют добавить перед фрагментами @dsl-version 1.2.
	// Сами фрагменты ниже сохранены дословно, без табличных <pre>.
	for _, tc := range []struct {
		name, fragment, subtitle string
		body                     []model.Line
		want                     [][6]int
	}{
		{"reference", "# Lesson\n## Vocabulary\nRead the words.", "Vocabulary", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"),
			v051TestText(t, "Read the words.", "Read the words.", 3, 1),
		}, nil},
		{"example", "# Lesson\n## Vocabulary\n## Grammar\nRead the text.", "", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"),
			v051TestHeading(t, "", "##", "2", "Grammar"),
			v051TestText(t, "Read the text.", "Read the text.", 4, 1),
		}, [][6]int{{4, 1, 3, 3, 1, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v051TestDocument(t, tc.subtitle, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[1:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v051TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV051Candidates(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"no second level", []model.Line{v051TestText(t, "Read.", "Read.", 2, 1)}, nil},
		{"one candidate at EOF", []model.Line{v051TestHeading(t, "", "##", "2", "Vocabulary")}, nil},
		{"identical names", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestHeading(t, "", "##", "2", "Vocabulary"),
		}, [][6]int{{4, 1, 3, 3, 1, 3}}},
		{"all repeats refer to first", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestHeading(t, "", "##", "2", "Grammar"),
			v051TestHeading(t, "", "##", "2", "Practice"),
		}, [][6]int{{4, 1, 3, 3, 1, 3}, {5, 1, 3, 3, 1, 3}}},
		{"original indentation", []model.Line{
			v051TestHeading(t, " \t", "##", "2", "Vocabulary"), v051TestHeading(t, "  \t", "##", "2", "Grammar"),
		}, [][6]int{{4, 4, 6, 3, 3, 5}}},
		{"only two hashes", []model.Line{
			v051TestHeading(t, "", "###", "3", "First"), v051TestInvalid(t, "#### Wrong", 3, 1),
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestHeading(t, "", "###", "3", "Next"),
			v051TestHeading(t, "", "##", "2", "Grammar"),
		}, [][6]int{{7, 1, 3, 5, 1, 3}}},
		{"later hashes are title", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestHeading(t, "", "##", "2", "# Title ##"),
		}, [][6]int{{4, 1, 3, 3, 1, 3}}},
		{"Unicode and case do not change count", []model.Line{
			v051TestHeading(t, "", "##", "2", "Слова"), v051TestHeading(t, "", "##", "2", "😀e\u0301"),
			v051TestHeading(t, "", "##", "2", "СЛОВА"),
		}, [][6]int{{4, 1, 3, 3, 1, 3}, {5, 1, 3, 3, 1, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			subtitle := ""
			if tc.name == "one candidate at EOF" {
				subtitle = "Vocabulary"
			}
			v051TestRun(t, v051TestDocument(t, subtitle, tc.body...), "current", tc.want)
		})
	}
}

func TestV051MalformedCandidates(t *testing.T) {
	for _, raw := range []string{"##", "##Title", "##\tTitle", "## \t ", "##Я", "##\t###", "##\\#"} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			t.Run("first without heading-level", func(t *testing.T) {
				v051TestRun(t, v051TestDocument(t, "",
					v051TestInvalid(t, raw, 2, 1), v051TestHeading(t, "", "##", "2", "Vocabulary")),
					"current", [][6]int{{4, 1, 3, 3, 1, 3}})
			})
			t.Run("repeat without heading-level", func(t *testing.T) {
				v051TestRun(t, v051TestDocument(t, "",
					v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestInvalid(t, raw, 3, 1)),
					"current", [][6]int{{4, 1, 3, 3, 1, 3}})
			})
			t.Run("single malformed candidate", func(t *testing.T) {
				v051TestRun(t, v051TestDocument(t, "", v051TestInvalid(t, raw, 2, 1)), "current", nil)
			})
		})
	}
}

func TestV051NonCandidates(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"ordinary", "Read.", "Read."},
		{"inside text", "Text ## Grammar", "Text ## Grammar"},
		{"nonbreaking space", "\u00a0## Grammar", "\u00a0## Grammar"},
		{"em space", "\u2003## Grammar", "\u2003## Grammar"},
		{"nonbreaking space after indentation", " \t\u00a0## Grammar", "\u00a0## Grammar"},
		{"em space after indentation", " \t\u2003## Grammar", "\u2003## Grammar"},
		{"escaped", `\## Grammar`, "## Grammar"},
		{"escaped after indentation", " \t\\## Grammar", "## Grammar"},
		{"literal backslash", `\\## Grammar`, `\## Grammar`},
		{"fullwidth hashes", "＃＃ Grammar", "＃＃ Grammar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// До H проверяем, что строка не становится первым кандидатом;
			// после H — что не порождает повторное объявление.
			v051TestRun(t, v051TestDocument(t, "Vocabulary",
				v051TestText(t, tc.raw, tc.value, 2, 1),
				v051TestHeading(t, "", "##", "2", "Vocabulary"),
				v051TestText(t, tc.raw, tc.value, 4, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", "  \t  "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v051TestRun(t, v051TestDocument(t, "Vocabulary",
				v051TestLine(t, raw, "blank", 2, 1),
				v051TestHeading(t, "", "##", "2", "Vocabulary"),
				v051TestText(t, "Read.", "Read.", 4, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"#", "#Title", "###", "###Title", "####", "######## Title"} {
		t.Run("other initial run "+strconv.Quote(raw), func(t *testing.T) {
			v051TestRun(t, v051TestDocument(t, "Vocabulary",
				v051TestInvalid(t, raw, 2, 1),
				v051TestHeading(t, "", "##", "2", "Vocabulary"),
				v051TestInvalid(t, raw, 4, 1)), "current", nil)
		})
	}
}

func TestV051NestedCandidatesAndContinuity(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"nested first", []model.Line{
			v051TestOpen(t, "text", 2, 1), v051TestInvalid(t, "## Inner", 3, 2),
			v051TestClose(t, 3, 2), v051TestHeading(t, "", "##", "2", "Vocabulary"),
		}, [][6]int{{6, 1, 3, 4, 1, 3}}},
		{"nested repeat", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestOpen(t, "text", 3, 1),
			v051TestInvalid(t, "## Inner", 4, 2), v051TestClose(t, 4, 2),
		}, [][6]int{{5, 1, 3, 3, 1, 3}}},
		{"deep nested repeat", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestOpen(t, "instr", 3, 1),
			v051TestText(t, "Read.", "Read.", 4, 2), v051TestOpen(t, "hint", 4, 2),
			v051TestInvalid(t, "## Inner", 6, 3), v051TestClose(t, 6, 3), v051TestClose(t, 4, 2),
		}, [][6]int{{7, 1, 3, 3, 1, 3}}},
		{"blank and block boundaries do not reset", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestLine(t, "", "blank", 3, 1),
			v051TestOpen(t, "text", 3, 1), v051TestText(t, "Read.", "Read.", 5, 2),
			v051TestClose(t, 5, 2), v051TestHeading(t, "", "##", "2", "Grammar"),
		}, [][6]int{{8, 1, 3, 3, 1, 3}}},
		{"steps and tasks do not reset", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestText(t, "Read.", "Read.", 3, 1),
			v051TestHeading(t, "", "###", "3", "Practice"), v051TestTag(t, "newpage", "", "", 0, 0),
			v051TestTag(t, "task", "identifier", "a", 6, 1), v051TestTag(t, "instr", "content", "Read.", 7, 2),
			v051TestTag(t, "endtask", "", "", 7, 2), v051TestHeading(t, "", "##", "2", "Grammar"),
		}, [][6]int{{10, 1, 3, 3, 1, 3}}},
		{"single-line editor", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestTag(t, "editor", "content", "## Literal", 3, 1),
			v051TestHeading(t, "", "##", "2", "Grammar"),
		}, [][6]int{{5, 1, 3, 3, 1, 3}}},
		{"task contents count", []model.Line{
			v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestTag(t, "task", "identifier", "a", 3, 1),
			v051TestTag(t, "instr", "content", "Read.", 4, 2), v051TestInvalid(t, "## Inner", 4, 2),
			v051TestTag(t, "endtask", "", "", 4, 2),
		}, [][6]int{{6, 1, 3, 3, 1, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v051TestRun(t, v051TestDocument(t, "", tc.body...), "current", tc.want)
		})
	}
}

func TestV051LiteralBlocks(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			t.Run("excluded first", func(t *testing.T) {
				v051TestRun(t, v051TestDocument(t, "Vocabulary",
					v051TestOpen(t, tag, 2, 1), v051TestText(t, "## Literal", "## Literal", 3, 2),
					v051TestClose(t, 3, 2), v051TestHeading(t, "", "##", "2", "Vocabulary")), "current", nil)
			})
			t.Run("excluded repeat preserves first", func(t *testing.T) {
				v051TestRun(t, v051TestDocument(t, "",
					v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestOpen(t, tag, 3, 1),
					v051TestText(t, "## Literal", "## Literal", 4, 2), v051TestClose(t, 4, 2),
					v051TestHeading(t, "", "##", "2", "Grammar")), "current", [][6]int{{7, 1, 3, 3, 1, 3}})
			})
			t.Run("literal block inside task", func(t *testing.T) {
				v051TestRun(t, v051TestDocument(t, "",
					v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestTag(t, "task", "identifier", "a", 3, 1),
					v051TestTag(t, "instr", "content", "Read.", 4, 2), v051TestOpen(t, tag, 4, 2),
					v051TestText(t, "## Literal", "## Literal", 6, 3), v051TestClose(t, 6, 3),
					v051TestTag(t, "endtask", "", "", 4, 2), v051TestHeading(t, "", "##", "2", "Grammar")),
					"current", [][6]int{{10, 1, 3, 3, 1, 3}})
			})
			t.Run("tag-like literal text creates no nested block", func(t *testing.T) {
				// По DSL 2.8.11/4.4.11 @text \{ здесь является текстом;
				// следующая отдельная } завершает именно литеральный блок.
				v051TestRun(t, v051TestDocument(t, "Vocabulary",
					v051TestOpen(t, tag, 2, 1), v051TestText(t, `@text \{`, "@text {", 3, 2),
					v051TestText(t, "## Literal", "## Literal", 3, 2), v051TestClose(t, 3, 2),
					v051TestHeading(t, "", "##", "2", "Vocabulary")), "current", nil)
			})
		})
	}
}

func TestV051EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		endings []model.LineEnding
	}{
		{"LF", []model.LineEnding{"\n", "\n", "\n", "\n"}},
		{"CRLF", []model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n"}},
		{"mixed", []model.LineEnding{"\r\n", "\n", "\r\n", "\n"}},
		{"EOF", []model.LineEnding{"\n", "\n", "\n", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v051TestDocument(t, "",
				v051TestHeading(t, "", "##", "2", "Vocabulary"), v051TestHeading(t, "", "##", "2", "Grammar"))
			for i, ending := range tc.endings {
				result.Lines[i].LineEnding = ending
			}
			v051TestRun(t, result, "first", [][6]int{{4, 1, 3, 3, 1, 3}})
			v051TestRun(t, result, "second", [][6]int{{4, 1, 3, 3, 1, 3}})
			v051TestRun(t, v051TestDocument(t, "Vocabulary",
				v051TestHeading(t, "", "##", "2", "Vocabulary")), "separate", nil)
		})
	}
}

func TestV051EmptyLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			// Строки недоступны; версия известна из результата парсера.
			result := &model.Result{
				Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version,
					Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
				Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
			}
			if version == "1.2" {
				v051TestRun(t, result, "current", nil)
				return
			}
			before := v051TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v051TestRegistry(t))
			if err == nil || checks != nil || v051TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v051TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Пустой subtitle
// означает отсутствие однозначно извлечённого подзаголовка. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V051.
func v051TestDocument(t *testing.T, subtitle string, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v051TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v051TestHeading(t, "", "#", "1", title),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	if subtitle != "" {
		result.Document.Metadata.Subtitle = &subtitle
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

// v051TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v051TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v051TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v051TestElement(t, "heading-level", marker, level, start, end),
		v051TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v051TestInvalid задаёт ошибочную строку без распознанного heading-level.
func v051TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v051TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v051TestLine(t, raw, "invalid", parent, depth, element)
}

// v051TestTag собирает однострочный тег с одним явно типизированным параметром.
func v051TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v051TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v051TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v051TestLine(t, raw, "tag", parent, depth, elements...)
}

// v051TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v051TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v051TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v051TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v051TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v051TestClose задаёт закрытие блока с явным родителем.
func v051TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v051TestLine(t, "}", "block-end", parent, depth, v051TestElement(t, "block-close", "}", "}", 1, 2))
}

// v051TestText сохраняет независимо заданные исходное и семантическое значения.
func v051TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v051TestLine(t, raw, "content", parent, depth,
		v051TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v051TestLine не классифицирует исходник и не вычисляет родительские связи.
func v051TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v051TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v051TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v051TestRegistry использует штатный каталог целиком.
func v051TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v051TestRun проверяет только новые V051. Каждая шестёрка задаёт явно основной
// и связанный диапазоны: строка, начало, исключающий конец для каждого.
func v051TestRun(t *testing.T, result *model.Result, processingID string, want [][6]int) {
	t.Helper()
	before := v051TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v051TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v051TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V051" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V051 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		related := model.Location{Start: model.Position{Line: position[3], Column: position[4]},
			End: model.Position{Line: position[3], Column: position[5]}}
		if entry.Location == nil || *entry.Location != location ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Заголовок второго уровня объявлен повторно." {
			t.Fatalf("occurrence %d = %+v; want location %+v, related %+v and exact V051 metadata", i, entry, location, related)
		}
	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v051TestSnapshot(t, result)), &restored); err != nil {
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
	if v051TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v051TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v051TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

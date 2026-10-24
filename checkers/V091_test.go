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

func TestV091Registration(t *testing.T) {
	definitions, err := v091TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V091" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Однострочный список слов не заканчивается точкой." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V091 definitions = %d, want 1", count)
	}
}

func TestV091ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		want          [][3]int
	}{
		{"reference", "Italian; Spanish.", nil},
		{"example", "Italian; Spanish", [][3]int{{3, 11, 27}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Только обязательный внешний контекст; фрагменты не изменены.
			result := v091TestDocument(t, v091TestNamed(t, "wordlist", "content", tc.content, 2, 1))
			v091TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@wordlist "+tc.content)
			v091TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV091LastCharacter(t *testing.T) {
	for _, tc := range []struct {
		raw, value string
		want       [][3]int
	}{
		{"a", "a", [][3]int{{3, 11, 12}}},
		{".", ".", nil},
		{"a.b", "a.b", [][3]int{{3, 11, 14}}},
		{"a...", "a...", nil},
		{"a…", "a…", [][3]int{{3, 11, 13}}},
		{"a。", "a。", [][3]int{{3, 11, 13}}},
		{"cat", "cat", [][3]int{{3, 11, 14}}},
		{"CAT", "CAT", [][3]int{{3, 11, 14}}},
		{"CAT.", "CAT.", nil},
		{"cat.\u00a0", "cat.\u00a0", [][3]int{{3, 11, 16}}},
		{"\u00a0", "\u00a0", [][3]int{{3, 11, 12}}},
		{"cat.\u2003", "cat.\u2003", [][3]int{{3, 11, 16}}},
		{"cat.\u200b", "cat.\u200b", [][3]int{{3, 11, 16}}},
		{"Я😀", "Я😀", [][3]int{{3, 11, 13}}},
		{"Я😀.", "Я😀.", nil},
		{"a  b", "a  b", [][3]int{{3, 11, 15}}},
		{"a\tb", "a\tb", [][3]int{{3, 11, 14}}},
		{`\{cat\}`, "{cat}", [][3]int{{3, 11, 18}}},
		{`a\;b; c.`, "a;b; c.", nil},
		{`a\;b; c`, "a;b; c", [][3]int{{3, 11, 18}}},
		{`cat.\;`, "cat.;", [][3]int{{3, 11, 17}}},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v091TestRun(t, v091TestDocument(t,
				v091TestLine(t, "@wordlist "+tc.raw, "tag", 2, 1,
					v091TestElement(t, "tag", "@wordlist", "wordlist", 1),
					v091TestElement(t, "content", tc.raw, tc.value, 11))),
				"current", tc.want)
		})
	}
}

func TestV091WhitespaceAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		name, raw, content string
		start              int
		want               [][3]int
	}{
		{"trim edges", "@wordlist   cat\t", "cat", 13, [][3]int{{3, 13, 16}}},
		{"dot before edges", "@wordlist cat. \t", "cat.", 11, nil},
		{"content only spaces", "@wordlist   ", "", 13, nil},
		{"content only tabs", "@wordlist \t\t", "", 13, nil},
		{"trim before NBSP", "@wordlist  \u00a0 \t", "\u00a0", 12, [][3]int{{3, 12, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v091TestDocument(t,
				v091TestLine(t, tc.raw, "tag", 2, 1,
					v091TestElement(t, "tag", "@wordlist", "wordlist", 1),
					v091TestElement(t, "content", tc.content, tc.content, tc.start)))
			if tc.content == "" {
				v091TestParserError(t, result, 3, -1, "Отсутствует содержимое wordlist.")
			}
			v091TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("no content element", func(t *testing.T) {
		result := v091TestDocument(t, v091TestLine(t, "@wordlist", "tag", 2, 1,
			v091TestElement(t, "tag", "@wordlist", "wordlist", 1)))
		v091TestParserError(t, result, 3, -1, "Отсутствует содержимое wordlist.")
		v091TestRun(t, result, "current", nil)
	})
	t.Run("indentation and canonical name", func(t *testing.T) {
		v091TestRun(t, v091TestDocument(t,
			v091TestLine(t, "  @WORDLIST cat", "tag", 2, 1,
				v091TestElement(t, "tag", "@WORDLIST", "wordlist", 3),
				v091TestElement(t, "content", "cat", "cat", 13))),
			"current", [][3]int{{3, 13, 16}})
	})
}

func TestV091RecognizedForm(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"unparsed without tag", v091TestInvalid(t, "@wordlist\tcat", 2, 1), nil},
		{"tag line without tag element", v091TestLine(t, "@wordlist\tcat", "tag", 2, 1,
			v091TestUnparsed(t, "@wordlist\tcat", 1)), nil},
		{"recognized single line", v091TestLine(t, "@wordlist\tcat", "tag", 2, 1,
			v091TestElement(t, "tag", "@wordlist", "wordlist", 1),
			v091TestUnparsed(t, "\tcat", 10)), [][3]int{{3, 11, 14}}},
		{"invalid form with tag", v091TestLine(t, "@wordlist\tcat", "invalid", 2, 1,
			v091TestElement(t, "tag", "@wordlist", "wordlist", 1),
			v091TestUnparsed(t, "\tcat", 10)), nil},
		{"unknown tag", v091TestInvalid(t, "@wordlistExtra cat", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Ошибка разделителя или неизвестное имя; форму задаёт модель.
			v091TestRun(t, v091TestDocument(t, tc.line), "current", tc.want)
		})
	}
	t.Run("unparsed remainder after content", func(t *testing.T) {
		// Лишняя закрывающая скобка не должна теряться за content.Value().
		v091TestRun(t, v091TestDocument(t,
			v091TestLine(t, "@wordlist cat. }", "tag", 2, 1,
				v091TestElement(t, "tag", "@wordlist", "wordlist", 1),
				v091TestElement(t, "content", "cat.", "cat.", 11),
				v091TestUnparsed(t, "}", 16))), "current", [][3]int{{3, 11, 17}})
	})
}

func TestV091Contexts(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@wordlist cat`, "@wordlist cat"},
		{"Mention @wordlist cat", "Mention @wordlist cat"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v091TestRun(t, v091TestDocument(t, v091TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("other tag", func(t *testing.T) {
		v091TestRun(t, v091TestDocument(t,
			v091TestNamed(t, "note", "content", "@wordlist cat", 2, 1)), "current", nil)
	})
	for _, named := range []bool{false, true} {
		t.Run("block named="+strconv.FormatBool(named), func(t *testing.T) {
			open := v091TestOpen(t, "wordlist", 2, 1)
			if named {
				open = v091TestLine(t, "@wordlist Words {", "block-start", 2, 1,
					v091TestElement(t, "tag", "@wordlist", "wordlist", 1),
					v091TestElement(t, "title", "Words", "Words", 11),
					v091TestElement(t, "block-open", "{", "{", 17))
			}
			v091TestRun(t, v091TestDocument(t, open,
				v091TestText(t, "cat", "cat", 3, 2), v091TestText(t, "dog", "dog", 3, 2),
				v091TestClose(t, 3, 2)), "current", nil)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			v091TestRun(t, v091TestDocument(t, v091TestOpen(t, tag, 2, 1),
				v091TestText(t, "@wordlist cat", "@wordlist cat", 3, 2),
				v091TestClose(t, 3, 2), v091TestNamed(t, "wordlist", "content", "cat", 2, 1)),
				"current", [][3]int{{6, 11, 14}})
		})
	}
	t.Run("forbidden nesting", func(t *testing.T) {
		result := v091TestDocument(t, v091TestOpen(t, "text", 2, 1),
			v091TestNamed(t, "wordlist", "content", "cat", 3, 2),
			v091TestText(t, "Read.", "Read.", 3, 2), v091TestClose(t, 3, 2))
		v091TestParserError(t, result, 4, -1, "Объявление wordlist запрещено в text.")
		v091TestRun(t, result, "current", [][3]int{{4, 11, 14}})
	})
	t.Run("branch", func(t *testing.T) {
		v091TestRun(t, v091TestDocument(t, v091TestOpen(t, "variants", 2, 1),
			v091TestNamed(t, "variant", "name", "A", 3, 2),
			v091TestNamed(t, "wordlist", "content", "cat", 4, 3),
			v091TestText(t, "Read.", "Read.", 4, 3),
			v091TestNamed(t, "variant", "name", "B", 3, 2),
			v091TestText(t, "Write.", "Write.", 7, 3), v091TestClose(t, 3, 2)),
			"current", [][3]int{{5, 11, 14}})
	})
}

func TestV091BoundariesAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v091TestDocument(t, v091TestNamed(t, "wordlist", "content", "Italian; Spanish", 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v091TestRun(t, result, "first", [][3]int{{3, 11, 27}})
			if result.Lines[2].HasErrors {
				t.Fatal("recommendation must not set line.hasErrors")
			}
			v091TestRun(t, result, "second", [][3]int{{3, 11, 27}})
		})
	}
	t.Run("each declaration once", func(t *testing.T) {
		v091TestRun(t, v091TestDocument(t,
			v091TestNamed(t, "wordlist", "content", "cat", 2, 1),
			v091TestNamed(t, "wordlist", "content", "cat", 2, 1)),
			"current", [][3]int{{3, 11, 14}, {4, 11, 14}})
	})
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v091TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v091TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v091TestDocument(t)
			result.Document.DSLVersion = &version
			before := v091TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v091TestRegistry(t))
			if err == nil || checks != nil || v091TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v091TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v091TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v091TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v091TestHeading(t, "#", "Lesson"),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v091TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v091TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v091TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		element := &line.Elements[index]
		scope, start, end = diagnostics.ScopeElement, element.Start, element.End
		element.ErrorIDs = []string{id}
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
}

func v091TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v091TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v091TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v091TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v091TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v091TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v091TestLine(t, raw, "invalid", parent, depth, v091TestUnparsed(t, raw, 1))
}

func v091TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v091TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v091TestElement(t, "tag", "@"+tag, tag, 1), v091TestElement(t, kind, value, value, len(tag)+3))
}

func v091TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v091TestLine(t, marker+" "+title, "heading", 0, 0,
		v091TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v091TestElement(t, "title", title, title, len(marker)+2))
}

func v091TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v091TestLine(t, raw, "content", parent, depth, v091TestElement(t, "content", raw, value, 1))
}

func v091TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v091TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v091TestElement(t, "tag", "@"+tag, tag, 1), v091TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v091TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v091TestLine(t, "}", "block-end", parent, depth, v091TestElement(t, "block-close", "}", "}", 1))
}

func v091TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v091TestRun выполняет полный каталог и сравнивает новые V091 с явными
// тройками строка/начало/конец. Правые границы исключаются.
func v091TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v091TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v091TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v091TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V091" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V091 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "Однострочный список слов не заканчивается точкой." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V091 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v091TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v091TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v091TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v091TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var source strings.Builder
	for _, line := range result.Lines {
		source.WriteString(line.Raw)
		source.WriteString(string(line.LineEnding))
	}
	if source.String() != want {
		t.Fatalf("source = %q, want %q", source.String(), want)
	}
}

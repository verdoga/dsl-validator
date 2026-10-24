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

func TestV056Registration(t *testing.T) {
	definitions, err := v056TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V056" {
			continue
		}
		count++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "В документе есть редакторский комментарий @editor." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V056 definitions = %d, want 1", count)
	}
}

func TestV056ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Версия и заголовок добавляют обязательный контекст перед эталоном.
		result := v056TestDocument(t, v056TestLine(t, "@note Read carefully.", "tag", 2, 1,
			v056TestElement(t, "tag", "@note", "note", 1),
			v056TestElement(t, "content", "Read carefully.", "Read carefully.", 7)))
		v056TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@note Read carefully.")
		v056TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		// Неизменённый пример размещён между версией и заголовком:
		// DSL 2.8.9 разрешает здесь обе формы editor.
		result := v056TestResult(t, []model.Line{
			v056TestVersion(t),
			v056TestLine(t, "@editor Check the wording.", "tag", 0, 0,
				v056TestElement(t, "tag", "@editor", "editor", 1),
				v056TestElement(t, "content", "Check the wording.", "Check the wording.", 9)),
			v056TestOpen(t, "editor", "editor", 0, 0),
			v056TestText(t, "Compare with the source.", "Compare with the source.", 3, 1),
			v056TestClose(t, 3, 1), v056TestHeading(t),
		})
		v056TestSource(t, result, "@dsl-version 1.2\n@editor Check the wording.\n@editor {\nCompare with the source.\n}\n# Lesson")
		v056TestRun(t, result, "current", [][3]int{{2, 1, 8}, {3, 1, 8}})
	})
}

func TestV056ParsedDeclarations(t *testing.T) {
	for _, tc := range []struct {
		raw, spelling, content string
		start, contentStart    int
		want                   [][3]int
	}{
		{"@editor Check.", "@editor", "Check.", 1, 9, [][3]int{{3, 1, 8}}},
		{"@EdItOr Check.", "@EdItOr", "Check.", 1, 9, [][3]int{{3, 1, 8}}},
		{" \t@editor Check.", "@editor", "Check.", 3, 11, [][3]int{{3, 3, 10}}},
		{"@editor Mention @editor here.", "@editor", "Mention @editor here.", 1, 9, [][3]int{{3, 1, 8}}},
		{"@editor Проверь 😀e\u0301.", "@editor", "Проверь 😀e\u0301.", 1, 9, [][3]int{{3, 1, 8}}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v056TestRun(t, v056TestDocument(t, v056TestLine(t, tc.raw, "tag", 2, 1,
				v056TestElement(t, "tag", tc.spelling, "editor", tc.start),
				v056TestElement(t, "content", tc.content, tc.content, tc.contentStart))), "current", tc.want)
		})
	}
	t.Run("one finding for parsed erroneous declaration", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestLine(t, "@editor\tCheck.", "invalid", 2, 1,
			v056TestElement(t, "tag", "@editor", "editor", 1), v056TestUnparsed(t, "\tCheck.", 8))),
			"current", [][3]int{{3, 1, 8}})
	})
	t.Run("two declarations", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestComment(t, 2, 1), v056TestComment(t, 2, 1)),
			"current", [][3]int{{3, 1, 8}, {4, 1, 8}})
	})
}

func TestV056RawFallback(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want [][3]int
	}{
		{"@editor", [][3]int{{3, 1, 8}}},
		{"@editor ", [][3]int{{3, 1, 8}}},
		{" \t@EDITOR", [][3]int{{3, 3, 10}}},
		{"@editor\tCheck.", [][3]int{{3, 1, 8}}},
		{"@editor\u00a0Check.", [][3]int{{3, 1, 8}}},
		{"@editor\u2003Check.", [][3]int{{3, 1, 8}}},
		{"@editorial", nil},
		{"@editor-name", nil},
		{"@editor}", nil},
		{"@edito", nil},
		{"@editor\u200bCheck.", nil},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v056TestRun(t, v056TestDocument(t,
				v056TestLine(t, tc.raw, "invalid", 2, 1, v056TestUnparsed(t, tc.raw, 1))), "current", tc.want)
		})
	}
}

func TestV056NoDeclaration(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"Read carefully.", "Read carefully."},
		{"Use @editor here.", "Use @editor here."},
		{`\@editor Check.`, "@editor Check."},
		{" \t\\@editor Check.", "@editor Check."},
		{`\\@editor Check.`, `\@editor Check.`},
		{"\u00a0@editor Check.", "\u00a0@editor Check."},
		{" \t\u2003@editor Check.", "\u2003@editor Check."},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v056TestRun(t, v056TestDocument(t, v056TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("mention in another tag", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestLine(t, "@note Mention @editor here.", "tag", 2, 1,
			v056TestElement(t, "tag", "@note", "note", 1),
			v056TestElement(t, "content", "Mention @editor here.", "Mention @editor here.", 7))), "current", nil)
	})
	t.Run("heading and blanks", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestLine(t, "", "blank", 2, 1),
			v056TestLine(t, " \t", "blank", 2, 1)), "current", nil)
	})
}

func TestV056LiteralBlocks(t *testing.T) {
	for _, tc := range []struct {
		spelling, name string
		broken         bool
		want           [][3]int
		after          [][3]int
	}{
		{"editor", "editor", false, [][3]int{{3, 1, 8}}, [][3]int{{3, 1, 8}, {7, 1, 8}}},
		{"EDITOR", "editor", false, [][3]int{{3, 1, 8}}, [][3]int{{3, 1, 8}, {7, 1, 8}}},
		{"multifill", "multifill", false, nil, [][3]int{{7, 1, 8}}},
		{"MultiFill", "multifill", false, nil, [][3]int{{7, 1, 8}}},
		{"editor", "editor", true, [][3]int{{3, 1, 8}}, [][3]int{{3, 1, 8}, {7, 1, 8}}},
		{"MULTIFILL", "multifill", true, nil, [][3]int{{7, 1, 8}}},
	} {
		t.Run(tc.spelling+"/broken="+strconv.FormatBool(tc.broken), func(t *testing.T) {
			open := v056TestOpen(t, tc.spelling, tc.name, 2, 1)
			if tc.broken {
				// Частичный разбор сохранил границу блока, но не выделил tag.
				// unparsed допустим и при типе block-start (уточнение A).
				open = v056TestLine(t, "@"+tc.spelling+"{", "block-start", 2, 1,
					v056TestUnparsed(t, "@"+tc.spelling, 1),
					v056TestElement(t, "block-open", "{", "{", len(tc.spelling)+2))
			}
			body := []model.Line{open,
				v056TestText(t, "@editor Check.", "@editor Check.", 3, 2),
				v056TestText(t, "@editor{", "@editor{", 3, 2), v056TestClose(t, 3, 2)}
			v056TestRun(t, v056TestDocument(t, body...), "current", tc.want)
			v056TestRun(t, v056TestDocument(t, append(body, v056TestComment(t, 2, 1))...), "current", tc.after)
		})
	}
	t.Run("ordinary multiline comment", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestOpen(t, "editor", "editor", 2, 1),
			v056TestText(t, "First.", "First.", 3, 2), v056TestLine(t, "", "blank", 3, 2),
			v056TestText(t, "Last.", "Last.", 3, 2), v056TestClose(t, 3, 2)), "current", [][3]int{{3, 1, 8}})
	})
}

func TestV056Placement(t *testing.T) {
	task := v056TestLine(t, "@task a", "tag", 2, 1, v056TestElement(t, "tag", "@task", "task", 1),
		v056TestElement(t, "identifier", "a", "a", 7))
	instruction := v056TestLine(t, "@instr Read.", "tag", 3, 2,
		v056TestElement(t, "tag", "@instr", "instr", 1), v056TestElement(t, "content", "Read.", "Read.", 8))
	t.Run("comment in task", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, task, instruction, v056TestComment(t, 3, 2)), "current", [][3]int{{5, 1, 8}})
	})
	for _, tc := range []struct {
		name string
		want [][3]int
	}{
		{"editor", [][3]int{{5, 1, 8}}}, {"multifill", nil},
	} {
		t.Run("literal block in task "+tc.name, func(t *testing.T) {
			v056TestRun(t, v056TestDocument(t, task, instruction, v056TestOpen(t, tc.name, tc.name, 3, 2),
				v056TestText(t, "@editor Check.", "@editor Check.", 5, 3), v056TestClose(t, 5, 3)), "current", tc.want)
		})
	}
	t.Run("forbidden comment in text", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestOpen(t, "text", "text", 2, 1),
			v056TestComment(t, 3, 2), v056TestText(t, "Read.", "Read.", 3, 2), v056TestClose(t, 3, 2)),
			"current", [][3]int{{4, 1, 8}})
	})
	t.Run("unparsed forbidden comment in text", func(t *testing.T) {
		v056TestRun(t, v056TestDocument(t, v056TestOpen(t, "text", "text", 2, 1),
			v056TestLine(t, "@editor Check.", "invalid", 3, 2, v056TestUnparsed(t, "@editor Check.", 1)),
			v056TestText(t, "Read.", "Read.", 3, 2), v056TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 8}})
	})
}

func TestV056EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v056TestDocument(t, v056TestComment(t, 2, 1))
			for i := range result.Lines {
				if ending != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = ending
				}
			}
			v056TestRun(t, result, "first", [][3]int{{3, 1, 8}})
			v056TestRun(t, result, "second", [][3]int{{3, 1, 8}})
		})
	}
}

func TestV056EmptyAndVersions(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("known count "+strconv.FormatBool(known), func(t *testing.T) {
			result := v056TestResult(t, []model.Line{})
			result.Document.Metadata.Title = nil
			if !known {
				result.Document.LineCount = nil
			}
			v056TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v056TestResult(t, []model.Line{})
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			result.Document.DSLVersion = &version
			before := v056TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v056TestRegistry(t))
			if err == nil || checks != nil || v056TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v056TestDocument добавляет версию и заголовок перед явно заданным телом.
func v056TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v056TestResult(t, append([]model.Line{v056TestVersion(t), v056TestHeading(t)}, body...))
}

// v056TestResult нумерует готовые строки без разбора Raw. Для unparsed создаёт
// обязательную исходную ошибку и ссылки; это не ожидаемые срабатывания V056.
func v056TestResult(t *testing.T, lines []model.Line) *model.Result {
	t.Helper()
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if count > 0 && lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		line := &lines[i]
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
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

func v056TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v056TestElement задаёт исходный диапазон элемента, исключая правую границу.
func v056TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v056TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v056TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v056TestVersion(t *testing.T) model.Line {
	t.Helper()
	return v056TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v056TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v056TestElement(t, "version", "1.2", "1.2", 14))
}

func v056TestHeading(t *testing.T) model.Line {
	t.Helper()
	return v056TestLine(t, "# Lesson", "heading", 0, 0,
		v056TestElement(t, "heading-level", "#", "1", 1), v056TestElement(t, "title", "Lesson", "Lesson", 3))
}

func v056TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v056TestLine(t, raw, "content", parent, depth, v056TestElement(t, "content", raw, value, 1))
}

func v056TestComment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v056TestLine(t, "@editor Check.", "tag", parent, depth,
		v056TestElement(t, "tag", "@editor", "editor", 1), v056TestElement(t, "content", "Check.", "Check.", 9))
}

func v056TestOpen(t *testing.T, spelling, name string, parent, depth int) model.Line {
	t.Helper()
	return v056TestLine(t, "@"+spelling+" {", "block-start", parent, depth,
		v056TestElement(t, "tag", "@"+spelling, name, 1), v056TestElement(t, "block-open", "{", "{", len(spelling)+3))
}

func v056TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v056TestLine(t, "}", "block-end", parent, depth, v056TestElement(t, "block-close", "}", "}", 1))
}

func v056TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v056TestRun выполняет весь штатный каталог и сравнивает только новые V056
// с явными тройками строка/начало/конец. Правая граница исключается.
func v056TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v056TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v056TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v056TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V056" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V056 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "В документе есть редакторский комментарий @editor." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V056 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всего каталога.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v056TestSnapshot(t, result)), &restored); err != nil {
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
	if v056TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v056TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v056TestSource(t *testing.T, result *model.Result, want string) {
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

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

func TestV069Registration(t *testing.T) {
	definitions, err := v069TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V069" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У @step отсутствует название этапа." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V069 definitions = %d, want 1", count)
	}
}

func TestV069ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		first        model.Line
		want         [][3]int
	}{
		{"reference", "@dsl-version 1.2\n# Lesson\n@task practice\n@step Read\nRead.\n@step Write\nWrite.",
			v069TestNamed(t, "step", "title", "Read", 3, 2), nil},
		{"example", "@dsl-version 1.2\n# Lesson\n@task practice\n@step\nRead.\n@step Write\nWrite.",
			v069TestInvalid(t, "@step", 3, 2), [][3]int{{4, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Перед неизменёнными фрагментами добавлены только версия и заголовок.
			result := v069TestDocument(t,
				v069TestNamed(t, "task", "identifier", "practice", 2, 1), tc.first,
				v069TestText(t, "Read.", "Read.", 4, 3),
				v069TestNamed(t, "step", "title", "Write", 3, 2), v069TestText(t, "Write.", "Write.", 6, 3))
			v069TestSource(t, result, tc.source)
			v069TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV069MissingTitles(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"tag without title", v069TestTag(t, "step", 2, 1), [][3]int{{3, 1, 6}}},
		{"invalid without tag", v069TestInvalid(t, "@step", 2, 1), [][3]int{{3, 1, 6}}},
		{"tag line with unparsed", v069TestLine(t, "@step", "tag", 2, 1,
			v069TestUnparsed(t, "@step", 1)), [][3]int{{3, 1, 6}}},
		{"partial invalid", v069TestLine(t, "@step \t", "invalid", 2, 1,
			v069TestElement(t, "tag", "@step", "step", 1), v069TestUnparsed(t, " \t", 6)), [][3]int{{3, 1, 6}}},
		{"parsed case and indentation", v069TestLine(t, " \t@StEp", "tag", 2, 1,
			v069TestElement(t, "tag", "@StEp", "step", 3)), [][3]int{{3, 3, 8}}},
		{"raw case and indentation", v069TestInvalid(t, " \t@StEp", 2, 1), [][3]int{{3, 3, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v069TestDocument(t, tc.line, v069TestText(t, "Read.", "Read.", 3, 2))
			// Отсутствие title и размещение вне task — реальные ошибки DSL.
			if len(result.Diagnostics) == 0 {
				v069TestParserError(t, result, 3, -1, "У этапа вне задания отсутствует название.")
			}
			v069TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("each declaration reports once", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t,
			v069TestNamed(t, "task", "identifier", "practice", 2, 1),
			v069TestInvalid(t, "@step", 3, 2), v069TestText(t, "Read.", "Read.", 4, 3),
			v069TestInvalid(t, "@step", 3, 2), v069TestText(t, "Write.", "Write.", 6, 3)),
			"current", [][3]int{{4, 1, 6}, {6, 1, 6}})
	})
}

func TestV069Whitespace(t *testing.T) {
	// Полный White_Space без CR/LF: эти два символа не входят в Raw
	// по контракту. Окончания физических строк проверяются отдельно.
	spaces := []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'}
	for _, space := range spaces {
		t.Run(strconv.QuoteRune(space), func(t *testing.T) {
			v069TestRun(t, v069TestDocument(t, v069TestInvalid(t, "@step"+string(space), 2, 1)),
				"current", [][3]int{{3, 1, 6}})
		})
	}
	t.Run("mixed whitespace", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t, v069TestInvalid(t, "@step"+string(spaces), 2, 1)),
			"current", [][3]int{{3, 1, 6}})
	})
}

func TestV069NonemptyTitles(t *testing.T) {
	for _, title := range []string{"Read", "1", "Читай 😀", "Read  carefully", "\u200b", "\ufeff", "\u001c", `\{`} {
		t.Run(strconv.Quote(title), func(t *testing.T) {
			value := title
			if title == `\{` {
				value = "{"
			}
			v069TestRun(t, v069TestDocument(t,
				v069TestNamed(t, "task", "identifier", "practice", 2, 1),
				v069TestLine(t, "@step "+title, "tag", 3, 2,
					v069TestElement(t, "tag", "@step", "step", 1), v069TestElement(t, "title", title, value, 7)),
				v069TestText(t, "Read.", "Read.", 4, 3)), "current", nil)
		})
	}
	t.Run("repeated titles case and internal spaces", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t,
			v069TestNamed(t, "task", "identifier", "practice", 2, 1),
			v069TestNamed(t, "step", "title", "Read now", 3, 2), v069TestText(t, "Read.", "Read.", 4, 3),
			v069TestNamed(t, "step", "title", "Read now", 3, 2), v069TestText(t, "Read.", "Read.", 6, 3),
			v069TestNamed(t, "step", "title", "READ  now", 3, 2), v069TestText(t, "Read.", "Read.", 8, 3)),
			"current", nil)
	})
	for _, tail := range []string{"{", "}", " {", " }", "\tRead", "\u00a0Read", " <invalid>"} {
		for _, parsed := range []bool{false, true} {
			t.Run(strconv.Quote(tail)+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v069TestInvalid(t, "@step"+tail, 3, 2)
				if parsed {
					line = v069TestLine(t, "@step"+tail, "invalid", 3, 2,
						v069TestElement(t, "tag", "@step", "step", 1), v069TestUnparsed(t, tail, 6))
				}
				// Ошибочный разделитель, скобка или запрещённая разметка
				// остаются непустым аргументом, даже без элемента title.
				v069TestRun(t, v069TestDocument(t,
					v069TestNamed(t, "task", "identifier", "practice", 2, 1), line,
					v069TestText(t, "Read.", "Read.", 4, 3)), "current", nil)
			})
		}
	}
}

func TestV069NonDeclarations(t *testing.T) {
	for _, raw := range []string{"@stepExtra", "@step-name", "@ste", "@step\u200b", "@ſtep"} {
		t.Run(raw, func(t *testing.T) {
			// По DSL 1.11 имя приводится к нижнему регистру:
			// длинная ſ не является вариантом регистра ASCII s.
			v069TestRun(t, v069TestDocument(t, v069TestInvalid(t, raw, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@step`, "@step"}, {`\\@step`, `\@step`},
		{"Mention @step", "Mention @step"}, {"\u00a0@step", "\u00a0@step"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v069TestRun(t, v069TestDocument(t, v069TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("note mention", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t, v069TestNamed(t, "note", "content", "Mention @step", 2, 1)), "current", nil)
	})
}

func TestV069Contexts(t *testing.T) {
	t.Run("branch without task", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t,
			v069TestOpen(t, "variants", 2, 1), v069TestNamed(t, "variant", "name", "A", 3, 2),
			v069TestInvalid(t, "@step", 4, 3), v069TestText(t, "Read.", "Read.", 5, 4),
			v069TestNamed(t, "variant", "name", "B", 3, 2), v069TestText(t, "Write.", "Write.", 7, 3),
			v069TestClose(t, 3, 2)), "current", [][3]int{{5, 1, 6}})
	})
	t.Run("forbidden content nesting", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t,
			v069TestOpen(t, "text", 2, 1), v069TestInvalid(t, "@step", 3, 2),
			v069TestText(t, "Read.", "Read.", 3, 2), v069TestClose(t, 3, 2)),
			"current", [][3]int{{4, 1, 6}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		for _, parsed := range []bool{false, true} {
			t.Run(tag+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				open := v069TestOpen(t, tag, 2, 1)
				if !parsed {
					// Нет обязательного пробела перед { (DSL 1.29).
					// Частичный разбор сохранил блок и его родительство.
					raw := "@" + strings.ToUpper(tag)
					open = v069TestLine(t, raw+"{", "block-start", 2, 1,
						v069TestUnparsed(t, raw, 1), v069TestElement(t, "block-open", "{", "{", len(raw)+1))
				}
				v069TestRun(t, v069TestDocument(t, open,
					v069TestText(t, "@step", "@step", 3, 2), v069TestClose(t, 3, 2),
					v069TestInvalid(t, "@step", 2, 1)), "current", [][3]int{{6, 1, 6}})
			})
		}
	}
	t.Run("literal ancestor below task", func(t *testing.T) {
		v069TestRun(t, v069TestDocument(t,
			v069TestNamed(t, "task", "identifier", "practice", 2, 1),
			v069TestOpen(t, "editor", 3, 2), v069TestText(t, "@step", "@step", 4, 3),
			v069TestText(t, "@text {", "@text {", 4, 3),
			v069TestText(t, "@step", "@step", 4, 3), v069TestClose(t, 4, 3),
			v069TestInvalid(t, "@step", 3, 2), v069TestText(t, "Read.", "Read.", 9, 3)),
			"current", [][3]int{{9, 1, 6}})
	})
}

func TestV069EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v069TestDocument(t, v069TestInvalid(t, "@step", 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v069TestRun(t, result, "first", [][3]int{{3, 1, 6}})
			v069TestRun(t, result, "second", [][3]int{{3, 1, 6}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v069TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v069TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v069TestDocument(t)
			result.Document.DSLVersion = &version
			before := v069TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v069TestRegistry(t))
			if err == nil || checks != nil || v069TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v069TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v069TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v069TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v069TestHeading(t, "#", "Lesson"),
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
				v069TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v069TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v069TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v069TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v069TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v069TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v069TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v069TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v069TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v069TestLine(t, raw, "invalid", parent, depth, v069TestUnparsed(t, raw, 1))
}

func v069TestTag(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v069TestLine(t, "@"+tag, "tag", parent, depth, v069TestElement(t, "tag", "@"+tag, tag, 1))
}

func v069TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v069TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v069TestElement(t, "tag", "@"+tag, tag, 1), v069TestElement(t, kind, value, value, len(tag)+3))
}

func v069TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v069TestLine(t, marker+" "+title, "heading", 0, 0,
		v069TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v069TestElement(t, "title", title, title, len(marker)+2))
}

func v069TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v069TestLine(t, raw, "content", parent, depth, v069TestElement(t, "content", raw, value, 1))
}

func v069TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v069TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v069TestElement(t, "tag", "@"+tag, tag, 1), v069TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v069TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v069TestLine(t, "}", "block-end", parent, depth, v069TestElement(t, "block-close", "}", "}", 1))
}

func v069TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v069TestRun выполняет полный каталог и сравнивает новые V069 с явными
// тройками строка/начало/конец. Правые границы исключаются.
func v069TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v069TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v069TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v069TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V069" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V069 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "У @step отсутствует название этапа." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V069 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v069TestSnapshot(t, result)), &restored); err != nil {
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
	if v069TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v069TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v069TestSource(t *testing.T, result *model.Result, want string) {
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

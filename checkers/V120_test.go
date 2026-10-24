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

// v120TestExpected задаёт исходный диапазон с исключённым концом и конкретизацию.
type v120TestExpected struct {
	line, start, end int
	detail           string
}

const (
	v120TestNote     = "Сохраните текстовое значение в однострочной форме @note CONTENT, заново учитывая экранирование нового контекста."
	v120TestExample  = "Сохраните текстовое значение в однострочной форме @example CONTENT, заново учитывая экранирование нового контекста."
	v120TestHint     = "Сохраните текстовое значение в однострочной форме @hint CONTENT, заново учитывая экранирование нового контекста."
	v120TestAnswer   = "Сохраните текстовое значение в однострочной форме @answer CONTENT, заново учитывая экранирование нового контекста."
	v120TestWordlist = "Сохраните текстовое значение как один элемент однострочного @wordlist: заново учтите экранирование нового контекста, экранируйте внутренние точки с запятой и оформите завершающую точку."
)

func TestV120Registration(t *testing.T) {
	definitions, err := v120TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V120" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Блок без названия содержит одну строку и допускает однострочную запись." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V120 definitions = %d, want 1", count)
	}
}

func TestV120ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		block          bool
		want           []v120TestExpected
	}{
		{"reference", "@note Read the text carefully.", false, nil},
		{"example", "@note {\nRead the text carefully.\n}", true, []v120TestExpected{{3, 1, 6, v120TestNote}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result *model.Result
			if tc.block {
				result = v120TestBlock(t, "note", v120TestText(t, "Read the text carefully.", "Read the text carefully.", 3, 2))
			} else {
				result = v120TestLesson(t, v120TestLine(t, tc.fragment, "tag", 2, 1,
					v120TestElement(t, "tag", "@note", "note", 1, 6), v120TestElement(t, "content", "Read the text carefully.", "Read the text carefully.", 7, 31)))
			}
			// Добавлены только обязательные версия и заголовок; исходный фрагмент сохранён.
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("fragment changed")
			}
			v120TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV120TagsAndRows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		end    int
		detail string
	}{
		{"example", 9, v120TestExample}, {"wordlist", 10, v120TestWordlist},
		{"instr", 7, "Сохраните текстовое значение в однострочной форме @instr CONTENT, заново учитывая экранирование нового контекста."},
		{"note", 6, v120TestNote}, {"alt", 5, "Сохраните текстовое значение в однострочной форме @alt CONTENT, заново учитывая экранирование нового контекста."},
		{"hint", 6, v120TestHint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v120TestRun(t, v120TestBlock(t, tc.name, v120TestText(t, "Read.", "Read.", 3, 2)), "current", []v120TestExpected{{3, 1, tc.end, tc.detail}})
		})
	}
	text := func(raw, value string) model.Line { return v120TestText(t, raw, value, 3, 2) }
	blank := func(raw string) model.Line { return v120TestLine(t, raw, "blank", 3, 2) }
	for _, tc := range []struct {
		name, tag string
		rows      []model.Line
		want      []v120TestExpected
	}{
		{"empty", "note", nil, nil},
		{"only blanks", "note", []model.Line{blank(""), blank(" \t")}, nil},
		{"two rows", "note", []model.Line{text("One.", "One."), text("Two.", "Two.")}, nil},
		{"blank edges", "note", []model.Line{blank(""), blank(" \t"), text(" \tТекст.\t ", "Текст."), blank("\t")}, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"blank between two rows", "note", []model.Line{text("One.", "One."), blank(""), text("Two.", "Two.")}, nil},
		{"literal leading tag", "note", []model.Line{text(`\@note Text.`, "@note Text.")}, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"escaped braces and slash", "note", []model.Line{text(`\{Текст\} \\`, `{Текст} \`)}, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"Unicode content", "note", []model.Line{text("Я😀", "Я😀")}, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"NBSP not trimmed", "note", []model.Line{text("\u00a0", "\u00a0")}, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"escaped example separator", "example", []model.Line{text(`\---`, "---")}, []v120TestExpected{{3, 1, 9, v120TestExample}}},
		{"ordinary note dashes", "note", []model.Line{text("---", "---")}, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"dashes in example text", "example", []model.Line{text("Before --- after", "Before --- after")}, []v120TestExpected{{3, 1, 9, v120TestExample}}},
		{"example separator", "example", []model.Line{v120TestLine(t, "---", "separator", 3, 2, v120TestElement(t, "content", "---", "---", 1, 4)), text("After.", "After.")}, nil},
		{"unparsed content", "note", []model.Line{v120TestLine(t, "broken", "invalid", 3, 2, v120TestElement(t, "unparsed", "broken", "", 1, 7))}, nil},
		{"single wordlist item with semicolon", "wordlist", []model.Line{text("red; blue", "red; blue")}, []v120TestExpected{{3, 1, 10, v120TestWordlist}}},
		{"wordlist terminal dot", "wordlist", []model.Line{text("blue.", "blue.")}, []v120TestExpected{{3, 1, 10, v120TestWordlist}}},
		{"unknown escape in block wordlist", "wordlist", []model.Line{text(`red\; blue`, `red\; blue`)}, []v120TestExpected{{3, 1, 10, v120TestWordlist}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v120TestRun(t, v120TestBlock(t, tc.tag, tc.rows...), "current", tc.want) })
	}
}

func TestV120OpeningsAndExclusions(t *testing.T) {
	for _, tc := range []struct {
		name, raw               string
		tagStart, tagEnd, brace int
		bad                     bool
		want                    []v120TestExpected
	}{
		{"spaces", "@note   {", 1, 6, 9, false, []v120TestExpected{{3, 1, 6, v120TestNote}}},
		{"case and indentation", "  @NoTe {", 3, 8, 9, false, []v120TestExpected{{3, 3, 8, v120TestNote}}},
		{"no space", "@note{", 1, 6, 6, true, nil},
		{"tab", "@note\t{", 1, 6, 7, true, nil},
		{"mixed gap", "@note \t {", 1, 6, 9, true, nil},
		{"NBSP gap", "@note\u00a0{", 1, 6, 7, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []rune(tc.raw)
			opening := v120TestLine(t, tc.raw, "block-start", 2, 1, v120TestElement(t, "tag", string(raw[tc.tagStart-1:tc.tagEnd-1]), "note", tc.tagStart, tc.tagEnd), v120TestElement(t, "block-open", "{", "{", tc.brace, tc.brace+1))
			result := v120TestLesson(t, opening, v120TestText(t, "Text.", "Text.", 3, 2), v120TestClose(t, 3, 2))
			if tc.bad {
				v120TestParserError(t, result, 3, 3, tc.brace+1, "line")
			}
			v120TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("title", func(t *testing.T) {
		opening := v120TestLine(t, "@note Title {", "block-start", 2, 1, v120TestElement(t, "tag", "@note", "note", 1, 6), v120TestElement(t, "title", "Title", "Title", 7, 12), v120TestElement(t, "block-open", "{", "{", 13, 14))
		v120TestRun(t, v120TestLesson(t, opening, v120TestText(t, "Text.", "Text.", 3, 2), v120TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("unparsed parameter", func(t *testing.T) {
		opening := v120TestLine(t, "@hint Extra {", "block-start", 2, 1, v120TestElement(t, "tag", "@hint", "hint", 1, 6), v120TestElement(t, "unparsed", "Extra", "", 7, 12), v120TestElement(t, "block-open", "{", "{", 13, 14))
		v120TestRun(t, v120TestLesson(t, opening, v120TestText(t, "Text.", "Text.", 3, 2), v120TestClose(t, 3, 2)), "current", nil)
	})
	for _, tag := range []string{"text", "table", "script", "key", "multifill"} {
		t.Run("excluded/"+tag, func(t *testing.T) {
			v120TestRun(t, v120TestBlock(t, tag, v120TestText(t, "Text.", "Text.", 3, 2)), "current", nil)
		})
	}
	t.Run("fragment excluded but nested note checked", func(t *testing.T) {
		opening := v120TestLine(t, "@fragment f {", "block-start", 2, 1, v120TestElement(t, "tag", "@fragment", "fragment", 1, 10), v120TestElement(t, "identifier", "f", "f", 11, 12), v120TestElement(t, "block-open", "{", "{", 13, 14))
		v120TestRun(t, v120TestLesson(t, opening, v120TestText(t, "Text.", "Text.", 3, 2), v120TestClose(t, 3, 2)), "current", nil)
		v120TestRun(t, v120TestLesson(t, opening, v120TestOpen(t, "note", 3, 2), v120TestText(t, "Text.", "Text.", 4, 3), v120TestClose(t, 4, 3), v120TestClose(t, 3, 2)), "current", []v120TestExpected{{4, 1, 6, v120TestNote}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal pseudo block/"+tag, func(t *testing.T) {
			v120TestRun(t, v120TestLesson(t, v120TestOpen(t, tag, 2, 1), v120TestText(t, "@note {", "@note {", 3, 2), v120TestText(t, "Text.", "Text.", 3, 2), v120TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV120NestingAndAnswers(t *testing.T) {
	text := func(raw string, parent, depth int) model.Line { return v120TestText(t, raw, raw, parent, depth) }
	t.Run("inline hint excludes instr", func(t *testing.T) {
		hint := v120TestLine(t, "@hint Help.", "tag", 3, 2, v120TestElement(t, "tag", "@hint", "hint", 1, 6), v120TestElement(t, "content", "Help.", "Help.", 7, 12))
		v120TestRun(t, v120TestLesson(t, v120TestOpen(t, "instr", 2, 1), text("Read.", 3, 2), hint, v120TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("nested hint checked independently", func(t *testing.T) {
		v120TestRun(t, v120TestLesson(t, v120TestOpen(t, "instr", 2, 1), text("Read.", 3, 2), v120TestOpen(t, "hint", 3, 2), text("Help.", 5, 3), v120TestClose(t, 5, 3), v120TestClose(t, 3, 2)), "current", []v120TestExpected{{5, 1, 6, v120TestHint}})
	})
	t.Run("answer follows question", func(t *testing.T) {
		question := v120TestLine(t, "@question Name a colour.", "tag", 2, 1, v120TestElement(t, "tag", "@question", "question", 1, 10), v120TestElement(t, "content", "Name a colour.", "Name a colour.", 11, 25))
		v120TestRun(t, v120TestLesson(t, question, v120TestOpen(t, "answer", 2, 1), text("Blue", 4, 2), v120TestClose(t, 4, 2)), "current", []v120TestExpected{{4, 1, 8, v120TestAnswer}})
	})
	t.Run("answer inside choice", func(t *testing.T) {
		v120TestRun(t, v120TestLesson(t, v120TestOpen(t, "choice", 2, 1), text("Wrong", 3, 2), v120TestOpen(t, "answer", 3, 2), text("Right", 5, 3), v120TestClose(t, 5, 3), v120TestClose(t, 3, 2)), "current", []v120TestExpected{{5, 1, 8, v120TestAnswer}})
	})
	t.Run("same text in two blocks", func(t *testing.T) {
		v120TestRun(t, v120TestLesson(t, v120TestOpen(t, "note", 2, 1), text("Text.", 3, 2), v120TestClose(t, 3, 2), v120TestOpen(t, "note", 2, 1), text("Text.", 6, 2), v120TestClose(t, 6, 2)), "current", []v120TestExpected{{3, 1, 6, v120TestNote}, {6, 1, 6, v120TestNote}})
	})
}

func TestV120TextElements(t *testing.T) {
	t.Run("several content elements in one row", func(t *testing.T) {
		row := v120TestLine(t, "One two", "content", 3, 2, v120TestElement(t, "content", "One ", "One ", 1, 5), v120TestElement(t, "content", "two", "two", 5, 8))
		v120TestRun(t, v120TestBlock(t, "note", row), "current", []v120TestExpected{{3, 1, 6, v120TestNote}})
	})
	t.Run("ordinary placeholder in textual row", func(t *testing.T) {
		// DSL 4.3.3: обычный плейсхолдер допустим в контентном блоке и остаётся видимым.
		// По уточнению пользователя парсер сохраняет его внутри content.
		row := v120TestText(t, "Fill _____.", "Fill _____.", 3, 2)
		v120TestRun(t, v120TestBlock(t, "note", row), "current", []v120TestExpected{{3, 1, 6, v120TestNote}})
	})
	t.Run("placeholder is the entire textual row", func(t *testing.T) {
		row := v120TestText(t, "_____", "_____", 3, 2)
		v120TestRun(t, v120TestBlock(t, "note", row), "current", []v120TestExpected{{3, 1, 6, v120TestNote}})
	})
}

func TestV120Lifecycle(t *testing.T) {
	text := func() model.Line { return v120TestText(t, "Text.", "Text.", 3, 2) }
	want := []v120TestExpected{{3, 1, 6, v120TestNote}}
	t.Run("empty input", func(t *testing.T) {
		result := v120TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v120TestRun(t, result, "current", nil)
	})
	t.Run("no candidates", func(t *testing.T) { v120TestRun(t, v120TestLesson(t), "current", nil) })
	t.Run("unclosed at EOF", func(t *testing.T) {
		result := v120TestLesson(t, v120TestOpen(t, "note", 2, 1), text())
		result.Lines[3].LineEnding = ""
		v120TestParserError(t, result, 3, 4, 6, "block")
		v120TestRun(t, result, "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v120TestBlock(t, "note", text())
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[4].LineEnding = ""
				}
				v120TestRun(t, result, "current", want)
			})
		}
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v120TestBlock(t, "note", text())
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v120TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v120TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v120TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v120TestBlock(t, "note", text())
		second := v120TestBlock(t, "note", text(), text())
		v120TestRun(t, first, "first", want)
		v120TestRun(t, second, "first", nil)
		v120TestRun(t, first, "second", want)
	})
}

// v120TestParserError фиксирует отдельную ошибку вложения или незакрытого блока.
func v120TestParserError(t *testing.T, result *model.Result, startLine, endLine, endColumn int, scope diagnostics.Scope) {
	t.Helper()
	result.Lines[startLine-1].HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-context-error", Source: "parser", DiagnosticCode: "fixture-context-error", SeverityLevel: "error", DiagnosticScope: scope,
		Message: "Ошибка структуры блока.", RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: startLine, Column: 1}, End: model.Position{Line: endLine, Column: endColumn}},
	})
}

// v120TestBlock добавляет указанный блок вокруг явно заданных строк с родителем 3.
func v120TestBlock(t *testing.T, name string, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v120TestOpen(t, name, 2, 1)}, rows...)
	body = append(body, v120TestClose(t, 3, 2))
	return v120TestLesson(t, body...)
}

func v120TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v120TestLine(t, raw, "content", parent, depth,
		v120TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v120TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v120TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v120TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v120TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v120TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v120TestLine(t, "# Lesson", "heading", 0, 0,
			v120TestElement(t, "heading-level", "#", "1", 1, 2), v120TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v120TestOpen задаёт реальное объявление блока с явным родителем.
func v120TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v120TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v120TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v120TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v120TestClose задаёт закрытие указанного блока.
func v120TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v120TestLine(t, "}", "block-end", parent, depth, v120TestElement(t, "block-close", "}", "}", 1, 2))
}

// v120TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v120TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v120TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v120TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v120TestRegistry использует весь штатный каталог регистраций.
func v120TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v120TestRun проверяет только новые V120 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v120TestRun(t *testing.T, result *model.Result, source string, want []v120TestExpected) {
	t.Helper()
	before := v120TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v120TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v120TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V120" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V120 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Блок без названия содержит одну строку и допускает однострочную запись.\nконкретизация: "+position.detail {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V120 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v120TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Производные флаги и ссылки проверены CheckModel. В копии возвращаем
	// прежние значения этих полей и удаляем только новые записи запуска.
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
	if v120TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v120TestSnapshot сохраняет независимое представление модели.
func v120TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

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

type v094TestExpected struct{ line, start, end int }

func TestV094Registration(t *testing.T) {
	definitions, err := v094TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V094" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "У @table отсутствует открывающая скобка блочной формы." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V094 definitions = %d, want 1", count)
	}
}

func TestV094ReferenceAndExample(t *testing.T) {
	reference := v094TestLesson(t, v094TestOpen(t, "table", 2, 1), v094TestText(t, "A | B", "A | B", 3, 2), v094TestClose(t, 3, 2))
	example := v094TestLesson(t, v094TestAttempt(t, "@table A | B", "tag", 1, 2, 1, true))
	for _, tc := range []struct {
		name, raw string
		result    *model.Result
		want      []v094TestExpected
	}{
		{"reference", "@table {\nA | B\n}", reference, nil},
		{"example", "@table A | B", example, []v094TestExpected{{3, 1, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Только обязательные версия и заголовок добавлены перед фрагментом.
			// HTML-запись &#124; в таблице задания означает буквальную |.
			var raw []string
			for _, line := range tc.result.Lines {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.raw {
				t.Fatal("normative fragment changed")
			}
			v094TestRun(t, tc.result, "current", tc.want)
		})
	}
}

func TestV094MissingOpening(t *testing.T) {
	for _, tc := range []struct {
		name, raw  string
		start, end int
	}{
		{"bare name", "@table", 1, 7},
		{"space", "@table ", 1, 7},
		{"tab", "@table\t", 1, 7},
		{"title", "@table Vocabulary", 1, 7},
		{"Unicode title", "@table Название 😀", 1, 7},
		{"escaped opening", `@table \{`, 1, 7},
		{"escaped braces", `@table \{Title\}`, 1, 7},
		{"only closing", "@table }", 1, 7},
		{"escaped closing", `@table \}`, 1, 7},
		{"three backslashes", `@table \\\{`, 1, 7},
		{"final backslash", `@table \`, 1, 7},
		{"paired backslashes alone", `@table \\`, 1, 7},
		{"several escaped openings", `@table \{ A \{ B`, 1, 7},
		{"case and indent", "  @TaBlE Title", 3, 9},
		{"tab indentation", "\t@TABLE Title", 2, 8},
	} {
		for _, representation := range []string{"tag", "invalid with tag", "invalid without tag"} {
			t.Run(tc.name+"/"+representation, func(t *testing.T) {
				kind := model.LineTypeInvalid
				if representation == "tag" {
					kind = model.LineTypeTag
				}
				line := v094TestAttempt(t, tc.raw, kind, tc.start, 2, 1, representation != "invalid without tag")
				v094TestRun(t, v094TestLesson(t, line), "current", []v094TestExpected{{3, tc.start, tc.end}})
			})
		}
	}
	t.Run("semantic brace is still escaped in Raw", func(t *testing.T) {
		line := v094TestLine(t, `@table \{`, "tag", 2, 1,
			v094TestElement(t, "tag", "@table", "table", 1, 7), v094TestElement(t, "title", `\{`, "{", 8, 10))
		v094TestRun(t, v094TestLesson(t, line), "current", []v094TestExpected{{3, 1, 7}})
	})
	t.Run("opening on following line does not count", func(t *testing.T) {
		result := v094TestLesson(t, v094TestAttempt(t, "@table", "tag", 1, 2, 1, true),
			v094TestAttempt(t, "{", "invalid", 1, 2, 1, false), v094TestText(t, "A | B", "A | B", 2, 1),
			v094TestAttempt(t, "}", "invalid", 1, 2, 1, false))
		v094TestRun(t, result, "current", []v094TestExpected{{3, 1, 7}})
	})
}

func TestV094PresentOpening(t *testing.T) {
	// Позиции и значения заголовков выписаны независимо от проверяемого кода.
	for _, tc := range []struct {
		name, raw, titleRaw, titleValue string
		open                            int
	}{
		{"without title", "@table {", "", "", 8},
		{"title", "@table Vocabulary {", "Vocabulary", "Vocabulary", 19},
		{"two backslashes", `@table \\ {`, `\\`, `\`, 11},
		{"paired backslashes next to opening", `@table \\{`, `\\`, `\`, 10},
		{"four backslashes", `@table \\\\{`, `\\\\`, `\\`, 12},
		{"escaped opening before real one", `@table \{ title {`, `\{ title`, `{ title`, 17},
		{"escaped closing before opening", `@table \} {`, `\}`, `}`, 11},
		{"backslash before ordinary character", `@table \q{`, `\q`, `\q`, 10},
		{"joined opening", "@table{", "", "", 7},
	} {
		t.Run("parsed/"+tc.name, func(t *testing.T) {
			elements := []model.Element{v094TestElement(t, "tag", "@table", "table", 1, 7)}
			if tc.titleRaw != "" {
				elements = append(elements, v094TestElement(t, "title", tc.titleRaw, tc.titleValue, 8, 8+utf8.RuneCountInString(tc.titleRaw)))
			}
			elements = append(elements, v094TestElement(t, "block-open", "{", "{", tc.open, tc.open+1))
			result := v094TestLesson(t, v094TestLine(t, tc.raw, "block-start", 2, 1, elements...),
				v094TestText(t, "A | B", "A | B", 3, 2), v094TestClose(t, 3, 2))
			v094TestRun(t, result, "current", nil)
		})
	}
	for _, raw := range []string{
		"@table Title {", "@table{", "@table { tail", "@table { A }", "@table { {",
		`@table "{"`, `@table _____{A}`, `@table \\{`, `@table \\\\{`, `@table \q{`, `@table \{ A {`,
	} {
		for _, recognized := range []bool{false, true} {
			t.Run("unparsed/"+raw+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				// Форма и парность могут быть ошибочны, но Raw содержит открытие.
				// Не создаём block-start/end для неразобранной строки.
				v094TestRun(t, v094TestLesson(t, v094TestAttempt(t, raw, "invalid", 1, 2, 1, recognized)), "current", nil)
			})
		}
	}
	t.Run("unclosed parsed block", func(t *testing.T) {
		result := v094TestLesson(t, v094TestOpen(t, "table", 2, 1), v094TestText(t, "A | B", "A | B", 3, 2))
		result.Lines[2].HasErrors, result.Document.HasErrors = true, true
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "unclosed", Source: "parser", DiagnosticCode: "fixture-unclosed",
			SeverityLevel: "error", DiagnosticScope: "block", Message: "Блок не закрыт.", RelatedLocations: []model.Location{},
			Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 9}}})
		v094TestRun(t, result, "current", nil)
	})
}

func TestV094NamesAndContexts(t *testing.T) {
	for _, raw := range []string{"@tableExtra", "@table1", "@table-name", "@tableЯ", "@table!", "@table\u200b", "@", "@note", "\u00a0@table"} {
		t.Run("not table/"+strconv.Quote(raw), func(t *testing.T) {
			v094TestRun(t, v094TestLesson(t, v094TestAttempt(t, raw, "invalid", 1, 2, 1, false)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@table`, "@table"}, {"Text @table", "Text @table"}, {`\\@table`, `\@table`},
	} {
		t.Run("text/"+tc.raw, func(t *testing.T) {
			v094TestRun(t, v094TestLesson(t, v094TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("inside fragment", func(t *testing.T) {
		v094TestRun(t, v094TestLesson(t, v094TestFragment(t, 2, 1),
			v094TestAttempt(t, "@table Title", "tag", 1, 3, 2, true), v094TestClose(t, 3, 2)),
			"current", []v094TestExpected{{4, 1, 7}})
	})
	t.Run("forbidden nested declaration still checked", func(t *testing.T) {
		v094TestRun(t, v094TestLesson(t, v094TestFragment(t, 2, 1), v094TestOpen(t, "text", 3, 2),
			v094TestAttempt(t, "@table Title", "invalid", 1, 4, 3, false), v094TestClose(t, 4, 3), v094TestClose(t, 3, 2)),
			"current", []v094TestExpected{{5, 1, 7}})
	})
	for _, name := range []string{"editor", "multifill"} {
		for _, recognized := range []bool{false, true} {
			t.Run("literal/"+name+"/parent tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				opening := v094TestOpen(t, name, 2, 1)
				if !recognized {
					// Имя родительского блока может сохраняться лишь в unparsed.
					opening.Elements[0].ElementType = "unparsed"
				}
				v094TestRun(t, v094TestLesson(t, opening, v094TestText(t, "@table", "@table", 3, 2),
					v094TestText(t, `@TABLE \{`, `@TABLE {`, 3, 2), v094TestClose(t, 3, 2)), "current", nil)
			})
		}
	}
	t.Run("literal context ends with block", func(t *testing.T) {
		v094TestRun(t, v094TestLesson(t, v094TestOpen(t, "editor", 2, 1), v094TestText(t, "@table", "@table", 3, 2),
			v094TestClose(t, 3, 2), v094TestAttempt(t, "@table", "tag", 1, 2, 1, true)), "current", []v094TestExpected{{6, 1, 7}})
	})
	t.Run("multiple declarations", func(t *testing.T) {
		v094TestRun(t, v094TestLesson(t, v094TestAttempt(t, "@table A", "tag", 1, 2, 1, true),
			v094TestAttempt(t, "@table B", "invalid", 1, 2, 1, false)), "current", []v094TestExpected{{3, 1, 7}, {4, 1, 7}})
	})
}

func TestV094VersionsAndLifecycle(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v094TestLesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v094TestRun(t, result, "current", nil)
	})
	t.Run("blank lines", func(t *testing.T) {
		v094TestRun(t, v094TestLesson(t, v094TestLine(t, "", "blank", 2, 1), v094TestLine(t, " \t", "blank", 2, 1)), "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run("EOL/"+strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v094TestLesson(t, v094TestAttempt(t, "@table Название 😀", "tag", 1, 2, 1, true))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[2].LineEnding = ""
				}
				v094TestRun(t, result, "current", []v094TestExpected{{3, 1, 7}})
			})
		}
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v094TestLesson(t, v094TestAttempt(t, "@table", "tag", 1, 2, 1, true))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v094TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, v094TestRegistry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v094TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed input")
			}
		})
	}
	t.Run("independent runs", func(t *testing.T) {
		first := v094TestLesson(t, v094TestAttempt(t, "@table", "tag", 1, 2, 1, true))
		second := v094TestLesson(t, v094TestOpen(t, "table", 2, 1), v094TestText(t, "A | B", "A | B", 3, 2), v094TestClose(t, 3, 2))
		v094TestRun(t, first, "first", []v094TestExpected{{3, 1, 7}})
		v094TestRun(t, second, "first", nil)
		v094TestRun(t, first, "second", []v094TestExpected{{3, 1, 7}})
	})
}

// v094TestAttempt задаёт явно выбранную форму ошибочного объявления.
// recognized оставляет tag и ошибочный хвост; false сохраняет весь Raw в unparsed.
func v094TestAttempt(t *testing.T, raw string, kind model.LineType, start, parent, depth int, recognized bool) model.Line {
	t.Helper()
	runes := []rune(raw)
	if !recognized {
		return v094TestLine(t, raw, kind, parent, depth, v094TestElement(t, "unparsed", raw, "", 1, len(runes)+1))
	}
	end := start + 6
	return v094TestLine(t, raw, kind, parent, depth,
		v094TestElement(t, "tag", string(runes[start-1:end-1]), "table", start, end),
		v094TestElement(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

func v094TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v094TestLine(t, raw, "content", parent, depth,
		v094TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v094TestFragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v094TestLine(t, "@fragment f {", "block-start", parent, depth,
		v094TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v094TestElement(t, "identifier", "f", "f", 11, 12), v094TestElement(t, "block-open", "{", "{", 13, 14))
}

// v094TestLesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v094TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v094TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v094TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v094TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v094TestLine(t, "# Lesson", "heading", 0, 0,
			v094TestElement(t, "heading-level", "#", "1", 1, 2), v094TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v094TestOpen задаёт реальное объявление блока с явным родителем.
func v094TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v094TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v094TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v094TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v094TestClose задаёт закрытие указанного блока.
func v094TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v094TestLine(t, "}", "block-end", parent, depth, v094TestElement(t, "block-close", "}", "}", 1, 2))
}

// v094TestLine сохраняет явно заданную структуру, не классифицируя исходный текст.
func v094TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v094TestElement задаёт исходные Unicode-колонки с исключающим концом.
func v094TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v094TestRegistry использует весь штатный каталог регистраций.
func v094TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v094TestRun проверяет только новые V094 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v094TestRun(t *testing.T, result *model.Result, source string, want []v094TestExpected) {
	t.Helper()
	before := v094TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v094TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v094TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V094" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V094 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "У @table отсутствует открывающая скобка блочной формы." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V094 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v094TestSnapshot(t, result)), &restored); err != nil {
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
	if v094TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v094TestSnapshot сохраняет независимое представление модели.
func v094TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

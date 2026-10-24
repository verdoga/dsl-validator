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

func TestV086Registration(t *testing.T) {
	definitions, err := v086Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V086" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "После контентного тега отсутствует обязательное значение." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V086 definitions = %d, want 1", count)
	}
}

type v086Expected struct {
	line, start, end int
	detail           string
}

func TestV086ReferenceAndExample(t *testing.T) {
	// К дословным фрагментам добавлены только @dsl-version 1.2 и # Lesson.
	reference := v086Tagged(t, "example", "She lives in London.", "content", 2, 1)
	if reference.Raw != "@example She lives in London." {
		t.Fatal("reference changed")
	}
	v086Run(t, v086Lesson(t, reference), "current", nil)
	v086Run(t, v086Lesson(t, v086Attempt(t, "@example", "example", 1, 2, 1)), "current",
		[]v086Expected{{3, 1, 9, "@example: отсутствует текст."}})
}

func TestV086TagsAndWhitespace(t *testing.T) {
	// Множество White_Space выписано независимо от проверяемого кода.
	// CR/LF отсутствуют в Raw; они проверяются как окончания строк.
	tails := []string{"", " ", "\t", " \t\u00a0", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"}
	for _, tc := range []struct {
		name, detail string
		end          int
	}{
		{"example", "@example: отсутствует текст.", 9},
		{"wordlist", "@wordlist: отсутствует список слов.", 10},
		{"instr", "@instr: отсутствует текст.", 7},
		{"note", "@note: отсутствует текст.", 6},
		{"alt", "@alt: отсутствует текст.", 5},
		{"hint", "@hint: отсутствует текст.", 6},
		{"include", "@include: отсутствует ID включения.", 9},
	} {
		for _, tail := range tails {
			for _, representation := range []string{"raw", "tag", "invalid with tag"} {
				t.Run(tc.name+"/"+strconv.Quote(tail)+"/"+representation, func(t *testing.T) {
					raw := "@" + tc.name + tail
					line := v086Invalid(t, raw, 2, 1)
					if representation != "raw" {
						line = v086Attempt(t, raw, tc.name, 1, 2, 1)
						if representation == "tag" {
							line.LineType = "tag"
						}
					}
					v086Run(t, v086Lesson(t, line), "current", []v086Expected{{3, 1, tc.end, tc.detail}})
				})
			}
		}
	}
}

func TestV086NonemptyValues(t *testing.T) {
	for _, name := range []string{"example", "wordlist", "instr", "note", "alt", "hint", "include"} {
		t.Run(name, func(t *testing.T) {
			argument, kind := "Read.", model.ElementTypeContent
			if name == "wordlist" {
				argument = "apple; pear."
			}
			if name == "include" {
				argument, kind = "f", model.ElementTypeIdentifier
			}
			body := []model.Line{v086Tagged(t, name, argument, kind, 2, 1)}
			if name == "include" {
				body = append(body, v086Fragment(t, 2, 1), v086Text(t, "Read.", "Read.", 4, 2), v086Close(t, 4, 2))
			}
			v086Run(t, v086Lesson(t, body...), "current", nil)
		})
	}
	for _, value := range []string{"\u200b", "\ufeff", "Привет 😀"} {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			v086Run(t, v086Lesson(t, v086Tagged(t, "example", value, "content", 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, name string }{
		{"@wordlist ;", "wordlist"}, {"@include \"\"", "include"},
		{"@example \\", "example"}, {"@example\tRead.", "example"},
	} {
		t.Run("nonempty unparsed/"+tc.raw, func(t *testing.T) {
			v086Run(t, v086Lesson(t, v086Attempt(t, tc.raw, tc.name, 1, 2, 1)), "current", nil)
		})
	}
	t.Run("parsed whitespace does not hide unparsed text", func(t *testing.T) {
		line := v086Line(t, "@example \u00a0\\", "invalid", 2, 1,
			v086Element(t, "tag", "@example", "example", 1, 9),
			v086Element(t, "content", "\u00a0", "\u00a0", 10, 11),
			v086Element(t, "unparsed", "\\", "", 11, 12))
		v086Run(t, v086Lesson(t, line), "current", nil)
	})
}

func TestV086RecognitionAndPositions(t *testing.T) {
	for _, recognized := range []bool{false, true} {
		t.Run("mixed case/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
			line := v086Invalid(t, "\t @ExAmPlE\t", 2, 1)
			if recognized {
				line = v086Attempt(t, "\t @ExAmPlE\t", "example", 3, 2, 1)
			}
			v086Run(t, v086Lesson(t, line), "current", []v086Expected{{3, 3, 11, "@ExAmPlE: отсутствует текст."}})
		})
	}
	for _, raw := range []string{"@exampleExtra", "@example.extra", "@", "@example{"} {
		t.Run(raw, func(t *testing.T) {
			v086Run(t, v086Lesson(t, v086Invalid(t, raw, 2, 1)), "current", nil)
		})
	}
	t.Run("escaped and inline pseudo tags", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Text(t, "\\@example", "@example", 2, 1),
			v086Text(t, "Use @example", "Use @example", 2, 1)), "current", nil)
	})
	t.Run("multiple missing values", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Attempt(t, "@example", "example", 1, 2, 1),
			v086Invalid(t, "@include", 2, 1)), "current", []v086Expected{
			{3, 1, 9, "@example: отсутствует текст."}, {4, 1, 9, "@include: отсутствует ID включения."},
		})
	})
}

func TestV086BlocksAndContexts(t *testing.T) {
	for _, name := range []string{"example", "wordlist", "instr", "note", "alt", "hint"} {
		t.Run("block/"+name, func(t *testing.T) {
			v086Run(t, v086Lesson(t, v086Open(t, name, 2, 1), v086Text(t, "Read.", "Read.", 3, 2),
				v086Close(t, 3, 2)), "current", nil)
		})
	}
	t.Run("empty block", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Open(t, "example", 2, 1), v086Close(t, 3, 2)), "current", nil)
	})
	t.Run("allowed nested hint", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Open(t, "instr", 2, 1), v086Text(t, "Read.", "Read.", 3, 2),
			v086Attempt(t, "@hint", "hint", 1, 3, 2), v086Close(t, 3, 2)),
			"current", []v086Expected{{5, 1, 6, "@hint: отсутствует текст."}})
	})
	t.Run("misplaced nested declaration", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Open(t, "text", 2, 1), v086Invalid(t, "@example", 3, 2),
			v086Close(t, 3, 2)), "current", []v086Expected{{4, 1, 9, "@example: отсутствует текст."}})
	})
	for _, name := range []string{"editor", "multifill"} {
		for _, recognized := range []bool{false, true} {
			t.Run("literal/"+name+"/parent tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				opening := v086Open(t, name, 2, 1)
				if !recognized {
					// Табуляция перед { ошибочна (DSL 1.29). Парсер сохранил
					// границу блока, а объявление осталось в unparsed.
					opening = v086Line(t, "@"+name+"\t{", "block-start", 2, 1,
						v086Element(t, "unparsed", "@"+name+"\t", "", 1, len(name)+3),
						v086Element(t, "block-open", "{", "{", len(name)+3, len(name)+4))
				}
				v086Run(t, v086Lesson(t, opening, v086Text(t, "@example", "@example", 3, 2),
					v086Text(t, "@include", "@include", 3, 2), v086Close(t, 3, 2)), "current", nil)
			})
		}
	}
	t.Run("literal exclusion ends at block boundary", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Open(t, "editor", 2, 1), v086Text(t, "@example", "@example", 3, 2),
			v086Close(t, 3, 2), v086Invalid(t, "@example", 2, 1)),
			"current", []v086Expected{{6, 1, 9, "@example: отсутствует текст."}})
	})
}

func TestV086ExcludedTags(t *testing.T) {
	for _, name := range []string{"media", "table", "script", "text", "key", "fragment", "question", "header", "step"} {
		for _, recognized := range []bool{false, true} {
			t.Run(name+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				line := v086Invalid(t, "@"+name, 2, 1)
				if recognized {
					line = v086Attempt(t, "@"+name, name, 1, 2, 1)
				}
				v086Run(t, v086Lesson(t, line), "current", nil)
			})
		}
	}
}

func TestV086EndingsVersionsAndRuns(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v086Lesson(t, v086Invalid(t, "@example", 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[2].LineEnding = ""
				}
				v086Run(t, result, "current", []v086Expected{{3, 1, 9, "@example: отсутствует текст."}})
			})
		}
	}
	t.Run("empty input", func(t *testing.T) {
		result := v086Lesson(t)
		result.Lines = []model.Line{}
		result.Document.LineCount, result.Document.Metadata.Title = nil, nil
		v086Run(t, result, "current", nil)
	})
	t.Run("blank lines", func(t *testing.T) {
		v086Run(t, v086Lesson(t, v086Line(t, "", "blank", 2, 1), v086Line(t, " \t", "blank", 2, 1)), "current", nil)
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v086Lesson(t, v086Invalid(t, "@example", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v086Snapshot(t, result)
			checks, err := validator.Prepare(result, v086Registry(t))
			if err == nil || checks != nil || before != v086Snapshot(t, result) {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
	first := v086Lesson(t, v086Invalid(t, "@example", 2, 1))
	second := v086Lesson(t, v086Tagged(t, "example", "Read.", "content", 2, 1))
	v086Run(t, first, "first", []v086Expected{{3, 1, 9, "@example: отсутствует текст."}})
	v086Run(t, second, "first", nil)
	v086Run(t, first, "second", []v086Expected{{3, 1, 9, "@example: отсутствует текст."}})
}

// v086Attempt задаёт разобранный tag и ошибочный остаток в unparsed.
func v086Attempt(t *testing.T, raw, name string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + len(name) + 1
	return v086Line(t, raw, "invalid", parent, depth,
		v086Element(t, "tag", string(runes[start-1:end-1]), name, start, end),
		v086Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v086Invalid задаёт целиком неразобранную строку без элемента tag.
func v086Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v086Line(t, raw, "invalid", parent, depth,
		v086Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v086Tagged задаёт готовую однострочную конструкцию с непустым аргументом.
func v086Tagged(t *testing.T, name, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	return v086Line(t, "@"+name+" "+argument, "tag", parent, depth,
		v086Element(t, "tag", "@"+name, strings.ToLower(name), 1, len(name)+2),
		v086Element(t, kind, argument, argument, len(name)+3, len(name)+3+utf8.RuneCountInString(argument)))
}

// v086Text задаёт исходное и семантическое содержимое независимо.
func v086Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v086Line(t, raw, "content", parent, depth,
		v086Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v086Fragment задаёт определение f; тело и закрытие добавляются отдельно.
func v086Fragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v086Line(t, "@fragment f {", "block-start", parent, depth,
		v086Element(t, "tag", "@fragment", "fragment", 1, 10),
		v086Element(t, "identifier", "f", "f", 11, 12), v086Element(t, "block-open", "{", "{", 13, 14))
}

// v086Lesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для явно заданных unparsed.
func v086Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v086Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v086Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v086Element(t, "version", "1.2", "1.2", 14, 17)),
		v086Line(t, "# Lesson", "heading", 0, 0,
			v086Element(t, "heading-level", "#", "1", 1, 2), v086Element(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v086Open задаёт реальное объявление блока с явным родителем.
func v086Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v086Line(t, "@"+name+" {", "block-start", parent, depth,
		v086Element(t, "tag", "@"+name, name, 1, len(name)+2),
		v086Element(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v086Close задаёт закрытие указанного блока.
func v086Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v086Line(t, "}", "block-end", parent, depth, v086Element(t, "block-close", "}", "}", 1, 2))
}

// v086Line сохраняет явно заданную структуру, не классифицируя исходный текст.
func v086Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v086Element задаёт исходные Unicode-колонки с исключающим концом.
func v086Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v086Registry использует весь штатный каталог регистраций.
func v086Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v086Run проверяет только новые V086 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v086Run(t *testing.T, result *model.Result, source string, want []v086Expected) {
	t.Helper()
	before := v086Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v086Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v086Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V086" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V086 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "После контентного тега отсутствует обязательное значение.\nконкретизация: "+position.detail {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V086 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v086Snapshot(t, result)), &restored); err != nil {
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
	if v086Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v086Snapshot сохраняет независимое представление модели.
func v086Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

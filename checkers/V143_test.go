package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV143Registration(t *testing.T) {
	registry := v143Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V143" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "У @question отсутствует текст вопроса." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V143 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted: %+v, %v", version, definitions, err)
		}
	}
}

func TestV143ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		line           model.Line
		want           [][3]int
	}{
		{"reference", "@question Where does Tom live?", v143Question(t, "Where does Tom live?", "Where does Tom live?", 2, 1), nil},
		{"example", "@question", v143Invalid(t, "@question", 1, true, 2, 1), [][3]int{{3, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v143Lesson(t, tc.line)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v143Run(t, result, "current", tc.want)
		})
	}
}

func TestV143MissingTextAndWhitespace(t *testing.T) {
	// White_Space перечислен независимо от реализации. CR/LF принадлежат EOL.
	tails := []string{"", " ", "   ", "\t", " \t\u00a0", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"}
	for _, tail := range tails {
		for _, form := range []string{"tag", "tag and unparsed", "all unparsed"} {
			t.Run(strconv.Quote(tail)+"/"+form, func(t *testing.T) {
				line := v143Invalid(t, "@question"+tail, 1, form != "all unparsed", 2, 1)
				if form == "tag" {
					line = v143Line(t, "@question"+tail, "tag", 2, 1, v143Element(t, "tag", "@question", "question", 1))
				}
				result := v143Lesson(t, line)
				if form == "tag" {
					v143ParserError(t, result, 3, "Отсутствует обязательный текст вопроса.")
				}
				v143Run(t, result, "current", [][3]int{{3, 1, 10}})
			})
		}
	}
	for _, known := range []bool{false, true} {
		t.Run("case and indentation/known="+strconv.FormatBool(known), func(t *testing.T) {
			v143Run(t, v143Lesson(t, v143Invalid(t, " \t@QuEsTiOn\t", 3, known, 2, 1)), "current", [][3]int{{3, 3, 12}})
		})
	}
	t.Run("blank content value", func(t *testing.T) {
		line := v143Line(t, "@question \t", "tag", 2, 1,
			v143Element(t, "tag", "@question", "question", 1), v143Element(t, "content", "\t", "", 11))
		result := v143Lesson(t, line)
		v143ParserError(t, result, 3, "Отсутствует обязательный текст вопроса.")
		v143Run(t, result, "current", [][3]int{{3, 1, 10}})
	})
}

func TestV143NonemptyRaw(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"Read", "Read"}, {"Где?", "Где?"}, {"東京", "東京"}, {"?", "?"}, {"...", "..."}, {`""`, `""`},
		{"1", "1"}, {"😀", "😀"}, {"\u0301", "\u0301"}, {"\u200b", "\u200b"}, {"\ufeff", "\ufeff"},
		{`\{`, "{"}, {`\}`, "}"}, {`\\`, `\`}, {`\q`, `\q`}, {"&#32;", "&#32;"}, {"&nbsp;", "&nbsp;"},
		{"<em></em>", "<em></em>"}, {"<br>", "<br>"}, {"\u00a0?\t ", "\u00a0?"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v143Run(t, v143Lesson(t, v143Question(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"@question\tRead.", "@question\u00a0Text", "@question }", "@question __"} {
		for _, known := range []bool{false, true} {
			t.Run("erroneous nonempty/"+strconv.Quote(raw)+"/known="+strconv.FormatBool(known), func(t *testing.T) {
				v143Run(t, v143Lesson(t, v143Invalid(t, raw, 1, known, 2, 1)), "current", nil)
			})
		}
	}
	t.Run("unparsed nonempty tail after blank content", func(t *testing.T) {
		line := v143Line(t, "@question \u00a0}", "invalid", 2, 1,
			v143Element(t, "tag", "@question", "question", 1), v143Element(t, "content", "\u00a0", "\u00a0", 11),
			v143Element(t, "unparsed", "}", "", 12))
		v143Run(t, v143Lesson(t, line), "current", nil)
	})
}

func TestV143RecognitionAndBlocks(t *testing.T) {
	for _, raw := range []string{"@questions", "@questionExtra", "@question.extra", "@question-", "@question?", "@", "\u00a0@question"} {
		t.Run("not exact initial name/"+raw, func(t *testing.T) {
			v143Run(t, v143Lesson(t, v143Invalid(t, raw, 1, false, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{{`\@question`, "@question"}, {"Text @question", "Text @question"}} {
		t.Run("literal text/"+tc.raw, func(t *testing.T) {
			v143Run(t, v143Lesson(t, v143Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("recognized forbidden block", func(t *testing.T) {
		result := v143Lesson(t, v143Open(t, "question", 2, 1), v143Text(t, "Where?", "Where?", 3, 2), v143Close(t, 3, 2))
		v143ParserError(t, result, 3, "Блочная форма question запрещена.")
		v143Run(t, result, "current", nil)
	})
	for _, raw := range []string{"@question {", "@question{", "@question { Text"} {
		for _, known := range []bool{false, true} {
			t.Run("unparsed forbidden block/"+raw+"/known="+strconv.FormatBool(known), func(t *testing.T) {
				// Парсер не выделил блок: текст остаётся на внешнем уровне,
				// отдельное закрытие также представлено как ошибочный исходник.
				v143Run(t, v143Lesson(t, v143Invalid(t, raw, 1, known, 2, 1), v143Text(t, "Where?", "Where?", 2, 1),
					v143Invalid(t, "}", 1, false, 2, 1)), "current", nil)
			})
		}
	}
}

func TestV143ContextsAndAnswers(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		for _, known := range []bool{false, true} {
			t.Run("literal/"+name+"/parent tag="+strconv.FormatBool(known), func(t *testing.T) {
				opening := v143Open(t, name, 2, 1)
				if !known {
					// Ошибочный разделитель по п. 1.29; блок и родители сохранены.
					opening = v143Line(t, "@"+name+"\t{", "block-start", 2, 1,
						v143Element(t, "unparsed", "@"+name+"\t", "", 1), v143Element(t, "block-open", "{", "{", len(name)+3))
				}
				v143Run(t, v143Lesson(t, opening, v143Text(t, "@question", "@question", 3, 2),
					v143Close(t, 3, 2), v143Text(t, "Read.", "Read.", 2, 1)), "current", nil)
			})
		}
	}
	t.Run("literal exclusion ends at block boundary", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Open(t, "editor", 2, 1), v143Text(t, "@question", "@question", 3, 2),
			v143Close(t, 3, 2), v143Invalid(t, "@question", 1, true, 2, 1)), "current", [][3]int{{6, 1, 10}})
	})
	t.Run("inline editor is not a container", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Line(t, "@editor Check.", "tag", 2, 1,
			v143Element(t, "tag", "@editor", "editor", 1), v143Element(t, "content", "Check.", "Check.", 9)),
			v143Invalid(t, "@question", 1, true, 2, 1)), "current", [][3]int{{4, 1, 10}})
	})
	t.Run("nested branch", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Open(t, "variants", 2, 1),
			v143Line(t, "@variant A", "tag", 3, 2, v143Element(t, "tag", "@variant", "variant", 1), v143Element(t, "name", "A", "A", 10)),
			v143Invalid(t, "@question", 1, true, 4, 3),
			v143Line(t, "@variant B", "tag", 3, 2, v143Element(t, "tag", "@variant", "variant", 1), v143Element(t, "name", "B", "B", 10)),
			v143Text(t, "Other material.", "Other material.", 6, 3), v143Close(t, 3, 2)), "current", [][3]int{{5, 1, 10}})
	})
	for _, missing := range []bool{false, true} {
		for _, answer := range []bool{false, true} {
			t.Run("missing="+strconv.FormatBool(missing)+"/answer="+strconv.FormatBool(answer), func(t *testing.T) {
				question := v143Question(t, "Where does Tom live?", "Where does Tom live?", 2, 1)
				var want [][3]int
				if missing {
					question, want = v143Invalid(t, "@question", 1, true, 2, 1), [][3]int{{3, 1, 10}}
				}
				body := []model.Line{question}
				if answer {
					body = append(body, v143Line(t, "@answer London", "tag", 2, 1,
						v143Element(t, "tag", "@answer", "answer", 1), v143Element(t, "content", "London", "London", 9)))
				}
				v143Run(t, v143Lesson(t, body...), "current", want)
			})
		}
	}
	t.Run("several occurrences", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Invalid(t, "@question", 1, true, 2, 1), v143Invalid(t, "@QUESTION ", 1, false, 2, 1)),
			"current", [][3]int{{3, 1, 10}, {4, 1, 10}})
	})
	t.Run("blank separates without merging", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Invalid(t, "@question", 1, true, 2, 1), v143Line(t, "", "blank", 2, 1),
			v143Invalid(t, "@question", 1, true, 2, 1)), "current", [][3]int{{3, 1, 10}, {5, 1, 10}})
	})
}

func TestV143Lifecycle(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(eol))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v143Lesson(t, v143Invalid(t, "@question", 1, false, 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if !final {
					result.Lines[2].LineEnding = ""
				}
				v143Run(t, result, "first", [][3]int{{3, 1, 10}})
				v143Run(t, result, "second", [][3]int{{3, 1, 10}})
			})
		}
	}
	t.Run("empty model", func(t *testing.T) {
		result := v143Lesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v143Run(t, result, "empty", nil)
	})
	t.Run("blank lines", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Line(t, "", "blank", 2, 1), v143Line(t, " \t", "blank", 2, 1)), "current", nil)
	})
	t.Run("independent documents", func(t *testing.T) {
		v143Run(t, v143Lesson(t, v143Invalid(t, "@question", 1, true, 2, 1)), "first", [][3]int{{3, 1, 10}})
		v143Run(t, v143Lesson(t, v143Question(t, "Where?", "Where?", 2, 1)), "second", nil)
	})
}

// v143Invalid задаёт ошибочную строку с известным question либо целиком unparsed.
// Позиции элементов вычисляются только для входа, не для ожидаемых срабатываний.
func v143Invalid(t *testing.T, raw string, start int, known bool, parent, depth int) model.Line {
	t.Helper()
	elements := []model.Element{}
	from := 1
	if known {
		elements = append(elements, v143Element(t, "tag", string([]rune(raw)[start-1:start+8]), "question", start))
		from = start + 9
	}
	elements = append(elements, v143Element(t, "unparsed", string([]rune(raw)[from-1:]), "", from))
	return v143Line(t, raw, "invalid", parent, depth, elements...)
}

// v143Question задаёт непустой однострочный вопрос с явными исходником и значением.
func v143Question(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v143Line(t, "@question "+raw, "tag", parent, depth,
		v143Element(t, "tag", "@question", "question", 1), v143Element(t, "content", raw, value, 11))
}

// v143Lesson добавляет обязательный контекст; body начинается со строки 3.
// Явно заданные unparsed получают согласованные исходные ошибки и обратные ссылки.
func v143Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v143Line(t, "@dsl-version 1.2", "tag", 0, 0, v143Element(t, "tag", "@dsl-version", "dsl-version", 1), v143Element(t, "version", "1.2", "1.2", 14)),
		v143Line(t, "# Lesson", "heading", 0, 0, v143Element(t, "heading-level", "#", "1", 1), v143Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
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
				ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Не разобран фрагмент исходной строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v143ParserError добавляет явно указанную ошибку формы или отсутствующего текста.
func v143ParserError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	result.Document.HasErrors, result.Lines[number-1].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-line-error", Source: "parser", DiagnosticCode: "fixture-line", SeverityLevel: "error", DiagnosticScope: "line",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(result.Lines[number-1].Raw) + 1}},
	})
}

// v143Open задаёт объявление блока с явным родителем.
func v143Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v143Line(t, "@"+name+" {", "block-start", parent, depth,
		v143Element(t, "tag", "@"+name, name, 1), v143Element(t, "block-open", "{", "{", len(name)+3))
}

// v143Close задаёт закрытие указанного блока.
func v143Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v143Line(t, "}", "block-end", parent, depth, v143Element(t, "block-close", "}", "}", 1))
}

// v143Text сохраняет явно заданные Raw и семантическое значение content.
func v143Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v143Line(t, raw, "content", parent, depth, v143Element(t, "content", raw, value, 1))
}

// v143Line сохраняет структуру входа без классификации исходного текста.
func v143Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v143Element измеряет входной элемент в Unicode-колонках, конец исключён.
// Ожидания диагностик задаются отдельно явными числами.
func v143Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v143Registry использует исключительно полный штатный каталог регистраций.
func v143Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v143Run проверяет новые V143 полного конвейера и сохранность входной модели.
// want содержит независимые тройки: строка, начало и исключённый конец диапазона.
func v143Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v143Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v143Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v143Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V143" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V143 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "У @question отсутствует текст вопроса." {
			t.Fatalf("occurrence %d = %+v; location = %+v, want %+v and exact V143 metadata", i, entry, entry.Location, location)
		}
		if !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line-scoped V143 linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v143Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// В копии возвращаем только производные флаги/ссылки и убираем новые записи.
	// Исходные строки, метаданные и прежняя история должны остаться дословными.
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
	if v143Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v143Snapshot получает независимое представление модели для сравнения.
func v143Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

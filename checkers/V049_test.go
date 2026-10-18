package checkers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v049Expected задаёт исходные диапазоны независимо от реализации.
type v049Expected struct{ line, start, end int }

func v049Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func v049UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

func v049ModelLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v049ModelElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

func v049Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v049ModelLine(t, raw, "content", parent, depth,
		v049ModelElement(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

func v049Block(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v049ModelLine(t, tag+" {", "block-start", parent, depth,
		v049ModelElement(t, "tag", tag, name, 1, open-1), v049ModelElement(t, "block-open", "{", "{", open, open+1))
}

func v049Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v049ModelLine(t, "}", "block-end", parent, depth, v049ModelElement(t, "block-close", "}", "}", 1, 2))
}

func v049Heading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v049ModelLine(t, marker+" "+title, "heading", 0, 0,
		v049ModelElement(t, "heading-level", marker, fmt.Sprint(level), 1, level+1),
		v049ModelElement(t, "title", title, title, level+2, level+2+utf8.RuneCountInString(title)))
}

func v049ParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	line.HasErrors = true
	result.Document.HasErrors = true
	element.ErrorIDs = append(element.ErrorIDs, id)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
		DiagnosticScope: "element", Message: message,
		Location: &model.Location{
			Start: model.Position{Line: number, Column: element.Start},
			End:   model.Position{Line: number, Column: element.End},
		}, RelatedLocations: []model.Location{},
	})
}

func v049Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v049Clone(t *testing.T, result *model.Result) *model.Result {
	t.Helper()
	var cloned model.Result
	if err := json.Unmarshal([]byte(v049Snapshot(t, result)), &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

func v049Document(t *testing.T, metadata, body []model.Line) *model.Result {
	t.Helper()
	result := v049UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{v049ModelLine(t, "@dsl-version 1.2", "tag", 0, 0,
		v049ModelElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
		v049ModelElement(t, "version", "1.2", "1.2", 14, 17))}
	result.Lines = append(result.Lines, metadata...)
	result.Lines = append(result.Lines, v049Heading(t, 1, "Unit 1"))
	result.Lines = append(result.Lines, body...)
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count, title, encoding := len(result.Lines), "Unit 1", "UTF-8"
	result.Document.LineCount = &count
	result.Document.Metadata.Title = &title
	result.Document.Encoding = &encoding
	return result
}

func TestV049Registration(t *testing.T) {
	registry := v049Registry(t)
	for _, version := range []string{"1.1", "1.2", "1.3"} {
		definitions, err := registry.ForVersion(dslversions.Version(version))
		if version != "1.2" {
			if err == nil || definitions != nil {
				t.Fatalf("unsupported version %s: definitions=%v err=%v", version, definitions, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, d := range definitions {
			if d.Code == "V049" {
				count++
				if d.Message != "Неэкранированная закрывающая фигурная скобка находится в строке с другим содержимым." || d.Severity != "error" || d.Scope != "line" || d.Check == nil || d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
					t.Errorf("definition: %+v", d)
				}
			}
		}
		want := 0
		if version == "1.2" {
			want = 1
		}
		if count != want {
			t.Errorf("version %s: count %d, want %d", version, count, want)
		}
	}
}

func v049Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v049ModelLine(t, raw, "invalid", parent, depth, model.Element{
		ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{},
	})
}

func v049Run(t *testing.T, result *model.Result, processingID string, want []v049Expected) {
	t.Helper()
	expected := v049Clone(t, result)
	diagnosticCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v049Registry(t))
	if err != nil {
		t.Fatal(err)
	}
	if v049Snapshot(t, result) != v049Snapshot(t, expected) {
		t.Fatal("Prepare changed input")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	expected.Processing = append(expected.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatal(err)
	}
	var got []model.Diagnostic
	otherNewIDs := make(map[string]bool)
	uniqueIDs := make(map[string]bool)
	for i, d := range result.Diagnostics {
		if d.ID == "" || uniqueIDs[d.ID] {
			t.Fatalf("empty or duplicate ID: %q", d.ID)
		}
		uniqueIDs[d.ID] = true
		if i < diagnosticCount {
			continue
		}
		if d.DiagnosticCode == "V049" {
			got = append(got, d)
		} else {
			otherNewIDs[d.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V049 count=%d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		d := got[i]
		location := model.Location{Start: model.Position{Line: w.line, Column: w.start}, End: model.Position{Line: w.line, Column: w.end}}
		if d.Location == nil || *d.Location != location || len(d.RelatedLocations) != 0 {
			t.Errorf("unexpected ranges: %+v, want %+v without related", d, location)
		}
		if d.DiagnosticCode != "V049" || d.Source != processingID || d.SeverityLevel != "error" || d.DiagnosticScope != "line" || d.Fatal || d.Message != "Неэкранированная закрывающая фигурная скобка находится в строке с другим содержимым.\nконкретизация: Граница блока записывается отдельной строкой; для буквального символа используйте \\}" {
			t.Errorf("unexpected metadata: %+v", d)
		}
		expected.Document.HasErrors = true
		expected.Lines[w.line-1].HasErrors = true
	}
	// Общие флаги проверяются штатным контролем целостности с учётом
	// всех диагностик. В снимке оставляем прежние данные и эффекты V049.
	if _, err := validator.Prepare(result, v049Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	actual := v049Clone(t, result)
	actual.Document.HasErrors = expected.Document.HasErrors
	for i := range actual.Lines {
		actual.Lines[i].HasErrors = expected.Lines[i].HasErrors
	}
	for i := range actual.Lines {
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			kept := element.ErrorIDs[:0]
			for _, id := range element.ErrorIDs {
				if !otherNewIDs[id] {
					kept = append(kept, id)
				}
			}
			element.ErrorIDs = kept
		}
	}
	// Ссылки V049 не добавляются: scope=line. Такая ссылка или изменение
	// прежней истории нарушат сравнение моделей; флаги проверены выше.
	expected.Diagnostics = append(expected.Diagnostics, result.Diagnostics[diagnosticCount:]...)
	if v049Snapshot(t, actual) != v049Snapshot(t, expected) {
		t.Fatal("unexpected source, flags, element links, metadata or previous history changes")
	}
}

func TestV049ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Эталон сохранён дословно; перед ним добавлены версия и заголовок.
		result := v049Document(t, nil, []model.Line{v049Block(t, "note", 2, 1), v049Text(t, "Read carefully.", 3, 2), v049Close(t, 3, 2)})
		v049Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		// Пример также сохранён дословно. Ошибочная последняя строка не
		// восстанавливается как закрытие; у незакрытого блока сохранена ошибка.
		result := v049Document(t, nil, []model.Line{v049Block(t, "note", 2, 1), v049Text(t, "Read carefully.", 3, 2), v049Invalid(t, "} Continue.", 3, 2)})
		v049ParserError(t, result, 3, 0, "Блок не закрыт.")
		v049ParserError(t, result, 5, 0, "Ошибочная строка закрытия блока.")
		v049Run(t, result, "current", []v049Expected{{5, 1, 2}})
	})
}

func TestV049RawCandidates(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v049Expected
	}{
		{"before", "A}", []v049Expected{{3, 2, 3}}},
		{"after", "}A", []v049Expected{{3, 1, 2}}},
		{"both", "A}B", []v049Expected{{3, 2, 3}}},
		{"two", "}}", []v049Expected{{3, 1, 2}, {3, 2, 3}}},
		{"three", "}}}", []v049Expected{{3, 1, 2}, {3, 2, 3}, {3, 3, 4}}},
		{"quoted", `"}"`, []v049Expected{{3, 2, 3}}},
		{"single quotes", "'}'", []v049Expected{{3, 2, 3}}},
		{"indented", " \t} X", []v049Expected{{3, 3, 4}}},
		{"Unicode columns", "Я😀}", []v049Expected{{3, 3, 4}}},
		{"combining mark", "e\u0301}", []v049Expected{{3, 3, 4}}},
		{"nonbreaking prefix", "\u00a0}", []v049Expected{{3, 2, 3}}},
		{"nonbreaking suffix", "}\u00a0", []v049Expected{{3, 1, 2}}},
		{"em space suffix", "}\u2003", []v049Expected{{3, 1, 2}}},
		{"alone without opening", "}", nil},
		{"ordinary spaces only", "  }   ", nil},
		{"tabs only", "\t}\t", nil},
		{"mixed whitespace only", " \t} \t", nil},
		{"opening only", "{", nil},
		{"pair on same line", "{}", []v049Expected{{3, 2, 3}}},
		{"unrecognized escape", `\q}`, []v049Expected{{3, 3, 4}}},
		{"escaped backslash", `\\}`, []v049Expected{{3, 3, 4}}},
		{"four backslashes", `\\\\}`, []v049Expected{{3, 5, 6}}},
		{"escaped opening", `\{}`, []v049Expected{{3, 3, 4}}},
		{"mixed escaped closing", `\}}`, []v049Expected{{3, 3, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v049Document(t, nil, []model.Line{v049Invalid(t, tc.raw, 2, 1)})
			// Для отдельной } это ошибка парности, а не предмет V049.
			v049ParserError(t, result, 3, 0, "Ошибочная запись фигурной скобки.")
			v049Run(t, result, "current", tc.want)
		})
	}
}

func TestV049EscapedAndEmpty(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\}`, "}"}, {`\\\}`, `\}`}, {` \} `, "}"}, {`\{\}`, "{}"}, {`\\`, `\`},
		{"｝", "｝"}, {"text", "text"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			// Raw сохраняет экранирование; Value задаётся независимо и не проверяется V049.
			start, end := 1, utf8.RuneCountInString(tc.raw)+1
			fragment := tc.raw
			if tc.raw == ` \} ` {
				fragment = `\}`
				start, end = 2, 4
			}
			line := v049ModelLine(t, tc.raw, "content", 2, 1, v049ModelElement(t, "content", fragment, tc.value, start, end))
			v049Run(t, v049Document(t, nil, []model.Line{line}), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run(fmt.Sprintf("blank %q", raw), func(t *testing.T) {
			v049Run(t, v049Document(t, nil, []model.Line{v049ModelLine(t, raw, "blank", 2, 1)}), "current", nil)
		})
	}
	v049Run(t, v049UnavailableDocument(t, "1.2"), "unavailable", nil)
	result := v049UnavailableDocument(t, "1.2")
	zero := 0
	result.Document.LineCount = &zero
	v049Run(t, result, "empty", nil)
}

// v049Placeholder задаёт готовый элемент ответа целиком, не распознавая Raw.
func v049Placeholder(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "placeholder", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

func TestV049MultifillAnswers(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		elements  []model.Element
		bad       bool
		want      []v049Expected
	}{
		{"answer", "_____{A}", []model.Element{v049Placeholder(t, "_____{A}", 1, 9)}, false, nil},
		{"empty answer", "_____{}", []model.Element{v049Placeholder(t, "_____{}", 1, 8)}, true, nil},
		{"escaped closing", `_____{A\}B}`, []model.Element{v049Placeholder(t, `_____{A\}B}`, 1, 12)}, false, nil},
		{"escaped opening", `_____{A\{B}`, []model.Element{v049Placeholder(t, `_____{A\{B}`, 1, 12)}, false, nil},
		{"escaped backslash in answer", `_____{A\\}`, []model.Element{v049Placeholder(t, `_____{A\\}`, 1, 11)}, false, nil},
		{"closing after answer", "_____{A}}", nil, true, []v049Expected{{4, 9, 10}}},
		{"four underscores", "____{A}", nil, true, []v049Expected{{4, 7, 8}}},
		{"six underscores", "______{A}", nil, true, []v049Expected{{4, 9, 10}}},
		{"seven underscores", "_______{A}", nil, true, []v049Expected{{4, 10, 11}}},
		{"gap before brace", "_____ {A}", nil, true, []v049Expected{{4, 9, 10}}},
		{"escaped underscores", `\_____{A}`, nil, true, []v049Expected{{4, 9, 10}}},
		{"escaped six underscores", `\______{A}`, nil, true, []v049Expected{{4, 10, 11}}},
		{"escaped opening after underscores", `_____\{A}`, nil, true, []v049Expected{{4, 9, 10}}},
		{"no nesting in answer", "_____{A{B}}", nil, true, []v049Expected{{4, 11, 12}}},
		{"unterminated answer", "_____{A", nil, true, nil},
		{"unterminated escaped ending", `_____{A\}`, nil, true, nil},
		{"unterminated trailing slash", `_____{A\`, nil, true, nil},
		{"backslash pair before placeholder", `\\_____{A}`, []model.Element{
			v049ModelElement(t, "content", `\\`, `\`, 1, 3), v049Placeholder(t, "_____{A}", 3, 11)}, false, nil},
		{"two answers", "_____{A} _____{B}", []model.Element{
			v049Placeholder(t, "_____{A}", 1, 9), v049Placeholder(t, "_____{B}", 10, 18)}, false, nil},
		{"mixed ordinary and answered placeholders", "_____ _____{A}", []model.Element{
			v049Placeholder(t, "_____", 1, 6), v049Placeholder(t, "_____{A}", 7, 15)}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v049ModelLine(t, tc.raw, "content", 3, 2, tc.elements...)
			if tc.elements == nil {
				line = v049Invalid(t, tc.raw, 3, 2)
			}
			result := v049Document(t, nil, []model.Line{v049Block(t, "multifill", 2, 1), line, v049Close(t, 3, 2)})
			if tc.bad {
				v049ParserError(t, result, 4, 0, "Некорректная запись плейсхолдера или скобки.")
			}
			v049Run(t, result, "current", tc.want)
		})
	}
	t.Run("answer does not continue on next line", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{
			v049Block(t, "multifill", 2, 1), v049Invalid(t, "_____{A", 3, 2), v049Invalid(t, "B}", 3, 2), v049Close(t, 3, 2),
		})
		v049ParserError(t, result, 4, 0, "Незавершённый ответ.")
		v049ParserError(t, result, 5, 0, "Неэкранированная скобка.")
		v049Run(t, result, "current", []v049Expected{{5, 2, 3}})
	})
}

func TestV049Contexts(t *testing.T) {
	t.Run("inline editor", func(t *testing.T) {
		line := v049ModelLine(t, "@editor A}", "tag", 2, 1,
			v049ModelElement(t, "tag", "@editor", "editor", 1, 8),
			v049ModelElement(t, "content", "A}", "A}", 9, 11))
		v049Run(t, v049Document(t, nil, []model.Line{line}), "current", []v049Expected{{3, 10, 11}})
	})
	t.Run("outside multifill", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{v049Invalid(t, "_____{A}", 2, 1)})
		v049ParserError(t, result, 3, 0, "Ответ вне multifill.")
		v049Run(t, result, "current", []v049Expected{{3, 8, 9}})
	})
	t.Run("editor does not recognize apparent multifill", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{
			v049Block(t, "editor", 2, 1), v049Text(t, "@multifill {", 3, 2), v049Text(t, "_____{A}", 3, 2), v049Close(t, 3, 2),
		})
		v049Run(t, result, "current", []v049Expected{{5, 8, 9}})
	})
	t.Run("multifill does not recognize apparent editor", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{
			v049Block(t, "multifill", 2, 1), v049Text(t, "@editor {", 3, 2),
			v049ModelLine(t, "_____{A}", "content", 3, 2, v049Placeholder(t, "_____{A}", 1, 9)), v049Close(t, 3, 2),
		})
		v049Run(t, result, "current", nil)
	})
	t.Run("multifill instruction is outside its content", func(t *testing.T) {
		opening := v049ModelLine(t, "@multifill _____{A} {", "block-start", 2, 1,
			v049ModelElement(t, "tag", "@multifill", "multifill", 1, 11),
			v049ModelElement(t, "content", "_____{A}", "_____{A}", 12, 20), v049ModelElement(t, "block-open", "{", "{", 21, 22))
		result := v049Document(t, nil, []model.Line{opening, v049Text(t, "Read.", 3, 2), v049Close(t, 3, 2)})
		v049ParserError(t, result, 3, 1, "Ответ в инструкции вместо предзаполненного содержимого.")
		v049Run(t, result, "current", []v049Expected{{3, 19, 20}})
	})
	t.Run("inline multifill is not a literal parent", func(t *testing.T) {
		line := v049ModelLine(t, "@multifill _____{A}", "tag", 2, 1,
			v049ModelElement(t, "tag", "@multifill", "multifill", 1, 11), v049ModelElement(t, "content", "_____{A}", "_____{A}", 12, 20))
		result := v049Document(t, nil, []model.Line{line})
		v049ParserError(t, result, 3, 1, "Ответ в инструкции вместо предзаполненного содержимого.")
		v049Run(t, result, "current", []v049Expected{{3, 19, 20}})
	})
	t.Run("after multifill closes", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{v049Block(t, "multifill", 2, 1),
			v049ModelLine(t, "_____{A}", "content", 3, 2, v049Placeholder(t, "_____{A}", 1, 9)), v049Close(t, 3, 2), v049Invalid(t, "_____{A}", 2, 1)})
		v049ParserError(t, result, 6, 0, "Ответ вне multifill.")
		v049Run(t, result, "current", []v049Expected{{6, 8, 9}})
	})
	t.Run("unparsed declaration is not reconstructed", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{v049Invalid(t, "@multifill {", 2, 1), v049Invalid(t, "_____{A}", 2, 1)})
		v049ParserError(t, result, 3, 0, "Объявление не распознано.")
		v049ParserError(t, result, 4, 0, "Ответ вне распознанного блока.")
		v049Run(t, result, "current", []v049Expected{{4, 8, 9}})
	})
	t.Run("erroneous recognized block and canonical name", func(t *testing.T) {
		opening := v049ModelLine(t, "@MULTIFILL { extra", "block-start", 2, 1,
			v049ModelElement(t, "tag", "@MULTIFILL", "multifill", 1, 11), v049ModelElement(t, "block-open", "{", "{", 12, 13),
			model.Element{ElementType: "unparsed", Raw: "extra", Start: 14, End: 19, ErrorIDs: []string{}})
		result := v049Document(t, nil, []model.Line{opening,
			v049ModelLine(t, "_____{A}", "content", 3, 2, v049Placeholder(t, "_____{A}", 1, 9)), v049Close(t, 3, 2)})
		v049ParserError(t, result, 3, 2, "Содержимое после открытия блока.")
		v049Run(t, result, "current", nil)
	})
	t.Run("nested in task", func(t *testing.T) {
		task := v049ModelLine(t, "@task t", "tag", 2, 1, v049ModelElement(t, "tag", "@task", "task", 1, 6), v049ModelElement(t, "identifier", "t", "t", 7, 8))
		result := v049Document(t, nil, []model.Line{task, v049Block(t, "multifill", 3, 2),
			v049ModelLine(t, "_____{A}", "content", 4, 3, v049Placeholder(t, "_____{A}", 1, 9)), v049Close(t, 4, 3)})
		v049Run(t, result, "current", nil)
	})
}

// v049Resource задаёт распознанное объявление с одним буквальным путём.
func v049Resource(t *testing.T, tag, rawPath, path string) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 1
	return v049ModelLine(t, tag+" "+rawPath, "tag", 0, 0,
		v049ModelElement(t, "tag", tag, "resource-dir", 1, end),
		v049ModelElement(t, "resource-path", rawPath, path, end+1, end+1+utf8.RuneCountInString(rawPath)))
}

func TestV049ResourceDirectories(t *testing.T) {
	for _, tc := range []struct{ name, tag, rawPath, path string }{
		{"unquoted", "@resource-dir", "assets}", "assets}"},
		{"quoted", "@resource-dir", `"assets}"`, "assets}"},
		{"literal backslashes", "@resource-dir", `"C:\assets\\}"`, `C:\assets\\}`},
		{"canonical name", "@RESOURCE-DIR", "assets}", "assets}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v049Document(t, []model.Line{v049Resource(t, tc.tag, tc.rawPath, tc.path)}, nil)
			result.Document.Metadata.ResourceDirs = []string{tc.path}
			v049Run(t, result, "current", nil)
		})
	}
	t.Run("trailing comma", func(t *testing.T) {
		line := v049Resource(t, "@resource-dir", "assets}", "assets}")
		line.Raw += ","
		line.Elements = append(line.Elements, model.Element{ElementType: "unparsed", Raw: ",", Start: 22, End: 23, ErrorIDs: []string{}})
		result := v049Document(t, []model.Line{line}, nil)
		v049ParserError(t, result, 2, 2, "Завершающая запятая в списке каталогов.")
		v049Run(t, result, "current", nil)
	})
	t.Run("invalid line with recognized tag", func(t *testing.T) {
		line := v049ModelLine(t, "@resource-dir assets},", "invalid", 0, 0,
			v049ModelElement(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			model.Element{ElementType: "unparsed", Raw: "assets},", Start: 15, End: 23, ErrorIDs: []string{}})
		result := v049Document(t, []model.Line{line}, nil)
		v049ParserError(t, result, 2, 1, "Некорректный список каталогов.")
		v049Run(t, result, "current", nil)
	})
	t.Run("misplaced declaration", func(t *testing.T) {
		line := v049Resource(t, "@resource-dir", "assets}", "assets}")
		parent := 2
		line.ParentLine = &parent
		line.NestingLevel = 1
		result := v049Document(t, nil, []model.Line{line})
		v049ParserError(t, result, 3, 0, "Метатег после заголовка.")
		v049Run(t, result, "current", nil)
	})
	t.Run("inside nonliteral block", func(t *testing.T) {
		line := v049Resource(t, "@resource-dir", "assets}", "assets}")
		parent := 3
		line.ParentLine = &parent
		line.NestingLevel = 2
		result := v049Document(t, nil, []model.Line{v049Block(t, "note", 2, 1), line, v049Close(t, 3, 2)})
		v049ParserError(t, result, 4, 0, "Метатег внутри блока.")
		v049Run(t, result, "current", nil)
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			result := v049Document(t, nil, []model.Line{v049Block(t, name, 2, 1), v049Text(t, "@resource-dir assets}", 3, 2), v049Close(t, 3, 2)})
			v049Run(t, result, "current", []v049Expected{{4, 21, 22}})
		})
	}
	t.Run("unparsed alone", func(t *testing.T) {
		result := v049Document(t, nil, []model.Line{v049Invalid(t, "@resource-dir assets}", 2, 1)})
		v049ParserError(t, result, 3, 0, "Объявление не распознано.")
		v049Run(t, result, "current", []v049Expected{{3, 21, 22}})
	})
}

func TestV049LinesAndRuns(t *testing.T) {
	result := v049Document(t, nil, []model.Line{v049Invalid(t, "A}", 2, 1), v049Invalid(t, "}B", 2, 1)})
	v049ParserError(t, result, 3, 0, "Неэкранированная скобка.")
	v049ParserError(t, result, 4, 0, "Неэкранированная скобка.")
	want := []v049Expected{{3, 2, 3}, {4, 1, 2}}
	v049Run(t, result, "first", want)
	v049Run(t, result, "second", want)
	v049Run(t, v049Document(t, nil, nil), "independent", nil)
}

func TestV049LineEndings(t *testing.T) {
	for _, eol := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF} {
		for _, lastNone := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/final-none-%t", eol, lastNone), func(t *testing.T) {
				result := v049Document(t, nil, []model.Line{v049Invalid(t, "Я😀}", 2, 1)})
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if lastNone {
					result.Lines[2].LineEnding = model.LineEndingNone
				}
				v049ParserError(t, result, 3, 0, "Неэкранированная скобка.")
				v049Run(t, result, "current", []v049Expected{{3, 3, 4}})
			})
		}
	}
}

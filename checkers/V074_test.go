package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v074TestMessage = "У @editor отсутствует текст комментария."

// v074TestWant задаёт исходный диапазон с исключающей правой границей.
type v074TestWant struct{ line, start, end int }

func TestV074RegistrationAndVersions(t *testing.T) {
	registry := v074TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V074" {
			continue
		}
		count++
		if definition.Message != v074TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V074 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if definitions, err := registry.ForVersion(dslversions.Version(version)); err == nil || definitions != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", definitions, err)
			}
			result := v074TestDocument(t)
			result.Document.DSLVersion = &version
			result.Document.LineCount = nil
			result.Document.Metadata.Title = nil
			result.Lines = []model.Line{}
			before := v074TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v074TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV074ReferenceAndExample(t *testing.T) {
	// Эталон и пример сохранены дословно на строке 3. Перед ними добавлены
	// версия и основной заголовок, после — отдельная строка содержимого.
	for _, tc := range []struct {
		name, comment string
		want          []v074TestWant
	}{
		{"reference", "Check the wording.", nil},
		{"example", "", []v074TestWant{{3, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v074TestRun(t, v074TestDocument(t, v074TestTag(t, "editor", tc.comment, "content", 2),
				v074TestText(t, "Read.", "Read.", 2)), "current", tc.want)
		})
	}
}

func TestV074WhitespaceAndFallback(t *testing.T) {
	// Явный перечень независим от unicode.White_Space проверяемого кода.
	// CR/LF не могут находиться в Raw; окончания строк проверяются отдельно.
	whitespace := []string{"", " ", "   ", "\t", " \t \t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000",
		" \t\u00a0\u2003\u3000 "}
	for _, tail := range whitespace {
		for _, parsed := range []bool{true, false} {
			t.Run(strconv.Quote(tail)+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				raw := "@editor" + tail
				line := v074TestInvalid(t, raw, 2)
				if parsed {
					elements := []model.Element{v074TestElement(t, "tag", "@editor", "editor", 1)}
					if tail != "" {
						// Пустой комментарий ошибочен независимо от того,
						// удалось ли парсеру распознать само имя.
						elements = append(elements, v074TestElement(t, "unparsed", tail, "", 8))
					}
					line = v074TestLine(t, raw, "tag", 2, elements...)
				}
				v074TestRun(t, v074TestDocument(t, line, v074TestText(t, "Read.", "Read.", 2)),
					"current", []v074TestWant{{3, 1, 8}})
			})
		}
	}
	for _, parsed := range []bool{true, false} {
		t.Run("indent and case parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
			line := v074TestInvalid(t, " \t@EdItOr \t", 2)
			if parsed {
				line = v074TestLine(t, " \t@EdItOr \t", "tag", 2,
					v074TestElement(t, "tag", "@EdItOr", "editor", 3))
			}
			// Наличие tag и совпадающего Raw не должно давать два сообщения.
			v074TestRun(t, v074TestDocument(t, line), "current", []v074TestWant{{3, 3, 10}})
		})
	}
}

func TestV074NonemptyRemainder(t *testing.T) {
	for _, comment := range []string{"x", ".", "Я😀", " \t.\u00a0", "\u200b", "\ufeff",
		"Check  the\twording.", "CHECK THE WORDING.", "\\{", "<br>", "&#32;"} {
		t.Run(strconv.Quote(comment), func(t *testing.T) {
			// editor не декодирует HTML и не оценивает видимость текста.
			value := comment
			if comment == "\\{" {
				value = "{"
			}
			line := v074TestLine(t, "@editor "+comment, "tag", 2,
				v074TestElement(t, "tag", "@editor", "editor", 1),
				v074TestElement(t, "content", comment, value, 9))
			v074TestRun(t, v074TestDocument(t, line), "current", nil)
		})
	}
	for _, tc := range []struct {
		raw, tail string
		start     int
	}{
		{"@editor }", "}", 9},
		{"@editor\tText", "\tText", 8},
		{"@editor\u00a0Text", "\u00a0Text", 8},
		{"@editor { extra", "{ extra", 9},
	} {
		for _, parsed := range []bool{true, false} {
			t.Run(tc.raw+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				// Ошибочная форма не означает отсутствия непустого Raw.
				line := v074TestInvalid(t, tc.raw, 2)
				if parsed {
					line = v074TestLine(t, tc.raw, "tag", 2,
						v074TestElement(t, "tag", "@editor", "editor", 1),
						v074TestElement(t, "unparsed", tc.tail, "", tc.start))
				}
				v074TestRun(t, v074TestDocument(t, line), "current", nil)
			})
		}
	}
}

func TestV074NonDeclarations(t *testing.T) {
	for _, raw := range []string{"@editorExtra", "@editor!", "@edit", "@editor\u200b"} {
		t.Run(raw, func(t *testing.T) {
			v074TestRun(t, v074TestDocument(t, v074TestInvalid(t, raw, 2)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{"Text @editor", "Text @editor"},
		{"\\@editor", "@editor"},
		{" \t\\@EDITOR", "@EDITOR"},
		{"\u00a0@editor", "\u00a0@editor"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v074TestRun(t, v074TestDocument(t, v074TestText(t, tc.raw, tc.value, 2)), "current", nil)
		})
	}
	t.Run("other recognized tag", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestTag(t, "header", "@editor", "title", 2),
			v074TestText(t, "Read.", "Read.", 2)), "current", nil)
	})
	t.Run("empty and whitespace lines", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestLine(t, "", "blank", 2),
			v074TestLine(t, " \t", "blank", 2)), "current", nil)
	})
}

func TestV074BlockForms(t *testing.T) {
	t.Run("nonempty block", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestOpen(t, "editor", 2),
			v074TestText(t, "Check the wording.", "Check the wording.", 3), v074TestClose(t, 3)), "current", nil)
	})
	t.Run("empty block belongs to another diagnostic", func(t *testing.T) {
		result := v074TestDocument(t, v074TestOpen(t, "editor", 2), v074TestClose(t, 3))
		v074TestParserError(t, result, 3, 0, "Пустой блок editor.")
		v074TestRun(t, result, "current", nil)
	})
	t.Run("unclosed block is not an empty single-line comment", func(t *testing.T) {
		result := v074TestDocument(t, v074TestOpen(t, "editor", 2),
			v074TestText(t, "@editor", "@editor", 3))
		v074TestParserError(t, result, 3, 1, "Незакрытый блок editor.")
		v074TestRun(t, result, "current", nil)
	})
	t.Run("missing separator before opening brace", func(t *testing.T) {
		// Ошибочное объявление сохранено как block-start: имя — unparsed,
		// а границы и родитель содержимого доступны в исходной модели.
		open := v074TestLine(t, "@editor{", "block-start", 2,
			v074TestElement(t, "unparsed", "@editor", "", 1),
			v074TestElement(t, "block-open", "{", "{", 8))
		v074TestRun(t, v074TestDocument(t, open, v074TestText(t, "@editor", "@editor", 3),
			v074TestClose(t, 3)), "current", nil)
	})
}

func TestV074LiteralAndNestedContexts(t *testing.T) {
	for _, tag := range []string{"editor", "EDITOR", "multifill", "MULTIFILL"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v074TestRun(t, v074TestDocument(t, v074TestOpen(t, tag, 2),
				v074TestText(t, "@editor", "@editor", 3), v074TestText(t, "@EDITOR \t", "@EDITOR", 3),
				v074TestClose(t, 3)), "current", nil)
		})
		t.Run("literal inside task "+tag, func(t *testing.T) {
			v074TestRun(t, v074TestDocument(t, v074TestTag(t, "task", "a", "identifier", 2),
				v074TestTag(t, "step", "Prepare", "title", 3), v074TestOpen(t, tag, 4),
				v074TestText(t, "@editor", "@editor", 5), v074TestClose(t, 5)), "current", nil)
		})
	}
	t.Run("after literal block", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestOpen(t, "editor", 2),
			v074TestText(t, "@editor", "@editor", 3), v074TestClose(t, 3),
			v074TestTag(t, "editor", "", "", 2)), "current", []v074TestWant{{6, 1, 8}})
	})
	for _, parsed := range []bool{true, false} {
		t.Run("real nested comment parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
			line := v074TestInvalid(t, "@editor", 3)
			if parsed {
				line = v074TestTag(t, "editor", "", "", 3)
			}
			result := v074TestDocument(t, v074TestOpen(t, "text", 2), line, v074TestClose(t, 3))
			if parsed {
				v074TestParserError(t, result, 4, 0, "Вложенный editor без текста.")
			}
			v074TestRun(t, result, "current", []v074TestWant{{4, 1, 8}})
		})
	}
	t.Run("task and step parents", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestTag(t, "task", "a", "identifier", 2),
			v074TestTag(t, "editor", "", "", 3), v074TestText(t, "Read.", "Read.", 3),
			v074TestTag(t, "step", "Prepare", "title", 3), v074TestTag(t, "editor", "", "", 6),
			v074TestText(t, "Write.", "Write.", 6)),
			"current", []v074TestWant{{4, 1, 8}, {7, 1, 8}})
	})
	t.Run("branches", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestOpen(t, "variants", 2),
			v074TestTag(t, "variant", "A", "name", 3), v074TestTag(t, "editor", "", "", 4),
			v074TestText(t, "Read.", "Read.", 4), v074TestTag(t, "variant", "B", "name", 3),
			v074TestTag(t, "editor", "", "", 7), v074TestText(t, "Write.", "Write.", 7),
			v074TestClose(t, 3)), "current", []v074TestWant{{5, 1, 8}, {8, 1, 8}})
	})
}

func TestV074MultipleEndingsAndHistory(t *testing.T) {
	t.Run("next line does not supply comment text", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestTag(t, "editor", "", "", 2),
			v074TestText(t, "Comment on the next line.", "Comment on the next line.", 2)),
			"current", []v074TestWant{{3, 1, 8}})
	})
	t.Run("multiple comments", func(t *testing.T) {
		v074TestRun(t, v074TestDocument(t, v074TestTag(t, "editor", "", "", 2),
			v074TestTag(t, "editor", "Check.", "content", 2), v074TestTag(t, "editor", "", "", 2)),
			"current", []v074TestWant{{3, 1, 8}, {5, 1, 8}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v074TestDocument(t, v074TestTag(t, "editor", "", "", 2))
			if ending == "" {
				result.Lines[2].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v074TestRun(t, result, "first", []v074TestWant{{3, 1, 8}})
			v074TestRun(t, v074TestDocument(t, v074TestTag(t, "editor", "Check.", "content", 2)), "other", nil)
			v074TestRun(t, result, "second", []v074TestWant{{3, 1, 8}})
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("no lines unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v074TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v074TestRun(t, result, "current", nil)
		})
	}
}

// v074TestLine сохраняет готовую классификацию и родителя, не разбирая Raw.
func v074TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v074TestElement вычисляет длину входного элемента, но не ожидаемый диапазон.
func v074TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v074TestTag строит известный тег с явно указанным типом аргумента.
func v074TestTag(t *testing.T, tag, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v074TestElement(t, "tag", raw, strings.ToLower(tag), 1)}
	if argument != "" {
		elements = append(elements, v074TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v074TestLine(t, raw, "tag", parent, elements...)
}

// v074TestText сохраняет исходное и семантическое содержимое независимо.
func v074TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v074TestLine(t, raw, "content", parent, v074TestElement(t, "content", raw, value, 1))
}

// v074TestInvalid задаёт целиком неразобранное ошибочное объявление.
func v074TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v074TestLine(t, raw, "invalid", parent, v074TestElement(t, "unparsed", raw, "", 1))
}

// v074TestOpen строит блочную форму с заданным родителем.
func v074TestOpen(t *testing.T, tag string, parent int) model.Line {
	t.Helper()
	line := v074TestTag(t, tag, "", "", parent)
	line.Elements = append(line.Elements, v074TestElement(t, "block-open", "{", "{", len(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v074TestClose связывает закрывающую строку с открытием блока.
func v074TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v074TestLine(t, "}", "block-end", parent, v074TestElement(t, "block-close", "}", "}", 1))
}

// v074TestDocument добавляет обязательный контекст. Глубины следуют явным
// родителям; ошибки unparsed получают исходные диагностики и обратные ссылки.
func v074TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{
		v074TestTag(t, "dsl-version", "1.2", "version", 0),
		v074TestLine(t, "# Lesson", "heading", 0,
			v074TestElement(t, "heading-level", "#", "1", 1),
			v074TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			element := &line.Elements[j]
			element.ErrorIDs = append([]string{}, element.ErrorIDs...)
			if element.ElementType == "unparsed" {
				v074TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v074TestParserError связывает элемент с исходной ошибкой парсера.
func v074TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}},
	})
}

// v074TestRegistry использует только полный штатный каталог регистраций.
func v074TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v074TestRun сравнивает новые V074 после штатных Prepare/Run с явным ожиданием.
func v074TestRun(t *testing.T, result *model.Result, processingID string, want []v074TestWant) {
	t.Helper()
	before := v074TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v074TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v074TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V074" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V074 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v074TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V074 occurrence %d = %+v, want %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V074 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V074 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v074TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Из сравнения исключаются только добавления конвейера и производные
	// флаги/ссылки. Исходные строки, метаданные и прежняя история сохраняются.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v074TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v074TestSnapshot сохраняет представление модели для проверки неизменности.
func v074TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

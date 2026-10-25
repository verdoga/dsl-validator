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

// v128TestExpected содержит независимые Unicode-координаты с исключённым концом.
type v128TestExpected struct{ line, start, end int }

func TestV128RegistrationAndVersions(t *testing.T) {
	registry := v128TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V128" {
			continue
		}
		count++
		if definition.Message != "Между @hint и открывающей скобкой записано содержимое." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V128 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v128TestBlock(t, v128TestOpen(t, "hint", "Grammar tip", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v128TestSnapshot(t, result)
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil || v128TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version accepted or input changed: %v", err)
			}
		})
	}
}

func TestV128ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, argument, fragment string
		want                     []v128TestExpected
	}{
		{"reference", "", "@hint {\nCheck the subject.\nThen choose the verb.\n}", nil},
		{"example", "Grammar tip", "@hint Grammar tip {\nCheck the subject.\nThen choose the verb.\n}", []v128TestExpected{{3, 7, 18}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v128TestBlock(t, v128TestOpen(t, "hint", tc.argument, 2, 1))
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference or example changed")
			}
			v128TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV128PartialParsing(t *testing.T) {
	const raw = "@hint Grammar tip {"
	for _, tc := range []struct {
		name string
		line model.Line
	}{
		{"tag and block-open", v128TestOpen(t, "hint", "Grammar tip", 2, 1)},
		{"tag without block-open", v128TestLine(t, raw, "invalid", 2, 1,
			v128TestElement(t, "tag", "@hint", "hint", 1, 6), v128TestElement(t, "unparsed", " Grammar tip {", "", 6, 20))},
		{"whole unparsed", v128TestInvalid(t, raw, 2, 1)},
		{"block-open without tag", v128TestLine(t, raw, "block-start", 2, 1,
			v128TestElement(t, "unparsed", "@hint Grammar tip ", "", 1, 19), v128TestElement(t, "block-open", "{", "{", 19, 20))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v128TestRun(t, v128TestBlock(t, tc.line), "current", []v128TestExpected{{3, 7, 18}})
		})
	}
	t.Run("recognized opening takes precedence", func(t *testing.T) {
		// Вход сохраняет распознанное открытие и ошибочный промежуток с другой {.
		// V128 обязана использовать block-open; повторный разбор не заменяет его.
		opening := v128TestLine(t, `@hint "A{B" {`, "block-start", 2, 1,
			v128TestElement(t, "tag", "@hint", "hint", 1, 6),
			v128TestElement(t, "unparsed", `"A{B"`, "", 7, 12), v128TestElement(t, "block-open", "{", "{", 13, 14))
		v128TestRun(t, v128TestBlock(t, opening), "current", []v128TestExpected{{3, 7, 12}})
	})
}

func TestV128Ranges(t *testing.T) {
	for _, tc := range []struct {
		name, argument string
		want           []v128TestExpected
	}{
		{"one character", "X", []v128TestExpected{{3, 7, 8}}},
		{"several words and spaces", " Grammar  tip ", []v128TestExpected{{3, 8, 20}}},
		{"internal tab", "A\tB", []v128TestExpected{{3, 7, 10}}},
		{"ordinary quotes", `""`, []v128TestExpected{{3, 7, 9}}},
		{"zero width space is content", "\u200b", []v128TestExpected{{3, 7, 8}}},
		{"internal BOM is content", "\ufeff", []v128TestExpected{{3, 7, 8}}},
		{"escaped braces remain raw", `\{X\}`, []v128TestExpected{{3, 7, 12}}},
		{"combining mark is a column", "е\u0301😀", []v128TestExpected{{3, 7, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v128TestRun(t, v128TestBlock(t, v128TestOpen(t, "hint", tc.argument, 2, 1)), "current", tc.want)
		})
	}
	t.Run("case indentation and Unicode", func(t *testing.T) {
		opening := v128TestLine(t, "\t@HiNt  Я😀  {", "block-start", 2, 1,
			v128TestElement(t, "tag", "@HiNt", "hint", 2, 7), v128TestElement(t, "unparsed", "Я😀", "", 9, 11),
			v128TestElement(t, "block-open", "{", "{", 13, 14))
		v128TestRun(t, v128TestBlock(t, opening), "current", []v128TestExpected{{3, 9, 11}})
	})
	t.Run("Unicode whitespace edges", func(t *testing.T) {
		v128TestRun(t, v128TestBlock(t, v128TestInvalid(t, "@hint\u00a0X\u2003{", 2, 1)), "current", []v128TestExpected{{3, 7, 8}})
	})
	t.Run("decoded content does not introduce opening", func(t *testing.T) {
		line := v128TestLine(t, `@hint Text \{`, "tag", 2, 1, v128TestElement(t, "tag", "@hint", "hint", 1, 6),
			v128TestElement(t, "content", `Text \{`, "Text {", 7, 14))
		v128TestRun(t, v128TestDocument(t, line), "current", nil)
	})
}

func TestV128WhitespaceOnly(t *testing.T) {
	// Полный White_Space без LF/CR, которые по контракту не входят в Raw строки.
	// Перечень задан явно, независимо от Unicode-таблиц реализации.
	for _, gap := range []string{"", " ", "   ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007",
		"\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run(strconv.Quote(gap), func(t *testing.T) {
			raw := "@hint" + gap + "{"
			opening := v128TestInvalid(t, raw, 2, 1)
			if gap == " " || gap == "   " {
				// Корректные разделители не снабжаются вымышленной ошибкой разбора.
				brace := 6 + utf8.RuneCountInString(gap)
				opening = v128TestLine(t, raw, "block-start", 2, 1,
					v128TestElement(t, "tag", "@hint", "hint", 1, 6), v128TestElement(t, "block-open", "{", "{", brace, brace+1))
			}
			v128TestRun(t, v128TestBlock(t, opening), "current", nil)
		})
	}
}

func TestV128RawOpening(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []v128TestExpected
	}{
		{`@hint \{ {`, []v128TestExpected{{3, 7, 9}}},
		{`@hint \} {`, []v128TestExpected{{3, 7, 9}}},
		{`@hint \\{`, []v128TestExpected{{3, 7, 9}}},
		{`@hint \\\\{`, []v128TestExpected{{3, 7, 11}}},
		{`@hint \q{`, []v128TestExpected{{3, 7, 9}}},
		{`@hint "A{B" {`, []v128TestExpected{{3, 7, 9}}},
		{`@hint X { Y {`, []v128TestExpected{{3, 7, 8}}},
		{`@hint }{`, []v128TestExpected{{3, 7, 8}}},
		{"  @HINT Grammar tip {", []v128TestExpected{{3, 9, 20}}},
		{`@hintText X {`, nil}, {`@hint2 X {`, nil}, {`@hint-extra X {`, nil},
		{`\@hint X {`, nil}, {`Read @hint X {`, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v128TestRun(t, v128TestBlock(t, v128TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	t.Run("next physical line is not an opening", func(t *testing.T) {
		v128TestRun(t, v128TestDocument(t,
			v128TestLine(t, "@hint Text", "tag", 2, 1, v128TestElement(t, "tag", "@hint", "hint", 1, 6), v128TestElement(t, "content", "Text", "Text", 7, 11)),
			v128TestInvalid(t, "{", 2, 1), v128TestText(t, "Check the subject.", 2, 1), v128TestInvalid(t, "}", 2, 1)), "current", nil)
	})
}

func TestV128InlineAndLiteralText(t *testing.T) {
	// По DSL 1.22 три обратные косые черты перед { не открывают блок:
	// это корректная однострочная подсказка, а не invalid с ошибкой разбора.
	for _, tc := range []struct{ raw, value string }{
		{"Grammar tip", "Grammar tip"}, {`Text \{`, "Text {"}, {`Text \\\{`, `Text \{`}, {`\\\{`, `\{`},
		{`Text \}`, "Text }"}, {`Text \\`, `Text \`}, {`Text &#123;`, `Text &#123;`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			line := v128TestLine(t, "@hint "+tc.raw, "tag", 2, 1, v128TestElement(t, "tag", "@hint", "hint", 1, 6),
				v128TestElement(t, "content", tc.raw, tc.value, 7, 7+utf8.RuneCountInString(tc.raw)))
			v128TestRun(t, v128TestDocument(t, line), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@hint X \{`, `@hint X {`}, {`Read @hint X \{`, `Read @hint X {`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			line := v128TestLine(t, tc.raw, "content", 2, 1, v128TestElement(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v128TestRun(t, v128TestDocument(t, line), "current", nil)
		})
	}
	v128TestRun(t, v128TestBlock(t, v128TestOpen(t, "note", "Grammar tip", 2, 1)), "current", nil)
}

func TestV128NestingAndExclusions(t *testing.T) {
	t.Run("hint inside instr", func(t *testing.T) {
		v128TestRun(t, v128TestDocument(t, v128TestOpen(t, "instr", "", 2, 1), v128TestText(t, "Read the text.", 3, 2),
			v128TestOpen(t, "hint", "Grammar tip", 3, 2), v128TestText(t, "Check the subject.", 5, 3),
			v128TestText(t, "Then choose the verb.", 5, 3), v128TestClose(t, 5, 3), v128TestClose(t, 3, 2)),
			"current", []v128TestExpected{{5, 7, 18}})
	})
	for _, tag := range []string{"editor", "multifill", "EdItOr", "MuLtIfIlL"} {
		t.Run(tag, func(t *testing.T) {
			// Псевдооткрытие не создаёт блока внутри литерального контейнера.
			// Единственная отдельная } закрывает editor/multifill.
			body := []model.Line{v128TestOpen(t, tag, "", 2, 1), v128TestText(t, "@hint Grammar tip {", 3, 2),
				v128TestText(t, "Check the subject.", 3, 2), v128TestText(t, "Then choose the verb.", 3, 2), v128TestClose(t, 3, 2)}
			v128TestRun(t, v128TestDocument(t, body...), "excluded", nil)
			body = append(body, v128TestOpen(t, "hint", "Grammar tip", 2, 1), v128TestText(t, "Check the subject.", 8, 2),
				v128TestText(t, "Then choose the verb.", 8, 2), v128TestClose(t, 8, 2))
			v128TestRun(t, v128TestDocument(t, body...), "outside", []v128TestExpected{{8, 7, 18}})
		})
	}
	t.Run("editor inside variants", func(t *testing.T) {
		// DSL 4.9.8 разрешает editor до первой ветви; обе ветви непусты.
		v128TestRun(t, v128TestDocument(t, v128TestOpen(t, "variants", "", 2, 1), v128TestOpen(t, "editor", "", 3, 2),
			v128TestText(t, "@hint Grammar tip {", 4, 3), v128TestClose(t, 4, 3),
			v128TestLine(t, "@variant A", "tag", 3, 2, v128TestElement(t, "tag", "@variant", "variant", 1, 9), v128TestElement(t, "name", "A", "A", 10, 11)),
			v128TestText(t, "Read.", 7, 3),
			v128TestLine(t, "@variant B", "tag", 3, 2, v128TestElement(t, "tag", "@variant", "variant", 1, 9), v128TestElement(t, "name", "B", "B", 10, 11)),
			v128TestText(t, "Write.", 9, 3), v128TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV128BoundariesAndHistory(t *testing.T) {
	t.Run("first and last declarations", func(t *testing.T) {
		v128TestRun(t, v128TestDocument(t, v128TestOpen(t, "hint", "X", 2, 1), v128TestText(t, "Read.", 3, 2), v128TestClose(t, 3, 2),
			v128TestOpen(t, "hint", "X", 2, 1), v128TestText(t, "Write.", 6, 2), v128TestClose(t, 6, 2)),
			"current", []v128TestExpected{{3, 7, 8}, {6, 7, 8}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("eol/"+strconv.Quote(string(eol)), func(t *testing.T) {
			result := v128TestBlock(t, v128TestOpen(t, "hint", "Grammar tip", 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v128TestRun(t, result, "first", []v128TestExpected{{3, 7, 18}})
			v128TestRun(t, result, "second", []v128TestExpected{{3, 7, 18}})
		})
	}
	t.Run("empty input", func(t *testing.T) {
		result := v128TestDocument(t)
		result.Lines = []model.Line{}
		result.Document.LineCount, result.Document.Metadata.Title = nil, nil
		v128TestRun(t, result, "current", nil)
	})
	t.Run("blank lines", func(t *testing.T) {
		v128TestRun(t, v128TestDocument(t, v128TestLine(t, "", "blank", 2, 1), v128TestLine(t, " \t", "blank", 2, 1),
			v128TestText(t, "Read.", 2, 1)), "current", nil)
	})
}

// v128TestBlock добавляет неизменённое содержимое примера и его закрытие.
// При нераспознанном открытии строки остаются в шаге, а } также не разобрана:
// родитель block-end не подменяется строкой invalid вопреки контракту модели.
func v128TestBlock(t *testing.T, opening model.Line) *model.Result {
	t.Helper()
	if opening.LineType == "block-start" {
		return v128TestDocument(t, opening, v128TestText(t, "Check the subject.", 3, 2),
			v128TestText(t, "Then choose the verb.", 3, 2), v128TestClose(t, 3, 2))
	}
	return v128TestDocument(t, opening, v128TestText(t, "Check the subject.", 2, 1),
		v128TestText(t, "Then choose the verb.", 2, 1), v128TestInvalid(t, "}", 2, 1))
}

// v128TestDocument добавляет обязательные версию и заголовок по DSL 1.1–1.3,
// 1.30, 2.1.3. Типы строк и родители заданы явно; повторного разбора DSL нет.
func v128TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v128TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v128TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v128TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v128TestLine(t, "# Lesson", "heading", 0, 0, v128TestElement(t, "heading-level", "#", "1", 1, 2), v128TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			// Условный код фикстуры не приписывается реальному каталогу парсера.
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора конструкции.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
			})
		}
	}
	return result
}

// v128TestOpen составляет объявление из заданных имени и аргумента.
// У hint аргумент запрещён и сохраняется как unparsed; note допускает title.
// Вычисляемые длины принадлежат только входу, ожидаемые диапазоны заданы явно.
func v128TestOpen(t *testing.T, name, argument string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name + " "
	elements := []model.Element{v128TestElement(t, "tag", "@"+name, strings.ToLower(name), 1, len(name)+2)}
	if argument != "" {
		kind := model.ElementTypeUnparsed
		if name == "note" {
			kind = model.ElementTypeTitle
		}
		start := utf8.RuneCountInString(raw) + 1
		elements = append(elements, v128TestElement(t, kind, argument, argument, start, start+utf8.RuneCountInString(argument)))
		raw += argument + " "
	}
	brace := utf8.RuneCountInString(raw) + 1
	elements = append(elements, v128TestElement(t, "block-open", "{", "{", brace, brace+1))
	return v128TestLine(t, raw+"{", "block-start", parent, depth, elements...)
}

func v128TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v128TestLine(t, "}", "block-end", parent, depth, v128TestElement(t, "block-close", "}", "}", 1, 2))
}

func v128TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v128TestLine(t, raw, "invalid", parent, depth, v128TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

func v128TestText(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v128TestLine(t, raw, "content", parent, depth, v128TestElement(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

func v128TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v128TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

func v128TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v128TestRun использует полный штатный конвейер; сравнивает только V128
// текущего запуска и проверяет неизменность источника и прежней истории.
func v128TestRun(t *testing.T, result *model.Result, source string, want []v128TestExpected) {
	t.Helper()
	before := v128TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v128TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v128TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V128" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V128 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		message := "Между @hint и открывающей скобкой записано содержимое.\nконкретизация: У @hint нет названия; текст подсказки размещается внутри блока"
		if entry.Location == nil || *entry.Location != location || entry.Message != message ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v and exact V128 metadata", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V128 did not mark document and line")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v128TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Флаги и ссылки полного каталога проверены CheckModel. В независимой копии
	// возвращаем прежние производные поля и убираем только записи нового запуска.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			for _, id := range restored.Lines[i].Elements[j].ErrorIDs {
				for _, entry := range got {
					if entry.ID == id {
						t.Fatal("line-scoped V128 linked to an element")
					}
				}
			}
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v128TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v128TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

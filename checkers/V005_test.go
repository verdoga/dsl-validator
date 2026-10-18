package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// Тексты ожиданий заданы независимо от реализации диагностики.
const (
	v005WantOpen  = "Открывающая скобка не имеет закрывающей"
	v005WantClose = "Закрывающая скобка не имеет открывающей"
)

// v005Expected задаёт явный исходный диапазон [start,end) одной строки.
type v005Expected struct {
	line, start, end int
	detail           string
}

func TestV005Registration(t *testing.T) {
	definitions, err := v005Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V005" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Нарушено оформление фигурных скобок." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V005 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V005 definitions = %d, want 1", matches)
	}
}

func TestV005ReferencesAndLineEndings(t *testing.T) {
	// Эталоны и пример таблицы сохранены целиком. v005Document добавляет
	// только обязательные версию и заголовок (DSL 1.1–1.3, 1.30, 2.1.3).
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v005Expected
	}{
		{"note reference", []model.Line{
			v005Open(t, "note", 2, 1), v005Text(t, "First line.", 3, 2),
			v005Text(t, "Second line.", 3, 2), v005Close(t, 3, 2),
		}, nil},
		{"multifill reference", []model.Line{
			v005Open(t, "multifill", 2, 1),
			v005Line(t, "Answer: _____{word}", "content", 3, 2,
				v005Element(t, "content", "Answer: ", "Answer: ", 1, 9),
				v005Element(t, "placeholder", "_____{word}", "word", 9, 20)),
			v005Close(t, 3, 2),
		}, nil},
		{"unclosed example", []model.Line{
			v005Open(t, "note", 2, 1), v005Text(t, "First line.", 3, 2),
			v005Text(t, "Second line.", 3, 2),
		}, []v005Expected{{3, 7, 8, v005WantOpen}}},
	} {
		for _, ending := range []struct {
			name string
			eol  model.LineEnding
			last model.LineEnding
		}{
			{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
			{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
			{"mixed", "\r\n", "\n"},
		} {
			t.Run(tc.name+"/"+ending.name, func(t *testing.T) {
				result := v005Document(t, tc.body...)
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending.eol
				}
				result.Lines[len(result.Lines)-1].LineEnding = ending.last
				v005Run(t, result, "current", tc.want)
			})
		}
	}
}

func TestV005ParityInUnparsedLines(t *testing.T) {
	// Эти строки нарушают исходные правила оформления/экранирования DSL
	// 1.18–1.21 или 4.3. Их явно заданное восстановление — invalid/unparsed
	// с исходной ошибкой. По уточнению A парность проверяется независимо от
	// этих ошибок; наличие пары не обязано означать корректность всего DSL.
	for _, tc := range []struct {
		name string
		raw  string
		want []v005Expected
	}{
		{"inline block", "@note { Text. }", nil},
		{"block declaration without opening", "@table", nil},
		{"no underscores", "{word}", nil},
		{"four underscores", "____{word}", nil},
		{"five underscores outside multifill", "_____{word}", nil},
		{"six underscores", "______{word}", nil},
		{"space before opening", "_____ {word}", nil},
		{"escaped underscores do not hide braces", `\_____{word}`, nil},
		{"nested braces", "{a{b}c}", nil},
		{"separate pairs", "{} {}", nil},
		{"empty pair", "{}", nil},
		{"opening on its own", "{", []v005Expected{{3, 1, 2, v005WantOpen}}},
		{"closing on its own", "}", []v005Expected{{3, 1, 2, v005WantClose}}},
		{"wrong order", "}{", []v005Expected{{3, 1, 2, v005WantClose}, {3, 2, 3, v005WantOpen}}},
		{"two openings", "{{", []v005Expected{{3, 1, 2, v005WantOpen}, {3, 2, 3, v005WantOpen}}},
		{"nearest opening closes", "{{}", []v005Expected{{3, 1, 2, v005WantOpen}}},
		{"two excess closings", "{}}}", []v005Expected{{3, 3, 4, v005WantClose}, {3, 4, 5, v005WantClose}}},
		{"closing with trailing content", "} tail", []v005Expected{{3, 1, 2, v005WantClose}}},
		{"trailing block content has no separate finding", "@note { x", []v005Expected{{3, 7, 8, v005WantOpen}}},
		{"four underscores unclosed", "____{word", []v005Expected{{3, 5, 6, v005WantOpen}}},
		{"five underscores unclosed", "_____{word", []v005Expected{{3, 6, 7, v005WantOpen}}},
		{"six underscores unclosed", "______{word", []v005Expected{{3, 7, 8, v005WantOpen}}},
		{"escaped underscores unclosed", `\_____{word`, []v005Expected{{3, 7, 8, v005WantOpen}}},
		{"quotes do not hide opening", `"{"`, []v005Expected{{3, 2, 3, v005WantOpen}}},
		{"quotes do not hide closing", `"}"`, []v005Expected{{3, 2, 3, v005WantClose}}},
		{"spaces and Unicode columns", " Я🙂 {", []v005Expected{{3, 5, 6, v005WantOpen}}},
		{"tabs retain one column", " \t{\t ", []v005Expected{{3, 3, 4, v005WantOpen}}},
		{"combining character retains a column", "е\u0301{", []v005Expected{{3, 3, 4, v005WantOpen}}},
		{"two backslashes before opening", `\\{`, []v005Expected{{3, 3, 4, v005WantOpen}}},
		{"two backslashes before closing", `\\}`, []v005Expected{{3, 3, 4, v005WantClose}}},
		{"four backslashes", `\\\\{`, []v005Expected{{3, 5, 6, v005WantOpen}}},
		{"backslash before ordinary character", `\q{`, []v005Expected{{3, 3, 4, v005WantOpen}}},
		{"escaped closing does not close opening", `{\}`, []v005Expected{{3, 1, 2, v005WantOpen}}},
		{"escaped opening does not match closing", `\{}`, []v005Expected{{3, 3, 4, v005WantClose}}},
		{"escaped tag does not hide brace", `\@note {`, []v005Expected{{3, 8, 9, v005WantOpen}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v005Run(t, v005Document(t, v005Invalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV005EscapesUseRaw(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"escaped braces", `\{x\}`, `{x}`},
		{"escaped unmatched opening", `\{`, `{`},
		{"escaped unmatched closing", `\}`, `}`},
		{"three backslashes", `\\\{`, `\{`},
		{"five backslashes", `\\\\\}`, `\\}`},
		{"literal answer", `\_____\{word\}`, `_____{word}`},
		{"escaped backslash at EOL", `text \\`, `text \`},
		{"other Unicode braces", "｛word｝", "｛word｝"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v005Line(t, tc.raw, "content", 2, 1,
				v005Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v005Run(t, v005Document(t, line), "current", nil)
		})
	}
}

func TestV005StackAcrossLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v005Expected
	}{
		{"standalone opening and closing", []model.Line{
			v005Invalid(t, "{", 2, 1), v005Text(t, "Text.", 2, 1), v005Invalid(t, "}", 2, 1),
		}, nil},
		{"answer across lines", []model.Line{
			v005Open(t, "multifill", 2, 1), v005Invalid(t, "_____{word", 3, 2),
			v005Invalid(t, "}", 3, 2), v005Close(t, 3, 2),
		}, nil},
		{"only one closing after broken answer", []model.Line{
			v005Open(t, "multifill", 2, 1), v005Invalid(t, "_____{word", 3, 2), v005Close(t, 3, 2),
		}, []v005Expected{{3, 12, 13, v005WantOpen}}},
		{"block and answer remain open", []model.Line{
			v005Open(t, "multifill", 2, 1), v005Invalid(t, "_____{word", 3, 2),
		}, []v005Expected{{3, 12, 13, v005WantOpen}, {4, 6, 7, v005WantOpen}}},
		{"blank inside block and comment after it", []model.Line{
			v005Open(t, "note", 2, 1), v005Line(t, "", "blank", 3, 2),
			v005Text(t, "Text.", 3, 2), v005Close(t, 3, 2),
			v005Line(t, "@editor Comment.", "tag", 2, 1,
				v005Element(t, "tag", "@editor", "editor", 1, 8),
				v005Element(t, "content", "Comment.", "Comment.", 9, 17)),
		}, nil},
		{"backslash cannot escape next physical line", []model.Line{
			v005Text(t, `text \`, 2, 1), v005Invalid(t, "{", 2, 1),
		}, []v005Expected{{4, 1, 2, v005WantOpen}}},
		{"tag case does not affect pairing", []model.Line{
			v005Open(t, "NoTe", 2, 1), v005Text(t, "Text.", 3, 2), v005Close(t, 3, 2),
		}, nil},
		{"opening on last line", []model.Line{v005Open(t, "note", 2, 1)},
			[]v005Expected{{3, 7, 8, v005WantOpen}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v005Run(t, v005Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV005NestedBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		ends []model.Line
		want []v005Expected
	}{
		{"both open", nil, []v005Expected{{3, 13, 14, v005WantOpen}, {4, 7, 8, v005WantOpen}}},
		{"inner closed", []model.Line{v005Close(t, 4, 3)}, []v005Expected{{3, 13, 14, v005WantOpen}}},
		{"both closed", []model.Line{v005Close(t, 4, 3), v005Close(t, 3, 2)}, nil},
		{"two closings on one line", []model.Line{v005Invalid(t, "}}", 4, 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{
				v005Line(t, "@fragment f {", "block-start", 2, 1,
					v005Element(t, "tag", "@fragment", "fragment", 1, 10),
					v005Element(t, "identifier", "f", "f", 11, 12),
					v005Element(t, "block-open", "{", "{", 13, 14)),
				v005Open(t, "note", 3, 2), v005Text(t, "Text.", 4, 3),
			}
			v005Run(t, v005Document(t, append(body, tc.ends...)...), "current", tc.want)
		})
	}
}

func TestV005MultifillHasNoSpecialPairing(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v005Expected
	}{
		{"four underscores", "____{word}", nil},
		{"six underscores", "______{word}", nil},
		{"nested answer", "_____{a{b}}", nil},
		{"nested answer not closed", "_____{a{b}", []v005Expected{{3, 12, 13, v005WantOpen}}},
		{"escaped underscores still allow pair", `\_____{word}`, nil},
		{"empty answer still paired", "_____{}", nil},
		{"space before brace", "_____ {word}", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v005Run(t, v005Document(t, v005Open(t, "multifill", 2, 1),
				v005Invalid(t, tc.raw, 3, 2), v005Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV005ResourcePaths(t *testing.T) {
	for _, tc := range []struct{ name, prefix, raw, value string }{
		{"opening", "@resource-dir ", "assets/{x", "assets/{x"},
		{"closing", "@resource-dir ", "assets/}x", "assets/}x"},
		{"backslash is literal", "@resource-dir ", `C:\assets\{x`, `C:\assets\{x`},
		{"quoted comma and brace", "@resource-dir ", `"assets,a/{x"`, "assets,a/{x"},
		{"case indentation and separators", " \t@ReSoUrCe-DiR  ", "assets/{x", "assets/{x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v005Document(t)
			start := strings.Index(tc.prefix, "@") + 1
			pathStart := utf8.RuneCountInString(tc.prefix) + 1
			path := v005Line(t, tc.prefix+tc.raw, "tag", 0, 0,
				v005Element(t, "tag", tc.prefix[start-1:start+12], "resource-dir", start, start+13),
				v005Element(t, "resource-path", tc.raw, tc.value, pathStart, pathStart+utf8.RuneCountInString(tc.raw)))
			result = v005Model(t, result.Lines[0], path, result.Lines[1])
			result.Document.Metadata.ResourceDirs = []string{tc.value}
			v005Run(t, result, "current", nil)
		})
	}
}

func TestV005ResourceExceptionRequiresDeclaration(t *testing.T) {
	// До заголовка допустим блочный editor. Строки внутри него, даже похожие
	// на заголовок или метаданные, являются содержимым (DSL 1.24, 2.8.11).
	base := v005Document(t)
	result := v005Model(t, base.Lines[0], v005Open(t, "editor", 0, 0),
		v005Text(t, "# Literal heading", 2, 1),
		v005Invalid(t, "@resource-dir {x", 2, 1), v005Close(t, 2, 1), base.Lines[1])
	v005Run(t, result, "current", []v005Expected{{2, 9, 10, v005WantOpen}})

	for _, tc := range []struct {
		name string
		body []model.Line
		want []v005Expected
	}{
		{"inside multifill", []model.Line{v005Open(t, "multifill", 2, 1),
			v005Invalid(t, "@resource-dir {x", 3, 2), v005Close(t, 3, 2)},
			[]v005Expected{{3, 12, 13, v005WantOpen}}},
		{"after heading", []model.Line{v005Invalid(t, "@resource-dir {x", 2, 1)},
			[]v005Expected{{3, 15, 16, v005WantOpen}}},
		{"comment is checked", []model.Line{v005Line(t, "@editor {x", "tag", 2, 1,
			v005Element(t, "tag", "@editor", "editor", 1, 8),
			v005Element(t, "content", "{x", "{x", 9, 11))},
			[]v005Expected{{3, 9, 10, v005WantOpen}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v005Run(t, v005Document(t, tc.body...), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		name, raw string
		column    int
	}{
		{"escaped tag", `\@resource-dir {x`, 16},
		{"longer tag name", "@resource-directory {x", 21},
		{"tab is not a tag separator", "@resource-dir\t{x", 15},
		{"missing tag separator", "@resource-dir{x", 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v005Model(t, base.Lines[0], v005Invalid(t, tc.raw, 0, 0), base.Lines[1])
			v005Run(t, result, "current", []v005Expected{{2, tc.column, tc.column + 1, v005WantOpen}})
		})
	}
}

func TestV005ResourceDeclarationAfterEditor(t *testing.T) {
	base := v005Document(t)
	result := v005Model(t, base.Lines[0], v005Open(t, "editor", 0, 0),
		v005Text(t, "# Literal heading", 2, 1), v005Text(t, "@resource-dir path", 2, 1),
		v005Close(t, 2, 1),
		v005Line(t, "@resource-dir assets/{x", "tag", 0, 0,
			v005Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v005Element(t, "resource-path", "assets/{x", "assets/{x", 15, 24)), base.Lines[1])
	result.Document.Metadata.ResourceDirs = []string{"assets/{x"}
	v005Run(t, result, "current", nil)
}

func TestV005FirstPhysicalLineAndRootClosing(t *testing.T) {
	base := v005Document(t)
	first := v005Line(t, "@dsl-version 1.2 }", "tag", 0, 0,
		v005Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
		v005Element(t, "version", "1.2", "1.2", 14, 17),
		model.Element{ElementType: "unparsed", Raw: "}", Start: 18, End: 19, ErrorIDs: []string{}})
	v005Run(t, v005Model(t, first, base.Lines[1]), "current",
		[]v005Expected{{1, 18, 19, v005WantClose}})

	// Контракт допускает корневую block-end при сохранённой исходной ошибке.
	// V005 повторяет сообщение о непарности, не читая эту историю или ParentLine.
	result := v005Document(t, v005Close(t, 0, 0))
	result.Document.HasErrors, result.Lines[2].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "parser-close", Source: "parser", DiagnosticCode: "fixture-parser-close",
		SeverityLevel: "error", DiagnosticScope: "line", Message: "Закрытие без блока.",
		Location: &model.Location{Start: model.Position{Line: 3, Column: 1},
			End: model.Position{Line: 3, Column: 2}}, RelatedLocations: []model.Location{},
	})
	v005Run(t, result, "current", []v005Expected{{3, 1, 2, v005WantClose}})
}

func TestV005OtherTechnicalValuesAreNotExcluded(t *testing.T) {
	// Исключение путей относится только к resource-dir, а не к SOURCE медиа.
	media := v005Line(t, `@media image "asset{.png"`, "tag", 2, 1,
		v005Element(t, "tag", "@media", "media", 1, 7),
		v005Element(t, "media-type", "image", "image", 8, 13),
		v005Element(t, "source", `"asset{.png"`, "asset{.png", 14, 26))
	v005Run(t, v005Document(t, media), "current", []v005Expected{{3, 20, 21, v005WantOpen}})

	// Заголовок содержит проверяемые скобки; его семантика не используется.
	base := v005Document(t)
	heading := v005Line(t, "# Unit {", "heading", 0, 0,
		v005Element(t, "heading-level", "#", "1", 1, 2),
		v005Element(t, "title", "Unit {", "Unit {", 3, 9))
	result := v005Model(t, base.Lines[0], heading)
	title := "Unit {"
	result.Document.Metadata.Title = &title
	v005Run(t, result, "current", []v005Expected{{2, 8, 9, v005WantOpen}})
}

func TestV005NoLinesAndVersionSelection(t *testing.T) {
	result := v005Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v005Run(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v005Document(t)
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v005Snapshot(t, result)
			checks, err := validator.Prepare(result, v005Registry(t))
			if err == nil || checks != nil || v005Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV005RepeatedRuns(t *testing.T) {
	result := v005Document(t, v005Open(t, "note", 2, 1), v005Text(t, "Text.", 3, 2))
	want := []v005Expected{{3, 7, 8, v005WantOpen}}
	v005Run(t, result, "first", want)
	v005Run(t, result, "second", want)
	v005Run(t, v005Document(t, v005Invalid(t, "}", 2, 1)), "current",
		[]v005Expected{{3, 1, 2, v005WantClose}})
	v005Run(t, v005Document(t, v005Text(t, "Text.", 2, 1)), "current", nil)
}

func TestV005CoexistsWithOtherChecks(t *testing.T) {
	result := v005Document(t, v005Open(t, "note", 2, 1),
		v005Text(t, "Wait. . .", 3, 2), v005Line(t, "", "blank", 3, 2),
		v005Line(t, "", "blank", 3, 2), v005Text(t, "End.", 3, 2))
	v005Run(t, result, "current", []v005Expected{{3, 7, 8, v005WantOpen}})
	for _, tc := range []struct {
		code     diagnostics.Code
		severity diagnostics.Severity
		scope    diagnostics.Scope
		message  string
		location model.Location
	}{
		{"V003", "warning", "line", "Точки многоточия разделены пробельными символами.",
			model.Location{Start: model.Position{Line: 4, Column: 5}, End: model.Position{Line: 4, Column: 10}}},
		{"V004", "recommendation", "block", "Несколько пустых строк подряд.",
			model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 6, Column: 1}}},
	} {
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "current" || entry.DiagnosticCode != tc.code {
				continue
			}
			count++
			if entry.Location == nil || *entry.Location != tc.location || entry.Message != tc.message ||
				entry.SeverityLevel != tc.severity || entry.DiagnosticScope != tc.scope || entry.Fatal || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
		if count != 1 {
			t.Errorf("%s count = %d, want 1", tc.code, count)
		}
	}
}

// v005Registry создаёт реестр только через штатный каталог регистраций.
func v005Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v005Document добавляет обязательные метаданные перед явно заданным телом.
func v005Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v005Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v005Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v005Element(t, "version", "1.2", "1.2", 14, 17)),
		v005Line(t, "# Unit 1", "heading", 0, 0,
			v005Element(t, "heading-level", "#", "1", 1, 2),
			v005Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v005Model(t, append(lines, body...)...)
}

// v005Model нумерует заданные строки и связывает явно указанные unparsed
// с тестовой историей ошибок. Raw не разбирается, дерево и типы не выводятся.
func v005Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, title := "1.2", "UTF-8", "Unit 1"
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", i+1, j+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Ошибка исходной разметки.",
				Location: &model.Location{Start: model.Position{Line: i + 1, Column: element.Start},
					End: model.Position{Line: i + 1, Column: element.End}}, RelatedLocations: []model.Location{},
			})
		}
	}
	return result
}

// v005Line сохраняет явно заданные тип, родителя и элементы физической строки.
func v005Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v005Element задаёт исходное и семантическое значения и диапазон [start,end).
func v005Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v005Text задаёт содержимое без преобразования семантического значения.
func v005Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v005Line(t, raw, "content", parent, depth,
		v005Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v005Invalid задаёт восстановление конкретной повреждённой строки целиком.
// История и ссылки добавляются v005Model, а не вычисляются из синтаксиса Raw.
func v005Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v005Line(t, raw, "invalid", parent, depth, model.Element{
		ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{},
	})
}

// v005Open задаёт блочное объявление без параметров с согласованными элементами.
func v005Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v005Line(t, "@"+tag+" {", "block-start", parent, depth,
		v005Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, end),
		v005Element(t, "block-open", "{", "{", end+1, end+2))
}

// v005Close задаёт закрытие с явно указанным родителем из входной модели.
func v005Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v005Line(t, "}", "block-end", parent, depth, v005Element(t, "block-close", "}", "}", 1, 2))
}

// v005Run проверяет V005 текущего запуска штатного конвейера, точные сообщения,
// диапазоны, флаги ошибок, неизменность исходника, элементов и прежней истории.
func v005Run(t *testing.T, result *model.Result, processingID string, want []v005Expected) {
	t.Helper()
	before := v005Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v005Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v005Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for _, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if entry.DiagnosticCode == "V005" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V005 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "Нарушено оформление фигурных скобок.\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v, location %+v; want %+v, %q", i, entry, entry.Location, location, expected.detail)
		}
	}
	if len(result.Processing) != len(original.Processing)+1 || len(result.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("unexpected processing or diagnostic history length")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	for _, occurrence := range want {
		expected.Document.HasErrors = true
		expected.Lines[occurrence.line-1].HasErrors = true
	}
	v005OtherFindings(t, &expected, result.Diagnostics[len(original.Diagnostics):], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:len(original.Processing)], result.Diagnostics[:len(original.Diagnostics)]
	if v005Snapshot(t, &previous) != v005Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v005Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v005Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v005OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v005OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V005" {
			continue
		}
		if entry.SeverityLevel == "error" {
			expected.Document.HasErrors = true
			if entry.Location != nil {
				expected.Lines[entry.Location.Start.Line-1].HasErrors = true
			}
		}
		if entry.DiagnosticScope != "element" {
			continue
		}
		if entry.Location == nil {
			t.Fatal("element diagnostic has no location")
		}
		matched := false
		for i := range expected.Lines[entry.Location.Start.Line-1].Elements {
			element := &expected.Lines[entry.Location.Start.Line-1].Elements[i]
			if element.Start == entry.Location.Start.Column && element.End == entry.Location.End.Column {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched = true
				break
			}
		}
		if !matched {
			t.Fatal("element diagnostic does not identify an element")
		}
	}
	for i := range expected.Lines {
		for j := range expected.Lines[i].Elements {
			slices.SortStableFunc(expected.Lines[i].Elements[j].ErrorIDs, func(a, b string) int { return order[a] - order[b] })
		}
	}
}

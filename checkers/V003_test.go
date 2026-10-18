package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV003Registration(t *testing.T) {
	definitions, err := v003Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V003" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Message != "Точки многоточия разделены пробельными символами." ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V003 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V003 definitions = %d, want 1", matches)
	}
}

func TestV003Chains(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Wait... Then…", nil},
		{"example", "Wait. . . Then.. .", [][3]int{{3, 5, 10}, {3, 15, 19}}},
		{"both gaps", ". . .", [][3]int{{3, 1, 6}}},
		{"last gap", ".. .", [][3]int{{3, 1, 5}}},
		{"first gap", ". ..", [][3]int{{3, 1, 5}}},
		{"no dots", "Wait", nil},
		{"one dot", ".", nil},
		{"two dots", ". .", nil},
		{"no gaps", "...", nil},
		{"four spaced dots", ". . . .", nil},
		{"four dots with final gap", "... .", nil},
		{"four dots with initial gap", ". ...", nil},
		{"four dots with middle gap", ".. ..", nil},
		{"five dots", ".. . ..", nil},
		{"six dots are one chain", ". . .   . . .", nil},
		{"punctuation separates chains", ". . .;.. .", [][3]int{{3, 1, 6}, {3, 7, 11}}},
		{"long chain does not hide following chain", ". . . .; . . .", [][3]int{{3, 10, 15}}},
		{"tab and space", ".\t. .", [][3]int{{3, 1, 6}}},
		{"multiple spaces and tab", ".  .\t.", [][3]int{{3, 1, 7}}},
		{"multiple tabs", ".\t\t..", [][3]int{{3, 1, 6}}},
		{"mixed separator sequence", ".\t\u00a0.\u2003\t.", [][3]int{{3, 1, 8}}},
		{"vertical tab is not a separator", ".\u000b. .", nil},
		{"form feed is not a separator", ".\u000c. .", nil},
		{"NEL is not a separator", ".\u0085. .", nil},
		{"line separator is not Zs", ".\u2028. .", nil},
		{"paragraph separator is not Zs", ".\u2029. .", nil},
		{"zero width space is not Zs", ".\u200b. .", nil},
		{"non-Zs ends a qualifying chain", ". . .\u2028.", [][3]int{{3, 1, 6}}},
		{"ellipsis is not a dot", ". . …", nil},
		{"ellipsis separates chains", ". . … . . .", [][3]int{{3, 7, 12}}},
		{"Unicode source columns", "Я😀e\u0301. . .", [][3]int{{3, 5, 10}}},
		{"uppercase example", "WAIT. . . THEN.. .", [][3]int{{3, 5, 10}, {3, 15, 19}}},
		{"literal HTML entity", ".&#32;. .", nil},
		{"literal backslash t", `.\t. .`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v003Run(t, v003Document(t, v003Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV003SpaceSeparators(t *testing.T) {
	// Явный нормативный набор Zs, независимо от unicode.Zs в реализации.
	for _, separator := range []rune{
		'\u0020', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005',
		'\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u202f', '\u205f', '\u3000',
	} {
		t.Run(fmt.Sprintf("U+%04X", separator), func(t *testing.T) {
			raw := "." + string(separator) + "." + string(separator) + "."
			v003Run(t, v003Document(t, v003Text(t, raw, 2, 1)), "current", [][3]int{{3, 1, 6}})
		})
	}
}

func TestV003RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty physical line", []model.Line{v003Line(t, "", "blank", 2, 1)}, nil},
		{"blank whitespace", []model.Line{v003Line(t, " \t ", "blank", 2, 1)}, nil},
		{"empty line before content", []model.Line{
			v003Line(t, "", "blank", 2, 1), v003Text(t, ". . .", 2, 1),
		}, [][3]int{{4, 1, 6}}},
		{"outside spaces are not part of chain", []model.Line{
			v003Line(t, "  . . .  ", "content", 2, 1, v003Element(t, "content", ". . .", ". . .", 3, 8)),
		}, [][3]int{{3, 3, 8}}},
		{"outside tabs are not internal gaps", []model.Line{
			v003Line(t, "\t...\t", "content", 2, 1, v003Element(t, "content", "...", "...", 2, 5)),
		}, nil},
		{"gaps do not join across lines", []model.Line{v003Text(t, ". .", 2, 1), v003Text(t, ".", 2, 1)}, nil},
		{"neighboring qualifying lines stay separate", []model.Line{v003Text(t, ". . .", 2, 1), v003Text(t, ".. .", 2, 1)}, [][3]int{{3, 1, 6}, {4, 1, 5}}},
		// DSL 1.21–1.22: экранирование изменяет Value, но не Raw и его колонки.
		{"escaped braces preserve source columns", []model.Line{
			v003Line(t, `\{. . .\}`, "content", 2, 1, v003Element(t, "content", `\{. . .\}`, "{. . .}", 1, 10)),
		}, [][3]int{{3, 3, 8}}},
		{"single-line comment", []model.Line{
			v003Line(t, "@editor Wait. . .", "tag", 2, 1,
				v003Element(t, "tag", "@editor", "editor", 1, 8), v003Element(t, "content", "Wait. . .", "Wait. . .", 9, 18)),
		}, [][3]int{{3, 13, 18}}},
		{"uppercase tag with text at section end", []model.Line{
			v003Line(t, "@EDITOR . . .", "tag", 2, 1,
				v003Element(t, "tag", "@EDITOR", "editor", 1, 8), v003Element(t, "content", ". . .", ". . .", 9, 14)),
		}, [][3]int{{3, 9, 14}}},
		{"block comment", []model.Line{
			v003Open(t, "editor", 2, 1), v003Text(t, "Wait. . .", 3, 2), v003Close(t, 3, 2),
		}, [][3]int{{4, 5, 10}}},
		// DSL 2.8.11: похожая на @media строка комментария — литеральное содержимое.
		{"technical-looking comment content is included", []model.Line{
			v003Open(t, "editor", 2, 1), v003Text(t, `@media audio ". . ."`, 3, 2), v003Close(t, 3, 2),
		}, [][3]int{{4, 15, 20}}},
		// DSL 3.8.6, 3.8.11: собственная инструкция и один вложенный @hint.
		{"nested hint", []model.Line{
			v003Open(t, "instr", 2, 1), v003Text(t, "Read the sentence.", 3, 2),
			v003Open(t, "hint", 3, 2), v003Text(t, "Wait. . .", 5, 3),
			v003Close(t, 5, 3), v003Close(t, 3, 2),
		}, [][3]int{{6, 5, 10}}},
		// DSL 3.2.8: непустое содержимое с обеих сторон разделителя.
		{"block separator", []model.Line{
			v003Open(t, "example", 2, 1), v003Text(t, ". .", 3, 2),
			v003Line(t, "---", "separator", 3, 2), v003Text(t, ".", 3, 2), v003Close(t, 3, 2),
		}, nil},
		{"block boundaries", []model.Line{
			v003Open(t, "example", 2, 1), v003Text(t, ". . .", 3, 2),
			v003Close(t, 3, 2), v003Text(t, ".. .", 2, 1),
		}, [][3]int{{4, 1, 6}, {6, 1, 5}}},
		{"title before opening brace", []model.Line{
			v003Line(t, "@example . . . {", "block-start", 2, 1,
				v003Element(t, "tag", "@example", "example", 1, 9),
				v003Element(t, "title", ". . .", ". . .", 10, 15), v003Element(t, "block-open", "{", "{", 16, 17)),
			v003Text(t, "Wait", 3, 2), v003Close(t, 3, 2),
		}, [][3]int{{3, 10, 15}}},
		// DSL 4.3.7–4.3.9: служебный ответ не исключён условиями V003.
		{"placeholder answer", []model.Line{
			v003Open(t, "multifill", 2, 1),
			v003Line(t, "_____{. . .}", "content", 3, 2, v003Element(t, "placeholder", "_____{. . .}", ". . .", 1, 13)),
			v003Close(t, 3, 2),
		}, [][3]int{{4, 7, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v003Run(t, v003Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV003HeadingAndName(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v003Document(t, v003Text(t, "Wait", 2, 1))
		result.Lines[1] = v003Line(t, "# Wait. . .", "heading", 0, 0,
			v003Element(t, "heading-level", "#", "1", 1, 2), v003Element(t, "title", "Wait. . .", "Wait. . .", 3, 12))
		result.Lines[1].Number = 2
		title := "Wait. . ."
		result.Document.Metadata.Title = &title
		v003Run(t, result, "current", [][3]int{{2, 7, 12}})
	})
	t.Run("section name", func(t *testing.T) {
		result := v003Document(t)
		result.Lines = append(result.Lines[:1],
			v003Line(t, "@section Wait. . .", "tag", 0, 0,
				v003Element(t, "tag", "@section", "section", 1, 9), v003Element(t, "name", "Wait. . .", "Wait. . .", 10, 19)),
			result.Lines[1], v003Text(t, "Wait", 3, 1))
		v003NumberLines(t, result)
		section := "Wait. . ."
		result.Document.Metadata.Section = &section
		v003Run(t, result, "current", [][3]int{{2, 14, 19}})
	})
}

func TestV003TechnicalElements(t *testing.T) {
	// DSL 1.9, 1.29, 1.34, 1.35, 3.1: технические значения сохраняют допустимый
	// синтаксис; пробельные цепочки точек находятся только в путях, где они разрешены.
	for _, tc := range []struct {
		name string
		text string
		want [][3]int
	}{
		{"excluded paths", "Wait... Then…", nil},
		{"included text after excluded paths", ". . .", [][3]int{{7, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v003Document(t)
			result.Lines = []model.Line{
				result.Lines[0],
				v003Line(t, "@document-id a...", "tag", 0, 0,
					v003Element(t, "tag", "@document-id", "document-id", 1, 13), v003Element(t, "identifier", "a...", "a...", 14, 18)),
				v003Line(t, "@order 12", "tag", 0, 0,
					v003Element(t, "tag", "@order", "order", 1, 7), v003Element(t, "number", "12", "12", 8, 10)),
				v003Line(t, `@resource-dir "a. . ."`, "tag", 0, 0,
					v003Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v003Element(t, "resource-path", `"a. . ."`, "a. . .", 15, 23)),
				result.Lines[1],
				v003Line(t, `@media audio "a. . .mp3"`, "tag", 5, 1,
					v003Element(t, "tag", "@media", "media", 1, 7), v003Element(t, "media-type", "audio", "audio", 8, 13),
					v003Element(t, "source", `"a. . .mp3"`, "a. . .mp3", 14, 25)),
				v003Text(t, tc.text, 5, 1),
			}
			v003NumberLines(t, result)
			id, order := "a...", "12"
			result.Document.Metadata.DocumentID, result.Document.Metadata.Order = &id, &order
			result.Document.Metadata.ResourceDirs = []string{"a. . ."}
			v003Run(t, result, "current", tc.want)
		})
	}
}

func TestV003Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want [][3]int
	}{
		{"unparsed does not report", "Wait", nil},
		{"unparsed does not hide next line", ". . .", [][3]int{{4, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.16, 1.29: неизвестное имя — ошибка. Код ошибки тестовой
			// истории непрозрачен для валидатора; unparsed сохраняет весь фрагмент.
			line := v003Line(t, "@unknown . . .", "invalid", 2, 1, model.Element{
				ElementType: "unparsed", Raw: "@unknown . . .", Start: 1, End: 15,
				ErrorIDs: []string{"parser-error"},
			})
			line.HasErrors = true
			result := v003Document(t, line, v003Text(t, tc.text, 2, 1))
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{
				ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный DSL-тег.",
				Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 15}},
				RelatedLocations: []model.Location{},
			}}
			v003Run(t, result, "current", tc.want)
		})
	}
}

func TestV003LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}},
		{"LF without final EOL", [3]model.LineEnding{"\n", "\n", ""}},
		{"CRLF without final EOL", [3]model.LineEnding{"\r\n", "\r\n", ""}},
		{"mixed endings", [3]model.LineEnding{"\r\n", "\n", "\r\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v003Document(t, v003Text(t, "Wait. . . Then.. .", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v003Run(t, result, "current", [][3]int{{3, 5, 10}, {3, 15, 19}})
		})
	}
}

func TestV003UnavailableLines(t *testing.T) {
	// Допустимое отсутствие доступных строк с известной версией, а не выдуманный DSL.
	v003Run(t, v003UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV003UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v003UnavailableDocument(t, version)
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v003Snapshot(t, result)
			checks, err := validator.Prepare(result, v003Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v003Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV003CoexistsWithOtherChecks(t *testing.T) {
	result := v003Document(t, v003Text(t, ".  .\t.", 2, 1))
	v003Run(t, result, "current", [][3]int{{3, 1, 7}})
	for _, tc := range []struct {
		code    diagnostics.Code
		message string
		start   int
		end     int
	}{
		{"V001", "Несколько пробелов подряд.", 2, 4},
		{"V002", "В строке есть табуляция.", 5, 6},
	} {
		matches := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != "current" || entry.DiagnosticCode != tc.code {
				continue
			}
			matches++
			want := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
			if entry.Location == nil || *entry.Location != want || entry.Message != tc.message ||
				entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
		if matches != 1 {
			t.Errorf("current %s count = %d, want 1", tc.code, matches)
		}
	}
}

func TestV003RepeatedRunsPreserveHistory(t *testing.T) {
	result := v003Document(t, v003Text(t, "Wait. . . Then.. .", 2, 1))
	v003Run(t, result, "first", [][3]int{{3, 5, 10}, {3, 15, 19}})
	v003Run(t, result, "second", [][3]int{{3, 5, 10}, {3, 15, 19}})
	v003Run(t, v003Document(t, v003Text(t, "Wait... Then…", 2, 1)), "first", nil)
}

// v003Open составляет объявление блока без параметров; входные координаты
// вычисляются только для фиксированных токенов, не для ожиданий диагностики.
func v003Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v003Line(t, "@"+tag+" {", "block-start", parent, depth,
		v003Element(t, "tag", "@"+tag, tag, 1, end), v003Element(t, "block-open", "{", "{", end+1, end+2))
}

// v003Close составляет закрывающую строку с явно заданным родителем и глубиной.
func v003Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v003Line(t, "}", "block-end", parent, depth, v003Element(t, "block-close", "}", "}", 1, 2))
}

// v003NumberLines задаёт физические номера после добавления метаданных;
// структуру элементов и родителей вызывающий тест задаёт явно.
func v003NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count := len(result.Lines)
	result.Document.LineCount = &count
}

// v003UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v003UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v003Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v003Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v003Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v003Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v003Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v003Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v003Element(t, "version", "1.2", "1.2", 14, 17)),
		v003Line(t, "# Unit 1", "heading", 0, 0,
			v003Element(t, "heading-level", "#", "1", 1, 2),
			v003Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}},
		},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       lines,
		Diagnostics: []model.Diagnostic{},
	}
}

// v003Line собирает строку с явно заданной структурой, без классификации Raw.
func v003Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v003Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v003Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v003Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v003Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v003Line(t, text, "content", parent, depth,
		v003Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v003Run проверяет штатный конвейер и явные тройки: строка, начало, конец.
// Правая граница исключается. Проверяются только V003 текущего запуска;
// прежние результаты, строки, флаги и ссылки должны сохраниться без изменений.
func v003Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v003Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v003Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v003Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V003" && entry.Source != processingID {
			t.Fatalf("new V003 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V003" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V003 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[0], Column: expected[2]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V003" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "Точки многоточия разделены пробельными символами." || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	v003OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v003Snapshot(t, &previous) != v003Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v003Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v003Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v003OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v003OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V003" {
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

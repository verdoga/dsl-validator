package checkers

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v123TestExpected задаёт исходную строку и Unicode-диапазон с исключённым концом.
type v123TestExpected struct{ line, start, end int }

func TestV123Registration(t *testing.T) {
	definitions, err := v123TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V123" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "В однострочной инструкции записан маркер @hint." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V123 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V123 definitions = %d, want 1", count)
	}
}

func TestV123ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v123TestExpected
	}{
		{"reference", "@instr {\nRead the text.\n@hint Look at the title.\n}", []model.Line{
			v123TestOpen(t, "instr", 2, 1),
			v123TestText(t, "Read the text.", "Read the text.", 3, 2),
			v123TestLine(t, "@hint Look at the title.", "tag", 3, 2,
				v123TestElement(t, "tag", "@hint", "hint", 1, 6),
				v123TestElement(t, "content", "Look at the title.", "Look at the title.", 7, 25)),
			v123TestClose(t, 3, 2),
		}, nil},
		{"example", "@instr Read the text. @hint Look at the title.", []model.Line{
			v123TestInstr(t, "Read the text. @hint Look at the title.", "Read the text. @hint Look at the title.", 2, 1),
		}, []v123TestExpected{{3, 23, 28}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v123TestDocument(t, tc.body...)
			var body []string
			for _, line := range result.Lines[2:] {
				body = append(body, line.Raw)
			}
			if strings.Join(body, "\n") != tc.fragment {
				t.Fatal("reference or example changed")
			}
			v123TestRun(t, result, "current", tc.want)
			if tc.name == "example" {
				// По уточнению пользователя V034 остаётся самостоятельным предупреждением.
				count := 0
				for _, entry := range result.Diagnostics {
					if entry.Source != "current" || entry.DiagnosticCode != "V034" {
						continue
					}
					count++
					want := model.Location{Start: model.Position{Line: 3, Column: 23}, End: model.Position{Line: 3, Column: 28}}
					if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "warning" ||
						entry.DiagnosticScope != "line" || entry.Fatal || len(entry.RelatedLocations) != 0 ||
						entry.Message != "Запись с @ внутри строки остаётся обычным текстом.\nконкретизация: Для отдельного тега нужна новая строка" {
						t.Fatalf("unexpected accompanying V034: %+v", entry)
					}
				}
				if count != 1 {
					t.Fatalf("accompanying V034 count = %d, want 1", count)
				}
			}
		})
	}
}

func TestV123MarkerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		want          []v123TestExpected
	}{
		{"content beginning", "@hint Help.", []v123TestExpected{{3, 8, 13}}},
		{"line end", "Read. @hint", []v123TestExpected{{3, 14, 19}}},
		{"marker only", "@hint", []v123TestExpected{{3, 8, 13}}},
		{"ordinary text", "Read the text.", nil},
		{"tabs", "Read.\t@hint\tHelp.", []v123TestExpected{{3, 14, 19}}},
		{"multiple", "@hint @HINT Help.", []v123TestExpected{{3, 8, 13}, {3, 14, 19}}},
		{"other markers ignored", "@note @hint Help.", []v123TestExpected{{3, 14, 19}}},
		{"short at", "@", nil}, {"short h", "@h", nil}, {"short hi", "@hi", nil}, {"short hin", "@hin", nil},
		{"longer name", "@hintText", nil}, {"digit suffix", "@hint2", nil}, {"hyphen suffix", "@hint-extra", nil},
		{"letter before", "x@hint", nil}, {"punctuation before", "(@hint", nil}, {"double at", "@@hint", nil},
		{"period after", "@hint.", nil}, {"parenthesis after", "@hint)", nil},
		{"NBSP before", "x\u00a0@hint", nil}, {"NBSP after", "@hint\u00a0x", nil},
		{"vertical tab before", "x\v@hint", nil}, {"vertical tab after", "@hint\vx", nil},
		{"line separator before", "x\u2028@hint", nil}, {"line separator after", "@hint\u2028x", nil},
		{"Unicode lookalike", "@hіnt", nil},
		{"escaped marker", `\@hint`, nil},
		{"escaped marker between matches", `@hint \@hint @hint`, []v123TestExpected{{3, 8, 13}, {3, 21, 26}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v123TestRun(t, v123TestDocument(t, v123TestInstr(t, tc.content, tc.content, 2, 1)), "current", tc.want)
		})
	}
}

func TestV123RawAndCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v123TestExpected
	}{
		{"case", v123TestLine(t, "@InStR Read. @HiNt Help.", "tag", 2, 1,
			v123TestElement(t, "tag", "@InStR", "instr", 1, 7),
			v123TestElement(t, "content", "Read. @HiNt Help.", "Read. @HiNt Help.", 8, 25)), []v123TestExpected{{3, 14, 19}}},
		{"Unicode and indentation", v123TestLine(t, "  @InStR Я😀 @HiNt", "tag", 2, 1,
			v123TestElement(t, "tag", "@InStR", "instr", 3, 9),
			v123TestElement(t, "content", "Я😀 @HiNt", "Я😀 @HiNt", 10, 18)), []v123TestExpected{{3, 13, 18}}},
		{"trailing spaces", v123TestLine(t, "@instr @hint  ", "tag", 2, 1,
			v123TestElement(t, "tag", "@instr", "instr", 1, 7),
			v123TestElement(t, "content", "@hint", "@hint", 8, 13)), []v123TestExpected{{3, 8, 13}}},
		{"tab indentation and tail", v123TestLine(t, "\t@instr @hint\t", "tag", 2, 1,
			v123TestElement(t, "tag", "@instr", "instr", 2, 8),
			v123TestElement(t, "content", "@hint", "@hint", 9, 14)), []v123TestExpected{{3, 9, 14}}},
		{"paired backslashes", v123TestInstr(t, `\\@hint`, `\@hint`, 2, 1), nil},
		{"escaped prefix", v123TestInstr(t, `\{x\} @hint`, `{x} @hint`, 2, 1), []v123TestExpected{{3, 14, 19}}},
		{"escaped right brace is not a delimiter", v123TestInstr(t, `@hint\{Help\}`, `@hint{Help}`, 2, 1), nil},
		{"escaped closing brace is not a delimiter", v123TestInstr(t, `@hint\}Help`, `@hint}Help`, 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v123TestRun(t, v123TestDocument(t, tc.line), "current", tc.want) })
	}
}

func TestV123BraceBoundaries(t *testing.T) {
	for _, content := range []string{"@hint{Help}", "@hint}Help"} {
		t.Run(content, func(t *testing.T) {
			result := v123TestDocument(t, v123TestInstr(t, content, content, 2, 1))
			// Таблица V123 прямо допускает обе скобки как правую границу маркера.
			// DSL 1.21 требует экранирования литеральных скобок. Сохраняем исходный
			// content и отдельную ошибку этой записи, не изображая корректный DSL
			// или настоящий блок. fixture-braces — тестовый код, не код dsl-parser.
			result.Document.HasErrors, result.Lines[2].HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: "brace-error", Source: "parser", DiagnosticCode: "fixture-braces", SeverityLevel: "error", DiagnosticScope: "line",
				Message: "Литеральные фигурные скобки требуют экранирования.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: 3, Column: 13}, End: model.Position{Line: 3, Column: 14}},
			})
			v123TestRun(t, result, "current", []v123TestExpected{{3, 8, 13}})
		})
	}
}

func TestV123Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v123TestExpected
	}{
		{"next hint", []model.Line{
			v123TestInstr(t, "Read.", "Read.", 2, 1),
			v123TestLine(t, "@hint Help.", "tag", 2, 1, v123TestElement(t, "tag", "@hint", "hint", 1, 6), v123TestElement(t, "content", "Help.", "Help.", 7, 12)),
		}, nil},
		{"indented next hint", []model.Line{
			v123TestInstr(t, "Read.", "Read.", 2, 1),
			v123TestLine(t, "\t@hint Help.", "tag", 2, 1, v123TestElement(t, "tag", "@hint", "hint", 2, 7), v123TestElement(t, "content", "Help.", "Help.", 8, 13)),
		}, nil},
		{"no joining lines", []model.Line{v123TestInstr(t, "Read. @hi", "Read. @hi", 2, 1), v123TestText(t, "nt", "nt", 2, 1)}, nil},
		{"other tag", []model.Line{
			v123TestLine(t, "@note Read. @hint Help.", "tag", 2, 1,
				v123TestElement(t, "tag", "@note", "note", 1, 6), v123TestElement(t, "content", "Read. @hint Help.", "Read. @hint Help.", 7, 24)),
		}, nil},
		{"literal instr", []model.Line{v123TestText(t, `\@instr Read. @hint Help.`, "@instr Read. @hint Help.", 2, 1)}, nil},
		{"block content", []model.Line{v123TestOpen(t, "instr", 2, 1), v123TestText(t, "Read. @hint Help.", "Read. @hint Help.", 3, 2), v123TestClose(t, 3, 2)}, nil},
		{"nested fragment", []model.Line{
			v123TestLine(t, "@fragment f {", "block-start", 2, 1,
				v123TestElement(t, "tag", "@fragment", "fragment", 1, 10), v123TestElement(t, "identifier", "f", "f", 11, 12), v123TestElement(t, "block-open", "{", "{", 13, 14)),
			v123TestInstr(t, "@hint Help.", "@hint Help.", 3, 2), v123TestClose(t, 3, 2),
			v123TestLine(t, "@include f", "tag", 2, 1, v123TestElement(t, "tag", "@include", "include", 1, 9), v123TestElement(t, "identifier", "f", "f", 10, 11)),
		}, []v123TestExpected{{4, 8, 13}}},
		{"first and last instructions", []model.Line{
			v123TestInstr(t, "@hint Help.", "@hint Help.", 2, 1), v123TestText(t, "Text.", "Text.", 2, 1), v123TestInstr(t, "@hint Help.", "@hint Help.", 2, 1),
		}, []v123TestExpected{{3, 8, 13}, {5, 8, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v123TestRun(t, v123TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal content of "+tag, func(t *testing.T) {
			v123TestRun(t, v123TestDocument(t, v123TestOpen(t, tag, 2, 1),
				v123TestText(t, "@instr Read. @hint Help.", "@instr Read. @hint Help.", 3, 2), v123TestClose(t, 3, 2),
				v123TestText(t, "Read.", "Read.", 2, 1)), "current", nil)
		})
	}
}

func TestV123PlaceholderBoundaries(t *testing.T) {
	// DSL 4.3.3: обычный плейсхолдер допустим в учебном тексте инструкции.
	// CONTENT начинается перед ним. Следующий content-элемент не начинает
	// содержимое инструкции заново; конец элемента также не равен концу строки.
	for _, tc := range []struct {
		name string
		line model.Line
		want []v123TestExpected
	}{
		{"placeholder before without separator", v123TestLine(t, "@instr _____@hint Help.", "tag", 2, 1,
			v123TestElement(t, "tag", "@instr", "instr", 1, 7), v123TestElement(t, "placeholder", "_____", "", 8, 13),
			v123TestElement(t, "content", "@hint Help.", "@hint Help.", 13, 24)), nil},
		{"placeholder before with separator", v123TestLine(t, "@instr _____ @hint Help.", "tag", 2, 1,
			v123TestElement(t, "tag", "@instr", "instr", 1, 7), v123TestElement(t, "placeholder", "_____", "", 8, 13),
			v123TestElement(t, "content", " @hint Help.", " @hint Help.", 13, 25)), []v123TestExpected{{3, 14, 19}}},
		{"placeholder after without separator", v123TestLine(t, "@instr @hint_____", "tag", 2, 1,
			v123TestElement(t, "tag", "@instr", "instr", 1, 7), v123TestElement(t, "content", "@hint", "@hint", 8, 13),
			v123TestElement(t, "placeholder", "_____", "", 13, 18)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v123TestRun(t, v123TestDocument(t, tc.line), "current", tc.want) })
	}
}

func TestV123LineEndingsEmptyInputAndVersions(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("eol/"+string(eol), func(t *testing.T) {
			result := v123TestDocument(t, v123TestInstr(t, "Read the text. @hint Look at the title.", "Read the text. @hint Look at the title.", 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v123TestRun(t, result, "current", []v123TestExpected{{3, 23, 28}})
		})
	}
	t.Run("empty lines", func(t *testing.T) {
		result := v123TestDocument(t)
		result.Lines = []model.Line{}
		result.Document.LineCount, result.Document.Metadata.Title = nil, nil
		v123TestRun(t, result, "current", nil)
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported/"+version, func(t *testing.T) {
			result := v123TestDocument(t, v123TestInstr(t, "@hint", "@hint", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v123TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v123TestRegistry(t))
			if err == nil || checks != nil || v123TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version accepted or changed input: checks=%v, err=%v", checks, err)
			}
		})
	}
}

func TestV123RepeatedRuns(t *testing.T) {
	result := v123TestDocument(t, v123TestInstr(t, "Read the text. @hint Look at the title.", "Read the text. @hint Look at the title.", 2, 1))
	v123TestRun(t, result, "first", []v123TestExpected{{3, 23, 28}})
	v123TestRun(t, result, "second", []v123TestExpected{{3, 23, 28}})
	v123TestRun(t, v123TestDocument(t, v123TestInstr(t, "Read.", "Read.", 2, 1)), "first", nil)
}

// v123TestDocument добавляет только обязательные версию и заголовок (DSL 1.1–1.3,
// 1.30, 2.1.3). Структура тела задана вызывающим тестом, исходный DSL не разбирается.
func v123TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v123TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v123TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v123TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v123TestLine(t, "# Lesson", "heading", 0, 0, v123TestElement(t, "heading-level", "#", "1", 1, 2), v123TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format:   model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
}

// v123TestInstr составляет однострочный instr из явно заданных Raw и Value.
// Подсчёт длины используется только для входных элементов, не для ожиданий.
func v123TestInstr(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v123TestLine(t, "@instr "+raw, "tag", parent, depth,
		v123TestElement(t, "tag", "@instr", "instr", 1, 7), v123TestElement(t, "content", raw, value, 8, 8+utf8.RuneCountInString(raw)))
}

func v123TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v123TestLine(t, raw, "content", parent, depth, v123TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

func v123TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v123TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v123TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v123TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

func v123TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v123TestLine(t, "}", "block-end", parent, depth, v123TestElement(t, "block-close", "}", "}", 1, 2))
}

func v123TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v123TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
	// Обычный плейсхолдер без ANSWER имеет null Value (модель и её тесты).
	if kind == "placeholder" && raw == "_____" {
		element.Value = nil
	}
	return element
}

// v123TestRegistry создаёт штатный реестр, не заменяя и не отбирая регистрации.
func v123TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v123TestRun запускает полный конвейер и проверяет текущие V123 независимо
// от остальных кодов. Исходник и предыдущая история должны сохраниться.
func v123TestRun(t *testing.T, result *model.Result, source string, want []v123TestExpected) {
	t.Helper()
	before := v123TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v123TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v123TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V123" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V123 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		message := "В однострочной инструкции записан маркер @hint.\nконкретизация: Для подсказки, вложенной в инструкцию, используйте блочный @instr и отдельное объявление @hint внутри него"
		if entry.Location == nil || *entry.Location != location || entry.Message != message ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v and exact V123 metadata", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V123 error did not mark document and line")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v123TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// CheckModel проверяет флаги, ID и ссылки всего каталога; для сравнения
	// источника восстанавливаем в независимой копии только производные поля.
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
						t.Fatal("line-scoped V123 linked to an element")
					}
				}
			}
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v123TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v123TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

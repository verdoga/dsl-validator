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

// v152Expected хранит явные исходные координаты [start,end), с 1.
type v152Expected struct{ line, start, end int }

func TestV152Registration(t *testing.T) {
	definitions, err := v152Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V152" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "В пропуске указан пустой ответ." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V152 count = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			result := v152Document(t)
			result.Document.DSLVersion = &version
			before := v152Snapshot(t, result)
			if checks, err := validator.Prepare(result, v152Registry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v152Snapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV152ReferenceAndExample(t *testing.T) {
	// Перед неизменёнными фрагментами добавлены только версия и заголовок документа.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v152Expected
	}{
		{"reference", "@multifill {\nShe _____{lives} here.\n}", []model.Line{
			v152Line(t, "She _____{lives} here.", "content", 3,
				v152Element(t, "content", "She ", "She ", 1), v152Element(t, "placeholder", "_____{lives}", "lives", 5),
				v152Element(t, "content", " here.", " here.", 17))}, nil},
		{"example", "@multifill {\nShe _____{} here.\nHe _____{   } there.\n}", []model.Line{
			v152Invalid(t, "She _____{} here.", 3), v152Invalid(t, "He _____{   } there.", 3)},
			[]v152Expected{{4, 5, 12}, {5, 4, 14}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v152Multifill(t, tc.body...)
			var source []string
			for _, row := range result.Lines[2:] {
				source = append(source, row.Raw)
			}
			if strings.Join(source, "\n") != tc.fragment {
				t.Fatal("control fragment changed")
			}
			v152Run(t, result, "current", tc.want)
		})
	}
}

func TestV152EmptyAndExcludedGroups(t *testing.T) {
	// Для повреждённых записей восстановление invalid/unparsed задано явно.
	for _, tc := range []struct {
		name, raw string
		want      []v152Expected
	}{
		{"empty", "_____{}", []v152Expected{{4, 1, 8}}},
		{"spaces", "_____{   }", []v152Expected{{4, 1, 11}}},
		{"tab and NBSP", "_____{\t\u00a0}", []v152Expected{{4, 1, 10}}},
		{"second variant empty", "_____{A}{}", []v152Expected{{4, 1, 11}}},
		{"two empty variants", "_____{}{   }", []v152Expected{{4, 1, 8}, {4, 1, 13}}},
		{"two equal empty variants", "_____{}{}", []v152Expected{{4, 1, 8}, {4, 1, 10}}},
		{"nonempty variants do not suppress empty ones", "_____{A}{}{B}{ }", []v152Expected{{4, 1, 11}, {4, 1, 17}}},
		{"only Unicode whitespace variant is empty", "Я😀 _____{!}{\u00a0\u2003\u202f}{ «—» }", []v152Expected{{4, 4, 17}}},
		{"escaped braces between empty variants", `Я😀 _____{}{\{\}}{}`, []v152Expected{{4, 4, 11}, {4, 4, 19}}},
		{"escaped backslash between empty variants", `Я😀 _____{}{\\}{}`, []v152Expected{{4, 4, 11}, {4, 4, 17}}},
		{"escaped backslash and braces between empty variants", `Я😀 _____{}{\\\{\}}{}`, []v152Expected{{4, 4, 11}, {4, 4, 21}}},
		{"two placeholders", "_____{} _____{ }", []v152Expected{{4, 1, 8}, {4, 9, 17}}},
		{"adjacent placeholders", "_____{}_____{}", []v152Expected{{4, 1, 8}, {4, 8, 15}}},
		{"space breaks variants", "_____{A} {}", nil},
		{"tab breaks variants", "_____{A}\t{}", nil},
		{"text breaks variants", "_____{A}x{}", nil},
		{"four underscores", "____{}", nil},
		{"six underscores", "______{}", nil},
		{"ten underscores", "__________{}", nil},
		{"space before opening", "_____ {}", nil},
		{"tab before opening", "_____\t{}", nil},
		{"NBSP before opening", "_____\u00a0{}", nil},
		{"escaped underscores", `\_____{}`, nil},
		{"escaped long run", `\______{}`, nil},
		{"escaped opening", `_____\{}`, nil},
		{"escaped literal before active placeholder", `Я😀 \_____\{\} _____{}`, []v152Expected{{4, 15, 22}}},
		{"even backslashes", `\\_____{}`, []v152Expected{{4, 3, 10}}},
		{"odd backslashes", `\\\_____{}`, nil},
		{"four backslashes", `\\\\_____{}`, []v152Expected{{4, 5, 12}}},
		{"unknown escape before placeholder", `\q_____{}`, []v152Expected{{4, 3, 10}}},
		{"unclosed", "_____{   ", nil},
		{"opening at end", "_____{", nil},
		{"trailing slash", `_____{\`, nil},
		{"escaped closing only", `_____ {\}`, nil},
		{"active escaped closing only", `_____{\}`, nil},
		{"internal opening", "_____{ { }", nil},
		{"internal apparent placeholder", "_____{X _____{} }", nil},
		{"bad first group and empty second", "_____{A{B}{}", []v152Expected{{4, 1, 13}}},
		{"empty first group and bad second", "_____{}{A{B}", []v152Expected{{4, 1, 8}}},
		{"empty first group and unclosed second", "_____{}{A", []v152Expected{{4, 1, 8}}},
		{"escaped opening breaks alternatives", `_____{}\{}`, []v152Expected{{4, 1, 8}}},
		{"Unicode prefix", "Я😀 _____{}", []v152Expected{{4, 4, 11}}},
		{"combining prefix", "e\u0301_____{}", []v152Expected{{4, 3, 10}}},
		{"indentation and trailing whitespace", " \t_____{} \t", []v152Expected{{4, 3, 10}}},
		{"non-ASCII brace", "_____｛｝", nil},
		{"non-ASCII closing", "_____{｝", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v152Run(t, v152Multifill(t, v152Invalid(t, tc.raw, 3)), "current", tc.want)
		})
	}
}

func TestV152UnicodeWhiteSpace(t *testing.T) {
	// Полный явный набор White_Space без CR/LF: они не могут входить в Raw.
	for _, char := range []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a',
		'\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(fmt.Sprintf("U+%04X", char), func(t *testing.T) {
			v152Run(t, v152Multifill(t, v152Invalid(t, "_____{"+string(char)+"}", 3)), "current", []v152Expected{{4, 1, 9}})
		})
	}
}

func TestV152NonemptyAndLiteralAnswers(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"ordinary text", "Read.", "Read."},
		{"ordinary placeholder", "_____", "_____"},
		{"letters and case", "_____{ Я É Σ }", "_____{ Я É Σ }"},
		{"two alternatives", "_____{A}{B}", "_____{A}{B}"},
		{"slash", "_____{/}", "_____{/}"},
		{"semicolon", "_____{;}", "_____{;}"},
		{"comma", "_____{,}", "_____{,}"},
		{"punctuation only", "_____{...}", "_____{...}"},
		{"exclamation mark", "_____{!}", "_____{!}"},
		{"not punctuation-separated alternatives", "_____{ / ; , }", "_____{ / ; , }"},
		{"Unicode punctuation", "_____{ «—» }", "_____{ «—» }"},
		{"digits", "_____{١}", "_____{١}"},
		{"diacritic", "_____{\u0301}", "_____{\u0301}"},
		{"zero width space", "_____{\u200b}", "_____{\u200b}"},
		{"BOM inside answer", "_____{\ufeff}", "_____{\ufeff}"},
		{"non-White-Space separator", "_____{\u001c}", "_____{\u001c}"},
		{"escaped slash", `_____{\\}`, `_____{\}`},
		{"escaped opening", `_____{\{}`, "_____{{}"},
		{"escaped closing", `_____{\}}`, "_____{}" + "}"},
		{"unknown escape", `_____{\q}`, `_____{\q}`},
		{"slash before space retained", `_____{\ }`, `_____{\ }`},
		{"fully escaped literal", `\_____\{\}`, "_____{}"},
		{"quotes are content", `_____{""}`, `_____{""}`},
		{"quotes do not hide closing", `_____{"}"`, `_____{"}"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v152Run(t, v152Multifill(t, v152Line(t, tc.raw, "content", 3, v152Element(t, "content", tc.raw, tc.value, 1))), "current", nil)
		})
	}
}

func TestV152SourceRepresentation(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"placeholder", v152Line(t, "_____{}", "content", 3, v152Element(t, "placeholder", "_____{}", "", 1))},
		{"unparsed", v152Line(t, "_____{}", "content", 3, v152Element(t, "unparsed", "_____{}", "", 1))},
		{"separate elements", v152Line(t, "_____{}", "content", 3, v152Element(t, "placeholder", "_____", "_____", 1), v152Element(t, "unparsed", "{}", "", 6))},
		{"content without placeholder", v152Line(t, "_____{}", "content", 3, v152Element(t, "content", "_____{}", "_____{}", 1))},
	} {
		t.Run(tc.name, func(t *testing.T) { v152Run(t, v152Multifill(t, tc.row), "current", []v152Expected{{4, 1, 8}}) })
	}
}

func TestV152Contexts(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		result := v152Document(t, v152Version(t), v152Heading(t),
			v152Line(t, "@task t", "tag", 2, v152Element(t, "tag", "@task", "task", 1), v152Element(t, "identifier", "t", "t", 7)),
			v152Open(t, "MULTIFILL", "multifill", 3), v152Invalid(t, "_____{}", 4), v152Close(t, 4),
			v152Line(t, "@endtask", "tag", 3, v152Element(t, "tag", "@endtask", "endtask", 1)))
		v152Run(t, result, "current", []v152Expected{{5, 1, 8}})
	})
	t.Run("variant", func(t *testing.T) {
		result := v152Document(t, v152Version(t), v152Heading(t), v152Open(t, "variants", "variants", 2),
			v152Line(t, "@variant A", "tag", 3, v152Element(t, "tag", "@variant", "variant", 1), v152Element(t, "name", "A", "A", 10)),
			v152Open(t, "multifill", "multifill", 4), v152Invalid(t, "_____{}", 5), v152Close(t, 5),
			v152Line(t, "@variant B", "tag", 3, v152Element(t, "tag", "@variant", "variant", 1), v152Element(t, "name", "B", "B", 10)),
			v152Line(t, "Read.", "content", 8, v152Element(t, "content", "Read.", "Read.", 1)), v152Close(t, 3))
		v152Run(t, result, "current", []v152Expected{{6, 1, 8}})
	})
	t.Run("outside multifill", func(t *testing.T) {
		v152Run(t, v152Document(t, v152Version(t), v152Heading(t), v152Invalid(t, "_____{}", 2)), "current", nil)
	})
	for _, tag := range []string{"editor", "note"} {
		t.Run(tag, func(t *testing.T) {
			v152Run(t, v152Document(t, v152Version(t), v152Heading(t), v152Open(t, tag, tag, 2), v152Invalid(t, "_____{}", 3), v152Close(t, 3)), "current", nil)
		})
	}
	t.Run("literal multifill in editor", func(t *testing.T) {
		v152Run(t, v152Document(t, v152Version(t), v152Heading(t), v152Open(t, "editor", "editor", 2),
			v152Line(t, `@multifill \{`, "content", 3, v152Element(t, "content", `@multifill \{`, "@multifill {", 1)),
			v152Line(t, `\_____\{\}`, "content", 3, v152Element(t, "content", `\_____\{\}`, "_____{}", 1)), v152Close(t, 3)), "current", nil)
	})
	t.Run("one-line instruction", func(t *testing.T) {
		v152Run(t, v152Document(t, v152Version(t), v152Heading(t), v152Line(t, "@multifill _____{}", "tag", 2,
			v152Element(t, "tag", "@multifill", "multifill", 1), v152Element(t, "unparsed", "_____{}", "", 12))), "current", nil)
	})
	t.Run("block instruction excluded", func(t *testing.T) {
		v152Run(t, v152Document(t, v152Version(t), v152Heading(t), v152Line(t, "@multifill _____{} {", "block-start", 2,
			v152Element(t, "tag", "@multifill", "multifill", 1), v152Element(t, "unparsed", "_____{}", "", 12), v152Element(t, "block-open", "{", "{", 20)),
			v152Line(t, "Read.", "content", 3, v152Element(t, "content", "Read.", "Read.", 1)), v152Close(t, 3)), "current", nil)
	})
	t.Run("tag-like content still checked", func(t *testing.T) {
		v152Run(t, v152Multifill(t, v152Invalid(t, "@note _____{}", 3)), "current", []v152Expected{{4, 7, 14}})
	})
}

func TestV152BoundariesAndHistory(t *testing.T) {
	t.Run("empty input", func(t *testing.T) { v152Run(t, v152Document(t), "current", nil) })
	t.Run("empty block", func(t *testing.T) { v152Run(t, v152Multifill(t), "current", nil) })
	t.Run("closing on next line is not answer closing", func(t *testing.T) {
		v152Run(t, v152Multifill(t, v152Invalid(t, "_____{   ", 3)), "current", nil)
	})
	t.Run("new line starts fresh", func(t *testing.T) {
		v152Run(t, v152Multifill(t, v152Invalid(t, "_____{", 3), v152Invalid(t, "} _____{}", 3)), "current", []v152Expected{{5, 3, 10}})
	})
	t.Run("first and last content lines separated by blank", func(t *testing.T) {
		v152Run(t, v152Multifill(t, v152Invalid(t, "_____{}", 3), v152Line(t, "", "blank", 3), v152Invalid(t, "_____{}", 3)), "current", []v152Expected{{4, 1, 8}, {6, 1, 8}})
	})
	t.Run("separate blocks", func(t *testing.T) {
		v152Run(t, v152Document(t, v152Version(t), v152Heading(t), v152Open(t, "multifill", "multifill", 2),
			v152Invalid(t, "_____{}", 3), v152Close(t, 3), v152Open(t, "multifill", "multifill", 2), v152Invalid(t, "_____{}", 6), v152Close(t, 6)), "current", []v152Expected{{4, 1, 8}, {7, 1, 8}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", eol), func(t *testing.T) {
			result := v152Multifill(t, v152Invalid(t, "She _____{} here.", 3), v152Invalid(t, "He _____{   } there.", 3))
			for i := range result.Lines {
				if eol != "" {
					result.Lines[i].LineEnding = eol
				}
			}
			result.Lines[len(result.Lines)-1].LineEnding = eol
			want := []v152Expected{{4, 5, 12}, {5, 4, 14}}
			v152Run(t, result, "first", want)
			v152Run(t, v152Multifill(t, v152Line(t, "Read.", "content", 3, v152Element(t, "content", "Read.", "Read.", 1))), "other", nil)
			v152Run(t, result, "second", want)
		})
	}
}

// v152Invalid задаёт восстановленную повреждённую строку с unparsed.
func v152Invalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v152Line(t, raw, "invalid", parent, v152Element(t, "unparsed", raw, "", 1))
}

// v152Open создаёт открытие блока с исходным и каноническим именем.
func v152Open(t *testing.T, rawName, name string, parent int) model.Line {
	t.Helper()
	return v152Line(t, "@"+rawName+" {", "block-start", parent,
		v152Element(t, "tag", "@"+rawName, name, 1), v152Element(t, "block-open", "{", "{", len(rawName)+3))
}

// v152Close создаёт структурное закрытие указанного блока.
func v152Close(t *testing.T, parent int) model.Line {
	t.Helper()
	return v152Line(t, "}", "block-end", parent, v152Element(t, "block-close", "}", "}", 1))
}

// v152Multifill добавляет полноценное окружение; содержимое начинается со строки 4.
func v152Multifill(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{v152Version(t), v152Heading(t), v152Open(t, "multifill", "multifill", 2)}, body...)
	return v152Document(t, append(rows, v152Close(t, 3))...)
}

// v152Element создаёт элемент модели; диапазоны ожиданий задаются отдельно.
func v152Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v152Line задаёт тип и родителя без повторного разбора исходного текста.
func v152Line(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v152Version задаёт технический тег версии для контекста примеров.
func v152Version(t *testing.T) model.Line {
	t.Helper()
	return v152Line(t, "@dsl-version 1.2", "tag", 0,
		v152Element(t, "tag", "@dsl-version", "dsl-version", 1), v152Element(t, "version", "1.2", "1.2", 14))
}

// v152Heading добавляет обязательный заголовок документа перед фрагментом.
func v152Heading(t *testing.T) model.Line {
	t.Helper()
	return v152Line(t, "# Lesson", "heading", 0,
		v152Element(t, "heading-level", "#", "1", 1), v152Element(t, "title", "Lesson", "Lesson", 3))
}

// v152Document оформляет заданную модель и обязательные исходные ссылки unparsed.
// Причины повреждения не восстанавливаются: классификация передана тестом явно.
func v152Document(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	version, count := "1.2", len(rows)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines: append([]model.Line{}, rows...), Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			if *line.ParentLine < 1 || *line.ParentLine >= line.Number {
				t.Fatal("invalid fixture parent")
			}
			line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
		}
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", line.Number, j)
			element.ErrorIDs = []string{id}
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Неразобранный фрагмент.",
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}}, RelatedLocations: []model.Location{}})
			line.HasErrors, result.Document.HasErrors = true, true
		}
	}
	return result
}

// v152Registry создаёт реестр из полного штатного каталога.
func v152Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v152Snapshot сохраняет модель для проверки неизменности истории и исходных данных.
func v152Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v152Run запускает штатный конвейер и проверяет только новые срабатывания V152.
func v152Run(t *testing.T, result *model.Result, processing string, want []v152Expected) {
	t.Helper()
	before := v152Snapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v152Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v152Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processing, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processing); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V152" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V152 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processing ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "В пропуске указан пустой ответ." {
			t.Fatalf("V152 occurrence = %+v, want %+v", entry, expected)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line scope created element link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("output integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v152Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Только производные флаги, новые ссылки и записи исключаются из сравнения.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("old element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:oldCount]
	if v152Snapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

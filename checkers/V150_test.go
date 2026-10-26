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

// v150Expected хранит явные исходные координаты [start,end), с 1.
type v150Expected struct{ line, start, end int }

func TestV150Registration(t *testing.T) {
	definitions, err := v150Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V150" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Ответ в фигурных скобках не закрыт в строке пропуска." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V150 count = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			result := v150Document(t)
			result.Document.DSLVersion = &version
			before := v150Snapshot(t, result)
			if checks, err := validator.Prepare(result, v150Registry(t)); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v150Snapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV150ReferenceAndExample(t *testing.T) {
	// Только версия и заголовок добавляются перед дословными фрагментами таблицы.
	for _, tc := range []struct {
		name, fragment string
		row            model.Line
		want           []v150Expected
	}{
		{"reference", "@multifill {\nShe _____{lives} here.\n}",
			v150Line(t, "She _____{lives} here.", "content", 3,
				v150Element(t, "content", "She ", "She ", 1),
				v150Element(t, "placeholder", "_____{lives}", "lives", 5),
				v150Element(t, "content", " here.", " here.", 17)), nil},
		{"example", "@multifill {\nShe _____{lives\n}",
			v150Invalid(t, "She _____{lives", 3), []v150Expected{{4, 10, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v150Multifill(t, tc.row)
			if result.Lines[2].Raw+"\n"+result.Lines[3].Raw+"\n"+result.Lines[4].Raw != tc.fragment {
				t.Fatal("control fragment changed")
			}
			v150Run(t, result, "current", tc.want)
		})
	}
}

func TestV150UnclosedAnswers(t *testing.T) {
	// Все строки ниже повреждены; восстановление invalid/unparsed задано явно.
	// Ожидаемые позиции не вычисляются по исходной строке.
	for _, tc := range []struct {
		name, raw string
		want      []v150Expected
	}{
		{"opening at end of line", "_____{", []v150Expected{{4, 6, 7}}},
		{"answer without closing", "_____{A", []v150Expected{{4, 6, 7}}},
		{"four underscores", "____{A", nil},
		{"six underscores", "______{A", nil},
		{"ten underscores", "__________{A", nil},
		{"space before brace", "_____ {A", nil},
		{"tab before brace", "_____\t{A", nil},
		{"escaped entire run", `\_____{A`, nil},
		{"escaped long run", `\______{A`, nil},
		{"escaped run followed by active run", `\_____ _____{A`, []v150Expected{{4, 13, 14}}},
		{"escaped opening", `_____\{A`, nil},
		{"even backslashes before run", `\\_____{A`, []v150Expected{{4, 8, 9}}},
		{"odd backslashes before run", `\\\_____{A`, nil},
		{"four backslashes before run", `\\\\_____{A`, []v150Expected{{4, 10, 11}}},
		{"ordinary escape does not hide run", `\q_____{A`, []v150Expected{{4, 8, 9}}},
		{"escaped closing", `_____{A\}`, []v150Expected{{4, 6, 7}}},
		{"odd backslashes before closing", `_____{A\\\}`, []v150Expected{{4, 6, 7}}},
		{"trailing backslash", `_____{A\`, []v150Expected{{4, 6, 7}}},
		{"escaped opening inside answer", `_____{A\{B`, []v150Expected{{4, 6, 7}}},
		{"nested opening has no stack", "_____{A{B}", nil},
		{"no closing after nested opening", "_____{A{B", []v150Expected{{4, 6, 7}}},
		{"closed then unclosed", "_____{A} _____{B", []v150Expected{{4, 15, 16}}},
		{"adjacent closed then unclosed", "_____{A}_____{B", []v150Expected{{4, 14, 15}}},
		{"ignore start inside unfinished answer", "_____{A _____{B", []v150Expected{{4, 6, 7}}},
		{"empty but closed answer", "_____{}", nil},
		{"Unicode coordinates", "Я😀 _____{A", []v150Expected{{4, 9, 10}}},
		{"combining character", "e\u0301_____{A", []v150Expected{{4, 8, 9}}},
		{"indentation retained", " \t_____{A \t", []v150Expected{{4, 8, 9}}},
		{"wrong run then valid start", "______{A _____{B", []v150Expected{{4, 15, 16}}},
		{"non-ASCII brace is not opening", "_____｛A", nil},
		{"non-ASCII brace is not closing", "_____{A｝", []v150Expected{{4, 6, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v150Run(t, v150Multifill(t, v150Invalid(t, tc.raw, 3)), "current", tc.want)
		})
	}
}

func TestV150ClosedAndLiteralAnswers(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"ordinary text", "Read this text.", "Read this text."},
		{"ordinary placeholder", "_____", "_____"},
		{"two alternatives", "_____{A}{B}", "_____{A}{B}"},
		{"three alternatives", "_____{A}{B}{C}", "_____{A}{B}{C}"},
		{"two placeholders with alternatives", "_____{A}{B} _____{C}{D}", "_____{A}{B} _____{C}{D}"},
		{"paired slashes before closing", `_____{A\\}`, `_____{A\}`},
		{"escaped braces followed by closing", `_____{A\{B\}C}`, "_____{A{B}C}"},
		{"quote does not hide closing", `_____{"}"`, `_____{"}"`},
		{"unknown escape retains closing", `_____{A\q}`, `_____{A\q}`},
		{"fully escaped literal", `\_____\{A\}`, "_____{A}"},
		{"decoded value must not become active", `\_____\{A`, "_____{A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Элементы placeholder необязательны по условиям V150; здесь модель
			// сохраняет текст единым content, а в эталоне выделяет placeholder.
			row := v150Line(t, tc.raw, "content", 3, v150Element(t, "content", tc.raw, tc.value, 1))
			v150Run(t, v150Multifill(t, row), "current", nil)
		})
	}
}

func TestV150ContentRepresentations(t *testing.T) {
	for _, row := range []model.Line{
		v150Line(t, "She _____{lives", "content", 3,
			v150Element(t, "content", "She ", "She ", 1), v150Element(t, "unparsed", "_____{lives", "", 5)),
		v150Line(t, "She _____{lives", "content", 3,
			v150Element(t, "content", "She ", "She ", 1), v150Element(t, "placeholder", "_____", "_____", 5),
			v150Element(t, "unparsed", "{lives", "", 10)),
	} {
		v150Run(t, v150Multifill(t, row), "current", []v150Expected{{4, 10, 11}})
	}
	// Текст, похожий на тег, остаётся содержимым multifill (DSL 4.4.11).
	v150Run(t, v150Multifill(t, v150Invalid(t, "@note _____{A", 3)), "current", []v150Expected{{4, 12, 13}})
}

func TestV150Contexts(t *testing.T) {
	t.Run("task and stage", func(t *testing.T) {
		result := v150Document(t, v150Version(t), v150Heading(t),
			v150Line(t, "@task t", "tag", 2, v150Element(t, "tag", "@task", "task", 1), v150Element(t, "identifier", "t", "t", 7)),
			v150Line(t, "@step Practice", "tag", 3, v150Element(t, "tag", "@step", "step", 1), v150Element(t, "title", "Practice", "Practice", 7)),
			v150Open(t, "MULTIFILL", "multifill", 4), v150Invalid(t, "She _____{lives", 5), v150Close(t, 5),
			v150Line(t, "@endtask", "tag", 3, v150Element(t, "tag", "@endtask", "endtask", 1)))
		v150Run(t, result, "current", []v150Expected{{6, 10, 11}})
	})
	t.Run("variant", func(t *testing.T) {
		result := v150Document(t, v150Version(t), v150Heading(t), v150Open(t, "variants", "variants", 2),
			v150Line(t, "@variant A", "tag", 3, v150Element(t, "tag", "@variant", "variant", 1), v150Element(t, "name", "A", "A", 10)),
			v150Open(t, "multifill", "multifill", 4), v150Invalid(t, "_____{A", 5), v150Close(t, 5),
			v150Line(t, "@variant B", "tag", 3, v150Element(t, "tag", "@variant", "variant", 1), v150Element(t, "name", "B", "B", 10)),
			v150Line(t, "Read.", "content", 8, v150Element(t, "content", "Read.", "Read.", 1)), v150Close(t, 3))
		v150Run(t, result, "current", []v150Expected{{6, 6, 7}})
	})
	t.Run("outside block", func(t *testing.T) {
		v150Run(t, v150Document(t, v150Version(t), v150Heading(t), v150Invalid(t, "_____{A", 2)), "current", nil)
	})
	for _, tag := range []string{"editor", "note"} {
		t.Run(tag, func(t *testing.T) {
			v150Run(t, v150Document(t, v150Version(t), v150Heading(t), v150Open(t, tag, tag, 2),
				v150Invalid(t, "_____{A", 3), v150Close(t, 3)), "current", nil)
		})
	}
	t.Run("literal multifill declaration in editor", func(t *testing.T) {
		v150Run(t, v150Document(t, v150Version(t), v150Heading(t), v150Open(t, "editor", "editor", 2),
			v150Line(t, `@multifill \{`, "content", 3, v150Element(t, "content", `@multifill \{`, "@multifill {", 1)),
			v150Invalid(t, "_____{A", 3), v150Close(t, 3)), "current", nil)
	})
	t.Run("one-line instruction", func(t *testing.T) {
		v150Run(t, v150Document(t, v150Version(t), v150Heading(t),
			v150Line(t, "@multifill _____{A", "tag", 2, v150Element(t, "tag", "@multifill", "multifill", 1),
				v150Element(t, "unparsed", "_____{A", "", 12))), "current", nil)
	})
	t.Run("block instruction", func(t *testing.T) {
		v150Run(t, v150Document(t, v150Version(t), v150Heading(t),
			v150Line(t, "@multifill _____{A {", "block-start", 2, v150Element(t, "tag", "@multifill", "multifill", 1),
				v150Element(t, "unparsed", "_____{A", "", 12), v150Element(t, "block-open", "{", "{", 20)),
			v150Line(t, "Read.", "content", 3, v150Element(t, "content", "Read.", "Read.", 1)), v150Close(t, 3)), "current", nil)
	})
}

func TestV150LineBoundariesAndHistory(t *testing.T) {
	t.Run("empty input", func(t *testing.T) { v150Run(t, v150Document(t), "current", nil) })
	t.Run("empty block", func(t *testing.T) { v150Run(t, v150Multifill(t), "current", nil) })
	t.Run("each line starts fresh", func(t *testing.T) {
		v150Run(t, v150Multifill(t, v150Invalid(t, `_____{A\`, 3), v150Invalid(t, "_____{B", 3)), "current",
			[]v150Expected{{4, 6, 7}, {5, 6, 7}})
	})
	t.Run("closing on next content line cannot repair answer", func(t *testing.T) {
		v150Run(t, v150Multifill(t, v150Invalid(t, "_____{A", 3), v150Invalid(t, "} _____{B", 3)), "current",
			[]v150Expected{{4, 6, 7}, {5, 8, 9}})
	})
	t.Run("separate blocks", func(t *testing.T) {
		v150Run(t, v150Document(t, v150Version(t), v150Heading(t), v150Open(t, "multifill", "multifill", 2),
			v150Invalid(t, "_____{A", 3), v150Close(t, 3), v150Open(t, "multifill", "multifill", 2),
			v150Invalid(t, "_____{B", 6), v150Close(t, 6)), "current", []v150Expected{{4, 6, 7}, {7, 6, 7}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		for _, closed := range []bool{false, true} {
			t.Run(fmt.Sprintf("ending %q closed=%t", eol, closed), func(t *testing.T) {
				result := v150Multifill(t, v150Invalid(t, "She _____{lives", 3))
				if !closed {
					result.Lines = result.Lines[:4]
					*result.Document.LineCount = 4
				}
				for i := range result.Lines {
					if eol != "" {
						result.Lines[i].LineEnding = eol
					}
				}
				result.Lines[len(result.Lines)-1].LineEnding = eol
				want := []v150Expected{{4, 10, 11}}
				v150Run(t, result, "first", want)
				v150Run(t, v150Multifill(t, v150Line(t, "Read.", "content", 3, v150Element(t, "content", "Read.", "Read.", 1))), "other", nil)
				v150Run(t, result, "second", want)
			})
		}
	}
}

// v150Invalid задаёт восстановленную повреждённую строку с unparsed.
func v150Invalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v150Line(t, raw, "invalid", parent, v150Element(t, "unparsed", raw, "", 1))
}

// v150Open создаёт открытие блока с исходным и каноническим именем.
func v150Open(t *testing.T, rawName, name string, parent int) model.Line {
	t.Helper()
	return v150Line(t, "@"+rawName+" {", "block-start", parent,
		v150Element(t, "tag", "@"+rawName, name, 1), v150Element(t, "block-open", "{", "{", len(rawName)+3))
}

// v150Close создаёт структурное закрытие указанного блока.
func v150Close(t *testing.T, parent int) model.Line {
	t.Helper()
	return v150Line(t, "}", "block-end", parent, v150Element(t, "block-close", "}", "}", 1))
}

// v150Multifill добавляет полноценное окружение; содержимое начинается со строки 4.
func v150Multifill(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{v150Version(t), v150Heading(t), v150Open(t, "multifill", "multifill", 2)}, body...)
	return v150Document(t, append(rows, v150Close(t, 3))...)
}

// v150Element создаёт элемент модели; диапазоны ожиданий задаются отдельно.
func v150Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v150Line задаёт тип и родителя без повторного разбора исходного текста.
func v150Line(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v150Version задаёт технический тег версии для контекста примеров.
func v150Version(t *testing.T) model.Line {
	t.Helper()
	return v150Line(t, "@dsl-version 1.2", "tag", 0,
		v150Element(t, "tag", "@dsl-version", "dsl-version", 1), v150Element(t, "version", "1.2", "1.2", 14))
}

// v150Heading добавляет обязательный заголовок документа перед фрагментом.
func v150Heading(t *testing.T) model.Line {
	t.Helper()
	return v150Line(t, "# Lesson", "heading", 0,
		v150Element(t, "heading-level", "#", "1", 1), v150Element(t, "title", "Lesson", "Lesson", 3))
}

// v150Document оформляет заданную модель и обязательные исходные ссылки unparsed.
// Причины повреждения не восстанавливаются: классификация передана тестом явно.
func v150Document(t *testing.T, rows ...model.Line) *model.Result {
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

// v150Registry создаёт реестр из полного штатного каталога.
func v150Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v150Snapshot сохраняет модель для проверки неизменности истории и исходных данных.
func v150Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v150Run запускает штатный конвейер и проверяет только новые срабатывания V150.
func v150Run(t *testing.T, result *model.Result, processing string, want []v150Expected) {
	t.Helper()
	before := v150Snapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v150Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v150Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processing, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processing); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V150" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V150 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.ID == "" || entry.Source != processing ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "Ответ в фигурных скобках не закрыт в строке пропуска." {
			t.Fatalf("V150 occurrence = %+v, want %+v", entry, expected)
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
	if err := json.Unmarshal([]byte(v150Snapshot(t, result)), &restored); err != nil {
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
	if v150Snapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

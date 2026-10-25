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

// v172Expected задаёт строку, начало и исключающий конец диапазона в Unicode.
type v172Expected [3]int

func TestV172Registration(t *testing.T) {
	definitions, err := v172Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V172" {
			continue
		}
		count++
		if definition.Message != "Тег не разрешён внутри @matching." || definition.Severity != "error" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V172 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v172Empty(t, version)
			before := v172Snapshot(t, result)
			checks, err := validator.Prepare(result, v172Registry(t))
			if err == nil || checks != nil || v172Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV172ReferenceAndExample(t *testing.T) {
	// Исходные фрагменты сохранены дословно; три строки префикса задают
	// версию, идентификатор документа и контекст урока, не меняя их содержимое.
	for _, tc := range []struct {
		name string
		rows []model.Line
		raw  string
		want []v172Expected
	}{
		{"reference", nil, "@matching {\nСолнце — звезда?\n---\nДа\nНет\n}", nil},
		{"example", []model.Line{v172TagLine(t, "note", "content", "Вспомните определение звезды.", 4, 2)},
			"@matching {\nСолнце — звезда?\n@note Вспомните определение звезды.\n---\nДа\nНет\n}", []v172Expected{{6, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v172Example(t, tc.rows...)
			var raw strings.Builder
			for _, line := range result.Lines[3:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.raw {
				t.Fatalf("source fragment changed: %q", raw.String())
			}
			v172Run(t, result, "current", tc.want)
		})
	}
}

func TestV172DirectDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *model.Result
		want   []v172Expected
	}{
		{"first child", v172Block(t, v172TagLine(t, "note", "content", "Текст.", 4, 2),
			v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2), v172Separator(t, "---", 4, 2),
			v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2)), []v172Expected{{5, 1, 6}}},
		{"last child below separator", v172Block(t, v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2),
			v172Separator(t, "---", 4, 2), v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2),
			v172TagLine(t, "instr", "content", "Текст.", 4, 2)), []v172Expected{{9, 1, 7}}},
		{"multiple occurrences", v172Block(t, v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2),
			v172TagLine(t, "note", "content", "Текст.", 4, 2), v172Separator(t, "---", 4, 2),
			v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2),
			v172TagLine(t, "instr", "content", "Текст.", 4, 2)), []v172Expected{{6, 1, 6}, {10, 1, 7}}},
		{"single line editor is forbidden", v172Example(t, v172TagLine(t, "editor", "content", "Комментарий.", 4, 2)), []v172Expected{{6, 1, 8}}},
		{"tag without content", v172Example(t, v172TagLine(t, "newpage", "", "", 4, 2)), []v172Expected{{6, 1, 9}}},
		{"case indentation and Unicode", v172Example(t, v172Line(t, " \t@NoTe Текст🙂", "tag", 4, 2,
			v172Element(t, "tag", "@NoTe", "note", 3, 8), v172Element(t, "content", "Текст🙂", "Текст🙂", 9, 15))), []v172Expected{{6, 3, 8}}},
		{"escaped tag", v172Example(t, v172Text(t, `\@note Текст.`, "@note Текст.", 4, 2)), nil},
		{"indented escaped tag", v172Example(t, v172Text(t, " \t\\@note Текст.", "@note Текст.", 4, 2)), nil},
		{"mention inside text", v172Example(t, v172Text(t, "Текст @note", "Текст @note", 4, 2)), nil},
		{"blank lines", v172Example(t, v172Line(t, "", "blank", 4, 2), v172Line(t, " \t ", "blank", 4, 2)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v172Run(t, tc.result, "current", tc.want) })
	}
	t.Run("matching case and instruction", func(t *testing.T) {
		v172Run(t, v172Document(t, v172Open(t, "MaTcHiNg", "Выберите ответ.", 3, 1),
			v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2), v172TagLine(t, "note", "content", "Текст.", 4, 2),
			v172Separator(t, "---", 4, 2), v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2), v172Close(t, 4, 2)),
			"current", []v172Expected{{6, 1, 6}})
	})
}

func TestV172Exceptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
	}{
		{"single line answer and hint", []model.Line{v172TagLine(t, "answer", "content", "Да", 4, 2), v172TagLine(t, "hint", "content", "Подумайте.", 4, 2)}},
		{"uppercase exceptions", []model.Line{v172TagLine(t, "ANSWER", "content", "Да", 4, 2), v172TagLine(t, "HiNt", "content", "Подумайте.", 4, 2)}},
		{"hint association is separate", []model.Line{v172TagLine(t, "hint", "content", "Без ответа.", 4, 2), v172TagLine(t, "hint", "content", "Повтор.", 4, 2)}},
		{"block answer and hint", []model.Line{v172Open(t, "answer", "", 4, 2), v172Text(t, "Да", "Да", 6, 3), v172Close(t, 6, 3),
			v172Open(t, "hint", "", 4, 2), v172Text(t, "Подумайте.", "Подумайте.", 9, 3), v172Close(t, 9, 3)}},
		{"tag inside answer is not a direct child", []model.Line{v172Open(t, "answer", "", 4, 2),
			v172Text(t, "Да", "Да", 6, 3), v172TagLine(t, "note", "content", "Текст.", 6, 3), v172Close(t, 6, 3)}},
		{"tag inside hint is not a direct child", []model.Line{v172TagLine(t, "answer", "content", "Да", 4, 2), v172Open(t, "hint", "", 4, 2),
			v172TagLine(t, "note", "content", "Текст.", 7, 3), v172Close(t, 7, 3)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v172Run(t, v172Example(t, tc.rows...), "current", nil) })
	}
	t.Run("answer and hint remain exempt beside forbidden include", func(t *testing.T) {
		v172Run(t, v172FragmentDocument(t, v172Open(t, "matching", "", 3, 1),
			v172Text(t, "Солнце?", "Солнце?", 4, 2), v172Separator(t, "---", 4, 2),
			v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2),
			v172TagLine(t, "answer", "content", "Да", 4, 2), v172TagLine(t, "hint", "content", "Текст.", 4, 2),
			v172TagLine(t, "include", "identifier", "f", 4, 2), v172Close(t, 4, 2)), "current", []v172Expected{{11, 1, 9}})
	})
}

func TestV172Includes(t *testing.T) {
	// DSL 3.13.10 запрещает include внутри интерактивного блока;
	// разрешённые answer/hint перечислены в 4.7.12 и 4.7.19.
	for _, tc := range []struct {
		name          string
		before, after []model.Line
		noSeparator   bool
		want          []v172Expected
	}{
		{"include above separator", []model.Line{v172TagLine(t, "include", "identifier", "f", 4, 2)}, nil,
			false, []v172Expected{{6, 1, 9}}},
		{"uppercase include", []model.Line{v172TagLine(t, "INCLUDE", "identifier", "f", 4, 2)}, nil,
			false, []v172Expected{{6, 1, 9}}},
		{"include below separator", nil, []model.Line{v172TagLine(t, "include", "identifier", "f", 4, 2)},
			false, []v172Expected{{9, 1, 9}}},
		{"two direct includes with case and indentation", []model.Line{v172TagLine(t, "include", "identifier", "f", 4, 2)},
			[]model.Line{v172Line(t, " \t@InClUdE F", "tag", 4, 2,
				v172Element(t, "tag", "@InClUdE", "include", 3, 11), v172Element(t, "identifier", "F", "F", 12, 13))},
			false, []v172Expected{{6, 1, 9}, {10, 3, 11}}},
		{"include without separator", []model.Line{v172TagLine(t, "include", "identifier", "f", 4, 2)}, nil,
			true, []v172Expected{{6, 1, 9}}},
		{"saved include tag with unparsed tail", []model.Line{v172Line(t, "@include f extra", "invalid", 4, 2,
			v172Element(t, "tag", "@include", "include", 1, 9), v172Element(t, "identifier", "f", "f", 10, 11),
			v172Unparsed(t, "extra", 12, 17))}, nil, false, []v172Expected{{6, 1, 9}}},
		{"erroneous include block is reported once", []model.Line{
			v172Line(t, "@include f {", "block-start", 4, 2, v172Element(t, "tag", "@include", "include", 1, 9),
				v172Element(t, "identifier", "f", "f", 10, 11), v172Element(t, "block-open", "{", "{", 12, 13)),
			v172TagLine(t, "include", "identifier", "f", 6, 3), v172TagLine(t, "note", "content", "Текст.", 6, 3), v172Close(t, 6, 3),
		}, nil, false, []v172Expected{{6, 1, 9}}},
		{"include inside forbidden note adds no occurrence", []model.Line{v172Open(t, "note", "", 4, 2),
			v172TagLine(t, "include", "identifier", "f", 6, 3), v172Close(t, 6, 3)}, nil, false, []v172Expected{{6, 1, 6}}},
		{"include inside hint is not a direct matching child", []model.Line{v172TagLine(t, "answer", "content", "Да", 4, 2),
			v172Open(t, "hint", "", 4, 2), v172TagLine(t, "include", "identifier", "f", 7, 3), v172Close(t, 7, 3)}, nil, false, nil},
		{"escaped and textual mentions", []model.Line{v172Text(t, `\@include f`, "@include f", 4, 2),
			v172Text(t, " \t\\@include f", "@include f", 4, 2), v172Text(t, "Текст @include f", "Текст @include f", 4, 2)}, nil, false, nil},
		{"unparsed include is not reconstructed", []model.Line{v172Line(t, "@include f", "invalid", 4, 2,
			v172Unparsed(t, "@include f", 1, 11))}, nil, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v172Open(t, "matching", "", 3, 1), v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2)}
			body = append(body, tc.before...)
			if !tc.noSeparator {
				body = append(body, v172Separator(t, "---", 4, 2))
			}
			body = append(body, v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2))
			body = append(body, tc.after...)
			body = append(body, v172Close(t, 4, 2))
			v172Run(t, v172FragmentDocument(t, body...), "current", tc.want)
		})
	}
	t.Run("permitted include outside matching", func(t *testing.T) {
		v172Run(t, v172FragmentDocument(t, v172TagLine(t, "task", "identifier", "practice", 3, 1),
			v172TagLine(t, "include", "identifier", "f", 4, 2), v172TagLine(t, "endtask", "", "", 3, 1)), "current", nil)
	})
}

func TestV172ForbiddenBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v172Expected
	}{
		{"note with nested tag", []model.Line{v172Open(t, "note", "", 4, 2),
			v172TagLine(t, "instr", "content", "Текст.", 6, 3), v172Close(t, 6, 3)}, []v172Expected{{6, 1, 6}}},
		{"empty forbidden block", []model.Line{v172Open(t, "note", "", 4, 2), v172Close(t, 6, 3)}, []v172Expected{{6, 1, 6}}},
		{"editor with literal tag", []model.Line{v172Open(t, "editor", "", 4, 2),
			v172Text(t, "@note Текст.", "@note Текст.", 6, 3), v172Close(t, 6, 3)}, []v172Expected{{6, 1, 8}}},
		{"choice", []model.Line{v172Open(t, "choice", "", 4, 2), v172TagLine(t, "answer", "content", "Да", 6, 3),
			v172Text(t, "Нет", "Нет", 6, 3), v172Close(t, 6, 3)}, []v172Expected{{6, 1, 8}}},
		{"multichoice", []model.Line{v172Open(t, "multichoice", "", 4, 2), v172TagLine(t, "answer", "content", "Да", 6, 3),
			v172Text(t, "Нет", "Нет", 6, 3), v172Close(t, 6, 3)}, []v172Expected{{6, 1, 13}}},
		{"nested matching checked independently", []model.Line{v172Open(t, "matching", "", 4, 2),
			v172Text(t, "Вода мокрая?", "Вода мокрая?", 6, 3), v172TagLine(t, "note", "content", "Текст.", 6, 3),
			v172Separator(t, "---", 6, 3), v172Text(t, "Да", "Да", 6, 3), v172Text(t, "Нет", "Нет", 6, 3), v172Close(t, 6, 3)},
			[]v172Expected{{6, 1, 10}, {8, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v172Run(t, v172Example(t, tc.rows...), "current", tc.want) })
	}
}

func TestV172Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v172Expected
	}{
		{"unparsed declaration", v172Line(t, "@note", "invalid", 4, 2, v172Unparsed(t, "@note", 1, 6)), nil},
		{"longer unknown name", v172Line(t, "@noteExtra", "invalid", 4, 2, v172Unparsed(t, "@noteExtra", 1, 11)), nil},
		{"saved tag in invalid line", v172Line(t, "@note Текст. }", "invalid", 4, 2,
			v172Element(t, "tag", "@note", "note", 1, 6), v172Element(t, "content", "Текст.", "Текст.", 7, 13), v172Unparsed(t, "}", 14, 15)),
			[]v172Expected{{6, 1, 6}}},
		{"saved tag with unparsed tail", v172Line(t, "@note Текст. }", "tag", 4, 2,
			v172Element(t, "tag", "@note", "note", 1, 6), v172Element(t, "content", "Текст.", "Текст.", 7, 13), v172Unparsed(t, "}", 14, 15)),
			[]v172Expected{{6, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v172Run(t, v172Example(t, tc.line), "current", tc.want) })
	}
}

func TestV172Context(t *testing.T) {
	t.Run("task stage", func(t *testing.T) {
		v172Run(t, v172Document(t, v172TagLine(t, "task", "identifier", "practice", 3, 1), v172TagLine(t, "step", "title", "Выбор", 4, 2),
			v172Open(t, "matching", "", 5, 3), v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 6, 4),
			v172TagLine(t, "note", "content", "Текст.", 6, 4), v172Separator(t, "---", 6, 4),
			v172Text(t, "Да", "Да", 6, 4), v172Text(t, "Нет", "Нет", 6, 4), v172Close(t, 6, 4), v172TagLine(t, "endtask", "", "", 3, 1)),
			"current", []v172Expected{{8, 1, 6}})
	})
	t.Run("variant", func(t *testing.T) {
		v172Run(t, v172Document(t, v172Open(t, "variants", "", 3, 1), v172TagLine(t, "variant", "name", "A", 4, 2),
			v172Open(t, "matching", "", 5, 3), v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 6, 4),
			v172TagLine(t, "note", "content", "Текст.", 6, 4), v172Separator(t, "---", 6, 4),
			v172Text(t, "Да", "Да", 6, 4), v172Text(t, "Нет", "Нет", 6, 4), v172Close(t, 6, 4),
			v172TagLine(t, "variant", "name", "B", 4, 2), v172TagLine(t, "note", "content", "Текст.", 13, 3), v172Close(t, 4, 2)),
			"current", []v172Expected{{8, 1, 6}})
	})
	t.Run("outside and after matching", func(t *testing.T) {
		v172Run(t, v172Document(t, v172TagLine(t, "note", "content", "До.", 3, 1), v172Open(t, "matching", "", 3, 1),
			v172Text(t, "Солнце?", "Солнце?", 5, 2), v172Separator(t, "---", 5, 2), v172Text(t, "Да", "Да", 5, 2),
			v172Text(t, "Нет", "Нет", 5, 2), v172Close(t, 5, 2), v172TagLine(t, "note", "content", "После.", 3, 1)), "current", nil)
	})
	t.Run("two independent blocks", func(t *testing.T) {
		v172Run(t, v172Document(t, v172Open(t, "matching", "", 3, 1), v172Text(t, "Солнце?", "Солнце?", 4, 2),
			v172TagLine(t, "note", "content", "Текст.", 4, 2), v172Separator(t, "---", 4, 2),
			v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2), v172Close(t, 4, 2),
			v172Open(t, "matching", "", 3, 1), v172Text(t, "Вода?", "Вода?", 11, 2), v172TagLine(t, "instr", "content", "Текст.", 11, 2),
			v172Separator(t, "---", 11, 2), v172Text(t, "Да", "Да", 11, 2), v172Text(t, "Нет", "Нет", 11, 2), v172Close(t, 11, 2)),
			"current", []v172Expected{{6, 1, 6}, {13, 1, 7}})
	})
}

func TestV172Boundaries(t *testing.T) {
	t.Run("empty input", func(t *testing.T) { v172Run(t, v172Empty(t, "1.2"), "current", nil) })
	t.Run("no matching", func(t *testing.T) {
		v172Run(t, v172Document(t, v172TagLine(t, "note", "content", "Текст.", 3, 1)), "current", nil)
	})
	t.Run("empty block", func(t *testing.T) { v172Run(t, v172Block(t), "current", nil) })
	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprintf("separator count %d", count), func(t *testing.T) {
			rows := []model.Line{v172Text(t, "Солнце?", "Солнце?", 4, 2), v172TagLine(t, "note", "content", "Текст.", 4, 2)}
			for i := 0; i < count; i++ {
				rows = append(rows, v172Separator(t, "---", 4, 2))
			}
			rows = append(rows, v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2))
			v172Run(t, v172Block(t, rows...), "current", []v172Expected{{6, 1, 6}})
		})
	}
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v172Example(t, v172TagLine(t, "note", "content", "Текст.", 4, 2))
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v172Run(t, result, "current", []v172Expected{{6, 1, 6}})
			})
		}
	}
	t.Run("history and links preserved", func(t *testing.T) {
		result := v172Example(t, v172TagLine(t, "note", "content", "Текст.", 4, 2))
		v172Run(t, result, "first", []v172Expected{{6, 1, 6}})
		v172Run(t, result, "second", []v172Expected{{6, 1, 6}})
	})
}

// v172Registry создаёт реестр из полного штатного каталога регистраций.
func v172Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v172Empty создаёт модель без исходных строк и с явно указанной версией.
func v172Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v172Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v172Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v172Empty(t, "1.2")
	title, id := "Lesson", "v172"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v172TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v172TagLine(t, "document-id", "identifier", "v172", 0, 0),
		v172Line(t, "# Lesson", "heading", 0, 0, v172Element(t, "heading-level", "#", "1", 1, 2), v172Element(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			errorID := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{errorID}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: errorID, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v172FragmentDocument добавляет определение f после сценария без сдвига его строк.
// DSL 3.12.9 разрешает ссылаться на фрагмент до места его объявления.
func v172FragmentDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	fragment := len(body) + 4
	body = append(body, v172Line(t, "@fragment f {", "block-start", 3, 1,
		v172Element(t, "tag", "@fragment", "fragment", 1, 10), v172Element(t, "identifier", "f", "f", 11, 12),
		v172Element(t, "block-open", "{", "{", 13, 14)),
		v172Text(t, "Материал фрагмента.", "Материал фрагмента.", fragment, 2), v172Close(t, fragment, 2))
	return v172Document(t, body...)
}

// v172Block обрамляет явно заданные строки блоком matching на строке 4.
func v172Block(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v172Open(t, "matching", "", 3, 1)}, rows...)
	body = append(body, v172Close(t, 4, 2))
	return v172Document(t, body...)
}

// v172Example сохраняет вопрос и нижние варианты примера, добавляя явные строки после вопроса.
func v172Example(t *testing.T, upper ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{v172Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2)}, upper...)
	rows = append(rows, v172Separator(t, "---", 4, 2), v172Text(t, "Да", "Да", 4, 2), v172Text(t, "Нет", "Нет", 4, 2))
	return v172Block(t, rows...)
}

// v172Line сохраняет явно заданные данные физической строки и её родителя.
func v172Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v172Element задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v172Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v172Unparsed задаёт фрагмент без значения; историю добавляет v172Document.
func v172Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v172TagLine создаёт объявление с явно заданным типом аргумента.
func v172TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v172Element(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v172Element(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v172Line(t, raw, "tag", parent, depth, elements...)
}

// v172Text задаёт обычную строку с независимо указанным семантическим значением.
func v172Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v172Line(t, raw, "content", parent, depth, v172Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v172Open задаёт блочное объявление с необязательной инструкцией.
func v172Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v172TagLine(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v172Element(t, "block-open", "{", "{", start, start+1))
	return line
}

// v172Close задаёт отдельную закрывающую скобку с явным родителем.
func v172Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v172Line(t, "}", "block-end", parent, depth, v172Element(t, "block-close", "}", "}", 1, 2))
}

// v172Separator задаёт явно классифицированную строку-разделитель.
func v172Separator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v172Line(t, raw, "separator", parent, depth)
}

// v172Run запускает неизменённую выборку штатного каталога и проверяет новые V172.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v172Run(t *testing.T, result *model.Result, source string, want []v172Expected) {
	t.Helper()
	before := v172Snapshot(t, result)
	checks, err := validator.Prepare(result, v172Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v172Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("unexpected new source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V172" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V172 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 0 ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "Тег не разрешён внутри @matching." {
			t.Errorf("occurrence = %+v, want range %+v and exact V172 metadata", entry, location)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id != entry.ID {
						continue
					}
					links++
					if line.Number != expected[0] || element.Start != expected[1] || element.End != expected[2] ||
						element.ElementType != "tag" {
						t.Error("V172 linked to wrong element")
					}
				}
			}
		}
		if links != 1 {
			t.Errorf("V172 links = %d, want 1", links)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v172Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Lines) != len(original.Lines) || len(actual.Processing) != oldProcessing+1 {
		t.Fatal("source or history length changed")
	}
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		if len(actual.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v172Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v172Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v172Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

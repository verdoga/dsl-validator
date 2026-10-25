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

// v174Expected задаёт начало и исключающий конец диапазона: строка, Unicode-колонка.
type v174Expected [4]int

func TestV174Registration(t *testing.T) {
	definitions, err := v174Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V174" {
			continue
		}
		count++
		if definition.Message != "В одном задании только часть вопросов с выбором ответа имеет автопроверку." || definition.Severity != "error" ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V174 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v174Empty(t, version)
			before := v174Snapshot(t, result)
			checks, err := validator.Prepare(result, v174Registry(t))
			if err == nil || checks != nil || v174Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV174ReferenceAndExample(t *testing.T) {
	// Три строки префикса задают версию, идентификатор и контекст урока.
	// Значимое содержимое обоих фрагментов таблицы не меняется.
	for _, tc := range []struct {
		name      string
		automatic bool
		raw       string
		want      []v174Expected
	}{
		{"reference", true, "@task 1\n@choice Choose a colour. {\n@answer Red\nBlue\n}\n@multichoice Choose animals. {\n@answer Cat\n@answer Dog\nStone\n}\n@endtask", nil},
		{"example", false, "@task 1\n@choice Choose a colour. {\n@answer Red\nBlue\n}\n@multichoice Choose animals. {\nCat\nDog\nStone\n}\n@endtask", []v174Expected{{5, 1, 13, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v174Document(t, v174TaskRows(t, 4, 3, 1, "1", true, tc.automatic)...)
			var raw strings.Builder
			for _, line := range result.Lines[3:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.raw {
				t.Fatalf("source fragment changed: %q", raw.String())
			}
			v174Run(t, result, "current", tc.want)
		})
	}
}

func TestV174Counts(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first, second bool
		want          []v174Expected
	}{
		{"neither automatic", false, false, nil},
		{"all automatic", true, true, nil},
		{"first automatic", true, false, []v174Expected{{5, 1, 13, 2}}},
		{"last automatic", false, true, []v174Expected{{5, 1, 13, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v174Run(t, v174Document(t, v174TaskRows(t, 4, 3, 1, "1", tc.first, tc.second)...), "current", tc.want)
		})
	}
	for _, automatic := range []bool{false, true} {
		t.Run(fmt.Sprintf("single question automatic %t", automatic), func(t *testing.T) {
			rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1)}
			rows = append(rows, v174Colour(t, 5, 4, 2, automatic)...)
			rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
			v174Run(t, v174Document(t, rows...), "current", nil)
		})
	}
	for _, name := range []string{"choice", "multichoice"} {
		t.Run("only "+name, func(t *testing.T) {
			rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
			rows[1] = v174Open(t, name, "Choose a colour.", 4, 2)
			rows[5] = v174Open(t, name, "Choose animals.", 4, 2)
			v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 13, 2}})
		})
	}
	t.Run("repeated block is a separate last question", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows = append(rows[:10], v174Colour(t, 14, 4, 2, true)...)
		rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 17, 2}})
	})
}

func TestV174Answers(t *testing.T) {
	t.Run("block answer", func(t *testing.T) {
		rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1), v174Open(t, "choice", "Choose a colour.", 4, 2),
			v174Open(t, "answer", "", 5, 3), v174Text(t, "Red", "Red", 6, 4), v174Close(t, 6, 4),
			v174Text(t, "Blue", "Blue", 5, 3), v174Close(t, 5, 3)}
		rows = append(rows, v174Animals(t, 11, 4, 2, false)...)
		rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 15, 2}})
	})
	for _, tc := range []struct {
		name string
		line model.Line
		want []v174Expected
	}{
		{"empty answer counts", v174TagLine(t, "answer", "", "", 5, 3), []v174Expected{{5, 1, 13, 2}}},
		{"unparsed content counts", v174Line(t, "@answer }", "tag", 5, 3,
			v174TestElement(t, "tag", "@answer", "answer", 1, 8), v174Unparsed(t, "}", 9, 10)), []v174Expected{{5, 1, 13, 2}}},
		{"invalid answer does not count", v174Line(t, "@answer }", "invalid", 5, 3,
			v174TestElement(t, "tag", "@answer", "answer", 1, 8), v174Unparsed(t, "}", 9, 10)), nil},
		{"unparsed declaration does not count", v174Line(t, "@answer", "invalid", 5, 3, v174Unparsed(t, "@answer", 1, 8)), nil},
		{"escaped answer is text", v174Text(t, `\@answer Red`, "@answer Red", 5, 3), nil},
		{"mentioned answer is text", v174Text(t, "Text @answer Red", "Text @answer Red", 5, 3), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
			rows[2] = tc.line
			v174Run(t, v174Document(t, rows...), "current", tc.want)
		})
	}
	t.Run("one or multiple answers give the same flag", func(t *testing.T) {
		for _, multiple := range []bool{false, true} {
			rows := v174TaskRows(t, 4, 3, 1, "1", true, true)
			if !multiple {
				rows[7] = v174Text(t, "Dog", "Dog", 9, 3)
			}
			v174Run(t, v174Document(t, rows...), "current", nil)
		}
	})
	t.Run("answer of nested hint is not owned by question", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows = append(rows[:9], v174Open(t, "hint", "", 9, 3), v174TagLine(t, "answer", "content", "Cat", 13, 4),
			v174Close(t, 13, 4), v174Close(t, 9, 3), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 16, 2}})
	})
	t.Run("empty block answer still counts", func(t *testing.T) {
		rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1), v174Open(t, "choice", "", 4, 2),
			v174Open(t, "answer", "", 5, 3), v174Close(t, 6, 4), v174Text(t, "Red", "Red", 5, 3),
			v174Text(t, "Blue", "Blue", 5, 3), v174Close(t, 5, 3)}
		rows = append(rows, v174Animals(t, 11, 4, 2, false)...)
		rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 15, 2}})
	})
}

func TestV174TaskGroups(t *testing.T) {
	t.Run("steps do not divide the task", func(t *testing.T) {
		rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1), v174TagLine(t, "step", "title", "Colours", 4, 2)}
		rows = append(rows, v174Colour(t, 6, 5, 3, true)...)
		rows = append(rows, v174TagLine(t, "step", "title", "Animals", 4, 2))
		rows = append(rows, v174Animals(t, 11, 10, 3, false)...)
		rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{6, 1, 15, 2}})
	})
	t.Run("nested content is included", func(t *testing.T) {
		// Проверка учитывает вопросы и при запрещённом размещении внутри
		// контентного блока: ограничение размещения не входит в V174.
		rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1), v174Open(t, "example", "", 4, 2)}
		rows = append(rows, v174Colour(t, 6, 5, 3, true)...)
		rows = append(rows, v174Animals(t, 10, 5, 3, false)...)
		rows = append(rows, v174Close(t, 5, 3), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{6, 1, 14, 2}})
	})
	t.Run("last question is selected by opening order", func(t *testing.T) {
		rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1), v174Open(t, "choice", "", 4, 2),
			v174TagLine(t, "answer", "content", "Red", 5, 3), v174Text(t, "Blue", "Blue", 5, 3)}
		rows = append(rows, v174Animals(t, 8, 5, 3, false)...)
		rows = append(rows, v174Close(t, 5, 3), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 12, 2}})
	})
	t.Run("same IDs do not merge tasks", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, true)
		rows = append(rows, v174TaskRows(t, 15, 3, 1, "1", false, false)...)
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
	t.Run("two mixed tasks produce two findings", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows = append(rows, v174TaskRows(t, 15, 3, 1, "2", false, true)...)
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 13, 2}, {16, 1, 24, 2}})
	})
	t.Run("branches remain separate", func(t *testing.T) {
		rows := []model.Line{v174Open(t, "variants", "", 3, 1), v174TagLine(t, "variant", "name", "A", 4, 2)}
		rows = append(rows, v174TaskRows(t, 6, 5, 3, "a-1", true, false)...)
		rows = append(rows, v174TagLine(t, "variant", "name", "B", 4, 2))
		rows = append(rows, v174TaskRows(t, 18, 17, 3, "b-1", true, true)...)
		rows = append(rows, v174Close(t, 4, 2))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{7, 1, 15, 2}})
	})
	t.Run("questions outside tasks are ignored", func(t *testing.T) {
		rows := v174Colour(t, 4, 3, 1, true)
		rows = append(rows, v174Animals(t, 8, 3, 1, false)...)
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
	t.Run("question after endtask is outside", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, true)
		rows = append(rows, v174Animals(t, 15, 3, 1, false)...)
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
}

func TestV174ExclusionsAndKey(t *testing.T) {
	t.Run("key does not cancel finding", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows = append(rows[:10], v174Open(t, "key", "", 4, 2), v174Text(t, "Red; Cat, Dog.", "Red; Cat, Dog.", 14, 3),
			v174Close(t, 14, 3), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 1, 13, 2}})
	})
	t.Run("other question type does not change group", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", false, false)
		rows = append(rows[:10], v174TagLine(t, "question", "content", "Where?", 4, 2),
			v174TagLine(t, "answer", "content", "London", 4, 2), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
	t.Run("matching answer is not a choice answer", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", false, false)
		rows = append(rows[:10], v174Open(t, "matching", "", 4, 2), v174Text(t, "Солнце — звезда?", "Солнце — звезда?", 14, 3),
			v174TagLine(t, "answer", "content", "Да", 14, 3), v174Line(t, "---", "separator", 14, 3),
			v174Text(t, "Да", "Да", 14, 3), v174Text(t, "Нет", "Нет", 14, 3), v174Close(t, 14, 3), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
	for _, text := range []struct{ raw, value string }{
		{`\@choice \{`, "@choice {"},
		{`Text @multichoice \{`, "Text @multichoice {"},
	} {
		t.Run(text.raw, func(t *testing.T) {
			rows := v174TaskRows(t, 4, 3, 1, "1", true, true)
			rows = append(rows[:10], v174Text(t, text.raw, text.value, 4, 2), v174TagLine(t, "endtask", "", "", 3, 1))
			v174Run(t, v174Document(t, rows...), "current", nil)
		})
	}
	t.Run("unparsed question name is not recovered", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows[5] = v174Line(t, "@multichoiceExtra {", "block-start", 4, 2,
			v174Unparsed(t, "@multichoiceExtra", 1, 18), v174TestElement(t, "block-open", "{", "{", 19, 20))
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
	t.Run("tag line is not a question block", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows = append(rows[:5], v174TagLine(t, "multichoice", "", "", 4, 2), v174TagLine(t, "endtask", "", "", 3, 1))
		v174Run(t, v174Document(t, rows...), "current", nil)
	})
}

func TestV174ClosingBoundaries(t *testing.T) {
	t.Run("missing final close", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		result := v174Document(t, rows[:9]...)
		v174ParserError(t, result, 9, 1, 13)
		v174Run(t, result, "current", nil)
	})
	for _, extra := range []int{1, 2} {
		t.Run(fmt.Sprintf("ambiguous final close with %d extras", extra), func(t *testing.T) {
			rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
			rows = rows[:10]
			for i := 0; i < extra; i++ {
				rows = append(rows, v174Close(t, 9, 3))
			}
			rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
			result := v174Document(t, rows...)
			for i := 0; i < extra; i++ {
				v174ParserError(t, result, 14+i, 1, 2)
			}
			v174Run(t, result, "current", nil)
		})
	}
	t.Run("ambiguity does not suppress another task", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows = append(rows[:10], v174Close(t, 9, 3), v174TagLine(t, "endtask", "", "", 3, 1))
		rows = append(rows, v174TaskRows(t, 16, 3, 1, "2", true, false)...)
		result := v174Document(t, rows...)
		v174ParserError(t, result, 14, 1, 2)
		v174Run(t, result, "current", []v174Expected{{17, 1, 25, 2}})
	})
	t.Run("earlier ambiguous close does not remove a question", func(t *testing.T) {
		rows := []model.Line{v174TagLine(t, "task", "identifier", "1", 3, 1)}
		rows = append(rows, v174Colour(t, 5, 4, 2, true)...)
		rows = append(rows, v174Close(t, 5, 3))
		rows = append(rows, v174Animals(t, 10, 4, 2, false)...)
		rows = append(rows, v174TagLine(t, "endtask", "", "", 3, 1))
		result := v174Document(t, rows...)
		v174ParserError(t, result, 9, 1, 2)
		v174Run(t, result, "current", []v174Expected{{5, 1, 14, 2}})
	})
}

func TestV174SourceAndEmptyInput(t *testing.T) {
	t.Run("empty input", func(t *testing.T) { v174Run(t, v174Empty(t, "1.2"), "current", nil) })
	t.Run("task without questions", func(t *testing.T) {
		v174Run(t, v174Document(t, v174TagLine(t, "task", "identifier", "1", 3, 1),
			v174TagLine(t, "note", "content", "Read.", 4, 2), v174TagLine(t, "endtask", "", "", 3, 1)), "current", nil)
	})
	t.Run("case spacing indentation and Unicode", func(t *testing.T) {
		rows := v174TaskRows(t, 4, 3, 1, "1", true, false)
		rows[0] = v174TagLine(t, "TaSk", "identifier", "1", 3, 1)
		rows[1] = v174Line(t, " \t@ChOiCe  Choose a colour.  {", "block-start", 4, 2,
			v174TestElement(t, "tag", "@ChOiCe", "choice", 3, 10), v174TestElement(t, "content", "Choose a colour.", "Choose a colour.", 12, 28),
			v174TestElement(t, "block-open", "{", "{", 30, 31))
		rows[2] = v174TagLine(t, "AnSwEr", "content", "Красный🙂", 5, 3)
		rows[5] = v174Open(t, "MULTICHOICE", "Выберите животных.", 4, 2)
		rows[9] = v174Line(t, " \t} \t", "block-end", 9, 3, v174TestElement(t, "block-close", "}", "}", 3, 4))
		v174Run(t, v174Document(t, rows...), "current", []v174Expected{{5, 3, 13, 4}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v174Document(t, v174TaskRows(t, 4, 3, 1, "1", true, false)...)
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v174Run(t, result, "current", []v174Expected{{5, 1, 13, 2}})
			})
		}
	}
	t.Run("history survives repeated execution", func(t *testing.T) {
		result := v174Document(t, v174TaskRows(t, 4, 3, 1, "1", true, false)...)
		v174Run(t, result, "first", []v174Expected{{5, 1, 13, 2}})
		v174Run(t, result, "second", []v174Expected{{5, 1, 13, 2}})
	})
}

// v174Colour задаёт четыре строки вопроса о цвете с явно выбранной формой варианта.
func v174Colour(t *testing.T, opening, parent, depth int, automatic bool) []model.Line {
	t.Helper()
	red := v174Text(t, "Red", "Red", opening, depth+1)
	if automatic {
		red = v174TagLine(t, "answer", "content", "Red", opening, depth+1)
	}
	return []model.Line{v174Open(t, "choice", "Choose a colour.", parent, depth), red,
		v174Text(t, "Blue", "Blue", opening, depth+1), v174Close(t, opening, depth+1)}
}

// v174Animals задаёт пять строк вопроса о животных; варианты и родители явные.
func v174Animals(t *testing.T, opening, parent, depth int, automatic bool) []model.Line {
	t.Helper()
	cat := v174Text(t, "Cat", "Cat", opening, depth+1)
	dog := v174Text(t, "Dog", "Dog", opening, depth+1)
	if automatic {
		cat = v174TagLine(t, "answer", "content", "Cat", opening, depth+1)
		dog = v174TagLine(t, "answer", "content", "Dog", opening, depth+1)
	}
	return []model.Line{v174Open(t, "multichoice", "Choose animals.", parent, depth), cat, dog,
		v174Text(t, "Stone", "Stone", opening, depth+1), v174Close(t, opening, depth+1)}
}

// v174TaskRows задаёт одиннадцать строк эталона или примера и готовые связи.
func v174TaskRows(t *testing.T, number, parent, depth int, id string, first, second bool) []model.Line {
	t.Helper()
	rows := []model.Line{v174TagLine(t, "task", "identifier", id, parent, depth)}
	rows = append(rows, v174Colour(t, number+1, number, depth+1, first)...)
	rows = append(rows, v174Animals(t, number+5, number, depth+1, second)...)
	return append(rows, v174TagLine(t, "endtask", "", "", parent, depth))
}

// v174ParserError сохраняет исходную ошибку границы блока на указанной строке.
func v174ParserError(t *testing.T, result *model.Result, line, start, end int) {
	t.Helper()
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1),
		Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "line",
		Message: "Ошибка закрытия блока.", Location: &model.Location{Start: model.Position{Line: line, Column: start}, End: model.Position{Line: line, Column: end}},
		RelatedLocations: []model.Location{}})
	result.Lines[line-1].HasErrors, result.Document.HasErrors = true, true
}

// v174Registry создаёт реестр из полного штатного каталога регистраций.
func v174Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v174Empty создаёт модель без исходных строк и с явно указанной версией.
func v174Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v174Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v174Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v174Empty(t, "1.2")
	title, id := "Lesson", "v174"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v174TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v174TagLine(t, "document-id", "identifier", "v174", 0, 0),
		v174Line(t, "# Lesson", "heading", 0, 0, v174TestElement(t, "heading-level", "#", "1", 1, 2), v174TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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

// v174Line сохраняет явно заданные данные физической строки и её родителя.
func v174Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v174TestElement задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v174TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v174Unparsed задаёт фрагмент без значения; историю добавляет v174Document.
func v174Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v174TagLine создаёт объявление с явно заданным типом аргумента.
func v174TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v174TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v174TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v174Line(t, raw, "tag", parent, depth, elements...)
}

// v174Text задаёт обычную строку с независимо указанным семантическим значением.
func v174Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v174Line(t, raw, "content", parent, depth, v174TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v174Open задаёт блочное объявление с необязательной инструкцией.
func v174Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v174TagLine(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v174TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v174Close задаёт отдельную закрывающую скобку с явным родителем.
func v174Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v174Line(t, "}", "block-end", parent, depth, v174TestElement(t, "block-close", "}", "}", 1, 2))
}

// v174Run запускает неизменённую выборку штатного каталога и проверяет новые V174.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v174Run(t *testing.T, result *model.Result, source string, want []v174Expected) {
	t.Helper()
	before := v174Snapshot(t, result)
	checks, err := validator.Prepare(result, v174Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v174Snapshot(t, result) != before {
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
		if entry.DiagnosticCode == "V174" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V174 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[2], Column: expected[3]}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 0 ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Message != "В одном задании только часть вопросов с выбором ответа имеет автопроверку." {
			t.Errorf("occurrence = %+v, want range %+v and exact V174 metadata", entry, location)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Error("block diagnostic must not create element links")
				}
			}
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
	if err := json.Unmarshal([]byte(v174Snapshot(t, result)), &actual); err != nil {
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
	if v174Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v174Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v174Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

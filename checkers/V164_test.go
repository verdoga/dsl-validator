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

// v164Expected задаёт строку, начало и исключающий конец диапазона в Unicode.
type v164Expected [3]int

func TestV164Registration(t *testing.T) {
	definitions, err := v164Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V164" {
			continue
		}
		count++
		if definition.Message != "В верхнем списке @matching нет элементов." || definition.Severity != "error" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V164 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v164Empty(t, version)
			before := v164Snapshot(t, result)
			checks, err := validator.Prepare(result, v164Registry(t))
			if err == nil || checks != nil || v164Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV164ReferenceAndExample(t *testing.T) {
	// Дословные фрагменты дополнены @dsl-version, @document-id и # Lesson
	// по DSL 1.1, 1.30, 1.31 и требованию V037. Открытие блока — строка 4.
	t.Run("reference", func(t *testing.T) {
		v164Run(t, v164Example(t, v164Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2)), "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v164Run(t, v164Example(t), "current", []v164Expected{{4, 1, 10}})
	})
}

func TestV164UpperItems(t *testing.T) {
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v164Expected
	}{
		{"one item", []model.Line{v164Text(t, "Солнце?", "Солнце?", 4, 2)}, nil},
		{"two identical items", []model.Line{v164Text(t, "Солнце?", "Солнце?", 4, 2), v164Text(t, "Солнце?", "Солнце?", 4, 2)}, nil},
		{"multiple Unicode items", []model.Line{v164Text(t, "Солнце ☀?", "Солнце ☀?", 4, 2), v164Text(t, "Étoile?", "Étoile?", 4, 2)}, nil},
		{"punctuation", []model.Line{v164Text(t, "?", "?", 4, 2)}, nil},
		{"NBSP preserved", []model.Line{v164Text(t, "\u00a0", "\u00a0", 4, 2)}, nil},
		{"escaped separator is an item", []model.Line{v164Text(t, `\---`, "---", 4, 2)}, nil},
		{"escaped tag is an item", []model.Line{v164Text(t, `\@answer Да`, "@answer Да", 4, 2)}, nil},
		{"escaped braces are an item", []model.Line{v164Text(t, `\{x\}`, "{x}", 4, 2)}, nil},
		{"blank rows", []model.Line{v164Line(t, "", "blank", 4, 2), v164Line(t, " \t ", "blank", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"blank before and after item", []model.Line{v164Line(t, "", "blank", 4, 2), v164Text(t, "Вопрос?", "Вопрос?", 4, 2),
			v164Line(t, "\t", "blank", 4, 2)}, nil},
		{"answer and hint are not items", []model.Line{v164Tag(t, "answer", "content", "Да", 4, 2),
			v164Tag(t, "hint", "content", "Подумай.", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"other tag is not an item", []model.Line{v164Tag(t, "note", "content", "Подумай.", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"editor is not an item", []model.Line{v164Tag(t, "editor", "content", "Комментарий.", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"block answer and hint are not items", []model.Line{v164Open(t, "answer", "", 4, 2),
			v164Text(t, "Да", "Да", 5, 3), v164Close(t, 5, 3), v164Open(t, "hint", "", 4, 2),
			v164Text(t, "Подумай.", "Подумай.", 8, 3), v164Close(t, 8, 3)}, []v164Expected{{4, 1, 10}}},
		{"nested text is not an item", []model.Line{v164Open(t, "note", "", 4, 2),
			v164Text(t, "Текст.", "Текст.", 5, 3), v164Close(t, 5, 3)}, []v164Expected{{4, 1, 10}}},
		{"item with its answer and hint", []model.Line{v164Text(t, "Солнце — звезда?", "Солнце — звезда?", 4, 2),
			v164Tag(t, "answer", "content", "Да", 4, 2), v164Tag(t, "hint", "content", "Подумай.", 4, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v164Run(t, v164Example(t, tc.upper...), "current", tc.want) })
	}
	t.Run("opening instruction is not an item", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Open(t, "matching", "Выберите ответ.", 3, 1),
			v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2)),
			"current", []v164Expected{{4, 1, 10}})
	})
}

func TestV164Separators(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v164Expected
	}{
		{"none", []model.Line{v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)}, nil},
		{"one", []model.Line{v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"two", []model.Line{v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Separator(t, "---", 4, 2)}, nil},
		{"three", []model.Line{v164Separator(t, "---", 4, 2), v164Separator(t, "---", 4, 2), v164Separator(t, "---", 4, 2)}, nil},
		{"only separator", []model.Line{v164Separator(t, "---", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"spaces and tabs", []model.Line{v164Separator(t, " \t---\t ", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)}, []v164Expected{{4, 1, 10}}},
		{"only nested separator", []model.Line{v164Open(t, "example", "", 4, 2), v164Text(t, "A", "A", 5, 3),
			v164Separator(t, "---", 5, 3), v164Text(t, "B", "B", 5, 3), v164Close(t, 5, 3)}, nil},
		{"direct and nested separators", []model.Line{v164Open(t, "example", "", 4, 2), v164Text(t, "A", "A", 5, 3),
			v164Separator(t, "---", 5, 3), v164Text(t, "B", "B", 5, 3), v164Close(t, 5, 3),
			v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)}, []v164Expected{{4, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v164Run(t, v164Block(t, tc.rows...), "current", tc.want) })
	}
	for _, raw := range []string{"----", "--- текст", "\u00a0---", "---\u00a0", "--", "- - -"} {
		t.Run("not separator/"+raw, func(t *testing.T) {
			v164Run(t, v164Block(t, v164Text(t, raw, raw, 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)), "current", nil)
		})
	}
	t.Run("escaped separator alone", func(t *testing.T) {
		v164Run(t, v164Block(t, v164Text(t, `\---`, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)), "current", nil)
	})
	for _, kind := range []model.LineType{"invalid", "content"} {
		t.Run("raw separator despite unparsed/"+string(kind), func(t *testing.T) {
			line := v164Line(t, " \t---\t ", kind, 4, 2, v164Unparsed(t, "---", 3, 6))
			v164Run(t, v164Block(t, line, v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2)), "current", []v164Expected{{4, 1, 10}})
		})
		t.Run("second raw separator despite unparsed/"+string(kind), func(t *testing.T) {
			line := v164Line(t, "---", kind, 4, 2, v164Unparsed(t, "---", 1, 4))
			v164Run(t, v164Block(t, v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), line), "current", nil)
		})
	}
}

func TestV164Uncertainty(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
	}{
		{"invalid", v164Line(t, "@unknown", "invalid", 4, 2, v164Unparsed(t, "@unknown", 1, 9))},
		{"whole content unparsed", v164Line(t, "{?", "content", 4, 2, v164Unparsed(t, "{?", 1, 3))},
		{"partial content", v164Line(t, "Текст }", "content", 4, 2, v164Element(t, "content", "Текст", "Текст", 1, 6), v164Unparsed(t, "}", 7, 8))},
		{"tag with unparsed tail", v164Line(t, "@answer Да }", "tag", 4, 2, v164Element(t, "tag", "@answer", "answer", 1, 8),
			v164Element(t, "content", "Да", "Да", 9, 11), v164Unparsed(t, "}", 12, 13))},
		// Контракт допускает частично представленную строку без content.
		// Это не подтверждённый пустой элемент и не повод восстанавливать Raw.
		{"content value unavailable", v164Line(t, "Неизвестный текст", "content", 4, 2)},
	} {
		t.Run("upper/"+tc.name, func(t *testing.T) {
			v164Run(t, v164Example(t, tc.line), "current", nil)
		})
		t.Run("lower/"+tc.name, func(t *testing.T) {
			v164Run(t, v164Block(t, v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2),
				v164Text(t, "Нет", "Нет", 4, 2), tc.line), "current", []v164Expected{{4, 1, 10}})
		})
	}
	t.Run("nested uncertainty does not suppress", func(t *testing.T) {
		v164Run(t, v164Example(t, v164Open(t, "note", "", 4, 2),
			v164Line(t, "@unknown", "invalid", 5, 3, v164Unparsed(t, "@unknown", 1, 9)), v164Close(t, 5, 3)),
			"current", []v164Expected{{4, 1, 10}})
	})
	t.Run("opening instruction uncertainty does not suppress", func(t *testing.T) {
		opening := v164Line(t, "@matching } {", "block-start", 3, 1,
			v164Element(t, "tag", "@matching", "matching", 1, 10), v164Unparsed(t, "}", 11, 12), v164Element(t, "block-open", "{", "{", 13, 14))
		v164Run(t, v164Document(t, opening, v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2),
			v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2)), "current", []v164Expected{{4, 1, 10}})
	})
	t.Run("unparsed name is not a saved matching tag", func(t *testing.T) {
		opening := v164Line(t, "@matching {", "block-start", 3, 1,
			v164Unparsed(t, "@matching", 1, 10), v164Element(t, "block-open", "{", "{", 11, 12))
		v164Run(t, v164Document(t, opening, v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2),
			v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2)), "current", nil)
	})
}

func TestV164Contexts(t *testing.T) {
	t.Run("task stage", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Tag(t, "task", "identifier", "practice", 3, 1), v164Tag(t, "step", "title", "Выбор", 4, 2),
			v164Open(t, "matching", "", 5, 3), v164Separator(t, "---", 6, 4), v164Text(t, "Да", "Да", 6, 4),
			v164Text(t, "Нет", "Нет", 6, 4), v164Close(t, 6, 4), v164Tag(t, "endtask", "", "", 4, 2)),
			"current", []v164Expected{{6, 1, 10}})
	})
	t.Run("branch", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Open(t, "variants", "", 3, 1), v164Tag(t, "variant", "name", "A", 4, 2),
			v164Open(t, "matching", "", 5, 3), v164Separator(t, "---", 6, 4), v164Text(t, "Да", "Да", 6, 4),
			v164Text(t, "Нет", "Нет", 6, 4), v164Close(t, 6, 4), v164Tag(t, "variant", "name", "B", 4, 2),
			v164Text(t, "Текст.", "Текст.", 11, 3), v164Close(t, 4, 2)), "current", []v164Expected{{6, 1, 10}})
	})
	t.Run("two independent blocks", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Open(t, "matching", "", 3, 1), v164Separator(t, "---", 4, 2),
			v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2),
			v164Open(t, "matching", "", 3, 1), v164Separator(t, "---", 9, 2),
			v164Text(t, "Да", "Да", 9, 2), v164Text(t, "Нет", "Нет", 9, 2), v164Close(t, 9, 2)),
			"current", []v164Expected{{4, 1, 10}, {9, 1, 10}})
	})
	t.Run("nested matching counted independently", func(t *testing.T) {
		v164Run(t, v164Example(t, v164Open(t, "matching", "", 4, 2), v164Separator(t, "---", 5, 3),
			v164Text(t, "Да", "Да", 5, 3), v164Text(t, "Нет", "Нет", 5, 3), v164Close(t, 5, 3)),
			"current", []v164Expected{{4, 1, 10}, {5, 1, 10}})
	})
	t.Run("uncertainty confined to its own block", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Open(t, "matching", "", 3, 1),
			v164Line(t, "@unknown", "invalid", 4, 2, v164Unparsed(t, "@unknown", 1, 9)),
			v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2),
			v164Open(t, "matching", "", 3, 1), v164Separator(t, "---", 10, 2),
			v164Text(t, "Да", "Да", 10, 2), v164Text(t, "Нет", "Нет", 10, 2), v164Close(t, 10, 2)),
			"current", []v164Expected{{10, 1, 10}})
	})
	t.Run("outside content does not supply upper item", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Text(t, "До блока.", "До блока.", 3, 1), v164Open(t, "matching", "", 3, 1),
			v164Separator(t, "---", 5, 2), v164Text(t, "Да", "Да", 5, 2), v164Text(t, "Нет", "Нет", 5, 2),
			v164Close(t, 5, 2), v164Text(t, "После блока.", "После блока.", 3, 1)), "current", []v164Expected{{5, 1, 10}})
	})
	t.Run("sibling separator does not supply missing separator", func(t *testing.T) {
		v164Run(t, v164Document(t, v164Open(t, "example", "", 3, 1), v164Text(t, "A", "A", 4, 2), v164Separator(t, "---", 4, 2),
			v164Text(t, "B", "B", 4, 2), v164Close(t, 4, 2), v164Open(t, "matching", "", 3, 1),
			v164Text(t, "Да", "Да", 9, 2), v164Text(t, "Нет", "Нет", 9, 2), v164Close(t, 9, 2)), "current", nil)
	})
	for _, name := range []string{"example", "ordering"} {
		t.Run("not matching/"+name, func(t *testing.T) {
			v164Run(t, v164Document(t, v164Open(t, name, "", 3, 1), v164Separator(t, "---", 4, 2),
				v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2)), "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal matching in "+name, func(t *testing.T) {
			v164Run(t, v164Document(t, v164Open(t, name, "", 3, 1), v164Text(t, "@matching {", "@matching {", 4, 2),
				v164Text(t, "---", "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2),
				v164Text(t, `\}`, "}", 4, 2), v164Close(t, 4, 2)), "current", nil)
		})
	}
}

func TestV164DocumentEdgesAndHistory(t *testing.T) {
	t.Run("case and indentation", func(t *testing.T) {
		opening := v164Line(t, " \t@MaTcHiNg {", "block-start", 3, 1,
			v164Element(t, "tag", "@MaTcHiNg", "matching", 3, 12), v164Element(t, "block-open", "{", "{", 13, 14))
		v164Run(t, v164Document(t, opening, v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2),
			v164Text(t, "Нет", "Нет", 4, 2), v164Close(t, 4, 2)), "current", []v164Expected{{4, 3, 12}})
	})
	for _, known := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty known count %t", known), func(t *testing.T) {
			result := v164Empty(t, "1.2")
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v164Run(t, result, "current", nil)
		})
	}
	t.Run("prefix only", func(t *testing.T) { v164Run(t, v164Document(t), "current", nil) })
	t.Run("empty block has no separator", func(t *testing.T) { v164Run(t, v164Block(t), "current", nil) })
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v164Example(t)
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v164Run(t, result, "current", []v164Expected{{4, 1, 10}})
			})
		}
	}
	t.Run("history and element links survive repeated run", func(t *testing.T) {
		result := v164Example(t)
		v164Run(t, result, "first", []v164Expected{{4, 1, 10}})
		v164Run(t, result, "second", []v164Expected{{4, 1, 10}})
	})
}

// v164Registry создаёт реестр из полного штатного каталога регистраций.
func v164Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v164Empty создаёт модель без исходных строк и с явно указанной версией.
func v164Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v164Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v164Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v164Empty(t, "1.2")
	title, id := "Lesson", "v164-test"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v164Tag(t, "dsl-version", "version", "1.2", 0, 0),
		v164Tag(t, "document-id", "identifier", "v164-test", 0, 0),
		v164Line(t, "# Lesson", "heading", 0, 0, v164Element(t, "heading-level", "#", "1", 1, 2), v164Element(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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

// v164Block обрамляет явно заданные строки блоком matching на строке 4.
func v164Block(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v164Open(t, "matching", "", 3, 1)}, rows...)
	body = append(body, v164Close(t, 4, 2))
	return v164Document(t, body...)
}

// v164Example добавляет после верхних строк разделитель и нижние варианты примера.
func v164Example(t *testing.T, upper ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{}, upper...)
	rows = append(rows, v164Separator(t, "---", 4, 2), v164Text(t, "Да", "Да", 4, 2), v164Text(t, "Нет", "Нет", 4, 2))
	return v164Block(t, rows...)
}

// v164Line сохраняет явно заданные данные физической строки и её родителя.
func v164Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v164Element задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v164Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v164Unparsed задаёт фрагмент без значения; историю добавляет v164Document.
func v164Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v164Tag создаёт объявление с явно заданным типом аргумента.
func v164Tag(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v164Element(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v164Element(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v164Line(t, raw, "tag", parent, depth, elements...)
}

// v164Text задаёт обычную строку с независимо указанным семантическим значением.
func v164Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v164Line(t, raw, "content", parent, depth, v164Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v164Open задаёт блочное объявление с необязательной инструкцией.
func v164Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v164Tag(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v164Element(t, "block-open", "{", "{", start, start+1))
	return line
}

// v164Close задаёт отдельную закрывающую скобку с явным родителем.
func v164Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v164Line(t, "}", "block-end", parent, depth, v164Element(t, "block-close", "}", "}", 1, 2))
}

// v164Separator задаёт явно классифицированную строку-разделитель.
func v164Separator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v164Line(t, raw, "separator", parent, depth)
}

// v164Run запускает неизменённую выборку штатного каталога и проверяет новые V164.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v164Run(t *testing.T, result *model.Result, source string, want []v164Expected) {
	t.Helper()
	before := v164Snapshot(t, result)
	checks, err := validator.Prepare(result, v164Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v164Snapshot(t, result) != before {
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
		if entry.DiagnosticCode == "V164" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V164 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 0 ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "В верхнем списке @matching нет элементов." {
			t.Errorf("occurrence = %+v, want range %+v and exact V164 metadata", entry, location)
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
						element.ElementType != "tag" || element.Value == nil || *element.Value != "matching" {
						t.Error("V164 linked to wrong element")
					}
				}
			}
		}
		if links != 1 {
			t.Errorf("V164 links = %d, want 1", links)
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
	if err := json.Unmarshal([]byte(v164Snapshot(t, result)), &actual); err != nil {
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
	if v164Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v164Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v164Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

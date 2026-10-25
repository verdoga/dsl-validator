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

// v154Expected задаёт основной и единственный связанный диапазоны.
// Каждая тройка содержит строку, начало и исключающий конец в Unicode-колонках с 1.
type v154Expected [2][3]int

func TestV154Registration(t *testing.T) {
	definitions, err := v154Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V154" {
			continue
		}
		count++
		if definition.Message != "@multifill без инструкции расположен сразу после однострочного @instr." ||
			definition.Severity != "recommendation" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V154 definitions = %d, want 1", count)
	}
}

func TestV154ReferenceAndExample(t *testing.T) {
	// Дословные эталон и пример дополнены только обязательными @dsl-version
	// и # Lesson по DSL 1.1–1.3, 1.30. Тело начинается с физической строки 3.
	t.Run("reference", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Tag(t, "multifill", "content", "Write about your family.", 2, 1)), "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v154Document(t, v154Instruction(t, 2, 1), v154Field(t, 2, 1))
		v154Run(t, result, "current", []v154Expected{{{4, 1, 11}, {3, 1, 32}}})
		// Общий Document.HasErrors учитывает также V037 об отсутствующем
		// document-id. Проверяем строки рекомендации отдельно от этой ошибки.
		if result.Lines[2].HasErrors || result.Lines[3].HasErrors {
			t.Fatalf("recommendation must not mark instruction or field as erroneous: %+v", result.Diagnostics)
		}
	})
}

func TestV154FieldInstruction(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v154Expected
	}{
		{"own instruction", []model.Line{v154Tag(t, "multifill", "content", "Write.", 2, 1)}, nil},
		{"empty block instruction", []model.Line{v154Open(t, "multifill", 2, 1),
			v154Text(t, "Preloaded text.", "Preloaded text.", 4, 2), v154Close(t, 4, 2)},
			[]v154Expected{{{4, 1, 11}, {3, 1, 32}}}},
		{"multiple preloaded rows", []model.Line{v154Open(t, "multifill", 2, 1),
			v154Text(t, "First.", "First.", 4, 2), v154Text(t, "Second.", "Second.", 4, 2), v154Close(t, 4, 2)},
			[]v154Expected{{{4, 1, 11}, {3, 1, 32}}}},
		{"own block instruction", []model.Line{
			v154Line(t, "@multifill Write. {", "block-start", 2, 1,
				v154TestElement(t, "tag", "@multifill", "multifill", 1, 11),
				v154TestElement(t, "content", "Write.", "Write.", 12, 18), v154TestElement(t, "block-open", "{", "{", 19, 20)),
			v154Text(t, "Text.", "Text.", 4, 2), v154Close(t, 4, 2)}, nil},
		{"separating spaces", []model.Line{v154Line(t, "@multifill   ", "tag", 2, 1,
			v154TestElement(t, "tag", "@multifill", "multifill", 1, 11))}, []v154Expected{{{4, 1, 11}, {3, 1, 32}}}},
		{"tab is not U+0020", []model.Line{v154Line(t, "@multifill\t", "tag", 2, 1,
			v154TestElement(t, "tag", "@multifill", "multifill", 1, 11))}, nil},
		{"NBSP is not U+0020", []model.Line{v154Line(t, "@multifill\u00a0", "invalid", 2, 1,
			v154TestElement(t, "tag", "@multifill", "multifill", 1, 11), v154Unparsed(t, "\u00a0", 11, 12))}, nil},
		{"tab before block opening", []model.Line{v154Line(t, "@multifill\t{", "block-start", 2, 1,
			v154TestElement(t, "tag", "@multifill", "multifill", 1, 11), v154TestElement(t, "block-open", "{", "{", 12, 13)),
			v154Text(t, "Text.", "Text.", 4, 2), v154Close(t, 4, 2)}, nil},
		{"unparsed is not absent instruction", []model.Line{v154Line(t, "@multifill }", "invalid", 2, 1,
			v154TestElement(t, "tag", "@multifill", "multifill", 1, 11), v154Unparsed(t, "}", 12, 13))}, nil},
		{"unparsed before block opening", []model.Line{v154Line(t, "@multifill } {", "block-start", 2, 1,
			v154TestElement(t, "tag", "@multifill", "multifill", 1, 11), v154Unparsed(t, "}", 12, 13),
			v154TestElement(t, "block-open", "{", "{", 14, 15)), v154Text(t, "Text.", "Text.", 4, 2), v154Close(t, 4, 2)}, nil},
		{"no saved multifill tag", []model.Line{v154Line(t, "@multifill", "invalid", 2, 1,
			v154Unparsed(t, "@multifill", 1, 11))}, nil},
		{"escaped multifill is text", []model.Line{v154Text(t, `\@multifill`, "@multifill", 2, 1)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v154Instruction(t, 2, 1)}, tc.body...)
			v154Run(t, v154Document(t, body...), "current", tc.want)
		})
	}
}

func TestV154PrecedingInstruction(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v154Expected
	}{
		{"empty instr", v154Tag(t, "instr", "", "", 2, 1), nil},
		{"only spaces and tabs", v154Line(t, "@instr \t  ", "tag", 2, 1,
			v154TestElement(t, "tag", "@instr", "instr", 1, 7)), nil},
		{"empty content value", v154Line(t, "@instr   ", "tag", 2, 1,
			v154TestElement(t, "tag", "@instr", "instr", 1, 7), v154TestElement(t, "content", "  ", "", 8, 10)), nil},
		{"only unparsed after instr", v154Line(t, "@instr }", "invalid", 2, 1,
			v154TestElement(t, "tag", "@instr", "instr", 1, 7), v154Unparsed(t, "}", 8, 9)), nil},
		{"content plus unparsed", v154Line(t, "@instr Read. }", "invalid", 2, 1,
			v154TestElement(t, "tag", "@instr", "instr", 1, 7), v154TestElement(t, "content", "Read.", "Read.", 8, 13),
			v154Unparsed(t, "}", 14, 15)), []v154Expected{{{4, 1, 11}, {3, 1, 15}}}},
		{"no saved instr tag", v154Line(t, "@instr Read.", "invalid", 2, 1,
			v154Unparsed(t, "@instr Read.", 1, 13)), nil},
		{"escaped instr is text", v154Text(t, `\@instr Read.`, "@instr Read.", 2, 1), nil},
		{"semantic content and raw related range", v154Line(t, `@instr \{x\}`, "tag", 2, 1,
			v154TestElement(t, "tag", "@instr", "instr", 1, 7), v154TestElement(t, "content", `\{x\}`, "{x}", 8, 13)),
			[]v154Expected{{{4, 1, 11}, {3, 1, 13}}}},
		{"NBSP is not normalized away", v154Tag(t, "instr", "content", "\u00a0", 2, 1),
			[]v154Expected{{{4, 1, 11}, {3, 1, 9}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v154Run(t, v154Document(t, tc.line, v154Field(t, 2, 1)), "current", tc.want)
		})
	}
	t.Run("block instr is not single line", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Open(t, "instr", 2, 1), v154Text(t, "Write.", "Write.", 3, 2),
			v154Close(t, 3, 2), v154Field(t, 2, 1)), "current", nil)
	})
	t.Run("case indentation and full related line", func(t *testing.T) {
		instruction := v154Line(t, " \t@InStR Пиши. \t", "tag", 2, 1,
			v154TestElement(t, "tag", "@InStR", "instr", 3, 9), v154TestElement(t, "content", "Пиши.", "Пиши.", 10, 15))
		field := v154Line(t, " \t@MuLtIfIlL  ", "tag", 2, 1, v154TestElement(t, "tag", "@MuLtIfIlL", "multifill", 3, 13))
		v154Run(t, v154Document(t, instruction, field), "current", []v154Expected{{{4, 3, 13}, {3, 1, 17}}})
	})
}

func TestV154Adjacency(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v154Expected
	}{
		{"blank", v154Line(t, " \t", "blank", 2, 1), []v154Expected{{{5, 1, 11}, {3, 1, 32}}}},
		{"editor", v154Tag(t, "editor", "content", "Comment.", 2, 1), []v154Expected{{{5, 1, 11}, {3, 1, 32}}}},
		{"ordinary text", v154Text(t, "Read.", "Read.", 2, 1), nil},
		{"note", v154Tag(t, "note", "content", "Read.", 2, 1), nil},
		{"unknown unparsed line", v154Line(t, "@unknown", "invalid", 2, 1, v154Unparsed(t, "@unknown", 1, 9)), nil},
		{"unparsed editor is not skipped", v154Line(t, "@editor Comment.", "invalid", 2, 1,
			v154Unparsed(t, "@editor Comment.", 1, 17)), nil},
		{"escaped editor", v154Text(t, `\@editor Comment.`, "@editor Comment.", 2, 1), nil},
		{"NBSP line is significant", v154Text(t, "\u00a0", "\u00a0", 2, 1), nil},
		{"nearest empty instr", v154Tag(t, "instr", "", "", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v154Run(t, v154Document(t, v154Instruction(t, 2, 1), tc.line, v154Field(t, 2, 1)), "current", tc.want)
		})
	}
	t.Run("block editor", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Instruction(t, 2, 1), v154Open(t, "editor", 2, 1),
			v154Text(t, "Comment.", "Comment.", 4, 2), v154Close(t, 4, 2), v154Field(t, 2, 1)),
			"current", []v154Expected{{{7, 1, 11}, {3, 1, 32}}})
	})
	t.Run("comment chain", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Instruction(t, 2, 1), v154Tag(t, "editor", "content", "One.", 2, 1),
			v154Open(t, "EDITOR", 2, 1), v154Text(t, "Two.", "Two.", 5, 2), v154Close(t, 5, 2),
			v154Line(t, "", "blank", 2, 1), v154Field(t, 2, 1)), "current", []v154Expected{{{9, 1, 11}, {3, 1, 32}}})
	})
	t.Run("comment content does not create declarations", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Instruction(t, 2, 1), v154Open(t, "editor", 2, 1),
			v154Text(t, "@instr Literal.", "@instr Literal.", 4, 2), v154Text(t, "@multifill", "@multifill", 4, 2),
			v154Close(t, 4, 2), v154Field(t, 2, 1)), "current", []v154Expected{{{8, 1, 11}, {3, 1, 32}}})
	})
	t.Run("other block stops lookup", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Instruction(t, 2, 1), v154Open(t, "note", 2, 1),
			v154Text(t, "Read.", "Read.", 4, 2), v154Close(t, 4, 2), v154Field(t, 2, 1)), "current", nil)
	})
	t.Run("only first consecutive field", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Instruction(t, 2, 1), v154Field(t, 2, 1), v154Field(t, 2, 1)),
			"current", []v154Expected{{{4, 1, 11}, {3, 1, 32}}})
	})
	t.Run("two independent pairs", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Instruction(t, 2, 1), v154Field(t, 2, 1),
			v154Instruction(t, 2, 1), v154Field(t, 2, 1)), "current",
			[]v154Expected{{{4, 1, 11}, {3, 1, 32}}, {{6, 1, 11}, {5, 1, 32}}})
	})
}

func TestV154ContextsAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v154Expected
	}{
		{"task", []model.Line{v154Tag(t, "task", "identifier", "practice", 2, 1),
			v154Instruction(t, 3, 2), v154Field(t, 3, 2), v154Tag(t, "endtask", "", "", 3, 2)},
			[]v154Expected{{{5, 1, 11}, {4, 1, 32}}}},
		{"stage", []model.Line{v154Tag(t, "task", "identifier", "practice", 2, 1),
			v154Tag(t, "step", "title", "Write", 3, 2), v154Instruction(t, 4, 3), v154Field(t, 4, 3),
			v154Tag(t, "endtask", "", "", 3, 2)}, []v154Expected{{{6, 1, 11}, {5, 1, 32}}}},
		{"heading boundary", []model.Line{v154Instruction(t, 2, 1), v154Heading(t, "###", "3", "Next"), v154Field(t, 4, 1)}, nil},
		{"task boundary", []model.Line{v154Instruction(t, 2, 1), v154Tag(t, "task", "identifier", "practice", 2, 1),
			v154Field(t, 4, 2), v154Tag(t, "endtask", "", "", 4, 2)}, nil},
		{"endtask boundary", []model.Line{v154Tag(t, "task", "identifier", "practice", 2, 1),
			v154Instruction(t, 3, 2), v154Tag(t, "endtask", "", "", 3, 2), v154Field(t, 2, 1)}, nil},
		{"header boundary", []model.Line{v154Instruction(t, 2, 1), v154Tag(t, "header", "title", "Next", 2, 1), v154Field(t, 2, 1)}, nil},
		{"step boundary", []model.Line{v154Tag(t, "task", "identifier", "practice", 2, 1), v154Instruction(t, 3, 2),
			v154Tag(t, "step", "title", "Next", 3, 2), v154Field(t, 5, 3), v154Tag(t, "endtask", "", "", 3, 2)}, nil},
		{"newpage boundary", []model.Line{v154Instruction(t, 2, 1), v154Tag(t, "newpage", "", "", 0, 0), v154Field(t, 4, 1)}, nil},
		{"opening block boundary", []model.Line{v154Instruction(t, 2, 1), v154Open(t, "note", 2, 1),
			v154Field(t, 4, 2), v154Close(t, 4, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v154Run(t, v154Document(t, tc.body...), "current", tc.want)
		})
	}
	// Контракт модели допускает логического родителя variant либо фигурного
	// родителя variants. Граница соседних ветвей сохраняется в обоих случаях.
	for _, branchParent := range []bool{false, true} {
		t.Run(fmt.Sprintf("branch parent %t", branchParent), func(t *testing.T) {
			parent, depth := 3, 2
			if branchParent {
				parent, depth = 4, 3
			}
			v154Run(t, v154Document(t, v154Open(t, "variants", 2, 1), v154Tag(t, "variant", "identifier", "a", 3, 2),
				v154Instruction(t, parent, depth), v154Field(t, parent, depth),
				v154Tag(t, "variant", "identifier", "b", 3, 2), v154Text(t, "Read.", "Read.", 3, 2), v154Close(t, 3, 2)),
				"current", []v154Expected{{{6, 1, 11}, {5, 1, 32}}})
			fieldParent := 3
			if branchParent {
				fieldParent = 6
			}
			v154Run(t, v154Document(t, v154Open(t, "variants", 2, 1), v154Tag(t, "variant", "identifier", "a", 3, 2),
				v154Instruction(t, parent, depth), v154Tag(t, "variant", "identifier", "b", 3, 2),
				v154Field(t, fieldParent, depth), v154Close(t, 3, 2)), "current", nil)
		})
	}
	t.Run("editor in branch", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Open(t, "variants", 2, 1), v154Tag(t, "variant", "identifier", "a", 3, 2),
			v154Instruction(t, 4, 3), v154Open(t, "editor", 4, 3), v154Text(t, "Comment.", "Comment.", 6, 4),
			v154Close(t, 6, 4), v154Field(t, 4, 3), v154Tag(t, "variant", "identifier", "b", 3, 2),
			v154Text(t, "Read.", "Read.", 10, 3), v154Close(t, 3, 2)),
			"current", []v154Expected{{{9, 1, 11}, {5, 1, 32}}})
	})
	t.Run("deep stage in branch", func(t *testing.T) {
		v154Run(t, v154Document(t, v154Open(t, "variants", 2, 1), v154Tag(t, "variant", "identifier", "a", 3, 2),
			v154Tag(t, "task", "identifier", "practice", 4, 3), v154Tag(t, "step", "title", "Write", 5, 4),
			v154Instruction(t, 6, 5), v154Field(t, 6, 5), v154Tag(t, "endtask", "", "", 5, 4),
			v154Tag(t, "variant", "identifier", "b", 3, 2), v154Text(t, "Read.", "Read.", 10, 3), v154Close(t, 3, 2)),
			"current", []v154Expected{{{8, 1, 11}, {7, 1, 32}}})
	})
}

func TestV154LiteralAndForbiddenEditor(t *testing.T) {
	for _, container := range []string{"editor", "multifill"} {
		t.Run(container+" literal content", func(t *testing.T) {
			v154Run(t, v154Document(t, v154Open(t, container, 2, 1),
				v154Text(t, "@instr Write about your family.", "@instr Write about your family.", 3, 2),
				v154Text(t, "@multifill", "@multifill", 3, 2), v154Close(t, 3, 2)), "current", nil)
		})
	}
	// DSL 1.24 сохраняет реальные запрещённые объявления, кроме литеральных
	// контейнеров. V154 применима на любой глубине; ошибку размещения проверяет
	// другая диагностика. Editor в fragment не разрешён (DSL 2.8.9).
	for _, tc := range []struct {
		name   string
		middle []model.Line
		want   []v154Expected
	}{
		{"no editor", nil, []v154Expected{{{5, 1, 11}, {4, 1, 32}}}},
		{"forbidden inline editor", []model.Line{v154Tag(t, "editor", "content", "Comment.", 3, 2)}, nil},
		{"forbidden block editor", []model.Line{v154Open(t, "editor", 3, 2),
			v154Text(t, "Comment.", "Comment.", 5, 3), v154Close(t, 5, 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fragment := v154Line(t, "@fragment f {", "block-start", 2, 1,
				v154TestElement(t, "tag", "@fragment", "fragment", 1, 10), v154TestElement(t, "identifier", "f", "f", 11, 12),
				v154TestElement(t, "block-open", "{", "{", 13, 14))
			body := append([]model.Line{fragment, v154Instruction(t, 3, 2)}, tc.middle...)
			body = append(body, v154Field(t, 3, 2), v154Close(t, 3, 2), v154Tag(t, "include", "identifier", "f", 2, 1))
			v154Run(t, v154Document(t, body...), "current", tc.want)
		})
	}
}

func TestV154DocumentEdgesAndHistory(t *testing.T) {
	t.Run("empty unknown line count", func(t *testing.T) {
		v154Run(t, v154Empty(t, "1.2"), "current", nil)
	})
	t.Run("empty known line count", func(t *testing.T) {
		result := v154Empty(t, "1.2")
		count := 0
		result.Document.LineCount = &count
		v154Run(t, result, "current", nil)
	})
	t.Run("prefix only", func(t *testing.T) { v154Run(t, v154Document(t), "current", nil) })
	t.Run("first field", func(t *testing.T) { v154Run(t, v154Document(t, v154Field(t, 2, 1)), "current", nil) })
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v154Document(t, v154Instruction(t, 2, 1), v154Field(t, 2, 1))
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v154Run(t, result, "current", []v154Expected{{{4, 1, 11}, {3, 1, 32}}})
			})
		}
	}
	t.Run("history survives repeated run", func(t *testing.T) {
		result := v154Document(t, v154Instruction(t, 2, 1), v154Field(t, 2, 1))
		v154Run(t, result, "first", []v154Expected{{{4, 1, 11}, {3, 1, 32}}})
		v154Run(t, result, "second", []v154Expected{{{4, 1, 11}, {3, 1, 32}}})
	})
	for _, version := range []string{"1.1", "1.3", "1.21"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v154Empty(t, version)
			before := v154Snapshot(t, result)
			checks, err := validator.Prepare(result, v154Registry(t))
			if err == nil || len(checks) != 0 || v154Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

// v154Registry использует полный штатный каталог регистраций.
func v154Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v154Empty создаёт модель без доступных исходных строк.
func v154Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v154Document добавляет обязательный префикс и согласованную историю unparsed.
// Типы, родители и элементы тела заданы тестом, а не восстановлены из Raw.
func v154Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v154Empty(t, "1.2")
	title := "Lesson"
	result.Document.Metadata.Title = &title
	result.Lines = append([]model.Line{v154Tag(t, "dsl-version", "version", "1.2", 0, 0),
		v154Heading(t, "#", "1", "Lesson")}, body...)
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
			id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v154Line сохраняет явно заданную структуру физической строки.
func v154Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v154TestElement сохраняет независимые Raw и Value с Unicode-диапазоном [start,end).
func v154TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v154Unparsed создаёт участок без текстового значения; историю добавляет v154Document.
func v154Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v154Tag собирает известное объявление с одним явно типизированным аргументом.
func v154Tag(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v154TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v154TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v154Line(t, raw, "tag", parent, depth, elements...)
}

// v154Instruction сохраняет дословную инструкцию из примера.
func v154Instruction(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v154Tag(t, "instr", "content", "Write about your family.", parent, depth)
}

// v154Field задаёт однострочный multifill без инструкции.
func v154Field(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v154Tag(t, "multifill", "", "", parent, depth)
}

// v154Text задаёт обычное содержимое без повторного разбора его значения.
func v154Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v154Line(t, raw, "content", parent, depth, v154TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v154Open задаёт объявление без параметров с открытием блока.
func v154Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v154Line(t, "@"+name+" {", "block-start", parent, depth,
		v154TestElement(t, "tag", "@"+name, strings.ToLower(name), 1, len(name)+2),
		v154TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v154Close задаёт закрытие явно указанного фигурного родителя.
func v154Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v154Line(t, "}", "block-end", parent, depth, v154TestElement(t, "block-close", "}", "}", 1, 2))
}

// v154Heading задаёт корневой заголовок с явно указанным уровнем.
func v154Heading(t *testing.T, marker, level, title string) model.Line {
	t.Helper()
	return v154Line(t, marker+" "+title, "heading", 0, 0,
		v154TestElement(t, "heading-level", marker, level, 1, len(marker)+1),
		v154TestElement(t, "title", title, title, len(marker)+2, len(marker)+2+utf8.RuneCountInString(title)))
}

// v154Run выполняет полный штатный конвейер и проверяет только текущие V154.
func v154Run(t *testing.T, result *model.Result, source string, want []v154Expected) {
	t.Helper()
	before := v154Snapshot(t, result)
	checks, err := validator.Prepare(result, v154Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v154Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	targetIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("new diagnostic source = %q, want %q", entry.Source, source)
		}
		if entry.DiagnosticCode == "V154" {
			got = append(got, entry)
			targetIDs[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V154 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		main := model.Location{Start: model.Position{Line: expected[0][0], Column: expected[0][1]}, End: model.Position{Line: expected[0][0], Column: expected[0][2]}}
		related := model.Location{Start: model.Position{Line: expected[1][0], Column: expected[1][1]}, End: model.Position{Line: expected[1][0], Column: expected[1][2]}}
		if entry.Location == nil || *entry.Location != main || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v / %+v", i, entry.Location, entry.RelatedLocations, main, related)
		}
		if entry.ID == "" || entry.Source != source || entry.DiagnosticCode != "V154" || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "@multifill без инструкции расположен сразу после однострочного @instr.\nконкретизация: Если инструкция относится только к этому полю, перенесите её в строку объявления @multifill как INSTRUCTION. Если она общая для нескольких действий, оставьте отдельный @instr." {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v154Snapshot(t, result)), &actual); err != nil {
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
			for _, id := range element.ErrorIDs {
				if targetIDs[id] {
					t.Errorf("ScopeLine V154 linked to element at %d:%d", i+1, j)
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v154Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v154Snapshot сохраняет модель для проверки исходных данных и истории.
func v154Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

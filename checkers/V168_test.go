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

// v168Expected задаёт строку, начало и исключающий конец диапазона в Unicode.
type v168Expected [3]int

func TestV168Registration(t *testing.T) {
	definitions, err := v168Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V168" {
			continue
		}
		count++
		if definition.Message != "Подсказка @hint не следует непосредственно за ответом элемента верхнего списка." ||
			definition.Severity != "error" || definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V168 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v168Empty(t, version)
			before := v168Snapshot(t, result)
			checks, err := validator.Prepare(result, v168Registry(t))
			if err == nil || checks != nil || v168Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV168ReferenceAndExample(t *testing.T) {
	// Дословные фрагменты дополнены @dsl-version, @document-id и # Lesson
	// по DSL 1.1, 1.30, 1.31 и требованию V037. Matching начинается на строке 4.
	t.Run("reference", func(t *testing.T) {
		v168Run(t, v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2)), "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v168Run(t, v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2),
			v168TestTag(t, "hint", "content", "Подумайте об источнике света.", 4, 2)), "current", []v168Expected{{8, 1, 6}})
	})
}

func TestV168TwoPreviousConstructions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v168Expected
	}{
		{"hint first", []model.Line{v168Hint(t, 4, 2), v168Item(t, 4, 2), v168Answer(t, 4, 2)}, []v168Expected{{5, 1, 6}}},
		{"item without answer", []model.Line{v168Item(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{6, 1, 6}}},
		{"answer without item", []model.Line{v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{6, 1, 6}}},
		{"blank lines", []model.Line{v168Item(t, 4, 2), v168Line(t, " \t", "blank", 4, 2),
			v168Answer(t, 4, 2), v168Line(t, "", "blank", 4, 2), v168Hint(t, 4, 2)}, nil},
		{"text after answer", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168Text(t, "Другой вопрос?", "Другой вопрос?", 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"tag between item and answer", []model.Line{v168Item(t, 4, 2), v168TestTag(t, "note", "content", "Текст.", 4, 2),
			v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"two answers", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"three hints", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2),
			v168Hint(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}, {9, 1, 6}}},
		{"inline editor", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168TestTag(t, "editor", "content", "Комментарий.", 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"editor before answer", []model.Line{v168Item(t, 4, 2), v168TestTag(t, "editor", "content", "Комментарий.", 4, 2),
			v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"invalid line intervenes", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168Line(t, "@unknown", "invalid", 4, 2, v168Unparsed(t, "@unknown", 1, 9)), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"invalid line is not an upper item", []model.Line{v168Line(t, "@unknown", "invalid", 4, 2,
			v168Unparsed(t, "@unknown", 1, 9)), v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{7, 1, 6}}},
		{"two text lines after answer", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168Text(t, "A", "A", 4, 2), v168Text(t, "B", "B", 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{9, 1, 6}}},
		{"second upper item is enough", []model.Line{v168Text(t, "Первый вопрос?", "Первый вопрос?", 4, 2), v168Answer(t, 4, 2),
			v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, nil},
		{"escaped answer is upper text", []model.Line{v168Text(t, `\@answer Да`, "@answer Да", 4, 2),
			v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, nil},
		{"NBSP is not blank", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168Text(t, "\u00a0", "\u00a0", 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{8, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v168Run(t, v168Example(t, tc.upper...), "current", tc.want) })
	}
}

func TestV168Blocks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		upper []model.Line
		want  []v168Expected
	}{
		{"completed block answer", []model.Line{v168Item(t, 4, 2), v168Open(t, "answer", "", 4, 2),
			v168Text(t, "Да", "Да", 6, 3), v168Text(t, "Нет", "Нет", 6, 3), v168Close(t, 6, 3), v168Hint(t, 4, 2)}, nil},
		{"answer body does not supply item", []model.Line{v168Open(t, "answer", "", 4, 2), v168Text(t, "Да", "Да", 5, 3),
			v168Text(t, "Нет", "Нет", 5, 3), v168Close(t, 5, 3), v168Hint(t, 4, 2)}, []v168Expected{{9, 1, 6}}},
		{"block hint", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Open(t, "hint", "", 4, 2),
			v168Text(t, "Подумайте.", "Подумайте.", 7, 3), v168Close(t, 7, 3)}, nil},
		{"repeated after block hint", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Open(t, "hint", "", 4, 2),
			v168Text(t, "Подумайте.", "Подумайте.", 7, 3), v168Close(t, 7, 3), v168Hint(t, 4, 2)}, []v168Expected{{10, 1, 6}}},
		{"repeated block hint", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2),
			v168Open(t, "hint", "", 4, 2), v168Text(t, "Подумайте.", "Подумайте.", 8, 3), v168Close(t, 8, 3)}, []v168Expected{{8, 1, 6}}},
		{"blank after closing answer", []model.Line{v168Item(t, 4, 2), v168Open(t, "answer", "", 4, 2),
			v168Line(t, "", "blank", 6, 3), v168Text(t, "Да", "Да", 6, 3), v168Line(t, " \t", "blank", 6, 3),
			v168Close(t, 6, 3), v168Line(t, "", "blank", 4, 2), v168Hint(t, 4, 2)}, nil},
		{"block editor intervenes", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Open(t, "editor", "", 4, 2),
			v168Text(t, "Комментарий.", "Комментарий.", 7, 3), v168Close(t, 7, 3), v168Hint(t, 4, 2)}, []v168Expected{{10, 1, 6}}},
		{"other block intervenes", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Open(t, "note", "", 4, 2),
			v168Text(t, "Текст.", "Текст.", 7, 3), v168Close(t, 7, 3), v168Hint(t, 4, 2)}, []v168Expected{{10, 1, 6}}},
		{"other block before answer", []model.Line{v168Open(t, "note", "", 4, 2), v168Item(t, 5, 3), v168Close(t, 5, 3),
			v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{9, 1, 6}}},
		{"hint inside answer excluded and body not inspected", []model.Line{v168Item(t, 4, 2), v168Open(t, "answer", "", 4, 2),
			v168Text(t, "Да", "Да", 6, 3), v168Hint(t, 6, 3), v168Close(t, 6, 3), v168Hint(t, 4, 2)}, nil},
		{"hint inside hint excluded", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Open(t, "hint", "", 4, 2),
			v168Hint(t, 7, 3), v168Close(t, 7, 3)}, nil},
		{"hint inside note excluded", []model.Line{v168Open(t, "note", "", 4, 2), v168Hint(t, 5, 3), v168Close(t, 5, 3),
			v168Item(t, 4, 2), v168Answer(t, 4, 2)}, nil},
		{"answer before another answer is not item", []model.Line{v168Open(t, "answer", "", 4, 2), v168Text(t, "Да", "Да", 5, 3),
			v168Close(t, 5, 3), v168Answer(t, 4, 2), v168Hint(t, 4, 2)}, []v168Expected{{9, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v168Run(t, v168Example(t, tc.upper...), "current", tc.want) })
	}
}

func TestV168SavedDeclarations(t *testing.T) {
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run("answer with tail/"+string(kind), func(t *testing.T) {
			answer := v168Line(t, "@answer Да }", kind, 4, 2, v168TestElement(t, "tag", "@answer", "answer", 1, 8),
				v168TestElement(t, "content", "Да", "Да", 9, 11), v168Unparsed(t, "}", 12, 13))
			v168Run(t, v168Example(t, v168Item(t, 4, 2), answer, v168Hint(t, 4, 2)), "current", nil)
		})
		t.Run("hint with tail/"+string(kind), func(t *testing.T) {
			hint := v168Line(t, "@hint Текст }", kind, 4, 2, v168TestElement(t, "tag", "@hint", "hint", 1, 6),
				v168TestElement(t, "content", "Текст", "Текст", 7, 12), v168Unparsed(t, "}", 13, 14))
			v168Run(t, v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2), hint),
				"current", []v168Expected{{8, 1, 6}})
		})
	}
	t.Run("unparsed hint not recovered", func(t *testing.T) {
		v168Run(t, v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168Line(t, "@hint Текст", "invalid", 4, 2, v168Unparsed(t, "@hint Текст", 1, 12))), "current", nil)
	})
	t.Run("unparsed answer not recovered", func(t *testing.T) {
		v168Run(t, v168Example(t, v168Item(t, 4, 2), v168Line(t, "@answer Да", "invalid", 4, 2,
			v168Unparsed(t, "@answer Да", 1, 11)), v168Hint(t, 4, 2)), "current", []v168Expected{{7, 1, 6}})
	})
	for _, tc := range []struct{ raw, value string }{
		{`\@hint Текст`, "@hint Текст"}, {"Текст @hint дальше.", "Текст @hint дальше."},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v168Run(t, v168Example(t, v168Text(t, tc.raw, tc.value, 4, 2), v168Answer(t, 4, 2)), "current", nil)
		})
	}
}

func TestV168Separators(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v168Expected
	}{
		{"none", []model.Line{v168Hint(t, 4, 2), v168Item(t, 4, 2), v168Answer(t, 4, 2)}, nil},
		{"two", []model.Line{v168Hint(t, 4, 2), v168Separator(t, "---", 4, 2), v168Text(t, "Да", "Да", 4, 2),
			v168Separator(t, "---", 4, 2), v168Text(t, "Нет", "Нет", 4, 2)}, nil},
		{"three", []model.Line{v168Hint(t, 4, 2), v168Separator(t, "---", 4, 2), v168Separator(t, "---", 4, 2),
			v168Separator(t, "---", 4, 2)}, nil},
		{"after separator", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Separator(t, "---", 4, 2),
			v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2), v168Hint(t, 4, 2)}, nil},
		{"space and tab edges", []model.Line{v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2), v168Hint(t, 4, 2),
			v168Separator(t, " \t---\t ", 4, 2), v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2)}, []v168Expected{{8, 1, 6}}},
		{"only nested separator", []model.Line{v168Hint(t, 4, 2), v168Open(t, "example", "", 4, 2),
			v168Text(t, "A", "A", 6, 3), v168Separator(t, "---", 6, 3), v168Text(t, "B", "B", 6, 3), v168Close(t, 6, 3)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v168Run(t, v168Block(t, tc.rows...), "current", tc.want) })
	}
	t.Run("nested separator does not duplicate outer separator", func(t *testing.T) {
		v168Run(t, v168Example(t, v168Hint(t, 4, 2), v168Open(t, "example", "", 4, 2),
			v168Text(t, "A", "A", 6, 3), v168Separator(t, "---", 6, 3), v168Text(t, "B", "B", 6, 3), v168Close(t, 6, 3)),
			"current", []v168Expected{{5, 1, 6}})
	})
	t.Run("escaped separator is ordinary upper item", func(t *testing.T) {
		v168Run(t, v168Example(t, v168Text(t, `\---`, "---", 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2)), "current", nil)
	})
	for _, tc := range []struct{ raw, value string }{
		{`\---`, "---"}, {"\u00a0---", "\u00a0---"}, {"---\u00a0", "---\u00a0"}, {"----", "----"},
	} {
		t.Run("not a separator/"+tc.raw, func(t *testing.T) {
			v168Run(t, v168Block(t, v168Hint(t, 4, 2), v168Text(t, tc.raw, tc.value, 4, 2),
				v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2)), "current", nil)
		})
	}
}

func TestV168Contexts(t *testing.T) {
	t.Run("task stage", func(t *testing.T) {
		v168Run(t, v168Document(t, v168TestTag(t, "task", "identifier", "practice", 3, 1), v168TestTag(t, "step", "title", "Выбор", 4, 2),
			v168Open(t, "matching", "", 5, 3), v168Item(t, 6, 4), v168Answer(t, 6, 4), v168Hint(t, 6, 4), v168Hint(t, 6, 4),
			v168Separator(t, "---", 6, 4), v168Text(t, "Да", "Да", 6, 4), v168Text(t, "Нет", "Нет", 6, 4),
			v168Close(t, 6, 4), v168TestTag(t, "endtask", "", "", 4, 2)), "current", []v168Expected{{10, 1, 6}})
	})
	t.Run("branch", func(t *testing.T) {
		v168Run(t, v168Document(t, v168Open(t, "variants", "", 3, 1), v168TestTag(t, "variant", "name", "A", 4, 2),
			v168Open(t, "matching", "", 5, 3), v168Item(t, 6, 4), v168Answer(t, 6, 4), v168Hint(t, 6, 4), v168Hint(t, 6, 4),
			v168Separator(t, "---", 6, 4), v168Text(t, "Да", "Да", 6, 4), v168Text(t, "Нет", "Нет", 6, 4), v168Close(t, 6, 4),
			v168TestTag(t, "variant", "name", "B", 4, 2), v168Text(t, "Текст.", "Текст.", 15, 3), v168Close(t, 4, 2)),
			"current", []v168Expected{{10, 1, 6}})
	})
	t.Run("new block cannot borrow answer", func(t *testing.T) {
		v168Run(t, v168Document(t, v168TestTag(t, "question", "content", "Солнце — звезда?", 3, 1),
			v168Open(t, "answer", "", 3, 1), v168Text(t, "Да", "Да", 5, 2), v168Text(t, "Нет", "Нет", 5, 2), v168Close(t, 5, 2),
			v168Open(t, "matching", "", 3, 1), v168Hint(t, 9, 2), v168Item(t, 9, 2), v168Answer(t, 9, 2),
			v168Separator(t, "---", 9, 2), v168Text(t, "Да", "Да", 9, 2), v168Text(t, "Нет", "Нет", 9, 2), v168Close(t, 9, 2)),
			"current", []v168Expected{{10, 1, 6}})
	})
	t.Run("two independent matching blocks", func(t *testing.T) {
		v168Run(t, v168Document(t, v168Open(t, "matching", "", 3, 1), v168Hint(t, 4, 2), v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168Separator(t, "---", 4, 2), v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2), v168Close(t, 4, 2),
			v168Open(t, "matching", "", 3, 1), v168Hint(t, 12, 2), v168Item(t, 12, 2), v168Answer(t, 12, 2),
			v168Separator(t, "---", 12, 2), v168Text(t, "Да", "Да", 12, 2), v168Text(t, "Нет", "Нет", 12, 2), v168Close(t, 12, 2)),
			"current", []v168Expected{{5, 1, 6}, {13, 1, 6}})
	})
	t.Run("include does not expand adjacency", func(t *testing.T) {
		fragment := v168Line(t, "@fragment f {", "block-start", 3, 1, v168TestElement(t, "tag", "@fragment", "fragment", 1, 10),
			v168TestElement(t, "identifier", "f", "f", 11, 12), v168TestElement(t, "block-open", "{", "{", 13, 14))
		v168Run(t, v168Document(t, v168Open(t, "matching", "", 3, 1), v168Item(t, 4, 2), v168Answer(t, 4, 2),
			v168TestTag(t, "include", "identifier", "f", 4, 2), v168Hint(t, 4, 2), v168Separator(t, "---", 4, 2),
			v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2), v168Close(t, 4, 2),
			fragment, v168Text(t, "Текст.", "Текст.", 13, 2), v168Close(t, 13, 2)), "current", []v168Expected{{8, 1, 6}})
	})
	for _, name := range []string{"example", "ordering"} {
		t.Run("not matching/"+name, func(t *testing.T) {
			v168Run(t, v168Document(t, v168Open(t, name, "", 3, 1), v168Item(t, 4, 2), v168Answer(t, 4, 2),
				v168Hint(t, 4, 2), v168Hint(t, 4, 2), v168Separator(t, "---", 4, 2),
				v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2), v168Close(t, 4, 2)), "current", nil)
		})
	}
	t.Run("outside matching", func(t *testing.T) { v168Run(t, v168Document(t, v168Hint(t, 3, 1)), "current", nil) })
}

func TestV168DocumentEdgesAndHistory(t *testing.T) {
	t.Run("case and indentation", func(t *testing.T) {
		second := v168Line(t, " \t@HiNt Подумайте.", "tag", 4, 2, v168TestElement(t, "tag", "@HiNt", "hint", 3, 8),
			v168TestElement(t, "content", "Подумайте.", "Подумайте.", 9, 19))
		v168Run(t, v168Document(t, v168Open(t, "MaTcHiNg", "", 3, 1), v168Item(t, 4, 2),
			v168TestTag(t, "AnSwEr", "content", "Да", 4, 2), v168TestTag(t, "HINT", "content", "Подумайте.", 4, 2), second,
			v168Separator(t, "---", 4, 2), v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2), v168Close(t, 4, 2)),
			"current", []v168Expected{{8, 3, 8}})
	})
	for _, known := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty known count %t", known), func(t *testing.T) {
			result := v168Empty(t, "1.2")
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v168Run(t, result, "current", nil)
		})
	}
	t.Run("prefix only", func(t *testing.T) { v168Run(t, v168Document(t), "current", nil) })
	t.Run("no hints", func(t *testing.T) { v168Run(t, v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2)), "current", nil) })
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2), v168Hint(t, 4, 2))
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v168Run(t, result, "current", []v168Expected{{8, 1, 6}})
			})
		}
	}
	t.Run("history and element links survive repeated run", func(t *testing.T) {
		result := v168Example(t, v168Item(t, 4, 2), v168Answer(t, 4, 2), v168Hint(t, 4, 2), v168Hint(t, 4, 2))
		v168Run(t, result, "first", []v168Expected{{8, 1, 6}})
		v168Run(t, result, "second", []v168Expected{{8, 1, 6}})
	})
}

// v168Item задаёт дословную строку верхнего элемента из эталона.
func v168Item(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v168Text(t, "Солнце — звезда?", "Солнце — звезда?", parent, depth)
}

// v168Answer задаёт дословное однострочное объявление ответа из эталона.
func v168Answer(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v168TestTag(t, "answer", "content", "Да", parent, depth)
}

// v168Hint задаёт дословную первую подсказку из эталона и примера.
func v168Hint(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v168TestTag(t, "hint", "content", "Вспомните определение звезды.", parent, depth)
}

// v168Registry создаёт реестр из полного штатного каталога регистраций.
func v168Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v168Empty создаёт модель без исходных строк и с явно указанной версией.
func v168Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v168Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v168Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v168Empty(t, "1.2")
	title, id := "Lesson", "v168-test"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v168TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v168TestTag(t, "document-id", "identifier", "v168-test", 0, 0),
		v168Line(t, "# Lesson", "heading", 0, 0, v168TestElement(t, "heading-level", "#", "1", 1, 2), v168TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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

// v168Block обрамляет явно заданные строки блоком matching на строке 4.
func v168Block(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v168Open(t, "matching", "", 3, 1)}, rows...)
	body = append(body, v168Close(t, 4, 2))
	return v168Document(t, body...)
}

// v168Example добавляет после верхних строк разделитель и нижние варианты примера.
func v168Example(t *testing.T, upper ...model.Line) *model.Result {
	t.Helper()
	rows := append([]model.Line{}, upper...)
	rows = append(rows, v168Separator(t, "---", 4, 2), v168Text(t, "Да", "Да", 4, 2), v168Text(t, "Нет", "Нет", 4, 2))
	return v168Block(t, rows...)
}

// v168Line сохраняет явно заданные данные физической строки и её родителя.
func v168Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v168TestElement задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v168TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v168Unparsed задаёт фрагмент без значения; историю добавляет v168Document.
func v168Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v168TestTag создаёт объявление с явно заданным типом аргумента.
func v168TestTag(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v168TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v168TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v168Line(t, raw, "tag", parent, depth, elements...)
}

// v168Text задаёт обычную строку с независимо указанным семантическим значением.
func v168Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v168Line(t, raw, "content", parent, depth, v168TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v168Open задаёт блочное объявление с необязательной инструкцией.
func v168Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v168TestTag(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v168TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v168Close задаёт отдельную закрывающую скобку с явным родителем.
func v168Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v168Line(t, "}", "block-end", parent, depth, v168TestElement(t, "block-close", "}", "}", 1, 2))
}

// v168Separator задаёт явно классифицированную строку-разделитель.
func v168Separator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v168Line(t, raw, "separator", parent, depth)
}

// v168Run запускает неизменённую выборку штатного каталога и проверяет новые V168.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v168Run(t *testing.T, result *model.Result, source string, want []v168Expected) {
	t.Helper()
	before := v168Snapshot(t, result)
	checks, err := validator.Prepare(result, v168Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v168Snapshot(t, result) != before {
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
		if entry.DiagnosticCode == "V168" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V168 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 0 ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "Подсказка @hint не следует непосредственно за ответом элемента верхнего списка." {
			t.Errorf("occurrence = %+v, want range %+v and exact V168 metadata", entry, location)
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
						element.ElementType != "tag" || element.Value == nil || *element.Value != "hint" {
						t.Error("V168 linked to wrong element")
					}
				}
			}
		}
		if links != 1 {
			t.Errorf("V168 links = %d, want 1", links)
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
	if err := json.Unmarshal([]byte(v168Snapshot(t, result)), &actual); err != nil {
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
	if v168Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v168Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v168Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

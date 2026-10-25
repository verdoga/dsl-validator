package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v167Expected задаёт исходный диапазон тега [start,end) и конкретизацию.
type v167Expected struct {
	line, start, end int
	detail           string
}

func TestV167ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		upper        []model.Line
		want         []v167Expected
	}{
		{"reference", "@matching {\nПодходящий цвет\n@answer {\nКрасный\nСиний\n}\n---\nКрасный\nСиний\n}",
			[]model.Line{v167TestText(t, "Подходящий цвет", "Подходящий цвет", 3, 2), v167TestOpen(t, "answer", 3, 2),
				v167TestText(t, "Красный", "Красный", 5, 3), v167TestText(t, "Синий", "Синий", 5, 3), v167TestClose(t, 5, 3)}, nil},
		{"example", "@matching {\nПодходящий цвет\n@answer Красный\n@answer Синий\n---\nКрасный\nСиний\n}",
			[]model.Line{v167TestText(t, "Подходящий цвет", "Подходящий цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2),
				v167TestTag(t, "answer", "content", "Синий", 3, 2)}, []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v167TestMatching(t, tc.upper...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			v167TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV167Adjacency(t *testing.T) {
	const repeat = "Повторный @answer для элемента верхнего списка."
	// Все строки таблицы — непосредственное содержимое matching на строке 3,
	// кроме явно заданных тел вложенных блоков. Родители не выводятся из Raw.
	for _, tc := range []struct {
		name      string
		upper     []model.Line
		want      []v167Expected
		errorLine int
	}{
		{"one inline answer", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, nil, 0},
		{"blank lines", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestBlank(t, "", 3, 2), v167TestBlank(t, " \t", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, nil, 0},
		{"answer first", []model.Line{v167TestTag(t, "answer", "content", "Синий", 3, 2), v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, []v167Expected{{4, 1, 8, ""}}, 0},
		{"three answers", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, []v167Expected{{6, 1, 8, repeat}, {7, 1, 8, repeat}}, 0},
		{"repeat across blank", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestBlank(t, "\t", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2)}, []v167Expected{{7, 1, 8, repeat}}, 0},
		{"repeat after hint", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "hint", "content", "Подумайте.", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2)}, []v167Expected{{7, 1, 8, repeat}}, 0},
		{"third construct is not inspected", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "hint", "content", "Подумайте.", 3, 2), v167TestTag(t, "hint", "content", "Ещё раз.", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2)}, []v167Expected{{8, 1, 8, ""}}, 7},
		{"new list item", []model.Line{v167TestText(t, "Первый цвет", "Первый цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestText(t, "Другой цвет", "Другой цвет", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2)}, nil, 0},
		{"hint interrupts item", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "hint", "content", "Подумайте.", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, []v167Expected{{6, 1, 8, ""}}, 5},
		{"inline editor interrupts item", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "editor", "content", "Комментарий", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, []v167Expected{{6, 1, 8, ""}}, 5},
		{"block editor interrupts item", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestOpen(t, "editor", 3, 2), v167TestText(t, "@answer Синий", "@answer Синий", 5, 3), v167TestClose(t, 5, 3), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, []v167Expected{{8, 1, 8, ""}}, 5},
		{"foreign block interrupts item", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestOpen(t, "note", 3, 2), v167TestText(t, "Комментарий", "Комментарий", 5, 3), v167TestClose(t, 5, 3), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, []v167Expected{{8, 1, 8, ""}}, 5},
		{"completed answer is one construct", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestOpen(t, "answer", 3, 2), v167TestText(t, "Красный", "Красный", 5, 3), v167TestText(t, "Синий", "Синий", 5, 3), v167TestClose(t, 5, 3), v167TestTag(t, "answer", "content", "Синий", 3, 2)}, []v167Expected{{9, 1, 8, repeat}}, 0},
		{"completed hint is one construct", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestOpen(t, "hint", 3, 2), v167TestText(t, "Подумайте.", "Подумайте.", 6, 3), v167TestClose(t, 6, 3), v167TestTag(t, "answer", "content", "Синий", 3, 2)}, []v167Expected{{9, 1, 8, repeat}}, 0},
		{"empty answer has its own check", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "", "", 3, 2)}, nil, 5},
		{"empty repeated answer still counts", []model.Line{v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "", "", 3, 2)}, []v167Expected{{6, 1, 8, repeat}}, 6},
		{"escaped separator is ordinary item", []model.Line{v167TestText(t, `\---`, "---", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, nil, 0},
		{"escaped tag is ordinary item", []model.Line{v167TestText(t, `\@answer текст`, "@answer текст", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, nil, 0},
		{"mention is ordinary item", []model.Line{v167TestText(t, "Слово @answer", "Слово @answer", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)}, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v167TestMatching(t, tc.upper...)
			if tc.errorLine != 0 {
				v167TestError(t, result, tc.errorLine, -1, "Нарушены правила содержимого или размещения тега.")
			}
			v167TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV167Separators(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		kind             model.LineType
		want             []v167Expected
	}{
		{"exact", "---", "---", "separator", []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}}},
		{"ASCII edges", " \t---\t ", "---", "separator", []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}}},
		{"escaped", `\---`, "---", "content", nil},
		{"NBSP edges", "\u00a0---\u00a0", "\u00a0---\u00a0", "content", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v167TestExample(t)
			line := v167TestLine(t, tc.raw, tc.kind, 3, 2)
			if tc.kind == "content" {
				line = v167TestText(t, tc.raw, tc.value, 3, 2)
			}
			line.Number = 7
			result.Lines[6] = line
			v167TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("absent", func(t *testing.T) {
		v167TestRun(t, v167TestDocument(t, v167TestOpen(t, "matching", 2, 1), v167TestText(t, "Цвет", "Цвет", 3, 2),
			v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2),
			v167TestText(t, "Красный", "Красный", 3, 2), v167TestText(t, "Синий", "Синий", 3, 2), v167TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("two own separators", func(t *testing.T) {
		result := v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2),
			v167TestTag(t, "answer", "content", "Синий", 3, 2), v167TestLine(t, "---", "separator", 3, 2))
		v167TestError(t, result, 8, -1, "В matching допустим только один разделитель.")
		v167TestRun(t, result, "current", nil)
	})
	for _, own := range []bool{false, true} {
		t.Run("nested separator own="+strconv.FormatBool(own), func(t *testing.T) {
			body := []model.Line{v167TestOpen(t, "matching", 2, 1), v167TestText(t, "Цвет", "Цвет", 3, 2),
				v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2),
				v167TestOpen(t, "hint", 3, 2), v167TestText(t, "---", "---", 7, 3), v167TestClose(t, 7, 3)}
			var want []v167Expected
			if own {
				body = append(body, v167TestLine(t, "---", "separator", 3, 2))
				want = []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}}
			}
			body = append(body, v167TestText(t, "Красный", "Красный", 3, 2), v167TestText(t, "Синий", "Синий", 3, 2), v167TestClose(t, 3, 2))
			v167TestRun(t, v167TestDocument(t, body...), "current", want)
		})
	}
	t.Run("answer after separator excluded", func(t *testing.T) {
		result := v167TestDocument(t, v167TestOpen(t, "matching", 2, 1), v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestLine(t, "---", "separator", 3, 2),
			v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestText(t, "Красный", "Красный", 3, 2), v167TestText(t, "Синий", "Синий", 3, 2), v167TestClose(t, 3, 2))
		v167TestError(t, result, 6, -1, "Ответы matching разрешены только до разделителя.")
		v167TestRun(t, result, "current", nil)
	})
}

func TestV167BoundariesAndMalformedAnswers(t *testing.T) {
	t.Run("multiple rows of one answer", func(t *testing.T) {
		v167TestRun(t, v167TestDocument(t, v167TestOpen(t, "matching", 2, 1), v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestOpen(t, "answer", 3, 2),
			v167TestText(t, "Красный", "Красный", 5, 3), v167TestText(t, `\@answer Синий`, "@answer Синий", 5, 3), v167TestClose(t, 5, 3),
			v167TestLine(t, "---", "separator", 3, 2), v167TestText(t, "Красный", "Красный", 3, 2), v167TestText(t, `\@answer Синий`, "@answer Синий", 3, 2), v167TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("recognized invalid answer", func(t *testing.T) {
		// Пункт 1.29 запрещает табуляцию вместо разделителя после имени.
		invalid := v167TestLine(t, "@answer\tСиний", "invalid", 3, 2, v167TestElement(t, "tag", "@answer", "answer", 1, 8), v167TestElement(t, "unparsed", "\tСиний", "", 8, 14))
		invalid.Elements[1].Value = nil
		result := v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), invalid)
		v167TestError(t, result, 6, 1, "Табуляция не заменяет пробел после имени тега.")
		v167TestRun(t, result, "current", []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}})
	})
	t.Run("unparsed without tag is not recovered", func(t *testing.T) {
		invalid := v167TestLine(t, "@answer{", "invalid", 3, 2, v167TestElement(t, "unparsed", "@answer{", "", 1, 9))
		invalid.Elements[0].Value = nil
		result := v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), invalid)
		v167TestError(t, result, 6, 0, "Объявление не разобрано парсером.")
		v167TestRun(t, result, "current", nil)
	})
	t.Run("unclosed matching still has upper list", func(t *testing.T) {
		result := v167TestExample(t)
		result.Lines = result.Lines[:len(result.Lines)-1]
		*result.Document.LineCount = len(result.Lines)
		result.Lines[len(result.Lines)-1].LineEnding = ""
		v167TestError(t, result, 3, -1, "Блок matching не закрыт.")
		v167TestRun(t, result, "current", []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal body of "+tag, func(t *testing.T) {
			v167TestRun(t, v167TestDocument(t, v167TestOpen(t, tag, 2, 1), v167TestText(t, "@matching {", "@matching {", 3, 2),
				v167TestText(t, "@answer Красный", "@answer Красный", 3, 2), v167TestText(t, "@answer Синий", "@answer Синий", 3, 2),
				v167TestText(t, "---", "---", 3, 2), v167TestClose(t, 3, 2)), "current", nil)
		})
	}
	for _, container := range []string{"choice", "ordering"} {
		t.Run("outside matching "+container, func(t *testing.T) {
			result := v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2))
			if container == "choice" {
				// У choice нет разделителя списков: оставляем только варианты
				// и два ответа, ошибочных для choice, но не проверяемых V167.
				result = v167TestDocument(t, v167TestOpen(t, "choice", 2, 1), v167TestText(t, "Зелёный", "Зелёный", 3, 2),
					v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2), v167TestClose(t, 3, 2))
			}
			opening := v167TestOpen(t, container, 2, 1)
			opening.Number = 3
			result.Lines[2] = opening
			v167TestError(t, result, 6, -1, "Нарушена структура другого интерактивного контейнера.")
			v167TestRun(t, result, "current", nil)
		})
	}
	t.Run("question answers excluded", func(t *testing.T) {
		v167TestRun(t, v167TestDocument(t, v167TestTag(t, "question", "content", "Какой цвет?", 2, 1), v167TestTag(t, "answer", "content", "Красный", 2, 1), v167TestTag(t, "answer", "content", "Синий", 2, 1)), "current", nil)
	})
}

func TestV167BlockConstructs(t *testing.T) {
	t.Run("repeated block answer", func(t *testing.T) {
		v167TestRun(t, v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2),
			v167TestOpen(t, "answer", 3, 2), v167TestText(t, "Синий", "Синий", 6, 3), v167TestText(t, "Красный", "Красный", 6, 3), v167TestClose(t, 6, 3)),
			"current", []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}})
	})
	t.Run("two completed blocks with blanks", func(t *testing.T) {
		v167TestRun(t, v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestOpen(t, "answer", 3, 2),
			v167TestText(t, "Красный", "Красный", 5, 3), v167TestClose(t, 5, 3), v167TestBlank(t, "", 3, 2),
			v167TestOpen(t, "hint", 3, 2), v167TestText(t, "Подумайте.", "Подумайте.", 9, 3), v167TestClose(t, 9, 3), v167TestBlank(t, "\t", 3, 2),
			v167TestTag(t, "answer", "content", "Синий", 3, 2)), "current", []v167Expected{{13, 1, 8, "Повторный @answer для элемента верхнего списка."}})
	})
	t.Run("nested answer is excluded", func(t *testing.T) {
		result := v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2),
			v167TestOpen(t, "hint", 3, 2), v167TestText(t, "Подумайте.", "Подумайте.", 6, 3), v167TestTag(t, "answer", "content", "Синий", 6, 3), v167TestClose(t, 6, 3),
			v167TestTag(t, "answer", "content", "Синий", 3, 2))
		v167TestError(t, result, 8, -1, "Вложенный тег запрещён в hint.")
		v167TestRun(t, result, "current", []v167Expected{{10, 1, 8, "Повторный @answer для элемента верхнего списка."}})
	})
	t.Run("hint first does not reach outside matching", func(t *testing.T) {
		result := v167TestMatching(t, v167TestOpen(t, "hint", 3, 2), v167TestText(t, "Подумайте.", "Подумайте.", 4, 3), v167TestClose(t, 4, 3),
			v167TestTag(t, "answer", "content", "Синий", 3, 2), v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2))
		v167TestError(t, result, 4, -1, "Подсказка должна следовать после ответа.")
		v167TestRun(t, result, "current", []v167Expected{{7, 1, 8, ""}})
	})
	t.Run("Unicode whitespace is not an ASCII blank line", func(t *testing.T) {
		v167TestRun(t, v167TestMatching(t, v167TestText(t, "\u00a0", "\u00a0", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)), "current", nil)
	})
}

func TestV167Contexts(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		prefix                  []model.Line
		matching, parent, depth int
		suffix                  []model.Line
		want                    []v167Expected
	}{
		{"task", []model.Line{v167TestTag(t, "task", "identifier", "first", 2, 1)}, 4, 3, 2, []model.Line{v167TestTag(t, "endtask", "", "", 2, 1)}, []v167Expected{{7, 1, 8, "Повторный @answer для элемента верхнего списка."}}},
		{"step", []model.Line{v167TestTag(t, "task", "identifier", "first", 2, 1), v167TestTag(t, "step", "title", "First", 3, 2)}, 5, 4, 3, []model.Line{v167TestTag(t, "endtask", "", "", 2, 1)}, []v167Expected{{8, 1, 8, "Повторный @answer для элемента верхнего списка."}}},
		{"variant", []model.Line{v167TestOpen(t, "variants", 2, 1), v167TestTag(t, "variant", "name", "A", 3, 2)}, 5, 4, 3,
			[]model.Line{v167TestTag(t, "variant", "name", "B", 3, 2), v167TestText(t, "Читайте.", "Читайте.", 13, 3), v167TestClose(t, 3, 2)}, []v167Expected{{8, 1, 8, "Повторный @answer для элемента верхнего списка."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.prefix, v167TestOpen(t, "matching", tc.parent, tc.depth), v167TestText(t, "Цвет", "Цвет", tc.matching, tc.depth+1),
				v167TestTag(t, "answer", "content", "Красный", tc.matching, tc.depth+1), v167TestTag(t, "answer", "content", "Синий", tc.matching, tc.depth+1),
				v167TestLine(t, "---", "separator", tc.matching, tc.depth+1), v167TestText(t, "Красный", "Красный", tc.matching, tc.depth+1), v167TestText(t, "Синий", "Синий", tc.matching, tc.depth+1), v167TestClose(t, tc.matching, tc.depth+1))
			body = append(body, tc.suffix...)
			v167TestRun(t, v167TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, repeat := range []bool{false, true} {
		t.Run("independent blocks repeat="+strconv.FormatBool(repeat), func(t *testing.T) {
			var body []model.Line
			for _, opening := range []int{3, 11} {
				body = append(body, v167TestOpen(t, "matching", 2, 1), v167TestText(t, "Цвет", "Цвет", opening, 2), v167TestTag(t, "answer", "content", "Красный", opening, 2))
				if repeat {
					body = append(body, v167TestTag(t, "answer", "content", "Синий", opening, 2))
				} else {
					body = append(body, v167TestBlank(t, "", opening, 2))
				}
				body = append(body, v167TestLine(t, "---", "separator", opening, 2), v167TestText(t, "Красный", "Красный", opening, 2), v167TestText(t, "Синий", "Синий", opening, 2), v167TestClose(t, opening, 2))
			}
			var want []v167Expected
			if repeat {
				want = []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}, {14, 1, 8, "Повторный @answer для элемента верхнего списка."}}
			}
			v167TestRun(t, v167TestDocument(t, body...), "current", want)
		})
	}
}

func TestV167RangesAndRuns(t *testing.T) {
	t.Run("case indentation and Unicode", func(t *testing.T) {
		result := v167TestExample(t)
		result.Lines[2] = v167TestLine(t, "@MaTcHiNg {", "block-start", 2, 1, v167TestElement(t, "tag", "@MaTcHiNg", "matching", 1, 10), v167TestElement(t, "block-open", "{", "{", 11, 12))
		result.Lines[2].Number = 3
		result.Lines[5] = v167TestLine(t, " \t@AnSwEr Синий", "tag", 3, 2, v167TestElement(t, "tag", "@AnSwEr", "answer", 3, 10), v167TestElement(t, "content", "Синий", "Синий", 11, 16))
		result.Lines[5].Number = 6
		v167TestRun(t, result, "current", []v167Expected{{6, 3, 10, "Повторный @answer для элемента верхнего списка."}})
	})
	t.Run("equal values still repeat", func(t *testing.T) {
		v167TestRun(t, v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)), "current", []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}})
	})
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v167TestExample(t)
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v167Expected{{6, 1, 8, "Повторный @answer для элемента верхнего списка."}}
			v167TestRun(t, result, "first", want)
			v167TestRun(t, result, "second", want)
			v167TestRun(t, v167TestMatching(t, v167TestText(t, "Цвет", "Цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2)), "separate", nil)
		})
	}
}

func TestV167EmptyAndRegistration(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v167TestRun(t, result, "empty", nil)
		})
	}
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V167" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "element" || definition.Check == nil || definition.Message != "Ответ @answer не следует непосредственно за элементом верхнего списка." || definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V167 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v167TestExample(t)
			result.Document.DSLVersion = &version
			result.Lines[0] = v167TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v167TestExample строит контрольный пример с двумя вводными строками.
func v167TestExample(t *testing.T) *model.Result {
	t.Helper()
	return v167TestMatching(t, v167TestText(t, "Подходящий цвет", "Подходящий цвет", 3, 2), v167TestTag(t, "answer", "content", "Красный", 3, 2), v167TestTag(t, "answer", "content", "Синий", 3, 2))
}

// v167TestMatching окружает явно заданный верхний список полным контейнером.
func v167TestMatching(t *testing.T, upper ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v167TestOpen(t, "matching", 2, 1)}, upper...)
	body = append(body, v167TestLine(t, "---", "separator", 3, 2), v167TestText(t, "Красный", "Красный", 3, 2), v167TestText(t, "Синий", "Синий", 3, 2), v167TestClose(t, 3, 2))
	return v167TestDocument(t, body...)
}

// v167TestRun запускает штатный конвейер и проверяет только V167 текущего запуска.
func v167TestRun(t *testing.T, result *model.Result, source string, want []v167Expected) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V167" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V167 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "Ответ @answer не следует непосредственно за элементом верхнего списка."
		if expected.detail != "" {
			message += "\nконкретизация: " + expected.detail
		}
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id != entry.ID {
						continue
					}
					links++
					if line.Number != expected.line || element.Start != expected.start || element.End != expected.end ||
						element.ElementType != "tag" || element.Value == nil || *element.Value != "answer" {
						t.Fatalf("V167 linked to wrong element: line %d %+v", line.Number, element)
					}
				}
			}
		}
		if links != 1 || !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatalf("links=%d or error flags missing for %+v", links, expected)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original model.Result
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	for index, line := range original.Lines {
		current := result.Lines[index]
		current.HasErrors = line.HasErrors
		current.Elements = append([]model.Element{}, current.Elements...)
		for element := range current.Elements {
			current.Elements[element].ErrorIDs = line.Elements[element].ErrorIDs
		}
		left, _ := json.Marshal(line)
		right, _ := json.Marshal(current)
		if string(left) != string(right) {
			t.Fatalf("source changed on line %d", index+1)
		}
	}
	for index, old := range original.Diagnostics {
		left, _ := json.Marshal(old)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v167TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v167TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
	})
}

// v167TestElement задаёт исходное и семантическое значения без преобразований.
func v167TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v167TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v167TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v167TestTag создаёт однострочное объявление с явно заданным параметром.
func v167TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v167TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v167TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v167TestLine(t, raw, "tag", parent, depth, elements...)
}

// v167TestOpen создаёт объявление блока без параметров.
func v167TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v167TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v167TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v167TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v167TestClose создаёт закрытие с заданным родителем.
func v167TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v167TestLine(t, "}", "block-end", parent, depth, v167TestElement(t, "block-close", "}", "}", 1, 2))
}

// v167TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v167TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v167TestLine(t, raw, "content", parent, depth, v167TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v167TestBlank задаёт обычную пустую строку без текстового элемента.
func v167TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v167TestLine(t, raw, "blank", parent, depth)
}

// v167TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v167TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v167TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v167TestLine(t, "# Lesson", "heading", 0, 0, v167TestElement(t, "heading-level", "#", "1", 1, 2), v167TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v148Expected задаёт явный диапазон: строка, начало, исключаемый конец.
// Колонки считаются в кодовых точках Unicode с единицы.
type v148Expected struct {
	primary [3]int
	detail  string
}

func TestV148ReferenceExampleAndRuns(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v148Expected
	}{
		{"reference", "We _____ here.", "We _____ here.", nil},
		{"example", `We _ / ____ / ______ / \_____ here.`, "We _ / ____ / ______ / _____ here.", []v148Expected{
			{[3]int{3, 4, 5}, "Длина последовательности: 1; требуется 5."},
			{[3]int{3, 8, 12}, "Длина последовательности: 4; требуется 5."},
			{[3]int{3, 15, 21}, "Длина последовательности: 6; требуется 5."},
			{[3]int{3, 25, 30}, "Литеральное употребление подчёркиваний запрещено."}}},
		{"single", "_", "_", []v148Expected{{[3]int{3, 1, 2}, "Длина последовательности: 1; требуется 5."}}},
		{"two", "__", "__", []v148Expected{{[3]int{3, 1, 3}, "Длина последовательности: 2; требуется 5."}}},
		{"four", "____", "____", []v148Expected{{[3]int{3, 1, 5}, "Длина последовательности: 4; требуется 5."}}},
		{"five", "_____", "_____", nil},
		{"six", "______", "______", []v148Expected{{[3]int{3, 1, 7}, "Длина последовательности: 6; требуется 5."}}},
		{"ten not two gaps", "__________", "__________", []v148Expected{{[3]int{3, 1, 11}, "Длина последовательности: 10; требуется 5."}}},
		{"several", "_ _____ __", "_ _____ __", []v148Expected{
			{[3]int{3, 1, 2}, "Длина последовательности: 1; требуется 5."}, {[3]int{3, 9, 11}, "Длина последовательности: 2; требуется 5."}}},
		{"literal five", `\_____`, "_____", []v148Expected{{[3]int{3, 2, 7}, "Литеральное употребление подчёркиваний запрещено."}}},
		{"literal wrong length", `\__`, "__", []v148Expected{{[3]int{3, 2, 4}, "Литеральное употребление подчёркиваний запрещено."}}},
		{"escaped backslash", `\\_____`, `\_____`, nil},
		{"odd backslashes", `\\\_____`, `\_____`, []v148Expected{{[3]int{3, 4, 9}, "Литеральное употребление подчёркиваний запрещено."}}},
		{"four backslashes", `\\\\_____`, `\\_____`, nil},
		{"escape separates raw runs", `__\___`, "_____", []v148Expected{
			{[3]int{3, 1, 3}, "Длина последовательности: 2; требуется 5."}, {[3]int{3, 4, 7}, "Литеральное употребление подчёркиваний запрещено."}}},
		{"Unicode positions", "Я _ 🙂 ____", "Я _ 🙂 ____", []v148Expected{
			{[3]int{3, 3, 4}, "Длина последовательности: 1; требуется 5."}, {[3]int{3, 7, 11}, "Длина последовательности: 4; требуется 5."}}},
		{"indentation preserved", " \t_ \t", "_", []v148Expected{{[3]int{3, 3, 4}, "Длина последовательности: 1; требуется 5."}}},
		{"inside word", "a_____b", "a_____b", nil},
		{"case irrelevant", "A_B", "A_B", []v148Expected{{[3]int{3, 2, 3}, "Длина последовательности: 1; требуется 5."}}},
		{"V012 uppercase input also violates V148", "A1-12_B", "A1-12_B", []v148Expected{{[3]int{3, 6, 7}, "Длина последовательности: 1; требуется 5."}}},
		{"V012 lowercase input also violates V148", "a1-12_b", "a1-12_b", []v148Expected{{[3]int{3, 6, 7}, "Длина последовательности: 1; требуется 5."}}},
		{"V022 input also violates V148", "he1_o", "he1_o", []v148Expected{{[3]int{3, 4, 5}, "Длина последовательности: 1; требуется 5."}}},
		{"HTML entity stays text", "&#95;", "&#95;", nil},
		{"punctuation separates", "__/_", "__/_", []v148Expected{
			{[3]int{3, 1, 3}, "Длина последовательности: 2; требуется 5."}, {[3]int{3, 4, 5}, "Длина последовательности: 1; требуется 5."}}},
		{"no underscores", "Read this.", "Read this.", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Фрагменты таблицы сохранены дословно в Raw. Добавляются только
			// версия и основной заголовок, необходимые по DSL 1.30.
			v148TestRun(t, v148TestDocument(t, v148TestText(t, tc.raw, tc.value, 2, 1)), "current", tc.want)
		})
	}
}

func TestV148TextContexts(t *testing.T) {
	for _, tag := range []string{"example", "wordlist", "table", "script", "text", "instr", "note", "alt", "hint", "fragment", "multifill"} {
		t.Run("block "+tag, func(t *testing.T) {
			opening := v148TestOpen(t, tag, 2, 1)
			if tag == "fragment" {
				opening = v148TestBlock(t, tag, "identifier", "f", 2, 1)
			}
			v148TestRun(t, v148TestDocument(t, opening, v148TestText(t, "_", "_", 3, 2), v148TestClose(t, 3, 2)), "current",
				[]v148Expected{{[3]int{4, 1, 2}, "Длина последовательности: 1; требуется 5."}})
		})
	}
	for _, tc := range []struct {
		tag, value string
		span       [3]int
	}{
		{"question", "_", [3]int{3, 11, 12}}, {"instr", "_", [3]int{3, 8, 9}},
		{"note", "_", [3]int{3, 7, 8}}, {"alt", "_", [3]int{3, 6, 7}},
		{"hint", "_", [3]int{3, 7, 8}}, {"multifill", "_", [3]int{3, 12, 13}},
		{"wordlist", "_; word.", [3]int{3, 11, 12}},
	} {
		t.Run("inline "+tc.tag, func(t *testing.T) {
			v148TestRun(t, v148TestDocument(t, v148TestTag(t, tc.tag, "content", tc.value, 2, 1)), "current",
				[]v148Expected{{tc.span, "Длина последовательности: 1; требуется 5."}})
		})
	}
	for _, tc := range []struct {
		tag  string
		span [3]int
	}{
		{"choice", [3]int{3, 9, 10}}, {"multichoice", [3]int{3, 14, 15}},
		{"matching", [3]int{3, 11, 12}}, {"ordering", [3]int{3, 11, 12}}, {"multifill", [3]int{3, 12, 13}},
	} {
		t.Run("instruction "+tc.tag, func(t *testing.T) {
			body := []model.Line{v148TestBlock(t, tc.tag, "content", "_", 2, 1), v148TestText(t, "A", "A", 3, 2)}
			if tc.tag == "matching" {
				body = append(body, v148TestLine(t, "---", "separator", 3, 2))
			}
			body = append(body, v148TestText(t, "B", "B", 3, 2), v148TestText(t, "C", "C", 3, 2), v148TestClose(t, 3, 2))
			v148TestRun(t, v148TestDocument(t, body...), "current", []v148Expected{{tc.span, "Длина последовательности: 1; требуется 5."}})
		})
	}
	t.Run("nested hint has own context", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "choice", 2, 1), v148TestTag(t, "answer", "content", "a_b", 3, 2),
			v148TestOpen(t, "hint", 3, 2), v148TestText(t, "_", "_", 5, 3), v148TestClose(t, 5, 3),
			v148TestText(t, "c_d", "c_d", 3, 2), v148TestClose(t, 3, 2)), "current",
			[]v148Expected{{[3]int{6, 1, 2}, "Длина последовательности: 1; требуется 5."}})
	})
	t.Run("task step and branch", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "variants", 2, 1), v148TestTag(t, "variant", "name", "A_B", 3, 2),
			v148TestTag(t, "task", "identifier", "task_id", 4, 3), v148TestTag(t, "step", "title", "First_step", 5, 4),
			v148TestTag(t, "question", "content", "_", 6, 5), v148TestTag(t, "answer", "content", "a_b", 6, 5),
			v148TestTag(t, "step", "title", "Second_step", 5, 4), v148TestText(t, "_", "_", 9, 5), v148TestTag(t, "endtask", "", "", 4, 3),
			v148TestTag(t, "variant", "name", "C_D", 3, 2), v148TestText(t, "_", "_", 12, 3), v148TestClose(t, 3, 2)), "current",
			[]v148Expected{{[3]int{7, 11, 12}, "Длина последовательности: 1; требуется 5."},
				{[3]int{10, 1, 2}, "Длина последовательности: 1; требуется 5."}, {[3]int{13, 1, 2}, "Длина последовательности: 1; требуется 5."}})
	})
}

func TestV148AnswersAndRepresentation(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v148Expected
		malformed        bool
	}{
		{"one answer", "_____{a_b}", "_____{a_b}", nil, false},
		{"alternatives", "_____{a_b}{c_d} _", "_____{a_b}{c_d} _", []v148Expected{{[3]int{4, 17, 18}, "Длина последовательности: 1; требуется 5."}}, false},
		{"escaped closing", `_____{a\}_b} _`, "_____{a}_b} _", []v148Expected{{[3]int{4, 14, 15}, "Длина последовательности: 1; требуется 5."}}, false},
		{"escaped opening", `_____{a\{_b} _`, "_____{a{_b} _", []v148Expected{{[3]int{4, 14, 15}, "Длина последовательности: 1; требуется 5."}}, false},
		{"backslash pair closes answer", `_____{a\\}_`, `_____{a\}_`, []v148Expected{{[3]int{4, 11, 12}, "Длина последовательности: 1; требуется 5."}}, false},
		{"underscore escape is answer data", `_____{\_}`, `_____{\_}`, nil, false},
		{"empty answer clarification A", "_____{}", "_____{}", nil, true},
		{"unclosed answer clarification A", "_____{word", "_____{word", nil, true},
		{"unclosed answer with underscore", "_____{a_b", "_____{a_b", nil, true},
		{"empty second answer", "_____{word}{} _", "_____{word}{} _", []v148Expected{{[3]int{4, 15, 16}, "Длина последовательности: 1; требуется 5."}}, true},
		{"wrong length does not start answer", "____{a_b}", "____{a_b}", []v148Expected{
			{[3]int{4, 1, 5}, "Длина последовательности: 4; требуется 5."}, {[3]int{4, 7, 8}, "Длина последовательности: 1; требуется 5."}}, true},
		{"literal special spelling", `\_____\{a_b\}`, "_____{a_b}", []v148Expected{
			{[3]int{4, 2, 7}, "Литеральное употребление подчёркиваний запрещено."}, {[3]int{4, 10, 11}, "Длина последовательности: 1; требуется 5."}}, false},
		{"space prevents answer group", "_____ {a_b}", "_____ {a_b}", []v148Expected{{[3]int{4, 9, 10}, "Длина последовательности: 1; требуется 5."}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v148TestDocument(t, v148TestOpen(t, "multifill", 2, 1), v148TestText(t, tc.raw, tc.value, 3, 2), v148TestClose(t, 3, 2))
			if tc.malformed {
				v148TestError(t, result, 4, -1, "Ошибочная форма ответа плейсхолдера.")
			}
			v148TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("special form outside multifill clarification A", func(t *testing.T) {
		result := v148TestDocument(t, v148TestText(t, "_____{word}", "_____{word}", 2, 1))
		v148TestError(t, result, 3, -1, "Специальная форма вне multifill запрещена.")
		v148TestRun(t, result, "current", nil)
	})
	for _, split := range []bool{false, true} {
		t.Run("model elements split="+strconv.FormatBool(split), func(t *testing.T) {
			line := v148TestText(t, "We _____{a_b} _", "We _____{a_b} _", 3, 2)
			if split {
				line.Elements = []model.Element{v148TestElement(t, "content", "We ", "We ", 1, 4),
					v148TestElement(t, "placeholder", "_____{a_b}", "a_b", 4, 14), v148TestElement(t, "content", " _", " _", 14, 16)}
			}
			v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "multifill", 2, 1), line, v148TestClose(t, 3, 2)), "current",
				[]v148Expected{{[3]int{4, 15, 16}, "Длина последовательности: 1; требуется 5."}})
		})
	}
	t.Run("ordinary placeholder without value", func(t *testing.T) {
		gap := v148TestElement(t, "placeholder", "_____", "", 4, 9)
		gap.Value = nil
		v148TestRun(t, v148TestDocument(t, v148TestLine(t, "We _____ here.", "content", 2, 1,
			v148TestElement(t, "content", "We ", "We ", 1, 4), gap, v148TestElement(t, "content", " here.", " here.", 9, 15))), "current", nil)
	})
	t.Run("adjacent content does not divide raw run", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestLine(t, "______", "content", 2, 1,
			v148TestElement(t, "content", "___", "___", 1, 4), v148TestElement(t, "content", "___", "___", 4, 7))), "current",
			[]v148Expected{{[3]int{3, 1, 7}, "Длина последовательности: 6; требуется 5."}})
	})
}

func TestV148HTML(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v148Expected
		malformed        bool
	}{
		{"formatted text", "<em>_</em>", "<em>_</em>", []v148Expected{{[3]int{4, 5, 6}, "Длина последовательности: 1; требуется 5."}}, false},
		{"attribute excluded", `<em class="a_b">_</em>`, `<em class="a_b">_</em>`, []v148Expected{{[3]int{4, 17, 18}, "Длина последовательности: 1; требуется 5."}}, true},
		{"quoted greater sign", `<x title="a>b_c">_</x>`, `<x title="a>b_c">_</x>`, []v148Expected{{[3]int{4, 18, 19}, "Длина последовательности: 1; требуется 5."}}, true},
		{"unknown marker", `<x data='a_b'>ok</x>`, `<x data='a_b'>ok</x>`, nil, true},
		{"escaped marker becomes text", `\<x a_b\>`, "<x a_b>", []v148Expected{{[3]int{4, 6, 7}, "Длина последовательности: 1; требуется 5."}}, false},
		{"paired slash before marker", `\\<em x="_">ok</em>`, `\<em x="_">ok</em>`, nil, true},
		{"marker separates runs", "__<em>___</em>", "__<em>___</em>", []v148Expected{
			{[3]int{4, 1, 3}, "Длина последовательности: 2; требуется 5."}, {[3]int{4, 7, 10}, "Длина последовательности: 3; требуется 5."}}, false},
		{"case", "<EM>_____</EM>", "<EM>_____</EM>", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v148TestDocument(t, v148TestOpen(t, "text", 2, 1), v148TestText(t, tc.raw, tc.value, 3, 2), v148TestClose(t, 3, 2))
			if tc.malformed {
				v148TestError(t, result, 4, -1, "Неподдерживаемый HTML или атрибут.")
			}
			v148TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("angle brackets outside text are literal", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestText(t, `<em x="_">ok</em>`, `<em x="_">ok</em>`, 2, 1)), "current",
			[]v148Expected{{[3]int{3, 8, 9}, "Длина последовательности: 1; требуется 5."}})
	})
}

func TestV148ExcludedData(t *testing.T) {
	t.Run("key content and title inside task", func(t *testing.T) {
		// DSL 3.7.2–3.7.5: у key только блочная форма внутри задания.
		v148TestRun(t, v148TestDocument(t, v148TestTag(t, "task", "identifier", "task_id", 2, 1),
			v148TestTag(t, "question", "content", "Where?", 3, 2), v148TestBlock(t, "key", "title", "Answer_key", 3, 2),
			v148TestText(t, "_ __ _____", "_ __ _____", 5, 3), v148TestClose(t, 5, 3), v148TestTag(t, "endtask", "", "", 2, 1)), "current", nil)
	})
	t.Run("erroneous inline key remains excluded", func(t *testing.T) {
		result := v148TestDocument(t, v148TestTag(t, "task", "identifier", "task_id", 2, 1),
			v148TestTag(t, "question", "content", "Where?", 3, 2), v148TestTag(t, "key", "content", "a_b", 3, 2), v148TestTag(t, "endtask", "", "", 2, 1))
		v148TestError(t, result, 5, -1, "Однострочная форма key не поддерживается (3.7.2).")
		v148TestRun(t, result, "current", nil)
	})
	for _, tag := range []string{"editor", "answer"} {
		for _, block := range []bool{false, true} {
			t.Run(tag+" block="+strconv.FormatBool(block), func(t *testing.T) {
				body := []model.Line{v148TestTag(t, "question", "content", "Where?", 2, 1)}
				if block {
					body = append(body, v148TestOpen(t, tag, 2, 1), v148TestText(t, "_ __ _____", "_ __ _____", 4, 2), v148TestClose(t, 4, 2))
				} else {
					body = append(body, v148TestTag(t, tag, "content", "_ __ _____", 2, 1))
				}
				v148TestRun(t, v148TestDocument(t, body...), "current", nil)
			})
		}
	}
	t.Run("literal pseudo tags in editor", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "editor", 2, 1), v148TestText(t, "@question _", "@question _", 3, 2),
			v148TestText(t, "@text _", "@text _", 3, 2), v148TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("pseudo tags in multifill are checked text", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "multifill", 2, 1), v148TestText(t, "@key _", "@key _", 3, 2), v148TestClose(t, 3, 2)), "current",
			[]v148Expected{{[3]int{4, 6, 7}, "Длина последовательности: 1; требуется 5."}})
	})
	// TITLE имеется только у перечисленных блоков, не у instr/hint (3.8.4, 3.11.8).
	for _, tag := range []string{"example", "wordlist", "table", "script", "text", "note", "alt", "fragment"} {
		t.Run("title or ID "+tag, func(t *testing.T) {
			kind := model.ElementTypeTitle
			if tag == "fragment" {
				kind = model.ElementTypeIdentifier
			}
			v148TestRun(t, v148TestDocument(t, v148TestBlock(t, tag, kind, "A_B", 2, 1), v148TestText(t, "Read.", "Read.", 3, 2), v148TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("metadata before heading", func(t *testing.T) {
		result := v148TestDocument(t)
		id, section := "doc_id", "section_name"
		result.Document.Metadata.DocumentID, result.Document.Metadata.Section = &id, &section
		result.Document.Metadata.ResourceDirs = []string{"a_b"}
		heading := result.Lines[1]
		heading.Number = 5
		result.Lines = append(result.Lines[:1], v148TestTag(t, "document-id", "identifier", id, 0, 0),
			v148TestTag(t, "section", "name", section, 0, 0), v148TestTag(t, "resource-dir", "resource-path", "a_b", 0, 0), heading)
		for index := range result.Lines {
			result.Lines[index].Number = index + 1
		}
		count := len(result.Lines)
		result.Document.LineCount = &count
		v148TestRun(t, result, "current", nil)
	})
	t.Run("media path", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestLine(t, "@media image a_b.png", "tag", 2, 1,
			v148TestElement(t, "tag", "@media", "media", 1, 7), v148TestElement(t, "media-type", "image", "image", 8, 13),
			v148TestElement(t, "source", "a_b.png", "a_b.png", 14, 21))), "current", nil)
	})
	t.Run("heading", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestLine(t, "### A_B", "heading", 0, 0,
			v148TestElement(t, "heading-level", "###", "3", 1, 4), v148TestElement(t, "title", "A_B", "A_B", 5, 8)),
			v148TestText(t, "Read.", "Read.", 3, 1)), "current", nil)
	})
}

func TestV148InteractiveLists(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice"} {
		t.Run(tag, func(t *testing.T) {
			v148TestRun(t, v148TestDocument(t, v148TestOpen(t, tag, 2, 1), v148TestText(t, "a_b", "a_b", 3, 2),
				v148TestTag(t, "answer", "content", "c_d", 3, 2), v148TestClose(t, 3, 2)), "current", nil)
		})
	}
	for _, tag := range []string{"matching", "ordering"} {
		t.Run(tag+" upper checked lower excluded", func(t *testing.T) {
			v148TestRun(t, v148TestDocument(t, v148TestOpen(t, tag, 2, 1), v148TestText(t, "_", "_", 3, 2),
				v148TestTag(t, "answer", "content", "a_b", 3, 2), v148TestText(t, "Second", "Second", 3, 2),
				v148TestTag(t, "answer", "content", "c_d", 3, 2), v148TestLine(t, "---", "separator", 3, 2),
				v148TestText(t, "a_b", "a_b", 3, 2), v148TestText(t, "c_d", "c_d", 3, 2), v148TestClose(t, 3, 2)), "current",
				[]v148Expected{{[3]int{4, 1, 2}, "Длина последовательности: 1; требуется 5."}})
		})
	}
	t.Run("short ordering and own context of nested hint", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "ordering", 2, 1), v148TestText(t, "_", "_", 3, 2),
			v148TestTag(t, "answer", "content", "1", 3, 2), v148TestTag(t, "hint", "content", "_", 3, 2),
			v148TestText(t, "Second", "Second", 3, 2), v148TestTag(t, "answer", "content", "2", 3, 2), v148TestClose(t, 3, 2)), "current",
			[]v148Expected{{[3]int{4, 1, 2}, "Длина последовательности: 1; требуется 5."}, {[3]int{6, 7, 8}, "Длина последовательности: 1; требуется 5."}})
	})
	t.Run("separator does not affect next block", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestOpen(t, "matching", 2, 1), v148TestText(t, "First", "First", 3, 2),
			v148TestLine(t, "---", "separator", 3, 2), v148TestText(t, "a_b", "a_b", 3, 2), v148TestText(t, "c_d", "c_d", 3, 2), v148TestClose(t, 3, 2),
			v148TestOpen(t, "ordering", 2, 1), v148TestText(t, "_", "_", 9, 2), v148TestText(t, "Second", "Second", 9, 2), v148TestClose(t, 9, 2)), "current",
			[]v148Expected{{[3]int{10, 1, 2}, "Длина последовательности: 1; требуется 5."}})
	})
}

func TestV148BoundariesEndingsAndHistory(t *testing.T) {
	for _, ending := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(ending, func(t *testing.T) {
			result := v148TestDocument(t, v148TestText(t, "_", "_", 2, 1), v148TestBlank(t, "", 2, 1), v148TestText(t, "____", "____", 2, 1))
			for index := range result.Lines {
				switch ending {
				case "LF":
					result.Lines[index].LineEnding = "\n"
				case "CRLF":
					result.Lines[index].LineEnding = "\r\n"
				case "mixed":
					if index%2 == 0 {
						result.Lines[index].LineEnding = "\r\n"
					}
				}
			}
			want := []v148Expected{{[3]int{3, 1, 2}, "Длина последовательности: 1; требуется 5."}, {[3]int{5, 1, 5}, "Длина последовательности: 4; требуется 5."}}
			v148TestRun(t, result, "first", want)
			v148TestRun(t, result, "second", want)
			v148TestRun(t, v148TestDocument(t, v148TestText(t, "We _____ here.", "We _____ here.", 2, 1)), "separate", nil)
		})
	}
	t.Run("adjacent lines do not join runs", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestText(t, "__", "__", 2, 1), v148TestText(t, "___", "___", 2, 1)), "current",
			[]v148Expected{{[3]int{3, 1, 3}, "Длина последовательности: 2; требуется 5."}, {[3]int{4, 1, 4}, "Длина последовательности: 3; требуется 5."}})
	})
	t.Run("unclosed answer does not consume next line", func(t *testing.T) {
		result := v148TestDocument(t, v148TestOpen(t, "multifill", 2, 1), v148TestText(t, "_____{a_b", "_____{a_b", 3, 2),
			v148TestText(t, "_", "_", 3, 2), v148TestClose(t, 3, 2))
		v148TestError(t, result, 4, -1, "Ответ не закрыт в пределах строки.")
		v148TestRun(t, result, "current", []v148Expected{{[3]int{5, 1, 2}, "Длина последовательности: 1; требуется 5."}})
	})
	t.Run("mixed case declaration", func(t *testing.T) {
		v148TestRun(t, v148TestDocument(t, v148TestLine(t, "@QuEsTiOn _", "tag", 2, 1,
			v148TestElement(t, "tag", "@QuEsTiOn", "question", 1, 10), v148TestElement(t, "content", "_", "_", 11, 12))), "current",
			[]v148Expected{{[3]int{3, 11, 12}, "Длина последовательности: 1; требуется 5."}})
	})
}

func TestV148EmptyAndRegistration(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v148TestRun(t, result, "empty", nil)
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
		if definition.Code != "V148" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Подчёркивания не образуют допустимый пропуск." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V148 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v148TestDocument(t, v148TestText(t, "_", "_", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0] = v148TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v148TestBlock задаёт параметр объявления перед открывающей скобкой.
func v148TestBlock(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	line := v148TestTag(t, tag, kind, value, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v148TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v148TestRun запускает штатный конвейер и проверяет только V148 текущего запуска.
func v148TestRun(t *testing.T, result *model.Result, source string, want []v148Expected) {
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
		if diagnostic.DiagnosticCode == "V148" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V148 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		primary := model.Location{Start: model.Position{Line: expected.primary[0], Column: expected.primary[1]}, End: model.Position{Line: expected.primary[0], Column: expected.primary[2]}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "Подчёркивания не образуют допустимый пропуск.\nконкретизация: "+expected.detail ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
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

// v148TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v148TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v148TestElement задаёт исходное и семантическое значения без преобразований.
func v148TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v148TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v148TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v148TestTag создаёт однострочное объявление с явно заданным параметром.
func v148TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v148TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v148TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v148TestLine(t, raw, "tag", parent, depth, elements...)
}

// v148TestOpen создаёт объявление блока без параметров.
func v148TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v148TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v148TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v148TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v148TestClose создаёт закрытие с заданным родителем.
func v148TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v148TestLine(t, "}", "block-end", parent, depth, v148TestElement(t, "block-close", "}", "}", 1, 2))
}

// v148TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v148TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v148TestLine(t, raw, "content", parent, depth, v148TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v148TestBlank задаёт обычную пустую строку без текстового элемента.
func v148TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v148TestLine(t, raw, "blank", parent, depth)
}

// v148TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v148TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v148TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v148TestLine(t, "# Lesson", "heading", 0, 0, v148TestElement(t, "heading-level", "#", "1", 1, 2), v148TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

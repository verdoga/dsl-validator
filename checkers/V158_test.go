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

// v158Expected задаёт строку, начало и исключаемый конец каждого диапазона.
type v158Expected struct {
	primary, related [3]int
}

func TestV158ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		last         model.Line
		want         []v158Expected
	}{
		{"reference", "@choice {\n@answer Italy\nSpain\nFrance\n}", v158TestText(t, "France", "France", 3, 2), nil},
		{"example", "@choice {\n@answer Italy\nSpain\n@answer France\n}", v158TestTag(t, "answer", "content", "France", 3, 2),
			[]v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
				v158TestText(t, "Spain", "Spain", 3, 2), tc.last, v158TestClose(t, 3, 2)}
			var raw []string
			for _, line := range body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// Дословные фрагменты дополнены только версией и основным
			// заголовком по DSL 1.30; их строки поэтому сдвинуты на две.
			v158TestRun(t, v158TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV158CountsAndInterveningContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v158Expected
	}{
		{"no answer", []model.Line{v158TestText(t, "Italy", "Italy", 3, 2), v158TestText(t, "Spain", "Spain", 3, 2)}, nil},
		{"adjacent answers", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestTag(t, "answer", "content", "France", 3, 2)},
			[]v158Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 8}}}},
		{"three answers keep first related", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestText(t, "Spain", "Spain", 3, 2),
			v158TestTag(t, "answer", "content", "France", 3, 2), v158TestText(t, "Portugal", "Portugal", 3, 2), v158TestTag(t, "answer", "content", "Germany", 3, 2)},
			[]v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}, {[3]int{8, 1, 8}, [3]int{4, 1, 8}}}},
		{"same answer text", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestText(t, "Spain", "Spain", 3, 2),
			v158TestTag(t, "answer", "content", "Italy", 3, 2)}, []v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}}},
		{"hint does not reset count", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestTag(t, "hint", "content", "Think of Rome.", 3, 2),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestTag(t, "answer", "content", "France", 3, 2)}, []v158Expected{{[3]int{7, 1, 8}, [3]int{4, 1, 8}}}},
		{"more than two intervening constructs", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestText(t, "Portugal", "Portugal", 3, 2), v158TestText(t, "Germany", "Germany", 3, 2),
			v158TestTag(t, "answer", "content", "France", 3, 2)}, []v158Expected{{[3]int{8, 1, 8}, [3]int{4, 1, 8}}}},
		{"blank line does not reset count", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestBlank(t, "", 3, 2),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestTag(t, "answer", "content", "France", 3, 2)}, []v158Expected{{[3]int{7, 1, 8}, [3]int{4, 1, 8}}}},
		{"block hint does not reset count", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestOpen(t, "hint", 3, 2),
			v158TestText(t, "Think of Rome.", "Think of Rome.", 5, 3), v158TestClose(t, 5, 3), v158TestText(t, "Spain", "Spain", 3, 2),
			v158TestTag(t, "answer", "content", "France", 3, 2)}, []v158Expected{{[3]int{9, 1, 8}, [3]int{4, 1, 8}}}},
		{"Unicode values", []model.Line{v158TestTag(t, "answer", "content", "Италия🙂", 3, 2), v158TestText(t, "Испания", "Испания", 3, 2),
			v158TestTag(t, "answer", "content", "日本", 3, 2)}, []v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v158TestOpen(t, "choice", 2, 1)}, tc.body...)
			v158TestRun(t, v158TestDocument(t, append(body, v158TestClose(t, 3, 2))...), "current", tc.want)
		})
	}
}

func TestV158AnswerForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v158Expected
	}{
		{"first block", []model.Line{v158TestOpen(t, "answer", 3, 2), v158TestText(t, "Italy", "Italy", 4, 3), v158TestClose(t, 4, 3),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestTag(t, "answer", "content", "France", 3, 2)}, []v158Expected{{[3]int{8, 1, 8}, [3]int{4, 1, 8}}}},
		{"second block", []model.Line{v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestText(t, "Spain", "Spain", 3, 2),
			v158TestOpen(t, "answer", 3, 2), v158TestText(t, "France", "France", 6, 3), v158TestClose(t, 6, 3)}, []v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}}},
		{"both blocks", []model.Line{v158TestOpen(t, "answer", 3, 2), v158TestText(t, "Italy", "Italy", 4, 3), v158TestClose(t, 4, 3),
			v158TestOpen(t, "answer", 3, 2), v158TestText(t, "France", "France", 7, 3), v158TestClose(t, 7, 3)}, []v158Expected{{[3]int{7, 1, 8}, [3]int{4, 1, 8}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v158TestOpen(t, "choice", 2, 1)}, tc.body...)
			v158TestRun(t, v158TestDocument(t, append(body, v158TestClose(t, 3, 2))...), "current", tc.want)
		})
	}
	for _, second := range []bool{false, true} {
		t.Run("multiline answer with second="+strconv.FormatBool(second), func(t *testing.T) {
			body := []model.Line{v158TestOpen(t, "choice", 2, 1), v158TestOpen(t, "answer", 3, 2),
				v158TestText(t, "Italy", "Italy", 4, 3), v158TestText(t, "France", "France", 4, 3), v158TestClose(t, 4, 3), v158TestText(t, "Spain", "Spain", 3, 2)}
			var want []v158Expected
			if second {
				body = append(body, v158TestTag(t, "answer", "content", "Germany", 3, 2))
				want = []v158Expected{{[3]int{9, 1, 8}, [3]int{4, 1, 8}}}
			}
			result := v158TestDocument(t, append(body, v158TestClose(t, 3, 2))...)
			v158TestError(t, result, 6, -1, "Блочный answer внутри choice допускает одну непустую строку.")
			v158TestRun(t, result, "current", want)
		})
	}
	for _, block := range []bool{false, true} {
		for _, first := range []bool{false, true} {
			t.Run("empty block="+strconv.FormatBool(block)+" first="+strconv.FormatBool(first), func(t *testing.T) {
				body := []model.Line{v158TestOpen(t, "choice", 2, 1)}
				if !first {
					body = append(body, v158TestTag(t, "answer", "content", "Italy", 3, 2))
				}
				emptyLine := len(body) + 3
				if block {
					body = append(body, v158TestOpen(t, "answer", 3, 2), v158TestClose(t, emptyLine, 3))
				} else {
					body = append(body, v158TestTag(t, "answer", "", "", 3, 2))
				}
				body = append(body, v158TestText(t, "Spain", "Spain", 3, 2), v158TestText(t, "Portugal", "Portugal", 3, 2))
				want := []v158Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 8}}}
				if first {
					body = append(body, v158TestTag(t, "answer", "content", "Italy", 3, 2))
					want = []v158Expected{{[3]int{7, 1, 8}, [3]int{4, 1, 8}}}
					if block {
						want = []v158Expected{{[3]int{8, 1, 8}, [3]int{4, 1, 8}}}
					}
				}
				result := v158TestDocument(t, append(body, v158TestClose(t, 3, 2))...)
				v158TestError(t, result, emptyLine, -1, "Ответ не содержит значения.")
				v158TestRun(t, result, "current", want)
			})
		}
	}
}

func TestV158ErroneousDeclarations(t *testing.T) {
	t.Run("recognized tag with tab separator", func(t *testing.T) {
		unparsed := v158TestElement(t, "unparsed", "\tItaly", "", 8, 14)
		unparsed.Value = nil
		result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "France", 3, 2),
			v158TestLine(t, "@answer\tItaly", "invalid", 3, 2, v158TestElement(t, "tag", "@answer", "answer", 1, 8), unparsed), v158TestClose(t, 3, 2))
		v158TestError(t, result, 5, 1, "Табуляция не заменяет разделитель после имени.")
		v158TestRun(t, result, "current", []v158Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 8}}})
	})
	t.Run("recognized block with missing separator", func(t *testing.T) {
		result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "France", 3, 2),
			v158TestLine(t, "@answer{", "block-start", 3, 2, v158TestElement(t, "tag", "@answer", "answer", 1, 8), v158TestElement(t, "block-open", "{", "{", 8, 9)),
			v158TestText(t, "Italy", "Italy", 5, 3), v158TestClose(t, 5, 3), v158TestClose(t, 3, 2))
		v158TestError(t, result, 5, -1, "Отсутствует разделитель перед открывающей скобкой.")
		v158TestRun(t, result, "current", []v158Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 8}}})
	})
	t.Run("unparsed declaration does not reset recognized first", func(t *testing.T) {
		unparsed := v158TestElement(t, "unparsed", "@answer\tItaly", "", 1, 14)
		unparsed.Value = nil
		result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "France", 3, 2),
			v158TestLine(t, "@answer\tItaly", "invalid", 3, 2, unparsed), v158TestTag(t, "answer", "content", "Spain", 3, 2), v158TestClose(t, 3, 2))
		v158TestError(t, result, 5, 0, "Объявление не разобрано.")
		v158TestRun(t, result, "current", []v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}})
	})
	for _, first := range []bool{false, true} {
		t.Run("recognized invalid first="+strconv.FormatBool(first), func(t *testing.T) {
			unparsed := v158TestElement(t, "unparsed", "}", "", 15, 16)
			unparsed.Value = nil
			broken := v158TestLine(t, "@answer Italy }", "invalid", 3, 2,
				v158TestElement(t, "tag", "@answer", "answer", 1, 8), v158TestElement(t, "content", "Italy", "Italy", 9, 14), unparsed)
			valid := v158TestTag(t, "answer", "content", "France", 3, 2)
			left, right, badLine := valid, broken, 6
			if first {
				left, right, badLine = broken, valid, 4
			}
			result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), left, v158TestText(t, "Spain", "Spain", 3, 2), right, v158TestClose(t, 3, 2))
			v158TestError(t, result, badLine, 2, "Лишняя неэкранированная скобка.")
			v158TestRun(t, result, "current", []v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}})
		})
	}
	for _, raw := range []string{"@answer{", "@answer\tItaly", "@answer", "@answerX Italy", "@answer! Italy"} {
		for _, first := range []bool{false, true} {
			t.Run("no tag "+raw+" first="+strconv.FormatBool(first), func(t *testing.T) {
				unparsed := v158TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
				unparsed.Value = nil
				broken := v158TestLine(t, raw, "invalid", 3, 2, unparsed)
				valid := v158TestTag(t, "answer", "content", "France", 3, 2)
				left, right, badLine := valid, broken, 6
				if first {
					left, right, badLine = broken, valid, 4
				}
				body := []model.Line{v158TestOpen(t, "choice", 2, 1), left, v158TestText(t, "Spain", "Spain", 3, 2), right}
				// Ошибочная открывающая скобка всё же представлена моделью как
				// граница answer; его содержимое не становится содержимым choice.
				if raw == "@answer{" {
					broken = v158TestLine(t, raw, "block-start", 3, 2,
						v158TestElement(t, "unparsed", "@answer", "", 1, 8), v158TestElement(t, "block-open", "{", "{", 8, 9))
					broken.Elements[0].Value = nil
					if first {
						body = []model.Line{v158TestOpen(t, "choice", 2, 1), broken, v158TestText(t, "Italy", "Italy", 4, 3),
							v158TestClose(t, 4, 3), v158TestText(t, "Spain", "Spain", 3, 2), valid}
					} else {
						body[3] = broken
						body = append(body, v158TestText(t, "Italy", "Italy", 6, 3), v158TestClose(t, 6, 3))
					}
				}
				result := v158TestDocument(t, append(body, v158TestClose(t, 3, 2))...)
				v158TestError(t, result, badLine, 0, "Объявление не разобрано.")
				v158TestRun(t, result, "current", nil)
			})
		}
	}
}

func TestV158Containers(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v158Expected
	}{
		{"multichoice", []model.Line{v158TestOpen(t, "multichoice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestTag(t, "answer", "content", "France", 3, 2), v158TestClose(t, 3, 2)}, nil},
		{"question outside choice", []model.Line{v158TestTag(t, "question", "content", "Which country?", 2, 1),
			v158TestTag(t, "answer", "content", "Italy", 2, 1), v158TestTag(t, "answer", "content", "France", 2, 1)}, nil},
		{"task", []model.Line{v158TestTag(t, "task", "identifier", "first", 2, 1), v158TestOpen(t, "choice", 3, 2),
			v158TestTag(t, "answer", "content", "Italy", 4, 3), v158TestText(t, "Spain", "Spain", 4, 3),
			v158TestTag(t, "answer", "content", "France", 4, 3), v158TestClose(t, 4, 3), v158TestTag(t, "endtask", "", "", 2, 1)},
			[]v158Expected{{[3]int{7, 1, 8}, [3]int{5, 1, 8}}}},
		{"internal step", []model.Line{v158TestTag(t, "task", "identifier", "first", 2, 1), v158TestTag(t, "step", "title", "First", 3, 2),
			v158TestOpen(t, "choice", 4, 3), v158TestTag(t, "answer", "content", "Italy", 5, 4), v158TestText(t, "Spain", "Spain", 5, 4),
			v158TestTag(t, "answer", "content", "France", 5, 4), v158TestClose(t, 5, 4), v158TestTag(t, "endtask", "", "", 2, 1)},
			[]v158Expected{{[3]int{8, 1, 8}, [3]int{6, 1, 8}}}},
		{"variant", []model.Line{v158TestOpen(t, "variants", 2, 1), v158TestTag(t, "variant", "name", "A", 3, 2),
			v158TestOpen(t, "choice", 4, 3), v158TestTag(t, "answer", "content", "Italy", 5, 4), v158TestText(t, "Spain", "Spain", 5, 4),
			v158TestTag(t, "answer", "content", "France", 5, 4), v158TestClose(t, 5, 4), v158TestTag(t, "variant", "name", "B", 3, 2),
			v158TestText(t, "Read.", "Read.", 10, 3), v158TestClose(t, 3, 2)}, []v158Expected{{[3]int{8, 1, 8}, [3]int{6, 1, 8}}}},
		{"adjacent independent choices", []model.Line{v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestClose(t, 3, 2), v158TestOpen(t, "choice", 2, 1),
			v158TestTag(t, "answer", "content", "France", 7, 2), v158TestText(t, "Germany", "Germany", 7, 2), v158TestClose(t, 7, 2)}, nil},
		{"two violating choices", []model.Line{v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
			v158TestTag(t, "answer", "content", "Spain", 3, 2), v158TestClose(t, 3, 2), v158TestOpen(t, "choice", 2, 1),
			v158TestTag(t, "answer", "content", "France", 7, 2), v158TestTag(t, "answer", "content", "Germany", 7, 2), v158TestClose(t, 7, 2)},
			[]v158Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 8}}, {[3]int{9, 1, 8}, [3]int{8, 1, 8}}}},
		{"choice instruction is not declaration", []model.Line{v158TestBlock(t, "choice", "content", "Read @answer Italy", 2, 1),
			v158TestTag(t, "answer", "content", "France", 3, 2), v158TestText(t, "Spain", "Spain", 3, 2), v158TestClose(t, 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v158TestRun(t, v158TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("nested choice has independent count", func(t *testing.T) {
		result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
			v158TestText(t, "Spain", "Spain", 3, 2), v158TestOpen(t, "choice", 3, 2), v158TestTag(t, "answer", "content", "France", 6, 3),
			v158TestTag(t, "answer", "content", "Germany", 6, 3), v158TestClose(t, 6, 3), v158TestTag(t, "answer", "content", "Portugal", 3, 2), v158TestClose(t, 3, 2))
		v158TestError(t, result, 6, -1, "Вложенный choice запрещён; его собственные объявления сохранены.")
		v158TestRun(t, result, "current", []v158Expected{{[3]int{8, 1, 8}, [3]int{7, 1, 8}}, {[3]int{10, 1, 8}, [3]int{4, 1, 8}}})
	})
	for _, second := range []bool{false, true} {
		t.Run("answer inside hint excluded second="+strconv.FormatBool(second), func(t *testing.T) {
			body := []model.Line{v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
				v158TestOpen(t, "hint", 3, 2), v158TestTag(t, "answer", "content", "France", 5, 3), v158TestClose(t, 5, 3), v158TestText(t, "Spain", "Spain", 3, 2)}
			var want []v158Expected
			if second {
				body = append(body, v158TestTag(t, "answer", "content", "Germany", 3, 2))
				want = []v158Expected{{[3]int{9, 1, 8}, [3]int{4, 1, 8}}}
			}
			result := v158TestDocument(t, append(body, v158TestClose(t, 3, 2))...)
			v158TestError(t, result, 6, -1, "Вложенный answer запрещён внутри hint.")
			v158TestRun(t, result, "current", want)
		})
	}
}

func TestV158LiteralText(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@answer France`, "@answer France"}, {"Read @answer France", "Read @answer France"},
		{`\\@answer France`, `\@answer France`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v158TestRun(t, v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
				v158TestText(t, tc.raw, tc.value, 3, 2), v158TestClose(t, 3, 2)), "current", nil)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			result := v158TestDocument(t, v158TestOpen(t, tag, 2, 1), v158TestText(t, "@choice {", "@choice {", 3, 2),
				v158TestText(t, "@answer Italy", "@answer Italy", 3, 2), v158TestText(t, "@answer France", "@answer France", 3, 2), v158TestClose(t, 3, 2))
			if tag == "multifill" {
				v158TestError(t, result, 5, -1, "Начальное @answer запрещено в multifill.")
				v158TestError(t, result, 6, -1, "Начальное @answer запрещено в multifill.")
			}
			v158TestRun(t, result, "current", nil)
		})
	}
}

func TestV158SourceRangesAndBoundaries(t *testing.T) {
	t.Run("mixed case and indentation", func(t *testing.T) {
		v158TestRun(t, v158TestDocument(t,
			v158TestLine(t, "@ChOiCe {", "block-start", 2, 1, v158TestElement(t, "tag", "@ChOiCe", "choice", 1, 8), v158TestElement(t, "block-open", "{", "{", 9, 10)),
			v158TestTag(t, "answer", "content", "Italy", 3, 2), v158TestText(t, "Spain", "Spain", 3, 2),
			v158TestLine(t, " \t@AnSwEr France", "tag", 3, 2, v158TestElement(t, "tag", "@AnSwEr", "answer", 3, 10), v158TestElement(t, "content", "France", "France", 11, 17)),
			v158TestClose(t, 3, 2)), "current", []v158Expected{{[3]int{6, 3, 10}, [3]int{4, 1, 8}}})
	})
	t.Run("related uses first declaration source columns", func(t *testing.T) {
		v158TestRun(t, v158TestDocument(t, v158TestOpen(t, "choice", 2, 1),
			v158TestLine(t, " \t@ANSWER Italy", "tag", 3, 2, v158TestElement(t, "tag", "@ANSWER", "answer", 3, 10), v158TestElement(t, "content", "Italy", "Italy", 11, 16)),
			v158TestTag(t, "answer", "content", "France", 3, 2), v158TestClose(t, 3, 2)), "current", []v158Expected{{[3]int{5, 1, 8}, [3]int{4, 3, 10}}})
	})
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
				v158TestText(t, "Spain", "Spain", 3, 2), v158TestTag(t, "answer", "content", "France", 3, 2), v158TestClose(t, 3, 2))
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v158Expected{{[3]int{6, 1, 8}, [3]int{4, 1, 8}}}
			v158TestRun(t, result, "first", want)
			v158TestRun(t, result, "second", want)
			v158TestRun(t, v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
				v158TestText(t, "Spain", "Spain", 3, 2), v158TestText(t, "France", "France", 3, 2), v158TestClose(t, 3, 2)), "separate", nil)
		})
	}
	t.Run("last declaration at EOF in unclosed choice", func(t *testing.T) {
		result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
			v158TestTag(t, "answer", "content", "France", 3, 2))
		v158TestError(t, result, 3, -1, "Блок choice не закрыт.")
		v158TestRun(t, result, "current", []v158Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 8}}})
	})
}

func TestV158EmptyAndRegistration(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v158TestRun(t, result, "empty", nil)
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
		if definition.Code != "V158" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "В @choice объявлено несколько тегов @answer." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V158 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v158TestDocument(t, v158TestOpen(t, "choice", 2, 1), v158TestTag(t, "answer", "content", "Italy", 3, 2),
				v158TestTag(t, "answer", "content", "France", 3, 2), v158TestClose(t, 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0] = v158TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v158TestBlock задаёт параметр объявления перед открывающей скобкой.
func v158TestBlock(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	line := v158TestTag(t, tag, kind, value, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v158TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v158TestRun запускает штатный конвейер и проверяет только V158 текущего запуска.
func v158TestRun(t *testing.T, result *model.Result, source string, want []v158Expected) {
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
		if diagnostic.DiagnosticCode == "V158" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V158 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		primary := model.Location{Start: model.Position{Line: expected.primary[0], Column: expected.primary[1]}, End: model.Position{Line: expected.primary[0], Column: expected.primary[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]}, End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "В @choice объявлено несколько тегов @answer.\nконкретизация: В @choice допускается только один правильный вариант." ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
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

// v158TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v158TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v158TestElement задаёт исходное и семантическое значения без преобразований.
func v158TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v158TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v158TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v158TestTag создаёт однострочное объявление с явно заданным параметром.
func v158TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v158TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v158TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v158TestLine(t, raw, "tag", parent, depth, elements...)
}

// v158TestOpen создаёт объявление блока без параметров.
func v158TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v158TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v158TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v158TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v158TestClose создаёт закрытие с заданным родителем.
func v158TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v158TestLine(t, "}", "block-end", parent, depth, v158TestElement(t, "block-close", "}", "}", 1, 2))
}

// v158TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v158TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v158TestLine(t, raw, "content", parent, depth, v158TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v158TestBlank задаёт обычную пустую строку без текстового элемента.
func v158TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v158TestLine(t, raw, "blank", parent, depth)
}

// v158TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v158TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v158TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v158TestLine(t, "# Lesson", "heading", 0, 0, v158TestElement(t, "heading-level", "#", "1", 1, 2), v158TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

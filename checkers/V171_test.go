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

// v171Expected задаёт основной и связанные исходные диапазоны [start,end).
type v171Expected struct {
	line, start, end int
	related          [][3]int
}

func TestV171ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		upper, lower []model.Line
		want         []v171Expected
	}{
		{"reference", "@matching {\nСолнце — звезда?\n@answer Да\nВода — жидкость?\n@answer Да\n---\nДа\nНет\n}",
			[]model.Line{v171TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestText(t, "Вода — жидкость?", "Вода — жидкость?", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)},
			[]model.Line{v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "Нет", "Нет", 3, 2)}, nil},
		{"example", "@matching {\nСолнце — звезда?\n@answer Да\n---\nДа\nда\n}",
			[]model.Line{v171TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)},
			[]model.Line{v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2)}, []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v171TestMatching(t, tc.upper, tc.lower...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			v171TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV171Groups(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		labels       []string
		want         []v171Expected
	}{
		{"unique", "Да", []string{"Да", "Нет"}, nil},
		{"three occurrences", "Да", []string{"Да", "да", "ДА"}, []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}, {9, 1, 3}}}}},
		{"two groups", "Да", []string{"Да", "да", "Нет", "нет"}, []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}, {10, 1, 4, [][3]int{{9, 1, 4}}}}},
		{"interleaved groups ordered by second occurrence", "Да", []string{"Да", "Нет", "нет", "да"}, []v171Expected{{9, 1, 4, [][3]int{{8, 1, 4}}}, {10, 1, 3, [][3]int{{7, 1, 3}}}}},
		{"unused duplicate", "Нет", []string{"Нет", "Да", "да"}, []v171Expected{{9, 1, 3, [][3]int{{8, 1, 3}}}}},
		{"answer matches primary and additional targets", "Yes", []string{"Yes", "YES!"}, []v171Expected{{8, 1, 5, [][3]int{{7, 1, 4}}}}},
		{"answer matches additional and primary targets", "Yes", []string{"YES!", "Yes"}, []v171Expected{{8, 1, 4, [][3]int{{7, 1, 5}}}}},
		{"unused normalized duplicate", "Нет", []string{"Нет", "Yes", "YES!"}, []v171Expected{{9, 1, 5, [][3]int{{8, 1, 4}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lower []model.Line
			for _, label := range tc.labels {
				lower = append(lower, v171TestText(t, label, label, 3, 2))
			}
			v171TestRun(t, v171TestMatching(t, []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", tc.answer, 3, 2)}, lower...), "current", tc.want)
		})
	}
}

func TestV171AutomaticMode(t *testing.T) {
	unknown := v171TestTag(t, "answer", "content", "{bad", 3, 2)
	unknown.Elements = unknown.Elements[:1]
	partial := v171TestTag(t, "answer", "content", "Да", 3, 2)
	partial.Raw += "{bad"
	partial.Elements = append(partial.Elements, v171TestElement(t, "unparsed", "{bad", "", 11, 15))
	partial.Elements[2].Value = nil
	for _, tc := range []struct {
		name      string
		upper     []model.Line
		want      []v171Expected
		errorLine int
	}{
		{"no answers", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2)}, nil, 0},
		{"partial answers", []model.Line{v171TestText(t, "Первый", "Первый", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestText(t, "Второй", "Второй", 3, 2)}, nil, 0},
		{"first item unanswered", []model.Line{v171TestText(t, "Первый", "Первый", 3, 2), v171TestText(t, "Второй", "Второй", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 0},
		{"all items answered", []model.Line{v171TestText(t, "Первый", "Первый", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestText(t, "Второй", "Второй", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, []v171Expected{{10, 1, 3, [][3]int{{9, 1, 3}}}}, 0},
		{"repeated answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 0},
		{"repeat after hint", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestTag(t, "hint", "content", "Подумайте.", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 0},
		{"no upper items", nil, nil, 3},
		{"orphan answer only", []model.Line{v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 4},
		{"blank between item and answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestBlank(t, " \t", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, []v171Expected{{9, 1, 3, [][3]int{{8, 1, 3}}}}, 0},
		{"hint before answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "hint", "content", "Подумайте.", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 5},
		{"inline editor interrupts", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "editor", "content", "Комментарий", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 5},
		{"block editor interrupts", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestOpen(t, "editor", 3, 2), v171TestText(t, "@answer Да", "@answer Да", 5, 3), v171TestClose(t, 5, 3), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 5},
		{"foreign block interrupts", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestOpen(t, "note", 3, 2), v171TestText(t, "Замечание", "Замечание", 5, 3), v171TestClose(t, 5, 3), v171TestTag(t, "answer", "content", "Да", 3, 2)}, nil, 5},
		{"block answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestOpen(t, "answer", 3, 2), v171TestText(t, "Да", "Да", 5, 3), v171TestText(t, "Нет", "Нет", 5, 3), v171TestClose(t, 5, 3)}, []v171Expected{{11, 1, 3, [][3]int{{10, 1, 3}}}}, 0},
		{"block hint after answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestOpen(t, "hint", 3, 2), v171TestText(t, "Подумайте.", "Подумайте.", 6, 3), v171TestClose(t, 6, 3)}, []v171Expected{{11, 1, 3, [][3]int{{10, 1, 3}}}}, 0},
		{"empty answer disables automatic mode", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "", "", 3, 2)}, nil, 5},
		{"empty content answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "", 3, 2)}, nil, 0},
		{"ASCII whitespace answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", " \t", 3, 2)}, nil, 0},
		{"Unicode whitespace answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "\u00a0\u2003", 3, 2)}, nil, 0},
		{"unknown raw answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), unknown}, nil, 0},
		{"partially parsed answer", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), partial}, nil, 0},
		{"later unavailable answer suppresses all duplicates", []model.Line{v171TestText(t, "Первый", "Первый", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2), v171TestText(t, "Второй", "Второй", 3, 2), unknown}, nil, 0},
		{"punctuation answer remains available", []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "!", 3, 2)}, []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v171TestMatching(t, tc.upper, v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2), v171TestText(t, "Нет", "Нет", 3, 2))
			for _, line := range result.Lines {
				for index, element := range line.Elements {
					if element.ElementType == "unparsed" {
						v171TestError(t, result, line.Number, index, "Не разобран остаток ответа.")
					}
				}
			}
			if tc.errorLine != 0 {
				v171TestError(t, result, tc.errorLine, -1, "Нарушены правила содержимого или размещения тега.")
			}
			v171TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV171BlockAnswerAvailability(t *testing.T) {
	unknown := v171TestLine(t, "{bad", "content", 5, 3)
	unparsed := v171TestLine(t, "{bad", "invalid", 5, 3,
		v171TestElement(t, "unparsed", "{bad", "", 1, 5))
	unparsed.Elements[0].Value = nil
	whitespaceUnparsed := v171TestLine(t, " \t", "invalid", 5, 3,
		v171TestElement(t, "unparsed", " \t", "", 1, 3))
	whitespaceUnparsed.Elements[0].Value = nil
	partial := v171TestLine(t, "Да{bad", "content", 5, 3,
		v171TestElement(t, "content", "Да", "Да", 1, 3), v171TestElement(t, "unparsed", "{bad", "", 3, 7))
	partial.Elements[1].Value = nil
	for _, tc := range []struct {
		name         string
		alternatives []model.Line
		available    bool
	}{
		{"all alternatives available", []model.Line{v171TestText(t, "Да", "Да", 5, 3), v171TestText(t, "Нет", "Нет", 5, 3)}, true},
		{"empty block", nil, false},
		{"blank block", []model.Line{v171TestBlank(t, " \t", 5, 3)}, false},
		{"blank lines around alternative", []model.Line{v171TestBlank(t, "", 5, 3), v171TestText(t, "Да", "Да", 5, 3), v171TestBlank(t, "\t", 5, 3)}, true},
		{"ASCII whitespace first alternative", []model.Line{v171TestText(t, " \t", " \t", 5, 3), v171TestText(t, "Да", "Да", 5, 3)}, false},
		{"ASCII whitespace last alternative", []model.Line{v171TestText(t, "Да", "Да", 5, 3), v171TestText(t, " \t", " \t", 5, 3)}, false},
		{"whitespace unparsed first alternative", []model.Line{whitespaceUnparsed, v171TestText(t, "Да", "Да", 5, 3)}, false},
		{"whitespace unparsed last alternative", []model.Line{v171TestText(t, "Да", "Да", 5, 3), whitespaceUnparsed}, false},
		{"ASCII whitespace raw with known value", []model.Line{v171TestText(t, " \t", "Да", 5, 3)}, true},
		{"empty first alternative", []model.Line{v171TestText(t, "\u00a0\u2003", "\u00a0\u2003", 5, 3), v171TestText(t, "Да", "Да", 5, 3)}, false},
		{"empty last alternative", []model.Line{v171TestText(t, "Да", "Да", 5, 3), v171TestText(t, "\u00a0\u2003", "\u00a0\u2003", 5, 3)}, false},
		{"unknown first alternative", []model.Line{unknown, v171TestText(t, "Да", "Да", 5, 3)}, false},
		{"unknown last alternative", []model.Line{v171TestText(t, "Да", "Да", 5, 3), unknown}, false},
		{"unparsed first alternative", []model.Line{unparsed, v171TestText(t, "Да", "Да", 5, 3)}, false},
		{"unparsed last alternative", []model.Line{v171TestText(t, "Да", "Да", 5, 3), unparsed}, false},
		{"partial first alternative", []model.Line{partial, v171TestText(t, "Да", "Да", 5, 3)}, false},
		{"partial last alternative", []model.Line{v171TestText(t, "Да", "Да", 5, 3), partial}, false},
		{"tag is not an alternative", []model.Line{v171TestTag(t, "hint", "content", "Да", 5, 3)}, false},
		{"punctuation alternative remains available", []model.Line{v171TestText(t, "!", "!", 5, 3)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upper := []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestOpen(t, "answer", 3, 2)}
			for _, alternative := range tc.alternatives {
				alternative.Elements = append([]model.Element{}, alternative.Elements...)
				upper = append(upper, alternative)
			}
			upper = append(upper, v171TestClose(t, 5, 3))
			first := len(upper) + 5
			var want []v171Expected
			if tc.available {
				want = []v171Expected{{first + 1, 1, 3, [][3]int{{first, 1, 3}}}}
			}
			result := v171TestMatching(t, upper, v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2))
			for _, line := range result.Lines {
				for index, element := range line.Elements {
					if element.ElementType == "unparsed" {
						v171TestError(t, result, line.Number, index, "Не разобран остаток ответа.")
					}
				}
			}
			v171TestRun(t, result, "current", want)
		})
	}
}

func TestV171PartialLabelDoesNotCreateDuplicate(t *testing.T) {
	for _, completeDuplicate := range []bool{false, true} {
		t.Run(strconv.FormatBool(completeDuplicate), func(t *testing.T) {
			unparsed := v171TestElement(t, "unparsed", "{bad", "", 6, 10)
			unparsed.Value = nil
			partial := v171TestLine(t, "Apple{bad", "content", 3, 2,
				v171TestElement(t, "content", "Apple", "Apple", 1, 6), unparsed)
			lower := []model.Line{partial, v171TestText(t, "Apple", "Apple", 3, 2)}
			var want []v171Expected
			if completeDuplicate {
				lower = append(lower, v171TestText(t, "apple", "apple", 3, 2))
				want = []v171Expected{{9, 1, 6, [][3]int{{8, 1, 6}}}}
			}
			upper := []model.Line{v171TestText(t, "Fruit", "Fruit", 3, 2), v171TestTag(t, "answer", "content", "Apple", 3, 2)}
			result := v171TestMatching(t, upper, lower...)
			v171TestError(t, result, 7, 1, "Не разобран остаток подписи.")
			v171TestRun(t, result, "current", want)
		})
	}
}

func TestV171IncompleteAndFragmentedContent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first, second model.Line
		want          []v171Expected
	}{
		{"escaped prefix removed during comparison", v171TestLine(t, `\@Apple`, "content", 3, 2,
			v171TestElement(t, "content", "Apple", "Apple", 3, 8)),
			v171TestText(t, "Apple", "Apple", 3, 2), []v171Expected{{8, 1, 6, [][3]int{{7, 1, 8}}}}},
		{"content omits escaped suffix", v171TestLine(t, `Apple\{x\}`, "content", 3, 2,
			v171TestElement(t, "content", "Apple", "Apple", 1, 6)),
			v171TestText(t, "Apple", "Apple", 3, 2), nil},
		{"complete raw restores Unicode prefix and escaped suffix", v171TestLine(t, " \t🌞Apple\\{x\\} \t", "content", 3, 2,
			v171TestElement(t, "content", "Apple", "Apple", 4, 9)),
			v171TestText(t, `🌞apple\{x\}`, "🌞apple{x}", 3, 2), []v171Expected{{8, 1, 12, [][3]int{{7, 3, 14}}}}},
		{"fragmented braces", v171TestLine(t, `\{Apple\}`, "content", 3, 2,
			v171TestElement(t, "content", `\{Apple`, "{Apple", 1, 8), v171TestElement(t, "content", `\}`, "}", 8, 10)),
			v171TestText(t, `\{apple\}`, "{apple}", 3, 2), []v171Expected{{8, 1, 10, [][3]int{{7, 1, 10}}}}},
		{"fragmented backslash removed during comparison", v171TestLine(t, `\\@Apple`, "content", 3, 2,
			v171TestElement(t, "content", `\\`, `\`, 1, 3), v171TestElement(t, "content", "@Apple", "@Apple", 3, 9)),
			v171TestText(t, `\@apple`, "@apple", 3, 2), []v171Expected{{8, 1, 8, [][3]int{{7, 1, 9}}}}},
		{"fragmented gaps collapsed during comparison", v171TestLine(t, "A  B", "content", 3, 2,
			v171TestElement(t, "content", "A", "A", 1, 2), v171TestElement(t, "content", "B", "B", 4, 5)),
			v171TestText(t, "A B", "A B", 3, 2), []v171Expected{{8, 1, 4, [][3]int{{7, 1, 5}}}}},
		{"fragmented gaps retained in duplicate", v171TestLine(t, "A  B", "content", 3, 2,
			v171TestElement(t, "content", "A", "A", 1, 2), v171TestElement(t, "content", "B", "B", 4, 5)),
			v171TestText(t, "a  b", "a  b", 3, 2), []v171Expected{{8, 1, 5, [][3]int{{7, 1, 5}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upper := []model.Line{v171TestText(t, "Question", "Question", 3, 2), v171TestTag(t, "answer", "content", "Answer", 3, 2)}
			v171TestRun(t, v171TestMatching(t, upper, tc.first, tc.second, v171TestText(t, "Answer", "Answer", 3, 2)), "current", tc.want)
		})
	}
}

func TestV171LabelsAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		name, raw1, value1, raw2, value2 string
		want                             []v171Expected
	}{
		{"inner spaces collapsed", "A B", "A B", "A  B", "A  B", []v171Expected{{8, 1, 5, [][3]int{{7, 1, 4}}}}},
		{"tab equals space", "A B", "A B", "A\tB", "A\tB", []v171Expected{{8, 1, 4, [][3]int{{7, 1, 4}}}}},
		{"punctuation removed", "Да", "Да", "Да!", "Да!", []v171Expected{{8, 1, 4, [][3]int{{7, 1, 3}}}}},
		{"NBSP trimmed", "Да", "Да", "Да\u00a0", "Да\u00a0", []v171Expected{{8, 1, 4, [][3]int{{7, 1, 3}}}}},
		{"Unicode spaces collapsed", "A\u00a0\u2003B", "A\u00a0\u2003B", "a\u2009b", "a\u2009b", []v171Expected{{8, 1, 4, [][3]int{{7, 1, 5}}}}},
		{"narrow NBSP equals space", "A B", "A B", "a\u202fb", "a\u202fb", []v171Expected{{8, 1, 4, [][3]int{{7, 1, 4}}}}},
		{"Unicode edge spaces trimmed", "\u2003Yes\u00a0", "\u2003Yes\u00a0", "\u202fYES!\u2009", "\u202fYES!\u2009", []v171Expected{{8, 1, 7, [][3]int{{7, 1, 6}}}}},
		{"Unicode empty labels excluded", "\u00a0\u2003", "\u00a0\u2003", "\u202f\u2009", "\u202f\u2009", nil},
		{"same punctuation remains duplicate", "!", "!", "!", "!", []v171Expected{{8, 1, 2, [][3]int{{7, 1, 2}}}}},
		{"different punctuation stays distinct", "!", "!", "?", "?", nil},
		{"long s stays distinct", "ſ", "ſ", "S", "S", nil},
		{"no Unicode normalization", "é", "é", "e\u0301", "e\u0301", nil},
		{"Unicode case", "ЁЖ", "ЁЖ", "ёж", "ёж", []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}}},
		{"Unicode columns", "🌞Да", "🌞Да", "🌞да", "🌞да", []v171Expected{{8, 1, 4, [][3]int{{7, 1, 4}}}}},
		{"escaped braces", `\{Да\}`, "{Да}", `\{да\}`, "{да}", []v171Expected{{8, 1, 7, [][3]int{{7, 1, 7}}}}},
		{"ordinary backslash preserved", `\q`, `\q`, `\\q`, `\q`, []v171Expected{{8, 1, 4, [][3]int{{7, 1, 3}}}}},
		{"backslash removed during comparison", `\\@note`, `\@note`, `\@note`, "@note", []v171Expected{{8, 1, 7, [][3]int{{7, 1, 8}}}}},
		{"initial at", `\@note`, "@note", `\@NOTE`, "@NOTE", []v171Expected{{8, 1, 7, [][3]int{{7, 1, 7}}}}},
		{"initial hash", `\#цвет`, "#цвет", `\#ЦВЕТ`, "#ЦВЕТ", []v171Expected{{8, 1, 7, [][3]int{{7, 1, 7}}}}},
		{"escaped separator", `\---`, "---", `\---`, "---", []v171Expected{{8, 1, 5, [][3]int{{7, 1, 5}}}}},
		{"noninitial at and slash removed", `A\@B`, `A\@B`, `A@B`, `A@B`, []v171Expected{{8, 1, 4, [][3]int{{7, 1, 5}}}}},
		{"noninitial hash and slash removed", `A\#B`, `A\#B`, `A#B`, `A#B`, []v171Expected{{8, 1, 4, [][3]int{{7, 1, 5}}}}},
		{"underscores not placeholders", `\_____`, `\_____`, `_____`, `_____`, nil},
		{"HTML backslashes removed during comparison", `\<b\>`, `\<b\>`, `<b>`, `<b>`, []v171Expected{{8, 1, 4, [][3]int{{7, 1, 6}}}}},
		{"cell separator backslash removed during comparison", `a\|b`, `a\|b`, `a|b`, `a|b`, []v171Expected{{8, 1, 4, [][3]int{{7, 1, 5}}}}},
	} {
		for _, fallback := range []bool{false, true} {
			t.Run(tc.name+" raw="+strconv.FormatBool(fallback), func(t *testing.T) {
				first := v171TestText(t, tc.raw1, tc.value1, 3, 2)
				second := v171TestText(t, tc.raw2, tc.value2, 3, 2)
				if fallback {
					first.Elements = []model.Element{}
					second.Elements = []model.Element{}
				}
				// Ответ ссылается на отдельный уникальный вариант, чтобы изолировать
				// сравнение двух проверяемых подписей от правил оформления @answer.
				v171TestRun(t, v171TestMatching(t, []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Ответ", 3, 2)}, first, second, v171TestText(t, "Ответ", "Ответ", 3, 2)), "current", tc.want)
			})
		}
	}
	for _, fallback := range []bool{false, true} {
		t.Run("source indentation raw="+strconv.FormatBool(fallback), func(t *testing.T) {
			first := v171TestLine(t, " \tДа \t", "content", 3, 2, v171TestElement(t, "content", "Да", "Да", 3, 5))
			second := v171TestLine(t, "\tда ", "content", 3, 2, v171TestElement(t, "content", "да", "да", 2, 4))
			if fallback {
				first.Elements = []model.Element{}
				second.Elements = []model.Element{}
			}
			v171TestRun(t, v171TestMatching(t, v171TestUpper(t), first, second), "current", []v171Expected{{8, 2, 4, [][3]int{{7, 3, 5}}}})
		})
	}
}

func TestV171SeparatorsAndNestedContent(t *testing.T) {
	t.Run("repeated block answer disables automatic mode", func(t *testing.T) {
		upper := append(v171TestUpper(t), v171TestOpen(t, "answer", 3, 2), v171TestText(t, "Да", "Да", 6, 3), v171TestClose(t, 6, 3))
		v171TestRun(t, v171TestMatching(t, upper, v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2)), "current", nil)
	})
	t.Run("nested answer cannot configure upper item", func(t *testing.T) {
		result := v171TestMatching(t, []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestOpen(t, "note", 3, 2),
			v171TestText(t, "Замечание", "Замечание", 5, 3), v171TestTag(t, "answer", "content", "Да", 5, 3), v171TestClose(t, 5, 3)},
			v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2))
		v171TestError(t, result, 5, -1, "Вложенный note запрещён в matching.")
		v171TestError(t, result, 7, -1, "Вложенный answer запрещён в note.")
		v171TestRun(t, result, "current", nil)
	})
	for _, tc := range []struct {
		name, raw, value string
		kind             model.LineType
		want             []v171Expected
	}{
		{"ASCII edges", " \t---\t ", "---", "separator", []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}}},
		{"escaped", `\---`, "---", "content", nil},
		{"NBSP edges", "\u00a0---\u00a0", "\u00a0---\u00a0", "content", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v171TestExample(t)
			line := v171TestLine(t, tc.raw, tc.kind, 3, 2)
			if tc.kind == "content" {
				line = v171TestText(t, tc.raw, tc.value, 3, 2)
			}
			line.Number = 6
			result.Lines[5] = line
			v171TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("no separator", func(t *testing.T) {
		v171TestRun(t, v171TestDocument(t, v171TestOpen(t, "matching", 2, 1), v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2),
			v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2), v171TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("two own separators", func(t *testing.T) {
		result := v171TestMatching(t, v171TestUpper(t), v171TestText(t, "Да", "Да", 3, 2), v171TestLine(t, "---", "separator", 3, 2), v171TestText(t, "да", "да", 3, 2))
		v171TestError(t, result, 8, -1, "В matching допустим один разделитель.")
		v171TestRun(t, result, "current", nil)
	})
	for _, own := range []bool{false, true} {
		t.Run("nested separator own="+strconv.FormatBool(own), func(t *testing.T) {
			body := []model.Line{v171TestOpen(t, "matching", 2, 1), v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2),
				v171TestOpen(t, "hint", 3, 2), v171TestText(t, "---", "---", 6, 3), v171TestClose(t, 6, 3)}
			var want []v171Expected
			if own {
				body = append(body, v171TestLine(t, "---", "separator", 3, 2))
				want = []v171Expected{{11, 1, 3, [][3]int{{10, 1, 3}}}}
			}
			body = append(body, v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2), v171TestClose(t, 3, 2))
			v171TestRun(t, v171TestDocument(t, body...), "current", want)
		})
	}
	t.Run("nested lower duplicate excluded", func(t *testing.T) {
		result := v171TestMatching(t, v171TestUpper(t), v171TestText(t, "Да", "Да", 3, 2), v171TestOpen(t, "hint", 3, 2),
			v171TestText(t, "да", "да", 8, 3), v171TestClose(t, 8, 3), v171TestText(t, "Нет", "Нет", 3, 2))
		v171TestError(t, result, 8, -1, "Подсказка matching допустима только после ответа верхнего списка.")
		v171TestRun(t, result, "current", nil)
	})
	t.Run("blank lines in both lists", func(t *testing.T) {
		v171TestRun(t, v171TestMatching(t, []model.Line{v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestBlank(t, "", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)},
			v171TestText(t, "Да", "Да", 3, 2), v171TestBlank(t, "\t", 3, 2), v171TestText(t, "да", "да", 3, 2)), "current", []v171Expected{{10, 1, 3, [][3]int{{8, 1, 3}}}})
	})
	t.Run("tag content is not lower label", func(t *testing.T) {
		result := v171TestMatching(t, v171TestUpper(t), v171TestText(t, "Да", "Да", 3, 2), v171TestTag(t, "answer", "content", "да", 3, 2), v171TestText(t, "Нет", "Нет", 3, 2))
		v171TestError(t, result, 8, -1, "Ответ matching запрещён после разделителя.")
		v171TestRun(t, result, "current", nil)
	})
	t.Run("empty lower list", func(t *testing.T) {
		result := v171TestMatching(t, v171TestUpper(t))
		v171TestError(t, result, 6, -1, "Нижний список matching пуст.")
		v171TestRun(t, result, "current", nil)
	})
	t.Run("one lower item", func(t *testing.T) {
		result := v171TestMatching(t, v171TestUpper(t), v171TestText(t, "Да", "Да", 3, 2))
		v171TestError(t, result, 7, -1, "Нижний список должен содержать не менее двух вариантов.")
		v171TestRun(t, result, "current", nil)
	})
	t.Run("unclosed matching with definite own rows", func(t *testing.T) {
		result := v171TestExample(t)
		result.Lines = result.Lines[:8]
		*result.Document.LineCount = 8
		result.Lines[7].LineEnding = ""
		v171TestError(t, result, 3, -1, "Блок matching не закрыт.")
		v171TestRun(t, result, "current", []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}})
	})
}

func TestV171Contexts(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		prefix, suffix         []model.Line
		opening, parent, depth int
		want                   []v171Expected
	}{
		{"task", []model.Line{v171TestTag(t, "task", "identifier", "first", 2, 1)}, []model.Line{v171TestTag(t, "endtask", "", "", 2, 1)}, 4, 3, 2, []v171Expected{{9, 1, 3, [][3]int{{8, 1, 3}}}}},
		{"step", []model.Line{v171TestTag(t, "task", "identifier", "first", 2, 1), v171TestTag(t, "step", "title", "First", 3, 2)}, []model.Line{v171TestTag(t, "endtask", "", "", 2, 1)}, 5, 4, 3, []v171Expected{{10, 1, 3, [][3]int{{9, 1, 3}}}}},
		{"variant", []model.Line{v171TestOpen(t, "variants", 2, 1), v171TestTag(t, "variant", "name", "A", 3, 2)},
			[]model.Line{v171TestTag(t, "variant", "name", "B", 3, 2), v171TestText(t, "Читайте.", "Читайте.", 12, 3), v171TestClose(t, 3, 2)}, 5, 4, 3, []v171Expected{{10, 1, 3, [][3]int{{9, 1, 3}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append(tc.prefix, v171TestOpen(t, "matching", tc.parent, tc.depth), v171TestText(t, "Вопрос", "Вопрос", tc.opening, tc.depth+1),
				v171TestTag(t, "answer", "content", "Да", tc.opening, tc.depth+1), v171TestLine(t, "---", "separator", tc.opening, tc.depth+1),
				v171TestText(t, "Да", "Да", tc.opening, tc.depth+1), v171TestText(t, "да", "да", tc.opening, tc.depth+1), v171TestClose(t, tc.opening, tc.depth+1))
			body = append(body, tc.suffix...)
			v171TestRun(t, v171TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, repeat := range []bool{false, true} {
		t.Run("independent blocks repeat="+strconv.FormatBool(repeat), func(t *testing.T) {
			var body []model.Line
			for _, opening := range []int{3, 10} {
				second := "Нет"
				if repeat {
					second = "да"
				}
				body = append(body, v171TestOpen(t, "matching", 2, 1), v171TestText(t, "Вопрос", "Вопрос", opening, 2), v171TestTag(t, "answer", "content", "Да", opening, 2),
					v171TestLine(t, "---", "separator", opening, 2), v171TestText(t, "Да", "Да", opening, 2), v171TestText(t, second, second, opening, 2), v171TestClose(t, opening, 2))
			}
			var want []v171Expected
			if repeat {
				want = []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}, {15, 1, 3, [][3]int{{14, 1, 3}}}}
			}
			v171TestRun(t, v171TestDocument(t, body...), "current", want)
		})
	}
	for _, tag := range []string{"choice", "multichoice", "ordering"} {
		t.Run("other container "+tag, func(t *testing.T) {
			body := []model.Line{v171TestOpen(t, tag, 2, 1), v171TestText(t, "Вопрос", "Вопрос", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}
			if tag == "ordering" {
				body = append(body, v171TestLine(t, "---", "separator", 3, 2))
			}
			body = append(body, v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2), v171TestClose(t, 3, 2))
			v171TestRun(t, v171TestDocument(t, body...), "current", nil)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v171TestRun(t, v171TestDocument(t, v171TestOpen(t, tag, 2, 1), v171TestText(t, "@matching {", "@matching {", 3, 2), v171TestText(t, "Вопрос", "Вопрос", 3, 2),
				v171TestText(t, "@answer Да", "@answer Да", 3, 2), v171TestText(t, "---", "---", 3, 2), v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2), v171TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV171MetadataEndingsAndHistory(t *testing.T) {
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
		if definition.Code == "V171" {
			count++
			if definition.Severity != "error" || definition.Scope != "line" || definition.Message != "В нижнем списке повторяется подпись варианта." || definition.Check == nil || definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
				t.Fatalf("unexpected definition: %+v", definition)
			}
		}
	}
	if count != 1 {
		t.Fatalf("V171 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v171TestExample(t)
			result.Document.DSLVersion = &version
			result.Lines[0] = v171TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v171TestRun(t, result, "current", nil)
		})
	}
	t.Run("mixed case declarations", func(t *testing.T) {
		result := v171TestExample(t)
		result.Lines[2] = v171TestLine(t, "@MaTcHiNg {", "block-start", 2, 1, v171TestElement(t, "tag", "@MaTcHiNg", "matching", 1, 10), v171TestElement(t, "block-open", "{", "{", 11, 12))
		result.Lines[2].Number = 3
		result.Lines[4] = v171TestLine(t, "@AnSwEr Да", "tag", 3, 2, v171TestElement(t, "tag", "@AnSwEr", "answer", 1, 8), v171TestElement(t, "content", "Да", "Да", 9, 11))
		result.Lines[4].Number = 5
		v171TestRun(t, result, "current", []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}})
	})
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v171TestExample(t)
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v171Expected{{8, 1, 3, [][3]int{{7, 1, 3}}}}
			v171TestRun(t, result, "first", want)
			v171TestRun(t, result, "second", want)
			v171TestRun(t, v171TestMatching(t, v171TestUpper(t), v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "Нет", "Нет", 3, 2)), "separate", nil)
		})
	}
}

// v171TestUpper задаёт полный верхний список из одного вопроса и ответа.
func v171TestUpper(t *testing.T) []model.Line {
	t.Helper()
	return []model.Line{v171TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v171TestTag(t, "answer", "content", "Да", 3, 2)}
}

// v171TestMatching окружает явно заданные списки полным контейнером.
func v171TestMatching(t *testing.T, upper []model.Line, lower ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v171TestOpen(t, "matching", 2, 1)}, upper...)
	body = append(body, v171TestLine(t, "---", "separator", 3, 2))
	body = append(body, lower...)
	body = append(body, v171TestClose(t, 3, 2))
	return v171TestDocument(t, body...)
}

// v171TestExample добавляет к примеру только версию и основной заголовок.
func v171TestExample(t *testing.T) *model.Result {
	t.Helper()
	return v171TestMatching(t, v171TestUpper(t), v171TestText(t, "Да", "Да", 3, 2), v171TestText(t, "да", "да", 3, 2))
}

// v171TestRun запускает штатный конвейер и проверяет только V171 текущего запуска.
func v171TestRun(t *testing.T, result *model.Result, source string, want []v171Expected) {
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
		if diagnostic.DiagnosticCode == "V171" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V171 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "В нижнем списке повторяется подпись варианта."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for index, related := range expected.related {
			location := model.Location{Start: model.Position{Line: related[0], Column: related[1]}, End: model.Position{Line: related[0], Column: related[2]}}
			if entry.RelatedLocations[index] != location {
				t.Fatalf("related %d = %+v, want %+v", index, entry.RelatedLocations[index], location)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("ScopeLine must not create element errorIds")
					}
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
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

// v171TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v171TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v171TestElement задаёт исходное и семантическое значения без преобразований.
func v171TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v171TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v171TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v171TestTag создаёт однострочное объявление с явно заданным параметром.
func v171TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v171TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v171TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v171TestLine(t, raw, "tag", parent, depth, elements...)
}

// v171TestOpen создаёт объявление блока без параметров.
func v171TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v171TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v171TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v171TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v171TestClose создаёт закрытие с заданным родителем.
func v171TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v171TestLine(t, "}", "block-end", parent, depth, v171TestElement(t, "block-close", "}", "}", 1, 2))
}

// v171TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v171TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v171TestLine(t, raw, "content", parent, depth, v171TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v171TestBlank задаёт обычную пустую строку без текстового элемента.
func v171TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v171TestLine(t, raw, "blank", parent, depth)
}

// v171TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v171TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v171TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v171TestLine(t, "# Lesson", "heading", 0, 0, v171TestElement(t, "heading-level", "#", "1", 1, 2), v171TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

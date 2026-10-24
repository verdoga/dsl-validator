package checkers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

// visibleExpectation задаёт исходные Unicode-колонки [start,end) строки 4.
type visibleExpectation struct {
	Range   [2]int
	Related [][2]int
}

func TestVisibleFormattingPreservesFindings(t *testing.T) {
	for _, tc := range []struct {
		code, raw string
		want      []visibleExpectation
	}{
		{"V003", ". . .", []visibleExpectation{{Range: [2]int{1, 6}}}},
		{"V003", "...", nil},
		{"V007", "1) Read (carefully", []visibleExpectation{{Range: [2]int{9, 10}}}},
		{"V007", "a) Read", nil},
		{"V013", "Я😀 - дом", []visibleExpectation{{Range: [2]int{4, 5}}}},
		{"V013", "северо-западный", nil},
		{"V014", "аб—вг", []visibleExpectation{{Range: [2]int{3, 4}}}},
		{"V014", "a—b", nil},
		{"V017", "Hi , I'm . . .", []visibleExpectation{{Range: [2]int{3, 4}}}},
		{"V018", "'We're here.'", nil},
		{"V018", "'Hi!'Next", []visibleExpectation{{Range: [2]int{4, 5}}}},
		{"V018", "3.14,Next", []visibleExpectation{{Range: [2]int{5, 6}}}},
		{"V018", "10:30", nil},
		{"V018", "https://example.org", nil},
		{"V018", "user@example.org", nil},
		{"V022", "Я😀 he11o", []visibleExpectation{{Range: [2]int{4, 9}}}},
		{"V022", "he44o", nil},
		{"V023", "2O05", []visibleExpectation{{Range: [2]int{1, 5}}}},
		{"V023", "OO", nil},
		{"V024", "1\u0301", []visibleExpectation{{Range: [2]int{2, 3}}}},
		{"V024", "a\u0301", nil},
		{"V026", "Hi!!", []visibleExpectation{{Range: [2]int{3, 5}}}},
		{"V026", "...", nil},
		{"V027", "'Hello'", []visibleExpectation{{Range: [2]int{1, 2}, Related: [][2]int{{7, 8}}}}},
		{"V027", "rock 'n' roll", nil},
		{"V027", "'tis", nil},
		{"V028", "( word)", []visibleExpectation{{Range: [2]int{2, 3}}}},
		{"V028", "( )", nil},
		{"V029", "1 – 2", []visibleExpectation{{Range: [2]int{2, 5}}}},
		{"V029", "a1 – 2", nil},
		{"V029", "1 – 2 – 3", nil},
	} {
		original := []rune(tc.raw)
		for index := range original {
			t.Run(fmt.Sprintf("%s/%s/character_%d", tc.code, tc.raw, index), func(t *testing.T) {
				// Вставка одного вложенного выделения не меняет видимый текст.
				open, close := "<u><EM>", "</EM></u>"
				raw := string(original[:index]) + open + string(original[index]) + close + string(original[index+1:])
				column := func(originalColumn int) int {
					if originalColumn-1 < index {
						return originalColumn
					}
					if originalColumn-1 == index {
						return originalColumn + len(open)
					}
					return originalColumn + len(open) + len(close)
				}
				span := func(originalRange [2]int) [2]int {
					return [2]int{column(originalRange[0]), column(originalRange[1]-1) + 1}
				}
				want := make([]visibleExpectation, len(tc.want))
				for i, expected := range tc.want {
					want[i].Range = span(expected.Range)
					for _, related := range expected.Related {
						want[i].Related = append(want[i].Related, span(related))
					}
				}
				result := v020TextBlock(t, raw)
				v021Run(t, result, "current", nil)
				visibleExpect(t, result, "current", tc.code, want)
			})
		}
	}
}

func TestVisibleTextRegressions(t *testing.T) {
	data := []byte(`[
  {"name":"V018/URL", "raw":"https://example.org", "want":{"V018":[]}},
  {"name":"V018/email", "raw":"user@example.org", "want":{"V018":[]}},
  {"name":"V018/decimal", "raw":"3<em>.</em>14", "want":{"V018":[]}},
  {"name":"V018/time", "raw":"10<em>:</em>30", "want":{"V018":[]}},
  {"name":"V018/mixed_quotes", "raw":"‘Are you students?'", "want":{"V018":[]}},
  {"name":"V018/unmatched_closing_quote", "raw":"we're.'", "want":{"V018":[]}},
  {"name":"V018/answer_closing_quote", "raw":"we are.'", "tag":"choice", "want":{"V018":[]}},
  {"name":"V018/mixed_quotes_answer", "raw":"‘Yes, you are.'", "want":{"V018":[]}},
  {"name":"V018/formatted_apostrophe", "raw":"'We<em>'</em>re ready.'", "want":{"V018":[]}},
  {"name":"V018/opening_quote", "raw":"Hi,'Next'", "want":{"V018":[{"range":[3,4]}]}},
  {"name":"V018/text_after_closing_quote", "raw":"'Hi!'Next", "want":{"V018":[{"range":[4,5]}]}},
  {"name":"V018/decimal_then_error", "raw":"3<em>.</em>14,Next", "want":{"V018":[{"range":[14,15]}]}},
  {"name":"V018/URL_then_error", "raw":"https://example.org Next,word", "want":{"V018":[{"range":[25,26]}]}},
  {"name":"V018/formatted_URL", "raw":"https<em>:</em>//example.org", "want":{"V018":[]}},
  {"name":"V018/formatted_email", "raw":"user@example<em>.</em>org", "want":{"V018":[]}},
  {"name":"V003_V017/educational_blank", "raw":"I'm from . . . in _____.", "tag":"instr", "want":{"V003":[{"range":[10,15]}],"V017":[]}},
  {"name":"V003_V017/formatted_spaced_ellipsis", "raw":". <em>.</em> .", "want":{"V003":[{"range":[1,15]}],"V017":[]}},
  {"name":"V003_V017/formatted_blank", "raw":"Wait <em>. . .</em>", "want":{"V003":[{"range":[10,15]}],"V017":[]}},
  {"name":"V003_V017/ordinary_period", "raw":"Wait .", "want":{"V003":[],"V017":[{"range":[5,6]}]}},
  {"name":"V003_V017/four_dots", "raw":"Wait . . . .", "want":{"V003":[{"range":[6,13]}],"V017":[{"range":[5,6]}]}},
  {"name":"V003_V017/other_error_remains", "raw":"Hi , I'm . . .", "want":{"V003":[{"range":[10,15]}],"V017":[{"range":[3,4]}]}},
  {"name":"V007/formatted_list", "raw":"<em>1)</em> Read", "want":{"V007":[]}},
  {"name":"V007/list_and_unmatched_bracket", "raw":"<em>a)</em> Read (carefully", "want":{"V007":[{"range":[18,19]}]}},
  {"name":"V007/list_requires_space", "raw":"<em>1)</em>Read", "want":{"V007":[{"range":[6,7]}]}},
  {"name":"V013/formatted_hyphen", "raw":"Москва <em>-</em> столица", "want":{"V013":[{"range":[12,13]}]}},
  {"name":"V013/word_hyphen", "raw":"северо<em>-</em>западный", "want":{"V013":[]}},
  {"name":"V014/formatted_dash", "raw":"северо<strong>–</strong>западный", "want":{"V014":[{"range":[15,16]}]}},
  {"name":"V014/single_letters", "raw":"a<em>—</em>b", "want":{"V014":[]}},
  {"name":"V022/split_word", "raw":"he<em>11</em>o", "want":{"V022":[{"range":[1,15]}]}},
  {"name":"V022/unsupported_HTML", "raw":"he<b>11</b>o", "want":{"V022":[]}},
  {"name":"V022/literal_context", "raw":"he<em>11</em>o", "tag":"editor", "want":{"V022":[]}},
  {"name":"V023/split_number", "raw":"2<em>O</em>05", "want":{"V023":[{"range":[1,14]}]}},
  {"name":"V023/digit_required", "raw":"O<em>O</em>", "want":{"V023":[]}},
  {"name":"V024/base_before_closing_tag", "raw":"<em>a</em>́", "want":{"V024":[]}},
  {"name":"V024/mark_after_opening_tag", "raw":"a<em>́</em>", "want":{"V024":[]}},
  {"name":"V024/digit_is_not_base", "raw":"<em>1</em>́", "want":{"V024":[{"range":[11,12]}]}},
  {"name":"V026/split_repeat", "raw":"Hello!<em>!</em>", "want":{"V026":[{"range":[6,12]}]}},
  {"name":"V026/three_dots", "raw":".<em>.</em>.", "want":{"V026":[],"V003":[]}},
  {"name":"V026/different_marks", "raw":"!<em>?</em>", "want":{"V026":[]}},
  {"name":"V027/formatted_quotes", "raw":"<em>'Hello'</em>", "want":{"V027":[{"range":[5,6],"related":[[11,12]]}]}},
  {"name":"V027/apostrophe", "raw":"We<em>'</em>re here", "want":{"V027":[]}},
  {"name":"V028/formatted_space", "raw":"(<em> word</em>)", "want":{"V028":[{"range":[6,7]}]}},
  {"name":"V028/empty_brackets", "raw":"(<em> </em>)", "want":{"V028":[]}},
  {"name":"V028/split_spaces", "raw":"( <em> </em>word)", "want":{"V028":[{"range":[2,3]},{"range":[7,8]}]}},
  {"name":"V029/formatted_gap", "raw":"1<em> – </em>2", "want":{"V029":[{"range":[6,9]}]}},
  {"name":"V029/no_space", "raw":"1<em>–</em>2", "want":{"V029":[]}},
  {"name":"V029/three_parts", "raw":"1 <em>–</em> 2 – 3", "want":{"V029":[]}},
  {"name":"V003/escaped_marker", "raw":". \\<em>.</em> .", "want":{"V003":[]}},
  {"name":"V003/literal_context", "raw":". <em>.</em> .", "tag":"editor", "want":{"V003":[]}},
  {"name":"V023/literal_context", "raw":"2<em>O</em>05", "tag":"editor", "want":{"V023":[]}},
  {"name":"V024/literal_context", "raw":"a<em>́</em>", "tag":"editor", "want":{"V024":[{"range":[6,7]}]}},
  {"name":"V024/split_orphan_marks", "raw":"1́<em>̀</em>", "want":{"V024":[{"range":[2,3]},{"range":[7,8]}]}},
  {"name":"V026/inline_br_is_literal", "raw":"!<br>!", "want":{"V026":[]}},
  {"name":"V026/escaped_marker", "raw":"!\\<em>!</em>", "want":{"V026":[]}},
  {"name":"V026/empty_formatting", "raw":"!<em></em>!", "want":{"V026":[{"range":[1,12]}]}},
  {"name":"V027/unsupported_HTML", "raw":"<b>'Hello'</b>", "want":{"V027":[]}},
  {"name":"V027/Unicode_related_columns", "raw":"Я😀 <em>'Hi'</em>", "want":{"V027":[{"range":[8,9],"related":[[11,12]]}]}},
  {"name":"V028/literal_context", "raw":"(<em> word</em>)", "tag":"editor", "want":{"V028":[]}},
  {"name":"V029/literal_context", "raw":"1<em> – </em>2", "tag":"editor", "want":{"V029":[]}}
]
`)
	var cases []struct {
		Name, Raw, Tag string
		Want           map[string][]visibleExpectation
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tag := tc.Tag
			if tag == "" {
				tag = "text"
			}
			result := v021Block(t, tag, v021Text(t, tc.Raw, 3, 2))
			for _, source := range []string{"first", "second"} {
				// Запускается весь каталог, с контролем модели и истории.
				v021Run(t, result, source, nil)
				for code, want := range tc.Want {
					visibleExpect(t, result, source, code, want)
				}
			}
		})
	}
}

// visibleExpect проверяет число сообщений, метаданные и все исходные диапазоны.
func visibleExpect(t *testing.T, result *model.Result, source, code string, want []visibleExpectation) {
	t.Helper()
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if string(entry.DiagnosticCode) == code && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%s: got %d findings, want %d: %+v", code, len(got), len(want), got)
	}
	definitions, err := v021Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		if string(definition.Code) != code {
			continue
		}
		for i, entry := range got {
			if entry.Location == nil || *entry.Location != visibleLocation(want[i].Range) ||
				string(entry.SeverityLevel) != string(definition.Severity) ||
				string(entry.DiagnosticScope) != string(definition.Scope) || entry.Fatal ||
				(entry.Message != definition.Message && !strings.HasPrefix(entry.Message, definition.Message+"\nконкретизация: ")) {
				t.Errorf("%s occurrence %d: %+v, location %+v; want %+v", code, i, entry, entry.Location, want[i])
			}
			if len(entry.RelatedLocations) != len(want[i].Related) {
				t.Fatalf("%s related = %+v, want %+v", code, entry.RelatedLocations, want[i].Related)
			}
			for j, related := range want[i].Related {
				if entry.RelatedLocations[j] != visibleLocation(related) {
					t.Errorf("%s related %d = %+v, want %+v", code, j, entry.RelatedLocations[j], related)
				}
			}
		}
		return
	}
	t.Fatalf("unknown diagnostic %s", code)
}

// visibleLocation возвращает диапазон строки содержимого тестового блока.
func visibleLocation(columns [2]int) model.Location {
	return model.Location{Start: model.Position{Line: 4, Column: columns[0]}, End: model.Position{Line: 4, Column: columns[1]}}
}

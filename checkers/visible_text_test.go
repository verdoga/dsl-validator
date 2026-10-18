package checkers

import (
	"encoding/json"
	"fmt"
	"os"
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
	data, err := os.ReadFile("testdata/visible_text.json")
	if err != nil {
		t.Fatal(err)
	}
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

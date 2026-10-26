package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV075Registration(t *testing.T) {
	definitions, err := v075Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V075" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" ||
			definition.Message != "Внутренний этап после @step не содержит учебного материала." ||
			definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V075 definitions = %d, want 1", count)
	}
}

func TestV075ReferenceAndExample(t *testing.T) {
	// К дословным фрагментам добавлены только @dsl-version 1.2 и # Lesson.
	for _, tc := range []struct {
		name, source string
		filled, eof  bool
		want         [][8]int
	}{
		{"reference", "@task a\n@instr Read the question.\n@step Discuss\nDiscuss your answer.\n@step Finish\n@instr Summarise your answer.\n@endtask", true, false, nil},
		{"example", "@task a\n@instr Read the question.\n@step Discuss\nDiscuss your answer.\n@step Finish\n@endtask", false, false, [][8]int{{7, 1, 7, 13, 8, 1, 8, 9}}},
		{"example at EOF", "@task a\n@instr Read the question.\n@step Discuss\nDiscuss your answer.\n@step Finish", false, true, [][8]int{{7, 1, 7, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "instr", "Read the question.", "content", 3, 2),
				v075TagLine(t, "step", "Discuss", "title", 3, 2),
				v075Text(t, "Discuss your answer.", "Discuss your answer.", 5, 3),
				v075TagLine(t, "step", "Finish", "title", 3, 2)}
			if tc.filled {
				body = append(body, v075TagLine(t, "instr", "Summarise your answer.", "content", 7, 3))
			}
			if !tc.eof {
				body = append(body, v075TagLine(t, "endtask", "", "", 2, 1))
			}
			var raw []string
			for _, line := range body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative example changed")
			}
			result := v075Lesson(t, body...)
			result.Lines[len(result.Lines)-1].LineEnding = ""
			v075Run(t, result, "current", tc.want)
		})
	}
}

func TestV075StageRanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][8]int
	}{
		{"no explicit stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil},
		{"outside task", []model.Line{
			v075TagLine(t, "step", "A", "title", 2, 1), v075Text(t, "Read.", "Read.", 3, 2),
		}, nil},
		{"immediate EOF", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
		}, [][8]int{{4, 1, 4, 8}}},
		{"adjacent stages", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075TagLine(t, "step", "B", "title", 3, 2), v075Text(t, "Read.", "Read.", 5, 3),
		}, [][8]int{{4, 1, 4, 8, 5, 1, 5, 8}}},
		{"two empty stages", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075TagLine(t, "step", "B", "title", 3, 2), v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 4, 8, 5, 1, 5, 8}, {5, 1, 5, 8, 6, 1, 6, 9}}},
		{"zero stage cannot fill named stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075Text(t, "Read.", "Read.", 3, 2),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{5, 1, 5, 8, 6, 1, 6, 9}}},
		{"blank last line included", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075Line(t, "", "blank", 4, 3), v075Line(t, " \t", "blank", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 6, 3, 7, 1, 7, 9}}},
		{"speaking is not material", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075TagLine(t, "speaking", "", "", 3, 2), v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 5, 10, 6, 1, 6, 9}}},
		{"plain text", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075Text(t, "Read.", "Read.", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil},
		{"editor body is excluded", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075Open(t, "editor", 4, 3), v075Text(t, "Read.", "Read.", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}}},
		{"single line editor is excluded", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
			v075TagLine(t, "editor", "Read.", "content", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 5, 14, 6, 1, 6, 9}}},
		{"empty stage title", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075Attempt(t, "@step", "step", 1, 3, 2),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 4, 6, 5, 1, 5, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v075Run(t, v075Lesson(t, tc.body...), "current", tc.want) })
	}
}

func TestV075EveryTaskBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		boundary model.Line
		end      int
	}{
		{"endtask", v075TagLine(t, "endtask", "", "", 2, 1), 9},
		{"header", v075TagLine(t, "header", "Theory", "title", 2, 1), 15},
		{"newpage", v075TagLine(t, "newpage", "", "", 2, 1), 9},
		{"task", v075TagLine(t, "task", "b", "identifier", 2, 1), 8},
		{"heading", v075Line(t, "### Reading", "heading", 0, 0,
			v075ModelElement(t, "heading-level", "###", "3", 1, 4),
			v075ModelElement(t, "title", "Reading", "Reading", 5, 12)), 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), tc.boundary}
			if tc.name == "task" {
				body = append(body, v075TagLine(t, "step", "B", "title", 5, 2), v075Text(t, "Read.", "Read.", 6, 3))
			} else {
				parent := 2
				if tc.name == "heading" {
					parent = 5
				}
				body = append(body, v075Text(t, "Read.", "Read.", parent, 1),
					v075TagLine(t, "step", "Outside", "title", parent, 1))
			}
			v075Run(t, v075Lesson(t, body...), "current", [][8]int{{4, 1, 4, 8, 5, 1, 5, tc.end}})
		})
	}
	for _, name := range []string{"task", "endtask", "header", "newpage", "variants", "variant"} {
		t.Run("nested "+name+" does not close", func(t *testing.T) {
			// Запрещённые структурные объявления в text не создают
			// границу этапа. Реальный текст после них наполняет этап.
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "text", 4, 3),
				v075Attempt(t, "@"+name, name, 1, 5, 4), v075Text(t, "Read.", "Read.", 5, 4),
				v075Close(t, 5, 4), v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
		})
	}
}

func TestV075NestedStepDoesNotChangeStage(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		want      [][8]int
		count     string
		forbidden [8]int
	}{
		{"nested step cannot start the first stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075Open(t, "text", 3, 2),
			v075TagLine(t, "step", "Fake", "title", 4, 3), v075Close(t, 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil, "", [8]int{5, 1, 5, 6, 4, 1, 4, 6}},
		{"empty outer stage retains its full range", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075TagLine(t, "step", "Fake", "title", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}}, "Найдено этапов: 1.", [8]int{6, 1, 6, 6, 5, 1, 5, 6}},
		{"material before nested step stays in the outer stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075Text(t, "Visible material.", "Visible material.", 5, 4),
			v075TagLine(t, "step", "Fake", "title", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil, "Найдено этапов: 1.", [8]int{7, 1, 7, 6, 5, 1, 5, 6}},
		{"material after nested step stays in the outer stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075TagLine(t, "step", "Fake", "title", 5, 4),
			v075Text(t, "Visible material.", "Visible material.", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil, "Найдено этапов: 1.", [8]int{6, 1, 6, 6, 5, 1, 5, 6}},
		{"material before the block stays in the outer stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Text(t, "Read.", "Read.", 4, 3), v075Open(t, "text", 4, 3),
			v075TagLine(t, "step", "Fake", "title", 6, 4), v075Close(t, 6, 4),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil, "Найдено этапов: 1.", [8]int{7, 1, 7, 6, 6, 1, 6, 6}},
		{"material after the block stays in the outer stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075TagLine(t, "step", "Fake", "title", 5, 4), v075Close(t, 5, 4),
			v075Text(t, "Read.", "Read.", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1),
		}, nil, "Найдено этапов: 1.", [8]int{6, 1, 6, 6, 5, 1, 5, 6}},
		{"next real stage still closes an empty stage", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075TagLine(t, "step", "Fake", "title", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "step", "Next", "title", 3, 2), v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 7, 2, 8, 1, 8, 11}, {8, 1, 8, 11, 9, 1, 9, 9}}, "", [8]int{6, 1, 6, 6, 5, 1, 5, 6}},
		{"empty neighbours of a filled real stage are still checked", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Before", "title", 3, 2),
			v075TagLine(t, "step", "Real", "title", 3, 2), v075Open(t, "text", 5, 3),
			v075Text(t, "Read.", "Read.", 6, 4), v075TagLine(t, "step", "Fake", "title", 6, 4), v075Close(t, 6, 4),
			v075TagLine(t, "step", "After", "title", 3, 2), v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 4, 13, 5, 1, 5, 11}, {10, 1, 10, 12, 11, 1, 11, 9}}, "", [8]int{8, 1, 8, 6, 6, 1, 6, 6}},
		{"recovered nested step is also excluded", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075Invalid(t, "\t@StEp Fake", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1),
		}, [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}}, "Найдено этапов: 1.", [8]int{6, 2, 6, 7, 5, 1, 5, 6}},
		{"partially parsed nested step cannot replace the stage at EOF", []model.Line{
			v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "Real", "title", 3, 2),
			v075Open(t, "text", 4, 3), v075Attempt(t, "@step Fake", "step", 1, 5, 4), v075Close(t, 5, 4),
		}, [][8]int{{4, 1, 7, 2}}, "Найдено этапов: 1.", [8]int{6, 1, 6, 6, 5, 1, 5, 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v075Lesson(t, tc.body...)
			v075Run(t, result, "current", tc.want)
			var counts []v075ContextFinding
			if tc.count != "" {
				counts = append(counts, v075ContextFinding{[8]int{3, 1, 3, 6}, tc.count})
			}
			v075CheckContext(t, result, "V068", counts)
			v075CheckContext(t, result, "V076", []v075ContextFinding{
				{tc.forbidden, "Вложенный тег: @step. Родительский блок: @text."},
			})
		})
	}
}

func TestV075NestedStepsInBranches(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run("shared="+strconv.FormatBool(shared), func(t *testing.T) {
			first, second, depth := 4, 13, 3
			if shared {
				first, second, depth = 3, 3, 2
			}
			result := v075Lesson(t, v075Open(t, "variants", 2, 1), v075TagLine(t, "variant", "A", "name", 3, 2),
				v075TagLine(t, "task", "a", "identifier", first, depth), v075TagLine(t, "step", "Real", "title", 5, depth+1),
				v075Open(t, "instr", 6, depth+2), v075Open(t, "hint", 7, depth+3),
				v075Attempt(t, "@step Fake", "step", 1, 8, depth+4), v075Close(t, 8, depth+4), v075Close(t, 7, depth+3),
				v075TagLine(t, "step", "Next", "title", 5, depth+1), v075TagLine(t, "variant", "B", "name", 3, 2),
				v075TagLine(t, "task", "b", "identifier", second, depth), v075TagLine(t, "step", "Other", "title", 14, depth+1),
				v075Close(t, 3, 2))
			v075Run(t, result, "current", [][8]int{
				{6, 1, 11, 2, 12, 1, 12, 11}, {12, 1, 12, 11, 13, 1, 13, 11}, {15, 1, 15, 12, 16, 1, 16, 2},
			})
			v075CheckContext(t, result, "V068", []v075ContextFinding{{[8]int{14, 1, 14, 6}, "Найдено этапов: 1."}})
			v075CheckContext(t, result, "V076", []v075ContextFinding{
				{[8]int{9, 1, 9, 6, 8, 1, 8, 6}, "Вложенный тег: @step. Родительский блок: @hint."},
			})
		})
	}
}

func TestV075NestedStepsInOtherBlocks(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		want      [][8]int
		forbidden []v075ContextFinding
	}{
		{"example block cannot contain a structural step", []model.Line{
			v075Open(t, "example", 4, 3), v075Attempt(t, "@step Fake", "step", 1, 5, 4), v075Close(t, 5, 4),
		}, [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}}, []v075ContextFinding{
			{[8]int{6, 1, 6, 6, 5, 1, 5, 9}, "Вложенный тег: @step. Родительский блок: @example."},
		}},
		{"interactive block cannot contain a structural step", []model.Line{
			v075Open(t, "choice", 4, 3), v075Attempt(t, "@step Fake", "step", 1, 5, 4), v075Close(t, 5, 4),
		}, [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}}, nil},
		{"unknown block cannot replace a filled outer stage", []model.Line{
			v075Text(t, "Read.", "Read.", 4, 3),
			v075Line(t, "@unknown {", "block-start", 4, 3,
				v075ModelElement(t, "unparsed", "@unknown", "", 1, 9), v075ModelElement(t, "block-open", "{", "{", 10, 11)),
			v075Attempt(t, "@step Fake", "step", 1, 6, 4), v075Close(t, 6, 4),
		}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "Real", "title", 3, 2)}, tc.body...)
			body = append(body, v075TagLine(t, "endtask", "", "", 2, 1))
			result := v075Lesson(t, body...)
			v075Run(t, result, "current", tc.want)
			v075CheckContext(t, result, "V068", []v075ContextFinding{{[8]int{3, 1, 3, 6}, "Найдено этапов: 1."}})
			v075CheckContext(t, result, "V076", tc.forbidden)
		})
	}
}

// v075ContextFinding задаёт независимые диапазоны и конкретизацию соседней проверки.
type v075ContextFinding struct {
	position [8]int
	detail   string
}

// v075CheckContext сверяет V068/V076 того же полного запуска с ожиданиями теста.
func v075CheckContext(t *testing.T, result *model.Result, code diagnostics.Code, want []v075ContextFinding) {
	t.Helper()
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == "current" && entry.DiagnosticCode == code {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%s count = %d, want %d: %+v", code, len(got), len(want), got)
	}
	message := "В задании меньше двух внутренних этапов @step.\nконкретизация: "
	if code == "V076" {
		message = "Вложенный тег запрещён в этом блоке.\nконкретизация: "
	}
	for i, expected := range want {
		entry, p := got[i], expected.position
		location := model.Location{Start: model.Position{Line: p[0], Column: p[1]}, End: model.Position{Line: p[2], Column: p[3]}}
		var related []model.Location
		if p[4] != 0 {
			related = append(related, model.Location{Start: model.Position{Line: p[4], Column: p[5]}, End: model.Position{Line: p[6], Column: p[7]}})
		}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(related) ||
			len(related) == 1 && entry.RelatedLocations[0] != related[0] || entry.ID == "" || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != message+expected.detail {
			t.Fatalf("%s occurrence %d = %+v; want %+v, related %+v and exact metadata", code, i, entry, location, related)
		}
	}
}

func TestV075ContentTags(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  int
	}{
		{"instr", 7}, {"note", 6}, {"hint", 6}, {"example", 9},
		{"wordlist", 10}, {"alt", 5}, {"question", 10},
	} {
		for _, filled := range []bool{false, true} {
			t.Run("inline/"+tc.name+"/filled="+strconv.FormatBool(filled), func(t *testing.T) {
				line := v075Attempt(t, "@"+tc.name, tc.name, 1, 4, 3)
				want := [][8]int{{4, 1, 5, tc.end, 6, 1, 6, 9}}
				if filled {
					line = v075TagLine(t, tc.name, "Read.", "content", 4, 3)
					want = nil
				}
				v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
					v075TagLine(t, "step", "A", "title", 3, 2), line, v075TagLine(t, "endtask", "", "", 2, 1)),
					"current", want)
			})
		}
	}
	for _, name := range []string{"instr", "note", "hint", "example", "wordlist", "table", "script", "text", "key", "alt"} {
		for _, filled := range []bool{false, true} {
			t.Run("block/"+name+"/filled="+strconv.FormatBool(filled), func(t *testing.T) {
				content := v075Text(t, "\u00a0", "\u00a0", 5, 4)
				want := [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}}
				if filled {
					content = v075Text(t, "Read.", "Read.", 5, 4)
					want = nil
				}
				// Script без media допустим как самостоятельный текст (3.5.7).
				v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
					v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, name, 4, 3), content,
					v075Close(t, 5, 4), v075TagLine(t, "endtask", "", "", 2, 1)), "current", want)
			})
		}
	}
	t.Run("block title is not content", func(t *testing.T) {
		open := v075Line(t, "@text Title {", "block-start", 4, 3,
			v075ModelElement(t, "tag", "@text", "text", 1, 6),
			v075ModelElement(t, "title", "Title", "Title", 7, 12), v075ModelElement(t, "block-open", "{", "{", 13, 14))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), open, v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1)), "current", [][8]int{{4, 1, 6, 2, 7, 1, 7, 9}})
	})
	for _, name := range []string{"example", "wordlist", "table", "script", "text", "key", "note", "alt"} {
		t.Run("unparsed block title/"+name, func(t *testing.T) {
			// По формату этих блоков единственное поле перед { — TITLE.
			// Табуляция перед скобкой нарушает DSL 1.29: парсер сохранил
			// название в unparsed, но Raw всё ещё позволяет отличить его
			// от содержимого. V075 запрещает считать название материалом.
			open := v075Line(t, "@"+name+" Title\t{", "block-start", 4, 3,
				v075ModelElement(t, "tag", "@"+name, name, 1, len(name)+2),
				v075ModelElement(t, "unparsed", " Title\t", "", len(name)+2, len(name)+9),
				v075ModelElement(t, "block-open", "{", "{", len(name)+9, len(name)+10))
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), open, v075Close(t, 5, 4),
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", [][8]int{{4, 1, 6, 2, 7, 1, 7, 9}})
		})
		t.Run("unparsed text after opening brace/"+name, func(t *testing.T) {
			// DSL 1.18 запрещает текст после {. Однако доступный текст
			// нельзя отбросить и объявить весь этап заведомо пустым.
			open := v075Line(t, "@"+name+" { Read.", "block-start", 4, 3,
				v075ModelElement(t, "tag", "@"+name, name, 1, len(name)+2),
				v075ModelElement(t, "block-open", "{", "{", len(name)+3, len(name)+4),
				v075ModelElement(t, "unparsed", " Read.", "", len(name)+4, len(name)+10))
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), open, v075Close(t, 5, 4),
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
		})
	}
	t.Run("separator is not material", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "example", 4, 3),
			v075Line(t, "---", "separator", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1)), "current", [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}})
	})
	t.Run("nested hint is material", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "instr", 4, 3),
			v075Open(t, "hint", 5, 4), v075Text(t, "Read.", "Read.", 6, 5),
			v075Close(t, 6, 5), v075Close(t, 5, 4), v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
	})
}

func TestV075InteractiveMaterial(t *testing.T) {
	for _, name := range []string{"choice", "multichoice", "ordering", "matching"} {
		t.Run(name, func(t *testing.T) {
			body := []model.Line{v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, name, 4, 3)}
			if name == "matching" {
				body = append(body, v075Text(t, "Capital of France", "Capital of France", 5, 4),
					v075Line(t, "---", "separator", 5, 4))
			}
			body = append(body, v075Text(t, "Paris", "Paris", 5, 4), v075Text(t, "Rome", "Rome", 5, 4),
				v075Close(t, 5, 4), v075TagLine(t, "endtask", "", "", 2, 1))
			v075Run(t, v075Lesson(t, body...), "current", nil)
		})
	}
	t.Run("answer content is material", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "multichoice", 4, 3),
			v075Open(t, "answer", 5, 4), v075Text(t, "Paris", "Paris", 6, 5),
			v075Text(t, "Rome", "Rome", 6, 5), v075Close(t, 6, 5),
			v075Close(t, 5, 4), v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
	})
	t.Run("bare multifill has no content", func(t *testing.T) {
		// DSL 4.4.4 допускает пустое поле; V075 строже и требует материал.
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "multifill", "", "", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1)), "current", [][8]int{{4, 1, 5, 11, 6, 1, 6, 9}})
	})
	t.Run("multifill instruction is content", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "multifill", "Read.", "content", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
	})
	for _, raw := range []string{"@step Fake", "@endtask", "@editor Comment.", "---", "_____"} {
		t.Run("literal multifill "+raw, func(t *testing.T) {
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "multifill", 4, 3),
				v075Text(t, raw, raw, 5, 4), v075Close(t, 5, 4),
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
		})
	}
}

func TestV075PlaceholderContent(t *testing.T) {
	for _, tc := range []struct{ name, raw, value string }{
		{"ordinary placeholder", "_____", "_____"},
		{"single answer", "_____{yes}", "_____{yes}"},
		{"alternative answers", "_____{yes}{no}", "_____{yes}{no}"},
		{"empty answer", "_____{}", "_____{}"},
		{"empty alternative", "_____{yes}{}", "_____{yes}{}"},
		{"with surrounding text", "Say _____{yes}.", "Say _____{yes}."},
		{"escaped literal", `\_____\{yes\}`, "_____{yes}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// По уточнению пользователя парсер сохраняет всю запись в content.
			// Value содержит текст, а не выделенный ответ. Ошибочный ответ
			// не делает сам видимый пропуск отсутствующим материалом.
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "multifill", 4, 3),
				v075Text(t, tc.raw, tc.value, 5, 4), v075Close(t, 5, 4),
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
		})
	}
}

func TestV075ServiceFieldsAreNotMaterial(t *testing.T) {
	for _, tc := range []struct {
		tag, argument string
		kind          model.ElementType
		end           int
	}{
		{"dsl-version", "1.2", "version", 17},
		{"document-id", "a", "identifier", 15},
		{"section", "A", "name", 11},
		{"order", "1", "number", 9},
		{"resource-dir", "assets", "resource-path", 21},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			// Неверное размещение служебного объявления — отдельная
			// проблема; разобранные метаданные не являются материалом.
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2),
				v075TagLine(t, tc.tag, tc.argument, tc.kind, 4, 3),
				v075TagLine(t, "endtask", "", "", 2, 1)),
				"current", [][8]int{{4, 1, 5, tc.end, 6, 1, 6, 9}})
		})
	}
}

func TestV075MediaAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want [][8]int
	}{
		{"@media\taudio sound.mp3", nil},
		{"@media\taudio \"путь к файлу.mp3\"", nil},
		{"@media\timage \"a\\\"b.png\"", nil},
		{"@media", [][8]int{{4, 1, 5, 7, 6, 1, 6, 9}}},
		{"@media audio", [][8]int{{4, 1, 5, 13, 6, 1, 6, 9}}},
		{`@media audio ""`, [][8]int{{4, 1, 5, 16, 6, 1, 6, 9}}},
		{`@media audio "unfinished`, nil},
		{`@media audio "a.mp3" extra`, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			line := v075Attempt(t, tc.raw, "media", 1, 4, 3)
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), line, v075TagLine(t, "endtask", "", "", 2, 1)),
				"current", tc.want)
		})
	}
	t.Run("parsed media", func(t *testing.T) {
		line := v075Line(t, "@media AUDIO sound.mp3", "tag", 4, 3,
			v075ModelElement(t, "tag", "@media", "media", 1, 7),
			v075ModelElement(t, "media-type", "AUDIO", "audio", 8, 13),
			v075ModelElement(t, "source", "sound.mp3", "sound.mp3", 14, 23))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), line, v075TagLine(t, "endtask", "", "", 2, 1)),
			"current", nil)
	})
	for _, tc := range []struct {
		raw  string
		want [][8]int
	}{
		{"@instr\tRead.", nil}, {"Read. \\", nil}, {"@unknown", nil},
		{"@instr", [][8]int{{4, 1, 5, 7, 6, 1, 6, 9}}},
		{"@instr\t\u00a0", [][8]int{{4, 1, 5, 9, 6, 1, 6, 9}}},
	} {
		t.Run("unparsed "+strconv.Quote(tc.raw), func(t *testing.T) {
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075Invalid(t, tc.raw, 4, 3),
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", tc.want)
		})
	}
	t.Run("unclosed block is uncertain", func(t *testing.T) {
		open := v075Open(t, "text", 4, 3)
		open.Elements = append(open.Elements, v075ModelElement(t, "unparsed", "", "", 8, 8))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), open), "current", nil)
	})
	t.Run("known text alongside an error", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Invalid(t, "@unknown", 4, 3),
			v075Text(t, "Read.", "Read.", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
	})
	t.Run("unparsed text after closing brace", func(t *testing.T) {
		close := v075Line(t, "} Read.", "block-end", 5, 4,
			v075ModelElement(t, "block-close", "}", "}", 1, 2),
			v075ModelElement(t, "unparsed", " Read.", "", 2, 8))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "text", 4, 3), close,
			v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
	})
}

func TestV075Fragments(t *testing.T) {
	t.Run("unused definition is not material", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075FragmentLine(t, 4, 3),
			v075Text(t, "Read.", "Read.", 5, 4), v075Close(t, 5, 4),
			v075TagLine(t, "endtask", "", "", 2, 1)), "current", [][8]int{{4, 1, 7, 2, 8, 1, 8, 9}})
	})
	for _, before := range []bool{false, true} {
		for _, filled := range []bool{false, true} {
			t.Run("before="+strconv.FormatBool(before)+"/filled="+strconv.FormatBool(filled), func(t *testing.T) {
				// Одно определение находится до либо после include.
				// Регистр F/f различается, Unicode White_Space не материал.
				text := "\u00a0"
				if filled {
					text = "Read."
				}
				body := []model.Line{
					v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2),
					v075TagLine(t, "include", "F", "identifier", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1),
					v075FragmentLine(t, 2, 1), v075Text(t, text, text, 7, 2), v075Close(t, 7, 2),
				}
				want := [][8]int{{4, 1, 5, 11, 6, 1, 6, 9}}
				if before {
					body = []model.Line{
						v075FragmentLine(t, 2, 1), v075Text(t, text, text, 3, 2), v075Close(t, 3, 2),
						v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 6, 2),
						v075TagLine(t, "include", "F", "identifier", 7, 3), v075TagLine(t, "endtask", "", "", 2, 1),
					}
					want = [][8]int{{7, 1, 8, 11, 9, 1, 9, 9}}
				}
				if filled {
					want = nil
				}
				v075Run(t, v075Lesson(t, body...), "current", want)
			})
		}
	}
	t.Run("empty definition", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "include", "f", "identifier", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1), v075FragmentLine(t, 2, 1), v075Close(t, 7, 2)),
			"current", [][8]int{{4, 1, 5, 11, 6, 1, 6, 9}})
	})
	for _, duplicate := range []bool{false, true} {
		t.Run("unresolved/duplicate="+strconv.FormatBool(duplicate), func(t *testing.T) {
			body := []model.Line{v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "include", "f", "identifier", 4, 3),
				v075TagLine(t, "endtask", "", "", 2, 1)}
			if duplicate {
				second := v075Line(t, "@fragment F {", "block-start", 2, 1,
					v075ModelElement(t, "tag", "@fragment", "fragment", 1, 10),
					v075ModelElement(t, "identifier", "F", "F", 11, 12),
					v075ModelElement(t, "block-open", "{", "{", 13, 14))
				// Даже два пустых определения не дают права выбрать одно.
				body = append(body, v075FragmentLine(t, 2, 1), v075Close(t, 7, 2), second, v075Close(t, 9, 2))
			}
			v075Run(t, v075Lesson(t, body...), "current", nil)
		})
	}
	t.Run("included nested content", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "include", "f", "identifier", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1), v075FragmentLine(t, 2, 1),
			v075Open(t, "text", 7, 2), v075Text(t, "Read.", "Read.", 8, 3), v075Close(t, 8, 3),
			v075Close(t, 7, 2)), "current", nil)
	})
	t.Run("unparsed include ID and inserted text", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Attempt(t, "@include\tF", "include", 1, 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1), v075FragmentLine(t, 2, 1),
			v075Invalid(t, "@instr\tRead.", 7, 2), v075Close(t, 7, 2)), "current", nil)
	})
	t.Run("invalid recursive include is uncertain", func(t *testing.T) {
		// DSL 3.12.12 запрещает include внутри fragment. Сохранённая
		// ошибочная вставка не раскрывается бесконечно и не доказывает пустоту.
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "include", "f", "identifier", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1), v075FragmentLine(t, 2, 1),
			v075Attempt(t, "@include f", "include", 1, 7, 2), v075Close(t, 7, 2)), "current", nil)
	})
	t.Run("parsed include ID does not hide an unparsed argument", func(t *testing.T) {
		include := v075Line(t, "@include f extra", "invalid", 4, 3,
			v075ModelElement(t, "tag", "@include", "include", 1, 9),
			v075ModelElement(t, "identifier", "f", "f", 10, 11),
			v075ModelElement(t, "unparsed", " extra", "", 11, 17))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), include,
			v075TagLine(t, "endtask", "", "", 2, 1), v075FragmentLine(t, 2, 1), v075Close(t, 7, 2)),
			"current", nil)
	})
	for _, tc := range []struct {
		name  string
		open  model.Line
		close model.Line
	}{
		{"after opening brace", v075Line(t, "@fragment f { Read.", "block-start", 2, 1,
			v075ModelElement(t, "tag", "@fragment", "fragment", 1, 10),
			v075ModelElement(t, "identifier", "f", "f", 11, 12),
			v075ModelElement(t, "block-open", "{", "{", 13, 14),
			v075ModelElement(t, "unparsed", " Read.", "", 14, 20)), v075Close(t, 7, 2)},
		{"extra parameter", v075Line(t, "@fragment f extra {", "block-start", 2, 1,
			v075ModelElement(t, "tag", "@fragment", "fragment", 1, 10),
			v075ModelElement(t, "identifier", "f", "f", 11, 12),
			v075ModelElement(t, "unparsed", " extra ", "", 12, 19),
			v075ModelElement(t, "block-open", "{", "{", 19, 20)), v075Close(t, 7, 2)},
		{"after closing brace", v075FragmentLine(t, 2, 1), v075Line(t, "} Read.", "block-end", 7, 2,
			v075ModelElement(t, "block-close", "}", "}", 1, 2),
			v075ModelElement(t, "unparsed", " Read.", "", 2, 8))},
	} {
		t.Run("unparsed fragment "+tc.name, func(t *testing.T) {
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "include", "f", "identifier", 4, 3),
				v075TagLine(t, "endtask", "", "", 2, 1), tc.open, tc.close), "current", nil)
		})
	}
	t.Run("recovered Unicode ID resolves an empty definition", func(t *testing.T) {
		// Ошибочные разделители не препятствуют чтению единственного ID;
		// unparsed сам по себе не делает заведомо пустую вставку неопределённой.
		fragment := v075Line(t, "@fragment\tф {", "block-start", 2, 1,
			v075ModelElement(t, "tag", "@fragment", "fragment", 1, 10),
			v075ModelElement(t, "unparsed", "\tф ", "", 10, 13),
			v075ModelElement(t, "block-open", "{", "{", 13, 14))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Invalid(t, "@INCLUDE\tФ", 4, 3),
			v075TagLine(t, "endtask", "", "", 2, 1), fragment, v075Close(t, 7, 2)),
			"current", [][8]int{{4, 1, 5, 11, 6, 1, 6, 9}})
	})
	t.Run("unparsed whitespace around a parsed ID is not uncertainty", func(t *testing.T) {
		include := v075Line(t, "@include\tf", "invalid", 4, 3,
			v075ModelElement(t, "tag", "@include", "include", 1, 9),
			v075ModelElement(t, "unparsed", "\t", "", 9, 10),
			v075ModelElement(t, "identifier", "f", "f", 10, 11))
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), include,
			v075TagLine(t, "endtask", "", "", 2, 1), v075FragmentLine(t, 2, 1), v075Close(t, 7, 2)),
			"current", [][8]int{{4, 1, 5, 11, 6, 1, 6, 9}})
	})
	t.Run("repeated empty include is not a cycle", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075TagLine(t, "include", "f", "identifier", 4, 3),
			v075TagLine(t, "include", "F", "identifier", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1),
			v075FragmentLine(t, 2, 1), v075Close(t, 8, 2)),
			"current", [][8]int{{4, 1, 6, 11, 7, 1, 7, 9}})
	})
}

func TestV075Branches(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run("shared="+strconv.FormatBool(shared), func(t *testing.T) {
			first, second, depth := 4, 7, 3
			if shared {
				first, second, depth = 3, 3, 2
			}
			v075Run(t, v075Lesson(t, v075Open(t, "variants", 2, 1),
				v075TagLine(t, "variant", "A", "name", 3, 2),
				v075TagLine(t, "task", "a", "identifier", first, depth),
				v075TagLine(t, "step", "A", "title", 5, depth+1),
				v075TagLine(t, "variant", "B", "name", 3, 2),
				v075TagLine(t, "task", "b", "identifier", second, depth),
				v075TagLine(t, "step", "B", "title", 8, depth+1), v075Close(t, 3, 2)),
				"current", [][8]int{{6, 1, 6, 8, 7, 1, 7, 11}, {9, 1, 9, 8, 10, 1, 10, 2}})
		})
	}
	t.Run("variants closes external stage and does not resume it", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "outer", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Open(t, "variants", 2, 1),
			v075TagLine(t, "variant", "A", "name", 5, 2), v075Text(t, "Read.", "Read.", 6, 3),
			v075TagLine(t, "variant", "B", "name", 5, 2), v075Text(t, "Write.", "Write.", 8, 3),
			v075Close(t, 5, 2), v075TagLine(t, "step", "Outside", "title", 2, 1)),
			"current", [][8]int{{4, 1, 4, 8, 5, 1, 5, 12}})
	})
	t.Run("literal editor cannot switch branch or stage", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075Open(t, "variants", 2, 1), v075TagLine(t, "variant", "A", "name", 3, 2),
			v075TagLine(t, "task", "a", "identifier", 4, 3), v075TagLine(t, "step", "A", "title", 5, 4),
			v075Open(t, "editor", 6, 5), v075Text(t, "@variant B", "@variant B", 7, 6),
			v075Text(t, "@step B", "@step B", 7, 6), v075Close(t, 7, 6),
			v075TagLine(t, "variant", "B", "name", 3, 2), v075Text(t, "Write.", "Write.", 11, 3),
			v075Close(t, 3, 2)), "current", [][8]int{{6, 1, 10, 2, 11, 1, 11, 11}})
	})
}

func TestV075WhitespacePositionsAndRuns(t *testing.T) {
	// White_Space задан явно, независимо от проверяемой реализации.
	for _, space := range []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(strconv.QuoteRune(space), func(t *testing.T) {
			content := v075Text(t, string(space), string(space), 4, 3)
			if space == '\t' || space == ' ' {
				content = v075Line(t, string(space), "blank", 4, 3)
			}
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), content,
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", [][8]int{{4, 1, 5, 2, 6, 1, 6, 9}})
		})
	}
	for _, text := range []string{"\u200b", "\ufeff", "Привет 😀", `\@step Fake`} {
		t.Run(strconv.Quote(text), func(t *testing.T) {
			value := text
			if text == `\@step Fake` {
				value = "@step Fake"
			}
			v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
				v075TagLine(t, "step", "A", "title", 3, 2), v075Text(t, text, value, 4, 3),
				v075TagLine(t, "endtask", "", "", 2, 1)), "current", nil)
		})
	}
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				step := v075Line(t, " \t@StEp A", "tag", 3, 2,
					v075ModelElement(t, "tag", "@StEp", "step", 3, 8),
					v075ModelElement(t, "title", "A", "A", 9, 10))
				result := v075Lesson(t, v075TagLine(t, "TaSk", "a", "identifier", 2, 1), step)
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[3].LineEnding = ""
				}
				v075Run(t, result, "current", [][8]int{{4, 3, 4, 10}})
			})
		}
	}
	t.Run("recovered task and step", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075Invalid(t, "@TASK", 2, 1),
			v075Invalid(t, " \t@STEP", 2, 1)), "current", [][8]int{{4, 3, 4, 8}})
	})
	t.Run("longer task name does not open a task", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075Invalid(t, "@taskExtra a", 2, 1),
			v075TagLine(t, "step", "Outside", "title", 2, 1)), "current", nil)
	})
	t.Run("longer step name does not open another stage", func(t *testing.T) {
		v075Run(t, v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1),
			v075TagLine(t, "step", "A", "title", 3, 2), v075Text(t, "Read.", "Read.", 4, 3),
			v075Invalid(t, "@stepExtra B", 4, 3), v075TagLine(t, "endtask", "", "", 2, 1)),
			"current", nil)
	})
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run("empty/"+version, func(t *testing.T) {
			result := v075Lesson(t)
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			result.Document.Metadata.Title = nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v075Run(t, result, "current", nil)
				return
			}
			before := v075Snapshot(t, result)
			checks, err := validator.Prepare(result, v075Registry(t))
			if err == nil || checks != nil || v075Snapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
	first := v075Lesson(t, v075TagLine(t, "task", "a", "identifier", 2, 1), v075TagLine(t, "step", "A", "title", 3, 2))
	second := v075Lesson(t, v075TagLine(t, "step", "Outside", "title", 2, 1))
	v075Run(t, first, "first", [][8]int{{4, 1, 4, 8}})
	v075Run(t, second, "first", nil)
	v075Run(t, first, "second", [][8]int{{4, 1, 4, 8}})
}

// v075FragmentLine задаёт определение f; его содержимое и закрытие задаются отдельно.
func v075FragmentLine(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v075Line(t, "@fragment f {", "block-start", parent, depth,
		v075ModelElement(t, "tag", "@fragment", "fragment", 1, 10),
		v075ModelElement(t, "identifier", "f", "f", 11, 12), v075ModelElement(t, "block-open", "{", "{", 13, 14))
}

// v075Attempt задаёт готовый tag и неразобранный остаток ошибочной строки.
func v075Attempt(t *testing.T, raw, name string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + len(name) + 1
	return v075Line(t, raw, "invalid", parent, depth,
		v075ModelElement(t, "tag", string(runes[start-1:end-1]), name, start, end),
		v075ModelElement(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v075Lesson добавляет обязательный заголовок; body начинается со строки 3.
func v075Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v075Document(t, append([]model.Line{v075Heading(t)}, body...)...)
}

// v075Document добавляет версию перед готовыми строками, содержащими # Lesson.
// Для явно заданных unparsed создаёт согласованные исходные ошибки и ссылки.
func v075Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v075TagLine(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v075Heading задаёт обязательный заголовок первого уровня.
func v075Heading(t *testing.T) model.Line {
	t.Helper()
	return v075Line(t, "# Lesson", "heading", 0, 0,
		v075ModelElement(t, "heading-level", "#", "1", 1, 2),
		v075ModelElement(t, "title", "Lesson", "Lesson", 3, 9))
}

// v075Invalid задаёт целиком неразобранную строку без элемента tag.
func v075Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v075Line(t, raw, "invalid", parent, depth, v075ModelElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v075Text задаёт Raw и семантическое значение независимо.
func v075Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v075Line(t, raw, "content", parent, depth, v075ModelElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v075TagLine задаёт известный однострочный тег с необязательным аргументом.
func v075TagLine(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v075ModelElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v075ModelElement(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v075Line(t, raw, "tag", parent, depth, elements...)
}

// v075Open задаёт блок с каноническим семантическим именем тега.
func v075Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v075Line(t, "@"+tag+" {", "block-start", parent, depth,
		v075ModelElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v075ModelElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v075Close задаёт закрытие блока с явным родителем.
func v075Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v075Line(t, "}", "block-end", parent, depth, v075ModelElement(t, "block-close", "}", "}", 1, 2))
}

// v075Line сохраняет явно заданную структуру без классификации Raw.
func v075Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v075ModelElement задаёт исходные Unicode-колонки с исключающим концом.
func v075ModelElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v075Registry создаёт реестр из полного штатного каталога.
func v075Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v075Run проверяет новые V075 по явным основным и связанным диапазонам.
// Конвейер выполняет полный каталог; результаты других диагностик не отбираются.
func v075Run(t *testing.T, result *model.Result, source string, want [][8]int) {
	t.Helper()
	before := v075Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v075Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v075Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V075" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V075 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[2], Column: position[3]}}
		related := []model.Location{}
		if position[4] != 0 {
			related = append(related, model.Location{Start: model.Position{Line: position[4], Column: position[5]}, End: model.Position{Line: position[6], Column: position[7]}})
		}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(related) ||
			len(related) == 1 && entry.RelatedLocations[0] != related[0] || entry.ID == "" || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "block" || entry.Fatal || entry.Message != "Внутренний этап после @step не содержит учебного материала." {
			t.Fatalf("occurrence %d = %+v; want %+v, related %+v and exact V075 metadata", i, entry, location, related)
		}
	}

	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v075Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// CheckModel проверил производные флаги/ссылки. В копии восстанавливаем
	// только их и убираем новые записи для сравнения всего исходника и истории.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v075Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v075Snapshot сохраняет независимое представление модели для сравнения.
func v075Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v075CacheDefinition задаёт явные связи и содержимое тестового фрагмента.
// Пустой text означает строку NBSP, не являющуюся учебным материалом по V075.
type v075CacheDefinition struct {
	id   string
	refs []string
	text string
}

// v075CacheLesson создаёт по одному этапу на ссылку, затем определения.
// Вложенные include запрещены DSL 3.12.12: они сохранены как ошибочные строки
// с tag/unparsed, как в существующем тесте рекурсивной вставки. Это вход
// валидатора, а не попытка разрешить такую структуру в DSL.
func v075CacheLesson(t *testing.T, refs []string, definitions []v075CacheDefinition) *model.Result {
	t.Helper()
	body := []model.Line{v075TagLine(t, "task", "a", "identifier", 2, 1)}
	for _, id := range refs {
		step := len(body) + 3
		body = append(body, v075TagLine(t, "step", "S", "title", 3, 2), v075TagLine(t, "include", id, "identifier", step, 3))
	}
	body = append(body, v075TagLine(t, "endtask", "", "", 2, 1))
	for _, definition := range definitions {
		opening := len(body) + 3
		width := utf8.RuneCountInString(definition.id)
		raw := "@fragment " + definition.id + " {"
		body = append(body, v075Line(t, raw, "block-start", 2, 1,
			v075ModelElement(t, "tag", "@fragment", "fragment", 1, 10),
			v075ModelElement(t, "identifier", definition.id, definition.id, 11, 11+width),
			v075ModelElement(t, "block-open", "{", "{", 12+width, 13+width)))
		for _, id := range definition.refs {
			body = append(body, v075Attempt(t, "@include "+id, "include", 1, opening, 2))
		}
		text := definition.text
		if text == "" {
			text = "\u00a0"
		}
		body = append(body, v075Text(t, text, text, opening, 2), v075Close(t, opening, 2))
	}
	return v075Lesson(t, body...)
}

func TestV075CachePathsAndCycles(t *testing.T) {
	for _, tc := range []struct {
		name        string
		refs        []string
		definitions []v075CacheDefinition
		want        [][8]int
	}{
		{"shared empty suffix and separate stage ranges", []string{"a", "c", "a"}, []v075CacheDefinition{
			{"a", []string{"b", "c"}, ""}, {"b", []string{"d"}, ""}, {"c", []string{"d"}, ""}, {"d", nil, ""}},
			[][8]int{{4, 1, 5, 11, 6, 1, 6, 8}, {6, 1, 7, 11, 8, 1, 8, 8}, {8, 1, 9, 11, 10, 1, 10, 9}}},
		{"cycle without material is not empty", []string{"a", "b", "a"}, []v075CacheDefinition{
			{"a", []string{"b"}, ""}, {"b", []string{"a"}, ""}}, nil},
		{"cross edge to pending branch without material", []string{"a", "c", "b"}, []v075CacheDefinition{
			{"a", []string{"b", "c"}, ""}, {"b", []string{"a"}, ""}, {"c", []string{"b"}, ""}}, nil},
		{"self cycle is not empty", []string{"a", "a"}, []v075CacheDefinition{{"a", []string{"a"}, ""}}, nil},
		{"material after back edge is reachable from every member", []string{"a", "b", "c"}, []v075CacheDefinition{
			{"a", []string{"b", "c"}, "Read."}, {"b", []string{"a"}, ""}, {"c", []string{"b"}, ""}}, nil},
		{"same cycle entered through another member", []string{"c", "b", "a"}, []v075CacheDefinition{
			{"a", []string{"b", "c"}, "Read."}, {"b", []string{"a"}, ""}, {"c", []string{"b"}, ""}}, nil},
		{"completed suffix shared by two cycles", []string{"a", "c", "b", "d"}, []v075CacheDefinition{
			{"a", []string{"b", "e"}, ""}, {"b", []string{"a"}, ""}, {"c", []string{"d", "e"}, ""}, {"d", []string{"c"}, ""}, {"e", nil, "Read."}}, nil},
		{"cycle uncertainty does not contaminate empty fragment", []string{"a", "e"}, []v075CacheDefinition{
			{"a", []string{"a", "e"}, ""}, {"e", nil, ""}}, [][8]int{{6, 1, 7, 11, 8, 1, 8, 9}}},
		{"unresolved reference does not contaminate empty fragment", []string{"a", "e", "a"}, []v075CacheDefinition{
			{"a", []string{"missing", "e"}, ""}, {"e", nil, ""}}, [][8]int{{6, 1, 7, 11, 8, 1, 8, 8}}},
		{"case insensitive reuse and different definition order", []string{"a", "A", "b"}, []v075CacheDefinition{
			{"b", []string{"A"}, ""}, {"a", nil, ""}}, [][8]int{{4, 1, 5, 11, 6, 1, 6, 8}, {6, 1, 7, 11, 8, 1, 8, 8}, {8, 1, 9, 11, 10, 1, 10, 9}}},
		{"ambiguous ID cannot use a unique neighbour cache", []string{"e", "a", "A"}, []v075CacheDefinition{
			{"e", nil, ""}, {"a", nil, ""}, {"A", nil, "Read."}}, [][8]int{{4, 1, 5, 11, 6, 1, 6, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v075Run(t, v075CacheLesson(t, tc.refs, tc.definitions), "current", tc.want) })
	}
}

func TestV075CacheDoesNotBypassReferenceValidation(t *testing.T) {
	for _, badFirst := range []bool{false, true} {
		t.Run("badFirst="+strconv.FormatBool(badFirst), func(t *testing.T) {
			result := v075CacheLesson(t, []string{"f", "f"}, []v075CacheDefinition{{"f", nil, ""}})
			number, parent := 7, 6
			want := [][8]int{{4, 1, 5, 11, 6, 1, 6, 8}}
			if badFirst {
				number, parent = 5, 4
				want = [][8]int{{6, 1, 7, 11, 8, 1, 8, 9}}
			}
			// Сохраняем tag, ID и ошибочный хвост; существующий сборщик создаёт ErrorIDs.
			bad := v075Line(t, "@include f extra", "invalid", parent, 3,
				v075ModelElement(t, "tag", "@include", "include", 1, 9), v075ModelElement(t, "identifier", "f", "f", 10, 11),
				v075ModelElement(t, "unparsed", " extra", "", 11, 17))
			body := append([]model.Line{}, result.Lines[2:]...)
			body[number-3] = bad
			v075Run(t, v075Lesson(t, body...), "current", want)
		})
	}
}

func TestV075CacheSharedChains(t *testing.T) {
	// 32 уровня с двумя одинаковыми ссылками на следующий фрагмент. Размер
	// документа линейный; обход всех путей без кеша потребовал бы 2^32 раскрытий.
	// Время не сравнивается с порогом, чтобы тест не зависел от скорости машины.
	for _, tail := range []string{"empty", "unresolved", "cycle", "cycle with material"} {
		t.Run(tail, func(t *testing.T) {
			var definitions []v075CacheDefinition
			for i := 0; i < 32; i++ {
				id, next := "f"+strconv.Itoa(i), "f"+strconv.Itoa(i+1)
				definitions = append(definitions, v075CacheDefinition{id, []string{next, next}, ""})
			}
			last := v075CacheDefinition{id: "f32"}
			var want [][8]int
			switch tail {
			case "empty":
				want = [][8]int{{4, 1, 5, 12, 6, 1, 6, 8}, {6, 1, 7, 12, 8, 1, 8, 9}}
			case "unresolved":
				last.refs = []string{"missing"}
			case "cycle":
				last.refs = []string{"f32"}
			case "cycle with material":
				last.refs = []string{"f0"}
				last.text = "Read."
			}
			definitions = append(definitions, last)
			v075Run(t, v075CacheLesson(t, []string{"f0", "F0"}, definitions), "current", want)
		})
	}
}

func TestV075CacheIsLocalToRun(t *testing.T) {
	empty := v075CacheLesson(t, []string{"a", "a"}, []v075CacheDefinition{{"a", nil, ""}})
	filled := v075CacheLesson(t, []string{"a", "a"}, []v075CacheDefinition{{"a", nil, "Read."}})
	uncertain := v075CacheLesson(t, []string{"a", "a"}, []v075CacheDefinition{{"a", []string{"a"}, ""}})
	want := [][8]int{{4, 1, 5, 11, 6, 1, 6, 8}, {6, 1, 7, 11, 8, 1, 8, 9}}
	v075Run(t, filled, "first", nil)
	v075Run(t, empty, "first", want)
	v075Run(t, uncertain, "first", nil)
	v075Run(t, empty, "second", want)
	// Те же строки и ID в повторном запуске, но материал уже изменён редактором.
	// Raw и Value изменены согласованно; тело необходимо исследовать заново.
	filled.Lines[9].Raw = "\u00a0"
	filled.Lines[9].Elements = []model.Element{v075ModelElement(t, "content", "\u00a0", "\u00a0", 1, 2)}
	v075Run(t, filled, "after-edit", want)
}

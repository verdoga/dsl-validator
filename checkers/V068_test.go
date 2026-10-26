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

// v068Expected задаёт независимо ожидаемые позиции и точную конкретизацию.
type v068Expected struct {
	line, start, end int
	detail           string
}

func TestV068Registration(t *testing.T) {
	definitions, err := v068TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V068" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "В задании меньше двух внутренних этапов @step." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) ||
			definition.Check == nil {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V068 definitions = %d, want 1", count)
	}
}

func TestV068ReferenceAndExample(t *testing.T) {
	// По DSL 1.3, 1.30 и 2.1.3 добавлены только версия и # Lesson.
	// Сами фрагменты из таблицы сохранены дословно.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v068Expected
	}{
		{"reference", "@task practice\n@step Read\nRead.\n@step Write\nWrite.", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestText(t, "Read.", "Read.", 4, 3),
			v068TestTag(t, "step", "title", "Write", 3, 2), v068TestText(t, "Write.", "Write.", 6, 3),
		}, nil},
		{"example", "@task practice\n@step Read\nRead the text.", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestText(t, "Read the text.", "Read the text.", 4, 3),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v068TestDocument(t, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v068TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV068CountsAndTaskBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v068Expected
	}{
		{"no tasks", []model.Line{v068TestText(t, "Read.", "Read.", 2, 1)}, nil},
		{"zero steps", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"content is not a step", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestText(t, "Read.", "Read.", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"task at EOF", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
		}, nil},
		{"two steps", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestTag(t, "step", "title", "Write", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"three steps", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestTag(t, "step", "title", "Write", 3, 2),
			v068TestTag(t, "step", "title", "Check", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"same names count independently", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"case insensitive names", []model.Line{
			v068TestTag(t, "TASK", "identifier", "practice", 2, 1),
			v068TestTag(t, "STEP", "title", "Read", 3, 2), v068TestTag(t, "Step", "title", "Write", 3, 2),
			v068TestTag(t, "ENDTASK", "", "", 3, 2),
		}, nil},
		{"next task resets count", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "task", "identifier", "other", 2, 1), v068TestTag(t, "endtask", "", "", 5, 2),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"later steps do not fill previous task", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "task", "identifier", "other", 2, 1),
			v068TestTag(t, "step", "title", "Read", 5, 2), v068TestTag(t, "step", "title", "Write", 5, 2),
			v068TestTag(t, "endtask", "", "", 5, 2),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"earlier steps do not fill next task", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestTag(t, "step", "title", "Write", 3, 2),
			v068TestTag(t, "task", "identifier", "other", 2, 1),
			v068TestTag(t, "step", "title", "Check", 6, 2), v068TestTag(t, "endtask", "", "", 6, 2),
		}, []v068Expected{{6, 1, 6, "Найдено этапов: 1."}}},
		{"header ends task", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "header", "title", "Theory", 2, 1),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"newpage ends task", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "newpage", "", "", 0, 0),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"heading three ends task", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestHeading(t, "", "###", "3", "Next"),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"content and nested close do not reset count", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestLine(t, "", "blank", 4, 3),
			v068TestTag(t, "editor", "content", "Check.", 4, 3), v068TestOpen(t, "text", 4, 3),
			v068TestText(t, "Read.", "Read.", 7, 4), v068TestClose(t, 7, 4),
			v068TestTag(t, "step", "title", "Write", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"original range with indentation and Unicode ID", []model.Line{
			v068TestLine(t, " \t@task учёба ", "tag", 2, 1,
				v068TestElement(t, "tag", "@task", "task", 3, 8),
				v068TestElement(t, "identifier", "учёба", "учёба", 9, 14)),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2),
		}, []v068Expected{{3, 3, 8, "Найдено этапов: 1."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v068TestRun(t, v068TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("step after endtask is outside task", func(t *testing.T) {
		result := v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2),
			v068TestTag(t, "step", "title", "Write", 2, 1))
		v068TestParserError(t, result, 6, "Этап находится вне задания.")
		v068TestRun(t, result, "current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
}

func TestV068Branches(t *testing.T) {
	t.Run("variants ends outer task", func(t *testing.T) {
		v068TestRun(t, v068TestDocument(t,
			v068TestTag(t, "task", "identifier", "practice", 2, 1), v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestOpen(t, "variants", 2, 1),
			v068TestTag(t, "variant", "name", "A", 5, 2), v068TestText(t, "Read.", "Read.", 6, 3),
			v068TestTag(t, "variant", "name", "B", 5, 2), v068TestText(t, "Write.", "Write.", 8, 3),
			v068TestClose(t, 5, 2)), "current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
	for _, commonParent := range []bool{false, true} {
		t.Run("common parent "+strconv.FormatBool(commonParent), func(t *testing.T) {
			parentA, parentB, depth := 4, 7, 3
			if commonParent {
				parentA, parentB, depth = 3, 3, 2
			}
			t.Run("one and two steps in separate branches", func(t *testing.T) {
				v068TestRun(t, v068TestDocument(t, v068TestOpen(t, "variants", 2, 1),
					v068TestTag(t, "variant", "name", "A", 3, 2),
					v068TestTag(t, "task", "identifier", "practice", parentA, depth),
					v068TestTag(t, "step", "title", "Read", 5, depth+1),
					v068TestTag(t, "variant", "name", "B", 3, 2),
					v068TestTag(t, "task", "identifier", "other", parentB, depth),
					v068TestTag(t, "step", "title", "Read", 8, depth+1),
					v068TestTag(t, "step", "title", "Write", 8, depth+1), v068TestClose(t, 3, 2)),
					"current", []v068Expected{{5, 1, 6, "Найдено этапов: 1."}})
			})
			t.Run("closing variants and subsequent task", func(t *testing.T) {
				branchParent := 6
				if commonParent {
					branchParent = 3
				}
				v068TestRun(t, v068TestDocument(t, v068TestOpen(t, "variants", 2, 1),
					v068TestTag(t, "variant", "name", "A", 3, 2), v068TestText(t, "Read.", "Read.", 4, 3),
					v068TestTag(t, "variant", "name", "B", 3, 2),
					v068TestTag(t, "task", "identifier", "practice", branchParent, depth),
					v068TestTag(t, "step", "title", "Read", 7, depth+1), v068TestClose(t, 3, 2),
					v068TestTag(t, "task", "identifier", "other", 2, 1), v068TestTag(t, "endtask", "", "", 10, 2)),
					"current", []v068Expected{{7, 1, 6, "Найдено этапов: 1."}})
			})
		})
	}
}

func TestV068StepsAfterTerminatorAreExcluded(t *testing.T) {
	for _, tc := range []struct {
		name       string
		terminator model.Line
		parent     int
		depth      int
	}{
		{"header", v068TestTag(t, "header", "title", "Theory", 2, 1), 5, 2},
		{"newpage", v068TestTag(t, "newpage", "", "", 0, 0), 5, 1},
		{"heading", v068TestHeading(t, "", "###", "3", "Next"), 5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "title", "Read", 3, 2), tc.terminator,
				v068TestTag(t, "step", "title", "Write", tc.parent, tc.depth))
			v068TestParserError(t, result, 6, "Этап находится вне задания.")
			v068TestRun(t, result, "current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
		})
	}
	t.Run("next variant ends task even without subsequent task", func(t *testing.T) {
		result := v068TestDocument(t, v068TestOpen(t, "variants", 2, 1),
			v068TestTag(t, "variant", "name", "A", 3, 2),
			v068TestTag(t, "task", "identifier", "practice", 4, 3),
			v068TestTag(t, "step", "title", "Read", 5, 4),
			v068TestTag(t, "variant", "name", "B", 3, 2),
			v068TestTag(t, "step", "title", "Write", 7, 3),
			v068TestText(t, "Write.", "Write.", 8, 4), v068TestClose(t, 3, 2))
		v068TestParserError(t, result, 8, "Этап находится вне задания.")
		v068TestRun(t, result, "current", []v068Expected{{5, 1, 6, "Найдено этапов: 1."}})
	})
	t.Run("closing variants excludes following step", func(t *testing.T) {
		result := v068TestDocument(t, v068TestOpen(t, "variants", 2, 1),
			v068TestTag(t, "variant", "name", "A", 3, 2), v068TestText(t, "Read.", "Read.", 4, 3),
			v068TestTag(t, "variant", "name", "B", 3, 2),
			v068TestTag(t, "task", "identifier", "practice", 6, 3),
			v068TestTag(t, "step", "title", "Read", 7, 4), v068TestClose(t, 3, 2),
			v068TestTag(t, "step", "title", "Write", 2, 1))
		v068TestParserError(t, result, 10, "Этап находится вне задания.")
		v068TestRun(t, result, "current", []v068Expected{{7, 1, 6, "Найдено этапов: 1."}})
	})
}

func TestV068ExcludedBlocks(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "instr", "hint", "text"} {
		t.Run(tag, func(t *testing.T) {
			step := v068TestTag(t, "step", "title", "Hidden", 5, 4)
			literal := tag == "editor" || tag == "multifill"
			if literal {
				step = v068TestText(t, "@step Hidden", "@step Hidden", 5, 4)
			}
			result := v068TestDocument(t,
				v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "title", "Read", 3, 2), v068TestOpen(t, tag, 4, 3),
				step, v068TestClose(t, 5, 4), v068TestTag(t, "endtask", "", "", 3, 2))
			if !literal {
				v068TestParserError(t, result, 6, "Структурный тег запрещён внутри контентного блока.")
			}
			v068TestRun(t, result, "current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
		})
	}
	t.Run("raw recovery is excluded in content block", func(t *testing.T) {
		v068TestRun(t, v068TestDocument(t,
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2), v068TestOpen(t, "instr", 4, 3),
			v068TestInvalid(t, "@step", 5, 4), v068TestClose(t, 5, 4),
			v068TestTag(t, "endtask", "", "", 3, 2)), "current",
			[]v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
	t.Run("nested task and terminator are not structural", func(t *testing.T) {
		result := v068TestDocument(t,
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestOpen(t, "instr", 4, 3), v068TestOpen(t, "hint", 5, 4),
			v068TestTag(t, "task", "identifier", "inner", 6, 5),
			v068TestTag(t, "endtask", "", "", 7, 6),
			v068TestClose(t, 6, 5), v068TestClose(t, 5, 4),
			v068TestTag(t, "step", "title", "Write", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2))
		v068TestParserError(t, result, 7, "Структурный тег запрещён внутри hint.")
		v068TestParserError(t, result, 8, "Структурный тег запрещён внутри hint.")
		v068TestRun(t, result, "current", nil)
	})
	t.Run("fragment and include do not supply steps", func(t *testing.T) {
		result := v068TestDocument(t,
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestFragment(t, "f", 4, 3), v068TestTag(t, "step", "title", "Hidden", 5, 4),
			v068TestClose(t, 5, 4), v068TestTag(t, "include", "identifier", "f", 4, 3),
			v068TestTag(t, "endtask", "", "", 3, 2))
		v068TestParserError(t, result, 6, "Структурный тег запрещён внутри fragment.")
		v068TestRun(t, result, "current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
	t.Run("single line editor cannot supply step", func(t *testing.T) {
		v068TestRun(t, v068TestDocument(t,
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "editor", "content", "@step Hidden", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2)), "current",
			nil)
	})
}

func TestV068RawRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v068Expected
	}{
		{"missing title", "@step", nil},
		{"space before malformed title", "@step Read }", nil},
		{"tab separator", "@step\tWrite", nil},
		{"leading whitespace and case", " \t@STEP", nil},
		{"trailing tab", "@step\t", nil},
		{"brace", "@step{", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"NBSP separator", "@step\u00a0Write", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"em space separator", "@step\u2003Write", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"longer name", "@steps", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"numeric suffix", "@step2", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"hyphen suffix", "@step-more", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"Unicode suffix", "@stepЯ", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"unknown declaration", "@unknown", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Тип invalid и unparsed заданы явно; начальное имя не вычисляет
			// тестовый помощник. Это полная ошибочная строка без элемента tag.
			v068TestRun(t, v068TestDocument(t,
				v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "title", "Read", 3, 2),
				v068TestInvalid(t, tc.raw, 4, 3), v068TestTag(t, "endtask", "", "", 3, 2)),
				"current", tc.want)
		})
	}
	t.Run("single recovered step reports one", func(t *testing.T) {
		v068TestRun(t, v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestInvalid(t, "@step", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2)),
			"current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
	t.Run("two recovered steps", func(t *testing.T) {
		v068TestRun(t, v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestInvalid(t, "@step", 3, 2), v068TestInvalid(t, "@step\tWrite", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2)), "current", nil)
	})
	t.Run("unparsed with tag line type", func(t *testing.T) {
		v068TestRun(t, v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestLine(t, "@step", "tag", 3, 2, v068TestUnparsed(t, "@step", 1, 6)),
			v068TestTag(t, "endtask", "", "", 3, 2)), "current",
			[]v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
}

func TestV068RecognizedErroneousDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v068Expected
	}{
		{"one recognized step with remainder counts once", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestBroken(t, v068TestTag(t, "step", "title", "Read", 3, 2), "}"),
			v068TestTag(t, "endtask", "", "", 3, 2),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"recognized step and recovered step", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestBroken(t, v068TestTag(t, "step", "title", "Read", 3, 2), "}"),
			v068TestInvalid(t, "@step", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2),
		}, nil},
		{"recognized endtask still terminates", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestBroken(t, v068TestTag(t, "endtask", "", "", 3, 2), "extra"),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"unknown interval does not suppress finding", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestInvalid(t, "@unknown", 3, 2), v068TestTag(t, "endtask", "", "", 3, 2),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
		{"recognized task without ID still opens area", []model.Line{
			v068TestBroken(t, v068TestTag(t, "task", "", "", 2, 1), "?"),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "endtask", "", "", 3, 2),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v068TestRun(t, v068TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, two := range []bool{false, true} {
		t.Run("missing titles two="+strconv.FormatBool(two), func(t *testing.T) {
			body := []model.Line{v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "", "", 3, 2)}
			want := []v068Expected{{3, 1, 6, "Найдено этапов: 1."}}
			if two {
				body = append(body, v068TestTag(t, "step", "", "", 3, 2))
				want = nil
			}
			body = append(body, v068TestTag(t, "endtask", "", "", 3, 2))
			result := v068TestDocument(t, body...)
			v068TestParserError(t, result, 4, "Название этапа отсутствует.")
			if two {
				v068TestParserError(t, result, 5, "Название этапа отсутствует.")
			}
			v068TestRun(t, result, "current", want)
		})
	}
	t.Run("invalid line retains recognized tag", func(t *testing.T) {
		step := v068TestBroken(t, v068TestTag(t, "step", "title", "Read", 3, 2), "}")
		step.LineType = "invalid"
		v068TestRun(t, v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
			step, v068TestTag(t, "endtask", "", "", 3, 2)),
			"current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
	})
}

func TestV068LiteralText(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@step Write`, "@step Write"}, {`\\@step Write`, `\@step Write`},
		{"Read @step Write", "Read @step Write"}, {"\u00a0@step Write", "\u00a0@step Write"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v068TestRun(t, v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "title", "Read", 3, 2),
				v068TestText(t, tc.raw, tc.value, 4, 3), v068TestTag(t, "endtask", "", "", 3, 2)),
				"current", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
		})
	}
}

func TestV068EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		endings []model.LineEnding
	}{
		{"LF", []model.LineEnding{"\n", "\n", "\n", "\n", "\n"}},
		{"CRLF", []model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n", "\r\n"}},
		{"mixed", []model.LineEnding{"\r\n", "\n", "\r\n", "\n", "\r\n"}},
		{"EOF", []model.LineEnding{"\n", "\n", "\n", "\n", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "title", "Read", 3, 2),
				v068TestText(t, "Read the text.", "Read the text.", 4, 3))
			for i, ending := range tc.endings {
				result.Lines[i].LineEnding = ending
			}
			v068TestRun(t, result, "first", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
			v068TestRun(t, result, "second", []v068Expected{{3, 1, 6, "Найдено этапов: 1."}})
			v068TestRun(t, v068TestDocument(t, v068TestTag(t, "task", "identifier", "practice", 2, 1),
				v068TestTag(t, "step", "title", "Read", 3, 2), v068TestText(t, "Read.", "Read.", 4, 3),
				v068TestTag(t, "step", "title", "Write", 3, 2), v068TestText(t, "Write.", "Write.", 6, 3)),
				"separate", nil)
		})
	}
}

func TestV068EmptyLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		for _, knownCount := range []bool{false, true} {
			t.Run(version+" count known="+strconv.FormatBool(knownCount), func(t *testing.T) {
				result := &model.Result{
					Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version,
						Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
					Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
					Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
				}
				if knownCount {
					count := 0
					result.Document.LineCount = &count
				}
				if version == "1.2" {
					v068TestRun(t, result, "current", nil)
					return
				}
				before := v068TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v068TestRegistry(t))
				if err == nil || checks != nil || v068TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
				}
			})
		}
	}
}

// v068TestFragment задаёт определение с явно указанным ID.
func v068TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v068TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
		v068TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v068TestElement(t, "identifier", id, id, 11, end),
		v068TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v068TestUnparsed задаёт неразобранную часть с отсутствующим Value.
// Документ добавляет требуемую исходную ошибку и ссылку на неё.
func v068TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v068TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v068TestBroken добавляет явно ошибочный остаток к распознанной строке.
func v068TestBroken(t *testing.T, line model.Line, extra string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " " + extra
	line.Elements = append(line.Elements, v068TestUnparsed(t, extra, start, start+utf8.RuneCountInString(extra)))
	return line
}

// v068TestParserError сохраняет ошибку размещения распознанной конструкции,
// не превращая её в unparsed. Это исходные данные, не ожидание результата V068.
func v068TestParserError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1},
			End: model.Position{Line: number, Column: utf8.RuneCountInString(line.Raw) + 1}},
	})
	line.HasErrors, result.Document.HasErrors = true, true
}

// v068TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V068.
func v068TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v068TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v068TestHeading(t, "", "#", "1", title),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	result.Lines[len(result.Lines)-1].LineEnding = ""
	return result
}

// v068TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v068TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v068TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v068TestElement(t, "heading-level", marker, level, start, end),
		v068TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v068TestInvalid задаёт ошибочную строку без распознанного heading-level.
func v068TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v068TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v068TestLine(t, raw, "invalid", parent, depth, element)
}

// v068TestTag собирает однострочный тег с одним явно типизированным параметром.
func v068TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v068TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v068TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v068TestLine(t, raw, "tag", parent, depth, elements...)
}

// v068TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v068TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v068TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v068TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v068TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v068TestClose задаёт закрытие блока с явным родителем.
func v068TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v068TestLine(t, "}", "block-end", parent, depth, v068TestElement(t, "block-close", "}", "}", 1, 2))
}

// v068TestText сохраняет независимо заданные исходное и семантическое значения.
func v068TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v068TestLine(t, raw, "content", parent, depth,
		v068TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v068TestLine не классифицирует исходник и не вычисляет родительские связи.
func v068TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v068TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v068TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v068TestRegistry использует штатный каталог целиком.
func v068TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v068TestRun проверяет только новые V068 по явно заданным диапазонам:
// строка, включающая колонка начала, исключающая колонка конца и конкретизация.
func v068TestRun(t *testing.T, result *model.Result, processingID string, want []v068Expected) {
	t.Helper()
	before := v068TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v068TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v068TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V068" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V068 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "В задании меньше двух внутренних этапов @step.\nконкретизация: "+expected.detail {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V068 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v068TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 ||
		len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
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
	if v068TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v068TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v068TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

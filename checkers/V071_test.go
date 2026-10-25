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

func TestV071Registration(t *testing.T) {
	definitions, err := v071Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V071" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Перед @speaking внутри задания есть другая значимая конструкция." ||
			definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V071 definitions = %d, want 1", count)
	}
}

func TestV071ReferenceAndExample(t *testing.T) {
	// DSL 1.30: добавлены только @dsl-version 1.2 и # Lesson.
	// Оба семистрочных фрагмента сохраняются дословно.
	for _, example := range []bool{false, true} {
		t.Run("example="+strconv.FormatBool(example), func(t *testing.T) {
			speaking := v071TagLine(t, "speaking", "", "", 3, 2)
			instruction := v071TagLine(t, "instr", "Discuss.", "content", 3, 2)
			body := []model.Line{v071TagLine(t, "task", "discussion", "identifier", 2, 1), speaking, instruction}
			var want [][6]int
			if example {
				body[1], body[2] = instruction, speaking
				want = [][6]int{{5, 1, 10, 4, 1, 16}}
			}
			body = append(body, v071TagLine(t, "step", "Prepare", "title", 3, 2),
				v071Text(t, "Read.", "Read.", 6, 3), v071TagLine(t, "step", "Discuss", "title", 3, 2),
				v071Text(t, "Speak.", "Speak.", 8, 3))
			v071Run(t, v071Lesson(t, body...), "current", want)
		})
	}
}

func TestV071FirstObstacle(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"ordinary text", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 3, 2),
			v071TagLine(t, "speaking", "", "", 3, 2),
		}, [][6]int{{5, 1, 10, 4, 1, 6}}},
		{"several obstacles", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 3, 2),
			v071TagLine(t, "instr", "Discuss.", "content", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2),
		}, [][6]int{{6, 1, 10, 4, 1, 6}}},
		{"step is significant", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071TagLine(t, "step", "Read", "title", 3, 2),
			v071Text(t, "Read.", "Read.", 4, 3), v071TagLine(t, "speaking", "", "", 4, 3),
		}, [][6]int{{6, 1, 10, 4, 1, 11}}},
		{"unknown tag is significant", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Invalid(t, "@unknown", 3, 2),
			v071TagLine(t, "speaking", "", "", 3, 2),
		}, [][6]int{{5, 1, 10, 4, 1, 9}}},
		{"first correct then repeated", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071TagLine(t, "speaking", "", "", 3, 2),
			v071Text(t, "Read.", "Read.", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2),
		}, nil},
		{"first misplaced then repeated", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 3, 2),
			v071TagLine(t, "speaking", "", "", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2),
		}, [][6]int{{5, 1, 10, 4, 1, 6}}},
		{"no task", []model.Line{
			v071Text(t, "Read.", "Read.", 2, 1), v071TagLine(t, "speaking", "", "", 2, 1),
		}, nil},
		{"no speaking", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 3, 2),
		}, nil},
		{"speaking before task does not consume first", []model.Line{
			v071TagLine(t, "speaking", "", "", 2, 1), v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Text(t, "Read.", "Read.", 4, 2), v071TagLine(t, "speaking", "", "", 4, 2),
		}, [][6]int{{6, 1, 10, 5, 1, 6}}},
		{"next task clears obstacle", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 3, 2),
			v071TagLine(t, "task", "b", "identifier", 2, 1), v071TagLine(t, "speaking", "", "", 5, 2),
		}, nil},
		{"next task clears first-speaking state", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071TagLine(t, "speaking", "", "", 3, 2),
			v071TagLine(t, "task", "b", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 5, 2),
			v071TagLine(t, "speaking", "", "", 5, 2),
		}, [][6]int{{7, 1, 10, 6, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v071Run(t, v071Lesson(t, tc.body...), "current", tc.want) })
	}
}

func TestV071Comments(t *testing.T) {
	t.Run("blanks and single line comment", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Line(t, "", "blank", 3, 2), v071Line(t, " \t", "blank", 3, 2),
			v071TagLine(t, "EDITOR", "Comment.", "content", 3, 2),
			v071TagLine(t, "speaking", "", "", 3, 2)), "current", nil)
	})
	t.Run("whole block including close", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Open(t, "EdItOr", 3, 2), v071Text(t, "@task fake", "@task fake", 4, 3),
			v071Text(t, "@speaking", "@speaking", 4, 3), v071Close(t, 4, 3),
			v071TagLine(t, "speaking", "", "", 3, 2)), "current", nil)
	})
	t.Run("comment does not erase earlier obstacle", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Text(t, "Read.", "Read.", 3, 2), v071Open(t, "editor", 3, 2),
			v071Text(t, "Comment.", "Comment.", 5, 3), v071Close(t, 5, 3),
			v071TagLine(t, "speaking", "", "", 3, 2)), "current", [][6]int{{8, 1, 10, 4, 1, 6}})
	})
	t.Run("obstacle after ignored comment", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Open(t, "editor", 3, 2), v071Text(t, "Comment.", "Comment.", 4, 3), v071Close(t, 4, 3),
			v071Text(t, "Read.", "Read.", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2)),
			"current", [][6]int{{8, 1, 10, 7, 1, 6}})
	})
	t.Run("escaped comment remains text", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Text(t, `\@editor Comment.`, "@editor Comment.", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2)),
			"current", [][6]int{{5, 1, 10, 4, 1, 18}})
	})
	for _, recognized := range []bool{false, true} {
		t.Run("unparsed editor is not exempt/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
			// DSL 2.8.3: отсутствует обязательное CONTENT.
			comment := v071Invalid(t, "@editor", 3, 2)
			if recognized {
				comment = v071Attempt(t, "@editor", "editor", 1, 3, 2)
			}
			v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
				comment, v071TagLine(t, "speaking", "", "", 3, 2)),
				"current", [][6]int{{5, 1, 10, 4, 1, 8}})
		})
	}
}

func TestV071BlocksAndIncludes(t *testing.T) {
	for _, tc := range []struct {
		block, body string
		end         int
	}{
		{"text", "Read.", 8}, {"multifill", "@speaking", 13},
	} {
		t.Run(tc.block, func(t *testing.T) {
			v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
				v071Open(t, tc.block, 3, 2), v071Text(t, tc.body, tc.body, 4, 3), v071Close(t, 4, 3),
				v071TagLine(t, "speaking", "", "", 3, 2)), "current", [][6]int{{7, 1, 10, 4, 1, tc.end}})
		})
	}
	t.Run("fragment definition is significant", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Fragment(t, 3, 2), v071Text(t, "Read.", "Read.", 4, 3), v071Close(t, 4, 3),
			v071TagLine(t, "speaking", "", "", 3, 2)), "current", [][6]int{{7, 1, 10, 4, 1, 14}})
	})
	t.Run("include is significant without expansion", func(t *testing.T) {
		// DSL 3.12.9: определение доступно до места своего объявления.
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071TagLine(t, "include", "f", "identifier", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2),
			v071Fragment(t, 3, 2), v071Text(t, "Read.", "Read.", 6, 3), v071Close(t, 6, 3)),
			"current", [][6]int{{5, 1, 10, 4, 1, 11}})
	})
	for _, name := range []string{"task", "endtask", "header", "newpage", "variants", "variant"} {
		t.Run("nested "+name+" is not a boundary", func(t *testing.T) {
			v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
				v071Open(t, "text", 3, 2), v071Attempt(t, "@"+name, name, 1, 4, 3),
				v071Close(t, 4, 3), v071TagLine(t, "speaking", "", "", 3, 2)),
				"current", [][6]int{{7, 1, 10, 4, 1, 8}})
		})
	}
	t.Run("nested task does not open a task", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071Open(t, "text", 2, 1),
			v071Attempt(t, "@task a", "task", 1, 3, 2), v071Close(t, 3, 2),
			v071TagLine(t, "speaking", "", "", 2, 1)), "current", nil)
	})
	t.Run("nested heading does not end task", func(t *testing.T) {
		heading := v071Line(t, "### Reading", "invalid", 4, 3,
			v071Element(t, "heading-level", "###", "3", 1, 4),
			v071Element(t, "unparsed", " Reading", "", 4, 12))
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Open(t, "text", 3, 2), heading, v071Close(t, 4, 3),
			v071TagLine(t, "speaking", "", "", 3, 2)), "current", [][6]int{{7, 1, 10, 4, 1, 8}})
	})
	t.Run("forbidden editor cannot hide parent block", func(t *testing.T) {
		comment := v071Open(t, "editor", 4, 3)
		comment.Elements = append(comment.Elements, v071Element(t, "unparsed", "", "", 10, 10))
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Fragment(t, 3, 2), comment, v071Text(t, "Comment.", "Comment.", 5, 4), v071Close(t, 5, 4),
			v071Text(t, "Read.", "Read.", 4, 3), v071Close(t, 4, 3), v071TagLine(t, "speaking", "", "", 3, 2)),
			"current", [][6]int{{10, 1, 10, 4, 1, 14}})
	})
	t.Run("nested speaking does not consume first outer speaking", func(t *testing.T) {
		// DSL 3.0.7 запрещает speaking внутри text. Первым speaking задания
		// по 2.6.5 остаётся внешнее объявление после значимого блока text.
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Open(t, "text", 3, 2), v071Attempt(t, "@speaking", "speaking", 1, 4, 3),
			v071Close(t, 4, 3), v071TagLine(t, "speaking", "", "", 3, 2)),
			"current", [][6]int{{7, 1, 10, 4, 1, 8}})
	})
	t.Run("nested variants do not change outer branch", func(t *testing.T) {
		// Вложенный variants сохранён моделью как настоящий блок с ошибкой
		// размещения. Его границы не заканчивают внешнее задание.
		nested := v071Open(t, "variants", 6, 5)
		nested.Elements = append(nested.Elements, v071Element(t, "unparsed", "", "", 12, 12))
		result := v071Lesson(t, v071Open(t, "variants", 2, 1), v071TagLine(t, "variant", "A", "name", 3, 2),
			v071TagLine(t, "task", "a", "identifier", 4, 3), v071Open(t, "text", 5, 4), nested,
			v071TagLine(t, "variant", "Inner A", "name", 7, 6), v071Text(t, "Read.", "Read.", 8, 7),
			v071TagLine(t, "variant", "Inner B", "name", 7, 6), v071Text(t, "Write.", "Write.", 10, 7),
			v071Close(t, 7, 6), v071Close(t, 6, 5), v071TagLine(t, "speaking", "", "", 5, 4),
			v071TagLine(t, "variant", "B", "name", 3, 2), v071Text(t, "Speak.", "Speak.", 15, 3),
			v071Close(t, 3, 2))
		v071Run(t, result, "current", [][6]int{{14, 1, 10, 6, 1, 8}})
	})
}

func TestV071NestedSpeakingPreservesOuterTask(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       []model.Line
		v071, v072 [][6]int
		v076       [][6]int
	}{
		{"no outer speaking", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Open(t, "text", 3, 2),
			v071TagLine(t, "speaking", "", "", 4, 3), v071Text(t, "Read.", "Read.", 4, 3), v071Close(t, 4, 3),
		}, nil, nil, [][6]int{{5, 1, 10, 4, 1, 6}}},
		{"material before and inside block", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Привет 😀", "Привет 😀", 3, 2),
			v071Open(t, "text", 3, 2), v071Text(t, "Read.", "Read.", 5, 3),
			v071Attempt(t, " \t@SpEaKiNg extra", "speaking", 3, 5, 3),
			v071Text(t, "Discuss.", "Discuss.", 5, 3), v071Close(t, 5, 3),
			v071Line(t, " \t@SpEaKiNg", "tag", 3, 2, v071Element(t, "tag", "@SpEaKiNg", "speaking", 3, 12)),
		}, [][6]int{{10, 3, 12, 4, 1, 9}}, nil, [][6]int{{7, 3, 12, 5, 1, 6}}},
		{"recovered nested speaking before repeated outer", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Open(t, "text", 3, 2),
			v071Invalid(t, " \t@SpEaKiNg", 4, 3), v071Text(t, "Read.", "Read.", 4, 3), v071Close(t, 4, 3),
			v071TagLine(t, "speaking", "", "", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2),
		}, [][6]int{{8, 1, 10, 4, 1, 8}}, [][6]int{{9, 1, 10, 8, 1, 10}}, [][6]int{{5, 3, 12, 4, 1, 6}}},
		{"nested speaking after genuine first", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071TagLine(t, "speaking", "", "", 3, 2),
			v071Open(t, "text", 3, 2), v071TagLine(t, "speaking", "", "", 5, 3), v071Close(t, 5, 3),
			v071TagLine(t, "speaking", "", "", 3, 2),
		}, nil, [][6]int{{8, 1, 10, 4, 1, 10}}, [][6]int{{6, 1, 10, 5, 1, 6}}},
		{"speaking inside permitted instr hint nesting", []model.Line{
			v071TagLine(t, "task", "a", "identifier", 2, 1), v071Open(t, "instr", 3, 2),
			v071Text(t, "Read.", "Read.", 4, 3), v071Open(t, "hint", 4, 3),
			v071TagLine(t, "speaking", "", "", 6, 4), v071Close(t, 6, 4), v071Close(t, 4, 3),
			v071TagLine(t, "speaking", "", "", 3, 2),
		}, [][6]int{{10, 1, 10, 4, 1, 9}}, nil, [][6]int{{7, 1, 10, 6, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v071Lesson(t, tc.body...)
			v071Run(t, result, "current", tc.v071)
			v071OtherFindings(t, result, "V070", nil)
			v071OtherFindings(t, result, "V072", tc.v072)
			v071OtherFindings(t, result, "V076", tc.v076)
		})
	}
}

func TestV071NestedSpeakingInBranches(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run("shared="+strconv.FormatBool(shared), func(t *testing.T) {
			first, second, depth := 4, 18, 3
			if shared {
				first, second, depth = 3, 3, 2
			}
			result := v071Lesson(t,
				v071Open(t, "variants", 2, 1), v071TagLine(t, "variant", "A", "name", 3, 2),
				v071TagLine(t, "task", "a", "identifier", first, depth), v071Open(t, "text", 5, depth+1),
				v071Open(t, "variants", 6, depth+2), v071TagLine(t, "variant", "Inner A", "name", 7, depth+3),
				v071TagLine(t, "task", "nested", "identifier", 8, depth+4),
				v071TagLine(t, "speaking", "", "", 9, depth+5), v071Text(t, "Read.", "Read.", 9, depth+5),
				v071TagLine(t, "variant", "Inner B", "name", 7, depth+3), v071TagLine(t, "speaking", "", "", 12, depth+4),
				v071Close(t, 7, depth+3), v071Close(t, 6, depth+2),
				v071TagLine(t, "speaking", "", "", 5, depth+1), v071TagLine(t, "speaking", "", "", 5, depth+1),
				v071TagLine(t, "variant", "B", "name", 3, 2), v071Open(t, "text", second, depth),
				v071TagLine(t, "speaking", "", "", 19, depth+1), v071Close(t, 19, depth+1),
				v071TagLine(t, "speaking", "", "", second, depth), v071TagLine(t, "task", "b", "identifier", second, depth),
				v071TagLine(t, "speaking", "", "", 23, depth+1), v071Open(t, "text", 23, depth+1),
				v071TagLine(t, "speaking", "", "", 25, depth+2), v071Close(t, 25, depth+2),
				v071TagLine(t, "speaking", "", "", 23, depth+1), v071Close(t, 3, 2),
				v071TagLine(t, "speaking", "", "", 2, 1))
			v071Run(t, result, "current", [][6]int{{16, 1, 10, 6, 1, 8}})
			v071OtherFindings(t, result, "V070", [][6]int{{22, 1, 10, 0, 0, 0}, {30, 1, 10, 0, 0, 0}})
			v071OtherFindings(t, result, "V072", [][6]int{{17, 1, 10, 16, 1, 10}, {28, 1, 10, 24, 1, 10}})
			v071OtherFindings(t, result, "V076", [][6]int{{7, 1, 10, 6, 1, 6}, {20, 1, 10, 19, 1, 6}, {26, 1, 10, 25, 1, 6}})
		})
	}
}

func TestV071Boundaries(t *testing.T) {
	for _, name := range []string{"endtask", "header", "newpage"} {
		for _, recovered := range []bool{false, true} {
			t.Run(name+"/raw="+strconv.FormatBool(recovered), func(t *testing.T) {
				line := v071TagLine(t, name, "", "", 2, 1)
				if name == "header" {
					line = v071TagLine(t, name, "Theory", "title", 2, 1)
				}
				if recovered {
					line = v071Invalid(t, "@"+strings.ToUpper(name)+"\t?", 2, 1)
				}
				v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
					v071Text(t, "Read.", "Read.", 3, 2), line, v071TagLine(t, "speaking", "", "", 2, 1)),
					"current", nil)
			})
		}
	}
	for _, tc := range []struct {
		level string
		want  [][6]int
	}{
		{"1", [][6]int{{5, 1, 10, 4, 1, 10}}},
		{"2", [][6]int{{5, 1, 10, 4, 1, 11}}},
		{"3", nil},
	} {
		t.Run("heading "+tc.level, func(t *testing.T) {
			// Нарушения размещения уровней 1/2 не расширяют установленный
			// V071 перечень границ: задание завершает только уровень 3.
			marker := strings.Repeat("#", int(tc.level[0]-'0'))
			heading := v071Line(t, marker+" Section", "heading", 0, 0,
				v071Element(t, "heading-level", marker, tc.level, 1, len(marker)+1),
				v071Element(t, "title", "Section", "Section", len(marker)+2, len(marker)+9))
			v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
				heading, v071TagLine(t, "speaking", "", "", 4, 1)), "current", tc.want)
		})
	}
}

func TestV071RecoveryAndPositions(t *testing.T) {
	tails := []string{"", " extra", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000", "{", "}"}
	for _, tail := range tails {
		for _, recognized := range []bool{false, true} {
			t.Run(strconv.Quote(tail)+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				first := v071Invalid(t, " \t@SpEaKiNg"+tail, 3, 2)
				if recognized {
					first = v071Attempt(t, " \t@SpEaKiNg"+tail, "speaking", 3, 3, 2)
				}
				body := []model.Line{v071TagLine(t, "task", "a", "identifier", 2, 1),
					v071Text(t, "Read.", "Read.", 3, 2), first}
				if tail == "{" {
					// speaking не имеет блочной формы. Ошибочные скобки
					// сохранены как unparsed, без фиктивного контейнера.
					body = append(body, v071Invalid(t, "}", 3, 2))
				}
				body = append(body, v071TagLine(t, "speaking", "", "", 3, 2))
				v071Run(t, v071Lesson(t, body...), "current", [][6]int{{5, 3, 12, 4, 1, 6}})
			})
		}
	}
	t.Run("ready tag and unparsed on tag line", func(t *testing.T) {
		line := v071Attempt(t, "@speaking extra", "speaking", 1, 3, 2)
		line.LineType = "tag"
		v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
			v071Text(t, "Read.", "Read.", 3, 2), line), "current", [][6]int{{5, 1, 10, 4, 1, 6}})
	})
	for _, raw := range []string{"@speakingExtra", "@speaking1", "@speaking-name", "@speaking.More", "@speaking\u200b"} {
		t.Run(raw, func(t *testing.T) {
			// Более длинное имя само не speaking и не поглощает первое
			// последующее объявление. Препятствие уже есть на строке 4.
			v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
				v071Text(t, "Read.", "Read.", 3, 2), v071Invalid(t, raw, 3, 2),
				v071TagLine(t, "speaking", "", "", 3, 2)), "current", [][6]int{{6, 1, 10, 4, 1, 6}})
		})
	}
	for _, raw := range []string{"@task", "@TaSk\tbad!", "@task{", "@task}"} {
		t.Run("recovered "+raw, func(t *testing.T) {
			body := []model.Line{v071Invalid(t, raw, 2, 1), v071Text(t, "Read.", "Read.", 2, 1),
				v071TagLine(t, "speaking", "", "", 2, 1)}
			if raw == "@task{" {
				body = append(body, v071Invalid(t, "}", 2, 1))
			}
			v071Run(t, v071Lesson(t, body...), "current", [][6]int{{5, 1, 10, 4, 1, 6}})
		})
	}
	t.Run("longer task name does not open", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071Invalid(t, "@taskExtra a", 2, 1),
			v071Text(t, "Read.", "Read.", 2, 1), v071TagLine(t, "speaking", "", "", 2, 1)), "current", nil)
	})
	t.Run("source indentation and semantic whitespace", func(t *testing.T) {
		speaking := v071Line(t, " \t@SpEaKiNg", "tag", 3, 2, v071Element(t, "tag", "@SpEaKiNg", "speaking", 3, 12))
		v071Run(t, v071Lesson(t, v071TagLine(t, "TaSk", "a", "identifier", 2, 1),
			v071Text(t, " \tRead. \t", "Read.", 3, 2), speaking), "current", [][6]int{{5, 3, 12, 4, 1, 10}})
	})
	for _, tc := range []struct {
		raw, value string
		end        int
	}{
		{`\@speaking`, "@speaking", 11}, {"Text @speaking", "Text @speaking", 15},
		{"\u00a0", "\u00a0", 2}, {"\u200b", "\u200b", 2}, {"Привет 😀", "Привет 😀", 9},
	} {
		t.Run("literal "+strconv.Quote(tc.raw), func(t *testing.T) {
			// Unicode White_Space используется только как граница имени.
			// Для blank спецификация удаляет лишь U+0020/U+0009.
			v071Run(t, v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
				v071Text(t, tc.raw, tc.value, 3, 2), v071TagLine(t, "speaking", "", "", 3, 2)),
				"current", [][6]int{{5, 1, 10, 4, 1, tc.end}})
		})
	}
}

func TestV071Branches(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, outer := range []bool{false, true} {
			for _, recovered := range []bool{false, true} {
				t.Run("shared="+strconv.FormatBool(shared)+"/outer="+strconv.FormatBool(outer)+"/raw="+strconv.FormatBool(recovered), func(t *testing.T) {
					// Обе формы связей уже используются штатными тестами:
					// дети variant либо непосредственные дети variants.
					block := 3
					body := []model.Line{}
					want := [][6]int{{7, 1, 10, 6, 1, 13}, {12, 1, 10, 11, 1, 14}}
					if outer {
						block = 5
						body = append(body, v071TagLine(t, "task", "outer", "identifier", 2, 1),
							v071Text(t, "Read.", "Read.", 3, 2))
						want = [][6]int{{9, 1, 10, 8, 1, 13}, {14, 1, 10, 13, 1, 14}}
					}
					firstParent, secondParent, depth := block+1, block+5, 3
					if shared {
						firstParent, secondParent, depth = block, block, 2
					}
					open := v071Open(t, "variants", 2, 1)
					first := v071TagLine(t, "variant", "A", "name", block, 2)
					second := v071TagLine(t, "variant", "B", "name", block, 2)
					if recovered {
						open = v071Line(t, "@VaRiAnTs\t{", "block-start", 2, 1,
							v071Element(t, "unparsed", "@VaRiAnTs\t", "", 1, 11),
							v071Element(t, "block-open", "{", "{", 11, 12))
						first = v071Invalid(t, "@VaRiAnT\tA", block, 2)
						second = v071Invalid(t, "@VARIANT\tB", block, 2)
					}
					body = append(body, open, first,
						v071TagLine(t, "task", "a", "identifier", firstParent, depth),
						v071TagLine(t, "instr", "Read.", "content", block+2, depth+1),
						v071TagLine(t, "speaking", "", "", block+2, depth+1), second,
						v071TagLine(t, "speaking", "", "", secondParent, depth),
						v071TagLine(t, "task", "b", "identifier", secondParent, depth),
						v071TagLine(t, "instr", "Write.", "content", block+7, depth+1),
						v071TagLine(t, "speaking", "", "", block+7, depth+1),
						v071Close(t, block, 2), v071TagLine(t, "speaking", "", "", 2, 1))
					v071Run(t, v071Lesson(t, body...), "current", want)
				})
			}
		}
	}
	t.Run("comment cannot switch branch", func(t *testing.T) {
		v071Run(t, v071Lesson(t, v071Open(t, "variants", 2, 1), v071TagLine(t, "variant", "A", "name", 3, 2),
			v071TagLine(t, "task", "a", "identifier", 4, 3), v071Open(t, "editor", 5, 4),
			v071Text(t, "@variant B", "@variant B", 6, 5), v071Text(t, "@endtask", "@endtask", 6, 5),
			v071Close(t, 6, 5), v071TagLine(t, "instr", "Read.", "content", 5, 4),
			v071TagLine(t, "speaking", "", "", 5, 4),
			v071TagLine(t, "variant", "B", "name", 3, 2), v071Text(t, "Write.", "Write.", 12, 3),
			v071Close(t, 3, 2)), "current", [][6]int{{11, 1, 10, 10, 1, 13}})
	})
}

func TestV071EndingsVersionsAndRuns(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
					v071Text(t, "Read.", "Read.", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[4].LineEnding = ""
				}
				v071Run(t, result, "current", [][6]int{{5, 1, 10, 4, 1, 6}})
			})
		}
	}
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run("empty/"+version, func(t *testing.T) {
			result := v071Lesson(t)
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			result.Document.Metadata.Title = nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v071Run(t, result, "current", nil)
				return
			}
			before := v071Snapshot(t, result)
			checks, err := validator.Prepare(result, v071Registry(t))
			if err == nil || checks != nil || v071Snapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
	// EOF и повторный запуск не переносят active/spoken/препятствие.
	first := v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1), v071Text(t, "Read.", "Read.", 3, 2))
	second := v071Lesson(t, v071TagLine(t, "speaking", "", "", 2, 1))
	third := v071Lesson(t, v071TagLine(t, "task", "a", "identifier", 2, 1),
		v071Text(t, "Read.", "Read.", 3, 2), v071TagLine(t, "speaking", "", "", 3, 2))
	v071Run(t, first, "first", nil)
	v071Run(t, second, "first", nil)
	v071Run(t, third, "first", [][6]int{{5, 1, 10, 4, 1, 6}})
	v071Run(t, third, "second", [][6]int{{5, 1, 10, 4, 1, 6}})
}

// v071Fragment задаёт определение f; его содержимое и закрытие задаются отдельно.
func v071Fragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v071Line(t, "@fragment f {", "block-start", parent, depth,
		v071Element(t, "tag", "@fragment", "fragment", 1, 10),
		v071Element(t, "identifier", "f", "f", 11, 12), v071Element(t, "block-open", "{", "{", 13, 14))
}

// v071Attempt задаёт готовый tag и неразобранный остаток ошибочной строки.
func v071Attempt(t *testing.T, raw, name string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + len(name) + 1
	return v071Line(t, raw, "invalid", parent, depth,
		v071Element(t, "tag", string(runes[start-1:end-1]), name, start, end),
		v071Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v071Lesson добавляет обязательный заголовок; body начинается со строки 3.
func v071Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v071Document(t, append([]model.Line{v071Heading(t)}, body...)...)
}

// v071Document добавляет версию перед готовыми строками, содержащими # Lesson.
// Для явно заданных unparsed создаёт согласованные исходные ошибки и ссылки.
func v071Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v071TagLine(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
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

// v071Heading задаёт обязательный заголовок первого уровня.
func v071Heading(t *testing.T) model.Line {
	t.Helper()
	return v071Line(t, "# Lesson", "heading", 0, 0,
		v071Element(t, "heading-level", "#", "1", 1, 2),
		v071Element(t, "title", "Lesson", "Lesson", 3, 9))
}

// v071Invalid задаёт целиком неразобранную строку без элемента tag.
func v071Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v071Line(t, raw, "invalid", parent, depth, v071Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v071Text задаёт Raw и семантическое значение независимо.
func v071Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v071Line(t, raw, "content", parent, depth, v071Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v071TagLine задаёт известный однострочный тег с необязательным аргументом.
func v071TagLine(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v071Element(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v071Element(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v071Line(t, raw, "tag", parent, depth, elements...)
}

// v071Open задаёт блок с каноническим семантическим именем тега.
func v071Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v071Line(t, "@"+tag+" {", "block-start", parent, depth,
		v071Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v071Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v071Close задаёт закрытие блока с явным родителем.
func v071Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v071Line(t, "}", "block-end", parent, depth, v071Element(t, "block-close", "}", "}", 1, 2))
}

// v071Line сохраняет явно заданную структуру без классификации Raw.
func v071Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v071Element задаёт исходные Unicode-колонки с исключающим концом.
func v071Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v071Registry создаёт реестр из полного штатного каталога.
func v071Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v071Run проверяет новые V071 по явным основным и связанным диапазонам.
// Конвейер выполняет полный каталог; результаты других диагностик не отбираются.
func v071Run(t *testing.T, result *model.Result, source string, want [][6]int) {
	t.Helper()
	before := v071Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v071Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v071Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V071" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V071 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 ||
			entry.RelatedLocations[0] != (model.Location{Start: model.Position{Line: position[3], Column: position[4]}, End: model.Position{Line: position[3], Column: position[5]}}) ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Перед @speaking внутри задания есть другая значимая конструкция." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V071 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v071Snapshot(t, result)), &restored); err != nil {
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
	if v071Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v071OtherFindings сверяет соседние диагностики того же полного запуска.
// Нулевая четвёртая координата означает отсутствие связанных диапазонов.
func v071OtherFindings(t *testing.T, result *model.Result, code diagnostics.Code, want [][6]int) {
	t.Helper()
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == code && entry.Source == "current" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%s count = %d, want %d: %+v", code, len(got), len(want), got)
	}
	for i, position := range want {
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if got[i].Location == nil || *got[i].Location != location {
			t.Fatalf("%s occurrence %d = %+v, want location %+v", code, i, got[i], location)
		}
		if position[3] == 0 {
			if len(got[i].RelatedLocations) != 0 {
				t.Fatalf("%s occurrence %d has unexpected related locations: %+v", code, i, got[i].RelatedLocations)
			}
			continue
		}
		related := model.Location{Start: model.Position{Line: position[3], Column: position[4]}, End: model.Position{Line: position[3], Column: position[5]}}
		if len(got[i].RelatedLocations) != 1 || got[i].RelatedLocations[0] != related {
			t.Fatalf("%s occurrence %d related = %+v, want %+v", code, i, got[i].RelatedLocations, related)
		}
	}
}

// v071Snapshot сохраняет независимое представление модели для сравнения.
func v071Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

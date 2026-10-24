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

func TestV119Registration(t *testing.T) {
	definitions, err := v119TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V119" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "@key находится вне активного задания." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V119 definitions = %d, want 1", count)
	}
}

func TestV119ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Версия и заголовок — единственное дополнение исходного фрагмента.
		result := v119TestDocument(t,
			v119TestNamed(t, "task", "identifier", "reading", 2, 1),
			v119TestNamed(t, "question", "content", "Where does Tom live?", 3, 2),
			v119TestOpen(t, "key", 3, 2), v119TestText(t, "In London.", "In London.", 5, 3),
			v119TestClose(t, 5, 3))
		v119TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@task reading\n@question Where does Tom live?\n@key {\nIn London.\n}")
		v119TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v119TestDocument(t, v119TestNamed(t, "header", "title", "Answers", 2, 1),
			v119TestOpen(t, "key", 2, 1), v119TestText(t, "In London.", "In London.", 4, 2),
			v119TestClose(t, 4, 2))
		v119TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@header Answers\n@key {\nIn London.\n}")
		v119TestRun(t, result, "current", [][3]int{{4, 1, 5}})
	})
}

func TestV119TaskState(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before        []model.Line
		parent, depth int
		want          [][3]int
	}{
		{"no task", nil, 2, 1, [][3]int{{3, 1, 5}}},
		{"active task", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1)}, 3, 2, nil},
		{"next task", []model.Line{v119TestNamed(t, "task", "identifier", "a", 2, 1),
			v119TestNamed(t, "task", "identifier", "b", 2, 1)}, 4, 2, nil},
		{"endtask", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestTag(t, "endtask", 2, 1)}, 2, 1, [][3]int{{5, 1, 5}}},
		{"header", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestNamed(t, "header", "title", "Answers", 2, 1)}, 2, 1, [][3]int{{5, 1, 5}}},
		{"newpage", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestTag(t, "newpage", 2, 1)}, 2, 1, [][3]int{{5, 1, 5}}},
		{"third level heading", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestHeading(t, "###", "Next")}, 4, 1, [][3]int{{5, 1, 5}}},
		{"step preserves task", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestNamed(t, "step", "title", "Read", 3, 2)}, 4, 3, nil},
		{"step alone does not open task", []model.Line{v119TestNamed(t, "step", "title", "Read", 2, 1)},
			3, 2, [][3]int{{4, 1, 5}}},
		{"another step", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestNamed(t, "step", "title", "Read", 3, 2),
			v119TestNamed(t, "step", "title", "Write", 3, 2)}, 5, 3, nil},
		{"new task after reset", []model.Line{v119TestNamed(t, "task", "identifier", "a", 2, 1),
			v119TestTag(t, "endtask", 2, 1), v119TestNamed(t, "task", "identifier", "b", 2, 1)}, 5, 2, nil},
		{"blank and comment", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestLine(t, " \t", "blank", 3, 2),
			v119TestNamed(t, "editor", "content", "Keep this.", 3, 2)}, 3, 2, nil},
		{"content block closing", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestOpen(t, "text", 3, 2), v119TestText(t, "Read.", "Read.", 4, 3),
			v119TestClose(t, 4, 3)}, 3, 2, nil},
		{"unparsed task does not open", []model.Line{v119TestInvalid(t, "@task a b", 2, 1)},
			2, 1, [][3]int{{4, 1, 5}}},
		{"unparsed endtask does not close", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestInvalid(t, "@endtask extra", 3, 2)}, 3, 2, nil},
		{"unparsed header does not close", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestInvalid(t, "@header", 3, 2)}, 3, 2, nil},
		{"unparsed newpage does not close", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestInvalid(t, "@newpage extra", 3, 2)}, 3, 2, nil},
		{"unparsed variants does not close", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestInvalid(t, "@variants", 3, 2)}, 3, 2, nil},
		{"escaped task does not open", []model.Line{v119TestText(t, `\@task t`, "@task t", 2, 1)},
			2, 1, [][3]int{{4, 1, 5}}},
		{"escaped endtask does not close", []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
			v119TestText(t, `\@endtask`, "@endtask", 3, 2)}, 3, 2, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{}, tc.before...)
			body = append(body, v119TestKey(t, len(body)+3, tc.parent, tc.depth)...)
			v119TestRun(t, v119TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV119KeysAndPartialParsing(t *testing.T) {
	t.Run("repeated keys", func(t *testing.T) {
		body := append(v119TestKey(t, 3, 2, 1), v119TestKey(t, 6, 2, 1)...)
		v119TestRun(t, v119TestDocument(t, body...), "current", [][3]int{{3, 1, 5}, {6, 1, 5}})
	})
	t.Run("repeated keys inside task", func(t *testing.T) {
		body := []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1)}
		body = append(body, v119TestKey(t, 4, 3, 2)...)
		body = append(body, v119TestKey(t, 7, 3, 2)...)
		v119TestRun(t, v119TestDocument(t, body...), "current", nil)
	})
	for _, active := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			t.Run("title active="+strconv.FormatBool(active)+" empty="+strconv.FormatBool(empty), func(t *testing.T) {
				var body []model.Line
				parent, depth, key := 2, 1, 3
				want := [][3]int{{3, 3, 7}}
				if active {
					body = append(body, v119TestNamed(t, "task", "identifier", "t", 2, 1))
					parent, depth, key, want = 3, 2, 4, nil
				}
				body = append(body, v119TestLine(t, "  @KeY Ответы {", "block-start", parent, depth,
					v119TestElement(t, "tag", "@KeY", "key", 3),
					v119TestElement(t, "title", "Ответы", "Ответы", 8),
					v119TestElement(t, "block-open", "{", "{", 15)))
				if !empty {
					body = append(body, v119TestText(t, "Ответ.", "Ответ.", key, depth+1))
				}
				body = append(body, v119TestClose(t, key, depth+1))
				v119TestRun(t, v119TestDocument(t, body...), "current", want)
			})
		}
	}
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"unparsed key", v119TestInvalid(t, "@key", 2, 1), nil},
		{"escaped key", v119TestText(t, `\@key`, "@key", 2, 1), nil},
		{"recognized key with bad tail", v119TestLine(t, "@KEY { extra", "invalid", 2, 1,
			v119TestElement(t, "tag", "@KEY", "key", 1), v119TestUnparsed(t, "{ extra", 6)), [][3]int{{3, 1, 5}}},
		{"longer unknown name", v119TestInvalid(t, "@keyExtra {", 2, 1), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v119TestRun(t, v119TestDocument(t, tc.line), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		name, raw, tag, tail string
		start                int
		open                 bool
	}{
		{"task", "@TASK a b", "task", "a b", 7, true},
		{"endtask", "@ENDTASK extra", "endtask", "extra", 10, false},
		{"header", "@HEADER\tNext", "header", "\tNext", 8, false},
		{"newpage", "@NEWPAGE extra", "newpage", "extra", 10, false},
		{"variants", "@VARIANTS extra", "variants", "extra", 11, false},
	} {
		t.Run("recognized erroneous "+tc.name, func(t *testing.T) {
			before := []model.Line{v119TestNamed(t, "task", "identifier", "previous", 2, 1)}
			if tc.open {
				before = []model.Line{v119TestNamed(t, "header", "title", "Answers", 2, 1)}
			}
			before = append(before, v119TestLine(t, tc.raw, "invalid", 2, 1,
				v119TestElement(t, "tag", "@"+strings.ToUpper(tc.tag), tc.tag, 1),
				v119TestUnparsed(t, tc.tail, tc.start)))
			want := [][3]int{{5, 1, 5}}
			parent, depth := 2, 1
			if tc.open {
				want, parent, depth = nil, 4, 2
			}
			before = append(before, v119TestKey(t, 5, parent, depth)...)
			v119TestRun(t, v119TestDocument(t, before...), "current", want)
		})
	}
}

func TestV119Branches(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"separate branches and outside", []model.Line{
			v119TestOpen(t, "variants", 2, 1), v119TestNamed(t, "variant", "name", "A", 3, 2),
			v119TestNamed(t, "task", "identifier", "a", 4, 3),
			v119TestOpen(t, "key", 5, 4), v119TestText(t, "Answer.", "Answer.", 6, 5), v119TestClose(t, 6, 5),
			v119TestNamed(t, "variant", "name", "B", 3, 2),
			v119TestOpen(t, "key", 9, 3), v119TestText(t, "Answer.", "Answer.", 10, 4), v119TestClose(t, 10, 4),
			v119TestClose(t, 3, 2),
			v119TestOpen(t, "key", 2, 1), v119TestText(t, "Answer.", "Answer.", 14, 2), v119TestClose(t, 14, 2),
		}, [][3]int{{10, 1, 5}, {14, 1, 5}}},
		{"external task not inherited or restored", []model.Line{
			v119TestNamed(t, "task", "identifier", "outer", 2, 1), v119TestOpen(t, "variants", 2, 1),
			v119TestNamed(t, "variant", "name", "A", 4, 2),
			v119TestOpen(t, "key", 5, 3), v119TestText(t, "Answer.", "Answer.", 6, 4), v119TestClose(t, 6, 4),
			v119TestNamed(t, "variant", "name", "B", 4, 2), v119TestText(t, "Read.", "Read.", 9, 3),
			v119TestClose(t, 4, 2),
			v119TestOpen(t, "key", 2, 1), v119TestText(t, "Answer.", "Answer.", 12, 2), v119TestClose(t, 12, 2),
		}, [][3]int{{6, 1, 5}, {12, 1, 5}}},
		{"each branch has task", []model.Line{
			v119TestOpen(t, "variants", 2, 1), v119TestNamed(t, "variant", "name", "A", 3, 2),
			v119TestNamed(t, "task", "identifier", "a", 4, 3),
			v119TestOpen(t, "key", 5, 4), v119TestText(t, "Answer.", "Answer.", 6, 5), v119TestClose(t, 6, 5),
			v119TestNamed(t, "variant", "name", "B", 3, 2), v119TestNamed(t, "task", "identifier", "b", 9, 3),
			v119TestNamed(t, "step", "title", "Read", 10, 4),
			v119TestOpen(t, "key", 11, 5), v119TestText(t, "Answer.", "Answer.", 12, 6), v119TestClose(t, 12, 6),
			v119TestClose(t, 3, 2),
		}, nil},
		{"last branch task ends at closing", []model.Line{
			v119TestOpen(t, "variants", 2, 1), v119TestNamed(t, "variant", "name", "A", 3, 2),
			v119TestText(t, "Read.", "Read.", 4, 3), v119TestNamed(t, "variant", "name", "B", 3, 2),
			v119TestNamed(t, "task", "identifier", "b", 6, 3), v119TestText(t, "Write.", "Write.", 7, 4),
			v119TestClose(t, 3, 2),
			v119TestOpen(t, "key", 2, 1), v119TestText(t, "Answer.", "Answer.", 10, 2), v119TestClose(t, 10, 2),
		}, [][3]int{{10, 1, 5}}},
		{"before first branch excluded", []model.Line{
			v119TestOpen(t, "variants", 2, 1), v119TestNamed(t, "task", "identifier", "wrong", 3, 2),
			v119TestOpen(t, "key", 4, 3), v119TestText(t, "Answer.", "Answer.", 5, 4), v119TestClose(t, 5, 4),
			v119TestNamed(t, "variant", "name", "A", 3, 2),
			v119TestOpen(t, "key", 8, 3), v119TestText(t, "Answer.", "Answer.", 9, 4), v119TestClose(t, 9, 4),
			v119TestNamed(t, "variant", "name", "B", 3, 2), v119TestText(t, "Read.", "Read.", 12, 3),
			v119TestClose(t, 3, 2),
		}, [][3]int{{9, 1, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, shared := range []bool{false, true} {
				t.Run("shared parent="+strconv.FormatBool(shared), func(t *testing.T) {
					result := v119TestDocument(t, tc.body...)
					if shared {
						v119TestSharedParents(t, result)
					}
					v119TestRun(t, result, "current", tc.want)
				})
			}
		})
	}
	for _, name := range []string{"endtask", "header", "newpage"} {
		t.Run("reset in branch "+name, func(t *testing.T) {
			event := v119TestTag(t, name, 4, 3)
			if name == "header" {
				event = v119TestNamed(t, name, "title", "Answers", 4, 3)
			}
			result := v119TestDocument(t, v119TestOpen(t, "variants", 2, 1),
				v119TestNamed(t, "variant", "name", "A", 3, 2),
				v119TestNamed(t, "task", "identifier", "a", 4, 3), event,
				v119TestOpen(t, "key", 4, 3), v119TestText(t, "Answer.", "Answer.", 7, 4), v119TestClose(t, 7, 4),
				v119TestNamed(t, "variant", "name", "B", 3, 2), v119TestText(t, "Read.", "Read.", 10, 3),
				v119TestClose(t, 3, 2))
			if name == "newpage" {
				// DSL запрещает newpage в ветви, но V119 явно сохраняет событие.
				v119TestParserError(t, result, 6, -1, "newpage запрещён в ветви.")
			}
			v119TestRun(t, result, "current", [][3]int{{7, 1, 5}})
		})
	}
	for _, recognized := range []bool{false, true} {
		t.Run("erroneous variant recognized="+strconv.FormatBool(recognized), func(t *testing.T) {
			event := v119TestInvalid(t, "@VARIANT", 3, 2) // Обязательное NAME отсутствует.
			parent, depth := 5, 4
			want := [][3]int(nil)
			if recognized {
				event = v119TestLine(t, "@VARIANT", "tag", 3, 2,
					v119TestElement(t, "tag", "@VARIANT", "variant", 1))
				parent, depth, want = 6, 3, [][3]int{{7, 1, 5}}
			}
			result := v119TestDocument(t, v119TestOpen(t, "variants", 2, 1),
				v119TestNamed(t, "variant", "name", "A", 3, 2), v119TestNamed(t, "task", "identifier", "a", 4, 3),
				event, v119TestOpen(t, "key", parent, depth),
				v119TestText(t, "Answer.", "Answer.", 7, depth+1), v119TestClose(t, 7, depth+1),
				v119TestNamed(t, "variant", "name", "B", 3, 2), v119TestText(t, "Read.", "Read.", 10, 3),
				v119TestClose(t, 3, 2))
			if recognized {
				v119TestParserError(t, result, 6, -1, "У variant отсутствует NAME.")
			}
			v119TestRun(t, result, "current", want)
		})
	}
}

func TestV119ExcludedContainers(t *testing.T) {
	for _, container := range []string{"editor", "multifill", "text", "note", "key"} {
		for _, event := range []string{"endtask", "header", "newpage", "task", "variants", "variant"} {
			t.Run(container+" ignores "+event, func(t *testing.T) {
				line := v119TestTag(t, event, 4, 3)
				switch event {
				case "header":
					line = v119TestNamed(t, event, "title", "Next", 4, 3)
				case "task":
					line = v119TestNamed(t, event, "identifier", "hidden", 4, 3)
				case "variant":
					line = v119TestNamed(t, event, "name", "B", 4, 3)
				}
				if container == "editor" || container == "multifill" {
					line = v119TestText(t, line.Raw, line.Raw, 4, 3)
				}
				body := []model.Line{v119TestNamed(t, "task", "identifier", "t", 2, 1),
					v119TestOpen(t, container, 3, 2), line, v119TestClose(t, 4, 3)}
				body = append(body, v119TestKey(t, 7, 3, 2)...)
				v119TestRun(t, v119TestDocument(t, body...), "current", nil)
			})
		}
		t.Run(container+" task cannot open outside", func(t *testing.T) {
			line := v119TestNamed(t, "task", "identifier", "hidden", 3, 2)
			if container == "editor" || container == "multifill" {
				line = v119TestText(t, line.Raw, line.Raw, 3, 2)
			}
			body := []model.Line{v119TestOpen(t, container, 2, 1), line, v119TestClose(t, 3, 2)}
			body = append(body, v119TestKey(t, 6, 2, 1)...)
			want := [][3]int{{6, 1, 5}}
			if container == "key" {
				want = [][3]int{{3, 1, 5}, {6, 1, 5}}
			}
			v119TestRun(t, v119TestDocument(t, body...), "current", want)
		})
	}
	for _, container := range []string{"text", "note", "table"} {
		t.Run("nested key in "+container, func(t *testing.T) {
			body := []model.Line{v119TestOpen(t, container, 2, 1)}
			body = append(body, v119TestKey(t, 4, 3, 2)...)
			body = append(body, v119TestClose(t, 3, 2))
			v119TestRun(t, v119TestDocument(t, body...), "current", nil)
		})
	}
	for _, container := range []string{"editor", "multifill"} {
		t.Run("literal key in "+container, func(t *testing.T) {
			v119TestRun(t, v119TestDocument(t, v119TestOpen(t, container, 2, 1),
				v119TestText(t, "@key {", "@key {", 3, 2), v119TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("events deeper in forbidden block", func(t *testing.T) {
		body := []model.Line{v119TestOpen(t, "note", 2, 1), v119TestNamed(t, "task", "identifier", "hidden", 3, 2)}
		body = append(body, v119TestKey(t, 5, 4, 3)...)
		body = append(body, v119TestClose(t, 3, 2))
		body = append(body, v119TestKey(t, 9, 2, 1)...)
		v119TestRun(t, v119TestDocument(t, body...), "current", [][3]int{{9, 1, 5}})
	})
	for _, active := range []bool{false, true} {
		t.Run("include does not change task active="+strconv.FormatBool(active), func(t *testing.T) {
			body := []model.Line{v119TestLine(t, "@fragment part {", "block-start", 2, 1,
				v119TestElement(t, "tag", "@fragment", "fragment", 1),
				v119TestElement(t, "identifier", "part", "part", 11),
				v119TestElement(t, "block-open", "{", "{", 16)),
				v119TestText(t, "Read.", "Read.", 3, 2), v119TestClose(t, 3, 2)}
			parent, depth, key := 2, 1, 7
			want := [][3]int{{7, 1, 5}}
			if active {
				body = append(body, v119TestNamed(t, "task", "identifier", "t", 2, 1))
				parent, depth, key, want = 6, 2, 8, nil
			}
			body = append(body, v119TestNamed(t, "include", "identifier", "part", parent, depth))
			body = append(body, v119TestKey(t, key, parent, depth)...)
			v119TestRun(t, v119TestDocument(t, body...), "current", want)
		})
	}
}

func TestV119EndingsVersionsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"}, {"no final EOL", "\n", ""},
		{"CRLF no final EOL", "\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v119TestDocument(t, v119TestKey(t, 3, 2, 1)...)
			for index := range result.Lines {
				result.Lines[index].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v119TestRun(t, result, "first", [][3]int{{3, 1, 5}})
			v119TestRun(t, result, "second", [][3]int{{3, 1, 5}})
		})
	}
	t.Run("unclosed key at EOF", func(t *testing.T) {
		v119TestRun(t, v119TestDocument(t, v119TestOpen(t, "key", 2, 1)), "current", [][3]int{{3, 1, 5}})
	})
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v119TestModel(t)
			result.Document.Metadata.Title = nil
			if !known {
				result.Document.LineCount = nil
			}
			v119TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v119TestDocument(t, v119TestKey(t, 3, 2, 1)...)
			result.Document.DSLVersion = &version
			before := v119TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v119TestRegistry(t))
			if err == nil || checks != nil || v119TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v119TestSharedParents задаёт допустимую модель с общим родителем variants
// у содержимого ветвей. Изменяется только вход, а не ожидаемые диапазоны.
func v119TestSharedParents(t *testing.T, result *model.Result) {
	t.Helper()
	for index := range result.Lines {
		line := &result.Lines[index]
		if line.ParentLine == nil {
			continue
		}
		parent := result.Lines[*line.ParentLine-1]
		for _, element := range parent.Elements {
			if element.ElementType == "tag" && element.Value != nil && *element.Value == "variant" {
				line.ParentLine = parent.ParentLine
				break
			}
		}
		line.NestingLevel = result.Lines[*line.ParentLine-1].NestingLevel + 1
	}
}

// v119TestKey создаёт полный входной блок ключа с указанными родителями.
// number задаётся для ссылок содержимого и не используется в ожиданиях.
func v119TestKey(t *testing.T, number, parent, depth int) []model.Line {
	t.Helper()
	return []model.Line{v119TestOpen(t, "key", parent, depth),
		v119TestText(t, "Answer.", "Answer.", number, depth+1), v119TestClose(t, number, depth+1)}
}

func v119TestTag(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v119TestLine(t, "@"+name, "tag", parent, depth, v119TestElement(t, "tag", "@"+name, name, 1))
}

// v119TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v119TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{
		v119TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v119TestHeading(t, "#", "Lesson"),
	}, body...)
	return v119TestModel(t, lines...)
}

// v119TestModel принимает готовые строки без разбора исходного DSL.
func v119TestModel(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines = append([]model.Line{}, lines...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if count != 0 && lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v119TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v119TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v119TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		element := &line.Elements[index]
		scope, start, end = diagnostics.ScopeElement, element.Start, element.End
		element.ErrorIDs = []string{id}
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
}

func v119TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v119TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v119TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v119TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v119TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v119TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v119TestLine(t, raw, "invalid", parent, depth, v119TestUnparsed(t, raw, 1))
}

func v119TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v119TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v119TestElement(t, "tag", "@"+tag, tag, 1), v119TestElement(t, kind, value, value, len(tag)+3))
}

func v119TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v119TestLine(t, marker+" "+title, "heading", 0, 0,
		v119TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v119TestElement(t, "title", title, title, len(marker)+2))
}

func v119TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v119TestLine(t, raw, "content", parent, depth, v119TestElement(t, "content", raw, value, 1))
}

func v119TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v119TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v119TestElement(t, "tag", "@"+tag, tag, 1), v119TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v119TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v119TestLine(t, "}", "block-end", parent, depth, v119TestElement(t, "block-close", "}", "}", 1))
}

func v119TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v119TestRun выполняет полный каталог и сравнивает только новые V119.
// Ожидания задают основной диапазон с исключённым концом; связанных нет.
func v119TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v119TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v119TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v119TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V119" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V119 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want location %+v and no related locations", i, entry, location)
		}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "@key находится вне активного задания." {
			t.Fatalf("unexpected V119 metadata: %+v", entry)
		}
	}
	// CheckModel проверяет уникальность ID, ссылки и флаги всех диагностик.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v119TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v119TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v119TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v119TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var source strings.Builder
	for _, line := range result.Lines {
		source.WriteString(line.Raw)
		source.WriteString(string(line.LineEnding))
	}
	if source.String() != want {
		t.Fatalf("source = %q, want %q", source.String(), want)
	}
}

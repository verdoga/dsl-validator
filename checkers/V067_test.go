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

func TestV067Registration(t *testing.T) {
	definitions, err := v067Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V067" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "@step находится вне активного задания." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V067 definitions = %d, want 1", count)
	}
}

func TestV067ReferenceAndExample(t *testing.T) {
	// DSL 1.30: перед дословными фрагментами добавлены только версия и # Lesson.
	t.Run("reference", func(t *testing.T) {
		result := v067Lesson(t, v067TagLine(t, "task", "practice", "identifier", 2, 1),
			v067TagLine(t, "step", "Read", "title", 3, 2), v067Text(t, "Read.", "Read.", 4, 3),
			v067TagLine(t, "step", "Write", "title", 3, 2), v067Text(t, "Write.", "Write.", 6, 3))
		v067Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v067Lesson(t, v067TagLine(t, "header", "Theory", "title", 2, 1),
			v067TagLine(t, "step", "Read", "title", 2, 1), v067Text(t, "Read the text.", "Read the text.", 4, 2))
		v067Run(t, result, "current", [][3]int{{4, 1, 6}})
	})
}

func TestV067TaskFlow(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"first and last steps", []model.Line{
			v067TagLine(t, "step", "Read", "title", 2, 1), v067Text(t, "Read.", "Read.", 3, 2),
			v067TagLine(t, "step", "Write", "title", 2, 1),
		}, [][3]int{{3, 1, 6}, {5, 1, 6}}},
		{"step does not open a task", []model.Line{
			v067TagLine(t, "step", "Read", "title", 2, 1), v067TagLine(t, "step", "Write", "title", 2, 1),
		}, [][3]int{{3, 1, 6}, {4, 1, 6}}},
		{"step does not close a task", []model.Line{
			v067TagLine(t, "task", "a", "identifier", 2, 1),
			v067TagLine(t, "step", "Read", "title", 3, 2), v067TagLine(t, "step", "Write", "title", 3, 2),
		}, nil},
		{"next task opens immediately", []model.Line{
			v067TagLine(t, "task", "a", "identifier", 2, 1), v067TagLine(t, "step", "Read", "title", 3, 2),
			v067TagLine(t, "task", "b", "identifier", 2, 1), v067TagLine(t, "step", "Write", "title", 5, 2),
		}, nil},
		{"opening after reported step", []model.Line{
			v067TagLine(t, "step", "Read", "title", 2, 1), v067TagLine(t, "task", "a", "identifier", 2, 1),
			v067TagLine(t, "step", "Write", "title", 4, 2),
		}, [][3]int{{3, 1, 6}}},
		{"empty step outside task", []model.Line{v067Attempt(t, "@step", "step", 1, 2, 1)}, [][3]int{{3, 1, 6}}},
		{"empty step inside task", []model.Line{
			v067TagLine(t, "task", "a", "identifier", 2, 1), v067Attempt(t, "@step", "step", 1, 3, 2),
		}, nil},
		{"empty task still opens", []model.Line{
			v067Attempt(t, "@task", "task", 1, 2, 1), v067TagLine(t, "step", "Read", "title", 3, 2),
		}, nil},
		{"misplaced variant does not end root task", []model.Line{
			v067TagLine(t, "task", "a", "identifier", 2, 1),
			// DSL 4.9.20: variant вне variants ошибочен. Граница variant
			// завершает задание только в ветви, как задано шагом 2 V067.
			v067Attempt(t, "@variant A", "variant", 1, 3, 2),
			v067TagLine(t, "step", "Read", "title", 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v067Run(t, v067Lesson(t, tc.body...), "current", tc.want) })
	}
}

func TestV067TaskBoundaries(t *testing.T) {
	t.Run("empty third level heading", func(t *testing.T) {
		heading := v067Line(t, "###", "invalid", 0, 0,
			v067Element(t, "heading-level", "###", "3", 1, 4),
			v067Element(t, "unparsed", "", "", 4, 4))
		v067Run(t, v067Lesson(t, v067TagLine(t, "task", "a", "identifier", 2, 1),
			v067Text(t, "Read.", "Read.", 3, 2), heading, v067TagLine(t, "step", "Read", "title", 2, 1)),
			"current", [][3]int{{6, 1, 6}})
	})
	for _, name := range []string{"endtask", "header", "newpage"} {
		for _, recovered := range []bool{false, true} {
			t.Run(name+"/raw="+strconv.FormatBool(recovered), func(t *testing.T) {
				boundary := v067TagLine(t, name, "", "", 2, 1)
				if name == "header" {
					boundary = v067TagLine(t, name, "Theory", "title", 2, 1)
				}
				if recovered {
					// Ошибочный разделитель/аргумент остаётся в unparsed;
					// принадлежность этапа не зависит от исправности границы.
					boundary = v067Invalid(t, "@"+strings.ToUpper(name)+"\t?", 2, 1)
				}
				result := v067Lesson(t, v067TagLine(t, "task", "a", "identifier", 2, 1),
					v067Text(t, "Read.", "Read.", 3, 2), boundary, v067TagLine(t, "step", "Read", "title", 2, 1))
				v067Run(t, result, "current", [][3]int{{6, 1, 6}})
			})
		}
	}
	for _, tc := range []struct {
		level string
		want  [][3]int
	}{
		{"1", nil}, {"2", nil}, {"3", [][3]int{{6, 1, 6}}},
	} {
		t.Run("heading "+tc.level, func(t *testing.T) {
			marker := strings.Repeat("#", int(tc.level[0]-'0'))
			heading := v067Line(t, marker+" Section", "heading", 0, 0,
				v067Element(t, "heading-level", marker, tc.level, 1, len(marker)+1),
				v067Element(t, "title", "Section", "Section", len(marker)+2, len(marker)+9))
			// Повторный уровень 1 — ошибка V054, однако V067 завершает
			// задание лишь на уровне 3, как прямо задано её алгоритмом.
			result := v067Lesson(t, v067TagLine(t, "task", "a", "identifier", 2, 1),
				v067Text(t, "Read.", "Read.", 3, 2), heading, v067TagLine(t, "step", "Read", "title", 5, 1))
			v067Run(t, result, "current", tc.want)
		})
	}
}

func TestV067CommentsAndPrefixes(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run("comments/active="+strconv.FormatBool(active), func(t *testing.T) {
			first := v067TagLine(t, "header", "Theory", "title", 2, 1)
			parent, depth := 2, 1
			want := [][3]int{{6, 1, 6}}
			if active {
				first = v067TagLine(t, "task", "a", "identifier", 2, 1)
				parent, depth, want = 3, 2, nil
			}
			result := v067Lesson(t, first, v067TagLine(t, "editor", "@task fake", "content", parent, depth),
				v067Line(t, " \t", "blank", parent, depth), v067TagLine(t, "step", "Read", "title", parent, depth))
			v067Run(t, result, "current", want)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@step Read`, "@step Read"}, {`\\@step Read`, `\@step Read`},
		{" \t\\@step Read", "@step Read"}, {"Text @step", "Text @step"},
		{"\u00a0@step Read", "\u00a0@step Read"}, {" \t\u2003@step", "\u2003@step"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v067Run(t, v067Lesson(t, v067Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("escaped task does not open", func(t *testing.T) {
		v067Run(t, v067Lesson(t, v067Text(t, `\@task a`, "@task a", 2, 1),
			v067TagLine(t, "step", "Read", "title", 2, 1)), "current", [][3]int{{4, 1, 6}})
	})
	t.Run("escaped boundary does not close", func(t *testing.T) {
		v067Run(t, v067Lesson(t, v067TagLine(t, "task", "a", "identifier", 2, 1),
			v067Text(t, `\@endtask`, "@endtask", 3, 2),
			v067TagLine(t, "step", "Read", "title", 3, 2)), "current", nil)
	})
	t.Run("canonical tag and source positions", func(t *testing.T) {
		step := v067Line(t, " \t@StEp Чтение 😀", "tag", 2, 1,
			v067Element(t, "tag", "@StEp", "step", 3, 8),
			v067Element(t, "title", "Чтение 😀", "Чтение 😀", 9, 17))
		v067Run(t, v067Lesson(t, step), "current", [][3]int{{3, 3, 8}})
		step.ParentLine, step.NestingLevel = new(int), 2
		*step.ParentLine = 3
		v067Run(t, v067Lesson(t, v067TagLine(t, "TaSk", "a", "identifier", 2, 1), step), "current", nil)
	})
}

func TestV067Recovery(t *testing.T) {
	// Перечень White_Space независим от реализации. CR/LF проверяются как EOL.
	tails := []string{"", " ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000", "{", "}"}
	for _, tail := range tails {
		t.Run(strconv.Quote(tail), func(t *testing.T) {
			body := []model.Line{v067Invalid(t, " \t@StEp"+tail, 2, 1)}
			if tail == "{" {
				// Блочная форма step запрещена: обе скобки сохранены
				// как ошибки восстановления, без выдуманного блока.
				body = append(body, v067Invalid(t, "}", 2, 1))
			}
			v067Run(t, v067Lesson(t, body...), "current", [][3]int{{3, 3, 8}})
		})
	}
	for _, raw := range []string{"@stepExtra", "@step1", "@step-name", "@step.More", "@step\u200b", "@step\ufeff"} {
		t.Run(raw, func(t *testing.T) {
			v067Run(t, v067Lesson(t, v067Invalid(t, raw, 2, 1)), "current", nil)
		})
	}
	t.Run("longer task name", func(t *testing.T) {
		v067Run(t, v067Lesson(t, v067Invalid(t, "@taskExtra a", 2, 1),
			v067TagLine(t, "step", "Read", "title", 2, 1)), "current", [][3]int{{4, 1, 6}})
	})
	for _, raw := range []string{"@endtaskExtra", "@headerExtra", "@newpageExtra", "@variantsExtra", "@variantExtra"} {
		t.Run(raw+" does not close", func(t *testing.T) {
			v067Run(t, v067Lesson(t, v067TagLine(t, "task", "a", "identifier", 2, 1),
				v067Invalid(t, raw, 3, 2), v067TagLine(t, "step", "Read", "title", 3, 2)), "current", nil)
		})
	}
	for _, raw := range []string{"@task", "@TASK\tbad!", "@TaSk\u00a0bad!", "@task{", "@task}"} {
		t.Run(raw+" opens", func(t *testing.T) {
			body := []model.Line{v067Invalid(t, raw, 2, 1),
				v067TagLine(t, "step", "Read", "title", 2, 1)}
			if raw == "@task{" {
				body = append(body, v067Invalid(t, "}", 2, 1))
			}
			v067Run(t, v067Lesson(t, body...), "current", nil)
		})
	}
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run("tag plus unparsed/"+string(kind), func(t *testing.T) {
			line := v067Attempt(t, "@step\tRead", "step", 1, 2, 1)
			line.LineType = kind
			v067Run(t, v067Lesson(t, line), "current", [][3]int{{3, 1, 6}})
		})
	}
}

func TestV067Branches(t *testing.T) {
	for _, sharedParent := range []bool{false, true} {
		for _, recovered := range []bool{false, true} {
			t.Run("shared="+strconv.FormatBool(sharedParent)+"/raw="+strconv.FormatBool(recovered), func(t *testing.T) {
				// Две используемые в репозитории формы модели: дети variant
				// или дети общего variants. Ветви определяются порядком строк.
				firstParent, secondParent, depth := 6, 10, 3
				if sharedParent {
					firstParent, secondParent, depth = 5, 5, 2
				}
				open := v067Open(t, "variants", 2, 1)
				first := v067TagLine(t, "variant", "A", "name", 5, 2)
				second := v067TagLine(t, "variant", "B", "name", 5, 2)
				if recovered {
					// Ошибочная табуляция-разделитель: блок и его скобки
					// сохранены, но имена остались в unparsed.
					open = v067Line(t, "@VaRiAnTs\t{", "block-start", 2, 1,
						v067Element(t, "unparsed", "@VaRiAnTs\t", "", 1, 11),
						v067Element(t, "block-open", "{", "{", 11, 12))
					first = v067Invalid(t, "@VaRiAnT\tA", 5, 2)
					second = v067Invalid(t, "@VARIANT\tB", 5, 2)
				}
				result := v067Lesson(t,
					v067TagLine(t, "task", "outer", "identifier", 2, 1),
					v067Text(t, "Read.", "Read.", 3, 2), open, first,
					v067TagLine(t, "step", "Before", "title", firstParent, depth),
					v067TagLine(t, "task", "a", "identifier", firstParent, depth),
					v067TagLine(t, "step", "Inside", "title", 8, depth+1), second,
					v067TagLine(t, "step", "Before", "title", secondParent, depth),
					v067TagLine(t, "task", "b", "identifier", secondParent, depth),
					v067TagLine(t, "step", "Inside", "title", 12, depth+1),
					v067Close(t, 5, 2), v067TagLine(t, "step", "After", "title", 2, 1))
				v067Run(t, result, "current", [][3]int{{7, 1, 6}, {11, 1, 6}, {15, 1, 6}})
			})
		}
	}
	for _, boundary := range []string{"header", "endtask", "newpage"} {
		t.Run("boundary inside branch/"+boundary, func(t *testing.T) {
			line := v067TagLine(t, boundary, "", "", 4, 3)
			if boundary == "header" {
				line = v067TagLine(t, boundary, "Theory", "title", 4, 3)
			}
			if boundary == "newpage" {
				// DSL 4.9.27 запрещает newpage в ветви. V067 всё равно
				// завершает задание; ошибка положения не отменяет границу.
				line = v067Attempt(t, "@newpage", "newpage", 1, 4, 3)
			}
			result := v067Lesson(t, v067Open(t, "variants", 2, 1),
				v067TagLine(t, "variant", "A", "name", 3, 2),
				v067TagLine(t, "task", "a", "identifier", 4, 3), v067Text(t, "Read.", "Read.", 5, 4),
				line, v067TagLine(t, "step", "Read", "title", 4, 3),
				v067TagLine(t, "variant", "B", "name", 3, 2), v067Text(t, "Write.", "Write.", 9, 3),
				v067Close(t, 3, 2))
			v067Run(t, result, "current", [][3]int{{8, 1, 6}})
		})
	}
	t.Run("new task after variants", func(t *testing.T) {
		result := v067Lesson(t, v067Open(t, "variants", 2, 1),
			v067TagLine(t, "variant", "A", "name", 3, 2), v067Text(t, "Read.", "Read.", 4, 3),
			v067TagLine(t, "variant", "B", "name", 3, 2), v067Text(t, "Write.", "Write.", 6, 3),
			v067Close(t, 3, 2), v067TagLine(t, "task", "a", "identifier", 2, 1),
			v067TagLine(t, "step", "Read", "title", 9, 2))
		v067Run(t, result, "current", nil)
	})
}

func TestV067ExcludedBlocks(t *testing.T) {
	for _, block := range []string{"editor", "multifill", "text"} {
		for _, active := range []bool{false, true} {
			for _, statement := range []struct{ name, raw string }{
				{"task", "@task a"}, {"step", "@step Read"}, {"endtask", "@endtask"}, {"header", "@header Theory"},
				{"newpage", "@newpage"}, {"variants", "@variants"}, {"variant", "@variant B"}, {"heading", "###"},
			} {
				t.Run(block+"/"+statement.name+"/active="+strconv.FormatBool(active), func(t *testing.T) {
					first := v067TagLine(t, "header", "Theory", "title", 2, 1)
					parent, depth := 2, 1
					want := [][3]int{{7, 1, 6}}
					if active {
						first = v067TagLine(t, "task", "a", "identifier", 2, 1)
						parent, depth, want = 3, 2, nil
					}
					content := v067Text(t, statement.raw, statement.raw, 4, depth+1)
					if block == "text" {
						// Структурное объявление запрещено внутри text:
						// готовый tag сохраняется с ошибкой, не как обычный текст.
						if statement.name == "heading" {
							content = v067Line(t, "###", "invalid", 4, depth+1,
								v067Element(t, "heading-level", "###", "3", 1, 4),
								v067Element(t, "unparsed", "", "", 4, 4))
						} else {
							content = v067Attempt(t, statement.raw, statement.name, 1, 4, depth+1)
						}
					}
					result := v067Lesson(t, first, v067Open(t, block, parent, depth),
						content, v067Close(t, 4, depth+1), v067TagLine(t, "step", "Read", "title", parent, depth))
					v067Run(t, result, "current", want)
				})
			}
		}
	}
	t.Run("variants nested in another block are excluded", func(t *testing.T) {
		open := v067Open(t, "variants", 3, 2)
		// Запрещённая вложенность отмечена исходной ошибкой на объявлении;
		// остальные связи и обе ветви настоящего блока сохраняются.
		open.Elements = append(open.Elements, v067Element(t, "unparsed", "", "", 12, 12))
		result := v067Lesson(t, v067Open(t, "text", 2, 1), open,
			v067TagLine(t, "variant", "A", "name", 4, 3),
			v067TagLine(t, "step", "Read", "title", 5, 4), v067Text(t, "Read.", "Read.", 6, 5),
			v067TagLine(t, "variant", "B", "name", 4, 3),
			v067TagLine(t, "task", "a", "identifier", 8, 4), v067TagLine(t, "step", "Write", "title", 9, 5),
			v067Close(t, 4, 3), v067Close(t, 3, 2), v067TagLine(t, "step", "After", "title", 2, 1))
		v067Run(t, result, "current", [][3]int{{13, 1, 6}})
	})
	t.Run("literal variant does not switch branch", func(t *testing.T) {
		result := v067Lesson(t, v067Open(t, "variants", 2, 1),
			v067TagLine(t, "variant", "A", "name", 3, 2), v067TagLine(t, "task", "a", "identifier", 4, 3),
			v067Open(t, "editor", 5, 4), v067Text(t, "@variant B", "@variant B", 6, 5),
			v067Text(t, "@endtask", "@endtask", 6, 5), v067Close(t, 6, 5),
			v067TagLine(t, "step", "Read", "title", 5, 4),
			v067TagLine(t, "variant", "B", "name", 3, 2), v067Text(t, "Write.", "Write.", 11, 3),
			v067Close(t, 3, 2))
		v067Run(t, result, "current", nil)
	})
}

func TestV067EndingsAndIndependentRuns(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v067Lesson(t, v067TagLine(t, "header", "Теория", "title", 2, 1),
					v067TagLine(t, "step", "Чтение 😀", "title", 2, 1), v067Text(t, "Read.", "Read.", 4, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !finalEOL {
					result.Lines[4].LineEnding = ""
				}
				v067Run(t, result, "current", [][3]int{{4, 1, 6}})
			})
		}
	}
	// Активное задание в конце одного документа не переносится в другой.
	active := v067Lesson(t, v067TagLine(t, "task", "a", "identifier", 2, 1),
		v067TagLine(t, "step", "Read", "title", 3, 2))
	inactive := v067Lesson(t, v067TagLine(t, "step", "Read", "title", 2, 1))
	v067Run(t, active, "first", nil)
	v067Run(t, inactive, "first", [][3]int{{3, 1, 6}})
	v067Run(t, active, "second", nil)
	v067Run(t, inactive, "second", [][3]int{{3, 1, 6}})
}

func TestV067EmptyAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v067Lesson(t)
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			result.Document.Metadata.Title = nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v067Run(t, result, "current", nil)
				return
			}
			before := v067Snapshot(t, result)
			checks, err := validator.Prepare(result, v067Registry(t))
			if err == nil || checks != nil || v067Snapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v067Attempt задаёт готовый tag и неразобранный остаток ошибочной строки.
func v067Attempt(t *testing.T, raw, name string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + len(name) + 1
	return v067Line(t, raw, "invalid", parent, depth,
		v067Element(t, "tag", string(runes[start-1:end-1]), name, start, end),
		v067Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v067Lesson добавляет обязательный заголовок; body начинается со строки 3.
func v067Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v067Document(t, append([]model.Line{v067Heading(t)}, body...)...)
}

// v067Document добавляет версию перед готовыми строками, содержащими # Lesson.
// Для явно заданных unparsed создаёт согласованные исходные ошибки и ссылки.
func v067Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v067TagLine(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
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

// v067Heading задаёт обязательный заголовок первого уровня.
func v067Heading(t *testing.T) model.Line {
	t.Helper()
	return v067Line(t, "# Lesson", "heading", 0, 0,
		v067Element(t, "heading-level", "#", "1", 1, 2),
		v067Element(t, "title", "Lesson", "Lesson", 3, 9))
}

// v067Invalid задаёт целиком неразобранную строку без элемента tag.
func v067Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v067Line(t, raw, "invalid", parent, depth, v067Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v067Text задаёт Raw и семантическое значение независимо.
func v067Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v067Line(t, raw, "content", parent, depth, v067Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v067TagLine задаёт известный однострочный тег с необязательным аргументом.
func v067TagLine(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v067Element(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v067Element(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v067Line(t, raw, "tag", parent, depth, elements...)
}

// v067Open задаёт блок с каноническим семантическим именем тега.
func v067Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v067Line(t, "@"+tag+" {", "block-start", parent, depth,
		v067Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v067Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v067Close задаёт закрытие блока с явным родителем.
func v067Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v067Line(t, "}", "block-end", parent, depth, v067Element(t, "block-close", "}", "}", 1, 2))
}

// v067Line сохраняет явно заданную структуру без классификации Raw.
func v067Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v067Element задаёт исходные Unicode-колонки с исключающим концом.
func v067Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v067Registry создаёт реестр из полного штатного каталога.
func v067Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v067Run проверяет новые V067 по явным тройкам строка/начало/исключающий конец.
// Конвейер выполняет полный каталог; результаты других диагностик не отбираются.
func v067Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v067Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v067Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v067Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V067" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V067 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "@step находится вне активного задания." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V067 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v067Snapshot(t, result)), &restored); err != nil {
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
	if v067Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v067Snapshot сохраняет независимое представление модели для сравнения.
func v067Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

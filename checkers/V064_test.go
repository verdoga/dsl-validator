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

func TestV064Registration(t *testing.T) {
	definitions, err := v064Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V064" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У @task отсутствует ID." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V064 definitions = %d, want 1", count)
	}
}

func TestV064ReferenceAndExample(t *testing.T) {
	// DSL 1.30: добавлены только @dsl-version 1.2 и # Lesson.
	// Все пять строк каждого фрагмента сохранены; задание занимает строки 3–7.
	for _, tc := range []struct {
		name string
		task model.Line
		want [][3]int
	}{
		{"reference", v064Tag(t, "task", "practice", "identifier", 2, 1), nil},
		{"example", v064Attempt(t, "@task", 1, 2, 1), [][3]int{{3, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v064Lesson(t, tc.task,
				v064Tag(t, "step", "Read", "title", 3, 2), v064Text(t, "Read.", "Read.", 4, 3),
				v064Tag(t, "step", "Write", "title", 3, 2), v064Text(t, "Write.", "Write.", 6, 3))
			v064Run(t, result, "current", tc.want)
		})
	}
}

func TestV064RecognitionAndPositions(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		tagStart  int
		want      [][3]int
	}{
		{"bare name counted once", "@task", 1, [][3]int{{3, 1, 6}}},
		{"uppercase", "@TASK", 1, [][3]int{{3, 1, 6}}},
		{"mixed case", "@TaSk", 1, [][3]int{{3, 1, 6}}},
		{"both edges", " \t@task \t", 3, [][3]int{{3, 3, 8}}},
		{"tab is one column", "\t@task", 2, [][3]int{{3, 2, 7}}},
		{"ordinary spaces", "@task   ", 1, [][3]int{{3, 1, 6}}},
		{"tab remainder", "@task\t", 1, [][3]int{{3, 1, 6}}},
		{"mixed remainder", "@task \t ", 1, [][3]int{{3, 1, 6}}},
	} {
		for _, recognized := range []bool{true, false} {
			t.Run(tc.name+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				line := v064Invalid(t, tc.raw, 2, 1)
				parent, depth := 2, 1
				if recognized {
					line = v064Attempt(t, tc.raw, tc.tagStart, 2, 1)
					parent, depth = 3, 2
				}
				result := v064Lesson(t, line, v064Text(t, "Read.", "Read.", parent, depth))
				v064Run(t, result, "current", tc.want)
			})
		}
	}
	t.Run("recognized tag on invalid line", func(t *testing.T) {
		line := v064Attempt(t, "@task\t", 1, 2, 1)
		line.LineType = "invalid"
		v064Run(t, v064Lesson(t, line, v064Text(t, "Read.", "Read.", 2, 1)),
			"current", [][3]int{{3, 1, 6}})
	})
}

func TestV064UnicodeWhitespace(t *testing.T) {
	// Множество White_Space задано независимо от проверяемого кода.
	// CR/LF не входят в Raw и проверяются как окончания строк.
	spaces := []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'}
	tails := []string{string(spaces) + " \t"}
	for _, space := range spaces {
		tails = append(tails, string(space))
	}
	for _, tail := range tails {
		for _, recognized := range []bool{true, false} {
			t.Run(strconv.Quote(tail)+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				line := v064Invalid(t, "@task"+tail, 2, 1)
				if recognized {
					line = v064Attempt(t, "@task"+tail, 1, 2, 1)
				}
				v064Run(t, v064Lesson(t, line), "current", [][3]int{{3, 1, 6}})
			})
		}
	}
}

func TestV064NonemptyArguments(t *testing.T) {
	for _, id := range []string{"practice", "PRACTICE", "упражнение"} {
		t.Run(id, func(t *testing.T) {
			v064Run(t, v064Lesson(t, v064Tag(t, "task", id, "identifier", 2, 1),
				v064Text(t, "Read.", "Read.", 3, 2)), "current", nil)
		})
	}
	t.Run("identifier between spaces", func(t *testing.T) {
		line := v064Line(t, " \t@task  practice \t", "tag", 2, 1,
			v064Element(t, "tag", "@task", "task", 3, 8),
			v064Element(t, "identifier", "practice", "practice", 10, 18))
		v064Run(t, v064Lesson(t, line, v064Text(t, "Read.", "Read.", 3, 2)), "current", nil)
	})
	for _, raw := range []string{"@task bad!", "@task bad id", "@task 😀", "@task\tpractice", "@task \u200b", `@task ""`} {
		for _, recognized := range []bool{true, false} {
			t.Run(strconv.Quote(raw)+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				// Ошибочный аргумент/разделитель не создаёт identifier,
				// однако непустота всего остатка определяется по Raw.
				line := v064Invalid(t, raw, 2, 1)
				parent, depth := 2, 1
				if recognized {
					line = v064Attempt(t, raw, 1, 2, 1)
					parent, depth = 3, 2
				}
				v064Run(t, v064Lesson(t, line, v064Text(t, "Read.", "Read.", parent, depth)), "current", nil)
			})
		}
	}
	for _, tc := range []struct {
		raw     string
		opening bool
	}{
		{"@task{", true}, {"@task {", true}, {"@task}", false}, {"@task }", false},
	} {
		for _, recognized := range []bool{true, false} {
			t.Run(tc.raw+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				// DSL 2.3.6: фигурная форма task запрещена. Не создаём
				// фиктивный блок; ошибочную открывающую попытку завершаем }.
				line := v064Invalid(t, tc.raw, 2, 1)
				if recognized {
					line = v064Attempt(t, tc.raw, 1, 2, 1)
					line.LineType = "invalid"
				}
				body := []model.Line{line, v064Text(t, "Read.", "Read.", 2, 1)}
				if tc.opening {
					body = append(body, v064Invalid(t, "}", 2, 1))
				}
				v064Run(t, v064Lesson(t, body...), "current", nil)
			})
		}
	}
}

func TestV064ExcludedNamesAndPrefixes(t *testing.T) {
	for _, raw := range []string{"@taskExtra", "@tasks", "@task1", "@task-name"} {
		t.Run(raw, func(t *testing.T) {
			v064Run(t, v064Lesson(t, v064Invalid(t, raw, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@task`, "@task"}, {" \t\\@task", "@task"}, {`\\@task`, `\@task`},
		{"Text @task", "Text @task"}, {"\u00a0@task", "\u00a0@task"}, {" \t\u2003@task", "\u2003@task"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v064Run(t, v064Lesson(t, v064Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v064Run(t, v064Lesson(t, v064Line(t, raw, "blank", 2, 1)), "current", nil)
		})
	}
	t.Run("other tag", func(t *testing.T) {
		v064Run(t, v064Lesson(t, v064Tag(t, "header", "Vocabulary", "title", 2, 1)), "current", nil)
	})
	t.Run("no task", func(t *testing.T) {
		v064Run(t, v064Lesson(t, v064Text(t, "Read.", "Read.", 2, 1)), "current", nil)
	})
}

func TestV064MisplacedAndNested(t *testing.T) {
	t.Run("before first level heading", func(t *testing.T) {
		line := v064Attempt(t, "@task", 1, 0, 0)
		line.LineType = "invalid"
		result := v064Document(t, line, v064Heading(t), v064Text(t, "Read.", "Read.", 3, 1))
		v064Run(t, result, "current", [][3]int{{2, 1, 6}})
	})
	for _, recognized := range []bool{true, false} {
		t.Run("text block/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
			line := v064Invalid(t, "@task", 3, 2)
			if recognized {
				line = v064Attempt(t, "@task", 1, 3, 2)
				line.LineType = "invalid"
			}
			result := v064Lesson(t, v064Open(t, "text", 2, 1), line,
				v064Text(t, "Read.", "Read.", 3, 2), v064Close(t, 3, 2))
			v064Run(t, result, "current", [][3]int{{4, 1, 6}})
		})
	}
}

func TestV064TasksInBranches(t *testing.T) {
	for _, tc := range []struct {
		name, firstID, secondID string
		want                    [][3]int
	}{
		{"both missing", "", "", [][3]int{{5, 1, 6}, {8, 1, 6}}},
		{"second missing", "practice", "", [][3]int{{8, 1, 6}}},
		{"both present", "practice", "writing", nil},
	} {
		for _, sharedParent := range []bool{false, true} {
			t.Run(tc.name+"/shared-parent="+strconv.FormatBool(sharedParent), func(t *testing.T) {
				// Два представления родителей уже используются в V033:
				// task принадлежит variant либо непосредственно variants.
				firstParent, secondParent, taskDepth := 4, 7, 3
				if sharedParent {
					firstParent, secondParent, taskDepth = 3, 3, 2
				}
				first := v064Attempt(t, "@task", 1, firstParent, taskDepth)
				second := v064Attempt(t, "@task", 1, secondParent, taskDepth)
				if tc.firstID != "" {
					first = v064Tag(t, "task", tc.firstID, "identifier", firstParent, taskDepth)
				}
				if tc.secondID != "" {
					second = v064Tag(t, "task", tc.secondID, "identifier", secondParent, taskDepth)
				}
				result := v064Lesson(t, v064Open(t, "variants", 2, 1),
					v064Tag(t, "variant", "A", "name", 3, 2),
					first, v064Text(t, "Read.", "Read.", 5, taskDepth+1),
					v064Tag(t, "variant", "B", "name", 3, 2),
					second, v064Text(t, "Write.", "Write.", 8, taskDepth+1), v064Close(t, 3, 2))
				v064Run(t, result, "current", tc.want)
			})
		}
	}
}

func TestV064LiteralContexts(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			t.Run("contents", func(t *testing.T) {
				v064Run(t, v064Lesson(t, v064Open(t, tag, 2, 1),
					v064Text(t, "@task", "@task", 3, 2), v064Close(t, 3, 2)), "current", nil)
			})
			t.Run("after closing", func(t *testing.T) {
				result := v064Lesson(t, v064Open(t, tag, 2, 1),
					v064Text(t, "@task", "@task", 3, 2), v064Close(t, 3, 2),
					v064Attempt(t, "@task", 1, 2, 1), v064Text(t, "Read.", "Read.", 6, 2))
				v064Run(t, result, "current", [][3]int{{6, 1, 6}})
			})
			t.Run("inside task", func(t *testing.T) {
				result := v064Lesson(t, v064Tag(t, "task", "practice", "identifier", 2, 1),
					v064Tag(t, "instr", "Read.", "content", 3, 2),
					v064Open(t, tag, 3, 2), v064Text(t, "@task", "@task", 5, 3), v064Close(t, 5, 3),
					v064Tag(t, "endtask", "", "", 2, 1))
				v064Run(t, result, "current", nil)
			})
			t.Run("single line form", func(t *testing.T) {
				result := v064Lesson(t, v064Tag(t, tag, "Write.", "content", 2, 1),
					v064Attempt(t, "@task", 1, 2, 1), v064Text(t, "Read.", "Read.", 4, 2))
				v064Run(t, result, "current", [][3]int{{4, 1, 6}})
			})
		})
	}
}

func TestV064SeveralAndIndependentRuns(t *testing.T) {
	t.Run("several declarations", func(t *testing.T) {
		result := v064Lesson(t, v064Attempt(t, "@task", 1, 2, 1), v064Text(t, "Read.", "Read.", 3, 2),
			v064Tag(t, "task", "practice", "identifier", 2, 1), v064Text(t, "Write.", "Write.", 5, 2),
			v064Attempt(t, "@task", 1, 2, 1), v064Text(t, "Speak.", "Speak.", 7, 2))
		v064Run(t, result, "current", [][3]int{{3, 1, 6}, {7, 1, 6}})
	})
	t.Run("independent and repeated", func(t *testing.T) {
		result := v064Lesson(t, v064Attempt(t, "@task", 1, 2, 1), v064Text(t, "Read.", "Read.", 3, 2))
		v064Run(t, result, "first", [][3]int{{3, 1, 6}})
		reference := v064Lesson(t, v064Tag(t, "task", "practice", "identifier", 2, 1),
			v064Tag(t, "step", "Read", "title", 3, 2), v064Text(t, "Read.", "Read.", 4, 3),
			v064Tag(t, "step", "Write", "title", 3, 2), v064Text(t, "Write.", "Write.", 6, 3))
		v064Run(t, reference, "independent", nil)
		v064Run(t, result, "second", [][3]int{{3, 1, 6}})
	})
}

func TestV064EndingsAndVersions(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		for _, recognized := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				line := v064Invalid(t, "@task", 2, 1)
				if recognized {
					line = v064Attempt(t, "@task", 1, 2, 1)
				}
				result := v064Lesson(t, line)
				if ending == "\r\n" {
					for i := range result.Lines {
						result.Lines[i].LineEnding = ending
					}
				}
				result.Lines[2].LineEnding = ending
				v064Run(t, result, "current", [][3]int{{3, 1, 6}})
			})
		}
	}
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version+" with unavailable lines", func(t *testing.T) {
			// Версия известна, физические строки недоступны.
			result := v064Lesson(t)
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			result.Document.Metadata.Title = nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v064Run(t, result, "current", nil)
				return
			}
			before := v064Snapshot(t, result)
			checks, err := validator.Prepare(result, v064Registry(t))
			if err == nil || checks != nil || v064Snapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v064Lesson добавляет обязательный заголовок; body начинается со строки 3.
func v064Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v064Document(t, append([]model.Line{v064Heading(t)}, body...)...)
}

// v064Document добавляет версию перед готовыми строками, содержащими # Lesson.
// Для явно заданных unparsed создаёт согласованные исходные ошибки и ссылки.
func v064Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v064Tag(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
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

// v064Attempt задаёт распознанное имя task на указанной Unicode-позиции
// и ошибочный остаток целиком в unparsed, включая допустимый пустой диапазон.
func v064Attempt(t *testing.T, raw string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + 5
	return v064Line(t, raw, "tag", parent, depth,
		v064Element(t, "tag", string(runes[start-1:end-1]), "task", start, end),
		v064Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v064Heading задаёт обязательный заголовок первого уровня.
func v064Heading(t *testing.T) model.Line {
	t.Helper()
	return v064Line(t, "# Lesson", "heading", 0, 0,
		v064Element(t, "heading-level", "#", "1", 1, 2),
		v064Element(t, "title", "Lesson", "Lesson", 3, 9))
}

// v064Invalid задаёт целиком неразобранную строку без элемента tag.
func v064Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v064Line(t, raw, "invalid", parent, depth, v064Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v064Text задаёт Raw и семантическое значение независимо.
func v064Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v064Line(t, raw, "content", parent, depth, v064Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v064Tag задаёт известный однострочный тег с необязательным аргументом.
func v064Tag(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v064Element(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v064Element(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v064Line(t, raw, "tag", parent, depth, elements...)
}

// v064Open задаёт блок с каноническим семантическим именем тега.
func v064Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v064Line(t, "@"+tag+" {", "block-start", parent, depth,
		v064Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v064Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v064Close задаёт закрытие блока с явным родителем.
func v064Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v064Line(t, "}", "block-end", parent, depth, v064Element(t, "block-close", "}", "}", 1, 2))
}

// v064Line сохраняет явно заданную структуру без классификации Raw.
func v064Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v064Element задаёт исходные Unicode-колонки с исключающим концом.
func v064Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v064Registry создаёт реестр из полного штатного каталога.
func v064Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v064Run проверяет новые V064 по явным тройкам строка/начало/исключающий конец.
// Конвейер выполняет полный каталог; результаты других диагностик не отбираются.
func v064Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v064Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v064Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v064Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V064" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V064 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "У @task отсутствует ID." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V064 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v064Snapshot(t, result)), &restored); err != nil {
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
	if v064Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v064Snapshot сохраняет независимое представление модели для сравнения.
func v064Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

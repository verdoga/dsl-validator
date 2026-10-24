package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v070TestMessage = "@speaking находится вне активного задания."

// v070TestWant содержит явный исходный диапазон с исключающей правой границей.
type v070TestWant struct{ line, start, end int }

func TestV070RegistrationAndVersions(t *testing.T) {
	registry := v070TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V070" {
			continue
		}
		count++
		if definition.Message != v070TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V070 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if definitions, err := registry.ForVersion(dslversions.Version(version)); err == nil || definitions != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", definitions, err)
			}
			result := v070TestDocument(t)
			result.Document.DSLVersion = &version
			// Версия известна, а исходные строки не предоставлены.
			result.Lines = []model.Line{}
			result.Document.LineCount = nil
			result.Document.Metadata.Title = nil
			before := v070TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v070TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV070ReferenceAndExample(t *testing.T) {
	// Перед неизменёнными фрагментами добавлены только версия и основной
	// заголовок. Первая строка каждого исходного фрагмента получает номер 3.
	t.Run("reference", func(t *testing.T) {
		v070TestRun(t, v070TestDocument(t,
			v070TestTag(t, "task", "discussion", "identifier", 2),
			v070TestTag(t, "speaking", "", "", 3),
			v070TestTag(t, "step", "Prepare", "title", 3), v070TestText(t, "Read.", "Read.", 5),
			v070TestTag(t, "step", "Discuss", "title", 3), v070TestText(t, "Speak.", "Speak.", 7)),
			"current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v070TestRun(t, v070TestDocument(t, v070TestTag(t, "speaking", "", "", 2),
			v070TestText(t, "Discuss the question.", "Discuss the question.", 2)),
			"current", []v070TestWant{{3, 1, 10}})
	})
}

func TestV070TaskTransitions(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v070TestWant
	}{
		{"before and after task", []model.Line{
			v070TestTag(t, "speaking", "", "", 2), v070TestTag(t, "task", "a", "identifier", 2),
			v070TestText(t, "Read.", "Read.", 4), v070TestTag(t, "speaking", "", "", 4),
			v070TestTag(t, "endtask", "", "", 4), v070TestTag(t, "speaking", "", "", 2),
		}, []v070TestWant{{3, 1, 10}, {8, 1, 10}}},
		{"repeated and late speaking is not checked", []model.Line{
			v070TestTag(t, "task", "a", "identifier", 2), v070TestText(t, "Read.", "Read.", 3),
			v070TestTag(t, "speaking", "", "", 3), v070TestTag(t, "speaking", "", "", 3),
		}, nil},
		{"next task replaces previous", []model.Line{
			v070TestTag(t, "task", "a", "identifier", 2), v070TestText(t, "Read.", "Read.", 3),
			v070TestTag(t, "task", "b", "identifier", 2), v070TestTag(t, "speaking", "", "", 5),
			v070TestText(t, "Speak.", "Speak.", 5),
		}, nil},
		{"task reopens after endtask", []model.Line{
			v070TestTag(t, "task", "a", "identifier", 2), v070TestText(t, "Read.", "Read.", 3),
			v070TestTag(t, "endtask", "", "", 3), v070TestTag(t, "task", "b", "identifier", 2),
			v070TestTag(t, "speaking", "", "", 6), v070TestText(t, "Speak.", "Speak.", 6),
		}, nil},
		{"blank and editor preserve task", []model.Line{
			v070TestTag(t, "task", "a", "identifier", 2), v070TestLine(t, "", "blank", 3),
			v070TestTag(t, "editor", "Note.", "content", 3), v070TestTag(t, "speaking", "", "", 3),
			v070TestText(t, "Read.", "Read.", 3),
		}, nil},
		{"step preserves active task", []model.Line{
			v070TestTag(t, "task", "a", "identifier", 2), v070TestTag(t, "step", "Prepare", "title", 3),
			v070TestText(t, "Read.", "Read.", 4), v070TestTag(t, "speaking", "", "", 4),
		}, nil},
		{"step alone does not open task", []model.Line{
			v070TestTag(t, "step", "Prepare", "title", 2), v070TestText(t, "Read.", "Read.", 3),
			v070TestTag(t, "speaking", "", "", 3),
		}, []v070TestWant{{5, 1, 10}}},
		{"unparsed task argument still opens task", []model.Line{
			v070TestTag(t, "task", "!", "unparsed", 2), v070TestTag(t, "speaking", "", "", 3),
			v070TestText(t, "Read.", "Read.", 3),
		}, nil},
		{"all names are case insensitive", []model.Line{
			v070TestTag(t, "TaSk", "a", "identifier", 2), v070TestTag(t, "SpEaKiNg", "", "", 3),
			v070TestText(t, "Read.", "Read.", 3), v070TestTag(t, "EnDtAsK", "", "", 3),
			v070TestTag(t, "SpEaKiNg", "", "", 2),
		}, []v070TestWant{{7, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v070TestRun(t, v070TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV070Terminators(t *testing.T) {
	for _, tc := range []struct {
		name       string
		line       model.Line
		nextParent int
	}{
		{"endtask", v070TestTag(t, "endtask", "", "", 3), 2},
		{"header", v070TestTag(t, "header", "Theory", "title", 2), 2},
		{"newpage", v070TestTag(t, "newpage", "", "", 0), 5},
		{"heading three", v070TestHeading(t, 3, "Next"), 5},
		{"uppercase header", v070TestTag(t, "HEADER", "Theory", "title", 2), 2},
		{"uppercase newpage", v070TestTag(t, "NEWPAGE", "", "", 0), 5},
		// Ошибочный newpage не создаёт допустимый шаг модели, но известное
		// имя завершает задание именно по требованиям V070.
		{"endtask with unparsed argument", v070TestTag(t, "endtask", "extra", "unparsed", 3), 2},
		{"newpage with unparsed argument", v070TestTag(t, "newpage", "extra", "unparsed", 3), 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v070TestDocument(t, v070TestTag(t, "task", "a", "identifier", 2),
				v070TestText(t, "Read.", "Read.", 3), tc.line,
				v070TestTag(t, "speaking", "", "", tc.nextParent),
				v070TestText(t, "More.", "More.", tc.nextParent))
			v070TestRun(t, result, "current", []v070TestWant{{6, 1, 10}})
		})
	}
	for _, level := range []int{1, 2} {
		t.Run("heading does not terminate "+strconv.Itoa(level), func(t *testing.T) {
			result := v070TestDocument(t, v070TestTag(t, "task", "a", "identifier", 2),
				v070TestText(t, "Read.", "Read.", 3), v070TestHeading(t, level, "Again"),
				v070TestTag(t, "speaking", "", "", 3))
			// По DSL 2.1.3–2.1.4 размещение ошибочно, но распознанный
			// уровень 1/2 не входит в перечень завершителей V070.
			v070TestParserError(t, result, 5, 0, "Недопустимое размещение заголовка.")
			v070TestRun(t, result, "current", nil)
		})
	}
}

func TestV070SourceRangesAndNonDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v070TestWant
	}{
		{"indented mixed case", v070TestLine(t, " \t@SpEaKiNg", "tag", 2,
			v070TestElement(t, "tag", "@SpEaKiNg", "speaking", 3)), []v070TestWant{{3, 3, 12}}},
		{"trailing whitespace excluded", v070TestLine(t, "@speaking \t", "tag", 2,
			v070TestElement(t, "tag", "@speaking", "speaking", 1)), []v070TestWant{{3, 1, 10}}},
		{"unparsed argument", v070TestTag(t, "speaking", "oral", "unparsed", 2), []v070TestWant{{3, 1, 10}}},
		{"Unicode argument", v070TestTag(t, "speaking", "устно😀", "unparsed", 2), []v070TestWant{{3, 1, 10}}},
		{"escaped beginning", v070TestText(t, "\\@speaking", "@speaking", 2), nil},
		{"inline mention", v070TestText(t, "Text @speaking", "Text @speaking", 2), nil},
		{"inline editor", v070TestTag(t, "editor", "@speaking", "content", 2), nil},
		{"different name", v070TestInvalid(t, "@speakingExtra", 2), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v070TestRun(t, v070TestDocument(t, tc.line, v070TestText(t, "Read.", "Read.", 2)), "current", tc.want)
		})
	}
	t.Run("escaped task does not open task", func(t *testing.T) {
		v070TestRun(t, v070TestDocument(t, v070TestText(t, "\\@task a", "@task a", 2),
			v070TestTag(t, "speaking", "", "", 2)), "current", []v070TestWant{{4, 1, 10}})
	})
}

func TestV070ExcludedBlocks(t *testing.T) {
	for _, tag := range []string{"editor", "EDITOR", "multifill", "MULTIFILL"} {
		for _, active := range []bool{false, true} {
			t.Run(tag+" active="+strconv.FormatBool(active), func(t *testing.T) {
				body := []model.Line{}
				parent, opening, last := 2, 3, 8
				want := []v070TestWant{{8, 1, 10}}
				if active {
					body = append(body, v070TestTag(t, "task", "a", "identifier", 2),
						v070TestText(t, "Read.", "Read.", 3))
					parent, opening, last, want = 3, 5, 10, nil
				}
				body = append(body, v070TestOpen(t, tag, parent),
					v070TestText(t, "@task hidden", "@task hidden", opening),
					v070TestText(t, "@endtask", "@endtask", opening),
					v070TestText(t, "@speaking", "@speaking", opening), v070TestClose(t, opening),
					v070TestTag(t, "speaking", "", "", parent))
				result := v070TestDocument(t, body...)
				if result.Lines[last-1].Raw != "@speaking" {
					t.Fatal("fixture last speaking moved")
				}
				v070TestRun(t, result, "current", want)
			})
		}
	}
	t.Run("nested task does not open outer task", func(t *testing.T) {
		result := v070TestDocument(t, v070TestOpen(t, "text", 2),
			v070TestTag(t, "task", "inner", "identifier", 3), v070TestTag(t, "speaking", "", "", 3),
			v070TestClose(t, 3), v070TestTag(t, "speaking", "", "", 2))
		v070TestParserError(t, result, 4, 0, "Вложенный тег запрещён.")
		v070TestParserError(t, result, 5, 0, "Вложенный тег запрещён.")
		v070TestRun(t, result, "current", []v070TestWant{{7, 1, 10}})
	})
	t.Run("nested endtask and block close preserve outer task", func(t *testing.T) {
		result := v070TestDocument(t, v070TestTag(t, "task", "outer", "identifier", 2),
			v070TestText(t, "Read.", "Read.", 3), v070TestOpen(t, "text", 3),
			v070TestTag(t, "endtask", "", "", 5), v070TestTag(t, "speaking", "", "", 5),
			v070TestClose(t, 5), v070TestTag(t, "speaking", "", "", 3))
		// Сохранены распознанные, но запрещённые в text вложенные теги.
		v070TestParserError(t, result, 6, 0, "Вложенный тег запрещён.")
		v070TestParserError(t, result, 7, 0, "Вложенный тег запрещён.")
		v070TestRun(t, result, "current", nil)
	})
	t.Run("fragment is not expanded", func(t *testing.T) {
		fragment := v070TestLine(t, "@fragment f {", "block-start", 2,
			v070TestElement(t, "tag", "@fragment", "fragment", 1),
			v070TestElement(t, "identifier", "f", "f", 11),
			v070TestElement(t, "block-open", "{", "{", 13))
		result := v070TestDocument(t, fragment, v070TestTag(t, "speaking", "", "", 3),
			v070TestText(t, "Read.", "Read.", 3), v070TestClose(t, 3),
			v070TestTag(t, "speaking", "", "", 2))
		v070TestParserError(t, result, 4, 0, "Тег speaking запрещён внутри fragment.")
		v070TestRun(t, result, "current", []v070TestWant{{7, 1, 10}})
	})
}

func TestV070Variants(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v070TestWant
	}{
		{"entry closes outer task", []model.Line{
			v070TestTag(t, "task", "outer", "identifier", 2), v070TestText(t, "Read.", "Read.", 3),
			v070TestOpen(t, "variants", 2), v070TestTag(t, "variant", "A", "name", 5),
			v070TestTag(t, "speaking", "", "", 6), v070TestText(t, "Read A.", "Read A.", 6),
			v070TestTag(t, "variant", "B", "name", 5), v070TestText(t, "Read B.", "Read B.", 9),
			v070TestClose(t, 5),
		}, []v070TestWant{{7, 1, 10}}},
		{"next branch closes task", []model.Line{
			v070TestOpen(t, "variants", 2), v070TestTag(t, "variant", "A", "name", 3),
			v070TestTag(t, "task", "a", "identifier", 4), v070TestTag(t, "speaking", "", "", 5),
			v070TestText(t, "Read A.", "Read A.", 5), v070TestTag(t, "variant", "B", "name", 3),
			v070TestTag(t, "speaking", "", "", 8), v070TestText(t, "Read B.", "Read B.", 8),
			v070TestClose(t, 3),
		}, []v070TestWant{{9, 1, 10}}},
		{"closing variants closes last branch task", []model.Line{
			v070TestOpen(t, "variants", 2), v070TestTag(t, "variant", "A", "name", 3),
			v070TestTag(t, "task", "a", "identifier", 4), v070TestTag(t, "speaking", "", "", 5),
			v070TestText(t, "Read A.", "Read A.", 5), v070TestTag(t, "variant", "B", "name", 3),
			v070TestTag(t, "task", "b", "identifier", 8), v070TestTag(t, "speaking", "", "", 9),
			v070TestText(t, "Read B.", "Read B.", 9), v070TestClose(t, 3),
			v070TestTag(t, "speaking", "", "", 2),
		}, []v070TestWant{{13, 1, 10}}},
		{"outer task is not restored", []model.Line{
			v070TestTag(t, "task", "outer", "identifier", 2), v070TestText(t, "Read.", "Read.", 3),
			v070TestOpen(t, "variants", 2), v070TestTag(t, "variant", "A", "name", 5),
			v070TestText(t, "Read A.", "Read A.", 6), v070TestTag(t, "variant", "B", "name", 5),
			v070TestText(t, "Read B.", "Read B.", 8), v070TestClose(t, 5),
			v070TestTag(t, "speaking", "", "", 2),
		}, []v070TestWant{{11, 1, 10}}},
		{"literal branch switch is ignored", []model.Line{
			v070TestOpen(t, "VARIANTS", 2), v070TestTag(t, "VARIANT", "A", "name", 3),
			v070TestTag(t, "TASK", "a", "identifier", 4), v070TestOpen(t, "editor", 5),
			v070TestText(t, "@variant Hidden", "@variant Hidden", 6), v070TestClose(t, 6),
			v070TestTag(t, "SPEAKING", "", "", 5), v070TestText(t, "Read A.", "Read A.", 5),
			v070TestTag(t, "VARIANT", "B", "name", 3), v070TestTag(t, "SPEAKING", "", "", 11),
			v070TestText(t, "Read B.", "Read B.", 11), v070TestClose(t, 3),
		}, []v070TestWant{{12, 1, 10}}},
		{"task after variants opens normally", []model.Line{
			v070TestOpen(t, "variants", 2), v070TestTag(t, "variant", "A", "name", 3),
			v070TestText(t, "Read A.", "Read A.", 4), v070TestTag(t, "variant", "B", "name", 3),
			v070TestText(t, "Read B.", "Read B.", 6), v070TestClose(t, 3),
			v070TestTag(t, "task", "after", "identifier", 2), v070TestTag(t, "speaking", "", "", 9),
			v070TestText(t, "Read.", "Read.", 9),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v070TestRun(t, v070TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("variant outside a block does not close task", func(t *testing.T) {
		result := v070TestDocument(t, v070TestTag(t, "task", "a", "identifier", 2),
			v070TestText(t, "Read.", "Read.", 3), v070TestTag(t, "variant", "Stray", "name", 3),
			v070TestTag(t, "speaking", "", "", 3))
		// Дополнительная граница задания возникает только в ветви variants.
		v070TestParserError(t, result, 5, 0, "Тег variant находится вне variants.")
		v070TestRun(t, result, "current", nil)
	})
}

func TestV070EndingsEmptyAndHistory(t *testing.T) {
	t.Run("speaking on first physical line", func(t *testing.T) {
		result := v070TestDocument(t)
		// Единственный speaking предшествует поздней версии и заголовку.
		result.Lines = append([]model.Line{v070TestTag(t, "speaking", "", "", 0)}, result.Lines...)
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		count := 3
		result.Document.LineCount = &count
		v070TestRun(t, result, "current", []v070TestWant{{1, 1, 10}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v070TestDocument(t, v070TestTag(t, "speaking", "", "", 2))
			if ending == "" {
				result.Lines[2].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v070TestRun(t, result, "first", []v070TestWant{{3, 1, 10}})
			// Завершающееся вместе с файлом задание не влияет на другой документ.
			v070TestRun(t, v070TestDocument(t, v070TestTag(t, "task", "a", "identifier", 2),
				v070TestText(t, "Read.", "Read.", 3)), "other", nil)
			v070TestRun(t, result, "second", []v070TestWant{{3, 1, 10}})
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("no lines unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			// Контракт модели допускает известную версию при недоступных строках.
			result := v070TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v070TestRun(t, result, "current", nil)
		})
	}
	t.Run("unknown version stops before checks", func(t *testing.T) {
		// У пустого исходника нет объявления версии и извлечённых метаданных.
		result := v070TestDocument(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		result.Document.DSLVersion = nil
		if err := validator.CheckModel(result); err != nil {
			t.Fatalf("input integrity: %v", err)
		}
		before := v070TestSnapshot(t, result)
		if checks, err := validator.Prepare(result, v070TestRegistry(t)); err == nil || checks != nil {
			t.Fatalf("unknown version: checks=%v, err=%v", checks, err)
		}
		if v070TestSnapshot(t, result) != before {
			t.Fatal("failed Prepare changed input")
		}
	})
}

// v070TestLine сохраняет готовую классификацию и родителя, не разбирая Raw.
func v070TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v070TestElement вычисляет длину входного элемента, но не ожидаемый диапазон.
func v070TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v070TestTag строит тег с явно заданным типом аргумента, включая unparsed.
func v070TestTag(t *testing.T, tag, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v070TestElement(t, "tag", raw, strings.ToLower(tag), 1)}
	if argument != "" {
		elements = append(elements, v070TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v070TestLine(t, raw, "tag", parent, elements...)
}

// v070TestHeading строит распознанный корневой заголовок.
func v070TestHeading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v070TestLine(t, marker+" "+title, "heading", 0,
		v070TestElement(t, "heading-level", marker, strconv.Itoa(level), 1),
		v070TestElement(t, "title", title, title, level+2))
}

// v070TestText сохраняет исходное и семантическое содержимое независимо.
func v070TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v070TestLine(t, raw, "content", parent, v070TestElement(t, "content", raw, value, 1))
}

// v070TestInvalid задаёт неразобранную ошибочную конструкцию.
func v070TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v070TestLine(t, raw, "invalid", parent, v070TestElement(t, "unparsed", raw, "", 1))
}

// v070TestOpen строит блочную форму с явно заданным родителем.
func v070TestOpen(t *testing.T, tag string, parent int) model.Line {
	t.Helper()
	line := v070TestTag(t, tag, "", "", parent)
	line.Elements = append(line.Elements, v070TestElement(t, "block-open", "{", "{", len(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v070TestClose связывает закрытие с открывающей строкой блока.
func v070TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v070TestLine(t, "}", "block-end", parent, v070TestElement(t, "block-close", "}", "}", 1))
}

// v070TestDocument добавляет версию и основной заголовок. Глубины определяются
// явными родителями; элементы не извлекаются повторным разбором исходника.
func v070TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v070TestTag(t, "dsl-version", "1.2", "version", 0),
		v070TestHeading(t, 1, "Lesson")}, body...)
	version, count := "1.2", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	titles := map[string][]string{}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		level, title := "", ""
		for j := range line.Elements {
			element := &line.Elements[j]
			element.ErrorIDs = append([]string{}, element.ErrorIDs...)
			switch element.ElementType {
			case "unparsed":
				v070TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			case "heading-level":
				level = *element.Value
			case "title":
				title = *element.Value
			}
		}
		if line.LineType == "heading" {
			titles[level] = append(titles[level], title)
		}
	}
	if len(titles["1"]) == 1 {
		result.Document.Metadata.Title = &titles["1"][0]
	}
	if len(titles["2"]) == 1 {
		result.Document.Metadata.Subtitle = &titles["2"][0]
	}
	return result
}

// v070TestParserError добавляет исходную ошибку с согласованными флагами и ссылкой.
func v070TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}},
	})
}

// v070TestRegistry использует полный штатный каталог без подмены регистраций.
func v070TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v070TestRun сравнивает новые V070 после штатных Prepare/Run с явным ожиданием.
func v070TestRun(t *testing.T, result *model.Result, processingID string, want []v070TestWant) {
	t.Helper()
	before := v070TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v070TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v070TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V070" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V070 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v070TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V070 occurrence %d = %+v, want %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V070 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V070 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v070TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Только добавления конвейера и производные флаги/ссылки исключаются
	// из сравнения исходных строк, метаданных и прежней истории.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v070TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v070TestSnapshot сохраняет представление модели для проверки неизменности.
func v070TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

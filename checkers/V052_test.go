package checkers

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const (
	v052TestMessage  = "Заголовок второго уровня находится в недопустимом месте."
	v052TestNested   = "Заголовок вложен в фигурный блок."
	v052TestBefore   = "Заголовок расположен до основного заголовка."
	v052TestObstacle = "Перед заголовком находится недопустимая конструкция."
)

// v052TestWant содержит заданные тестом исходные диапазоны, конец исключается.
// Нулевой related означает отсутствие связанных диапазонов.
type v052TestWant struct {
	location [3]int
	reason   string
	related  [3]int
}

func TestV052Registration(t *testing.T) {
	definitions, err := v052TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V052" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != v052TestMessage || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V052 definitions = %d, want 1", matches)
	}
}

func TestV052Positions(t *testing.T) {
	h := v052TestHeading(t, 1, "Lesson")
	s := v052TestHeading(t, 2, "Vocabulary")
	r := v052TestText(t, "Read carefully.", "Read carefully.", 2)
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v052TestWant
	}{
		{"no candidates", []model.Line{h, r}, nil},
		{"immediately after main", []model.Line{h, s}, nil},
		// К обоим обязательным фрагментам добавлена только версия на строке 1.
		{"reference verbatim", []model.Line{
			h, v052TestOpen(t, "fragment", "intro", 2),
			v052TestText(t, "Read carefully.", "Read carefully.", 3), v052TestClose(t, 3),
			s, v052TestTag(t, "include", "intro", "identifier", 6),
		}, nil},
		{"example verbatim", []model.Line{h, r, s},
			[]v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 16}}}},
		{"repetition alone", []model.Line{h, s, s}, nil},
		{"malformed previous candidate is not obstacle", []model.Line{
			h, v052TestInvalid(t, "##Missing separator", 2), s,
		}, nil},
		{"each repetition after obstacle", []model.Line{h, r, s, s},
			[]v052TestWant{
				{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 16}},
				{[3]int{5, 1, 14}, v052TestObstacle, [3]int{3, 1, 16}},
			}},
		{"first obstacle only", []model.Line{h, r, v052TestTag(t, "note", "Note", "content", 2), s},
			[]v052TestWant{{[3]int{5, 1, 14}, v052TestObstacle, [3]int{3, 1, 16}}}},
		{"first main is not replaced", []model.Line{h, v052TestHeading(t, 1, "Later"), s},
			[]v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 8}}}},
		{"before existing main", []model.Line{s, h},
			[]v052TestWant{{[3]int{2, 1, 14}, v052TestBefore, [3]int{}}}},
		{"first and last candidates", []model.Line{s, h, s},
			[]v052TestWant{{[3]int{2, 1, 14}, v052TestBefore, [3]int{}}}},
		{"no main", []model.Line{s}, nil},
		{"no main despite prior text", []model.Line{
			v052TestText(t, "Read carefully.", "Read carefully.", 0), s,
		}, nil},
		{"nested without main", []model.Line{
			v052TestOpen(t, "text", "", 0), v052TestInvalid(t, "## Vocabulary", 2), v052TestClose(t, 2),
		}, []v052TestWant{{[3]int{3, 1, 14}, v052TestNested, [3]int{}}}},
		{"nested before main combines reasons", []model.Line{
			v052TestOpen(t, "text", "", 0), v052TestInvalid(t, "## Vocabulary", 2), v052TestClose(t, 2), h,
		}, []v052TestWant{{[3]int{3, 1, 14}, v052TestNested + " " + v052TestBefore, [3]int{}}}},
		{"nested after main combines reasons", []model.Line{
			h, v052TestOpen(t, "text", "", 2), v052TestInvalid(t, "## Vocabulary", 3), v052TestClose(t, 3),
		}, []v052TestWant{{[3]int{4, 1, 14}, v052TestNested + " " + v052TestObstacle, [3]int{3, 1, 8}}}},
		{"deep nesting", []model.Line{
			h, v052TestOpen(t, "instr", "", 2), v052TestOpen(t, "hint", "", 3),
			v052TestInvalid(t, "## Vocabulary", 4), v052TestClose(t, 4), v052TestClose(t, 3),
		}, []v052TestWant{{[3]int{5, 1, 14}, v052TestNested + " " + v052TestObstacle, [3]int{3, 1, 9}}}},
		{"fragment is skipped but its candidate is checked", []model.Line{
			h, v052TestOpen(t, "fragment", "intro", 2),
			v052TestInvalid(t, "## Vocabulary", 3), v052TestClose(t, 3), s,
		}, []v052TestWant{{[3]int{4, 1, 14}, v052TestNested, [3]int{}}}},
		{"nested fragment content and both closing lines skipped", []model.Line{
			h, v052TestOpen(t, "fragment", "intro", 2), v052TestOpen(t, "text", "", 3),
			v052TestText(t, "Read carefully.", "Read carefully.", 4),
			v052TestClose(t, 4), v052TestClose(t, 3), s,
		}, nil},
		{"text after fragment is obstacle", []model.Line{
			h, v052TestOpen(t, "fragment", "intro", 2), v052TestText(t, "Read.", "Read.", 3),
			v052TestClose(t, 3), r, s,
		}, []v052TestWant{{[3]int{7, 1, 14}, v052TestObstacle, [3]int{6, 1, 16}}}},
		{"multifill declaration is obstacle", []model.Line{
			h, v052TestOpen(t, "multifill", "", 2), v052TestText(t, "Read carefully.", "Read carefully.", 3),
			v052TestClose(t, 3), s,
		}, []v052TestWant{{[3]int{6, 1, 14}, v052TestObstacle, [3]int{3, 1, 13}}}},
		{"escaped heading is obstacle", []model.Line{
			h, v052TestText(t, `\# Text`, "# Text", 2), s,
		}, []v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 8}}}},
		{"escaped editor is obstacle", []model.Line{
			h, v052TestText(t, `\@editor Note`, "@editor Note", 2), s,
		}, []v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 14}}}},
		{"nested main candidate is not main", []model.Line{
			v052TestOpen(t, "text", "", 0), v052TestInvalid(t, "# Lesson", 2),
			v052TestClose(t, 2), s,
		}, nil},
		{"nested main does not hide later root main", []model.Line{
			v052TestOpen(t, "text", "", 0), v052TestInvalid(t, "# Lesson", 2),
			v052TestClose(t, 2), s, h,
		}, []v052TestWant{{[3]int{5, 1, 14}, v052TestBefore, [3]int{}}}},
		{"blank and editor variants", []model.Line{
			h, v052TestLine(t, "", "blank", 2),
			v052TestLine(t, " \t ", "blank", 2),
			v052TestTag(t, "editor", "Comment.", "content", 2),
			v052TestOpen(t, "EDITOR", "", 2),
			v052TestText(t, "## Literal", "## Literal", 6), v052TestClose(t, 6), s,
		}, nil},
		{"multiple definitions remain invisible", []model.Line{
			h, v052TestOpen(t, "fragment", "one", 2), v052TestText(t, "One.", "One.", 3), v052TestClose(t, 3),
			v052TestOpen(t, "FRAGMENT", "two", 2), v052TestText(t, "Two.", "Two.", 6), v052TestClose(t, 6), s,
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v052TestRun(t, v052TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV052ObstacleKinds(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		end  int
	}{
		{"task", v052TestTag(t, "task", "1", "identifier", 2), 8},
		{"header", v052TestTag(t, "header", "Intro", "title", 2), 14},
		{"include", v052TestTag(t, "include", "intro", "identifier", 2), 15},
		{"third heading", v052TestHeading(t, 3, "Step"), 9},
		{"newpage", v052TestTag(t, "newpage", "", "", 2), 9},
		{"displayed note", v052TestTag(t, "note", "Note", "content", 2), 11},
		{"unknown construction", v052TestInvalid(t, "@unknown", 2), 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v052TestHeading(t, 1, "Lesson"), tc.line, v052TestHeading(t, 2, "Vocabulary")}
			if tc.name == "include" {
				// Существующее определение после включения разрешено DSL 3.12.9.
				body = append(body, v052TestOpen(t, "fragment", "intro", 4),
					v052TestText(t, "Read.", "Read.", 5), v052TestClose(t, 5))
			}
			v052TestRun(t, v052TestDocument(t, body...), "current",
				[]v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, tc.end}}})
		})
	}
}

func TestV052Candidates(t *testing.T) {
	for _, tc := range []struct {
		raw string
		end int
	}{
		{"##", 3}, {"##X", 4}, {"##\tX", 5}, {"##  \t", 6},
		{"  \t##X", 7}, {"##Я###", 7},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
				v052TestText(t, "Read carefully.", "Read carefully.", 2), v052TestInvalid(t, tc.raw, 2))
			v052TestRun(t, result, "current",
				[]v052TestWant{{[3]int{4, 1, tc.end}, v052TestObstacle, [3]int{3, 1, 16}}})
		})
	}
	for _, raw := range []string{"#Lesson", "#", "#\tLesson", " \t#Lesson"} {
		t.Run("malformed main "+raw, func(t *testing.T) {
			v052TestRun(t, v052TestDocument(t, v052TestInvalid(t, raw, 0),
				v052TestHeading(t, 2, "Vocabulary")), "current", nil)
			// Этот положительный случай отличает найденный ошибочный #
			// от отсутствующего основного заголовка (уточнение пользователя B).
			v052TestRun(t, v052TestDocument(t, v052TestInvalid(t, raw, 0),
				v052TestText(t, "Read carefully.", "Read carefully.", 2),
				v052TestHeading(t, 2, "Vocabulary")), "current",
				[]v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 16}}})
		})
	}
	t.Run("hash in title does not extend marker", func(t *testing.T) {
		v052TestRun(t, v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
			v052TestText(t, "Read carefully.", "Read carefully.", 2), v052TestHeading(t, 2, "# X")), "current",
			[]v052TestWant{{[3]int{4, 1, 7}, v052TestObstacle, [3]int{3, 1, 16}}})
	})
	for _, tc := range []struct {
		raw, value string
	}{
		{`\## X`, "## X"}, {`\\## X`, `\## X`}, {"Text ## X", "Text ## X"},
		{"＃＃ X", "＃＃ X"}, {"\u00a0## X", "\u00a0## X"},
		{" \t\u2003## X", "\u2003## X"}, {" \t\\## X", "## X"},
	} {
		t.Run("not candidate "+tc.raw, func(t *testing.T) {
			v052TestRun(t, v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
				v052TestText(t, "Read carefully.", "Read carefully.", 2),
				v052TestText(t, tc.raw, tc.value, 2)), "current", nil)
		})
	}
	for _, line := range []model.Line{
		v052TestHeading(t, 1, "X"), v052TestHeading(t, 3, "X"), v052TestInvalid(t, "#### X", 2),
	} {
		t.Run("other level "+line.Raw, func(t *testing.T) {
			v052TestRun(t, v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
				v052TestText(t, "Read carefully.", "Read carefully.", 2), line), "current", nil)
		})
	}
	t.Run("recognized level and unparsed title", func(t *testing.T) {
		bad := v052TestElement(t, "unparsed", "X", "", 4)
		bad.Value = nil
		line := v052TestLine(t, "##\tX", "invalid", 2,
			v052TestElement(t, "heading-level", "##", "2", 1), bad)
		v052TestRun(t, v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
			v052TestText(t, "Read carefully.", "Read carefully.", 2), line), "current",
			[]v052TestWant{{[3]int{4, 1, 5}, v052TestObstacle, [3]int{3, 1, 16}}})
	})
	t.Run("source indentation Unicode and trailing space", func(t *testing.T) {
		line := v052TestLine(t, " \t## Я😀 ", "heading", 0,
			v052TestElement(t, "heading-level", "##", "2", 3),
			v052TestElement(t, "title", "Я😀", "Я😀", 6))
		v052TestRun(t, v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
			v052TestText(t, "Read carefully.", "Read carefully.", 2), line), "current",
			[]v052TestWant{{[3]int{4, 1, 9}, v052TestObstacle, [3]int{3, 1, 16}}})
	})
}

func TestV052LiteralContexts(t *testing.T) {
	for _, tag := range []string{"editor", "EDITOR", "multifill", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			// Оба вида строк буквальны; в модели они content, а не heading.
			result := v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
				v052TestOpen(t, tag, "", 2),
				v052TestText(t, "# Literal main", "# Literal main", 3),
				v052TestText(t, "## Literal subtitle", "## Literal subtitle", 3), v052TestClose(t, 3))
			v052TestRun(t, result, "current", nil)
		})
		t.Run(tag+" inside task", func(t *testing.T) {
			result := v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
				v052TestTag(t, "task", "1", "identifier", 2),
				v052TestTag(t, "instr", "Read.", "content", 3),
				v052TestOpen(t, tag, "", 3), v052TestText(t, "## Literal", "## Literal", 5),
				v052TestClose(t, 5))
			v052TestRun(t, result, "current", nil)
		})
	}
	t.Run("literal main is not an anchor", func(t *testing.T) {
		result := v052TestDocument(t, v052TestOpen(t, "editor", "", 0),
			v052TestText(t, "# Literal", "# Literal", 2), v052TestClose(t, 2),
			v052TestText(t, "Text.", "Text.", 0), v052TestHeading(t, 2, "Vocabulary"))
		v052TestRun(t, result, "current", nil)
	})
	t.Run("one-line editor does not hide following candidate", func(t *testing.T) {
		result := v052TestDocument(t, v052TestTag(t, "editor", "Comment.", "content", 0),
			v052TestHeading(t, 2, "Vocabulary"), v052TestHeading(t, 1, "Lesson"))
		v052TestRun(t, result, "current",
			[]v052TestWant{{[3]int{3, 1, 14}, v052TestBefore, [3]int{}}})
	})
}

func TestV052EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
				v052TestText(t, "Read carefully.", "Read carefully.", 2), v052TestHeading(t, 2, "Vocabulary"))
			for i := range result.Lines {
				if ending != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 16}}}
			v052TestRun(t, result, "first", want)
			v052TestRun(t, result, "second", want)
		})
	}
	t.Run("Unicode obstacle uses source columns", func(t *testing.T) {
		result := v052TestDocument(t, v052TestHeading(t, 1, "Lesson"),
			v052TestText(t, " \tЯ😀e\u0301 ", "Я😀e\u0301", 2), v052TestHeading(t, 2, "Vocabulary"))
		v052TestRun(t, result, "current",
			[]v052TestWant{{[3]int{4, 1, 14}, v052TestObstacle, [3]int{3, 1, 8}}})
	})
}

func TestV052UnavailableLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v052TestDocument(t)
			result.Document.DSLVersion = &version
			result.Lines = []model.Line{}
			count := 0
			result.Document.LineCount = &count
			if version == "1.2" {
				v052TestRun(t, result, "empty", nil)
				result.Document.LineCount = nil
				v052TestRun(t, result, "unavailable", nil)
				return
			}
			before := v052TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v052TestRegistry(t))
			if err == nil || checks != nil || v052TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v052TestLine сохраняет заданные элементы и родителя без распознавания Raw.
func v052TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v052TestElement вычисляет длину входного элемента, но не ожидаемый диапазон V052.
func v052TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v052TestHeading строит корректный корневой заголовок заданного уровня.
func v052TestHeading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v052TestLine(t, marker+" "+title, "heading", 0,
		v052TestElement(t, "heading-level", marker, strconv.Itoa(level), 1),
		v052TestElement(t, "title", title, title, level+2))
}

// v052TestText задаёт исходное и семантическое содержимое независимо.
func v052TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v052TestLine(t, raw, "content", parent, v052TestElement(t, "content", raw, value, 1))
}

// v052TestTag строит объявление с явно заданным типом единственного параметра.
func v052TestTag(t *testing.T, tag, parameter string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v052TestElement(t, "tag", raw, strings.ToLower(tag), 1)}
	if parameter != "" {
		elements = append(elements, v052TestElement(t, kind, parameter, parameter, len(raw)+2))
		raw += " " + parameter
	}
	return v052TestLine(t, raw, "tag", parent, elements...)
}

// v052TestOpen строит фигурный блок, принимающий при необходимости идентификатор.
func v052TestOpen(t *testing.T, tag, identifier string, parent int) model.Line {
	t.Helper()
	line := v052TestTag(t, tag, identifier, "identifier", parent)
	line.Elements = append(line.Elements, v052TestElement(t, "block-open", "{", "{", len(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v052TestClose задаёт закрывающую строку с родителем — открытием блока.
func v052TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v052TestLine(t, "}", "block-end", parent, v052TestElement(t, "block-close", "}", "}", 1))
}

// v052TestInvalid задаёт ошибочную строку без восстановленного heading-level.
func v052TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	element := v052TestElement(t, "unparsed", raw, "", 1)
	element.Value = nil
	return v052TestLine(t, raw, "invalid", parent, element)
}

// v052TestDocument добавляет только версию; все остальные строки заданы тестом.
// Глубина следует явным родителям, метаданные — однозначным готовым заголовкам.
// Для unparsed создаются обязательные исходные ошибки, не ожидания V052.
func v052TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v052TestTag(t, "dsl-version", "1.2", "version", 0)}, body...)
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
				t.Fatalf("invalid test parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		level, title := "", ""
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "heading-level" {
				level = *element.Value
			}
			if element.ElementType == "title" {
				title = *element.Value
			}
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

// v052TestRegistry использует полный штатный каталог без подмены регистраций.
func v052TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v052TestRun выполняет штатный конвейер и проверяет только новые V052.
func v052TestRun(t *testing.T, result *model.Result, processingID string, want []v052TestWant) {
	t.Helper()
	before := v052TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v052TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v052TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V052" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V052 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := v052TestLocation(expected.location)
		related := []model.Location{}
		if expected.related != ([3]int{}) {
			related = append(related, v052TestLocation(expected.related))
		}
		if entry.Location == nil || *entry.Location != location || !reflect.DeepEqual(entry.RelatedLocations, related) ||
			entry.Message != v052TestMessage+"\nконкретизация: "+expected.reason ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.ID == "" || entry.Source != processingID {
			t.Fatalf("V052 occurrence %d = %+v; want %+v, related %+v, reason %q", i, entry, location, related, expected.reason)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	// Флаги ошибок и ссылки производны. Остальные данные и история неизменны.
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v052TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
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
	if v052TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v052TestLocation переводит явно заданные координаты в модель диапазона.
func v052TestLocation(position [3]int) model.Location {
	return model.Location{Start: model.Position{Line: position[0], Column: position[1]},
		End: model.Position{Line: position[0], Column: position[2]}}
}

// v052TestSnapshot сохраняет полную модель для проверки побочных изменений.
func v052TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

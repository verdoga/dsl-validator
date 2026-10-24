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

func TestV054Registration(t *testing.T) {
	definitions, err := v054Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V054" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Заголовок первого уровня объявлен повторно." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V054 definitions = %d, want 1", count)
	}
}

func TestV054ReferenceAndExample(t *testing.T) {
	// По DSL 1.30 добавлена только обязательная строка версии.
	// Исходные строки эталона и примера сохраняются и начинаются со строки 2.
	t.Run("reference", func(t *testing.T) {
		result := v054Document(t, "Lesson",
			v054Heading(t, "# Lesson", "Lesson", 1, 1, 3),
			v054Heading(t, "### Reading", "Reading", 3, 1, 5),
			v054Text(t, "Read.", "Read.", 3, 1))
		v054Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v054Base(t, "",
			v054Heading(t, "# Another lesson", "Another lesson", 1, 1, 3),
			v054Text(t, "Write.", "Write.", 4, 1))
		v054Run(t, result, "current", [][4]int{{4, 1, 2, 1}})
	})
}

func TestV054Candidates(t *testing.T) {
	for _, tc := range []struct {
		name, raw, title string
		start, titleAt   int
		want             [4]int
	}{
		{"different title", "# Other", "Other", 1, 3, [4]int{4, 1, 2, 1}},
		{"same title", "# Lesson", "Lesson", 1, 3, [4]int{4, 1, 2, 1}},
		{"different case", "# lesson", "lesson", 1, 3, [4]int{4, 1, 2, 1}},
		{"Unicode title", "# Урок 😀e\u0301", "Урок 😀e\u0301", 1, 3, [4]int{4, 1, 2, 1}},
		{"leading space and tab", " \t# Second", "Second", 3, 5, [4]int{4, 3, 2, 1}},
		{"tab is one column", "\t# Second", "Second", 2, 4, [4]int{4, 2, 2, 1}},
		{"another number sign in title", "# # Title", "# Title", 1, 3, [4]int{4, 1, 2, 1}},
		{"trailing whitespace", "# Second \t", "Second", 1, 3, [4]int{4, 1, 2, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v054Base(t, "", v054Heading(t, tc.raw, tc.title, 1, tc.start, tc.titleAt))
			v054Run(t, result, "current", [][4]int{tc.want})
		})
	}
	for _, raw := range []string{"#Title", "#\tTitle", "#", "# \t"} {
		t.Run("invalid "+strconv.Quote(raw), func(t *testing.T) {
			result := v054Base(t, "Lesson", v054Invalid(t, raw, 2, 1))
			v054Run(t, result, "current", [][4]int{{4, 1, 2, 1}})
		})
	}
	t.Run("unparsed title on heading line", func(t *testing.T) {
		// Разобранный маркер и пустое ошибочное название: нулевой диапазон
		// unparsed разрешён контрактом; ошибка относится к концу строки.
		line := v054Line(t, "#", "heading", 0, 0,
			v054Element(t, "heading-level", "#", "1", 1, 2),
			v054Element(t, "unparsed", "", "", 2, 2))
		v054Run(t, v054Base(t, "Lesson", line), "current", [][4]int{{4, 1, 2, 1}})
	})
	t.Run("first candidate is invalid", func(t *testing.T) {
		result := v054Document(t, "Second", v054Invalid(t, "#Lesson", 0, 0),
			v054Heading(t, "# Second", "Second", 1, 1, 3))
		v054Run(t, result, "current", [][4]int{{3, 1, 2, 1}})
	})
	t.Run("related position includes original indentation", func(t *testing.T) {
		result := v054Document(t, "",
			v054Heading(t, "\t # Lesson", "Lesson", 1, 3, 5),
			v054Text(t, "Read.", "Read.", 2, 1),
			v054Heading(t, "# Second", "Second", 1, 1, 3))
		v054Run(t, result, "current", [][4]int{{4, 1, 2, 3}})
	})
	t.Run("all repeats refer to first", func(t *testing.T) {
		result := v054Base(t, "",
			v054Heading(t, "# Second", "Second", 1, 1, 3),
			v054Heading(t, "# Third", "Third", 1, 1, 3))
		v054Run(t, result, "current", [][4]int{{4, 1, 2, 1}, {5, 1, 2, 1}})
	})
}

func TestV054ExcludedPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
	}{
		{"inside text", "Text # Title", "Text # Title"},
		{"escaped", `\# Title`, "# Title"},
		{"escaped after indentation", " \t\\# Title", "# Title"},
		{"literal backslash", `\\# Title`, `\# Title`},
		{"nonbreaking space", "\u00a0# Title", "\u00a0# Title"},
		{"em space after indentation", " \t\u2003# Title", "\u2003# Title"},
		{"fullwidth marker", "＃ Title", "＃ Title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v054Run(t, v054Base(t, "Lesson", v054Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v054Run(t, v054Base(t, "Lesson", v054Line(t, raw, "blank", 2, 1)), "current", nil)
		})
	}
	for _, level := range []int{2, 3} {
		t.Run("heading level "+strconv.Itoa(level), func(t *testing.T) {
			// Второй уровень размещён до отображаемого материала по DSL 2.1.4.
			result := v054Document(t, "Lesson",
				v054Heading(t, "# Lesson", "Lesson", 1, 1, 3),
				v054Heading(t, strings.Repeat("#", level)+" Section", "Section", level, 1, level+2),
				v054Text(t, "Read.", "Read.", 3, 1))
			if level == 2 {
				subtitle := "Section"
				result.Document.Metadata.Subtitle = &subtitle
			}
			v054Run(t, result, "current", nil)
		})
	}
	for _, raw := range []string{"#### Title", "##Title", "###", "####"} {
		t.Run("invalid longer marker "+raw, func(t *testing.T) {
			v054Run(t, v054Base(t, "Lesson", v054Invalid(t, raw, 2, 1)), "current", nil)
		})
	}
}

func TestV054BoundariesAndContainers(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"page boundaries", []model.Line{
			v054Heading(t, "### Next", "Next", 3, 1, 5),
			v054Text(t, "Read.", "Read.", 4, 1),
			v054Tag(t, "newpage", "", "", 4, 1),
			v054Heading(t, "# Second", "Second", 1, 1, 3),
		}, [][4]int{{7, 1, 2, 1}}},
		{"blank between repeats", []model.Line{
			v054Heading(t, "# Second", "Second", 1, 1, 3),
			v054Line(t, "", "blank", 4, 1),
			v054Heading(t, "# Third", "Third", 1, 1, 3),
		}, [][4]int{{4, 1, 2, 1}, {6, 1, 2, 1}}},
		{"text block", []model.Line{
			v054Open(t, "text", 2, 1), v054Invalid(t, "# Nested", 4, 2), v054Close(t, 4, 2),
		}, [][4]int{{5, 1, 2, 1}}},
		{"deeply nested attempt", []model.Line{
			v054Tag(t, "task", "a", "identifier", 2, 1),
			v054Tag(t, "instr", "Read.", "content", 4, 2),
			v054Open(t, "text", 4, 2), v054Invalid(t, "# Nested", 6, 3), v054Close(t, 6, 3),
			v054Tag(t, "endtask", "", "", 2, 1),
		}, [][4]int{{7, 1, 2, 1}}},
		{"separator and block boundary", []model.Line{
			// DSL 4.7.5–4.7.7: разделитель допустим в matching;
			// кроме ошибочного заголовка заданы верхний элемент и два варианта.
			v054Open(t, "matching", 2, 1),
			v054Invalid(t, "# Nested", 4, 2),
			v054Text(t, "Word", "Word", 4, 2),
			v054Line(t, "---", "separator", 4, 2),
			v054Text(t, "First meaning", "First meaning", 4, 2),
			v054Text(t, "Second meaning", "Second meaning", 4, 2),
			v054Close(t, 4, 2),
			v054Heading(t, "# Second", "Second", 1, 1, 3),
		}, [][4]int{{5, 1, 2, 1}, {11, 1, 2, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			title := ""
			if tc.name == "text block" || tc.name == "deeply nested attempt" {
				title = "Lesson"
			}
			v054Run(t, v054Base(t, title, tc.body...), "current", tc.want)
		})
	}
}

func TestV054LiteralBlocks(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			t.Run("contents only", func(t *testing.T) {
				result := v054Base(t, "Lesson", v054Open(t, tag, 2, 1),
					v054Text(t, "# Literal", "# Literal", 4, 2), v054Close(t, 4, 2))
				v054Run(t, result, "current", nil)
			})
			t.Run("after closing", func(t *testing.T) {
				result := v054Base(t, "", v054Open(t, tag, 2, 1),
					v054Text(t, "# Literal", "# Literal", 4, 2), v054Close(t, 4, 2),
					v054Heading(t, "# Second", "Second", 1, 1, 3))
				v054Run(t, result, "current", [][4]int{{7, 1, 2, 1}})
			})
			t.Run("inside task", func(t *testing.T) {
				result := v054Base(t, "",
					v054Tag(t, "task", "a", "identifier", 2, 1),
					v054Tag(t, "instr", "Read.", "content", 4, 2),
					v054Open(t, tag, 4, 2), v054Text(t, "# Literal", "# Literal", 6, 3), v054Close(t, 6, 3),
					v054Tag(t, "endtask", "", "", 2, 1),
					v054Heading(t, "# Second", "Second", 1, 1, 3))
				v054Run(t, result, "current", [][4]int{{10, 1, 2, 1}})
			})
			t.Run("single line form", func(t *testing.T) {
				result := v054Base(t, "", v054Tag(t, tag, "Write.", "content", 2, 1),
					v054Heading(t, "# Second", "Second", 1, 1, 3))
				v054Run(t, result, "current", [][4]int{{5, 1, 2, 1}})
			})
		})
	}
	t.Run("editor before first heading", func(t *testing.T) {
		result := v054Document(t, "", v054Open(t, "editor", 0, 0),
			v054Text(t, "# Literal", "# Literal", 2, 1), v054Close(t, 2, 1),
			v054Heading(t, "# Lesson", "Lesson", 1, 1, 3), v054Text(t, "Read.", "Read.", 5, 1),
			v054Heading(t, "# Second", "Second", 1, 1, 3))
		v054Run(t, result, "current", [][4]int{{7, 1, 5, 1}})
	})
}

func TestV054EndingsAndIndependentRuns(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v054Base(t, "", v054Heading(t, "# Second", "Second", 1, 1, 3))
			if ending == "\r\n" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			result.Lines[3].LineEnding = ending
			v054Run(t, result, "first", [][4]int{{4, 1, 2, 1}})
			v054Run(t, v054Base(t, "Lesson"), "independent", nil)
			v054Run(t, result, "second", [][4]int{{4, 1, 2, 1}})
		})
	}
}

func TestV054EmptyInputAndVersions(t *testing.T) {
	t.Run("version only", func(t *testing.T) {
		v054Run(t, v054Document(t, ""), "current", nil)
	})
	t.Run("one candidate at last line", func(t *testing.T) {
		result := v054Document(t, "Lesson", v054Heading(t, "# Lesson", "Lesson", 1, 1, 3))
		v054Run(t, result, "current", nil)
	})
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			// Версия известна, но исходные строки недоступны; это не
			// фиктивный пустой DSL-файл с отсутствующей строкой версии.
			result := v054Document(t, "")
			result.Document.DSLVersion = &version
			result.Document.LineCount = nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v054Run(t, result, "current", nil)
				return
			}
			before := v054Snapshot(t, result)
			checks, err := validator.Prepare(result, v054Registry(t))
			if err == nil || checks != nil || !reflect.DeepEqual(before, v054Snapshot(t, result)) {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v054Base добавляет заголовок и обычный текст; первая строка tail получает номер 4.
// title задаёт явно извлечённые метаданные; пустое значение означает неоднозначность.
func v054Base(t *testing.T, title string, tail ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v054Heading(t, "# Lesson", "Lesson", 1, 1, 3), v054Text(t, "Read.", "Read.", 2, 1)}
	return v054Document(t, title, append(body, tail...)...)
}

// v054Document добавляет версию перед готовыми строками, не разбирая Raw.
// Для явно заданных unparsed добавляет согласованные исходные ошибки и ссылки.
func v054Document(t *testing.T, title string, body ...model.Line) *model.Result {
	t.Helper()
	version := "1.2"
	lines := append([]model.Line{v054Tag(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
	if title != "" {
		result.Document.Metadata.Title = &title
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
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора заголовка.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v054Heading задаёт исходную строку, название и позиции элементов явно.
func v054Heading(t *testing.T, raw, title string, level, start, titleAt int) model.Line {
	t.Helper()
	return v054Line(t, raw, "heading", 0, 0,
		v054Element(t, "heading-level", strings.Repeat("#", level), strconv.Itoa(level), start, start+level),
		v054Element(t, "title", title, title, titleAt, titleAt+utf8.RuneCountInString(title)))
}

// v054Invalid сохраняет ошибочную попытку целиком как неразобранный фрагмент.
func v054Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v054Line(t, raw, "invalid", parent, depth, v054Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v054Text задаёт исходный текст и семантическое значение независимо.
func v054Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v054Line(t, raw, "content", parent, depth, v054Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v054Tag создаёт известный тег с явно выбранным типом необязательного аргумента.
func v054Tag(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v054Element(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v054Element(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v054Line(t, raw, "tag", parent, depth, elements...)
}

// v054Open задаёт блочную форму известного тега без параметров.
func v054Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v054Line(t, "@"+tag+" {", "block-start", parent, depth,
		v054Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v054Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v054Close задаёт закрытие блока с явным родителем.
func v054Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v054Line(t, "}", "block-end", parent, depth, v054Element(t, "block-close", "}", "}", 1, 2))
}

// v054Line сохраняет готовую структуру строки; её номер назначает v054Document.
func v054Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v054Element задаёт элемент с исходными Unicode-колонками и исключающим концом.
func v054Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v054Registry создаёт реестр исключительно из штатного каталога.
func v054Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v054Run проверяет новые V054 по явным четвёркам:
// строка/колонка повтора, строка/колонка первого маркера. Оба диапазона длиной 1.
func v054Run(t *testing.T, result *model.Result, source string, want [][4]int) {
	t.Helper()
	before := v054Snapshot(t, result)
	checks, err := validator.Prepare(result, v054Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !reflect.DeepEqual(before, v054Snapshot(t, result)) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V054" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V054 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[1] + 1}}
		related := model.Location{Start: model.Position{Line: position[2], Column: position[3]}, End: model.Position{Line: position[2], Column: position[3] + 1}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Заголовок первого уровня объявлен повторно." {
			t.Fatalf("occurrence %d = %+v; want %+v related %+v and exact metadata", i, entry, location, related)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	// Сравниваем исходник и историю, исключая только новые записи и
	// производные флаги/ссылки, согласованность которых проверил CheckModel.
	restored := v054Snapshot(t, result)
	if len(restored.Lines) != len(before.Lines) || len(restored.Processing) != len(before.Processing)+1 || len(restored.Diagnostics) < len(before.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	restored.Document.HasErrors = before.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(before.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = before.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = before.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(before.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(before.Diagnostics)]
	if !reflect.DeepEqual(before, restored) {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v054Snapshot возвращает независимую копию входной модели для проверки сохранности.
func v054Snapshot(t *testing.T, result *model.Result) model.Result {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot model.Result
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

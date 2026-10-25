package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV131Registration(t *testing.T) {
	registry := v131TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V131" {
			continue
		}
		count++
		if definition.Message != "Для @include не найден @fragment с указанным идентификатором." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V131 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V131 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		// Соседние версии отвергаются штатным каталогом; поддержку не расширяем.
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("ForVersion(%q) = %+v, %v; want rejection", version, definitions, err)
		}
	}
}

func TestV131ReferenceAndExample(t *testing.T) {
	registry := v131TestRegistry(t)
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][3]int
	}{
		{"reference", "@include f\n@fragment f {\nRead this text.\n}",
			append([]model.Line{v131TestInclude(t, "f", 2, 1)}, v131TestDefinition(t, "f", 4, 2, 1)...), nil},
		{"example", "@include missing", []model.Line{v131TestInclude(t, "missing", 2, 1)}, [][3]int{{3, 10, 17}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v131TestLesson(t, tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только обязательные версия и заголовок;
			// содержимое и порядок строк эталона и примера сохранены дословно.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v131TestRun(t, registry, result, "current", tc.want)
		})
	}
}

func TestV131ResolutionAndOccurrences(t *testing.T) {
	registry := v131TestRegistry(t)
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"definition before include", append(v131TestDefinition(t, "f", 3, 2, 1), v131TestInclude(t, "f", 2, 1)), nil},
		{"different definition", append([]model.Line{v131TestInclude(t, "f", 2, 1)}, v131TestDefinition(t, "g", 4, 2, 1)...), [][3]int{{3, 10, 11}}},
		{"task ID is not a fragment", []model.Line{v131TestInclude(t, "f", 2, 1),
			v131TestLine(t, "@task f", "tag", 2, 1, v131TestElement(t, "tag", "@task", "task", 1), v131TestElement(t, "identifier", "f", "f", 7)),
			v131TestText(t, "Read this text.", "Read this text.", 4, 2)}, [][3]int{{3, 10, 11}}},
		{"duplicate definitions still exist", append(append(v131TestDefinition(t, "f", 3, 2, 1),
			v131TestDefinition(t, "F", 6, 2, 1)...), v131TestInclude(t, "f", 2, 1)), nil},
		{"repeated resolved includes", append([]model.Line{v131TestInclude(t, "f", 2, 1), v131TestInclude(t, "F", 2, 1)},
			v131TestDefinition(t, "f", 5, 2, 1)...), nil},
		{"repeated missing ID", []model.Line{v131TestInclude(t, "missing", 2, 1), v131TestInclude(t, "missing", 2, 1)}, [][3]int{{3, 10, 17}, {4, 10, 17}}},
		{"different missing IDs", []model.Line{v131TestInclude(t, "f", 2, 1), v131TestInclude(t, "g", 2, 1)}, [][3]int{{3, 10, 11}, {4, 10, 11}}},
		{"blank does not group occurrences", []model.Line{v131TestInclude(t, "f", 2, 1), v131TestLine(t, "", "blank", 2, 1),
			v131TestInclude(t, "f", 2, 1)}, [][3]int{{3, 10, 11}, {5, 10, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v131TestRun(t, registry, v131TestLesson(t, tc.body...), "current", tc.want) })
	}
	for _, tc := range []struct {
		name, reference, definition string
		want                        [][3]int
	}{
		{"Latin case", "F", "f", nil},
		{"Cyrillic case", "Ф", "ф", nil},
		{"mixed ID", "Good-А_2.1", "good-а_2.1", nil},
		{"supplementary case", "𐐀", "𐐨", nil},
		{"no compatibility normalization", "Ａ", "A", [][3]int{{3, 10, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v131TestInclude(t, tc.reference, 2, 1)}, v131TestDefinition(t, tc.definition, 4, 2, 1)...)
			v131TestRun(t, registry, v131TestLesson(t, body...), "current", tc.want)
		})
	}
}

func TestV131IDAlphabet(t *testing.T) {
	registry := v131TestRegistry(t)
	for _, tc := range []struct {
		id  string
		end int
	}{
		{"12", 12}, {"١", 11}, {".", 11}, {"-", 11}, {"_", 11}, {"𐐀", 11}, {"漢字", 12}, {"Good-А_2.1", 20},
	} {
		t.Run("valid missing "+tc.id, func(t *testing.T) {
			v131TestRun(t, registry, v131TestLesson(t, v131TestInclude(t, tc.id, 2, 1)), "current", [][3]int{{3, 10, tc.end}})
		})
		t.Run("valid resolved "+tc.id, func(t *testing.T) {
			body := append([]model.Line{v131TestInclude(t, tc.id, 2, 1)}, v131TestDefinition(t, tc.id, 4, 2, 1)...)
			v131TestRun(t, registry, v131TestLesson(t, body...), "current", nil)
		})
	}
	for _, id := range []string{"bad/id", "a\u0301", "²", "Ⅳ", "a😀", `a\b`, `\u0066`, "f,", "missing\ttail", "missing\u00a0tail"} {
		t.Run("invalid ID "+id, func(t *testing.T) {
			line := v131TestLine(t, "@include "+id, "tag", 2, 1,
				v131TestElement(t, "tag", "@include", "include", 1), v131TestElement(t, "unparsed", id, "", 10))
			v131TestRun(t, registry, v131TestLesson(t, line), "current", nil)
		})
	}
	t.Run("invalid definition does not define valid prefix", func(t *testing.T) {
		opening := v131TestLine(t, "@fragment f/ {", "block-start", 2, 1,
			v131TestElement(t, "tag", "@fragment", "fragment", 1), v131TestElement(t, "unparsed", "f/", "", 11),
			v131TestElement(t, "block-open", "{", "{", 14))
		v131TestRun(t, registry, v131TestLesson(t, v131TestInclude(t, "f", 2, 1), opening,
			v131TestText(t, "Read.", "Read.", 4, 2), v131TestClose(t, 4, 2)), "current", [][3]int{{3, 10, 11}})
	})
	for _, raw := range []string{"@include", "@include   ", "@include \t "} {
		t.Run("missing ID "+raw, func(t *testing.T) {
			result := v131TestLesson(t, v131TestLine(t, raw, "tag", 2, 1, v131TestElement(t, "tag", "@include", "include", 1)))
			// Отсутствующий обязательный аргумент уже отмечен парсером;
			// это согласованная ошибка DSL, которую V131 не дублирует.
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: "parser-missing-id", Source: "parser", DiagnosticCode: "fixture-missing-id", SeverityLevel: "error", DiagnosticScope: "line",
				Message: "Отсутствует обязательный ID.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 9}},
			})
			result.Document.HasErrors, result.Lines[2].HasErrors = true, true
			v131TestRun(t, registry, result, "current", nil)
		})
	}
}

func TestV131FirstArgumentAndRawRecovery(t *testing.T) {
	registry := v131TestRegistry(t)
	for _, separateID := range []bool{false, true} {
		for _, tc := range []struct {
			name, id string
			defined  bool
			want     [][3]int
		}{
			{"missing with tail", "missing", false, [][3]int{{3, 10, 17}}},
			{"resolved with tail", "f", true, nil},
		} {
			t.Run(tc.name+" identifier="+strconv.FormatBool(separateID), func(t *testing.T) {
				elements := []model.Element{v131TestElement(t, "tag", "@include", "include", 1)}
				if separateID {
					elements = append(elements, v131TestElement(t, "identifier", tc.id, tc.id, 10),
						v131TestElement(t, "unparsed", "extra", "", 11+len(tc.id)))
				} else {
					elements = append(elements, v131TestElement(t, "unparsed", tc.id+" extra", "", 10))
				}
				body := []model.Line{v131TestLine(t, "@include "+tc.id+" extra", "invalid", 2, 1, elements...)}
				if tc.defined {
					body = append(body, v131TestDefinition(t, tc.id, 4, 2, 1)...)
				}
				v131TestRun(t, registry, v131TestLesson(t, body...), "current", tc.want)
			})
		}
	}
	for _, tc := range []struct {
		name, raw                string
		tagStart, idStart, idEnd int
	}{
		{"uppercase tag", "@INCLUDE missing", 1, 10, 17},
		{"indent and two spaces", "  @include  missing", 3, 13, 20},
		{"trailing spaces", "@include missing  ", 1, 10, 17},
		{"trailing tab", "@include missing\t", 1, 10, 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tagRaw := "@include"
			if tc.name == "uppercase tag" {
				tagRaw = "@INCLUDE"
			}
			line := v131TestLine(t, tc.raw, "tag", 2, 1,
				v131TestElement(t, "tag", tagRaw, "include", tc.tagStart), v131TestElement(t, "identifier", "missing", "missing", tc.idStart))
			v131TestRun(t, registry, v131TestLesson(t, line), "current", [][3]int{{3, tc.idStart, tc.idEnd}})
		})
	}
	t.Run("invalid separator is not ordinary space", func(t *testing.T) {
		line := v131TestLine(t, "@include\tmissing", "invalid", 2, 1,
			v131TestElement(t, "tag", "@include", "include", 1), v131TestElement(t, "unparsed", "\tmissing", "", 9))
		v131TestRun(t, registry, v131TestLesson(t, line), "current", nil)
	})
	t.Run("canonical identifier value retains original columns", func(t *testing.T) {
		line := v131TestLine(t, "@include F", "tag", 2, 1,
			v131TestElement(t, "tag", "@include", "include", 1), v131TestElement(t, "identifier", "F", "f", 10))
		v131TestRun(t, registry, v131TestLesson(t, line), "current", [][3]int{{3, 10, 11}})
	})
}

func TestV131WholeDocumentScope(t *testing.T) {
	registry := v131TestRegistry(t)
	variant := func(name string) model.Line {
		t.Helper()
		return v131TestLine(t, "@variant "+name, "tag", 3, 2,
			v131TestElement(t, "tag", "@variant", "variant", 1), v131TestElement(t, "name", name, name, 10))
	}
	t.Run("definition in later branch", func(t *testing.T) {
		body := []model.Line{v131TestOpen(t, "variants", 2, 1), variant("A"), v131TestInclude(t, "f", 4, 3), variant("B")}
		body = append(body, v131TestDefinition(t, "f", 7, 6, 3)...)
		// Определение само не отображается; ветвь B получает собственный материал.
		body = append(body, v131TestText(t, "Other text.", "Other text.", 6, 3), v131TestClose(t, 3, 2))
		v131TestRun(t, registry, v131TestLesson(t, body...), "current", nil)
	})
	t.Run("missing references in every branch", func(t *testing.T) {
		v131TestRun(t, registry, v131TestLesson(t, v131TestOpen(t, "variants", 2, 1), variant("A"),
			v131TestInclude(t, "missing", 4, 3), variant("B"), v131TestInclude(t, "missing", 6, 3), v131TestClose(t, 3, 2)),
			"current", [][3]int{{5, 10, 17}, {7, 10, 17}})
	})
	t.Run("definition in another step", func(t *testing.T) {
		first := v131TestLine(t, "### First", "heading", 0, 0,
			v131TestElement(t, "heading-level", "###", "3", 1), v131TestElement(t, "title", "First", "First", 5))
		second := v131TestLine(t, "### Second", "heading", 0, 0,
			v131TestElement(t, "heading-level", "###", "3", 1), v131TestElement(t, "title", "Second", "Second", 5))
		body := []model.Line{first, v131TestInclude(t, "f", 3, 1), second}
		body = append(body, v131TestDefinition(t, "f", 6, 5, 1)...)
		body = append(body, v131TestText(t, "Other text.", "Other text.", 5, 1))
		v131TestRun(t, registry, v131TestLesson(t, body...), "current", nil)
	})
}

func TestV131LiteralContent(t *testing.T) {
	registry := v131TestRegistry(t)
	for _, container := range []string{"editor", "multifill"} {
		t.Run("pseudo include in "+container, func(t *testing.T) {
			body := []model.Line{v131TestOpen(t, container, 2, 1), v131TestText(t, "@include missing", "@include missing", 3, 2)}
			if container == "multifill" {
				gap := v131TestElement(t, "placeholder", "_____", "", 1)
				gap.Value = nil
				body = append(body, v131TestLine(t, "_____", "content", 3, 2, gap))
			}
			body = append(body, v131TestClose(t, 3, 2), v131TestText(t, "Read.", "Read.", 2, 1))
			v131TestRun(t, registry, v131TestLesson(t, body...), "current", nil)
		})
		t.Run("pseudo definition in "+container, func(t *testing.T) {
			v131TestRun(t, registry, v131TestLesson(t, v131TestInclude(t, "f", 2, 1), v131TestOpen(t, container, 2, 1),
				v131TestText(t, `@fragment f \{`, "@fragment f {", 4, 2), v131TestClose(t, 4, 2)), "current", [][3]int{{3, 10, 11}})
		})
	}
	t.Run("escaped pseudo include", func(t *testing.T) {
		v131TestRun(t, registry, v131TestLesson(t, v131TestText(t, `\@include missing`, "@include missing", 2, 1)), "current", nil)
	})
	t.Run("escaped pseudo definition", func(t *testing.T) {
		v131TestRun(t, registry, v131TestLesson(t, v131TestInclude(t, "f", 2, 1),
			v131TestText(t, `\@fragment f \{`, "@fragment f {", 2, 1)), "current", [][3]int{{3, 10, 11}})
	})
	t.Run("inline editor content", func(t *testing.T) {
		line := v131TestLine(t, "@editor @include missing", "tag", 2, 1,
			v131TestElement(t, "tag", "@editor", "editor", 1), v131TestElement(t, "content", "@include missing", "@include missing", 9))
		v131TestRun(t, registry, v131TestLesson(t, line, v131TestText(t, "Read.", "Read.", 2, 1)), "current", nil)
	})
	t.Run("fully unparsed line", func(t *testing.T) {
		line := v131TestLine(t, "@include{missing", "invalid", 2, 1, v131TestElement(t, "unparsed", "@include{missing", "", 1))
		v131TestRun(t, registry, v131TestLesson(t, line), "current", nil)
	})
}

func TestV131EndingsHistoryAndState(t *testing.T) {
	registry := v131TestRegistry(t)
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("last line ending "+string(eol), func(t *testing.T) {
			result := v131TestLesson(t, v131TestInclude(t, "missing", 2, 1))
			if eol == "\r\n" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			result.Lines[2].LineEnding = eol
			v131TestRun(t, registry, result, "first", [][3]int{{3, 10, 17}})
			v131TestRun(t, registry, result, "second", [][3]int{{3, 10, 17}})
		})
	}
	t.Run("definitions do not leak between documents", func(t *testing.T) {
		body := append([]model.Line{v131TestInclude(t, "f", 2, 1)}, v131TestDefinition(t, "f", 4, 2, 1)...)
		v131TestRun(t, registry, v131TestLesson(t, body...), "defined", nil)
		v131TestRun(t, registry, v131TestLesson(t, v131TestInclude(t, "f", 2, 1)), "missing", [][3]int{{3, 10, 11}})
		v131TestRun(t, registry, v131TestLesson(t, body...), "defined-again", nil)
	})
	t.Run("empty model", func(t *testing.T) {
		result := v131TestLesson(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		// Допустимое отсутствие строк при известной версии в модели,
		// а не утверждение о корректности пустого DSL-файла.
		v131TestRun(t, registry, result, "empty", nil)
	})
}

// v131TestLesson добавляет обязательный контекст; body начинается со строки 3.
// Для явно заданных unparsed создаётся согласованная история ошибок парсера.
func v131TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v131TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v131TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v131TestElement(t, "version", "1.2", "1.2", 14)),
		v131TestLine(t, "# Lesson", "heading", 0, 0,
			v131TestElement(t, "heading-level", "#", "1", 1), v131TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
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
				ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Не разобран служебный аргумент или синтаксис объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v131TestInclude задаёт распознанный ID обращения без дополнительного хвоста.
func v131TestInclude(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	return v131TestLine(t, "@include "+id, "tag", parent, depth,
		v131TestElement(t, "tag", "@include", "include", 1), v131TestElement(t, "identifier", id, id, 10))
}

// v131TestDefinition создаёт закрытое определение с непустым содержимым.
// number явно задаёт номер объявления для родительских связей, не ожиданий.
func v131TestDefinition(t *testing.T, id string, number, parent, depth int) []model.Line {
	t.Helper()
	return []model.Line{
		v131TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
			v131TestElement(t, "tag", "@fragment", "fragment", 1), v131TestElement(t, "identifier", id, id, 11),
			v131TestElement(t, "block-open", "{", "{", 12+utf8.RuneCountInString(id))),
		v131TestText(t, "Read this text.", "Read this text.", number, depth+1), v131TestClose(t, number, depth+1),
	}
}

// v131TestText сохраняет явно заданные исходный текст и семантическое значение.
func v131TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v131TestLine(t, raw, "content", parent, depth, v131TestElement(t, "content", raw, value, 1))
}

// v131TestOpen задаёт объявление блока без параметров с указанным родителем.
func v131TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v131TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v131TestElement(t, "tag", "@"+name, name, 1), v131TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v131TestClose задаёт закрытие указанного блока.
func v131TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v131TestLine(t, "}", "block-end", parent, depth, v131TestElement(t, "block-close", "}", "}", 1))
}

// v131TestLine сохраняет явно заданную структуру; Raw не классифицируется.
func v131TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v131TestElement задаёт входной элемент; его конец исключён и измерен в рунах.
// Вычисление длины входа не используется для ожидаемых диапазонов V131.
func v131TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v131TestRegistry использует исключительно полный штатный каталог регистраций.
func v131TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v131TestRun проверяет V131 текущего запуска полного конвейера и сохранность входа.
// want содержит явные тройки: строка, начальная и исключённая конечная колонка.
func v131TestRun(t *testing.T, registry *diagnostics.Registry, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v131TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v131TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V131" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V131 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location {
			t.Fatalf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Для @include не найден @fragment с указанным идентификатором." ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d metadata = %+v", i, entry)
		}
		if !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
			t.Fatalf("missing error flags for occurrence %d", i)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line-scoped V131 linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v131TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// В копии восстанавливаем только производные флаги и ссылки и убираем
	// новые записи. Все остальные данные обязаны совпасть с исходным снимком.
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
	if v131TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v131TestSnapshot получает независимое представление модели для сравнения.
func v131TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

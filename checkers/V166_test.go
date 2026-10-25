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

func TestV166Registration(t *testing.T) {
	registry := v166Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V166" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "element" || definition.Check == nil ||
			definition.Message != "Тег @answer или @hint размещён после разделителя ---." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V166 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func TestV166ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][3]int
	}{
		{"reference", "@matching {\nСолнце — звезда?\n@answer Да\n@hint Вспомните определение звезды.\n---\nДа\nНет\n}", []model.Line{
			v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Inline(t, "answer", "Да", 3, 2),
			v166Inline(t, "hint", "Вспомните определение звезды.", 3, 2), v166Separator(t, "---", 3, 2),
			v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2)}, nil},
		{"example", "@matching {\nСолнце — звезда?\n---\nДа\nНет\n@answer Да\n}", []model.Line{
			v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
			v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2), v166Inline(t, "answer", "Да", 3, 2)}, [][3]int{{8, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v166Block(t, "matching", tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v166Run(t, result, tc.want, nil)
			v166Run(t, result, tc.want, nil)
		})
	}
}

func TestV166DirectTags(t *testing.T) {
	for _, tc := range []struct {
		name string
		tail []model.Line
		want [][3]int
	}{
		{"answer", []model.Line{v166Inline(t, "answer", "Да", 3, 2)}, [][3]int{{8, 1, 8}}},
		{"hint", []model.Line{v166Inline(t, "hint", "Подсказка", 3, 2)}, [][3]int{{8, 1, 6}}},
		{"each declaration once", []model.Line{v166Inline(t, "answer", "Да", 3, 2), v166Inline(t, "hint", "Подсказка", 3, 2),
			v166Inline(t, "answer", "Нет", 3, 2)}, [][3]int{{8, 1, 8}, {9, 1, 6}, {10, 1, 8}}},
		{"Unicode content", []model.Line{v166Inline(t, "answer", "Да 🌍", 3, 2)}, [][3]int{{8, 1, 8}}},
		{"empty content", []model.Line{v166Inline(t, "answer", "", 3, 2), v166Inline(t, "hint", "", 3, 2)}, [][3]int{{8, 1, 8}, {9, 1, 6}}},
		{"blocks counted once", []model.Line{v166Open(t, "answer", 3, 2), v166Text(t, "Да", "Да", 8, 3), v166Text(t, "Нет", "Нет", 8, 3), v166Close(t, 8, 3),
			v166Open(t, "hint", 3, 2), v166Text(t, "Подсказка", "Подсказка", 12, 3), v166Close(t, 12, 3)}, [][3]int{{8, 1, 8}, {12, 1, 6}}},
		{"nested tag is not direct", []model.Line{v166Open(t, "hint", 3, 2), v166Inline(t, "answer", "Да", 8, 3), v166Close(t, 8, 3)}, [][3]int{{8, 1, 6}}},
		{"other tag", []model.Line{v166Inline(t, "instr", "Read.", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
				v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2)}
			v166Run(t, v166Block(t, "matching", append(body, tc.tail...)...), tc.want, nil)
		})
	}
	t.Run("immediately after separator", func(t *testing.T) {
		v166Run(t, v166Block(t, "matching", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
			v166Inline(t, "answer", "Да", 3, 2), v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2)), [][3]int{{6, 1, 8}}, nil)
	})
	t.Run("blank after separator", func(t *testing.T) {
		v166Run(t, v166Block(t, "matching", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
			v166Line(t, " \t", "blank", 3, 2), v166Inline(t, "answer", "Да", 3, 2), v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2)),
			[][3]int{{7, 1, 8}}, nil)
	})
	t.Run("case indentation and exact element", func(t *testing.T) {
		answer := v166Line(t, " \t@AnSwEr Да", "tag", 3, 2, v166Element(t, "tag", "@AnSwEr", "answer", 3), v166Element(t, "content", "Да", "Да", 11))
		v166Run(t, v166Block(t, "MaTcHiNg", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
			v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2), answer, v166Inline(t, "HINT", "Помощь", 3, 2)),
			[][3]int{{8, 3, 10}, {9, 1, 6}}, nil)
	})
}

func TestV166SeparatorBoundaries(t *testing.T) {
	text := func(value string) model.Line { t.Helper(); return v166Text(t, value, value, 3, 2) }
	answer := func() model.Line { t.Helper(); return v166Inline(t, "answer", "Да", 3, 2) }
	separator := func() model.Line { t.Helper(); return v166Separator(t, "---", 3, 2) }
	for _, tc := range []struct {
		name          string
		body          []model.Line
		want, want147 [][3]int
	}{
		{"no separator", []model.Line{text("Солнце — звезда?"), answer(), text("Да"), text("Нет")}, nil, [][3]int{{5, 1, 8}}},
		{"two separators", []model.Line{text("Солнце — звезда?"), separator(), text("Да"), separator(), text("Нет"), answer()}, nil, [][3]int{{9, 1, 8}}},
		{"later second separator invalidates lower part", []model.Line{text("Солнце — звезда?"), separator(), text("Да"), answer(), text("Нет"), separator()}, nil, [][3]int{{7, 1, 8}}},
		{"ASCII edge spaces", []model.Line{text("Солнце — звезда?"), v166Separator(t, " \t---\t ", 3, 2), text("Да"), text("Нет"), answer()}, [][3]int{{8, 1, 8}}, nil},
		{"NBSP is not trimmed", []model.Line{text("Солнце — звезда?"), text("\u00a0---\u00a0"), text("Да"), text("Нет"), answer()}, nil, [][3]int{{8, 1, 8}}},
		{"internal spaces", []model.Line{text("Солнце — звезда?"), text("- - -"), text("Да"), text("Нет"), answer()}, nil, [][3]int{{8, 1, 8}}},
		{"four hyphens", []model.Line{text("Солнце — звезда?"), text("----"), text("Да"), text("Нет"), answer()}, nil, [][3]int{{8, 1, 8}}},
		{"escaped delimiter before real separator", []model.Line{text("Солнце — звезда?"), v166Text(t, `\---`, "---", 3, 2), answer(), separator(), text("Да"), text("Нет")}, nil, nil},
		{"escaped delimiter below real separator", []model.Line{text("Солнце — звезда?"), separator(), v166Text(t, `\---`, "---", 3, 2), text("Да"), answer()}, [][3]int{{8, 1, 8}}, nil},
		{"only escaped delimiter", []model.Line{text("Солнце — звезда?"), v166Text(t, `\---`, "---", 3, 2), text("Да"), text("Нет"), answer()}, nil, [][3]int{{8, 1, 8}}},
		{"only literal backslash before delimiter", []model.Line{text("Солнце — звезда?"), v166Text(t, `\\---`, `\---`, 3, 2), text("Да"), text("Нет"), answer()}, nil, [][3]int{{8, 1, 8}}},
		{"orphan upper answer remains V147", []model.Line{answer(), text("Солнце — звезда?"), separator(), text("Да"), text("Нет")}, nil, [][3]int{{4, 1, 8}}},
		{"repeated upper answer remains V147", []model.Line{text("Солнце — звезда?"), answer(), answer(), separator(), text("Да"), text("Нет")}, nil, [][3]int{{6, 1, 8}}},
		{"no tags", []model.Line{text("Солнце — звезда?"), separator(), text("Да"), text("Нет")}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v166Run(t, v166Block(t, "matching", tc.body...), tc.want, tc.want147) })
	}
	t.Run("Raw separator preserved in invalid line", func(t *testing.T) {
		unparsed := v166Element(t, "unparsed", "---", "", 1)
		unparsed.Value = nil
		line := v166Line(t, "---", "invalid", 3, 2, unparsed)
		v166Run(t, v166Block(t, "matching", text("Солнце — звезда?"), line, text("Да"), text("Нет"), answer()), [][3]int{{8, 1, 8}}, nil)
	})
}

func TestV166NestedAndSeparateContexts(t *testing.T) {
	t.Run("upper block forms and nested delimiter", func(t *testing.T) {
		v166Run(t, v166Block(t, "matching", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
			v166Open(t, "answer", 3, 2), v166Text(t, "Да", "Да", 5, 3), v166Close(t, 5, 3),
			v166Open(t, "hint", 3, 2), v166Text(t, "---", "---", 8, 3), v166Close(t, 8, 3),
			v166Separator(t, "---", 3, 2), v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2)), nil, nil)
	})
	t.Run("nested delimiter does not make two", func(t *testing.T) {
		v166Run(t, v166Block(t, "matching", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Inline(t, "answer", "Да", 3, 2),
			v166Open(t, "hint", 3, 2), v166Text(t, "---", "---", 6, 3), v166Close(t, 6, 3), v166Separator(t, "---", 3, 2),
			v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2), v166Inline(t, "answer", "Да", 3, 2)), [][3]int{{12, 1, 8}}, nil)
	})
	t.Run("escaped tag lines are options", func(t *testing.T) {
		v166Run(t, v166Block(t, "matching", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
			v166Text(t, `\@answer Да`, "@answer Да", 3, 2), v166Text(t, `\@hint Подсказка`, "@hint Подсказка", 3, 2)), nil, nil)
	})
	t.Run("ordering is unchanged", func(t *testing.T) {
		// 4.8.11 требует минимум два элемента верхнего списка.
		v166Run(t, v166Block(t, "ordering", v166Text(t, "Item", "Item", 3, 2), v166Text(t, "Other", "Other", 3, 2), v166Separator(t, "---", 3, 2),
			v166Text(t, "First", "First", 3, 2), v166Text(t, "Second", "Second", 3, 2), v166Inline(t, "answer", "First", 3, 2)), nil, [][3]int{{9, 1, 8}})
	})
	t.Run("answer outside closed matching", func(t *testing.T) {
		// По 4.7.17 исходный matching допустим без автоматической проверки.
		v166Run(t, v166Document(t, v166Open(t, "matching", 2, 1), v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
			v166Separator(t, "---", 3, 2), v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2),
			v166Close(t, 3, 2), v166Inline(t, "answer", "Да", 2, 1)), nil, [][3]int{{9, 1, 8}})
	})
	t.Run("two blocks", func(t *testing.T) {
		v166Run(t, v166Document(t, v166Open(t, "matching", 2, 1), v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
			v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2), v166Inline(t, "answer", "Да", 3, 2), v166Close(t, 3, 2),
			v166Open(t, "matching", 2, 1), v166Text(t, "Item", "Item", 10, 2), v166Separator(t, "---", 10, 2), v166Text(t, "Да", "Да", 10, 2),
			v166Text(t, "Нет", "Нет", 10, 2), v166Inline(t, "answer", "Да", 10, 2), v166Close(t, 10, 2)), [][3]int{{8, 1, 8}, {15, 1, 8}}, nil)
	})
	t.Run("branch at depth", func(t *testing.T) {
		first := v166Line(t, "@variant A", "tag", 3, 2, v166Element(t, "tag", "@variant", "variant", 1), v166Element(t, "name", "A", "A", 10))
		second := v166Line(t, "@variant B", "tag", 3, 2, v166Element(t, "tag", "@variant", "variant", 1), v166Element(t, "name", "B", "B", 10))
		v166Run(t, v166Document(t, v166Open(t, "variants", 2, 1), first, v166Open(t, "matching", 4, 3),
			v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 5, 4), v166Separator(t, "---", 5, 4), v166Text(t, "Да", "Да", 5, 4), v166Text(t, "Нет", "Нет", 5, 4),
			v166Inline(t, "answer", "Да", 5, 4), v166Close(t, 5, 4), second, v166Text(t, "Read.", "Read.", 12, 3), v166Close(t, 3, 2)), [][3]int{{10, 1, 8}}, nil)
	})
}

func TestV166EmptyAndLineEndings(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := v166Document(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		v166Run(t, result, nil, nil)
	})
	for _, eol := range []string{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(eol)+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v166Block(t, "matching", v166Text(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v166Separator(t, "---", 3, 2),
					v166Text(t, "Да", "Да", 3, 2), v166Text(t, "Нет", "Нет", 3, 2), v166Inline(t, "answer", "Да", 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = model.LineEnding(eol)
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v166Run(t, result, [][3]int{{8, 1, 8}}, nil)
			})
		}
	}
}

// v166Element сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v166Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v166Line задаёт структуру строки без повторного разбора DSL.
func v166Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v166Open задаёт блочное объявление без параметров с явным родителем.
func v166Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v166Line(t, "@"+name+" {", "block-start", parent, depth,
		v166Element(t, "tag", "@"+name, strings.ToLower(name), 1), v166Element(t, "block-open", "{", "{", len(name)+3))
}

// v166Close задаёт закрытие указанного блока.
func v166Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v166Line(t, "}", "block-end", parent, depth, v166Element(t, "block-close", "}", "}", 1))
}

// v166Inline задаёт известный тег с обычным содержимым без экранирования.
func v166Inline(t *testing.T, name, content string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v166Element(t, "tag", raw, strings.ToLower(name), 1)}
	if content != "" {
		elements = append(elements, v166Element(t, "content", content, content, len(name)+3))
		raw += " " + content
	}
	return v166Line(t, raw, "tag", parent, depth, elements...)
}

// v166Text сохраняет исходное и семантическое содержимое независимо.
func v166Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v166Line(t, raw, "content", parent, depth, v166Element(t, "content", raw, value, 1))
}

// v166Separator задаёт собственную служебную строку без элементов.
func v166Separator(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v166Line(t, raw, "separator", parent, depth)
}

// v166Block окружает заданное тело блоком на строке 3 без смены родителей.
func v166Block(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v166Open(t, name, 2, 1)}, body...)
	return v166Document(t, append(lines, v166Close(t, 3, 2))...)
}

// v166Document добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v166Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v166Line(t, "@dsl-version 1.2", "tag", 0, 0, v166Element(t, "tag", "@dsl-version", "dsl-version", 1), v166Element(t, "version", "1.2", "1.2", 14)),
		v166Line(t, "# Lesson", "heading", 0, 0, v166Element(t, "heading-level", "#", "1", 1), v166Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Ошибка исходной конструкции.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}})
		}
	}
	return result
}

// v166Registry использует только полный штатный каталог.
func v166Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v166Snapshot сохраняет независимое представление модели для сравнения.
func v166Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v166Run проверяет точные результаты V166 и границы исключения V147 через
// штатный конвейер, включая обратные ссылки и сохранность исходных данных.
func v166Run(t *testing.T, result *model.Result, want, want147 [][3]int) {
	t.Helper()
	before := v166Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v166Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v166Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v166-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, expected := range []struct {
		code, scope, message string
		ranges               [][3]int
	}{
		{"V166", "element", "Тег @answer или @hint размещён после разделителя ---.", want},
		{"V147", "line", "Для @answer нет допустимого вопроса или элемента, к которому он относится.", want147},
	} {
		var got []model.Diagnostic
		for _, entry := range result.Diagnostics {
			if string(entry.DiagnosticCode) == expected.code && entry.Source == source {
				got = append(got, entry)
			}
		}
		if len(got) != len(expected.ranges) {
			t.Fatalf("%s count = %d, want %d: %+v", expected.code, len(got), len(expected.ranges), got)
		}
		for i, position := range expected.ranges {
			entry := got[i]
			location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
			if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
				entry.ID == "" || entry.SeverityLevel != "error" || string(entry.DiagnosticScope) != expected.scope || entry.Fatal || entry.Message != expected.message {
				t.Fatalf("%s occurrence %d = %+v, location %+v; want %+v and exact metadata", expected.code, i, entry, entry.Location, location)
			}
			links := 0
			for _, line := range result.Lines {
				for _, element := range line.Elements {
					for _, id := range element.ErrorIDs {
						if id != entry.ID {
							continue
						}
						links++
						if line.Number != position[0] || element.Start != position[1] || element.End != position[2] || element.ElementType != "tag" {
							t.Fatal("diagnostic linked to wrong element")
						}
					}
				}
			}
			wantedLinks := 0
			if expected.code == "V166" {
				wantedLinks = 1
			}
			if links != wantedLinks || !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
				t.Fatal("incorrect diagnostic links or error flags")
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v166Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v166Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

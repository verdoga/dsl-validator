package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v140Expected содержит независимые основной и связанный диапазоны.
// Колонки начинаются с 1, правые границы исключаются.
type v140Expected struct {
	line, start, end                   int
	parentLine, parentStart, parentEnd int
}

func TestV140Registration(t *testing.T) {
	definitions, err := v140Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V140" {
			continue
		}
		count++
		if definition.Message != "Внутри @answer записан вложенный тег." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V140 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v140Unavailable(t, version)
			before := v140Snapshot(t, result)
			checks, err := validator.Prepare(result, v140Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v140Snapshot(t, result) != before {
				t.Fatal("unsupported input changed")
			}
		})
	}
}

func TestV140ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// К дословному эталону добавлены только обязательные @dsl-version
		// и заголовок по DSL 1.1–1.3, 1.30. Hint имеет того же родителя, что answer.
		result := v140Document(t, v140Question(t, 2, 1), v140Open(t, "answer", 2, 1),
			v140Text(t, "London", 4, 2), v140Close(t, 4, 2),
			v140TagLine(t, "hint", "content", "Think of a capital.", 2, 1))
		v140Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v140Run(t, v140Example(t), "current", []v140Expected{{6, 1, 6, 4, 1, 8}})
	})
}

func TestV140KnownSingleLineNames(t *testing.T) {
	// Формы и параметры взяты из разделов соответствующих тегов DSL 1.2.
	// Все они намеренно находятся в запрещённом контейнере answer; остальные
	// проверки расположения не определяют результат V140.
	for _, tc := range []struct {
		name  string
		kind  model.ElementType
		value string
		end   int
	}{
		{"dsl-version", "version", "1.2", 13}, {"document-id", "identifier", "lesson-a", 13},
		{"section", "name", "Practice", 9}, {"order", "number", "2", 7}, {"resource-dir", "resource-path", "Audio", 14},
		{"header", "title", "Practice", 8}, {"task", "identifier", "practice", 6}, {"endtask", "", "", 9},
		{"step", "title", "First", 6}, {"speaking", "", "", 10}, {"newpage", "", "", 9},
		{"editor", "content", "Check.", 8}, {"example", "content", "Hello.", 9},
		{"wordlist", "content", "one; two.", 10}, {"instr", "content", "Read.", 7},
		{"note", "content", "Read.", 6}, {"alt", "content", "A picture.", 5}, {"hint", "content", "Help.", 6},
		{"include", "identifier", "f", 9}, {"answer", "content", "Yes", 8}, {"question", "content", "Ready?", 10},
		{"multifill", "content", "Write.", 11}, {"variant", "name", "First", 9},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.name+" recovered="+fmt.Sprint(recovered), func(t *testing.T) {
				line := v140TagLine(t, tc.name, tc.kind, tc.value, 4, 2)
				if recovered {
					line = v140Invalid(t, line.Raw, 4, 2)
				}
				body := []model.Line{line}
				if tc.name == "task" || tc.name == "step" || tc.name == "variant" {
					body = append(body, v140Text(t, "Read.", 6, 3))
				}
				body = append(body, v140Close(t, 4, 2))
				if tc.name == "include" {
					body = append(body, v140Fragment(t, "f", 2, 1), v140Text(t, "Read.", 8, 2), v140Close(t, 8, 2))
				}
				v140Run(t, v140Within(t, body...), "current", []v140Expected{{6, 1, tc.end, 4, 1, 8}})
			})
		}
	}
	for _, recovered := range []bool{false, true} {
		t.Run("media recovered="+fmt.Sprint(recovered), func(t *testing.T) {
			line := v140Line(t, "@media image picture.png", "tag", 4, 2,
				v140Element(t, "tag", "@media", "media", 1, 7), v140Element(t, "media-type", "image", "image", 8, 13),
				v140Element(t, "source", "picture.png", "picture.png", 14, 25))
			if recovered {
				line = v140Invalid(t, line.Raw, 4, 2)
			}
			v140Run(t, v140Within(t, line, v140Close(t, 4, 2)), "current", []v140Expected{{6, 1, 7, 4, 1, 8}})
		})
	}
}

func TestV140KnownBlockNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  int
	}{
		{"table", 7}, {"script", 8}, {"text", 6}, {"key", 5}, {"choice", 8},
		{"multichoice", 13}, {"matching", 10}, {"ordering", 10}, {"fragment", 10}, {"variants", 10},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.name+" recovered="+fmt.Sprint(recovered), func(t *testing.T) {
				opening := v140Open(t, tc.name, 4, 2)
				if tc.name == "fragment" {
					opening = v140Fragment(t, "f", 4, 2)
				}
				if recovered {
					// Парсер сохранил блок, скобку и родительство, но имя осталось unparsed.
					opening.Elements[0] = v140Unparsed(t, opening.Elements[0].Raw, 1, tc.end)
				}
				body := []model.Line{opening}
				switch tc.name {
				case "variants":
					body = append(body, v140TagLine(t, "variant", "name", "First", 6, 3), v140Text(t, "One", 7, 4),
						v140TagLine(t, "variant", "name", "Second", 6, 3), v140Text(t, "Two", 9, 4))
				case "matching":
					body = append(body, v140Text(t, "One", 6, 3), v140Line(t, "---", "separator", 6, 3),
						v140Text(t, "Two", 6, 3), v140Text(t, "Three", 6, 3))
				default:
					body = append(body, v140Text(t, "One", 6, 3), v140Text(t, "Two", 6, 3))
				}
				body = append(body, v140Close(t, 6, 3), v140Close(t, 4, 2))
				v140Run(t, v140Within(t, body...), "current", []v140Expected{{6, 1, tc.end, 4, 1, 8}})
			})
		}
	}
}

func TestV140RecoveryBoundaries(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []v140Expected
	}{
		{"@hint", []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{"@hint Text", []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{"@hint ", []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{"@hint \tText", []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{" \t@HiNt Я😀", []v140Expected{{6, 3, 8, 4, 1, 8}}},
		{"@hint{", nil}, {"@hint\tText", nil}, {"@hint\t", nil}, {"@hint\u00a0Text", nil},
		{"@hint\u2003Text", nil}, {"@hint}", nil}, {"@hint!", nil}, {"@hintText", nil},
		{"@unknown", nil}, {"@", nil}, {"@{", nil},
		{"\u00a0@hint", nil}, {"Read @hint", nil}, {`\@hint`, nil}, {`\\@hint`, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v140Run(t, v140Within(t, v140Invalid(t, tc.raw, 4, 2), v140Close(t, 4, 2)), "current", tc.want)
		})
	}
	for _, tail := range []string{"\tText", "\u00a0Text", "\u2003Text", "!"} {
		t.Run("saved tag "+tail, func(t *testing.T) {
			line := v140Line(t, "@hint"+tail, "invalid", 4, 2,
				v140Element(t, "tag", "@hint", "hint", 1, 6), v140Unparsed(t, tail, 6, 6+utf8.RuneCountInString(tail)))
			v140Run(t, v140Within(t, line, v140Close(t, 4, 2)), "current", []v140Expected{{6, 1, 6, 4, 1, 8}})
		})
	}
	t.Run("saved block tag without separator", func(t *testing.T) {
		line := v140Line(t, "@hint{", "block-start", 4, 2,
			v140Element(t, "tag", "@hint", "hint", 1, 6), v140Element(t, "block-open", "{", "{", 6, 7))
		v140Run(t, v140Within(t, line, v140Text(t, "Help.", 6, 3), v140Close(t, 6, 3), v140Close(t, 4, 2)),
			"current", []v140Expected{{6, 1, 6, 4, 1, 8}})
	})
	t.Run("saved empty tag", func(t *testing.T) {
		v140Run(t, v140Within(t, v140TagLine(t, "hint", "", "", 4, 2), v140Close(t, 4, 2)),
			"current", []v140Expected{{6, 1, 6, 4, 1, 8}})
	})
	t.Run("content classification with unparsed declaration", func(t *testing.T) {
		line := v140Invalid(t, "@hint Text", 4, 2)
		line.LineType = "content"
		v140Run(t, v140Within(t, line, v140Close(t, 4, 2)), "current", []v140Expected{{6, 1, 6, 4, 1, 8}})
	})
}

func TestV140NearestBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v140Expected
	}{
		{"several siblings", []model.Line{
			v140TagLine(t, "hint", "content", "First.", 4, 2), v140TagLine(t, "note", "content", "Second.", 4, 2), v140Close(t, 4, 2),
		}, []v140Expected{{6, 1, 6, 4, 1, 8}, {7, 1, 6, 4, 1, 8}}},
		{"nested note stops outer answer", []model.Line{
			v140Open(t, "note", 4, 2), v140TagLine(t, "hint", "content", "Help.", 6, 3), v140Close(t, 6, 3), v140Close(t, 4, 2),
		}, []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{"unknown block also stops outer answer", []model.Line{
			v140Line(t, "@unknown {", "block-start", 4, 2, v140Unparsed(t, "@unknown", 1, 9), v140Element(t, "block-open", "{", "{", 10, 11)),
			v140TagLine(t, "hint", "content", "Help.", 6, 3), v140Close(t, 6, 3), v140Close(t, 4, 2),
		}, nil},
		{"inner answer is checked independently", []model.Line{
			v140Open(t, "answer", 4, 2), v140TagLine(t, "hint", "content", "Help.", 6, 3), v140Close(t, 6, 3), v140Close(t, 4, 2),
		}, []v140Expected{{6, 1, 8, 4, 1, 8}, {7, 1, 6, 6, 1, 8}}},
		{"non-block parent does not stop search", []model.Line{
			v140TagLine(t, "task", "identifier", "practice", 4, 2), v140TagLine(t, "hint", "content", "Help.", 6, 3), v140Close(t, 4, 2),
		}, []v140Expected{{6, 1, 6, 4, 1, 8}, {7, 1, 6, 4, 1, 8}}},
		{"following sibling still checked", []model.Line{
			v140Open(t, "note", 4, 2), v140Text(t, "Read.", 6, 3), v140Close(t, 6, 3),
			v140TagLine(t, "hint", "content", "Help.", 4, 2), v140Close(t, 4, 2),
		}, []v140Expected{{6, 1, 6, 4, 1, 8}, {9, 1, 6, 4, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v140Run(t, v140Within(t, tc.body...), "current", tc.want) })
	}
	t.Run("task step depth", func(t *testing.T) {
		result := v140Document(t, v140TagLine(t, "task", "identifier", "t", 2, 1), v140TagLine(t, "step", "title", "First", 3, 2),
			v140Question(t, 4, 3), v140Open(t, "answer", 4, 3), v140Text(t, "London", 6, 4),
			v140TagLine(t, "hint", "content", "Help.", 6, 4), v140Close(t, 6, 4))
		v140Run(t, result, "current", []v140Expected{{8, 1, 6, 6, 1, 8}})
	})
	t.Run("first and last child", func(t *testing.T) {
		result := v140Document(t, v140Question(t, 2, 1), v140Open(t, "answer", 2, 1),
			v140TagLine(t, "hint", "content", "First.", 4, 2), v140Text(t, "London", 4, 2),
			v140Line(t, "", "blank", 4, 2), v140TagLine(t, "hint", "content", "Last.", 4, 2), v140Close(t, 4, 2))
		v140Run(t, result, "current", []v140Expected{{5, 1, 6, 4, 1, 8}, {8, 1, 6, 4, 1, 8}})
	})
}

func TestV140ParentRecoveryAndPositions(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		brace, end int
		want       []v140Expected
	}{
		{"@answer {", 9, 8, []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{"@ANSWER {", 9, 8, []v140Expected{{6, 1, 6, 4, 1, 8}}},
		{"@answer{", 8, 8, nil},
		{"@answer\t{", 9, 8, nil},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			opening := v140Line(t, tc.raw, "block-start", 2, 1,
				v140Unparsed(t, string([]rune(tc.raw)[:tc.end-1]), 1, tc.end), v140Element(t, "block-open", "{", "{", tc.brace, tc.brace+1))
			result := v140Document(t, v140Question(t, 2, 1), opening, v140Text(t, "London", 4, 2),
				v140TagLine(t, "hint", "content", "Help.", 4, 2), v140Close(t, 4, 2))
			v140Run(t, result, "current", tc.want)
		})
	}
	t.Run("indented original ranges", func(t *testing.T) {
		result := v140Document(t, v140Question(t, 2, 1),
			v140Line(t, "  @AnSwEr {", "block-start", 2, 1,
				v140Element(t, "tag", "@AnSwEr", "answer", 3, 10), v140Element(t, "block-open", "{", "{", 11, 12)),
			v140Text(t, "London", 4, 2), v140Line(t, "\t @HiNt Я😀", "tag", 4, 2,
				v140Element(t, "tag", "@HiNt", "hint", 3, 8), v140Element(t, "content", "Я😀", "Я😀", 9, 11)), v140Close(t, 4, 2))
		v140Run(t, result, "current", []v140Expected{{6, 3, 8, 4, 3, 10}})
	})
}

func TestV140LiteralAndExcludedContexts(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@hint Help.`, "@hint Help."}, {"London @hint", "London @hint"}, {"&#64;hint", "&#64;hint"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			line := v140Line(t, tc.raw, "content", 4, 2, v140Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v140Run(t, v140Within(t, line, v140Close(t, 4, 2)), "current", nil)
		})
	}
	for _, tc := range []struct {
		name string
		end  int
	}{
		{"editor", 8}, {"multifill", 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 2.8.11 и 4.4.11: псевдотег внутри этого блока остаётся content.
			v140Run(t, v140Within(t, v140Open(t, tc.name, 4, 2), v140Text(t, "@hint Help.", 6, 3),
				v140Close(t, 6, 3), v140Close(t, 4, 2)), "current", []v140Expected{{6, 1, tc.end, 4, 1, 8}})
		})
	}
	t.Run("literal answer opening is not a block", func(t *testing.T) {
		v140Run(t, v140Document(t, v140Open(t, "editor", 2, 1), v140Text(t, "@answer {", 3, 2),
			v140Text(t, "@hint Help.", 3, 2), v140Close(t, 3, 2), v140Text(t, "Continue.", 2, 1)), "current", nil)
	})
	t.Run("single-line answer", func(t *testing.T) {
		v140Run(t, v140Document(t, v140Question(t, 2, 1), v140TagLine(t, "answer", "content", "London", 2, 1),
			v140TagLine(t, "hint", "content", "Help.", 2, 1)), "current", nil)
	})
	t.Run("empty answer block", func(t *testing.T) {
		v140Run(t, v140Document(t, v140Question(t, 2, 1), v140Open(t, "answer", 2, 1), v140Close(t, 4, 2)), "current", nil)
	})
	v140Run(t, v140Unavailable(t, "1.2"), "current", nil)
}

func TestV140EndingsAndRepeatedRuns(t *testing.T) {
	for _, tc := range []struct {
		name string
		eol  model.LineEnding
		last model.LineEnding
	}{
		{"LF", model.LineEndingLF, model.LineEndingLF}, {"CRLF", model.LineEndingCRLF, model.LineEndingCRLF},
		{"LF no final EOL", model.LineEndingLF, model.LineEndingNone}, {"CRLF no final EOL", model.LineEndingCRLF, model.LineEndingNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v140Example(t)
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v140Run(t, result, "first", []v140Expected{{6, 1, 6, 4, 1, 8}})
			v140Run(t, result, "second", []v140Expected{{6, 1, 6, 4, 1, 8}})
		})
	}
}

// v140Registry использует полный штатный каталог регистраций.
func v140Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v140Unavailable создаёт согласованную модель с недоступными исходными строками.
func v140Unavailable(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v140Document добавляет обязательный префикс и согласованные ошибки для unparsed.
// Структура тела задаётся тестами явно; Raw здесь не разбирается.
func v140Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v140Unavailable(t, "1.2")
	title, encoding := "Lesson", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = append([]model.Line{
		v140TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v140Line(t, "# Lesson", "heading", 0, 0, v140Element(t, "heading-level", "#", "1", 1, 2), v140Element(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	return result
}

// v140Within размещает заданные строки после вопроса, открытия answer и London.
// Первая переданная строка получает номер 6; закрытие блока задаёт вызывающий тест.
func v140Within(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v140Document(t, append([]model.Line{v140Question(t, 2, 1), v140Open(t, "answer", 2, 1), v140Text(t, "London", 4, 2)}, body...)...)
}

// v140Example сохраняет дословный пример нарушения из таблицы.
func v140Example(t *testing.T) *model.Result {
	t.Helper()
	return v140Within(t, v140TagLine(t, "hint", "content", "Think of a capital.", 4, 2), v140Close(t, 4, 2))
}

// v140Line сохраняет готовую структуру строки и исходные элементы.
func v140Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v140Element сохраняет независимо заданные Raw, Value и исходные Unicode-колонки.
func v140Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v140Unparsed создаёт элемент с null Value; историю ошибки добавляет v140Document.
func v140Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v140Invalid задаёт целиком неразобранную физическую строку.
func v140Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v140Line(t, raw, "invalid", parent, depth, v140Unparsed(t, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v140TagLine создаёт заданный тег с необязательным типизированным параметром.
func v140TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	end := utf8.RuneCountInString(raw) + 1
	elements := []model.Element{v140Element(t, "tag", raw, name, 1, end)}
	if kind != "" {
		raw += " " + value
		elements = append(elements, v140Element(t, kind, value, value, end+1, end+1+utf8.RuneCountInString(value)))
	}
	return v140Line(t, raw, "tag", parent, depth, elements...)
}

// v140Question сохраняет вопрос из эталона и примера без изменений.
func v140Question(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v140TagLine(t, "question", "content", "Where does Tom live?", parent, depth)
}

// v140Text создаёт содержимое без экранирования и краевых пробелов.
func v140Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v140Line(t, raw, "content", parent, depth, v140Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v140Open создаёт объявление фигурного блока без параметров.
func v140Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v140Line(t, tag+" {", "block-start", parent, depth,
		v140Element(t, "tag", tag, name, 1, open-1), v140Element(t, "block-open", "{", "{", open, open+1))
}

// v140Fragment создаёт открытие блока с обязательным ID.
func v140Fragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v140Line(t, "@fragment "+id+" {", "block-start", parent, depth,
		v140Element(t, "tag", "@fragment", "fragment", 1, 10), v140Element(t, "identifier", id, id, 11, end),
		v140Element(t, "block-open", "{", "{", end+1, end+2))
}

// v140Close закрывает явно указанный родительский блок.
func v140Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v140Line(t, "}", "block-end", parent, depth, v140Element(t, "block-close", "}", "}", 1, 2))
}

// v140Run выполняет полный штатный конвейер и проверяет только текущую V140.
func v140Run(t *testing.T, result *model.Result, source string, want []v140Expected) {
	t.Helper()
	before := v140Snapshot(t, result)
	checks, err := validator.Prepare(result, v140Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v140Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	targetIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("new diagnostic has source %q, want %q", entry.Source, source)
		}
		if entry.DiagnosticCode == "V140" {
			got = append(got, entry)
			targetIDs[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V140 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		related := model.Location{Start: model.Position{Line: expected.parentLine, Column: expected.parentStart}, End: model.Position{Line: expected.parentLine, Column: expected.parentEnd}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v / %+v", i, entry.Location, entry.RelatedLocations, location, related)
		}
		if entry.Message != "Внутри @answer записан вложенный тег." || entry.DiagnosticCode != "V140" || entry.Source != source ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v140Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	// Убираются только добавленная история и производные эффекты конвейера.
	// Основной и связанный диапазоны ScopeLine не создают ссылок Element.ErrorIDs.
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			for _, id := range element.ErrorIDs {
				if targetIDs[id] {
					t.Errorf("V140 linked to element at %d:%d", i+1, j)
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v140Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v140Snapshot сохраняет модель для проверки отсутствия побочных изменений.
func v140Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

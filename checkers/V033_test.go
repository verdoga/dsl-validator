package checkers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v033Expected задаёт независимые основные и связанные Unicode-диапазоны.
type v033Expected struct {
	line, start, end                         int
	previousLine, previousStart, previousEnd int
}

func TestV033Registration(t *testing.T) {
	definitions, err := v033Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V033" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "element" ||
			definition.Message != "ID задания повторяется в пределах одного шага." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V033 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V033 definitions = %d, want 1", matches)
	}
}

func TestV033IDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
		want []v033Expected
	}{
		{"no tasks", nil, nil},
		{"one task", []string{"x"}, nil},
		{"different IDs", []string{"x", "y"}, nil},
		{"initial step and ignored case", []string{"x", "X"}, []v033Expected{{5, 7, 8, 3, 7, 8}}},
		{"three occurrences", []string{"x", "X", "x"}, []v033Expected{{5, 7, 8, 3, 7, 8}, {7, 7, 8, 3, 7, 8}}},
		{"Unicode case", []string{"КОД", "код"}, []v033Expected{{5, 7, 10, 3, 7, 10}}},
		{"allowed punctuation", []string{"a_1.2-b", "A_1.2-B"}, []v033Expected{{5, 7, 14, 3, 7, 14}}},
		{"period ID", []string{".", "."}, []v033Expected{{5, 7, 8, 3, 7, 8}}},
		{"underscore ID", []string{"_", "_"}, []v033Expected{{5, 7, 8, 3, 7, 8}}},
		{"Unicode digits", []string{"١٢", "١٢"}, []v033Expected{{5, 7, 9, 3, 7, 9}}},
		{"no width normalization", []string{"A", "Ａ"}, nil},
		{"no full case folding", []string{"ß", "SS"}, nil},
		{"no final sigma folding", []string{"Σ", "ς"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v033Run(t, v033PlainTasks(t, tc.ids...), "current", tc.want)
		})
	}
}

func TestV033InvalidIDs(t *testing.T) {
	for _, id := range []string{"", "bad!", "bad id", " x", "x ", "a/b", "e\u0301"} {
		t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
			// Контракт допускает identifier с ошибочным Value и точным исходным
			// диапазоном, включая пустой. Ошибка парсера сохранена на этом элементе.
			result := v033PlainTasks(t, id, id)
			v033ParserError(t, result, 3, 1, "Недопустимый ID задания.")
			v033ParserError(t, result, 5, 1, "Недопустимый ID задания.")
			v033Run(t, result, "current", nil)
		})
	}
	t.Run("missing identifier", func(t *testing.T) {
		result := v033Document(t,
			v033TagLine(t, "task", 2, 1), v033Text(t, "Read.", 3, 2),
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Write.", 5, 2))
		v033ParserError(t, result, 3, 0, "Отсутствует ID задания.")
		v033Run(t, result, "current", nil)
	})
	t.Run("invalid ID does not reset history", func(t *testing.T) {
		result := v033PlainTasks(t, "x", "bad!", "X")
		v033ParserError(t, result, 5, 1, "Недопустимый ID задания.")
		v033Run(t, result, "current", []v033Expected{{7, 7, 8, 3, 7, 8}})
	})
	t.Run("invalid ID is not remembered", func(t *testing.T) {
		result := v033PlainTasks(t, " x", "x", "X")
		v033ParserError(t, result, 3, 1, "Недопустимый ID задания.")
		v033Run(t, result, "current", []v033Expected{{7, 7, 8, 5, 7, 8}})
	})
}

func TestV033SourcePositionsAndCanonicalTag(t *testing.T) {
	result := v033PlainTasks(t, "x", "X")
	// DSL 1.11 и 1.14: имя тега нечувствительно к регистру, отступы и
	// несколько разделительных пробелов не меняют его смысл и исходные позиции.
	result.Lines[2] = v033ModelLine(t, " @task  x", "tag", 2, 1,
		v033ModelElement(t, "tag", "@task", "task", 2, 7),
		v033ModelElement(t, "identifier", "x", "x", 9, 10))
	result.Lines[2].Number = 3
	result.Lines[4] = v033ModelLine(t, "  @TASK   X", "tag", 2, 1,
		v033ModelElement(t, "tag", "@TASK", "task", 3, 8),
		v033ModelElement(t, "identifier", "X", "X", 11, 12))
	result.Lines[4].Number = 5
	v033Run(t, result, "current", []v033Expected{{5, 11, 12, 3, 9, 10}})
}

func TestV033OtherIdentifierFieldsAreNotTasks(t *testing.T) {
	result := v033Document(t)
	id := "x"
	result.Document.Metadata.DocumentID = &id
	result.Lines = []model.Line{
		result.Lines[0],
		v033NamedLine(t, "document-id", "identifier", "x", 0, 0),
		result.Lines[1],
		v033TaskLine(t, "x", 3, 1), v033Text(t, "Read.", 4, 2),
	}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count := 5
	result.Document.LineCount = &count
	v033Run(t, result, "current", nil)
}

func TestV033ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Обе обязательные строки документа добавляются перед фрагментом.
		// Эталон сохраняет точные текст, регистр и последовательность строк.
		result := v033Document(t,
			v033Heading(t, 3, "First"),
			v033TaskLine(t, "A", 3, 1), v033Text(t, "Read.", 4, 2),
			v033TagLine(t, "newpage", 0, 0),
			v033TaskLine(t, "a", 6, 1), v033Text(t, "Write.", 7, 2))
		v033Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v033Document(t,
			v033Heading(t, 3, "Practice"),
			v033Block(t, "variants", 3, 1),
			v033VariantLine(t, "A", 4, 2),
			v033TaskLine(t, "x", 5, 3), v033Text(t, "Read.", 6, 4),
			v033TaskLine(t, "X", 5, 3), v033Text(t, "Write.", 8, 4),
			v033VariantLine(t, "B", 4, 2),
			v033TaskLine(t, "x", 10, 3), v033Text(t, "Speak.", 11, 4),
			v033Close(t, 4, 2))
		v033Run(t, result, "current", []v033Expected{{8, 7, 8, 6, 7, 8}})
	})
}

func TestV033Variants(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v033Expected
	}{
		{"different branches of one block", []model.Line{
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 4, 3), v033Text(t, "Read.", 5, 4),
			v033VariantLine(t, "B", 3, 2),
			v033TaskLine(t, "X", 7, 3), v033Text(t, "Write.", 8, 4),
			v033Close(t, 3, 2),
		}, nil},
		{"first matching ID is excluded but later one is eligible", []model.Line{
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 4, 3), v033Text(t, "Read.", 5, 4),
			v033VariantLine(t, "B", 3, 2),
			v033TaskLine(t, "x", 7, 3), v033Text(t, "Read.", 8, 4),
			v033TaskLine(t, "X", 7, 3), v033Text(t, "Write.", 10, 4),
			v033Close(t, 3, 2),
		}, []v033Expected{{10, 7, 8, 8, 7, 8}}},
		{"outside task before variants compares with both branches", []model.Line{
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 5, 2),
			v033TaskLine(t, "x", 6, 3), v033Text(t, "Read.", 7, 4),
			v033VariantLine(t, "B", 5, 2),
			v033TaskLine(t, "X", 9, 3), v033Text(t, "Write.", 10, 4),
			v033Close(t, 5, 2),
		}, []v033Expected{{7, 7, 8, 3, 7, 8}, {10, 7, 8, 3, 7, 8}}},
		{"outside task after variants chooses first branch occurrence", []model.Line{
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 4, 3), v033Text(t, "Read.", 5, 4),
			v033VariantLine(t, "B", 3, 2),
			v033TaskLine(t, "x", 7, 3), v033Text(t, "Speak.", 8, 4),
			v033Close(t, 3, 2),
			v033TaskLine(t, "X", 2, 1), v033Text(t, "Write.", 11, 2),
		}, []v033Expected{{11, 7, 8, 5, 7, 8}}},
		{"different blocks compare despite equal branch names", []model.Line{
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 4, 3), v033Text(t, "Read.", 5, 4),
			v033VariantLine(t, "B", 3, 2),
			v033TaskLine(t, "y", 7, 3), v033Text(t, "Write.", 8, 4),
			v033Close(t, 3, 2),
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 11, 2),
			v033TaskLine(t, "X", 12, 3), v033Text(t, "Read.", 13, 4),
			v033VariantLine(t, "B", 11, 2),
			v033TaskLine(t, "z", 15, 3), v033Text(t, "Speak.", 16, 4),
			v033Close(t, 11, 2),
		}, []v033Expected{{13, 7, 8, 5, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v033Run(t, v033Document(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("branch identity uses declaration line rather than name", func(t *testing.T) {
		result := v033Document(t,
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 4, 3), v033Text(t, "Read.", 5, 4),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 7, 3), v033Text(t, "Write.", 8, 4),
			v033Close(t, 3, 2))
		// DSL 4.9.6: одинаковое имя ветви ошибочно. V033 не проверяет имена
		// и обязана отличать эти две распознанные ветви по строкам объявлений.
		v033ParserError(t, result, 7, 1, "Повторное имя ветви.")
		v033Run(t, result, "current", nil)
	})
}

func TestV033StepBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v033Expected
	}{
		{"heading three starts a new step", []model.Line{
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
			v033Heading(t, 3, "Next"),
			v033TaskLine(t, "X", 5, 1), v033Text(t, "Write.", 6, 2),
		}, nil},
		{"newpage starts a new step", []model.Line{
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
			v033TagLine(t, "newpage", 0, 0),
			v033TaskLine(t, "X", 5, 1), v033Text(t, "Write.", 6, 2),
		}, nil},
		{"header does not start a new step", []model.Line{
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
			v033NamedLine(t, "header", "title", "Part", 2, 1),
			v033TaskLine(t, "X", 2, 1), v033Text(t, "Write.", 6, 2),
		}, []v033Expected{{6, 7, 8, 3, 7, 8}}},
		{"endtask does not start a new step", []model.Line{
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
			v033TagLine(t, "endtask", 3, 2),
			v033TaskLine(t, "X", 2, 1), v033Text(t, "Write.", 6, 2),
		}, []v033Expected{{6, 7, 8, 3, 7, 8}}},
		{"step does not start a new document step", []model.Line{
			v033TaskLine(t, "x", 2, 1),
			v033NamedLine(t, "step", "title", "First", 3, 2), v033Text(t, "Read.", 4, 3),
			v033TaskLine(t, "X", 2, 1), v033Text(t, "Write.", 6, 2),
		}, []v033Expected{{6, 7, 8, 3, 7, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v033Run(t, v033Document(t, tc.body...), "current", tc.want)
		})
	}
	for _, level := range []int{1, 2} {
		t.Run(fmt.Sprintf("heading %d does not reset history", level), func(t *testing.T) {
			result := v033Document(t,
				v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
				v033Heading(t, level, "Extra"),
				v033TaskLine(t, "X", 5, 1), v033Text(t, "Write.", 6, 2))
			if level == 1 {
				result.Document.Metadata.Title = nil
			} else {
				subtitle := "Extra"
				result.Document.Metadata.Subtitle = &subtitle
			}
			// DSL 2.1.3–2.1.4: размещение заголовка ошибочно, но распознанный
			// уровень 1 или 2 по условиям V033 не сбрасывает историю заданий.
			v033ParserError(t, result, 5, 0, "Недопустимое размещение заголовка.")
			v033Run(t, result, "current", []v033Expected{{6, 7, 8, 3, 7, 8}})
		})
	}
}

func TestV033InvalidNewpage(t *testing.T) {
	t.Run("newpage inside a figure block does not reset history", func(t *testing.T) {
		result := v033Document(t,
			v033Block(t, "variants", 2, 1),
			v033VariantLine(t, "A", 3, 2),
			v033TaskLine(t, "x", 4, 3), v033Text(t, "Read.", 5, 4),
			v033TagLine(t, "newpage", 4, 3),
			v033TaskLine(t, "X", 4, 3), v033Text(t, "Write.", 8, 4),
			v033VariantLine(t, "B", 3, 2),
			v033TaskLine(t, "y", 10, 3), v033Text(t, "Speak.", 11, 4),
			v033Close(t, 3, 2))
		v033ParserError(t, result, 7, 0, "Внутри variants запрещён newpage.")
		v033Run(t, result, "current", []v033Expected{{8, 7, 8, 5, 7, 8}})
	})
	t.Run("newpage with extra parameters does not reset history", func(t *testing.T) {
		line := v033ModelLine(t, "@newpage extra", "tag", 2, 1,
			v033ModelElement(t, "tag", "@newpage", "newpage", 1, 9),
			model.Element{ElementType: "unparsed", Raw: "extra", Start: 10, End: 15, ErrorIDs: []string{}})
		result := v033Document(t,
			v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2), line,
			v033TaskLine(t, "X", 2, 1), v033Text(t, "Write.", 6, 2))
		v033ParserError(t, result, 5, 1, "Тег newpage не принимает параметры.")
		v033Run(t, result, "current", []v033Expected{{6, 7, 8, 3, 7, 8}})
	})
}

func TestV033LiteralContent(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			// DSL 2.8.11 и 4.4.11: строки, похожие на теги, остаются содержимым.
			result := v033Document(t,
				v033TaskLine(t, "x", 2, 1), v033Text(t, "Read.", 3, 2),
				v033Block(t, name, 3, 2),
				v033Text(t, "@newpage", 5, 3), v033Text(t, "@task x", 5, 3),
				v033Close(t, 5, 3),
				v033TaskLine(t, "X", 2, 1), v033Text(t, "Write.", 9, 2))
			v033Run(t, result, "current", []v033Expected{{9, 7, 8, 3, 7, 8}})
		})
	}
}

func TestV033LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name     string
		eol      model.LineEnding
		lastNone bool
	}{
		{"LF", model.LineEndingLF, false},
		{"CRLF", model.LineEndingCRLF, false},
		{"LF without final EOL", model.LineEndingLF, true},
		{"CRLF without final EOL", model.LineEndingCRLF, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v033PlainTasks(t, "x", "X")
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			if tc.lastNone {
				result.Lines[len(result.Lines)-1].LineEnding = model.LineEndingNone
			}
			v033Run(t, result, "current", []v033Expected{{5, 7, 8, 3, 7, 8}})
		})
	}
}

func TestV033RepeatedRuns(t *testing.T) {
	result := v033PlainTasks(t, "x", "X")
	v033Run(t, result, "first", []v033Expected{{5, 7, 8, 3, 7, 8}})
	v033Run(t, result, "second", []v033Expected{{5, 7, 8, 3, 7, 8}})
	v033Run(t, v033PlainTasks(t, "x"), "first", nil)
}

func TestV033UnavailableLines(t *testing.T) {
	v033Run(t, v033UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV033UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v033UnavailableDocument(t, version)
			before := v033Snapshot(t, result)
			checks, err := validator.Prepare(result, v033Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v033Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

// v033Registry использует штатный каталог без замены регистраций.
func v033Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v033UnavailableDocument задаёт версию при недоступных исходных строках.
func v033UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v033Document добавляет обязательные @dsl-version и заголовок по DSL 1.1–1.3,
// 1.30 и 2.1.3. Все родители и элементы тела заданы вызывающим тестом явно.
func v033Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v033UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{
		v033ModelLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v033ModelElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v033ModelElement(t, "version", "1.2", "1.2", 14, 17)),
		v033Heading(t, 1, "Unit 1"),
	}
	result.Lines = append(result.Lines, body...)
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count, title, encoding := len(result.Lines), "Unit 1", "UTF-8"
	result.Document.LineCount = &count
	result.Document.Metadata.Title = &title
	result.Document.Encoding = &encoding
	return result
}

// v033PlainTasks строит полные задания заданных ID в начальном шаге.
// Вычисление номеров относится только к входной структуре, не к ожиданиям.
func v033PlainTasks(t *testing.T, ids ...string) *model.Result {
	t.Helper()
	var body []model.Line
	for i, id := range ids {
		body = append(body, v033TaskLine(t, id, 2, 1), v033Text(t, "Read.", 3+2*i, 2))
	}
	return v033Document(t, body...)
}

// v033ModelLine сохраняет явно заданную структуру строки, не разбирая Raw.
func v033ModelLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v033ModelElement сохраняет заданные Raw, Value и исходный Unicode-диапазон.
func v033ModelElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v033Text строит готовое содержимое без краевых пробелов и экранирования.
func v033Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v033ModelLine(t, raw, "content", parent, depth,
		v033ModelElement(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v033TagLine строит заданный однострочный тег без параметров.
func v033TagLine(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	return v033ModelLine(t, raw, "tag", parent, depth,
		v033ModelElement(t, "tag", raw, name, 1, utf8.RuneCountInString(raw)+1))
}

// v033NamedLine строит заданный тег с одним явно типизированным параметром.
func v033NamedLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	start := utf8.RuneCountInString(tag) + 2
	return v033ModelLine(t, tag+" "+value, "tag", parent, depth,
		v033ModelElement(t, "tag", tag, name, 1, start-1),
		v033ModelElement(t, kind, value, value, start, start+utf8.RuneCountInString(value)))
}

// v033TaskLine строит объявление задания без проверки допустимости его ID.
func v033TaskLine(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	return v033NamedLine(t, "task", "identifier", id, parent, depth)
}

// v033VariantLine строит объявление ветви с заданным именем и родителем.
func v033VariantLine(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v033NamedLine(t, "variant", "name", name, parent, depth)
}

// v033Block строит объявление заданного фигурного блока без параметров.
func v033Block(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v033ModelLine(t, tag+" {", "block-start", parent, depth,
		v033ModelElement(t, "tag", tag, name, 1, open-1), v033ModelElement(t, "block-open", "{", "{", open, open+1))
}

// v033Close строит закрытие заданного фигурного родителя.
func v033Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v033ModelLine(t, "}", "block-end", parent, depth, v033ModelElement(t, "block-close", "}", "}", 1, 2))
}

// v033Heading строит корневой заголовок заданного уровня.
func v033Heading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v033ModelLine(t, marker+" "+title, "heading", 0, 0,
		v033ModelElement(t, "heading-level", marker, fmt.Sprint(level), 1, level+1),
		v033ModelElement(t, "title", title, title, level+2, level+2+utf8.RuneCountInString(title)))
}

// v033ParserError добавляет согласованную прежнюю ошибку на заданный элемент.
// Код тестовый и непрозрачный; конкретные коды внешнего парсера не предполагаются.
func v033ParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	line.HasErrors = true
	result.Document.HasErrors = true
	element.ErrorIDs = append(element.ErrorIDs, id)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
		DiagnosticScope: "element", Message: message,
		Location: &model.Location{
			Start: model.Position{Line: number, Column: element.Start},
			End:   model.Position{Line: number, Column: element.End},
		}, RelatedLocations: []model.Location{},
	})
}

// v033Run проверяет штатный конвейер, точные диапазоны V033 и эффекты ScopeElement.
// Флаги и обратные ссылки ожидаемой модели задаются только явными ожиданиями теста.
func v033Run(t *testing.T, result *model.Result, processingID string, want []v033Expected) {
	t.Helper()
	before := v033Snapshot(t, result)
	expected := v033Clone(t, result)
	diagnosticCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v033Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v033Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	processing := model.Processing{ID: processingID, Tool: "dsl-validator"}
	result.Processing = append(result.Processing, processing)
	expected.Processing = append(expected.Processing, processing)
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V033" && entry.Source != processingID {
			t.Fatalf("new V033 has unexpected source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V033" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V033 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: w.line, Column: w.start}, End: model.Position{Line: w.line, Column: w.end}}
		related := model.Location{Start: model.Position{Line: w.previousLine, Column: w.previousStart}, End: model.Position{Line: w.previousLine, Column: w.previousEnd}}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("occurrence %d related locations = %+v, want %+v", i, entry.RelatedLocations, related)
		}
		if entry.DiagnosticCode != "V033" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "element" || entry.Fatal || entry.Message != "ID задания повторяется в пределах одного шага." {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
		expected.Document.HasErrors = true
		line := &expected.Lines[w.line-1]
		line.HasErrors = true
		found := false
		for index := range line.Elements {
			element := &line.Elements[index]
			if element.ElementType == "identifier" && element.Start == w.start && element.End == w.end {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				found = true
			}
		}
		if !found {
			t.Fatal("expected occurrence does not point to an input identifier")
		}
	}
	otherNewIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V033" {
			otherNewIDs[entry.ID] = true
		}
	}
	// Общие флаги проверяются штатным контролем целостности с учётом
	// всех диагностик. В снимке оставляем прежние данные и эффекты V033.
	if _, err := validator.Prepare(result, v033Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	actual := v033Clone(t, result)
	actual.Document.HasErrors = expected.Document.HasErrors
	for i := range actual.Lines {
		actual.Lines[i].HasErrors = expected.Lines[i].HasErrors
	}
	for i := range actual.Lines {
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			kept := element.ErrorIDs[:0]
			for _, id := range element.ErrorIDs {
				if !otherNewIDs[id] {
					kept = append(kept, id)
				}
			}
			element.ErrorIDs = kept
		}
	}
	expected.Diagnostics = append(expected.Diagnostics, result.Diagnostics[diagnosticCount:]...)
	if v033Snapshot(t, actual) != v033Snapshot(t, expected) {
		t.Fatal("unexpected document, line, flag, element link, processing or previous history changes")
	}
}

// v033Snapshot сохраняет полную модель для сравнения без побочных изменений.
func v033Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v033Clone создаёт независимую копию входной модели для явных ожидаемых эффектов.
func v033Clone(t *testing.T, result *model.Result) *model.Result {
	t.Helper()
	var cloned model.Result
	if err := json.Unmarshal([]byte(v033Snapshot(t, result)), &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

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

// v084TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V084.
func v084TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v084TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v084TestHeading(t, "", "#", "1", title),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
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
	}
	result.Lines[len(result.Lines)-1].LineEnding = ""
	return result
}

// v084TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v084TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v084TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v084TestElement(t, "heading-level", marker, level, start, end),
		v084TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v084TestInvalid задаёт ошибочную строку без распознанного tag.
func v084TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v084TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v084TestLine(t, raw, "invalid", parent, depth, element)
}

// v084TestTag собирает однострочный тег с одним явно типизированным параметром.
func v084TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v084TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v084TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v084TestLine(t, raw, "tag", parent, depth, elements...)
}

// v084TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v084TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v084TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v084TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v084TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v084TestClose задаёт закрытие блока с явным родителем.
func v084TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v084TestLine(t, "}", "block-end", parent, depth, v084TestElement(t, "block-close", "}", "}", 1, 2))
}

// v084TestText сохраняет независимо заданные исходное и семантическое значения.
func v084TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v084TestLine(t, raw, "content", parent, depth,
		v084TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v084TestLine не классифицирует исходник и не вычисляет родительские связи.
func v084TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v084TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v084TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v084TestRegistry использует штатный каталог целиком.
func v084TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v084TestRun проверяет новые V084: строка, начало и исключающий конец.
func v084TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v084TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v084TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v084TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V084" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V084 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "После закрывающей кавычки пути медиа есть лишнее содержимое." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V084 metadata", i, entry, location)
		}
		if len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v084TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 ||
		len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v084TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v084TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v084TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v084TestMedia задаёт тип и SOURCE раздельно, не разбирая путь и экранирование.
// broken сохраняет SOURCE целиком как unparsed; значение задано независимо.
func v084TestMedia(t *testing.T, kind, source, value string, broken bool, parent, depth int) model.Line {
	t.Helper()
	end := 14 + utf8.RuneCountInString(source)
	element := v084TestElement(t, "source", source, value, 14, end)
	if broken {
		element = v084TestElement(t, "unparsed", source, "", 14, end)
		element.Value = nil
	}
	return v084TestLine(t, "@media "+kind+" "+source, "tag", parent, depth,
		v084TestElement(t, "tag", "@media", "media", 1, 7),
		v084TestElement(t, "media-type", kind, strings.ToLower(kind), 8, 13), element)
}

func TestV084Registration(t *testing.T) {
	definitions, err := v084TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V084" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "После закрывающей кавычки пути медиа есть лишнее содержимое." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V084 definitions = %d, want 1", count)
	}
}

func TestV084ReferenceAndExample(t *testing.T) {
	// DSL 1.3, 1.30 и 2.1.3 требуют дополнить фрагменты версией и # Lesson.
	// Сами фрагменты сохранены без изменений и расположены на строке 3.
	for _, tc := range []struct {
		name, fragment, source, value string
		broken                        bool
		want                          [][3]int
	}{
		{"reference", `@media audio "track.mp3"`, `"track.mp3"`, "track.mp3", false, nil},
		{"example", `@media audio "track.mp3" extra`, `"track.mp3" extra`, "", true, [][3]int{{3, 26, 31}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v084TestDocument(t, v084TestMedia(t, "audio", tc.source, tc.value, tc.broken, 2, 1))
			if result.Lines[2].Raw != tc.fragment || result.Lines[2].LineEnding != "" {
				t.Fatal("fragment changed")
			}
			v084TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV084TailRanges(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         [][3]int
	}{
		{"one word", `"a" extra`, [][3]int{{3, 18, 23}}},
		{"one character", `"a"x`, [][3]int{{3, 17, 18}}},
		{"trim only edges", "\"a\" \textra \t", [][3]int{{3, 19, 24}}},
		{"internal spaces", `"a" one  TWO`, [][3]int{{3, 18, 26}}},
		{"internal tab", "\"a\" x\ty", [][3]int{{3, 18, 21}}},
		{"second source", `"a" "b"`, [][3]int{{3, 18, 21}}},
		{"adjacent second source", `"a""b"`, [][3]int{{3, 17, 20}}},
		{"empty second source", `"a" ""`, [][3]int{{3, 18, 20}}},
		{"unclosed second source", `"a" "b`, [][3]int{{3, 18, 20}}},
		{"multiple values", `"a" "b" "c"`, [][3]int{{3, 18, 25}}},
		{"NBSP only", "\"a\"\u00a0", [][3]int{{3, 17, 18}}},
		{"NBSP between trimmed edges", "\"a\" \t\u00a0 \t", [][3]int{{3, 19, 20}}},
		{"em space", "\"a\"\u2003", [][3]int{{3, 17, 18}}},
		{"vertical tab", "\"a\"\v", [][3]int{{3, 17, 18}}},
		{"Unicode source and tail", `"Я😀" 尾😀`, [][3]int{{3, 19, 21}}},
		{"combining mark in tail", "\"a\" e\u0301", [][3]int{{3, 18, 20}}},
		{"tail is not decoded", `"a" \"`, [][3]int{{3, 18, 20}}},
		{"backslash does not protect trailing space", `"a" x\ `, [][3]int{{3, 18, 20}}},
		{"closing brace is extra", `"a" }`, [][3]int{{3, 18, 19}}},
		{"second tag is entire tail", `"a" @media audio "b"`, [][3]int{{3, 18, 34}}},
		{"empty source with tail", `"" extra`, [][3]int{{3, 17, 22}}},
		{"blank source with tail", `" " extra`, [][3]int{{3, 18, 23}}},
		{"empty source without tail", `""`, nil},
		{"blank source without tail", `"   "`, nil},
		{"opening only", `"`, nil},
		{"unclosed source", `"path`, nil},
		{"unclosed with spaces", `"path extra`, nil},
		{"typographic closing quote", `"path” extra`, nil},
		{"trailing backslash in source", `"path\`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Ошибочная область SOURCE и хвоста сохранена парсером как unparsed.
			v084TestRun(t, v084TestDocument(t, v084TestMedia(t, "audio", tc.source, "", true, 2, 1)), "current", tc.want)
		})
	}
	for _, tail := range []string{"", " ", "\t", " \t \t"} {
		t.Run("allowed tail="+strconv.Quote(tail), func(t *testing.T) {
			line := v084TestMedia(t, "audio", `"a"`, "a", false, 2, 1)
			line.Raw += tail
			v084TestRun(t, v084TestDocument(t, line), "current", nil)
		})
	}
}

func TestV084Escaping(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         [][3]int
	}{
		{"escaped quote then closure", `"a\"b" tail`, [][3]int{{3, 21, 25}}},
		{"escaped quote without closure", `"a\"b tail`, nil},
		{"two backslashes", `"a\\" tail`, [][3]int{{3, 20, 24}}},
		{"three backslashes", `"a\\\" tail`, nil},
		{"four backslashes", `"a\\\\" tail`, [][3]int{{3, 22, 26}}},
		{"five backslashes", `"a\\\\\" tail`, nil},
		{"six backslashes", `"a\\\\\\" tail`, [][3]int{{3, 24, 28}}},
		{"three backslashes then separate closure", `"a\\\"" tail`, [][3]int{{3, 22, 26}}},
		{"backslash before ordinary character", `"a\q" tail`, [][3]int{{3, 20, 24}}},
		{"pair before ordinary character", `"a\\q" tail`, [][3]int{{3, 21, 25}}},
		{"two escaped quotes", `"a\"b\"c" tail`, [][3]int{{3, 24, 28}}},
		{"quote in tail cannot change first closure", `"a" extra\"`, [][3]int{{3, 18, 25}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v084TestRun(t, v084TestDocument(t, v084TestMedia(t, "audio", tc.source, "", true, 2, 1)), "current", tc.want)
		})
	}
	t.Run("decoded source is not rescanned", func(t *testing.T) {
		line := v084TestMedia(t, "audio", `"a\"b"`, `a"b`, false, 2, 1)
		line.Raw += " tail"
		unparsed := v084TestElement(t, "unparsed", "tail", "", 21, 25)
		unparsed.Value = nil
		line.Elements = append(line.Elements, unparsed)
		v084TestRun(t, v084TestDocument(t, line), "current", [][3]int{{3, 21, 25}})
	})
}

func TestV084PrefixesAndParsing(t *testing.T) {
	for _, kind := range []string{"audio", "video", "image", "AUDIO", "Video", "IMAGE"} {
		t.Run(kind, func(t *testing.T) {
			v084TestRun(t, v084TestDocument(t, v084TestMedia(t, kind, `"a" extra`, "", true, 2, 1)), "current", [][3]int{{3, 18, 23}})
		})
	}
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"fully unparsed", `@media audio "track.mp3" extra`, [][3]int{{3, 26, 31}}},
		{"case insensitive", `@MEDIA VIDEO "a" EXTRA`, [][3]int{{3, 18, 23}}},
		{"indentation", " \t@media audio \"a\" extra", [][3]int{{3, 20, 25}}},
		{"multiple field spaces", `@media  audio   "a" extra`, [][3]int{{3, 21, 26}}},
		{"invalid type", `@media sound "a" extra`, nil},
		{"longer type", `@media audioExtra "a" extra`, nil},
		{"missing type", `@media "a" extra`, nil},
		{"missing source", "@media audio", nil},
		{"only tag", "@media", nil},
		{"longer tag", `@mediaExtra audio "a" extra`, nil},
		{"tag tab separator", "@media\taudio \"a\" extra", nil},
		{"type tab separator", "@media audio\t\"a\" extra", nil},
		{"mixed tag separator", "@media \taudio \"a\" extra", nil},
		{"mixed type separator", "@media audio \t\"a\" extra", nil},
		{"tag NBSP separator", "@media\u00a0audio \"a\" extra", nil},
		{"type NBSP separator", "@media audio\u00a0\"a\" extra", nil},
		{"no tag separator", `@mediaaudio "a" extra`, nil},
		{"no type separator", `@media audio"a" extra`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v084TestRun(t, v084TestDocument(t, v084TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@media audio "a" extra`, `@media audio "a" extra`},
		{`Read @media audio "a" extra`, `Read @media audio "a" extra`},
		{"\u00a0@media audio \"a\" extra", "\u00a0@media audio \"a\" extra"},
		{"\u2003@media audio \"a\" extra", "\u2003@media audio \"a\" extra"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v084TestRun(t, v084TestDocument(t, v084TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, source := range []string{"12", "track.mp3"} {
		t.Run("unquoted "+source, func(t *testing.T) {
			v084TestRun(t, v084TestDocument(t, v084TestMedia(t, "audio", source, source, false, 2, 1)), "current", nil)
		})
	}
	t.Run("quote in unquoted source", func(t *testing.T) {
		v084TestRun(t, v084TestDocument(t, v084TestMedia(t, "audio", `path" extra`, "", true, 2, 1)), "current", nil)
	})
	for _, lineType := range []model.LineType{"tag", "invalid"} {
		t.Run("parsed source and unparsed tail "+string(lineType), func(t *testing.T) {
			line := v084TestMedia(t, "audio", `"track.mp3"`, "track.mp3", false, 2, 1)
			line.LineType = lineType
			line.Raw += " extra"
			tail := v084TestElement(t, "unparsed", "extra", "", 26, 31)
			tail.Value = nil
			line.Elements = append(line.Elements, tail)
			v084TestRun(t, v084TestDocument(t, line), "current", [][3]int{{3, 26, 31}})
		})
	}
	t.Run("unparsed type source and tail", func(t *testing.T) {
		unparsed := v084TestElement(t, "unparsed", `audio "track.mp3" extra`, "", 8, 31)
		unparsed.Value = nil
		line := v084TestLine(t, `@media audio "track.mp3" extra`, "invalid", 2, 1,
			v084TestElement(t, "tag", "@media", "media", 1, 7), unparsed)
		v084TestRun(t, v084TestDocument(t, line), "current", [][3]int{{3, 26, 31}})
	})
}

func TestV084PhysicalLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"two violations", []model.Line{
			v084TestMedia(t, "audio", `"a" extra`, "", true, 2, 1), v084TestMedia(t, "video", `"a" extra`, "", true, 2, 1),
		}, [][3]int{{3, 18, 23}, {4, 18, 23}}},
		{"last object", []model.Line{
			v084TestMedia(t, "audio", `"a"`, "a", false, 2, 1), v084TestMedia(t, "video", `"a" extra`, "", true, 2, 1),
		}, [][3]int{{4, 18, 23}}},
		{"first declaration only", []model.Line{
			v084TestMedia(t, "audio", `"a" extra`, "", true, 2, 1), v084TestMedia(t, "image", `"a"`, "a", false, 2, 1),
		}, [][3]int{{3, 18, 23}}},
		{"closing on next line is not used", []model.Line{
			v084TestMedia(t, "audio", `"a`, "", true, 2, 1), v084TestText(t, `" extra`, `" extra`, 2, 1),
		}, nil},
		{"text on next line is not tail", []model.Line{
			v084TestMedia(t, "audio", `"a"`, "a", false, 2, 1), v084TestText(t, "extra", "extra", 2, 1),
		}, nil},
		{"unclosed preceding source does not affect next", []model.Line{
			v084TestMedia(t, "audio", `"a`, "", true, 2, 1), v084TestMedia(t, "video", `"a" extra`, "", true, 2, 1),
		}, [][3]int{{4, 18, 23}}},
		{"blank between violations", []model.Line{
			v084TestMedia(t, "audio", `"a" extra`, "", true, 2, 1), v084TestLine(t, " \t", "blank", 2, 1),
			v084TestMedia(t, "video", `"a" extra`, "", true, 2, 1),
		}, [][3]int{{3, 18, 23}, {5, 18, 23}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v084TestRun(t, v084TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV084Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"forbidden nesting still checked", []model.Line{
			v084TestOpen(t, "note", 2, 1), v084TestMedia(t, "audio", `"a" extra`, "", true, 3, 2), v084TestClose(t, 3, 2),
		}, [][3]int{{4, 18, 23}}},
		{"nested blocks", []model.Line{
			v084TestOpen(t, "note", 2, 1), v084TestOpen(t, "hint", 3, 2),
			v084TestMedia(t, "audio", `"a" extra`, "", true, 4, 3), v084TestClose(t, 4, 3), v084TestClose(t, 3, 2),
		}, [][3]int{{5, 18, 23}}},
		{"fragment is not literal", []model.Line{
			v084TestLine(t, "@fragment f {", "block-start", 2, 1,
				v084TestElement(t, "tag", "@fragment", "fragment", 1, 10), v084TestElement(t, "identifier", "f", "f", 11, 12),
				v084TestElement(t, "block-open", "{", "{", 13, 14)),
			v084TestMedia(t, "audio", `"a" extra`, "", true, 3, 2), v084TestClose(t, 3, 2),
		}, [][3]int{{4, 18, 23}}},
		{"branch", []model.Line{
			v084TestOpen(t, "variants", 2, 1), v084TestTag(t, "variant", "name", "First", 3, 2),
			v084TestMedia(t, "audio", `"a" extra`, "", true, 4, 3), v084TestTag(t, "variant", "name", "Second", 3, 2),
			v084TestText(t, "Read.", "Read.", 6, 3), v084TestClose(t, 3, 2),
		}, [][3]int{{5, 18, 23}}},
		{"inline editor", []model.Line{
			v084TestTag(t, "editor", "content", `@media audio "a" extra`, 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v084TestRun(t, v084TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			v084TestRun(t, v084TestDocument(t, v084TestOpen(t, tag, 2, 1),
				v084TestText(t, `@media audio "a" extra`, `@media audio "a" extra`, 3, 2), v084TestClose(t, 3, 2)), "literal", nil)
			v084TestRun(t, v084TestDocument(t, v084TestOpen(t, tag, 2, 1),
				v084TestText(t, `@media audio "a" extra`, `@media audio "a" extra`, 3, 2), v084TestClose(t, 3, 2),
				v084TestMedia(t, "audio", `"a" extra`, "", true, 2, 1)), "after", [][3]int{{6, 18, 23}})
		})
	}
	t.Run("literal block in note", func(t *testing.T) {
		v084TestRun(t, v084TestDocument(t, v084TestOpen(t, "note", 2, 1), v084TestOpen(t, "editor", 3, 2),
			v084TestText(t, `@media audio "a" extra`, `@media audio "a" extra`, 4, 3), v084TestClose(t, 4, 3),
			v084TestMedia(t, "audio", `"a" extra`, "", true, 3, 2), v084TestClose(t, 3, 2)), "current", [][3]int{{7, 18, 23}})
	})
	t.Run("unparsed literal block name", func(t *testing.T) {
		// Запрещённое размещение editor в note сохранено как блок с unparsed именем.
		opening := v084TestOpen(t, "editor", 3, 2)
		opening.Elements[0] = v084TestElement(t, "unparsed", "@editor", "", 1, 8)
		opening.Elements[0].Value = nil
		v084TestRun(t, v084TestDocument(t, v084TestOpen(t, "note", 2, 1), opening,
			v084TestText(t, `@media audio "a" extra`, `@media audio "a" extra`, 4, 3), v084TestClose(t, 4, 3), v084TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV084EndingsAndHistory(t *testing.T) {
	for _, name := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(name, func(t *testing.T) {
			result := v084TestDocument(t, v084TestMedia(t, "audio", `"track.mp3" extra`, "", true, 2, 1))
			for i := range result.Lines {
				switch name {
				case "LF":
					result.Lines[i].LineEnding = "\n"
				case "CRLF":
					result.Lines[i].LineEnding = "\r\n"
				case "mixed":
					if i%2 == 0 {
						result.Lines[i].LineEnding = "\r\n"
					} else {
						result.Lines[i].LineEnding = "\n"
					}
				}
			}
			v084TestRun(t, result, "first", [][3]int{{3, 26, 31}})
			v084TestRun(t, result, "second", [][3]int{{3, 26, 31}})
			v084TestRun(t, v084TestDocument(t, v084TestMedia(t, "audio", `"ok"`, "ok", false, 2, 1)), "separate", nil)
		})
	}
	t.Run("ordinary document", func(t *testing.T) {
		v084TestRun(t, v084TestDocument(t, v084TestText(t, "Read.", "Read.", 2, 1)), "current", nil)
	})
	t.Run("blank last line", func(t *testing.T) {
		result := v084TestDocument(t, v084TestLine(t, "", "blank", 2, 1))
		result.Lines[2].LineEnding = "\n"
		v084TestRun(t, result, "current", nil)
	})
}

func TestV084EmptyAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		for _, known := range []bool{false, true} {
			t.Run(version+" known count="+strconv.FormatBool(known), func(t *testing.T) {
				result := &model.Result{Format: model.FormatVersion1,
					Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
					Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
				if known {
					count := 0
					result.Document.LineCount = &count
				}
				if version == "1.2" {
					v084TestRun(t, result, "current", nil)
					return
				}
				before := v084TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v084TestRegistry(t))
				if err == nil || checks != nil || v084TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

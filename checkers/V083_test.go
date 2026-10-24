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

// v083TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V083.
func v083TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v083TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v083TestHeading(t, "", "#", "1", title),
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

// v083TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v083TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v083TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v083TestElement(t, "heading-level", marker, level, start, end),
		v083TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v083TestInvalid задаёт ошибочную строку без распознанного tag.
func v083TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v083TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v083TestLine(t, raw, "invalid", parent, depth, element)
}

// v083TestTag собирает однострочный тег с одним явно типизированным параметром.
func v083TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v083TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v083TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v083TestLine(t, raw, "tag", parent, depth, elements...)
}

// v083TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v083TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v083TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v083TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v083TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v083TestClose задаёт закрытие блока с явным родителем.
func v083TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v083TestLine(t, "}", "block-end", parent, depth, v083TestElement(t, "block-close", "}", "}", 1, 2))
}

// v083TestText сохраняет независимо заданные исходное и семантическое значения.
func v083TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v083TestLine(t, raw, "content", parent, depth,
		v083TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v083TestLine не классифицирует исходник и не вычисляет родительские связи.
func v083TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v083TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v083TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v083TestRegistry использует штатный каталог целиком.
func v083TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v083TestRun проверяет новые V083: строка, начало и исключающий конец.
func v083TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v083TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v083TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v083TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V083" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V083 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Заключённый в кавычки путь медиа не закрыт до конца строки." {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V083 metadata", i, entry, location)
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
	if err := json.Unmarshal([]byte(v083TestSnapshot(t, result)), &restored); err != nil {
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
	if v083TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v083TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v083TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v083TestMedia задаёт тип и SOURCE раздельно, не разбирая путь и экранирование.
// broken сохраняет SOURCE целиком как unparsed; значение задано независимо.
func v083TestMedia(t *testing.T, kind, source, value string, broken bool, parent, depth int) model.Line {
	t.Helper()
	end := 14 + utf8.RuneCountInString(source)
	element := v083TestElement(t, "source", source, value, 14, end)
	if broken {
		element = v083TestElement(t, "unparsed", source, "", 14, end)
		element.Value = nil
	}
	return v083TestLine(t, "@media "+kind+" "+source, "tag", parent, depth,
		v083TestElement(t, "tag", "@media", "media", 1, 7),
		v083TestElement(t, "media-type", kind, strings.ToLower(kind), 8, 13), element)
}

func TestV083Registration(t *testing.T) {
	definitions, err := v083TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V083" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Заключённый в кавычки путь медиа не закрыт до конца строки." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V083 definitions = %d, want 1", count)
	}
}

func TestV083ReferenceAndExample(t *testing.T) {
	// По DSL 1.3, 1.30, 2.1.3 фрагменты дополнены только версией и # Lesson.
	// Добавленные строки сдвигают фрагмент на строку 3, не меняя его содержимого.
	for _, tc := range []struct {
		name, fragment, source, value string
		broken                        bool
		want                          [][3]int
	}{
		{"reference", `@media audio "Audio/track.mp3"`, `"Audio/track.mp3"`, "Audio/track.mp3", false, nil},
		{"example", `@media audio "Audio/track.mp3`, `"Audio/track.mp3`, "", true, [][3]int{{3, 14, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v083TestDocument(t, v083TestMedia(t, "audio", tc.source, tc.value, tc.broken, 2, 1))
			if result.Lines[2].Raw != tc.fragment || result.Lines[2].LineEnding != "" {
				t.Fatal("fragment changed")
			}
			v083TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV083Escaping(t *testing.T) {
	for _, tc := range []struct {
		name, source, value string
		broken              bool
		want                [][3]int
	}{
		{"opening only", `"`, "", true, [][3]int{{3, 14, 15}}},
		{"empty closed path", `""`, "", true, nil},
		{"closed whitespace path", `"   "`, "", true, nil},
		{"unclosed whitespace path", `"   `, "", true, [][3]int{{3, 14, 15}}},
		{"escaped quote only", `"a\"b`, "", true, [][3]int{{3, 14, 15}}},
		{"escaped quote then closure", `"a\"b"`, `a"b`, false, nil},
		{"two backslashes", `"a\\"`, `a\`, false, nil},
		{"three backslashes", `"a\\\"`, "", true, [][3]int{{3, 14, 15}}},
		{"four backslashes", `"a\\\\"`, `a\\`, false, nil},
		{"five backslashes", `"a\\\\\"`, "", true, [][3]int{{3, 14, 15}}},
		{"six backslashes", `"a\\\\\\"`, `a\\\`, false, nil},
		{"trailing backslash", `"a\`, "", true, [][3]int{{3, 14, 15}}},
		{"trailing pair", `"a\\`, "", true, [][3]int{{3, 14, 15}}},
		{"backslash ordinary character", `"a\q"`, `a\q`, false, nil},
		{"pair followed by ordinary character", `"a\\q"`, `a\q`, false, nil},
		{"pair then ordinary then escaped quote", `"a\\q\"`, "", true, [][3]int{{3, 14, 15}}},
		{"multiple escaped quotes", `"a\"b\"c`, "", true, [][3]int{{3, 14, 15}}},
		{"later closure", `"a\"b\"c"`, `a"b"c`, false, nil},
		{"extra unclosed value", `"a" "unfinished`, "", true, nil},
		{"suffix after closure", `"a"tail\`, "", true, nil},
		{"trailing escaped quote and tab", "\"a\\\" \t", "", true, [][3]int{{3, 14, 15}}},
		{"quoted number", `"12"`, "12", false, nil},
		{"unquoted number", "12", "12", false, nil},
		{"unquoted path", "Audio/track.mp3", "Audio/track.mp3", false, nil},
		{"quote within unquoted source", `path"tail`, "", true, nil},
		{"escaped initial quote", `\"path`, `"path`, false, nil},
		{"Unicode unclosed", `"Аудио/😀`, "", true, [][3]int{{3, 14, 15}}},
		{"Unicode closed", `"Аудио/😀"`, `Аудио/😀`, false, nil},
		{"typographic quote does not close", `"path”`, "", true, [][3]int{{3, 14, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v083TestRun(t, v083TestDocument(t, v083TestMedia(t, "audio", tc.source, tc.value, tc.broken, 2, 1)), "current", tc.want)
		})
	}
}

func TestV083PrefixesAndUnparsed(t *testing.T) {
	for _, kind := range []string{"audio", "video", "image", "AUDIO", "Video", "IMAGE"} {
		t.Run(kind, func(t *testing.T) {
			v083TestRun(t, v083TestDocument(t, v083TestMedia(t, kind, `"path`, "", true, 2, 1)), "current", [][3]int{{3, 14, 15}})
		})
	}
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"entire declaration unparsed", `@media audio "path`, [][3]int{{3, 14, 15}}},
		{"all fields case insensitive", `@MeDiA AuDiO "path`, [][3]int{{3, 14, 15}}},
		{"indentation and spaces", " \t@MEDIA  VIDEO   \"path", [][3]int{{3, 19, 20}}},
		{"invalid type", `@media sound "path`, nil},
		{"longer type", `@media audioExtra "path`, nil},
		{"missing type", `@media "path`, nil},
		{"missing source", "@media audio", nil},
		{"only tag", "@media", nil},
		{"longer tag", `@mediaExtra audio "path`, nil},
		{"tag tab separator", "@media\taudio \"path", nil},
		{"type tab separator", "@media audio\t\"path", nil},
		{"mixed separator after tag", "@media \taudio \"path", nil},
		{"mixed separator after type", "@media audio \t\"path", nil},
		{"tag NBSP separator", "@media\u00a0audio \"path", nil},
		{"type NBSP separator", "@media audio\u00a0\"path", nil},
		{"no tag separator", `@mediaaudio "path`, nil},
		{"no type separator", `@media audio"path`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v083TestRun(t, v083TestDocument(t, v083TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	t.Run("invalid retains prefix", func(t *testing.T) {
		line := v083TestMedia(t, "audio", `"path`, "", true, 2, 1)
		line.LineType = "invalid"
		v083TestRun(t, v083TestDocument(t, line), "current", [][3]int{{3, 14, 15}})
	})
	t.Run("unparsed from type onwards", func(t *testing.T) {
		unparsed := v083TestElement(t, "unparsed", `audio "path`, "", 8, 19)
		unparsed.Value = nil
		line := v083TestLine(t, `@media audio "path`, "invalid", 2, 1,
			v083TestElement(t, "tag", "@media", "media", 1, 7), unparsed)
		v083TestRun(t, v083TestDocument(t, line), "current", [][3]int{{3, 14, 15}})
	})
	t.Run("raw overrides decoded source escaping", func(t *testing.T) {
		// Value содержит литеральную кавычку, Raw — экранированную. Конечная
		// исходная кавычка, а не кавычка Value, завершает путь.
		v083TestRun(t, v083TestDocument(t, v083TestMedia(t, "audio", `"a\"b"`, `a"b`, false, 2, 1)), "current", nil)
	})
	for _, tc := range []struct{ raw, value string }{
		{`\@media audio "path`, `@media audio "path`},
		{`Read @media audio "path`, `Read @media audio "path`},
		{"\u00a0@media audio \"path", "\u00a0@media audio \"path"},
		{"\u2003@media audio \"path", "\u2003@media audio \"path"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v083TestRun(t, v083TestDocument(t, v083TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
}

func TestV083PhysicalLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"next line cannot close", []model.Line{
			v083TestMedia(t, "audio", `"path`, "", true, 2, 1), v083TestText(t, `Read."`, `Read."`, 2, 1),
		}, [][3]int{{3, 14, 15}}},
		{"next media is independent", []model.Line{
			v083TestMedia(t, "audio", `"one`, "", true, 2, 1), v083TestMedia(t, "video", `"two"`, "two", false, 2, 1),
		}, [][3]int{{3, 14, 15}}},
		{"two unclosed sources", []model.Line{
			v083TestMedia(t, "audio", `"one`, "", true, 2, 1), v083TestMedia(t, "video", `"two`, "", true, 2, 1),
		}, [][3]int{{3, 14, 15}, {4, 14, 15}}},
		{"last source after closed", []model.Line{
			v083TestMedia(t, "audio", `"one"`, "one", false, 2, 1), v083TestMedia(t, "image", `"two`, "", true, 2, 1),
		}, [][3]int{{4, 14, 15}}},
		{"blank line separates declarations", []model.Line{
			v083TestMedia(t, "audio", `"one`, "", true, 2, 1), v083TestLine(t, " \t", "blank", 2, 1),
			v083TestMedia(t, "video", `"two`, "", true, 2, 1),
		}, [][3]int{{3, 14, 15}, {5, 14, 15}}},
		{"two media mentions on one line", []model.Line{
			// DSL 1.16: вторая запись — добавочный текст первого объявления.
			v083TestMedia(t, "audio", `"one" @media audio "two`, "", true, 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v083TestRun(t, v083TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV083Context(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"forbidden nesting still checked", []model.Line{
			v083TestOpen(t, "note", 2, 1), v083TestMedia(t, "audio", `"path`, "", true, 3, 2), v083TestClose(t, 3, 2),
		}, [][3]int{{4, 14, 15}}},
		{"multiple nested blocks", []model.Line{
			v083TestOpen(t, "note", 2, 1), v083TestOpen(t, "hint", 3, 2),
			v083TestMedia(t, "audio", `"path`, "", true, 4, 3), v083TestClose(t, 4, 3), v083TestClose(t, 3, 2),
		}, [][3]int{{5, 14, 15}}},
		{"fragment definition is not excluded", []model.Line{
			v083TestLine(t, "@fragment f {", "block-start", 2, 1,
				v083TestElement(t, "tag", "@fragment", "fragment", 1, 10), v083TestElement(t, "identifier", "f", "f", 11, 12),
				v083TestElement(t, "block-open", "{", "{", 13, 14)),
			v083TestMedia(t, "audio", `"path`, "", true, 3, 2), v083TestClose(t, 3, 2),
		}, [][3]int{{4, 14, 15}}},
		{"branch", []model.Line{
			v083TestOpen(t, "variants", 2, 1), v083TestTag(t, "variant", "name", "First", 3, 2),
			v083TestMedia(t, "audio", `"path`, "", true, 4, 3),
			v083TestTag(t, "variant", "name", "Second", 3, 2), v083TestText(t, "Read.", "Read.", 6, 3), v083TestClose(t, 3, 2),
		}, [][3]int{{5, 14, 15}}},
		{"inline editor is ordinary comment", []model.Line{
			v083TestTag(t, "editor", "content", `@media audio "path`, 2, 1),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v083TestRun(t, v083TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			v083TestRun(t, v083TestDocument(t, v083TestOpen(t, tag, 2, 1),
				v083TestText(t, `@media audio "path`, `@media audio "path`, 3, 2), v083TestClose(t, 3, 2)), "literal", nil)
			v083TestRun(t, v083TestDocument(t, v083TestOpen(t, tag, 2, 1),
				v083TestText(t, `@media audio "path`, `@media audio "path`, 3, 2), v083TestClose(t, 3, 2),
				v083TestMedia(t, "audio", `"path`, "", true, 2, 1)), "after", [][3]int{{6, 14, 15}})
			// Незакрытая кавычка в литеральном тексте не влияет на следующее media.
			v083TestRun(t, v083TestDocument(t, v083TestOpen(t, tag, 2, 1),
				v083TestText(t, `@media audio "path`, `@media audio "path`, 3, 2), v083TestClose(t, 3, 2),
				v083TestMedia(t, "audio", `"path"`, "path", false, 2, 1)), "closed after", nil)
		})
	}
	t.Run("literal block nested in note", func(t *testing.T) {
		v083TestRun(t, v083TestDocument(t, v083TestOpen(t, "note", 2, 1), v083TestOpen(t, "editor", 3, 2),
			v083TestText(t, `@media audio "path`, `@media audio "path`, 4, 3), v083TestClose(t, 4, 3),
			v083TestMedia(t, "audio", `"path`, "", true, 3, 2), v083TestClose(t, 3, 2)), "current", [][3]int{{7, 14, 15}})
	})
	t.Run("unparsed literal block declaration", func(t *testing.T) {
		// Запрещённый editor в note: скобка и блок сохранены, имя осталось unparsed.
		opening := v083TestOpen(t, "editor", 3, 2)
		opening.Elements[0] = v083TestElement(t, "unparsed", "@editor", "", 1, 8)
		opening.Elements[0].Value = nil
		v083TestRun(t, v083TestDocument(t, v083TestOpen(t, "note", 2, 1), opening,
			v083TestText(t, `@media audio "path`, `@media audio "path`, 4, 3), v083TestClose(t, 4, 3), v083TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV083EndingsAndHistory(t *testing.T) {
	for _, name := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(name, func(t *testing.T) {
			result := v083TestDocument(t, v083TestMedia(t, "audio", `"Аудио/😀`, "", true, 2, 1))
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
			v083TestRun(t, result, "first", [][3]int{{3, 14, 15}})
			v083TestRun(t, result, "second", [][3]int{{3, 14, 15}})
			v083TestRun(t, v083TestDocument(t, v083TestMedia(t, "audio", `"ok"`, "ok", false, 2, 1)), "separate", nil)
		})
	}
	t.Run("source with trailing whitespace", func(t *testing.T) {
		line := v083TestMedia(t, "audio", `"a"`, "a", false, 2, 1)
		line.Raw += " \t"
		v083TestRun(t, v083TestDocument(t, line), "current", nil)
	})
	t.Run("ordinary document", func(t *testing.T) {
		v083TestRun(t, v083TestDocument(t, v083TestText(t, "Read.", "Read.", 2, 1)), "current", nil)
	})
	t.Run("blank last line", func(t *testing.T) {
		result := v083TestDocument(t, v083TestLine(t, "", "blank", 2, 1))
		result.Lines[2].LineEnding = "\n"
		v083TestRun(t, result, "current", nil)
	})
}

func TestV083EmptyAndVersions(t *testing.T) {
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
					v083TestRun(t, result, "current", nil)
					return
				}
				before := v083TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v083TestRegistry(t))
				if err == nil || checks != nil || v083TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

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

// v108TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V108.
func v108TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v108TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v108TestHeading(t, "", "#", "1", title),
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

// v108TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v108TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v108TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v108TestElement(t, "heading-level", marker, level, start, end),
		v108TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v108TestTag собирает однострочный тег с одним явно типизированным параметром.
func v108TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v108TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v108TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v108TestLine(t, raw, "tag", parent, depth, elements...)
}

// v108TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v108TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v108TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v108TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v108TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v108TestClose задаёт закрытие блока с явным родителем.
func v108TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v108TestLine(t, "}", "block-end", parent, depth, v108TestElement(t, "block-close", "}", "}", 1, 2))
}

// v108TestText сохраняет независимо заданные исходное и семантическое значения.
func v108TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v108TestLine(t, raw, "content", parent, depth,
		v108TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v108TestLine не классифицирует исходник и не вычисляет родительские связи.
func v108TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v108TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v108TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v108TestRegistry использует штатный каталог целиком.
func v108TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v108TestRun проверяет новые V108: строка, начало и исключающий конец.
func v108TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v108TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v108TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v108TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V108" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V108 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Перед @script нет соседнего @media audio или @media video.\nконкретизация: Если это расшифровка записи, разместите её следующей значимой конструкцией после соответствующего @media audio или @media video" {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V108 metadata", i, entry, location)
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
	if err := json.Unmarshal([]byte(v108TestSnapshot(t, result)), &restored); err != nil {
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
	if v108TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v108TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v108TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV108Registration(t *testing.T) {
	definitions, err := v108TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V108" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Перед @script нет соседнего @media audio или @media video." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V108 definitions = %d, want 1", count)
	}
}

func TestV108EmptyAndVersions(t *testing.T) {
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
					v108TestRun(t, result, "current", nil)
					return
				}
				before := v108TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v108TestRegistry(t))
				if err == nil || checks != nil || v108TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

// v108TestMedia задаёт два отдельных элемента объявления медиа.
// Raw источника и его семантическое значение передаются независимо.
func v108TestMedia(t *testing.T, tag, kind, raw, value string, parent, depth int) model.Line {
	t.Helper()
	start := len(tag) + len(kind) + 4
	return v108TestLine(t, "@"+tag+" "+kind+" "+raw, "tag", parent, depth,
		v108TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v108TestElement(t, "media-type", kind, strings.ToLower(kind), len(tag)+3, len(tag)+3+len(kind)),
		v108TestElement(t, "source", raw, value, start, start+utf8.RuneCountInString(raw)))
}

// v108TestScript задаёт полный блок с явными номером объявления и родителем.
func v108TestScript(t *testing.T, number, parent, depth int) []model.Line {
	t.Helper()
	return []model.Line{v108TestOpen(t, "script", parent, depth),
		v108TestText(t, "Hello!", "Hello!", number, depth+1), v108TestClose(t, number, depth+1)}
}

// v108TestFragment задаёт обязательный ID определения, а не название блока.
func v108TestFragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v108TestLine(t, "@fragment transcript {", "block-start", parent, depth,
		v108TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v108TestElement(t, "identifier", "transcript", "transcript", 11, 21),
		v108TestElement(t, "block-open", "{", "{", 22, 23))
}

// v108TestUnparsed создаёт ошибочный участок; ссылки добавляет TestDocument.
func v108TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v108TestElement(t, "unparsed", raw, "", start, start+utf8.RuneCountInString(raw))
	element.Value = nil
	return element
}

func TestV108ReferenceAndExample(t *testing.T) {
	// Перед исходными примерами добавлены только обязательные версия и заголовок.
	for _, tc := range []struct {
		name   string
		middle model.Line
		want   [][3]int
	}{
		{"reference", v108TestLine(t, "", "blank", 2, 1), nil},
		{"example", v108TestTag(t, "note", "content", "Listen carefully.", 2, 1), [][3]int{{5, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1), tc.middle}
			body = append(body, v108TestScript(t, 5, 2, 1)...)
			v108TestRun(t, v108TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV108MediaKindsAndSources(t *testing.T) {
	for _, tc := range []struct {
		name  string
		media model.Line
		want  [][3]int
	}{
		{"audio", v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1), nil},
		{"video", v108TestMedia(t, "media", "video", "track.mp4", "track.mp4", 2, 1), nil},
		{"case", v108TestMedia(t, "MeDiA", "AuDiO", "track.mp3", "track.mp3", 2, 1), nil},
		{"number", v108TestMedia(t, "media", "audio", "001", "001", 2, 1), nil},
		{"quoted Unicode", v108TestMedia(t, "media", "video", `"запись урока.mp4"`, "запись урока.mp4", 2, 1), nil},
		{"image", v108TestMedia(t, "media", "image", "picture.png", "picture.png", 2, 1), [][3]int{{4, 1, 8}}},
		{"empty semantic source", v108TestMedia(t, "media", "audio", `""`, "", 2, 1), [][3]int{{4, 1, 8}}},
		{"missing source", v108TestTag(t, "media", "media-type", "audio", 2, 1), [][3]int{{4, 1, 8}}},
		{"unparsed type", v108TestLine(t, "@media unknown file", "tag", 2, 1,
			v108TestElement(t, "tag", "@media", "media", 1, 7), v108TestUnparsed(t, "unknown file", 8)), [][3]int{{4, 1, 8}}},
		{"known fields with unparsed tail", v108TestLine(t, `@media audio "track.mp3" extra`, "tag", 2, 1,
			v108TestElement(t, "tag", "@media", "media", 1, 7), v108TestElement(t, "media-type", "audio", "audio", 8, 13),
			v108TestElement(t, "source", `"track.mp3"`, "track.mp3", 14, 25), v108TestUnparsed(t, "extra", 26)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{tc.media}, v108TestScript(t, 4, 2, 1)...)
			body[1] = v108TestOpen(t, "ScRiPt", 2, 1)
			v108TestRun(t, v108TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV108WhitespaceAndBarriers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		middle model.Line
		want   [][3]int
	}{
		{"ASCII whitespace", v108TestLine(t, " \t  \t", "blank", 2, 1), nil},
		{"NBSP", v108TestText(t, "\u00a0", "\u00a0", 2, 1), [][3]int{{5, 1, 8}}},
		{"Unicode whitespace", v108TestText(t, "\u2003", "\u2003", 2, 1), [][3]int{{5, 1, 8}}},
		{"text", v108TestText(t, "Listen.", "Listen.", 2, 1), [][3]int{{5, 1, 8}}},
		{"escaped editor", v108TestText(t, `\@editor Note.`, "@editor Note.", 2, 1), [][3]int{{5, 1, 8}}},
		{"editor mention", v108TestText(t, "See @editor Note.", "See @editor Note.", 2, 1), [][3]int{{5, 1, 8}}},
		{"heading", v108TestHeading(t, "", "###", "3", "Next"), [][3]int{{5, 1, 8}}},
		{"header", v108TestTag(t, "header", "title", "Next", 2, 1), [][3]int{{5, 1, 8}}},
		{"speaking", v108TestTag(t, "speaking", "", "", 2, 1), [][3]int{{5, 1, 8}}},
		{"newpage", v108TestTag(t, "newpage", "", "", 0, 0), [][3]int{{5, 1, 8}}},
		{"inline editor", v108TestTag(t, "EDITOR", "content", "Check.", 2, 1), nil},
		{"uninterpreted error", v108TestLine(t, "@unknown", "invalid", 2, 1,
			v108TestUnparsed(t, "@unknown", 1)), [][3]int{{5, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1), tc.middle}
			parent, depth := 2, 1
			if tc.name == "heading" || tc.name == "newpage" {
				parent, depth = 4, 1
			}
			body = append(body, v108TestScript(t, 5, parent, depth)...)
			v108TestRun(t, v108TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV108BlockCommentsAndOtherBlocks(t *testing.T) {
	for _, tag := range []string{"editor", "note", "text", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			body := []model.Line{v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1),
				v108TestOpen(t, tag, 2, 1), v108TestText(t, "Listen.", "Listen.", 4, 2), v108TestClose(t, 4, 2)}
			body = append(body, v108TestScript(t, 7, 2, 1)...)
			want := [][3]int{{7, 1, 8}}
			if tag == "editor" {
				want = nil
			}
			v108TestRun(t, v108TestDocument(t, body...), "current", want)
		})
	}
	t.Run("chain and literal content", func(t *testing.T) {
		body := []model.Line{v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1),
			v108TestTag(t, "editor", "content", "First.", 2, 1), v108TestLine(t, "\t", "blank", 2, 1),
			v108TestOpen(t, "EDITOR", 2, 1), v108TestText(t, "@script {", "@script {", 6, 2),
			v108TestText(t, "@variant Other", "@variant Other", 6, 2), v108TestClose(t, 6, 2),
			v108TestLine(t, "", "blank", 2, 1), v108TestOpen(t, "editor", 2, 1),
			v108TestText(t, "Second.", "Second.", 11, 2), v108TestClose(t, 11, 2)}
		body = append(body, v108TestScript(t, 14, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", nil)
	})
}

func TestV108FragmentsAndIncludes(t *testing.T) {
	for _, comment := range []string{"none", "inline", "block"} {
		t.Run("fragment "+comment, func(t *testing.T) {
			body := []model.Line{v108TestFragment(t, 2, 1), v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 3, 2)}
			scriptLine := 5
			var want [][3]int
			switch comment {
			case "inline":
				body = append(body, v108TestTag(t, "editor", "content", "Check.", 3, 2))
				scriptLine, want = 6, [][3]int{{6, 1, 8}}
			case "block":
				body = append(body, v108TestOpen(t, "editor", 3, 2), v108TestText(t, "Check.", "Check.", 5, 3), v108TestClose(t, 5, 3))
				scriptLine, want = 8, [][3]int{{8, 1, 8}}
			}
			body = append(body, v108TestScript(t, scriptLine, 3, 2)...)
			body = append(body, v108TestClose(t, 3, 2))
			v108TestRun(t, v108TestDocument(t, body...), "current", want)
		})
	}
	t.Run("outside media and repeated includes", func(t *testing.T) {
		body := []model.Line{v108TestMedia(t, "media", "audio", "outside.mp3", "outside.mp3", 2, 1), v108TestFragment(t, 2, 1)}
		body = append(body, v108TestScript(t, 5, 4, 2)...)
		body = append(body, v108TestClose(t, 4, 2),
			v108TestTag(t, "include", "identifier", "transcript", 2, 1), v108TestTag(t, "include", "identifier", "transcript", 2, 1))
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{5, 1, 8}})
	})
	t.Run("include remains significant", func(t *testing.T) {
		body := []model.Line{v108TestFragment(t, 2, 1), v108TestText(t, "Read.", "Read.", 3, 2), v108TestClose(t, 3, 2),
			v108TestMedia(t, "media", "audio", "outside.mp3", "outside.mp3", 2, 1), v108TestTag(t, "include", "identifier", "transcript", 2, 1)}
		body = append(body, v108TestScript(t, 8, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{8, 1, 8}})
	})
	t.Run("media in fragment cannot fill outer script", func(t *testing.T) {
		body := []model.Line{v108TestFragment(t, 2, 1), v108TestMedia(t, "media", "audio", "inside.mp3", "inside.mp3", 3, 2), v108TestClose(t, 3, 2)}
		body = append(body, v108TestScript(t, 6, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{6, 1, 8}})
	})
}

func TestV108TaskAndStepBoundaries(t *testing.T) {
	t.Run("comments within task stages", func(t *testing.T) {
		body := []model.Line{v108TestTag(t, "task", "identifier", "practice", 2, 1),
			v108TestTag(t, "step", "title", "Listen", 3, 2), v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 4, 3),
			v108TestTag(t, "editor", "content", "Check.", 4, 3), v108TestOpen(t, "editor", 4, 3),
			v108TestText(t, "Check again.", "Check again.", 7, 4), v108TestClose(t, 7, 4)}
		body = append(body, v108TestScript(t, 10, 4, 3)...)
		body = append(body, v108TestTag(t, "step", "title", "Write", 3, 2), v108TestText(t, "Answer.", "Answer.", 13, 3),
			v108TestTag(t, "endtask", "", "", 3, 2))
		v108TestRun(t, v108TestDocument(t, body...), "current", nil)
	})
	t.Run("task starts between media and script", func(t *testing.T) {
		body := []model.Line{v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1),
			v108TestTag(t, "task", "identifier", "practice", 2, 1)}
		body = append(body, v108TestScript(t, 5, 4, 2)...)
		body = append(body, v108TestTag(t, "step", "title", "Read", 4, 2), v108TestTag(t, "step", "title", "Write", 4, 2),
			v108TestTag(t, "endtask", "", "", 4, 2))
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{5, 1, 8}})
	})
	t.Run("step boundary", func(t *testing.T) {
		body := []model.Line{v108TestTag(t, "task", "identifier", "practice", 2, 1),
			v108TestTag(t, "step", "title", "Listen", 3, 2), v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 4, 3),
			v108TestTag(t, "step", "title", "Read", 3, 2)}
		body = append(body, v108TestScript(t, 7, 6, 3)...)
		body = append(body, v108TestTag(t, "endtask", "", "", 3, 2))
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{7, 1, 8}})
	})
	t.Run("task end", func(t *testing.T) {
		body := []model.Line{v108TestTag(t, "task", "identifier", "practice", 2, 1),
			v108TestTag(t, "step", "title", "Read", 3, 2), v108TestText(t, "Read.", "Read.", 4, 3),
			v108TestTag(t, "step", "title", "Listen", 3, 2), v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 6, 3),
			v108TestTag(t, "endtask", "", "", 3, 2)}
		body = append(body, v108TestScript(t, 9, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{9, 1, 8}})
	})
}

func TestV108Variants(t *testing.T) {
	// Контракт допускает логического родителя variant либо общий variants.
	// Физическая строка следующей ветви должна остановить поиск в обоих случаях.
	for _, commonParent := range []bool{false, true} {
		t.Run("common parent="+strconv.FormatBool(commonParent), func(t *testing.T) {
			parentA, parentB, depth := 4, 12, 3
			if commonParent {
				parentA, parentB, depth = 3, 3, 2
			}
			body := []model.Line{v108TestOpen(t, "variants", 2, 1), v108TestTag(t, "variant", "name", "A", 3, 2),
				v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", parentA, depth), v108TestOpen(t, "editor", parentA, depth),
				v108TestText(t, "Check.", "Check.", 6, depth+1), v108TestClose(t, 6, depth+1)}
			body = append(body, v108TestScript(t, 9, parentA, depth)...)
			body = append(body, v108TestTag(t, "variant", "name", "B", 3, 2))
			body = append(body, v108TestScript(t, 13, parentB, depth)...)
			body = append(body, v108TestClose(t, 3, 2))
			v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{13, 1, 8}})
		})
		t.Run("media in prior branch common parent="+strconv.FormatBool(commonParent), func(t *testing.T) {
			parentA, parentB, depth := 4, 6, 3
			if commonParent {
				parentA, parentB, depth = 3, 3, 2
			}
			body := []model.Line{v108TestOpen(t, "variants", 2, 1), v108TestTag(t, "variant", "name", "A", 3, 2),
				v108TestMedia(t, "media", "video", "track.mp4", "track.mp4", parentA, depth),
				v108TestTag(t, "variant", "name", "B", 3, 2), v108TestTag(t, "editor", "content", "Check.", parentB, depth)}
			body = append(body, v108TestScript(t, 8, parentB, depth)...)
			body = append(body, v108TestClose(t, 3, 2))
			v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{8, 1, 8}})
		})
	}
	t.Run("closing variants", func(t *testing.T) {
		body := []model.Line{v108TestOpen(t, "variants", 2, 1), v108TestTag(t, "variant", "name", "A", 3, 2),
			v108TestText(t, "Read.", "Read.", 4, 3), v108TestTag(t, "variant", "name", "B", 3, 2),
			v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 6, 3), v108TestClose(t, 3, 2)}
		body = append(body, v108TestScript(t, 9, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{9, 1, 8}})
	})
}

func TestV108ExcludedAndPartialDeclarations(t *testing.T) {
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v108TestRun(t, v108TestDocument(t, v108TestOpen(t, tag, 2, 1),
				v108TestText(t, "@script {", "@script {", 3, 2), v108TestText(t, "Hello!", "Hello!", 3, 2),
				v108TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("inline error and escaped script", func(t *testing.T) {
		v108TestRun(t, v108TestDocument(t, v108TestLine(t, "@script Hello!", "invalid", 2, 1,
			v108TestElement(t, "tag", "@script", "script", 1, 8), v108TestUnparsed(t, "Hello!", 9)),
			v108TestText(t, `\@script {`, "@script {", 2, 1)), "current", nil)
	})
	t.Run("known block with erroneous tail", func(t *testing.T) {
		v108TestRun(t, v108TestDocument(t, v108TestLine(t, "@script { extra", "block-start", 2, 1,
			v108TestElement(t, "tag", "@script", "script", 1, 8), v108TestElement(t, "block-open", "{", "{", 9, 10),
			v108TestUnparsed(t, "extra", 11)), v108TestText(t, "Hello!", "Hello!", 3, 2), v108TestClose(t, 3, 2)),
			"current", [][3]int{{3, 1, 8}})
	})
	t.Run("media only inside skipped comment", func(t *testing.T) {
		body := []model.Line{v108TestOpen(t, "editor", 2, 1),
			v108TestText(t, "@media audio track.mp3", "@media audio track.mp3", 3, 2), v108TestClose(t, 3, 2)}
		body = append(body, v108TestScript(t, 6, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{6, 1, 8}})
	})
}

func TestV108RangesEndingsAndIndependentScripts(t *testing.T) {
	t.Run("consecutive scripts", func(t *testing.T) {
		body := []model.Line{v108TestMedia(t, "media", "audio", "track.mp3", "track.mp3", 2, 1)}
		body = append(body, v108TestScript(t, 4, 2, 1)...)
		body = append(body, v108TestScript(t, 7, 2, 1)...)
		v108TestRun(t, v108TestDocument(t, body...), "current", [][3]int{{7, 1, 8}})
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			body := []model.Line{v108TestLine(t, " \t@script Расшифровка {", "block-start", 2, 1,
				v108TestElement(t, "tag", "@script", "script", 3, 10),
				v108TestElement(t, "title", "Расшифровка", "Расшифровка", 11, 22), v108TestElement(t, "block-open", "{", "{", 23, 24)),
				v108TestText(t, "Привет!", "Привет!", 3, 2), v108TestClose(t, 3, 2)}
			body = append(body, v108TestScript(t, 6, 2, 1)...)
			result := v108TestDocument(t, body...)
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v108TestRun(t, result, "first", [][3]int{{3, 3, 10}, {6, 1, 8}})
			v108TestRun(t, result, "second", [][3]int{{3, 3, 10}, {6, 1, 8}})
		})
	}
	t.Run("last line opening", func(t *testing.T) {
		v108TestRun(t, v108TestDocument(t, v108TestOpen(t, "script", 2, 1)), "current", [][3]int{{3, 1, 8}})
	})
	t.Run("first physical line", func(t *testing.T) {
		result := v108TestDocument(t, v108TestScript(t, 1, 0, 0)...)
		result.Lines = result.Lines[2:]
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		count := 3
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		// Модель неполного документа: отсутствие обязательного префикса проверяют
		// другие диагностики. V108 должна обработать начало физического потока.
		v108TestRun(t, result, "current", [][3]int{{1, 1, 8}})
	})
}

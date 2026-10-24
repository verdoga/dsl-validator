package checkers

import (
	"encoding/json"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV079Registration(t *testing.T) {
	definitions, err := v079Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V079" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Тип ресурса отличается от audio, video и image." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V079 definitions = %d, want 1", count)
	}
}

func TestV079ReferenceAndExample(t *testing.T) {
	// Дословные фрагменты дополнены только обязательными @dsl-version и
	// # Lesson. Их содержимое не меняется; проверяемая строка получает номер 3.
	for _, tc := range []struct {
		name, raw string
		line      model.Line
		want      [][3]int
	}{
		{"reference", "@media video film.mp4", v079Media(t, "video", "video", "film.mp4", 2, 1), nil},
		{"example", "@media document manual.pdf", v079Media(t, "document", "", "manual.pdf", 2, 1), [][3]int{{3, 8, 16}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.line.Raw != tc.raw {
				t.Fatal("normative example changed")
			}
			v079Run(t, v079Lesson(t, tc.line), "current", tc.want)
		})
	}
}

func TestV079AllowedTypes(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"audio", "audio"}, {"video", "video"}, {"image", "image"},
		{"AUDIO", "audio"}, {"ViDeO", "video"}, {"iMaGe", "image"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v079Run(t, v079Lesson(t, v079Media(t, tc.raw, tc.value, "file.dat", 2, 1)), "current", nil)
		})
	}
}

func TestV079RawArgument(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		tagStart  int
		want      [][3]int
	}{
		{"absent", "@media", 1, nil},
		{"spaces only", "@media   ", 1, nil},
		{"tabs only", "@media\t\t", 1, nil},
		{"mixed whitespace only", "@media \t ", 1, nil},
		{"invalid type without source", "@media pdf", 1, [][3]int{{3, 8, 11}}},
		{"valid type without source", "@media audio", 1, nil},
		{"source is not checked", "@media audio \"unfinished", 1, nil},
		{"quoted type", "@media \"audio\" x", 1, [][3]int{{3, 8, 15}}},
		{"empty quotes are an argument", "@media \"\" x", 1, [][3]int{{3, 8, 10}}},
		{"space inside quoted type ends argument", "@media \"audio video\" x", 1, [][3]int{{3, 8, 14}}},
		{"tab delimiter", "@media\tpdf x", 1, [][3]int{{3, 8, 11}}},
		{"tab ends type", "@media pdf\tx", 1, [][3]int{{3, 8, 11}}},
		{"tabs with allowed type", "@media\tAuDiO\tx", 1, nil},
		{"indentation and mixed case tag", "\t @MeDiA   pdf x", 3, [][3]int{{3, 12, 15}}},
		{"Unicode space in argument", "@media audio\u00a0x", 1, [][3]int{{3, 8, 15}}},
		{"NBSP is an argument", "@media \u00a0", 1, [][3]int{{3, 8, 9}}},
		{"Unicode type", "@media документ x", 1, [][3]int{{3, 8, 16}}},
		{"supplementary character", "@media 😀 x", 1, [][3]int{{3, 8, 9}}},
		{"no decoding", "@media a\\udio x", 1, [][3]int{{3, 8, 14}}},
		{"exact allowed word", "@media audioExtra x", 1, [][3]int{{3, 8, 18}}},
		{"brace is an argument", "@media { x", 1, [][3]int{{3, 8, 9}}},
		{"source cannot replace type", "@media film.mp4", 1, [][3]int{{3, 8, 16}}},
	} {
		for _, kind := range []model.LineType{"tag", "invalid"} {
			t.Run(tc.name+"/"+string(kind), func(t *testing.T) {
				line := v079Attempt(t, tc.raw, tc.tagStart, 2, 1)
				line.LineType = kind
				v079Run(t, v079Lesson(t, line), "current", tc.want)
			})
		}
	}
	t.Run("parsed type with unparsed source", func(t *testing.T) {
		line := v079Line(t, "@media audio \"unfinished", "invalid", 2, 1,
			v079Element(t, "tag", "@media", "media", 1, 7),
			v079Element(t, "media-type", "audio", "audio", 8, 13),
			v079Element(t, "unparsed", " \"unfinished", "", 13, 25))
		v079Run(t, v079Lesson(t, line), "current", nil)
	})
}

func TestV079Contexts(t *testing.T) {
	t.Run("nested misplaced declaration", func(t *testing.T) {
		fragment := v079Line(t, "@fragment f {", "block-start", 2, 1,
			v079Element(t, "tag", "@fragment", "fragment", 1, 10),
			v079Element(t, "identifier", "f", "f", 11, 12), v079Element(t, "block-open", "{", "{", 13, 14))
		// Media в instr запрещён, но это реальное объявление, а не литеральный текст.
		v079Run(t, v079Lesson(t, fragment, v079Open(t, "instr", 3, 2),
			v079Media(t, "document", "", "x", 4, 3), v079Close(t, 4, 3), v079Close(t, 3, 2)),
			"current", [][3]int{{5, 8, 16}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			body := []model.Line{v079Open(t, name, 2, 1),
				v079Line(t, "@media document x", "content", 3, 2,
					v079Element(t, "content", "@media document x", "@media document x", 1, 18)),
				v079Close(t, 3, 2)}
			v079Run(t, v079Lesson(t, body...), "current", nil)
			v079Run(t, v079Lesson(t, append(body, v079Media(t, "document", "", "x", 2, 1))...),
				"current", [][3]int{{6, 8, 16}})
		})
	}
	t.Run("escaped and inline pseudo tags", func(t *testing.T) {
		v079Run(t, v079Lesson(t,
			v079Line(t, "\\@media document x", "content", 2, 1,
				v079Element(t, "content", "\\@media document x", "@media document x", 1, 19)),
			v079Line(t, "Use @media document x", "content", 2, 1,
				v079Element(t, "content", "Use @media document x", "Use @media document x", 1, 22))), "current", nil)
	})
	t.Run("another real tag is not media", func(t *testing.T) {
		v079Run(t, v079Lesson(t, v079Line(t, "@instr document", "tag", 2, 1,
			v079Element(t, "tag", "@instr", "instr", 1, 7),
			v079Element(t, "content", "document", "document", 8, 16))), "current", nil)
	})
	t.Run("first and last declarations counted once", func(t *testing.T) {
		v079Run(t, v079Lesson(t, v079Media(t, "pdf", "", "x", 2, 1),
			v079Media(t, "document", "", "y", 2, 1)), "current", [][3]int{{3, 8, 11}, {4, 8, 16}})
	})
}

func TestV079EndingsVersionsAndRuns(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v079Lesson(t, v079Media(t, "document", "", "manual.pdf", 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[2].LineEnding = ""
				}
				v079Run(t, result, "current", [][3]int{{3, 8, 16}})
			})
		}
	}
	t.Run("empty input", func(t *testing.T) {
		result := v079Lesson(t)
		result.Lines = []model.Line{}
		result.Document.LineCount, result.Document.Metadata.Title = nil, nil
		v079Run(t, result, "current", nil)
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v079Lesson(t, v079Media(t, "document", "", "manual.pdf", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v079Snapshot(t, result)
			checks, err := validator.Prepare(result, v079Registry(t))
			if err == nil || checks != nil || before != v079Snapshot(t, result) {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
	first := v079Lesson(t, v079Media(t, "document", "", "manual.pdf", 2, 1))
	second := v079Lesson(t, v079Media(t, "video", "video", "film.mp4", 2, 1))
	v079Run(t, first, "first", [][3]int{{3, 8, 16}})
	v079Run(t, first, "second", [][3]int{{3, 8, 16}})
	v079Run(t, second, "first", nil)
}

// v079Media задаёт поля модели из явно заданных частей; пустой canonical
// означает неизвестный тип с null Value и исходной ошибкой парсера.
func v079Media(t *testing.T, kind, canonical, source string, parent, depth int) model.Line {
	t.Helper()
	end := 8 + utf8.RuneCountInString(kind)
	typeElement := v079Element(t, "media-type", kind, canonical, 8, end)
	if canonical == "" {
		typeElement.Value = nil
	}
	return v079Line(t, "@media "+kind+" "+source, "tag", parent, depth,
		v079Element(t, "tag", "@media", "media", 1, 7), typeElement,
		v079Element(t, "source", source, source, end+1, end+1+utf8.RuneCountInString(source)))
}

// v079Attempt задаёт разобранный tag и ошибочный остаток в unparsed.
func v079Attempt(t *testing.T, raw string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + 6
	return v079Line(t, raw, "invalid", parent, depth,
		v079Element(t, "tag", string(runes[start-1:end-1]), "media", start, end),
		v079Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v079Lesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для unparsed и неизвестного media-type.
func v079Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v079Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v079Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v079Element(t, "version", "1.2", "1.2", 14, 17)),
		v079Line(t, "# Lesson", "heading", 0, 0,
			v079Element(t, "heading-level", "#", "1", 1, 2), v079Element(t, "title", "Lesson", "Lesson", 3, 9)),
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
			if element.ElementType != "unparsed" && !(element.ElementType == "media-type" && element.Value == nil) {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v079Open задаёт реальное объявление блока с явным родителем.
func v079Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v079Line(t, "@"+name+" {", "block-start", parent, depth,
		v079Element(t, "tag", "@"+name, name, 1, len(name)+2),
		v079Element(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v079Close задаёт закрытие указанного блока.
func v079Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v079Line(t, "}", "block-end", parent, depth, v079Element(t, "block-close", "}", "}", 1, 2))
}

// v079Line сохраняет явно заданную структуру, не классифицируя исходный текст.
func v079Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v079Element задаёт исходные Unicode-колонки с исключающим концом.
func v079Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v079Registry использует весь штатный каталог регистраций.
func v079Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v079Run проверяет только новые V079 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v079Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v079Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v079Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v079Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V079" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V079 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Тип ресурса отличается от audio, video и image.\nконкретизация: Допустимые значения: audio, video, image." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V079 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v079Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Производные флаги и ссылки проверены CheckModel. В копии возвращаем
	// прежние значения этих полей и удаляем только новые записи запуска.
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
	if v079Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v079Snapshot сохраняет независимое представление модели.
func v079Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

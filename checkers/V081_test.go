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

func TestV081Registration(t *testing.T) {
	definitions, err := v081Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V081" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Источник медиа записан несколькими частями без общих кавычек." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V081 definitions = %d, want 1", count)
	}
}

func TestV081ReferenceAndExample(t *testing.T) {
	// К дословным фрагментам добавлены только @dsl-version 1.2 и # Lesson.
	reference := v081Line(t, "@media audio \"Audio files/track 1.mp3\"", "tag", 2, 1,
		v081Element(t, "tag", "@media", "media", 1, 7),
		v081Element(t, "media-type", "audio", "audio", 8, 13),
		v081Element(t, "source", "\"Audio files/track 1.mp3\"", "Audio files/track 1.mp3", 14, 39))
	v081Run(t, v081Lesson(t, reference), "current", nil)
	v081Run(t, v081Lesson(t, v081Attempt(t, "@media audio Audio files/track 1.mp3", 1, 2, 1)),
		"current", [][3]int{{3, 14, 37}})
}

func TestV081SplitSourcesAndPositions(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		tagStart  int
		want      [][3]int
	}{
		{"two parts", "@media audio a b", 1, [][3]int{{3, 14, 17}}},
		{"three parts give one message", "@media audio a b c", 1, [][3]int{{3, 14, 19}}},
		{"separator run gives one message", "@media audio a \t b", 1, [][3]int{{3, 14, 19}}},
		{"video", "@media video a b", 1, [][3]int{{3, 14, 17}}},
		{"image", "@media image a b", 1, [][3]int{{3, 14, 17}}},
		{"case insensitive", "@MeDiA AuDiO a b", 1, [][3]int{{3, 14, 17}}},
		{"video case", "@MEDIA ViDeO a b", 1, [][3]int{{3, 14, 17}}},
		{"image case", "@media IMAGE a b", 1, [][3]int{{3, 14, 17}}},
		{"tab separators", "@media\taudio\ta\tb", 1, [][3]int{{3, 14, 17}}},
		{"trim only outer ASCII whitespace", "@media audio \t a b \t", 1, [][3]int{{3, 16, 19}}},
		{"trailing ASCII whitespace", "@media audio a b \t", 1, [][3]int{{3, 14, 17}}},
		{"indentation", "\t @media audio a b", 3, [][3]int{{3, 16, 19}}},
		{"Unicode edges retained", "@media audio \u00a0a b\u00a0", 1, [][3]int{{3, 14, 19}}},
		{"inner quotes do not group", "@media audio a \"b c\"", 1, [][3]int{{3, 14, 21}}},
		{"no decoding", "@media audio a\\ b", 1, [][3]int{{3, 14, 18}}},
		{"Unicode columns", "@media audio путь 😀", 1, [][3]int{{3, 14, 20}}},
		{"NBSP does not expose starting quote", "@media audio \u00a0\"a b\"", 1, [][3]int{{3, 14, 20}}},
		{"invalid type", "@media document a b", 1, nil},
		{"quoted type", "@media \"audio\" a b", 1, nil},
		{"longer type", "@media audioExtra a b", 1, nil},
		{"absent type", "@media", 1, nil},
		{"absent source", "@media audio", 1, nil},
		{"blank source", "@media audio \t ", 1, nil},
		{"empty quoted source", "@media audio \"\"", 1, nil},
		{"quote after leading ASCII whitespace", "@media audio \t \"a b\"", 1, nil},
		{"unclosed quote", "@media audio \"unfinished path", 1, nil},
		{"extra text after quoted source", "@media audio \"a\" extra", 1, nil},
	} {
		for _, kind := range []model.LineType{"tag", "invalid"} {
			t.Run(tc.name+"/"+string(kind), func(t *testing.T) {
				// Синтаксическая ошибка сохранена в unparsed; tag уже известен.
				line := v081Attempt(t, tc.raw, tc.tagStart, 2, 1)
				line.LineType = kind
				v081Run(t, v081Lesson(t, line), "current", tc.want)
			})
		}
	}
}

func TestV081UnicodeWhitespaceAndSingleParts(t *testing.T) {
	// Множество выписано явно, независимо от unicode.White_Space реализации.
	// CR/LF отсутствуют в Raw по контракту, окончания строк проверяются отдельно.
	for _, space := range []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(strconv.QuoteRune(space), func(t *testing.T) {
			v081Run(t, v081Lesson(t, v081Attempt(t, "@media audio a"+string(space)+"b", 1, 2, 1)),
				"current", [][3]int{{3, 14, 17}})
		})
	}
	for _, source := range []string{"track.mp3", "0012", "a\u200bb", "a\ufeffb", "\u00a0a", "a\u00a0", "\u00a0a\u00a0", "\u00a0", "\u00a0\u2003"} {
		t.Run("no split/"+strconv.Quote(source), func(t *testing.T) {
			// V081 исследует Raw независимо от семантического source.
			v081Run(t, v081Lesson(t, v081Media(t, "audio", "audio", source, 2, 1)), "current", nil)
		})
	}
}

func TestV081ParsedFieldsAndUnparsed(t *testing.T) {
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run("parsed first part/"+string(kind), func(t *testing.T) {
			line := v081Line(t, "@media audio a b", kind, 2, 1,
				v081Element(t, "tag", "@media", "media", 1, 7),
				v081Element(t, "media-type", "audio", "audio", 8, 13),
				v081Element(t, "source", "a", "a", 14, 15),
				v081Element(t, "unparsed", " b", "", 15, 17))
			v081Run(t, v081Lesson(t, line), "current", [][3]int{{3, 14, 17}})
		})
	}
	t.Run("parsed type and unparsed source", func(t *testing.T) {
		line := v081Line(t, "@media audio a b", "invalid", 2, 1,
			v081Element(t, "tag", "@media", "media", 1, 7),
			v081Element(t, "media-type", "audio", "audio", 8, 13),
			v081Element(t, "unparsed", " a b", "", 13, 17))
		v081Run(t, v081Lesson(t, line), "current", [][3]int{{3, 14, 17}})
	})
	t.Run("unknown parsed type is excluded", func(t *testing.T) {
		line := v081Line(t, "@media document a b", "invalid", 2, 1,
			v081Element(t, "tag", "@media", "media", 1, 7),
			v081Element(t, "media-type", "document", "", 8, 16),
			v081Element(t, "unparsed", " a b", "", 16, 20))
		line.Elements[1].Value = nil
		v081Run(t, v081Lesson(t, line), "current", nil)
	})
	t.Run("quoted source with spaces in semantic value", func(t *testing.T) {
		line := v081Line(t, "@media audio \"a b\"", "tag", 2, 1,
			v081Element(t, "tag", "@media", "media", 1, 7),
			v081Element(t, "media-type", "audio", "audio", 8, 13),
			v081Element(t, "source", "\"a b\"", "a b", 14, 19))
		v081Run(t, v081Lesson(t, line), "current", nil)
	})
}

func TestV081Contexts(t *testing.T) {
	t.Run("nested misplaced declaration", func(t *testing.T) {
		fragment := v081Line(t, "@fragment f {", "block-start", 2, 1,
			v081Element(t, "tag", "@fragment", "fragment", 1, 10),
			v081Element(t, "identifier", "f", "f", 11, 12), v081Element(t, "block-open", "{", "{", 13, 14))
		v081Run(t, v081Lesson(t, fragment, v081Open(t, "instr", 3, 2),
			v081Attempt(t, "@media audio a b", 1, 4, 3), v081Close(t, 4, 3), v081Close(t, 3, 2)),
			"current", [][3]int{{5, 14, 17}})
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			body := []model.Line{v081Open(t, name, 2, 1),
				v081Line(t, "@media audio a b", "content", 3, 2,
					v081Element(t, "content", "@media audio a b", "@media audio a b", 1, 17)), v081Close(t, 3, 2)}
			v081Run(t, v081Lesson(t, body...), "current", nil)
			v081Run(t, v081Lesson(t, append(body, v081Attempt(t, "@media audio a b", 1, 2, 1))...),
				"current", [][3]int{{6, 14, 17}})
		})
	}
	t.Run("escaped and inline pseudo tags", func(t *testing.T) {
		v081Run(t, v081Lesson(t,
			v081Line(t, "\\@media audio a b", "content", 2, 1,
				v081Element(t, "content", "\\@media audio a b", "@media audio a b", 1, 18)),
			v081Line(t, "Use @media audio a b", "content", 2, 1,
				v081Element(t, "content", "Use @media audio a b", "Use @media audio a b", 1, 21))), "current", nil)
	})
	t.Run("another real tag is not media", func(t *testing.T) {
		v081Run(t, v081Lesson(t, v081Line(t, "@instr audio a b", "tag", 2, 1,
			v081Element(t, "tag", "@instr", "instr", 1, 7),
			v081Element(t, "content", "audio a b", "audio a b", 8, 17))), "current", nil)
	})
	t.Run("first and last declarations counted once", func(t *testing.T) {
		v081Run(t, v081Lesson(t, v081Attempt(t, "@media audio a b", 1, 2, 1),
			v081Attempt(t, "@media audio a b", 1, 2, 1)), "current", [][3]int{{3, 14, 17}, {4, 14, 17}})
	})
}

func TestV081EndingsVersionsAndRuns(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v081Lesson(t, v081Attempt(t, "@media audio Audio files/track 1.mp3", 1, 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
				if !final {
					result.Lines[2].LineEnding = ""
				}
				v081Run(t, result, "current", [][3]int{{3, 14, 37}})
			})
		}
	}
	t.Run("empty input", func(t *testing.T) {
		result := v081Lesson(t)
		result.Lines = []model.Line{}
		result.Document.LineCount, result.Document.Metadata.Title = nil, nil
		v081Run(t, result, "current", nil)
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v081Lesson(t, v081Attempt(t, "@media audio a b", 1, 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0].Raw = "@dsl-version " + version
			result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			before := v081Snapshot(t, result)
			checks, err := validator.Prepare(result, v081Registry(t))
			if err == nil || checks != nil || before != v081Snapshot(t, result) {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
	first := v081Lesson(t, v081Attempt(t, "@media audio a b", 1, 2, 1))
	second := v081Lesson(t, v081Media(t, "audio", "audio", "track.mp3", 2, 1))
	v081Run(t, first, "first", [][3]int{{3, 14, 17}})
	v081Run(t, second, "first", nil)
	v081Run(t, first, "second", [][3]int{{3, 14, 17}})
}

// v081Media задаёт поля модели из явно заданных частей; пустой canonical
// означает неизвестный тип с null Value и исходной ошибкой парсера.
func v081Media(t *testing.T, kind, canonical, source string, parent, depth int) model.Line {
	t.Helper()
	end := 8 + utf8.RuneCountInString(kind)
	typeElement := v081Element(t, "media-type", kind, canonical, 8, end)
	if canonical == "" {
		typeElement.Value = nil
	}
	return v081Line(t, "@media "+kind+" "+source, "tag", parent, depth,
		v081Element(t, "tag", "@media", "media", 1, 7), typeElement,
		v081Element(t, "source", source, source, end+1, end+1+utf8.RuneCountInString(source)))
}

// v081Attempt задаёт разобранный tag и ошибочный остаток в unparsed.
func v081Attempt(t *testing.T, raw string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + 6
	return v081Line(t, raw, "invalid", parent, depth,
		v081Element(t, "tag", string(runes[start-1:end-1]), "media", start, end),
		v081Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v081Lesson добавляет обязательные версию и заголовок; body начинается с 3.
// Согласованные ошибки и ссылки создаются для unparsed и неизвестного media-type.
func v081Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v081Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v081Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v081Element(t, "version", "1.2", "1.2", 14, 17)),
		v081Line(t, "# Lesson", "heading", 0, 0,
			v081Element(t, "heading-level", "#", "1", 1, 2), v081Element(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v081Open задаёт реальное объявление блока с явным родителем.
func v081Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v081Line(t, "@"+name+" {", "block-start", parent, depth,
		v081Element(t, "tag", "@"+name, name, 1, len(name)+2),
		v081Element(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v081Close задаёт закрытие указанного блока.
func v081Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v081Line(t, "}", "block-end", parent, depth, v081Element(t, "block-close", "}", "}", 1, 2))
}

// v081Line сохраняет явно заданную структуру, не классифицируя исходный текст.
func v081Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v081Element задаёт исходные Unicode-колонки с исключающим концом.
func v081Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v081Registry использует весь штатный каталог регистраций.
func v081Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v081Run проверяет только новые V081 полного конвейера по явным диапазонам.
// Снимки модели контролируют сохранность источника и прежней истории.
func v081Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v081Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v081Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v081Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V081" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V081 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Источник медиа записан несколькими частями без общих кавычек.\nконкретизация: Один путь с пробелами заключается в общие двойные кавычки; несколько ресурсов записываются отдельными тегами" {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V081 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v081Snapshot(t, result)), &restored); err != nil {
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
	if v081Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v081Snapshot сохраняет независимое представление модели.
func v081Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

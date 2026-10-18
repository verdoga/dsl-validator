package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV043Registration(t *testing.T) {
	definitions, err := v043Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V043" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "document" || definition.Check == nil ||
			definition.Message != "В документе с @media отсутствует @resource-dir." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V043 definitions = %d, want 1", count)
	}
}

func TestV043ReferenceAndExample(t *testing.T) {
	// Оба полных примера сохраняются без добавлений и изменения исходного текста.
	for _, reference := range []bool{true, false} {
		t.Run(map[bool]string{true: "reference", false: "example"}[reference], func(t *testing.T) {
			lines := []model.Line{v043Version(t), v043Tag(t, "document-id", "identifier", "lesson-a", 0, 0)}
			parent := 3
			var want [][3]int
			if reference {
				lines = append(lines, v043Tag(t, "resource-dir", "resource-path", "Audio", 0, 0))
				parent = 4
			} else {
				want = [][3]int{{4, 1, 7}}
			}
			lines = append(lines, v043Heading(t), v043Media(t, "media", "audio", "Audio/track.mp3", "Audio/track.mp3", "", parent, 1))
			result := v043Model(t, lines...)
			id, title := "lesson-a", "Lesson"
			result.Document.Metadata.DocumentID, result.Document.Metadata.Title = &id, &title
			if reference {
				result.Document.Metadata.ResourceDirs = []string{"Audio"}
			}
			v043Run(t, result, "current", want)
		})
	}
}

func TestV043MediaSources(t *testing.T) {
	for _, tc := range []struct{ name, kind, raw, value string }{
		{"number", "audio", "0012", "0012"},
		{"relative audio", "audio", "Audio/track.mp3", "Audio/track.mp3"},
		{"relative video", "video", "Video/clip.mp4", "Video/clip.mp4"},
		{"relative image", "image", "Images/image.png", "Images/image.png"},
		{"absolute path", "audio", "/Audio/track.mp3", "/Audio/track.mp3"},
		{"quoted path", "audio", `"Audio/track one.mp3"`, "Audio/track one.mp3"},
		{"quoted number is a path", "audio", `"12"`, "12"},
		{"Unicode path", "image", "Картинки/🙂.png", "Картинки/🙂.png"},
		{"resource name is not a declaration", "audio", `"@resource-dir"`, "@resource-dir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v043Run(t, v043Document(t, v043Media(t, "media", tc.kind, tc.raw, tc.value, "", 2, 1)), "current", [][3]int{{3, 1, 7}})
		})
	}
	t.Run("canonical name and original indentation", func(t *testing.T) {
		v043Run(t, v043Document(t, v043Media(t, "MeDiA", "audio", "track.mp3", "track.mp3", " \t", 2, 1)), "current", [][3]int{{3, 3, 9}})
	})
	t.Run("only first of several media", func(t *testing.T) {
		v043Run(t, v043Document(t, v043Media(t, "media", "audio", "one.mp3", "one.mp3", "", 2, 1),
			v043Media(t, "media", "video", "two.mp4", "two.mp4", "", 2, 1)), "current", [][3]int{{3, 1, 7}})
	})
	t.Run("first physical line", func(t *testing.T) {
		// Восстановленная модель содержит ошибочно расположенное объявление,
		// но V043 независимо проверяет наличие метатега во всём документе.
		v043Run(t, v043Model(t, v043Media(t, "media", "audio", "one.mp3", "one.mp3", "", 0, 0), v043Version(t), v043Heading(t)), "current", [][3]int{{1, 1, 7}})
	})
}

func TestV043ResourceDeclarations(t *testing.T) {
	t.Run("multiple directories before media", func(t *testing.T) {
		result := v043Model(t, v043Version(t), v043Tag(t, "resource-dir", "resource-path", "Audio", 0, 0),
			v043Tag(t, "resource-dir", "resource-path", "Images", 0, 0), v043Heading(t),
			v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 4, 1))
		result.Document.Metadata.ResourceDirs = []string{"Audio", "Images"}
		v043Run(t, result, "current", nil)
	})
	t.Run("canonical resource name", func(t *testing.T) {
		resource := v043Tag(t, "resource-dir", "resource-path", "Audio", 0, 0)
		resource.Raw, resource.Elements[0].Raw = "@RESOURCE-DIR Audio", "@RESOURCE-DIR"
		result := v043Model(t, v043Version(t), resource, v043Heading(t), v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 3, 1))
		result.Document.Metadata.ResourceDirs = []string{"Audio"}
		v043Run(t, result, "current", nil)
	})
	for _, tc := range []struct {
		name  string
		lines []model.Line
		other [][3]int
	}{
		{"after media at EOF", []model.Line{v043Version(t), v043Heading(t),
			v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1), v043Tag(t, "resource-dir", "resource-path", "Audio", 2, 1)}, [][3]int{{4, 1, 14}}},
		{"before version", []model.Line{v043Tag(t, "resource-dir", "resource-path", "Audio", 0, 0), v043Version(t), v043Heading(t),
			v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 3, 1)}, [][3]int{{1, 1, 14}}},
		{"inside block", []model.Line{v043Version(t), v043Heading(t), v043Open(t, "note", 2, 1),
			v043Tag(t, "resource-dir", "resource-path", "Audio", 3, 2), v043Close(t, 3, 2),
			v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1)}, [][3]int{{4, 1, 14}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v043Run(t, v043Model(t, tc.lines...), "current", nil, tc.other...) })
	}
	t.Run("empty recognized resource", func(t *testing.T) {
		resource := v043Line(t, "@resource-dir", "tag", 0, 0, v043Element(t, "tag", "@resource-dir", "resource-dir", 1, 14))
		result := v043Model(t, v043Version(t), resource, v043Heading(t), v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 3, 1))
		v043ParserError(t, result, 2, 0, "Не указан путь каталога.")
		v043Run(t, result, "current", nil)
	})
	t.Run("recognized resource with unparsed value", func(t *testing.T) {
		resource := v043Line(t, `@resource-dir "Audio`, "tag", 0, 0,
			v043Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			model.Element{ElementType: "unparsed", Raw: `"Audio`, Start: 15, End: 21, ErrorIDs: []string{}})
		result := v043Model(t, v043Version(t), resource, v043Heading(t), v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 3, 1))
		v043ParserError(t, result, 2, 1, "Не закрыта кавычка пути.")
		v043Run(t, result, "current", nil)
	})
}

func TestV043NestedAndLiteralContent(t *testing.T) {
	for _, include := range []bool{false, true} {
		t.Run(map[bool]string{false: "unused fragment", true: "fragment included twice"}[include], func(t *testing.T) {
			fragment := v043Line(t, "@fragment clip {", "block-start", 2, 1,
				v043Element(t, "tag", "@fragment", "fragment", 1, 10), v043Element(t, "identifier", "clip", "clip", 11, 15),
				v043Element(t, "block-open", "{", "{", 16, 17))
			result := v043Document(t, fragment, v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 3, 2), v043Close(t, 3, 2))
			if include {
				result = v043Model(t, append(result.Lines, v043Tag(t, "include", "identifier", "clip", 2, 1), v043Tag(t, "include", "identifier", "clip", 2, 1))...)
			}
			v043Run(t, result, "current", [][3]int{{4, 1, 7}})
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run(tag+"/literal media", func(t *testing.T) {
			v043Run(t, v043Document(t, v043Open(t, tag, 2, 1), v043Text(t, "@media audio track.mp3", 3, 2), v043Close(t, 3, 2)), "current", nil)
		})
		t.Run(tag+"/literal resource does not count", func(t *testing.T) {
			v043Run(t, v043Document(t, v043Open(t, tag, 2, 1), v043Text(t, "@resource-dir Audio", 3, 2), v043Close(t, 3, 2),
				v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1)), "current", [][3]int{{6, 1, 7}})
		})
		t.Run(tag+"/first recognized media after literal", func(t *testing.T) {
			v043Run(t, v043Document(t, v043Open(t, tag, 2, 1), v043Text(t, "@media audio ignored.mp3", 3, 2), v043Close(t, 3, 2),
				v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1)), "current", [][3]int{{6, 1, 7}})
		})
		t.Run(tag+"/inline text is not an announcement", func(t *testing.T) {
			v043Run(t, v043Document(t, v043Tag(t, tag, "content", "@media audio track.mp3", 2, 1)), "current", nil)
		})
	}
	t.Run("escaped and ordinary text", func(t *testing.T) {
		line := v043Line(t, `\@media audio track.mp3`, "content", 2, 1,
			v043Element(t, "content", `\@media audio track.mp3`, "@media audio track.mp3", 1, 24))
		v043Run(t, v043Document(t, line, v043Text(t, "Mention @media and @resource-dir.", 2, 1)), "current", nil)
	})
	t.Run("include of text fragment", func(t *testing.T) {
		fragment := v043Line(t, "@fragment clip {", "block-start", 2, 1,
			v043Element(t, "tag", "@fragment", "fragment", 1, 10), v043Element(t, "identifier", "clip", "clip", 11, 15),
			v043Element(t, "block-open", "{", "{", 16, 17))
		v043Run(t, v043Document(t, v043Tag(t, "include", "identifier", "clip", 2, 1), fragment,
			v043Text(t, "Read.", 4, 2), v043Close(t, 4, 2)), "current", nil)
	})
}

func TestV043Unparsed(t *testing.T) {
	t.Run("whole media unparsed", func(t *testing.T) {
		line := v043Line(t, "@media unknown file", "invalid", 2, 1,
			model.Element{ElementType: "unparsed", Raw: "@media unknown file", Start: 1, End: 20, ErrorIDs: []string{}})
		result := v043Document(t, line)
		v043ParserError(t, result, 3, 0, "Не удалось разобрать объявление.")
		v043Run(t, result, "current", nil)
	})
	for _, kind := range []model.LineType{"tag", "invalid"} {
		t.Run("recognized media on "+string(kind), func(t *testing.T) {
			line := v043Line(t, "@media unknown file", kind, 2, 1,
				v043Element(t, "tag", "@media", "media", 1, 7), model.Element{ElementType: "unparsed", Raw: "unknown file", Start: 8, End: 20, ErrorIDs: []string{}})
			result := v043Document(t, line)
			v043ParserError(t, result, 3, 1, "Не удалось разобрать параметры.")
			v043Run(t, result, "current", [][3]int{{3, 1, 7}})
		})
	}
	t.Run("whole resource unparsed does not count", func(t *testing.T) {
		resource := v043Line(t, "@resource-dir Audio", "invalid", 2, 1,
			model.Element{ElementType: "unparsed", Raw: "@resource-dir Audio", Start: 1, End: 20, ErrorIDs: []string{}})
		result := v043Document(t, v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1), resource)
		v043ParserError(t, result, 4, 0, "Не удалось разобрать объявление.")
		v043Run(t, result, "current", [][3]int{{3, 1, 7}})
	})
}

func TestV043EmptyVersionsEndingsAndHistory(t *testing.T) {
	v043Run(t, v043Model(t), "current", nil)
	v043Run(t, v043Document(t), "current", nil)
	v043Run(t, v043Document(t, v043Line(t, "", "blank", 2, 1)), "current", nil)
	result := v043Model(t, v043Version(t), v043Tag(t, "resource-dir", "resource-path", "Audio", 0, 0), v043Heading(t))
	result.Document.Metadata.ResourceDirs = []string{"Audio"}
	v043Run(t, result, "current", nil)
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, last := range []model.LineEnding{eol, ""} {
			t.Run("endings/"+string(eol)+"/"+string(last), func(t *testing.T) {
				result := v043Document(t, v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				result.Lines[2].LineEnding = last
				v043Run(t, result, "current", [][3]int{{3, 1, 7}})
			})
		}
	}
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v043Document(t, v043Media(t, "media", "audio", "track.mp3", "track.mp3", "", 2, 1))
			if version == "unknown" {
				result.Document.DSLVersion = nil
			} else {
				result.Document.DSLVersion = &version
				result.Lines[0].Raw = "@dsl-version " + version
				result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			}
			before := v043Snapshot(t, result)
			checks, err := validator.Prepare(result, v043Registry(t))
			if err == nil || checks != nil || v043Snapshot(t, result) != before {
				t.Fatal("unsupported version accepted or model changed")
			}
		})
	}
	t.Run("repeat and independent documents", func(t *testing.T) {
		result := v043Document(t, v043Media(t, "MeDiA", "audio", "track.mp3", "track.mp3", "", 2, 1))
		v043Run(t, result, "first", [][3]int{{3, 1, 7}})
		v043Run(t, result, "second", [][3]int{{3, 1, 7}})
		v043Run(t, v043Document(t), "current", nil)
		v043Run(t, v043Document(t, v043Media(t, "media", "image", "x.png", "x.png", "", 2, 1)), "current", [][3]int{{3, 1, 7}})
	})
}

// v043Registry создаёт реестр исключительно через штатный каталог.
func v043Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v043Model задаёт номера явно построенным строкам без разбора DSL.
func v043Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, count := "1.2", "UTF-8", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	return result
}

// v043Document добавляет обязательные версию и заголовок к фрагменту тела.
func v043Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v043Model(t, append([]model.Line{v043Version(t), v043Heading(t)}, body...)...)
	title := "Lesson"
	result.Document.Metadata.Title = &title
	return result
}

// v043Line сохраняет явно заданную структуру физической строки.
func v043Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v043Element задаёт исходное и семантическое значения и диапазон [start,end).
func v043Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v043Tag строит объявление из заданных полей. Длины вычисляются только для
// входной модели; ожидаемые диапазоны во всех тестах заданы явно.
func v043Tag(t *testing.T, tag string, field model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 2
	return v043Line(t, "@"+tag+" "+value, "tag", parent, depth, v043Element(t, "tag", "@"+tag, tag, 1, end),
		v043Element(t, field, value, value, end+1, end+1+utf8.RuneCountInString(value)))
}

// v043Version задаёт объявление версии DSL 1.2.
func v043Version(t *testing.T) model.Line {
	t.Helper()
	return v043Tag(t, "dsl-version", "version", "1.2", 0, 0)
}

// v043Heading задаёт корневой заголовок первого уровня из примеров.
func v043Heading(t *testing.T) model.Line {
	t.Helper()
	return v043Line(t, "# Lesson", "heading", 0, 0, v043Element(t, "heading-level", "#", "1", 1, 2), v043Element(t, "title", "Lesson", "Lesson", 3, 9))
}

// v043Media сохраняет исходную запись и семантическое значение источника.
func v043Media(t *testing.T, spelling, kind, raw, value, prefix string, parent, depth int) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(prefix) + 1
	end := start + len(spelling) + 1
	source := end + len(kind) + 2
	return v043Line(t, prefix+"@"+spelling+" "+kind+" "+raw, "tag", parent, depth,
		v043Element(t, "tag", "@"+spelling, "media", start, end), v043Element(t, "media-type", kind, kind, end+1, source-1),
		v043Element(t, "source", raw, value, source, source+utf8.RuneCountInString(raw)))
}

// v043Open задаёт блочное объявление без параметров.
func v043Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 2
	return v043Line(t, "@"+tag+" {", "block-start", parent, depth, v043Element(t, "tag", "@"+tag, tag, 1, end),
		v043Element(t, "block-open", "{", "{", end+1, end+2))
}

// v043Close задаёт закрытие блока с указанным родителем.
func v043Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v043Line(t, "}", "block-end", parent, depth, v043Element(t, "block-close", "}", "}", 1, 2))
}

// v043Text задаёт буквальное содержимое без преобразований.
func v043Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v043Line(t, raw, "content", parent, depth, v043Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v043ParserError добавляет исходную ошибку на явно выбранный элемент,
// обеспечивая согласованность ErrorIDs и флагов во входе.
func v043ParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	line.HasErrors, result.Document.HasErrors = true, true
	element.ErrorIDs = append(element.ErrorIDs, "parser-error")
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start}, End: model.Position{Line: number, Column: element.End}}})
}

// v043Run проверяет V043 текущего запуска и неизменность исходника и истории.
// want содержит связанные диапазоны, по одному на ожидаемое сообщение V043.
// otherTags явно задаёт независимые ошибки V036 на неверно размещённых метатегах;
// они учитываются отдельно и не подменяют результаты проверяемой диагностики.
func v043Run(t *testing.T, result *model.Result, source string, want [][3]int, otherTags ...[3]int) {
	t.Helper()
	before := v043Snapshot(t, result)
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v043Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v043Snapshot(t, result) != before {
		t.Fatal("Prepare changed model")
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got, others []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i < diagnosticCount {
			continue
		}
		if entry.Source != source {
			t.Fatalf("new diagnostic source = %q, want %q", entry.Source, source)
		}
		if entry.DiagnosticCode == "V043" {
			got = append(got, entry)
		} else if entry.SeverityLevel == "error" {
			others = append(others, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V043 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location != nil || entry.SeverityLevel != "error" || entry.DiagnosticScope != "document" || entry.Fatal ||
			entry.Message != "В документе с @media отсутствует @resource-dir." || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != location {
			t.Errorf("V043 = %+v; want document error with related %+v", entry, location)
		}
		expected.Document.HasErrors = true
	}
	if len(others) != len(otherTags) {
		t.Fatalf("other current errors = %+v, want V036 on %v", others, otherTags)
	}
	for i, position := range otherTags {
		entry := others[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.DiagnosticCode != "V036" || entry.DiagnosticScope != "element" || entry.Location == nil || *entry.Location != location {
			t.Fatalf("unexpected other error: %+v", entry)
		}
		expected.Document.HasErrors = true
		line := &expected.Lines[position[0]-1]
		line.HasErrors = true
		matched := 0
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "tag" && element.Start == position[1] && element.End == position[2] {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched++
			}
		}
		if matched != 1 {
			t.Fatal("expected V036 range does not identify one tag")
		}
	}
	if len(result.Processing) != processingCount+1 || result.Processing[processingCount].ID != source || result.Processing[processingCount].Tool != "dsl-validator" || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected history changes")
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v043Snapshot(t, &previous) != v043Snapshot(t, &expected) {
		t.Fatal("source, flags, links or prior history differ from expected changes")
	}
}

// v043Snapshot фиксирует полную модель для сравнения сохранности данных.
func v043Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

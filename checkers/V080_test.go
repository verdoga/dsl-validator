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

func TestV080Registration(t *testing.T) {
	definitions, err := v080TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V080" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У @media отсутствует путь или номер ресурса." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V080 definitions = %d, want 1", count)
	}
}

func TestV080ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Перед неизменённым эталоном добавлены только версия и заголовок.
		result := v080TestDocument(t, v080TestLine(t, "@media audio 012", "tag", 2, 1,
			v080TestElement(t, "tag", "@media", "media", 1),
			v080TestElement(t, "media-type", "audio", "audio", 8), v080TestElement(t, "source", "012", "012", 14)))
		v080TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@media audio 012")
		v080TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		// Перед неизменённым примером добавлены только версия и заголовок.
		result := v080TestDocument(t, v080TestInvalid(t, "@media audio", 2, 1))
		v080TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@media audio")
		v080TestRun(t, result, "current", [][3]int{{3, 1, 7}})
	})
}

func TestV080MissingSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"parsed audio", v080TestNamed(t, "media", "media-type", "audio", 2, 1), [][3]int{{3, 1, 7}}},
		{"parsed video", v080TestNamed(t, "media", "media-type", "video", 2, 1), [][3]int{{3, 1, 7}}},
		{"parsed image", v080TestNamed(t, "media", "media-type", "image", 2, 1), [][3]int{{3, 1, 7}}},
		{"unparsed declaration", v080TestInvalid(t, "@media audio", 2, 1), [][3]int{{3, 1, 7}}},
		{"partially parsed type", v080TestLine(t, "@media audio", "invalid", 2, 1,
			v080TestElement(t, "tag", "@media", "media", 1), v080TestUnparsed(t, "audio", 8)), [][3]int{{3, 1, 7}}},
		{"unparsed tag line", v080TestLine(t, "@media\taudio", "tag", 2, 1,
			v080TestUnparsed(t, "@media\taudio", 1)), [][3]int{{3, 1, 7}}},
		{"parsed case and indentation", v080TestLine(t, " \t@MeDiA AuDiO", "tag", 2, 1,
			v080TestElement(t, "tag", "@MeDiA", "media", 3),
			v080TestElement(t, "media-type", "AuDiO", "audio", 10)), [][3]int{{3, 3, 9}}},
		{"raw case and indentation", v080TestInvalid(t, " \t@MeDiA AuDiO", 2, 1), [][3]int{{3, 3, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v080TestDocument(t, tc.line)
			if len(result.Diagnostics) == 0 {
				v080TestParserError(t, result, 3, -1, "Отсутствует источник медиа.")
			}
			v080TestRun(t, result, "current", tc.want)
		})
	}
	for _, raw := range []string{"@media\taudio", "@media \t video", "@media audio ", "@media audio\t", "@media audio \t ", "@MEDIA\t\tIMAGE\t"} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			v080TestRun(t, v080TestDocument(t, v080TestInvalid(t, raw, 2, 1)), "current", [][3]int{{3, 1, 7}})
		})
	}
	t.Run("each declaration reports once", func(t *testing.T) {
		v080TestRun(t, v080TestDocument(t,
			v080TestInvalid(t, "@media audio", 2, 1), v080TestInvalid(t, "@media video", 2, 1)),
			"current", [][3]int{{3, 1, 7}, {4, 1, 7}})
	})
}

func TestV080TypeAndNameBoundaries(t *testing.T) {
	for _, raw := range []string{"@media", "@media \t", "@media sound", "@media audioExtra", `@media "audio"`,
		"@media\u00a0audio", "@media\u2003audio", "@media \u00a0audio", "@media audio\u00a0", "@media audio}",
		"@media audio{", "@mediaExtra audio", "@media-name audio", "@medi audio"} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			v080TestRun(t, v080TestDocument(t, v080TestInvalid(t, raw, 2, 1)), "current", nil)
		})
	}
	t.Run("unknown media type has null value", func(t *testing.T) {
		kind := v080TestElement(t, "media-type", "sound", "", 8)
		kind.Value = nil
		result := v080TestDocument(t, v080TestLine(t, "@media sound", "tag", 2, 1,
			v080TestElement(t, "tag", "@media", "media", 1), kind))
		v080TestParserError(t, result, 3, 1, "Неизвестный тип медиа.")
		v080TestRun(t, result, "current", nil)
	})
}

func TestV080PresentSource(t *testing.T) {
	for _, tc := range []struct{ kind, raw, value string }{
		{"audio", "track.mp3", "track.mp3"}, {"video", "clip.mp4", "clip.mp4"}, {"image", "photo.png", "photo.png"},
		{"audio", "0", "0"}, {"audio", "012", "012"},
		{"audio", "0001234567890123456789012345678901234567890", "0001234567890123456789012345678901234567890"},
		{"audio", `"dialogue tracks/track 01.mp3"`, "dialogue tracks/track 01.mp3"},
		{"audio", `"012"`, "012"},
	} {
		t.Run(tc.kind+" "+tc.raw, func(t *testing.T) {
			v080TestRun(t, v080TestDocument(t, v080TestLine(t, "@media "+tc.kind+" "+tc.raw, "tag", 2, 1,
				v080TestElement(t, "tag", "@media", "media", 1), v080TestElement(t, "media-type", tc.kind, tc.kind, 8),
				v080TestElement(t, "source", tc.raw, tc.value, 14))), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{{`""`, ""}, {`" "`, " "}} {
		t.Run("empty quoted source "+tc.raw, func(t *testing.T) {
			result := v080TestDocument(t, v080TestLine(t, "@media audio "+tc.raw, "tag", 2, 1,
				v080TestElement(t, "tag", "@media", "media", 1), v080TestElement(t, "media-type", "audio", "audio", 8),
				v080TestElement(t, "source", tc.raw, tc.value, 14)))
			v080TestParserError(t, result, 3, 2, "Пустой путь медиа в кавычках.")
			v080TestRun(t, result, "current", nil)
		})
	}
	for _, source := range []string{"\u00a0", "\u2003", "\u3000", "\v", "\u200b", "\ufeff"} {
		t.Run("preserve source "+strconv.Quote(source), func(t *testing.T) {
			// Исходный аргумент не пуст после удаления только U+0020/U+0009.
			// Доступность или допустимость имени файла не моделируются.
			v080TestRun(t, v080TestDocument(t, v080TestLine(t, "@media audio "+source+"\t ", "tag", 2, 1,
				v080TestElement(t, "tag", "@media", "media", 1), v080TestElement(t, "media-type", "audio", "audio", 8),
				v080TestElement(t, "source", source, source, 14))), "current", nil)
		})
	}
	for _, tail := range []string{` ""`, ` " "`, " {", ` "track.mp3`, " track.mp3 extra", "\ttrack.mp3"} {
		for _, parsed := range []bool{false, true} {
			t.Run(strconv.Quote(tail)+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v080TestInvalid(t, "@media audio"+tail, 2, 1)
				if parsed {
					line = v080TestLine(t, "@media audio"+tail, "invalid", 2, 1,
						v080TestElement(t, "tag", "@media", "media", 1), v080TestElement(t, "media-type", "audio", "audio", 8),
						v080TestUnparsed(t, tail, 13))
				}
				// Ошибочный источник остаётся непустым; кавычки не декодируются.
				v080TestRun(t, v080TestDocument(t, line), "current", nil)
			})
		}
	}
}

func TestV080Contexts(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@media audio`, "@media audio"}, {`\\@media audio`, `\@media audio`},
		{"Mention @media audio", "Mention @media audio"}, {"\u00a0@media audio", "\u00a0@media audio"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v080TestRun(t, v080TestDocument(t, v080TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("note mention", func(t *testing.T) {
		v080TestRun(t, v080TestDocument(t, v080TestNamed(t, "note", "content", "Mention @media audio", 2, 1)), "current", nil)
	})
	t.Run("forbidden nesting still checked", func(t *testing.T) {
		v080TestRun(t, v080TestDocument(t, v080TestOpen(t, "text", 2, 1),
			v080TestInvalid(t, "@media audio", 3, 2), v080TestText(t, "Read.", "Read.", 3, 2),
			v080TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 7}})
	})
	t.Run("branch", func(t *testing.T) {
		v080TestRun(t, v080TestDocument(t, v080TestOpen(t, "variants", 2, 1),
			v080TestNamed(t, "variant", "name", "A", 3, 2), v080TestInvalid(t, "@media audio", 4, 3),
			v080TestText(t, "Read.", "Read.", 4, 3), v080TestNamed(t, "variant", "name", "B", 3, 2),
			v080TestText(t, "Write.", "Write.", 7, 3), v080TestClose(t, 3, 2)), "current", [][3]int{{5, 1, 7}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		for _, parsed := range []bool{false, true} {
			t.Run(tag+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				open := v080TestOpen(t, tag, 2, 1)
				if !parsed {
					// Ошибка разделителя по DSL 1.29; разбор сохранил блок.
					raw := "@" + strings.ToUpper(tag)
					open = v080TestLine(t, raw+"{", "block-start", 2, 1,
						v080TestUnparsed(t, raw, 1), v080TestElement(t, "block-open", "{", "{", len(raw)+1))
				}
				v080TestRun(t, v080TestDocument(t, open,
					v080TestText(t, "@media audio", "@media audio", 3, 2), v080TestClose(t, 3, 2),
					v080TestInvalid(t, "@media audio", 2, 1)), "current", [][3]int{{6, 1, 7}})
			})
		}
	}
	t.Run("literal block in task", func(t *testing.T) {
		v080TestRun(t, v080TestDocument(t, v080TestNamed(t, "task", "identifier", "a", 2, 1),
			v080TestOpen(t, "editor", 3, 2), v080TestText(t, "@media audio", "@media audio", 4, 3),
			v080TestClose(t, 4, 3), v080TestInvalid(t, "@media audio", 3, 2),
			v080TestText(t, "Read.", "Read.", 3, 2)), "current", [][3]int{{7, 1, 7}})
	})
}

func TestV080EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v080TestDocument(t, v080TestInvalid(t, "@media audio", 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v080TestRun(t, result, "first", [][3]int{{3, 1, 7}})
			v080TestRun(t, result, "second", [][3]int{{3, 1, 7}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v080TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v080TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v080TestDocument(t)
			result.Document.DSLVersion = &version
			before := v080TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v080TestRegistry(t))
			if err == nil || checks != nil || v080TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v080TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v080TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v080TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v080TestHeading(t, "#", "Lesson"),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v080TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v080TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v080TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		element := &line.Elements[index]
		scope, start, end = diagnostics.ScopeElement, element.Start, element.End
		element.ErrorIDs = []string{id}
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
}

func v080TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v080TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v080TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v080TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v080TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v080TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v080TestLine(t, raw, "invalid", parent, depth, v080TestUnparsed(t, raw, 1))
}

func v080TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v080TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v080TestElement(t, "tag", "@"+tag, tag, 1), v080TestElement(t, kind, value, value, len(tag)+3))
}

func v080TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v080TestLine(t, marker+" "+title, "heading", 0, 0,
		v080TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v080TestElement(t, "title", title, title, len(marker)+2))
}

func v080TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v080TestLine(t, raw, "content", parent, depth, v080TestElement(t, "content", raw, value, 1))
}

func v080TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v080TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v080TestElement(t, "tag", "@"+tag, tag, 1), v080TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v080TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v080TestLine(t, "}", "block-end", parent, depth, v080TestElement(t, "block-close", "}", "}", 1))
}

func v080TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v080TestRun выполняет полный каталог и сравнивает новые V080 с явными
// тройками строка/начало/конец. Правые границы исключаются.
func v080TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v080TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v080TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v080TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V080" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V080 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "У @media отсутствует путь или номер ресурса." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V080 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v080TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v080TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v080TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v080TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var source strings.Builder
	for _, line := range result.Lines {
		source.WriteString(line.Raw)
		source.WriteString(string(line.LineEnding))
	}
	if source.String() != want {
		t.Fatalf("source = %q, want %q", source.String(), want)
	}
}

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

func TestV078Registration(t *testing.T) {
	definitions, err := v078TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V078" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У @media отсутствует тип ресурса." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V078 definitions = %d, want 1", count)
	}
}

func TestV078ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// Только версия и заголовок добавлены перед неизменённым эталоном.
		result := v078TestDocument(t, v078TestLine(t, "@media audio track.mp3", "tag", 2, 1,
			v078TestElement(t, "tag", "@media", "media", 1),
			v078TestElement(t, "media-type", "audio", "audio", 8),
			v078TestElement(t, "source", "track.mp3", "track.mp3", 14)))
		v078TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@media audio track.mp3")
		v078TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		// Только версия и заголовок добавлены перед неизменённым примером.
		result := v078TestDocument(t, v078TestInvalid(t, "@media", 2, 1))
		v078TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@media")
		v078TestRun(t, result, "current", [][3]int{{3, 1, 7}})
	})
}

func TestV078MissingType(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"parsed tag", v078TestTag(t, "media", 2, 1), [][3]int{{3, 1, 7}}},
		{"invalid without tag", v078TestInvalid(t, "@media", 2, 1), [][3]int{{3, 1, 7}}},
		{"tag line with unparsed", v078TestLine(t, "@media", "tag", 2, 1,
			v078TestUnparsed(t, "@media", 1)), [][3]int{{3, 1, 7}}},
		{"partial invalid", v078TestLine(t, "@media \t", "invalid", 2, 1,
			v078TestElement(t, "tag", "@media", "media", 1), v078TestUnparsed(t, " \t", 7)), [][3]int{{3, 1, 7}}},
		{"parsed case and indentation", v078TestLine(t, " \t@MeDiA \t", "tag", 2, 1,
			v078TestElement(t, "tag", "@MeDiA", "media", 3)), [][3]int{{3, 3, 9}}},
		{"raw case and indentation", v078TestInvalid(t, " \t@MeDiA \t", 2, 1), [][3]int{{3, 3, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v078TestDocument(t, tc.line)
			if len(result.Diagnostics) == 0 {
				v078TestParserError(t, result, 3, -1, "Отсутствуют обязательные аргументы media.")
			}
			v078TestRun(t, result, "current", tc.want)
		})
	}
	for _, tail := range []string{" ", "   ", "\t", "\t\t", " \t "} {
		t.Run(strconv.Quote(tail), func(t *testing.T) {
			v078TestRun(t, v078TestDocument(t, v078TestInvalid(t, "@media"+tail, 2, 1)),
				"current", [][3]int{{3, 1, 7}})
		})
	}
	t.Run("each declaration reports once", func(t *testing.T) {
		v078TestRun(t, v078TestDocument(t,
			v078TestInvalid(t, "@media", 2, 1), v078TestInvalid(t, "@media", 2, 1)),
			"current", [][3]int{{3, 1, 7}, {4, 1, 7}})
	})
}

func TestV078PresentType(t *testing.T) {
	for _, tc := range []struct{ spelling, value, source string }{
		{"audio", "audio", "track.mp3"}, {"video", "video", "clip.mp4"}, {"image", "image", "photo.png"},
		{"AUDIO", "audio", "track.mp3"},
	} {
		t.Run(tc.spelling, func(t *testing.T) {
			v078TestRun(t, v078TestDocument(t, v078TestLine(t, "@media "+tc.spelling+" "+tc.source, "tag", 2, 1,
				v078TestElement(t, "tag", "@media", "media", 1),
				v078TestElement(t, "media-type", tc.spelling, tc.value, 8),
				v078TestElement(t, "source", tc.source, tc.source, 14))), "current", nil)
		})
	}
	t.Run("missing source only", func(t *testing.T) {
		result := v078TestDocument(t, v078TestNamed(t, "media", "media-type", "audio", 2, 1))
		v078TestParserError(t, result, 3, -1, "Отсутствует источник медиа.")
		v078TestRun(t, result, "current", nil)
	})
	t.Run("unknown media type has null value", func(t *testing.T) {
		kind := v078TestElement(t, "media-type", "sound", "", 8)
		kind.Value = nil
		result := v078TestDocument(t, v078TestLine(t, "@media sound file.bin", "tag", 2, 1,
			v078TestElement(t, "tag", "@media", "media", 1), kind,
			v078TestElement(t, "source", "file.bin", "file.bin", 14)))
		v078TestParserError(t, result, 3, 1, "Неизвестный тип медиа.")
		v078TestRun(t, result, "current", nil)
	})
	for _, tail := range []string{" sound file.bin", " track.mp3", " {", " }", ` ""`, "\taudio track.mp3", " звук ресурс.bin", "{", "}"} {
		for _, parsed := range []bool{false, true} {
			t.Run(strconv.Quote(tail)+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v078TestInvalid(t, "@media"+tail, 2, 1)
				if parsed {
					line = v078TestLine(t, "@media"+tail, "invalid", 2, 1,
						v078TestElement(t, "tag", "@media", "media", 1), v078TestUnparsed(t, tail, 7))
				}
				// Ошибочный тип или разделитель не означает пустой аргумент.
				v078TestRun(t, v078TestDocument(t, line), "current", nil)
			})
		}
	}
}

func TestV078DoesNotTrimOtherCharacters(t *testing.T) {
	// CR/LF не входят в Raw; все прочие White_Space, кроме U+0020/U+0009,
	// остаются в аргументе. U+200B и U+FEFF также не удаляются.
	for _, char := range []rune{'\v', '\f', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\u200b', '\ufeff'} {
		for _, surrounded := range []bool{false, true} {
			t.Run(strconv.QuoteRune(char)+"/surrounded="+strconv.FormatBool(surrounded), func(t *testing.T) {
				tail := string(char)
				if surrounded {
					tail = " \t" + tail + "\t "
				}
				// Распознанный tag гарантирует проверку самого остатка:
				// результат не может объясняться отказом резервного поиска.
				v078TestRun(t, v078TestDocument(t, v078TestLine(t, "@media"+tail, "invalid", 2, 1,
					v078TestElement(t, "tag", "@media", "media", 1), v078TestUnparsed(t, tail, 7))), "current", nil)
				v078TestRun(t, v078TestDocument(t, v078TestInvalid(t, "@media"+tail, 2, 1)), "raw", nil)
			})
		}
	}
}

func TestV078NonDeclarations(t *testing.T) {
	for _, raw := range []string{"@mediaExtra", "@media-name", "@medi"} {
		t.Run(raw, func(t *testing.T) {
			v078TestRun(t, v078TestDocument(t, v078TestInvalid(t, raw, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@media`, "@media"}, {`\\@media`, `\@media`},
		{"Mention @media", "Mention @media"}, {"\u00a0@media", "\u00a0@media"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v078TestRun(t, v078TestDocument(t, v078TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	t.Run("note mention", func(t *testing.T) {
		v078TestRun(t, v078TestDocument(t, v078TestNamed(t, "note", "content", "Mention @media", 2, 1)), "current", nil)
	})
}

func TestV078Contexts(t *testing.T) {
	t.Run("forbidden nesting still checked", func(t *testing.T) {
		v078TestRun(t, v078TestDocument(t, v078TestOpen(t, "text", 2, 1),
			v078TestInvalid(t, "@media", 3, 2), v078TestText(t, "Read.", "Read.", 3, 2),
			v078TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 7}})
	})
	t.Run("branch", func(t *testing.T) {
		v078TestRun(t, v078TestDocument(t, v078TestOpen(t, "variants", 2, 1),
			v078TestNamed(t, "variant", "name", "A", 3, 2), v078TestInvalid(t, "@media", 4, 3),
			v078TestText(t, "Read.", "Read.", 4, 3), v078TestNamed(t, "variant", "name", "B", 3, 2),
			v078TestText(t, "Write.", "Write.", 7, 3), v078TestClose(t, 3, 2)), "current", [][3]int{{5, 1, 7}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		for _, parsed := range []bool{false, true} {
			t.Run(tag+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				open := v078TestOpen(t, tag, 2, 1)
				if !parsed {
					// Ошибка разделителя по DSL 1.29; разбор сохранил блок.
					raw := "@" + strings.ToUpper(tag)
					open = v078TestLine(t, raw+"{", "block-start", 2, 1,
						v078TestUnparsed(t, raw, 1), v078TestElement(t, "block-open", "{", "{", len(raw)+1))
				}
				v078TestRun(t, v078TestDocument(t, open,
					v078TestText(t, "@media", "@media", 3, 2), v078TestClose(t, 3, 2),
					v078TestInvalid(t, "@media", 2, 1)), "current", [][3]int{{6, 1, 7}})
			})
		}
	}
	t.Run("literal block in task", func(t *testing.T) {
		v078TestRun(t, v078TestDocument(t, v078TestNamed(t, "task", "identifier", "a", 2, 1),
			v078TestOpen(t, "editor", 3, 2), v078TestText(t, "@media", "@media", 4, 3),
			v078TestClose(t, 4, 3), v078TestInvalid(t, "@media", 3, 2),
			v078TestText(t, "Read.", "Read.", 3, 2)), "current", [][3]int{{7, 1, 7}})
	})
}

func TestV078EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v078TestDocument(t, v078TestInvalid(t, "@media", 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v078TestRun(t, result, "first", [][3]int{{3, 1, 7}})
			v078TestRun(t, result, "second", [][3]int{{3, 1, 7}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v078TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v078TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v078TestDocument(t)
			result.Document.DSLVersion = &version
			before := v078TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v078TestRegistry(t))
			if err == nil || checks != nil || v078TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v078TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v078TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v078TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v078TestHeading(t, "#", "Lesson"),
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
				v078TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v078TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v078TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v078TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v078TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v078TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v078TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v078TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v078TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v078TestLine(t, raw, "invalid", parent, depth, v078TestUnparsed(t, raw, 1))
}

func v078TestTag(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v078TestLine(t, "@"+tag, "tag", parent, depth, v078TestElement(t, "tag", "@"+tag, tag, 1))
}

func v078TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v078TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v078TestElement(t, "tag", "@"+tag, tag, 1), v078TestElement(t, kind, value, value, len(tag)+3))
}

func v078TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v078TestLine(t, marker+" "+title, "heading", 0, 0,
		v078TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v078TestElement(t, "title", title, title, len(marker)+2))
}

func v078TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v078TestLine(t, raw, "content", parent, depth, v078TestElement(t, "content", raw, value, 1))
}

func v078TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v078TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v078TestElement(t, "tag", "@"+tag, tag, 1), v078TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v078TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v078TestLine(t, "}", "block-end", parent, depth, v078TestElement(t, "block-close", "}", "}", 1))
}

func v078TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v078TestRun выполняет полный каталог и сравнивает новые V078 с явными
// тройками строка/начало/конец. Правые границы исключаются.
func v078TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v078TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v078TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v078TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V078" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V078 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "У @media отсутствует тип ресурса." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V078 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v078TestSnapshot(t, result)), &restored); err != nil {
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
	if v078TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v078TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v078TestSource(t *testing.T, result *model.Result, want string) {
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

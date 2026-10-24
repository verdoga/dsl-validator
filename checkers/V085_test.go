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

func TestV085Registration(t *testing.T) {
	definitions, err := v085TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V085" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Литеральная обратная косая черта в пути медиа не удвоена." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V085 definitions = %d, want 1", count)
	}
}

func TestV085ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source, value string
		want                [][3]int
	}{
		{"reference", `"Audio\\track.mp3"`, `Audio\track.mp3`, nil},
		{"example", `"Audio\track.mp3"`, `Audio\track.mp3`, [][3]int{{3, 20, 21}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Добавлены только версия и заголовок; сам фрагмент не изменён.
			result := v085TestDocument(t, v085TestMedia(t, "audio", tc.source, tc.value, 2, 1))
			v085TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@media audio "+tc.source)
			v085TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV085QuotedContents(t *testing.T) {
	for _, tc := range []struct {
		name, source, value string
		want                [][3]int
	}{
		{"plain", `"track.mp3"`, "track.mp3", nil},
		{"all allowed pairs", `"\\\"\{\}"`, `\"{}`, nil},
		{"one slash", `"\q"`, `\q`, [][3]int{{3, 15, 16}}},
		{"two slashes", `"\\q"`, `\q`, nil},
		{"three slashes", `"\\\q"`, `\\q`, [][3]int{{3, 17, 18}}},
		{"four slashes", `"\\\\q"`, `\\q`, nil},
		{"two violations", `"\q\z"`, `\q\z`, [][3]int{{3, 15, 16}, {3, 17, 18}}},
		{"space", `"\ "`, `\ `, [][3]int{{3, 15, 16}}},
		{"tab", "\"\\\t\"", "\\\t", [][3]int{{3, 15, 16}}},
		{"nbsp", "\"\\\u00a0\"", "\\\u00a0", [][3]int{{3, 15, 16}}},
		{"cyrillic", `"\Ж"`, `\Ж`, [][3]int{{3, 15, 16}}},
		{"emoji", `"\😀"`, `\😀`, [][3]int{{3, 15, 16}}},
		{"no special C escapes", `"\n\t"`, `\n\t`, [][3]int{{3, 15, 16}, {3, 17, 18}}},
		{"unicode positions", `"Я😀\q"`, `Я😀\q`, [][3]int{{3, 17, 18}}},
		{"escaped quote does not close", `"\"\q"`, `"\q`, [][3]int{{3, 17, 18}}},
		{"paired slash does not escape closer", `"\\"`, `\`, nil},
		{"unquoted", `x\q`, `x\q`, nil},
		{"numeric", "012", "012", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Неизвестные escape-пары сохраняются по DSL 1.22:
			// это вход для V085, а не выдуманная ошибка парсера.
			v085TestRun(t, v085TestDocument(t, v085TestMedia(t, "audio", tc.source, tc.value, 2, 1)), "current", tc.want)
		})
	}
	for _, kind := range []string{"audio", "video", "image"} {
		t.Run(kind, func(t *testing.T) {
			v085TestRun(t, v085TestDocument(t, v085TestMedia(t, kind, `"\q"`, `\q`, 2, 1)),
				"current", [][3]int{{3, 15, 16}})
		})
	}
	t.Run("empty quoted path is another problem", func(t *testing.T) {
		result := v085TestDocument(t, v085TestMedia(t, "audio", `""`, "", 2, 1))
		v085TestParserError(t, result, 3, 2, "Пустой путь медиа.")
		v085TestRun(t, result, "current", nil)
	})
}

func TestV085UnparsedAndQuoteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"unterminated with violation", `@media audio "\q`, [][3]int{{3, 15, 16}}},
		{"terminal slash", `@media audio "x\`, [][3]int{{3, 16, 17}}},
		{"unterminated plain", `@media audio "plain`, nil},
		{"opening quote only", `@media audio "`, nil},
		{"terminal escaped quote", `@media audio "\"`, nil},
		{"terminal pair", `@media audio "\\`, nil},
		{"tail after closer", `@media audio "ok" \q`, nil},
		{"tail after paired slashes and closer", `@media audio "\\" \q`, nil},
		{"later quotes are not source", `@media audio x "\q"`, nil},
	} {
		for _, partial := range []bool{false, true} {
			t.Run(tc.name+"/partial="+strconv.FormatBool(partial), func(t *testing.T) {
				line := v085TestInvalid(t, tc.raw, 2, 1)
				if partial {
					line = v085TestLine(t, tc.raw, "invalid", 2, 1,
						v085TestElement(t, "tag", "@media", "media", 1),
						v085TestElement(t, "media-type", "audio", "audio", 8),
						v085TestUnparsed(t, tc.raw[13:], 14))
				}
				// Во всех случаях нарушены кавычки либо есть лишний аргумент.
				v085TestRun(t, v085TestDocument(t, line), "current", tc.want)
			})
		}
	}
}

func TestV085SeparatorsAndNames(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want [][3]int
	}{
		{"@MEDIA\tAUDIO\t\"\\q\"", [][3]int{{3, 15, 16}}},
		{"@media\taudio \"\\q\"", [][3]int{{3, 15, 16}}},
		{"@media audio\t\"\\q\"", [][3]int{{3, 15, 16}}},
		{"@media \t audio \t \"\\q\"", [][3]int{{3, 19, 20}}},
		{"@media\u00a0audio \"\\q\"", nil},
		{"@media audio\u00a0\"\\q\"", nil},
		{"@media audio \u00a0\"\\q\"", nil},
		{"@media\u2003audio \"\\q\"", nil},
		{`@mediaExtra audio "\q"`, nil},
		{`@media audioExtra "\q"`, nil},
		{`@media sound "\q"`, nil},
		{`@media "audio" "\q"`, nil},
		{`@media "\q"`, nil},
		{"@media", nil},
		{"@media audio", nil},
		{`@media audio"\q"`, nil},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			// Ошибочные разделители, имя, тип или отсутствующий аргумент.
			v085TestRun(t, v085TestDocument(t, v085TestInvalid(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
	t.Run("unparsed on tag line", func(t *testing.T) {
		v085TestRun(t, v085TestDocument(t,
			v085TestLine(t, "@media\taudio \"\\q\"", "tag", 2, 1,
				v085TestElement(t, "tag", "@media", "media", 1),
				v085TestUnparsed(t, "\taudio \"\\q\"", 7))), "current", [][3]int{{3, 15, 16}})
	})
	t.Run("indentation and case", func(t *testing.T) {
		v085TestRun(t, v085TestDocument(t,
			v085TestLine(t, " \t@MeDiA AuDiO \"\\q\"", "tag", 2, 1,
				v085TestElement(t, "tag", "@MeDiA", "media", 3),
				v085TestElement(t, "media-type", "AuDiO", "audio", 10),
				v085TestElement(t, "source", `"\q"`, `\q`, 16))), "current", [][3]int{{3, 17, 18}})
	})
}

func TestV085Contexts(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@media audio "\q"`, `@media audio "\q"`},
		{`See @media audio "\q"`, `See @media audio "\q"`},
		{"\u00a0@media audio \"\\q\"", "\u00a0@media audio \"\\q\""},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v085TestRun(t, v085TestDocument(t, v085TestText(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, tag := range []string{"note", "editor"} {
		t.Run("single line "+tag, func(t *testing.T) {
			v085TestRun(t, v085TestDocument(t,
				v085TestNamed(t, tag, "content", `Mention @media audio "\q"`, 2, 1)), "current", nil)
		})
	}
	t.Run("nested declaration", func(t *testing.T) {
		result := v085TestDocument(t, v085TestOpen(t, "text", 2, 1),
			v085TestMedia(t, "audio", `"\q"`, `\q`, 3, 2),
			v085TestText(t, "Read.", "Read.", 3, 2), v085TestClose(t, 3, 2))
		v085TestParserError(t, result, 4, -1, "Объявление media запрещено в text.")
		v085TestRun(t, result, "current", [][3]int{{4, 15, 16}})
	})
	t.Run("branch", func(t *testing.T) {
		v085TestRun(t, v085TestDocument(t, v085TestOpen(t, "variants", 2, 1),
			v085TestNamed(t, "variant", "name", "A", 3, 2),
			v085TestMedia(t, "audio", `"\q"`, `\q`, 4, 3),
			v085TestText(t, "Read.", "Read.", 4, 3),
			v085TestNamed(t, "variant", "name", "B", 3, 2),
			v085TestText(t, "Write.", "Write.", 7, 3), v085TestClose(t, 3, 2)),
			"current", [][3]int{{5, 15, 16}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		for _, parsed := range []bool{false, true} {
			t.Run(tag+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				open := v085TestOpen(t, tag, 2, 1)
				if !parsed {
					// Ошибочная граница имени не меняет сохранённый блок.
					name := "@" + strings.ToUpper(tag)
					open = v085TestLine(t, name+"{", "block-start", 2, 1,
						v085TestUnparsed(t, name, 1),
						v085TestElement(t, "block-open", "{", "{", len(name)+1))
				}
				v085TestRun(t, v085TestDocument(t, open,
					v085TestText(t, `@media audio "\q"`, `@media audio "\q"`, 3, 2),
					v085TestClose(t, 3, 2),
					v085TestMedia(t, "audio", `"\q"`, `\q`, 2, 1)),
					"current", [][3]int{{6, 15, 16}})
			})
		}
	}
	t.Run("resource directories are literal", func(t *testing.T) {
		result := v085TestDocument(t)
		result.Lines = []model.Line{result.Lines[0],
			v085TestLine(t, `@resource-dir "Audio\tracks"`, "tag", 0, 0,
				v085TestElement(t, "tag", "@resource-dir", "resource-dir", 1),
				v085TestElement(t, "resource-path", `"Audio\tracks"`, `Audio\tracks`, 15)),
			result.Lines[1]}
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		count := 3
		result.Document.LineCount = &count
		result.Document.Metadata.ResourceDirs = []string{`Audio\tracks`}
		v085TestRun(t, result, "current", nil)
	})
}

func TestV085EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v085TestDocument(t, v085TestMedia(t, "audio", `"Audio\track.mp3"`, `Audio\track.mp3`, 2, 1))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v085TestRun(t, result, "first", [][3]int{{3, 20, 21}})
			v085TestRun(t, result, "second", [][3]int{{3, 20, 21}})
		})
	}
	t.Run("each declaration once", func(t *testing.T) {
		v085TestRun(t, v085TestDocument(t,
			v085TestMedia(t, "audio", `"\q"`, `\q`, 2, 1),
			v085TestMedia(t, "audio", `"\q"`, `\q`, 2, 1)),
			"current", [][3]int{{3, 15, 16}, {4, 15, 16}})
	})
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v085TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v085TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v085TestDocument(t)
			result.Document.DSLVersion = &version
			before := v085TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v085TestRegistry(t))
			if err == nil || checks != nil || v085TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v085TestMedia принимает независимые исходное и семантическое значения пути.
func v085TestMedia(t *testing.T, kind, source, value string, parent, depth int) model.Line {
	t.Helper()
	return v085TestLine(t, "@media "+kind+" "+source, "tag", parent, depth,
		v085TestElement(t, "tag", "@media", "media", 1),
		v085TestElement(t, "media-type", kind, kind, 8),
		v085TestElement(t, "source", source, value, 14))
}

// v085TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v085TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v085TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v085TestHeading(t, "#", "Lesson"),
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
				v085TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v085TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v085TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v085TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v085TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v085TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v085TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v085TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v085TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v085TestLine(t, raw, "invalid", parent, depth, v085TestUnparsed(t, raw, 1))
}

func v085TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v085TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v085TestElement(t, "tag", "@"+tag, tag, 1), v085TestElement(t, kind, value, value, len(tag)+3))
}

func v085TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v085TestLine(t, marker+" "+title, "heading", 0, 0,
		v085TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v085TestElement(t, "title", title, title, len(marker)+2))
}

func v085TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v085TestLine(t, raw, "content", parent, depth, v085TestElement(t, "content", raw, value, 1))
}

func v085TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v085TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v085TestElement(t, "tag", "@"+tag, tag, 1), v085TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v085TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v085TestLine(t, "}", "block-end", parent, depth, v085TestElement(t, "block-close", "}", "}", 1))
}

func v085TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v085TestRun выполняет полный каталог и сравнивает новые V085 с явными
// тройками строка/начало/конец. Правые границы исключаются.
func v085TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v085TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v085TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v085TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V085" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V085 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "Литеральная обратная косая черта в пути медиа не удвоена.\nконкретизация: Для буквальной обратной косой черты используйте \\\\." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V085 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v085TestSnapshot(t, result)), &restored); err != nil {
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
	if v085TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v085TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v085TestSource(t *testing.T, result *model.Result, want string) {
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

package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v082TestMessage = "Путь медиа в кавычках пуст или состоит только из пробелов."

// v082TestWant задаёт независимый диапазон с исключающей правой границей.
type v082TestWant struct{ line, start, end int }

func TestV082RegistrationAndVersions(t *testing.T) {
	registry := v082TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V082" {
			continue
		}
		count++
		if definition.Message != v082TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V082 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.1.6"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v082TestDocument(t)
			result.Document.DSLVersion = &version
			before := v082TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v082TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV082ReferenceAndExample(t *testing.T) {
	// Фрагменты сохранены дословно на строке 4. Версия, основной заголовок
	// и заголовок шага перед ними обеспечивают корректный внешний контекст.
	t.Run("reference", func(t *testing.T) {
		result := v082TestDocument(t, v082TestMedia(t, "audio", `"12"`, "12", 3))
		if result.Lines[3].Raw != `@media audio "12"` {
			t.Fatal("reference changed")
		}
		v082TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v082TestDocument(t, v082TestMedia(t, "audio", `""`, "", 3))
		v082TestParserError(t, result, 4, 2, "Пустой путь.")
		if result.Lines[3].Raw != `@media audio ""` {
			t.Fatal("example changed")
		}
		v082TestRun(t, result, "current", []v082TestWant{{4, 14, 16}})
	})
}

func TestV082WhitespaceAndTypes(t *testing.T) {
	// Перечень независим от unicode.White_Space реализации. CR и LF
	// являются окончаниями физических строк и не могут находиться внутри Raw.
	whitespace := []string{"\t", "\v", "\f", " ", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001",
		"\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a",
		"\u2028", "\u2029", "\u202f", "\u205f", "\u3000"}
	for _, space := range whitespace {
		t.Run(strconv.Quote(space), func(t *testing.T) {
			result := v082TestDocument(t, v082TestMedia(t, "audio", `"`+space+`"`, space, 3))
			v082TestParserError(t, result, 4, 2, "Путь содержит только пробельный символ.")
			v082TestRun(t, result, "current", []v082TestWant{{4, 14, 17}})
		})
	}
	for _, mediaType := range []string{"audio", "video", "image", "AUDIO", "Video", "iMaGe"} {
		for _, tc := range []struct {
			value string
			end   int
		}{{"", 16}, {"   ", 19}, {" \t\u00a0 ", 20}} {
			t.Run(mediaType+strconv.Quote(tc.value), func(t *testing.T) {
				result := v082TestDocument(t, v082TestMedia(t, mediaType, `"`+tc.value+`"`, tc.value, 3))
				v082TestParserError(t, result, 4, 2, "Пустой или пробельный путь.")
				v082TestRun(t, result, "current", []v082TestWant{{4, 14, tc.end}})
			})
		}
	}
}

func TestV082NonemptyAndEscapes(t *testing.T) {
	for _, tc := range []struct{ source, value string }{
		{`"x"`, "x"}, {`" 12 "`, " 12 "}, {`" . "`, " . "}, {`" Я😀 "`, " Я😀 "},
		{"\"\u200b\"", "\u200b"}, {"\"\ufeff\"", "\ufeff"},
		{`"\""`, `"`}, {`"\\"`, `\`}, {`"\\\\"`, `\\`}, {`"\\\""`, `\"`},
		{`"\{"`, "{"}, {`"\}"`, "}"}, {`"\t"`, `\t`}, {`"\n"`, `\n`}, {`"\q"`, `\q`},
		{`"\ "`, `\ `}, {`"&#32;"`, "&#32;"}, {`"  a\\b\"c\{d\}  "`, `  a\b"c{d}  `},
		{`12`, "12"}, {`file.mp3`, "file.mp3"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			v082TestRun(t, v082TestDocument(t, v082TestMedia(t, "audio", tc.source, tc.value, 3)), "current", nil)
		})
	}
}

func TestV082UnparsedAndTails(t *testing.T) {
	for _, tc := range []struct {
		name, remainder string
		want            []v082TestWant
	}{
		{"empty", `""`, []v082TestWant{{4, 14, 16}}},
		{"spaces", `"   "`, []v082TestWant{{4, 14, 19}}},
		{"text tail", `"" extra`, []v082TestWant{{4, 14, 16}}},
		{"quoted tail", `"" "x"`, []v082TestWant{{4, 14, 16}}},
		{"unclosed tail", `"" "`, []v082TestWant{{4, 14, 16}}},
		{"escaped tail", `"" \"`, []v082TestWant{{4, 14, 16}}},
		{"attached tail", `""x`, []v082TestWant{{4, 14, 16}}},
		{"empty pair only in tail", `"x" ""`, nil},
		{"unquoted source then empty pair", `file.mp3 ""`, nil},
		{"only opening quote", `"`, nil}, {"unclosed whitespace", `"   `, nil},
		{"escaped last quote", `" \"`, nil}, {"three backslashes before last quote", `"\\\"`, nil},
		{"last backslash", `"\`, nil}, {"last backslash pair", `"\\`, nil},
	} {
		for _, mode := range []string{"source remainder", "type remainder", "whole declaration"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				raw := "@media audio " + tc.remainder
				var elements []model.Element
				switch mode {
				case "source remainder":
					elements = []model.Element{v082TestElement(t, "tag", "@media", "media", 1),
						v082TestElement(t, "media-type", "audio", "audio", 8),
						v082TestElement(t, "unparsed", tc.remainder, "", 14)}
				case "type remainder":
					elements = []model.Element{v082TestElement(t, "tag", "@media", "media", 1),
						v082TestElement(t, "unparsed", "audio "+tc.remainder, "", 8)}
				case "whole declaration":
					elements = []model.Element{v082TestElement(t, "unparsed", raw, "", 1)}
				}
				result := v082TestDocument(t, v082TestLine(t, raw, "invalid", 3, elements...))
				v082TestRun(t, result, "current", tc.want)
			})
		}
	}
	t.Run("parsed source followed by unparsed tail", func(t *testing.T) {
		line := v082TestMedia(t, "audio", `""`, "", 3)
		line.Raw += ` "x"`
		line.Elements = append(line.Elements, v082TestElement(t, "unparsed", `"x"`, "", 17))
		line.LineType = "invalid"
		result := v082TestDocument(t, line)
		v082TestParserError(t, result, 4, 2, "Пустой путь.")
		v082TestRun(t, result, "current", []v082TestWant{{4, 14, 16}})
	})
	t.Run("next line cannot close source", func(t *testing.T) {
		result := v082TestDocument(t, v082TestLine(t, `@media audio "   `, "invalid", 3,
			v082TestElement(t, "tag", "@media", "media", 1),
			v082TestElement(t, "media-type", "audio", "audio", 8),
			v082TestElement(t, "unparsed", `"   `, "", 14)), v082TestText(t, `"`, `"`, 3))
		v082TestRun(t, result, "current", nil)
	})
}

func TestV082PositionsAndExcludedDeclarations(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		start    int
		elements []model.Element
	}{
		{"\t @MEDIA AUDIO \"\"", 16, []model.Element{
			v082TestElement(t, "tag", "@MEDIA", "media", 3), v082TestElement(t, "media-type", "AUDIO", "audio", 10),
			v082TestElement(t, "source", `""`, "", 16)}},
		{`@media  audio  ""`, 16, []model.Element{
			v082TestElement(t, "tag", "@media", "media", 1), v082TestElement(t, "media-type", "audio", "audio", 9),
			v082TestElement(t, "source", `""`, "", 16)}},
		{"@media audio \"\" \t", 14, []model.Element{
			v082TestElement(t, "tag", "@media", "media", 1), v082TestElement(t, "media-type", "audio", "audio", 8),
			v082TestElement(t, "source", `""`, "", 14)}},
	} {
		for _, whole := range []bool{false, true} {
			t.Run(tc.raw+" whole="+strconv.FormatBool(whole), func(t *testing.T) {
				line := v082TestLine(t, tc.raw, "tag", 3, tc.elements...)
				if whole {
					line = v082TestLine(t, tc.raw, "invalid", 3, v082TestElement(t, "unparsed", tc.raw, "", 1))
				}
				result := v082TestDocument(t, line)
				if !whole {
					v082TestParserError(t, result, 4, 2, "Пустой путь.")
				}
				v082TestRun(t, result, "current", []v082TestWant{{4, tc.start, tc.start + 2}})
			})
		}
	}
	for _, raw := range []string{`@media sound ""`, `@media ""`, `@media`, `@media audio`,
		`@media audioExtra ""`, `@mediaExtra audio ""`, `@media image { "" }`} {
		t.Run(raw, func(t *testing.T) {
			v082TestRun(t, v082TestDocument(t, v082TestLine(t, raw, "invalid", 3,
				v082TestElement(t, "unparsed", raw, "", 1))), "current", nil)
		})
	}
	t.Run("recognized invalid media type", func(t *testing.T) {
		kind := v082TestElement(t, "media-type", "sound", "", 8)
		kind.Value = nil
		result := v082TestDocument(t, v082TestLine(t, `@media sound ""`, "invalid", 3,
			v082TestElement(t, "tag", "@media", "media", 1), kind, v082TestElement(t, "unparsed", `""`, "", 14)))
		v082TestParserError(t, result, 4, 1, "Неизвестный тип медиа.")
		v082TestRun(t, result, "current", nil)
	})
	for _, tc := range []struct{ raw, value string }{
		{`\@media audio ""`, `@media audio ""`}, {`Text @media audio ""`, `Text @media audio ""`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v082TestRun(t, v082TestDocument(t, v082TestText(t, tc.raw, tc.value, 3)), "current", nil)
		})
	}
	t.Run("another recognized tag", func(t *testing.T) {
		v082TestRun(t, v082TestDocument(t, v082TestTag(t, "instr", `Say "".`, "content", 3)), "current", nil)
	})
}

func TestV082NestedAndLiteralContent(t *testing.T) {
	for _, container := range []string{"fragment", "text"} {
		t.Run(container, func(t *testing.T) {
			argument, kind := "", model.ElementType("")
			if container == "fragment" {
				argument, kind = "note", "identifier"
			}
			result := v082TestDocument(t, v082TestOpen(t, container, argument, kind, 3),
				v082TestMedia(t, "audio", `""`, "", 4), v082TestClose(t, 4), v082TestText(t, "Read.", "Read.", 3))
			v082TestParserError(t, result, 5, 2, "Пустой путь.")
			if container == "text" {
				v082TestParserError(t, result, 5, 0, "Вложение media запрещено внутри text.")
			}
			v082TestRun(t, result, "current", []v082TestWant{{5, 14, 16}})
		})
	}
	for _, container := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(container, func(t *testing.T) {
			result := v082TestDocument(t, v082TestTag(t, "task", "practice", "identifier", 3),
				v082TestTag(t, "step", "Prepare", "title", 4), v082TestOpen(t, container, "", "", 5),
				v082TestText(t, `@media audio ""`, `@media audio ""`, 6),
				v082TestText(t, `@media image "   "`, `@media image "   "`, 6), v082TestClose(t, 6),
				v082TestText(t, "Read.", "Read.", 5))
			v082TestRun(t, result, "current", nil)
		})
	}
}

func TestV082MultipleEndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v082TestDocument(t, v082TestMedia(t, "audio", `""`, "", 3),
				v082TestMedia(t, "image", `"Я😀"`, "Я😀", 3), v082TestMedia(t, "video", `""`, "", 3))
			v082TestParserError(t, result, 4, 2, "Пустой путь.")
			v082TestParserError(t, result, 6, 2, "Пустой путь.")
			if eol == "" {
				result.Lines[5].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v082TestWant{{4, 14, 16}, {6, 14, 16}}
			v082TestRun(t, result, "first", want)
			v082TestRun(t, v082TestDocument(t, v082TestMedia(t, "audio", `"12"`, "12", 3)), "other", nil)
			v082TestRun(t, result, "second", want)
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v082TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v082TestRun(t, result, "current", nil)
		})
	}
}

// v082TestMedia задаёт готовые элементы media без разбора или декодирования пути.
func v082TestMedia(t *testing.T, mediaType, source, value string, parent int) model.Line {
	t.Helper()
	return v082TestLine(t, "@media "+mediaType+" "+source, "tag", parent,
		v082TestElement(t, "tag", "@media", "media", 1),
		v082TestElement(t, "media-type", mediaType, strings.ToLower(mediaType), 8),
		v082TestElement(t, "source", source, value, 14))
}

// v082TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v082TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v082TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v082TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v082TestTag задаёт известное имя и явно указанный тип аргумента.
func v082TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v082TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v082TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v082TestLine(t, raw, "tag", parent, elements...)
}

// v082TestOpen добавляет открытие к известному объявлению блока.
func v082TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v082TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v082TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v082TestClose задаёт закрытие конкретного фигурного блока.
func v082TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v082TestLine(t, "}", "block-end", parent, v082TestElement(t, "block-close", "}", "}", 1))
}

// v082TestText сохраняет исходное и семантическое содержимое независимо.
func v082TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v082TestLine(t, raw, "content", parent, v082TestElement(t, "content", raw, value, 1))
}

// v082TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v082TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v082TestTag(t, "dsl-version", "1.2", "version", 0),
		v082TestLine(t, "# Lesson", "heading", 0, v082TestElement(t, "heading-level", "#", "1", 1),
			v082TestElement(t, "title", "Lesson", "Lesson", 3)),
		v082TestLine(t, "### Practice", "heading", 0, v082TestElement(t, "heading-level", "###", "3", 1),
			v082TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			line.Elements[j].ErrorIDs = append([]string{}, line.Elements[j].ErrorIDs...)
			if line.Elements[j].ElementType == "unparsed" {
				v082TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v082TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v082TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	// Все записи здесь принадлежат исходному парсеру. Контракт требует
	// порядка по позиции, даже если ошибки элементов добавлялись позднее.
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v082TestRegistry использует полный штатный каталог без отдельной регистрации.
func v082TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v082TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v082TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v082TestRun сравнивает новые V082 после штатных Prepare/Run с явным ожиданием.
func v082TestRun(t *testing.T, result *model.Result, processingID string, want []v082TestWant) {
	t.Helper()
	before := v082TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v082TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v082TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V082" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V082 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v082TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V082 occurrence %d = %+v, want %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V082 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V082 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v082TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Из сравнения исключаются только добавления конвейера и производные
	// флаги/ссылки. Исходные строки, метаданные и прежняя история сохраняются.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v082TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

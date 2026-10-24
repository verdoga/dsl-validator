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

func TestV058Registration(t *testing.T) {
	definitions, err := v058Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V058" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У заголовка отсутствует название." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V058 definitions = %d, want 1", count)
	}
}

func TestV058ReferenceAndExample(t *testing.T) {
	// DSL 1.30: перед неизменёнными фрагментами добавлены только
	// обязательные @dsl-version 1.2 и # Lesson. Фрагменты начинаются со строки 3.
	t.Run("reference", func(t *testing.T) {
		v058Run(t, v058Lesson(t, v058Heading(t, "###", "Practice"),
			v058Text(t, "Read.", "Read.", 3, 1)), "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		heading := v058Line(t, "###", "heading", 0, 0,
			v058Element(t, "heading-level", "###", "3", 1, 4),
			v058Element(t, "unparsed", "", "", 4, 4))
		v058Run(t, v058Lesson(t, heading, v058Text(t, "Read.", "Read.", 3, 1)),
			"current", [][3]int{{3, 1, 4}})
	})
}

func TestV058MarkersAndEmptyTails(t *testing.T) {
	t.Run("first level without another first level", func(t *testing.T) {
		heading := v058Line(t, "#", "heading", 0, 0,
			v058Element(t, "heading-level", "#", "1", 1, 2),
			v058Element(t, "unparsed", "", "", 2, 2))
		v058Run(t, v058Document(t, "", heading, v058Text(t, "Read.", "Read.", 2, 1)),
			"current", [][3]int{{2, 1, 2}})
	})
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"second level", "##", [][3]int{{3, 1, 3}}},
		{"third level", "###", [][3]int{{3, 1, 4}}},
		{"fourth excluded", "####", nil},
		{"fifth excluded", "#####", nil},
		{"long run excluded", strings.Repeat("#", 64), nil},
		{"fourth with spaces excluded", "#### \t", nil},
		{"indentation", " \t###", [][3]int{{3, 3, 6}}},
		{"tab is one column", "\t###", [][3]int{{3, 2, 5}}},
		{"ordinary spaces", "###   ", [][3]int{{3, 1, 4}}},
		{"tab after marker", "###\t", [][3]int{{3, 1, 4}}},
		{"mixed whitespace", "### \t ", [][3]int{{3, 1, 4}}},
		{"missing separator but text exists", "###Practice", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Ошибочная попытка сохранена целиком, без heading-level/title.
			result := v058Lesson(t, v058Invalid(t, tc.raw, 2, 1), v058Text(t, "Read.", "Read.", 2, 1))
			v058Run(t, result, "current", tc.want)
		})
	}
}

func TestV058UnicodeWhitespace(t *testing.T) {
	// Все White_Space, допустимые в Raw, перечислены независимо от реализации.
	// CR/LF принадлежат EOL и проверяются отдельно. Смесь проверяет весь остаток.
	spaces := []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'}
	for _, space := range spaces {
		t.Run(strconv.QuoteRune(space), func(t *testing.T) {
			v058Run(t, v058Lesson(t, v058Invalid(t, "###"+string(space), 2, 1)),
				"current", [][3]int{{3, 1, 4}})
		})
	}
	t.Run("mixture", func(t *testing.T) {
		v058Run(t, v058Lesson(t, v058Invalid(t, "###"+string(spaces), 2, 1)),
			"current", [][3]int{{3, 1, 4}})
	})
}

func TestV058AnyNonWhitespaceTitle(t *testing.T) {
	for _, title := range []string{"A", "7", ".", "#", `\`, "\u200b", "Практика", "PRACTICE", "😀e\u0301"} {
		t.Run(strconv.Quote(title), func(t *testing.T) {
			v058Run(t, v058Lesson(t, v058Heading(t, "###", title),
				v058Text(t, "Read.", "Read.", 3, 1)), "current", nil)
		})
	}
	t.Run("nonspace surrounded by whitespace", func(t *testing.T) {
		// Краевые U+0020/U+0009 отсутствуют в title.Value, но остаются в Raw.
		line := v058Line(t, "### \t A \t", "heading", 0, 0,
			v058Element(t, "heading-level", "###", "3", 1, 4),
			v058Element(t, "title", "A", "A", 7, 8))
		v058Run(t, v058Lesson(t, line, v058Text(t, "Read.", "Read.", 3, 1)), "current", nil)
	})
	for _, tc := range []struct {
		name, raw, tail string
		start, end      int
		want            [][3]int
	}{
		{"nonspace unparsed without separator", "###Practice", "Practice", 4, 12, nil},
		{"nonspace after whitespace in unparsed", "###\tPractice", "\tPractice", 4, 13, nil},
		{"whitespace unparsed", "### \t", " \t", 4, 6, [][3]int{{3, 1, 4}}},
		{"empty unparsed", "###", "", 4, 4, [][3]int{{3, 1, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v058Line(t, tc.raw, "heading", 0, 0,
				v058Element(t, "heading-level", "###", "3", 1, 4),
				v058Element(t, "unparsed", tc.tail, "", tc.start, tc.end))
			v058Run(t, v058Lesson(t, line, v058Text(t, "Read.", "Read.", 3, 1)), "current", tc.want)
		})
	}
}

func TestV058ExcludedPrefixes(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{"Text ###", "Text ###"}, {"＃＃＃", "＃＃＃"},
		{`\###`, "###"}, {" \t\\###", "###"}, {`\\###`, `\###`},
		{"\u00a0###", "\u00a0###"}, {" \t\u2003###", "\u2003###"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v058Run(t, v058Lesson(t, v058Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v058Run(t, v058Lesson(t, v058Line(t, raw, "blank", 2, 1)), "current", nil)
		})
	}
}

func TestV058NestedAttempts(t *testing.T) {
	t.Run("text block", func(t *testing.T) {
		result := v058Lesson(t, v058Open(t, "text", 2, 1), v058Invalid(t, "###", 3, 2),
			v058Text(t, "Read.", "Read.", 3, 2), v058Close(t, 3, 2))
		v058Run(t, result, "current", [][3]int{{4, 1, 4}})
	})
	t.Run("text block in task", func(t *testing.T) {
		result := v058Lesson(t, v058Tag(t, "task", "a", "identifier", 2, 1),
			v058Tag(t, "instr", "Read.", "content", 3, 2),
			v058Open(t, "text", 3, 2), v058Invalid(t, "###", 5, 3),
			v058Text(t, "Read.", "Read.", 5, 3), v058Close(t, 5, 3),
			v058Tag(t, "endtask", "", "", 2, 1))
		v058Run(t, result, "current", [][3]int{{6, 1, 4}})
	})
}

func TestV058LiteralContexts(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			t.Run("contents", func(t *testing.T) {
				v058Run(t, v058Lesson(t, v058Open(t, tag, 2, 1),
					v058Text(t, "###", "###", 3, 2), v058Close(t, 3, 2)), "current", nil)
			})
			t.Run("after closing", func(t *testing.T) {
				v058Run(t, v058Lesson(t, v058Open(t, tag, 2, 1),
					v058Text(t, "###", "###", 3, 2), v058Close(t, 3, 2),
					v058Invalid(t, "###", 2, 1)), "current", [][3]int{{6, 1, 4}})
			})
			t.Run("inside task", func(t *testing.T) {
				result := v058Lesson(t, v058Tag(t, "task", "a", "identifier", 2, 1),
					v058Tag(t, "instr", "Read.", "content", 3, 2),
					v058Open(t, tag, 3, 2), v058Text(t, "###", "###", 5, 3), v058Close(t, 5, 3),
					v058Tag(t, "endtask", "", "", 2, 1))
				v058Run(t, result, "current", nil)
			})
			t.Run("single line form", func(t *testing.T) {
				v058Run(t, v058Lesson(t, v058Tag(t, tag, "Write.", "content", 2, 1),
					v058Invalid(t, "###", 2, 1)), "current", [][3]int{{4, 1, 4}})
			})
		})
	}
}

func TestV058MultipleOccurrences(t *testing.T) {
	result := v058Lesson(t, v058Invalid(t, "###", 2, 1), v058Text(t, "Read.", "Read.", 2, 1),
		v058Invalid(t, "###", 2, 1), v058Text(t, "Write.", "Write.", 2, 1))
	v058Run(t, result, "current", [][3]int{{3, 1, 4}, {5, 1, 4}})
}

func TestV058IndependentAndRepeatedRuns(t *testing.T) {
	result := v058Lesson(t, v058Invalid(t, "###", 2, 1), v058Text(t, "Read.", "Read.", 2, 1))
	v058Run(t, result, "first", [][3]int{{3, 1, 4}})
	v058Run(t, v058Lesson(t, v058Heading(t, "###", "Practice"),
		v058Text(t, "Read.", "Read.", 3, 1)), "independent", nil)
	v058Run(t, result, "second", [][3]int{{3, 1, 4}})
}

func TestV058EndingsAndEmptyInput(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v058Lesson(t, v058Invalid(t, "###", 2, 1))
			if ending == "\r\n" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			result.Lines[2].LineEnding = ending
			v058Run(t, result, "current", [][3]int{{3, 1, 4}})
		})
	}
	t.Run("version only", func(t *testing.T) {
		v058Run(t, v058Document(t, ""), "current", nil)
	})
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version+" with unavailable lines", func(t *testing.T) {
			// Версия известна из модели; исходные строки недоступны.
			result := v058Document(t, "")
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v058Run(t, result, "current", nil)
				return
			}
			before := v058Snapshot(t, result)
			checks, err := validator.Prepare(result, v058Registry(t))
			if err == nil || checks != nil || v058Snapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v058Lesson добавляет обязательный заголовок; body начинается со строки 3.
func v058Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v058Document(t, "Lesson", append([]model.Line{v058Heading(t, "#", "Lesson")}, body...)...)
}

// v058Document добавляет строку версии и согласованные ошибки для явно заданных
// unparsed. Не разбирает Raw и не вычисляет ожидаемые результаты диагностики.
func v058Document(t *testing.T, title string, body ...model.Line) *model.Result {
	t.Helper()
	version := "1.2"
	lines := append([]model.Line{v058Tag(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
	if title != "" {
		result.Document.Metadata.Title = &title
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора заголовка.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v058Heading задаёт корректный корневой заголовок с явно указанным названием.
func v058Heading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v058Line(t, marker+" "+title, "heading", 0, 0,
		v058Element(t, "heading-level", marker, strconv.Itoa(len(marker)), 1, len(marker)+1),
		v058Element(t, "title", title, title, len(marker)+2, len(marker)+2+utf8.RuneCountInString(title)))
}

// v058Invalid задаёт неразобранную ошибочную попытку, сохраняя весь исходный текст.
func v058Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v058Line(t, raw, "invalid", parent, depth, v058Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v058Text задаёт Raw и семантическое значение независимо.
func v058Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v058Line(t, raw, "content", parent, depth, v058Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v058Tag задаёт известный однострочный тег и необязательный аргумент.
func v058Tag(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v058Element(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v058Element(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v058Line(t, raw, "tag", parent, depth, elements...)
}

// v058Open задаёт блок с каноническим семантическим именем тега.
func v058Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v058Line(t, "@"+tag+" {", "block-start", parent, depth,
		v058Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v058Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v058Close задаёт закрытие блока с явным родителем.
func v058Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v058Line(t, "}", "block-end", parent, depth, v058Element(t, "block-close", "}", "}", 1, 2))
}

// v058Line сохраняет явно заданную структуру без классификации исходного текста.
func v058Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v058Element задаёт исходный диапазон элемента с исключающим концом.
func v058Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v058Registry создаёт реестр из полного штатного каталога.
func v058Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v058Run проверяет новые V058 по явным тройкам строка/начало/исключающий конец.
// Конвейер выполняет полный каталог; ожидания не зависят от чужих диагностик.
func v058Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v058Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v058Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v058Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V058" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V058 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "У заголовка отсутствует название." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V058 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v058Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// CheckModel проверил производные флаги/ссылки. В копии восстанавливаем
	// только их и убираем новые записи, чтобы сравнить весь исходник и историю.
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
	if v058Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v058Snapshot сохраняет независимое представление модели для сравнения.
func v058Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

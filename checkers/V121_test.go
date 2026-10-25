package checkers

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v121TestMessage задан независимо от констант проверяемой диагностики.
const v121TestMessage = "В инструкции есть вопросительный знак.\nконкретизация: Проверьте, требуется ли на этот вопрос отдельный ответ пользователя. Если требуется отдельное поле, оформите вопрос через @question. Вопросительная формулировка самой инструкции допустима"

func TestV121Registration(t *testing.T) {
	registry := v121TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V121" {
			continue
		}
		count++
		if definition.Message != "В инструкции есть вопросительный знак." ||
			definition.Severity != "recommendation" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V121 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V121 definitions = %d, want 1", count)
	}
	// Соседние версии не поддерживаются самим конвейером; каталог не расширяем.
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("ForVersion(%q) = %+v, %v; want rejection", version, definitions, err)
		}
	}
}

func TestV121ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][3]int
	}{
		{"reference", "@instr Read the text.\n@question Where does Tom live?", []model.Line{
			v121TestInline(t, "instr", "Read the text.", "Read the text.", 2, 1),
			v121TestInline(t, "question", "Where does Tom live?", "Where does Tom live?", 2, 1),
		}, nil},
		{"example", "@instr Where does Tom live? What does he do?", []model.Line{
			v121TestInline(t, "instr", "Where does Tom live? What does he do?", "Where does Tom live? What does he do?", 2, 1),
		}, [][3]int{{3, 8, 28}, {3, 29, 45}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v121TestLesson(t, tc.body...)
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// Добавлены только обязательные версия и заголовок. По п. 4.2.9
			// вопрос без answer допустим; сам эталон не дополнен ответом.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v121TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV121InlineQuestions(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"one question", "Go?", "Go?", [][3]int{{3, 8, 11}}},
		{"period", "Go.", "Go.", nil},
		{"unfinished", "Go", "Go", nil},
		{"no letters", "123? ??? !?", "123? ??? !?", nil},
		{"no preceding text", "?!???", "?!???", nil},
		{"symbol is not a letter", "😀?", "😀?", nil},
		{"one letter", "1a?", "1a?", [][3]int{{3, 8, 11}}},
		{"lower case", "go?", "go?", [][3]int{{3, 8, 11}}},
		{"groups", "Go?! Next???", "Go?! Next???", [][3]int{{3, 8, 12}, {3, 13, 20}}},
		{"all terminal signs", "Go...?!", "Go...?!", [][3]int{{3, 8, 15}}},
		{"previous period and exclamation", "Stop. Go! Why?", "Stop. Go! Why?", [][3]int{{3, 18, 22}}},
		{"no separating space", "Go?Why?", "Go?Why?", [][3]int{{3, 8, 11}, {3, 11, 15}}},
		{"leading group", "?! Go?", "?! Go?", [][3]int{{3, 11, 14}}},
		{"letter does not cross period", "Go. 123?", "Go. 123?", nil},
		{"letter does not cross question", "Go? 123?", "Go? 123?", [][3]int{{3, 8, 11}}},
		{"quote after question excluded", `"Go?"`, `"Go?"`, [][3]int{{3, 8, 12}}},
		{"colon is not a boundary", "Read: why?", "Read: why?", [][3]int{{3, 8, 18}}},
		{"ellipsis rune is not a boundary", "Go… Why?", "Go… Why?", [][3]int{{3, 8, 16}}},
		{"abbreviation has no exception", "e.g. Why?", "e.g. Why?", [][3]int{{3, 13, 17}}},
		{"NBSP before text", "\u00a0Go?", "\u00a0Go?", [][3]int{{3, 9, 12}}},
		{"Cyrillic and emoji columns", "Я😀?", "Я😀?", [][3]int{{3, 8, 11}}},
		{"Chinese letter", "你?", "你?", [][3]int{{3, 8, 10}}},
		{"combining mark preserved", "е\u0301?", "е\u0301?", [][3]int{{3, 8, 11}}},
		{"fullwidth sign is not ASCII question", "Go？", "Go？", nil},
		{"Arabic sign is not ASCII question", "Go؟", "Go؟", nil},
		{"no entity decoding", "Go&#63;", "Go&#63;", nil},
		{"escaped braces", `\{Go\}?`, `{Go}?`, [][3]int{{3, 8, 15}}},
		{"unknown question escape", `Go\?`, `Go\?`, [][3]int{{3, 8, 12}}},
		{"paired slashes", `Go\\?`, `Go\?`, [][3]int{{3, 8, 13}}},
		{"three slashes", `Go\\\?`, `Go\\?`, [][3]int{{3, 8, 14}}},
		{"slash separates groups", `Go?\?`, `Go?\?`, [][3]int{{3, 8, 11}}},
		{"inline at sign is not line start", `\@Go?`, `\@Go?`, [][3]int{{3, 8, 13}}},
		{"inline hash is not line start", `\#Go?`, `\#Go?`, [][3]int{{3, 8, 13}}},
		{"single underscore not escaped", `\_Go?`, `\_Go?`, [][3]int{{3, 8, 13}}},
		{"escaped leading slash", `\\Go?`, `\Go?`, [][3]int{{3, 8, 13}}},
		{"trailing slash is literal", `Go?\`, `Go?\`, [][3]int{{3, 8, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v121TestRun(t, v121TestLesson(t, v121TestInline(t, "instr", tc.raw, tc.value, 2, 1)), "current", tc.want)
		})
	}
}

func TestV121OriginalPositionsAndBlockEscapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"tabs and trailing spaces", "\tGo\t now?  ", "Go\t now?", [][3]int{{4, 2, 10}}},
		{"leading escaped at sign", `\@Go?`, `@Go?`, [][3]int{{4, 1, 6}}},
		{"leading escaped hash", `\#Go?`, `#Go?`, [][3]int{{4, 1, 6}}},
		{"indent before escaped at sign", " \t\\@Go?", "@Go?", [][3]int{{4, 3, 8}}},
		{"at sign after text", `Read \@Go?`, `Read \@Go?`, [][3]int{{4, 1, 11}}},
		{"two underscores", `\__ Go?`, `__ Go?`, [][3]int{{4, 1, 8}}},
		{"five underscores", `\_____ Go?`, `_____ Go?`, [][3]int{{4, 1, 11}}},
		{"six underscores", `\______ Go?`, `______ Go?`, [][3]int{{4, 1, 12}}},
		{"escaped closing brace first", `\}Go?`, `}Go?`, [][3]int{{4, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v121TestRun(t, v121TestLesson(t, v121TestOpen(t, "instr", 2, 1),
				v121TestText(t, tc.raw, tc.value, 3, 2), v121TestClose(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("inline margins and internal spaces", func(t *testing.T) {
		line := v121TestLine(t, "  @instr Go  now?  ", "tag", 2, 1,
			v121TestElement(t, "tag", "@instr", "instr", 3, 9),
			v121TestElement(t, "content", "Go  now?", "Go  now?", 10, 18))
		v121TestRun(t, v121TestLesson(t, line), "current", [][3]int{{3, 10, 18}})
	})
	t.Run("uppercase tag has canonical value", func(t *testing.T) {
		line := v121TestLine(t, "@INSTR GO?", "tag", 2, 1,
			v121TestElement(t, "tag", "@INSTR", "instr", 1, 7),
			v121TestElement(t, "content", "GO?", "GO?", 8, 11))
		v121TestRun(t, v121TestLesson(t, line), "current", [][3]int{{3, 8, 11}})
	})
}

func TestV121RowsAndExcludedContent(t *testing.T) {
	text := func(raw string, parent, depth int) model.Line {
		t.Helper()
		return v121TestText(t, raw, raw, parent, depth)
	}
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"rows are independent", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go", 3, 2), text("?", 3, 2), text("Why?", 3, 2), v121TestClose(t, 3, 2),
		}, [][3]int{{6, 1, 5}}},
		{"inline hint excluded", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go?", 3, 2), v121TestInline(t, "hint", "Why?", "Why?", 3, 2),
			text("Next?", 3, 2), v121TestClose(t, 3, 2),
		}, [][3]int{{4, 1, 4}, {6, 1, 6}}},
		{"block hint excluded", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go?", 3, 2), v121TestOpen(t, "hint", 3, 2),
			text("Why?", 5, 3), v121TestClose(t, 5, 3), text("Next?", 3, 2), v121TestClose(t, 3, 2),
		}, [][3]int{{4, 1, 4}, {8, 1, 6}}},
		{"question only in hint", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go.", 3, 2), v121TestOpen(t, "hint", 3, 2),
			text("Why?", 5, 3), v121TestClose(t, 5, 3), v121TestClose(t, 3, 2),
		}, nil},
		{"outside closed instr", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go.", 3, 2), v121TestClose(t, 3, 2), text("Why?", 2, 1),
		}, nil},
		{"literal editor content", []model.Line{
			v121TestOpen(t, "editor", 2, 1), text("@instr Why?", 3, 2), text("@instr {", 3, 2),
			text("Why?", 3, 2), v121TestClose(t, 3, 2), text("Read.", 2, 1),
		}, nil},
		{"blank between instructions", []model.Line{
			v121TestInline(t, "instr", "Go?", "Go?", 2, 1), v121TestLine(t, "", "blank", 2, 1),
			v121TestInline(t, "instr", "Why?", "Why?", 2, 1),
		}, [][3]int{{3, 8, 11}, {5, 8, 12}}},
		{"blank and punctuation between block questions", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go?", 3, 2), v121TestLine(t, " \t", "blank", 3, 2),
			text("???", 3, 2), text("Why?", 3, 2), v121TestClose(t, 3, 2),
		}, [][3]int{{4, 1, 4}, {7, 1, 5}}},
		{"consecutive blocks do not share letters", []model.Line{
			v121TestOpen(t, "instr", 2, 1), text("Go", 3, 2), v121TestClose(t, 3, 2),
			v121TestOpen(t, "instr", 2, 1), text("?", 6, 2), text("Read.", 6, 2), v121TestClose(t, 6, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v121TestRun(t, v121TestLesson(t, tc.body...), "current", tc.want) })
	}
	for _, name := range []string{"note", "question", "hint", "editor"} {
		t.Run("other inline "+name, func(t *testing.T) {
			v121TestRun(t, v121TestLesson(t, v121TestInline(t, name, "Why?", "Why?", 2, 1), text("Read.", 2, 1)), "current", nil)
		})
	}
	for _, name := range []string{"note", "hint", "text", "example"} {
		t.Run("other block "+name, func(t *testing.T) {
			v121TestRun(t, v121TestLesson(t, v121TestOpen(t, name, 2, 1), text("Why?", 3, 2), v121TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV121NestedInstructions(t *testing.T) {
	fragment := v121TestLine(t, "@fragment sample {", "block-start", 2, 1,
		v121TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v121TestElement(t, "identifier", "sample", "sample", 11, 17), v121TestElement(t, "block-open", "{", "{", 18, 19))
	include := v121TestLine(t, "@include sample", "tag", 2, 1,
		v121TestElement(t, "tag", "@include", "include", 1, 9), v121TestElement(t, "identifier", "sample", "sample", 10, 16))
	t.Run("inline in fragment", func(t *testing.T) {
		v121TestRun(t, v121TestLesson(t, fragment, v121TestInline(t, "instr", "Why?", "Why?", 3, 2),
			v121TestClose(t, 3, 2), include), "current", [][3]int{{4, 8, 12}})
	})
	t.Run("block in fragment", func(t *testing.T) {
		v121TestRun(t, v121TestLesson(t, fragment, v121TestOpen(t, "instr", 3, 2), v121TestText(t, "Why?", "Why?", 4, 3),
			v121TestClose(t, 4, 3), v121TestClose(t, 3, 2), include), "current", [][3]int{{5, 1, 5}})
	})
}

func TestV121ContentWithPlaceholders(t *testing.T) {
	// По п. 4.3.3 обычный пропуск входит в текст инструкции. Модель может
	// выделить его отдельно; пробелы между элементами сохраняются в Raw.
	gap := v121TestElement(t, "placeholder", "_____", "", 11, 16)
	gap.Value = nil
	line := v121TestLine(t, "@instr Go _____ now?", "tag", 2, 1,
		v121TestElement(t, "tag", "@instr", "instr", 1, 7),
		v121TestElement(t, "content", "Go", "Go", 8, 10), gap,
		v121TestElement(t, "content", "now?", "now?", 17, 21))
	v121TestRun(t, v121TestLesson(t, line), "current", [][3]int{{3, 8, 21}})
	gap.Start, gap.End = 8, 13
	line = v121TestLine(t, "@instr _____ Go?", "tag", 2, 1,
		v121TestElement(t, "tag", "@instr", "instr", 1, 7), gap,
		v121TestElement(t, "content", "Go?", "Go?", 14, 17))
	v121TestRun(t, v121TestLesson(t, line), "current", [][3]int{{3, 8, 17}})
	gap.Start, gap.End = 4, 9
	line = v121TestLine(t, "Go _____ now?", "content", 3, 2,
		v121TestElement(t, "content", "Go", "Go", 1, 3), gap,
		v121TestElement(t, "content", "now?", "now?", 10, 14))
	v121TestRun(t, v121TestLesson(t, v121TestOpen(t, "instr", 2, 1), line, v121TestClose(t, 3, 2)), "current", [][3]int{{4, 1, 14}})
}

func TestV121EndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("last line ending "+string(eol), func(t *testing.T) {
			result := v121TestLesson(t, v121TestInline(t, "instr", "Go?", "Go?", 2, 1))
			if eol == "\r\n" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			result.Lines[2].LineEnding = eol
			v121TestRun(t, result, "first", [][3]int{{3, 8, 11}})
			// Уже накопленные V121 сохраняются и не включаются в ожидание нового запуска.
			v121TestRun(t, result, "second", [][3]int{{3, 8, 11}})
		})
	}
	t.Run("empty model after questions", func(t *testing.T) {
		result := v121TestLesson(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		// Это отсутствие строк в модели при известной версии, а не корректный
		// пустой DSL-файл: CheckModel разрешает такой вход.
		v121TestRun(t, result, "empty", nil)
		v121TestRun(t, v121TestLesson(t, v121TestInline(t, "instr", "Read.", "Read.", 2, 1)), "no-question", nil)
	})
}

// v121TestLesson добавляет необходимый контекст; первая строка body имеет номер 3.
func v121TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v121TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v121TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v121TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v121TestLine(t, "# Lesson", "heading", 0, 0,
			v121TestElement(t, "heading-level", "#", "1", 1, 2), v121TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
}

// v121TestInline строит явно указанную однострочную форму, не классифицируя Raw.
func v121TestInline(t *testing.T, name, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v121TestLine(t, "@"+name+" "+raw, "tag", parent, depth,
		v121TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v121TestElement(t, "content", raw, value, len(name)+3, len(name)+3+utf8.RuneCountInString(raw)))
}

// v121TestText сохраняет независимо заданные исходный текст и семантическое значение.
func v121TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v121TestLine(t, raw, "content", parent, depth,
		v121TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v121TestOpen задаёт объявление блока без параметров с указанным родителем.
func v121TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v121TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v121TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v121TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v121TestClose задаёт закрытие указанного блока.
func v121TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v121TestLine(t, "}", "block-end", parent, depth, v121TestElement(t, "block-close", "}", "}", 1, 2))
}

// v121TestLine сохраняет заданную структуру и не выводит её из исходного текста.
func v121TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v121TestElement задаёт исходный диапазон с исключённой правой границей.
func v121TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v121TestRegistry использует исключительно полный штатный каталог.
func v121TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v121TestRun проверяет только V121 текущего запуска полного конвейера.
// want содержит явные тройки: строка, первая колонка, исключённая конечная колонка.
func v121TestRun(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v121TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v121TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v121TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V121" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V121 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != v121TestMessage {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V121 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v121TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Возвращаем в копии только производные флаги и ссылки, затем убираем
	// новые записи. Остальная модель обязана совпасть с исходным снимком.
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
	if v121TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v121TestSnapshot получает независимое представление модели для сравнения.
func v121TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

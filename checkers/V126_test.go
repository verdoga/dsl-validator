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

func TestV126Registration(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V126" {
			continue
		}
		count++
		if definition.Severity != "warning" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "В тифлокомментарии есть служебная фраза о его начале или конце." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V126 definitions = %d, want 1", count)
	}
}

func TestV126SingleLine(t *testing.T) {
	// Первые две строки таблицы сохраняют эталон и пример дословно.
	for _, tc := range []struct {
		name, raw string
		want      [][4]int
	}{
		{"reference", "@alt Семья сидит за столом.", nil},
		{"example", "@alt Начало тифлокомментария. Семья сидит за столом. Конец тифлокомментария.", [][4]int{{3, 6, 3, 29}, {3, 54, 3, 76}}},
		{"beginning", "@alt Начало тифлокомментария", [][4]int{{3, 6, 3, 29}}},
		{"ending", "@alt Конец тифлокомментария", [][4]int{{3, 6, 3, 28}}},
		{"repeated", "@alt Начало тифлокомментария. Начало тифлокомментария.", [][4]int{{3, 6, 3, 29}, {3, 31, 3, 54}}},
		{"reversed phrases", "@alt Конец тифлокомментария. Начало тифлокомментария", [][4]int{{3, 6, 3, 28}, {3, 30, 3, 53}}},
		{"uppercase", "@ALT НАЧАЛО ТИФЛОКОММЕНТАРИЯ", [][4]int{{3, 6, 3, 29}}},
		{"mixed case", "@alt нАчАлО ТиФлОкОмМеНтАрИя", [][4]int{{3, 6, 3, 29}}},
		{"spaces", "@alt Начало  тифлокомментария", [][4]int{{3, 6, 3, 30}}},
		{"tabs", "@alt Начало\t\tтифлокомментария", [][4]int{{3, 6, 3, 30}}},
		{"NBSP", "@alt Начало\u00a0\u00a0тифлокомментария", [][4]int{{3, 6, 3, 30}}},
		{"Unicode spaces", "@alt Начало\u2003\u202fтифлокомментария", [][4]int{{3, 6, 3, 30}}},
		{"quoted", "@alt «Начало тифлокомментария».", [][4]int{{3, 7, 3, 30}}},
		{"comma between words", "@alt Начало, тифлокомментария", nil},
		{"hyphen between words", "@alt Начало-тифлокомментария", nil},
		{"zero width space is not whitespace", "@alt Начало\u200bтифлокомментария", nil},
		{"no separator", "@alt Началотифлокомментария", nil},
		{"first word continued", "@alt предначало тифлокомментария", nil},
		{"last word continued", "@alt начало тифлокомментариями", nil},
		{"Latin prefix", "@alt Xначало тифлокомментария", nil},
		{"Latin suffix", "@alt конец тифлокомментарияX", nil},
		{"Unicode letter prefix", "@alt 界начало тифлокомментария", nil},
		{"digits are not letters", "@alt 1начало тифлокомментария2", [][4]int{{3, 7, 3, 30}}},
		{"underscores are not letters", "@alt _конец тифлокомментария_", [][4]int{{3, 7, 3, 29}}},
		{"other service phrase", "@alt Описание изображения.", nil},
		{"Unicode columns", "@alt 😀 Начало тифлокомментария.", [][4]int{{3, 8, 3, 31}}},
		{"leading and trailing spaces", "@alt   Начало тифлокомментария  ", [][4]int{{3, 8, 3, 31}}},
		{"HTML is not removed", "@alt Начало <em>тифлокомментария</em>", nil},
		{"entities are not decoded", "@alt Начало&nbsp;тифлокомментария", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Имя тега и content заданы явно: помощник не разбирает DSL.
			line := v126TestLine(t, tc.raw, "tag", 2, 1,
				v126TestElement(t, "tag", tc.raw[:4], "alt", 1, 5),
				v126TestElement(t, "content", tc.raw[5:], tc.raw[5:], 6, utf8.RuneCountInString(tc.raw)+1))
			result := v126TestDocument(t, line)
			if result.Lines[2].Raw != tc.raw {
				t.Fatal("source fragment changed")
			}
			v126TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV126BlockText(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		rows        []string
		want        [][4]int
	}{
		{"whole phrase", "", []string{"Начало тифлокомментария"}, [][4]int{{4, 1, 4, 24}}},
		{"across lines", "", []string{"Начало", "тифлокомментария"}, [][4]int{{4, 1, 5, 17}}},
		{"across blank", "", []string{"Начало", "", "тифлокомментария"}, [][4]int{{4, 1, 6, 17}}},
		{"across several blanks", "", []string{"Начало", " \t", "", "тифлокомментария"}, [][4]int{{4, 1, 7, 17}}},
		{"indentation", "", []string{"\t Начало  ", "  тифлокомментария  "}, [][4]int{{4, 3, 5, 19}}},
		{"word split", "", []string{"Начало тифло", "комментария"}, nil},
		{"first word split", "", []string{"На", "чало тифлокомментария"}, nil},
		{"punctuation retained", "", []string{"Начало,", "тифлокомментария"}, nil},
		{"nonempty text intervenes", "", []string{"Начало", "описания", "тифлокомментария"}, nil},
		{"title excluded", "Начало тифлокомментария", []string{"Семья сидит за столом."}, nil},
		{"body with title", "Семья", []string{"Конец тифлокомментария"}, [][4]int{{4, 1, 4, 23}}},
		{"first and last rows", "", []string{"Начало тифлокомментария", "Семья сидит за столом.", "Конец тифлокомментария"}, [][4]int{{4, 1, 4, 24}, {6, 1, 6, 23}}},
		{"two multiline phrases", "", []string{"Начало", "тифлокомментария. Конец", "тифлокомментария"}, [][4]int{{4, 1, 5, 17}, {5, 19, 6, 17}}},
		{"empty block", "", nil, nil},
		{"blank block", "", []string{"", " \t"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v126TestOpen(t, "alt", tc.title, 2, 1)}
			for _, raw := range tc.rows {
				if strings.Trim(raw, " \t") == "" {
					body = append(body, v126TestLine(t, raw, "blank", 3, 2))
				} else {
					body = append(body, v126TestText(t, raw, raw, 3, 2))
				}
			}
			body = append(body, v126TestClose(t, 3, 2))
			v126TestRun(t, v126TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV126Containers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       []model.Line
		errorLines []int
		want       [][4]int
	}{
		{"single-line nested construction is a barrier", []model.Line{
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "Начало", "Начало", 3, 2),
			v126TestTag(t, "note", "Пояснение.", 3, 2), v126TestText(t, "тифлокомментария", "тифлокомментария", 3, 2), v126TestClose(t, 3, 2),
		}, []int{5}, nil},
		{"nested content excluded", []model.Line{
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "Семья.", "Семья.", 3, 2),
			v126TestTag(t, "note", "Начало тифлокомментария", 3, 2), v126TestClose(t, 3, 2),
		}, []int{5}, nil},
		{"block nested construction is a barrier", []model.Line{
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "Начало", "Начало", 3, 2),
			v126TestOpen(t, "note", "", 3, 2), v126TestText(t, "Конец тифлокомментария", "Конец тифлокомментария", 5, 3), v126TestClose(t, 5, 3),
			v126TestText(t, "тифлокомментария", "тифлокомментария", 3, 2), v126TestClose(t, 3, 2),
		}, []int{5}, nil},
		{"nested alt checked once", []model.Line{
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "Начало", "Начало", 3, 2),
			v126TestTag(t, "alt", "Конец тифлокомментария", 3, 2), v126TestText(t, "тифлокомментария", "тифлокомментария", 3, 2), v126TestClose(t, 3, 2),
		}, []int{5}, [][4]int{{5, 6, 5, 28}}},
		{"nested block alt and own text after it", []model.Line{
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "Семья.", "Семья.", 3, 2),
			v126TestOpen(t, "alt", "", 3, 2), v126TestText(t, "Начало", "Начало", 5, 3),
			v126TestText(t, "тифлокомментария", "тифлокомментария", 5, 3), v126TestClose(t, 5, 3),
			v126TestText(t, "Конец тифлокомментария", "Конец тифлокомментария", 3, 2), v126TestClose(t, 3, 2),
		}, []int{5}, [][4]int{{6, 1, 7, 17}, {9, 1, 9, 23}}},
		{"adjacent alt blocks", []model.Line{
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "Начало", "Начало", 3, 2), v126TestClose(t, 3, 2),
			v126TestOpen(t, "alt", "", 2, 1), v126TestText(t, "тифлокомментария", "тифлокомментария", 6, 2), v126TestClose(t, 6, 2),
		}, nil, nil},
		{"outside content excluded", []model.Line{
			v126TestTag(t, "alt", "Начало", 2, 1), v126TestText(t, "тифлокомментария", "тифлокомментария", 2, 1),
			v126TestText(t, "Конец тифлокомментария", "Конец тифлокомментария", 2, 1),
		}, nil, nil},
		{"literal pseudo alt", []model.Line{
			v126TestOpen(t, "editor", "", 2, 1),
			v126TestText(t, "@alt Начало тифлокомментария", "@alt Начало тифлокомментария", 3, 2), v126TestClose(t, 3, 2),
		}, nil, nil},
		{"escaped pseudo alt", []model.Line{
			v126TestText(t, `\@alt Начало тифлокомментария`, "@alt Начало тифлокомментария", 2, 1),
		}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v126TestDocument(t, tc.body...)
			for _, number := range tc.errorLines {
				v126TestError(t, result, number, "Вложенный тег запрещён в alt.")
			}
			v126TestRun(t, result, "current", tc.want)
		})
	}
	// fragment имеет обязательный ID; глубина здесь задаётся явно.
	fragment := v126TestLine(t, "@fragment f {", "block-start", 2, 1,
		v126TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v126TestElement(t, "identifier", "f", "f", 11, 12), v126TestElement(t, "block-open", "{", "{", 13, 14))
	v126TestRun(t, v126TestDocument(t, fragment, v126TestTag(t, "alt", "Начало тифлокомментария", 3, 2),
		v126TestClose(t, 3, 2)), "fragment", [][4]int{{4, 6, 4, 29}})
	v126TestRun(t, v126TestDocument(t, fragment, v126TestOpen(t, "alt", "", 3, 2),
		v126TestText(t, "Конец", "Конец", 4, 3), v126TestText(t, "тифлокомментария", "тифлокомментария", 4, 3),
		v126TestClose(t, 4, 3), v126TestClose(t, 3, 2)), "deep", [][4]int{{5, 1, 6, 17}})
}

func TestV126RawAndContentElements(t *testing.T) {
	// Экранирование снимается парсером в Value. V126 не выполняет этот шаг снова.
	line := v126TestLine(t, `@alt \{Начало тифлокомментария\}`, "tag", 2, 1,
		v126TestElement(t, "tag", "@alt", "alt", 1, 5),
		v126TestElement(t, "content", `\{Начало тифлокомментария\}`, "{Начало тифлокомментария}", 6, 33))
	v126TestRun(t, v126TestDocument(t, line), "raw", [][4]int{{3, 8, 3, 31}})
	// content вокруг плейсхолдера не склеивается; подчёркивания остаются границей.
	row := v126TestLine(t, "Начало _____ тифлокомментария", "content", 3, 2,
		v126TestElement(t, "content", "Начало ", "Начало ", 1, 8),
		v126TestElement(t, "placeholder", "_____", "_____", 8, 13),
		v126TestElement(t, "content", " тифлокомментария", " тифлокомментария", 13, 30))
	v126TestRun(t, v126TestDocument(t, v126TestOpen(t, "alt", "", 2, 1), row, v126TestClose(t, 3, 2)), "placeholder", nil)
	// Несколько content сохраняют исходные промежутки и не получают новых пробелов.
	for _, tc := range []struct {
		name string
		row  model.Line
		want [][4]int
	}{
		{"whitespace gap", v126TestLine(t, "Начало  тифлокомментария", "content", 3, 2,
			v126TestElement(t, "content", "Начало", "Начало", 1, 7),
			v126TestElement(t, "content", "тифлокомментария", "тифлокомментария", 9, 25)), [][4]int{{4, 1, 4, 25}}},
		{"punctuation gap", v126TestLine(t, "Начало, тифлокомментария", "content", 3, 2,
			v126TestElement(t, "content", "Начало", "Начало", 1, 7),
			v126TestElement(t, "content", "тифлокомментария", "тифлокомментария", 9, 25)), nil},
		{"adjacent parts do not insert a space", v126TestLine(t, "Началотифлокомментария", "content", 3, 2,
			v126TestElement(t, "content", "Начало", "Начало", 1, 7),
			v126TestElement(t, "content", "тифлокомментария", "тифлокомментария", 7, 23)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v126TestRun(t, v126TestDocument(t, v126TestOpen(t, "alt", "", 2, 1), tc.row, v126TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV126EndingsHistoryAndVersions(t *testing.T) {
	for _, ending := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(ending, func(t *testing.T) {
			result := v126TestDocument(t, v126TestOpen(t, "alt", "", 2, 1),
				v126TestText(t, "Начало", "Начало", 3, 2), v126TestText(t, "тифлокомментария", "тифлокомментария", 3, 2), v126TestClose(t, 3, 2))
			example := v126TestDocument(t, v126TestTag(t, "alt", "Начало тифлокомментария. Семья сидит за столом. Конец тифлокомментария.", 2, 1))
			for _, document := range []*model.Result{result, example} {
				for index := range document.Lines {
					switch ending {
					case "LF":
						document.Lines[index].LineEnding = "\n"
					case "CRLF":
						document.Lines[index].LineEnding = "\r\n"
					case "mixed":
						if index%2 == 0 {
							document.Lines[index].LineEnding = "\r\n"
						}
					}
				}
			}
			v126TestRun(t, result, "first", [][4]int{{4, 1, 5, 17}})
			v126TestRun(t, result, "second", [][4]int{{4, 1, 5, 17}})
			v126TestRun(t, example, "first", [][4]int{{3, 6, 3, 29}, {3, 54, 3, 76}})
			v126TestRun(t, example, "second", [][4]int{{3, 6, 3, 29}, {3, 54, 3, 76}})
			v126TestRun(t, v126TestDocument(t, v126TestTag(t, "alt", "Семья сидит за столом.", 2, 1)), "separate", nil)
		})
	}
	t.Run("unclosed block retains own content", func(t *testing.T) {
		result := v126TestDocument(t, v126TestOpen(t, "alt", "", 2, 1),
			v126TestText(t, "Начало", "Начало", 3, 2), v126TestText(t, "тифлокомментария", "тифлокомментария", 3, 2))
		v126TestError(t, result, 3, "Блок не закрыт до конца файла.")
		v126TestRun(t, result, "current", [][4]int{{4, 1, 5, 17}})
	})
	v126TestRun(t, v126TestDocument(t), "no-alt", nil)
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run("empty "+version, func(t *testing.T) {
			count := 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if version == "1.2" {
				v126TestRun(t, result, "empty", nil)
				return
			}
			registry, err := diagnostics.NewRegistry(RegistrationCheckers())
			if err != nil {
				t.Fatal(err)
			}
			before := v126TestSnapshot(t, result)
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil || v126TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v126TestDocument добавляет только обязательные версию и заголовок по DSL 1.30.
// Родители и глубины всех строк задаются тестами явно с учётом этих двух строк.
func v126TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v126TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v126TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v126TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v126TestLine(t, "# Lesson", "heading", 0, 0, v126TestElement(t, "heading-level", "#", "1", 1, 2), v126TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
	}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}}, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

// v126TestLine собирает строку без распознавания DSL и восстановления родителей.
func v126TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v126TestElement сохраняет явно заданные исходное и семантическое значения.
func v126TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v126TestTag создаёт однострочный тег с содержимым, не заголовком.
func v126TestTag(t *testing.T, tag, content string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag + " " + content
	return v126TestLine(t, raw, "tag", parent, depth, v126TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v126TestElement(t, "content", content, content, len(tag)+3, utf8.RuneCountInString(raw)+1))
}

// v126TestOpen создаёт блок с необязательным TITLE.
func v126TestOpen(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag + " "
	elements := []model.Element{v126TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2)}
	if title != "" {
		elements = append(elements, v126TestElement(t, "title", title, title, len(tag)+3, len(tag)+3+utf8.RuneCountInString(title)))
		raw += title + " "
	}
	column := utf8.RuneCountInString(raw) + 1
	return v126TestLine(t, raw+"{", "block-start", parent, depth, append(elements, v126TestElement(t, "block-open", "{", "{", column, column+1))...)
}

// v126TestClose создаёт закрывающую строку с явно указанным родителем.
func v126TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v126TestLine(t, "}", "block-end", parent, depth, v126TestElement(t, "block-close", "}", "}", 1, 2))
}

// v126TestText сохраняет Raw и Value без взаимного преобразования.
func v126TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v126TestLine(t, raw, "content", parent, depth, v126TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v126TestError добавляет исходную ошибку парсера для явно ошибочного контекста.
func v126TestError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "p" + strconv.Itoa(len(result.Diagnostics)+1), Source: "parser",
		DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: utf8.RuneCountInString(line.Raw) + 1}}})
}

// v126TestRun проверяет только новые V126, включая отсутствие связанных мест и
// конкретизации. Ожидания — буквальные диапазоны с исключённой правой границей.
func v126TestRun(t *testing.T, result *model.Result, source string, want [][4]int) {
	t.Helper()
	before := v126TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v126TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics[len(original.Diagnostics):] {
		if diagnostic.DiagnosticCode == "V126" {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V126 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, position := range want {
		diagnostic := got[index]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[2], Column: position[3]}}
		if diagnostic.Location == nil || *diagnostic.Location != location || len(diagnostic.RelatedLocations) != 0 || diagnostic.RelatedLocations == nil ||
			diagnostic.Message != "В тифлокомментарии есть служебная фраза о его начале или конце." || diagnostic.SeverityLevel != "warning" ||
			diagnostic.DiagnosticScope != "block" || diagnostic.Fatal || diagnostic.Source != source || diagnostic.ID == "" {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V126 metadata", index, diagnostic, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	// Возвращаем производные поля только в копии: исходник и история неизменны.
	var restored model.Result
	if err := json.Unmarshal([]byte(v126TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source shape changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for index := range restored.Lines {
		if len(restored.Lines[index].Elements) != len(original.Lines[index].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[index].HasErrors = original.Lines[index].HasErrors
		for element := range restored.Lines[index].Elements {
			restored.Lines[index].Elements[element].ErrorIDs = original.Lines[index].Elements[element].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v126TestSnapshot(t, &restored) != before {
		t.Fatal("source or history changed")
	}
}

// v126TestSnapshot сохраняет полную модель для сравнения исходника и истории.
func v126TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

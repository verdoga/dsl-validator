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

// v127TestExpected задаёт основной и связанные исходные диапазоны.
// main содержит строку и колонку начала и конца, related — строку и две колонки.
// Все правые границы исключены; ожидания не вычисляются по исходному тексту.
type v127TestExpected struct {
	main    [4]int
	related [][3]int
}

// v127TestMessage задан независимо от данных проверяемой диагностики.
const v127TestMessage = "В тифлокомментарии есть буквы вне русского алфавита.\nконкретизация: Редактору необходимо проверить и проанализировать, не является ли комментарий подсказкой или готовым ответом к упражнению. Обычное описание допустимо; видимые надписи сохраняются на языке источника внутри русского описания. Не удаляйте учебно значимые признаки изображения только потому, что они нужны для ответа"

func TestV127Registration(t *testing.T) {
	registry := v127TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V127" {
			continue
		}
		count++
		if definition.Message != "В тифлокомментарии есть буквы вне русского алфавита." ||
			definition.Severity != "recommendation" || definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V127 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V127 definitions = %d, want 1", count)
	}
	// Соседние версии отвергаются штатным каталогом поддержки; не расширяем его.
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("ForVersion(%q) = %+v, %v; want rejection", version, definitions, err)
		}
	}
}

func TestV127ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, content, fragment string
		want                    []v127TestExpected
	}{
		{"reference", "На табличке изображена стрелка вправо.", "@alt На табличке изображена стрелка вправо.", nil},
		{"example", "На двери видна надпись «Open».", "@alt На двери видна надпись «Open».",
			[]v127TestExpected{{[4]int{3, 1, 3, 36}, [][3]int{{3, 30, 34}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v127TestLesson(t, v127TestInline(t, "alt", tc.content, tc.content, 2, 1))
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// Добавлены только обязательные версия и заголовок. По п. 3.10.7
			// alt не требует технической привязки к media image.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v127TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV127LettersWordsAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		end              int
		related          [][2]int
	}{
		{"one Latin letter", "A", "A", 7, [][2]int{{6, 7}}},
		{"lower case letter", "a", "a", 7, [][2]int{{6, 7}}},
		{"Russian uppercase", "АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ", "АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ", 0, nil},
		{"Russian lowercase", "абвгдежзийклмнопрстуфхцчшщъыьэюя", "абвгдежзийклмнопрстуфхцчшщъыьэюя", 0, nil},
		{"both yo letters", "Ёё", "Ёё", 0, nil},
		{"no letters", "123 — !", "123 — !", 0, nil},
		{"emoji is not a letter", "😀", "😀", 0, nil},
		{"letter number is not Letter", "Ⅳ", "Ⅳ", 0, nil},
		{"Cyrillic outside Russian", "і", "і", 7, [][2]int{{6, 7}}},
		{"uncased letters", "漢字", "漢字", 8, [][2]int{{6, 8}}},
		{"titlecase letter", "ǅ", "ǅ", 7, [][2]int{{6, 7}}},
		{"modifier letter stays in word", "аʼ", "аʼ", 8, [][2]int{{6, 8}}},
		{"supplementary letter is one column", "𝐀", "𝐀", 7, [][2]int{{6, 7}}},
		{"mixed alphabet word", "Сat", "Сat", 9, [][2]int{{6, 9}}},
		{"repeated word", "Open Open", "Open Open", 15, [][2]int{{6, 10}, {11, 15}}},
		{"several foreign letters one word", "Open", "Open", 10, [][2]int{{6, 10}}},
		{"hyphen splits words", "Open-air", "Open-air", 14, [][2]int{{6, 10}, {11, 14}}},
		{"apostrophe splits words", "don't", "don't", 11, [][2]int{{6, 9}, {10, 11}}},
		{"curly apostrophe splits words", "don’t", "don’t", 11, [][2]int{{6, 9}, {10, 11}}},
		{"label with digit", "A1", "A1", 8, [][2]int{{6, 7}}},
		{"digit between letters", "A1B", "A1B", 9, [][2]int{{6, 7}, {8, 9}}},
		{"digit before letter", "1A", "1A", 8, [][2]int{{7, 8}}},
		{"punctuation splits repeated words", "Open,Open.", "Open,Open.", 16, [][2]int{{6, 10}, {11, 15}}},
		{"NBSP splits words", "A\u00a0B", "A\u00a0B", 9, [][2]int{{6, 7}, {8, 9}}},
		{"combining Russian accent", "а\u0301", "а\u0301", 0, nil},
		{"combining mark alone", "\u0301", "\u0301", 0, nil},
		{"mark within word", "A\u0301B", "A\u0301B", 9, [][2]int{{6, 9}}},
		{"mark after word", "A\u0301", "A\u0301", 8, [][2]int{{6, 8}}},
		{"enclosing mark after letter", "A\u20dd", "A\u20dd", 8, [][2]int{{6, 8}}},
		{"leading mark excluded", "\u0301A", "\u0301A", 8, [][2]int{{7, 8}}},
		{"precomposed Cyrillic is outside set", "ӑ", "ӑ", 7, [][2]int{{6, 7}}},
		{"decomposed Russian letter is allowed", "а\u0306", "а\u0306", 0, nil},
		{"escaped braces", `\{Open\}`, `{Open}`, 14, [][2]int{{8, 12}}},
		{"paired slashes", `\\Open`, `\Open`, 12, [][2]int{{8, 12}}},
		{"unknown escape preserved", `\Open`, `\Open`, 11, [][2]int{{7, 11}}},
		{"three slashes processed left to right", `\\\Open`, `\\Open`, 13, [][2]int{{9, 13}}},
		{"escaped underscores", `\_____Open`, `_____Open`, 16, [][2]int{{12, 16}}},
		{"single underscore escape preserved", `\_Open`, `\_Open`, 12, [][2]int{{8, 12}}},
		{"at sign is not physical line start", `\@Open`, `\@Open`, 12, [][2]int{{8, 12}}},
		{"trailing slash preserved", `Open\`, `Open\`, 11, [][2]int{{6, 10}}},
		{"no Unicode escape decoding", `\u0041`, `\u0041`, 12, [][2]int{{7, 8}}},
		{"no numeric entity decoding", "&#65;", "&#65;", 0, nil},
		{"angle brackets are literal in alt", "<b>Open</b>", "<b>Open</b>", 17, [][2]int{{7, 8}, {9, 13}, {15, 16}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Вся таблица относится к строке 3, начинающейся в колонке 1.
			// Конец конструкции и колонки слов заданы числами, не подсчитаны по raw.
			var want []v127TestExpected
			if tc.end != 0 {
				expected := v127TestExpected{main: [4]int{3, 1, 3, tc.end}}
				for _, span := range tc.related {
					expected.related = append(expected.related, [3]int{3, span[0], span[1]})
				}
				want = append(want, expected)
			}
			v127TestRun(t, v127TestLesson(t, v127TestInline(t, "alt", tc.raw, tc.value, 2, 1)), "current", want)
		})
	}
}

func TestV127OriginalPositions(t *testing.T) {
	t.Run("inline margins and tab", func(t *testing.T) {
		line := v127TestLine(t, "  @alt \tOpen  ", "tag", 2, 1,
			v127TestElement(t, "tag", "@alt", "alt", 3, 7),
			v127TestElement(t, "content", "Open", "Open", 9, 13))
		v127TestRun(t, v127TestLesson(t, line), "current", []v127TestExpected{{[4]int{3, 1, 3, 15}, [][3]int{{3, 9, 13}}}})
	})
	t.Run("canonical tag value", func(t *testing.T) {
		line := v127TestLine(t, "@ALT Open", "tag", 2, 1,
			v127TestElement(t, "tag", "@ALT", "alt", 1, 5), v127TestElement(t, "content", "Open", "Open", 6, 10))
		v127TestRun(t, v127TestLesson(t, line), "current", []v127TestExpected{{[4]int{3, 1, 3, 10}, [][3]int{{3, 6, 10}}}})
	})
	for _, tc := range []struct {
		name, raw, value string
		start, end       int
	}{
		{"leading escaped at sign", `\@Open`, `@Open`, 3, 7},
		{"leading escaped hash", `\#Open`, `#Open`, 3, 7},
		{"indented escaped at sign", " \t\\@Open  ", "@Open", 5, 9},
		{"two escaped underscores", `\__Open`, `__Open`, 4, 8},
		{"six escaped underscores", `\______Open`, `______Open`, 8, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v127TestRun(t, v127TestLesson(t, v127TestOpen(t, "alt", 2, 1),
				v127TestText(t, tc.raw, tc.value, 3, 2), v127TestClose(t, 3, 2)), "current",
				[]v127TestExpected{{[4]int{3, 1, 5, 2}, [][3]int{{4, tc.start, tc.end}}}})
		})
	}
}

func TestV127BlocksTitlesAndExclusions(t *testing.T) {
	text := func(raw string, parent, depth int) model.Line {
		t.Helper()
		return v127TestText(t, raw, raw, parent, depth)
	}
	titled := func(title string) model.Line {
		t.Helper()
		return v127TestLine(t, "@alt "+title+" {", "block-start", 2, 1,
			v127TestElement(t, "tag", "@alt", "alt", 1, 5),
			v127TestElement(t, "title", title, title, 6, 6+len(title)),
			v127TestElement(t, "block-open", "{", "{", 7+len(title), 8+len(title)))
	}
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v127TestExpected
	}{
		{"foreign title only", []model.Line{titled("Open"), text("Стрелка вправо.", 3, 2), v127TestClose(t, 3, 2)}, nil},
		{"title not related", []model.Line{titled("Title"), text("Open", 3, 2), v127TestClose(t, 3, 2)},
			[]v127TestExpected{{[4]int{3, 1, 5, 2}, [][3]int{{4, 1, 5}}}}},
		{"several content rows", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Open", 3, 2), text("Closed", 3, 2), v127TestClose(t, 3, 2)},
			[]v127TestExpected{{[4]int{3, 1, 6, 2}, [][3]int{{4, 1, 5}, {5, 1, 7}}}}},
		{"blank between repeated words", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Open", 3, 2),
			v127TestLine(t, "", "blank", 3, 2), text("Open", 3, 2), v127TestClose(t, 3, 2)},
			[]v127TestExpected{{[4]int{3, 1, 7, 2}, [][3]int{{4, 1, 5}, {6, 1, 5}}}}},
		{"adjacent rows stay separate", []model.Line{v127TestOpen(t, "alt", 2, 1), text("O", 3, 2), text("pen", 3, 2), v127TestClose(t, 3, 2)},
			[]v127TestExpected{{[4]int{3, 1, 6, 2}, [][3]int{{4, 1, 2}, {5, 1, 4}}}}},
		{"foreign text after block excluded", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Стрелка.", 3, 2), v127TestClose(t, 3, 2), text("Open", 2, 1)}, nil},
		{"inline nested hint excluded", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Стрелка.", 3, 2),
			v127TestInline(t, "hint", "Open", "Open", 3, 2), v127TestClose(t, 3, 2)}, nil},
		{"block nested hint excluded", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Стрелка.", 3, 2), v127TestOpen(t, "hint", 3, 2),
			text("Open", 5, 3), v127TestClose(t, 5, 3), v127TestClose(t, 3, 2)}, nil},
		{"own content before nested hint", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Open", 3, 2), v127TestOpen(t, "hint", 3, 2),
			text("Open", 5, 3), v127TestClose(t, 5, 3), v127TestClose(t, 3, 2)},
			[]v127TestExpected{{[4]int{3, 1, 8, 2}, [][3]int{{4, 1, 5}}}}},
		{"own content after nested hint", []model.Line{v127TestOpen(t, "alt", 2, 1), v127TestOpen(t, "hint", 3, 2),
			text("Open", 4, 3), v127TestClose(t, 4, 3), text("Closed", 3, 2), v127TestClose(t, 3, 2)},
			[]v127TestExpected{{[4]int{3, 1, 8, 2}, [][3]int{{7, 1, 7}}}}},
		{"other nested construction excluded", []model.Line{v127TestOpen(t, "alt", 2, 1), text("Стрелка.", 3, 2), v127TestOpen(t, "note", 3, 2),
			text("Open", 5, 3), v127TestClose(t, 5, 3), v127TestClose(t, 3, 2)}, nil},
		{"two independent alts", []model.Line{v127TestInline(t, "alt", "Open", "Open", 2, 1), v127TestInline(t, "alt", "Open", "Open", 2, 1)},
			[]v127TestExpected{{[4]int{3, 1, 3, 10}, [][3]int{{3, 6, 10}}}, {[4]int{4, 1, 4, 10}, [][3]int{{4, 6, 10}}}}},
		{"neighboring Russian alt unaffected", []model.Line{v127TestInline(t, "alt", "Open", "Open", 2, 1), v127TestInline(t, "alt", "Стрелка.", "Стрелка.", 2, 1)},
			[]v127TestExpected{{[4]int{3, 1, 3, 10}, [][3]int{{3, 6, 10}}}}},
		{"literal editor content and other areas", []model.Line{v127TestInline(t, "note", "Open", "Open", 2, 1), text("Open", 2, 1),
			v127TestOpen(t, "editor", 2, 1), text("@alt Open", 5, 2), v127TestClose(t, 5, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Вложения hint/note в alt запрещены п. 1.24 DSL, но исключение
			// их содержимого прямо требуется V127. Модель сохраняет реальные
			// распознанные конструкции и родителей, а не выдаёт их за обычный текст.
			v127TestRun(t, v127TestLesson(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV127NestedAltAndPlaceholders(t *testing.T) {
	fragment := v127TestLine(t, "@fragment sample {", "block-start", 2, 1,
		v127TestElement(t, "tag", "@fragment", "fragment", 1, 10), v127TestElement(t, "identifier", "sample", "sample", 11, 17),
		v127TestElement(t, "block-open", "{", "{", 18, 19))
	include := v127TestLine(t, "@include sample", "tag", 2, 1,
		v127TestElement(t, "tag", "@include", "include", 1, 9), v127TestElement(t, "identifier", "sample", "sample", 10, 16))
	t.Run("inline alt in fragment", func(t *testing.T) {
		v127TestRun(t, v127TestLesson(t, fragment, v127TestInline(t, "alt", "Open", "Open", 3, 2), v127TestClose(t, 3, 2), include),
			"current", []v127TestExpected{{[4]int{4, 1, 4, 10}, [][3]int{{4, 6, 10}}}})
	})
	t.Run("block alt in fragment", func(t *testing.T) {
		v127TestRun(t, v127TestLesson(t, fragment, v127TestOpen(t, "alt", 3, 2), v127TestText(t, "Open", "Open", 4, 3),
			v127TestClose(t, 4, 3), v127TestClose(t, 3, 2), include), "current",
			[]v127TestExpected{{[4]int{4, 1, 6, 2}, [][3]int{{5, 1, 5}}}})
	})
	t.Run("placeholder separates content elements", func(t *testing.T) {
		gap := v127TestElement(t, "placeholder", "_____", "", 6, 11)
		gap.Value = nil
		line := v127TestLine(t, "Open _____ Air", "content", 3, 2,
			v127TestElement(t, "content", "Open", "Open", 1, 5), gap, v127TestElement(t, "content", "Air", "Air", 12, 15))
		v127TestRun(t, v127TestLesson(t, v127TestOpen(t, "alt", 2, 1), line, v127TestClose(t, 3, 2)), "current",
			[]v127TestExpected{{[4]int{3, 1, 5, 2}, [][3]int{{4, 1, 5}, {4, 12, 15}}}})
	})
}

func TestV127EndingsHistoryAndEmptyInput(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("last line ending "+string(eol), func(t *testing.T) {
			result := v127TestLesson(t, v127TestInline(t, "alt", "Open", "Open", 2, 1))
			if eol == "\r\n" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			result.Lines[2].LineEnding = eol
			want := []v127TestExpected{{[4]int{3, 1, 3, 10}, [][3]int{{3, 6, 10}}}}
			v127TestRun(t, result, "first", want)
			v127TestRun(t, result, "second", want)
			v127TestRun(t, v127TestLesson(t, v127TestInline(t, "alt", "Стрелка.", "Стрелка.", 2, 1)), "Russian", nil)
		})
	}
	t.Run("empty model", func(t *testing.T) {
		result := v127TestLesson(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		// CheckModel допускает отсутствие строк при известной версии.
		// Это ситуация входной модели, а не корректный пустой DSL-файл.
		v127TestRun(t, result, "empty", nil)
	})
}

// v127TestLesson добавляет обязательный контекст; первая строка body имеет номер 3.
func v127TestLesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v127TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v127TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13), v127TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v127TestLine(t, "# Lesson", "heading", 0, 0,
			v127TestElement(t, "heading-level", "#", "1", 1, 2), v127TestElement(t, "title", "Lesson", "Lesson", 3, 9)),
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

// v127TestInline задаёт однострочную форму с явными исходным и текстовым значениями.
func v127TestInline(t *testing.T, name, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v127TestLine(t, "@"+name+" "+raw, "tag", parent, depth,
		v127TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v127TestElement(t, "content", raw, value, len(name)+3, len(name)+3+utf8.RuneCountInString(raw)))
}

// v127TestText сохраняет независимо заданные исходный текст и семантическое значение.
func v127TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v127TestLine(t, raw, "content", parent, depth,
		v127TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v127TestOpen задаёт объявление блока без параметров с указанным родителем.
func v127TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v127TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v127TestElement(t, "tag", "@"+name, name, 1, len(name)+2),
		v127TestElement(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v127TestClose задаёт закрытие указанного блока.
func v127TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v127TestLine(t, "}", "block-end", parent, depth, v127TestElement(t, "block-close", "}", "}", 1, 2))
}

// v127TestLine сохраняет явно заданную структуру и не классифицирует Raw.
func v127TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v127TestElement задаёт исходный Unicode-диапазон с исключённой правой границей.
func v127TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v127TestRegistry использует исключительно полный штатный каталог регистраций.
func v127TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v127TestRun проверяет V127 текущего запуска полного конвейера и сохранность входа.
func v127TestRun(t *testing.T, result *model.Result, source string, want []v127TestExpected) {
	t.Helper()
	before := v127TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v127TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v127TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V127" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V127 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.main[0], Column: expected.main[1]},
			End: model.Position{Line: expected.main[2], Column: expected.main[3]}}
		if entry.Location == nil || *entry.Location != location {
			t.Fatalf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.ID == "" || entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "block" || entry.Fatal || entry.Message != v127TestMessage {
			t.Fatalf("occurrence %d metadata = %+v", i, entry)
		}
		if entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("occurrence %d related = %+v, want %+v", i, entry.RelatedLocations, expected.related)
		}
		for j, span := range expected.related {
			related := model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[0], Column: span[2]}}
			if entry.RelatedLocations[j] != related {
				t.Fatalf("occurrence %d related %d = %+v, want %+v", i, j, entry.RelatedLocations[j], related)
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v127TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// В копии возвращаем только производные флаги и ссылки, удаляем новые
	// записи запуска. Остальные данные должны совпасть с исходным снимком.
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
	if v127TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v127TestSnapshot получает независимое представление модели для сравнения.
func v127TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

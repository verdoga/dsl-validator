package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v046Expected задаёт независимые ожидания: строку и Unicode-колонки [start,end).
type v046Expected struct {
	line, start, end int
	detail           string
}

const (
	v046Missing = "Перед скобкой нет разделителя."
	v046Wrong   = "Разделитель содержит символы, отличные от U+0020."
)

func TestV046Registration(t *testing.T) {
	definitions, err := v046Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V046" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Перед открывающей фигурной скобкой отсутствует разделитель из обычных пробелов." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V046 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V046 definitions = %d, want 1", matches)
	}
}

func TestV046ReferenceAndExample(t *testing.T) {
	// К неизменённым фрагментам таблицы добавлены версия и заголовок (строки 1–2).
	t.Run("reference", func(t *testing.T) {
		result := v046Document(t, v046Open(t, "note", 2, 1),
			v046Text(t, "Read carefully.", 3, 2), v046Close(t, 3, 2))
		v046Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v046Document(t,
			v046Line(t, "@note{", "block-start", 2, 1, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 6, 7)),
			v046Text(t, "Read carefully.", 3, 2), v046Close(t, 3, 2),
			v046Line(t, "@note\t{", "block-start", 2, 1, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 7, 8)),
			v046Text(t, "Write carefully.", 6, 2), v046Close(t, 6, 2))
		v046Run(t, result, "current", []v046Expected{{3, 6, 7, v046Missing}, {6, 6, 7, v046Wrong}})
	})
}

func TestV046Separators(t *testing.T) {
	for _, tc := range []struct {
		name, gap string
		want      []v046Expected
	}{
		{"none", "", []v046Expected{{3, 6, 7, v046Missing}}},
		{"one space", " ", nil},
		{"three spaces", "   ", nil},
		{"tab", "\t", []v046Expected{{3, 6, 7, v046Wrong}}},
		{"two tabs", "\t\t", []v046Expected{{3, 6, 8, v046Wrong}}},
		{"mixed", " \t ", []v046Expected{{3, 6, 9, v046Wrong}}},
		{"mixed Unicode", "\u00a0 \t\u2003", []v046Expected{{3, 6, 10, v046Wrong}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			open := 6 + utf8.RuneCountInString(tc.gap)
			result := v046Document(t, v046Line(t, "@note"+tc.gap+"{", "block-start", 2, 1,
				v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", open, open+1)),
				v046Text(t, "Read.", 3, 2), v046Close(t, 3, 2))
			v046Run(t, result, "current", tc.want)
		})
	}
	// Все White_Space кроме разрешённого U+0020 и CR/LF, запрещённых в Raw.
	// Множество задано явно, независимо от unicode.White_Space реализации.
	for _, gap := range []string{"\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007",
		"\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run(fmt.Sprintf("U+%04X", []rune(gap)[0]), func(t *testing.T) {
			result := v046Document(t, v046Line(t, "@note"+gap+"{", "block-start", 2, 1,
				v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 7, 8)),
				v046Text(t, "Read.", 3, 2), v046Close(t, 3, 2))
			v046Run(t, result, "current", []v046Expected{{3, 6, 7, v046Wrong}})
		})
	}
}

func TestV046RawRecognition(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v046Expected
	}{
		{"indent and case", " \t@NoTe{", []v046Expected{{3, 8, 9, v046Missing}}},
		{"Unicode title", "@text Я😀{", []v046Expected{{3, 9, 10, v046Missing}}},
		{"title final mixed gap", "@text A\tB \t {", []v046Expected{{3, 10, 13, v046Wrong}}},
		{"name continuation", "@noteText{", nil},
		{"unknown name", "@unknown{", nil},
		{"inline mention", "text @note{", nil},
		{"escaped tag", `\@note{`, nil},
		{"punctuation boundary", "@note!{", nil},
		{"digit boundary", "@note1{", nil},
		{"Unicode letter boundary", "@noteЯ{", nil},
		{"NBSP indentation", "\u00a0@note{", nil},
		{"name at end", "@note", nil},
		{"bare at", "@", nil},
		{"first opening only", "@note{ {", []v046Expected{{3, 6, 7, v046Missing}}},
		{"trailing text", "@note{ Read.", []v046Expected{{3, 6, 7, v046Missing}}},
		{"zero width in title", "@text A\u200b{", []v046Expected{{3, 9, 10, v046Missing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Неразобранные строки с сохранённой историей парсера нужны для
			// проверки Raw независимо от наличия tag/block-open и ошибок формы.
			result := v046Document(t, v046Unparsed(t, tc.raw, 2, 1), v046Text(t, "Read.", 2, 1))
			v046PriorError(t, result, 3, 0, "Объявление не разобрано.")
			v046Run(t, result, "current", tc.want)
		})
	}
}

func TestV046NamesAndContexts(t *testing.T) {
	// Каждый тег получает свой допустимый контекст и непустое содержимое.
	// Ошибочный вариант отличается только удалением разделителя перед {.
	for _, tc := range []struct {
		tag          string
		line, column int
	}{
		{"editor", 3, 8}, {"instr", 3, 7}, {"hint", 5, 6}, {"answer", 4, 8},
		{"example", 3, 9}, {"wordlist", 3, 10}, {"note", 3, 6}, {"alt", 3, 5},
		{"table", 3, 7}, {"script", 4, 8}, {"text", 3, 6}, {"key", 5, 5},
		{"choice", 3, 8}, {"multichoice", 3, 13}, {"matching", 3, 10},
		{"ordering", 3, 10}, {"fragment", 3, 12}, {"variants", 3, 10}, {"multifill", 3, 11},
	} {
		for _, gap := range []string{" ", ""} {
			t.Run(tc.tag+fmt.Sprintf("/gap=%q", gap), func(t *testing.T) {
				result := v046TagDocument(t, tc.tag, gap)
				var want []v046Expected
				if gap == "" {
					want = []v046Expected{{tc.line, tc.column, tc.column + 1, v046Missing}}
				}
				v046Run(t, result, "current", want)
			})
		}
	}
	t.Run("internal tab in title", func(t *testing.T) {
		result := v046Document(t, v046Line(t, "@text A\tB {", "block-start", 2, 1,
			v046Element(t, "tag", "@text", "text", 1, 6), v046Element(t, "title", "A\tB", "A\tB", 7, 10),
			v046Element(t, "block-open", "{", "{", 11, 12)), v046Text(t, "Read.", 3, 2), v046Close(t, 3, 2))
		v046Run(t, result, "current", nil)
		// Повторный запуск сохраняет прежние ссылки и добавляет новые.
		v046Run(t, result, "second", nil)
	})
}

// v046TagDocument задаёт структуру для каждого тега по разделам 2–4 DSL.
// Выбор контекста явный; Raw не разбирается. Длины служат только сборке входа.
func v046TagDocument(t *testing.T, tag, gap string) *model.Result {
	t.Helper()
	var before []model.Line
	parent, depth := 2, 1
	switch tag {
	case "hint":
		before = []model.Line{v046Open(t, "instr", 2, 1), v046Text(t, "Read the text.", 3, 2)}
		parent, depth = 3, 2
	case "answer":
		before = []model.Line{v046Line(t, "@question Which city?", "tag", 2, 1,
			v046Element(t, "tag", "@question", "question", 1, 10), v046Element(t, "content", "Which city?", "Which city?", 11, 22))}
	case "key":
		before = []model.Line{v046Line(t, "@task a", "tag", 2, 1, v046Element(t, "tag", "@task", "task", 1, 6), v046Element(t, "identifier", "a", "a", 7, 8)),
			v046Text(t, "Name the capital of France.", 3, 2)}
		parent, depth = 3, 2
	case "script":
		before = []model.Line{v046Line(t, "@media audio track.mp3", "tag", 2, 1,
			v046Element(t, "tag", "@media", "media", 1, 7), v046Element(t, "media-type", "audio", "audio", 8, 13),
			v046Element(t, "source", "track.mp3", "track.mp3", 14, 23))}
	}
	opening := len(before) + 3
	raw := "@" + tag
	end := utf8.RuneCountInString(raw) + 1
	elements := []model.Element{v046Element(t, "tag", raw, tag, 1, end)}
	if tag == "fragment" {
		raw += " f"
		elements = append(elements, v046Element(t, "identifier", "f", "f", 11, 12))
	}
	raw += gap + "{"
	brace := utf8.RuneCountInString(raw)
	elements = append(elements, v046Element(t, "block-open", "{", "{", brace, brace+1))
	body := append(before, v046Line(t, raw, "block-start", parent, depth, elements...))
	texts := []string{"Read."}
	switch tag {
	case "editor":
		texts = []string{"Check the source."}
	case "hint":
		texts = []string{"Find the subject first."}
	case "answer":
		texts = []string{"London"}
	case "wordlist":
		texts = []string{"book", "pen"}
	case "alt":
		texts = []string{"На столе лежит книга."}
	case "table":
		texts = []string{"Name | Colour", "Sky | Blue"}
	case "script":
		texts = []string{"Hello!"}
	case "key":
		texts = []string{"Paris"}
	case "choice", "multichoice":
		texts = []string{"Red", "Blue"}
	case "ordering":
		texts = []string{"Mix the ingredients", "Bake the cake"}
	case "matching":
		texts = []string{"Is water wet?", "---", "Yes", "No"}
	case "variants":
		texts = nil
	}
	for _, text := range texts {
		if text == "---" {
			body = append(body, v046Line(t, text, "separator", opening, depth+1))
		} else {
			body = append(body, v046Text(t, text, opening, depth+1))
		}
	}
	if tag == "variants" {
		body = append(body,
			v046Line(t, "@variant A", "tag", 3, 2, v046Element(t, "tag", "@variant", "variant", 1, 9), v046Element(t, "name", "A", "A", 10, 11)),
			v046Text(t, "Read A.", 4, 3),
			v046Line(t, "@variant B", "tag", 3, 2, v046Element(t, "tag", "@variant", "variant", 1, 9), v046Element(t, "name", "B", "B", 10, 11)),
			v046Text(t, "Read B.", 6, 3))
	}
	body = append(body, v046Close(t, opening, depth+1))
	if tag == "hint" {
		body = append(body, v046Close(t, 3, 2))
	}
	if tag == "editor" {
		body = append(body, v046Text(t, "Read.", 2, 1))
	}
	if tag == "fragment" {
		body = append(body, v046Line(t, "@include f", "tag", 2, 1, v046Element(t, "tag", "@include", "include", 1, 9), v046Element(t, "identifier", "f", "f", 10, 11)))
	}
	return v046Document(t, body...)
}

func TestV046Escaping(t *testing.T) {
	for _, tc := range []struct {
		name, title, value string
		open               int
		want               []v046Expected
	}{
		{"escaped opening", `\{`, `{`, 0, nil},
		{"even slashes", `\\`, `\`, 9, []v046Expected{{3, 9, 10, v046Missing}}},
		{"odd slashes", `\\\{`, `\{`, 0, nil},
		{"four slashes", `\\\\`, `\\`, 11, []v046Expected{{3, 11, 12, v046Missing}}},
		{"opening after escaped one", `\{ X`, `{ X`, 11, []v046Expected{{3, 11, 12, v046Missing}}},
		{"escaped closing", `\}`, `}`, 9, []v046Expected{{3, 9, 10, v046Missing}}},
		{"backslash before ordinary character", `\q`, `\q`, 9, []v046Expected{{3, 9, 10, v046Missing}}},
		{"escaped underscores", `\_____`, `_____`, 13, []v046Expected{{3, 13, 14, v046Missing}}},
		{"four underscores", `____`, `____`, 11, []v046Expected{{3, 11, 12, v046Missing}}},
		{"six underscores", `______`, `______`, 13, []v046Expected{{3, 13, 14, v046Missing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "@text " + tc.title
			elements := []model.Element{v046Element(t, "tag", "@text", "text", 1, 6),
				v046Element(t, "title", tc.title, tc.value, 7, 7+utf8.RuneCountInString(tc.title))}
			if tc.open == 0 {
				// У text нет однострочной формы: сохраняем ошибочную строку без блока.
				result := v046Document(t, v046Unparsed(t, raw, 2, 1), v046Text(t, "Read.", 2, 1))
				v046PriorError(t, result, 3, 0, "Не найдено открытие text.")
				v046Run(t, result, "current", tc.want)
				return
			}
			raw += "{"
			elements = append(elements, v046Element(t, "block-open", "{", "{", tc.open, tc.open+1))
			result := v046Document(t, v046Line(t, raw, "block-start", 2, 1, elements...),
				v046Text(t, "Read.", 3, 2), v046Close(t, 3, 2))
			v046Run(t, result, "current", tc.want)
		})
	}
}

func TestV046Placeholders(t *testing.T) {
	for _, tc := range []struct {
		name, instruction, gap string
		brace                  int
		want                   []v046Expected
	}{
		{"answer then spaced opening", "_____{yes}", " ", 23, nil},
		{"answer then tab", "_____{yes}", "\t", 23, []v046Expected{{3, 22, 23, v046Wrong}}},
		{"answer then adjacent opening", "_____{yes}", "", 22, []v046Expected{{3, 22, 23, v046Missing}}},
		{"escaped answer closing", `_____{a\}b}`, "\t", 24, []v046Expected{{3, 23, 24, v046Wrong}}},
		{"escaped answer opening", `_____{a\{b}`, "\t", 24, []v046Expected{{3, 23, 24, v046Wrong}}},
		{"paired backslashes in answer", `_____{a\\}`, "\t", 23, []v046Expected{{3, 22, 23, v046Wrong}}},
		{"two answers", "_____{a} _____{b}", "\t", 30, []v046Expected{{3, 29, 30, v046Wrong}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// В инструкции форма ответа запрещена DSL 4.3.10, но V046 прямо
			// исключает её скобки. Сохраняем ошибочный аргумент как unparsed.
			end := 12 + utf8.RuneCountInString(tc.instruction)
			result := v046Document(t, v046Line(t, "@multifill "+tc.instruction+tc.gap+"{", "block-start", 2, 1,
				v046Element(t, "tag", "@multifill", "multifill", 1, 11),
				model.Element{ElementType: "unparsed", Raw: tc.instruction, Start: 12, End: end, ErrorIDs: []string{}},
				v046Element(t, "block-open", "{", "{", tc.brace, tc.brace+1)),
				v046Text(t, "@note{", 3, 2), v046Close(t, 3, 2),
				v046Line(t, "@note{", "block-start", 2, 1, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 6, 7)),
				v046Text(t, "Read.", 6, 2), v046Close(t, 6, 2))
			v046PriorError(t, result, 3, 1, "Ответ плейсхолдера недопустим в инструкции.")
			want := append(append([]v046Expected{}, tc.want...), v046Expected{6, 6, 7, v046Missing})
			v046Run(t, result, "current", want)
		})
	}
	t.Run("answer in literal body", func(t *testing.T) {
		result := v046Document(t, v046Open(t, "multifill", 2, 1),
			v046Line(t, "_____{yes}", "content", 3, 2, v046Element(t, "placeholder", "_____{yes}", "yes", 1, 11)), v046Close(t, 3, 2))
		v046Run(t, result, "current", nil)
	})
	t.Run("answer is not a block opening", func(t *testing.T) {
		result := v046Document(t, v046Unparsed(t, "@multifill _____{yes}", 2, 1),
			v046Line(t, "@note{", "block-start", 2, 1, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 6, 7)),
			v046Text(t, "Read.", 4, 2), v046Close(t, 4, 2))
		v046PriorError(t, result, 3, 0, "Ответ плейсхолдера недопустим в инструкции.")
		v046Run(t, result, "current", []v046Expected{{4, 6, 7, v046Missing}})
	})
}

func TestV046LiteralContexts(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		column int
	}{{"editor", 8}, {"multifill", 11}} {
		for _, unparsed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unparsed=%v", tc.tag, unparsed), func(t *testing.T) {
				first := v046Line(t, "@"+tc.tag+"{", "block-start", 2, 1,
					v046Element(t, "tag", "@"+tc.tag, tc.tag, 1, tc.column), v046Element(t, "block-open", "{", "{", tc.column, tc.column+1))
				close := v046Line(t, " \t} \t", "block-end", 3, 2, v046Element(t, "block-close", "}", "}", 3, 4))
				if unparsed {
					first = v046Unparsed(t, "@"+tc.tag+"{", 2, 1)
					// Парсер не восстановил блок; V046 обязана восстановить свой
					// контекст по Raw. Модель сохраняет ошибочные строки без него.
					close = v046Unparsed(t, " \t} \t", 2, 1)
				}
				parent, depth := 3, 2
				if unparsed {
					parent, depth = 2, 1
				}
				result := v046Document(t, first, v046Text(t, "@note{", parent, depth),
					v046Line(t, `\}`, "content", parent, depth, v046Element(t, "content", `\}`, "}", 1, 3)),
					v046Text(t, "@editor{", parent, depth), close,
					v046Line(t, "@note{", "block-start", 2, 1, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 6, 7)),
					v046Text(t, "Read.", 8, 2), v046Close(t, 8, 2))
				if unparsed {
					v046PriorError(t, result, 3, 0, "Не удалось разобрать объявление блока.")
					v046PriorError(t, result, 7, 0, "Не найдено открытие блока.")
				}
				v046Run(t, result, "current", []v046Expected{{3, tc.column, tc.column + 1, v046Missing}, {8, 6, 7, v046Missing}})
			})
		}
	}
	t.Run("nested literal block closes only itself", func(t *testing.T) {
		// editor до первой ветви variants разрешён DSL 4.9.8.
		result := v046Document(t, v046Open(t, "variants", 2, 1), v046Open(t, "editor", 3, 2),
			v046Text(t, "@note{", 4, 3), v046Close(t, 4, 3),
			v046Line(t, "@variant A", "tag", 3, 2, v046Element(t, "tag", "@variant", "variant", 1, 9), v046Element(t, "name", "A", "A", 10, 11)),
			v046Line(t, "@note{", "block-start", 7, 3, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 6, 7)),
			v046Text(t, "Read A.", 8, 4), v046Close(t, 8, 4),
			v046Line(t, "@variant B", "tag", 3, 2, v046Element(t, "tag", "@variant", "variant", 1, 9), v046Element(t, "name", "B", "B", 10, 11)),
			v046Text(t, "Read B.", 11, 3), v046Close(t, 3, 2),
			v046Line(t, "@note{", "block-start", 2, 1, v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "block-open", "{", "{", 6, 7)),
			v046Text(t, "Read C.", 14, 2), v046Close(t, 14, 2))
		v046Run(t, result, "current", []v046Expected{{8, 6, 7, v046Missing}, {14, 6, 7, v046Missing}})
	})
}

func TestV046ExcludedAndEmpty(t *testing.T) {
	v046Run(t, v046UnavailableDocument(t, "1.2"), "current", nil)
	t.Run("no declaration", func(t *testing.T) {
		v046Run(t, v046Document(t, v046Text(t, "Read.", 2, 1), v046Line(t, " \t", "blank", 2, 1)), "current", nil)
	})
	t.Run("inline note", func(t *testing.T) {
		v046Run(t, v046Document(t, v046Line(t, "@note Read.", "tag", 2, 1,
			v046Element(t, "tag", "@note", "note", 1, 6), v046Element(t, "content", "Read.", "Read.", 7, 12))), "current", nil)
	})
	t.Run("escaped content", func(t *testing.T) {
		v046Run(t, v046Document(t, v046Line(t, `\@note\{`, "content", 2, 1,
			v046Element(t, "content", `\@note\{`, "@note{", 1, 9))), "current", nil)
	})
	t.Run("resource path", func(t *testing.T) {
		result := v046Document(t, v046Text(t, "Read.", 2, 1))
		line := v046Line(t, "@resource-dir folder{", "tag", 0, 0,
			v046Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v046Element(t, "resource-path", "folder{", "folder{", 15, 22))
		result.Lines = append([]model.Line{result.Lines[0], line}, result.Lines[1:]...)
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		parent, count := 3, 4
		result.Lines[3].ParentLine = &parent
		result.Document.LineCount = &count
		result.Document.Metadata.ResourceDirs = []string{"folder{"}
		v046Run(t, result, "current", nil)
	})
}

func TestV046BlockBoundaries(t *testing.T) {
	for _, raw := range []string{"{", "}"} {
		t.Run("standalone "+raw, func(t *testing.T) {
			result := v046Document(t, v046Unparsed(t, raw, 2, 1), v046Text(t, "Read.", 2, 1))
			v046PriorError(t, result, 3, 0, "Лишняя скобка.")
			v046Run(t, result, "current", nil)
		})
	}
	t.Run("unclosed block", func(t *testing.T) {
		result := v046Document(t, v046Open(t, "note", 2, 1), v046Text(t, "Read.", 3, 2))
		v046PriorError(t, result, 3, 1, "Блок не закрыт.")
		v046Run(t, result, "current", nil)
	})
	t.Run("unclosed literal block", func(t *testing.T) {
		result := v046Document(t, v046Open(t, "editor", 2, 1), v046Text(t, "@note{", 3, 2))
		v046PriorError(t, result, 3, 1, "Блок не закрыт.")
		v046Run(t, result, "current", nil)
	})
	t.Run("closing must be alone", func(t *testing.T) {
		result := v046Document(t, v046Open(t, "editor", 2, 1), v046Text(t, "} extra", 3, 2),
			v046Text(t, "@note{", 3, 2), v046Close(t, 3, 2), v046Text(t, "Read.", 2, 1))
		v046Run(t, result, "current", nil)
	})
	t.Run("Unicode whitespace is not closing-line trim", func(t *testing.T) {
		result := v046Document(t, v046Open(t, "editor", 2, 1), v046Text(t, "\u00a0}", 3, 2),
			v046Text(t, "@note{", 3, 2), v046Close(t, 3, 2), v046Text(t, "Read.", 2, 1))
		v046Run(t, result, "current", nil)
	})
	t.Run("last line without EOL", func(t *testing.T) {
		result := v046Document(t, v046Text(t, "Read.", 2, 1), v046Unparsed(t, "@note{", 2, 1))
		result.Lines[3].LineEnding = ""
		v046PriorError(t, result, 4, 0, "Незавершённое ошибочное объявление.")
		v046Run(t, result, "current", []v046Expected{{4, 6, 7, v046Missing}})
	})
	t.Run("first physical line", func(t *testing.T) {
		// DSLVersion уже определена парсером, но метаданные стоят после
		// комментария: существующая ошибка размещения не исключает V046.
		result := v046UnavailableDocument(t, "1.2")
		result.Lines = []model.Line{
			v046Line(t, "@editor{", "block-start", 0, 0, v046Element(t, "tag", "@editor", "editor", 1, 8), v046Element(t, "block-open", "{", "{", 8, 9)),
			v046Text(t, "Check source.", 1, 1), v046Close(t, 1, 1),
			v046Line(t, "@dsl-version 1.2", "tag", 0, 0, v046Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v046Element(t, "version", "1.2", "1.2", 14, 17)),
			v046Line(t, "# Unit 1", "heading", 0, 0, v046Element(t, "heading-level", "#", "1", 1, 2), v046Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
			v046Text(t, "Read.", 5, 1),
		}
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		count, title := 6, "Unit 1"
		result.Document.LineCount, result.Document.Metadata.Title = &count, &title
		v046PriorError(t, result, 1, 0, "Содержимое перед объявлением версии.")
		v046Run(t, result, "current", []v046Expected{{1, 8, 9, v046Missing}})
	})
}

func TestV046LineEndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name string
		eol  model.LineEnding
		bom  bool
	}{
		{"LF", "\n", false}, {"CRLF", "\r\n", false}, {"BOM", "\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v046TagDocument(t, "note", "")
			result.Document.HasBOM = &tc.bom
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			v046Run(t, result, "current", []v046Expected{{3, 6, 7, v046Missing}})
		})
	}
	t.Run("repeated runs and no cross-document state", func(t *testing.T) {
		result := v046Document(t, v046Unparsed(t, "@editor{", 2, 1), v046Text(t, "@note{", 2, 1))
		v046PriorError(t, result, 3, 0, "Незавершённое ошибочное объявление.")
		v046Run(t, result, "first", []v046Expected{{3, 8, 9, v046Missing}})
		v046Run(t, result, "second", []v046Expected{{3, 8, 9, v046Missing}})
		v046Run(t, v046TagDocument(t, "note", ""), "first", []v046Expected{{3, 6, 7, v046Missing}})
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v046UnavailableDocument(t, version)
			before := v046Snapshot(t, result)
			checks, err := validator.Prepare(result, v046Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) did not refuse: %v, %v", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v046Snapshot(t, result) != before {
				t.Fatal("model changed on refusal")
			}
		})
	}
}

// v046Unparsed сохраняет исходную строку без распознанных элементов.
// Вызывающий обязан добавить связанную ошибку парсера до запуска конвейера.
func v046Unparsed(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v046Line(t, raw, "invalid", parent, depth,
		model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
}

// v046PriorError добавляет согласованную историю парсера на явно выбранный
// элемент. Все вызовы в ситуациях идут в исходном порядке строк и элементов.
func v046PriorError(t *testing.T, result *model.Result, lineNumber, elementIndex int, message string) {
	t.Helper()
	id := fmt.Sprintf("parser-%d", len(result.Diagnostics)+1)
	line := &result.Lines[lineNumber-1]
	element := &line.Elements[elementIndex]
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: message,
		Location: &model.Location{Start: model.Position{Line: lineNumber, Column: element.Start}, End: model.Position{Line: lineNumber, Column: element.End}}, RelatedLocations: []model.Location{},
	})
}

// v046Open создаёт объявление блока без параметров с явными родителем и глубиной.
func v046Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v046Line(t, "@"+tag+" {", "block-start", parent, depth,
		v046Element(t, "tag", "@"+tag, tag, 1, end), v046Element(t, "block-open", "{", "{", end+1, end+2))
}

// v046Close создаёт закрывающую строку указанного блока.
func v046Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v046Line(t, "}", "block-end", parent, depth, v046Element(t, "block-close", "}", "}", 1, 2))
}

// v046Registry создаёт реестр через штатный каталог без подмены регистраций.
func v046Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v046UnavailableDocument сохраняет известную версию при недоступных строках.
func v046UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v046Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v046Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v046Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v046Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v046Element(t, "version", "1.2", "1.2", 14, 17)),
		v046Line(t, "# Unit 1", "heading", 0, 0,
			v046Element(t, "heading-level", "#", "1", 1, 2),
			v046Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}},
		},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       lines,
		Diagnostics: []model.Diagnostic{},
	}
}

// v046Line собирает строку с явно заданной структурой, без классификации Raw.
func v046Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v046Element сохраняет исходное и семантическое значения и явный диапазон.
func v046Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v046Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v046Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v046Line(t, text, "content", parent, depth,
		v046Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v046Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v046Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v046Run проверяет V046 через штатный каталог и конвейер. Диапазоны want
// заданы независимо. Сохраняются исходные данные, история и прежние ссылки.
// Новые ссылки других диагностик проверяются по их диапазонам и через CheckModel.
func v046Run(t *testing.T, result *model.Result, processingID string, want []v046Expected) {
	t.Helper()
	before := v046Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v046Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v046Snapshot(t, result) != before {
		t.Fatal("Prepare changed model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.Source != processingID {
			t.Fatalf("unexpected source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V046" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V046 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("location = %+v, want %+v", entry.Location, location)
		}
		if entry.Message != "Перед открывающей фигурной скобкой отсутствует разделитель из обычных пробелов.\nконкретизация: "+expected.detail || entry.DiagnosticCode != "V046" ||
			entry.Source != processingID || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
	}
	// Срабатывания проверяемой диагностики заданы want. Ошибки других
	// зарегистрированных кодов также устанавливают общие флаги модели.
	otherDocumentError := false
	otherLineErrors := make(map[int]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V046" && entry.SeverityLevel == "error" {
			otherDocumentError = true
			if entry.Location != nil {
				otherLineErrors[entry.Location.Start.Line] = true
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	if result.Document.HasErrors != (original.Document.HasErrors || len(want) > 0 || otherDocumentError) {
		t.Error("unexpected document error flag")
	}
	if len(result.Lines) != len(original.Lines) {
		t.Fatal("line count changed")
	}
	for i, line := range result.Lines {
		expectedFlag := original.Lines[i].HasErrors || otherLineErrors[i+1]
		for _, expected := range want {
			if expected.line == i+1 {
				expectedFlag = true
			}
		}
		if line.HasErrors != expectedFlag {
			t.Errorf("line %d error flag = %v, want %v", i+1, line.HasErrors, expectedFlag)
		}
		if len(line.Elements) != len(original.Lines[i].Elements) {
			t.Fatalf("line %d element count changed", i+1)
		}
		for j, element := range line.Elements {
			expectedLinks := append([]string{}, original.Lines[i].Elements[j].ErrorIDs...)
			for _, entry := range result.Diagnostics[diagnosticCount:] {
				location := entry.Location
				if entry.DiagnosticScope == "element" && entry.DiagnosticCode != "V046" && location != nil &&
					location.Start.Line == line.Number && location.End.Line == line.Number &&
					location.Start.Column == element.Start && location.End.Column == element.End {
					expectedLinks = append(expectedLinks, entry.ID)
				}
			}
			if element.ErrorIDs == nil || !slices.Equal(element.ErrorIDs, expectedLinks) {
				t.Errorf("line %d element %d links = %v, want %v", i+1, j, element.ErrorIDs, expectedLinks)
			}
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("lost history or unexpected processing")
	}
	// После отдельной проверки разрешённых изменений сравниваем весь остаток
	// модели со снимком, не меняя проверяемый result.
	var previous model.Result
	if err := json.Unmarshal([]byte(v046Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	previous.Processing = previous.Processing[:processingCount]
	previous.Diagnostics = previous.Diagnostics[:diagnosticCount]
	previous.Document.HasErrors = original.Document.HasErrors
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
	}
	// В снимке исключаются новые ссылки других элементных диагностик каталога.
	newElementIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[len(previous.Diagnostics):] {
		if entry.DiagnosticScope == "element" && entry.DiagnosticCode != "V046" {
			newElementIDs[entry.ID] = true
		}
	}
	previous.Lines = append([]model.Line{}, previous.Lines...)
	for i := range previous.Lines {
		previous.Lines[i].Elements = append([]model.Element{}, previous.Lines[i].Elements...)
		for j := range previous.Lines[i].Elements {
			element := &previous.Lines[i].Elements[j]
			element.ErrorIDs = slices.DeleteFunc(slices.Clone(element.ErrorIDs), func(id string) bool { return newElementIDs[id] })
		}
	}
	if v046Snapshot(t, &previous) != before {
		t.Fatal("source model or previous history changed")
	}
}

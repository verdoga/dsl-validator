package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV035SelectedBlocks(t *testing.T) {
	for _, tc := range []struct {
		tag, argument, text, context string
		want                         [4]int
	}{
		{"editor", "", "Проверьте формулировку.", "", [4]int{3, 1, 5, 2}},
		{"example", "", "She lives in Rome.", "", [4]int{3, 1, 5, 2}},
		{"wordlist", "", "Italian", "", [4]int{3, 1, 5, 2}},
		{"table", "", "Word | Meaning", "", [4]int{3, 1, 5, 2}},
		{"script", "", "A: Hello!", "", [4]int{3, 1, 5, 2}},
		{"text", "", "Read carefully.", "", [4]int{3, 1, 5, 2}},
		{"key", "", "Ответ: Анна.", "task", [4]int{5, 1, 7, 2}},
		{"instr", "", "Прочитайте предложение.", "", [4]int{3, 1, 5, 2}},
		{"note", "", "Обратите внимание.", "", [4]int{3, 1, 5, 2}},
		{"alt", "", "На фотографии дом.", "", [4]int{3, 1, 5, 2}},
		{"hint", "", "Вспомните правило.", "", [4]int{3, 1, 5, 2}},
		{"multifill", "", "Напишите ответ.", "", [4]int{3, 1, 5, 2}},
		{"fragment", "f", "Текст.", "", [4]int{3, 1, 5, 2}},
		{"answer", "", "Анна", "question", [4]int{4, 1, 6, 2}},
	} {
		for _, filled := range []bool{false, true} {
			name := "empty"
			if filled {
				name = "filled"
			}
			t.Run(tc.tag+"/"+name, func(t *testing.T) {
				var body []model.Line
				parent, depth := 2, 1
				switch tc.context {
				case "task":
					body = append(body, v035TestTag(t, "task", "t1", "identifier", 2, 1),
						v035TestTag(t, "instr", "Назовите имя.", "content", 3, 2))
					parent, depth = 3, 2
				case "question":
					body = append(body, v035TestTag(t, "question", "Как вас зовут?", "content", 2, 1))
				}
				opening := len(body) + 3
				body = append(body, v035TestOpen(t, tc.tag, tc.argument, parent, depth))
				var want [][4]int
				if filled {
					body = append(body, v035TestText(t, tc.text, opening, depth+1))
				} else {
					body = append(body, v035TestLine(t, "", "blank", opening, depth+1))
					want = [][4]int{tc.want}
				}
				body = append(body, v035TestClose(t, opening, depth+1))
				v035TestRun(t, v035TestDocument(t, body...), "current", want)
			})
		}
	}
}

func TestV035BasicContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		// Эталон и пример неизменны; две обязательные строки контекста
		// добавляет v035TestDocument, поэтому блок начинается на строке 3.
		{"reference", []model.Line{
			v035TestOpen(t, "note", "Reminder", 2, 1), v035TestText(t, "Read carefully.", 3, 2), v035TestClose(t, 3, 2),
		}, nil},
		{"example", []model.Line{
			v035TestOpen(t, "note", "Reminder", 2, 1), v035TestLine(t, "", "blank", 3, 2), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 5, 2}}},
		{"immediate closing", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 4, 2}}},
		{"spaces and tabs", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestLine(t, " \t ", "blank", 3, 2), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 5, 2}}},
		{"several blanks", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestLine(t, " ", "blank", 3, 2),
			v035TestLine(t, "\t", "blank", 3, 2), v035TestLine(t, "", "blank", 3, 2), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 7, 2}}},
		{"one nonempty character", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestText(t, "Я", 3, 2), v035TestClose(t, 3, 2),
		}, nil},
		{"edge whitespace around content", []model.Line{
			v035TestOpen(t, "note", "", 2, 1),
			v035TestLine(t, " \tЯ \t", "content", 3, 2, v035TestElement(t, "content", "Я", "Я", 3, 4)),
			v035TestClose(t, 3, 2),
		}, nil},
		{"nonbreaking space is not removed", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestText(t, "\u00A0", 3, 2), v035TestClose(t, 3, 2),
		}, nil},
		{"only block name", []model.Line{
			v035TestOpen(t, "note", "Очень важное", 2, 1), v035TestLine(t, "", "blank", 3, 2), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 5, 2}}},
		{"structural separator is not content", []model.Line{
			v035TestOpen(t, "example", "", 2, 1), v035TestLine(t, "---", "separator", 3, 2), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 5, 2}}},
		{"independent neighboring blocks", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestLine(t, "", "blank", 3, 2), v035TestClose(t, 3, 2),
			v035TestOpen(t, "hint", "", 2, 1), v035TestLine(t, "", "blank", 6, 2), v035TestClose(t, 6, 2),
		}, [][4]int{{3, 1, 5, 2}, {6, 1, 8, 2}}},
		{"visual blank line does not fill text", []model.Line{
			v035TestOpen(t, "text", "", 2, 1), v035TestText(t, "<br>", 3, 2), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 5, 2}}},
		{"table row is nonempty source content", []model.Line{
			v035TestOpen(t, "table", "", 2, 1), v035TestText(t, "|", 3, 2), v035TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v035TestRun(t, v035TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		for _, text := range []string{"---", "@note Literal text.", "# Literal heading"} {
			t.Run(tag+" literal "+text, func(t *testing.T) {
				v035TestRun(t, v035TestDocument(t, v035TestOpen(t, tag, "", 2, 1),
					v035TestText(t, text, 3, 2), v035TestClose(t, 3, 2)), "current", nil)
			})
		}
	}
}

func TestV035FormattingContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		empty     bool
	}{
		{"empty emphasis", "<em></em>", true},
		{"nested formatting", "<strong><u> \t</u></strong>", true},
		{"adjacent formatting", "<em></em> <u></u>", true},
		{"marker case", "<EM> </eM>", true},
		{"indented visual blank", " \t<BR> \t", true},
		{"HTML balance is not checked here", "<em>", true},
		{"visible text", "<em>Hello</em>", false},
		{"text after empty formatting", "<em></em>Я", false},
		{"nonbreaking space stays content", "<em>\u00a0</em>", false},
		{"escaped markers stay literal", `\<em>\</em>`, false},
		{"unknown markers are not stripped", "<span></span>", false},
		{"attributes are not stripped", `<em class="x"></em>`, false},
		{"unsupported br form", "<br/>", false},
		{"inline br is not supported formatting", "<em></em><br>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want [][4]int
			if tc.empty {
				want = [][4]int{{3, 1, 5, 2}}
			}
			v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "text", "", 2, 1),
				v035TestText(t, tc.raw, 3, 2), v035TestClose(t, 3, 2)), "current", want)
		})
	}
	for _, tag := range []string{"editor", "answer", "note", "table"} {
		t.Run("literal context "+tag, func(t *testing.T) {
			v035TestRun(t, v035TestDocument(t, v035TestOpen(t, tag, "", 2, 1),
				v035TestText(t, "<em></em>", 3, 2), v035TestClose(t, 3, 2)), "current", nil)
		})
	}
	for _, texts := range [][]string{{"<em></em>", "<br>"}, {"<em></em>", "<u>Read.</u>"}, {"Read.", "<em></em>"}} {
		var want [][4]int
		if texts[1] == "<br>" {
			want = [][4]int{{3, 1, 6, 2}}
		}
		v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "text", "", 2, 1),
			v035TestText(t, texts[0], 3, 2), v035TestText(t, texts[1], 3, 2),
			v035TestClose(t, 3, 2)), "multiple content lines", want)
	}
	v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "fragment", "f", 2, 1),
		v035TestOpen(t, "text", "", 3, 2), v035TestText(t, "<em></em>", 4, 3),
		v035TestClose(t, 4, 3), v035TestClose(t, 3, 2)), "empty nested text",
		[][4]int{{3, 1, 7, 2}, {4, 1, 6, 2}})
	v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "fragment", "f", 2, 1),
		v035TestOpen(t, "text", "", 3, 2), v035TestText(t, "<em>Hello</em>", 4, 3),
		v035TestClose(t, 4, 3), v035TestClose(t, 3, 2)), "filled nested text", nil)
	// Даже ошибочное вложение editor в text не меняет литеральный контекст editor.
	v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "text", "", 2, 1),
		v035TestOpen(t, "editor", "", 3, 2), v035TestText(t, "<em></em>", 4, 3),
		v035TestClose(t, 4, 3), v035TestClose(t, 3, 2)), "literal child of text", [][4]int{{3, 1, 7, 2}})
}

func TestV035NestedContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"empty parent and child", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestOpen(t, "note", "", 3, 2),
			v035TestClose(t, 4, 3), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 6, 2}, {4, 1, 5, 2}}},
		{"child name does not fill parent", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestOpen(t, "note", "Reminder", 3, 2),
			v035TestClose(t, 4, 3), v035TestClose(t, 3, 2),
		}, [][4]int{{3, 1, 6, 2}, {4, 1, 5, 2}}},
		{"filled child fills parent", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestOpen(t, "note", "", 3, 2),
			v035TestText(t, "Text.", 4, 3), v035TestClose(t, 4, 3), v035TestClose(t, 3, 2),
		}, nil},
		{"single-line child fills parent", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestTag(t, "note", "Text.", "content", 3, 2), v035TestClose(t, 3, 2),
		}, nil},
		{"only empty child is reported", []model.Line{
			v035TestOpen(t, "instr", "", 2, 1), v035TestText(t, "Read.", 3, 2),
			v035TestOpen(t, "hint", "", 3, 2), v035TestLine(t, "", "blank", 5, 3),
			v035TestClose(t, 5, 3), v035TestClose(t, 3, 2),
		}, [][4]int{{5, 1, 7, 2}}},
		{"several descendant levels", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestOpen(t, "instr", "", 3, 2),
			v035TestText(t, "Read.", 4, 3), v035TestOpen(t, "hint", "", 4, 3),
			v035TestText(t, "Remember the rule.", 6, 4), v035TestClose(t, 6, 4),
			v035TestClose(t, 4, 3), v035TestClose(t, 3, 2),
		}, nil},
		{"answer in excluded choice", []model.Line{
			v035TestOpen(t, "choice", "", 2, 1), v035TestText(t, "Yes", 3, 2), v035TestText(t, "No", 3, 2),
			v035TestOpen(t, "answer", "", 3, 2), v035TestLine(t, "", "blank", 6, 3),
			v035TestClose(t, 6, 3), v035TestClose(t, 3, 2),
		}, [][4]int{{6, 1, 8, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v035TestRun(t, v035TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		kind, source string
	}{
		{"image", "photo.png"}, {"audio", "clip.mp3"}, {"video", "clip.mp4"}, {"audio", "12"},
	} {
		t.Run("media "+tc.kind+" "+tc.source, func(t *testing.T) {
			raw := "@media " + tc.kind + " " + tc.source
			start := 9 + len(tc.kind)
			media := v035TestLine(t, raw, "tag", 3, 2,
				v035TestElement(t, "tag", "@media", "media", 1, 7),
				v035TestElement(t, "media-type", tc.kind, tc.kind, 8, 8+len(tc.kind)),
				v035TestElement(t, "source", tc.source, tc.source, start, start+len(tc.source)))
			v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "fragment", "f", 2, 1), media,
				v035TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV035UnparsedAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		bad  int
		end  int
		want [][4]int
	}{
		{"invalid inside", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestUnparsed(t, "@bad", "invalid", 3, 2), v035TestClose(t, 3, 2),
		}, 4, 5, nil},
		{"unparsed inside content", []model.Line{
			v035TestOpen(t, "text", "", 2, 1), v035TestUnparsed(t, "<bad>", "content", 3, 2), v035TestClose(t, 3, 2),
		}, 4, 6, nil},
		{"unparsed in descendant", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestOpen(t, "note", "", 3, 2),
			v035TestUnparsed(t, "@bad", "invalid", 4, 3), v035TestClose(t, 4, 3), v035TestClose(t, 3, 2),
		}, 5, 5, nil},
		{"unparsed does not suppress an independent sibling", []model.Line{
			v035TestOpen(t, "fragment", "f", 2, 1), v035TestOpen(t, "note", "", 3, 2),
			v035TestUnparsed(t, "@bad", "invalid", 4, 3), v035TestClose(t, 4, 3),
			v035TestOpen(t, "hint", "", 3, 2), v035TestLine(t, "", "blank", 7, 3),
			v035TestClose(t, 7, 3), v035TestClose(t, 3, 2),
		}, 5, 5, [][4]int{{7, 1, 9, 2}}},
		{"unparsed outside does not suppress empty block", []model.Line{
			v035TestOpen(t, "note", "", 2, 1), v035TestLine(t, "", "blank", 3, 2),
			v035TestClose(t, 3, 2), v035TestUnparsed(t, "@bad", "invalid", 2, 1),
		}, 6, 5, [][4]int{{3, 1, 5, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v035TestDocument(t, tc.body...)
			v035TestParserError(t, result, tc.bad, tc.end, "element")
			v035TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("missing closing", func(t *testing.T) {
		result := v035TestDocument(t, v035TestOpen(t, "note", "", 2, 1))
		v035TestParserError(t, result, 3, 8, "block")
		v035TestRun(t, result, "current", nil)
	})
	t.Run("ambiguous closing links", func(t *testing.T) {
		// Контракт CheckModel допускает две block-end с одним родителем.
		// При такой неоднозначности V035 не выбирает удобное закрытие.
		result := v035TestDocument(t, v035TestOpen(t, "note", "", 2, 1),
			v035TestClose(t, 3, 2), v035TestClose(t, 3, 2))
		v035TestParserError(t, result, 5, 2, "line")
		v035TestRun(t, result, "current", nil)
	})
	t.Run("closed child in unclosed parent", func(t *testing.T) {
		result := v035TestDocument(t, v035TestOpen(t, "fragment", "f", 2, 1),
			v035TestOpen(t, "note", "", 3, 2), v035TestClose(t, 4, 3))
		v035TestParserError(t, result, 3, 14, "block")
		v035TestRun(t, result, "current", [][4]int{{4, 1, 5, 2}})
	})
}

func TestV035CoordinatesAndCase(t *testing.T) {
	t.Run("indented boundaries", func(t *testing.T) {
		result := v035TestDocument(t,
			v035TestLine(t, "  @note {", "block-start", 2, 1,
				v035TestElement(t, "tag", "@note", "note", 3, 8),
				v035TestElement(t, "block-open", "{", "{", 9, 10)),
			v035TestLine(t, "", "blank", 3, 2),
			v035TestLine(t, "  } \t", "block-end", 3, 2, v035TestElement(t, "block-close", "}", "}", 3, 4)))
		v035TestRun(t, result, "current", [][4]int{{3, 3, 5, 4}})
	})
	t.Run("uppercase tag", func(t *testing.T) {
		opening := v035TestOpen(t, "note", "", 2, 1)
		opening.Raw, opening.Elements[0].Raw = "@NOTE {", "@NOTE"
		v035TestRun(t, v035TestDocument(t, opening, v035TestLine(t, "", "blank", 3, 2),
			v035TestClose(t, 3, 2)), "current", [][4]int{{3, 1, 5, 2}})
	})
	t.Run("Unicode title is still not content", func(t *testing.T) {
		v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "note", "Я😀", 2, 1),
			v035TestLine(t, "", "blank", 3, 2), v035TestClose(t, 3, 2)), "current", [][4]int{{3, 1, 5, 2}})
	})
	t.Run("content after closing is outside", func(t *testing.T) {
		v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "note", "", 2, 1),
			v035TestClose(t, 3, 2), v035TestText(t, "Outside.", 2, 1)), "current", [][4]int{{3, 1, 4, 2}})
	})
}

func TestV035ExcludedContainers(t *testing.T) {
	for _, tag := range []string{"choice", "multichoice", "matching", "ordering", "variants"} {
		t.Run(tag, func(t *testing.T) {
			// Здесь проверяется только исключение из V035. Собственные
			// минимумы этих контейнеров относятся к другим диагностикам.
			v035TestRun(t, v035TestDocument(t, v035TestOpen(t, tag, "", 2, 1),
				v035TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV035LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
		{"mixed", "\r\n", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v035TestDocument(t, v035TestOpen(t, "note", "", 2, 1),
				v035TestLine(t, "", "blank", 3, 2), v035TestClose(t, 3, 2))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[4].LineEnding = tc.last
			v035TestRun(t, result, "current", [][4]int{{3, 1, 5, 2}})
		})
	}
}

func TestV035HistoryAndOtherChecks(t *testing.T) {
	result := v035TestDocument(t, v035TestOpen(t, "note", "", 2, 1),
		v035TestLine(t, "  \t", "blank", 3, 2), v035TestClose(t, 3, 2))
	want := [][4]int{{3, 1, 5, 2}}
	v035TestRun(t, result, "first", want)
	v035TestRun(t, result, "second", want)
	for _, source := range []string{"first", "second"} {
		for _, tc := range []struct {
			code       diagnostics.Code
			start, end int
		}{
			{"V001", 1, 3}, {"V002", 3, 4},
		} {
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source != source || entry.DiagnosticCode != tc.code || entry.Location == nil {
					continue
				}
				location := model.Location{Start: model.Position{Line: 4, Column: tc.start}, End: model.Position{Line: 4, Column: tc.end}}
				if *entry.Location == location {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s at [%d,%d) in %s: count = %d, want 1", tc.code, tc.start, tc.end, source, count)
			}
		}
	}
	v035TestRun(t, v035TestDocument(t, v035TestOpen(t, "note", "", 2, 1),
		v035TestText(t, "Read carefully.", 3, 2), v035TestClose(t, 3, 2)), "first", nil)
}

func TestV035UnavailableAndUnsupportedVersions(t *testing.T) {
	v035TestRun(t, v035TestUnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v035TestUnavailableDocument(t, version)
			before := v035TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v035TestRegistry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v035TestSnapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

// v035TestUnparsed создаёт фрагмент со ссылкой на задаваемую тестом ошибку p1.
func v035TestUnparsed(t *testing.T, raw string, kind model.LineType, parent, depth int) model.Line {
	t.Helper()
	element := v035TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value, element.ErrorIDs = nil, []string{"p1"}
	return v035TestLine(t, raw, kind, parent, depth, element)
}

// v035TestParserError добавляет исходную ошибку с явно заданным диапазоном.
func v035TestParserError(t *testing.T, result *model.Result, line, end int, scope diagnostics.Scope) {
	t.Helper()
	result.Document.HasErrors, result.Lines[line-1].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{
		ID: "p1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		Message: "Ошибка разбора исходного документа.", DiagnosticScope: scope,
		Location:         &model.Location{Start: model.Position{Line: line, Column: 1}, End: model.Position{Line: line, Column: end}},
		RelatedLocations: []model.Location{},
	}}
}

// v035TestOpen создаёт объявление блока; argument — TITLE либо ID fragment.
func v035TestOpen(t *testing.T, tag, argument string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v035TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if argument != "" {
		kind := model.ElementTypeTitle
		if tag == "fragment" {
			kind = model.ElementTypeIdentifier
		}
		start := utf8.RuneCountInString(raw) + 2
		elements = append(elements, v035TestElement(t, kind, argument, argument, start, start+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	raw += " {"
	column := utf8.RuneCountInString(raw)
	elements = append(elements, v035TestElement(t, "block-open", "{", "{", column, column+1))
	return v035TestLine(t, raw, "block-start", parent, depth, elements...)
}

// v035TestClose создаёт закрывающую строку с явной родительской связью.
func v035TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v035TestLine(t, "}", "block-end", parent, depth, v035TestElement(t, "block-close", "}", "}", 1, 2))
}

// v035TestTag создаёт однострочный тег с явно указанным типом значения.
func v035TestTag(t *testing.T, tag, text string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	start := len(tag) + 3
	return v035TestLine(t, "@"+tag+" "+text, "tag", parent, depth,
		v035TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v035TestElement(t, kind, text, text, start, start+utf8.RuneCountInString(text)))
}

func TestV035Registration(t *testing.T) {
	definitions, err := v035TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V035" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Message != "Блок не содержит непустого содержимого." ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V035 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V035 definitions = %d, want 1", matches)
	}
}

// v035TestUnavailableDocument сохраняет известную версию при недоступном исходнике.
func v035TestUnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v035TestRegistry создаёт только штатный реестр; тест не подменяет регистрации.
func v035TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v035TestDocument добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v035TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v035TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v035TestElement(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v035TestElement(t, "version", "1.2", "1.2", 14, 17)),
		v035TestLine(t, "# Unit 1", "heading", 0, 0,
			v035TestElement(t, "heading-level", "#", "1", 1, 2),
			v035TestElement(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v035TestLine собирает строку с явно заданной структурой, без классификации Raw.
func v035TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v035TestElement сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v035TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v035TestText создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v035TestText(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v035TestLine(t, text, "content", parent, depth,
		v035TestElement(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v035TestRun выполняет штатный каталог и проверяет только V035 текущего
// запуска по явно заданным диапазонам. Флаги ошибок ожидаются на строках
// начала этих диапазонов; исходные ошибки и вся прежняя история сохраняются.
func v035TestRun(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v035TestSnapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v035TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v035TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V035" && entry.Source != processingID {
			t.Fatalf("new V035 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V035" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V035 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[2], Column: expected[3]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V035" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.Message != "Блок не содержит непустого содержимого." || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	v035AssertPreserved(t, before, result)
}

// v035TestSnapshot фиксирует модель для проверки отсутствия побочных изменений.
func v035TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v035AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V035 заданы явно выше.
func v035AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v035TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
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
	if v035TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

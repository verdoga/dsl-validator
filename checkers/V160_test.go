package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v160Expected задаёт основной и единственный связанный диапазоны независимо
// от проверки: строка, начальная и исключающая конечная Unicode-колонки с 1.
type v160Expected [2][3]int

func TestV160Registration(t *testing.T) {
	definitions, err := v160Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V160" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "В блоке выбора использован запрещённый вложенный тег." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V160 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v160Empty(t, version)
			before := v160Snapshot(t, result)
			checks, err := validator.Prepare(result, v160Registry(t))
			if err == nil || checks != nil || v160Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV160ReferenceAndExample(t *testing.T) {
	// Дословные фрагменты дополнены @dsl-version, @document-id и # Lesson
	// по DSL 1.1, 1.30, 1.31 и требованию V037. Фрагмент начинается со строки 4.
	t.Run("reference", func(t *testing.T) {
		v160Run(t, v160Within(t, "choice"), "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v160Run(t, v160Within(t, "choice", v160TagLine(t, "instr", "content", "Choose a country.", 4, 2)),
			"current", []v160Expected{{{5, 1, 7}, {4, 1, 8}}})
	})
}

func TestV160KnownSingleLineTags(t *testing.T) {
	// Имена и концы диапазонов заданы явно. Допустимость размещения этих
	// объявлений и есть предмет V160; другие диагностики проверяются отдельно.
	for _, container := range []struct {
		name string
		end  int
	}{{"choice", 8}, {"multichoice", 13}} {
		for _, tc := range []struct {
			name  string
			kind  model.ElementType
			value string
			end   int
		}{
			{"dsl-version", "version", "1.2", 13}, {"document-id", "identifier", "other", 13},
			{"section", "name", "Practice", 9}, {"order", "number", "1", 7}, {"resource-dir", "resource-path", "media", 14},
			{"header", "title", "Read", 8}, {"task", "identifier", "practice", 6}, {"endtask", "", "", 9},
			{"step", "title", "Write", 6}, {"speaking", "", "", 10}, {"newpage", "", "", 9},
			{"editor", "content", "Comment.", 8}, {"example", "content", "Example.", 9},
			{"wordlist", "content", "red; blue.", 10}, {"instr", "content", "Choose a country.", 7},
			{"note", "content", "Read.", 6}, {"alt", "content", "Description.", 5},
			{"question", "content", "Where?", 10}, {"multifill", "content", "Write.", 11}, {"variant", "name", "A", 9},
		} {
			t.Run(container.name+"/"+tc.name, func(t *testing.T) {
				v160Run(t, v160Within(t, container.name, v160TagLine(t, tc.name, tc.kind, tc.value, 4, 2)),
					"current", []v160Expected{{{5, 1, tc.end}, {4, 1, container.end}}})
			})
		}
		t.Run(container.name+"/media", func(t *testing.T) {
			media := v160Line(t, "@media image photo.jpg", "tag", 4, 2,
				v160Element(t, "tag", "@media", "media", 1, 7), v160Element(t, "media-type", "image", "image", 8, 13),
				v160Element(t, "source", "photo.jpg", "photo.jpg", 14, 23))
			v160Run(t, v160Within(t, container.name, media), "current", []v160Expected{{{5, 1, 7}, {4, 1, container.end}}})
		})
	}
}

func TestV160KnownBlockTags(t *testing.T) {
	for _, container := range []struct {
		name string
		end  int
	}{{"choice", 8}, {"multichoice", 13}} {
		for _, tc := range []struct {
			name string
			end  int
		}{
			{"editor", 8}, {"example", 9}, {"wordlist", 10}, {"table", 7}, {"script", 8}, {"text", 6},
			{"key", 5}, {"instr", 7}, {"note", 6}, {"alt", 5}, {"question", 10}, {"multifill", 11},
			{"choice", 8}, {"multichoice", 13}, {"ordering", 10},
		} {
			t.Run(container.name+"/"+tc.name, func(t *testing.T) {
				v160Run(t, v160Within(t, container.name, v160Open(t, tc.name, 4, 2),
					v160Text(t, "First.", "First.", 5, 3), v160Text(t, "Second.", "Second.", 5, 3), v160Close(t, 5, 3)),
					"current", []v160Expected{{{5, 1, tc.end}, {4, 1, container.end}}})
			})
		}
		t.Run(container.name+"/fragment", func(t *testing.T) {
			v160Run(t, v160Within(t, container.name, v160Fragment(t, 4, 2),
				v160Text(t, "Read.", "Read.", 5, 3), v160Close(t, 5, 3)),
				"current", []v160Expected{{{5, 1, 10}, {4, 1, container.end}}})
		})
		t.Run(container.name+"/matching", func(t *testing.T) {
			v160Run(t, v160Within(t, container.name, v160Open(t, "matching", 4, 2),
				v160Text(t, "Country", "Country", 5, 3), v160Line(t, "---", "separator", 5, 3),
				v160Text(t, "Spain", "Spain", 5, 3), v160Text(t, "France", "France", 5, 3), v160Close(t, 5, 3)),
				"current", []v160Expected{{{5, 1, 10}, {4, 1, container.end}}})
		})
		t.Run(container.name+"/variants", func(t *testing.T) {
			v160Run(t, v160Within(t, container.name, v160Open(t, "variants", 4, 2),
				v160TagLine(t, "variant", "name", "A", 5, 3), v160Text(t, "Read.", "Read.", 6, 4),
				v160TagLine(t, "variant", "name", "B", 5, 3), v160Text(t, "Write.", "Write.", 8, 4), v160Close(t, 5, 3)),
				"current", []v160Expected{{{5, 1, 10}, {4, 1, container.end}}})
		})
	}
}

func TestV160Exceptions(t *testing.T) {
	for _, container := range []string{"choice", "multichoice"} {
		t.Run(container+"/inline answer and hint", func(t *testing.T) {
			v160Run(t, v160Document(t, v160Open(t, container, 3, 1),
				v160TagLine(t, "ANSWER", "content", "Spain", 4, 2), v160TagLine(t, "HiNt", "content", "Think.", 4, 2),
				v160Text(t, "France", "France", 4, 2), v160Close(t, 4, 2)), "current", nil)
		})
		t.Run(container+"/block answer and hint", func(t *testing.T) {
			v160Run(t, v160Document(t, v160Open(t, container, 3, 1), v160Open(t, "answer", 4, 2),
				v160Text(t, "Spain", "Spain", 5, 3), v160Close(t, 5, 3), v160Open(t, "hint", 4, 2),
				v160Text(t, "Think.", "Think.", 8, 3), v160Close(t, 8, 3),
				v160Text(t, "France", "France", 4, 2), v160Close(t, 4, 2)), "current", nil)
		})
		t.Run(container+"/unlinked hint", func(t *testing.T) {
			v160Run(t, v160Within(t, container, v160TagLine(t, "hint", "content", "Think.", 4, 2)), "current", nil)
		})
		for _, tc := range []struct{ raw, value string }{
			{`\@instr Choose.`, "@instr Choose."}, {"Text @instr More.", "Text @instr More."},
			{"Text @editor Comment.", "Text @editor Comment."}, {`\@choice {`, "@choice {"},
			{"Испания 😀", "Испания 😀"},
		} {
			t.Run(container+"/text/"+tc.raw, func(t *testing.T) {
				v160Run(t, v160Within(t, container, v160Text(t, tc.raw, tc.value, 4, 2)), "current", nil)
			})
		}
		for _, name := range []string{"editor", "multifill"} {
			t.Run(container+"/literal inside "+name, func(t *testing.T) {
				v160Run(t, v160Document(t, v160Open(t, name, 3, 1),
					v160Text(t, "@"+container+" {", "@"+container+" {", 4, 2),
					v160Text(t, "@instr Choose.", "@instr Choose.", 4, 2),
					v160Text(t, `\}`, "}", 4, 2), v160Close(t, 4, 2)), "current", nil)
			})
		}
	}
}

func TestV160Includes(t *testing.T) {
	// DSL 3.13.10 запрещает include в блоках выбора. Определение f исключает
	// отсутствующую ссылку; V160 отмечает само размещение без раскрытия вставки.
	for _, container := range []struct {
		name, mixed string
		end         int
	}{{"choice", "ChOiCe", 8}, {"multichoice", "MuLtIcHoIcE", 13}} {
		for _, tc := range []struct {
			name string
			body []model.Line
			want []v160Expected
		}{
			{"inline include", []model.Line{
				v160TagLine(t, "INCLUDE", "identifier", "f", 7, 2),
			}, []v160Expected{{{8, 1, 9}, {7, 1, container.end}}}},
			{"several includes receive separate findings", []model.Line{
				v160TagLine(t, "include", "identifier", "f", 7, 2), v160TagLine(t, "include", "identifier", "F", 7, 2),
				v160TagLine(t, "include", "identifier", "f", 7, 2),
			}, []v160Expected{{{8, 1, 9}, {7, 1, container.end}}, {{9, 1, 9}, {7, 1, container.end}}, {{10, 1, 9}, {7, 1, container.end}}}},
			{"malformed block include is one forbidden declaration", []model.Line{
				v160Line(t, "@include f {", "block-start", 7, 2, v160Element(t, "tag", "@include", "include", 1, 9),
					v160Element(t, "identifier", "f", "f", 10, 11), v160Element(t, "block-open", "{", "{", 12, 13)),
				v160TagLine(t, "instr", "content", "Read.", 8, 3), v160TagLine(t, "include", "identifier", "f", 8, 3),
				v160Close(t, 8, 3),
			}, []v160Expected{{{8, 1, 9}, {7, 1, container.end}}}},
			{"include inside hint does not belong to the outer choice", []model.Line{
				v160Open(t, "hint", 7, 2), v160TagLine(t, "include", "identifier", "f", 8, 3), v160Close(t, 8, 3),
			}, nil},
			{"include inside note does not add a second finding", []model.Line{
				v160Open(t, "note", 7, 2), v160TagLine(t, "include", "identifier", "f", 8, 3), v160Close(t, 8, 3),
			}, []v160Expected{{{8, 1, 6}, {7, 1, container.end}}}},
			{"escaped and textual mentions are not declarations", []model.Line{
				// DSL 1.24: известный начальный тег в choice нельзя выдавать за content.
				// Здесь литеральность обеспечивают экранирование и положение внутри текста.
				v160Text(t, `\@include f`, "@include f", 7, 2), v160Text(t, "Text @include f", "Text @include f", 7, 2),
			}, nil},
			{"literal include inside editor is not a declaration", []model.Line{
				// DSL 2.8.11 сохраняет похожую на тег строку editor как обычный текст.
				// V160 отмечает запрещённый editor, но не его литеральное содержимое.
				v160Open(t, "editor", 7, 2), v160Text(t, "@include f", "@include f", 8, 3), v160Close(t, 8, 3),
			}, []v160Expected{{{8, 1, 8}, {7, 1, container.end}}}},
			{"unparsed include is not recovered", []model.Line{
				v160Line(t, "@include f", "invalid", 7, 2, v160Unparsed(t, "@include f", 1, 11)),
			}, nil},
			{"saved include in an invalid line remains excluded", []model.Line{
				v160Line(t, "@include f }", "invalid", 7, 2, v160Element(t, "tag", "@include", "include", 1, 9),
					v160Element(t, "identifier", "f", "f", 10, 11), v160Unparsed(t, "}", 12, 13)),
			}, nil},
			{"saved include does not require recovery of its tail", []model.Line{
				v160Line(t, "@include f }", "tag", 7, 2, v160Element(t, "tag", "@include", "include", 1, 9),
					v160Element(t, "identifier", "f", "f", 10, 11), v160Unparsed(t, "}", 12, 13)),
			}, []v160Expected{{{8, 1, 9}, {7, 1, container.end}}}},
		} {
			t.Run(container.name+"/"+tc.name, func(t *testing.T) {
				body := []model.Line{v160Fragment(t, 3, 1), v160Text(t, "Read.", "Read.", 4, 2), v160Close(t, 4, 2),
					v160Open(t, container.name, 3, 1)}
				body = append(body, tc.body...)
				body = append(body, v160TagLine(t, "answer", "content", "Spain", 7, 2),
					v160Text(t, "France", "France", 7, 2), v160Close(t, 7, 2))
				v160Run(t, v160Document(t, body...), "current", tc.want)
			})
		}
		t.Run(container.name+"/case and indentation", func(t *testing.T) {
			parent := v160Line(t, " \t@"+container.mixed+" {", "block-start", 3, 1,
				v160Element(t, "tag", "@"+container.mixed, container.name, 3, container.end+2),
				v160Element(t, "block-open", "{", "{", container.end+3, container.end+4))
			child := v160Line(t, "\t @InClUdE F", "tag", 7, 2,
				v160Element(t, "tag", "@InClUdE", "include", 3, 11), v160Element(t, "identifier", "F", "F", 12, 13))
			v160Run(t, v160Document(t, v160Fragment(t, 3, 1), v160Text(t, "Read.", "Read.", 4, 2), v160Close(t, 4, 2),
				parent, child, v160TagLine(t, "answer", "content", "Spain", 7, 2),
				v160Text(t, "France", "France", 7, 2), v160Close(t, 7, 2)),
				"current", []v160Expected{{{8, 3, 11}, {7, 3, container.end + 2}}})
		})
		t.Run(container.name+"/include outside container remains allowed", func(t *testing.T) {
			v160Run(t, v160Document(t, v160Fragment(t, 3, 1), v160Text(t, "Read.", "Read.", 4, 2), v160Close(t, 4, 2),
				v160TagLine(t, "include", "identifier", "f", 3, 1), v160Open(t, container.name, 3, 1),
				v160TagLine(t, "answer", "content", "Spain", 8, 2), v160Text(t, "France", "France", 8, 2), v160Close(t, 8, 2),
				v160TagLine(t, "include", "identifier", "f", 3, 1)), "current", nil)
		})
	}
}

func TestV160UnparsedAndInvalid(t *testing.T) {
	for _, container := range []struct {
		name string
		end  int
	}{{"choice", 8}, {"multichoice", 13}} {
		for _, raw := range []string{"@instr", "@instr Text.", "@InStR{", "@instr\tText.", "@instr\u00a0Text.", "@instrExtra", "@unknown"} {
			t.Run(container.name+"/no recovery/"+raw, func(t *testing.T) {
				v160Run(t, v160Within(t, container.name, v160Line(t, raw, "invalid", 4, 2,
					v160Unparsed(t, raw, 1, utf8.RuneCountInString(raw)+1))), "current", nil)
			})
		}
		for _, kind := range []model.LineType{"invalid", "tag"} {
			t.Run(container.name+"/saved tag and unparsed/"+string(kind), func(t *testing.T) {
				line := v160Line(t, "@instr Read. }", kind, 4, 2,
					v160Element(t, "tag", "@instr", "instr", 1, 7), v160Element(t, "content", "Read.", "Read.", 8, 13),
					v160Unparsed(t, "}", 14, 15))
				var want []v160Expected
				if kind == "tag" {
					want = []v160Expected{{{5, 1, 7}, {4, 1, container.end}}}
				}
				v160Run(t, v160Within(t, container.name, line), "current", want)
			})
		}
		t.Run(container.name+"/saved malformed block", func(t *testing.T) {
			line := v160Line(t, "@note } {", "block-start", 4, 2, v160Element(t, "tag", "@note", "note", 1, 6),
				v160Unparsed(t, "}", 7, 8), v160Element(t, "block-open", "{", "{", 9, 10))
			v160Run(t, v160Within(t, container.name, line, v160TagLine(t, "instr", "content", "Read.", 5, 3), v160Close(t, 5, 3)),
				"current", []v160Expected{{{5, 1, 6}, {4, 1, container.end}}})
		})
		t.Run(container.name+"/unknown nearest block", func(t *testing.T) {
			line := v160Line(t, "@unknown {", "block-start", 4, 2, v160Unparsed(t, "@unknown", 1, 9),
				v160Element(t, "block-open", "{", "{", 10, 11))
			v160Run(t, v160Within(t, container.name, line, v160TagLine(t, "instr", "content", "Read.", 5, 3), v160Close(t, 5, 3)), "current", nil)
		})
		t.Run(container.name+"/known unparsed block name", func(t *testing.T) {
			line := v160Line(t, "@note {", "block-start", 4, 2, v160Unparsed(t, "@note", 1, 6),
				v160Element(t, "block-open", "{", "{", 7, 8))
			v160Run(t, v160Within(t, container.name, line, v160TagLine(t, "instr", "content", "Read.", 5, 3), v160Close(t, 5, 3)), "current", nil)
		})
		t.Run(container.name+"/saved tag without separating space", func(t *testing.T) {
			line := v160Line(t, "@InStR{", "block-start", 4, 2, v160Element(t, "tag", "@InStR", "instr", 1, 7),
				v160Element(t, "block-open", "{", "{", 7, 8))
			v160Run(t, v160Within(t, container.name, line, v160Text(t, "Read.", "Read.", 5, 3), v160Close(t, 5, 3)),
				"current", []v160Expected{{{5, 1, 7}, {4, 1, container.end}}})
		})
		t.Run(container.name+"/invalid nearest boundary", func(t *testing.T) {
			// Незакрытый ошибочный блок в конце файла: контракт не допускает
			// block-end с родителем invalid, поэтому фиктивное закрытие не добавляем.
			line := v160Line(t, "@choice } {", "invalid", 4, 2, v160Element(t, "tag", "@choice", "choice", 1, 8),
				v160Unparsed(t, "}", 9, 10), v160Element(t, "block-open", "{", "{", 11, 12))
			v160Run(t, v160Document(t, v160Open(t, container.name, 3, 1), line,
				v160TagLine(t, "instr", "content", "Read.", 5, 3)), "current", nil)
		})
	}
}

func TestV160NearestContainer(t *testing.T) {
	for _, container := range []struct {
		name string
		end  int
	}{{"choice", 8}, {"multichoice", 13}} {
		t.Run(container.name+"/several siblings", func(t *testing.T) {
			v160Run(t, v160Within(t, container.name, v160TagLine(t, "instr", "content", "Read.", 4, 2),
				v160TagLine(t, "note", "content", "Think.", 4, 2)), "current",
				[]v160Expected{{{5, 1, 7}, {4, 1, container.end}}, {{6, 1, 6}, {4, 1, container.end}}})
		})
		for _, inner := range []struct {
			name string
			want []v160Expected
		}{
			{"note", []v160Expected{{{5, 1, 6}, {4, 1, container.end}}}},
			{"answer", nil}, {"hint", nil},
			{"choice", []v160Expected{{{5, 1, 8}, {4, 1, container.end}}, {{6, 1, 7}, {5, 1, 8}}}},
			{"multichoice", []v160Expected{{{5, 1, 13}, {4, 1, container.end}}, {{6, 1, 7}, {5, 1, 13}}}},
		} {
			t.Run(container.name+"/nearest "+inner.name, func(t *testing.T) {
				// Состав answer/hint с запрещённым instr проверяется отдельно;
				// V160 не должна относить его к внешнему блоку выбора.
				v160Run(t, v160Within(t, container.name, v160Open(t, inner.name, 4, 2),
					v160TagLine(t, "instr", "content", "Choose.", 5, 3), v160Text(t, "Spain", "Spain", 5, 3),
					v160Text(t, "France", "France", 5, 3), v160Close(t, 5, 3)), "current", inner.want)
			})
		}
		t.Run(container.name+"/logical non-brace parent", func(t *testing.T) {
			v160Run(t, v160Within(t, container.name, v160TagLine(t, "task", "identifier", "practice", 4, 2),
				v160TagLine(t, "instr", "content", "Read.", 5, 3), v160TagLine(t, "endtask", "", "", 4, 2)), "current",
				[]v160Expected{{{5, 1, 6}, {4, 1, container.end}}, {{6, 1, 7}, {4, 1, container.end}}, {{7, 1, 9}, {4, 1, container.end}}})
		})
		t.Run(container.name+"/after closing block", func(t *testing.T) {
			v160Run(t, v160Document(t, v160Open(t, container.name, 3, 1),
				v160TagLine(t, "answer", "content", "Spain", 4, 2), v160Text(t, "France", "France", 4, 2),
				v160Close(t, 4, 2), v160TagLine(t, "instr", "content", "Read.", 3, 1)), "current", nil)
		})
		t.Run(container.name+"/task stage", func(t *testing.T) {
			v160Run(t, v160Document(t, v160TagLine(t, "task", "identifier", "practice", 3, 1),
				v160TagLine(t, "step", "title", "Choose", 4, 2), v160Open(t, container.name, 5, 3),
				v160TagLine(t, "instr", "content", "Read.", 6, 4), v160TagLine(t, "answer", "content", "Spain", 6, 4),
				v160Text(t, "France", "France", 6, 4), v160Close(t, 6, 4), v160TagLine(t, "endtask", "", "", 4, 2)),
				"current", []v160Expected{{{7, 1, 7}, {6, 1, container.end}}})
		})
		t.Run(container.name+"/branch", func(t *testing.T) {
			v160Run(t, v160Document(t, v160Open(t, "variants", 3, 1), v160TagLine(t, "variant", "name", "A", 4, 2),
				v160Open(t, container.name, 5, 3), v160TagLine(t, "instr", "content", "Read.", 6, 4),
				v160TagLine(t, "answer", "content", "Spain", 6, 4), v160Text(t, "France", "France", 6, 4), v160Close(t, 6, 4),
				v160TagLine(t, "variant", "name", "B", 4, 2), v160Text(t, "Read.", "Read.", 11, 3), v160Close(t, 4, 2)),
				"current", []v160Expected{{{7, 1, 7}, {6, 1, container.end}}})
		})
	}
	t.Run("unrelated container", func(t *testing.T) {
		v160Run(t, v160Document(t, v160Open(t, "note", 3, 1), v160TagLine(t, "instr", "content", "Read.", 4, 2),
			v160Close(t, 4, 2)), "current", nil)
	})
	t.Run("nested editor is one forbidden declaration", func(t *testing.T) {
		v160Run(t, v160Within(t, "choice", v160Open(t, "editor", 4, 2),
			v160Text(t, "@instr Literal.", "@instr Literal.", 5, 3), v160Text(t, "@choice {", "@choice {", 5, 3),
			v160Text(t, `\}`, "}", 5, 3), v160Close(t, 5, 3)), "current", []v160Expected{{{5, 1, 8}, {4, 1, 8}}})
	})
}

func TestV160PositionsAndDocumentEdges(t *testing.T) {
	t.Run("case indentation and raw Unicode positions", func(t *testing.T) {
		parent := v160Line(t, " \t@ChOiCe Выбери. {", "block-start", 3, 1,
			v160Element(t, "tag", "@ChOiCe", "choice", 3, 10), v160Element(t, "content", "Выбери.", "Выбери.", 11, 18),
			v160Element(t, "block-open", "{", "{", 19, 20))
		child := v160Line(t, " \t@InStR Выбери 😀.", "tag", 4, 2,
			v160Element(t, "tag", "@InStR", "instr", 3, 9), v160Element(t, "content", "Выбери 😀.", "Выбери 😀.", 10, 19))
		v160Run(t, v160Document(t, parent, child, v160TagLine(t, "answer", "content", "Spain", 4, 2),
			v160Text(t, "France", "France", 4, 2), v160Close(t, 4, 2)), "current", []v160Expected{{{5, 3, 9}, {4, 3, 10}}})
	})
	t.Run("blank and empty content", func(t *testing.T) {
		v160Run(t, v160Within(t, "choice", v160Line(t, " \t", "blank", 4, 2),
			v160Line(t, "", "blank", 4, 2)), "current", nil)
	})
	t.Run("first and last child", func(t *testing.T) {
		v160Run(t, v160Document(t, v160Open(t, "choice", 3, 1), v160TagLine(t, "instr", "", "", 4, 2),
			v160TagLine(t, "answer", "content", "Spain", 4, 2), v160Text(t, "France", "France", 4, 2),
			v160TagLine(t, "note", "", "", 4, 2), v160Close(t, 4, 2)), "current",
			[]v160Expected{{{5, 1, 7}, {4, 1, 8}}, {{8, 1, 6}, {4, 1, 8}}})
	})
	t.Run("last physical line in unclosed block", func(t *testing.T) {
		v160Run(t, v160Document(t, v160Open(t, "multichoice", 3, 1), v160Text(t, "Spain", "Spain", 4, 2),
			v160Text(t, "France", "France", 4, 2), v160TagLine(t, "instr", "content", "Read.", 4, 2)),
			"current", []v160Expected{{{7, 1, 7}, {4, 1, 13}}})
	})
	for _, known := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty known count %t", known), func(t *testing.T) {
			result := v160Empty(t, "1.2")
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v160Run(t, result, "current", nil)
		})
	}
	t.Run("prefix only", func(t *testing.T) { v160Run(t, v160Document(t), "current", nil) })
	t.Run("root declaration", func(t *testing.T) {
		v160Run(t, v160Document(t, v160TagLine(t, "instr", "content", "Read.", 3, 1)), "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(fmt.Sprintf("EOL %q final %t", ending, final), func(t *testing.T) {
				result := v160Within(t, "choice", v160TagLine(t, "instr", "content", "Choose a country.", 4, 2))
				for i := range result.Lines {
					if i < len(result.Lines)-1 || final {
						result.Lines[i].LineEnding = ending
					}
				}
				v160Run(t, result, "current", []v160Expected{{{5, 1, 7}, {4, 1, 8}}})
			})
		}
	}
	t.Run("history survives repeated run", func(t *testing.T) {
		result := v160Within(t, "choice", v160TagLine(t, "instr", "content", "Choose a country.", 4, 2))
		v160Run(t, result, "first", []v160Expected{{{5, 1, 7}, {4, 1, 8}}})
		v160Run(t, result, "second", []v160Expected{{{5, 1, 7}, {4, 1, 8}}})
	})
}

// v160Registry использует полный штатный каталог, без отдельного тестового реестра.
func v160Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v160Empty создаёт согласованную модель без исходных строк.
func v160Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v160Document дополняет фрагмент техническим префиксом и заголовком.
// Дерево и элементы заданы тестами; unparsed получает согласованную историю.
func v160Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v160Empty(t, "1.2")
	title, id := "Lesson", "v160-test"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v160TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v160TagLine(t, "document-id", "identifier", "v160-test", 0, 0),
		v160Line(t, "# Lesson", "heading", 0, 0, v160Element(t, "heading-level", "#", "1", 1, 2),
			v160Element(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			errorID := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{errorID}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: errorID, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v160Within помещает заданные строки перед двумя вариантами эталона.
// Контейнер находится на строке 4, первая дополнительная строка — на строке 5.
func v160Within(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v160Open(t, name, 3, 1)}, body...)
	lines = append(lines, v160TagLine(t, "answer", "content", "Spain", 4, 2), v160Text(t, "France", "France", 4, 2), v160Close(t, 4, 2))
	return v160Document(t, lines...)
}

// v160Line сохраняет явно заданные тип, дерево и элементы физической строки.
func v160Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v160Element задаёт независимые Raw и Value с исходным диапазоном [start,end).
func v160Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v160Unparsed задаёт фрагмент без значения; историю добавляет v160Document.
func v160Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v160TagLine создаёт сохранённое объявление с явно типизированным аргументом.
func v160TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v160Element(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v160Element(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v160Line(t, raw, "tag", parent, depth, elements...)
}

// v160Text сохраняет обычный текст и отдельно заданное семантическое значение.
func v160Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v160Line(t, raw, "content", parent, depth, v160Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v160Open создаёт объявление фигурного блока без параметров.
func v160Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v160Line(t, "@"+name+" {", "block-start", parent, depth,
		v160Element(t, "tag", "@"+name, strings.ToLower(name), 1, len(name)+2),
		v160Element(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v160Close создаёт отдельную закрывающую строку с заданным родителем.
func v160Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v160Line(t, "}", "block-end", parent, depth, v160Element(t, "block-close", "}", "}", 1, 2))
}

// v160Fragment создаёт блочное определение с обязательным идентификатором.
func v160Fragment(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v160Line(t, "@fragment f {", "block-start", parent, depth, v160Element(t, "tag", "@fragment", "fragment", 1, 10),
		v160Element(t, "identifier", "f", "f", 11, 12), v160Element(t, "block-open", "{", "{", 13, 14))
}

// v160Run проверяет только новые V160 после полного штатного конвейера.
// Исходные строки, элементы и история обязаны сохраняться без изменений.
func v160Run(t *testing.T, result *model.Result, source string, want []v160Expected) {
	t.Helper()
	before := v160Snapshot(t, result)
	checks, err := validator.Prepare(result, v160Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v160Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	ids := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("new diagnostic source = %q", entry.Source)
		}
		if entry.DiagnosticCode == "V160" {
			got = append(got, entry)
			ids[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V160 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		main := model.Location{Start: model.Position{Line: expected[0][0], Column: expected[0][1]}, End: model.Position{Line: expected[0][0], Column: expected[0][2]}}
		related := model.Location{Start: model.Position{Line: expected[1][0], Column: expected[1][1]}, End: model.Position{Line: expected[1][0], Column: expected[1][2]}}
		if entry.Location == nil || *entry.Location != main || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v / %+v", i, entry.Location, entry.RelatedLocations, main, related)
		}
		if entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "В блоке выбора использован запрещённый вложенный тег." {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0][0]-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v160Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Lines) != len(original.Lines) || len(actual.Processing) != oldProcessing+1 {
		t.Fatal("source or history length changed")
	}
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		if len(actual.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			for _, id := range element.ErrorIDs {
				if ids[id] {
					t.Error("ScopeLine V160 linked to element")
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v160Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v160Snapshot сохраняет модель для сравнения исходных данных и истории.
func v160Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

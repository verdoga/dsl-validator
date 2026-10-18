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

// v048Expected содержит явно заданные диапазоны [start,end) одной строки.
type v048Expected struct {
	line, start, end, relatedStart, relatedEnd int
}

// v048Parity задаёт независимое ожидание V005 в ситуациях с нарушенной парностью.
type v048Parity struct {
	line, start, end int
	detail           string
}

func TestV048Registration(t *testing.T) {
	definitions, err := v048Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V048" {
			continue
		}
		count++
		if definition.Message != "После открывающей фигурной скобки блока есть содержимое." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V048 definitions = %d, want 1", count)
	}
}

func TestV048ReferenceAndExample(t *testing.T) {
	// Исходные фрагменты сохранены целиком; добавлены только обязательные
	// @dsl-version и заголовок (DSL 1.30). Ошибочный остаток сохранён как unparsed.
	t.Run("reference", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "note", "", "", "", 2, 1),
			v048Text(t, "Read carefully.", 3, 2), v048Close(t, 3, 2)), "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "note", "", "", " Read carefully.", 2, 1), v048Close(t, 3, 2)),
			"current", []v048Expected{{3, 7, 8, 9, 24}})
	})
	t.Run("raw content independent of brace elements", func(t *testing.T) {
		// Как в существующих входах V005, строка может сохраняться как tag/content.
		// V048 обязана проверять Raw, даже без элемента block-open и ошибки парсера.
		v048Run(t, v048Document(t, v048Tag(t, "note", "content", "{ x }", 2, 1)),
			"current", []v048Expected{{3, 7, 8, 9, 12}})
	})
}

func TestV048RawRecognition(t *testing.T) {
	// Ошибочные объявления восстановлены целиком в invalid/unparsed.
	// Следующие отдельные закрытия также заданы явно: модель не придумывает
	// block-start там, где исходное объявление осталось неразобранным.
	for _, tc := range []struct {
		name, raw string
		closings  int
		want      []v048Expected
	}{
		{"ordinary content", "@note { x", 1, []v048Expected{{3, 7, 8, 9, 10}}},
		{"no separator after brace", "@note {x", 1, []v048Expected{{3, 7, 8, 8, 9}}},
		{"trailing spaces and tab", "@note { x \t", 1, []v048Expected{{3, 7, 8, 9, 10}}},
		{"initial tab in remainder", "@note {\t x", 1, []v048Expected{{3, 7, 8, 10, 11}}},
		{"NBSP alone is content", "@note {\u00a0", 1, []v048Expected{{3, 7, 8, 8, 9}}},
		{"NBSP is not trimmed at start", "@note {\u00a0x", 1, []v048Expected{{3, 7, 8, 8, 10}}},
		{"NBSP is not trimmed at end", "@note { x\u00a0 \t", 1, []v048Expected{{3, 7, 8, 9, 11}}},
		{"case indentation and joined brace", " \t@NoTe{ x", 1, []v048Expected{{3, 8, 9, 10, 11}}},
		{"longer tag is unknown", "@notebook { x", 1, nil},
		{"zero width space is not a name boundary", "@note\u200b{ x", 1, nil},
		{"NBSP is not indentation", "\u00a0@note { x", 1, nil},
		{"escaped tag", `\@note { x`, 1, nil},
		{"two backslashes", `@note \\{ x`, 1, []v048Expected{{3, 9, 10, 11, 12}}},
		{"four backslashes", `@note \\\\{ x`, 1, []v048Expected{{3, 11, 12, 13, 14}}},
		{"escaped brace before opening", `@note \{ a { x`, 1, []v048Expected{{3, 12, 13, 14, 15}}},
		{"escaped close before opening", `@note \} { x`, 1, []v048Expected{{3, 10, 11, 12, 13}}},
		{"backslash before ordinary character", `@note \q{ x`, 1, []v048Expected{{3, 9, 10, 11, 12}}},
		{"quotes do not hide opening", `@note "{ x`, 1, []v048Expected{{3, 8, 9, 10, 11}}},
		{"several words and braces", "@note { x { y", 2, []v048Expected{{3, 7, 8, 9, 14}}},
		{"closing on declaration line is content", "@note { x }", 0, []v048Expected{{3, 7, 8, 9, 12}}},
		{"standalone opening is outside V048", "{ x", 1, nil},
		{"missing opening", "@table", 0, nil},
		{"bare at sign", "@", 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v048Invalid(t, tc.raw, 2, 1)}
			for i := 0; i < tc.closings; i++ {
				body = append(body, v048Invalid(t, "}", 2, 1))
			}
			v048Run(t, v048Document(t, body...), "current", tc.want)
		})
	}
	for _, space := range []rune{' ', '\t', '\v', '\f', '\u0085', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(fmt.Sprintf("Unicode name boundary U+%04X", space), func(t *testing.T) {
			// LF/CR не входят в Raw по контракту модели; остальные White_Space
			// проверяются как границы имени по принятому варианту A.
			v048Run(t, v048Document(t, v048Invalid(t, "@note"+string(space)+"{ x", 2, 1), v048Invalid(t, "}", 2, 1)),
				"current", []v048Expected{{3, 7, 8, 9, 10}})
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\{ x`, "{ x"}, {`\\\{ x`, `\{ x`}, {`\} x`, "} x"},
	} {
		t.Run("literal "+tc.raw, func(t *testing.T) {
			line := v048Line(t, "@note "+tc.raw, "tag", 2, 1, v048Element(t, "tag", "@note", "note", 1, 6),
				v048Element(t, "content", tc.raw, tc.value, 7, 7+utf8.RuneCountInString(tc.raw)))
			v048Run(t, v048Document(t, line), "current", nil)
		})
	}
	for _, tail := range []string{"", " ", "\t", " \t "} {
		t.Run("allowed trailing whitespace "+tail, func(t *testing.T) {
			opening := v048Open(t, "note", "", "", "", 2, 1)
			opening.Raw += tail
			v048Run(t, v048Document(t, opening, v048Text(t, "Read.", 3, 2), v048Close(t, 3, 2)), "current", nil)
		})
	}
	t.Run("Unicode title", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "text", "Я🙂", "title", " x", 2, 1), v048Text(t, "Read.", 3, 2), v048Close(t, 3, 2)),
			"current", []v048Expected{{3, 10, 11, 12, 13}})
	})
}

func TestV048AnswersBeforeBlockOpening(t *testing.T) {
	// Специальный ответ в инструкции запрещён DSL 4.3.10 и сохранён в
	// unparsed. Однако условия V048 исключают его скобки и на ошибочной строке.
	for _, tc := range []struct {
		name, raw string
		want      []v048Expected
	}{
		{"answer then proper opening", "@multifill _____{yes} {", nil},
		{"answer then trailing content", "@multifill _____{yes} { x", []v048Expected{{3, 23, 24, 25, 26}}},
		{"escaped closing in answer", `@multifill _____{a\}b} { x`, []v048Expected{{3, 24, 25, 26, 27}}},
		{"escaped opening in answer", `@multifill _____{a\{b} { x`, []v048Expected{{3, 24, 25, 26, 27}}},
		{"paired backslashes in answer", `@multifill _____{a\\} { x`, []v048Expected{{3, 23, 24, 25, 26}}},
		{"two answers", "@multifill _____{a} _____{b} { x", []v048Expected{{3, 30, 31, 32, 33}}},
		{"four underscores are not an answer", "@note ____{x", []v048Expected{{3, 11, 12, 12, 13}}},
		{"six underscores are not an answer", "@note ______{x", []v048Expected{{3, 13, 14, 14, 15}}},
		{"escaped underscore run is not an answer", `@note \_____{x`, []v048Expected{{3, 13, 14, 14, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v048Run(t, v048Document(t, v048Invalid(t, tc.raw, 2, 1), v048Invalid(t, "}", 2, 1)), "current", tc.want)
		})
	}
	t.Run("answer alone does not enter literal context", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Invalid(t, "@multifill _____{yes}", 2, 1),
			v048Open(t, "note", "", "", " x", 2, 1), v048Text(t, "Read.", 4, 2), v048Close(t, 4, 2)),
			"current", []v048Expected{{4, 7, 8, 9, 10}})
	})
	t.Run("unclosed answer does not open a block", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Invalid(t, "@multifill _____{yes", 2, 1)), "current", nil,
			v048Parity{3, 17, 18, "Открывающая скобка не имеет закрывающей"})
	})
}

func TestV048AllBlockTags(t *testing.T) {
	for _, tc := range []struct {
		tag  string
		want v048Expected
	}{
		{"editor", v048Expected{3, 9, 10, 11, 12}},
		{"instr", v048Expected{3, 8, 9, 10, 11}},
		{"hint", v048Expected{5, 7, 8, 9, 10}},
		{"answer", v048Expected{4, 9, 10, 11, 12}},
		{"example", v048Expected{3, 10, 11, 12, 13}},
		{"wordlist", v048Expected{3, 11, 12, 13, 14}},
		{"note", v048Expected{3, 7, 8, 9, 10}},
		{"alt", v048Expected{3, 6, 7, 8, 9}},
		{"table", v048Expected{3, 8, 9, 10, 11}},
		{"script", v048Expected{3, 9, 10, 11, 12}},
		{"text", v048Expected{3, 7, 8, 9, 10}},
		{"key", v048Expected{4, 6, 7, 8, 9}},
		{"choice", v048Expected{3, 9, 10, 11, 12}},
		{"multichoice", v048Expected{3, 14, 15, 16, 17}},
		{"matching", v048Expected{3, 11, 12, 13, 14}},
		{"ordering", v048Expected{3, 11, 12, 13, 14}},
		{"fragment", v048Expected{3, 13, 14, 15, 16}},
		{"variants", v048Expected{3, 11, 12, 13, 14}},
		{"multifill", v048Expected{3, 12, 13, 14, 15}},
	} {
		for _, broken := range []bool{false, true} {
			t.Run(tc.tag+map[bool]string{false: "/correct", true: "/trailing content"}[broken], func(t *testing.T) {
				var body []model.Line
				parent, depth, argument := 2, 1, ""
				var field model.ElementType
				switch tc.tag {
				case "key":
					body = append(body, v048Tag(t, "task", "identifier", "task-a", 2, 1))
					parent, depth = 3, 2
				case "answer":
					body = append(body, v048Tag(t, "question", "content", "Choose.", 2, 1))
				case "hint":
					body = append(body, v048Open(t, "instr", "", "", "", 2, 1), v048Text(t, "Read.", 3, 2))
					parent, depth = 3, 2
				case "fragment":
					argument, field = "f", "identifier"
				}
				tail := ""
				var want []v048Expected
				if broken {
					tail, want = " x", []v048Expected{tc.want}
				}
				body = append(body, v048Open(t, tc.tag, argument, field, tail, parent, depth))
				opening := tc.want.line
				switch tc.tag {
				case "choice", "multichoice", "ordering":
					body = append(body, v048Text(t, "First", opening, depth+1), v048Text(t, "Second", opening, depth+1))
				case "matching":
					body = append(body, v048Text(t, "A", 3, 2), v048Text(t, "B", 3, 2), v048Line(t, "---", "separator", 3, 2),
						v048Text(t, "X", 3, 2), v048Text(t, "Y", 3, 2))
				case "variants":
					body = append(body, v048Tag(t, "variant", "name", "A", 3, 2), v048Text(t, "Read.", 4, 3),
						v048Tag(t, "variant", "name", "B", 3, 2), v048Text(t, "Write.", 6, 3))
				case "table":
					body = append(body, v048Text(t, "A | B", opening, depth+1))
				case "script":
					body = append(body, v048Text(t, "Teacher: Hello!", opening, depth+1), v048Text(t, "Student: Hello!", opening, depth+1))
				default:
					body = append(body, v048Text(t, "Read.", opening, depth+1))
				}
				body = append(body, v048Close(t, opening, depth+1))
				if tc.tag == "hint" {
					body = append(body, v048Close(t, 3, 2))
				}
				if tc.tag == "key" {
					body = append(body, v048Tag(t, "question", "content", "Answer.", 3, 2),
						v048Line(t, "@endtask", "tag", 3, 2, v048Element(t, "tag", "@endtask", "endtask", 1, 9)))
				}
				v048Run(t, v048Document(t, body...), "current", want)
			})
		}
	}
}

func TestV048Contexts(t *testing.T) {
	t.Run("nested declarations independently", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "instr", "", "", " x", 2, 1), v048Text(t, "Read.", 3, 2),
			v048Open(t, "hint", "", "", " y", 3, 2), v048Text(t, "Helpful.", 5, 3), v048Close(t, 5, 3), v048Close(t, 3, 2)),
			"current", []v048Expected{{3, 8, 9, 10, 11}, {5, 7, 8, 9, 10}})
	})
	for _, tc := range []struct {
		tag     string
		opening v048Expected
	}{
		{"editor", v048Expected{3, 9, 10, 11, 12}}, {"multifill", v048Expected{3, 12, 13, 14, 15}},
	} {
		for _, broken := range []bool{false, true} {
			t.Run(tc.tag+fmt.Sprintf("/opening error %t", broken), func(t *testing.T) {
				tail := ""
				var want []v048Expected
				if broken {
					tail, want = " x", []v048Expected{tc.opening}
				}
				want = append(want, v048Expected{6, 7, 8, 9, 10})
				v048Run(t, v048Document(t, v048Open(t, tc.tag, "", "", tail, 2, 1),
					v048Text(t, "@note { y }", 3, 2), v048Close(t, 3, 2), v048Open(t, "note", "", "", " x", 2, 1),
					v048Text(t, "Read.", 6, 2), v048Close(t, 6, 2)), "current", want)
			})
		}
		t.Run(tc.tag+"/literal only", func(t *testing.T) {
			v048Run(t, v048Document(t, v048Open(t, tc.tag, "", "", "", 2, 1),
				v048Text(t, "@note { x }", 3, 2), v048Close(t, 3, 2)), "current", nil)
		})
	}
	t.Run("placeholder answer is not a block", func(t *testing.T) {
		answer := v048Line(t, "Answer: _____{word}", "content", 3, 2, v048Element(t, "content", "Answer: ", "Answer: ", 1, 9),
			v048Element(t, "placeholder", "_____{word}", "word", 9, 20))
		v048Run(t, v048Document(t, v048Open(t, "multifill", "", "", "", 2, 1), answer, v048Close(t, 3, 2)), "current", nil)
	})
	t.Run("resource path braces are literal", func(t *testing.T) {
		base := v048Document(t)
		result := v048Model(t, base.Lines[0], v048Tag(t, "resource-dir", "resource-path", "assets/{x}", 0, 0), base.Lines[1])
		result.Document.Metadata.ResourceDirs = []string{"assets/{x}"}
		v048Run(t, result, "current", nil)
	})
	t.Run("close with trailing text does not leave literal context", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "editor", "", "", "", 2, 1), v048Invalid(t, "} tail", 3, 2),
			v048Text(t, "@note { x", 3, 2), v048Close(t, 3, 2), v048Open(t, "note", "", "", " x", 2, 1),
			v048Text(t, "Read.", 7, 2), v048Close(t, 7, 2)), "current", []v048Expected{{7, 7, 8, 9, 10}})
	})
	t.Run("escaped close does not leave literal context", func(t *testing.T) {
		literalClose := v048Line(t, `\}`, "content", 3, 2, v048Element(t, "content", `\}`, "}", 1, 3))
		v048Run(t, v048Document(t, v048Open(t, "editor", "", "", "", 2, 1), literalClose,
			v048Text(t, "@note { x }", 3, 2), v048Close(t, 3, 2)), "current", nil)
	})
	t.Run("indented close leaves literal context", func(t *testing.T) {
		close := v048Line(t, " \t}\t ", "block-end", 3, 2, v048Element(t, "block-close", "}", "}", 3, 4))
		v048Run(t, v048Document(t, v048Open(t, "editor", "", "", "", 2, 1), v048Text(t, "Comment.", 3, 2), close,
			v048Open(t, "note", "", "", " x", 2, 1), v048Text(t, "Read.", 6, 2), v048Close(t, 6, 2)),
			"current", []v048Expected{{6, 7, 8, 9, 10}})
	})
	t.Run("raw context survives completely unparsed opening", func(t *testing.T) {
		// Парсер не выделил block-start; V048 всё равно сохраняет editor до }.
		v048Run(t, v048Document(t, v048Invalid(t, "@editor{x", 2, 1), v048Invalid(t, "@note { y }", 2, 1), v048Invalid(t, "}", 2, 1)),
			"current", []v048Expected{{3, 8, 9, 9, 10}})
	})
}

func TestV048BoundariesAndOtherDiagnostics(t *testing.T) {
	v048Run(t, v048Model(t), "current", nil)
	v048Run(t, v048Document(t), "current", nil)
	v048Run(t, v048Document(t, v048Line(t, "", "blank", 2, 1), v048Line(t, " \t", "blank", 2, 1)), "current", nil)
	t.Run("first physical line", func(t *testing.T) {
		base := v048Document(t)
		v048Run(t, v048Model(t, v048Invalid(t, "@note{x", 0, 0), base.Lines[0], base.Lines[1], v048Invalid(t, "}", 3, 1)),
			"current", []v048Expected{{1, 6, 7, 7, 8}})
	})
	t.Run("unclosed block without remainder", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "note", "", "", "", 2, 1)), "current", nil,
			v048Parity{3, 7, 8, "Открывающая скобка не имеет закрывающей"})
	})
	t.Run("last line with remainder and no closing", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Open(t, "note", "", "", " x", 2, 1)), "current", []v048Expected{{3, 7, 8, 9, 10}},
			v048Parity{3, 7, 8, "Открывающая скобка не имеет закрывающей"})
	})
	t.Run("orphan close", func(t *testing.T) {
		v048Run(t, v048Document(t, v048Invalid(t, "}", 2, 1)), "current", nil,
			v048Parity{3, 1, 2, "Закрывающая скобка не имеет открывающей"})
	})
	for _, broken := range []bool{false, true} {
		for _, eol := range []model.LineEnding{"\n", "\r\n"} {
			for _, last := range []model.LineEnding{eol, ""} {
				t.Run(fmt.Sprintf("endings/%t/%q/%q", broken, eol, last), func(t *testing.T) {
					tail := ""
					var want []v048Expected
					if broken {
						tail, want = " x", []v048Expected{{3, 7, 8, 9, 10}}
					}
					result := v048Document(t, v048Open(t, "note", "", "", tail, 2, 1), v048Text(t, "Read.", 3, 2), v048Close(t, 3, 2))
					for i := range result.Lines {
						result.Lines[i].LineEnding = eol
					}
					result.Lines[4].LineEnding = last
					v048Run(t, result, "current", want)
				})
			}
		}
	}
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v048Document(t, v048Open(t, "note", "", "", " x", 2, 1), v048Close(t, 3, 2))
			if version == "unknown" {
				result.Document.DSLVersion = nil
			} else {
				result.Document.DSLVersion = &version
				result.Lines[0].Raw = "@dsl-version " + version
				result.Lines[0].Elements[1].Raw, result.Lines[0].Elements[1].Value = version, &version
			}
			before := v048Snapshot(t, result)
			checks, err := validator.Prepare(result, v048Registry(t))
			if err == nil || checks != nil || v048Snapshot(t, result) != before {
				t.Fatal("unsupported version accepted or model changed")
			}
		})
	}
	t.Run("repeated and independent runs", func(t *testing.T) {
		result := v048Document(t, v048Open(t, "editor", "", "", " x", 2, 1), v048Text(t, "Comment.", 3, 2))
		v048Run(t, result, "first", []v048Expected{{3, 9, 10, 11, 12}}, v048Parity{3, 9, 10, "Открывающая скобка не имеет закрывающей"})
		v048Run(t, result, "second", []v048Expected{{3, 9, 10, 11, 12}}, v048Parity{3, 9, 10, "Открывающая скобка не имеет закрывающей"})
		v048Run(t, v048Document(t, v048Open(t, "note", "", "", " x", 2, 1), v048Close(t, 3, 2)), "current", []v048Expected{{3, 7, 8, 9, 10}})
		v048Run(t, v048Document(t), "current", nil)
	})
}

// v048Registry использует штатный каталог без отдельной регистрации для тестов.
func v048Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v048Model назначает физические номера и исходные ошибки уже заданным
// unparsed-элементам. Синтаксис Raw не разбирается, контексты не вычисляются.
func v048Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, count := "1.2", "UTF-8", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-%d-%d", i+1, j)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
				SeverityLevel: "error", DiagnosticScope: "element", Message: "Не удалось разобрать фрагмент.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}}})
		}
	}
	return result
}

// v048Document добавляет обязательные версию и заголовок к явному телу.
func v048Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v048Tag(t, "dsl-version", "version", "1.2", 0, 0),
		v048Line(t, "# Lesson", "heading", 0, 0, v048Element(t, "heading-level", "#", "1", 1, 2), v048Element(t, "title", "Lesson", "Lesson", 3, 9)),
	}
	result := v048Model(t, append(lines, body...)...)
	title := "Lesson"
	result.Document.Metadata.Title = &title
	return result
}

// v048Line сохраняет явные исходные данные и отношения родителей.
func v048Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v048Element задаёт исходный и семантический текст и диапазон [start,end).
func v048Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v048Tag задаёт однострочное объявление с одним явно типизированным значением.
func v048Tag(t *testing.T, tag string, field model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 2
	return v048Line(t, "@"+tag+" "+value, "tag", parent, depth, v048Element(t, "tag", "@"+tag, tag, 1, end),
		v048Element(t, field, value, value, end+1, end+1+utf8.RuneCountInString(value)))
}

// v048Open задаёт распознанный блок; непустой tail обозначает ошибочный
// неразобранный остаток. Длины вычисляются только для входа, не для ожиданий.
func v048Open(t *testing.T, tag, argument string, field model.ElementType, tail string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v048Element(t, "tag", raw, tag, 1, len(raw)+1)}
	if argument != "" {
		start := utf8.RuneCountInString(raw) + 2
		elements = append(elements, v048Element(t, field, argument, argument, start, start+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	opening := utf8.RuneCountInString(raw) + 2
	raw += " {"
	elements = append(elements, v048Element(t, "block-open", "{", "{", opening, opening+1))
	if tail != "" {
		elements = append(elements, model.Element{ElementType: "unparsed", Raw: tail, Start: opening + 1, End: opening + 1 + utf8.RuneCountInString(tail), ErrorIDs: []string{}})
		raw += tail
	}
	return v048Line(t, raw, "block-start", parent, depth, elements...)
}

// v048Close задаёт отдельное закрытие существующего блока.
func v048Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v048Line(t, "}", "block-end", parent, depth, v048Element(t, "block-close", "}", "}", 1, 2))
}

// v048Text задаёт текст без преобразования его значения.
func v048Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v048Line(t, raw, "content", parent, depth, v048Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v048Invalid задаёт восстановление ошибочной строки целиком в unparsed.
func v048Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v048Line(t, raw, "invalid", parent, depth, model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: utf8.RuneCountInString(raw) + 1, ErrorIDs: []string{}})
}

// v048Run проверяет V048 отдельно от ожидаемых ошибок парности V005.
// История, Raw, структура, ErrorIDs и флаги сравниваются с явными ожиданиями.
func v048Run(t *testing.T, result *model.Result, source string, want []v048Expected, parity ...v048Parity) {
	t.Helper()
	before := v048Snapshot(t, result)
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v048Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v048Snapshot(t, result) != before {
		t.Fatal("Prepare changed model")
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got, others []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i < diagnosticCount {
			continue
		}
		if entry.Source != source {
			t.Fatal("new diagnostic has incorrect source")
		}
		if entry.DiagnosticCode == "V048" {
			got = append(got, entry)
		} else if entry.DiagnosticCode == "V005" {
			others = append(others, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V048 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, occurrence := range want {
		entry := got[i]
		main := model.Location{Start: model.Position{Line: occurrence.line, Column: occurrence.start}, End: model.Position{Line: occurrence.line, Column: occurrence.end}}
		related := model.Location{Start: model.Position{Line: occurrence.line, Column: occurrence.relatedStart}, End: model.Position{Line: occurrence.line, Column: occurrence.relatedEnd}}
		if entry.Location == nil || *entry.Location != main || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "После открывающей фигурной скобки блока есть содержимое." || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Errorf("V048 = %+v at %+v, want %+v with related %+v", entry, entry.Location, main, related)
		}
		expected.Document.HasErrors, expected.Lines[occurrence.line-1].HasErrors = true, true
	}
	if len(others) != len(parity) {
		t.Fatalf("other current errors = %+v, want V005 %v", others, parity)
	}
	for i, occurrence := range parity {
		entry := others[i]
		location := model.Location{Start: model.Position{Line: occurrence.line, Column: occurrence.start}, End: model.Position{Line: occurrence.line, Column: occurrence.end}}
		if entry.DiagnosticCode != "V005" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Location == nil || *entry.Location != location ||
			entry.Message != "Нарушено оформление фигурных скобок.\nконкретизация: "+occurrence.detail || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected other error: %+v at %+v", entry, entry.Location)
		}
		expected.Document.HasErrors, expected.Lines[occurrence.line-1].HasErrors = true, true
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount || result.Processing[processingCount].ID != source || result.Processing[processingCount].Tool != "dsl-validator" {
		t.Fatal("unexpected history changes")
	}
	v048OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], source)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v048Snapshot(t, &previous) != v048Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v048Snapshot сохраняет полную модель для проверки отсутствия побочных изменений.
func v048Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v048OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v048OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V048" || entry.DiagnosticCode == "V005" {
			continue
		}
		if entry.SeverityLevel == "error" {
			expected.Document.HasErrors = true
			if entry.Location != nil {
				expected.Lines[entry.Location.Start.Line-1].HasErrors = true
			}
		}
		if entry.DiagnosticScope != "element" {
			continue
		}
		if entry.Location == nil {
			t.Fatal("element diagnostic has no location")
		}
		matched := false
		for i := range expected.Lines[entry.Location.Start.Line-1].Elements {
			element := &expected.Lines[entry.Location.Start.Line-1].Elements[i]
			if element.Start == entry.Location.Start.Column && element.End == entry.Location.End.Column {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched = true
				break
			}
		}
		if !matched {
			t.Fatal("element diagnostic does not identify an element")
		}
	}
	for i := range expected.Lines {
		for j := range expected.Lines[i].Elements {
			slices.SortStableFunc(expected.Lines[i].Elements[j].ErrorIDs, func(a, b string) int { return order[a] - order[b] })
		}
	}
}

package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v142Expected хранит явные диапазоны [start,end) и конкретизацию.
// Тройка содержит строку и две Unicode-колонки с единицы.
type v142Expected struct {
	primary, related [3]int
	detail           string
}

func TestV142CommentsAndAdjacency(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v142Expected
	}{
		{"blank rows", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestBlank(t, "", 2, 1),
			v142TestBlank(t, " \t", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{6, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"inline editor", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "editor", "content", "Check.", 2, 1),
			v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{5, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"editor block", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestOpen(t, "editor", 2, 1),
			v142TestText(t, "@answer Literal", "@answer Literal", 4, 2), v142TestClose(t, 4, 2), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{7, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"editor and blanks after block", []model.Line{v142TestOpen(t, "multifill", 2, 1), v142TestText(t, "Read.", "Read.", 3, 2),
			v142TestClose(t, 3, 2), v142TestBlank(t, "", 2, 1), v142TestOpen(t, "editor", 2, 1), v142TestBlank(t, "", 7, 2),
			v142TestText(t, "Check.", "Check.", 7, 2), v142TestClose(t, 7, 2), v142TestTag(t, "editor", "content", "Again.", 2, 1),
			v142TestBlank(t, "", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{13, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"editor-like content inside multifill", []model.Line{v142TestOpen(t, "multifill", 2, 1),
			v142TestText(t, "@editor Comment.", "@editor Comment.", 3, 2), v142TestText(t, "@answer London", "@answer London", 3, 2), v142TestClose(t, 3, 2)},
			[]v142Expected{{[3]int{5, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"literal field inside editor", []model.Line{v142TestOpen(t, "editor", 2, 1), v142TestText(t, "@multifill", "@multifill", 3, 2),
			v142TestText(t, "@answer London", "@answer London", 3, 2), v142TestClose(t, 3, 2)}, nil},
		{"literal field inside multifill is not another field", []model.Line{v142TestOpen(t, "multifill", 2, 1),
			v142TestText(t, "@multifill", "@multifill", 3, 2), v142TestText(t, "@answer London", "@answer London", 3, 2), v142TestClose(t, 3, 2)},
			[]v142Expected{{[3]int{5, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v142TestRun(t, v142TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tc := range []struct {
		name    string
		barrier model.Line
	}{
		{"ordinary text", v142TestText(t, "Read.", "Read.", 2, 1)},
		{"NBSP is content", v142TestText(t, "\u00a0", "\u00a0", 2, 1)},
		{"escaped editor", v142TestText(t, `\@editor Comment.`, "@editor Comment.", 2, 1)},
		{"note", v142TestTag(t, "note", "content", "Read.", 2, 1)},
		{"question", v142TestTag(t, "question", "content", "Where?", 2, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1), tc.barrier, v142TestTag(t, "answer", "content", "London", 2, 1))
			if tc.name != "question" {
				v142TestError(t, result, 5, -1, "Ответ без поддерживающего интерактивного элемента.")
			}
			v142TestRun(t, result, "current", nil)
		})
	}
	t.Run("forbidden editor is significant", func(t *testing.T) {
		result := v142TestDocument(t, v142TestOpen(t, "instr", 2, 1), v142TestTag(t, "multifill", "", "", 3, 2),
			v142TestTag(t, "editor", "content", "Check.", 3, 2), v142TestTag(t, "answer", "content", "London", 3, 2), v142TestClose(t, 3, 2))
		for _, number := range []int{4, 5, 6} {
			v142TestError(t, result, number, -1, "Вложенный тег внутри instr запрещён.")
		}
		v142TestRun(t, result, "current", nil)
	})
	for _, block := range []bool{false, true} {
		t.Run("empty editor block="+strconv.FormatBool(block), func(t *testing.T) {
			body := []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "editor", "", "", 2, 1)}
			if block {
				body[1] = v142TestOpen(t, "editor", 2, 1)
				body = append(body, v142TestClose(t, 4, 2))
			}
			body = append(body, v142TestTag(t, "answer", "content", "London", 2, 1))
			result := v142TestDocument(t, body...)
			v142TestError(t, result, 4, -1, "Отсутствует текст комментария.")
			v142TestRun(t, result, "current", nil)
		})
	}
}

func TestV142StructuralContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v142Expected
	}{
		{"active task", []model.Line{v142TestTag(t, "task", "identifier", "t", 2, 1), v142TestTag(t, "multifill", "", "", 3, 2),
			v142TestTag(t, "answer", "content", "London", 3, 2), v142TestTag(t, "endtask", "", "", 2, 1)},
			[]v142Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 11}, "Непосредственно после @multifill."}}},
		{"internal step", []model.Line{v142TestTag(t, "task", "identifier", "t", 2, 1), v142TestTag(t, "step", "title", "First", 3, 2),
			v142TestOpen(t, "multifill", 4, 3), v142TestText(t, "@answer London", "@answer London", 5, 4), v142TestClose(t, 5, 4),
			v142TestTag(t, "answer", "content", "London", 4, 3), v142TestTag(t, "step", "title", "Second", 3, 2),
			v142TestText(t, "Read.", "Read.", 9, 3), v142TestTag(t, "endtask", "", "", 2, 1)},
			[]v142Expected{{[3]int{6, 1, 8}, [3]int{5, 1, 11}, "Внутри блока @multifill."},
				{[3]int{8, 1, 8}, [3]int{5, 1, 11}, "Непосредственно после @multifill."}}},
		{"branch parent", []model.Line{v142TestOpen(t, "variants", 2, 1), v142TestTag(t, "variant", "name", "A", 3, 2),
			v142TestTag(t, "multifill", "", "", 4, 3), v142TestOpen(t, "editor", 4, 3), v142TestText(t, "Check.", "Check.", 6, 4),
			v142TestClose(t, 6, 4), v142TestTag(t, "answer", "content", "London", 4, 3),
			v142TestTag(t, "variant", "name", "B", 3, 2), v142TestText(t, "Read.", "Read.", 10, 3), v142TestClose(t, 3, 2)},
			[]v142Expected{{[3]int{9, 1, 8}, [3]int{5, 1, 11}, "Непосредственно после @multifill."}}},
		{"shared variants parent", []model.Line{v142TestOpen(t, "variants", 2, 1), v142TestTag(t, "variant", "name", "A", 3, 2),
			v142TestTag(t, "multifill", "", "", 3, 2), v142TestTag(t, "editor", "content", "Check.", 3, 2),
			v142TestTag(t, "answer", "content", "London", 3, 2), v142TestTag(t, "variant", "name", "B", 3, 2),
			v142TestText(t, "Read.", "Read.", 3, 2), v142TestClose(t, 3, 2)},
			[]v142Expected{{[3]int{7, 1, 8}, [3]int{5, 1, 11}, "Непосредственно после @multifill."}}},
		{"next branch with shared parent", []model.Line{v142TestOpen(t, "variants", 2, 1), v142TestTag(t, "variant", "name", "A", 3, 2),
			v142TestTag(t, "multifill", "", "", 3, 2), v142TestTag(t, "variant", "name", "B", 3, 2),
			v142TestTag(t, "answer", "content", "London", 3, 2), v142TestClose(t, 3, 2)}, nil},
		{"closing variants", []model.Line{v142TestOpen(t, "variants", 2, 1), v142TestTag(t, "variant", "name", "A", 3, 2),
			v142TestText(t, "Read.", "Read.", 4, 3), v142TestTag(t, "variant", "name", "B", 3, 2),
			v142TestTag(t, "multifill", "", "", 6, 3), v142TestClose(t, 3, 2), v142TestTag(t, "answer", "content", "London", 2, 1)}, nil},
		{"another block is significant", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestOpen(t, "note", 2, 1),
			v142TestText(t, "Read.", "Read.", 4, 2), v142TestClose(t, 4, 2), v142TestTag(t, "answer", "content", "London", 2, 1)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v142TestRun(t, v142TestDocument(t, tc.body...), "current", tc.want) })
	}
	for _, tag := range []string{"task", "step", "endtask", "header", "newpage"} {
		t.Run("boundary "+tag, func(t *testing.T) {
			boundary := v142TestTag(t, tag, "", "", 2, 1)
			answerParent, answerDepth := 2, 1
			switch tag {
			case "task":
				boundary = v142TestTag(t, tag, "identifier", "second", 2, 1)
				answerParent, answerDepth = 6, 2
			case "step":
				boundary = v142TestTag(t, tag, "title", "Second", 3, 2)
				answerParent, answerDepth = 6, 3
			case "header":
				boundary = v142TestTag(t, tag, "title", "More", 2, 1)
				answerParent, answerDepth = 6, 2
			case "newpage":
				answerParent, answerDepth = 6, 2
			}
			result := v142TestDocument(t, v142TestTag(t, "task", "identifier", "first", 2, 1), v142TestTag(t, "step", "title", "First", 3, 2),
				v142TestTag(t, "multifill", "", "", 4, 3), boundary, v142TestTag(t, "answer", "content", "London", answerParent, answerDepth))
			v142TestError(t, result, 7, -1, "Ответ не относится к поддерживающему интерактивному элементу.")
			v142TestRun(t, result, "current", nil)
		})
	}
	t.Run("heading boundary", func(t *testing.T) {
		result := v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1),
			v142TestLine(t, "### Next", "heading", 0, 0, v142TestElement(t, "heading-level", "###", "3", 1, 4), v142TestElement(t, "title", "Next", "Next", 5, 9)),
			v142TestTag(t, "answer", "content", "London", 4, 1))
		v142TestError(t, result, 5, -1, "Ответ не относится к поддерживающему интерактивному элементу.")
		v142TestRun(t, result, "current", nil)
	})
}

func TestV142PositionsEndingsAndHistory(t *testing.T) {
	for _, ending := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(ending, func(t *testing.T) {
			result := v142TestDocument(t,
				v142TestLine(t, " \t@MuLtIfIlL Текст.", "tag", 2, 1, v142TestElement(t, "tag", "@MuLtIfIlL", "multifill", 3, 13),
					v142TestElement(t, "content", "Текст.", "Текст.", 14, 20)),
				v142TestLine(t, "\t@AnSwEr Лондон", "tag", 2, 1, v142TestElement(t, "tag", "@AnSwEr", "answer", 2, 9),
					v142TestElement(t, "content", "Лондон", "Лондон", 10, 16)))
			for index := range result.Lines {
				switch ending {
				case "LF":
					result.Lines[index].LineEnding = "\n"
				case "CRLF":
					result.Lines[index].LineEnding = "\r\n"
				case "mixed":
					if index%2 == 0 {
						result.Lines[index].LineEnding = "\r\n"
					}
				}
			}
			want := []v142Expected{{[3]int{4, 2, 9}, [3]int{3, 3, 13}, "Непосредственно после @multifill."}}
			v142TestRun(t, result, "first", want)
			v142TestRun(t, result, "second", want)
			v142TestRun(t, v142TestDocument(t, v142TestTag(t, "question", "content", "Where?", 2, 1),
				v142TestTag(t, "answer", "content", "London", 2, 1)), "separate", nil)
		})
	}
	t.Run("unclosed field still checks own content", func(t *testing.T) {
		result := v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1), v142TestText(t, "@answer London", "@answer London", 3, 2))
		v142TestError(t, result, 3, -1, "Блок не закрыт до конца файла.")
		v142TestRun(t, result, "current", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}})
	})
	t.Run("field at EOF", func(t *testing.T) {
		v142TestRun(t, v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1)), "current", nil)
	})
	t.Run("closing field at EOF", func(t *testing.T) {
		v142TestRun(t, v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1), v142TestText(t, "Read.", "Read.", 3, 2), v142TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("escaped field", func(t *testing.T) {
		v142TestRun(t, v142TestDocument(t, v142TestText(t, `\@multifill`, "@multifill", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1)), "current", nil)
	})
	t.Run("escaped answer is significant content", func(t *testing.T) {
		v142TestRun(t, v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1),
			v142TestText(t, `\@answer London`, "@answer London", 2, 1), v142TestTag(t, "answer", "content", "Paris", 2, 1)), "current", nil)
	})
	t.Run("recognized answer with unparsed remainder", func(t *testing.T) {
		unparsed := v142TestElement(t, "unparsed", "}", "", 16, 17)
		unparsed.Value = nil
		result := v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1),
			v142TestLine(t, "@answer London }", "invalid", 2, 1, v142TestElement(t, "tag", "@answer", "answer", 1, 8),
				v142TestElement(t, "content", "London", "London", 9, 15), unparsed))
		v142TestError(t, result, 4, 2, "Лишняя закрывающая скобка.")
		v142TestRun(t, result, "current", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}})
	})
	t.Run("erroneous editor is not skipped", func(t *testing.T) {
		unparsed := v142TestElement(t, "unparsed", "}", "", 16, 17)
		unparsed.Value = nil
		result := v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1),
			v142TestLine(t, "@editor Check. }", "invalid", 2, 1, v142TestElement(t, "tag", "@editor", "editor", 1, 8),
				v142TestElement(t, "content", "Check.", "Check.", 9, 15), unparsed), v142TestTag(t, "answer", "content", "London", 2, 1))
		v142TestError(t, result, 4, 2, "Лишняя закрывающая скобка.")
		v142TestRun(t, result, "current", nil)
	})
	t.Run("unclosed editor does not expose literal answer", func(t *testing.T) {
		result := v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1), v142TestOpen(t, "editor", 2, 1),
			v142TestText(t, "@answer London", "@answer London", 4, 2))
		v142TestError(t, result, 4, -1, "Комментарий не закрыт до конца файла.")
		v142TestRun(t, result, "current", nil)
	})
}

func TestV142EmptyAndVersions(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v142TestRun(t, result, "empty", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v142TestDocument(t, v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1))
			result.Document.DSLVersion = &version
			result.Lines[0] = v142TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			registry, err := diagnostics.NewRegistry(RegistrationCheckers())
			if err != nil {
				t.Fatal(err)
			}
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

func TestV142Registration(t *testing.T) {
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
		if definition.Code != "V142" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "@answer размещён внутри @multifill или непосредственно после него." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V142 definitions = %d, want 1", count)
	}
}

func TestV142ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v142Expected
	}{
		{"reference", "@multifill {\nShe _____{lives} in London.\n}", []model.Line{
			v142TestOpen(t, "multifill", 2, 1),
			v142TestLine(t, "She _____{lives} in London.", "content", 3, 2,
				v142TestElement(t, "content", "She ", "She ", 1, 5), v142TestElement(t, "placeholder", "_____{lives}", "lives", 5, 17),
				v142TestElement(t, "content", " in London.", " in London.", 17, 28)), v142TestClose(t, 3, 2)}, nil},
		{"example", "@multifill Write your answer.\n@answer London", []model.Line{
			v142TestTag(t, "multifill", "content", "Write your answer.", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.30: дословный фрагмент дополнен только версией и # Lesson.
			result := v142TestDocument(t, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v142TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV142InsideSource(t *testing.T) {
	for _, tc := range []struct {
		raw, value string
		want       []v142Expected
	}{
		{"@answer London", "@answer London", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{" \t@AnSwEr London", "@AnSwEr London", []v142Expected{{[3]int{4, 3, 10}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"@ANSWER", "@ANSWER", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"@answer\tLondon", "@answer\tLondon", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"@answer\u00a0London", "@answer\u00a0London", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"@answer{", "@answer{", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"@answer}", "@answer}", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"@answerExtra", "@answerExtra", nil}, {"@answer!", "@answer!", nil}, {"@answer_1", "@answer_1", nil},
		{"@answer\u200bLondon", "@answer\u200bLondon", nil}, {"\u00a0@answer London", "\u00a0@answer London", nil},
		{`\@answer London`, "@answer London", nil}, {`\\@answer London`, `\@answer London`, nil},
		{"Text @answer London", "Text @answer London", nil}, {"<span>@answer London</span>", "<span>@answer London</span>", nil},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			result := v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1), v142TestText(t, tc.raw, tc.value, 3, 2), v142TestClose(t, 3, 2))
			if len(tc.want) != 0 {
				v142TestError(t, result, 4, -1, "Тег answer внутри multifill запрещён.")
			}
			v142TestRun(t, result, "current", tc.want)
		})
	}
	for _, value := range []string{"London", "@answer"} {
		t.Run("placeholder "+value, func(t *testing.T) {
			raw := "_____{" + value + "}"
			result := v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1),
				v142TestLine(t, raw, "content", 3, 2, v142TestElement(t, "placeholder", raw, value, 1, utf8.RuneCountInString(raw)+1)), v142TestClose(t, 3, 2))
			v142TestRun(t, result, "current", nil)
		})
	}
	for _, kind := range []model.LineType{"content", "invalid", "tag"} {
		t.Run("unparsed "+string(kind), func(t *testing.T) {
			unparsed := v142TestElement(t, "unparsed", "@answer London", "", 1, 15)
			unparsed.Value = nil
			result := v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1), v142TestLine(t, "@answer London", kind, 3, 2, unparsed), v142TestClose(t, 3, 2))
			v142TestError(t, result, 4, 0, "Неразобранное запрещённое объявление.")
			v142TestRun(t, result, "current", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}})
		})
	}
	t.Run("recognized tag gives only one message", func(t *testing.T) {
		result := v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1), v142TestTag(t, "answer", "content", "London", 3, 2), v142TestClose(t, 3, 2))
		v142TestError(t, result, 4, -1, "Тег answer внутри multifill запрещён.")
		v142TestRun(t, result, "current", []v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}})
	})
}

func TestV142FormsAndOccurrences(t *testing.T) {
	for _, checked := range []bool{false, true} {
		t.Run("placeholder does not permit following answer checked="+strconv.FormatBool(checked), func(t *testing.T) {
			placeholder := v142TestElement(t, "placeholder", "_____", "", 1, 6)
			placeholder.Value = nil
			if checked {
				placeholder = v142TestElement(t, "placeholder", "_____{London}", "London", 1, 14)
			}
			result := v142TestDocument(t, v142TestOpen(t, "multifill", 2, 1),
				v142TestLine(t, placeholder.Raw, "content", 3, 2, placeholder), v142TestClose(t, 3, 2),
				v142TestTag(t, "answer", "content", "London", 2, 1))
			v142TestRun(t, result, "current", []v142Expected{{[3]int{6, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}})
		})
	}
	t.Run("real field in forbidden container remains checked", func(t *testing.T) {
		result := v142TestDocument(t, v142TestOpen(t, "instr", 2, 1), v142TestOpen(t, "multifill", 3, 2),
			v142TestText(t, "@answer London", "@answer London", 4, 3), v142TestClose(t, 4, 3),
			v142TestTag(t, "answer", "content", "Paris", 3, 2), v142TestClose(t, 3, 2))
		v142TestError(t, result, 4, -1, "Поле multifill внутри instr запрещено.")
		v142TestError(t, result, 5, -1, "Тег answer внутри multifill запрещён.")
		v142TestError(t, result, 7, -1, "Тег answer внутри instr запрещён.")
		v142TestRun(t, result, "current", []v142Expected{
			{[3]int{5, 1, 8}, [3]int{4, 1, 11}, "Внутри блока @multifill."},
			{[3]int{7, 1, 8}, [3]int{4, 1, 11}, "Непосредственно после @multifill."}})
	})
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v142Expected
	}{
		{"bare inline field", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"after block", []model.Line{v142TestOpen(t, "multifill", 2, 1), v142TestText(t, "Read.", "Read.", 3, 2),
			v142TestClose(t, 3, 2), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{6, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"block answer after field", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestOpen(t, "answer", 2, 1),
			v142TestText(t, "London", "London", 4, 2), v142TestClose(t, 4, 2)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"inside and after", []model.Line{v142TestOpen(t, "multifill", 2, 1), v142TestText(t, "@answer London", "@answer London", 3, 2),
			v142TestClose(t, 3, 2), v142TestTag(t, "answer", "content", "Paris", 2, 1)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."},
				{[3]int{6, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"two inside", []model.Line{v142TestOpen(t, "multifill", 2, 1), v142TestText(t, "@answer London", "@answer London", 3, 2),
			v142TestText(t, "@answer Paris", "@answer Paris", 3, 2), v142TestClose(t, 3, 2)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."},
				{[3]int{5, 1, 8}, [3]int{3, 1, 11}, "Внутри блока @multifill."}}},
		{"two following answers", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1),
			v142TestTag(t, "answer", "content", "Paris", 2, 1)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
		{"two fields", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "answer", "content", "London", 2, 1),
			v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "answer", "content", "Paris", 2, 1)},
			[]v142Expected{{[3]int{4, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."},
				{[3]int{6, 1, 8}, [3]int{5, 1, 11}, "Непосредственно после @multifill."}}},
		{"only latest adjacent field", []model.Line{v142TestTag(t, "multifill", "", "", 2, 1), v142TestTag(t, "multifill", "content", "Write.", 2, 1),
			v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{5, 1, 8}, [3]int{4, 1, 11}, "Непосредственно после @multifill."}}},
		{"instruction and preloaded text", []model.Line{
			v142TestLine(t, "@multifill Write. {", "block-start", 2, 1,
				v142TestElement(t, "tag", "@multifill", "multifill", 1, 11), v142TestElement(t, "content", "Write.", "Write.", 12, 18), v142TestElement(t, "block-open", "{", "{", 19, 20)),
			v142TestText(t, "Read.", "Read.", 3, 2), v142TestClose(t, 3, 2), v142TestTag(t, "answer", "content", "London", 2, 1)},
			[]v142Expected{{[3]int{6, 1, 8}, [3]int{3, 1, 11}, "Непосредственно после @multifill."}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v142TestRun(t, v142TestDocument(t, tc.body...), "current", tc.want) })
	}
}

// v142TestRun запускает штатный конвейер и проверяет только V142 текущего запуска.
func v142TestRun(t *testing.T, result *model.Result, source string, want []v142Expected) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V142" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V142 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		primary := model.Location{Start: model.Position{Line: expected.primary[0], Column: expected.primary[1]}, End: model.Position{Line: expected.primary[0], Column: expected.primary[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]}, End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "@answer размещён внутри @multifill или непосредственно после него.\nконкретизация: "+expected.detail ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original model.Result
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	for index, line := range original.Lines {
		current := result.Lines[index]
		current.HasErrors = line.HasErrors
		current.Elements = append([]model.Element{}, current.Elements...)
		for element := range current.Elements {
			current.Elements[element].ErrorIDs = line.Elements[element].ErrorIDs
		}
		left, _ := json.Marshal(line)
		right, _ := json.Marshal(current)
		if string(left) != string(right) {
			t.Fatalf("source changed on line %d", index+1)
		}
	}
	for index, old := range original.Diagnostics {
		left, _ := json.Marshal(old)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v142TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v142TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
	})
}

// v142TestElement задаёт исходное и семантическое значения без преобразований.
func v142TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v142TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v142TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v142TestTag создаёт однострочное объявление с явно заданным параметром.
func v142TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v142TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v142TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v142TestLine(t, raw, "tag", parent, depth, elements...)
}

// v142TestOpen создаёт объявление блока без параметров.
func v142TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v142TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v142TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v142TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v142TestClose создаёт закрытие с заданным родителем.
func v142TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v142TestLine(t, "}", "block-end", parent, depth, v142TestElement(t, "block-close", "}", "}", 1, 2))
}

// v142TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v142TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v142TestLine(t, raw, "content", parent, depth, v142TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v142TestBlank задаёт обычную пустую строку без текстового элемента.
func v142TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v142TestLine(t, raw, "blank", parent, depth)
}

// v142TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v142TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v142TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v142TestLine(t, "# Lesson", "heading", 0, 0, v142TestElement(t, "heading-level", "#", "1", 1, 2), v142TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

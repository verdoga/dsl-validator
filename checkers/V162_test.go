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

// v162Expected задаёт строку и Unicode-колонки [начало,конец) элемента тега.
type v162Expected struct{ line, start, end int }

func TestV162ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		separator    bool
		want         []v162Expected
	}{
		{"reference", "@matching {\nСолнце — звезда?\n---\nДа\nНет\n}", true, nil},
		{"example", "@matching {\nСолнце — звезда?\nДа\nНет\n}", false, []v162Expected{{3, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []model.Line{v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2)}
			if tc.separator {
				body = append(body, v162TestLine(t, "---", "separator", 3, 2))
			}
			body = append(body, v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
			var raw []string
			for _, line := range body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// Добавлены только версия и основной заголовок по DSL 1.30.
			v162TestRun(t, v162TestDocument(t, body...), "current", tc.want)
		})
	}
}

func TestV162SeparatorSpelling(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		separator        bool
		want             []v162Expected
	}{
		{"exact", "---", "---", true, nil},
		{"edge spaces", "  ---  ", "---", true, nil},
		{"edge tabs", "\t---\t", "---", true, nil},
		{"mixed edge spaces", " \t ---\t \t", "---", true, nil},
		{"escaped", `\---`, "---", false, []v162Expected{{3, 1, 10}}},
		{"escaped slash", `\\---`, `\---`, false, []v162Expected{{3, 1, 10}}},
		{"two hyphens", "--", "--", false, []v162Expected{{3, 1, 10}}},
		{"four hyphens", "----", "----", false, []v162Expected{{3, 1, 10}}},
		{"inner spaces", "- - -", "- - -", false, []v162Expected{{3, 1, 10}}},
		{"inner tab", "--\t-", "--\t-", false, []v162Expected{{3, 1, 10}}},
		{"trailing text", "--- текст", "--- текст", false, []v162Expected{{3, 1, 10}}},
		{"leading text", "текст ---", "текст ---", false, []v162Expected{{3, 1, 10}}},
		{"Unicode dashes", "———", "———", false, []v162Expected{{3, 1, 10}}},
		{"NBSP before", "\u00a0---", "\u00a0---", false, []v162Expected{{3, 1, 10}}},
		{"NBSP after", "---\u00a0", "---\u00a0", false, []v162Expected{{3, 1, 10}}},
		{"em space before", "\u2003---", "\u2003---", false, []v162Expected{{3, 1, 10}}},
		{"em space after", "---\u2003", "---\u2003", false, []v162Expected{{3, 1, 10}}},
		{"vertical tab before", "\v---", "\v---", false, []v162Expected{{3, 1, 10}}},
		{"vertical tab after", "---\v", "---\v", false, []v162Expected{{3, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v162TestText(t, tc.raw, tc.value, 3, 2)
			if tc.separator {
				line = v162TestLine(t, tc.raw, "separator", 3, 2)
			}
			v162TestRun(t, v162TestDocument(t, v162TestOpen(t, "matching", 2, 1),
				v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), line,
				v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("blank lines are not separators", func(t *testing.T) {
		v162TestRun(t, v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestBlank(t, "", 3, 2),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v162TestBlank(t, " \t", 3, 2),
			v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2)), "current", []v162Expected{{3, 1, 10}})
	})
	t.Run("split lines are not joined", func(t *testing.T) {
		v162TestRun(t, v162TestDocument(t, v162TestOpen(t, "matching", 2, 1),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v162TestText(t, "--", "--", 3, 2), v162TestText(t, "-", "-", 3, 2),
			v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2)), "current", []v162Expected{{3, 1, 10}})
	})
	for _, unparsed := range []bool{false, true} {
		t.Run("second separator unparsed="+strconv.FormatBool(unparsed), func(t *testing.T) {
			line := v162TestLine(t, "---", "separator", 3, 2)
			if unparsed {
				element := v162TestElement(t, "unparsed", "---", "", 1, 4)
				element.Value = nil
				line = v162TestLine(t, "---", "invalid", 3, 2, element)
			}
			result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
				v162TestLine(t, "---", "separator", 3, 2), v162TestText(t, "Да", "Да", 3, 2), line, v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
			index := -1
			if unparsed {
				index = 0
			}
			v162TestError(t, result, 7, index, "В matching допустим один разделитель.")
			v162TestRun(t, result, "current", nil)
		})
	}
}

func TestV162NestedContent(t *testing.T) {
	for _, own := range []bool{false, true} {
		for _, tag := range []string{"answer", "hint"} {
			t.Run(tag+" own separator="+strconv.FormatBool(own), func(t *testing.T) {
				body := []model.Line{v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2)}
				if tag == "hint" {
					body = append(body, v162TestTag(t, "answer", "content", "Да", 3, 2))
				}
				opening := len(body) + 3
				body = append(body, v162TestOpen(t, tag, 3, 2), v162TestText(t, "---", "---", opening, 3), v162TestClose(t, opening, 3))
				want := []v162Expected{{3, 1, 10}}
				if own {
					body = append(body, v162TestLine(t, "---", "separator", 3, 2))
					want = nil
				}
				// Литеральный нижний вариант обеспечивает допустимую ссылку
				// ответа "---"; его экранированная запись не разделяет списки.
				body = append(body, v162TestText(t, `\---`, "---", 3, 2), v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
				v162TestRun(t, v162TestDocument(t, body...), "current", want)
			})
		}
	}
	for _, childSeparator := range []bool{false, true} {
		t.Run("nested matching separator="+strconv.FormatBool(childSeparator), func(t *testing.T) {
			body := []model.Line{v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
				v162TestOpen(t, "matching", 3, 2), v162TestText(t, "Вода — жидкость?", "Вода — жидкость?", 5, 3)}
			want := []v162Expected{{3, 1, 10}, {5, 1, 10}}
			if childSeparator {
				body = append(body, v162TestLine(t, "---", "separator", 5, 3))
				want = []v162Expected{{3, 1, 10}}
			}
			body = append(body, v162TestText(t, "Да", "Да", 5, 3), v162TestText(t, "Нет", "Нет", 5, 3), v162TestClose(t, 5, 3),
				v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
			result := v162TestDocument(t, body...)
			v162TestError(t, result, 5, -1, "Вложенный matching запрещён; его границы сохранены.")
			v162TestRun(t, result, "current", want)
		})
	}
}

func TestV162ContextsAndIndependentBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v162Expected
	}{
		{"task", []model.Line{v162TestTag(t, "task", "identifier", "first", 2, 1), v162TestOpen(t, "matching", 3, 2),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 4, 3), v162TestText(t, "Да", "Да", 4, 3), v162TestText(t, "Нет", "Нет", 4, 3),
			v162TestClose(t, 4, 3), v162TestTag(t, "endtask", "", "", 2, 1)}, []v162Expected{{4, 1, 10}}},
		{"internal step", []model.Line{v162TestTag(t, "task", "identifier", "first", 2, 1), v162TestTag(t, "step", "title", "First", 3, 2),
			v162TestOpen(t, "matching", 4, 3), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 5, 4),
			v162TestText(t, "Да", "Да", 5, 4), v162TestText(t, "Нет", "Нет", 5, 4), v162TestClose(t, 5, 4), v162TestTag(t, "endtask", "", "", 2, 1)}, []v162Expected{{5, 1, 10}}},
		{"variant", []model.Line{v162TestOpen(t, "variants", 2, 1), v162TestTag(t, "variant", "name", "A", 3, 2), v162TestOpen(t, "matching", 4, 3),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 5, 4), v162TestText(t, "Да", "Да", 5, 4), v162TestText(t, "Нет", "Нет", 5, 4),
			v162TestClose(t, 5, 4), v162TestTag(t, "variant", "name", "B", 3, 2), v162TestText(t, "Читайте.", "Читайте.", 10, 3), v162TestClose(t, 3, 2)}, []v162Expected{{5, 1, 10}}},
		{"instruction is not body", []model.Line{v162TestBlock(t, "matching", "content", "---", 2, 1),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2)}, []v162Expected{{3, 1, 10}}},
		{"other container", []model.Line{v162TestOpen(t, "ordering", 2, 1), v162TestText(t, "First", "First", 3, 2),
			v162TestText(t, "Second", "Second", 3, 2), v162TestClose(t, 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v162TestDocument(t, tc.body...)
			if tc.name == "other container" {
				v162TestError(t, result, 3, -1, "В ordering отсутствует разделитель списков.")
			}
			v162TestRun(t, result, "current", tc.want)
		})
	}
	for _, secondSeparator := range []bool{false, true} {
		t.Run("second block separator="+strconv.FormatBool(secondSeparator), func(t *testing.T) {
			body := []model.Line{v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
				v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2),
				v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Вода — жидкость?", "Вода — жидкость?", 8, 2)}
			want := []v162Expected{{3, 1, 10}, {8, 1, 10}}
			if secondSeparator {
				body = append(body, v162TestLine(t, "---", "separator", 8, 2))
				want = []v162Expected{{3, 1, 10}}
			}
			body = append(body, v162TestText(t, "Да", "Да", 8, 2), v162TestText(t, "Нет", "Нет", 8, 2), v162TestClose(t, 8, 2))
			v162TestRun(t, v162TestDocument(t, body...), "current", want)
		})
	}
	t.Run("earlier block separator cannot fill later block", func(t *testing.T) {
		v162TestRun(t, v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
			v162TestLine(t, "---", "separator", 3, 2), v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2),
			v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Вода — жидкость?", "Вода — жидкость?", 9, 2),
			v162TestText(t, "Да", "Да", 9, 2), v162TestText(t, "Нет", "Нет", 9, 2), v162TestClose(t, 9, 2)), "current", []v162Expected{{9, 1, 10}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v162TestRun(t, v162TestDocument(t, v162TestOpen(t, tag, 2, 1), v162TestText(t, "@matching {", "@matching {", 3, 2),
				v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2),
				v162TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV162BlockBoundaries(t *testing.T) {
	t.Run("inline matching excluded", func(t *testing.T) {
		result := v162TestDocument(t, v162TestTag(t, "matching", "content", "Выберите ответ.", 2, 1))
		v162TestError(t, result, 3, -1, "Matching допускает только блочную форму.")
		v162TestRun(t, result, "current", nil)
	})
	t.Run("missing closing", func(t *testing.T) {
		result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
			v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2))
		v162TestError(t, result, 3, -1, "Блок matching не закрыт.")
		v162TestRun(t, result, "current", nil)
	})
	t.Run("ambiguous closing", func(t *testing.T) {
		result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
			v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2), v162TestClose(t, 3, 2))
		// Такая ошибочная связь допустима по контракту CheckModel;
		// V162 не выбирает удобное закрытие из двух сохранённых парсером.
		v162TestError(t, result, 8, -1, "Неоднозначная связь закрытия блока.")
		v162TestRun(t, result, "current", nil)
	})
	t.Run("closed matching inside unclosed parent", func(t *testing.T) {
		result := v162TestDocument(t, v162TestBlock(t, "fragment", "identifier", "f", 2, 1), v162TestOpen(t, "matching", 3, 2),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 4, 3), v162TestText(t, "Да", "Да", 4, 3), v162TestText(t, "Нет", "Нет", 4, 3), v162TestClose(t, 4, 3))
		v162TestError(t, result, 3, -1, "Родительский fragment не закрыт.")
		v162TestError(t, result, 4, -1, "Интерактивный matching запрещён в fragment.")
		v162TestRun(t, result, "current", []v162Expected{{4, 1, 10}})
	})
	t.Run("empty closed block", func(t *testing.T) {
		result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestClose(t, 3, 2))
		v162TestError(t, result, 3, -1, "Отсутствуют элементы списков.")
		v162TestRun(t, result, "current", []v162Expected{{3, 1, 10}})
	})
	t.Run("first body line even when unparsed", func(t *testing.T) {
		element := v162TestElement(t, "unparsed", "---", "", 1, 4)
		element.Value = nil
		result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestLine(t, "---", "invalid", 3, 2, element),
			v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
		v162TestError(t, result, 4, 0, "Разделитель перед пустым верхним списком.")
		v162TestRun(t, result, "current", nil)
	})
	t.Run("last body line", func(t *testing.T) {
		result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
			v162TestLine(t, "---", "separator", 3, 2), v162TestClose(t, 3, 2))
		v162TestError(t, result, 5, -1, "Нижний список пуст.")
		v162TestRun(t, result, "current", nil)
	})
	t.Run("recognized malformed opening still has boundaries", func(t *testing.T) {
		result := v162TestDocument(t, v162TestLine(t, "@matching{", "block-start", 2, 1,
			v162TestElement(t, "tag", "@matching", "matching", 1, 10), v162TestElement(t, "block-open", "{", "{", 10, 11)),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
		v162TestError(t, result, 3, -1, "Нет разделителя перед открывающей скобкой.")
		v162TestRun(t, result, "current", []v162Expected{{3, 1, 10}})
	})
}

func TestV162RangesEndingsAndRepeatedRuns(t *testing.T) {
	t.Run("mixed case and indentation", func(t *testing.T) {
		v162TestRun(t, v162TestDocument(t, v162TestLine(t, " \t@MaTcHiNg {", "block-start", 2, 1,
			v162TestElement(t, "tag", "@MaTcHiNg", "matching", 3, 12), v162TestElement(t, "block-open", "{", "{", 13, 14)),
			v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2), v162TestText(t, "Да", "Да", 3, 2),
			v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2)), "current", []v162Expected{{3, 3, 12}})
	})
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
				v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			v162TestRun(t, result, "first", []v162Expected{{3, 1, 10}})
			v162TestRun(t, result, "second", []v162Expected{{3, 1, 10}})
			v162TestRun(t, v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
				v162TestLine(t, "---", "separator", 3, 2), v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2)), "separate", nil)
		})
	}
}

func TestV162EmptyAndRegistration(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v162TestRun(t, result, "empty", nil)
		})
	}
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
		if definition.Code != "V162" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "element" || definition.Check == nil ||
			definition.Message != "В @matching отсутствует разделитель списков ---." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V162 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v162TestDocument(t, v162TestOpen(t, "matching", 2, 1), v162TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3, 2),
				v162TestText(t, "Да", "Да", 3, 2), v162TestText(t, "Нет", "Нет", 3, 2), v162TestClose(t, 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0] = v162TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v162TestBlock задаёт параметр объявления перед открывающей скобкой.
func v162TestBlock(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	line := v162TestTag(t, tag, kind, value, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v162TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v162TestRun запускает штатный конвейер и проверяет только V162 текущего запуска.
func v162TestRun(t *testing.T, result *model.Result, source string, want []v162Expected) {
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
		if diagnostic.DiagnosticCode == "V162" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V162 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" ||
			entry.Message != "В @matching отсутствует разделитель списков ---." ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id != entry.ID {
						continue
					}
					links++
					if line.Number != expected.line || element.Start != expected.start || element.End != expected.end ||
						element.ElementType != "tag" || element.Value == nil || *element.Value != "matching" {
						t.Fatalf("V162 linked to wrong element: line %d %+v", line.Number, element)
					}
				}
			}
		}
		if links != 1 || !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatalf("links=%d or error flags missing for %+v", links, expected)
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

// v162TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v162TestError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v162TestElement задаёт исходное и семантическое значения без преобразований.
func v162TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v162TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v162TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v162TestTag создаёт однострочное объявление с явно заданным параметром.
func v162TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v162TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v162TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v162TestLine(t, raw, "tag", parent, depth, elements...)
}

// v162TestOpen создаёт объявление блока без параметров.
func v162TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v162TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v162TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v162TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v162TestClose создаёт закрытие с заданным родителем.
func v162TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v162TestLine(t, "}", "block-end", parent, depth, v162TestElement(t, "block-close", "}", "}", 1, 2))
}

// v162TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v162TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v162TestLine(t, raw, "content", parent, depth, v162TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v162TestBlank задаёт обычную пустую строку без текстового элемента.
func v162TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v162TestLine(t, raw, "blank", parent, depth)
}

// v162TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v162TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v162TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v162TestLine(t, "# Lesson", "heading", 0, 0, v162TestElement(t, "heading-level", "#", "1", 1, 2), v162TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

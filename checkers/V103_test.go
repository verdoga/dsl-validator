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

func TestV103Registration(t *testing.T) {
	definitions, err := v103TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V103" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Заголовочная часть таблицы содержит больше одной строки." ||
			definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V103 definitions = %d, want 1", count)
	}
}

func TestV103ReferenceAndExample(t *testing.T) {
	// Добавлены только обязательные версия и заголовок документа.
	// &#124; в таблице задания представляет |, а не текст HTML-сущности.
	t.Run("reference", func(t *testing.T) {
		result := v103TestTable(t, v103TestRow(t, "Форма | Значение"),
			v103TestSeparator(t, "---"), v103TestRow(t, "am | есть"))
		v103TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@table {\nФорма | Значение\n---\nam | есть\n}")
		v103TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v103TestTable(t, v103TestRow(t, "Форма | Значение"),
			v103TestRow(t, "Вид | Перевод"), v103TestSeparator(t, "---"), v103TestRow(t, "am | есть"))
		v103TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@table {\nФорма | Значение\nВид | Перевод\n---\nam | есть\n}")
		v103TestRun(t, result, "current", [][6]int{{5, 1, 14, 4, 1, 17}})
	})
}

func TestV103HeaderRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want [][6]int
	}{
		{"empty table", nil, nil},
		{"no separator", []model.Line{v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestRow(t, "D")}, nil},
		{"one header", []model.Line{
			v103TestRow(t, "A"), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, nil},
		{"two headers", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}}},
		{"three headers related to first", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestRow(t, "C"),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}, {6, 1, 2, 4, 1, 2}}},
		{"identical rows", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "A"), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}}},
		{"case does not matter", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "a"), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}}},
		{"blank between headers", []model.Line{
			v103TestRow(t, "A"), v103TestLine(t, "", "blank", 3, 2), v103TestRow(t, "B"),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{6, 1, 2, 4, 1, 2}}},
		{"leading and trailing blank rows", []model.Line{
			v103TestLine(t, " \t", "blank", 3, 2), v103TestRow(t, "A"),
			v103TestLine(t, "\t ", "blank", 3, 2), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, nil},
		{"trim only edges", []model.Line{
			v103TestText(t, "  A \t", "A", 3, 2), v103TestText(t, "\t B\t", "B", 3, 2),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 3, 4, 4, 3, 4}}},
		{"internal spaces and tabs stay in range", []model.Line{
			v103TestText(t, "\tA B\t", "A B", 3, 2), v103TestText(t, " B\t C ", "B\t C", 3, 2),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 2, 6, 4, 2, 5}}},
		{"empty cells are rows", []model.Line{
			v103TestRow(t, "|"), v103TestRow(t, "|"), v103TestSeparator(t, "---"), v103TestRow(t, "|"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}}},
		{"many split cells still one physical row", []model.Line{
			v103TestRow(t, "<A|B> | <C|D>"), v103TestSeparator(t, "---"), v103TestRow(t, "X | Y"),
		}, nil},
		{"split cells on two rows", []model.Line{
			v103TestRow(t, "<A|B>"), v103TestRow(t, "<C|D>"), v103TestSeparator(t, "---"), v103TestRow(t, "X"),
		}, [][6]int{{5, 1, 6, 4, 1, 6}}},
		{"Unicode and NBSP", []model.Line{
			v103TestRow(t, "Я😀"), v103TestRow(t, "\u00a0"), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 2, 4, 1, 3}}},
		{"combining marks use source columns", []model.Line{
			v103TestText(t, " е\u0301 ", "е\u0301", 3, 2), v103TestRow(t, "😀Я"),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 3, 4, 2, 4}}},
		{"body rows are not headers", []model.Line{
			v103TestRow(t, "A"), v103TestSeparator(t, "---"),
			v103TestRow(t, "B"), v103TestRow(t, "C"), v103TestRow(t, "D"),
		}, nil},
		{"escaped data uses original range", []model.Line{
			v103TestText(t, `A\|B`, "A|B", 3, 2), v103TestText(t, `C\|D`, "C|D", 3, 2),
			v103TestSeparator(t, "---"), v103TestRow(t, "X"),
		}, [][6]int{{5, 1, 5, 4, 1, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v103TestRun(t, v103TestTable(t, tc.rows...), "current", tc.want)
		})
	}
}

func TestV103Separators(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want [][6]int
	}{
		{"no header", []model.Line{v103TestSeparator(t, "---"), v103TestRow(t, "D")}, nil},
		{"only separator", []model.Line{v103TestSeparator(t, "---")}, nil},
		{"no body does not suppress extra header", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}}},
		{"two separators", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"),
			v103TestRow(t, "D"), v103TestSeparator(t, "---"), v103TestRow(t, "E"),
		}, nil},
		{"consecutive separators", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, nil},
		{"invalid repeated separator still counted", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"),
			v103TestRow(t, "D"), v103TestInvalid(t, "---", 3, 2), v103TestRow(t, "E"),
		}, nil},
		{"content with unparsed repeated separator still counted", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"),
			v103TestRow(t, "D"), v103TestLine(t, "---", "content", 3, 2, v103TestUnparsed(t, "---", 1)),
			v103TestRow(t, "E"),
		}, nil},
		{"spaces and tabs around separator", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, " \t---\t "), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 2, 4, 1, 2}}},
		{"escaped separator is header data", []model.Line{
			v103TestRow(t, "A"), v103TestText(t, `\---`, "---", 3, 2),
			v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 5, 4, 1, 2}}},
		{"escaped separator alone", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestText(t, `\---`, "---", 3, 2),
		}, nil},
		{"NBSP prevents separator", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestRow(t, "\u00a0---"),
		}, nil},
		{"trailing NBSP prevents separator", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestRow(t, "---\u00a0"),
		}, nil},
		{"separator must occupy whole row", []model.Line{
			v103TestRow(t, "A | B"), v103TestRow(t, "--- | C"),
		}, nil},
		{"four dashes are data", []model.Line{
			v103TestRow(t, "A"), v103TestRow(t, "----"), v103TestSeparator(t, "---"), v103TestRow(t, "D"),
		}, [][6]int{{5, 1, 5, 4, 1, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v103TestRun(t, v103TestTable(t, tc.rows...), "current", tc.want)
		})
	}
}

func TestV103BrokenRowsStillCount(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want [][6]int
	}{
		{"invalid extra header", []model.Line{
			v103TestRow(t, "A"), v103TestInvalid(t, "@unknown", 3, 2),
		}, [][6]int{{5, 1, 9, 4, 1, 2}}},
		{"invalid first header", []model.Line{
			v103TestInvalid(t, "@unknown", 3, 2), v103TestRow(t, "B"),
		}, [][6]int{{5, 1, 2, 4, 1, 9}}},
		{"unparsed first header", []model.Line{
			v103TestLine(t, "<A<B>>", "content", 3, 2, v103TestUnparsed(t, "<A<B>>", 1)), v103TestRow(t, "B"),
		}, [][6]int{{5, 1, 2, 4, 1, 7}}},
		{"unparsed extra header", []model.Line{
			v103TestRow(t, "A"), v103TestLine(t, "<A<B>>", "content", 3, 2, v103TestUnparsed(t, "<A<B>>", 1)),
		}, [][6]int{{5, 1, 7, 4, 1, 2}}},
		{"partially unparsed extra header", []model.Line{
			v103TestRow(t, "A | B"), v103TestLine(t, " A | <B<C>> \t", "content", 3, 2,
				v103TestElement(t, "content", "A | ", "A | ", 2), v103TestUnparsed(t, "<B<C>>", 6)),
		}, [][6]int{{5, 2, 12, 4, 1, 6}}},
		{"single broken header is not extra", []model.Line{v103TestInvalid(t, "@unknown", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := append(tc.rows, v103TestSeparator(t, "---"), v103TestRow(t, "D"))
			v103TestRun(t, v103TestTable(t, rows...), "current", tc.want)
		})
	}
}

func TestV103Contexts(t *testing.T) {
	t.Run("title and tag case", func(t *testing.T) {
		v103TestRun(t, v103TestDocument(t,
			v103TestLine(t, "@TABLE Title {", "block-start", 2, 1,
				v103TestElement(t, "tag", "@TABLE", "table", 1),
				v103TestElement(t, "title", "Title", "Title", 8),
				v103TestElement(t, "block-open", "{", "{", 14)),
			v103TestRow(t, "A"), v103TestSeparator(t, "---"), v103TestRow(t, "D"), v103TestClose(t, 3, 2)),
			"current", nil)
	})
	t.Run("nested block and single-line declaration are excluded", func(t *testing.T) {
		v103TestRun(t, v103TestTable(t,
			v103TestRow(t, "A"), v103TestOpen(t, "note", 3, 2),
			v103TestText(t, "B", "B", 5, 3), v103TestText(t, "C", "C", 5, 3), v103TestClose(t, 5, 3),
			v103TestNamed(t, "editor", "content", "Comment", 3, 2),
			v103TestSeparator(t, "---"), v103TestRow(t, "D")), "current", nil)
	})
	t.Run("nested separator does not suppress outer error", func(t *testing.T) {
		v103TestRun(t, v103TestTable(t,
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestOpen(t, "editor", 3, 2),
			v103TestText(t, "---", "---", 6, 3), v103TestClose(t, 6, 3),
			v103TestSeparator(t, "---"), v103TestRow(t, "D")), "current", [][6]int{{5, 1, 2, 4, 1, 2}})
	})
	t.Run("table inside fragment", func(t *testing.T) {
		v103TestRun(t, v103TestDocument(t,
			v103TestLine(t, "@fragment part {", "block-start", 2, 1,
				v103TestElement(t, "tag", "@fragment", "fragment", 1),
				v103TestElement(t, "name", "part", "part", 11),
				v103TestElement(t, "block-open", "{", "{", 16)),
			v103TestOpen(t, "table", 3, 2), v103TestText(t, "A", "A", 4, 3),
			v103TestText(t, "B", "B", 4, 3), v103TestLine(t, "---", "separator", 4, 3),
			v103TestText(t, "D", "D", 4, 3), v103TestClose(t, 4, 3), v103TestClose(t, 3, 2)),
			"current", [][6]int{{6, 1, 2, 5, 1, 2}})
	})
	t.Run("tables have independent first headers", func(t *testing.T) {
		v103TestRun(t, v103TestDocument(t, v103TestOpen(t, "table", 2, 1),
			v103TestRow(t, "A"), v103TestSeparator(t, "---"), v103TestRow(t, "D"), v103TestClose(t, 3, 2),
			v103TestOpen(t, "table", 2, 1), v103TestText(t, "B", "B", 8, 2),
			v103TestLine(t, "---", "separator", 8, 2), v103TestText(t, "E", "E", 8, 2),
			v103TestClose(t, 8, 2)), "current", nil)
	})
	t.Run("each table reports its own extra header", func(t *testing.T) {
		v103TestRun(t, v103TestDocument(t, v103TestOpen(t, "table", 2, 1),
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"),
			v103TestRow(t, "D"), v103TestClose(t, 3, 2),
			v103TestOpen(t, "table", 2, 1), v103TestText(t, "C", "C", 9, 2),
			v103TestText(t, "E", "E", 9, 2), v103TestLine(t, "---", "separator", 9, 2),
			v103TestText(t, "F", "F", 9, 2), v103TestClose(t, 9, 2)), "current",
			[][6]int{{5, 1, 2, 4, 1, 2}, {11, 1, 2, 10, 1, 2}})
	})
	t.Run("nested table has independent separator and rows", func(t *testing.T) {
		// Незаконная вложенность не смешивает строки и разделители таблиц.
		v103TestRun(t, v103TestTable(t,
			v103TestRow(t, "A"), v103TestOpen(t, "table", 3, 2),
			v103TestText(t, "B", "B", 5, 3), v103TestText(t, "C", "C", 5, 3),
			v103TestLine(t, "---", "separator", 5, 3), v103TestText(t, "D", "D", 5, 3),
			v103TestClose(t, 5, 3), v103TestSeparator(t, "---"), v103TestRow(t, "E")),
			"current", [][6]int{{7, 1, 2, 6, 1, 2}})
	})
	for _, tag := range []string{"editor", "multifill", "text"} {
		t.Run("not a table: "+tag, func(t *testing.T) {
			v103TestRun(t, v103TestDocument(t, v103TestOpen(t, tag, 2, 1),
				v103TestText(t, "A", "A", 3, 2), v103TestText(t, "B", "B", 3, 2),
				v103TestText(t, "---", "---", 3, 2), v103TestText(t, "D", "D", 3, 2),
				v103TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("literal table declaration does not create table", func(t *testing.T) {
		v103TestRun(t, v103TestDocument(t, v103TestOpen(t, "editor", 2, 1),
			v103TestText(t, "@table {", "@table {", 3, 2), v103TestText(t, "A", "A", 3, 2),
			v103TestText(t, "B", "B", 3, 2), v103TestText(t, "---", "---", 3, 2),
			v103TestText(t, "D", "D", 3, 2), v103TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("unfinished table does not need a block range", func(t *testing.T) {
		v103TestRun(t, v103TestDocument(t, v103TestOpen(t, "table", 2, 1),
			v103TestRow(t, "A"), v103TestRow(t, "B"), v103TestSeparator(t, "---"), v103TestRow(t, "D")),
			"current", [][6]int{{5, 1, 2, 4, 1, 2}})
	})
}

func TestV103EndingsVersionsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"no final EOL", "\n", ""}, {"CRLF no final EOL", "\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v103TestTable(t, v103TestRow(t, "A"), v103TestRow(t, "B"),
				v103TestSeparator(t, "---"), v103TestRow(t, "D"))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v103TestRun(t, result, "first", [][6]int{{5, 1, 2, 4, 1, 2}})
			v103TestRun(t, result, "second", [][6]int{{5, 1, 2, 4, 1, 2}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v103TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v103TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v103TestTable(t, v103TestRow(t, "A"), v103TestRow(t, "B"),
				v103TestSeparator(t, "---"), v103TestRow(t, "D"))
			result.Document.DSLVersion = &version
			before := v103TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v103TestRegistry(t))
			if err == nil || checks != nil || v103TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v103TestTable помещает явно заданные строки в таблицу на строке 3.
func v103TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v103TestOpen(t, "table", 2, 1)}, rows...)
	return v103TestDocument(t, append(body, v103TestClose(t, 3, 2))...)
}

// v103TestRow задаёт обычную строку с неизменённым семантическим текстом.
func v103TestRow(t *testing.T, raw string) model.Line {
	t.Helper()
	return v103TestText(t, raw, raw, 3, 2)
}

// v103TestSeparator задаёт разделитель с родителем table на строке 3.
func v103TestSeparator(t *testing.T, raw string) model.Line {
	t.Helper()
	return v103TestLine(t, raw, "separator", 3, 2)
}

// v103TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v103TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v103TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v103TestHeading(t, "#", "Lesson"),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		lines[i].Elements = append([]model.Element{}, lines[i].Elements...)
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v103TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v103TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v103TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		element := &line.Elements[index]
		scope, start, end = diagnostics.ScopeElement, element.Start, element.End
		element.ErrorIDs = []string{id}
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
}

func v103TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v103TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v103TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v103TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v103TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v103TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v103TestLine(t, raw, "invalid", parent, depth, v103TestUnparsed(t, raw, 1))
}

func v103TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v103TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v103TestElement(t, "tag", "@"+tag, tag, 1), v103TestElement(t, kind, value, value, len(tag)+3))
}

func v103TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v103TestLine(t, marker+" "+title, "heading", 0, 0,
		v103TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v103TestElement(t, "title", title, title, len(marker)+2))
}

func v103TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v103TestLine(t, raw, "content", parent, depth, v103TestElement(t, "content", raw, value, 1))
}

func v103TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v103TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v103TestElement(t, "tag", "@"+tag, tag, 1), v103TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v103TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v103TestLine(t, "}", "block-end", parent, depth, v103TestElement(t, "block-close", "}", "}", 1))
}

func v103TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v103TestRun выполняет полный каталог. Ожидания заданы шестёрками:
// строка/начало/конец лишней строки, строка/начало/конец первой строки.
// Все правые границы исключаются; сравниваются только новые V103.
func v103TestRun(t *testing.T, result *model.Result, processingID string, want [][6]int) {
	t.Helper()
	before := v103TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v103TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v103TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V103" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V103 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		related := model.Location{Start: model.Position{Line: position[3], Column: position[4]},
			End: model.Position{Line: position[3], Column: position[5]}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("occurrence %d = %+v, want location %+v and related %+v", i, entry, location, related)
		}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Заголовочная часть таблицы содержит больше одной строки." {
			t.Fatalf("unexpected V103 metadata: %+v", entry)
		}
	}
	// CheckModel проверяет уникальность ID, ссылки и флаги всех диагностик.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v103TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v103TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v103TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v103TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var source strings.Builder
	for _, line := range result.Lines {
		source.WriteString(line.Raw)
		source.WriteString(string(line.LineEnding))
	}
	if source.String() != want {
		t.Fatalf("source = %q, want %q", source.String(), want)
	}
}

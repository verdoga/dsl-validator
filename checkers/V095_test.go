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

func TestV095Registration(t *testing.T) {
	definitions, err := v095TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V095" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "block" ||
			definition.Message != "Таблица не содержит строк данных." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V095 definitions = %d, want 1", count)
	}
}

func TestV095ReferenceAndExample(t *testing.T) {
	t.Run("reference", func(t *testing.T) {
		// &#124; в таблице задания представляет |, а не текст HTML-сущности.
		// Добавлены только обязательные версия и заголовок.
		result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestText(t, "|", "|", 3, 2), v095TestClose(t, 3, 2))
		v095TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@table {\n|\n}")
		v095TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestLine(t, "", "blank", 3, 2), v095TestClose(t, 3, 2))
		v095TestSource(t, result, "@dsl-version 1.2\n# Lesson\n@table {\n\n}")
		v095TestRun(t, result, "current", [][4]int{{3, 1, 5, 2}})
	})
}

func TestV095DataAndEmptyRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want [][4]int
	}{
		{"no rows", nil, [][4]int{{3, 1, 4, 2}}},
		{"spaces and tabs", []model.Line{
			v095TestLine(t, "   ", "blank", 3, 2),
			v095TestLine(t, "\t \t", "blank", 3, 2),
		}, [][4]int{{3, 1, 6, 2}}},
		{"minimal data", []model.Line{v095TestText(t, "a", "a", 3, 2)}, nil},
		{"three empty cells", []model.Line{v095TestText(t, "||", "||", 3, 2)}, nil},
		{"spaces around empty cells", []model.Line{v095TestText(t, " | ", "|", 3, 2)}, nil},
		{"NBSP", []model.Line{v095TestText(t, "\u00a0", "\u00a0", 3, 2)}, nil},
		{"Unicode", []model.Line{v095TestText(t, "Я | 😀", "Я | 😀", 3, 2)}, nil},
		{"internal tab", []model.Line{v095TestText(t, "A\tB", "A\tB", 3, 2)}, nil},
		{"first row is data", []model.Line{
			v095TestText(t, "A | B", "A | B", 3, 2), v095TestLine(t, "", "blank", 3, 2),
		}, nil},
		{"last row is data", []model.Line{
			v095TestLine(t, "", "blank", 3, 2), v095TestText(t, "A | B", "A | B", 3, 2),
		}, nil},
		{"data between blanks", []model.Line{
			v095TestLine(t, "", "blank", 3, 2), v095TestText(t, "A | B", "A | B", 3, 2),
			v095TestLine(t, "", "blank", 3, 2),
		}, nil},
		{"escaped separator is data", []model.Line{v095TestText(t, `\---`, "---", 3, 2)}, nil},
		{"escaped pipe is data", []model.Line{v095TestText(t, `\|`, "|", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v095TestOpen(t, "table", 2, 1)}, tc.rows...)
			body = append(body, v095TestClose(t, 3, 2))
			v095TestRun(t, v095TestDocument(t, body...), "current", tc.want)
		})
	}
	t.Run("title is not data", func(t *testing.T) {
		v095TestRun(t, v095TestDocument(t,
			v095TestLine(t, "@table Я😀 {", "block-start", 2, 1,
				v095TestElement(t, "tag", "@table", "table", 1),
				v095TestElement(t, "title", "Я😀", "Я😀", 8),
				v095TestElement(t, "block-open", "{", "{", 11)),
			v095TestLine(t, "", "blank", 3, 2), v095TestClose(t, 3, 2)),
			"current", [][4]int{{3, 1, 5, 2}})
	})
	t.Run("indented boundaries and case", func(t *testing.T) {
		v095TestRun(t, v095TestDocument(t,
			v095TestLine(t, "  @TABLE {", "block-start", 2, 1,
				v095TestElement(t, "tag", "@TABLE", "table", 3),
				v095TestElement(t, "block-open", "{", "{", 10)),
			v095TestLine(t, "", "blank", 3, 2),
			v095TestLine(t, "  } ", "block-end", 3, 2,
				v095TestElement(t, "block-close", "}", "}", 3))),
			"current", [][4]int{{3, 3, 5, 4}})
	})
}

func TestV095Separators(t *testing.T) {
	for _, raw := range []string{"---", " \t---\t "} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			separator := v095TestLine(t, raw, "separator", 3, 2)
			result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
				separator, v095TestClose(t, 3, 2))
			v095TestParserError(t, result, 4, -1, "Отсутствуют строки с обеих сторон разделителя.")
			v095TestRun(t, result, "current", nil)
		})
	}
	for _, kind := range []model.LineType{"invalid", "content"} {
		t.Run("raw separator despite "+string(kind), func(t *testing.T) {
			// Парсер сохранил проблемный разделитель без распознанного элемента.
			// V095 самостоятельно обнаруживает его, даже при пустых частях.
			result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
				v095TestLine(t, "---", kind, 3, 2, v095TestUnparsed(t, "---", 1)),
				v095TestClose(t, 3, 2))
			v095TestRun(t, result, "current", nil)
		})
	}
	for _, tc := range []struct {
		name string
		rows []model.Line
	}{
		{"separator first", []model.Line{
			v095TestLine(t, "---", "separator", 3, 2), v095TestText(t, "|", "|", 3, 2),
		}},
		{"separator last", []model.Line{
			v095TestText(t, "|", "|", 3, 2), v095TestLine(t, "---", "separator", 3, 2),
		}},
		{"repeated separator", []model.Line{
			v095TestLine(t, "---", "separator", 3, 2), v095TestLine(t, "---", "separator", 3, 2),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v095TestOpen(t, "table", 2, 1)}, tc.rows...)
			body = append(body, v095TestClose(t, 3, 2))
			result := v095TestDocument(t, body...)
			v095TestParserError(t, result, 3, -1, "Нарушено разделение частей таблицы.")
			v095TestRun(t, result, "current", nil)
		})
	}
	t.Run("broken cell is not a separator", func(t *testing.T) {
		// Незакрытая разделённая ячейка делает всю строку неразобранной.
		// Она не является отдельным --- и не заполняет таблицу.
		v095TestRun(t, v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestInvalid(t, "<---\u00a0", 3, 2), v095TestClose(t, 3, 2)),
			"current", [][4]int{{3, 1, 5, 2}})
	})
}

func TestV095BrokenContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][4]int
	}{
		{"invalid", v095TestInvalid(t, "@unknown", 3, 2), [][4]int{{3, 1, 5, 2}}},
		{"unparsed content", v095TestLine(t, "<broken", "content", 3, 2,
			v095TestUnparsed(t, "<broken", 1)), [][4]int{{3, 1, 5, 2}}},
		{"unparsed with only spaces outside", v095TestLine(t, " \t<broken \t", "content", 3, 2,
			v095TestUnparsed(t, "<broken", 3)), [][4]int{{3, 1, 5, 2}}},
		{"recognized prefix", v095TestLine(t, "A | <broken", "content", 3, 2,
			v095TestElement(t, "content", "A | ", "A | ", 1),
			v095TestUnparsed(t, "<broken", 5)), nil},
		{"recognized suffix", v095TestLine(t, "<bad>> | B", "content", 3, 2,
			v095TestUnparsed(t, "<bad>>", 1),
			v095TestElement(t, "content", " | B", " | B", 7)), nil},
		{"two broken cells", v095TestLine(t, "<a>>\t<b>>", "content", 3, 2,
			v095TestUnparsed(t, "<a>>", 1), v095TestUnparsed(t, "<b>>", 6)), [][4]int{{3, 1, 5, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v095TestRun(t, v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
				tc.line, v095TestClose(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("valid data beside invalid", func(t *testing.T) {
		v095TestRun(t, v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestInvalid(t, "@unknown", 3, 2), v095TestText(t, "|", "|", 3, 2),
			v095TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("independent from V035", func(t *testing.T) {
		result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestInvalid(t, "@unknown", 3, 2), v095TestClose(t, 3, 2))
		v095TestRun(t, result, "current", [][4]int{{3, 1, 5, 2}})
		for _, entry := range result.Diagnostics {
			if entry.Source == "current" && entry.DiagnosticCode == "V035" {
				t.Fatal("V035 must retain its separate handling of unknown content")
			}
		}
	})
}

func TestV095NestedBlocks(t *testing.T) {
	t.Run("single line tag is not data", func(t *testing.T) {
		result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestNamed(t, "note", "content", "Text.", 3, 2), v095TestClose(t, 3, 2))
		v095TestParserError(t, result, 4, -1, "Вложенный тег запрещён в table.")
		v095TestRun(t, result, "current", [][4]int{{3, 1, 5, 2}})
	})
	for _, raw := range []string{"Text.", "---", "@unknown"} {
		t.Run("nested note "+raw, func(t *testing.T) {
			row := v095TestText(t, raw, raw, 4, 3)
			if raw == "@unknown" {
				row = v095TestInvalid(t, raw, 4, 3)
			}
			result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
				v095TestOpen(t, "note", 3, 2), row,
				v095TestClose(t, 4, 3), v095TestClose(t, 3, 2))
			v095TestParserError(t, result, 4, -1, "Вложенный блок запрещён в table.")
			if raw == "@unknown" {
				// Контракт требует позиционного порядка: ошибка объявления
				// на строке 4 предшествует ошибке его содержимого на строке 5.
				result.Diagnostics[0], result.Diagnostics[1] = result.Diagnostics[1], result.Diagnostics[0]
			}
			v095TestRun(t, result, "current", [][4]int{{3, 1, 7, 2}})
		})
	}
	for _, tc := range []struct {
		name string
		row  model.Line
		want [][4]int
	}{
		{"filled child", v095TestText(t, "|", "|", 4, 3), [][4]int{{3, 1, 7, 2}}},
		{"empty child", v095TestLine(t, "", "blank", 4, 3), [][4]int{{3, 1, 7, 2}, {4, 1, 6, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
				v095TestOpen(t, "table", 3, 2), tc.row,
				v095TestClose(t, 4, 3), v095TestClose(t, 3, 2))
			v095TestParserError(t, result, 4, -1, "Вложенные таблицы запрещены.")
			v095TestRun(t, result, "current", tc.want)
		})
	}
	t.Run("table in text", func(t *testing.T) {
		result := v095TestDocument(t, v095TestOpen(t, "text", 2, 1),
			v095TestOpen(t, "table", 3, 2), v095TestLine(t, "", "blank", 4, 3),
			v095TestClose(t, 4, 3), v095TestClose(t, 3, 2))
		v095TestParserError(t, result, 4, -1, "Вложенная таблица запрещена в text.")
		v095TestRun(t, result, "current", [][4]int{{4, 1, 6, 2}})
	})
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			// Первая неэкранированная } закрывает литеральный контейнер.
			v095TestRun(t, v095TestDocument(t, v095TestOpen(t, tag, 2, 1),
				v095TestText(t, "@table {", "@table {", 3, 2),
				v095TestText(t, "|", "|", 3, 2), v095TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV095Boundaries(t *testing.T) {
	t.Run("unclosed table", func(t *testing.T) {
		result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1))
		v095TestParserError(t, result, 3, -1, "Не закрыт блок table.")
		v095TestRun(t, result, "current", nil)
	})
	t.Run("ambiguous closing", func(t *testing.T) {
		// CheckModel допускает сохранённые ошибочные связи парсера;
		// два закрытия не дают однозначного диапазона блока.
		result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestClose(t, 3, 2), v095TestClose(t, 3, 2))
		v095TestParserError(t, result, 5, -1, "Неоднозначная закрывающая скобка.")
		v095TestRun(t, result, "current", nil)
	})
	t.Run("neighboring tables", func(t *testing.T) {
		v095TestRun(t, v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestLine(t, "", "blank", 3, 2), v095TestClose(t, 3, 2),
			v095TestOpen(t, "table", 2, 1), v095TestLine(t, "", "blank", 6, 2),
			v095TestClose(t, 6, 2)), "current", [][4]int{{3, 1, 5, 2}, {6, 1, 8, 2}})
	})
	t.Run("text after closing", func(t *testing.T) {
		v095TestRun(t, v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
			v095TestLine(t, "", "blank", 3, 2), v095TestClose(t, 3, 2),
			v095TestText(t, "Outside.", "Outside.", 2, 1)), "current", [][4]int{{3, 1, 5, 2}})
	})
}

func TestV095EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v095TestDocument(t, v095TestOpen(t, "table", 2, 1),
				v095TestLine(t, "", "blank", 3, 2), v095TestClose(t, 3, 2))
			for i := range result.Lines {
				if eol != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = eol
				}
			}
			v095TestRun(t, result, "first", [][4]int{{3, 1, 5, 2}})
			v095TestRun(t, result, "second", [][4]int{{3, 1, 5, 2}})
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			result := v095TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v095TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v095TestDocument(t)
			result.Document.DSLVersion = &version
			before := v095TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v095TestRegistry(t))
			if err == nil || checks != nil || v095TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v095TestDocument добавляет версию и заголовок перед готовыми строками.
// Родители заданы абсолютными номерами; исходный текст не разбирается.
func v095TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v095TestNamed(t, "dsl-version", "version", "1.2", 0, 0), v095TestHeading(t, "#", "Lesson"),
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
				v095TestParserError(t, result, i+1, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v095TestParserError создаёт обязательную исходную ошибку; index=-1 задаёт
// область line, иначе ошибка привязывается к указанному элементу.
func v095TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

func v095TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v095TestElement задаёт исходный Unicode-диапазон, исключая правую границу.
func v095TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v095TestUnparsed(t *testing.T, raw string, start int) model.Element {
	t.Helper()
	element := v095TestElement(t, "unparsed", raw, "", start)
	element.Value = nil
	return element
}

func v095TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v095TestLine(t, raw, "invalid", parent, depth, v095TestUnparsed(t, raw, 1))
}

func v095TestNamed(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	return v095TestLine(t, "@"+tag+" "+value, "tag", parent, depth,
		v095TestElement(t, "tag", "@"+tag, tag, 1), v095TestElement(t, kind, value, value, len(tag)+3))
}

func v095TestHeading(t *testing.T, marker, title string) model.Line {
	t.Helper()
	return v095TestLine(t, marker+" "+title, "heading", 0, 0,
		v095TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v095TestElement(t, "title", title, title, len(marker)+2))
}

func v095TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v095TestLine(t, raw, "content", parent, depth, v095TestElement(t, "content", raw, value, 1))
}

func v095TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v095TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v095TestElement(t, "tag", "@"+tag, tag, 1), v095TestElement(t, "block-open", "{", "{", len(tag)+3))
}

func v095TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v095TestLine(t, "}", "block-end", parent, depth, v095TestElement(t, "block-close", "}", "}", 1))
}

func v095TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v095TestRun выполняет полный каталог и сравнивает новые V095 с явными
// четвёрками начальная строка/колонка и конечная строка/колонка. Правые границы исключаются.
func v095TestRun(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v095TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v095TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v095TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V095" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V095 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[2], Column: position[3]}}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "block" || entry.Fatal || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.Message != "Таблица не содержит строк данных." {
			t.Fatalf("occurrence %d = %+v, want location %+v and exact V095 metadata", i, entry, location)
		}
	}
	// CheckModel проверяет также уникальность ID, ссылки и флаги всех проверок.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v095TestSnapshot(t, result)), &restored); err != nil {
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
	if v095TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v095TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v095TestSource(t *testing.T, result *model.Result, want string) {
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

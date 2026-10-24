package checkers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v097TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V097.
func v097TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v097TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v097TestHeading(t, "", "#", "1", title),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	result.Lines[len(result.Lines)-1].LineEnding = ""
	return result
}

// v097TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v097TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v097TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v097TestElement(t, "heading-level", marker, level, start, end),
		v097TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v097TestTag собирает однострочный тег с одним явно типизированным параметром.
func v097TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v097TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v097TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v097TestLine(t, raw, "tag", parent, depth, elements...)
}

// v097TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v097TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v097TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v097TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v097TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v097TestClose задаёт закрытие блока с явным родителем.
func v097TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v097TestLine(t, "}", "block-end", parent, depth, v097TestElement(t, "block-close", "}", "}", 1, 2))
}

// v097TestText сохраняет независимо заданные исходное и семантическое значения.
func v097TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v097TestLine(t, raw, "content", parent, depth,
		v097TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v097TestLine не классифицирует исходник и не вычисляет родительские связи.
func v097TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v097TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v097TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v097TestRegistry использует штатный каталог целиком.
func v097TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v097TestRun проверяет новые V097: явные диапазоны строки/образца и оба количества ячеек.
func v097TestRun(t *testing.T, result *model.Result, processingID string, want [][6]int) {
	t.Helper()
	before := v097TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v097TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v097TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V097" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V097 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: 1},
			End: model.Position{Line: position[0], Column: position[1]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != fmt.Sprintf("Количество внешних ячеек отличается от других строк таблицы.\nконкретизация: Найдено внешних ячеек: %d; в строке-образце: %d.", position[4], position[5]) {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V097 metadata", i, entry, location)
		}
		related := model.Location{Start: model.Position{Line: position[2], Column: 1}, End: model.Position{Line: position[2], Column: position[3]}}
		if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v097TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 ||
		len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v097TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v097TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v097TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV097Registration(t *testing.T) {
	definitions, err := v097TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V097" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Количество внешних ячеек отличается от других строк таблицы." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V097 definitions = %d, want 1", count)
	}
}

// v097TestTitled задаёт блок с отдельным элементом названия.
func v097TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v097TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v097TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v097TestElement(t, "title", title, title, len(tag)+3, end),
		v097TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v097TestBroken сохраняет ошибочные данные вместе с unparsed; тип content
// означает, что назначение строки известно, хотя её содержимое не разобрано.
func v097TestBroken(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v097TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v097TestLine(t, raw, "content", parent, depth, element)
}

func TestV097ReferenceAndExample(t *testing.T) {
	// Добавлены только обязательные @dsl-version и # Lesson по DSL 1.30.
	// Сущности и <br> из таблицы задания — оформление, а не символы DSL.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           [][6]int
	}{
		{"reference", "@table {\nName | Forms\n---\nA | <x | y>\nB | <x | y | z>\n}", []model.Line{
			v097TestOpen(t, "table", 2, 1), v097TestText(t, "Name | Forms", "Name | Forms", 3, 2),
			v097TestLine(t, "---", "separator", 3, 2), v097TestText(t, "A | <x | y>", "A | <x | y>", 3, 2),
			v097TestText(t, "B | <x | y | z>", "B | <x | y | z>", 3, 2), v097TestClose(t, 3, 2)}, nil},
		{"example", "@table {\nA | B\n| C | D |\n}", []model.Line{
			v097TestOpen(t, "table", 2, 1), v097TestText(t, "A | B", "A | B", 3, 2),
			v097TestText(t, "| C | D |", "| C | D |", 3, 2), v097TestClose(t, 3, 2)}, [][6]int{{5, 10, 4, 6, 4, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v097TestDocument(t, tc.body...)
			var raw strings.Builder
			for _, line := range result.Lines[2:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", raw.String())
			}
			v097TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV097Cells(t *testing.T) {
	// Ожидания: строка, исключающий конец, строка/конец образца, найдено/образец.
	for _, tc := range []struct {
		name, raw, value string
		want             [][6]int
	}{
		{"same", "C | D", "C | D", nil},
		{"fewer", "C", "C", [][6]int{{5, 2, 4, 6, 1, 2}}},
		{"more", "C | D | E", "C | D | E", [][6]int{{5, 10, 4, 6, 3, 2}}},
		{"empty middle", "||", "||", [][6]int{{5, 3, 4, 6, 3, 2}}},
		{"two empty cells", "|", "|", nil},
		{"leading empty", "|A", "|A", nil},
		{"trailing empty", "A|", "A|", nil},
		{"empty edges and middle", "|A||", "|A||", [][6]int{{5, 5, 4, 6, 4, 2}}},
		{"split is one cell", "<x|y|z>", "<x|y|z>", [][6]int{{5, 8, 4, 6, 1, 2}}},
		{"empty internal parts", "<|>", "<|>", [][6]int{{5, 4, 4, 6, 1, 2}}},
		{"different internal counts", "<a|b> | <x|y|z>", "<a|b> | <x|y|z>", nil},
		{"split then ordinary", "<a|b> | C", "<a|b> | C", nil},
		{"ordinary then split", "A | <b|c>", "A | <b|c>", nil},
		{"no spaces", "A|<b|c>", "A|<b|c>", nil},
		{"split and empty", "<a|b>|", "<a|b>|", nil},
		{"empty and split", "|<a|b>", "|<a|b>", nil},
		{"split plus two empty", "|<a|b>|", "|<a|b>|", [][6]int{{5, 8, 4, 6, 3, 2}}},
		{"double quotes", `"A|B"`, `"A|B"`, nil},
		{"single quotes", `'A|B'`, `'A|B'`, nil},
		{"escaped pipe", `\|`, "|", [][6]int{{5, 3, 4, 6, 1, 2}}},
		{"paired slashes", `\\|`, `\|`, nil},
		{"three slashes", `\\\|`, `\|`, [][6]int{{5, 5, 4, 6, 1, 2}}},
		{"four slashes", `\\\\|`, `\\|`, nil},
		{"escaped opening", `A | \<x|y\>`, "A | <x|y>", [][6]int{{5, 12, 4, 6, 3, 2}}},
		{"escaped angles inside", `A | <1 \< 2 | 3 \> 2>`, "A | <1 < 2 | 3 > 2>", nil},
		{"paired slashes before closing", `<a|b\\>`, `<a|b\>`, [][6]int{{5, 8, 4, 6, 1, 2}}},
		{"paired slashes before inner pipe", `<a\\|b>`, `<a\|b>`, [][6]int{{5, 8, 4, 6, 1, 2}}},
		{"slash text prevents split opening", `\\<a|b\>`, `\<a|b>`, nil},
		{"ordinary angles", "1 < 2 | 3 > 0", "1 < 2 | 3 > 0", nil},
		{"ordinary internal opening", "a<b | c", "a<b | c", nil},
		{"opening inside quotes is ordinary", `"<a|b>"`, `"<a|b>"`, nil},
		{"punctuation", "A, B; C: D!", "A, B; C: D!", [][6]int{{5, 12, 4, 6, 1, 2}}},
		{"tabs do not split", "A\tB", "A\tB", [][6]int{{5, 4, 4, 6, 1, 2}}},
		{"whole raw range", "\t C \t", "C", [][6]int{{5, 6, 4, 6, 1, 2}}},
		{"unicode columns", "Я😀", "Я😀", [][6]int{{5, 3, 4, 6, 1, 2}}},
		{"nbsp is data", "\u00a0", "\u00a0", [][6]int{{5, 2, 4, 6, 1, 2}}},
		{"nbsp prevents split opening", "\u00a0<a|b>z", "\u00a0<a|b>z", nil},
		{"ascii margins around split", " \t<a|b>\t | C \t", "<a|b>\t | C", nil},
		{"escaped last greater", `A | B\>`, "A | B>", nil},
		{"escaped separator row", `\---`, "---", [][6]int{{5, 5, 4, 6, 1, 2}}},
		{"hyphens with cells", "---|---", "---|---", nil},
		{"four hyphens are data", "----", "----", [][6]int{{5, 5, 4, 6, 1, 2}}},
		{"escaped initial at", `\@note | C`, "@note | C", nil},
		{"escaped initial hash", `\# title | C`, "# title | C", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
				v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, tc.raw, tc.value, 3, 2),
				v097TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV097MalformedCells(t *testing.T) {
	for _, raw := range []string{
		"<a|b", "<a>", "<>", `<a\|b>`, `<a\\\|b>`, "<a|<b|c>>", "<a|b<c>",
		"<a|b>text", "<a|b>\ttext", "<a|b>>", "<a|b><c|d>", "<a|b>\u00a0",
		`<a|b>\|C`, `<a|b>\\|C`, `<a|b\>`, `<a|b\\\>`,
		"A | B>", "A> | B", ">", "a>b>\t", "x<a|b>",
	} {
		t.Run(raw, func(t *testing.T) {
			// Даже строка с определимой одной внешней ячейкой не должна получить V097.
			v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
				v097TestText(t, "A | B", "A | B", 3, 2), v097TestBroken(t, raw, 3, 2),
				v097TestClose(t, 3, 2)), "excluded", nil)
			// Та же ошибка не становится образцом; первый пригодный образец — строка 5.
			v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
				v097TestBroken(t, raw, 3, 2), v097TestText(t, "A | B", "A | B", 3, 2),
				v097TestText(t, "C", "C", 3, 2), v097TestClose(t, 3, 2)),
				"after invalid", [][6]int{{6, 2, 5, 6, 1, 2}})
		})
	}
}

func TestV097ComparisonAndScope(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][6]int
	}{
		{"first reference persists", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, "C", "C", 3, 2),
			v097TestText(t, "D | E | F", "D | E | F", 3, 2), v097TestText(t, "G | H", "G | H", 3, 2),
			v097TestClose(t, 3, 2)}, [][6]int{{5, 2, 4, 6, 1, 2}, {6, 10, 4, 6, 3, 2}}},
		{"one column reference", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A", "A", 3, 2), v097TestText(t, "B|C", "B|C", 3, 2), v097TestClose(t, 3, 2)},
			[][6]int{{5, 4, 4, 2, 2, 1}}},
		{"invalid middle preserves reference", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestBroken(t, "<a>", 3, 2),
			v097TestText(t, "C", "C", 3, 2), v097TestClose(t, 3, 2)}, [][6]int{{6, 2, 4, 6, 1, 2}}},
		{"header and body share reference", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestLine(t, "---", "separator", 3, 2),
			v097TestText(t, "C", "C", 3, 2), v097TestClose(t, 3, 2)}, [][6]int{{6, 2, 4, 6, 1, 2}}},
		{"header rows also compared", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, "C", "C", 3, 2),
			v097TestLine(t, "---", "separator", 3, 2), v097TestText(t, "D | E", "D | E", 3, 2),
			v097TestClose(t, 3, 2)}, [][6]int{{5, 2, 4, 6, 1, 2}}},
		{"title blank and separator excluded", []model.Line{v097TestTitled(t, "table", "One", 2, 1),
			v097TestLine(t, " \t", "blank", 3, 2), v097TestText(t, "A | B", "A | B", 3, 2),
			v097TestLine(t, " \t---\t", "separator", 3, 2), v097TestLine(t, "", "blank", 3, 2),
			v097TestText(t, "C | D", "C | D", 3, 2), v097TestClose(t, 3, 2)}, nil},
		{"nested content excluded", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestOpen(t, "note", 3, 2),
			v097TestText(t, "C | D | E", "C | D | E", 5, 3), v097TestClose(t, 5, 3),
			v097TestText(t, "F | G", "F | G", 3, 2), v097TestClose(t, 3, 2)}, nil},
		{"inline declaration excluded", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestTag(t, "note", "content", "C | D | E", 3, 2),
			v097TestText(t, "F | G", "F | G", 3, 2), v097TestClose(t, 3, 2)}, nil},
		{"independent tables", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, "C | D", "C | D", 3, 2), v097TestClose(t, 3, 2),
			v097TestOpen(t, "table", 2, 1), v097TestText(t, "A | B | C", "A | B | C", 7, 2),
			v097TestText(t, "D | E | F", "D | E | F", 7, 2), v097TestClose(t, 7, 2)}, nil},
		{"uppercase table", []model.Line{v097TestOpen(t, "TABLE", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, "C", "C", 3, 2), v097TestClose(t, 3, 2)},
			[][6]int{{5, 2, 4, 6, 1, 2}}},
		{"whole reference range", []model.Line{v097TestOpen(t, "table", 2, 1),
			v097TestText(t, " \tЯ | 😀 \t", "Я | 😀", 3, 2), v097TestText(t, "C", "C", 3, 2), v097TestClose(t, 3, 2)},
			[][6]int{{5, 2, 4, 10, 1, 2}}},
		{"no table", []model.Line{v097TestText(t, "A | B", "A | B", 2, 1), v097TestText(t, "C", "C", 2, 1)}, nil},
		{"empty table", []model.Line{v097TestOpen(t, "table", 2, 1), v097TestClose(t, 3, 2)}, nil},
		{"one row", []model.Line{v097TestOpen(t, "table", 2, 1), v097TestText(t, "A|B", "A|B", 3, 2), v097TestClose(t, 3, 2)}, nil},
		{"all invalid", []model.Line{v097TestOpen(t, "table", 2, 1), v097TestBroken(t, "<a>", 3, 2),
			v097TestBroken(t, "<x|y", 3, 2), v097TestClose(t, 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v097TestRun(t, v097TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV097RawAndElements(t *testing.T) {
	for _, fragmented := range []bool{false, true} {
		t.Run("fragmented="+strconv.FormatBool(fragmented), func(t *testing.T) {
			row := v097TestText(t, "A | <x|y> | Z", "A | <x|y> | Z", 3, 2)
			if fragmented {
				row.Elements = []model.Element{
					v097TestElement(t, "content", "A", "A", 1, 2),
					v097TestElement(t, "content", "x", "x", 6, 7),
					v097TestElement(t, "content", "y", "y", 8, 9),
					v097TestElement(t, "content", "Z", "Z", 13, 14),
				}
			}
			v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
				v097TestText(t, "A | B", "A | B", 3, 2), row, v097TestClose(t, 3, 2)),
				"current", [][6]int{{5, 14, 4, 6, 3, 2}})
		})
	}
	t.Run("malformed split without unparsed", func(t *testing.T) {
		// Разбор выделил текст, но дефект ячейки должен установить сам V097.
		v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, "<a>", "<a>", 3, 2),
			v097TestText(t, "C", "C", 3, 2), v097TestClose(t, 3, 2)), "current", [][6]int{{6, 2, 4, 6, 1, 2}})
	})
	t.Run("invalid classification still independently examined", func(t *testing.T) {
		row := v097TestBroken(t, "<a>", 3, 2)
		row.LineType = "invalid"
		v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1), row,
			v097TestText(t, "A|B", "A|B", 3, 2), v097TestText(t, "C", "C", 3, 2),
			v097TestClose(t, 3, 2)), "current", [][6]int{{6, 2, 5, 4, 1, 2}})
	})
	t.Run("invalid declaration is not a data row", func(t *testing.T) {
		row := v097TestBroken(t, "@unknown A|B|C", 3, 2)
		row.LineType = "invalid"
		v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1), row,
			v097TestText(t, "A|B", "A|B", 3, 2), v097TestText(t, "C|D", "C|D", 3, 2),
			v097TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV097NestedAndLiteralBlocks(t *testing.T) {
	t.Run("nested table has its own reference", func(t *testing.T) {
		// Вложенность нарушает отдельное правило DSL, но не смешивает строки таблиц.
		v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A|B", "A|B", 3, 2), v097TestOpen(t, "table", 3, 2),
			v097TestText(t, "C|D|E", "C|D|E", 5, 3), v097TestText(t, "F", "F", 5, 3),
			v097TestClose(t, 5, 3), v097TestText(t, "G", "G", 3, 2), v097TestClose(t, 3, 2)),
			"current", [][6]int{{7, 2, 6, 6, 1, 3}, {9, 2, 4, 4, 1, 2}})
	})
	for _, tag := range []string{"note", "editor", "multifill"} {
		t.Run("rows of "+tag+" are not table data", func(t *testing.T) {
			v097TestRun(t, v097TestDocument(t, v097TestOpen(t, tag, 2, 1),
				v097TestText(t, "A|B", "A|B", 3, 2), v097TestText(t, "C", "C", 3, 2),
				v097TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV097EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
				v097TestText(t, "A | B", "A | B", 3, 2), v097TestText(t, "| C | D |", "| C | D |", 3, 2),
				v097TestClose(t, 3, 2))
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v097TestRun(t, result, "first", [][6]int{{5, 10, 4, 6, 4, 2}})
			v097TestRun(t, result, "second", [][6]int{{5, 10, 4, 6, 4, 2}})
			v097TestRun(t, v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
				v097TestText(t, "A", "A", 3, 2), v097TestText(t, "B", "B", 3, 2), v097TestClose(t, 3, 2)), "separate", nil)
		})
	}
	t.Run("last physical line is data", func(t *testing.T) {
		result := v097TestDocument(t, v097TestOpen(t, "table", 2, 1),
			v097TestText(t, "A|B", "A|B", 3, 2), v097TestText(t, "C", "C", 3, 2))
		// Отсутствие закрытия блока — самостоятельная исходная ошибка.
		result.Document.HasErrors, result.Lines[2].HasErrors = true, true
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
			ID: "unclosed", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line",
			Message: "Блок не закрыт до конца файла.", RelatedLocations: []model.Location{},
			Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 9}},
		})
		v097TestRun(t, result, "current", [][6]int{{5, 2, 4, 4, 1, 2}})
	})
}

func TestV097EmptyAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		for _, known := range []bool{false, true} {
			t.Run(version+" known count="+strconv.FormatBool(known), func(t *testing.T) {
				result := &model.Result{Format: model.FormatVersion1,
					Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
					Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
				if known {
					count := 0
					result.Document.LineCount = &count
				}
				if version == "1.2" {
					v097TestRun(t, result, "current", nil)
					return
				}
				before := v097TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v097TestRegistry(t))
				if err == nil || checks != nil || v097TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

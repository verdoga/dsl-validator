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

// v112TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V112.
func v112TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v112TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v112TestHeading(t, "", "#", "1", title),
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

// v112TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v112TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v112TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v112TestElement(t, "heading-level", marker, level, start, end),
		v112TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v112TestTag собирает однострочный тег с одним явно типизированным параметром.
func v112TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v112TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v112TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v112TestLine(t, raw, "tag", parent, depth, elements...)
}

// v112TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v112TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v112TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v112TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v112TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v112TestClose задаёт закрытие блока с явным родителем.
func v112TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v112TestLine(t, "}", "block-end", parent, depth, v112TestElement(t, "block-close", "}", "}", 1, 2))
}

// v112TestText сохраняет независимо заданные исходное и семантическое значения.
func v112TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v112TestLine(t, raw, "content", parent, depth,
		v112TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v112TestLine не классифицирует исходник и не вычисляет родительские связи.
func v112TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v112TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v112TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v112TestRegistry использует штатный каталог целиком.
func v112TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v112TestRun проверяет новые V112: строка, начало и исключающий конец.
func v112TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v112TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v112TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v112TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V112" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V112 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Тег выделения не имеет пары в своей строке.\nконкретизация: Открытие и закрытие выделения должны находиться в одной текстовой строке" {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V112 metadata", i, entry, location)
		}
		if len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v112TestSnapshot(t, result)), &restored); err != nil {
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
	if v112TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v112TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v112TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV112Registration(t *testing.T) {
	definitions, err := v112TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V112" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Тег выделения не имеет пары в своей строке." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V112 definitions = %d, want 1", count)
	}
}

// v112TestTitled задаёт блок с отдельным элементом названия.
func v112TestTitled(t *testing.T, tag, title string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 3 + utf8.RuneCountInString(title)
	return v112TestLine(t, "@"+tag+" "+title+" {", "block-start", parent, depth,
		v112TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v112TestElement(t, "title", title, title, len(tag)+3, end),
		v112TestElement(t, "block-open", "{", "{", end+1, end+2))
}

func TestV112EmptyAndVersions(t *testing.T) {
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
					v112TestRun(t, result, "current", nil)
					return
				}
				before := v112TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v112TestRegistry(t))
				if err == nil || checks != nil || v112TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

// v112TestBlock окружает строки полным text; версия и заголовок добавляются
// TestDocument. Данные эталона и примера сохранены без изменения содержимого.
func v112TestBlock(t *testing.T, rows ...string) *model.Result {
	t.Helper()
	body := []model.Line{v112TestOpen(t, "text", 2, 1)}
	for _, raw := range rows {
		if raw == "" {
			body = append(body, v112TestLine(t, raw, "blank", 3, 2))
		} else {
			body = append(body, v112TestText(t, raw, raw, 3, 2))
		}
	}
	body = append(body, v112TestClose(t, 3, 2))
	return v112TestDocument(t, body...)
}

func TestV112ReferenceAndExample(t *testing.T) {
	v112TestRun(t, v112TestBlock(t, "<strong>First paragraph.</strong>", "<strong>Second paragraph.</strong>"), "current", nil)
	v112TestRun(t, v112TestBlock(t, "<strong>First paragraph.", "Second paragraph.</strong>"), "current", [][3]int{{4, 1, 9}, {5, 18, 27}})
}

func TestV112Pairing(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"opening", "<em>", [][3]int{{4, 1, 5}}},
		{"closing", "</u>", [][3]int{{4, 1, 5}}},
		{"all names", "<strong>x</strong><em>y</em><u>z</u>", nil},
		{"empty pair", "<u></u>", nil},
		{"same name nested", "<em><em>x</em></em>", nil},
		{"latest opening matched", "<em><em>x</em>", [][3]int{{4, 1, 5}}},
		{"extra closing", "<u>x</u></u>", [][3]int{{4, 9, 13}}},
		{"all openings retained", "<em><em>", [][3]int{{4, 1, 5}, {4, 5, 9}}},
		{"different openings", "<em><u>", [][3]int{{4, 1, 5}, {4, 5, 8}}},
		{"names do not match", "<em>x</u>", [][3]int{{4, 1, 5}, {4, 6, 10}}},
		{"closing before opening", "</em><em>", [][3]int{{4, 1, 6}, {4, 6, 10}}},
		{"crossed names outside scope", "<strong><em>x</strong></em>", nil},
		{"case insensitive", "<STRONG><eM>x</Strong></Em>", nil},
		{"self closed", "<em/>", [][3]int{{4, 1, 6}}},
		{"spaced self closed", "<u />", [][3]int{{4, 1, 6}}},
		{"self closed does not open", "<em/>x</em>", [][3]int{{4, 1, 6}, {4, 7, 12}}},
		{"self closed does not consume opening", "<em><em/></em>", [][3]int{{4, 5, 10}}},
		{"self closed leaves opening", "<u><u/>", [][3]int{{4, 1, 4}, {4, 4, 8}}},
		{"clarification B", "<em <u>x</u>", [][3]int{{4, 1, 8}, {4, 9, 13}}},
		{"clarification B paired by first name", "<em <u>x</em>", nil},
		{"unsupported tags", "<span>x</span><br>", nil},
		{"names with suffix", "<emphasis><u2><strong-name><em:other>", nil},
		{"incomplete marker", "<em", nil},
		{"empty row", "", nil},
		{"ordinary text", "Read this.", nil},
		{"Unicode positions", "Я🙂 <em>", [][3]int{{4, 4, 8}}},
		{"combining character", "e\u0301<u>", [][3]int{{4, 3, 6}}},
		{"indent and trailing whitespace", " \t<em>\t ", [][3]int{{4, 3, 7}}},
		{"inline br does not create a physical row", "<em>x<br>y</em>", nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v112TestRun(t, v112TestBlock(t, tc.raw), "current", tc.want) })
	}
}

func TestV112AttributesAndQuotes(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"attributes do not block pairing", `<em class="x">x</em>`, nil},
		{"attributes on closing", `<em>x</em class="x">`, nil},
		{"unquoted attribute", `<u id=x>x</u>`, nil},
		{"double quoted greater sign", `<em title=">">`, [][3]int{{4, 1, 15}}},
		{"single quoted nested marker", `<em title='<u>'>x</em>`, nil},
		{"opposite quote inside value", `<em title="'>">x</em>`, nil},
		{"unknown tag attributes are not rescanned", `<span title='<em>'>x</span>`, nil},
		{"slash in attribute is not self closing", `<em title="/">x</em>`, nil},
		{"self closing with attributes", `<u id="x"/>`, [][3]int{{4, 1, 12}}},
		{"unclosed attribute", `<em title="x>`, nil},
		{"unclosed attribute hides embedded marker", `<em title="<u>`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v112TestRun(t, v112TestBlock(t, tc.raw), "current", tc.want) })
	}
}

func TestV112EscapesAndRaw(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"both escaped", `\<em>x\</em>`, `<em>x</em>`, nil},
		{"opening escaped", `\<em>x</em>`, `<em>x</em>`, [][3]int{{4, 7, 12}}},
		{"closing escaped", `<em>x\</em>`, `<em>x</em>`, [][3]int{{4, 1, 5}}},
		{"right angle escaped", `<em\>x</em>`, `<em>x</em>`, [][3]int{{4, 7, 12}}},
		{"double slash", `\\<em>`, `\<em>`, [][3]int{{4, 3, 7}}},
		{"triple slash", `\\\<em>`, `\<em>`, nil},
		{"four slashes", `\\\\<em>`, `\\<em>`, [][3]int{{4, 5, 9}}},
		{"escaped right angle in attribute", `<em a=\>>x</em>`, `<em a=>>x</em>`, nil},
		{"quote is not SOURCE escape", `<em a="\">x</em>`, `<em a="\">x</em>`, nil},
		{"HTML entities preserved", `&lt;em&gt;`, `&lt;em&gt;`, nil},
		{"escaped line start is content", `\@note <em>`, `@note <em>`, [][3]int{{4, 8, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1),
				v112TestText(t, tc.raw, tc.value, 3, 2), v112TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV112LineAndBlockBoundaries(t *testing.T) {
	for _, middle := range []string{"", "<br>", "Plain text."} {
		t.Run("middle="+middle, func(t *testing.T) {
			v112TestRun(t, v112TestBlock(t, "<em>", middle, "</em>"), "current", [][3]int{{4, 1, 5}, {6, 1, 6}})
		})
	}
	t.Run("different blocks", func(t *testing.T) {
		v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1), v112TestText(t, "<em>", "<em>", 3, 2), v112TestClose(t, 3, 2),
			v112TestOpen(t, "text", 2, 1), v112TestText(t, "</em>", "</em>", 6, 2), v112TestClose(t, 6, 2)), "current", [][3]int{{4, 1, 5}, {7, 1, 6}})
	})
	t.Run("text nested independently", func(t *testing.T) {
		v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1), v112TestOpen(t, "text", 3, 2),
			v112TestText(t, "<u>", "<u>", 4, 3), v112TestClose(t, 4, 3), v112TestText(t, "</u>", "</u>", 3, 2), v112TestClose(t, 3, 2)),
			"current", [][3]int{{5, 1, 4}, {7, 1, 5}})
	})
	t.Run("text in fragment", func(t *testing.T) {
		fragment := v112TestLine(t, "@fragment read {", "block-start", 2, 1,
			v112TestElement(t, "tag", "@fragment", "fragment", 1, 10), v112TestElement(t, "identifier", "read", "read", 11, 15),
			v112TestElement(t, "block-open", "{", "{", 16, 17))
		v112TestRun(t, v112TestDocument(t, fragment, v112TestOpen(t, "TeXt", 3, 2), v112TestText(t, "<u>", "<u>", 4, 3),
			v112TestClose(t, 4, 3), v112TestClose(t, 3, 2)), "current", [][3]int{{5, 1, 4}})
	})
}

func TestV112ExcludedRows(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		v112TestRun(t, v112TestDocument(t, v112TestTitled(t, "text", "<em>", 2, 1),
			v112TestText(t, "Read.", "Read.", 3, 2), v112TestClose(t, 3, 2)), "current", nil)
	})
	for _, tag := range []string{"note", "script", "editor", "multifill"} {
		t.Run("other block "+tag, func(t *testing.T) {
			v112TestRun(t, v112TestDocument(t, v112TestOpen(t, tag, 2, 1), v112TestText(t, "<em>", "<em>", 3, 2),
				v112TestClose(t, 3, 2)), "current", nil)
		})
		t.Run("nested block "+tag, func(t *testing.T) {
			v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1), v112TestOpen(t, tag, 3, 2),
				v112TestText(t, "<em>", "<em>", 4, 3), v112TestClose(t, 4, 3), v112TestClose(t, 3, 2)), "current", nil)
		})
	}
	t.Run("nested declaration", func(t *testing.T) {
		v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1), v112TestTag(t, "note", "content", "<em>", 3, 2),
			v112TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("outside text", func(t *testing.T) {
		v112TestRun(t, v112TestDocument(t, v112TestText(t, "<em>", "<em>", 2, 1)), "current", nil)
	})
}

// v112TestUnparsed сохраняет null Value; исходную ошибку добавляет TestDocument.
func v112TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v112TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

func TestV112Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"all unparsed", v112TestLine(t, "<em>", "invalid", 3, 2, v112TestUnparsed(t, "<em>", 1, 5)), [][3]int{{4, 1, 5}}},
		{"marker crosses element boundary", v112TestLine(t, "<em>", "content", 3, 2,
			v112TestElement(t, "content", "<e", "<e", 1, 3), v112TestUnparsed(t, "m>", 3, 5)), [][3]int{{4, 1, 5}}},
		{"recognized pair plus error", v112TestLine(t, "<em>x</em> broken", "content", 3, 2,
			v112TestElement(t, "content", "<em>x</em>", "<em>x</em>", 1, 11), v112TestUnparsed(t, "broken", 12, 18)), nil},
		{"erroneous DSL declaration", v112TestLine(t, "@unknown <em>", "invalid", 3, 2,
			v112TestUnparsed(t, "@unknown <em>", 1, 14)), nil},
		{"erroneous heading", v112TestLine(t, "#### <em>", "invalid", 3, 2,
			v112TestUnparsed(t, "#### <em>", 1, 10)), nil},
		{"clarification B in unparsed", v112TestLine(t, "<em <u>x</u>", "invalid", 3, 2,
			v112TestUnparsed(t, "<em <u>x</u>", 1, 13)), [][3]int{{4, 1, 8}, {4, 9, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1), tc.line, v112TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV112EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v112TestBlock(t, "<em>", "</em>")
			for i := 0; i < len(result.Lines)-1; i++ {
				result.Lines[i].LineEnding = ending
			}
			v112TestRun(t, result, "first", [][3]int{{4, 1, 5}, {5, 1, 6}})
			v112TestRun(t, result, "second", [][3]int{{4, 1, 5}, {5, 1, 6}})
			v112TestRun(t, v112TestBlock(t, "<em>x</em>"), "separate", nil)
		})
	}
	t.Run("last physical row without EOL", func(t *testing.T) {
		v112TestRun(t, v112TestDocument(t, v112TestOpen(t, "text", 2, 1), v112TestText(t, "<em>", "<em>", 3, 2)), "current", [][3]int{{4, 1, 5}})
	})
}

func TestV112WholeUnknownMarkers(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"underscore name", "<span_name title='<em>'>TEXT</span>", nil},
		{"unicode space", "<span\u00a0title='<em>'>TEXT</span>", nil},
		{"different unicode space", "<span\u2003title='<u>'>TEXT</span>", nil},
		{"supported name prefix", "<em_name title='<u>'>TEXT</em_name>", nil},
		{"quoted greater sign", "<span_name title='><em>'>TEXT</span_name>", nil},
		{"double quoted marker", `<span_name title="> <em>">TEXT</span_name>`, nil},
		{"unfinished attribute", "<span_name title='<em>", nil},
		{"real following marker", "<s_n a='<em>'><u>", [][3]int{{4, 15, 18}}},
		{"real preceding marker", "<u><s_n a='</u>'>", [][3]int{{4, 1, 4}}},
		{"known name unicode boundary", "<em\u00a0title='<u>'>TEXT</em>", nil},
		{"unpaired known name unicode boundary", "<em\u00a0x>", [][3]int{{4, 1, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v112TestRun(t, v112TestBlock(t, tc.raw), "current", tc.want)
		})
	}
}

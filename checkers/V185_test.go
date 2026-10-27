package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v185TestMessage = "Внутри @ordering использован запрещённый вложенный тег."

// v185TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v185TestWant struct {
	location, related [3]int
	detail            string
}

func TestV185RegistrationAndVersions(t *testing.T) {
	registry := v185TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V185" {
			continue
		}
		count++
		if definition.Message != v185TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V185 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v185TestDocument(t)
			result.Document.DSLVersion = &version
			before := v185TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v185TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v185TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v185TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v185TestOpen(t, name, "", "", 2)}, body...)
	return v185TestDocument(t, append(lines, v185TestClose(t, 3))...)
}

// v185TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v185TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v185TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v185TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v185TestTag задаёт известное имя и явно указанный тип аргумента.
func v185TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v185TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v185TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v185TestLine(t, raw, "tag", parent, elements...)
}

// v185TestOpen добавляет открытие к известному объявлению блока.
func v185TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v185TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v185TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v185TestClose задаёт закрытие конкретного фигурного блока.
func v185TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v185TestLine(t, "}", "block-end", parent, v185TestElement(t, "block-close", "}", "}", 1))
}

// v185TestText сохраняет исходное и семантическое содержимое независимо.
func v185TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v185TestLine(t, raw, "content", parent, v185TestElement(t, "content", raw, value, 1))
}

// v185TestDocument добавляет версию и заголовок документа; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v185TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v185TestTag(t, "dsl-version", "1.2", "version", 0),
		v185TestLine(t, "# Lesson", "heading", 0, v185TestElement(t, "heading-level", "#", "1", 1),
			v185TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			line.Elements[j].ErrorIDs = append([]string{}, line.Elements[j].ErrorIDs...)
			if line.Elements[j].ElementType == "unparsed" {
				v185TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v185TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v185TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	// Все записи здесь принадлежат исходному парсеру. Контракт требует
	// порядка по позиции, даже если ошибки элементов добавлялись позднее.
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v185TestRegistry использует полный штатный каталог без отдельной регистрации.
func v185TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v185TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v185TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v185TestRun сравнивает новые V185 после полного Prepare/Run с явным ожиданием.
func v185TestRun(t *testing.T, result *model.Result, processingID string, want []v185TestWant) {
	t.Helper()
	before := v185TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v185TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v185TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V185" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V185 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v185TestMessage+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V185 occurrence %d = %+v, want location %+v, related %+v, detail %q", i, entry, location, related, expected.detail)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V185 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V185 created an element error link")
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v185TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Убираются только добавленная история и производные флаги/ссылки.
	// Исходные строки, семантические значения и прежние диагностики сохраняются.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			newIDs := restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("prior element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v185TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

func TestV185ReferenceAndExample(t *testing.T) {
	// Только версия и заголовок добавлены перед неизменёнными фрагментами таблицы.
	for _, tc := range []struct {
		name, instruction, fragment string
		body                        []model.Line
		want                        []v185TestWant
	}{
		{"reference", "Put in order.", "@ordering Put in order. {\nA\nB\n}", nil, nil},
		{"example", "", "@ordering {\n@note Read carefully.\nA\nB\n}",
			[]model.Line{v185TestTag(t, "note", "Read carefully.", "content", 3)},
			[]v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v185TestOpen(t, "ordering", tc.instruction, "content", 2)}, tc.body...)
			body = append(body, v185TestText(t, "A", "A", 3), v185TestText(t, "B", "B", 3), v185TestClose(t, 3))
			result := v185TestDocument(t, body...)
			var raw []string
			for _, line := range result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v185TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV185KnownDeclarations(t *testing.T) {
	// Независимый перечень DSL 1.2. task/endtask покрыты ниже полноценным
	// логическим контейнером. Размещение этих объявлений внутри ordering —
	// намеренное нарушение; параметры и формы объявлений сохраняются.
	for _, tc := range []struct {
		name, argument string
		kind           model.ElementType
		block          bool
		end            int
	}{
		{"dsl-version", "1.2", "version", false, 13},
		{"document-id", "document-1", "identifier", false, 13},
		{"section", "Workbook", "name", false, 9},
		{"order", "1", "number", false, 7},
		{"resource-dir", "Audio", "resource-path", false, 14},
		{"header", "Title", "title", false, 8},
		{"step", "Practice", "title", false, 6},
		{"speaking", "", "", false, 10},
		{"newpage", "", "", false, 9},
		{"editor", "Comment.", "content", false, 8},
		{"media", "", "", false, 7},
		{"example", "Example.", "content", false, 9},
		{"wordlist", "cat; dog", "content", false, 10},
		{"table", "", "", true, 7},
		{"script", "", "", true, 8},
		{"text", "Text.", "content", false, 6},
		{"key", "Key.", "content", false, 5},
		{"instr", "Read.", "content", false, 7},
		{"note", "Read.", "content", false, 6},
		{"alt", "Description.", "content", false, 5},
		{"fragment", "f", "identifier", true, 10},
		{"question", "Where?", "content", false, 10},
		{"multifill", "", "", true, 11},
		{"choice", "", "", true, 8},
		{"multichoice", "", "", true, 13},
		{"matching", "", "", true, 10},
		{"ordering", "", "", true, 10},
		{"variants", "", "", true, 10},
		{"variant", "First", "name", false, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body []model.Line
			if tc.block {
				body = []model.Line{v185TestOpen(t, tc.name, tc.argument, tc.kind, 3)}
				switch tc.name {
				case "matching":
					body = append(body, v185TestText(t, "A", "A", 4), v185TestText(t, "B", "B", 4),
						v185TestLine(t, "---", "separator", 4), v185TestText(t, "First", "First", 4), v185TestText(t, "Last", "Last", 4))
				case "multifill":
					body = append(body, v185TestText(t, "She _____{lives} here.", "She _____{lives} here.", 4))
				case "variants":
					body = append(body, v185TestTag(t, "variant", "First", "name", 4), v185TestTag(t, "note", "One.", "content", 5),
						v185TestTag(t, "variant", "Second", "name", 4), v185TestTag(t, "note", "Two.", "content", 7))
				case "table":
					body = append(body, v185TestText(t, "A | B", "A | B", 4), v185TestText(t, "C | D", "C | D", 4))
				case "script":
					body = append(body, v185TestText(t, "Tom: Hello.", "Tom: Hello.", 4))
				case "fragment":
					body = append(body, v185TestTag(t, "note", "Read.", "content", 4))
				default:
					body = append(body, v185TestText(t, "A", "A", 4), v185TestText(t, "B", "B", 4))
				}
				body = append(body, v185TestClose(t, 4))
			} else if tc.name == "media" {
				body = []model.Line{v185TestLine(t, "@media image ./image.png", "tag", 3,
					v185TestElement(t, "tag", "@media", "media", 1), v185TestElement(t, "media-type", "image", "image", 8),
					v185TestElement(t, "source", "./image.png", "./image.png", 14))}
			} else {
				body = []model.Line{v185TestTag(t, tc.name, tc.argument, tc.kind, 3)}
			}
			body = append(body, v185TestText(t, "A", "A", 3), v185TestText(t, "B", "B", 3))
			v185TestRun(t, v185TestBlockDocument(t, "ordering", body...), "current",
				[]v185TestWant{{[3]int{4, 1, tc.end}, [3]int{3, 1, 10}, "Запрещённый тег: @" + tc.name + "."}})
		})
	}
}

func TestV185RecoveryBoundaries(t *testing.T) {
	for _, raw := range []string{"@note", "@note Text", "@note\tText", "@note{", "@note}", "@NoTe{Text"} {
		t.Run(raw, func(t *testing.T) {
			v185TestRun(t, v185TestBlockDocument(t, "ordering",
				v185TestLine(t, raw, "invalid", 3, v185TestElement(t, "unparsed", raw, "", 1)),
				v185TestText(t, "A", "A", 3), v185TestText(t, "B", "B", 3)), "current",
				[]v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."}})
		})
	}
	for _, raw := range []string{"@noteExtra", "@note2", "@note-extra", "@note!", "@unknown", "@", "@@note",
		"@note\u00a0Text", "@note\u2003Text", "@note\vText", "@note\u2028Text", "\u00a0@note Text"} {
		t.Run("not a known initial name "+strconv.Quote(raw), func(t *testing.T) {
			v185TestRun(t, v185TestBlockDocument(t, "ordering", v185TestLine(t, raw, "invalid", 3,
				v185TestElement(t, "unparsed", raw, "", 1))), "current", nil)
		})
	}
	for _, tc := range []struct {
		name       string
		line       model.Line
		start, end int
	}{
		{"recognized", v185TestTag(t, "note", "Read.", "content", 3), 1, 6},
		{"recognized with unparsed", v185TestLine(t, "@note Read. {", "invalid", 3,
			v185TestElement(t, "tag", "@note", "note", 1), v185TestElement(t, "content", "Read.", "Read.", 7),
			v185TestElement(t, "unparsed", "{", "", 13)), 1, 6},
		{"unparsed on tag row", v185TestLine(t, "@note\tText", "tag", 3,
			v185TestElement(t, "unparsed", "@note\tText", "", 1)), 1, 6},
		{"indented recognized", v185TestLine(t, " \t@NoTe Read.", "tag", 3,
			v185TestElement(t, "tag", "@NoTe", "note", 3), v185TestElement(t, "content", "Read.", "Read.", 9)), 3, 8},
		{"indented recovered", v185TestLine(t, " \t@NoTe\t🌞", "invalid", 3,
			v185TestElement(t, "unparsed", "@NoTe\t🌞", "", 3)), 3, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v185TestRun(t, v185TestBlockDocument(t, "ordering", tc.line), "current",
				[]v185TestWant{{[3]int{4, tc.start, tc.end}, [3]int{3, 1, 10}, "Запрещённый тег: @note."}})
		})
	}
}

func TestV185LiteralAndExcludedDeclarations(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\@note Read.`, "@note Read."}, {` \@note Read.`, " @note Read."},
		{`\\@note Read.`, `\@note Read.`}, {"Text @note Read.", "Text @note Read."},
		{"🌞 @note Read.", "🌞 @note Read."}, {"", ""}, {" \t", " \t"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			line := v185TestText(t, tc.raw, tc.value, 3)
			if strings.Trim(tc.raw, " \t") == "" {
				line = v185TestLine(t, tc.raw, "blank", 3)
			}
			v185TestRun(t, v185TestBlockDocument(t, "ordering", line), "current", nil)
		})
	}
	for _, after := range []bool{false, true} {
		for _, block := range []bool{false, true} {
			t.Run("whitelist after="+strconv.FormatBool(after)+" block="+strconv.FormatBool(block), func(t *testing.T) {
				body := []model.Line{v185TestText(t, "A", "A", 3)}
				if after {
					body = append(body, v185TestLine(t, "---", "separator", 3))
				}
				for _, name := range []string{"answer", "hint"} {
					if block {
						parent := len(body) + 4
						body = append(body, v185TestOpen(t, name, "", "", 3), v185TestText(t, "Text.", "Text.", parent), v185TestClose(t, parent))
					} else {
						body = append(body, v185TestTag(t, name, "Text.", "content", 3))
					}
				}
				body = append(body, v185TestTag(t, "include", "f", "identifier", 3), v185TestText(t, "B", "B", 3))
				v185TestRun(t, v185TestBlockDocument(t, "ordering", body...), "current", nil)
			})
		}
	}
}
func TestV185StructuralContexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v185TestWant
	}{
		{"both sides", []model.Line{
			v185TestTag(t, "note", "Read.", "content", 3), v185TestLine(t, "---", "separator", 3),
			v185TestTag(t, "instr", "Read.", "content", 3)},
			[]v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
				{[3]int{6, 1, 7}, [3]int{3, 1, 10}, "Запрещённый тег: @instr."}}},
		{"multiple separators", []model.Line{v185TestLine(t, "---", "separator", 3), v185TestLine(t, "---", "invalid", 3,
			v185TestElement(t, "unparsed", "---", "", 1)), v185TestTag(t, "editor", "Comment.", "content", 3)},
			[]v185TestWant{{[3]int{6, 1, 8}, [3]int{3, 1, 10}, "Запрещённый тег: @editor."}}},
		{"forbidden block is one declaration", []model.Line{v185TestOpen(t, "note", "", "", 3),
			v185TestText(t, "Read.", "Read.", 4), v185TestText(t, "Think.", "Think.", 4), v185TestClose(t, 4)},
			[]v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."}}},
		{"nearest brace parent", []model.Line{v185TestOpen(t, "note", "", "", 3),
			v185TestTag(t, "instr", "Read.", "content", 4), v185TestClose(t, 4)},
			[]v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."}}},
		{"nested ordering owns its declarations", []model.Line{v185TestOpen(t, "ordering", "", "", 3),
			v185TestTag(t, "note", "Read.", "content", 4), v185TestText(t, "A", "A", 4), v185TestText(t, "B", "B", 4), v185TestClose(t, 4)},
			[]v185TestWant{{[3]int{4, 1, 10}, [3]int{3, 1, 10}, "Запрещённый тег: @ordering."},
				{[3]int{5, 1, 6}, [3]int{4, 1, 10}, "Запрещённый тег: @note."}}},
		{"logical parents are not curly containers", []model.Line{v185TestTag(t, "task", "practice", "identifier", 3),
			v185TestTag(t, "note", "Read.", "content", 4), v185TestTag(t, "endtask", "", "", 4)},
			[]v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @task."},
				{[3]int{5, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
				{[3]int{6, 1, 9}, [3]int{3, 1, 10}, "Запрещённый тег: @endtask."}}},
		{"no scan into answer", []model.Line{v185TestText(t, "A", "A", 3), v185TestOpen(t, "answer", "", "", 3),
			v185TestTag(t, "note", "Read.", "content", 5), v185TestClose(t, 5), v185TestText(t, "B", "B", 3)}, nil},
		{"no scan into hint", []model.Line{v185TestText(t, "A", "A", 3), v185TestTag(t, "answer", "2", "content", 3),
			v185TestOpen(t, "hint", "", "", 3), v185TestTag(t, "note", "Read.", "content", 6), v185TestClose(t, 6), v185TestText(t, "B", "B", 3)}, nil},
		{"literal block body", []model.Line{v185TestOpen(t, "editor", "", "", 3),
			v185TestText(t, "@note Read.", "@note Read.", 4), v185TestText(t, "@ordering {", "@ordering {", 4), v185TestClose(t, 4)},
			[]v185TestWant{{[3]int{4, 1, 8}, [3]int{3, 1, 10}, "Запрещённый тег: @editor."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v185TestRun(t, v185TestBlockDocument(t, "ordering", tc.body...), "current", tc.want)
		})
	}
	t.Run("depth and parent case", func(t *testing.T) {
		result := v185TestDocument(t, v185TestOpen(t, "fragment", "f", "identifier", 2),
			v185TestLine(t, " \t@OrDeRiNg {", "block-start", 3,
				v185TestElement(t, "tag", "@OrDeRiNg", "ordering", 3), v185TestElement(t, "block-open", "{", "{", 13)),
			v185TestTag(t, "NoTe", "Read.", "content", 4), v185TestText(t, "A", "A", 4), v185TestText(t, "B", "B", 4),
			v185TestClose(t, 4), v185TestClose(t, 3), v185TestTag(t, "include", "f", "identifier", 2))
		v185TestRun(t, result, "current", []v185TestWant{{[3]int{5, 1, 6}, [3]int{4, 3, 12}, "Запрещённый тег: @note."}})
	})
	t.Run("independent neighboring blocks", func(t *testing.T) {
		result := v185TestDocument(t, v185TestOpen(t, "ordering", "", "", 2), v185TestTag(t, "note", "One.", "content", 3),
			v185TestText(t, "A", "A", 3), v185TestText(t, "B", "B", 3), v185TestClose(t, 3),
			v185TestOpen(t, "ordering", "", "", 2), v185TestText(t, "A", "A", 8), v185TestText(t, "B", "B", 8),
			v185TestTag(t, "note", "Two.", "content", 8), v185TestClose(t, 8))
		v185TestRun(t, result, "current", []v185TestWant{
			{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
			{[3]int{11, 1, 6}, [3]int{8, 1, 10}, "Запрещённый тег: @note."}})
	})
	for _, name := range []string{"note", "answer", "hint", "matching", "choice", "multichoice"} {
		t.Run("other container "+name, func(t *testing.T) {
			v185TestRun(t, v185TestBlockDocument(t, name, v185TestTag(t, "note", "Read.", "content", 3)), "current", nil)
		})
	}
	t.Run("outside ordering", func(t *testing.T) {
		v185TestRun(t, v185TestDocument(t, v185TestTag(t, "note", "Read.", "content", 2)), "current", nil)
	})
	t.Run("nonblock ordering is not a curly parent", func(t *testing.T) {
		v185TestRun(t, v185TestDocument(t, v185TestTag(t, "ordering", "Order.", "content", 2),
			v185TestTag(t, "note", "Read.", "content", 2)), "current", nil)
	})
}

func TestV185EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v185TestBlockDocument(t, "ordering", v185TestTag(t, "note", "Read 🌞.", "content", 3),
				v185TestText(t, "A", "A", 3), v185TestText(t, "B", "B", 3), v185TestTag(t, "note", "Read 🌞.", "content", 3))
			if ending == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ending
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v185TestWant{{[3]int{4, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
				{[3]int{7, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."}}
			v185TestRun(t, result, "first", want)
			v185TestRun(t, result, "second", want)
			v185TestRun(t, v185TestBlockDocument(t, "ordering", v185TestText(t, "A", "A", 3), v185TestText(t, "B", "B", 3)), "other", nil)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v185TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v185TestRun(t, result, "current", nil)
	})
}

func TestV185PhysicalHeadingContinuation(t *testing.T) {
	definitions, err := v185TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(definitions, func(d diagnostics.Definition) bool { return d.Code == "V185" }) {
		t.Fatal("V185 is missing from the full catalog")
	}
	for _, heading := range []bool{false, true} {
		t.Run("root heading="+strconv.FormatBool(heading), func(t *testing.T) {
			middle, parent := v185TestLine(t, "", "blank", 3), 3
			if heading {
				middle, parent = v185TestRootHeading(t), 4
			}
			// Пара heading/blank-control сохраняет исходный пример аудита.
			result := v185TestDocument(t,
				v185TestOpen(t, "ordering", "", "", 2), middle,
				v185TestTag(t, "note", "Forbidden", "content", parent),
				v185TestText(t, "One", "One", parent),
				v185TestText(t, "Two", "Two", parent),
				v185TestLine(t, "---", "separator", parent),
				v185TestText(t, "A", "A", parent),
				v185TestText(t, "B", "B", parent), v185TestClose(t, 3))
			v185TestRun(t, result, "current", []v185TestWant{
				{[3]int{5, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
			})
		})
	}
}

func TestV185PhysicalRecoveryAndAllowedTags(t *testing.T) {
	result := v185TestDocument(t,
		v185TestOpen(t, "ordering", "", "", 2),
		v185TestRootHeading(t),
		v185TestTag(t, "answer", "1", "content", 4),
		v185TestTag(t, "hint", "Hint", "content", 4),
		v185TestTag(t, "include", "f", "identifier", 4),
		v185TestLine(t, "\t@NoTe\tBefore", "invalid", 4,
			v185TestElement(t, "unparsed", "@NoTe\tBefore", "", 2)),
		v185TestLine(t, "---", "separator", 4),
		v185TestRootHeading(t),
		v185TestLine(t, "@InStR After", "content", 10,
			v185TestElement(t, "unparsed", "@InStR After", "", 1)),
		v185TestTag(t, "editor", "Comment", "content", 10),
		v185TestTag(t, "answer", "1", "content", 10),
		v185TestTag(t, "hint", "Hint", "content", 10),
		v185TestTag(t, "include", "f", "identifier", 10),
		v185TestClose(t, 3))
	v185TestRun(t, result, "current", []v185TestWant{
		{[3]int{8, 2, 7}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
		{[3]int{11, 1, 7}, [3]int{3, 1, 10}, "Запрещённый тег: @instr."},
		{[3]int{12, 1, 8}, [3]int{3, 1, 10}, "Запрещённый тег: @editor."},
	})
}

func TestV185PhysicalNestedBoundaries(t *testing.T) {
	result := v185TestDocument(t,
		v185TestOpen(t, "ordering", "", "", 2),
		v185TestRootHeading(t),
		v185TestOpen(t, "note", "", "", 4),
		v185TestRootHeading(t),
		v185TestTag(t, "instr", "Nested", "content", 6),
		v185TestClose(t, 5),
		v185TestOpen(t, "ordering", "", "", 6),
		v185TestRootHeading(t),
		v185TestTag(t, "note", "Inner", "content", 10),
		v185TestClose(t, 9),
		v185TestTag(t, "instr", "Outer", "content", 10),
		v185TestClose(t, 3),
		v185TestTag(t, "note", "Outside", "content", 10))
	v185TestRun(t, result, "current", []v185TestWant{
		{[3]int{5, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
		{[3]int{9, 1, 10}, [3]int{3, 1, 10}, "Запрещённый тег: @ordering."},
		{[3]int{11, 1, 6}, [3]int{9, 1, 10}, "Запрещённый тег: @note."},
		{[3]int{13, 1, 7}, [3]int{3, 1, 10}, "Запрещённый тег: @instr."},
	})
}

func TestV185PhysicalLiteralBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, openingRaw, closingValue, detail string
		end                                    int
	}{
		{"editor", `@ordering \{`, "}", "Запрещённый тег: @editor.", 8},
		{"multifill", `@ordering \{`, "}", "Запрещённый тег: @multifill.", 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v185TestDocument(t,
				v185TestOpen(t, "ordering", "", "", 2),
				v185TestRootHeading(t),
				v185TestOpen(t, tc.name, "", "", 4),
				v185TestText(t, tc.openingRaw, "@ordering {", 4),
				v185TestLine(t, "@note Fake", "invalid", 4,
					v185TestElement(t, "unparsed", "@note Fake", "", 1)),
				v185TestText(t, `\}`, tc.closingValue, 4),
				v185TestLine(t, "@instr Fake", "content", 4,
					v185TestElement(t, "unparsed", "@instr Fake", "", 1)),
				v185TestLine(t, " \t}\t ", "invalid", 4,
					v185TestElement(t, "unparsed", " \t}\t ", "", 1)),
				v185TestTag(t, "note", "Real", "content", 4),
				v185TestClose(t, 3),
				v185TestTag(t, "note", "Outside", "content", 4))
			v185TestRun(t, result, "current", []v185TestWant{
				{[3]int{5, 1, tc.end}, [3]int{3, 1, 10}, tc.detail},
				{[3]int{11, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
			})
		})
	}
}

func TestV185PhysicalPartialBoundaries(t *testing.T) {
	answer := v185TestOpen(t, "answer", "", "", 4)
	answer.Raw += " ?"
	answer.LineType = "invalid"
	answer.Elements = append(answer.Elements, v185TestElement(t, "unparsed", "?", "", 11))
	ordering := v185TestOpen(t, "ordering", "", "", 4)
	ordering.Raw += " ?"
	ordering.LineType = "invalid"
	ordering.Elements = append(ordering.Elements, v185TestElement(t, "unparsed", "?", "", 13))
	result := v185TestDocument(t,
		v185TestOpen(t, "ordering", "", "", 2),
		v185TestRootHeading(t), answer,
		v185TestTag(t, "note", "Nested", "content", 4),
		v185TestLine(t, "} ?", "invalid", 4,
			v185TestElement(t, "block-close", "}", "}", 1),
			v185TestElement(t, "unparsed", "?", "", 3)),
		v185TestTag(t, "instr", "Outer", "content", 4), ordering,
		v185TestRootHeading(t),
		v185TestTag(t, "note", "Inner", "content", 10),
		v185TestLine(t, "}", "invalid", 10, v185TestElement(t, "unparsed", "}", "", 1)),
		v185TestTag(t, "note", "Outer again", "content", 10),
		v185TestClose(t, 3),
		v185TestTag(t, "note", "Outside", "content", 10))
	v185TestRun(t, result, "current", []v185TestWant{
		{[3]int{8, 1, 7}, [3]int{3, 1, 10}, "Запрещённый тег: @instr."},
		{[3]int{9, 1, 10}, [3]int{3, 1, 10}, "Запрещённый тег: @ordering."},
		{[3]int{11, 1, 6}, [3]int{9, 1, 10}, "Запрещённый тег: @note."},
		{[3]int{13, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
	})
}

func TestV185PhysicalUnknownOpening(t *testing.T) {
	result := v185TestDocument(t,
		v185TestOpen(t, "ordering", "", "", 2),
		v185TestRootHeading(t),
		v185TestLine(t, "@unknown {", "invalid", 4,
			v185TestElement(t, "unparsed", "@unknown", "", 1),
			v185TestElement(t, "block-open", "{", "{", 10)),
		v185TestTag(t, "note", "Nested", "content", 4),
		v185TestLine(t, "}", "invalid", 4, v185TestElement(t, "unparsed", "}", "", 1)),
		v185TestTag(t, "note", "Outer", "content", 4), v185TestClose(t, 3))
	v185TestRun(t, result, "current", []v185TestWant{
		{[3]int{8, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
	})
}

func TestV185PhysicalClosingAndAdjacentBlocks(t *testing.T) {
	for _, closing := range []string{"parsed", "unparsed", "partial"} {
		t.Run(closing, func(t *testing.T) {
			end := v185TestClose(t, 3)
			if closing == "unparsed" {
				end = v185TestLine(t, "}", "invalid", 4, v185TestElement(t, "unparsed", "}", "", 1))
			}
			if closing == "partial" {
				end = v185TestLine(t, "} ?", "invalid", 4,
					v185TestElement(t, "block-close", "}", "}", 1),
					v185TestElement(t, "unparsed", "?", "", 3))
			}
			result := v185TestDocument(t,
				v185TestOpen(t, "ordering", "", "", 2),
				v185TestRootHeading(t),
				v185TestTag(t, "note", "First", "content", 4),
				v185TestText(t, `\}`, "}", 4),
				v185TestTag(t, "instr", "Still inside", "content", 4), end,
				v185TestTag(t, "note", "Outside", "content", 4),
				v185TestOpen(t, "ordering", "", "", 4),
				v185TestRootHeading(t),
				v185TestTag(t, "note", "Second", "content", 11),
				v185TestClose(t, 10),
				v185TestTag(t, "note", "Outside again", "content", 11))
			v185TestRun(t, result, "current", []v185TestWant{
				{[3]int{5, 1, 6}, [3]int{3, 1, 10}, "Запрещённый тег: @note."},
				{[3]int{7, 1, 7}, [3]int{3, 1, 10}, "Запрещённый тег: @instr."},
				{[3]int{12, 1, 6}, [3]int{10, 1, 10}, "Запрещённый тег: @note."},
			})
		})
	}
}

// v185TestRootHeading задаёт heading с обязательными nil ParentLine и глубиной 0.
func v185TestRootHeading(t *testing.T) model.Line {
	t.Helper()
	return v185TestLine(t, "### Inside", "heading", 0,
		v185TestElement(t, "heading-level", "###", "3", 1),
		v185TestElement(t, "title", "Inside", "Inside", 5))
}

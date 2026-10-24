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

// v076Expected задаёт независимо выписанные диапазоны и конкретизацию.
// Первые четыре координаты — основной диапазон, следующие — связанный.
type v076Expected struct {
	position [8]int
	detail   string
}

const v076HintInNote = "Вложенный тег: @hint. Родительский блок: @note."

// v076TestFragment задаёт определение с явно указанным ID.
func v076TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v076TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
		v076TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v076TestElement(t, "identifier", id, id, 11, end),
		v076TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v076TestUnparsed задаёт неразобранную часть с отсутствующим Value.
// Документ добавляет требуемую исходную ошибку и ссылку на неё.
func v076TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v076TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v076TestBroken добавляет явно ошибочный остаток к распознанной строке.
func v076TestBroken(t *testing.T, line model.Line, extra string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " " + extra
	line.Elements = append(line.Elements, v076TestUnparsed(t, extra, start, start+utf8.RuneCountInString(extra)))
	return line
}

// v076TestParserError сохраняет ошибку размещения распознанной конструкции,
// не превращая её в unparsed. Это исходные данные, не ожидание результата V076.
func v076TestParserError(t *testing.T, result *model.Result, number int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "line",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: 1},
			End: model.Position{Line: number, Column: utf8.RuneCountInString(line.Raw) + 1}},
	})
	line.HasErrors, result.Document.HasErrors = true, true
}

// v076TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V076.
func v076TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v076TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v076TestHeading(t, "", "#", "1", title),
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

// v076TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v076TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v076TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v076TestElement(t, "heading-level", marker, level, start, end),
		v076TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v076TestInvalid задаёт ошибочную строку без распознанного tag.
func v076TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v076TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v076TestLine(t, raw, "invalid", parent, depth, element)
}

// v076TestTag собирает однострочный тег с одним явно типизированным параметром.
func v076TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v076TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v076TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v076TestLine(t, raw, "tag", parent, depth, elements...)
}

// v076TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v076TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v076TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v076TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v076TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v076TestClose задаёт закрытие блока с явным родителем.
func v076TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v076TestLine(t, "}", "block-end", parent, depth, v076TestElement(t, "block-close", "}", "}", 1, 2))
}

// v076TestText сохраняет независимо заданные исходное и семантическое значения.
func v076TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v076TestLine(t, raw, "content", parent, depth,
		v076TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v076TestLine не классифицирует исходник и не вычисляет родительские связи.
func v076TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v076TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v076TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v076TestRegistry использует штатный каталог целиком.
func v076TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v076TestRun проверяет только новые V076 по явно заданным диапазонам:
// первые четыре числа задают основной диапазон, следующие четыре — связанный.
// Нули связанного диапазона означают его отсутствие. Концы исключаются.
func v076TestRun(t *testing.T, result *model.Result, processingID string, want []v076Expected) {
	t.Helper()
	before := v076TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v076TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v076TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V076" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V076 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		position := expected.position
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[2], Column: position[3]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil ||
			entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Вложенный тег запрещён в этом блоке.\nконкретизация: "+expected.detail {
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V076 metadata", i, entry, location)
		}
		if position[4] == 0 {
			if len(entry.RelatedLocations) != 0 {
				t.Fatalf("unexpected related locations: %+v", entry.RelatedLocations)
			}
		} else {
			related := model.Location{Start: model.Position{Line: position[4], Column: position[5]},
				End: model.Position{Line: position[6], Column: position[7]}}
			if len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
				t.Fatalf("related locations = %+v, want exactly %+v", entry.RelatedLocations, related)
			}
		}

	}
	// CheckModel проверяет уникальность ID, флаги и ссылки всех диагностик.
	// В копии восстанавливаем лишь производные поля для сравнения исходника.
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v076TestSnapshot(t, result)), &restored); err != nil {
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
	if v076TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v076TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v076TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV076Registration(t *testing.T) {
	definitions, err := v076TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V076" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "Вложенный тег запрещён в этом блоке." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) || definition.Check == nil {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V076 definitions = %d, want 1", count)
	}
}

func TestV076ReferenceAndExample(t *testing.T) {
	// Версия и # Lesson дополняют фрагменты по DSL 1.3, 1.30 и 2.1.3.
	// Сам текст фрагментов, включая пробелы и переводы строк, сохранён.
	for _, tc := range []struct {
		name, parent, fragment string
		want                   []v076Expected
	}{
		{"reference", "instr", "@instr {\nRead carefully.\n@hint Look at the example.\n}", nil},
		{"example", "note", "@note {\nRead carefully.\n@hint Look at the example.\n}",
			[]v076Expected{{[8]int{5, 1, 5, 6, 3, 1, 3, 6}, v076HintInNote}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v076TestDocument(t, v076TestOpen(t, tc.parent, 2, 1),
				v076TestText(t, "Read carefully.", "Read carefully.", 3, 2),
				v076TestTag(t, "hint", "content", "Look at the example.", 3, 2), v076TestClose(t, 3, 2))
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v076TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV076Parents(t *testing.T) {
	for _, tc := range []struct {
		parent string
		end    int
		detail string
	}{
		{"example", 9, "Вложенный тег: @hint. Родительский блок: @example."},
		{"wordlist", 10, "Вложенный тег: @hint. Родительский блок: @wordlist."},
		{"table", 7, "Вложенный тег: @hint. Родительский блок: @table."},
		{"script", 8, "Вложенный тег: @hint. Родительский блок: @script."},
		{"text", 6, "Вложенный тег: @hint. Родительский блок: @text."},
		{"key", 5, "Вложенный тег: @hint. Родительский блок: @key."},
		{"note", 6, v076HintInNote},
		{"alt", 5, "Вложенный тег: @hint. Родительский блок: @alt."},
		{"hint", 6, "Вложенный тег: @hint. Родительский блок: @hint."},
	} {
		t.Run(tc.parent, func(t *testing.T) {
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, tc.parent, 2, 1),
				v076TestText(t, "Read.", "Read.", 3, 2), v076TestTag(t, "hint", "content", "Help.", 3, 2), v076TestClose(t, 3, 2)),
				"current", []v076Expected{{[8]int{5, 1, 5, 6, 3, 1, 3, tc.end}, tc.detail}})
		})
	}
}

func TestV076NearestBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v076Expected
	}{
		{"block hint in instr", []model.Line{
			v076TestOpen(t, "instr", 2, 1), v076TestText(t, "Read.", "Read.", 3, 2),
			v076TestOpen(t, "hint", 3, 2), v076TestText(t, "Help.", "Help.", 5, 3), v076TestClose(t, 5, 3), v076TestClose(t, 3, 2),
		}, nil},
		{"only hint exception", []model.Line{
			v076TestOpen(t, "instr", 2, 1), v076TestText(t, "Read.", "Read.", 3, 2),
			v076TestTag(t, "note", "content", "Help.", 3, 2), v076TestClose(t, 3, 2),
		}, []v076Expected{{[8]int{5, 1, 5, 6, 3, 1, 3, 7}, "Вложенный тег: @note. Родительский блок: @instr."}}},
		{"table in table", []model.Line{
			v076TestOpen(t, "table", 2, 1), v076TestOpen(t, "table", 3, 2),
			v076TestText(t, "One | Two", "One | Two", 4, 3), v076TestClose(t, 4, 3), v076TestClose(t, 3, 2),
		}, []v076Expected{{[8]int{4, 1, 4, 7, 3, 1, 3, 7}, "Вложенный тег: @table. Родительский блок: @table."}}},
		{"each declaration once", []model.Line{
			v076TestOpen(t, "note", 2, 1), v076TestOpen(t, "hint", 3, 2),
			v076TestTag(t, "note", "content", "Text.", 4, 3), v076TestClose(t, 4, 3), v076TestClose(t, 3, 2),
		}, []v076Expected{
			{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote},
			{[8]int{5, 1, 5, 6, 4, 1, 4, 6}, "Вложенный тег: @note. Родительский блок: @hint."},
		}},
		{"exception ignores distant note", []model.Line{
			v076TestOpen(t, "note", 2, 1), v076TestOpen(t, "instr", 3, 2),
			v076TestText(t, "Read.", "Read.", 4, 3), v076TestTag(t, "hint", "content", "Help.", 4, 3),
			v076TestClose(t, 4, 3), v076TestClose(t, 3, 2),
		}, []v076Expected{{[8]int{4, 1, 4, 7, 3, 1, 3, 6}, "Вложенный тег: @instr. Родительский блок: @note."}}},
		{"hint in hint is not instr exception", []model.Line{
			v076TestOpen(t, "instr", 2, 1), v076TestText(t, "Read.", "Read.", 3, 2),
			v076TestOpen(t, "hint", 3, 2), v076TestTag(t, "hint", "content", "Help.", 5, 3),
			v076TestClose(t, 5, 3), v076TestClose(t, 3, 2),
		}, []v076Expected{{[8]int{6, 1, 6, 6, 5, 1, 5, 6}, "Вложенный тег: @hint. Родительский блок: @hint."}}},
		{"non-block parent does not conceal note", []model.Line{
			v076TestOpen(t, "note", 2, 1), v076TestTag(t, "task", "identifier", "practice", 3, 2),
			v076TestTag(t, "hint", "content", "Help.", 4, 3), v076TestTag(t, "endtask", "", "", 3, 2), v076TestClose(t, 3, 2),
		}, []v076Expected{
			{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, "Вложенный тег: @task. Родительский блок: @note."},
			{[8]int{5, 1, 5, 6, 3, 1, 3, 6}, v076HintInNote},
			{[8]int{6, 1, 6, 9, 3, 1, 3, 6}, "Вложенный тег: @endtask. Родительский блок: @note."},
		}},
		{"unknown nearest block does not expose distant note", []model.Line{
			v076TestOpen(t, "note", 2, 1),
			v076TestLine(t, "@unknown {", "block-start", 3, 2,
				v076TestUnparsed(t, "@unknown", 1, 9), v076TestElement(t, "block-open", "{", "{", 10, 11)),
			v076TestTag(t, "hint", "content", "Help.", 4, 3), v076TestClose(t, 4, 3), v076TestClose(t, 3, 2),
		}, nil},
		{"after closing block", []model.Line{
			v076TestOpen(t, "note", 2, 1), v076TestTag(t, "hint", "content", "Inside.", 3, 2),
			v076TestClose(t, 3, 2), v076TestTag(t, "hint", "content", "Outside.", 2, 1),
		}, []v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}}},
		{"several siblings", []model.Line{
			v076TestOpen(t, "note", 2, 1), v076TestTag(t, "hint", "content", "First.", 3, 2),
			v076TestLine(t, "", "blank", 3, 2), v076TestText(t, "Read.", "Read.", 3, 2),
			v076TestTag(t, "hint", "content", "Second.", 3, 2), v076TestClose(t, 3, 2),
		}, []v076Expected{
			{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}, {[8]int{7, 1, 7, 6, 3, 1, 3, 6}, v076HintInNote},
		}},
		{"no block", []model.Line{
			v076TestTag(t, "note", "content", "Read.", 2, 1), v076TestTag(t, "hint", "content", "Help.", 2, 1),
		}, nil},
		{"allowed fragment container", []model.Line{
			v076TestFragment(t, "f", 2, 1), v076TestTag(t, "hint", "content", "Help.", 3, 2), v076TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v076TestRun(t, v076TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV076LiteralContent(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		end    int
		detail string
	}{
		{"editor", 8, "Вложенный тег: @editor. Родительский блок: @note."},
		{"multifill", 11, "Вложенный тег: @multifill. Родительский блок: @note."},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			// Текст внутри литерального блока не объявляет вложенные блоки.
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, tc.tag, 2, 1),
				v076TestText(t, "@note {", "@note {", 3, 2), v076TestText(t, "@hint Help.", "@hint Help.", 3, 2),
				v076TestClose(t, 3, 2)), "literal", nil)
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1), v076TestOpen(t, tc.tag, 3, 2),
				v076TestText(t, "@note {", "@note {", 4, 3), v076TestText(t, "@hint Help.", "@hint Help.", 4, 3),
				v076TestClose(t, 4, 3), v076TestClose(t, 3, 2)), "nested",
				[]v076Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail}})
		})
	}
	// Даже ошибочная строка @answer в multifill (DSL 4.4.18) не относится к V076.
	v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "multifill", 2, 1),
		v076TestInvalid(t, "@answer Yes", 3, 2), v076TestClose(t, 3, 2)), "answer", nil)
	for _, tc := range []struct{ raw, value string }{
		{`\@hint Help.`, "@hint Help."},
		{"Read @hint Help.", "Read @hint Help."},
		{"<strong>Read</strong><em>@hint</em><u>Help.</u>", "<strong>Read</strong><em>@hint</em><u>Help.</u>"},
		{"<br>", "<br>"},
		{"&#64;hint Help.", "&#64;hint Help."},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "text", 2, 1),
				v076TestText(t, tc.raw, tc.value, 3, 2), v076TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV076Recovery(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		start, end int
		want       bool
	}{
		{"@hint", 1, 6, true}, {"@hint Help.", 1, 6, true}, {"@hint\tHelp.", 1, 6, true},
		{" \t@HiNt", 3, 8, true}, {"@hint ", 1, 6, true}, {"@hint\t", 1, 6, true},
		{"@hintExtra", 0, 0, false}, {"@hint\u00a0Help.", 0, 0, false},
		{"@hint}", 0, 0, false}, {"@hint!", 0, 0, false}, {"@unknown", 0, 0, false},
		{"@", 0, 0, false}, {"@{", 0, 0, false}, {"Read @hint", 0, 0, false},
		{`\@hint`, 0, 0, false}, {"\u00a0@hint", 0, 0, false}, {"\u2003@hint", 0, 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			// Запрещённое вложение может остаться полностью неразобранным.
			// Отдельные случаи дополнительно проверяют ошибки имени/разделителя.
			var want []v076Expected
			if tc.want {
				want = []v076Expected{{[8]int{4, tc.start, 4, tc.end, 3, 1, 3, 6}, v076HintInNote}}
			}
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1),
				v076TestInvalid(t, tc.raw, 3, 2), v076TestClose(t, 3, 2)), "current", want)
		})
	}
	for _, tc := range []struct {
		name string
		line model.Line
		want []v076Expected
	}{
		{"recognized with unparsed remainder", v076TestBroken(t, v076TestTag(t, "hint", "content", "Help.", 3, 2), "__"),
			[]v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}}},
		{"content with unparsed declaration", v076TestLine(t, "@hint Help.", "content", 3, 2, v076TestUnparsed(t, "@hint Help.", 1, 12)),
			[]v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}}},
		{"recognized name with erroneous separator", v076TestLine(t, "@hint\u00a0Help.", "invalid", 3, 2,
			v076TestElement(t, "tag", "@hint", "hint", 1, 6), v076TestUnparsed(t, "\u00a0Help.", 6, 12)),
			[]v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}}},
		{"unparsed mention after text", v076TestBroken(t, v076TestText(t, "Read @hint", "Read @hint", 3, 2), "__"), nil},
		{"escaped with unparsed remainder", v076TestBroken(t, v076TestText(t, `\@hint Help.`, "@hint Help.", 3, 2), "__"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1), tc.line, v076TestClose(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("invalid retains tag", func(t *testing.T) {
		line := v076TestBroken(t, v076TestTag(t, "hint", "content", "Help.", 3, 2), "__")
		line.LineType = "invalid"
		v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1), line, v076TestClose(t, 3, 2)), "current",
			[]v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}})
	})
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v076Expected
	}{
		{"brace after recovered child", []model.Line{
			v076TestOpen(t, "note", 2, 1), v076TestLine(t, "@hint{", "block-start", 3, 2,
				v076TestUnparsed(t, "@hint", 1, 6), v076TestElement(t, "block-open", "{", "{", 6, 7)),
			v076TestText(t, "Help.", "Help.", 4, 3), v076TestClose(t, 4, 3), v076TestClose(t, 3, 2),
		}, []v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}}},
		{"recovered parent", []model.Line{
			v076TestLine(t, "@note{", "block-start", 2, 1,
				v076TestUnparsed(t, "@note", 1, 6), v076TestElement(t, "block-open", "{", "{", 6, 7)),
			v076TestTag(t, "hint", "content", "Help.", 3, 2), v076TestClose(t, 3, 2),
		}, []v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}}},
		{"recovered instr exception", []model.Line{
			v076TestLine(t, "@INSTR{", "block-start", 2, 1,
				v076TestUnparsed(t, "@INSTR", 1, 7), v076TestElement(t, "block-open", "{", "{", 7, 8)),
			v076TestText(t, "Read.", "Read.", 3, 2), v076TestTag(t, "hint", "content", "Help.", 3, 2), v076TestClose(t, 3, 2),
		}, nil},
		{"recovered editor excludes literal body", []model.Line{
			v076TestLine(t, "@editor{", "block-start", 2, 1,
				v076TestUnparsed(t, "@editor", 1, 8), v076TestElement(t, "block-open", "{", "{", 8, 9)),
			v076TestText(t, "@hint Help.", "@hint Help.", 3, 2), v076TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v076TestRun(t, v076TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV076KnownSingleLineTags(t *testing.T) {
	// Параметры соответствуют формам DSL. Запрещённое размещение внутри note
	// сохраняется как распознанное объявление либо полностью как unparsed.
	for _, tc := range []struct {
		tag    string
		kind   model.ElementType
		value  string
		end    int
		detail string
	}{
		{"dsl-version", "version", "1.2", 13, "Вложенный тег: @dsl-version. Родительский блок: @note."},
		{"document-id", "identifier", "lesson-a", 13, "Вложенный тег: @document-id. Родительский блок: @note."},
		{"section", "name", "Practice", 9, "Вложенный тег: @section. Родительский блок: @note."},
		{"order", "number", "2", 7, "Вложенный тег: @order. Родительский блок: @note."},
		{"resource-dir", "resource-path", "Audio", 14, "Вложенный тег: @resource-dir. Родительский блок: @note."},
		{"header", "title", "Practice", 8, "Вложенный тег: @header. Родительский блок: @note."},
		{"task", "identifier", "practice", 6, "Вложенный тег: @task. Родительский блок: @note."},
		{"endtask", "", "", 9, "Вложенный тег: @endtask. Родительский блок: @note."},
		{"step", "title", "Read", 6, "Вложенный тег: @step. Родительский блок: @note."},
		{"speaking", "", "", 10, "Вложенный тег: @speaking. Родительский блок: @note."},
		{"newpage", "", "", 9, "Вложенный тег: @newpage. Родительский блок: @note."},
		{"editor", "content", "Check.", 8, "Вложенный тег: @editor. Родительский блок: @note."},
		{"example", "content", "Hello.", 9, "Вложенный тег: @example. Родительский блок: @note."},
		{"wordlist", "content", "one; two.", 10, "Вложенный тег: @wordlist. Родительский блок: @note."},
		{"instr", "content", "Read.", 7, "Вложенный тег: @instr. Родительский блок: @note."},
		{"note", "content", "Read.", 6, "Вложенный тег: @note. Родительский блок: @note."},
		{"alt", "content", "A picture.", 5, "Вложенный тег: @alt. Родительский блок: @note."},
		{"hint", "content", "Help.", 6, v076HintInNote},
		{"include", "identifier", "f", 9, "Вложенный тег: @include. Родительский блок: @note."},
		{"answer", "content", "Yes", 8, "Вложенный тег: @answer. Родительский блок: @note."},
		{"question", "content", "Ready?", 10, "Вложенный тег: @question. Родительский блок: @note."},
		{"multifill", "content", "Write.", 11, "Вложенный тег: @multifill. Родительский блок: @note."},
		{"variant", "name", "First", 9, "Вложенный тег: @variant. Родительский блок: @note."},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.tag+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				line := v076TestTag(t, tc.tag, tc.kind, tc.value, 3, 2)
				if recovered {
					line = v076TestInvalid(t, line.Raw, 3, 2)
				}
				body := []model.Line{v076TestOpen(t, "note", 2, 1), line, v076TestClose(t, 3, 2)}
				if tc.tag == "include" {
					body = append(body, v076TestFragment(t, "f", 2, 1), v076TestText(t, "Read.", "Read.", 6, 2), v076TestClose(t, 6, 2))
				}
				v076TestRun(t, v076TestDocument(t, body...), "current", []v076Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail}})
			})
		}
	}
	// media имеет два раздельных параметра, поэтому задаётся явно.
	for _, recovered := range []bool{false, true} {
		t.Run("media recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v076TestLine(t, "@media image picture.png", "tag", 3, 2,
				v076TestElement(t, "tag", "@media", "media", 1, 7), v076TestElement(t, "media-type", "image", "image", 8, 13),
				v076TestElement(t, "source", "picture.png", "picture.png", 14, 25))
			if recovered {
				line = v076TestInvalid(t, line.Raw, 3, 2)
			}
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1), line, v076TestClose(t, 3, 2)), "current",
				[]v076Expected{{[8]int{4, 1, 4, 7, 3, 1, 3, 6}, "Вложенный тег: @media. Родительский блок: @note."}})
		})
	}
}

func TestV076KnownBlockTags(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		end    int
		detail string
	}{
		{"table", 7, "Вложенный тег: @table. Родительский блок: @note."},
		{"script", 8, "Вложенный тег: @script. Родительский блок: @note."},
		{"text", 6, "Вложенный тег: @text. Родительский блок: @note."},
		{"key", 5, "Вложенный тег: @key. Родительский блок: @note."},
		{"choice", 8, "Вложенный тег: @choice. Родительский блок: @note."},
		{"multichoice", 13, "Вложенный тег: @multichoice. Родительский блок: @note."},
		{"matching", 10, "Вложенный тег: @matching. Родительский блок: @note."},
		{"ordering", 10, "Вложенный тег: @ordering. Родительский блок: @note."},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.tag+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				opening := v076TestOpen(t, tc.tag, 3, 2)
				if recovered {
					// Блок и его скобка сохранены, запрещённое имя осталось unparsed.
					opening.Elements[0] = v076TestUnparsed(t, opening.Elements[0].Raw, 1, opening.Elements[0].End)
				}
				body := []model.Line{v076TestOpen(t, "note", 2, 1), opening, v076TestText(t, "One", "One", 4, 3)}
				if tc.tag == "matching" {
					body = append(body, v076TestLine(t, "---", "separator", 4, 3))
				}
				body = append(body, v076TestText(t, "Two", "Two", 4, 3), v076TestText(t, "Three", "Three", 4, 3),
					v076TestClose(t, 4, 3), v076TestClose(t, 3, 2))
				v076TestRun(t, v076TestDocument(t, body...), "current", []v076Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail}})
			})
		}
	}
	for _, recovered := range []bool{false, true} {
		t.Run("fragment recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v076TestFragment(t, "f", 3, 2)
			if recovered {
				line.Elements[0] = v076TestUnparsed(t, "@fragment", 1, 10)
			}
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1), line,
				v076TestText(t, "Read.", "Read.", 4, 3), v076TestClose(t, 4, 3), v076TestClose(t, 3, 2)), "current",
				[]v076Expected{{[8]int{4, 1, 4, 10, 3, 1, 3, 6}, "Вложенный тег: @fragment. Родительский блок: @note."}})
		})
		t.Run("variants recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v076TestOpen(t, "variants", 3, 2)
			if recovered {
				line.Elements[0] = v076TestUnparsed(t, "@variants", 1, 10)
			}
			v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "note", 2, 1), line,
				v076TestTag(t, "variant", "name", "First", 4, 3), v076TestTag(t, "hint", "content", "Help.", 5, 4),
				v076TestTag(t, "variant", "name", "Second", 4, 3), v076TestText(t, "Read.", "Read.", 7, 4),
				v076TestClose(t, 4, 3), v076TestClose(t, 3, 2)), "current",
				[]v076Expected{{[8]int{4, 1, 4, 10, 3, 1, 3, 6}, "Вложенный тег: @variants. Родительский блок: @note."}})
		})
	}
}

func TestV076BranchesAndOrdinaryParents(t *testing.T) {
	// Ветви изолированы; ParentLine ветви — не фигурный блок.
	v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "variants", 2, 1),
		v076TestTag(t, "variant", "name", "First", 3, 2), v076TestOpen(t, "note", 4, 3),
		v076TestTag(t, "hint", "content", "Help.", 5, 4), v076TestClose(t, 5, 4),
		v076TestTag(t, "variant", "name", "Second", 3, 2), v076TestOpen(t, "instr", 8, 3),
		v076TestText(t, "Read.", "Read.", 9, 4), v076TestTag(t, "hint", "content", "Help.", 9, 4),
		v076TestClose(t, 9, 4), v076TestClose(t, 3, 2)), "current",
		[]v076Expected{{[8]int{6, 1, 6, 6, 5, 1, 5, 6}, v076HintInNote}})
	// Указанные контейнеры не входят в запрещающий список.
	v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "choice", 2, 1),
		v076TestText(t, "No", "No", 3, 2), v076TestTag(t, "answer", "content", "Yes", 3, 2),
		v076TestTag(t, "hint", "content", "Help.", 3, 2), v076TestClose(t, 3, 2)), "choice", nil)
}

func TestV076RangesEndingsAndHistory(t *testing.T) {
	for _, endings := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(endings, func(t *testing.T) {
			result := v076TestDocument(t,
				v076TestLine(t, "\t @NoTe {", "block-start", 2, 1,
					v076TestElement(t, "tag", "@NoTe", "note", 3, 8), v076TestElement(t, "block-open", "{", "{", 9, 10)),
				v076TestLine(t, " \t@HiNt Я😀", "tag", 3, 2,
					v076TestElement(t, "tag", "@HiNt", "hint", 3, 8), v076TestElement(t, "content", "Я😀", "Я😀", 9, 11)),
				v076TestClose(t, 3, 2))
			for i := range result.Lines {
				switch endings {
				case "LF":
					result.Lines[i].LineEnding = "\n"
				case "CRLF":
					result.Lines[i].LineEnding = "\r\n"
				case "mixed":
					if i%2 == 0 {
						result.Lines[i].LineEnding = "\r\n"
					} else {
						result.Lines[i].LineEnding = "\n"
					}
				}
			}
			want := []v076Expected{{[8]int{4, 3, 4, 8, 3, 3, 3, 8}, v076HintInNote}}
			v076TestRun(t, result, "first", want)
			v076TestRun(t, result, "second", want)
			v076TestRun(t, v076TestDocument(t, v076TestTag(t, "hint", "content", "Help.", 2, 1)), "separate", nil)
		})
	}
	t.Run("last declaration at EOF", func(t *testing.T) {
		result := v076TestDocument(t, v076TestOpen(t, "note", 2, 1), v076TestTag(t, "hint", "content", "Help.", 3, 2))
		v076TestParserError(t, result, 3, "Блок не закрыт до конца файла.")
		v076TestRun(t, result, "current", []v076Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v076HintInNote}})
	})
	t.Run("no body", func(t *testing.T) { v076TestRun(t, v076TestDocument(t), "current", nil) })
	t.Run("blank and separator do not declare tags", func(t *testing.T) {
		v076TestRun(t, v076TestDocument(t, v076TestOpen(t, "example", 2, 1),
			v076TestText(t, "Read.", "Read.", 3, 2), v076TestLine(t, " \t", "blank", 3, 2),
			v076TestLine(t, "---", "separator", 3, 2), v076TestText(t, "Write.", "Write.", 3, 2), v076TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV076EmptyAndVersions(t *testing.T) {
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
					v076TestRun(t, result, "current", nil)
					return
				}
				before := v076TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v076TestRegistry(t))
				if err == nil || checks != nil || v076TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

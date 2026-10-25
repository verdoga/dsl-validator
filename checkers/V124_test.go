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

// v124Expected задаёт независимо выписанные диапазоны и конкретизацию.
// Первые четыре координаты — основной диапазон, следующие — связанный.
type v124Expected struct {
	position [8]int
	detail   string
}

const v124HintInNote = "Тег @hint запрещён внутри блока @note."

// v124TestFragment задаёт определение с явно указанным ID.
func v124TestFragment(t *testing.T, id string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(id)
	return v124TestLine(t, "@fragment "+id+" {", "block-start", parent, depth,
		v124TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v124TestElement(t, "identifier", id, id, 11, end),
		v124TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v124TestUnparsed задаёт неразобранную часть с отсутствующим Value.
// Документ добавляет требуемую исходную ошибку и ссылку на неё.
func v124TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v124TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v124TestBroken добавляет явно ошибочный остаток к распознанной строке.
func v124TestBroken(t *testing.T, line model.Line, extra string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " " + extra
	line.Elements = append(line.Elements, v124TestUnparsed(t, extra, start, start+utf8.RuneCountInString(extra)))
	return line
}

// v124TestParserError сохраняет ошибку размещения распознанной конструкции,
// не превращая её в unparsed. Это исходные данные, не ожидание результата V124.
func v124TestParserError(t *testing.T, result *model.Result, number int, message string) {
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

// v124TestDocument добавляет обязательные версию и # Lesson перед заданными
// строками. Родители задаются явно с учётом этого префикса. Для unparsed
// создаётся согласованная исходная ошибка парсера, не являющаяся ожиданием V124.
func v124TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v124TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v124TestHeading(t, "", "#", "1", title),
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

// v124TestHeading собирает заголовок из явно заданных отступа, маркера и уровня.
func v124TestHeading(t *testing.T, indent, marker, level, title string) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(indent) + 1
	end := start + len(marker)
	return v124TestLine(t, indent+marker+" "+title, "heading", 0, 0,
		v124TestElement(t, "heading-level", marker, level, start, end),
		v124TestElement(t, "title", title, title, end+1, end+1+utf8.RuneCountInString(title)))
}

// v124TestInvalid задаёт ошибочную строку без распознанного tag.
func v124TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v124TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v124TestLine(t, raw, "invalid", parent, depth, element)
}

// v124TestTag собирает однострочный тег с одним явно типизированным параметром.
func v124TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v124TestElement(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v124TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v124TestLine(t, raw, "tag", parent, depth, elements...)
}

// v124TestOpen собирает блок; каноническое значение имени тега задано моделью.
func v124TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v124TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v124TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v124TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v124TestClose задаёт закрытие блока с явным родителем.
func v124TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v124TestLine(t, "}", "block-end", parent, depth, v124TestElement(t, "block-close", "}", "}", 1, 2))
}

// v124TestText сохраняет независимо заданные исходное и семантическое значения.
func v124TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v124TestLine(t, raw, "content", parent, depth,
		v124TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v124TestLine не классифицирует исходник и не вычисляет родительские связи.
func v124TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v124TestElement задаёт исходный диапазон в Unicode-колонках, конец исключается.
func v124TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v124TestRegistry использует штатный каталог целиком.
func v124TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v124TestRun проверяет только новые V124 по явно заданным диапазонам:
// первые четыре числа задают основной диапазон, следующие четыре — связанный.
// Нули связанного диапазона означают его отсутствие. Концы исключаются.
func v124TestRun(t *testing.T, result *model.Result, processingID string, want []v124Expected) {
	t.Helper()
	before := v124TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v124TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v124TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V124" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V124 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
			t.Fatalf("occurrence %d = %+v; want location %+v and exact V124 metadata", i, entry, location)
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
	if err := json.Unmarshal([]byte(v124TestSnapshot(t, result)), &restored); err != nil {
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
	if v124TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v124TestSnapshot сохраняет модель для проверки неизменности исходных данных.
func v124TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV124Registration(t *testing.T) {
	definitions, err := v124TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count, legacy := 0, 0
	for _, definition := range definitions {
		if definition.Code == "V076" {
			legacy++
		}
		if definition.Code != "V124" {
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
		t.Fatalf("V124 definitions = %d, want 1", count)
	}
	if legacy != 1 {
		t.Fatalf("V076 must remain registered during migration, got %d", legacy)
	}
}

func TestV124ReferenceAndExample(t *testing.T) {
	// Версия и # Lesson дополняют фрагменты по DSL 1.3, 1.30 и 2.1.3.
	// Сам текст фрагментов, включая пробелы и переводы строк, сохранён.
	for _, tc := range []struct {
		name, child, content, fragment string
		want                           []v124Expected
	}{
		{"reference", "hint", "Look at the title.", "@instr {\nRead the text.\n@hint Look at the title.\n}", nil},
		{"example", "note", "Use the present tense.", "@instr {\nRead the text.\n@note Use the present tense.\n}",
			[]v124Expected{{[8]int{5, 1, 5, 6, 3, 1, 3, 7}, "Тег @note запрещён внутри блока @instr."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v124TestDocument(t, v124TestOpen(t, "instr", 2, 1),
				v124TestText(t, "Read the text.", "Read the text.", 3, 2),
				v124TestTag(t, tc.child, "content", tc.content, 3, 2), v124TestClose(t, 3, 2))
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v124TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV124Parents(t *testing.T) {
	for _, tc := range []struct {
		parent string
		end    int
		detail string
	}{
		{"example", 9, "Тег @hint запрещён внутри блока @example."},
		{"wordlist", 10, "Тег @hint запрещён внутри блока @wordlist."},
		{"table", 7, "Тег @hint запрещён внутри блока @table."},
		{"script", 8, "Тег @hint запрещён внутри блока @script."},
		{"text", 6, "Тег @hint запрещён внутри блока @text."},
		{"key", 5, "Тег @hint запрещён внутри блока @key."},
		{"note", 6, v124HintInNote},
		{"alt", 5, "Тег @hint запрещён внутри блока @alt."},
		{"hint", 6, "Тег @hint запрещён внутри блока @hint."},
	} {
		t.Run(tc.parent, func(t *testing.T) {
			if tc.parent == "key" {
				// DSL 3.7.5: key требует активного задания.
				v124TestRun(t, v124TestDocument(t, v124TestTag(t, "task", "identifier", "practice", 2, 1),
					v124TestOpen(t, "key", 3, 2), v124TestText(t, "Yes", "Yes", 4, 3),
					v124TestTag(t, "hint", "content", "Help.", 4, 3), v124TestClose(t, 4, 3)),
					"current", []v124Expected{{[8]int{6, 1, 6, 6, 4, 1, 4, 5}, tc.detail}})
				return
			}
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, tc.parent, 2, 1),
				v124TestText(t, "Read.", "Read.", 3, 2), v124TestTag(t, "hint", "content", "Help.", 3, 2), v124TestClose(t, 3, 2)),
				"current", []v124Expected{{[8]int{5, 1, 5, 6, 3, 1, 3, tc.end}, tc.detail}})
		})
	}
}

func TestV124NearestBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v124Expected
	}{
		{"block hint in instr", []model.Line{
			v124TestOpen(t, "instr", 2, 1), v124TestText(t, "Read.", "Read.", 3, 2),
			v124TestOpen(t, "hint", 3, 2), v124TestText(t, "Help.", "Help.", 5, 3), v124TestClose(t, 5, 3), v124TestClose(t, 3, 2),
		}, nil},
		{"only hint exception", []model.Line{
			v124TestOpen(t, "instr", 2, 1), v124TestText(t, "Read.", "Read.", 3, 2),
			v124TestTag(t, "note", "content", "Help.", 3, 2), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{5, 1, 5, 6, 3, 1, 3, 7}, "Тег @note запрещён внутри блока @instr."}}},
		{"table in table", []model.Line{
			v124TestOpen(t, "table", 2, 1), v124TestOpen(t, "table", 3, 2),
			v124TestText(t, "One | Two", "One | Two", 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 7, 3, 1, 3, 7}, "Тег @table запрещён внутри блока @table."}}},
		{"each declaration once", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestOpen(t, "hint", 3, 2),
			v124TestTag(t, "note", "content", "Text.", 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{
			{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote},
			{[8]int{5, 1, 5, 6, 4, 1, 4, 6}, "Тег @note запрещён внутри блока @hint."},
		}},
		{"exception ignores distant note", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestOpen(t, "instr", 3, 2),
			v124TestText(t, "Read.", "Read.", 4, 3), v124TestTag(t, "hint", "content", "Help.", 4, 3),
			v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 7, 3, 1, 3, 6}, "Тег @instr запрещён внутри блока @note."}}},
		{"hint in hint is not instr exception", []model.Line{
			v124TestOpen(t, "instr", 2, 1), v124TestText(t, "Read.", "Read.", 3, 2),
			v124TestOpen(t, "hint", 3, 2), v124TestTag(t, "hint", "content", "Help.", 5, 3),
			v124TestClose(t, 5, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{6, 1, 6, 6, 5, 1, 5, 6}, "Тег @hint запрещён внутри блока @hint."}}},
		{"non-block parent does not conceal note", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "task", "identifier", "practice", 3, 2),
			v124TestTag(t, "hint", "content", "Help.", 4, 3), v124TestTag(t, "endtask", "", "", 3, 2), v124TestClose(t, 3, 2),
		}, []v124Expected{
			{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, "Тег @task запрещён внутри блока @note."},
			{[8]int{5, 1, 5, 6, 3, 1, 3, 6}, v124HintInNote},
			{[8]int{6, 1, 6, 9, 3, 1, 3, 6}, "Тег @endtask запрещён внутри блока @note."},
		}},
		{"unknown nearest block does not expose distant note", []model.Line{
			v124TestOpen(t, "note", 2, 1),
			v124TestLine(t, "@unknown {", "block-start", 3, 2,
				v124TestUnparsed(t, "@unknown", 1, 9), v124TestElement(t, "block-open", "{", "{", 10, 11)),
			v124TestTag(t, "hint", "content", "Help.", 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, nil},
		{"after closing block", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "hint", "content", "Inside.", 3, 2),
			v124TestClose(t, 3, 2), v124TestTag(t, "hint", "content", "Outside.", 2, 1),
		}, []v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}}},
		{"several siblings", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "hint", "content", "First.", 3, 2),
			v124TestLine(t, "", "blank", 3, 2), v124TestText(t, "Read.", "Read.", 3, 2),
			v124TestTag(t, "hint", "content", "Second.", 3, 2), v124TestClose(t, 3, 2),
		}, []v124Expected{
			{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}, {[8]int{7, 1, 7, 6, 3, 1, 3, 6}, v124HintInNote},
		}},
		{"no block", []model.Line{
			v124TestTag(t, "note", "content", "Read.", 2, 1), v124TestTag(t, "hint", "content", "Help.", 2, 1),
		}, nil},
		{"allowed fragment container", []model.Line{
			v124TestFragment(t, "f", 2, 1), v124TestTag(t, "hint", "content", "Help.", 3, 2), v124TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v124TestRun(t, v124TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV124LiteralContent(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		end    int
		detail string
	}{
		{"editor", 8, "Тег @editor запрещён внутри блока @note."},
		{"multifill", 11, "Тег @multifill запрещён внутри блока @note."},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			// Текст внутри литерального блока не объявляет вложенные блоки.
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, tc.tag, 2, 1),
				v124TestText(t, "@note {", "@note {", 3, 2), v124TestText(t, "@hint Help.", "@hint Help.", 3, 2),
				v124TestClose(t, 3, 2)), "literal", nil)
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1), v124TestOpen(t, tc.tag, 3, 2),
				v124TestText(t, "@note {", "@note {", 4, 3), v124TestText(t, "@hint Help.", "@hint Help.", 4, 3),
				v124TestClose(t, 4, 3), v124TestClose(t, 3, 2)), "nested",
				[]v124Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail}})
		})
	}
	// Даже ошибочная строка @answer в multifill (DSL 4.4.18) не относится к V124.
	v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "multifill", 2, 1),
		v124TestInvalid(t, "@answer Yes", 3, 2), v124TestClose(t, 3, 2)), "answer", nil)
	for _, tc := range []struct{ raw, value string }{
		{`\@hint Help.`, "@hint Help."},
		{"Read @hint Help.", "Read @hint Help."},
		{"<strong>Read</strong><em>@hint</em><u>Help.</u>", "<strong>Read</strong><em>@hint</em><u>Help.</u>"},
		{"<br>", "<br>"},
		{"&#64;hint Help.", "&#64;hint Help."},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "text", 2, 1),
				v124TestText(t, tc.raw, tc.value, 3, 2), v124TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV124Recovery(t *testing.T) {
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
			var want []v124Expected
			if tc.want {
				want = []v124Expected{{[8]int{4, tc.start, 4, tc.end, 3, 1, 3, 6}, v124HintInNote}}
			}
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1),
				v124TestInvalid(t, tc.raw, 3, 2), v124TestClose(t, 3, 2)), "current", want)
		})
	}
	for _, tc := range []struct {
		name string
		line model.Line
		want []v124Expected
	}{
		{"recognized with unparsed remainder", v124TestBroken(t, v124TestTag(t, "hint", "content", "Help.", 3, 2), "__"),
			[]v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}}},
		{"content with unparsed declaration", v124TestLine(t, "@hint Help.", "content", 3, 2, v124TestUnparsed(t, "@hint Help.", 1, 12)),
			[]v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}}},
		{"recognized name with erroneous separator", v124TestLine(t, "@hint\u00a0Help.", "invalid", 3, 2,
			v124TestElement(t, "tag", "@hint", "hint", 1, 6), v124TestUnparsed(t, "\u00a0Help.", 6, 12)),
			[]v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}}},
		{"unparsed mention after text", v124TestBroken(t, v124TestText(t, "Read @hint", "Read @hint", 3, 2), "__"), nil},
		{"escaped with unparsed remainder", v124TestBroken(t, v124TestText(t, `\@hint Help.`, "@hint Help.", 3, 2), "__"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1), tc.line, v124TestClose(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("invalid retains tag", func(t *testing.T) {
		line := v124TestBroken(t, v124TestTag(t, "hint", "content", "Help.", 3, 2), "__")
		line.LineType = "invalid"
		v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1), line, v124TestClose(t, 3, 2)), "current",
			[]v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}})
	})
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v124Expected
	}{
		{"brace after recovered child", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestLine(t, "@hint{", "block-start", 3, 2,
				v124TestUnparsed(t, "@hint", 1, 6), v124TestElement(t, "block-open", "{", "{", 6, 7)),
			v124TestText(t, "Help.", "Help.", 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}}},
		{"recovered parent", []model.Line{
			v124TestLine(t, "@note{", "block-start", 2, 1,
				v124TestUnparsed(t, "@note", 1, 6), v124TestElement(t, "block-open", "{", "{", 6, 7)),
			v124TestTag(t, "hint", "content", "Help.", 3, 2), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}}},
		{"recovered instr exception", []model.Line{
			v124TestLine(t, "@INSTR{", "block-start", 2, 1,
				v124TestUnparsed(t, "@INSTR", 1, 7), v124TestElement(t, "block-open", "{", "{", 7, 8)),
			v124TestText(t, "Read.", "Read.", 3, 2), v124TestTag(t, "hint", "content", "Help.", 3, 2), v124TestClose(t, 3, 2),
		}, nil},
		{"recovered editor excludes literal body", []model.Line{
			v124TestLine(t, "@editor{", "block-start", 2, 1,
				v124TestUnparsed(t, "@editor", 1, 8), v124TestElement(t, "block-open", "{", "{", 8, 9)),
			v124TestText(t, "@hint Help.", "@hint Help.", 3, 2), v124TestClose(t, 3, 2),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v124TestRun(t, v124TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV124KnownSingleLineTags(t *testing.T) {
	// Параметры соответствуют формам DSL. Запрещённое размещение внутри note
	// сохраняется как распознанное объявление либо полностью как unparsed.
	for _, tc := range []struct {
		tag    string
		kind   model.ElementType
		value  string
		end    int
		detail string
	}{
		{"dsl-version", "version", "1.2", 13, "Тег @dsl-version запрещён внутри блока @note."},
		{"document-id", "identifier", "lesson-a", 13, "Тег @document-id запрещён внутри блока @note."},
		{"section", "name", "Practice", 9, "Тег @section запрещён внутри блока @note."},
		{"order", "number", "2", 7, "Тег @order запрещён внутри блока @note."},
		{"resource-dir", "resource-path", "Audio", 14, "Тег @resource-dir запрещён внутри блока @note."},
		{"header", "title", "Practice", 8, "Тег @header запрещён внутри блока @note."},
		{"task", "identifier", "practice", 6, "Тег @task запрещён внутри блока @note."},
		{"endtask", "", "", 9, "Тег @endtask запрещён внутри блока @note."},
		{"step", "title", "Read", 6, "Тег @step запрещён внутри блока @note."},
		{"speaking", "", "", 10, "Тег @speaking запрещён внутри блока @note."},
		{"newpage", "", "", 9, "Тег @newpage запрещён внутри блока @note."},
		{"editor", "content", "Check.", 8, "Тег @editor запрещён внутри блока @note."},
		{"example", "content", "Hello.", 9, "Тег @example запрещён внутри блока @note."},
		{"wordlist", "content", "one; two.", 10, "Тег @wordlist запрещён внутри блока @note."},
		{"instr", "content", "Read.", 7, "Тег @instr запрещён внутри блока @note."},
		{"note", "content", "Read.", 6, "Тег @note запрещён внутри блока @note."},
		{"alt", "content", "A picture.", 5, "Тег @alt запрещён внутри блока @note."},
		{"hint", "content", "Help.", 6, v124HintInNote},
		{"answer", "content", "Yes", 8, "Тег @answer запрещён внутри блока @note."},
		{"question", "content", "Ready?", 10, "Тег @question запрещён внутри блока @note."},
		{"multifill", "content", "Write.", 11, "Тег @multifill запрещён внутри блока @note."},
		{"variant", "name", "First", 9, "Тег @variant запрещён внутри блока @note."},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.tag+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				if tc.tag == "answer" {
					// DSL 4.1.2: вопрос и ответ находятся на одном уровне.
					answer := v124TestTag(t, "answer", "content", "Yes", 3, 2)
					if recovered {
						answer = v124TestInvalid(t, "@answer Yes", 3, 2)
					}
					v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1),
						v124TestTag(t, "question", "content", "Ready?", 3, 2), answer, v124TestClose(t, 3, 2)), "current",
						[]v124Expected{
							{[8]int{4, 1, 4, 10, 3, 1, 3, 6}, "Тег @question запрещён внутри блока @note."},
							{[8]int{5, 1, 5, 8, 3, 1, 3, 6}, tc.detail},
						})
					return
				}
				if tc.tag == "step" || tc.tag == "endtask" || tc.tag == "speaking" {
					// Служебные теги задания имеют внешнюю активную task.
					line := v124TestTag(t, tc.tag, tc.kind, tc.value, 4, 3)
					if recovered {
						line = v124TestInvalid(t, line.Raw, 4, 3)
					}
					v124TestRun(t, v124TestDocument(t, v124TestTag(t, "task", "identifier", "practice", 2, 1),
						v124TestOpen(t, "note", 3, 2), line, v124TestClose(t, 4, 3)), "current",
						[]v124Expected{{[8]int{5, 1, 5, tc.end, 4, 1, 4, 6}, tc.detail}})
					return
				}
				line := v124TestTag(t, tc.tag, tc.kind, tc.value, 3, 2)
				if recovered {
					line = v124TestInvalid(t, line.Raw, 3, 2)
				}
				body := []model.Line{v124TestOpen(t, "note", 2, 1), line, v124TestClose(t, 3, 2)}
				v124TestRun(t, v124TestDocument(t, body...), "current", []v124Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail}})
			})
		}
	}
	// media имеет два раздельных параметра, поэтому задаётся явно.
	for _, recovered := range []bool{false, true} {
		t.Run("media recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v124TestLine(t, "@media image picture.png", "tag", 3, 2,
				v124TestElement(t, "tag", "@media", "media", 1, 7), v124TestElement(t, "media-type", "image", "image", 8, 13),
				v124TestElement(t, "source", "picture.png", "picture.png", 14, 25))
			if recovered {
				line = v124TestInvalid(t, line.Raw, 3, 2)
			}
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1), line, v124TestClose(t, 3, 2)), "current",
				[]v124Expected{{[8]int{4, 1, 4, 7, 3, 1, 3, 6}, "Тег @media запрещён внутри блока @note."}})
		})
	}
}

func TestV124KnownBlockTags(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		end    int
		detail string
	}{
		{"table", 7, "Тег @table запрещён внутри блока @note."},
		{"script", 8, "Тег @script запрещён внутри блока @note."},
		{"text", 6, "Тег @text запрещён внутри блока @note."},
		{"key", 5, "Тег @key запрещён внутри блока @note."},
		{"choice", 8, "Тег @choice запрещён внутри блока @note."},
		{"multichoice", 13, "Тег @multichoice запрещён внутри блока @note."},
		{"matching", 10, "Тег @matching запрещён внутри блока @note."},
		{"ordering", 10, "Тег @ordering запрещён внутри блока @note."},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.tag+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				if tc.tag == "key" {
					opening := v124TestOpen(t, "key", 4, 3)
					if recovered {
						opening.Elements[0] = v124TestUnparsed(t, "@key", 1, 5)
					}
					v124TestRun(t, v124TestDocument(t, v124TestTag(t, "task", "identifier", "practice", 2, 1),
						v124TestOpen(t, "note", 3, 2), opening, v124TestText(t, "Yes", "Yes", 5, 4),
						v124TestClose(t, 5, 4), v124TestClose(t, 4, 3)), "current",
						[]v124Expected{{[8]int{5, 1, 5, 5, 4, 1, 4, 6}, tc.detail}})
					return
				}
				opening := v124TestOpen(t, tc.tag, 3, 2)
				if recovered {
					// Блок и его скобка сохранены, запрещённое имя осталось unparsed.
					opening.Elements[0] = v124TestUnparsed(t, opening.Elements[0].Raw, 1, opening.Elements[0].End)
				}
				body := []model.Line{v124TestOpen(t, "note", 2, 1), opening, v124TestText(t, "One", "One", 4, 3)}
				if tc.tag == "matching" {
					body = append(body, v124TestLine(t, "---", "separator", 4, 3))
				}
				body = append(body, v124TestText(t, "Two", "Two", 4, 3), v124TestText(t, "Three", "Three", 4, 3),
					v124TestClose(t, 4, 3), v124TestClose(t, 3, 2))
				v124TestRun(t, v124TestDocument(t, body...), "current", []v124Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail}})
			})
		}
	}
	for _, recovered := range []bool{false, true} {
		t.Run("fragment recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v124TestFragment(t, "f", 3, 2)
			if recovered {
				line.Elements[0] = v124TestUnparsed(t, "@fragment", 1, 10)
			}
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1), line,
				v124TestText(t, "Read.", "Read.", 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2)), "current",
				[]v124Expected{{[8]int{4, 1, 4, 10, 3, 1, 3, 6}, "Тег @fragment запрещён внутри блока @note."}})
		})
		t.Run("variants recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v124TestOpen(t, "variants", 3, 2)
			if recovered {
				line.Elements[0] = v124TestUnparsed(t, "@variants", 1, 10)
			}
			v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "note", 2, 1), line,
				v124TestTag(t, "variant", "name", "First", 4, 3), v124TestTag(t, "hint", "content", "Help.", 5, 4),
				v124TestTag(t, "variant", "name", "Second", 4, 3), v124TestText(t, "Read.", "Read.", 7, 4),
				v124TestClose(t, 4, 3), v124TestClose(t, 3, 2)), "current",
				[]v124Expected{{[8]int{4, 1, 4, 10, 3, 1, 3, 6}, "Тег @variants запрещён внутри блока @note."}})
		})
	}
}

func TestV124BranchesAndOrdinaryParents(t *testing.T) {
	// Ветви изолированы; ParentLine ветви — не фигурный блок.
	v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "variants", 2, 1),
		v124TestTag(t, "variant", "name", "First", 3, 2), v124TestOpen(t, "note", 4, 3),
		v124TestTag(t, "hint", "content", "Help.", 5, 4), v124TestClose(t, 5, 4),
		v124TestTag(t, "variant", "name", "Second", 3, 2), v124TestOpen(t, "instr", 8, 3),
		v124TestText(t, "Read.", "Read.", 9, 4), v124TestTag(t, "hint", "content", "Help.", 9, 4),
		v124TestClose(t, 9, 4), v124TestClose(t, 3, 2)), "current",
		[]v124Expected{{[8]int{6, 1, 6, 6, 5, 1, 5, 6}, v124HintInNote}})
	// Указанные контейнеры не входят в запрещающий список.
	v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "choice", 2, 1),
		v124TestText(t, "No", "No", 3, 2), v124TestTag(t, "answer", "content", "Yes", 3, 2),
		v124TestTag(t, "hint", "content", "Help.", 3, 2), v124TestClose(t, 3, 2)), "choice", nil)
}

func TestV124RangesEndingsAndHistory(t *testing.T) {
	for _, endings := range []string{"LF", "CRLF", "mixed", "EOF"} {
		t.Run(endings, func(t *testing.T) {
			result := v124TestDocument(t,
				v124TestLine(t, "\t @NoTe {", "block-start", 2, 1,
					v124TestElement(t, "tag", "@NoTe", "note", 3, 8), v124TestElement(t, "block-open", "{", "{", 9, 10)),
				v124TestLine(t, " \t@HiNt Я😀", "tag", 3, 2,
					v124TestElement(t, "tag", "@HiNt", "hint", 3, 8), v124TestElement(t, "content", "Я😀", "Я😀", 9, 11)),
				v124TestClose(t, 3, 2))
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
			want := []v124Expected{{[8]int{4, 3, 4, 8, 3, 3, 3, 8}, v124HintInNote}}
			v124TestRun(t, result, "first", want)
			v124TestRun(t, result, "second", want)
			v124TestRun(t, v124TestDocument(t, v124TestTag(t, "hint", "content", "Help.", 2, 1)), "separate", nil)
		})
	}
	t.Run("last declaration at EOF", func(t *testing.T) {
		result := v124TestDocument(t, v124TestOpen(t, "note", 2, 1), v124TestTag(t, "hint", "content", "Help.", 3, 2))
		v124TestParserError(t, result, 3, "Блок не закрыт до конца файла.")
		v124TestRun(t, result, "current", []v124Expected{{[8]int{4, 1, 4, 6, 3, 1, 3, 6}, v124HintInNote}})
	})
	t.Run("no body", func(t *testing.T) { v124TestRun(t, v124TestDocument(t), "current", nil) })
	t.Run("blank and separator do not declare tags", func(t *testing.T) {
		v124TestRun(t, v124TestDocument(t, v124TestOpen(t, "example", 2, 1),
			v124TestText(t, "Read.", "Read.", 3, 2), v124TestLine(t, " \t", "blank", 3, 2),
			v124TestLine(t, "---", "separator", 3, 2), v124TestText(t, "Write.", "Write.", 3, 2), v124TestClose(t, 3, 2)), "current", nil)
	})
}

func TestV124EmptyAndVersions(t *testing.T) {
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
					v124TestRun(t, result, "current", nil)
					return
				}
				before := v124TestSnapshot(t, result)
				checks, err := validator.Prepare(result, v124TestRegistry(t))
				if err == nil || checks != nil || v124TestSnapshot(t, result) != before {
					t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
				}
			})
		}
	}
}

func TestV124FragmentSingleLineTags(t *testing.T) {
	// Ожидания перечислены по закрытому списку V124. Пустая конкретизация
	// в таблице означает ровно ноль срабатываний, а не пропуск проверки.
	for _, tc := range []struct {
		tag, value string
		kind       model.ElementType
		end        int
		detail     string
	}{
		{"dsl-version", "1.2", "version", 13, "Тег @dsl-version запрещён внутри блока @fragment."},
		{"document-id", "lesson-a", "identifier", 13, "Тег @document-id запрещён внутри блока @fragment."},
		{"section", "Practice", "name", 9, "Тег @section запрещён внутри блока @fragment."},
		{"order", "2", "number", 7, "Тег @order запрещён внутри блока @fragment."},
		{"resource-dir", "Audio", "resource-path", 14, "Тег @resource-dir запрещён внутри блока @fragment."},
		{"header", "Practice", "title", 8, "Тег @header запрещён внутри блока @fragment."},
		{"task", "practice", "identifier", 6, "Тег @task запрещён внутри блока @fragment."},
		{"endtask", "", "", 9, "Тег @endtask запрещён внутри блока @fragment."},
		{"step", "Read", "title", 6, "Тег @step запрещён внутри блока @fragment."},
		{"speaking", "", "", 10, "Тег @speaking запрещён внутри блока @fragment."},
		{"newpage", "", "", 9, "Тег @newpage запрещён внутри блока @fragment."},
		{"editor", "Check.", "content", 8, "Тег @editor запрещён внутри блока @fragment."},
		{"example", "Hello.", "content", 9, ""},
		{"wordlist", "one; two.", "content", 10, ""},
		{"instr", "Read.", "content", 7, ""},
		{"note", "Read.", "content", 6, ""},
		{"alt", "A picture.", "content", 5, ""},
		{"hint", "Help.", "content", 6, ""},
		{"answer", "Yes", "content", 8, "Тег @answer запрещён внутри блока @fragment."},
		{"question", "Ready?", "content", 10, "Тег @question запрещён внутри блока @fragment."},
		{"multifill", "Write.", "content", 11, "Тег @multifill запрещён внутри блока @fragment."},
		{"variant", "First", "name", 9, "Тег @variant запрещён внутри блока @fragment."},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.tag+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				if tc.tag == "answer" {
					answer := v124TestTag(t, "answer", "content", "Yes", 3, 2)
					if recovered {
						answer = v124TestInvalid(t, "@answer Yes", 3, 2)
					}
					v124TestRun(t, v124TestDocument(t, v124TestFragment(t, "f", 2, 1),
						v124TestTag(t, "question", "content", "Ready?", 3, 2), answer, v124TestClose(t, 3, 2)), "current",
						[]v124Expected{
							{[8]int{4, 1, 4, 10, 3, 1, 3, 10}, "Тег @question запрещён внутри блока @fragment."},
							{[8]int{5, 1, 5, 8, 3, 1, 3, 10}, tc.detail},
						})
					return
				}
				if tc.tag == "step" || tc.tag == "endtask" || tc.tag == "speaking" {
					line := v124TestTag(t, tc.tag, tc.kind, tc.value, 4, 3)
					if recovered {
						line = v124TestInvalid(t, line.Raw, 4, 3)
					}
					v124TestRun(t, v124TestDocument(t, v124TestTag(t, "task", "identifier", "practice", 2, 1),
						v124TestFragment(t, "f", 3, 2), line, v124TestClose(t, 4, 3)), "current",
						[]v124Expected{{[8]int{5, 1, 5, tc.end, 4, 1, 4, 10}, tc.detail}})
					return
				}
				line := v124TestTag(t, tc.tag, tc.kind, tc.value, 3, 2)
				if recovered {
					line = v124TestInvalid(t, line.Raw, 3, 2)
				}
				var want []v124Expected
				if tc.detail != "" {
					want = []v124Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 10}, tc.detail}}
				}
				v124TestRun(t, v124TestDocument(t, v124TestFragment(t, "f", 2, 1), line,
					v124TestClose(t, 3, 2)), "current", want)
			})
		}
	}
	for _, recovered := range []bool{false, true} {
		t.Run("media recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			line := v124TestLine(t, "@media image picture.png", "tag", 3, 2,
				v124TestElement(t, "tag", "@media", "media", 1, 7),
				v124TestElement(t, "media-type", "image", "image", 8, 13),
				v124TestElement(t, "source", "picture.png", "picture.png", 14, 25))
			if recovered {
				line = v124TestInvalid(t, line.Raw, 3, 2)
			}
			v124TestRun(t, v124TestDocument(t, v124TestFragment(t, "f", 2, 1), line,
				v124TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV124FragmentBlockTags(t *testing.T) {
	for _, tc := range []struct {
		tag, content string
		end          int
		detail       string
	}{
		{"example", "Hello.", 9, ""}, {"wordlist", "one; two.", 10, ""},
		{"table", "A | B", 7, ""}, {"script", "A: Hello.", 8, ""},
		{"text", "Read.", 6, ""}, {"instr", "Read.", 7, ""},
		{"note", "Read.", 6, ""}, {"alt", "A picture.", 5, ""}, {"hint", "Help.", 6, ""},
		{"key", "Yes", 5, "Тег @key запрещён внутри блока @fragment."},
		{"editor", "@note Literal text.", 8, "Тег @editor запрещён внутри блока @fragment."},
		{"multifill", "@hint Literal text.", 11, "Тег @multifill запрещён внутри блока @fragment."},
		{"choice", "One", 8, "Тег @choice запрещён внутри блока @fragment."},
		{"multichoice", "One", 13, "Тег @multichoice запрещён внутри блока @fragment."},
		{"matching", "One", 10, "Тег @matching запрещён внутри блока @fragment."},
		{"ordering", "One", 10, "Тег @ordering запрещён внутри блока @fragment."},
	} {
		for _, recovered := range []bool{false, true} {
			t.Run(tc.tag+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				if tc.tag == "key" {
					opening := v124TestOpen(t, "key", 4, 3)
					if recovered {
						opening.Elements[0] = v124TestUnparsed(t, "@key", 1, 5)
					}
					v124TestRun(t, v124TestDocument(t, v124TestTag(t, "task", "identifier", "practice", 2, 1),
						v124TestFragment(t, "f", 3, 2), opening, v124TestText(t, "Yes", "Yes", 5, 4),
						v124TestClose(t, 5, 4), v124TestClose(t, 4, 3)), "current",
						[]v124Expected{{[8]int{5, 1, 5, 5, 4, 1, 4, 10}, tc.detail}})
					return
				}
				opening := v124TestOpen(t, tc.tag, 3, 2)
				if recovered {
					opening.Elements[0] = v124TestUnparsed(t, opening.Elements[0].Raw, 1, opening.Elements[0].End)
				}
				body := []model.Line{v124TestFragment(t, "f", 2, 1), opening,
					v124TestText(t, tc.content, tc.content, 4, 3)}
				if tc.tag == "matching" {
					body = append(body, v124TestLine(t, "---", "separator", 4, 3))
				}
				second, third := "Two", "Three"
				if tc.tag == "table" {
					// DSL 3.4.9: обе следующие строки также имеют две ячейки.
					second, third = "C | D", "E | F"
				}
				body = append(body, v124TestText(t, second, second, 4, 3),
					v124TestText(t, third, third, 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2))
				var want []v124Expected
				if tc.detail != "" {
					want = []v124Expected{{[8]int{4, 1, 4, tc.end, 3, 1, 3, 10}, tc.detail}}
				}
				v124TestRun(t, v124TestDocument(t, body...), "current", want)
			})
		}
	}
	for _, recovered := range []bool{false, true} {
		t.Run("fragment recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			opening := v124TestFragment(t, "inner", 3, 2)
			if recovered {
				opening.Elements[0] = v124TestUnparsed(t, "@fragment", 1, 10)
			}
			v124TestRun(t, v124TestDocument(t, v124TestFragment(t, "outer", 2, 1), opening,
				v124TestText(t, "Read.", "Read.", 4, 3), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2)), "current",
				[]v124Expected{{[8]int{4, 1, 4, 10, 3, 1, 3, 10}, "Тег @fragment запрещён внутри блока @fragment."}})
		})
		t.Run("variants recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
			opening := v124TestOpen(t, "variants", 3, 2)
			if recovered {
				opening.Elements[0] = v124TestUnparsed(t, "@variants", 1, 10)
			}
			v124TestRun(t, v124TestDocument(t, v124TestFragment(t, "f", 2, 1), opening,
				v124TestTag(t, "variant", "name", "First", 4, 3), v124TestTag(t, "hint", "content", "Help.", 5, 4),
				v124TestTag(t, "variant", "name", "Second", 4, 3), v124TestText(t, "Read.", "Read.", 7, 4),
				v124TestClose(t, 4, 3), v124TestClose(t, 3, 2)), "current",
				[]v124Expected{{[8]int{4, 1, 4, 10, 3, 1, 3, 10}, "Тег @variants запрещён внутри блока @fragment."}})
		})
	}
}

func TestV124IncludeExcludedIndependently(t *testing.T) {
	for _, parent := range []string{"example", "wordlist", "table", "script", "text", "key", "note", "alt", "hint", "instr", "fragment"} {
		for _, recovered := range []bool{false, true} {
			t.Run(parent+" recovered="+strconv.FormatBool(recovered), func(t *testing.T) {
				if parent == "key" {
					include := v124TestTag(t, "include", "identifier", "f", 4, 3)
					if recovered {
						include = v124TestInvalid(t, "@InClUdE\tf", 4, 3)
					}
					result := v124TestDocument(t, v124TestTag(t, "task", "identifier", "practice", 2, 1),
						v124TestOpen(t, "key", 3, 2), include, v124TestClose(t, 4, 3),
						v124TestFragment(t, "f", 3, 2), v124TestText(t, "Read.", "Read.", 7, 3), v124TestClose(t, 7, 3))
					v124TestRun(t, result, "first", nil)
					v124TestRun(t, result, "second", nil)
					return
				}
				opening := v124TestOpen(t, parent, 2, 1)
				if parent == "fragment" {
					opening = v124TestFragment(t, "outer", 2, 1)
				}
				include := v124TestTag(t, "InClUdE", "identifier", "f", 3, 2)
				if recovered {
					include = v124TestInvalid(t, "@InClUdE\tf", 3, 2)
				}
				result := v124TestDocument(t, opening, include, v124TestClose(t, 3, 2),
					v124TestFragment(t, "f", 2, 1), v124TestText(t, "Read.", "Read.", 6, 2), v124TestClose(t, 6, 2))
				v124TestRun(t, result, "first", nil)
				v124TestRun(t, result, "second", nil)
			})
		}
	}
}

func TestV124FragmentNearestParentAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v124Expected
	}{
		{"allowed nested instr hint", []model.Line{
			v124TestFragment(t, "f", 2, 1), v124TestOpen(t, "instr", 3, 2),
			v124TestText(t, "Read.", "Read.", 4, 3), v124TestTag(t, "hint", "content", "Help.", 4, 3),
			v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, nil},
		{"note inside allowed instr is forbidden", []model.Line{
			v124TestFragment(t, "f", 2, 1), v124TestOpen(t, "instr", 3, 2),
			v124TestText(t, "Read.", "Read.", 4, 3), v124TestTag(t, "note", "content", "Help.", 4, 3),
			v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{6, 1, 6, 6, 4, 1, 4, 7}, "Тег @note запрещён внутри блока @instr."}}},
		{"recovered fragment parent preserves spelling and range", []model.Line{
			v124TestLine(t, " \t@FrAgMeNt f{", "block-start", 2, 1,
				v124TestUnparsed(t, "@FrAgMeNt f", 3, 14), v124TestElement(t, "block-open", "{", "{", 14, 15)),
			v124TestTag(t, "editor", "content", "Check.", 3, 2), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 8, 3, 3, 3, 12}, "Тег @editor запрещён внутри блока @fragment."}}},
		{"ordinary nearest block hides distant note", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestOpen(t, "choice", 3, 2),
			v124TestText(t, "No", "No", 4, 3), v124TestTag(t, "answer", "content", "Yes", 4, 3),
			v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 8, 3, 1, 3, 6}, "Тег @choice запрещён внутри блока @note."}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v124TestRun(t, v124TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV124NewpageUsesPhysicalContainer(t *testing.T) {
	result := v124TestDocument(t,
		v124TestOpen(t, "variants", 2, 1),
		v124TestTag(t, "variant", "name", "A", 3, 2),
		v124TestLine(t, " \t@NoTe {", "block-start", 4, 3,
			v124TestElement(t, "tag", "@NoTe", "note", 3, 8),
			v124TestElement(t, "block-open", "{", "{", 9, 10)),
		// Переход страницы меняет логического родителя, но не закрывает note.
		v124TestLine(t, " \t@NeWpAgE", "tag", 2, 1,
			v124TestElement(t, "tag", "@NeWpAgE", "newpage", 3, 11)),
		v124TestClose(t, 5, 4),
		v124TestTag(t, "newpage", "", "", 6, 2),
		v124TestTag(t, "variant", "name", "B", 3, 2),
		v124TestText(t, "Write.", "Write.", 9, 3),
		v124TestClose(t, 3, 2),
		v124TestTag(t, "newpage", "", "", 2, 1))
	v124TestRun(t, result, "current", []v124Expected{
		{[8]int{6, 3, 6, 11, 5, 3, 5, 8}, "Тег @newpage запрещён внутри блока @note."},
	})
	// За пределами note, но до закрытия variants владельцем остаётся V196.
	var count int
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Source != "current" || diagnostic.DiagnosticCode != "V196" {
			continue
		}
		count++
		location := model.Location{Start: model.Position{Line: 8, Column: 1}, End: model.Position{Line: 8, Column: 9}}
		related := model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 10}}
		if diagnostic.Location == nil || *diagnostic.Location != location ||
			len(diagnostic.RelatedLocations) != 1 || diagnostic.RelatedLocations[0] != related {
			t.Fatalf("V196 must own only the newpage directly in variants: %+v", diagnostic)
		}
	}
	if count != 1 {
		t.Fatalf("V196 count = %d, want 1", count)
	}
}

func TestV124NewpageRecoversPhysicalParent(t *testing.T) {
	v124TestRun(t, v124TestDocument(t,
		v124TestLine(t, " \t@NoTe{", "block-start", 2, 1,
			v124TestUnparsed(t, "@NoTe", 3, 8), v124TestElement(t, "block-open", "{", "{", 8, 9)),
		v124TestInvalid(t, " \t@NeWpAgE Текст😀", 2, 1),
		v124TestClose(t, 3, 2)), "current", []v124Expected{
		{[8]int{4, 3, 4, 11, 3, 3, 3, 8}, "Тег @newpage запрещён внутри блока @note."},
	})
}

func TestV124NewpagePartiallyKnownPhysicalBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		errorLine int
		want      []v124Expected
	}{
		{"unclosed note", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "newpage", "", "", 3, 2),
		}, 3, []v124Expected{{[8]int{4, 1, 4, 9, 3, 1, 3, 6}, "Тег @newpage запрещён внутри блока @note."}}},
		{"unparsed closing", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "newpage", "", "", 3, 2),
			v124TestInvalid(t, "} extra", 3, 2),
		}, 0, []v124Expected{{[8]int{4, 1, 4, 9, 3, 1, 3, 6}, "Тег @newpage запрещён внутри блока @note."}}},
		{"unclosed unknown nearest block hides note", []model.Line{
			v124TestOpen(t, "note", 2, 1),
			v124TestLine(t, "@unknown {", "block-start", 3, 2,
				v124TestUnparsed(t, "@unknown", 1, 9), v124TestElement(t, "block-open", "{", "{", 10, 11)),
			v124TestTag(t, "newpage", "", "", 3, 2), v124TestClose(t, 3, 2),
		}, 0, nil},
		{"ambiguous closing", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "newpage", "", "", 3, 2),
			v124TestClose(t, 3, 2), v124TestClose(t, 3, 2),
		}, 6, []v124Expected{{[8]int{4, 1, 4, 9, 3, 1, 3, 6}, "Тег @newpage запрещён внутри блока @note."}}},
		{"duplicate closing does not reopen note", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "newpage", "", "", 3, 2),
			v124TestClose(t, 3, 2),
			// Логическая цепочка страниц сохраняется после фигурной границы.
			v124TestTag(t, "newpage", "", "", 4, 3), v124TestClose(t, 3, 2),
		}, 7, []v124Expected{{[8]int{4, 1, 4, 9, 3, 1, 3, 6}, "Тег @newpage запрещён внутри блока @note."}}},
		{"crossing intervals", []model.Line{
			v124TestOpen(t, "choice", 2, 1), v124TestOpen(t, "note", 3, 2),
			v124TestTag(t, "newpage", "", "", 4, 3), v124TestClose(t, 3, 2), v124TestClose(t, 4, 3),
		}, 6, nil},
		{"crossing starts after declaration", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestTag(t, "newpage", "", "", 3, 2),
			v124TestOpen(t, "choice", 3, 2), v124TestClose(t, 3, 2), v124TestClose(t, 5, 3),
		}, 6, []v124Expected{
			{[8]int{4, 1, 4, 9, 3, 1, 3, 6}, "Тег @newpage запрещён внутри блока @note."},
			{[8]int{5, 1, 5, 8, 3, 1, 3, 6}, "Тег @choice запрещён внутри блока @note."},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v124TestDocument(t, tc.body...)
			if tc.errorLine != 0 {
				v124TestParserError(t, result, tc.errorLine, "Граница блока не определена однозначно.")
			}
			v124TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV124NewpageNearestPhysicalBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v124Expected
	}{
		{"unknown nearest name hides note", []model.Line{
			v124TestOpen(t, "note", 2, 1),
			v124TestLine(t, "@unknown {", "block-start", 3, 2,
				v124TestUnparsed(t, "@unknown", 1, 9), v124TestElement(t, "block-open", "{", "{", 10, 11)),
			v124TestTag(t, "newpage", "", "", 3, 2), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, nil},
		{"known nearest choice hides note", []model.Line{
			v124TestOpen(t, "note", 2, 1), v124TestOpen(t, "choice", 3, 2),
			v124TestTag(t, "newpage", "", "", 3, 2), v124TestClose(t, 4, 3), v124TestClose(t, 3, 2),
		}, []v124Expected{{[8]int{4, 1, 4, 8, 3, 1, 3, 6}, "Тег @choice запрещён внутри блока @note."}}},
		{"known inner note survives unclosed outer", []model.Line{
			v124TestOpen(t, "choice", 2, 1), v124TestOpen(t, "note", 3, 2),
			v124TestTag(t, "newpage", "", "", 2, 1), v124TestClose(t, 4, 3),
		}, []v124Expected{{[8]int{5, 1, 5, 9, 4, 1, 4, 6}, "Тег @newpage запрещён внутри блока @note."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v124TestDocument(t, tc.body...)
			if tc.name == "known inner note survives unclosed outer" {
				v124TestParserError(t, result, 3, "Блок @choice не закрыт.")
			}
			v124TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV124NewpageLiteralPhysicalContent(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		end    int
		detail string
	}{
		{"editor", 8, "Тег @editor запрещён внутри блока @note."},
		{"multifill", 11, "Тег @multifill запрещён внутри блока @note."},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			v124TestRun(t, v124TestDocument(t,
				v124TestOpen(t, "note", 2, 1), v124TestOpen(t, tc.tag, 3, 2),
				v124TestText(t, "@newpage", "@newpage", 4, 3),
				// Ошибка экранирования не превращает литеральную строку в тег.
				v124TestLine(t, `@newpage \`, "content", 4, 3,
					v124TestElement(t, "content", "@newpage ", "@newpage ", 1, 10),
					v124TestUnparsed(t, "\\", 10, 11)),
				v124TestClose(t, 4, 3), v124TestClose(t, 3, 2)), "current", []v124Expected{
				{[8]int{4, 1, 4, tc.end, 3, 1, 3, 6}, tc.detail},
			})
		})
	}
}

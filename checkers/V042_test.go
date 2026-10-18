package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v042Expected задаёт независимые ожидания: строку и Unicode-колонки [start,end).
type v042Expected struct{ line, start, end int }

func TestV042Registration(t *testing.T) {
	definitions, err := v042Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V042" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Значение @order не является положительным целым числом." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V042 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V042 definitions = %d, want 1", matches)
	}
}

func TestV042Arguments(t *testing.T) {
	for _, tc := range []struct {
		name, argument, kind string
		want                 []v042Expected
	}{
		{"reference", "020", "number", nil},
		{"one", "1", "number", nil},
		{"leading zeroes", "0001", "number", nil},
		{"zero", "0", "number", []v042Expected{{2, 8, 9}}},
		{"all zeroes", "000", "number", []v042Expected{{2, 8, 11}}},
		{"example", "2.5", "unparsed", []v042Expected{{2, 8, 11}}},
		{"comma", "2,5", "unparsed", []v042Expected{{2, 8, 11}}},
		{"plus", "+2", "unparsed", []v042Expected{{2, 8, 10}}},
		{"minus", "-2", "unparsed", []v042Expected{{2, 8, 10}}},
		{"Unicode minus", "−2", "unparsed", []v042Expected{{2, 8, 10}}},
		{"exponent", "2e3", "unparsed", []v042Expected{{2, 8, 11}}},
		{"internal space", "1 2", "unparsed", []v042Expected{{2, 8, 11}}},
		{"internal tab", "1\t2", "unparsed", []v042Expected{{2, 8, 11}}},
		{"several defects one finding", "+2.5x", "unparsed", []v042Expected{{2, 8, 13}}},
		{"Arabic digit", "٢", "unparsed", []v042Expected{{2, 8, 9}}},
		{"fullwidth digit", "２", "unparsed", []v042Expected{{2, 8, 9}}},
		{"supplementary digit", "𝟚", "unparsed", []v042Expected{{2, 8, 9}}},
		{"mixed digits", "1٢", "unparsed", []v042Expected{{2, 8, 10}}},
		{"quoted", "\"2\"", "unparsed", []v042Expected{{2, 8, 11}}},
		{"letters", "abc", "unparsed", []v042Expected{{2, 8, 11}}},
		{"emoji", "😀", "unparsed", []v042Expected{{2, 8, 9}}},
		{"punctuation", "!", "unparsed", []v042Expected{{2, 8, 9}}},
		{"NBSP", "\u00a0", "unparsed", []v042Expected{{2, 8, 9}}},
		{"em space", "\u2003", "unparsed", []v042Expected{{2, 8, 9}}},
		{"NBSP before digit", "\u00a02", "unparsed", []v042Expected{{2, 8, 10}}},
		{"NBSP after digit", "2\u00a0", "unparsed", []v042Expected{{2, 8, 10}}},
		{"beyond uint64", "18446744073709551616", "number", nil},
		{"long positive", strings.Repeat("9", 4096), "number", nil},
		{"long zero", strings.Repeat("0", 4096), "number", []v042Expected{{2, 8, 4104}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Входной диапазон строится по длине аргумента; ожидаемый задан в таблице.
			element := v042Element(t, model.ElementType(tc.kind), tc.argument, tc.argument, 8, 8+utf8.RuneCountInString(tc.argument))
			if tc.kind == "unparsed" {
				element.Value = nil
			}
			result := v042Metadata(t, v042Line(t, "@order "+tc.argument, "tag", 0, 0,
				v042Element(t, "tag", "@order", "order", 1, 7), element))
			if tc.kind == "unparsed" {
				v042PriorError(t, result, 2, 1, "Неверный аргумент order.")
			} else {
				result.Document.Metadata.Order = &tc.argument
			}
			v042Run(t, result, "current", tc.want)
		})
	}
}

func TestV042WhitespaceAndCase(t *testing.T) {
	for _, tc := range []struct {
		name, raw, tag, argument string
		tagStart, start, end     int
		want                     []v042Expected
	}{
		{"empty", "@order", "@order", "", 1, 0, 0, nil},
		{"spaces only", "@order   ", "@order", "", 1, 0, 0, nil},
		{"tabs only", "@order\t\t", "@order", "", 1, 0, 0, nil},
		{"mixed empty", "\t @order \t ", "@order", "", 3, 0, 0, nil},
		{"trim valid", "@order \t 020 \t", "@order", "020", 1, 10, 13, nil},
		{"trim invalid", "  @order \t 2.5 \t", "@order", "2.5", 3, 12, 15, []v042Expected{{2, 12, 15}}},
		{"uppercase", "@ORDER 0", "@ORDER", "0", 1, 8, 9, []v042Expected{{2, 8, 9}}},
		{"mixed case", "@OrDeR 0", "@OrDeR", "0", 1, 8, 9, []v042Expected{{2, 8, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v042Line(t, tc.raw, "tag", 0, 0, v042Element(t, "tag", tc.tag, "order", tc.tagStart, tc.tagStart+6))
			if tc.argument != "" {
				line.Elements = append(line.Elements, v042Element(t, "number", tc.argument, tc.argument, tc.start, tc.end))
			}
			result := v042Metadata(t, line)
			if tc.argument == "2.5" {
				result.Lines[1].Elements[1].ElementType = "unparsed"
				result.Lines[1].Elements[1].Value = nil
				v042PriorError(t, result, 2, 1, "Неверный аргумент order.")
			} else if tc.argument != "" {
				result.Document.Metadata.Order = &tc.argument
			}
			v042Run(t, result, "current", tc.want)
		})
	}
}

func TestV042UnparsedSuffixAndHistory(t *testing.T) {
	result := v042Metadata(t, v042Line(t, "@order 2.5", "tag", 0, 0,
		v042Element(t, "tag", "@order", "order", 1, 7),
		v042Element(t, "number", "2", "2", 8, 9),
		model.Element{ElementType: "unparsed", Raw: ".5", Start: 9, End: 11, ErrorIDs: []string{}}))
	v042PriorError(t, result, 2, 2, "Недопустимый остаток аргумента.")
	v042Run(t, result, "first", []v042Expected{{2, 8, 11}})
	v042Run(t, result, "second", []v042Expected{{2, 8, 11}})
}

func TestV042PlacementAndBoundaries(t *testing.T) {
	t.Run("after heading", func(t *testing.T) {
		result := v042Document(t, v042Order(t, "0", 2, 1), v042Text(t, "Read.", 2, 1))
		v042PriorError(t, result, 3, 0, "Мета-тег расположен после заголовка.")
		v042Run(t, result, "current", []v042Expected{{3, 8, 9}})
	})
	t.Run("nested recognized tag", func(t *testing.T) {
		result := v042Document(t, v042Open(t, "text", 2, 1),
			v042Line(t, "@order 2.5", "tag", 3, 2, v042Element(t, "tag", "@order", "order", 1, 7),
				model.Element{ElementType: "unparsed", Raw: "2.5", Start: 8, End: 11, ErrorIDs: []string{}}),
			v042Text(t, "Read.", 3, 2), v042Close(t, 3, 2))
		v042PriorError(t, result, 4, 0, "Вложенный тег запрещён.")
		v042PriorError(t, result, 4, 1, "Неверный аргумент order.")
		v042Run(t, result, "current", []v042Expected{{4, 8, 11}})
	})
	t.Run("next line is not an argument", func(t *testing.T) {
		result := v042Document(t, v042Line(t, "@order", "tag", 2, 1,
			v042Element(t, "tag", "@order", "order", 1, 7)), v042Text(t, "2.5", 2, 1))
		v042PriorError(t, result, 3, 0, "Мета-тег расположен после заголовка.")
		v042Run(t, result, "current", nil)
	})
	t.Run("multiple declarations", func(t *testing.T) {
		result := v042Metadata(t, v042Order(t, "0", 0, 0), v042Order(t, "020", 0, 0),
			v042Line(t, "@order 2.5", "tag", 0, 0, v042Element(t, "tag", "@order", "order", 1, 7),
				model.Element{ElementType: "unparsed", Raw: "2.5", Start: 8, End: 11, ErrorIDs: []string{}}))
		v042PriorError(t, result, 3, 0, "Повторное объявление order.")
		v042PriorError(t, result, 4, 0, "Повторное объявление order.")
		v042PriorError(t, result, 4, 1, "Неверный аргумент order.")
		v042Run(t, result, "current", []v042Expected{{2, 8, 9}, {4, 8, 11}})
	})
	t.Run("last line without EOL", func(t *testing.T) {
		result := v042Document(t, v042Text(t, "Read.", 2, 1), v042Order(t, "0", 2, 1))
		result.Lines[3].LineEnding = ""
		v042PriorError(t, result, 4, 0, "Мета-тег расположен после заголовка.")
		v042Run(t, result, "current", []v042Expected{{4, 8, 9}})
	})
}

func TestV042LiteralContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"no declaration", []model.Line{v042Text(t, "Read.", 2, 1)}},
		{"editor", []model.Line{v042Open(t, "editor", 2, 1), v042Text(t, "@order 0", 3, 2), v042Close(t, 3, 2), v042Text(t, "Read.", 2, 1)}},
		{"multifill", []model.Line{v042Open(t, "multifill", 2, 1), v042Text(t, "@order 0", 3, 2), v042Close(t, 3, 2)}},
		{"escaped tag", []model.Line{v042Line(t, "\\@order 0", "content", 2, 1, v042Element(t, "content", "\\@order 0", "@order 0", 1, 10))}},
		{"inline mention", []model.Line{v042Text(t, "текст @order 0", 2, 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v042Run(t, v042Document(t, tc.body...), "current", nil) })
	}
	t.Run("unrecognized line", func(t *testing.T) {
		result := v042Document(t, v042Line(t, "#bad @order 0", "invalid", 2, 1,
			model.Element{ElementType: "unparsed", Raw: "#bad @order 0", Start: 1, End: 14, ErrorIDs: []string{}}))
		v042PriorError(t, result, 3, 0, "Неверный заголовок.")
		v042Run(t, result, "current", nil)
	})
	t.Run("other metadata", func(t *testing.T) {
		result := v042Metadata(t, v042Line(t, "@section Name", "tag", 0, 0,
			v042Element(t, "tag", "@section", "section", 1, 9), v042Element(t, "name", "Name", "Name", 10, 14)))
		name := "Name"
		result.Document.Metadata.Section = &name
		v042Run(t, result, "current", nil)
	})
}

func TestV042LineEndingsAndBOM(t *testing.T) {
	for _, tc := range []struct {
		name string
		eol  model.LineEnding
		bom  bool
	}{
		{"LF", "\n", false}, {"CRLF", "\r\n", false}, {"BOM", "\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// EOL и начальный BOM не входят в Raw по контракту модели.
			result := v042Metadata(t, v042Line(t, "@order 2.5", "tag", 0, 0,
				v042Element(t, "tag", "@order", "order", 1, 7),
				model.Element{ElementType: "unparsed", Raw: "2.5", Start: 8, End: 11, ErrorIDs: []string{}}))
			result.Document.HasBOM = &tc.bom
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			v042PriorError(t, result, 2, 1, "Неверный аргумент order.")
			v042Run(t, result, "current", []v042Expected{{2, 8, 11}})
		})
	}
}

func TestV042EmptyLinesAndVersions(t *testing.T) {
	v042Run(t, v042UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v042UnavailableDocument(t, version)
			before := v042Snapshot(t, result)
			checks, err := validator.Prepare(result, v042Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) did not refuse: %v, %v", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v042Snapshot(t, result) != before {
				t.Fatal("model changed on refusal")
			}
		})
	}
}

// v042Order создаёт входное объявление с лексически цифровым аргументом.
func v042Order(t *testing.T, argument string, parent, depth int) model.Line {
	t.Helper()
	return v042Line(t, "@order "+argument, "tag", parent, depth,
		v042Element(t, "tag", "@order", "order", 1, 7),
		v042Element(t, "number", argument, argument, 8, 8+utf8.RuneCountInString(argument)))
}

// v042Metadata вставляет явно заданные метаданные до заголовка по DSL 1.31.2.
// Эталон и пример сохраняются без изменений; добавлены только версия,
// заголовок и обычное содержимое для полноценного документа.
func v042Metadata(t *testing.T, declarations ...model.Line) *model.Result {
	t.Helper()
	result := v042Document(t, v042Text(t, "Read.", 2, 1))
	lines := append([]model.Line{result.Lines[0]}, declarations...)
	lines = append(lines, result.Lines[1:]...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	parent, count := len(declarations)+2, len(lines)
	lines[count-1].ParentLine = &parent
	result.Lines = lines
	result.Document.LineCount = &count
	return result
}

// v042PriorError добавляет согласованную историю парсера на явно выбранный
// элемент. Все вызовы в ситуациях идут в исходном порядке строк и элементов.
func v042PriorError(t *testing.T, result *model.Result, lineNumber, elementIndex int, message string) {
	t.Helper()
	id := fmt.Sprintf("parser-%d", len(result.Diagnostics)+1)
	line := &result.Lines[lineNumber-1]
	element := &line.Elements[elementIndex]
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: message,
		Location: &model.Location{Start: model.Position{Line: lineNumber, Column: element.Start}, End: model.Position{Line: lineNumber, Column: element.End}}, RelatedLocations: []model.Location{},
	})
}

// v042Open создаёт объявление блока без параметров с явными родителем и глубиной.
func v042Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v042Line(t, "@"+tag+" {", "block-start", parent, depth,
		v042Element(t, "tag", "@"+tag, tag, 1, end), v042Element(t, "block-open", "{", "{", end+1, end+2))
}

// v042Close создаёт закрывающую строку указанного блока.
func v042Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v042Line(t, "}", "block-end", parent, depth, v042Element(t, "block-close", "}", "}", 1, 2))
}

// v042Registry создаёт реестр через штатный каталог без подмены регистраций.
func v042Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v042UnavailableDocument сохраняет известную версию при недоступных строках.
func v042UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v042Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v042Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v042Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v042Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v042Element(t, "version", "1.2", "1.2", 14, 17)),
		v042Line(t, "# Unit 1", "heading", 0, 0,
			v042Element(t, "heading-level", "#", "1", 1, 2),
			v042Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}},
		},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       lines,
		Diagnostics: []model.Diagnostic{},
	}
}

// v042Line собирает строку с явно заданной структурой, без классификации Raw.
func v042Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v042Element сохраняет исходное и семантическое значения и явный диапазон.
func v042Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v042Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v042Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v042Line(t, text, "content", parent, depth,
		v042Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v042Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v042Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v042Run проверяет V042 через штатный каталог и конвейер. Диапазоны want
// заданы независимо. Сохраняются исходные данные, история и ссылки элементов;
// Общие флаги и ссылки учитывают полный каталог; исходник и история сохраняются.
func v042Run(t *testing.T, result *model.Result, processingID string, want []v042Expected) {
	t.Helper()
	before := v042Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v042Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v042Snapshot(t, result) != before {
		t.Fatal("Prepare changed model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.Source != processingID {
			t.Fatalf("unexpected source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V042" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V042 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("location = %+v, want %+v", entry.Location, location)
		}
		if entry.Message != "Значение @order не является положительным целым числом." || entry.DiagnosticCode != "V042" ||
			entry.Source != processingID || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected history lengths")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	for _, occurrence := range want {
		expected.Document.HasErrors = true
		line := &expected.Lines[occurrence.line-1]
		line.HasErrors = true
	}
	v042OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v042Snapshot(t, &previous) != v042Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v042OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v042OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V042" {
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

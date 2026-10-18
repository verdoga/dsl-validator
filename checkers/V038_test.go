package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v038Expected задаёт независимые ожидания: строку и Unicode-колонки [start,end).
type v038Expected struct{ line, start, end int }

func TestV038Registration(t *testing.T) {
	definitions, err := v038Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V038" {
			continue
		}
		matches++
		if definition.Severity != "error" || definition.Scope != "element" || definition.Check == nil ||
			definition.Message != "У @document-id отсутствует идентификатор." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V038 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V038 definitions = %d, want 1", matches)
	}
}

func TestV038MissingArgument(t *testing.T) {
	for _, tc := range []struct {
		name, raw, tag string
		start, end     int
	}{
		{"example", "@document-id", "@document-id", 1, 13},
		{"spaces", "@document-id   ", "@document-id", 1, 13},
		{"tabs", "@document-id\t\t", "@document-id", 1, 13},
		{"mixed trailing spaces", "@document-id \t ", "@document-id", 1, 13},
		{"indentation", "\t  @document-id  ", "@document-id", 4, 16},
		{"uppercase", "@DOCUMENT-ID", "@DOCUMENT-ID", 1, 13},
		{"mixed case", "@DoCuMeNt-Id", "@DoCuMeNt-Id", 1, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v038Line(t, tc.raw, "tag", 0, 0, v038Element(t, "tag", tc.tag, "document-id", tc.start, tc.end))
			v038Run(t, v038Metadata(t, line), "current", []v038Expected{{2, tc.start, tc.end}})
		})
	}
}

func TestV038PresentArgument(t *testing.T) {
	for _, tc := range []struct {
		name, raw, id string
		start, end    int
	}{
		{"reference", "@document-id lesson-a", "lesson-a", 14, 22},
		{"surrounding spaces", "@document-id \t lesson-a \t", "lesson-a", 16, 24},
		{"Unicode", "@document-id урок-а", "урок-а", 14, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v038Line(t, tc.raw, "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13),
				v038Element(t, "identifier", tc.id, tc.id, tc.start, tc.end))
			result := v038Metadata(t, line)
			result.Document.Metadata.DocumentID = &tc.id
			v038Run(t, result, "current", nil)
		})
	}
}

func TestV038NonemptyUnparsedArgument(t *testing.T) {
	// DSL 1.9: кавычки, пунктуация, обратная косая черта и пробельные символы
	// недопустимы в ID. Ошибка сохранена в модели, но непустой Raw исключает V038.
	for _, tc := range []struct{ name, rest string }{
		{"quotes", "\"\""}, {"punctuation", "!"}, {"backslash", "\\"},
		{"NBSP", "\u00A0"}, {"em space", "\u2003"}, {"line separator", "\u2028"},
		{"NBSP with ordinary whitespace", "\u00A0 \t"}, {"escaped braces", "\\{\\}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v038Line(t, "@document-id "+tc.rest, "tag", 0, 0,
				v038Element(t, "tag", "@document-id", "document-id", 1, 13),
				model.Element{ElementType: "unparsed", Raw: tc.rest, Start: 14, End: 14 + utf8.RuneCountInString(tc.rest), ErrorIDs: []string{}})
			result := v038Metadata(t, line)
			v038PriorError(t, result, 2, 1, "Недопустимый идентификатор.")
			v038Run(t, result, "current", nil)
		})
	}
}

func TestV038PlacementAndMultipleDeclarations(t *testing.T) {
	t.Run("after heading and argument on next line", func(t *testing.T) {
		result := v038Document(t,
			v038Line(t, "@document-id", "tag", 2, 1, v038Element(t, "tag", "@document-id", "document-id", 1, 13)),
			v038Text(t, "lesson-a", 2, 1))
		v038PriorError(t, result, 3, 0, "Мета-тег расположен после заголовка.")
		v038Run(t, result, "current", []v038Expected{{3, 1, 13}})
	})
	t.Run("inside text block", func(t *testing.T) {
		// DSL 1.24: запрещённый вложенный тег всё ещё является распознанным.
		result := v038Document(t, v038Open(t, "text", 2, 1),
			v038Line(t, "@document-id", "tag", 3, 2, v038Element(t, "tag", "@document-id", "document-id", 1, 13)),
			v038Text(t, "Read.", 3, 2), v038Close(t, 3, 2))
		v038PriorError(t, result, 4, 0, "Вложенный тег запрещён.")
		v038Run(t, result, "current", []v038Expected{{4, 1, 13}})
	})
	t.Run("last line without EOL", func(t *testing.T) {
		result := v038Document(t, v038Text(t, "Read.", 2, 1),
			v038Line(t, "@document-id", "tag", 2, 1, v038Element(t, "tag", "@document-id", "document-id", 1, 13)))
		result.Lines[3].LineEnding = ""
		v038PriorError(t, result, 4, 0, "Мета-тег расположен после заголовка.")
		v038Run(t, result, "current", []v038Expected{{4, 1, 13}})
	})
	t.Run("two missing declarations", func(t *testing.T) {
		result := v038Metadata(t,
			v038Line(t, "@document-id", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13)),
			v038Line(t, "@document-id", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13)))
		v038PriorError(t, result, 3, 0, "Повторное объявление document-id.")
		v038Run(t, result, "current", []v038Expected{{2, 1, 13}, {3, 1, 13}})
	})
	t.Run("filled declaration between missing ones", func(t *testing.T) {
		result := v038Metadata(t,
			v038Line(t, "@document-id", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13)),
			v038Line(t, "@document-id lesson-a", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13), v038Element(t, "identifier", "lesson-a", "lesson-a", 14, 22)),
			v038Line(t, "@document-id", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13)))
		v038PriorError(t, result, 3, 0, "Повторное объявление document-id.")
		v038PriorError(t, result, 4, 0, "Повторное объявление document-id.")
		v038Run(t, result, "current", []v038Expected{{2, 1, 13}, {4, 1, 13}})
	})
}

func TestV038LiteralContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"no declaration", []model.Line{v038Text(t, "Read.", 2, 1)}},
		{"inline editor", []model.Line{v038Line(t, "@editor @document-id", "tag", 2, 1,
			v038Element(t, "tag", "@editor", "editor", 1, 8), v038Element(t, "content", "@document-id", "@document-id", 9, 21)), v038Text(t, "Read.", 2, 1)}},
		{"block editor", []model.Line{v038Open(t, "editor", 2, 1), v038Text(t, "@document-id", 3, 2), v038Close(t, 3, 2), v038Text(t, "Read.", 2, 1)}},
		{"multifill", []model.Line{v038Open(t, "multifill", 2, 1), v038Text(t, "@document-id", 3, 2), v038Close(t, 3, 2)}},
		{"escaped beginning", []model.Line{v038Line(t, "\\@document-id", "content", 2, 1,
			v038Element(t, "content", "\\@document-id", "@document-id", 1, 14))}},
		{"inline mention", []model.Line{v038Text(t, "текст @document-id", 2, 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v038Run(t, v038Document(t, tc.body...), "current", nil) })
	}
	t.Run("other metadata tag", func(t *testing.T) {
		result := v038Metadata(t, v038Line(t, "@section Name", "tag", 0, 0,
			v038Element(t, "tag", "@section", "section", 1, 9), v038Element(t, "name", "Name", "Name", 10, 14)))
		name := "Name"
		result.Document.Metadata.Section = &name
		v038Run(t, result, "current", nil)
	})
	t.Run("whole unparsed line is not a recognized declaration", func(t *testing.T) {
		// Ошибочная попытка заголовка по DSL 1.15: тег в остатке не распознан.
		const raw = "#bad @document-id"
		result := v038Document(t, v038Line(t, raw, "invalid", 2, 1,
			model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: 18, ErrorIDs: []string{}}))
		v038PriorError(t, result, 3, 0, "Неверный заголовок.")
		v038Run(t, result, "current", nil)
	})
}

func TestV038LineEndingsAndBOM(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [4]model.LineEnding
		bom  bool
	}{
		{"LF", [4]model.LineEnding{"\n", "\n", "\n", "\n"}, false},
		{"CRLF", [4]model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n"}, false},
		{"mixed", [4]model.LineEnding{"\n", "\r\n", "\n", ""}, false},
		{"BOM", [4]model.LineEnding{"\n", "\n", "\n", "\n"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.13: BOM и EOL не входят в Raw.
			result := v038Metadata(t, v038Line(t, "@document-id", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13)))
			result.Document.HasBOM = &tc.bom
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v038Run(t, result, "current", []v038Expected{{2, 1, 13}})
		})
	}
}

func TestV038EmptyLinesAndVersions(t *testing.T) {
	v038Run(t, v038UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v038UnavailableDocument(t, version)
			before := v038Snapshot(t, result)
			checks, err := validator.Prepare(result, v038Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) did not refuse: %v, %v", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v038Snapshot(t, result) != before {
				t.Fatal("model changed on refusal")
			}
		})
	}
}

func TestV038RepeatedRunsPreserveErrors(t *testing.T) {
	result := v038Metadata(t, v038Line(t, "@document-id", "tag", 0, 0, v038Element(t, "tag", "@document-id", "document-id", 1, 13)))
	v038PriorError(t, result, 2, 0, "Отсутствует обязательный аргумент.")
	v038Run(t, result, "first", []v038Expected{{2, 1, 13}})
	v038Run(t, result, "second", []v038Expected{{2, 1, 13}})
	if len(result.Lines[1].Elements[0].ErrorIDs) != 3 {
		t.Fatal("prior error and two V038 links must remain")
	}
	v038Run(t, v038Document(t, v038Text(t, "Read.", 2, 1)), "first", nil)
}

// v038Metadata вставляет явно заданные метаданные до заголовка по DSL 1.31.2.
// Эталон и пример сохраняются без изменений; добавлены только версия,
// заголовок и обычное содержимое для полноценного документа.
func v038Metadata(t *testing.T, declarations ...model.Line) *model.Result {
	t.Helper()
	result := v038Document(t, v038Text(t, "Read.", 2, 1))
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

// v038PriorError добавляет согласованную историю парсера на явно выбранный
// элемент. Все вызовы в ситуациях идут в исходном порядке строк и элементов.
func v038PriorError(t *testing.T, result *model.Result, lineNumber, elementIndex int, message string) {
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

// v038Open создаёт объявление блока без параметров с явными родителем и глубиной.
func v038Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v038Line(t, "@"+tag+" {", "block-start", parent, depth,
		v038Element(t, "tag", "@"+tag, tag, 1, end), v038Element(t, "block-open", "{", "{", end+1, end+2))
}

// v038Close создаёт закрывающую строку указанного блока.
func v038Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v038Line(t, "}", "block-end", parent, depth, v038Element(t, "block-close", "}", "}", 1, 2))
}

// v038Registry создаёт реестр через штатный каталог без подмены регистраций.
func v038Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v038UnavailableDocument сохраняет известную версию при недоступных строках.
func v038UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v038Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v038Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v038Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v038Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v038Element(t, "version", "1.2", "1.2", 14, 17)),
		v038Line(t, "# Unit 1", "heading", 0, 0,
			v038Element(t, "heading-level", "#", "1", 1, 2),
			v038Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v038Line собирает строку с явно заданной структурой, без классификации Raw.
func v038Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v038Element сохраняет исходное и семантическое значения и явный диапазон.
func v038Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v038Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v038Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v038Line(t, text, "content", parent, depth,
		v038Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v038Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v038Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v038Run проверяет V038 через штатный каталог и конвейер. Ожидаемые ссылки
// и ошибочные строки заданы диапазонами want, а не вычисляются проверкой.
// Исходные данные и история должны сохраниться; допускаются только новые
// Processing/Diagnostics, ссылки на V038 и соответствующие флаги ошибок.
func v038Run(t *testing.T, result *model.Result, processingID string, want []v038Expected) {
	t.Helper()
	before := v038Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v038Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v038Snapshot(t, result) != before {
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
		if entry.DiagnosticCode == "V038" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V038 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("location = %+v, want %+v", entry.Location, location)
		}
		if entry.Message != "У @document-id отсутствует идентификатор." || entry.DiagnosticCode != "V038" ||
			entry.Source != processingID || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
	}
	if result.Document.HasErrors != (original.Document.HasErrors || len(want) > 0) {
		t.Error("unexpected document error flag")
	}
	if len(result.Lines) != len(original.Lines) {
		t.Fatal("line count changed")
	}
	for i, line := range result.Lines {
		expectedFlag := original.Lines[i].HasErrors
		for _, expected := range want {
			if expected.line == i+1 {
				expectedFlag = true
			}
		}
		if line.HasErrors != expectedFlag {
			t.Errorf("line %d error flag = %v, want %v", i+1, line.HasErrors, expectedFlag)
		}
		if len(line.Elements) != len(original.Lines[i].Elements) {
			t.Fatalf("line %d element count changed", i+1)
		}
		for j, element := range line.Elements {
			expectedLinks := append([]string{}, original.Lines[i].Elements[j].ErrorIDs...)
			for k, expected := range want {
				if expected.line == i+1 && expected.start == element.Start && expected.end == element.End {
					if element.ElementType != "tag" {
						t.Error("V038 target is not a tag")
					}
					expectedLinks = append(expectedLinks, got[k].ID)
				}
			}
			if element.ErrorIDs == nil || !slices.Equal(element.ErrorIDs, expectedLinks) {
				t.Errorf("line %d element %d links = %v, want %v", i+1, j, element.ErrorIDs, expectedLinks)
			}
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("lost history or unexpected processing")
	}
	// После отдельной проверки разрешённых изменений сравниваем весь остаток
	// модели со снимком, не меняя проверяемый result.
	var previous model.Result
	if err := json.Unmarshal([]byte(v038Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	previous.Processing = previous.Processing[:processingCount]
	previous.Diagnostics = previous.Diagnostics[:diagnosticCount]
	previous.Document.HasErrors = original.Document.HasErrors
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range previous.Lines[i].Elements {
			previous.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	if v038Snapshot(t, &previous) != before {
		t.Fatal("source model or previous history changed")
	}
}

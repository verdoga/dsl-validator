package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

type v036Expected struct {
	line, start, end int
	detail           string
}

func TestV036Registration(t *testing.T) {
	definitions, err := v036Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V036" {
			continue
		}
		count++
		if definition.Message != "Метатег находится вне области метаданных." || definition.Severity != "error" ||
			definition.Scope != "element" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V036 definitions = %d, want 1", count)
	}
}

func TestV036ReferenceAndExample(t *testing.T) {
	// Оба примера уже содержат версию и заголовок; дополнительного контекста нет.
	for _, example := range []bool{false, true} {
		t.Run(map[bool]string{false: "reference", true: "example"}[example], func(t *testing.T) {
			section := v036Tag(t, "section", "section", "name", "Studentbook", "", 0, 0)
			heading := v036Heading(t, "#", "1")
			lines := []model.Line{v036Version(t, 0, 0), v036Tag(t, "document-id", "document-id", "identifier", "lesson-a", "", 0, 0), section, heading}
			var want []v036Expected
			if example {
				lines[2] = heading
				lines[3] = v036Tag(t, "section", "section", "name", "Studentbook", "", 3, 1)
				want = []v036Expected{{4, 1, 9, "После первого заголовка уровня 1"}}
			}
			result := v036Model(t, lines...)
			id, title, name := "lesson-a", "Lesson", "Studentbook"
			result.Document.Metadata.DocumentID = &id
			result.Document.Metadata.Title = &title
			if !example {
				result.Document.Metadata.Section = &name
			}
			v036Run(t, result, "current", want)
		})
	}
}

func TestV036Placement(t *testing.T) {
	version := func() model.Line { return v036Version(t, 0, 0) }
	heading := func() model.Line { return v036Heading(t, "#", "1") }
	section := func(parent, depth int) model.Line {
		return v036Tag(t, "section", "section", "name", "Studentbook", "", parent, depth)
	}
	for _, tc := range []struct {
		name  string
		lines []model.Line
		want  []v036Expected
	}{
		{"empty", nil, nil},
		{"no metadata", []model.Line{version(), heading()}, nil},
		{"immediately before version", []model.Line{section(0, 0), version(), heading()}, []v036Expected{{1, 1, 9, "До корневого @dsl-version"}}},
		{"between boundaries", []model.Line{version(), section(0, 0), heading()}, nil},
		{"no heading and last line", []model.Line{version(), section(0, 0)}, nil},
		// Версия запуска известна из Document по контракту Prepare; проверка
		// всё равно обязана обнаружить отсутствие корневого объявления в Lines.
		{"no root version", []model.Line{section(0, 0)}, []v036Expected{{1, 1, 9, "Не найден корневой @dsl-version"}}},
		{"missing version and after heading", []model.Line{heading(), section(1, 1)}, []v036Expected{{2, 1, 9, "Не найден корневой @dsl-version; После первого заголовка уровня 1"}}},
		{"heading parent is not a block", []model.Line{version(), heading(), section(2, 1)}, []v036Expected{{3, 1, 9, "После первого заголовка уровня 1"}}},
		{"first root version remains boundary", []model.Line{version(), section(0, 0), version(), heading()}, nil},
		{"first heading remains boundary", []model.Line{version(), heading(), section(2, 1), heading()}, []v036Expected{{3, 1, 9, "После первого заголовка уровня 1"}}},
		{"level two does not close metadata", []model.Line{version(), v036Heading(t, "##", "2"), section(2, 1)}, nil},
		{"level three does not close metadata", []model.Line{version(), v036Heading(t, "###", "3"), section(2, 1)}, nil},
		{"comment and blank allowed", []model.Line{version(), v036Line(t, "", "blank", 0, 0),
			v036Tag(t, "editor", "editor", "content", "Comment.", "", 0, 0), section(0, 0), heading()}, nil},
		{"closed editor before metadata", []model.Line{version(), v036Open(t, "editor", 0, 0),
			v036Text(t, "Comment.", 2, 1), v036Close(t, 2, 1), section(0, 0), heading()}, nil},
		{"canonical value", []model.Line{version(), heading(), v036Tag(t, "Section", "section", "name", "Раздел🙂", "", 2, 1)}, []v036Expected{{3, 1, 9, "После первого заголовка уровня 1"}}},
		{"canonical version", []model.Line{v036Tag(t, "DSL-VERSION", "dsl-version", "version", "1.2", "", 0, 0), section(0, 0), heading()}, nil},
		{"original indentation", []model.Line{version(), heading(), v036Tag(t, "section", "section", "name", "Раздел", " \t", 2, 1)}, []v036Expected{{3, 3, 11, "После первого заголовка уровня 1"}}},
		{"other tag", []model.Line{version(), heading(), v036Tag(t, "note", "note", "content", "Read.", "", 2, 1)}, nil},
		{"names are not declarations", []model.Line{version(), v036Tag(t, "section", "section", "name", "@order Studentbook", "", 0, 0), heading()}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v036Run(t, v036Model(t, tc.lines...), "current", tc.want) })
	}
}

func TestV036AllMetadataAndDuplicates(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "all allowed", true: "all after heading"}[after], func(t *testing.T) {
			lines := []model.Line{v036Version(t, 0, 0)}
			parent, depth := 0, 0
			if after {
				lines = append(lines, v036Heading(t, "#", "1"))
				parent, depth = 2, 1
			}
			lines = append(lines,
				v036Tag(t, "document-id", "document-id", "identifier", "lesson-a", "", parent, depth),
				v036Tag(t, "section", "section", "name", "Studentbook", "", parent, depth),
				v036Tag(t, "order", "order", "number", "10", "", parent, depth),
				v036Tag(t, "resource-dir", "resource-dir", "resource-path", "assets", "", parent, depth))
			var want []v036Expected
			if after {
				want = []v036Expected{{3, 1, 13, "После первого заголовка уровня 1"}, {4, 1, 9, "После первого заголовка уровня 1"},
					{5, 1, 7, "После первого заголовка уровня 1"}, {6, 1, 14, "После первого заголовка уровня 1"}}
			} else {
				lines = append(lines, v036Heading(t, "#", "1"))
			}
			result := v036Model(t, lines...)
			if !after {
				id, name, order := "lesson-a", "Studentbook", "10"
				result.Document.Metadata.DocumentID, result.Document.Metadata.Section, result.Document.Metadata.Order = &id, &name, &order
				result.Document.Metadata.ResourceDirs = []string{"assets"}
			}
			v036Run(t, result, "current", want)
		})
	}
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "repeated paths allowed", true: "each misplaced path reported"}[after], func(t *testing.T) {
			lines := []model.Line{v036Version(t, 0, 0)}
			parent, depth := 0, 0
			if after {
				lines = append(lines, v036Heading(t, "#", "1"))
				parent, depth = 2, 1
			}
			for _, path := range []string{"assets", "images"} {
				lines = append(lines, v036Tag(t, "resource-dir", "resource-dir", "resource-path", path, "", parent, depth))
			}
			var want []v036Expected
			if after {
				want = []v036Expected{{3, 1, 14, "После первого заголовка уровня 1"}, {4, 1, 14, "После первого заголовка уровня 1"}}
			}
			result := v036Model(t, lines...)
			if !after {
				result.Document.Metadata.ResourceDirs = []string{"assets", "images"}
			}
			v036Run(t, result, "current", want)
		})
	}
	t.Run("duplicate section is not a placement error", func(t *testing.T) {
		v036Run(t, v036Model(t, v036Version(t, 0, 0),
			v036Tag(t, "section", "section", "name", "One", "", 0, 0),
			v036Tag(t, "section", "section", "name", "Two", "", 0, 0), v036Heading(t, "#", "1")), "current", nil)
	})
}

func TestV036Blocks(t *testing.T) {
	section := func(parent, depth int) model.Line {
		return v036Tag(t, "section", "section", "name", "Studentbook", "", parent, depth)
	}
	for _, tc := range []struct {
		name  string
		lines []model.Line
		want  []v036Expected
	}{
		{"after heading inside note", []model.Line{v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Open(t, "note", 2, 1),
			section(3, 2), v036Close(t, 3, 2)}, []v036Expected{{4, 1, 9, "После первого заголовка уровня 1; Внутри фигурного блока"}}},
		{"nested blocks one message", []model.Line{v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Open(t, "instr", 2, 1),
			v036Text(t, "Read.", 3, 2), v036Open(t, "hint", 3, 2), section(5, 3), v036Close(t, 5, 3), v036Close(t, 3, 2)},
			[]v036Expected{{6, 1, 9, "После первого заголовка уровня 1; Внутри фигурного блока"}}},
		{"after block closes", []model.Line{v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Open(t, "note", 2, 1),
			v036Text(t, "Read.", 3, 2), v036Close(t, 3, 2), section(2, 1)}, []v036Expected{{6, 1, 9, "После первого заголовка уровня 1"}}},
		// Эти входы моделируют распознанные объявления при ошибочном расположении
		// блока: границы метаданных и наличие предка проверяются независимо.
		{"only block reason", []model.Line{v036Version(t, 0, 0), v036Open(t, "note", 0, 0), section(2, 1), v036Close(t, 2, 1)},
			[]v036Expected{{3, 1, 9, "Внутри фигурного блока"}}},
		{"before version and in block", []model.Line{v036Open(t, "note", 0, 0), section(1, 1), v036Close(t, 1, 1), v036Version(t, 0, 0)},
			[]v036Expected{{2, 1, 9, "До корневого @dsl-version; Внутри фигурного блока"}}},
		{"all reasons", []model.Line{v036Heading(t, "#", "1"), v036Open(t, "note", 1, 1), section(2, 2), v036Close(t, 2, 2)},
			[]v036Expected{{3, 1, 9, "Не найден корневой @dsl-version; После первого заголовка уровня 1; Внутри фигурного блока"}}},
		{"nested version is not root", []model.Line{v036Heading(t, "#", "1"), v036Open(t, "instr", 1, 1), v036Version(t, 2, 2),
			v036Close(t, 2, 2), section(1, 1)}, []v036Expected{{5, 1, 9, "Не найден корневой @dsl-version; После первого заголовка уровня 1"}}},
		{"nested version before actual root", []model.Line{v036Open(t, "note", 0, 0), v036Version(t, 1, 1), v036Close(t, 1, 1),
			section(0, 0), v036Version(t, 0, 0)}, []v036Expected{{4, 1, 9, "До корневого @dsl-version"}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v036Run(t, v036Model(t, tc.lines...), "current", tc.want) })
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v036Run(t, v036Model(t, v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Open(t, tag, 2, 1),
				v036Text(t, "@section Studentbook", 3, 2), v036Text(t, "@order 10", 3, 2), v036Close(t, 3, 2)), "current", nil)
		})
	}
	t.Run("ordinary text is not a declaration", func(t *testing.T) {
		v036Run(t, v036Model(t, v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Open(t, "text", 2, 1),
			v036Text(t, "Mention @section Studentbook.", 3, 2), v036Close(t, 3, 2)), "current", nil)
	})
}

func TestV036Unparsed(t *testing.T) {
	for _, tc := range []struct {
		name       string
		line       model.Line
		start, end int
		want       []v036Expected
	}{
		{"whole unparsed", v036Line(t, "@section Studentbook", "invalid", 2, 1,
			model.Element{ElementType: "unparsed", Raw: "@section Studentbook", Start: 1, End: 21, ErrorIDs: []string{"parser-error"}}), 1, 21, nil},
		{"recognized tag with invalid value", v036Line(t, "@order zero", "tag", 2, 1,
			v036Element(t, "tag", "@order", "order", 1, 7),
			model.Element{ElementType: "unparsed", Raw: "zero", Start: 8, End: 12, ErrorIDs: []string{"parser-error"}}), 8, 12,
			[]v036Expected{{3, 1, 7, "После первого заголовка уровня 1"}}},
		{"recognized tag on invalid line", v036Line(t, "@order zero", "invalid", 2, 1,
			v036Element(t, "tag", "@order", "order", 1, 7),
			model.Element{ElementType: "unparsed", Raw: "zero", Start: 8, End: 12, ErrorIDs: []string{"parser-error"}}), 8, 12,
			[]v036Expected{{3, 1, 7, "После первого заголовка уровня 1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v036Model(t, v036Version(t, 0, 0), v036Heading(t, "#", "1"), tc.line)
			result.Document.HasErrors, result.Lines[2].HasErrors = true, true
			result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Не удалось разобрать фрагмент.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}}}
			v036Run(t, result, "current", tc.want)
		})
	}
}

func TestV036EndingsVersionsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, finalEOL := range []model.LineEnding{eol, ""} {
			t.Run("endings/"+string(eol)+"/"+string(finalEOL), func(t *testing.T) {
				result := v036Model(t, v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Tag(t, "section", "section", "name", "Studentbook", "", 2, 1))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				result.Lines[2].LineEnding = finalEOL
				v036Run(t, result, "current", []v036Expected{{3, 1, 9, "После первого заголовка уровня 1"}})
			})
		}
	}
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v036Model(t, v036Version(t, 0, 0))
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			} else {
				result.Lines[0].Raw = "@dsl-version " + version
				result.Lines[0].Elements[1].Raw = version
				result.Lines[0].Elements[1].Value = &version
			}
			before := v036Snapshot(t, result)
			checks, err := validator.Prepare(result, v036Registry(t))
			if err == nil || checks != nil || v036Snapshot(t, result) != before {
				t.Fatal("unsupported version accepted or model changed")
			}
		})
	}
	t.Run("repeated runs preserve history and links", func(t *testing.T) {
		result := v036Model(t, v036Version(t, 0, 0), v036Heading(t, "#", "1"), v036Tag(t, "Section", "section", "name", "Studentbook", "", 2, 1))
		want := []v036Expected{{3, 1, 9, "После первого заголовка уровня 1"}}
		v036Run(t, result, "first", want)
		v036Run(t, result, "second", want)
		for _, source := range []string{"first", "second"} {
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source == source && entry.DiagnosticCode == "V031" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("V031 count for %s = %d, want 1", source, count)
			}
		}
	})
}

// v036Registry использует единственный штатный каталог регистраций.
func v036Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v036Model задаёт номера физических строк, не распознавая исходный DSL.
func v036Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, count := "1.2", "UTF-8", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	return result
}

// v036Line сохраняет явно указанную классификацию и структуру строки.
func v036Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v036Element задаёт независимый входной элемент и диапазон [start,end).
func v036Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v036Tag строит объявление из явно заданных элементов; вычисляемая длина
// применяется только к входу, а ожидаемые диапазоны записаны в тестах числами.
func v036Tag(t *testing.T, spelling, canonical string, field model.ElementType, value, prefix string, parent, depth int) model.Line {
	t.Helper()
	start := utf8.RuneCountInString(prefix) + 1
	end := start + utf8.RuneCountInString(spelling) + 1
	return v036Line(t, prefix+"@"+spelling+" "+value, "tag", parent, depth,
		v036Element(t, "tag", "@"+spelling, canonical, start, end),
		v036Element(t, field, value, value, end+1, end+1+utf8.RuneCountInString(value)))
}

// v036Version задаёт распознанное объявление версии.
func v036Version(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v036Tag(t, "dsl-version", "dsl-version", "version", "1.2", "", parent, depth)
}

// v036Heading задаёт заголовок указанного уровня с точным текстом эталона.
func v036Heading(t *testing.T, marker, level string) model.Line {
	t.Helper()
	return v036Line(t, marker+" Lesson", "heading", 0, 0, v036Element(t, "heading-level", marker, level, 1, len(marker)+1),
		v036Element(t, "title", "Lesson", "Lesson", len(marker)+2, len(marker)+8))
}

// v036Open задаёт объявление фигурного блока и его родителя.
func v036Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := len(tag) + 2
	return v036Line(t, "@"+tag+" {", "block-start", parent, depth, v036Element(t, "tag", "@"+tag, tag, 1, end),
		v036Element(t, "block-open", "{", "{", end+1, end+2))
}

// v036Close задаёт закрытие с явно указанным родителем.
func v036Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v036Line(t, "}", "block-end", parent, depth, v036Element(t, "block-close", "}", "}", 1, 2))
}

// v036Text задаёт литеральное содержимое, не разбирая похожие на теги слова.
func v036Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v036Line(t, raw, "content", parent, depth, v036Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v036Run проверяет результаты текущего штатного запуска, полную историю,
// исходные данные, флаги и добавление ErrorIDs исключительно к ожидаемым тегам.
func v036Run(t *testing.T, result *model.Result, source string, want []v036Expected) {
	t.Helper()
	before := v036Snapshot(t, result)
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v036Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v036Snapshot(t, result) != before {
		t.Fatal("Prepare changed model")
	}
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V036" && entry.Source != source {
			t.Fatal("incorrect V036 source")
		}
		if entry.DiagnosticCode == "V036" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V036 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, occurrence := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: occurrence.line, Column: occurrence.start}, End: model.Position{Line: occurrence.line, Column: occurrence.end}}
		if entry.Location == nil || *entry.Location != location || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "Метатег находится вне области метаданных.\nконкретизация: "+occurrence.detail || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v at %+v; want %+v, detail %q", i, entry, entry.Location, location, occurrence.detail)
		}
		expected.Document.HasErrors = true
		line := &expected.Lines[occurrence.line-1]
		line.HasErrors = true
		matched := 0
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "tag" && element.Start == occurrence.start && element.End == occurrence.end {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched++
			}
		}
		if matched != 1 {
			t.Fatal("expected location does not identify one tag")
		}
	}
	if len(result.Processing) != processingCount+1 || result.Processing[processingCount].ID != source || result.Processing[processingCount].Tool != "dsl-validator" || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected history lengths or processing entry")
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v036Snapshot(t, &previous) != v036Snapshot(t, &expected) {
		t.Fatal("source, history, error flags or element links differ from expected changes")
	}
}

// v036Snapshot сохраняет полную модель для независимого сравнения изменений.
func v036Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

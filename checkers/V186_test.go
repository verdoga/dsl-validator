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

// v186Expected задаёт независимые ожидания исходных диапазонов [start,end).
type v186Expected struct {
	line, start, end int
	related          [][3]int
}

func TestV186Registration(t *testing.T) {
	definitions, err := v186Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V186" {
			continue
		}
		count++
		if definition.Message != "Внутри @ordering записан заголовок." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V186 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v186EmptyModel(t, version)
			before := v186Snapshot(t, result)
			checks, err := validator.Prepare(result, v186Registry(t))
			if err == nil || checks != nil || v186Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV186ReferenceAndExample(t *testing.T) {
	// Дословным фрагментам добавлен только префикс версии, ID и урока.
	// Вложенный заголовок — invalid: модель не допускает heading с родителем.
	for _, tc := range []struct {
		name, raw string
		rows      []model.Line
		want      []v186Expected
	}{
		{"reference", "@ordering {\n\\# First\nB\n}", []model.Line{
			v186TextLine(t, `\# First`, "# First", 4, 2), v186TextLine(t, "B", "B", 4, 2)}, nil},
		{"example", "@ordering {\n# Stages\nA\nB\n}", []model.Line{
			v186Invalid(t, "# Stages", 4, 2), v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2)},
			[]v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v186OrderingDocument(t, tc.rows...)
			var raw strings.Builder
			for _, line := range result.Lines[3:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.raw {
				t.Fatalf("source fragment changed: %q", raw.String())
			}
			v186Run(t, result, "current", tc.want)
		})
	}
}

func TestV186InvalidHeadings(t *testing.T) {
	for _, tc := range []struct {
		name, raw  string
		start, end int
	}{
		{"level one", "# Stages", 1, 9}, {"level two", "## Stages", 1, 10}, {"level three", "### Stages", 1, 11},
		{"tab separator", "#\tStages", 1, 9}, {"NBSP separator", "#\u00a0Stages", 1, 9},
		{"em space separator", "#\u2003Stages", 1, 9}, {"several separators", "# \t Stages", 1, 11},
		{"no title space", "# ", 1, 2}, {"no title tab", "#\t", 1, 2},
		{"outer space and tab", " \t# Stages \t", 3, 11}, {"Unicode title", "# Этап🙂", 1, 8},
		{"trailing NBSP preserved", "# Stages\u00a0", 1, 10},
		{"missing separator", "#Stages", 0, 0}, {"bare marker", "#", 0, 0},
		{"zero width space is not separator", "#\u200bStages", 0, 0},
		{"four markers", "#### Stages", 0, 0}, {"five markers", "##### Stages", 0, 0},
		{"four bare markers", "####", 0, 0}, {"NBSP is not indentation", "\u00a0# Stages", 0, 0},
		{"escaped marker", `\# Stages`, 0, 0}, {"escaped with indentation", " \t\\## Stages", 0, 0},
		{"middle of text", "Text # Stages", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []v186Expected
			if tc.start != 0 {
				want = []v186Expected{{5, tc.start, tc.end, [][3]int{{4, 1, 10}}}}
			}
			v186Run(t, v186WithHeading(t, v186Invalid(t, tc.raw, 4, 2)), "current", want)
		})
	}
	// Полный перечень White_Space без CR/LF: эти два символа относятся к EOL.
	for _, separator := range []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		t.Run(fmt.Sprintf("White_Space U+%04X", separator), func(t *testing.T) {
			v186Run(t, v186WithHeading(t, v186Invalid(t, "#"+string(separator)+"Stages", 4, 2)), "current",
				[]v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}})
		})
	}
}

func TestV186PreservedElements(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		end  int
	}{
		{"marker retained", v186Line(t, "# Stages", "invalid", 4, 2,
			v186TestElement(t, "heading-level", "#", "1", 1, 2), v186Unparsed(t, "Stages", 3, 9)), 9},
		{"marker and title retained before bad escape", v186Line(t, `# Stages\q`, "invalid", 4, 2,
			v186TestElement(t, "heading-level", "#", "1", 1, 2), v186TestElement(t, "title", "Stages", "Stages", 3, 9),
			v186Unparsed(t, `\q`, 9, 11)), 11},
		{"recognized marker with missing separator", v186Line(t, "#Stages", "invalid", 4, 2,
			v186TestElement(t, "heading-level", "#", "1", 1, 2), v186Unparsed(t, "Stages", 2, 8)), 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v186Run(t, v186WithHeading(t, tc.line), "current", []v186Expected{{5, 1, tc.end, [][3]int{{4, 1, 10}}}})
		})
	}
}

func TestV186OrdinaryText(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\# Stages`, "# Stages"}, {" \t\\## Stages", "## Stages"}, {"Text # Stages", "Text # Stages"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v186Run(t, v186WithHeading(t, v186TextLine(t, tc.raw, tc.value, 4, 2)), "current", nil)
		})
	}
}

func TestV186FormatsAndOccurrences(t *testing.T) {
	for _, title := range []string{"# Stages", "# stages"} {
		t.Run("separate occurrences/"+title, func(t *testing.T) {
			v186Run(t, v186OrderingDocument(t, v186Invalid(t, "# Stages", 4, 2), v186Invalid(t, title, 4, 2),
				v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2)), "current",
				[]v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}, {6, 1, 9, [][3]int{{4, 1, 10}}}})
		})
	}
	t.Run("expanded both sides", func(t *testing.T) {
		v186Run(t, v186OrderingDocument(t, v186Invalid(t, "# Stages", 4, 2), v186TextLine(t, "A", "A", 4, 2),
			v186TextLine(t, "B", "B", 4, 2), v186Line(t, "---", "separator", 4, 2),
			v186Invalid(t, "# Stages", 4, 2), v186TextLine(t, "P", "P", 4, 2), v186TextLine(t, "Q", "Q", 4, 2)), "current",
			[]v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}, {9, 1, 9, [][3]int{{4, 1, 10}}}})
	})
	t.Run("expanded lower list", func(t *testing.T) {
		v186Run(t, v186OrderingDocument(t, v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2),
			v186Line(t, "---", "separator", 4, 2), v186Invalid(t, "# Stages", 4, 2),
			v186TextLine(t, "P", "P", 4, 2), v186TextLine(t, "Q", "Q", 4, 2)), "current",
			[]v186Expected{{8, 1, 9, [][3]int{{4, 1, 10}}}})
	})
	t.Run("last child", func(t *testing.T) {
		v186Run(t, v186OrderingDocument(t, v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2),
			v186Invalid(t, "# Stages", 4, 2)), "current", []v186Expected{{7, 1, 9, [][3]int{{4, 1, 10}}}})
	})
	t.Run("independent ordering blocks", func(t *testing.T) {
		v186Run(t, v186Document(t, v186Open(t, "ordering", "", 3, 1), v186Invalid(t, "# Stages", 4, 2),
			v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2), v186Close(t, 4, 2),
			v186Open(t, "ordering", "", 3, 1), v186Invalid(t, "# Stages", 9, 2),
			v186TextLine(t, "C", "C", 9, 2), v186TextLine(t, "D", "D", 9, 2), v186Close(t, 9, 2)), "current",
			[]v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}, {10, 1, 9, [][3]int{{9, 1, 10}}}})
	})
	t.Run("escaped lower list heading is a position", func(t *testing.T) {
		v186Run(t, v186OrderingDocument(t, v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2),
			v186Line(t, "---", "separator", 4, 2), v186TextLine(t, `\# First`, "# First", 4, 2),
			v186TextLine(t, "Second", "Second", 4, 2)), "current", nil)
	})
}

func TestV186Containers(t *testing.T) {
	for _, name := range []string{"answer", "hint", "editor"} {
		t.Run("nested "+name, func(t *testing.T) {
			rows := []model.Line{v186TextLine(t, "A", "A", 4, 2)}
			parent := 6
			if name == "hint" {
				rows = append(rows, v186TagLine(t, "answer", "content", "1", 4, 2))
				parent = 7
			}
			child := v186Invalid(t, "# Stages", parent, 3)
			if name == "editor" {
				child = v186TextLine(t, "# Stages", "# Stages", parent, 3)
			}
			rows = append(rows, v186Open(t, name, "", 4, 2), child)
			if name == "answer" {
				rows = append(rows, v186TextLine(t, "1", "1", parent, 3))
			}
			rows = append(rows, v186Close(t, parent, 3), v186TextLine(t, "B", "B", 4, 2))
			if name != "editor" {
				rows = append(rows, v186TagLine(t, "answer", "content", "2", 4, 2))
			}
			v186Run(t, v186OrderingDocument(t, rows...), "current", nil)
		})
	}
	t.Run("nested ordering reports only its own heading", func(t *testing.T) {
		v186Run(t, v186OrderingDocument(t, v186Open(t, "ordering", "", 4, 2), v186Invalid(t, "# Stages", 5, 3),
			v186TextLine(t, "A", "A", 5, 3), v186TextLine(t, "B", "B", 5, 3), v186Close(t, 5, 3),
			v186TextLine(t, "C", "C", 4, 2), v186TextLine(t, "D", "D", 4, 2)), "current",
			[]v186Expected{{6, 1, 9, [][3]int{{5, 1, 10}}}})
	})
	t.Run("ordering wrongly nested in fragment", func(t *testing.T) {
		fragment := v186Line(t, "@fragment f {", "block-start", 3, 1,
			v186TestElement(t, "tag", "@fragment", "fragment", 1, 10), v186TestElement(t, "identifier", "f", "f", 11, 12),
			v186TestElement(t, "block-open", "{", "{", 13, 14))
		v186Run(t, v186Document(t, fragment, v186Open(t, "ordering", "", 4, 2), v186Invalid(t, "# Stages", 5, 3),
			v186TextLine(t, "A", "A", 5, 3), v186TextLine(t, "B", "B", 5, 3), v186Close(t, 5, 3), v186Close(t, 4, 2)),
			"current", []v186Expected{{6, 1, 9, [][3]int{{5, 1, 10}}}})
	})
	t.Run("logical parent does not hide brace container", func(t *testing.T) {
		// Ошибочно вложенный логический этап не заменяет фигурный контейнер.
		v186Run(t, v186OrderingDocument(t, v186TagLine(t, "step", "title", "Stage", 4, 2),
			v186Invalid(t, "# Stages", 5, 3), v186TextLine(t, "A", "A", 5, 3), v186TextLine(t, "B", "B", 5, 3)),
			"current", []v186Expected{{6, 1, 9, [][3]int{{4, 1, 10}}}})
	})
	t.Run("case indentation and instruction in declaration", func(t *testing.T) {
		opening := v186Line(t, " \t@OrDeRiNg Order. {", "block-start", 3, 1,
			v186TestElement(t, "tag", "@OrDeRiNg", "ordering", 3, 12), v186TestElement(t, "content", "Order.", "Order.", 13, 19),
			v186TestElement(t, "block-open", "{", "{", 20, 21))
		v186Run(t, v186Document(t, opening, v186Invalid(t, "# Stages", 4, 2), v186TextLine(t, "A", "A", 4, 2),
			v186TextLine(t, "B", "B", 4, 2), v186Close(t, 4, 2)), "current", []v186Expected{{5, 1, 9, [][3]int{{4, 3, 12}}}})
	})
	t.Run("heading after closed ordering", func(t *testing.T) {
		v186Run(t, v186Document(t, v186Open(t, "ordering", "", 3, 1), v186TextLine(t, "A", "A", 4, 2),
			v186TextLine(t, "B", "B", 4, 2), v186Close(t, 4, 2),
			v186Line(t, "# Stages", "heading", 0, 0, v186TestElement(t, "heading-level", "#", "1", 1, 2),
				v186TestElement(t, "title", "Stages", "Stages", 3, 9))), "current", nil)
	})
	t.Run("invalid outside any block", func(t *testing.T) {
		v186Run(t, v186Document(t, v186Invalid(t, "# Stages", 0, 0)), "current", nil)
	})
}

func TestV186EmptyEndingsAndHistory(t *testing.T) {
	v186Run(t, v186EmptyModel(t, "1.2"), "current", nil)
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", ending), func(t *testing.T) {
			result := v186WithHeading(t, v186Invalid(t, "# Stages", 4, 2))
			if ending != "" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v186Run(t, result, "first", []v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}})
			v186Run(t, result, "second", []v186Expected{{5, 1, 9, [][3]int{{4, 1, 10}}}})
		})
	}
}

// v186Invalid задаёт неразобранную строку, не восстанавливая дерево парсера.
func v186Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v186Line(t, raw, "invalid", parent, depth, v186Unparsed(t, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v186WithHeading сохраняет два обычных элемента независимо от заголовка.
func v186WithHeading(t *testing.T, heading model.Line) *model.Result {
	t.Helper()
	return v186OrderingDocument(t, heading, v186TextLine(t, "A", "A", 4, 2), v186TextLine(t, "B", "B", 4, 2))
}

// v186OrderingDocument обрамляет явные строки блоком ordering на строке 4.
func v186OrderingDocument(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v186Open(t, "ordering", "", 3, 1)}, rows...)
	body = append(body, v186Close(t, 4, 2))
	return v186Document(t, body...)
}

// v186Registry создаёт реестр из полного штатного каталога регистраций.
func v186Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v186EmptyModel создаёт модель без исходных строк и с явно указанной версией.
func v186EmptyModel(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v186Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v186Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v186EmptyModel(t, "1.2")
	title, id := "Lesson", "v186"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v186TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v186TagLine(t, "document-id", "identifier", "v186", 0, 0),
		v186Line(t, "# Lesson", "heading", 0, 0, v186TestElement(t, "heading-level", "#", "1", 1, 2), v186TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			errorID := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{errorID}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: errorID, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v186Line сохраняет явно заданные данные физической строки и её родителя.
func v186Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v186TestElement задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v186TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v186Unparsed задаёт фрагмент без значения; историю добавляет v186Document.
func v186Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v186TagLine создаёт объявление с явно заданным типом аргумента.
func v186TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v186TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v186TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v186Line(t, raw, "tag", parent, depth, elements...)
}

// v186TextLine задаёт обычную строку с независимо указанным семантическим значением.
func v186TextLine(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v186Line(t, raw, "content", parent, depth, v186TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v186Open задаёт блочное объявление с необязательной инструкцией.
func v186Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v186TagLine(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v186TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v186Close задаёт отдельную закрывающую скобку с явным родителем.
func v186Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v186Line(t, "}", "block-end", parent, depth, v186TestElement(t, "block-close", "}", "}", 1, 2))
}

// v186Run запускает неизменённую выборку штатного каталога и проверяет новые V186.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v186Run(t *testing.T, result *model.Result, source string, want []v186Expected) {
	t.Helper()
	before := v186Snapshot(t, result)
	checks, err := validator.Prepare(result, v186Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v186Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("unexpected new source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V186" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V186 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Внутри @ordering записан заголовок." {
			t.Errorf("occurrence = %+v, want range %+v and exact V186 metadata", entry, location)
		}
		var related []model.Location
		for _, span := range expected.related {
			related = append(related, model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[0], Column: span[2]}})
		}
		if !slices.Equal(entry.RelatedLocations, related) {
			t.Errorf("related = %+v, want %+v", entry.RelatedLocations, related)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Error("line diagnostic must not create element links")
				}
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v186Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Lines) != len(original.Lines) || len(actual.Processing) != oldProcessing+1 {
		t.Fatal("source or history length changed")
	}
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		if len(actual.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v186Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v186Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v186Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

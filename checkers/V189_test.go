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

// v189Expected задаёт независимый исходный диапазон [start,end) в Unicode.
type v189Expected struct{ line, start, end int }

func TestV189Registration(t *testing.T) {
	definitions, err := v189Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V189" {
			continue
		}
		count++
		if definition.Message != "У @variant отсутствует название ветви." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V189 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v189EmptyModel(t, version)
			before := v189Snapshot(t, result)
			checks, err := validator.Prepare(result, v189Registry(t))
			if err == nil || checks != nil || v189Snapshot(t, result) != before {
				t.Fatalf("unsupported version was not refused without mutation: %v", err)
			}
		})
	}
}

func TestV189ReferenceAndExample(t *testing.T) {
	// Добавлен только технический префикс версии, ID и урока.
	// Оба фрагмента таблицы сохраняются дословно; variants начинается на строке 4.
	for _, tc := range []struct {
		name, raw     string
		first, second model.Line
		want          []v189Expected
	}{
		{"reference", "@variants {\n@variant Student A\nRead.\n@variant Student B\nWrite.\n}",
			v189TagLine(t, "variant", "name", "Student A", 4, 2), v189TagLine(t, "variant", "name", "Student B", 4, 2), nil},
		{"example", "@variants {\n@variant\nRead.\n@variant B\nWrite.\n}",
			v189TagLine(t, "variant", "", "", 4, 2), v189TagLine(t, "variant", "name", "B", 4, 2), []v189Expected{{5, 1, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v189Branches(t, tc.first, tc.second)
			var raw strings.Builder
			for _, line := range result.Lines[3:] {
				raw.WriteString(line.Raw)
				raw.WriteString(string(line.LineEnding))
			}
			if raw.String() != tc.raw {
				t.Fatalf("source fragment changed: %q", raw.String())
			}
			v189Run(t, result, "current", tc.want)
		})
	}
}

func TestV189Names(t *testing.T) {
	for _, name := range []string{"Student A", "Student  A", "Student\tA", "A", "0", ".", "!", "Ветка 🙂", "\u200b", "A\u00a0B", "\u00a0A\u00a0", "A @variant", "NAME {literal}"} {
		t.Run("nonempty/"+name, func(t *testing.T) {
			// Фигурные скобки в текстовом названии экранированы в Raw.
			rawName := strings.NewReplacer("{", `\{`, "}", `\}`).Replace(name)
			line := v189Line(t, "@variant "+rawName, "tag", 4, 2,
				v189TestElement(t, "tag", "@variant", "variant", 1, 9),
				v189TestElement(t, "name", rawName, name, 10, 10+utf8.RuneCountInString(rawName)))
			result := v189First(t, line)
			v189Run(t, result, "current", nil)
			for _, entry := range result.Diagnostics {
				if entry.Source == "current" && entry.DiagnosticCode == "V032" {
					t.Fatal("branch name treated as ID")
				}
			}
		})
	}
	// Перечень White_Space в пределах физической строки задан независимо от проверки.
	for _, name := range []string{"", " ", "\t", " \t", "\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000", " \u00a0\t\u2003"} {
		t.Run(fmt.Sprintf("whitespace name %q", name), func(t *testing.T) {
			// Краевые SP/TAB удалены из Value по DSL 1.14, исходник сохранён.
			line := v189Line(t, "@variant "+name, "tag", 4, 2,
				v189TestElement(t, "tag", "@variant", "variant", 1, 9),
				v189TestElement(t, "name", name, strings.Trim(name, " \t"), 10, 10+utf8.RuneCountInString(name)))
			v189Run(t, v189First(t, line), "current", []v189Expected{{5, 1, 9}})
		})
	}
	t.Run("case and indentation", func(t *testing.T) {
		v189Run(t, v189First(t, v189Line(t, " \t@VaRiAnT \t", "tag", 4, 2,
			v189TestElement(t, "tag", "@VaRiAnT", "variant", 3, 11))), "current", []v189Expected{{5, 3, 11}})
	})
}

func TestV189RawRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v189Expected
	}{
		{"whole declaration", "@variant", []v189Expected{{5, 1, 9}}},
		{"trailing space", "@variant ", []v189Expected{{5, 1, 9}}},
		{"trailing tab", "@variant\t", []v189Expected{{5, 1, 9}}},
		{"outer spaces and case", " \t@VaRiAnT\t ", []v189Expected{{5, 3, 11}}},
		{"space before NBSP", "@variant \u00a0", []v189Expected{{5, 1, 9}}},
		{"NBSP is not tag boundary", "@variant\u00a0", nil},
		{"tab before NBSP is not boundary", "@variant\t\u00a0", nil},
		{"non ASCII indentation", "\u00a0@variant", nil},
		{"longer name", "@variantExtra", nil}, {"punctuation in tag", "@variant:", nil},
		{"unknown escape in name", `@variant \q`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v189Run(t, v189First(t, v189Invalid(t, tc.raw, 4, 2)), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		raw  string
		want []v189Expected
	}{
		{"@variant Student A", nil}, {"@variant", []v189Expected{{5, 1, 9}}},
		{"@variant \t", []v189Expected{{5, 1, 9}}},
	} {
		t.Run("saved tag without name/"+tc.raw, func(t *testing.T) {
			v189Run(t, v189First(t, v189Line(t, tc.raw, "tag", 4, 2,
				v189TestElement(t, "tag", "@variant", "variant", 1, 9))), "current", tc.want)
		})
	}
	for _, separator := range []string{"\t", "\u00a0"} {
		t.Run("retained tag with wrong separator/"+separator, func(t *testing.T) {
			v189Run(t, v189First(t, v189Line(t, "@variant"+separator, "invalid", 4, 2,
				v189TestElement(t, "tag", "@variant", "variant", 1, 9), v189Unparsed(t, separator, 9, 10))),
				"current", []v189Expected{{5, 1, 9}})
		})
	}
}

func TestV189PartialNames(t *testing.T) {
	for _, kind := range []model.LineType{"tag", "invalid"} {
		for _, tc := range []struct {
			name, raw string
			elements  []model.Element
			want      []v189Expected
		}{
			{"only unparsed name", `@variant \q`, []model.Element{v189Unparsed(t, `\q`, 10, 12)}, nil},
			{"empty name then unparsed", `@variant \q`, []model.Element{
				v189TestElement(t, "name", "", "", 10, 10), v189Unparsed(t, `\q`, 10, 12)}, nil},
			{"whitespace name then unparsed", "@variant \u00a0\\q", []model.Element{
				v189TestElement(t, "name", "\u00a0", "\u00a0", 10, 11), v189Unparsed(t, `\q`, 11, 13)}, nil},
			{"nonempty name then unparsed", `@variant A\q`, []model.Element{
				v189TestElement(t, "name", "A", "A", 10, 11), v189Unparsed(t, `\q`, 11, 13)}, nil},
			{"unparsed before whitespace name", "@variant \\q\u00a0", []model.Element{
				v189Unparsed(t, `\q`, 10, 12), v189TestElement(t, "name", "\u00a0", "\u00a0", 12, 13)}, nil},
			{"empty name and whitespace unparsed", "@variant \u00a0", []model.Element{
				v189TestElement(t, "name", "", "", 10, 10), v189Unparsed(t, "\u00a0", 10, 11)}, []v189Expected{{5, 1, 9}}},
			{"whitespace unparsed", "@variant \u00a0", []model.Element{v189Unparsed(t, "\u00a0", 10, 11)}, []v189Expected{{5, 1, 9}}},
		} {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				elements := append([]model.Element{v189TestElement(t, "tag", "@variant", "variant", 1, 9)}, tc.elements...)
				v189Run(t, v189First(t, v189Line(t, tc.raw, kind, 4, 2, elements...)), "current", tc.want)
			})
		}
	}
}

func TestV189Escaping(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\\`, `\`}, {`\{`, "{"}, {`\}`, "}"}, {`\\\{`, `\{`}, {`A \\ B`, `A \ B`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			for _, saved := range []bool{false, true} {
				elements := []model.Element{v189TestElement(t, "tag", "@variant", "variant", 1, 9)}
				if saved {
					elements = append(elements, v189TestElement(t, "name", tc.raw, tc.value, 10, 10+utf8.RuneCountInString(tc.raw)))
				}
				v189Run(t, v189First(t, v189Line(t, "@variant "+tc.raw, "tag", 4, 2, elements...)), "current", nil)
			}
		})
	}
}

func TestV189ContextAndOccurrences(t *testing.T) {
	t.Run("both branches missing names", func(t *testing.T) {
		v189Run(t, v189Branches(t, v189TagLine(t, "variant", "", "", 4, 2), v189TagLine(t, "variant", "", "", 4, 2)),
			"current", []v189Expected{{5, 1, 9}, {7, 1, 9}})
	})
	t.Run("last branch missing name", func(t *testing.T) {
		v189Run(t, v189Branches(t, v189TagLine(t, "variant", "name", "A", 4, 2), v189TagLine(t, "variant", "", "", 4, 2)),
			"current", []v189Expected{{7, 1, 9}})
	})
	t.Run("outside variants", func(t *testing.T) {
		v189Run(t, v189Document(t, v189TagLine(t, "variant", "", "", 3, 1), v189TextLine(t, "Read.", "Read.", 4, 2)),
			"current", []v189Expected{{4, 1, 9}})
	})
	t.Run("nested in a nonliteral block", func(t *testing.T) {
		v189Run(t, v189Document(t, v189Open(t, "text", "", 3, 1), v189TagLine(t, "variant", "", "", 4, 2),
			v189TextLine(t, "Read.", "Read.", 5, 3), v189Close(t, 4, 2)), "current", []v189Expected{{5, 1, 9}})
	})
	for _, parent := range []string{"editor", "multifill"} {
		t.Run("literal "+parent, func(t *testing.T) {
			v189Run(t, v189Document(t, v189Open(t, parent, "", 3, 1), v189TextLine(t, "@variant", "@variant", 4, 2),
				v189Close(t, 4, 2)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@variant`, "@variant"}, {" \t\\@variant", "@variant"}, {"Read @variant", "Read @variant"},
	} {
		t.Run("ordinary text/"+tc.raw, func(t *testing.T) {
			v189Run(t, v189Document(t, v189Open(t, "variants", "", 3, 1), v189TagLine(t, "variant", "name", "A", 4, 2),
				v189TextLine(t, tc.raw, tc.value, 5, 3), v189TagLine(t, "variant", "name", "B", 4, 2),
				v189TextLine(t, "Write.", "Write.", 7, 3), v189Close(t, 4, 2)), "current", nil)
		})
	}
	t.Run("content parent is the outer container", func(t *testing.T) {
		result := v189First(t, v189TagLine(t, "variant", "", "", 4, 2))
		for _, index := range []int{5, 7} {
			parent := 4
			result.Lines[index].ParentLine = &parent
			result.Lines[index].NestingLevel = 2
		}
		v189Run(t, result, "current", []v189Expected{{5, 1, 9}})
	})
}

func TestV189BlockFormsBelongToV109(t *testing.T) {
	for _, tc := range []struct {
		name    string
		line    model.Line
		opening int
	}{
		{"whole unparsed", v189Invalid(t, "@variant {", 4, 2), 10},
		{"saved tag and unparsed opening", v189Line(t, "@variant {", "invalid", 4, 2,
			v189TestElement(t, "tag", "@variant", "variant", 1, 9), v189Unparsed(t, "{", 10, 11)), 10},
		{"saved opening in invalid", v189Line(t, "@variant { tail", "invalid", 4, 2,
			v189TestElement(t, "tag", "@variant", "variant", 1, 9), v189TestElement(t, "block-open", "{", "{", 10, 11),
			v189Unparsed(t, "tail", 12, 16)), 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v189First(t, tc.line)
			v189Run(t, result, "current", nil)
			v189AssertBlockError(t, result, tc.opening)
		})
	}
	for _, raw := range []string{"@variant {", "@variant{"} {
		t.Run("parsed block/"+raw, func(t *testing.T) {
			opening := utf8.RuneCountInString(raw)
			result := v189Document(t, v189Open(t, "variants", "", 3, 1),
				v189Line(t, raw, "block-start", 4, 2, v189TestElement(t, "tag", "@variant", "variant", 1, 9),
					v189TestElement(t, "block-open", "{", "{", opening, opening+1)),
				v189TextLine(t, "Read.", "Read.", 5, 3), v189Close(t, 5, 3),
				v189TagLine(t, "variant", "name", "B", 4, 2), v189TextLine(t, "Write.", "Write.", 8, 3), v189Close(t, 4, 2))
			v189Run(t, result, "current", nil)
			v189AssertBlockError(t, result, opening)
		})
	}
}

func TestV189EmptyEndingsAndHistory(t *testing.T) {
	v189Run(t, v189EmptyModel(t, "1.2"), "current", nil)
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("ending %q", ending), func(t *testing.T) {
			result := v189First(t, v189TagLine(t, "variant", "", "", 4, 2))
			if ending != "" {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			v189Run(t, result, "first", []v189Expected{{5, 1, 9}})
			v189Run(t, result, "second", []v189Expected{{5, 1, 9}})
		})
	}
	t.Run("final physical line", func(t *testing.T) {
		v189Run(t, v189Document(t, v189TagLine(t, "variant", "", "", 3, 1)), "current", []v189Expected{{4, 1, 9}})
	})
}

// v189AssertBlockError проверяет передачу ошибочной формы уже подключённой V109.
func v189AssertBlockError(t *testing.T, result *model.Result, column int) {
	t.Helper()
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == "current" && entry.DiagnosticCode == "V109" {
			got = append(got, entry)
		}
	}
	want := model.Location{Start: model.Position{Line: 5, Column: column}, End: model.Position{Line: 5, Column: column + 1}}
	if len(got) != 1 || got[0].Location == nil || *got[0].Location != want {
		t.Fatalf("V109 = %+v, want one at %+v", got, want)
	}
	related := []model.Location{{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 9}}}
	if got[0].SeverityLevel != "error" || got[0].DiagnosticScope != "line" || got[0].Fatal ||
		got[0].Message != "У однострочного тега использована блочная форма.\nконкретизация: Тег @variant поддерживает только однострочную запись" ||
		!slices.Equal(got[0].RelatedLocations, related) {
		t.Fatalf("unexpected V109 metadata or related ranges: %+v", got[0])
	}
}

// v189Invalid задаёт полностью неразобранное ошибочное объявление.
func v189Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v189Line(t, raw, "invalid", parent, depth, v189Unparsed(t, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v189First помещает проверяемое объявление перед второй именованной ветвью.
func v189First(t *testing.T, first model.Line) *model.Result {
	t.Helper()
	return v189Branches(t, first, v189TagLine(t, "variant", "name", "B", 4, 2))
}

// v189Branches задаёт две ветви с непустым содержимым и явными родителями.
func v189Branches(t *testing.T, first, second model.Line) *model.Result {
	t.Helper()
	return v189Document(t, v189Open(t, "variants", "", 3, 1), first, v189TextLine(t, "Read.", "Read.", 5, 3),
		second, v189TextLine(t, "Write.", "Write.", 7, 3), v189Close(t, 4, 2))
}

// v189Registry создаёт реестр из полного штатного каталога регистраций.
func v189Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v189EmptyModel создаёт модель без исходных строк и с явно указанной версией.
func v189EmptyModel(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v189Document добавляет технический префикс и историю ошибок unparsed.
// Структура, классификация и значения элементов заданы тестами, без парсера.
func v189Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v189EmptyModel(t, "1.2")
	title, id := "Lesson", "v189"
	result.Document.Metadata.Title, result.Document.Metadata.DocumentID = &title, &id
	result.Lines = append([]model.Line{v189TagLine(t, "dsl-version", "version", "1.2", 0, 0),
		v189TagLine(t, "document-id", "identifier", "v189", 0, 0),
		v189Line(t, "# Lesson", "heading", 0, 0, v189TestElement(t, "heading-level", "#", "1", 1, 2), v189TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
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

// v189Line сохраняет явно заданные данные физической строки и её родителя.
func v189Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v189TestElement задаёт отдельные Raw и Value с диапазоном [start,end) в Unicode.
func v189TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v189Unparsed задаёт фрагмент без значения; историю добавляет v189Document.
func v189Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v189TagLine создаёт объявление с явно заданным типом аргумента.
func v189TagLine(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v189TestElement(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v189TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v189Line(t, raw, "tag", parent, depth, elements...)
}

// v189TextLine задаёт обычную строку с независимо указанным семантическим значением.
func v189TextLine(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v189Line(t, raw, "content", parent, depth, v189TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v189Open задаёт блочное объявление с необязательной инструкцией.
func v189Open(t *testing.T, name, instruction string, parent, depth int) model.Line {
	t.Helper()
	kind := model.ElementType("")
	if instruction != "" {
		kind = "content"
	}
	line := v189TagLine(t, name, kind, instruction, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v189TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v189Close задаёт отдельную закрывающую скобку с явным родителем.
func v189Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v189Line(t, "}", "block-end", parent, depth, v189TestElement(t, "block-close", "}", "}", 1, 2))
}

// v189Run запускает неизменённую выборку штатного каталога и проверяет новые V189.
// История и исходная модель сохраняются; новые ссылки допускаются только по API.
func v189Run(t *testing.T, result *model.Result, source string, want []v189Expected) {
	t.Helper()
	before := v189Snapshot(t, result)
	checks, err := validator.Prepare(result, v189Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v189Snapshot(t, result) != before {
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
		if entry.DiagnosticCode == "V189" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V189 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Location == nil || *entry.Location != location ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "У @variant отсутствует название ветви." {
			t.Errorf("occurrence = %+v, want range %+v and exact V189 metadata", entry, location)
		}
		if len(entry.RelatedLocations) != 0 {
			t.Errorf("unexpected related locations: %+v", entry.RelatedLocations)
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
	if err := json.Unmarshal([]byte(v189Snapshot(t, result)), &actual); err != nil {
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
	if v189Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v189Snapshot сохраняет модель для проверки неизменности исходника и истории.
func v189Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

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

func TestV129RegistrationAndVersions(t *testing.T) {
	registry := v129TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V129" {
			continue
		}
		count++
		if definition.Message != "У @fragment отсутствует идентификатор." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V129 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.0"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v129TestDocument(t, v129TestInvalid(t, "@fragment", 3))
			result.Document.DSLVersion = &version
			before := v129TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v129TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV129ReferenceAndExample(t *testing.T) {
	// DSL 1.30: перед неизменёнными фрагментами добавлены версия, заголовок и шаг.
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           [][3]int
	}{
		{"reference", "@fragment f {\nRead this text.\n}\n@include f",
			[]model.Line{v129TestOpen(t, "fragment", "f", 3), v129TestText(t, "Read this text.", "Read this text.", 4),
				v129TestClose(t, 4), v129TestTag(t, "include", "f", "identifier", 3)}, nil},
		{"example", "@fragment {\nRead this text.\n}",
			[]model.Line{v129TestMissing(t, 3), v129TestText(t, "Read this text.", "Read this text.", 4),
				v129TestClose(t, 4)}, [][3]int{{4, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v129TestDocument(t, tc.rows...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v129TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV129UnparsedNamesAndArguments(t *testing.T) {
	// Полностью нераспознанная строка не получает выдуманных дочерних строк
	// или закрытия блока. Ожидаемые диапазоны заданы независимо от исходника.
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"bare name", "@fragment", [][3]int{{4, 1, 10}}},
		{"edge spaces and tabs", "@fragment \t ", [][3]int{{4, 1, 10}}},
		{"unparsed opening brace", "@fragment {", [][3]int{{4, 1, 10}}},
		{"adjacent opening brace", "@fragment{", [][3]int{{4, 1, 10}}},
		{"tab delimiter", "@fragment\t{", [][3]int{{4, 1, 10}}},
		{"whitespace argument", "@fragment \t \t {", [][3]int{{4, 1, 10}}},
		{"uppercase name", "@FRAGMENT {", [][3]int{{4, 1, 10}}},
		{"indent and mixed case", "\t @FrAgMeNt {", [][3]int{{4, 3, 12}}},
		{"trailing garbage", "@fragment { extra", [][3]int{{4, 1, 10}}},
		{"first of several braces", "@fragment {{", [][3]int{{4, 1, 10}}},
		{"empty braces", "@fragment {}", [][3]int{{4, 1, 10}}},
		{"nonempty without brace", "@fragment f", nil},
		{"nonempty before brace", "@fragment f {", nil},
		{"invalid nonempty without brace", "@fragment bad/id", nil},
		{"invalid nonempty before brace", "@fragment bad/id { extra", nil},
		{"quotes are data", `@fragment "" {`, nil},
		{"slash is data", "@fragment / {", nil},
		{"internal space", "@fragment a b {", nil},
		{"Unicode nonempty", "@fragment 😀 {", nil},
		{"NBSP is not trimmed", "@fragment \u00a0 {", nil},
		{"em space is not trimmed", "@fragment \u2003 {", nil},
		{"zero width space is not trimmed", "@fragment \u200b {", nil},
		{"other whitespace is not trimmed", "@fragment \v\f {", nil},
		{"longer name", "@fragmentExtra {", nil},
		{"hyphenated name", "@fragment-extra {", nil},
		{"NBSP is not name boundary", "@fragment\u00a0{", nil},
		{"punctuation is not name boundary", "@fragment: {", nil},
		{"escaped brace", `@fragment \{`, nil},
		{"backslash pair before brace", `@fragment \\{`, nil},
		{"three backslashes before brace", `@fragment \\\{`, nil},
		{"escaped then unescaped brace", `@fragment \{ {`, nil},
		{"trailing backslash", `@fragment \`, nil},
		{"closing brace is nonempty argument", "@fragment }", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v129TestRun(t, v129TestDocument(t, v129TestInvalid(t, tc.raw, 3)), "current", tc.want)
		})
	}
}

func TestV129RecognizedAndPartialDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opening model.Line
		want    [][3]int
	}{
		{"tag and missing identifier", v129TestMissing(t, 3), [][3]int{{4, 1, 10}}},
		{"minimal identifier", v129TestOpen(t, "fragment", "f", 3), nil},
		{"indented recognized tag", v129TestLine(t, "\t @FrAgMeNt {", "block-start", 3,
			v129TestElement(t, "tag", "@FrAgMeNt", "fragment", 3),
			v129TestElement(t, "unparsed", "", "", 13), v129TestElement(t, "block-open", "{", "{", 13)),
			[][3]int{{4, 3, 12}}},
		{"unparsed name with recognized brace", v129TestLine(t, "@fragment {", "block-start", 3,
			v129TestElement(t, "unparsed", "@fragment", "", 1), v129TestElement(t, "block-open", "{", "{", 11)),
			[][3]int{{4, 1, 10}}},
		{"missing identifier element with nonempty unparsed", v129TestLine(t, "@fragment bad/id {", "block-start", 3,
			v129TestElement(t, "tag", "@fragment", "fragment", 1), v129TestElement(t, "unparsed", "bad/id", "", 11),
			v129TestElement(t, "block-open", "{", "{", 18)), nil},
		{"identifier and unparsed remainder", v129TestLine(t, "@fragment good bad/id {", "block-start", 3,
			v129TestElement(t, "tag", "@fragment", "fragment", 1), v129TestElement(t, "identifier", "good", "good", 11),
			v129TestElement(t, "unparsed", "bad/id", "", 16), v129TestElement(t, "block-open", "{", "{", 23)), nil},
		{"identifier has canonical value", v129TestLine(t, "@fragment ID {", "block-start", 3,
			v129TestElement(t, "tag", "@fragment", "fragment", 1), v129TestElement(t, "identifier", "ID", "id", 11),
			v129TestElement(t, "block-open", "{", "{", 14)), nil},
		{"empty unparsed is not a second declaration", v129TestLine(t, "@fragment \t {", "block-start", 3,
			v129TestElement(t, "tag", "@fragment", "fragment", 1), v129TestElement(t, "unparsed", "\t", "", 11),
			v129TestElement(t, "block-open", "{", "{", 13)), [][3]int{{4, 1, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v129TestDocument(t, tc.opening, v129TestText(t, "Read this text.", "Read this text.", 4),
				v129TestText(t, "Another paragraph.", "Another paragraph.", 4), v129TestClose(t, 4))
			added := v129TestRun(t, result, "current", tc.want)
			if tc.name == "missing identifier element with nonempty unparsed" {
				var got []model.Diagnostic
				for _, entry := range added {
					if entry.DiagnosticCode == "V032" {
						got = append(got, entry)
					}
				}
				location := model.Location{Start: model.Position{Line: 4, Column: 11}, End: model.Position{Line: 4, Column: 17}}
				if len(got) != 1 || got[0].Location == nil || *got[0].Location != location ||
					got[0].Message != "Идентификатор содержит недопустимые символы.\nконкретизация: Недопустимые символы: \"/\" (U+002F)." {
					t.Fatalf("V032 = %+v, want one finding on bad/id", got)
				}
			}
		})
	}
	for _, tc := range []struct {
		name, raw, tail string
		want            [][3]int
	}{
		{"recognized bare tag", "@fragment", "", [][3]int{{4, 1, 10}}},
		{"recognized tag without block-open", "@fragment { extra", " { extra", [][3]int{{4, 1, 10}}},
		{"recognized nonempty without block-open", "@fragment f {", " f {", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v129TestLine(t, tc.raw, "invalid", 3, v129TestElement(t, "tag", "@fragment", "fragment", 1),
				v129TestElement(t, "unparsed", tc.tail, "", 10))
			v129TestRun(t, v129TestDocument(t, line), "current", tc.want)
		})
	}
}

func TestV129NestingAndIndependentDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name          string
		rows          []model.Line
		want          [][3]int
		forbiddenLine int
	}{
		{"nested missing identifier", []model.Line{v129TestOpen(t, "fragment", "outer", 3),
			v129TestMissing(t, 4), v129TestText(t, "Read.", "Read.", 5), v129TestClose(t, 5),
			v129TestClose(t, 4), v129TestTag(t, "include", "outer", "identifier", 3)}, [][3]int{{5, 1, 10}}, 5},
		{"two nested missing identifiers", []model.Line{v129TestMissing(t, 3), v129TestMissing(t, 4),
			v129TestText(t, "Read.", "Read.", 5), v129TestClose(t, 5), v129TestClose(t, 4)},
			[][3]int{{4, 1, 10}, {5, 1, 10}}, 5},
		{"inside content block", []model.Line{v129TestOpen(t, "note", "", 3), v129TestMissing(t, 4),
			v129TestText(t, "Read.", "Read.", 5), v129TestClose(t, 5), v129TestClose(t, 4)}, [][3]int{{5, 1, 10}}, 5},
		{"inside task", []model.Line{v129TestTag(t, "task", "t", "identifier", 3), v129TestMissing(t, 4),
			v129TestText(t, "Read.", "Read.", 5), v129TestClose(t, 5), v129TestTag(t, "endtask", "", "", 3)},
			[][3]int{{5, 1, 10}}, 0},
		{"adjacent blocks", []model.Line{v129TestMissing(t, 3), v129TestText(t, "Read.", "Read.", 4),
			v129TestClose(t, 4), v129TestMissing(t, 3), v129TestText(t, "Write.", "Write.", 7), v129TestClose(t, 7)},
			[][3]int{{4, 1, 10}, {7, 1, 10}}, 0},
		{"blank between declarations", []model.Line{v129TestInvalid(t, "@fragment", 3),
			v129TestLine(t, " \t", "blank", 3), v129TestInvalid(t, "@fragment", 3)},
			[][3]int{{4, 1, 10}, {6, 1, 10}}, 0},
		{"unparsed declaration inside fragment", []model.Line{v129TestOpen(t, "fragment", "outer", 3),
			v129TestInvalid(t, "@fragment { extra", 4), v129TestText(t, "Read.", "Read.", 4), v129TestClose(t, 4),
			v129TestTag(t, "include", "outer", "identifier", 3)}, [][3]int{{5, 1, 10}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v129TestDocument(t, tc.rows...)
			if tc.forbiddenLine != 0 {
				v129TestParserError(t, result, tc.forbiddenLine, 0, "Вложение fragment запрещено в этом блоке.")
			}
			v129TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV129LiteralContextsAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"escaped name", v129TestText(t, `\@fragment`, "@fragment", 3)},
		{"indented escaped name", v129TestText(t, " \t\\@fragment", "@fragment", 3)},
		{"backslash pair before name", v129TestText(t, `\\@fragment`, `\@fragment`, 3)},
		{"mention", v129TestText(t, "Text @fragment", "Text @fragment", 3)},
		{"inside note argument", v129TestTag(t, "note", "@fragment", "content", 3)},
		{"inside editor argument", v129TestTag(t, "editor", "@fragment", "content", 3)},
		{"Unicode indentation", v129TestText(t, "\u00a0@fragment", "\u00a0@fragment", 3)},
		{"empty line", v129TestLine(t, "", "blank", 3)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v129TestRun(t, v129TestDocument(t, tc.row), "current", nil)
		})
	}
	for _, name := range []string{"editor", "multifill"} {
		for _, after := range []bool{false, true} {
			t.Run(name+"/after="+strconv.FormatBool(after), func(t *testing.T) {
				rows := []model.Line{v129TestOpen(t, name, "", 3),
					v129TestText(t, "@fragment", "@fragment", 4), v129TestClose(t, 4)}
				var want [][3]int
				if after {
					rows = append(rows, v129TestMissing(t, 3), v129TestText(t, "Read.", "Read.", 7), v129TestClose(t, 7))
					want = [][3]int{{7, 1, 10}}
				}
				v129TestRun(t, v129TestDocument(t, rows...), "current", want)
			})
		}
	}
	t.Run("nested literal container", func(t *testing.T) {
		result := v129TestDocument(t, v129TestTag(t, "task", "t", "identifier", 3),
			v129TestOpen(t, "editor", "", 4), v129TestText(t, "@fragment", "@fragment", 5),
			v129TestClose(t, 5), v129TestMissing(t, 4), v129TestText(t, "Read.", "Read.", 8),
			v129TestClose(t, 8), v129TestTag(t, "endtask", "", "", 3))
		v129TestRun(t, result, "current", [][3]int{{8, 1, 10}})
	})
	t.Run("unparsed placeholder in literal multifill", func(t *testing.T) {
		// DSL 4.3.2 и 4.4.11: ошибка длины плейсхолдера не делает @fragment тегом.
		result := v129TestDocument(t, v129TestOpen(t, "multifill", "", 3),
			v129TestLine(t, "@fragment __", "content", 4,
				v129TestElement(t, "content", "@fragment", "@fragment", 1), v129TestElement(t, "unparsed", "__", "", 11)),
			v129TestClose(t, 4))
		v129TestRun(t, result, "current", nil)
	})
}

func TestV129EmptyEndingsAndHistory(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v129TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v129TestRun(t, result, "current", nil)
		})
	}
	t.Run("last line without EOL", func(t *testing.T) {
		result := v129TestDocument(t, v129TestInvalid(t, "@fragment", 3))
		result.Lines[3].LineEnding = ""
		v129TestRun(t, result, "current", [][3]int{{4, 1, 10}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(string(eol))+"/final="+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v129TestDocument(t, v129TestMissing(t, 3), v129TestText(t, "Read this text.", "Read this text.", 4),
					v129TestClose(t, 4))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v129TestRun(t, result, "first", [][3]int{{4, 1, 10}})
				v129TestRun(t, v129TestDocument(t, v129TestOpen(t, "fragment", "f", 3),
					v129TestText(t, "Read.", "Read.", 4), v129TestClose(t, 4)), "other-document", nil)
				v129TestRun(t, result, "second", [][3]int{{4, 1, 10}})
			})
		}
	}
}

// v129TestElement создаёт входной элемент; ожидания задаются отдельно.
func v129TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v129TestLine задаёт классификацию и родителя явно, без разбора исходника.
func v129TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v129TestTag задаёт известное имя и явный тип аргумента.
func v129TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v129TestElement(t, "tag", raw, name, 1)}
	if argument != "" {
		elements = append(elements, v129TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v129TestLine(t, raw, "tag", parent, elements...)
}

// v129TestOpen задаёт блок; непустой аргумент в этих входах является ID fragment.
func v129TestOpen(t *testing.T, name, id string, parent int) model.Line {
	t.Helper()
	line := v129TestTag(t, name, id, "identifier", parent)
	line.Elements = append(line.Elements, v129TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v129TestMissing задаёт разобранный блок с пустым ошибочным местом ID перед {.
func v129TestMissing(t *testing.T, parent int) model.Line {
	t.Helper()
	return v129TestLine(t, "@fragment {", "block-start", parent,
		v129TestElement(t, "tag", "@fragment", "fragment", 1),
		v129TestElement(t, "unparsed", "", "", 11), v129TestElement(t, "block-open", "{", "{", 11))
}

// v129TestClose закрывает явно указанный блок.
func v129TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v129TestLine(t, "}", "block-end", parent, v129TestElement(t, "block-close", "}", "}", 1))
}

// v129TestText сохраняет исходное и семантическое содержимое независимо.
func v129TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v129TestLine(t, raw, "content", parent, v129TestElement(t, "content", raw, value, 1))
}

// v129TestInvalid задаёт полностью неразобранную строку с известным родителем.
func v129TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v129TestLine(t, raw, "invalid", parent, v129TestElement(t, "unparsed", raw, "", 1))
}

// v129TestDocument добавляет контекст шага, номера строк и историю unparsed.
// Родители передаются явно; глубина вычисляется только для согласованности входа.
func v129TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v129TestTag(t, "dsl-version", "1.2", "version", 0),
		v129TestLine(t, "# Lesson", "heading", 0, v129TestElement(t, "heading-level", "#", "1", 1),
			v129TestElement(t, "title", "Lesson", "Lesson", 3)),
		v129TestLine(t, "### Practice", "heading", 0, v129TestElement(t, "heading-level", "###", "3", 1),
			v129TestElement(t, "title", "Practice", "Practice", 5)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			if line.Elements[j].ElementType == "unparsed" {
				v129TestParserError(t, result, line.Number, j, "Ошибочный исходный фрагмент.")
			}
		}
	}
	return result
}

// v129TestParserError добавляет исходную ошибку с обратной ссылкой и флагами.
// Код условный: каталог диагностик внешнего парсера не входит в репозиторий.
func v129TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v129TestRegistry использует только штатный каталог регистраций.
func v129TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v129TestSnapshot сохраняет полную модель для проверки неизменности исходника.
func v129TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v129TestRun проверяет новые V129 после полного штатного конвейера.
// Возвращает новые диагностики для отдельной проверки разграничения с V032.
func v129TestRun(t *testing.T, result *model.Result, processingID string, want [][3]int) []model.Diagnostic {
	t.Helper()
	before := v129TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v129TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v129TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	added := result.Diagnostics[oldCount:]
	var got []model.Diagnostic
	for _, entry := range added {
		if entry.DiagnosticCode == "V129" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V129 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != "У @fragment отсутствует идентификатор." ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.ID == "" ||
			entry.Source != processingID || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V129 occurrence %d = %+v, want location %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Fatal("V129 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V129 created an element error link")
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
	if err := json.Unmarshal([]byte(v129TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Исключаются только новые записи и производные флаги/ссылки.
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
				t.Fatal("previous element error links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v129TestSnapshot(t, &restored) != before {
		t.Fatal("source data or previous history changed")
	}
	return added
}

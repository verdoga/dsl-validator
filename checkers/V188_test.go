package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v188Expected задаёт точный диапазон тега (конец исключается) и число ветвей.
type v188Expected struct{ line, start, end, count int }

func TestV188ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		body         []model.Line
		want         []v188Expected
	}{
		{"reference", "@variants {\n@variant A\nRead.\n@variant B\nWrite.\n}", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestText(t, "Read.", "Read.", 4, 3), v188TestTag(t, "variant", "name", "B", 3, 2),
			v188TestText(t, "Write.", "Write.", 6, 3), v188TestClose(t, 3, 2),
		}, nil},
		{"example", "@variants {\n@variant A\nRead.\n}", v188TestBase(t), []v188Expected{{3, 1, 10, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.body {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.source {
				t.Fatal("normative fragment changed")
			}
			// Добавлены только обязательные версия и основной заголовок по DSL 1.30.
			v188TestRun(t, v188TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV188BranchCountAndNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []model.Line
		want []v188Expected
	}{
		{"zero", nil, []v188Expected{{3, 1, 10, 0}}},
		{"blank rows", []model.Line{v188TestBlank(t, "", 3, 2), v188TestBlank(t, " \t ", 3, 2)}, []v188Expected{{3, 1, 10, 0}}},
		{"one empty branch", []model.Line{v188TestTag(t, "variant", "name", "A", 3, 2)}, []v188Expected{{3, 1, 10, 1}}},
		{"two empty branches", []model.Line{v188TestTag(t, "variant", "name", "A", 3, 2), v188TestTag(t, "variant", "name", "B", 3, 2)}, nil},
		{"three branches", []model.Line{v188TestTag(t, "variant", "name", "A", 3, 2), v188TestTag(t, "variant", "name", "B", 3, 2), v188TestTag(t, "variant", "name", "C", 3, 2)}, nil},
		{"one missing name", []model.Line{v188TestTag(t, "variant", "", "", 3, 2)}, []v188Expected{{3, 1, 10, 1}}},
		{"two missing names", []model.Line{v188TestTag(t, "variant", "", "", 3, 2), v188TestTag(t, "variant", "", "", 3, 2)}, nil},
		{"duplicate names", []model.Line{v188TestTag(t, "variant", "name", "A", 3, 2), v188TestTag(t, "variant", "name", "A", 3, 2)}, nil},
		{"case duplicate names", []model.Line{v188TestTag(t, "variant", "name", "A", 3, 2), v188TestTag(t, "variant", "name", "a", 3, 2)}, nil},
		{"spaces tabs Unicode in names", []model.Line{v188TestTag(t, "variant", "name", "Роль A\t😀", 3, 2), v188TestTag(t, "variant", "name", "Роль  B е\u0301", 3, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{v188TestOpen(t, "variants", 2, 1)}, tc.rows...)
			body = append(body, v188TestClose(t, 3, 2))
			// Пустота и имена ветвей намеренно нарушают отдельные правила 4.9.14/25;
			// распознанные объявления при этом должны участвовать в количестве.
			result := v188TestDocument(t, body...)
			v188TestRun(t, result, "current", tc.want)
			if tc.name == "zero" {
				for _, entry := range result.Diagnostics {
					if entry.DiagnosticCode == "V035" && entry.Source == "current" {
						t.Fatal("V035 duplicates empty variants")
					}
				}
			}
		})
	}
	t.Run("recognized tag with invalid name", func(t *testing.T) {
		unparsed := v188TestElement(t, "unparsed", "\tA", "", 9, 11)
		unparsed.Value = nil
		line := v188TestLine(t, "@variant\tA", "invalid", 3, 2,
			v188TestElement(t, "tag", "@variant", "variant", 1, 9), unparsed)
		v188TestRun(t, v188TestDocument(t, v188TestOpen(t, "variants", 2, 1), line,
			v188TestText(t, "Read.", "Read.", 4, 3), v188TestClose(t, 3, 2)), "current", []v188Expected{{3, 1, 10, 1}})
	})
}

func TestV188LogicalParents(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v188Expected
	}{
		{"previous variant", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestText(t, "Read.", "Read.", 4, 3), v188TestTag(t, "variant", "name", "B", 4, 3),
			v188TestText(t, "Write.", "Write.", 6, 4), v188TestClose(t, 3, 2),
		}, nil},
		{"task", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestTag(t, "task", "identifier", "t", 4, 3), v188TestText(t, "Read.", "Read.", 5, 4),
			v188TestTag(t, "variant", "name", "B", 5, 4), v188TestText(t, "Write.", "Write.", 7, 5), v188TestClose(t, 3, 2),
		}, nil},
		{"step in task", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestTag(t, "task", "identifier", "t", 4, 3), v188TestTag(t, "step", "", "", 5, 4),
			v188TestText(t, "Read.", "Read.", 6, 5), v188TestTag(t, "variant", "name", "B", 6, 5),
			v188TestText(t, "Write.", "Write.", 8, 6), v188TestClose(t, 3, 2),
		}, nil},
		{"one branch with task and step", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestTag(t, "task", "identifier", "t", 4, 3), v188TestTag(t, "step", "", "", 5, 4),
			v188TestText(t, "Read.", "Read.", 6, 5), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}},
		{"variants inside task", []model.Line{
			v188TestTag(t, "task", "identifier", "t", 2, 1), v188TestOpen(t, "variants", 3, 2),
			v188TestTag(t, "variant", "name", "A", 4, 3), v188TestText(t, "Read.", "Read.", 5, 4),
			v188TestClose(t, 4, 3), v188TestTag(t, "endtask", "", "", 3, 2),
		}, []v188Expected{{4, 1, 10, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v188TestRun(t, v188TestDocument(t, tc.body...), "current", tc.want) })
	}
}

func TestV188CurlyContainersAndLiterals(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		want      []v188Expected
		errorLine int
	}{
		{"variant inside note", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "note", 4, 3),
			v188TestTag(t, "variant", "name", "B", 5, 4), v188TestText(t, "Read.", "Read.", 6, 5), v188TestClose(t, 5, 4), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 6},
		{"logical ancestors inside note", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "note", 4, 3),
			v188TestTag(t, "task", "identifier", "t", 5, 4), v188TestTag(t, "step", "", "", 6, 5),
			v188TestTag(t, "variant", "name", "B", 7, 6), v188TestText(t, "Read.", "Read.", 8, 7), v188TestClose(t, 5, 4), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 6},
		{"editor before first branch", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestOpen(t, "editor", 3, 2),
			v188TestText(t, "@variant A", "@variant A", 4, 3), v188TestClose(t, 4, 3), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 0}}, 0},
		{"multifill in branch", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "multifill", 4, 3),
			v188TestText(t, "@variant B", "@variant B", 5, 4), v188TestClose(t, 5, 4), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 0},
		{"escaped pseudo tag", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestText(t, `\@variant B`, "@variant B", 4, 3), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 0},
		{"mention in text", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
			v188TestText(t, "Read @variant B", "Read @variant B", 4, 3), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 0},
		{"independent nested variants", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "variants", 4, 3),
			v188TestTag(t, "variant", "name", "B", 5, 4), v188TestText(t, "Read.", "Read.", 6, 5), v188TestClose(t, 5, 4), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}, {5, 1, 10, 1}}, 5},
		{"two branches only in nested variants", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "variants", 4, 3),
			v188TestTag(t, "variant", "name", "B", 5, 4), v188TestText(t, "Read.", "Read.", 6, 5),
			v188TestTag(t, "variant", "name", "C", 5, 4), v188TestText(t, "Write.", "Write.", 8, 5), v188TestClose(t, 5, 4), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 5},
		{"unknown row inside bounded note", []model.Line{
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "note", 4, 3),
			v188TestInvalid(t, "@variant{", 5, 4), v188TestClose(t, 5, 4), v188TestClose(t, 3, 2),
		}, []v188Expected{{3, 1, 10, 1}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v188TestDocument(t, tc.body...)
			if tc.errorLine != 0 {
				v188TestError(t, result, tc.errorLine, -1, "Объявление запрещено в этом контейнере.")
			}
			v188TestRun(t, result, "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal variants in "+tag, func(t *testing.T) {
			v188TestRun(t, v188TestDocument(t, v188TestOpen(t, tag, 2, 1),
				v188TestText(t, "@variants {", "@variants {", 3, 2), v188TestText(t, "@variant A", "@variant A", 3, 2),
				v188TestText(t, `\}`, "}", 3, 2), v188TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV188KnownStructureDespiteOtherErrors(t *testing.T) {
	t.Run("recognized forbidden heading", func(t *testing.T) {
		unparsed := v188TestElement(t, "unparsed", " H", "", 2, 4)
		unparsed.Value = nil
		heading := v188TestLine(t, "# H", "invalid", 4, 3,
			v188TestElement(t, "heading-level", "#", "1", 1, 2), unparsed)
		result := v188TestDocument(t, v188TestOpen(t, "variants", 2, 1),
			v188TestTag(t, "variant", "name", "A", 3, 2), heading, v188TestClose(t, 3, 2))
		v188TestRun(t, result, "current", []v188Expected{{3, 1, 10, 1}})
	})
	t.Run("newpage changes logical parent", func(t *testing.T) {
		result := v188TestDocument(t, v188TestOpen(t, "variants", 2, 1),
			v188TestTag(t, "newpage", "", "", 2, 1), v188TestText(t, "Read.", "Read.", 4, 2),
			v188TestTag(t, "variant", "name", "A", 4, 2), v188TestClose(t, 3, 2))
		v188TestError(t, result, 4, -1, "Недопустимая граница шага внутри variants.")
		v188TestRun(t, result, "current", []v188Expected{{3, 1, 10, 1}})
	})
}

func TestV188Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      []model.Line
		errorLine int
	}{
		{"unknown possible branch", []model.Line{v188TestOpen(t, "variants", 2, 1), v188TestInvalid(t, "@variant{", 3, 2), v188TestClose(t, 3, 2)}, 0},
		{"missing closing", []model.Line{v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestText(t, "Read.", "Read.", 4, 3)}, 3},
		{"ambiguous closing", []model.Line{v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestClose(t, 3, 2), v188TestClose(t, 3, 2)}, 6},
		{"unclosed nested container", []model.Line{v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2), v188TestOpen(t, "note", 4, 3), v188TestText(t, "Read.", "Read.", 5, 4), v188TestClose(t, 3, 2)}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v188TestDocument(t, tc.body...)
			if tc.errorLine != 0 {
				v188TestError(t, result, tc.errorLine, -1, "Границы блока неоднозначны.")
			}
			v188TestRun(t, result, "current", nil)
		})
	}
	t.Run("branches before and after physical block", func(t *testing.T) {
		result := v188TestDocument(t, v188TestTag(t, "variant", "name", "Before", 2, 1),
			v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 4, 2),
			v188TestText(t, "Read.", "Read.", 5, 3), v188TestClose(t, 4, 2), v188TestTag(t, "variant", "name", "After", 2, 1))
		v188TestError(t, result, 3, -1, "Ветвь вне variants.")
		v188TestError(t, result, 8, -1, "Ветвь вне variants.")
		v188TestRun(t, result, "current", []v188Expected{{4, 1, 10, 1}})
	})
	t.Run("independent containers", func(t *testing.T) {
		body := append(v188TestBase(t), v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "B", 7, 2),
			v188TestText(t, "Write.", "Write.", 8, 3), v188TestClose(t, 7, 2))
		v188TestRun(t, v188TestDocument(t, body...), "current", []v188Expected{{3, 1, 10, 1}, {7, 1, 10, 1}})
	})
	t.Run("case insensitive declarations", func(t *testing.T) {
		result := v188TestDocument(t, v188TestBase(t)...)
		result.Lines[2].Raw, result.Lines[2].Elements[0].Raw = "@VaRiAnTs {", "@VaRiAnTs"
		result.Lines[3].Raw, result.Lines[3].Elements[0].Raw = "@VaRiAnT A", "@VaRiAnT"
		v188TestRun(t, result, "current", []v188Expected{{3, 1, 10, 1}})
	})
	t.Run("indented tag range", func(t *testing.T) {
		result := v188TestDocument(t, v188TestBase(t)...)
		result.Lines[2].Raw = " \t@variants {"
		for i := range result.Lines[2].Elements {
			result.Lines[2].Elements[i].Start += 2
			result.Lines[2].Elements[i].End += 2
		}
		v188TestRun(t, result, "current", []v188Expected{{3, 3, 12, 1}})
	})
}

func TestV188RegistrationAndHistory(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V188" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "element" || definition.Message != "В @variants меньше двух ветвей @variant." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V188 definitions=%d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v188TestDocument(t, v188TestBase(t)...)
			result.Document.DSLVersion = &version
			result.Lines[0] = v188TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
				Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v188TestRun(t, result, "current", nil)
		})
	}
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v188TestDocument(t, v188TestBase(t)...)
			for i := range result.Lines {
				if ending == "LF" {
					result.Lines[i].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[i].LineEnding = "\r\n"
				}
			}
			want := []v188Expected{{3, 1, 10, 1}}
			v188TestRun(t, result, "first", want)
			v188TestRun(t, result, "second", want)
			v188TestRun(t, v188TestDocument(t, v188TestOpen(t, "variants", 2, 1),
				v188TestTag(t, "variant", "name", "A", 3, 2), v188TestText(t, "Read.", "Read.", 4, 3),
				v188TestTag(t, "variant", "name", "B", 3, 2), v188TestText(t, "Write.", "Write.", 6, 3), v188TestClose(t, 3, 2)), "separate", nil)
		})
	}
}

// v188TestBase сохраняет пример таблицы; номера учитывают две вводные строки.
func v188TestBase(t *testing.T) []model.Line {
	t.Helper()
	return []model.Line{v188TestOpen(t, "variants", 2, 1), v188TestTag(t, "variant", "name", "A", 3, 2),
		v188TestText(t, "Read.", "Read.", 4, 3), v188TestClose(t, 3, 2)}
}

// v188TestInvalid задаёт ошибочную строку с сохранённым контекстом.
// Обязательную исходную диагностику для unparsed добавляет v188TestDocument.
func v188TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v188TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v188TestLine(t, raw, "invalid", parent, depth, element)
}

// v188TestRun запускает штатный конвейер и проверяет только V188 текущего запуска.
func v188TestRun(t *testing.T, result *model.Result, source string, want []v188Expected) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V188" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V188 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		message := "В @variants меньше двух ветвей @variant.\nконкретизация: Найдено ветвей: " + strconv.Itoa(expected.count) + "."
		primary := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "element" ||
			entry.Message != message ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != 0 {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		attached := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						attached++
						if line.Number != expected.line || element.ElementType != "tag" || element.Start != expected.start || element.End != expected.end {
							t.Fatal("finding attached outside target tag")
						}
					}
				}
			}
		}
		if attached != 1 {
			t.Fatalf("finding attached %d times, want 1", attached)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("missing error flags")
		}

	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original model.Result
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	for index, line := range original.Lines {
		current := result.Lines[index]
		current.HasErrors = line.HasErrors
		current.Elements = append([]model.Element{}, current.Elements...)
		for element := range current.Elements {
			current.Elements[element].ErrorIDs = line.Elements[element].ErrorIDs
		}
		left, _ := json.Marshal(line)
		right, _ := json.Marshal(current)
		if string(left) != string(right) {
			t.Fatalf("source changed on line %d", index+1)
		}
	}
	for index, old := range original.Diagnostics {
		left, _ := json.Marshal(old)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v188TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v188TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
	})
}

// v188TestElement задаёт исходное и семантическое значения без преобразований.
func v188TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v188TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v188TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v188TestTag создаёт однострочное объявление с явно заданным параметром.
func v188TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v188TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v188TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v188TestLine(t, raw, "tag", parent, depth, elements...)
}

// v188TestOpen создаёт объявление блока без параметров.
func v188TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v188TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v188TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v188TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v188TestClose создаёт закрытие с заданным родителем.
func v188TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v188TestLine(t, "}", "block-end", parent, depth, v188TestElement(t, "block-close", "}", "}", 1, 2))
}

// v188TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v188TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v188TestLine(t, raw, "content", parent, depth, v188TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v188TestBlank задаёт обычную пустую строку без текстового элемента.
func v188TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v188TestLine(t, raw, "blank", parent, depth)
}

// v188TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v188TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v188TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v188TestLine(t, "# Lesson", "heading", 0, 0, v188TestElement(t, "heading-level", "#", "1", 1, 2), v188TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		for j, element := range result.Lines[i].Elements {
			if element.ElementType == "unparsed" {
				v188TestError(t, result, i+1, j, "Строка разобрана не полностью.")
			}
		}
	}
	return result

}

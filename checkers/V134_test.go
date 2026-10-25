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

// v134Expected задаёт явные диапазоны [start,end) и конкретизацию.
// Каждая тройка содержит строку, начальную и конечную Unicode-колонки.
type v134Expected struct {
	primary [3]int
	related [3]int
	detail  string
}

func TestV134Containers(t *testing.T) {
	for _, tc := range []struct {
		tag    string
		end    int
		detail string
	}{
		{"example", 9, "Родительский блок: @example."}, {"wordlist", 10, "Родительский блок: @wordlist."},
		{"table", 7, "Родительский блок: @table."}, {"script", 8, "Родительский блок: @script."},
		{"text", 6, "Родительский блок: @text."}, {"key", 5, "Родительский блок: @key."},
		{"instr", 7, "Родительский блок: @instr."}, {"note", 6, "Родительский блок: @note."},
		{"alt", 5, "Родительский блок: @alt."}, {"fragment", 10, "Родительский блок: @fragment."},
		{"choice", 8, "Родительский блок: @choice."}, {"multichoice", 13, "Родительский блок: @multichoice."},
		{"ordering", 10, "Родительский блок: @ordering."},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			opening := v134TestOpen(t, tc.tag, 3, 2)
			if tc.tag == "fragment" {
				opening = v134TestFragment(t, "container", "container", 3, 2)
			}
			// key требует активного задания; интерактивным спискам оставлены два обычных элемента.
			result := v134TestDocument(t, v134TestTag(t, "task", "identifier", "t", 2, 1), opening,
				v134TestText(t, "First", "First", 4, 3), v134TestTag(t, "include", "identifier", "f", 4, 3),
				v134TestText(t, "Second", "Second", 4, 3), v134TestClose(t, 4, 3), v134TestTag(t, "endtask", "", "", 2, 1))
			v134TestRun(t, result, "current", []v134Expected{{[3]int{6, 1, 9}, [3]int{4, 1, tc.end}, tc.detail}})
		})
	}
	t.Run("matching before and after separator", func(t *testing.T) {
		result := v134TestDocument(t, v134TestOpen(t, "matching", 2, 1), v134TestText(t, "Question?", "Question?", 3, 2),
			v134TestTag(t, "include", "identifier", "f", 3, 2), v134TestLine(t, "---", "separator", 3, 2),
			v134TestText(t, "First", "First", 3, 2), v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestText(t, "Second", "Second", 3, 2), v134TestClose(t, 3, 2))
		v134TestRun(t, result, "current", []v134Expected{
			{[3]int{5, 1, 9}, [3]int{3, 1, 10}, "Родительский блок: @matching."},
			{[3]int{8, 1, 9}, [3]int{3, 1, 10}, "Родительский блок: @matching."}})
	})
	t.Run("answer after question", func(t *testing.T) {
		result := v134TestDocument(t, v134TestTag(t, "question", "content", "Name?", 2, 1), v134TestOpen(t, "answer", 2, 1),
			v134TestText(t, "Anna", "Anna", 4, 2), v134TestTag(t, "include", "identifier", "f", 4, 2), v134TestClose(t, 4, 2))
		v134TestRun(t, result, "current", []v134Expected{{[3]int{6, 1, 9}, [3]int{4, 1, 8}, "Родительский блок: @answer."}})
	})
	t.Run("hint in instruction chooses nearest block", func(t *testing.T) {
		result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1), v134TestText(t, "Read.", "Read.", 3, 2),
			v134TestOpen(t, "hint", 3, 2), v134TestText(t, "Think.", "Think.", 5, 3),
			v134TestTag(t, "include", "identifier", "f", 5, 3), v134TestClose(t, 5, 3), v134TestClose(t, 3, 2))
		v134TestRun(t, result, "current", []v134Expected{{[3]int{7, 1, 9}, [3]int{5, 1, 6}, "Родительский блок: @hint."}})
	})
	t.Run("fragment containing instruction", func(t *testing.T) {
		result := v134TestDocument(t, v134TestFragment(t, "g", "g", 2, 1), v134TestOpen(t, "instr", 3, 2),
			v134TestTag(t, "include", "identifier", "f", 4, 3), v134TestText(t, "Read.", "Read.", 4, 3),
			v134TestClose(t, 4, 3), v134TestClose(t, 3, 2))
		v134TestRun(t, result, "current", []v134Expected{{[3]int{5, 1, 9}, [3]int{4, 1, 7}, "Родительский блок: @instr."}})
	})
	t.Run("task cannot bypass instruction", func(t *testing.T) {
		result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1), v134TestTag(t, "task", "identifier", "t", 3, 2),
			v134TestTag(t, "include", "identifier", "f", 4, 3), v134TestClose(t, 3, 2))
		v134TestError(t, result, 4, -1, "Задание внутри инструкции запрещено.")
		v134TestRun(t, result, "current", []v134Expected{{[3]int{5, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}})
	})
	t.Run("multiple includes and end of block", func(t *testing.T) {
		result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1), v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestTag(t, "include", "identifier", "g", 3, 2), v134TestClose(t, 3, 2), v134TestTag(t, "include", "identifier", "f", 2, 1))
		v134TestRun(t, result, "current", []v134Expected{
			{[3]int{4, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."},
			{[3]int{5, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}})
	})
}

func TestV134RecoveryAndSourcePositions(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v134Expected
	}{
		{"missing identifier", v134TestTag(t, "include", "", "", 3, 2),
			[]v134Expected{{[3]int{4, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}}},
		{"partial invalid declaration", v134TestLine(t, "@include f extra", "invalid", 3, 2,
			v134TestElement(t, "tag", "@include", "include", 1, 9), v134TestElement(t, "identifier", "f", "f", 10, 11),
			v134TestUnparsed(t, "extra", 12, 17)),
			[]v134Expected{{[3]int{4, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}}},
		{"case and indentation", v134TestLine(t, " \t@InClUdE ф", "tag", 3, 2,
			v134TestElement(t, "tag", "@InClUdE", "include", 3, 11), v134TestElement(t, "identifier", "ф", "ф", 12, 13)),
			[]v134Expected{{[3]int{4, 3, 11}, [3]int{3, 1, 7}, "Родительский блок: @instr."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1), tc.line,
				v134TestText(t, "Read.", "Read.", 3, 2), v134TestClose(t, 3, 2))
			if tc.name == "missing identifier" {
				v134TestError(t, result, 4, -1, "Отсутствует ID включения.")
			}
			v134TestRun(t, result, "current", tc.want)
		})
	}
	for _, tc := range []struct {
		raw        string
		start, end int
		report     bool
	}{
		{"@include", 1, 9, true}, {"@include f extra", 1, 9, true}, {"@include\tf", 1, 9, true},
		{"@include{", 1, 9, true}, {" \t@INCLUDE\tf", 3, 11, true},
		{"@includeX f", 0, 0, false}, {"@include! f", 0, 0, false}, {"@include-ф", 0, 0, false},
		{"@include\u00a0f", 0, 0, false}, {"@included f", 0, 0, false},
	} {
		t.Run("unparsed "+tc.raw, func(t *testing.T) {
			line := v134TestLine(t, tc.raw, "invalid", 3, 2, v134TestUnparsed(t, tc.raw, 1, utf8.RuneCountInString(tc.raw)+1))
			result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1), line,
				v134TestText(t, "Read.", "Read.", 3, 2), v134TestClose(t, 3, 2))
			var want []v134Expected
			if tc.report {
				want = []v134Expected{{[3]int{4, tc.start, tc.end}, [3]int{3, 1, 7}, "Родительский блок: @instr."}}
			}
			v134TestRun(t, result, "current", want)
		})
	}
	t.Run("unparsed without invalid line type", func(t *testing.T) {
		result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1),
			v134TestLine(t, "@include\tf", "tag", 3, 2, v134TestUnparsed(t, "@include\tf", 1, 11)),
			v134TestText(t, "Read.", "Read.", 3, 2), v134TestClose(t, 3, 2))
		v134TestRun(t, result, "current", []v134Expected{{[3]int{4, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}})
	})
	t.Run("indented parent and include with Unicode ID", func(t *testing.T) {
		result := v134TestDocument(t,
			v134TestLine(t, "\t@InStR {", "block-start", 2, 1, v134TestElement(t, "tag", "@InStR", "instr", 2, 8),
				v134TestElement(t, "block-open", "{", "{", 9, 10)),
			v134TestTag(t, "include", "identifier", "фрагмент", 3, 2), v134TestClose(t, 3, 2))
		v134TestRun(t, result, "current", []v134Expected{{[3]int{4, 1, 9}, [3]int{3, 2, 8}, "Родительский блок: @instr."}})
	})
}

func TestV134Exclusions(t *testing.T) {
	for _, tag := range []string{"editor", "multifill"} {
		t.Run(tag, func(t *testing.T) {
			result := v134TestDocument(t, v134TestOpen(t, tag, 2, 1),
				v134TestText(t, "@include f", "@include f", 3, 2), v134TestText(t, "@include{", "@include{", 3, 2),
				v134TestText(t, "# Literal heading", "# Literal heading", 3, 2),
				v134TestText(t, "@variant Fake", "@variant Fake", 3, 2), v134TestClose(t, 3, 2))
			v134TestRun(t, result, "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@include f`, "@include f"}, {"Read @include f", "Read @include f"},
		{`<span>@include f</span>`, `<span>@include f</span>`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v134TestRun(t, v134TestDocument(t, v134TestOpen(t, "text", 2, 1),
				v134TestText(t, tc.raw, tc.value, 3, 2), v134TestClose(t, 3, 2)), "current", nil)
		})
	}
}

func TestV134HeadingAndEmptyInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines func(*model.Result) []model.Line
		want  []v134Expected
	}{
		{"before heading", func(base *model.Result) []model.Line {
			return []model.Line{base.Lines[0], v134TestTag(t, "include", "identifier", "f", 0, 0), base.Lines[1]}
		}, []v134Expected{{[3]int{2, 1, 9}, [3]int{}, ""}}},
		{"no heading", func(base *model.Result) []model.Line {
			base.Document.Metadata.Title = nil
			return []model.Line{base.Lines[0], v134TestTag(t, "include", "identifier", "f", 0, 0)}
		}, []v134Expected{{[3]int{2, 1, 9}, [3]int{}, ""}}},
		{"first physical line", func(base *model.Result) []model.Line {
			return []model.Line{v134TestTag(t, "include", "identifier", "f", 0, 0), base.Lines[0], base.Lines[1]}
		}, []v134Expected{{[3]int{1, 1, 9}, [3]int{}, ""}}},
		{"literal heading does not enable include", func(base *model.Result) []model.Line {
			return []model.Line{base.Lines[0], v134TestOpen(t, "editor", 0, 0),
				v134TestText(t, "# Fake", "# Fake", 2, 1), v134TestClose(t, 2, 1),
				v134TestTag(t, "include", "identifier", "f", 0, 0), base.Lines[1]}
		}, []v134Expected{{[3]int{5, 1, 9}, [3]int{}, ""}}},
		{"lower heading is not main heading", func(base *model.Result) []model.Line {
			return []model.Line{base.Lines[0], v134TestHeading(t, "###", "3", "Step"),
				v134TestTag(t, "include", "identifier", "f", 2, 1), base.Lines[1]}
		}, []v134Expected{{[3]int{3, 1, 9}, [3]int{}, ""}}},
		{"before heading with forbidden parent", func(base *model.Result) []model.Line {
			return []model.Line{base.Lines[0], v134TestOpen(t, "instr", 0, 0),
				v134TestTag(t, "include", "identifier", "f", 2, 1), v134TestClose(t, 2, 1), base.Lines[1]}
		}, []v134Expected{{[3]int{3, 1, 9}, [3]int{2, 1, 7}, "Родительский блок: @instr."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v134TestDocument(t)
			result.Lines = tc.lines(result)
			count := len(result.Lines)
			result.Document.LineCount = &count
			for index := range result.Lines {
				result.Lines[index].Number = index + 1
				result.Lines[index].LineEnding = "\n"
			}
			result.Lines[count-1].LineEnding = ""
			v134TestError(t, result, tc.want[0].primary[0], -1, "Отображаемое содержимое до основного заголовка.")
			v134TestRun(t, result, "current", tc.want)
		})
	}
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v134TestRun(t, result, "empty", nil)
		})
	}
}

func TestV134EndingsHistoryAndVersions(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1),
				v134TestText(t, "Прочитайте.", "Прочитайте.", 3, 2), v134TestTag(t, "include", "identifier", "ф", 3, 2), v134TestClose(t, 3, 2))
			if ending != "" {
				for index := range result.Lines {
					result.Lines[index].LineEnding = ending
				}
			}
			want := []v134Expected{{[3]int{5, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}}
			v134TestRun(t, result, "first", want)
			v134TestRun(t, result, "second", want)
			v134TestRun(t, v134TestDocument(t, v134TestTag(t, "include", "identifier", "ф", 2, 1)), "separate", nil)
		})
	}
	t.Run("include at EOF in unclosed block", func(t *testing.T) {
		result := v134TestDocument(t, v134TestOpen(t, "instr", 2, 1), v134TestTag(t, "include", "identifier", "f", 3, 2))
		v134TestError(t, result, 3, -1, "Блок не закрыт до конца файла.")
		v134TestRun(t, result, "current", []v134Expected{{[3]int{4, 1, 9}, [3]int{3, 1, 7}, "Родительский блок: @instr."}})
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v134TestDocument(t)
			result.Document.DSLVersion = &version
			result.Lines[0] = v134TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			registry, err := diagnostics.NewRegistry(RegistrationCheckers())
			if err != nil {
				t.Fatal(err)
			}
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

func TestV134Registration(t *testing.T) {
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
		if definition.Code != "V134" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "@include находится в месте, где вставки запрещены." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V134 definitions = %d, want 1", count)
	}
	if RegistrationCheckers()["V076"] == nil {
		t.Fatal("V076 must remain connected")
	}
}

// v134TestRun запускает штатный конвейер и сравнивает только V134 текущего запуска.
func v134TestRun(t *testing.T, result *model.Result, source string, want []v134Expected) {
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
		if diagnostic.DiagnosticCode == "V134" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V134 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		diagnostic := got[index]
		message := "@include находится в месте, где вставки запрещены."
		if expected.detail != "" {
			message += "\nконкретизация: " + expected.detail
		}
		position := model.Location{Start: model.Position{Line: expected.primary[0], Column: expected.primary[1]},
			End: model.Position{Line: expected.primary[0], Column: expected.primary[2]}}
		if diagnostic.ID == "" || diagnostic.Fatal || diagnostic.SeverityLevel != "error" || diagnostic.DiagnosticScope != "line" ||
			diagnostic.Message != message || diagnostic.Location == nil || *diagnostic.Location != position {
			t.Fatalf("occurrence %d = %+v, want %+v, message %q", index, diagnostic, expected, message)
		}
		if expected.related == [3]int{} {
			if len(diagnostic.RelatedLocations) != 0 {
				t.Fatalf("unexpected related: %+v", diagnostic.RelatedLocations)
			}
		} else {
			related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
				End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
			if len(diagnostic.RelatedLocations) != 1 || diagnostic.RelatedLocations[0] != related {
				t.Fatalf("related = %+v, want %+v", diagnostic.RelatedLocations, related)
			}
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
		if current.Raw != line.Raw || current.LineEnding != line.LineEnding || current.NestingLevel != line.NestingLevel {
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

func TestV134ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v134Expected
	}{
		{"reference", "@fragment f {\nRead this text.\n}\n@include f", []model.Line{
			v134TestFragment(t, "f", "f", 2, 1), v134TestText(t, "Read this text.", "Read this text.", 3, 2),
			v134TestClose(t, 3, 2), v134TestTag(t, "include", "identifier", "f", 2, 1)}, nil},
		{"example", "@fragment f {\nRead this text.\n}\n@instr {\nRead carefully.\n@include f\n}", []model.Line{
			v134TestFragment(t, "f", "f", 2, 1), v134TestText(t, "Read this text.", "Read this text.", 3, 2),
			v134TestClose(t, 3, 2), v134TestOpen(t, "instr", 2, 1), v134TestText(t, "Read carefully.", "Read carefully.", 6, 2),
			v134TestTag(t, "include", "identifier", "f", 6, 2), v134TestClose(t, 6, 2)},
			[]v134Expected{{[3]int{8, 1, 9}, [3]int{6, 1, 7}, "Родительский блок: @instr."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.30: перед дословным фрагментом добавлены версия и # Lesson.
			result := v134TestDocument(t, tc.body...)
			var source strings.Builder
			for _, line := range result.Lines[2:] {
				source.WriteString(line.Raw)
				source.WriteString(string(line.LineEnding))
			}
			if source.String() != tc.fragment {
				t.Fatalf("fragment changed: %q", source.String())
			}
			v134TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV134StructuralAreas(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"implicit step", []model.Line{v134TestTag(t, "include", "identifier", "missing", 2, 1)}},
		{"third level heading", []model.Line{v134TestHeading(t, "###", "3", "Step"), v134TestTag(t, "include", "identifier", "f", 3, 1)}},
		{"newpage", []model.Line{v134TestTag(t, "newpage", "", "", 2, 1), v134TestTag(t, "include", "identifier", "f", 3, 2)}},
		{"task and steps", []model.Line{
			v134TestTag(t, "task", "identifier", "t", 2, 1), v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestTag(t, "step", "title", "First", 3, 2), v134TestTag(t, "include", "identifier", "f", 5, 3),
			v134TestTag(t, "step", "title", "Second", 3, 2), v134TestTag(t, "include", "identifier", "f", 7, 3),
			v134TestTag(t, "endtask", "", "", 2, 1), v134TestTag(t, "include", "identifier", "f", 2, 1)}},
		{"header closes task", []model.Line{
			v134TestTag(t, "task", "identifier", "t", 2, 1), v134TestText(t, "Read.", "Read.", 3, 2),
			v134TestTag(t, "header", "title", "More", 2, 1), v134TestTag(t, "include", "identifier", "f", 5, 2)}},
		{"forward reference", []model.Line{
			v134TestTag(t, "include", "identifier", "f", 2, 1), v134TestFragment(t, "f", "f", 2, 1),
			v134TestText(t, "Read.", "Read.", 4, 2), v134TestClose(t, 4, 2)}},
		{"one-line content is not a container", []model.Line{
			v134TestTag(t, "instr", "content", "Read.", 2, 1), v134TestTag(t, "include", "identifier", "f", 2, 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) { v134TestRun(t, v134TestDocument(t, tc.body...), "current", nil) })
	}
}

func TestV134Variants(t *testing.T) {
	t.Run("variant inside instruction does not begin outer branch", func(t *testing.T) {
		// Между ложным началом ветви и variants находится instr. После его
		// закрытия include всё ещё расположен до первой настоящей ветви.
		result := v134TestDocument(t, v134TestOpen(t, "variants", 2, 1), v134TestOpen(t, "instr", 3, 2),
			v134TestTag(t, "variant", "name", "Fake", 4, 3), v134TestText(t, "Read.", "Read.", 4, 3),
			v134TestClose(t, 4, 3), v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestTag(t, "variant", "name", "A", 3, 2), v134TestText(t, "First.", "First.", 9, 3),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestText(t, "Second.", "Second.", 11, 3),
			v134TestClose(t, 3, 2))
		v134TestError(t, result, 4, -1, "Инструкция до первой ветви запрещена.")
		v134TestError(t, result, 5, -1, "Ветвь внутри инструкции запрещена.")
		v134TestRun(t, result, "current", []v134Expected{{[3]int{8, 1, 9}, [3]int{3, 1, 10}, "Родительский блок: @variants."}})
	})
	t.Run("branch cannot bypass outer fragment", func(t *testing.T) {
		// DSL 3.12.12 запрещает variants внутри fragment. Однако объявленная
		// ветвь уже началась: по V134 запрещает включение именно внешний fragment,
		// а не правило о содержимом variants до первой ветви.
		result := v134TestDocument(t, v134TestFragment(t, "g", "g", 2, 1), v134TestOpen(t, "variants", 3, 2),
			v134TestTag(t, "variant", "name", "A", 4, 3), v134TestTag(t, "include", "identifier", "f", 5, 4),
			v134TestTag(t, "variant", "name", "B", 4, 3), v134TestText(t, "Read.", "Read.", 7, 4),
			v134TestClose(t, 4, 3), v134TestClose(t, 3, 2))
		v134TestError(t, result, 4, -1, "Группа вариантов внутри фрагмента запрещена.")
		v134TestRun(t, result, "current", []v134Expected{{[3]int{6, 1, 9}, [3]int{3, 1, 10}, "Родительский блок: @fragment."}})
	})
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v134Expected
	}{
		{"logical parents", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "variant", "name", "A", 3, 2), v134TestTag(t, "include", "identifier", "f", 4, 3),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestTag(t, "include", "identifier", "f", 6, 3), v134TestClose(t, 3, 2)}, nil},
		{"shared parent and block close", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "variant", "name", "A", 3, 2), v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestTag(t, "include", "identifier", "f", 3, 2), v134TestClose(t, 3, 2),
			v134TestTag(t, "include", "identifier", "f", 2, 1)}, nil},
		{"before first branch", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestTag(t, "variant", "name", "A", 3, 2), v134TestText(t, "A.", "A.", 5, 3),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestText(t, "B.", "B.", 7, 3), v134TestClose(t, 3, 2)},
			[]v134Expected{{[3]int{4, 1, 9}, [3]int{3, 1, 10}, "Родительский блок: @variants."}}},
		{"second block needs own branch", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "variant", "name", "A", 3, 2), v134TestText(t, "A.", "A.", 4, 3),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestText(t, "B.", "B.", 6, 3), v134TestClose(t, 3, 2),
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "include", "identifier", "f", 9, 2),
			v134TestTag(t, "variant", "name", "C", 9, 2), v134TestText(t, "C.", "C.", 11, 3),
			v134TestTag(t, "variant", "name", "D", 9, 2), v134TestText(t, "D.", "D.", 13, 3), v134TestClose(t, 9, 2)},
			[]v134Expected{{[3]int{10, 1, 9}, [3]int{9, 1, 10}, "Родительский блок: @variants."}}},
		{"task in branch does not require include as direct model child", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "variant", "name", "A", 3, 2),
			v134TestTag(t, "task", "identifier", "t", 4, 3), v134TestTag(t, "step", "title", "First", 5, 4),
			v134TestTag(t, "include", "identifier", "f", 6, 5), v134TestTag(t, "step", "title", "Second", 5, 4),
			v134TestTag(t, "include", "identifier", "f", 8, 5), v134TestTag(t, "variant", "name", "B", 3, 2),
			v134TestTag(t, "include", "identifier", "f", 10, 3), v134TestClose(t, 3, 2)}, nil},
		{"content block inside branch", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestTag(t, "variant", "name", "A", 3, 2),
			v134TestOpen(t, "instr", 4, 3), v134TestText(t, "Read.", "Read.", 5, 4),
			v134TestTag(t, "include", "identifier", "f", 5, 4), v134TestClose(t, 5, 4),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestTag(t, "include", "identifier", "f", 9, 3), v134TestClose(t, 3, 2)},
			[]v134Expected{{[3]int{7, 1, 9}, [3]int{5, 1, 7}, "Родительский блок: @instr."}}},
		{"literal variant does not begin branch", []model.Line{
			v134TestOpen(t, "variants", 2, 1), v134TestOpen(t, "editor", 3, 2),
			v134TestText(t, "@variant Fake", "@variant Fake", 4, 3), v134TestClose(t, 4, 3),
			v134TestTag(t, "include", "identifier", "f", 3, 2),
			v134TestTag(t, "variant", "name", "A", 3, 2), v134TestText(t, "A.", "A.", 8, 3),
			v134TestTag(t, "variant", "name", "B", 3, 2), v134TestText(t, "B.", "B.", 10, 3), v134TestClose(t, 3, 2)},
			[]v134Expected{{[3]int{7, 1, 9}, [3]int{3, 1, 10}, "Родительский блок: @variants."}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v134TestRun(t, v134TestDocument(t, tc.body...), "current", tc.want) })
	}
}

// v134TestDocument добавляет обязательные версию и заголовок по DSL 1.30.
// Родители заданы тестами явно; ошибки unparsed получают исходные ссылки.
func v134TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v134TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v134TestHeading(t, "#", "1", "Lesson"),
	}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}}, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for index := range result.Lines {
		for element, value := range result.Lines[index].Elements {
			if value.ElementType == "unparsed" {
				v134TestError(t, result, index+1, element, "Ошибка разбора объявления.")
			}
		}
	}
	return result
}

// v134TestLine сохраняет явные тип, родителей и элементы без разбора исходника.
func v134TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v134TestElement задаёт исходное и семантическое значения независимо.
func v134TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v134TestUnparsed создаёт неразобранный фрагмент с null Value.
func v134TestUnparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	element := v134TestElement(t, "unparsed", raw, "", start, end)
	element.Value = nil
	return element
}

// v134TestFragment сохраняет полный ID, в том числе ошибочный, без обрезки.
func v134TestFragment(t *testing.T, rawID, value string, parent, depth int) model.Line {
	t.Helper()
	end := 11 + utf8.RuneCountInString(rawID)
	return v134TestLine(t, "@fragment "+rawID+" {", "block-start", parent, depth,
		v134TestElement(t, "tag", "@fragment", "fragment", 1, 10), v134TestElement(t, "identifier", rawID, value, 11, end),
		v134TestElement(t, "block-open", "{", "{", end+1, end+2))
}

// v134TestTag создаёт однострочное объявление с одним параметром.
func v134TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v134TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v134TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v134TestLine(t, raw, "tag", parent, depth, elements...)
}

// v134TestOpen создаёт блок без параметров.
func v134TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v134TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v134TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v134TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v134TestClose создаёт закрытие с явно указанным родителем.
func v134TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v134TestLine(t, "}", "block-end", parent, depth, v134TestElement(t, "block-close", "}", "}", 1, 2))
}

// v134TestText не преобразует Raw и Value друг в друга.
func v134TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v134TestLine(t, raw, "content", parent, depth, v134TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v134TestHeading создаёт корневой заголовок с заданным уровнем.
func v134TestHeading(t *testing.T, marker, level, title string) model.Line {
	t.Helper()
	return v134TestLine(t, marker+" "+title, "heading", 0, 0,
		v134TestElement(t, "heading-level", marker, level, 1, len(marker)+1),
		v134TestElement(t, "title", title, title, len(marker)+2, len(marker)+2+utf8.RuneCountInString(title)))
}

// v134TestError добавляет исходную ошибку; index=-1 задаёт область line.
// Все исходные ошибки parser упорядочиваются по позиции до запуска конвейера.
func v134TestError(t *testing.T, result *model.Result, number, index int, message string) {
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
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
}

package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v199TestMessage = "Указанная допустимая позиция недостижима для элемента в любом правильном полном порядке."

type v199TestWant struct {
	primary [3]int
	related [][3]int
}

type v199TestItem struct {
	label   string
	answers []string
	inline  bool
}

// v199TestElement создаёт цельное семантическое значение с исходным диапазоном.
func v199TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value,
		Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v199TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v199TestTag(t *testing.T, name, value string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	tag := "@" + name
	return v199TestLine(t, tag+" "+value, "tag", parent,
		v199TestElement(t, "tag", tag, name, 1), v199TestElement(t, kind, value, value, len(tag)+2))
}

func v199TestOpen(t *testing.T, name string, parent int) model.Line {
	t.Helper()
	tag := "@" + name
	return v199TestLine(t, tag+" {", "block-start", parent,
		v199TestElement(t, "tag", tag, name, 1), v199TestElement(t, "block-open", "{", "{", len(tag)+2))
}

func v199TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v199TestLine(t, "}", "block-end", parent, v199TestElement(t, "block-close", "}", "}", 1))
}

func v199TestText(t *testing.T, value string, parent int) model.Line {
	t.Helper()
	return v199TestLine(t, value, "content", parent, v199TestElement(t, "content", value, value, 1))
}

// v199TestDocument добавляет только обязательный внешний контекст по DSL 1.30.
// Типы и родители задаются явно; помощник не разбирает DSL.
func v199TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v199TestTag(t, "dsl-version", "1.2", "version", 0),
		v199TestLine(t, "# Lesson", "heading", 0, v199TestElement(t, "heading-level", "#", "1", 1), v199TestElement(t, "title", "Lesson", "Lesson", 3)),
		v199TestLine(t, "### Ordering", "heading", 0, v199TestElement(t, "heading-level", "###", "3", 1), v199TestElement(t, "title", "Ordering", "Ordering", 5)),
		v199TestTag(t, "task", "order", "identifier", 3),
	}
	lines = append(lines, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: "1.0", Lines: lines,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for index := range result.Lines {
		line := &result.Lines[index]
		line.Number = index + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
	}
	result.Lines[len(lines)-1].LineEnding = ""
	return result
}

// v199TestItems задаёт экземпляры и ответы без вычисления ожидаемых сообщений.
// nil answers означает отсутствие ответа, пустой непустой срез — пустой блок.
func v199TestItems(t *testing.T, items ...v199TestItem) *model.Result {
	t.Helper()
	body := []model.Line{v199TestOpen(t, "ordering", 4)}
	for _, item := range items {
		body = append(body, v199TestText(t, item.label, 5))
		if item.answers == nil {
			continue
		}
		if item.inline {
			if len(item.answers) != 1 {
				t.Fatal("inline fixture requires one answer")
			}
			body = append(body, v199TestTag(t, "answer", item.answers[0], "content", 5))
			continue
		}
		parent := len(body) + 5
		body = append(body, v199TestOpen(t, "answer", 5))
		for _, answer := range item.answers {
			body = append(body, v199TestText(t, answer, parent))
		}
		body = append(body, v199TestClose(t, parent))
	}
	return v199TestDocument(t, append(body, v199TestClose(t, 5))...)
}

func v199TestExample(t *testing.T) *model.Result {
	t.Helper()
	return v199TestItems(t, v199TestItem{"A", []string{"1.", "2!"}, false}, v199TestItem{"B", []string{"2"}, true})
}

func v199TestJSON(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v199TestLocation(expected [3]int) model.Location {
	return model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
		End: model.Position{Line: expected[0], Column: expected[2]}}
}

// v199TestRun использует весь штатный каталог и неизменённую выборку Prepare.
// Проверяет только новые V199; прочие диагностики не подменяют её результат.
func v199TestRun(t *testing.T, result *model.Result, source string, want []v199TestWant) {
	t.Helper()
	before := v199TestJSON(t, result)
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v199TestJSON(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldCount := len(result.Diagnostics)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V199" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V199 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		if entry.Location == nil || *entry.Location != v199TestLocation(expected.primary) ||
			entry.Message != v199TestMessage || entry.SeverityLevel != "warning" || entry.DiagnosticScope != "element" ||
			entry.Fatal || entry.Source != source || entry.ID == "" || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V199[%d] = %+v, want %+v with exact metadata and no detail", index, entry, expected)
		}
		for index, related := range expected.related {
			if entry.RelatedLocations[index] != v199TestLocation(related) {
				t.Fatalf("related[%d] = %+v, want %v", index, entry.RelatedLocations[index], related)
			}
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					links++
					if line.Number != expected.primary[0] || element.Start != expected.primary[1] || element.End != expected.primary[2] {
						t.Fatal("V199 link outside its primary content")
					}
				}
			}
		}
		if links != 1 {
			t.Fatalf("V199 has %d element links, want exactly one", links)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v199TestJSON(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for index := range restored.Lines {
		line, old := &restored.Lines[index], original.Lines[index]
		if len(line.Elements) != len(old.Elements) {
			t.Fatal("element count changed")
		}
		line.HasErrors = old.HasErrors
		for index := range line.Elements {
			prior, current := old.Elements[index].ErrorIDs, line.Elements[index].ErrorIDs
			if len(current) < len(prior) || !slices.Equal(prior, current[:len(prior)]) {
				t.Fatal("prior element links changed")
			}
			line.Elements[index].ErrorIDs = prior
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v199TestJSON(t, &restored) != before {
		t.Fatal("source values, ranges, metadata or prior history changed")
	}
}

func TestV199Registration(t *testing.T) {
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
		if definition.Code != "V199" {
			continue
		}
		count++
		if definition.Message != v199TestMessage || definition.Severity != "warning" || definition.Scope != "element" ||
			definition.Versions.Mode != diagnostics.VersionExact || definition.Versions.Version != "1.2" || definition.Check == nil {
			t.Fatalf("wrong definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V199 registration count = %d", count)
	}
	for _, version := range []string{"", "1.1", "1.3", "2.0"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v199TestExample(t)
			result.Document.DSLVersion = &version
			before := v199TestJSON(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatal("unsupported version accepted")
			}
			if v199TestJSON(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV199ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, fragment string
		result         *model.Result
		want           []v199TestWant
	}{
		{"reference", "@ordering {\nA\n@answer 1.\nB\n@answer 2.\n}",
			v199TestItems(t, v199TestItem{"A", []string{"1."}, true}, v199TestItem{"B", []string{"2."}, true}), nil},
		{"example", "@ordering {\nA\n@answer {\n1.\n2!\n}\nB\n@answer 2\n}",
			v199TestExample(t), []v199TestWant{{[3]int{9, 1, 3}, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.result.Lines[4:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v199TestRun(t, tc.result, "current", tc.want)
		})
	}
}

func TestV199MatchingAndGrouping(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []v199TestItem
		want  []v199TestWant
	}{
		{"repeated alternative", []v199TestItem{{"A", []string{"1.", "2!", "2."}, false}, {"B", []string{"2"}, true}},
			[]v199TestWant{{[3]int{9, 1, 3}, [][3]int{{10, 1, 3}}}}},
		{"multiple unreachable positions", []v199TestItem{{"A", []string{"1", "2", "3"}, false}, {"B", []string{"2"}, true}, {"C", []string{"3"}, true}},
			[]v199TestWant{{[3]int{9, 1, 2}, nil}, {[3]int{10, 1, 2}, nil}}},
		{"all edges reachable", []v199TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"1", "2"}, false}}, nil},
		{"no complete matching", []v199TestItem{{"A", []string{"1"}, true}, {"B", []string{"1"}, true}}, nil},
		{"Hall deficiency", []v199TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"1", "2"}, false}, {"C", []string{"1", "2"}, false}, {"D", []string{"3", "4"}, false}}, nil},
		{"later group instance saves position", []v199TestItem{{"A", []string{"1", "2!"}, false}, {"a", []string{"2"}, true}}, nil},
		{"earlier group instance saves position", []v199TestItem{{"A", []string{"2"}, true}, {"a", []string{"1", "2!"}, false}}, nil},
		{"group preserves individual restrictions", []v199TestItem{{"A", []string{"1", "3!"}, false}, {"a", []string{"2", "3."}, false}, {"B", []string{"3"}, true}},
			[]v199TestWant{{[3]int{9, 1, 3}, [][3]int{{14, 1, 3}}}}},
		{"matching needs reassignment", []v199TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"1"}, true}, {"C", []string{"2", "3"}, false}},
			[]v199TestWant{{[3]int{8, 1, 2}, nil}, {[3]int{15, 1, 2}, nil}}},
		{"last item owns first position", []v199TestItem{{"B", []string{"2"}, true}, {"A", []string{"2!", "1"}, false}},
			[]v199TestWant{{[3]int{10, 1, 3}, nil}}},
		{"different groups same dead position", []v199TestItem{{"A", []string{"1", "3"}, false}, {"B", []string{"2", "3"}, false}, {"C", []string{"3"}, true}},
			[]v199TestWant{{[3]int{9, 1, 2}, nil}, {[3]int{14, 1, 2}, nil}}},
		{"reference order differs from position order", []v199TestItem{{"A", []string{"3", "2", "1"}, false}, {"B", []string{"2"}, true}, {"C", []string{"3"}, true}},
			[]v199TestWant{{[3]int{8, 1, 2}, nil}, {[3]int{9, 1, 2}, nil}}},
		{"single element", []v199TestItem{{"A", []string{"1"}, true}}, nil},
		{"empty ordering", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { v199TestRun(t, v199TestItems(t, tc.items...), "current", tc.want) })
	}
}

func TestV199LabelNormalization(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		unreachable         bool
	}{
		{"Unicode lowercase", "ЁЖ", "ёж", false},
		{"Unicode whitespace", "А\t Б", "а\u00a0б", false},
		{"punctuation then whitespace", "А ! Б", "а б", false},
		{"edge punctuation then whitespace", "! А !", "а", false},
		{"punctuation deletion joins letters", "А—Б", "аб", false},
		{"identical punctuation primary", "!", "!", false},
		{"distinct empty secondary forms", "!", "?", true},
		{"diacritics preserved", "é", "e\u0301", true},
		{"special symbols preserved", "A+", "A", true},
		{"digits preserved", "A1", "A2", true},
		{"no Unicode compatibility normalization", "Ａ", "A", true},
		{"no case folding beyond lowercase", "Straße", "STRASSE", true},
		{"empty first primary", "\u00a0", "B", false},
		{"empty last primary", "A", "\u2003", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []v199TestWant
			if tc.unreachable {
				want = []v199TestWant{{[3]int{9, 1, 3}, nil}}
			}
			v199TestRun(t, v199TestItems(t, v199TestItem{tc.first, []string{"1.", "2!"}, false}, v199TestItem{tc.second, []string{"2"}, true}), "current", want)
		})
	}
}

func TestV199AnswerNormalizationAndResolution(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		end          int
	}{
		{"exact position", "2", 2},
		{"ASCII punctuation", "2!", 3},
		{"Unicode punctuation", "“2”—", 5},
		{"punctuation and spaces", "( 2 )", 6},
		{"minus is punctuation", "-2", 3},
		{"out of range", "3", 0},
		{"zero", "0", 0},
		{"leading zero", "02", 0},
		{"plus is symbol", "+2", 0},
		{"Arabic digit", "٢", 0},
		{"full width digit", "２", 0},
		{"punctuation only", "!", 0},
		{"empty primary", "\u00a0", 0},
		{"zero width space retained", "2\u200b", 0},
		{"combining accent retained", "2\u0301", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []v199TestWant
			if tc.end != 0 {
				want = []v199TestWant{{[3]int{9, 1, tc.end}, nil}}
			}
			v199TestRun(t, v199TestItems(t, v199TestItem{"A", []string{"1", tc.answer}, false}, v199TestItem{"B", []string{"2"}, true}), "current", want)
		})
	}
	t.Run("all alternatives must resolve", func(t *testing.T) {
		for _, alternatives := range [][]string{{"3", "1", "2!"}, {"1", "3", "2!"}, {"1", "2!", "3"}} {
			v199TestRun(t, v199TestItems(t, v199TestItem{"A", alternatives, false}, v199TestItem{"B", []string{"2"}, true}), "current", nil)
		}
	})
	t.Run("two digit position uses full normalization", func(t *testing.T) {
		items := []v199TestItem{{"A", []string{"1", "1.0"}, false}}
		for number := 2; number <= 10; number++ {
			items = append(items, v199TestItem{"Item " + strconv.Itoa(number), []string{strconv.Itoa(number)}, true})
		}
		v199TestRun(t, v199TestItems(t, items...), "current", []v199TestWant{{[3]int{9, 1, 4}, nil}})
	})
}

func TestV199CompleteSetup(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []v199TestItem
	}{
		{"no answers", []v199TestItem{{"A", nil, false}, {"B", nil, false}}},
		{"missing first answer", []v199TestItem{{"A", nil, false}, {"B", []string{"2"}, true}}},
		{"missing last answer", []v199TestItem{{"A", []string{"1", "2"}, false}, {"B", nil, false}}},
		{"empty answer block", []v199TestItem{{"A", []string{}, false}, {"B", []string{"2"}, true}}},
		{"inline whitespace answer", []v199TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"\u00a0"}, true}}},
		{"unresolved inline answer", []v199TestItem{{"A", []string{"1", "2"}, false}, {"B", []string{"3"}, true}}},
		{"inline list is one value", []v199TestItem{{"A", []string{"1 2"}, true}, {"B", []string{"2"}, true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v199TestRun(t, v199TestItems(t, tc.items...), "current", nil)
		})
	}
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"orphan answer", []model.Line{
			v199TestOpen(t, "ordering", 4), v199TestTag(t, "answer", "1", "content", 5),
			v199TestText(t, "A", 5), v199TestTag(t, "answer", "1", "content", 5),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5)}},
		{"duplicate answer", []model.Line{
			v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 5),
			v199TestTag(t, "answer", "1", "content", 5), v199TestTag(t, "answer", "2", "content", 5),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5)}},
		{"hint interrupts item answer link", []model.Line{
			v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 5),
			v199TestTag(t, "hint", "Hint", "content", 5), v199TestTag(t, "answer", "1", "content", 5),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5)}},
		{"answer block containing only blank lines", []model.Line{
			v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 5), v199TestOpen(t, "answer", 5),
			v199TestLine(t, " \t", "blank", 7), v199TestClose(t, 7),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5)}},
		{"nested answer is not an alternative", []model.Line{
			v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 5), v199TestOpen(t, "answer", 5),
			v199TestText(t, "1", 7), v199TestText(t, "2!", 7),
			v199TestTag(t, "answer", "1", "content", 7), v199TestClose(t, 7),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v199TestRun(t, v199TestDocument(t, tc.body...), "current", nil)
		})
	}
	t.Run("unclosed ordering", func(t *testing.T) {
		result := v199TestExample(t)
		result.Lines = result.Lines[:len(result.Lines)-1]
		*result.Document.LineCount = len(result.Lines)
		v199TestRun(t, result, "current", nil)
	})
}

func TestV199BlankLinesAndHints(t *testing.T) {
	t.Run("blank before answer preserves adjacency", func(t *testing.T) {
		result := v199TestDocument(t,
			v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 5), v199TestLine(t, "", "blank", 5),
			v199TestOpen(t, "answer", 5), v199TestText(t, "1.", 8), v199TestText(t, "2!", 8), v199TestClose(t, 8),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5))
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{10, 1, 3}, nil}})
	})
	t.Run("blank alternatives are ignored separately", func(t *testing.T) {
		result := v199TestDocument(t,
			v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 5), v199TestOpen(t, "answer", 5),
			v199TestLine(t, "\t", "blank", 7), v199TestText(t, "1.", 7), v199TestLine(t, "", "blank", 7),
			v199TestText(t, "2!", 7), v199TestClose(t, 7), v199TestLine(t, " ", "blank", 5),
			v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5))
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{11, 1, 3}, nil}})
	})
	for _, tc := range []struct {
		name string
		hint []model.Line
	}{
		{"inline hint", []model.Line{v199TestTag(t, "hint", "3", "content", 5)}},
		{"block hint", []model.Line{v199TestOpen(t, "hint", 5), v199TestText(t, "---", 11),
			v199TestText(t, "3", 11), v199TestClose(t, 11)}},
		{"blank before hint", []model.Line{v199TestLine(t, "", "blank", 5), v199TestTag(t, "hint", "3", "content", 5)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := append([]model.Line{}, v199TestExample(t).Lines[4:10]...)
			body = append(body, tc.hint...)
			body = append(body, v199TestText(t, "B", 5), v199TestTag(t, "answer", "2", "content", 5), v199TestClose(t, 5))
			v199TestRun(t, v199TestDocument(t, body...), "current", []v199TestWant{{[3]int{9, 1, 3}, nil}})
		})
	}
	t.Run("hint after inline answer", func(t *testing.T) {
		body := append([]model.Line{}, v199TestExample(t).Lines[4:12]...)
		body = append(body, v199TestOpen(t, "hint", 5), v199TestText(t, "3", 13), v199TestClose(t, 13), v199TestClose(t, 5))
		v199TestRun(t, v199TestDocument(t, body...), "current", []v199TestWant{{[3]int{9, 1, 3}, nil}})
	})
}

func TestV199ScopeAndBlockBoundaries(t *testing.T) {
	t.Run("extended format", func(t *testing.T) {
		body := append([]model.Line{}, v199TestExample(t).Lines[4:12]...)
		body = append(body, v199TestLine(t, "---", "separator", 5),
			v199TestText(t, "1", 5), v199TestText(t, "2", 5), v199TestClose(t, 5))
		v199TestRun(t, v199TestDocument(t, body...), "current", nil)
	})
	t.Run("escaped separator remains an item", func(t *testing.T) {
		result := v199TestExample(t)
		result.Lines[5].Raw = "\\---"
		result.Lines[5].Elements = []model.Element{v199TestElement(t, "content", "\\---", "---", 1)}
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{9, 1, 3}, nil}})
	})
	t.Run("two independent blocks", func(t *testing.T) {
		body := append([]model.Line{}, v199TestExample(t).Lines[4:]...)
		body = append(body, v199TestOpen(t, "ordering", 4), v199TestText(t, "A", 14), v199TestOpen(t, "answer", 14),
			v199TestText(t, "1.", 16), v199TestText(t, "2!", 16), v199TestClose(t, 16),
			v199TestText(t, "B", 14), v199TestTag(t, "answer", "2", "content", 14), v199TestClose(t, 14))
		body[8].LineEnding = "\n"
		v199TestRun(t, v199TestDocument(t, body...), "current",
			[]v199TestWant{{[3]int{9, 1, 3}, nil}, {[3]int{18, 1, 3}, nil}})
	})
	t.Run("valid variants parent", func(t *testing.T) {
		// variants находится на уровне шага; предыдущее задание завершается.
		result := v199TestDocument(t,
			v199TestLine(t, "@endtask", "tag", 3, v199TestElement(t, "tag", "@endtask", "endtask", 1)),
			v199TestOpen(t, "variants", 3), v199TestTag(t, "variant", "First", "name", 6),
			v199TestOpen(t, "ordering", 7), v199TestText(t, "A", 8), v199TestOpen(t, "answer", 8),
			v199TestText(t, "1.", 10), v199TestText(t, "2!", 10), v199TestClose(t, 10),
			v199TestText(t, "B", 8), v199TestTag(t, "answer", "2", "content", 8), v199TestClose(t, 8),
			v199TestTag(t, "variant", "Second", "name", 6), v199TestText(t, "Read.", 17), v199TestClose(t, 6))
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{12, 1, 3}, nil}})
	})
	t.Run("ordering spelling inside editor is literal", func(t *testing.T) {
		result := v199TestDocument(t, v199TestOpen(t, "editor", 4),
			v199TestText(t, "@ordering {", 5), v199TestText(t, "A", 5),
			v199TestText(t, "@answer 2!", 5), v199TestClose(t, 5))
		v199TestRun(t, result, "current", nil)
	})
	t.Run("other interactive tag", func(t *testing.T) {
		result := v199TestDocument(t, v199TestOpen(t, "matching", 4),
			v199TestText(t, "A", 5), v199TestOpen(t, "answer", 5), v199TestText(t, "1.", 7),
			v199TestText(t, "2!", 7), v199TestClose(t, 7), v199TestText(t, "B", 5),
			v199TestTag(t, "answer", "2", "content", 5), v199TestLine(t, "---", "separator", 5),
			v199TestText(t, "1", 5), v199TestText(t, "2", 5), v199TestClose(t, 5))
		v199TestRun(t, result, "current", nil)
	})
}

func TestV199SourceValuesAndRanges(t *testing.T) {
	t.Run("source range survives whitespace normalization", func(t *testing.T) {
		result := v199TestExample(t)
		result.Lines[8].Raw = "\t2!\u00a0"
		result.Lines[8].Elements = []model.Element{v199TestElement(t, "content", "2!\u00a0", "2!\u00a0", 2)}
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{9, 2, 5}, nil}})
	})
	t.Run("Unicode content range and repeated reference", func(t *testing.T) {
		result := v199TestItems(t, v199TestItem{"A", []string{"1", "«2»", "2!"}, false}, v199TestItem{"B", []string{"2"}, true})
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{9, 1, 4}, [][3]int{{10, 1, 3}}}})
	})
	t.Run("semantic labels are already unescaped", func(t *testing.T) {
		result := v199TestExample(t)
		// \{ снимает экранирование; неэкранированная { не используется как текст.
		result.Lines[5].Raw = "\\{"
		result.Lines[5].Elements = []model.Element{v199TestElement(t, "content", "\\{", "{", 1)}
		result.Lines[10].Raw = "\\{"
		result.Lines[10].Elements = []model.Element{v199TestElement(t, "content", "\\{", "{", 1)}
		v199TestRun(t, result, "current", nil)
	})
	t.Run("semantic labels must not be decoded twice", func(t *testing.T) {
		result := v199TestExample(t)
		// Три обратные косые черты перед { дают буквальные \ и {.
		// Повторное декодирование ошибочно сделало бы подписи одинаковыми.
		result.Lines[5].Raw = "\\\\\\{"
		result.Lines[5].Elements = []model.Element{v199TestElement(t, "content", "\\\\\\{", "\\{", 1)}
		result.Lines[10].Raw = "\\{"
		result.Lines[10].Elements = []model.Element{v199TestElement(t, "content", "\\{", "{", 1)}
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{9, 1, 3}, nil}})
	})
	t.Run("escaped brace in answer uses its original range", func(t *testing.T) {
		result := v199TestExample(t)
		result.Lines[8].Raw = "2\\}"
		result.Lines[8].Elements = []model.Element{v199TestElement(t, "content", "2\\}", "2}", 1)}
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{9, 1, 4}, nil}})
	})
	t.Run("canonical tag names retain source case", func(t *testing.T) {
		result := v199TestExample(t)
		result.Lines[4].Raw = "@OrDeRiNg {"
		result.Lines[4].Elements[0].Raw = "@OrDeRiNg"
		result.Lines[6].Raw = "@AnSwEr {"
		result.Lines[6].Elements[0].Raw = "@AnSwEr"
		v199TestRun(t, result, "current", []v199TestWant{{[3]int{9, 1, 3}, nil}})
	})
}

func TestV199EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v199TestExample(t)
				for index := range result.Lines {
					result.Lines[index].LineEnding = ending
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := []v199TestWant{{[3]int{9, 1, 3}, nil}}
				v199TestRun(t, result, "first", want)
				v199TestRun(t, result, "second", want)
				v199TestRun(t, v199TestItems(t, v199TestItem{"A", []string{"1"}, true}, v199TestItem{"B", []string{"2"}, true}), "other", nil)
			})
		}
	}
	t.Run("empty input", func(t *testing.T) {
		result := v199TestDocument(t)
		result.Lines = []model.Line{}
		*result.Document.LineCount = 0
		result.Document.Metadata.Title = nil
		v199TestRun(t, result, "current", nil)
	})
}

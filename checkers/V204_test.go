package checkers

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV204RegistrationAndVersions(t *testing.T) {
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
		if definition.Code != "V204" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "element" || definition.Check == nil ||
			definition.Message != "Регистр заголовка @note отличается от схемы «Тема» или «РАЗДЕЛ: Тема»." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V204 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatal("unsupported version accepted")
			}
			result := v204Document(t)
			result.Lines, result.Document.LineCount, result.Document.Metadata.Title = []model.Line{}, nil, nil
			result.Document.DSLVersion = &version
			v204Refusal(t, result)
		})
	}
}

func TestV204TitleCases(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"single uppercase", "А", "А", nil},
		{"single lowercase", "а", "а", [][3]int{{3, 7, 8}}},
		{"sentence", "Тема", "Тема", nil},
		{"lowercase first", "тема", "тема", [][3]int{{3, 7, 11}}},
		{"all uppercase", "ТЕМА", "ТЕМА", [][3]int{{3, 7, 11}}},
		{"new word does not reset", "Тема Урока", "Тема Урока", [][3]int{{3, 7, 17}}},
		{"no abbreviation exception", "Тема NASA", "Тема NASA", [][3]int{{3, 7, 16}}},
		{"both parts correct", "РАЗДЕЛ: Тема", "РАЗДЕЛ: Тема", nil},
		{"prefix violation", "Раздел: Тема", "Раздел: Тема", [][3]int{{3, 7, 19}}},
		{"suffix violation", "РАЗДЕЛ: тема", "РАЗДЕЛ: тема", [][3]int{{3, 7, 19}}},
		{"both parts one finding", "Раздел: тема", "Раздел: тема", [][3]int{{3, 7, 19}}},
		{"later colon is punctuation", "РАЗДЕЛ: Тема: ещё", "РАЗДЕЛ: Тема: ещё", nil},
		{"later colon does not reset", "РАЗДЕЛ: Тема: Ещё", "РАЗДЕЛ: Тема: Ещё", [][3]int{{3, 7, 24}}},
		{"empty prefix", ": Тема", ": Тема", nil},
		{"empty suffix", "РАЗДЕЛ:", "РАЗДЕЛ:", nil},
		{"empty parts", ":", ":", nil},
		{"empty prefix with violation", ": тема", ": тема", [][3]int{{3, 7, 13}}},
		{"empty suffix with violation", "раздел:", "раздел:", [][3]int{{3, 7, 14}}},
		{"no cased letters", "123: 中文 !", "123: 中文 !", nil},
		{"uncased sentence", "123 中文 !", "123 中文 !", nil},
		{"leading punctuation and digits", "123 «Тема»", "123 «Тема»", nil},
		{"punctuation does not reset", "Тема. Урок", "Тема. Урок", [][3]int{{3, 7, 17}}},
		{"only ASCII colon splits", "РАЗДЕЛ： Тема", "РАЗДЕЛ： Тема", [][3]int{{3, 7, 19}}},
		{"tab inside", "Тема\tурока", "Тема\tурока", nil},
		{"tab does not reset", "Тема\tУрока", "Тема\tУрока", [][3]int{{3, 7, 17}}},
		{"titlecase first", "ǅ", "ǅ", [][3]int{{3, 7, 8}}},
		{"titlecase later", "Aǅ", "Aǅ", [][3]int{{3, 7, 9}}},
		{"uppercase digraph", "Ǆ", "Ǆ", nil},
		{"other lowercase first", "ʰ", "ʰ", [][3]int{{3, 7, 8}}},
		{"other lowercase later", "Aʰ", "Aʰ", nil},
		{"lowercase without simple mapping", "ß", "ß", [][3]int{{3, 7, 8}}},
		{"supplementary letters", "𐐀𐐨", "𐐀𐐨", nil},
		{"supplementary lowercase first", "𐐨", "𐐨", [][3]int{{3, 7, 8}}},
		{"combining mark", "A\u0301b", "A\u0301b", nil},
		{"combining mark does not reset", "A\u0301B", "A\u0301B", [][3]int{{3, 7, 10}}},
		{"Unicode spaces and emoji", "А\u00a0😀б", "А\u00a0😀б", nil},
		{"Unicode columns", "А\u00a0😀Б", "А\u00a0😀Б", [][3]int{{3, 7, 11}}},
		// DSL 1.21–1.22: Value уже декодирован; выделяется весь исходный элемент.
		{"decoded title original range", `\{ТЕМА\}`, "{ТЕМА}", [][3]int{{3, 7, 15}}},
		{"decoded valid title", `\{Тема\}`, "{Тема}", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v204Run(t, v204Document(t, v204Block(t, "note", tc.raw, tc.value, "Body.", 3, 2, 1)...), "current", tc.want)
		})
	}
}

func TestV204ExamplesAndIndependentHeadings(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][3]int
	}{
		{"reference", "GRAMMAR: Be", "Writing", nil},
		{"example", "Grammar: be", "WRITING", [][3]int{{3, 7, 18}, {7, 7, 14}}},
		{"repeated titles", "WRITING", "WRITING", [][3]int{{3, 7, 14}, {7, 7, 14}}},
		{"first only", "WRITING", "Writing", [][3]int{{3, 7, 14}}},
		{"last only", "Writing", "WRITING", [][3]int{{7, 7, 14}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v204Pair(t, tc.first, tc.second)
			v204Run(t, result, "first", tc.want)
			v204Run(t, result, "second", tc.want)
			v204Run(t, v204Pair(t, "GRAMMAR: Be", "Writing"), "independent", nil)
		})
	}
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"}, {"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			result := v204Pair(t, "Grammar: be", "WRITING")
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v204Run(t, result, "current", [][3]int{{3, 7, 18}, {7, 7, 14}})
		})
	}
}

func TestV204ContextAndAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, tag, title, body string
		want                   [][3]int
	}{
		{"tag spelling", "NoTe", "WRITING", "Body.", [][3]int{{3, 7, 14}}},
		{"no title", "note", "", "WRITING", nil},
		{"other tag", "hint", "WRITING", "Body.", nil},
		{"body ignored", "note", "Writing", "UNRELATED Mixed CASE", nil},
		{"literal pseudo declaration", "editor", "", "@note WRITING {", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v204Run(t, v204Document(t, v204Block(t, tc.tag, tc.title, tc.title, tc.body, 3, 2, 1)...), "current", tc.want)
		})
	}
	t.Run("inline note", func(t *testing.T) {
		v204Run(t, v204Document(t, v204Line(t, "@note WRITING", "tag", 2, 1, v204Element(t, "tag", "@note", "note", 1), v204Element(t, "content", "WRITING", "WRITING", 7))), "current", nil)
	})
	t.Run("no declarations", func(t *testing.T) { v204Run(t, v204Document(t), "current", nil) })
	t.Run("no available lines", func(t *testing.T) {
		result := v204Document(t)
		result.Lines, result.Document.LineCount, result.Document.Metadata.Title = []model.Line{}, nil, nil
		v204Run(t, result, "current", nil)
	})
	t.Run("source indentation and trailing whitespace", func(t *testing.T) {
		body := v204Block(t, "note", "ТЕМА", "ТЕМА", "Body.", 3, 2, 1)
		body[0] = v204Line(t, " \t@note  ТЕМА { \t", "block-start", 2, 1, v204Element(t, "tag", "@note", "note", 3), v204Element(t, "title", "ТЕМА", "ТЕМА", 10), v204Element(t, "block-open", "{", "{", 15))
		v204Run(t, v204Document(t, body...), "current", [][3]int{{3, 10, 14}})
	})
	t.Run("nested note in complete fragment", func(t *testing.T) {
		body := []model.Line{v204Line(t, "@fragment note-1 {", "block-start", 2, 1, v204Element(t, "tag", "@fragment", "fragment", 1), v204Element(t, "identifier", "note-1", "note-1", 11), v204Element(t, "block-open", "{", "{", 18))}
		body = append(body, v204Block(t, "note", "WRITING", "WRITING", "Body.", 4, 3, 2)...)
		body = append(body, v204Line(t, "}", "block-end", 3, 2, v204Element(t, "block-close", "}", "}", 1)), v204Line(t, "@include note-1", "tag", 2, 1, v204Element(t, "tag", "@include", "include", 1), v204Element(t, "identifier", "note-1", "note-1", 10)))
		v204Run(t, v204Document(t, body...), "current", [][3]int{{4, 7, 14}})
	})
	for _, kind := range []string{"null title", "noncanonical tag value"} {
		t.Run(kind, func(t *testing.T) {
			result := v204Document(t, v204Block(t, "note", "WRITING", "WRITING", "Body.", 3, 2, 1)...)
			if kind == "null title" {
				result.Lines[2].Elements[1].Value = nil
			} else {
				name := "NOTE"
				result.Lines[2].Elements[0].Value = &name
			}
			v204Refusal(t, result)
		})
	}
}

// v204Pair сохраняет оба тела и пустую строку эталона/примера без изменений.
func v204Pair(t *testing.T, first, second string) *model.Result {
	t.Helper()
	body := v204Block(t, "note", first, first, "Use be to describe a state.", 3, 2, 1)
	body = append(body, v204Line(t, "", "blank", 2, 1))
	return v204Document(t, append(body, v204Block(t, "note", second, second, "Start with a plan.", 7, 2, 1)...)...)
}

// v204Block собирает известные элементы объявления и полный блок без разбора DSL.
func v204Block(t *testing.T, tag, raw, value, body string, opening, parent, depth int) []model.Line {
	t.Helper()
	declaration := "@" + tag
	elements := []model.Element{v204Element(t, "tag", declaration, strings.ToLower(tag), 1)}
	if raw != "" {
		elements = append(elements, v204Element(t, "title", raw, value, len(declaration)+2))
		declaration += " " + raw
	}
	elements = append(elements, v204Element(t, "block-open", "{", "{", utf8.RuneCountInString(declaration)+2))
	return []model.Line{v204Line(t, declaration+" {", "block-start", parent, depth, elements...), v204Line(t, body, "content", opening, depth+1, v204Element(t, "content", body, body, 1)), v204Line(t, "}", "block-end", opening, depth+1, v204Element(t, "block-close", "}", "}", 1))}
}

// v204Document дополняет фрагменты обязательной версией и заголовком первого уровня.
func v204Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "УРОК"
	lines := append([]model.Line{v204Line(t, "@dsl-version 1.2", "tag", 0, 0, v204Element(t, "tag", "@dsl-version", "dsl-version", 1), v204Element(t, "version", "1.2", "1.2", 14)), v204Line(t, "# УРОК", "heading", 0, 0, v204Element(t, "heading-level", "#", "1", 1), v204Element(t, "title", title, title, 3))}, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}}, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}

func v204Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v204Element вычисляет только входные координаты; ожидаемые диапазоны заданы явно.
func v204Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v204Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v204Refusal(t *testing.T, result *model.Result) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before := v204Snapshot(t, result)
	if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
		t.Fatal("Prepare must refuse this model")
	}
	if v204Snapshot(t, result) != before {
		t.Fatal("refused input changed")
	}
}

// v204Run проверяет V204 текущего запуска через штатный каталог и конвейер.
// want — явные тройки: строка, включаемое начало, исключающий конец.
func v204Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before := v204Snapshot(t, result)
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatal(err)
	}
	if v204Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatal(err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V204" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V204 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, entry := range got {
		location := model.Location{Start: model.Position{Line: want[i][0], Column: want[i][1]}, End: model.Position{Line: want[i][0], Column: want[i][2]}}
		if entry.ID == "" || entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "element" || entry.Fatal || entry.Message != "Регистр заголовка @note отличается от схемы «Тема» или «РАЗДЕЛ: Тема»." || entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected V204: %+v, want %+v", entry, location)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if !slices.Contains(element.ErrorIDs, entry.ID) {
					continue
				}
				links++
				if element.ElementType != "title" || line.Number != want[i][0] || element.Start != want[i][1] || element.End != want[i][2] {
					t.Fatal("V204 linked to another element")
				}
			}
		}
		if links != 1 {
			t.Fatalf("V204 links = %d, want 1", links)
		}
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v204Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Processing) != len(original.Processing)+1 || len(restored.Lines) != len(original.Lines) || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Исключаем только дополнения конвейера; прежние ссылки и диагностики сохраняются.
	restored.Processing, restored.Diagnostics = restored.Processing[:len(original.Processing)], restored.Diagnostics[:len(original.Diagnostics)]
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		for j := range restored.Lines[i].Elements {
			oldIDs, newIDs := original.Lines[i].Elements[j].ErrorIDs, restored.Lines[i].Elements[j].ErrorIDs
			if len(newIDs) < len(oldIDs) || !slices.Equal(newIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			restored.Lines[i].Elements[j].ErrorIDs = oldIDs
		}
	}
	if v204Snapshot(t, &restored) != before {
		t.Fatal("source model or previous history changed")
	}
}

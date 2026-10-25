package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

type v197Want struct {
	main    [4]int
	related [][4]int
}

func TestV197Registration(t *testing.T) {
	registry := v197TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range definitions {
		if d.Code != "V197" {
			continue
		}
		count++
		if d.Severity != "warning" || d.Scope != "block" || d.Check == nil || d.Message != "Одинаковый материал повторяется во всех ветвях @variants." || d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("definition: %+v", d)
		}
	}
	if count != 1 {
		t.Fatalf("registrations: %d", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.2.0", "1.3"} {
		if ds, err := registry.ForVersion(version); err == nil || len(ds) != 0 {
			t.Fatalf("unsupported version %s accepted", version)
		}
	}
}

func v197TestRun(t *testing.T, result *model.Result, want []v197Want) {
	t.Helper()
	before := v197TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v197TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v197TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	source := "v197-run-" + strconv.Itoa(len(result.Processing))
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, d := range result.Diagnostics {
		if d.Source == source && d.DiagnosticCode == "V197" {
			got = append(got, d)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V197 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		d := got[i]
		location := v197TestLocation(expected.main)
		if d.Location == nil || *d.Location != location || d.SeverityLevel != "warning" || d.DiagnosticScope != "block" || d.Fatal || d.ID == "" || d.Message != "Одинаковый материал повторяется во всех ветвях @variants.\nконкретизация: Проверьте, следует ли вынести этот материал перед блоком @variants." || d.RelatedLocations == nil || len(d.RelatedLocations) != len(expected.related) {
			t.Fatalf("occurrence %d: %+v location=%+v; want %+v", i, d, d.Location, expected)
		}
		for j, span := range expected.related {
			if d.RelatedLocations[j] != v197TestLocation(span) {
				t.Fatalf("related %d: %+v want %+v", j, d.RelatedLocations[j], span)
			}
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == d.ID {
						t.Fatal("warning has element error backlink")
					}
				}
			}
		}
	}
	// Штатная проверка целостности подтверждает вычисление hasErrors только по error.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v197TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v197TestSnapshot(t, &restored) != before {
		t.Fatal("input or preceding history changed")
	}
}

func v197TestLocation(span [4]int) model.Location {
	return model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[2], Column: span[3]}}
}

// v197TestPair помещает явно заданные единицы в две физические ветви.
func v197TestPair(t *testing.T, left, right func(int) []model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v197TestNamed(t, "variant", "name", "A", 3, 2)}
	body = append(body, left(4)...)
	second := len(body) + 4
	body = append(body, v197TestNamed(t, "variant", "name", "B", 3, 2))
	body = append(body, right(second)...)
	return v197TestBlock(t, "variants", body...)
}

func v197TestNamed(t testing.TB, tag, kind, value string, parent, depth int) model.Line {
	t.Helper()
	return v197TestLine(t, "@"+tag+" "+value, "tag", parent, depth, v197TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1), v197TestElement(t, model.ElementType(kind), value, value, utf8.RuneCountInString(tag)+3))
}

func v197TestInline(t *testing.T, tag, raw, value string) func(int) []model.Line {
	t.Helper()
	return func(parent int) []model.Line {
		return []model.Line{v197TestLine(t, "@"+tag+" "+raw, "tag", parent, 3, v197TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1), v197TestElement(t, "content", raw, value, utf8.RuneCountInString(tag)+3))}
	}
}

// v197TestMaterial задаёт блочную форму и непосредственные строки содержимого.
// Типы строк, raw и value передаются явно, синтаксис помощником не распознаётся.
func v197TestMaterial(t *testing.T, tag, title string, rows ...model.Line) func(int) []model.Line {
	t.Helper()
	return func(parent int) []model.Line {
		open := v197TestOpen(t, tag, parent, 3)
		if title != "" {
			raw := "@" + tag + " " + title + " {"
			open = v197TestLine(t, raw, "block-start", parent, 3, v197TestElement(t, "tag", "@"+tag, strings.ToLower(tag), 1), v197TestElement(t, "title", title, title, len(tag)+3), v197TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(raw)))
		}
		body := []model.Line{open}
		for _, row := range rows {
			p := parent + 1
			row.ParentLine = &p
			row.NestingLevel = 4
			body = append(body, row)
		}
		return append(body, v197TestClose(t, parent+1, 4))
	}
}

// v197TestElement сохраняет явные исходное и семантическое значения.
// Вычисление длины используется только для входа; ожидания заданы числами.
func v197TestElement(t testing.TB, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v197TestLine задаёт структуру строки без повторного разбора DSL.
func v197TestLine(t testing.TB, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v197TestOpen задаёт блочное объявление без параметров с явным родителем.
func v197TestOpen(t testing.TB, name string, parent, depth int) model.Line {
	t.Helper()
	return v197TestLine(t, "@"+name+" {", "block-start", parent, depth,
		v197TestElement(t, "tag", "@"+name, strings.ToLower(name), 1), v197TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v197TestClose задаёт закрытие указанного блока.
func v197TestClose(t testing.TB, parent, depth int) model.Line {
	t.Helper()
	return v197TestLine(t, "}", "block-end", parent, depth, v197TestElement(t, "block-close", "}", "}", 1))
}

// v197TestText сохраняет исходное и семантическое содержимое независимо.
func v197TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v197TestLine(t, raw, "content", parent, depth, v197TestElement(t, "content", raw, value, 1))
}

// v197TestBlock окружает заданное тело блоком на строке 3 без смены родителей.
func v197TestBlock(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v197TestOpen(t, name, 2, 1)}, body...)
	return v197TestDocument(t, append(lines, v197TestClose(t, 3, 2))...)
}

// v197TestDocument добавляет обязательные версию и заголовок, а также исходные
// ошибки для явно заданных unparsed. Родители и типы заданы тестом, не парсером.
func v197TestDocument(t testing.TB, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v197TestLine(t, "@dsl-version 1.2", "tag", 0, 0, v197TestElement(t, "tag", "@dsl-version", "dsl-version", 1), v197TestElement(t, "version", "1.2", "1.2", 14)),
		v197TestLine(t, "# Lesson", "heading", 0, 0, v197TestElement(t, "heading-level", "#", "1", 1), v197TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := "parser-error-" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture", SeverityLevel: "error",
				DiagnosticScope: "element", Message: "Ошибка исходной конструкции.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}}})
		}
	}
	return result
}

// v197TestRegistry использует только полный штатный каталог.
func v197TestRegistry(t testing.TB) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v197TestSnapshot сохраняет независимое представление модели для сравнения.
func v197TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestV197ReferenceAndExample(t *testing.T) {
	reference := "@note Use the map.\n@variants {\n@variant A\nFind London.\n@variant B\nFind Bristol.\n}"
	example := "@variants {\n@variant A\n@note Use the map.\nFind London.\n@variant B\n@note Use the map.\nFind Bristol.\n}"
	first := func(parent int) []model.Line {
		return []model.Line{v197TestNamed(t, "note", "content", "Use the map.", parent, 3), v197TestText(t, "Find London.", "Find London.", parent, 3)}
	}
	second := func(parent int) []model.Line {
		return []model.Line{v197TestNamed(t, "note", "content", "Use the map.", parent, 3), v197TestText(t, "Find Bristol.", "Find Bristol.", parent, 3)}
	}
	ex := v197TestPair(t, first, second)
	ref := v197TestDocument(t, v197TestNamed(t, "note", "content", "Use the map.", 2, 1), v197TestOpen(t, "variants", 2, 1), v197TestNamed(t, "variant", "name", "A", 4, 2), v197TestText(t, "Find London.", "Find London.", 5, 3), v197TestNamed(t, "variant", "name", "B", 4, 2), v197TestText(t, "Find Bristol.", "Find Bristol.", 7, 3), v197TestClose(t, 4, 2))
	for _, tc := range []struct {
		name, raw string
		doc       *model.Result
		want      []v197Want
	}{
		{"reference", reference, ref, nil},
		{"example", example, ex, []v197Want{{[4]int{8, 1, 8, 19}, [][4]int{{5, 1, 5, 19}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.doc.Lines {
				raw = append(raw, line.Raw)
			}
			// Добавлены только обязательные версия и заголовок по 1.30.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.raw {
				t.Fatal("normative example changed")
			}
			v197TestRun(t, tc.doc, tc.want)
			v197TestRun(t, tc.doc, tc.want)
		})
	}
}

func TestV197GroupsAndBranches(t *testing.T) {
	note := v197TestInline(t, "note", "Use the map.", "Use the map.")
	t.Run("entirely identical branches remain eligible", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, note, note), []v197Want{{[4]int{7, 1, 7, 19}, [][4]int{{5, 1, 5, 19}}}})
	})
	t.Run("all occurrences in one group", func(t *testing.T) {
		twice := func(p int) []model.Line { return append(note(p), note(p)...) }
		v197TestRun(t, v197TestPair(t, twice, twice), []v197Want{{[4]int{8, 1, 8, 19}, [][4]int{{5, 1, 5, 19}, {6, 1, 6, 19}, {9, 1, 9, 19}}}})
	})
	t.Run("different signatures", func(t *testing.T) {
		units := func(p int) []model.Line { return append(note(p), v197TestNamed(t, "hint", "content", "Look.", p, 3)) }
		v197TestRun(t, v197TestPair(t, units, units), []v197Want{{[4]int{8, 1, 8, 19}, [][4]int{{5, 1, 5, 19}}}, {[4]int{9, 1, 9, 12}, [][4]int{{6, 1, 6, 12}}}})
	})
	for _, third := range []bool{false, true} {
		t.Run("three branches/common="+strconv.FormatBool(third), func(t *testing.T) {
			body := []model.Line{v197TestNamed(t, "variant", "name", "A", 3, 2)}
			body = append(body, note(4)...)
			body = append(body, v197TestNamed(t, "variant", "name", "B", 3, 2))
			body = append(body, note(6)...)
			body = append(body, v197TestNamed(t, "variant", "name", "C", 3, 2))
			if third {
				body = append(body, note(8)...)
			} else {
				body = append(body, v197TestText(t, "Other.", "Other.", 8, 3))
			}
			var want []v197Want
			if third {
				want = []v197Want{{[4]int{7, 1, 7, 19}, [][4]int{{5, 1, 5, 19}, {9, 1, 9, 19}}}}
			}
			v197TestRun(t, v197TestBlock(t, "variants", body...), want)
		})
	}
	t.Run("repeat only in first branch", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, func(p int) []model.Line { return append(note(p), note(p)...) }, v197TestInline(t, "note", "Other.", "Other.")), nil)
	})
	t.Run("zero branches", func(t *testing.T) { v197TestRun(t, v197TestBlock(t, "variants"), nil) })
	t.Run("one branch", func(t *testing.T) {
		v197TestRun(t, v197TestBlock(t, "variants", v197TestNamed(t, "variant", "name", "A", 3, 2), v197TestNamed(t, "note", "content", "Use the map.", 4, 3)), nil)
	})
	t.Run("empty input", func(t *testing.T) {
		doc := v197TestDocument(t)
		doc.Lines = []model.Line{}
		count := 0
		doc.Document.LineCount = &count
		v197TestRun(t, doc, nil)
	})
	t.Run("independent containers", func(t *testing.T) {
		doc := v197TestPair(t, note, v197TestInline(t, "note", "Other.", "Other."))
		more := []model.Line{v197TestOpen(t, "variants", 2, 1), v197TestNamed(t, "variant", "name", "C", 9, 2), v197TestNamed(t, "note", "content", "Use the map.", 10, 3), v197TestNamed(t, "variant", "name", "D", 9, 2), v197TestNamed(t, "note", "content", "Last.", 12, 3), v197TestClose(t, 9, 2)}
		body := append(append([]model.Line{}, doc.Lines[2:]...), more...)
		v197TestRun(t, v197TestDocument(t, body...), nil)
	})
}

func TestV197IndexedGroupsPreserveOccurrences(t *testing.T) {
	body := []model.Line{v197TestNamed(t, "variant", "name", "A", 3, 2)}
	for _, value := range []string{"A.", "B.", "A.", "C."} {
		body = append(body, v197TestNamed(t, "note", "content", value, 4, 3))
	}
	body = append(body, v197TestNamed(t, "variant", "name", "B", 3, 2))
	for _, value := range []string{"B.", "A.", "B.", "C."} {
		body = append(body, v197TestNamed(t, "note", "content", value, 9, 3))
	}
	body = append(body, v197TestNamed(t, "variant", "name", "C", 3, 2))
	for _, value := range []string{"C.", "A.", "B.", "A."} {
		body = append(body, v197TestNamed(t, "note", "content", value, 14, 3))
	}
	body = append(body, v197TestNamed(t, "variant", "name", "D", 3, 2))
	for _, value := range []string{"B.", "A.", "B."} {
		body = append(body, v197TestNamed(t, "note", "content", value, 19, 3))
	}
	// C присутствует в первых трёх ветвях, но отсутствует в последней.
	// Итоговые сообщения упорядочивает Run по primary; related сохраняют исходник.
	v197TestRun(t, v197TestBlock(t, "variants", body...), []v197Want{
		{[4]int{10, 1, 10, 9}, [][4]int{{6, 1, 6, 9}, {12, 1, 12, 9}, {17, 1, 17, 9}, {20, 1, 20, 9}, {22, 1, 22, 9}}},
		{[4]int{11, 1, 11, 9}, [][4]int{{5, 1, 5, 9}, {7, 1, 7, 9}, {16, 1, 16, 9}, {18, 1, 18, 9}, {21, 1, 21, 9}}},
	})
}

func TestV197IndexedIncludesPreserveGroupOrderAndDeduplicateSpans(t *testing.T) {
	doc := v197TestBlock(t, "variants",
		v197TestNamed(t, "variant", "name", "A", 3, 2),
		v197TestNamed(t, "note", "content", "A.", 4, 3),
		v197TestNamed(t, "note", "content", "B.", 4, 3),
		v197TestNamed(t, "note", "content", "A.", 4, 3),
		v197TestNamed(t, "variant", "name", "B", 3, 2),
		v197TestNamed(t, "include", "identifier", "f", 8, 3),
		v197TestNamed(t, "note", "content", "B.", 8, 3),
		v197TestNamed(t, "variant", "name", "C", 3, 2),
		v197TestNamed(t, "include", "identifier", "F", 11, 3),
		v197TestNamed(t, "note", "content", "A.", 11, 3),
		v197TestNamed(t, "include", "identifier", "f", 11, 3))
	doc = v197TestAppendFragment(t, doc, "f", func(p int) []model.Line {
		return []model.Line{
			v197TestNamed(t, "note", "content", "B.", p, 3),
			v197TestNamed(t, "note", "content", "A.", p, 3),
			v197TestNamed(t, "note", "content", "B.", p, 3),
			v197TestNamed(t, "note", "content", "A.", p, 3),
		}
	})
	// Обе primary совпадают. Устойчивый порядок A, B задаёт первая ветвь,
	// хотя в содержимом include единицы идут B, A; повторы одного вызова объединены.
	v197TestRun(t, doc, []v197Want{
		{[4]int{9, 1, 9, 11}, [][4]int{{5, 1, 5, 9}, {7, 1, 7, 9}, {12, 1, 12, 11}, {13, 1, 13, 9}, {14, 1, 14, 11}}},
		{[4]int{9, 1, 9, 11}, [][4]int{{6, 1, 6, 9}, {10, 1, 10, 9}, {12, 1, 12, 11}, {14, 1, 14, 11}}},
	})
}

// v197TestGrowingBranches создаёт три ветви из отдельных корректных @note.
// Во второй порядок обратный; последняя либо совпадает, либо содержит свои тексты.
func v197TestGrowingBranches(t testing.TB, size int, common bool) *model.Result {
	t.Helper()
	body := []model.Line{v197TestOpen(t, "variants", 2, 1)}
	for branch := 0; branch < 3; branch++ {
		parent := len(body) + 3
		body = append(body, v197TestNamed(t, "variant", "name", strconv.Itoa(branch+1), 3, 2))
		for i := 0; i < size; i++ {
			index := i
			if branch == 1 {
				index = size - i - 1
			}
			value := "Material " + strconv.Itoa(index) + "."
			if branch == 2 && !common {
				value = "Other " + strconv.Itoa(index) + "."
			}
			body = append(body, v197TestNamed(t, "note", "content", value, parent, 3))
		}
	}
	return v197TestDocument(t, append(body, v197TestClose(t, 3, 2))...)
}

func TestV197GrowingBranchSets(t *testing.T) {
	for _, size := range []int{64, 256, 1024} {
		t.Run("missing in last/"+strconv.Itoa(size), func(t *testing.T) {
			v197TestRun(t, v197TestGrowingBranches(t, size, false), nil)
		})
	}
	t.Run("all common", func(t *testing.T) {
		const size = 128
		var want []v197Want
		for i := 0; i < size; i++ {
			index := size - i - 1
			// @note Material N. имеет 16 постоянных колонок и цифры N.
			end := 17 + len(strconv.Itoa(index))
			first, second, third := 5+index, 6+size+i, 7+2*size+index
			want = append(want, v197Want{[4]int{second, 1, second, end}, [][4]int{{first, 1, first, end}, {third, 1, third, end}}})
		}
		v197TestRun(t, v197TestGrowingBranches(t, size, true), want)
	})
}

// BenchmarkV197GrowingBranchSets измеряет штатные Prepare/Run с полным каталогом.
// В последней ветви нет общих единиц, поэтому объём вывода не скрывает группировку.
func BenchmarkV197GrowingBranchSets(b *testing.B) {
	for _, size := range []int{64, 256, 1024} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			original := v197TestGrowingBranches(b, size, false)
			registry := v197TestRegistry(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := *original
				checks, err := validator.Prepare(&result, registry)
				if err != nil {
					b.Fatal(err)
				}
				result.Processing = append(append([]model.Processing{}, original.Processing...), model.Processing{ID: "benchmark", Tool: "dsl-validator"})
				if err := validator.Run(&result, checks, "benchmark"); err != nil {
					b.Fatal(err)
				}
				for _, entry := range result.Diagnostics {
					if entry.DiagnosticCode == "V197" {
						b.Fatal("material absent in the last branch matched")
					}
				}
			}
		})
	}
}

func TestV197MaterialKinds(t *testing.T) {
	for _, tc := range []struct {
		tag       string
		inlineEnd int
	}{{"example", 12}, {"wordlist", 13}, {"note", 9}, {"alt", 8}, {"hint", 9}} {
		t.Run(tc.tag+"/inline", func(t *testing.T) {
			content := "X."
			unit := v197TestInline(t, tc.tag, content, content)
			v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{7, 1, 7, tc.inlineEnd}, [][4]int{{5, 1, 5, tc.inlineEnd}}}})
		})
	}
	for _, tag := range []string{"text", "example", "wordlist", "table", "script", "note", "alt", "hint"} {
		t.Run(tag+"/block", func(t *testing.T) {
			row := v197TestText(t, "X.", "X.", 0, 0)
			unit := v197TestMaterial(t, tag, "", row)
			v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{9, 1, 11, 2}, [][4]int{{5, 1, 7, 2}}}})
		})
	}
	for _, tag := range []string{"note", "example", "alt", "hint"} {
		t.Run(tag+"/inline and one paragraph block", func(t *testing.T) {
			unit := v197TestMaterial(t, tag, "", v197TestText(t, "X.", "X.", 0, 0))
			// 3.0, 3.9.6: фигурные скобки сами по себе не добавляют визуальный абзац.
			ends := map[string]int{"note": 9, "example": 12, "alt": 8, "hint": 9}
			v197TestRun(t, v197TestPair(t, v197TestInline(t, tag, "X.", "X."), unit), []v197Want{{[4]int{7, 1, 9, 2}, [][4]int{{5, 1, 5, ends[tag]}}}})
		})
	}
	t.Run("different kinds", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, v197TestInline(t, "note", "X.", "X."), v197TestInline(t, "hint", "X.", "X.")), nil)
	})
	t.Run("no inner partial matches", func(t *testing.T) {
		left := v197TestMaterial(t, "note", "Same", v197TestText(t, "First.", "First.", 0, 0), v197TestText(t, "Left.", "Left.", 0, 0))
		right := v197TestMaterial(t, "note", "Same", v197TestText(t, "First.", "First.", 0, 0), v197TestText(t, "Right.", "Right.", 0, 0))
		v197TestRun(t, v197TestPair(t, left, right), nil)
	})
	t.Run("title is significant", func(t *testing.T) {
		row := v197TestText(t, "X.", "X.", 0, 0)
		v197TestRun(t, v197TestPair(t, v197TestMaterial(t, "note", "Title", row), v197TestMaterial(t, "note", "title", row)), nil)
	})
}

func TestV197PlainText(t *testing.T) {
	plain := func(p int) []model.Line {
		return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestText(t, "Last.", "Last.", p, 3)}
	}
	t.Run("maximal run", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, plain, plain), []v197Want{{[4]int{8, 1, 9, 6}, [][4]int{{5, 1, 6, 6}}}})
	})
	t.Run("blank and permitted comments", func(t *testing.T) {
		spaced := func(p int) []model.Line {
			return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestLine(t, " \t", "blank", p, 3), v197TestNamed(t, "editor", "content", "Remark.", p, 3), v197TestText(t, "Last.", "Last.", p, 3)}
		}
		v197TestRun(t, v197TestPair(t, plain, spaced), []v197Want{{[4]int{8, 1, 11, 6}, [][4]int{{5, 1, 6, 6}}}})
	})
	t.Run("comment block does not split run", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestOpen(t, "editor", p, 3), v197TestText(t, "Ignored.", "Ignored.", p+2, 4), v197TestClose(t, p+2, 4), v197TestText(t, "Last.", "Last.", p, 3)}
		}
		v197TestRun(t, v197TestPair(t, plain, unit), []v197Want{{[4]int{8, 1, 12, 6}, [][4]int{{5, 1, 6, 6}}}})
	})
	t.Run("paragraph boundary", func(t *testing.T) {
		one := func(p int) []model.Line { return []model.Line{v197TestText(t, "First. Last.", "First. Last.", p, 3)} }
		v197TestRun(t, v197TestPair(t, plain, one), nil)
	})
	t.Run("excluded instruction is a significant barrier", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestNamed(t, "instr", "content", "Read.", p, 3), v197TestText(t, "Last.", "Last.", p, 3)}
		}
		v197TestRun(t, v197TestPair(t, plain, unit), nil)
	})
	t.Run("task and step boundaries", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestNamed(t, "task", "identifier", "t"+strconv.Itoa(p), p, 3), v197TestText(t, "First.", "First.", p+1, 4), v197TestNamed(t, "step", "title", "Part", p+1, 4), v197TestText(t, "Last.", "Last.", p+3, 5)}
		}
		v197TestRun(t, v197TestPair(t, plain, unit), nil)
	})
	t.Run("material inside task and step", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestNamed(t, "task", "identifier", "t"+strconv.Itoa(p), p, 3), v197TestNamed(t, "step", "title", "Part", p+1, 4), v197TestNamed(t, "note", "content", "Use the map.", p+2, 5)}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{11, 1, 11, 19}, [][4]int{{7, 1, 7, 19}}}})
	})
}

func TestV197ExactValues(t *testing.T) {
	for _, tc := range []struct{ name, left, right string }{
		{"case", "Text", "text"}, {"punctuation", "Text!", "Text."}, {"spaces", "a  b", "a b"}, {"tab", "a\tb", "a b"}, {"unicode space", "a\u00a0b", "a b"}, {"unicode composition", "é", "e\u0301"}, {"unicode case", "Москва", "москва"}, {"unknown escape", `a\qb`, `aqb`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v197TestRun(t, v197TestPair(t, v197TestInline(t, "note", tc.left, tc.left), v197TestInline(t, "note", tc.right, tc.right)), nil)
		})
	}
	t.Run("tag case", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, v197TestInline(t, "note", "X.", "X."), v197TestInline(t, "NoTe", "X.", "X.")), []v197Want{{[4]int{7, 1, 7, 9}, [][4]int{{5, 1, 5, 9}}}})
	})
	t.Run("ready value not unescaped twice", func(t *testing.T) {
		unit := v197TestInline(t, "note", `a\\\{b\}`, `a\{b}`)
		different := v197TestInline(t, "note", `a\{b\}`, `a{b}`)
		v197TestRun(t, v197TestPair(t, unit, different), nil)
	})
	t.Run("fallback from raw", func(t *testing.T) {
		left := v197TestInline(t, "note", `a\{b\}`, `a{b}`)
		right := func(p int) []model.Line { rows := left(p); rows[0].Elements = rows[0].Elements[:1]; return rows }
		v197TestRun(t, v197TestPair(t, left, right), []v197Want{{[4]int{7, 1, 7, 13}, [][4]int{{5, 1, 5, 13}}}})
	})
	t.Run("trim only external space and tab", func(t *testing.T) {
		left := v197TestInline(t, "note", "X.", "X.")
		right := func(p int) []model.Line {
			return []model.Line{v197TestLine(t, " \t@note X.\t ", "tag", p, 3, v197TestElement(t, "tag", "@note", "note", 3), v197TestElement(t, "content", "X.", "X.", 9))}
		}
		v197TestRun(t, v197TestPair(t, left, right), []v197Want{{[4]int{7, 1, 7, 13}, [][4]int{{5, 1, 5, 9}}}})
	})
	t.Run("unicode ranges", func(t *testing.T) {
		unit := v197TestInline(t, "note", "Карта 🗺.", "Карта 🗺.")
		v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{7, 1, 7, 15}, [][4]int{{5, 1, 5, 15}}}})
	})
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		t.Run("EOL="+strconv.Quote(string(eol)), func(t *testing.T) {
			unit := v197TestInline(t, "note", "X.", "X.")
			doc := v197TestPair(t, unit, unit)
			for i := range doc.Lines {
				doc.Lines[i].LineEnding = eol
			}
			doc.Lines[len(doc.Lines)-1].LineEnding = ""
			v197TestRun(t, doc, []v197Want{{[4]int{7, 1, 7, 9}, [][4]int{{5, 1, 5, 9}}}})
		})
	}
}

func TestV197ExcludedContent(t *testing.T) {
	for _, tag := range []string{"instr", "question", "editor"} {
		t.Run(tag+"/inline", func(t *testing.T) {
			unit := v197TestInline(t, tag, "X.", "X.")
			v197TestRun(t, v197TestPair(t, unit, unit), nil)
		})
	}
	for _, tag := range []string{"instr", "choice", "multichoice", "ordering", "multifill"} {
		t.Run(tag+"/block", func(t *testing.T) {
			unit := v197TestMaterial(t, tag, "", v197TestText(t, "X.", "X.", 0, 0), v197TestText(t, "Y.", "Y.", 0, 0))
			v197TestRun(t, v197TestPair(t, unit, unit), nil)
		})
	}
	t.Run("instruction hint excluded", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestOpen(t, "instr", p, 3), v197TestText(t, "Read.", "Read.", p+1, 4), v197TestNamed(t, "hint", "content", "Look.", p+1, 4), v197TestClose(t, p+1, 4)}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), nil)
	})
	for _, block := range []bool{false, true} {
		t.Run("answer hint/block="+strconv.FormatBool(block), func(t *testing.T) {
			unit := func(p int) []model.Line {
				rows := []model.Line{v197TestNamed(t, "question", "content", "Where?", p, 3)}
				if block {
					rows = append(rows, v197TestOpen(t, "answer", p, 3), v197TestText(t, "Here.", "Here.", p+2, 4), v197TestClose(t, p+2, 4))
				} else {
					rows = append(rows, v197TestNamed(t, "answer", "content", "Here.", p, 3))
				}
				rows = append(rows, v197TestLine(t, "", "blank", p, 3), v197TestNamed(t, "editor", "content", "Comment.", p, 3), v197TestNamed(t, "hint", "content", "Look.", p, 3))
				return rows
			}
			v197TestRun(t, v197TestPair(t, unit, unit), nil)
		})
	}
	t.Run("hint does not cross include boundary", func(t *testing.T) {
		left := func(p int) []model.Line {
			return []model.Line{v197TestNamed(t, "question", "content", "Where?", p, 3), v197TestNamed(t, "answer", "content", "Here.", p, 3), v197TestNamed(t, "include", "identifier", "f", p, 3)}
		}
		right := v197TestInline(t, "hint", "Look.", "Look.")
		doc := v197TestPair(t, left, right)
		doc = v197TestAppendFragment(t, doc, "f", v197TestInline(t, "hint", "Look.", "Look."))
		v197TestRun(t, doc, []v197Want{{[4]int{9, 1, 9, 12}, [][4]int{{7, 1, 7, 11}}}})
	})
}

// v197TestAppendFragment добавляет определение после вызовов. Вложенные элементы
// сохраняют явно заданную структуру; меняется только глубина под общим заголовком.
func v197TestAppendFragment(t *testing.T, doc *model.Result, id string, material func(int) []model.Line) *model.Result {
	t.Helper()
	number := len(doc.Lines) + 1
	raw := "@fragment " + id + " {"
	open := v197TestLine(t, raw, "block-start", 2, 1, v197TestElement(t, "tag", "@fragment", "fragment", 1), v197TestElement(t, "identifier", id, id, 11), v197TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(raw)))
	body := append([]model.Line{}, doc.Lines[2:]...)
	body = append(body, open)
	rows := material(number)
	for i := range rows {
		rows[i].NestingLevel--
	}
	body = append(body, rows...)
	body = append(body, v197TestClose(t, number, 2))
	return v197TestDocument(t, body...)
}

func TestV197Includes(t *testing.T) {
	include := func(id string) func(int) []model.Line {
		return func(p int) []model.Line { return []model.Line{v197TestNamed(t, "include", "identifier", id, p, 3)} }
	}
	note := v197TestInline(t, "note", "Use the map.", "Use the map.")
	want := []v197Want{{[4]int{7, 1, 7, 11}, [][4]int{{5, 1, 5, 11}}}}
	t.Run("same fragment after calls and case insensitive ID", func(t *testing.T) {
		doc := v197TestPair(t, include("f"), include("F"))
		doc = v197TestAppendFragment(t, doc, "f", note)
		v197TestRun(t, doc, want)
	})
	t.Run("different fragment IDs same material", func(t *testing.T) {
		doc := v197TestPair(t, include("f"), include("g"))
		doc = v197TestAppendFragment(t, doc, "f", note)
		doc = v197TestAppendFragment(t, doc, "g", note)
		v197TestRun(t, doc, want)
	})
	t.Run("direct and included", func(t *testing.T) {
		doc := v197TestAppendFragment(t, v197TestPair(t, note, include("f")), "f", note)
		v197TestRun(t, doc, []v197Want{{[4]int{7, 1, 7, 11}, [][4]int{{5, 1, 5, 19}}}})
	})
	t.Run("same call range merged", func(t *testing.T) {
		twice := func(p int) []model.Line { return append(note(p), note(p)...) }
		doc := v197TestAppendFragment(t, v197TestPair(t, include("f"), include("f")), "f", twice)
		v197TestRun(t, doc, want)
	})
	t.Run("different units at same calls", func(t *testing.T) {
		units := func(p int) []model.Line { return append(note(p), v197TestNamed(t, "hint", "content", "Look.", p, 3)) }
		doc := v197TestAppendFragment(t, v197TestPair(t, include("f"), include("f")), "f", units)
		v197TestRun(t, doc, []v197Want{{[4]int{7, 1, 7, 11}, [][4]int{{5, 1, 5, 11}}}, {[4]int{7, 1, 7, 11}, [][4]int{{5, 1, 5, 11}}}})
	})
	t.Run("distinct calls retained", func(t *testing.T) {
		calls := func(p int) []model.Line { return append(include("f")(p), include("f")(p)...) }
		doc := v197TestAppendFragment(t, v197TestPair(t, calls, calls), "f", note)
		v197TestRun(t, doc, []v197Want{{[4]int{8, 1, 8, 11}, [][4]int{{5, 1, 5, 11}, {6, 1, 6, 11}, {9, 1, 9, 11}}}})
	})
	t.Run("undefined", func(t *testing.T) { v197TestRun(t, v197TestPair(t, include("f"), include("f")), nil) })
	t.Run("duplicate definition", func(t *testing.T) {
		doc := v197TestPair(t, include("f"), include("f"))
		doc = v197TestAppendFragment(t, doc, "f", note)
		doc = v197TestAppendFragment(t, doc, "F", note)
		v197TestRun(t, doc, nil)
	})
	t.Run("forbidden include inside instruction", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestOpen(t, "instr", p, 3), v197TestNamed(t, "include", "identifier", "f", p+1, 4), v197TestClose(t, p+1, 4)}
		}
		doc := v197TestAppendFragment(t, v197TestPair(t, unit, unit), "f", note)
		v197TestRun(t, doc, nil)
	})
	t.Run("forbidden fragment content", func(t *testing.T) {
		doc := v197TestAppendFragment(t, v197TestPair(t, include("f"), include("f")), "f", v197TestInline(t, "question", "Where?", "Where?"))
		v197TestRun(t, doc, nil)
	})
	t.Run("definitions are not displays", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{
				v197TestLine(t, "@fragment f"+strconv.Itoa(p)+" {", "block-start", p, 3, v197TestElement(t, "tag", "@fragment", "fragment", 1), v197TestElement(t, "identifier", "f"+strconv.Itoa(p), "f"+strconv.Itoa(p), 11), v197TestElement(t, "block-open", "{", "{", 14)),
				v197TestNamed(t, "note", "content", "Use the map.", p+1, 4), v197TestClose(t, p+1, 4)}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), nil)
	})
}

func TestV197StructuredContent(t *testing.T) {
	row := func(raw, value string) model.Line { return v197TestText(t, raw, value, 0, 0) }
	separator := v197TestLine(t, "---", "separator", 0, 0, v197TestElement(t, "content", "---", "---", 1))
	for _, tc := range []struct {
		name, tag, leftRaw, leftValue, rightRaw, rightValue string
		same                                                bool
	}{
		{"HTML tag case", "text", "<STRONG>X</STRONG>", "<STRONG>X</STRONG>", "<strong>X</strong>", "<strong>X</strong>", true},
		{"HTML literal versus formatting", "text", `\<strong\>X\</strong\>`, "<strong>X</strong>", "<strong>X</strong>", "<strong>X</strong>", false},
		{"HTML text case", "text", "<em>X</em>", "<em>X</em>", "<em>x</em>", "<em>x</em>", false},
		{"HTML nesting", "text", "<strong><em>X</em></strong>", "<strong><em>X</em></strong>", "<em><strong>X</strong></em>", "<em><strong>X</strong></em>", false},
		{"HTML break case", "text", "<BR>", "<BR>", "<br>", "<br>", true},
		{"HTML literal break", "text", `\<br\>`, "<br>", "<br>", "<br>", false},
		{"table padding", "table", "A | B", "A | B", "A|B", "A|B", true},
		{"table empty edge cells", "table", "|A|B|", "|A|B|", "A|B", "A|B", false},
		{"table literal pipe", "table", `A\|B`, "A|B", "A|B", "A|B", false},
		{"table split versus columns", "table", "<A|B>", "<A|B>", "A|B", "A|B", false},
		{"table split padding", "table", "<A | B> | C", "<A | B> | C", "<A|B>|C", "<A|B>|C", true},
		{"table escaped edge angles", "table", `\<A\>`, `<A>`, `\<A\>`, `<A>`, true},
		{"table literal internal angles", "table", `a\<b`, `a\<b`, `a<b`, `a<b`, false},
		{"table internal tabs", "table", "a\tb | c", "a\tb | c", "a b | c", "a b | c", false},
		{"table punctuation", "table", "a!|b", "a!|b", "a|b", "a|b", false},
		{"example literal separator", "example", `\---`, "---", `\---`, "---", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left := v197TestMaterial(t, tc.tag, "", row(tc.leftRaw, tc.leftValue))
			right := v197TestMaterial(t, tc.tag, "", row(tc.rightRaw, tc.rightValue))
			var want []v197Want
			if tc.same {
				want = []v197Want{{[4]int{9, 1, 11, 2}, [][4]int{{5, 1, 7, 2}}}}
			}
			v197TestRun(t, v197TestPair(t, left, right), want)
		})
	}
	for _, tag := range []string{"example", "table"} {
		t.Run(tag+"/separator boundary", func(t *testing.T) {
			left := v197TestMaterial(t, tag, "", row("A", "A"), separator, row("B", "B"))
			right := v197TestMaterial(t, tag, "", row("A", "A"), row(`\---`, "---"), row("B", "B"))
			v197TestRun(t, v197TestPair(t, left, right), nil)
		})
	}
	t.Run("equal complete table", func(t *testing.T) {
		unit := v197TestMaterial(t, "table", "Forms", row("Verb | Form", "Verb | Form"), separator, row("be | <am | is>", "be | <am | is>"))
		v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{11, 1, 15, 2}, [][4]int{{5, 1, 9, 2}}}})
	})
	t.Run("equal complete example", func(t *testing.T) {
		unit := v197TestMaterial(t, "example", "Fix", row("He live.", "He live."), separator, row("He lives.", "He lives."))
		v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{11, 1, 15, 2}, [][4]int{{5, 1, 9, 2}}}})
	})
	t.Run("wordlist escaped delimiter", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, v197TestInline(t, "wordlist", `a\;b; c.`, "a;b; c."), v197TestInline(t, "wordlist", "a;b; c.", "a;b; c.")), nil)
	})
	t.Run("wordlist internal spaces preserved", func(t *testing.T) {
		v197TestRun(t, v197TestPair(t, v197TestInline(t, "wordlist", "a;  b.", "a;  b."), v197TestInline(t, "wordlist", "a; b.", "a; b.")), nil)
	})
	t.Run("significant visual break", func(t *testing.T) {
		left := v197TestMaterial(t, "text", "", row("A", "A"), row("<br>", "<br>"), row("B", "B"))
		right := v197TestMaterial(t, "text", "", row("A", "A"), v197TestLine(t, "", "blank", 0, 0), row("B", "B"))
		v197TestRun(t, v197TestPair(t, left, right), nil)
	})
}

func v197TestMedia(t *testing.T, kind, raw, value string, ready bool) func(int) []model.Line {
	t.Helper()
	return func(p int) []model.Line {
		elements := []model.Element{v197TestElement(t, "tag", "@media", "media", 1), v197TestElement(t, "media-type", kind, strings.ToLower(kind), 8)}
		if ready {
			elements = append(elements, v197TestElement(t, "source", raw, value, 9+utf8.RuneCountInString(kind)))
		}
		return []model.Line{v197TestLine(t, "@media "+kind+" "+raw, "tag", p, 3, elements...)}
	}
}

func TestV197Media(t *testing.T) {
	for _, tc := range []struct {
		name, leftKind, leftRaw, leftValue, rightKind, rightRaw, rightValue string
		same                                                                bool
		leftEnd, rightEnd                                                   int
	}{
		{"same path", "image", "map.png", "map.png", "image", "map.png", "map.png", true, 21, 21},
		{"type case and optional quotes", "audio", `"a.mp3"`, "a.mp3", "AUDIO", "a.mp3", "a.mp3", true, 21, 19},
		{"different type", "audio", "a.mp3", "a.mp3", "video", "a.mp3", "a.mp3", false, 0, 0},
		{"path case", "image", "Map.png", "Map.png", "image", "map.png", "map.png", false, 0, 0},
		{"number versus quoted path", "audio", "12", "12", "audio", `"12"`, "12", false, 0, 0},
		{"number leading zeros", "audio", "012", "012", "audio", "12", "12", false, 0, 0},
		{"quoted escape", "image", `"a\\b.png"`, `a\b.png`, "image", `"a\\b.png"`, `a\b.png`, true, 24, 24},
		{"quoted internal space", "audio", `"a  b.mp3"`, "a  b.mp3", "audio", `"a b.mp3"`, "a b.mp3", false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left := v197TestMedia(t, tc.leftKind, tc.leftRaw, tc.leftValue, true)
			right := v197TestMedia(t, tc.rightKind, tc.rightRaw, tc.rightValue, true)
			var want []v197Want
			if tc.same {
				want = []v197Want{{[4]int{7, 1, 7, tc.rightEnd}, [][4]int{{5, 1, 5, tc.leftEnd}}}}
			}
			v197TestRun(t, v197TestPair(t, left, right), want)
		})
	}
	t.Run("source fallback", func(t *testing.T) {
		left := v197TestMedia(t, "image", `"a\\b.png"`, `a\b.png`, true)
		right := v197TestMedia(t, "image", `"a\\b.png"`, `a\b.png`, false)
		v197TestRun(t, v197TestPair(t, left, right), []v197Want{{[4]int{7, 1, 7, 24}, [][4]int{{5, 1, 5, 24}}}})
	})
}

func TestV197AmbiguityAndBoundaries(t *testing.T) {
	t.Run("material before first branch ignored", func(t *testing.T) {
		doc := v197TestBlock(t, "variants", v197TestNamed(t, "note", "content", "X.", 3, 2), v197TestNamed(t, "variant", "name", "A", 3, 2), v197TestNamed(t, "note", "content", "X.", 5, 3), v197TestNamed(t, "variant", "name", "B", 3, 2), v197TestNamed(t, "note", "content", "Y.", 7, 3))
		v197TestRun(t, doc, nil)
	})
	t.Run("literal pseudo tags remain text", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestText(t, `\@note X.`, `@note X.`, p, 3), v197TestText(t, `\# Title`, `# Title`, p, 3)}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), []v197Want{{[4]int{8, 1, 9, 9}, [][4]int{{5, 1, 6, 9}}}})
	})
	t.Run("missing tag elements with established line types", func(t *testing.T) {
		unit := v197TestInline(t, "note", "X.", "X.")
		doc := v197TestPair(t, unit, unit)
		for _, index := range []int{2, 3, 4, 5, 6} {
			doc.Lines[index].Elements = doc.Lines[index].Elements[1:]
		}
		v197TestRun(t, doc, []v197Want{{[4]int{7, 1, 7, 9}, [][4]int{{5, 1, 5, 9}}}})
	})
	t.Run("ambiguous branch declaration", func(t *testing.T) {
		unit := v197TestInline(t, "note", "X.", "X.")
		doc := v197TestPair(t, unit, unit)
		bad := v197TestLine(t, "@variant", "invalid", 3, 2, model.Element{ElementType: "unparsed", Raw: "@variant", Start: 1, End: 9, ErrorIDs: []string{}})
		body := append([]model.Line{}, doc.Lines[2:]...)
		body[3] = bad
		v197TestRun(t, v197TestDocument(t, body...), nil)
	})
	t.Run("unparsed material not compared", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestLine(t, "@unknown x", "invalid", p, 3, model.Element{ElementType: "unparsed", Raw: "@unknown x", Start: 1, End: 11, ErrorIDs: []string{}})}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), nil)
	})
	t.Run("warning does not set error flags", func(t *testing.T) {
		unit := v197TestInline(t, "note", "Use the map.", "Use the map.")
		doc := v197TestPair(t, unit, unit)
		// Для проверки флагов документ должен удовлетворять также V037:
		// обязательный document-id добавлен до заголовка, вне примеров таблицы.
		id := "lesson-v197"
		metadata := v197TestNamed(t, "document-id", "identifier", id, 0, 0)
		lines := append([]model.Line{doc.Lines[0], metadata}, doc.Lines[1:]...)
		for i := range lines {
			lines[i].Number = i + 1
			if i > 1 && lines[i].ParentLine != nil {
				parent := *lines[i].ParentLine + 1
				lines[i].ParentLine = &parent
			}
		}
		doc.Lines = lines
		count := len(lines)
		doc.Document.LineCount = &count
		doc.Document.Metadata.DocumentID = &id
		v197TestRun(t, doc, []v197Want{{[4]int{8, 1, 8, 19}, [][4]int{{6, 1, 6, 19}}}})
		for _, d := range doc.Diagnostics {
			if d.SeverityLevel == "error" {
				t.Fatalf("unexpected fixture error: %+v", d)
			}
		}
		if doc.Document.HasErrors {
			t.Fatal("warning set document error flag")
		}
		for _, line := range doc.Lines {
			if line.HasErrors {
				t.Fatal("warning set line error flag")
			}
		}
	})
}

func TestV197ExcludedCompleteContexts(t *testing.T) {
	for _, block := range []bool{false, true} {
		t.Run("answer/block="+strconv.FormatBool(block), func(t *testing.T) {
			unit := func(p int) []model.Line {
				lines := []model.Line{v197TestNamed(t, "question", "content", "Where?", p, 3)}
				if !block {
					return append(lines, v197TestNamed(t, "answer", "content", "Here.", p, 3))
				}
				return append(lines, v197TestOpen(t, "answer", p, 3), v197TestText(t, "Here.", "Here.", p+2, 4), v197TestText(t, "There.", "There.", p+2, 4), v197TestClose(t, p+2, 4))
			}
			v197TestRun(t, v197TestPair(t, unit, unit), nil)
		})
	}
	t.Run("key inside task", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestNamed(t, "task", "identifier", "t"+strconv.Itoa(p), p, 3), v197TestNamed(t, "question", "content", "Where?", p+1, 4), v197TestOpen(t, "key", p+1, 4), v197TestText(t, "Here.", "Here.", p+3, 5), v197TestClose(t, p+3, 5)}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), nil)
	})
	t.Run("matching with two lists", func(t *testing.T) {
		unit := func(p int) []model.Line {
			return []model.Line{v197TestOpen(t, "matching", p, 3), v197TestText(t, "Sun?", "Sun?", p+1, 4), v197TestText(t, "Moon?", "Moon?", p+1, 4), v197TestLine(t, "---", "separator", p+1, 4, v197TestElement(t, "content", "---", "---", 1)), v197TestText(t, "Star", "Star", p+1, 4), v197TestText(t, "Satellite", "Satellite", p+1, 4), v197TestClose(t, p+1, 4)}
		}
		v197TestRun(t, v197TestPair(t, unit, unit), nil)
	})
}

func TestV197EscapeFallbackContexts(t *testing.T) {
	for _, tc := range []struct{ name, tag, raw, value string }{
		{"plain braces", "plain", `x\{y\}`, "x{y}"},
		{"plain backslash", "plain", `x\\y`, `x\y`},
		{"short literal underscores", "plain", `x\__y`, `x__y`},
		{"long literal underscores", "note", `x\______y`, `x______y`},
		{"single underscore retains slash", "note", `x\_y`, `x\_y`},
		{"unknown escape", "note", `x\qy`, `x\qy`},
		{"escaped leading tag", "plain", `\@note X.`, `@note X.`},
		{"escaped leading heading", "plain", `\# X.`, `# X.`},
		{"inline at is not a tag", "note", `\@x`, `\@x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left := func(p int) []model.Line {
				if tc.tag == "plain" {
					return []model.Line{v197TestText(t, tc.raw, tc.value, p, 3)}
				}
				return v197TestInline(t, tc.tag, tc.raw, tc.value)(p)
			}
			right := func(p int) []model.Line {
				rows := left(p)
				if tc.tag == "plain" {
					rows[0].Elements = []model.Element{}
				} else {
					rows[0].Elements = rows[0].Elements[:1]
				}
				return rows
			}
			// Явные исходные длины для независимых ожидаемых диапазонов.
			ends := map[string]int{"plain braces": 7, "plain backslash": 5, "short literal underscores": 6, "long literal underscores": 16, "single underscore retains slash": 11, "unknown escape": 11, "escaped leading tag": 10, "escaped leading heading": 6, "inline at is not a tag": 10}
			end := ends[tc.name]
			v197TestRun(t, v197TestPair(t, left, right), []v197Want{{[4]int{7, 1, 7, end}, [][4]int{{5, 1, 5, end}}}})
		})
	}
}

func TestV197InsertionBoundaries(t *testing.T) {
	t.Run("fragment paragraphs mapped to call", func(t *testing.T) {
		plain := func(p int) []model.Line {
			return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestText(t, "Last.", "Last.", p, 3)}
		}
		include := func(p int) []model.Line { return []model.Line{v197TestNamed(t, "include", "identifier", "f", p, 3)} }
		doc := v197TestAppendFragment(t, v197TestPair(t, plain, include), "f", plain)
		v197TestRun(t, doc, []v197Want{{[4]int{8, 1, 8, 11}, [][4]int{{5, 1, 6, 6}}}})
	})
	t.Run("fragment is not textual substitution", func(t *testing.T) {
		left := func(p int) []model.Line {
			return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestText(t, "Last.", "Last.", p, 3)}
		}
		right := func(p int) []model.Line {
			return []model.Line{v197TestText(t, "First.", "First.", p, 3), v197TestNamed(t, "include", "identifier", "f", p, 3)}
		}
		inserted := func(p int) []model.Line { return []model.Line{v197TestText(t, "Last.", "Last.", p, 3)} }
		doc := v197TestAppendFragment(t, v197TestPair(t, left, right), "f", inserted)
		v197TestRun(t, doc, nil)
	})
	t.Run("include has additional arguments", func(t *testing.T) {
		call := func(p int) []model.Line {
			return []model.Line{v197TestLine(t, "@include f extra", "tag", p, 3, v197TestElement(t, "tag", "@include", "include", 1), v197TestElement(t, "identifier", "f", "f", 10), model.Element{ElementType: "unparsed", Raw: "extra", Start: 12, End: 17, ErrorIDs: []string{}})}
		}
		doc := v197TestAppendFragment(t, v197TestPair(t, call, call), "f", v197TestInline(t, "note", "X.", "X."))
		v197TestRun(t, doc, nil)
	})
	t.Run("two empty branches", func(t *testing.T) {
		empty := func(int) []model.Line { return nil }
		v197TestRun(t, v197TestPair(t, empty, empty), nil)
	})
	t.Run("different order of common units", func(t *testing.T) {
		left := func(p int) []model.Line {
			return []model.Line{v197TestNamed(t, "note", "content", "X.", p, 3), v197TestNamed(t, "hint", "content", "Y.", p, 3)}
		}
		right := func(p int) []model.Line {
			return []model.Line{v197TestNamed(t, "hint", "content", "Y.", p, 3), v197TestNamed(t, "note", "content", "X.", p, 3)}
		}
		v197TestRun(t, v197TestPair(t, left, right), []v197Want{{[4]int{8, 1, 8, 9}, [][4]int{{6, 1, 6, 9}}}, {[4]int{9, 1, 9, 9}, [][4]int{{5, 1, 5, 9}}}})
	})
}

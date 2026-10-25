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

func TestV151Registration(t *testing.T) {
	registry := v151TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V151" {
			continue
		}
		count++
		if definition.Message != "Внутри ответа пропуска есть неэкранированная открывающая скобка." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected V151 definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V151 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || len(got) != 0 {
				t.Fatalf("unsupported version accepted: %v, %v", got, err)
			}
			result := v151TestDocument(t)
			result.Document.DSLVersion = &version
			before := v151TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("Prepare accepted version: %v, %v", checks, err)
			}
			if before != v151TestSnapshot(t, result) {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV151ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value, fragment string
		invalid                    bool
		want                       [][3]int
		v005Count                  int
	}{
		{"reference", `Enter _____{a\{b}.`, `Enter _____{a{b}.`, "@multifill {\nEnter _____{a\\{b}.\n}", false, nil, 0},
		{"example", "Enter _____{a{b}.", "", "@multifill {\nEnter _____{a{b}.\n}", true, [][3]int{{5, 14, 15}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v151TestMultifill(t, v151TestRow(t, tc.raw, tc.value, tc.invalid, 4, 3))
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только версия, заголовок и контекст задания.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Unit 1\n@task 1\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v151TestRun(t, result, "current", tc.want)
			// Уточнение A сохраняет общую стековую парность V005 без изменений.
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source != "current" || entry.DiagnosticCode != "V005" {
					continue
				}
				count++
				want := model.Location{Start: model.Position{Line: 4, Column: 12}, End: model.Position{Line: 4, Column: 13}}
				if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
					entry.Message != "Нарушено оформление фигурных скобок.\nконкретизация: Открывающая скобка не имеет закрывающей" {
					t.Fatalf("unexpected V005: %+v, location %+v", entry, entry.Location)
				}
			}
			if count != tc.v005Count {
				t.Fatalf("V005 count = %d, want %d", count, tc.v005Count)
			}
		})
	}
}

func TestV151Answers(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		invalid          bool
		want             [][3]int
	}{
		{"ordinary placeholder", "_____", "_____", false, nil},
		{"one answer", "_____{A}", "_____{A}", false, nil},
		{"two valid alternatives", "_____{A}{B}", "_____{A}{B}", false, nil},
		{"four valid alternatives", "_____{A}{B}{C}{D}", "_____{A}{B}{C}{D}", false, nil},
		{"second alternative", "_____{A}{B{C}", "", true, [][3]int{{5, 11, 12}}},
		{"third alternative", "_____{A}{B}{C{D}", "", true, [][3]int{{5, 14, 15}}},
		{"two openings", "_____{a{{b}", "", true, [][3]int{{5, 8, 9}, {5, 9, 10}}},
		{"first closing ends group", "_____{a{b}{c{d}", "", true, [][3]int{{5, 8, 9}, {5, 13, 14}}},
		{"outside opening ignored", "_____{a{b} tail {", "", true, [][3]int{{5, 8, 9}}},
		{"independent placeholders", "_____{a{b} _____{c{d}", "", true, [][3]int{{5, 8, 9}, {5, 19, 20}}},
		{"placeholder shape inside answer", "_____{x_____{y{z}", "", true, [][3]int{{5, 13, 14}, {5, 15, 16}}},
		{"placeholder shape inside alternative", "_____{A}{_____{B}", "", true, [][3]int{{5, 15, 16}}},
		{"unclosed with opening", "_____{a{", "", true, [][3]int{{5, 8, 9}}},
		{"unclosed without opening", "_____{a", "", true, nil},
		{"unclosed alternative", "_____{A}{B{", "", true, [][3]int{{5, 11, 12}}},
		{"empty alternatives", "_____{}{}", "", true, nil},
		{"empty before malformed alternative", "_____{}{a{b}", "", true, [][3]int{{5, 10, 11}}},
		{"space breaks alternatives", "_____{A} {B{C}", "", true, nil},
		{"tab breaks alternatives", "_____{A}\t{B{C}", "", true, nil},
		{"mixed ordinary and special", "_____ _____{a{b}", "", true, [][3]int{{5, 14, 15}}},
		{"first and last symbols of answer", "_____{{a{}", "", true, [][3]int{{5, 7, 8}, {5, 9, 10}}},
		{"indent and Unicode", "  Я😀 _____{a{b}", "", true, [][3]int{{5, 13, 14}}},
		{"tab and combining character", "\tе\u0301_____{a{b}", "", true, [][3]int{{5, 11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v151TestRun(t, v151TestMultifill(t, v151TestRow(t, tc.raw, tc.value, tc.invalid, 4, 3)), "current", tc.want)
		})
	}
}

func TestV151EscapingAndActivation(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		invalid          bool
		want             [][3]int
	}{
		{"escaped opening", `_____{a\{b}`, `_____{a{b}`, false, nil},
		{"two slashes before opening", `_____{a\\{b}`, "", true, [][3]int{{5, 10, 11}}},
		{"three slashes before opening", `_____{a\\\{b}`, `_____{a\{b}`, false, nil},
		{"four slashes before opening", `_____{a\\\\{b}`, "", true, [][3]int{{5, 12, 13}}},
		{"escaped closing", `_____{a\}b{c}`, "", true, [][3]int{{5, 11, 12}}},
		{"paired slashes then alternative", `_____{a\\}{b{c}`, "", true, [][3]int{{5, 13, 14}}},
		{"ordinary backslash", `_____{a\q{b}`, "", true, [][3]int{{5, 10, 11}}},
		{"quotes do not hide opening", `_____{"{"}`, "", true, [][3]int{{5, 8, 9}}},
		{"quotes do not hide closing", `_____{"}" {`, "", true, nil},
		{"literal full notation", `\_____\{a\{b\}`, `_____{a{b}`, false, nil},
		{"literal underscores", `\_____{a{b}`, "", true, nil},
		{"two slashes activate underscores", `\\_____{a{b}`, "", true, [][3]int{{5, 10, 11}}},
		{"three slashes hide underscores", `\\\_____{a{b}`, "", true, nil},
		{"literal longer run", `\_______{a{b}`, "", true, nil},
		{"one underscore", "_{a{b}", "", true, nil},
		{"four underscores", "____{a{b}", "", true, nil},
		{"six underscores", "______{a{b}", "", true, nil},
		{"space before brace", "_____ {a{b}", "", true, nil},
		{"tab before brace", "_____\t{a{b}", "", true, nil},
		{"Unicode space before brace", "_____\u00a0{a{b}", "", true, nil},
		{"escaped first brace", `_____\{a{b}`, "", true, nil},
		{"escaped alternative opening", `_____{A}\{B{C}`, "", true, nil},
		{"literal group then real placeholder", `\_____{A} _____{b{c}`, "", true, [][3]int{{5, 18, 19}}},
		{"trailing backslash", `_____{a{\`, "", true, [][3]int{{5, 8, 9}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v151TestRun(t, v151TestMultifill(t, v151TestRow(t, tc.raw, tc.value, tc.invalid, 4, 3)), "current", tc.want)
		})
	}
}

func TestV151SourceRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
		want [][3]int
	}{
		{"decoded literal content", v151TestRow(t, `\_____\{a\{b\}`, "_____{a{b}", false, 4, 3), nil},
		{"decoded escaped answer element", v151TestLine(t, `_____{a\{b}`, "content", 4, 3,
			v151TestElement(t, "placeholder", `_____{a\{b}`, "a{b", 1)), nil},
		{"partial content and unparsed", v151TestLine(t, "Enter _____{a{b}.", "content", 4, 3,
			v151TestElement(t, "content", "Enter ", "Enter ", 1), v151TestElement(t, "unparsed", "_____{a{b}", "", 7),
			v151TestElement(t, "content", ".", ".", 17)), [][3]int{{5, 14, 15}}},
		{"invalid line keeps raw", v151TestRow(t, "Enter _____{a{b}.", "", true, 4, 3), [][3]int{{5, 14, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v151TestRun(t, v151TestMultifill(t, tc.row), "current", tc.want) })
	}
}

func TestV151Contexts(t *testing.T) {
	for _, name := range []string{"note", "editor", "key", "answer"} {
		t.Run("other block/"+name, func(t *testing.T) {
			body := []model.Line{v151TestOpen(t, name, 3, 2), v151TestRow(t, "_____{a{b}", "_____{a{b}", name != "editor", 4, 3), v151TestClose(t, 4, 3)}
			v151TestRun(t, v151TestDocument(t, body...), "current", nil)
		})
	}
	t.Run("ordinary task text", func(t *testing.T) {
		v151TestRun(t, v151TestDocument(t, v151TestRow(t, "Enter _____{a{b}.", "", true, 3, 2)), "current", nil)
	})
	t.Run("inline multifill", func(t *testing.T) {
		line := v151TestLine(t, "@multifill _____{a{b}", "tag", 3, 2, v151TestElement(t, "tag", "@multifill", "multifill", 1),
			v151TestElement(t, "unparsed", "_____{a{b}", "", 12))
		v151TestRun(t, v151TestDocument(t, line, v151TestRow(t, "_____{c{d}", "", true, 3, 2)), "current", nil)
	})
	t.Run("block instruction", func(t *testing.T) {
		line := v151TestLine(t, "@multifill _____{a{b} {", "block-start", 3, 2, v151TestElement(t, "tag", "@multifill", "multifill", 1),
			v151TestElement(t, "unparsed", "_____{a{b}", "", 12), v151TestElement(t, "block-open", "{", "{", 23))
		v151TestRun(t, v151TestDocument(t, line, v151TestRow(t, "Text", "Text", false, 4, 3), v151TestClose(t, 4, 3)), "current", nil)
	})
	t.Run("literal multifill in editor", func(t *testing.T) {
		v151TestRun(t, v151TestDocument(t, v151TestOpen(t, "editor", 3, 2),
			v151TestRow(t, "@multifill {", "@multifill {", false, 4, 3),
			v151TestRow(t, "Enter _____{a{b}.", "Enter _____{a{b}.", false, 4, 3), v151TestClose(t, 4, 3)), "current", nil)
	})
	for _, tc := range []struct {
		raw  string
		want [][3]int
	}{
		{"@note _____{a{b}", [][3]int{{5, 14, 15}}}, {"@editor _____{a{b}", [][3]int{{5, 16, 17}}},
	} {
		t.Run("literal tag in multifill/"+tc.raw, func(t *testing.T) {
			// Имя псевдотега не меняет литеральный контекст строки multifill.
			v151TestRun(t, v151TestMultifill(t, v151TestRow(t, tc.raw, "", true, 4, 3)), "current", tc.want)
		})
	}
	t.Run("case and indentation", func(t *testing.T) {
		opening := v151TestLine(t, "\t @MuLtIfIlL {", "block-start", 3, 2,
			v151TestElement(t, "tag", "@MuLtIfIlL", "multifill", 3), v151TestElement(t, "block-open", "{", "{", 14))
		v151TestRun(t, v151TestDocument(t, opening, v151TestRow(t, "_____{a{b}", "", true, 4, 3), v151TestClose(t, 4, 3)), "current", [][3]int{{5, 8, 9}})
	})
	t.Run("deep branch task stage", func(t *testing.T) {
		v151TestRun(t, v151TestDocument(t, v151TestOpen(t, "variants", 2, 1), v151TestTag(t, "variant", "name", "A", 4, 2),
			v151TestTag(t, "task", "identifier", "a-1", 5, 3), v151TestTag(t, "step", "title", "Fill", 6, 4), v151TestOpen(t, "multifill", 7, 5),
			v151TestRow(t, "_____{a{b}", "", true, 8, 6), v151TestClose(t, 8, 6), v151TestTag(t, "variant", "name", "B", 4, 2),
			v151TestRow(t, "Other", "Other", false, 11, 3), v151TestClose(t, 4, 2)), "current", [][3]int{{9, 8, 9}})
	})
}

func TestV151LineAndBlockBoundaries(t *testing.T) {
	t.Run("reset at each line", func(t *testing.T) {
		v151TestRun(t, v151TestMultifill(t, v151TestRow(t, "_____{a", "", true, 4, 3), v151TestRow(t, "{b", "", true, 4, 3),
			v151TestRow(t, "_____{c{d}", "", true, 4, 3)), "current", [][3]int{{7, 8, 9}})
	})
	t.Run("alternatives cannot cross lines", func(t *testing.T) {
		v151TestRun(t, v151TestMultifill(t, v151TestRow(t, "_____{A}", "_____{A}", false, 4, 3),
			v151TestRow(t, "{B{C}", "", true, 4, 3)), "current", nil)
	})
	t.Run("backslash cannot escape next line", func(t *testing.T) {
		v151TestRun(t, v151TestMultifill(t, v151TestRow(t, `Text \`, `Text \`, false, 4, 3),
			v151TestRow(t, "_____{a{b}", "", true, 4, 3)), "current", [][3]int{{6, 8, 9}})
	})
	t.Run("only own block content", func(t *testing.T) {
		v151TestRun(t, v151TestDocument(t, v151TestRow(t, "_____{a{b}", "", true, 3, 2), v151TestOpen(t, "multifill", 3, 2),
			v151TestRow(t, "_____{A}{B}", "_____{A}{B}", false, 5, 3), v151TestClose(t, 5, 3),
			v151TestRow(t, "_____{c{d}", "", true, 3, 2), v151TestOpen(t, "multifill", 3, 2),
			v151TestRow(t, "_____{e{f}", "", true, 9, 3), v151TestClose(t, 9, 3)), "current", [][3]int{{10, 8, 9}})
	})
	t.Run("empty input", func(t *testing.T) {
		result := v151TestDocument(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		v151TestRun(t, result, "current", nil)
	})
	t.Run("blank body", func(t *testing.T) {
		v151TestRun(t, v151TestMultifill(t, v151TestLine(t, " \t", "blank", 4, 3)), "current", nil)
	})
	for _, ending := range []model.LineEnding{"\n", "\r\n"} {
		t.Run("EOL="+strconv.Quote(string(ending)), func(t *testing.T) {
			result := v151TestMultifill(t, v151TestRow(t, "_____{a{b} _____{c{d}", "", true, 4, 3))
			for i := range result.Lines {
				result.Lines[i].LineEnding = ending
			}
			result.Lines[len(result.Lines)-1].LineEnding = ""
			want := [][3]int{{5, 8, 9}, {5, 19, 20}}
			v151TestRun(t, result, "first", want)
			v151TestRun(t, result, "second", want)
		})
	}
	t.Run("unclosed block at EOF", func(t *testing.T) {
		result := v151TestDocument(t, v151TestOpen(t, "multifill", 3, 2), v151TestRow(t, "_____{a{", "", true, 4, 3))
		result.Lines[4].LineEnding = ""
		v151TestRun(t, result, "current", [][3]int{{5, 8, 9}})
	})
}

// v151TestMultifill окружает явно заданные строки реальным блоком в строке 4.
func v151TestMultifill(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v151TestOpen(t, "multifill", 3, 2)}, rows...)
	return v151TestDocument(t, append(body, v151TestClose(t, 4, 3))...)
}

// v151TestDocument добавляет обязательный контекст и ошибки для unparsed.
// Типы, родители, значения и глубина заданы тестами, а не восстановлены парсером.
func v151TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Unit 1"
	lines := append([]model.Line{
		v151TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v151TestLine(t, "# Unit 1", "heading", 0, 0, v151TestElement(t, "heading-level", "#", "1", 1), v151TestElement(t, "title", "Unit 1", "Unit 1", 3)),
		v151TestTag(t, "task", "identifier", "1", 2, 1),
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
			element.Value, element.ErrorIDs = nil, []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Не разобран фрагмент исходной строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v151TestRow задаёт содержимое или явно ошибочный неразобранный текст.
func v151TestRow(t *testing.T, raw, value string, invalid bool, parent, depth int) model.Line {
	t.Helper()
	if invalid {
		return v151TestLine(t, raw, "invalid", parent, depth, v151TestElement(t, "unparsed", raw, "", 1))
	}
	return v151TestLine(t, raw, "content", parent, depth, v151TestElement(t, "content", raw, value, 1))
}

// v151TestTag строит известное объявление с явно указанным типом аргумента.
func v151TestTag(t *testing.T, name string, kind model.ElementType, text string, parent, depth int) model.Line {
	t.Helper()
	return v151TestLine(t, "@"+name+" "+text, "tag", parent, depth,
		v151TestElement(t, "tag", "@"+name, name, 1), v151TestElement(t, kind, text, text, len(name)+3))
}

// v151TestOpen задаёт объявление настоящего блока.
func v151TestOpen(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v151TestLine(t, "@"+name+" {", "block-start", parent, depth, v151TestElement(t, "tag", "@"+name, name, 1),
		v151TestElement(t, "block-open", "{", "{", len(name)+3))
}

// v151TestClose закрывает явно указанный блок.
func v151TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v151TestLine(t, "}", "block-end", parent, depth, v151TestElement(t, "block-close", "}", "}", 1))
}

// v151TestLine сохраняет заданные тип, родителя и глубину строки.
func v151TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v151TestElement измеряет входной элемент; ожидания V151 заданы отдельно числами.
func v151TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v151TestRegistry использует полный штатный каталог регистраций.
func v151TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v151TestRun проверяет новые V151 полного конвейера и сохранность исходной модели.
// want задаёт строку, включённую и исключённую Unicode-колонки каждого сообщения.
func v151TestRun(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v151TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v151TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v151TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.Source == source && entry.DiagnosticCode == "V151" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V151 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.Fatal || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" ||
			entry.Message != "Внутри ответа пропуска есть неэкранированная открывающая скобка." {
			t.Fatalf("occurrence %d = %+v, location %+v; want %+v and exact metadata", i, entry, entry.Location, location)
		}
		if !result.Document.HasErrors || !result.Lines[position[0]-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line diagnostic linked to element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v151TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v151TestSnapshot(t, &restored) != before {
		t.Fatal("source or prior history changed")
	}
}

// v151TestSnapshot сохраняет независимое представление модели для сравнения.
func v151TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

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

const v165TestMessage = "В нижнем списке @matching меньше двух вариантов."

// v165TestWant задаёт исходные строку/колонку начала и исключающего конца, с единицы.
type v165TestWant [4]int

func TestV165RegistrationAndVersions(t *testing.T) {
	registry := v165TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V165" {
			continue
		}
		count++
		if definition.Message != v165TestMessage || definition.Severity != "error" ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V165 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.2.8"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v165TestDocument(t)
			result.Document.DSLVersion = &version
			before := v165TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v165TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v165TestSeparator задаёт классификацию строки явно, без распознавания Raw.
func v165TestSeparator(t *testing.T, raw string, parent int, invalid bool) model.Line {
	t.Helper()
	if invalid {
		return v165TestLine(t, raw, "invalid", parent, v165TestElement(t, "unparsed", raw, "", 1))
	}
	return v165TestLine(t, raw, "separator", parent)
}

// v165TestBlockDocument окружает явно заданное содержимое блоком на строке 3.
func v165TestBlockDocument(t *testing.T, name string, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v165TestOpen(t, name, "", "", 2)}, body...)
	return v165TestDocument(t, append(lines, v165TestClose(t, 3))...)
}

// v165TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v165TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v165TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v165TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v165TestTag задаёт известное имя и явно указанный тип аргумента.
func v165TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v165TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v165TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v165TestLine(t, raw, "tag", parent, elements...)
}

// v165TestOpen добавляет открытие к известному объявлению блока.
func v165TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v165TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v165TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v165TestClose задаёт закрытие конкретного фигурного блока.
func v165TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v165TestLine(t, "}", "block-end", parent, v165TestElement(t, "block-close", "}", "}", 1))
}

// v165TestText сохраняет исходное и семантическое содержимое независимо.
func v165TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v165TestLine(t, raw, "content", parent, v165TestElement(t, "content", raw, value, 1))
}

// v165TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v165TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v165TestTag(t, "dsl-version", "1.2", "version", 0),
		v165TestLine(t, "# Lesson", "heading", 0, v165TestElement(t, "heading-level", "#", "1", 1),
			v165TestElement(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j := range line.Elements {
			line.Elements[j].ErrorIDs = append([]string{}, line.Elements[j].ErrorIDs...)
			if line.Elements[j].ElementType == "unparsed" {
				v165TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v165TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v165TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}}})
	// Все записи здесь принадлежат исходному парсеру. Контракт требует
	// порядка по позиции, даже если ошибки элементов добавлялись позднее.
	slices.SortStableFunc(result.Diagnostics, func(a, b model.Diagnostic) int {
		if a.Location.Start.Line != b.Location.Start.Line {
			return a.Location.Start.Line - b.Location.Start.Line
		}
		return a.Location.Start.Column - b.Location.Start.Column
	})
}

// v165TestRegistry использует полный штатный каталог без отдельной регистрации.
func v165TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v165TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v165TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v165TestLineReads считает обращения V165 к публичному построчному входу.
type v165TestLineReads struct {
	diagnostics.Lines
	reads int
}

func (lines *v165TestLineReads) Line(number int) (diagnostics.Line, bool) {
	lines.reads++
	return lines.Lines.Line(number)
}

// v165TestRun сравнивает новые V165 после полного Prepare/Run с явным ожиданием.
func v165TestRun(t *testing.T, result *model.Result, processingID string, want []v165TestWant) {
	t.Helper()
	before := v165TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v165TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !slices.ContainsFunc(checks, func(d diagnostics.Definition) bool { return d.Code == "V165" }) {
		t.Fatal("V165 is missing from the prepared catalog")
	}
	if v165TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	var reads int
	for i := range checks {
		if checks[i].Code == "V165" {
			check := checks[i].Check
			checks[i].Check = func(lines diagnostics.Lines, findings diagnostics.Findings) error {
				observed := &v165TestLineReads{Lines: lines}
				err := check(observed, findings)
				reads = observed.reads
				return err
			}
		}
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Запас на несколько проходов исключает повторное сканирование хвоста и предков.
	if limit := 16*len(result.Lines) + 16; reads > limit {
		t.Errorf("V165 read %d lines, want at most %d for %d source lines", reads, limit, len(result.Lines))
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V165" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V165 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[2], Column: expected[3]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v165TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V165 occurrence %d = %+v, want location %+v, no detail or related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Fatal("V165 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("block-scoped V165 created an element error link")
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
	if err := json.Unmarshal([]byte(v165TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Убираются только добавленная история и производные флаги/ссылки.
	// Исходные строки, семантические значения и прежние диагностики сохраняются.
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("source elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:oldCount]
	if v165TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

// v165TestLower задаёт верхний элемент на строке 4 и разделитель на строке 5.
// Явно переданные нижние строки начинаются со строки 6; закрытие добавляется после них.
func v165TestLower(t *testing.T, lower ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v165TestText(t, "Солнце — звезда?", "Солнце — звезда?", 3), v165TestSeparator(t, "---", 3, false)}, lower...)
	return v165TestBlockDocument(t, "matching", body...)
}

func TestV165ReferenceAndExample(t *testing.T) {
	// Добавлены только техническая версия и заголовок документа.
	for _, tc := range []struct {
		name, fragment string
		result         *model.Result
		want           []v165TestWant
	}{
		{"reference", "@matching {\nСолнце — звезда?\n---\nДа\nНет\n}",
			v165TestLower(t, v165TestText(t, "Да", "Да", 3), v165TestText(t, "Нет", "Нет", 3)), nil},
		{"example", "@matching {\nСолнце — звезда?\n---\nДа\n}",
			v165TestLower(t, v165TestText(t, "Да", "Да", 3)), []v165TestWant{{5, 1, 7, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw []string
			for _, line := range tc.result.Lines[2:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("table fragment changed")
			}
			v165TestRun(t, tc.result, "current", tc.want)
		})
	}
}

func TestV165Counts(t *testing.T) {
	text := func(raw string) model.Line { return v165TestText(t, raw, raw, 3) }
	for _, tc := range []struct {
		name  string
		lower []model.Line
		want  []v165TestWant
	}{
		{"zero", nil, []v165TestWant{{5, 1, 6, 2}}},
		{"one", []model.Line{text("Да")}, []v165TestWant{{5, 1, 7, 2}}},
		{"two", []model.Line{text("Да"), text("Нет")}, nil},
		{"three", []model.Line{text("Да"), text("Нет"), text("Возможно")}, nil},
		{"identical labels count twice", []model.Line{text("Да"), text("Да")}, nil},
		{"case does not affect count", []model.Line{text("Да"), text("ДА")}, nil},
		{"punctuation counts", []model.Line{text("?"), text("!")}, nil},
		{"NBSP is not normalized", []model.Line{text("Да"), text("\u00a0")}, nil},
		{"Unicode labels", []model.Line{text("🌞"), text("星")}, nil},
		{"blank lines only", []model.Line{v165TestLine(t, "", "blank", 3), v165TestLine(t, " \t", "blank", 3)}, []v165TestWant{{5, 1, 8, 2}}},
		{"blank between options", []model.Line{text("Да"), v165TestLine(t, "", "blank", 3), text("Нет")}, nil},
		{"one option with blanks", []model.Line{v165TestLine(t, "\t", "blank", 3), text("Да"), v165TestLine(t, "", "blank", 3)}, []v165TestWant{{5, 1, 9, 2}}},
		{"several content elements form one option", []model.Line{v165TestLine(t, "Да Нет", "content", 3,
			v165TestElement(t, "content", "Да", "Да", 1), v165TestElement(t, "content", "Нет", "Нет", 4))}, []v165TestWant{{5, 1, 7, 2}}},
		{"escaped separator is an option", []model.Line{v165TestText(t, `\---`, "---", 3), text("Плюс")}, nil},
		{"escaped tag and heading are options", []model.Line{v165TestText(t, `\@hint`, "@hint", 3), v165TestText(t, `\# Heading`, "# Heading", 3)}, nil},
		{"escaped brace does not close block", []model.Line{v165TestText(t, `\}`, "}", 3), text("Нет")}, nil},
		{"inline tag excluded", []model.Line{v165TestTag(t, "hint", "Подумайте.", "content", 3)}, []v165TestWant{{5, 1, 7, 2}}},
		{"answer tag excluded", []model.Line{v165TestTag(t, "answer", "Да", "content", 3)}, []v165TestWant{{5, 1, 7, 2}}},
		{"ordinary and tag", []model.Line{text("Да"), v165TestTag(t, "hint", "Подумайте.", "content", 3)}, []v165TestWant{{5, 1, 8, 2}}},
		{"nested hint excluded", []model.Line{v165TestOpen(t, "hint", "", "", 3), v165TestText(t, "Да", "Да", 6), v165TestText(t, "Нет", "Нет", 6), v165TestClose(t, 6)}, []v165TestWant{{5, 1, 10, 2}}},
		{"nested answer excluded", []model.Line{v165TestOpen(t, "answer", "", "", 3), v165TestText(t, "Да", "Да", 6), v165TestText(t, "Нет", "Нет", 6), v165TestClose(t, 6)}, []v165TestWant{{5, 1, 10, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v165TestRun(t, v165TestLower(t, tc.lower...), "current", tc.want) })
	}
	t.Run("different list sizes", func(t *testing.T) {
		v165TestRun(t, v165TestBlockDocument(t, "matching", text("Солнце — звезда?"), text("Земля — планета?"), text("Луна — звезда?"),
			v165TestSeparator(t, "---", 3, false), text("Да"), text("Нет")), "current", nil)
	})
	t.Run("upper size cannot replace lower", func(t *testing.T) {
		v165TestRun(t, v165TestBlockDocument(t, "matching", text("Солнце — звезда?"), text("Земля — планета?"), text("Луна — звезда?"),
			v165TestSeparator(t, "---", 3, false), text("Да")), "current", []v165TestWant{{7, 1, 9, 2}})
	})
}

func TestV165Separators(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		v165TestRun(t, v165TestBlockDocument(t, "matching", v165TestText(t, "Вопрос?", "Вопрос?", 3)), "current", nil)
	})
	t.Run("two", func(t *testing.T) {
		v165TestRun(t, v165TestLower(t, v165TestText(t, "Да", "Да", 3), v165TestText(t, "Нет", "Нет", 3), v165TestSeparator(t, "---", 3, false)), "current", nil)
	})
	for _, raw := range []string{`\---`, "----", "--", "- - -", "--- текст", "\u00a0---", "---\u00a0"} {
		t.Run("not a separator "+strconv.Quote(raw), func(t *testing.T) {
			value := raw
			if raw == `\---` {
				value = "---"
			}
			v165TestRun(t, v165TestBlockDocument(t, "matching", v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestText(t, raw, value, 3), v165TestText(t, "Да", "Да", 3)), "current", nil)
		})
	}
	t.Run("only nested separator", func(t *testing.T) {
		v165TestRun(t, v165TestBlockDocument(t, "matching", v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestOpen(t, "answer", "", "", 3),
			v165TestText(t, "---", "---", 5), v165TestClose(t, 5), v165TestText(t, "Да", "Да", 3)), "current", nil)
	})
	t.Run("nested separator does not duplicate direct one", func(t *testing.T) {
		v165TestRun(t, v165TestLower(t, v165TestOpen(t, "hint", "", "", 3), v165TestText(t, "---", "---", 6), v165TestClose(t, 6),
			v165TestText(t, "Да", "Да", 3)), "current", []v165TestWant{{5, 1, 10, 2}})
	})
	t.Run("invalid separator still recognizable by raw", func(t *testing.T) {
		v165TestRun(t, v165TestBlockDocument(t, "matching", v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, true),
			v165TestText(t, "Да", "Да", 3)), "current", []v165TestWant{{5, 1, 7, 2}})
	})
	t.Run("indented delimiter and closing", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 3),
			v165TestSeparator(t, " \t--- \t", 3, false), v165TestText(t, "Да", "Да", 3),
			v165TestLine(t, "\t} ", "block-end", 3, v165TestElement(t, "block-close", "}", "}", 2)))
		v165TestRun(t, result, "current", []v165TestWant{{5, 3, 7, 3}})
	})
}

// v165TestInvalid задаёт исходную ошибочную строку, не восстанавливая её смысл.
func v165TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v165TestLine(t, raw, "invalid", parent, v165TestElement(t, "unparsed", raw, "", 1))
}

func TestV165UncertainContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  model.Line
	}{
		{"invalid", v165TestInvalid(t, "@unknown", 3)},
		{"partly parsed", v165TestLine(t, "Нет {", "content", 3, v165TestElement(t, "content", "Нет", "Нет", 1), v165TestElement(t, "unparsed", "{", "", 5))},
	} {
		t.Run("lower "+tc.name, func(t *testing.T) {
			v165TestRun(t, v165TestLower(t, v165TestText(t, "Да", "Да", 3), tc.row), "current", nil)
		})
		t.Run("upper "+tc.name, func(t *testing.T) {
			v165TestRun(t, v165TestBlockDocument(t, "matching", tc.row, v165TestSeparator(t, "---", 3, false), v165TestText(t, "Да", "Да", 3)), "current", []v165TestWant{{5, 1, 7, 2}})
		})
	}
	t.Run("excluded nested invalid content", func(t *testing.T) {
		v165TestRun(t, v165TestLower(t, v165TestOpen(t, "hint", "", "", 3), v165TestInvalid(t, "@unknown", 6), v165TestClose(t, 6),
			v165TestText(t, "Да", "Да", 3)), "current", []v165TestWant{{5, 1, 10, 2}})
	})
	t.Run("excluded inline tag with bad argument", func(t *testing.T) {
		v165TestRun(t, v165TestLower(t, v165TestLine(t, "@hint {oops", "tag", 3, v165TestElement(t, "tag", "@hint", "hint", 1),
			v165TestElement(t, "unparsed", "{oops", "", 7)), v165TestText(t, "Да", "Да", 3)), "current", []v165TestWant{{5, 1, 8, 2}})
	})
	t.Run("unknown content in another block", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "note", "", "", 2), v165TestInvalid(t, "@unknown", 3), v165TestClose(t, 3),
			v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 6), v165TestSeparator(t, "---", 6, false),
			v165TestText(t, "Да", "Да", 6), v165TestClose(t, 6))
		v165TestRun(t, result, "current", []v165TestWant{{8, 1, 10, 2}})
	})
}

func TestV165UncertainBoundaries(t *testing.T) {
	t.Run("no closing", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false), v165TestText(t, "Да", "Да", 3))
		v165TestParserError(t, result, 3, 0, "Не закрыт блок.")
		v165TestRun(t, result, "current", nil)
	})
	t.Run("duplicate closing", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Да", "Да", 3), v165TestClose(t, 3), v165TestClose(t, 3))
		v165TestParserError(t, result, 8, 0, "Неоднозначное закрытие.")
		v165TestRun(t, result, "current", nil)
	})
	t.Run("nested unclosed block", func(t *testing.T) {
		result := v165TestLower(t, v165TestOpen(t, "hint", "", "", 3), v165TestText(t, "Да", "Да", 6))
		v165TestParserError(t, result, 6, 0, "Не закрыт вложенный блок.")
		v165TestRun(t, result, "current", nil)
	})
	t.Run("content after closing", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestClose(t, 3), v165TestText(t, "Да", "Да", 3))
		v165TestParserError(t, result, 7, 0, "Неоднозначная граница блока.")
		v165TestRun(t, result, "current", nil)
	})
	t.Run("unparsed closing tail", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Да", "Да", 3), v165TestLine(t, "} extra", "block-end", 3, v165TestElement(t, "block-close", "}", "}", 1), v165TestElement(t, "unparsed", "extra", "", 3)))
		v165TestRun(t, result, "current", nil)
	})
}

func TestV165Contexts(t *testing.T) {
	for _, name := range []string{"matching", "MATCHING", "Matching"} {
		t.Run(name, func(t *testing.T) {
			v165TestRun(t, v165TestBlockDocument(t, name, v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false),
				v165TestText(t, "Да", "Да", 3)), "current", []v165TestWant{{5, 1, 7, 2}})
		})
	}
	t.Run("instruction is not an option", func(t *testing.T) {
		v165TestRun(t, v165TestDocument(t, v165TestOpen(t, "matching", "Выберите ответ.", "content", 2),
			v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false), v165TestText(t, "Да", "Да", 3), v165TestClose(t, 3)),
			"current", []v165TestWant{{5, 1, 7, 2}})
	})
	t.Run("task", func(t *testing.T) {
		result := v165TestDocument(t, v165TestTag(t, "task", "t", "identifier", 2), v165TestOpen(t, "matching", "", "", 3),
			v165TestText(t, "Вопрос?", "Вопрос?", 4), v165TestSeparator(t, "---", 4, false), v165TestText(t, "Да", "Да", 4), v165TestClose(t, 4),
			v165TestTag(t, "endtask", "", "", 3))
		v165TestRun(t, result, "current", []v165TestWant{{6, 1, 8, 2}})
	})
	t.Run("nested variant", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "variants", "", "", 2), v165TestTag(t, "variant", "A", "name", 3),
			v165TestOpen(t, "matching", "", "", 4), v165TestText(t, "Вопрос?", "Вопрос?", 5), v165TestSeparator(t, "---", 5, false),
			v165TestText(t, "Да", "Да", 5), v165TestClose(t, 5), v165TestTag(t, "variant", "B", "name", 3), v165TestText(t, "Read.", "Read.", 10), v165TestClose(t, 3))
		v165TestRun(t, result, "current", []v165TestWant{{7, 1, 9, 2}})
	})
	t.Run("independent blocks", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Да", "Да", 3), v165TestClose(t, 3), v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Вопрос?", "Вопрос?", 8),
			v165TestSeparator(t, "---", 8, false), v165TestClose(t, 8))
		v165TestRun(t, result, "current", []v165TestWant{{5, 1, 7, 2}, {10, 1, 11, 2}})
	})
	t.Run("nested matching counted independently", func(t *testing.T) {
		result := v165TestLower(t, v165TestOpen(t, "matching", "", "", 3), v165TestText(t, "Вопрос?", "Вопрос?", 6), v165TestSeparator(t, "---", 6, false),
			v165TestText(t, "Да", "Да", 6), v165TestClose(t, 6), v165TestText(t, "Нет", "Нет", 3))
		v165TestParserError(t, result, 6, 0, "Недопустимое вложение matching.")
		v165TestRun(t, result, "current", []v165TestWant{{5, 1, 12, 2}, {8, 1, 10, 2}})
	})
	for _, name := range []string{"example", "table", "hint", "editor"} {
		t.Run("excluded "+name, func(t *testing.T) {
			middle := v165TestSeparator(t, "---", 3, false)
			if name == "hint" || name == "editor" {
				middle = v165TestText(t, "---", "---", 3)
			}
			v165TestRun(t, v165TestBlockDocument(t, name, v165TestText(t, "Вопрос?", "Вопрос?", 3), middle, v165TestText(t, "Да", "Да", 3)), "current", nil)
		})
	}
	t.Run("literal matching template", func(t *testing.T) {
		v165TestRun(t, v165TestBlockDocument(t, "editor", v165TestText(t, `@matching \{`, "@matching {", 3),
			v165TestText(t, "Вопрос?", "Вопрос?", 3), v165TestText(t, "---", "---", 3), v165TestText(t, "Да", "Да", 3), v165TestText(t, `\}`, "}", 3)), "current", nil)
	})
	t.Run("inline matching does not create a block", func(t *testing.T) {
		result := v165TestDocument(t, v165TestTag(t, "matching", "Вопрос?", "content", 2), v165TestText(t, "---", "---", 2), v165TestText(t, "Да", "Да", 2))
		v165TestParserError(t, result, 3, 0, "Отсутствует открытие блока.")
		v165TestRun(t, result, "current", nil)
	})
}

func TestV165BlankContextPairs(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v165TestWant
	}{
		{"upper", []model.Line{v165TestText(t, "Question?", "Question?", 3), v165TestLine(t, " \t", "blank", 3),
			v165TestSeparator(t, "---", 3, false), v165TestText(t, "Yes", "Yes", 3)}, []v165TestWant{{6, 1, 8, 2}}},
		{"lower", []model.Line{v165TestText(t, "Question?", "Question?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Yes", "Yes", 3), v165TestLine(t, " \t", "blank", 3)}, []v165TestWant{{5, 1, 8, 2}}},
		{"zero lower options", []model.Line{v165TestText(t, "Question?", "Question?", 3),
			v165TestSeparator(t, "---", 3, false), v165TestLine(t, " \t", "blank", 3)}, []v165TestWant{{5, 1, 7, 2}}},
		{"two lower options", []model.Line{v165TestText(t, "Question?", "Question?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Yes", "Yes", 3), v165TestLine(t, " \t", "blank", 3), v165TestText(t, "Yes", "Yes", 3)}, nil},
		{"excluded hint", []model.Line{v165TestText(t, "Question?", "Question?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Yes", "Yes", 3), v165TestOpen(t, "hint", "", "", 3), v165TestText(t, "Think.", "Think.", 7),
			v165TestLine(t, " \t", "blank", 7), v165TestClose(t, 7)}, []v165TestWant{{5, 1, 11, 2}}},
		{"unknown upper content", []model.Line{v165TestInvalid(t, "@unknown", 3), v165TestLine(t, " \t", "blank", 3),
			v165TestSeparator(t, "---", 3, false), v165TestText(t, "Yes", "Yes", 3)}, []v165TestWant{{6, 1, 8, 2}}},
		{"unknown excluded content", []model.Line{v165TestText(t, "Question?", "Question?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Yes", "Yes", 3), v165TestOpen(t, "hint", "", "", 3), v165TestInvalid(t, "@unknown", 7),
			v165TestLine(t, " \t", "blank", 7), v165TestClose(t, 7)}, []v165TestWant{{5, 1, 11, 2}}},
		{"unknown lower content", []model.Line{v165TestText(t, "Question?", "Question?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Yes", "Yes", 3), v165TestLine(t, " \t", "blank", 3), v165TestInvalid(t, "@unknown", 3)}, nil},
	} {
		for _, root := range []bool{false, true} {
			t.Run(tc.name+"/root="+strconv.FormatBool(root), func(t *testing.T) {
				result := v165TestBlockDocument(t, "matching", tc.body...)
				if root {
					for i := range result.Lines {
						if result.Lines[i].LineType == "blank" {
							result.Lines[i].ParentLine, result.Lines[i].NestingLevel = nil, 0
						}
					}
				}
				v165TestRun(t, result, "current", tc.want)
			})
		}
	}
}

func TestV165BlankDoesNotHideUncertainBounds(t *testing.T) {
	for _, row := range []model.Line{
		v165TestText(t, "Foreign row", "Foreign row", 0),
		v165TestLine(t, " ", "blank", 0, v165TestElement(t, "unparsed", " ", "", 1)),
	} {
		t.Run(strconv.Quote(row.Raw), func(t *testing.T) {
			v165TestRun(t, v165TestLower(t, v165TestLine(t, "", "blank", 0), row,
				v165TestText(t, "Yes", "Yes", 3)), "current", nil)
		})
	}
	t.Run("unclosed excluded upper block", func(t *testing.T) {
		result := v165TestBlockDocument(t, "matching", v165TestOpen(t, "hint", "", "", 3),
			v165TestText(t, "Think.", "Think.", 4), v165TestLine(t, "", "blank", 0),
			v165TestSeparator(t, "---", 3, false), v165TestText(t, "Yes", "Yes", 3))
		v165TestParserError(t, result, 4, 0, "Не закрыт вложенный блок.")
		v165TestRun(t, result, "current", nil)
	})
	t.Run("independent blocks around root blank", func(t *testing.T) {
		result := v165TestDocument(t, v165TestOpen(t, "matching", "", "", 2),
			v165TestText(t, "Question?", "Question?", 3), v165TestSeparator(t, "---", 3, false),
			v165TestText(t, "Yes", "Yes", 3), v165TestClose(t, 3), v165TestLine(t, "", "blank", 0),
			v165TestOpen(t, "matching", "", "", 2), v165TestText(t, "Question?", "Question?", 9),
			v165TestSeparator(t, "---", 9, false), v165TestClose(t, 9))
		v165TestRun(t, result, "current", []v165TestWant{{5, 1, 7, 2}, {11, 1, 12, 2}})
	})
}

func TestV165LinearPreindex(t *testing.T) {
	for _, count := range []int{32, 96} {
		t.Run("independent blocks="+strconv.Itoa(count), func(t *testing.T) {
			var body []model.Line
			for i := 0; i < count; i++ {
				opening := len(body) + 3
				body = append(body, v165TestOpen(t, "matching", "", "", 2),
					v165TestText(t, "Question?", "Question?", opening), v165TestSeparator(t, "---", opening, false),
					v165TestText(t, "Yes", "Yes", opening), v165TestText(t, "No", "No", opening), v165TestClose(t, opening))
			}
			v165TestRun(t, v165TestDocument(t, body...), "current", nil)
		})
	}
	t.Run("deeply nested matching blocks", func(t *testing.T) {
		var body []model.Line
		var openings []int
		parent := 2
		for i := 0; i < 32; i++ {
			opening := len(body) + 3
			body = append(body, v165TestOpen(t, "matching", "", "", parent),
				v165TestText(t, "Question?", "Question?", opening), v165TestSeparator(t, "---", opening, false),
				v165TestText(t, "Yes", "Yes", opening), v165TestText(t, "No", "No", opening))
			openings, parent = append(openings, opening), opening
		}
		for i := len(openings) - 1; i >= 0; i-- {
			body = append(body, v165TestClose(t, openings[i]))
		}
		v165TestRun(t, v165TestDocument(t, body...), "current", nil)
	})
}

func TestV165EndingsAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run("ending "+strconv.Quote(string(eol)), func(t *testing.T) {
			result := v165TestLower(t, v165TestText(t, "Да", "Да", 3))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v165TestWant{{5, 1, 7, 2}}
			v165TestRun(t, result, "first", want)
			v165TestRun(t, v165TestLower(t, v165TestText(t, "Да", "Да", 3), v165TestText(t, "Нет", "Нет", 3)), "other", nil)
			v165TestRun(t, result, "second", want)
		})
	}
	t.Run("empty document", func(t *testing.T) {
		result := v165TestDocument(t)
		result.Lines = []model.Line{}
		result.Document.Metadata.Title = nil
		count := 0
		result.Document.LineCount = &count
		v165TestRun(t, result, "current", nil)
	})
}

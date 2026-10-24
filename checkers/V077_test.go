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

const v077TestMessage = "В инструкции объявлено несколько подсказок @hint."

// v077TestWant задаёт независимые диапазоны: строка, начало, исключающий конец.
type v077TestWant struct{ location, related [3]int }

func TestV077RegistrationAndVersions(t *testing.T) {
	registry := v077TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V077" {
			continue
		}
		count++
		if definition.Message != v077TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V077 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.8.11"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v077TestDocument(t)
			result.Document.DSLVersion = &version
			before := v077TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v077TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

func TestV077ReferenceAndExample(t *testing.T) {
	// Три строки версии и заголовков добавляют необходимый контекст шага.
	// Сам фрагмент задания, включая пробелы и переводы строк, не изменяется.
	for _, tc := range []struct {
		name, fragment string
		second         bool
		want           []v077TestWant
	}{
		{"reference", "@instr {\nComplete the sentence.\n@hint Use the present tense.\n}", false, nil},
		{"example", "@instr {\nComplete the sentence.\n@hint Use the present tense.\n@hint Check the subject.\n}", true,
			[]v077TestWant{{[3]int{7, 1, 6}, [3]int{6, 1, 6}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tail := []model.Line{v077TestTag(t, "hint", "Use the present tense.", "content", 4)}
			if tc.second {
				tail = append(tail, v077TestTag(t, "hint", "Check the subject.", "content", 4))
			}
			result := v077TestInstruction(t, tail...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v077TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV077FormsAndCounts(t *testing.T) {
	for _, tc := range []struct {
		name string
		tail []model.Line
		want []v077TestWant
	}{
		{"no hints", nil, nil},
		{"one block", []model.Line{v077TestOpen(t, "hint", "", "", 4),
			v077TestText(t, "Read carefully.", "Read carefully.", 6), v077TestClose(t, 6)}, nil},
		{"three inline", []model.Line{
			v077TestTag(t, "hint", "One.", "content", 4), v077TestTag(t, "hint", "Two.", "content", 4),
			v077TestTag(t, "hint", "Three.", "content", 4)},
			[]v077TestWant{{[3]int{7, 1, 6}, [3]int{6, 1, 6}}, {[3]int{8, 1, 6}, [3]int{6, 1, 6}}}},
		{"block then inline", []model.Line{v077TestOpen(t, "hint", "", "", 4),
			v077TestText(t, "Read carefully.", "Read carefully.", 6), v077TestClose(t, 6),
			v077TestTag(t, "hint", "Check.", "content", 4)},
			[]v077TestWant{{[3]int{9, 1, 6}, [3]int{6, 1, 6}}}},
		{"inline then block", []model.Line{v077TestTag(t, "hint", "Read carefully.", "content", 4),
			v077TestOpen(t, "hint", "", "", 4), v077TestText(t, "Check.", "Check.", 7), v077TestClose(t, 7)},
			[]v077TestWant{{[3]int{7, 1, 6}, [3]int{6, 1, 6}}}},
		{"two blocks", []model.Line{v077TestOpen(t, "hint", "", "", 4),
			v077TestText(t, "Read carefully.", "Read carefully.", 6), v077TestClose(t, 6),
			v077TestOpen(t, "hint", "", "", 4), v077TestText(t, "Check.", "Check.", 9), v077TestClose(t, 9)},
			[]v077TestWant{{[3]int{9, 1, 6}, [3]int{6, 1, 6}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v077TestRun(t, v077TestInstruction(t, tc.tail...), "current", tc.want)
		})
	}
}

func TestV077TextCaseAndWhitespace(t *testing.T) {
	for _, text := range []string{"Use the present tense.", "Different.", "Я😀", "Same  words\twith spaces."} {
		t.Run(text, func(t *testing.T) {
			result := v077TestInstruction(t,
				v077TestTag(t, "hint", "Use the present tense.", "content", 4),
				v077TestTag(t, "hint", text, "content", 4))
			v077TestRun(t, result, "current", []v077TestWant{{[3]int{7, 1, 6}, [3]int{6, 1, 6}}})
		})
	}
	t.Run("indent and mixed case", func(t *testing.T) {
		result := v077TestInstruction(t,
			v077TestLine(t, "\t @HiNt First.", "tag", 4, v077TestElement(t, "tag", "@HiNt", "hint", 3),
				v077TestElement(t, "content", "First.", "First.", 9)),
			v077TestLine(t, "  @HINT Я😀", "tag", 4, v077TestElement(t, "tag", "@HINT", "hint", 3),
				v077TestElement(t, "content", "Я😀", "Я😀", 9)))
		result.Lines[3] = v077TestOpen(t, "InStR", "", "", 3)
		result.Lines[3].Number, result.Lines[3].NestingLevel = 4, 1
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{7, 3, 8}, [3]int{6, 3, 8}}})
	})
	t.Run("blank and editor do not reset count", func(t *testing.T) {
		result := v077TestInstruction(t, v077TestTag(t, "hint", "First.", "content", 4),
			v077TestLine(t, " \t", "blank", 4), v077TestTag(t, "editor", "Check wording.", "content", 4),
			v077TestTag(t, "hint", "Second.", "content", 4))
		// DSL 3.8.14 запрещает editor внутри instr; V077 не проверяет вложение.
		v077TestParserError(t, result, 8, 0, "Недопустимое вложение editor.")
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{9, 1, 6}, [3]int{6, 1, 6}}})
	})
}

func TestV077InstructionBoundariesAndDepth(t *testing.T) {
	t.Run("first child is hint", func(t *testing.T) {
		result := v077TestDocument(t, v077TestOpen(t, "instr", "", "", 3),
			v077TestTag(t, "hint", "First.", "content", 4), v077TestText(t, "Read.", "Read.", 4),
			v077TestTag(t, "hint", "Second.", "content", 4), v077TestClose(t, 4))
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{7, 1, 6}, [3]int{5, 1, 6}}})
	})
	t.Run("independent instructions", func(t *testing.T) {
		result := v077TestDocument(t,
			v077TestOpen(t, "instr", "", "", 3), v077TestText(t, "Read.", "Read.", 4),
			v077TestTag(t, "hint", "First.", "content", 4), v077TestClose(t, 4),
			v077TestOpen(t, "instr", "", "", 3), v077TestText(t, "Write.", "Write.", 8),
			v077TestTag(t, "hint", "First.", "content", 8), v077TestTag(t, "hint", "Second.", "content", 8),
			v077TestClose(t, 8))
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{11, 1, 6}, [3]int{10, 1, 6}}})
	})
	t.Run("instruction in fragment", func(t *testing.T) {
		result := v077TestDocument(t, v077TestOpen(t, "fragment", "note", "identifier", 3),
			v077TestOpen(t, "instr", "", "", 4), v077TestText(t, "Read.", "Read.", 5),
			v077TestTag(t, "hint", "First.", "content", 5), v077TestTag(t, "hint", "Second.", "content", 5),
			v077TestClose(t, 5), v077TestClose(t, 4), v077TestTag(t, "include", "note", "identifier", 3))
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{8, 1, 6}, [3]int{7, 1, 6}}})
	})
	t.Run("instruction in task stage fragment", func(t *testing.T) {
		result := v077TestDocument(t, v077TestTag(t, "task", "practice", "identifier", 3),
			v077TestTag(t, "step", "Prepare", "title", 4), v077TestOpen(t, "fragment", "note", "identifier", 5),
			v077TestOpen(t, "instr", "", "", 6), v077TestText(t, "Read.", "Read.", 7),
			v077TestTag(t, "hint", "First.", "content", 7), v077TestTag(t, "hint", "Second.", "content", 7),
			v077TestClose(t, 7), v077TestClose(t, 6), v077TestTag(t, "include", "note", "identifier", 5))
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{10, 1, 6}, [3]int{9, 1, 6}}})
	})
	t.Run("nearest block is hint", func(t *testing.T) {
		result := v077TestInstruction(t, v077TestOpen(t, "hint", "", "", 4),
			v077TestTag(t, "hint", "Nested.", "content", 6), v077TestClose(t, 6),
			v077TestTag(t, "hint", "Second direct hint.", "content", 4))
		// Распознанное запрещённое вложение по DSL 3.11.10 не становится
		// непосредственной подсказкой внешней инструкции.
		v077TestParserError(t, result, 7, 0, "Недопустимое вложение hint.")
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{9, 1, 6}, [3]int{6, 1, 6}}})
	})
	t.Run("standalone and after inline instr", func(t *testing.T) {
		result := v077TestDocument(t, v077TestTag(t, "hint", "Standalone.", "content", 3),
			v077TestTag(t, "instr", "Read.", "content", 3),
			v077TestTag(t, "hint", "One.", "content", 3), v077TestTag(t, "hint", "Two.", "content", 3))
		v077TestRun(t, result, "current", nil)
	})
	t.Run("hint in another block precedes first direct hint", func(t *testing.T) {
		result := v077TestInstruction(t, v077TestOpen(t, "text", "", "", 4),
			v077TestText(t, "Read carefully.", "Read carefully.", 6),
			v077TestTag(t, "hint", "Nested.", "content", 6), v077TestClose(t, 6),
			v077TestTag(t, "hint", "First direct hint.", "content", 4),
			v077TestTag(t, "hint", "Second direct hint.", "content", 4))
		// DSL 3.0.7 и 3.8.14: оба запрещённых вложения сохранены парсером.
		v077TestParserError(t, result, 6, 0, "Недопустимое вложение text.")
		v077TestParserError(t, result, 8, 0, "Недопустимое вложение hint.")
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{11, 1, 6}, [3]int{10, 1, 6}}})
	})
}

func TestV077RecognizedDeclarations(t *testing.T) {
	t.Run("escaped text is not hint", func(t *testing.T) {
		result := v077TestInstruction(t, v077TestTag(t, "hint", "Only hint.", "content", 4),
			v077TestText(t, "\\@hint Escaped.", "@hint Escaped.", 4))
		v077TestRun(t, result, "current", nil)
	})
	for _, container := range []string{"editor", "multifill"} {
		t.Run("literal "+container, func(t *testing.T) {
			result := v077TestDocument(t, v077TestOpen(t, container, "", "", 3),
				v077TestText(t, "@instr \\{", "@instr {", 4),
				v077TestText(t, "@hint First.", "@hint First.", 4),
				v077TestText(t, "@hint Second.", "@hint Second.", 4), v077TestClose(t, 4),
				v077TestText(t, "Read.", "Read.", 3))
			v077TestRun(t, result, "current", nil)
		})
	}
	t.Run("block comment does not reset count", func(t *testing.T) {
		result := v077TestInstruction(t, v077TestTag(t, "hint", "First.", "content", 4),
			v077TestOpen(t, "editor", "", "", 4), v077TestText(t, "@hint Literal one.", "@hint Literal one.", 7),
			v077TestText(t, "@hint Literal two.", "@hint Literal two.", 7), v077TestClose(t, 7),
			v077TestTag(t, "hint", "Second.", "content", 4))
		// Запрет editor внутри instr относится к вложению, а не к счёту hint.
		v077TestParserError(t, result, 7, 0, "Недопустимое вложение editor.")
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{11, 1, 6}, [3]int{6, 1, 6}}})
	})
	t.Run("recognized tag with unparsed argument", func(t *testing.T) {
		result := v077TestInstruction(t, v077TestTag(t, "hint", "First.", "content", 4),
			v077TestLine(t, "@hint\tSecond.", "invalid", 4,
				v077TestElement(t, "tag", "@hint", "hint", 1),
				v077TestElement(t, "unparsed", "\tSecond.", "", 6)))
		v077TestRun(t, result, "current", []v077TestWant{{[3]int{7, 1, 6}, [3]int{6, 1, 6}}})
	})
}

func TestV077EndingsEmptyInputAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, finalEOL := range []bool{false, true} {
			t.Run(strconv.Quote(string(eol))+strconv.FormatBool(finalEOL), func(t *testing.T) {
				result := v077TestInstruction(t, v077TestTag(t, "hint", "First.", "content", 4),
					v077TestTag(t, "hint", "Second.", "content", 4))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if !finalEOL {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				want := []v077TestWant{{[3]int{7, 1, 6}, [3]int{6, 1, 6}}}
				v077TestRun(t, result, "first-run", want)
				v077TestRun(t, v077TestInstruction(t, v077TestTag(t, "hint", "Only.", "content", 4)), "other-document", nil)
				v077TestRun(t, result, "second-run", want)
			})
		}
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v077TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v077TestRun(t, result, "current", nil)
		})
	}
}

// v077TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v077TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v077TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v077TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v077TestTag задаёт известное имя и явно указанный тип аргумента.
func v077TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v077TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v077TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v077TestLine(t, raw, "tag", parent, elements...)
}

// v077TestOpen добавляет открытие к известному объявлению блока.
func v077TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v077TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v077TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v077TestClose задаёт закрытие конкретного фигурного блока.
func v077TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v077TestLine(t, "}", "block-end", parent, v077TestElement(t, "block-close", "}", "}", 1))
}

// v077TestText сохраняет исходное и семантическое содержимое независимо.
func v077TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v077TestLine(t, raw, "content", parent, v077TestElement(t, "content", raw, value, 1))
}

// v077TestInstruction окружает подсказки полноценной инструкцией на строке 4.
func v077TestInstruction(t *testing.T, tail ...model.Line) *model.Result {
	t.Helper()
	body := []model.Line{v077TestOpen(t, "instr", "", "", 3),
		v077TestText(t, "Complete the sentence.", "Complete the sentence.", 4)}
	body = append(body, tail...)
	return v077TestDocument(t, append(body, v077TestClose(t, 4))...)
}

// v077TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v077TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v077TestTag(t, "dsl-version", "1.2", "version", 0),
		v077TestLine(t, "# Lesson", "heading", 0, v077TestElement(t, "heading-level", "#", "1", 1),
			v077TestElement(t, "title", "Lesson", "Lesson", 3)),
		v077TestLine(t, "### Practice", "heading", 0, v077TestElement(t, "heading-level", "###", "3", 1),
			v077TestElement(t, "title", "Practice", "Practice", 5)),
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
				v077TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v077TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v077TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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
}

// v077TestRegistry использует полный штатный каталог без отдельной регистрации.
func v077TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v077TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v077TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v077TestRun сравнивает новые V077 после полного Prepare/Run с явным ожиданием.
func v077TestRun(t *testing.T, result *model.Result, processingID string, want []v077TestWant) {
	t.Helper()
	before := v077TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v077TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v077TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V077" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V077 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.location[0], Column: expected.location[1]},
			End: model.Position{Line: expected.location[0], Column: expected.location[2]}}
		related := model.Location{Start: model.Position{Line: expected.related[0], Column: expected.related[1]},
			End: model.Position{Line: expected.related[0], Column: expected.related[2]}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v077TestMessage ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("V077 occurrence %d = %+v, want location %+v, related %+v, no detail", i, entry, location, related)
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V077 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V077 created an element error link")
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
	if err := json.Unmarshal([]byte(v077TestSnapshot(t, result)), &restored); err != nil {
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
	if v077TestSnapshot(t, &restored) != before {
		t.Fatal("source data or prior history changed")
	}
}

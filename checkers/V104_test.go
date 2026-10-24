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

const v104TestMessage = "Все ячейки строки таблицы пусты."

// v104TestWant задаёт независимый диапазон с исключающей правой границей.
type v104TestWant struct{ line, start, end int }

func TestV104RegistrationAndVersions(t *testing.T) {
	registry := v104TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V104" {
			continue
		}
		count++
		if definition.Message != v104TestMessage || definition.Severity != "recommendation" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V104 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "3.4.7"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v104TestDocument(t)
			result.Document.DSLVersion = &version
			before := v104TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v104TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v104TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v104TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v104TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v104TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v104TestTag задаёт известное имя и явно указанный тип аргумента.
func v104TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v104TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v104TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v104TestLine(t, raw, "tag", parent, elements...)
}

// v104TestOpen добавляет открытие к известному объявлению блока.
func v104TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v104TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v104TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v104TestClose задаёт закрытие конкретного фигурного блока.
func v104TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v104TestLine(t, "}", "block-end", parent, v104TestElement(t, "block-close", "}", "}", 1))
}

// v104TestText сохраняет исходное и семантическое содержимое независимо.
func v104TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v104TestLine(t, raw, "content", parent, v104TestElement(t, "content", raw, value, 1))
}

// v104TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v104TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v104TestTag(t, "dsl-version", "1.2", "version", 0),
		v104TestLine(t, "# Lesson", "heading", 0, v104TestElement(t, "heading-level", "#", "1", 1),
			v104TestElement(t, "title", "Lesson", "Lesson", 3)),
		v104TestLine(t, "### Practice", "heading", 0, v104TestElement(t, "heading-level", "###", "3", 1),
			v104TestElement(t, "title", "Practice", "Practice", 5)),
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
				v104TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v104TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v104TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v104TestRegistry использует полный штатный каталог без отдельной регистрации.
func v104TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v104TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v104TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v104TestRun сравнивает новые V104 после штатных Prepare/Run с явным ожиданием.
func v104TestRun(t *testing.T, result *model.Result, processingID string, want []v104TestWant) {
	t.Helper()
	before := v104TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v104TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v104TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V104" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V104 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v104TestMessage ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V104 occurrence %d = %+v, want %+v, no related", i, entry, location)
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V104 created an element error link")
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
	if err := json.Unmarshal([]byte(v104TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Из сравнения исключаются только добавления конвейера и производные
	// флаги/ссылки. Исходные строки, метаданные и прежняя история сохраняются.
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
	if v104TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV104ReferenceAndExample(t *testing.T) {
	// Фрагменты сохранены после версии и заголовков контекста шага.
	// HTML-сущности таблицы задания означают обычные символы |, < и > в DSL.
	for _, tc := range []struct {
		name, fragment string
		rows           []model.Line
		want           []v104TestWant
	}{
		{"reference", "@table {\nA | B\nC | D\n}", []model.Line{
			v104TestText(t, "A | B", "A | B", 4), v104TestText(t, "C | D", "C | D", 4)}, nil},
		{"example", "@table {\nA | B\n|\n< | > |\nC | D\n}", []model.Line{
			v104TestText(t, "A | B", "A | B", 4), v104TestText(t, "|", "|", 4),
			v104TestText(t, "< | > |", "< | > |", 4), v104TestText(t, "C | D", "C | D", 4)},
			[]v104TestWant{{6, 1, 2}, {7, 1, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v104TestTable(t, tc.rows...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v104TestRun(t, result, "current", tc.want)
			// Документ может иметь V037 из-за минимального контекста без ID.
			// Рекомендация V104 не должна выставлять ошибки строк примера.
			for _, line := range result.Lines[3:] {
				if line.HasErrors {
					t.Fatal("recommendation introduced a line error flag")
				}
			}
		})
	}
}

func TestV104EmptyCellsAndPositions(t *testing.T) {
	for _, tc := range []struct {
		raw        string
		start, end int
	}{
		{"|", 1, 2}, {"||", 1, 3}, {"| |", 1, 4}, {"| \t |", 1, 6},
		{" \t| \t", 3, 4}, {"<|>", 1, 4}, {"< | | >", 1, 8}, {"<|>|<||>", 1, 9},
		{"<\t|\t>", 1, 6}, {"< | > |", 1, 8}, {" \t< | > | \t", 3, 10},
		{"|<|>", 1, 5}, {"<|>||", 1, 6}, {"<|> \t |", 1, 8},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			// Одна строка данных исключает постороннее нарушение однородности.
			v104TestRun(t, v104TestTable(t, v104TestText(t, tc.raw, strings.Trim(tc.raw, " \t"), 4)),
				"current", []v104TestWant{{5, tc.start, tc.end}})
		})
	}
	t.Run("separate content elements", func(t *testing.T) {
		row := v104TestLine(t, "<|> |", "content", 4, v104TestElement(t, "content", "<", "<", 1),
			v104TestElement(t, "content", "|>", "|>", 2), v104TestElement(t, "content", " |", " |", 4))
		v104TestRun(t, v104TestTable(t, row), "current", []v104TestWant{{5, 1, 6}})
	})
}

func TestV104NonemptyValues(t *testing.T) {
	for _, raw := range []string{"A |", "| A", "| A |", "<A|>", "<|A>", "<|A|>", "<|> | A", "<|>|<|A>",
		"<A|>|<|>", "<|>|A|<|>", ">", "0", "—", "😀", "Я", "--", "----", "x < y", "| > |", "&nbsp; |", "&#124;",
		"\u200b", "\ufeff", "＃", "｜", "＜|＞"} {
		t.Run(strconv.Quote(raw), func(t *testing.T) {
			v104TestRun(t, v104TestTable(t, v104TestText(t, raw, raw, 4)), "current", nil)
		})
	}
	// Перечень задан независимо от функций диагностики. Только U+0020/U+0009
	// исключаются из содержимого; остальные Unicode-пробелы не означают пустоту.
	for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680", "\u2000", "\u2001", "\u2002", "\u2003",
		"\u2004", "\u2005", "\u2006", "\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		for _, raw := range []string{space + "|", "|" + space, "<" + space + "|>", "<|" + space + ">"} {
			t.Run(strconv.Quote(raw), func(t *testing.T) {
				v104TestRun(t, v104TestTable(t, v104TestText(t, raw, raw, 4)), "current", nil)
			})
		}
	}
}

func TestV104Escapes(t *testing.T) {
	for _, tc := range []struct{ raw, value string }{
		{`\|`, "|"}, {`\<`, "<"}, {`\>`, ">"}, {`\\`, `\`},
		{`<\||>`, "<||>"}, {`\<|>`, "<|>"}, {`\---`, "---"},
		{`<\<|>`, "<<|>"}, {`<|\>>`, "<|>>"}, {`<\\|>`, `<\|>`},
		{`\\|`, `\|`}, {`\\\|`, `\|`}, {`\q |`, `\q |`},
		{`\ `, `\`}, {`|\`, `|\`}, {`\{|\}`, "{|}"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v104TestRun(t, v104TestTable(t, v104TestText(t, tc.raw, tc.value, 4)), "current", nil)
		})
	}
}

func TestV104MalformedDividedCells(t *testing.T) {
	for _, raw := range []string{"<|", "| <|", "<|<|>>", "<>", "< >", "| <> |", "<|>text", "<|>>", "<|><|>",
		"<|> <|>", "| <|>text |", "<|> | <>", "<|> | <|", "<||> | <|<|>>", `<|\>`, `<\|>`,
		`<|>\|`, `<|>\\|`, `<|>\ `} {
		t.Run(raw, func(t *testing.T) {
			// Неразобранная строка сопровождается исходной ошибкой парсера.
			v104TestRun(t, v104TestTable(t, v104TestInvalid(t, raw, 4)), "current", nil)
		})
	}
	t.Run("structure error does not propagate to next row", func(t *testing.T) {
		v104TestRun(t, v104TestTable(t, v104TestInvalid(t, "<|", 4), v104TestText(t, "|", "|", 4)),
			"current", []v104TestWant{{6, 1, 2}})
	})
	t.Run("partially parsed malformed cell", func(t *testing.T) {
		row := v104TestLine(t, "| <>", "invalid", 4, v104TestElement(t, "content", "| ", "| ", 1),
			v104TestElement(t, "unparsed", "<>", "", 3))
		v104TestRun(t, v104TestTable(t, row), "current", nil)
	})
}

func TestV104TablePartsAndRepeats(t *testing.T) {
	t.Run("header and body", func(t *testing.T) {
		result := v104TestTable(t, v104TestText(t, "|", "|", 4), v104TestSeparator(t, "---", 4), v104TestText(t, "|", "|", 4))
		v104TestRun(t, result, "current", []v104TestWant{{5, 1, 2}, {7, 1, 2}})
	})
	t.Run("repeated rows", func(t *testing.T) {
		v104TestRun(t, v104TestTable(t, v104TestText(t, "|", "|", 4), v104TestText(t, "|", "|", 4)),
			"current", []v104TestWant{{5, 1, 2}, {6, 1, 2}})
	})
	t.Run("physical blanks and separator are skipped", func(t *testing.T) {
		result := v104TestTable(t, v104TestLine(t, "", "blank", 4), v104TestText(t, "A | B", "A | B", 4),
			v104TestLine(t, " \t", "blank", 4), v104TestSeparator(t, " \t--- \t", 4),
			v104TestLine(t, "\t", "blank", 4), v104TestText(t, "C | D", "C | D", 4))
		v104TestRun(t, result, "current", nil)
	})
	t.Run("neighbors do not merge", func(t *testing.T) {
		result := v104TestTable(t, v104TestText(t, "A | B", "A | B", 4), v104TestText(t, "|", "|", 4),
			v104TestText(t, "C | D", "C | D", 4), v104TestText(t, "<|> |", "<|> |", 4))
		v104TestRun(t, result, "current", []v104TestWant{{6, 1, 2}, {8, 1, 6}})
	})
	t.Run("independent tables", func(t *testing.T) {
		result := v104TestDocument(t, v104TestOpen(t, "table", "", "", 3), v104TestText(t, "|", "|", 4), v104TestClose(t, 4),
			v104TestOpen(t, "table", "", "", 3), v104TestText(t, "<|>", "<|>", 7), v104TestClose(t, 7))
		v104TestRun(t, result, "current", []v104TestWant{{5, 1, 2}, {8, 1, 4}})
	})
}

func TestV104ContextsAndNesting(t *testing.T) {
	for _, name := range []string{"table", "TABLE", "TaBlE"} {
		t.Run(name, func(t *testing.T) {
			result := v104TestDocument(t, v104TestOpen(t, name, "|", "title", 3), v104TestText(t, "|", "|", 4), v104TestClose(t, 4))
			v104TestRun(t, result, "current", []v104TestWant{{5, 1, 2}})
		})
	}
	t.Run("table in fragment", func(t *testing.T) {
		result := v104TestDocument(t, v104TestOpen(t, "fragment", "intro", "identifier", 3), v104TestOpen(t, "table", "", "", 4),
			v104TestText(t, "|", "|", 5), v104TestClose(t, 5), v104TestClose(t, 4))
		v104TestRun(t, result, "current", []v104TestWant{{6, 1, 2}})
	})
	for _, inner := range []string{"note", "table"} {
		t.Run("nested "+inner, func(t *testing.T) {
			result := v104TestDocument(t, v104TestOpen(t, "table", "", "", 3), v104TestOpen(t, inner, "", "", 4),
				v104TestText(t, "|", "|", 5), v104TestClose(t, 5), v104TestText(t, "|", "|", 4), v104TestClose(t, 4))
			v104TestParserError(t, result, 5, 0, "Вложение блока запрещено внутри table.")
			want := []v104TestWant{{8, 1, 2}}
			if inner == "table" {
				want = []v104TestWant{{6, 1, 2}, {8, 1, 2}}
			}
			v104TestRun(t, result, "current", want)
		})
	}
	t.Run("logical nested parent", func(t *testing.T) {
		result := v104TestDocument(t, v104TestOpen(t, "table", "", "", 3), v104TestTag(t, "task", "nested", "identifier", 4),
			v104TestText(t, "|", "|", 5), v104TestClose(t, 4))
		v104TestParserError(t, result, 5, 0, "Вложение task запрещено внутри table.")
		v104TestRun(t, result, "current", nil)
	})
	t.Run("outside table", func(t *testing.T) {
		result := v104TestDocument(t, v104TestText(t, "|", "|", 3), v104TestOpen(t, "table", "", "", 3),
			v104TestText(t, "A | B", "A | B", 5), v104TestClose(t, 5), v104TestText(t, "<|>", "<|>", 3))
		v104TestRun(t, result, "current", nil)
	})
	for _, name := range []string{"editor", "multifill"} {
		t.Run("literal "+name, func(t *testing.T) {
			result := v104TestDocument(t, v104TestTag(t, "task", "practice", "identifier", 3), v104TestOpen(t, name, "", "", 4),
				v104TestText(t, "@table {", "@table {", 5), v104TestText(t, "|", "|", 5), v104TestText(t, "<|>", "<|>", 5), v104TestClose(t, 5))
			v104TestRun(t, result, "current", nil)
		})
	}
}

func TestV104EndingsEmptyAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v104TestTable(t, v104TestText(t, "A | B", "A | B", 4), v104TestText(t, "|", "|", 4),
				v104TestText(t, "< | > |", "< | > |", 4), v104TestText(t, "C | D", "C | D", 4))
			if eol == "" {
				result.Lines[len(result.Lines)-1].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v104TestWant{{6, 1, 2}, {7, 1, 8}}
			v104TestRun(t, result, "first", want)
			v104TestRun(t, v104TestTable(t, v104TestText(t, `\|`, "|", 4)), "other", nil)
			v104TestRun(t, result, "second", want)
		})
	}
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v104TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v104TestRun(t, result, "current", nil)
		})
	}
}

// v104TestInvalid задаёт ошибочную строку с целиком неразобранным Raw.
func v104TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v104TestLine(t, raw, "invalid", parent, v104TestElement(t, "unparsed", raw, "", 1))
}

// v104TestSeparator задаёт служебный разделитель, сохраняя исходные отступы.
func v104TestSeparator(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v104TestLine(t, raw, "separator", parent, v104TestElement(t, "content", raw, "---", 1))
}

// v104TestTable задаёт полноценную таблицу внутри шага с явными строками данных.
func v104TestTable(t *testing.T, rows ...model.Line) *model.Result {
	t.Helper()
	body := append([]model.Line{v104TestOpen(t, "table", "", "", 3)}, rows...)
	return v104TestDocument(t, append(body, v104TestClose(t, 4))...)
}

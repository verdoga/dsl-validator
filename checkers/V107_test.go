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

const v107TestMessage = "Тег записан без обязательного открытия блока."

const v107TestTextDetail = "Тег @text требует открытия { в строке объявления; закрывающая } должна находиться на отдельной строке после содержимого."

// v107TestWant задаёт независимый диапазон с исключающей правой границей.
type v107TestWant struct {
	line, start, end int
	detail           string
}

func TestV107RegistrationAndVersions(t *testing.T) {
	registry := v107TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count, tables := 0, 0
	for _, definition := range definitions {
		if definition.Code == "V094" {
			tables++
		}
		if definition.Code != "V107" {
			continue
		}
		count++
		if definition.Message != v107TestMessage || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V107 definitions = %d, want 1", count)
	}
	if tables != 1 {
		t.Fatalf("V094 definitions = %d, want 1 alongside V107", tables)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", got, err)
			}
			result := v107TestDocument(t)
			result.Document.DSLVersion = &version
			before := v107TestSnapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v107TestSnapshot(t, result) != before {
				t.Fatal("failed Prepare changed input")
			}
		})
	}
}

// v107TestElement создаёт входной элемент; ожидаемые диапазоны задаются отдельно.
func v107TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v107TestLine сохраняет явно заданные классификацию и родителя без разбора DSL.
func v107TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v107TestTag задаёт известное имя и явно указанный тип аргумента.
func v107TestTag(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v107TestElement(t, "tag", raw, strings.ToLower(name), 1)}
	if argument != "" {
		elements = append(elements, v107TestElement(t, kind, argument, argument, len(raw)+2))
		raw += " " + argument
	}
	return v107TestLine(t, raw, "tag", parent, elements...)
}

// v107TestOpen добавляет открытие к известному объявлению блока.
func v107TestOpen(t *testing.T, name, argument string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	line := v107TestTag(t, name, argument, kind, parent)
	line.Elements = append(line.Elements, v107TestElement(t, "block-open", "{", "{", utf8.RuneCountInString(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v107TestClose задаёт закрытие конкретного фигурного блока.
func v107TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v107TestLine(t, "}", "block-end", parent, v107TestElement(t, "block-close", "}", "}", 1))
}

// v107TestText сохраняет исходное и семантическое содержимое независимо.
func v107TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v107TestLine(t, raw, "content", parent, v107TestElement(t, "content", raw, value, 1))
}

// v107TestDocument добавляет контекст шага; родители передаются явно.
// unparsed получает исходную диагностику и обратную ссылку по контракту модели.
func v107TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v107TestTag(t, "dsl-version", "1.2", "version", 0),
		v107TestLine(t, "# Lesson", "heading", 0, v107TestElement(t, "heading-level", "#", "1", 1),
			v107TestElement(t, "title", "Lesson", "Lesson", 3)),
		v107TestLine(t, "### Practice", "heading", 0, v107TestElement(t, "heading-level", "###", "3", 1),
			v107TestElement(t, "title", "Practice", "Practice", 5)),
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
				v107TestParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v107TestParserError фиксирует исходную ошибку без несогласованного unparsed.
func v107TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
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

// v107TestRegistry использует полный штатный каталог без отдельной регистрации.
func v107TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v107TestSnapshot сохраняет исходные данные для проверки отсутствия изменений.
func v107TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// v107TestRun сравнивает новые V107 после штатных Prepare/Run с явным ожиданием.
func v107TestRun(t *testing.T, result *model.Result, processingID string, want []v107TestWant) {
	t.Helper()
	before := v107TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v107TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v107TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V107" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V107 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || entry.Message != v107TestMessage+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V107 occurrence %d = %+v, want %+v with specified detail and no related", i, entry, location)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Fatal("V107 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V107 created an element error link")
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
	if err := json.Unmarshal([]byte(v107TestSnapshot(t, result)), &restored); err != nil {
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
	if v107TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func TestV107ReferenceAndExample(t *testing.T) {
	// Версия и два заголовка задают контекст шага перед дословными фрагментами.
	for _, tc := range []struct {
		name, fragment string
		body           []model.Line
		want           []v107TestWant
	}{
		{"reference", "@text Reading {\nTom lives in London.\n}",
			[]model.Line{v107TestOpen(t, "text", "Reading", "title", 3),
				v107TestText(t, "Tom lives in London.", "Tom lives in London.", 4), v107TestClose(t, 4)}, nil},
		{"example", "@text Tom lives in London.",
			[]model.Line{v107TestMissing(t, "@text Tom lives in London.", "text", 1, 6, 3)},
			[]v107TestWant{{4, 1, 6, v107TestTextDetail}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v107TestDocument(t, tc.body...)
			var raw []string
			for _, line := range result.Lines[3:] {
				raw = append(raw, line.Raw)
			}
			if strings.Join(raw, "\n") != tc.fragment {
				t.Fatal("reference fragment changed")
			}
			v107TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV107AllTagsAndForms(t *testing.T) {
	for _, tc := range []struct {
		name, argument, detail string
		end                    int
	}{
		{"script", "Conversation", "Тег @script требует открытия { в строке объявления; закрывающая } должна находиться на отдельной строке после содержимого.", 8},
		{"text", "Reading", v107TestTextDetail, 6},
		{"key", "Answers", "Тег @key требует открытия { в строке объявления; закрывающая } должна находиться на отдельной строке после содержимого.", 5},
		{"fragment", "intro", "Тег @fragment требует открытия { в строке объявления; закрывающая } должна находиться на отдельной строке после содержимого.", 10},
	} {
		for _, parsed := range []bool{false, true} {
			t.Run(tc.name+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				// Активное задание обеспечивает допустимое окружение key.
				// Ошибка именно в отсутствии открытия; имя fragment присутствует.
				line := v107TestInvalid(t, "@"+tc.name+" "+tc.argument, 4)
				if parsed {
					line = v107TestMissing(t, line.Raw, tc.name, 1, tc.end, 4)
				}
				v107TestRun(t, v107TestDocument(t, v107TestTag(t, "task", "practice", "identifier", 3), line),
					"current", []v107TestWant{{5, 1, tc.end, tc.detail}})
			})
		}
		t.Run(tc.name+" opening", func(t *testing.T) {
			kind := model.ElementType("title")
			if tc.name == "fragment" {
				kind = "identifier"
			}
			v107TestRun(t, v107TestDocument(t, v107TestTag(t, "task", "practice", "identifier", 3),
				v107TestOpen(t, tc.name, tc.argument, kind, 4), v107TestText(t, "Read.", "Read.", 5),
				v107TestClose(t, 5)), "current", nil)
		})
	}
	for _, name := range []string{"text", "TEXT", "TeXt"} {
		for _, parsed := range []bool{false, true} {
			t.Run("case "+name+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v107TestInvalid(t, " \t@"+name+" Reading \t", 3)
				if parsed {
					line = v107TestMissing(t, line.Raw, "text", 3, 8, 3)
				}
				v107TestRun(t, v107TestDocument(t, line), "current", []v107TestWant{{4, 3, 8, v107TestTextDetail}})
			})
		}
	}
	t.Run("recognized tag without unparsed", func(t *testing.T) {
		result := v107TestDocument(t, v107TestTag(t, "text", "Reading", "title", 3))
		v107TestParserError(t, result, 4, 0, "Отсутствует открытие обязательного блока.")
		v107TestRun(t, result, "current", []v107TestWant{{4, 1, 6, v107TestTextDetail}})
	})
}

func TestV107NameBoundaries(t *testing.T) {
	// Перечень White_Space задан независимо от таблицы Unicode проверяемого кода.
	// CR и LF находятся только в LineEnding, а не внутри Raw.
	for _, boundary := range []string{"", " ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007", "\u2008",
		"\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000", "}"} {
		t.Run(strconv.Quote(boundary), func(t *testing.T) {
			v107TestRun(t, v107TestDocument(t, v107TestInvalid(t, "@text"+boundary, 3)), "current",
				[]v107TestWant{{4, 1, 6, v107TestTextDetail}})
		})
	}
	for _, raw := range []string{"@scriptExtra", "@textual", "@text.Title", "@text\u200bReading", "@text\ufeffReading",
		"@text\\{", "@TEXT-extra", "@fragmentID", "@unknown"} {
		t.Run(raw, func(t *testing.T) {
			v107TestRun(t, v107TestDocument(t, v107TestInvalid(t, raw, 3)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@text Reading`, "@text Reading"}, {` 	\@text Reading`, "@text Reading"},
		{`\\@text Reading`, `\@text Reading`}, {"Text @text Reading", "Text @text Reading"},
		{"\u00a0@text Reading", "\u00a0@text Reading"}, {"＠text Reading", "＠text Reading"},
	} {
		t.Run("literal "+tc.raw, func(t *testing.T) {
			v107TestRun(t, v107TestDocument(t, v107TestText(t, tc.raw, tc.value, 3)), "current", nil)
		})
	}
	for _, name := range []string{"example", "editor", "instr", "note", "alt", "hint", "wordlist"} {
		t.Run("inline "+name, func(t *testing.T) {
			v107TestRun(t, v107TestDocument(t, v107TestTag(t, name, "Read.", "content", 3)), "current", nil)
		})
	}
}

func TestV107LeavesTablesToV094(t *testing.T) {
	for _, name := range []string{"table", "TaBlE"} {
		for _, representation := range []string{"tag", "invalid with tag", "unparsed"} {
			for _, nested := range []bool{false, true} {
				t.Run(name+"/"+representation+"/nested="+strconv.FormatBool(nested), func(t *testing.T) {
					parent, number := 3, 4
					var body []model.Line
					if nested {
						body = append(body, v107TestOpen(t, "fragment", "intro", "identifier", 3))
						parent, number = 4, 5
					}
					raw := "@" + name + " Forms"
					line := v107TestInvalid(t, raw, parent)
					switch representation {
					case "tag":
						line = v107TestTag(t, name, "Forms", "title", parent)
					case "invalid with tag":
						line = v107TestMissing(t, raw, "table", 1, 7, parent)
					}
					body = append(body, line, v107TestMissing(t, "@text Reading", "text", 1, 6, parent))
					if nested {
						body = append(body, v107TestClose(t, parent))
					}
					result := v107TestDocument(t, body...)
					if representation == "tag" {
						v107TestParserError(t, result, number, 0, "Отсутствует открытие обязательного блока.")
					}
					// Обе проверки выполняются штатным каталогом: V107 относится
					// только к text, а отсутствие открытия table отмечается V094.
					v107TestRun(t, result, "current", []v107TestWant{{number + 1, 1, 6, v107TestTextDetail}})
					var tables []model.Diagnostic
					for _, entry := range result.Diagnostics {
						if entry.Source == "current" && entry.DiagnosticCode == "V094" {
							tables = append(tables, entry)
						}
					}
					location := model.Location{Start: model.Position{Line: number, Column: 1}, End: model.Position{Line: number, Column: 7}}
					if len(tables) != 1 || tables[0].Location == nil || *tables[0].Location != location ||
						tables[0].Message != "У @table отсутствует открывающая скобка блочной формы." ||
						tables[0].SeverityLevel != "error" || tables[0].DiagnosticScope != "line" || len(tables[0].RelatedLocations) != 0 {
						t.Fatalf("expected one specific V094 on table: %+v", tables)
					}
				})
			}
		}
	}
}

func TestV107OpeningEscapesAndPlacement(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		missing bool
	}{
		{`@text Reading \{`, true}, {`@text Reading \\{ tail`, false},
		{`@text Reading \\\{`, true}, {`@text Reading \\\\{ tail`, false},
		{`@text Reading \}`, true}, {`@text Reading \\}`, true},
		{`@text Reading \} { tail`, false}, {`@text Reading \{ { tail`, false},
		{`@text Reading \q{ tail`, false}, {`@text Reading \`, true},
		{`@text "Reading {"`, false}, {`@text "Reading \{"`, true},
		{`@text "Reading"`, true}, {`@text Reading &#123;`, true},
		{`@text Reading { tail`, false}, {`@text{`, false}, {`@text }{`, false},
		{"@text Чтение 😀 е\u0301 \\{", true}, {"@text Чтение 😀 { tail", false},
	} {
		for _, parsed := range []bool{false, true} {
			t.Run(tc.raw+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				// Эти строки ошибочны: нет открытия либо оно расположено неверно.
				// Сохраняются целиком unparsed или распознанный tag и unparsed-остаток.
				line := v107TestInvalid(t, tc.raw, 3)
				if parsed {
					line = v107TestMissing(t, tc.raw, "text", 1, 6, 3)
				}
				var want []v107TestWant
				if tc.missing {
					want = []v107TestWant{{4, 1, 6, v107TestTextDetail}}
				}
				v107TestRun(t, v107TestDocument(t, line), "current", want)
			})
		}
	}
	t.Run("decoded title does not provide opening", func(t *testing.T) {
		line := v107TestLine(t, `@text Reading \{`, "tag", 3,
			v107TestElement(t, "tag", "@text", "text", 1),
			v107TestElement(t, "title", `Reading \{`, "Reading {", 7))
		result := v107TestDocument(t, line)
		v107TestParserError(t, result, 4, 0, "Отсутствует открытие обязательного блока.")
		v107TestRun(t, result, "current", []v107TestWant{{4, 1, 6, v107TestTextDetail}})
	})
	t.Run("next physical line does not compensate", func(t *testing.T) {
		v107TestRun(t, v107TestDocument(t, v107TestInvalid(t, "@text Reading", 3),
			v107TestInvalid(t, "{", 3), v107TestText(t, "Read.", "Read.", 3), v107TestInvalid(t, "}", 3)),
			"current", []v107TestWant{{4, 1, 6, v107TestTextDetail}})
	})
	t.Run("parsed opening with unparsed tail", func(t *testing.T) {
		line := v107TestOpen(t, "text", "Reading", "title", 3)
		line.Raw += " tail"
		line.Elements = append(line.Elements, v107TestElement(t, "unparsed", "tail", "", 17))
		v107TestRun(t, v107TestDocument(t, line, v107TestText(t, "Read.", "Read.", 4), v107TestClose(t, 4)), "current", nil)
	})
	t.Run("missing closing belongs to other checks", func(t *testing.T) {
		v107TestRun(t, v107TestDocument(t, v107TestOpen(t, "text", "Reading", "title", 3),
			v107TestText(t, "Read.", "Read.", 4)), "current", nil)
	})
}

func TestV107NestedAndLiteralContents(t *testing.T) {
	t.Run("nested declaration", func(t *testing.T) {
		v107TestRun(t, v107TestDocument(t, v107TestOpen(t, "note", "", "", 3),
			v107TestInvalid(t, "@text Reading", 4), v107TestText(t, "Read.", "Read.", 4), v107TestClose(t, 4)),
			"current", []v107TestWant{{5, 1, 6, v107TestTextDetail}})
	})
	t.Run("deeper declaration", func(t *testing.T) {
		v107TestRun(t, v107TestDocument(t, v107TestOpen(t, "fragment", "intro", "identifier", 3),
			v107TestOpen(t, "note", "", "", 4), v107TestInvalid(t, "@text Reading", 5),
			v107TestText(t, "Read.", "Read.", 5), v107TestClose(t, 5), v107TestClose(t, 4)),
			"current", []v107TestWant{{6, 1, 6, v107TestTextDetail}})
	})
	for _, literal := range []string{"editor", "multifill", "EdItOr", "MuLtIfIlL"} {
		t.Run(literal, func(t *testing.T) {
			// В литеральном блоке псевдотеги остаются content, а не вымышленными
			// вложенными block-start. Проверяем также выход из области исключения.
			v107TestRun(t, v107TestDocument(t, v107TestTag(t, "task", "practice", "identifier", 3),
				v107TestOpen(t, literal, "", "", 4),
				v107TestText(t, "@text Reading", "@text Reading", 5),
				v107TestText(t, "@table", "@table", 5), v107TestText(t, "@script", "@script", 5),
				v107TestText(t, "@key", "@key", 5), v107TestText(t, "@fragment intro", "@fragment intro", 5),
				v107TestClose(t, 5), v107TestInvalid(t, "@text Outside", 4)), "current",
				[]v107TestWant{{12, 1, 6, v107TestTextDetail}})
		})
	}
	t.Run("editor under outer block", func(t *testing.T) {
		// DSL 2.8.9 и 4.9.8 разрешают editor перед первой ветвью variants;
		// внутри fragment редакторские теги запрещены пунктом 3.12.12.
		v107TestRun(t, v107TestDocument(t, v107TestOpen(t, "variants", "", "", 3),
			v107TestOpen(t, "editor", "", "", 4), v107TestText(t, "@text Reading", "@text Reading", 5),
			v107TestClose(t, 5), v107TestTag(t, "variant", "First", "name", 4),
			v107TestText(t, "Read.", "Read.", 8), v107TestTag(t, "variant", "Second", "name", 4),
			v107TestText(t, "Write.", "Write.", 10), v107TestClose(t, 4)), "current", nil)
	})
	t.Run("allowed container with missing opening", func(t *testing.T) {
		v107TestRun(t, v107TestDocument(t, v107TestOpen(t, "fragment", "intro", "identifier", 3),
			v107TestMissing(t, "@text Reading", "text", 1, 6, 4),
			v107TestText(t, "Read.", "Read.", 4), v107TestClose(t, 4)),
			"current", []v107TestWant{{5, 1, 6, v107TestTextDetail}})
	})
	t.Run("inline editor does not hide following declaration", func(t *testing.T) {
		v107TestRun(t, v107TestDocument(t, v107TestTag(t, "editor", "Check wording.", "content", 3),
			v107TestInvalid(t, "@text Reading", 3)), "current", []v107TestWant{{5, 1, 6, v107TestTextDetail}})
	})
}

func TestV107BoundariesAndHistory(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v107TestDocument(t, v107TestInvalid(t, "@text First", 3), v107TestInvalid(t, "@text Second", 3))
			if eol == "" {
				result.Lines[4].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
			}
			want := []v107TestWant{{4, 1, 6, v107TestTextDetail}, {5, 1, 6, v107TestTextDetail}}
			v107TestRun(t, result, "first", want)
			v107TestRun(t, v107TestDocument(t), "other", nil)
			v107TestRun(t, result, "second", want)
		})
	}
	t.Run("first and last physical line", func(t *testing.T) {
		result := v107TestDocument(t)
		line := v107TestTag(t, "text", "Reading", "title", 0)
		line.Number, line.LineEnding = 1, ""
		result.Lines = []model.Line{line}
		count := 1
		result.Document.LineCount, result.Document.Metadata.Title = &count, nil
		v107TestParserError(t, result, 1, 0, "Отсутствует открытие обязательного блока.")
		v107TestRun(t, result, "current", []v107TestWant{{1, 1, 6, v107TestTextDetail}})
	})
	for _, unavailable := range []bool{false, true} {
		t.Run("empty unavailable="+strconv.FormatBool(unavailable), func(t *testing.T) {
			result := v107TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			count := 0
			result.Document.LineCount = &count
			if unavailable {
				result.Document.LineCount = nil
			}
			v107TestRun(t, result, "current", nil)
		})
	}
}

func TestV107CoexistsWithBraceBalance(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		balance bool
	}{
		{"@text Reading", false}, {"@text Reading }", true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			result := v107TestDocument(t, v107TestInvalid(t, tc.raw, 3))
			v107TestRun(t, result, "current", []v107TestWant{{4, 1, 6, v107TestTextDetail}})
			var balance []model.Diagnostic
			for _, entry := range result.Diagnostics {
				if entry.Source != "current" {
					continue
				}
				if entry.DiagnosticCode == "V094" {
					t.Fatal("V094 must not diagnose @text")
				}
				if entry.DiagnosticCode == "V005" {
					balance = append(balance, entry)
				}
			}
			if !tc.balance {
				if len(balance) != 0 {
					t.Fatalf("V005 duplicates missing opening: %+v", balance)
				}
				return
			}
			location := model.Location{Start: model.Position{Line: 4, Column: 15}, End: model.Position{Line: 4, Column: 16}}
			if len(balance) != 1 || balance[0].Location == nil || *balance[0].Location != location ||
				balance[0].Message != "Нарушено оформление фигурных скобок.\nконкретизация: Закрывающая скобка не имеет открывающей" ||
				balance[0].SeverityLevel != "error" || balance[0].DiagnosticScope != "line" || len(balance[0].RelatedLocations) != 0 {
				t.Fatalf("independent V005 balance error = %+v", balance)
			}
		})
	}
}

// v107TestInvalid сохраняет ошибочное объявление с целиком неразобранным Raw.
func v107TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v107TestLine(t, raw, "invalid", parent, v107TestElement(t, "unparsed", raw, "", 1))
}

// v107TestMissing задаёт частично разобранное ошибочное объявление: имя известно,
// остаток не разобран. Границы имени заданы явно, распознавания DSL здесь нет.
func v107TestMissing(t *testing.T, raw, name string, start, end, parent int) model.Line {
	t.Helper()
	runes := []rune(raw)
	return v107TestLine(t, raw, "invalid", parent,
		v107TestElement(t, "tag", string(runes[start-1:end-1]), name, start),
		v107TestElement(t, "unparsed", string(runes[end-1:]), "", end))
}

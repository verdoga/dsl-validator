package checkers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV062Registration(t *testing.T) {
	definitions, err := v062Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V062" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" ||
			definition.Message != "У @header отсутствует название." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V062 definitions = %d, want 1", count)
	}
}

func TestV062ReferenceAndExample(t *testing.T) {
	// DSL 1.30: перед неизменёнными фрагментами добавлены только
	// @dsl-version 1.2 и # Lesson; строки фрагментов получают номера 3 и 4.
	t.Run("reference", func(t *testing.T) {
		result := v062Lesson(t, v062Tag(t, "header", "Vocabulary", "title", 2, 1),
			v062Text(t, "Read the words.", "Read the words.", 2, 1))
		v062Run(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v062Lesson(t, v062Attempt(t, "@header", 1, 2, 1),
			v062Text(t, "Read the words.", "Read the words.", 2, 1))
		v062Run(t, result, "current", [][3]int{{3, 1, 8}})
	})
}

func TestV062RecognitionAndPositions(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		tagStart  int
		want      [][3]int
	}{
		{"bare name counted once", "@header", 1, [][3]int{{3, 1, 8}}},
		{"uppercase", "@HEADER", 1, [][3]int{{3, 1, 8}}},
		{"mixed case", "@HeAdEr", 1, [][3]int{{3, 1, 8}}},
		{"space and tab indentation", " \t@header", 3, [][3]int{{3, 3, 10}}},
		{"tab is one column", "\t@header", 2, [][3]int{{3, 2, 9}}},
		{"ordinary spaces", "@header   ", 1, [][3]int{{3, 1, 8}}},
		{"tab remainder", "@header\t", 1, [][3]int{{3, 1, 8}}},
		{"mixed remainder", "@header \t ", 1, [][3]int{{3, 1, 8}}},
	} {
		for _, recognized := range []bool{true, false} {
			t.Run(tc.name+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				line := v062Invalid(t, tc.raw, 2, 1)
				if recognized {
					line = v062Attempt(t, tc.raw, tc.tagStart, 2, 1)
				}
				v062Run(t, v062Lesson(t, line), "current", tc.want)
			})
		}
	}
	t.Run("recognized tag on invalid line", func(t *testing.T) {
		line := v062Attempt(t, "@header\t", 1, 2, 1)
		line.LineType = "invalid"
		v062Run(t, v062Lesson(t, line), "current", [][3]int{{3, 1, 8}})
	})
}

func TestV062UnicodeWhitespace(t *testing.T) {
	// Множество задано независимо от unicode.White_Space проверяемого кода.
	// CR/LF не входят в Raw по контракту; они проверяются как окончания строк.
	spaces := []rune{'\t', '\v', '\f', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'}
	tails := []string{string(spaces)}
	for _, space := range spaces {
		tails = append(tails, string(space))
	}
	for _, tail := range tails {
		for _, recognized := range []bool{true, false} {
			t.Run(strconv.Quote(tail)+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				raw := "@header" + tail
				line := v062Invalid(t, raw, 2, 1)
				if recognized {
					line = v062Attempt(t, raw, 1, 2, 1)
				}
				v062Run(t, v062Lesson(t, line), "current", [][3]int{{3, 1, 8}})
			})
		}
	}
}

func TestV062NonemptyArguments(t *testing.T) {
	for _, title := range []string{"Vocabulary", "Словарь", "VOCABULARY", "😀e\u0301", "\u200b", `""`, "."} {
		t.Run(strconv.Quote(title), func(t *testing.T) {
			v062Run(t, v062Lesson(t, v062Tag(t, "header", title, "title", 2, 1),
				v062Text(t, "Read.", "Read.", 2, 1)), "current", nil)
		})
	}
	t.Run("nonspace surrounded by whitespace", func(t *testing.T) {
		line := v062Line(t, "@header \t A \t", "tag", 2, 1,
			v062Element(t, "tag", "@header", "header", 1, 8),
			v062Element(t, "title", "A", "A", 11, 12))
		v062Run(t, v062Lesson(t, line), "current", nil)
	})
	for _, recognized := range []bool{true, false} {
		t.Run("unparsed argument/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
			// DSL 1.29: табуляция не заменяет разделитель параметра.
			// Название осталось в unparsed, но его непустота видна в Raw.
			line := v062Invalid(t, "@header\tVocabulary", 2, 1)
			if recognized {
				line = v062Attempt(t, "@header\tVocabulary", 1, 2, 1)
			}
			v062Run(t, v062Lesson(t, line), "current", nil)
		})
		t.Run("unsupported block form/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
			// DSL 2.2.3: header не имеет блочной формы. Ошибочная попытка
			// и её закрытие сохранены неразобранными; { уже непустой аргумент.
			line := v062Invalid(t, "@header {", 2, 1)
			if recognized {
				line = v062Attempt(t, "@header {", 1, 2, 1)
			}
			v062Run(t, v062Lesson(t, line, v062Text(t, "Read.", "Read.", 2, 1),
				v062Invalid(t, "}", 2, 1)), "current", nil)
		})
	}
}

func TestV062ExcludedNamesAndPrefixes(t *testing.T) {
	for _, raw := range []string{"@headerExtra", "@headers", "@header1", "@header{"} {
		t.Run(raw, func(t *testing.T) {
			v062Run(t, v062Lesson(t, v062Invalid(t, raw, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@header`, "@header"}, {" \t\\@header", "@header"}, {`\\@header`, `\@header`},
		{"Text @header", "Text @header"}, {"\u00a0@header", "\u00a0@header"},
		{" \t\u2003@header", "\u2003@header"},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v062Run(t, v062Lesson(t, v062Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, raw := range []string{"", " ", "\t", " \t "} {
		t.Run("blank "+strconv.Quote(raw), func(t *testing.T) {
			v062Run(t, v062Lesson(t, v062Line(t, raw, "blank", 2, 1)), "current", nil)
		})
	}
	t.Run("other tag", func(t *testing.T) {
		v062Run(t, v062Lesson(t, v062Tag(t, "editor", "Comment", "content", 2, 1)), "current", nil)
	})
	t.Run("no header", func(t *testing.T) {
		v062Run(t, v062Lesson(t, v062Text(t, "Read.", "Read.", 2, 1)), "current", nil)
	})
}

func TestV062MisplacedAndNested(t *testing.T) {
	t.Run("before first level heading", func(t *testing.T) {
		result := v062Document(t, v062Attempt(t, "@header", 1, 0, 0),
			v062Heading(t), v062Text(t, "Read.", "Read.", 3, 1))
		v062Run(t, result, "current", [][3]int{{2, 1, 8}})
	})
	for _, recognized := range []bool{true, false} {
		t.Run("text block/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
			line := v062Invalid(t, "@header", 3, 2)
			if recognized {
				line = v062Attempt(t, "@header", 1, 3, 2)
				line.LineType = "invalid"
			}
			result := v062Lesson(t, v062Open(t, "text", 2, 1), line,
				v062Text(t, "Read.", "Read.", 3, 2), v062Close(t, 3, 2))
			v062Run(t, result, "current", [][3]int{{4, 1, 8}})
		})
	}
	t.Run("text block inside task", func(t *testing.T) {
		result := v062Lesson(t, v062Tag(t, "task", "a", "identifier", 2, 1),
			v062Tag(t, "instr", "Read.", "content", 3, 2),
			v062Open(t, "text", 3, 2), v062Invalid(t, "@header", 5, 3),
			v062Text(t, "Read.", "Read.", 5, 3), v062Close(t, 5, 3),
			v062Tag(t, "endtask", "", "", 2, 1))
		v062Run(t, result, "current", [][3]int{{6, 1, 8}})
	})
}

func TestV062LiteralContexts(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "EDITOR", "MULTIFILL"} {
		t.Run(tag, func(t *testing.T) {
			t.Run("contents", func(t *testing.T) {
				v062Run(t, v062Lesson(t, v062Open(t, tag, 2, 1),
					v062Text(t, "@header", "@header", 3, 2), v062Close(t, 3, 2)), "current", nil)
			})
			t.Run("after closing", func(t *testing.T) {
				result := v062Lesson(t, v062Open(t, tag, 2, 1),
					v062Text(t, "@header", "@header", 3, 2), v062Close(t, 3, 2),
					v062Attempt(t, "@header", 1, 2, 1))
				v062Run(t, result, "current", [][3]int{{6, 1, 8}})
			})
			t.Run("inside task", func(t *testing.T) {
				result := v062Lesson(t, v062Tag(t, "task", "a", "identifier", 2, 1),
					v062Tag(t, "instr", "Read.", "content", 3, 2),
					v062Open(t, tag, 3, 2), v062Text(t, "@header", "@header", 5, 3), v062Close(t, 5, 3),
					v062Tag(t, "endtask", "", "", 2, 1))
				v062Run(t, result, "current", nil)
			})
			t.Run("single line form", func(t *testing.T) {
				result := v062Lesson(t, v062Tag(t, tag, "Write.", "content", 2, 1),
					v062Attempt(t, "@header", 1, 2, 1))
				v062Run(t, result, "current", [][3]int{{4, 1, 8}})
			})
		})
	}
}

func TestV062SeveralAndIndependentRuns(t *testing.T) {
	t.Run("several declarations", func(t *testing.T) {
		result := v062Lesson(t, v062Attempt(t, "@header", 1, 2, 1),
			v062Tag(t, "header", "Vocabulary", "title", 2, 1), v062Invalid(t, "@header", 2, 1))
		v062Run(t, result, "current", [][3]int{{3, 1, 8}, {5, 1, 8}})
	})
	t.Run("independent and repeated", func(t *testing.T) {
		result := v062Lesson(t, v062Attempt(t, "@header", 1, 2, 1))
		v062Run(t, result, "first", [][3]int{{3, 1, 8}})
		v062Run(t, v062Lesson(t, v062Tag(t, "header", "Vocabulary", "title", 2, 1),
			v062Text(t, "Read the words.", "Read the words.", 2, 1)), "independent", nil)
		v062Run(t, result, "second", [][3]int{{3, 1, 8}})
	})
}

func TestV062EndingsAndVersions(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		for _, recognized := range []bool{true, false} {
			t.Run(strconv.Quote(string(ending))+"/tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				line := v062Invalid(t, "@header", 2, 1)
				if recognized {
					line = v062Attempt(t, "@header", 1, 2, 1)
				}
				result := v062Lesson(t, line)
				if ending == "\r\n" {
					for i := range result.Lines {
						result.Lines[i].LineEnding = ending
					}
				}
				result.Lines[2].LineEnding = ending
				v062Run(t, result, "current", [][3]int{{3, 1, 8}})
			})
		}
	}
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version+" with unavailable lines", func(t *testing.T) {
			// Версия известна, физические строки недоступны.
			result := v062Lesson(t)
			result.Document.DSLVersion, result.Document.LineCount = &version, nil
			result.Document.Metadata.Title = nil
			result.Lines = []model.Line{}
			if version == "1.2" {
				v062Run(t, result, "current", nil)
				return
			}
			before := v062Snapshot(t, result)
			checks, err := validator.Prepare(result, v062Registry(t))
			if err == nil || checks != nil || v062Snapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v062Lesson добавляет обязательный заголовок; body начинается со строки 3.
func v062Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	return v062Document(t, append([]model.Line{v062Heading(t)}, body...)...)
}

// v062Document добавляет версию перед готовыми строками, содержащими # Lesson.
// Для явно заданных unparsed создаёт согласованные исходные ошибки и ссылки.
func v062Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v062Tag(t, "dsl-version", "1.2", "version", 0, 0)}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines: lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{},
	}
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
				ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора объявления.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v062Attempt задаёт распознанное имя header на указанной Unicode-позиции
// и ошибочный остаток целиком в unparsed, включая допустимый пустой диапазон.
func v062Attempt(t *testing.T, raw string, start, parent, depth int) model.Line {
	t.Helper()
	runes := []rune(raw)
	end := start + 7
	return v062Line(t, raw, "tag", parent, depth,
		v062Element(t, "tag", string(runes[start-1:end-1]), "header", start, end),
		v062Element(t, "unparsed", string(runes[end-1:]), "", end, len(runes)+1))
}

// v062Heading задаёт обязательный заголовок первого уровня.
func v062Heading(t *testing.T) model.Line {
	t.Helper()
	return v062Line(t, "# Lesson", "heading", 0, 0,
		v062Element(t, "heading-level", "#", "1", 1, 2),
		v062Element(t, "title", "Lesson", "Lesson", 3, 9))
}

// v062Invalid задаёт целиком неразобранную строку, без элемента tag.
func v062Invalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v062Line(t, raw, "invalid", parent, depth, v062Element(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1))
}

// v062Text задаёт Raw и семантическое значение независимо.
func v062Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v062Line(t, raw, "content", parent, depth, v062Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v062Tag задаёт известный однострочный тег с необязательным аргументом.
func v062Tag(t *testing.T, tag, argument string, kind model.ElementType, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v062Element(t, "tag", raw, strings.ToLower(tag), 1, len(raw)+1)}
	if argument != "" {
		elements = append(elements, v062Element(t, kind, argument, argument, len(raw)+2, len(raw)+2+utf8.RuneCountInString(argument)))
		raw += " " + argument
	}
	return v062Line(t, raw, "tag", parent, depth, elements...)
}

// v062Open задаёт блок с каноническим семантическим именем тега.
func v062Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v062Line(t, "@"+tag+" {", "block-start", parent, depth,
		v062Element(t, "tag", "@"+tag, strings.ToLower(tag), 1, len(tag)+2),
		v062Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v062Close задаёт закрытие блока с явным родителем.
func v062Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v062Line(t, "}", "block-end", parent, depth, v062Element(t, "block-close", "}", "}", 1, 2))
}

// v062Line сохраняет явно заданную структуру без классификации Raw.
func v062Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v062Element задаёт исходные Unicode-колонки с исключающим концом.
func v062Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v062Registry создаёт реестр из полного штатного каталога.
func v062Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v062Run проверяет новые V062 по явным тройкам строка/начало/исключающий конец.
// Конвейер выполняет полный каталог; результаты других диагностик не отбираются.
func v062Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v062Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v062Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v062Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V062" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V062 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]}, End: model.Position{Line: position[0], Column: position[2]}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "У @header отсутствует название." {
			t.Fatalf("occurrence %d = %+v; want %+v and exact V062 metadata", i, entry, location)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v062Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// CheckModel проверил производные флаги/ссылки. В копии восстанавливаем
	// только их и убираем новые записи для сравнения всего исходника и истории.
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
	if v062Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v062Snapshot сохраняет независимое представление модели для сравнения.
func v062Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

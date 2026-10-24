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

const (
	v066Message  = "После имени тега есть запрещённое содержимое."
	v066Newpage  = "Тег @newpage записывается без параметров и блочного содержимого."
	v066Speaking = "Тег @speaking записывается без параметров и блочного содержимого."
	v066Endtask  = "Тег @endtask записывается без параметров и блочного содержимого."
)

// v066Want задаёт независимые исходные координаты с исключающим концом.
type v066Want struct {
	line, start, end int
	detail           string
}

func TestV066RegistrationAndVersions(t *testing.T) {
	registry := v066Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V066" {
			continue
		}
		count++
		if definition.Message != v066Message || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V066 registrations = %d, want 1", count)
	}
	// ForVersion отвергает неподдерживаемую версию до выбора диагностик.
	for _, version := range []string{"1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if definitions, err := registry.ForVersion(dslversions.Version(version)); err == nil || definitions != nil {
				t.Fatalf("unsupported version: definitions=%v, err=%v", definitions, err)
			}
			result := v066Document(t)
			result.Document.DSLVersion = &version
			before := v066Snapshot(t, result)
			if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v066Snapshot(t, result) != before {
				t.Fatal("failed Prepare changed the model")
			}
		})
	}
}

func TestV066ReferenceAndThreeTags(t *testing.T) {
	// Эталон и пример @newpage/Read the text. сохранены дословно;
	// перед ними добавлены только версия и основной заголовок (строки 1–2).
	// Для speaking/endtask добавлено предшествующее задание по DSL 2.4/2.6.
	for _, tc := range []struct {
		tag, tail string
		want      v066Want
	}{
		{"newpage", "Practice", v066Want{3, 10, 18, v066Newpage}},
		{"speaking", "oral", v066Want{4, 11, 15, v066Speaking}},
		{"endtask", "practice", v066Want{5, 10, 18, v066Endtask}},
	} {
		for _, ending := range []string{"", " ", "\t", " \t "} {
			t.Run(tc.tag+" allowed "+strconv.Quote(ending), func(t *testing.T) {
				line := v066TestDeclaration(t, "", tc.tag, "", "", ending, true)
				v066Run(t, v066Context(t, tc.tag, line, true), "current", nil)
			})
		}
		for _, parsed := range []bool{true, false} {
			t.Run(tc.tag+" forbidden parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v066TestDeclaration(t, "", tc.tag, " ", tc.tail, "", parsed)
				v066Run(t, v066Context(t, tc.tag, line, false), "current", []v066Want{tc.want})
			})
		}
	}
}

func TestV066TailRanges(t *testing.T) {
	for _, tc := range []struct {
		name, indent, tag, gap, tail, trailing string
		want                                   v066Want
	}{
		{"indent and case", " \t", "NeWpAgE", " \t", "Practice", " \t", v066Want{3, 13, 21, v066Newpage}},
		{"multiple words", "", "newpage", " ", "one  two", "", v066Want{3, 10, 18, v066Newpage}},
		{"tabs around tail", "", "newpage", "\t\t", "Practice", "\t", v066Want{3, 11, 19, v066Newpage}},
		{"mixed trailing spaces", "", "newpage", " ", "Practice", " \t ", v066Want{3, 10, 18, v066Newpage}},
		{"Unicode", "", "newpage", " ", "😀Я", "", v066Want{3, 10, 12, v066Newpage}},
		{"combining characters", "", "newpage", " ", "e\u0301", "", v066Want{3, 10, 12, v066Newpage}},
		{"NBSP between ordinary edges", "", "newpage", " \t", "\u00a0", " \t", v066Want{3, 11, 12, v066Newpage}},
		{"NBSP edges stay", "", "newpage", " ", "\u00a0X\u00a0", "\t", v066Want{3, 10, 13, v066Newpage}},
		{"internal tab stays", "", "newpage", " ", "one\t two", "", v066Want{3, 10, 18, v066Newpage}},
		{"escape is not decoded", "", "newpage", " ", "\\{x\\}", "", v066Want{3, 10, 15, v066Newpage}},
		{"zero width is forbidden tail", "", "newpage", " ", "\u200b", "", v066Want{3, 10, 11, v066Newpage}},
		{"several inline constructs", "", "newpage", " ", "one @speaking oral", "", v066Want{3, 10, 28, v066Newpage}},
	} {
		for _, parsed := range []bool{true, false} {
			t.Run(tc.name+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v066TestDeclaration(t, tc.indent, tc.tag, tc.gap, tc.tail, tc.trailing, parsed)
				v066Run(t, v066Context(t, "newpage", line, false), "current", []v066Want{tc.want})
			})
		}
	}
}

func TestV066UnicodeWhitespaceBoundaries(t *testing.T) {
	// Явное множество White_Space без CR/LF: внутри Raw они запрещены
	// контрактом модели. U+0020/U+0009 отдельно проверяются как допустимые края.
	for _, space := range []string{"\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		for _, parsed := range []bool{true, false} {
			t.Run(strconv.Quote(space)+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v066TestDeclaration(t, "", "newpage", "", space, "", parsed)
				v066Run(t, v066Context(t, "newpage", line, false), "current",
					[]v066Want{{3, 9, 10, v066Newpage}})
			})
		}
	}
}

func TestV066NonDeclarations(t *testing.T) {
	for _, raw := range []string{"@newpageExtra", "@newpage!", "@newpage\u200b", "@new",
		"@speakingExtra oral", "@endtaskExtra practice", "@endtask.practice"} {
		t.Run(raw, func(t *testing.T) {
			v066Run(t, v066Document(t, v066Invalid(t, raw, 2), v066Text(t, "Read.", "Read.", 2)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{"Text @newpage Practice", "Text @newpage Practice"},
		{"\\@newpage Practice", "@newpage Practice"},
		{" \t\\@speaking oral", "@speaking oral"},
		{"\\@endtask practice", "@endtask practice"},
		{"\u00a0@newpage Practice", "\u00a0@newpage Practice"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v066Run(t, v066Document(t, v066Text(t, tc.raw, tc.value, 2)), "current", nil)
		})
	}
	for _, tag := range []string{"header", "editor", "instr"} {
		t.Run("other tag "+tag, func(t *testing.T) {
			kind := model.ElementTypeContent
			if tag == "header" {
				kind = model.ElementTypeTitle
			}
			v066Run(t, v066Document(t, v066Tag(t, tag, "@newpage Practice", kind, 2),
				v066Text(t, "Read.", "Read.", 2)), "current", nil)
		})
	}
	v066Run(t, v066Document(t, v066Line(t, "", "blank", 2)), "current", nil)
}

func TestV066Braces(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want v066Want
	}{
		{"@newpage{", v066Want{3, 9, 10, v066Newpage}},
		{"@newpage}", v066Want{3, 9, 10, v066Newpage}},
		{"@newpage{}", v066Want{3, 9, 11, v066Newpage}},
		{"@newpage { words } \t", v066Want{3, 10, 19, v066Newpage}},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			v066Run(t, v066Document(t, v066Invalid(t, tc.raw, 2),
				v066Text(t, "Read.", "Read.", 2)), "current", []v066Want{tc.want})
		})
	}
	t.Run("recognized forbidden block", func(t *testing.T) {
		result := v066Document(t, v066Open(t, "newpage", 2),
			v066Text(t, "Practice", "Practice", 3), v066Close(t, 3))
		v066ParserError(t, result, 3, 1, "Тег newpage не принимает блочное содержимое.")
		v066Run(t, result, "current", []v066Want{{3, 10, 11, v066Newpage}})
	})
	t.Run("unclosed forbidden block", func(t *testing.T) {
		result := v066Document(t, v066Open(t, "newpage", 2),
			v066Text(t, "Practice", "Practice", 3))
		v066ParserError(t, result, 3, 1, "Запрещённый незакрытый блок newpage.")
		v066Run(t, result, "current", []v066Want{{3, 10, 11, v066Newpage}})
	})
}

func TestV066LiteralAndNestedContext(t *testing.T) {
	for _, tag := range []string{"editor", "EDITOR", "multifill", "MULTIFILL"} {
		t.Run("literal "+tag, func(t *testing.T) {
			v066Run(t, v066Document(t, v066Open(t, tag, 2),
				v066Text(t, "@newpage Practice", "@newpage Practice", 3),
				v066Text(t, "@speaking oral", "@speaking oral", 3),
				v066Text(t, "@endtask practice", "@endtask practice", 3), v066Close(t, 3)), "current", nil)
		})
		t.Run("literal in task "+tag, func(t *testing.T) {
			v066Run(t, v066Document(t, v066Tag(t, "task", "practice", "identifier", 2),
				v066Tag(t, "instr", "Read.", "content", 3), v066Open(t, tag, 3),
				v066Text(t, "@newpage Practice", "@newpage Practice", 5), v066Close(t, 5)), "current", nil)
		})
	}
	t.Run("after literal block", func(t *testing.T) {
		line := v066TestDeclaration(t, "", "newpage", " ", "Practice", "", true)
		parent := 2
		line.ParentLine = &parent
		v066Run(t, v066Document(t, v066Open(t, "editor", 2),
			v066Text(t, "@newpage Practice", "@newpage Practice", 3), v066Close(t, 3),
			line, v066Text(t, "Read.", "Read.", 2)), "current", []v066Want{{6, 10, 18, v066Newpage}})
	})
	for _, tc := range []struct {
		tag, tail string
		want      v066Want
	}{
		{"newpage", "Practice", v066Want{4, 10, 18, v066Newpage}},
		{"speaking", "oral", v066Want{4, 11, 15, v066Speaking}},
		{"endtask", "practice", v066Want{4, 10, 18, v066Endtask}},
	} {
		for _, parsed := range []bool{true, false} {
			t.Run("nested "+tc.tag+" parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
				line := v066TestDeclaration(t, "", tc.tag, " ", tc.tail, "", parsed)
				parent := 3
				line.ParentLine = &parent
				v066Run(t, v066Document(t, v066Open(t, "text", 2), line, v066Close(t, 3)),
					"current", []v066Want{tc.want})
			})
		}
	}
	t.Run("block inside task", func(t *testing.T) {
		// Полное окружение task -> text -> speaking сохраняет вложенное объявление.
		line := v066TestDeclaration(t, "", "speaking", " ", "oral", "", true)
		parent := 4
		line.ParentLine = &parent
		result := v066Document(t, v066Tag(t, "task", "practice", "identifier", 2),
			v066Open(t, "text", 3), line,
			v066Text(t, "Read.", "Read.", 4), v066Close(t, 4))
		v066Run(t, result, "current", []v066Want{{5, 11, 15, v066Speaking}})
	})
	t.Run("fallback with no tail in invalid nested declaration", func(t *testing.T) {
		// Ошибка здесь — вложение; V066 не диагностирует отсутствие хвоста.
		v066Run(t, v066Document(t, v066Open(t, "text", 2),
			v066Invalid(t, "@newpage", 3), v066Invalid(t, "@speaking \t", 3), v066Close(t, 3)), "current", nil)
	})
	t.Run("inline editor does not hide following declaration", func(t *testing.T) {
		v066Run(t, v066Document(t, v066Tag(t, "editor", "Comment.", "content", 2),
			v066Invalid(t, "@newpage Practice", 2), v066Text(t, "Read.", "Read.", 2)),
			"current", []v066Want{{4, 10, 18, v066Newpage}})
	})
}

func TestV066MultipleEndingsAndEmpty(t *testing.T) {
	result := v066Document(t, v066Invalid(t, "@newpage Practice", 2), v066Text(t, "Read.", "Read.", 2),
		v066Invalid(t, "@newpage Practice", 2), v066Text(t, "Write.", "Write.", 2))
	v066Run(t, result, "current", []v066Want{{3, 10, 18, v066Newpage}, {5, 10, 18, v066Newpage}})
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		t.Run(strconv.Quote(string(eol)), func(t *testing.T) {
			result := v066Context(t, "newpage", v066TestDeclaration(t, "", "newpage", " ", "Practice", "", true), false)
			for i := range result.Lines {
				result.Lines[i].LineEnding = eol
			}
			v066Run(t, result, "first", []v066Want{{3, 10, 18, v066Newpage}})
			v066Run(t, result, "second", []v066Want{{3, 10, 18, v066Newpage}})
		})
	}
	t.Run("last line without EOL", func(t *testing.T) {
		result := v066Context(t, "endtask", v066TestDeclaration(t, "", "endtask", " ", "practice", "", true), false)
		result.Lines[4].LineEnding = ""
		v066Run(t, result, "current", []v066Want{{5, 10, 18, v066Endtask}})
	})
	t.Run("empty input", func(t *testing.T) {
		result := v066Document(t)
		result.Lines = []model.Line{}
		count := 0
		result.Document.LineCount = &count
		result.Document.Metadata.Title = nil
		v066Run(t, result, "current", nil)
	})
}

// v066Line строит строку с явно указанными элементами и родителем, без разбора.
func v066Line(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v066Element вычисляет только координаты входных элементов, не ожидания.
func v066Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	element := model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
	if kind == "unparsed" {
		element.Value = nil
	}
	return element
}

// v066TestDeclaration собирает заданные части объявления без анализа хвоста.
func v066TestDeclaration(t *testing.T, indent, tag, gap, tail, trailing string, parsed bool) model.Line {
	t.Helper()
	raw := indent + "@" + tag + gap + tail + trailing
	if !parsed {
		return v066Invalid(t, raw, 0)
	}
	start := utf8.RuneCountInString(indent) + 1
	elements := []model.Element{v066Element(t, "tag", "@"+tag, strings.ToLower(tag), start)}
	if tail != "" {
		elements = append(elements, v066Element(t, "unparsed", tail, "",
			start+1+utf8.RuneCountInString(tag+gap)))
	}
	return v066Line(t, raw, "tag", 0, elements...)
}

// v066Invalid представляет ошибочную конструкцию целиком неразобранной.
func v066Invalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	return v066Line(t, raw, "invalid", parent, v066Element(t, "unparsed", raw, "", 1))
}

// v066Text сохраняет исходное и семантическое содержимое независимо.
func v066Text(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v066Line(t, raw, "content", parent, v066Element(t, "content", raw, value, 1))
}

// v066Tag строит известный тег с явно заданным типом параметра.
func v066Tag(t *testing.T, tag, parameter string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v066Element(t, "tag", raw, strings.ToLower(tag), 1)}
	if parameter != "" {
		elements = append(elements, v066Element(t, kind, parameter, parameter, len(raw)+2))
		raw += " " + parameter
	}
	return v066Line(t, raw, "tag", parent, elements...)
}

// v066Open строит объявление блока; вызывающий задаёт ошибки запрещённого блока.
func v066Open(t *testing.T, tag string, parent int) model.Line {
	t.Helper()
	line := v066Tag(t, tag, "", "", parent)
	line.Elements = append(line.Elements, v066Element(t, "block-open", "{", "{", len(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v066Close связывает закрывающую строку с открытием блока.
func v066Close(t *testing.T, parent int) model.Line {
	t.Helper()
	return v066Line(t, "}", "block-end", parent, v066Element(t, "block-close", "}", "}", 1))
}

// v066Context добавляет необходимое окружение конкретного тега.
// validNewpage указывает реальную границу шага; ошибочное объявление её не создаёт.
func v066Context(t *testing.T, tag string, line model.Line, validNewpage bool) *model.Result {
	t.Helper()
	parent := 3
	switch tag {
	case "speaking":
		line.ParentLine = &parent
		return v066Document(t, v066Tag(t, "task", "practice", "identifier", 2), line,
			v066Tag(t, "instr", "Speak.", "content", 3), v066Text(t, "Read.", "Read.", 3))
	case "endtask":
		line.ParentLine = &parent
		return v066Document(t, v066Tag(t, "task", "practice", "identifier", 2),
			v066Text(t, "Read.", "Read.", 3), line)
	default:
		if !validNewpage {
			parent = 2
			line.ParentLine = &parent
		}
		return v066Document(t, line, v066Text(t, "Read the text.", "Read the text.", parent))
	}
}

// v066Document добавляет версию и основной заголовок; глубины следуют родителям.
// unparsed получает исходную диагностику парсера, а не выдуманное Value.
func v066Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{
		v066Tag(t, "dsl-version", "1.2", "version", 0),
		v066Line(t, "# Lesson", "heading", 0, v066Element(t, "heading-level", "#", "1", 1),
			v066Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	version, title, count := "1.2", "Lesson", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid fixture parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		for j, element := range line.Elements {
			if element.ElementType == "unparsed" {
				v066ParserError(t, result, line.Number, j, "Ошибка разбора конструкции.")
			}
		}
	}
	return result
}

// v066ParserError связывает существующий элемент с исходной ошибкой.
func v066ParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: element.Start},
			End: model.Position{Line: number, Column: element.End}},
	})
}

// v066Registry использует полный штатный каталог без подмены регистраций.
func v066Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v066Run проверяет новые V066 через Prepare/Run отдельно от прочих диагностик.
func v066Run(t *testing.T, result *model.Result, processingID string, want []v066Want) {
	t.Helper()
	before := v066Snapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v066Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v066Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V066" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V066 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start},
			End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != v066Message+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Source != processingID || entry.ID == "" ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("V066 occurrence %d = %+v; want %+v, detail %q, no related", i, entry, location, expected.detail)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v066Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Убираются только допустимые дополнения конвейера и производные флаги.
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
	if v066Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v066Snapshot сохраняет точное представление модели для проверки неизменности.
func v066Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

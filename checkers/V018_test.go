package checkers

import (
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV018Registration(t *testing.T) {
	definitions, err := v018Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V018" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "После знака препинания отсутствует пробел." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V018 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V018 definitions = %d, want 1", matches)
	}
}

func TestV018Text(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want [][3]int
	}{
		{"reference", "Hello, world! Value: 3.14.", nil},
		{"example", "Hello,world!", [][3]int{{3, 6, 7}}},
		{"no punctuation", "Hello world", nil},
		{"first character", ",А", [][3]int{{3, 1, 2}}},
		{"multiple occurrences", "А,Б;В", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"separate terminal signs", "Что?!Да", [][3]int{{3, 4, 5}, {3, 5, 6}}},
		{"three dots before text", "Wait...Next", [][3]int{{3, 5, 6}, {3, 6, 7}, {3, 7, 8}}},
		{"three dots at end", "Wait...", [][3]int{{3, 5, 6}, {3, 6, 7}}},
		{"Unicode ellipsis is not checked", "А…Б", nil},
		{"ASCII period between digits", "3.14", nil},
		{"ASCII comma between digits", "3,14", nil},
		{"ASCII time", "12:30", nil},
		{"Unicode period between digits", "٣.١٤", nil},
		{"Unicode comma between digits", "٣,١٤", nil},
		{"Unicode time", "١٢:٣٠", nil},
		{"mixed Unicode digits", "3.١٤", nil},
		{"digit only on right", "А.14", [][3]int{{3, 2, 3}}},
		{"digit only on left", "3.А", [][3]int{{3, 2, 3}}},
		{"time with nondigit on right", "12:ab", [][3]int{{3, 3, 4}}},
		{"comma with nondigit on left", "А,14", [][3]int{{3, 2, 3}}},
		{"question mark between digits", "1?2", [][3]int{{3, 2, 3}}},
		{"exclamation between digits", "1!2", [][3]int{{3, 2, 3}}},
		{"semicolon between digits", "1;2", [][3]int{{3, 2, 3}}},
		{"final period after decimal", "3.14.", nil},
		{"Unicode columns", "Я😀,Б", [][3]int{{3, 3, 4}}},
		{"combining mark is a column", "e\u0301,Б", [][3]int{{3, 3, 4}}},
		{"closing quote is not skipped", "«Hi!» There", [][3]int{{3, 4, 5}}},
		{"closing bracket is not skipped", "(А.) Б", [][3]int{{3, 3, 4}}},
		{"opening quote after punctuation", `Hi,"there"`, [][3]int{{3, 3, 4}}},
		{"abbreviations are not recognized", "e.g.Next", [][3]int{{3, 2, 3}, {3, 4, 5}}},
		{"URL is not recognized", "https://x.y", [][3]int{{3, 6, 7}, {3, 10, 11}}},
		{"email is not recognized", "a@b.cd", [][3]int{{3, 4, 5}}},
		{"slashes do not hide punctuation", "/a,b/", [][3]int{{3, 3, 4}}},
		{"square brackets do not hide punctuation", "[ʃ,a]", [][3]int{{3, 3, 4}}},
		{"upper case", "А,Б", [][3]int{{3, 2, 3}}},
		{"lower case", "а,б", [][3]int{{3, 2, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v018Run(t, v018Document(t, v018Text(t, tc.raw, 2, 1)), "current", tc.want)
		})
	}
}

func TestV018PunctuationBoundaries(t *testing.T) {
	for _, sign := range []string{"?", ".", "!", ",", ":", ";"} {
		t.Run(sign, func(t *testing.T) {
			for _, tc := range []struct {
				name, raw string
				want      [][3]int
			}{
				{"followed by text", "А" + sign + "Б", [][3]int{{3, 2, 3}}},
				{"followed by space", "А" + sign + " Б", nil},
				{"at end of line", "А" + sign, nil},
				{"only character", sign, nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					v018Run(t, v018Document(t, v018Text(t, tc.raw, 2, 1)), "current", tc.want)
				})
			}
		})
	}
}

func TestV018Whitespace(t *testing.T) {
	// Входы заданы явно, без обращения к классификатору проверяемой диагностики.
	for _, space := range []rune{
		'\t', '\u0020', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004',
		'\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u202f', '\u205f', '\u3000',
	} {
		t.Run(fmt.Sprintf("horizontal U+%04X", space), func(t *testing.T) {
			v018Run(t, v018Document(t, v018Text(t, "А,"+string(space)+"Б", 2, 1)), "current", nil)
		})
	}
	// LF и CR не входят в Raw; их влияние проверяется через физические строки и EOL.
	for _, other := range []rune{'\v', '\f', '\u0085', '\u2028', '\u2029', '\u200b'} {
		t.Run(fmt.Sprintf("not horizontal U+%04X", other), func(t *testing.T) {
			v018Run(t, v018Document(t, v018Text(t, "А,"+string(other)+"Б", 2, 1)), "current", [][3]int{{3, 2, 3}})
		})
	}
}

func TestV018RawAndStructure(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"empty physical line", []model.Line{v018Line(t, "", "blank", 2, 1)}, nil},
		{"blank whitespace", []model.Line{v018Line(t, " \t ", "blank", 2, 1)}, nil},
		// DSL 1.14: пробелы по краям отсутствуют в элементе, но сохраняются в Raw.
		{"original columns", []model.Line{v018Line(t, "  А,Б ", "content", 2, 1,
			v018Element(t, "content", "А,Б", "А,Б", 3, 6))}, [][3]int{{3, 4, 5}}},
		{"space outside element", []model.Line{v018Line(t, "А, ", "content", 2, 1,
			v018Element(t, "content", "А,", "А,", 1, 3))}, nil},
		// DSL 1.21–1.22: Value снимает экранирование фигурных скобок; Raw — нет.
		{"raw differs from semantic value", []model.Line{v018Line(t, `\{А,Б\}`, "content", 2, 1,
			v018Element(t, "content", `\{А,Б\}`, "{А,Б}", 1, 8))}, [][3]int{{3, 4, 5}}},
		{"line boundary", []model.Line{v018Text(t, "А,", 2, 1), v018Text(t, "Б", 2, 1)}, nil},
		{"single-line comment", []model.Line{v018Line(t, "@editor А,Б", "tag", 2, 1,
			v018Element(t, "tag", "@editor", "editor", 1, 8), v018Element(t, "content", "А,Б", "А,Б", 9, 12))},
			[][3]int{{3, 10, 11}}},
		// DSL 3.8.6, 3.8.11: инструкция имеет собственный текст и вложенный hint.
		{"nested hint", []model.Line{
			v018Line(t, "@instr {", "block-start", 2, 1,
				v018Element(t, "tag", "@instr", "instr", 1, 7), v018Element(t, "block-open", "{", "{", 8, 9)),
			v018Text(t, "Read.", 3, 2),
			v018Line(t, "@hint {", "block-start", 3, 2,
				v018Element(t, "tag", "@hint", "hint", 1, 6), v018Element(t, "block-open", "{", "{", 7, 8)),
			v018Text(t, "А,Б", 5, 3),
			v018Line(t, "}", "block-end", 5, 3, v018Element(t, "block-close", "}", "}", 1, 2)),
			v018Line(t, "}", "block-end", 3, 2, v018Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{6, 2, 3}}},
		{"HTML is not removed", []model.Line{
			v018Line(t, "@text {", "block-start", 2, 1,
				v018Element(t, "tag", "@text", "text", 1, 6), v018Element(t, "block-open", "{", "{", 7, 8)),
			v018Text(t, "А,<em>Б</em>", 3, 2),
			v018Line(t, "}", "block-end", 3, 2, v018Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 2, 3}}},
		// DSL 3.4.14–3.4.17: таблица без заголовочной части и разделителя допустима.
		// Конец элемента ячейки не равен концу физической строки: после запятой стоит |.
		{"next character outside element", []model.Line{
			v018Line(t, "@table {", "block-start", 2, 1,
				v018Element(t, "tag", "@table", "table", 1, 7), v018Element(t, "block-open", "{", "{", 8, 9)),
			v018Line(t, "А,|Б", "content", 3, 2,
				v018Element(t, "content", "А,", "А,", 1, 3), v018Element(t, "content", "Б", "Б", 4, 5)),
			v018Line(t, "}", "block-end", 3, 2, v018Element(t, "block-close", "}", "}", 1, 2)),
		}, [][3]int{{4, 2, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v018Run(t, v018Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV018TitleAndName(t *testing.T) {
	t.Run("heading title", func(t *testing.T) {
		result := v018Document(t)
		title := "А,Б"
		result.Document.Metadata.Title = &title
		result.Lines[1] = v018Line(t, "# А,Б", "heading", 0, 0,
			v018Element(t, "heading-level", "#", "1", 1, 2), v018Element(t, "title", title, title, 3, 6))
		result.Lines[1].Number = 2
		v018Run(t, result, "current", [][3]int{{2, 4, 5}})
	})
	t.Run("section name", func(t *testing.T) {
		result := v018Document(t)
		section := "А,Б"
		result.Document.Metadata.Section = &section
		line := v018Line(t, "@section А,Б", "tag", 0, 0,
			v018Element(t, "tag", "@section", "section", 1, 9), v018Element(t, "name", section, section, 10, 13))
		v018InsertMetadata(t, result, line)
		v018Run(t, result, "current", [][3]int{{2, 11, 12}})
	})
}

func TestV018ExcludedFields(t *testing.T) {
	t.Run("service version", func(t *testing.T) {
		v018Run(t, v018Document(t), "current", nil)
	})
	t.Run("identifier", func(t *testing.T) {
		v018Run(t, v018Document(t, v018Line(t, "@task a.b", "tag", 2, 1,
			v018Element(t, "tag", "@task", "task", 1, 6), v018Element(t, "identifier", "a.b", "a.b", 7, 10))), "current", nil)
	})
	t.Run("resource path", func(t *testing.T) {
		result := v018Document(t)
		result.Document.Metadata.ResourceDirs = []string{"a,b"}
		v018InsertMetadata(t, result, v018Line(t, `@resource-dir "a,b"`, "tag", 0, 0,
			v018Element(t, "tag", "@resource-dir", "resource-dir", 1, 14),
			v018Element(t, "resource-path", `"a,b"`, "a,b", 15, 20)))
		v018Run(t, result, "current", nil)
	})
	t.Run("unparsed", func(t *testing.T) {
		// DSL 1.16: неизвестное имя тега ошибочно; сохраняется согласованная
		// история парсера с непрозрачным тестовым кодом, а не выдуманный реальный код.
		line := v018Line(t, "@unknown А,Б", "invalid", 2, 1, model.Element{
			ElementType: "unparsed", Raw: "@unknown А,Б", Start: 1, End: 13, ErrorIDs: []string{"parser-error"},
		})
		line.HasErrors = true
		result := v018Document(t, line)
		result.Document.HasErrors = true
		result.Diagnostics = []model.Diagnostic{{
			ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
			SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
			Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 13}},
			RelatedLocations: []model.Location{},
		}}
		v018Run(t, result, "current", nil)
	})
}

func TestV018LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}},
		{"LF without final EOL", [3]model.LineEnding{"\n", "\n", ""}},
		{"CRLF without final EOL", [3]model.LineEnding{"\r\n", "\r\n", ""}},
		{"mixed endings", [3]model.LineEnding{"\r\n", "\n", "\r\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v018Document(t, v018Text(t, "Hello,world!", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v018Run(t, result, "current", [][3]int{{3, 6, 7}})
		})
	}
}

func TestV018UnavailableLines(t *testing.T) {
	// Контракт модели допускает известную версию при отсутствии доступных строк.
	v018Run(t, v018UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV018UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"", "1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v018UnavailableDocument(t, version)
			before := v018Snapshot(t, result)
			checks, err := validator.Prepare(result, v018Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%q) = %v, %v; want refusal", version, checks, err)
			}
			if v018Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV018RepeatedRuns(t *testing.T) {
	result := v018Document(t, v018Text(t, "А,Б", 2, 1))
	v018Run(t, result, "first", [][3]int{{3, 2, 3}})
	v018Run(t, result, "second", [][3]int{{3, 2, 3}})
	v018Run(t, v018Document(t, v018Text(t, "А, Б", 2, 1)), "first", nil)
}

// v018Registry использует только штатный каталог без замены регистраций.
func v018Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v018UnavailableDocument задаёт версию при недоступном исходном тексте.
func v018UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      []model.Line{}, Diagnostics: []model.Diagnostic{},
	}
}

// v018Document дополняет фрагмент обязательными @dsl-version и заголовком
// по DSL 1.1–1.3, 1.30 и 2.1.3. Текст образует шаг по 2.1.15; элементы,
// родители и глубина тела заданы явно, без повторного разбора DSL.
func v018Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v018UnavailableDocument(t, "1.2")
	result.Lines = []model.Line{
		v018Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v018Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v018Element(t, "version", "1.2", "1.2", 14, 17)),
		v018Line(t, "# Unit 1", "heading", 0, 0,
			v018Element(t, "heading-level", "#", "1", 1, 2), v018Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	result.Lines = append(result.Lines, body...)
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count, title, encoding := len(result.Lines), "Unit 1", "UTF-8"
	result.Document.LineCount = &count
	result.Document.Metadata.Title = &title
	result.Document.Encoding = &encoding
	return result
}

// v018InsertMetadata помещает заданный тег перед заголовком по DSL 1.31.
func v018InsertMetadata(t *testing.T, result *model.Result, line model.Line) {
	t.Helper()
	heading := result.Lines[1]
	heading.Number = 3
	line.Number = 2
	result.Lines = []model.Line{result.Lines[0], line, heading}
	count := 3
	result.Document.LineCount = &count
}

// v018Line сохраняет явно заданную структуру строки без интерпретации Raw.
func v018Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v018Element сохраняет заданные Raw, Value и исходный Unicode-диапазон.
func v018Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v018Text создаёт содержимое без краевых пробелов и снимаемого экранирования.
// Длина вычисляется только для входного элемента, не для ожидаемых результатов.
func v018Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v018Line(t, text, "content", parent, depth,
		v018Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v018Run проверяет V018 текущего запуска через полный штатный конвейер.
// Явные тройки задают строку, начало и исключающую правую границу диапазона.
func v018Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v018Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v018Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v018Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V018" && entry.Source != processingID {
			t.Fatalf("new V018 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V018" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V018 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[0], Column: expected[2]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V018" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "После знака препинания отсутствует пробел." || entry.DiagnosticScope != "line" ||
			entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	// Prepare проверяет флаги и связи полного результата по контракту модели.
	// Затем из копии убираем только эффекты соседних диагностик для снимка
	// исходных данных; ожидания V018 и прежняя история остаются неизменными.
	if _, err := validator.Prepare(result, v018Registry(t)); err != nil {
		t.Fatalf("invalid resulting model: %v", err)
	}
	var previous, original model.Result
	if err := json.Unmarshal([]byte(v018Snapshot(t, result)), &previous); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	otherNewIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[diagnosticCount:] {
		if entry.DiagnosticCode != "V018" {
			otherNewIDs[entry.ID] = true
		}
	}
	previous.Document.HasErrors = original.Document.HasErrors
	for i := range previous.Lines {
		previous.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range previous.Lines[i].Elements {
			element := &previous.Lines[i].Elements[j]
			kept := element.ErrorIDs[:0]
			for _, id := range element.ErrorIDs {
				if !otherNewIDs[id] {
					kept = append(kept, id)
				}
			}
			element.ErrorIDs = kept
		}
	}
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v018Snapshot(t, &previous) != before {
		t.Fatal("document, lines, flags, element links or previous history changed")
	}
}

// v018Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v018Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

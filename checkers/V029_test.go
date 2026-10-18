package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v029Expected задаёт строку и исходные Unicode-колонки [start,end).
type v029Expected struct{ line, start, end int }

func TestV029Registration(t *testing.T) {
	definitions, err := v029Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V029" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Короткое тире между числами отделено пробелами." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V029 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V029 definitions = %d, want 1", matches)
	}
}

func TestV029Candidates(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v029Expected
	}{
		{"reference", "Задания 1–12.", nil},
		{"example", "Задания 1 – 12.", []v029Expected{{3, 10, 13}}},
		{"left space only", "1 –12", []v029Expected{{3, 2, 4}}},
		{"right space only", "1– 12", []v029Expected{{3, 2, 4}}},
		{"maximal spaces", "1  –   2", []v029Expected{{3, 2, 8}}},
		{"mixed horizontal spaces", "1\t\u00A0–\u2003 \t2", []v029Expected{{3, 2, 8}}},
		{"Arabic digits", "١ – ٢", []v029Expected{{3, 2, 5}}},
		{"fullwidth digits", "１ – ２", []v029Expected{{3, 2, 5}}},
		{"supplementary plane digits", "𝟙 – 𝟚", []v029Expected{{3, 2, 5}}},
		{"mixed digit scripts", "١2 – ３4", []v029Expected{{3, 3, 6}}},
		{"outside Nd", "² – ³", nil},
		{"Roman numeral", "Ⅰ – 2", nil},
		{"Unicode minus", "−1 – −2", []v029Expected{{3, 3, 6}}},
		{"ASCII minus", "-1 – -2", []v029Expected{{3, 3, 6}}},
		{"plus", "+1 – +2", []v029Expected{{3, 3, 6}}},
		{"different signs", "−1 – +2", []v029Expected{{3, 3, 6}}},
		{"right sign excluded with left space only", "1 –−2", []v029Expected{{3, 2, 4}}},
		{"right sign excluded with right space only", "1– −2", []v029Expected{{3, 2, 4}}},
		{"signed without spaces", "−1–−2", nil},
		{"detached sign", "1 – − 2", nil},
		{"dot and comma", "1.5 – 2,5", []v029Expected{{3, 4, 7}}},
		// Уточнение пользователя: не проверять формат десятичной дроби.
		{"multiple dots accepted", "1.2.3 – 4", []v029Expected{{3, 6, 9}}},
		{"multiple mixed separators accepted", "1,2.3 – 4.5,6", []v029Expected{{3, 6, 9}}},
		{"leading zeros", "0001 – 0002", []v029Expected{{3, 5, 8}}},
		{"zero and descending range", "12 – 0", []v029Expected{{3, 3, 6}}},
		{"Latin prefix", "A1 – 12", nil},
		{"Cyrillic suffix", "1 – 12Б", nil},
		{"lowercase both ends", "a1 – 2b", nil},
		{"prefix on right", "1 – A2", nil},
		{"suffix on left", "1A – 2", nil},
		{"letter beyond dot parts", "A1.2 – 3", nil},
		{"marked letter prefix", "a\u03011 – 2", nil},
		{"punctuation separates designation", "A:1 – 2", []v029Expected{{3, 4, 7}}},
		{"chain of three", "1 – 2 – 3", nil},
		{"chain compact on left", "1–2 – 3", nil},
		{"chain compact on right", "1 – 2–3", nil},
		{"chain of four", "1 – 2 – 3 – 4", nil},
		{"signed chain with dots", "−1.2 – +3 – 4", nil},
		{"chain with Unicode digits and tabs", "١\t–٢–\u00A0３", nil},
		{"two independent ranges", "1 – 2; 3 – 4", []v029Expected{{3, 2, 5}, {3, 9, 12}}},
		{"unrelated number before pair", "0 1 – 2", []v029Expected{{3, 4, 7}}},
		{"pair after chain", "1–2–3; 4 – 5", []v029Expected{{3, 9, 12}}},
		{"pair before chain", "1 – 2; 3–4–5", []v029Expected{{3, 2, 5}}},
		{"hyphen", "1 - 2", nil},
		{"em dash", "1 — 2", nil},
		{"two en dashes", "1 – – 2", nil},
		{"no left number", "– 2", nil},
		{"no right number", "1 –", nil},
		{"word instead of number", "слово – 2", nil},
		{"only dash", "–", nil},
		{"no dash", "1 2", nil},
		{"Unicode columns", "😀 1 – 2", []v029Expected{{3, 4, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v029Run(t, v029Document(t, v029Text(t, tc.raw, 2, 1)), "current", tc.want) })
	}
}

func TestV029HorizontalSpaces(t *testing.T) {
	// Явное множество Zs и U+0009; ожидания не вычисляются классификатором.
	for _, tc := range []struct{ name, space string }{
		{"tab", "\t"}, {"U+0020", " "}, {"U+00A0", "\u00A0"}, {"U+1680", "\u1680"},
		{"U+2000", "\u2000"}, {"U+2001", "\u2001"}, {"U+2002", "\u2002"},
		{"U+2003", "\u2003"}, {"U+2004", "\u2004"}, {"U+2005", "\u2005"},
		{"U+2006", "\u2006"}, {"U+2007", "\u2007"}, {"U+2008", "\u2008"},
		{"U+2009", "\u2009"}, {"U+200A", "\u200A"}, {"U+202F", "\u202F"},
		{"U+205F", "\u205F"}, {"U+3000", "\u3000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v029Run(t, v029Document(t, v029Text(t, "1"+tc.space+"–"+tc.space+"2", 2, 1)), "current", []v029Expected{{3, 2, 5}})
		})
	}
	for _, tc := range []struct{ name, raw string }{
		{"vertical tab", "1\u000B–2"}, {"form feed", "1–\u000C2"}, {"NEL", "1\u0085–2"},
		{"line separator", "1–\u20282"}, {"paragraph separator", "1\u2029–2"},
		{"zero width space", "1–\u200B2"}, {"internal BOM", "1\uFEFF–2"},
	} {
		t.Run(tc.name, func(t *testing.T) { v029Run(t, v029Document(t, v029Text(t, tc.raw, 2, 1)), "current", nil) })
	}
}

func TestV029LinesAndRaw(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v029Expected
	}{
		{"blank lines", []model.Line{v029Line(t, "", "blank", 2, 1), v029Line(t, " \t ", "blank", 2, 1), v029Text(t, "Read this.", 2, 1)}, nil},
		{"do not join lines", []model.Line{v029Text(t, "1 –", 2, 1), v029Text(t, "2", 2, 1)}, nil},
		{"first and last body lines", []model.Line{v029Text(t, "1 – 2", 2, 1), v029Text(t, "3–4", 2, 1), v029Text(t, "5 – 6", 2, 1)}, []v029Expected{{3, 2, 5}, {5, 2, 5}}},
		{"chains do not cross lines", []model.Line{v029Text(t, "1 – 2", 2, 1), v029Text(t, "– 3", 2, 1)}, []v029Expected{{3, 2, 5}}},
		{"original indentation", []model.Line{v029Line(t, "  1 – 2  ", "content", 2, 1,
			v029Element(t, "content", "1 – 2", "1 – 2", 3, 8))}, []v029Expected{{3, 4, 7}}},
		// DSL 1.21: экранирование снимается только в Value, колонки исходного Raw сохраняются.
		{"escaped prefix", []model.Line{v029Line(t, "\\{x\\} 1 – 2", "content", 2, 1,
			v029Element(t, "content", "\\{x\\} 1 – 2", "{x} 1 – 2", 1, 12))}, []v029Expected{{3, 8, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v029Run(t, v029Document(t, tc.body...), "current", tc.want) })
	}
}

func TestV029Blocks(t *testing.T) {
	for _, tc := range []struct {
		name, tag, raw string
		want           []v029Expected
	}{
		{"HTML is not transparent", "text", "1<em> – </em>2", nil},
		{"range inside HTML", "text", "<em>1 – 2</em>", []v029Expected{{4, 6, 9}}},
		{"table is one raw line", "table", "1 – 2 | текст", []v029Expected{{4, 2, 5}}},
		{"table cells do not form chain", "table", "1 – 2 | 3 – 4", []v029Expected{{4, 2, 5}, {4, 10, 13}}},
		{"comment", "editor", "1 – 2", []v029Expected{{4, 2, 5}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 3.6, 3.4, 2.8: полноценные блоки и отображаемый текст после editor.
			v029Run(t, v029Document(t, v029Open(t, tc.tag, 2, 1), v029Text(t, tc.raw, 3, 2), v029Close(t, 3, 2),
				v029Text(t, "Read this.", 2, 1)), "current", tc.want)
		})
	}
	t.Run("nested text and next block", func(t *testing.T) {
		// DSL 3.12.9–3.12.10 допускает @text внутри @fragment.
		result := v029Document(t,
			v029Line(t, "@fragment f {", "block-start", 2, 1,
				v029Element(t, "tag", "@fragment", "fragment", 1, 10), v029Element(t, "identifier", "f", "f", 11, 12), v029Element(t, "block-open", "{", "{", 13, 14)),
			v029Open(t, "text", 3, 2), v029Text(t, "1 – 2", 4, 3), v029Close(t, 4, 3), v029Close(t, 3, 2),
			v029Open(t, "text", 2, 1), v029Text(t, "3 – 4", 8, 2), v029Close(t, 8, 2))
		v029Run(t, result, "current", []v029Expected{{5, 2, 5}, {9, 2, 5}})
	})
	t.Run("table separator", func(t *testing.T) {
		result := v029Document(t, v029Open(t, "table", 2, 1), v029Text(t, "1 – 2", 3, 2),
			v029Line(t, "---", "separator", 3, 2), v029Text(t, "3 – 4", 3, 2), v029Close(t, 3, 2))
		v029Run(t, result, "current", []v029Expected{{4, 2, 5}, {6, 2, 5}})
	})
	t.Run("placeholder answer", func(t *testing.T) {
		// DSL 4.3.7–4.3.10: ответ в блочном @multifill.
		result := v029Document(t, v029Open(t, "multifill", 2, 1),
			v029Line(t, "_____{1 – 2}", "content", 3, 2, v029Element(t, "placeholder", "_____{1 – 2}", "1 – 2", 1, 13)), v029Close(t, 3, 2))
		v029Run(t, result, "current", []v029Expected{{4, 8, 11}})
	})
}

func TestV029HeadingNameAndPath(t *testing.T) {
	t.Run("heading", func(t *testing.T) {
		result := v029Document(t, v029Text(t, "Read this.", 2, 1))
		title := "1 – 2"
		result.Document.Metadata.Title = &title
		result.Lines[1].Raw = "# 1 – 2"
		result.Lines[1].Elements[1] = v029Element(t, "title", title, title, 3, 8)
		v029Run(t, result, "current", []v029Expected{{2, 4, 7}})
	})
	t.Run("name", func(t *testing.T) {
		result := v029Document(t, v029Text(t, "Read this.", 2, 1))
		name, parent, count := "1 – 2", 3, 4
		meta := v029Line(t, "@section 1 – 2", "tag", 0, 0,
			v029Element(t, "tag", "@section", "section", 1, 9), v029Element(t, "name", name, name, 10, 15))
		result.Lines = []model.Line{result.Lines[0], meta, result.Lines[1], result.Lines[2]}
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		result.Lines[3].ParentLine = &parent
		result.Document.LineCount = &count
		result.Document.Metadata.Section = &name
		v029Run(t, result, "current", []v029Expected{{2, 11, 14}})
	})
	t.Run("media path", func(t *testing.T) {
		result := v029Document(t, v029Line(t, "@media image \"1 – 2.png\"", "tag", 2, 1,
			v029Element(t, "tag", "@media", "media", 1, 7), v029Element(t, "media-type", "image", "image", 8, 13),
			v029Element(t, "source", "\"1 – 2.png\"", "1 – 2.png", 14, 25)))
		v029Run(t, result, "current", []v029Expected{{3, 16, 19}})
	})
}

func TestV029Unparsed(t *testing.T) {
	// DSL 1.16: неизвестный тег — сохранённая ошибка парсера; V029 продолжает проверку.
	line := v029Line(t, "@unknown 1 – 2", "invalid", 2, 1, model.Element{
		ElementType: "unparsed", Raw: "@unknown 1 – 2", Start: 1, End: 15, ErrorIDs: []string{"parser-error"},
	})
	line.HasErrors = true
	result := v029Document(t, line, v029Text(t, "3 – 4", 2, 1))
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{
		ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
		Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 15}}, RelatedLocations: []model.Location{},
	}}
	v029Run(t, result, "current", []v029Expected{{3, 11, 14}, {4, 2, 5}})
}

func TestV029LineEndingsAndBOM(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [3]model.LineEnding
		bom  bool
	}{
		{"LF", [3]model.LineEnding{"\n", "\n", "\n"}, false},
		{"CRLF", [3]model.LineEnding{"\r\n", "\r\n", "\r\n"}, false},
		{"mixed", [3]model.LineEnding{"\n", "\r\n", "\n"}, false},
		{"no final EOL", [3]model.LineEnding{"\n", "\n", ""}, false},
		{"BOM", [3]model.LineEnding{"\n", "\n", "\n"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DSL 1.13: BOM и EOL отсутствуют в Raw.
			result := v029Document(t, v029Text(t, "Задания 1 – 12.", 2, 1))
			result.Document.HasBOM = &tc.bom
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v029Run(t, result, "current", []v029Expected{{3, 10, 13}})
		})
	}
}

func TestV029EmptyLinesAndVersions(t *testing.T) {
	v029Run(t, v029UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v029UnavailableDocument(t, version)
			before := v029Snapshot(t, result)
			checks, err := validator.Prepare(result, v029Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) did not refuse: %v, %v", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v029Snapshot(t, result) != before {
				t.Fatal("model changed on refusal")
			}
		})
	}
}

func TestV029RepeatedRuns(t *testing.T) {
	result := v029Document(t, v029Text(t, "Задания 1 – 12.", 2, 1))
	want := []v029Expected{{3, 10, 13}}
	v029Run(t, result, "first", want)
	v029Run(t, result, "second", want)
	v029Run(t, v029Document(t, v029Text(t, "Задания 1–12.", 2, 1)), "first", nil)
}

// v029Open создаёт объявление блока без параметров с явными родителем и глубиной.
func v029Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v029Line(t, "@"+tag+" {", "block-start", parent, depth,
		v029Element(t, "tag", "@"+tag, tag, 1, end), v029Element(t, "block-open", "{", "{", end+1, end+2))
}

// v029Close создаёт закрывающую строку указанного блока.
func v029Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v029Line(t, "}", "block-end", parent, depth, v029Element(t, "block-close", "}", "}", 1, 2))
}

// v029Registry создаёт реестр через штатный каталог без подмены регистраций.
func v029Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v029UnavailableDocument сохраняет известную версию при недоступных строках.
func v029UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v029Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v029Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v029Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v029Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v029Element(t, "version", "1.2", "1.2", 14, 17)),
		v029Line(t, "# Unit 1", "heading", 0, 0,
			v029Element(t, "heading-level", "#", "1", 1, 2),
			v029Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}},
		},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       lines,
		Diagnostics: []model.Diagnostic{},
	}
}

// v029Line собирает строку с явно заданной структурой, без классификации Raw.
func v029Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v029Element сохраняет исходное и семантическое значения и явный диапазон.
func v029Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v029Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v029Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v029Line(t, text, "content", parent, depth,
		v029Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v029Run проверяет текущие V029 через штатный конвейер и независимые ожидания.
// Проверяет неизменность документа, строк, флагов, ссылок и прежней истории.
func v029Run(t *testing.T, result *model.Result, processingID string, want []v029Expected) {
	t.Helper()
	before := v029Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v029Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v029Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V029" && entry.Source != processingID {
			t.Fatalf("new V029 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V029" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V029 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected.line, Column: expected.start},
			End:   model.Position{Line: expected.line, Column: expected.end},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		message := "Короткое тире между числами отделено пробелами.\nконкретизация: Для числового диапазона рекомендуется запись без пробелов: 1–12"
		if entry.Message != message {
			t.Errorf("occurrence %d message = %q, want %q", i, entry.Message, message)
		}
		if entry.DiagnosticCode != "V029" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing count or lost previous diagnostics")
	}
	previous := *result
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v029Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v029Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v029Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

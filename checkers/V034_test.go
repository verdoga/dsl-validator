package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v034Expected задаёт строку и исходные Unicode-колонки [start,end).
type v034Expected struct{ line, start, end int }

func TestV034Registration(t *testing.T) {
	definitions, err := v034Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V034" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Запись с @ внутри строки остаётся обычным текстом." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V034 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V034 definitions = %d, want 1", matches)
	}
}

func TestV034KnownTags(t *testing.T) {
	// Полный независимый перечень из DSL 1.2, разделы 1–4.
	// Конечные колонки заданы явно, без вызова классификатора или подсчёта имени.
	for _, tc := range []struct {
		tag string
		end int
	}{
		{"alt", 7}, {"answer", 10}, {"choice", 10}, {"document-id", 15}, {"dsl-version", 15},
		{"editor", 10}, {"endtask", 11}, {"example", 11}, {"fragment", 12}, {"header", 10},
		{"hint", 8}, {"include", 11}, {"instr", 9}, {"key", 7}, {"matching", 12},
		{"media", 9}, {"multichoice", 15}, {"multifill", 13}, {"newpage", 11}, {"note", 8},
		{"order", 9}, {"ordering", 12}, {"question", 12}, {"resource-dir", 16}, {"script", 10},
		{"section", 11}, {"speaking", 12}, {"step", 8}, {"table", 9}, {"task", 8},
		{"text", 8}, {"variant", 11}, {"variants", 12}, {"wordlist", 12},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			v034Run(t, v034Document(t, v034Text(t, "x @"+tc.tag, 2, 1)), "current", []v034Expected{{3, 3, tc.end}})
		})
	}
}

func TestV034TextBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v034Expected
	}{
		{"end of line", "текст @hint", []v034Expected{{3, 7, 12}}},
		{"space after name", "текст @hint далее", []v034Expected{{3, 7, 12}}},
		{"mixed case", "текст @HiNt далее", []v034Expected{{3, 7, 12}}},
		{"uppercase hyphenated name", "x @DSL-VERSION 1.2", []v034Expected{{3, 3, 15}}},
		{"tabs", "текст\t@hint\tдалее", []v034Expected{{3, 7, 12}}},
		{"multiple", "текст @note @hint", []v034Expected{{3, 7, 12}, {3, 13, 18}}},
		{"email", "user@note", nil},
		{"punctuation before", "текст (@hint далее)", nil},
		{"punctuation after", "текст @hint.", nil},
		{"closing punctuation after", "текст @hint)", nil},
		{"NBSP before", "текст\u00A0@hint", nil},
		{"NBSP after", "текст @hint\u00A0далее", nil},
		{"vertical tab before", "текст\u000B@hint", nil},
		{"line separator after", "текст @hint\u2028далее", nil},
		{"incomplete name", "текст @hin", nil},
		{"longer name", "текст @hinting", nil},
		{"name with digit", "текст @hint2", nil},
		{"name with hyphen", "текст @hint-extra", nil},
		{"unknown name", "текст @unknown", nil},
		{"no name", "текст @", nil},
		{"two at signs", "текст @@hint", nil},
		{"Unicode columns", "😀 @hint", []v034Expected{{3, 3, 8}}},
		// Упоминание editor не создаёт контекст редакторского комментария.
		{"literal context names", "текст @editor @multifill @hint", []v034Expected{{3, 7, 14}, {3, 15, 25}, {3, 26, 31}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v034Run(t, v034Document(t, v034Text(t, tc.raw, 2, 1)), "current", tc.want) })
	}
}

func TestV034ExamplesAndRaw(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v034Expected
	}{
		// Эталон дополнен только обязательными версией и заголовком.
		{"reference", []model.Line{
			v034Line(t, "@note Read.", "tag", 2, 1, v034Element(t, "tag", "@note", "note", 1, 6), v034Element(t, "content", "Read.", "Read.", 7, 12)),
			v034Line(t, "@hint Look at the example.", "tag", 2, 1, v034Element(t, "tag", "@hint", "hint", 1, 6), v034Element(t, "content", "Look at the example.", "Look at the example.", 7, 27)),
		}, nil},
		{"example", []model.Line{v034Example(t)}, []v034Expected{{3, 13, 18}}},
		{"element beginning", []model.Line{
			v034Line(t, "@note @hint Look.", "tag", 2, 1, v034Element(t, "tag", "@note", "note", 1, 6), v034Element(t, "content", "@hint Look.", "@hint Look.", 7, 18)),
		}, []v034Expected{{3, 7, 12}}},
		{"only spaces before real tag", []model.Line{
			v034Line(t, "  @hint Look.", "tag", 2, 1, v034Element(t, "tag", "@hint", "hint", 3, 8), v034Element(t, "content", "Look.", "Look.", 9, 14)),
		}, nil},
		{"only tab before real tag", []model.Line{
			v034Line(t, "\t@hint Look.", "tag", 2, 1, v034Element(t, "tag", "@hint", "hint", 2, 7), v034Element(t, "content", "Look.", "Look.", 8, 13)),
		}, nil},
		// DSL 1.22: перед @ внутри строки обратная косая черта сохраняется.
		{"backslash", []model.Line{v034Text(t, "текст \\@hint", 2, 1)}, nil},
		{"paired backslashes", []model.Line{v034Line(t, "текст \\\\@hint", "content", 2, 1,
			v034Element(t, "content", "текст \\\\@hint", "текст \\@hint", 1, 14))}, nil},
		{"escaped initial tag", []model.Line{v034Line(t, "\\@hint Look.", "content", 2, 1,
			v034Element(t, "content", "\\@hint Look.", "@hint Look.", 1, 13))}, nil},
		{"escaped prefix keeps columns", []model.Line{v034Line(t, "\\{x\\} @hint", "content", 2, 1,
			v034Element(t, "content", "\\{x\\} @hint", "{x} @hint", 1, 12))}, []v034Expected{{3, 7, 12}}},
		{"trailing spaces", []model.Line{v034Line(t, "текст @hint  ", "content", 2, 1,
			v034Element(t, "content", "текст @hint", "текст @hint", 1, 12))}, []v034Expected{{3, 7, 12}}},
		{"blank lines", []model.Line{v034Line(t, "", "blank", 2, 1), v034Line(t, " \t", "blank", 2, 1), v034Text(t, "Read.", 2, 1)}, nil},
		{"no joining across lines", []model.Line{v034Text(t, "текст @hi", 2, 1), v034Text(t, "nt", 2, 1)}, nil},
		{"first and last body lines", []model.Line{v034Text(t, "x @hint", 2, 1), v034Text(t, "Read.", 2, 1), v034Text(t, "x @note", 2, 1)}, []v034Expected{{3, 3, 8}, {5, 3, 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v034Run(t, v034Document(t, tc.body...), "current", tc.want) })
	}
}

func TestV034ElementBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want []v034Expected
	}{
		{"element end is not line end", v034Line(t, "текст @hint_____", "content", 2, 1,
			v034Element(t, "content", "текст @hint", "текст @hint", 1, 12),
			model.Element{ElementType: "placeholder", Raw: "_____", Start: 12, End: 17, ErrorIDs: []string{}}), nil},
		{"element beginning after placeholder", v034Line(t, "_____@hint Look.", "content", 2, 1,
			model.Element{ElementType: "placeholder", Raw: "_____", Start: 1, End: 6, ErrorIDs: []string{}},
			v034Element(t, "content", "@hint Look.", "@hint Look.", 6, 17)), []v034Expected{{3, 6, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v034Run(t, v034Document(t, tc.line), "current", tc.want) })
	}
}

func TestV034Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v034Expected
	}{
		{"single line editor", []model.Line{
			v034Line(t, "@editor текст @hint далее", "tag", 2, 1,
				v034Element(t, "tag", "@editor", "editor", 1, 8), v034Element(t, "content", "текст @hint далее", "текст @hint далее", 9, 26)),
			v034Text(t, "Read.", 2, 1),
		}, nil},
		{"block editor then ordinary text", []model.Line{
			v034Open(t, "editor", 2, 1), v034Text(t, "текст @hint", 3, 2), v034Close(t, 3, 2), v034Text(t, "текст @hint", 2, 1),
		}, []v034Expected{{6, 7, 12}}},
		{"uppercase editor", []model.Line{
			v034Line(t, "@EDITOR {", "block-start", 2, 1, v034Element(t, "tag", "@EDITOR", "editor", 1, 8), v034Element(t, "block-open", "{", "{", 9, 10)),
			v034Text(t, "текст @hint", 3, 2), v034Close(t, 3, 2), v034Text(t, "Read.", 2, 1),
		}, nil},
		{"multifill literal content", []model.Line{
			v034Open(t, "multifill", 2, 1), v034Text(t, "текст @hint", 3, 2), v034Close(t, 3, 2),
		}, nil},
		{"placeholder answer", []model.Line{
			v034Open(t, "multifill", 2, 1), v034Line(t, "_____{текст @hint}", "content", 3, 2,
				v034Element(t, "placeholder", "_____{текст @hint}", "текст @hint", 1, 19)), v034Close(t, 3, 2),
		}, nil},
		{"multifill inline instruction", []model.Line{
			v034Line(t, "@multifill текст @hint далее", "tag", 2, 1,
				v034Element(t, "tag", "@multifill", "multifill", 1, 11), v034Element(t, "content", "текст @hint далее", "текст @hint далее", 12, 29)),
		}, []v034Expected{{3, 18, 23}}},
		{"multifill block instruction", []model.Line{
			v034Line(t, "@multifill текст @hint далее {", "block-start", 2, 1,
				v034Element(t, "tag", "@multifill", "multifill", 1, 11), v034Element(t, "content", "текст @hint далее", "текст @hint далее", 12, 29), v034Element(t, "block-open", "{", "{", 30, 31)),
			v034Text(t, "текст @hint", 3, 2), v034Close(t, 3, 2), v034Text(t, "x @hint", 2, 1),
		}, []v034Expected{{3, 18, 23}, {6, 3, 8}}},
		{"nested text", []model.Line{
			v034Line(t, "@fragment f {", "block-start", 2, 1, v034Element(t, "tag", "@fragment", "fragment", 1, 10), v034Element(t, "identifier", "f", "f", 11, 12), v034Element(t, "block-open", "{", "{", 13, 14)),
			v034Open(t, "text", 3, 2), v034Text(t, "текст @hint", 4, 3), v034Close(t, 4, 3), v034Close(t, 3, 2), v034Text(t, "Read.", 2, 1),
		}, []v034Expected{{5, 7, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) { v034Run(t, v034Document(t, tc.body...), "current", tc.want) })
	}
}

func TestV034HeadingNameAndExcludedFields(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		result := v034Document(t, v034Text(t, "Read.", 2, 1))
		title := "@hint Look."
		result.Document.Metadata.Title = &title
		result.Lines[1].Raw = "# @hint Look."
		result.Lines[1].Elements[1] = v034Element(t, "title", title, title, 3, 14)
		v034Run(t, result, "current", []v034Expected{{2, 3, 8}})
	})
	t.Run("name", func(t *testing.T) {
		result := v034Document(t, v034Text(t, "Read.", 2, 1))
		name, parent, count := "@hint Look.", 3, 4
		meta := v034Line(t, "@section @hint Look.", "tag", 0, 0,
			v034Element(t, "tag", "@section", "section", 1, 9), v034Element(t, "name", name, name, 10, 21))
		result.Lines = []model.Line{result.Lines[0], meta, result.Lines[1], result.Lines[2]}
		for i := range result.Lines {
			result.Lines[i].Number = i + 1
		}
		result.Lines[3].ParentLine = &parent
		result.Document.LineCount = &count
		result.Document.Metadata.Section = &name
		v034Run(t, result, "current", []v034Expected{{2, 10, 15}})
	})
	t.Run("media source", func(t *testing.T) {
		result := v034Document(t, v034Line(t, "@media image \"text @hint file.png\"", "tag", 2, 1,
			v034Element(t, "tag", "@media", "media", 1, 7), v034Element(t, "media-type", "image", "image", 8, 13),
			v034Element(t, "source", "\"text @hint file.png\"", "text @hint file.png", 14, 35)))
		v034Run(t, result, "current", nil)
	})
	t.Run("identifier", func(t *testing.T) {
		result := v034Document(t, v034Line(t, "@task hint", "tag", 2, 1,
			v034Element(t, "tag", "@task", "task", 1, 6), v034Element(t, "identifier", "hint", "hint", 7, 11)), v034Text(t, "Read.", 3, 2))
		v034Run(t, result, "current", nil)
	})
}

func TestV034Unparsed(t *testing.T) {
	// DSL 1.16: неизвестный начальный тег — ошибка парсера; его неразобранный
	// остаток с @hint исключён. Соседняя корректная строка всё равно проверяется.
	for _, tc := range []struct {
		raw, following string
		want           []v034Expected
	}{
		{"@unknown text", "Read.", nil},
		{"@unknown текст @hint", "Read.", nil},
		{"@unknown текст @hint", "x @hint", []v034Expected{{4, 3, 8}}},
	} {
		t.Run(tc.raw+"/"+tc.following, func(t *testing.T) {
			raw := tc.raw
			end := utf8.RuneCountInString(raw) + 1
			line := v034Line(t, raw, "invalid", 2, 1, model.Element{ElementType: "unparsed", Raw: raw, Start: 1, End: end, ErrorIDs: []string{"parser-error"}})
			line.HasErrors = true
			result := v034Document(t, line, v034Text(t, tc.following, 2, 1))
			result.Document.HasErrors = true
			result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный тег.",
				Location: &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: end}}, RelatedLocations: []model.Location{}}}
			v034Run(t, result, "current", tc.want)
		})
	}
}

func TestV034LineEndingsAndBOM(t *testing.T) {
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
			result := v034Document(t, v034Example(t))
			result.Document.HasBOM = &tc.bom
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v034Run(t, result, "current", []v034Expected{{3, 13, 18}})
		})
	}
}

func TestV034EmptyLinesAndVersions(t *testing.T) {
	v034Run(t, v034UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v034UnavailableDocument(t, version)
			before := v034Snapshot(t, result)
			checks, err := validator.Prepare(result, v034Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) did not refuse: %v, %v", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v034Snapshot(t, result) != before {
				t.Fatal("model changed on refusal")
			}
		})
	}
}

func TestV034RepeatedRuns(t *testing.T) {
	result := v034Document(t, v034Example(t))
	v034Run(t, result, "first", []v034Expected{{3, 13, 18}})
	v034Run(t, result, "second", []v034Expected{{3, 13, 18}})
	v034Run(t, v034Document(t, v034Text(t, "Read.", 2, 1)), "first", nil)
}

// v034Example сохраняет значимое содержимое примера и готовые элементы строки.
func v034Example(t *testing.T) model.Line {
	t.Helper()
	return v034Line(t, "@note Read. @hint Look at the example.", "tag", 2, 1,
		v034Element(t, "tag", "@note", "note", 1, 6), v034Element(t, "content", "Read. @hint Look at the example.", "Read. @hint Look at the example.", 7, 39))
}

// v034Open создаёт объявление блока без параметров с явными родителем и глубиной.
func v034Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v034Line(t, "@"+tag+" {", "block-start", parent, depth,
		v034Element(t, "tag", "@"+tag, tag, 1, end), v034Element(t, "block-open", "{", "{", end+1, end+2))
}

// v034Close создаёт закрывающую строку указанного блока.
func v034Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v034Line(t, "}", "block-end", parent, depth, v034Element(t, "block-close", "}", "}", 1, 2))
}

// v034Registry создаёт реестр через штатный каталог без подмены регистраций.
func v034Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v034UnavailableDocument сохраняет известную версию при недоступных строках.
func v034UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v034Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело содержит явно заданные элементы, родителей и глубину; разбора нет.
func v034Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v034Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v034Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v034Element(t, "version", "1.2", "1.2", 14, 17)),
		v034Line(t, "# Unit 1", "heading", 0, 0,
			v034Element(t, "heading-level", "#", "1", 1, 2),
			v034Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v034Line собирает строку с явно заданной структурой, без классификации Raw.
func v034Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v034Element сохраняет исходное и семантическое значения и явный диапазон.
func v034Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v034Text создаёт готовое неэкранированное содержимое без обычных отступов.
// Длина используется для входного элемента, не для ожидаемых срабатываний.
func v034Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v034Line(t, text, "content", parent, depth,
		v034Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v034Run проверяет текущие V034 через штатный конвейер и независимые ожидания.
// Проверяет неизменность документа, строк, флагов, ссылок и прежней истории.
func v034Run(t *testing.T, result *model.Result, processingID string, want []v034Expected) {
	t.Helper()
	before := v034Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v034Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v034Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V034" && entry.Source != processingID {
			t.Fatalf("new V034 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V034" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V034 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
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
		message := "Запись с @ внутри строки остаётся обычным текстом.\nконкретизация: Для отдельного тега нужна новая строка"
		if entry.Message != message {
			t.Errorf("occurrence %d message = %q, want %q", i, entry.Message, message)
		}
		if entry.DiagnosticCode != "V034" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
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
	if v034Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v034Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v034Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

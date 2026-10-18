package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV004Registration(t *testing.T) {
	definitions, err := v004Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V004" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Message != "Несколько пустых строк подряд." ||
			definition.Scope != "block" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V004 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V004 definitions = %d, want 1", matches)
	}
}

func TestV004Sequences(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"no blank lines", []model.Line{
			v004Note(t, "First.", 2, 1), v004Note(t, "Second.", 2, 1),
		}, nil},
		// Фрагменты таблицы сохранены: одна и две пустые строки
		// между @note. Обязательный внешний контекст добавляет v004Document.
		{"reference", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, nil},
		{"example", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1),
			v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 5, 1}}},
		{"three blank lines form one occurrence", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1),
			v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 6, 1}}},
		{"spaces and tabs remain in range", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "  \t", 2, 1),
			v004Blank(t, "  \t", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 5, 4}}},
		{"empty last raw ends at column one", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "  \t", 2, 1),
			v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 5, 1}}},
		{"two separate runs", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1),
			v004Note(t, "Second.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1),
		}, [][4]int{{4, 1, 5, 1}, {7, 1, 8, 1}}},
		{"single trailing blank", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1),
		}, nil},
		{"two trailing blanks", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1),
		}, [][4]int{{4, 1, 5, 1}}},
		{"first body lines and EOF", []model.Line{
			v004Blank(t, "", 2, 1), v004Blank(t, "\t", 2, 1),
		}, [][4]int{{3, 1, 4, 2}}},
		{"comment separates single blanks", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1),
			v004Line(t, "@editor Check.", "tag", 2, 1,
				v004Element(t, "tag", "@editor", "editor", 1, 8), v004Element(t, "content", "Check.", "Check.", 9, 15)),
			v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, nil},
		{"comment separates two runs", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1),
			v004Line(t, "@editor Check.", "tag", 2, 1,
				v004Element(t, "tag", "@editor", "editor", 1, 8), v004Element(t, "content", "Check.", "Check.", 9, 15)),
			v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 5, 1}, {7, 1, 8, 1}}},
		{"Unicode neighboring text", []model.Line{
			v004Text(t, "Привет 😀", 2, 1), v004Blank(t, "", 2, 1),
			v004Blank(t, "  \t", 2, 1), v004Text(t, "Конец.", 2, 1),
		}, [][4]int{{4, 1, 5, 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v004Run(t, v004Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV004BlocksAndAncestors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"blank lines inside editor are included", []model.Line{
			v004Open(t, "editor", 2, 1), v004Text(t, "Check.", 3, 2),
			v004Blank(t, "", 3, 2), v004Blank(t, "", 3, 2), v004Close(t, 3, 2),
		}, [][4]int{{5, 1, 6, 1}}},
		// DSL 2.8.11: @multifill внутри комментария — content, а не тег.
		{"literal multifill inside editor is not an ancestor", []model.Line{
			v004Open(t, "editor", 2, 1), v004Text(t, "@multifill", 3, 2),
			v004Blank(t, "", 3, 2), v004Blank(t, "", 3, 2), v004Close(t, 3, 2),
		}, [][4]int{{5, 1, 6, 1}}},
		{"ordinary content block", []model.Line{
			v004Open(t, "note", 2, 1), v004Text(t, "First.", 3, 2),
			v004Blank(t, "", 3, 2), v004Blank(t, "", 3, 2), v004Text(t, "Second.", 3, 2), v004Close(t, 3, 2),
		}, [][4]int{{5, 1, 6, 1}}},
		// DSL 3.9.4: название блока допустимо, но элемент title не является tag.
		{"title named multifill does not exclude blanks", []model.Line{
			v004Line(t, "@note multifill {", "block-start", 2, 1,
				v004Element(t, "tag", "@note", "note", 1, 6), v004Element(t, "title", "multifill", "multifill", 7, 16),
				v004Element(t, "block-open", "{", "{", 17, 18)),
			v004Text(t, "First.", 3, 2), v004Blank(t, "", 3, 2), v004Blank(t, "", 3, 2), v004Close(t, 3, 2),
		}, [][4]int{{5, 1, 6, 1}}},
		// DSL 3.8.6, 3.8.11: инструкция содержит собственный текст и один @hint.
		{"nested hint with multiple ancestors", []model.Line{
			v004Open(t, "instr", 2, 1), v004Text(t, "Read the sentence.", 3, 2),
			v004Open(t, "hint", 3, 2), v004Text(t, "Use context.", 5, 3),
			v004Blank(t, "", 5, 3), v004Blank(t, "", 5, 3), v004Text(t, "Check the subject.", 5, 3),
			v004Close(t, 5, 3), v004Close(t, 3, 2),
		}, [][4]int{{7, 1, 8, 1}}},
		// DSL 3.2.8: содержимое с обеих сторон разделителя непустое.
		{"separator terminates runs", []model.Line{
			v004Open(t, "example", 2, 1), v004Text(t, "First.", 3, 2), v004Blank(t, "", 3, 2),
			v004Line(t, "---", "separator", 3, 2), v004Blank(t, "", 3, 2),
			v004Text(t, "Second.", 3, 2), v004Close(t, 3, 2),
		}, nil},
		{"closing boundary terminates runs", []model.Line{
			v004Open(t, "note", 2, 1), v004Text(t, "First.", 3, 2), v004Blank(t, "", 3, 2),
			v004Close(t, 3, 2), v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, nil},
		{"opening boundary terminates runs", []model.Line{
			v004Blank(t, "", 2, 1), v004Open(t, "note", 2, 1), v004Blank(t, "", 4, 2),
			v004Text(t, "First.", 4, 2), v004Close(t, 4, 2),
		}, nil},
		{"inline multifill does not exclude following blanks", []model.Line{
			v004Line(t, "@multifill", "tag", 2, 1, v004Element(t, "tag", "@multifill", "multifill", 1, 11)),
			v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 5, 1}}},
		{"runs before and after multifill", []model.Line{
			v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1),
			v004Open(t, "multifill", 2, 1), v004Text(t, "Text.", 6, 2),
			v004Blank(t, "", 6, 2), v004Blank(t, "", 6, 2), v004Close(t, 6, 2),
			v004Blank(t, "", 2, 1), v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1),
		}, [][4]int{{4, 1, 5, 1}, {11, 1, 12, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v004Run(t, v004Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV004MultifillExclusion(t *testing.T) {
	// DSL 4.4.10–4.4.11: пустые строки сохраняются, вложенные DSL-блоки
	// не распознаются. Модели с вымышленными вложенными блоками не создаются.
	for _, tc := range []struct {
		name string
		tag  string
		raws []string
	}{
		{"two empty lines", "@multifill", []string{"", ""}},
		{"three empty lines", "@multifill", []string{"", "", ""}},
		{"spaces and tabs", "@multifill", []string{"  ", "\t", " \t "}},
		{"uppercase source tag", "@MULTIFILL", []string{"", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opening := v004Open(t, "multifill", 2, 1)
			opening.Raw = tc.tag + " {"
			opening.Elements[0].Raw = tc.tag
			body := []model.Line{opening, v004Text(t, "Text.", 3, 2)}
			for _, raw := range tc.raws {
				body = append(body, v004Blank(t, raw, 3, 2))
			}
			body = append(body, v004Close(t, 3, 2))
			v004Run(t, v004Document(t, body...), "current", nil)
		})
	}
}

func TestV004FirstPermittedBlankLines(t *testing.T) {
	// DSL 1.3, 2.1.3: первая строка — версия; пустые строки допустимы после неё.
	result := v004Document(t)
	result.Lines = []model.Line{
		result.Lines[0], v004Blank(t, "", 0, 0), v004Blank(t, "", 0, 0),
		result.Lines[1], v004Note(t, "First.", 4, 1),
	}
	v004NumberLines(t, result)
	v004Run(t, result, "current", [][4]int{{2, 1, 3, 1}})
}

func TestV004FinalWhitespaceWithoutEOL(t *testing.T) {
	// Полностью пустая строка без EOL не существует в контракте модели.
	result := v004Document(t, v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1), v004Blank(t, "  \t", 2, 1))
	result.Lines[4].LineEnding = model.LineEndingNone
	v004Run(t, result, "current", [][4]int{{4, 1, 5, 4}})
}

func TestV004LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name string
		eols [6]model.LineEnding
	}{
		{"LF", [6]model.LineEnding{"\n", "\n", "\n", "\n", "\n", "\n"}},
		{"CRLF", [6]model.LineEnding{"\r\n", "\r\n", "\r\n", "\r\n", "\r\n", "\r\n"}},
		{"mixed", [6]model.LineEnding{"\r\n", "\n", "\r\n", "\n", "\r\n", "\n"}},
		{"no final EOL", [6]model.LineEnding{"\n", "\n", "\n", "\n", "\n", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v004Document(t, v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1),
				v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1))
			for i, eol := range tc.eols {
				result.Lines[i].LineEnding = eol
			}
			v004Run(t, result, "current", [][4]int{{4, 1, 5, 1}})
		})
	}
}

func TestV004UnavailableLines(t *testing.T) {
	// Известная версия при недоступном исходнике не заменяется пустым DSL-файлом.
	v004Run(t, v004UnavailableDocument(t, "1.2"), "current", nil)
}

func TestV004UnsupportedVersions(t *testing.T) {
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v004UnavailableDocument(t, version)
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v004Snapshot(t, result)
			checks, err := validator.Prepare(result, v004Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if len(result.Diagnostics) != 0 || v004Snapshot(t, result) != before {
				t.Fatal("unsupported version produced diagnostics or changed the model")
			}
		})
	}
}

func TestV004CoexistsWithOtherChecks(t *testing.T) {
	result := v004Document(t, v004Note(t, "First.", 2, 1), v004Blank(t, "  \t", 2, 1),
		v004Blank(t, "  \t", 2, 1), v004Note(t, "Second.", 2, 1))
	v004Run(t, result, "current", [][4]int{{4, 1, 5, 4}})
	for _, tc := range []struct {
		code    diagnostics.Code
		message string
		want    [][4]int
	}{
		{"V001", "Несколько пробелов подряд.", [][4]int{{4, 1, 4, 3}, {5, 1, 5, 3}}},
		{"V002", "В строке есть табуляция.", [][4]int{{4, 3, 4, 4}, {5, 3, 5, 4}}},
		{"V003", "Точки многоточия разделены пробельными символами.", nil},
	} {
		var got []model.Diagnostic
		for _, entry := range result.Diagnostics {
			if entry.Source == "current" && entry.DiagnosticCode == tc.code {
				got = append(got, entry)
			}
		}
		if len(got) != len(tc.want) {
			t.Fatalf("current %s count = %d, want %d", tc.code, len(got), len(tc.want))
		}
		for i, expected := range tc.want {
			entry := got[i]
			want := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[2], Column: expected[3]}}
			if entry.Location == nil || *entry.Location != want || entry.Message != tc.message ||
				entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected %s occurrence: %+v", tc.code, entry)
			}
		}
	}
}

func TestV004RepeatedRunsPreserveHistory(t *testing.T) {
	result := v004Document(t, v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1),
		v004Blank(t, "", 2, 1), v004Note(t, "Second.", 2, 1))
	v004Run(t, result, "first", [][4]int{{4, 1, 5, 1}})
	v004Run(t, result, "second", [][4]int{{4, 1, 5, 1}})
	v004Run(t, v004Document(t, v004Note(t, "First.", 2, 1), v004Blank(t, "", 2, 1),
		v004Note(t, "Second.", 2, 1)), "first", nil)
}

// v004Blank задаёт готовую строку blank; её Raw не классифицируется и не обрезается.
func v004Blank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v004Line(t, raw, "blank", parent, depth)
}

// v004Note составляет однострочное примечание с явно заданным содержимым.
// Координаты входного элемента не используются как ожидания диагностики.
func v004Note(t *testing.T, content string, parent, depth int) model.Line {
	t.Helper()
	return v004Line(t, "@note "+content, "tag", parent, depth,
		v004Element(t, "tag", "@note", "note", 1, 6),
		v004Element(t, "content", content, content, 7, 7+utf8.RuneCountInString(content)))
}

// v004Open составляет объявление блока без параметров; входные координаты
// вычисляются только для фиксированных токенов, не для ожиданий диагностики.
func v004Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v004Line(t, "@"+tag+" {", "block-start", parent, depth,
		v004Element(t, "tag", "@"+tag, tag, 1, end), v004Element(t, "block-open", "{", "{", end+1, end+2))
}

// v004Close составляет закрывающую строку с явно заданным родителем и глубиной.
func v004Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v004Line(t, "}", "block-end", parent, depth, v004Element(t, "block-close", "}", "}", 1, 2))
}

// v004NumberLines задаёт физические номера после добавления метаданных;
// структуру элементов и родителей вызывающий тест задаёт явно.
func v004NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count := len(result.Lines)
	result.Document.LineCount = &count
}

// v004UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v004UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v004Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v004Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v004Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v004Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v004Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v004Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v004Element(t, "version", "1.2", "1.2", 14, 17)),
		v004Line(t, "# Unit 1", "heading", 0, 0,
			v004Element(t, "heading-level", "#", "1", 1, 2),
			v004Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
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

// v004Line собирает строку с явно заданной структурой, без классификации Raw.
func v004Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
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

// v004Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v004Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v004Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v004Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v004Line(t, text, "content", parent, depth,
		v004Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v004Run проверяет штатный конвейер и явные координаты: строка и колонка начала, строка и колонка конца.
// Правая граница исключается. Проверяются только V004 текущего запуска;
// прежние результаты, строки, флаги и ссылки должны сохраниться без изменений.
func v004Run(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v004Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v004Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v004Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V004" && entry.Source != processingID {
			t.Fatalf("new V004 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V004" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V004 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected[0], Column: expected[1]},
			End:   model.Position{Line: expected[2], Column: expected[3]},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V004" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.Message != "Несколько пустых строк подряд." || entry.DiagnosticScope != "block" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	previous := *result
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v004Snapshot(t, &previous) != before {
		t.Fatal("document, lines, error flags, element links or previous history changed")
	}
}

// v004Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v004Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

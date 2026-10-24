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

func TestV061Registration(t *testing.T) {
	definitions, err := v061TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V061" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != "Число в заголовках шагов повторяется." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V061 definitions = %d, want 1", count)
	}
}

func TestV061ReferenceAndExample(t *testing.T) {
	// Перед неизменёнными фрагментами добавлены только версия и заголовок
	// первого уровня. Номера строк фрагментов увеличиваются на два.
	t.Run("reference", func(t *testing.T) {
		result := v061TestDocument(t,
			v061TestHeading(t, "###", "Reading 1", "Reading 1"), v061TestText(t, "Read.", "Read.", 3, 1),
			v061TestHeading(t, "###", "Practice 2", "Practice 2"), v061TestText(t, "Write.", "Write.", 5, 1))
		v061TestSource(t, result, "@dsl-version 1.2\n# Lesson\n### Reading 1\nRead.\n### Practice 2\nWrite.")
		v061TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v061TestDocument(t,
			v061TestHeading(t, "###", "Reading 1", "Reading 1"), v061TestText(t, "Read.", "Read.", 3, 1),
			v061TestHeading(t, "###", "Practice 2", "Practice 2"), v061TestText(t, "Write.", "Write.", 5, 1),
			v061TestHeading(t, "###", "Review 02", "Review 02"), v061TestText(t, "Speak.", "Speak.", 7, 1))
		v061TestSource(t, result, "@dsl-version 1.2\n# Lesson\n### Reading 1\nRead.\n### Practice 2\nWrite.\n### Review 02\nSpeak.")
		v061TestRun(t, result, "current", [][6]int{{7, 12, 14, 5, 14, 15}})
	})
}

func TestV061NumberGroups(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][6]int
	}{
		{"no digits", "Reading", "Practice", nil},
		{"different values", "9", "10", nil},
		{"leading zeros in repeat", "2", "02", [][6]int{{5, 5, 7, 3, 5, 6}}},
		{"leading zeros in first", "02", "2", [][6]int{{5, 5, 6, 3, 5, 7}}},
		{"zero", "0", "000", [][6]int{{5, 5, 8, 3, 5, 6}}},
		{"independent groups", "1/2", "01 и 02", [][6]int{{5, 5, 7, 3, 5, 6}, {5, 10, 12, 3, 7, 8}}},
		{"groups are not joined", "12", "1 2", nil},
		{"dot separates groups", "1", "1.1", [][6]int{{5, 5, 6, 3, 5, 6}, {5, 7, 8, 3, 5, 6}}},
		{"sign is outside group", "2", "-02", [][6]int{{5, 6, 8, 3, 5, 6}}},
		{"letters and case do not matter", "A2b", "c02D", [][6]int{{5, 6, 8, 3, 6, 7}}},
		{"Arabic digit", "2", "٢", [][6]int{{5, 5, 6, 3, 5, 6}}},
		{"mixed scripts form one group", "1٢", "１２", [][6]int{{5, 5, 7, 3, 5, 7}}},
		{"Unicode leading zeros", "٠٠٢", "2", [][6]int{{5, 5, 6, 3, 5, 8}}},
		{"Unicode all-zero group", "٠０", "0", [][6]int{{5, 5, 6, 3, 5, 7}}},
		{"nondecimal numbers excluded", "2", "² Ⅱ", nil},
		{"beyond machine integer", "123456789012345678901234567890", "00123456789012345678901234567890",
			[][6]int{{5, 5, 37, 3, 5, 35}}},
		{"different large values", "123456789012345678901234567890", "123456789012345678901234567891", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v061TestRun(t, v061TestPair(t, tc.first, tc.second), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		title string
		want  [][6]int
	}{
		{"2 / 02 / ٢", [][6]int{{3, 9, 11, 3, 5, 6}, {3, 14, 15, 3, 5, 6}}},
		{"2\t02", [][6]int{{3, 7, 9, 3, 5, 6}}},
		{"2\u0301٢", [][6]int{{3, 7, 8, 3, 5, 6}}},
		{"2²2", [][6]int{{3, 7, 8, 3, 5, 6}}},
	} {
		t.Run("within title "+tc.title, func(t *testing.T) {
			v061TestRun(t, v061TestDocument(t, v061TestHeading(t, "###", tc.title, tc.title),
				v061TestText(t, "Read.", "Read.", 3, 1)), "current", tc.want)
		})
	}
}

func TestV061UnicodeDigitValues(t *testing.T) {
	// Последовательности и ожидания заданы явно, независимо от unicode.Nd
	// и проверяемого преобразования. В каждой последовательности цифры 0..9.
	for _, digits := range []string{
		"٠١٢٣٤٥٦٧٨٩", "۰۱۲۳۴۵۶۷۸۹", "०१२३४५६७८९", "０１２３４５６７８９",
		"𐒠𐒡𐒢𐒣𐒤𐒥𐒦𐒧𐒨𐒩",
		"𝟎𝟏𝟐𝟑𝟒𝟓𝟔𝟕𝟖𝟗", "𝟘𝟙𝟚𝟛𝟜𝟝𝟞𝟟𝟠𝟡", "𝟢𝟣𝟤𝟥𝟦𝟧𝟨𝟩𝟪𝟫",
		"𝟬𝟭𝟮𝟯𝟰𝟱𝟲𝟳𝟴𝟵", "𝟶𝟷𝟸𝟹𝟺𝟻𝟼𝟽𝟾𝟿",
	} {
		t.Run(digits, func(t *testing.T) {
			v061TestRun(t, v061TestPair(t, "0123456789", digits), "current", [][6]int{{5, 5, 15, 3, 5, 15}})
		})
	}
	// Все пять математических наборов входят в один диапазон Nd.
	for _, digit := range []string{"\U000104a2", "\U0001d7d0", "\U0001d7da", "\U0001d7e4", "\U0001d7ee", "\U0001d7f8"} {
		t.Run("two "+digit, func(t *testing.T) {
			v061TestRun(t, v061TestPair(t, "2", digit), "current", [][6]int{{5, 5, 6, 3, 5, 6}})
		})
	}
}

func TestV061OriginalCoordinates(t *testing.T) {
	t.Run("escaped braces in title", func(t *testing.T) {
		v061TestRun(t, v061TestDocument(t,
			v061TestHeading(t, "###", "2", "2"), v061TestText(t, "Read.", "Read.", 3, 1),
			v061TestHeading(t, "###", `\{02\}`, "{02}"), v061TestText(t, "Write.", "Write.", 5, 1)),
			"current", [][6]int{{5, 7, 9, 3, 5, 6}})
	})
	t.Run("indentation and supplementary character", func(t *testing.T) {
		v061TestRun(t, v061TestDocument(t,
			v061TestHeading(t, "###", "2", "2"), v061TestText(t, "Read.", "Read.", 3, 1),
			v061TestLine(t, " \t### 😀02", "heading", 0, 0,
				v061TestElement(t, "heading-level", "###", "3", 3),
				v061TestElement(t, "title", "😀02", "😀02", 7)),
			v061TestText(t, "Write.", "Write.", 5, 1)), "current", [][6]int{{5, 8, 10, 3, 5, 6}})
	})
}

func TestV061DocumentLifetime(t *testing.T) {
	t.Run("third occurrence still refers to first", func(t *testing.T) {
		v061TestRun(t, v061TestDocument(t,
			v061TestHeading(t, "###", "2", "2"), v061TestText(t, "Read.", "Read.", 3, 1),
			v061TestHeading(t, "###", "02", "02"), v061TestText(t, "Write.", "Write.", 5, 1),
			v061TestHeading(t, "###", "002", "002"), v061TestText(t, "Speak.", "Speak.", 7, 1)),
			"current", [][6]int{{5, 5, 7, 3, 5, 6}, {7, 5, 8, 3, 5, 6}})
	})
	t.Run("newpage does not reset", func(t *testing.T) {
		v061TestRun(t, v061TestDocument(t,
			v061TestHeading(t, "###", "2", "2"), v061TestText(t, "Read.", "Read.", 3, 1),
			v061TestLine(t, "@newpage", "tag", 0, 0, v061TestElement(t, "tag", "@newpage", "newpage", 1)),
			v061TestHeading(t, "###", "02", "02"), v061TestText(t, "Write.", "Write.", 6, 1)),
			"current", [][6]int{{6, 5, 7, 3, 5, 6}})
	})
	t.Run("other levels do not populate numbers", func(t *testing.T) {
		result := v061TestDocument(t, v061TestHeading(t, "##", "2", "2"),
			v061TestHeading(t, "###", "2", "2"), v061TestText(t, "Read.", "Read.", 4, 1))
		result.Lines[1] = v061TestHeading(t, "#", "2", "2")
		result.Lines[1].Number = 2
		title := "2"
		result.Document.Metadata.Title, result.Document.Metadata.Subtitle = &title, &title
		v061TestRun(t, result, "current", nil)
	})
	for _, marker := range []string{"#", "##"} {
		t.Run("other level does not reset "+marker, func(t *testing.T) {
			result := v061TestDocument(t,
				v061TestHeading(t, "###", "2", "2"), v061TestText(t, "Read.", "Read.", 3, 1),
				v061TestHeading(t, marker, "Other", "Other"), v061TestText(t, "Text.", "Text.", 5, 1),
				v061TestHeading(t, "###", "02", "02"), v061TestText(t, "Write.", "Write.", 7, 1))
			// DSL 2.1.3–2.1.4 запрещает такое размещение. Модель сохраняет
			// распознанный корневой заголовок с исходной ошибкой размещения.
			if marker == "#" {
				result.Document.Metadata.Title = nil
			} else {
				subtitle := "Other"
				result.Document.Metadata.Subtitle = &subtitle
			}
			v061TestParserError(t, result, 5, -1, "Недопустимое размещение заголовка.")
			v061TestRun(t, result, "current", [][6]int{{7, 5, 7, 3, 5, 6}})
		})
	}
}

func TestV061ExcludedContexts(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "text"} {
		t.Run(tag, func(t *testing.T) {
			pseudo := v061TestText(t, "### 2", "### 2", 3, 2)
			if tag == "text" {
				unparsed := v061TestElement(t, "unparsed", "### 2", "", 1)
				unparsed.Value = nil
				pseudo = v061TestLine(t, "### 2", "invalid", 3, 2, unparsed)
			}
			v061TestRun(t, v061TestDocument(t,
				v061TestLine(t, "@"+tag+" {", "block-start", 2, 1,
					v061TestElement(t, "tag", "@"+tag, tag, 1),
					v061TestElement(t, "block-open", "{", "{", len(tag)+3)),
				pseudo,
				v061TestLine(t, "}", "block-end", 3, 2, v061TestElement(t, "block-close", "}", "}", 1)),
				v061TestHeading(t, "###", "02", "02"), v061TestText(t, "Read.", "Read.", 6, 1)), "current", nil)
		})
	}
	t.Run("escaped heading", func(t *testing.T) {
		v061TestRun(t, v061TestDocument(t,
			v061TestText(t, `\### 2`, "### 2", 2, 1),
			v061TestHeading(t, "###", "02", "02"), v061TestText(t, "Read.", "Read.", 4, 1)), "current", nil)
	})
	t.Run("ordinary content numbers", func(t *testing.T) {
		v061TestRun(t, v061TestDocument(t,
			v061TestText(t, "Read 2.", "Read 2.", 2, 1),
			v061TestHeading(t, "###", "02", "02"), v061TestText(t, "Read.", "Read.", 4, 1)), "current", nil)
	})
}

func TestV061EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v061TestPair(t, "2", "02")
			for i := range result.Lines {
				if ending != "" || i == len(result.Lines)-1 {
					result.Lines[i].LineEnding = ending
				}
			}
			v061TestRun(t, result, "first", [][6]int{{5, 5, 7, 3, 5, 6}})
			v061TestRun(t, result, "second", [][6]int{{5, 5, 7, 3, 5, 6}})
			// Другой документ с единственным вхождением не наследует числа.
			v061TestRun(t, v061TestDocument(t, v061TestHeading(t, "###", "2", "2"),
				v061TestText(t, "Read.", "Read.", 3, 1)), "current", nil)
		})
	}
}

func TestV061EmptyAndVersions(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("known count "+strconv.FormatBool(known), func(t *testing.T) {
			result := v061TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v061TestRun(t, result, "current", nil)
		})
	}
	for _, version := range []string{"1.1", "1.3", "1.2.1", ""} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v061TestDocument(t)
			result.Lines = []model.Line{}
			result.Document.Metadata.Title = nil
			result.Document.LineCount = nil
			result.Document.DSLVersion = &version
			before := v061TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v061TestRegistry(t))
			if err == nil || checks != nil || v061TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

// v061TestPair задаёт два полноценных шага; их заголовки имеют номера 3 и 5.
func v061TestPair(t *testing.T, first, second string) *model.Result {
	t.Helper()
	return v061TestDocument(t,
		v061TestHeading(t, "###", first, first), v061TestText(t, "Read.", "Read.", 3, 1),
		v061TestHeading(t, "###", second, second), v061TestText(t, "Write.", "Write.", 5, 1))
}

// v061TestDocument добавляет версию и заголовок, сохраняя явно заданные
// элементы и родителей. Для unparsed создаёт обязательные исходные ошибки.
func v061TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v061TestLine(t, "@dsl-version 1.2", "tag", 0, 0,
			v061TestElement(t, "tag", "@dsl-version", "dsl-version", 1),
			v061TestElement(t, "version", "1.2", "1.2", 14)),
		v061TestHeading(t, "#", "Lesson", "Lesson"),
	}, body...)
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{},
	}
	if lines[count-1].Raw != "" {
		lines[count-1].LineEnding = ""
	}
	for i := range lines {
		lines[i].Number = i + 1
		for j, element := range lines[i].Elements {
			if element.ElementType == "unparsed" {
				v061TestParserError(t, result, i+1, j, "Запрещённый вложенный заголовок.")
			}
		}
	}
	return result
}

// v061TestParserError связывает исходную ошибку с элементом либо всей строкой
// при index=-1. Она не является ожидаемым результатом проверки V061.
func v061TestParserError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		element := &line.Elements[index]
		scope, start, end = diagnostics.ScopeElement, element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: scope,
		Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}},
	})
}

func v061TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v061TestElement задаёт исходный диапазон элемента, исключая правую границу.
func v061TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

func v061TestHeading(t *testing.T, marker, rawTitle, title string) model.Line {
	t.Helper()
	return v061TestLine(t, marker+" "+rawTitle, "heading", 0, 0,
		v061TestElement(t, "heading-level", marker, strconv.Itoa(len(marker)), 1),
		v061TestElement(t, "title", rawTitle, title, len(marker)+2))
}

func v061TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v061TestLine(t, raw, "content", parent, depth, v061TestElement(t, "content", raw, value, 1))
}

func v061TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v061TestRun выполняет полный каталог. Ожидания заданы шестёрками:
// строка/начало/конец повтора, строка/начало/конец первого вхождения.
// Все правые границы исключаются; сравниваются только новые V061.
func v061TestRun(t *testing.T, result *model.Result, processingID string, want [][6]int) {
	t.Helper()
	before := v061TestSnapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v061TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v061TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[len(original.Diagnostics):] {
		if entry.DiagnosticCode == "V061" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V061 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position[0], Column: position[1]},
			End: model.Position{Line: position[0], Column: position[2]}}
		related := model.Location{Start: model.Position{Line: position[3], Column: position[4]},
			End: model.Position{Line: position[3], Column: position[5]}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 1 || entry.RelatedLocations[0] != related {
			t.Fatalf("occurrence %d = %+v, want location %+v and related %+v", i, entry, location, related)
		}
		if entry.ID == "" || entry.Source != processingID || entry.SeverityLevel != "recommendation" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Число в заголовках шагов повторяется." {
			t.Fatalf("unexpected V061 metadata: %+v", entry)
		}
	}
	// CheckModel проверяет уникальность ID, ссылки и флаги всех диагностик.
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v061TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or processing length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("element count changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v061TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

func v061TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v061TestSource(t *testing.T, result *model.Result, want string) {
	t.Helper()
	var source strings.Builder
	for _, line := range result.Lines {
		source.WriteString(line.Raw)
		source.WriteString(string(line.LineEnding))
	}
	if source.String() != want {
		t.Fatalf("source = %q, want %q", source.String(), want)
	}
}

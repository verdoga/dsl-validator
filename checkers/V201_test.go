package checkers

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV201RegistrationAndVersions(t *testing.T) {
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V201" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "element" || definition.Check == nil ||
			definition.Message != "Не все буквы заголовка первого уровня прописные." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V201 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3"} {
		t.Run(version, func(t *testing.T) {
			if got, err := registry.ForVersion(dslversions.Version(version)); err == nil || got != nil {
				t.Fatal("unsupported version accepted")
			}
			result := v201Document(t)
			result.Document.DSLVersion = &version
			v201Refusal(t, result)
		})
	}
}

func TestV201Titles(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"reference", "НОВЫЙ УРОК", "НОВЫЙ УРОК", nil},
		{"example", "Новый Урок", "Новый Урок", [][3]int{{2, 3, 13}}},
		{"one lowercase letter", "а", "а", [][3]int{{2, 3, 4}}},
		{"one uppercase letter", "А", "А", nil},
		{"first letter", "аАА", "аАА", [][3]int{{2, 3, 6}}},
		{"last letter", "ААа", "ААа", [][3]int{{2, 3, 6}}},
		{"many violations one finding", "новый урок", "новый урок", [][3]int{{2, 3, 13}}},
		{"no word exceptions", "УРОК и ТЕМА", "УРОК и ТЕМА", [][3]int{{2, 3, 14}}},
		{"no cased letters", "中文 123 !", "中文 123 !", nil},
		{"punctuation only", "!?", "!?", nil},
		{"uppercase with uncased characters", "ABC 中文 123 !", "ABC 中文 123 !", nil},
		{"titlecase", "ǅ", "ǅ", [][3]int{{2, 3, 4}}},
		{"uppercase digraph", "Ǆ", "Ǆ", nil},
		{"other lowercase property", "ʰ", "ʰ", [][3]int{{2, 3, 4}}},
		{"lowercase without simple uppercase mapping", "ß", "ß", [][3]int{{2, 3, 4}}},
		{"supplementary uppercase", "𐐀", "𐐀", nil},
		{"supplementary lowercase", "𐐨", "𐐨", [][3]int{{2, 3, 4}}},
		{"Unicode columns", "Я😀а", "Я😀а", [][3]int{{2, 3, 6}}},
		{"uppercase combining sequence", "A\u0301", "A\u0301", nil},
		{"lowercase combining sequence", "a\u0301", "a\u0301", [][3]int{{2, 3, 5}}},
		{"tab inside title", "A\tб", "A\tб", [][3]int{{2, 3, 6}}},
		{"Unicode spaces", "А\u00a0\u2003б", "А\u00a0\u2003б", [][3]int{{2, 3, 7}}},
		// DSL 1.21–1.22: Value уже содержит снятое экранирование, позиции относятся к Raw.
		{"decoded value original range", `\{Новый\}`, "{Новый}", [][3]int{{2, 3, 12}}},
		{"decoded uppercase", `\{НОВЫЙ\}`, "{НОВЫЙ}", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v201Run(t, v201Document(t, v201Heading(t, "#", tc.raw, tc.value)), "current", tc.want)
		})
	}
}

func TestV201ContextAndRanges(t *testing.T) {
	t.Run("source indentation and trailing whitespace", func(t *testing.T) {
		heading := v201Line(t, " \t#  Новый  Урок \t", "heading", 0, 0,
			v201Element(t, "heading-level", "#", "1", 3), v201Element(t, "title", "Новый  Урок", "Новый  Урок", 6))
		v201Run(t, v201Document(t, heading), "current", [][3]int{{2, 6, 17}})
	})
	t.Run("other levels body internal heading and literal block", func(t *testing.T) {
		result := v201Document(t, v201Heading(t, "#", "УРОК", "УРОК"),
			v201Heading(t, "##", "Подзаголовок", "Подзаголовок"), v201Heading(t, "###", "Шаг", "Шаг"),
			v201Line(t, "строчный текст", "content", 4, 1, v201Element(t, "content", "строчный текст", "строчный текст", 1)),
			v201Line(t, "@header Внутренний", "tag", 4, 1, v201Element(t, "tag", "@header", "header", 1), v201Element(t, "title", "Внутренний", "Внутренний", 9)),
			v201Line(t, "@editor {", "block-start", 4, 1, v201Element(t, "tag", "@editor", "editor", 1), v201Element(t, "block-open", "{", "{", 9)),
			v201Line(t, "# Новый", "content", 7, 2, v201Element(t, "content", "# Новый", "# Новый", 1)),
			v201Line(t, "}", "block-end", 7, 2, v201Element(t, "block-close", "}", "}", 1)))
		subtitle := "Подзаголовок"
		result.Document.Metadata.Subtitle = &subtitle
		v201Run(t, result, "current", nil)
	})
	t.Run("body does not change heading finding", func(t *testing.T) {
		v201Run(t, v201Document(t, v201Heading(t, "#", "Новый Урок", "Новый Урок"),
			v201Line(t, "строчный текст", "content", 2, 1, v201Element(t, "content", "строчный текст", "строчный текст", 1))), "current", [][3]int{{2, 3, 13}})
	})
	t.Run("no available lines", func(t *testing.T) { v201Run(t, v201Document(t), "current", nil) })
	for _, eols := range [][2]model.LineEnding{{"\n", "\n"}, {"\r\n", "\r\n"}, {"\n", ""}, {"\r\n", ""}, {"\r\n", "\n"}} {
		t.Run("EOL/"+string(eols[0])+"/"+string(eols[1]), func(t *testing.T) {
			result := v201Document(t, v201Heading(t, "#", "Новый Урок", "Новый Урок"))
			result.Lines[0].LineEnding, result.Lines[1].LineEnding = eols[0], eols[1]
			v201Run(t, result, "current", [][3]int{{2, 3, 13}})
		})
	}
}

func TestV201AdmissionAndHistory(t *testing.T) {
	t.Run("nested heading refused before checks", func(t *testing.T) {
		result := v201Document(t, v201Heading(t, "#", "УРОК", "УРОК"), v201Heading(t, "###", "Шаг", "Шаг"))
		parent := 2
		result.Lines[2].ParentLine, result.Lines[2].NestingLevel = &parent, 1
		v201Refusal(t, result)
	})
	t.Run("null title refused before checks", func(t *testing.T) {
		result := v201Document(t, v201Heading(t, "#", "Новый", "Новый"))
		result.Lines[1].Elements[1].Value = nil
		v201Refusal(t, result)
	})
	t.Run("separate runs and documents", func(t *testing.T) {
		result := v201Document(t, v201Heading(t, "#", "Новый Урок", "Новый Урок"))
		v201Run(t, result, "first", [][3]int{{2, 3, 13}})
		v201Run(t, result, "second", [][3]int{{2, 3, 13}})
		v201Run(t, v201Document(t, v201Heading(t, "#", "НОВЫЙ УРОК", "НОВЫЙ УРОК")), "first", nil)
	})
}

// v201Element задаёт входной элемент; ожидаемые диапазоны в тестах заданы отдельно.
func v201Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v201Line сохраняет явно заданную структуру строки, не разбирая DSL.
func v201Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

func v201Heading(t *testing.T, marker, raw, value string) model.Line {
	t.Helper()
	level := map[string]string{"#": "1", "##": "2", "###": "3"}[marker]
	return v201Line(t, marker+" "+raw, "heading", 0, 0, v201Element(t, "heading-level", marker, level, 1), v201Element(t, "title", raw, value, len(marker)+2))
}

// v201Document дополняет фрагменты только обязательной первой строкой DSL 1.2.
// Без строк моделируется недоступный исходник с известной версией, а не выдуманный DSL.
func v201Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version := "1.2"
	result := &model.Result{Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
	if len(body) == 0 {
		return result
	}
	result.Lines = append([]model.Line{v201Line(t, "@dsl-version 1.2", "tag", 0, 0,
		v201Element(t, "tag", "@dsl-version", "dsl-version", 1), v201Element(t, "version", "1.2", "1.2", 14))}, body...)
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	count := len(result.Lines)
	result.Document.LineCount, result.Document.Metadata.Title = &count, body[0].Elements[1].Value
	return result
}

func v201Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func v201Refusal(t *testing.T, result *model.Result) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before := v201Snapshot(t, result)
	if checks, err := validator.Prepare(result, registry); err == nil || checks != nil {
		t.Fatal("Prepare must refuse this model")
	}
	if v201Snapshot(t, result) != before {
		t.Fatal("refused input changed")
	}
}

// v201Run проверяет V201 текущего запуска через полный штатный каталог.
// want содержит явные тройки: строка, включаемое начало, исключающий конец.
func v201Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before := v201Snapshot(t, result)
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatal(err)
	}
	if v201Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatal(err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V201" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V201 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, entry := range got {
		location := model.Location{Start: model.Position{Line: want[i][0], Column: want[i][1]}, End: model.Position{Line: want[i][0], Column: want[i][2]}}
		if entry.ID == "" || entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "Не все буквы заголовка первого уровня прописные." || entry.Location == nil || *entry.Location != location ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected V201: %+v, want %+v", entry, location)
		}
		links := 0
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if !slices.Contains(element.ErrorIDs, entry.ID) {
					continue
				}
				links++
				if element.ElementType != "title" || line.Number != want[i][0] || element.Start != want[i][1] || element.End != want[i][2] {
					t.Fatal("V201 linked to another element")
				}
			}
		}
		if links != 1 {
			t.Fatalf("V201 links = %d, want 1", links)
		}
	}
	// История сохраняется; допустимые дополнения всех диагностик не считаются изменением исходника.
	var previous, restored model.Result
	if err := json.Unmarshal([]byte(before), &previous); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v201Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if v201Snapshot(t, &model.Result{Diagnostics: restored.Diagnostics[:len(previous.Diagnostics)]}) != v201Snapshot(t, &model.Result{Diagnostics: previous.Diagnostics}) {
		t.Fatal("diagnostic history changed")
	}
	restored.Diagnostics, restored.Processing, restored.Document.HasErrors = previous.Diagnostics, previous.Processing, previous.Document.HasErrors
	for i := range restored.Lines {
		restored.Lines[i].HasErrors = previous.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			for _, id := range previous.Lines[i].Elements[j].ErrorIDs {
				if !slices.Contains(restored.Lines[i].Elements[j].ErrorIDs, id) {
					t.Fatal("old element link removed")
				}
			}
			restored.Lines[i].Elements[j].ErrorIDs = previous.Lines[i].Elements[j].ErrorIDs
		}
	}
	if v201Snapshot(t, &restored) != before {
		t.Fatal("source model changed")
	}
}

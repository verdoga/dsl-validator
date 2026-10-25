package checkers

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV149Registration(t *testing.T) {
	definitions, err := v149Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V149" {
			continue
		}
		count++
		if definition.Message != "Между пропуском и открывающей скобкой ответа есть пробельные символы." ||
			definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V149 definitions = %d, want 1", count)
	}
}

func TestV149ReferenceAndExample(t *testing.T) {
	// К дословным эталону и примеру добавлены только обязательные версия
	// и заголовок по DSL 1.1–1.3, 1.30. Строка содержимого получает номер 4.
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"reference", "She _____{lives} here.", nil},
		{"example", "She _____ {lives} here.", [][3]int{{4, 10, 11}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v149Run(t, v149Document(t, v149Open(t, "multifill", 2, 1),
				v149Text(t, tc.raw, tc.raw, 3, 2), v149Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV149RawPatterns(t *testing.T) {
	// Value задаётся отдельно там, где разрешённое экранирование изменяет текст.
	// Пустое поле value в таблице означает неизменённое значение raw.
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"one space", "_____ {A}", "", [][3]int{{4, 6, 7}}},
		{"three spaces", "_____   {A}", "", [][3]int{{4, 6, 9}}},
		{"Unicode spaces", "_____\u00a0\u2003{A}", "", [][3]int{{4, 6, 8}}},
		{"tab", "_____\t{A}", "", [][3]int{{4, 6, 7}}},
		{"adjacent alternatives", "_____{A}{B}", "", nil},
		{"many adjacent alternatives", "_____{A}{B}{C}{D}{E}", "", nil},
		{"space between alternatives", "_____{A} {B}", "", [][3]int{{4, 9, 10}}},
		{"first and second group separated", "_____ {A} {B}", "", [][3]int{{4, 6, 7}, {4, 10, 11}}},
		{"spaces and tab between groups", "_____{A}  {B}\t{C}", "", [][3]int{{4, 9, 11}, {4, 14, 15}}},
		{"later alternative", "_____{A}{B} {C}", "", [][3]int{{4, 12, 13}}},
		{"text ends chain", "_____{A} text {B}", "", nil},
		{"punctuation ends chain", "_____{A}, {B}", "", nil},
		{"two placeholders", "_____ {A} _____ {B}", "", [][3]int{{4, 6, 7}, {4, 16, 17}}},
		{"ordinary then checked placeholder", "_____ text _____ {B}", "", [][3]int{{4, 17, 18}}},
		{"one underscore", "_ {A}", "", nil},
		{"four underscores", "____ {A}", "", nil},
		{"six underscores", "______ {A}", "", nil},
		{"ten underscores", "__________ {A}", "", nil},
		{"no opening", "_____   ", "", nil},
		{"no placeholder", "text {A} {B}", "", nil},
		{"escaped run", `\_____ {A}`, "_____ {A}", nil},
		{"escaped longer run", `\______ {A}`, "______ {A}", nil},
		{"fully literal form", `\_____\{A\}`, "_____{A}", nil},
		{"paired backslashes", `\\_____ {A}`, `\_____ {A}`, [][3]int{{4, 8, 9}}},
		{"three backslashes", `\\\_____ {A}`, `\_____ {A}`, nil},
		{"escaped remainder of run", `_\_____ {A}`, "______ {A}", nil},
		{"escaped first brace", `_____ \{A}`, "_____ {A}", nil},
		{"escaped alternative brace", `_____{A} \{B}`, "_____{A} {B}", nil},
		{"escaped braces break chain", `_____\{A\} {B}`, "_____{A} {B}", nil},
		{"answer interior", "_____{_____ {A}}", "", nil},
		{"malformed answer interior", "_____ {_____ {A}}", "", [][3]int{{4, 6, 7}}},
		{"alternative interior", "_____{A} {_____ {B}}", "", [][3]int{{4, 9, 10}}},
		{"escaped close is data", `_____{\}_____ {A}}`, "_____{}_____ {A}}", nil},
		{"escaped slash allows close", `_____{A\\} {B}`, `_____{A\} {B}`, [][3]int{{4, 11, 12}}},
		{"first close without nesting", "_____{A{B} _____ {C}", "", [][3]int{{4, 17, 18}}},
		{"empty first answer", "_____{} {B}", "", [][3]int{{4, 8, 9}}},
		{"unclosed separated answer", "_____ {A", "", [][3]int{{4, 6, 7}}},
		{"unclosed alternative", "_____{A} {B", "", [][3]int{{4, 9, 10}}},
		{"Unicode prefix", "Я🙂 _____ {A}", "", [][3]int{{4, 9, 10}}},
		{"indent and trailing whitespace", " \t_____ {A} \t", "", [][3]int{{4, 8, 9}}},
		{"zero width space is not White_Space", "_____\u200b{A}", "", nil},
		{"BOM is not White_Space", "_____\ufeff{A}", "", nil},
		{"zero width space breaks alternatives", "_____{A} \u200b {B}", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.value
			if value == "" {
				value = tc.raw
			}
			v149Run(t, v149Document(t, v149Open(t, "multifill", 2, 1),
				v149Text(t, tc.raw, value, 3, 2), v149Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV149WhiteSpaceCharacters(t *testing.T) {
	// LF и CR представлены окончаниями физических строк в отдельных тестах.
	// Остальные White_Space проверяются как исходные символы промежутка.
	for _, gap := range []string{"\t", "\v", "\f", " ", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006", "\u2007",
		"\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"} {
		t.Run(fmt.Sprintf("%U", []rune(gap)[0]), func(t *testing.T) {
			raw := "_____" + gap + "{A}" + gap + "{B}"
			v149Run(t, v149Document(t, v149Open(t, "multifill", 2, 1), v149Text(t, raw, raw, 3, 2),
				v149Close(t, 3, 2)), "current", [][3]int{{4, 6, 7}, {4, 10, 11}})
		})
	}
}

func TestV149InitialAnswerAndPseudotags(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"answer excluded", "@answer _____ {A}", "", nil},
		{"indented mixed case answer", " \t@AnSwEr _____ {A}", "", nil},
		{"answer at end", "@answer", "", nil},
		{"tab is not name boundary", "@answer\t_____ {A}", "", [][3]int{{4, 14, 15}}},
		{"NBSP is not name boundary", "@answer\u00a0_____ {A}", "", [][3]int{{4, 14, 15}}},
		{"brace is not name boundary", "@answer{_____ {A}}", "", [][3]int{{4, 14, 15}}},
		{"longer name", "@answerText _____ {A}", "", [][3]int{{4, 18, 19}}},
		{"escaped answer", `\@answer _____ {A}`, "@answer _____ {A}", [][3]int{{4, 15, 16}}},
		{"hint is text", "@hint _____ {A}", "", [][3]int{{4, 12, 13}}},
		{"editor is text", "@editor _____ {A}", "", [][3]int{{4, 14, 15}}},
		{"pseudo block is text", "@multifill _____ {A}", "", [][3]int{{4, 17, 18}}},
		{"heading is text", "# _____ {A}", "", [][3]int{{4, 8, 9}}},
		{"mention in middle", "See @answer _____ {A}", "", [][3]int{{4, 18, 19}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.value
			if value == "" {
				value = tc.raw
			}
			v149Run(t, v149Document(t, v149Open(t, "multifill", 2, 1),
				v149Text(t, tc.raw, value, 3, 2), v149Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV149PhysicalLines(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][3]int
	}{
		{"newline after placeholder", "_____", "{A}", nil},
		{"trailing spaces before newline", "_____  ", " {A}", nil},
		{"newline between alternatives", "_____{A}", " {B}", nil},
		{"open answer reset", "_____{A", "_____ {B}", [][3]int{{5, 6, 7}}},
		{"malformed answer reset", "_____ {A", "_____ {B}", [][3]int{{4, 6, 7}, {5, 6, 7}}},
		{"trailing backslash reset", `_____{A\`, "_____ {B}", [][3]int{{5, 6, 7}}},
	} {
		for _, ending := range []model.LineEnding{"\n", "\r\n"} {
			t.Run(tc.name+fmt.Sprintf(" %q", ending), func(t *testing.T) {
				result := v149Document(t, v149Open(t, "multifill", 2, 1), v149Text(t, tc.first, tc.first, 3, 2),
					v149Text(t, tc.second, tc.second, 3, 2), v149Close(t, 3, 2))
				for i := 0; i < len(result.Lines)-1; i++ {
					result.Lines[i].LineEnding = ending
				}
				v149Run(t, result, "current", tc.want)
			})
		}
	}
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(fmt.Sprintf("example final ending %q", ending), func(t *testing.T) {
			result := v149Document(t, v149Open(t, "multifill", 2, 1),
				v149Text(t, "She _____ {lives} here.", "She _____ {lives} here.", 3, 2), v149Close(t, 3, 2))
			result.Lines[4].LineEnding = ending
			v149Run(t, result, "current", [][3]int{{4, 10, 11}})
		})
	}
}

func TestV149ElementRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name string
		line model.Line
		want [][3]int
	}{
		{"entire line unparsed", v149Line(t, "She _____ {lives} here.", "invalid", 3, 2,
			v149Unparsed(t, "She _____ {lives} here.", 1, 24)), [][3]int{{4, 10, 11}}},
		{"split around unparsed", v149Line(t, "She _____ {lives} here.", "invalid", 3, 2,
			v149Element(t, "content", "She ", "She ", 1, 5),
			model.Element{ElementType: "placeholder", Raw: "_____", Start: 5, End: 10, ErrorIDs: []string{}},
			v149Unparsed(t, " {lives}", 10, 18), v149Element(t, "content", " here.", " here.", 18, 24)), [][3]int{{4, 10, 11}}},
		{"recognized placeholder then malformed alternative", v149Line(t, "_____{A} {B}", "invalid", 3, 2,
			v149Element(t, "placeholder", "_____{A}", "A", 1, 9), v149Unparsed(t, " {B}", 9, 13)), [][3]int{{4, 9, 10}}},
		{"reference split into elements", v149Line(t, "She _____{lives} here.", "content", 3, 2,
			v149Element(t, "content", "She ", "She ", 1, 5), v149Element(t, "placeholder", "_____{lives}", "lives", 5, 17),
			v149Element(t, "content", " here.", " here.", 17, 23)), nil},
		{"forbidden answer unparsed", v149Line(t, "@answer _____ {A}", "invalid", 3, 2,
			v149Unparsed(t, "@answer _____ {A}", 1, 18)), nil},
		{"forbidden answer with saved tag", v149Line(t, "@answer _____ {A}", "invalid", 3, 2,
			v149Element(t, "tag", "@answer", "answer", 1, 8), v149Unparsed(t, " _____ {A}", 8, 18)), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v149Run(t, v149Document(t, v149Open(t, "multifill", 2, 1), tc.line, v149Close(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV149ContextsAndBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][3]int
	}{
		{"outside multifill", []model.Line{v149Text(t, "_____ {A}", "_____ {A}", 2, 1)}, nil},
		{"single line multifill", []model.Line{v149Tag(t, "multifill", "content", "_____ {A}", 2, 1)}, nil},
		{"instruction is excluded", []model.Line{
			v149Line(t, "@multifill _____ {A} {", "block-start", 2, 1,
				v149Element(t, "tag", "@multifill", "multifill", 1, 11),
				v149Element(t, "content", "_____ {A}", "_____ {A}", 12, 21), v149Element(t, "block-open", "{", "{", 22, 23)),
			v149Text(t, "Read.", "Read.", 3, 2), v149Close(t, 3, 2)}, nil},
		{"instruction does not hide body", []model.Line{
			v149Line(t, "@multifill Write. {", "block-start", 2, 1,
				v149Element(t, "tag", "@multifill", "multifill", 1, 11),
				v149Element(t, "content", "Write.", "Write.", 12, 18), v149Element(t, "block-open", "{", "{", 19, 20)),
			v149Text(t, "_____ {A}", "_____ {A}", 3, 2), v149Close(t, 3, 2)}, [][3]int{{4, 6, 7}}},
		{"editor block", []model.Line{v149Open(t, "editor", 2, 1), v149Text(t, "_____ {A}", "_____ {A}", 3, 2), v149Close(t, 3, 2)}, nil},
		{"pseudo multifill in editor", []model.Line{v149Open(t, "editor", 2, 1),
			v149Text(t, "@multifill {", "@multifill {", 3, 2), v149Text(t, "_____ {A}", "_____ {A}", 3, 2), v149Close(t, 3, 2)}, nil},
		{"note block", []model.Line{v149Open(t, "note", 2, 1), v149Text(t, "_____ {A}", "_____ {A}", 3, 2), v149Close(t, 3, 2)}, nil},
		{"task context", []model.Line{v149Tag(t, "task", "identifier", "t", 2, 1), v149Open(t, "multifill", 3, 2),
			v149Text(t, "She _____ {lives} here.", "She _____ {lives} here.", 4, 3), v149Close(t, 4, 3),
			v149Tag(t, "endtask", "", "", 3, 2)}, [][3]int{{5, 10, 11}}},
		{"case of multifill", []model.Line{v149Open(t, "MuLtIfIlL", 2, 1),
			v149Text(t, "_____ {A}", "_____ {A}", 3, 2), v149Close(t, 3, 2)}, [][3]int{{4, 6, 7}}},
		{"first and last blocks", []model.Line{v149Open(t, "multifill", 2, 1),
			v149Text(t, "_____ {A}", "_____ {A}", 3, 2), v149Close(t, 3, 2), v149Open(t, "multifill", 2, 1),
			v149Text(t, "_____ {B}", "_____ {B}", 6, 2), v149Close(t, 6, 2)}, [][3]int{{4, 6, 7}, {7, 6, 7}}},
		{"state cannot cross blocks", []model.Line{v149Open(t, "multifill", 2, 1),
			v149Text(t, "_____{A", "_____{A", 3, 2), v149Close(t, 3, 2), v149Open(t, "multifill", 2, 1),
			v149Text(t, "_____ {B}", "_____ {B}", 6, 2), v149Close(t, 6, 2)}, [][3]int{{7, 6, 7}}},
		{"blank body", []model.Line{v149Open(t, "multifill", 2, 1), v149Line(t, "", "blank", 3, 2), v149Close(t, 3, 2)}, nil},
		{"end of file inside block", []model.Line{v149Open(t, "multifill", 2, 1),
			v149Text(t, "_____ {A}", "_____ {A}", 3, 2)}, [][3]int{{4, 6, 7}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v149Run(t, v149Document(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("task stage within branch", func(t *testing.T) {
		v149Run(t, v149Document(t, v149Open(t, "variants", 2, 1), v149Tag(t, "variant", "name", "A", 3, 2),
			v149Tag(t, "task", "identifier", "t", 4, 3), v149Tag(t, "step", "title", "Part", 5, 4),
			v149Open(t, "multifill", 6, 5), v149Text(t, "_____ {A} {B}", "_____ {A} {B}", 7, 6), v149Close(t, 7, 6),
			v149Tag(t, "endtask", "", "", 5, 4), v149Tag(t, "variant", "name", "B", 3, 2),
			v149Text(t, "Read.", "Read.", 11, 3), v149Close(t, 3, 2)), "current", [][3]int{{8, 6, 7}, {8, 10, 11}})
	})
}

func TestV149DoesNotSuppressV005(t *testing.T) {
	// По уточнению пользователя V005 продолжает независимо считать скобки.
	// Её стек после последней строки сохраняет открытие multifill и первую
	// скобку ошибочного шаблона; V149 отмечает лишь промежуток перед шаблоном.
	result := v149Document(t, v149Open(t, "multifill", 2, 1),
		v149Line(t, "_____ {A{B", "invalid", 3, 2, v149Unparsed(t, "_____ {A{B", 1, 11)), v149Close(t, 3, 2))
	v149Run(t, result, "current", [][3]int{{4, 6, 7}})
	var got []model.Location
	for _, entry := range result.Diagnostics {
		if entry.Source != "current" || entry.DiagnosticCode != "V005" {
			continue
		}
		if entry.Location == nil || entry.Message != "Нарушено оформление фигурных скобок.\nконкретизация: Открывающая скобка не имеет закрывающей" {
			t.Fatalf("unexpected V005: %+v", entry)
		}
		got = append(got, *entry.Location)
	}
	want := []model.Location{
		{Start: model.Position{Line: 3, Column: 12}, End: model.Position{Line: 3, Column: 13}},
		{Start: model.Position{Line: 4, Column: 7}, End: model.Position{Line: 4, Column: 8}},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("V005 locations = %+v, want %+v", got, want)
	}
}

func TestV149EmptyVersionsAndHistory(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+fmt.Sprint(known), func(t *testing.T) {
			result := v149Empty(t, "1.2")
			if known {
				count := 0
				result.Document.LineCount = &count
			}
			v149Run(t, result, "current", nil)
		})
	}
	t.Run("only required prefix", func(t *testing.T) {
		v149Run(t, v149Document(t), "current", nil)
	})
	for _, version := range []string{"1.1", "1.3", "1.21"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v149Empty(t, version)
			before := v149Snapshot(t, result)
			checks, err := validator.Prepare(result, v149Registry(t))
			if err == nil || checks != nil || v149Snapshot(t, result) != before {
				t.Fatalf("Prepare(%q) = %v, %v; want unchanged input and refusal", version, checks, err)
			}
		})
	}
	t.Run("previous results retained separately", func(t *testing.T) {
		result := v149Document(t, v149Open(t, "multifill", 2, 1),
			v149Text(t, "_____{A} {B}", "_____{A} {B}", 3, 2), v149Close(t, 3, 2))
		v149Run(t, result, "first", [][3]int{{4, 9, 10}})
		v149Run(t, result, "second", [][3]int{{4, 9, 10}})
	})
}

// v149Registry использует полный штатный каталог без подмены регистраций.
func v149Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v149Empty создаёт модель с недоступными исходными строками.
func v149Empty(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v149Document добавляет обязательный префикс, сохраняя явно заданную структуру тела.
// Для unparsed добавляется согласованная исходная диагностика парсера.
func v149Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v149Empty(t, "1.2")
	title := "Lesson"
	result.Document.Metadata.Title = &title
	result.Lines = append([]model.Line{v149Tag(t, "dsl-version", "version", "1.2", 0, 0),
		v149Line(t, "# Lesson", "heading", 0, 0, v149Element(t, "heading-level", "#", "1", 1, 2),
			v149Element(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType != "unparsed" {
				continue
			}
			id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
				Message: "Сохранённая ошибка разбора.", SeverityLevel: "error", DiagnosticScope: "element",
				Location:         &model.Location{Start: model.Position{Line: i + 1, Column: element.Start}, End: model.Position{Line: i + 1, Column: element.End}},
				RelatedLocations: []model.Location{}})
		}
	}
	result.Lines[count-1].LineEnding = ""
	return result
}

// v149Line сохраняет тип, родителей и элементы без разбора исходной строки.
func v149Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v149Element задаёт независимые Raw и Value с исходным Unicode-диапазоном [start,end).
func v149Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v149Unparsed задаёт неразобранный участок; его историю добавляет v149Document.
func v149Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v149Tag собирает объявление с явно указанным видом единственного аргумента.
func v149Tag(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + name
	elements := []model.Element{v149Element(t, "tag", raw, strings.ToLower(name), 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v149Element(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v149Line(t, raw, "tag", parent, depth, elements...)
}

// v149Text сохраняет независимо заданные исходный и текстовый варианты содержимого.
func v149Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v149Line(t, raw, "content", parent, depth, v149Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v149Open задаёт объявление без инструкции с открывающей скобкой блока.
func v149Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v149Line(t, "@"+name+" {", "block-start", parent, depth,
		v149Element(t, "tag", "@"+name, strings.ToLower(name), 1, len(name)+2),
		v149Element(t, "block-open", "{", "{", len(name)+3, len(name)+4))
}

// v149Close задаёт отдельную строку закрытия указанного блока.
func v149Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v149Line(t, "}", "block-end", parent, depth, v149Element(t, "block-close", "}", "}", 1, 2))
}

// v149Run выполняет полный конвейер и проверяет только текущие V149.
// Ожидания — явно заданные тройки: строка, начало и исключающий конец с 1.
func v149Run(t *testing.T, result *model.Result, source string, want [][3]int) {
	t.Helper()
	before := v149Snapshot(t, result)
	checks, err := validator.Prepare(result, v149Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v149Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldDiagnostics, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	targetIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldDiagnostics:] {
		if entry.Source != source {
			t.Fatalf("new diagnostic source = %q, want %q", entry.Source, source)
		}
		if entry.DiagnosticCode == "V149" {
			got = append(got, entry)
			targetIDs[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V149 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		main := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]}, End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != main || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v and no related ranges", i, entry.Location, entry.RelatedLocations, main)
		}
		if entry.ID == "" || entry.Source != source || entry.DiagnosticCode != "V149" || entry.SeverityLevel != "error" ||
			entry.DiagnosticScope != "line" || entry.Fatal || entry.Message != "Между пропуском и открывающей скобкой ответа есть пробельные символы.\nконкретизация: Запишите ответ непосредственно после пропуска: _____{ANSWER}" {
			t.Errorf("unexpected occurrence: %+v", entry)
		}
		if !result.Document.HasErrors || !result.Lines[expected[0]-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v149Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Lines) != len(original.Lines) || len(actual.Processing) != oldProcessing+1 {
		t.Fatal("source or history length changed")
	}
	actual.Diagnostics, actual.Processing = actual.Diagnostics[:oldDiagnostics], actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		if len(actual.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			for _, id := range element.ErrorIDs {
				if targetIDs[id] {
					t.Errorf("ScopeLine V149 linked to element at %d:%d", i+1, j)
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || !slices.Equal(element.ErrorIDs[:len(oldIDs)], oldIDs) {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v149Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v149Snapshot сохраняет модель для проверки отсутствия побочных изменений.
func v149Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

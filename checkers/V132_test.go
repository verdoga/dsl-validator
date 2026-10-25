package checkers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v132Expected задаёт независимый исходный диапазон, конец исключается.
type v132Expected struct{ line, start, end int }

func TestV132Registration(t *testing.T) {
	definitions, err := v132Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V132" {
			continue
		}
		count++
		if definition.Message != "На @fragment нет ссылок @include." || definition.Severity != "error" ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V132 registrations = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3", "1.21"} {
		t.Run(version, func(t *testing.T) {
			result := v132Unavailable(t, version)
			before := v132Snapshot(t, result)
			checks, err := validator.Prepare(result, v132Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
			if v132Snapshot(t, result) != before {
				t.Fatal("Prepare changed unsupported input")
			}
		})
	}
}

func TestV132ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name string
		last model.Line
		want []v132Expected
	}{
		{"reference", v132Named(t, "include", "identifier", "f", 2, 1), nil},
		{"example", v132Text(t, "Continue.", 2, 1), []v132Expected{{3, 11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Перед исходным примером добавлены только обязательные версия и
			// заголовок (DSL 1.1–1.3, 1.30). Continue. после неиспользуемого
			// определения даёт шагу отображаемое содержимое (3.12.8–3.12.9).
			body := v132Fragment(t, "f", 3, 2, 1)
			body = append(body, tc.last)
			v132Run(t, v132Document(t, body...), "current", tc.want)
		})
	}
}

func TestV132DefinitionsAndReferences(t *testing.T) {
	for _, tc := range []struct {
		name        string
		definitions []string
		references  []string
		want        []v132Expected
	}{
		{"no definitions", nil, nil, nil},
		{"one unused", []string{"f"}, nil, []v132Expected{{3, 11, 12}}},
		{"one reference", []string{"f"}, []string{"f"}, nil},
		{"many references", []string{"f"}, []string{"f", "F", "f"}, nil},
		{"first and last unused", []string{"f", "g", "h"}, []string{"g"}, []v132Expected{{3, 11, 12}, {9, 11, 12}}},
		{"all unused", []string{"f", "g"}, nil, []v132Expected{{3, 11, 12}, {6, 11, 12}}},
		{"duplicate definitions", []string{"f", "f"}, nil, nil},
		{"case duplicate definitions", []string{"f", "F"}, nil, nil},
		{"three duplicates and unique", []string{"f", "F", "f", "g"}, nil, []v132Expected{{12, 11, 12}}},
		{"referenced duplicates", []string{"f", "F"}, []string{"f"}, nil},
		{"other reference", []string{"f"}, []string{"g"}, []v132Expected{{3, 11, 12}}},
		{"Unicode case duplicate", []string{"Код", "код"}, nil, nil},
		{"width does not create duplicate", []string{"Ａ", "A"}, nil, []v132Expected{{3, 11, 12}, {6, 11, 12}}},
		{"full folding does not create duplicate", []string{"ß", "ss"}, nil, []v132Expected{{3, 11, 12}, {6, 11, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v132Definitions(t, tc.definitions...)
			for _, id := range tc.references {
				v132Append(t, result, v132Named(t, "include", "identifier", id, 2, 1))
			}
			v132Run(t, result, "current", tc.want)
		})
	}
	t.Run("reference before definition", func(t *testing.T) {
		body := []model.Line{v132Named(t, "include", "identifier", "F", 2, 1)}
		body = append(body, v132Fragment(t, "f", 4, 2, 1)...)
		v132Run(t, v132Document(t, body...), "current", nil)
	})
	t.Run("new step does not reset references", func(t *testing.T) {
		result := v132Definitions(t, "f")
		v132Append(t, result,
			v132Line(t, "@newpage", "tag", 0, 0, v132Element(t, "tag", "@newpage", "newpage", 1, 9)),
			v132Named(t, "include", "identifier", "f", 7, 1))
		v132Run(t, result, "current", nil)
	})
}

func TestV132IDValues(t *testing.T) {
	for _, tc := range []struct {
		name, definition, reference string
		want                        []v132Expected
	}{
		{"ASCII case", "Ab_1.-", "aB_1.-", nil},
		{"Unicode letters and decimal digits", "Код_١.２", "код_١.２", nil},
		{"unused Unicode range", "Код_١.２", "other", []v132Expected{{3, 11, 18}}},
		{"period", ".", "other", []v132Expected{{3, 11, 12}}},
		{"hyphen", "-", "other", []v132Expected{{3, 11, 12}}},
		{"underscore", "_", "other", []v132Expected{{3, 11, 12}}},
		{"no width normalization", "Ａ", "A", []v132Expected{{3, 11, 12}}},
		{"no full folding", "ß", "SS", []v132Expected{{3, 11, 12}}},
		{"simple sharp S folding", "ß", "ẞ", nil},
		// Прежний тест «no final sigma folding» закреплял найденное пользователем
		// противоречие: V194 разрешал эту ссылку, а V132 считал её отсутствующей.
		// Сравнение ID фрагментов теперь единообразно использует simple folding.
		{"final sigma folding", "Σ", "ς", nil},
		{"lowercase sigma", "Σ", "σ", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v132Definitions(t, tc.definition)
			v132Append(t, result, v132Named(t, "include", "identifier", tc.reference, 2, 1))
			v132Run(t, result, "current", tc.want)
		})
	}
	for _, id := range []string{"", "bad/id", "bad!", "bad id", " f", "f ", "e\u0301", "²", `"f"`, `\f`} {
		t.Run(fmt.Sprintf("invalid definition %q", id), func(t *testing.T) {
			// identifier с ошибочным Value остаётся элементом; ошибка парсера
			// привязана к его точному диапазону, в том числе пустому.
			result := v132Definitions(t, id)
			v132ParserError(t, result, 3, 1)
			v132Run(t, result, "current", nil)
		})
		t.Run(fmt.Sprintf("invalid reference %q", id), func(t *testing.T) {
			result := v132Definitions(t, "f")
			v132Append(t, result, v132Named(t, "include", "identifier", id, 2, 1))
			v132ParserError(t, result, 7, 1)
			v132Run(t, result, "current", []v132Expected{{3, 11, 12}})
		})
	}
	t.Run("invalid ID does not suppress valid definition", func(t *testing.T) {
		result := v132Definitions(t, " f", "f")
		v132ParserError(t, result, 3, 1)
		v132Run(t, result, "current", []v132Expected{{6, 11, 12}})
	})
	t.Run("no NFC normalization", func(t *testing.T) {
		result := v132Definitions(t, "é", "e\u0301")
		v132Append(t, result, v132Named(t, "include", "identifier", "e\u0301", 2, 1))
		// Комбинируемый знак недопустим по DSL 1.9: ни определение, ни ссылка
		// с ним не превращаются в допустимый ID посредством NFC.
		v132ParserError(t, result, 6, 1)
		v132ParserError(t, result, 10, 1)
		v132Run(t, result, "current", []v132Expected{{3, 11, 12}})
	})
}

func TestV132SimpleFoldClasses(t *testing.T) {
	// API identifier допускает сохранённое Value в любом регистре: тесты
	// не предполагают конкретный способ нормализации во внешнем парсере.
	for _, class := range []string{"Σσς", "KkK"} {
		for _, definition := range class {
			for _, reference := range class {
				t.Run(fmt.Sprintf("reference/%c/%c", definition, reference), func(t *testing.T) {
					result := v132Definitions(t, string(definition))
					v132Append(t, result, v132Named(t, "include", "identifier", string(reference), 2, 1))
					v132Run(t, result, "current", nil)
				})
				t.Run(fmt.Sprintf("duplicates/%c/%c", definition, reference), func(t *testing.T) {
					result := v132Definitions(t, string(definition), string(reference), "unique")
					v132Run(t, result, "current", []v132Expected{{9, 11, 17}})
				})
			}
		}
		t.Run("whole duplicate class/"+class, func(t *testing.T) {
			var ids []string
			for _, id := range class {
				ids = append(ids, string(id))
			}
			result := v132Definitions(t, append(ids, "unique")...)
			v132Run(t, result, "current", []v132Expected{{12, 11, 17}})
		})
	}
}

func TestV132V194FragmentResolution(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty fragment %t", empty), func(t *testing.T) {
			body := []model.Line{
				v132Open(t, "variants", 2, 1),
				v132Named(t, "variant", "name", "A", 3, 2),
				v132Named(t, "include", "identifier", "Σ", 4, 3),
				v132Named(t, "variant", "name", "B", 3, 2),
				v132Text(t, "Write.", 6, 3),
				v132Close(t, 3, 2),
			}
			body = append(body, v132Fragment(t, "ς", 9, 2, 1)...)
			if empty {
				body[7] = v132Line(t, "", "blank", 9, 2)
			}
			result := v132Document(t, body...)
			// v132Run использует полный штатный каталог Prepare/Run. Пустое тело
			// обязано дать V194: отсутствие этой ошибки само по себе не отличает
			// непустой разрешённый fragment от неразрешённого include.
			v132Run(t, result, "current", nil)
			var got []model.Diagnostic
			for _, entry := range result.Diagnostics {
				if entry.Source == "current" && entry.DiagnosticCode == "V194" {
					got = append(got, entry)
				}
			}
			if !empty {
				if len(got) != 0 {
					t.Fatalf("nonempty included fragment: unexpected V194: %+v", got)
				}
				return
			}
			location := model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 5, Column: 11}}
			related := model.Location{Start: model.Position{Line: 6, Column: 1}, End: model.Position{Line: 6, Column: 9}}
			if len(got) != 1 || got[0].Location == nil || *got[0].Location != location ||
				len(got[0].RelatedLocations) != 1 || got[0].RelatedLocations[0] != related {
				t.Fatalf("empty included fragment: V194 = %+v, want %v related %v", got, location, related)
			}
			if got[0].SeverityLevel != "error" || got[0].DiagnosticScope != "block" || got[0].Fatal ||
				got[0].Message != "Ветвь @variant не содержит отображаемого материала." {
				t.Errorf("V194 metadata = %+v", got[0])
			}
		})
	}
}

func TestV132OriginalPositions(t *testing.T) {
	for _, used := range []bool{false, true} {
		t.Run(fmt.Sprint(used), func(t *testing.T) {
			result := v132Definitions(t, "Я")
			result.Lines[2] = v132Line(t, "  @FRAGMENT Я {", "block-start", 2, 1,
				v132Element(t, "tag", "@FRAGMENT", "fragment", 3, 12),
				v132Element(t, "identifier", "Я", "я", 13, 14),
				v132Element(t, "block-open", "{", "{", 15, 16))
			result.Lines[2].Number = 3
			want := []v132Expected{{3, 13, 14}}
			if used {
				// Канонические Value и исходные Raw различаются только регистром.
				v132Append(t, result, v132Line(t, "\t@INCLUDE\tЯ", "tag", 2, 1,
					v132Element(t, "tag", "@INCLUDE", "include", 2, 10),
					v132Element(t, "identifier", "Я", "я", 11, 12)))
				want = nil
			}
			v132Run(t, result, "current", want)
		})
	}
}

func TestV132BranchesAndDepth(t *testing.T) {
	for _, mode := range []string{"unused", "reference in other branch", "duplicate in other branch"} {
		t.Run(mode, func(t *testing.T) {
			body := []model.Line{v132Open(t, "variants", 2, 1), v132Named(t, "variant", "name", "A", 3, 2)}
			body = append(body, v132Fragment(t, "f", 5, 4, 3)...)
			body = append(body, v132Text(t, "First branch.", 4, 3), v132Named(t, "variant", "name", "B", 3, 2))
			var want []v132Expected
			switch mode {
			case "unused":
				body = append(body, v132Text(t, "Second branch.", 9, 3))
				want = []v132Expected{{5, 11, 12}}
			case "reference in other branch":
				body = append(body, v132Named(t, "include", "identifier", "F", 9, 3))
			case "duplicate in other branch":
				body = append(body, v132Fragment(t, "F", 10, 9, 3)...)
				body = append(body, v132Text(t, "Second branch.", 9, 3))
			}
			body = append(body, v132Close(t, 3, 2))
			v132Run(t, v132Document(t, body...), "current", want)
		})
	}
	for _, used := range []bool{false, true} {
		t.Run("task step "+fmt.Sprint(used), func(t *testing.T) {
			body := []model.Line{v132Named(t, "task", "identifier", "t", 2, 1), v132Named(t, "step", "title", "First", 3, 2)}
			body = append(body, v132Fragment(t, "f", 5, 4, 3)...)
			want := []v132Expected{{5, 11, 12}}
			if used {
				body = append(body, v132Named(t, "include", "identifier", "f", 4, 3))
				want = nil
			} else {
				body = append(body, v132Text(t, "Continue.", 4, 3))
			}
			v132Run(t, v132Document(t, body...), "current", want)
		})
	}
}

func TestV132SavedArguments(t *testing.T) {
	for _, tc := range []struct {
		name  string
		line  model.Line
		error int
		want  []v132Expected
	}{
		{"extra parameter", v132Line(t, "@include f extra", "tag", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Element(t, "identifier", "f", "f", 10, 11),
			v132Unparsed(t, "extra", 12, 17)), 2, nil},
		{"invalid line with saved ID", v132Line(t, "@include f extra", "invalid", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Element(t, "identifier", "f", "f", 10, 11),
			v132Unparsed(t, "extra", 12, 17)), 2, nil},
		{"unparsed line is not recovered", v132Line(t, "@include f", "invalid", 2, 1,
			v132Unparsed(t, "@include f", 1, 11)), 0, []v132Expected{{3, 11, 12}}},
		{"missing identifier", v132Line(t, "@include f", "invalid", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Unparsed(t, "f", 10, 11)), 1, []v132Expected{{3, 11, 12}}},
		{"missing tag", v132Line(t, "@include f", "invalid", 2, 1,
			v132Unparsed(t, "@include", 1, 9), v132Element(t, "identifier", "f", "f", 10, 11)), 0, []v132Expected{{3, 11, 12}}},
		{"unparsed first argument before later ID", v132Line(t, "@include bad! f", "invalid", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Unparsed(t, "bad!", 10, 14),
			v132Element(t, "identifier", "f", "f", 15, 16)), 1, []v132Expected{{3, 11, 12}}},
		{"invalid first ID before later ID", v132Line(t, "@include bad! f", "tag", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Element(t, "identifier", "bad!", "bad!", 10, 14),
			v132Element(t, "identifier", "f", "f", 15, 16)), 1, []v132Expected{{3, 11, 12}}},
		{"valid prefix of invalid argument", v132Line(t, "@include f!", "invalid", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Element(t, "identifier", "f", "f", 10, 11),
			v132Unparsed(t, "!", 11, 12)), 2, []v132Expected{{3, 11, 12}}},
		{"valid first ID before other identifier", v132Line(t, "@include f g", "tag", 2, 1,
			v132Element(t, "tag", "@include", "include", 1, 9), v132Element(t, "identifier", "f", "f", 10, 11),
			v132Element(t, "identifier", "g", "g", 12, 13)), 2, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v132Definitions(t, "f")
			v132Append(t, result, tc.line)
			v132ParserError(t, result, 7, tc.error)
			v132Run(t, result, "current", tc.want)
		})
	}
	for _, missing := range []string{"tag", "identifier"} {
		t.Run("definition missing "+missing, func(t *testing.T) {
			result := v132Definitions(t, "f")
			index := 0
			if missing == "identifier" {
				index = 1
			}
			element := &result.Lines[2].Elements[index]
			element.ElementType, element.Value = "unparsed", nil
			v132ParserError(t, result, 3, index)
			v132Run(t, result, "current", nil)
		})
	}
}

func TestV132MisplacedReferences(t *testing.T) {
	t.Run("self reference counts without expansion", func(t *testing.T) {
		result := v132Definitions(t, "f")
		result.Lines[3] = v132Named(t, "include", "identifier", "f", 3, 2)
		result.Lines[3].Number = 4
		v132ParserError(t, result, 4, 0)
		v132Run(t, result, "current", nil)
	})
	t.Run("reference inside note", func(t *testing.T) {
		result := v132Definitions(t, "f")
		v132Append(t, result, v132Open(t, "note", 2, 1),
			v132Named(t, "include", "identifier", "f", 7, 2), v132Close(t, 7, 2))
		v132ParserError(t, result, 8, 0)
		v132Run(t, result, "current", nil)
	})
}

func TestV132LiteralAndOtherIDs(t *testing.T) {
	for _, name := range []string{"editor", "multifill"} {
		t.Run(name, func(t *testing.T) {
			result := v132Definitions(t, "f")
			// По DSL 2.8.11 и 4.4.11 это content, а не объявления.
			// Псевдоопределение не создаёт дубль; псевдовключение не даёт ссылку.
			v132Append(t, result, v132Open(t, name, 2, 1),
				v132Text(t, "@fragment f {", 7, 2), v132Text(t, "@include f", 7, 2), v132Close(t, 7, 2))
			v132Run(t, result, "current", []v132Expected{{3, 11, 12}})
		})
	}
	for _, raw := range []string{"f", "See @include f here.", "@includeText f", `\@include f`} {
		t.Run(raw, func(t *testing.T) {
			result := v132Definitions(t, "f")
			line := v132Text(t, raw, 2, 1)
			if raw == `\@include f` {
				value := "@include f"
				line.Elements[0].Value = &value
			}
			// @includeText — неизвестное имя, не include; парсер сохраняет
			// ошибочную строку целиком, не создавая канонический tag.
			if raw == "@includeText f" {
				line = v132Line(t, raw, "invalid", 2, 1, v132Unparsed(t, raw, 1, 15))
			}
			v132Append(t, result, line)
			if raw == "@includeText f" {
				v132ParserError(t, result, 7, 0)
			}
			v132Run(t, result, "current", []v132Expected{{3, 11, 12}})
		})
	}
	t.Run("task namespace is separate", func(t *testing.T) {
		result := v132Definitions(t, "f")
		v132Append(t, result, v132Named(t, "task", "identifier", "f", 2, 1), v132Text(t, "Write.", 7, 2))
		v132Run(t, result, "current", []v132Expected{{3, 11, 12}})
	})
}

func TestV132LineEndingsAndRepeatedRuns(t *testing.T) {
	for _, tc := range []struct {
		name string
		eol  model.LineEnding
		last model.LineEnding
	}{
		{"LF", model.LineEndingLF, model.LineEndingLF},
		{"CRLF", model.LineEndingCRLF, model.LineEndingCRLF},
		{"no final newline", model.LineEndingLF, model.LineEndingNone},
		{"CRLF no final newline", model.LineEndingCRLF, model.LineEndingNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v132Definitions(t, "f")
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[len(result.Lines)-1].LineEnding = tc.last
			v132Run(t, result, "first", []v132Expected{{3, 11, 12}})
			v132Run(t, result, "second", []v132Expected{{3, 11, 12}})
		})
	}
	v132Run(t, v132Unavailable(t, "1.2"), "current", nil)
	v132Run(t, v132Definitions(t), "current", nil)
}

// v132Registry подключает полный штатный каталог без подмены регистраций.
func v132Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v132Unavailable задаёт согласованную модель с недоступными исходными строками.
func v132Unavailable(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1,
		Document:   model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v132Document добавляет обязательный префикс; структуру тела задаёт сам тест.
func v132Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v132Unavailable(t, "1.2")
	title, encoding := "Lesson", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	v132Append(t, result,
		v132Named(t, "dsl-version", "version", "1.2", 0, 0),
		v132Line(t, "# Lesson", "heading", 0, 0,
			v132Element(t, "heading-level", "#", "1", 1, 2), v132Element(t, "title", "Lesson", "Lesson", 3, 9)))
	v132Append(t, result, body...)
	return result
}

// v132Append нумерует входные строки, не вычисляя ожидаемые диагностики.
func v132Append(t *testing.T, result *model.Result, lines ...model.Line) {
	t.Helper()
	for _, line := range lines {
		line.Number = len(result.Lines) + 1
		result.Lines = append(result.Lines, line)
	}
	count := len(result.Lines)
	result.Document.LineCount = &count
}

// v132Definitions создаёт полные соседние определения и отображаемую строку.
func v132Definitions(t *testing.T, ids ...string) *model.Result {
	t.Helper()
	var body []model.Line
	for _, id := range ids {
		body = append(body, v132Fragment(t, id, len(body)+3, 2, 1)...)
	}
	return v132Document(t, append(body, v132Text(t, "Continue.", 2, 1))...)
}

// v132Fragment строит полный блок с явно заданным окружением.
func v132Fragment(t *testing.T, id string, number, parent, depth int) []model.Line {
	t.Helper()
	line := v132Named(t, "fragment", "identifier", id, parent, depth)
	open := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v132Element(t, "block-open", "{", "{", open, open+1))
	return []model.Line{line, v132Text(t, "Read this text.", number, depth+1), v132Close(t, number, depth+1)}
}

// v132Line сохраняет готовую входную структуру, не разбирая Raw.
func v132Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v132Element сохраняет независимо заданные Raw, Value и Unicode-колонки.
func v132Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v132Unparsed создаёт элемент, которому тест обязан добавить ошибку парсера.
func v132Unparsed(t *testing.T, raw string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: "unparsed", Raw: raw, Start: start, End: end, ErrorIDs: []string{}}
}

// v132Named строит заданный тег и один аргумент без проверки допустимости ID.
func v132Named(t *testing.T, name string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	start := utf8.RuneCountInString(tag) + 2
	return v132Line(t, tag+" "+value, "tag", parent, depth,
		v132Element(t, "tag", tag, name, 1, start-1), v132Element(t, kind, value, value, start, start+utf8.RuneCountInString(value)))
}

// v132Text создаёт содержимое, семантическое значение которого по умолчанию равно Raw.
func v132Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v132Line(t, raw, "content", parent, depth, v132Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v132Open создаёт объявление фигурного блока без параметров.
func v132Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	tag := "@" + name
	open := utf8.RuneCountInString(tag) + 2
	return v132Line(t, tag+" {", "block-start", parent, depth,
		v132Element(t, "tag", tag, name, 1, open-1), v132Element(t, "block-open", "{", "{", open, open+1))
}

// v132Close закрывает явно указанный родительский блок.
func v132Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v132Line(t, "}", "block-end", parent, depth, v132Element(t, "block-close", "}", "}", 1, 2))
}

// v132ParserError связывает ошибочный элемент с сохранённой диагностикой парсера.
// Непрозрачный тестовый код не предполагает кодов реального внешнего парсера.
func v132ParserError(t *testing.T, result *model.Result, number, index int) {
	t.Helper()
	line := &result.Lines[number-1]
	element := &line.Elements[index]
	id := fmt.Sprintf("parser-error-%d", len(result.Diagnostics)+1)
	element.ErrorIDs = append(element.ErrorIDs, id)
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", Message: "Сохранённая ошибка разбора.", DiagnosticScope: "element",
		Location:         &model.Location{Start: model.Position{Line: number, Column: element.Start}, End: model.Position{Line: number, Column: element.End}},
		RelatedLocations: []model.Location{}})
}

// v132Run выполняет полный Prepare/Run и сравнивает только текущую V132.
// Остальные диагностики не удаляются; целостность флагов проверяет CheckModel.
func v132Run(t *testing.T, result *model.Result, source string, want []v132Expected) {
	t.Helper()
	before := v132Snapshot(t, result)
	checks, err := validator.Prepare(result, v132Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v132Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	oldCount, oldProcessing := len(result.Diagnostics), len(result.Processing)
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	currentIDs := make(map[string]bool)
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V132" {
			if entry.Source != source {
				t.Fatalf("new V132 has source %q, want %q", entry.Source, source)
			}
			got = append(got, entry)
			currentIDs[entry.ID] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V132 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected.line, Column: expected.start}, End: model.Position{Line: expected.line, Column: expected.end}}
		if entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d ranges = %+v / %+v, want %+v / none", i, entry.Location, entry.RelatedLocations, location)
		}
		if entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal || entry.Message !=
			"На @fragment нет ссылок @include.\nконкретизация: Добавьте необходимое включение либо удалите неиспользуемое определение после редакторской проверки" {
			t.Errorf("occurrence %d metadata = %+v", i, entry)
		}
		if !result.Document.HasErrors || !result.Lines[expected.line-1].HasErrors {
			t.Error("missing error flags")
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("invalid result: %v", err)
	}
	// Сравниваем исходник и прежнюю историю, убрав только добавленные
	// конвейером записи, флаги и ссылки чужих element-диагностик.
	var original, actual model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v132Snapshot(t, result)), &actual); err != nil {
		t.Fatal(err)
	}
	actual.Diagnostics = actual.Diagnostics[:oldCount]
	actual.Processing = actual.Processing[:oldProcessing]
	actual.Document.HasErrors = original.Document.HasErrors
	for i := range actual.Lines {
		actual.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range actual.Lines[i].Elements {
			element := &actual.Lines[i].Elements[j]
			for _, id := range element.ErrorIDs {
				if currentIDs[id] {
					t.Errorf("ScopeLine V132 linked to element at %d:%d", i+1, j)
				}
			}
			oldIDs := original.Lines[i].Elements[j].ErrorIDs
			if len(element.ErrorIDs) < len(oldIDs) || strings.Join(element.ErrorIDs[:len(oldIDs)], "\x00") != strings.Join(oldIDs, "\x00") {
				t.Fatal("previous element links changed")
			}
			element.ErrorIDs = element.ErrorIDs[:len(oldIDs)]
		}
	}
	if v132Snapshot(t, &actual) != before {
		t.Fatal("source or previous history changed")
	}
}

// v132Snapshot сериализует модель для проверки отсутствия побочных изменений.
func v132Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

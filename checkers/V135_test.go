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

type v135Expected struct{ line, start, end int }

func TestV135Registration(t *testing.T) {
	registry := v135Registry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V135" {
			continue
		}
		count++
		if definition.Severity != "error" || definition.Scope != "line" || definition.Check == nil ||
			definition.Message != "Между @answer и открывающей скобкой записано содержимое." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V135 definitions = %d, want 1", count)
	}
	for _, version := range []dslversions.Version{"1.1", "1.3"} {
		if definitions, err := registry.ForVersion(version); err == nil || len(definitions) != 0 {
			t.Fatalf("unsupported version %s accepted: %+v, %v", version, definitions, err)
		}
	}
}

func TestV135ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, raw, fragment string
		opening             int
		bad                 bool
		want                []v135Expected
	}{
		{"reference", "@answer {", "@question Where does Tom live?\n@answer {\nLondon\nParis\n}", 9, false, nil},
		{"example", "@answer Cities {", "@question Where does Tom live?\n@answer Cities {\nLondon\nParis\n}", 16, true, []v135Expected{{4, 9, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v135QuestionBlock(t, v135OpeningLine(t, tc.raw, 1, tc.opening, tc.bad))
			var raw []string
			for _, line := range result.Lines {
				raw = append(raw, line.Raw)
			}
			// По п. 1.30 добавлены только обязательные версия и заголовок.
			if strings.Join(raw, "\n") != "@dsl-version 1.2\n# Lesson\n"+tc.fragment {
				t.Fatal("normative fragment changed")
			}
			v135Run(t, result, "current", tc.want)
		})
	}
}

func TestV135ContentAndEscapes(t *testing.T) {
	for _, tc := range []struct {
		name, raw                string
		tag, opening, start, end int
	}{
		{"one word", "@answer Cities {", 1, 16, 9, 15},
		{"several words", "@answer Two words {", 1, 19, 9, 18},
		{"case indent Unicode", "\t@AnSwEr Город 😀 {", 2, 18, 10, 17},
		{"several edge spaces", "  @ANSWER   Cities   {", 3, 22, 13, 19},
		{"internal tabs preserved", "@answer A\tB {", 1, 13, 9, 12},
		{"zero width space is content", "@answer \u200b {", 1, 11, 9, 10},
		{"internal BOM is content", "@answer \ufeff {", 1, 11, 9, 10},
		{"combining mark and supplementary character", "@answer A\u0301😀 {", 1, 13, 9, 12},
		{"escaped opening before real opening", `@answer \{ Cities {`, 1, 19, 9, 18},
		{"two backslashes", `@answer \\{`, 1, 11, 9, 11},
		{"four backslashes", `@answer \\\\{`, 1, 13, 9, 13},
		{"three backslashes and later opening", `@answer \\\{ {`, 1, 14, 9, 13},
		{"escaped closing", `@answer \} {`, 1, 12, 9, 11},
		{"backslash before ordinary character", `@answer \q{`, 1, 11, 9, 11},
		{"double quotes do not hide opening", `@answer "{"`, 1, 10, 9, 10},
		{"single quotes do not hide opening", "@answer '{'", 1, 10, 9, 10},
		{"first opening", "@answer A { B {", 1, 11, 9, 10},
		{"content touches opening", "@answer Cities{", 1, 15, 9, 15},
	} {
		for _, form := range []string{"block-open", "tag and unparsed", "all unparsed"} {
			t.Run(tc.name+"/"+form, func(t *testing.T) {
				line := v135OpeningLine(t, tc.raw, tc.tag, tc.opening, true)
				if form != "block-open" {
					line = v135Attempt(t, tc.raw, tc.tag, form == "tag and unparsed", 2, 1)
				}
				v135Run(t, v135QuestionBlock(t, line), "current", []v135Expected{{4, tc.start, tc.end}})
			})
		}
	}
}

func TestV135Whitespace(t *testing.T) {
	// Полный перечень White_Space выписан независимо от проверяемого кода.
	// CR/LF не допускаются внутри Raw; окончания строк проверены отдельно.
	spaces := []string{" ", "\t", "\v", "\f", "\u0085", "\u00a0", "\u1680",
		"\u2000", "\u2001", "\u2002", "\u2003", "\u2004", "\u2005", "\u2006",
		"\u2007", "\u2008", "\u2009", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000"}
	for _, space := range spaces {
		for _, content := range []bool{false, true} {
			forms := []bool{true}
			if content || space != " " {
				forms = append(forms, false)
			}
			for _, parsed := range forms {
				t.Run(strconv.Quote(space)+"/content="+strconv.FormatBool(content)+"/parsed="+strconv.FormatBool(parsed), func(t *testing.T) {
					raw, opening := "@answer"+space+"{", 9
					var want []v135Expected
					if content {
						raw, opening = "@answer "+space+"Cities"+space+"{", 17
						want = []v135Expected{{4, 10, 16}}
					}
					line := v135Attempt(t, raw, 1, false, 2, 1)
					if parsed {
						line = v135OpeningLine(t, raw, 1, opening, content || space != " ")
					}
					v135Run(t, v135QuestionBlock(t, line), "current", want)
				})
			}
		}
	}
	for _, tc := range []struct {
		raw     string
		opening int
		bad     bool
	}{
		{"@answer{", 8, true}, {"@answer   {", 11, false}, {"@answer \t\u00a0 {", 12, true},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			v135Run(t, v135QuestionBlock(t, v135OpeningLine(t, tc.raw, 1, tc.opening, tc.bad)), "current", nil)
		})
	}
}

func TestV135RecognitionAndNoOpening(t *testing.T) {
	for _, raw := range []string{
		"@answerExtra Cities {", "@answers Cities {", "@answer.extra Cities {", "@answer- Cities {",
		"@answer! Cities {", "\u00a0@answer Cities {", "@",
	} {
		t.Run("not exact initial name/"+strconv.Quote(raw), func(t *testing.T) {
			v135Run(t, v135Lesson(t, v135Question(t, 2, 1), v135Attempt(t, raw, 1, false, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{`\@answer Cities \{`, "@answer Cities {"}, {`Text @answer Cities \{`, "Text @answer Cities {"},
	} {
		t.Run("literal text/"+tc.raw, func(t *testing.T) {
			v135Run(t, v135Lesson(t, v135Text(t, tc.raw, tc.value, 2, 1)), "current", nil)
		})
	}
	for _, tc := range []struct{ raw, value string }{
		{"London", "London"}, {`Cities \{`, "Cities {"}, {`\\\{`, `\{`},
		{`\{ A \{ B`, "{ A { B"}, {`\}`, "}"}, {`\\`, `\`},
	} {
		t.Run("inline answer/"+tc.raw, func(t *testing.T) {
			line := v135Line(t, "@answer "+tc.raw, "tag", 2, 1,
				v135Element(t, "tag", "@answer", "answer", 1), v135Element(t, "content", tc.raw, tc.value, 9))
			v135Run(t, v135Lesson(t, v135Question(t, 2, 1), line), "current", nil)
		})
	}
	t.Run("opening on another line", func(t *testing.T) {
		v135Run(t, v135Lesson(t, v135Question(t, 2, 1),
			v135Attempt(t, "@answer Cities", 1, true, 2, 1), v135Attempt(t, "{", 1, false, 2, 1),
			v135Text(t, "London", "London", 2, 1), v135Attempt(t, "}", 1, false, 2, 1)), "current", nil)
	})
}

func TestV135ContextsAndOccurrences(t *testing.T) {
	t.Run("every occurrence", func(t *testing.T) {
		result := v135QuestionBlock(t, v135OpeningLine(t, "@answer Cities {", 1, 16, true))
		body := append([]model.Line{}, result.Lines[2:]...)
		body = append(body, v135Question(t, 2, 1), v135OpeningLine(t, "@answer Cities {", 1, 16, true),
			v135Text(t, "London", "London", 9, 2), v135Text(t, "Paris", "Paris", 9, 2), v135Close(t, 9, 2))
		v135Run(t, v135Lesson(t, body...), "current", []v135Expected{{4, 9, 15}, {9, 9, 15}})
	})
	t.Run("nested branch", func(t *testing.T) {
		bad := v135OpeningLine(t, "@answer Cities {", 1, 16, true)
		parent := 4
		bad.ParentLine, bad.NestingLevel = &parent, 3
		good := v135OpeningLine(t, "@answer {", 1, 9, false)
		other := 10
		good.ParentLine, good.NestingLevel = &other, 3
		v135Run(t, v135Lesson(t, v135Open(t, "variants", 2, 1),
			v135Line(t, "@variant A", "tag", 3, 2, v135Element(t, "tag", "@variant", "variant", 1), v135Element(t, "name", "A", "A", 10)),
			v135Question(t, 4, 3), bad, v135Text(t, "London", "London", 6, 4), v135Text(t, "Paris", "Paris", 6, 4), v135Close(t, 6, 4),
			v135Line(t, "@variant B", "tag", 3, 2, v135Element(t, "tag", "@variant", "variant", 1), v135Element(t, "name", "B", "B", 10)),
			v135Question(t, 10, 3), good, v135Text(t, "London", "London", 12, 4), v135Text(t, "Paris", "Paris", 12, 4), v135Close(t, 12, 4),
			v135Close(t, 3, 2)), "current", []v135Expected{{6, 9, 15}})
	})
	for _, name := range []string{"editor", "multifill"} {
		for _, recognized := range []bool{false, true} {
			t.Run("literal content/"+name+"/parent tag="+strconv.FormatBool(recognized), func(t *testing.T) {
				opening := v135Open(t, name, 2, 1)
				if !recognized {
					// Ошибка распознавания имени не теряет сохранённую структуру блока.
					opening.Elements[0].ElementType = "unparsed"
				}
				raw := `@answer Cities \{`
				if name == "multifill" {
					raw = `\@answer Cities \{`
				}
				v135Run(t, v135Lesson(t, opening, v135Text(t, raw, "@answer Cities {", 3, 2),
					v135Close(t, 3, 2), v135Text(t, "Read.", "Read.", 2, 1)), "current", nil)
			})
		}
	}
	t.Run("forbidden answer in multifill is separate", func(t *testing.T) {
		v135Run(t, v135Lesson(t, v135Open(t, "multifill", 2, 1),
			v135Attempt(t, "@answer Cities {", 1, false, 3, 2), v135Text(t, "London", "London", 3, 2),
			v135Close(t, 3, 2)), "current", nil)
	})
	t.Run("literal context ends at closing", func(t *testing.T) {
		v135Run(t, v135Lesson(t, v135Open(t, "editor", 2, 1), v135Text(t, `@answer Cities \{`, "@answer Cities {", 3, 2),
			v135Close(t, 3, 2), v135Question(t, 2, 1), v135OpeningLine(t, "@answer Cities {", 1, 16, true),
			v135Text(t, "London", "London", 7, 2), v135Text(t, "Paris", "Paris", 7, 2), v135Close(t, 7, 2)), "current", []v135Expected{{7, 9, 15}})
	})
	t.Run("inline editor does not create literal context", func(t *testing.T) {
		v135Run(t, v135Lesson(t, v135Question(t, 2, 1),
			v135Line(t, "@editor Check.", "tag", 2, 1, v135Element(t, "tag", "@editor", "editor", 1), v135Element(t, "content", "Check.", "Check.", 9)),
			v135OpeningLine(t, "@answer Cities {", 1, 16, true), v135Text(t, "London", "London", 5, 2), v135Close(t, 5, 2)),
			"current", []v135Expected{{5, 9, 15}})
	})
}

func TestV135EndingsAndLifecycle(t *testing.T) {
	for _, eol := range []model.LineEnding{"\n", "\r\n"} {
		for _, final := range []bool{false, true} {
			t.Run(strconv.Quote(string(eol))+"/final="+strconv.FormatBool(final), func(t *testing.T) {
				result := v135QuestionBlock(t, v135OpeningLine(t, "@answer Cities {", 1, 16, true))
				for i := range result.Lines {
					result.Lines[i].LineEnding = eol
				}
				if !final {
					result.Lines[len(result.Lines)-1].LineEnding = ""
				}
				v135Run(t, result, "first", []v135Expected{{4, 9, 15}})
				v135Run(t, result, "second", []v135Expected{{4, 9, 15}})
			})
		}
	}
	t.Run("last declaration without EOL", func(t *testing.T) {
		// Неразобранное объявление на EOF не создаёт выдуманного закрытия блока.
		result := v135Lesson(t, v135Question(t, 2, 1), v135Attempt(t, "@answer Cities {", 1, false, 2, 1))
		result.Lines[3].LineEnding = ""
		v135Run(t, result, "current", []v135Expected{{4, 9, 15}})
	})
	t.Run("empty model", func(t *testing.T) {
		result := v135Lesson(t)
		result.Lines, result.Document.Metadata.Title = []model.Line{}, nil
		*result.Document.LineCount = 0
		v135Run(t, result, "empty", nil)
	})
	t.Run("blank lines", func(t *testing.T) {
		v135Run(t, v135Lesson(t, v135Line(t, "", "blank", 2, 1), v135Line(t, " \t", "blank", 2, 1)), "current", nil)
	})
	t.Run("independent documents", func(t *testing.T) {
		v135Run(t, v135QuestionBlock(t, v135OpeningLine(t, "@answer Cities {", 1, 16, true)), "first", []v135Expected{{4, 9, 15}})
		v135Run(t, v135QuestionBlock(t, v135OpeningLine(t, "@answer {", 1, 9, false)), "second", nil)
	})
}

// v135QuestionBlock дополняет явно заданное объявление вопросом и ответами.
// Если парсер не выделил блок, строки ответа остаются на уровне вопроса,
// а отдельная закрывающая скобка также сохраняется в unparsed с ошибкой.
func v135QuestionBlock(t *testing.T, opening model.Line) *model.Result {
	t.Helper()
	parent, depth := 4, 2
	closing := v135Close(t, parent, depth)
	if opening.LineType != "block-start" {
		parent, depth = 2, 1
		closing = v135Attempt(t, "}", 1, false, parent, depth)
	}
	return v135Lesson(t, v135Question(t, 2, 1), opening,
		v135Text(t, "London", "London", parent, depth), v135Text(t, "Paris", "Paris", parent, depth), closing)
}

// v135OpeningLine задаёт распознанные имя и открытие по явным колонкам.
// badGap сохраняет ошибочный промежуток в unparsed; это входная модель,
// а не вычисление ожидаемого диапазона V135. После открытия хвост ошибочен.
func v135OpeningLine(t *testing.T, raw string, tag, opening int, badGap bool) model.Line {
	t.Helper()
	runes := []rune(raw)
	elements := []model.Element{v135Element(t, "tag", string(runes[tag-1:tag+6]), "answer", tag)}
	if badGap && opening > tag+7 {
		elements = append(elements, v135Element(t, "unparsed", string(runes[tag+6:opening-1]), "", tag+7))
	}
	elements = append(elements, v135Element(t, "block-open", "{", "{", opening))
	if opening < len(runes) {
		elements = append(elements, v135Element(t, "unparsed", string(runes[opening:]), "", opening+1))
	}
	return v135Line(t, raw, "block-start", 2, 1, elements...)
}

// v135Attempt сохраняет ошибочное объявление без распознанного открытия.
// knownTag оставляет полный answer; иначе исходник целиком становится unparsed.
func v135Attempt(t *testing.T, raw string, tag int, knownTag bool, parent, depth int) model.Line {
	t.Helper()
	elements := []model.Element{}
	start := 1
	if knownTag {
		elements = append(elements, v135Element(t, "tag", string([]rune(raw)[tag-1:tag+6]), "answer", tag))
		start = tag + 7
	}
	elements = append(elements, v135Element(t, "unparsed", string([]rune(raw)[start-1:]), "", start))
	return v135Line(t, raw, "invalid", parent, depth, elements...)
}

// v135Question задаёт однострочный вопрос с явно указанным родителем.
func v135Question(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v135Line(t, "@question Where does Tom live?", "tag", parent, depth,
		v135Element(t, "tag", "@question", "question", 1), v135Element(t, "content", "Where does Tom live?", "Where does Tom live?", 11))
}

// v135Lesson добавляет обязательный контекст и согласованные ошибки unparsed.
// Первой строкой body становится строка 3; родительские связи задают сами тесты.
func v135Lesson(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{
		v135Line(t, "@dsl-version 1.2", "tag", 0, 0, v135Element(t, "tag", "@dsl-version", "dsl-version", 1), v135Element(t, "version", "1.2", "1.2", 14)),
		v135Line(t, "# Lesson", "heading", 0, 0, v135Element(t, "heading-level", "#", "1", 1), v135Element(t, "title", "Lesson", "Lesson", 3)),
	}, body...)
	count := len(lines)
	result := &model.Result{Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count, Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Lines:    lines, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Diagnostics: []model.Diagnostic{}}
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
				ID: id, Source: "parser", DiagnosticCode: "fixture-unparsed", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Не разобран фрагмент исходной строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start}, End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
	}
	return result
}

// v135Open задаёт объявление блока без параметров.
func v135Open(t *testing.T, name string, parent, depth int) model.Line {
	t.Helper()
	return v135Line(t, "@"+name+" {", "block-start", parent, depth,
		v135Element(t, "tag", "@"+name, name, 1), v135Element(t, "block-open", "{", "{", len(name)+3))
}

// v135Close задаёт закрытие указанного родительского блока.
func v135Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v135Line(t, "}", "block-end", parent, depth, v135Element(t, "block-close", "}", "}", 1))
}

// v135Text сохраняет явно заданные исходный текст и семантическое значение.
func v135Text(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v135Line(t, raw, "content", parent, depth, v135Element(t, "content", raw, value, 1))
}

// v135Line сохраняет явную структуру входа без распознавания исходника.
func v135Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v135Element измеряет только входной элемент в рунах, конец исключается.
// Ожидаемые диапазоны срабатываний задаются отдельно, явными числами.
func v135Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v135Registry использует только полный штатный каталог регистраций.
func v135Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v135Run проверяет новые V135 штатного конвейера по независимым ожиданиям.
// Снимки контролируют неизменность исходника, метаданных и прежней истории.
func v135Run(t *testing.T, result *model.Result, source string, want []v135Expected) {
	t.Helper()
	before := v135Snapshot(t, result)
	var original model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, v135Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v135Snapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticCode == "V135" && entry.Source == source {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V135 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, position := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: position.line, Column: position.start}, End: model.Position{Line: position.line, Column: position.end}}
		if entry.Location == nil || *entry.Location != location || entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 ||
			entry.ID == "" || entry.SeverityLevel != "error" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.Message != "Между @answer и открывающей скобкой записано содержимое." {
			t.Fatalf("occurrence %d = %+v; location = %+v, want %+v and exact V135 metadata", i, entry, entry.Location, location)
		}
		if !result.Document.HasErrors || !result.Lines[position.line-1].HasErrors {
			t.Fatal("missing error flags")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				for _, id := range element.ErrorIDs {
					if id == entry.ID {
						t.Fatal("line-scoped V135 linked to an element")
					}
				}
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var restored model.Result
	if err := json.Unmarshal([]byte(v135Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	// Возвращаем в копии только производные флаги/ссылки и убираем новые записи.
	// Остальное, включая старые диагностики, обязано совпасть с исходным снимком.
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
	if v135Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v135Snapshot получает независимое представление модели для сравнения.
func v135Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

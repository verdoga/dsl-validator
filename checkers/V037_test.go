package checkers

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV037Registration(t *testing.T) {
	definitions, err := v037Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range definitions {
		if d.Code != "V037" {
			continue
		}
		count++
		if d.Severity != "error" || d.Scope != "document" || d.Check == nil ||
			d.Message != "В документе отсутствует @document-id." ||
			d.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected definition: %+v", d)
		}
	}
	if count != 1 {
		t.Fatalf("V037 definitions = %d, want 1", count)
	}
}

func TestV037ReferenceAndExample(t *testing.T) {
	// Обе ситуации из таблицы уже являются полными документами DSL.
	t.Run("reference", func(t *testing.T) { v037Run(t, v037Reference(t, "@document-id"), "current", 0) })
	t.Run("example", func(t *testing.T) { v037Run(t, v037Document(t), "current", 1) })
	for _, tag := range []string{"@DOCUMENT-ID", "@Document-Id"} {
		t.Run(tag, func(t *testing.T) {
			// Value тега каноническое по контракту модели; регистр исходного Raw сохранён.
			v037Run(t, v037Reference(t, tag), "current", 0)
		})
	}
}

func TestV037MissingDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
	}{
		{"blank lines", []model.Line{v037Line(t, "", "blank", 2, 1), v037Line(t, " \t", "blank", 2, 1)}},
		{"text mention", []model.Line{v037Text(t, "Use @document-id lesson-a here.", 2, 1)}},
		{"Unicode text mention", []model.Line{v037Text(t, "Я😀: @DOCUMENT-ID lesson-a", 2, 1)}},
		{"single-line comment", []model.Line{v037Line(t, "@editor @document-id lesson-a", "tag", 2, 1, v037Element(t, "tag", "@editor", "editor", 1), v037Element(t, "content", "@document-id lesson-a", "@document-id lesson-a", 9))}},
		{"escaped tag", []model.Line{v037Line(t, `\@document-id lesson-a`, "content", 2, 1, v037Element(t, "content", `\@document-id lesson-a`, "@document-id lesson-a", 1))}},
		{"different recognized tag", []model.Line{v037Line(t, "@header document-id", "tag", 2, 1, v037Element(t, "tag", "@header", "header", 1), v037Element(t, "title", "document-id", "document-id", 9))}},
	} {
		t.Run(tc.name, func(t *testing.T) { v037Run(t, v037Document(t, tc.body...), "current", 1) })
	}
	for _, tag := range []string{"editor", "multifill"} {
		t.Run("literal "+tag, func(t *testing.T) {
			result := v037Document(t,
				v037Line(t, "@"+tag+" {", "block-start", 2, 1, v037Element(t, "tag", "@"+tag, tag, 1), v037Element(t, "block-open", "{", "{", len(tag)+3)),
				v037Text(t, "@document-id lesson-a", 3, 2),
				v037Text(t, "@DOCUMENT-ID lesson-b", 3, 2),
				v037Text(t, "<b>@document-id lesson-c</b> | @document-id lesson-d", 3, 2),
				v037Line(t, "}", "block-end", 3, 2, v037Element(t, "block-close", "}", "}", 1)))
			v037Run(t, result, "current", 1)
		})
	}
	for _, raw := range []string{"@document-id lesson-a", "@document-id-extra lesson-a"} {
		t.Run("unparsed "+raw, func(t *testing.T) {
			element := v037Element(t, "unparsed", raw, "", 1)
			element.Value, element.ErrorIDs = nil, []string{"parser-error"}
			result := v037Document(t, v037Line(t, raw, "invalid", 2, 1, element))
			v037ParserError(t, result, 3, "element", 1, element.End)
			v037Run(t, result, "current", 1)
		})
	}
}

func TestV037DeclarationWithOtherProblems(t *testing.T) {
	t.Run("last line and wrong position", func(t *testing.T) {
		result := v037Document(t, v037Text(t, "Read.", 2, 1), v037Declaration(t, "@document-id", "lesson-a", 2, 1))
		v037ParserError(t, result, 4, "line", 1, 13)
		v037Run(t, result, "current", 0)
	})
	t.Run("nested recognized declaration", func(t *testing.T) {
		result := v037Document(t,
			v037Line(t, "@text {", "block-start", 2, 1, v037Element(t, "tag", "@text", "text", 1), v037Element(t, "block-open", "{", "{", 7)),
			v037Text(t, "Read.", 3, 2), v037Declaration(t, "@document-id", "lesson-a", 3, 2),
			v037Line(t, "}", "block-end", 3, 2, v037Element(t, "block-close", "}", "}", 1)))
		v037ParserError(t, result, 5, "line", 1, 13)
		v037Run(t, result, "current", 0)
	})
	t.Run("empty argument", func(t *testing.T) {
		result := v037Reference(t, "@document-id")
		result.Document.Metadata.DocumentID = nil
		result.Lines[1] = v037Line(t, "@document-id", "tag", 0, 0, v037Element(t, "tag", "@document-id", "document-id", 1))
		v037NumberLines(t, result)
		v037ParserError(t, result, 2, "line", 1, 13)
		v037Run(t, result, "current", 0)
	})
	t.Run("unparsed argument with recognized tag", func(t *testing.T) {
		result := v037Reference(t, "@document-id")
		result.Document.Metadata.DocumentID = nil
		argument := v037Element(t, "unparsed", "bad/id", "", 14)
		argument.Value, argument.ErrorIDs = nil, []string{"parser-error"}
		result.Lines[1] = v037Line(t, "@document-id bad/id", "invalid", 0, 0, v037Element(t, "tag", "@document-id", "document-id", 1), argument)
		v037NumberLines(t, result)
		v037ParserError(t, result, 2, "element", 14, 20)
		v037Run(t, result, "current", 0)
		count := 0
		for _, d := range result.Diagnostics {
			if d.Source == "current" && d.DiagnosticCode == "V032" {
				count++
				want := model.Location{Start: model.Position{Line: 2, Column: 14}, End: model.Position{Line: 2, Column: 20}}
				if d.Location == nil || *d.Location != want {
					t.Errorf("V032 range = %+v", d.Location)
				}
			}
		}
		if count != 1 {
			t.Errorf("V032 count = %d, want 1", count)
		}
	})
	t.Run("repeated declarations", func(t *testing.T) {
		result := v037Reference(t, "@document-id")
		result.Document.Metadata.DocumentID = nil
		result.Lines = append(result.Lines[:2], v037Declaration(t, "@document-id", "lesson-b", 0, 0), result.Lines[2])
		v037NumberLines(t, result)
		v037ParserError(t, result, 3, "line", 1, 13)
		v037Run(t, result, "current", 0)
	})
	t.Run("mention followed by actual declaration", func(t *testing.T) {
		result := v037Reference(t, "@document-id")
		comment := v037Line(t, "@editor @document-id lesson-b", "tag", 0, 0, v037Element(t, "tag", "@editor", "editor", 1), v037Element(t, "content", "@document-id lesson-b", "@document-id lesson-b", 9))
		result.Lines = append(result.Lines[:1], comment, result.Lines[1], result.Lines[2])
		v037NumberLines(t, result)
		v037Run(t, result, "current", 0)
	})
}

func TestV037UnavailableLinesAndVersions(t *testing.T) {
	t.Run("unavailable lines", func(t *testing.T) { v037Run(t, v037Unavailable(t, "1.2"), "current", 0) })
	t.Run("zero known lines", func(t *testing.T) {
		result := v037Unavailable(t, "1.2")
		count := 0
		result.Document.LineCount = &count
		v037Run(t, result, "current", 0)
	})
	t.Run("unavailable after technical failure", func(t *testing.T) {
		result := v037Unavailable(t, "1.2")
		result.Document.HasErrors = true
		result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-technical-error", SeverityLevel: "error", DiagnosticScope: "document", Message: "Строки недоступны.", Fatal: true, RelatedLocations: []model.Location{}})
		v037Run(t, result, "current", 0)
	})
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v037Unavailable(t, version)
			before := v037Snapshot(t, result)
			checks, err := validator.Prepare(result, v037Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal", version, checks, err)
			}
			if v037Snapshot(t, result) != before {
				t.Fatal("unsupported version changed model")
			}
		})
	}
}

func TestV037EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		present bool
		want    int
	}{
		{"reference", true, 0}, {"example", false, 1},
	} {
		for _, ending := range []model.LineEnding{model.LineEndingLF, model.LineEndingCRLF, model.LineEndingNone} {
			t.Run(tc.name+string(ending), func(t *testing.T) {
				result := v037Document(t)
				if tc.present {
					result = v037Reference(t, "@document-id")
				}
				if ending == model.LineEndingNone {
					result.Lines[len(result.Lines)-1].LineEnding = ending
				} else {
					for i := range result.Lines {
						result.Lines[i].LineEnding = ending
					}
				}
				v037Run(t, result, "current", tc.want)
			})
		}
	}
	result := v037Document(t, v037Text(t, "Read  this.", 2, 1))
	v037Run(t, result, "first", 1)
	v037Run(t, result, "second", 1)
	count := 0
	for _, d := range result.Diagnostics {
		if d.Source == "second" && d.DiagnosticCode == "V001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("V001 count = %d, want 1", count)
	}
	v037Run(t, v037Reference(t, "@document-id"), "first", 0)
	v037Run(t, v037Document(t), "first", 1)
	v037Run(t, v037Reference(t, "@document-id"), "first", 0)
}

// v037Registry создаёт реестр исключительно через штатный каталог.
func v037Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v037Unavailable представляет известную версию при отсутствии доступных строк.
func v037Unavailable(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
}

// v037Document сохраняет две строки примера без изменений и добавляет
// явно заданный тестом контекст с готовыми родителями и элементами.
func v037Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	result := v037Unavailable(t, "1.2")
	title, encoding := "Lesson", "UTF-8"
	result.Document.Metadata.Title, result.Document.Encoding = &title, &encoding
	result.Lines = []model.Line{
		v037Line(t, "@dsl-version 1.2", "tag", 0, 0, v037Element(t, "tag", "@dsl-version", "dsl-version", 1), v037Element(t, "version", "1.2", "1.2", 14)),
		v037Line(t, "# Lesson", "heading", 0, 0, v037Element(t, "heading-level", "#", "1", 1), v037Element(t, "title", "Lesson", "Lesson", 3)),
	}
	result.Lines = append(result.Lines, body...)
	v037NumberLines(t, result)
	return result
}

// v037Reference создаёт точный эталон; tag позволяет менять только регистр Raw.
func v037Reference(t *testing.T, tag string) *model.Result {
	t.Helper()
	result := v037Document(t)
	id := "lesson-a"
	result.Document.Metadata.DocumentID = &id
	result.Lines = append(result.Lines[:1], v037Declaration(t, tag, id, 0, 0), result.Lines[1])
	v037NumberLines(t, result)
	return result
}

// v037Declaration задаёт распознанный тег и аргумент без повторного разбора DSL.
func v037Declaration(t *testing.T, tag, id string, parent, depth int) model.Line {
	t.Helper()
	return v037Line(t, tag+" "+id, "tag", parent, depth, v037Element(t, "tag", tag, "document-id", 1), v037Element(t, "identifier", id, id, 14))
}

// v037NumberLines назначает физические номера явно заданным строкам.
func v037NumberLines(t *testing.T, result *model.Result) {
	t.Helper()
	count := len(result.Lines)
	result.Document.LineCount = &count
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
}

// v037Line создаёт строку с заданными структурой и элементами.
func v037Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v037Element вычисляет длину только входного диапазона, не ожидания диагностики.
func v037Element(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v037Text создаёт обычный текст без преобразований значения.
func v037Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v037Line(t, raw, "content", parent, depth, v037Element(t, "content", raw, raw, 1))
}

// v037ParserError задаёт явную ошибку исходного примера, сохраняя согласованность
// флагов. Код условный: каталог диагностик внешнего парсера недоступен.
func v037ParserError(t *testing.T, result *model.Result, line int, scope diagnostics.Scope, start, end int) {
	t.Helper()
	result.Document.HasErrors, result.Lines[line-1].HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error", SeverityLevel: "error", DiagnosticScope: scope, Message: "Ошибка исходного объявления.", Location: &model.Location{Start: model.Position{Line: line, Column: start}, End: model.Position{Line: line, Column: end}}, RelatedLocations: []model.Location{}})
}

// v037Run проверяет только V037 текущего запуска, выполняя весь штатный конвейер.
// want задаёт явное число срабатываний. В ситуациях с ошибочным аргументом его
// строка уже отмечена ошибкой парсера, поэтому новые ошибки V032 не меняют флаги.
func v037Run(t *testing.T, result *model.Result, processingID string, want int) {
	t.Helper()
	before := v037Snapshot(t, result)
	oldDocumentError := result.Document.HasErrors
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v037Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v037Snapshot(t, result) != before {
		t.Fatal("Prepare changed model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	count := 0
	seen := make(map[string]bool)
	for i, d := range result.Diagnostics {
		if d.ID == "" || seen[d.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", d.ID)
		}
		seen[d.ID] = true
		if i >= diagnosticCount && d.Source != processingID {
			t.Errorf("unexpected source %q", d.Source)
		}
		if d.Source != processingID || d.DiagnosticCode != "V037" {
			continue
		}
		count++
		if d.SeverityLevel != "error" || d.DiagnosticScope != "document" || d.Message != "В документе отсутствует @document-id." || d.Location != nil || d.RelatedLocations == nil || len(d.RelatedLocations) != 0 || d.Fatal {
			t.Errorf("unexpected V037 occurrence: %+v", d)
		}
	}
	if count != want {
		t.Fatalf("V037 count = %d, want %d; diagnostics: %+v", count, want, result.Diagnostics)
	}
	if result.Document.HasErrors != (oldDocumentError || want > 0) {
		t.Errorf("unexpected document.hasErrors: %v", result.Document.HasErrors)
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("changed processing count or lost diagnostic history")
	}
	previous := *result
	previous.Document.HasErrors = oldDocumentError
	previous.Processing = result.Processing[:processingCount]
	previous.Diagnostics = result.Diagnostics[:diagnosticCount]
	if v037Snapshot(t, &previous) != before {
		t.Fatal("source, line flags, element links or previous history changed")
	}
}

// v037Snapshot сохраняет модель для сравнения исходных данных и истории.
func v037Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

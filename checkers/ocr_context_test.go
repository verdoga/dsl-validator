package checkers

import (
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/model"
)

func TestOCRRecognizedTechnicalFields(t *testing.T) {
	for _, token := range []string{"GZ6K", "20O5", "he1o", "HE11O", "СЛ0ВО"} {
		for _, tc := range []struct{ tag, kind, prefix, suffix string }{
			{"document-id", "identifier", "", ""},
			{"task", "identifier", "", ""},
			{"include", "identifier", "", ""},
			{"resource-dir", "resource-path", "assets/", "/file"},
			{"resource-dir", "resource-path", `"файлы/`, `/file"`},
			{"media", "source", "assets/", "/file.wav"},
			{"media", "source", `"файлы\`, `\file.wav"`},
		} {
			t.Run(tc.tag+"/"+tc.prefix+token, func(t *testing.T) {
				prefix := "@" + tc.tag + " "
				elements := []model.Element{v021Element(t, "tag", "@"+tc.tag, tc.tag, 1, len(prefix))}
				if tc.tag == "media" {
					elements = append(elements, v021Element(t, "media-type", "audio", "audio", 8, 13))
					prefix += "audio "
				}
				raw := tc.prefix + token + tc.suffix
				start := utf8.RuneCountInString(prefix) + 1
				elements = append(elements, v021Element(t, model.ElementType(tc.kind), raw, raw, start, start+utf8.RuneCountInString(raw)))
				line := v021Line(t, prefix+raw, "tag", 2, 1, elements...)
				result := v021Document(t, line)
				if tc.tag == "document-id" || tc.tag == "resource-dir" {
					line.ParentLine, line.NestingLevel = nil, 0
					result = v021Model(t, result.Lines[0], line, result.Lines[1])
				}
				v021Run(t, result, "first", nil)
				ocrExpect(t, result, "first", nil, nil)
				v021Run(t, result, "second", nil)
				ocrExpect(t, result, "second", nil, nil)
			})
		}
	}
}

func TestOCRKeepsTextAndUnparsedBesideTechnicalFields(t *testing.T) {
	// Ошибочный хвост не исключается вместе с распознанным путём.
	path := `"Я/HE11O/20O5"`
	line := v021Line(t, "@media audio "+path+" HE11O 20O5", "invalid", 2, 1,
		v021Element(t, "tag", "@media", "media", 1, 7),
		v021Element(t, "media-type", "audio", "audio", 8, 13),
		v021Element(t, "source", path, "Я/HE11O/20O5", 14, 28),
		model.Element{ElementType: "unparsed", Raw: "HE11O 20O5", Start: 29, End: 39, ErrorIDs: []string{"parser-error"}})
	result := v021Document(t, line)
	result.Document.HasErrors, result.Lines[2].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: "Лишний аргумент.",
		Location: &model.Location{Start: model.Position{Line: 3, Column: 29}, End: model.Position{Line: 3, Column: 39}}, RelatedLocations: []model.Location{}}}
	v021Run(t, result, "current", nil)
	ocrExpect(t, result, "current", [][3]int{{3, 29, 34}}, [][3]int{{3, 35, 39}})

	// Вид пути в обычном тексте не превращает его в распознанный SOURCE.
	result = v021Document(t, v021Text(t, "Я /HE11O/ /20O5/", 2, 1))
	v021Run(t, result, "current", nil)
	ocrExpect(t, result, "current", [][3]int{{3, 4, 9}}, [][3]int{{3, 12, 16}})
}

// ocrExpect проверяет обе OCR-диагностики текущего запуска полного каталога.
// Диапазоны заданы в исходных Unicode-колонках, правая граница исключается.
func ocrExpect(t *testing.T, result *model.Result, source string, want22, want23 [][3]int) {
	t.Helper()
	for _, tc := range []struct {
		code string
		want [][3]int
	}{{"V022", want22}, {"V023", want23}} {
		var got []model.Diagnostic
		for _, entry := range result.Diagnostics {
			if string(entry.DiagnosticCode) == tc.code && entry.Source == source {
				got = append(got, entry)
			}
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s count=%d, want %d: %+v", tc.code, len(got), len(tc.want), got)
		}
		for i, want := range tc.want {
			location := model.Location{Start: model.Position{Line: want[0], Column: want[1]}, End: model.Position{Line: want[0], Column: want[2]}}
			if got[i].Location == nil || *got[i].Location != location || got[i].SeverityLevel != "warning" || got[i].DiagnosticScope != "line" {
				t.Errorf("%s occurrence=%+v, want location=%+v", tc.code, got[i], location)
			}
		}
	}
}

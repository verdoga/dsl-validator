package validator

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestCheckModelRejectsBrokenInvariantsWithoutChanges(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, tc := range []struct {
		name   string
		change func(*model.Result)
	}{
		{"format", func(r *model.Result) { r.Format = "2.0" }},
		{"document UTF-8", func(r *model.Result) { r.Document.Metadata.Title = text("\xff") }},
		{"resource path UTF-8", func(r *model.Result) { r.Document.Metadata.ResourceDirs = []string{"\xff"} }},
		{"empty file name", func(r *model.Result) { r.Document.FileName = text("") }},
		{"file name contains path", func(r *model.Result) { r.Document.FileName = text("dir/input.dsl") }},
		{"relative file path", func(r *model.Result) { r.Document.FilePath = text("input.dsl") }},
		{"unclean file path", func(r *model.Result) { r.Document.FilePath = text("/missing/../input.dsl") }},
		{"file path contains NUL", func(r *model.Result) { r.Document.FilePath = text("/missing/\x00input.dsl") }},
		{"path without name", func(r *model.Result) { r.Document.FileName = nil }},
		{"path name mismatch", func(r *model.Result) { r.Document.FileName = text("other.dsl") }},
		{"encoding", func(r *model.Result) { r.Document.Encoding = text("utf-8") }},
		{"negative byte length", func(r *model.Result) { *r.Document.ByteLength = -1 }},
		{"BOM larger than source", func(r *model.Result) { *r.Document.HasBOM, *r.Document.ByteLength = true, 2 }},
		{"short SHA256", func(r *model.Result) { r.Document.SHA256 = text("abc") }},
		{"uppercase SHA256", func(r *model.Result) { r.Document.SHA256 = text(strings.Repeat("A", 64)) }},
		{"nonhex SHA256", func(r *model.Result) { r.Document.SHA256 = text(strings.Repeat("g", 64)) }},
		{"null processing", func(r *model.Result) { r.Processing = nil }},
		{"empty processing ID", func(r *model.Result) { r.Processing[0].ID = "" }},
		{"duplicate processing ID", func(r *model.Result) { r.Processing[1].ID = r.Processing[0].ID }},
		{"empty tool", func(r *model.Result) { r.Processing[0].Tool = "" }},
		{"tool UTF-8", func(r *model.Result) { r.Processing[0].Tool = "\xff" }},
		{"version UTF-8", func(r *model.Result) { r.Processing[0].Version = text("\xff") }},
		{"timestamp without timezone", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-01-01T00:00:00") }},
		{"impossible date", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-02-30T00:00:00Z") }},
		{"single digit hour", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-01-01T1:00:00Z") }},
		{"comma fraction", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-01-01T00:00:00,5Z") }},
		{"timezone hour overflow", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-01-01T00:00:00+24:00") }},
		{"timezone minute overflow", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-01-01T00:00:00+03:60") }},
		{"negative duration", func(r *model.Result) { *r.Processing[0].DurationMs = -1 }},
		{"null lines", func(r *model.Result) { r.Lines = nil }},
		{"negative line count", func(r *model.Result) { *r.Document.LineCount = -1 }},
		{"line count mismatch", func(r *model.Result) { *r.Document.LineCount = 3 }},
		{"unknown count with lines", func(r *model.Result) { r.Document.LineCount = nil }},
		{"line numbering", func(r *model.Result) { r.Lines[1].Number = 1 }},
		{"line type", func(r *model.Result) { r.Lines[0].LineType = "unknown" }},
		{"line UTF-8", func(r *model.Result) { r.Lines[3].Raw = "\xff" }},
		{"raw newline", func(r *model.Result) { r.Lines[3].Raw = "\n" }},
		{"raw CR", func(r *model.Result) { r.Lines[3].Raw = "\r" }},
		{"raw BOM", func(r *model.Result) { r.Lines[0].Raw = "\uFEFF" + r.Lines[0].Raw }},
		{"CR ending", func(r *model.Result) { r.Lines[3].LineEnding = "\r" }},
		{"nonfinal missing EOL", func(r *model.Result) { r.Lines[0].LineEnding = "" }},
		{"fictitious final empty line", func(r *model.Result) { r.Lines[3].LineEnding = "" }},
		{"negative depth", func(r *model.Result) { r.Lines[3].NestingLevel = -1 }},
		{"root depth", func(r *model.Result) { r.Lines[3].NestingLevel = 1 }},
		{"parent zero", func(r *model.Result) { *r.Lines[1].ParentLine = 0 }},
		{"parent self", func(r *model.Result) { *r.Lines[1].ParentLine = 2 }},
		{"parent future", func(r *model.Result) { *r.Lines[1].ParentLine = 4 }},
		{"child depth", func(r *model.Result) { r.Lines[1].NestingLevel = 2 }},
		{"heading parent", func(r *model.Result) { r.Lines[1].LineType = model.LineTypeHeading }},
		{"block-end wrong parent", func(r *model.Result) { r.Lines[0].LineType = model.LineTypeTag }},
		{"null elements", func(r *model.Result) { r.Lines[3].Elements = nil }},
		{"unknown element type", func(r *model.Result) { r.Lines[1].Elements[0].ElementType = "unknown" }},
		{"null required value", func(r *model.Result) { r.Lines[1].Elements[0].Value = nil }},
		{"value UTF-8", func(r *model.Result) { r.Lines[1].Elements[0].Value = text("\xff") }},
		{"element start", func(r *model.Result) { r.Lines[1].Elements[0].Start = 0 }},
		{"element end", func(r *model.Result) { r.Lines[1].Elements[0].End = 4 }},
		{"element raw mismatch", func(r *model.Result) { r.Lines[1].Elements[0].Raw = "Я" }},
		{"element order", func(r *model.Result) {
			r.Lines[0].Elements[0], r.Lines[0].Elements[1] = r.Lines[0].Elements[1], r.Lines[0].Elements[0]
		}},
		{"element overlap", func(r *model.Result) { r.Lines[0].Elements = append(r.Lines[0].Elements[:1], r.Lines[0].Elements[0]) }},
		{"invalid without unparsed", func(r *model.Result) { r.Lines[1].LineType = model.LineTypeInvalid }},
		{"unparsed with value", func(r *model.Result) { r.Lines[1].Elements[0].ElementType = model.ElementTypeUnparsed }},
		{"unparsed without link", func(r *model.Result) {
			e := &r.Lines[1].Elements[0]
			e.ElementType, e.Value, e.ErrorIDs = model.ElementTypeUnparsed, nil, []string{}
		}},
		{"noncanonical tag", func(r *model.Result) { r.Lines[0].Elements[0].Value = text("TEXT") }},
		{"missing block-open", func(r *model.Result) { r.Lines[0].Elements = r.Lines[0].Elements[:1] }},
		{"missing block-close", func(r *model.Result) { r.Lines[2].Elements = []model.Element{} }},
		{"null errorIds", func(r *model.Result) { r.Lines[0].Elements[0].ErrorIDs = nil }},
		{"null diagnostics", func(r *model.Result) { r.Diagnostics = nil }},
		{"empty diagnostic ID", func(r *model.Result) { r.Diagnostics[0].ID = "" }},
		{"duplicate diagnostic ID", func(r *model.Result) { r.Diagnostics[2].ID = r.Diagnostics[0].ID }},
		{"unknown source", func(r *model.Result) { r.Diagnostics[0].Source = "absent" }},
		{"empty code", func(r *model.Result) { r.Diagnostics[0].DiagnosticCode = "" }},
		{"message UTF-8", func(r *model.Result) { r.Diagnostics[0].Message = "\xff" }},
		{"severity", func(r *model.Result) { r.Diagnostics[0].SeverityLevel = "fatal" }},
		{"scope", func(r *model.Result) { r.Diagnostics[0].DiagnosticScope = "unknown" }},
		{"fatal warning", func(r *model.Result) { r.Diagnostics[2].Fatal = true }},
		{"null relatedLocations", func(r *model.Result) { r.Diagnostics[0].RelatedLocations = nil }},
		{"element without location", func(r *model.Result) { r.Diagnostics[1].Location = nil }},
		{"range outside source", func(r *model.Result) { r.Diagnostics[2].Location.End.Column = 3 }},
		{"line spanning lines", func(r *model.Result) {
			r.Diagnostics[2].DiagnosticScope = diagnostics.ScopeLine
			r.Diagnostics[2].Location.End.Line = 4
		}},
		{"related invalid", func(r *model.Result) { r.Diagnostics[2].RelatedLocations[0].End.Column = 99 }},
		{"related matches main", func(r *model.Result) { r.Diagnostics[2].RelatedLocations[0] = *r.Diagnostics[2].Location }},
		{"related duplicate", func(r *model.Result) {
			r.Diagnostics[2].RelatedLocations = append(r.Diagnostics[2].RelatedLocations, r.Diagnostics[2].RelatedLocations[0])
		}},
		{"missing error reference", func(r *model.Result) { r.Lines[1].Elements[0].ErrorIDs[0] = "absent" }},
		{"duplicate error reference", func(r *model.Result) { r.Lines[1].Elements[0].ErrorIDs[1] = "old-e" }},
		{"error reference order", func(r *model.Result) { r.Lines[1].Elements[0].ErrorIDs = []string{"new-w", "old-e"} }},
		{"wrong reference scope", func(r *model.Result) { r.Lines[1].Elements[0].ErrorIDs = []string{"old-block"} }},
		{"wrong reference range", func(r *model.Result) { r.Lines[0].Elements[0].ErrorIDs = []string{"old-e"} }},
		{"missing backlink", func(r *model.Result) { r.Lines[1].Elements[0].ErrorIDs = []string{"old-e"} }},
		{"source order", func(r *model.Result) { r.Diagnostics[2], r.Diagnostics[3] = r.Diagnostics[3], r.Diagnostics[2] }},
		{"position order", func(r *model.Result) { r.Diagnostics[1], r.Diagnostics[2] = r.Diagnostics[2], r.Diagnostics[1] }},
		{"nil location order", func(r *model.Result) { r.Diagnostics[0], r.Diagnostics[1] = r.Diagnostics[1], r.Diagnostics[0] }},
		{"document false flag", func(r *model.Result) { r.Document.HasErrors = false }},
		{"line false flag", func(r *model.Result) { r.Lines[1].HasErrors = false }},
		{"parent inherited flag", func(r *model.Result) { r.Lines[0].HasErrors = true }},
		{"warning flag", func(r *model.Result) { r.Lines[2].HasErrors = true }},
		{"document true flag without errors", func(r *model.Result) {
			r.Diagnostics[1].SeverityLevel = diagnostics.SeverityWarning
			r.Lines[1].HasErrors = false
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integrityModel(t)
			tc.change(r)
			assertIntegrityResult(t, r, false)
		})
	}
	assertIntegrityResult(t, nil, false)
}

func TestCheckModelAcceptsHistoricalAndUnknownData(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, tc := range []struct {
		name   string
		change func(*model.Result)
	}{
		{"unchanged history", func(r *model.Result) {}},
		{"unknown document fields", func(r *model.Result) {
			count := r.Document.LineCount
			r.Document = model.Document{LineCount: count, HasErrors: true}
		}},
		{"unknown processing details", func(r *model.Result) {
			r.Processing[0].StartedAt, r.Processing[0].DurationMs, r.Processing[0].Version = nil, nil, nil
		}},
		{"processing ordered by array not time", func(r *model.Result) { r.Processing[1].StartedAt = text("2000-01-01T00:00:00Z") }},
		{"negative timezone", func(r *model.Result) { r.Processing[0].StartedAt = text("2026-01-01T00:00:00-05:30") }},
		{"null resourceDirs", func(r *model.Result) { r.Document.Metadata.ResourceDirs = nil }},
		{"empty resourceDirs", func(r *model.Result) { r.Document.Metadata.ResourceDirs = []string{} }},
		{"unsupported DSL version preserved", func(r *model.Result) { *r.Document.DSLVersion = "future-version" }},
		{"fatal historical error", func(r *model.Result) { r.Diagnostics[1].Fatal = true }},
		{"historic line without location", func(r *model.Result) { r.Diagnostics[0].DiagnosticScope = diagnostics.ScopeLine }},
		{"historic block without location", func(r *model.Result) { r.Diagnostics[0].DiagnosticScope = diagnostics.ScopeBlock }},
		{"invalid with linked unparsed", func(r *model.Result) {
			r.Lines[1].LineType = model.LineTypeInvalid
			r.Lines[1].Elements[0].ElementType, r.Lines[1].Elements[0].Value = model.ElementTypeUnparsed, nil
		}},
		{"unknown media with links", func(r *model.Result) {
			r.Lines[1].Elements[0].ElementType, r.Lines[1].Elements[0].Value = model.ElementTypeMediaType, nil
		}},
		{"equal positions in any detection order", func(r *model.Result) {
			r.Diagnostics[2].Location = r.Diagnostics[1].Location
			r.Diagnostics[1], r.Diagnostics[2] = r.Diagnostics[2], r.Diagnostics[1]
		}},
		{"last physical line without EOL", func(r *model.Result) { r.Lines[3].Raw, r.Lines[3].LineEnding = " ", "" }},
		{"heading root", func(r *model.Result) { r.Lines[3].LineType = model.LineTypeHeading }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integrityModel(t)
			tc.change(r)
			assertIntegrityResult(t, r, true)
		})
	}
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty lines unknown=%v", unknown), func(t *testing.T) {
			zero := 0
			r := &model.Result{Format: model.FormatVersion1, Document: model.Document{LineCount: &zero}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if unknown {
				r.Document.LineCount = nil
			}
			assertIntegrityResult(t, r, true)
			r.Processing = []model.Processing{{ID: "io", Tool: "unknown-tool"}}
			r.Diagnostics = []model.Diagnostic{{ID: "io-error", Source: "io", DiagnosticCode: "CUSTOM", SeverityLevel: diagnostics.SeverityError, DiagnosticScope: diagnostics.ScopeDocument, Fatal: true, RelatedLocations: []model.Location{}}}
			r.Document.HasErrors = true
			assertIntegrityResult(t, r, true)
		})
	}
}

func TestCheckModelRootBlockEndRequiresPrimaryError(t *testing.T) {
	for _, kind := range []string{"error", "warning", "recommendation", "related only", "no location"} {
		t.Run(kind, func(t *testing.T) {
			r := integrityModel(t)
			r.Lines[2].ParentLine, r.Lines[2].NestingLevel = nil, 0
			d := &r.Diagnostics[2]
			d.DiagnosticCode, d.SeverityLevel = "custom-root", diagnostics.SeverityError
			r.Lines[2].HasErrors = true
			switch kind {
			case "warning", "recommendation":
				d.SeverityLevel, r.Lines[2].HasErrors = diagnostics.Severity(kind), false
			case "related only":
				d.RelatedLocations = []model.Location{*d.Location}
				d.Location = r.Diagnostics[1].Location
				r.Lines[2].HasErrors = false
			case "no location":
				d.Location, r.Lines[2].HasErrors = nil, false
				r.Diagnostics[0], r.Diagnostics[2] = r.Diagnostics[2], r.Diagnostics[0]
			}
			assertIntegrityResult(t, r, kind == "error")
		})
	}
}

func TestCheckElementPositionsValuesAndUnicode(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, tc := range []struct {
		name  string
		kind  model.ElementType
		raw   string
		value *string
		valid bool
	}{
		{"Unicode", model.ElementTypeContent, "Я🌍e\u0301", text("смысл"), true},
		{"empty string", model.ElementTypeContent, "", text(""), true},
		{"null content", model.ElementTypeContent, "", nil, false},
		{"placeholder without answer", model.ElementTypePlaceholder, "_____", nil, true},
		{"placeholder with answer", model.ElementTypePlaceholder, "_____{Я}", text("Я"), true},
		{"heading one", model.ElementTypeHeadingLevel, "#", text("1"), true},
		{"heading three", model.ElementTypeHeadingLevel, "###", text("3"), true},
		{"heading mismatch", model.ElementTypeHeadingLevel, "##", text("1"), false},
		{"heading empty", model.ElementTypeHeadingLevel, "#", text(""), false},
		{"heading unknown", model.ElementTypeHeadingLevel, "####", text("4"), false},
		{"media audio", model.ElementTypeMediaType, "audio", text("audio"), true},
		{"media video", model.ElementTypeMediaType, "video", text("video"), true},
		{"media image", model.ElementTypeMediaType, "image", text("image"), true},
		{"media unknown", model.ElementTypeMediaType, "other", text("other"), false},
		{"media null without link", model.ElementTypeMediaType, "other", nil, false},
		{"open brace", model.ElementTypeBlockOpen, "{", text("{"), true},
		{"close brace", model.ElementTypeBlockClose, "}", text("}"), true},
		{"wrong brace value", model.ElementTypeBlockClose, "}", text("{"), false},
		{"wrong brace raw", model.ElementTypeBlockOpen, "x", text("{"), false},
		{"empty tag", model.ElementTypeTag, "@", text(""), false},
		{"canonical tag", model.ElementTypeTag, "@TEXT", text("text"), true},
		{"arbitrary name", model.ElementTypeName, "Свободное имя", text("Свободное имя"), true},
		{"old element version", model.ElementTypeVersion, "9.8", text("9.8"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := model.Line{Number: 1, LineType: model.LineTypeContent, Raw: tc.raw, Elements: []model.Element{{ElementType: tc.kind, Raw: tc.raw, Value: tc.value, Start: 1, End: len([]rune(tc.raw)) + 1, ErrorIDs: []string{}}}}
			r := &model.Result{Lines: []model.Line{line}}
			before := integritySnapshot(t, r)
			if err := checkElementPositions(line); (err == nil) != tc.valid {
				t.Errorf("checkElementPositions() = %v, valid=%v", err, tc.valid)
			}
			if integritySnapshot(t, r) != before {
				t.Error("element validation changed input")
			}
		})
	}
}

func TestErrorFlagsUsesOnlyPrimaryLocationsAndErrorReferences(t *testing.T) {
	r := integrityModel(t)
	r.Diagnostics[1].Location.End.Line = 3
	r.Diagnostics[1].RelatedLocations = []model.Location{{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 1}}}
	r.Lines[1].Elements[0].ErrorIDs = []string{}
	before := integritySnapshot(t, r)
	document, lines := errorFlags(r)
	if !document || !lines[2] || lines[1] || lines[3] || lines[4] {
		t.Fatalf("flags propagate beyond primary start: %v, %v", document, lines)
	}
	lines[1], lines[2] = true, false
	_, again := errorFlags(r)
	if again[1] || !again[2] || integritySnapshot(t, r) != before {
		t.Error("returned map is shared or model changed")
	}
	r.Diagnostics[1].Location = nil
	r.Lines[1].Elements[0].ErrorIDs = []string{"old-e"}
	document, lines = errorFlags(r)
	if !document || !lines[2] {
		t.Fatal("error reference did not mark its line")
	}
	r.Diagnostics[1].SeverityLevel = diagnostics.SeverityWarning
	document, lines = errorFlags(r)
	if document || len(lines) != 0 {
		t.Fatalf("nonerrors set flags: %v, %v", document, lines)
	}
}

func TestCheckModelEmptyElementBacklinksAndColumnOrder(t *testing.T) {
	r := integrityModel(t)
	empty, content := "", "x"
	r.Lines[3].LineType, r.Lines[3].Raw = model.LineTypeContent, content
	r.Lines[3].Elements = []model.Element{
		{ElementType: model.ElementTypeContent, Value: &empty, Start: 1, End: 1, ErrorIDs: []string{"empty"}},
		{ElementType: model.ElementTypeContent, Value: &empty, Start: 1, End: 1, ErrorIDs: []string{}},
		{ElementType: model.ElementTypeContent, Raw: content, Value: &content, Start: 1, End: 2, ErrorIDs: []string{}},
	}
	r.Diagnostics = append(r.Diagnostics, model.Diagnostic{
		ID: "empty", Source: "new", DiagnosticCode: "legacy-empty", SeverityLevel: diagnostics.SeverityRecommendation,
		DiagnosticScope: diagnostics.ScopeElement, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 1}},
	})
	assertIntegrityResult(t, r, true)
	r.Lines[3].Elements[1].ErrorIDs = []string{"empty"}
	assertIntegrityResult(t, r, true)
	r.Lines[3].Elements[0], r.Lines[3].Elements[2] = r.Lines[3].Elements[2], r.Lines[3].Elements[0]
	assertIntegrityResult(t, r, false)

	r = integrityModel(t)
	for i := 1; i <= 2; i++ {
		r.Diagnostics = append(r.Diagnostics, model.Diagnostic{
			ID: fmt.Sprintf("column-%d", i), Source: "new", DiagnosticCode: "custom", SeverityLevel: diagnostics.SeverityWarning,
			DiagnosticScope: diagnostics.ScopeLine, RelatedLocations: []model.Location{},
			Location: &model.Location{Start: model.Position{Line: 2, Column: i}, End: model.Position{Line: 2, Column: i}},
		})
	}
	assertIntegrityResult(t, r, true)
	r.Diagnostics[4], r.Diagnostics[5] = r.Diagnostics[5], r.Diagnostics[4]
	assertIntegrityResult(t, r, false)
}

func integrityModel(t *testing.T) *model.Result {
	t.Helper()
	version, fileName, filePath, encoding := "1.2", "input.dsl", "/missing/input.dsl", "UTF-8"
	hash, bom, count, size := strings.Repeat("a", 64), false, 4, int64(19)
	parent, duration, started := 1, 0, "2026-10-11T12:00:00.123456789+03:00"
	text, open, close, content := "text", "{", "}", "Я🌍"
	location := model.Location{Start: model.Position{Line: 2, Column: 1}, End: model.Position{Line: 2, Column: 3}}
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, FileName: &fileName, FilePath: &filePath, Encoding: &encoding,
			HasBOM: &bom, LineCount: &count, ByteLength: &size, SHA256: &hash, HasErrors: true},
		Processing: []model.Processing{{ID: "old", Tool: "dsl-parser", StartedAt: &started, DurationMs: &duration}, {ID: "new", Tool: "other-tool"}},
		Lines: []model.Line{
			{Number: 1, LineType: model.LineTypeBlockStart, Raw: "@text {", LineEnding: model.LineEndingLF, Elements: []model.Element{
				{ElementType: model.ElementTypeTag, Raw: "@text", Value: &text, Start: 1, End: 6, ErrorIDs: []string{}},
				{ElementType: model.ElementTypeBlockOpen, Raw: "{", Value: &open, Start: 7, End: 8, ErrorIDs: []string{}},
			}},
			{Number: 2, LineType: model.LineTypeContent, Raw: content, LineEnding: model.LineEndingCRLF, NestingLevel: 1, ParentLine: &parent, HasErrors: true,
				Elements: []model.Element{{ElementType: model.ElementTypeContent, Raw: content, Value: &content, Start: 1, End: 3, ErrorIDs: []string{"old-e", "new-w"}}}},
			{Number: 3, LineType: model.LineTypeBlockEnd, Raw: "}", LineEnding: model.LineEndingLF, NestingLevel: 1, ParentLine: &parent,
				Elements: []model.Element{{ElementType: model.ElementTypeBlockClose, Raw: "}", Value: &close, Start: 1, End: 2, ErrorIDs: []string{}}}},
			{Number: 4, LineType: model.LineTypeBlank, LineEnding: model.LineEndingLF, Elements: []model.Element{}},
		},
		Diagnostics: []model.Diagnostic{
			{ID: "old-doc", Source: "old", DiagnosticCode: "custom/code", SeverityLevel: diagnostics.SeverityRecommendation, DiagnosticScope: diagnostics.ScopeDocument, RelatedLocations: []model.Location{}},
			{ID: "old-e", Source: "old", DiagnosticCode: "P001", SeverityLevel: diagnostics.SeverityError, DiagnosticScope: diagnostics.ScopeElement, Location: &location, RelatedLocations: []model.Location{}},
			{ID: "old-block", Source: "old", DiagnosticCode: "legacy", SeverityLevel: diagnostics.SeverityWarning, DiagnosticScope: diagnostics.ScopeBlock,
				Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 2}},
				RelatedLocations: []model.Location{{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 6}}}},
			{ID: "new-w", Source: "new", DiagnosticCode: "V001", SeverityLevel: diagnostics.SeverityWarning, DiagnosticScope: diagnostics.ScopeElement, Location: &location, RelatedLocations: []model.Location{}},
		},
	}
}

func assertIntegrityResult(t *testing.T, r *model.Result, valid bool) {
	t.Helper()
	before := integritySnapshot(t, r)
	err := CheckModel(r)
	if (err == nil) != valid {
		t.Errorf("CheckModel() = %v, valid=%v", err, valid)
	}
	if integritySnapshot(t, r) != before {
		t.Error("CheckModel changed its input")
	}
}

func integritySnapshot(t *testing.T, r *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	// JSON фиксирует вложенные значения и null; %#v дополнительно сохраняет
	// неверный UTF-8 и адреса указателей. Значения строковых указателей ниже
	// фиксируются отдельно, поскольку JSON заменяет неверные байты на U+FFFD.
	snapshot := fmt.Sprintf("%#v %s", r, encoded)
	if r == nil {
		return snapshot
	}
	d := r.Document
	texts := []*string{d.DSLVersion, d.FileName, d.FilePath, d.Encoding, d.SHA256,
		d.Metadata.DocumentID, d.Metadata.Title, d.Metadata.Subtitle, d.Metadata.Section, d.Metadata.Order}
	for _, entry := range r.Processing {
		texts = append(texts, entry.Version, entry.StartedAt)
	}
	for _, line := range r.Lines {
		for _, element := range line.Elements {
			texts = append(texts, element.Value)
		}
	}
	for _, text := range texts {
		if text != nil {
			snapshot += fmt.Sprintf(" %q", *text)
		}
	}
	return snapshot
}

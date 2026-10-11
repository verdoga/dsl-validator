package validator

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestCopyForUpdateIsolatesMutableParts(t *testing.T) {
	original := integrityModel(t)
	original.Document.Metadata.ResourceDirs = []string{"assets"}
	before := integritySnapshot(t, original)
	candidate := copyForUpdate(original)
	if applySnapshot(t, candidate) != applySnapshot(t, original) {
		t.Fatal("copy changed model contents")
	}
	if candidate.Document.DSLVersion != original.Document.DSLVersion || candidate.Document.LineCount != original.Document.LineCount ||
		&candidate.Document.Metadata.ResourceDirs[0] != &original.Document.Metadata.ResourceDirs[0] || &candidate.Processing[0] != &original.Processing[0] {
		t.Error("copy rewrote immutable document fields or processing")
	}
	candidate.Document.HasErrors = false
	candidate.Diagnostics[0].Message = "changed"
	candidate.Diagnostics[1].Location.Start.Column = 99
	candidate.Diagnostics[2].RelatedLocations[0].End.Column = 99
	candidate.Lines[0].Raw = "changed"
	candidate.Lines[1].HasErrors = false
	*candidate.Lines[1].ParentLine = 99
	*candidate.Lines[1].Elements[0].Value = "changed"
	candidate.Lines[1].Elements[0].ErrorIDs[0] = "changed"
	candidate.Lines[1].Elements[0].Raw = "changed"
	if integritySnapshot(t, original) != before {
		t.Fatal("changing candidate changed original model")
	}
	candidateBefore := integritySnapshot(t, candidate)
	original.Diagnostics[1].Location.End.Column = 77
	original.Diagnostics[2].RelatedLocations[0].Start.Column = 77
	*original.Lines[1].ParentLine = 77
	*original.Lines[1].Elements[0].Value = "source changed"
	original.Lines[1].Elements[0].ErrorIDs[0] = "source changed"
	original.Lines[0] = model.Line{}
	if integritySnapshot(t, candidate) != candidateBefore {
		t.Error("changing original model changed candidate")
	}
}

func TestCopyForUpdatePreservesNullAndEmptyValues(t *testing.T) {
	for _, r := range []*model.Result{
		{},
		{Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}},
		{Lines: []model.Line{{Elements: nil}, {Elements: []model.Element{}}}, Diagnostics: []model.Diagnostic{{RelatedLocations: nil}, {RelatedLocations: []model.Location{}}}},
		{Lines: []model.Line{{Elements: []model.Element{{ErrorIDs: nil}, {ErrorIDs: []string{}}}}}},
	} {
		if got := copyForUpdate(r); applySnapshot(t, got) != applySnapshot(t, r) {
			t.Errorf("copy changed null or empty fields: %+v", got)
		}
	}
}

func TestNextDiagnosticIDUsesAllExistingIDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
		want string
	}{
		{"empty", nil, "d1"},
		{"first occupied", []string{"d1"}, "d2"},
		{"gap", []string{"d3", "d1"}, "d2"},
		{"several runs", []string{"d2", "d1", "d3"}, "d4"},
		{"other IDs", []string{"d0", "d01", "d-1", "D1", "custom", "d999999999999999999999999999999"}, "d1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := make([]model.Diagnostic, len(tc.ids))
			for i, id := range tc.ids {
				entries[i] = model.Diagnostic{ID: id}
			}
			if got := nextDiagnosticID(entries); got != tc.want {
				t.Errorf("nextDiagnosticID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyOccurrencesPreservesHistoryAndLinksAllSeverities(t *testing.T) {
	original := integrityModel(t)
	original.Diagnostics[0].ID, original.Diagnostics[2].ID = "d1", "d3"
	before := integritySnapshot(t, original)
	candidate := copyForUpdate(original)
	candidate.Processing = append(candidate.Processing, model.Processing{ID: "p100", Tool: "dsl-validator"})
	occurrences := []model.Diagnostic{}
	for _, severity := range []diagnostics.Severity{diagnostics.SeverityError, diagnostics.SeverityWarning, diagnostics.SeverityRecommendation} {
		occurrences = append(occurrences, model.Diagnostic{
			DiagnosticCode: "V001", SeverityLevel: severity, DiagnosticScope: diagnostics.ScopeElement, Message: string(severity),
			Location: &model.Location{Start: model.Position{Line: 2, Column: 1}, End: model.Position{Line: 2, Column: 3}}, RelatedLocations: []model.Location{},
		})
	}
	occurrences = append(occurrences,
		model.Diagnostic{DiagnosticCode: "V002", SeverityLevel: diagnostics.SeverityWarning, DiagnosticScope: diagnostics.ScopeLine,
			Location: &model.Location{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 6}}, RelatedLocations: []model.Location{}},
		model.Diagnostic{DiagnosticCode: "V003", SeverityLevel: diagnostics.SeverityError, DiagnosticScope: diagnostics.ScopeBlock,
			Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 2}},
			RelatedLocations: []model.Location{{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 1}}}},
		model.Diagnostic{DiagnosticCode: "V004", SeverityLevel: diagnostics.SeverityRecommendation, DiagnosticScope: diagnostics.ScopeDocument, RelatedLocations: []model.Location{}},
	)
	inputBefore := applySnapshot(t, &model.Result{Diagnostics: occurrences})
	if err := appendOccurrences(candidate, "p100", occurrences); err != nil {
		t.Fatal(err)
	}
	if len(candidate.Diagnostics) != 10 {
		t.Fatalf("got %d diagnostics, want 10", len(candidate.Diagnostics))
	}
	for i, id := range []string{"d2", "d4", "d5", "d6", "d7", "d8"} {
		entry := candidate.Diagnostics[4+i]
		if entry.ID != id || entry.Source != "p100" || entry.Message != occurrences[i].Message || entry.DiagnosticCode != occurrences[i].DiagnosticCode || entry.SeverityLevel != occurrences[i].SeverityLevel || entry.DiagnosticScope != occurrences[i].DiagnosticScope || entry.Fatal {
			t.Errorf("new diagnostic %d has unexpected attributes: %+v", i, entry)
		}
	}
	wantLinks := []string{"old-e", "new-w", "d2", "d4", "d5"}
	if !slices.Equal(candidate.Lines[1].Elements[0].ErrorIDs, wantLinks) {
		t.Fatalf("element references = %v, want %v", candidate.Lines[1].Elements[0].ErrorIDs, wantLinks)
	}
	// Старый префикс ссылок остаётся в исходном порядке; новые ссылки
	// намеренно перемешаны для проверки согласования с Diagnostics.
	candidate.Lines[1].Elements[0].ErrorIDs = []string{"old-e", "new-w", "d5", "d2", "d4"}
	orderNewDiagnostics(candidate, "p100")
	setErrorFlags(candidate)
	for i, id := range []string{"d8", "d6", "d2", "d4", "d5", "d7"} {
		if candidate.Diagnostics[4+i].ID != id {
			t.Errorf("sorted new diagnostic %d = %q, want %q", i, candidate.Diagnostics[4+i].ID, id)
		}
	}
	for i := range original.Diagnostics {
		if rangeSnapshot(t, nil, candidate.Diagnostics[i]) != rangeSnapshot(t, nil, original.Diagnostics[i]) {
			t.Errorf("historical diagnostic %d changed", i)
		}
	}
	if !slices.Equal(candidate.Lines[1].Elements[0].ErrorIDs, wantLinks) || len(candidate.Lines[0].Elements[0].ErrorIDs) != 0 || len(candidate.Lines[2].Elements[0].ErrorIDs) != 0 {
		t.Error("element references are unordered or include non-element diagnostics")
	}
	assertIntegrityResult(t, candidate, true)
	if !candidate.Document.HasErrors || candidate.Lines[0].HasErrors || !candidate.Lines[1].HasErrors || !candidate.Lines[2].HasErrors || candidate.Lines[3].HasErrors {
		t.Error("flags do not reflect primary error locations across runs")
	}
	if integritySnapshot(t, original) != before || applySnapshot(t, &model.Result{Diagnostics: occurrences}) != inputBefore {
		t.Error("applying occurrences changed original model or supplied occurrences")
	}
}

func TestLinkElementErrorSelectsFirstEmptyElementAndAvoidsDuplicates(t *testing.T) {
	lines := []model.Line{{Number: 1, Raw: "x", Elements: []model.Element{
		{Start: 1, End: 1, ErrorIDs: []string{"prior"}}, {Start: 1, End: 1, ErrorIDs: []string{}},
	}}}
	entry := model.Diagnostic{ID: "d9", DiagnosticScope: diagnostics.ScopeElement,
		Location: &model.Location{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 1}}}
	for i := 0; i < 2; i++ {
		if err := linkElementError(lines, entry); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(lines[0].Elements[0].ErrorIDs, []string{"prior", "d9"}) || len(lines[0].Elements[1].ErrorIDs) != 0 {
		t.Fatalf("wrong element selected or duplicate reference added: %+v", lines[0].Elements)
	}
}

func TestAppendOccurrencesRejectsMissingElement(t *testing.T) {
	for _, location := range []*model.Location{
		nil,
		{Start: model.Position{Line: 0, Column: 1}, End: model.Position{Line: 0, Column: 2}},
		{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 2}},
		{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 2, Column: 1}},
		{Start: model.Position{Line: 2, Column: 1}, End: model.Position{Line: 2, Column: 2}},
	} {
		r := integrityModel(t)
		before := integritySnapshot(t, r)
		entry := model.Diagnostic{DiagnosticScope: diagnostics.ScopeElement, Location: location}
		if err := appendOccurrences(r, "current", []model.Diagnostic{entry}); err == nil {
			t.Errorf("expected missing-element error for %v", location)
		}
		if integritySnapshot(t, r) != before {
			t.Error("failed element attachment changed existing model")
		}
	}
}

func TestOrderNewDiagnosticsIsStableAndUsesStartOnly(t *testing.T) {
	r := &model.Result{Diagnostics: []model.Diagnostic{
		{ID: "old", Source: "prior", Location: &model.Location{Start: model.Position{Line: 9, Column: 9}}},
		{ID: "long", Source: "current", Location: &model.Location{Start: model.Position{Line: 2, Column: 1}, End: model.Position{Line: 3, Column: 1}}},
		{ID: "nil-first", Source: "current"},
		{ID: "right", Source: "current", Location: &model.Location{Start: model.Position{Line: 2, Column: 2}}},
		{ID: "short", Source: "current", Location: &model.Location{Start: model.Position{Line: 2, Column: 1}, End: model.Position{Line: 2, Column: 1}}},
		{ID: "nil-second", Source: "current"},
	}}
	orderNewDiagnostics(r, "current")
	for i, id := range []string{"old", "nil-first", "nil-second", "long", "short", "right"} {
		if r.Diagnostics[i].ID != id {
			t.Errorf("diagnostic %d = %q, want %q", i, r.Diagnostics[i].ID, id)
		}
	}
	before := applySnapshot(t, r)
	if err := appendOccurrences(r, "absent", nil); err != nil {
		t.Fatal(err)
	}
	orderNewDiagnostics(r, "absent")
	if applySnapshot(t, r) != before {
		t.Error("empty append or sorting absent run changed model")
	}
}

func TestSetErrorFlagsClearsStaleFlagsAndDoesNotPropagate(t *testing.T) {
	r := integrityModel(t)
	r.Diagnostics[1].Location.End.Line = 3
	r.Diagnostics[1].RelatedLocations = []model.Location{{Start: model.Position{Line: 4, Column: 1}, End: model.Position{Line: 4, Column: 1}}}
	for i := range r.Lines {
		r.Lines[i].HasErrors = true
	}
	r.Document.HasErrors = false
	setErrorFlags(r)
	if !r.Document.HasErrors || r.Lines[0].HasErrors || !r.Lines[1].HasErrors || r.Lines[2].HasErrors || r.Lines[3].HasErrors {
		t.Fatal("error flags propagated to parent, range end or related location")
	}
	r.Diagnostics[1].Location = nil
	setErrorFlags(r)
	if !r.Document.HasErrors || !r.Lines[1].HasErrors {
		t.Error("error backlink did not set flags")
	}
	r.Lines[1].Elements[0].ErrorIDs = []string{"new-w"}
	setErrorFlags(r)
	if !r.Document.HasErrors || r.Lines[1].HasErrors {
		t.Error("unpositioned document error or warning backlink set incorrect flags")
	}
	r.Diagnostics[1].SeverityLevel = diagnostics.SeverityRecommendation
	setErrorFlags(r)
	if r.Document.HasErrors || slices.ContainsFunc(r.Lines, func(line model.Line) bool { return line.HasErrors }) {
		t.Error("non-error severities retained error flags")
	}
}

func applySnapshot(t *testing.T, r *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

package validator

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestCheckFailurePreservesCodeAndCauseThroughWrapping(t *testing.T) {
	cause := &strconv.NumError{Func: "check", Num: "x", Err: strconv.ErrSyntax}
	failure := &CheckFailure{Code: "V1000", Err: cause}
	if failure.Error() != "сбой диагностики V1000: "+cause.Error() || failure.Unwrap() != cause {
		t.Fatalf("unexpected failure message or cause: %v", failure)
	}
	wrapped := fmt.Errorf("file: %w", failure)
	var got *CheckFailure
	var gotCause *strconv.NumError
	if !errors.As(wrapped, &got) || got != failure || !errors.As(wrapped, &gotCause) || gotCause != cause || !errors.Is(wrapped, strconv.ErrSyntax) || !strings.Contains(wrapped.Error(), "V1000") {
		t.Fatalf("wrapped failure lost code or cause: %v", wrapped)
	}
}

func TestRunCommitsAllSeveritiesAfterSuccessfulChecks(t *testing.T) {
	r := integrityModel(t)
	var calls []diagnostics.Code
	var collectors []diagnostics.Findings
	var before string
	definitions := []diagnostics.Definition{}
	for _, tc := range []struct {
		code     diagnostics.Code
		severity diagnostics.Severity
		scope    diagnostics.Scope
		coords   [4]int
		message  string
	}{
		{"V1000", diagnostics.SeverityRecommendation, diagnostics.ScopeDocument, [4]int{}, " \t"},
		{"V002", diagnostics.SeverityWarning, diagnostics.ScopeLine, [4]int{1, 1, 1, 6}, "подробности\nстрока"},
		{"V001", diagnostics.SeverityError, diagnostics.ScopeElement, [4]int{2, 1, 2, 3}, ""},
	} {
		definition := runDefinition(t, tc.code, func(lines diagnostics.Lines, findings diagnostics.Findings) error {
			calls = append(calls, tc.code)
			if integritySnapshot(t, r) != before {
				t.Error("model changed before all checks completed")
			}
			if lines.Len() != 4 {
				t.Fatalf("Lines.Len() = %d, want 4", lines.Len())
			}
			for _, line := range lines.(*linesView).lines {
				if line.HasErrors {
					t.Error("check received HasErrors")
				}
				for _, element := range line.Elements {
					if element.ErrorIDs != nil {
						t.Error("check received ErrorIDs")
					}
				}
			}
			for _, previous := range collectors {
				if previous == findings {
					t.Error("checks shared a collector")
				}
			}
			collectors = append(collectors, findings)
			p := tc.coords
			number, err := findings.Add(p[0], p[1], p[2], p[3], tc.message)
			if number != 1 || err != nil {
				t.Fatalf("Add() = %d, %v; want local occurrence 1", number, err)
			}
			return nil
		})
		definition.Scope, definition.Severity = tc.scope, tc.severity
		definitions = append(definitions, definition)
	}
	checks := runPrepared(t, r, definitions...)
	if len(calls) != 0 {
		t.Fatal("Prepare executed Check")
	}
	before = integritySnapshot(t, r)
	historical := *r
	if err := Run(r, checks, "current"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []diagnostics.Code{"V001", "V002", "V1000"}) {
		t.Errorf("Check order = %v", calls)
	}
	if len(r.Diagnostics) != 7 || len(r.Processing) != 3 {
		t.Fatalf("unexpected record counts: diagnostics=%d, processing=%d", len(r.Diagnostics), len(r.Processing))
	}
	for i, want := range []struct {
		code     diagnostics.Code
		severity diagnostics.Severity
		scope    diagnostics.Scope
		message  string
	}{
		{"V1000", diagnostics.SeverityRecommendation, diagnostics.ScopeDocument, "общее V1000\nконкретизация:  \t"},
		{"V002", diagnostics.SeverityWarning, diagnostics.ScopeLine, "общее V002\nконкретизация: подробности\nстрока"},
		{"V001", diagnostics.SeverityError, diagnostics.ScopeElement, "общее V001"},
	} {
		entry := r.Diagnostics[4+i]
		if entry.DiagnosticCode != want.code || entry.SeverityLevel != want.severity || entry.DiagnosticScope != want.scope || entry.Message != want.message || entry.Fatal || entry.Source != "current" || entry.ID == "" {
			t.Errorf("unexpected committed occurrence: %+v", entry)
		}
	}
	if !slices.Equal(r.Lines[1].Elements[0].ErrorIDs, []string{"old-e", "new-w", r.Diagnostics[6].ID}) || !r.Document.HasErrors || !r.Lines[1].HasErrors || r.Lines[0].HasErrors {
		t.Error("committed backlinks or error flags are incorrect")
	}
	assertIntegrityResult(t, r, true)
	if integritySnapshot(t, &historical) != before {
		t.Error("Run modified memory belonging to the input model")
	}
	committed := integritySnapshot(t, r)
	for _, findings := range collectors {
		if number, err := findings.Add(0, 0, 0, 0, "late"); number != 0 || err == nil {
			t.Error("successful Check left collector open")
		}
	}
	if integritySnapshot(t, r) != committed {
		t.Error("closed collector call changed committed model")
	}
}

func TestRunWithoutOccurrencesAndWithUnknownLineCount(t *testing.T) {
	version := "1.2"
	r := &model.Result{Format: model.FormatVersion1, Document: model.Document{DSLVersion: &version}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
	calls := 0
	checks := runPrepared(t, r, runDefinition(t, "V001", func(lines diagnostics.Lines, findings diagnostics.Findings) error {
		calls++
		if lines.Len() != 0 {
			t.Error("unknown line count produced fictitious lines")
		}
		return nil
	}))
	before := applySnapshot(t, r)
	if err := Run(r, checks, "current"); err != nil || calls != 1 {
		t.Fatalf("Run() = %v; calls=%d", err, calls)
	}
	if applySnapshot(t, r) != before || r.Document.LineCount != nil {
		t.Error("no-findings run changed content or invented a line count")
	}
}

func TestRunRejectsInvalidCurrentProcessingBeforeChecks(t *testing.T) {
	for _, kind := range []string{"nil result", "nil checks", "empty checks", "missing processing", "empty ID", "wrong ID", "not last", "duplicate ID", "already used"} {
		t.Run(kind, func(t *testing.T) {
			r := integrityModel(t)
			calls := 0
			checks := runPrepared(t, r, runDefinition(t, "V001", func(diagnostics.Lines, diagnostics.Findings) error { calls++; return nil }))
			id := "current"
			switch kind {
			case "nil result":
				r = nil
			case "nil checks":
				checks = nil
			case "empty checks":
				checks = []diagnostics.Definition{}
			case "missing processing":
				r.Processing = nil
			case "empty ID":
				id, r.Processing[2].ID = "", ""
			case "wrong ID":
				id = "absent"
			case "not last":
				id = "new"
			case "duplicate ID":
				r.Processing[0].ID = id
			case "already used":
				r.Diagnostics[0].Source = id
			}
			before := integritySnapshot(t, r)
			if err := Run(r, checks, id); err == nil {
				t.Error("Run accepted invalid input")
			}
			if calls != 0 || integritySnapshot(t, r) != before {
				t.Error("invalid input ran Check or changed model")
			}
		})
	}
}

func TestRunStopsAndRollsBackOnCheckFailure(t *testing.T) {
	for _, kind := range []string{"returned error", "ignored Add error", "ignored AddRelated error", "collector before return", "panic error", "panic string", "panic nil", "collector before panic"} {
		t.Run(kind, func(t *testing.T) {
			r := integrityModel(t)
			cause := &strconv.NumError{Func: "check", Num: "x", Err: strconv.ErrSyntax}
			var first error
			var retained diagnostics.Findings
			var calls []diagnostics.Code
			firstCheck := runDefinition(t, "V001", func(_ diagnostics.Lines, f diagnostics.Findings) error {
				calls = append(calls, "V001")
				_, err := f.Add(2, 1, 2, 3, "earlier success")
				return err
			})
			firstCheck.Scope = diagnostics.ScopeElement
			failedCheck := runDefinition(t, "V002", func(_ diagnostics.Lines, f diagnostics.Findings) error {
				calls = append(calls, "V002")
				retained = f
				if _, err := f.Add(0, 0, 0, 0, "partial"); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "ignored Add error", "collector before return", "collector before panic":
					_, first = f.Add(1, 0, 1, 1, "invalid")
				case "ignored AddRelated error":
					first = f.AddRelated(2, 1, 1, 1, 1)
				}
				switch kind {
				case "returned error", "collector before return":
					return cause
				case "panic error", "collector before panic":
					panic(cause)
				case "panic string":
					panic("unexpected defect")
				case "panic nil":
					panic(nil)
				}
				return nil
			})
			lastCheck := runDefinition(t, "V003", func(diagnostics.Lines, diagnostics.Findings) error { calls = append(calls, "V003"); return nil })
			checks := runPrepared(t, r, lastCheck, failedCheck, firstCheck)
			before := integritySnapshot(t, r)
			err := Run(r, checks, "current")
			failure, ok := err.(*CheckFailure)
			if !ok || failure.Code != "V002" || failure.Err == nil {
				t.Fatalf("Run() = %T %v, want CheckFailure for V002", err, err)
			}
			if first != nil && (!errors.Is(err, first) || failure.Err != first || errors.Is(err, cause)) {
				t.Errorf("collector failure lost priority: %v", err)
			}
			if kind == "returned error" || kind == "panic error" {
				var found *strconv.NumError
				if !errors.Is(err, cause) || !errors.As(err, &found) || found != cause {
					t.Errorf("Check cause lost: %v", err)
				}
			}
			if !slices.Equal(calls, []diagnostics.Code{"V001", "V002"}) || integritySnapshot(t, r) != before {
				t.Error("Run continued after failure or retained partial changes")
			}
			if !retained.(*collector).closed {
				t.Error("failed Check left collector open")
			}
			if number, err := retained.Add(0, 0, 0, 0, "late"); number != 0 || err == nil {
				t.Error("failed Check collector accepted a late occurrence")
			}
		})
	}
}

func TestRunFinalIntegrityFailureRollsBackAfterAllChecks(t *testing.T) {
	r := integrityModel(t)
	calls := 0
	definition := runDefinition(t, "V001", func(_ diagnostics.Lines, findings diagnostics.Findings) error {
		calls++
		_, err := findings.Add(2, 1, 2, 3, "would change links and flags")
		return err
	})
	definition.Scope = diagnostics.ScopeElement
	checks := runPrepared(t, r, definition, runDefinition(t, "V002", func(diagnostics.Lines, diagnostics.Findings) error { calls++; return nil }))
	// Ошибка только в новой записи app: полный входной контроль не должен
	// повторяться до Check, но итоговый контроль обязан отклонить кандидата.
	negative := -1
	r.Processing[len(r.Processing)-1].DurationMs = &negative
	before := integritySnapshot(t, r)
	err := Run(r, checks, "current")
	var failure *CheckFailure
	if err == nil || errors.As(err, &failure) || !strings.Contains(err.Error(), "итоговая целостность") || calls != 2 {
		t.Fatalf("Run() = %v; Check calls=%d, want final integrity error after both checks", err, calls)
	}
	if integritySnapshot(t, r) != before {
		t.Error("final integrity failure changed input model")
	}
}

func TestRunCheckDiscardsFindingsAndAssignsOnlyDefinitionAttributes(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%v", fail), func(t *testing.T) {
			cause := errors.New("check failure")
			var retained diagnostics.Findings
			definition := runDefinition(t, "V999", func(_ diagnostics.Lines, f diagnostics.Findings) error {
				retained = f
				if _, err := f.Add(0, 0, 0, 0, "конкретно"); err != nil {
					return err
				}
				if fail {
					return cause
				}
				return nil
			})
			entries, err := runCheck(newLinesView(nil), definition)
			if !retained.(*collector).closed {
				t.Error("runCheck did not close collector")
			}
			if fail {
				if entries != nil || !errors.Is(err, cause) {
					t.Fatalf("runCheck() = %v, %v; want nil and original cause", entries, err)
				}
				return
			}
			if err != nil || len(entries) != 1 {
				t.Fatalf("runCheck() = %v, %v", entries, err)
			}
			entry := entries[0]
			if entry.ID != "" || entry.Source != "" || entry.DiagnosticCode != definition.Code || entry.SeverityLevel != definition.Severity || entry.DiagnosticScope != definition.Scope || entry.Fatal || entry.Message != "общее V999\nконкретизация: конкретно" {
				t.Errorf("unexpected preliminary occurrence: %+v", entry)
			}
		})
	}
}

func runDefinition(t *testing.T, code diagnostics.Code, check diagnostics.CheckFunc) diagnostics.Definition {
	t.Helper()
	return diagnostics.Definition{Code: code, Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeDocument,
		Message: "общее " + string(code), Versions: diagnostics.VersionRule{Mode: diagnostics.VersionAll}, Check: check}
}

func runPrepared(t *testing.T, r *model.Result, definitions ...diagnostics.Definition) []diagnostics.Definition {
	t.Helper()
	registry, err := diagnostics.NewRegistry(map[string]diagnostics.RegistrationFunc{"checks": func(registry *diagnostics.Registry) error {
		for _, definition := range definitions {
			if err := registry.Register(definition); err != nil {
				return err
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	checks, err := Prepare(r, registry)
	if err != nil {
		t.Fatal(err)
	}
	startedAt, version := "2026-10-11T12:00:00Z", "test-version"
	r.Processing = append(r.Processing, model.Processing{ID: "current", Tool: "dsl-validator", Version: &version, StartedAt: &startedAt})
	return checks
}

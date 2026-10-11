package validator

import (
	"encoding/json"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestCollectorAccumulatesIndependentOccurrences(t *testing.T) {
	c := newCollector(collectorLines(t), diagnostics.ScopeElement)
	var findings diagnostics.Findings = c
	messages := []string{"", "Конкретная проблема 🌍\nподробности", " \t "}
	for i, message := range messages {
		if number, err := findings.Add(1, 1, 1, 2, message); err != nil || number != i+1 {
			t.Fatalf("Add() = %d, %v; want %d, nil", number, err, i+1)
		}
	}
	if err := findings.AddRelated(2, 2, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if err := findings.AddRelated(2, 1, 3, 2, 1); err != nil {
		t.Fatal(err)
	}
	entries, err := c.finish()
	if err != nil || len(entries) != len(messages) {
		t.Fatalf("finish() = %v, %v", entries, err)
	}
	want := model.Location{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 2}}
	for i, entry := range entries {
		if entry.Message != messages[i] || entry.Location == nil || *entry.Location != want {
			t.Errorf("occurrence %d: got %+v", i+1, entry)
		}
		if entry.ID != "" || entry.Source != "" || entry.DiagnosticCode != "" || entry.SeverityLevel != "" || entry.DiagnosticScope != "" || entry.Fatal {
			t.Errorf("occurrence %d: collector assigned attributes owned by the pipeline", i+1)
		}
		if entry.RelatedLocations == nil {
			t.Errorf("occurrence %d: relatedLocations must be an array", i+1)
		}
	}
	if len(entries[0].RelatedLocations) != 0 || len(entries[2].RelatedLocations) != 0 || len(entries[1].RelatedLocations) != 2 {
		t.Fatalf("related locations attached to wrong occurrence: %+v", entries)
	}
	for i, want := range []model.Location{
		{Start: model.Position{Line: 2, Column: 1}, End: model.Position{Line: 2, Column: 3}},
		{Start: model.Position{Line: 1, Column: 3}, End: model.Position{Line: 2, Column: 1}},
	} {
		if entries[1].RelatedLocations[i] != want {
			t.Errorf("related location %d = %v, want %v", i, entries[1].RelatedLocations[i], want)
		}
	}
	other := newCollector(c.lines, diagnostics.ScopeElement)
	if number, err := other.Add(1, 1, 1, 2, "other check"); number != 1 || err != nil {
		t.Fatalf("independent collector: Add() = %d, %v; want 1, nil", number, err)
	}
	if entries[0].Location == entries[1].Location {
		t.Error("separate occurrences share a mutable location")
	}
}

func TestCollectorEmptyAndUnpositionedResults(t *testing.T) {
	for _, withOccurrence := range []bool{false, true} {
		name := "empty"
		if withOccurrence {
			name = "document without lines"
		}
		t.Run(name, func(t *testing.T) {
			c := newCollector(nil, diagnostics.ScopeDocument)
			if withOccurrence {
				if number, err := c.Add(0, 0, 0, 0, "document"); number != 1 || err != nil {
					t.Fatalf("Add() = %d, %v", number, err)
				}
			}
			entries, err := c.finish()
			if err != nil || !c.closed {
				t.Fatalf("finish() error = %v, closed = %v", err, c.closed)
			}
			if !withOccurrence && len(entries) != 0 {
				t.Fatalf("empty collector returned %v", entries)
			}
			if withOccurrence && (len(entries) != 1 || entries[0].Location != nil || entries[0].Message != "document") {
				t.Fatalf("unpositioned occurrence = %v", entries)
			}
		})
	}
}

func TestCollectorKeepsFirstFailureAndDiscardsPartialResults(t *testing.T) {
	for _, tc := range []struct {
		name    string
		related bool
		number  int
		coords  [4]int
	}{
		{"add partial zero", false, 0, [4]int{1, 0, 1, 2}},
		{"add negative", false, 0, [4]int{-1, 1, 1, 2}},
		{"add missing line", false, 0, [4]int{3, 1, 3, 2}},
		{"add beyond Unicode end", false, 0, [4]int{1, 1, 1, 4}},
		{"add mismatched element", false, 0, [4]int{1, 1, 1, 3}},
		{"add missing required location", false, 0, [4]int{}},
		{"related zero occurrence", true, 0, [4]int{2, 1, 2, 2}},
		{"related negative occurrence", true, -1, [4]int{2, 1, 2, 2}},
		{"related missing occurrence", true, 3, [4]int{2, 1, 2, 2}},
		{"related no position", true, 1, [4]int{}},
		{"related partial zero", true, 1, [4]int{1, 0, 1, 2}},
		{"related negative", true, 1, [4]int{1, -1, 1, 2}},
		{"related invalid range", true, 1, [4]int{2, 3, 2, 1}},
		{"related main duplicate", true, 1, [4]int{1, 1, 1, 2}},
		{"related duplicate", true, 1, [4]int{2, 1, 2, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := populatedCollector(t)
			before := collectorSnapshot(t, c)
			p := tc.coords
			var first error
			if tc.related {
				first = c.AddRelated(tc.number, p[0], p[1], p[2], p[3])
			} else {
				var number int
				number, first = c.Add(p[0], p[1], p[2], p[3], "rejected")
				if number != 0 {
					t.Errorf("failed Add returned occurrence %d", number)
				}
			}
			if first == nil || c.failure != first {
				t.Fatalf("first failure not retained: returned %v, stored %v", first, c.failure)
			}
			if number, err := c.Add(1, 1, 1, 2, "ignored failure"); number != 0 || err != first {
				t.Errorf("Add after failure = %d, %v; want 0 and first error", number, err)
			}
			if err := c.AddRelated(2, 2, 2, 2, 3); err != first {
				t.Errorf("AddRelated replaced first error: %v", err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if entries, err := c.finish(); entries != nil || err != first || !c.closed {
					t.Errorf("finish after failure = %v, %v; closed = %v", entries, err, c.closed)
				}
			}
			if collectorSnapshot(t, c) != before {
				t.Error("failure or subsequent calls changed accumulated findings or input lines")
			}
		})
	}
}

func TestCollectorRejectsCallsAfterFinish(t *testing.T) {
	for _, operation := range []string{"Add", "AddRelated", "finish"} {
		t.Run(operation, func(t *testing.T) {
			c := populatedCollector(t)
			before := collectorSnapshot(t, c)
			if _, err := c.finish(); err != nil {
				t.Fatal(err)
			}
			var first error
			switch operation {
			case "Add":
				var number int
				number, first = c.Add(1, 1, 1, 2, "late")
				if number != 0 {
					t.Errorf("closed Add returned %d", number)
				}
			case "AddRelated":
				first = c.AddRelated(2, 2, 2, 2, 3)
			case "finish":
				var entries []model.Diagnostic
				entries, first = c.finish()
				if entries != nil {
					t.Error("second finish published findings")
				}
			}
			if first == nil || c.failure != first {
				t.Fatalf("closed call did not retain failure: %v", first)
			}
			if number, err := c.Add(1, 1, 1, 2, "late"); number != 0 || err != first {
				t.Errorf("late Add = %d, %v", number, err)
			}
			if err := c.AddRelated(2, 2, 2, 2, 3); err != first {
				t.Errorf("late AddRelated replaced failure: %v", err)
			}
			if entries, err := c.finish(); entries != nil || err != first {
				t.Errorf("late finish = %v, %v", entries, err)
			}
			if collectorSnapshot(t, c) != before {
				t.Error("closed calls changed accumulated findings or input lines")
			}
		})
	}
}

func TestCollectorFinishReturnsDeepCopy(t *testing.T) {
	c := populatedCollector(t)
	before := collectorSnapshot(t, c)
	entries, err := c.finish()
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Location.Start.Column = 99
	entries[0].RelatedLocations[0].End.Column = 99
	entries[0].RelatedLocations = append(entries[0].RelatedLocations, model.Location{})
	entries[1] = model.Diagnostic{Message: "replaced"}
	if collectorSnapshot(t, c) != before {
		t.Error("mutating returned findings changed collector or input lines")
	}
}

func collectorLines(t *testing.T) []model.Line {
	t.Helper()
	text := "Я"
	return newLinesView([]model.Line{
		{Number: 1, Raw: "Я🌍", Elements: []model.Element{{Raw: text, Value: &text, Start: 1, End: 2}}},
		{Number: 2, Raw: "xy"},
	}).lines
}

func populatedCollector(t *testing.T) *collector {
	t.Helper()
	c := newCollector(collectorLines(t), diagnostics.ScopeElement)
	before := rangeSnapshot(t, c.lines, model.Diagnostic{})
	for i := 1; i <= 2; i++ {
		if number, err := c.Add(1, 1, 1, 2, "message"); number != i || err != nil {
			t.Fatalf("Add() = %d, %v; want %d, nil", number, err, i)
		}
	}
	if err := c.AddRelated(1, 2, 1, 2, 2); err != nil {
		t.Fatal(err)
	}
	if rangeSnapshot(t, c.lines, model.Diagnostic{}) != before {
		t.Fatal("successful accumulation changed input lines")
	}
	return c
}

func collectorSnapshot(t *testing.T, c *collector) string {
	t.Helper()
	encoded, err := json.Marshal(struct {
		Lines       []model.Line
		Occurrences []model.Diagnostic
	}{c.lines, c.occurrences})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

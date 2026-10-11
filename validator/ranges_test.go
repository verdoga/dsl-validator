package validator

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestMakeLocation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		coords [4]int
	}{
		{"absent", [4]int{}},
		{"position", [4]int{1, 2, 1, 2}},
		{"multiline", [4]int{1, 5, 3, 1}},
		{"order checked separately", [4]int{3, 2, 1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.coords
			got, err := makeLocation(c[0], c[1], c[2], c[3])
			if err != nil {
				t.Fatal(err)
			}
			if c == [4]int{} {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			want := model.Location{Start: model.Position{Line: c[0], Column: c[1]}, End: model.Position{Line: c[2], Column: c[3]}}
			if got == nil || *got != want {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
	for mask := 1; mask < 15; mask++ {
		t.Run(fmt.Sprintf("partial zero %04b", mask), func(t *testing.T) {
			var c [4]int
			for i := range c {
				c[i] = (mask >> i) & 1
			}
			if got, err := makeLocation(c[0], c[1], c[2], c[3]); got != nil || err == nil {
				t.Fatalf("coordinates %v: got %v, %v; want nil and error", c, got, err)
			}
		})
	}
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprintf("negative coordinate %d", i), func(t *testing.T) {
			c := [4]int{1, 1, 1, 1}
			c[i] = -1
			if got, err := makeLocation(c[0], c[1], c[2], c[3]); got != nil || err == nil {
				t.Fatalf("got %v, %v; want nil and error", got, err)
			}
		})
	}
}

func TestCheckLocationUnicodeBoundsAndOrder(t *testing.T) {
	lines := []model.Line{{Number: 1, Raw: "А🌍e\u0301"}, {Number: 2, Raw: ""}, {Number: 3, Raw: "xyz"}}
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name   string
		coords [4]int
		valid  bool
	}{
		{"whole Unicode line", [4]int{1, 1, 1, 5}, true},
		{"astral character", [4]int{1, 2, 1, 3}, true},
		{"combining character", [4]int{1, 4, 1, 5}, true},
		{"empty position", [4]int{1, 2, 1, 2}, true},
		{"end position", [4]int{1, 5, 1, 5}, true},
		{"empty line", [4]int{2, 1, 2, 1}, true},
		{"multiline decreasing columns", [4]int{1, 5, 3, 1}, true},
		{"last line end", [4]int{3, 1, 3, 4}, true},
		{"all zero", [4]int{}, false},
		{"zero start line", [4]int{0, 1, 1, 1}, false},
		{"negative start line", [4]int{-1, 1, 1, 1}, false},
		{"missing start line", [4]int{4, 1, 4, 1}, false},
		{"zero end line", [4]int{1, 1, 0, 1}, false},
		{"negative end line", [4]int{1, 1, -1, 1}, false},
		{"missing end line", [4]int{1, 1, 4, 1}, false},
		{"zero start column", [4]int{1, 0, 1, 1}, false},
		{"negative start column", [4]int{1, -1, 1, 1}, false},
		{"start beyond Unicode end", [4]int{1, 6, 3, 1}, false},
		{"zero end column", [4]int{1, 1, 1, 0}, false},
		{"negative end column", [4]int{1, 1, 1, -1}, false},
		{"end beyond Unicode end", [4]int{1, 1, 1, 6}, false},
		{"empty line overflow", [4]int{2, 1, 2, 2}, false},
		{"reversed columns", [4]int{1, 3, 1, 2}, false},
		{"reversed lines", [4]int{3, 1, 1, 5}, false},
		{"huge column", [4]int{1, 1, 1, maxInt}, false},
		{"huge line", [4]int{1, 1, maxInt, 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.coords
			location := model.Location{Start: model.Position{Line: c[0], Column: c[1]}, End: model.Position{Line: c[2], Column: c[3]}}
			occurrence := model.Diagnostic{Location: &location}
			before := rangeSnapshot(t, lines, occurrence)
			if err := checkLocation(lines, location); (err == nil) != tc.valid {
				t.Errorf("checkLocation(%v) = %v; valid = %v", location, err, tc.valid)
			}
			if rangeSnapshot(t, lines, occurrence) != before {
				t.Error("checkLocation changed its input")
			}
		})
	}
	for _, empty := range [][]model.Line{nil, {}} {
		if err := checkLocation(empty, model.Location{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 1}}); err == nil {
			t.Error("a position without lines must fail")
		}
	}
}

func TestCheckOccurrenceScopesAndRelatedLocations(t *testing.T) {
	text := "А🌍"
	lines := []model.Line{
		{Number: 1, Raw: "А🌍e\u0301", HasErrors: true, Elements: []model.Element{
			{Start: 1, End: 3, Raw: text, Value: &text, ErrorIDs: []string{"old"}},
			{Start: 3, End: 5, Raw: "e\u0301"},
			{Start: 5, End: 5}, {Start: 5, End: 5},
		}},
		{Number: 2, Raw: ""},
		{Number: 3, Raw: "xyz"},
	}
	first := model.Location{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 3}}
	second := model.Location{Start: model.Position{Line: 1, Column: 3}, End: model.Position{Line: 1, Column: 5}}
	partial := model.Location{Start: model.Position{Line: 1, Column: 2}, End: model.Position{Line: 1, Column: 3}}
	span := model.Location{Start: first.Start, End: second.End}
	point := model.Location{Start: second.End, End: second.End}
	multiline := model.Location{Start: first.Start, End: model.Position{Line: 3, Column: 1}}
	otherLine := model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 3}}
	invalid := model.Location{Start: first.Start, End: model.Position{Line: 1, Column: 6}}
	for _, tc := range []struct {
		name     string
		scope    diagnostics.Scope
		location *model.Location
		related  []model.Location
		valid    bool
	}{
		{"document without position", diagnostics.ScopeDocument, nil, nil, true},
		{"document with position", diagnostics.ScopeDocument, &first, nil, true},
		{"document multiline", diagnostics.ScopeDocument, &multiline, nil, true},
		{"document invalid position", diagnostics.ScopeDocument, &invalid, nil, false},
		{"element without position", diagnostics.ScopeElement, nil, nil, false},
		{"line without position", diagnostics.ScopeLine, nil, nil, false},
		{"block without position", diagnostics.ScopeBlock, nil, nil, false},
		{"unknown scope", "unknown", &first, nil, false},
		{"empty scope", "", nil, nil, false},
		{"element exact", diagnostics.ScopeElement, &first, nil, true},
		{"element second", diagnostics.ScopeElement, &second, nil, true},
		{"element empty repeated range", diagnostics.ScopeElement, &point, nil, true},
		{"element partial", diagnostics.ScopeElement, &partial, nil, false},
		{"element spanning two", diagnostics.ScopeElement, &span, nil, false},
		{"element wrong line", diagnostics.ScopeElement, &otherLine, nil, false},
		{"element multiline", diagnostics.ScopeElement, &multiline, nil, false},
		{"line partial", diagnostics.ScopeLine, &partial, nil, true},
		{"line empty position", diagnostics.ScopeLine, &point, nil, true},
		{"line multiline", diagnostics.ScopeLine, &multiline, nil, false},
		{"block one line", diagnostics.ScopeBlock, &partial, nil, true},
		{"block empty position", diagnostics.ScopeBlock, &point, nil, true},
		{"block multiline", diagnostics.ScopeBlock, &multiline, nil, true},
		{"related without main", diagnostics.ScopeDocument, nil, []model.Location{second, otherLine}, true},
		{"related outside element scope", diagnostics.ScopeElement, &first, []model.Location{partial, multiline, otherLine, point}, true},
		{"related outside line scope", diagnostics.ScopeLine, &first, []model.Location{multiline}, true},
		{"related main duplicate", diagnostics.ScopeElement, &first, []model.Location{first}, false},
		{"related repeated", diagnostics.ScopeBlock, &first, []model.Location{second, otherLine, second}, false},
		{"related empty position repeated", diagnostics.ScopeDocument, nil, []model.Location{point, point}, false},
		{"related invalid", diagnostics.ScopeDocument, nil, []model.Location{invalid}, false},
		{"related zero", diagnostics.ScopeDocument, nil, []model.Location{{}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			occurrence := model.Diagnostic{Location: tc.location, RelatedLocations: tc.related, Message: "без изменений"}
			before := rangeSnapshot(t, lines, occurrence)
			if err := checkOccurrence(lines, tc.scope, occurrence); (err == nil) != tc.valid {
				t.Errorf("checkOccurrence() = %v; valid = %v", err, tc.valid)
			}
			if rangeSnapshot(t, lines, occurrence) != before {
				t.Error("checkOccurrence changed its input")
			}
		})
	}
}

func TestCheckOccurrenceWithoutLines(t *testing.T) {
	for _, scope := range []diagnostics.Scope{diagnostics.ScopeDocument, diagnostics.ScopeLine, diagnostics.ScopeBlock, diagnostics.ScopeElement} {
		t.Run(string(scope), func(t *testing.T) {
			for _, lines := range [][]model.Line{nil, {}} {
				if err := checkOccurrence(lines, scope, model.Diagnostic{}); (err == nil) != (scope == diagnostics.ScopeDocument) {
					t.Errorf("scope %q without lines or position: %v", scope, err)
				}
				position := model.Location{Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 1}}
				for _, occurrence := range []model.Diagnostic{{Location: &position}, {RelatedLocations: []model.Location{position}}} {
					if err := checkOccurrence(lines, scope, occurrence); err == nil {
						t.Error("a main or related position without lines must fail")
					}
				}
			}
		})
	}
}

func rangeSnapshot(t *testing.T, lines []model.Line, occurrence model.Diagnostic) string {
	t.Helper()
	encoded, err := json.Marshal(struct {
		Lines      []model.Line
		Occurrence model.Diagnostic
	}{lines, occurrence})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

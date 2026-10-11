package validator

import (
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestLinesViewEmpty(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []model.Line
	}{
		{"nil", nil},
		{"empty", []model.Line{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var view diagnostics.Lines = newLinesView(tc.lines)
			if view.Len() != 0 {
				t.Fatalf("Len() = %d, want 0", view.Len())
			}
			for _, number := range []int{-1, 0, 1} {
				if line, found := view.Line(number); line != nil || found {
					t.Errorf("Line(%d) = %v, %v; want nil, false", number, line, found)
				}
			}
		})
	}
}

func TestLinesViewPreservesContentAndTraversal(t *testing.T) {
	parent, text, empty := 1, "Привет 🌍", ""
	lines := []model.Line{
		{Number: 1, LineType: model.LineTypeHeading, Raw: "# Заголовок", LineEnding: model.LineEndingLF},
		{
			Number: 2, LineType: model.LineTypeContent, NestingLevel: 1, ParentLine: &parent,
			Raw: "\tПривет \\🌍", LineEnding: model.LineEndingCRLF,
			Elements: []model.Element{
				{ElementType: model.ElementTypeContent, Raw: "Привет \\🌍", Value: &text, Start: 2, End: 11},
				{ElementType: model.ElementTypeContent, Raw: "", Value: &empty, Start: 11, End: 11},
				{ElementType: model.ElementTypeUnparsed, Raw: "", Value: nil, Start: 11, End: 11},
			},
		},
		{Number: 3, LineType: model.LineTypeBlank, LineEnding: model.LineEndingNone, Elements: []model.Element{}},
	}
	var view diagnostics.Lines = newLinesView(lines)
	if view.Len() != len(lines) {
		t.Fatalf("Len() = %d, want %d", view.Len(), len(lines))
	}
	for _, tc := range []struct {
		name    string
		numbers []int
	}{
		{"forward", []int{1, 2, 3}},
		{"backward", []int{3, 2, 1}},
		{"repeated", []int{2, 1, 2, 3, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, number := range tc.numbers {
				assertLineView(t, view, lines[number-1])
			}
		})
	}
	for _, number := range []int{-1, 0, 4} {
		if line, found := view.Line(number); line != nil || found {
			t.Errorf("Line(%d) = %v, %v; want nil, false", number, line, found)
		}
	}
}

func TestLinesViewCopiesSourceAndClearsErrors(t *testing.T) {
	parent, text := 1, "Я🌍"
	ids := []string{"d1", "d2"}
	elements := []model.Element{{
		ElementType: model.ElementTypeContent, Raw: "Я🌍", Value: &text,
		Start: 2, End: 4, ErrorIDs: ids,
	}}
	lines := []model.Line{
		{Number: 1, LineType: model.LineTypeBlank, LineEnding: model.LineEndingLF},
		{
			Number: 2, LineType: model.LineTypeContent, NestingLevel: 1, ParentLine: &parent,
			Raw: "\tЯ🌍", LineEnding: model.LineEndingNone, HasErrors: true, Elements: elements,
		},
	}
	view := newLinesView(lines)
	if !lines[1].HasErrors || len(elements[0].ErrorIDs) != 2 || ids[0] != "d1" || ids[1] != "d2" {
		t.Fatal("constructing the view changed source error information")
	}
	for i, line := range view.lines {
		if line.HasErrors {
			t.Errorf("projection line %d retains HasErrors", i+1)
		}
		for j, element := range line.Elements {
			if element.ErrorIDs != nil {
				t.Errorf("projection line %d element %d retains ErrorIDs", i+1, j)
			}
		}
	}
	if view.lines[1].ParentLine == &parent || view.lines[1].Elements[0].Value == &text {
		t.Fatal("projection retains a source pointer")
	}
	retainedLine, found := view.Line(2)
	if !found {
		t.Fatal("Line(2) not found")
	}
	retainedElement, found := retainedLine.Element(0)
	if !found {
		t.Fatal("Element(0) not found")
	}
	parent, text, ids[0] = 99, "изменено", "changed"
	elements[0] = model.Element{ElementType: model.ElementTypeUnparsed, Raw: "changed"}
	lines[1] = model.Line{Number: 99, Raw: "changed"}

	wantParent, wantText := 1, "Я🌍"
	want := model.Line{
		Number: 2, LineType: model.LineTypeContent, NestingLevel: 1, ParentLine: &wantParent,
		Raw: "\tЯ🌍", LineEnding: model.LineEndingNone,
		Elements: []model.Element{{ElementType: model.ElementTypeContent, Raw: "Я🌍", Value: &wantText, Start: 2, End: 4}},
	}
	assertLineView(t, view, want)
	if got, found := retainedLine.ParentLine(); got != 1 || !found {
		t.Errorf("retained ParentLine() = %d, %v; want 1, true", got, found)
	}
	if retainedLine.Raw() != want.Raw || retainedElement.Raw() != "Я🌍" {
		t.Error("retained wrappers changed with source")
	}
	if got, present := retainedElement.Value(); got != wantText || !present {
		t.Errorf("retained Value() = %q, %v; want %q, true", got, present, wantText)
	}
	if view.lines[1].HasErrors || view.lines[1].Elements[0].ErrorIDs != nil {
		t.Error("source mutation restored error information in projection")
	}
}

func assertLineView(t *testing.T, view diagnostics.Lines, want model.Line) {
	t.Helper()
	line, found := view.Line(want.Number)
	if !found || line == nil {
		t.Fatalf("Line(%d) = %v, %v; want a line", want.Number, line, found)
	}
	if line.Number() != want.Number || line.Type() != string(want.LineType) || line.NestingLevel() != want.NestingLevel {
		t.Errorf("line %d: number/type/depth = %d/%q/%d", want.Number, line.Number(), line.Type(), line.NestingLevel())
	}
	parent, hasParent := line.ParentLine()
	if want.ParentLine == nil {
		if parent != 0 || hasParent {
			t.Errorf("line %d: ParentLine() = %d, %v; want 0, false", want.Number, parent, hasParent)
		}
	} else if parent != *want.ParentLine || !hasParent {
		t.Errorf("line %d: ParentLine() = %d, %v; want %d, true", want.Number, parent, hasParent, *want.ParentLine)
	}
	if line.Raw() != want.Raw || line.EOL() != string(want.LineEnding) {
		t.Errorf("line %d: Raw/EOL = %q/%q; want %q/%q", want.Number, line.Raw(), line.EOL(), want.Raw, want.LineEnding)
	}
	if line.ElementCount() != len(want.Elements) {
		t.Fatalf("line %d: ElementCount() = %d, want %d", want.Number, line.ElementCount(), len(want.Elements))
	}
	for _, index := range []int{-1, len(want.Elements), len(want.Elements) + 1} {
		if element, found := line.Element(index); element != nil || found {
			t.Errorf("line %d: Element(%d) = %v, %v; want nil, false", want.Number, index, element, found)
		}
	}
	for i, expected := range want.Elements {
		element, found := line.Element(i)
		if !found || element == nil {
			t.Fatalf("line %d: Element(%d) = %v, %v; want an element", want.Number, i, element, found)
		}
		if element.Type() != string(expected.ElementType) || element.Raw() != expected.Raw {
			t.Errorf("line %d element %d: Type/Raw = %q/%q; want %q/%q", want.Number, i, element.Type(), element.Raw(), expected.ElementType, expected.Raw)
		}
		if element.Start() != expected.Start || element.End() != expected.End {
			t.Errorf("line %d element %d: range = [%d,%d), want [%d,%d)", want.Number, i, element.Start(), element.End(), expected.Start, expected.End)
		}
		text, present := element.Value()
		if expected.Value == nil {
			if text != "" || present {
				t.Errorf("line %d element %d: Value() = %q, %v; want empty, false", want.Number, i, text, present)
			}
		} else if text != *expected.Value || !present {
			t.Errorf("line %d element %d: Value() = %q, %v; want %q, true", want.Number, i, text, present, *expected.Value)
		}
	}
}

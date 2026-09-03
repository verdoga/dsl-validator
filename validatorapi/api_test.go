package validatorapi

import "testing"

func TestSpanValid(t *testing.T) {
	cases := []struct {
		name string
		span Span
		want bool
	}{{"single", Span{Position{1, 1}, Position{1, 2}}, true}, {"empty", Span{Position{2, 3}, Position{2, 3}}, true}, {"zero", Span{}, false}, {"reverse line", Span{Position{2, 1}, Position{1, 2}}, false}, {"reverse column", Span{Position{1, 3}, Position{1, 2}}, false}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.span.Valid(); got != tc.want {
				t.Fatalf("Valid()=%v, want %v", got, tc.want)
			}
		})
	}
}
func TestSpanByRuneOffsets(t *testing.T) {
	span, err := SpanByRuneOffsets("я🙂e\u0301", Position{4, 7}, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := Span{Position{4, 8}, Position{4, 10}}
	if span != want {
		t.Fatalf("got %#v, want %#v", span, want)
	}
	for _, args := range []struct {
		text       string
		start, end int
	}{{"a\nb", 0, 1}, {"abc", -1, 1}, {"abc", 2, 1}, {"abc", 0, 4}} {
		if _, err := SpanByRuneOffsets(args.text, Position{1, 1}, args.start, args.end); err == nil {
			t.Errorf("expected error for %#v", args)
		}
	}
}

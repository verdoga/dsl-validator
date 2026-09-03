package parseradapter

import (
	"dslparser/validatorapi"
	"errors"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func TestParseReaderClassifiesRealResults(t *testing.T) {
	cases := []struct {
		name, input string
		code        validatorapi.ParserCode
	}{{"invalid utf8", string([]byte{0xff}), validatorapi.ParserInvalidUTF8}, {"missing version", "text", validatorapi.ParserMissingVersion}, {"unsupported", "@dsl-version 9.9", validatorapi.ParserUnsupportedVersion}}
	a := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.ParseReader(strings.NewReader(tc.input))
			if got.Fatal == nil || got.Fatal.Code != tc.code || got.Document != nil || got.Failure != nil {
				t.Fatalf("unexpected result: %#v", got)
			}
		})
	}
	got := a.ParseReader(failingReader{})
	if got.Fatal == nil || got.Fatal.Code != validatorapi.ParserReadFailure {
		t.Fatalf("read result: %#v", got)
	}
}
func TestParseReaderBuildsReadOnlyContext(t *testing.T) {
	got := New().ParseReader(strings.NewReader("@dsl-version 1.1\n@HeAdEr Title\nText"))
	if got.Document == nil || got.Context == nil || got.Fatal != nil || got.Failure != nil {
		t.Fatalf("unexpected result %#v", got)
	}
	if got.Document.Version() != "1.1" {
		t.Fatal(got.Document.Version())
	}
	physical := got.Context.PhysicalNodes()
	if len(physical) < 2 {
		t.Fatalf("physical=%d", len(physical))
	}
}

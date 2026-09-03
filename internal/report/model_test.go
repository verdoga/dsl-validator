package report

import (
	"dslparser/validatorapi"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFinishCountsSortsAndNumbers(t *testing.T) {
	r := New(filepath.Join(t.TempDir(), "x.txt"), "auto", nil, time.Unix(1, 0))
	r.Entries = []Entry{{Code: "V", Message: "v", Level: validatorapi.SeverityWarning, Source: validatorapi.SourceValidator}, {Code: "P", Message: "p", Level: validatorapi.SeverityError, Source: validatorapi.SourceParser, Location: &Location{Start: validatorapi.Position{Line: 1, Column: 2}, End: validatorapi.Position{Line: 1, Column: 3}, EndExclusive: true}}}
	r.Finish(time.Unix(1, 2e6))
	if r.Entries[0].Code != "P" || r.Entries[0].Number != 1 || r.Summary.Total != 2 || !r.Summary.HasErrors || r.Analysis.Status != "completed_with_errors" {
		t.Fatalf("report=%#v", r)
	}
}
func TestWriteAtomicPreservesConflictingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lesson.txt")
	r := New(path, "auto", nil, time.Now())
	r.Finish(time.Now())
	if err := WriteAtomic(r); err != nil {
		t.Fatal(err)
	}
	target := CanonicalPath(path)
	if _, err := DecodeStrict(mustRead(t, target)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("foreign"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(r); err == nil {
		t.Fatal("conflict accepted")
	}
	if string(mustRead(t, target)) != "foreign" {
		t.Fatal("conflict overwritten")
	}
}
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

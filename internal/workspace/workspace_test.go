package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsCaseInsensitiveTxtRecursively(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.TXT", "a.txt", "skip.md", "nested/c.TxT"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := NewScanner(nil).Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 3 {
		t.Fatalf("files=%d", len(got.Files))
	}
	if got.Files[0].Name != "a.txt" || got.Files[2].Name != "c.TxT" {
		t.Fatalf("order: %#v", got.Files)
	}
}
func TestValidateRootRejectsRelativeAndSymlink(t *testing.T) {
	if _, err := ValidateRoot("relative"); err == nil {
		t.Fatal("relative accepted")
	}
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err == nil {
		if _, err := ValidateRoot(link); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}

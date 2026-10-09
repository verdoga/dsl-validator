package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestNormalizePathReturnsAbsoluteCleanPath(t *testing.T) {
	root := pathTestTree(t)
	t.Chdir(root)
	tests := []struct {
		name, raw, want string
	}{
		{"empty", "", root},
		{"dot", ".", root},
		{"parent", "..", filepath.Dir(root)},
		{"relative_file", "sample.json", filepath.Join(root, "sample.json")},
		{"absolute_file", filepath.Join(root, "sample.json"), filepath.Join(root, "sample.json")},
		{"unclean", "sub//nested/../../sample.json", filepath.Join(root, "sample.json")},
		{"trailing_separator", "sub//", filepath.Join(root, "sub")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := PathInfo(tt.raw); err != nil {
				t.Fatalf("invalid original path: %v", err)
			}
			got, err := normalizePath(tt.raw)
			if err != nil || got != tt.want || !filepath.IsAbs(got) {
				t.Fatalf("normalizePath(%q) = %q, %v; want %q", tt.raw, got, err, tt.want)
			}
		})
	}
}

func TestPathInfoReturnsFinalComponent(t *testing.T) {
	root := pathTestTree(t)
	t.Chdir(root)
	tests := []struct {
		name, path, target string
	}{
		{"empty", "", root},
		{"dot", ".", root},
		{"parent", "..", filepath.Dir(root)},
		{"root", string(os.PathSeparator), string(os.PathSeparator)},
		{"absolute", filepath.Join(root, "sample.json"), filepath.Join(root, "sample.json")},
		{"relative", "sample.json", filepath.Join(root, "sample.json")},
		{"other_extension", "notes.txt", filepath.Join(root, "notes.txt")},
		{"components_before_parent", "sub//nested/../../sample.json", filepath.Join(root, "sample.json")},
		{"directory_with_separator", "sub//", filepath.Join(root, "sub")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.Lstat(tt.target)
			if err != nil {
				t.Fatal(err)
			}
			got, err := PathInfo(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || !os.SameFile(got, want) || got.Mode() != want.Mode() {
				t.Fatalf("PathInfo(%q) did not return info for %q", tt.path, tt.target)
			}
		})
	}
}

func TestPathChecksRejectInvalidComponentsBeforeCleaning(t *testing.T) {
	root := pathTestTree(t)
	t.Chdir(root)
	for _, link := range []struct{ name, target string }{
		{"file-link.json", "sample.json"},
		{"dir-link", "sub"},
		{"dangling.json", "missing"},
	} {
		if err := os.Symlink(filepath.Join(root, link.target), filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
		if info, err := sourceComponentInfo(link.name); err == nil || info != nil {
			t.Fatalf("sourceComponentInfo(%q) = %v, %v; want rejection", link.name, info, err)
		}
	}
	tests := []struct {
		path  string
		cause error
	}{
		{"missing", os.ErrNotExist},
		{"missing/../sample.json", os.ErrNotExist},
		{"sample.json/child", syscall.ENOTDIR},
		{"sample.json/../sample.json", syscall.ENOTDIR},
		{"sample.json/.", syscall.ENOTDIR},
		{"sample.json/", syscall.ENOTDIR},
		{"file-link.json", nil},
		{"dir-link", nil},
		{"dir-link/nested", nil},
		{"dir-link/../sample.json", nil},
		{"dir-link/.", nil},
		{"dir-link/", nil},
		{"dangling.json", nil},
		{"dangling.json/../sample.json", nil},
		{root + "/dir-link/../sample.json", nil},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			info, pathErr := PathInfo(tt.path)
			if info != nil {
				t.Fatalf("PathInfo returned info on failure: %v", info)
			}
			for _, err := range []error{pathErr, validateSourcePath(tt.path)} {
				if err == nil {
					t.Fatal("invalid source path was accepted")
				}
				if tt.cause != nil {
					var original *os.PathError
					if !errors.Is(err, tt.cause) || !errors.As(err, &original) {
						t.Errorf("error %v does not preserve filesystem cause %v", err, tt.cause)
					}
				} else if !strings.Contains(err.Error(), "символическая ссылка") {
					t.Errorf("expected rejection of a symbolic link, got %v", err)
				}
			}
		})
	}
}

func TestValidateSourcePathChecksTypeAndExtension(t *testing.T) {
	root := pathTestTree(t)
	t.Chdir(root)
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.json"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := PathInfo("pipe.json")
	if err != nil || info == nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("PathInfo(pipe.json) = %v, %v; want named pipe info", info, err)
	}
	for _, tt := range []struct {
		path string
		ok   bool
	}{
		{"", true}, {".", true}, {"sub", true}, {root, true},
		{"sample.json", true}, {"upper.JSON", true}, {"mixed.JsOn", true},
		{filepath.Join(root, "sample.json"), true},
		{"notes.txt", false}, {"notes.jsonc", false}, {"noextension", false},
		{"pipe.json", false},
	} {
		t.Run(tt.path, func(t *testing.T) {
			if err := validateSourcePath(tt.path); (err == nil) != tt.ok {
				t.Errorf("validateSourcePath(%q) = %v; want accepted=%v", tt.path, err, tt.ok)
			}
		})
	}
	contents, err := os.ReadFile("sample.json")
	if err != nil || string(contents) != "not JSON" {
		t.Fatalf("invalid JSON contents changed: %q, %v", contents, err)
	}
}

func TestValidateSourcePathChecksWorkingDirectoryParents(t *testing.T) {
	root := pathTestTree(t)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(root, "sub"), alias); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "sub", "nested"))
	t.Setenv("PWD", filepath.Join(alias, "nested"))
	for _, path := range []string{"", ".", "..", "../.."} {
		t.Run(path, func(t *testing.T) {
			if _, err := PathInfo(path); err != nil {
				t.Fatalf("original relative path is accessible: %v", err)
			}
			if err := validateSourcePath(path); err == nil || !strings.Contains(err.Error(), "символическая ссылка") {
				t.Fatalf("validateSourcePath(%q) = %v; want rejection of the linked parent", path, err)
			}
		})
	}
	if err := validateSourcePath(filepath.Join(root, "sample.json")); err != nil {
		t.Fatalf("absolute source does not depend on the working directory: %v", err)
	}
}

func TestPathNormalizationPreservesWorkingDirectoryError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	path, err := normalizePath("")
	if path != "" || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("normalizePath from removed directory = %q, %v", path, err)
	}
	if err := validateSourcePath(""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("validateSourcePath from removed directory = %v", err)
	}
}

func pathTestTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sample.json", "upper.JSON", "mixed.JsOn", "notes.txt", "notes.jsonc", "noextension"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("not JSON"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

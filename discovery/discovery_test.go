package discovery

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/verdoga/dsl-validator/console"
)

func TestIsJSONMatchesOnlyFinalExtension(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"sample.json", true}, {"sample.JSON", true}, {"sample.JsOn", true},
		{".json", true}, {"sub/sample.json", true}, {"", false},
		{"sample", false}, {"sample.jsonc", false}, {"sample.json.bak", false},
		{"sample.json ", false}, {"sample.json/child", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := isJSON(tc.path); got != tc.want {
				t.Errorf("isJSON(%q) = %t, want %t", tc.path, got, tc.want)
			}
		})
	}
}

func TestFindSingleFileIgnoresDepthAndContents(t *testing.T) {
	root := t.TempDir()
	path := discoveryFile(t, root, "invalid.JsOn")
	for _, depth := range []string{"0", "3"} {
		t.Run(depth, func(t *testing.T) {
			params := discoveryParams(t, "--depth", depth, path)
			got, err := Find(params)
			if err != nil || !slices.Equal(got, []string{path}) {
				t.Fatalf("Find = %v, %v; want [%s]", got, err, path)
			}
			contents, err := os.ReadFile(path)
			if err != nil || string(contents) != "not JSON" {
				t.Fatalf("contents changed: %q, %v", contents, err)
			}
		})
	}
}

func TestFindAndWalkDirectoryRespectDepth(t *testing.T) {
	root := t.TempDir()
	a := discoveryFile(t, root, "a.json")
	z := discoveryFile(t, root, "z.JSON")
	b := discoveryFile(t, root, "sub/b.JsOn")
	c := discoveryFile(t, root, "sub/deep/c.json")
	d := discoveryFile(t, root, "sub/deep/further/d.json")
	for _, name := range []string{"notes.jsonc", "notes.txt", "notes.json.bak", "noextension"} {
		discoveryFile(t, root, name)
	}
	if err := os.Mkdir(filepath.Join(root, "empty.json"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"root only", []string{"--depth", "0", root}, []string{a, z}},
		{"one level", []string{"--depth", "1", root}, []string{a, b, z}},
		{"two levels", []string{"--depth", "2", root}, []string{a, c, b, z}},
		{"unlimited", []string{root}, []string{a, b, c, d, z}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := discoveryParams(t, tc.args...)
			slices.Sort(tc.want)
			got, err := Find(params)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("Find = %v, %v; want %v", got, err, tc.want)
			}
			for _, path := range got {
				if !filepath.IsAbs(path) {
					t.Errorf("relative result: %q", path)
				}
			}
			got, err = walkDirectory(params.Path, params.Depth)
			slices.Sort(got)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("walkDirectory = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	t.Run("empty directory", func(t *testing.T) {
		got, err := Find(discoveryParams(t, filepath.Join(root, "empty.json")))
		if err != nil || len(got) != 0 {
			t.Fatalf("Find = %v, %v; want empty success", got, err)
		}
	})
}

func TestFindSkipsSymlinksAndSpecialFilesButKeepsHardLinkPaths(t *testing.T) {
	root := t.TempDir()
	file := discoveryFile(t, root, "a.json")
	nested := discoveryFile(t, root, "sub/b.json")
	hardLink := filepath.Join(root, "hard.json")
	if err := os.Link(file, hardLink); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"file-link.json": file, "dir-link": filepath.Join(root, "sub"),
		"broken.json": filepath.Join(root, "missing"), "cycle": root,
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.json"), 0600); err != nil {
		t.Fatal(err)
	}
	want := []string{file, hardLink, nested}
	slices.Sort(want)
	got, err := Find(discoveryParams(t, root))
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("Find = %v, %v; want %v", got, err, want)
	}
}

func TestFindRechecksSourceAfterParsing(t *testing.T) {
	for _, scenario := range []string{"removed", "file symlink", "parent symlink", "special file"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			path := discoveryFile(t, root, "sub/source.json")
			target := discoveryFile(t, root, "target.json")
			params := discoveryParams(t, path)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			var setupErr error
			switch scenario {
			case "file symlink":
				setupErr = os.Symlink(target, path)
			case "parent symlink":
				if err := os.Remove(filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
				setupErr = os.Symlink(root, filepath.Dir(path))
			case "special file":
				setupErr = syscall.Mkfifo(path, 0600)
			}
			if setupErr != nil {
				t.Fatal(setupErr)
			}
			got, err := Find(params)
			if err == nil || len(got) != 0 {
				t.Fatalf("Find = %v, %v; want rejection", got, err)
			}
			if scenario == "removed" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing original cause: %v", err)
			}
		})
	}
}

func TestFindStopsAtFirstWalkError(t *testing.T) {
	// Под root запрет чтения не действует: запускаем этот тест без привилегий.
	if os.Geteuid() == 0 {
		dir, err := os.MkdirTemp("/tmp", "discovery-unprivileged-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		if err := os.Chmod(dir, 0755); err != nil {
			t.Fatal(err)
		}
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := os.ReadFile(executable)
		if err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(dir, "discovery.test")
		if err := os.WriteFile(binary, contents, 0755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "-test.run=^TestFindStopsAtFirstWalkError$", "-test.v")
		cmd.Dir, cmd.Env = "/tmp", []string{"TMPDIR=/tmp"}
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("unprivileged test: %v\n%s", err, output)
		}
		return
	}
	root := t.TempDir()
	inside := discoveryFile(t, root, "a/first.json")
	before := discoveryFile(t, root, "a.json")
	discoveryFile(t, root, "b-denied/hidden.json")
	discoveryFile(t, root, "z-after.json")
	denied := filepath.Join(root, "b-denied")
	params := discoveryParams(t, root)
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(denied, 0700); err != nil {
			t.Error(err)
		}
	})
	got, err := Find(params)
	want := []string{before, inside}
	var pathErr *os.PathError
	if !errors.Is(err, os.ErrPermission) || !errors.As(err, &pathErr) || pathErr.Path != denied {
		t.Fatalf("Find error = %v; want permission error for %s", err, denied)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("partial result = %v; want sorted %v", got, want)
	}
	t.Run("inaccessible directory beyond depth is not read", func(t *testing.T) {
		depth := 0
		params.Depth = &depth
		got, err := Find(params)
		want := []string{before, filepath.Join(root, "z-after.json")}
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("Find = %v, %v; want %v", got, err, want)
		}
	})
}

func discoveryFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func discoveryParams(t *testing.T, args ...string) console.Params {
	t.Helper()
	params, proceed, err := console.Parse(args, io.Discard, io.Discard)
	if err != nil || !proceed {
		t.Fatalf("Parse(%v): proceed=%t, error=%v", args, proceed, err)
	}
	return params
}

package storage

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCheckUnchangedChecksIdentityBytesAndPath(t *testing.T) {
	for _, name := range []string{"unchanged", "timestamp_only", "changed_bytes", "replacement", "missing", "file_link", "parent_link", "directory"} {
		t.Run(name, func(t *testing.T) {
			snapshot := writeTestSnapshot(t)
			before := *snapshot
			before.original = bytes.Clone(snapshot.original)
			var err error
			switch name {
			case "timestamp_only":
				err = os.Chtimes(snapshot.path, time.Unix(1, 0), time.Unix(1, 0))
			case "changed_bytes":
				err = os.WriteFile(snapshot.path, bytes.Repeat([]byte("x"), len(snapshot.original)), 0600)
			case "replacement", "file_link":
				old := snapshot.path + ".old"
				if err = os.Rename(snapshot.path, old); err == nil {
					if name == "replacement" {
						err = os.WriteFile(snapshot.path, snapshot.original, 0600)
					} else {
						err = os.Symlink(old, snapshot.path)
					}
				}
			case "missing", "directory":
				if err = os.Remove(snapshot.path); err == nil && name == "directory" {
					err = os.Mkdir(snapshot.path, 0700)
				}
			case "parent_link":
				parent := filepath.Dir(snapshot.path)
				if err = os.Rename(parent, parent+".old"); err == nil {
					err = os.Symlink(parent+".old", parent)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			contentsBefore, readErr := os.ReadFile(snapshot.path)
			infoBefore, statErr := os.Lstat(snapshot.path)
			err = checkUnchanged(snapshot)
			wantSuccess := name == "unchanged" || name == "timestamp_only"
			if (err == nil) != wantSuccess {
				t.Fatalf("checkUnchanged = %v; want accepted=%v", err, wantSuccess)
			}
			if name == "missing" && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("missing filesystem cause: %v", err)
			}
			if snapshot.path != before.path || snapshot.info != before.info || !bytes.Equal(snapshot.original, before.original) {
				t.Error("checkUnchanged modified the snapshot")
			}
			if contents, err := os.ReadFile(snapshot.path); readErr == nil && (err != nil || !bytes.Equal(contents, contentsBefore)) {
				t.Errorf("checkUnchanged changed file contents: %v", err)
			}
			infoAfter, err := os.Lstat(snapshot.path)
			if statErr == nil && (err != nil || !os.SameFile(infoBefore, infoAfter) || infoBefore.Mode() != infoAfter.Mode()) {
				t.Errorf("checkUnchanged changed the path: %v", err)
			}
			if statErr != nil && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("checkUnchanged recreated the missing file: %v", err)
			}
			writeTestClosed(t, before.info)
		})
	}
}

func TestWriteTemporaryWritesFullPayloadAndPreservesPermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0600, 0640, 0400} {
		for _, payload := range [][]byte{{}, bytes.Repeat([]byte("JSON <>&\n"), 32768)} {
			snapshot := writeTestSnapshot(t)
			if err := os.Chmod(snapshot.path, mode); err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(payload)
			temporary, err := writeTemporary(snapshot.path, payload, mode)
			if err != nil {
				t.Fatal(err)
			}
			if temporary == snapshot.path || filepath.Dir(temporary) != filepath.Dir(snapshot.path) {
				t.Fatalf("unexpected temporary path %q", temporary)
			}
			written, err := os.ReadFile(temporary)
			if err != nil || !bytes.Equal(written, payload) || !bytes.Equal(payload, before) {
				t.Fatalf("incomplete write or changed payload: %v", err)
			}
			info, err := os.Stat(temporary)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode {
				t.Fatalf("incorrect temporary file permissions: %v, %v", info, err)
			}
			writeTestClosed(t, info)
			original, err := os.ReadFile(snapshot.path)
			if err != nil || !bytes.Equal(original, snapshot.original) {
				t.Fatalf("temporary write changed the original: %v", err)
			}
		}
	}
}

func TestWriteTemporaryRejectsInvalidParent(t *testing.T) {
	snapshot := writeTestSnapshot(t)
	for _, parent := range []string{snapshot.path, filepath.Join(filepath.Dir(snapshot.path), "missing")} {
		temporary, err := writeTemporary(filepath.Join(parent, "new.json"), []byte("{}"), 0600)
		if temporary != "" || err == nil {
			t.Fatalf("writeTemporary = %q, %v; want empty path and error", temporary, err)
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("lost filesystem error: %v", err)
		}
	}
	writeTestOnlyNames(t, filepath.Dir(snapshot.path), filepath.Base(snapshot.path))
}

func TestWriteTemporaryCleansPartialWrite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("this failure scenario uses Linux RLIMIT_FSIZE")
	}
	if target := os.Getenv("DSL_VALIDATOR_TEST_FILE_LIMIT_TARGET"); target != "" {
		// Лимит действует только в дочернем процессе и не меняет условия других тестов.
		signal.Ignore(syscall.SIGXFSZ)
		var limit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		limit.Cur = 1024
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		temporary, err := writeTemporary(target, bytes.Repeat([]byte("x"), 2048), 0600)
		if temporary != "" || !errors.Is(err, syscall.EFBIG) {
			t.Fatalf("partial write = %q, %v; want empty path and EFBIG", temporary, err)
		}
		return
	}
	snapshot := writeTestSnapshot(t)
	child := exec.Command(os.Args[0], "-test.run=^TestWriteTemporaryCleansPartialWrite$")
	child.Env = append(os.Environ(), "DSL_VALIDATOR_TEST_FILE_LIMIT_TARGET="+snapshot.path)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("limited writer failed: %v\n%s", err, output)
	}
	writeTestOnlyNames(t, filepath.Dir(snapshot.path), filepath.Base(snapshot.path))
	if original, err := os.ReadFile(snapshot.path); err != nil || !bytes.Equal(original, snapshot.original) {
		t.Fatalf("failed write changed the original: %v", err)
	}
}

func TestReplaceExistingInstallsPreparedFileAndRemovesBackup(t *testing.T) {
	snapshot := writeTestSnapshot(t)
	payload := []byte("{\"new\":true}\n")
	temporary, err := writeTemporary(snapshot.path, payload, snapshot.info.Mode())
	if err != nil {
		t.Fatal(err)
	}
	preparedInfo, err := os.Stat(temporary)
	if err != nil {
		t.Fatal(err)
	}
	if updated, err := replaceExisting(temporary, snapshot.path); !updated || err != nil {
		t.Fatalf("replaceExisting = %v, %v; want true, nil", updated, err)
	}
	installed, err := os.ReadFile(snapshot.path)
	if err != nil || !bytes.Equal(installed, payload) {
		t.Fatalf("new version was not installed: %v", err)
	}
	info, err := os.Stat(snapshot.path)
	if err != nil || !os.SameFile(preparedInfo, info) {
		t.Fatalf("prepared file was not moved into place: %v", err)
	}
	writeTestOnlyNames(t, filepath.Dir(snapshot.path), filepath.Base(snapshot.path))
	writeTestClosed(t, snapshot.info)
	writeTestClosed(t, info)
}

func TestReplaceExistingRestoresOriginalAfterInstallationFailure(t *testing.T) {
	snapshot := writeTestSnapshot(t)
	missing := filepath.Join(filepath.Dir(snapshot.path), "missing-temporary")
	updated, err := replaceExisting(missing, snapshot.path)
	if updated || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replaceExisting = %v, %v; want false and missing-file error", updated, err)
	}
	if err := checkUnchanged(snapshot); err != nil {
		t.Fatalf("original file was not restored: %v", err)
	}
	writeTestOnlyNames(t, filepath.Dir(snapshot.path), filepath.Base(snapshot.path))
}

func TestReplaceExistingKeepsPreparedFileWhenOriginalIsMissing(t *testing.T) {
	snapshot := writeTestSnapshot(t)
	temporary, err := writeTemporary(snapshot.path, []byte("new"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(snapshot.path); err != nil {
		t.Fatal(err)
	}
	if updated, err := replaceExisting(temporary, snapshot.path); updated || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replaceExisting = %v, %v; want false and missing-file error", updated, err)
	}
	if prepared, err := os.ReadFile(temporary); err != nil || string(prepared) != "new" {
		t.Fatalf("the only remaining copy was lost: %v", err)
	}
	writeTestOnlyNames(t, filepath.Dir(snapshot.path), filepath.Base(temporary))
}

func writeTestSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "input")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "original.json")
	source := []byte("{\n  \"old\": true\n}\n")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return &Snapshot{path: path, original: source, info: info}
}

func writeTestOnlyNames(t *testing.T, directory string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, entry := range entries {
		actual = append(actual, entry.Name())
	}
	if strings.Join(actual, "\n") != strings.Join(names, "\n") {
		t.Fatalf("unexpected directory contents: %v; want %v", actual, names)
	}
}

func writeTestClosed(t *testing.T, info os.FileInfo) {
	t.Helper()
	if runtime.GOOS != "linux" {
		return
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		opened, err := os.Stat(filepath.Join("/proc/self/fd", entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || os.SameFile(info, opened) {
			t.Fatalf("descriptor %s remains open or cannot be checked: %v", entry.Name(), err)
		}
	}
}

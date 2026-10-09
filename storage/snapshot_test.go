package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

func TestSnapshotKeepsAbsoluteJSONPath(t *testing.T) {
	path, source := snapshotTestFile(t)
	t.Chdir(filepath.Dir(path))
	result, snapshot, err := Open(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || snapshot == nil {
		t.Fatal("Open returned no model or snapshot")
	}
	if snapshot.path != path || !filepath.IsAbs(snapshot.path) {
		t.Fatalf("snapshot path = %q; want %q", snapshot.path, path)
	}
	if result.Document.FilePath == nil || *result.Document.FilePath == path {
		t.Fatal("fixture must refer to a separate DSL file")
	}
	dslPath := *result.Document.FilePath
	dslBefore, err := os.ReadFile(dslPath)
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	decoy := filepath.Join(other, filepath.Base(path))
	if err := os.WriteFile(decoy, source, 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(other)
	result.Processing = append(result.Processing, model.Processing{ID: "validation", Tool: "dsl-validator"})
	if updated, err := Save(snapshot, result); !updated || err != nil {
		t.Fatalf("Save = %v, %v; want true, nil", updated, err)
	}
	saved, _, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || len(saved.Processing) != 2 || saved.Processing[1].ID != "validation" {
		t.Fatal("the original JSON path did not receive the new processing entry")
	}
	for _, untouched := range []struct {
		path string
		want []byte
	}{{dslPath, dslBefore}, {decoy, source}} {
		got, err := os.ReadFile(untouched.path)
		if err != nil || !bytes.Equal(got, untouched.want) {
			t.Errorf("file %q changed: %v", untouched.path, err)
		}
	}
}

func TestSnapshotOriginalDoesNotChangeWithModel(t *testing.T) {
	path, source := snapshotTestFile(t)
	result, snapshot, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || snapshot == nil {
		t.Fatal("Open returned no model or snapshot")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	assertSnapshotUnchanged(t, snapshot, path, source, info)
	*result.Document.DSLVersion = "changed"
	*result.Document.FilePath = "changed.dsl"
	result.Document.HasErrors = true
	result.Processing[0].Tool = "changed"
	result.Lines[0].Raw = "changed"
	*result.Lines[0].Elements[0].Value = "changed"
	result.Lines[0].Elements[0].ErrorIDs = append(result.Lines[0].Elements[0].ErrorIDs, "changed")
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: "changed"})
	// Изменённая модель намеренно не сохраняется: здесь проверяется владение памятью.
	assertSnapshotUnchanged(t, snapshot, path, source, info)
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, source) {
		t.Fatalf("model changes affected the source file: %v", err)
	}
}

func TestSnapshotRemainsOriginalAfterSave(t *testing.T) {
	path, source := snapshotTestFile(t)
	result, snapshot, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || snapshot == nil {
		t.Fatal("Open returned no model or snapshot")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	result.Processing = append(result.Processing, model.Processing{ID: "validation", Tool: "dsl-validator"})
	if updated, err := Save(snapshot, result); !updated || err != nil {
		t.Fatalf("Save = %v, %v; want true, nil", updated, err)
	}
	assertSnapshotUnchanged(t, snapshot, path, source, info)
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(saved, source) {
		t.Fatal("Save did not install the updated JSON")
	}
	if updated, err := Save(snapshot, result); updated || err == nil {
		t.Fatalf("Save with stale snapshot = %v, %v; want false, error", updated, err)
	}
	assertSnapshotUnchanged(t, snapshot, path, source, info)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, saved) {
		t.Fatalf("rejected Save changed the installed JSON: %v", err)
	}
}

func TestSnapshotDoesNotRetainFileDescriptor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("descriptor inspection requires Linux /proc/self/fd")
	}
	path, _ := snapshotTestFile(t)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	result, snapshot, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || snapshot == nil {
		t.Fatal("Open returned no model or snapshot")
	}
	assertSnapshotFileClosed(t, info)
	result.Processing = append(result.Processing, model.Processing{ID: "validation", Tool: "dsl-validator"})
	if updated, err := Save(snapshot, result); !updated || err != nil {
		t.Fatalf("Save = %v, %v; want true, nil", updated, err)
	}
	installed, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	assertSnapshotFileClosed(t, info)
	assertSnapshotFileClosed(t, installed)
	runtime.KeepAlive(snapshot)
}

func snapshotTestFile(t *testing.T) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	dslVersion, fileName, encoding := "1.2", "source.dsl", "UTF-8"
	dslPath := filepath.Join(dir, fileName)
	raw := "Текст"
	hasBOM, lineCount, byteLength := false, 1, int64(len(raw))
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	result := model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &dslVersion, FileName: &fileName, FilePath: &dslPath, Encoding: &encoding,
			HasBOM: &hasBOM, LineCount: &lineCount, ByteLength: &byteLength, SHA256: &digest,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}},
		},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines: []model.Line{{
			Number: 1, LineType: model.LineTypeContent, Raw: raw, LineEnding: model.LineEndingNone,
			Elements: []model.Element{{
				ElementType: model.ElementTypeContent, Raw: raw, Value: &raw,
				Start: 1, End: len([]rune(raw)) + 1, ErrorIDs: []string{},
			}},
		}},
		Diagnostics: []model.Diagnostic{},
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	// Необычные пробелы и неизвестное поле должны остаться в точных байтах снимка.
	source := append([]byte(" \n{\"extension\": {\"kept\": true}, "), encoded[1:]...)
	source = append(source, '\n', '\t')
	path := filepath.Join(dir, "result.json")
	if err := os.WriteFile(dslPath, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	return path, source
}

func assertSnapshotUnchanged(t *testing.T, snapshot *Snapshot, path string, source []byte, info os.FileInfo) {
	t.Helper()
	if snapshot.path != path || !bytes.Equal(snapshot.original, source) {
		t.Error("snapshot path or exact original bytes changed")
	}
	got := snapshot.info
	if got == nil || !os.SameFile(got, info) || got.Name() != info.Name() ||
		got.Mode() != info.Mode() || got.Size() != info.Size() || !got.ModTime().Equal(info.ModTime()) {
		t.Error("snapshot does not retain the original file information")
	}
}

func assertSnapshotFileClosed(t *testing.T, info os.FileInfo) {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		opened, err := os.Stat(filepath.Join("/proc/self/fd", entry.Name()))
		// Дескриптор самого ReadDir уже закрыт и может присутствовать в списке.
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(opened, info) {
			t.Errorf("file %q still has open descriptor %s", info.Name(), entry.Name())
		}
	}
}

package storage_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/storage"
)

func TestOpenSavePreservesHistoryAndSupportsReopening(t *testing.T) {
	path, original, _ := storageTestFile(t, "arbitrary.JsOn")
	result, snapshot, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	result.Processing = append(result.Processing, model.Processing{ID: "p1", Tool: "dsl-validator"})
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
		ID: "d1", Source: "p1", DiagnosticCode: diagnostics.Code("V001"),
		SeverityLevel: diagnostics.SeverityError, DiagnosticScope: diagnostics.ScopeElement, Message: "<ошибка>&",
		Location:         &model.Location{Start: model.Position{Line: 1, Column: 14}, End: model.Position{Line: 1, Column: 17}},
		RelatedLocations: []model.Location{},
	})
	result.Document.HasErrors, result.Lines[0].HasErrors = true, true
	result.Lines[0].Elements[1].ErrorIDs = append(result.Lines[0].Elements[1].ErrorIDs, "d1")
	expected := storageTestModel(t, result)
	if updated, err := storage.Save(snapshot, result); !updated || err != nil {
		t.Fatalf("Save = %v, %v; want true, nil", updated, err)
	}
	if !bytes.Equal(storageTestModel(t, result), expected) {
		t.Fatal("Save mutated the supplied model")
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	storageTestPreservedHistory(t, original, saved)
	if updated, err := storage.Save(snapshot, result); updated || err == nil {
		t.Fatalf("Save with old snapshot = %v, %v; want false, error", updated, err)
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(saved, after) {
		t.Fatalf("rejected Save changed the installed file: %v", err)
	}
	reopened, fresh, err := storage.Open(path)
	if err != nil || !bytes.Equal(storageTestModel(t, reopened), expected) {
		t.Fatalf("reopened model differs from the saved model: %v", err)
	}
	reopened.Processing = append(reopened.Processing, model.Processing{ID: "p2", Tool: "dsl-validator"})
	if updated, err := storage.Save(fresh, reopened); !updated || err != nil {
		t.Fatalf("Save after reopening = %v, %v; want true, nil", updated, err)
	}
	final, _, err := storage.Open(path)
	if err != nil || !bytes.Equal(storageTestModel(t, final), storageTestModel(t, reopened)) {
		t.Fatalf("second Save lost history or invented records: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	storageTestPreservedHistory(t, saved, contents)
	if _, err := os.Stat(*final.Document.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Save unexpectedly created the DSL source: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("temporary or backup files remain: %v, %v", entries, err)
	}
}

func TestOpenAcceptsRelativePathsAndSaveAddsNoRecords(t *testing.T) {
	for _, name := range []string{"anything.json", "UPPER.JSON", "mixed.JsOn"} {
		t.Run(name, func(t *testing.T) {
			path, original, expected := storageTestFile(t, name)
			t.Chdir(filepath.Dir(path))
			if err := os.Mkdir("sub", 0700); err != nil {
				t.Fatal(err)
			}
			result, snapshot, err := storage.Open("sub/../" + name)
			if err != nil || !bytes.Equal(storageTestModel(t, result), storageTestModel(t, expected)) {
				t.Fatalf("Open changed the model or failed: %v", err)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, original) {
				t.Fatalf("Open changed the source: %v", err)
			}
			if updated, err := storage.Save(snapshot, result); !updated || err != nil {
				t.Fatalf("Save without additions = %v, %v", updated, err)
			}
			reopened, _, err := storage.Open(path)
			if err != nil || !bytes.Equal(storageTestModel(t, reopened), storageTestModel(t, expected)) {
				t.Fatalf("Save without additions changed the model: %v", err)
			}
		})
	}
}

func TestOpenRejectsInvalidPathsBeforeCleaning(t *testing.T) {
	path, original, _ := storageTestFile(t, "source.json")
	t.Chdir(filepath.Dir(path))
	if err := os.Mkdir("sub", 0700); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{"file-link.json": path, "dir-link": "sub"} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"source.jsonc", "source.txt", "source"} {
		if err := os.WriteFile(name, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"", ".", "sub", "missing.json", "file-link.json", "dir-link/../source.json",
		"missing/../source.json", "source.json/../source.json", "source.json/", "source.jsonc", "source.txt", "source"} {
		t.Run(name, func(t *testing.T) {
			result, snapshot, err := storage.Open(name)
			if err == nil || result != nil || snapshot != nil {
				t.Fatalf("Open = %v, %v, %v; want nil, nil, error", result, snapshot, err)
			}
		})
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, original) {
		t.Fatalf("failed Open changed the source: %v", err)
	}
}

func TestOpenLeavesMalformedJSONUntouched(t *testing.T) {
	path, valid, _ := storageTestFile(t, "source.json")
	for name, source := range map[string][]byte{
		"syntax": []byte("not JSON"), "shape": []byte(`{}`), "bom": append([]byte("\xef\xbb\xbf"), valid...),
		"version": bytes.Replace(valid, []byte(`"formatVersion":"1.0"`), []byte(`"formatVersion":"2.0"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, source, 0600); err != nil {
				t.Fatal(err)
			}
			result, snapshot, err := storage.Open(path)
			if err == nil || result != nil || snapshot != nil {
				t.Fatalf("Open = %v, %v, %v; want nil, nil, error", result, snapshot, err)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, source) {
				t.Fatalf("failed decoding changed the file: %v", err)
			}
		})
	}
}

func TestSaveRejectsChangedSourceAndInvalidCountsWithoutWriting(t *testing.T) {
	for _, name := range []string{"changed_bytes", "replaced_file", "missing", "symlink", "line_count"} {
		t.Run(name, func(t *testing.T) {
			path, original, _ := storageTestFile(t, "source.json")
			result, snapshot, err := storage.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			expected := original
			switch name {
			case "changed_bytes":
				expected = append(bytes.Clone(original), '\n')
				err = os.WriteFile(path, expected, 0600)
			case "replaced_file", "symlink":
				if err = os.Rename(path, path+".old"); err == nil {
					if name == "symlink" {
						err = os.Symlink(path+".old", path)
					} else {
						err = os.WriteFile(path, original, 0600)
					}
				}
			case "missing":
				err = os.Remove(path)
			case "line_count":
				result.Lines = result.Lines[:0]
			}
			if err != nil {
				t.Fatal(err)
			}
			modelBefore := storageTestModel(t, result)
			if updated, err := storage.Save(snapshot, result); updated || err == nil {
				t.Fatalf("Save = %v, %v; want false, error", updated, err)
			}
			after, err := os.ReadFile(path)
			if name == "missing" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Save recreated the source: %v", err)
				}
			} else if err != nil || !bytes.Equal(after, expected) {
				t.Fatalf("rejected Save changed the source: %v", err)
			}
			if !bytes.Equal(storageTestModel(t, result), modelBefore) {
				t.Fatal("rejected Save changed the model")
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() != "source.json" && entry.Name() != "source.json.old" {
					t.Errorf("unexpected file after rejected Save: %s", entry.Name())
				}
			}
		})
	}
}

func storageTestFile(t *testing.T, name string) (string, []byte, *model.Result) {
	t.Helper()
	dir := t.TempDir()
	dslVersion, fileName, encoding, tag := "1.2", "missing.dsl", "UTF-8", "dsl-version"
	dslPath, raw := filepath.Join(dir, fileName), "@dsl-version 1.2"
	hasBOM, lineCount, byteLength := false, 1, int64(len(raw))
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &dslVersion, FileName: &fileName, FilePath: &dslPath, Encoding: &encoding,
			HasBOM: &hasBOM, LineCount: &lineCount, ByteLength: &byteLength, SHA256: &digest,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "p0", Tool: "dsl-parser"}},
		Lines: []model.Line{{Number: 1, LineType: model.LineTypeTag, Raw: raw, LineEnding: model.LineEndingNone,
			Elements: []model.Element{
				{ElementType: model.ElementTypeTag, Raw: "@dsl-version", Value: &tag, Start: 1, End: 13, ErrorIDs: []string{}},
				{ElementType: model.ElementTypeVersion, Raw: "1.2", Value: &dslVersion, Start: 14, End: 17, ErrorIDs: []string{}},
			}}},
		Diagnostics: []model.Diagnostic{{ID: "d0", Source: "p0", DiagnosticCode: diagnostics.Code("P001"),
			SeverityLevel: diagnostics.SeverityWarning, DiagnosticScope: diagnostics.ScopeDocument,
			Message: "old warning", RelatedLocations: []model.Location{}}},
	}
	source := storageTestModel(t, result)
	source = bytes.Replace(source, []byte("{"), []byte(`{"extension":{"large":9007199254740993123456789,"nested":[null,{"x":"<>&"}]},`), 1)
	source = bytes.Replace(source, []byte(`"id":"p0"`), []byte(`"id":"p0","extra":{"parser":true}`), 1)
	source = bytes.Replace(source, []byte(`"id":"d0"`), []byte(`"id":"d0","extra":[1,"old"]`), 1)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	return path, source, result
}

func storageTestModel(t *testing.T, result *model.Result) []byte {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func storageTestPreservedHistory(t *testing.T, before, after []byte) {
	t.Helper()
	var original, saved map[string]json.RawMessage
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &saved); err != nil {
		t.Fatal(err)
	}
	previous, current := []json.RawMessage{original["extension"]}, []json.RawMessage{saved["extension"]}
	for _, name := range []string{"processing", "diagnostics"} {
		var oldRecords, newRecords []json.RawMessage
		if err := json.Unmarshal(original[name], &oldRecords); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(saved[name], &newRecords); err != nil || len(newRecords) < len(oldRecords) {
			t.Fatalf("lost history in %s: %v", name, err)
		}
		previous = append(previous, oldRecords...)
		current = append(current, newRecords[:len(oldRecords)]...)
	}
	for i := range previous {
		var oldCompact, newCompact bytes.Buffer
		if err := json.Compact(&oldCompact, previous[i]); err != nil {
			t.Fatal(err)
		}
		if err := json.Compact(&newCompact, current[i]); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(oldCompact.String()) != strings.TrimSpace(newCompact.String()) {
			t.Errorf("unknown fields or existing record changed: %s -> %s", &oldCompact, &newCompact)
		}
	}
}

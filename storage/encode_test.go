package storage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestEncodeUpdatePreservesOriginalAndAddsOnlyAllowedFields(t *testing.T) {
	for _, additions := range []bool{false, true} {
		t.Run("additions="+strconv.FormatBool(additions), func(t *testing.T) {
			snapshot, result := encodeTestFixture(t)
			if additions {
				result.Processing = append(result.Processing,
					model.Processing{ID: "p1", Tool: "<tool>&"}, model.Processing{ID: "p2", Tool: "validator"})
				result.Diagnostics = append(result.Diagnostics,
					model.Diagnostic{ID: "d<new>&", Source: "p1", DiagnosticCode: diagnostics.Code("V001"),
						SeverityLevel: diagnostics.SeverityError, DiagnosticScope: diagnostics.ScopeElement,
						Message: "<new>& Русский 🌍", Location: &model.Location{
							Start: model.Position{Line: 1, Column: 1}, End: model.Position{Line: 1, Column: 4}},
						RelatedLocations: []model.Location{}},
					model.Diagnostic{ID: "d2", Source: "p2", DiagnosticCode: diagnostics.Code("V002"),
						SeverityLevel: diagnostics.SeverityWarning, DiagnosticScope: diagnostics.ScopeDocument,
						Message: "second", RelatedLocations: []model.Location{}})
				result.Document.HasErrors, result.Lines[0].HasErrors = true, true
				result.Lines[0].Elements[0].ErrorIDs = append(result.Lines[0].Elements[0].ErrorIDs, "d<new>&")
			}
			before := *snapshot
			before.original = bytes.Clone(snapshot.original)
			modelBefore := encodeTestModel(t, result)
			payload, err := encodeUpdate(snapshot, result)
			if err != nil {
				t.Fatal(err)
			}
			encodeTestInputsUnchanged(t, snapshot, &before, result, modelBefore)
			decoded, err := decodeResult(payload)
			if err != nil || !bytes.Equal(encodeTestModel(t, decoded), modelBefore) {
				t.Fatalf("encoded known fields do not match the supplied model: %v", err)
			}
			// UseNumber сохраняет точные числа неизвестных полей при сравнении.
			var originalJSON, updatedJSON any
			for _, document := range []struct {
				source []byte
				target *any
			}{{before.original, &originalJSON}, {payload, &updatedJSON}} {
				decoder := json.NewDecoder(bytes.NewReader(document.source))
				decoder.UseNumber()
				if err := decoder.Decode(document.target); err != nil {
					t.Fatal(err)
				}
			}
			encodeTestPreserved(t, originalJSON, updatedJSON, "")
			var indented bytes.Buffer
			if err := json.Indent(&indented, bytes.TrimSpace(payload), "", "  "); err != nil {
				t.Fatal(err)
			}
			indented.WriteByte('\n')
			if !bytes.Equal(payload, indented.Bytes()) {
				t.Fatal("expected two-space indentation and exactly one final LF")
			}
			literals := []string{`<>&`, `"unknown": "<old>&"`, `9007199254740993123456789`, `1e9999`, `1.2300`}
			if additions {
				literals = append(literals, `<tool>&`, `d<new>&`, `<new>& Русский 🌍`)
			}
			for _, literal := range literals {
				if !bytes.Contains(payload, []byte(literal)) {
					t.Errorf("output lost or escaped %q", literal)
				}
			}
			payload[0] = '!'
			encodeTestInputsUnchanged(t, snapshot, &before, result, modelBefore)
		})
	}
}

func TestEncodeUpdateRejectsChangedCountsWithoutMutatingInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*model.Result)
	}{
		{"shorter_processing", func(r *model.Result) { r.Processing = r.Processing[:0] }},
		{"shorter_diagnostics", func(r *model.Result) { r.Diagnostics = r.Diagnostics[:0] }},
		{"fewer_lines", func(r *model.Result) { r.Lines = r.Lines[:0] }},
		{"more_lines", func(r *model.Result) { r.Lines = append(r.Lines, r.Lines[0]) }},
		{"fewer_elements", func(r *model.Result) { r.Lines[0].Elements = r.Lines[0].Elements[:0] }},
		{"more_elements", func(r *model.Result) {
			r.Lines[0].Elements = append(r.Lines[0].Elements, r.Lines[0].Elements[0])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, result := encodeTestFixture(t)
			tc.change(result)
			before := *snapshot
			before.original = bytes.Clone(snapshot.original)
			modelBefore := encodeTestModel(t, result)
			payload, err := encodeUpdate(snapshot, result)
			if err == nil || payload != nil {
				t.Fatalf("encodeUpdate = %s, %v; want nil, error", payload, err)
			}
			encodeTestInputsUnchanged(t, snapshot, &before, result, modelBefore)
		})
	}
}

func TestEncodeEmptyArraysStayArrays(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func() (json.RawMessage, error)
	}{
		{"processing", func() (json.RawMessage, error) { return appendProcessing([]byte(`[]`), []model.Processing{}) }},
		{"diagnostics", func() (json.RawMessage, error) { return appendDiagnostics([]byte(`[]`), []model.Diagnostic{}) }},
		{"lines", func() (json.RawMessage, error) { return updateLineFlags([]byte(`[]`), []model.Line{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := tc.run()
			if err != nil || !bytes.Equal(bytes.TrimSpace(encoded), []byte(`[]`)) {
				t.Fatalf("encoded = %s, %v; want []", encoded, err)
			}
		})
	}
}

func TestUpdateLineFlagsPreservesEmptyElementsAndErrorIDs(t *testing.T) {
	original := json.RawMessage(`[{"hasErrors":true,"elements":[],"unknown":"<>&"},
	{"hasErrors":true,"elements":[{"errorIds":[],"unknown":9007199254740993123456789}]}]`)
	before := bytes.Clone(original)
	encoded, err := updateLineFlags(original, []model.Line{
		{Elements: []model.Element{}}, {Elements: []model.Element{{ErrorIDs: []string{}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, encoded); err != nil {
		t.Fatal(err)
	}
	want := `[{"elements":[],"hasErrors":false,"unknown":"<>&"},{"elements":[{"errorIds":[],"unknown":9007199254740993123456789}],"hasErrors":false}]`
	if compact.String() != want || !bytes.Equal(original, before) {
		t.Fatalf("unexpected output or changed input: %s", compact.String())
	}
}

func encodeTestFixture(t *testing.T) (*Snapshot, *model.Result) {
	t.Helper()
	source := []byte(`{
 "unknown":{"huge":9007199254740993123456789,"exponent":1e9999,"fraction":1.2300,"nested":[null,{"text":"<old>&"}]},
 "formatVersion":"1.0",
 "document":{"dslVersion":"1.2","fileName":null,"filePath":null,"encoding":null,"hasBom":null,
 "lineCount":1,"byteLength":null,"sha256":null,"hasErrors":false,"HasErrors":{"untouched":true},
 "metadata":{"documentId":null,"title":null,"subtitle":null,"section":null,"order":null,"resourceDirs":null,"unknown":[1,{"x":2}]},"unknown":"<old>&"},
 "processing":[{"id":"old","tool":"parser","version":null,"startedAt":null,"durationMs":null,"unknown":{"n":9007199254740993123456789}}],
 "lines":[{"line":1,"type":"content","nestingLevel":0,"parentLine":null,"raw":"<>&","eol":"","hasErrors":false,"unknown":{"x":[1,2]},
 "elements":[{"type":"content","raw":"<>&","value":"<>&","start":1,"end":4,"errorIds":["d-old"],"unknown":{"escaped":"\u003clegacy\u003e"}}]}],
 "diagnostics":[{"id":"d-old","source":"old","code":"P001","severity":"warning","message":"old <>&","scope":"element","fatal":false,"unknown":1.2300,
 "location":{"start":{"line":1,"column":1,"unknown":[true]},"end":{"line":1,"column":4,"unknown":null},"unknown":{"range":true}},
 "relatedLocations":[{"start":{"line":1,"column":4,"unknown":42},"end":{"line":1,"column":4,"unknown":false},"unknown":"related"}]}]
}
`)
	result, err := decodeResult(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "original.json")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return &Snapshot{path: path, original: source, info: info}, result
}

func encodeTestModel(t *testing.T, result *model.Result) []byte {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func encodeTestInputsUnchanged(t *testing.T, snapshot, before *Snapshot, result *model.Result, modelBefore []byte) {
	t.Helper()
	if snapshot.path != before.path || !bytes.Equal(snapshot.original, before.original) || snapshot.info != before.info {
		t.Error("encoding changed the snapshot")
	}
	if !bytes.Equal(encodeTestModel(t, result), modelBefore) {
		t.Error("encoding changed the supplied model")
	}
	source, err := os.ReadFile(before.path)
	if err != nil || !bytes.Equal(source, before.original) {
		t.Errorf("encoding changed the source file: %v", err)
	}
}

func encodeTestPreserved(t *testing.T, original, updated any, path string) {
	t.Helper()
	if path == "/document/hasErrors" || path == "/lines/0/hasErrors" || path == "/lines/0/elements/0/errorIds" {
		return
	}
	switch old := original.(type) {
	case map[string]any:
		current, ok := updated.(map[string]any)
		if !ok || len(current) != len(old) {
			t.Fatalf("object fields changed at %s", path)
		}
		for key, previous := range old {
			next, present := current[key]
			if !present {
				t.Fatalf("field %s/%s disappeared", path, key)
			}
			encodeTestPreserved(t, previous, next, path+"/"+key)
		}
	case []any:
		current, ok := updated.([]any)
		appendOnly := path == "/processing" || path == "/diagnostics"
		if !ok || len(current) < len(old) || !appendOnly && len(current) != len(old) {
			t.Fatalf("array changed at %s", path)
		}
		for i := range old {
			encodeTestPreserved(t, old[i], current[i], path+"/"+strconv.Itoa(i))
		}
	default:
		if original != updated {
			t.Errorf("value changed at %s: %v -> %v", path, original, updated)
		}
	}
}

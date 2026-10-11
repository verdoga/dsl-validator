package app

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/verdoga/dsl-validator/model"
)

func TestNextProcessingIDIsFreeAcrossHistory(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{"empty", nil},
		{"occupied prefix", []string{"p1", "p2", "p3"}},
		{"sparse unordered", []string{"p7", "p1", "p3"}},
		{"arbitrary IDs", []string{"parser", "p0", "p01", "p-1", "P1", "p1"}},
		{"large IDs", []string{"p18446744073709551616", "p" + strings.Repeat("9", 200), "p1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := make([]model.Processing, len(tc.ids))
			for i, id := range tc.ids {
				entries[i] = model.Processing{ID: id, Tool: "previous"}
			}
			result := &model.Result{Processing: entries}
			before := processingSnapshot(t, result)
			id := nextProcessingID(entries)
			if len(id) < 2 || id[0] != 'p' || id[1] < '1' || id[1] > '9' {
				t.Fatalf("ID %q is not pN with N >= 1", id)
			}
			for _, digit := range id[1:] {
				if digit < '0' || digit > '9' {
					t.Fatalf("ID %q contains a nondecimal digit", id)
				}
			}
			for _, previous := range tc.ids {
				if id == previous {
					t.Fatalf("ID %q is already occupied", id)
				}
			}
			if processingSnapshot(t, result) != before {
				t.Fatal("ID selection changed the history")
			}
		})
	}
}

func TestStartProcessingAppendsOneEntryAndPreservesModel(t *testing.T) {
	for _, history := range []bool{false, true} {
		t.Run(strconv.FormatBool(history), func(t *testing.T) {
			result := processingModel(t)
			if !history {
				result.Processing = []model.Processing{}
				result.Diagnostics = []model.Diagnostic{}
			}
			started := time.Date(2026, 10, 11, 12, 34, 56, 123456789, time.FixedZone("UTC+3", 3*60*60))
			for _, version := range []string{"dev", "2.3.4"} {
				before := processingSnapshot(t, result)
				previousCount := len(result.Processing)
				id := startProcessing(result, version, started)
				if len(result.Processing) != previousCount+1 {
					t.Fatal("start did not append exactly one entry")
				}
				entry := result.Processing[previousCount]
				if entry.ID != id || id == "" || entry.Tool != "dsl-validator" {
					t.Fatalf("unexpected entry identity: %+v", entry)
				}
				for _, old := range result.Processing[:previousCount] {
					if old.ID == id {
						t.Fatalf("new ID %q collides with history", id)
					}
				}
				if entry.Version == nil || *entry.Version != version || entry.DurationMs != nil {
					t.Fatalf("unexpected version or initial duration: %+v", entry)
				}
				if entry.StartedAt == nil || *entry.StartedAt != "2026-10-11T09:34:56.123456789Z" {
					t.Fatalf("start time is not UTC/RFC3339Nano: %+v", entry)
				}
				withoutCurrent := *result
				withoutCurrent.Processing = result.Processing[:previousCount]
				if processingSnapshot(t, &withoutCurrent) != before {
					t.Fatal("start changed existing model data")
				}
			}
		})
	}
}

func TestFinishProcessingUpdatesOnlyRequestedEntry(t *testing.T) {
	result := processingModel(t)
	started := time.Now().Add(-1500*time.Millisecond - 123*time.Microsecond)
	id := startProcessing(result, "dev", started)
	index := len(result.Processing) - 1
	result.Processing = append(result.Processing, model.Processing{ID: "later", Tool: "other"})
	before := processingSnapshot(t, result)
	lower := time.Since(started)
	duration, err := finishProcessing(result, id, started)
	upper := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if duration < lower || duration > upper {
		t.Fatalf("duration %v outside measured interval [%v, %v]", duration, lower, upper)
	}
	stored := result.Processing[index].DurationMs
	if stored == nil || int64(*stored) != duration.Milliseconds() {
		t.Fatalf("stored duration %v does not match %v", stored, duration)
	}
	result.Processing[index].DurationMs = nil
	if processingSnapshot(t, result) != before {
		t.Fatal("finish changed data other than the requested duration")
	}
}

func TestFinishProcessingClampsNegativeDuration(t *testing.T) {
	result := processingModel(t)
	started := time.Now().Add(time.Hour)
	id := startProcessing(result, "dev", started)
	duration, err := finishProcessing(result, id, started)
	if err != nil || duration != 0 {
		t.Fatalf("finish = %v, %v; want zero duration and no error", duration, err)
	}
	stored := result.Processing[len(result.Processing)-1].DurationMs
	if stored == nil || *stored != 0 {
		t.Fatalf("stored duration = %v; want known zero", stored)
	}
}

func TestFinishProcessingMissingIDDoesNotChangeModel(t *testing.T) {
	for _, id := range []string{"missing", ""} {
		t.Run(id, func(t *testing.T) {
			result := processingModel(t)
			before := processingSnapshot(t, result)
			duration, err := finishProcessing(result, id, time.Now())
			if err == nil || duration < 0 {
				t.Fatalf("finish = %v, %v; want error and nonnegative duration", duration, err)
			}
			if processingSnapshot(t, result) != before {
				t.Fatal("missing ID changed the model")
			}
		})
	}
}

func TestFinishProcessingChecksIntCapacityBeforeMutation(t *testing.T) {
	result := processingModel(t)
	started := time.Now().Add(-time.Duration(1<<31) * time.Millisecond)
	id := startProcessing(result, "dev", started)
	before := processingSnapshot(t, result)
	duration, err := finishProcessing(result, id, started)
	if duration.Milliseconds() < 1<<31 {
		t.Fatalf("duration %v is too short for the boundary case", duration)
	}
	if strconv.IntSize == 32 {
		if err == nil {
			t.Fatal("duration overflowing int32 was accepted")
		}
		if processingSnapshot(t, result) != before {
			t.Fatal("overflow changed the model")
		}
		return
	}
	if err != nil {
		t.Fatalf("duration fitting int64 was rejected: %v", err)
	}
	stored := result.Processing[len(result.Processing)-1].DurationMs
	if stored == nil || int64(*stored) != duration.Milliseconds() {
		t.Fatal("duration fitting int64 was not stored correctly")
	}
}

func processingModel(t *testing.T) *model.Result {
	t.Helper()
	version, name, path, encoding := "1.2", "input.dsl", "/input.dsl", "UTF-8"
	count, size, bom, hash := 1, int64(1), false, strings.Repeat("a", 64)
	oldVersion, oldStarted, oldDuration := "parser-version", "2026-10-10T12:00:00+03:00", 42
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, FileName: &name, FilePath: &path, Encoding: &encoding,
			HasBOM: &bom, LineCount: &count, ByteLength: &size, SHA256: &hash,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{"resources"}},
		},
		Processing: []model.Processing{
			{ID: "p3", Tool: "dsl-parser", Version: &oldVersion, StartedAt: &oldStarted, DurationMs: &oldDuration},
			{ID: "p1", Tool: "previous-validator"},
		},
		Lines: []model.Line{{Number: 1, LineType: model.LineTypeBlank, LineEnding: model.LineEndingLF, Elements: []model.Element{}}},
		Diagnostics: []model.Diagnostic{{
			ID: "d1", Source: "p3", DiagnosticCode: "legacy", SeverityLevel: "recommendation",
			DiagnosticScope: "document", Message: "сохранённое сообщение", RelatedLocations: []model.Location{},
		}},
	}
}

func processingSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

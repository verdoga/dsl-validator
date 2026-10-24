package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/storage"
)

func TestRunPrintsBuildVersionBeforeDiagnosticCount(t *testing.T) {
	for _, tc := range []struct {
		version, banner string
	}{
		{"1.0.1", "dsl-validator v1.0.1\n"},
		{"v2.3.4", "dsl-validator v2.3.4\n"},
		{"dev", "dsl-validator dev\n"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			input := strings.Replace(runValidJSON, `"tool": "dsl-parser", "version": null`,
				`"tool": "dsl-parser", "version": "json-only-version"`, 1)
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			code, output, errorOutput := runCaptured(t, []string{path}, tc.version, "", "")
			wantStart := tc.banner + strings.TrimPrefix(runReportStart(t), "dsl-validator dev\n")
			if code != 0 || errorOutput != "" || !strings.HasPrefix(output, wantStart) ||
				strings.Count(output, "dsl-validator ") != 1 || strings.Contains(output, "json-only-version") {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
			saved, _, err := storage.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			current := saved.Processing[len(saved.Processing)-1]
			if current.Version == nil || *current.Version != tc.version ||
				saved.Processing[0].Version == nil || *saved.Processing[0].Version != "json-only-version" {
				t.Fatalf("unexpected processing versions: %+v", saved.Processing)
			}
		})
	}
}

func TestRunBannerWriteFailureDoesNotChangeJSON(t *testing.T) {
	for _, mode := range []string{"closed", "read-only"} {
		t.Run(mode, func(t *testing.T) {
			path, original := runnerFile(t, processingModel(t))
			code, output, errorOutput := runCaptured(t, []string{path}, "1.0.1", mode, "")
			if code != 1 || output != "" || !strings.HasPrefix(errorOutput, "Ошибка: ") || strings.Count(errorOutput, "\n") != 1 {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
			runnerUnchanged(t, path, original)
		})
	}
}

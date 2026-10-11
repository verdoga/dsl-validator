package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/reporter"
	"github.com/verdoga/dsl-validator/storage"
	"github.com/verdoga/dsl-validator/validator"
)

func TestProcessFileSavesEachSeverityAndPreservesHistory(t *testing.T) {
	for severity, counts := range map[diagnostics.Severity]string{
		"error":          "ошибок: 2, проблем: 0, рекомендаций: 0",
		"warning":        "ошибок: 0, проблем: 2, рекомендаций: 0",
		"recommendation": "ошибок: 0, проблем: 0, рекомендаций: 2",
	} {
		t.Run(string(severity), func(t *testing.T) {
			previous := processingModel(t)
			path, _ := runnerFile(t, previous)
			definition := runnerDefinition(func(_ diagnostics.Lines, f diagnostics.Findings) error {
				_, err := f.Add(0, 0, 0, 0, "найдено")
				return err
			})
			definition.Severity = severity
			var output, errorOutput bytes.Buffer
			r := runner{version: "tool-version", registry: runnerRegistry(t, definition), reporter: reporter.New(&output, &errorOutput)}
			for attempt := 0; attempt < 2; attempt++ {
				output.Reset()
				if err := r.processFile(path); err != nil {
					t.Fatal(err)
				}
				saved, _, err := storage.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := validator.CheckModel(saved); err != nil {
					t.Fatal(err)
				}
				if len(saved.Processing) != len(previous.Processing)+1 || len(saved.Diagnostics) != len(previous.Diagnostics)+1 {
					t.Fatal("unexpected number of appended entries")
				}
				current := saved.Processing[len(saved.Processing)-1]
				if current.Version == nil || *current.Version != r.version || current.DurationMs == nil || *current.DurationMs < 0 {
					t.Fatalf("invalid current processing: %+v", current)
				}
				if saved.Diagnostics[len(saved.Diagnostics)-1].Source != current.ID {
					t.Fatal("wrong diagnostic source")
				}
				prefix := *saved
				prefix.Processing = saved.Processing[:len(previous.Processing)]
				prefix.Diagnostics = saved.Diagnostics[:len(previous.Diagnostics)]
				prefix.Document.HasErrors = previous.Document.HasErrors
				if processingSnapshot(t, &prefix) != processingSnapshot(t, previous) {
					t.Fatal("history or document changed")
				}
				if strings.Count(output.String(), "\n") != 1 || errorOutput.Len() != 0 || !strings.Contains(output.String(), fmt.Sprintf("время: %d мс", *current.DurationMs)) {
					t.Fatalf("unexpected file report: %q / %q", output.String(), errorOutput.String())
				}
				previous = saved
			}
			runnerTotal(t, r.reporter, &output, 0, "всего: 2, обработано: 2, сбоев: 0, сохранено: 2, "+counts)
		})
	}
}

func TestProcessFileRejectsInputWithoutSavingOrRunningChecks(t *testing.T) {
	for _, kind := range []string{"missing", "invalid JSON", "unknown", "malformed", "unsupported", "no checks", "integrity"} {
		t.Run(kind, func(t *testing.T) {
			result := processingModel(t)
			version, reason := "1.2", ""
			switch kind {
			case "unknown":
				result.Document.DSLVersion, reason = nil, "не задана"
			case "malformed":
				version, reason = "v1.2", "v1.2"
			case "unsupported":
				version, reason = "2.0", "2.0"
			case "no checks":
				reason = "нет подходящих диагностик"
			case "integrity":
				result.Lines[0].Number, reason = 2, "целостность модели"
			}
			if kind != "unknown" {
				result.Document.DSLVersion = &version
			}
			path, original := runnerFile(t, result)
			if kind == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "invalid JSON" {
				original = "{"
				if err := os.WriteFile(path, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			definition := runnerDefinition(func(diagnostics.Lines, diagnostics.Findings) error { calls++; return nil })
			if kind == "no checks" {
				definition.Versions = diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "2.0"}
			}
			var output, errorOutput bytes.Buffer
			r := runner{version: "dev", registry: runnerRegistry(t, definition), reporter: reporter.New(&output, &errorOutput)}
			err := r.processFile(path)
			if err == nil || calls != 0 {
				t.Fatalf("error=%v, Check calls=%d", err, calls)
			}
			if kind == "missing" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("lost open error: %v", err)
				}
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("missing file was created")
				}
			} else {
				runnerUnchanged(t, path, original)
			}
			line := errorOutput.String()
			if output.Len() != 0 || strings.Count(line, "\n") != 1 || !strings.Contains(line, fmt.Sprintf("%q", path)) || !strings.Contains(line, reason) || !strings.Contains(line, "сохранён: нет, строк: нет данных, ошибок: 0, проблем: 0, рекомендаций: 0") {
				t.Fatalf("unexpected refusal report: %q / %q", output.String(), line)
			}
			if kind == "no checks" && !strings.Contains(line, version) {
				t.Fatal("version missing from refusal")
			}
			runnerTotal(t, r.reporter, &output, 1, "всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0")
		})
	}
}

func TestProcessFilePreparesVersionAgainForNextDocument(t *testing.T) {
	var output, errorOutput bytes.Buffer
	calls := 0
	r := runner{version: "2.0", registry: runnerRegistry(t, runnerDefinition(func(diagnostics.Lines, diagnostics.Findings) error { calls++; return nil })), reporter: reporter.New(&output, &errorOutput)}
	first, _ := runnerFile(t, processingModel(t))
	if err := r.processFile(first); err != nil {
		t.Fatal(err)
	}
	second := processingModel(t)
	second.Document.DSLVersion = &r.version
	path, original := runnerFile(t, second)
	if err := r.processFile(path); err == nil || calls != 1 {
		t.Fatalf("error=%v, calls=%d", err, calls)
	}
	runnerUnchanged(t, path, original)
	runnerTotal(t, r.reporter, &output, 1, "всего: 2, обработано: 1, сбоев: 1, сохранено: 1, ошибок: 0, проблем: 0, рекомендаций: 0")
}

func TestProcessFileCheckFailurePreservesCauseAndJoinsOutputError(t *testing.T) {
	for _, brokenOutput := range []bool{false, true} {
		t.Run(fmt.Sprint(brokenOutput), func(t *testing.T) {
			cause, writeErr := errors.New("Check failed"), errors.New("output failed")
			path, original := runnerFile(t, processingModel(t))
			calls := 0
			first := runnerDefinition(func(_ diagnostics.Lines, f diagnostics.Findings) error {
				calls++
				_, err := f.Add(0, 0, 0, 0, "discarded")
				return err
			})
			failed := runnerDefinition(func(_ diagnostics.Lines, f diagnostics.Findings) error {
				calls++
				if _, err := f.Add(0, 0, 0, 0, "partial"); err != nil {
					return err
				}
				return cause
			})
			failed.Code = "V002"
			last := runnerDefinition(func(diagnostics.Lines, diagnostics.Findings) error { t.Error("Check ran after failure"); return nil })
			last.Code = "V003"
			var output bytes.Buffer
			errorOutput := &runnerWriter{}
			if brokenOutput {
				errorOutput.err = writeErr
			}
			r := runner{version: "dev", registry: runnerRegistry(t, last, failed, first), reporter: reporter.New(&output, errorOutput)}
			err := r.processFile(path)
			var failure *validator.CheckFailure
			if !errors.As(err, &failure) || failure.Code != "V002" || !errors.Is(err, cause) || calls != 2 {
				t.Fatalf("lost Check failure: %v, calls=%d", err, calls)
			}
			if errors.Is(err, writeErr) != brokenOutput || errorOutput.calls != 1 || output.Len() != 0 {
				t.Fatalf("wrong reporting behavior: %v", err)
			}
			if !brokenOutput && (!strings.Contains(errorOutput.String(), "V002") || !strings.Contains(errorOutput.String(), cause.Error())) {
				t.Fatal("report lost Check cause")
			}
			runnerUnchanged(t, path, original)
			runnerTotal(t, r.reporter, &output, 1, "всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0")
		})
	}
}

func TestProcessFileDoesNotOverwriteChangedSource(t *testing.T) {
	path, original := runnerFile(t, processingModel(t))
	changed := original + "\n"
	definition := runnerDefinition(func(_ diagnostics.Lines, f diagnostics.Findings) error {
		// Только тестовый callback имитирует внешнее изменение между Open и Save.
		if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := f.Add(0, 0, 0, 0, "accepted before Save")
		return err
	})
	var output, errorOutput bytes.Buffer
	r := runner{version: "dev", registry: runnerRegistry(t, definition), reporter: reporter.New(&output, &errorOutput)}
	if err := r.processFile(path); err == nil {
		t.Fatal("changed source was accepted")
	}
	runnerUnchanged(t, path, changed)
	if output.Len() != 0 || strings.Count(errorOutput.String(), "\n") != 1 || !strings.Contains(errorOutput.String(), "сохранён: нет, строк: 1, ошибок: 0, проблем: 1") {
		t.Fatalf("wrong save failure report: %q", errorOutput.String())
	}
	runnerTotal(t, r.reporter, &output, 1, "всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 0, проблем: 1, рекомендаций: 0")
}

func TestProcessFileReturnsOutputFailureAfterSuccessfulSave(t *testing.T) {
	path, _ := runnerFile(t, processingModel(t))
	writeErr := errors.New("output failed")
	output := &runnerWriter{err: writeErr}
	var errorOutput bytes.Buffer
	r := runner{version: "dev", registry: runnerRegistry(t, runnerDefinition(func(diagnostics.Lines, diagnostics.Findings) error { return nil })), reporter: reporter.New(output, &errorOutput)}
	if err := r.processFile(path); !errors.Is(err, writeErr) {
		t.Fatalf("lost output error: %v", err)
	}
	saved, _, err := storage.Open(path)
	if err != nil || len(saved.Processing) != 3 || output.calls != 1 || strings.Count(errorOutput.String(), "\n") != 1 {
		t.Fatalf("wrong save or report count: %v", err)
	}
	output.err = nil
	runnerTotal(t, r.reporter, &output.Buffer, 0, "всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 0, проблем: 0, рекомендаций: 0")
}

func runnerDefinition(check diagnostics.CheckFunc) diagnostics.Definition {
	return diagnostics.Definition{Code: "V001", Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeDocument, Message: "проверка", Versions: diagnostics.VersionRule{Mode: diagnostics.VersionAll}, Check: check}
}

func runnerRegistry(t *testing.T, definitions ...diagnostics.Definition) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(map[string]diagnostics.RegistrationFunc{"test": func(r *diagnostics.Registry) error {
		for _, definition := range definitions {
			if err := r.Register(definition); err != nil {
				return err
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func runnerFile(t *testing.T, result *model.Result) (string, string) {
	t.Helper()
	path, original := filepath.Join(t.TempDir(), "input.json"), processingSnapshot(t, result)
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	return path, original
}

func runnerUnchanged(t *testing.T, path, want string) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != want {
		t.Fatalf("file differs from expected contents: %v", err)
	}
}

func runnerTotal(t *testing.T, reports *reporter.Reporter, output *bytes.Buffer, wantCode int, want string) {
	t.Helper()
	output.Reset()
	code, err := reports.Finish()
	if code != wantCode || err != nil || output.String() != "ИТОГО: "+want+"\n" {
		t.Fatalf("Finish = %d, %v, %q; want %d, %q", code, err, output.String(), wantCode, want)
	}
}

type runnerWriter struct {
	bytes.Buffer
	err   error
	calls int
}

func (w *runnerWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.err != nil {
		return 0, w.err
	}
	return w.Buffer.Write(p)
}

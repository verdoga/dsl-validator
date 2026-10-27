package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/checkers"
	"github.com/verdoga/dsl-validator/console"
	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/storage"
	"github.com/verdoga/dsl-validator/validator"
)

func TestRunPrintsHelpOnceWithoutRequiringToolVersion(t *testing.T) {
	var expected bytes.Buffer
	if err := console.PrintHelp(&expected); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"--unknown", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, output, errorOutput := runCaptured(t, args, "", "", "")
			if code != 0 || output != expected.String() || errorOutput != "" {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
		})
	}
}

func TestRunReturnsUsageAndDoesNotRepeatArgumentErrors(t *testing.T) {
	root := t.TempDir()
	textPath := filepath.Join(root, "input.txt")
	if err := os.WriteFile(textPath, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		reason string
	}{
		{"missing argument", nil, "не указан исходный путь"},
		{"unknown flag", []string{"--unknown"}, "неизвестный параметр"},
		{"negative depth", []string{"--depth", "-1", root}, "значение --depth"},
		{"multiple paths", []string{root, root}, "ровно один исходный путь"},
		{"missing path", []string{filepath.Join(root, "missing.json")}, "не удалось проверить компонент пути"},
		{"wrong extension", []string{textPath}, "должен иметь расширение .json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, output, errorOutput := runCaptured(t, tc.args, "", "", "")
			if code != 2 || output != "" || strings.Count(errorOutput, "\n") != 1 || !strings.HasPrefix(errorOutput, "Ошибка: ") || !strings.Contains(errorOutput, tc.reason) {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
		})
	}
}

func TestRunRejectsEmptyToolVersionWithoutChangingJSON(t *testing.T) {
	path, original := runnerFile(t, processingModel(t))
	code, output, errorOutput := runCaptured(t, []string{path}, "", "", "")
	if code != 1 || output != "" || errorOutput != "Ошибка: версия инструмента не должна быть пустой\n" {
		t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
	}
	runnerUnchanged(t, path, original)
}

func TestRunProcessesJSONAndPreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(runValidJSON), 0600); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		previous, _, err := storage.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validator.CheckModel(previous); err != nil {
			t.Fatal(err)
		}
		code, output, errorOutput := runCaptured(t, []string{path}, "dev", "", "")
		if code != 0 || errorOutput != "" {
			t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
		}
		saved, _, err := storage.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validator.CheckModel(saved); err != nil {
			t.Fatal(err)
		}
		if len(saved.Processing) != len(previous.Processing)+1 || len(saved.Diagnostics) != len(previous.Diagnostics)+1 {
			t.Fatal("unexpected processing or diagnostic count")
		}
		current := saved.Processing[len(saved.Processing)-1]
		if current.Tool != "dsl-validator" || current.Version == nil || *current.Version != "dev" || current.StartedAt == nil || current.DurationMs == nil || *current.DurationMs < 0 {
			t.Fatalf("invalid processing entry: %+v", current)
		}
		// Каждый запуск сохраняет новую V201, не меняя прежние рекомендации.
		entry := saved.Diagnostics[len(previous.Diagnostics)]
		location := model.Location{Start: model.Position{Line: 3, Column: 3}, End: model.Position{Line: 3, Column: 9}}
		if entry.ID == "" || entry.Source != current.ID || entry.DiagnosticCode != "V201" ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "element" || entry.Fatal ||
			entry.Message != "Не все буквы заголовка первого уровня прописные." ||
			entry.Location == nil || *entry.Location != location || len(entry.RelatedLocations) != 0 {
			t.Fatalf("unexpected heading recommendation: %+v", entry)
		}
		oldLinks := previous.Lines[2].Elements[1].ErrorIDs
		wantLinks := append(append([]string{}, oldLinks...), entry.ID)
		if !slices.Equal(saved.Lines[2].Elements[1].ErrorIDs, wantLinks) {
			t.Fatalf("title links = %v, want %v", saved.Lines[2].Elements[1].ErrorIDs, wantLinks)
		}
		want := runReportStart(t) + fmt.Sprintf("ОБРАБОТАН %q, сохранён: да, строк: 4, ошибок: 0, проблем: 0, рекомендаций: 1, время: %d мс\n", path, *current.DurationMs) +
			"ИТОГО: всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 0, проблем: 0, рекомендаций: 1\n"
		if output != want {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
		saved.Processing = saved.Processing[:len(previous.Processing)]
		saved.Diagnostics = saved.Diagnostics[:len(previous.Diagnostics)]
		saved.Lines[2].Elements[1].ErrorIDs = saved.Lines[2].Elements[1].ErrorIDs[:len(oldLinks)]
		if processingSnapshot(t, saved) != processingSnapshot(t, previous) {
			t.Fatal("document, lines or previous history changed")
		}
	}
}

func TestRunReportsEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	code, output, errorOutput := runCaptured(t, []string{root}, "dev", "", "")
	want := runReportStart(t) + "ИТОГО: всего: 0, обработано: 0, сбоев: 0, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0\n"
	if code != 0 || output != want || errorOutput != "" {
		t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unexpected files after empty run: %v, %v", entries, err)
	}
}

func TestRunRejectsMalformedJSONAndStopsBeforeNextFile(t *testing.T) {
	for _, kind := range []string{"directory with JSON", "single invalid JSON"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := root
			broken := filepath.Join(root, "broken.json")
			files := map[string]string{broken: "{invalid JSON"}
			if kind == "single invalid JSON" {
				path = broken
			} else {
				files[filepath.Join(root, "valid.JSON")] = runValidJSON
			}
			for file, contents := range files {
				if err := os.WriteFile(file, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, output, errorOutput := runCaptured(t, []string{path}, "dev", "", "")
			want := runReportStart(t) + "ИТОГО: всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0\n"
			prefix := fmt.Sprintf("СБОЙ %q, сохранён: нет, строк: нет данных, ошибок: 0, проблем: 0, рекомендаций: 0, время: ", broken)
			if code != 1 || output != want || strings.Count(errorOutput, "\n") != 1 || !strings.HasPrefix(errorOutput, prefix) || !strings.Contains(errorOutput, "ожидался ровно один корректный JSON-объект") {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
			for file, contents := range files {
				runnerUnchanged(t, file, contents)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != len(files) {
				t.Fatalf("unexpected files after refusal: %v, %v", entries, err)
			}
		})
	}
}

func TestRunTreatsOutputFailuresAsTechnicalFailures(t *testing.T) {
	root := t.TempDir()
	for _, mode := range []string{"closed", "read-only"} {
		for _, tc := range []struct {
			name    string
			args    []string
			version string
			stdout  bool
		}{
			{"help", []string{"--help"}, "dev", true},
			{"arguments", nil, "dev", false},
			{"source path", []string{filepath.Join(root, "missing.json")}, "dev", false},
			{"configuration", []string{root}, "", false},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				stdoutMode, stderrMode := "", mode
				if tc.stdout {
					stdoutMode, stderrMode = mode, ""
				}
				code, output, errorOutput := runCaptured(t, tc.args, tc.version, stdoutMode, stderrMode)
				if code != 1 || output != "" || errorOutput != "" {
					t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
				}
			})
		}
		t.Run(mode+"/report start", func(t *testing.T) {
			code, output, errorOutput := runCaptured(t, []string{root}, "dev", mode, "")
			if code != 1 || output != "" || !strings.HasPrefix(errorOutput, "Ошибка: ") || strings.Count(errorOutput, "\n") != 1 {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
		})
		t.Run(mode+"/file failure report", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "broken.json")
			if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
			code, output, errorOutput := runCaptured(t, []string{path}, "dev", "", mode)
			want := runReportStart(t) + "ИТОГО: всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0\n"
			if code != 1 || output != want || errorOutput != "" {
				t.Fatalf("Run = %d, stdout=%q, stderr=%q", code, output, errorOutput)
			}
			runnerUnchanged(t, path, "{")
		})
	}
}

// runReportStart задаёт стартовый вывод dev-сборки без запуска Check.
func runReportStart(t *testing.T) string {
	t.Helper()
	registry, err := diagnostics.NewRegistry(checkers.RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("dsl-validator dev\nЗарегистрировано диагностик: %d\n", registry.Len())
}

// runValidJSON — полный документ с обычным текстом и историей разбора.
const runValidJSON = `{
  "formatVersion": "1.0",
  "document": {
    "dslVersion": "1.2", "fileName": "input.dsl", "filePath": null,
    "encoding": "UTF-8", "hasBom": false, "lineCount": 4, "byteLength": null, "sha256": null, "hasErrors": false,
    "metadata": {"documentId": "unit-1", "title": "Unit 1", "subtitle": null, "section": null, "order": null, "resourceDirs": []}
  },
  "processing": [{"id": "p1", "tool": "dsl-parser", "version": null, "startedAt": null, "durationMs": null}],
  "lines": [
    {"line": 1, "type": "tag", "nestingLevel": 0, "parentLine": null, "raw": "@dsl-version 1.2", "eol": "\n", "hasErrors": false,
     "elements": [{"type": "tag", "raw": "@dsl-version", "value": "dsl-version", "start": 1, "end": 13, "errorIds": []}, {"type": "version", "raw": "1.2", "value": "1.2", "start": 14, "end": 17, "errorIds": []}]},
    {"line": 2, "type": "tag", "nestingLevel": 0, "parentLine": null, "raw": "@document-id unit-1", "eol": "\n", "hasErrors": false,
     "elements": [{"type": "tag", "raw": "@document-id", "value": "document-id", "start": 1, "end": 13, "errorIds": []}, {"type": "identifier", "raw": "unit-1", "value": "unit-1", "start": 14, "end": 20, "errorIds": []}]},
    {"line": 3, "type": "heading", "nestingLevel": 0, "parentLine": null, "raw": "# Unit 1", "eol": "\n", "hasErrors": false,
     "elements": [{"type": "heading-level", "raw": "#", "value": "1", "start": 1, "end": 2, "errorIds": []}, {"type": "title", "raw": "Unit 1", "value": "Unit 1", "start": 3, "end": 9, "errorIds": []}]},
    {"line": 4, "type": "content", "nestingLevel": 1, "parentLine": 3, "raw": "Read this sentence.", "eol": "", "hasErrors": false,
     "elements": [{"type": "content", "raw": "Read this sentence.", "value": "Read this sentence.", "start": 1, "end": 20, "errorIds": []}]}
  ],
  "diagnostics": []
}`

// runCaptured подменяет потоки только на время Run. Тесты с ним не параллельны;
// временные файлы позволяют читать вывод без горутин и блокировки каналов.
func runCaptured(t *testing.T, args []string, version, stdoutMode, stderrMode string) (int, string, string) {
	t.Helper()
	root := t.TempDir()
	var streams [2]*os.File
	for i, mode := range []string{stdoutMode, stderrMode} {
		name := "stdout"
		if i == 1 {
			name = "stderr"
		}
		flags := os.O_CREATE | os.O_RDWR
		if mode == "read-only" {
			flags = os.O_CREATE | os.O_RDONLY
		}
		file, err := os.OpenFile(filepath.Join(root, name), flags, 0600)
		if err != nil {
			t.Fatal(err)
		}
		streams[i] = file
		t.Cleanup(func() {
			if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		})
		if mode == "closed" {
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	previousOutput, previousErrorOutput := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = streams[0], streams[1]
	defer func() { os.Stdout, os.Stderr = previousOutput, previousErrorOutput }()
	code := Run(args, version)
	var captured [2]string
	for i, file := range streams {
		contents, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		captured[i] = string(contents)
	}
	return code, captured[0], captured[1]
}

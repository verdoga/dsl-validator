package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/verdoga/dsl-validator/checkers"
	"github.com/verdoga/dsl-validator/console"
	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/storage"
	"github.com/verdoga/dsl-validator/validator"
)

func TestVersionDefaultsToDev(t *testing.T) {
	if version != "dev" {
		t.Fatalf("version = %q, want dev", version)
	}
}

func TestMainPreservesApplicationResults(t *testing.T) {
	var help bytes.Buffer
	if err := console.PrintHelp(&help); err != nil {
		t.Fatal(err)
	}
	goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goTool += ".exe"
	}
	for _, build := range []struct {
		name    string
		flags   []string
		version string
	}{
		{"default", nil, "dev"},
		{"release", []string{"-ldflags", "-X main.version=1.0.0"}, "1.0.0"},
		{"empty version", []string{"-ldflags", "-X main.version="}, ""},
	} {
		t.Run(build.name, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "dsl-validator")
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			args := append([]string{"build"}, build.flags...)
			args = append(args, "-o", binary, ".")
			command := exec.CommandContext(t.Context(), goTool, args...)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, output)
			}
			fileCode, fileError := 0, ""
			if build.version == "" {
				fileCode, fileError = 1, "Ошибка: версия инструмента не должна быть пустой\n"
			}
			for _, tc := range []struct {
				name    string
				args    []string
				code    int
				stdout  string
				stderr  string
				process bool
			}{
				{"help", []string{"--help"}, 0, help.String(), "", false},
				{"short help", []string{"-h"}, 0, help.String(), "", false},
				{"help after unknown flag", []string{"--unknown", "--help"}, 0, help.String(), "", false},
				{"no arguments", nil, 2, "", "Ошибка: не указан исходный путь\n", false},
				{"unknown flag", []string{"--unknown"}, 2, "", "Ошибка: неизвестный параметр \"--unknown\"\n", false},
				{"source file", []string{"input file.json"}, fileCode, "", fileError, true},
				{"depth and source file", []string{"--depth", "0", "input file.json"}, fileCode, "", fileError, true},
				{"separator and source file", []string{"--", "input file.json"}, fileCode, "", fileError, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "input file.json")
					if err := os.WriteFile(path, []byte(mainValidJSON), 0600); err != nil {
						t.Fatal(err)
					}
					previous, _, err := storage.Open(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := validator.CheckModel(previous); err != nil {
						t.Fatal(err)
					}
					command := exec.CommandContext(t.Context(), binary, tc.args...)
					command.Dir = root
					var stdout, stderr bytes.Buffer
					command.Stdout = &stdout
					command.Stderr = &stderr
					code := 0
					if err := command.Run(); err != nil {
						var exitErr *exec.ExitError
						if !errors.As(err, &exitErr) {
							t.Fatal(err)
						}
						code = exitErr.ExitCode()
					}
					if code != tc.code || stderr.String() != tc.stderr {
						t.Fatalf("process = (%d, %q, %q), want code %d and stderr %q",
							code, stdout.String(), stderr.String(), tc.code, tc.stderr)
					}
					wantOutput := tc.stdout
					if tc.process && build.version != "" {
						wantOutput = mainSavedReport(t, path, previous, build.version)
					} else {
						contents, err := os.ReadFile(path)
						if err != nil || string(contents) != mainValidJSON {
							t.Fatalf("input JSON changed or unreadable: %v", err)
						}
					}
					if stdout.String() != wantOutput {
						t.Fatalf("stdout = %q, want %q", stdout.String(), wantOutput)
					}
				})
			}
		})
	}
}

// mainSavedReport проверяет сохранённую модель и задаёт ожидаемый отчёт;
// длительность берётся из проверенной записи запуска, так как зависит от часов.
func mainSavedReport(t *testing.T, path string, previous *model.Result, version string) string {
	t.Helper()
	saved, _, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validator.CheckModel(saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Processing) != len(previous.Processing)+1 || len(saved.Diagnostics) != 0 {
		t.Fatal("unexpected processing or diagnostic count")
	}
	current := saved.Processing[len(saved.Processing)-1]
	if current.Tool != "dsl-validator" || current.Version == nil || *current.Version != version || current.StartedAt == nil || current.DurationMs == nil || *current.DurationMs < 0 {
		t.Fatalf("invalid processing entry: %+v", current)
	}
	saved.Processing = saved.Processing[:len(previous.Processing)]
	before, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("document, lines or previous history changed")
	}
	registry, err := diagnostics.NewRegistry(checkers.RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	banner := "dsl-validator dev\n"
	if version == "1.0.0" {
		banner = "dsl-validator v1.0.0\n"
	}
	return banner + fmt.Sprintf("Зарегистрировано диагностик: %d\nОБРАБОТАН %q, сохранён: да, строк: 4, ошибок: 0, проблем: 0, рекомендаций: 0, время: %d мс\n", registry.Len(), path, *current.DurationMs) +
		"ИТОГО: всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 0, проблем: 0, рекомендаций: 0\n"
}

// mainValidJSON — полный документ с обычным текстом и историей разбора.
const mainValidJSON = `{
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

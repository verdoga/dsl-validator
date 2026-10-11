package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/console"
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

func TestRunRejectsEmptyRegistryBeforeOpeningOrReportingJSON(t *testing.T) {
	for _, kind := range []string{"empty directory", "directory with JSON", "single invalid JSON"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := root
			files := map[string]string{}
			if kind != "empty directory" {
				files[filepath.Join(root, "broken.json")] = "{invalid JSON"
				if kind == "single invalid JSON" {
					path = filepath.Join(root, "broken.json")
				} else {
					files[filepath.Join(root, "valid.JSON")] = processingSnapshot(t, processingModel(t))
				}
			}
			for file, contents := range files {
				if err := os.WriteFile(file, []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, output, errorOutput := runCaptured(t, []string{path}, "dev", "", "")
			if code != 1 || output != "" || errorOutput != "Ошибка: диагностики не зарегистрированы\n" {
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
			{"empty registry", []string{root}, "dev", false},
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
	}
}

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

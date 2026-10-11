package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/verdoga/dsl-validator/console"
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
		failure string
	}{
		{"default", nil, "Ошибка: диагностики не зарегистрированы\n"},
		{"release", []string{"-ldflags", "-X main.version=1.0.0"}, "Ошибка: диагностики не зарегистрированы\n"},
		{"empty version", []string{"-ldflags", "-X main.version="}, "Ошибка: версия инструмента не должна быть пустой\n"},
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
			for _, tc := range []struct {
				name   string
				args   []string
				code   int
				stdout string
				stderr string
			}{
				{"help", []string{"--help"}, 0, help.String(), ""},
				{"short help", []string{"-h"}, 0, help.String(), ""},
				{"help after unknown flag", []string{"--unknown", "--help"}, 0, help.String(), ""},
				{"no arguments", nil, 2, "", "Ошибка: не указан исходный путь\n"},
				{"unknown flag", []string{"--unknown"}, 2, "", "Ошибка: неизвестный параметр \"--unknown\"\n"},
				{"source file", []string{"input file.json"}, 1, "", build.failure},
				{"depth and source file", []string{"--depth", "0", "input file.json"}, 1, "", build.failure},
				{"separator and source file", []string{"--", "input file.json"}, 1, "", build.failure},
			} {
				t.Run(tc.name, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "input file.json")
					const original = "{\"untouched\": true}\n"
					if err := os.WriteFile(path, []byte(original), 0600); err != nil {
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
					if code != tc.code || stdout.String() != tc.stdout || stderr.String() != tc.stderr {
						t.Fatalf("process = (%d, %q, %q), want (%d, %q, %q)",
							code, stdout.String(), stderr.String(), tc.code, tc.stdout, tc.stderr)
					}
					contents, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if string(contents) != original {
						t.Fatalf("input JSON changed: %q", contents)
					}
				})
			}
		})
	}
}

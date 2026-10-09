package console

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParseReturnsValidatedParams(t *testing.T) {
	root := pathTestTree(t)
	t.Chdir(root)
	for _, name := range []string{"--help", "-h", "-"} {
		if err := os.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name  string
		args  []string
		path  string
		depth int
	}{
		{"directory", []string{"sub"}, "sub", -1},
		{"absolute", []string{root}, "", -1},
		{"file", []string{"sample.json"}, "sample.json", -1},
		{"uppercase_extension", []string{"upper.JSON"}, "upper.JSON", -1},
		{"empty_path", []string{""}, "", -1},
		{"zero_depth", []string{"--depth", "0", "."}, "", 0},
		{"positive_depth", []string{"--depth", "2", "sub"}, "sub", 2},
		{"file_with_depth", []string{"--depth", "3", "sample.json"}, "sample.json", 3},
		{"unclean_path", []string{"sub//nested/../../sample.json"}, "sample.json", -1},
		{"separator", []string{"--", "sample.json"}, "sample.json", -1},
		{"help_named_path", []string{"--", "--help"}, "--help", -1},
		{"short_help_named_path", []string{"--depth", "1", "--", "-h"}, "-h", 1},
		{"single_dash_path", []string{"-"}, "-", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			params, proceed, err := Parse(tt.args, &stdout, &stderr)
			if err != nil || !proceed {
				t.Fatalf("Parse = %v, %v, %v; stderr=%q", params, proceed, err, stderr.String())
			}
			if want := filepath.Join(root, tt.path); params.Path != want {
				t.Errorf("Path = %q, want %q", params.Path, want)
			}
			if tt.depth < 0 {
				if params.Depth != nil {
					t.Errorf("Depth = %v, want nil", params.Depth)
				}
			} else if params.Depth == nil || *params.Depth != tt.depth {
				t.Errorf("Depth = %v, want %d", params.Depth, tt.depth)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("successful parse produced output: stdout=%q, stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestParseRejectsArgumentsAndSourcePaths(t *testing.T) {
	root := pathTestTree(t)
	t.Chdir(root)
	if err := os.Symlink(filepath.Join(root, "sub"), "link"); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		args []string
		hint string
	}{
		{"no_path", nil, "не указан"},
		{"multiple_paths", []string{".", "sub"}, "ровно один"},
		{"flags_after_path", []string{".", "--depth", "1"}, "ровно один"},
		{"help_after_path", []string{".", "--help"}, "ровно один"},
		{"help_after_empty_path", []string{"", "-h"}, "ровно один"},
		{"help_after_separator", []string{"--", ".", "--help"}, "ровно один"},
		{"depth_missing", []string{"--depth"}, "требуется значение"},
		{"depth_before_separator", []string{"--depth", "--", "--help"}, "требуется значение"},
		{"depth_empty", []string{"--depth", "", "."}, "целым числом"},
		{"depth_invalid", []string{"--depth", "abc", "."}, "целым числом"},
		{"depth_negative_for_file", []string{"--depth", "-1", "sample.json"}, "целым числом"},
		{"depth_overflow", []string{"--depth", strconv.Itoa(int(^uint(0)>>1)) + "0", "."}, "целым числом"},
		{"depth_without_path", []string{"--depth", "1"}, "не указан"},
		{"separator_without_path", []string{"--"}, "не указан"},
		{"depth_equals", []string{"--depth=1", "."}, "неизвестный параметр"},
		{"depth_single_dash", []string{"-depth", "1", "."}, "неизвестный параметр"},
		{"help_single_dash", []string{"-help"}, "неизвестный параметр"},
		{"h_double_dash", []string{"--h"}, "неизвестный параметр"},
		{"help_equals", []string{"--help=true"}, "неизвестный параметр"},
		{"h_equals", []string{"-h=true"}, "неизвестный параметр"},
		{"replace", []string{"--replace", "."}, "неизвестный параметр"},
		{"unknown", []string{"--unknown", "."}, "неизвестный параметр"},
		{"repeated_depth", []string{"--depth", "1", "--depth", "2", "."}, "повторно"},
		{"missing_source", []string{"missing"}, "компонент пути"},
		{"wrong_extension", []string{"notes.txt"}, "расширение .json"},
		{"symlink_before_parent", []string{"link/../sample.json"}, "символическая ссылка"},
		{"missing_before_parent", []string{"missing/../sample.json"}, "компонент пути"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, proceed, err := Parse(tt.args, &stdout, &stderr)
			if err == nil || proceed {
				t.Fatalf("Parse accepted invalid input: proceed=%v, err=%v", proceed, err)
			}
			if !strings.Contains(err.Error(), tt.hint) {
				t.Errorf("error %q does not explain %q", err, tt.hint)
			}
			if stdout.Len() != 0 || strings.Count(stderr.String(), err.Error()) != 1 {
				t.Errorf("expected error once in stderr: stdout=%q, stderr=%q", stdout.String(), stderr.String())
			}
			if tt.name == "missing_source" && !errors.Is(err, os.ErrNotExist) {
				t.Errorf("filesystem cause lost: %v", err)
			}
		})
	}
}

func TestParseHelpOverridesArgumentErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	var expected bytes.Buffer
	if err := PrintHelp(&expected); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--help"}, {"-h"}, {"--help", missing}, {"--help", ".", "extra"},
		{"--depth", "abc", "--help"}, {"--depth", "-1", "-h"},
		{"--depth", "", "--help"}, {"--depth", "--help"},
		{"--replace", "-h"}, {"--help", "--replace"},
		{"--depth=bad", "--help"}, {"--unknown", "--help"},
		{"--depth", "1", "--depth", "2", "--help"},
		{"--depth", "--depth", "1", "--help"},
		{"--help", "--", missing},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, proceed, err := Parse(args, &stdout, &stderr)
			if err != nil || proceed || stderr.Len() != 0 || stdout.String() != expected.String() {
				t.Fatalf("help: proceed=%v, err=%v, stdout=%q, stderr=%q", proceed, err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestParseDepthChecksIntBounds(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, want := range []int{0, 7, maxInt} {
		t.Run(strconv.Itoa(want), func(t *testing.T) {
			if got, err := parseDepth(strconv.Itoa(want)); err != nil || got != want {
				t.Fatalf("parseDepth = %d, %v; want %d", got, err, want)
			}
		})
	}
	for _, raw := range []string{"", "abc", "-1", "1.5", " 1", "1 ", strconv.FormatUint(uint64(maxInt)+1, 10)} {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseDepth(raw); err == nil {
				t.Fatalf("parseDepth(%q) succeeded", raw)
			}
		})
	}
}

func TestParseKeepsCallsIndependent(t *testing.T) {
	root := t.TempDir()
	first, proceed, err := Parse([]string{"--depth", "2", root}, io.Discard, io.Discard)
	if err != nil || !proceed || first.Depth == nil {
		t.Fatalf("first call: %v, %v, %v", first, proceed, err)
	}
	second, proceed, err := Parse([]string{"--depth", "5", root}, io.Discard, io.Discard)
	if err != nil || !proceed || second.Depth == nil {
		t.Fatalf("second call: %v, %v, %v", second, proceed, err)
	}
	last, proceed, err := Parse([]string{root}, io.Discard, io.Discard)
	if err != nil || !proceed || last.Depth != nil || *first.Depth != 2 || *second.Depth != 5 {
		t.Fatalf("state leaked across calls: first=%v, second=%v, last=%v, err=%v", first, second, last, err)
	}
}

func TestParseReturnsOutputErrorsWithoutReprinting(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		help bool
	}{
		{"help_output", []string{"--help"}, true},
		{"argument_error_output", nil, false},
		{"path_error_output", []string{filepath.Join(t.TempDir(), "missing")}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := errors.New("запись недоступна")
			broken := &parseErrorWriter{err: want}
			var other bytes.Buffer
			var stdout, stderr io.Writer = &other, broken
			if tt.help {
				stdout, stderr = broken, &other
			}
			_, proceed, err := Parse(tt.args, stdout, stderr)
			if proceed || !errors.Is(err, want) || broken.writes != 1 || other.Len() != 0 {
				t.Fatalf("output failure: proceed=%v, err=%v, writes=%d, other=%q", proceed, err, broken.writes, other.String())
			}
		})
	}
}

type parseErrorWriter struct {
	err    error
	writes int
}

func (w *parseErrorWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, w.err
}

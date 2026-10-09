package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestCheckJSONSyntax(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"empty_object", `{}`, true},
		{"whitespace", " \t\r\n{}\r\n\t ", true},
		{"unicode", `{"я":"🌍\uD83C\uDF0D\ufffd","escaped":"\\uD800"}`, true},
		{"nested", `{"a":[{"x":1},{"x":2}],"b":{"x":3}}`, true},
		{"large_unknown_number", `{"n":1e9999}`, true},
		{"empty", ``, false}, {"array", `[]`, false}, {"null", `null`, false},
		{"scalar", `"object"`, false}, {"bom", "\xef\xbb\xbf{}", false},
		{"comment", `{"x":1/* comment */}`, false}, {"line_comment", "{}//comment", false},
		{"second_object", `{} {}`, false}, {"second_value", `{} null`, false},
		{"trailing_text", `{}x`, false}, {"trailing_comma", `{"x":1,}`, false},
		{"non_json_space", "{}\u00a0", false}, {"bad_utf8", "{\"x\":\"\xff\"}", false},
		{"utf8_surrogate", "{\"x\":\"\xed\xa0\x80\"}", false},
		{"high_surrogate", `{"x":"\uD800"}`, false}, {"low_surrogate", `{"x":"\uDC00"}`, false},
		{"wrong_pair", `{"x":"\uD800\u0041"}`, false},
		{"reversed_pair", `{"x":"\uDC00\uD800"}`, false},
		{"escaped_second", `{"x":"\uD800\\uDC00"}`, false},
		{"bad_escape", `{"x":"\uZZZZ"}`, false}, {"truncated_escape", `{"x":"\u12`, false},
		{"surrogate_key", `{"\uD800":1}`, false},
		{"duplicate", `{"x":1,"x":2}`, false},
		{"escaped_duplicate", `{"x":1,"\u0078":2}`, false},
		{"surrogate_duplicate", `{"🌍":1,"\uD83C\uDF0D":2}`, false},
		{"unknown_nested_duplicate", `{"extra":[{"x":1,"x":2}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkJSONSyntax([]byte(tc.source)); (err == nil) != tc.valid {
				t.Fatalf("checkJSONSyntax = %v; want accepted=%v", err, tc.valid)
			}
		})
	}
}

func TestDecodeResultAcceptsShapesWithoutSemanticValidation(t *testing.T) {
	for _, tc := range []struct{ path, replacement string }{
		{"", ""}, {"document/dslVersion", `null`}, {"document/dslVersion", `"unknown-version"`},
		{"processing", `[]`}, {"lines", `[]`}, {"diagnostics", `[]`},
		{"document/metadata/resourceDirs", `null`}, {"document/metadata/resourceDirs", `["ресурсы",""]`},
		{"lines/0/elements", `[]`}, {"lines/0/parentLine", `999`},
		{"lines/0/elements/0/value", `null`}, {"lines/0/elements/0/errorIds", `["missing"]`},
		{"diagnostics/0/source", `"missing"`}, {"diagnostics/0/location", `null`},
		{"diagnostics/0/location/start/column", `-1`}, {"document/hasErrors", `false`},
		{"lines/0/type", `"unknown-type"`}, {"diagnostics/0/severity", `"unknown-severity"`},
		{"extra", `{"nested":[null,{},1e9999]}`},
		{"document/HasErrors", `{"ignored":true}`}, {"FormatVersion", `false`},
		{"lines/0/RAW", `null`}, {"lines/0/elements/0/Value", `[]`},
		{"processing/0/ID", `false`}, {"diagnostics/0/Fatal", `"ignored"`},
		{"document/metadata/Title", `123`}, {"diagnostics/0/location/extra", `42`},
		{"diagnostics/0/location/Start", `false`}, {"diagnostics/0/location/start/Line", `false`},
		{"diagnostics/0/relatedLocations/0/start/Column", `false`},
	} {
		t.Run(tc.path+"="+tc.replacement, func(t *testing.T) {
			source := readTestJSON()
			if tc.path != "" {
				source = readTestChange(t, source, tc.path, tc.replacement)
			}
			before := bytes.Clone(source)
			result, err := decodeResult(source)
			if err != nil || result == nil {
				t.Fatalf("decodeResult = %v, %v", result, err)
			}
			if !bytes.Equal(before, source) {
				t.Fatal("decodeResult changed source bytes")
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			// После удаления неизвестных полей ожидаем ровно известную часть входа.
			var expected, actual any
			known := source
			field := tc.path[strings.LastIndex(tc.path, "/")+1:]
			if field == "extra" || len(field) > 0 && field[0] >= 'A' && field[0] <= 'Z' {
				known = readTestJSON()
			}
			if err := json.Unmarshal(known, &expected); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(expected)
			got, _ := json.Marshal(actual)
			if !bytes.Equal(got, want) {
				t.Fatalf("decoded values changed: got %s; want %s", got, want)
			}
		})
	}
}

func TestDecodeResultRejectsMissingFieldsAndWrongTypes(t *testing.T) {
	// Суффикс ? обозначает единственные поля, для которых null разрешён.
	groups := map[string]string{
		"":                        "formatVersion document processing lines diagnostics",
		"document/":               "dslVersion? fileName? filePath? encoding? hasBom? lineCount? byteLength? sha256? metadata hasErrors",
		"document/metadata/":      "documentId? title? subtitle? section? order? resourceDirs?",
		"processing/0/":           "id tool version? startedAt? durationMs?",
		"lines/0/":                "line type nestingLevel parentLine? raw eol hasErrors elements",
		"lines/0/elements/0/":     "type raw value? start end errorIds",
		"diagnostics/0/":          "id source code severity message scope fatal location? relatedLocations",
		"diagnostics/0/location/": "start end", "diagnostics/0/location/start/": "line column",
		"diagnostics/0/location/end/": "line column", "diagnostics/0/relatedLocations/0/": "start end",
		"diagnostics/0/relatedLocations/0/start/": "line column", "diagnostics/0/relatedLocations/0/end/": "line column",
	}
	for prefix, fields := range groups {
		for _, field := range strings.Fields(fields) {
			path := prefix + strings.TrimSuffix(field, "?")
			for _, replacement := range []string{"", "null", `0.5`} {
				t.Run(path+"="+replacement, func(t *testing.T) {
					result, err := decodeResult(readTestChange(t, readTestJSON(), path, replacement))
					allowed := replacement == "null" && strings.HasSuffix(field, "?")
					if allowed && (err != nil || result == nil) || !allowed && (err == nil || result != nil) {
						t.Fatalf("decodeResult = %v, %v; want accepted=%v", result, err, allowed)
					}
				})
			}
		}
	}
	for _, tc := range []struct{ path, replacement string }{
		{"formatVersion", `"2.0"`}, {"formatVersion", `1`}, {"formatVersion", `[]`},
		{"document/lineCount", `9223372036854775808`}, {"document/byteLength", `1e1`},
		{"processing/0", `null`}, {"lines/0", `null`}, {"lines/0/elements/0", `null`},
		{"diagnostics/0", `null`}, {"diagnostics/0/relatedLocations/0", `null`},
		{"document/metadata/resourceDirs", `[null]`}, {"lines/0/elements/0/errorIds", `[null]`},
		{"document/metadata/resourceDirs", `[false]`}, {"lines/0/elements/0/errorIds", `[{}]`},
		{"document/hasErrors", `"false"`}, {"lines/0/raw", `true`},
		{"lines/0/elements", `{}`}, {"diagnostics/0/location", `[]`},
	} {
		t.Run(tc.path+"="+tc.replacement, func(t *testing.T) {
			if result, err := decodeResult(readTestChange(t, readTestJSON(), tc.path, tc.replacement)); err == nil || result != nil {
				t.Fatalf("decodeResult = %v, %v; want nil, error", result, err)
			}
		})
	}
}

func TestDecodeResultRejectsSyntaxErrorsInCompleteDocument(t *testing.T) {
	valid := string(readTestJSON())
	for name, source := range map[string]string{
		"bom":                     "\xef\xbb\xbf" + valid,
		"trailing_object":         valid + `{}`,
		"duplicate_known":         strings.Replace(valid, `"formatVersion":"1.0"`, `"formatVersion":"1.0","formatVersion":"1.0"`, 1),
		"duplicate_unknown":       strings.Replace(valid, "{", `{"extra":{"x":1,"\u0078":2},`, 1),
		"invalid_unicode_unknown": strings.Replace(valid, "{", `{"extra":"\uD800",`, 1),
		"invalid_utf8_unknown":    strings.Replace(valid, "{", "{\"extra\":\"\xff\",", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if result, err := decodeResult([]byte(source)); err == nil || result != nil {
				t.Fatalf("decodeResult = %v, %v; want nil, error", result, err)
			}
		})
	}
}

func TestReadFileReturnsExactBytesAndClosesFile(t *testing.T) {
	dir := t.TempDir()
	for _, source := range [][]byte{{}, bytes.Repeat([]byte("\x00\xff\r\n"), 65536), readTestJSON()} {
		path := filepath.Join(dir, "source.json")
		if err := os.WriteFile(path, source, 0600); err != nil {
			t.Fatal(err)
		}
		wantInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		got, info, err := readFile(path)
		if err != nil || info == nil || !bytes.Equal(got, source) || !os.SameFile(info, wantInfo) || info.Size() != int64(len(source)) {
			t.Fatalf("readFile = %d bytes, %v, %v", len(got), info, err)
		}
		if runtime.GOOS == "linux" {
			entries, err := os.ReadDir("/proc/self/fd")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				opened, err := os.Stat(filepath.Join("/proc/self/fd", entry.Name()))
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil || os.SameFile(info, opened) {
					t.Fatalf("descriptor %s remains open or cannot be checked: %v", entry.Name(), err)
				}
			}
		}
		if len(got) > 0 {
			got[0] ^= 0xff
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, source) {
			t.Fatalf("readFile changed the source file: %v", err)
		}
	}
}

func TestReadFileRejectsUnsuitablePaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "source.json"), filepath.Join(dir, "file-link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "missing", "pipe", "file-link", "link/source.json", "link/../source.json"} {
		t.Run(name, func(t *testing.T) {
			source, info, err := readFile(dir + "/" + name)
			if err == nil || source != nil || info != nil {
				t.Fatalf("readFile = %v, %v, %v; want nil, nil, error", source, info, err)
			}
			if name == "missing" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("lost filesystem error: %v", err)
			}
		})
	}
}

func readTestJSON() []byte {
	return []byte(`{
	"formatVersion":"1.0",
	"document":{"dslVersion":"1.2","fileName":null,"filePath":null,"encoding":null,
	"hasBom":null,"lineCount":1,"byteLength":null,"sha256":null,"hasErrors":true,
	"metadata":{"documentId":null,"title":null,"subtitle":null,"section":null,"order":null,"resourceDirs":[]}},
	"processing":[{"id":"p1","tool":"parser","version":null,"startedAt":null,"durationMs":null}],
	"lines":[{"line":1,"type":"content","nestingLevel":0,"parentLine":null,"raw":"x","eol":"","hasErrors":true,
	"elements":[{"type":"content","raw":"x","value":"x","start":1,"end":2,"errorIds":["d1"]}]}],
	"diagnostics":[{"id":"d1","source":"p1","code":"P001","severity":"error","message":"ошибка","scope":"element","fatal":true,
	"location":{"start":{"line":1,"column":1},"end":{"line":1,"column":2}},
	"relatedLocations":[{"start":{"line":1,"column":2},"end":{"line":1,"column":2}}]}]
}`)
}

func readTestChange(t *testing.T, source []byte, path, replacement string) []byte {
	t.Helper()
	var root, changed any
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		t.Fatal(err)
	}
	if replacement != "" {
		decoder = json.NewDecoder(strings.NewReader(replacement))
		decoder.UseNumber()
		if err := decoder.Decode(&changed); err != nil {
			t.Fatal(err)
		}
	}
	parts := strings.Split(path, "/")
	parent := root
	for i, part := range parts {
		last := i == len(parts)-1
		switch container := parent.(type) {
		case map[string]any:
			if last {
				if replacement == "" {
					delete(container, part)
				} else {
					container[part] = changed
				}
			}
			parent = container[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(container) {
				t.Fatalf("invalid fixture index %q", part)
			}
			if last {
				container[index] = changed
			}
			parent = container[index]
		default:
			t.Fatalf("invalid fixture path %q at %q", path, part)
		}
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

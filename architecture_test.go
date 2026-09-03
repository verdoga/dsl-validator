package dslparser_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestParserDependencyBoundary закрепляет единственную допустимую границу внешнего парсера.
func TestParserDependencyBoundary(t *testing.T) {
	err := filepath.WalkDir(".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(name) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			slashName := strings.TrimPrefix(filepath.ToSlash(name), "./")
			if path == "github.com/verdoga/dslparser" && !strings.HasPrefix(slashName, "internal/parseradapter/") {
				t.Errorf("%s импортирует внешний парсер вне адаптера", name)
			}
			if strings.HasPrefix(slashName, "Diagnostics/") && (path == "github.com/verdoga/dslparser" || path == "dslparser/internal/parseradapter") {
				t.Errorf("%s нарушает границу Diagnostics", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

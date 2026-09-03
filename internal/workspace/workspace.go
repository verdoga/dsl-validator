package workspace

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File описывает найденный сценарий без сохранения AST.
type File struct {
	Path         string
	Name         string
	Size         int64
	Version      string
	VersionKnown bool
	ID           string
}

// Issue описывает восстановимую ошибку сканирования.
type Issue struct {
	Code    string
	Path    string
	Message string
}

// Scan содержит отсортированные сценарии и ошибки.
type Scan struct {
	Root   string
	Files  []File
	Issues []Issue
}

// VersionReader определяет версию только через подключённый parser adapter.
type VersionReader interface {
	Version(path string) (string, bool, error)
}

// ValidateRoot проверяет абсолютный обычный читаемый каталог.
func ValidateRoot(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("WORKDIR_NOT_ABSOLUTE")
	}
	clean := filepath.Clean(path)
	info, err := os.Lstat(clean)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("WORKDIR_NOT_FOUND")
	}
	if err != nil {
		return "", fmt.Errorf("WORKDIR_ACCESS_FAILURE: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("WORKDIR_REPARSE_POINT")
	}
	if !info.IsDir() {
		return "", fmt.Errorf("WORKDIR_NOT_DIRECTORY")
	}
	f, err := os.Open(clean)
	if err != nil {
		return "", fmt.Errorf("WORKDIR_ACCESS_FAILURE: %w", err)
	}
	if _, err = f.Readdirnames(1); err != nil && err.Error() != "EOF" {
		f.Close()
		return "", fmt.Errorf("WORKDIR_ACCESS_FAILURE: %w", err)
	}
	f.Close()
	return clean, nil
}

// Scanner выполняет последовательный рекурсивный обход.
type Scanner struct{ versions VersionReader }

// NewScanner создаёт scanner с реальным источником версий.
func NewScanner(versions VersionReader) Scanner { return Scanner{versions: versions} }

// Scan обнаруживает txt без учёта регистра и не переходит по symlink.
func (s Scanner) Scan(root string) (Scan, error) {
	clean, err := ValidateRoot(root)
	if err != nil {
		return Scan{}, err
	}
	result := Scan{Root: clean, Files: []File{}, Issues: []Issue{}}
	err = filepath.WalkDir(clean, func(path string, e fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Issues = append(result.Issues, Issue{"SCAN_ENTRY_FAILURE", path, walkErr.Error()})
			if e != nil && e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path != clean && e.Type()&os.ModeSymlink != 0 {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
			return nil
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			result.Issues = append(result.Issues, Issue{"FILE_INFO_FAILURE", path, infoErr.Error()})
			return nil
		}
		id, idErr := opaqueID()
		if idErr != nil {
			result.Issues = append(result.Issues, Issue{"SCAN_ENTRY_FAILURE", path, idErr.Error()})
			return nil
		}
		file := File{Path: path, Name: e.Name(), Size: info.Size(), ID: id}
		if s.versions != nil {
			version, ok, versionErr := s.versions.Version(path)
			if versionErr != nil {
				result.Issues = append(result.Issues, Issue{"FILE_OPEN_FAILURE", path, versionErr.Error()})
			} else {
				file.Version, file.VersionKnown = version, ok
			}
		}
		result.Files = append(result.Files, file)
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("SCAN_ENTRY_FAILURE: %w", err)
	}
	sort.Slice(result.Files, func(i, j int) bool {
		a, b := strings.ToLower(result.Files[i].Path), strings.ToLower(result.Files[j].Path)
		if a == b {
			return result.Files[i].Path < result.Files[j].Path
		}
		return a < b
	})
	return result, nil
}

// opaqueID создаёт новый непредсказуемый ID текущего сканирования.
func opaqueID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("генерация ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

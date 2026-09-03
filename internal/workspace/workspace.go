package workspace

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dslparser/internal/report"
)

// File описывает найденный сценарий без сохранения AST.
type File struct {
	Path         string
	Name         string
	Size         int64
	Version      string
	VersionKnown bool
	ID           string
	ReportID     string
	ReportPath   string
	ReportTime   time.Time
}

// Issue описывает восстановимую ошибку сканирования.
type Issue struct {
	Code    string
	Path    string
	Message string
}

// Scan содержит отсортированные сценарии и ошибки.
type Scan struct {
	Root            string
	Files           []File
	Issues          []Issue
	RejectedReports []Issue
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
	if _, err = f.Readdirnames(1); err != nil && !errors.Is(err, io.EOF) {
		f.Close()
		return "", fmt.Errorf("WORKDIR_ACCESS_FAILURE: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("WORKDIR_ACCESS_FAILURE: %w", err)
	}
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
	result := Scan{Root: clean, Files: []File{}, Issues: []Issue{}, RejectedReports: []Issue{}}
	reports := make([]reportCandidate, 0)
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
		if e.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), "report.json") {
			reports = append(reports, readReportCandidate(path, e))
			return nil
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
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
	matchReports(&result, reports)
	return result, nil
}

// reportCandidate хранит результат строгого чтения одного кандидата отчёта.
type reportCandidate struct {
	path     string
	modified time.Time
	value    report.Report
	issue    *Issue
}

// schemaHeader позволяет отличить неподдерживаемую схему без нетипизированного JSON.
type schemaHeader struct {
	SchemaVersion int `json:"schema_version"`
}

// readReportCandidate строго читает и классифицирует кандидат отчёта.
func readReportCandidate(path string, entry fs.DirEntry) reportCandidate {
	candidate := reportCandidate{path: path}
	info, err := entry.Info()
	if err != nil {
		candidate.issue = &Issue{"REPORT_READ_FAILURE", path, err.Error()}
		return candidate
	}
	candidate.modified = info.ModTime()
	data, err := os.ReadFile(path)
	if err != nil {
		candidate.issue = &Issue{"REPORT_READ_FAILURE", path, err.Error()}
		return candidate
	}
	var header schemaHeader
	if err := json.Unmarshal(data, &header); err != nil {
		candidate.issue = &Issue{"REPORT_INVALID_JSON", path, err.Error()}
		return candidate
	}
	if header.SchemaVersion != 1 {
		candidate.issue = &Issue{"REPORT_SCHEMA_UNSUPPORTED", path, fmt.Sprintf("schema_version %d", header.SchemaVersion)}
		return candidate
	}
	candidate.value, err = report.DecodeStrict(data)
	if err != nil {
		candidate.issue = &Issue{"REPORT_SCHEMA_INVALID", path, err.Error()}
	}
	return candidate
}

// matchReports выбирает активные отчёты и классифицирует остальные кандидаты.
func matchReports(scan *Scan, candidates []reportCandidate) {
	bySource := make(map[string][]reportCandidate)
	for _, candidate := range candidates {
		if candidate.issue != nil {
			scan.RejectedReports = append(scan.RejectedReports, *candidate.issue)
			continue
		}
		key := pathKey(candidate.value.SourceFile.Path)
		found := false
		for _, file := range scan.Files {
			if pathKey(file.Path) == key && file.Name == candidate.value.SourceFile.Name {
				found = true
				break
			}
		}
		if !found {
			scan.RejectedReports = append(scan.RejectedReports, Issue{"REPORT_ORPHAN", candidate.path, "исходный сценарий не найден"})
			continue
		}
		bySource[key] = append(bySource[key], candidate)
	}
	for i := range scan.Files {
		values := bySource[pathKey(scan.Files[i].Path)]
		if len(values) == 0 {
			continue
		}
		sort.Slice(values, func(i, j int) bool {
			if !values[i].value.Analysis.FinishedAt.Equal(values[j].value.Analysis.FinishedAt) {
				return values[i].value.Analysis.FinishedAt.After(values[j].value.Analysis.FinishedAt)
			}
			if !values[i].modified.Equal(values[j].modified) {
				return values[i].modified.After(values[j].modified)
			}
			return values[i].path < values[j].path
		})
		id, err := opaqueID()
		if err != nil {
			scan.Issues = append(scan.Issues, Issue{"SCAN_ENTRY_FAILURE", values[0].path, err.Error()})
			continue
		}
		scan.Files[i].ReportID, scan.Files[i].ReportPath, scan.Files[i].ReportTime = id, values[0].path, values[0].value.Analysis.FinishedAt
		for _, duplicate := range values[1:] {
			scan.RejectedReports = append(scan.RejectedReports, Issue{"REPORT_DUPLICATE", duplicate.path, "найден более новый активный отчёт"})
		}
	}
	sort.Slice(scan.RejectedReports, func(i, j int) bool { return scan.RejectedReports[i].Path < scan.RejectedReports[j].Path })
}

// pathKey нормализует путь для сравнения на текущей ОС.
func pathKey(path string) string {
	path = filepath.Clean(path)
	if isWindows() {
		return strings.ToLower(path)
	}
	return path
}

// opaqueID создаёт новый непредсказуемый ID текущего сканирования.
func opaqueID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("генерация ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

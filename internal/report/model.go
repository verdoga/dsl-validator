package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dslparser/validatorapi"
)

// Report представляет строгий JSON-отчёт schema v1.
type Report struct {
	SchemaVersion int        `json:"schema_version"`
	SourceFile    SourceFile `json:"source_file"`
	DSLVersion    DSLVersion `json:"dsl_version"`
	Analysis      Analysis   `json:"analysis"`
	Summary       Summary    `json:"summary"`
	Entries       []Entry    `json:"entries"`
}

// SourceFile идентифицирует сценарий абсолютным путём и именем.
type SourceFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// DSLVersion описывает определённую или неопределённую версию.
type DSLVersion struct {
	Status    string  `json:"status"`
	Raw       *string `json:"raw"`
	Canonical *string `json:"canonical"`
}

// Analysis описывает выполнение конвейера.
type Analysis struct {
	VersionMode     string    `json:"version_mode"`
	SelectedVersion *string   `json:"selected_version"`
	Status          string    `json:"status"`
	FailedStage     *string   `json:"failed_stage"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	DurationMS      int64     `json:"duration_ms"`
	Timezone        *string   `json:"timezone"`
}

// Summary содержит полные счётчики уровней и источников.
type Summary struct {
	Total     int          `json:"total"`
	ByLevel   LevelCounts  `json:"by_level"`
	BySource  SourceCounts `json:"by_source"`
	HasErrors bool         `json:"has_errors"`
}

// LevelCounts содержит счётчики каждого уровня.
type LevelCounts struct {
	Error          int `json:"error"`
	Warning        int `json:"warning"`
	Recommendation int `json:"recommendation"`
}

// SourceCounts содержит счётчики каждого источника.
type SourceCounts struct {
	Parser     int `json:"parser"`
	Diagnostic int `json:"diagnostic"`
	Validator  int `json:"validator"`
}

// Entry представляет одну пронумерованную запись.
type Entry struct {
	Number          int                   `json:"number"`
	Code            string                `json:"code"`
	Level           validatorapi.Severity `json:"level"`
	Source          validatorapi.Source   `json:"source"`
	Category        *string               `json:"category"`
	Title           *string               `json:"title"`
	Message         string                `json:"message"`
	Description     *string               `json:"description"`
	Basis           *string               `json:"basis"`
	Location        *Location             `json:"location"`
	RelatedLocation *Location             `json:"related_location"`
	Context         *Context              `json:"context"`
	Reference       *string               `json:"reference"`
	BadExample      *string               `json:"bad_example"`
	order           int
}

// Location сохраняет полуоткрытый диапазон без преобразования.
type Location struct {
	Start        validatorapi.Position `json:"start"`
	End          validatorapi.Position `json:"end"`
	EndExclusive bool                  `json:"end_exclusive"`
}

// Context содержит нормализованные строки вокруг проблемы.
type Context struct {
	Lines []ContextLine `json:"lines"`
}

// ContextLine содержит строку контекста и её роль.
type ContextLine struct {
	Number int    `json:"number"`
	Role   string `json:"role"`
	Text   string `json:"text"`
}

// New создаёт начальную модель отчёта.
func New(path, mode string, selected *string, started time.Time) Report {
	return Report{SchemaVersion: 1, SourceFile: SourceFile{Path: path, Name: filepath.Base(path)}, DSLVersion: DSLVersion{Status: "undefined"}, Analysis: Analysis{VersionMode: mode, SelectedVersion: selected, Status: "completed", StartedAt: started}, Entries: []Entry{}}
}

// Finish сортирует, нумерует и пересчитывает отчёт; finished задаёт время завершения.
func (r *Report) Finish(finished time.Time) {
	sortEntries(r.Entries)
	r.Summary = Summary{}
	for i := range r.Entries {
		e := &r.Entries[i]
		e.Number = i + 1
		r.Summary.Total++
		switch e.Level {
		case validatorapi.SeverityError:
			r.Summary.ByLevel.Error++
		case validatorapi.SeverityWarning:
			r.Summary.ByLevel.Warning++
		case validatorapi.SeverityRecommendation:
			r.Summary.ByLevel.Recommendation++
		}
		switch e.Source {
		case validatorapi.SourceParser:
			r.Summary.BySource.Parser++
		case validatorapi.SourceDiagnostic:
			r.Summary.BySource.Diagnostic++
		case validatorapi.SourceValidator:
			r.Summary.BySource.Validator++
		}
	}
	r.Summary.HasErrors = r.Summary.ByLevel.Error > 0
	if r.Analysis.Status == "completed" && r.Summary.HasErrors {
		r.Analysis.Status = "completed_with_errors"
	}
	r.Analysis.FinishedAt = finished
	r.Analysis.DurationMS = finished.Sub(r.Analysis.StartedAt).Milliseconds()
	if r.Analysis.DurationMS < 0 {
		r.Analysis.DurationMS = 0
	}
}

// sortEntries упорядочивает записи по позиции, источнику и порядку.
func sortEntries(entries []Entry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entryLess(entries[j], entries[j-1]); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// entryLess сообщает порядок: true — a должна предшествовать b, false — не должна.
func entryLess(a, b Entry) bool {
	if a.Location == nil {
		return false
	}
	if b.Location == nil {
		return true
	}
	ap, bp := a.Location.Start, b.Location.Start
	if ap.Line != bp.Line {
		return ap.Line < bp.Line
	}
	if ap.Column != bp.Column {
		return ap.Column < bp.Column
	}
	rank := func(s validatorapi.Source) int {
		if s == validatorapi.SourceParser {
			return 0
		}
		if s == validatorapi.SourceDiagnostic {
			return 1
		}
		return 2
	}
	if rank(a.Source) != rank(b.Source) {
		return rank(a.Source) < rank(b.Source)
	}
	return a.order < b.order
}

// Validate проверяет обязательные поля, enum и согласованность счётчиков.
func (r Report) Validate() error {
	if r.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая schema_version")
	}
	if !filepath.IsAbs(r.SourceFile.Path) || r.SourceFile.Name != filepath.Base(r.SourceFile.Path) {
		return fmt.Errorf("неверный источник")
	}
	if r.Analysis.VersionMode != "auto" && r.Analysis.VersionMode != "explicit" {
		return fmt.Errorf("неверный режим")
	}
	if len(r.Entries) != r.Summary.Total {
		return fmt.Errorf("неверный total")
	}
	for i, e := range r.Entries {
		if e.Number != i+1 || e.Code == "" || e.Message == "" {
			return fmt.Errorf("неверная запись %d", i+1)
		}
	}
	return nil
}

// DecodeStrict строго декодирует один JSON-отчёт без неизвестных полей.
func DecodeStrict(data []byte) (Report, error) {
	var r Report
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return Report{}, err
	}
	if d.More() {
		return Report{}, fmt.Errorf("лишние JSON-значения")
	}
	if err := r.Validate(); err != nil {
		return Report{}, err
	}
	return r, nil
}

// CanonicalPath возвращает фиксированный путь отчёта рядом со сценарием.
func CanonicalPath(source string) string {
	ext := filepath.Ext(source)
	return strings.TrimSuffix(source, ext) + " - report.json"
}

// WriteAtomic сериализует и атомарно заменяет только допустимый канонический отчёт.
func WriteAtomic(r Report) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("REPORT_SERIALIZE_FAILURE: %w", err)
	}
	target := CanonicalPath(r.SourceFile.Path)
	if old, err := os.ReadFile(target); err == nil {
		existing, e := DecodeStrict(old)
		if e != nil || filepath.Clean(existing.SourceFile.Path) != filepath.Clean(r.SourceFile.Path) {
			return fmt.Errorf("REPORT_PATH_CONFLICT")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("REPORT_PATH_CONFLICT: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("REPORT_SERIALIZE_FAILURE: %w", err)
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(filepath.Dir(target), ".dsl-report-*")
	if err != nil {
		return fmt.Errorf("REPORT_TEMP_CREATE_FAILURE: %w", err)
	}
	name := f.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(name)
		}
	}()
	if _, err = f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("REPORT_WRITE_FAILURE: %w", err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("REPORT_SYNC_FAILURE: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("REPORT_CLOSE_FAILURE: %w", err)
	}
	if err = replaceFile(name, target); err != nil {
		return fmt.Errorf("REPORT_REPLACE_FAILURE: %w", err)
	}
	ok = true
	return nil
}

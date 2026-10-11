package reporter

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

func TestCountDiagnosticsSelectsExactSource(t *testing.T) {
	result := &model.Result{Diagnostics: []model.Diagnostic{
		{Source: "p1", SeverityLevel: diagnostics.SeverityError},
		{Source: "p2", SeverityLevel: diagnostics.SeverityError},
		{Source: "p2", SeverityLevel: diagnostics.SeverityWarning},
		{Source: "p2", SeverityLevel: diagnostics.SeverityError, Fatal: true},
		{Source: "p2", SeverityLevel: diagnostics.SeverityRecommendation},
		{Source: "p1", SeverityLevel: diagnostics.SeverityWarning},
		{Source: "p2", SeverityLevel: diagnostics.SeverityRecommendation},
		{Source: "p2", SeverityLevel: diagnostics.SeverityRecommendation},
		{Source: "p20", SeverityLevel: diagnostics.SeverityError},
		{Source: "P2", SeverityLevel: diagnostics.SeverityWarning},
		{Source: " p2", SeverityLevel: diagnostics.SeverityRecommendation},
		{Source: "p2 ", SeverityLevel: diagnostics.SeverityRecommendation},
		{Source: "", SeverityLevel: diagnostics.SeverityError},
	}}
	tests := []struct {
		name   string
		result *model.Result
		source string
		want   counts
	}{
		{"current_run", result, "p2", counts{errors: 2, problems: 1, recommendations: 3}},
		{"explicit_old_run", result, "p1", counts{errors: 1, problems: 1}},
		{"prefix_is_not_match", result, "p", counts{}},
		{"unknown_source", result, "p3", counts{}},
		{"empty_source", result, "", counts{}},
		{"nil_model", nil, "p2", counts{}},
		{"empty_diagnostics", &model.Result{}, "p2", counts{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countDiagnostics(tt.result, tt.source); got != tt.want {
				t.Errorf("countDiagnostics = %+v, требуется %+v", got, tt.want)
			}
		})
	}
	result.Diagnostics[3].Fatal = false
	if got := countDiagnostics(result, "p2"); got != (counts{errors: 2, problems: 1, recommendations: 3}) {
		t.Errorf("изменение Fatal повлияло на подсчёт: %+v", got)
	}
}

func TestSummarizeFileClassifiesTechnicalFailures(t *testing.T) {
	cause := errors.New("проверка недоступна")
	failure := fmt.Errorf("обработка JSON: %w", fmt.Errorf("сбой диагностики V017: %w", cause))
	result := &model.Result{Diagnostics: []model.Diagnostic{
		{Source: "p1", SeverityLevel: diagnostics.SeverityError},
		{Source: "p2", SeverityLevel: diagnostics.SeverityError},
		{Source: "p2", SeverityLevel: diagnostics.SeverityWarning},
		{Source: "p2", SeverityLevel: diagnostics.SeverityRecommendation},
	}}
	tests := []struct {
		name       string
		updated    bool
		err        error
		wantFailed bool
		wantReason string
	}{
		{"saved", true, nil, false, ""},
		{"not_saved_without_error", false, nil, true, "результат не сохранён"},
		{"not_saved_with_error", false, failure, true, "обработка JSON: сбой диагностики V017: проверка недоступна"},
		{"saved_with_error", true, failure, true, "обработка JSON: сбой диагностики V017: проверка недоступна"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := FileReport{
				Path: "/input/result.json", Result: result, ProcessingID: "p2",
				Updated: tt.updated, Err: tt.err, Duration: 1234567 * time.Nanosecond,
			}
			got := summarizeFile(report)
			if got.failed != tt.wantFailed || got.updated != tt.updated || got.reason != tt.wantReason {
				t.Errorf("сбой/сохранение/причина = %t/%t/%q, требуется %t/%t/%q",
					got.failed, got.updated, got.reason, tt.wantFailed, tt.updated, tt.wantReason)
			}
			if got.path != report.Path || got.duration != report.Duration {
				t.Errorf("путь или длительность изменены: %q, %v", got.path, got.duration)
			}
			if got.counts != (counts{errors: 1, problems: 1, recommendations: 1}) {
				t.Errorf("срабатывания текущего запуска потеряны: %+v", got.counts)
			}
		})
	}
}

func TestSummarizeFilePreservesKnownAndUnknownLineCount(t *testing.T) {
	zero, positive := 0, 7
	tests := []struct {
		name   string
		result *model.Result
		want   *int
	}{
		{"nil_model", nil, nil},
		{"unknown", &model.Result{}, nil},
		{"known_zero", &model.Result{Document: model.Document{LineCount: &zero}}, &zero},
		{"known_positive", &model.Result{Document: model.Document{LineCount: &positive}}, &positive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeFile(FileReport{
				Path: "/input/unreadable.json", Result: tt.result,
				Err: errors.New("чтение недоступно"), Duration: 3500 * time.Microsecond,
			})
			if tt.want == nil {
				if got.lineCount != nil {
					t.Errorf("lineCount = %d, требуется неизвестное количество", *got.lineCount)
				}
			} else if got.lineCount == nil || *got.lineCount != *tt.want {
				t.Errorf("lineCount = %v, требуется %d", got.lineCount, *tt.want)
			}
			if got.path != "/input/unreadable.json" || got.duration != 3500*time.Microsecond {
				t.Errorf("потеряны путь или длительность отказа: %q, %v", got.path, got.duration)
			}
			if got.counts != (counts{}) {
				t.Errorf("до начала запуска требуются нули, получено %+v", got.counts)
			}
		})
	}
}

func TestSummarizeFileWithEmptyProcessingIDExcludesHistory(t *testing.T) {
	result := &model.Result{Diagnostics: []model.Diagnostic{
		{Source: "p1", SeverityLevel: diagnostics.SeverityError},
		{Source: "", SeverityLevel: diagnostics.SeverityWarning},
	}}
	if got := summarizeFile(FileReport{Result: result}).counts; got != (counts{}) {
		t.Errorf("пустой ProcessingID должен давать нули, получено %+v", got)
	}
}

func TestSummarizeFileDoesNotMutateOrRetainModelValues(t *testing.T) {
	lineCount, durationMs := 1, 99
	result := &model.Result{
		Format:     model.FormatVersion1,
		Document:   model.Document{LineCount: &lineCount, HasErrors: true},
		Processing: []model.Processing{{ID: "p2", Tool: "dsl-validator", DurationMs: &durationMs}},
		Lines:      []model.Line{{Number: 1, Raw: "исходная строка", HasErrors: true}},
		Diagnostics: []model.Diagnostic{
			{ID: "d1", Source: "p2", SeverityLevel: diagnostics.SeverityError, Message: "ошибка"},
		},
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	summary := summarizeFile(FileReport{
		Result: result, ProcessingID: "p2", Updated: true, Duration: 1500 * time.Microsecond,
	})
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("построение сводки изменило исходную модель")
	}
	if summary.duration != 1500*time.Microsecond {
		t.Errorf("длительность отчёта подменена значением модели: %v", summary.duration)
	}
	lineCount = 8
	result.Diagnostics[0].SeverityLevel = diagnostics.SeverityRecommendation
	if summary.lineCount == nil || *summary.lineCount != 1 {
		t.Fatal("сводка не сохранила независимое значение числа строк")
	}
	if summary.counts != (counts{errors: 1}) {
		t.Errorf("изменение модели повлияло на сводку: %+v", summary.counts)
	}
	*summary.lineCount = 12
	if lineCount != 8 {
		t.Fatal("изменение сводки затронуло исходное число строк")
	}
}

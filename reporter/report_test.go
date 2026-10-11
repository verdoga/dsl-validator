package reporter

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/verdoga/dsl-validator/model"
)

func TestFileReportRepresentsProcessingAttempts(t *testing.T) {
	result := &model.Result{Format: model.FormatVersion1}
	openCause := errors.New("доступ запрещён")
	openErr := fmt.Errorf("не удалось открыть JSON: %w", openCause)
	cleanupCause := errors.New("резерв недоступен")
	cleanupErr := fmt.Errorf("новый JSON установлен; не удалось удалить резерв: %w", cleanupCause)

	tests := []struct {
		name           string
		report         FileReport
		wantResult     *model.Result
		wantProcessing string
		wantUpdated    bool
		wantErr        error
		wantCause      error
		wantMessage    string
		wantDuration   time.Duration
	}{
		{
			name: "open_failure_before_model",
			report: FileReport{
				Path: "/input/result.json", Err: openErr,
				Duration: 1500 * time.Microsecond,
			},
			wantErr: openErr, wantCause: openCause,
			wantMessage:  "не удалось открыть JSON: доступ запрещён",
			wantDuration: 1500 * time.Microsecond,
		},
		{
			name: "saved_without_failure",
			report: FileReport{
				Path: "/input/result.json", Result: result, ProcessingID: "p2",
				Updated: true, Duration: 23 * time.Millisecond,
			},
			wantResult: result, wantProcessing: "p2", wantUpdated: true,
			wantDuration: 23 * time.Millisecond,
		},
		{
			name: "saved_with_cleanup_failure",
			report: FileReport{
				Path: "/input/result.json", Result: result, ProcessingID: "p3",
				Updated: true, Err: cleanupErr, Duration: 42 * time.Millisecond,
			},
			wantResult: result, wantProcessing: "p3", wantUpdated: true,
			wantErr: cleanupErr, wantCause: cleanupCause,
			wantMessage:  "новый JSON установлен; не удалось удалить резерв: резерв недоступен",
			wantDuration: 42 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.report.Path != "/input/result.json" {
				t.Errorf("Path = %q, требуется абсолютный путь JSON %q", tt.report.Path, "/input/result.json")
			}
			if tt.report.Result != tt.wantResult {
				t.Errorf("Result = %p, требуется %p", tt.report.Result, tt.wantResult)
			}
			if tt.report.ProcessingID != tt.wantProcessing {
				t.Errorf("ProcessingID = %q, требуется %q", tt.report.ProcessingID, tt.wantProcessing)
			}
			if tt.report.Updated != tt.wantUpdated {
				t.Errorf("Updated = %t, требуется %t", tt.report.Updated, tt.wantUpdated)
			}
			if tt.report.Duration != tt.wantDuration {
				t.Errorf("Duration = %v, требуется %v", tt.report.Duration, tt.wantDuration)
			}
			if tt.report.Err != tt.wantErr {
				t.Fatalf("Err = %v, требуется переданная ошибка %v", tt.report.Err, tt.wantErr)
			}
			if !errors.Is(tt.report.Err, tt.wantCause) {
				t.Errorf("Err = %v, требуется исходная причина %v", tt.report.Err, tt.wantCause)
			}
			if tt.report.Err != nil && tt.report.Err.Error() != tt.wantMessage {
				t.Errorf("Err.Error() = %q, требуется %q", tt.report.Err.Error(), tt.wantMessage)
			}
		})
	}
}

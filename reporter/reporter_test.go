package reporter

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

type reportTestWriter struct {
	bytes.Buffer
	err   error
	calls int
}

func (w *reportTestWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.err != nil {
		return 0, w.err
	}
	return w.Buffer.Write(p)
}

func TestReporterStartAndEmptyFinish(t *testing.T) {
	var output, errorOutput bytes.Buffer
	r := New(&output, &errorOutput)
	if output.Len() != 0 || errorOutput.Len() != 0 {
		t.Fatal("New выполнил вывод")
	}
	if err := r.Start(17); err != nil {
		t.Fatal(err)
	}
	code, err := r.Finish()
	want := "Зарегистрировано диагностик: 17\nИТОГО: всего: 0, обработано: 0, сбоев: 0, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0\n"
	if code != 0 || err != nil || output.String() != want || errorOutput.Len() != 0 {
		t.Fatalf("Finish = (%d, %v), output = %q, errorOutput = %q", code, err, output.String(), errorOutput.String())
	}
}

func TestReporterRoutesRecordsAndAccumulatesOnlyCurrentRuns(t *testing.T) {
	result := &model.Result{Diagnostics: []model.Diagnostic{
		{Source: "old", SeverityLevel: diagnostics.SeverityError},
		{Source: "p2", SeverityLevel: diagnostics.SeverityError, Fatal: true},
		{Source: "p2", SeverityLevel: diagnostics.SeverityWarning},
		{Source: "p2", SeverityLevel: diagnostics.SeverityRecommendation},
	}}
	success := FileReport{Path: "/input/one.json", Result: result, ProcessingID: "p2", Updated: true}
	cleanup := FileReport{Path: "/input/two.json", Result: result, ProcessingID: "p2", Updated: true, Err: errors.New("не удалось удалить резерв")}
	checkErr := fmt.Errorf("обработка: %w", errors.New("сбой диагностики V017: недоступно"))
	tests := []struct {
		name       string
		reports    []FileReport
		wantCode   int
		wantTotal  string
		wantErrors string
	}{
		{
			name: "findings_are_success", reports: []FileReport{success},
			wantTotal: "ИТОГО: всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 1, проблем: 1, рекомендаций: 1\n",
		},
		{
			name: "saved_with_cleanup_failure", reports: []FileReport{success, cleanup}, wantCode: 1,
			wantTotal:  "ИТОГО: всего: 2, обработано: 1, сбоев: 1, сохранено: 2, ошибок: 2, проблем: 2, рекомендаций: 2\n",
			wantErrors: "СБОЙ \"/input/two.json\", сохранён: да, строк: нет данных, ошибок: 1, проблем: 1, рекомендаций: 1, время: 0 мс, причина: \"не удалось удалить резерв\"\n",
		},
		{
			name: "early_stop_after_check_failure", reports: []FileReport{success, {Path: "/input/two.json", Err: checkErr}}, wantCode: 1,
			wantTotal:  "ИТОГО: всего: 2, обработано: 1, сбоев: 1, сохранено: 1, ошибок: 1, проблем: 1, рекомендаций: 1\n",
			wantErrors: "СБОЙ \"/input/two.json\", сохранён: нет, строк: нет данных, ошибок: 0, проблем: 0, рекомендаций: 0, время: 0 мс, причина: \"обработка: сбой диагностики V017: недоступно\"\n",
		},
		{
			name: "not_saved_without_error", reports: []FileReport{{Path: "/input/two.json", Result: result}}, wantCode: 1,
			wantTotal:  "ИТОГО: всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0\n",
			wantErrors: "СБОЙ \"/input/two.json\", сохранён: нет, строк: нет данных, ошибок: 0, проблем: 0, рекомендаций: 0, время: 0 мс, причина: \"результат не сохранён\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output, errorOutput bytes.Buffer
			r := New(&output, &errorOutput)
			for _, report := range tt.reports {
				if err := r.Record(report); err != nil {
					t.Fatalf("Record вернул ошибку обработки вместо результата вывода: %v", err)
				}
			}
			code, err := r.Finish()
			wantOutput := tt.wantTotal
			if tt.reports[0].Updated {
				wantOutput = "ОБРАБОТАН \"/input/one.json\", сохранён: да, строк: нет данных, ошибок: 1, проблем: 1, рекомендаций: 1, время: 0 мс\n" + wantOutput
			}
			if code != tt.wantCode || err != nil {
				t.Errorf("Finish = (%d, %v), требуется (%d, nil)", code, err, tt.wantCode)
			}
			if output.String() != wantOutput || errorOutput.String() != tt.wantErrors {
				t.Errorf("output = %q, требуется %q; errorOutput = %q, требуется %q",
					output.String(), wantOutput, errorOutput.String(), tt.wantErrors)
			}
		})
	}
}

func TestReporterDoesNotRetainModelForTotals(t *testing.T) {
	var output, errorOutput bytes.Buffer
	r := New(&output, &errorOutput)
	result := &model.Result{Diagnostics: []model.Diagnostic{{Source: "p1", SeverityLevel: diagnostics.SeverityError}}}
	if err := r.Record(FileReport{Result: result, ProcessingID: "p1", Updated: true}); err != nil {
		t.Fatal(err)
	}
	result.Diagnostics[0].SeverityLevel = diagnostics.SeverityRecommendation
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{Source: "p1", SeverityLevel: diagnostics.SeverityWarning})
	output.Reset()
	code, err := r.Finish()
	want := "ИТОГО: всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 1, проблем: 0, рекомендаций: 0\n"
	if code != 0 || err != nil || output.String() != want {
		t.Fatalf("Finish = (%d, %v), итог после изменения модели = %q", code, err, output.String())
	}
}

func TestReporterRecordWriterFailuresPreserveAccounting(t *testing.T) {
	outputErr, errorOutputErr := errors.New("output недоступен"), errors.New("errorOutput недоступен")
	processingErr := errors.New("технический отказ")
	tests := []struct {
		name          string
		updated       bool
		processingErr error
		outputErr     error
		errorErr      error
		outputCalls   int
		wantCode      int
		wantTotal     string
	}{
		{
			name: "output_failure", updated: true, outputErr: outputErr, outputCalls: 1,
			wantTotal: "ИТОГО: всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 1, проблем: 0, рекомендаций: 0\n",
		},
		{
			name: "both_outputs_fail", updated: true, outputErr: outputErr, errorErr: errorOutputErr, outputCalls: 1,
			wantTotal: "ИТОГО: всего: 1, обработано: 1, сбоев: 0, сохранено: 1, ошибок: 1, проблем: 0, рекомендаций: 0\n",
		},
		{
			name: "failure_report_output_fails", processingErr: processingErr, errorErr: errorOutputErr, wantCode: 1,
			wantTotal: "ИТОГО: всего: 1, обработано: 0, сбоев: 1, сохранено: 0, ошибок: 1, проблем: 0, рекомендаций: 0\n",
		},
		{
			name: "saved_failure_report_output_fails", updated: true, processingErr: processingErr, errorErr: errorOutputErr, wantCode: 1,
			wantTotal: "ИТОГО: всего: 1, обработано: 0, сбоев: 1, сохранено: 1, ошибок: 1, проблем: 0, рекомендаций: 0\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &reportTestWriter{err: tt.outputErr}
			errorOutput := &reportTestWriter{err: tt.errorErr}
			r := New(output, errorOutput)
			result := &model.Result{Diagnostics: []model.Diagnostic{{Source: "p1", SeverityLevel: diagnostics.SeverityError}}}
			err := r.Record(FileReport{Path: "/input/result.json", Result: result, ProcessingID: "p1", Updated: tt.updated, Err: tt.processingErr})
			if err == nil {
				t.Fatal("Record не вернул ошибку записи")
			}
			if tt.outputErr != nil && !errors.Is(err, tt.outputErr) {
				t.Errorf("потеряна ошибка output: %v", err)
			}
			if tt.errorErr != nil && !errors.Is(err, tt.errorErr) {
				t.Errorf("потеряна ошибка errorOutput: %v", err)
			}
			if errors.Is(err, processingErr) {
				t.Errorf("Record вернул техническую ошибку обработки: %v", err)
			}
			if output.calls != tt.outputCalls || errorOutput.calls != 1 {
				t.Errorf("число записей output/errorOutput = %d/%d, требуется %d/1", output.calls, errorOutput.calls, tt.outputCalls)
			}
			if tt.errorErr == nil {
				message := errorOutput.String()
				if !strings.Contains(message, "/input/result.json") || !strings.Contains(message, outputErr.Error()) || strings.Count(message, "\n") != 1 {
					t.Errorf("уведомление не содержит путь и причину одной строкой: %q", message)
				}
			}
			output.err = nil
			output.Reset()
			code, finishErr := r.Finish()
			if code != tt.wantCode || finishErr != nil || output.String() != tt.wantTotal {
				t.Errorf("учёт после ошибки вывода: Finish = (%d, %v), итог = %q", code, finishErr, output.String())
			}
		})
	}
}

func TestReporterStartReturnsWriterError(t *testing.T) {
	wantErr := errors.New("запись старта недоступна")
	output, errorOutput := &reportTestWriter{err: wantErr}, &reportTestWriter{}
	r := New(output, errorOutput)
	if err := r.Start(3); !errors.Is(err, wantErr) {
		t.Errorf("Start = %v, требуется %v", err, wantErr)
	}
	if output.calls != 1 || errorOutput.calls != 0 {
		t.Errorf("число записей output/errorOutput = %d/%d, требуется 1/0", output.calls, errorOutput.calls)
	}
	output.err = nil
	code, err := r.Finish()
	want := "ИТОГО: всего: 0, обработано: 0, сбоев: 0, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0\n"
	if code != 0 || err != nil || output.String() != want {
		t.Errorf("ошибка Start изменила статистику: (%d, %v), %q", code, err, output.String())
	}
}

func TestReporterFinishReturnsCodeIndependentlyOfWriterError(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed_%t", failed), func(t *testing.T) {
			wantErr := errors.New("запись итога недоступна")
			output, errorOutput := &reportTestWriter{}, &reportTestWriter{}
			r := New(output, errorOutput)
			wantCode := 0
			if failed {
				if err := r.Record(FileReport{Path: "/input/result.json"}); err != nil {
					t.Fatal(err)
				}
				wantCode = 1
			}
			previousErrorCalls := errorOutput.calls
			output.err = wantErr
			code, err := r.Finish()
			if code != wantCode || !errors.Is(err, wantErr) {
				t.Errorf("Finish = (%d, %v), требуется (%d, %v)", code, err, wantCode, wantErr)
			}
			if output.calls != 1 || errorOutput.calls != previousErrorCalls {
				t.Errorf("Finish выполнил лишние записи: output=%d, errorOutput=%d", output.calls, errorOutput.calls)
			}
		})
	}
}

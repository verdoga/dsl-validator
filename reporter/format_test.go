package reporter

import (
	"strings"
	"testing"
	"time"
)

func TestFormatFileLine(t *testing.T) {
	zero, lineCount := 0, 12
	tests := []struct {
		name    string
		summary fileSummary
		want    string
	}{
		{
			name: "processed_with_findings_and_fractional_millisecond",
			summary: fileSummary{
				path: "/input/result.json", lineCount: &lineCount, updated: true,
				counts:   counts{errors: 2, problems: 3, recommendations: 4},
				duration: 1999999 * time.Nanosecond,
			},
			want: `ОБРАБОТАН "/input/result.json", сохранён: да, строк: 12, ошибок: 2, проблем: 3, рекомендаций: 4, время: 1 мс`,
		},
		{
			name: "known_zero_lines_and_zero_duration",
			summary: fileSummary{
				path: "/input/empty.json", lineCount: &zero, updated: true,
			},
			want: `ОБРАБОТАН "/input/empty.json", сохранён: да, строк: 0, ошибок: 0, проблем: 0, рекомендаций: 0, время: 0 мс`,
		},
		{
			name: "unknown_lines",
			summary: fileSummary{
				path: "/input/unreadable.json", failed: true,
				reason: "доступ запрещён", duration: 5 * time.Millisecond,
			},
			want: `СБОЙ "/input/unreadable.json", сохранён: нет, строк: нет данных, ошибок: 0, проблем: 0, рекомендаций: 0, время: 5 мс, причина: "доступ запрещён"`,
		},
		{
			name: "unsaved_findings_and_submillisecond_duration",
			summary: fileSummary{
				path: "/input/result.json", lineCount: &lineCount, failed: true,
				counts: counts{errors: 2, problems: 3, recommendations: 4},
				reason: "запись недоступна", duration: 999999 * time.Nanosecond,
			},
			want: `СБОЙ "/input/result.json", сохранён: нет, строк: 12, ошибок: 2, проблем: 3, рекомендаций: 4, время: 0 мс, причина: "запись недоступна"`,
		},
		{
			name: "saved_with_failure",
			summary: fileSummary{
				path: "/input/result.json", lineCount: &lineCount,
				updated: true, failed: true, reason: "не удалось удалить резерв",
				duration: 25 * time.Millisecond,
			},
			want: `СБОЙ "/input/result.json", сохранён: да, строк: 12, ошибок: 0, проблем: 0, рекомендаций: 0, время: 25 мс, причина: "не удалось удалить резерв"`,
		},
		{
			name: "escaped_path_and_reason",
			summary: fileSummary{
				path: "/input/\"a\\b\tc\n\r.json", failed: true,
				reason: "сбой диагностики V017: \"a\\b\"\tпричина\nстрока\rконец",
			},
			want: `СБОЙ "/input/\"a\\b\tc\n\r.json", сохранён: нет, строк: нет данных, ошибок: 0, проблем: 0, рекомендаций: 0, время: 0 мс, причина: "сбой диагностики V017: \"a\\b\"\tпричина\nстрока\rконец"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFileLine(tt.summary)
			if got != tt.want {
				t.Errorf("formatFileLine = %q, требуется %q", got, tt.want)
			}
			if strings.ContainsAny(got, "\r\n\t") {
				t.Errorf("строка содержит неэкранированные управляющие символы: %q", got)
			}
		})
	}
}

func TestFormatTotalLine(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		failed    int
		updated   int
		totals    counts
		want      string
	}{
		{
			name: "no_attempts",
			want: "ИТОГО: всего: 0, обработано: 0, сбоев: 0, сохранено: 0, ошибок: 0, проблем: 0, рекомендаций: 0",
		},
		{
			name:  "mixed_results",
			total: 5, completed: 3, failed: 2, updated: 4,
			totals: counts{errors: 7, problems: 11, recommendations: 13},
			want:   "ИТОГО: всего: 5, обработано: 3, сбоев: 2, сохранено: 4, ошибок: 7, проблем: 11, рекомендаций: 13",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTotalLine(tt.total, tt.completed, tt.failed, tt.updated, tt.totals)
			if got != tt.want {
				t.Errorf("formatTotalLine = %q, требуется %q", got, tt.want)
			}
			if strings.ContainsAny(got, "\r\n") {
				t.Errorf("итог содержит перевод строки: %q", got)
			}
		})
	}
}

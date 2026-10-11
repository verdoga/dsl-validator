package reporter

import (
	"fmt"
	"strconv"
)

// formatFileLine формирует одну строку: ОБРАБОТАН или СБОЙ, путь JSON,
// сохранён: да/нет, строк: N/нет данных, ошибок: N, проблем: N,
// рекомендаций: N, время: N мс; при отказе добавляет причину.
// Пути и причины экранирует как %q, чтобы сохранялся однострочный формат.
// Эти числа означают полученные результаты даже при неудачной записи файла.
func formatFileLine(summary fileSummary) string {
	status := "ОБРАБОТАН"
	if summary.failed {
		status = "СБОЙ"
	}
	updated := "нет"
	if summary.updated {
		updated = "да"
	}
	lineCount := "нет данных"
	if summary.lineCount != nil {
		lineCount = strconv.Itoa(*summary.lineCount)
	}
	line := fmt.Sprintf("%s %q, сохранён: %s, строк: %s, ошибок: %d, проблем: %d, рекомендаций: %d, время: %d мс",
		status, summary.path, updated, lineCount, summary.counts.errors,
		summary.counts.problems, summary.counts.recommendations, summary.duration.Milliseconds())
	if summary.failed {
		line += fmt.Sprintf(", причина: %q", summary.reason)
	}
	return line
}

// formatTotalLine формирует ИТОГО с числами всего, обработано, сбоев,
// сохранено и суммами ошибок, проблем и рекомендаций.
func formatTotalLine(total, completed, failed, updated int, totals counts) string {
	return fmt.Sprintf("ИТОГО: всего: %d, обработано: %d, сбоев: %d, сохранено: %d, ошибок: %d, проблем: %d, рекомендаций: %d",
		total, completed, failed, updated, totals.errors, totals.problems, totals.recommendations)
}

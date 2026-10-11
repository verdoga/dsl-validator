package reporter

import (
	"time"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// counts хранит числа трёх уровней без отдельного уровня fatal.
type counts struct {
	// errors — число error.
	errors int
	// problems — число warning, отображаемое как «проблем».
	problems int
	// recommendations — число recommendation.
	recommendations int
}

// fileSummary содержит только значения для одной консольной строки.
type fileSummary struct {
	// path — абсолютный путь обрабатываемого JSON.
	path string
	// lineCount — число строк либо nil при неизвестном количестве.
	lineCount *int
	// counts — уровни только текущего Processing.
	counts counts
	// updated — фактическое сохранение результата.
	updated bool
	// failed — наличие технического отказа; false означает завершённую обработку.
	failed bool
	// reason — пояснение первого отказа, включая V-код сбойной Check.
	reason string
	// duration — длительность обработки до сохранения.
	duration time.Duration
}

// countDiagnostics считает три уровня по точному Source, не читает Fatal
// и не суммирует прежние запуски; nil-модель либо пустой source дают нули.
func countDiagnostics(result *model.Result, source string) counts {
	var totals counts
	if result == nil || source == "" {
		return totals
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Source != source {
			continue
		}
		switch diagnostic.SeverityLevel {
		case diagnostics.SeverityError:
			totals.errors++
		case diagnostics.SeverityWarning:
			totals.problems++
		case diagnostics.SeverityRecommendation:
			totals.recommendations++
		}
	}
	return totals
}

// summarizeFile извлекает неизменяемые значения и причины технических отказов.
func summarizeFile(report FileReport) fileSummary {
	summary := fileSummary{
		path:     report.Path,
		counts:   countDiagnostics(report.Result, report.ProcessingID),
		updated:  report.Updated,
		failed:   report.Err != nil || !report.Updated,
		duration: report.Duration,
	}
	if report.Result != nil && report.Result.Document.LineCount != nil {
		lineCount := *report.Result.Document.LineCount
		summary.lineCount = &lineCount
	}
	if report.Err != nil {
		summary.reason = report.Err.Error()
	} else if !report.Updated {
		summary.reason = "результат не сохранён"
	}
	return summary
}

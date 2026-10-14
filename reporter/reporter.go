package reporter

import (
	"errors"
	"fmt"
	"io"
)

// Reporter хранит счётчики одного последовательного запуска и потоки вывода.
type Reporter struct {
	// output — поток русскоязычного отчёта, обычно os.Stdout.
	output io.Writer
	// errorOutput — поток сообщений о технических отказах, обычно os.Stderr.
	errorOutput io.Writer
	// total — число FileReport, переданных в Record.
	total int
	// completed — число сохранённых файлов без технического отказа.
	completed int
	// failed — число файлов с техническим отказом; total=completed+failed.
	failed int
	// updated — число фактически установленных JSON, включая отказ очистки резерва.
	updated int
	// totals — суммы уровней текущих запусков, принятых валидатором в модели.
	totals counts
}

// New создаёт отчётчик с нулевыми счётчиками и двумя ненулевыми потоками.
func New(output, errorOutput io.Writer) *Reporter {
	return &Reporter{output: output, errorOutput: errorOutput}
}

// Start печатает число зарегистрированных диагностик.
func (r *Reporter) Start(registered int) error {
	_, err := fmt.Fprintf(r.output, "Зарегистрировано диагностик: %d\n", registered)
	return err
}

// Record печатает одну строку отчёта для JSON: штатную — в output,
// технический отказ — в errorOutput с путём и причиной, включая V-код Check.
// Причина ошибки берётся из Err без потери обёрнутого сообщения.
// Обновляет счётчики даже при ошибке вывода; возвращает только ошибку записи.
// При отказе output пытается один раз сообщить об этом в errorOutput;
// отказ самого errorOutput возвращает без повторных попыток печати.
// Err!=nil или Updated=false означают технический отказ.
// Три уровня считаются по Result.Diagnostics с Source=ProcessingID;
// пустой ID означает нули, а не все исторические диагностики.
func (r *Reporter) Record(report FileReport) error {
	summary := summarizeFile(report)
	r.total++
	if summary.failed {
		r.failed++
	} else {
		r.completed++
	}
	if summary.updated {
		r.updated++
	}
	r.totals.errors += summary.counts.errors
	r.totals.problems += summary.counts.problems
	r.totals.recommendations += summary.counts.recommendations

	line := formatFileLine(summary)
	if summary.failed {
		_, err := fmt.Fprintln(r.errorOutput, line)
		return err
	}
	_, err := fmt.Fprintln(r.output, line)
	if err != nil {
		_, notificationErr := fmt.Fprintf(r.errorOutput, "Ошибка вывода отчёта для %q: %q\n", summary.path, err.Error())
		if notificationErr != nil {
			return errors.Join(err, notificationErr)
		}
	}
	return err
}

// Finish печатает итог в output и возвращает код 1 только при failed>0,
// иначе 0. Числа error, warning и recommendation на код не влияют.
// При ранней остановке итог охватывает только файлы с вызванным Record;
// ещё не открытые файлы не считаются обработанными.
// Ошибку вывода возвращает отдельно; повторно после Finish не вызывается.
func (r *Reporter) Finish() (code int, err error) {
	if r.failed > 0 {
		code = 1
	}
	_, err = fmt.Fprintln(r.output, formatTotalLine(r.total, r.completed, r.failed, r.updated, r.totals))
	return code, err
}

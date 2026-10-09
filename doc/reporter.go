// Package reporter печатает отчёт каждого JSON и общий итог текущего запуска.
// Различает ошибки, проблемы и рекомендации; технические отказы учитывает
// отдельно от срабатываний. Пакет не меняет модель и не сохраняет файлы.
// Числа диагностик относятся только к явно переданному ProcessingID.
// Срабатывания всех трёх уровней считаются штатным результатом и не влияют
// на код завершения. Сообщения о технических отказах отправляются в stderr.
package reporter

file report.go:

// FileReport — фактический результат одной попытки обработки JSON.
type FileReport struct {
	// Path — абсолютный путь JSON, включая файл, который не удалось открыть.
	Path string
	// Result — согласованная модель либо nil, если её нельзя использовать;
	// передаётся только на время Record и отчётчиком не сохраняется.
	Result *model.Result
	// ProcessingID — ID текущего запуска либо пустая строка до его начала.
	ProcessingID string
	// Updated — установлен ли дополненный JSON; не вычисляется по наличию ошибок.
	Updated bool
	// Err — первый технический отказ открытия, модели, Check, конвейера или записи.
	// Для сбоя Check содержит error с его V-кодом; в JSON не сохраняется.
	Err error
	// Duration — время до попытки сохранения, как Processing.DurationMs;
	// известно также при отказе чтения, когда модели ещё нет.
	Duration time.Duration
}

file reporter.go:

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
func New(output, errorOutput io.Writer) *Reporter

// Start печатает положительное число зарегистрированных диагностик.
// При пустом реестре app завершает запуск до создания Reporter.
func (r *Reporter) Start(registered int) error

// Record печатает одну строку отчёта для JSON: штатную — в output,
// технический отказ — в errorOutput с путём и причиной, включая V-код Check.
// Причина ошибки берётся из Err без потери обёрнутого сообщения.
// Обновляет счётчики даже при ошибке вывода; возвращает только ошибку записи.
// При отказе output пытается один раз сообщить об этом в errorOutput;
// отказ самого errorOutput возвращает без повторных попыток печати.
// Err!=nil или Updated=false означают технический отказ.
// Три уровня считаются по Result.Diagnostics с Source=ProcessingID;
// пустой ID означает нули, а не все исторические диагностики.
func (r *Reporter) Record(report FileReport) error

// Finish печатает итог в output и возвращает код 1 только при failed>0,
// иначе 0. Числа error, warning и recommendation на код не влияют.
// При ранней остановке итог охватывает только файлы с вызванным Record;
// ещё не открытые файлы не считаются обработанными.
// Ошибку вывода возвращает отдельно; повторно после Finish не вызывается.
func (r *Reporter) Finish() (code int, err error)

file summary.go:

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
func countDiagnostics(result *model.Result, source string) counts

// summarizeFile извлекает неизменяемые значения и причины технических отказов.
func summarizeFile(report FileReport) fileSummary

file format.go:

// formatFileLine формирует одну строку: ОБРАБОТАН или СБОЙ, путь JSON,
// сохранён: да/нет, строк: N/нет данных, ошибок: N, проблем: N,
// рекомендаций: N, время: N мс; при отказе добавляет причину.
// Пути и причины экранирует как %q, чтобы сохранялся однострочный формат.
// Эти числа означают полученные результаты даже при неудачной записи файла.
func formatFileLine(summary fileSummary) string

// formatTotalLine формирует ИТОГО с числами всего, обработано, сбоев,
// сохранено и суммами ошибок, проблем и рекомендаций.
func formatTotalLine(total, completed, failed, updated int, totals counts) string

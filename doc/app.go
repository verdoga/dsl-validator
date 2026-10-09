// Package app связывает параметры, discovery, реестр, storage, validator
// и reporter в одно последовательное приложение. Пакет владеет моделью
// текущего файла, добавляет Processing и измеряет время; не разбирает DSL,
// не реализует диагностики, не сериализует JSON и не обходит каталоги.
package app

file run.go:

const (
	// exitSuccess — штатное завершение при любом числе срабатываний трёх уровней.
	exitSuccess = 0
	// exitFailure — технический отказ валидатора/Check или ошибка подготовки/обхода.
	exitFailure = 1
	// exitUsage — неверные аргументы либо непригодный исходный путь.
	exitUsage = 2
	// toolName — стабильное имя валидатора в Processing.Tool.
	toolName = "dsl-validator"
)

// Run вызывает console.Parse, затем discovery.Find и diagnostics.NewRegistry
// с checkers.RegistrationCheckers(). Версия инструмента должна быть непустой.
// При ошибке аргументов возвращает 2, при справке — 0; повторно их не печатает.
// При ошибке обхода или реестра выводит причину в stderr и возвращает 1
// до открытия JSON; найденные при неудачном обходе пути не обрабатываются.
// После создания реестра проверяет Len: при нуле выводит в stderr
// «Ошибка: диагностики не зарегистрированы» и возвращает 1.
// Поиск файлов к этому моменту завершён, но storage.Open, startProcessing,
// validator.Prepare/Run и отчёты отдельных файлов не вызываются; JSON не меняются.
// При непустом реестре создаёт reporter.New с os.Stdout и os.Stderr,
// вызывает Start и последовательно processFile для каждого пути.
// Первый технический отказ processFile прекращает цикл с итоговым кодом 1;
// причина уже выведена Record в stderr и повторно здесь не печатается.
// После начатого цикла вызывает Finish ровно один раз, включая раннюю остановку;
// после отказа разрешены только закрытие ресурсов и вывод итогов.
// Пустой список файлов при непустом реестре допустим.
// Ошибки Start/Finish и конфигурации выводит в stderr и возвращает 1.
// Срабатывания error, warning и recommendation не влияют на код завершения.
// Не вызывает os.Exit и не создаёт горутины.
func Run(args []string, version string) int

file runner.go:

// runner хранит готовые зависимости одного запуска; состояния разных файлов
// и разных Check между собой не разделяются.
type runner struct {
	// version — непустая версия исполняемого инструмента.
	version string
	// registry — подготовленный и закрытый для регистрации реестр.
	registry *diagnostics.Registry
	// reporter — единый накопитель консольной статистики.
	reporter *reporter.Reporter
}

// processFile фиксирует начало перед единственным storage.Open и вызывает
// validator.Prepare до любого изменения модели. Версия и выборка checks
// определяются заново для каждого документа, независимо от остальных файлов
// и версии исполняемого инструмента. При отказе Prepare не вызывает
// startProcessing, validator.Run или Save: даже модель в памяти не меняется.
// После успешного Prepare вызывает startProcessing и validator.Run с checks.
// После успешного Run задаёт длительность через finishProcessing
// и при err=nil сохраняет результат через storage.Save.
// При первом сбое Check, включая *validator.CheckFailure, Save не вызывает:
// текущий JSON не меняется, в том числе не дописывается Processing.
// То же правило действует при ошибке открытия, версии, целостности или конвейера.
// При отказе длительность для отчёта вычисляет без изменения Processing.
// Отказ версии и отсутствие подходящих проверок передаёт Record как error;
// сообщение в stderr содержит путь, версию и причину; ProcessingID остаётся пустым.
// При любом отказе Prepare передаёт Record с Result=nil и Updated=false.
// В любом случае Record вызывается ровно один раз с фактическим Updated.
// Возвращает причину технического отказа, сохраняя тип CheckFailure и V-код;
// ошибку вывода Record при необходимости присоединяет через errors.Join.
// Ненулевой результат запрещает Run переходить к следующему файлу.
func (r *runner) processFile(path string) error

file processing.go:

// startProcessing добавляет ровно одну запись с новым ID, toolName,
// версией и StartedAt в UTC/RFC3339Nano; DurationMs до завершения равно nil.
// Вызывается только после успешного Prepare: версия поддерживается,
// выборка диагностик непуста и целостность модели проверена.
// Document, Lines и старые Processing не переписывает.
func startProcessing(result *model.Result, version string, startedAt time.Time) (processingID string)

// nextProcessingID выбирает свободный pN, N>=1, с учётом всей истории.
// ID уникален внутри данного JSON, глобальная уникальность не требуется.
func nextProcessingID(entries []model.Processing) string

// finishProcessing находит ровно текущий ID и устанавливает DurationMs
// в неотрицательных миллисекундах от startedAt до завершения конвейера.
// Чтение и закрытие JSON, подготовка по версии, контроль модели и Check
// входят в длительность;
// сохранение и консольный вывод не входят, как и в существующем парсере.
// Использует монотонную составляющую time.Time; переполнение int — ошибка.
// Возвращает ту же длительность для FileReport; отсутствие ID — ошибка.
func finishProcessing(result *model.Result, processingID string, startedAt time.Time) (duration time.Duration, err error)

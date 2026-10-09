// Package storage открывает и закрывает существующие JSON-файлы, распознаёт
// формат DSL и безопасно сохраняет дополненную модель по тому же пути.
// Пакет отвечает за JSON и файловый I/O; не запускает диагностики, не проверяет
// семантические инварианты ссылок, не считает уровни и не создаёт Processing.
// Неизвестные поля поддерживаемого формата сохраняются при записи.
package storage

file snapshot.go:

// Snapshot — закрытые сведения об исходном JSON для сохранения по тому же пути.
// Не является открытым дескриптором и не требует закрытия вызывающим кодом.
type Snapshot struct {
	// path — абсолютный путь открытого JSON, а не Document.FilePath исходного DSL.
	path string
	// original — собственные точные байты прочитанного JSON, включая неизвестные поля.
	original []byte
	// info — сведения об исходном обычном файле для проверки перед заменой.
	info os.FileInfo
}

file storage.go:

// Open проверяет обычный .json через console.PathInfo, открывает файл,
// полностью читает и закрывает его до возврата. Ошибка закрытия также возвращается.
// Распознаёт структуру Contract.jsonc 1.0 по содержимому, а не имени файла
// или Document.FilePath. Возвращает модель и связанный с ней Snapshot.
// При ошибке возвращает nil, nil, err и не изменяет файл.
func Open(path string) (result *model.Result, snapshot *Snapshot, err error)

// Save сохраняет согласованный result в существующий JSON из snapshot.
// Модель, snapshot и история старых данных не изменяются. Нужна ранее успешная
// validator.CheckModel; storage не повторяет контроль ссылок или hasErrors.
// Сериализует только разрешённые дополнения и производные флаги, сохраняя
// неизвестные поля. Сначала проверяет неизменность исходного файла и готовит
// полный временный файл в том же каталоге, затем заменяет исходный с резервом.
// updated=true означает, что новый JSON установлен, даже если очистка резерва
// завершилась ошибкой. updated=false означает, что новая версия не установлена.
// При невозможности восстановления старый JSON остаётся в резерве, чей путь
// входит в err. Прямого усечения исходного файла и параметра Replace нет.
func Save(snapshot *Snapshot, result *model.Result) (updated bool, err error)

file read.go:

// readFile получает байты и FileInfo из одного открытого дескриптора,
// сверяет тип с PathInfo и обязательно закрывает дескриптор на всех путях.
// Ошибки чтения и закрытия объединяет через errors.Join.
func readFile(path string) (source []byte, info os.FileInfo, err error)

// decodeResult принимает ровно один UTF-8 JSON-объект без JSONC-комментариев,
// BOM и хвоста после объекта; пробельный хвост допустим.
// Отклоняет повторные ключи, отсутствие обязательных полей, неверные JSON-типы
// и неподдерживаемый formatVersion. Неизвестные поля игнорирует семантически.
// Наличие DSL-версии 1.2 не является сигнатурой файла: null и неизвестная
// версия могут быть законным результатом парсера и сохраняются без подмены.
func decodeResult(source []byte) (*model.Result, error)

// checkJSONSyntax проверяет единственность корневого объекта, корректность
// UTF-8/Unicode-последовательностей и уникальность ключей на всех уровнях.
// Не допускает молчаливой замены повреждённого текста декодером JSON.
func checkJSONSyntax(source []byte) error

// checkRootShape проверяет наличие formatVersion, document, processing,
// lines, diagnostics и вызывает проверки форм вложенных объектов.
// Все обязательные массивы представлены [], а не null; исключение
// metadata.resourceDirs допускает null согласно контракту.
func checkRootShape(root map[string]json.RawMessage) error

// checkDocumentShape проверяет обязательные поля document и metadata,
// их JSON-типы и разрешённый null без проверки достоверности метаданных.
func checkDocumentShape(raw json.RawMessage) error

// checkProcessingShape проверяет обязательные поля и JSON-типы записей processing.
func checkProcessingShape(raw json.RawMessage) error

// checkLinesShape проверяет обязательные поля строк и элементов, включая
// наличие hasErrors, errorIds и допустимость null у parentLine/value.
func checkLinesShape(raw json.RawMessage) error

// checkDiagnosticsShape проверяет обязательные поля диагностик и диапазонов,
// включая сохранённое поле fatal; ID и координаты проверяет validator.
func checkDiagnosticsShape(raw json.RawMessage) error

file encode.go:

// encodeUpdate готовит полный JSON на основе snapshot.original.
// Меняет только новые суффиксы processing/diagnostics, document.hasErrors,
// lines[].hasErrors и дополненные elements[].errorIds.
// Предусловие: остальные известные поля и прежние записи не менялись.
// Неизвестные поля на любом уровне и значения прежних записей сохраняются.
// Результат имеет отступ два пробела и конечный LF; побайтовое форматирование
// всего исходного JSON не обещается. Экспортируемые строки не HTML-экранируются.
func encodeUpdate(snapshot *Snapshot, result *model.Result) ([]byte, error)

// appendProcessing добавляет к сырому массиву только новые Processing;
// прежние объекты, включая неизвестные поля, переносит без пересоздания.
func appendProcessing(original json.RawMessage, entries []model.Processing) (json.RawMessage, error)

// appendDiagnostics добавляет только новые Diagnostics, сохраняя прежние
// объекты и неизвестные поля; сокращение прежнего массива отвергает.
func appendDiagnostics(original json.RawMessage, entries []model.Diagnostic) (json.RawMessage, error)

// updateDocumentFlag заменяет только document.hasErrors в исходном объекте.
func updateDocumentFlag(original json.RawMessage, hasErrors bool) (json.RawMessage, error)

// updateLineFlags переносит флаги строк и дополненные ErrorIDs, сохраняя
// все остальные поля строк и элементов. Изменение количества строк/элементов
// отвергает; порядок и содержимое строк являются предусловием Save.
func updateLineFlags(original json.RawMessage, lines []model.Line) (json.RawMessage, error)

file write.go:

// checkUnchanged повторно проверяет путь без ссылок, идентичность обычного
// файла и точное совпадение его байтов с snapshot.original; при расхождении
// возвращает ошибку без записи. Дескрипторы открывает и закрывает внутри.
func checkUnchanged(snapshot *Snapshot) error

// writeTemporary создаёт временный файл в каталоге назначения, записывает
// полный payload, сохраняет доступные права исходного файла, вызывает Sync
// и закрывает его. При отказе удаляет незавершённый временный файл.
func writeTemporary(targetPath string, payload []byte, mode os.FileMode) (temporaryPath string, err error)

// replaceExisting резервирует старый файл и устанавливает подготовленный.
// При отказе установки восстанавливает прежний путь; ошибки восстановления
// и очистки сохраняет в err, не удаляя единственную пригодную копию данных.
// updated=true означает установленную новую версию; false — отсутствие установки.
// Не обещает атомарность нескольких переименований при аварийном отключении ОС.
func replaceExisting(temporaryPath, targetPath string) (updated bool, err error)

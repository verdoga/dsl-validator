// Package validator определяет версию уже открытой model.Result, выбирает
// применимые проверки и последовательно запускает их. Только этот пакет
// переносит срабатывания в Diagnostics, назначает ID, связывает элементы
// и вычисляет hasErrors.
// Пакет не читает файлы, не разбирает DSL, не печатает отчёт и не измеряет время.
// Контроль структуры JSON-модели не является повторной проверкой грамматики.
package validator

file prepare.go:

// Prepare определяет версию по Document.DSLVersion и вызывает Registry.ForVersion
// с её значением как dslversions.Version; nil передаётся как пустая версия.
// Не извлекает версию повторно из Raw и не подставляет текущую версию каталога.
// Неподдерживаемая, некорректная или неизвестная версия даёт error до полного
// контроля модели, копирования строк, создания Processing и вызовов Check.
// Отсутствие подходящих диагностик также даёт error с версией документа.
// При пригодной версии и непустой выборке вызывает CheckModel ровно один раз
// и возвращает checks в порядке V-кодов. nil result/registry дают error.
// При любой ошибке checks=nil. Модель и реестр не изменяет, I/O не выполняет.
func Prepare(result *model.Result, registry *diagnostics.Registry) (checks []diagnostics.Definition, err error)

file run.go:

// CheckFailure — технический сбой одной проверки, а не срабатывание DSL.
// Реализует error, содержит существующий V-код сбойной диагностики
// и не становится записью model.Diagnostic.
type CheckFailure struct {
	// Code — код проверки, которая не смогла завершиться.
	Code diagnostics.Code
	// Err — первая причина сбоя Check или нарушения им API срабатываний.
	Err error
}

// Error возвращает сообщение «сбой диагностики V001: <причина>» с фактическим
// Code. Сообщение сохраняет идентификатор при обычном оборачивании через %w.
func (e *CheckFailure) Error() string

// Unwrap возвращает исходную причину для errors.Is и errors.As.
func (e *CheckFailure) Unwrap() error

// Run принимает модель и неизменённую выборку checks из успешного Prepare
// для этого же документа; с момента Prepare app добавляет только Processing.
// Проверяет текущий Processing, создаёт закрытое представление Lines
// и выполняет Check в порядке checks. Повторно версию и реестр не просматривает,
// полный контроль входа не дублирует; итоговый CheckModel остаётся обязательным.
// Каждый Check получает новый накопитель. При первом техническом сбое
// возвращает *CheckFailure с V-кодом, отбрасывает срабатывания сбойной проверки
// и не запускает оставшиеся Check. app прекращает также обработку следующих файлов.
// Успешные срабатывания применяются к отдельной копии изменяемых частей модели;
// result обновляется только после успеха всех Check и итогового контроля.
// При err!=nil result не меняется; app не сохраняет этот файл.
// Любое число error/warning/recommendation не создаёт возвращаемую ошибку.
// nil result либо пустая выборка даёт err без изменения result; штатный путь
// app до Run отсекает пустой реестр, неподдерживаемую версию и пустую выборку.
func Run(result *model.Result, checks []diagnostics.Definition, processingID string) error

// checkCurrentProcessing проверяет, что processingID — ID последней записи,
// что он уникален и ещё не используется диагностиками; время задаёт app.
func checkCurrentProcessing(result *model.Result, processingID string) error

// runCheck выполняет одну Check с новым collector и закрывает накопитель.
// При первой ошибке возвращает nil и *CheckFailure с Code текущего Definition;
// более ранняя ошибка накопителя имеет приоритет перед ошибкой возврата Check.
// Непредвиденный panic только внутри вызова Check преобразует в такой же отказ:
// это обработка дефекта с остановкой, а не штатный способ возврата ошибок.
// Ни частичные срабатывания, ни сам сбой в модель не переносятся. При успехе заполняет
// Code, Severity, Scope, Message и Fatal=false по Definition, но ещё не ID/Source.
func runCheck(lines *linesView, definition diagnostics.Definition) ([]model.Diagnostic, error)

file lines_view.go:

// linesView — внутренняя неизменяемая проекция только физических строк.
type linesView struct {
	// lines — собственные глубокие копии строк; HasErrors=false и ErrorIDs=nil.
	// Метаданные документа, история и диагностики никогда сюда не передаются.
	lines []model.Line
}

// newLinesView копирует строки и изменяемые вложенные значения.
// Внутри validator возвращает закрытый тип; в Check он передаётся только
// как diagnostics.Lines. Исходные срезы и указатели не удерживает.
func newLinesView(lines []model.Line) *linesView

// Len возвращает количество строк проекции.
func (v *linesView) Len() int

// Line возвращает закрытую обёртку строки; вне 1..Len возвращает nil, false.
func (v *linesView) Line(number int) (line diagnostics.Line, found bool)

// lineView скрывает модель строки и предоставляет только скалярные значения.
type lineView struct {
	// line — строка приватной проекции без сведений об ошибках.
	line model.Line
}

// Number возвращает физический номер строки.
func (v lineView) Number() int

// Type возвращает каноническое строковое значение LineType.
func (v lineView) Type() string

// NestingLevel возвращает глубину строки.
func (v lineView) NestingLevel() int

// ParentLine возвращает значение родителя; false и 0 означают корень.
func (v lineView) ParentLine() (number int, found bool)

// Raw возвращает точный текст строки без изменения экранирования.
func (v lineView) Raw() string

// EOL возвращает исходное окончание строки.
func (v lineView) EOL() string

// ElementCount возвращает количество элементов.
func (v lineView) ElementCount() int

// Element возвращает закрытую обёртку элемента; неверный индекс даёт nil, false.
func (v lineView) Element(index int) (element diagnostics.Element, found bool)

// elementView скрывает модель элемента и не предоставляет ErrorIDs.
type elementView struct {
	// element — элемент приватной проекции без ссылок на ошибки.
	element model.Element
}

// Type возвращает каноническое строковое значение ElementType.
func (v elementView) Type() string

// Raw возвращает точный фрагмент исходной строки.
func (v elementView) Raw() string

// Value возвращает копию значения; false означает null, true — наличие строки.
func (v elementView) Value() (text string, present bool)

// Start возвращает включаемую Unicode-колонку с 1.
func (v elementView) Start() int

// End возвращает исключающую Unicode-колонку.
func (v elementView) End() int

file collector.go:

// collector накапливает срабатывания ровно одного синхронного вызова Check.
// Диагностике передаётся только интерфейс diagnostics.Findings.
type collector struct {
	// lines — строки очищенной проекции linesView для проверки координат;
	// исходные HasErrors/ErrorIDs отсутствуют, наружу срез не выдаётся.
	lines []model.Line
	// scope — неизменяемая область из определения текущей проверки.
	scope diagnostics.Scope
	// occurrences — предварительные записи без глобальных ID и Source.
	occurrences []model.Diagnostic
	// failure — первая ошибка API, даже если Check проигнорировала её;
	// после неё никакие новые срабатывания или связанные места не принимаются.
	failure error
	// closed — запрет любых вызовов после окончания Check.
	closed bool
}

// newCollector создаёт пустой накопитель; получает только очищенные строки
// из linesView, без доступа к исходной модели и её старым диагностикам.
func newCollector(lines []model.Line, scope diagnostics.Scope) *collector

// Add проверяет диапазон и область, затем добавляет отдельное срабатывание.
// Возвращает локальный номер с 1; при ошибке возвращает 0, сохраняет failure
// и не добавляет запись. Не меняет исходную модель и не создаёт JSON ID.
// При уже установленной failure сразу возвращает её без изменения накопителя.
func (c *collector) Add(startLine, startColumn, endLine, endColumn int, message string) (occurrence int, err error)

// AddRelated проверяет существование локального occurrence в этом накопителе,
// координаты и отсутствие дубликата, затем дополняет только эту запись.
// При отказе сохраняет failure; вызов на закрытом накопителе также запрещён.
// Установленную ранее failure не заменяет и новых связанных мест не добавляет.
func (c *collector) AddRelated(occurrence, startLine, startColumn, endLine, endColumn int) error

// finish закрывает накопитель и возвращает глубокую копию записей либо
// nil и накопленную ошибку. Повторный вызов считается ошибкой API.
func (c *collector) finish() ([]model.Diagnostic, error)

file ranges.go:

// makeLocation переводит четыре числа в существующий model.Location.
// Четыре нуля дают nil; частично нулевые и отрицательные координаты — ошибку.
// Допустимость nil для конкретной области проверяет checkOccurrence.
func makeLocation(startLine, startColumn, endLine, endColumn int) (*model.Location, error)

// checkLocation проверяет существование строк, диапазон Unicode-колонок,
// порядок позиций и исключающую правую границу; пустая позиция допустима.
func checkLocation(lines []model.Line, location model.Location) error

// checkOccurrence проверяет обязательность Location, соответствие Scope,
// связанные диапазоны и точное совпадение с элементом для ScopeElement.
// Для новых ScopeLine и ScopeBlock отсутствие Location запрещено.
func checkOccurrence(lines []model.Line, scope diagnostics.Scope, occurrence model.Diagnostic) error

file apply.go:

// copyForUpdate копирует Result и все изменяемые здесь вложенные части:
// Diagnostics с диапазонами, Lines, Elements и ErrorIDs.
// Неизменяемые Document-поля и Processing не переписывает; общей изменяемой
// памяти с исходной моделью в перечисленных частях не оставляет.
func copyForUpdate(result *model.Result) *model.Result

// appendOccurrences назначает ID и Source новым записям, добавляет их
// к кандидату и формирует обратные ссылки только для ScopeElement.
// Диагностики прежних запусков и их ID не изменяет и не удаляет.
func appendOccurrences(result *model.Result, processingID string, occurrences []model.Diagnostic) error

// nextDiagnosticID выбирает свободный dN, N>=1, среди всех Diagnostics;
// номер запуска не используется как предположение об отсутствии совпадений.
func nextDiagnosticID(entries []model.Diagnostic) string

// linkElementError добавляет ID к существующему элементу с точным диапазоном.
// Для одинаковых пустых диапазонов выбирает первый подходящий элемент.
// Повторную ссылку не добавляет; отсутствие элемента — ошибка инварианта.
func linkElementError(lines []model.Line, diagnostic model.Diagnostic) error

// orderNewDiagnostics устойчиво упорядочивает только новые записи по позиции:
// nil Location первой, затем начальная строка, колонка и порядок обнаружения.
// Прежний префикс Diagnostics сохраняет; ErrorIDs упорядочивает по итоговому
// порядку Diagnostics без изменения существующих ID и их относительного порядка.
func orderNewDiagnostics(result *model.Result, processingID string)

// setErrorFlags вычисляет Document.HasErrors и Line.HasErrors по всем запускам.
// Только severity=error устанавливает признак. Строка отмечается по началу
// основной Location либо её ErrorIDs; связанные места, родители и дети не влияют.
func setErrorFlags(result *model.Result)

file integrity.go:

// CheckModel проверяет целостность модели без изменения данных и без
// запуска диагностик: обязательные массивы, допустимые значения, ID, ссылки,
// координаты, порядок, связанные места и точность вычисленных hasErrors.
// Применяется Prepare до добавления Processing и Run к итоговой копии модели.
// Существующие DSL-ошибки сами по себе не запрещают валидацию.
// Нарушенные инварианты не исправляются молча и не выдаются за DSL-диагностику.
func CheckModel(result *model.Result) error

// checkDocument проверяет Format, значения и взаимную согласованность полей
// Document, включая null, кодировку, неотрицательные размеры и форму SHA256.
// Не открывает исходный DSL и не проверяет существование Document.FilePath.
func checkDocument(result *model.Result) error

// checkProcessing проверяет непустые уникальные ID, Tool, формат известного
// StartedAt с часовым поясом и неотрицательные известные DurationMs.
func checkProcessing(entries []model.Processing) error

// checkLineSequence проверяет обязательный массив Lines, нумерацию 1..N,
// согласованность известного LineCount, типы строк, UTF-8, Raw и допустимый EOL.
// При неизвестном LineCount разрешён только пустой Lines; это не выдуманный
// пустой исходник, и диагностики должны уметь обрабатывать отсутствие строк.
func checkLineSequence(result *model.Result) error

// checkParentLinks проверяет предшествующего родителя, глубину и корневые
// заголовки. У block-end непустой родитель обязан иметь тип block-start.
// Сохранённая парсером ошибочная корневая block-end допустима только при
// существующей основной диагностике error на этой строке; коды P не зашиваются.
// Не восстанавливает дерево и не повторяет грамматический стек парсера.
func checkParentLinks(result *model.Result) error

// checkElementPositions проверяет типы, допустимость Value, координаты Raw
// по кодовым точкам Unicode, порядок и отсутствие пересечений элементов.
// invalid требует unparsed, а unparsed — null Value и существующую ссылку;
// семантические ограничения конкретных DSL-тегов здесь не проверяются.
func checkElementPositions(line model.Line) error

// checkDiagnosticSources проверяет непустые уникальные Diagnostic.ID,
// существующие Source и непустые Code без ограничения старых кодов префиксом V.
// Проверяет Severity/Scope и правило Fatal=true только при severity=error.
func checkDiagnosticSources(result *model.Result) error

// checkDiagnosticRanges проверяет Location, RelatedLocations, отсутствие
// повторов и допустимость области по существующим правилам JSON-контракта.
// Старые неэлементные записи могут иметь Location=nil, как допускает контракт.
func checkDiagnosticRanges(result *model.Result) error

// checkElementLinks проверяет существование, уникальность и порядок ErrorIDs,
// область element и точное совпадение диапазона; каждая element-диагностика
// обязана иметь хотя бы одну обратную ссылку.
func checkElementLinks(result *model.Result) error

// checkDiagnosticOrder проверяет порядок по Processing и позиции,
// не переставляя записи; любой существующий порядок равных позиций допустим.
func checkDiagnosticOrder(result *model.Result) error

// checkErrorFlags проверяет эквивалентность hasErrors условию контракта,
// не исправляя исходную модель; как setErrorFlags, использует errorFlags.
func checkErrorFlags(result *model.Result) error

// errorFlags вычисляет ожидаемый признак документа и номера ошибочных строк
// по основной Location и ErrorIDs только записей error. Модель не изменяет;
// true означает наличие ошибки, false — её отсутствие. Карта принадлежит вызывающему.
func errorFlags(result *model.Result) (documentHasErrors bool, lineHasErrors map[int]bool)

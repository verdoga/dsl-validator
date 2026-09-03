# Аудит реализации `dsl-validator` относительно `TZ.md`

## 1. Объект и границы проверки

- Репозиторий: [verdoga/dsl-validator](https://github.com/verdoga/dsl-validator).
- Проверенное состояние: ветка `main`, коммит [`188a8e03aa4706c5bd5ff475a40d9b75e1c06e00`](https://github.com/verdoga/dsl-validator/commit/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00) от 2026-09-03.
- Источник требований: [`TZ.md`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/TZ.md).
- Проверены все production-файлы и тесты валидатора: корень, `validatorapi`, `Diagnostics`, `internal/parseradapter`, `internal/workspace`, `internal/pipeline`, `internal/report`, `internal/webapp`, `web`, `cmd/validator`, schema и документация.
- Внутренняя логика `third_party/dslparser/**` не оценивалась. Публичные типы парсера просматривались только для проверки корректности адаптера.
- Проверка является статической: в рабочем окружении отсутствует исполняемый файл `go`, поэтому команды сборки, тестов, race detector и `go vet` повторно выполнить невозможно. Это ограничение проверки, а не доказательство успешности или неуспешности сборки.

## 2. Итоговая оценка

Текущий код не соответствует критериям приёмки `TZ.md`. Это частичный прототип инфраструктуры:

- некоторые основы реализованы правильно: loopback-адрес фиксирован, AST скрыт за интерфейсами, production-реестр диагностик пуст, файлы обрабатываются одной фоновой горутиной последовательно, parser issues добавляются только из документа;
- отсутствуют целые обязательные подсистемы: обнаружение и сопоставление существующих отчётов, полноценный report view, фильтрация и сортировка, строгая schema v1, контекст записей, значительная часть состояния и UI;
- есть блокирующие нарушения зависимости и Windows-надёжности;
- тесты покрывают малую долю обязательной матрицы и не способны подтвердить заявленную готовность.

Ниже перечислены 73 самостоятельных дефекта. Приоритеты: **блокирующий** — критерий приёмки принципиально не выполняется; **высокий** — потеря корректности/надёжности либо крупная недореализация; **средний** — существенный дефект качества, тестируемости или документации.

## 3. Реестр обнаруженных дефектов

### A. Сборка, зависимость и архитектурные границы

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 1 | Блокирующий | [`go.mod:3`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/go.mod#L3) | Указан Go `1.25.1`, тогда как §§2, 23 `TZ.md` и `AGENTS.md` требуют Go `1.26`. |
| 2 | Блокирующий | [`go.mod:7–8`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/go.mod#L7-L8), `third_party/dslparser/**` | Парсер подключён через `replace ... => ./third_party/dslparser`, а его исходники скопированы в проект. §2 прямо запрещает `replace`, vendoring и копию парсера; §24 требует внешний закреплённый модуль. Псевдоверсия в `require` фактически не определяет собираемый код, потому что её заменяет локальный каталог. |
| 3 | Блокирующий | корень репозитория | Файл `go.sum` отсутствует, хотя §2 требует зафиксированные контрольные суммы внешнего модуля. |
| 4 | Высокий | тесты проекта | Нет обязательного архитектурного теста на `go/parser`/`go/ast`, который запрещает импорт парсера вне `internal/parseradapter` и импорты parser/adapter из `Diagnostics` (§4). Текущее соблюдение границы держится только на ручной дисциплине. |
| 5 | Высокий | [`README.md:9`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/README.md#L9) | Документация не отмечает `replace` как незавершённую блокировку, а предписывает использовать запрещённый ТЗ локальный снимок. Это делает неверную сборочную схему официальным пользовательским сценарием. |

### B. `validatorapi`, адаптер и навигация AST

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 6 | Высокий | [`parseradapter.Adapter.parseReader`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/adapter.go#L50-L74) | Результат с одновременно ненулевыми `Document` и `error` классифицируется по типу ошибки: `FatalError` станет fatal, иной error — `PARSER_UNEXPECTED_ERROR`. По матрице §14 любая пара «документ + ошибка» должна прежде всего стать `PARSER_CONTRACT_FAILURE`. Проверка `document != nil && err != nil` отсутствует. |
| 7 | Высокий | [`elementView.Kind`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/adapter.go#L124-L128), [`nodeView.Kind`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/adapter.go#L145-L149) | Виды отображаются индексированием литерала-среза. Любое неизвестное/повреждённое enum-значение вызывает `index out of range` уже после выхода из parser-boundary `recover`; паника может быть ошибочно приписана диагностике или обрушить иной потребитель. Нужен явный `switch` с контролируемым contract failure. |
| 8 | Средний | [`blockView.Mode`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/adapter.go#L107-L122) | Любое значение режима, отличное от `BodyStructural`, молча преобразуется в `opaque`. Неизвестное значение внешнего контракта не обнаруживается, а искажает AST-представление. |
| 9 | Высокий | [`adapter_test.go`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/adapter_test.go) | Тесты проверяют четыре fatal-случая и лишь наличие нескольких physical nodes. Нет 16 обязательных local fixtures, проверки `RelatedSpan`, регистра, блоков, элементов, всех вариантов contract failure и parser panic (§§14, 22). |
| 10 | Высокий | `internal/parseradapter` tests | Нет теста полного совпадения всех 20 `validatorapi.ParserCode` с экспортированным каталогом закреплённого парсера, хотя это прямо требует §6.1. Изменение кода в зависимости останется незамеченным. |
| 11 | Средний | [`contextView.Walk`, `find`, `cursorView.Children`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/context.go) | Навигация построена с многократным линейным поиском по всем cursor: `Walk` делает `find` для каждого узла, `Children` снова сканирует весь список, `Ancestors` повторяет `find`. Полные обходы имеют квадратическую сложность. При отсутствии лимита размера файла (§3.2) это некачественная и потенциально непригодная для крупных AST реализация. |
| 12 | Высокий | [`context.go`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/parseradapter/context.go), tests | Не проверены наблюдаемые контракты `Walk`, pruning по `false`, `Path`, `Depth`, parent/siblings/ancestors, стабильный physical order, поиск по kind/name и точное совпадение `HasParserIssue` (§§6.3, 22). |
| 13 | Средний | [`validatorapi/api.go`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/validatorapi/api.go), другие Go-файлы | Нарушена обязательная политика документации: у полей структур и большинства методов интерфейсов нет собственных русских комментариев; комментарии групп констант не начинаются с точных идентификаторов. Отсутствует и требуемый AST-тест комментариев (§23, `AGENTS.md`). |

### C. Реестр диагностик и исполнение диагностик

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 14 | Высокий | [`diagnostics.validate`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/Diagnostics/registry.go#L40-L68) | Валидируется принадлежность version scope списку `supported`, но сам список поддерживаемых версий не проверяется на пустые, неканонические и повторяющиеся значения. Требование уникальности и каноничности версий реализовано неполно. |
| 15 | Средний | [`diagnostics.validate`: проверка паспорта](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/Diagnostics/registry.go#L43-L49) | «Непустые» поля проверяются только на `== ""`; строки из пробелов принимаются как валидные код, категория, заголовок и остальные обязательные поля. Для машинного кода нет проверки допустимой/канонической формы. |
| 16 | Высокий | [`All`, `ForVersion`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/Diagnostics/registry.go#L9-L24) | Инфраструктура не обеспечивает глубокие защитные копии `Definition.Versions.Versions`. После появления определений вызывающий сможет менять срез области версии, нарушая read-only ownership и результаты следующих вызовов. |
| 17 | Высокий | [`ForVersion`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/Diagnostics/registry.go#L16-L24) | Для неподдерживаемой версии функция всё равно вернёт все определения с `All: true`. По §11 «all» означает все поддерживаемые версии успешно построенного документа, а не произвольную строку версии. |
| 18 | Высокий | пакет `Diagnostics`, tests | Нет ни одного test-only определения и ни одного теста реестра: не проверены одна/несколько/все версии, порядок, неполный паспорт, конфликт областей, повторы версий и nil `Check` (§§7, 22). |
| 19 | Высокий | архитектурные/policy tests | Запрет диагностике на filesystem, network, goroutine и global mutation (§6.4) никак не контролируется. Нет хотя бы статической проверки импортов/запрещённых конструкций для `Diagnostics/**`; заявленный запрет остаётся текстом. |

### D. Рабочий каталог и сканирование

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 20 | Блокирующий | [`workspace.ValidateRoot`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/workspace/workspace.go#L44-L71) | Корень проверяется только по `os.ModeSymlink`. Windows junction и прочие `FILE_ATTRIBUTE_REPARSE_POINT` не обнаруживаются, хотя §10 прямо требует проверять Windows-атрибут. |
| 21 | Блокирующий | [`Scanner.Scan`: обход](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/workspace/workspace.go#L87-L103) | Та же ошибка внутри дерева: пропускаются только POSIX-style symlink entries. Junction/reparse directory может быть пройден, что нарушает границы workspace и способно увести рекурсивный обход за выбранный каталог. |
| 22 | Высокий | [`ValidateRoot:66`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/workspace/workspace.go#L62-L70) | EOF распознаётся сравнением `err.Error() != "EOF"`, вопреки правилам проекта и идиоматике Go. Обёртка/локализация ошибки изменит поведение; требуется `errors.Is(err, io.EOF)`. |
| 23 | Средний | [`ValidateRoot:62–71`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/workspace/workspace.go#L62-L71) | Ошибка `Close` дескриптора каталога игнорируется. Код объявляет каталог доступным, даже если завершение операции ввода-вывода сообщило ошибку. |
| 24 | Блокирующий | пакет `internal/workspace` | Полностью отсутствует обнаружение файлов, чьё имя без учёта регистра оканчивается на `report.json` (§12). Scanner ищет только `.txt`. |
| 25 | Блокирующий | пакет `internal/workspace` | Не реализованы строгая типизированная проверка кандидатов и причины `REPORT_READ_FAILURE`, `REPORT_INVALID_JSON`, `REPORT_SCHEMA_UNSUPPORTED`, `REPORT_SCHEMA_INVALID`, `REPORT_SOURCE_MISMATCH`. |
| 26 | Блокирующий | пакет `internal/workspace` | Не реализованы выбор самого нового активного отчёта, tie-break по mtime/path, `REPORT_DUPLICATE`, `REPORT_ORPHAN` и отдельный список отклонённых отчётов (§12). |
| 27 | Высокий | [`workspace.File`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/workspace/workspace.go#L14-L22) | Модель файла не содержит состояния существующего отчёта, времени последней проверки и ID активного отчёта. Обязательные данные таблицы и report route невозможно выразить. |
| 28 | Высокий | [`pipeline.Runner.Version`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L24-L36) → `Scanner.Scan` | Parser panic, contract failure и unexpected error превращаются в `("", false, nil)`: Scanner молча показывает «версия не определена» и не фиксирует восстановимую validator-ошибку. Реальная причина теряется. |
| 29 | Высокий | [`workspace_test.go:31–41`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/workspace/workspace_test.go#L31-L41) | Если `os.Symlink` не разрешён (типично на Windows без привилегий), тест молча ничего не проверяет и проходит. Junction/reparse points, недоступные ветви, `FILE_INFO_FAILURE` и продолжение обхода не тестируются. |

### E. Последовательный конвейер

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 30 | Высокий | [`Runner.Run`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L38-L91) | Обязательная стадия повторного `stat` перед открытием отсутствует. Актуальный размер не получается, `FILE_INFO_FAILURE` не может возникнуть и минимальный отчёт для этой ветви не формируется (§13, §15). |
| 31 | Высокий | [`Runner.Run:55–59`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L55-L59) | Ошибка закрытия уже открытого файла маркируется `FILE_OPEN_FAILURE`. Это неверная стадия и код; в результате отчёт сообщает, что файл не открыт, хотя parsing уже выполнен. |
| 32 | Высокий | [`report.New`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L109-L112), [`Report.Validate`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L189-L209) | Не обеспечена связка режима и selected version: `auto` с ненулевым `selected_version` и `explicit` с `null` проходят валидацию, вопреки §17. |
| 33 | Высокий | [`Runner.Run:73–75`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L71-L75) | Исключение по версии возвращается не типизированным исходом, а произвольной русской строкой ошибки. Это смешивает штатный результат выбора версии с ошибкой выполнения и ломает стабильный контракт между pipeline и webapp. |
| 34 | Высокий | [`webapp.run:202–212`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L202-L212) | Webapp распознаёт исключение через `strings.Contains(err.Error(), "исключена выбором")`. Любое изменение текста/обёртка меняет счётчик на `incomplete`; это прямо запрещённое сравнение ошибок по сообщению. |
| 35 | Блокирующий | [`addParser`, `findingReporter.Report`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L93-L143) | Поле `context` не строится ни для parser, ни для diagnostic entries. Требуемое построение из `Node.Raw`, body elements и block close span (§17) отсутствует целиком. |
| 36 | Высокий | [`Runner`, `runDefinitions`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go) | Runner жёстко обращается к production-глобальному `diagnostics.ForVersion`. Нет предусмотренного способа подать test-only реестр/definitions и проверить требуемые error, panic, invalid finding, порядок и продолжение следующих функций без модификации production-кода. |
| 37 | Высокий | [`findingReporter.Report`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L117-L143) | Валидность finding сводится к непустому сообщению и геометрически корректному `Span`. Не проверяется, что related span не существует без основного, а диапазоны принадлежат доступной области документа; произвольные координаты диагностики попадут в «валидный» отчёт. |
| 38 | Высокий | [`Runner.Run:86–90`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/pipeline/pipeline.go#L86-L90) | Ошибка serialization/write возвращается наружу без типизированного operation result. Webapp считает любую такую ошибку `incomplete`, хотя модель анализа может иметь статус `completed`/`completed_with_errors`; итоговые счётчики перестают описывать фактическую стадию. |
| 39 | Высокий | пакет `internal/pipeline` | Тестов pipeline нет вообще. Не проверены fatal/no diagnostics, local/continue, единственное добавление parser issues, version selection, порядок, dedup, diagnostic failures, статусы и минимальные отчёты (§§13–16, 22). |

### F. Модель, schema, чтение и атомарная запись отчёта

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 40 | Блокирующий | [`schema/report-v1.schema.json`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/schema/report-v1.schema.json) | Schema фактически является заглушкой: `analysis` и `summary` — любые объекты, `entries` — любой массив. Нет required/enum/nullability/range/date-time/location/context/source-specific правил. Такая schema не описывает формат §17 и принимает почти любой мусор. |
| 41 | Блокирующий | [`Report.Validate`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L189-L209) | Runtime-валидация проверяет лишь schema version, путь/name, mode, total и несколько полей entry. Не проверяются DSLVersion, analysis status/failed_stage/times/duration/timezone, severity/source enums, nullable-поля и контекст. Повреждённый или чужой по семантике JSON может быть признан активным и разрешён к перезаписи. |
| 42 | Блокирующий | `Report.Validate` | Не проверяется правило паспортов: у `source: diagnostic` category/title/description/basis/reference/bad_example обязательны, у parser/validator должны быть `null`. Источники могут смешиваться на уровне полей, нарушая §17 и §24. |
| 43 | Высокий | `Report.Validate` | Сводка не пересчитывается и не сопоставляется с entries: проверяется только `summary.total`. `by_level`, `by_source` и `has_errors` могут противоречить записям и всё равно пройти `DecodeStrict`. |
| 44 | Высокий | `Report.Validate` | Не проверены инварианты статуса: допустимые enum, `failed_stage == null` для completed statuses, разрешённая стадия для incomplete и согласованность статуса с error entries. |
| 45 | Высокий | `Report.Validate` | Не проверены `Location.EndExclusive == true`, валидность координат, `related_location`, роли/номера context lines. Некорректные диапазоны могут попасть в UI и сортировку. |
| 46 | Высокий | [`DecodeStrict:212–225`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L211-L225) | После первого `Decode` вызывается `Decoder.More()`, предназначенный для элементов массива/объекта, вместо второго `Decode` с ожиданием `io.EOF`. Строгое отклонение второго JSON-значения или хвоста не реализовано надёжно. |
| 47 | Высокий | [`WriteAtomic:240–247`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L239-L247) | При проверке принадлежности существующего отчёта пути сравниваются регистрозависимо на всех ОС. §12 требует case-insensitive сравнение на Windows. Валидный отчёт того же файла с иной капитализацией пути будет объявлен конфликтом. |
| 48 | Блокирующий | [`replace_windows.go`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/replace_windows.go) | Windows-реализация просто вызывает `os.Rename`. §18 требует `MoveFileExW` с `MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH`. Не выполнены ни гарантированная замена существующего файла, ни требуемая write-through семантика. |
| 49 | Высокий | [`report.New`, `Finish`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L109-L148) | `analysis.timezone` нигде не определяется и всегда остаётся `null`, даже когда `time.Now().Location()` даёт известную системную зону. §17 требует имя зоны, если его удалось определить. |
| 50 | Средний | [`report.New:110–111`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L109-L112) | Путь не приводится к абсолютному очищенному виду внутри модели, а `SelectedVersion` сохраняется как переданный указатель без копирования. Корректность и ownership зависят от каждого вызывающего. |
| 51 | Высокий | [`WriteAtomic`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model.go#L234-L280) | Между проверкой существующего target и rename есть TOCTOU-окно: иной процесс может подменить target, после чего валидатор перезапишет уже не проверенный файл. Для требования «не перезаписывать чужой/повреждённый JSON» защита неполна. |
| 52 | Высокий | [`model_test.go`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/report/model_test.go) | Проверены только базовая сортировка и один конфликт. Нет fault injection для create/write/sync/close/replace, проверки сохранности старого файла, Windows replacement, schema, всех nullable/enum/summary/status инвариантов (§§17, 18, 22). |

### G. HTTP-состояние, безопасность и пользовательский интерфейс

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 53 | Высокий | [`App.Handler:81–82`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L81-L82) | Ошибка `fs.Sub` для assets игнорируется. Повреждение embed-layout не становится `ASSET_INIT_FAILURE` при запуске, а проявится позже как panic/500 на запросе. |
| 54 | Высокий | [`webapp.New`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L55-L66) | Startup-проверка читает только `base.html`; наличие/доступность обязательных CSS и JS не валидируется до bind/interface. Критерий встроенных и корректно инициализированных assets не подтверждается. |
| 55 | Высокий | [`middleware:95–99`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L95-L99) | Panic handler теряет значение panic, ничего не отправляет в console sink и возвращает plain-text `http.Error`, а не доступную HTML 500-page. Если headers/body уже записаны, попытка заменить ответ ненадёжна. Нарушены требования для `HTTP_HANDLER_PANIC`. |
| 56 | Высокий | [`App.render:113–117`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L113-L117) | Template выполняется прямо в `ResponseWriter`; при ошибке получается частично записанная страница, после которой вызывается `http.Error`. Ошибка не логируется как `TEMPLATE_RENDER_FAILURE`. Нужен render в буфер до отправки status/body. |
| 57 | Высокий | [`workspace handler:145–151`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L145-L151) | Любое значение version, кроме точной строки `auto`, принимается как explicit version. Произвольная/пустая/неподдерживаемая версия не даёт `INVALID_REQUEST`. |
| 58 | Высокий | [`workspace handler:140–143`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L140-L143) | Ошибка пути возвращается со статусом HTTP 200 на отдельной странице; введённые path/version и само поле теряются. Нет field error, связанного с input через `aria-describedby`, как требует §§15, 20.1. |
| 59 | Блокирующий | [`workspace handler:152–156`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L152-L156), [`App.run:188–202`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L188-L202) | Новый `POST /workspace` разрешён во время анализа и заменяет `a.scan`. Фоновая операция хранит индексы старого scan, но читает `a.scan.Files[index]` из нового: возможны анализ не того файла и `index out of range`; panic фоновой goroutine завершит весь процесс. Mutex защищает отдельные обращения, но не целостность snapshot. |
| 60 | Высокий | [`analysisAll`, `analysisFile`, `cancelAnalysis`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L233-L289) | Маршруты доступны до успешного выбора workspace. `/analysis/all` запускает фиктивную операцию из нуля файлов, cancel меняет состояние при отсутствии операции. Нет `INVALID_REQUEST` для недопустимых state transitions. |
| 61 | Блокирующий | [`files handler`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L159-L173) | Страница не показывает выбранный каталог, режим версии, сводку сканирования, имя отдельно, состояние/время отчёта, scan issues и таблицу отклонённых/конфликтных отчётов (§20.2). Неопределённая версия отображается пустой строкой вместо «Не определена». |
| 62 | Высокий | [`files handler:168–170`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L168-L170) | Для explicit mismatch кнопка отдельного анализа остаётся активной и не имеет текстового объяснения, хотя §11 требует недоступное действие. |
| 63 | Блокирующий | `files handler` | Действие «Открыть отчёт» при наличии активного валидного отчёта отсутствует полностью. |
| 64 | Высокий | [`analysisFile:246–264`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L246-L264) | `ParseForm` вызывается без `MaxBytesReader`, а ошибка игнорируется. Большое/повреждённое form body не ограничено и не даёт гарантированный `INVALID_REQUEST`; ограничение есть только у `/workspace`. |
| 65 | Блокирующий | [`progressPage`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L266-L271) | Серверная страница не отображает snapshot текущей операции и обязательные счётчики/current/status. Без JavaScript видна только «Подготовка…», что нарушает server-side/no-JS fallback §§20.3, 21. |
| 66 | Высокий | [`web/assets/app.js`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/web/assets/app.js) | Каждый poll без сравнения события снова присваивает live-region тот же текст; screen reader может получать повторные объявления каждые 700 мс. Нет обработки network/JSON errors, поэтому один reject останавливает polling. |
| 67 | Высокий | [`progressPage:270`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L270) | «Вернуться к списку» — обычный GET `/files`. §19–20 требует новое сканирование, чтобы инвалидировать старые opaque IDs и увидеть изменения/новые отчёты. |
| 68 | Блокирующий | [`reportPage`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L291-L296) | Страница отчёта не реализована: ID не извлекается и не разрешается, JSON не загружается, любой `/report/... ` показывает одинаковую заглушку. Старые/произвольные IDs не отклоняются. |
| 69 | Блокирующий | `internal/webapp`, `web/assets/app.js` | Отсутствуют таблица entries, фильтры level/source/category, серверная сортировка, направления, missing-last, tie-break по number, details/summary, включительный display end, сохранение focus и объявление количества (§20.4). |
| 70 | Высокий | [`pageData.Body template.HTML`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L18-L23) | Основное содержимое страниц строится строковой конкатенацией и принудительно помечается `template.HTML`, обходя автоэкранирование шаблона. Текущие отдельные значения экранируются вручную, но архитектура хрупкая: любое забытое поле превращается в XSS на локальном origin. |
| 71 | Высокий | webapp state/results | Ошибки scan, rejected reports и ошибки, при которых отчёт невозможно записать, не имеют типизированной таблицы operation errors. Report write failures остаются одной строкой `Progress.Event` и не выводятся в консоль, вопреки §15. |

### H. Запуск, остановка, тесты и документация

| № | Приоритет | Пакет / место | Конкретное расхождение |
|---:|---|---|---|
| 72 | Блокирующий | [`shutdownHandler`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/internal/webapp/app.go#L297-L305), [`cmd/validator.run`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/cmd/validator/main.go#L39-L61) | `shutdown` запускается в goroutine, возвращённая ошибка полностью игнорируется. `SHUTDOWN_FAILURE` не выводится, конечный timeout не отражается в exit code; `Serve` обычно вернёт `ErrServerClosed`, после чего процесс завершится кодом 0. |
| 73 | Блокирующий | весь набор тестов и docs | В репозитории только 10 test functions (по факту несколько базовых happy-path проверок). Нет registry/pipeline/lifecycle/integration tests, race snapshot test, строгих schema tests, report discovery, 20 route cases, cancellation, version modes, atomic fault matrix, HTML golden и filter/sort tests. Документы [`architecture.md`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/docs/architecture.md) и [`report-format.md`](https://github.com/verdoga/dsl-validator/blob/188a8e03aa4706c5bd5ff475a40d9b75e1c06e00/docs/report-format.md) при этом утверждают, что строгая schema, pipeline и атомарная замена реализованы; `adding-diagnostics.md` не фиксирует обязательную таблицу текущих parser-overlap. Это одновременно нарушение §§8, 22, 23 и недостоверная документация. |

## 4. Что реализовано корректно или близко к ТЗ

Чтобы не смешивать отсутствие дефекта с общей незавершённостью, отдельно отмечены работающие основы:

- адрес сервера фиксирован как `127.0.0.1:8580`, listener создаётся через `tcp4`;
- HTTP-сервер имеет конечные read/header/write/idle timeouts;
- Host allowlist содержит требуемые два значения;
- production `Diagnostics.All()` пуст, `SupportedVersions()` возвращает `1.1`;
- только `internal/parseradapter` импортирует внешний parser path среди файлов валидатора;
- parser issues берутся из `Document.ParserIssues()`, а не повторно из node/element;
- fatal result не запускает diagnostics;
- diagnostic error/panic изолируется на границе одной функции, уже принятые findings сохраняются;
- основные срезы AST, cursor paths и списки issues выдаются новыми срезами;
- обнаружение `.txt` регистронезависимое и сортировка файлов детерминирована;
- JSON сначала сериализуется в память, временный файл создаётся рядом, выполняются `Sync` и `Close`;
- второй анализ отклоняется флагом `running`; сами файлы внутри одной операции идут последовательно;
- базовые CSP, `nosniff`, skip-link, landmark `main`, status region, table caption и focus style присутствуют.

Эти пункты не компенсируют блокирующие расхождения выше, но их следует сохранить при переработке.

## 5. Рекомендуемый порядок исправления

1. Устранить нарушения dependency/build: внешний модуль, `go.sum`, Go 1.26; удалить `third_party` и `replace`.
2. Реализовать строгую модель/schema/decoder и Windows `MoveFileExW`; только после этого строить report discovery.
3. Реализовать workspace report matching и immutable scan snapshot.
4. Исправить parser result matrix, pipeline outcomes, повторный stat и context.
5. Перестроить web state: запрет/изоляция rescan во время operation, типизированные результаты, report IDs.
6. Реализовать обязательные страницы и server-side fallback, затем минимальный JS.
7. Добавить тесты по матрице §22 до объявления готовности; отдельно запустить `go test -race ./...` и Windows cross-build.
8. После фактической реализации исправить документацию и выполнить весь набор команд §23.

## 6. Вывод

Коммит `188a8e03...` нельзя считать реализацией `TZ.md`. Он содержит полезную заготовку пакетов и часть API-границы, но не реализует значительную часть пользовательского продукта и не выполняет ключевые требования безопасности/надёжности. До исправления блокирующих пунктов нельзя безопасно начинать добавление предметных диагностик: текущие report, workspace и web-контракты не дают надёжно сохранить, обнаружить и показать их результаты.


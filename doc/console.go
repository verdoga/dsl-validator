// Package console разбирает параметры dsl-validator, проверяет исходный путь
// и печатает русскоязычную справку. Пакет не читает содержимое JSON.
// PathInfo предоставляет общую проверку пути без символических ссылок
// для console, discovery и storage, чтобы не дублировать эту проверку.
package console

file params.go:

// Params — проверенные параметры одного последовательного запуска.
type Params struct {
	// Path — абсолютный очищенный путь к обычному JSON-файлу или каталогу.
	Path string
	// Depth — предел уровней подкаталогов; nil означает отсутствие ограничения.
	// Ноль означает только исходный каталог; для файла значение игнорируется.
	Depth *int
}

file parse.go:

// Parse принимает аргументы без имени программы. proceed=true означает
// действительные Params; false с nil — справку; false с err — отказ запуска.
// Ошибки параметров уже выведены в stderr; ошибки вывода только возвращаются.
// Проверяет исходную запись пути до очистки, затем её абсолютную форму.
func Parse(args []string, stdout, stderr io.Writer) (params Params, proceed bool, err error)

// parseFlags принимает только --depth N и один путь, а также --help/-h.
// Флаги стоят до пути; -- завершает их разбор. --replace не поддерживается.
// Пустой позиционный аргумент означает текущий каталог; отсутствие — ошибка.
// help=true означает запрос справки, false — обычный запуск.
func parseFlags(args []string) (params Params, help bool, err error)

// parseFlagArguments задаёт зарегистрированные флаги локального FlagSet
// и возвращает позиционные аргументы; сообщения об ошибках — на русском.
func parseFlagArguments(flags *flag.FlagSet, args []string) ([]string, error)

// parseDepth преобразует --depth в int не меньше нуля с проверкой переполнения.
func parseDepth(raw string) (depth int, err error)

file path.go:

// normalizePath получает абсолютный очищенный путь после проверки его
// исходной записи; пустая строка означает текущий каталог.
func normalizePath(raw string) (path string, err error)

// validateSourcePath допускает каталог либо обычный файл с расширением .json
// без учёта регистра. Проверяет PathInfo до очистки и после получения
// абсолютного пути, включая родителей текущего каталога.
func validateSourcePath(path string) error

// PathInfo проверяет каждый компонент пути через Lstat без следования ссылкам,
// включая компоненты перед ..; возвращает сведения о последнем компоненте.
// Не проверяет расширение и не читает содержимое. Пустой путь означает точку.
func PathInfo(path string) (os.FileInfo, error)

// sourceComponentInfo получает сведения об одном компоненте, отклоняя ссылку.
func sourceComponentInfo(path string) (os.FileInfo, error)

file help.go:

// PrintHelp печатает dsl-validator [--depth N] <path>, правила глубины,
// обработки JSON на месте и коды завершения; возвращает ошибку вывода.
func PrintHelp(output io.Writer) error

package console

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse принимает аргументы без имени программы. proceed=true означает
// действительные Params; false с nil — справку; false с err — отказ запуска.
// Ошибки параметров уже выведены в stderr; ошибки вывода только возвращаются.
// Проверяет исходную запись пути до очистки, затем её абсолютную форму.
func Parse(args []string, stdout, stderr io.Writer) (params Params, proceed bool, err error) {
	params, help, err := parseFlags(args)
	if err == nil && help {
		return Params{}, false, PrintHelp(stdout)
	}
	if err == nil {
		err = validateSourcePath(params.Path)
	}
	if err == nil {
		params.Path, err = normalizePath(params.Path)
	}
	if err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "Ошибка: %v\n", err); writeErr != nil {
			return Params{}, false, writeErr
		}
		return Params{}, false, err
	}
	return params, true, nil
}

// parseFlags принимает только --depth N и один путь, а также --help/-h.
// Флаги стоят до пути; -- завершает их разбор. --replace не поддерживается.
// Пустой позиционный аргумент означает текущий каталог; отсутствие — ошибка.
// help=true означает запрос справки, false — обычный запуск.
func parseFlags(args []string) (params Params, help bool, err error) {
	// Справка имеет приоритет даже над ошибками, встретившимися раньше неё.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		if arg == "--help" || arg == "-h" {
			return Params{}, true, nil
		}
		if arg == "--depth" && i+1 < len(args) {
			switch args[i+1] {
			case "--", "--depth", "--help", "-h":
			default:
				i++
			}
		}
	}

	flags := flag.NewFlagSet("dsl-validator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rawDepth := flags.String("depth", "", "Предел уровней подкаталогов")
	positional, err := parseFlagArguments(flags, args)
	if err != nil {
		return Params{}, false, err
	}
	if len(positional) == 0 {
		return Params{}, false, fmt.Errorf("не указан исходный путь")
	}
	if len(positional) != 1 {
		return Params{}, false, fmt.Errorf("нужно указать ровно один исходный путь")
	}
	params.Path = positional[0]
	if flags.NFlag() != 0 {
		depth, err := parseDepth(*rawDepth)
		if err != nil {
			return Params{}, false, err
		}
		params.Depth = &depth
	}
	return params, false, nil
}

// parseFlagArguments задаёт зарегистрированные флаги локального FlagSet
// и возвращает позиционные аргументы; сообщения об ошибках — на русском.
func parseFlagArguments(flags *flag.FlagSet, args []string) ([]string, error) {
	depthSeen := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--":
			return args[i+1:], nil
		case "--depth":
			if depthSeen {
				return nil, fmt.Errorf("параметр --depth указан повторно")
			}
			if i+1 == len(args) || args[i+1] == "--" {
				return nil, fmt.Errorf("после --depth требуется значение")
			}
			i++
			if err := flags.Set("depth", args[i]); err != nil {
				return nil, fmt.Errorf("не удалось задать --depth: %w", err)
			}
			depthSeen = true
		default:
			if args[i] == "-" || !strings.HasPrefix(args[i], "-") {
				return args[i:], nil
			}
			return nil, fmt.Errorf("неизвестный параметр %q", args[i])
		}
	}
	return nil, nil
}

// parseDepth преобразует --depth в int не меньше нуля с проверкой переполнения.
func parseDepth(raw string) (depth int, err error) {
	depth, err = strconv.Atoi(raw)
	if err != nil || depth < 0 {
		return 0, fmt.Errorf("значение --depth %q должно быть целым числом от 0 до %d", raw, int(^uint(0)>>1))
	}
	return depth, nil
}

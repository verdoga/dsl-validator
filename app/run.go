// Package app связывает параметры, discovery, реестр, storage, validator
// и reporter в одно последовательное приложение. Пакет владеет моделью
// текущего файла, добавляет Processing и измеряет время; не разбирает DSL,
// не реализует диагностики, не сериализует JSON и не обходит каталоги.
package app

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/verdoga/dsl-validator/checkers"
	"github.com/verdoga/dsl-validator/console"
	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/discovery"
	"github.com/verdoga/dsl-validator/reporter"
)

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
func Run(args []string, version string) int {
	params, proceed, err := console.Parse(args, os.Stdout, os.Stderr)
	if err != nil {
		// Parse получает *os.File: ошибка записи отличается от ошибки пути
		// операцией write. Неполная запись также является техническим отказом.
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Op == "write" || errors.Is(err, io.ErrShortWrite) {
			return exitFailure
		}
		return exitUsage
	}
	if !proceed {
		return exitSuccess
	}
	if version == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: версия инструмента не должна быть пустой")
		return exitFailure
	}
	paths, err := discovery.Find(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		return exitFailure
	}
	registry, err := diagnostics.NewRegistry(checkers.RegistrationCheckers())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		return exitFailure
	}
	if registry.Len() == 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: диагностики не зарегистрированы")
		return exitFailure
	}
	reports := reporter.New(os.Stdout, os.Stderr)
	if err := reports.Start(registry.Len()); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		return exitFailure
	}
	r := runner{version: version, registry: registry, reporter: reports}
	code := exitSuccess
	for _, path := range paths {
		if err := r.processFile(path); err != nil {
			code = exitFailure
			break
		}
	}
	finalCode, err := reports.Finish()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		return exitFailure
	}
	if code != exitSuccess {
		return code
	}
	return finalCode
}

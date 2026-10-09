// Package discovery находит обычные JSON-файлы и возвращает их абсолютные
// пути. Пакет не открывает JSON, не определяет принадлежность к DSL,
// не анализирует модель и не следует по символическим ссылкам.
package discovery

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/verdoga/dsl-validator/console"
)

// Find принимает проверенные Params и повторно проверяет исходный путь
// через console.PathInfo. Для файла возвращает один путь, для каталога —
// найденные JSON в пределах Depth. Расширение сравнивается без учёта регистра.
// Пути уникальны и лексикографически отсортированы даже при частичном отказе.
// Ошибка обхода возвращается вместе с найденными до отказа путями;
// app не обрабатывает этот частичный список. Пустой список без ошибки допустим.
func Find(params console.Params) (paths []string, err error) {
	info, err := console.PathInfo(params.Path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("путь %q не указывает на обычный файл или каталог", params.Path)
		}
		if !isJSON(params.Path) {
			return nil, fmt.Errorf("файл %q должен иметь расширение .json", params.Path)
		}
		return []string{params.Path}, nil
	}
	paths, err = walkDirectory(params.Path, params.Depth)
	slices.Sort(paths)
	return slices.Compact(paths), err
}

// walkDirectory собирает обычные JSON-файлы. Ноль ограничивает обход корнем,
// nil снимает ограничение; ссылки пропускаются. Первая ошибка обхода
// прекращает дальнейший поиск и возвращается с уже найденными путями.
func walkDirectory(root string, maxDepth *int) (paths []string, err error) {
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("не удалось обойти путь %q: %w", path, walkErr)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			if maxDepth != nil && path != root {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return fmt.Errorf("не удалось определить глубину пути %q: %w", path, err)
				}
				depth := strings.Count(filepath.ToSlash(relative), "/") + 1
				if depth > *maxDepth {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !isJSON(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("не удалось проверить файл %q: %w", path, err)
		}
		if info.Mode().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

// isJSON возвращает true для расширения .json без учёта регистра,
// false — для остальных расширений; JSONC не является входным форматом.
func isJSON(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}

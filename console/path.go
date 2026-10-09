package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// normalizePath получает абсолютный очищенный путь после проверки его
// исходной записи; пустая строка означает текущий каталог.
func normalizePath(raw string) (path string, err error) {
	if raw == "" {
		raw = "."
	}
	path, err = filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("не удалось получить абсолютный путь %q: %w", raw, err)
	}
	return path, nil
}

// validateSourcePath допускает каталог либо обычный файл с расширением .json
// без учёта регистра. Проверяет PathInfo до очистки и после получения
// абсолютного пути, включая родителей текущего каталога.
func validateSourcePath(path string) error {
	if _, err := PathInfo(path); err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("не удалось получить текущий каталог: %w", err)
		}
		// Родители текущего каталога могут исчезнуть при очистке "..".
		if _, err := PathInfo(workingDirectory); err != nil {
			return err
		}
	}
	absolute, err := normalizePath(path)
	if err != nil {
		return err
	}
	info, err := PathInfo(absolute)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("путь %q не указывает на обычный файл или каталог", absolute)
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".json") {
		return fmt.Errorf("файл %q должен иметь расширение .json", absolute)
	}
	return nil
}

// PathInfo проверяет каждый компонент пути через Lstat без следования ссылкам,
// включая компоненты перед ..; возвращает сведения о последнем компоненте.
// Не проверяет расширение и не читает содержимое. Пустой путь означает точку.
func PathInfo(path string) (os.FileInfo, error) {
	if path == "" {
		path = "."
	}
	separator := string(os.PathSeparator)
	volume := filepath.VolumeName(path)
	remainder := path[len(volume):]
	current := volume + "."
	if filepath.IsAbs(path) || (len(remainder) > 0 && os.IsPathSeparator(remainder[0])) {
		current = volume + separator
	}
	info, err := sourceComponentInfo(current)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(filepath.ToSlash(remainder), "/") {
		if component == "" {
			continue
		}
		// Join очистил бы путь и скрыл непроверенный компонент перед "..".
		if !os.IsPathSeparator(current[len(current)-1]) {
			current += separator
		}
		current += component
		info, err = sourceComponentInfo(current)
		if err != nil {
			return nil, err
		}
	}
	if os.IsPathSeparator(path[len(path)-1]) {
		// Завершающий разделитель требует каталога, даже если файл уже найден.
		return sourceComponentInfo(current + separator)
	}
	return info, nil
}

// sourceComponentInfo получает сведения об одном компоненте, отклоняя ссылку.
func sourceComponentInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось проверить компонент пути %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("символическая ссылка %q недопустима", path)
	}
	return info, nil
}

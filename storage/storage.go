// Package storage открывает и закрывает существующие JSON-файлы, распознаёт
// формат DSL и безопасно сохраняет дополненную модель по тому же пути.
// Пакет отвечает за JSON и файловый I/O; не запускает диагностики, не проверяет
// семантические инварианты ссылок, не считает уровни и не создаёт Processing.
// Неизвестные поля поддерживаемого формата сохраняются при записи.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/verdoga/dsl-validator/console"
	"github.com/verdoga/dsl-validator/model"
)

// Open проверяет обычный .json через console.PathInfo, открывает файл,
// полностью читает и закрывает его до возврата. Ошибка закрытия также возвращается.
// Распознаёт структуру Contract.jsonc 1.0 по содержимому, а не имени файла
// или Document.FilePath. Возвращает модель и связанный с ней Snapshot.
// При ошибке возвращает nil, nil, err и не изменяет файл.
func Open(path string) (result *model.Result, snapshot *Snapshot, err error) {
	expected, err := console.PathInfo(path)
	if err != nil {
		return nil, nil, err
	}
	if !expected.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("путь %q не указывает на обычный файл", path)
	}
	if !filepath.IsAbs(path) {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("не удалось получить текущий каталог: %w", err)
		}
		if _, err := console.PathInfo(workingDirectory); err != nil {
			return nil, nil, err
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось получить абсолютный путь %q: %w", path, err)
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".json") {
		return nil, nil, fmt.Errorf("файл %q должен иметь расширение .json", absolute)
	}
	source, info, err := readFile(absolute)
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(expected, info) {
		return nil, nil, fmt.Errorf("файл %q изменился при открытии", path)
	}
	decoded, err := decodeResult(source)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось разобрать JSON %q: %w", absolute, err)
	}
	return decoded, &Snapshot{path: absolute, original: source, info: info}, nil
}

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
func Save(snapshot *Snapshot, result *model.Result) (updated bool, err error) {
	if err := checkUnchanged(snapshot); err != nil {
		return false, err
	}
	payload, err := encodeUpdate(snapshot, result)
	if err != nil {
		return false, err
	}
	temporaryPath, err := writeTemporary(snapshot.path, payload, snapshot.info.Mode())
	if err != nil {
		return false, err
	}
	defer func() {
		if !updated {
			if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("не удалось удалить временный файл %q: %w", temporaryPath, removeErr))
			}
		}
	}()
	return replaceExisting(temporaryPath, snapshot.path)
}

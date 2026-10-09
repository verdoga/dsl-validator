package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// checkUnchanged повторно проверяет путь без ссылок, идентичность обычного
// файла и точное совпадение его байтов с snapshot.original; при расхождении
// возвращает ошибку без записи. Дескрипторы открывает и закрывает внутри.
func checkUnchanged(snapshot *Snapshot) error {
	if snapshot == nil || snapshot.info == nil {
		return fmt.Errorf("для проверки требуется снимок исходного файла")
	}
	source, info, err := readFile(snapshot.path)
	if err != nil {
		return fmt.Errorf("не удалось проверить исходный файл: %w", err)
	}
	if !os.SameFile(snapshot.info, info) {
		return fmt.Errorf("исходный файл %q заменён", snapshot.path)
	}
	if !bytes.Equal(source, snapshot.original) {
		return fmt.Errorf("содержимое исходного файла %q изменилось", snapshot.path)
	}
	return nil
}

// writeTemporary создаёт временный файл в каталоге назначения, записывает
// полный payload, сохраняет доступные права исходного файла, вызывает Sync
// и закрывает его. При отказе удаляет незавершённый временный файл.
func writeTemporary(targetPath string, payload []byte, mode os.FileMode) (temporaryPath string, err error) {
	file, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("не удалось создать временный файл для %q: %w", targetPath, err)
	}
	path := file.Name()
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("не удалось закрыть временный файл %q: %w", path, closeErr))
		}
		if err != nil {
			if removeErr := os.Remove(path); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("не удалось удалить временный файл %q: %w", path, removeErr))
			}
			temporaryPath = ""
		}
	}()
	written, err := file.Write(payload)
	if err == nil && written != len(payload) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return "", fmt.Errorf("не удалось полностью записать временный файл %q: %w", path, err)
	}
	permissions := mode.Perm() | mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky)
	if err := file.Chmod(permissions); err != nil {
		return "", fmt.Errorf("не удалось сохранить права временного файла %q: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("не удалось синхронизировать временный файл %q: %w", path, err)
	}
	return path, nil
}

// replaceExisting резервирует старый файл и устанавливает подготовленный.
// При отказе установки восстанавливает прежний путь; ошибки восстановления
// и очистки сохраняет в err, не удаляя единственную пригодную копию данных.
// updated=true означает установленную новую версию; false — отсутствие установки.
// Не обещает атомарность нескольких переименований при аварийном отключении ОС.
func replaceExisting(temporaryPath, targetPath string) (updated bool, err error) {
	reserved, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".backup-*")
	if err != nil {
		return false, fmt.Errorf("не удалось зарезервировать путь для %q: %w", targetPath, err)
	}
	backupPath := reserved.Name()
	if closeErr := reserved.Close(); closeErr != nil {
		err = fmt.Errorf("не удалось закрыть резервный файл %q: %w", backupPath, closeErr)
		if removeErr := os.Remove(backupPath); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("не удалось удалить пустой резерв %q: %w", backupPath, removeErr))
		}
		return false, err
	}
	if renameErr := os.Rename(targetPath, backupPath); renameErr != nil {
		err = fmt.Errorf("не удалось переместить исходный файл %q в резерв %q: %w", targetPath, backupPath, renameErr)
		if removeErr := os.Remove(backupPath); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("не удалось удалить пустой резерв %q: %w", backupPath, removeErr))
		}
		return false, err
	}
	if installErr := os.Rename(temporaryPath, targetPath); installErr != nil {
		err = fmt.Errorf("не удалось установить файл %q по пути %q: %w", temporaryPath, targetPath, installErr)
		if restoreErr := os.Rename(backupPath, targetPath); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("не удалось восстановить %q; старый JSON сохранён в резерве %q: %w", targetPath, backupPath, restoreErr))
		}
		return false, err
	}
	if removeErr := os.Remove(backupPath); removeErr != nil {
		return true, fmt.Errorf("новый JSON установлен; не удалось удалить резерв %q: %w", backupPath, removeErr)
	}
	return true, nil
}

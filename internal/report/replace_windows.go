//go:build windows

package report

import "os"

// replaceFile атомарно заменяет файл средствами стандартной библиотеки.
func replaceFile(source, target string) error {
	return os.Rename(source, target)
}

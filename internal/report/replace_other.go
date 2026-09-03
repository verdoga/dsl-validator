//go:build !windows

package report

import "os"

// replaceFile атомарно заменяет файл на системах, отличных от Windows.
func replaceFile(source, target string) error { return os.Rename(source, target) }

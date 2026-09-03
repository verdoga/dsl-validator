//go:build !windows

package workspace

// isWindows сообщает, применяется ли регистронезависимое сравнение путей.
func isWindows() bool { return false }

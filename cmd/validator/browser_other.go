//go:build !windows

package main

import "fmt"

// openBrowser сообщает, что автоматическое открытие поддерживается только в Windows.
func openBrowser(string) error {
	return fmt.Errorf("автоматическое открытие браузера доступно только в Windows")
}

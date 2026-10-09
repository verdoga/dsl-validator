// Package main задаёт точку входа единственного консольного приложения
// dsl-validator для Go 1.25. Пакет передаёт аргументы и версию в app;
// только здесь вызывается os.Exit. Разбор DSL и работа с JSON сюда не входят.
package main

file main.go:

// version — версия сборки; значение dev заменяется через -ldflags -X main.version.
var version string = "dev"

// main передаёт os.Args[1:] и version в app.Run, затем завершает процесс
// с возвращённым кодом. Других точек входа и фоновых процессов нет.
func main()

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"dslparser/Diagnostics"
	"dslparser/internal/parseradapter"
	"dslparser/internal/pipeline"
	"dslparser/internal/webapp"
	"dslparser/internal/workspace"
)

// serverAddress задаёт единственный разрешённый IPv4 loopback endpoint.
const serverAddress = "127.0.0.1:8580"

// main запускает приложение и выставляет код завершения.
func main() { os.Exit(run()) }

// run выполняет проверку, bind, serve и корректное завершение.
func run() int {
	if err := diagnostics.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "DIAGNOSTIC_REGISTRY_FAILURE:", err)
		return 1
	}
	parser := parseradapter.New()
	runner := pipeline.New(parser)
	scanner := workspace.NewScanner(runner)
	listener, err := net.Listen("tcp4", serverAddress)
	if err != nil {
		fmt.Fprintln(os.Stderr, "SERVER_BIND_FAILURE:", err)
		return 1
	}
	var server *http.Server
	shutdown := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
	app, err := webapp.New(scanner, runner, shutdown)
	if err != nil {
		listener.Close()
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	server = &http.Server{Addr: serverAddress, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	url := "http://" + serverAddress + "/"
	if err := openBrowser(url); err != nil {
		fmt.Fprintln(os.Stderr, "BROWSER_OPEN_FAILURE:", err, "Откройте", url)
	}
	err = server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "SERVER_RUNTIME_FAILURE:", err)
		return 1
	}
	return 0
}

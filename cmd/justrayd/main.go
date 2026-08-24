package main

// DAEMON ENTRYPOINT

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"

	"github.com/luynrs/justray/internal/daemon/connection"
	"github.com/luynrs/justray/internal/daemon/engine/singbox"
	"github.com/luynrs/justray/internal/daemon/server"
	"github.com/luynrs/justray/internal/daemon/store"
	"github.com/luynrs/justray/internal/daemon/subscription"
	"github.com/luynrs/justray/internal/shared/rpc"
)

func main() {
	dir, err := rpc.Dir()
	if err != nil {
		die("resolve config dir:", err)
	}
	if err := rpc.EnsureDir(dir); err != nil {
		die("create config dir:", err)
	}

	logFile, err := os.OpenFile(rpc.DaemonLog(dir), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		die("open log file:", err)
	}
	defer func() { _ = logFile.Close() }()
	logger := log.NewWithOptions(io.MultiWriter(os.Stderr, logFile), log.Options{
		ReportTimestamp: true,
		Prefix:          "justrayd",
	})

	if err := rpc.ClearLog(rpc.EngineLog(dir)); err != nil {
		logger.Print(err)
	}

	socket := rpc.Socket(dir)
	ln, err := server.Listen(socket)
	if err != nil {
		if strings.Contains(err.Error(), "already listening") {
			logger.Printf("%v, exiting", err)
			return
		}
		logger.Fatal(err)
	}
	logger.Printf("listening on %s", socket)

	st := store.Disk{Dir: dir}
	conn := connection.New(dir, st, singbox.New, singbox.Probe, logger)
	subs := subscription.New(st, logger)
	srv := server.New(logger, conn, subs)
	srv.Restore()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	defer close(done)
	go srv.AutoRefresh(done)

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case s := <-sig:
		logger.Printf("shutting down (%s)", s)
	case err := <-served:
		logger.Printf("shutting down (%v)", err)
	}

	_ = ln.Close()
	srv.Shutdown()
}

func die(v ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"justrayd:"}, v...)...)
	os.Exit(1)
}

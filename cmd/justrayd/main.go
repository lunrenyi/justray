package main

//
// DAEMON ENTRYPOINT
//

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"

	"github.com/luynrs/justray/internal/daemon"
)

func main() {
	dir := flag.String("config-dir", "", "config directory (default: $JUSTRAY_CONFIG_DIR, else the OS user config dir + /justray)")
	flag.Parse()

	if *dir == "" {
		d, err := daemon.Dir()
		if err != nil {
			die("resolve config dir:", err)
		}
		*dir = d
	}
	if err := daemon.EnsureDir(*dir); err != nil {
		die("create config dir:", err)
	}

	logFile, err := os.OpenFile(daemon.DaemonLog(*dir), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		die("open log file:", err)
	}
	defer logFile.Close()
	logger := log.NewWithOptions(io.MultiWriter(os.Stderr, logFile), log.Options{
		ReportTimestamp: true,
		Prefix:          "justrayd",
	})

	socket := daemon.Socket(*dir)
	ln, err := daemon.Listen(socket)
	if err != nil {
		if strings.Contains(err.Error(), "already listening") {
			logger.Printf("%v, exiting", err)
			return
		}
		logger.Fatal(err)
	}
	logger.Printf("listening on %s", socket)

	srv := daemon.New(*dir, logger)
	srv.Restore()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case s := <-sig:
		logger.Printf("shutting down (%s)", s)
	case err := <-served:
		logger.Printf("shutting down (%v)", err)
	}

	ln.Close()
	srv.Shutdown()
}

func die(v ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"justrayd:"}, v...)...)
	os.Exit(1)
}

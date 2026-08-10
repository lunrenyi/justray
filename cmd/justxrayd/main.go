package main

//
// DAEMON
//

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/luynrs/justxray/internal/daemon"
)

func main() {
	dir := flag.String("config-dir", "", "config directory (default: $JUSTXRAY_CONFIG_DIR, else the OS user config dir + /justxray)")
	xrayBin := flag.String("xray-bin", "xray", "path to the xray-core binary")
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

	logFile, err := os.OpenFile(daemon.DaemonLog(*dir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		die("open log file:", err)
	}
	defer logFile.Close()
	logger := log.New(io.MultiWriter(os.Stderr, logFile), "justxrayd: ", log.LstdFlags)

	bin, err := exec.LookPath(*xrayBin)
	if err != nil {
		bin = *xrayBin
		logger.Printf("warning: xray binary %q not in PATH", bin)
	}

	socket := daemon.Socket(*dir)
	ln, err := daemon.Listen(socket)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("listening on %s (xray-bin=%s)", socket, bin)

	srv := daemon.New(*dir, bin, logger)
	srv.Restore()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case s := <-sig:
		logger.Printf("got %s, shutting down", s)
	case err := <-served:
		logger.Printf("serve: %v", err)
	}

	ln.Close()
	srv.Shutdown()
	logger.Print("stopped")
}

func die(v ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"justxrayd:"}, v...)...)
	os.Exit(1)
}

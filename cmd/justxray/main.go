package main

//
// TUI
//

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/luynrs/justxray/internal/daemon"
	"github.com/luynrs/justxray/internal/ui"
)

func main() {
	dir := flag.String("config-dir", "", "config directory (default: $JUSTXRAY_CONFIG_DIR, else the OS user config dir + /justxray)")
	xrayBin := flag.String("xray-bin", "xray", "path to the xray-core binary, passed on to a daemon we spawn")
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

	client := daemon.NewClient(daemon.Socket(*dir))
	if client.Ping() != nil {
		fmt.Println("justxray: no daemon running, starting justxrayd in the background…")
		if err := spawn(*dir, *xrayBin); err != nil {
			die("start daemon:", err)
		}
		if err := wait(client, 10*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, "justxray:", err, "— see", daemon.DaemonLog(*dir))
			os.Exit(1)
		}
	}

	if err := ui.Run(client); err != nil {
		die(err)
	}
}

func spawn(dir, xrayBin string) error {
	bin, err := justxrayd()
	if err != nil {
		return err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(bin, "--config-dir", dir, "--xray-bin", xrayBin)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start() // detached on purpose
}

func justxrayd() (string, error) {
	if self, err := os.Executable(); err == nil {
		bin := filepath.Join(filepath.Dir(self), "justxrayd")
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
	}
	bin, err := exec.LookPath("justxrayd")
	if err != nil {
		return "", fmt.Errorf("justxrayd not found next to justxray or in PATH; build it with \"go build ./cmd/justxrayd\"")
	}
	return bin, nil
}

func wait(c *daemon.Client, timeout time.Duration) error {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		if c.Ping() == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not come up within %s", timeout)
}

func die(v ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"justxray:"}, v...)...)
	os.Exit(1)
}

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
	"runtime"
	"time"

	"github.com/luynrs/justray/internal/daemon"
	"github.com/luynrs/justray/internal/daemon/procgroup"
	"github.com/luynrs/justray/internal/ui"
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

	client := daemon.NewClient(daemon.Socket(*dir))
	if client.Ping() != nil {
		fmt.Println("justray: no daemon running, starting justrayd in the background…")
		if err := spawn(*dir); err != nil {
			die("start daemon:", err)
		}
		if err := wait(client, 10*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, "justray:", err, "— see", daemon.DaemonLog(*dir))
			os.Exit(1)
		}
	}

	if err := ui.Run(client); err != nil {
		die(err)
	}
}

func spawn(dir string) error {
	bin, err := justrayd()
	if err != nil {
		return err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(bin, "--config-dir", dir)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	procgroup.Detach(cmd)
	return cmd.Start() // detached on purpose
}

func justrayd() (string, error) {
	if bin := nextToSelf("justrayd"); bin != "" {
		return bin, nil
	}
	bin, err := exec.LookPath(exeName("justrayd"))
	if err != nil {
		return "", fmt.Errorf("justrayd not found next to justray or in PATH; build it with \"go build ./cmd/justrayd\"")
	}
	return bin, nil
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func nextToSelf(name string) string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(self), exeName(name))
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
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
	fmt.Fprintln(os.Stderr, append([]any{"justray:"}, v...)...)
	os.Exit(1)
}

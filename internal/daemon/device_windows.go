//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

// justrayd runs detached with no console, so a child console app (the
// powershell calls above) would otherwise get a fresh, visible one.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

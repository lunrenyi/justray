//go:build windows

package console

import (
	"os/exec"
	"syscall"
)

// justrayd runs detached with no console, so a child console app would
// otherwise get a fresh, visible one.
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

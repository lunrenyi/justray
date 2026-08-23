//go:build windows

package console

import (
	"os/exec"
	"syscall"
)

// justrayd runs detached, so a child console app would pop a visible window
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

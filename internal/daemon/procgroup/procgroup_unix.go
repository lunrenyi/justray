//go:build unix

package procgroup

import (
	"os/exec"
	"syscall"
)

func Detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

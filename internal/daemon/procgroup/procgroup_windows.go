//go:build windows

package procgroup

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func Setup(cmd *exec.Cmd) {}

func Terminate(cmd *exec.Cmd) { cmd.Process.Kill() }

func Kill(cmd *exec.Cmd) { cmd.Process.Kill() }

func Detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008} // DETACHED_PROCESS
}

func init() {
	windows.SetConsoleOutputCP(65001) // utf8
}

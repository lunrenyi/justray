//go:build !windows

package console

import "os/exec"

func Hide(cmd *exec.Cmd) {}

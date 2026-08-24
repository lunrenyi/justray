//go:build windows

package autostart

import (
	"fmt"
	"os"
	"os/exec"
)

const name = "justrayd"

func Enabled() bool {
	return exec.Command("schtasks", "/Query", "/TN", name).Run() == nil
}

func Enable() error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("schtasks", "/Create", "/F", "/RL", "HIGHEST", "/SC", "ONLOGON", "/TN", name, "/TR", `"`+bin+`"`)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks create: %v: %s", err, out)
	}
	return nil
}

func Disable() error {
	_ = exec.Command("schtasks", "/Delete", "/F", "/TN", name).Run()
	return nil
}

//go:build windows

package elevate

import (
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/sys/windows"
)

func Needed(err error) bool {
	if err == nil || windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "access is denied") || strings.Contains(e, "operation not permitted")
}

func Tun(logger *log.Logger, dir string) {
	self, err := os.Executable()
	if err != nil {
		logger.Printf("elevate: %v", err)
		return
	}

	ps := "Start-Process -FilePath $env:JUSTRAY_ELEVATE -ArgumentList '--config-dir',$env:JUSTRAY_DIR -Verb RunAs -WindowStyle Hidden"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.Env = append(os.Environ(), "JUSTRAY_ELEVATE="+self, "JUSTRAY_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		logger.Printf("elevate: powershell: %v: %s", err, out)
		return
	}

	logger.Print("elevate: relaunched elevated, exiting")
	os.Exit(0)
}

//go:build darwin

package elevate

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func Needed(err error) bool {
	self, _ := os.Executable()
	return err != nil && strings.Contains(err.Error(), "operation not permitted") && !isSetuidRoot(self)
}

func Tun(logger *log.Logger, dir string) {
	target, err := cachedCopy(dir)
	if err != nil {
		logger.Print(err)
		return
	}

	if !isSetuidRoot(target) {
		script := `do shell script "chown root:wheel \"$JUSTRAY_ELEVATE\" && chmod u+s \"$JUSTRAY_ELEVATE\"" with administrator privileges`
		cmd := exec.Command("osascript", "-e", script)
		cmd.Env = append(os.Environ(), "JUSTRAY_ELEVATE="+target)
		if out, err := cmd.CombinedOutput(); err != nil {
			logger.Printf("%v: %s", err, out)
			return
		}
	}

	if err := syscall.Exec(target, os.Args, os.Environ()); err != nil {
		logger.Print(err)
	}
}

func isSetuidRoot(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && info.Mode()&os.ModeSetuid != 0
}

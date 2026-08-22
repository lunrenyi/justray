//go:build windows

package elevate

import (
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/sys/windows"
)

// elevatedArg guards against re-prompting UAC forever: Restore() retries a
// persisted connection on every daemon startup, so one denied/failed relaunch
// would otherwise loop indefinitely.
const elevatedArg = "--elevated"

func Needed(err error) bool {
	if err == nil || windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "access is denied") || strings.Contains(e, "operation not permitted")
}

func Tun(logger *log.Logger, _ string) {
	if slices.Contains(os.Args, elevatedArg) {
		logger.Print("elevate: already relaunched elevated once, not retrying")
		return
	}

	self, err := os.Executable()
	if err != nil {
		logger.Printf("elevate: %v", err)
		return
	}

	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(self)
	args, _ := windows.UTF16PtrFromString(elevatedArg)
	if err := windows.ShellExecute(0, verb, file, args, nil, windows.SW_HIDE); err != nil {
		logger.Printf("elevate: runas: %v", err)
		return
	}

	logger.Print("elevate: relaunched elevated, exiting")
	os.Exit(0)
}

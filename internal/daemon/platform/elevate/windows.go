//go:build windows

package elevate

import (
	"errors"
	"os"
	"slices"
	"strings"

	"golang.org/x/sys/windows"
)

// elevatedArg stops Restore() from re-prompting UAC on every startup
const elevatedArg = "--elevated"

func Needed(err error) bool {
	if err == nil || windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "access is denied") || strings.Contains(e, "operation not permitted")
}

func Restart(_ string) error {
	if slices.Contains(os.Args, elevatedArg) {
		return nil
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}

	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(self)
	args, _ := windows.UTF16PtrFromString(elevatedArg)
	if err := windows.ShellExecute(0, verb, file, args, nil, windows.SW_HIDE); err != nil {
		return errors.New("could not grant permissions")
	}
	return nil
}

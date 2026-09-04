//go:build windows

package lock

import (
	"os"
	"strconv"

	"golang.org/x/sys/windows"
)

func File(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	o := &windows.Overlapped{Offset: 4096}
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, o); err != nil {
		_ = f.Close()
		if err == windows.ERROR_LOCK_VIOLATION {
			return nil, ErrLocked
		}
		return nil, err
	}
	exe, err := os.Executable()
	if err == nil {
		err = f.Truncate(0)
	}
	if err == nil {
		_, err = f.WriteString(strconv.Itoa(os.Getpid()) + "\n" + exe)
	}
	if err != nil {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, o)
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, o)
		_ = f.Close()
	}, nil
}

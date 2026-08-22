package rpc

import (
	"os"
	"path/filepath"
)

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "justray"), nil
}

func EnsureDir(dir string) error {
	for _, sub := range []string{"logs", "ipc"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			return err
		}
	}
	return os.Chmod(dir, 0o700)
}

func Socket(dir string) string    { return filepath.Join(dir, "ipc", "justrayd.sock") }
func DaemonLog(dir string) string { return filepath.Join(dir, "logs", "daemon.log") }
func CoreLog(dir string) string   { return filepath.Join(dir, "logs", "core.log") }

func ClearLog(path string) error { return os.Truncate(path, 0) }

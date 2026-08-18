package daemon

import (
	"os"
	"path/filepath"
)

// $JUSTRAY_CONFIG_DIR, .config/justray
func Dir() (string, error) {
	if d := os.Getenv("JUSTRAY_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "justray"), nil
}

// creds
func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }

func Socket(dir string) string    { return filepath.Join(dir, "justrayd.sock") }
func DaemonLog(dir string) string { return filepath.Join(dir, "justrayd.log") }

func coreLog(dir string) string { return filepath.Join(dir, "core.log") }

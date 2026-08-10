package daemon

import (
	"os"
	"path/filepath"
)

// $JUSTXRAY_CONFIG_DIR, .config/justxray
func Dir() (string, error) {
	if d := os.Getenv("JUSTXRAY_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "justxray"), nil
}

// creds
func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }

func Socket(dir string) string    { return filepath.Join(dir, "justxrayd.sock") }
func DaemonLog(dir string) string { return filepath.Join(dir, "justxrayd.log") }

func hwidPath(dir string) string { return filepath.Join(dir, "hwid") }
func xrayConf(dir string) string { return filepath.Join(dir, "xray-config.json") }
func xrayLog(dir string) string  { return filepath.Join(dir, "xray.log") }

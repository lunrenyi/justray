//go:build darwin

package owner

import "os"

func File(path string) error {
	if os.Geteuid() == 0 && os.Getuid() != 0 {
		return os.Chown(path, os.Getuid(), os.Getgid())
	}
	return nil
}

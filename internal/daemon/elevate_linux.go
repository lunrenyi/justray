//go:build linux

package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func tunPermissionErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "operation not permitted")
}

// CAP_NET_ADMIN
func elevateTun(logger *log.Logger, dir string) {
	self, err := os.Executable()
	if err != nil {
		logger.Printf("elevate: %v", err)
		return
	}

	target, err := cachedCopy(self, dir)
	if err != nil {
		logger.Printf("elevate: %v", err)
		return
	}

	if !hasNetAdmin(target) {
		elevate := "pkexec"
		if _, err := exec.LookPath(elevate); err != nil {
			elevate = "sudo"
		}
		if out, err := exec.Command(elevate, "setcap", "cap_net_admin+ep", target).CombinedOutput(); err != nil {
			logger.Printf("elevate: setcap: %v: %s", err, out)
			return
		}
	}

	logger.Print("elevate: got cap_net_admin, restarting")
	if err := syscall.Exec(target, os.Args, append(os.Environ(), tunEnv+"=1")); err != nil {
		logger.Printf("elevate: re-exec: %v", err)
	}
}

func cachedCopy(self, dir string) (string, error) {
	sum, err := hashFile(self)
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(dir, "elevated")
	target := filepath.Join(cacheDir, sum+"-justrayd")
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	if err := copyFile(self, target); err != nil {
		return "", err
	}
	return target, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)[:8]), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func hasNetAdmin(path string) bool {
	buf := make([]byte, 128)
	n, err := syscall.Getxattr(path, "security.capability", buf)
	return err == nil && n > 0
}

//go:build linux

package elevate

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

const tunEnv = "JUSTRAY_TUN"

func Needed(err error) bool {
	self, _ := os.Executable()
	return err != nil && strings.Contains(err.Error(), "operation not permitted") && !hasNetAdmin(self)
}

// set across the re-exec below, so the daemon comes back with tun still on
func Restarted() bool { return os.Getenv(tunEnv) == "1" }

// CAP_NET_ADMIN
func Tun(logger *log.Logger, dir string) {
	target, err := cachedCopy(dir)
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

func cachedCopy(dir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	sum, err := hashFile(self)
	if err != nil {
		return "", err
	}

	root := filepath.Join(dir, "elevated")
	target := filepath.Join(root, sum, "justrayd")
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	if err := os.RemoveAll(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	return target, copyFile(self, target)
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

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

func hasNetAdmin(path string) bool {
	buf := make([]byte, 128)
	n, err := syscall.Getxattr(path, "security.capability", buf)
	return err == nil && n > 0
}

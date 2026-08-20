//go:build linux

package elevate

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
)

func Needed(err error) bool {
	self, _ := os.Executable()
	return err != nil && strings.Contains(err.Error(), "operation not permitted") && !hasNetAdmin(self)
}

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
	if err := syscall.Exec(target, os.Args, os.Environ()); err != nil {
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
	if verified(target, sum) {
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

func verified(path, sum string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	got, err := hashFile(path)
	return err == nil && got == sum
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

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o700)
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

const capNetAdmin = 12

func hasNetAdmin(path string) bool {
	buf := make([]byte, 32) // fits VFS_CAP_REVISION_3 (24 bytes)
	n, err := syscall.Getxattr(path, "security.capability", buf)
	if err != nil || n < 8 {
		return false
	}
	if binary.LittleEndian.Uint32(buf[0:4])&0x1 == 0 {
		return false
	}
	return binary.LittleEndian.Uint32(buf[4:8])&(1<<capNetAdmin) != 0
}

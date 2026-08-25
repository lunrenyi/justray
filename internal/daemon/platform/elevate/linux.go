//go:build linux

package elevate

import (
	"encoding/binary"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func Needed(err error) bool {
	self, _ := os.Executable()
	return err != nil && strings.Contains(err.Error(), "operation not permitted") && !hasNetAdmin(self)
}

func Tun(logger *log.Logger, dir string) {
	target, err := cachedCopy(dir)
	if err != nil {
		logger.Print(err)
		return
	}

	if !hasNetAdmin(target) {
		elevate := "pkexec"
		if _, err := exec.LookPath(elevate); err != nil {
			elevate = "sudo"
		}
		if out, err := exec.Command(elevate, "setcap", "cap_net_admin+ep", target).CombinedOutput(); err != nil {
			logger.Printf("%v: %s", err, out)
			return
		}
	}

	if err := syscall.Exec(target, os.Args, os.Environ()); err != nil {
		logger.Print(err)
	}
}

func hasNetAdmin(path string) bool {
	buf := make([]byte, 32) // fits VFS_CAP_REVISION_3 (24 bytes)
	n, err := syscall.Getxattr(path, "security.capability", buf)
	if err != nil || n < 8 {
		return false
	}
	if binary.LittleEndian.Uint32(buf[0:4])&0x1 == 0 {
		return false
	}
	return binary.LittleEndian.Uint32(buf[4:8])&(1<<12) != 0 // CAP_NET_ADMIN
}

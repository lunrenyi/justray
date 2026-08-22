//go:build linux || darwin

package elevate

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

func cachedCopy(dir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	sum, err := hashFile(self)
	if err != nil {
		return "", err
	}

	target := filepath.Join(dir, "elevated", "justrayd")
	if verified(target, sum) {
		return target, nil
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
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
